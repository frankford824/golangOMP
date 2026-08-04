#!/usr/bin/env python3
"""Fail-closed G0-G10 finalizer for one formal V8 A/B run.

The finalizer never executes a migration, database query, HTTP request, or UI
action.  It verifies hashes and the small, stable result envelopes produced by
those independent executors.  A release can be ``GO`` only when every required
artifact is present, hash-bound, reports ``PASS``, and the three required
review roles sign the exact evidence index.
"""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import pathlib
import re
from typing import Any

try:
    from scripts.ab import clone_b_auth_policy
except ModuleNotFoundError:
    import clone_b_auth_policy


SHA256 = re.compile(r"^[0-9a-f]{64}$")
G4_ROW_FINGERPRINT_ALGORITHM = (
    "sha256(sorted(sha256(canonical-json-cells-v1)),duplicates-preserved)-v1"
)
RUN_ID = re.compile(r"^[a-z0-9][a-z0-9_-]{2,63}$")
GATES = tuple(f"G{index}" for index in range(11))
REQUIRED_ROLES = {
    "independent_sql_verifier",
    "adversarial_reviewer",
    "release_commander",
}
EMPTY_SHA256 = hashlib.sha256(b"").hexdigest()
SQL_GATE_NAMES = (
    "00_snapshot_fingerprint",
    "01_task_state_parity",
    "02_group_coverage",
    "03_revision_chain",
    "04_asset_role_scope",
    "05_reference_integrity",
    "06_storage_integrity",
    "07_event_history_checksum",
    "08_planning_retouch",
    "09_search_publish_outbox",
    "10_negative_assertions",
    "11_manifest_state",
    "12_legacy_timestamp_contract",
)
MANIFEST_DATABASE_GATES = {
    "G01",
    "G02",
    "G03",
    "G04",
    "G05",
    "G07",
    "G08",
    "G09",
}
API_COMBINATIONS = {
    "external_external_a": ("external", "external", "A"),
    "dev_dev_b": ("dev-plus", "dev-plus", "B"),
    "external_dev_b": ("external", "dev-plus", "B"),
    "dev_external_a": ("dev-plus", "external", "A"),
}


def canonical_bytes(value: Any) -> bytes:
    return (
        json.dumps(
            value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
        )
        + "\n"
    ).encode("utf-8")


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def read_json(path: pathlib.Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{path}: expected a JSON object")
    return value


def safe_artifact_path(run_dir: pathlib.Path, value: str) -> pathlib.Path:
    candidate = pathlib.PurePosixPath(value)
    if (
        candidate.is_absolute()
        or not candidate.parts
        or any(part in {"", ".", ".."} for part in candidate.parts)
    ):
        raise ValueError(f"unsafe artifact path: {value!r}")
    path = run_dir.joinpath(*candidate.parts)
    resolved_parent = path.parent.resolve()
    try:
        resolved_parent.relative_to(run_dir)
    except ValueError:
        raise ValueError(f"artifact escapes run directory: {value!r}") from None
    if path.is_symlink() or not path.is_file():
        raise ValueError(f"artifact is missing or symlinked: {value!r}")
    return path


def is_pass(payload: dict[str, Any]) -> bool:
    return (
        payload.get("status") == "PASS"
        and payload.get("violation_count", 0) == 0
    )


def require_hash(value: Any, label: str) -> str:
    text = str(value or "")
    if not SHA256.fullmatch(text):
        raise ValueError(f"{label} must be a lowercase SHA-256")
    return text


def is_int(value: Any, *, minimum: int = 0) -> bool:
    return (
        isinstance(value, int)
        and not isinstance(value, bool)
        and value >= minimum
    )


def exact_fields(
    payload: dict[str, Any], expected: set[str], label: str
) -> list[str]:
    actual = set(payload)
    if actual == expected:
        return []
    return [
        f"{label} field contract differs: "
        f"missing={sorted(expected - actual)},unexpected={sorted(actual - expected)}"
    ]


def valid_violation_rows(value: Any) -> bool:
    return isinstance(value, list) and all(
        isinstance(row, dict)
        and set(row) >= {"violation_code", "entity_key", "detail"}
        and all(
            isinstance(row.get(field), str)
            for field in ("violation_code", "entity_key", "detail")
        )
        for row in value
    )


def validate_common_envelope(
    payload: dict[str, Any],
    *,
    label: str,
    run_id_required: bool = True,
) -> list[str]:
    violations: list[str] = []
    if payload.get("schema_version") != 1:
        violations.append(f"{label} schema_version must be 1")
    if run_id_required and not RUN_ID.fullmatch(str(payload.get("run_id") or "")):
        violations.append(f"{label} run_id is invalid")
    if payload.get("status") != "PASS":
        violations.append(f"{label} status must be PASS")
    count = payload.get("violation_count")
    rows = payload.get("violations")
    if not is_int(count) or not valid_violation_rows(rows):
        violations.append(f"{label} violation envelope is invalid")
    elif count != len(rows) or count != 0:
        violations.append(f"{label} violation count is not an exact empty set")
    return violations


def validate_self_hash(
    payload: dict[str, Any],
    *,
    field: str,
    label: str,
    newline: bool,
) -> list[str]:
    value = str(payload.get(field) or "")
    if not SHA256.fullmatch(value):
        return [f"{label} {field} is invalid"]
    unsigned = {key: item for key, item in payload.items() if key != field}
    encoded = (
        canonical_bytes(unsigned)
        if newline
        else json.dumps(
            unsigned,
            ensure_ascii=False,
            sort_keys=True,
            separators=(",", ":"),
        ).encode("utf-8")
    )
    if hashlib.sha256(encoded).hexdigest() != value:
        return [f"{label} {field} does not bind the artifact contents"]
    return []


def validate_g1(payload: dict[str, Any]) -> list[str]:
    fields = {
        "schema_version",
        "run_id",
        "status",
        "violation_count",
        "violations",
        "snapshot_sha256",
        "baseline_fingerprint_sha256",
        "source_attestation_sha256",
        "target_attestation_sha256",
        "evidence_sha256",
    }
    violations = exact_fields(payload, fields, "G1")
    violations.extend(validate_common_envelope(payload, label="G1"))
    for field in (
        "snapshot_sha256",
        "baseline_fingerprint_sha256",
        "source_attestation_sha256",
        "target_attestation_sha256",
    ):
        try:
            require_hash(payload.get(field), f"G1.{field}")
        except ValueError as exc:
            violations.append(str(exc))
    violations.extend(
        validate_self_hash(
            payload,
            field="evidence_sha256",
            label="G1",
            newline=True,
        )
    )
    return violations


def validate_g2(payload: dict[str, Any]) -> list[str]:
    fields = {
        "schema_version",
        "run_id",
        "status",
        "violation_count",
        "violations",
        "expected_entities",
        "observed_entities",
        "manifest_sha256",
        "observations_sha256",
        "required_gates",
        "evidence_sha256",
    }
    violations = exact_fields(payload, fields, "G2")
    violations.extend(validate_common_envelope(payload, label="G2"))
    expected = payload.get("expected_entities")
    observed = payload.get("observed_entities")
    if (
        not is_int(expected, minimum=1)
        or not is_int(observed, minimum=1)
        or expected != observed
    ):
        violations.append("G2 expected/observed entity counts are invalid or differ")
    if payload.get("required_gates") != sorted(MANIFEST_DATABASE_GATES):
        violations.append("G2 required_gates is not the fixed database gate set")
    for field in ("manifest_sha256", "observations_sha256"):
        try:
            require_hash(payload.get(field), f"G2.{field}")
        except ValueError as exc:
            violations.append(str(exc))
    violations.extend(
        validate_self_hash(
            payload,
            field="evidence_sha256",
            label="G2",
            newline=True,
        )
    )
    return violations


def validate_environment(payload: dict[str, Any]) -> list[str]:
    violations: list[str] = []
    candidate = payload.get("candidate")
    if not isinstance(candidate, dict):
        return ["environment candidate metadata is missing"]
    if not re.fullmatch(r"[0-9a-f]{40}", str(candidate.get("git_head") or "")):
        violations.append("candidate git_head is not an exact commit")
    if candidate.get("worktree_diff_sha256") != EMPTY_SHA256:
        violations.append("candidate worktree is not clean")
    required_hashes = (
        "openapi_sha256",
        "external_backend_image_digest",
        "dev_plus_backend_image_digest",
        "external_frontend_manifest_sha256",
        "dev_plus_frontend_manifest_sha256",
        "configuration_sha256",
        "migration_mapping_sha256",
        "snapshot_sha256",
        "review_manifest_sha256",
        "api_oracle_sha256",
        "api_rules_sha256",
        "comparator_sha256",
        "build_api_oracle_sha256",
    )
    for field in required_hashes:
        value = str(payload.get(field) or "")
        if field.endswith("_image_digest"):
            if not re.fullmatch(r"sha256:[0-9a-f]{64}", value):
                violations.append(f"{field} is missing or mutable")
        elif not SHA256.fullmatch(value):
            violations.append(f"{field} is missing")
    return violations


def validate_g3(payload: dict[str, Any]) -> list[str]:
    violations: list[str] = []
    if payload.get("decision") not in {"APPROVED", "CONFIRMED", "confirmed"}:
        violations.append("review decision is not approved")
    summary = payload.get("summary")
    if not isinstance(summary, dict):
        violations.append("review summary is missing")
        return violations
    for field in ("proposed_review_count", "hard_blocked_count"):
        if summary.get(field) != 0:
            violations.append(f"{field} is not zero")
    require_hash(payload.get("candidate_sha256"), "candidate_sha256")
    require_hash(payload.get("cohort_digest"), "cohort_digest")
    return violations


def validate_g4_component_chain(
    payload: dict[str, Any],
    run_dir: pathlib.Path,
    inventory_paths: dict[str, pathlib.Path],
) -> tuple[list[str], dict[str, str]]:
    violations: list[str] = []
    required_hashes: dict[str, str] = {}
    chain = payload.get("component_chain")
    if not isinstance(chain, dict) or set(chain) != {"recovery", "bundle"}:
        return ["G4 component chain is missing"], required_hashes
    recovery_ownership_artifacts = {
        *{
            f"recovery-ownership-{asset_id}.json"
            for asset_id in (23989, 23990, 23991)
        },
        *{
            f"recovery-staging-ownership-{asset_id}.json"
            for asset_id in (23989, 23990, 23991)
        },
    }
    bundle_ownership_artifacts = {
        *{
            f"bundle-ownership-{asset_id}.json"
            for asset_id in range(25557, 25564)
        },
        *{
            f"bundle-staging-ownership-{asset_id}.json"
            for asset_id in range(25557, 25564)
        },
    }
    expected_artifacts = {
        ("recovery", "apply"): {
            "recovery-file-write-ahead.json",
            "recovery-materialization-plan.json",
            "recovery-guard-before.json",
            "recovery-guard-provision.json",
            "recovery-db-apply.json",
            "recovery-db-idempotent.json",
        }
        | recovery_ownership_artifacts,
        ("recovery", "rollback"): {
            "recovery-db-rollback.json",
            "recovery-guard-restore.json",
            "recovery-file-rollback.json",
        }
        | recovery_ownership_artifacts,
        ("bundle", "apply"): {
            "bundle-staging-write-ahead.json",
            "bundle-file-write-ahead.json",
            "bundle-materialize-report.json",
            "bundle-registry.json",
            "bundle-guard-before.json",
            "bundle-guard-provision.json",
            "bundle-db-rollback-journal.json",
            "bundle-db-apply.json",
            "bundle-db-idempotent.json",
        }
        | bundle_ownership_artifacts,
        ("bundle", "rollback"): {
            "bundle-db-rollback-journal.json",
            "bundle-db-rollback.json",
            "bundle-guard-restore.json",
            "bundle-file-rollback.json",
        }
        | bundle_ownership_artifacts,
    }
    run_id = str(payload.get("run_id") or "")
    database: str | None = None
    host: str | None = None
    loaded: dict[tuple[str, str], dict[str, dict[str, Any]]] = {}
    artifact_hashes: dict[tuple[str, str], dict[str, str]] = {}
    for component in ("recovery", "bundle"):
        value = chain.get(component)
        if not isinstance(value, dict) or set(value) != {"apply", "rollback"}:
            violations.append(f"G4 {component} component actions are missing")
            continue
        for action in ("apply", "rollback"):
            label = f"{component}-{action}"
            report = value.get(action)
            try:
                if not isinstance(report, dict):
                    raise ValueError("component report is not an object")
                report_sha = require_hash(
                    report.get("artifact_sha256"),
                    f"G4 {label} report artifact_sha256",
                )
                report_name = f"{component}-component-{action}.json"
                report_path = inventory_paths.get(report_name)
                if (
                    report_path is None
                    or sha256_file(report_path) != report_sha
                ):
                    raise ValueError("component report bytes are absent/drifted")
                encoded_report = json.loads(
                    report_path.read_text(encoding="utf-8")
                )
                encoded_report["artifact_sha256"] = report_sha
                if encoded_report != report:
                    raise ValueError("embedded component report differs from bytes")
                self_hash = str(report.get("evidence_sha256") or "")
                unhashed = dict(report)
                unhashed.pop("artifact_sha256", None)
                unhashed.pop("evidence_sha256", None)
                if (
                    not SHA256.fullmatch(self_hash)
                    or hashlib.sha256(canonical_bytes(unhashed)).hexdigest()
                    != self_hash
                ):
                    raise ValueError("component report self hash is stale")
                expected_status = (
                    "APPLIED" if action == "apply" else "ROLLED_BACK"
                )
                if (
                    report.get("schema_version") != 1
                    or report.get("status") != expected_status
                    or report.get("component") != component
                    or report.get("action") != action
                    or report.get("run_id") != run_id
                    or report.get("database_writes_executed") is not True
                    or report.get("production_writes_executed") is not False
                    or report.get("guard_retained_for_rollback")
                    is not (action == "apply")
                    or report.get("guard_exactly_restored")
                    is not (action == "rollback")
                    or report.get("ownership_receipt_contract_version")
                    != 1
                ):
                    raise ValueError("component report envelope is invalid")
                if database is None:
                    database = str(report.get("database") or "")
                    host = str(report.get("host") or "")
                if (
                    report.get("database") != database
                    or report.get("host") != host
                    or not database
                    or host not in {"127.0.0.1", "localhost"}
                ):
                    raise ValueError("component database/host binding differs")
                artifacts = report.get("artifacts")
                if not isinstance(artifacts, list):
                    raise ValueError("component artifacts are missing")
                names: set[str] = set()
                values: dict[str, dict[str, Any]] = {}
                hashes: dict[str, str] = {}
                for item in artifacts:
                    if (
                        not isinstance(item, dict)
                        or set(item) != {"path", "sha256", "size"}
                    ):
                        raise ValueError("component artifact shape is invalid")
                    name = str(item["path"])
                    digest = require_hash(
                        item["sha256"], f"G4 {label} {name}"
                    )
                    path = inventory_paths.get(name)
                    if (
                        pathlib.PurePosixPath(name).name != name
                        or name in names
                        or path is None
                        or path.is_symlink()
                        or not path.is_file()
                        or isinstance(item["size"], bool)
                        or not isinstance(item["size"], int)
                        or item["size"] < 0
                        or path.stat().st_size != item["size"]
                        or sha256_file(path) != digest
                    ):
                        raise ValueError(f"component artifact {name} drifted")
                    names.add(name)
                    hashes[name] = digest
                    document = json.loads(path.read_text(encoding="utf-8"))
                    if not isinstance(document, dict):
                        raise ValueError(f"component artifact {name} is not an object")
                    values[name] = document
                    required_hashes[f"{label}:{name}"] = digest
                if names != expected_artifacts[(component, action)]:
                    raise ValueError("component artifact set differs")
                loaded[(component, action)] = values
                artifact_hashes[(component, action)] = hashes
                required_hashes[f"{label}:report"] = report_sha
            except (
                OSError,
                UnicodeError,
                json.JSONDecodeError,
                ValueError,
            ) as exc:
                violations.append(f"G4 {label} component is invalid: {exc}")

    for component, expected_changed in (("recovery", 3), ("bundle", 7)):
        try:
            apply = loaded[(component, "apply")]
            rollback = loaded[(component, "rollback")]
            changed_field = (
                "changed_entries"
                if component == "recovery"
                else "changed_bundle_count"
            )
            already_field = (
                "already_in_target_state_entries"
                if component == "recovery"
                else "already_applied_bundle_count"
            )
            db_apply = apply[f"{component}-db-apply.json"]
            db_idempotent = apply[f"{component}-db-idempotent.json"]
            db_rollback = rollback[f"{component}-db-rollback.json"]
            for mode, document, changed, already in (
                ("apply", db_apply, expected_changed, 0),
                ("apply", db_idempotent, 0, expected_changed),
                ("rollback", db_rollback, expected_changed, 0),
            ):
                if (
                    document.get("mode") != mode
                    or document.get("run_id") != run_id
                    or document.get("database") != database
                    or document.get("host") != host
                    or document.get(changed_field) != changed
                    or document.get(already_field) != already
                    or document.get("database_transaction_committed")
                    is not True
                ):
                    raise ValueError(
                        f"{component} {mode} database report is invalid"
                    )
            if component == "recovery":
                plan_hash = artifact_hashes[(component, "apply")][
                    "recovery-materialization-plan.json"
                ]
                if (
                    db_apply.get("version") != 1
                    or db_apply.get("object_storage_writes_executed")
                    is not False
                    or db_idempotent.get("plan_sha256") != plan_hash
                    or db_apply.get("plan_sha256") != plan_hash
                    or db_rollback.get("plan_sha256") != plan_hash
                ):
                    raise ValueError("recovery plan/report hash binding differs")
            else:
                journal = apply["bundle-db-rollback-journal.json"]
                rollback_journal = rollback[
                    "bundle-db-rollback-journal.json"
                ]
                journal_evidence = str(
                    journal.get("evidence_sha256") or ""
                )
                unhashed_journal = dict(journal)
                unhashed_journal.pop("evidence_sha256", None)
                compact_journal_hash = hashlib.sha256(
                    json.dumps(
                        unhashed_journal,
                        ensure_ascii=False,
                        sort_keys=True,
                        separators=(",", ":"),
                    ).encode("utf-8")
                ).hexdigest()
                auto_before = journal.get("auto_increment_before")
                auto_ceilings = journal.get("auto_increment_ceilings")
                auto_tables = ["design_assets", "task_assets"]
                if (
                    journal.get("kind")
                    != "source-bundle-clone-b-rollback-journal"
                    or journal.get("status") != "PREPARED"
                    or journal.get("run_id") != run_id
                    or journal.get("database") != database
                    or journal.get("host") != host
                    or journal.get("expected_bundle_count") != 7
                    or journal.get("expected_member_count") != 22
                    or journal.get(
                        "prepared_before_first_database_mutation"
                    )
                    is not True
                    or journal.get("database_commit_state") != "unknown"
                    or journal.get("production_writes_executed")
                    is not False
                    or not isinstance(auto_before, list)
                    or not isinstance(auto_ceilings, list)
                    or [
                        item.get("table") for item in auto_before
                        if isinstance(item, dict)
                    ]
                    != auto_tables
                    or [
                        item.get("table") for item in auto_ceilings
                        if isinstance(item, dict)
                    ]
                    != auto_tables
                    or any(
                        isinstance(before_state.get("next_value"), bool)
                        or not isinstance(
                            before_state.get("next_value"), int
                        )
                        or before_state["next_value"] <= 0
                        or isinstance(ceiling.get("next_value"), bool)
                        or not isinstance(ceiling.get("next_value"), int)
                        or ceiling["next_value"]
                        < before_state["next_value"]
                        for before_state, ceiling in zip(
                            auto_before, auto_ceilings
                        )
                    )
                    or not SHA256.fullmatch(journal_evidence)
                    or compact_journal_hash != journal_evidence
                    or rollback_journal != journal
                    or artifact_hashes[(component, "apply")][
                        "bundle-db-rollback-journal.json"
                    ]
                    != artifact_hashes[(component, "rollback")][
                        "bundle-db-rollback-journal.json"
                    ]
                ):
                    raise ValueError(
                        "bundle rollback journal is invalid or drifted"
                    )
                for document in (db_apply, db_idempotent, db_rollback):
                    if (
                        document.get("schema_version") != 1
                        or document.get("status") != "PASS"
                        or document.get("rollback_journal_sha256")
                        != artifact_hashes[(component, "apply")][
                            "bundle-db-rollback-journal.json"
                        ]
                        or document.get(
                            "rollback_journal_evidence_sha256"
                        )
                        != journal_evidence
                    ):
                        raise ValueError("bundle DB report envelope is invalid")
                for field in (
                    "candidate_sha256",
                    "registry_sha256",
                    "manifest_sha256",
                ):
                    if (
                        not SHA256.fullmatch(str(db_apply.get(field) or ""))
                        or db_idempotent.get(field) != db_apply.get(field)
                        or db_rollback.get(field) != db_apply.get(field)
                    ):
                        raise ValueError(f"bundle {field} binding differs")
                if (
                    db_apply.get("registry_sha256")
                    != artifact_hashes[(component, "apply")][
                        "bundle-registry.json"
                    ]
                ):
                    raise ValueError("bundle registry hash binding differs")
            before = apply[f"{component}-guard-before.json"]
            provision = apply[f"{component}-guard-provision.json"]
            restore = rollback[f"{component}-guard-restore.json"]
            before_hash = artifact_hashes[(component, "apply")][
                f"{component}-guard-before.json"
            ]
            for label, document in (
                ("before", before),
                ("provision", provision),
                ("restore", restore),
            ):
                evidence_sha = str(document.get("evidence_sha256") or "")
                unhashed = dict(document)
                unhashed.pop("evidence_sha256", None)
                if (
                    not SHA256.fullmatch(evidence_sha)
                    or hashlib.sha256(canonical_bytes(unhashed)).hexdigest()
                    != evidence_sha
                ):
                    raise ValueError(f"{component} guard {label} hash is stale")
            if (
                before.get("kind") != "clone-b-guard-state"
                or before.get("component") != component
                or before.get("database") != database
                or provision.get("status") != "PROVISIONED"
                or provision.get("before_artifact_sha256") != before_hash
                or restore.get("status") != "RESTORED"
                or restore.get("before_artifact_sha256") != before_hash
                or restore.get("exact") is not True
            ):
                raise ValueError(f"{component} guard lifecycle is invalid")
            binding = provision.get("binding")
            if not isinstance(binding, dict):
                raise ValueError(f"{component} guard binding is missing")
            if component == "recovery":
                if binding.get("plan_sha256") != db_apply.get("plan_sha256"):
                    raise ValueError("recovery guard plan binding differs")
            elif (
                binding.get("candidate_sha256")
                != db_apply.get("candidate_sha256")
                or binding.get("registry_sha256")
                != db_apply.get("registry_sha256")
            ):
                raise ValueError("bundle guard hash binding differs")
            file_report = rollback[f"{component}-file-rollback.json"]
            if (
                file_report.get("status") != "ROLLED_BACK"
                or file_report.get("database_write_performed") is not False
            ):
                raise ValueError(f"{component} file rollback is invalid")
        except (KeyError, TypeError, ValueError) as exc:
            violations.append(f"G4 {component} component linkage is invalid: {exc}")
    return violations, required_hashes


def validate_g4(
    payload: dict[str, Any], run_dir: pathlib.Path | None = None
) -> list[str]:
    violations: list[str] = []
    required_steps = (
        ("capture_baseline_fingerprint", "validate"),
        ("recovery_apply", "apply"),
        ("bundle_apply", "apply"),
        ("dry_run_before", "validate"),
        ("workflow_apply", "apply"),
        ("idempotent_apply", "apply"),
        ("validate_after_apply", "validate"),
        ("search_snapshot", "apply"),
        ("search_reindex", "apply"),
        ("search_rollback", "rollback"),
        ("workflow_rollback", "rollback"),
        ("bundle_rollback", "rollback"),
        ("recovery_rollback", "rollback"),
        ("validate_after_rollback_fingerprint", "rollback"),
    )
    exit_code = payload.get("exit_code")
    elapsed = payload.get("elapsed_seconds")
    if isinstance(exit_code, bool) or not isinstance(exit_code, int) or exit_code != 0:
        violations.append("G4 exit_code is not numeric zero")
    if (
        isinstance(elapsed, bool)
        or not isinstance(elapsed, (int, float))
        or elapsed < 0
    ):
        violations.append("G4 elapsed_seconds is invalid")
    steps = payload.get("steps")
    if not isinstance(steps, list):
        return ["G4 steps are missing"]
    actual: list[tuple[str, str]] = []
    for index, row in enumerate(steps):
        if not isinstance(row, dict):
            violations.append(f"G4 step[{index}] is not an object")
            continue
        actual.append(
            (str(row.get("step") or ""), str(row.get("phase") or ""))
        )
        row_exit = row.get("exit_code")
        row_elapsed = row.get("elapsed_seconds")
        if (
            row.get("status") != "PASS"
            or isinstance(row_exit, bool)
            or not isinstance(row_exit, int)
            or row_exit != 0
        ):
            violations.append(f"G4 step[{index}] did not pass with numeric zero")
        if (
            isinstance(row_elapsed, bool)
            or not isinstance(row_elapsed, (int, float))
            or row_elapsed < 0
        ):
            violations.append(f"G4 step[{index}] elapsed_seconds is invalid")
        if not SHA256.fullmatch(str(row.get("command_sha256") or "")):
            violations.append(f"G4 step[{index}] command hash is missing")
    if actual != list(required_steps):
        violations.append(f"G4 ordered step sequence differs: {actual}")
    baseline_fingerprint = payload.get("baseline_fingerprint")
    baseline_artifact_sha = str(
        payload.get("baseline_fingerprint_artifact_sha256") or ""
    )
    baseline_internal_sha = ""
    if not isinstance(baseline_fingerprint, dict):
        violations.append("G4 baseline fingerprint is missing")
    else:
        baseline_tables = baseline_fingerprint.get("tables")
        baseline_internal_sha = str(
            baseline_fingerprint.get("fingerprint_sha256") or ""
        )
        valid_baseline_tables = all(
            isinstance(name, str)
            and bool(name)
            and isinstance(value, dict)
            and set(value)
            == {
                "row_count",
                "content_sha256",
                "schema_sha256",
                "content_fingerprint_algorithm",
                "auto_increment",
            }
            and not isinstance(value["row_count"], bool)
            and isinstance(value["row_count"], int)
            and value["row_count"] >= 0
            and SHA256.fullmatch(str(value["content_sha256"] or ""))
            and SHA256.fullmatch(str(value["schema_sha256"] or ""))
            and value["content_fingerprint_algorithm"]
            == G4_ROW_FINGERPRINT_ALGORITHM
            and (
                value["auto_increment"] is None
                or (
                    not isinstance(value["auto_increment"], bool)
                    and isinstance(value["auto_increment"], int)
                    and value["auto_increment"] > 0
                )
            )
            for name, value in (
                baseline_tables.items()
                if isinstance(baseline_tables, dict)
                else ()
            )
        )
        if (
            baseline_fingerprint.get("schema_version") != 1
            or baseline_fingerprint.get("kind")
            != "clone-b-baseline-fingerprint"
            or baseline_fingerprint.get("fingerprint_algorithm")
            != G4_ROW_FINGERPRINT_ALGORITHM
            or not isinstance(baseline_tables, dict)
            or not baseline_tables
            or not valid_baseline_tables
            or baseline_internal_sha
            != hashlib.sha256(canonical_bytes(baseline_tables)).hexdigest()
            or not SHA256.fullmatch(baseline_artifact_sha)
            or baseline_artifact_sha
            != hashlib.sha256(
                canonical_bytes(baseline_fingerprint)
            ).hexdigest()
        ):
            violations.append("G4 baseline fingerprint envelope is invalid")
    fingerprint = payload.get("rollback_fingerprint")
    if not isinstance(fingerprint, dict):
        violations.append("G4 rollback fingerprint is missing")
    else:
        baseline = str(fingerprint.get("baseline_fingerprint_sha256") or "")
        rollback = str(fingerprint.get("rollback_fingerprint_sha256") or "")
        if (
            fingerprint.get("schema_version") != 1
            or fingerprint.get("status") != "PASS"
            or fingerprint.get("violation_count") != 0
            or not SHA256.fullmatch(baseline)
            or rollback != baseline
            or baseline != baseline_internal_sha
            or fingerprint.get("baseline_artifact_sha256")
            != baseline_artifact_sha
        ):
            violations.append("G4 rollback fingerprint is not equal/PASS")
    if not SHA256.fullmatch(
        str(payload.get("rollback_fingerprint_sha256") or "")
    ):
        violations.append("G4 rollback fingerprint evidence hash is missing")
    search_restore = payload.get("search_restore")
    if not isinstance(search_restore, dict) or set(search_restore) != {
        "snapshot",
        "rollback",
    }:
        violations.append("G4 search restore evidence is missing")
    else:
        snapshot = search_restore["snapshot"]
        rollback = search_restore["rollback"]
        tables = snapshot.get("tables") if isinstance(snapshot, dict) else None
        table_names = {
            "task_search_documents",
            "task_asset_group_search_documents",
            "product_search_documents",
        }
        valid_tables = isinstance(tables, dict) and set(tables) == table_names
        if valid_tables:
            for value in tables.values():
                if (
                    not isinstance(value, dict)
                    or set(value) != {"row_count", "content_sha256"}
                    or isinstance(value["row_count"], bool)
                    or not isinstance(value["row_count"], int)
                    or value["row_count"] < 0
                    or not SHA256.fullmatch(str(value["content_sha256"] or ""))
                ):
                    valid_tables = False
                    break
        archive = snapshot.get("archive") if isinstance(snapshot, dict) else None
        valid_archive = (
            isinstance(archive, dict)
            and set(archive) == {"format", "sha256", "size"}
            and archive.get("format") == "deterministic-jsonl-v1"
            and SHA256.fullmatch(str(archive.get("sha256") or "")) is not None
            and not isinstance(archive.get("size"), bool)
            and isinstance(archive.get("size"), int)
            and archive["size"] >= 0
        )
        snapshot_sha = (
            hashlib.sha256(canonical_bytes(tables)).hexdigest()
            if valid_tables
            else ""
        )
        if (
            not isinstance(snapshot, dict)
            or snapshot.get("schema_version") != 1
            or snapshot.get("status") != "CAPTURED"
            or snapshot.get("violation_count") != 0
            or snapshot.get("snapshot_sha256") != snapshot_sha
            or not isinstance(rollback, dict)
            or rollback.get("schema_version") != 1
            or rollback.get("status") != "PASS"
            or rollback.get("violation_count") != 0
            or rollback.get("snapshot_sha256") != snapshot_sha
            or rollback.get("restored_snapshot_sha256") != snapshot_sha
            or rollback.get("restored_tables") != tables
            or not valid_archive
            or (
                valid_archive
                and payload.get("search_snapshot_archive_sha256")
                != archive["sha256"]
            )
            or rollback.get("source_archive_sha256") != archive["sha256"]
        ):
            violations.append("G4 search rollback is not an exact snapshot restore")
    for field in (
        "search_snapshot_sha256",
        "search_snapshot_archive_sha256",
        "search_rollback_sha256",
    ):
        if not SHA256.fullmatch(str(payload.get(field) or "")):
            violations.append(f"G4 {field} is missing")
    inputs = payload.get("input_sha256")
    if not isinstance(inputs, dict) or set(inputs) != {
        "command_plan",
        "mapping",
        "auth_settings",
        "frontend_access_settings",
    }:
        violations.append("G4 input hash binding is incomplete")
    else:
        for name, value in inputs.items():
            if not SHA256.fullmatch(str(value or "")):
                violations.append(f"G4 {name} input hash is invalid")
    auth_attestation = payload.get("auth_settings_attestation")
    expected_auth_attestation_fields = {
        "frozen_input_path",
        "byte_count",
        "sha256",
        "read_only",
        "super_admin_count",
        "department_admin_key_count",
        "configured_user_assignment_count",
    }
    if (
        not isinstance(auth_attestation, dict)
        or set(auth_attestation) != expected_auth_attestation_fields
    ):
        violations.append("G4 Clone B auth settings attestation is incomplete")
    elif (
        auth_attestation.get("sha256")
        != (inputs or {}).get("auth_settings")
        or auth_attestation.get("read_only") is not True
        or type(auth_attestation.get("byte_count")) is not int
        or auth_attestation.get("byte_count", 0) <= 0
        or type(auth_attestation.get("super_admin_count")) is not int
        or auth_attestation.get("super_admin_count") != 0
        or type(auth_attestation.get("department_admin_key_count")) is not int
        or auth_attestation.get("department_admin_key_count") != 0
        or type(auth_attestation.get("configured_user_assignment_count"))
        is not int
        or auth_attestation.get("configured_user_assignment_count") != 0
    ):
        violations.append("G4 Clone B auth settings attestation is invalid")
    inventory = payload.get("evidence_inventory")
    if not isinstance(inventory, list) or not inventory:
        violations.append("G4 evidence inventory is missing")
    elif run_dir is None:
        violations.append("G4 evidence inventory has no finalizer run root")
    else:
        seen_paths: set[str] = set()
        inventory_hashes: set[str] = set()
        inventory_hashes_by_path: dict[str, str] = {}
        inventory_paths: dict[str, pathlib.Path] = {}
        for index, record in enumerate(inventory):
            try:
                if not isinstance(record, dict) or set(record) != {
                    "path",
                    "sha256",
                }:
                    raise ValueError("record shape is invalid")
                relative = str(record["path"])
                if relative in seen_paths:
                    raise ValueError("record path is duplicated")
                seen_paths.add(relative)
                expected = require_hash(
                    record["sha256"], f"G4.evidence_inventory[{index}].sha256"
                )
                actual = sha256_file(safe_artifact_path(run_dir, relative))
                if actual != expected:
                    raise ValueError(
                        f"hash mismatch expected={expected} actual={actual}"
                    )
                inventory_hashes.add(actual)
                inventory_hashes_by_path[relative] = actual
                basename = pathlib.PurePosixPath(relative).name
                if basename in inventory_paths:
                    raise ValueError(
                        f"record basename is duplicated: {basename}"
                    )
                inventory_paths[basename] = safe_artifact_path(
                    run_dir, relative
                )
            except (OSError, ValueError) as exc:
                violations.append(f"G4 evidence[{index}] is invalid: {exc}")
        if isinstance(auth_attestation, dict):
            try:
                auth_relative = str(auth_attestation["frozen_input_path"])
                pure_auth_relative = pathlib.PurePosixPath(auth_relative)
                if (
                    pure_auth_relative.is_absolute()
                    or ".." in pure_auth_relative.parts
                    or pure_auth_relative.parts[-2:]
                    != ("inputs", "auth_identity.clone-b.json")
                ):
                    raise ValueError("frozen auth path is not run-scoped")
                frozen_auth_path = safe_artifact_path(run_dir, auth_relative)
                if frozen_auth_path.is_symlink() or not frozen_auth_path.is_file():
                    raise ValueError("frozen auth file is not a regular file")
                if frozen_auth_path.stat().st_mode & 0o222:
                    raise ValueError("frozen auth file is writable")
                raw_auth = frozen_auth_path.read_bytes()
                clone_b_auth_policy.validate(raw_auth)
                actual_auth_hash = hashlib.sha256(raw_auth).hexdigest()
                if (
                    len(raw_auth) != auth_attestation["byte_count"]
                    or actual_auth_hash != auth_attestation["sha256"]
                    or inventory_hashes_by_path.get(auth_relative)
                    != actual_auth_hash
                ):
                    raise ValueError(
                        "frozen auth bytes, attestation, and inventory differ"
                    )
                frontend_relative = (
                    pure_auth_relative.parent / "frontend_access.json"
                ).as_posix()
                if (
                    inventory_hashes_by_path.get(frontend_relative)
                    != (inputs or {}).get("frontend_access_settings")
                ):
                    raise ValueError(
                        "frozen frontend access config is not exactly inventory-bound"
                    )
            except (KeyError, OSError, TypeError, ValueError) as exc:
                violations.append(
                    f"G4 Clone B auth settings evidence is invalid: {exc}"
                )
        required_hashes: dict[str, Any] = {
            "baseline_fingerprint": payload.get(
                "baseline_fingerprint_artifact_sha256"
            ),
            "rollback_fingerprint": payload.get(
                "rollback_fingerprint_sha256"
            ),
            "search_snapshot": payload.get("search_snapshot_sha256"),
            "search_snapshot_archive": payload.get(
                "search_snapshot_archive_sha256"
            ),
            "search_rollback": payload.get("search_rollback_sha256"),
            "auth_settings": (inputs or {}).get("auth_settings"),
        }
        if isinstance(inputs, dict):
            required_hashes.update(inputs)
        component_violations, component_hashes = validate_g4_component_chain(
            payload, run_dir, inventory_paths
        )
        violations.extend(component_violations)
        required_hashes.update(component_hashes)
        for name, value in required_hashes.items():
            if SHA256.fullmatch(str(value or "")):
                if value not in inventory_hashes:
                    violations.append(
                        f"G4 {name} bytes are absent from evidence inventory"
                    )
    violations.extend(validate_g10(payload))
    return violations


def validate_g5(payload: dict[str, Any]) -> list[str]:
    fields = {
        "schema_version",
        "run_id",
        "status",
        "violation_count",
        "violations",
        "gates",
        "immutable_event_parity",
    }
    violations = exact_fields(payload, fields, "G5")
    violations.extend(validate_common_envelope(payload, label="G5"))
    gates = payload.get("gates")
    if not isinstance(gates, list) or len(gates) != len(SQL_GATE_NAMES):
        violations.append("G5 must contain exactly the 00-12 SQL gates")
    else:
        actual_names: list[str] = []
        for index, row in enumerate(gates):
            expected_fields = {
                "gate",
                "a_assessment",
                "b_assessment",
                "a_violation_count",
                "b_violation_count",
                "a_json_sha256",
                "b_json_sha256",
            }
            if not isinstance(row, dict) or set(row) != expected_fields:
                violations.append(f"G5 gate[{index}] field contract differs")
                continue
            actual_names.append(str(row["gate"]))
            if (
                row["a_assessment"] != "baseline_or_immutable_parity"
                or row["b_assessment"] != "approved_manifest_and_v8_invariants"
            ):
                violations.append(f"G5 gate[{index}] assessment contract differs")
            for side in ("a", "b"):
                if row[f"{side}_violation_count"] != 0:
                    violations.append(
                        f"G5 gate[{index}] {side.upper()} violation count is not zero"
                    )
                try:
                    require_hash(
                        row[f"{side}_json_sha256"],
                        f"G5.gates[{index}].{side}_json_sha256",
                    )
                except ValueError as exc:
                    violations.append(str(exc))
        if actual_names != list(SQL_GATE_NAMES):
            violations.append(
                f"G5 SQL gate order/set differs: {actual_names}"
            )
    parity = payload.get("immutable_event_parity")
    parity_fields = {
        "schema_version",
        "gate",
        "status",
        "violation_count",
        "violations",
        "source_evidence_sha256",
        "target_evidence_sha256",
    }
    if not isinstance(parity, dict) or set(parity) != parity_fields:
        violations.append("G5 immutable event parity field contract differs")
    else:
        violations.extend(
            validate_common_envelope(
                parity,
                label="G5 immutable event parity",
                run_id_required=False,
            )
        )
        if parity.get("gate") != "07_event_history_checksum":
            violations.append("G5 immutable event parity gate is not 07")
        source_hash = str(parity.get("source_evidence_sha256") or "")
        target_hash = str(parity.get("target_evidence_sha256") or "")
        if (
            not SHA256.fullmatch(source_hash)
            or not SHA256.fullmatch(target_hash)
            or source_hash != target_hash
        ):
            violations.append("G5 immutable event parity hashes are invalid or differ")
    return violations


def validate_g6(payload: dict[str, Any]) -> list[str]:
    fields = {
        "schema_version",
        "run_id",
        "status",
        "task_count",
        "group_count",
        "task_asset_count",
        "legacy_task_asset_count",
        "manifest_oracle_check_count",
        "semantic_comparison_count",
        "request_count",
        "combination_matrix",
        "identities",
        "task_ids_sha256",
        "matrix_sha256",
        "rules_sha256",
        "manifest_sha256",
        "api_oracle_sha256",
        "api_oracle_mapping_sha256",
        "download_allowed_hosts",
        "download_allowed_hosts_sha256",
        "comparator_sha256",
        "build_api_oracle_sha256",
        "used_rule_ids",
        "used_rule_applications",
        "unused_rule_ids",
        "observations",
        "violation_count",
        "violations",
        "evidence_sha256",
    }
    violations = exact_fields(payload, fields, "G6")
    violations.extend(validate_common_envelope(payload, label="G6"))
    for field in ("task_count", "request_count"):
        if not is_int(payload.get(field), minimum=1):
            violations.append(f"G6 {field} must be a positive integer")
    for field in ("manifest_oracle_check_count", "semantic_comparison_count"):
        if not is_int(payload.get(field), minimum=1):
            violations.append(f"G6 {field} must be a positive integer")
    for field in ("group_count", "task_asset_count", "legacy_task_asset_count"):
        if not is_int(payload.get(field)):
            violations.append(f"G6 {field} must be a nonnegative integer")
    matrix = payload.get("combination_matrix")
    actual_matrix: dict[str, tuple[Any, Any, Any]] = {}
    if not isinstance(matrix, list) or len(matrix) != 4:
        violations.append("G6 combination matrix does not contain four rows")
    else:
        for index, row in enumerate(matrix):
            if not isinstance(row, dict) or set(row) != {
                "id",
                "frontend",
                "backend",
                "data",
                "origin_sha256",
            }:
                violations.append(f"G6 combination[{index}] field contract differs")
                continue
            combo_id = str(row["id"])
            if combo_id in actual_matrix:
                violations.append(f"G6 combination id is duplicated: {combo_id}")
            actual_matrix[combo_id] = (
                row["frontend"],
                row["backend"],
                row["data"],
            )
            if not SHA256.fullmatch(str(row.get("origin_sha256") or "")):
                violations.append(
                    f"G6 combination[{index}] origin_sha256 is invalid"
                )
        if actual_matrix != API_COMBINATIONS:
            violations.append("G6 combination matrix differs from the fixed four edges")
        origin_by_id = {
            str(row["id"]): str(row["origin_sha256"])
            for row in matrix
            if isinstance(row, dict)
            and set(row) == {
                "id",
                "frontend",
                "backend",
                "data",
                "origin_sha256",
            }
        }
        a_origins = {
            origin_by_id.get("external_external_a"),
            origin_by_id.get("dev_external_a"),
        }
        b_origins = {
            origin_by_id.get("dev_dev_b"),
            origin_by_id.get("external_dev_b"),
        }
        if None in a_origins | b_origins or a_origins & b_origins:
            violations.append("G6 A/B physical origins are missing or overlap")
    identities = payload.get("identities")
    identity_ids: set[str] = set()
    if not isinstance(identities, list) or not identities:
        violations.append("G6 identities must be a non-empty array")
    else:
        for index, row in enumerate(identities):
            if (
                not isinstance(row, dict)
                or set(row) != {"id", "role"}
                or not str(row.get("id") or "").strip()
                or not str(row.get("role") or "").strip()
                or row["id"] in identity_ids
            ):
                violations.append(f"G6 identity[{index}] is invalid or duplicated")
                continue
            identity_ids.add(row["id"])
    observations = payload.get("observations")
    if not isinstance(observations, list):
        violations.append("G6 observations must be an array")
    else:
        if payload.get("request_count") != len(observations):
            violations.append("G6 request_count differs from observations length")
        observation_keys: set[tuple[str, str, str, str]] = set()
        observed_combinations: set[str] = set()
        observed_identities: set[str] = set()
        for index, row in enumerate(observations):
            if not isinstance(row, dict) or set(row) != {
                "combination",
                "identity",
                "route",
                "entity_key",
                "status",
                "body_sha256",
                "raw_sha256",
                "body_bytes",
            }:
                violations.append(f"G6 observation[{index}] field contract differs")
                continue
            key = (
                str(row["combination"]),
                str(row["identity"]),
                str(row["route"]),
                str(row["entity_key"]),
            )
            if (
                key in observation_keys
                or key[0] not in API_COMBINATIONS
                or key[1] not in identity_ids
                or not key[2].startswith("/v1/")
                or not key[3]
                or not is_int(row["status"], minimum=100)
                or row["status"] > 599
                or not is_int(row["body_bytes"])
                or not SHA256.fullmatch(str(row["body_sha256"] or ""))
                or not SHA256.fullmatch(str(row["raw_sha256"] or ""))
            ):
                violations.append(f"G6 observation[{index}] is invalid or duplicated")
            observation_keys.add(key)
            observed_combinations.add(key[0])
            observed_identities.add(key[1])
        if observed_combinations != set(API_COMBINATIONS):
            violations.append("G6 observations do not cover all four combinations")
        if observed_identities != identity_ids:
            violations.append("G6 observations do not cover every declared identity")
    for field in (
        "task_ids_sha256",
        "matrix_sha256",
        "rules_sha256",
        "manifest_sha256",
        "api_oracle_sha256",
        "api_oracle_mapping_sha256",
        "download_allowed_hosts_sha256",
        "comparator_sha256",
        "build_api_oracle_sha256",
    ):
        try:
            require_hash(payload.get(field), f"G6.{field}")
        except ValueError as exc:
            violations.append(str(exc))
    allowed_hosts = payload.get("download_allowed_hosts")
    if (
        not isinstance(allowed_hosts, list)
        or any(not isinstance(host, str) or not host for host in allowed_hosts)
        or allowed_hosts != sorted(set(allowed_hosts))
    ):
        violations.append("G6 download host allowlist is invalid")
    elif SHA256.fullmatch(
        str(payload.get("download_allowed_hosts_sha256") or "")
    ) and hashlib.sha256(canonical_bytes(allowed_hosts)[:-1]).hexdigest() != (
        payload.get("download_allowed_hosts_sha256")
    ):
        violations.append("G6 download host allowlist hash differs")
    used = payload.get("used_rule_ids")
    applications = payload.get("used_rule_applications")
    unused = payload.get("unused_rule_ids")
    if (
        not isinstance(used, list)
        or not isinstance(unused, list)
        or any(not isinstance(item, str) or not item for item in used + unused)
        or len(set(used)) != len(used)
        or len(set(unused)) != len(unused)
        or set(used) & set(unused)
        or used != sorted(used)
        or unused != sorted(unused)
    ):
        violations.append("G6 used/unused normalization rule sets are invalid")
    application_fields = {
        "rule_id",
        "rule_identity",
        "identity",
        "route",
        "direction",
        "from_status",
        "to_status",
    }
    application_keys: list[tuple[Any, ...]] = []
    application_rule_ids: set[str] = set()
    valid_directions = {
        f"{left}->{right}"
        for offset, left in enumerate(API_COMBINATIONS)
        for right in list(API_COMBINATIONS)[offset + 1 :]
    }
    if not isinstance(applications, list):
        violations.append("G6 used_rule_applications must be an array")
    else:
        for index, row in enumerate(applications):
            if not isinstance(row, dict) or set(row) != application_fields:
                violations.append(
                    f"G6 used_rule_applications[{index}] field contract differs"
                )
                continue
            rule_identity = row["rule_identity"]
            identity = row["identity"]
            key = (
                row["rule_id"],
                rule_identity or "",
                identity,
                row["route"],
                row["direction"],
                row["from_status"],
                row["to_status"],
            )
            if (
                not isinstance(row["rule_id"], str)
                or not row["rule_id"]
                or (
                    rule_identity is not None
                    and (
                        not isinstance(rule_identity, str)
                        or rule_identity != identity
                    )
                )
                or identity not in identity_ids
                or not isinstance(row["route"], str)
                or not row["route"].startswith("/v1/")
                or row["direction"] not in valid_directions
                or not is_int(row["from_status"], minimum=100)
                or row["from_status"] > 599
                or not is_int(row["to_status"], minimum=100)
                or row["to_status"] > 599
            ):
                violations.append(
                    f"G6 used_rule_applications[{index}] is invalid"
                )
            application_keys.append(key)
            application_rule_ids.add(row["rule_id"])
        if application_keys != sorted(application_keys) or len(
            application_keys
        ) != len(set(application_keys)):
            violations.append(
                "G6 used_rule_applications are duplicated or not sorted"
            )
        if isinstance(used, list) and application_rule_ids != set(used):
            violations.append(
                "G6 used_rule_applications do not cover used_rule_ids"
            )
    violations.extend(
        validate_self_hash(
            payload,
            field="evidence_sha256",
            label="G6",
            newline=False,
        )
    )
    return violations


def validate_g7(payload: dict[str, Any]) -> list[str]:
    fields = {
        "schema_version",
        "gate",
        "status",
        "run_id",
        "scenario_catalog_sha256",
        "browser_evidence_sha256",
        "playwright_evidence_sha256",
        "required_case_count",
        "passed_case_count",
        "failed_case_count",
        "critical_pass_rate",
        "source_kinds",
        "failures",
        "cases",
        "generated_at",
    }
    violations = exact_fields(payload, fields, "G7")
    if payload.get("schema_version") != 1:
        violations.append("G7 schema_version must be 1")
    if payload.get("gate") != "G7":
        violations.append("G7 gate marker differs")
    if payload.get("status") != "PASS":
        violations.append("G7 status must be PASS")
    if not RUN_ID.fullmatch(str(payload.get("run_id") or "")):
        violations.append("G7 run_id is invalid")
    for field in (
        "scenario_catalog_sha256",
        "browser_evidence_sha256",
        "playwright_evidence_sha256",
    ):
        try:
            require_hash(payload.get(field), f"G7.{field}")
        except ValueError as exc:
            violations.append(str(exc))
    required = payload.get("required_case_count")
    passed = payload.get("passed_case_count")
    failed = payload.get("failed_case_count")
    rate = payload.get("critical_pass_rate")
    cases = payload.get("cases")
    if (
        not is_int(required, minimum=1)
        or not is_int(passed)
        or not is_int(failed)
        or passed != required
        or failed != 0
        or isinstance(rate, bool)
        or not isinstance(rate, (int, float))
        or rate != 1.0
        or not isinstance(cases, list)
        or len(cases) != required
    ):
        violations.append("G7 case counts or critical pass rate are not exact")
    if payload.get("source_kinds") != [
        "browser_computer_use",
        "playwright",
    ]:
        violations.append("G7 source_kinds is not the fixed independent pair")
    failures = payload.get("failures")
    if not isinstance(failures, list) or failures:
        violations.append("G7 failures must be an empty array")
    case_keys: set[tuple[str, str, str]] = set()
    if isinstance(cases, list):
        observed_combinations: set[str] = set()
        observed_viewports: set[str] = set()
        for index, row in enumerate(cases):
            if not isinstance(row, dict) or set(row) != {
                "scenario_id",
                "combination",
                "viewport",
                "status",
                "browser_record_sha256",
                "playwright_record_sha256",
                "pair_sha256",
            }:
                violations.append(f"G7 case[{index}] field contract differs")
                continue
            key = (
                str(row["scenario_id"]),
                str(row["combination"]),
                str(row["viewport"]),
            )
            if (
                not key[0]
                or key[1]
                not in {
                    "external_external",
                    "devplus_devplus",
                    "external_devplus",
                    "devplus_external",
                }
                or key[2] not in {"desktop", "mobile"}
                or key in case_keys
                or row["status"] != "PASS"
            ):
                violations.append(f"G7 case[{index}] identity/status is invalid")
            case_keys.add(key)
            observed_combinations.add(key[1])
            observed_viewports.add(key[2])
            for field in (
                "browser_record_sha256",
                "playwright_record_sha256",
                "pair_sha256",
            ):
                if not SHA256.fullmatch(str(row.get(field) or "")):
                    violations.append(f"G7 case[{index}] {field} is invalid")
            if all(
                SHA256.fullmatch(str(row.get(field) or ""))
                for field in (
                    "browser_record_sha256",
                    "playwright_record_sha256",
                    "pair_sha256",
                )
            ) and SHA256.fullmatch(
                str(payload.get("scenario_catalog_sha256") or "")
            ):
                expected_pair = hashlib.sha256(
                    json.dumps(
                        {
                            "browser_record_sha256": row[
                                "browser_record_sha256"
                            ],
                            "case_key": list(key),
                            "playwright_record_sha256": row[
                                "playwright_record_sha256"
                            ],
                            "scenario_catalog_sha256": payload.get(
                                "scenario_catalog_sha256"
                            ),
                        },
                        ensure_ascii=False,
                        sort_keys=True,
                        separators=(",", ":"),
                    ).encode("utf-8")
                ).hexdigest()
                if row["pair_sha256"] != expected_pair:
                    violations.append(
                        f"G7 case[{index}] pair_sha256 does not bind the pair"
                    )
        if observed_combinations != {
            "external_external",
            "devplus_devplus",
            "external_devplus",
            "devplus_external",
        }:
            violations.append("G7 cases do not cover all four combinations")
        if observed_viewports != {"desktop", "mobile"}:
            violations.append("G7 cases do not cover desktop and mobile viewports")
    generated_at = str(payload.get("generated_at") or "")
    try:
        parsed = dt.datetime.fromisoformat(generated_at.replace("Z", "+00:00"))
        if parsed.tzinfo is None:
            raise ValueError
    except ValueError:
        violations.append("G7 generated_at is not timezone-aware RFC3339")
    return violations


def validate_g8(payload: dict[str, Any]) -> list[str]:
    fields = {
        "schema_version",
        "status",
        "violation_count",
        "checked_count",
        "manifest_sha256",
        "exception_count",
        "exception_evidence_sha256",
        "mapping_sha256",
        "mapping_row_hash",
        "exceptions",
        "violations",
        "evidence_hash",
    }
    violations = exact_fields(payload, fields, "G8")
    violations.extend(
        validate_common_envelope(
            payload,
            label="G8",
            run_id_required=False,
        )
    )
    if not is_int(payload.get("checked_count"), minimum=1):
        violations.append("G8 checked_count must be a positive integer")
    try:
        require_hash(payload.get("manifest_sha256"), "G8.manifest_sha256")
    except ValueError as exc:
        violations.append(str(exc))
    if payload.get("exception_count") != 1:
        violations.append("G8 exception_count must be exactly 1")
    for field in (
        "exception_evidence_sha256",
        "mapping_sha256",
        "mapping_row_hash",
    ):
        try:
            value = require_hash(payload.get(field), f"G8.{field}")
            if value == "0" * 64:
                violations.append(f"G8.{field} must not be the zero hash")
        except ValueError as exc:
            violations.append(str(exc))
    exceptions = payload.get("exceptions")
    exception_fields = {
        "entity_key",
        "task_id",
        "missing_task_asset_id",
        "expected_http_status",
        "observed_http_status",
        "mapping_row_hash",
        "object_row_sha256",
        "working_reference_count",
        "finalized_reference_count",
    }
    if not isinstance(exceptions, list) or len(exceptions) != 1:
        violations.append("G8 exceptions must contain exactly one record")
    else:
        exception = exceptions[0]
        if not isinstance(exception, dict) or set(exception) != exception_fields:
            violations.append("G8 exception field contract differs")
        else:
            if (
                exception.get("entity_key") != "task_asset:12323"
                or exception.get("task_id") != 2199
                or exception.get("missing_task_asset_id") != 12323
            ):
                violations.append("G8 exception entity must be task 2199 asset 12323")
            if (
                exception.get("expected_http_status") != 410
                or exception.get("observed_http_status") != 410
            ):
                violations.append("G8 exception must prove exact HTTP 410")
            if exception.get("mapping_row_hash") != payload.get("mapping_row_hash"):
                violations.append("G8 exception mapping row hash differs")
            try:
                require_hash(
                    exception.get("object_row_sha256"),
                    "G8.exception.object_row_sha256",
                )
            except ValueError as exc:
                violations.append(str(exc))
            if (
                exception.get("working_reference_count") != 0
                or exception.get("finalized_reference_count") != 0
            ):
                violations.append("G8 exception has a current revision reference")
    violations.extend(
        validate_self_hash(
            payload,
            field="evidence_hash",
            label="G8",
            newline=False,
        )
    )
    return violations


def validate_g9(payload: dict[str, Any]) -> list[str]:
    if payload.get("unresolved_p0_count") != 0:
        return ["adversarial review has unresolved P0"]
    if payload.get("unresolved_p1_count") != 0:
        return ["adversarial review has unresolved P1"]
    return []


def validate_g10(payload: dict[str, Any]) -> list[str]:
    violations: list[str] = []
    timings = payload.get("timings_seconds")
    if not isinstance(timings, dict):
        return ["G10 timing evidence is missing"]
    for name in ("apply", "validate", "rollback"):
        value = timings.get(name)
        if (
            isinstance(value, bool)
            or not isinstance(value, (int, float))
            or value < 0
            or value > 600
        ):
            violations.append(f"{name} exceeds 600 seconds or is invalid")
    total = timings.get("total")
    if (
        isinstance(total, bool)
        or not isinstance(total, (int, float))
        or total < 0
        or total > 1800
    ):
        violations.append("total maintenance rehearsal exceeds 1800 seconds")
    return violations


VALIDATORS = {
    "G0": validate_environment,
    "G1": validate_g1,
    "G2": validate_g2,
    "G3": validate_g3,
    "G4": validate_g4,
    "G5": validate_g5,
    "G6": validate_g6,
    "G7": validate_g7,
    "G8": validate_g8,
    "G9": validate_g9,
    "G10": validate_g10,
}


def validate_index(
    run_dir: pathlib.Path, index_path: pathlib.Path
) -> tuple[dict[str, Any], dict[str, Any]]:
    index = read_json(index_path)
    if index.get("schema_version") != 1:
        raise ValueError("evidence index schema_version must be 1")
    run_id = str(index.get("run_id") or "")
    if not RUN_ID.fullmatch(run_id) or run_dir.name != run_id:
        raise ValueError("run_id must match the run directory")
    artifacts = index.get("gates")
    if not isinstance(artifacts, dict) or set(artifacts) != set(GATES):
        raise ValueError("evidence index must contain exactly G0-G10")

    gate_results: dict[str, Any] = {}
    gate_payloads: dict[str, dict[str, Any]] = {}
    for gate in GATES:
        record = artifacts[gate]
        if not isinstance(record, dict) or set(record) != {
            "path",
            "sha256",
            "executor",
            "reviewer",
        }:
            raise ValueError(f"{gate} evidence record has an invalid shape")
        path = safe_artifact_path(run_dir, str(record["path"]))
        expected = require_hash(record["sha256"], f"{gate}.sha256")
        actual = sha256_file(path)
        violations: list[str] = []
        if actual != expected:
            violations.append(
                f"artifact hash mismatch expected={expected} actual={actual}"
            )
            payload: dict[str, Any] = {}
        else:
            payload = read_json(path)
            gate_payloads[gate] = payload
            if payload.get("run_id") not in {None, run_id}:
                violations.append("artifact belongs to another run")
            if not is_pass(payload):
                violations.append("artifact status is not PASS/zero violations")
            validator = VALIDATORS.get(gate)
            if validator is not None:
                try:
                    if gate == "G4":
                        violations.extend(validate_g4(payload, run_dir))
                    else:
                        violations.extend(validator(payload))
                except ValueError as exc:
                    violations.append(str(exc))
        executor = str(record.get("executor") or "").strip()
        reviewer = str(record.get("reviewer") or "").strip()
        if not executor or not reviewer or executor == reviewer:
            violations.append("executor/reviewer independence is not proven")
        gate_results[gate] = {
            "status": "PASS" if not violations else "BLOCKED",
            "violations": violations,
            "evidence": str(record["path"]),
            "evidence_sha256": actual,
            "executor": executor or None,
            "reviewer": reviewer or None,
        }

    manifest_bindings = {
        "G0": gate_payloads.get("G0", {}).get("review_manifest_sha256"),
        "G2": gate_payloads.get("G2", {}).get("manifest_sha256"),
        "G6": gate_payloads.get("G6", {}).get("manifest_sha256"),
        "G8": gate_payloads.get("G8", {}).get("manifest_sha256"),
    }
    if len(set(manifest_bindings.values())) != 1 or not all(
        isinstance(value, str) and SHA256.fullmatch(value)
        for value in manifest_bindings.values()
    ):
        detail = "cross-gate reviewed manifest hash binding differs"
        for gate in manifest_bindings:
            gate_results[gate]["violations"].append(detail)
            gate_results[gate]["status"] = "BLOCKED"
    mapping_bindings = {
        "G0": gate_payloads.get("G0", {}).get("migration_mapping_sha256"),
        "G6": gate_payloads.get("G6", {}).get("api_oracle_mapping_sha256"),
        "G8": gate_payloads.get("G8", {}).get("mapping_sha256"),
    }
    if len(set(mapping_bindings.values())) != 1 or not all(
        isinstance(value, str) and SHA256.fullmatch(value)
        for value in mapping_bindings.values()
    ):
        detail = "cross-gate migration mapping hash binding differs"
        for gate in mapping_bindings:
            gate_results[gate]["violations"].append(detail)
            gate_results[gate]["status"] = "BLOCKED"
    g6_input_bindings = {
        "api_oracle_sha256": (
            gate_payloads.get("G0", {}).get("api_oracle_sha256"),
            gate_payloads.get("G6", {}).get("api_oracle_sha256"),
        ),
        "api_rules_sha256": (
            gate_payloads.get("G0", {}).get("api_rules_sha256"),
            gate_payloads.get("G6", {}).get("rules_sha256"),
        ),
        "comparator_sha256": (
            gate_payloads.get("G0", {}).get("comparator_sha256"),
            gate_payloads.get("G6", {}).get("comparator_sha256"),
        ),
        "build_api_oracle_sha256": (
            gate_payloads.get("G0", {}).get("build_api_oracle_sha256"),
            gate_payloads.get("G6", {}).get("build_api_oracle_sha256"),
        ),
    }
    for label, values in g6_input_bindings.items():
        if (
            values[0] != values[1]
            or not all(
                isinstance(value, str) and SHA256.fullmatch(value)
                for value in values
            )
        ):
            detail = f"cross-gate {label} binding differs"
            for gate in ("G0", "G6"):
                gate_results[gate]["violations"].append(detail)
                gate_results[gate]["status"] = "BLOCKED"

    signatures = index.get("signatures")
    signature_violations: list[str] = []
    if not isinstance(signatures, list):
        signature_violations.append("signatures must be an array")
        signatures = []
    index_unsigned = dict(index)
    index_unsigned["signatures"] = []
    index_digest = hashlib.sha256(canonical_bytes(index_unsigned)).hexdigest()
    seen_roles: set[str] = set()
    seen_signers: set[str] = set()
    for position, signature in enumerate(signatures):
        if not isinstance(signature, dict):
            signature_violations.append(f"signature[{position}] is invalid")
            continue
        role = str(signature.get("role") or "")
        signer = str(signature.get("signer") or "").strip()
        if (
            role not in REQUIRED_ROLES
            or role in seen_roles
            or not signer
            or signer in seen_signers
            or signature.get("decision") != "GO"
            or signature.get("evidence_index_sha256") != index_digest
            or not str(signature.get("signed_at") or "").strip()
        ):
            signature_violations.append(
                f"signature[{position}] is incomplete, duplicate, or hash-drifted"
            )
            continue
        seen_roles.add(role)
        seen_signers.add(signer)
    missing_roles = REQUIRED_ROLES - seen_roles
    if missing_roles:
        signature_violations.append(
            f"missing signature roles: {sorted(missing_roles)}"
        )

    passed = all(item["status"] == "PASS" for item in gate_results.values())
    passed = passed and not signature_violations
    report = {
        "schema_version": 1,
        "run_id": run_id,
        "decision": "GO" if passed else "NO-GO",
        "status": "PASS" if passed else "BLOCKED",
        "gate_count": len(GATES),
        "passed_gate_count": sum(
            item["status"] == "PASS" for item in gate_results.values()
        ),
        "gates": gate_results,
        "signature_violations": signature_violations,
        "evidence_index_sha256": sha256_file(index_path),
        "unsigned_evidence_index_sha256": index_digest,
        "finalized_at": dt.datetime.now(dt.timezone.utc)
        .isoformat()
        .replace("+00:00", "Z"),
    }
    return index, report


def write_outputs(
    run_dir: pathlib.Path,
    report: dict[str, Any],
    gate_report_path: pathlib.Path,
    decision_path: pathlib.Path,
    ledger_path: pathlib.Path,
) -> None:
    for path in (gate_report_path, decision_path, ledger_path):
        if path.exists():
            raise FileExistsError(f"refusing to overwrite final artifact: {path}")
        if path.parent.resolve() != run_dir:
            raise ValueError("final artifacts must be direct children of run_dir")
    gate_report_path.write_bytes(canonical_bytes(report))
    decision = (
        f"# V8 A/B decision: {report['decision']}\n\n"
        f"Run: `{report['run_id']}`\n\n"
        f"Passed gates: {report['passed_gate_count']}/{report['gate_count']}\n\n"
    )
    blocked = [
        (gate, row)
        for gate, row in report["gates"].items()
        if row["status"] != "PASS"
    ]
    if blocked:
        decision += "## Blocking gates\n\n"
        for gate, row in blocked:
            decision += f"- `{gate}`: {'; '.join(row['violations'])}\n"
    if report["signature_violations"]:
        decision += "\n## Signature blockers\n\n"
        for item in report["signature_violations"]:
            decision += f"- {item}\n"
    decision_path.write_text(decision, encoding="utf-8")
    ledger = {
        "timestamp": report["finalized_at"],
        "gate": "FINAL",
        "claim": f"formal V8 A/B decision is {report['decision']}",
        "status": report["status"],
        "evidence": [
            gate_report_path.name,
            decision_path.name,
        ],
        "executor": "release_finalizer",
        "reviewer": (
            "three_role_signature_set"
            if not report["signature_violations"]
            else None
        ),
        "boundary": "hash-only aggregation; no database, object, HTTP, or UI action",
        "uncertainty": "",
        "blockers": [
            gate
            for gate, row in report["gates"].items()
            if row["status"] != "PASS"
        ]
        + list(report["signature_violations"]),
    }
    ledger_path.write_bytes(canonical_bytes(ledger))


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-dir", type=pathlib.Path, required=True)
    parser.add_argument("--evidence-index", type=pathlib.Path, required=True)
    parser.add_argument("--gate-report", default="final-gate-report.json")
    parser.add_argument("--decision", default="go-no-go.md")
    parser.add_argument("--ledger", default="final-decision-ledger.json")
    args = parser.parse_args()
    try:
        run_dir = args.run_dir.resolve(strict=True)
        if not run_dir.is_dir() or run_dir.is_symlink():
            raise ValueError("run-dir must be an existing non-symlink directory")
        index_path = args.evidence_index.resolve(strict=True)
        if index_path.parent != run_dir or index_path.is_symlink():
            raise ValueError("evidence-index must be a direct run-dir file")
        _, report = validate_index(run_dir, index_path)
        write_outputs(
            run_dir,
            report,
            run_dir / args.gate_report,
            run_dir / args.decision,
            run_dir / args.ledger,
        )
        return 0 if report["decision"] == "GO" else 1
    except (OSError, UnicodeDecodeError, ValueError, json.JSONDecodeError) as exc:
        print(str(exc))
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
