#!/usr/bin/env python3
"""Independently verify exactly three G06 Clone B recovery objects read-only."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import tempfile
from typing import Any

try:
    from scripts.ab import g06_recovery_contract as contract
except ModuleNotFoundError:
    import g06_recovery_contract as contract


SCHEMA_VERSION = 1
VERDICT_TYPE = "g06_clone_b_recovery_verifier_v1"
FIELDS = {
    "schema_version",
    "verdict_type",
    "status",
    "violation_count",
    "checked_count",
    "recovery_manifest_sha256",
    "mapping_sha256",
    "recovery_plan_sha256",
    "recovery_db_apply_sha256",
    "recovery_db_idempotent_sha256",
    "recovery_component_apply_sha256",
    "database",
    "read_only_local_get_count",
    "database_write_performed",
    "production_write_performed",
    "violations",
    "evidence_hash",
}


def canonical_json(value: Any) -> str:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    )


def canonical_jsonl(rows: list[dict[str, Any]]) -> bytes:
    return "".join(canonical_json(row) + "\n" for row in rows).encode("utf-8")


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def safe_path(root: pathlib.Path, object_key: str) -> pathlib.Path:
    if (
        not object_key
        or object_key.startswith("/")
        or "\\" in object_key
        or any(part in {"", ".", ".."} for part in object_key.split("/"))
    ):
        raise ValueError("recovery object key is unsafe")
    current = root
    for segment in object_key.split("/"):
        current = current / segment
        if current.is_symlink():
            raise ValueError("recovery object path contains a symlink")
    try:
        resolved = current.resolve(strict=True)
        resolved.relative_to(root)
    except (FileNotFoundError, RuntimeError, ValueError) as exc:
        raise ValueError("recovery object is missing or outside the local root") from exc
    if not resolved.is_file():
        raise ValueError("recovery object is not a regular file")
    return resolved


def verify(
    *,
    object_root: pathlib.Path,
    mapping_path: pathlib.Path,
    expected_mapping_sha256: str,
    plan_path: pathlib.Path,
    expected_plan_sha256: str,
    db_apply_path: pathlib.Path,
    db_idempotent_path: pathlib.Path,
    component_apply_path: pathlib.Path,
    require_frozen: bool = True,
) -> dict[str, Any]:
    violations: list[dict[str, str]] = []
    checked = 0
    local_gets = 0
    recovery_sha = "0" * 64
    mapping_sha = "0" * 64
    plan_sha = "0" * 64
    receipt_hashes = {
        "recovery_db_apply_sha256": "0" * 64,
        "recovery_db_idempotent_sha256": "0" * 64,
        "recovery_component_apply_sha256": "0" * 64,
    }
    try:
        if object_root.is_symlink() or not object_root.is_dir():
            raise ValueError(
                "recovery object root must be an existing non-symlink directory"
            )
        resolved_root = object_root.resolve(strict=True)
        mapping_rows, plan_entries, hashes = contract.load_contract(
            mapping_path=mapping_path,
            expected_mapping_sha256=expected_mapping_sha256,
            plan_path=plan_path,
            expected_plan_sha256=expected_plan_sha256,
        )
        mapping_sha = hashes["recovery_mapping_sha256"]
        plan_sha = hashes["recovery_plan_sha256"]
        if require_frozen:
            contract.require_frozen_hashes(mapping_sha, plan_sha)
        receipt_hashes = contract.validate_apply_receipts(
            plan_path=plan_path,
            db_apply_path=db_apply_path,
            db_idempotent_path=db_idempotent_path,
            component_apply_path=component_apply_path,
            require_frozen=require_frozen,
        )
        rows = contract.recovery_manifest_rows(mapping_rows, plan_entries)
        recovery_sha = sha256_bytes(canonical_jsonl(rows))
        for row in rows:
            path = safe_path(resolved_root, row["object_key"])
            local_gets += 1
            actual_size = path.stat().st_size
            actual_sha = contract.sha256_file(path)
            if actual_size != row["size"]:
                violations.append(
                    {
                        "violation_code": "g06.recovery_size_mismatch",
                        "entity_key": row["entity_key"],
                        "detail": "Clone B recovery size differs from plan",
                    }
                )
            elif actual_sha != row["sha256"]:
                violations.append(
                    {
                        "violation_code": "g06.recovery_sha256_mismatch",
                        "entity_key": row["entity_key"],
                        "detail": "Clone B recovery SHA-256 differs from plan",
                    }
                )
            else:
                checked += 1
    except (OSError, ValueError) as exc:
        violations.append(
            {
                "violation_code": "g06.recovery_contract",
                "entity_key": "*",
                "detail": str(exc),
            }
        )
    result: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "verdict_type": VERDICT_TYPE,
        "status": "PASS" if not violations and checked == 3 else "BLOCKED",
        "violation_count": len(violations),
        "checked_count": checked,
        "recovery_manifest_sha256": recovery_sha,
        "mapping_sha256": mapping_sha,
        "recovery_plan_sha256": plan_sha,
        **{
            key: receipt_hashes[key]
            for key in (
                "recovery_db_apply_sha256",
                "recovery_db_idempotent_sha256",
                "recovery_component_apply_sha256",
            )
        },
        "database": contract.EXPECTED_DATABASE,
        "read_only_local_get_count": local_gets,
        "database_write_performed": False,
        "production_write_performed": False,
        "violations": sorted(
            violations,
            key=lambda item: (
                item["entity_key"],
                item["violation_code"],
                item["detail"],
            ),
        ),
    }
    result["evidence_hash"] = sha256_bytes(
        canonical_json(result).encode("utf-8")
    )
    return result


def atomic_write(path: pathlib.Path, payload: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    if path.exists():
        if path.is_file() and not path.is_symlink() and path.read_bytes() == payload:
            return
        raise FileExistsError(f"refusing to overwrite different output: {path}")
    temporary: pathlib.Path | None = None
    try:
        with tempfile.NamedTemporaryFile(
            dir=path.parent,
            prefix=path.name + ".",
            suffix=".tmp",
            delete=False,
        ) as handle:
            temporary = pathlib.Path(handle.name)
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
        temporary = None
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--object-root", type=pathlib.Path, required=True)
    parser.add_argument("--mapping", type=pathlib.Path, required=True)
    parser.add_argument("--expected-mapping-sha256", required=True)
    parser.add_argument("--recovery-plan", type=pathlib.Path, required=True)
    parser.add_argument("--expected-recovery-plan-sha256", required=True)
    parser.add_argument(
        "--recovery-db-apply", type=pathlib.Path, required=True
    )
    parser.add_argument(
        "--recovery-db-idempotent", type=pathlib.Path, required=True
    )
    parser.add_argument(
        "--recovery-component-apply", type=pathlib.Path, required=True
    )
    parser.add_argument("--output", type=pathlib.Path, required=True)
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    inputs = {
        args.mapping.resolve(),
        args.recovery_plan.resolve(),
        args.recovery_db_apply.resolve(),
        args.recovery_db_idempotent.resolve(),
        args.recovery_component_apply.resolve(),
    }
    if args.output.resolve() in inputs:
        raise ValueError("output must not overwrite an input")
    result = verify(
        object_root=args.object_root,
        mapping_path=args.mapping,
        expected_mapping_sha256=args.expected_mapping_sha256,
        plan_path=args.recovery_plan,
        expected_plan_sha256=args.expected_recovery_plan_sha256,
        db_apply_path=args.recovery_db_apply,
        db_idempotent_path=args.recovery_db_idempotent,
        component_apply_path=args.recovery_component_apply,
    )
    atomic_write(
        args.output, (canonical_json(result) + "\n").encode("utf-8")
    )
    return 0 if result["status"] == "PASS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
