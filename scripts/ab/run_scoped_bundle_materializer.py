#!/usr/bin/env python3
"""Prepare, materialize, and clean reviewed source bundles in Clone B only.

The tool is deliberately file-only. It never connects to MySQL and never
uploads to an external service. ``prepare`` validates an administrator-
confirmed manifest and every source byte below a frozen run-scoped seed root.
``materialize --execute`` writes deterministic ZIPs only below
``<run-root>/fixture-upload-b/objects`` and emits DB insertion candidates plus
an exact cleanup registry. ``rollback --execute`` removes only registry-listed
ZIPs whose hashes still match.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import re
import tempfile
from typing import Any

try:
    from scripts.ab import deterministic_source_bundle as bundle_builder
except ModuleNotFoundError:
    import deterministic_source_bundle as bundle_builder


SHA256 = re.compile(r"^[0-9a-f]{64}$")
SAFE_RUN_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{2,127}$")
SAFE_STORAGE_REF = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:-]{2,127}$")
ALLOWED_SCOPE_KINDS = {"task", "sku", "retouch_requirement"}


def canonical_bytes(value: object) -> bytes:
    return (
        json.dumps(
            value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
        )
        + "\n"
    ).encode("utf-8")


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def resolved_directory(path: pathlib.Path, label: str) -> pathlib.Path:
    if not path.is_absolute() or not path.is_dir() or path.is_symlink():
        raise ValueError(f"{label} must be an existing absolute non-symlink directory")
    return path.resolve()


def contained(root: pathlib.Path, relative: pathlib.PurePosixPath) -> pathlib.Path:
    if (
        relative.is_absolute()
        or not relative.parts
        or any(part in {"", ".", ".."} for part in relative.parts)
    ):
        raise ValueError("object key is not a safe relative path")
    target = root.joinpath(*relative.parts)
    resolved_parent = target.parent.resolve()
    try:
        resolved_parent.relative_to(root)
    except ValueError:
        raise ValueError("object key escapes the configured root") from None
    if target.is_symlink():
        raise ValueError("symlinked source/output objects are forbidden")
    return target


def exact_b_root(run_root: pathlib.Path, b_root: pathlib.Path) -> pathlib.Path:
    resolved = resolved_directory(b_root, "b_root")
    expected = run_root / "fixture-upload-b"
    if resolved != expected:
        raise ValueError("b_root must be exactly <run-root>/fixture-upload-b")
    return resolved


def source_path(source_root: pathlib.Path, object_key: str) -> pathlib.Path:
    target = contained(source_root, pathlib.PurePosixPath(object_key))
    if not target.is_file():
        raise ValueError(f"source object is unavailable: {object_key}")
    return target


def bundle_object_key(
    run_id: str, task_id: int, scope_kind: str, scope_ref_id: int, revision_no: int
) -> str:
    return (
        f"fixture/{run_id}/migration-bundles/task-{task_id}/"
        f"{scope_kind}-{scope_ref_id}/revision-{revision_no}/source-bundle.zip"
    )


def validate_manifest(
    manifest: dict[str, Any], source_root: pathlib.Path
) -> list[dict[str, Any]]:
    if manifest.get("schema_version") != 1 or manifest.get("status") != "CONFIRMED":
        raise ValueError("manifest must be schema_version=1 and status=CONFIRMED")
    run_id = str(manifest.get("run_id") or "")
    if not SAFE_RUN_ID.fullmatch(run_id):
        raise ValueError("manifest.run_id is invalid")
    reviewer = manifest.get("confirmed_by")
    if isinstance(reviewer, bool) or not isinstance(reviewer, int) or reviewer <= 0:
        raise ValueError("manifest.confirmed_by must be a positive reviewer id")
    confirmed_at = str(manifest.get("confirmed_at") or "").strip()
    note = str(manifest.get("confirmation_note") or "").strip()
    if not confirmed_at or not note:
        raise ValueError("manifest confirmation metadata is incomplete")
    candidate_hash = str(manifest.get("source_candidate_sha256") or "")
    if not SHA256.fullmatch(candidate_hash):
        raise ValueError("manifest.source_candidate_sha256 must be SHA-256")
    bundles = manifest.get("bundles")
    if not isinstance(bundles, list) or not bundles:
        raise ValueError("manifest.bundles must be a non-empty array")

    seen_scopes: set[tuple[int, str, int, int]] = set()
    seen_bundle_assets: set[int] = set()
    seen_storage_refs: set[str] = set()
    prepared = []
    for index, item in enumerate(bundles):
        path = f"bundles[{index}]"
        if not isinstance(item, dict) or item.get("confirmed") is not True:
            raise ValueError(f"{path} must be an administrator-confirmed object")
        task_id = item.get("task_id")
        scope_kind = str(item.get("scope_kind") or "")
        scope_ref_id = item.get("scope_ref_id")
        revision_no = item.get("revision_no")
        bundle_task_asset_id = item.get("bundle_task_asset_id")
        bundle_asset_id = item.get("bundle_asset_id")
        storage_ref_id = str(item.get("bundle_storage_ref_id") or "")
        integer_values = (
            task_id,
            scope_ref_id,
            revision_no,
            bundle_task_asset_id,
            bundle_asset_id,
        )
        if any(isinstance(v, bool) or not isinstance(v, int) for v in integer_values):
            raise ValueError(f"{path} integer fields are invalid")
        if (
            task_id <= 0
            or scope_kind not in ALLOWED_SCOPE_KINDS
            or revision_no <= 0
            or bundle_task_asset_id <= 0
            or bundle_asset_id <= 0
            or (scope_kind == "task" and scope_ref_id != 0)
            or (scope_kind != "task" and scope_ref_id <= 0)
        ):
            raise ValueError(f"{path} scope or identifier fields are invalid")
        scope_key = (task_id, scope_kind, scope_ref_id, revision_no)
        if scope_key in seen_scopes:
            raise ValueError(f"{path} duplicates a task/scope/revision")
        if bundle_task_asset_id in seen_bundle_assets:
            raise ValueError(f"{path} duplicates bundle_task_asset_id")
        if not SAFE_STORAGE_REF.fullmatch(storage_ref_id) or storage_ref_id in seen_storage_refs:
            raise ValueError(f"{path}.bundle_storage_ref_id is invalid or duplicate")
        seen_scopes.add(scope_key)
        seen_bundle_assets.add(bundle_task_asset_id)
        seen_storage_refs.add(storage_ref_id)

        members = item.get("ordered_members")
        if not isinstance(members, list) or len(members) < 2:
            raise ValueError(f"{path}.ordered_members requires at least two members")
        normalized_members = []
        seen_member_ids: set[int] = set()
        for member_index, member in enumerate(members):
            member_path = f"{path}.ordered_members[{member_index}]"
            if not isinstance(member, dict) or member.get("confirmed") is not True:
                raise ValueError(f"{member_path} must be confirmed=true")
            task_asset_id = member.get("task_asset_id")
            asset_id = member.get("asset_id")
            size = member.get("size")
            if (
                isinstance(task_asset_id, bool)
                or not isinstance(task_asset_id, int)
                or task_asset_id <= 0
                or task_asset_id in seen_member_ids
                or isinstance(asset_id, bool)
                or not isinstance(asset_id, int)
                or asset_id <= 0
                or isinstance(size, bool)
                or not isinstance(size, int)
                or size < 0
            ):
                raise ValueError(f"{member_path} identifiers/size are invalid")
            if member.get("task_id") != task_id:
                raise ValueError(f"{member_path} belongs to another task")
            digest = str(member.get("sha256") or "")
            if not SHA256.fullmatch(digest):
                raise ValueError(f"{member_path}.sha256 is invalid")
            object_key = str(member.get("object_key") or "")
            local_path = source_path(source_root, object_key)
            if local_path.stat().st_size != size or sha256_file(local_path) != digest:
                raise ValueError(f"{member_path} source byte size/SHA-256 drifted")
            evidence = member.get("evidence_event_ids")
            if (
                not isinstance(evidence, list)
                or not evidence
                or any(
                    not isinstance(value, str)
                    or not bundle_builder.EVIDENCE_RE.fullmatch(value)
                    for value in evidence
                )
            ):
                raise ValueError(f"{member_path}.evidence_event_ids is invalid")
            normalized_members.append(
                {
                    "task_asset_id": task_asset_id,
                    "asset_id": asset_id,
                    "storage_ref_id": str(member.get("storage_ref_id") or ""),
                    "original_file_name": str(
                        member.get("original_file_name") or ""
                    ),
                    "local_path": str(local_path),
                    "sha256": digest,
                    "source_stage": str(member.get("source_stage") or ""),
                    "evidence_event_ids": evidence,
                    "confirmed": True,
                }
            )
            seen_member_ids.add(task_asset_id)

        object_key = bundle_object_key(
            run_id, task_id, scope_kind, scope_ref_id, revision_no
        )
        builder_plan = {
            "version": 1,
            "bundle_task_asset_id": bundle_task_asset_id,
            "confirmed_by": reviewer,
            "confirmed_at": confirmed_at,
            "confirmation_note": note,
            "members": normalized_members,
        }
        bundle_builder.validate_plan(builder_plan)
        prepared.append(
            {
                "scope_key": scope_key,
                "bundle_asset_id": bundle_asset_id,
                "bundle_storage_ref_id": storage_ref_id,
                "object_key": object_key,
                "builder_plan": builder_plan,
            }
        )
    return prepared


def prepare_document(
    manifest_path: pathlib.Path,
    source_root: pathlib.Path,
    prepared: list[dict[str, Any]],
) -> dict[str, Any]:
    return {
        "schema_version": 1,
        "status": "PREPARED",
        "manifest_sha256": sha256_file(manifest_path),
        "source_root": str(source_root),
        "bundle_count": len(prepared),
        "member_count": sum(
            len(item["builder_plan"]["members"]) for item in prepared
        ),
        "object_keys": [item["object_key"] for item in prepared],
        "database_write_performed": False,
        "object_write_performed": False,
    }


def atomic_write(path: pathlib.Path, value: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    encoded = canonical_bytes(value)
    if path.exists():
        if path.read_bytes() == encoded:
            return
        raise FileExistsError(f"refusing to overwrite different artifact: {path}")
    with tempfile.NamedTemporaryFile(
        dir=path.parent, prefix=path.name + ".", suffix=".tmp", delete=False
    ) as handle:
        temporary = pathlib.Path(handle.name)
        handle.write(encoded)
    try:
        os.replace(temporary, path)
    finally:
        if temporary.exists():
            temporary.unlink()


def self_bound(value: dict[str, Any]) -> dict[str, Any]:
    result = dict(value)
    result["evidence_sha256"] = hashlib.sha256(
        canonical_bytes(result)
    ).hexdigest()
    return result


def verify_self_bound(value: dict[str, Any], label: str) -> None:
    expected = str(value.get("evidence_sha256") or "")
    unsigned = dict(value)
    unsigned.pop("evidence_sha256", None)
    if (
        not SHA256.fullmatch(expected)
        or hashlib.sha256(canonical_bytes(unsigned)).hexdigest() != expected
    ):
        raise ValueError(f"{label} self hash is missing or stale")


def materialize(
    manifest: dict[str, Any],
    manifest_path: pathlib.Path,
    prepared: list[dict[str, Any]],
    b_root: pathlib.Path,
    registry_path: pathlib.Path,
    write_ahead_path: pathlib.Path,
    staging_write_ahead_path: pathlib.Path,
) -> dict[str, Any]:
    if registry_path.exists():
        existing = json.loads(registry_path.read_text(encoding="utf-8"))
        verify_self_bound(existing, "existing bundle registry")
        if (
            existing.get("schema_version") != 1
            or existing.get("status") != "MATERIALIZED"
            or existing.get("manifest_sha256") != sha256_file(manifest_path)
            or pathlib.Path(str(existing.get("b_root") or "")).resolve()
            != b_root
        ):
            raise FileExistsError(
                "existing cleanup registry does not match this materialization"
            )
        for entry in existing.get("entries") or []:
            target = contained(
                b_root,
                pathlib.PurePosixPath(
                    str(entry.get("relative_object_path") or "")
                ),
            )
            if (
                not target.is_file()
                or sha256_file(target) != entry.get("bundle_sha256")
            ):
                raise ValueError(
                    "existing registry bundle is missing or has drifted"
                )
        return existing
    stage_specs = [
        {
            "path": str(
                (
                    write_ahead_path.parent
                    / (
                        ".bundle-stage-"
                        f"{item['builder_plan']['bundle_task_asset_id']}.zip"
                    )
                ).resolve()
            ),
            "object_key": item["object_key"],
        }
        for item in prepared
    ]
    for item in stage_specs:
        stage = pathlib.Path(item["path"])
        if stage.exists() or stage.is_symlink():
            raise FileExistsError(
                f"staging path existed before write-ahead: {stage}"
            )
    staging_write_ahead = self_bound(
        {
            "schema_version": 1,
            "status": "STAGING_WRITE_AHEAD",
            "run_id": manifest["run_id"],
            "manifest_sha256": sha256_file(manifest_path),
            "b_root": str(b_root),
            "database_write_performed": False,
            "stage_specs": stage_specs,
        }
    )
    atomic_write(staging_write_ahead_path, staging_write_ahead)
    created_paths: list[pathlib.Path] = []
    staged_paths: list[pathlib.Path] = []
    staged: list[
        tuple[pathlib.Path, pathlib.Path, str, pathlib.Path]
    ] = []
    entries: list[dict[str, Any]] = []
    try:
        for item in prepared:
            target = contained(
                b_root / "objects", pathlib.PurePosixPath(item["object_key"])
            )
            plan_bytes = canonical_bytes(item["builder_plan"])
            with tempfile.NamedTemporaryFile(
                dir=write_ahead_path.parent,
                prefix="bundle-plan.",
                suffix=".json",
                delete=False,
            ) as handle:
                plan_path = pathlib.Path(handle.name)
                handle.write(plan_bytes)
            temporary_zip = write_ahead_path.parent / (
                f".bundle-stage-{item['builder_plan']['bundle_task_asset_id']}.zip"
            )
            try:
                if temporary_zip.exists():
                    raise FileExistsError(
                        f"stale bundle candidate already exists: {temporary_zip}"
                    )
                result = bundle_builder.build(plan_path, temporary_zip)
                if target.exists():
                    if not target.is_file() or sha256_file(target) != result["bundle_sha256"]:
                        raise FileExistsError(
                            f"existing bundle differs from reviewed bytes: {target}"
                        )
                    disposition = "reused_identical"
                else:
                    disposition = "created"
            finally:
                plan_path.unlink(missing_ok=True)

            size = temporary_zip.stat().st_size
            staged_paths.append(temporary_zip)
            scope_task, scope_kind, scope_ref, revision_no = item["scope_key"]
            source_bundle = result["source_bundle"]
            ownership_receipt_path = (
                write_ahead_path.parent
                / f"bundle-ownership-{source_bundle['task_asset_id']}.json"
            ).resolve()
            if disposition == "created" and (
                ownership_receipt_path.exists()
                or ownership_receipt_path.is_symlink()
            ):
                raise FileExistsError(
                    "bundle ownership receipt existed before write-ahead"
                )
            staged.append(
                (
                    temporary_zip,
                    target,
                    disposition,
                    ownership_receipt_path,
                )
            )
            entries.append(
                {
                    "task_id": scope_task,
                    "scope_kind": scope_kind,
                    "scope_ref_id": scope_ref,
                    "revision_no": revision_no,
                    "relative_object_path": str(
                        target.relative_to(b_root).as_posix()
                    ),
                    "object_key": item["object_key"],
                    "bundle_sha256": result["bundle_sha256"],
                    "size": size,
                    "disposition": disposition,
                    "source_bundle": source_bundle,
                    "asset_storage_ref_candidate": {
                        "ref_id": item["bundle_storage_ref_id"],
                        "storage_adapter": "upload_service",
                        "ref_key": item["object_key"],
                        "file_name": "source-bundle.zip",
                        "file_size": size,
                        "mime_type": "application/zip",
                        "checksum_hint": result["bundle_sha256"],
                        "status": "recorded",
                        "is_placeholder": False,
                    },
                    "task_asset_candidate": {
                        "id": source_bundle["task_asset_id"],
                        "task_id": scope_task,
                        "asset_id": item["bundle_asset_id"],
                        "asset_type": "source",
                        "scope_kind": scope_kind,
                        "scope_ref_id": scope_ref,
                        "storage_ref_id": item["bundle_storage_ref_id"],
                        "file_name": "source-bundle.zip",
                        "mime_type": "application/zip",
                        "file_size": size,
                        "storage_key": item["object_key"],
                        "whole_hash": result["bundle_sha256"],
                        "upload_status": "uploaded",
                        "source_module_key": "migration",
                    },
                    "rollback_candidate": {
                        "task_asset_id": source_bundle["task_asset_id"],
                        "storage_ref_id": item["bundle_storage_ref_id"],
                        "relative_object_path": str(
                            target.relative_to(b_root).as_posix()
                        ),
                        "expected_sha256": result["bundle_sha256"],
                        "ownership_receipt_path": str(
                            ownership_receipt_path
                        ),
                    },
                }
            )
        write_ahead = self_bound({
            "schema_version": 1,
            "status": "WRITE_AHEAD",
            "run_id": manifest["run_id"],
            "manifest_sha256": sha256_file(manifest_path),
            "b_root": str(b_root),
            "database_write_performed": False,
            "entries": entries,
            "staging_files": [
                {
                    "path": str(path.resolve()),
                    "sha256": sha256_file(path),
                    "size": path.stat().st_size,
                }
                for path in staged_paths
            ],
        })
        atomic_write(write_ahead_path, write_ahead)
        for (
            temporary_zip,
            target,
            disposition,
            ownership_receipt_path,
        ) in staged:
            if disposition == "created":
                if target.exists():
                    raise FileExistsError(
                        f"bundle target appeared after write-ahead: {target}"
                    )
                target.parent.mkdir(parents=True, exist_ok=True)
                os.link(temporary_zip, target)
                source_stat = temporary_zip.stat()
                target_stat = target.stat()
                if not os.path.samestat(source_stat, target_stat):
                    raise RuntimeError("bundle hard-link ownership proof failed")
                ownership_receipt = self_bound(
                    {
                        "schema_version": 1,
                        "status": "OWNED_LINK",
                        "run_id": manifest["run_id"],
                        "target_path": str(target.resolve()),
                        "staging_path": str(temporary_zip.resolve()),
                        "device": target_stat.st_dev,
                        "inode": target_stat.st_ino,
                        "size": target_stat.st_size,
                        "sha256": sha256_file(target),
                    }
                )
                atomic_write(
                    ownership_receipt_path,
                    ownership_receipt,
                )
                temporary_zip.unlink()
                created_paths.append(target)
            else:
                temporary_zip.unlink()
    except Exception as original:
        if write_ahead_path.is_file() and not write_ahead_path.is_symlink():
            try:
                seed = json.loads(
                    write_ahead_path.read_text(encoding="utf-8")
                )
                if not isinstance(seed, dict):
                    raise ValueError("bundle write-ahead must be an object")
                rollback(seed, b_root)
            except Exception as compensation:
                raise RuntimeError(
                    "bundle materialization failed and exact compensation "
                    f"could not complete: {original}; "
                    f"compensation={compensation}"
                ) from original
        else:
            for path in staged_paths:
                path.unlink(missing_ok=True)
        raise

    registry = self_bound({
        "schema_version": 1,
        "status": "MATERIALIZED",
        "run_id": manifest["run_id"],
        "manifest_sha256": sha256_file(manifest_path),
        "b_root": str(b_root),
        "database_write_performed": False,
        "entries": entries,
        "write_ahead_sha256": sha256_file(write_ahead_path),
    })
    atomic_write(registry_path, registry)
    return registry


def rollback(registry: dict[str, Any], b_root: pathlib.Path) -> dict[str, Any]:
    verify_self_bound(registry, "cleanup registry")
    if (
        registry.get("schema_version") != 1
        or registry.get("status")
        not in {"STAGING_WRITE_AHEAD", "WRITE_AHEAD", "MATERIALIZED"}
    ):
        raise ValueError("cleanup registry is invalid")
    if pathlib.Path(str(registry.get("b_root") or "")).resolve() != b_root:
        raise ValueError("cleanup registry belongs to another B root")
    if registry.get("status") == "STAGING_WRITE_AHEAD":
        removed_staging = []
        specs = registry.get("stage_specs")
        if not isinstance(specs, list) or not specs:
            raise ValueError("staging cleanup registry has no stage specs")
        for index, item in enumerate(specs):
            if not isinstance(item, dict):
                raise ValueError(f"stage_specs[{index}] is invalid")
            stage = pathlib.Path(str(item.get("path") or ""))
            try:
                stage.resolve(strict=False).relative_to(b_root.parent.resolve())
            except ValueError:
                raise ValueError("staging file escapes the run root") from None
            if stage.is_symlink():
                raise ValueError("symlinked staging files are forbidden")
            if stage.exists():
                if not stage.is_file():
                    raise ValueError("staging cleanup target is not a file")
                stage.unlink()
                removed_staging.append(str(stage))
        return {
            "schema_version": 1,
            "status": "ROLLED_BACK",
            "removed_object_paths": [],
            "pruned_empty_directories": [],
            "already_absent_count": 0,
            "retained_reused_object_paths": [],
            "removed_staging_paths": removed_staging,
            "database_cleanup_candidates": [],
            "database_write_performed": False,
        }
    targets = []
    retained_reused = []
    for index, entry in enumerate(registry.get("entries") or []):
        relative = pathlib.PurePosixPath(
            str(entry.get("relative_object_path") or "")
        )
        target = contained(b_root, relative)
        disposition = str(entry.get("disposition") or "")
        if disposition not in {"created", "reused_identical"}:
            raise ValueError(
                f"entries[{index}].disposition is invalid"
            )
        expected = str(entry.get("bundle_sha256") or "")
        if not SHA256.fullmatch(expected):
            raise ValueError(f"entries[{index}].bundle_sha256 is invalid")
        if target.exists() and (
            not target.is_file() or sha256_file(target) != expected
        ):
            raise ValueError(
                f"refusing cleanup because bundle bytes drifted: {relative}"
            )
        if disposition == "created":
            receipt_path = pathlib.Path(
                str(
                    entry.get("rollback_candidate", {}).get(
                        "ownership_receipt_path"
                    )
                    or ""
                )
            )
            try:
                receipt_path.resolve(strict=False).relative_to(
                    b_root.parent.resolve()
                )
            except ValueError:
                raise ValueError("ownership receipt escapes the run root") from None
            if not receipt_path.is_absolute() or receipt_path.is_symlink():
                raise ValueError("ownership receipt path is invalid")
            targets.append((relative, target, expected, receipt_path))
        else:
            if not target.exists():
                raise ValueError(
                    f"refusing cleanup because reused bundle is missing: {relative}"
                )
            retained_reused.append(relative.as_posix())
    removed = []
    for relative, target, expected, receipt_path in targets:
        if target.exists():
            if not target.is_file() or sha256_file(target) != expected:
                raise ValueError(
                    f"refusing cleanup because bundle bytes drifted: {relative}"
                )
            owned = False
            if receipt_path.is_file() and not receipt_path.is_symlink():
                receipt = json.loads(
                    receipt_path.read_text(encoding="utf-8")
                )
                verify_self_bound(receipt, "bundle ownership receipt")
                stat = target.stat()
                owned = (
                    receipt.get("status") == "OWNED_LINK"
                    and receipt.get("run_id") == registry.get("run_id")
                    and receipt.get("target_path") == str(target.resolve())
                    and receipt.get("device") == stat.st_dev
                    and receipt.get("inode") == stat.st_ino
                    and receipt.get("size") == stat.st_size
                    and receipt.get("sha256") == expected
                )
            if not owned:
                matching_stage = next(
                    (
                        pathlib.Path(str(item.get("path") or ""))
                        for item in registry.get("staging_files") or []
                        if isinstance(item, dict)
                        and pathlib.Path(str(item.get("path") or "")).exists()
                        and os.path.samestat(
                            pathlib.Path(str(item.get("path"))).stat(),
                            target.stat(),
                        )
                    ),
                    None,
                )
                owned = matching_stage is not None
            if not owned:
                raise ValueError(
                    f"bundle target ownership cannot be proven: {relative}"
                )
            target.unlink()
            removed.append(relative.as_posix())
    object_root = (b_root / "objects").resolve()
    pruned_directories: list[str] = []
    for _, target, _, _ in sorted(
        targets, key=lambda item: len(item[1].parts), reverse=True
    ):
        parent = target.parent
        while parent != object_root and object_root in parent.parents:
            try:
                parent.rmdir()
            except OSError:
                break
            pruned_directories.append(
                parent.relative_to(b_root).as_posix()
            )
            parent = parent.parent
    removed_staging = []
    for index, item in enumerate(registry.get("staging_files") or []):
        if not isinstance(item, dict):
            raise ValueError(f"staging_files[{index}] is invalid")
        stage = pathlib.Path(str(item.get("path") or ""))
        try:
            stage.resolve(strict=False).relative_to(b_root.parent.resolve())
        except ValueError:
            raise ValueError("staging file escapes the run root") from None
        expected = str(item.get("sha256") or "")
        size = item.get("size")
        if (
            stage.is_symlink()
            or not SHA256.fullmatch(expected)
            or isinstance(size, bool)
            or not isinstance(size, int)
            or size < 0
        ):
            raise ValueError("staging file identity is invalid")
        if stage.exists():
            if (
                not stage.is_file()
                or stage.stat().st_size != size
                or sha256_file(stage) != expected
            ):
                raise ValueError("refusing cleanup because staging bytes drifted")
            stage.unlink()
            removed_staging.append(str(stage))
    return {
        "schema_version": 1,
        "status": "ROLLED_BACK",
        "removed_object_paths": removed,
        "pruned_empty_directories": sorted(set(pruned_directories)),
        "already_absent_count": len(targets) - len(removed),
        "retained_reused_object_paths": retained_reused,
        "removed_staging_paths": removed_staging,
        "database_cleanup_candidates": [
            entry["rollback_candidate"] for entry in registry["entries"]
        ],
        "database_write_performed": False,
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=("prepare", "materialize", "rollback"))
    parser.add_argument("--run-root", type=pathlib.Path, required=True)
    parser.add_argument("--source-root", type=pathlib.Path)
    parser.add_argument("--b-root", type=pathlib.Path, required=True)
    parser.add_argument("--manifest", type=pathlib.Path)
    parser.add_argument("--report", type=pathlib.Path, required=True)
    parser.add_argument("--registry", type=pathlib.Path)
    parser.add_argument("--write-ahead-registry", type=pathlib.Path)
    parser.add_argument("--staging-write-ahead-registry", type=pathlib.Path)
    parser.add_argument("--execute", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    run_root = resolved_directory(args.run_root, "run_root")
    b_root = exact_b_root(run_root, args.b_root)
    if args.action == "rollback":
        if not args.execute:
            raise ValueError("rollback is plan-only unless --execute is explicit")
        if args.registry is None or not args.registry.is_file():
            raise ValueError("--registry must name an existing cleanup registry")
        result = rollback(
            json.loads(args.registry.read_text(encoding="utf-8")), b_root
        )
        atomic_write(args.report, result)
        return 0

    if args.manifest is None or not args.manifest.is_file():
        raise ValueError("--manifest must name an existing confirmed manifest")
    if args.source_root is None:
        raise ValueError("--source-root is required")
    source_root = resolved_directory(args.source_root, "source_root")
    try:
        source_root.relative_to(run_root)
    except ValueError:
        raise ValueError("source_root must be inside run_root") from None
    if source_root == b_root:
        raise ValueError("source_root and b_root must differ")
    manifest = json.loads(args.manifest.read_text(encoding="utf-8"))
    prepared = validate_manifest(manifest, source_root)
    if args.action == "prepare":
        atomic_write(
            args.report,
            prepare_document(args.manifest, source_root, prepared),
        )
        return 0
    if not args.execute:
        raise ValueError("materialize is plan-only unless --execute is explicit")
    if args.registry is None:
        raise ValueError("--registry is required for materialize")
    if args.write_ahead_registry is None:
        raise ValueError("--write-ahead-registry is required for materialize")
    if args.staging_write_ahead_registry is None:
        raise ValueError(
            "--staging-write-ahead-registry is required for materialize"
        )
    for path in (
        args.registry,
        args.write_ahead_registry,
        args.staging_write_ahead_registry,
    ):
        try:
            path.resolve(strict=False).relative_to(run_root)
        except ValueError:
            raise ValueError("bundle registry paths must be inside run_root") from None
        if path.is_symlink():
            raise ValueError("symlinked bundle registry paths are forbidden")
    result = materialize(
        manifest,
        args.manifest,
        prepared,
        b_root,
        args.registry,
        args.write_ahead_registry,
        args.staging_write_ahead_registry,
    )
    atomic_write(args.report, result)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
