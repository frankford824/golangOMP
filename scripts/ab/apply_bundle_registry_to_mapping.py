#!/usr/bin/env python3
"""Merge reviewed, materialized source bundles into an exact V8 mapping.

This is a file-only bridge between ``run_scoped_bundle_materializer.py`` and
``workflow-groups-migrate``.  It deliberately does not approve business
policies: resolving the physical multi-source blocker changes a target row to
``proposed_review`` with its zero reviewer metadata intact.  A later,
independent policy review must supply the revision confirmation.
"""

from __future__ import annotations

import argparse
import copy
import hashlib
import json
import os
import pathlib
import re
import tempfile
from typing import Any


SHA256 = re.compile(r"^[0-9a-f]{64}$")
ZERO_TIME = "0001-01-01T00:00:00Z"
MULTI_SOURCE_BLOCKERS = {
    "multiple source assets require a reviewed deterministic ZIP bundle",
    "design revision has no uniquely evidenced source asset",
}
ALLOWED_SCOPE_KINDS = {"task", "sku", "retouch_requirement"}
# Historical compatibility contract used only by
# ``reapply_approved_source_bundles.py`` and legacy G06 fixtures.  The current
# registry bridge derives its complete scope set from the candidate mapping
# and does not consult this constant.
EXACT_SCOPES: dict[tuple[int, str, int, int], tuple[int, ...]] = {
    (485, "sku", 365, 1): (293, 297),
    (523, "sku", 398, 1): (402, 403, 404, 405),
    (523, "sku", 400, 1): (358, 359, 360, 361),
    (2234, "sku", 2401, 2): (12672, 12673),
    (2251, "sku", 2417, 2): (13103, 13104, 13105, 13106, 13107),
    (2477, "sku", 2725, 2): (18989, 18991, 18993),
    (2598, "sku", 2869, 2): (20799, 20802),
}


def canonical_bytes(value: object) -> bytes:
    return (
        json.dumps(
            value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
        )
        + "\n"
    ).encode("utf-8")


def canonical_hash(value: object) -> str:
    return hashlib.sha256(canonical_bytes(value).rstrip(b"\n")).hexdigest()


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def reject_duplicate_keys(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate JSON object key: {key}")
        result[key] = value
    return result


def load_object(path: pathlib.Path, label: str) -> dict[str, Any]:
    if not path.is_file() or path.is_symlink():
        raise ValueError(f"{label} must be an existing non-symlink file")
    try:
        value = json.loads(
            path.read_text(encoding="utf-8"),
            object_pairs_hook=reject_duplicate_keys,
        )
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise ValueError(f"{label} is not valid UTF-8 JSON: {exc}") from exc
    if not isinstance(value, dict):
        raise ValueError(f"{label} must be a JSON object")
    return value


def require_sha256(value: Any, path: str) -> str:
    if not isinstance(value, str) or not SHA256.fullmatch(value):
        raise ValueError(f"{path} must be lowercase SHA-256")
    return value


def require_positive_int(value: Any, path: str) -> int:
    if isinstance(value, bool) or not isinstance(value, int) or value <= 0:
        raise ValueError(f"{path} must be a positive integer")
    return value


def scope_key(value: dict[str, Any], path: str) -> tuple[int, str, int, int]:
    task_id = require_positive_int(value.get("task_id"), f"{path}.task_id")
    kind = value.get("scope_kind")
    ref_id = value.get("scope_ref_id")
    revision_no = require_positive_int(
        value.get("revision_no"), f"{path}.revision_no"
    )
    if kind not in ALLOWED_SCOPE_KINDS:
        raise ValueError(f"{path}.scope_kind is invalid")
    if (
        isinstance(ref_id, bool)
        or not isinstance(ref_id, int)
        or (kind == "task" and ref_id != 0)
        or (kind != "task" and ref_id <= 0)
    ):
        raise ValueError(f"{path}.scope_ref_id is invalid")
    return task_id, kind, ref_id, revision_no


def mapping_bundle_scopes(
    mapping: dict[str, Any],
) -> dict[tuple[int, str, int, int], tuple[int, ...]]:
    if mapping.get("version") != 2 or not isinstance(
        mapping.get("resources"), list
    ):
        raise ValueError("mapping must be version 2 with a resources array")
    result: dict[tuple[int, str, int, int], tuple[int, ...]] = {}
    for resource_index, resource in enumerate(mapping["resources"]):
        path = f"resources[{resource_index}]"
        if not isinstance(resource, dict) or not isinstance(
            resource.get("history"), list
        ):
            raise ValueError(f"{path}.history must be an array")
        for revision_index, revision in enumerate(resource["history"]):
            revision_path = f"{path}.history[{revision_index}]"
            if not isinstance(revision, dict):
                raise ValueError(f"{revision_path} must be an object")
            candidate = revision.get("source_bundle_candidate")
            if candidate is None:
                continue
            key = scope_key(
                {
                    "task_id": resource.get("task_id"),
                    "scope_kind": resource.get("scope_kind"),
                    "scope_ref_id": resource.get("scope_ref_id"),
                    "revision_no": revision.get("revision_no"),
                },
                revision_path,
            )
            blockers = revision.get("blockers")
            members = (
                candidate.get("ordered_member_task_asset_ids")
                if isinstance(candidate, dict)
                else None
            )
            if (
                key in result
                or revision.get("confidence") != "hard_blocked"
                or not isinstance(blockers, list)
                or not blockers
                or any(
                    blocker not in MULTI_SOURCE_BLOCKERS
                    for blocker in blockers
                )
                or "multiple source assets require a reviewed deterministic ZIP bundle"
                not in blockers
                or not isinstance(candidate, dict)
                or candidate.get("ordering")
                != "completion_time_then_task_asset_id"
                or not isinstance(members, list)
                or len(members) < 2
                or any(
                    isinstance(member, bool)
                    or not isinstance(member, int)
                    or member <= 0
                    for member in members
                )
                or len(members) != len(set(members))
            ):
                raise ValueError(
                    f"{revision_path} has an invalid bundle candidate"
                )
            result[key] = tuple(members)
    if not result:
        raise ValueError("mapping contains no deterministic bundle candidates")
    return result


def manifest_hash_for_source_bundle(source_bundle: dict[str, Any]) -> str:
    return canonical_hash(
        {
            "format": source_bundle["format"],
            "members": source_bundle["members"],
        }
    )


def validate_source_bundle(
    source_bundle: Any,
    expected_members: tuple[int, ...],
    manifest_bundle: dict[str, Any],
    path: str,
) -> dict[str, Any]:
    if not isinstance(source_bundle, dict):
        raise ValueError(f"{path} must be an object")
    task_asset_id = require_positive_int(
        source_bundle.get("task_asset_id"), f"{path}.task_asset_id"
    )
    if task_asset_id != manifest_bundle["bundle_task_asset_id"]:
        raise ValueError(f"{path}.task_asset_id drifted from confirmed manifest")
    if source_bundle.get("format") != "zip":
        raise ValueError(f"{path}.format must be zip")
    bundle_sha = require_sha256(
        source_bundle.get("bundle_sha256"), f"{path}.bundle_sha256"
    )
    require_sha256(
        source_bundle.get("manifest_sha256"), f"{path}.manifest_sha256"
    )
    members = source_bundle.get("members")
    if not isinstance(members, list):
        raise ValueError(f"{path}.members must be an array")
    member_ids: list[int] = []
    member_hashes: list[str] = []
    for index, member in enumerate(members):
        member_path = f"{path}.members[{index}]"
        if not isinstance(member, dict) or member.get("confirmed") is not True:
            raise ValueError(f"{member_path} must be confirmed=true")
        member_ids.append(
            require_positive_int(
                member.get("task_asset_id"), f"{member_path}.task_asset_id"
            )
        )
        member_hashes.append(
            require_sha256(member.get("sha256"), f"{member_path}.sha256")
        )
    if tuple(member_ids) != expected_members:
        raise ValueError(f"{path}.members order/identity drifted")
    confirmed_members = manifest_bundle["ordered_members"]
    if member_hashes != [member["sha256"] for member in confirmed_members]:
        raise ValueError(f"{path}.members SHA-256 drifted from confirmed manifest")
    expected_manifest_hash = manifest_hash_for_source_bundle(source_bundle)
    if source_bundle["manifest_sha256"] != expected_manifest_hash:
        raise ValueError(f"{path}.manifest_sha256 is stale")
    reviewer = require_positive_int(
        source_bundle.get("confirmed_by"), f"{path}.confirmed_by"
    )
    if reviewer != manifest_bundle["_confirmed_by"]:
        raise ValueError(f"{path}.confirmed_by drifted from confirmed manifest")
    if (
        source_bundle.get("confirmed_at") != manifest_bundle["_confirmed_at"]
        or source_bundle.get("confirmation_note")
        != manifest_bundle["_confirmation_note"]
    ):
        raise ValueError(f"{path} confirmation metadata drifted")
    # Return only the mapping contract.  Unknown registry fields never leak.
    return {
        "task_asset_id": task_asset_id,
        "format": "zip",
        "bundle_sha256": bundle_sha,
        "manifest_sha256": source_bundle["manifest_sha256"],
        "members": copy.deepcopy(members),
        "confirmed_by": reviewer,
        "confirmed_at": source_bundle["confirmed_at"],
        "confirmation_note": source_bundle["confirmation_note"],
    }


def validate_manifest(
    manifest: dict[str, Any],
    mapping_sha256: str,
    expected_scopes: dict[
        tuple[int, str, int, int], tuple[int, ...]
    ],
) -> tuple[dict[tuple[int, str, int, int], dict[str, Any]], str]:
    if manifest.get("schema_version") != 1 or manifest.get("status") != "CONFIRMED":
        raise ValueError("manifest must be schema_version=1 and status=CONFIRMED")
    if manifest.get("mapping_sha256") != mapping_sha256:
        raise ValueError("manifest.mapping_sha256 does not bind the input mapping")
    require_sha256(
        manifest.get("source_candidate_sha256"),
        "manifest.source_candidate_sha256",
    )
    reviewer = require_positive_int(
        manifest.get("confirmed_by"), "manifest.confirmed_by"
    )
    confirmed_at = manifest.get("confirmed_at")
    note = manifest.get("confirmation_note")
    if (
        not isinstance(confirmed_at, str)
        or not confirmed_at.strip()
        or confirmed_at == ZERO_TIME
        or not isinstance(note, str)
        or not note.strip()
    ):
        raise ValueError("manifest confirmation metadata is incomplete")
    run_id = manifest.get("run_id")
    if not isinstance(run_id, str) or not run_id.strip():
        raise ValueError("manifest.run_id is required")
    bundles = manifest.get("bundles")
    if not isinstance(bundles, list):
        raise ValueError("manifest.bundles must be an array")
    result: dict[tuple[int, str, int, int], dict[str, Any]] = {}
    seen_bundle_task_assets: set[int] = set()
    seen_bundle_assets: set[int] = set()
    seen_storage_refs: set[str] = set()
    for index, bundle in enumerate(bundles):
        path = f"manifest.bundles[{index}]"
        if not isinstance(bundle, dict) or bundle.get("confirmed") is not True:
            raise ValueError(f"{path} must be confirmed=true")
        key = scope_key(bundle, path)
        if key not in expected_scopes or key in result:
            raise ValueError(f"{path} has an unexpected or duplicate scope")
        task_asset_id = require_positive_int(
            bundle.get("bundle_task_asset_id"),
            f"{path}.bundle_task_asset_id",
        )
        asset_id = require_positive_int(
            bundle.get("bundle_asset_id"), f"{path}.bundle_asset_id"
        )
        storage_ref = bundle.get("bundle_storage_ref_id")
        if not isinstance(storage_ref, str) or not storage_ref.strip():
            raise ValueError(f"{path}.bundle_storage_ref_id is required")
        if (
            task_asset_id in seen_bundle_task_assets
            or asset_id in seen_bundle_assets
            or storage_ref in seen_storage_refs
        ):
            raise ValueError(f"{path} reuses a bundle identifier")
        seen_bundle_task_assets.add(task_asset_id)
        seen_bundle_assets.add(asset_id)
        seen_storage_refs.add(storage_ref)
        members = bundle.get("ordered_members")
        if not isinstance(members, list):
            raise ValueError(f"{path}.ordered_members must be an array")
        ids: list[int] = []
        for member_index, member in enumerate(members):
            member_path = f"{path}.ordered_members[{member_index}]"
            if not isinstance(member, dict) or member.get("confirmed") is not True:
                raise ValueError(f"{member_path} must be confirmed=true")
            if member.get("task_id") != key[0]:
                raise ValueError(f"{member_path}.task_id drifted from scope")
            ids.append(
                require_positive_int(
                    member.get("task_asset_id"),
                    f"{member_path}.task_asset_id",
                )
            )
            require_positive_int(member.get("asset_id"), f"{member_path}.asset_id")
            require_sha256(member.get("sha256"), f"{member_path}.sha256")
            if not isinstance(member.get("storage_ref_id"), str) or not member[
                "storage_ref_id"
            ].strip():
                raise ValueError(f"{member_path}.storage_ref_id is required")
        if tuple(ids) != expected_scopes[key]:
            raise ValueError(f"{path}.ordered_members order/identity drifted")
        if task_asset_id in ids:
            raise ValueError(f"{path} bundle output reuses a source member id")
        normalized = copy.deepcopy(bundle)
        normalized["_confirmed_by"] = reviewer
        normalized["_confirmed_at"] = confirmed_at
        normalized["_confirmation_note"] = note
        result[key] = normalized
    if set(result) != set(expected_scopes):
        raise ValueError(
            "manifest scopes differ from the mapping bundle candidates"
        )
    return result, run_id


def validate_registry(
    registry: dict[str, Any],
    manifest_sha256: str,
    run_id: str,
    manifest_bundles: dict[tuple[int, str, int, int], dict[str, Any]],
    expected_scopes: dict[
        tuple[int, str, int, int], tuple[int, ...]
    ],
) -> dict[tuple[int, str, int, int], dict[str, Any]]:
    if (
        registry.get("schema_version") != 1
        or registry.get("status") != "MATERIALIZED"
        or registry.get("database_write_performed") is not False
    ):
        raise ValueError(
            "registry must be schema_version=1, status=MATERIALIZED, file-only"
        )
    if registry.get("run_id") != run_id:
        raise ValueError("registry.run_id drifted from confirmed manifest")
    if registry.get("manifest_sha256") != manifest_sha256:
        raise ValueError("registry.manifest_sha256 drifted from manifest bytes")
    entries = registry.get("entries")
    if not isinstance(entries, list):
        raise ValueError("registry.entries must be an array")
    result: dict[tuple[int, str, int, int], dict[str, Any]] = {}
    for index, entry in enumerate(entries):
        path = f"registry.entries[{index}]"
        if not isinstance(entry, dict):
            raise ValueError(f"{path} must be an object")
        key = scope_key(entry, path)
        if key not in expected_scopes or key in result:
            raise ValueError(f"{path} has an unexpected or duplicate scope")
        manifest_bundle = manifest_bundles[key]
        source_bundle = validate_source_bundle(
            entry.get("source_bundle"),
            expected_scopes[key],
            manifest_bundle,
            f"{path}.source_bundle",
        )
        bundle_sha = source_bundle["bundle_sha256"]
        if entry.get("bundle_sha256") != bundle_sha:
            raise ValueError(f"{path}.bundle_sha256 drifted")
        size = require_positive_int(entry.get("size"), f"{path}.size")
        object_key = entry.get("object_key")
        relative_path = entry.get("relative_object_path")
        expected_object_key = (
            f"fixture/{run_id}/migration-bundles/task-{key[0]}/"
            f"{key[1]}-{key[2]}/revision-{key[3]}/source-bundle.zip"
        )
        if (
            object_key != expected_object_key
            or not isinstance(relative_path, str)
            or relative_path != f"objects/{object_key}"
        ):
            raise ValueError(f"{path} object path is invalid")
        if entry.get("disposition") not in {"created", "reused_identical"}:
            raise ValueError(f"{path}.disposition is invalid")
        storage = entry.get("asset_storage_ref_candidate")
        task_asset = entry.get("task_asset_candidate")
        rollback = entry.get("rollback_candidate")
        if not all(isinstance(value, dict) for value in (storage, task_asset, rollback)):
            raise ValueError(f"{path} candidates must be objects")
        expected_storage_ref = manifest_bundle["bundle_storage_ref_id"]
        if (
            storage.get("ref_id") != expected_storage_ref
            or storage.get("storage_adapter") != "upload_service"
            or storage.get("ref_key") != object_key
            or storage.get("file_name") != "source-bundle.zip"
            or storage.get("file_size") != size
            or storage.get("checksum_hint") != bundle_sha
            or storage.get("mime_type") != "application/zip"
            or storage.get("status") != "recorded"
            or storage.get("is_placeholder") is not False
        ):
            raise ValueError(f"{path}.asset_storage_ref_candidate drifted")
        if (
            task_asset.get("id") != source_bundle["task_asset_id"]
            or task_asset.get("task_id") != key[0]
            or task_asset.get("asset_id") != manifest_bundle["bundle_asset_id"]
            or task_asset.get("asset_type") != "source"
            or task_asset.get("scope_kind") != key[1]
            or task_asset.get("scope_ref_id") != key[2]
            or task_asset.get("storage_ref_id") != expected_storage_ref
            or task_asset.get("file_name") != "source-bundle.zip"
            or task_asset.get("file_size") != size
            or task_asset.get("storage_key") != object_key
            or task_asset.get("whole_hash") != bundle_sha
            or task_asset.get("mime_type") != "application/zip"
            or task_asset.get("upload_status") != "uploaded"
            or task_asset.get("source_module_key") != "migration"
        ):
            raise ValueError(f"{path}.task_asset_candidate drifted")
        if (
            rollback.get("task_asset_id") != source_bundle["task_asset_id"]
            or rollback.get("storage_ref_id") != expected_storage_ref
            or rollback.get("relative_object_path") != relative_path
            or rollback.get("expected_sha256") != bundle_sha
        ):
            raise ValueError(f"{path}.rollback_candidate drifted")
        result[key] = source_bundle
    if set(result) != set(expected_scopes):
        raise ValueError(
            "registry scopes differ from the mapping bundle candidates"
        )
    return result


def revision_hash(revision: dict[str, Any]) -> str:
    return canonical_hash(
        {
            key: value
            for key, value in revision.items()
            if key not in {"manifest_row_hash", "_blockers"}
        }
    )


def merge_mapping(
    mapping: dict[str, Any],
    source_bundles: dict[tuple[int, str, int, int], dict[str, Any]],
    expected_scopes: dict[
        tuple[int, str, int, int], tuple[int, ...]
    ],
) -> tuple[dict[str, Any], list[dict[str, Any]]]:
    if mapping.get("version") != 2 or not isinstance(mapping.get("resources"), list):
        raise ValueError("mapping must be version 2 with a resources array")
    output = copy.deepcopy(mapping)
    targets: dict[tuple[int, str, int, int], dict[str, Any]] = {}
    for resource_index, resource in enumerate(output["resources"]):
        path = f"resources[{resource_index}]"
        if not isinstance(resource, dict) or not isinstance(
            resource.get("history"), list
        ):
            raise ValueError(f"{path}.history must be an array")
        task_id = resource.get("task_id")
        kind = resource.get("scope_kind")
        ref_id = resource.get("scope_ref_id")
        for revision_index, revision in enumerate(resource["history"]):
            if not isinstance(revision, dict):
                raise ValueError(f"{path}.history[{revision_index}] must be an object")
            key = (task_id, kind, ref_id, revision.get("revision_no"))
            if key not in expected_scopes:
                continue
            if key in targets:
                raise ValueError(f"mapping duplicates frozen scope {key}")
            targets[key] = revision
    if set(targets) != set(expected_scopes):
        raise ValueError(
            "mapping revisions differ from the frozen bundle candidates"
        )

    evidence_rows = []
    for key in expected_scopes:
        revision = targets[key]
        prior_hash = require_sha256(
            revision.get("manifest_row_hash"),
            f"revision {key}.manifest_row_hash",
        )
        if revision_hash(revision) != prior_hash:
            raise ValueError(f"revision {key} manifest_row_hash is stale")
        blockers = revision.get("blockers")
        if (
            revision.get("confidence") != "hard_blocked"
            or not isinstance(blockers, list)
            or not blockers
            or len(blockers) != len(set(blockers))
            or "multiple source assets require a reviewed deterministic ZIP bundle"
            not in blockers
            or any(blocker not in MULTI_SOURCE_BLOCKERS for blocker in blockers)
        ):
            raise ValueError(
                f"revision {key} is not the exact multi-source hard blocker"
            )
        if revision.get("source_bundle") is not None:
            raise ValueError(f"revision {key} already has source_bundle")
        if revision.get("source_task_asset_id") is not None:
            raise ValueError(f"revision {key} has an unexpected source_task_asset_id")
        if (
            revision.get("confirmed_by") != 0
            or revision.get("confirmed_at") != ZERO_TIME
            or revision.get("confirmation_note") not in {"", None}
        ):
            raise ValueError(
                f"revision {key} has unexpected business reviewer metadata"
            )
        displaced_alias = revision.pop("source_alias_from_task_asset_id", None)
        revision["source_bundle"] = copy.deepcopy(source_bundles[key])
        # Proposal-only metadata is consumed once the confirmed immutable
        # bundle exists. The Go migration contract accepts the resulting
        # source_bundle snapshot, not its candidate.
        revision.pop("source_bundle_candidate", None)
        revision.pop("blockers", None)
        revision["confidence"] = "proposed_review"
        revision["confirmed_by"] = 0
        revision["confirmed_at"] = ZERO_TIME
        revision["confirmation_note"] = ""
        revision["manifest_row_hash"] = revision_hash(revision)
        evidence_rows.append(
            {
                "task_id": key[0],
                "scope_kind": key[1],
                "scope_ref_id": key[2],
                "revision_no": key[3],
                "ordered_member_task_asset_ids": list(
                    expected_scopes[key]
                ),
                "bundle_task_asset_id": source_bundles[key]["task_asset_id"],
                "bundle_sha256": source_bundles[key]["bundle_sha256"],
                "prior_manifest_row_hash": prior_hash,
                "output_manifest_row_hash": revision["manifest_row_hash"],
                "removed_blockers": list(blockers),
                "displaced_source_alias_from_task_asset_id": displaced_alias,
                "output_confidence": "proposed_review",
                "business_policy_review_required": True,
            }
        )
    return output, evidence_rows


def prepare_outputs(
    mapping_path: pathlib.Path,
    manifest_path: pathlib.Path,
    registry_path: pathlib.Path,
    expected_mapping_sha256: str,
    expected_manifest_sha256: str,
    expected_registry_sha256: str,
) -> tuple[dict[str, Any], dict[str, Any]]:
    actual = {
        "mapping_sha256": sha256_file(mapping_path),
        "manifest_sha256": sha256_file(manifest_path),
        "registry_sha256": sha256_file(registry_path),
    }
    expected = {
        "mapping_sha256": require_sha256(
            expected_mapping_sha256, "expected_mapping_sha256"
        ),
        "manifest_sha256": require_sha256(
            expected_manifest_sha256, "expected_manifest_sha256"
        ),
        "registry_sha256": require_sha256(
            expected_registry_sha256, "expected_registry_sha256"
        ),
    }
    if actual != expected:
        raise ValueError(f"input SHA-256 mismatch: expected={expected} actual={actual}")
    mapping = load_object(mapping_path, "mapping")
    manifest = load_object(manifest_path, "manifest")
    registry = load_object(registry_path, "registry")
    expected_scopes = mapping_bundle_scopes(mapping)
    manifest_bundles, run_id = validate_manifest(
        manifest,
        actual["mapping_sha256"],
        expected_scopes,
    )
    source_bundles = validate_registry(
        registry,
        actual["manifest_sha256"],
        run_id,
        manifest_bundles,
        expected_scopes,
    )
    output, rows = merge_mapping(
        mapping,
        source_bundles,
        expected_scopes,
    )
    output_bytes = canonical_bytes(output)
    evidence = {
        "schema_version": 1,
        "status": "PASS",
        "operation": "apply_materialized_bundle_registry_to_mapping",
        "run_id": run_id,
        "input_sha256": actual,
        "output_mapping_sha256": hashlib.sha256(output_bytes).hexdigest(),
        "target_count": len(rows),
        "targets": rows,
        "allowed_removed_blockers": sorted(MULTI_SOURCE_BLOCKERS),
        "database_write_performed": False,
        "business_policy_review_performed": False,
        "all_targets_require_business_policy_review": True,
    }
    return output, evidence


def atomic_write_many(outputs: list[tuple[pathlib.Path, bytes]]) -> None:
    resolved = [path.resolve() for path, _ in outputs]
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
        for temporary, _ in pending:
            temporary.unlink(missing_ok=True)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mapping", type=pathlib.Path, required=True)
    parser.add_argument("--manifest", type=pathlib.Path, required=True)
    parser.add_argument("--registry", type=pathlib.Path, required=True)
    parser.add_argument("--expected-mapping-sha256", required=True)
    parser.add_argument("--expected-manifest-sha256", required=True)
    parser.add_argument("--expected-registry-sha256", required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--evidence", type=pathlib.Path, required=True)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    input_paths = {
        args.mapping.resolve(),
        args.manifest.resolve(),
        args.registry.resolve(),
    }
    if args.output.resolve() in input_paths or args.evidence.resolve() in input_paths:
        raise ValueError("outputs must not overwrite any input")
    output, evidence = prepare_outputs(
        args.mapping,
        args.manifest,
        args.registry,
        args.expected_mapping_sha256,
        args.expected_manifest_sha256,
        args.expected_registry_sha256,
    )
    atomic_write_many(
        [
            (args.output, canonical_bytes(output)),
            (args.evidence, canonical_bytes(evidence)),
        ]
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
