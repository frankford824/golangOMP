#!/usr/bin/env python3
"""Build and validate the single task-2199 historical-unavailable exception.

The exception is deliberately separate from the object manifest.  It binds an
exact final-reviewed mapping row, the exact object-manifest row, read-only SQL
evidence that the asset is not referenced by a current working/finalized
revision, and a controlled API response proving HTTP 410.  No other task,
asset, status, or HTTP response is eligible.
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


SCHEMA_VERSION = 1
TASK_ID = 2199
TASK_ASSET_ID = 12323
ENTITY_KEY = "task_asset:12323"
STRATEGY = "historical_unavailable_tombstone_v1"
POLICY_ID = "legacy_historical_asset_unavailable_v1"
EXPECTED_HTTP_STATUS = 410
ZERO_TIME = "0001-01-01T00:00:00Z"
SHA256 = re.compile(r"^[0-9a-f]{64}$")
CLONE_B_DATABASE = re.compile(r"^ab_[A-Za-z0-9_]*_b(?:_|$)[A-Za-z0-9_]*$")

SQL_FIELDS = {
    "schema_version",
    "status",
    "mapping_sha256",
    "mapping_row_hash",
    "database",
    "transaction",
    "task_id",
    "missing_task_asset_id",
    "working_reference_count",
    "finalized_reference_count",
    "query_sha256",
    "evidence_hash",
}
API_FIELDS = {
    "schema_version",
    "status",
    "mapping_sha256",
    "mapping_row_hash",
    "task_id",
    "task_asset_id",
    "method",
    "request_path",
    "http_status",
    "error_code",
    "evidence_hash",
}
EXCEPTION_FIELDS = {
    "entity_key",
    "owner_kind",
    "owner_id",
    "task_id",
    "missing_task_asset_id",
    "strategy",
    "policy_id",
    "expected_http_status",
    "storage_ref_id",
    "object_row_sha256",
    "mapping_row_hash",
    "working_reference_count",
    "finalized_reference_count",
}
ATTESTATION_FIELDS = {
    "schema_version",
    "status",
    "exception_count",
    "mapping_sha256",
    "mapping_row_hash",
    "object_manifest_sha256",
    "sql_evidence_sha256",
    "api_evidence_sha256",
    "exceptions",
    "evidence_hash",
}


class ExceptionContractError(ValueError):
    pass


def canonical_json(value: Any) -> str:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    )


def canonical_hash(value: Any) -> str:
    return hashlib.sha256(canonical_json(value).encode("utf-8")).hexdigest()


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def self_hash(value: dict[str, Any]) -> str:
    return canonical_hash(
        {key: item for key, item in value.items() if key != "evidence_hash"}
    )


def read_object(path: pathlib.Path, label: str) -> dict[str, Any]:
    if path.is_symlink() or not path.is_file():
        raise ExceptionContractError(f"{label} must be an existing non-symlink file")
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ExceptionContractError(f"{label} must contain valid UTF-8 JSON") from exc
    if not isinstance(value, dict):
        raise ExceptionContractError(f"{label} must contain a JSON object")
    return value


def require_sha(value: Any, label: str) -> str:
    if not isinstance(value, str) or not SHA256.fullmatch(value):
        raise ExceptionContractError(f"{label} must be lowercase SHA-256")
    return value


def validate_mapping(path: pathlib.Path) -> tuple[str, str, dict[str, Any]]:
    mapping = read_object(path, "mapping")
    if mapping.get("version") != 2:
        raise ExceptionContractError("mapping.version must be 2")
    rows = mapping.get("asset_recoveries")
    if not isinstance(rows, list):
        raise ExceptionContractError("mapping.asset_recoveries must be an array")
    matches = [
        row
        for row in rows
        if isinstance(row, dict)
        and row.get("task_id") == TASK_ID
        and row.get("missing_task_asset_id") == TASK_ASSET_ID
    ]
    if len(matches) != 1:
        raise ExceptionContractError("mapping must contain exactly one task 2199 asset 12323 row")
    row = matches[0]
    if (
        row.get("strategy") != STRATEGY
        or row.get("review_policy_ids") != [POLICY_ID]
        or row.get("confidence") != "confirmed_auto"
        or isinstance(row.get("confirmed_by"), bool)
        or not isinstance(row.get("confirmed_by"), int)
        or row["confirmed_by"] <= 0
        or row.get("confirmed_at") in {"", None, ZERO_TIME}
        or not str(row.get("confirmation_note") or "").strip()
        or row.get("recovery_source_task_asset_id") != 0
        or row.get("blockers")
        or not str(row.get("original_storage_ref_id") or "").strip()
    ):
        raise ExceptionContractError("historical-unavailable mapping row is not final-reviewed")
    row_hash = require_sha(row.get("manifest_row_hash"), "mapping row hash")
    expected = canonical_hash(
        {key: value for key, value in row.items() if key != "manifest_row_hash"}
    )
    if row_hash != expected:
        raise ExceptionContractError("historical-unavailable mapping row hash is stale")
    return sha256_file(path), row_hash, row


def read_manifest_row(
    path: pathlib.Path, storage_ref_id: str
) -> tuple[str, str, dict[str, Any]]:
    try:
        from scripts.ab import object_manifest_verifier as object_verifier
    except ModuleNotFoundError:
        import object_manifest_verifier as object_verifier

    if path.is_symlink() or not path.is_file():
        raise ExceptionContractError("object manifest must be an existing non-symlink file")
    matches: list[dict[str, Any]] = []
    try:
        with path.open("r", encoding="utf-8") as handle:
            for line_no, line in enumerate(handle, 1):
                if not line.strip():
                    continue
                try:
                    row = json.loads(line)
                except json.JSONDecodeError as exc:
                    raise ExceptionContractError(
                        f"object manifest row {line_no} is invalid JSON"
                    ) from exc
                if isinstance(row, dict) and row.get("entity_key") == ENTITY_KEY:
                    matches.append(row)
    except UnicodeDecodeError as exc:
        raise ExceptionContractError("object manifest is not UTF-8") from exc
    if len(matches) != 1:
        raise ExceptionContractError("object manifest must contain exactly task_asset:12323")
    row = matches[0]
    problems = object_verifier.validate_contract(row, 1)
    if problems:
        raise ExceptionContractError("task_asset:12323 violates the object manifest contract")
    if (
        row.get("owner_kind") != "task_asset"
        or row.get("owner_id") != TASK_ASSET_ID
        or row.get("task_id") != TASK_ID
        or row.get("storage_ref_id") != storage_ref_id
    ):
        raise ExceptionContractError("task_asset:12323 object identity differs from mapping")
    return sha256_file(path), canonical_hash(row), row


def validate_bound_evidence(
    value: dict[str, Any],
    *,
    expected_fields: set[str],
    label: str,
    mapping_sha: str,
    row_hash: str,
) -> None:
    if set(value) != expected_fields:
        raise ExceptionContractError(f"{label} field contract differs")
    if value.get("evidence_hash") != self_hash(value):
        raise ExceptionContractError(f"{label} self-hash differs")
    if (
        value.get("schema_version") != 1
        or value.get("status") != "PASS"
        or value.get("mapping_sha256") != mapping_sha
        or value.get("mapping_row_hash") != row_hash
        or value.get("task_id") != TASK_ID
    ):
        raise ExceptionContractError(f"{label} mapping/task binding differs")


def validate_sql_evidence(
    value: dict[str, Any], mapping_sha: str, row_hash: str
) -> None:
    validate_bound_evidence(
        value,
        expected_fields=SQL_FIELDS,
        label="SQL evidence",
        mapping_sha=mapping_sha,
        row_hash=row_hash,
    )
    if (
        value.get("missing_task_asset_id") != TASK_ASSET_ID
        or value.get("working_reference_count") != 0
        or value.get("finalized_reference_count") != 0
        or value.get("transaction") != "consistent_read_only"
        or not CLONE_B_DATABASE.fullmatch(str(value.get("database") or ""))
    ):
        raise ExceptionContractError("SQL evidence does not prove zero current references")
    require_sha(value.get("query_sha256"), "SQL evidence query_sha256")


def validate_api_evidence(
    value: dict[str, Any], mapping_sha: str, row_hash: str
) -> None:
    validate_bound_evidence(
        value,
        expected_fields=API_FIELDS,
        label="API evidence",
        mapping_sha=mapping_sha,
        row_hash=row_hash,
    )
    if (
        value.get("task_asset_id") != TASK_ASSET_ID
        or value.get("method") != "GET"
        or value.get("request_path")
        not in {
            "/v1/task-assets/12323/preview",
            "/v1/task-assets/12323/download",
        }
        or value.get("http_status") != EXPECTED_HTTP_STATUS
        or value.get("error_code") != "asset_historically_unavailable"
    ):
        raise ExceptionContractError("API evidence is not the exact task_asset 12323 HTTP 410")


def build(
    mapping_path: pathlib.Path,
    object_manifest_path: pathlib.Path,
    sql_evidence_path: pathlib.Path,
    api_evidence_path: pathlib.Path,
) -> dict[str, Any]:
    mapping_sha, row_hash, mapping_row = validate_mapping(mapping_path)
    manifest_sha, object_row_hash, _object_row = read_manifest_row(
        object_manifest_path, str(mapping_row["original_storage_ref_id"])
    )
    sql_evidence = read_object(sql_evidence_path, "SQL evidence")
    api_evidence = read_object(api_evidence_path, "API evidence")
    validate_sql_evidence(sql_evidence, mapping_sha, row_hash)
    validate_api_evidence(api_evidence, mapping_sha, row_hash)
    exception = {
        "entity_key": ENTITY_KEY,
        "owner_kind": "task_asset",
        "owner_id": TASK_ASSET_ID,
        "task_id": TASK_ID,
        "missing_task_asset_id": TASK_ASSET_ID,
        "strategy": STRATEGY,
        "policy_id": POLICY_ID,
        "expected_http_status": EXPECTED_HTTP_STATUS,
        "storage_ref_id": mapping_row["original_storage_ref_id"],
        "object_row_sha256": object_row_hash,
        "mapping_row_hash": row_hash,
        "working_reference_count": 0,
        "finalized_reference_count": 0,
    }
    result = {
        "schema_version": SCHEMA_VERSION,
        "status": "PASS",
        "exception_count": 1,
        "mapping_sha256": mapping_sha,
        "mapping_row_hash": row_hash,
        "object_manifest_sha256": manifest_sha,
        "sql_evidence_sha256": sha256_file(sql_evidence_path),
        "api_evidence_sha256": sha256_file(api_evidence_path),
        "exceptions": [exception],
    }
    result["evidence_hash"] = self_hash(result)
    return result


def validate_attestation(
    value: dict[str, Any],
    *,
    manifest_path: pathlib.Path | None = None,
) -> dict[str, Any]:
    if set(value) != ATTESTATION_FIELDS:
        raise ExceptionContractError("historical-unavailable attestation field contract differs")
    if (
        value.get("schema_version") != SCHEMA_VERSION
        or value.get("status") != "PASS"
        or value.get("exception_count") != 1
        or value.get("evidence_hash") != self_hash(value)
    ):
        raise ExceptionContractError("historical-unavailable attestation is not a valid PASS")
    for field in (
        "mapping_sha256",
        "mapping_row_hash",
        "object_manifest_sha256",
        "sql_evidence_sha256",
        "api_evidence_sha256",
    ):
        require_sha(value.get(field), f"attestation.{field}")
    exceptions = value.get("exceptions")
    if not isinstance(exceptions, list) or len(exceptions) != 1:
        raise ExceptionContractError("attestation must contain exactly one exception")
    exception = exceptions[0]
    if not isinstance(exception, dict) or set(exception) != EXCEPTION_FIELDS:
        raise ExceptionContractError("attestation exception field contract differs")
    if (
        exception.get("entity_key") != ENTITY_KEY
        or exception.get("owner_kind") != "task_asset"
        or exception.get("owner_id") != TASK_ASSET_ID
        or exception.get("task_id") != TASK_ID
        or exception.get("missing_task_asset_id") != TASK_ASSET_ID
        or exception.get("strategy") != STRATEGY
        or exception.get("policy_id") != POLICY_ID
        or exception.get("expected_http_status") != EXPECTED_HTTP_STATUS
        or exception.get("mapping_row_hash") != value["mapping_row_hash"]
        or exception.get("working_reference_count") != 0
        or exception.get("finalized_reference_count") != 0
        or not str(exception.get("storage_ref_id") or "").strip()
        or not SHA256.fullmatch(str(exception.get("object_row_sha256") or ""))
    ):
        raise ExceptionContractError("attestation exception identity differs")
    if manifest_path is not None:
        if sha256_file(manifest_path) != value["object_manifest_sha256"]:
            raise ExceptionContractError("attestation is not bound to the object manifest")
        _manifest_sha, object_row_hash, row = read_manifest_row(
            manifest_path, str(exception["storage_ref_id"])
        )
        if (
            object_row_hash != exception["object_row_sha256"]
            or canonical_hash(row) != exception["object_row_sha256"]
        ):
            raise ExceptionContractError("attested object row hash differs")
    return exception


def load_attestation(
    path: pathlib.Path, *, manifest_path: pathlib.Path | None = None
) -> tuple[dict[str, Any], dict[str, Any], str]:
    value = read_object(path, "historical-unavailable attestation")
    exception = validate_attestation(value, manifest_path=manifest_path)
    return value, exception, sha256_file(path)


def atomic_write(path: pathlib.Path, value: dict[str, Any]) -> None:
    if path.exists():
        raise FileExistsError(f"refusing to overwrite artifact: {path}")
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = (canonical_json(value) + "\n").encode("utf-8")
    with tempfile.NamedTemporaryFile(
        dir=path.parent, prefix=path.name + ".", suffix=".tmp", delete=False
    ) as handle:
        temporary = pathlib.Path(handle.name)
        handle.write(payload)
        handle.flush()
        os.fsync(handle.fileno())
    try:
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--mapping", type=pathlib.Path, required=True)
    parser.add_argument("--object-manifest", type=pathlib.Path, required=True)
    parser.add_argument("--sql-evidence", type=pathlib.Path, required=True)
    parser.add_argument("--api-evidence", type=pathlib.Path, required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    inputs = {
        args.mapping.resolve(),
        args.object_manifest.resolve(),
        args.sql_evidence.resolve(),
        args.api_evidence.resolve(),
    }
    if args.output.resolve() in inputs:
        raise ExceptionContractError("output must not overwrite an input")
    atomic_write(
        args.output,
        build(
            args.mapping,
            args.object_manifest,
            args.sql_evidence,
            args.api_evidence,
        ),
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
