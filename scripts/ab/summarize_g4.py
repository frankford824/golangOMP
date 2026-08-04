#!/usr/bin/env python3
"""Summarize one complete Clone B G4/G10 rehearsal.

The summarizer is intentionally read-only.  It verifies the ordered step
ledger, every recorded stdout/stderr/declared-artifact hash, the rollback
fingerprint envelope, and the G4/G10 timing limits.  Its output is directly
consumable by ``finalize_release_gates.py``.
"""

from __future__ import annotations

import argparse
import csv
import hashlib
import json
import pathlib
import re
from typing import Any


SHA256 = re.compile(r"^[0-9a-f]{64}$")
ROW_FINGERPRINT_ALGORITHM = (
    "sha256(sorted(sha256(canonical-json-cells-v1)),duplicates-preserved)-v1"
)
REQUIRED_STEPS = (
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
SEARCH_TABLES = {
    "task_search_documents",
    "task_asset_group_search_documents",
    "product_search_documents",
}
RECOVERY_OWNERSHIP_ARTIFACTS = {
    *{
        f"recovery-ownership-{asset_id}.json"
        for asset_id in (23989, 23990, 23991)
    },
    *{
        f"recovery-staging-ownership-{asset_id}.json"
        for asset_id in (23989, 23990, 23991)
    },
}
BUNDLE_OWNERSHIP_ARTIFACTS = {
    *{
        f"bundle-ownership-{asset_id}.json"
        for asset_id in range(25557, 25564)
    },
    *{
        f"bundle-staging-ownership-{asset_id}.json"
        for asset_id in range(25557, 25564)
    },
}
COMPONENT_ARTIFACTS = {
    ("recovery", "apply"): {
        "recovery-file-write-ahead.json",
        "recovery-materialization-plan.json",
        "recovery-guard-before.json",
        "recovery-guard-provision.json",
        "recovery-db-apply.json",
        "recovery-db-idempotent.json",
    }
    | RECOVERY_OWNERSHIP_ARTIFACTS,
    ("recovery", "rollback"): {
        "recovery-db-rollback.json",
        "recovery-guard-restore.json",
        "recovery-file-rollback.json",
    }
    | RECOVERY_OWNERSHIP_ARTIFACTS,
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
    | BUNDLE_OWNERSHIP_ARTIFACTS,
    ("bundle", "rollback"): {
        "bundle-db-rollback-journal.json",
        "bundle-db-rollback.json",
        "bundle-guard-restore.json",
        "bundle-file-rollback.json",
    }
    | BUNDLE_OWNERSHIP_ARTIFACTS,
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


def read_steps(path: pathlib.Path) -> list[dict[str, Any]]:
    if path.suffix == ".jsonl":
        rows: list[dict[str, Any]] = []
        for line_number, line in enumerate(
            path.read_text(encoding="utf-8").splitlines(), 1
        ):
            if not line.strip():
                continue
            value = json.loads(line)
            if not isinstance(value, dict):
                raise ValueError(f"steps line {line_number} is not an object")
            rows.append(value)
        return rows
    with path.open(encoding="utf-8", newline="") as handle:
        return list(csv.DictReader(handle, delimiter="\t"))


def safe_recorded_path(
    roots: dict[str, pathlib.Path], record: dict[str, Any]
) -> pathlib.Path:
    root_name = str(record.get("root") or "")
    relative = pathlib.PurePosixPath(str(record.get("path") or ""))
    if (
        root_name not in roots
        or relative.is_absolute()
        or not relative.parts
        or any(part in {"", ".", ".."} for part in relative.parts)
    ):
        raise ValueError("recorded artifact path is unsafe")
    root = roots[root_name]
    target = root.joinpath(*relative.parts)
    if target.is_symlink() or not target.is_file():
        raise ValueError("recorded artifact is missing or symlinked")
    try:
        target.resolve().relative_to(root)
    except ValueError:
        raise ValueError("recorded artifact escapes its allowed root") from None
    return target


def numeric(value: Any, label: str) -> float:
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise ValueError(f"{label} must be numeric")
    if value < 0:
        raise ValueError(f"{label} must be non-negative")
    return float(value)


def validate_fingerprint(
    baseline_path: pathlib.Path,
    rollback_path: pathlib.Path,
) -> tuple[dict[str, Any], dict[str, Any], list[dict[str, str]]]:
    violations: list[dict[str, str]] = []
    try:
        baseline_payload = json.loads(
            baseline_path.read_text(encoding="utf-8")
        )
        payload = json.loads(rollback_path.read_text(encoding="utf-8"))
        if not isinstance(baseline_payload, dict):
            raise ValueError("baseline fingerprint document is not an object")
        if not isinstance(payload, dict):
            raise ValueError("fingerprint document is not an object")
        baseline_tables = baseline_payload.get("tables")
        baseline_internal = str(
            baseline_payload.get("fingerprint_sha256") or ""
        )
        baseline_artifact = sha256_file(baseline_path)
        table_contract_valid = all(
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
            == ROW_FINGERPRINT_ALGORITHM
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
            baseline_payload.get("schema_version") != 1
            or baseline_payload.get("kind")
            != "clone-b-baseline-fingerprint"
            or baseline_payload.get("fingerprint_algorithm")
            != ROW_FINGERPRINT_ALGORITHM
            or not isinstance(baseline_tables, dict)
            or not baseline_tables
            or not table_contract_valid
            or baseline_internal
            != hashlib.sha256(canonical_bytes(baseline_tables)).hexdigest()
            or baseline_artifact
            != hashlib.sha256(canonical_bytes(baseline_payload)).hexdigest()
        ):
            raise ValueError("baseline fingerprint envelope is invalid")
        baseline = str(payload.get("baseline_fingerprint_sha256") or "")
        rollback = str(payload.get("rollback_fingerprint_sha256") or "")
        if (
            payload.get("schema_version") != 1
            or payload.get("status") != "PASS"
            or payload.get("violation_count") != 0
            or not SHA256.fullmatch(baseline)
            or rollback != baseline
            or baseline != baseline_internal
            or payload.get("baseline_artifact_sha256")
            != baseline_artifact
        ):
            violations.append(
                {
                    "violation_code": "g4.rollback_fingerprint_mismatch",
                    "entity_key": "rollback_fingerprint",
                    "detail": "fingerprint envelope is not PASS/equal/zero-violation",
                }
            )
    except (OSError, UnicodeError, json.JSONDecodeError, ValueError) as exc:
        baseline_payload = {}
        payload = {}
        violations.append(
            {
                "violation_code": "g4.rollback_fingerprint_unreadable",
                "entity_key": "rollback_fingerprint",
                "detail": str(exc),
            }
        )
    return baseline_payload, payload, violations


def validate_search_restore(
    snapshot_path: pathlib.Path,
    snapshot_archive_path: pathlib.Path,
    rollback_path: pathlib.Path,
) -> tuple[dict[str, Any], dict[str, Any], list[dict[str, str]]]:
    violations: list[dict[str, str]] = []
    try:
        snapshot = json.loads(snapshot_path.read_text(encoding="utf-8"))
        rollback = json.loads(rollback_path.read_text(encoding="utf-8"))
        if not isinstance(snapshot, dict) or not isinstance(rollback, dict):
            raise ValueError("search restore documents must be objects")
        tables = snapshot.get("tables")
        if (
            snapshot.get("schema_version") != 1
            or snapshot.get("status") != "CAPTURED"
            or snapshot.get("violation_count") != 0
            or not isinstance(tables, dict)
            or set(tables) != SEARCH_TABLES
        ):
            raise ValueError("search snapshot envelope is invalid")
        archive = snapshot.get("archive")
        if (
            not isinstance(archive, dict)
            or set(archive) != {"format", "sha256", "size"}
            or archive.get("format") != "deterministic-jsonl-v1"
            or not SHA256.fullmatch(str(archive.get("sha256") or ""))
            or isinstance(archive.get("size"), bool)
            or not isinstance(archive.get("size"), int)
            or archive["size"] < 0
            or not snapshot_archive_path.is_file()
            or snapshot_archive_path.is_symlink()
            or snapshot_archive_path.stat().st_size != archive["size"]
            or sha256_file(snapshot_archive_path) != archive["sha256"]
        ):
            raise ValueError("search snapshot archive is missing or drifted")
        for name, value in tables.items():
            if (
                not isinstance(value, dict)
                or set(value) != {"row_count", "content_sha256"}
                or isinstance(value["row_count"], bool)
                or not isinstance(value["row_count"], int)
                or value["row_count"] < 0
                or not SHA256.fullmatch(str(value["content_sha256"] or ""))
            ):
                raise ValueError(f"search snapshot table {name} is invalid")
        snapshot_sha = hashlib.sha256(canonical_bytes(tables)).hexdigest()
        if snapshot.get("snapshot_sha256") != snapshot_sha:
            raise ValueError("search snapshot SHA-256 is stale")
        if (
            rollback.get("schema_version") != 1
            or rollback.get("status") != "PASS"
            or rollback.get("violation_count") != 0
            or rollback.get("snapshot_sha256") != snapshot_sha
            or rollback.get("restored_snapshot_sha256") != snapshot_sha
            or rollback.get("restored_tables") != tables
            or rollback.get("source_archive_sha256") != archive["sha256"]
        ):
            raise ValueError("search rollback is not an exact snapshot restore")
    except (OSError, UnicodeError, json.JSONDecodeError, ValueError) as exc:
        snapshot = {}
        rollback = {}
        violations.append(
            {
                "violation_code": "g4.search_restore_mismatch",
                "entity_key": "search_documents",
                "detail": str(exc),
            }
        )
    return snapshot, rollback, violations


def _read_object(path: pathlib.Path, label: str) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{label} is not an object")
    return value


def _validate_self_hash(value: dict[str, Any], label: str) -> None:
    expected = str(value.get("evidence_sha256") or "")
    unhashed = dict(value)
    unhashed.pop("evidence_sha256", None)
    if (
        not SHA256.fullmatch(expected)
        or hashlib.sha256(canonical_bytes(unhashed)).hexdigest() != expected
    ):
        raise ValueError(f"{label} self hash is stale")


def validate_component_chain(
    run_dir: pathlib.Path, run_id: str
) -> tuple[dict[str, Any], list[dict[str, str]]]:
    violations: list[dict[str, str]] = []
    chain: dict[str, Any] = {}
    database: str | None = None
    host: str | None = None
    for component in ("recovery", "bundle"):
        chain[component] = {}
        for action in ("apply", "rollback"):
            label = f"{component}-{action}"
            report_path = run_dir / f"{component}-component-{action}.json"
            try:
                report = _read_object(report_path, label)
                _validate_self_hash(report, label)
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
                    raise ValueError(f"{label} envelope is invalid")
                if database is None:
                    database = str(report.get("database") or "")
                    host = str(report.get("host") or "")
                if (
                    report.get("database") != database
                    or report.get("host") != host
                    or not database
                    or host not in {"127.0.0.1", "localhost"}
                ):
                    raise ValueError(f"{label} database/host binding differs")
                artifacts = report.get("artifacts")
                if not isinstance(artifacts, list):
                    raise ValueError(f"{label} artifacts are missing")
                actual_names: set[str] = set()
                artifact_values: dict[str, dict[str, Any]] = {}
                for item in artifacts:
                    if (
                        not isinstance(item, dict)
                        or set(item) != {"path", "sha256", "size"}
                    ):
                        raise ValueError(f"{label} artifact shape is invalid")
                    name = str(item["path"])
                    if (
                        pathlib.PurePosixPath(name).name != name
                        or name in actual_names
                        or not SHA256.fullmatch(str(item["sha256"] or ""))
                        or isinstance(item["size"], bool)
                        or not isinstance(item["size"], int)
                        or item["size"] < 0
                    ):
                        raise ValueError(f"{label} artifact identity is invalid")
                    target = run_dir / name
                    if (
                        target.is_symlink()
                        or not target.is_file()
                        or target.stat().st_size != item["size"]
                        or sha256_file(target) != item["sha256"]
                    ):
                        raise ValueError(f"{label} artifact {name} drifted")
                    actual_names.add(name)
                    artifact_values[name] = _read_object(target, name)
                if actual_names != COMPONENT_ARTIFACTS[(component, action)]:
                    raise ValueError(f"{label} artifact set differs")

                db_name = f"{component}-db-{action}.json"
                db_report = artifact_values[db_name]
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
                expected_changed = 3 if component == "recovery" else 7
                if (
                    db_report.get("mode") != action
                    or db_report.get("run_id") != run_id
                    or db_report.get("database") != database
                    or db_report.get("host") != host
                    or db_report.get(changed_field) != expected_changed
                    or db_report.get(already_field) != 0
                    or db_report.get("database_transaction_committed")
                    is not True
                ):
                    raise ValueError(f"{label} DB report is invalid")
                if component == "recovery" and (
                    db_report.get("version") != 1
                    or db_report.get("object_storage_writes_executed")
                    is not False
                ):
                    raise ValueError("recovery DB report contract is invalid")
                if component == "bundle" and (
                    db_report.get("schema_version") != 1
                    or db_report.get("status") != "PASS"
                ):
                    raise ValueError("bundle DB report contract is invalid")
                if component == "bundle":
                    journal = artifact_values[
                        "bundle-db-rollback-journal.json"
                    ]
                    journal_evidence = str(
                        journal.get("evidence_sha256") or ""
                    )
                    unhashed_journal = dict(journal)
                    unhashed_journal.pop("evidence_sha256", None)
                    journal_hash = artifact_values_hash(
                        artifacts, "bundle-db-rollback-journal.json"
                    )
                    auto_before = journal.get("auto_increment_before")
                    auto_ceilings = journal.get(
                        "auto_increment_ceilings"
                    )
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
                        or journal.get("database_commit_state")
                        != "unknown"
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
                            isinstance(
                                before_state.get("next_value"), bool
                            )
                            or not isinstance(
                                before_state.get("next_value"), int
                            )
                            or before_state["next_value"] <= 0
                            or isinstance(
                                ceiling.get("next_value"), bool
                            )
                            or not isinstance(
                                ceiling.get("next_value"), int
                            )
                            or ceiling["next_value"]
                            < before_state["next_value"]
                            for before_state, ceiling in zip(
                                auto_before, auto_ceilings
                            )
                        )
                        or not SHA256.fullmatch(journal_evidence)
                        or hashlib.sha256(
                            json.dumps(
                                unhashed_journal,
                                ensure_ascii=False,
                                sort_keys=True,
                                separators=(",", ":"),
                            ).encode("utf-8")
                        ).hexdigest()
                        != journal_evidence
                        or db_report.get("rollback_journal_sha256")
                        != journal_hash
                        or db_report.get(
                            "rollback_journal_evidence_sha256"
                        )
                        != journal_evidence
                    ):
                        raise ValueError(
                            "bundle rollback journal binding is invalid"
                        )

                guard_before = artifact_values.get(
                    f"{component}-guard-before.json"
                )
                if guard_before is not None:
                    _validate_self_hash(guard_before, f"{label} guard-before")
                    if (
                        guard_before.get("kind") != "clone-b-guard-state"
                        or guard_before.get("component") != component
                        or guard_before.get("database") != database
                        or not isinstance(guard_before.get("rows"), list)
                        or not isinstance(guard_before.get("schema"), list)
                    ):
                        raise ValueError(f"{label} guard baseline is invalid")
                    provision = artifact_values[
                        f"{component}-guard-provision.json"
                    ]
                    _validate_self_hash(provision, f"{label} guard-provision")
                    if (
                        provision.get("status") != "PROVISIONED"
                        or provision.get("component") != component
                        or provision.get("database") != database
                        or provision.get("before_artifact_sha256")
                        != artifact_values_hash(
                            artifacts, f"{component}-guard-before.json"
                        )
                        or not isinstance(provision.get("binding"), dict)
                    ):
                        raise ValueError(f"{label} guard provision is invalid")
                    idempotent = artifact_values[
                        f"{component}-db-idempotent.json"
                    ]
                    if (
                        idempotent.get("mode") != "apply"
                        or idempotent.get("run_id") != run_id
                        or idempotent.get("database") != database
                        or idempotent.get("host") != host
                        or idempotent.get(changed_field) != 0
                        or idempotent.get(already_field) != expected_changed
                        or idempotent.get("database_transaction_committed")
                        is not True
                    ):
                        raise ValueError(
                            f"{label} idempotent DB report is invalid"
                        )
                else:
                    restore = artifact_values[
                        f"{component}-guard-restore.json"
                    ]
                    _validate_self_hash(restore, f"{label} guard-restore")
                    if (
                        restore.get("status") != "RESTORED"
                        or restore.get("component") != component
                        or restore.get("database") != database
                        or restore.get("exact") is not True
                    ):
                        raise ValueError(f"{label} guard restore is invalid")
                    file_report = artifact_values[
                        f"{component}-file-rollback.json"
                    ]
                    if (
                        file_report.get("status") != "ROLLED_BACK"
                        or file_report.get("database_write_performed")
                        is not False
                    ):
                        raise ValueError(f"{label} file rollback is invalid")
                chain[component][action] = report
                chain[component][action]["artifact_sha256"] = sha256_file(
                    report_path
                )
            except (
                OSError,
                UnicodeError,
                json.JSONDecodeError,
                ValueError,
            ) as exc:
                violations.append(
                    {
                        "violation_code": "g4.component_chain_invalid",
                        "entity_key": label,
                        "detail": str(exc),
                    }
                )
                chain[component][action] = {}
    return chain, violations


def artifact_values_hash(
    artifacts: list[dict[str, Any]], name: str
) -> str:
    for item in artifacts:
        if item.get("path") == name:
            return str(item.get("sha256") or "")
    return ""


def summarize(
    *,
    run_id: str,
    run_dir: pathlib.Path,
    clone_root: pathlib.Path,
    steps_path: pathlib.Path,
    baseline_fingerprint_path: pathlib.Path,
    rollback_fingerprint_path: pathlib.Path,
    search_snapshot_path: pathlib.Path,
    search_snapshot_archive_path: pathlib.Path,
    search_rollback_path: pathlib.Path,
    total_seconds: float,
    max_step: float,
    max_phase: float,
    max_total: float,
) -> dict[str, Any]:
    violations: list[dict[str, str]] = []
    rows = read_steps(steps_path)
    actual_sequence = [
        (str(row.get("step") or ""), str(row.get("phase") or ""))
        for row in rows
    ]
    if actual_sequence != list(REQUIRED_STEPS):
        violations.append(
            {
                "violation_code": "g4.step_sequence",
                "entity_key": "steps",
                "detail": repr(actual_sequence),
            }
        )

    roots = {"run_dir": run_dir.resolve(), "clone_root": clone_root.resolve()}
    timings = {"apply": 0.0, "validate": 0.0, "rollback": 0.0}
    normalized_steps: list[dict[str, Any]] = []
    for index, row in enumerate(rows):
        step = str(row.get("step") or f"index-{index}")
        phase = str(row.get("phase") or "")
        try:
            exit_code = row.get("exit_code")
            if isinstance(exit_code, bool) or not isinstance(exit_code, int):
                raise ValueError("exit_code must be an integer")
            elapsed = numeric(row.get("elapsed_seconds"), "elapsed_seconds")
        except ValueError as exc:
            violations.append(
                {
                    "violation_code": "g4.step_numeric_invalid",
                    "entity_key": step,
                    "detail": str(exc),
                }
            )
            exit_code, elapsed = 125, 0.0
        if phase in timings:
            timings[phase] += elapsed
        else:
            violations.append(
                {
                    "violation_code": "g4.phase_invalid",
                    "entity_key": step,
                    "detail": phase,
                }
            )
        if exit_code != 0:
            violations.append(
                {
                    "violation_code": "g4.step_failed",
                    "entity_key": step,
                    "detail": str(exit_code),
                }
            )
        if elapsed > max_step:
            violations.append(
                {
                    "violation_code": "g4.step_timeout",
                    "entity_key": step,
                    "detail": str(elapsed),
                }
            )
        for evidence in row.get("evidence", []):
            try:
                if not isinstance(evidence, dict):
                    raise ValueError("evidence entry is not an object")
                expected = str(evidence.get("sha256") or "")
                if not SHA256.fullmatch(expected):
                    raise ValueError("evidence SHA-256 is invalid")
                target = safe_recorded_path(roots, evidence)
                actual = sha256_file(target)
                if actual != expected:
                    raise ValueError(
                        f"hash mismatch expected={expected} actual={actual}"
                    )
            except (OSError, ValueError) as exc:
                violations.append(
                    {
                        "violation_code": "g4.evidence_drift",
                        "entity_key": step,
                        "detail": str(exc),
                    }
                )
        normalized_steps.append(
            {
                "step": step,
                "phase": phase,
                "status": "PASS" if exit_code == 0 else "BLOCKED",
                "exit_code": int(exit_code),
                "elapsed_seconds": elapsed,
                "command_sha256": str(row.get("command_sha256") or ""),
                "evidence": row.get("evidence", []),
            }
        )

    for phase, elapsed in timings.items():
        if elapsed > max_phase:
            violations.append(
                {
                    "violation_code": "g10.phase_timeout",
                    "entity_key": phase,
                    "detail": str(elapsed),
                }
            )
    total = numeric(total_seconds, "total_seconds")
    timings["total"] = total
    if total > max_total:
        violations.append(
            {
                "violation_code": "g10.total_timeout",
                "entity_key": "total",
                "detail": str(total),
            }
        )

    baseline_fingerprint, fingerprint, fingerprint_violations = (
        validate_fingerprint(
            baseline_fingerprint_path,
            rollback_fingerprint_path,
        )
    )
    violations.extend(fingerprint_violations)
    search_snapshot, search_rollback, search_violations = validate_search_restore(
        search_snapshot_path,
        search_snapshot_archive_path,
        search_rollback_path,
    )
    violations.extend(search_violations)
    component_chain, component_violations = validate_component_chain(
        run_dir, run_id
    )
    violations.extend(component_violations)
    status = "PASS" if not violations else "BLOCKED"
    return {
        "schema_version": 2,
        "run_id": run_id,
        "status": status,
        "violation_count": len(violations),
        "violations": violations,
        "exit_code": 0 if status == "PASS" else 1,
        "elapsed_seconds": total,
        "total_seconds": total,
        "timings_seconds": timings,
        "steps": normalized_steps,
        "baseline_fingerprint": baseline_fingerprint,
        "baseline_fingerprint_artifact_sha256": (
            sha256_file(baseline_fingerprint_path)
            if baseline_fingerprint_path.is_file()
            else None
        ),
        "rollback_fingerprint": fingerprint,
        "rollback_fingerprint_sha256": (
            sha256_file(rollback_fingerprint_path)
            if rollback_fingerprint_path.is_file()
            else None
        ),
        "search_restore": {
            "snapshot": search_snapshot,
            "rollback": search_rollback,
        },
        "component_chain": component_chain,
        "search_snapshot_sha256": (
            sha256_file(search_snapshot_path)
            if search_snapshot_path.is_file()
            else None
        ),
        "search_rollback_sha256": (
            sha256_file(search_rollback_path)
            if search_rollback_path.is_file()
            else None
        ),
        "search_snapshot_archive_sha256": (
            sha256_file(search_snapshot_archive_path)
            if search_snapshot_archive_path.is_file()
            else None
        ),
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--run-dir", type=pathlib.Path, required=True)
    parser.add_argument("--clone-root", type=pathlib.Path, required=True)
    parser.add_argument("--steps", type=pathlib.Path, required=True)
    parser.add_argument(
        "--baseline-fingerprint", type=pathlib.Path, required=True
    )
    parser.add_argument("--rollback-fingerprint", type=pathlib.Path, required=True)
    parser.add_argument("--search-snapshot", type=pathlib.Path, required=True)
    parser.add_argument(
        "--search-snapshot-archive", type=pathlib.Path, required=True
    )
    parser.add_argument("--search-rollback", type=pathlib.Path, required=True)
    parser.add_argument("--total-seconds", type=float, required=True)
    parser.add_argument("--max-step", type=float, default=600)
    parser.add_argument("--max-phase", type=float, default=600)
    parser.add_argument("--max-total", type=float, default=1800)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    args = parser.parse_args()
    result = summarize(
        run_id=args.run_id,
        run_dir=args.run_dir,
        clone_root=args.clone_root,
        steps_path=args.steps,
        baseline_fingerprint_path=args.baseline_fingerprint,
        rollback_fingerprint_path=args.rollback_fingerprint,
        search_snapshot_path=args.search_snapshot,
        search_snapshot_archive_path=args.search_snapshot_archive,
        search_rollback_path=args.search_rollback,
        total_seconds=args.total_seconds,
        max_step=args.max_step,
        max_phase=args.max_phase,
        max_total=args.max_total,
    )
    args.output.write_bytes(canonical_bytes(result))
    return result["exit_code"]


if __name__ == "__main__":
    raise SystemExit(main())
