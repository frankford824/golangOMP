#!/usr/bin/env python3
"""Freeze A-side projection inputs and derive independent G09 expectations.

The build path never connects to clone B and never reads either search-document
table.  It implements the source SELECTs used by the Go rebuild code.  Where
the Go query has no declared ordering (task asset/planning GROUP_CONCAT), the
affected entity is retained as ``hard_blocked`` instead of guessing an order.
"""

from __future__ import annotations

import argparse
import hashlib
import ipaddress
import json
import pathlib
import re
import subprocess
import sys
from collections import defaultdict
from typing import Any, Iterable

try:
    from scripts.ab import apply_bundle_registry_to_mapping as bundle_registry
except ModuleNotFoundError:
    import apply_bundle_registry_to_mapping as bundle_registry


STATE_MAP = {
    "PendingAuditA": "PendingAudit", "PendingAuditB": "PendingAudit",
    "PendingCustomizationReview": "PendingAudit", "PendingOutsourceReview": "PendingAudit",
    "PendingEffectReview": "PendingAudit", "RejectedByAuditA": "InProgress",
    "RejectedByAuditB": "InProgress", "PendingCustomizationProduction": "InProgress",
    "PendingEffectRevision": "InProgress", "PendingOutsource": "InProgress",
    "Outsourcing": "InProgress", "PendingWarehouseQC": "Completed",
    "PendingWarehouseReceive": "Completed", "PendingProductionTransfer": "Completed",
    "PendingClose": "Completed",
}
SHA256 = re.compile(r"^[0-9a-f]{64}$")
GROUP_CONCAT_MAX_LEN = 1048576


def confirmed_time(value: Any) -> bool:
    return isinstance(value, str) and bool(value) and not value.startswith("0001-01-01T00:00:00")

ALGORITHM_SPEC = {
    "version": 2,
    "authority": [
        "repo/mysql/task_search_document.go:reindexTaskSearchDocumentProjection",
        "cmd/tools/search-reindex/main.go:assetReindexSQL",
        "repo/mysql/task_resource_group.go:reindexTaskAssetGroupSearchDocument",
    ],
    "task_search": {
        "separator": " ",
        "group_concat_max_len": GROUP_CONCAT_MAX_LEN,
        "asset_group_concat": "ORDER BY task_assets.id",
        "planning_group_concat": "ORDER BY task_sku_items.id, task_planning_sku_revisions.id",
    },
    "group_search": {
        "reference_order": "sort_order ascending",
        "final_order": "sort_order ascending",
    },
    "client_pin": "natural group/revision/item coordinates; no generated surrogate ids",
}


def canonical_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_text(value: str) -> str:
    return sha256_bytes(value.encode("utf-8"))


def mysql_concat_ws(values: Iterable[Any]) -> str:
    """Match CONCAT_WS(' ', ...): skip NULL, retain empty strings."""
    return " ".join(str(value) for value in values if value is not None)


def is_loopback(host: str) -> bool:
    if host.lower() == "localhost":
        return True
    try:
        return ipaddress.ip_address(host).is_loopback
    except ValueError:
        return False


FREEZE_SQL = r"""
SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
SET SESSION TRANSACTION READ ONLY;
START TRANSACTION WITH CONSISTENT SNAPSHOT, READ ONLY;
SELECT CONCAT('task\t',JSON_OBJECT(
  'id',t.id,'task_no',t.task_no,'product_name_snapshot',t.product_name_snapshot,
  'sku_code',t.sku_code,'primary_sku_code',t.primary_sku_code,'task_type',t.task_type,
  'task_status',t.task_status,'current_handler_id',t.current_handler_id,'priority',t.priority,
  'owner_department',t.owner_department,'owner_team',t.owner_team,'owner_org_team',t.owner_org_team,
  'detail_category',COALESCE(NULLIF(td.category,''),NULLIF(td.category_name,''),''),
  'category_code',td.category_code,'product_short_name',td.product_short_name,'demand_text',td.demand_text,
  'copy_text',td.copy_text,'remark',td.remark,'change_request',td.change_request,
  'design_requirement',td.design_requirement,'material',td.material,'spec_text',td.spec_text,
  'size_text',td.size_text,'craft_text',td.craft_text,'process',td.process,'reference_link',td.reference_link,
  'creator_name',COALESCE(NULLIF(creator.display_name,''),creator.username,''),
  'requester_name',COALESCE(NULLIF(requester.display_name,''),requester.username,''),
  'designer_name',COALESCE(NULLIF(designer.display_name,''),designer.username,''),
  'handler_name',COALESCE(NULLIF(handler.display_name,''),handler.username,''),
  'created_date',DATE_FORMAT(t.created_at,'%Y-%m-%d'),'created_compact',DATE_FORMAT(t.created_at,'%Y%m%d'),
  'deadline_date',DATE_FORMAT(t.deadline_at,'%Y-%m-%d')))
FROM tasks t
LEFT JOIN task_details td ON td.task_id=t.id
LEFT JOIN users creator ON creator.id=t.creator_id
LEFT JOIN users requester ON requester.id=t.requester_id
LEFT JOIN users designer ON designer.id=t.designer_id
LEFT JOIN users handler ON handler.id=t.current_handler_id
ORDER BY t.id;
SELECT CONCAT('asset\t',JSON_OBJECT(
  'id',id,'task_id',task_id,'asset_type',asset_type,'file_name',file_name,
  'original_filename',original_filename,'storage_key',storage_key,'source_module_key',source_module_key,
  'deleted_at',deleted_at,'cleaned_at',cleaned_at))
FROM task_assets ORDER BY task_id,id;
SELECT CONCAT('sku\t',JSON_OBJECT('id',id,'task_id',task_id,'sku_code',sku_code))
FROM task_sku_items ORDER BY task_id,id;
SELECT CONCAT('reference\t',JSON_OBJECT('id',r.id,'task_id',r.task_id,'file_name',COALESCE(s.file_name,'')))
FROM reference_file_refs r LEFT JOIN asset_storage_refs s ON s.ref_id=r.ref_id ORDER BY r.task_id,r.id;
SELECT CONCAT('planning\t',JSON_OBJECT(
  'task_id',si.task_id,'task_sku_item_id',si.id,'sku_code',si.sku_code,
  'description_spec',rev.description_spec,'note',rev.note,'erp_product_i_id',rev.erp_product_i_id,
  'erp_product_name',rev.erp_product_name,
  'latest_status',COALESCE((SELECT latest.status FROM task_erp_outbox latest
    WHERE latest.task_sku_item_id=si.id ORDER BY latest.generation DESC,latest.id DESC LIMIT 1),'')))
FROM task_sku_items si
JOIN task_planning_sku_details d ON d.task_sku_item_id=si.id
JOIN task_planning_sku_revisions rev ON rev.id=d.current_revision_id
ORDER BY si.task_id,si.id;
SELECT CONCAT('client_pin\t',JSON_OBJECT(
  'id',c.id,'asset_id',c.asset_id,'source_type',c.source_type,'source_ref',c.source_ref,'enabled',c.enabled,
  'task_id',g.task_id,'scope_kind',g.scope_kind,'scope_ref_id',g.scope_ref_id,
  'finalized_revision_no',rev.revision_no,'cover_sort_order',item.sort_order))
FROM asset_workbench_client_materials c
LEFT JOIN task_asset_groups g ON g.id=c.resource_group_id
LEFT JOIN task_asset_group_revisions rev ON rev.group_id=g.id AND rev.id=c.finalized_revision_id
LEFT JOIN task_asset_group_revision_items item ON item.revision_id=rev.id AND item.id=c.cover_revision_item_id
ORDER BY c.id;
COMMIT;
"""


def freeze_a(args: argparse.Namespace) -> None:
    if not is_loopback(args.host):
        raise ValueError("--host must be loopback; only frozen clone A is allowed")
    if not args.database or args.database in {"mysql", "information_schema", "performance_schema", "sys"}:
        raise ValueError("--database must name an application clone")
    if not args.snapshot_sha256 or len(args.snapshot_sha256) != 64:
        raise ValueError("--snapshot-sha256 is required")
    command = [args.mysql, f"--host={args.host}", f"--port={args.port}", f"--user={args.user}",
               "--default-character-set=utf8mb4", "--batch", "--raw", "--skip-column-names", args.database]
    if args.defaults_extra_file:
        command.insert(1, f"--defaults-extra-file={args.defaults_extra_file}")
    completed = subprocess.run(command, input=FREEZE_SQL, text=True, capture_output=True, check=False)
    if completed.returncode:
        raise RuntimeError(f"mysql read-only projection freeze failed: {completed.stderr.strip()}")
    records = []
    allowed = {"task", "asset", "sku", "reference", "planning", "client_pin"}
    for line_no, line in enumerate(completed.stdout.splitlines(), 1):
        if not line:
            continue
        try:
            kind, raw = line.split("\t", 1)
            if kind not in allowed:
                raise ValueError(kind)
            records.append({"kind": kind, "value": json.loads(raw)})
        except (ValueError, json.JSONDecodeError) as exc:
            raise RuntimeError(f"unexpected mysql output at line {line_no}") from exc
    if not any(row["kind"] == "task" for row in records):
        raise RuntimeError("frozen A produced no tasks")
    records.sort(key=record_sort_key)
    header = {"kind": "header", "value": {"schema_version": 1, "snapshot_sha256": args.snapshot_sha256,
              "algorithm_sha256": sha256_text(canonical_json(ALGORITHM_SPEC))}}
    output = pathlib.Path(args.output)
    output.write_text("".join(canonical_json(row) + "\n" for row in [header, *records]), encoding="utf-8")


def record_sort_key(row: dict[str, Any]) -> tuple[Any, ...]:
    value = row["value"]
    kind_order = {"task": 0, "asset": 1, "sku": 2, "reference": 3, "planning": 4, "client_pin": 5}
    return (kind_order[row["kind"]], int(value.get("task_id") or value.get("id") or 0), int(value.get("id") or value.get("task_sku_item_id") or 0))


def load_frozen(path: pathlib.Path, snapshot_sha256: str) -> tuple[dict[str, list[dict[str, Any]]], str]:
    raw = path.read_bytes()
    rows = [json.loads(line) for line in raw.decode("utf-8").splitlines() if line]
    if not rows or rows[0].get("kind") != "header":
        raise ValueError("frozen A JSONL must start with a header")
    header = rows[0].get("value") or {}
    if header.get("schema_version") != 1 or header.get("snapshot_sha256") != snapshot_sha256:
        raise ValueError("frozen A header does not match the attested snapshot")
    if header.get("algorithm_sha256") != sha256_text(canonical_json(ALGORITHM_SPEC)):
        raise ValueError("frozen A was produced for a different projection algorithm")
    grouped: dict[str, list[dict[str, Any]]] = defaultdict(list)
    seen: set[tuple[str, int]] = set()
    for row in rows[1:]:
        kind, value = row.get("kind"), row.get("value")
        if kind not in {"task", "asset", "sku", "reference", "planning", "client_pin"} or not isinstance(value, dict):
            raise ValueError("invalid frozen A record")
        identity_field = "task_sku_item_id" if kind == "planning" else "id"
        try:
            identity = int(value[identity_field])
        except (KeyError, TypeError, ValueError) as exc:
            raise ValueError(f"invalid frozen A {kind} identity") from exc
        natural = (kind, identity)
        if natural in seen:
            raise ValueError(f"duplicate frozen A {kind} identity {identity}")
        seen.add(natural)
        grouped[kind].append(value)
    if not grouped["task"]:
        raise ValueError("frozen A contains no tasks")
    return grouped, sha256_bytes(raw)


def require_reviewed_mapping(mapping: dict[str, Any]) -> None:
    if mapping.get("version") != 2:
        raise ValueError("reviewed mapping must be version 2")
    for group in mapping.get("resources", []):
        for revision in group.get("history", []):
            if (revision.get("confidence") != "confirmed_auto" or not revision.get("confirmed_by")
                    or not confirmed_time(revision.get("confirmed_at")) or not revision.get("confirmation_note")
                    or not SHA256.fullmatch(str(revision.get("manifest_row_hash") or ""))):
                raise ValueError("all resource revisions must be confirmed_auto with reviewer metadata")
    for planning in mapping.get("planning_tasks", []):
        if (planning.get("confidence") != "confirmed_auto" or not planning.get("confirmed_by")
                or not confirmed_time(planning.get("confirmed_at")) or not planning.get("confirmation_note")):
            raise ValueError("all planning tasks must be confirmed_auto with reviewer metadata")
    for decision in mapping.get("task_state_decisions", []):
        if (not decision.get("confirmed_by") or not confirmed_time(decision.get("confirmed_at")) or not decision.get("confirmation_note")
                or not SHA256.fullmatch(str(decision.get("manifest_row_hash") or ""))):
            raise ValueError("all task state decisions require reviewer metadata")


def read_json_object(path_value: str, label: str) -> tuple[dict[str, Any], str]:
    path = pathlib.Path(path_value)
    raw = path.read_bytes()
    value = json.loads(raw.decode("utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{label} must contain a JSON object")
    return value, sha256_bytes(raw)


def materialized_bundle_assets(
    mapping: dict[str, Any],
    bundle_mapping_path_value: str | None,
    manifest_path_value: str | None,
    registry_path_value: str | None,
) -> tuple[list[dict[str, Any]], dict[str, str]]:
    supplied = [
        bool(bundle_mapping_path_value),
        bool(manifest_path_value),
        bool(registry_path_value),
    ]
    if any(supplied) and not all(supplied):
        raise ValueError(
            "bundle base mapping, manifest, and registry must be supplied together"
        )
    if not manifest_path_value:
        return [], {}
    bundle_mapping, bundle_mapping_sha = read_json_object(
        str(bundle_mapping_path_value), "source bundle base mapping"
    )
    if (
        bundle_mapping.get("version") != 2
        or not isinstance(bundle_mapping.get("resources"), list)
    ):
        raise ValueError("source bundle base mapping must be a V2 mapping")
    manifest, manifest_sha = read_json_object(
        manifest_path_value, "source bundle manifest"
    )
    confirmed, run_id = bundle_registry.validate_manifest(
        manifest, bundle_mapping_sha
    )
    registry, registry_sha = read_json_object(
        str(registry_path_value), "source bundle registry"
    )
    normalized = bundle_registry.validate_registry(
        registry, manifest_sha, run_id, confirmed
    )
    mapped: dict[tuple[int, str, int, int], dict[str, Any]] = {}
    for resource in mapping.get("resources", []):
        for revision in resource.get("history", []):
            source_bundle = revision.get("source_bundle")
            if source_bundle:
                key = (
                    int(resource["task_id"]),
                    str(resource["scope_kind"]),
                    int(resource["scope_ref_id"]),
                    int(revision["revision_no"]),
                )
                mapped[key] = source_bundle
    if set(mapped) != set(normalized):
        raise ValueError("reviewed mapping and bundle registry scopes differ")
    for key, expected in normalized.items():
        if mapped[key] != expected:
            raise ValueError(f"reviewed mapping source bundle drifted for {key}")
    assets: list[dict[str, Any]] = []
    seen: set[int] = set()
    for entry in registry["entries"]:
        candidate = entry["task_asset_candidate"]
        asset_id = int(candidate["id"])
        if asset_id in seen:
            raise ValueError(f"duplicate materialized bundle task asset {asset_id}")
        seen.add(asset_id)
        assets.append(
            {
                "id": asset_id,
                "task_id": int(candidate["task_id"]),
                "asset_type": "source",
                "file_name": "source-bundle.zip",
                "original_filename": "source-bundle.zip",
                "storage_key": str(candidate["storage_key"]),
                "source_module_key": "migration",
                "deleted_at": None,
                "cleaned_at": None,
            }
        )
    return assets, {
        "source_bundle_base_mapping_sha256": bundle_mapping_sha,
        "source_bundle_manifest_sha256": manifest_sha,
        "source_bundle_registry_sha256": registry_sha,
    }


def apply_recovery_plan(
    mapping: dict[str, Any],
    mapping_sha: str,
    assets: dict[int, dict[str, Any]],
    plan_path_value: str | None,
) -> dict[str, str]:
    expected = {
        int(row["missing_task_asset_id"]): row
        for row in mapping.get("asset_recoveries", [])
        if row.get("strategy") == "clone_b_prematerialized_storage_ref_v1"
    }
    if not plan_path_value:
        if expected:
            raise ValueError("reviewed asset recoveries require --recovery-plan")
        return {}
    plan, plan_sha = read_json_object(plan_path_value, "recovery plan")
    if (
        plan.get("version") != 1
        or plan.get("status") != "MATERIALIZED"
        or plan.get("mapping_sha256") != mapping_sha
        or plan.get("production_writes_executed") is not False
        or plan.get("database_writes_executed") is not False
        or not isinstance(plan.get("run_id"), str)
        or not plan["run_id"].strip()
        or not isinstance(plan.get("entries"), list)
    ):
        raise ValueError("recovery plan contract or mapping binding differs")
    actual: set[int] = set()
    for entry in plan["entries"]:
        if not isinstance(entry, dict):
            raise ValueError("recovery plan entry must be an object")
        asset_id = int(entry.get("missing_task_asset_id") or 0)
        if asset_id in actual or asset_id not in expected:
            raise ValueError(f"unexpected or duplicate recovered task asset {asset_id}")
        actual.add(asset_id)
        mapping_row = expected[asset_id]
        asset = assets.get(asset_id)
        update = (entry.get("db_apply_plan") or {}).get("update_task_asset") or {}
        update_set = update.get("set") or {}
        if (
            asset is None
            or int(asset.get("task_id") or 0) != int(mapping_row["task_id"])
            or entry.get("source_sha256") != mapping_row["recovery_source_sha256"]
            or entry.get("source_size") != mapping_row["expected_file_size"]
            or update.get("where") != {"id": asset_id}
            or update_set.get("storage_key") != entry.get("target_object_key")
            or update_set.get("whole_hash") != entry.get("source_sha256")
            or update_set.get("deleted_at") is not None
            or update_set.get("cleaned_at") is not None
        ):
            raise ValueError(f"recovery plan entry {asset_id} drifted")
        asset["storage_key"] = str(entry["target_object_key"])
        asset["deleted_at"] = None
        asset["cleaned_at"] = None
    if actual != set(expected):
        raise ValueError("recovery plan does not cover the reviewed recovery set")
    return {"recovery_plan_sha256": plan_sha}


def is_active_asset(row: dict[str, Any]) -> bool:
    return row.get("deleted_at") is None and row.get("cleaned_at") is None


def blocked_entity(entity_key: str, blockers: list[str], detail: dict[str, Any]) -> dict[str, Any]:
    return {"gate_name": "G09", "entity_key": entity_key, "expected_state": "blocked",
            "review_state": "hard_blocked", "derivation_method": "independent_projection",
            "components": [entity_key, "hard_blocked", sha256_text(canonical_json(sorted(set(blockers))))],
            "detail": {**detail, "blockers": sorted(set(blockers))}}


def pass_entity(entity_key: str, components: list[str], detail: dict[str, Any]) -> dict[str, Any]:
    return {"gate_name": "G09", "entity_key": entity_key, "expected_state": "approved",
            "review_state": "pass", "derivation_method": "independent_projection",
            "components": components, "detail": detail}


def build_expected(args: argparse.Namespace) -> None:
    mapping_path = pathlib.Path(args.mapping)
    mapping = json.loads(mapping_path.read_text(encoding="utf-8"))
    require_reviewed_mapping(mapping)
    frozen, frozen_sha = load_frozen(pathlib.Path(args.frozen_a), args.snapshot_sha256)
    mapping_sha = sha256_bytes(mapping_path.read_bytes())
    algorithm_sha = sha256_text(canonical_json(ALGORITHM_SPEC))
    provenance = {"mapping_sha256": mapping_sha, "frozen_a_sha256": frozen_sha,
                  "snapshot_sha256": args.snapshot_sha256, "algorithm_sha256": algorithm_sha}

    tasks = {int(row["id"]): dict(row) for row in frozen["task"]}
    assets = {int(row["id"]): dict(row) for row in frozen["asset"]}
    bundle_assets, bundle_provenance = materialized_bundle_assets(
        mapping,
        getattr(args, "source_bundle_mapping", None),
        getattr(args, "source_bundle_manifest", None),
        getattr(args, "source_bundle_registry", None),
    )
    for row in bundle_assets:
        asset_id = int(row["id"])
        if asset_id in assets:
            raise ValueError(f"materialized bundle task asset {asset_id} already exists")
        assets[asset_id] = row
    recovery_provenance = apply_recovery_plan(
        mapping,
        mapping_sha,
        assets,
        getattr(args, "recovery_plan", None),
    )
    provenance.update(bundle_provenance)
    provenance.update(recovery_provenance)
    assets_by_task: dict[int, list[dict[str, Any]]] = defaultdict(list)
    for row in assets.values():
        if is_active_asset(row):
            assets_by_task[int(row["task_id"])].append(row)
    skus = {int(row["id"]): row for row in frozen["sku"]}
    refs = {int(row["id"]): row for row in frozen["reference"]}
    planning_by_task: dict[int, list[dict[str, Any]]] = defaultdict(list)
    for row in frozen["planning"]:
        planning_by_task[int(row["task_id"])].append(row)

    reviewed_decisions: dict[int, dict[str, Any]] = {}
    for decision in mapping.get("task_state_decisions", []):
        task_id = int(decision["task_id"])
        if task_id in reviewed_decisions:
            raise ValueError(f"duplicate state decision for task {task_id}")
        reviewed_decisions[task_id] = decision
        task = tasks.get(task_id)
        if task is None:
            raise ValueError(f"state decision task {decision['task_id']} is absent from frozen A")
        if str(task.get("task_status") or "") != decision["from_status"]:
            raise ValueError(f"state decision task {decision['task_id']} no longer matches from_status")
        task["task_status"] = decision["target_status"]
    for task_id, task in tasks.items():
        if task_id not in reviewed_decisions:
            task["task_status"] = STATE_MAP.get(
                str(task.get("task_status") or ""),
                task.get("task_status") or "",
            )

    # A source alias is one extra task_assets row per natural group/origin.
    aliases_by_task: dict[int, list[dict[str, Any]]] = defaultdict(list)
    alias_sequence = 0
    for group in mapping.get("resources", []):
        group_key = (int(group["task_id"]), str(group["scope_kind"]), int(group["scope_ref_id"]))
        origins = []
        for revision in group.get("history", []):
            origin = revision.get("source_alias_from_task_asset_id")
            if origin and int(origin) not in origins:
                origins.append(int(origin))
        for origin_id in origins:
            origin = assets.get(origin_id)
            if origin is None:
                continue
            alias_sequence += 1
            alias = dict(origin)
            alias["source_module_key"] = "migration"
            alias["_natural_alias_key"] = f"{group_key[0]}:{group_key[1]}:{group_key[2]}:{origin_id}"
            alias["_alias_sequence"] = alias_sequence
            aliases_by_task[group_key[0]].append(alias)

    for planning in mapping.get("planning_tasks", []):
        task_id = int(planning["task_id"])
        task = tasks.get(task_id)
        if task is None:
            raise ValueError(f"planning task {task_id} is absent from frozen A")
        task["task_type"] = "sku_planning"
        task["task_status"] = planning["target_task_status"]
        if not planning_by_task[task_id]:
            for item in planning.get("items", []):
                sku = skus.get(int(item["task_sku_item_id"]))
                planning_by_task[task_id].append({"task_id": task_id, "task_sku_item_id": item["task_sku_item_id"],
                    "sku_code": "" if sku is None else sku.get("sku_code", ""),
                    "description_spec": item.get("description_spec", ""), "note": item.get("note", ""),
                    "erp_product_i_id": item.get("erp_product_i_id", ""),
                    "erp_product_name": item.get("erp_product_name", ""), "latest_status": ""})

    entities: list[dict[str, Any]] = []
    task_search_fields = ["id", "task_no", "product_name_snapshot", "sku_code", "primary_sku_code", "task_type",
        "task_status", "priority", "owner_department", "owner_team", "owner_org_team", "detail_category",
        "category_code", "product_short_name", "demand_text", "copy_text", "remark", "change_request",
        "design_requirement", "material", "spec_text", "size_text", "craft_text", "process", "reference_link",
        "creator_name", "requester_name", "designer_name", "handler_name", "created_date", "created_compact", "deadline_date"]
    for task_id in sorted(tasks):
        task = tasks[task_id]
        task_assets = sorted([*assets_by_task[task_id], *aliases_by_task[task_id]],
                             key=lambda row: (1, int(row["_alias_sequence"])) if "_alias_sequence" in row else (0, int(row["id"])))
        planning_rows = sorted(planning_by_task[task_id], key=lambda row: int(row["task_sku_item_id"]))
        blockers = []
        asset_text = mysql_concat_ws([
            mysql_concat_ws([row.get("file_name"), row.get("original_filename"), row.get("storage_key"), row.get("source_module_key")])
            for row in task_assets
        ]) if task_assets else ""
        planning_text = mysql_concat_ws([
            mysql_concat_ws([row.get("sku_code"), row.get("description_spec"), row.get("note"),
                             row.get("erp_product_i_id"), row.get("erp_product_name"), row.get("latest_status", "")])
            for row in planning_rows
        ]) if planning_rows else ""
        if len(asset_text.encode("utf-8")) > GROUP_CONCAT_MAX_LEN:
            blockers.append("ordered task asset text exceeds group_concat_max_len")
        if len(planning_text.encode("utf-8")) > GROUP_CONCAT_MAX_LEN:
            blockers.append("ordered planning text exceeds group_concat_max_len")
        search_text = mysql_concat_ws([task.get(field) for field in task_search_fields] + [asset_text, planning_text])
        key = f"task-search:{task_id}"
        detail = {**provenance, "projection_kind": "task_search", "search_text_sha256": sha256_text(search_text)}
        components = [str(task_id), str(task.get("task_type") or ""), str(task.get("task_status") or ""),
                      "" if task.get("current_handler_id") is None else str(task["current_handler_id"]), sha256_text(search_text)]
        entities.append(blocked_entity(key, blockers, detail) if blockers else pass_entity(key, components, detail))

    for group in sorted(mapping.get("resources", []), key=lambda row: (int(row["task_id"]), row["scope_kind"], int(row["scope_ref_id"]))):
        finalized_no = group.get("finalized_revision_no")
        if finalized_no is None:
            continue
        task_id, scope_kind, scope_id = int(group["task_id"]), str(group["scope_kind"]), int(group["scope_ref_id"])
        key = f"group-search:{task_id}:{scope_kind}:{scope_id}"
        blockers = []
        task = tasks.get(task_id)
        revision = next((row for row in group.get("history", []) if int(row["revision_no"]) == int(finalized_no)), None)
        if task is None or revision is None:
            blockers.append("task or finalized revision is absent from reviewed inputs")
        if blockers:
            entities.append(blocked_entity(key, blockers, {**provenance, "projection_kind": "group_search"}))
            continue
        source_id = revision.get("source_task_asset_id")
        if not source_id and revision.get("source_alias_from_task_asset_id"):
            source_id = revision["source_alias_from_task_asset_id"]
        if not source_id and revision.get("source_bundle"):
            source_id = revision["source_bundle"].get("task_asset_id")
        source = assets.get(int(source_id)) if source_id else None
        if source_id and source is None:
            blockers.append(f"source task asset {source_id} lacks a frozen filename")
        final_rows = []
        for final_id in revision.get("final_task_asset_ids", []):
            row = assets.get(int(final_id))
            if row is None:
                blockers.append(f"final task asset {final_id} lacks a frozen filename")
            else:
                final_rows.append(row)
        reference_rows = []
        for reference_id in revision.get("reference_file_ref_ids", []):
            row = refs.get(int(reference_id))
            if row is None:
                blockers.append(f"reference {reference_id} lacks a frozen filename")
            else:
                reference_rows.append(row)
        sku_code = ""
        if scope_kind == "sku":
            sku = skus.get(scope_id)
            if sku is None or int(sku.get("task_id") or 0) != task_id:
                blockers.append(f"SKU scope {scope_id} is absent or belongs to another task")
            else:
                sku_code = str(sku.get("sku_code") or "")
        final_names = mysql_concat_ws([row.get("file_name") for row in final_rows])
        ref_names = mysql_concat_ws([row.get("file_name") for row in reference_rows])
        common = [task_id, task.get("task_no"), task.get("sku_code"), task.get("primary_sku_code"), task.get("product_name_snapshot"), sku_code]
        internal_text = mysql_concat_ws(common + ["" if source is None else source.get("file_name"), ref_names, final_names])
        final_text = mysql_concat_ws(common + [final_names])
        components = [str(task_id), scope_kind, str(scope_id), str(finalized_no), sha256_text(internal_text), sha256_text(final_text)]
        detail = {**provenance, "projection_kind": "group_search", "internal_text_sha256": sha256_text(internal_text), "final_text_sha256": sha256_text(final_text)}
        entities.append(blocked_entity(key, blockers, detail) if blockers else pass_entity(key, components, detail))

    resources = {(int(row["task_id"]), str(row["scope_kind"]), int(row["scope_ref_id"])): row for row in mapping.get("resources", [])}
    for pin in sorted(frozen["client_pin"], key=lambda row: int(row["id"])):
        key = f"client-pin:{pin['id']}"
        blockers = []
        if pin.get("source_type") == "task_resource_group":
            natural = (int(pin.get("task_id") or 0), str(pin.get("scope_kind") or ""), int(pin.get("scope_ref_id") or 0))
            resource = resources.get(natural)
            if resource is None:
                blockers.append("task-resource-group publication has no reviewed natural group mapping")
            else:
                pinned_no = int(pin.get("finalized_revision_no") or 0)
                pinned_revision = next(
                    (revision for revision in resource.get("history", []) if int(revision.get("revision_no") or 0) == pinned_no),
                    None,
                )
                if pinned_revision is None:
                    blockers.append("publication pin revision is absent from reviewed history")
                elif pinned_revision.get("status") not in {"finalized", "superseded"}:
                    blockers.append("publication pin revision is not an immutable finalized/superseded snapshot")
                elif pin.get("cover_sort_order") is not None:
                    cover_order = int(pin["cover_sort_order"])
                    if cover_order < 0 or cover_order >= len(pinned_revision.get("final_task_asset_ids", [])):
                        blockers.append("publication cover item order is outside the pinned revision")
            if pin.get("cover_sort_order") is None:
                blockers.append("task-resource-group publication has no derivable cover item order")
        components = [str(pin["id"]), str(pin.get("source_type") or ""), str(pin.get("source_ref") or ""),
            "" if pin.get("asset_id") is None else str(pin["asset_id"]), str(int(pin.get("enabled") or 0)),
            "" if pin.get("task_id") is None else str(pin["task_id"]), str(pin.get("scope_kind") or ""),
            "" if pin.get("scope_ref_id") is None else str(pin["scope_ref_id"]),
            "" if pin.get("finalized_revision_no") is None else str(pin["finalized_revision_no"]),
            "" if pin.get("cover_sort_order") is None else str(pin["cover_sort_order"])]
        detail = {**provenance, "projection_kind": "client_publish_pin"}
        entities.append(blocked_entity(key, blockers, detail) if blockers else pass_entity(key, components, detail))

    entities.sort(key=lambda row: row["entity_key"])
    pathlib.Path(args.output).write_text("".join(canonical_json(row) + "\n" for row in entities), encoding="utf-8")


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)
    freeze = sub.add_parser("freeze-a")
    freeze.add_argument("--host", default="127.0.0.1"); freeze.add_argument("--port", type=int, default=3306)
    freeze.add_argument("--user", required=True); freeze.add_argument("--database", required=True)
    freeze.add_argument("--defaults-extra-file"); freeze.add_argument("--mysql", default="mysql")
    freeze.add_argument("--snapshot-sha256", required=True); freeze.add_argument("--output", required=True)
    build = sub.add_parser("build")
    build.add_argument("--mapping", required=True); build.add_argument("--frozen-a", required=True)
    build.add_argument("--source-bundle-mapping")
    build.add_argument("--source-bundle-manifest")
    build.add_argument("--source-bundle-registry")
    build.add_argument("--recovery-plan")
    build.add_argument("--snapshot-sha256", required=True); build.add_argument("--output", required=True)
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    try:
        args = parse_args(argv)
        freeze_a(args) if args.command == "freeze-a" else build_expected(args)
    except (OSError, UnicodeDecodeError, ValueError, RuntimeError, json.JSONDecodeError) as exc:
        print(str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
