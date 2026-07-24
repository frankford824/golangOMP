#!/usr/bin/env python3
"""Capture the three tables actually rebuilt by cmd/tools/search-reindex."""

from __future__ import annotations

import argparse
import pathlib

try:
    from scripts.ab import g4_clone_db as db
except ModuleNotFoundError:
    import g4_clone_db as db


def run(args: argparse.Namespace) -> dict:
    connection = db.Connection.confirmed_clone_b(args.database, args.mysql)
    schema = db.discover_schema(connection, db.SEARCH_COLUMNS)
    db.validate_search_schema(schema)
    captured_schema, rows = db.capture(connection, schema)
    tables = db.table_summaries(
        captured_schema, rows, include_schema=False
    )
    archive = db.make_search_archive(captured_schema, rows)
    db.atomic_write(args.archive, archive)
    archive_sha = db.sha256_bytes(archive)
    payload = {
        "schema_version": 1,
        "status": "CAPTURED",
        "violation_count": 0,
        "database": args.database,
        "tables": tables,
        "snapshot_sha256": db.sha256_bytes(db.canonical_bytes(tables)),
        "archive": {
            "format": "deterministic-jsonl-v1",
            "sha256": archive_sha,
            "size": len(archive),
        },
    }
    db.atomic_write(args.output, db.canonical_bytes(payload))
    return payload


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--database", required=True)
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
