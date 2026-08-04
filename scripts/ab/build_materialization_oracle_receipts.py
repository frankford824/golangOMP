#!/usr/bin/env python3
"""Build schema-v2 materialization receipts from frozen, offline evidence.

This module deliberately has no database, HTTP, subprocess, or object-store
client.  It cross-binds reviewed migration intent to already-created Clone B
materialization evidence.  Frozen A is the authority for legacy task-asset
identity and metadata; controlled-read recovery evidence is the authority for
content hashes when the legacy ``whole_hash`` column is null.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import re
import tempfile
import zipfile
from collections.abc import Iterable, Mapping
from typing import Any


SCHEMA_VERSION = 2
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
BUNDLE_KIND = "bundle_materialization_v2"
RECOVERY_KIND = "recovery_materialization_v2"
BUNDLE_STRATEGY = "verified_oss_recovery_v1"


class ReceiptError(ValueError):
    """Raised when evidence is incomplete, inconsistent, or unconsumed."""


def canonical(value: Any) -> bytes:
    return json.dumps(
        value,
        ensure_ascii=False,
        sort_keys=True,
        separators=(",", ":"),
    ).encode("utf-8")


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def require_object(value: Any, label: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise ReceiptError(f"{label} must be an object")
    return value


def require_array(value: Any, label: str) -> list[Any]:
    if not isinstance(value, list):
        raise ReceiptError(f"{label} must be an array")
    return value


def require_exact_fields(
    value: Mapping[str, Any], expected: set[str], label: str
) -> None:
    actual = set(value)
    if actual != expected:
        raise ReceiptError(
            f"{label} field contract differs: "
            f"missing={sorted(expected - actual)} extra={sorted(actual - expected)}"
        )


def require_fields(
    value: Mapping[str, Any], expected: set[str], label: str
) -> None:
    missing = expected - set(value)
    if missing:
        raise ReceiptError(f"{label} lacks fields {sorted(missing)}")


def required_int(value: Any, label: str) -> int:
    if isinstance(value, bool):
        raise ReceiptError(f"{label} must be an integer")
    try:
        result = int(value)
    except (TypeError, ValueError) as exc:
        raise ReceiptError(f"{label} must be an integer") from exc
    if str(value).strip() not in {str(result), f"+{result}"}:
        raise ReceiptError(f"{label} must be a canonical integer")
    return result


def required_positive_int(value: Any, label: str) -> int:
    result = required_int(value, label)
    if result <= 0:
        raise ReceiptError(f"{label} must be positive")
    return result


def required_text(value: Any, label: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ReceiptError(f"{label} must be non-empty text")
    return value


def required_sha256(value: Any, label: str) -> str:
    result = required_text(value, label)
    if not SHA256_RE.fullmatch(result):
        raise ReceiptError(f"{label} must be a lowercase SHA-256")
    return result


def equal(actual: Any, expected: Any, label: str) -> None:
    if actual != expected:
        raise ReceiptError(f"{label} differs: {actual!r} != {expected!r}")


def load_json(path: pathlib.Path, label: str) -> dict[str, Any]:
    if path.is_symlink():
        raise ReceiptError(f"{label} must not be a symlink: {path}")
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise ReceiptError(f"cannot read {label}: {path}: {exc}") from exc
    return require_object(value, label)


def validate_self_hash(
    document: Mapping[str, Any],
    label: str,
    field: str = "evidence_sha256",
    *,
    canonical_newline: bool = False,
) -> str:
    digest = required_sha256(document.get(field), f"{label}.{field}")
    unsigned = {key: value for key, value in document.items() if key != field}
    payload = canonical(unsigned) + (b"\n" if canonical_newline else b"")
    equal(digest, sha256(payload), f"{label}.{field}")
    return digest


def normalized_path(path: pathlib.Path) -> str:
    return os.path.normpath(str(path))


def assert_receipt_path(
    declared: Any, actual: pathlib.Path, label: str
) -> None:
    declared_path = pathlib.Path(required_text(declared, label))
    if not declared_path.is_absolute():
        declared_path = actual.parent / declared_path
    declared_normalized = normalized_path(declared_path)
    actual_normalized = normalized_path(actual)
    if declared_normalized == actual_normalized:
        return
    # Evidence created inside the isolated runner uses /evidence-root while
    # the same immutable file is later inspected through the repository path.
    # Bind the complete run-relative path, never only a basename.
    marker = os.path.join("tmp", "v8-ab") + os.sep
    declared_suffix = (
        declared_normalized.split(marker, 1)[1]
        if marker in declared_normalized
        else None
    )
    actual_suffix = (
        actual_normalized.split(marker, 1)[1]
        if marker in actual_normalized
        else None
    )
    if declared_suffix is None or declared_suffix != actual_suffix:
        raise ReceiptError(
            f"{label} differs: {declared_path!s} != {actual!s}"
        )


def object_key_sha256(value: str) -> str:
    return sha256(value.encode("utf-8"))


def _resolve_evidence_target(
    declared: pathlib.Path, evidence_path: pathlib.Path, label: str
) -> pathlib.Path:
    """Resolve an isolated /evidence-root path to the same local run path."""
    if declared.is_file() and not declared.is_symlink():
        return declared
    marker = ("tmp", "v8-ab")
    declared_parts = declared.parts
    evidence_parts = evidence_path.resolve().parts
    try:
        declared_index = next(
            index
            for index in range(len(declared_parts) - 1)
            if declared_parts[index : index + 2] == marker
        )
        evidence_index = next(
            index
            for index in range(len(evidence_parts) - 1)
            if evidence_parts[index : index + 2] == marker
        )
    except StopIteration as exc:
        raise ReceiptError(f"{label} cannot be mapped to the frozen run") from exc
    candidate = pathlib.Path(
        *evidence_parts[:evidence_index],
        *declared_parts[declared_index:],
    )
    if not candidate.is_file() or candidate.is_symlink():
        raise ReceiptError(f"{label} is absent or symlinked: {candidate}")
    return candidate


def _validate_bundle_archive(
    *,
    archive_path: pathlib.Path,
    bundle_sha256: str,
    bundle_size: int,
    source_bundle: Mapping[str, Any],
    frozen_rows: Mapping[int, dict[str, Any]],
    task_id: int,
    label: str,
) -> tuple[list[dict[str, Any]], str]:
    equal(sha256_file(archive_path), bundle_sha256, f"{label} archive SHA-256")
    equal(archive_path.stat().st_size, bundle_size, f"{label} archive size")
    mapping_manifest = canonical(
        {
            "format": "zip",
            "members": source_bundle["members"],
        }
    )
    equal(
        sha256(mapping_manifest),
        source_bundle["manifest_sha256"],
        f"{label} mapping manifest SHA-256",
    )
    try:
        with zipfile.ZipFile(archive_path) as bundle:
            infos = bundle.infolist()
            names = [info.filename for info in infos]
            if len(names) != len(set(names)) or not names or names[0] != "manifest.json":
                raise ReceiptError(f"{label} ZIP entries are duplicate or unordered")
            manifest_bytes = bundle.read("manifest.json")
            embedded = require_object(
                json.loads(manifest_bytes), f"{label} embedded manifest"
            )
            require_exact_fields(
                embedded,
                {
                    "confirmation",
                    "deterministic_profile",
                    "members",
                    "version",
                },
                f"{label} embedded manifest",
            )
            equal(embedded["version"], 1, f"{label} embedded manifest.version")
            equal(
                embedded["deterministic_profile"],
                "zip-stored-fixed-1980-0644-v1",
                f"{label} embedded manifest.deterministic_profile",
            )
            confirmation = require_object(
                embedded["confirmation"], f"{label} embedded confirmation"
            )
            equal(
                confirmation,
                {
                    "confirmed_by": source_bundle["confirmed_by"],
                    "confirmed_at": source_bundle["confirmed_at"],
                    "confirmation_note": source_bundle["confirmation_note"],
                },
                f"{label} embedded confirmation",
            )
            embedded_members = require_array(
                embedded["members"], f"{label} embedded members"
            )
            mapping_members = require_array(
                source_bundle["members"], f"{label} mapping members"
            )
            if len(embedded_members) != len(mapping_members):
                raise ReceiptError(f"{label} embedded member count differs")
            expected_names = ["manifest.json"]
            receipt_members: list[dict[str, Any]] = []
            for index, (raw_embedded, raw_mapping) in enumerate(
                zip(embedded_members, mapping_members, strict=True), 1
            ):
                member = require_object(
                    raw_embedded, f"{label} embedded member {index}"
                )
                mapping_member = require_object(
                    raw_mapping, f"{label} mapping member {index}"
                )
                require_exact_fields(
                    member,
                    {
                        "archive_path",
                        "asset_id",
                        "confirmed",
                        "evidence_event_ids",
                        "original_file_name",
                        "sha256",
                        "source_stage",
                        "storage_ref_id",
                        "task_asset_id",
                    },
                    f"{label} embedded member {index}",
                )
                member_id = required_positive_int(
                    mapping_member["task_asset_id"],
                    f"{label} mapping member {index}.task_asset_id",
                )
                content_hash = required_sha256(
                    mapping_member["sha256"],
                    f"{label} mapping member {index}.sha256",
                )
                equal(
                    member["task_asset_id"],
                    member_id,
                    f"{label} embedded member {index}.task_asset_id",
                )
                equal(
                    member["sha256"],
                    content_hash,
                    f"{label} embedded member {index}.sha256",
                )
                equal(
                    member["confirmed"],
                    True,
                    f"{label} embedded member {index}.confirmed",
                )
                row = frozen_rows.get(member_id)
                if row is None:
                    raise ReceiptError(
                        f"{label} member {member_id} is absent from frozen A"
                    )
                equal(
                    required_positive_int(
                        row.get("task_id"),
                        f"frozen A bundle member {member_id}.task_id",
                    ),
                    task_id,
                    f"{label} member {member_id}.task_id",
                )
                storage_ref_id = required_text(
                    row.get("storage_ref_id"),
                    f"frozen A bundle member {member_id}.storage_ref_id",
                )
                size = required_positive_int(
                    row.get("file_size"),
                    f"frozen A bundle member {member_id}.file_size",
                )
                mime_type = required_text(
                    row.get("mime_type"),
                    f"frozen A bundle member {member_id}.mime_type",
                )
                equal(
                    member["storage_ref_id"],
                    storage_ref_id,
                    f"{label} member {member_id}.storage_ref_id",
                )
                equal(
                    required_positive_int(
                        member["asset_id"],
                        f"{label} member {member_id}.asset_id",
                    ),
                    required_positive_int(
                        row.get("asset_id"),
                        f"frozen A bundle member {member_id}.asset_id",
                    ),
                    f"{label} member {member_id}.asset_id",
                )
                frozen_hash = row.get("whole_hash")
                if frozen_hash not in (None, ""):
                    equal(
                        required_sha256(
                            frozen_hash,
                            f"frozen A bundle member {member_id}.whole_hash",
                        ),
                        content_hash,
                        f"{label} member {member_id}.whole_hash",
                    )
                archive_name = required_text(
                    member["archive_path"],
                    f"{label} member {member_id}.archive_path",
                )
                if (
                    pathlib.PurePosixPath(archive_name).is_absolute()
                    or ".." in pathlib.PurePosixPath(archive_name).parts
                    or not archive_name.startswith(f"{index:03d}_{member_id}_")
                ):
                    raise ReceiptError(
                        f"{label} member {member_id} archive path is unsafe"
                    )
                info = bundle.getinfo(archive_name)
                equal(
                    info.compress_type,
                    zipfile.ZIP_STORED,
                    f"{label} member {member_id} compression",
                )
                equal(info.file_size, size, f"{label} member {member_id} size")
                equal(
                    sha256(bundle.read(archive_name)),
                    content_hash,
                    f"{label} member {member_id} content SHA-256",
                )
                expected_names.append(archive_name)
                receipt_members.append(
                    {
                        "task_asset_id": member_id,
                        "storage_ref_id": storage_ref_id,
                        "size": size,
                        "mime_type": mime_type,
                        "sha256": content_hash,
                    }
                )
            equal(names, expected_names, f"{label} ZIP entry order")
            return receipt_members, sha256(manifest_bytes)
    except (OSError, KeyError, zipfile.BadZipFile, json.JSONDecodeError) as exc:
        raise ReceiptError(f"{label} archive is invalid: {exc}") from exc


def _load_reviewed_manifest(
    path: pathlib.Path,
    mapping_sha256: str,
    required_locators: set[str],
) -> str:
    if path.is_symlink():
        raise ReceiptError(f"reviewed manifest must not be a symlink: {path}")
    manifest_sha256 = sha256_file(path)
    seen: set[str] = set()
    try:
        with path.open(encoding="utf-8") as handle:
            for line_no, raw in enumerate(handle, 1):
                if not raw.strip():
                    continue
                try:
                    row = require_object(
                        json.loads(raw), f"reviewed manifest line {line_no}"
                    )
                except json.JSONDecodeError as exc:
                    raise ReceiptError(
                        f"reviewed manifest line {line_no} is invalid JSON"
                    ) from exc
                detail = require_object(
                    row.get("detail_json"),
                    f"reviewed manifest line {line_no}.detail_json",
                )
                input_hashes = detail.get("input_sha256", {})
                if not isinstance(input_hashes, dict):
                    raise ReceiptError(
                        f"reviewed manifest line {line_no}.input_sha256 "
                        "must be an object"
                    )
                bound_mapping = input_hashes.get("mapping_sha256")
                if bound_mapping is not None:
                    equal(
                        bound_mapping,
                        mapping_sha256,
                        f"reviewed manifest line {line_no} mapping_sha256",
                    )
                entity = str(row.get("entity_key", ""))
                if (
                    row.get("gate_name") != "G04"
                    or not entity.startswith("revision-source:")
                    or row.get("review_state") != "pass"
                ):
                    continue
                locator = ":".join(entity.split(":")[1:5])
                if locator in seen:
                    raise ReceiptError(
                        f"reviewed manifest duplicates G04 locator {locator}"
                    )
                if bound_mapping is None:
                    raise ReceiptError(
                        f"reviewed manifest G04 locator {locator} lacks "
                        "mapping_sha256 binding"
                    )
                seen.add(locator)
    except (OSError, UnicodeError) as exc:
        raise ReceiptError(f"cannot read reviewed manifest {path}: {exc}") from exc

    missing = required_locators - seen
    if missing:
        raise ReceiptError(
            f"reviewed manifest lacks approved G04 locators {sorted(missing)}"
        )
    return manifest_sha256


def _bundle_mapping(
    mapping: Mapping[str, Any],
) -> dict[str, dict[str, Any]]:
    require_exact_fields(
        mapping,
        {
            "version",
            "access_decisions",
            "asset_recoveries",
            "organization_mappings",
            "planning_tasks",
            "resources",
            "task_state_decisions",
        },
        "mapping",
    )
    equal(mapping["version"], 2, "mapping.version")
    result: dict[str, dict[str, Any]] = {}
    allocated_task_assets: set[int] = set()
    for resource_index, raw_resource in enumerate(
        require_array(mapping["resources"], "mapping.resources")
    ):
        resource = require_object(
            raw_resource, f"mapping.resources[{resource_index}]"
        )
        task_id = required_positive_int(
            resource.get("task_id"), f"resource {resource_index}.task_id"
        )
        scope_kind = required_text(
            resource.get("scope_kind"), f"resource {resource_index}.scope_kind"
        )
        scope_ref_id = required_int(
            resource.get("scope_ref_id"),
            f"resource {resource_index}.scope_ref_id",
        )
        for history_index, raw_revision in enumerate(
            require_array(
                resource.get("history"),
                f"resource {resource_index}.history",
            )
        ):
            revision = require_object(
                raw_revision,
                f"resource {resource_index}.history[{history_index}]",
            )
            source_bundle = revision.get("source_bundle")
            if source_bundle is None:
                continue
            source_bundle = require_object(
                source_bundle,
                f"resource {resource_index}.history[{history_index}].source_bundle",
            )
            require_exact_fields(
                source_bundle,
                {
                    "bundle_sha256",
                    "confirmation_note",
                    "confirmed_at",
                    "confirmed_by",
                    "format",
                    "manifest_sha256",
                    "members",
                    "task_asset_id",
                },
                f"bundle {task_id}:{scope_kind}:{scope_ref_id}",
            )
            equal(source_bundle["format"], "zip", "source_bundle.format")
            required_sha256(
                source_bundle["bundle_sha256"], "source_bundle.bundle_sha256"
            )
            required_sha256(
                source_bundle["manifest_sha256"],
                "source_bundle.manifest_sha256",
            )
            required_positive_int(
                source_bundle["confirmed_by"], "source_bundle.confirmed_by"
            )
            required_text(
                source_bundle["confirmed_at"], "source_bundle.confirmed_at"
            )
            required_text(
                source_bundle["confirmation_note"],
                "source_bundle.confirmation_note",
            )
            bundle_task_asset_id = required_positive_int(
                source_bundle["task_asset_id"], "source_bundle.task_asset_id"
            )
            if bundle_task_asset_id in allocated_task_assets:
                raise ReceiptError(
                    f"duplicate bundle task asset allocation {bundle_task_asset_id}"
                )
            allocated_task_assets.add(bundle_task_asset_id)
            members: list[dict[str, Any]] = []
            seen_member_ids: set[int] = set()
            for member_index, raw_member in enumerate(
                require_array(source_bundle["members"], "source_bundle.members")
            ):
                member = require_object(
                    raw_member, f"source_bundle.members[{member_index}]"
                )
                require_exact_fields(
                    member,
                    {"confirmed", "sha256", "task_asset_id"},
                    f"source_bundle.members[{member_index}]",
                )
                equal(
                    member["confirmed"],
                    True,
                    f"source_bundle.members[{member_index}].confirmed",
                )
                member_id = required_positive_int(
                    member["task_asset_id"],
                    f"source_bundle.members[{member_index}].task_asset_id",
                )
                if member_id in seen_member_ids:
                    raise ReceiptError(
                        f"source bundle duplicates member {member_id}"
                    )
                seen_member_ids.add(member_id)
                members.append(
                    {
                        "task_asset_id": member_id,
                        "sha256": required_sha256(
                            member["sha256"],
                            f"source_bundle.members[{member_index}].sha256",
                        ),
                    }
                )
            if len(members) < 2:
                raise ReceiptError("source bundle must have at least two members")
            revision_no = required_positive_int(
                revision.get("revision_no"),
                f"resource {resource_index}.history[{history_index}].revision_no",
            )
            locator = (
                f"{task_id}:{scope_kind}:{scope_ref_id}:{revision_no}"
            )
            if locator in result:
                raise ReceiptError(f"duplicate bundle locator {locator}")
            result[locator] = {
                "task_id": task_id,
                "scope_kind": scope_kind,
                "scope_ref_id": scope_ref_id,
                "revision_no": revision_no,
                "bundle_task_asset_id": bundle_task_asset_id,
                "bundle_sha256": source_bundle["bundle_sha256"],
                "internal_manifest_sha256": source_bundle["manifest_sha256"],
                "members": members,
                "source_bundle": source_bundle,
            }
    if not result:
        raise ReceiptError("mapping contains no source bundles")
    return result


def _recovery_mapping(
    mapping: Mapping[str, Any],
) -> dict[int, dict[str, Any]]:
    result: dict[int, dict[str, Any]] = {}
    for index, raw in enumerate(
        require_array(mapping["asset_recoveries"], "mapping.asset_recoveries")
    ):
        recovery = require_object(raw, f"mapping.asset_recoveries[{index}]")
        strategy = required_text(
            recovery.get("strategy"), f"asset_recovery {index}.strategy"
        )
        if strategy != BUNDLE_STRATEGY:
            continue
        require_fields(
            recovery,
            {
                "confidence",
                "confirmed_at",
                "confirmed_by",
                "controlled_read_evidence_sha256",
                "expected_file_size",
                "manifest_row_hash",
                "missing_task_asset_id",
                "recovery_source_sha256",
                "recovery_source_storage_ref_id",
                "recovery_source_task_asset_id",
                "strategy",
                "task_id",
            },
            f"asset_recovery {index}",
        )
        equal(
            recovery["confidence"],
            "confirmed_auto",
            f"asset_recovery {index}.confidence",
        )
        missing_id = required_positive_int(
            recovery["missing_task_asset_id"],
            f"asset_recovery {index}.missing_task_asset_id",
        )
        if missing_id in result:
            raise ReceiptError(f"duplicate materialized recovery {missing_id}")
        required_positive_int(
            recovery["confirmed_by"], f"asset_recovery {index}.confirmed_by"
        )
        required_text(
            recovery["confirmed_at"], f"asset_recovery {index}.confirmed_at"
        )
        required_sha256(
            recovery["controlled_read_evidence_sha256"],
            f"asset_recovery {index}.controlled_read_evidence_sha256",
        )
        required_sha256(
            recovery["recovery_source_sha256"],
            f"asset_recovery {index}.recovery_source_sha256",
        )
        required_sha256(
            recovery["manifest_row_hash"],
            f"asset_recovery {index}.manifest_row_hash",
        )
        required_positive_int(
            recovery["expected_file_size"],
            f"asset_recovery {index}.expected_file_size",
        )
        result[missing_id] = recovery
    if not result:
        raise ReceiptError("mapping contains no prematerialized recoveries")
    return result


def _validate_frozen_a_rows(
    manifest_path: pathlib.Path, required_ids: set[int]
) -> tuple[dict[int, dict[str, Any]], list[str]]:
    manifest = load_json(manifest_path, "frozen A manifest")
    require_exact_fields(
        manifest,
        {
            "database",
            "datasets",
            "evidence_sha256",
            "export_contract",
            "mysql_evidence",
            "schema_version",
            "transaction",
        },
        "frozen A manifest",
    )
    equal(manifest["schema_version"], 2, "frozen A manifest.schema_version")
    equal(
        manifest["export_contract"],
        "frozen_a_oracle_v2",
        "frozen A manifest.export_contract",
    )
    manifest_evidence = validate_self_hash(manifest, "frozen A manifest")
    matches = [
        require_object(item, "frozen A dataset")
        for item in require_array(manifest["datasets"], "frozen A datasets")
        if isinstance(item, dict) and item.get("dataset") == "task_assets"
    ]
    if len(matches) != 1:
        raise ReceiptError("frozen A must contain exactly one task_assets dataset")
    descriptor = matches[0]
    require_exact_fields(
        descriptor,
        {
            "columns_sha256",
            "dataset",
            "dataset_sha256",
            "file",
            "file_sha256",
            "first_key",
            "key",
            "last_key",
            "row_count",
            "schema",
            "schema_sha256",
            "source_table",
        },
        "frozen A task_assets descriptor",
    )
    equal(descriptor["key"], "id", "task_assets descriptor.key")
    equal(
        descriptor["source_table"],
        "task_assets",
        "task_assets descriptor.source_table",
    )
    schema = require_array(descriptor["schema"], "task_assets descriptor.schema")
    equal(
        required_sha256(
            descriptor["schema_sha256"], "task_assets descriptor.schema_sha256"
        ),
        sha256(canonical(schema)),
        "task_assets descriptor.schema_sha256",
    )
    filename = pathlib.PurePosixPath(
        required_text(descriptor["file"], "task_assets descriptor.file")
    )
    if filename.is_absolute() or ".." in filename.parts:
        raise ReceiptError("task_assets descriptor.file escapes package")
    dataset_path = manifest_path.parent.joinpath(*filename.parts)
    if dataset_path.is_symlink():
        raise ReceiptError("task_assets dataset must not be a symlink")
    file_digest = sha256_file(dataset_path)
    equal(
        file_digest,
        required_sha256(
            descriptor["file_sha256"], "task_assets descriptor.file_sha256"
        ),
        "task_assets file_sha256",
    )

    selected: dict[int, dict[str, Any]] = {}
    row_hashes: list[str] = []
    previous_key: int | None = None
    first_key: int | None = None
    last_key: int | None = None
    try:
        with dataset_path.open(encoding="utf-8") as handle:
            for line_no, raw in enumerate(handle, 1):
                if not raw.endswith("\n"):
                    raise ReceiptError(
                        f"task_assets line {line_no} lacks newline terminator"
                    )
                line = require_object(
                    json.loads(raw), f"task_assets line {line_no}"
                )
                require_exact_fields(
                    line,
                    {"dataset", "row", "row_key", "row_sha256"},
                    f"task_assets line {line_no}",
                )
                equal(
                    line["dataset"],
                    "task_assets",
                    f"task_assets line {line_no}.dataset",
                )
                row = require_object(
                    line["row"], f"task_assets line {line_no}.row"
                )
                key = required_positive_int(
                    line["row_key"], f"task_assets line {line_no}.row_key"
                )
                equal(
                    required_positive_int(
                        row.get("id"), f"task_assets line {line_no}.row.id"
                    ),
                    key,
                    f"task_assets line {line_no} key",
                )
                if previous_key is not None and key <= previous_key:
                    raise ReceiptError(
                        f"task_assets keys duplicate or out of order at {key}"
                    )
                row_hash = required_sha256(
                    line["row_sha256"],
                    f"task_assets line {line_no}.row_sha256",
                )
                expected_row_hash = sha256(
                    canonical(
                        {
                            "dataset": "task_assets",
                            "row": row,
                            "schema_version": 2,
                        }
                    )
                )
                equal(
                    row_hash,
                    expected_row_hash,
                    f"task_assets line {line_no}.row_sha256",
                )
                if key in required_ids:
                    if key in selected:
                        raise ReceiptError(f"duplicate frozen A task asset {key}")
                    selected[key] = row
                row_hashes.append(row_hash)
                if first_key is None:
                    first_key = key
                last_key = key
                previous_key = key
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise ReceiptError(f"cannot validate task_assets dataset: {exc}") from exc

    equal(
        len(row_hashes),
        required_int(descriptor["row_count"], "task_assets row_count"),
        "task_assets row_count",
    )
    equal(first_key, descriptor["first_key"], "task_assets first_key")
    equal(last_key, descriptor["last_key"], "task_assets last_key")
    dataset_digest = sha256(
        "".join(f"{item}\n" for item in row_hashes).encode("ascii")
    )
    equal(
        dataset_digest,
        required_sha256(
            descriptor["dataset_sha256"],
            "task_assets descriptor.dataset_sha256",
        ),
        "task_assets dataset_sha256",
    )
    missing = required_ids - set(selected)
    if missing:
        raise ReceiptError(
            f"frozen A task_assets lacks required rows {sorted(missing)}"
        )
    return selected, [
        manifest_evidence,
        file_digest,
        dataset_digest,
    ]


def _load_ownership_receipts(
    paths: Iterable[pathlib.Path],
    run_id: str,
    label: str,
    *,
    canonical_newline: bool,
) -> list[tuple[pathlib.Path, dict[str, Any], str]]:
    result: list[tuple[pathlib.Path, dict[str, Any], str]] = []
    seen_paths: set[str] = set()
    for index, path in enumerate(paths):
        key = normalized_path(path)
        if key in seen_paths:
            raise ReceiptError(f"duplicate {label} path {path}")
        seen_paths.add(key)
        document = load_json(path, f"{label} {index}")
        require_exact_fields(
            document,
            {
                "device",
                "evidence_sha256",
                "inode",
                "run_id",
                "schema_version",
                "sha256",
                "size",
                "staging_path",
                "status",
                "target_path",
            },
            f"{label} {index}",
        )
        equal(document["schema_version"], 1, f"{label} {index}.schema_version")
        equal(document["run_id"], run_id, f"{label} {index}.run_id")
        equal(document["status"], "OWNED_LINK", f"{label} {index}.status")
        required_positive_int(document["device"], f"{label} {index}.device")
        required_positive_int(document["inode"], f"{label} {index}.inode")
        required_sha256(document["sha256"], f"{label} {index}.sha256")
        required_positive_int(document["size"], f"{label} {index}.size")
        required_text(
            document["staging_path"], f"{label} {index}.staging_path"
        )
        required_text(document["target_path"], f"{label} {index}.target_path")
        evidence = validate_self_hash(
            document,
            f"{label} {index}",
            canonical_newline=canonical_newline,
        )
        result.append((path, document, evidence))
    return result


def _bundle_entries(
    bundle_mapping: Mapping[str, dict[str, Any]],
    frozen_rows: Mapping[int, dict[str, Any]],
    registry_path: pathlib.Path,
    ownership_paths: Iterable[pathlib.Path],
) -> tuple[list[dict[str, Any]], list[str]]:
    registry = load_json(registry_path, "bundle registry")
    require_exact_fields(
        registry,
        {
            "b_root",
            "database_write_performed",
            "entries",
            "evidence_sha256",
            "manifest_sha256",
            "run_id",
            "schema_version",
            "status",
            "write_ahead_sha256",
        },
        "bundle registry",
    )
    equal(registry["schema_version"], 1, "bundle registry.schema_version")
    equal(registry["status"], "MATERIALIZED", "bundle registry.status")
    equal(
        registry["database_write_performed"],
        False,
        "bundle registry.database_write_performed",
    )
    run_id = required_text(registry["run_id"], "bundle registry.run_id")
    b_root = pathlib.Path(required_text(registry["b_root"], "bundle registry.b_root"))
    required_sha256(registry["manifest_sha256"], "bundle registry.manifest_sha256")
    required_sha256(
        registry["write_ahead_sha256"], "bundle registry.write_ahead_sha256"
    )
    registry_evidence = validate_self_hash(
        registry, "bundle registry", canonical_newline=True
    )
    ownerships = _load_ownership_receipts(
        ownership_paths,
        run_id,
        "bundle ownership receipt",
        canonical_newline=True,
    )
    ownership_by_target: dict[str, tuple[pathlib.Path, dict[str, Any], str]] = {}
    for item in ownerships:
        target = normalized_path(pathlib.Path(item[1]["target_path"]))
        if target in ownership_by_target:
            raise ReceiptError(f"duplicate bundle ownership target {target}")
        ownership_by_target[target] = item

    result: list[dict[str, Any]] = []
    archive_evidence: list[str] = []
    consumed: set[str] = set()
    seen: set[str] = set()
    for index, raw in enumerate(
        require_array(registry["entries"], "bundle registry.entries")
    ):
        entry = require_object(raw, f"bundle registry.entries[{index}]")
        require_exact_fields(
            entry,
            {
                "asset_storage_ref_candidate",
                "bundle_sha256",
                "disposition",
                "object_key",
                "relative_object_path",
                "revision_no",
                "rollback_candidate",
                "scope_kind",
                "scope_ref_id",
                "size",
                "source_bundle",
                "task_asset_candidate",
                "task_id",
            },
            f"bundle registry.entries[{index}]",
        )
        task_id = required_positive_int(entry["task_id"], f"bundle entry {index}.task_id")
        scope_kind = required_text(
            entry["scope_kind"], f"bundle entry {index}.scope_kind"
        )
        scope_ref_id = required_int(
            entry["scope_ref_id"], f"bundle entry {index}.scope_ref_id"
        )
        revision_no = required_positive_int(
            entry["revision_no"], f"bundle entry {index}.revision_no"
        )
        locator = f"{task_id}:{scope_kind}:{scope_ref_id}:{revision_no}"
        if locator in seen:
            raise ReceiptError(f"bundle registry duplicates locator {locator}")
        seen.add(locator)
        expected = bundle_mapping.get(locator)
        if expected is None:
            raise ReceiptError(f"unconsumed bundle registry locator {locator}")
        equal(
            entry["source_bundle"],
            expected["source_bundle"],
            f"bundle {locator}.source_bundle",
        )
        bundle_sha = required_sha256(
            entry["bundle_sha256"], f"bundle {locator}.bundle_sha256"
        )
        equal(bundle_sha, expected["bundle_sha256"], f"bundle {locator}.bundle_sha256")
        size = required_positive_int(entry["size"], f"bundle {locator}.size")
        object_key = required_text(
            entry["object_key"], f"bundle {locator}.object_key"
        )
        relative = pathlib.PurePosixPath(
            required_text(
                entry["relative_object_path"],
                f"bundle {locator}.relative_object_path",
            )
        )
        if relative.is_absolute() or ".." in relative.parts:
            raise ReceiptError(f"bundle {locator} relative path escapes b_root")
        expected_target = b_root.joinpath(*relative.parts)
        task_candidate = require_object(
            entry["task_asset_candidate"],
            f"bundle {locator}.task_asset_candidate",
        )
        require_fields(
            task_candidate,
            {
                "asset_id",
                "asset_type",
                "file_size",
                "id",
                "mime_type",
                "storage_key",
                "storage_ref_id",
                "whole_hash",
            },
            f"bundle {locator}.task_asset_candidate",
        )
        bundle_task_asset_id = required_positive_int(
            task_candidate["id"], f"bundle {locator}.task_asset_candidate.id"
        )
        equal(
            bundle_task_asset_id,
            expected["bundle_task_asset_id"],
            f"bundle {locator}.bundle_task_asset_id",
        )
        bundle_root_asset_id = required_positive_int(
            task_candidate["asset_id"],
            f"bundle {locator}.task_asset_candidate.asset_id",
        )
        equal(
            task_candidate["asset_type"],
            "source",
            f"bundle {locator}.task_asset_candidate.asset_type",
        )
        equal(
            required_positive_int(
                task_candidate["file_size"],
                f"bundle {locator}.task_asset_candidate.file_size",
            ),
            size,
            f"bundle {locator}.task_asset_candidate.file_size",
        )
        equal(
            task_candidate["mime_type"],
            "application/zip",
            f"bundle {locator}.task_asset_candidate.mime_type",
        )
        equal(
            task_candidate["storage_key"],
            object_key,
            f"bundle {locator}.task_asset_candidate.storage_key",
        )
        equal(
            task_candidate["whole_hash"],
            bundle_sha,
            f"bundle {locator}.task_asset_candidate.whole_hash",
        )
        storage_ref_id = required_text(
            task_candidate["storage_ref_id"],
            f"bundle {locator}.task_asset_candidate.storage_ref_id",
        )
        storage_candidate = require_object(
            entry["asset_storage_ref_candidate"],
            f"bundle {locator}.asset_storage_ref_candidate",
        )
        require_fields(
            storage_candidate,
            {
                "checksum_hint",
                "file_size",
                "mime_type",
                "ref_id",
                "ref_key",
            },
            f"bundle {locator}.asset_storage_ref_candidate",
        )
        equal(storage_candidate["ref_id"], storage_ref_id, f"bundle {locator}.storage.ref_id")
        equal(storage_candidate["ref_key"], object_key, f"bundle {locator}.storage.ref_key")
        equal(
            storage_candidate["checksum_hint"],
            bundle_sha,
            f"bundle {locator}.storage.checksum_hint",
        )
        equal(
            required_positive_int(
                storage_candidate["file_size"], f"bundle {locator}.storage.file_size"
            ),
            size,
            f"bundle {locator}.storage.file_size",
        )
        equal(
            storage_candidate["mime_type"],
            "application/zip",
            f"bundle {locator}.storage.mime_type",
        )
        rollback = require_object(
            entry["rollback_candidate"], f"bundle {locator}.rollback_candidate"
        )
        require_fields(
            rollback,
            {
                "expected_sha256",
                "ownership_receipt_path",
                "relative_object_path",
                "storage_ref_id",
                "task_asset_id",
            },
            f"bundle {locator}.rollback_candidate",
        )
        equal(
            required_positive_int(
                rollback["task_asset_id"], f"bundle {locator}.rollback.task_asset_id"
            ),
            bundle_task_asset_id,
            f"bundle {locator}.rollback.task_asset_id",
        )
        equal(
            rollback["storage_ref_id"],
            storage_ref_id,
            f"bundle {locator}.rollback.storage_ref_id",
        )
        equal(
            rollback["relative_object_path"],
            str(relative),
            f"bundle {locator}.rollback.relative_object_path",
        )
        equal(
            rollback["expected_sha256"],
            bundle_sha,
            f"bundle {locator}.rollback.expected_sha256",
        )
        target_key = normalized_path(expected_target)
        ownership = ownership_by_target.get(target_key)
        if ownership is None:
            raise ReceiptError(f"bundle {locator} lacks ownership receipt")
        ownership_path, ownership_document, _ = ownership
        assert_receipt_path(
            rollback["ownership_receipt_path"],
            ownership_path,
            f"bundle {locator}.rollback.ownership_receipt_path",
        )
        equal(
            ownership_document["sha256"],
            bundle_sha,
            f"bundle {locator}.ownership.sha256",
        )
        equal(
            required_positive_int(
                ownership_document["size"], f"bundle {locator}.ownership.size"
            ),
            size,
            f"bundle {locator}.ownership.size",
        )
        local_archive = _resolve_evidence_target(
            pathlib.Path(ownership_document["target_path"]),
            registry_path,
            f"bundle {locator} archive",
        )
        receipt_members, embedded_manifest_sha256 = _validate_bundle_archive(
            archive_path=local_archive,
            bundle_sha256=bundle_sha,
            bundle_size=size,
            source_bundle=expected["source_bundle"],
            frozen_rows=frozen_rows,
            task_id=task_id,
            label=f"bundle {locator}",
        )
        archive_evidence.append(embedded_manifest_sha256)
        consumed.add(target_key)
        result.append(
            {
                "task_id": task_id,
                "scope_kind": scope_kind,
                "scope_ref_id": scope_ref_id,
                "revision_no": revision_no,
                "bundle_task_asset_id": bundle_task_asset_id,
                "bundle_root_asset_id": bundle_root_asset_id,
                "bundle_storage_ref_id": storage_ref_id,
                "object_key_sha256": object_key_sha256(object_key),
                "bundle_sha256": bundle_sha,
                "internal_manifest_sha256": expected[
                    "internal_manifest_sha256"
                ],
                "size": size,
                "mime_type": "application/zip",
                "members": receipt_members,
            }
        )

    missing = set(bundle_mapping) - seen
    if missing:
        raise ReceiptError(f"bundle registry lacks locators {sorted(missing)}")
    unused = set(ownership_by_target) - consumed
    if unused:
        raise ReceiptError(f"unconsumed bundle ownership receipts {sorted(unused)}")
    if len(ownerships) != len(bundle_mapping):
        raise ReceiptError("bundle ownership receipt count differs from mapping")
    result.sort(
        key=lambda item: (
            item["task_id"],
            item["scope_kind"],
            item["scope_ref_id"],
            item["revision_no"],
        )
    )
    return (
        result,
        [registry_evidence]
        + [item[2] for item in ownerships]
        + archive_evidence,
    )


def _controlled_read_by_missing_id(
    evidence_path: pathlib.Path,
    mapping_sha256: str,
) -> tuple[dict[int, dict[str, Any]], list[str]]:
    document = load_json(evidence_path, "recovery controlled-read evidence")
    require_fields(
        document,
        {
            "controlled_read_receipts_sha256",
            "database_writes_executed",
            "evidence_sha256",
            "mapping_sha256",
            "production_connections_opened",
            "recoveries",
            "run_id",
            "status",
            "version",
        },
        "recovery controlled-read evidence",
    )
    equal(document["version"], 1, "recovery evidence.version")
    equal(document["status"], "PASS", "recovery evidence.status")
    equal(
        document["mapping_sha256"],
        mapping_sha256,
        "recovery evidence.mapping_sha256",
    )
    equal(
        document["database_writes_executed"],
        False,
        "recovery evidence.database_writes_executed",
    )
    equal(
        document["production_connections_opened"],
        False,
        "recovery evidence.production_connections_opened",
    )
    aggregate = required_sha256(
        document["controlled_read_receipts_sha256"],
        "recovery evidence.controlled_read_receipts_sha256",
    )
    evidence_hash = validate_self_hash(
        document, "recovery controlled-read evidence"
    )
    result: dict[int, dict[str, Any]] = {}
    individual_hashes: list[str] = []
    for index, raw in enumerate(
        require_array(document["recoveries"], "recovery evidence.recoveries")
    ):
        recovery = require_object(raw, f"recovery evidence.recoveries[{index}]")
        require_fields(
            recovery,
            {
                "missing_task_asset_id",
                "source_fetch_receipt",
                "source_sha256",
                "source_task_asset",
            },
            f"recovery evidence.recoveries[{index}]",
        )
        missing_id = required_positive_int(
            recovery["missing_task_asset_id"],
            f"recovery evidence {index}.missing_task_asset_id",
        )
        if missing_id in result:
            raise ReceiptError(
                f"recovery evidence duplicates missing asset {missing_id}"
            )
        source = require_object(
            recovery["source_task_asset"],
            f"recovery evidence {index}.source_task_asset",
        )
        require_fields(
            source,
            {
                "file_size",
                "id",
                "mime_type",
                "storage_ref_id",
                "task_id",
            },
            f"recovery evidence {index}.source_task_asset",
        )
        fetch = require_object(
            recovery["source_fetch_receipt"],
            f"recovery evidence {index}.source_fetch_receipt",
        )
        require_fields(
            fetch,
            {
                "mime_type",
                "sha256",
                "size",
                "storage_ref_id",
                "task_asset_id",
            },
            f"recovery evidence {index}.source_fetch_receipt",
        )
        content_sha = required_sha256(
            recovery["source_sha256"],
            f"recovery evidence {index}.source_sha256",
        )
        equal(
            required_sha256(
                fetch["sha256"],
                f"recovery evidence {index}.source_fetch_receipt.sha256",
            ),
            content_sha,
            f"recovery evidence {index}.source_fetch_receipt.sha256",
        )
        for fetch_field, source_field in (
            ("size", "file_size"),
            ("mime_type", "mime_type"),
            ("storage_ref_id", "storage_ref_id"),
        ):
            equal(
                fetch[fetch_field],
                source[source_field],
                f"recovery evidence {index}.source_fetch_receipt.{fetch_field}",
            )
        equal(
            required_positive_int(
                fetch["task_asset_id"],
                f"recovery evidence {index}.source_fetch_receipt.task_asset_id",
            ),
            required_positive_int(
                source["id"], f"recovery evidence {index}.source_task_asset.id"
            ),
            f"recovery evidence {index}.source_fetch_receipt.task_asset_id",
        )
        fetch_hash = fetch.get("evidence_sha256")
        if fetch_hash is not None:
            fetch_hash = required_sha256(
                fetch_hash,
                f"recovery evidence {index}.source_fetch_receipt.evidence_sha256",
            )
            individual_hashes.append(fetch_hash)
        result[missing_id] = {
            "source": source,
            "fetch": fetch,
            "content_sha256": content_sha,
        }
    # The aggregate is the reviewed source-receipt identity even when the
    # embedded per-fetch objects predate individual evidence_sha256 fields.
    return result, [evidence_hash, aggregate] + individual_hashes


def _recovery_entries(
    recovery_mapping: Mapping[int, dict[str, Any]],
    frozen_rows: Mapping[int, dict[str, Any]],
    plan_path: pathlib.Path,
    evidence_path: pathlib.Path,
    ownership_paths: Iterable[pathlib.Path],
    mapping_sha256: str,
) -> tuple[list[dict[str, Any]], list[str]]:
    plan = load_json(plan_path, "recovery materialization plan")
    require_exact_fields(
        plan,
        {
            "database_writes_executed",
            "entries",
            "evidence_sha256",
            "mapping_sha256",
            "production_writes_executed",
            "run_id",
            "status",
            "version",
        },
        "recovery materialization plan",
    )
    equal(plan["version"], 1, "recovery plan.version")
    equal(plan["status"], "MATERIALIZED", "recovery plan.status")
    equal(plan["mapping_sha256"], mapping_sha256, "recovery plan.mapping_sha256")
    equal(
        plan["database_writes_executed"],
        False,
        "recovery plan.database_writes_executed",
    )
    equal(
        plan["production_writes_executed"],
        False,
        "recovery plan.production_writes_executed",
    )
    run_id = required_text(plan["run_id"], "recovery plan.run_id")
    plan_evidence = validate_self_hash(plan, "recovery materialization plan")
    controlled, controlled_evidence = _controlled_read_by_missing_id(
        evidence_path, mapping_sha256
    )
    ownerships = _load_ownership_receipts(
        ownership_paths,
        run_id,
        "recovery ownership receipt",
        canonical_newline=False,
    )
    ownership_by_target: dict[str, tuple[pathlib.Path, dict[str, Any], str]] = {}
    for item in ownerships:
        target = normalized_path(pathlib.Path(item[1]["target_path"]))
        if target in ownership_by_target:
            raise ReceiptError(f"duplicate recovery ownership target {target}")
        ownership_by_target[target] = item

    result: list[dict[str, Any]] = []
    seen: set[int] = set()
    consumed_targets: set[str] = set()
    for index, raw in enumerate(
        require_array(plan["entries"], "recovery plan.entries")
    ):
        entry = require_object(raw, f"recovery plan.entries[{index}]")
        require_exact_fields(
            entry,
            {
                "db_apply_plan",
                "derivative_lineage",
                "missing_task_asset_id",
                "rollback_registry",
                "source_local_path",
                "source_sha256",
                "source_size",
                "source_task_asset_id",
                "target_object_key",
                "target_storage_ref_id",
            },
            f"recovery plan.entries[{index}]",
        )
        missing_id = required_positive_int(
            entry["missing_task_asset_id"],
            f"recovery plan entry {index}.missing_task_asset_id",
        )
        if missing_id in seen:
            raise ReceiptError(f"recovery plan duplicates {missing_id}")
        seen.add(missing_id)
        mapping_item = recovery_mapping.get(missing_id)
        if mapping_item is None:
            raise ReceiptError(f"unconsumed recovery plan entry {missing_id}")
        controlled_item = controlled.get(missing_id)
        if controlled_item is None:
            raise ReceiptError(
                f"recovery {missing_id} lacks controlled-read evidence"
            )
        source_id = required_positive_int(
            entry["source_task_asset_id"],
            f"recovery {missing_id}.source_task_asset_id",
        )
        equal(
            source_id,
            required_positive_int(
                mapping_item["recovery_source_task_asset_id"],
                f"recovery {missing_id}.mapping.source_task_asset_id",
            ),
            f"recovery {missing_id}.source_task_asset_id",
        )
        source_row = frozen_rows[source_id]
        target_row = frozen_rows[missing_id]
        source_task_id = required_positive_int(
            source_row.get("task_id"), f"frozen A source {source_id}.task_id"
        )
        source_storage_ref_id = required_text(
            source_row.get("storage_ref_id"),
            f"frozen A source {source_id}.storage_ref_id",
        )
        source_size = required_positive_int(
            source_row.get("file_size"),
            f"frozen A source {source_id}.file_size",
        )
        source_mime = required_text(
            source_row.get("mime_type"),
            f"frozen A source {source_id}.mime_type",
        )
        if source_row.get("deleted_at") is not None:
            raise ReceiptError(f"frozen A source {source_id} is deleted")
        if source_row.get("object_deleted_at") is not None:
            raise ReceiptError(f"frozen A source {source_id} object is deleted")
        content_sha = required_sha256(
            controlled_item["content_sha256"],
            f"controlled recovery {missing_id}.content_sha256",
        )
        # Legacy recovered source rows have whole_hash=NULL.  If a frozen row
        # carries a hash, it must agree, but the controlled read remains the
        # content authority.
        row_hash = source_row.get("whole_hash")
        if row_hash not in (None, ""):
            equal(
                required_sha256(row_hash, f"frozen A source {source_id}.whole_hash"),
                content_sha,
                f"frozen A source {source_id}.whole_hash",
            )
        controlled_source = controlled_item["source"]
        for actual, expected, label in (
            (controlled_source["id"], source_id, "id"),
            (controlled_source["task_id"], source_task_id, "task_id"),
            (
                controlled_source["storage_ref_id"],
                source_storage_ref_id,
                "storage_ref_id",
            ),
            (controlled_source["file_size"], source_size, "file_size"),
            (controlled_source["mime_type"], source_mime, "mime_type"),
        ):
            if label in {"id", "task_id", "file_size"}:
                actual = required_positive_int(
                    actual, f"controlled recovery {missing_id}.{label}"
                )
            equal(
                actual,
                expected,
                f"controlled recovery {missing_id}.source_task_asset.{label}",
            )
        for actual, expected, label in (
            (entry["source_sha256"], content_sha, "source_sha256"),
            (entry["source_size"], source_size, "source_size"),
            (
                mapping_item["recovery_source_sha256"],
                content_sha,
                "mapping.recovery_source_sha256",
            ),
            (
                mapping_item["recovery_source_storage_ref_id"],
                source_storage_ref_id,
                "mapping.recovery_source_storage_ref_id",
            ),
            (
                mapping_item["expected_file_size"],
                source_size,
                "mapping.expected_file_size",
            ),
        ):
            if label in {"source_size", "mapping.expected_file_size"}:
                actual = required_positive_int(
                    actual, f"recovery {missing_id}.{label}"
                )
            equal(actual, expected, f"recovery {missing_id}.{label}")
        target_task_id = required_positive_int(
            target_row.get("task_id"), f"frozen A target {missing_id}.task_id"
        )
        equal(
            target_task_id,
            required_positive_int(
                mapping_item["task_id"], f"recovery {missing_id}.mapping.task_id"
            ),
            f"recovery {missing_id}.target_task_id",
        )
        target_root_asset_id = required_positive_int(
            target_row.get("asset_id"),
            f"frozen A target {missing_id}.asset_id",
        )
        target_object_key = required_text(
            entry["target_object_key"],
            f"recovery {missing_id}.target_object_key",
        )
        target_storage_ref_id = required_text(
            entry["target_storage_ref_id"],
            f"recovery {missing_id}.target_storage_ref_id",
        )
        db_plan = require_object(
            entry["db_apply_plan"], f"recovery {missing_id}.db_apply_plan"
        )
        require_fields(
            db_plan,
            {"insert_asset_storage_ref", "update_task_asset", "update_upload_request"},
            f"recovery {missing_id}.db_apply_plan",
        )
        insert_ref = require_object(
            db_plan["insert_asset_storage_ref"],
            f"recovery {missing_id}.insert_asset_storage_ref",
        )
        require_fields(
            insert_ref,
            {
                "asset_id",
                "checksum_hint",
                "file_size",
                "mime_type",
                "ref_id",
                "ref_key",
            },
            f"recovery {missing_id}.insert_asset_storage_ref",
        )
        for actual, expected, label in (
            (insert_ref["asset_id"], target_root_asset_id, "asset_id"),
            (insert_ref["checksum_hint"], content_sha, "checksum_hint"),
            (insert_ref["file_size"], source_size, "file_size"),
            (insert_ref["mime_type"], source_mime, "mime_type"),
            (insert_ref["ref_id"], target_storage_ref_id, "ref_id"),
            (insert_ref["ref_key"], target_object_key, "ref_key"),
        ):
            if label in {"asset_id", "file_size"}:
                actual = required_positive_int(
                    actual, f"recovery {missing_id}.insert.{label}"
                )
            equal(actual, expected, f"recovery {missing_id}.insert.{label}")
        update_asset = require_object(
            db_plan["update_task_asset"],
            f"recovery {missing_id}.update_task_asset",
        )
        require_fields(
            update_asset,
            {"set", "where"},
            f"recovery {missing_id}.update_task_asset",
        )
        where = require_object(
            update_asset["where"], f"recovery {missing_id}.update.where"
        )
        equal(
            required_positive_int(where.get("id"), f"recovery {missing_id}.where.id"),
            missing_id,
            f"recovery {missing_id}.where.id",
        )
        update_set = require_object(
            update_asset["set"], f"recovery {missing_id}.update.set"
        )
        require_fields(
            update_set,
            {
                "storage_key",
                "storage_ref_id",
                "whole_hash",
            },
            f"recovery {missing_id}.update.set",
        )
        for actual, expected, label in (
            (update_set["storage_key"], target_object_key, "storage_key"),
            (
                update_set["storage_ref_id"],
                target_storage_ref_id,
                "storage_ref_id",
            ),
            (update_set["whole_hash"], content_sha, "whole_hash"),
        ):
            equal(actual, expected, f"recovery {missing_id}.update.{label}")
        if "file_size" in update_set:
            equal(
                required_positive_int(
                    update_set["file_size"],
                    f"recovery {missing_id}.update.file_size",
                ),
                source_size,
                f"recovery {missing_id}.update.file_size",
            )
        if "mime_type" in update_set:
            equal(
                update_set["mime_type"],
                source_mime,
                f"recovery {missing_id}.update.mime_type",
            )
        rollback = require_object(
            entry["rollback_registry"], f"recovery {missing_id}.rollback_registry"
        )
        require_fields(
            rollback,
            {"expected_fixture_sha256", "ownership_receipt_path"},
            f"recovery {missing_id}.rollback_registry",
        )
        equal(
            rollback["expected_fixture_sha256"],
            content_sha,
            f"recovery {missing_id}.rollback.expected_fixture_sha256",
        )
        expected_suffix = pathlib.PurePosixPath(
            "objects", target_object_key
        ).parts
        candidates = [
            item
            for target, item in ownership_by_target.items()
            if pathlib.PurePosixPath(target).parts[-len(expected_suffix) :]
            == expected_suffix
        ]
        if len(candidates) != 1:
            raise ReceiptError(
                f"recovery {missing_id} requires one ownership target ending "
                f"objects/{target_object_key}, found {len(candidates)}"
            )
        ownership_path, ownership_document, _ = candidates[0]
        target_key = normalized_path(pathlib.Path(ownership_document["target_path"]))
        assert_receipt_path(
            rollback["ownership_receipt_path"],
            ownership_path,
            f"recovery {missing_id}.rollback.ownership_receipt_path",
        )
        equal(
            ownership_document["sha256"],
            content_sha,
            f"recovery {missing_id}.ownership.sha256",
        )
        equal(
            required_positive_int(
                ownership_document["size"],
                f"recovery {missing_id}.ownership.size",
            ),
            source_size,
            f"recovery {missing_id}.ownership.size",
        )
        source_receipt_sha = required_sha256(
            mapping_item["controlled_read_evidence_sha256"],
            f"recovery {missing_id}.controlled_read_evidence_sha256",
        )
        controlled_evidence.append(source_receipt_sha)
        consumed_targets.add(target_key)
        result.append(
            {
                "missing_task_asset_id": missing_id,
                "target_root_asset_id": target_root_asset_id,
                "target_task_id": target_task_id,
                "target_storage_ref_id": target_storage_ref_id,
                "target_object_key_sha256": object_key_sha256(target_object_key),
                "target_content_sha256": content_sha,
                "target_size": source_size,
                "target_mime": source_mime,
                "source_task_asset_id": source_id,
                "source_task_id": source_task_id,
                "source_storage_ref_id": source_storage_ref_id,
                "source_content_sha256": content_sha,
                "source_size": source_size,
                "source_mime": source_mime,
                "strategy": BUNDLE_STRATEGY,
                "source_receipt_sha256": source_receipt_sha,
            }
        )
    missing_plan = set(recovery_mapping) - seen
    if missing_plan:
        raise ReceiptError(f"recovery plan lacks entries {sorted(missing_plan)}")
    unused_controlled = set(controlled) - seen
    if unused_controlled:
        raise ReceiptError(
            f"unconsumed controlled-read recoveries {sorted(unused_controlled)}"
        )
    unused_ownership = set(ownership_by_target) - consumed_targets
    if unused_ownership:
        raise ReceiptError(
            f"unconsumed recovery ownership receipts {sorted(unused_ownership)}"
        )
    if len(ownerships) != len(recovery_mapping):
        raise ReceiptError("recovery ownership receipt count differs from mapping")
    result.sort(key=lambda item: item["missing_task_asset_id"])
    return (
        result,
        [plan_evidence]
        + controlled_evidence
        + [item[2] for item in ownerships],
    )


def _receipt(
    *,
    kind: str,
    mapping_sha256: str,
    reviewed_manifest_sha256: str,
    source_evidence_sha256: Iterable[str],
    entries: list[dict[str, Any]],
) -> dict[str, Any]:
    evidence = sorted(set(source_evidence_sha256))
    if not evidence:
        raise ReceiptError(f"{kind} requires source evidence")
    for index, digest in enumerate(evidence):
        required_sha256(digest, f"{kind}.source_evidence_sha256[{index}]")
    unsigned: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "kind": kind,
        "status": "approved",
        "mapping_sha256": mapping_sha256,
        "reviewed_manifest_sha256": reviewed_manifest_sha256,
        "source_evidence_sha256": evidence,
        "entries": entries,
    }
    result = dict(unsigned)
    result["evidence_sha256"] = sha256(canonical(unsigned))
    return result


def build_receipts(
    *,
    reviewed_mapping_path: pathlib.Path,
    expected_mapping_sha256: str,
    reviewed_manifest_path: pathlib.Path,
    a_snapshot_manifest_path: pathlib.Path,
    bundle_registry_path: pathlib.Path,
    bundle_ownership_paths: Iterable[pathlib.Path],
    recovery_plan_path: pathlib.Path,
    recovery_evidence_path: pathlib.Path,
    recovery_ownership_paths: Iterable[pathlib.Path],
) -> tuple[dict[str, Any], dict[str, Any]]:
    """Validate all evidence and return bundle/recovery schema-v2 receipts."""
    expected_mapping_sha256 = required_sha256(
        expected_mapping_sha256, "expected_mapping_sha256"
    )
    mapping_bytes = reviewed_mapping_path.read_bytes()
    mapping_sha256 = sha256(mapping_bytes)
    equal(mapping_sha256, expected_mapping_sha256, "reviewed mapping SHA-256")
    mapping = require_object(json.loads(mapping_bytes), "reviewed mapping")
    bundles = _bundle_mapping(mapping)
    recoveries = _recovery_mapping(mapping)
    reviewed_manifest_sha256 = _load_reviewed_manifest(
        reviewed_manifest_path, mapping_sha256, set(bundles)
    )
    required_frozen_ids = set(recoveries)
    required_frozen_ids.update(
        required_positive_int(
            item["recovery_source_task_asset_id"],
            "mapping recovery_source_task_asset_id",
        )
        for item in recoveries.values()
    )
    required_frozen_ids.update(
        member["task_asset_id"]
        for bundle in bundles.values()
        for member in bundle["members"]
    )
    frozen_rows, frozen_evidence = _validate_frozen_a_rows(
        a_snapshot_manifest_path, required_frozen_ids
    )
    bundle_entries, bundle_evidence = _bundle_entries(
        bundles,
        frozen_rows,
        bundle_registry_path,
        tuple(bundle_ownership_paths),
    )
    recovery_entries, recovery_evidence = _recovery_entries(
        recoveries,
        frozen_rows,
        recovery_plan_path,
        recovery_evidence_path,
        tuple(recovery_ownership_paths),
        mapping_sha256,
    )
    return (
        _receipt(
            kind=BUNDLE_KIND,
            mapping_sha256=mapping_sha256,
            reviewed_manifest_sha256=reviewed_manifest_sha256,
            source_evidence_sha256=bundle_evidence,
            entries=bundle_entries,
        ),
        _receipt(
            kind=RECOVERY_KIND,
            mapping_sha256=mapping_sha256,
            reviewed_manifest_sha256=reviewed_manifest_sha256,
            source_evidence_sha256=frozen_evidence + recovery_evidence,
            entries=recovery_entries,
        ),
    )


def write_immutable(path: pathlib.Path, document: Mapping[str, Any]) -> None:
    """Atomically create an immutable evidence file, allowing identical reruns."""
    if path.is_symlink():
        raise ReceiptError(f"output must not be a symlink: {path}")
    data = canonical(document)
    if path.exists():
        if path.read_bytes() == data:
            return
        raise ReceiptError(f"refusing to replace non-identical receipt {path}")
    if not path.parent.is_dir():
        raise ReceiptError(f"output directory does not exist: {path.parent}")
    with tempfile.NamedTemporaryFile(
        mode="wb", dir=path.parent, prefix=f".{path.name}.", delete=False
    ) as handle:
        temporary = pathlib.Path(handle.name)
        handle.write(data)
        handle.flush()
        os.fsync(handle.fileno())
    try:
        os.replace(temporary, path)
    except BaseException:
        temporary.unlink(missing_ok=True)
        raise


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Build direct schema-v2 bundle/recovery receipts from offline "
            "materialization evidence; never queries Clone B"
        )
    )
    parser.add_argument("--reviewed-mapping", type=pathlib.Path, required=True)
    parser.add_argument("--expected-mapping-sha256", required=True)
    parser.add_argument("--reviewed-manifest", type=pathlib.Path, required=True)
    parser.add_argument("--a-snapshot-manifest", type=pathlib.Path, required=True)
    parser.add_argument("--bundle-registry", type=pathlib.Path, required=True)
    parser.add_argument(
        "--bundle-ownership-receipt",
        type=pathlib.Path,
        action="append",
        required=True,
    )
    parser.add_argument("--recovery-plan", type=pathlib.Path, required=True)
    parser.add_argument("--recovery-evidence", type=pathlib.Path, required=True)
    parser.add_argument(
        "--recovery-ownership-receipt",
        type=pathlib.Path,
        action="append",
        required=True,
    )
    parser.add_argument("--bundle-output", type=pathlib.Path, required=True)
    parser.add_argument("--recovery-output", type=pathlib.Path, required=True)
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    if normalized_path(args.bundle_output) == normalized_path(args.recovery_output):
        raise ReceiptError("bundle and recovery outputs must be distinct")
    bundle, recovery = build_receipts(
        reviewed_mapping_path=args.reviewed_mapping,
        expected_mapping_sha256=args.expected_mapping_sha256,
        reviewed_manifest_path=args.reviewed_manifest,
        a_snapshot_manifest_path=args.a_snapshot_manifest,
        bundle_registry_path=args.bundle_registry,
        bundle_ownership_paths=args.bundle_ownership_receipt,
        recovery_plan_path=args.recovery_plan,
        recovery_evidence_path=args.recovery_evidence,
        recovery_ownership_paths=args.recovery_ownership_receipt,
    )
    # All validation completes before either output is created.
    bundle_preexisted = args.bundle_output.exists()
    write_immutable(args.bundle_output, bundle)
    try:
        write_immutable(args.recovery_output, recovery)
    except BaseException:
        # Preserve an already-existing identical bundle receipt, but roll back
        # a newly-created bundle file so a failed pair is never half-published.
        if (
            not bundle_preexisted
            and args.bundle_output.exists()
            and args.bundle_output.read_bytes() == canonical(bundle)
        ):
            args.bundle_output.unlink()
        raise
    print(
        json.dumps(
            {
                "bundle_entries": len(bundle["entries"]),
                "bundle_evidence_sha256": bundle["evidence_sha256"],
                "recovery_entries": len(recovery["entries"]),
                "recovery_evidence_sha256": recovery["evidence_sha256"],
            },
            sort_keys=True,
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
