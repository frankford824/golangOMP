#!/usr/bin/env python3
"""Derive the complete SQL-11 canonical entity set from frozen Clone A.

The database connection is restricted to loopback and runs one repeatable-read,
read-only transaction.  The script never connects to Clone B and never mutates
the database.  It predicts only the deterministic writes performed by
workflow-groups-migrate mapping v2.
"""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import ipaddress
import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile
from collections import defaultdict
from typing import Any, Iterable


SHA256 = re.compile(r"^[0-9a-f]{64}$")
RFC3339 = re.compile(
    r"^(?P<base>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})"
    r"(?:\.(?P<fraction>\d{1,6}))?"
    r"(?P<zone>Z|[+-]\d{2}:\d{2})$"
)
STATE_MAP = {
    "PendingAuditA": "PendingAudit",
    "PendingAuditB": "PendingAudit",
    "PendingCustomizationReview": "PendingAudit",
    "PendingOutsourceReview": "PendingAudit",
    "PendingEffectReview": "PendingAudit",
    "RejectedByAuditA": "InProgress",
    "RejectedByAuditB": "InProgress",
    "PendingCustomizationProduction": "InProgress",
    "PendingEffectRevision": "InProgress",
    "PendingOutsource": "InProgress",
    "Outsourcing": "InProgress",
    "PendingWarehouseQC": "Completed",
    "PendingWarehouseReceive": "Completed",
    "PendingProductionTransfer": "Completed",
    "PendingClose": "Completed",
}
GATE_METHOD = {
    "G01": "reviewed_mapping_a_truth",
    "G02": "reviewed_mapping_a_truth",
    "G03": "reviewed_mapping_a_truth",
    "G04": "reviewed_mapping_a_truth",
    "G05": "reviewed_mapping_a_truth",
    "G07": "immutable_a_truth",
    "G08": "reviewed_mapping_a_truth",
}

FREEZE_SQL = r"""
SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
SET SESSION TRANSACTION READ ONLY;
START TRANSACTION WITH CONSISTENT SNAPSHOT, READ ONLY;
SELECT CONCAT('meta	',JSON_OBJECT(
  'task_count',(SELECT COUNT(*) FROM tasks),
  'group_count',(SELECT COUNT(*) FROM task_asset_groups),
  'revision_count',(SELECT COUNT(*) FROM task_asset_group_revisions),
  'max_task_asset_id',COALESCE((SELECT MAX(id) FROM task_assets),0)));
SELECT CONCAT('task	',JSON_OBJECT(
  'id',id,'task_type',task_type,'task_status',task_status,
  'current_handler_id',current_handler_id,'workflow_revision',workflow_revision))
FROM tasks ORDER BY id;
SELECT CONCAT('group	',JSON_OBJECT(
  'id',id,'task_id',task_id,'scope_kind',scope_kind,'scope_ref_id',scope_ref_id,
  'migration_incomplete',migration_incomplete,'migration_issue',migration_issue))
FROM task_asset_groups ORDER BY task_id,scope_kind,scope_ref_id,id;
SELECT CONCAT('asset	',JSON_OBJECT(
  'id',id,'asset_id',asset_id,'task_id',task_id,'asset_type',asset_type,
  'storage_ref_id',COALESCE(storage_ref_id,''),'whole_hash',COALESCE(whole_hash,''),
  'binding_state',binding_state,'bound_role',bound_role,'scope_sku_code',COALESCE(scope_sku_code,''),
  'retouch_requirement_id',retouch_requirement_id))
FROM task_assets ORDER BY id;
SELECT CONCAT('sku	',JSON_OBJECT('id',id,'task_id',task_id,'sku_code',COALESCE(sku_code,'')))
FROM task_sku_items ORDER BY id;
SELECT CONCAT('reference	',JSON_OBJECT(
  'id',r.id,'task_id',r.task_id,'ref_id',r.ref_id,'sku_item_id',r.sku_item_id,
  'retouch_requirement_id',r.retouch_requirement_id,'file_name',COALESCE(s.file_name,'')))
FROM reference_file_refs r LEFT JOIN asset_storage_refs s ON s.ref_id=r.ref_id ORDER BY r.id;
SELECT CONCAT('task_event	',JSON_OBJECT(
  'id',id,'task_id',task_id,'sequence',sequence,'event_type',event_type,
  'operator_id',operator_id,'payload_text',CAST(payload AS CHAR),
  'created_at_text',DATE_FORMAT(created_at,'%Y-%m-%dT%H:%i:%s.%f')))
FROM task_event_logs ORDER BY task_id,sequence,id;
SELECT CONCAT('module_event	',JSON_OBJECT(
  'id',id,'task_module_id',task_module_id,'event_type',event_type,
  'from_state',from_state,'to_state',to_state,'actor_id',actor_id,
  'actor_snapshot_text',CAST(actor_snapshot AS CHAR),'payload_text',CAST(payload AS CHAR),
  'created_at_text',DATE_FORMAT(created_at,'%Y-%m-%dT%H:%i:%s.%f')))
FROM task_module_events ORDER BY task_module_id,id;
SELECT CONCAT('planning_revision	',JSON_OBJECT(
  'task_sku_item_id',r.task_sku_item_id,'version_no',r.version_no,
  'description_spec',r.description_spec,'quantity',r.quantity,
  'target_price',CAST(r.target_price AS CHAR),'currency',r.currency,'note',r.note,
  'reference_url',r.reference_url,'erp_product_i_id',r.erp_product_i_id,
  'erp_product_name',r.erp_product_name,'reason',r.reason,'created_by',r.created_by,
  'image_storage_ref_id',i.storage_ref_id))
FROM task_planning_sku_revisions r
LEFT JOIN task_planning_sku_revision_images i ON i.revision_id=r.id
ORDER BY r.task_sku_item_id,r.version_no,i.storage_ref_id;
SELECT CONCAT('planning_setting	',JSON_OBJECT('task_id',task_id))
FROM task_planning_settings ORDER BY task_id;
SELECT CONCAT('retouch	',JSON_OBJECT(
  'id',id,'task_id',task_id,'description',description,'sku_code',sku_code,'spec',spec,
  'remark',remark,'sort_order',sort_order,'deleted',IF(deleted_at IS NULL,0,1)))
FROM task_retouch_requirements ORDER BY task_id,id;
COMMIT;
"""


def canonical_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_file(path: pathlib.Path) -> str:
    return sha256_bytes(path.read_bytes())


def scalar(value: Any) -> str:
    return "" if value is None else str(value)


def confirmed_time(value: Any) -> bool:
    return isinstance(value, str) and bool(value) and not value.startswith("0001-01-01T00:00:00")


def is_loopback(host: str) -> bool:
    if host.lower() == "localhost":
        return True
    try:
        return ipaddress.ip_address(host).is_loopback
    except ValueError:
        return False


def parse_json(path: pathlib.Path, label: str) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{label} must be a JSON object")
    return value


def validate_inputs(
    mapping: dict[str, Any],
    baseline: dict[str, Any],
    decisions: dict[str, Any],
    object_verdict: dict[str, Any],
) -> str:
    if mapping.get("version") != 2:
        raise ValueError("reviewed mapping must be version 2")
    if not isinstance(mapping.get("resources"), list) or not isinstance(mapping.get("planning_tasks", []), list):
        raise ValueError("reviewed mapping has invalid resource/planning collections")
    seen_scopes: set[tuple[int, str, int]] = set()
    for group_index, group in enumerate(mapping["resources"]):
        scope = (int(group["task_id"]), str(group["scope_kind"]), int(group["scope_ref_id"]))
        if scope in seen_scopes:
            raise ValueError(f"duplicate resource scope {scope}")
        seen_scopes.add(scope)
        history = group.get("history")
        if not isinstance(history, list):
            raise ValueError(f"resources[{group_index}] must declare history")
        for revision_index, revision in enumerate(history):
            path = f"resources[{group_index}].history[{revision_index}]"
            if revision.get("confidence") != "confirmed_auto" or revision.get("blockers"):
                raise ValueError(f"{path} is not a blocker-free confirmed_auto row")
            if not revision.get("confirmed_by") or not confirmed_time(revision.get("confirmed_at")) or not revision.get("confirmation_note"):
                raise ValueError(f"{path} has incomplete confirmation metadata")
            if not SHA256.fullmatch(str(revision.get("manifest_row_hash", ""))):
                raise ValueError(f"{path} has no valid manifest_row_hash")
    for index, planning in enumerate(mapping.get("planning_tasks", [])):
        if planning.get("confidence") != "confirmed_auto" or planning.get("blockers"):
            raise ValueError(f"planning_tasks[{index}] is not a blocker-free confirmed_auto row")
        if not planning.get("confirmed_by") or not confirmed_time(planning.get("confirmed_at")) or not planning.get("confirmation_note"):
            raise ValueError(f"planning_tasks[{index}] has incomplete confirmation metadata")
    for index, decision in enumerate(mapping.get("task_state_decisions", [])):
        if not decision.get("confirmed_by") or not confirmed_time(decision.get("confirmed_at")) or not decision.get("confirmation_note"):
            raise ValueError(f"task_state_decisions[{index}] has incomplete confirmation metadata")
        if not SHA256.fullmatch(str(decision.get("manifest_row_hash", ""))):
            raise ValueError(f"task_state_decisions[{index}] has no valid manifest_row_hash")
    snapshot = baseline.get("snapshot_sha256")
    if not isinstance(snapshot, str) or not SHA256.fullmatch(snapshot):
        raise ValueError("baseline attestation has no valid snapshot_sha256")
    fingerprint = baseline.get("baseline_fingerprint_sha256")
    if not isinstance(fingerprint, str) or not SHA256.fullmatch(fingerprint):
        raise ValueError("baseline attestation has no valid baseline_fingerprint_sha256")
    if decisions.get("decision") != "confirmed":
        raise ValueError("approved decisions must contain decision=confirmed")
    if object_verdict.get("status") != "PASS" or object_verdict.get("violation_count") != 0:
        raise ValueError("object/projection verdict must be PASS with violation_count=0")
    return snapshot


def run_freeze(args: argparse.Namespace) -> tuple[dict[str, list[dict[str, Any]]], str]:
    if not is_loopback(args.host):
        raise ValueError("--host must be loopback; only frozen Clone A is allowed")
    if not args.database or args.database in {"mysql", "information_schema", "performance_schema", "sys"}:
        raise ValueError("--database must name an application clone")
    command = [
        args.mysql,
        f"--host={args.host}",
        f"--port={args.port}",
        f"--user={args.user}",
        "--default-character-set=utf8mb4",
        "--batch",
        "--raw",
        "--skip-column-names",
        args.database,
    ]
    if args.defaults_extra_file:
        command.insert(1, f"--defaults-extra-file={args.defaults_extra_file}")
    completed = subprocess.run(command, input=FREEZE_SQL, text=True, capture_output=True, check=False)
    if completed.returncode:
        raise RuntimeError(f"mysql read-only canonical freeze failed: {completed.stderr.strip()}")
    rows: dict[str, list[dict[str, Any]]] = defaultdict(list)
    allowed = {
        "meta", "task", "group", "asset", "sku", "reference", "task_event",
        "module_event", "planning_revision", "planning_setting", "retouch",
    }
    normalized: list[tuple[str, dict[str, Any]]] = []
    for line_no, line in enumerate(completed.stdout.splitlines(), 1):
        if not line:
            continue
        try:
            kind, raw = line.split("\t", 1)
            value = json.loads(raw)
        except (ValueError, json.JSONDecodeError) as exc:
            raise RuntimeError(f"unexpected mysql output at line {line_no}") from exc
        if kind not in allowed or not isinstance(value, dict):
            raise RuntimeError(f"unexpected mysql record kind at line {line_no}")
        rows[kind].append(value)
        normalized.append((kind, value))
    if len(rows["meta"]) != 1 or not rows["task"]:
        raise RuntimeError("frozen Clone A returned incomplete metadata/tasks")
    freeze_hash = sha256_bytes(
        "".join(f"{kind}\t{canonical_json(value)}\n" for kind, value in normalized).encode("utf-8")
    )
    return rows, freeze_hash


def parse_rfc3339(value: Any, label: str) -> dt.datetime:
    if not isinstance(value, str):
        raise ValueError(f"{label} must be an RFC3339 string")
    match = RFC3339.fullmatch(value)
    if match is None:
        raise ValueError(f"{label} must be an RFC3339 string")
    fraction = match.group("fraction")
    normalized = match.group("base")
    if fraction is not None:
        normalized += "." + fraction.ljust(6, "0")
    normalized += (
        "+00:00" if match.group("zone") == "Z" else match.group("zone")
    )
    parsed = dt.datetime.fromisoformat(normalized)
    if parsed.tzinfo is None:
        raise ValueError(f"{label} must include a timezone")
    return parsed


def mysql_time(value: Any) -> str:
    if value in (None, ""):
        return ""
    parsed = parse_rfc3339(value, "revision time")
    return parsed.astimezone(dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%f")


def persisted_reason(revision: dict[str, Any]) -> str:
    evidence = sorted(str(item) for item in revision.get("evidence_event_ids", []))
    confirmed_at = (
        parse_rfc3339(revision["confirmed_at"], "revision confirmed_at")
        .astimezone(dt.timezone.utc)
        .isoformat(timespec="seconds")
        .replace("+00:00", "Z")
    )
    metadata = (
        f"[migration_v2 manifest={revision['manifest_row_hash']} "
        f"confidence={revision['confidence']} confirmed_by={int(revision['confirmed_by'])} "
        f"confirmed_at={confirmed_at} evidence_count={len(evidence)}"
    )
    if evidence:
        metadata += f" first_evidence={evidence[0]}"
    metadata += "]"
    original_reason = str(revision.get("reason", "")).strip()
    reason = " ".join(part for part in (original_reason, metadata) if part)
    if len(reason) <= 512:
        return reason
    compact = (
        f"[migration_v2 manifest={revision['manifest_row_hash']} "
        f"reason_sha256={sha256_bytes(original_reason.encode('utf-8'))} "
        f"confidence={revision['confidence']} confirmed_by={int(revision['confirmed_by'])} "
        f"confirmed_at={confirmed_at} evidence_count={len(evidence)}"
    )
    if evidence:
        compact += f" first_evidence={evidence[0]}"
    compact += "]"
    if len(compact) > 512:
        raise ValueError(
            "revision evidence cannot fit task_asset_group_revisions.reason"
        )
    return compact


def stable_source_identity(
    asset: dict[str, Any] | None,
    bundle_sha256: str | None = None,
) -> str:
    """Return the source identity that survives rollback/re-apply ID drift."""
    if bundle_sha256 is not None:
        if not SHA256.fullmatch(bundle_sha256):
            raise ValueError("source bundle has no valid bundle_sha256")
        return f"bundle:{bundle_sha256}"
    if asset is None:
        return ""
    asset_root = asset.get("asset_id")
    storage_ref = str(asset.get("storage_ref_id") or "")
    if asset_root is None or int(asset_root) <= 0 or not storage_ref:
        raise ValueError(
            f"source task asset {asset.get('id')} lacks stable asset/storage identity"
        )
    return f"asset:{int(asset_root)}:{storage_ref}"


def entity(
    gate: str,
    key: str,
    components: Iterable[Any],
    freeze_hash: str,
    detail: dict[str, Any] | None = None,
) -> dict[str, Any]:
    return {
        "gate_name": gate,
        "entity_key": key,
        "expected_state": "matched",
        "review_state": "pass",
        "derivation_method": GATE_METHOD[gate],
        "components": [scalar(value) for value in components],
        "detail": {"frozen_clone_a_sha256": freeze_hash, **(detail or {})},
    }


def unique_index(rows: list[dict[str, Any]], field: str, kind: str) -> dict[int, dict[str, Any]]:
    result: dict[int, dict[str, Any]] = {}
    for row in rows:
        identity = int(row[field])
        if identity in result:
            raise ValueError(f"duplicate frozen {kind} identity {identity}")
        result[identity] = row
    return result


def scope_values(group: dict[str, Any], skus: dict[int, dict[str, Any]]) -> tuple[str, str]:
    kind, ref = str(group["scope_kind"]), int(group["scope_ref_id"])
    if kind == "sku":
        sku = skus.get(ref)
        if not sku or int(sku["task_id"]) != int(group["task_id"]):
            raise ValueError(f"scope sku {ref} is absent or belongs to another task")
        return scalar(sku.get("sku_code")), ""
    if kind == "retouch_requirement":
        return "", str(ref)
    if kind != "task" or ref != 0:
        raise ValueError(f"unsupported resource scope {kind}/{ref}")
    return "", ""


def validate_projection(path: pathlib.Path) -> list[dict[str, Any]]:
    result = []
    seen: set[str] = set()
    for line_no, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if not line:
            continue
        value = json.loads(line)
        if (
            not isinstance(value, dict)
            or value.get("gate_name") != "G09"
            or value.get("derivation_method") != "independent_projection"
            or value.get("review_state") != "pass"
            or not isinstance(value.get("components"), list)
            or not value.get("entity_key")
        ):
            raise ValueError(f"projection expected row {line_no} is not an independently derived PASS entity")
        if value["entity_key"] in seen:
            raise ValueError(f"duplicate projection entity {value['entity_key']}")
        seen.add(value["entity_key"])
        result.append(value)
    if not result:
        raise ValueError("projection expected JSONL has no G09 entities")
    return result


def build_entities(
    mapping: dict[str, Any],
    rows: dict[str, list[dict[str, Any]]],
    freeze_hash: str,
    projection_entities: list[dict[str, Any]],
    decisions: dict[str, Any],
    object_verdict: dict[str, Any],
) -> list[dict[str, Any]]:
    meta = rows["meta"][0]
    if int(meta.get("task_count", -1)) != len(rows["task"]):
        raise ValueError("frozen task count does not match metadata")
    if int(meta.get("group_count", -1)) != len(rows["group"]):
        raise ValueError("frozen group count does not match metadata")
    if int(meta.get("revision_count", -1)) != 0:
        raise ValueError("Clone A already contains resource revisions; deterministic replay prediction is unsafe")

    tasks = unique_index(rows["task"], "id", "task")
    assets = unique_index(rows["asset"], "id", "asset")
    for recovery in mapping.get("asset_recoveries", []):
        if recovery.get("strategy") != "clone_b_prematerialized_storage_ref_v1":
            continue
        recovered_id = int(recovery.get("missing_task_asset_id") or 0)
        recovered_hash = str(recovery.get("recovery_source_sha256") or "")
        recovered = assets.get(recovered_id)
        if recovered is None or not SHA256.fullmatch(recovered_hash):
            raise ValueError(
                f"asset recovery {recovered_id} lacks a frozen task asset or valid source hash"
            )
        recovered["whole_hash"] = recovered_hash
    skus = unique_index(rows["sku"], "id", "sku")
    references = unique_index(rows["reference"], "id", "reference")
    groups_by_scope: dict[tuple[int, str, int], dict[str, Any]] = {}
    for row in rows["group"]:
        key = (int(row["task_id"]), str(row["scope_kind"]), int(row["scope_ref_id"]))
        if key in groups_by_scope:
            raise ValueError(f"duplicate frozen resource group scope {key}")
        groups_by_scope[key] = row
    mapping_scopes = {
        (int(group["task_id"]), str(group["scope_kind"]), int(group["scope_ref_id"]))
        for group in mapping["resources"]
    }
    unmapped = sorted(set(groups_by_scope) - mapping_scopes)
    if unmapped:
        raise ValueError(
            "resource mapping/frozen group coverage differs: "
            f"unmapped={unmapped[:5]}"
        )

    decisions_by_task: dict[int, dict[str, Any]] = {}
    for decision in mapping.get("task_state_decisions", []):
        task_id = int(decision["task_id"])
        if task_id in decisions_by_task:
            raise ValueError(f"duplicate task state decision {task_id}")
        decisions_by_task[task_id] = decision

    status_by_task: dict[int, str] = {}
    revision_by_task: dict[int, int] = {}
    type_by_task: dict[int, str] = {}
    for task_id, row in tasks.items():
        old = str(row["task_status"])
        decision = decisions_by_task.get(task_id)
        if decision is not None:
            if old != str(decision["from_status"]):
                raise ValueError(f"task state decision {task_id} no longer matches its from_status")
            status_by_task[task_id] = str(decision["target_status"])
            revision_by_task[task_id] = int(row["workflow_revision"]) + 1
        else:
            status_by_task[task_id] = STATE_MAP.get(old, old)
            revision_by_task[task_id] = int(row["workflow_revision"]) + (1 if old in STATE_MAP else 0)
        type_by_task[task_id] = str(row["task_type"])
    existing_planning = {int(row["task_id"]) for row in rows["planning_setting"]}
    for planning in mapping.get("planning_tasks", []):
        task_id = int(planning["task_id"])
        if task_id not in tasks:
            raise ValueError(f"planning task {task_id} is absent from Clone A")
        if task_id in existing_planning:
            raise ValueError(f"planning task {task_id} already has settings in Clone A; idempotent state cannot be predicted from mapping alone")
        if type_by_task[task_id] != "purchase_task":
            raise ValueError(f"planning task {task_id} is not purchase_task in Clone A")
        type_by_task[task_id] = "sku_planning"
        status_by_task[task_id] = str(planning["target_task_status"])
        revision_by_task[task_id] += 1

    result: list[dict[str, Any]] = []
    for task_id in sorted(tasks):
        row = tasks[task_id]
        result.append(entity("G01", f"task:{task_id}", [
            task_id, type_by_task[task_id], status_by_task[task_id],
            row.get("current_handler_id"), revision_by_task[task_id],
        ], freeze_hash))

    # Generated source-alias surrogate IDs are deliberately excluded from the
    # canonical contract. InnoDB does not rewind AUTO_INCREMENT on rollback, so
    # a rehearsal/re-apply may assign different row IDs while preserving the
    # exact same immutable asset root and storage ref.
    bundle_by_id: dict[int, dict[str, Any]] = {}
    for group in mapping["resources"]:
        for revision in group["history"]:
            bundle = revision.get("source_bundle")
            if bundle:
                bundle_id = int(bundle["task_asset_id"])
                if bundle_id in assets or bundle_id in bundle_by_id:
                    raise ValueError(f"source bundle task_asset_id {bundle_id} is not a fresh unique ID")
                if not SHA256.fullmatch(str(bundle.get("bundle_sha256", ""))):
                    raise ValueError(f"source bundle {bundle_id} has no valid bundle_sha256")
                bundle_by_id[bundle_id] = bundle
    asset_roles: dict[int, tuple[tuple[int, str, int], str]] = {}
    for group in mapping["resources"]:
        scope = (int(group["task_id"]), str(group["scope_kind"]), int(group["scope_ref_id"]))
        for revision in group["history"]:
            direct = revision.get("source_task_asset_id")
            if direct is not None:
                identity = int(direct)
                prior = asset_roles.setdefault(identity, (scope, "source"))
                if prior != (scope, "source"):
                    raise ValueError(f"task asset {identity} has conflicting mapped roles/scopes")
            for final_id_raw in revision.get("final_task_asset_ids", []):
                identity = int(final_id_raw)
                prior = asset_roles.setdefault(identity, (scope, "final"))
                if prior != (scope, "final"):
                    raise ValueError(f"task asset {identity} has conflicting mapped roles/scopes")

    for group in mapping["resources"]:
        task_id, kind, ref = int(group["task_id"]), str(group["scope_kind"]), int(group["scope_ref_id"])
        scope = (task_id, kind, ref)
        if task_id not in tasks:
            raise ValueError(f"resource task {task_id} is absent from Clone A")
        by_no = {int(revision["revision_no"]): revision for revision in group["history"]}
        if sorted(by_no) != list(range(1, len(by_no) + 1)):
            raise ValueError(f"resource {scope} revision numbers are not contiguous from 1")
        working_no = group.get("working_revision_no")
        finalized_no = group.get("finalized_revision_no")
        if working_no is not None and int(working_no) not in by_no:
            raise ValueError(f"resource {scope} has an invalid working pointer")
        if finalized_no is not None and int(finalized_no) not in by_no:
            raise ValueError(f"resource {scope} has an invalid finalized pointer")
        result.append(entity("G02", f"group:{task_id}:{kind}:{ref}", [
            task_id, kind, ref,
            working_no, by_no[int(working_no)]["status"] if working_no is not None else "",
            finalized_no, by_no[int(finalized_no)]["status"] if finalized_no is not None else "",
            0, "",
        ], freeze_hash))
        sku_code, retouch_id = scope_values(group, skus)
        for revision_no in sorted(by_no):
            revision = by_no[revision_no]
            source_asset: dict[str, Any] | None = None
            source_identity = ""
            if revision.get("source_task_asset_id") is not None:
                source_id = int(revision["source_task_asset_id"])
                source_asset = assets.get(source_id)
                if not source_asset or int(source_asset["task_id"]) != task_id:
                    raise ValueError(
                        f"source task asset {source_id} is absent or belongs to another task"
                    )
                source_identity = stable_source_identity(source_asset)
            elif revision.get("source_bundle"):
                source_identity = stable_source_identity(
                    None, str(revision["source_bundle"]["bundle_sha256"])
                )
            elif revision.get("source_alias_from_task_asset_id") is not None:
                origin_id = int(revision["source_alias_from_task_asset_id"])
                source_asset = assets.get(origin_id)
                if not source_asset or int(source_asset["task_id"]) != task_id:
                    raise ValueError(
                        f"source alias origin {origin_id} is absent or belongs to another task"
                    )
                source_identity = stable_source_identity(source_asset)
            result.append(entity("G03", f"revision:{task_id}:{kind}:{ref}:{revision_no}", [
                task_id, kind, ref, revision_no, revision["status"], revision["mode"],
                source_identity, revision["source_stage"], revision["created_by"], persisted_reason(revision),
                mysql_time(revision.get("submitted_at")), mysql_time(revision.get("finalized_at")),
            ], freeze_hash, {"manifest_row_hash": revision["manifest_row_hash"]}))

            if not source_identity:
                source_components = ["", "", "", "", "", "", ""]
            elif revision.get("source_bundle"):
                source_components = [
                    source_identity, "source",
                    revision["source_bundle"]["bundle_sha256"],
                    "bound", "source", sku_code, retouch_id,
                ]
            elif revision.get("source_alias_from_task_asset_id") is not None:
                source_components = [
                    source_identity, "source", source_asset.get("whole_hash"),
                    "bound", "source", sku_code, retouch_id,
                ]
            else:
                source_components = [
                    source_identity, source_asset["asset_type"],
                    source_asset.get("whole_hash"), "bound", "source",
                    sku_code, retouch_id,
                ]
            result.append(entity("G04", f"revision-source:{task_id}:{kind}:{ref}:{revision_no}", source_components, freeze_hash))

            for order, final_id_raw in enumerate(revision.get("final_task_asset_ids", [])):
                final_id = int(final_id_raw)
                original = assets.get(final_id)
                if not original or int(original["task_id"]) != task_id:
                    raise ValueError(f"final task asset {final_id} is absent or belongs to another task")
                result.append(entity("G04", f"revision-final:{task_id}:{kind}:{ref}:{revision_no}:{order}", [
                    final_id, order, "", original["asset_type"], original.get("whole_hash"),
                    "bound", "final", sku_code, retouch_id,
                ], freeze_hash))
            for order, reference_id_raw in enumerate(revision.get("reference_file_ref_ids", [])):
                reference_id = int(reference_id_raw)
                reference = references.get(reference_id)
                if not reference or int(reference["task_id"]) != task_id:
                    raise ValueError(f"reference {reference_id} is absent or belongs to another task")
                scope_snapshot = (
                    f"retouch_requirement:{int(reference['retouch_requirement_id'])}"
                    if reference.get("retouch_requirement_id") is not None
                    else f"sku:{int(reference['sku_item_id'])}"
                    if reference.get("sku_item_id") is not None
                    else "task"
                )
                result.append(entity("G05", f"revision-reference:{task_id}:{kind}:{ref}:{revision_no}:{order}", [
                    reference_id, "", order, reference["ref_id"], reference.get("file_name"), scope_snapshot,
                ], freeze_hash))

    for row in rows["task_event"]:
        key = f"task-event:{int(row['task_id'])}:{int(row['sequence'])}"
        result.append(entity("G07", key, [
            row["id"], row["task_id"], row["sequence"], row["event_type"],
            row.get("operator_id"), row["payload_text"], row["created_at_text"],
        ], freeze_hash))
    for row in rows["module_event"]:
        key = f"module-event:{int(row['task_module_id'])}:{int(row['id'])}"
        result.append(entity("G07", key, [
            row["id"], row["task_module_id"], row["event_type"], row.get("from_state"),
            row.get("to_state"), row.get("actor_id"), row.get("actor_snapshot_text"),
            row["payload_text"], row["created_at_text"],
        ], freeze_hash))

    planning_keys: set[tuple[int, int]] = set()
    for row in rows["planning_revision"]:
        key = (int(row["task_sku_item_id"]), int(row["version_no"]))
        if key in planning_keys:
            raise ValueError(f"planning revision {key} has duplicate image/rows and cannot match SQL 11 uniquely")
        planning_keys.add(key)
        result.append(entity("G08", f"planning-revision:{key[0]}:{key[1]}", [
            key[0], key[1], row["description_spec"], row["quantity"], row.get("target_price"),
            row["currency"], row["note"], row["reference_url"], row["erp_product_i_id"],
            row["erp_product_name"], row["reason"], row["created_by"], row.get("image_storage_ref_id"),
        ], freeze_hash))
    for planning in mapping.get("planning_tasks", []):
        policies = set(planning.get("review_policy_ids", []))
        if "legacy_incomplete_uat_planning_tombstone_v1" in policies:
            continue
        for item in planning.get("items", []):
            key = (int(item["task_sku_item_id"]), 1)
            if key in planning_keys:
                raise ValueError(f"new planning revision {key} conflicts with Clone A")
            planning_keys.add(key)
            result.append(entity("G08", f"planning-revision:{key[0]}:1", [
                key[0], 1, item["description_spec"], item["quantity"], item.get("target_price"),
                "CNY", item.get("note", ""), item.get("reference_url", ""),
                item.get("erp_product_i_id", ""), item.get("erp_product_name", ""),
                "confirmed legacy planning migration", planning["created_by"],
                item.get("image_storage_ref_id", ""),
            ], freeze_hash, {
                "migration_created": True,
                "task_id": task_id,
            }))
    for row in rows["retouch"]:
        result.append(entity("G08", f"retouch-requirement:{int(row['task_id'])}:{int(row['id'])}", [
            row["task_id"], row["id"], row["description"], row.get("sku_code"), row.get("spec"),
            row.get("remark"), row["sort_order"], row["deleted"],
        ], freeze_hash))

    result.extend(projection_entities)
    g06_detail = {
        "derivation_method": "object_verifier",
        "verdict": "PASS",
        "violation_count": 0,
        "source_status": object_verdict["status"],
    }
    result.append({
        "gate_name": "G06", "entity_key": "object-verdict", "expected_state": "verified",
        "review_state": "pass", "derivation_method": "object_verifier", "components": [],
        "detail": g06_detail,
    })
    result.append({
        "gate_name": "G10", "entity_key": "release-decision", "expected_state": "confirmed",
        "review_state": "pass", "derivation_method": "human_decision", "components": [],
        "detail": {
            "derivation_method": "human_decision",
            "decision": "confirmed",
            "reviewer_id": decisions.get("reviewer_id"),
            "confirmed_at": decisions.get("confirmed_at"),
        },
    })

    result.sort(key=lambda item: (str(item["gate_name"]), str(item["entity_key"])))
    seen_entities: set[tuple[str, str]] = set()
    for item in result:
        natural = (str(item["gate_name"]), str(item["entity_key"]))
        if natural in seen_entities:
            raise ValueError(f"duplicate canonical entity {natural}")
        seen_entities.add(natural)
    required = {"G01", "G02", "G03", "G04", "G05", "G06", "G07", "G08", "G09", "G10"}
    actual = {str(item["gate_name"]) for item in result}
    if actual != required:
        raise ValueError(f"canonical entities missing gates: {sorted(required - actual)}")
    return result


def atomic_write(path: pathlib.Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    except BaseException:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise


def build(args: argparse.Namespace) -> None:
    paths = {
        "mapping_sha256": pathlib.Path(args.mapping),
        "baseline_attestation_sha256": pathlib.Path(args.baseline_attestation),
        "approved_decisions_sha256": pathlib.Path(args.approved_decisions),
        "object_verdict_sha256": pathlib.Path(args.object_verdict),
        "projection_expected_sha256": pathlib.Path(args.projection_expected),
    }
    mapping = parse_json(paths["mapping_sha256"], "reviewed mapping")
    baseline = parse_json(paths["baseline_attestation_sha256"], "baseline attestation")
    decisions = parse_json(paths["approved_decisions_sha256"], "approved decisions")
    object_verdict = parse_json(paths["object_verdict_sha256"], "object verdict")
    validate_inputs(mapping, baseline, decisions, object_verdict)
    projection = validate_projection(paths["projection_expected_sha256"])
    rows, freeze_hash = run_freeze(args)
    entities = build_entities(mapping, rows, freeze_hash, projection, decisions, object_verdict)
    document = {
        "schema_version": 1,
        "input_sha256": {key: sha256_file(path) for key, path in paths.items()},
        "entities": entities,
    }
    atomic_write(pathlib.Path(args.output), canonical_json(document) + "\n")


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mapping", required=True)
    parser.add_argument("--baseline-attestation", required=True)
    parser.add_argument("--approved-decisions", required=True)
    parser.add_argument("--object-verdict", required=True)
    parser.add_argument("--projection-expected", required=True)
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=3306)
    parser.add_argument("--user", required=True)
    parser.add_argument("--database", required=True)
    parser.add_argument("--defaults-extra-file")
    parser.add_argument("--mysql", default="mysql")
    parser.add_argument("--output", required=True)
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    try:
        build(parse_args(argv))
    except (OSError, UnicodeDecodeError, ValueError, RuntimeError, json.JSONDecodeError) as exc:
        print(str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
