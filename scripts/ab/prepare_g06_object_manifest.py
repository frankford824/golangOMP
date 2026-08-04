#!/usr/bin/env python3
"""Prepare and finalize the canonical G06 object manifest.

``prepare`` removes the reviewed historical-unavailable tombstone and the three
approved original-404 recovery rows from the frozen object manifest so only
remote-verifiable objects are hydrated. ``finalize`` proves that hydration
changed only byte metadata, restores the tombstone, transforms the three rows
to their hash-bound Clone B recovery targets, and appends the seven reviewed
Clone B source-bundle objects.

All outputs are canonical JSON/JSONL and are immutable once written.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import tempfile
from typing import Any

try:
    from scripts.ab import apply_bundle_registry_to_mapping as bundle_validator
    from scripts.ab import g06_recovery_contract as recovery_contract
    from scripts.ab import hydrate_object_manifest as hydrator
    from scripts.ab import object_manifest_verifier as object_verifier
except ModuleNotFoundError:  # Direct execution from scripts/ab.
    import apply_bundle_registry_to_mapping as bundle_validator
    import g06_recovery_contract as recovery_contract
    import hydrate_object_manifest as hydrator
    import object_manifest_verifier as object_verifier


SCHEMA_VERSION = 1
TOMBSTONE_ENTITY = "task_asset:12323"
TOMBSTONE_CONTRACT = {
    "entity_key": TOMBSTONE_ENTITY,
    "owner_kind": "task_asset",
    "owner_id": 12323,
    "task_id": 2199,
    "storage_ref_id": "c0a135a1-080f-46a0-a41a-461aef0ea0fb",
    "storage_adapter": "oss_upload_service",
    "object_key": (
        "tasks/RW-20260709-A-002196/assets/AST-0002/v1/delivery/"
        "1783575756672661314_d97ed925.psd"
    ),
    "size": 17755216,
    "mime_type": "image/vnd.adobe.photoshop",
    "sha256": "",
    "status": "recorded",
    "is_placeholder": False,
}
HYDRATABLE_FIELDS = {"size", "mime_type", "sha256"}


def reject_duplicate_keys(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate JSON object key: {key}")
        result[key] = value
    return result


def canonical_json(value: Any) -> str:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    )


def canonical_jsonl(rows: list[dict[str, Any]]) -> bytes:
    return "".join(canonical_json(row) + "\n" for row in rows).encode("utf-8")


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def require_expected_hash(path: pathlib.Path, expected: str, label: str) -> str:
    bundle_validator.require_sha256(expected, f"expected_{label}_sha256")
    actual = sha256_file(path)
    if actual != expected:
        raise ValueError(
            f"{label} SHA-256 mismatch: expected={expected} actual={actual}"
        )
    return actual


def row_sort_key(row: dict[str, Any]) -> tuple[str, int, int, str]:
    return (
        row["owner_kind"],
        row["owner_id"],
        row["task_id"],
        row["entity_key"],
    )


def load_jsonl(path: pathlib.Path, label: str) -> list[dict[str, Any]]:
    if not path.is_file() or path.is_symlink():
        raise ValueError(f"{label} must be an existing non-symlink file")
    rows: list[dict[str, Any]] = []
    try:
        with path.open("r", encoding="utf-8") as handle:
            for line_no, line in enumerate(handle, 1):
                if not line.strip():
                    continue
                try:
                    value = json.loads(
                        line, object_pairs_hook=reject_duplicate_keys
                    )
                except json.JSONDecodeError as exc:
                    raise ValueError(
                        f"{label} row {line_no} is invalid JSON"
                    ) from exc
                if not isinstance(value, dict):
                    raise ValueError(f"{label} row {line_no} must be an object")
                rows.append(value)
    except UnicodeDecodeError as exc:
        raise ValueError(f"{label} must be UTF-8") from exc
    if not rows:
        raise ValueError(f"{label} contains no rows")
    return rows


def validate_unique_entities(rows: list[dict[str, Any]], label: str) -> None:
    seen: set[str] = set()
    for row in rows:
        entity = row.get("entity_key")
        if entity in seen:
            raise ValueError(f"{label} contains duplicate entity_key {entity}")
        seen.add(entity)


def split_source_rows(
    rows: list[dict[str, Any]],
    original_recovery_rows: list[dict[str, Any]],
) -> tuple[list[dict[str, Any]], dict[str, Any]]:
    validate_unique_entities(rows, "source manifest")
    tombstones = [row for row in rows if row.get("entity_key") == TOMBSTONE_ENTITY]
    if len(tombstones) != 1 or tombstones[0] != TOMBSTONE_CONTRACT:
        raise ValueError(
            "source manifest must contain the one exact task_asset:12323 tombstone"
        )
    expected_recoveries = {
        row["entity_key"]: row for row in original_recovery_rows
    }
    if set(expected_recoveries) != {
        f"task_asset:{owner_id}" for owner_id in recovery_contract.RECOVERY_IDS
    }:
        raise ValueError("recovery contract did not produce exactly three rows")
    actual_recoveries = {
        row["entity_key"]: row
        for row in rows
        if row.get("entity_key") in expected_recoveries
    }
    if actual_recoveries != expected_recoveries:
        raise ValueError(
            "source manifest must contain the three exact original-404 recovery rows"
        )
    prepared: list[dict[str, Any]] = []
    for line_no, row in enumerate(rows, 1):
        if (
            row["entity_key"] == TOMBSTONE_ENTITY
            or row["entity_key"] in expected_recoveries
        ):
            continue
        hydrator.validate_hydration_row(row, line_no)
        prepared.append(row)
    return sorted(prepared, key=row_sort_key), tombstones[0]


def prepare_manifest(
    source_path: pathlib.Path,
    expected_source_sha256: str,
    mapping_path: pathlib.Path,
    expected_mapping_sha256: str,
    recovery_plan_path: pathlib.Path,
    expected_recovery_plan_sha256: str,
) -> tuple[bytes, dict[str, Any]]:
    source_sha = require_expected_hash(
        source_path, expected_source_sha256, "source_manifest"
    )
    mapping_rows, plan_entries, recovery_hashes = recovery_contract.load_contract(
        mapping_path=mapping_path,
        expected_mapping_sha256=expected_mapping_sha256,
        plan_path=recovery_plan_path,
        expected_plan_sha256=expected_recovery_plan_sha256,
    )
    recovery_sources = recovery_contract.original_manifest_rows(
        mapping_rows, plan_entries
    )
    source_rows = load_jsonl(source_path, "source manifest")
    prepared, _tombstone = split_source_rows(source_rows, recovery_sources)
    output = canonical_jsonl(prepared)
    summary = {
        "schema_version": SCHEMA_VERSION,
        "status": "PASS",
        "operation": "prepare_g06_hydration_manifest",
        "source_manifest_sha256": source_sha,
        **recovery_hashes,
        "source_row_count": len(source_rows),
        "excluded_entity_keys": [
            TOMBSTONE_ENTITY,
            *[
                f"task_asset:{owner_id}"
                for owner_id in recovery_contract.RECOVERY_IDS
            ],
        ],
        "historical_unavailable_row_count": 1,
        "recovery_original_404_row_count": 3,
        "excluded_row_count": 4,
        "hydration_row_count": len(prepared),
        "hydration_manifest_sha256": sha256_bytes(output),
        "database_write_performed": False,
        "production_write_performed": False,
    }
    return output, summary


def verify_hydrated_rows(
    source_rows: list[dict[str, Any]],
    hydrated_rows: list[dict[str, Any]],
    original_recovery_rows: list[dict[str, Any]],
) -> list[dict[str, Any]]:
    expected_rows, _tombstone = split_source_rows(
        source_rows, original_recovery_rows
    )
    validate_unique_entities(hydrated_rows, "hydrated manifest")
    expected = {row["entity_key"]: row for row in expected_rows}
    actual = {row.get("entity_key"): row for row in hydrated_rows}
    if set(actual) != set(expected):
        missing = sorted(set(expected) - set(actual))
        extra = sorted(set(actual) - set(expected))
        raise ValueError(
            f"hydrated manifest entity set drifted: missing={missing} extra={extra}"
        )
    for line_no, entity in enumerate(sorted(actual), 1):
        row = actual[entity]
        problems = object_verifier.validate_contract(row, line_no)
        if problems:
            raise ValueError(
                f"hydrated manifest row {entity} is invalid: "
                f"{problems[0]['violation_code']}"
            )
        source = expected[entity]
        for field in object_verifier.REQUIRED_FIELDS - HYDRATABLE_FIELDS:
            if row[field] != source[field]:
                raise ValueError(
                    f"hydrated manifest changed immutable field {entity}.{field}"
                )
        if source.get("sha256") not in {"", None}:
            for field in HYDRATABLE_FIELDS:
                if row[field] != source[field]:
                    raise ValueError(
                        f"hydrated manifest changed complete field {entity}.{field}"
                    )
    return sorted(hydrated_rows, key=row_sort_key)


def bundle_rows(
    mapping_path: pathlib.Path,
    expected_mapping_sha256: str,
    manifest_path: pathlib.Path,
    expected_manifest_sha256: str,
    registry_path: pathlib.Path,
    expected_registry_sha256: str,
) -> tuple[
    list[dict[str, Any]],
    dict[str, str],
    str,
    dict[tuple[int, str, int, int], dict[str, Any]],
]:
    hashes = {
        "bundle_mapping_sha256": require_expected_hash(
            mapping_path, expected_mapping_sha256, "bundle_mapping"
        ),
        "bundle_manifest_sha256": require_expected_hash(
            manifest_path, expected_manifest_sha256, "bundle_manifest"
        ),
        "bundle_registry_sha256": require_expected_hash(
            registry_path, expected_registry_sha256, "bundle_registry"
        ),
    }
    mapping = bundle_validator.load_object(mapping_path, "bundle mapping")
    manifest = bundle_validator.load_object(manifest_path, "bundle manifest")
    registry = bundle_validator.load_object(registry_path, "bundle registry")
    # The mapping bytes are part of the review boundary even though this
    # operation only needs the resulting materialized registry rows.
    if mapping.get("version") != 2 or not isinstance(mapping.get("resources"), list):
        raise ValueError("bundle mapping must be a V2 mapping")
    confirmed, run_id = bundle_validator.validate_manifest(
        manifest, hashes["bundle_mapping_sha256"]
    )
    materialized = bundle_validator.validate_registry(
        registry,
        hashes["bundle_manifest_sha256"],
        run_id,
        confirmed,
    )
    if set(materialized) != set(bundle_validator.EXACT_SCOPES):
        raise ValueError("materialized bundle scopes are incomplete")

    entries_by_scope: dict[tuple[int, str, int, int], dict[str, Any]] = {}
    for entry in registry["entries"]:
        key = bundle_validator.scope_key(entry, "bundle registry entry")
        if key in entries_by_scope:
            raise ValueError(f"bundle registry duplicates scope {key}")
        entries_by_scope[key] = entry

    rows: list[dict[str, Any]] = []
    for key in sorted(bundle_validator.EXACT_SCOPES):
        entry = entries_by_scope[key]
        task_asset = entry["task_asset_candidate"]
        storage = entry["asset_storage_ref_candidate"]
        rows.append(
            {
                "entity_key": f"task_asset:{task_asset['id']}",
                "owner_kind": "task_asset",
                "owner_id": task_asset["id"],
                "task_id": task_asset["task_id"],
                "storage_ref_id": storage["ref_id"],
                "storage_adapter": "clone_b_bundle",
                "object_key": entry["object_key"],
                "size": entry["size"],
                "mime_type": "application/zip",
                "sha256": entry["bundle_sha256"],
                "status": "recorded",
                "is_placeholder": False,
            }
        )
    for line_no, row in enumerate(rows, 1):
        problems = object_verifier.validate_contract(row, line_no)
        if problems:
            raise ValueError(
                f"generated bundle row is invalid: {problems[0]['violation_code']}"
            )
    return rows, hashes, run_id, materialized


def validate_reviewed_bundle_semantics(
    reviewed_mapping: dict[str, Any],
    normalized: dict[tuple[int, str, int, int], dict[str, Any]],
) -> tuple[int, str]:
    """Bind the seven materialized bundles to the final-reviewed mapping."""
    if (
        reviewed_mapping.get("version") != 2
        or not isinstance(reviewed_mapping.get("resources"), list)
    ):
        raise ValueError("final-reviewed mapping must be a V2 mapping")
    mapped: dict[tuple[int, str, int, int], dict[str, Any]] = {}
    for resource in reviewed_mapping["resources"]:
        if not isinstance(resource, dict) or not isinstance(
            resource.get("history"), list
        ):
            raise ValueError("final-reviewed mapping resource history is invalid")
        for revision in resource["history"]:
            if not isinstance(revision, dict):
                raise ValueError(
                    "final-reviewed mapping revision must be an object"
                )
            source_bundle = revision.get("source_bundle")
            if source_bundle is None:
                continue
            key = (
                int(resource["task_id"]),
                str(resource["scope_kind"]),
                int(resource["scope_ref_id"]),
                int(revision["revision_no"]),
            )
            if key in mapped:
                raise ValueError(
                    f"final-reviewed mapping duplicates bundle scope {key}"
                )
            mapped[key] = source_bundle
    expected_scopes = set(bundle_validator.EXACT_SCOPES)
    if set(normalized) != expected_scopes:
        raise ValueError(
            "validated bundle registry does not contain the exact seven scopes"
        )
    if set(mapped) != expected_scopes:
        raise ValueError(
            "final-reviewed mapping and bundle registry scopes differ"
        )
    semantic_rows: list[dict[str, Any]] = []
    for key in sorted(expected_scopes):
        if mapped[key] != normalized[key]:
            raise ValueError(
                f"final-reviewed mapping source_bundle drifted for {key}"
            )
        semantic_rows.append(
            {
                "task_id": key[0],
                "scope_kind": key[1],
                "scope_ref_id": key[2],
                "revision_no": key[3],
                "source_bundle": mapped[key],
            }
        )
    return len(semantic_rows), sha256_bytes(
        canonical_json(semantic_rows).encode("utf-8")
    )


def ensure_new_bundle_identifiers(
    existing: list[dict[str, Any]], additions: list[dict[str, Any]]
) -> None:
    # Historical manifests legitimately share object keys and storage refs.
    # New materialized bundles must not collide with either history or one
    # another, however.
    for field in ("entity_key", "storage_ref_id", "object_key"):
        old_values = {row[field] for row in existing}
        new_values = [row[field] for row in additions]
        if len(new_values) != len(set(new_values)):
            raise ValueError(f"bundle rows contain duplicate {field}")
        collisions = sorted(old_values.intersection(new_values))
        if collisions:
            raise ValueError(f"bundle rows collide on {field}: {collisions}")


def finalize_manifest(
    *,
    source_path: pathlib.Path,
    expected_source_sha256: str,
    hydrated_path: pathlib.Path,
    expected_hydrated_sha256: str,
    recovery_mapping_path: pathlib.Path,
    expected_recovery_mapping_sha256: str,
    bundle_mapping_path: pathlib.Path,
    expected_bundle_mapping_sha256: str,
    bundle_manifest_path: pathlib.Path,
    expected_bundle_manifest_sha256: str,
    registry_path: pathlib.Path,
    expected_registry_sha256: str,
    recovery_plan_path: pathlib.Path,
    expected_recovery_plan_sha256: str,
) -> tuple[bytes, dict[str, Any]]:
    source_sha = require_expected_hash(
        source_path, expected_source_sha256, "source_manifest"
    )
    hydrated_sha = require_expected_hash(
        hydrated_path, expected_hydrated_sha256, "hydrated_manifest"
    )
    mapping_rows, plan_entries, recovery_hashes = recovery_contract.load_contract(
        mapping_path=recovery_mapping_path,
        expected_mapping_sha256=expected_recovery_mapping_sha256,
        plan_path=recovery_plan_path,
        expected_plan_sha256=expected_recovery_plan_sha256,
    )
    recovery_sources = recovery_contract.original_manifest_rows(
        mapping_rows, plan_entries
    )
    source_rows = load_jsonl(source_path, "source manifest")
    hydrated_rows = verify_hydrated_rows(
        source_rows,
        load_jsonl(hydrated_path, "hydrated manifest"),
        recovery_sources,
    )
    _prepared, tombstone = split_source_rows(source_rows, recovery_sources)
    recovery_rows = recovery_contract.recovery_manifest_rows(
        mapping_rows, plan_entries
    )
    for line_no, row in enumerate(recovery_rows, 1):
        problems = object_verifier.validate_contract(row, line_no)
        if problems:
            raise ValueError(
                "generated recovery row is invalid: "
                f"{problems[0]['violation_code']}"
            )
    additions, bundle_hashes, run_id, normalized = bundle_rows(
        bundle_mapping_path,
        expected_bundle_mapping_sha256,
        bundle_manifest_path,
        expected_bundle_manifest_sha256,
        registry_path,
        expected_registry_sha256,
    )
    reviewed_mapping = bundle_validator.load_object(
        recovery_mapping_path, "final-reviewed mapping"
    )
    reviewed_bundle_scope_count, reviewed_bundle_semantics_sha256 = (
        validate_reviewed_bundle_semantics(reviewed_mapping, normalized)
    )
    base_rows = hydrated_rows + [tombstone]
    ensure_new_bundle_identifiers(base_rows, recovery_rows + additions)
    final_rows = sorted(
        base_rows + recovery_rows + additions, key=row_sort_key
    )
    validate_unique_entities(final_rows, "final manifest")
    output = canonical_jsonl(final_rows)
    summary = {
        "schema_version": SCHEMA_VERSION,
        "status": "PASS",
        "operation": "finalize_g06_object_manifest",
        "source_manifest_sha256": source_sha,
        "hydrated_manifest_sha256": hydrated_sha,
        **recovery_hashes,
        **bundle_hashes,
        "bundle_run_id": run_id,
        "reviewed_bundle_scope_count": reviewed_bundle_scope_count,
        "reviewed_bundle_semantics_sha256": (
            reviewed_bundle_semantics_sha256
        ),
        "hydrated_row_count": len(hydrated_rows),
        "historical_unavailable_row_count": 1,
        "recovery_row_count": len(recovery_rows),
        "bundle_row_count": len(additions),
        "final_row_count": len(final_rows),
        "final_manifest_sha256": sha256_bytes(output),
        "database_write_performed": False,
        "production_write_performed": False,
    }
    return output, summary


def atomic_write_many(outputs: list[tuple[pathlib.Path, bytes]]) -> None:
    resolved = [path.resolve() for path, _data in outputs]
    if len(resolved) != len(set(resolved)):
        raise ValueError("output paths must be distinct")
    pending: list[tuple[pathlib.Path, pathlib.Path]] = []
    try:
        for path, data in outputs:
            path.parent.mkdir(parents=True, exist_ok=True)
            if path.exists():
                if path.is_file() and not path.is_symlink() and path.read_bytes() == data:
                    continue
                raise FileExistsError(f"refusing to overwrite different output: {path}")
            with tempfile.NamedTemporaryFile(
                dir=path.parent,
                prefix=path.name + ".",
                suffix=".tmp",
                delete=False,
            ) as handle:
                temporary = pathlib.Path(handle.name)
                handle.write(data)
                handle.flush()
                os.fsync(handle.fileno())
            pending.append((temporary, path))
        for temporary, path in pending:
            os.replace(temporary, path)
    finally:
        for temporary, _path in pending:
            temporary.unlink(missing_ok=True)


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    prepare = subparsers.add_parser("prepare")
    prepare.add_argument("--source-manifest", type=pathlib.Path, required=True)
    prepare.add_argument("--expected-source-sha256", required=True)
    prepare.add_argument("--mapping", type=pathlib.Path, required=True)
    prepare.add_argument("--expected-mapping-sha256", required=True)
    prepare.add_argument(
        "--recovery-plan", type=pathlib.Path, required=True
    )
    prepare.add_argument("--expected-recovery-plan-sha256", required=True)
    prepare.add_argument("--output", type=pathlib.Path, required=True)
    prepare.add_argument("--summary", type=pathlib.Path, required=True)

    finalize = subparsers.add_parser("finalize")
    finalize.add_argument("--source-manifest", type=pathlib.Path, required=True)
    finalize.add_argument("--expected-source-sha256", required=True)
    finalize.add_argument("--hydrated-manifest", type=pathlib.Path, required=True)
    finalize.add_argument("--expected-hydrated-sha256", required=True)
    finalize.add_argument("--recovery-mapping", type=pathlib.Path, required=True)
    finalize.add_argument(
        "--expected-recovery-mapping-sha256", required=True
    )
    finalize.add_argument("--bundle-mapping", type=pathlib.Path, required=True)
    finalize.add_argument("--expected-bundle-mapping-sha256", required=True)
    finalize.add_argument("--bundle-manifest", type=pathlib.Path, required=True)
    finalize.add_argument("--expected-bundle-manifest-sha256", required=True)
    finalize.add_argument("--bundle-registry", type=pathlib.Path, required=True)
    finalize.add_argument("--expected-bundle-registry-sha256", required=True)
    finalize.add_argument(
        "--recovery-plan", type=pathlib.Path, required=True
    )
    finalize.add_argument("--expected-recovery-plan-sha256", required=True)
    finalize.add_argument("--output", type=pathlib.Path, required=True)
    finalize.add_argument("--summary", type=pathlib.Path, required=True)
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    expected_recovery_mapping_sha256 = (
        args.expected_mapping_sha256
        if args.command == "prepare"
        else args.expected_recovery_mapping_sha256
    )
    recovery_contract.require_frozen_hashes(
        expected_recovery_mapping_sha256,
        args.expected_recovery_plan_sha256,
    )
    if args.command == "prepare":
        output, summary = prepare_manifest(
            args.source_manifest,
            args.expected_source_sha256,
            args.mapping,
            args.expected_mapping_sha256,
            args.recovery_plan,
            args.expected_recovery_plan_sha256,
        )
        input_paths = {
            args.source_manifest.resolve(),
            args.mapping.resolve(),
            args.recovery_plan.resolve(),
        }
    else:
        output, summary = finalize_manifest(
            source_path=args.source_manifest,
            expected_source_sha256=args.expected_source_sha256,
            hydrated_path=args.hydrated_manifest,
            expected_hydrated_sha256=args.expected_hydrated_sha256,
            recovery_mapping_path=args.recovery_mapping,
            expected_recovery_mapping_sha256=(
                args.expected_recovery_mapping_sha256
            ),
            bundle_mapping_path=args.bundle_mapping,
            expected_bundle_mapping_sha256=(
                args.expected_bundle_mapping_sha256
            ),
            bundle_manifest_path=args.bundle_manifest,
            expected_bundle_manifest_sha256=args.expected_bundle_manifest_sha256,
            registry_path=args.bundle_registry,
            expected_registry_sha256=args.expected_bundle_registry_sha256,
            recovery_plan_path=args.recovery_plan,
            expected_recovery_plan_sha256=(
                args.expected_recovery_plan_sha256
            ),
        )
        input_paths = {
            args.source_manifest.resolve(),
            args.hydrated_manifest.resolve(),
            args.recovery_mapping.resolve(),
            args.bundle_mapping.resolve(),
            args.bundle_manifest.resolve(),
            args.bundle_registry.resolve(),
            args.recovery_plan.resolve(),
        }
    if args.output.resolve() in input_paths or args.summary.resolve() in input_paths:
        raise ValueError("outputs must not overwrite inputs")
    atomic_write_many(
        [
            (args.output, output),
            (args.summary, (canonical_json(summary) + "\n").encode("utf-8")),
        ]
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
