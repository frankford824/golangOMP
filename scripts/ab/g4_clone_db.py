#!/usr/bin/env python3
"""Strict local Clone B database evidence helpers for the G4 rehearsal.

The module deliberately uses the MySQL command-line client instead of adding a
Python database dependency.  Connections are accepted only from the
orchestrator-provided ``MYSQL_DSN`` and only when both independent Clone B
confirmation environment variables match the requested ``ab_*_b*`` database.
"""

from __future__ import annotations

import dataclasses
import hashlib
import json
import os
import pathlib
import re
import subprocess
from typing import Any, Iterable


CLONE_B_DB = re.compile(r"^ab_[A-Za-z0-9_]*_b(?:_|$)[A-Za-z0-9_]*$")
DSN = re.compile(
    r"^(?P<user>[^:@]+):(?P<password>.*)@tcp\("
    r"(?P<host>127\.0\.0\.1|localhost):(?P<port>[0-9]{1,5})\)"
    r"/(?P<database>[A-Za-z0-9_]+)(?:\?.*)?$"
)
SEARCH_COLUMNS = {
    "task_search_documents": (
        "task_id",
        "task_no",
        "product_name_snapshot",
        "sku_code",
        "primary_sku_code",
        "product_i_id",
        "task_type",
        "task_status",
        "priority",
        "owner_department",
        "owner_team",
        "owner_org_team",
        "creator_id",
        "creator_name",
        "requester_id",
        "requester_name",
        "designer_id",
        "designer_name",
        "current_handler_id",
        "current_handler_name",
        "created_at",
        "updated_at",
        "deadline_at",
        "asset_text",
        "search_text",
    ),
    "task_asset_group_search_documents": (
        "group_id",
        "task_id",
        "finalized_revision_id",
        "internal_text",
        "final_text",
        "updated_at",
    ),
    "product_search_documents": (
        "sku_code",
        "product_name",
        "i_id",
        "category",
        "search_text",
        "semantic_text",
        "semantic_enriched_at",
        "source_updated_at",
        "updated_at",
    ),
}


def canonical_bytes(value: Any) -> bytes:
    return (
        json.dumps(
            value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
        )
        + "\n"
    ).encode("utf-8")


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def atomic_write(path: pathlib.Path, data: bytes) -> None:
    if path.exists():
        raise FileExistsError(f"refusing to overwrite artifact: {path}")
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(f".{path.name}.{os.getpid()}.tmp")
    temporary.write_bytes(data)
    os.replace(temporary, path)


@dataclasses.dataclass(frozen=True)
class Connection:
    mysql: str
    host: str
    port: int
    user: str
    password: str
    database: str

    @classmethod
    def confirmed_clone_b(
        cls, database: str, mysql: str = "mysql"
    ) -> "Connection":
        if not CLONE_B_DB.fullmatch(database):
            raise ValueError("--database must be an exact ab_*_b* Clone B name")
        if os.environ.get("AB_CONFIRMED_CLONE_SIDE") != "B":
            raise ValueError("AB_CONFIRMED_CLONE_SIDE must be B")
        if os.environ.get("AB_CONFIRMED_CLONE_DATABASE") != database:
            raise ValueError(
                "AB_CONFIRMED_CLONE_DATABASE must equal --database"
            )
        raw = os.environ.get("MYSQL_DSN", "")
        match = DSN.fullmatch(raw)
        if not match:
            raise ValueError(
                "MYSQL_DSN must use user:password@tcp(127.0.0.1|localhost:port)"
            )
        port = int(match.group("port"))
        if not 1 <= port <= 65535:
            raise ValueError("MYSQL_DSN port is invalid")
        if match.group("database") != database:
            raise ValueError("MYSQL_DSN database differs from --database")
        return cls(
            mysql=mysql,
            host=match.group("host"),
            port=port,
            user=match.group("user"),
            password=match.group("password"),
            database=database,
        )

    def execute(self, sql: str) -> str:
        command = [
            self.mysql,
            "--protocol=TCP",
            f"--host={self.host}",
            f"--port={self.port}",
            f"--user={self.user}",
            f"--database={self.database}",
            "--batch",
            "--raw",
            "--skip-column-names",
            "--default-character-set=utf8mb4",
        ]
        env = dict(os.environ)
        env["MYSQL_PWD"] = self.password
        completed = subprocess.run(
            command,
            input=sql,
            text=True,
            encoding="utf-8",
            capture_output=True,
            env=env,
            check=False,
        )
        if completed.returncode != 0:
            detail = completed.stderr.strip()
            raise RuntimeError(
                f"local Clone B mysql command failed ({completed.returncode}): "
                f"{detail}"
            )
        return completed.stdout


def quote_identifier(value: str) -> str:
    if not re.fullmatch(r"[A-Za-z0-9_]+", value):
        raise ValueError(f"unsafe MySQL identifier: {value!r}")
    return f"`{value}`"


def _schema_query(only_tables: Iterable[str] | None) -> str:
    predicate = ""
    if only_tables is not None:
        names = sorted(set(only_tables))
        if not names:
            raise ValueError("table allowlist is empty")
        predicate = " AND c.table_name IN (" + ",".join(
            "'" + name + "'" for name in names
        ) + ")"
    return f"""
SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
SET SESSION TRANSACTION READ ONLY;
START TRANSACTION WITH CONSISTENT SNAPSHOT;
SELECT JSON_OBJECT(
  'kind','column',
  'table',c.table_name,
  'ordinal',c.ordinal_position,
  'name',c.column_name,
  'column_type',c.column_type,
  'nullable',c.is_nullable,
  'default_hex',IF(c.column_default IS NULL,NULL,HEX(CAST(c.column_default AS BINARY))),
  'extra',c.extra,
  'generation_expression',c.generation_expression,
  'character_set',c.character_set_name,
  'collation',c.collation_name
)
FROM information_schema.columns c
JOIN information_schema.tables t
  ON t.table_schema=c.table_schema AND t.table_name=c.table_name
WHERE c.table_schema=DATABASE() AND t.table_type='BASE TABLE'{predicate}
ORDER BY c.table_name,c.ordinal_position;
COMMIT;
"""


def _read_json_lines(raw: str) -> list[dict[str, Any]]:
    result: list[dict[str, Any]] = []
    for line_number, line in enumerate(raw.splitlines(), 1):
        if not line.strip():
            continue
        try:
            value = json.loads(line)
        except json.JSONDecodeError as exc:
            raise RuntimeError(
                f"unexpected MySQL JSON output at line {line_number}"
            ) from exc
        if not isinstance(value, dict):
            raise RuntimeError(
                f"MySQL JSON output line {line_number} is not an object"
            )
        result.append(value)
    return result


def discover_schema(
    connection: Connection, only_tables: Iterable[str] | None = None
) -> dict[str, list[dict[str, Any]]]:
    rows = _read_json_lines(connection.execute(_schema_query(only_tables)))
    schema: dict[str, list[dict[str, Any]]] = {}
    for row in rows:
        if row.get("kind") != "column":
            raise RuntimeError("schema query returned a non-column record")
        table = str(row.get("table") or "")
        name = str(row.get("name") or "")
        if not table or not name:
            raise RuntimeError("schema query returned an incomplete column")
        schema.setdefault(table, []).append(row)
    requested = None if only_tables is None else set(only_tables)
    if requested is not None and set(schema) != requested:
        missing = sorted(requested - set(schema))
        extra = sorted(set(schema) - requested)
        raise RuntimeError(
            f"required search table schema mismatch: missing={missing}, extra={extra}"
        )
    if not schema:
        raise RuntimeError("Clone B contains no base table schema")
    for table, columns in schema.items():
        ordinals = [row.get("ordinal") for row in columns]
        if ordinals != list(range(1, len(columns) + 1)):
            raise RuntimeError(f"{table} column ordinals are not contiguous")
    return schema


def validate_search_schema(
    schema: dict[str, list[dict[str, Any]]]
) -> None:
    if set(schema) != set(SEARCH_COLUMNS):
        raise RuntimeError("search schema table set differs from the G4 contract")
    for table, expected in SEARCH_COLUMNS.items():
        actual = tuple(str(row["name"]) for row in schema[table])
        if actual != expected:
            raise RuntimeError(
                f"{table} explicit columns differ: "
                f"expected={expected!r}, actual={actual!r}"
            )
        for row in schema[table]:
            if row.get("generation_expression") not in {"", None}:
                raise RuntimeError(
                    f"{table}.{row['name']} is unexpectedly generated"
                )


def _capture_sql(schema: dict[str, list[dict[str, Any]]]) -> str:
    statements = [
        "SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ",
        "SET SESSION TRANSACTION READ ONLY",
        "START TRANSACTION WITH CONSISTENT SNAPSHOT",
    ]
    for table in sorted(schema):
        columns = schema[table]
        for row in columns:
            statements.append(
                "SELECT "
                + "JSON_OBJECT("
                + "'kind','column',"
                + f"'table','{table}',"
                + f"'ordinal',{int(row['ordinal'])},"
                + f"'name','{row['name']}',"
                + "'column_type',COLUMN_TYPE,"
                + "'nullable',IS_NULLABLE,"
                + "'default_hex',IF(COLUMN_DEFAULT IS NULL,NULL,HEX(CAST(COLUMN_DEFAULT AS BINARY))),"
                + "'extra',EXTRA,"
                + "'generation_expression',GENERATION_EXPRESSION,"
                + "'character_set',CHARACTER_SET_NAME,"
                + "'collation',COLLATION_NAME)"
                + " FROM information_schema.columns"
                + " WHERE table_schema=DATABASE()"
                + f" AND table_name='{table}'"
                + f" AND ordinal_position={int(row['ordinal'])}"
            )
        cells = ",".join(
            "IF("
            + quote_identifier(str(row["name"]))
            + " IS NULL,NULL,HEX(CAST("
            + quote_identifier(str(row["name"]))
            + " AS BINARY)))"
            for row in columns
        )
        statements.append(
            "SELECT JSON_OBJECT('kind','row',"
            + f"'table','{table}','cells',JSON_ARRAY({cells}))"
            + f" FROM {quote_identifier(table)}"
        )
    statements.append("COMMIT")
    return ";\n".join(statements) + ";\n"


def capture(
    connection: Connection,
    schema: dict[str, list[dict[str, Any]]],
) -> tuple[
    dict[str, list[dict[str, Any]]],
    dict[str, list[list[str | None]]],
]:
    records = _read_json_lines(connection.execute(_capture_sql(schema)))
    captured_schema: dict[str, list[dict[str, Any]]] = {
        table: [] for table in schema
    }
    rows: dict[str, list[list[str | None]]] = {
        table: [] for table in schema
    }
    for record in records:
        table = str(record.get("table") or "")
        if table not in schema:
            raise RuntimeError("capture returned an unexpected table")
        if record.get("kind") == "column":
            captured_schema[table].append(record)
        elif record.get("kind") == "row":
            cells = record.get("cells")
            if (
                not isinstance(cells, list)
                or len(cells) != len(schema[table])
                or any(
                    value is not None
                    and (
                        not isinstance(value, str)
                        or not re.fullmatch(r"(?:[0-9A-F]{2})*", value)
                    )
                    for value in cells
                )
            ):
                raise RuntimeError(f"{table} returned an invalid row encoding")
            rows[table].append(cells)
        else:
            raise RuntimeError("capture returned an unknown record kind")
    if captured_schema != schema:
        raise RuntimeError("database schema drifted during capture")
    for table in rows:
        rows[table].sort(key=canonical_bytes)
    return captured_schema, rows


def table_summaries(
    schema: dict[str, list[dict[str, Any]]],
    rows: dict[str, list[list[str | None]]],
    *,
    include_schema: bool,
) -> dict[str, dict[str, Any]]:
    result: dict[str, dict[str, Any]] = {}
    for table in sorted(schema):
        row_bytes = b"".join(canonical_bytes(row) for row in rows[table])
        value: dict[str, Any] = {
            "row_count": len(rows[table]),
            "content_sha256": sha256_bytes(row_bytes),
        }
        if include_schema:
            value["schema_sha256"] = sha256_bytes(
                canonical_bytes(schema[table])
            )
        result[table] = value
    return result


def make_search_archive(
    schema: dict[str, list[dict[str, Any]]],
    rows: dict[str, list[list[str | None]]],
) -> bytes:
    parts: list[bytes] = []
    for table in sorted(schema):
        parts.append(
            canonical_bytes(
                {"kind": "schema", "table": table, "columns": schema[table]}
            )
        )
        for cells in rows[table]:
            parts.append(
                canonical_bytes(
                    {"kind": "row", "table": table, "cells": cells}
                )
            )
    return b"".join(parts)


def parse_search_archive(
    data: bytes,
) -> tuple[
    dict[str, list[dict[str, Any]]],
    dict[str, list[list[str | None]]],
]:
    schema: dict[str, list[dict[str, Any]]] = {}
    rows: dict[str, list[list[str | None]]] = {}
    for line_number, line in enumerate(data.splitlines(), 1):
        try:
            record = json.loads(line)
        except json.JSONDecodeError as exc:
            raise ValueError(
                f"archive line {line_number} is not JSON"
            ) from exc
        if not isinstance(record, dict):
            raise ValueError(f"archive line {line_number} is not an object")
        table = str(record.get("table") or "")
        if table not in SEARCH_COLUMNS:
            raise ValueError(f"archive line {line_number} has invalid table")
        if record.get("kind") == "schema":
            if table in schema or not isinstance(record.get("columns"), list):
                raise ValueError(f"archive line {line_number} has invalid schema")
            schema[table] = record["columns"]
            rows[table] = []
        elif record.get("kind") == "row":
            cells = record.get("cells")
            if table not in schema or not isinstance(cells, list):
                raise ValueError(f"archive line {line_number} has invalid row")
            rows[table].append(cells)
        else:
            raise ValueError(f"archive line {line_number} has invalid kind")
    validate_search_schema(schema)
    for table in rows:
        expected_count = len(schema[table])
        for cells in rows[table]:
            if (
                len(cells) != expected_count
                or any(
                    value is not None
                    and (
                        not isinstance(value, str)
                        or not re.fullmatch(r"(?:[0-9A-F]{2})*", value)
                    )
                    for value in cells
                )
            ):
                raise ValueError(f"archive {table} contains an invalid row")
        if rows[table] != sorted(rows[table], key=canonical_bytes):
            raise ValueError(f"archive {table} rows are not canonical")
    return schema, rows


def restore_sql(
    schema: dict[str, list[dict[str, Any]]],
    rows: dict[str, list[list[str | None]]],
) -> str:
    statements = [
        "SET SESSION TRANSACTION ISOLATION LEVEL SERIALIZABLE",
        "START TRANSACTION",
    ]
    for table in sorted(schema):
        statements.append(f"DELETE FROM {quote_identifier(table)}")
        column_sql = ",".join(
            quote_identifier(str(column["name"])) for column in schema[table]
        )
        for offset in range(0, len(rows[table]), 100):
            values: list[str] = []
            for cells in rows[table][offset : offset + 100]:
                encoded = ",".join(
                    "NULL" if value is None else f"UNHEX('{value}')"
                    for value in cells
                )
                values.append(f"({encoded})")
            statements.append(
                f"INSERT INTO {quote_identifier(table)} ({column_sql}) VALUES "
                + ",".join(values)
            )
    statements.append("COMMIT")
    return ";\n".join(statements) + ";\n"


def load_object(path: pathlib.Path, label: str) -> dict[str, Any]:
    if path.is_symlink() or not path.is_file():
        raise ValueError(f"{label} must be an existing non-symlink file")
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{label} must be a JSON object")
    return value
