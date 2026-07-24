#!/usr/bin/env python3
"""Create and verify hash-bound A/B snapshot import attestations."""

from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import re
from typing import Any


SHA256 = re.compile(r"^[0-9a-f]{64}$")
RUN_ID = re.compile(r"^[a-z0-9][a-z0-9_-]{2,63}$")
ATTESTATION_FIELDS = {
    "schema_version",
    "run_id",
    "clone_label",
    "clone_database",
    "snapshot_sha256",
    "source_coordinates",
    "baseline_fingerprint_sha256",
    "import_receipt_sha256",
}


def canonical_bytes(value: Any) -> bytes:
    return (
        json.dumps(
            value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
        )
        + "\n"
    ).encode("utf-8")


def sha(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def read_object(path: pathlib.Path, label: str) -> dict[str, Any]:
    if path.is_symlink() or not path.is_file():
        raise ValueError(f"{label} must be an existing non-symlink file")
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{label} must be a JSON object")
    return value


def validate_run_id(run_id: str) -> str:
    if not RUN_ID.fullmatch(run_id):
        raise ValueError("run-id is invalid")
    return run_id


def create(args: argparse.Namespace) -> int:
    run_id = validate_run_id(args.run_id)
    coordinates = read_object(args.source_coordinates, "source coordinates")
    if not coordinates:
        raise ValueError("source coordinates must be non-empty")
    payload = {
        "schema_version": 1,
        "run_id": run_id,
        "clone_label": args.clone_label,
        "clone_database": args.clone_database,
        "snapshot_sha256": sha(args.snapshot_file),
        "source_coordinates": coordinates,
        "baseline_fingerprint_sha256": sha(args.baseline_fingerprint),
        "import_receipt_sha256": sha(args.import_receipt),
    }
    args.output.write_bytes(canonical_bytes(payload))
    return 0


def validate_attestation(
    item: dict[str, Any],
    *,
    label: str,
    expected_run_id: str,
    expected_clone_label: str,
) -> list[dict[str, str]]:
    violations: list[dict[str, str]] = []

    def add(code: str, detail: str) -> None:
        violations.append(
            {"violation_code": code, "entity_key": label, "detail": detail}
        )

    if set(item) != ATTESTATION_FIELDS:
        add("snapshot.attestation_field_contract", "exact field set mismatch")
        return violations
    if item.get("schema_version") != 1:
        add("snapshot.attestation_version", "schema_version must be 1")
    if item.get("run_id") != expected_run_id:
        add("snapshot.attestation_run_id", "run_id mismatch")
    if item.get("clone_label") != expected_clone_label:
        add("snapshot.attestation_clone_label", "clone label mismatch")
    database = item.get("clone_database")
    if not isinstance(database, str) or not re.fullmatch(
        r"ab_[A-Za-z0-9_]+", database
    ):
        add("snapshot.attestation_database", "clone database is invalid")
    for field in (
        "snapshot_sha256",
        "baseline_fingerprint_sha256",
        "import_receipt_sha256",
    ):
        if not SHA256.fullmatch(str(item.get(field) or "")):
            add("snapshot.attestation_hash", f"{field} is invalid")
    coordinates = item.get("source_coordinates")
    if not isinstance(coordinates, dict) or not coordinates:
        add("snapshot.attestation_coordinates", "source coordinates are empty")
    return violations


def verify(args: argparse.Namespace) -> int:
    run_id = validate_run_id(args.run_id)
    source = read_object(args.source, "source attestation")
    target = read_object(args.target, "target attestation")
    violations = validate_attestation(
        source,
        label="A",
        expected_run_id=run_id,
        expected_clone_label="A",
    )
    violations.extend(
        validate_attestation(
            target,
            label="B",
            expected_run_id=run_id,
            expected_clone_label="B",
        )
    )
    if source.get("clone_database") == target.get("clone_database"):
        violations.append(
            {
                "violation_code": "snapshot.clone_not_distinct",
                "entity_key": "clone_database",
                "detail": "A and B database names are equal",
            }
        )
    for field in (
        "snapshot_sha256",
        "source_coordinates",
        "baseline_fingerprint_sha256",
    ):
        if source.get(field) != target.get(field):
            violations.append(
                {
                    "violation_code": "snapshot.identity_mismatch",
                    "entity_key": field,
                    "detail": "A and B attestations differ",
                }
            )
    if (
        args.expected_snapshot_sha256
        and source.get("snapshot_sha256") != args.expected_snapshot_sha256
    ):
        violations.append(
            {
                "violation_code": "snapshot.expected_hash_mismatch",
                "entity_key": "snapshot_sha256",
                "detail": "snapshot hash differs from the frozen expectation",
            }
        )
    result: dict[str, Any] = {
        "schema_version": 1,
        "run_id": run_id,
        "status": "PASS" if not violations else "FAIL",
        "violation_count": len(violations),
        "violations": violations,
        "snapshot_sha256": source.get("snapshot_sha256"),
        "baseline_fingerprint_sha256": source.get(
            "baseline_fingerprint_sha256"
        ),
        "source_attestation_sha256": sha(args.source),
        "target_attestation_sha256": sha(args.target),
    }
    result["evidence_sha256"] = hashlib.sha256(canonical_bytes(result)).hexdigest()
    args.output.write_bytes(canonical_bytes(result))
    return 0 if not violations else 1


def failure_result(run_id: str, detail: str) -> dict[str, Any]:
    result: dict[str, Any] = {
        "schema_version": 1,
        "run_id": run_id if RUN_ID.fullmatch(run_id) else None,
        "status": "FAIL",
        "violation_count": 1,
        "violations": [
            {
                "violation_code": "snapshot.attestation_error",
                "entity_key": "*",
                "detail": detail,
            }
        ],
    }
    result["evidence_sha256"] = hashlib.sha256(canonical_bytes(result)).hexdigest()
    return result


def main() -> None:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)
    make = sub.add_parser("create")
    make.add_argument("--run-id", required=True)
    make.add_argument("--clone-label", required=True, choices=("A", "B"))
    make.add_argument("--clone-database", required=True)
    make.add_argument("--snapshot-file", required=True, type=pathlib.Path)
    make.add_argument("--source-coordinates", required=True, type=pathlib.Path)
    make.add_argument("--baseline-fingerprint", required=True, type=pathlib.Path)
    make.add_argument("--import-receipt", required=True, type=pathlib.Path)
    make.add_argument("--output", required=True, type=pathlib.Path)
    check = sub.add_parser("verify")
    check.add_argument("--run-id", required=True)
    check.add_argument("--source", required=True, type=pathlib.Path)
    check.add_argument("--target", required=True, type=pathlib.Path)
    check.add_argument("--expected-snapshot-sha256", default="")
    check.add_argument("--output", required=True, type=pathlib.Path)
    args = parser.parse_args()
    try:
        code = create(args) if args.command == "create" else verify(args)
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        if hasattr(args, "output"):
            args.output.write_bytes(
                canonical_bytes(failure_result(str(args.run_id), str(exc)))
            )
        code = 1
    raise SystemExit(code)


if __name__ == "__main__":
    main()
