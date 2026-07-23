#!/usr/bin/env python3
"""Create/verify evidence that A and B were imported from one snapshot."""
from __future__ import annotations

import argparse
import hashlib
import json
import pathlib


def sha(path: pathlib.Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def create(args: argparse.Namespace) -> int:
    coordinates = json.loads(args.source_coordinates.read_text(encoding="utf-8"))
    if not isinstance(coordinates, dict) or not coordinates:
        raise ValueError("source coordinates must be a non-empty JSON object")
    payload = {
        "version": 1,
        "clone_label": args.clone_label,
        "clone_database": args.clone_database,
        "snapshot_sha256": sha(args.snapshot_file),
        "source_coordinates": coordinates,
        "baseline_fingerprint_sha256": sha(args.baseline_fingerprint),
        "baseline_fingerprint_file_sha256": sha(args.baseline_fingerprint),
        "import_receipt_sha256": sha(args.import_receipt),
    }
    args.output.write_text(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return 0


def verify(args: argparse.Namespace) -> int:
    left = json.loads(args.source.read_text(encoding="utf-8"))
    right = json.loads(args.target.read_text(encoding="utf-8"))
    violations = []
    required = {"version", "clone_label", "clone_database", "snapshot_sha256", "source_coordinates",
                "baseline_fingerprint_sha256", "baseline_fingerprint_file_sha256", "import_receipt_sha256"}
    for label, item in (("source", left), ("target", right)):
        missing = required - set(item)
        if missing:
            violations.append({"violation_code": "snapshot.attestation_missing_fields", "entity_key": label, "detail": str(sorted(missing))})
        if item.get("version") != 1:
            violations.append({"violation_code": "snapshot.attestation_version", "entity_key": label, "detail": str(item.get("version"))})
    for field in ("snapshot_sha256", "source_coordinates", "baseline_fingerprint_sha256"):
        if left.get(field) != right.get(field):
            violations.append({"violation_code": "snapshot.identity_mismatch", "entity_key": field, "detail": "A and B attestations differ"})
    if left.get("clone_database") == right.get("clone_database"):
        violations.append({"violation_code": "snapshot.clone_not_distinct", "entity_key": "clone_database", "detail": str(left.get("clone_database"))})
    if args.expected_snapshot_sha256 and left.get("snapshot_sha256") != args.expected_snapshot_sha256:
        violations.append({"violation_code": "snapshot.expected_hash_mismatch", "entity_key": "snapshot_sha256", "detail": str(left.get("snapshot_sha256"))})
    result = {"violation_count": len(violations), "violations": violations,
              "snapshot_sha256": left.get("snapshot_sha256"),
              "baseline_fingerprint_sha256": left.get("baseline_fingerprint_sha256")}
    args.output.write_text(json.dumps(result, ensure_ascii=False, sort_keys=True, separators=(",", ":")) + "\n", encoding="utf-8")
    return 0 if not violations else 1


def main() -> None:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)
    make = sub.add_parser("create")
    make.add_argument("--clone-label", required=True, choices=("A", "B"))
    make.add_argument("--clone-database", required=True)
    make.add_argument("--snapshot-file", required=True, type=pathlib.Path)
    make.add_argument("--source-coordinates", required=True, type=pathlib.Path)
    make.add_argument("--baseline-fingerprint", required=True, type=pathlib.Path)
    make.add_argument("--import-receipt", required=True, type=pathlib.Path)
    make.add_argument("--output", required=True, type=pathlib.Path)
    check = sub.add_parser("verify")
    check.add_argument("--source", required=True, type=pathlib.Path)
    check.add_argument("--target", required=True, type=pathlib.Path)
    check.add_argument("--expected-snapshot-sha256", default="")
    check.add_argument("--output", required=True, type=pathlib.Path)
    args = parser.parse_args()
    try:
        code = create(args) if args.command == "create" else verify(args)
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        if hasattr(args, "output"):
            args.output.write_text(json.dumps({"violation_count": 1, "violations": [{"violation_code": "snapshot.attestation_error", "entity_key": "*", "detail": str(exc)}]}, sort_keys=True) + "\n", encoding="utf-8")
        code = 1
    raise SystemExit(code)


if __name__ == "__main__":
    main()
