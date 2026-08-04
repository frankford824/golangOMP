#!/usr/bin/env python3
"""Prove post-apply delivery-source aliases reuse verified predecessor objects.

The V8 migration creates one ``task_assets`` source alias per
``(resource-group, delivery predecessor)``.  The alias intentionally reuses the
predecessor's storage reference; it does not create a second object.  This
read-only verifier binds the non-deterministic Clone B alias IDs back to:

* the exact final-reviewed mapping;
* the exact post-hydration G06 object manifest and PASS verdict;
* the exact predecessor task asset and storage reference in Clone B; and
* every revision which must point at the generated alias.

Credentials are accepted only through ``g4_clone_db.Connection``.  The tool
never performs a database write and refuses non-Clone-B connection state.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import re
import sys
from collections import defaultdict
from typing import Any

try:
    from scripts.ab import g4_clone_db
    from scripts.ab import historical_unavailable_exception
    from scripts.ab import object_manifest_verifier
except ModuleNotFoundError:  # Direct execution from scripts/ab.
    import g4_clone_db
    import historical_unavailable_exception
    import object_manifest_verifier


SCHEMA_VERSION = 1
SHA256 = re.compile(r"^[0-9a-f]{64}$")
ZERO_SHA256 = "0" * 64
RUN_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$")
ALIAS_REMARK = re.compile(
    r"^v8-source-alias:group=(?P<group_id>[1-9][0-9]*):"
    r"origin=(?P<origin_id>[1-9][0-9]*)$"
)
POLICY = "delivery_source_alias"
OBJECT_VERDICT_FIELDS = {
    "schema_version",
    "status",
    "violation_count",
    "violations",
    "checked_count",
    "evidence_hash",
    "exception_count",
    "exception_evidence_sha256",
    "exceptions",
    "manifest_sha256",
    "mapping_row_hash",
    "mapping_sha256",
}
OBJECT_VERDICT_EXCEPTION_FIELDS = {
    "entity_key",
    "task_id",
    "missing_task_asset_id",
    "expected_http_status",
    "observed_http_status",
    "mapping_row_hash",
    "object_row_sha256",
    "working_reference_count",
    "finalized_reference_count",
}
ALLOWED_PREDECESSOR_TYPES = {
    "delivery",
    "draft",
    "revised",
    "final",
    "outsource_return",
}
ALLOWED_EXCEPTION_REVISION_STATES = {"rejected", "superseded"}

# These are the columns copied by ensureSourceAlias's INSERT ... SELECT.
COPIED_FIELDS = (
    "task_id",
    "asset_id",
    "scope_sku_code",
    "retouch_requirement_id",
    "asset_version_no",
    "upload_mode",
    "upload_request_id",
    "storage_ref_id",
    "file_name",
    "original_filename",
    "remote_file_id",
    "mime_type",
    "file_size",
    "file_path",
    "storage_key",
    "whole_hash",
    "upload_status",
    "preview_status",
    "uploaded_by",
    "uploaded_at",
)


ALIAS_QUERY = r"""
SELECT JSON_OBJECT(
  'alias', JSON_OBJECT(
    'id', a.id,
    'task_id', a.task_id,
    'asset_id', a.asset_id,
    'scope_sku_code', a.scope_sku_code,
    'retouch_requirement_id', a.retouch_requirement_id,
    'asset_type', a.asset_type,
    'binding_state', a.binding_state,
    'bound_group_id', a.bound_group_id,
    'bound_role', a.bound_role,
    'version_no', a.version_no,
    'asset_version_no', a.asset_version_no,
    'upload_mode', a.upload_mode,
    'upload_request_id', a.upload_request_id,
    'storage_ref_id', a.storage_ref_id,
    'file_name', a.file_name,
    'original_filename', a.original_filename,
    'remote_file_id', a.remote_file_id,
    'mime_type', a.mime_type,
    'file_size', a.file_size,
    'file_path', a.file_path,
    'storage_key', a.storage_key,
    'whole_hash', a.whole_hash,
    'upload_status', a.upload_status,
    'preview_status', a.preview_status,
    'uploaded_by', a.uploaded_by,
    'uploaded_at', DATE_FORMAT(a.uploaded_at, '%Y-%m-%dT%H:%i:%s.%fZ'),
    'remark', a.remark,
    'source_module_key', a.source_module_key,
    'source_task_module_id', a.source_task_module_id,
    'is_archived', a.is_archived,
    'flow_review_status', a.flow_review_status,
    'deleted_at', DATE_FORMAT(a.deleted_at, '%Y-%m-%dT%H:%i:%s.%fZ'),
    'cleaned_at', DATE_FORMAT(a.cleaned_at, '%Y-%m-%dT%H:%i:%s.%fZ'),
    'access_revoked_at', DATE_FORMAT(a.access_revoked_at, '%Y-%m-%dT%H:%i:%s.%fZ'),
    'object_deleted_at', DATE_FORMAT(a.object_deleted_at, '%Y-%m-%dT%H:%i:%s.%fZ')
  ),
  'predecessor', IF(
    p.id IS NULL,
    NULL,
    JSON_OBJECT(
      'id', p.id,
      'task_id', p.task_id,
      'asset_id', p.asset_id,
      'scope_sku_code', p.scope_sku_code,
      'retouch_requirement_id', p.retouch_requirement_id,
      'asset_type', p.asset_type,
      'binding_state', p.binding_state,
      'bound_group_id', p.bound_group_id,
      'bound_role', p.bound_role,
      'version_no', p.version_no,
      'asset_version_no', p.asset_version_no,
      'upload_mode', p.upload_mode,
      'upload_request_id', p.upload_request_id,
      'storage_ref_id', p.storage_ref_id,
      'file_name', p.file_name,
      'original_filename', p.original_filename,
      'remote_file_id', p.remote_file_id,
      'mime_type', p.mime_type,
      'file_size', p.file_size,
      'file_path', p.file_path,
      'storage_key', p.storage_key,
      'whole_hash', p.whole_hash,
      'upload_status', p.upload_status,
      'preview_status', p.preview_status,
      'uploaded_by', p.uploaded_by,
      'uploaded_at', DATE_FORMAT(p.uploaded_at, '%Y-%m-%dT%H:%i:%s.%fZ'),
      'remark', p.remark,
      'source_module_key', p.source_module_key,
      'source_task_module_id', p.source_task_module_id,
      'is_archived', p.is_archived,
      'flow_review_status', p.flow_review_status,
      'deleted_at', DATE_FORMAT(p.deleted_at, '%Y-%m-%dT%H:%i:%s.%fZ'),
      'cleaned_at', DATE_FORMAT(p.cleaned_at, '%Y-%m-%dT%H:%i:%s.%fZ'),
      'access_revoked_at', DATE_FORMAT(p.access_revoked_at, '%Y-%m-%dT%H:%i:%s.%fZ'),
      'object_deleted_at', DATE_FORMAT(p.object_deleted_at, '%Y-%m-%dT%H:%i:%s.%fZ')
    )
  ),
  'resource_group', IF(
    g.id IS NULL,
    NULL,
    JSON_OBJECT(
      'id', g.id,
      'task_id', g.task_id,
      'scope_kind', g.scope_kind,
      'scope_ref_id', g.scope_ref_id,
      'working_revision_id', g.working_revision_id,
      'finalized_revision_id', g.finalized_revision_id
    )
  ),
  'storage_ref', IF(
    s.ref_id IS NULL,
    NULL,
    JSON_OBJECT(
      'ref_id', s.ref_id,
      'asset_id', s.asset_id,
      'owner_type', s.owner_type,
      'owner_id', s.owner_id,
      'upload_request_id', s.upload_request_id,
      'storage_adapter', s.storage_adapter,
      'ref_type', s.ref_type,
      'ref_key', s.ref_key,
      'file_name', s.file_name,
      'mime_type', s.mime_type,
      'file_size', s.file_size,
      'is_placeholder', s.is_placeholder,
      'checksum_hint', s.checksum_hint,
      'status', s.status
    )
  )
)
FROM task_assets a
LEFT JOIN task_asset_groups g ON g.id = a.bound_group_id
LEFT JOIN task_assets p
  ON p.id = CAST(SUBSTRING_INDEX(a.remark, 'origin=', -1) AS UNSIGNED)
LEFT JOIN asset_storage_refs s ON s.ref_id = a.storage_ref_id
WHERE BINARY a.asset_type = BINARY 'source'
  AND BINARY a.source_module_key = BINARY 'migration'
ORDER BY a.id;
"""


USAGE_QUERY = r"""
SELECT JSON_OBJECT(
  'alias_id', a.id,
  'revision_id', r.id,
  'group_id', r.group_id,
  'revision_no', r.revision_no,
  'status', r.status,
  'is_working', IF(g.working_revision_id = r.id, TRUE, FALSE),
  'is_finalized', IF(g.finalized_revision_id = r.id, TRUE, FALSE)
)
FROM task_assets a
JOIN task_asset_group_revisions r ON r.source_task_asset_id = a.id
JOIN task_asset_groups g ON g.id = r.group_id
WHERE BINARY a.asset_type = BINARY 'source'
  AND BINARY a.source_module_key = BINARY 'migration'
ORDER BY a.id, r.revision_no, r.id;
"""


def canonical_bytes(value: Any) -> bytes:
    return (
        json.dumps(
            value,
            ensure_ascii=False,
            sort_keys=True,
            separators=(",", ":"),
        )
        + "\n"
    ).encode("utf-8")


def canonical_value_hash(value: Any) -> str:
    return sha256_bytes(canonical_bytes(value)[:-1])


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def require_file(path: pathlib.Path, label: str) -> pathlib.Path:
    if not path.is_file() or path.is_symlink():
        raise ValueError(f"{label} must be an existing non-symlink file")
    return path.resolve()


def require_expected_hash(
    path: pathlib.Path, expected: str, label: str
) -> str:
    if not SHA256.fullmatch(expected):
        raise ValueError(f"{label} expected SHA-256 is invalid")
    actual = sha256_file(require_file(path, label))
    if actual != expected:
        raise ValueError(f"{label} SHA-256 mismatch")
    return actual


def load_object(path: pathlib.Path, label: str) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ValueError(f"{label} is not valid UTF-8 JSON") from exc
    if not isinstance(value, dict):
        raise ValueError(f"{label} must contain a JSON object")
    return value


def load_jsonl(path: pathlib.Path, label: str) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    try:
        with path.open("r", encoding="utf-8") as handle:
            for line_no, line in enumerate(handle, 1):
                try:
                    row = json.loads(line)
                except json.JSONDecodeError as exc:
                    raise ValueError(
                        f"{label} line {line_no} is invalid JSON"
                    ) from exc
                if not isinstance(row, dict):
                    raise ValueError(
                        f"{label} line {line_no} must be a JSON object"
                    )
                rows.append(row)
    except UnicodeDecodeError as exc:
        raise ValueError(f"{label} is not UTF-8") from exc
    if not rows:
        raise ValueError(f"{label} must not be empty")
    return rows


def parse_json_rows(raw: str, label: str) -> list[dict[str, Any]]:
    result: list[dict[str, Any]] = []
    for line_no, line in enumerate(raw.splitlines(), 1):
        if not line:
            continue
        try:
            row = json.loads(line)
        except json.JSONDecodeError as exc:
            raise ValueError(
                f"{label} row {line_no} is not valid JSON"
            ) from exc
        if not isinstance(row, dict):
            raise ValueError(f"{label} row {line_no} is not an object")
        result.append(row)
    return result


def violation(code: str, entity: str, detail: str) -> dict[str, str]:
    return {
        "violation_code": code,
        "entity_key": entity,
        "detail": detail,
    }


def parse_mapping(
    mapping: dict[str, Any],
) -> dict[tuple[int, str, int, int], dict[str, Any]]:
    if mapping.get("version") != 2 or not isinstance(
        mapping.get("resources"), list
    ):
        raise ValueError("mapping must be a V2 resource mapping")
    expected: dict[tuple[int, str, int, int], dict[str, Any]] = {}
    for group_index, group in enumerate(mapping["resources"]):
        if not isinstance(group, dict) or not isinstance(
            group.get("history"), list
        ):
            raise ValueError(
                f"mapping.resources[{group_index}] is malformed"
            )
        task_id = int(group["task_id"])
        scope_kind = str(group["scope_kind"])
        scope_ref_id = int(group["scope_ref_id"])
        working_no = group.get("working_revision_no")
        finalized_no = group.get("finalized_revision_no")
        for revision in group["history"]:
            origin_value = revision.get("source_alias_from_task_asset_id")
            if origin_value is None:
                continue
            origin_id = int(origin_value)
            policies = revision.get("review_policy_ids")
            if (
                origin_id <= 0
                or not isinstance(policies, list)
                or POLICY not in policies
                or origin_id
                not in [int(value) for value in revision.get(
                    "final_task_asset_ids", []
                )]
            ):
                raise ValueError(
                    "mapping source alias is outside the approved policy "
                    f"contract for task {task_id} revision "
                    f"{revision.get('revision_no')}"
                )
            key = (task_id, scope_kind, scope_ref_id, origin_id)
            entry = expected.setdefault(
                key,
                {
                    "task_id": task_id,
                    "scope_kind": scope_kind,
                    "scope_ref_id": scope_ref_id,
                    "origin_task_asset_id": origin_id,
                    "revisions": [],
                },
            )
            revision_no = int(revision["revision_no"])
            entry["revisions"].append(
                {
                    "revision_no": revision_no,
                    "status": str(revision["status"]),
                    "is_working": (
                        working_no is not None
                        and revision_no == int(working_no)
                    ),
                    "is_finalized": (
                        finalized_no is not None
                        and revision_no == int(finalized_no)
                    ),
                    "manifest_row_hash": str(
                        revision.get("manifest_row_hash") or ""
                    ),
                }
            )
    for entry in expected.values():
        entry["revisions"].sort(key=lambda row: row["revision_no"])
        numbers = [row["revision_no"] for row in entry["revisions"]]
        if len(numbers) != len(set(numbers)):
            raise ValueError("mapping duplicates an alias revision")
    recovery_hashes: dict[int, str] = {}
    for index, recovery in enumerate(mapping.get("asset_recoveries", [])):
        if not isinstance(recovery, dict):
            raise ValueError(
                f"mapping.asset_recoveries[{index}] is malformed"
            )
        missing_id = int(recovery.get("missing_task_asset_id") or 0)
        row_hash = str(recovery.get("manifest_row_hash") or "")
        unsigned = {
            key: value
            for key, value in recovery.items()
            if key != "manifest_row_hash"
        }
        if (
            missing_id <= 0
            or missing_id in recovery_hashes
            or not SHA256.fullmatch(row_hash)
            or canonical_value_hash(unsigned) != row_hash
        ):
            raise ValueError(
                f"mapping.asset_recoveries[{index}] row hash is invalid"
            )
        recovery_hashes[missing_id] = row_hash
    for entry in expected.values():
        entry["recovery_mapping_row_hash"] = recovery_hashes.get(
            entry["origin_task_asset_id"]
        )
    return expected


def parse_approved_bundles(mapping: dict[str, Any]) -> dict[int, dict[str, Any]]:
    result: dict[int, dict[str, Any]] = {}
    for group_index, group in enumerate(mapping.get("resources", [])):
        for revision_index, revision in enumerate(group.get("history", [])):
            bundle = revision.get("source_bundle")
            if bundle is None:
                continue
            if not isinstance(bundle, dict):
                raise ValueError(
                    f"mapping.resources[{group_index}].history"
                    f"[{revision_index}].source_bundle is malformed"
                )
            task_asset_id = bundle.get("task_asset_id")
            members = bundle.get("members")
            if (
                not isinstance(task_asset_id, int)
                or isinstance(task_asset_id, bool)
                or task_asset_id <= 0
                or task_asset_id in result
                or bundle.get("format") != "zip"
                or not SHA256.fullmatch(str(bundle.get("bundle_sha256") or ""))
                or not SHA256.fullmatch(
                    str(bundle.get("manifest_sha256") or "")
                )
                or not isinstance(members, list)
                or not members
                or any(
                    not isinstance(member, dict)
                    or member.get("confirmed") is not True
                    or int(member.get("task_asset_id") or 0) <= 0
                    or not SHA256.fullmatch(str(member.get("sha256") or ""))
                    for member in members
                )
            ):
                raise ValueError("mapping source_bundle task asset IDs are invalid")
            revision_no = int(revision["revision_no"])
            result[task_asset_id] = {
                "task_id": int(group["task_id"]),
                "scope_kind": str(group["scope_kind"]),
                "scope_ref_id": int(group["scope_ref_id"]),
                "revision_no": revision_no,
                "status": str(revision["status"]),
                "is_working": group.get("working_revision_no") == revision_no,
                "is_finalized": group.get("finalized_revision_no") == revision_no,
                "bundle_sha256": str(bundle["bundle_sha256"]),
                "manifest_sha256": str(bundle["manifest_sha256"]),
                "confirmed_by": int(bundle.get("confirmed_by") or 0),
                "confirmed_at": str(bundle.get("confirmed_at") or ""),
                "confirmation_note": str(
                    bundle.get("confirmation_note") or ""
                ),
                "members": [
                    {
                        "task_asset_id": int(member["task_asset_id"]),
                        "sha256": str(member["sha256"]),
                    }
                    for member in members
                ],
            }
    return result


def bind_confirmed_bundle_manifest(
    manifest: dict[str, Any],
    manifest_sha256: str,
    decision_template_sha256: str,
    approved: dict[int, dict[str, Any]],
) -> None:
    bundles = manifest.get("bundles")
    if (
        manifest.get("schema_version") != 1
        or manifest.get("status") != "CONFIRMED"
        or manifest.get("bundle_count") != len(approved)
        or not isinstance(bundles, list)
        or len(bundles) != len(approved)
        or int(manifest.get("confirmed_by") or 0) <= 0
        or not str(manifest.get("confirmed_at") or "")
        or not str(manifest.get("confirmation_note") or "")
        or manifest.get("decision_template_sha256")
        != decision_template_sha256
    ):
        raise ValueError("source bundle confirmation manifest is invalid")
    observed: dict[int, tuple[int, str, int, int]] = {}
    rows_by_bundle_id: dict[int, dict[str, Any]] = {}
    for bundle in bundles:
        if not isinstance(bundle, dict) or bundle.get("confirmed") is not True:
            raise ValueError("source bundle confirmation row is invalid")
        bundle_id = int(bundle.get("bundle_task_asset_id") or 0)
        if bundle_id <= 0 or bundle_id in observed:
            raise ValueError("source bundle confirmation IDs are invalid")
        observed[bundle_id] = (
            int(bundle.get("task_id") or 0),
            str(bundle.get("scope_kind") or ""),
            int(bundle.get("scope_ref_id") or 0),
            int(bundle.get("revision_no") or 0),
        )
        rows_by_bundle_id[bundle_id] = bundle
    expected = {
        bundle_id: (
            row["task_id"],
            row["scope_kind"],
            row["scope_ref_id"],
            row["revision_no"],
        )
        for bundle_id, row in approved.items()
    }
    if observed != expected:
        raise ValueError(
            "source bundle confirmation scopes differ from reviewed mapping"
        )
    for bundle_id, row in approved.items():
        confirmed_row = rows_by_bundle_id[bundle_id]
        ordered_members = confirmed_row.get("ordered_members")
        confirmed_members = (
            [
                {
                    "task_asset_id": int(member.get("task_asset_id") or 0),
                    "sha256": str(member.get("sha256") or ""),
                }
                for member in ordered_members
                if isinstance(member, dict)
                and member.get("confirmed") is True
            ]
            if isinstance(ordered_members, list)
            else []
        )
        if (
            row["confirmed_by"] != int(manifest["confirmed_by"])
            or row["confirmed_at"] != str(manifest["confirmed_at"])
            or row["confirmation_note"] != str(manifest["confirmation_note"])
            or int(confirmed_row.get("bundle_asset_id") or 0) <= 0
            or not str(confirmed_row.get("bundle_storage_ref_id") or "")
            or not isinstance(ordered_members, list)
            or any(
                not isinstance(member, dict)
                or member.get("confirmed") is not True
                for member in ordered_members
            )
            or confirmed_members != row["members"]
        ):
            raise ValueError(
                "source bundle confirmation metadata differs from mapping"
            )
        row["materialization_manifest_sha256"] = manifest_sha256
        row["bundle_asset_id"] = int(confirmed_row["bundle_asset_id"])
        row["bundle_storage_ref_id"] = str(
            confirmed_row["bundle_storage_ref_id"]
        )


def validate_verdict(
    verdict: dict[str, Any],
    *,
    mapping_sha256: str,
    manifest_sha256: str,
    exception_attestation_sha256: str = ZERO_SHA256,
) -> dict[str, dict[str, Any]]:
    if (
        set(verdict) != OBJECT_VERDICT_FIELDS
        or verdict.get("schema_version") != 1
        or verdict.get("status") != "PASS"
        or verdict.get("violation_count") != 0
        or verdict.get("violations") != []
        or verdict.get("mapping_sha256") != mapping_sha256
        or verdict.get("manifest_sha256") != manifest_sha256
    ):
        raise ValueError(
            "object verdict does not bind the exact mapping/manifest PASS"
        )
    unsigned = {
        key: value for key, value in verdict.items() if key != "evidence_hash"
    }
    if verdict.get("evidence_hash") != canonical_value_hash(unsigned):
        raise ValueError("object verdict self-hash differs")
    exception_values = verdict.get("exceptions")
    if not isinstance(exception_values, list):
        raise ValueError("object verdict exceptions must be an array")
    exceptions: dict[str, dict[str, Any]] = {}
    for value in exception_values:
        if not isinstance(value, dict) or set(value) != OBJECT_VERDICT_EXCEPTION_FIELDS:
            raise ValueError("object verdict exception is malformed")
        entity = value.get("entity_key")
        if (
            not isinstance(entity, str)
            or entity in exceptions
            or value.get("expected_http_status") != 410
            or value.get("observed_http_status") != 410
            or value.get("working_reference_count") != 0
            or value.get("finalized_reference_count") != 0
        ):
            raise ValueError("object verdict exception contract failed")
        if (
            not SHA256.fullmatch(str(value.get("object_row_sha256") or ""))
            or not SHA256.fullmatch(str(value.get("mapping_row_hash") or ""))
        ):
            raise ValueError("object verdict exception hashes are invalid")
        exceptions[entity] = value
    if verdict.get("exception_count") != len(exceptions):
        raise ValueError("object verdict exception_count is inconsistent")
    exception_evidence_sha = str(
        verdict.get("exception_evidence_sha256") or ""
    )
    mapping_row_hash = str(verdict.get("mapping_row_hash") or "")
    if not exceptions:
        if (
            exception_evidence_sha != ZERO_SHA256
            or mapping_row_hash != ZERO_SHA256
            or exception_attestation_sha256 != ZERO_SHA256
        ):
            raise ValueError(
                "object verdict without exceptions must use zero exception hashes"
            )
    elif (
        len(exceptions) != 1
        or not SHA256.fullmatch(exception_evidence_sha)
        or exception_evidence_sha == ZERO_SHA256
        or exception_evidence_sha != exception_attestation_sha256
        or not SHA256.fullmatch(mapping_row_hash)
        or mapping_row_hash == ZERO_SHA256
        or any(
            value["mapping_row_hash"] != mapping_row_hash
            for value in exceptions.values()
        )
    ):
        raise ValueError("object verdict exception binding is inconsistent")
    return exceptions


def index_manifest(
    rows: list[dict[str, Any]],
    exceptions: dict[str, dict[str, Any]],
) -> dict[str, dict[str, Any]]:
    result: dict[str, dict[str, Any]] = {}
    for line_no, row in enumerate(rows, 1):
        entity = row.get("entity_key")
        if not isinstance(entity, str) or entity in result:
            raise ValueError("object manifest entity keys are invalid")
        if entity in exceptions:
            try:
                historical_unavailable_exception.validate_exception_object_row(
                    row
                )
            except ValueError as exc:
                raise ValueError(
                    f"object manifest exception row {entity} failed its contract"
                ) from exc
            if (
                canonical_value_hash(row)
                != exceptions[entity]["object_row_sha256"]
            ):
                raise ValueError(
                    f"object manifest exception row {entity} hash differs"
                )
        else:
            problems = object_manifest_verifier.validate_contract(row, line_no)
            if problems:
                raise ValueError(
                    f"object manifest row {entity} failed its contract"
                )
        result[entity] = row
    if not set(exceptions).issubset(result):
        raise ValueError("object verdict exception is absent from manifest")
    return result


def canonical_row_hash(value: dict[str, Any]) -> str:
    return sha256_bytes(canonical_bytes(value))


def observed_key(
    record: dict[str, Any],
) -> tuple[tuple[int, str, int, int] | None, list[dict[str, str]]]:
    problems: list[dict[str, str]] = []
    alias = record.get("alias")
    group = record.get("resource_group")
    if not isinstance(alias, dict):
        return None, [
            violation("alias_coverage.alias_malformed", "*", "alias is absent")
        ]
    entity = f"task_asset:{alias.get('id')}"
    if not isinstance(group, dict):
        return None, [
            violation(
                "alias_coverage.group_missing",
                entity,
                "bound resource group is absent",
            )
        ]
    remark = alias.get("remark")
    match = ALIAS_REMARK.fullmatch(str(remark or ""))
    if not match:
        return None, [
            violation(
                "alias_coverage.remark_invalid",
                entity,
                "source alias remark is not canonical",
            )
        ]
    group_id = int(match.group("group_id"))
    origin_id = int(match.group("origin_id"))
    if group_id != int(group.get("id") or 0):
        problems.append(
            violation(
                "alias_coverage.remark_group_mismatch",
                entity,
                "remark group differs from bound group",
            )
        )
    return (
        (
            int(group.get("task_id") or 0),
            str(group.get("scope_kind") or ""),
            int(group.get("scope_ref_id") or 0),
            origin_id,
        ),
        problems,
    )

def approved_bundle_problems(
    expected: dict[str, Any],
    record: dict[str, Any],
    manifest_row: dict[str, Any] | None,
) -> list[dict[str, str]]:
    alias = record.get("alias")
    group = record.get("resource_group")
    storage = record.get("storage_ref")
    alias_id = int(alias.get("id") or 0) if isinstance(alias, dict) else 0
    entity = f"task_asset:{alias_id}"
    if not all(
        isinstance(value, dict)
        for value in (alias, group, storage, manifest_row)
    ):
        return [
            violation(
                "alias_coverage.bundle_shape_drift",
                entity,
                "approved bundle asset/group/storage row is incomplete",
            )
        ]
    storage_key = str(alias.get("storage_key") or "")
    remark = str(alias.get("remark") or "")
    expected_remark = (
        f"v8-migration-source-bundle:{storage_key}:"
        f"{expected['materialization_manifest_sha256']}"
    )
    shape_ok = (
        int(alias.get("task_id") or 0) == expected["task_id"]
        and int(alias.get("asset_id") or 0) == expected["bundle_asset_id"]
        and alias.get("asset_type") == "source"
        and alias.get("binding_state") == "bound"
        and int(alias.get("bound_group_id") or 0) == int(group.get("id") or 0)
        and alias.get("bound_role") == "source"
        and alias.get("source_module_key") == "migration"
        and alias.get("source_task_module_id") is None
        and alias.get("upload_status") == "uploaded"
        and int(alias.get("is_archived") or 0) == 0
        and alias.get("flow_review_status") == "not_applicable"
        and alias.get("mime_type") == "application/zip"
        and alias.get("whole_hash") == expected["bundle_sha256"]
        and alias.get("storage_ref_id")
        == expected["bundle_storage_ref_id"]
        and bool(storage_key)
        and remark == expected_remark
        and all(
            alias.get(field) is None
            for field in (
                "deleted_at",
                "cleaned_at",
                "access_revoked_at",
                "object_deleted_at",
            )
        )
        and int(group.get("task_id") or 0) == expected["task_id"]
        and group.get("scope_kind") == expected["scope_kind"]
        and int(group.get("scope_ref_id") or 0) == expected["scope_ref_id"]
        and storage.get("ref_id") == alias.get("storage_ref_id")
        and int(storage.get("asset_id") or 0) == alias_id
        and storage.get("owner_type") == "task_asset"
        and int(storage.get("owner_id") or 0) == alias_id
        and storage.get("storage_adapter") == "oss_upload_service"
        and storage.get("ref_type") == "task_asset_object"
        and storage.get("ref_key") == storage_key
        and storage.get("mime_type") == "application/zip"
        and storage.get("file_size") == alias.get("file_size")
        and storage.get("checksum_hint") == expected["bundle_sha256"]
        and storage.get("status") == "recorded"
        and int(storage.get("is_placeholder") or 0) == 0
        and manifest_row.get("entity_key") == entity
        and manifest_row.get("owner_kind") == "task_asset"
        and int(manifest_row.get("owner_id") or 0) == alias_id
        and int(manifest_row.get("task_id") or 0) == expected["task_id"]
        and manifest_row.get("storage_ref_id")
        == expected["bundle_storage_ref_id"]
        and manifest_row.get("object_key") == storage_key
        and manifest_row.get("size") == alias.get("file_size")
        and manifest_row.get("mime_type") == "application/zip"
        and manifest_row.get("sha256") == expected["bundle_sha256"]
        and manifest_row.get("status") == "recorded"
        and manifest_row.get("is_placeholder") is False
    )
    if shape_ok:
        return []
    return [
        violation(
            "alias_coverage.bundle_shape_drift",
            entity,
            "approved bundle asset/group/storage row differs from mapping",
        )
    ]


def verify_coverage(
    *,
    run_id: str,
    database: str,
    mapping_sha256: str,
    manifest_sha256: str,
    verdict_sha256: str,
    exception_attestation_sha256: str = ZERO_SHA256,
    source_bundle_confirmed_manifest_sha256: str = ZERO_SHA256,
    source_bundle_decision_template_sha256: str = ZERO_SHA256,
    expected: dict[tuple[int, str, int, int], dict[str, Any]],
    manifest: dict[str, dict[str, Any]],
    exceptions: dict[str, dict[str, Any]],
    observed_aliases: list[dict[str, Any]],
    observed_usages: list[dict[str, Any]],
    approved_bundles: dict[int, dict[str, Any]] | None = None,
) -> dict[str, Any]:
    approved_bundles = approved_bundles or {}
    problems: list[dict[str, str]] = []
    observed: dict[tuple[int, str, int, int], dict[str, Any]] = {}
    alias_id_to_key: dict[int, tuple[int, str, int, int]] = {}
    observed_bundle_ids: set[int] = set()
    observed_bundle_records: dict[int, dict[str, Any]] = {}
    failed_bundle_ids: set[int] = set()
    observed_alias_candidate_count = 0
    for record in observed_aliases:
        alias = record.get("alias")
        if not isinstance(alias, dict):
            key, row_problems = observed_key(record)
            problems.extend(row_problems)
            continue
        alias_id = int(alias.get("id") or 0)
        if alias_id in approved_bundles:
            if alias_id in observed_bundle_ids:
                problems.append(
                    violation(
                        "alias_coverage.bundle_id_duplicate",
                        f"task_asset:{alias_id}",
                        "approved bundle asset row is duplicated",
                    )
                )
            observed_bundle_ids.add(alias_id)
            observed_bundle_records.setdefault(alias_id, record)
            row_problems = approved_bundle_problems(
                approved_bundles[alias_id],
                record,
                manifest.get(f"task_asset:{alias_id}"),
            )
            if row_problems:
                failed_bundle_ids.add(alias_id)
            problems.extend(row_problems)
            continue
        observed_alias_candidate_count += 1
        key, row_problems = observed_key(record)
        problems.extend(row_problems)
        if alias_id <= 0 or alias_id in alias_id_to_key:
            problems.append(
                violation(
                    "alias_coverage.alias_id_duplicate",
                    f"task_asset:{alias_id}",
                    "alias ID is invalid or duplicated",
                )
            )
            continue
        if key is None:
            continue
        alias_id_to_key[alias_id] = key
        if key in observed:
            problems.append(
                violation(
                    "alias_coverage.natural_key_duplicate",
                    f"task_asset:{alias_id}",
                    "multiple aliases use one mapping natural key",
                )
            )
            continue
        observed[key] = record

    usage_by_alias: dict[int, list[dict[str, Any]]] = defaultdict(list)
    observed_alias_usage_count = 0
    for usage in observed_usages:
        try:
            alias_id = int(usage["alias_id"])
        except (KeyError, TypeError, ValueError):
            problems.append(
                violation(
                    "alias_coverage.usage_malformed",
                    "*",
                    "revision usage row has no valid alias ID",
                )
            )
            continue
        if alias_id in approved_bundles:
            continue
        observed_alias_usage_count += 1
        usage_by_alias[alias_id].append(usage)
    for values in usage_by_alias.values():
        values.sort(key=lambda row: (int(row["revision_no"]), int(row["revision_id"])))
    for alias_id in sorted(set(approved_bundles) - observed_bundle_ids):
        failed_bundle_ids.add(alias_id)
        problems.append(
            violation(
                "alias_coverage.approved_bundle_missing",
                f"task_asset:{alias_id}",
                "approved bundle asset is absent from Clone B",
            )
        )
    for alias_id, wanted in approved_bundles.items():
        bundle_usages = sorted([
            row
            for row in observed_usages
            if int(row.get("alias_id") or 0) == alias_id
        ], key=lambda row: (int(row.get("revision_no") or 0), int(row.get("revision_id") or 0)))
        bundle_record = observed_bundle_records.get(alias_id, {})
        bundle_group = bundle_record.get("resource_group")
        expected_group_id = (
            int(bundle_group.get("id") or 0)
            if isinstance(bundle_group, dict)
            else 0
        )
        expected_projection = [{
            "group_id": expected_group_id,
            "revision_no": wanted["revision_no"],
            "status": wanted["status"],
            "is_working": wanted["is_working"],
            "is_finalized": wanted["is_finalized"],
        }]
        actual_projection = [
            {
                "group_id": int(row.get("group_id") or 0),
                "revision_no": int(row.get("revision_no") or 0),
                "status": str(row.get("status") or ""),
                "is_working": bool(row.get("is_working")),
                "is_finalized": bool(row.get("is_finalized")),
            }
            for row in bundle_usages
        ]
        if actual_projection != expected_projection:
            failed_bundle_ids.add(alias_id)
            problems.append(
                violation(
                    "alias_coverage.bundle_usage_drift",
                    f"task_asset:{alias_id}",
                    "approved bundle revision usage differs from mapping",
                )
            )

    missing = sorted(set(expected) - set(observed))
    extra = sorted(set(observed) - set(expected))
    for key in missing:
        problems.append(
            violation(
                "alias_coverage.expected_alias_missing",
                ":".join(map(str, key)),
                "reviewed mapping alias is absent from Clone B",
            )
        )
    for key in extra:
        alias_id = int(observed[key]["alias"]["id"])
        problems.append(
            violation(
                "alias_coverage.unexpected_alias",
                f"task_asset:{alias_id}",
                "Clone B alias is absent from reviewed mapping",
            )
        )

    entries: list[dict[str, Any]] = []
    normal_count = 0
    exception_count = 0
    consumed_exceptions: set[str] = set()
    for key in sorted(set(expected) & set(observed)):
        wanted = expected[key]
        record = observed[key]
        alias = record["alias"]
        predecessor = record.get("predecessor")
        group = record.get("resource_group")
        storage = record.get("storage_ref")
        alias_id = int(alias["id"])
        entity = f"task_asset:{alias_id}"
        origin_id = int(wanted["origin_task_asset_id"])
        origin_entity = f"task_asset:{origin_id}"

        if not isinstance(predecessor, dict) or int(
            predecessor.get("id") or 0
        ) != origin_id:
            problems.append(
                violation(
                    "alias_coverage.predecessor_missing",
                    entity,
                    f"predecessor {origin_id} is absent",
                )
            )
            continue
        if not isinstance(group, dict) or not isinstance(storage, dict):
            problems.append(
                violation(
                    "alias_coverage.storage_or_group_missing",
                    entity,
                    "resource group or storage reference is absent",
                )
            )
            continue

        expected_remark = (
            f"v8-source-alias:group={group['id']}:origin={origin_id}"
        )
        alias_shape = (
            int(alias.get("task_id") or 0) == wanted["task_id"]
            and alias.get("asset_type") == "source"
            and alias.get("binding_state") == "bound"
            and int(alias.get("bound_group_id") or 0) == int(group["id"])
            and alias.get("bound_role") == "source"
            and alias.get("remark") == expected_remark
            and alias.get("source_module_key") == "migration"
            and alias.get("source_task_module_id") is None
            and int(alias.get("is_archived") or 0) == 0
            and alias.get("flow_review_status") == "not_applicable"
            and all(
                alias.get(field) is None
                for field in (
                    "deleted_at",
                    "cleaned_at",
                    "access_revoked_at",
                    "object_deleted_at",
                )
            )
        )
        if not alias_shape:
            problems.append(
                violation(
                    "alias_coverage.alias_shape_drift",
                    entity,
                    "alias lifecycle/binding fields differ from contract",
                )
            )

        drifted = [
            field
            for field in COPIED_FIELDS
            if alias.get(field) != predecessor.get(field)
        ]
        if drifted:
            problems.append(
                violation(
                    "alias_coverage.predecessor_copy_drift",
                    entity,
                    "copied fields drifted: " + ",".join(drifted),
                )
            )
        if (
            predecessor.get("asset_type") not in ALLOWED_PREDECESSOR_TYPES
            or int(predecessor.get("task_id") or 0) != wanted["task_id"]
            or predecessor.get("binding_state") != "bound"
            or int(predecessor.get("bound_group_id") or 0)
            != int(group["id"])
            or predecessor.get("bound_role") != "final"
        ):
            problems.append(
                violation(
                    "alias_coverage.predecessor_shape_drift",
                    entity,
                    "predecessor is not the bound final in this group",
                )
            )

        storage_ref = str(alias.get("storage_ref_id") or "")
        manifest_row = manifest.get(origin_entity)
        if manifest_row is None:
            problems.append(
                violation(
                    "alias_coverage.predecessor_manifest_missing",
                    entity,
                    f"{origin_entity} is absent from object manifest",
                )
            )
            continue
        exception = exceptions.get(origin_entity)
        expected_object_key = (
            predecessor.get("storage_key") or storage.get("ref_key")
        )
        storage_matches = (
            storage_ref
            and predecessor.get("storage_ref_id") == storage_ref
            and storage.get("ref_id") == storage_ref
            and storage.get("owner_type") == "task_asset"
            and int(storage.get("owner_id") or 0) == origin_id
            and manifest_row.get("owner_kind") == "task_asset"
            and int(manifest_row.get("owner_id") or 0) == origin_id
            and int(manifest_row.get("task_id") or 0) == wanted["task_id"]
            and manifest_row.get("storage_ref_id") == storage_ref
            and manifest_row.get("storage_adapter")
            == storage.get("storage_adapter")
            and manifest_row.get("object_key") == expected_object_key
            and (
                exception is not None
                or (
                    bool(manifest_row.get("is_placeholder"))
                    == bool(storage.get("is_placeholder"))
                    and manifest_row.get("status") == storage.get("status")
                )
            )
        )
        if not storage_matches:
            problems.append(
                violation(
                    "alias_coverage.object_identity_drift",
                    entity,
                    "alias/predecessor/storage/manifest identity differs",
                )
            )

        actual_usage = usage_by_alias.get(alias_id, [])
        actual_usage_projection = [
            {
                "revision_no": int(row["revision_no"]),
                "status": str(row["status"]),
                "is_working": bool(row["is_working"]),
                "is_finalized": bool(row["is_finalized"]),
            }
            for row in actual_usage
        ]
        expected_usage_projection = [
            {
                "revision_no": int(row["revision_no"]),
                "status": str(row["status"]),
                "is_working": bool(row["is_working"]),
                "is_finalized": bool(row["is_finalized"]),
            }
            for row in wanted["revisions"]
        ]
        if actual_usage_projection != expected_usage_projection or any(
            int(row.get("group_id") or 0) != int(group["id"])
            for row in actual_usage
        ):
            problems.append(
                violation(
                    "alias_coverage.revision_usage_drift",
                    entity,
                    "observed source revisions differ from reviewed mapping",
                )
            )

        if exception is None:
            if not SHA256.fullmatch(str(manifest_row.get("sha256") or "")):
                problems.append(
                    violation(
                        "alias_coverage.predecessor_not_byte_verified",
                        entity,
                        "predecessor manifest row has no verified SHA-256",
                    )
                )
            coverage_mode = "verified_predecessor_object"
            normal_count += 1
        else:
            consumed_exceptions.add(origin_entity)
            exception_shape = (
                exception.get("missing_task_asset_id") == origin_id
                and exception.get("task_id") == wanted["task_id"]
                and exception.get("object_row_sha256")
                == canonical_value_hash(manifest_row)
                and exception.get("mapping_row_hash")
                == wanted.get("recovery_mapping_row_hash")
            )
            if not exception_shape:
                problems.append(
                    violation(
                        "alias_coverage.unavailable_exception_drift",
                        entity,
                        "historical exception does not bind this mapping/object row",
                    )
                )
            if any(
                row["status"] not in ALLOWED_EXCEPTION_REVISION_STATES
                or row["is_working"]
                or row["is_finalized"]
                for row in actual_usage_projection
            ):
                problems.append(
                    violation(
                        "alias_coverage.unavailable_alias_is_current",
                        entity,
                        "historically unavailable predecessor is current",
                    )
                )
            coverage_mode = "historical_unavailable_exception"
            exception_count += 1

        entries.append(
            {
                "task_id": wanted["task_id"],
                "scope_kind": wanted["scope_kind"],
                "scope_ref_id": wanted["scope_ref_id"],
                "group_id": int(group["id"]),
                "alias_task_asset_id": alias_id,
                "predecessor_task_asset_id": origin_id,
                "storage_ref_id": storage_ref,
                "object_key": manifest_row.get("object_key"),
                "object_size": manifest_row.get("size"),
                "object_mime_type": manifest_row.get("mime_type"),
                "object_sha256": manifest_row.get("sha256"),
                "coverage_mode": coverage_mode,
                "alias_row_sha256": canonical_row_hash(alias),
                "predecessor_row_sha256": canonical_row_hash(predecessor),
                "storage_row_sha256": canonical_row_hash(storage),
                "manifest_entity_key": origin_entity,
                "manifest_row_sha256": canonical_row_hash(manifest_row),
                "revisions": [
                    {
                        **row,
                        "revision_id": next(
                            (
                                int(actual["revision_id"])
                                for actual in actual_usage
                                if int(actual["revision_no"])
                                == int(row["revision_no"])
                            ),
                            None,
                        ),
                    }
                    for row in wanted["revisions"]
                ],
            }
        )

    for entity in sorted(set(exceptions) - consumed_exceptions):
        problems.append(
            violation(
                "alias_coverage.unused_historical_exception",
                entity,
                "object verdict exception is not consumed by an expected alias",
            )
        )

    unknown_usage_ids = sorted(set(usage_by_alias) - set(alias_id_to_key))
    for alias_id in unknown_usage_ids:
        problems.append(
            violation(
                "alias_coverage.usage_without_alias",
                f"task_asset:{alias_id}",
                "revision usage refers to an unobserved alias",
            )
        )

    entries.sort(
        key=lambda row: (
            row["task_id"],
            row["scope_kind"],
            row["scope_ref_id"],
            row["predecessor_task_asset_id"],
        )
    )
    problems.sort(
        key=lambda row: (
            row["violation_code"],
            row["entity_key"],
            row["detail"],
        )
    )
    bundle_entries: list[dict[str, Any]] = []
    for alias_id in sorted(approved_bundles):
        wanted = approved_bundles[alias_id]
        record = observed_bundle_records.get(alias_id, {})
        alias = record.get("alias")
        group = record.get("resource_group")
        storage = record.get("storage_ref")
        bundle_usages = sorted(
            [
                row
                for row in observed_usages
                if int(row.get("alias_id") or 0) == alias_id
            ],
            key=lambda row: (
                int(row.get("revision_no") or 0),
                int(row.get("revision_id") or 0),
            ),
        )
        bundle_entries.append(
            {
                "task_asset_id": alias_id,
                "bundle_asset_id": wanted["bundle_asset_id"],
                "bundle_storage_ref_id": wanted["bundle_storage_ref_id"],
                "task_id": wanted["task_id"],
                "scope_kind": wanted["scope_kind"],
                "scope_ref_id": wanted["scope_ref_id"],
                "revision_no": wanted["revision_no"],
                "bundle_sha256": wanted["bundle_sha256"],
                "zip_manifest_sha256": wanted["manifest_sha256"],
                "members_sha256": sha256_bytes(
                    canonical_bytes(wanted["members"])
                ),
                "alias_row_sha256": (
                    canonical_row_hash(alias)
                    if isinstance(alias, dict)
                    else ZERO_SHA256
                ),
                "group_row_sha256": (
                    canonical_row_hash(group)
                    if isinstance(group, dict)
                    else ZERO_SHA256
                ),
                "storage_row_sha256": (
                    canonical_row_hash(storage)
                    if isinstance(storage, dict)
                    else ZERO_SHA256
                ),
                "object_manifest_row_sha256": (
                    canonical_row_hash(manifest[f"task_asset:{alias_id}"])
                    if f"task_asset:{alias_id}" in manifest
                    else ZERO_SHA256
                ),
                "revision_usage_sha256": sha256_bytes(
                    canonical_bytes(bundle_usages)
                ),
            }
        )
    verified_bundle_ids = sorted(
        set(approved_bundles) - failed_bundle_ids
    )
    body: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "status": "PASS" if not problems else "FAIL",
        "operation": "verify_delivery_source_alias_object_coverage",
        "run_id": run_id,
        "database": database,
        "mapping_sha256": mapping_sha256,
        "object_manifest_sha256": manifest_sha256,
        "object_verdict_sha256": verdict_sha256,
        "historical_exception_attestation_sha256":
            exception_attestation_sha256,
        "source_bundle_confirmed_manifest_sha256":
            source_bundle_confirmed_manifest_sha256,
        "source_bundle_decision_template_sha256":
            source_bundle_decision_template_sha256,
        "approved_bundle_count": len(approved_bundles),
        "verified_bundle_count": len(verified_bundle_ids),
        "verified_bundle_ids": verified_bundle_ids,
        "bundle_entry_count": len(bundle_entries),
        "bundle_entries_sha256": sha256_bytes(
            canonical_bytes(bundle_entries)
        ),
        "bundle_entries": bundle_entries,
        "expected_alias_count": len(expected),
        "expected_alias_revision_count": sum(
            len(value["revisions"]) for value in expected.values()
        ),
        "observed_alias_count": observed_alias_candidate_count,
        "observed_alias_revision_count": observed_alias_usage_count,
        "verified_predecessor_count": normal_count,
        "historical_unavailable_exception_count": exception_count,
        "entry_count": len(entries),
        "entries_sha256": sha256_bytes(canonical_bytes(entries)),
        "entries": entries,
        "violation_count": len(problems),
        "violations": problems,
        "database_write_performed": False,
        "production_write_performed": False,
    }
    body["evidence_sha256"] = sha256_bytes(canonical_bytes(body))
    return body


def atomic_write(path: pathlib.Path, data: bytes) -> None:
    if path.exists() or path.is_symlink():
        raise FileExistsError(f"refusing to overwrite output: {path}")
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(f".{path.name}.tmp")
    if temporary.exists() or temporary.is_symlink():
        raise FileExistsError(f"temporary output already exists: {temporary}")
    temporary.write_bytes(data)
    temporary.replace(path)


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--database", required=True)
    parser.add_argument("--mapping", type=pathlib.Path, required=True)
    parser.add_argument("--expected-mapping-sha256", required=True)
    parser.add_argument("--object-manifest", type=pathlib.Path, required=True)
    parser.add_argument("--expected-object-manifest-sha256", required=True)
    parser.add_argument("--object-verdict", type=pathlib.Path, required=True)
    parser.add_argument("--expected-object-verdict-sha256", required=True)
    parser.add_argument(
        "--historical-exception-attestation",
        type=pathlib.Path,
        required=True,
    )
    parser.add_argument(
        "--expected-historical-exception-attestation-sha256",
        required=True,
    )
    parser.add_argument(
        "--source-bundle-confirmed-manifest",
        type=pathlib.Path,
        required=True,
    )
    parser.add_argument(
        "--expected-source-bundle-confirmed-manifest-sha256",
        required=True,
    )
    parser.add_argument(
        "--source-bundle-decision-template",
        type=pathlib.Path,
        required=True,
    )
    parser.add_argument(
        "--expected-source-bundle-decision-template-sha256",
        required=True,
    )
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--mysql", default="mysql")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    if not RUN_ID.fullmatch(args.run_id):
        raise ValueError("--run-id is invalid")
    mapping_sha = require_expected_hash(
        args.mapping, args.expected_mapping_sha256, "mapping"
    )
    manifest_sha = require_expected_hash(
        args.object_manifest,
        args.expected_object_manifest_sha256,
        "object manifest",
    )
    verdict_sha = require_expected_hash(
        args.object_verdict,
        args.expected_object_verdict_sha256,
        "object verdict",
    )
    attestation_sha = require_expected_hash(
        args.historical_exception_attestation,
        args.expected_historical_exception_attestation_sha256,
        "historical exception attestation",
    )
    bundle_confirmation_sha = require_expected_hash(
        args.source_bundle_confirmed_manifest,
        args.expected_source_bundle_confirmed_manifest_sha256,
        "source bundle confirmed manifest",
    )
    bundle_decision_template_sha = require_expected_hash(
        args.source_bundle_decision_template,
        args.expected_source_bundle_decision_template_sha256,
        "source bundle decision template",
    )
    input_paths = {
        args.mapping.resolve(),
        args.object_manifest.resolve(),
        args.object_verdict.resolve(),
        args.historical_exception_attestation.resolve(),
        args.source_bundle_confirmed_manifest.resolve(),
        args.source_bundle_decision_template.resolve(),
    }
    if args.output.resolve() in input_paths:
        raise ValueError("output must not overwrite an input")

    mapping = load_object(args.mapping, "mapping")
    expected = parse_mapping(mapping)
    approved_bundles = parse_approved_bundles(mapping)
    bind_confirmed_bundle_manifest(
        load_object(
            args.source_bundle_confirmed_manifest,
            "source bundle confirmed manifest",
        ),
        bundle_confirmation_sha,
        bundle_decision_template_sha,
        approved_bundles,
    )
    verdict = load_object(args.object_verdict, "object verdict")
    exceptions = validate_verdict(
        verdict,
        mapping_sha256=mapping_sha,
        manifest_sha256=manifest_sha,
        exception_attestation_sha256=attestation_sha,
    )
    attestation, attested_exception, loaded_attestation_sha = (
        historical_unavailable_exception.load_attestation(
            args.historical_exception_attestation,
            manifest_path=args.object_manifest,
        )
    )
    if (
        loaded_attestation_sha != attestation_sha
        or attestation.get("mapping_sha256") != mapping_sha
        or attestation.get("object_manifest_sha256") != manifest_sha
        or set(exceptions) != {attested_exception["entity_key"]}
    ):
        raise ValueError(
            "historical exception attestation does not bind mapping/manifest/verdict"
        )
    accepted_exception = exceptions[attested_exception["entity_key"]]
    for field in (
        "task_id",
        "missing_task_asset_id",
        "mapping_row_hash",
        "object_row_sha256",
        "working_reference_count",
        "finalized_reference_count",
    ):
        if accepted_exception.get(field) != attested_exception.get(field):
            raise ValueError(
                "historical exception attestation differs from object verdict"
            )
    manifest = index_manifest(
        load_jsonl(args.object_manifest, "object manifest"),
        exceptions,
    )
    if verdict.get("checked_count") != len(manifest):
        raise ValueError(
            "object verdict checked_count differs from object manifest rows"
        )
    connection = g4_clone_db.Connection.confirmed_clone_b(
        args.database, mysql=args.mysql
    )
    observed_aliases = parse_json_rows(
        connection.execute(ALIAS_QUERY), "alias query"
    )
    observed_usages = parse_json_rows(
        connection.execute(USAGE_QUERY), "alias usage query"
    )
    result = verify_coverage(
        run_id=args.run_id,
        database=connection.database,
        mapping_sha256=mapping_sha,
        manifest_sha256=manifest_sha,
        verdict_sha256=verdict_sha,
        expected=expected,
        manifest=manifest,
        exceptions=exceptions,
        observed_aliases=observed_aliases,
        observed_usages=observed_usages,
        approved_bundles=approved_bundles,
        exception_attestation_sha256=attestation_sha,
        source_bundle_confirmed_manifest_sha256=bundle_confirmation_sha,
        source_bundle_decision_template_sha256=bundle_decision_template_sha,
    )
    atomic_write(args.output, canonical_bytes(result))
    return 0 if result["status"] == "PASS" else 1


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (FileExistsError, RuntimeError, ValueError) as exc:
        print(str(exc), file=sys.stderr)
        raise SystemExit(2) from exc
