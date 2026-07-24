#!/usr/bin/env python3
"""Compose the final G06 verdict without re-reading remote object bytes.

The adjudicator binds four independently produced evidence domains:

* the remote hydration input/evidence and the remote subset of the finalized
  object manifest;
* exactly seven Clone B bundle rows and an independent local object-verifier
  PASS over their canonical JSONL subset;
* the one reviewed historical-unavailable tombstone;
* the original read-only Clone B SQL and controlled GET/410 API evidence used
  to build the historical-unavailable attestation.

Only small JSON/JSONL evidence files are read.  No network, database, object
storage, or production write capability exists in this module.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import re
import tempfile
from typing import Any

try:
    from scripts.ab import historical_unavailable_exception as historical
    from scripts.ab import hydrate_object_manifest as hydrator
    from scripts.ab import object_manifest_verifier as object_verifier
except ModuleNotFoundError:  # Direct execution from scripts/ab.
    import historical_unavailable_exception as historical
    import hydrate_object_manifest as hydrator
    import object_manifest_verifier as object_verifier


SCHEMA_VERSION = 1
ZERO_SHA256 = "0" * 64
SHA256 = re.compile(r"^[0-9a-f]{64}$")
HYDRATION_FIELDS = {
    "schema_version",
    "status",
    "input_manifest_sha256",
    "hydrated_manifest_sha256",
    "checkpoint_sha256",
    "row_count",
    "already_complete_count",
    "missing_sha256_count",
    "configured_target_row_count",
    "unique_target_count",
    "resumed_target_count",
    "resumed_failure_target_count",
    "retried_transient_failure_target_count",
    "retried_authorized_failure_target_count",
    "failure_retry_authorization_sha256",
    "read_only_get_count",
    "hydrated_row_count",
    "deduplicated_get_count",
    "failure_count",
    "failures",
    "evidence_hash",
}
OBJECT_VERDICT_FIELDS = {
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
BUNDLE_TASKS = {
    25557: 485,
    25558: 523,
    25559: 523,
    25560: 2234,
    25561: 2251,
    25562: 2477,
    25563: 2598,
}
HYDRATABLE_FIELDS = {"size", "mime_type", "sha256"}


class AdjudicationError(ValueError):
    """A secret-free fail-closed adjudication error."""

    def __init__(self, code: str, detail: str) -> None:
        super().__init__(detail)
        self.code = code
        self.detail = detail


def reject_duplicate_keys(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise AdjudicationError(
                "g06.json_duplicate_key", f"duplicate JSON object key: {key}"
            )
        result[key] = value
    return result


def canonical_json(value: Any) -> str:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    )


def canonical_jsonl(rows: list[dict[str, Any]]) -> bytes:
    return "".join(canonical_json(row) + "\n" for row in rows).encode("utf-8")


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def require_sha(value: Any, label: str, *, nonzero: bool = True) -> str:
    if (
        not isinstance(value, str)
        or not SHA256.fullmatch(value)
        or (nonzero and value == ZERO_SHA256)
    ):
        raise AdjudicationError(
            "g06.hash_invalid", f"{label} must be a non-zero lowercase SHA-256"
        )
    return value


def read_json(path: pathlib.Path, label: str) -> tuple[dict[str, Any], str]:
    if path.is_symlink() or not path.is_file():
        raise AdjudicationError("g06.evidence_missing", f"{label} is missing")
    try:
        value = json.loads(
            path.read_text(encoding="utf-8"),
            object_pairs_hook=reject_duplicate_keys,
        )
    except UnicodeDecodeError as exc:
        raise AdjudicationError(
            "g06.evidence_invalid", f"{label} is not UTF-8 JSON"
        ) from exc
    except json.JSONDecodeError as exc:
        raise AdjudicationError(
            "g06.evidence_invalid", f"{label} is not valid JSON"
        ) from exc
    if not isinstance(value, dict):
        raise AdjudicationError(
            "g06.evidence_invalid", f"{label} must contain a JSON object"
        )
    return value, sha256_file(path)


def row_sort_key(row: dict[str, Any]) -> tuple[str, int, int, str]:
    return (
        str(row["owner_kind"]),
        int(row["owner_id"]),
        int(row["task_id"]),
        str(row["entity_key"]),
    )


def read_jsonl(
    path: pathlib.Path,
    label: str,
    *,
    final_manifest: bool,
) -> tuple[list[dict[str, Any]], str]:
    if path.is_symlink() or not path.is_file():
        raise AdjudicationError("g06.manifest_missing", f"{label} is missing")
    try:
        raw = path.read_bytes()
        text = raw.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise AdjudicationError(
            "g06.manifest_invalid", f"{label} is not UTF-8"
        ) from exc
    rows: list[dict[str, Any]] = []
    seen: set[str] = set()
    for line_no, line in enumerate(text.splitlines(), 1):
        if not line:
            raise AdjudicationError(
                "g06.manifest_invalid", f"{label} row {line_no} is blank"
            )
        try:
            row = json.loads(line, object_pairs_hook=reject_duplicate_keys)
        except json.JSONDecodeError as exc:
            raise AdjudicationError(
                "g06.manifest_invalid", f"{label} row {line_no} is invalid JSON"
            ) from exc
        if not isinstance(row, dict):
            raise AdjudicationError(
                "g06.manifest_invalid", f"{label} row {line_no} is not an object"
            )
        if final_manifest and row.get("entity_key") == historical.ENTITY_KEY:
            try:
                historical.validate_exception_object_row(row)
                problems = []
            except historical.ExceptionContractError as exc:
                raise AdjudicationError(
                    "g06.exception_row_invalid", str(exc)
                ) from exc
        elif final_manifest:
            problems = object_verifier.validate_contract(row, line_no)
        else:
            try:
                hydrator.validate_hydration_row(row, line_no)
                problems = []
            except ValueError as exc:
                raise AdjudicationError(
                    "g06.manifest_invalid", str(exc)
                ) from exc
        if problems:
            raise AdjudicationError(
                "g06.manifest_invalid",
                f"{label} row {line_no}: {problems[0]['violation_code']}",
            )
        entity = str(row["entity_key"])
        if entity in seen:
            raise AdjudicationError(
                "g06.manifest_duplicate", f"{label} duplicates {entity}"
            )
        seen.add(entity)
        rows.append(row)
    if not rows:
        raise AdjudicationError("g06.manifest_empty", f"{label} is empty")
    if rows != sorted(rows, key=row_sort_key) or raw != canonical_jsonl(rows):
        raise AdjudicationError(
            "g06.manifest_noncanonical",
            f"{label} is not canonical sorted JSONL",
        )
    return rows, sha256_bytes(raw)


def split_final_rows(
    rows: list[dict[str, Any]],
) -> tuple[list[dict[str, Any]], list[dict[str, Any]], dict[str, Any]]:
    exceptions = [
        row for row in rows if row["entity_key"] == historical.ENTITY_KEY
    ]
    if len(exceptions) != 1:
        raise AdjudicationError(
            "g06.exception_count",
            "final manifest must contain exactly one historical exception",
        )
    bundles = [
        row for row in rows if row["storage_adapter"] == "clone_b_bundle"
    ]
    bundle_ids = {int(row["owner_id"]) for row in bundles}
    if bundle_ids != set(BUNDLE_TASKS) or len(bundles) != len(BUNDLE_TASKS):
        raise AdjudicationError(
            "g06.bundle_count",
            "final manifest must contain exactly the seven reviewed Clone B bundles",
        )
    for row in bundles:
        owner_id = int(row["owner_id"])
        if (
            row["entity_key"] != f"task_asset:{owner_id}"
            or row["owner_kind"] != "task_asset"
            or int(row["task_id"]) != BUNDLE_TASKS[owner_id]
            or row["mime_type"] != "application/zip"
            or not str(row["object_key"]).endswith("/source-bundle.zip")
        ):
            raise AdjudicationError(
                "g06.bundle_identity",
                f"Clone B bundle task_asset:{owner_id} identity differs",
            )
    excluded = {historical.ENTITY_KEY} | {
        str(row["entity_key"]) for row in bundles
    }
    remote = [row for row in rows if row["entity_key"] not in excluded]
    for row in remote:
        adapter = str(row["storage_adapter"]).strip().lower()
        if adapter not in object_verifier.UPLOAD_ADAPTERS | object_verifier.OSS_ADAPTERS:
            raise AdjudicationError(
                "g06.remote_adapter",
                "remote manifest subset contains a non-remote adapter",
            )
    if len(rows) != len(remote) + len(BUNDLE_TASKS) + 1:
        raise AdjudicationError(
            "g06.final_count", "final manifest composition count differs"
        )
    return remote, bundles, exceptions[0]


def target_kind(storage_adapter: str) -> str:
    normalized = storage_adapter.strip().lower()
    if normalized in object_verifier.UPLOAD_ADAPTERS:
        return "upload"
    if normalized in object_verifier.OSS_ADAPTERS:
        return "oss"
    raise AdjudicationError(
        "g06.remote_adapter", "hydration input contains an unsupported adapter"
    )


def verify_remote_hydration(
    input_rows: list[dict[str, Any]],
    input_sha: str,
    remote_rows: list[dict[str, Any]],
    evidence_path: pathlib.Path,
) -> tuple[dict[str, Any], str, str]:
    evidence, evidence_sha = read_json(evidence_path, "hydration evidence")
    if set(evidence) != HYDRATION_FIELDS:
        raise AdjudicationError(
            "g06.hydration_contract", "hydration evidence field contract differs"
        )
    unsigned = {
        key: value for key, value in evidence.items() if key != "evidence_hash"
    }
    if evidence.get("evidence_hash") != sha256_bytes(
        canonical_json(unsigned).encode("utf-8")
    ):
        raise AdjudicationError(
            "g06.hydration_self_hash", "hydration evidence self-hash differs"
        )
    if evidence.get("schema_version") != 1 or evidence.get("status") != "PASS":
        raise AdjudicationError(
            "g06.hydration_status", "hydration evidence is not PASS"
        )
    if evidence.get("input_manifest_sha256") != input_sha:
        raise AdjudicationError(
            "g06.hydration_input_hash",
            "hydration evidence is not bound to the supplied input",
        )
    if len(input_rows) != len(remote_rows):
        raise AdjudicationError(
            "g06.remote_row_count",
            "hydration input and finalized remote subset row counts differ",
        )
    for index, (source, final) in enumerate(zip(input_rows, remote_rows), 1):
        if source["entity_key"] != final["entity_key"]:
            raise AdjudicationError(
                "g06.remote_entity_order",
                f"remote entity order differs at row {index}",
            )
        for field in object_verifier.REQUIRED_FIELDS - HYDRATABLE_FIELDS:
            if source[field] != final[field]:
                raise AdjudicationError(
                    "g06.remote_immutable_drift",
                    f"hydration changed immutable field {source['entity_key']}.{field}",
                )
        if source["sha256"]:
            for field in HYDRATABLE_FIELDS:
                if source[field] != final[field]:
                    raise AdjudicationError(
                        "g06.remote_complete_drift",
                        f"hydration changed reviewed field {source['entity_key']}.{field}",
                    )
    remote_sha = sha256_bytes(canonical_jsonl(remote_rows))
    if evidence.get("hydrated_manifest_sha256") != remote_sha:
        raise AdjudicationError(
            "g06.hydration_output_hash",
            "hydration output is not the finalized remote manifest subset",
        )

    complete_count = sum(bool(row["sha256"]) for row in input_rows)
    missing_count = len(input_rows) - complete_count
    unique_targets = {
        (target_kind(row["storage_adapter"]), row["object_key"])
        for row in input_rows
        if not row["sha256"]
    }
    expected_counts = {
        "row_count": len(input_rows),
        "already_complete_count": complete_count,
        "missing_sha256_count": missing_count,
        "configured_target_row_count": missing_count,
        "unique_target_count": len(unique_targets),
        "resumed_failure_target_count": 0,
        "hydrated_row_count": missing_count,
        "deduplicated_get_count": missing_count - len(unique_targets),
        "failure_count": 0,
    }
    for field, expected in expected_counts.items():
        value = evidence.get(field)
        if (
            not isinstance(value, int)
            or isinstance(value, bool)
            or value != expected
        ):
            raise AdjudicationError(
                "g06.hydration_count",
                f"hydration evidence {field} does not equal {expected}",
            )
    resumed = evidence.get("resumed_target_count")
    read_gets = evidence.get("read_only_get_count")
    retried = evidence.get("retried_transient_failure_target_count")
    retried_authorized = evidence.get(
        "retried_authorized_failure_target_count"
    )
    if any(
        not isinstance(value, int) or isinstance(value, bool) or value < 0
        for value in (resumed, read_gets, retried, retried_authorized)
    ):
        raise AdjudicationError(
            "g06.hydration_count", "hydration execution counts are invalid"
        )
    if (
        resumed + read_gets != len(unique_targets)
        or retried + retried_authorized > read_gets
    ):
        raise AdjudicationError(
            "g06.hydration_coverage",
            "checkpoint resumes and read-only GETs do not cover every remote target",
        )
    authorization_sha = require_sha(
        evidence.get("failure_retry_authorization_sha256"),
        "failure retry authorization",
        nonzero=False,
    )
    if (authorization_sha == ZERO_SHA256) != (retried_authorized == 0):
        raise AdjudicationError(
            "g06.hydration_authorization",
            "authorized retry count and authorization hash disagree",
        )
    if evidence.get("failures") != []:
        raise AdjudicationError(
            "g06.hydration_failures", "hydration evidence contains failures"
        )
    require_sha(evidence.get("checkpoint_sha256"), "hydration checkpoint")
    return evidence, evidence_sha, remote_sha


def verify_bundle_verdict(
    bundles: list[dict[str, Any]], verdict_path: pathlib.Path
) -> tuple[str, str]:
    verdict, verdict_sha = read_json(verdict_path, "bundle verifier verdict")
    if set(verdict) != OBJECT_VERDICT_FIELDS:
        raise AdjudicationError(
            "g06.bundle_verdict_contract",
            "bundle verifier verdict field contract differs",
        )
    unsigned = {
        key: value for key, value in verdict.items() if key != "evidence_hash"
    }
    if verdict.get("evidence_hash") != sha256_bytes(
        canonical_json(unsigned).encode("utf-8")
    ):
        raise AdjudicationError(
            "g06.bundle_verdict_self_hash",
            "bundle verifier verdict self-hash differs",
        )
    bundle_sha = sha256_bytes(canonical_jsonl(bundles))
    if (
        verdict.get("schema_version") != object_verifier.SCHEMA_VERSION
        or verdict.get("status") != "PASS"
        or verdict.get("violation_count") != 0
        or verdict.get("checked_count") != len(BUNDLE_TASKS)
        or verdict.get("manifest_sha256") != bundle_sha
        or verdict.get("exception_count") != 0
        or verdict.get("exception_evidence_sha256") != ZERO_SHA256
        or verdict.get("mapping_sha256") != ZERO_SHA256
        or verdict.get("mapping_row_hash") != ZERO_SHA256
        or verdict.get("exceptions") != []
        or verdict.get("violations") != []
    ):
        raise AdjudicationError(
            "g06.bundle_verdict",
            "bundle verifier did not independently pass the exact seven-row subset",
        )
    return verdict_sha, bundle_sha


def verify_historical_exception(
    *,
    mapping_path: pathlib.Path,
    expected_mapping_sha256: str,
    final_manifest_path: pathlib.Path,
    exception_path: pathlib.Path,
    sql_path: pathlib.Path,
    api_path: pathlib.Path,
) -> tuple[str, str, str, str]:
    mapping_sha, mapping_row_hash, _row = historical.validate_mapping(mapping_path)
    if mapping_sha != expected_mapping_sha256:
        raise AdjudicationError(
            "g06.mapping_hash", "mapping SHA-256 differs from the frozen candidate"
        )
    attestation, _exception, attestation_sha = historical.load_attestation(
        exception_path, manifest_path=final_manifest_path
    )
    if (
        attestation["mapping_sha256"] != mapping_sha
        or attestation["mapping_row_hash"] != mapping_row_hash
    ):
        raise AdjudicationError(
            "g06.exception_mapping",
            "historical exception mapping binding differs",
        )
    sql, sql_sha = read_json(sql_path, "historical exception SQL evidence")
    api, api_sha = read_json(api_path, "historical exception API evidence")
    historical.validate_sql_evidence(sql, mapping_sha, mapping_row_hash)
    historical.validate_api_evidence(api, mapping_sha, mapping_row_hash)
    if (
        attestation["sql_evidence_sha256"] != sql_sha
        or attestation["api_evidence_sha256"] != api_sha
    ):
        raise AdjudicationError(
            "g06.exception_evidence_hash",
            "historical exception is not bound to the supplied SQL/API evidence",
        )
    return attestation_sha, sql_sha, api_sha, mapping_row_hash


def result_document(
    *,
    status: str,
    violations: list[dict[str, str]],
    hashes: dict[str, str],
    remote_row_count: int,
    bundle_row_count: int,
    exception_count: int,
    final_row_count: int,
) -> dict[str, Any]:
    result: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "status": status,
        "violation_count": len(violations),
        "violations": violations,
        "checked_count": final_row_count if status == "PASS" else 0,
        "remote_row_count": remote_row_count,
        "bundle_row_count": bundle_row_count,
        "exception_count": exception_count,
        "final_row_count": final_row_count,
        **hashes,
    }
    result["evidence_hash"] = sha256_bytes(
        canonical_json(result).encode("utf-8")
    )
    return result


def object_verdict_from_composition(
    composition: dict[str, Any],
    exception_path: pathlib.Path,
) -> dict[str, Any]:
    """Adapt the rich composition ledger to the existing G8/G06 envelope."""
    if composition.get("status") != "PASS":
        first = (composition.get("violations") or [{}])[0]
        return object_verifier.finalize_result(
            str(composition.get("final_manifest_sha256") or ZERO_SHA256),
            0,
            [
                object_verifier.violation(
                    str(
                        first.get("violation_code")
                        or "object_manifest.g06_composition_blocked"
                    ),
                    "*",
                    str(first.get("detail") or "G06 composition is blocked"),
                    blocked=True,
                )
            ],
        )
    attestation, exception, exception_sha = historical.load_attestation(
        exception_path
    )
    if (
        exception_sha != composition["historical_exception_sha256"]
        or attestation["mapping_sha256"] != composition["mapping_sha256"]
        or attestation["mapping_row_hash"]
        != composition["historical_mapping_row_hash"]
    ):
        raise AdjudicationError(
            "g06.object_verdict_binding",
            "standard object verdict inputs differ from the PASS composition",
        )
    accepted_exception = {
        "entity_key": exception["entity_key"],
        "task_id": exception["task_id"],
        "missing_task_asset_id": exception["missing_task_asset_id"],
        "expected_http_status": historical.EXPECTED_HTTP_STATUS,
        "observed_http_status": historical.EXPECTED_HTTP_STATUS,
        "mapping_row_hash": exception["mapping_row_hash"],
        "object_row_sha256": exception["object_row_sha256"],
        "working_reference_count": exception["working_reference_count"],
        "finalized_reference_count": exception["finalized_reference_count"],
    }
    return object_verifier.finalize_result(
        composition["final_manifest_sha256"],
        int(composition["final_row_count"]),
        [],
        exception_evidence_sha256=exception_sha,
        mapping_sha256=composition["mapping_sha256"],
        mapping_row_hash=composition["historical_mapping_row_hash"],
        exceptions=[accepted_exception],
    )


def bind_standard_verdict(
    composition: dict[str, Any],
    object_verdict: dict[str, Any],
) -> dict[str, Any]:
    """Return a rich ledger that names the exact downstream verdict bytes."""
    if set(object_verdict) != OBJECT_VERDICT_FIELDS:
        raise AdjudicationError(
            "g06.object_verdict_contract",
            "standard object verdict field contract differs",
        )
    unsigned_verdict = {
        key: value
        for key, value in object_verdict.items()
        if key != "evidence_hash"
    }
    if object_verdict.get("evidence_hash") != sha256_bytes(
        canonical_json(unsigned_verdict).encode("utf-8")
    ):
        raise AdjudicationError(
            "g06.object_verdict_self_hash",
            "standard object verdict self-hash differs",
        )
    if object_verdict.get("status") != composition.get("status"):
        raise AdjudicationError(
            "g06.object_verdict_status",
            "standard object verdict status differs from the composition",
        )
    ledger = {
        key: value
        for key, value in composition.items()
        if key != "evidence_hash"
    }
    ledger["object_verdict_sha256"] = sha256_bytes(
        (canonical_json(object_verdict) + "\n").encode("utf-8")
    )
    ledger["evidence_hash"] = sha256_bytes(
        canonical_json(ledger).encode("utf-8")
    )
    return ledger


def adjudicate(
    *,
    mapping_path: pathlib.Path,
    expected_mapping_sha256: str,
    hydration_input_path: pathlib.Path,
    expected_hydration_input_sha256: str,
    hydration_evidence_path: pathlib.Path,
    final_manifest_path: pathlib.Path,
    expected_final_manifest_sha256: str,
    bundle_verdict_path: pathlib.Path,
    exception_path: pathlib.Path,
    sql_path: pathlib.Path,
    api_path: pathlib.Path,
) -> dict[str, Any]:
    hashes = {
        "mapping_sha256": ZERO_SHA256,
        "hydration_input_sha256": ZERO_SHA256,
        "hydration_evidence_sha256": ZERO_SHA256,
        "remote_hydrated_manifest_sha256": ZERO_SHA256,
        "final_manifest_sha256": ZERO_SHA256,
        "bundle_manifest_sha256": ZERO_SHA256,
        "bundle_verdict_sha256": ZERO_SHA256,
        "historical_exception_sha256": ZERO_SHA256,
        "historical_sql_evidence_sha256": ZERO_SHA256,
        "historical_api_evidence_sha256": ZERO_SHA256,
        "historical_mapping_row_hash": ZERO_SHA256,
    }
    remote_count = bundle_count = exception_count = final_count = 0
    try:
        require_sha(expected_mapping_sha256, "expected mapping SHA-256")
        require_sha(
            expected_hydration_input_sha256,
            "expected hydration input SHA-256",
        )
        require_sha(
            expected_final_manifest_sha256,
            "expected final manifest SHA-256",
        )
        final_rows, final_sha = read_jsonl(
            final_manifest_path, "final manifest", final_manifest=True
        )
        hashes["final_manifest_sha256"] = final_sha
        if final_sha != expected_final_manifest_sha256:
            raise AdjudicationError(
                "g06.final_manifest_hash",
                "final manifest SHA-256 differs from the finalized boundary",
            )
        remote, bundles, _exception_row = split_final_rows(final_rows)
        remote_count, bundle_count, exception_count, final_count = (
            len(remote),
            len(bundles),
            1,
            len(final_rows),
        )
        hydration_input, hydration_input_sha = read_jsonl(
            hydration_input_path,
            "hydration input",
            final_manifest=False,
        )
        hashes["hydration_input_sha256"] = hydration_input_sha
        if hydration_input_sha != expected_hydration_input_sha256:
            raise AdjudicationError(
                "g06.hydration_input_hash",
                "hydration input SHA-256 differs from the prepared boundary",
            )
        _hydration, hydration_evidence_sha, remote_sha = (
            verify_remote_hydration(
                hydration_input,
                hydration_input_sha,
                remote,
                hydration_evidence_path,
            )
        )
        hashes["hydration_evidence_sha256"] = hydration_evidence_sha
        hashes["remote_hydrated_manifest_sha256"] = remote_sha
        bundle_verdict_sha, bundle_manifest_sha = verify_bundle_verdict(
            bundles, bundle_verdict_path
        )
        hashes["bundle_verdict_sha256"] = bundle_verdict_sha
        hashes["bundle_manifest_sha256"] = bundle_manifest_sha
        (
            exception_sha,
            sql_sha,
            api_sha,
            mapping_row_hash,
        ) = verify_historical_exception(
            mapping_path=mapping_path,
            expected_mapping_sha256=expected_mapping_sha256,
            final_manifest_path=final_manifest_path,
            exception_path=exception_path,
            sql_path=sql_path,
            api_path=api_path,
        )
        hashes["mapping_sha256"] = expected_mapping_sha256
        hashes["historical_exception_sha256"] = exception_sha
        hashes["historical_sql_evidence_sha256"] = sql_sha
        hashes["historical_api_evidence_sha256"] = api_sha
        hashes["historical_mapping_row_hash"] = mapping_row_hash
        return result_document(
            status="PASS",
            violations=[],
            hashes=hashes,
            remote_row_count=remote_count,
            bundle_row_count=bundle_count,
            exception_count=exception_count,
            final_row_count=final_count,
        )
    except (
        AdjudicationError,
        historical.ExceptionContractError,
        OSError,
        ValueError,
    ) as exc:
        if isinstance(exc, AdjudicationError):
            code, detail = exc.code, exc.detail
        elif isinstance(exc, historical.ExceptionContractError):
            code, detail = "g06.historical_exception", str(exc)
        elif isinstance(exc, OSError):
            code, detail = "g06.io_error", "G06 evidence I/O failed"
        else:
            code, detail = "g06.contract_error", str(exc)
        return result_document(
            status="BLOCKED",
            violations=[
                {
                    "violation_code": code,
                    "entity_key": "*",
                    "detail": detail,
                }
            ],
            hashes=hashes,
            remote_row_count=remote_count,
            bundle_row_count=bundle_count,
            exception_count=exception_count,
            final_row_count=final_count,
        )


def atomic_write(path: pathlib.Path, payload: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    if path.exists():
        if path.is_file() and not path.is_symlink() and path.read_bytes() == payload:
            return
        raise FileExistsError(f"refusing to overwrite different output: {path}")
    temporary: pathlib.Path | None = None
    try:
        with tempfile.NamedTemporaryFile(
            dir=path.parent,
            prefix=path.name + ".",
            suffix=".tmp",
            delete=False,
        ) as handle:
            temporary = pathlib.Path(handle.name)
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
        temporary = None
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def atomic_write_ordered(outputs: list[tuple[pathlib.Path, bytes]]) -> None:
    """Stage every output, then publish in order with the verdict last.

    Publishing the rich ledger before the standard verdict ensures a PASS
    verdict is never introduced by this command without its hash-bound ledger.
    An already-published final output with a missing prerequisite is rejected
    instead of silently repairing an unverifiable publication order.
    """
    if not outputs:
        raise ValueError("at least one output is required")
    resolved = [path.resolve() for path, _payload in outputs]
    if len(set(resolved)) != len(resolved):
        raise ValueError("ordered outputs must be distinct")

    missing: list[bool] = []
    for path, payload in outputs:
        path.parent.mkdir(parents=True, exist_ok=True)
        if not path.exists():
            missing.append(True)
            continue
        if (
            not path.is_file()
            or path.is_symlink()
            or path.read_bytes() != payload
        ):
            raise FileExistsError(
                f"refusing to overwrite different output: {path}"
            )
        missing.append(False)
    if not missing[-1] and any(missing[:-1]):
        raise FileExistsError(
            "refusing to publish missing ledger after the final verdict exists"
        )

    staged: list[tuple[pathlib.Path, pathlib.Path]] = []
    try:
        for (path, payload), is_missing in zip(outputs, missing):
            if not is_missing:
                continue
            with tempfile.NamedTemporaryFile(
                dir=path.parent,
                prefix=path.name + ".",
                suffix=".tmp",
                delete=False,
            ) as handle:
                temporary = pathlib.Path(handle.name)
                handle.write(payload)
                handle.flush()
                os.fsync(handle.fileno())
            staged.append((temporary, path))
        for temporary, path in staged:
            os.replace(temporary, path)
    finally:
        for temporary, _path in staged:
            temporary.unlink(missing_ok=True)


def extract_bundles(
    final_manifest_path: pathlib.Path,
    expected_final_manifest_sha256: str,
) -> bytes:
    require_sha(expected_final_manifest_sha256, "expected final manifest SHA-256")
    rows, manifest_sha = read_jsonl(
        final_manifest_path, "final manifest", final_manifest=True
    )
    if manifest_sha != expected_final_manifest_sha256:
        raise AdjudicationError(
            "g06.final_manifest_hash",
            "final manifest SHA-256 differs from the finalized boundary",
        )
    _remote, bundles, _exception = split_final_rows(rows)
    return canonical_jsonl(bundles)


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    extract = subparsers.add_parser("extract-bundles")
    extract.add_argument("--final-manifest", type=pathlib.Path, required=True)
    extract.add_argument("--expected-final-manifest-sha256", required=True)
    extract.add_argument("--output", type=pathlib.Path, required=True)

    decide = subparsers.add_parser("adjudicate")
    decide.add_argument("--mapping", type=pathlib.Path, required=True)
    decide.add_argument("--expected-mapping-sha256", required=True)
    decide.add_argument("--hydration-input", type=pathlib.Path, required=True)
    decide.add_argument("--expected-hydration-input-sha256", required=True)
    decide.add_argument("--hydration-evidence", type=pathlib.Path, required=True)
    decide.add_argument("--final-manifest", type=pathlib.Path, required=True)
    decide.add_argument("--expected-final-manifest-sha256", required=True)
    decide.add_argument("--bundle-verdict", type=pathlib.Path, required=True)
    decide.add_argument(
        "--historical-unavailable-exception",
        type=pathlib.Path,
        required=True,
    )
    decide.add_argument(
        "--historical-sql-evidence", type=pathlib.Path, required=True
    )
    decide.add_argument(
        "--historical-api-evidence", type=pathlib.Path, required=True
    )
    decide.add_argument("--output", type=pathlib.Path, required=True)
    decide.add_argument(
        "--ledger-output",
        type=pathlib.Path,
        required=True,
        help="rich hash/count composition evidence; --output is the standard object verdict",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    if args.command == "extract-bundles":
        inputs = {args.final_manifest.resolve()}
        if args.output.resolve() in inputs:
            raise ValueError("output must not overwrite the final manifest")
        payload = extract_bundles(
            args.final_manifest, args.expected_final_manifest_sha256
        )
        atomic_write(args.output, payload)
        return 0
    inputs = {
        args.mapping.resolve(),
        args.hydration_input.resolve(),
        args.hydration_evidence.resolve(),
        args.final_manifest.resolve(),
        args.bundle_verdict.resolve(),
        args.historical_unavailable_exception.resolve(),
        args.historical_sql_evidence.resolve(),
        args.historical_api_evidence.resolve(),
    }
    outputs = {args.output.resolve(), args.ledger_output.resolve()}
    if len(outputs) != 2 or outputs.intersection(inputs):
        raise ValueError("outputs must be distinct and must not overwrite inputs")
    composition = adjudicate(
        mapping_path=args.mapping,
        expected_mapping_sha256=args.expected_mapping_sha256,
        hydration_input_path=args.hydration_input,
        expected_hydration_input_sha256=args.expected_hydration_input_sha256,
        hydration_evidence_path=args.hydration_evidence,
        final_manifest_path=args.final_manifest,
        expected_final_manifest_sha256=args.expected_final_manifest_sha256,
        bundle_verdict_path=args.bundle_verdict,
        exception_path=args.historical_unavailable_exception,
        sql_path=args.historical_sql_evidence,
        api_path=args.historical_api_evidence,
    )
    object_verdict = object_verdict_from_composition(
        composition, args.historical_unavailable_exception
    )
    ledger = bind_standard_verdict(composition, object_verdict)
    atomic_write_ordered(
        [
            (
                args.ledger_output,
                (canonical_json(ledger) + "\n").encode("utf-8"),
            ),
            (
                args.output,
                (canonical_json(object_verdict) + "\n").encode("utf-8"),
            ),
        ]
    )
    return 0 if object_verdict["status"] == "PASS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
