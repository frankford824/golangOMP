#!/usr/bin/env python3
"""Export a deterministic, read-only Frozen-A oracle from MySQL.

The exporter deliberately uses one mysql client process.  All metadata, schema,
and row reads therefore share one READ ONLY, REPEATABLE READ transaction with a
consistent snapshot.  Credentials are neither accepted on the command line nor
written to the evidence manifest; use normal mysql option-file/socket handling.
"""

from __future__ import annotations

import argparse
import dataclasses
import hashlib
import json
import math
import os
import re
import subprocess
import sys
from pathlib import Path
from typing import Any, Callable, Iterable, Mapping, Sequence


SCHEMA_VERSION = 2
_DATABASE_RE = re.compile(r"^[A-Za-z0-9_]+$")
_CONTAINER_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_.-]*$")
_CANONICAL_DATETIME_RE = re.compile(
    r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$"
)
_CANONICAL_DECIMAL_RE = re.compile(r"^-?(?:0|[1-9]\d*)(?:\.\d+)?$")
_INTEGER_TYPE_RE = re.compile(
    r"^(?:tinyint|smallint|mediumint|int|integer|bigint)(?:\(\d+\))?(?: unsigned)?$",
    re.IGNORECASE,
)
_DECIMAL_TYPE_RE = re.compile(
    r"^(?:decimal|numeric)(?:\(\d+(?:,\d+)?\))?(?: unsigned)?$",
    re.IGNORECASE,
)
_FLOAT_TYPE_RE = re.compile(
    r"^(?:float|double|real)(?:\(\d+(?:,\d+)?\))?(?: unsigned)?$",
    re.IGNORECASE,
)
_DATETIME_TYPE_RE = re.compile(
    r"^(?:datetime|timestamp)(?:\(\d+\))?$", re.IGNORECASE
)
_JSON_TYPE_RE = re.compile(r"^json$", re.IGNORECASE)
_TEXT_TYPE_RE = re.compile(
    r"^(?:char|varchar|tinytext|text|mediumtext|longtext|enum|set)"
    r"(?:\([^)]*\))?(?: CHARACTER SET \w+)?(?: COLLATE \w+)?$",
    re.IGNORECASE,
)
_BINARY_TYPE_RE = re.compile(
    r"^(?:binary|varbinary|tinyblob|blob|mediumblob|longblob)(?:\(\d+\))?$",
    re.IGNORECASE,
)


class OracleExportError(RuntimeError):
    """Raised when the snapshot stream violates the frozen-oracle contract."""


@dataclasses.dataclass(frozen=True)
class DatasetSpec:
    name: str
    table: str
    key: str
    columns: tuple[str, ...]
    key_kind: str = "integer"
    excluded_schema_columns: tuple[str, ...] = ()


DATASETS: tuple[DatasetSpec, ...] = (
    DatasetSpec(
        "tasks",
        "tasks",
        "id",
        (
            "id", "task_no", "source_mode", "product_id", "sku_code",
            "product_name_snapshot", "task_type", "operator_group_id",
            "creator_id", "requester_id", "designer_id", "current_handler_id",
            "task_status", "workflow_revision", "priority", "deadline_at",
            "need_outsource", "created_at", "updated_at", "owner_team",
            "is_outsource", "business_lane", "is_batch_task",
            "batch_item_count", "batch_mode", "primary_sku_code",
            "sku_generation_status", "owner_department", "owner_department_id",
            "owner_org_team", "owner_team_id", "customization_required",
            "customization_source_type", "last_customization_operator_id",
            "warehouse_reject_reason", "warehouse_reject_category",
        ),
    ),
    DatasetSpec(
        "roots",
        "design_assets",
        "id",
        (
            "id", "task_id", "asset_no", "source_asset_id", "scope_sku_code",
            "retouch_requirement_id", "asset_type", "current_version_id",
            "created_by", "created_at", "updated_at",
        ),
    ),
    DatasetSpec(
        "task_assets",
        "task_assets",
        "id",
        (
            "id", "task_id", "asset_id", "scope_sku_code",
            "retouch_requirement_id", "asset_type", "binding_state",
            "bound_group_id", "bound_role", "staged_task_sku_item_id",
            "staged_retouch_requirement_id", "staged_role", "staged_by",
            "upload_session_id", "staged_expires_at", "access_revoked_at",
            "access_revoked_reason", "object_deleted_at", "version_no",
            "asset_version_no", "upload_mode", "upload_request_id",
            "storage_ref_id", "file_name", "original_filename",
            "remote_file_id", "mime_type", "file_size", "file_path",
            "storage_key", "whole_hash", "upload_status", "preview_status",
            "uploaded_by", "uploaded_at", "remark", "created_at",
            "source_module_key", "source_task_module_id", "is_archived",
            "archived_at", "archived_by", "flow_review_status", "approved_at",
            "approved_by", "rejected_at", "rejected_by",
            "superseded_by_version_id", "superseded_at", "cleanup_after_at",
            "source_asset_version_id", "cleaned_at", "deleted_at",
        ),
        excluded_schema_columns=("sort_time",),
    ),
    DatasetSpec(
        "objects",
        "asset_storage_refs",
        "ref_id",
        (
            "ref_id", "asset_id", "owner_type", "owner_id",
            "upload_request_id", "storage_adapter", "ref_type", "ref_key",
            "file_name", "mime_type", "file_size", "is_placeholder",
            "checksum_hint", "status", "created_at",
        ),
        key_kind="string",
    ),
    DatasetSpec(
        "skus",
        "task_sku_items",
        "id",
        (
            "id", "task_id", "sequence_no", "sku_code", "sku_status",
            "sku_origin", "product_id", "erp_product_id", "filing_status",
            "erp_sync_status", "erp_sync_required", "erp_sync_version",
            "last_filed_at", "filing_error_message", "product_name_snapshot",
            "product_i_id", "product_short_name", "category_code",
            "material_mode", "cost_price_mode", "quantity", "base_sale_price",
            "cost_price", "estimated_cost", "cost_rule_id", "cost_rule_name",
            "cost_rule_source", "matched_rule_version", "prefill_source",
            "prefill_at", "requires_manual_review", "manual_cost_override",
            "manual_cost_override_reason", "override_actor", "override_at",
            "design_requirement", "variant_json",
            "reference_file_refs_json", "dedupe_key", "sku_code_type",
            "created_at", "updated_at",
        ),
    ),
    DatasetSpec(
        "retouch_requirements",
        "task_retouch_requirements",
        "id",
        (
            "id", "task_id", "description", "sku_code", "spec", "remark",
            "sort_order", "created_by", "updated_by", "created_at",
            "updated_at", "deleted_at",
        ),
    ),
    DatasetSpec(
        "reference_file_refs",
        "reference_file_refs",
        "id",
        (
            "id", "task_id", "sku_item_id", "retouch_requirement_id",
            "ref_id", "owner_module_key", "context", "attached_at",
        ),
    ),
)

_DECIMAL_FIELDS = frozenset({"base_sale_price", "cost_price", "estimated_cost"})
_JSON_TEXT_FIELDS = frozenset({"reference_file_refs_json"})
_NATIVE_JSON_FIELDS = frozenset({"variant_json"})
_MARKER = "__FROZEN_A_ORACLE__"


def canonical_json_bytes(value: Any) -> bytes:
    """Return the only JSON representation accepted for hashes and NDJSON."""
    return json.dumps(
        value,
        ensure_ascii=False,
        sort_keys=True,
        separators=(",", ":"),
        allow_nan=False,
    ).encode("utf-8")


def sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def _quote_identifier(value: str) -> str:
    if not _DATABASE_RE.fullmatch(value):
        raise OracleExportError(f"unsafe MySQL identifier: {value!r}")
    return f"`{value}`"


def _sql_value_expression(column: str) -> str:
    quoted = _quote_identifier(column)
    if column in _DECIMAL_FIELDS:
        return (
            f"CASE WHEN {quoted} IS NULL THEN NULL "
            f"ELSE CAST({quoted} AS CHAR) END"
        )
    if column.endswith("_at") or column == "deadline_at":
        return (
            f"CASE WHEN {quoted} IS NULL THEN NULL ELSE "
            f"DATE_FORMAT({quoted},'%Y-%m-%dT%H:%i:%s.%fZ') END"
        )
    if column in _NATIVE_JSON_FIELDS:
        return (
            f"CASE WHEN {quoted} IS NULL THEN NULL "
            f"ELSE JSON_EXTRACT({quoted},'$') END"
        )
    return quoted


def build_snapshot_sql(database: str) -> str:
    """Build the single-session, read-only snapshot program."""
    db = _quote_identifier(database)
    lines = [
        "SET SESSION time_zone = '+00:00';",
        "SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;",
        "START TRANSACTION READ ONLY, WITH CONSISTENT SNAPSHOT;",
        f"SELECT '{_MARKER}','meta','all';",
        "SELECT HEX(CONVERT(CAST(JSON_OBJECT("
        "'mysql_version',VERSION(),"
        "'session_time_zone',@@session.time_zone,"
        "'system_time_zone',@@system_time_zone,"
        "'server_uuid',@@server_uuid"
        ") AS CHAR) USING utf8mb4));",
    ]
    for spec in DATASETS:
        table = f"{db}.{_quote_identifier(spec.table)}"
        lines.append(f"SELECT '{_MARKER}','schema','{spec.name}';")
        lines.append(f"SHOW COLUMNS FROM {table};")
        lines.append(f"SELECT '{_MARKER}','data','{spec.name}';")
        pairs: list[str] = []
        for column in spec.columns:
            pairs.extend((f"'{column}'", _sql_value_expression(column)))
        payload = "JSON_OBJECT(" + ",".join(pairs) + ")"
        key = _quote_identifier(spec.key)
        lines.append(
            "SELECT "
            f"HEX(CONVERT(CAST({key} AS CHAR) USING utf8mb4)),"
            f"HEX(CONVERT(CAST({payload} AS CHAR) USING utf8mb4)) "
            f"FROM {table} ORDER BY {key};"
        )
    lines.extend(
        (
            "COMMIT;",
            f"SELECT '{_MARKER}','final','all';",
            "",
        )
    )
    return "\n".join(lines)


def _decode_hex(value: str, label: str) -> str:
    try:
        return bytes.fromhex(value).decode("utf-8")
    except (ValueError, UnicodeDecodeError) as exc:
        raise OracleExportError(f"invalid UTF-8 hex for {label}") from exc


def _parse_json_hex(value: str, label: str) -> Any:
    try:
        return json.loads(_decode_hex(value, label))
    except json.JSONDecodeError as exc:
        raise OracleExportError(f"invalid JSON for {label}: {exc}") from exc


def _schema_row(fields: Sequence[str]) -> dict[str, Any]:
    if len(fields) != 6:
        raise OracleExportError(
            f"SHOW COLUMNS row has {len(fields)} fields instead of 6"
        )
    return {
        "field": fields[0],
        "type": fields[1],
        "null": fields[2],
        "key": fields[3],
        "default": None if fields[4] == "NULL" else fields[4],
        "extra": fields[5],
    }


def parse_mysql_output(
    output: str,
) -> tuple[dict[str, Any], dict[str, list[dict[str, Any]]], dict[str, list[tuple[str, Any]]]]:
    """Parse the marker-delimited output of one mysql client invocation."""
    schemas: dict[str, list[dict[str, Any]]] = {spec.name: [] for spec in DATASETS}
    rows: dict[str, list[tuple[str, Any]]] = {spec.name: [] for spec in DATASETS}
    metadata: dict[str, Any] | None = None
    mode: str | None = None
    dataset: str | None = None
    final_seen = False

    for line_number, line in enumerate(output.splitlines(), start=1):
        fields = line.split("\t")
        if fields and fields[0] == _MARKER:
            if len(fields) != 3:
                raise OracleExportError(f"bad marker at output line {line_number}")
            mode, dataset = fields[1], fields[2]
            if mode not in {"meta", "schema", "data", "final"}:
                raise OracleExportError(f"unknown marker mode {mode!r}")
            if mode in {"schema", "data"} and dataset not in schemas:
                raise OracleExportError(f"unknown dataset marker {dataset!r}")
            if mode == "final":
                final_seen = True
            continue

        if final_seen:
            raise OracleExportError("unexpected data after final marker")
        if mode == "meta":
            if metadata is not None or len(fields) != 1:
                raise OracleExportError("invalid metadata row")
            parsed = _parse_json_hex(fields[0], "metadata")
            if not isinstance(parsed, dict):
                raise OracleExportError("metadata must be a JSON object")
            metadata = parsed
        elif mode == "schema" and dataset is not None:
            schemas[dataset].append(_schema_row(fields))
        elif mode == "data" and dataset is not None:
            if len(fields) != 2:
                raise OracleExportError(
                    f"data row for {dataset} has {len(fields)} fields"
                )
            key = _decode_hex(fields[0], f"{dataset} row key")
            payload = _parse_json_hex(fields[1], f"{dataset} row {key}")
            rows[dataset].append((key, payload))
        elif line:
            raise OracleExportError(
                f"unmarked mysql output at line {line_number}"
            )

    if not final_seen:
        raise OracleExportError("mysql output ended before final marker")
    if metadata is None:
        raise OracleExportError("mysql output omitted metadata")
    if metadata.get("session_time_zone") != "+00:00":
        raise OracleExportError("snapshot session time zone is not +00:00")
    return metadata, schemas, rows


def _validate_schema(
    spec: DatasetSpec, schema: Sequence[Mapping[str, Any]]
) -> dict[str, Mapping[str, Any]]:
    fields = [str(item["field"]) for item in schema]
    selected = [
        field for field in fields if field not in spec.excluded_schema_columns
    ]
    if selected != list(spec.columns):
        raise OracleExportError(
            f"{spec.name} column drift: expected {list(spec.columns)!r}, "
            f"got {selected!r}"
        )
    unexpected_excluded = set(fields) - set(spec.columns) - set(
        spec.excluded_schema_columns
    )
    if unexpected_excluded:
        raise OracleExportError(
            f"{spec.name} has unapproved columns: {sorted(unexpected_excluded)!r}"
        )
    schema_by_field = {str(item["field"]): item for item in schema}
    if len(schema_by_field) != len(schema):
        raise OracleExportError(f"{spec.name} schema contains duplicate fields")
    return schema_by_field


def _normalize_json_value(value: Any, label: str) -> Any:
    if value is None or isinstance(value, (str, int, bool)):
        return value
    if isinstance(value, float):
        if not math.isfinite(value):
            raise OracleExportError(f"{label} contains a non-finite number")
        return value
    if isinstance(value, list):
        return [
            _normalize_json_value(item, f"{label}[]")
            for item in value
        ]
    if isinstance(value, dict):
        if not all(isinstance(key, str) for key in value):
            raise OracleExportError(f"{label} has a non-string object key")
        return {
            key: _normalize_json_value(value[key], f"{label}.{key}")
            for key in sorted(value)
        }
    raise OracleExportError(f"{label} has unsupported JSON type {type(value)!r}")


def _normalize_column(
    dataset: str,
    column: str,
    value: Any,
    schema: Mapping[str, Any],
) -> Any:
    label = f"{dataset}.{column}"
    nullable = str(schema["null"]).upper() == "YES"
    if value is None:
        if not nullable:
            raise OracleExportError(f"{label} is NULL but schema is NOT NULL")
        return None

    mysql_type = str(schema["type"]).strip()
    if column in _JSON_TEXT_FIELDS:
        if not isinstance(value, str):
            raise OracleExportError(f"{label} JSON text is not a string")
        try:
            return _normalize_json_value(json.loads(value), label)
        except json.JSONDecodeError as exc:
            raise OracleExportError(f"{label} contains invalid JSON") from exc
    if _JSON_TYPE_RE.fullmatch(mysql_type):
        if isinstance(value, str):
            try:
                value = json.loads(value)
            except json.JSONDecodeError as exc:
                raise OracleExportError(f"{label} contains invalid JSON") from exc
        return _normalize_json_value(value, label)
    if _DECIMAL_TYPE_RE.fullmatch(mysql_type):
        if not isinstance(value, str) or not _CANONICAL_DECIMAL_RE.fullmatch(value):
            raise OracleExportError(
                f"{label} DECIMAL must be an exact fixed-point string"
            )
        return value
    if _DATETIME_TYPE_RE.fullmatch(mysql_type):
        if not isinstance(value, str) or not _CANONICAL_DATETIME_RE.fullmatch(value):
            raise OracleExportError(
                f"{label} DATETIME must be UTC with six fractional digits"
            )
        return value
    if _INTEGER_TYPE_RE.fullmatch(mysql_type):
        if isinstance(value, bool) or not isinstance(value, int):
            raise OracleExportError(f"{label} integer has wrong JSON type")
        return value
    if _FLOAT_TYPE_RE.fullmatch(mysql_type):
        if isinstance(value, bool) or not isinstance(value, (int, float)):
            raise OracleExportError(f"{label} floating value has wrong JSON type")
        if not math.isfinite(float(value)):
            raise OracleExportError(f"{label} floating value is non-finite")
        return value
    if _TEXT_TYPE_RE.fullmatch(mysql_type):
        if not isinstance(value, str):
            raise OracleExportError(f"{label} text has wrong JSON type")
        return value
    if _BINARY_TYPE_RE.fullmatch(mysql_type):
        if not isinstance(value, str):
            raise OracleExportError(f"{label} binary value must be encoded text")
        return value
    raise OracleExportError(f"{label} has unsupported MySQL type {mysql_type!r}")


def _normalize_row(
    spec: DatasetSpec,
    payload: Any,
    schema_by_field: Mapping[str, Mapping[str, Any]],
) -> dict[str, Any]:
    if not isinstance(payload, dict):
        raise OracleExportError(f"{spec.name} row payload is not an object")
    if set(payload) != set(spec.columns):
        missing = sorted(set(spec.columns) - set(payload))
        extra = sorted(set(payload) - set(spec.columns))
        raise OracleExportError(
            f"{spec.name} row fields differ; missing={missing}, extra={extra}"
        )
    return {
        column: _normalize_column(
            spec.name, column, payload[column], schema_by_field[column]
        )
        for column in spec.columns
    }


def _normalized_key(spec: DatasetSpec, key: str) -> int | str:
    if spec.key_kind == "integer":
        try:
            value = int(key, 10)
        except ValueError as exc:
            raise OracleExportError(
                f"{spec.name} row key is not an integer: {key!r}"
            ) from exc
        if str(value) != key:
            raise OracleExportError(
                f"{spec.name} row key is not canonical: {key!r}"
            )
        return value
    if spec.key_kind == "string":
        return key
    raise OracleExportError(f"unsupported key kind {spec.key_kind!r}")


def build_evidence(
    database: str,
    metadata: Mapping[str, Any],
    schemas: Mapping[str, Sequence[Mapping[str, Any]]],
    rows: Mapping[str, Sequence[tuple[str, Any]]],
) -> tuple[dict[str, Any], dict[str, bytes]]:
    """Validate the snapshot and construct all evidence files in memory."""
    files: dict[str, bytes] = {}
    dataset_manifests: list[dict[str, Any]] = []

    for spec in DATASETS:
        schema = list(schemas.get(spec.name, ()))
        schema_by_field = _validate_schema(spec, schema)
        previous_key: int | str | None = None
        row_hashes: list[str] = []
        output_lines: list[bytes] = []
        first_key: int | str | None = None
        last_key: int | str | None = None

        for raw_key, raw_payload in rows.get(spec.name, ()):
            key = _normalized_key(spec, raw_key)
            if previous_key is not None and key <= previous_key:
                raise OracleExportError(
                    f"{spec.name} keys are duplicate or out of order: {key!r}"
                )
            normalized = _normalize_row(spec, raw_payload, schema_by_field)
            if normalized[spec.key] != key:
                raise OracleExportError(
                    f"{spec.name} key column does not match row key {key!r}"
                )
            hash_payload = {
                "dataset": spec.name,
                "row": normalized,
                "schema_version": SCHEMA_VERSION,
            }
            row_hash = sha256_hex(canonical_json_bytes(hash_payload))
            line = {
                "dataset": spec.name,
                "row": normalized,
                "row_key": key,
                "row_sha256": row_hash,
            }
            output_lines.append(canonical_json_bytes(line) + b"\n")
            row_hashes.append(row_hash)
            if first_key is None:
                first_key = key
            last_key = key
            previous_key = key

        filename = f"{spec.name}.ndjson"
        file_bytes = b"".join(output_lines)
        files[filename] = file_bytes
        columns_evidence = {
            "dataset": spec.name,
            "excluded_schema_columns": list(spec.excluded_schema_columns),
            "selected_columns": list(spec.columns),
        }
        dataset_manifests.append(
            {
                "columns_sha256": sha256_hex(
                    canonical_json_bytes(columns_evidence)
                ),
                "dataset": spec.name,
                "dataset_sha256": sha256_hex(
                    "".join(f"{item}\n" for item in row_hashes).encode("ascii")
                ),
                "file": filename,
                "file_sha256": sha256_hex(file_bytes),
                "first_key": first_key,
                "key": spec.key,
                "last_key": last_key,
                "row_count": len(row_hashes),
                "schema": schema,
                "schema_sha256": sha256_hex(canonical_json_bytes(schema)),
                "source_table": spec.table,
            }
        )

    manifest_without_hash = {
        "database": database,
        "datasets": dataset_manifests,
        "export_contract": "frozen_a_oracle_v2",
        "mysql_evidence": dict(metadata),
        "schema_version": SCHEMA_VERSION,
        "transaction": {
            "access_mode": "READ ONLY",
            "consistent_snapshot": True,
            "isolation_level": "REPEATABLE READ",
            "session_time_zone": "+00:00",
            "single_connection": True,
        },
    }
    evidence_sha256 = sha256_hex(canonical_json_bytes(manifest_without_hash))
    manifest = dict(manifest_without_hash)
    manifest["evidence_sha256"] = evidence_sha256
    files["manifest.json"] = canonical_json_bytes(manifest) + b"\n"
    return manifest, files


Runner = Callable[[Sequence[str], str], str]


def _subprocess_runner(command: Sequence[str], sql: str) -> str:
    completed = subprocess.run(
        list(command),
        input=sql,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        encoding="utf-8",
        errors="strict",
        check=False,
    )
    if completed.returncode != 0:
        stderr = completed.stderr.strip()
        raise OracleExportError(
            f"mysql client failed with exit {completed.returncode}: {stderr}"
        )
    return completed.stdout


def _mysql_command(
    mysql_bin: str,
    docker_container: str | None,
    defaults_extra_file: Path | None,
    host: str | None,
    port: int | None,
    user: str | None,
    socket: str | None,
) -> list[str]:
    if docker_container is not None:
        if not _CONTAINER_RE.fullmatch(docker_container):
            raise OracleExportError(
                f"unsafe Docker container name: {docker_container!r}"
            )
        command = ["docker", "exec", "-i", docker_container, mysql_bin]
    else:
        command = [mysql_bin]
    if defaults_extra_file is not None:
        command.append(f"--defaults-extra-file={defaults_extra_file}")
    command.extend(
        (
            "--batch",
            "--raw",
            "--skip-column-names",
            "--binary-mode",
            "--default-character-set=utf8mb4",
        )
    )
    if host:
        command.append(f"--host={host}")
    if port is not None:
        command.append(f"--port={port}")
    if user:
        command.append(f"--user={user}")
    if socket:
        command.append(f"--socket={socket}")
    return command


def _atomic_write(path: Path, content: bytes) -> None:
    temporary = path.with_name(f".{path.name}.tmp.{os.getpid()}")
    try:
        with temporary.open("xb") as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if temporary.exists():
            temporary.unlink()


def export_frozen_a_oracle(
    *,
    database: str,
    output_dir: Path,
    mysql_bin: str = "mysql",
    docker_container: str | None = None,
    defaults_extra_file: Path | None = None,
    host: str | None = None,
    port: int | None = None,
    user: str | None = None,
    socket: str | None = None,
    runner: Runner = _subprocess_runner,
) -> dict[str, Any]:
    """Execute and persist a complete Frozen-A oracle evidence package."""
    _quote_identifier(database)
    command = _mysql_command(
        mysql_bin,
        docker_container,
        defaults_extra_file,
        host,
        port,
        user,
        socket,
    )
    sql = build_snapshot_sql(database)
    output = runner(command, sql)
    metadata, schemas, rows = parse_mysql_output(output)
    manifest, files = build_evidence(database, metadata, schemas, rows)
    output_dir.mkdir(parents=True, exist_ok=True)
    for filename, content in files.items():
        if filename != "manifest.json":
            _atomic_write(output_dir / filename, content)
    _atomic_write(output_dir / "manifest.json", files["manifest.json"])
    return manifest


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description=(
            "Export seven canonical NDJSON datasets from one MySQL "
            "READ ONLY consistent snapshot."
        )
    )
    parser.add_argument("--database", required=True)
    parser.add_argument("--output-dir", required=True, type=Path)
    parser.add_argument("--mysql-bin", default="mysql")
    parser.add_argument(
        "--docker-container",
        help=(
            "run one mysql client inside this container; no container "
            "environment or credentials are recorded"
        ),
    )
    parser.add_argument(
        "--defaults-extra-file",
        type=Path,
        help="mysql option file path; its contents are never read or recorded",
    )
    parser.add_argument("--host")
    parser.add_argument("--port", type=int)
    parser.add_argument("--user")
    parser.add_argument("--socket")
    return parser


def main(argv: Iterable[str] | None = None) -> int:
    args = _build_parser().parse_args(argv)
    try:
        manifest = export_frozen_a_oracle(
            database=args.database,
            output_dir=args.output_dir,
            mysql_bin=args.mysql_bin,
            docker_container=args.docker_container,
            defaults_extra_file=args.defaults_extra_file,
            host=args.host,
            port=args.port,
            user=args.user,
            socket=args.socket,
        )
    except OracleExportError as exc:
        print(f"frozen-a oracle export failed: {exc}", file=sys.stderr)
        return 1
    print(
        json.dumps(
            {
                "evidence_sha256": manifest["evidence_sha256"],
                "output_dir": str(args.output_dir),
            },
            ensure_ascii=False,
            sort_keys=True,
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
