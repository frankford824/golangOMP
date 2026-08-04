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
LOGICAL_DATABASE = re.compile(r"^ab_[A-Za-z0-9_]+$")
PHYSICAL_DATABASE = "jst_erp"
PHYSICAL_ISOLATION_KIND = "docker_published_port_v1"
CONTAINER_ID = re.compile(r"^[0-9a-f]{64}$")
IMAGE_DIGEST = re.compile(r"^sha256:[0-9a-f]{64}$")
CONTAINER_NAME = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$")
ATTESTATION_FIELDS_V1 = {
    "schema_version",
    "run_id",
    "clone_label",
    "clone_database",
    "snapshot_sha256",
    "source_coordinates",
    "baseline_fingerprint_sha256",
    "import_receipt_sha256",
}
PHYSICAL_ISOLATION_FIELDS = {
    "clone_side",
    "isolation_kind",
    "database_host",
    "database_port",
    "container_port",
    "container_name",
    "container_id",
    "container_image_digest",
    "container_inspect_sha256",
    "source_compound_snapshot_sha256",
    "production_write_performed",
}
ATTESTATION_FIELDS_V2 = ATTESTATION_FIELDS_V1 | PHYSICAL_ISOLATION_FIELDS


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
    physical = bool(getattr(args, "physical_docker_isolation", False))
    snapshot_sha256 = sha(args.snapshot_file)
    payload: dict[str, Any] = {
        "schema_version": 2 if physical else 1,
        "run_id": run_id,
        "clone_label": args.clone_label,
        "clone_database": args.clone_database,
        "snapshot_sha256": snapshot_sha256,
        "source_coordinates": coordinates,
        "baseline_fingerprint_sha256": sha(args.baseline_fingerprint),
        "import_receipt_sha256": sha(args.import_receipt),
    }
    if physical:
        inspect_file = getattr(args, "container_inspect_file", None)
        if (
            not isinstance(inspect_file, pathlib.Path)
            or inspect_file.is_symlink()
            or not inspect_file.is_file()
        ):
            raise ValueError(
                "physical clone attestation requires container inspect file"
            )
        payload.update(
            {
                "clone_side": args.clone_label,
                "isolation_kind": PHYSICAL_ISOLATION_KIND,
                "database_host": getattr(args, "database_host", None),
                "database_port": getattr(args, "database_port", None),
                "container_port": getattr(args, "container_port", None),
                "container_name": getattr(args, "container_name", None),
                "container_id": getattr(args, "container_id", None),
                "container_image_digest": getattr(
                    args, "container_image_digest", None
                ),
                "container_inspect_sha256": sha(inspect_file),
                "source_compound_snapshot_sha256": snapshot_sha256,
                "production_write_performed": False,
            }
        )
    violations = validate_attestation(
        payload,
        label=args.clone_label,
        expected_run_id=run_id,
        expected_clone_label=args.clone_label,
    )
    if violations:
        raise ValueError(violations[0]["detail"])
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

    schema_version = item.get("schema_version")
    expected_fields = (
        ATTESTATION_FIELDS_V1
        if not isinstance(schema_version, bool) and schema_version == 1
        else ATTESTATION_FIELDS_V2
        if not isinstance(schema_version, bool) and schema_version == 2
        else set()
    )
    if not expected_fields or set(item) != expected_fields:
        add("snapshot.attestation_field_contract", "exact field set mismatch")
        return violations
    if item.get("run_id") != expected_run_id:
        add("snapshot.attestation_run_id", "run_id mismatch")
    if item.get("clone_label") != expected_clone_label:
        add("snapshot.attestation_clone_label", "clone label mismatch")
    database = item.get("clone_database")
    if schema_version == 1 and (
        not isinstance(database, str)
        or not LOGICAL_DATABASE.fullmatch(database)
    ):
        add("snapshot.attestation_database", "clone database is invalid")
    if schema_version == 2:
        side = item.get("clone_side")
        port = item.get("database_port")
        container_name = str(item.get("container_name") or "")
        expected_marker = re.compile(
            rf"(?:^|[-_])(?:clone|prebundle)[-_]?"
            rf"{expected_clone_label.lower()}(?:[-_.]|$)"
        )
        if database != PHYSICAL_DATABASE:
            add(
                "snapshot.attestation_database",
                "physical clone database must be jst_erp",
            )
        if (
            side != expected_clone_label
            or item.get("isolation_kind") != PHYSICAL_ISOLATION_KIND
            or item.get("database_host") != "127.0.0.1"
            or isinstance(port, bool)
            or not isinstance(port, int)
            or port < 1024
            or port > 65535
            or port == 3306
            or item.get("container_port") != 3306
            or item.get("production_write_performed") is not False
        ):
            add(
                "snapshot.attestation_physical_isolation",
                "physical clone network/side/write boundary is invalid",
            )
        if (
            not CONTAINER_NAME.fullmatch(container_name)
            or not expected_marker.search(container_name.lower())
            or not CONTAINER_ID.fullmatch(
                str(item.get("container_id") or "")
            )
            or not IMAGE_DIGEST.fullmatch(
                str(item.get("container_image_digest") or "")
            )
            or not SHA256.fullmatch(
                str(item.get("container_inspect_sha256") or "")
            )
        ):
            add(
                "snapshot.attestation_container",
                "physical clone container identity is invalid",
            )
        if (
            item.get("source_compound_snapshot_sha256")
            != item.get("snapshot_sha256")
        ):
            add(
                "snapshot.attestation_compound_snapshot",
                "source compound snapshot hash differs",
            )
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
    elif schema_version == 2 and (
        not isinstance(coordinates.get("binlog_file"), str)
        or not coordinates.get("binlog_file")
        or isinstance(coordinates.get("binlog_position"), bool)
        or not isinstance(coordinates.get("binlog_position"), int)
        or coordinates["binlog_position"] < 0
        or coordinates.get("snapshot_sha256")
        != item.get("source_compound_snapshot_sha256")
    ):
        add(
            "snapshot.attestation_coordinates",
            "physical clone source coordinates are invalid",
        )
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
    source_version = source.get("schema_version")
    target_version = target.get("schema_version")
    if source_version != target_version:
        violations.append(
            {
                "violation_code": "snapshot.attestation_version_mismatch",
                "entity_key": "schema_version",
                "detail": "A and B attestation versions differ",
            }
        )
    if (
        source.get("clone_database") == target.get("clone_database")
        and not (source_version == target_version == 2)
    ):
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
        "source_compound_snapshot_sha256",
    ):
        if field not in source and field not in target:
            continue
        if source.get(field) != target.get(field):
            violations.append(
                {
                    "violation_code": "snapshot.identity_mismatch",
                    "entity_key": field,
                    "detail": "A and B attestations differ",
                }
            )
    if source_version == target_version == 2:
        for field in (
            "database_port",
            "container_name",
            "container_id",
            "container_inspect_sha256",
        ):
            if source.get(field) == target.get(field):
                violations.append(
                    {
                        "violation_code": "snapshot.clone_not_distinct",
                        "entity_key": field,
                        "detail": "physical Clone A and B identities are equal",
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
        "schema_version": (
            2 if source_version == target_version == 2 else 1
        ),
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
    make.add_argument(
        "--physical-docker-isolation", action="store_true"
    )
    make.add_argument("--database-host", default="")
    make.add_argument("--database-port", type=int)
    make.add_argument("--container-port", type=int)
    make.add_argument("--container-name", default="")
    make.add_argument("--container-id", default="")
    make.add_argument("--container-image-digest", default="")
    make.add_argument("--container-inspect-file", type=pathlib.Path)
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
