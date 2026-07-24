#!/usr/bin/env python3
"""Capture a full Clone B baseline or compare rollback state to that baseline."""

from __future__ import annotations

import argparse
import pathlib

try:
    from scripts.ab import g4_clone_db as db
except ModuleNotFoundError:
    import g4_clone_db as db


def capture_document(connection: db.Connection) -> dict:
    schema = db.discover_schema(connection)
    _, tables = db.capture_fingerprint(connection, schema)
    return {
        "schema_version": 1,
        "kind": "clone-b-baseline-fingerprint",
        "database": connection.database,
        "fingerprint_algorithm": db.ROW_FINGERPRINT_ALGORITHM,
        "tables": tables,
        "fingerprint_sha256": db.sha256_bytes(db.canonical_bytes(tables)),
    }


def run(args: argparse.Namespace) -> dict:
    connection = db.Connection.confirmed_clone_b(args.database, args.mysql)
    current = capture_document(connection)
    if args.capture_baseline:
        db.atomic_write(args.output, db.canonical_bytes(current))
        return current
    if args.baseline is None:
        raise ValueError("--baseline is required for rollback comparison")
    baseline = db.load_object(args.baseline, "baseline")
    if (
        baseline.get("schema_version") != 1
        or baseline.get("kind") != "clone-b-baseline-fingerprint"
        or baseline.get("database") != args.database
        or baseline.get("fingerprint_algorithm")
        != db.ROW_FINGERPRINT_ALGORITHM
        or not isinstance(baseline.get("tables"), dict)
        or baseline.get("fingerprint_sha256")
        != db.sha256_bytes(db.canonical_bytes(baseline["tables"]))
    ):
        raise ValueError("baseline fingerprint is invalid or stale")
    baseline_sha = str(baseline["fingerprint_sha256"])
    rollback_sha = str(current["fingerprint_sha256"])
    status = "PASS" if current["tables"] == baseline["tables"] else "FAIL"
    changed_tables = sorted(
        name
        for name in set(baseline["tables"]) | set(current["tables"])
        if baseline["tables"].get(name) != current["tables"].get(name)
    )
    payload = {
        "schema_version": 1,
        "status": status,
        "violation_count": 0 if status == "PASS" else 1,
        "database": args.database,
        "baseline_artifact_sha256": db.sha256_file(args.baseline),
        "baseline_fingerprint_sha256": baseline_sha,
        "rollback_fingerprint_sha256": rollback_sha,
        "changed_tables": changed_tables if status != "PASS" else [],
    }
    db.atomic_write(args.output, db.canonical_bytes(payload))
    if status != "PASS":
        raise RuntimeError(
            "rollback fingerprint differs from the explicit pre-apply baseline"
        )
    return payload


def main() -> int:
    parser = argparse.ArgumentParser()
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--capture-baseline", action="store_true")
    mode.add_argument("--baseline", type=pathlib.Path)
    parser.add_argument("--database", required=True)
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
