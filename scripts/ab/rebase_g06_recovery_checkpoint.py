#!/usr/bin/env python3
"""Rebase a G06 checkpoint after removing exactly three approved 404 rows."""
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
    from scripts.ab import hydrate_object_manifest as hydrator
    from scripts.ab import object_manifest_verifier as verifier
except ModuleNotFoundError:
    import g06_recovery_contract as contract
    import hydrate_object_manifest as hydrator
    import object_manifest_verifier as verifier


SCHEMA_VERSION = 1


def canonical_json(value: Any) -> str:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    )


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def _read_checkpoint(path: pathlib.Path) -> dict[str, Any]:
    if path.is_symlink() or not path.is_file():
        raise ValueError("old checkpoint must be an existing non-symlink file")
    value = contract.read_json(path, "old checkpoint")
    if (
        value.get("schema_version") != hydrator.CHECKPOINT_SCHEMA_VERSION
        or not isinstance(value.get("input_manifest_sha256"), str)
        or not isinstance(value.get("adapter_fingerprints"), dict)
        or not isinstance(value.get("completed"), list)
        or not isinstance(value.get("failed"), list)
    ):
        raise ValueError("old checkpoint must use the current checkpoint schema")
    return value


def _target_kind(row: dict[str, Any]) -> str:
    adapter = str(row["storage_adapter"]).strip().lower()
    if adapter in verifier.UPLOAD_ADAPTERS:
        return "upload"
    if adapter in verifier.OSS_ADAPTERS:
        return "oss"
    raise ValueError("removed recovery row has an unsupported remote adapter")


def _canonical_manifest(path: pathlib.Path, label: str) -> tuple[list[dict[str, Any]], str]:
    rows, digest = hydrator.read_manifest(path)
    expected = "".join(canonical_json(row) + "\n" for row in rows).encode(
        "utf-8"
    )
    if path.read_bytes() != expected:
        raise ValueError(f"{label} must be canonical JSONL")
    return rows, digest


def rebase(
    *,
    old_input_path: pathlib.Path,
    expected_old_input_sha256: str,
    new_input_path: pathlib.Path,
    expected_new_input_sha256: str,
    old_checkpoint_path: pathlib.Path,
    expected_old_checkpoint_sha256: str,
    mapping_path: pathlib.Path,
    expected_mapping_sha256: str,
    plan_path: pathlib.Path,
    expected_plan_sha256: str,
) -> tuple[bytes, dict[str, Any]]:
    contract.require_hash(
        old_input_path, expected_old_input_sha256, "old hydration input"
    )
    contract.require_hash(
        new_input_path, expected_new_input_sha256, "new hydration input"
    )
    old_checkpoint_sha = contract.require_hash(
        old_checkpoint_path,
        expected_old_checkpoint_sha256,
        "old checkpoint",
    )
    mapping_rows, plan_entries, hashes = contract.load_contract(
        mapping_path=mapping_path,
        expected_mapping_sha256=expected_mapping_sha256,
        plan_path=plan_path,
        expected_plan_sha256=expected_plan_sha256,
    )
    exact_removed = {
        row["entity_key"]: row
        for row in contract.original_manifest_rows(mapping_rows, plan_entries)
    }
    old_rows, old_sha = _canonical_manifest(old_input_path, "old input")
    new_rows, new_sha = _canonical_manifest(new_input_path, "new input")
    old_by_entity = {row["entity_key"]: row for row in old_rows}
    new_by_entity = {row["entity_key"]: row for row in new_rows}
    if len(old_by_entity) != len(old_rows) or len(new_by_entity) != len(new_rows):
        raise ValueError("hydration inputs contain duplicate entities")
    removed = set(old_by_entity) - set(new_by_entity)
    added = set(new_by_entity) - set(old_by_entity)
    if removed != set(exact_removed) or added:
        raise ValueError(
            "old/new hydration inputs must differ by exactly three recovery rows"
        )
    for entity, row in exact_removed.items():
        if old_by_entity.get(entity) != row:
            raise ValueError(f"old hydration input recovery row {entity} drifted")
    for entity in new_by_entity:
        if old_by_entity[entity] != new_by_entity[entity]:
            raise ValueError(
                f"unchanged hydration input row {entity} differs"
            )

    checkpoint = _read_checkpoint(old_checkpoint_path)
    if checkpoint["input_manifest_sha256"] != old_sha:
        raise ValueError("old checkpoint is not bound to the old hydration input")
    fingerprints = checkpoint["adapter_fingerprints"]
    completed, failed = hydrator.load_checkpoint(
        old_checkpoint_path, old_sha, fingerprints
    )
    old_targets = {
        hydrator.checkpoint_key(_target_kind(row), row["object_key"])
        for row in old_rows
        if not row["sha256"]
    }
    if (set(completed) | set(failed)) - old_targets:
        raise ValueError("old checkpoint contains results outside the old input")

    removed_keys: set[str] = set()
    for entity in sorted(exact_removed):
        row = exact_removed[entity]
        key = hydrator.checkpoint_key(
            _target_kind(row), row["object_key"]
        )
        removed_keys.add(key)
        record = failed.get(key)
        if (
            record is None
            or record.get("violation_code") != "object_manifest.missing"
            or record.get("detail") != "http_status=404"
            or key in completed
        ):
            raise ValueError(
                f"checkpoint does not contain exact original 404 for {entity}"
            )
    remaining_completed = {
        key: value for key, value in completed.items()
        if key not in removed_keys
    }
    remaining_failed = {
        key: value for key, value in failed.items()
        if key not in removed_keys
    }
    new_targets = {
        hydrator.checkpoint_key(_target_kind(row), row["object_key"])
        for row in new_rows
        if not row["sha256"]
    }
    if (set(remaining_completed) | set(remaining_failed)) - new_targets:
        raise ValueError("rebased checkpoint contains results outside the new input")

    preserved_completed_rows = [
        record
        for record in checkpoint["completed"]
        if hydrator.checkpoint_key(
            record["adapter_kind"], record["object_key"]
        )
        not in removed_keys
    ]
    preserved_failed_rows = [
        record
        for record in checkpoint["failed"]
        if hydrator.checkpoint_key(
            record["adapter_kind"], record["object_key"]
        )
        not in removed_keys
    ]
    if (
        preserved_completed_rows
        != [
            remaining_completed[key] for key in sorted(remaining_completed)
        ]
        or preserved_failed_rows
        != [remaining_failed[key] for key in sorted(remaining_failed)]
    ):
        raise ValueError(
            "old checkpoint result ordering is noncanonical; refusing byte-changing rebase"
        )
    rebased = {
        "schema_version": hydrator.CHECKPOINT_SCHEMA_VERSION,
        "input_manifest_sha256": new_sha,
        "adapter_fingerprints": fingerprints,
        "completed": preserved_completed_rows,
        "failed": preserved_failed_rows,
    }
    payload = (canonical_json(rebased) + "\n").encode("utf-8")
    summary = {
        "schema_version": SCHEMA_VERSION,
        "status": "PASS",
        "operation": "rebase_g06_recovery_checkpoint",
        "old_input_sha256": old_sha,
        "new_input_sha256": new_sha,
        "old_checkpoint_sha256": old_checkpoint_sha,
        "new_checkpoint_sha256": sha256_bytes(payload),
        **hashes,
        "removed_original_404_entity_keys": sorted(exact_removed),
        "removed_failed_record_count": 3,
        "preserved_completed_record_count": len(remaining_completed),
        "preserved_failed_record_count": len(remaining_failed),
        "database_write_performed": False,
        "production_write_performed": False,
    }
    return payload, summary


def atomic_write_many(outputs: list[tuple[pathlib.Path, bytes]]) -> None:
    resolved = [path.resolve() for path, _ in outputs]
    if len(resolved) != len(set(resolved)):
        raise ValueError("outputs must be distinct")
    pending: list[tuple[pathlib.Path, pathlib.Path]] = []
    try:
        for path, payload in outputs:
            path.parent.mkdir(parents=True, exist_ok=True)
            if path.exists():
                if path.is_file() and not path.is_symlink() and path.read_bytes() == payload:
                    continue
                raise FileExistsError(
                    f"refusing to overwrite different output: {path}"
                )
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
            pending.append((temporary, path))
        for temporary, path in pending:
            os.replace(temporary, path)
    finally:
        for temporary, _ in pending:
            temporary.unlink(missing_ok=True)


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--old-input", type=pathlib.Path, required=True)
    parser.add_argument("--expected-old-input-sha256", required=True)
    parser.add_argument("--new-input", type=pathlib.Path, required=True)
    parser.add_argument("--expected-new-input-sha256", required=True)
    parser.add_argument("--old-checkpoint", type=pathlib.Path, required=True)
    parser.add_argument("--expected-old-checkpoint-sha256", required=True)
    parser.add_argument("--mapping", type=pathlib.Path, required=True)
    parser.add_argument("--expected-mapping-sha256", required=True)
    parser.add_argument("--recovery-plan", type=pathlib.Path, required=True)
    parser.add_argument("--expected-recovery-plan-sha256", required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--summary", type=pathlib.Path, required=True)
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    contract.require_frozen_hashes(
        args.expected_mapping_sha256,
        args.expected_recovery_plan_sha256,
    )
    inputs = {
        args.old_input.resolve(),
        args.new_input.resolve(),
        args.old_checkpoint.resolve(),
        args.mapping.resolve(),
        args.recovery_plan.resolve(),
    }
    outputs = {args.output.resolve(), args.summary.resolve()}
    if len(outputs) != 2 or outputs.intersection(inputs):
        raise ValueError("outputs must be distinct and not overwrite inputs")
    payload, summary = rebase(
        old_input_path=args.old_input,
        expected_old_input_sha256=args.expected_old_input_sha256,
        new_input_path=args.new_input,
        expected_new_input_sha256=args.expected_new_input_sha256,
        old_checkpoint_path=args.old_checkpoint,
        expected_old_checkpoint_sha256=args.expected_old_checkpoint_sha256,
        mapping_path=args.mapping,
        expected_mapping_sha256=args.expected_mapping_sha256,
        plan_path=args.recovery_plan,
        expected_plan_sha256=args.expected_recovery_plan_sha256,
    )
    atomic_write_many(
        [
            (args.output, payload),
            (
                args.summary,
                (canonical_json(summary) + "\n").encode("utf-8"),
            ),
        ]
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
