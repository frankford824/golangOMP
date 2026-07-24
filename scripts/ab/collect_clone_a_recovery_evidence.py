#!/usr/bin/env python3
"""Collect task 2807 recovery-before evidence from a loopback Clone A.

The collector performs one consistent read-only transaction through a MySQL
client, validates the exact frozen recovery allowlist and controlled-read
receipts, and emits the envelope consumed by ``prepare_asset_recovery.py``.
It never prints or persists DSN credentials.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import re
import subprocess
import tempfile
from typing import Any


SHA256 = re.compile(r"^[0-9a-f]{64}$")
RUN_ID = re.compile(r"^[a-z0-9][a-z0-9_-]{2,63}$")
DSN = re.compile(
    r"^(?P<user>[^:@/\s]+):(?P<password>.*)@tcp\("
    r"(?P<host>127\.0\.0\.1|localhost):(?P<port>[0-9]{1,5})\)"
    r"/(?P<database>[A-Za-z0-9_]+)(?:\?.*)?$"
)
CLONE_A_DB = re.compile(r"^ab_[A-Za-z0-9_]*_a(?:_|$)[A-Za-z0-9_]*$")
POLICY = "legacy_deleted_asset_recovery_v1"
STRATEGY = "clone_b_prematerialized_storage_ref_v1"
SOURCE_TASK_ID = 2098
ALLOWLIST = {
    23989: (2807, 24034, 683001),
    23990: (2807, 24033, 689291),
    23991: (2807, 24040, 686447),
}
TASK_ASSET_IDS = tuple(
    sorted(set(ALLOWLIST) | {value[1] for value in ALLOWLIST.values()})
)
def canonical_bytes(value: Any) -> bytes:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def load_object(path: pathlib.Path, label: str) -> dict[str, Any]:
    if path.is_symlink() or not path.is_file():
        raise ValueError(f"{label} must be an existing non-symlink file")
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{label} must be a JSON object")
    return value


def validate_mapping(path: pathlib.Path) -> tuple[dict[int, dict[str, Any]], str]:
    mapping = load_object(path, "mapping")
    if mapping.get("version") != 2:
        raise ValueError("mapping.version must be 2")
    rows = mapping.get("asset_recoveries")
    if not isinstance(rows, list):
        raise ValueError("mapping.asset_recoveries must be an array")
    by_missing: dict[int, dict[str, Any]] = {}
    for row in rows:
        if not isinstance(row, dict):
            continue
        missing_id = row.get("missing_task_asset_id")
        if missing_id not in ALLOWLIST:
            continue
        task_id, source_id, size = ALLOWLIST[missing_id]
        if (
            missing_id in by_missing
            or row.get("task_id") != task_id
            or row.get("recovery_source_task_asset_id") != source_id
            or row.get("expected_file_size") != size
            or row.get("strategy") != STRATEGY
            or row.get("confidence") != "confirmed_auto"
            or row.get("review_policy_ids") != [POLICY]
            or not isinstance(row.get("confirmed_by"), int)
            or isinstance(row.get("confirmed_by"), bool)
            or row["confirmed_by"] <= 0
            or not str(row.get("confirmed_at") or "").strip()
            or not str(row.get("confirmation_note") or "").strip()
            or row.get("blockers")
            or not SHA256.fullmatch(str(row.get("preview_whole_hash") or ""))
            or not SHA256.fullmatch(str(row.get("design_thumb_whole_hash") or ""))
            or not str(row.get("original_storage_ref_id") or "").strip()
            or not str(row.get("recovery_source_storage_ref_id") or "").strip()
        ):
            raise ValueError(f"mapping recovery {missing_id} drifted from allowlist")
        manifest_hash = str(row.get("manifest_row_hash") or "")
        unhashed = dict(row)
        unhashed["manifest_row_hash"] = ""
        if (
            not SHA256.fullmatch(manifest_hash)
            or hashlib.sha256(canonical_bytes(unhashed)).hexdigest()
            != manifest_hash
        ):
            raise ValueError(f"mapping recovery {missing_id} row hash is invalid")
        by_missing[missing_id] = row
    if set(by_missing) != set(ALLOWLIST):
        raise ValueError("mapping must contain exactly all three frozen recoveries")
    return by_missing, sha256_file(path)


def validate_receipts(
    path: pathlib.Path, mapping: dict[int, dict[str, Any]]
) -> dict[int, dict[str, Any]]:
    evidence = load_object(path, "controlled-read receipts")
    unsigned = dict(evidence)
    evidence_sha = str(unsigned.pop("evidence_sha256", ""))
    if (
        evidence.get("version") != 1
        or evidence.get("status") != "PASS"
        or evidence.get("protocol") != "controlled-asset-read-v1"
        or evidence.get("production_writes_executed") is not False
        or evidence.get("database_connections_opened") is not False
        or evidence.get("remote_operation") != "GET"
        or not SHA256.fullmatch(evidence_sha)
        or hashlib.sha256(canonical_bytes(unsigned)).hexdigest() != evidence_sha
    ):
        raise ValueError("controlled-read receipt envelope is invalid")
    rows = evidence.get("recoveries")
    if not isinstance(rows, list):
        raise ValueError("controlled-read recoveries must be an array")
    result: dict[int, dict[str, Any]] = {}
    for row in rows:
        if not isinstance(row, dict):
            raise ValueError("controlled-read recovery row is invalid")
        missing_id = row.get("missing_task_asset_id")
        if missing_id not in mapping or missing_id in result:
            raise ValueError("controlled-read receipt scope is unexpected or duplicate")
        expected_source = ALLOWLIST[missing_id][1]
        expected_size = ALLOWLIST[missing_id][2]
        receipt = row.get("source_fetch_receipt")
        local_path = pathlib.Path(str(row.get("source_local_path") or ""))
        if (
            not isinstance(receipt, dict)
            or not local_path.is_absolute()
            or not local_path.is_file()
            or local_path.is_symlink()
        ):
            raise ValueError(f"controlled-read source {missing_id} is unavailable")
        digest = sha256_file(local_path)
        if (
            row.get("task_asset_id") != expected_source
            or row.get("source_sha256") != digest
            or receipt.get("protocol") != "controlled-asset-read-v1"
            or receipt.get("task_asset_id") != expected_source
            or receipt.get("storage_ref_id")
            != mapping[missing_id]["recovery_source_storage_ref_id"]
            or receipt.get("size") != expected_size
            or receipt.get("sha256") != digest
            or local_path.stat().st_size != expected_size
            or not str(receipt.get("object_key") or "").strip()
            or not str(receipt.get("fetched_at") or "").strip()
        ):
            raise ValueError(f"controlled-read source {missing_id} drifted")
        result[missing_id] = row
    if set(result) != set(ALLOWLIST):
        raise ValueError("controlled-read receipt must cover the exact allowlist")
    return result


def parse_dsn(path: pathlib.Path, expected_database: str) -> dict[str, Any]:
    if path.is_symlink() or not path.is_file():
        raise ValueError("Clone A DSN file must be an existing non-symlink file")
    match = DSN.fullmatch(path.read_text(encoding="utf-8").strip())
    if not match:
        raise ValueError("Clone A DSN must use loopback tcp host and explicit port")
    port = int(match.group("port"))
    database = match.group("database")
    if not 1 <= port <= 65535:
        raise ValueError("Clone A DSN port is invalid")
    if database != expected_database or not CLONE_A_DB.fullmatch(database):
        raise ValueError("DSN database is not the confirmed Clone A database")
    return {
        "user": match.group("user"),
        "password": match.group("password"),
        "host": match.group("host"),
        "port": port,
        "database": database,
    }


def query_sql() -> str:
    missing_ids = ",".join(str(value) for value in sorted(ALLOWLIST))
    all_ids = ",".join(str(value) for value in TASK_ASSET_IDS)
    return f"""
SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci;
SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
SET SESSION TRANSACTION READ ONLY;
START TRANSACTION WITH CONSISTENT SNAPSHOT, READ ONLY;
SELECT JSON_OBJECT(
  'kind','database','database',DATABASE(),
  'transaction_read_only',@@session.transaction_read_only
);
SELECT JSON_OBJECT(
  'kind','task_asset','id',ta.id,'task_id',ta.task_id,'asset_id',ta.asset_id,
  'upload_request_id',ta.upload_request_id,'storage_ref_id',ta.storage_ref_id,
  'storage_key',ta.storage_key,'whole_hash',ta.whole_hash,
  'upload_status',ta.upload_status,'deleted_at',ta.deleted_at,
  'cleaned_at',ta.cleaned_at,'object_deleted_at',ta.object_deleted_at,
  'access_revoked_at',ta.access_revoked_at,
  'access_revoked_reason',ta.access_revoked_reason,'file_size',ta.file_size,
  'file_name',ta.file_name,'mime_type',ta.mime_type,'asset_type',ta.asset_type,
  'source_asset_version_id',ta.source_asset_version_id
) FROM task_assets ta WHERE ta.id IN ({all_ids}) ORDER BY ta.id;
SELECT JSON_OBJECT(
  'kind','upload_request','missing_task_asset_id',ta.id,
  'request_id',ur.request_id,'bound_ref_id',ur.bound_ref_id,
  'checksum_hint',ur.checksum_hint,'file_size',ur.file_size,
  'status',ur.status,'session_status',ur.session_status
) FROM task_assets ta JOIN upload_requests ur ON ur.request_id=ta.upload_request_id
  WHERE ta.id IN ({missing_ids}) ORDER BY ta.id;
SELECT JSON_OBJECT(
  'kind','storage_ref','missing_task_asset_id',ta.id,'ref_id',sr.ref_id,
  'asset_id',sr.asset_id,'owner_type',sr.owner_type,'owner_id',sr.owner_id,
  'upload_request_id',sr.upload_request_id,'storage_adapter',sr.storage_adapter,
  'ref_type',sr.ref_type,'ref_key',sr.ref_key,'file_name',sr.file_name,
  'mime_type',sr.mime_type,'file_size',sr.file_size,
  'is_placeholder',sr.is_placeholder,'checksum_hint',sr.checksum_hint,
  'status',sr.status
) FROM task_assets ta JOIN asset_storage_refs sr ON sr.ref_id=ta.storage_ref_id
  WHERE ta.id IN ({missing_ids}) ORDER BY ta.id;
SELECT JSON_OBJECT(
  'kind','derivative','id',d.id,'asset_type',d.asset_type,
  'source_asset_version_id',d.source_asset_version_id,'whole_hash',d.whole_hash
) FROM task_assets d
  WHERE d.source_asset_version_id IN ({all_ids})
    AND d.asset_type IN ('preview','design_thumb')
  ORDER BY d.source_asset_version_id,d.asset_type,d.id;
COMMIT;
""".strip() + "\n"


def query_clone(
    mysql_bin: str, connection: dict[str, Any], timeout_seconds: int
) -> list[dict[str, Any]]:
    env = dict(os.environ)
    env["MYSQL_PWD"] = connection["password"]
    command = [
        mysql_bin,
        "--batch",
        "--raw",
        "--skip-column-names",
        "--binary-mode",
        "--protocol=TCP",
        "--host",
        connection["host"],
        "--port",
        str(connection["port"]),
        "--user",
        connection["user"],
        "--database",
        connection["database"],
    ]
    try:
        result = subprocess.run(
            command,
            input=query_sql(),
            text=True,
            encoding="utf-8",
            capture_output=True,
            timeout=timeout_seconds,
            check=False,
            env=env,
        )
    except subprocess.TimeoutExpired as exc:
        raise ValueError("Clone A read-only query timed out") from exc
    if result.returncode != 0:
        raise ValueError(
            f"Clone A read-only query failed exit={result.returncode}"
        )
    rows = []
    for line_number, line in enumerate(result.stdout.splitlines(), 1):
        if not line.strip():
            continue
        try:
            value = json.loads(line)
        except json.JSONDecodeError as exc:
            raise ValueError(
                f"Clone A query row {line_number} is not JSON"
            ) from exc
        if not isinstance(value, dict):
            raise ValueError(f"Clone A query row {line_number} is not an object")
        rows.append(value)
    return rows


def validate_rows(
    rows: list[dict[str, Any]],
    database: str,
    mapping: dict[int, dict[str, Any]],
    receipts: dict[int, dict[str, Any]],
) -> list[dict[str, Any]]:
    database_rows = [row for row in rows if row.get("kind") == "database"]
    if database_rows != [
        {
            "kind": "database",
            "database": database,
            "transaction_read_only": 1,
        }
    ]:
        raise ValueError("Clone A query did not prove the exact database")
    assets = {
        row.get("id"): {key: value for key, value in row.items() if key != "kind"}
        for row in rows
        if row.get("kind") == "task_asset"
    }
    if set(assets) != set(TASK_ASSET_IDS):
        raise ValueError("Clone A task_asset evidence coverage drifted")
    upload_requests = {
        row.get("missing_task_asset_id"): {
            key: value
            for key, value in row.items()
            if key not in {"kind", "missing_task_asset_id"}
        }
        for row in rows
        if row.get("kind") == "upload_request"
    }
    storage_refs = {
        row.get("missing_task_asset_id"): {
            key: value
            for key, value in row.items()
            if key not in {"kind", "missing_task_asset_id"}
        }
        for row in rows
        if row.get("kind") == "storage_ref"
    }
    if set(upload_requests) != set(ALLOWLIST) or set(storage_refs) != set(ALLOWLIST):
        raise ValueError("Clone A upload/storage evidence coverage drifted")
    derivatives: dict[int, list[dict[str, Any]]] = {}
    for row in rows:
        if row.get("kind") != "derivative":
            continue
        source_id = row.get("source_asset_version_id")
        derivative = {
            "asset_type": row.get("asset_type"),
            "source_asset_version_id": source_id,
            "whole_hash": row.get("whole_hash"),
        }
        derivatives.setdefault(source_id, []).append(derivative)

    output = []
    for missing_id in sorted(ALLOWLIST):
        task_id, source_id, size = ALLOWLIST[missing_id]
        missing = assets[missing_id]
        source = assets[source_id]
        mapping_row = mapping[missing_id]
        receipt_row = receipts[missing_id]
        if (
            missing.get("id") != missing_id
            or missing.get("task_id") != task_id
            or missing.get("file_size") != size
            or missing.get("storage_ref_id")
            != mapping_row["original_storage_ref_id"]
            or source.get("id") != source_id
            or source.get("task_id") != SOURCE_TASK_ID
            or source.get("asset_type") != "delivery"
            or source.get("file_size") != size
            or source.get("storage_ref_id")
            != mapping_row["recovery_source_storage_ref_id"]
            or source.get("storage_key")
            != receipt_row["source_fetch_receipt"]["object_key"]
            or source.get("upload_status") != "uploaded"
            or source.get("deleted_at") is not None
            or source.get("object_deleted_at") is not None
            or upload_requests[missing_id].get("request_id")
            != missing.get("upload_request_id")
            or storage_refs[missing_id].get("ref_id")
            != mapping_row["original_storage_ref_id"]
        ):
            raise ValueError(f"Clone A recovery identity {missing_id} drifted")
        expected_derivatives = {
            "preview": mapping_row["preview_whole_hash"],
            "design_thumb": mapping_row["design_thumb_whole_hash"],
        }
        for asset_id in (missing_id, source_id):
            actual = derivatives.get(asset_id, [])
            if (
                len(actual) != 2
                or {item["asset_type"]: item["whole_hash"] for item in actual}
                != expected_derivatives
            ):
                raise ValueError(
                    f"Clone A derivative lineage {asset_id} drifted"
                )
        output.append(
            {
                "missing_task_asset_id": missing_id,
                "source_local_path": receipt_row["source_local_path"],
                "source_sha256": receipt_row["source_sha256"],
                "missing_task_asset_before": missing,
                "source_task_asset": source,
                "source_fetch_receipt": receipt_row["source_fetch_receipt"],
                "upload_request_before": upload_requests[missing_id],
                "original_storage_ref_before": storage_refs[missing_id],
                "missing_derivatives": derivatives[missing_id],
                "source_derivatives": derivatives[source_id],
            }
        )
    return output


def atomic_write(path: pathlib.Path, value: dict[str, Any]) -> None:
    encoded = canonical_bytes(value) + b"\n"
    if path.exists():
        if path.is_file() and not path.is_symlink() and path.read_bytes() == encoded:
            return
        raise FileExistsError(f"refusing to overwrite evidence: {path}")
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, name = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    temporary = pathlib.Path(name)
    try:
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(encoded)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)


def run(args: argparse.Namespace) -> dict[str, Any]:
    if not RUN_ID.fullmatch(args.run_id):
        raise ValueError("run-id is invalid")
    mapping, mapping_sha = validate_mapping(args.mapping)
    receipts = validate_receipts(args.controlled_read_receipts, mapping)
    connection = parse_dsn(args.clone_a_dsn_file, args.confirm_clone_a_database)
    rows = query_clone(args.mysql_bin, connection, args.timeout_seconds)
    recoveries = validate_rows(
        rows, connection["database"], mapping, receipts
    )
    evidence = {
        "version": 1,
        "run_id": args.run_id,
        "status": "PASS",
        "source": {
            "side": "A",
            "database": connection["database"],
            "host_class": "loopback",
            "transaction": "consistent_read_only",
        },
        "mapping_sha256": mapping_sha,
        "controlled_read_receipts_sha256": sha256_file(
            args.controlled_read_receipts
        ),
        "database_writes_executed": False,
        "production_connections_opened": False,
        "recoveries": recoveries,
    }
    evidence["evidence_sha256"] = hashlib.sha256(
        canonical_bytes(evidence)
    ).hexdigest()
    atomic_write(args.output, evidence)
    return evidence


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--clone-a-dsn-file", type=pathlib.Path, required=True)
    parser.add_argument("--confirm-clone-a-database", required=True)
    parser.add_argument("--mapping", type=pathlib.Path, required=True)
    parser.add_argument(
        "--controlled-read-receipts", type=pathlib.Path, required=True
    )
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--mysql-bin", default="mysql")
    parser.add_argument("--timeout-seconds", type=int, default=120)
    args = parser.parse_args()
    if args.timeout_seconds <= 0:
        parser.error("--timeout-seconds must be positive")
    return args


def main() -> int:
    run(parse_args())
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
