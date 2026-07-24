#!/usr/bin/env python3
"""Restore exact search rows to an explicitly confirmed local Clone B."""

from __future__ import annotations

import argparse
import pathlib

try:
    from scripts.ab import g4_clone_db as db
except ModuleNotFoundError:
    import g4_clone_db as db


def run(args: argparse.Namespace) -> dict:
    connection = db.Connection.confirmed_clone_b(args.database, args.mysql)
    snapshot = db.load_object(args.snapshot, "snapshot")
    archive = args.archive.read_bytes()
    archive_sha = db.sha256_bytes(archive)
    archive_info = snapshot.get("archive")
    if (
        snapshot.get("schema_version") != 1
        or snapshot.get("status") != "CAPTURED"
        or snapshot.get("violation_count") != 0
        or not isinstance(archive_info, dict)
        or archive_info.get("format") != "deterministic-jsonl-v1"
        or archive_info.get("sha256") != archive_sha
        or archive_info.get("size") != len(archive)
    ):
        raise ValueError("search snapshot/archive binding is invalid")
    archive_schema, archive_rows = db.parse_search_archive(archive)
    archive_tables = db.table_summaries(
        archive_schema, archive_rows, include_schema=False
    )
    snapshot_sha = db.sha256_bytes(db.canonical_bytes(archive_tables))
    if (
        snapshot.get("database") != args.database
        or snapshot.get("tables") != archive_tables
        or snapshot.get("snapshot_sha256") != snapshot_sha
    ):
        raise ValueError("search snapshot content is stale or for another DB")
    current_schema = db.discover_schema(connection, db.SEARCH_COLUMNS)
    db.validate_search_schema(current_schema)
    if current_schema != archive_schema:
        raise ValueError("current search schema differs from captured schema")
    connection.execute(db.restore_sql(archive_schema, archive_rows))
    restored_schema = db.discover_schema(connection, db.SEARCH_COLUMNS)
    restored_schema, restored_rows = db.capture(connection, restored_schema)
    restored_tables = db.table_summaries(
        restored_schema, restored_rows, include_schema=False
    )
    restored_sha = db.sha256_bytes(db.canonical_bytes(restored_tables))
    status = "PASS" if restored_tables == archive_tables else "FAIL"
    payload = {
        "schema_version": 1,
        "status": status,
        "violation_count": 0 if status == "PASS" else 1,
        "database": args.database,
        "snapshot_sha256": snapshot_sha,
        "restored_snapshot_sha256": restored_sha,
        "restored_tables": restored_tables,
        "source_archive_sha256": archive_sha,
    }
    db.atomic_write(args.output, db.canonical_bytes(payload))
    if status != "PASS":
        raise RuntimeError("search tables differ after restore")
    return payload


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--database", required=True)
    parser.add_argument("--snapshot", required=True, type=pathlib.Path)
    parser.add_argument("--archive", required=True, type=pathlib.Path)
    parser.add_argument("--output", required=True, type=pathlib.Path)
    parser.add_argument("--mysql", default="mysql")
    args = parser.parse_args()
    try:
        run(args)
    except Exception as exc:
        parser.error(str(exc))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
