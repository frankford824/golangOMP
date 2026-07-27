#!/usr/bin/env python3
"""Build hash-bound source-alias allocation and Clone B apply receipts.

The expected allocation is derived only from the reviewed mapping and the
workflow snapshot's pre-apply AUTO_INCREMENT value.  Clone B rows are an
observed result: they must match the expected allocation row-for-row before an
apply receipt is emitted.
"""
from __future__ import annotations

import argparse
import csv
import hashlib
import json
import pathlib
import re
from typing import Any


SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
ACTUAL_FIELDS = (
    "alias_task_asset_id",
    "task_id",
    "scope_kind",
    "scope_ref_id",
    "group_id",
    "origin_task_asset_id",
    "root_asset_id",
    "storage_ref_id",
    "object_key_sha256",
    "content_sha256",
    "file_size",
    "mime_type",
    "scope_sku_code",
    "retouch_requirement_id",
    "asset_type",
    "binding_state",
    "bound_role",
    "flow_review_status",
    "source_module_key",
    "remark",
    "origin_root_asset_id",
    "origin_storage_ref_id",
    "origin_object_key_sha256",
    "origin_content_sha256",
    "origin_file_size",
    "origin_mime_type",
)


def canonical(value: object) -> bytes:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")


def go_canonical(value: object) -> bytes:
    """Approximate encoding/json Marshal for decoded snapshot evidence."""
    text = json.dumps(value, ensure_ascii=False, separators=(",", ":"))
    return (
        text.replace("&", "\\u0026")
        .replace("<", "\\u003c")
        .replace(">", "\\u003e")
        .encode("utf-8")
    )


def sha256(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def file_sha256(path: pathlib.Path) -> str:
    return sha256(path.read_bytes())


def require_sha256(value: object, label: str, *, allow_empty: bool = False) -> str:
    text = str(value or "")
    if allow_empty and not text:
        return ""
    if not SHA256_RE.fullmatch(text):
        raise ValueError(f"{label} must be a lowercase SHA-256")
    return text


def required_int(value: object, label: str, *, minimum: int = 1) -> int:
    if isinstance(value, bool):
        raise ValueError(f"{label} must be an integer")
    try:
        number = int(str(value))
    except (TypeError, ValueError) as exc:
        raise ValueError(f"{label} must be an integer") from exc
    if number < minimum:
        raise ValueError(f"{label} must be >= {minimum}")
    return number


def nullable_int(value: object, label: str) -> int | None:
    text = str(value or "").strip()
    if text in {"", "NULL", "\\N"}:
        return None
    return required_int(text, label)


def signed(document: dict[str, Any]) -> dict[str, Any]:
    if "evidence_sha256" in document:
        raise ValueError("unsigned document unexpectedly contains evidence_sha256")
    return {**document, "evidence_sha256": sha256(canonical(document))}


def verify_snapshot_integrity(snapshot: dict[str, Any]) -> None:
    integrity = require_sha256(
        snapshot.get("integrity_sha256"), "workflow snapshot integrity"
    )
    unsigned = dict(snapshot)
    unsigned["integrity_sha256"] = ""
    if sha256(go_canonical(unsigned)) != integrity:
        raise ValueError("workflow snapshot integrity hash mismatch")


def allocation_entries(
    mapping: dict[str, Any], first_alias_id: int
) -> list[dict[str, Any]]:
    resources = mapping.get("resources")
    if not isinstance(resources, list):
        raise ValueError("reviewed mapping lacks resources")
    output: list[dict[str, Any]] = []
    seen_keys: set[tuple[int, str, int, int]] = set()
    for resource_index, resource in enumerate(resources):
        if not isinstance(resource, dict):
            raise ValueError(f"resource {resource_index} is invalid")
        task_id = required_int(resource.get("task_id"), "resource task_id")
        scope_kind = str(resource.get("scope_kind") or "")
        scope_ref_id = required_int(
            resource.get("scope_ref_id"), "resource scope_ref_id", minimum=0
        )
        if scope_kind not in {"task", "sku", "retouch_requirement"}:
            raise ValueError(f"resource {resource_index} has invalid scope_kind")
        if (scope_kind == "task") != (scope_ref_id == 0):
            raise ValueError(f"resource {resource_index} has invalid task scope")
        history = resource.get("history")
        if not isinstance(history, list):
            raise ValueError(f"resource {resource_index} lacks history")
        for revision_index, revision in enumerate(history):
            if not isinstance(revision, dict):
                raise ValueError(
                    f"resource {resource_index} revision {revision_index} is invalid"
                )
            origin_value = revision.get("source_alias_from_task_asset_id")
            if origin_value is None:
                continue
            origin_id = required_int(
                origin_value,
                f"resource {resource_index} revision {revision_index} alias origin",
            )
            key = (task_id, scope_kind, scope_ref_id, origin_id)
            if key in seen_keys:
                continue
            seen_keys.add(key)
            sequence = len(output)
            output.append(
                {
                    "sequence": sequence,
                    "task_id": task_id,
                    "scope_kind": scope_kind,
                    "scope_ref_id": scope_ref_id,
                    "origin_task_asset_id": origin_id,
                    "expected_alias_task_asset_id": first_alias_id + sequence,
                    "canonical_locator": (
                        f"alias:v1:{task_id}:{scope_kind}:{scope_ref_id}:"
                        f"origin-task-asset:{origin_id}"
                    ),
                }
            )
    return output


def load_actual_rows(path: pathlib.Path) -> list[dict[str, Any]]:
    with path.open(encoding="utf-8", newline="") as handle:
        reader = csv.DictReader(handle, delimiter="\t")
        if tuple(reader.fieldnames or ()) != ACTUAL_FIELDS:
            raise ValueError("source-alias TSV field contract differs")
        rows = [dict(row) for row in reader]
    if not rows:
        raise ValueError("source-alias TSV is empty")
    return rows


def normalize_actual(row: dict[str, str], index: int) -> dict[str, Any]:
    label = f"source-alias TSV row {index + 2}"
    output: dict[str, Any] = {
        "alias_task_asset_id": required_int(
            row["alias_task_asset_id"], f"{label} alias_task_asset_id"
        ),
        "task_id": required_int(row["task_id"], f"{label} task_id"),
        "scope_kind": row["scope_kind"],
        "scope_ref_id": required_int(
            row["scope_ref_id"], f"{label} scope_ref_id", minimum=0
        ),
        "group_id": required_int(row["group_id"], f"{label} group_id"),
        "origin_task_asset_id": required_int(
            row["origin_task_asset_id"], f"{label} origin_task_asset_id"
        ),
        "root_asset_id": required_int(
            row["root_asset_id"], f"{label} root_asset_id"
        ),
        "storage_ref_id": row["storage_ref_id"],
        "object_key_sha256": require_sha256(
            row["object_key_sha256"],
            f"{label} object_key_sha256",
            allow_empty=True,
        ),
        "content_sha256": require_sha256(
            row["content_sha256"],
            f"{label} content_sha256",
            allow_empty=True,
        ),
        "file_size": required_int(
            row["file_size"], f"{label} file_size", minimum=0
        ),
        "mime_type": row["mime_type"],
        "scope_sku_code": row["scope_sku_code"],
        "retouch_requirement_id": nullable_int(
            row["retouch_requirement_id"], f"{label} retouch_requirement_id"
        ),
        "asset_type": row["asset_type"],
        "binding_state": row["binding_state"],
        "bound_role": row["bound_role"],
        "flow_review_status": row["flow_review_status"],
        "source_module_key": row["source_module_key"],
        "remark": row["remark"],
        "origin_root_asset_id": required_int(
            row["origin_root_asset_id"], f"{label} origin_root_asset_id"
        ),
        "origin_storage_ref_id": row["origin_storage_ref_id"],
        "origin_object_key_sha256": require_sha256(
            row["origin_object_key_sha256"],
            f"{label} origin_object_key_sha256",
            allow_empty=True,
        ),
        "origin_content_sha256": require_sha256(
            row["origin_content_sha256"],
            f"{label} origin_content_sha256",
            allow_empty=True,
        ),
        "origin_file_size": required_int(
            row["origin_file_size"], f"{label} origin_file_size", minimum=0
        ),
        "origin_mime_type": row["origin_mime_type"],
    }
    return output


def validate_actual(
    expected: list[dict[str, Any]], raw_rows: list[dict[str, str]]
) -> list[dict[str, Any]]:
    actual = [normalize_actual(row, index) for index, row in enumerate(raw_rows)]
    actual.sort(key=lambda row: row["alias_task_asset_id"])
    if len(actual) != len(expected):
        raise ValueError(
            f"source-alias apply count {len(actual)} differs from "
            f"expected {len(expected)}"
        )
    output: list[dict[str, Any]] = []
    for planned, observed in zip(expected, actual, strict=True):
        identity = {
            "alias_task_asset_id": observed["alias_task_asset_id"],
            "task_id": observed["task_id"],
            "scope_kind": observed["scope_kind"],
            "scope_ref_id": observed["scope_ref_id"],
            "origin_task_asset_id": observed["origin_task_asset_id"],
        }
        expected_identity = {
            "alias_task_asset_id": planned["expected_alias_task_asset_id"],
            "task_id": planned["task_id"],
            "scope_kind": planned["scope_kind"],
            "scope_ref_id": planned["scope_ref_id"],
            "origin_task_asset_id": planned["origin_task_asset_id"],
        }
        if identity != expected_identity:
            raise ValueError(
                f"source-alias apply sequence {planned['sequence']} "
                "does not match the deterministic allocation"
            )
        group_id = observed["group_id"]
        expected_remark = (
            f"v8-source-alias:group={group_id}:"
            f"origin={observed['origin_task_asset_id']}"
        )
        if (
            observed["asset_type"] != "source"
            or observed["binding_state"] != "bound"
            or observed["bound_role"] != "source"
            or observed["flow_review_status"] != "not_applicable"
            or observed["source_module_key"] != "migration"
            or observed["remark"] != expected_remark
        ):
            raise ValueError(
                f"source-alias apply sequence {planned['sequence']} "
                "role or lineage differs"
            )
        if (
            observed["root_asset_id"] != observed["origin_root_asset_id"]
            or observed["storage_ref_id"] != observed["origin_storage_ref_id"]
            or observed["object_key_sha256"]
            != observed["origin_object_key_sha256"]
            or observed["content_sha256"] != observed["origin_content_sha256"]
            or observed["file_size"] != observed["origin_file_size"]
            or observed["mime_type"] != observed["origin_mime_type"]
        ):
            raise ValueError(
                f"source-alias apply sequence {planned['sequence']} "
                "immutable origin identity differs"
            )
        if planned["scope_kind"] == "task" and (
            observed["scope_sku_code"]
            or observed["retouch_requirement_id"] is not None
        ):
            raise ValueError("task-scoped source alias has an asset scope")
        if planned["scope_kind"] == "sku" and (
            not observed["scope_sku_code"]
            or observed["retouch_requirement_id"] is not None
        ):
            raise ValueError("SKU-scoped source alias has an invalid asset scope")
        if planned["scope_kind"] == "retouch_requirement" and (
            observed["retouch_requirement_id"] != planned["scope_ref_id"]
        ):
            raise ValueError(
                "retouch-scoped source alias has an invalid requirement scope"
            )
        output.append(
            {
                **planned,
                "alias_task_asset_id": observed["alias_task_asset_id"],
                "group_id": group_id,
                "root_asset_id": observed["root_asset_id"],
                "storage_ref_id": observed["storage_ref_id"],
                "object_key_sha256": observed["object_key_sha256"],
                "content_sha256": observed["content_sha256"],
                "file_size": observed["file_size"],
                "mime_type": observed["mime_type"],
                "scope_sku_code": observed["scope_sku_code"],
                "retouch_requirement_id": observed["retouch_requirement_id"],
                "asset_type": observed["asset_type"],
                "binding_state": observed["binding_state"],
                "bound_role": observed["bound_role"],
                "flow_review_status": observed["flow_review_status"],
                "source_module_key": observed["source_module_key"],
                "remark": observed["remark"],
            }
        )
    return output


def build_receipts(
    *,
    run_id: str,
    mapping_path: pathlib.Path,
    expected_mapping_sha256: str,
    expected_mapping_canonical_sha256: str,
    workflow_snapshot_path: pathlib.Path,
    expected_workflow_snapshot_sha256: str,
    actual_aliases_tsv_path: pathlib.Path,
) -> tuple[dict[str, Any], dict[str, Any]]:
    require_sha256(expected_mapping_sha256, "expected mapping file SHA-256")
    require_sha256(
        expected_mapping_canonical_sha256,
        "expected canonical mapping SHA-256",
    )
    require_sha256(
        expected_workflow_snapshot_sha256,
        "expected workflow snapshot SHA-256",
    )
    if file_sha256(mapping_path) != expected_mapping_sha256:
        raise ValueError("reviewed mapping file hash differs")
    if file_sha256(workflow_snapshot_path) != expected_workflow_snapshot_sha256:
        raise ValueError("workflow snapshot file hash differs")
    mapping = json.loads(mapping_path.read_text(encoding="utf-8"))
    snapshot = json.loads(workflow_snapshot_path.read_text(encoding="utf-8"))
    if not isinstance(mapping, dict) or not isinstance(snapshot, dict):
        raise ValueError("mapping and workflow snapshot must be objects")
    verify_snapshot_integrity(snapshot)
    if (
        snapshot.get("mapping_sha256") != expected_mapping_canonical_sha256
        or snapshot.get("apply_state") != "applied"
        or snapshot.get("database") == ""
    ):
        raise ValueError("workflow snapshot mapping or apply binding differs")
    auto_rows = snapshot.get("auto_increments_before")
    if not isinstance(auto_rows, list):
        raise ValueError("workflow snapshot lacks pre-apply AUTO_INCREMENT")
    task_asset_values = [
        required_int(row.get("next_value"), "task_assets AUTO_INCREMENT")
        for row in auto_rows
        if isinstance(row, dict) and row.get("table") == "task_assets"
    ]
    if len(task_asset_values) != 1:
        raise ValueError("workflow snapshot task_assets AUTO_INCREMENT differs")
    entries = allocation_entries(mapping, task_asset_values[0])
    if not entries:
        raise ValueError("reviewed mapping contains no source aliases")
    allocation_unsigned: dict[str, Any] = {
        "schema_version": 1,
        "kind": "source_alias_allocation_v1",
        "status": "planned",
        "run_id": run_id,
        "database": snapshot["database"],
        "mapping_file_sha256": expected_mapping_sha256,
        "mapping_canonical_sha256": expected_mapping_canonical_sha256,
        "workflow_snapshot_file_sha256": expected_workflow_snapshot_sha256,
        "workflow_snapshot_integrity_sha256": snapshot["integrity_sha256"],
        "task_assets_auto_increment_before": task_asset_values[0],
        "entry_count": len(entries),
        "entries": entries,
    }
    allocation = signed(allocation_unsigned)
    allocation_file_bytes = canonical(allocation) + b"\n"
    actual = validate_actual(entries, load_actual_rows(actual_aliases_tsv_path))
    inserted_ids = snapshot.get("inserted_alias_asset_ids")
    expected_ids = [entry["expected_alias_task_asset_id"] for entry in entries]
    if inserted_ids != expected_ids:
        raise ValueError("workflow snapshot inserted alias IDs differ")
    apply_unsigned: dict[str, Any] = {
        "schema_version": 1,
        "kind": "source_alias_apply_v1",
        "status": "verified",
        "run_id": run_id,
        "database": snapshot["database"],
        "mapping_file_sha256": expected_mapping_sha256,
        "mapping_canonical_sha256": expected_mapping_canonical_sha256,
        "workflow_snapshot_file_sha256": expected_workflow_snapshot_sha256,
        "workflow_snapshot_integrity_sha256": snapshot["integrity_sha256"],
        "allocation_receipt_sha256": sha256(allocation_file_bytes),
        "actual_aliases_tsv_sha256": file_sha256(actual_aliases_tsv_path),
        "entry_count": len(actual),
        "entries": actual,
    }
    return allocation, signed(apply_unsigned)


def write_receipt(path: pathlib.Path, document: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(canonical(document) + b"\n")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--mapping", type=pathlib.Path, required=True)
    parser.add_argument("--expected-mapping-sha256", required=True)
    parser.add_argument("--expected-mapping-canonical-sha256", required=True)
    parser.add_argument("--workflow-snapshot", type=pathlib.Path, required=True)
    parser.add_argument("--expected-workflow-snapshot-sha256", required=True)
    parser.add_argument("--actual-aliases-tsv", type=pathlib.Path, required=True)
    parser.add_argument("--allocation-output", type=pathlib.Path, required=True)
    parser.add_argument("--apply-output", type=pathlib.Path, required=True)
    args = parser.parse_args(argv)
    try:
        allocation, apply = build_receipts(
            run_id=args.run_id,
            mapping_path=args.mapping,
            expected_mapping_sha256=args.expected_mapping_sha256,
            expected_mapping_canonical_sha256=(
                args.expected_mapping_canonical_sha256
            ),
            workflow_snapshot_path=args.workflow_snapshot,
            expected_workflow_snapshot_sha256=(
                args.expected_workflow_snapshot_sha256
            ),
            actual_aliases_tsv_path=args.actual_aliases_tsv,
        )
        write_receipt(args.allocation_output, allocation)
        write_receipt(args.apply_output, apply)
        return 0
    except (OSError, ValueError, KeyError, json.JSONDecodeError) as exc:
        print(str(exc))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
