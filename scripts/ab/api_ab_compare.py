#!/usr/bin/env python3
"""Deterministic, GET-only, four-combination API acceptance comparator.

The comparator never emits resolved authentication headers or response bodies.
It records status/body hashes and requires every accepted difference to be
bound to an exact route, direction, status pair, reason hash, and rule hash.
"""
from __future__ import annotations

import argparse
import dataclasses
import datetime as dt
import hashlib
import json
import os
import pathlib
import re
import threading
import urllib.error
import urllib.parse
import urllib.request
from collections.abc import Callable, Iterable
from concurrent.futures import Future, ThreadPoolExecutor
from typing import Any

COMBINATION_IDS = (
    "external_external_a",
    "dev_dev_b",
    "external_dev_b",
    "dev_external_a",
)
CORE_ROUTES = (
    "/v1/tasks/{task_id}",
    "/v1/tasks/{task_id}/detail",
    "/v1/tasks/{task_id}/events",
    "/v1/tasks/{task_id}/resource-bundle",
    "/v1/tasks/{task_id}/assets",
)
GROUP_ROUTES = (
    "/v1/resource-groups/{group_id}",
    "/v1/resource-groups/{group_id}/revisions",
)
ASSET_ROUTES = (
    "/v1/task-assets/{task_asset_id}/preview",
    "/v1/task-assets/{task_asset_id}/download",
)
ASSET_ROUTE_ALLOWED_STATUSES = {
    "/v1/task-assets/{task_asset_id}/preview": frozenset({200, 403, 404, 409, 410}),
    "/v1/task-assets/{task_asset_id}/download": frozenset({200, 403, 404, 410}),
}
ALL_ROUTE_TEMPLATES = frozenset((*CORE_ROUTES, *GROUP_ROUTES, *ASSET_ROUTES))
MAX_BODY_BYTES = 4 * 1024 * 1024
ID_RE = re.compile(r"[1-9][0-9]*")
SHA256_RE = re.compile(r"[0-9a-f]{64}")
REQUEST_CACHE_POLICY_VERSION = "exact_base_url_identity_path_v1"
CURRENT_VERSION_ROLES = frozenset(
    {"current_version", "approved_version", "current_approved_version"}
)
REQUIRED_IDENTITY_ROLES = frozenset({"admin", "view_only", "no_view"})
ORACLE_V2_INPUT_FIELDS = frozenset(
    {
        "reviewed_mapping_sha256",
        "reviewed_manifest_sha256",
        "snapshot_verdict_sha256",
        "clone_a_attestation_sha256",
        "a_snapshot_manifest_sha256",
        "a_snapshot_evidence_sha256",
        "source_snapshot_sha256",
        "clone_a_database",
        "bundle_receipts_sha256",
        "recovery_receipts_sha256",
    }
)
ORACLE_V3_ALIAS_INPUT_FIELDS = frozenset(
    {
        "source_alias_allocation_receipt_sha256",
        "source_alias_apply_receipt_sha256",
        "source_alias_workflow_snapshot_sha256",
        "source_alias_workflow_snapshot_integrity_sha256",
        "source_alias_mapping_canonical_sha256",
    }
)


def canonical(value: object) -> bytes:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")


def sha256(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def file_sha256(path: pathlib.Path) -> str:
    return sha256(path.read_bytes())


def component_sha256(components: list[str]) -> str:
    return sha256("\x1f".join(components).encode("utf-8"))


def load_object(path: pathlib.Path, label: str) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{label} must be a JSON object")
    return value


def load_tasks(path: pathlib.Path) -> list[str]:
    ids: list[str] = []
    for line_no, raw_line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        line = raw_line.strip()
        if not line:
            continue
        try:
            value = json.loads(line)
            value = value.get("task_id") if isinstance(value, dict) else value
        except json.JSONDecodeError:
            value = line.split(",", 1)[0]
        task_id = str(value)
        if not ID_RE.fullmatch(task_id):
            raise ValueError(f"invalid task id on line {line_no}")
        ids.append(task_id)
    if not ids or len(ids) != len(set(ids)):
        raise ValueError("task list must be non-empty and unique")
    return sorted(ids, key=int)


def manifest_task_ids(path: pathlib.Path, run_id: str) -> set[str]:
    found: set[str] = set()
    for line_no, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if not line.strip():
            continue
        row = json.loads(line)
        if not isinstance(row, dict):
            raise ValueError(f"manifest line {line_no} must be an object")
        entity_key = str(row.get("entity_key", ""))
        if (
            row.get("run_id") == run_id
            and row.get("gate_name") == "G01"
            and entity_key.startswith("task:")
        ):
            task_id = entity_key.split(":", 1)[1]
            if not ID_RE.fullmatch(task_id):
                raise ValueError(f"invalid G01 task entity on manifest line {line_no}")
            found.add(task_id)
    if not found:
        raise ValueError("reviewed manifest has no G01 task entities")
    return found


def load_manifest_expectations(
    path: pathlib.Path,
    run_id: str,
    *,
    expected_mapping_sha256: str,
    expected_baseline_attestation_sha256: str,
) -> dict[str, dict[str, Any]]:
    """Load the reviewed G01-G05 oracle used to judge the migrated B API.

    The SQL gates already prove the rows against Clone B.  G6 independently
    binds the public read model to the same immutable rows instead of treating
    every V8 addition or retired legacy field as an arbitrary JSON exception.
    """

    if not SHA256_RE.fullmatch(expected_mapping_sha256):
        raise ValueError("expected reviewed mapping binding is not SHA-256")
    if not SHA256_RE.fullmatch(expected_baseline_attestation_sha256):
        raise ValueError("expected baseline attestation binding is not SHA-256")
    output: dict[str, dict[str, Any]] = {
        "tasks": {},
        "groups": {},
        "revisions": {},
        "finals": {},
        "sources": {},
        "references": {},
    }
    seen: set[tuple[str, str]] = set()
    for line_no, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if not line.strip():
            continue
        row = json.loads(line)
        if not isinstance(row, dict) or row.get("run_id") != run_id:
            continue
        gate = row.get("gate_name")
        if gate not in {"G01", "G02", "G03", "G04", "G05"}:
            continue
        if row.get("review_state") != "pass" or row.get("expected_state") != "matched":
            raise ValueError(f"manifest line {line_no} is not a passed matched row")
        detail = row.get("detail_json")
        components = detail.get("components") if isinstance(detail, dict) else None
        if not isinstance(components, list) or not all(
            isinstance(item, str) for item in components
        ):
            raise ValueError(f"manifest line {line_no} lacks string components")
        input_hashes = detail.get("input_sha256") if isinstance(detail, dict) else None
        if not isinstance(input_hashes, dict):
            raise ValueError(f"manifest line {line_no} lacks input_sha256")
        if input_hashes.get("mapping_sha256") != expected_mapping_sha256:
            raise ValueError(f"manifest line {line_no} mapping hash differs")
        if (
            input_hashes.get("baseline_attestation_sha256")
            != expected_baseline_attestation_sha256
        ):
            raise ValueError(
                f"manifest line {line_no} baseline attestation hash differs"
            )
        entity = str(row.get("entity_key", ""))
        key = (str(gate), entity)
        if key in seen:
            raise ValueError(f"manifest line {line_no} duplicates {gate}/{entity}")
        seen.add(key)
        expected_hash = row.get("expected_hash")
        if (
            not isinstance(expected_hash, str)
            or expected_hash != component_sha256(components)
        ):
            raise ValueError(f"manifest line {line_no} component hash mismatch")
        if gate == "G01":
            if len(components) != 5 or not entity.startswith("task:"):
                raise ValueError(f"manifest line {line_no} has invalid G01 components")
            task_id = entity.split(":", 1)[1]
            output["tasks"][task_id] = {
                "task_id": components[0],
                "task_type": components[1],
                "task_status": components[2],
                "current_handler_id": components[3],
                "workflow_revision": components[4],
            }
        elif gate == "G02":
            if len(components) != 9 or not entity.startswith("group:"):
                raise ValueError(f"manifest line {line_no} has invalid G02 components")
            locator = ":".join(components[:3])
            output["groups"][locator] = {
                "task_id": components[0],
                "scope_kind": components[1],
                "scope_ref_id": components[2],
                "working_revision_no": components[3],
                "working_revision_status": components[4],
                "finalized_revision_no": components[5],
                "finalized_revision_status": components[6],
                "migration_incomplete": components[7],
                "migration_issue": components[8],
            }
        elif gate == "G03":
            if len(components) != 12 or not entity.startswith("revision:"):
                raise ValueError(f"manifest line {line_no} has invalid G03 components")
            locator = ":".join(components[:4])
            output["revisions"][locator] = {
                "task_id": components[0],
                "scope_kind": components[1],
                "scope_ref_id": components[2],
                "revision_no": components[3],
                "status": components[4],
                "mode": components[5],
                "source_locator": components[6],
                "source_stage": components[7],
                "created_by": components[8],
                "reason": components[9],
                "submitted_at": components[10],
                "finalized_at": components[11],
            }
        elif gate == "G04":
            if entity.startswith("revision-final:"):
                if len(components) != 9:
                    raise ValueError(
                        f"manifest line {line_no} has invalid G04 final components"
                    )
                locator = ":".join(entity.split(":")[1:5])
                output["finals"].setdefault(locator, []).append(
                    {
                        "task_asset_id": components[0],
                        "sort_order": components[1],
                        "formal_storage_ref_id": components[2],
                        "asset_type": components[3],
                        "whole_hash": components[4],
                        "binding": components[5],
                        "role": components[6],
                        "sku_code": components[7],
                        "retouch_requirement_id": components[8],
                    }
                )
            elif entity.startswith("revision-source:"):
                if len(components) != 7:
                    raise ValueError(
                        f"manifest line {line_no} has invalid G04 source components"
                    )
                locator = ":".join(entity.split(":")[1:5])
                output["sources"][locator] = {
                    "source_locator": components[0],
                    "role": components[1],
                    "whole_hash": components[2],
                    "binding": components[3],
                    "binding_role": components[4],
                    "sku_code": components[5],
                    "retouch_requirement_id": components[6],
                }
            else:
                raise ValueError(f"manifest line {line_no} has invalid G04 entity")
        else:
            if len(components) != 6 or not entity.startswith("revision-reference:"):
                raise ValueError(f"manifest line {line_no} has invalid G05 components")
            locator = ":".join(entity.split(":")[1:5])
            output["references"].setdefault(locator, []).append(
                {
                    "reference_file_ref_id": components[0],
                        "formal_storage_ref_id": components[1],
                    "sort_order": components[2],
                    "ref_id": components[3],
                    "file_name": components[4],
                    "scope": components[5],
                }
            )
    if not output["tasks"] or not output["groups"]:
        raise ValueError("reviewed manifest lacks G01/G02 API oracle rows")
    if set(output["tasks"]) != {
        value["task_id"] for value in output["tasks"].values()
    }:
        raise ValueError("G01 task entity and component IDs differ")
    for collection in ("finals", "references"):
        for rows in output[collection].values():
            rows.sort(key=lambda item: int(item["sort_order"]))
    return output


def load_asset_identity_map(
    path: pathlib.Path, run_id: str, manifest_sha256: str
) -> dict[str, dict[str, Any]]:
    document = load_object(path, "asset identity map")
    if set(document) != {
        "schema_version",
        "oracle_kind",
        "run_id",
        "inputs",
        "tasks",
        "roots",
        "versions",
        "revision_roles",
        "revision_reasons",
        "route_expectations",
        "evidence_sha256",
    }:
        raise ValueError("asset identity map has unexpected fields")
    evidence_sha = document["evidence_sha256"]
    unsigned = {
        key: value for key, value in document.items() if key != "evidence_sha256"
    }
    schema_version = document["schema_version"]
    oracle_kind = document["oracle_kind"]
    is_v2 = schema_version == 2 and oracle_kind == "non_circular_g6_v2"
    is_v3 = schema_version == 3 and oracle_kind == "non_circular_g6_v3"
    if (
        not (is_v2 or is_v3)
        or document["run_id"] != run_id
        or not isinstance(evidence_sha, str)
        or evidence_sha != sha256(canonical(unsigned))
    ):
        raise ValueError("asset identity map binding or evidence hash differs")
    inputs = document["inputs"]
    expected_v3_inputs = ORACLE_V2_INPUT_FIELDS | ORACLE_V3_ALIAS_INPUT_FIELDS
    if not isinstance(inputs, dict):
        raise ValueError("asset identity map input bindings differ")
    if is_v3 and set(inputs) != expected_v3_inputs:
        raise ValueError("asset identity map v3 input field contract differs")
    if (
        inputs.get("reviewed_manifest_sha256") != manifest_sha256
        or not isinstance(inputs.get("reviewed_mapping_sha256"), str)
        or not SHA256_RE.fullmatch(inputs["reviewed_mapping_sha256"])
        or not isinstance(inputs.get("snapshot_verdict_sha256"), str)
        or not SHA256_RE.fullmatch(inputs["snapshot_verdict_sha256"])
    ):
        raise ValueError("asset identity map input bindings differ")
    if is_v3:
        for field in sorted(expected_v3_inputs - {"clone_a_database"}):
            if (
                not isinstance(inputs[field], str)
                or not SHA256_RE.fullmatch(inputs[field])
            ):
                raise ValueError(
                    f"asset identity map v3 input {field} is not SHA-256"
                )
        if (
            not isinstance(inputs["clone_a_database"], str)
            or not inputs["clone_a_database"].strip()
        ):
            raise ValueError("asset identity map v3 clone A binding differs")
    versions = document["versions"]
    roots = document["roots"]
    tasks = document["tasks"]
    revision_roles = document["revision_roles"]
    revision_reasons = document["revision_reasons"]
    route_expectations = document["route_expectations"]
    if (
        not isinstance(versions, list)
        or not isinstance(roots, list)
        or not isinstance(tasks, list)
        or not isinstance(revision_roles, list)
        or not isinstance(revision_reasons, list)
        or not isinstance(route_expectations, dict)
    ):
        raise ValueError(
            "asset identity map collections have invalid types"
        )
    task_output: dict[str, dict[str, Any]] = {}
    for index, row in enumerate(tasks):
        if not isinstance(row, dict) or set(row) != {
            "task_id",
            "task_type",
            "task_status",
            "current_handler_id",
            "workflow_revision",
            "owner_department_id",
            "owner_team_id",
        }:
            raise ValueError(f"task oracle row {index} field contract differs")
        task_id = row["task_id"]
        if (
            not isinstance(task_id, int)
            or isinstance(task_id, bool)
            or task_id <= 0
            or any(
                value is not None
                and (
                    not isinstance(value, int)
                    or isinstance(value, bool)
                    or value <= 0
                )
                for value in (
                    row["owner_department_id"],
                    row["owner_team_id"],
                )
            )
            or str(task_id) in task_output
        ):
            raise ValueError(f"task oracle row {index} has invalid values")
        task_output[str(task_id)] = dict(row)

    root_required = {
        "root_asset_id",
        "task_id",
        "intrinsic_asset_type",
        "scope_sku_code",
        "retouch_requirement_id",
        "current_locator",
        "approved_locator",
        "provenance",
    }
    root_output: dict[str, dict[str, Any]] = {}
    for index, row in enumerate(roots):
        if not isinstance(row, dict) or set(row) != root_required:
            raise ValueError(f"root oracle row {index} field contract differs")
        root_id = row["root_asset_id"]
        task_id = row["task_id"]
        if (
            not isinstance(root_id, int)
            or isinstance(root_id, bool)
            or root_id <= 0
            or not isinstance(task_id, int)
            or isinstance(task_id, bool)
            or str(task_id) not in task_output
            or str(root_id) in root_output
            or not isinstance(row["intrinsic_asset_type"], str)
            or not isinstance(row["scope_sku_code"], str)
            or not isinstance(row["provenance"], dict)
        ):
            raise ValueError(f"root oracle row {index} has invalid values")
        root_output[str(root_id)] = dict(row)

    version_required = {
        "task_asset_id",
        "task_id",
        "root_asset_id",
        "stable_locator",
        "intrinsic_asset_type",
        "scope_sku_code",
        "retouch_requirement_id",
        "storage_ref_id",
        "object_key_sha256",
        "content_sha256",
        "size",
        "mime_type",
        "upload_status",
        "deleted_at",
        "cleaned_at",
        "object_deleted_at",
        "asset_version_no",
        "flow_review_status",
        "approved_at",
        "approved_by",
        "created_at",
        "source_asset_version_id",
        "content_availability",
        "expected_roles",
        "provenance",
    }
    if is_v3:
        version_required |= {
            "binding_state",
            "bound_role",
            "bound_resource_locator",
        }
    version_native: dict[str, dict[str, Any]] = {}
    locator_output: dict[str, dict[str, Any]] = {}
    for index, row in enumerate(versions):
        if not isinstance(row, dict) or set(row) != version_required:
            raise ValueError(f"asset identity row {index} field contract differs")
        asset_id = row["task_asset_id"]
        task_id = row["task_id"]
        root_id = row["root_asset_id"]
        retouch_id = row["retouch_requirement_id"]
        if (
            not isinstance(asset_id, int)
            or isinstance(asset_id, bool)
            or asset_id <= 0
            or not isinstance(task_id, int)
            or isinstance(task_id, bool)
            or task_id <= 0
            or not isinstance(root_id, int)
            or isinstance(root_id, bool)
            or root_id <= 0
            or str(root_id) not in root_output
            or root_output[str(root_id)]["task_id"] != task_id
            or (
                retouch_id is not None
                and (
                    not isinstance(retouch_id, int)
                    or isinstance(retouch_id, bool)
                    or retouch_id <= 0
                )
            )
            or str(task_id) not in task_output
            or not isinstance(row["stable_locator"], str)
            or not row["stable_locator"]
            or not isinstance(row["intrinsic_asset_type"], str)
            or not isinstance(row["scope_sku_code"], str)
            or not isinstance(row["storage_ref_id"], str)
            or not isinstance(row["object_key_sha256"], str)
            or (
                row["object_key_sha256"]
                and not SHA256_RE.fullmatch(row["object_key_sha256"])
            )
            or not isinstance(row["content_sha256"], str)
            or (
                row["content_sha256"]
                and not SHA256_RE.fullmatch(row["content_sha256"])
            )
            or not isinstance(row["size"], int)
            or isinstance(row["size"], bool)
            or row["size"] < 0
            or not isinstance(row["mime_type"], str)
            or not isinstance(row["approved_at"], str)
            or (
                row["approved_by"] is not None
                and (
                    not isinstance(row["approved_by"], int)
                    or isinstance(row["approved_by"], bool)
                    or row["approved_by"] <= 0
                )
            )
            or not isinstance(row["expected_roles"], list)
            or sorted(set(row["expected_roles"])) != row["expected_roles"]
            or not isinstance(row["provenance"], dict)
            or (
                is_v3
                and (
                    not isinstance(row["binding_state"], str)
                    or not isinstance(row["bound_role"], str)
                    or not isinstance(row["bound_resource_locator"], str)
                    or (
                        row["bound_resource_locator"]
                        and (
                            row["binding_state"] != "bound"
                            or row["bound_role"] not in {"source", "final"}
                            or row["bound_role"] not in row["expected_roles"]
                            or not re.fullmatch(
                                r"[1-9][0-9]*:"
                                r"(?:task:0|sku:[1-9][0-9]*|"
                                r"retouch_requirement:[1-9][0-9]*)",
                                row["bound_resource_locator"],
                            )
                        )
                    )
                )
            )
        ):
            raise ValueError(f"asset identity row {index} has invalid values")
        key = str(asset_id)
        if key in version_native or row["stable_locator"] in locator_output:
            raise ValueError(f"asset identity row {index} duplicates {asset_id}")
        version_native[key] = dict(row)
        locator_output[row["stable_locator"]] = version_native[key]

    if is_v3:
        for index, row in enumerate(versions):
            provenance = row["provenance"]
            if provenance.get("kind") != "source_alias_apply_receipt":
                continue
            if set(provenance) != {
                "kind",
                "origin_task_asset_id",
                "origin_locator",
                "group_id",
                "remark",
            }:
                raise ValueError(
                    f"source-alias identity row {index} provenance differs"
                )
            origin_id = provenance["origin_task_asset_id"]
            group_id = provenance["group_id"]
            if (
                not isinstance(origin_id, int)
                or isinstance(origin_id, bool)
                or origin_id <= 0
                or not isinstance(group_id, int)
                or isinstance(group_id, bool)
                or group_id <= 0
                or origin_id == row["task_asset_id"]
            ):
                raise ValueError(
                    f"source-alias identity row {index} allocation differs"
                )
            origin = version_native.get(str(origin_id))
            expected_locator = (
                f"alias:v1:{row['bound_resource_locator']}:"
                f"origin-task-asset:{origin_id}"
            )
            expected_remark = (
                f"v8-source-alias:group={group_id}:origin={origin_id}"
            )
            immutable_fields = (
                "task_id",
                "root_asset_id",
                "storage_ref_id",
                "object_key_sha256",
                "content_sha256",
                "size",
                "mime_type",
                "upload_status",
                "asset_version_no",
                "content_availability",
            )
            if (
                origin is None
                or provenance["origin_locator"] != origin["stable_locator"]
                or origin["intrinsic_asset_type"]
                not in {
                    "delivery",
                    "draft",
                    "revised",
                    "final",
                    "outsource_return",
                }
                or row["stable_locator"] != expected_locator
                or provenance["remark"] != expected_remark
                or row["intrinsic_asset_type"] != "source"
                or row["binding_state"] != "bound"
                or row["bound_role"] != "source"
                or row["expected_roles"] != ["source"]
                or row["flow_review_status"] != "not_applicable"
                or row["approved_at"]
                or row["approved_by"] is not None
                or row["source_asset_version_id"] is not None
                or any(row[field] != origin[field] for field in immutable_fields)
            ):
                raise ValueError(
                    f"source-alias identity row {index} lineage differs"
                )

    expected_route_fields = {
        "detail_visible_locators",
        "list_root_ids",
        "current_locators",
        "approved_locators",
        "historical_unavailable_locators",
    }
    if set(route_expectations) != expected_route_fields or not all(
        isinstance(route_expectations[field], list)
        and len(route_expectations[field])
        == len(set(route_expectations[field]))
        for field in expected_route_fields
    ):
        raise ValueError("asset identity map route expectations differ")
    locator_fields = (
        "detail_visible_locators",
        "current_locators",
        "approved_locators",
        "historical_unavailable_locators",
    )
    for field in locator_fields:
        if not all(
            isinstance(locator, str) and locator in locator_output
            for locator in route_expectations[field]
        ):
            raise ValueError(f"asset identity map {field} has unknown locator")
    if sorted(route_expectations["list_root_ids"]) != sorted(
        int(root_id)
        for root_id, root in root_output.items()
        if root["current_locator"] is not None
    ):
        raise ValueError("asset identity map list root IDs differ")
    for root_id, root in root_output.items():
        for pointer, field in (
            ("current_locator", "current_locators"),
            ("approved_locator", "approved_locators"),
        ):
            locator = root[pointer]
            if locator is not None and (
                locator not in locator_output
                or locator_output[locator]["root_asset_id"] != int(root_id)
                or locator not in route_expectations[field]
            ):
                raise ValueError(f"asset identity map root {pointer} differs")

    output: dict[str, dict[str, Any]] = {}
    detail_locators = set(route_expectations["detail_visible_locators"])
    current_locators = set(route_expectations["current_locators"])
    approved_locators = set(route_expectations["approved_locators"])
    for key, row in version_native.items():
        root = root_output[str(row["root_asset_id"])]
        provenance = row["provenance"]
        manifest_locator = row["stable_locator"]
        if is_v3:
            manifest_locator = (
                f"bundle:{row['content_sha256']}"
                if provenance.get("kind") == "bundle_receipt"
                else f"asset:{row['root_asset_id']}:{row['storage_ref_id']}"
            )
        output[key] = {
            **row,
            "asset_type": row["intrinsic_asset_type"],
            "whole_hash": row["content_sha256"],
            "manifest_locator": manifest_locator,
            "binding_state": (
                row["binding_state"]
                if is_v3
                else str(provenance.get("a_binding_state", "bound"))
            ),
            "bound_role": (
                row["bound_role"]
                if is_v3
                else str(provenance.get("a_bound_role", ""))
            ),
            "root_asset_type": root["intrinsic_asset_type"],
            "root_scope_sku_code": root["scope_sku_code"],
            "root_retouch_requirement_id": root["retouch_requirement_id"],
            "detail_visible": row["stable_locator"] in detail_locators,
            "list_current_version": row["stable_locator"] in current_locators,
            "list_approved_version": row["stable_locator"] in approved_locators,
        }

    role_output: dict[str, dict[str, Any]] = {}
    role_required = {
        "revision_locator",
        "task_id",
        "scope_kind",
        "scope_ref_id",
        "revision_no",
        "status",
        "source_stage",
        "source_kind",
        "source_locator",
        "final_locators",
        "reference_file_ref_ids",
        "reference_locators",
        "is_working",
        "is_finalized",
    }
    for index, row in enumerate(revision_roles):
        if (
            not isinstance(row, dict)
            or set(row) != role_required
            or not isinstance(row["revision_locator"], str)
            or row["revision_locator"] in role_output
            or (
                row["source_locator"] is not None
                and row["source_locator"] not in locator_output
            )
            or not isinstance(row["final_locators"], list)
            or any(item not in locator_output for item in row["final_locators"])
        ):
            raise ValueError(f"revision role oracle row {index} is invalid")
        if is_v3:
            resource_locator = (
                f"{row['task_id']}:{row['scope_kind']}:{row['scope_ref_id']}"
            )
            source = (
                locator_output.get(row["source_locator"])
                if row["source_locator"] is not None
                else None
            )
            if source is not None and (
                source["binding_state"] != "bound"
                or source["bound_role"] != "source"
                or source["bound_resource_locator"] != resource_locator
                or "source" not in source["expected_roles"]
            ):
                raise ValueError(
                    f"revision role oracle row {index} source binding differs"
                )
            if row["source_kind"] == "delivery_source_alias" and (
                source is None
                or source["provenance"].get("kind")
                != "source_alias_apply_receipt"
            ):
                raise ValueError(
                    f"revision role oracle row {index} source alias differs"
                )
            for final_locator in row["final_locators"]:
                final = locator_output[final_locator]
                if (
                    final["binding_state"] != "bound"
                    or final["bound_role"] != "final"
                    or final["bound_resource_locator"] != resource_locator
                    or "final" not in final["expected_roles"]
                ):
                    raise ValueError(
                        f"revision role oracle row {index} final binding differs"
                    )
        role_output[row["revision_locator"]] = dict(row)
    reason_output: dict[str, str] = {}
    for index, row in enumerate(revision_reasons):
        if (
            not isinstance(row, dict)
            or set(row) != {"revision_locator", "reason_sha256"}
            or not isinstance(row["revision_locator"], str)
            or not row["revision_locator"]
            or not isinstance(row["reason_sha256"], str)
            or not SHA256_RE.fullmatch(row["reason_sha256"])
            or row["revision_locator"] in reason_output
        ):
            raise ValueError(
                f"revision reason oracle row {index} is invalid"
            )
        reason_output[row["revision_locator"]] = row["reason_sha256"]
    return {
        "tasks": task_output,
        "assets": output,
        "roots": root_output,
        "versions": version_native,
        "versions_by_locator": locator_output,
        "revision_roles": role_output,
        "route_expectations": route_expectations,
        "revision_reasons": reason_output,
        "reviewed_mapping_sha256": inputs["reviewed_mapping_sha256"],
        "inputs": dict(inputs),
        "schema_version": schema_version,
        "oracle_kind": oracle_kind,
    }


def local_url(url: str) -> str:
    parsed = urllib.parse.urlparse(url)
    if (
        parsed.scheme not in {"http", "https"}
        or parsed.hostname not in {"127.0.0.1", "localhost", "host.docker.internal"}
        or parsed.username
        or parsed.password
        or parsed.query
        or parsed.fragment
    ):
        raise ValueError("API clone URL must be a credential-free local URL")
    return url.rstrip("/")


def _validate_headers(value: object) -> dict[str, str]:
    if not isinstance(value, dict) or not value:
        raise ValueError("identity headers must be a non-empty string map")
    headers: dict[str, str] = {}
    forbidden = {"host", "content-length", "transfer-encoding"}
    for key, item in value.items():
        if (
            not isinstance(key, str)
            or not isinstance(item, str)
            or not key.strip()
            or "\r" in key
            or "\n" in key
            or "\r" in item
            or "\n" in item
            or key.lower() in forbidden
        ):
            raise ValueError("identity headers contain an invalid entry")
        headers[key] = item
    return headers


def resolve_headers(identity: dict[str, Any]) -> dict[str, str]:
    sources = [
        key
        for key in ("headers_file", "headers_file_env", "headers_json_env")
        if key in identity
    ]
    if len(sources) != 1:
        raise ValueError(
            f"identity {identity.get('id')} must declare exactly one header source"
        )
    source = sources[0]
    if source == "headers_file":
        path = pathlib.Path(str(identity[source]))
        value = json.loads(path.read_text(encoding="utf-8"))
    elif source == "headers_file_env":
        env_name = str(identity[source])
        if env_name not in os.environ:
            raise ValueError(f"missing header-file environment variable {env_name}")
        value = json.loads(pathlib.Path(os.environ[env_name]).read_text(encoding="utf-8"))
    else:
        env_name = str(identity[source])
        if env_name not in os.environ:
            raise ValueError(f"missing header-json environment variable {env_name}")
        value = json.loads(os.environ[env_name])
    return _validate_headers(value)


@dataclasses.dataclass(frozen=True)
class HttpResult:
    status: int
    body: object
    raw_sha256: str
    body_bytes: int


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):  # noqa: ANN001
        return None


def http_get(base_url: str, path: str, headers: dict[str, str]) -> HttpResult:
    request = urllib.request.Request(base_url + path, headers=headers, method="GET")
    try:
        with urllib.request.build_opener(_NoRedirect()).open(request, timeout=20) as response:
            status = int(response.status)
            raw = response.read(MAX_BODY_BYTES + 1)
    except urllib.error.HTTPError as exc:
        status = int(exc.code)
        raw = exc.read(MAX_BODY_BYTES + 1)
    if len(raw) > MAX_BODY_BYTES:
        raise ValueError(f"GET {path} response exceeds {MAX_BODY_BYTES} bytes")
    try:
        body: object = json.loads(raw) if raw else None
    except json.JSONDecodeError:
        body = {
            "_non_json_sha256": sha256(raw),
            "_bytes": len(raw),
        }
    return HttpResult(status, body, sha256(raw), len(raw))


def _download_origin(
    value: str,
    *,
    require_explicit_port: bool,
    allow_path: bool,
) -> str:
    text = value.strip()
    parsed = urllib.parse.urlsplit(text)
    if (
        parsed.scheme.lower() not in {"http", "https"}
        or not parsed.hostname
        or parsed.username is not None
        or parsed.password is not None
        or (not allow_path and parsed.path not in {"", "/"})
        or (not allow_path and (parsed.query or parsed.fragment))
    ):
        raise ValueError(
            "download allowlist entries require scheme and explicit port"
        )
    try:
        explicit_port = parsed.port
    except ValueError as exc:
        raise ValueError(
            "download allowlist entries require scheme and explicit port"
        ) from exc
    if require_explicit_port and explicit_port is None:
        raise ValueError(
            "download allowlist entries require scheme and explicit port"
        )
    port = explicit_port
    if port is None:
        port = 443 if parsed.scheme.lower() == "https" else 80
    hostname = parsed.hostname.lower()
    display_host = f"[{hostname}]" if ":" in hostname else hostname
    return f"{parsed.scheme.lower()}://{display_host}:{port}"


def download_allowed_hosts() -> tuple[str, ...]:
    origins = {
        _download_origin(
            value,
            require_explicit_port=True,
            allow_path=False,
        )
        for value in os.environ.get(
            "AB_DOWNLOAD_ALLOWED_HOSTS", ""
        ).split(",")
        if value.strip()
    }
    return tuple(sorted(origins))


def http_download(
    base_url: str, path: str, headers: dict[str, str]
) -> bytes:
    metadata_request = urllib.request.Request(
        base_url + path, headers=headers, method="GET"
    )
    with urllib.request.build_opener(_NoRedirect()).open(
        metadata_request, timeout=30
    ) as response:
        metadata_raw = response.read(MAX_BODY_BYTES + 1)
    if len(metadata_raw) > MAX_BODY_BYTES:
        raise ValueError(
            f"recovery GET {path} metadata exceeds {MAX_BODY_BYTES} bytes"
        )
    try:
        metadata = unwrap_data(json.loads(metadata_raw))
    except json.JSONDecodeError as exc:
        raise ValueError(
            f"recovery GET {path} did not return controlled download metadata"
        ) from exc
    download_url = (
        metadata.get("download_url") if isinstance(metadata, dict) else None
    )
    if not isinstance(download_url, str) or not download_url:
        raise ValueError(f"recovery GET {path} lacks download_url")
    parsed = urllib.parse.urlsplit(download_url)
    if parsed.scheme and parsed.scheme not in {"http", "https"}:
        raise ValueError(f"recovery GET {path} returned an unsafe download URL")
    if parsed.username is not None or parsed.password is not None:
        raise ValueError(f"recovery GET {path} returned a credentialed URL")
    if parsed.scheme or parsed.netloc:
        try:
            origin = _download_origin(
                download_url,
                require_explicit_port=False,
                allow_path=True,
            )
        except ValueError as exc:
            raise ValueError(
                f"recovery GET {path} returned an unsafe download URL"
            ) from exc
        allowed_hosts = set(download_allowed_hosts())
        if origin not in allowed_hosts:
            raise ValueError(
                f"recovery GET {path} returned a non-allowlisted origin"
            )
    resolved_url = urllib.parse.urljoin(base_url + "/", download_url)
    download_headers = headers if not parsed.scheme and not parsed.netloc else {}
    request = urllib.request.Request(
        resolved_url, headers=download_headers, method="GET"
    )
    with urllib.request.build_opener(_NoRedirect()).open(
        request, timeout=30
    ) as response:
        raw = response.read(MAX_BODY_BYTES + 1)
    if len(raw) > MAX_BODY_BYTES:
        raise ValueError(
            f"recovery GET {path} response exceeds {MAX_BODY_BYTES} bytes"
        )
    return raw


def _rule_without_hash(rule: dict[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in rule.items() if key != "rule_sha256"}


def load_rules(
    path: pathlib.Path,
    retired_routes: Iterable[str],
    known_identities: Iterable[str],
) -> list[dict[str, Any]]:
    document = load_object(path, "rules")
    if set(document) != {"schema_version", "rules"} or document["schema_version"] != 1:
        raise ValueError("rules must contain only schema_version=1 and rules")
    rules = document["rules"]
    if not isinstance(rules, list):
        raise ValueError("rules.rules must be an array")
    allowed_routes = ALL_ROUTE_TEMPLATES | frozenset(retired_routes)
    allowed_identities = frozenset(known_identities)
    seen: set[str] = set()
    output: list[dict[str, Any]] = []
    for index, value in enumerate(rules):
        if not isinstance(value, dict):
            raise ValueError(f"rule {index} must be an object")
        required = {
            "rule_id",
            "route",
            "direction",
            "from_status",
            "to_status",
            "reason",
            "reason_sha256",
            "operations",
            "rule_sha256",
        }
        fields = set(value)
        if not required <= fields or fields - required not in (set(), {"identity"}):
            raise ValueError(f"rule {index} has unexpected or missing fields")
        rule_id = value["rule_id"]
        if not isinstance(rule_id, str) or not rule_id or rule_id in seen:
            raise ValueError(f"rule {index} has invalid or duplicate rule_id")
        seen.add(rule_id)
        if "identity" in value and (
            not isinstance(value["identity"], str)
            or not value["identity"].strip()
            or value["identity"] not in allowed_identities
        ):
            raise ValueError(f"rule {rule_id} binds an unknown identity")
        if value["route"] not in allowed_routes:
            raise ValueError(f"rule {rule_id} binds an unknown route")
        if value["direction"] not in {
            f"{left}->{right}"
            for offset, left in enumerate(COMBINATION_IDS)
            for right in COMBINATION_IDS[offset + 1 :]
        }:
            raise ValueError(f"rule {rule_id} has an invalid direction")
        if not all(isinstance(value[key], int) for key in ("from_status", "to_status")):
            raise ValueError(f"rule {rule_id} statuses must be integers")
        reason = value["reason"]
        if (
            not isinstance(reason, str)
            or not reason.strip()
            or value["reason_sha256"] != sha256(reason.encode("utf-8"))
        ):
            raise ValueError(f"rule {rule_id} reason hash mismatch")
        operations = value["operations"]
        if not isinstance(operations, list):
            raise ValueError(f"rule {rule_id} operations must be an array")
        for operation in operations:
            if not isinstance(operation, dict) or operation.get("op") not in {
                "remove",
                "map",
            }:
                raise ValueError(f"rule {rule_id} has an invalid operation")
            if operation["op"] == "remove" and set(operation) != {"op", "path"}:
                raise ValueError(f"rule {rule_id} remove operation is invalid")
            if operation["op"] == "map" and set(operation) != {
                "op",
                "path",
                "from",
                "to",
            }:
                raise ValueError(f"rule {rule_id} map operation is invalid")
            path_value = operation.get("path")
            if not isinstance(path_value, str) or not path_value.startswith("/"):
                raise ValueError(f"rule {rule_id} operation path is invalid")
            path_tokens = _tokens(path_value)
            protected = {
                "allowed_actions",
                "items",
                "references",
                "revision_no",
                "sort_order",
                "source_task_asset_id",
                "task_asset_id",
                "task_asset_ids",
                "final_task_asset_id",
                "final_task_asset_ids",
                "formal_task_asset_id",
                "formal_task_asset_ids",
                "task_id",
                "scope_kind",
                "scope_ref_id",
                "task_sku_item_id",
                "retouch_requirement_id",
            }
            if protected & set(path_tokens):
                verb = "remove" if operation["op"] == "remove" else "normalize"
                raise ValueError(
                    f"rule {rule_id} cannot {verb} an ordered, permission, "
                    "identity, or scope field"
                )
            if operation["op"] == "remove":
                removable_leaf_fields = {
                    "access_hint",
                    "display_name",
                    "expires_at",
                    "generated_at",
                    "notes",
                    "trace_id",
                    "updated_at",
                }
                if (
                    len(path_tokens) < 2
                    or path_tokens[-1] not in removable_leaf_fields
                    or "*" in path_tokens
                    or any(token.isdigit() for token in path_tokens)
                ):
                    raise ValueError(
                        f"rule {rule_id} cannot remove a non-volatile or "
                        "structural payload field"
                    )
        if (
            not isinstance(value["rule_sha256"], str)
            or not SHA256_RE.fullmatch(value["rule_sha256"])
            or value["rule_sha256"] != sha256(canonical(_rule_without_hash(value)))
        ):
            raise ValueError(f"rule {rule_id} rule hash mismatch")
        output.append(value)
    return output


def load_matrix(path: pathlib.Path) -> tuple[dict[str, str], list[dict[str, str]], dict[str, dict[str, str]], list[str]]:
    document = load_object(path, "matrix")
    if set(document) != {
        "schema_version",
        "combinations",
        "identities",
        "retired_routes",
    } or document["schema_version"] != 1:
        raise ValueError("matrix has unexpected fields or schema_version")
    combinations = document["combinations"]
    if not isinstance(combinations, list) or len(combinations) != 4:
        raise ValueError("matrix must contain exactly four combinations")
    urls: dict[str, str] = {}
    metadata: list[dict[str, str]] = []
    expected = {
        "external_external_a": ("external", "external", "A"),
        "dev_dev_b": ("dev-plus", "dev-plus", "B"),
        "external_dev_b": ("external", "dev-plus", "B"),
        "dev_external_a": ("dev-plus", "external", "A"),
    }
    for combination in combinations:
        if not isinstance(combination, dict) or set(combination) != {
            "id",
            "frontend",
            "backend",
            "data",
            "base_url",
        }:
            raise ValueError("combination has unexpected fields")
        combo_id = combination["id"]
        if combo_id not in expected or combo_id in urls:
            raise ValueError("combination id is invalid or duplicated")
        actual = (combination["frontend"], combination["backend"], combination["data"])
        if actual != expected[combo_id]:
            raise ValueError(f"combination {combo_id} does not match the fixed matrix")
        urls[combo_id] = local_url(str(combination["base_url"]))
        metadata.append(
            {
                "id": combo_id,
                "frontend": str(combination["frontend"]),
                "backend": str(combination["backend"]),
                "data": str(combination["data"]),
                "origin_sha256": sha256(urls[combo_id].encode("utf-8")),
            }
        )
    if set(urls) != set(COMBINATION_IDS):
        raise ValueError("all four combination URLs must be present")
    a_origins = {
        urls["external_external_a"],
        urls["dev_external_a"],
    }
    b_origins = {
        urls["dev_dev_b"],
        urls["external_dev_b"],
    }
    if a_origins & b_origins:
        raise ValueError("A and B combinations must use disjoint physical origins")
    identities_value = document["identities"]
    if not isinstance(identities_value, list) or not identities_value:
        raise ValueError("matrix must contain at least one identity")
    identities: list[dict[str, str]] = []
    resolved: dict[str, dict[str, str]] = {}
    observed_roles: set[str] = set()
    for identity in identities_value:
        if not isinstance(identity, dict) or not {"id", "role"} <= set(identity):
            raise ValueError("identity must contain id and role")
        unexpected = set(identity) - {
            "id",
            "role",
            "headers_file",
            "headers_file_env",
            "headers_json_env",
        }
        identity_id = identity["id"]
        role = identity["role"]
        if (
            unexpected
            or not isinstance(identity_id, str)
            or not identity_id
            or not isinstance(role, str)
            or role not in REQUIRED_IDENTITY_ROLES
            or identity_id in resolved
        ):
            raise ValueError("identity is invalid or duplicated")
        resolved[identity_id] = resolve_headers(identity)
        identities.append({"id": identity_id, "role": role})
        observed_roles.add(role)
    if observed_roles != REQUIRED_IDENTITY_ROLES:
        raise ValueError(
            "matrix identities must contain admin, view_only, and no_view roles"
        )
    retired = document["retired_routes"]
    if not isinstance(retired, list) or not retired:
        raise ValueError("retired_routes must be a non-empty array")
    retired_routes: list[str] = []
    for route in retired:
        if (
            not isinstance(route, str)
            or not route.startswith("/v1/")
            or set(re.findall(r"{([^}]+)}", route)) - {"task_id"}
            or route in ALL_ROUTE_TEMPLATES
            or route in retired_routes
        ):
            raise ValueError("retired route is invalid or duplicated")
        retired_routes.append(route)
    return urls, sorted(metadata, key=lambda item: COMBINATION_IDS.index(item["id"])), resolved, sorted(retired_routes)


def group_ids(value: object) -> set[str]:
    found: set[str] = set()
    if isinstance(value, dict):
        group_id = value.get("group_id")
        if isinstance(group_id, int) and group_id > 0:
            found.add(str(group_id))
        if (
            "scope_kind" in value
            and isinstance(value.get("task_id"), int)
            and isinstance(value.get("id"), int)
            and value["id"] > 0
        ):
            found.add(str(value["id"]))
        for child in value.values():
            found.update(group_ids(child))
    elif isinstance(value, list):
        for child in value:
            found.update(group_ids(child))
    return found


def task_asset_ids(value: object) -> set[str]:
    found: set[str] = set()
    scalar_keys = {
        "task_asset_id",
        "source_task_asset_id",
        "final_task_asset_id",
        "formal_task_asset_id",
    }
    list_keys = {
        "task_asset_ids",
        "final_task_asset_ids",
        "formal_task_asset_ids",
    }
    if isinstance(value, dict):
        # /v1/tasks/{id}/assets returns DesignAssetVersion objects whose
        # immutable task_assets identity is the generic `id` field.
        if (
            isinstance(value.get("id"), int)
            and value["id"] > 0
            and isinstance(value.get("version_no"), int)
            and isinstance(value.get("asset_type"), str)
        ):
            found.add(str(value["id"]))
        for key, child in value.items():
            if key in scalar_keys and isinstance(child, int) and child > 0:
                found.add(str(child))
            elif key in list_keys and isinstance(child, list):
                found.update(str(item) for item in child if isinstance(item, int) and item > 0)
            found.update(task_asset_ids(child))
    elif isinstance(value, list):
        for child in value:
            found.update(task_asset_ids(child))
    return found


def normalize_transport_noise(value: object) -> object:
    """Remove only request-instance metadata that is not an API contract.

    Error trace IDs are deliberately random for every request.  Normalizing
    that exact envelope path lets same-backend comparisons remain strict
    without hiding business payload, permission, identity, or ordering fields.
    """

    if isinstance(value, dict):
        output = {
            key: normalize_transport_noise(child)
            for key, child in value.items()
        }
        error = output.get("error")
        if isinstance(error, dict):
            error = dict(error)
            error.pop("trace_id", None)
            output["error"] = error
        return output
    if isinstance(value, list):
        return [normalize_transport_noise(child) for child in value]
    return value


def field_paths(value: object, field: str, prefix: str = "$") -> list[str]:
    found: list[str] = []
    if isinstance(value, dict):
        for key, child in value.items():
            path = f"{prefix}.{key}"
            if key == field:
                found.append(path)
            found.extend(field_paths(child, field, path))
    elif isinstance(value, list):
        for index, child in enumerate(value):
            found.extend(field_paths(child, field, f"{prefix}[{index}]"))
    return found


def unwrap_data(value: object) -> object:
    if isinstance(value, dict) and set(value) == {"data"}:
        return value["data"]
    return value


def _drop_keys(value: object, keys: frozenset[str]) -> object:
    if isinstance(value, dict):
        return {
            key: _drop_keys(child, keys)
            for key, child in value.items()
            if key not in keys
        }
    if isinstance(value, list):
        return [_drop_keys(child, keys) for child in value]
    return value


TASK_STABLE_FIELDS = (
    "id",
    "task_no",
    "source_mode",
    "sku_code",
    "primary_sku_code",
    "product_name_snapshot",
    "assignee_id",
    "assignee_name",
    "creator_id",
    "creator_name",
    "requester_id",
    "requester_name",
    "designer_id",
    "designer_name",
    "batch_item_count",
    "batch_mode",
    "is_batch_task",
    "business_lane",
    "customization_required",
    "owner_department",
    "owner_org_team",
    "owner_team",
    "priority",
    "deadline_at",
    "created_at",
    "design_requirement",
    "reference_file_refs",
    "retouch_requirements",
    "sku_items",
)

TERMINAL_MODULE_STATES = frozenset(
    {"completed", "closed", "forcibly_closed", "closed_by_admin"}
)
ACTIVE_MODULE_STATES = frozenset(
    {
        "active",
        "pending",
        "pending_claim",
        "in_progress",
        "submitted",
        "approved",
        "preparing",
        "received",
        "review",
    }
)


def project_task(value: object) -> object:
    data = unwrap_data(value)
    if not isinstance(data, dict):
        return data
    normalized = _drop_keys(data, frozenset({"set_mode_hint"}))
    assert isinstance(normalized, dict)
    return {
        field: {
            "present": field in normalized,
            "value": (
                _drop_keys(
                    normalized.get(field),
                    frozenset({"updated_at"}),
                )
                if field == "sku_items"
                else (
                    project_nested_asset_versions(normalized.get(field))
                    if field == "retouch_requirements"
                    else normalized.get(field)
                )
            ),
        }
        for field in TASK_STABLE_FIELDS
    }


def project_asset_version(value: object) -> object:
    if not isinstance(value, dict):
        return value
    # V8 assigns legacy task-asset versions to their reviewed SKU scope.
    # B-side asset/task/type/scope values are checked against the hash-bound
    # API oracle before this intrinsic A/B projection.  This is deliberately a
    # direct-field projection: an undeclared scope field on the DesignAsset
    # root must remain visible as a contract difference.
    output = {
        key: child
        for key, child in value.items()
        if key
        not in {
            "retouch_requirement_id",
            "scope_sku_code",
            "warehouse_ready",
        }
    }
    role = output.get("current_version_role")
    if role == "current_warehouse_ready_version":
        output["current_version_role"] = "current_approved_version"
    elif role == "warehouse_ready_version":
        output["current_version_role"] = "approved_version"
    asset_type = str(output.get("asset_type") or "reference")
    copy_pairs = {
        "source": {
            "access_hint": (
                "Use task_no={task_no} asset_no={asset_no} "
                "version_no={version_no} object_key={storage_key} to fetch "
                "the OSS-backed source file.",
                "源文件属于任务 {task_no}，文件编号 {asset_no}，"
                "第 {version_no} 版。",
            ),
            "notes": (
                "Source files remain OSS-backed business assets; "
                "no NAS-only path is required.",
                "当前源文件由任务资源组统一管理。",
            ),
        },
        "delivery": {
            "access_hint": (
                "Delivery assets are the business flow truth for audit "
                "and warehouse after approval.",
                "成品图通过审核后，作为任务当前有效成品。",
            ),
            "notes": (
                "Warehouse and audit should consume the "
                "warehouse_ready_version or approved_version based on "
                "current task status.",
                "当前成品图由任务资源组统一管理。",
            ),
        },
        "preview": {
            "access_hint": (
                "Preview assets are auxiliary only and must not replace "
                "delivery assets in business flow.",
                "该文件仅用于预览，不替代正式成品图。",
            ),
            "notes": (
                "Preview artifacts are not the formal source of truth.",
                "预览文件不是正式业务文件。",
            ),
        },
        "design_thumb": {
            "access_hint": (
                "Design thumb assets are lightweight preview derivatives "
                "for list/detail rendering.",
                "该缩略图仅用于页面预览。",
            ),
            "notes": (
                "Design thumb artifacts are backend-owned derivatives "
                "for preview rendering only.",
                "缩略图只用于页面预览。",
            ),
        },
        "reference": {
            "access_hint": (
                "Reference assets are task-scoped files for task creation, "
                "design reference, and business understanding only.",
                "参考图用于说明任务需求。",
            ),
            "notes": (
                "Reference assets never enter the "
                "warehouse_ready_version path.",
                "参考图用于说明任务需求。",
            ),
        },
    }
    pair = copy_pairs.get(
        "reference" if asset_type == "erp_product_image" else asset_type
    )
    format_values = {
        "task_no": str(output.get("task_no") or ""),
        "asset_no": str(output.get("asset_no") or ""),
        "version_no": output.get("version_no", 0),
        "storage_key": str(output.get("storage_key") or ""),
    }
    if pair is not None:
        for field in ("access_hint", "notes"):
            legacy, current = (
                template.format(**format_values)
                for template in pair[field]
            )
            if output.get(field) in {legacy, current}:
                output[field] = f"v8_asset_copy_v1:{asset_type}:{field}"
    return output


def expected_asset_usable_state(expected: dict[str, Any]) -> str:
    intrinsic_type = str(expected.get("intrinsic_asset_type") or "")
    delivery_type = intrinsic_type in {
        "delivery",
        "draft",
        "final",
        "outsource_return",
        "revised",
    }
    if expected.get("cleaned_at") or not expected.get("object_key_sha256"):
        return "cleaned" if delivery_type else "not_applicable"
    status = str(expected.get("flow_review_status") or "")
    if status not in {
        "approved",
        "cleaned",
        "not_applicable",
        "pending_review",
        "rejected",
        "superseded",
    }:
        status = "pending_review" if delivery_type else "not_applicable"
    return {
        "approved": "ready_for_use",
        "cleaned": "cleaned",
        "not_applicable": "not_applicable",
        "pending_review": "pending_review",
        "rejected": "rejected",
        "superseded": "history",
    }[status]


def effective_oracle_scope_sku_code(expected: dict[str, Any]) -> str:
    return str(
        expected.get("scope_sku_code")
        or expected.get("root_scope_sku_code")
        or ""
    )


def effective_oracle_retouch_requirement_id(
    expected: dict[str, Any],
) -> int | None:
    value = (
        expected.get("retouch_requirement_id")
        or expected.get("root_retouch_requirement_id")
    )
    return value if isinstance(value, int) and not isinstance(value, bool) else None


def project_nested_asset_versions(value: object) -> object:
    """Apply the asset-version projection to embedded version contracts."""

    if isinstance(value, dict):
        if (
            isinstance(value.get("id"), int)
            and not isinstance(value.get("id"), bool)
            and value["id"] > 0
            and isinstance(value.get("task_id"), int)
            and not isinstance(value.get("task_id"), bool)
            and value["task_id"] > 0
            and isinstance(value.get("asset_id"), int)
            and not isinstance(value.get("asset_id"), bool)
            and value["asset_id"] > 0
            and isinstance(value.get("version_no"), int)
            and not isinstance(value.get("version_no"), bool)
            and value["version_no"] > 0
            and isinstance(value.get("asset_type"), str)
        ):
            return project_asset_version(value)
        return {
            key: project_nested_asset_versions(child)
            for key, child in value.items()
        }
    if isinstance(value, list):
        return [project_nested_asset_versions(child) for child in value]
    return value


def project_asset_resource(
    value: object,
    *,
    normalize_approval: bool = False,
    recovery_task_asset_ids: frozenset[str] = frozenset(),
    bundle_member_task_asset_ids: frozenset[str] = frozenset(),
) -> object:
    if not isinstance(value, dict):
        return value
    excluded_root_fields = {
        "retouch_requirement_id",
        "scope_sku_code",
        "warehouse_ready_version",
        "warehouse_ready_version_id",
    }
    if normalize_approval:
        excluded_root_fields.update(
            {"approved_version", "approved_version_id"}
        )
    output = {
        key: child
        for key, child in value.items()
        if key not in excluded_root_fields
    }
    for key in ("current_version", "approved_version"):
        if key in output:
            projected = project_asset_version(output[key])
            if isinstance(projected, dict) and normalize_approval:
                projected = {
                    field: child
                    for field, child in projected.items()
                    if field
                    not in {
                        "approved_at",
                        "approved_for_flow",
                        "approved_by",
                        "current_version_role",
                        "file_hash",
                        "flow_review_status",
                        "usable_state",
                    }
                }
            if (
                isinstance(projected, dict)
                and str(projected.get("id")) in recovery_task_asset_ids
            ):
                projected = dict(projected)
                for field in (
                    "download_url",
                    "file_hash",
                    "storage_key",
                    "usable_state",
                ):
                    if field in projected:
                        projected[field] = (
                            f"v8_recovery_receipt_v1:{field}"
                        )
            if (
                isinstance(projected, dict)
                and str(projected.get("id"))
                in bundle_member_task_asset_ids
            ):
                projected["file_hash"] = (
                    "v8_source_bundle_member_receipt_v1:file_hash"
                )
            output[key] = projected
    return output


def project_assets(value: object) -> object:
    data = unwrap_data(value)
    if not isinstance(data, list):
        return data
    rows = [project_asset_resource(item) for item in data]
    return sorted(
        rows,
        key=lambda item: (
            item.get("id", 0) if isinstance(item, dict) else 0,
            canonical(item),
        ),
    )


def project_asset_pair(
    baseline_value: object,
    candidate_value: object,
    approved_candidate_root_ids: frozenset[str],
    asset_identity_oracle: dict[str, dict[str, Any]],
) -> tuple[object, object]:
    baseline_data = unwrap_data(baseline_value)
    candidate_data = unwrap_data(candidate_value)
    if not isinstance(baseline_data, list) or not isinstance(candidate_data, list):
        return project_assets(baseline_value), project_assets(candidate_value)
    baseline_root_ids = {
        str(item.get("id"))
        for item in baseline_data
        if isinstance(item, dict)
        and isinstance(item.get("id"), int)
        and not isinstance(item.get("id"), bool)
        and item["id"] > 0
    }
    candidate_without_reviewed_additions = [
        item
        for item in candidate_data
        if not (
            isinstance(item, dict)
            and isinstance(item.get("id"), int)
            and not isinstance(item.get("id"), bool)
            and item["id"] > 0
            and str(item["id"]) not in baseline_root_ids
            and str(item["id"]) in approved_candidate_root_ids
        )
    ]
    recovery_task_asset_ids = frozenset(
        asset_id
        for asset_id, row in asset_identity_oracle.items()
        if isinstance(row.get("provenance"), dict)
        and row["provenance"].get("kind") == "recovery_receipt"
    )
    bundle_member_task_asset_ids = frozenset(
        asset_id
        for asset_id, row in asset_identity_oracle.items()
        if isinstance(row.get("provenance"), dict)
        and isinstance(row["provenance"].get("bundle_member_receipt"), str)
        and bool(row["provenance"]["bundle_member_receipt"])
    )

    def project_pair_side(rows: list[object]) -> list[object]:
        projected = [
            project_asset_resource(
                item,
                normalize_approval=(
                    isinstance(item, dict)
                    and str(item.get("id"))
                    in approved_candidate_root_ids
                ),
                recovery_task_asset_ids=recovery_task_asset_ids,
                bundle_member_task_asset_ids=bundle_member_task_asset_ids,
            )
            for item in rows
        ]
        return sorted(
            projected,
            key=lambda item: (
                item.get("id", 0) if isinstance(item, dict) else 0,
                canonical(item),
            ),
        )

    return project_pair_side(baseline_data), project_pair_side(
        candidate_without_reviewed_additions
    )


def project_detail(value: object) -> object:
    data = unwrap_data(value)
    if not isinstance(data, dict):
        return data
    output: dict[str, object] = {}
    for field in (
        "assignee_id",
        "assignee_name",
        "creator_id",
        "creator_name",
        "current_handler_id",
        "current_handler_name",
        "designer_id",
        "designer_name",
        "reference_file_refs",
        "requester_id",
        "requester_name",
        "retouch_requirements",
        "sku_items",
        "task_detail",
    ):
        field_value = _drop_keys(
            data.get(field),
            frozenset(
                {
                    "set_mode_hint",
                    "product_channel",
                    *(
                        {"updated_at"}
                        if field == "sku_items"
                        else set()
                    ),
                }
            ),
        )
        if field == "retouch_requirements":
            field_value = project_nested_asset_versions(field_value)
        output[field] = {
            "present": field in data,
            "value": field_value,
        }
    output["events"] = {
        "present": "events" in data,
        "value": data.get("events"),
    }
    output["task"] = project_task(data.get("task"))
    modules = data.get("modules")
    task = data.get("task")
    terminal_task = (
        isinstance(task, dict) and task.get("task_status") == "Completed"
    )
    if isinstance(modules, list):
        output["modules"] = sorted(
            (
                _drop_keys(
                    item,
                    (
                        frozenset(
                            {
                                "state",
                                "terminal_at",
                                "updated_at",
                                "claimed_by",
                                "claimed_team_code",
                            }
                        )
                        if terminal_task
                        else frozenset()
                    ),
                )
                for item in modules
            ),
            key=lambda item: (
                str(item.get("module_key", "")) if isinstance(item, dict) else "",
                canonical(item),
            ),
        )
    else:
        output["modules"] = modules
    return output


def project_detail_pair(
    baseline_value: object, candidate_value: object
) -> tuple[object, object]:
    baseline_projection = project_detail(baseline_value)
    candidate_projection = project_detail(candidate_value)
    baseline_data = unwrap_data(baseline_value)
    candidate_data = unwrap_data(candidate_value)
    baseline_task = (
        baseline_data.get("task") if isinstance(baseline_data, dict) else None
    )
    candidate_task = (
        candidate_data.get("task") if isinstance(candidate_data, dict) else None
    )
    if (
        isinstance(baseline_projection, dict)
        and isinstance(candidate_projection, dict)
        and isinstance(baseline_task, dict)
        and isinstance(candidate_task, dict)
        and baseline_task.get("task_status") == "Completed"
        and candidate_task.get("task_status") == "InProgress"
        and baseline_task.get("task_type") == "retouch_task"
        and candidate_task.get("task_type") == "retouch_task"
    ):
        candidate_projection["modules"] = _drop_keys(
            candidate_projection.get("modules"),
            frozenset(
                {
                    "state",
                    "terminal_at",
                    "updated_at",
                    "claimed_by",
                    "claimed_team_code",
                }
            ),
        )
    if (
        isinstance(baseline_projection, dict)
        and isinstance(candidate_projection, dict)
        and isinstance(candidate_task, dict)
        and candidate_task.get("task_status") == "Completed"
    ):
        terminal_only_fields = frozenset(
            {
                "state",
                "terminal_at",
                "updated_at",
                "claimed_by",
                "claimed_team_code",
            }
        )
        baseline_projection["modules"] = _drop_keys(
            baseline_projection.get("modules"), terminal_only_fields
        )
        candidate_projection["modules"] = _drop_keys(
            candidate_projection.get("modules"), terminal_only_fields
        )
    return baseline_projection, candidate_projection


def expected_design_sub_status(
    task: object, modules: object
) -> str | None:
    if not isinstance(task, dict):
        return None
    task_type = task.get("task_type")
    task_status = task.get("task_status")
    if task_type not in {
        "original_product_development",
        "new_product_development",
        "retouch_task",
    }:
        return "not_required"
    by_status = {
        "PendingAssign": "pending_design",
        "PendingAudit": "pending_audit",
        "Blocked": "rework_required",
        "Completed": "final_ready",
        "Archived": "final_ready",
        "Cancelled": "not_required",
    }
    if task_status in by_status:
        return by_status[task_status]
    module_key = "retouch" if task_type == "retouch_task" else "design"
    if isinstance(modules, list):
        by_module_state = {
            "pending_claim": "pending_design",
            "in_progress": "in_progress",
            "submitted": "pending_audit",
            "closed": "completed",
            "completed": "completed",
        }
        for module in modules:
            if (
                isinstance(module, dict)
                and module.get("module_key") == module_key
                and module.get("state") in by_module_state
            ):
                return by_module_state[module["state"]]
    return "in_progress" if task_status == "InProgress" else "pending_design"


def detail_asset_versions(value: object) -> list[object] | None:
    data = unwrap_data(value)
    if not isinstance(data, dict) or not isinstance(data.get("asset_versions"), list):
        return None
    return [project_asset_version(item) for item in data["asset_versions"]]


def normalize_manifest_time(value: str) -> str:
    if not value:
        return ""
    normalized = value.replace(".000000", "").replace("+00:00", "Z")
    if re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}", normalized):
        # The reviewed mapping is exported from UTC MySQL DATETIME columns,
        # while the API serializes the same instant with an explicit Z.
        return normalized + "Z"
    return normalized


def is_rfc3339_datetime(value: object) -> bool:
    if not isinstance(value, str) or not value:
        return False
    candidate = value[:-1] + "+00:00" if value.endswith("Z") else value
    try:
        parsed = dt.datetime.fromisoformat(candidate)
    except ValueError:
        return False
    return parsed.tzinfo is not None


def allowed_actions_by_path(value: object, path: str = "") -> dict[str, frozenset[str]]:
    found: dict[str, frozenset[str]] = {}
    if isinstance(value, dict):
        for key, child in value.items():
            child_path = f"{path}/{key}"
            if key == "allowed_actions" and isinstance(child, list) and all(
                isinstance(item, str) for item in child
            ):
                found[child_path] = frozenset(child)
            found.update(allowed_actions_by_path(child, child_path))
    elif isinstance(value, list):
        for index, child in enumerate(value):
            found.update(allowed_actions_by_path(child, f"{path}/{index}"))
    return found


def expected_evidence_summary(
    reason: str, reviewed_reason_sha256: str
) -> dict[str, Any] | None:
    marker = re.search(r"\[migration_v2 ([^\]]+)\]$", reason)
    if marker is None:
        return None
    fields: dict[str, str] = {}
    for token in marker.group(1).split(" "):
        if "=" not in token:
            raise ValueError("migration_v2 reason marker token is invalid")
        key, value = token.split("=", 1)
        if not key or key in fields:
            raise ValueError("migration_v2 reason marker field is invalid")
        fields[key] = value
    required = {
        "manifest",
        "confidence",
        "confirmed_by",
        "confirmed_at",
        "evidence_count",
        "first_evidence",
    }
    if not required <= set(fields):
        raise ValueError("migration_v2 reason marker lacks required evidence")
    evidence_count = int(fields["evidence_count"])
    confirmed_by = int(fields["confirmed_by"])
    if evidence_count <= 0 or confirmed_by <= 0:
        raise ValueError("migration_v2 reason marker numeric field is invalid")
    business_reason = reason[: marker.start()].strip()
    output: dict[str, Any] = {
        "schema_version": "migration_v2",
        "manifest_sha256": fields["manifest"],
        "confidence": fields["confidence"],
        "confirmed_by": confirmed_by,
        "confirmed_at": normalize_manifest_time(fields["confirmed_at"]),
        "evidence_event_count": evidence_count,
        "evidence_event_ids": [fields["first_evidence"]],
        "evidence_event_ids_complete": evidence_count == 1,
        "upload_session_ids": [],
        "upload_sessions_known": False,
    }
    if business_reason:
        if sha256(business_reason.encode("utf-8")) != reviewed_reason_sha256:
            raise ValueError(
                "migration_v2 business reason differs from reviewed mapping"
            )
        output["business_reason"] = business_reason
    reason_hash = fields.get("reason_sha256")
    if reason_hash:
        if reason_hash != reviewed_reason_sha256:
            raise ValueError("migration_v2 reviewed reason hash differs")
        output["business_reason_sha256"] = reason_hash
    elif not business_reason and reviewed_reason_sha256 != sha256(b""):
        raise ValueError("migration_v2 omitted reviewed reason evidence")
    return output


def _tokens(path: str) -> list[str]:
    return [
        token.replace("~1", "/").replace("~0", "~")
        for token in path.lstrip("/").split("/")
    ]


def _apply_at(value: object, tokens: list[str], transform: Callable[[object], object]) -> tuple[object, int]:
    if not tokens:
        return transform(value), 1
    head, tail = tokens[0], tokens[1:]
    count = 0
    if isinstance(value, dict):
        output = dict(value)
        keys = sorted(output) if head == "*" else [head]
        for key in keys:
            if key not in output:
                continue
            if not tail and transform is _DELETE:
                del output[key]
                count += 1
            else:
                output[key], changed = _apply_at(output[key], tail, transform)
                count += changed
        return output, count
    if isinstance(value, list):
        output = list(value)
        indexes = range(len(output)) if head == "*" else ([int(head)] if head.isdigit() else [])
        for index in indexes:
            if index >= len(output):
                continue
            if not tail and transform is _DELETE:
                output[index] = _REMOVED
                count += 1
            else:
                output[index], changed = _apply_at(output[index], tail, transform)
                count += changed
        if transform is _DELETE:
            output = [item for item in output if item is not _REMOVED]
        return output, count
    return value, 0


def _DELETE(value: object) -> object:
    return _REMOVED


_REMOVED = object()


def apply_rule(
    left: object, right: object, rule: dict[str, Any]
) -> tuple[object, object]:
    for operation in rule["operations"]:
        path = _tokens(operation["path"])
        if operation["op"] == "remove":
            left, left_count = _apply_at(left, path, _DELETE)
            right, right_count = _apply_at(right, path, _DELETE)
            if left_count == 0 and right_count == 0:
                raise ValueError(
                    f"normalization rule {rule['rule_id']} remove path did not match"
                )
        else:
            expected_from, expected_to = operation["from"], operation["to"]

            def map_left(value: object) -> object:
                if value != expected_from:
                    raise ValueError(
                        f"normalization rule {rule['rule_id']} from value mismatch"
                    )
                return expected_to

            def verify_right(value: object) -> object:
                if value != expected_to:
                    raise ValueError(
                        f"normalization rule {rule['rule_id']} to value mismatch"
                    )
                return value

            left, left_count = _apply_at(left, path, map_left)
            right, right_count = _apply_at(right, path, verify_right)
            if left_count == 0 or right_count == 0:
                raise ValueError(
                    f"normalization rule {rule['rule_id']} map path did not match both sides"
                )
    return left, right


def history_items(value: object) -> list[dict[str, Any]]:
    pages = value if isinstance(value, list) else [value]
    items: list[dict[str, Any]] = []
    for page in pages:
        data = page.get("data", page) if isinstance(page, dict) else {}
        rows = data.get("items", []) if isinstance(data, dict) else []
        if isinstance(rows, list):
            items.extend(row for row in rows if isinstance(row, dict))
    return items


def revision_order_errors(value: object) -> list[str]:
    errors: list[str] = []
    for revision in history_items(value):
        revision_no = revision.get("revision_no", "?")
        for field in ("items", "references"):
            rows = revision.get(field, [])
            if rows is None:
                continue
            if not isinstance(rows, list):
                errors.append(f"revision {revision_no} {field} is not an array")
                continue
            sort_orders = [
                row.get("sort_order") for row in rows if isinstance(row, dict)
            ]
            if len(sort_orders) != len(rows) or not all(
                isinstance(item, int) for item in sort_orders
            ):
                errors.append(f"revision {revision_no} {field} lacks sort_order")
                continue
            if sort_orders != sorted(sort_orders) or any(
                right != left + 1
                for left, right in zip(sort_orders, sort_orders[1:])
            ):
                errors.append(f"revision {revision_no} {field} order is invalid")
    return errors


class Runner:
    def __init__(
        self,
        urls: dict[str, str],
        identities: list[dict[str, str]],
        resolved_headers: dict[str, dict[str, str]],
        rules: list[dict[str, Any]],
        retired_routes: list[str],
        expectations: dict[str, dict[str, Any]],
        api_oracle: dict[str, dict[str, Any]],
        requester: Callable[[str, str, dict[str, str]], HttpResult],
    ):
        self.urls = dict(urls)
        self.identities = [dict(identity) for identity in identities]
        self.identity_roles = {
            identity["id"]: identity["role"] for identity in self.identities
        }
        self.resolved_headers = {
            identity: dict(headers)
            for identity, headers in resolved_headers.items()
        }
        self.rules = rules
        self.retired_routes = frozenset(retired_routes)
        self.expectations = expectations
        self.task_oracle = api_oracle["tasks"]
        self.asset_identity_oracle = api_oracle["assets"]
        self.root_oracle = api_oracle["roots"]
        self.version_oracle = api_oracle["versions"]
        self.revision_role_oracle = api_oracle["revision_roles"]
        self.route_expectations = api_oracle["route_expectations"]
        self.revision_reason_oracle = api_oracle["revision_reasons"]
        if set(self.revision_reason_oracle) != set(
            self.expectations["revisions"]
        ):
            raise ValueError(
                "API oracle revision reasons do not cover reviewed revisions"
            )
        self.list_current_by_root: dict[str, dict[str, Any]] = {}
        self.list_approved_by_root: dict[str, dict[str, Any]] = {}
        self.migration_addition_root_ids: set[str] = set()
        for row in self.asset_identity_oracle.values():
            root_id = str(row.get("root_asset_id") or "")
            provenance = row.get("provenance")
            if (
                root_id
                and isinstance(provenance, dict)
                and provenance.get("kind")
                in {"bundle_receipt", "recovery_receipt"}
            ):
                self.migration_addition_root_ids.add(root_id)
            if row["list_current_version"]:
                if not root_id or root_id in self.list_current_by_root:
                    raise ValueError("API oracle current asset root is invalid")
                self.list_current_by_root[root_id] = row
            if row["list_approved_version"]:
                if not root_id or root_id in self.list_approved_by_root:
                    raise ValueError("API oracle approved asset root is invalid")
                self.list_approved_by_root[root_id] = row
        for root_id, approved in self.list_approved_by_root.items():
            current = self.list_current_by_root.get(root_id)
            if current is None or any(
                approved[field] != current[field]
                for field in (
                    "task_id",
                    "root_asset_id",
                    "root_asset_type",
                    "root_scope_sku_code",
                    "root_retouch_requirement_id",
                )
            ):
                raise ValueError("API oracle approved asset root differs")
        self.requester = requester
        self.observations: list[dict[str, Any]] = []
        self.results: dict[tuple[str, str, str, str], HttpResult] = {}
        self.violations: list[dict[str, str]] = []
        self.used_rules: set[str] = set()
        self.used_rule_applications: set[
            tuple[str, str | None, str, str, str, int, int]
        ] = set()
        self._state_lock = threading.RLock()
        self._requests: dict[tuple[str, str, str], Future[HttpResult]] = {}
        self.logical_request_count = 0
        self.physical_request_count = 0
        self.manifest_oracle_check_count = 0
        self.semantic_comparison_count = 0
        self.governed_assets: dict[tuple[str, str], set[str]] = {}
        self.task_asset_metadata: dict[
            tuple[str, str, str], dict[str, Any]
        ] = {}
        self.required_oracle_identities = frozenset(
            identity["id"]
            for identity in identities
            if identity.get("role") == "admin"
        )
        if not self.required_oracle_identities:
            raise ValueError("G6 requires at least one admin oracle identity")
        self.oracle_coverage: dict[
            tuple[str, str, str], set[str]
        ] = {}
        self.view_only_task_statuses: dict[
            str, dict[int, set[str]]
        ] = {
            combination: {200: set(), 403: set()}
            for combination in COMBINATION_IDS
            if combination.endswith("_b")
        }

    def cover(
        self, combination: str, identity: str, kind: str, locator: str
    ) -> None:
        with self._state_lock:
            self.oracle_coverage.setdefault(
                (combination, identity, kind), set()
            ).add(locator)

    def validate_oracle_coverage(self) -> None:
        expected = {
            "task": set(self.expectations["tasks"]),
            "detail": set(self.expectations["tasks"]),
            "bundle": set(self.expectations["tasks"]),
            "group": set(self.expectations["groups"]),
            "revision": set(self.expectations["revisions"]),
            "detail_asset": {
                asset_id
                for asset_id, row in self.asset_identity_oracle.items()
                if row["detail_visible"]
            },
            "list_asset": {
                asset_id
                for asset_id, row in self.asset_identity_oracle.items()
                if row["list_current_version"]
            },
            "list_approved_asset": {
                asset_id
                for asset_id, row in self.asset_identity_oracle.items()
                if row["list_approved_version"]
            },
        }
        for combination in ("dev_dev_b", "external_dev_b"):
            for identity in sorted(self.required_oracle_identities):
                for kind, expected_locators in expected.items():
                    actual = self.oracle_coverage.get(
                        (combination, identity, kind), set()
                    )
                    if actual != expected_locators:
                        self.oracle_violation(
                            f"coverage:{kind}",
                            f"{combination}/{identity} coverage "
                            f"{sha256(canonical(sorted(actual)))}!="
                            f"{sha256(canonical(sorted(expected_locators)))}",
                        )

    def validate_identity_coverage(self) -> None:
        for combination, statuses in sorted(
            self.view_only_task_statuses.items()
        ):
            for status in (200, 403):
                if not statuses[status]:
                    self.violation(
                        "api.view_only_scope_unproven",
                        f"{combination}:view_only",
                        f"no view_only task returned {status}",
                    )
        for combination in sorted(self.view_only_task_statuses):
            for identity, role in sorted(self.identity_roles.items()):
                if role != "view_only":
                    continue
                for task_id in sorted(self.expectations["tasks"], key=int):
                    primary_entity = f"task:{task_id}:{CORE_ROUTES[0]}"
                    primary = self.results.get(
                        (
                            combination,
                            identity,
                            CORE_ROUTES[0],
                            primary_entity,
                        )
                    )
                    if primary is None or primary.status != 403:
                        continue
                    for route in CORE_ROUTES[1:]:
                        entity = f"task:{task_id}:{route}"
                        sibling = self.results.get(
                            (combination, identity, route, entity)
                        )
                        if sibling is not None and sibling.status == 200:
                            self.violation(
                                "api.view_only_scope_bypass",
                                entity,
                                f"{combination}/{identity} primary task is 403 "
                                f"but {route} returned 200",
                            )

    @staticmethod
    def _scope_ref(group: dict[str, Any]) -> str:
        scope = str(group.get("scope_kind", ""))
        key = {
            "task": None,
            "sku": "task_sku_item_id",
            "retouch_requirement": "retouch_requirement_id",
        }.get(scope)
        if scope == "task":
            return "0"
        value = group.get(key) if key else None
        return str(value) if isinstance(value, int) and value > 0 else ""

    def oracle_violation(self, entity: str, detail: str) -> None:
        self.violation("api.manifest_oracle_mismatch", entity, detail)

    @staticmethod
    def _positive_int(value: object) -> bool:
        return isinstance(value, int) and not isinstance(value, bool) and value > 0

    @staticmethod
    def _nonnegative_int(value: object) -> bool:
        return isinstance(value, int) and not isinstance(value, bool) and value >= 0

    def validate_revision_contract_shape(
        self,
        combination: str,
        identity: str,
        entity: str,
        revision: object,
        *,
        historical: bool = False,
    ) -> None:
        if not isinstance(revision, dict):
            self.oracle_violation(
                entity, f"{combination}/{identity} revision is not an object"
            )
            return
        required = {
            "id",
            "group_id",
            "revision_no",
            "status",
            "mode",
            "source_stage",
            "created_by",
            "legacy_migration",
            "items",
            "references",
            "created_at",
        }
        missing = sorted(required - revision.keys())
        if missing:
            self.oracle_violation(
                entity,
                f"{combination}/{identity} revision misses required fields "
                f"{','.join(missing)}",
            )
        for field in ("id", "group_id", "revision_no", "created_by"):
            if field in revision and not self._positive_int(revision[field]):
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} revision {field} is invalid",
                )
        if revision.get("status") not in {
            "draft",
            "submitted",
            "finalized",
            "rejected",
            "superseded",
        }:
            self.oracle_violation(
                entity, f"{combination}/{identity} revision status is invalid"
            )
        if revision.get("mode") not in {"single", "set"}:
            self.oracle_violation(
                entity, f"{combination}/{identity} revision mode is invalid"
            )
        if revision.get("source_stage") not in {
            "design",
            "audit",
            "retouch",
            "migration",
            "reopen",
        }:
            self.oracle_violation(
                entity, f"{combination}/{identity} revision source_stage is invalid"
            )
        if not isinstance(revision.get("legacy_migration"), bool):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} revision legacy_migration is invalid",
            )
        if not isinstance(revision.get("created_at"), str) or not revision.get(
            "created_at"
        ):
            self.oracle_violation(
                entity, f"{combination}/{identity} revision created_at is invalid"
            )
        revision_id = revision.get("id")
        source_id = revision.get("source_task_asset_id")
        source_file = revision.get("source_file")
        if source_file is not None:
            if (
                not isinstance(source_file, dict)
                or not self._positive_int(source_file.get("task_asset_id"))
                or not isinstance(source_file.get("file_name"), str)
                or not source_file.get("file_name")
            ):
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} revision source_file is invalid",
                )
            elif source_file["task_asset_id"] != source_id:
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} revision source_file task_asset_id differs",
                )
            if isinstance(source_file, dict):
                self.validate_revision_file_access(
                    combination,
                    identity,
                    entity,
                    "source_file",
                    source_file,
                )
        items = revision.get("items")
        if not isinstance(items, list):
            self.oracle_violation(
                entity, f"{combination}/{identity} revision items is invalid"
            )
        else:
            for index, item in enumerate(items):
                item_required = {"id", "revision_id", "task_asset_id", "sort_order"}
                if not isinstance(item, dict):
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} revision item {index} is invalid",
                    )
                    continue
                missing_item = sorted(item_required - item.keys())
                if missing_item:
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} revision item {index} misses "
                        f"{','.join(missing_item)}",
                    )
                for field in ("id", "revision_id", "task_asset_id"):
                    if field in item and not self._positive_int(item[field]):
                        self.oracle_violation(
                            entity,
                            f"{combination}/{identity} revision item {index} "
                            f"{field} is invalid",
                        )
                if "sort_order" in item and not self._nonnegative_int(
                    item["sort_order"]
                ):
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} revision item {index} "
                        "sort_order is invalid",
                    )
                if (
                    self._positive_int(revision_id)
                    and self._positive_int(item.get("revision_id"))
                    and item["revision_id"] != revision_id
                ):
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} revision item {index} "
                        "revision_id differs",
                    )
                file_row = item.get("file")
                if file_row is not None:
                    if (
                        not isinstance(file_row, dict)
                        or not self._positive_int(file_row.get("task_asset_id"))
                        or not isinstance(file_row.get("file_name"), str)
                        or not file_row.get("file_name")
                    ):
                        self.oracle_violation(
                            entity,
                            f"{combination}/{identity} revision item {index} "
                            "file is invalid",
                        )
                    elif file_row["task_asset_id"] != item.get("task_asset_id"):
                        self.oracle_violation(
                            entity,
                            f"{combination}/{identity} revision item {index} "
                            "file task_asset_id differs",
                        )
                    revision_item_id = file_row.get("revision_item_id")
                    if (
                        revision_item_id is not None
                        and revision_item_id != item.get("id")
                    ):
                        self.oracle_violation(
                            entity,
                            f"{combination}/{identity} revision item {index} "
                            "file revision_item_id differs",
                        )
                    if isinstance(file_row, dict):
                        self.validate_revision_file_access(
                            combination,
                            identity,
                            entity,
                            f"item[{index}].file",
                            file_row,
                        )
        references = revision.get("references")
        if not isinstance(references, list):
            self.oracle_violation(
                entity, f"{combination}/{identity} revision references is invalid"
            )
        else:
            for index, reference in enumerate(references):
                reference_required = {
                    "id",
                    "revision_id",
                    "reference_file_ref_id",
                    "sort_order",
                    "ref_id",
                    "created_at",
                }
                if not isinstance(reference, dict):
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} revision reference {index} is invalid",
                    )
                    continue
                missing_reference = sorted(reference_required - reference.keys())
                if missing_reference:
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} revision reference {index} misses "
                        f"{','.join(missing_reference)}",
                    )
                for field in ("id", "revision_id", "reference_file_ref_id"):
                    if field in reference and not self._positive_int(reference[field]):
                        self.oracle_violation(
                            entity,
                            f"{combination}/{identity} revision reference {index} "
                            f"{field} is invalid",
                        )
                if "sort_order" in reference and not self._nonnegative_int(
                    reference["sort_order"]
                ):
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} revision reference {index} "
                        "sort_order is invalid",
                    )
                if not isinstance(reference.get("ref_id"), str) or not reference.get(
                    "ref_id"
                ):
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} revision reference {index} "
                        "ref_id is invalid",
                    )
                if not isinstance(reference.get("created_at"), str) or not reference.get(
                    "created_at"
                ):
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} revision reference {index} "
                        "created_at is invalid",
                    )
                if (
                    self._positive_int(revision_id)
                    and self._positive_int(reference.get("revision_id"))
                    and reference["revision_id"] != revision_id
                ):
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} revision reference {index} "
                        "revision_id differs",
                    )
                self.validate_revision_file_access(
                    combination,
                    identity,
                    entity,
                    f"reference[{index}]",
                    reference,
                    controlled_urls=(
                        "required"
                        if self._positive_int(
                            reference.get("formal_task_asset_id")
                        )
                        else ("forbidden" if historical else "optional")
                    ),
                )

    def validate_revision_file_access(
        self,
        combination: str,
        identity: str,
        entity: str,
        label: str,
        file_row: dict[str, Any],
        *,
        controlled_urls: str = "required",
    ) -> None:
        if controlled_urls not in {"required", "forbidden", "optional"}:
            raise ValueError("controlled_urls mode is invalid")
        availability = file_row.get("availability")
        unavailable_reason = file_row.get("unavailable_reason")
        preview_url = file_row.get("preview_url")
        download_url = file_row.get("download_url")
        if availability not in {"available", "historical_unavailable"}:
            self.oracle_violation(
                entity,
                f"{combination}/{identity} {label} availability is invalid",
            )
            return
        for field, value in (
            ("unavailable_reason", unavailable_reason),
            ("preview_url", preview_url),
            ("download_url", download_url),
        ):
            if value is not None and not isinstance(value, str):
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} {label} {field} is invalid",
                )
                return
        if availability == "historical_unavailable":
            if not isinstance(unavailable_reason, str) or not unavailable_reason:
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} {label} unavailable_reason is required",
                )
            if preview_url or download_url:
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} {label} unavailable file exposes URL",
                )
            return
        if unavailable_reason:
            self.oracle_violation(
                entity,
                f"{combination}/{identity} {label} available file has unavailable_reason",
            )
        if controlled_urls == "forbidden":
            if preview_url or download_url:
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} {label} without formal task asset "
                    "exposes controlled URL",
                )
            return
        role = self.identity_roles.get(identity)
        if role == "admin" and controlled_urls == "required":
            if not preview_url or not download_url:
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} {label} available admin file "
                    "lacks preview or download URL",
                )
        elif role == "view_only" and download_url:
            self.oracle_violation(
                entity,
                f"{combination}/{identity} {label} view_only file exposes download URL",
            )

    @staticmethod
    def revision_link_projection(revision: dict[str, Any]) -> dict[str, Any]:
        source_file = revision.get("source_file")
        raw_items = revision.get("items")
        raw_references = revision.get("references")
        return {
            "id": revision.get("id"),
            "group_id": revision.get("group_id"),
            "revision_no": revision.get("revision_no"),
            "status": revision.get("status"),
            "mode": revision.get("mode"),
            "source_task_asset_id": revision.get("source_task_asset_id"),
            "source_file_task_asset_id": (
                source_file.get("task_asset_id")
                if isinstance(source_file, dict)
                else None
            ),
            "source_stage": revision.get("source_stage"),
            "created_by": revision.get("created_by"),
            "reason": revision.get("reason"),
            "legacy_migration": revision.get("legacy_migration"),
            "items": (
                [
                    {
                        "id": item.get("id"),
                        "revision_id": item.get("revision_id"),
                        "task_asset_id": item.get("task_asset_id"),
                        "sort_order": item.get("sort_order"),
                        "file_task_asset_id": (
                            item["file"].get("task_asset_id")
                            if isinstance(item.get("file"), dict)
                            else None
                        ),
                    }
                    for item in raw_items
                    if isinstance(item, dict)
                ]
                if isinstance(raw_items, list)
                else {"invalid_type": type(raw_items).__name__}
            ),
            "references": (
                [
                    {
                        "id": reference.get("id"),
                        "revision_id": reference.get("revision_id"),
                        "reference_file_ref_id": reference.get(
                            "reference_file_ref_id"
                        ),
                        "formal_task_asset_id": reference.get(
                            "formal_task_asset_id"
                        ),
                        "sort_order": reference.get("sort_order"),
                        "ref_id": reference.get("ref_id"),
                    }
                    for reference in raw_references
                    if isinstance(reference, dict)
                ]
                if isinstance(raw_references, list)
                else {"invalid_type": type(raw_references).__name__}
            ),
        }

    @staticmethod
    def storage_ref_from_stable_locator(value: object) -> str:
        parts = str(value or "").split(":", 2)
        return parts[2] if len(parts) == 3 and parts[0] == "asset" else ""

    def validate_version_identity(
        self,
        combination: str,
        identity: str,
        entity: str,
        version: object,
        *,
        surface: str,
        coverage_kind: str | None = None,
    ) -> dict[str, Any] | None:
        if not isinstance(version, dict):
            self.oracle_violation(
                entity, f"{combination}/{identity} {surface} asset is invalid"
            )
            return None
        asset_id = version.get("id")
        expected = self.asset_identity_oracle.get(str(asset_id))
        if not self._positive_int(asset_id) or expected is None:
            self.oracle_violation(
                entity,
                f"{combination}/{identity} {surface} asset {asset_id} "
                "is absent from the v2 oracle",
            )
            return None
        raw_scope_value = version.get("scope_sku_code")
        if raw_scope_value is not None and not isinstance(
            raw_scope_value, str
        ):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} {surface} asset {asset_id} "
                "scope_sku_code is not nullable text",
            )
            return None
        scope_value = str(raw_scope_value or "")
        retouch_value = version.get("retouch_requirement_id")
        if retouch_value is not None and not self._positive_int(
            retouch_value
        ):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} {surface} asset {asset_id} "
                "retouch_requirement_id is not a positive integer",
            )
            return None
        storage_key = version.get("storage_key")
        raw_file_hash = version.get("file_hash")
        if raw_file_hash is not None and not isinstance(raw_file_hash, str):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} {surface} asset {asset_id} "
                "file_hash is not nullable text",
            )
            return None
        file_hash = str(raw_file_hash or "")
        if file_hash and not SHA256_RE.fullmatch(file_hash):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} {surface} asset {asset_id} "
                "file_hash is not SHA-256",
            )
            return None
        file_size = version.get("file_size")
        mime_type = version.get("mime_type")
        approved_at = version.get("approved_at")
        if approved_at is not None and (
            not isinstance(approved_at, str)
            or not is_rfc3339_datetime(approved_at)
        ):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} {surface} asset {asset_id} "
                "approved_at is not nullable RFC3339",
            )
            return None
        approved_by = version.get("approved_by")
        if approved_by is not None and not self._positive_int(approved_by):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} {surface} asset {asset_id} "
                "approved_by is not a positive integer",
            )
            return None
        historical_unavailable = (
            expected.get("content_availability")
            == "historical_unavailable"
        )
        if historical_unavailable and (
            storage_key
            or file_hash
            or version.get("download_url")
            or version.get("public_download_allowed") is True
        ):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} {surface} asset {asset_id} "
                "historical-unavailable metadata exposes file access",
            )
            return None
        if (
            not self._positive_int(version.get("task_id"))
            or not self._positive_int(version.get("asset_id"))
            or not isinstance(version.get("asset_type"), str)
            or not isinstance(storage_key, str)
            or not self._nonnegative_int(file_size)
            or not isinstance(mime_type, str)
            or not isinstance(version.get("flow_review_status"), str)
            or not isinstance(version.get("usable_state"), str)
        ):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} {surface} asset {asset_id} "
                "identity fields have invalid types",
            )
            return None
        if surface == "list":
            expected_type = expected["root_asset_type"]
        else:
            expected_type = expected["intrinsic_asset_type"]
        expected_scope = effective_oracle_scope_sku_code(expected)
        expected_retouch = effective_oracle_retouch_requirement_id(expected)
        actual = {
            "task_asset_id": asset_id,
            "task_id": version.get("task_id"),
            "root_asset_id": version.get("asset_id"),
            "asset_type": version.get("asset_type"),
            "scope_sku_code": scope_value,
            "retouch_requirement_id": retouch_value,
            "object_key_sha256": (
                sha256(storage_key.encode("utf-8"))
                if isinstance(storage_key, str)
                else None
            ),
            "content_sha256": file_hash,
            "size": file_size,
            "mime_type": mime_type,
            "flow_review_status": str(
                version.get("flow_review_status") or ""
            ),
            "usable_state": version.get("usable_state"),
            "approved_at": (
                normalize_manifest_time(str(approved_at or ""))
                or None
            ),
            "approved_by": approved_by,
        }
        expected_projection = {
            "task_asset_id": expected["task_asset_id"],
            "task_id": expected["task_id"],
            "root_asset_id": expected["root_asset_id"],
            "asset_type": expected_type,
            "scope_sku_code": expected_scope,
            "retouch_requirement_id": expected_retouch,
            "object_key_sha256": expected["object_key_sha256"],
            "content_sha256": expected["content_sha256"],
            "size": expected["size"],
            "mime_type": expected["mime_type"],
            "flow_review_status": expected["flow_review_status"],
            "usable_state": expected_asset_usable_state(expected),
            "approved_at": (
                normalize_manifest_time(str(expected["approved_at"] or ""))
                or None
            ),
            "approved_by": expected["approved_by"],
        }
        if historical_unavailable:
            # The frozen oracle retains the legacy object-key hash as evidence,
            # while the V8 read model deliberately suppresses the unusable raw
            # key. The access check above proves that suppression before the
            # comparison substitutes the deterministic empty-key projection.
            expected_projection["object_key_sha256"] = sha256(b"")
            expected_projection["content_sha256"] = ""
        if actual != expected_projection:
            self.oracle_violation(
                entity,
                f"{combination}/{identity} {surface} asset {asset_id} "
                f"identity differs {sha256(canonical(actual))}!="
                f"{sha256(canonical(expected_projection))}",
            )
            return None
        if coverage_kind is not None:
            self.cover(
                combination, identity, coverage_kind, str(asset_id)
            )
        key = (combination, identity, str(asset_id))
        with self._state_lock:
            prior = self.task_asset_metadata.get(key)
            if prior is not None and any(
                prior.get(field) != version.get(field)
                for field in (
                    "task_id",
                    "asset_id",
                    "asset_type",
                    "scope_sku_code",
                    "retouch_requirement_id",
                    "storage_key",
                    "file_hash",
                    "file_size",
                    "mime_type",
                    "flow_review_status",
                    "approved_at",
                    "approved_by",
                )
            ):
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} asset {asset_id} "
                    "has inconsistent nested identities",
                )
            else:
                self.task_asset_metadata[key] = dict(version)
        return expected

    @staticmethod
    def _download_url_matches_object_key(
        download_url: str, expected_object_key_sha256: str
    ) -> bool:
        if not SHA256_RE.fullmatch(expected_object_key_sha256):
            return False
        parsed = urllib.parse.urlsplit(download_url)
        decoded_path = urllib.parse.unquote(parsed.path)
        proxy_prefix = "/v1/assets/files/"
        candidates: set[str] = set()
        if decoded_path.startswith(proxy_prefix):
            candidates.add(decoded_path[len(proxy_prefix) :].lstrip("/"))
        else:
            parts = [part for part in decoded_path.split("/") if part]
            candidates.update(
                "/".join(parts[index:]) for index in range(len(parts))
            )
        return any(
            candidate
            and sha256(candidate.encode("utf-8"))
            == expected_object_key_sha256
            for candidate in candidates
        )

    def validate_task_asset_access_oracle(
        self,
        combination: str,
        identity: str,
        requested_asset_id: str,
        route: str,
        body: object,
    ) -> None:
        if not combination.endswith("_b"):
            return
        entity = f"task-asset:{requested_asset_id}"
        expected = self.asset_identity_oracle.get(requested_asset_id)
        data = unwrap_data(body)
        with self._state_lock:
            self.manifest_oracle_check_count += 1
        if not isinstance(expected, dict) or not isinstance(data, dict):
            self.violation(
                "api.task_asset_metadata_mismatch",
                entity,
                f"{combination}/{identity} {route} lacks requested asset metadata",
            )
            return
        download_mode = data.get("download_mode")
        download_url = data.get("download_url")
        access_hint = data.get("access_hint")
        preview_available = data.get("preview_available")
        filename = data.get("filename")
        file_size = data.get("file_size")
        mime_type = data.get("mime_type")
        expires_at = data.get("expires_at")
        expected_mime_type = str(expected.get("mime_type") or "")
        mime_matches = mime_type == expected_mime_type
        if (
            route == "/v1/task-assets/{task_asset_id}/preview"
            and expected_mime_type.startswith("image/")
            and mime_type == "image/jpeg"
        ):
            mime_matches = True
        known = self.task_asset_metadata.get(
            (combination, identity, requested_asset_id), {}
        )
        expected_filename = ""
        if known.get("has_original_filename") is True:
            expected_filename = str(known.get("original_filename") or "")
        if not expected_filename:
            expected_filename = str(
                known.get("file_name")
                or known.get("original_filename")
                or ""
            )
        is_preview = route == "/v1/task-assets/{task_asset_id}/preview"
        valid = (
            download_mode in {"direct", "proxy"}
            and isinstance(download_url, str)
            and bool(download_url)
            and isinstance(access_hint, str)
            and isinstance(preview_available, bool)
            and isinstance(filename, str)
            and bool(filename)
            and self._nonnegative_int(file_size)
            and isinstance(mime_type, str)
            and bool(mime_type)
            and (
                expires_at is None
                or (
                    isinstance(expires_at, str)
                    and is_rfc3339_datetime(expires_at)
                )
            )
            and (
                "items" not in data
                or data.get("items") is None
                or data.get("items") == []
            )
        )
        if is_preview:
            # Preview metadata describes the exact-version derivative selected
            # by the controlled task-asset preview endpoint. Its filename,
            # size, and MIME type are not the immutable source-file metadata.
            valid = valid and preview_available is True
        else:
            valid = (
                valid
                and file_size == expected["size"]
                and mime_matches
                and (
                    not expected_filename
                    or filename == expected_filename
                )
            )
        if (
            valid
            and route == "/v1/task-assets/{task_asset_id}/download"
            and not self._download_url_matches_object_key(
                download_url, expected["object_key_sha256"]
            )
        ):
            valid = False
        if not valid:
            actual_projection = {
                "download_mode": download_mode,
                "download_url_has_value": (
                    isinstance(download_url, str) and bool(download_url)
                ),
                "access_hint_type": type(access_hint).__name__,
                "preview_available": preview_available,
                "filename": filename,
                "file_size": file_size,
                "mime_type": mime_type,
                "expires_at_valid": (
                    expires_at is None
                    or (
                        isinstance(expires_at, str)
                        and is_rfc3339_datetime(expires_at)
                    )
                ),
            }
            self.violation(
                "api.task_asset_metadata_mismatch",
                entity,
                f"{combination}/{identity} {route} requested asset "
                f"metadata differs {sha256(canonical(actual_projection))}",
            )

    @staticmethod
    def nested_design_asset_versions(value: object) -> list[dict[str, Any]]:
        found: list[dict[str, Any]] = []
        if isinstance(value, dict):
            if (
                isinstance(value.get("id"), int)
                and not isinstance(value.get("id"), bool)
                and isinstance(value.get("task_id"), int)
                and not isinstance(value.get("task_id"), bool)
                and isinstance(value.get("asset_id"), int)
                and not isinstance(value.get("asset_id"), bool)
                and isinstance(value.get("version_no"), int)
                and not isinstance(value.get("version_no"), bool)
                and isinstance(value.get("asset_type"), str)
            ):
                found.append(value)
            for child in value.values():
                found.extend(Runner.nested_design_asset_versions(child))
        elif isinstance(value, list):
            for child in value:
                found.extend(Runner.nested_design_asset_versions(child))
        return found

    def validate_task_oracle(
        self, combination: str, identity: str, task_id: str, route: str, body: object
    ) -> None:
        if not combination.endswith("_b"):
            return
        if route == "/v1/tasks/{task_id}/assets":
            self.validate_asset_list_oracle(
                combination, identity, task_id, body
            )
            return
        if route not in {
            "/v1/tasks/{task_id}",
            "/v1/tasks/{task_id}/detail",
        }:
            return
        detail_data = unwrap_data(body)
        data = detail_data
        if route.endswith("/detail"):
            data = data.get("task") if isinstance(data, dict) else None
        expected = self.expectations["tasks"].get(task_id)
        entity = f"task:{task_id}:{route}"
        with self._state_lock:
            self.manifest_oracle_check_count += 1
        if not isinstance(data, dict) or not isinstance(expected, dict):
            self.oracle_violation(
                entity, f"{combination}/{identity} lacks task oracle payload"
            )
            return
        self.cover(
            combination,
            identity,
            "detail" if route.endswith("/detail") else "task",
            task_id,
        )
        actual = {
            "task_id": str(data.get("id", "")),
            "task_type": str(data.get("task_type", "")),
            "task_status": str(data.get("task_status", "")),
            "current_handler_id": str(data.get("current_handler_id", "")),
            "workflow_revision": str(data.get("workflow_revision", "")),
        }
        oracle = {
            key: expected[key]
            for key in (
                "task_id",
                "task_type",
                "task_status",
                "current_handler_id",
                "workflow_revision",
            )
        }
        if actual != oracle:
            self.oracle_violation(
                entity,
                f"{combination}/{identity} task projection "
                f"{sha256(canonical(actual))}!={sha256(canonical(oracle))}",
            )
        if data.get("workflow_contract_version") != 2:
            self.oracle_violation(
                entity,
                f"{combination}/{identity} workflow_contract_version is not 2",
            )
        if (
            route.endswith("/detail")
            and isinstance(detail_data, dict)
            and detail_data.get("current_handler_id")
            != data.get("current_handler_id")
        ):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} detail current_handler_id "
                "differs from nested task",
            )
        task_row = self.task_oracle.get(task_id)
        task_oracle = (
            {
                "task_id": task_row["task_id"],
                "owner_department_id": task_row["owner_department_id"],
                "owner_team_id": task_row["owner_team_id"],
            }
            if isinstance(task_row, dict)
            else None
        )
        actual_org = {
            "task_id": data.get("id"),
            "owner_department_id": data.get("owner_department_id"),
            "owner_team_id": data.get("owner_team_id"),
        }
        if task_oracle != actual_org:
            self.oracle_violation(
                entity,
                f"{combination}/{identity} task organization projection "
                f"{sha256(canonical(actual_org))}!="
                f"{sha256(canonical(task_oracle))}",
            )
        actions = data.get("allowed_actions")
        if route == "/v1/tasks/{task_id}" or actions is not None:
            if not isinstance(actions, list) or not all(
                isinstance(item, str) and item for item in actions
            ) or len(actions) != len(set(actions)):
                self.oracle_violation(
                    entity, f"{combination}/{identity} allowed_actions is invalid"
                )
        if route.endswith("/detail"):
            detail = unwrap_data(body)
            modules = (
                detail.get("modules") if isinstance(detail, dict) else None
            )
            # Keep this aligned with domain.ModuleState.Terminal and the
            # migration/SQL gate contract. Retired legacy modules may remain
            # closed or forcibly closed as immutable history; migration only
            # normalizes modules that were still open when a task became
            # Completed.
            terminal_module_states = TERMINAL_MODULE_STATES
            active_module_states = ACTIVE_MODULE_STATES
            tolerated_rejected_states = {"rejected"}
            if isinstance(modules, list):
                for module in modules:
                    module_id = (
                        module.get("id")
                        if isinstance(module, dict)
                        else None
                    )
                    state = (
                        module.get("state")
                        if isinstance(module, dict)
                        else None
                    )
                    terminal_at = (
                        module.get("terminal_at")
                        if isinstance(module, dict)
                        else None
                    )
                    if (
                        not isinstance(module, dict)
                        or state
                        not in (
                            terminal_module_states
                            | active_module_states
                            | tolerated_rejected_states
                        )
                        or not is_rfc3339_datetime(module.get("updated_at"))
                        or (
                            state in terminal_module_states
                            and not is_rfc3339_datetime(terminal_at)
                        )
                        or (
                            state in active_module_states
                            and terminal_at is not None
                        )
                        or (
                            state in tolerated_rejected_states
                            and terminal_at is not None
                            and not is_rfc3339_datetime(terminal_at)
                        )
                    ):
                        self.oracle_violation(
                            entity,
                            f"{combination}/{identity} task module "
                            f"{module_id} violates lifecycle invariant",
                        )
            derived_sub_status = expected_design_sub_status(data, modules)
            if (
                not isinstance(detail, dict)
                or detail.get("design_sub_status") != derived_sub_status
            ):
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} design_sub_status is not "
                    "the V8 task/module derivation",
                )
            if (
                expected["task_type"] == "retouch_task"
                and expected["task_status"] == "InProgress"
            ):
                retouch_modules = [
                    module
                    for module in modules or []
                    if isinstance(module, dict)
                    and module.get("module_key") == "retouch"
                ]
                if (
                    len(retouch_modules) != 1
                    or retouch_modules[0].get("state") != "in_progress"
                    or retouch_modules[0].get("terminal_at") is not None
                ):
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} reopened retouch task "
                        "does not expose one active non-terminal retouch module",
                    )
            if expected["task_status"] == "Completed":
                if not isinstance(modules, list):
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} completed task modules "
                        "are invalid",
                    )
                else:
                    for module in modules:
                        if (
                            not isinstance(module, dict)
                            or module.get("state")
                            not in terminal_module_states
                            or not is_rfc3339_datetime(module.get("terminal_at"))
                            or not is_rfc3339_datetime(module.get("updated_at"))
                        ):
                            module_id = (
                                module.get("id")
                                if isinstance(module, dict)
                                else None
                            )
                            self.oracle_violation(
                                entity,
                                f"{combination}/{identity} completed task "
                                f"module {module_id} violates terminal "
                                "migration invariant",
                            )
            sku_surfaces = []
            if isinstance(data.get("sku_items"), list):
                sku_surfaces.append(data["sku_items"])
            if (
                isinstance(detail_data, dict)
                and detail_data is not data
                and isinstance(detail_data.get("sku_items"), list)
            ):
                sku_surfaces.append(detail_data["sku_items"])
            for sku_items in sku_surfaces:
                for sku_item in sku_items:
                    if (
                        isinstance(sku_item, dict)
                        and "updated_at" in sku_item
                        and not is_rfc3339_datetime(sku_item["updated_at"])
                    ):
                        self.oracle_violation(
                            entity,
                            f"{combination}/{identity} sku item "
                            f"{sku_item.get('id')} updated_at is not RFC3339",
                        )
            versions = (
                detail.get("asset_versions") if isinstance(detail, dict) else None
            )
            if not isinstance(versions, list):
                self.oracle_violation(
                    entity, f"{combination}/{identity} asset_versions is invalid"
                )
            else:
                seen_versions: set[int] = set()
                for version in versions:
                    asset_id = version.get("id") if isinstance(version, dict) else None
                    if not self._positive_int(asset_id):
                        self.oracle_violation(
                            entity,
                            f"{combination}/{identity} asset version identity is invalid",
                        )
                        continue
                    if asset_id in seen_versions:
                        self.oracle_violation(
                            entity,
                            f"{combination}/{identity} asset version "
                            f"{asset_id} is duplicated",
                        )
                        continue
                    seen_versions.add(asset_id)
                    if "warehouse_ready" in version:
                        self.oracle_violation(
                            entity,
                            f"{combination}/{identity} asset {asset_id} "
                            "resurrects retired warehouse_ready",
                        )
                    if (
                        "current_version_role" in version
                        and version.get("current_version_role")
                        not in CURRENT_VERSION_ROLES
                    ):
                        self.oracle_violation(
                            entity,
                            f"{combination}/{identity} asset {asset_id} "
                            "current_version_role is invalid",
                        )
                    identity_row = self.validate_version_identity(
                        combination,
                        identity,
                        entity,
                        version,
                        surface="detail",
                        coverage_kind="detail_asset",
                    )
                    if (
                        identity_row is not None
                        and not identity_row["detail_visible"]
                    ):
                        self.oracle_violation(
                            entity,
                            f"{combination}/{identity} asset {asset_id} "
                            "is not detail-visible in the v2 oracle",
                        )
            nested_versions = self.nested_design_asset_versions(
                detail.get("retouch_requirements")
                if isinstance(detail, dict)
                else None
            )
            seen_nested: set[int] = set()
            for nested in nested_versions:
                asset_id = nested["id"]
                if asset_id in seen_nested:
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} nested retouch asset "
                        f"{asset_id} is duplicated",
                    )
                    continue
                seen_nested.add(asset_id)
                self.validate_version_identity(
                    combination,
                    identity,
                    entity,
                    nested,
                    surface="nested_retouch",
                )

    def validate_asset_list_oracle(
        self,
        combination: str,
        identity: str,
        task_id: str,
        body: object,
    ) -> None:
        entity = f"task:{task_id}:/v1/tasks/{{task_id}}/assets"
        data = unwrap_data(body)
        with self._state_lock:
            self.manifest_oracle_check_count += 1
        if not isinstance(data, list):
            self.oracle_violation(
                entity, f"{combination}/{identity} asset list is invalid"
            )
            return
        requested_task_id = (
            int(task_id) if ID_RE.fullmatch(str(task_id)) else None
        )
        expected_root_ids = {
            int(root_id)
            for root_id, row in self.list_current_by_root.items()
            if row["task_id"] == requested_task_id
        }
        actual_root_ids = [
            asset.get("id")
            for asset in data
            if isinstance(asset, dict)
            and self._positive_int(asset.get("id"))
        ]
        if (
            requested_task_id is None
            or len(actual_root_ids) != len(data)
            or len(actual_root_ids) != len(set(actual_root_ids))
            or set(actual_root_ids) != expected_root_ids
        ):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} requested task {task_id} "
                "asset root set differs",
            )
        for asset in data:
            if not isinstance(asset, dict):
                self.oracle_violation(
                    entity, f"{combination}/{identity} asset root is invalid"
                )
                continue
            root_id = asset.get("id")
            if asset.get("task_id") != requested_task_id:
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} asset root {root_id} "
                    f"is outside requested task {task_id}",
                )
            if (
                "warehouse_ready_version" in asset
                or "warehouse_ready_version_id" in asset
            ):
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} asset root {root_id} "
                    "resurrects retired warehouse-ready pointer",
                )
            current_oracle = (
                self.list_current_by_root.get(str(root_id))
                if isinstance(root_id, int) and root_id > 0
                else None
            )
            root_scope = asset.get("scope_sku_code")
            root_retouch = asset.get("retouch_requirement_id")
            root_scope_valid = root_scope is None or isinstance(root_scope, str)
            root_retouch_valid = (
                root_retouch is None
                or (
                    isinstance(root_retouch, int)
                    and not isinstance(root_retouch, bool)
                    and root_retouch > 0
                )
            )
            root_actual = {
                "root_id": root_id,
                "task_id": asset.get("task_id"),
                "asset_type": str(asset.get("asset_type") or ""),
                "scope_sku_code": str(root_scope or ""),
                "retouch_requirement_id": root_retouch,
            }
            root_expected = (
                {
                    "root_id": current_oracle["root_asset_id"],
                    "task_id": current_oracle["task_id"],
                    "asset_type": current_oracle["root_asset_type"],
                    "scope_sku_code": current_oracle[
                        "root_scope_sku_code"
                    ],
                    "retouch_requirement_id": current_oracle[
                        "root_retouch_requirement_id"
                    ],
                }
                if isinstance(current_oracle, dict)
                else None
            )
            if (
                not root_scope_valid
                or not root_retouch_valid
                or root_actual != root_expected
            ):
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} asset root {root_id} "
                    "identity projection differs",
                )
                continue
            versions = (
                (
                    "current_version",
                    "current_version_id",
                    current_oracle,
                    "list_asset",
                ),
                (
                    "approved_version",
                    "approved_version_id",
                    self.list_approved_by_root.get(str(root_id)),
                    "list_approved_asset",
                ),
            )
            approved_for_root = self.list_approved_by_root.get(str(root_id))
            for field, pointer_field, expected_version, coverage_kind in versions:
                version = asset.get(field)
                pointer = asset.get(pointer_field)
                if expected_version is None:
                    if version is not None or pointer is not None:
                        self.oracle_violation(
                            entity,
                            f"{combination}/{identity} asset root {root_id} "
                            f"unexpectedly exposes {field}",
                        )
                    continue
                if (
                    isinstance(version, dict)
                    and "warehouse_ready" in version
                ):
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} asset root {root_id} "
                        f"{field} resurrects retired warehouse_ready",
                    )
                identity_row = self.validate_version_identity(
                    combination,
                    identity,
                    entity,
                    version,
                    surface="list",
                )
                actual_pointer = {
                    "current_version_role": (
                        version.get("current_version_role")
                        if isinstance(version, dict)
                        else None
                    ),
                    "pointer_id": pointer,
                }
                expected_pointer = {
                    "current_version_role": (
                        "current_approved_version"
                        if (
                            field == "current_version"
                            and approved_for_root is not None
                            and approved_for_root["task_asset_id"]
                            == expected_version["task_asset_id"]
                        )
                        else (
                            "current_version"
                            if field == "current_version"
                            else (
                                "current_approved_version"
                                if current_oracle["task_asset_id"]
                                == expected_version["task_asset_id"]
                                else "approved_version"
                            )
                        )
                    ),
                    "pointer_id": expected_version["task_asset_id"],
                }
                if identity_row is None or actual_pointer != expected_pointer:
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} asset root {root_id} "
                        f"{field} identity projection differs",
                    )
                    continue
                self.cover(
                    combination,
                    identity,
                    coverage_kind,
                    str(expected_version["task_asset_id"]),
                )

    def validate_resource_bundle_oracle(
        self, combination: str, identity: str, task_id: str, body: object
    ) -> None:
        if not combination.endswith("_b"):
            return
        entity = f"task:{task_id}:/v1/tasks/{{task_id}}/resource-bundle"
        data = unwrap_data(body)
        with self._state_lock:
            self.manifest_oracle_check_count += 1
        if not isinstance(data, dict) or not isinstance(data.get("groups"), list):
            self.oracle_violation(
                entity, f"{combination}/{identity} resource bundle is invalid"
            )
            return
        self.cover(combination, identity, "bundle", task_id)
        expected_task = self.expectations["tasks"].get(task_id, {})
        if (
            str(data.get("task_id", "")) != task_id
            or str(data.get("workflow_revision", ""))
            != str(expected_task.get("workflow_revision", ""))
        ):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} resource bundle task projection differs",
            )
        expected_groups = {
            locator: expected
            for locator, expected in self.expectations["groups"].items()
            if expected["task_id"] == task_id
        }
        actual_groups: dict[str, dict[str, Any]] = {}
        for group in data["groups"]:
            if not isinstance(group, dict):
                self.oracle_violation(
                    entity, f"{combination}/{identity} contains a non-object group"
                )
                continue
            locator = (
                f"{group.get('task_id')}:{group.get('scope_kind')}:"
                f"{self._scope_ref(group)}"
            )
            if locator in actual_groups:
                self.oracle_violation(
                    entity, f"{combination}/{identity} duplicates group {locator}"
                )
            actual_groups[locator] = group
        if set(actual_groups) != set(expected_groups):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} group locators "
                f"{sha256(canonical(sorted(actual_groups)))}!="
                f"{sha256(canonical(sorted(expected_groups)))}",
            )
        for locator in sorted(set(actual_groups) & set(expected_groups)):
            self.validate_group_oracle(
                combination, identity, actual_groups[locator], expected_groups[locator]
            )

    def validate_group_oracle(
        self,
        combination: str,
        identity: str,
        group: dict[str, Any],
        expected: dict[str, Any] | None = None,
        *,
        requested_group_id: str | None = None,
    ) -> None:
        locator = (
            f"{group.get('task_id')}:{group.get('scope_kind')}:"
            f"{self._scope_ref(group)}"
        )
        entity = f"group:{requested_group_id or group.get('id', '?')}"
        expected = expected or self.expectations["groups"].get(locator)
        with self._state_lock:
            self.manifest_oracle_check_count += 1
        if (
            requested_group_id is not None
            and str(group.get("id", "")) != requested_group_id
        ):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} requested group id "
                f"{requested_group_id} returned {group.get('id')}",
            )
        if not isinstance(expected, dict):
            self.oracle_violation(
                entity, f"{combination}/{identity} unexpected group locator {locator}"
            )
            return
        group_required = {
            "id",
            "task_id",
            "scope_kind",
            "lock_version",
            "migration_incomplete",
            "created_at",
            "updated_at",
        }
        missing_group = sorted(group_required - group.keys())
        if missing_group:
            self.oracle_violation(
                entity,
                f"{combination}/{identity} group misses required fields "
                f"{','.join(missing_group)}",
            )
        for field in ("id", "task_id"):
            if field in group and not self._positive_int(group[field]):
                self.oracle_violation(
                    entity, f"{combination}/{identity} group {field} is invalid"
                )
        if "lock_version" in group and not self._nonnegative_int(
            group["lock_version"]
        ):
            self.oracle_violation(
                entity, f"{combination}/{identity} group lock_version is invalid"
            )
        if not isinstance(group.get("migration_incomplete"), bool):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} group migration_incomplete is invalid",
            )
        for field in ("created_at", "updated_at"):
            if not isinstance(group.get(field), str) or not group.get(field):
                self.oracle_violation(
                    entity, f"{combination}/{identity} group {field} is invalid"
                )
        self.cover(combination, identity, "group", locator)
        working = group.get("working_revision")
        finalized = group.get("finalized_revision")
        actual = {
            "task_id": str(group.get("task_id", "")),
            "scope_kind": str(group.get("scope_kind", "")),
            "scope_ref_id": self._scope_ref(group),
            "working_revision_no": (
                str(working.get("revision_no", ""))
                if isinstance(working, dict)
                else ""
            ),
            "working_revision_status": (
                str(working.get("status", "")) if isinstance(working, dict) else ""
            ),
            "finalized_revision_no": (
                str(finalized.get("revision_no", ""))
                if isinstance(finalized, dict)
                else ""
            ),
            "finalized_revision_status": (
                str(finalized.get("status", ""))
                if isinstance(finalized, dict)
                else ""
            ),
            "migration_incomplete": "1" if group.get("migration_incomplete") else "0",
            "migration_issue": str(group.get("migration_issue") or ""),
        }
        oracle = dict(expected)
        if actual != oracle:
            self.oracle_violation(
                entity,
                f"{combination}/{identity} group projection "
                f"{sha256(canonical(actual))}!={sha256(canonical(oracle))}",
            )
        for pointer_name in ("working", "finalized"):
            revision = group.get(f"{pointer_name}_revision")
            pointer = group.get(f"{pointer_name}_revision_id")
            if isinstance(revision, dict):
                self.validate_revision_contract_shape(
                    combination, identity, entity, revision
                )
                if pointer != revision.get("id"):
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} {pointer_name} pointer differs",
                    )
                if revision.get("group_id") not in {None, group.get("id")}:
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} {pointer_name} group differs",
                    )
            elif pointer is not None:
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} dangling {pointer_name} pointer",
                )

    def validate_history_oracle(
        self,
        combination: str,
        identity: str,
        group: dict[str, Any],
        history: object,
    ) -> None:
        locator = (
            f"{group.get('task_id')}:{group.get('scope_kind')}:"
            f"{self._scope_ref(group)}"
        )
        entity = f"group:{group.get('id', '?')}"
        expected_revisions = {
            key: expected
            for key, expected in self.expectations["revisions"].items()
            if key.startswith(locator + ":")
        }
        rows = history_items(history)
        pages = history if isinstance(history, list) else [history]
        first_data = (
            unwrap_data(pages[0])
            if pages
            else None
        )
        if not isinstance(first_data, dict):
            self.oracle_violation(
                entity, f"{combination}/{identity} history envelope is invalid"
            )
        else:
            required_page = {"items", "page", "page_size", "total"}
            missing_page = sorted(required_page - first_data.keys())
            if missing_page:
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} history misses required fields "
                    f"{','.join(missing_page)}",
                )
            for pointer_name in ("working", "finalized"):
                history_pointer = first_data.get(f"{pointer_name}_revision_id")
                group_pointer = group.get(f"{pointer_name}_revision_id")
                if history_pointer != group_pointer:
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} history {pointer_name} pointer differs",
                    )
        actual_revisions = {
            f"{locator}:{row.get('revision_no')}": row
            for row in rows
            if isinstance(row, dict)
        }
        with self._state_lock:
            self.manifest_oracle_check_count += 1
        if len(actual_revisions) != len(rows):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} contains duplicate or invalid revisions",
            )
        if set(actual_revisions) != set(expected_revisions):
            self.oracle_violation(
                entity,
                f"{combination}/{identity} revision locators "
                f"{sha256(canonical(sorted(actual_revisions)))}!="
                f"{sha256(canonical(sorted(expected_revisions)))}",
            )
        history_by_id = {
            row.get("id"): row
            for row in rows
            if isinstance(row, dict) and self._positive_int(row.get("id"))
        }
        for pointer_name in ("working", "finalized"):
            pointer_id = group.get(f"{pointer_name}_revision_id")
            embedded = group.get(f"{pointer_name}_revision")
            if pointer_id is None and embedded is None:
                continue
            history_row = history_by_id.get(pointer_id)
            if (
                not isinstance(embedded, dict)
                or not isinstance(history_row, dict)
                or self.revision_link_projection(embedded)
                != self.revision_link_projection(history_row)
            ):
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} {pointer_name} revision is not "
                    "identical to its history row",
                )
        for revision_locator in sorted(
            set(actual_revisions) & set(expected_revisions)
        ):
            self.cover(combination, identity, "revision", revision_locator)
            row = actual_revisions[revision_locator]
            expected = expected_revisions[revision_locator]
            self.validate_revision_contract_shape(
                combination,
                identity,
                entity,
                row,
                historical=True,
            )
            source_id = row.get("source_task_asset_id")
            expected_source = expected["source_locator"]
            actual = {
                "task_id": locator.split(":", 1)[0],
                "scope_kind": locator.split(":")[1],
                "scope_ref_id": locator.split(":")[2],
                "revision_no": str(row.get("revision_no", "")),
                "status": str(row.get("status", "")),
                "mode": str(row.get("mode", "")),
                "source_present": (
                    "1"
                    if isinstance(source_id, int) and source_id > 0
                    else ""
                ),
                "source_stage": str(row.get("source_stage", "")),
                "created_by": str(row.get("created_by", "")),
                "reason": str(row.get("reason", "")),
                "submitted_at": normalize_manifest_time(
                    str(row.get("submitted_at", ""))
                ),
                "finalized_at": normalize_manifest_time(
                    str(row.get("finalized_at", ""))
                ),
            }
            oracle = {
                **{
                    key: expected[key]
                    for key in (
                        "task_id",
                        "scope_kind",
                        "scope_ref_id",
                        "revision_no",
                        "status",
                        "mode",
                    )
                },
                "source_present": "1" if expected_source else "",
                **{
                    key: normalize_manifest_time(expected[key])
                    if key.endswith("_at")
                    else expected[key]
                    for key in (
                        "source_stage",
                        "created_by",
                        "reason",
                        "submitted_at",
                        "finalized_at",
                    )
                },
            }
            if actual != oracle:
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} revision {revision_locator} "
                    f"{sha256(canonical(actual))}!={sha256(canonical(oracle))}",
                )
            role_oracle = self.revision_role_oracle.get(revision_locator)
            actual_source_locator = (
                self.asset_identity_oracle.get(str(source_id), {}).get(
                    "stable_locator"
                )
                if self._positive_int(source_id)
                else None
            )
            raw_items = row.get("items")
            actual_final_locators = (
                [
                    self.asset_identity_oracle.get(
                        str(item.get("task_asset_id")), {}
                    ).get("stable_locator")
                    for item in raw_items
                    if isinstance(item, dict)
                ]
                if isinstance(raw_items, list)
                else None
            )
            raw_refs = row.get("references")
            actual_reference_locators = (
                [
                    f"reference:{item.get('reference_file_ref_id')}:"
                    f"{item.get('ref_id')}"
                    for item in raw_refs
                    if isinstance(item, dict)
                ]
                if isinstance(raw_refs, list)
                else None
            )
            role_actual = {
                "status": row.get("status"),
                "source_stage": row.get("source_stage"),
                "source_locator": actual_source_locator,
                "final_locators": actual_final_locators,
                "reference_file_ref_ids": (
                    [
                        item.get("reference_file_ref_id")
                        for item in raw_refs
                        if isinstance(item, dict)
                    ]
                    if isinstance(raw_refs, list)
                    else None
                ),
                "reference_locators": actual_reference_locators,
                "is_working": (
                    row.get("id") == group.get("working_revision_id")
                ),
                "is_finalized": (
                    row.get("id") == group.get("finalized_revision_id")
                ),
            }
            role_expected = (
                {
                    key: role_oracle[key]
                    for key in (
                        "status",
                        "source_stage",
                        "source_locator",
                        "final_locators",
                        "reference_file_ref_ids",
                        "reference_locators",
                        "is_working",
                        "is_finalized",
                    )
                }
                if isinstance(role_oracle, dict)
                else None
            )
            if role_actual != role_expected:
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} revision {revision_locator} "
                    "v2 role projection differs",
                )
            if row.get("group_id") not in {None, group.get("id")}:
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} revision {revision_locator} group differs",
                )
            revision_id = row.get("id")
            if (
                revision_id is not None
                and (
                    not isinstance(revision_id, int)
                    or isinstance(revision_id, bool)
                    or revision_id <= 0
                )
            ):
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} revision {revision_locator} id is invalid",
                )
            if "[migration_v2 " in expected["reason"]:
                evidence = row.get("evidence_summary")
                try:
                    expected_evidence = expected_evidence_summary(
                        expected["reason"],
                        self.revision_reason_oracle.get(
                            revision_locator, ""
                        ),
                    )
                except (TypeError, ValueError) as exc:
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} revision {revision_locator} "
                        f"evidence marker invalid: {exc}",
                    )
                    expected_evidence = None
                normalized_evidence = (
                    {
                        key: (
                            normalize_manifest_time(str(value))
                            if key == "confirmed_at"
                            else value
                        )
                        for key, value in evidence.items()
                    }
                    if isinstance(evidence, dict)
                    else evidence
                )
                if (
                    row.get("legacy_migration") is not True
                    or normalized_evidence != expected_evidence
                ):
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} revision {revision_locator} "
                        "legacy evidence differs",
                    )
            expected_source_row = self.expectations["sources"].get(
                revision_locator
            )
            if not isinstance(expected_source_row, dict):
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} revision {revision_locator} lacks source oracle",
                )
            else:
                identity_row = (
                    self.asset_identity_oracle.get(str(source_id))
                    if isinstance(source_id, int) and source_id > 0
                    else None
                )
                source_actual = {
                    "source_locator": "",
                    "role": "",
                    "whole_hash": "",
                    "binding": "",
                    "binding_role": "",
                    "sku_code": "",
                    "retouch_requirement_id": "",
                }
                if isinstance(identity_row, dict):
                    if str(identity_row.get("task_id", "")) != expected["task_id"]:
                        self.oracle_violation(
                            entity,
                            f"{combination}/{identity} revision {revision_locator} "
                            "source task differs",
                        )
                    source_actual = {
                        "source_locator": str(
                            identity_row.get(
                                "manifest_locator",
                                identity_row["stable_locator"],
                            )
                        ),
                        "role": str(identity_row["asset_type"]),
                        "whole_hash": str(identity_row["whole_hash"]),
                        "binding": str(identity_row["binding_state"]),
                        "binding_role": str(identity_row["bound_role"]),
                        "sku_code": effective_oracle_scope_sku_code(
                            identity_row
                        ),
                        "retouch_requirement_id": str(
                            effective_oracle_retouch_requirement_id(identity_row)
                            or ""
                        ),
                    }
                expected_source_projection = dict(expected_source_row)
                if source_actual != expected_source_projection:
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} revision {revision_locator} source differs",
                    )
            expected_items = self.expectations["finals"].get(revision_locator, [])
            items = row.get("items")
            if isinstance(items, list) and not all(
                isinstance(item, dict) for item in items
            ):
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} revision {revision_locator} has invalid final item",
                )
            actual_items = (
                [
                    {
                        "task_asset_id": str(item.get("task_asset_id", "")),
                        "sort_order": str(item.get("sort_order", "")),
                        "formal_storage_ref_id": "",
                        "asset_type": str(
                            self.asset_identity_oracle.get(
                                str(item.get("task_asset_id", "")), {}
                            ).get("asset_type", "")
                        ),
                        "whole_hash": str(
                            self.asset_identity_oracle.get(
                                str(item.get("task_asset_id", "")), {}
                            ).get("whole_hash")
                            or ""
                        ),
                        "binding": str(
                            self.asset_identity_oracle.get(
                                str(item.get("task_asset_id", "")), {}
                            ).get("binding_state", "")
                        ),
                        "role": str(
                            self.asset_identity_oracle.get(
                                str(item.get("task_asset_id", "")), {}
                            ).get("bound_role", "")
                        ),
                        "sku_code": effective_oracle_scope_sku_code(
                            self.asset_identity_oracle.get(
                                str(item.get("task_asset_id", "")), {}
                            )
                        ),
                        "retouch_requirement_id": str(
                            effective_oracle_retouch_requirement_id(
                                self.asset_identity_oracle.get(
                                    str(item.get("task_asset_id", "")), {}
                                )
                            )
                            or ""
                        ),
                    }
                    for item in items
                    if isinstance(item, dict)
                ]
                if isinstance(items, list)
                else None
            )
            for item in items if isinstance(items, list) else []:
                item_oracle = self.asset_identity_oracle.get(
                    str(item.get("task_asset_id", ""))
                ) if isinstance(item, dict) else None
                if (
                    not isinstance(item_oracle, dict)
                    or str(item_oracle.get("task_id", "")) != expected["task_id"]
                ):
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} revision {revision_locator} "
                        "final task differs",
                    )
            if actual_items != expected_items:
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} revision {revision_locator} finals differ",
                )
            expected_refs = self.expectations["references"].get(revision_locator, [])
            references = row.get("references")
            if isinstance(references, list) and not all(
                isinstance(item, dict) for item in references
            ):
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} revision {revision_locator} has invalid reference",
                )
            actual_refs = (
                [
                    {
                        "reference_file_ref_id": str(
                            item.get("reference_file_ref_id", "")
                        ),
                        "formal_storage_ref_id": (
                            self.storage_ref_from_stable_locator(
                                self.asset_identity_oracle.get(
                                    str(item.get("formal_task_asset_id", "")), {}
                                ).get("stable_locator", "")
                            )
                        ),
                        "sort_order": str(item.get("sort_order", "")),
                        "ref_id": str(item.get("ref_id", "")),
                        "file_name": str(item.get("file_name", "")),
                        "scope": str(item.get("scope", "")),
                    }
                    for item in references
                    if isinstance(item, dict)
                ]
                if isinstance(references, list)
                else None
            )
            for reference_index, reference in enumerate(
                references if isinstance(references, list) else []
            ):
                if not isinstance(reference, dict):
                    continue
                formal_id = reference.get("formal_task_asset_id")
                expected_formal_ref = (
                    expected_refs[reference_index]["formal_storage_ref_id"]
                    if reference_index < len(expected_refs)
                    else None
                )
                if not expected_formal_ref:
                    if formal_id is not None:
                        self.oracle_violation(
                            entity,
                            f"{combination}/{identity} revision {revision_locator} "
                            "reference unexpectedly has a formal asset",
                        )
                    continue
                formal_oracle = self.asset_identity_oracle.get(str(formal_id))
                if (
                    not self._positive_int(formal_id)
                    or not isinstance(formal_oracle, dict)
                    or str(formal_oracle.get("task_id", "")) != expected["task_id"]
                    or self.storage_ref_from_stable_locator(
                        formal_oracle.get("stable_locator")
                    )
                    != expected_formal_ref
                    or formal_oracle.get("asset_type") != "reference"
                    or formal_oracle.get("bound_role") not in {"", "NULL"}
                ):
                    self.oracle_violation(
                        entity,
                        f"{combination}/{identity} revision {revision_locator} "
                        "reference formal asset differs",
                    )
            if actual_refs != expected_refs:
                self.oracle_violation(
                    entity,
                    f"{combination}/{identity} revision {revision_locator} references differ",
                )

    def request(
        self,
        combination: str,
        identity: str,
        path: str,
    ) -> HttpResult:
        """Return one immutable result per exact origin/identity/path.

        The Future provides single-flight behavior: one worker owns the
        physical GET while every concurrent logical caller waits for and
        receives the same frozen HttpResult, or the same exception. Identity
        IDs deliberately remain in the key even when resolved headers happen
        to be byte-for-byte equal.
        """
        key = (self.urls[combination], identity, path)
        with self._state_lock:
            self.logical_request_count += 1
            future = self._requests.get(key)
            owner = future is None
            if owner:
                future = Future()
                self._requests[key] = future
                self.physical_request_count += 1
        assert future is not None
        if owner:
            try:
                result = self.requester(
                    self.urls[combination],
                    path,
                    dict(self.resolved_headers[identity]),
                )
                if not isinstance(result, HttpResult):
                    raise TypeError("requester must return HttpResult")
            except BaseException as exc:
                future.set_exception(exc)
            else:
                future.set_result(result)
        return future.result()

    def observation(
        self,
        combination: str,
        identity: str,
        route: str,
        entity: str,
        result: HttpResult,
    ) -> None:
        row = {
            "combination": combination,
            "identity": identity,
            "route": route,
            "entity_key": entity,
            "status": result.status,
            "body_sha256": sha256(
                canonical(normalize_transport_noise(result.body))
            ),
            "raw_sha256": result.raw_sha256,
            "body_bytes": result.body_bytes,
        }
        with self._state_lock:
            self.observations.append(row)
            role = self.identity_roles.get(identity)
            if (
                combination.endswith("_b")
                and role == "view_only"
                and route == CORE_ROUTES[0]
                and result.status in {200, 403}
            ):
                self.view_only_task_statuses[combination][result.status].add(
                    entity
                )
        role = self.identity_roles.get(identity)
        if (
            combination.endswith("_b")
            and role == "no_view"
            and route in ALL_ROUTE_TEMPLATES
            and result.status == 200
        ):
            self.violation(
                "api.no_view_access_granted",
                entity,
                f"{combination}/{identity} {route} returned 200",
            )
        if (
            combination.endswith("_b")
            and role == "view_only"
            and route == "/v1/task-assets/{task_asset_id}/download"
            and result.status == 200
        ):
            self.violation(
                "api.view_only_download_granted",
                entity,
                f"{combination}/{identity} asset download returned 200",
            )
        if (
            combination.endswith("_b")
            and role == "view_only"
            and result.status == 200
            and route != "/v1/task-assets/{task_asset_id}/preview"
        ):
            paths = field_paths(result.body, "download_url")
            if paths:
                self.violation(
                    "api.view_only_download_url_exposed",
                    entity,
                    f"{combination}/{identity} exposes download_url at "
                    f"{sha256(canonical(paths))}",
                )

    def violation(self, code: str, entity: str, detail: str) -> None:
        with self._state_lock:
            self.violations.append(
                {"violation_code": code, "entity_key": entity, "detail": detail}
            )

    def fetch(
        self,
        combination: str,
        identity: str,
        route: str,
        path: str,
        entity: str,
    ) -> HttpResult:
        result = self.request(combination, identity, path)
        with self._state_lock:
            self.results[(combination, identity, route, entity)] = result
        self.observation(combination, identity, route, entity, result)
        if result.status >= 500:
            self.violation(
                "api.server_error", entity, f"{combination}/{identity} returned {result.status}"
            )
        return result

    def fetch_history(
        self, combination: str, identity: str, group_id: str, entity: str
    ) -> HttpResult:
        pages: list[object] = []
        page = 1
        declared_total: int | None = None
        while True:
            path = (
                f"/v1/resource-groups/{group_id}/revisions"
                f"?page={page}&page_size=200"
            )
            result = self.request(combination, identity, path)
            self.observation(
                combination,
                identity,
                GROUP_ROUTES[1],
                f"{entity}:page:{page}",
                result,
            )
            if result.status >= 500:
                self.violation(
                    "api.server_error",
                    entity,
                    f"{combination}/{identity} returned {result.status}",
                )
            if result.status != 200:
                final = result
                break
            pages.append(result.body)
            data = (
                result.body.get("data", result.body)
                if isinstance(result.body, dict)
                else {}
            )
            total = data.get("total", 0) if isinstance(data, dict) else 0
            rows = data.get("items", []) if isinstance(data, dict) else []
            if (
                not isinstance(total, int)
                or isinstance(total, bool)
                or total < 0
                or not isinstance(rows, list)
            ):
                self.violation(
                    "api.invalid_pagination", entity, f"{combination}/{identity}"
                )
                final = HttpResult(200, pages, sha256(canonical(pages)), len(canonical(pages)))
                break
            if (
                data.get("page") != page
                or data.get("page_size") != 200
                or len(rows) > 200
            ):
                self.violation(
                    "api.invalid_pagination",
                    entity,
                    f"{combination}/{identity} page metadata differs",
                )
            if declared_total is None:
                declared_total = total
            elif total != declared_total:
                self.violation(
                    "api.invalid_pagination",
                    entity,
                    f"{combination}/{identity} total changed across pages",
                )
            if page * 200 >= total or not rows:
                final = HttpResult(200, pages, sha256(canonical(pages)), len(canonical(pages)))
                break
            page += 1
            if page > 10000:
                raise ValueError(f"history pagination did not terminate for group {group_id}")
        with self._state_lock:
            self.results[(combination, identity, GROUP_ROUTES[1], entity)] = final
        if final.status == 200:
            if declared_total is None or len(history_items(final.body)) != declared_total:
                self.violation(
                    "api.invalid_pagination",
                    entity,
                    f"{combination}/{identity} fetched rows differ from total",
                )
            numbers = [
                item.get("revision_no")
                for item in history_items(final.body)
                if isinstance(item.get("revision_no"), int)
            ]
            if len(numbers) != len(history_items(final.body)) or any(
                left <= right for left, right in zip(numbers, numbers[1:])
            ):
                self.violation(
                    "api.revision_order_invalid", entity, f"{combination}/{identity}"
                )
            for detail in revision_order_errors(final.body):
                self.violation(
                    "api.asset_order_invalid",
                    entity,
                    f"{combination}/{identity}: {detail}",
                )
        return final

    def find_rule(
        self,
        identity: str,
        route: str,
        direction: str,
        from_status: int,
        to_status: int,
    ) -> dict[str, Any] | None:
        matches = [
            rule
            for rule in self.rules
            if rule["route"] == route
            and rule["direction"] == direction
            and rule["from_status"] == from_status
            and rule["to_status"] == to_status
            and (
                rule.get("identity") is None
                or rule["identity"] == identity
            )
        ]
        if len(matches) > 1:
            raise ValueError(
                f"multiple normalization rules match {identity} {route} {direction} "
                f"{from_status}->{to_status}"
            )
        return matches[0] if matches else None

    def compare_result(
        self,
        route: str,
        entity: str,
        identity: str,
        left_combo: str,
        right_combo: str,
    ) -> None:
        left = self.results[(left_combo, identity, route, entity)]
        right = self.results[(right_combo, identity, route, entity)]
        direction = f"{left_combo}->{right_combo}"
        # Reused direct-backend URLs share the same single-flight HttpResult.
        # Object identity therefore proves exact origin/identity/path parity;
        # the two logical observations remain in evidence, while duplicate
        # deep canonicalization is unnecessary.
        if left is right:
            return
        # Permission widening is directional from the frozen A baseline to the
        # migrated B candidate, regardless of the pair iteration order.  The
        # four-combination matrix also contains B->A and same-data comparisons;
        # treating those lexical directions as authorization transitions would
        # misclassify candidate permission tightening as widening.
        left_data = "A" if left_combo.endswith("_a") else "B"
        right_data = "A" if right_combo.endswith("_a") else "B"
        rule = self.find_rule(
            identity, route, direction, left.status, right.status
        )
        if left_data != right_data:
            if left_data == "A":
                baseline_combo, baseline = left_combo, left
                candidate_combo, candidate = right_combo, right
            else:
                baseline_combo, baseline = right_combo, right
                candidate_combo, candidate = left_combo, left
        else:
            baseline_combo = candidate_combo = ""
            baseline = candidate = None
        if (
            baseline is not None
            and candidate is not None
            and baseline.status in {401, 403}
            and candidate.status not in {401, 403}
        ):
            self.violation(
                "api.permission_widened",
                entity,
                f"{identity} {baseline_combo}->{candidate_combo} "
                f"{baseline.status}->{candidate.status}",
            )
            return
        if baseline is not None and candidate is not None:
            baseline_actions = allowed_actions_by_path(
                normalize_transport_noise(baseline.body)
            )
            candidate_actions = allowed_actions_by_path(
                normalize_transport_noise(candidate.body)
            )
            for path in sorted(set(baseline_actions) & set(candidate_actions)):
                widened = candidate_actions[path] - baseline_actions[path]
                if widened:
                    self.violation(
                        "api.allowed_actions_widened",
                        entity,
                        f"{identity} {baseline_combo}->{candidate_combo} "
                        f"{path} added {sha256(canonical(sorted(widened)))}",
                    )
                    return
            if (
                baseline.status in {401, 403}
                and candidate.status in {401, 403}
            ):
                # Authentication/authorization error envelope wording is not
                # an A/B compatibility contract. Both sides denied access, and
                # the directional widening check above still fails closed if
                # the B candidate ever grants it.
                with self._state_lock:
                    self.semantic_comparison_count += 1
                return
        if route in self.retired_routes:
            # Each retired route is already asserted to be exactly 404. Error
            # envelope wording is deliberately not a compatibility contract.
            return
        if route in ASSET_ROUTES:
            allowed = ASSET_ROUTE_ALLOWED_STATUSES[route]
            if left.status not in allowed or right.status not in allowed:
                self.violation(
                    "api.asset_contract_status_invalid",
                    entity,
                    f"{identity} {direction} {left.status}->{right.status}",
                )
                return
            if left.status == 200 and right.status == 410:
                # A tombstone is an asset loss unless an exact status rule approves it.
                pass
        if left_data != right_data and rule is None:
            if left_data == "A":
                a_combo, a_result = left_combo, left
                b_combo, b_result = right_combo, right
            else:
                a_combo, a_result = right_combo, right
                b_combo, b_result = left_combo, left
            if (
                route
                in {
                    "/v1/tasks/{task_id}/resource-bundle",
                    *GROUP_ROUTES,
                }
                and a_result.status == 404
                and b_result.status in {200, 403}
            ):
                with self._state_lock:
                    self.semantic_comparison_count += 1
                return
            if route in ASSET_ROUTES and a_result.status == 404:
                asset_id = entity.split(":", 1)[1]
                governed_for_b = set().union(
                    *(
                        assets
                        for (combo, _identity), assets in self.governed_assets.items()
                        if combo == b_combo
                    )
                )
                identity_governed = self.governed_assets.get(
                    (b_combo, identity), set()
                )
                allowed_governed = (
                    governed_for_b if b_result.status == 403 else identity_governed
                )
                if (
                    asset_id in allowed_governed
                    and b_result.status
                    in ASSET_ROUTE_ALLOWED_STATUSES[route] - {404}
                ):
                    with self._state_lock:
                        self.semantic_comparison_count += 1
                    return
            if (
                left.status == right.status == 200
                and route
                in {
                    "/v1/tasks/{task_id}",
                    "/v1/tasks/{task_id}/detail",
                    "/v1/tasks/{task_id}/assets",
                }
            ):
                projector = {
                    "/v1/tasks/{task_id}": project_task,
                    "/v1/tasks/{task_id}/detail": project_detail,
                    "/v1/tasks/{task_id}/assets": project_assets,
                }[route]
                if route == "/v1/tasks/{task_id}/assets":
                    a_projection, b_projection = project_asset_pair(
                        a_result.body,
                        b_result.body,
                        frozenset(
                            set(self.list_current_by_root)
                            | self.migration_addition_root_ids
                        ),
                        self.asset_identity_oracle,
                    )
                elif route == "/v1/tasks/{task_id}/detail":
                    a_projection, b_projection = project_detail_pair(
                        a_result.body,
                        b_result.body,
                    )
                else:
                    a_projection = projector(a_result.body)
                    b_projection = projector(b_result.body)
                with self._state_lock:
                    self.semantic_comparison_count += 1
                if canonical(a_projection) != canonical(b_projection):
                    self.violation(
                        "api.semantic_body_mismatch",
                        entity,
                        f"{identity} {a_combo}->{b_combo} "
                        f"{sha256(canonical(a_projection))}!="
                        f"{sha256(canonical(b_projection))}",
                    )
                    return
                if route == "/v1/tasks/{task_id}/detail":
                    a_versions = detail_asset_versions(a_result.body)
                    b_versions = detail_asset_versions(b_result.body)
                    if (a_versions is None) != (b_versions is None):
                        self.violation(
                            "api.semantic_body_mismatch",
                            entity,
                            f"{identity} {a_combo}->{b_combo} lacks asset_versions",
                        )
                        return
                    a_versions_by_id = {
                        str(version.get("id")): version
                        for version in a_versions or []
                        if isinstance(version, dict)
                        and self._positive_int(version.get("id"))
                    }
                    b_versions_by_id = {
                        str(version.get("id")): version
                        for version in b_versions or []
                        if isinstance(version, dict)
                        and self._positive_int(version.get("id"))
                    }
                    for version_id, version in a_versions_by_id.items():
                        if version_id not in b_versions_by_id:
                            self.violation(
                                "api.legacy_asset_not_preserved",
                                entity,
                                f"{identity} {a_combo}->{b_combo} "
                                f"{sha256(canonical(version))}",
                            )
                    governed_for_b = set().union(
                        *(
                            assets
                            for (combo, _identity), assets in self.governed_assets.items()
                            if combo == b_combo
                        )
                    )
                    for version_id in sorted(
                        set(b_versions_by_id) - set(a_versions_by_id),
                        key=int,
                    ):
                        if version_id not in governed_for_b:
                            self.violation(
                                "api.ungoverned_asset_added",
                                entity,
                                f"{identity} {a_combo}->{b_combo} asset {version_id}",
                            )
                return
        left_body = normalize_transport_noise(left.body)
        right_body = normalize_transport_noise(right.body)
        different = left.status != right.status or canonical(left_body) != canonical(right_body)
        if rule is not None and different:
            try:
                left_body, right_body = apply_rule(left_body, right_body, rule)
            except ValueError as exc:
                self.violation("api.normalization_failed", entity, str(exc))
                return
            with self._state_lock:
                self.used_rules.add(rule["rule_id"])
                self.used_rule_applications.add(
                    (
                        rule["rule_id"],
                        rule.get("identity"),
                        identity,
                        route,
                        direction,
                        left.status,
                        right.status,
                    )
                )
        if left.status != right.status:
            if rule is None:
                code = (
                    "api.asset_lost"
                    if route in ASSET_ROUTES and left.status == 200
                    else "api.status_mismatch"
                )
                self.violation(
                    code,
                    entity,
                    f"{identity} {direction} {left.status}->{right.status}",
                )
            return
        if canonical(left_body) != canonical(right_body):
            self.violation(
                "api.body_mismatch",
                entity,
                f"{identity} {direction} "
                f"{sha256(canonical(left_body))}!={sha256(canonical(right_body))}",
            )

    def compare_all(self) -> None:
        for identity in [item["id"] for item in self.identities]:
            for offset, left in enumerate(COMBINATION_IDS):
                for right in COMBINATION_IDS[offset + 1 :]:
                    shared = sorted(
                        {
                            (route, entity)
                            for combo, ident, route, entity in self.results
                            if combo == left and ident == identity
                        }
                        & {
                            (route, entity)
                            for combo, ident, route, entity in self.results
                            if combo == right and ident == identity
                        }
                    )
                    for route, entity in shared:
                        self.compare_result(route, entity, identity, left, right)


def compare(
    *,
    matrix_path: pathlib.Path,
    task_ids_path: pathlib.Path,
    rules_path: pathlib.Path,
    manifest_path: pathlib.Path,
    api_oracle_path: pathlib.Path,
    run_id: str,
    requester: Callable[[str, str, dict[str, str]], HttpResult] = http_get,
    downloader: Callable[[str, str, dict[str, str]], bytes] = http_download,
    workers: int = 16,
    request_metrics: dict[str, Any] | None = None,
) -> dict[str, Any]:
    if not isinstance(workers, int) or isinstance(workers, bool) or not 1 <= workers <= 64:
        raise ValueError("workers must be an integer between 1 and 64")
    comparator_path = pathlib.Path(__file__).resolve()
    oracle_builder_path = comparator_path.with_name("build_api_oracle.py")
    source_hashes = {
        "comparator_sha256": file_sha256(comparator_path),
        "build_api_oracle_sha256": file_sha256(oracle_builder_path),
    }
    frozen_download_allowed_hosts = download_allowed_hosts()
    if request_metrics is not None:
        request_metrics.clear()
    urls, combinations, resolved, retired_routes = load_matrix(matrix_path)
    identities = [
        {"id": identity_id, "role": next(
            item["role"]
            for item in load_object(matrix_path, "matrix")["identities"]
            if item["id"] == identity_id
        )}
        for identity_id in sorted(resolved)
    ]
    rules = load_rules(rules_path, retired_routes, resolved)
    tasks = load_tasks(task_ids_path)
    manifest_tasks = manifest_task_ids(manifest_path, run_id)
    manifest_sha = file_sha256(manifest_path)
    api_oracle = load_asset_identity_map(
        api_oracle_path, run_id, manifest_sha
    )
    expectations = load_manifest_expectations(
        manifest_path,
        run_id,
        expected_mapping_sha256=api_oracle["inputs"][
            "reviewed_mapping_sha256"
        ],
        expected_baseline_attestation_sha256=api_oracle["inputs"][
            "snapshot_verdict_sha256"
        ],
    )
    if set(tasks) != manifest_tasks:
        raise ValueError(
            "task list does not exactly match reviewed manifest: "
            f"list={len(tasks)},manifest={len(manifest_tasks)}"
        )
    runner = Runner(
        urls,
        identities,
        resolved,
        rules,
        retired_routes,
        expectations,
        api_oracle,
        requester,
    )
    dynamic_groups: dict[tuple[str, str, str], set[str]] = {}
    dynamic_assets: dict[tuple[str, str, str], set[str]] = {}
    visible_bundle_groups: dict[tuple[str, str], set[str]] = {}

    def fetch_task(
        job: tuple[str, str, str]
    ) -> tuple[tuple[str, str, str], set[str], set[str], set[str]]:
        combination, identity, task_id = job
        bodies: list[object] = []
        bundle_groups: set[str] = set()
        for route in CORE_ROUTES:
            path = route.format(task_id=task_id)
            entity = f"task:{task_id}:{route}"
            result = runner.fetch(combination, identity, route, path, entity)
            if route == "/v1/tasks/{task_id}/resource-bundle":
                # The four-combination matrix deliberately exercises both the
                # additive V8 surface and the external-backend rollback
                # counterexample.  Exact 404<->200 transitions are approved
                # only by a hash-bound direction rule during pair comparison.
                allowed_statuses = {200, 403, 404}
            else:
                allowed_statuses = {200, 403}
            if result.status not in allowed_statuses:
                runner.violation(
                    "api.task_status_invalid",
                    entity,
                    f"{combination}/{identity} returned {result.status}",
                )
            if (
                combination.endswith("_b")
                and identity in runner.required_oracle_identities
                and result.status != 200
            ):
                runner.oracle_violation(
                    entity,
                    f"{combination}/{identity} required oracle route returned "
                    f"{result.status}",
                )
            if result.status == 200:
                bodies.append(result.body)
                runner.validate_task_oracle(
                    combination, identity, task_id, route, result.body
                )
                if route == "/v1/tasks/{task_id}/resource-bundle":
                    bundle_groups.update(group_ids(result.body))
                    if allowed_actions_by_path(result.body):
                        runner.oracle_violation(
                            entity,
                            f"{combination}/{identity} resource bundle exposes "
                            "undeclared allowed_actions",
                        )
                    runner.validate_resource_bundle_oracle(
                        combination, identity, task_id, result.body
                    )
        for route in retired_routes:
            entity = f"task:{task_id}:retired:{route}"
            result = runner.fetch(
                combination,
                identity,
                route,
                route.format(task_id=task_id),
                entity,
            )
            if result.status != 404:
                runner.violation(
                    "api.retired_route_not_404",
                    entity,
                    f"{combination}/{identity} returned {result.status}",
                )
        return (
            job,
            set().union(*(group_ids(body) for body in bodies)),
            set().union(*(task_asset_ids(body) for body in bodies)),
            bundle_groups,
        )

    identity_ids = [item["id"] for item in identities]
    task_jobs = [
        (combination, identity, task_id)
        for combination in COMBINATION_IDS
        for identity in identity_ids
        for task_id in tasks
    ]
    with ThreadPoolExecutor(max_workers=workers) as executor:
        for key, groups, assets, bundle_groups in executor.map(fetch_task, task_jobs):
            dynamic_groups[key] = groups
            dynamic_assets[key] = assets
            visible_bundle_groups.setdefault((key[0], key[1]), set()).update(
                bundle_groups
            )
    all_groups = sorted(
        set().union(*dynamic_groups.values()) if dynamic_groups else set(), key=int
    )
    def fetch_group(job: tuple[str, str, str]) -> tuple[str, str, set[str]]:
        combination, identity, group_id = job
        entity = f"group:{group_id}"
        detail = runner.fetch(
            combination,
            identity,
            GROUP_ROUTES[0],
            GROUP_ROUTES[0].format(group_id=group_id),
            entity,
        )
        assets = task_asset_ids(detail.body) if detail.status == 200 else set()
        group = unwrap_data(detail.body)
        if (
            combination.endswith("_b")
            and detail.status == 200
            and isinstance(group, dict)
        ):
            if allowed_actions_by_path(detail.body):
                runner.oracle_violation(
                    entity,
                    f"{combination}/{identity} group detail exposes "
                    "undeclared allowed_actions",
                )
            runner.validate_group_oracle(
                combination,
                identity,
                group,
                requested_group_id=group_id,
            )
        if (
            combination.endswith("_b")
            and group_id in visible_bundle_groups.get((combination, identity), set())
            and detail.status != 200
        ):
            runner.oracle_violation(
                entity,
                f"{combination}/{identity} visible bundle group detail returned "
                f"{detail.status}",
            )
        history = runner.fetch_history(combination, identity, group_id, entity)
        if history.status == 200:
            assets.update(task_asset_ids(history.body))
            if combination.endswith("_b") and isinstance(group, dict):
                if allowed_actions_by_path(history.body):
                    runner.oracle_violation(
                        entity,
                        f"{combination}/{identity} revision history exposes "
                        "undeclared allowed_actions",
                    )
                runner.validate_history_oracle(
                    combination, identity, group, history.body
                )
        if (
            combination.endswith("_b")
            and group_id in visible_bundle_groups.get((combination, identity), set())
            and history.status != 200
        ):
            runner.oracle_violation(
                entity,
                f"{combination}/{identity} visible bundle group history returned "
                f"{history.status}",
            )
        return combination, identity, assets

    group_jobs = [
        (combination, identity, group_id)
        for combination in COMBINATION_IDS
        for identity in identity_ids
        for group_id in all_groups
    ]
    group_assets_by_combination: dict[tuple[str, str], set[str]] = {
        (combination, identity): set()
        for combination in COMBINATION_IDS
        for identity in identity_ids
    }
    with ThreadPoolExecutor(max_workers=workers) as executor:
        for combination, identity, assets in executor.map(fetch_group, group_jobs):
            group_assets_by_combination[(combination, identity)].update(assets)
    runner.governed_assets = {
        key: set(value) for key, value in group_assets_by_combination.items()
    }
    runner.validate_oracle_coverage()
    all_assets = sorted(
        (
            set().union(*group_assets_by_combination.values())
            if group_assets_by_combination
            else set()
        ),
        key=int,
    )
    def fetch_asset(job: tuple[str, str, str]) -> None:
        combination, identity, asset_id = job
        entity = f"task-asset:{asset_id}"
        for route in ASSET_ROUTES:
            result = runner.fetch(
                combination,
                identity,
                route,
                route.format(task_asset_id=asset_id),
                entity,
            )
            allowed_statuses = ASSET_ROUTE_ALLOWED_STATUSES[route]
            if result.status not in allowed_statuses:
                runner.violation(
                    "api.asset_contract_status_invalid",
                    entity,
                    f"{combination}/{identity} {route} returned {result.status}",
                )
            elif result.status == 404 and combination.endswith("_b"):
                runner.violation(
                    "api.expected_asset_missing",
                    entity,
                    f"{combination}/{identity} {route} returned 404",
                )
            if result.status == 200:
                runner.validate_task_asset_access_oracle(
                    combination,
                    identity,
                    asset_id,
                    route,
                    result.body,
                )
            if (
                route == "/v1/task-assets/{task_asset_id}/download"
                and combination.endswith("_b")
                and identity in runner.required_oracle_identities
                and runner.asset_identity_oracle.get(asset_id, {})
                .get("provenance", {})
                .get("kind")
                == "recovery_receipt"
            ):
                expected = runner.asset_identity_oracle[asset_id]
                if result.status != 200:
                    runner.violation(
                        "api.recovery_download_failed",
                        entity,
                        f"{combination}/{identity} recovery download "
                        f"returned {result.status}",
                    )
                    continue
                try:
                    raw = downloader(
                        runner.urls[combination],
                        route.format(task_asset_id=asset_id),
                        dict(runner.resolved_headers[identity]),
                    )
                except Exception as exc:
                    runner.violation(
                        "api.recovery_download_failed",
                        entity,
                        f"{combination}/{identity} recovery GET failed: "
                        f"{type(exc).__name__}",
                    )
                    continue
                actual_sha256 = sha256(raw) if isinstance(raw, bytes) else ""
                actual_size = len(raw) if isinstance(raw, bytes) else -1
                if (
                    not isinstance(raw, bytes)
                    or actual_size != expected["size"]
                    or actual_sha256 != expected["content_sha256"]
                ):
                    runner.violation(
                        "api.recovery_download_hash_mismatch",
                        entity,
                        f"{combination}/{identity} recovery bytes differ",
                    )

    asset_jobs = [
        (combination, identity, asset_id)
        for combination in COMBINATION_IDS
        for identity in identity_ids
        for asset_id in all_assets
    ]
    with ThreadPoolExecutor(max_workers=workers) as executor:
        for _ in executor.map(fetch_asset, asset_jobs):
            pass
    runner.validate_identity_coverage()
    runner.compare_all()
    if (
        file_sha256(comparator_path) != source_hashes["comparator_sha256"]
        or file_sha256(oracle_builder_path)
        != source_hashes["build_api_oracle_sha256"]
        or download_allowed_hosts() != frozen_download_allowed_hosts
    ):
        raise ValueError("G6 executable source or download allowlist changed")
    observations = sorted(
        runner.observations,
        key=lambda item: (
            COMBINATION_IDS.index(item["combination"]),
            item["identity"],
            item["route"],
            item["entity_key"],
        ),
    )
    violations = sorted(
        runner.violations,
        key=lambda item: (
            item["violation_code"],
            item["entity_key"],
            item["detail"],
        ),
    )
    result: dict[str, Any] = {
        "schema_version": 1,
        "run_id": run_id,
        "status": "PASS" if not violations else "BLOCKED",
        "task_count": len(tasks),
        "group_count": len(all_groups),
        "task_asset_count": len(all_assets),
        "legacy_task_asset_count": len(
            set().union(*dynamic_assets.values()) if dynamic_assets else set()
        ),
        "manifest_oracle_check_count": runner.manifest_oracle_check_count,
        "semantic_comparison_count": runner.semantic_comparison_count,
        "request_count": len(observations),
        "combination_matrix": combinations,
        "identities": identities,
        "task_ids_sha256": sha256(canonical(tasks)),
        "matrix_sha256": file_sha256(matrix_path),
        "rules_sha256": file_sha256(rules_path),
        "manifest_sha256": manifest_sha,
        "api_oracle_sha256": file_sha256(api_oracle_path),
        "api_oracle_mapping_sha256": api_oracle["reviewed_mapping_sha256"],
        "download_allowed_hosts": list(frozen_download_allowed_hosts),
        "download_allowed_hosts_sha256": sha256(
            canonical(frozen_download_allowed_hosts)
        ),
        **source_hashes,
        "used_rule_ids": sorted(runner.used_rules),
        "used_rule_applications": [
            {
                "rule_id": rule_id,
                "rule_identity": rule_identity,
                "identity": identity,
                "route": route,
                "direction": direction,
                "from_status": from_status,
                "to_status": to_status,
            }
            for (
                rule_id,
                rule_identity,
                identity,
                route,
                direction,
                from_status,
                to_status,
            ) in sorted(
                runner.used_rule_applications,
                key=lambda item: (
                    item[0],
                    item[1] or "",
                    item[2],
                    item[3],
                    item[4],
                    item[5],
                    item[6],
                ),
            )
        ],
        "unused_rule_ids": sorted(
            set(rule["rule_id"] for rule in rules) - runner.used_rules
        ),
        "observations": observations,
        "violation_count": len(violations),
        "violations": violations,
    }
    result["evidence_sha256"] = sha256(canonical(result))
    if request_metrics is not None:
        logical_count = runner.logical_request_count
        physical_count = runner.physical_request_count
        if (
            logical_count != result["request_count"]
            or physical_count < 0
            or physical_count > logical_count
        ):
            raise ValueError("request counters differ from logical observations")
        metrics: dict[str, Any] = {
            "schema_version": 1,
            "run_id": run_id,
            "cache_policy_version": REQUEST_CACHE_POLICY_VERSION,
            "logical_request_count": logical_count,
            "physical_request_count": physical_count,
            "deduplicated_request_count": logical_count - physical_count,
            "api_evidence_sha256": result["evidence_sha256"],
            "task_ids_sha256": result["task_ids_sha256"],
            "matrix_sha256": result["matrix_sha256"],
            "rules_sha256": result["rules_sha256"],
            "manifest_sha256": result["manifest_sha256"],
            "api_oracle_sha256": result["api_oracle_sha256"],
            "api_oracle_mapping_sha256": result[
                "api_oracle_mapping_sha256"
            ],
            "comparator_sha256": result["comparator_sha256"],
            "build_api_oracle_sha256": result[
                "build_api_oracle_sha256"
            ],
        }
        metrics["evidence_sha256"] = sha256(canonical(metrics))
        request_metrics.update(metrics)
    return result


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--matrix", type=pathlib.Path, required=True)
    parser.add_argument("--task-ids", type=pathlib.Path, required=True)
    parser.add_argument("--rules", type=pathlib.Path, required=True)
    parser.add_argument("--manifest", type=pathlib.Path, required=True)
    parser.add_argument("--api-oracle", type=pathlib.Path, required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument(
        "--request-metrics-output",
        type=pathlib.Path,
        help=(
            "optional hash-bound request-count sidecar; the primary G6 "
            "evidence contract remains unchanged"
        ),
    )
    parser.add_argument("--workers", type=int, default=16)
    args = parser.parse_args()
    if (
        args.request_metrics_output
        and args.request_metrics_output.resolve() == args.output.resolve()
    ):
        parser.error("request metrics output must differ from API evidence output")
    try:
        request_metrics: dict[str, Any] = {}
        result = compare(
            matrix_path=args.matrix,
            task_ids_path=args.task_ids,
            rules_path=args.rules,
            manifest_path=args.manifest,
            api_oracle_path=args.api_oracle,
            run_id=args.run_id,
            workers=args.workers,
            request_metrics=request_metrics,
        )
    except (OSError, ValueError, json.JSONDecodeError, urllib.error.URLError) as exc:
        result = {
            "schema_version": 1,
            "run_id": args.run_id,
            "status": "BLOCKED",
            "violation_count": 1,
            "violations": [
                {
                    "violation_code": "api.comparison_error",
                    "entity_key": "*",
                    "detail": str(exc),
                }
            ],
        }
        result["evidence_sha256"] = sha256(canonical(result))
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_bytes(canonical(result) + b"\n")
    if args.request_metrics_output and request_metrics:
        args.request_metrics_output.parent.mkdir(parents=True, exist_ok=True)
        args.request_metrics_output.write_bytes(canonical(request_metrics) + b"\n")
    raise SystemExit(0 if result["violation_count"] == 0 else 1)


if __name__ == "__main__":
    main()
