#!/usr/bin/env python3
"""Generate conservative V8 migration candidates from a frozen local clone."""

from __future__ import annotations

import argparse
import copy
import csv
import datetime as dt
import hashlib
import ipaddress
import json
import pathlib
import subprocess
import sys
from collections import defaultdict
from typing import Any, Iterable


SUBMIT_EVENTS = {"task.design.submitted", "task.design_submitted", "submitted"}
APPROVE_EVENTS = {"task.audit.approved", "approved"}
REJECT_EVENTS = {"task.audit.returned_to_design", "task.audit.rejected", "rejected"}
REOPEN_EVENTS = {"task.reopened", "reopened"}
CLOSE_EVENTS = {"task.closed", "closed"}
WAREHOUSE_REOPEN_EVENTS = {"task.warehouse.rejected"}
SUPPLEMENT_EVENTS = {"task.audit.supplement_uploaded"}
UPLOAD_SESSION_COMPLETED_EVENTS = {"task.asset.upload_session.completed"}
BATCH_SUBMIT_POLICY = "legacy_multi_sku_atomic_batch_submit_v1"
ATOMIC_UPLOAD_BATCH_SUBMIT_POLICY = "legacy_atomic_upload_batch_submit_v1"
AUDIT_STAGE_FINAL_SNAPSHOT_POLICY = "legacy_audit_stage_final_snapshot_v1"
EXPLICIT_EVENT_REPLAY_POLICY = "explicit_event_replay"
DELIVERY_SOURCE_ALIAS_POLICY = "delivery_source_alias"
REJECTED_HISTORY_POLICY = "rejected_history"
REOPEN_POLICY = "reopen"
POST_CLOSE_REPLACEMENT_POLICY = "legacy_post_close_replacement_v1"
UPLOAD_CLEANUP_GRACE_SECONDS = 90
RETOUCH_SOURCE_OPTIONAL_POLICY = "retouch_source_optional"
RETOUCH_TERMINAL_SUBMIT_POLICY = "legacy_retouch_terminal_submit_v1"
RETOUCH_UNSCOPED_ATOMIC_BATCH_POLICY = "legacy_retouch_unscoped_atomic_batch_v1"
RETOUCH_PREMATURE_TERMINAL_PARTIAL_POLICY = (
    "legacy_retouch_premature_terminal_partial_v1"
)
RETOUCH_VISUAL_SCOPE_TASK2533_POLICY = (
    "legacy_retouch_visual_scope_task2533_v1"
)
LEGACY_PURCHASE_TO_PLANNING_POLICY = "legacy_purchase_to_sku_planning_v1"
FROZEN_PLANNING_RULE_POLICY = "frozen_sku_planning_rule_revision_9_v1"
PRODUCT_NAME_DESCRIPTION_FALLBACK_POLICY = "product_name_snapshot_description_fallback_v1"
RETIRED_PLANNING_STATUS_POLICY = "retired_planning_status_to_completed_v1"
ORG_UNIQUE_STABLE_POLICY = "legacy_org_unique_stable_match_v1"
ORG_ALIAS_LINEAGE_POLICY = "legacy_org_alias_lineage_v1"
ORG_MANUAL_TARGET_POLICY = "legacy_org_manual_target_required_v1"
WAREHOUSE_NO_GRANT_POLICY = "retired_warehouse_no_new_grant_v1"
EXISTING_ACCESS_PRESERVED_POLICY = "existing_access_assignment_preserved_v1"
OUTSOURCE_ACCESS_DECISION_POLICY = "legacy_outsource_access_decision_v1"
ORG_ADMIN_ACCESS_DECISION_POLICY = "legacy_org_admin_access_decision_v1"
UAT_ORPHAN_ORG_POLICY = "legacy_uat_orphan_org_to_unassigned_v1"
INCOMPLETE_UAT_PLANNING_TOMBSTONE_POLICY = (
    "legacy_incomplete_uat_planning_tombstone_v1"
)
WAREHOUSE_REOPEN_STATE_POLICY = "legacy_warehouse_reopen_state_v1"
CUSTOMIZATION_TERMINAL_WITHOUT_ASSETS_POLICY = (
    "legacy_customization_terminal_without_assets_to_inprogress_v1"
)
DELETED_ASSET_RECOVERY_POLICY = "legacy_deleted_asset_recovery_v1"
HISTORICAL_ASSET_UNAVAILABLE_POLICY = (
    "legacy_historical_asset_unavailable_v1"
)
REVIEW_POLICY_ORDER = (
    EXPLICIT_EVENT_REPLAY_POLICY,
    DELIVERY_SOURCE_ALIAS_POLICY,
    REJECTED_HISTORY_POLICY,
    REOPEN_POLICY,
    POST_CLOSE_REPLACEMENT_POLICY,
    RETOUCH_SOURCE_OPTIONAL_POLICY,
    RETOUCH_TERMINAL_SUBMIT_POLICY,
    RETOUCH_UNSCOPED_ATOMIC_BATCH_POLICY,
    RETOUCH_PREMATURE_TERMINAL_PARTIAL_POLICY,
    RETOUCH_VISUAL_SCOPE_TASK2533_POLICY,
    BATCH_SUBMIT_POLICY,
    ATOMIC_UPLOAD_BATCH_SUBMIT_POLICY,
    AUDIT_STAGE_FINAL_SNAPSHOT_POLICY,
    LEGACY_PURCHASE_TO_PLANNING_POLICY,
    INCOMPLETE_UAT_PLANNING_TOMBSTONE_POLICY,
    FROZEN_PLANNING_RULE_POLICY,
    PRODUCT_NAME_DESCRIPTION_FALLBACK_POLICY,
    RETIRED_PLANNING_STATUS_POLICY,
    ORG_UNIQUE_STABLE_POLICY,
    ORG_ALIAS_LINEAGE_POLICY,
    ORG_MANUAL_TARGET_POLICY,
    WAREHOUSE_NO_GRANT_POLICY,
    EXISTING_ACCESS_PRESERVED_POLICY,
    OUTSOURCE_ACCESS_DECISION_POLICY,
    ORG_ADMIN_ACCESS_DECISION_POLICY,
    UAT_ORPHAN_ORG_POLICY,
    WAREHOUSE_REOPEN_STATE_POLICY,
    CUSTOMIZATION_TERMINAL_WITHOUT_ASSETS_POLICY,
    DELETED_ASSET_RECOVERY_POLICY,
    HISTORICAL_ASSET_UNAVAILABLE_POLICY,
)
ZERO_TIME = "0001-01-01T00:00:00Z"
LEGACY_PLANNING_RULE_REVISION_ID = 9
LEGACY_PLANNING_STATUS_MAP = {
    "PendingClose": "Completed",
    "PendingProductionTransfer": "Completed",
}
CURRENT_PLANNING_STATUSES = {
    "Draft",
    "PendingAssign",
    "Assigned",
    "InProgress",
    "PendingAudit",
    "Completed",
    "Archived",
    "Blocked",
    "Cancelled",
}

# These rows are not a heuristic. They are a frozen, independently reviewed
# exception list for legacy retouch submissions whose delivery files omitted
# retouch_requirement_id. A candidate is emitted only when the complete task
# scope, complete delivery set, upload-completion events, and sole terminal
# submit still match this snapshot exactly.
LEGACY_RETOUCH_UNSCOPED_ATOMIC_FINALS = {
    1562: {69: [7900, 7901, 7902], 70: [7903, 7904, 7905]},
    1773: {88: [8622, 8623], 89: [8624, 8625], 90: [8626, 8627]},
    1938: {
        98: [10845, 10846, 10847, 10848, 10849],
        99: [10850, 10851, 10852],
        100: [10853, 10854, 10855],
    },
    1986: {103: [10385, 10386, 10387], 104: [10388]},
    2003: {
        107: [
            11397,
            11398,
            11399,
            11400,
            11401,
            11402,
            11403,
            11404,
            11405,
            11406,
        ],
        108: [11407, 11408, 11409, 11410],
    },
    2495: {
        176: [22936, 22937, 22938, 22939, 22940, 22941, 22942, 22943, 22944],
        177: [22945, 22946, 22947, 22948, 22949, 22950, 22951, 22952, 22953],
    },
    2672: {209: [23109, 23110, 23111, 23112, 23113], 210: [23114]},
    2825: {226: [23900], 227: [23901]},
}

# These five legacy tasks reached Completed after only a partial, task-wide
# retouch submit. The only scope assignments accepted below are the ones
# proven by the frozen filename/order evidence. An empty list intentionally
# means "preserve an editable draft; do not assign the unscoped delivery".
LEGACY_RETOUCH_PREMATURE_PARTIAL_FINALS = {
    981: {8: [2763], 9: [], 10: [], 11: [], 12: [], 13: [], 14: [], 15: [], 16: [], 17: []},
    1035: {21: [3859], 22: [], 23: [], 24: [], 25: []},
    1045: {26: [], 27: [], 28: []},
    1052: {30: [], 31: [], 32: []},
    1214: {43: [5769], 44: []},
}

# Only these assignments are strong enough to retain an immutable finalized
# snapshot. Task 1214/requirement 43 is a partial draft, not a finalized scope.
LEGACY_RETOUCH_PREMATURE_FINALIZED_SCOPES = {(981, 8), (1035, 21)}

# This is the single legacy retouch task whose unscoped delivery membership
# was resolved by read-only visual comparison of the production task page.
# The sixth delivery is deliberately retained as an unassigned legacy asset:
# its silver/black content does not match any of requirements 183..187.
LEGACY_RETOUCH_VISUAL_SCOPE_TASK2533 = {
    183: {"source": 19299, "final": 19789, "references": [3211, 3212]},
    184: {"source": 19301, "final": 19790, "references": [3213]},
    185: {"source": 19304, "final": 19791, "references": [3214, 3215]},
    186: {"source": 19306, "final": 19800, "references": [3216]},
    187: {"source": 19308, "final": 19802, "references": [3217]},
}
LEGACY_RETOUCH_VISUAL_UNASSIGNED_TASK2533 = [19803]
UAT_ORPHAN_ORG_TARGETS = {
    463: (3, 14),
    464: (3, 14),
}
INCOMPLETE_UAT_PLANNING_TOMBSTONES = {
    497: {
        "task_sku_item_ids": (380,),
        "target_task_status": "Cancelled",
    },
}

# These six frozen rows crossed the retired customization-review boundary into
# PendingWarehouseReceive without a complete final asset snapshot.  They must
# become editable V8 drafts rather than fabricated Completed revisions.  The
# exact scope and the sole proven source member are part of the policy
# contract; any snapshot drift falls back to the ordinary hard-blocked replay.
LEGACY_CUSTOMIZATION_TERMINAL_WITHOUT_ASSETS = {
    449: {("task", 0): []},
    450: {("task", 0): []},
    451: {("task", 0): []},
    452: {("task", 0): [207]},
    756: {("sku", 578): []},
    757: {("sku", 579): [], ("sku", 580): []},
}

# This frozen Completed customization row lost its only approved final object.
# The immutable object probe is a 404, so the migration must preserve the
# approval-time revision as unavailable history and reopen an empty draft.
LEGACY_COMPLETED_CUSTOMIZATION_MISSING_FINAL = {
    3091: {
        "scope_kind": "sku",
        "scope_ref_id": 3311,
        "final_task_asset_id": 29144,
        "source_alias_candidates": [28966, 29023, 29144],
        "deleted_at": "2026-07-28T03:30:26Z",
        "storage_ref_id": "a40e151c-7fb0-4dfa-a8a2-624992c2832c",
    }
}

# These are evidence contracts, not permission to copy bytes.  The three
# task-2807 pairs have identical file size and independently generated preview
# plus design-thumb hashes.  Task asset 12323 deliberately has no source:
# same-root successors 14510/14514 have different byte sizes and therefore
# cannot be substituted.  The Go migrator accepts all rows only as candidates:
# recoverable rows still need a run-scoped Clone B object plus rollback-complete
# storage registration, while 12323 needs an explicit historical-unavailable
# API/UI/gate contract that never claims the original object exists.
LEGACY_DELETED_ASSET_RECOVERY_EVIDENCE = {
    23989: {
        "task_id": 2807,
        "recovery_source_task_asset_id": 24034,
        "original_storage_ref_id": "f511c5d4-507f-4a69-bf10-70bae369429d",
        "recovery_source_storage_ref_id": "983a746c-c674-4f5c-8812-073be989b194",
        "expected_file_size": 683001,
        "preview_whole_hash": "471739776f4c230a80ae5514e83e92fd3f1e104d203ced3ac793c65c25a525e4",
        "design_thumb_whole_hash": "3442c0ac91eb61371d4057d6c0de232f8ba4f3c25cb6b68cff63142aa155e6ef",
        "rejected_source_task_asset_ids": [],
        "controlled_read_protocol": "controlled-asset-read-v1",
        "controlled_read_evidence_sha256": "b39e0d9d26e6fdd35941b195bdc413eb12dd6795e23276a48c9b9bd49f829b08",
        "recovery_source_sha256": "d0558b1a9d4a7afed5a03b6b97d4a765d34050866686e396ab0acf9f08f0dec5",
    },
    23990: {
        "task_id": 2807,
        "recovery_source_task_asset_id": 24033,
        "original_storage_ref_id": "ca292dff-6824-4fe9-89cf-e439254f4383",
        "recovery_source_storage_ref_id": "85c01c4c-0e27-4df4-a851-4b888f54a837",
        "expected_file_size": 689291,
        "preview_whole_hash": "311d508fde06f4b7ae73ebfb915abda67c316f02d6356f052731d818d5e0ca47",
        "design_thumb_whole_hash": "7d38a5ff3cc65aa89aa15476e479a5eb0af611c4c60f145bbec40497a00cb62c",
        "rejected_source_task_asset_ids": [],
        "controlled_read_protocol": "controlled-asset-read-v1",
        "controlled_read_evidence_sha256": "b39e0d9d26e6fdd35941b195bdc413eb12dd6795e23276a48c9b9bd49f829b08",
        "recovery_source_sha256": "64cdfed11adc778fb6ede7f03c49f7c70e8655870236bdcd92a8207e41a8dfb8",
    },
    23991: {
        "task_id": 2807,
        "recovery_source_task_asset_id": 24040,
        "original_storage_ref_id": "107bbca3-b716-4043-b036-54dab1d52b0d",
        "recovery_source_storage_ref_id": "769e687f-fd71-4f37-930c-fd3f566350e6",
        "expected_file_size": 686447,
        "preview_whole_hash": "e4d8c77d270fb03cbcce3b8285b3373779a231605a09af515d3e2697118370a3",
        "design_thumb_whole_hash": "fd4a43d2b1e8cf2013c84a37a948538cc102f28a1a886f6662c50bdc08c5234d",
        "rejected_source_task_asset_ids": [],
        "controlled_read_protocol": "controlled-asset-read-v1",
        "controlled_read_evidence_sha256": "b39e0d9d26e6fdd35941b195bdc413eb12dd6795e23276a48c9b9bd49f829b08",
        "recovery_source_sha256": "ebfecf3407e05c576bcddf74673d2e7568207ecc27855aa0e08c453d5a0d119a",
    },
    12323: {
        "task_id": 2199,
        "recovery_source_task_asset_id": 0,
        "original_storage_ref_id": "c0a135a1-080f-46a0-a41a-461aef0ea0fb",
        "recovery_source_storage_ref_id": "",
        "expected_file_size": 17755216,
        "preview_whole_hash": "82b35a045540d27f9656d6d02c99eb2814a62e9d048d33b20823fb8c0017aa4c",
        "design_thumb_whole_hash": "54dbf569874243a212c11c3e83e80f19944c2581f12c9473a793bc273ec666a3",
        "rejected_source_task_asset_ids": [14510, 14514],
        "object_probe_result": "not_found",
        "object_probe_input_manifest_sha256": "3f17b37296d2670235ca9bfcfd4388823b81adecf8fbac0826e6f241923579c7",
        "object_probe_evidence_hash": "f1c78819e1f3d5f4e7a4b25ff3d173368574a5639f4c6df45c8aae5482d047b8",
        "object_probe_object_key_sha256": "e732f6cd269a93d6bac168b0852dbcf8480af8966847278cb073cd6905b0efdd",
        "object_probe_read_only_get_count": 1,
    },
}


def is_loopback(host: str) -> bool:
    if host.lower() == "localhost":
        return True
    try:
        return ipaddress.ip_address(host).is_loopback
    except ValueError:
        return False


def canonical_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"), sort_keys=True)


def sha256_json(value: Any) -> str:
    return hashlib.sha256(canonical_json(value).encode()).hexdigest()


def ordered_review_policy_ids(values: Iterable[str]) -> list[str]:
    selected = set(values)
    unknown = selected - set(REVIEW_POLICY_ORDER)
    if unknown:
        raise ValueError(f"unknown review policy ids: {sorted(unknown)}")
    return [policy for policy in REVIEW_POLICY_ORDER if policy in selected]


def revision_review_policy_ids(
    scope: dict[str, Any], revision: dict[str, Any]
) -> list[str]:
    policies = [EXPLICIT_EVENT_REPLAY_POLICY]
    if revision.get("source_alias_from_task_asset_id") is not None:
        policies.append(DELIVERY_SOURCE_ALIAS_POLICY)
    if revision.get("status") == "rejected":
        policies.append(REJECTED_HISTORY_POLICY)
    if revision.get("source_stage") == "reopen":
        policies.append(REOPEN_POLICY)
    if revision.get("reason", "").startswith(
        f"policy {POST_CLOSE_REPLACEMENT_POLICY}:"
    ):
        policies.append(POST_CLOSE_REPLACEMENT_POLICY)
    has_source = any(
        revision.get(field) is not None
        for field in (
            "source_task_asset_id",
            "source_alias_from_task_asset_id",
            "source_bundle",
        )
    )
    if scope.get("scope_kind") == "retouch_requirement" and not has_source:
        policies.append(RETOUCH_SOURCE_OPTIONAL_POLICY)
    if revision.get("reason", "").startswith(
        f"policy {RETOUCH_TERMINAL_SUBMIT_POLICY}:"
    ):
        policies.append(RETOUCH_TERMINAL_SUBMIT_POLICY)
    if revision.get("reason", "").startswith(
        f"policy {RETOUCH_UNSCOPED_ATOMIC_BATCH_POLICY}:"
    ):
        policies.append(RETOUCH_UNSCOPED_ATOMIC_BATCH_POLICY)
    if revision.get("reason", "").startswith(
        f"policy {RETOUCH_PREMATURE_TERMINAL_PARTIAL_POLICY}:"
    ):
        policies.append(RETOUCH_PREMATURE_TERMINAL_PARTIAL_POLICY)
    if revision.get("reason", "").startswith(
        f"policy {RETOUCH_VISUAL_SCOPE_TASK2533_POLICY}:"
    ):
        policies.append(RETOUCH_VISUAL_SCOPE_TASK2533_POLICY)
    if revision.get("reason", "").startswith(f"policy {BATCH_SUBMIT_POLICY}:"):
        policies.append(BATCH_SUBMIT_POLICY)
    if revision.get("reason", "").startswith(
        f"policy {ATOMIC_UPLOAD_BATCH_SUBMIT_POLICY}:"
    ):
        policies.append(ATOMIC_UPLOAD_BATCH_SUBMIT_POLICY)
    if revision.get("reason", "").startswith(
        f"policy {CUSTOMIZATION_TERMINAL_WITHOUT_ASSETS_POLICY}:"
    ):
        policies.append(CUSTOMIZATION_TERMINAL_WITHOUT_ASSETS_POLICY)
    policies.extend(revision.get("_review_policy_ids") or [])
    return ordered_review_policy_ids(policies)


def scope_matches(scope: dict[str, Any], row: dict[str, Any]) -> bool:
    """Match exact scope, with one provable legacy exception for a sole SKU."""
    if int(scope["task_id"]) != int(row["task_id"]):
        return False
    if scope["scope_kind"] == "task":
        return not row.get("scope_sku_code") and not row.get("retouch_requirement_id")
    if scope["scope_kind"] == "sku":
        if row.get("retouch_requirement_id"):
            return False
        if row.get("scope_sku_code") == scope.get("sku_code"):
            return True
        return bool(scope.get("_single_sku")) and not row.get("scope_sku_code")
    requirement_id = int(row.get("retouch_requirement_id") or 0)
    if requirement_id == int(scope["scope_ref_id"]):
        return True
    return bool(scope.get("_single_requirement")) and requirement_id == 0 and not row.get("scope_sku_code")


def reference_scope_matches(scope: dict[str, Any], row: dict[str, Any]) -> bool:
    """References inherit from task scope, matching the V8 runtime contract."""
    if int(scope["task_id"]) != int(row["task_id"]):
        return False
    is_task_reference = not row.get("scope_sku_code") and not row.get("retouch_requirement_id")
    if scope["scope_kind"] == "task":
        return is_task_reference
    if is_task_reference:
        return True
    if scope["scope_kind"] == "sku":
        return not row.get("retouch_requirement_id") and row.get("scope_sku_code") == scope.get("sku_code")
    return not row.get("scope_sku_code") and int(row.get("retouch_requirement_id") or 0) == int(scope["scope_ref_id"])


def event_payload(event: dict[str, Any]) -> dict[str, Any]:
    payload = event.get("payload") or {}
    if isinstance(payload, str):
        try:
            payload = json.loads(payload)
        except json.JSONDecodeError:
            return {}
    return payload if isinstance(payload, dict) else {}


def event_kind(event: dict[str, Any]) -> str | None:
    event_type = str(event.get("event_type") or "").lower()
    if event_type == "task.status.changed":
        payload = event_payload(event)
        if (
            str(payload.get("from_task_status") or "") == "Completed"
            and str(payload.get("to_task_status") or "") == "InProgress"
            and "reopen" in str(payload.get("reason") or "").lower()
        ):
            return "reopen"
    if event_type in SUBMIT_EVENTS:
        return "submit"
    if event_type in APPROVE_EVENTS:
        return "approve"
    if event_type in REJECT_EVENTS:
        return "reject"
    if event_type in REOPEN_EVENTS:
        return "reopen"
    if event_type in CLOSE_EVENTS:
        return "close"
    if event_type in WAREHOUSE_REOPEN_EVENTS:
        return "reopen"
    if event_type in SUPPLEMENT_EVENTS:
        return "supplement"
    if event_type == "task.customization.reviewed":
        payload = event_payload(event)
        action = str(
            payload.get("customization_review_decision")
            or payload.get("action")
            or payload.get("decision")
            or ""
        ).lower()
        if action in {"approved", "approve", "pass", "passed"}:
            return "approve"
        if action in {"rejected", "reject", "return", "returned", "return_to_designer"}:
            return "reject"
    return None


def active_asset(row: dict[str, Any]) -> bool:
    return row.get("upload_status") == "uploaded" and not any(
        row.get(key) for key in ("deleted_at", "cleaned_at", "access_revoked_at", "object_deleted_at")
    )


def planning_image_candidates(
    rows: dict[str, list[dict[str, Any]]],
) -> dict[tuple[int, str], list[dict[str, Any]]]:
    """Index exact, usable legacy ERP product images by task and SKU code."""
    indexed: dict[tuple[int, str], list[dict[str, Any]]] = defaultdict(list)
    for asset in rows.get("assets", []):
        sku_code = str(asset.get("scope_sku_code") or "").strip(" ")
        storage_ref_id = str(asset.get("storage_ref_id") or "")
        if (
            asset.get("asset_type") != "erp_product_image"
            or not active_asset(asset)
            or bool(asset.get("is_archived"))
            or asset.get("superseded_by_version_id") is not None
            or not sku_code
            or not storage_ref_id
            or not str(asset.get("ref_key") or "").strip(" ")
            or str(asset.get("storage_owner_type") or "") != "task_asset"
            or int(asset.get("storage_owner_id") or 0) != int(asset["id"])
            or bool(asset.get("is_placeholder"))
            or str(asset.get("storage_status") or "") not in {"active", "recorded"}
        ):
            continue
        indexed[(int(asset["task_id"]), sku_code)].append(asset)
    for candidates in indexed.values():
        candidates.sort(key=lambda row: int(row["id"]))
    return indexed


def revision_asset_eligible(row: dict[str, Any], expected_modules: set[str], when: str = "") -> tuple[bool, str]:
    if row.get("asset_type") not in {"source", "delivery"}:
        return False, "asset role is not source/delivery"
    if row.get("upload_status") != "uploaded":
        return False, "asset lifecycle is not active/uploaded"
    for key in ("deleted_at", "cleaned_at", "access_revoked_at", "object_deleted_at"):
        stamp = str(row.get(key) or "")
        if stamp and (not when or stamp <= when):
            return False, f"asset lifecycle reached {key}"
    superseded_at = str(row.get("superseded_at") or "")
    if row.get("superseded_by_version_id") and (not when or not superseded_at or superseded_at <= when):
        return False, "asset version is rejected, cleaned, or superseded"
    review_status = str(row.get("flow_review_status") or "").lower()
    rejected_at = str(row.get("rejected_at") or "")
    if review_status == "rejected" and (not when or not rejected_at or rejected_at <= when):
        return False, "asset version is rejected, cleaned, or superseded"
    if review_status == "cleaned":
        return False, "asset version is rejected, cleaned, or superseded"
    if review_status == "superseded" and (not when or not superseded_at or superseded_at <= when):
        return False, "asset version is rejected, cleaned, or superseded"
    module_key = str(row.get("source_module_key") or "").strip().lower()
    if expected_modules and module_key not in expected_modules:
        return False, f"asset source_module_key={module_key or '<empty>'} is outside expected stage {sorted(expected_modules)}"
    return True, ""


def at_or_before(row: dict[str, Any], when: str) -> bool:
    # task_assets.uploaded_at in the production legacy rows is not a stable
    # business boundary (some rows differ from created_at by +8h). Replay must
    # use the database creation timestamp consistently.
    stamp = row.get("created_at") or ""
    return bool(stamp and stamp <= when)


def stable_event_id(event: dict[str, Any]) -> str:
    return f'{event["namespace"]}:{event["id"]}'


def event_evidence_ids(event: dict[str, Any]) -> list[str]:
    return [stable_event_id(event)] + list(event.get("_duplicate_evidence_ids") or [])


def sorted_evidence_ids(
    evidence_ids: Iterable[str], all_events: Iterable[dict[str, Any]]
) -> list[str]:
    """Return unique evidence in the database's stable chronological order."""
    event_by_id = {
        stable_event_id(event): event
        for event in all_events
    }
    unique = list(dict.fromkeys(evidence_ids))

    def sort_key(stable_id: str) -> tuple[Any, ...]:
        event = event_by_id.get(stable_id)
        if event is None:
            return ("9999-12-31T23:59:59Z", 2, 2**63 - 1, stable_id)
        namespace = str(event.get("namespace") or "")
        try:
            sequence = int(event.get("sequence") or 0)
        except (TypeError, ValueError):
            sequence = 0
        return (
            str(event.get("created_at") or ""),
            1 if namespace == "task_module_event" else 0,
            sequence,
            stable_id,
        )

    return sorted(unique, key=sort_key)


def merge_member_completion_evidence(
    revision: dict[str, Any], all_events: Iterable[dict[str, Any]]
) -> None:
    """Carry exact upload-completion evidence for every inherited snapshot member."""
    member_ids = list(revision.get("final_task_asset_ids") or [])
    for field in ("source_task_asset_id", "source_alias_from_task_asset_id"):
        if revision.get(field) is not None:
            member_ids.append(int(revision[field]))
    bundle = revision.get("source_bundle") or {}
    if bundle.get("task_asset_id") is not None:
        member_ids.append(int(bundle["task_asset_id"]))
    for member in bundle.get("members") or []:
        if member.get("task_asset_id") is not None:
            member_ids.append(int(member["task_asset_id"]))
    if not member_ids:
        return
    boundary = str(
        revision.get("finalized_at")
        or revision.get("submitted_at")
        or revision.get("created_at")
        or ""
    )
    evidence = list(revision.get("evidence_event_ids") or [])
    targets = set(member_ids)
    for event in all_events:
        if (
            str(event.get("event_type") or "").lower()
            not in UPLOAD_SESSION_COMPLETED_EVENTS
            or (boundary and str(event.get("created_at") or "") > boundary)
        ):
            continue
        if targets.intersection(payload_asset_version_ids(event_payload(event))):
            evidence.extend(event_evidence_ids(event))
    revision["evidence_event_ids"] = evidence


def clear_rejected_members_from_reopen_draft(
    revision: dict[str, Any], assets: Iterable[dict[str, Any]]
) -> None:
    """A rejection clone keeps history/references, not rejected current members."""
    if revision.get("status") != "draft" or revision.get("source_stage") != "reopen":
        return
    boundary = str(revision.get("created_at") or "")
    asset_by_id = {int(asset["id"]): asset for asset in assets}

    def rejected(member_id: Any) -> bool:
        asset = asset_by_id.get(int(member_id or 0))
        rejected_at = str((asset or {}).get("rejected_at") or "")
        return bool(
            asset
            and str(asset.get("flow_review_status") or "").lower() == "rejected"
            and rejected_at
            and rejected_at <= boundary
        )

    for field in ("source_task_asset_id", "source_alias_from_task_asset_id"):
        if revision.get(field) is not None and rejected(revision[field]):
            revision.pop(field, None)
    revision["final_task_asset_ids"] = [
        int(member_id)
        for member_id in revision.get("final_task_asset_ids") or []
        if not rejected(member_id)
    ]
    revision["mode"] = (
        "set" if len(revision["final_task_asset_ids"]) > 1 else "single"
    )


def prune_inherited_reopen_snapshot(
    revision: dict[str, Any],
    assets: Iterable[dict[str, Any]],
    event: dict[str, Any],
    scope_kind: str,
    protected_member_ids: Iterable[int] = (),
) -> None:
    """Drop lifecycle-invalid inherited members from an explicit reopen snapshot.

    Upload completion and audit-supplement handlers may persist their asset
    lifecycle cleanup a few seconds after the immutable event row. Treat that
    bounded cleanup as part of the same operation; other reopen boundaries use
    the exact event timestamp.
    """

    if revision.get("source_stage") != "reopen":
        return
    boundary = str(event.get("created_at") or "")
    event_type = str(event.get("event_type") or "").lower()
    if event_type in UPLOAD_SESSION_COMPLETED_EVENTS or event_type == "task.audit.supplement_uploaded":
        boundary = (
            parse_utc_timestamp(boundary)
            + dt.timedelta(seconds=UPLOAD_CLEANUP_GRACE_SECONDS)
        ).isoformat().replace("+00:00", "Z")
    asset_by_id = {int(asset["id"]): asset for asset in assets}
    protected = {int(member_id) for member_id in protected_member_ids}
    exact_boundary = str(event.get("created_at") or "")

    def eligible(member_id: Any) -> bool:
        parsed_member_id = int(member_id or 0)
        asset = asset_by_id.get(parsed_member_id)
        if parsed_member_id in protected:
            allowed_at_event, _ = revision_asset_eligible(
                asset or {}, set(), exact_boundary
            )
            if allowed_at_event:
                return True
        allowed, _ = revision_asset_eligible(asset or {}, set(), boundary)
        return allowed

    for field in ("source_task_asset_id", "source_alias_from_task_asset_id"):
        if revision.get(field) is not None and not eligible(revision[field]):
            revision.pop(field, None)
    revision["final_task_asset_ids"] = [
        int(member_id)
        for member_id in revision.get("final_task_asset_ids") or []
        if eligible(member_id)
    ]
    if (
        scope_kind != "retouch_requirement"
        and not revision.get("source_task_asset_id")
        and not revision.get("source_alias_from_task_asset_id")
        and revision["final_task_asset_ids"]
    ):
        revision["source_alias_from_task_asset_id"] = revision[
            "final_task_asset_ids"
        ][0]
    revision["mode"] = (
        "set" if len(revision["final_task_asset_ids"]) > 1 else "single"
    )


def scoped_assets(scope, assets, when):
    return sorted(
        (a for a in assets if scope_matches(scope, a) and active_asset(a) and at_or_before(a, when)),
        key=lambda x: (x["created_at"], int(x["id"])),
    )


def scoped_references(scope, references, when):
    return sorted(
        (r for r in references if reference_scope_matches(scope, r) and r["attached_at"] <= when),
        key=lambda x: (x["attached_at"], int(x["id"])),
    )


def add_blocker(revision: dict[str, Any], reason: str) -> None:
    blockers = revision.setdefault("_blockers", [])
    if reason not in blockers:
        blockers.append(reason)
    revision["confidence"] = "hard_blocked"


def recompute_revision_hash(revision: dict[str, Any]) -> None:
    revision["mode"] = "set" if len(revision["final_task_asset_ids"]) > 1 else "single"
    revision["manifest_row_hash"] = sha256_json(
        {k: v for k, v in revision.items() if k not in {"manifest_row_hash", "_blockers"}}
    )


def recompute_planning_hash(planning: dict[str, Any]) -> None:
    planning["manifest_row_hash"] = sha256_json(
        {key: value for key, value in planning.items() if key != "manifest_row_hash"}
    )


def recompute_mapping_row_hash(row: dict[str, Any]) -> None:
    row["manifest_row_hash"] = sha256_json(
        {key: value for key, value in row.items() if key != "manifest_row_hash"}
    )


def make_revision(
    scope,
    event,
    revision_no,
    status,
    stage,
    assets,
    references,
    inherited=None,
    selected_assets=None,
    extra_evidence=None,
    selection_blockers=None,
):
    # Revision membership is event-linked, never "every asset that existed
    # before the boundary". Reopen rows inherit an immutable snapshot; submit
    # and audit rows must pass an explicit selected_assets set.
    eligible = list(selected_assets or [])
    sources = [a for a in eligible if a["asset_type"] == "source"]
    finals = [a for a in eligible if a["asset_type"] == "delivery"]
    if inherited is not None:
        source_id = inherited.get("source_task_asset_id")
        source_alias_id = inherited.get("source_alias_from_task_asset_id")
        final_ids = list(inherited.get("final_task_asset_ids", []))
        reference_ids = list(inherited.get("reference_file_ref_ids", []))
    else:
        source_id = int(sources[0]["id"]) if len(sources) == 1 else None
        source_alias_id = None
        batch_membership = (
            event.get("_batch_scope_memberships") or {}
        ).get(str(scope.get("sku_code") or ""))
        if (
            not sources
            and isinstance(batch_membership, dict)
            and batch_membership.get("source_alias_asset_version_id")
        ):
            source_alias_id = int(
                batch_membership["source_alias_asset_version_id"]
            )
        final_ids = [int(a["id"]) for a in finals]
        reference_ids = [int(r["id"]) for r in scoped_references(scope, references, event["created_at"])]
    policy = str(event.get("_migration_policy") or "").strip()
    reason = "candidate reconstructed from explicit legacy workflow boundaries; human confirmation remains required"
    if policy == BATCH_SUBMIT_POLICY:
        reason = (
            f"policy {policy}: the last scoped submit triggers the task-level "
            "atomic transition after independently proven full SKU coverage; "
            "human confirmation remains required"
        )
    elif event.get("_atomic_upload_batch"):
        reason = (
            f"policy {ATOMIC_UPLOAD_BATCH_SUBMIT_POLICY}: contiguous "
            "same-actor completed upload sessions around the submit boundary "
            "form one deterministic atomic submission snapshot"
        )
    elif policy:
        reason = (
            f"policy {policy}: frozen exception-list facts matched exactly; "
            "human confirmation remains required"
        )
    revision = {
        "revision_no": revision_no,
        "status": status,
        "mode": "set" if len(final_ids) > 1 else "single",
        "source_stage": stage,
        "final_task_asset_ids": final_ids,
        "reference_file_ref_ids": reference_ids,
        "evidence_event_ids": list(extra_evidence or []) + event_evidence_ids(event),
        "confidence": "proposed_review",
        "confirmed_by": 0,
        "confirmed_at": ZERO_TIME,
        "confirmation_note": "",
        "manifest_row_hash": "",
        "reason": reason,
        "created_by": int(event.get("actor_id") or 0),
        "created_at": event["created_at"],
        "_blockers": [],
    }
    if source_id:
        revision["source_task_asset_id"] = source_id
    if source_alias_id:
        revision["source_alias_from_task_asset_id"] = source_alias_id
    if status in {"submitted", "finalized"}:
        revision["submitted_at"] = event["created_at"]
    if status == "finalized":
        revision["finalized_at"] = event["created_at"]
    if inherited is None and len(sources) > 1:
        revision["source_bundle_candidate"] = {
            "ordered_member_task_asset_ids": [
                int(asset["id"]) for asset in sources
            ],
            "ordering": "completion_time_then_task_asset_id",
        }
        add_blocker(revision, "multiple source assets require a reviewed deterministic ZIP bundle")
    for blocker in selection_blockers or []:
        add_blocker(revision, blocker)
    if (
        inherited is None
        and scope["scope_kind"] != "retouch_requirement"
        and not revision.get("source_task_asset_id")
        and final_ids
    ):
        # Legacy submissions frequently used one completed session as the
        # state-transition trigger while several delivery files formed the
        # atomic final set. In the absence of an independent source file, the
        # first event-ordered delivery is the deterministic immutable source
        # alias; the complete ordered delivery set remains intact.
        revision["source_alias_from_task_asset_id"] = final_ids[0]
    if status in {"submitted", "finalized"} and scope["scope_kind"] != "retouch_requirement" and not revision.get("source_task_asset_id") and not revision.get("source_alias_from_task_asset_id"):
        add_blocker(revision, "design revision has no uniquely evidenced source asset")
    if status == "finalized" and not revision["final_task_asset_ids"]:
        add_blocker(revision, "finalized revision has no delivery asset at the event timestamp")
    if not event.get("actor_id"):
        add_blocker(revision, "legacy event has no actor_id")
    recompute_revision_hash(revision)
    return revision


def payload_ids(payload: dict[str, Any], *keys: str) -> list[int]:
    values: list[int] = []
    for key in keys:
        raw = payload.get(key)
        if raw is None:
            continue
        raw_values = raw if isinstance(raw, list) else [raw]
        for value in raw_values:
            try:
                parsed = int(value)
            except (TypeError, ValueError):
                continue
            if parsed > 0 and parsed not in values:
                values.append(parsed)
    return values


def nested_number(payload: dict[str, Any], key: str) -> int | None:
    value = payload.get(key)
    if isinstance(value, dict):
        value = value.get("count") or value.get("asset_count") or value.get("version_count")
    try:
        return int(value) if value is not None else None
    except (TypeError, ValueError):
        return None


def event_precedes_boundary(
    candidate: dict[str, Any],
    boundary: dict[str, Any],
) -> bool:
    """Use immutable task-event sequence when a repaired timestamp drifted."""
    if (
        event_payload(boundary).get("repair_reason")
        and candidate.get("namespace") == "task_event_log"
        and boundary.get("namespace") == "task_event_log"
        and int(candidate.get("task_id") or 0)
        == int(boundary.get("task_id") or 0)
        and int(candidate.get("sequence") or 0) > 0
        and int(boundary.get("sequence") or 0) > 0
    ):
        return int(candidate["sequence"]) <= int(boundary["sequence"])
    return str(candidate.get("created_at") or "") <= str(
        boundary.get("created_at") or ""
    )


def boundary_membership_time(
    boundary: dict[str, Any],
    completion: dict[str, Any],
) -> str:
    payload = event_payload(boundary)
    if (
        payload.get("repair_reason")
        and event_precedes_boundary(completion, boundary)
    ):
        return max(
            str(boundary.get("created_at") or ""),
            str(completion.get("created_at") or ""),
        )
    if boundary.get("_atomic_upload_batch"):
        return max(
            str(boundary.get("created_at") or ""),
            str(completion.get("created_at") or ""),
        )
    return str(boundary.get("created_at") or "")


def payload_asset_version_ids(payload: dict[str, Any], *extra_keys: str) -> list[int]:
    # asset_id is a design-asset/root identifier in the real legacy payload;
    # only asset_version_id identifies task_assets.id.
    return payload_ids(
        payload,
        "asset_version_id",
        "asset_version_ids",
        "task_asset_id",
        "task_asset_ids",
        *extra_keys,
    )


def expected_submit_modules(scope: dict[str, Any], event: dict[str, Any]) -> set[str]:
    payload = event_payload(event)
    explicit = str(event.get("module_key") or payload.get("module_key") or payload.get("from_module") or "").strip().lower()
    if explicit:
        return {explicit}
    if scope["scope_kind"] == "retouch_requirement":
        return {"retouch"}
    return {"design", "customization"}


def resolve_submission_assets(scope, submit_event, all_events, assets):
    retouch_memberships = submit_event.get("_retouch_scope_memberships")
    if isinstance(retouch_memberships, dict):
        membership = retouch_memberships.get(str(scope.get("scope_ref_id") or ""))
        if not isinstance(membership, dict):
            return [], [], [
                f"{submit_event.get('_migration_policy')}: no reviewed membership "
                "exists for this retouch requirement"
            ]
        asset_by_id = {int(asset["id"]): asset for asset in assets}
        selected = []
        for asset_id in membership.get("asset_version_ids") or []:
            asset = asset_by_id.get(int(asset_id))
            if asset is None:
                return [], [], [
                    f"{submit_event.get('_migration_policy')}: asset_version_id "
                    f"{asset_id} no longer resolves"
                ]
            selected.append(asset)
        return (
            selected,
            list(membership.get("completion_event_ids") or []),
            [],
        )

    batch_memberships = submit_event.get("_batch_scope_memberships")
    if isinstance(batch_memberships, dict):
        membership = batch_memberships.get(str(scope.get("sku_code") or ""))
        if not isinstance(membership, dict):
            return [], [], [f"{BATCH_SUBMIT_POLICY}: no reviewed membership exists for this SKU scope"]
        asset_by_id = {int(candidate["id"]): candidate for candidate in assets}
        selected = []
        for asset_id in membership.get("asset_version_ids") or []:
            asset = asset_by_id.get(int(asset_id))
            if asset is None:
                return [], [], [
                    f"{BATCH_SUBMIT_POLICY}: asset_version_id {asset_id} "
                    "no longer resolves"
                ]
            selected.append(asset)
        return (
            selected,
            [str(value) for value in membership.get("completion_event_ids") or []],
            [],
        )

    payload = event_payload(submit_event)
    session_id = str(payload.get("upload_session_id") or "").strip()
    blockers: list[str] = []
    if not session_id:
        return [], [], ["design submission lacks an explicit upload_session_id"]
    by_version_id = {int(asset["id"]): asset for asset in assets}
    expected_modules = expected_submit_modules(scope, submit_event)
    completions = []
    for event in all_events:
        if str(event.get("event_type") or "").lower() not in UPLOAD_SESSION_COMPLETED_EVENTS:
            continue
        if not event_precedes_boundary(event, submit_event):
            continue
        completion_payload = event_payload(event)
        if str(completion_payload.get("upload_session_id") or "").strip() == session_id:
            completions.append(event)
    if not completions:
        return [], [], [f"upload session {session_id} has no completed event before submission"]
    submit_actor = int(submit_event.get("actor_id") or 0)
    try:
        submit_time = parse_utc_timestamp(str(submit_event.get("created_at") or ""))
    except (TypeError, ValueError):
        submit_time = None
    explicit_completion_ids = {
        stable_event_id(event) for event in completions
    }
    if submit_actor > 0 and submit_time is not None:
        for sibling in all_events:
            if (
                str(sibling.get("event_type") or "").lower()
                not in UPLOAD_SESSION_COMPLETED_EVENTS
                or int(sibling.get("actor_id") or 0) != submit_actor
                or stable_event_id(sibling) in explicit_completion_ids
                or not event_precedes_boundary(sibling, submit_event)
            ):
                continue
            try:
                sibling_time = parse_utc_timestamp(
                    str(sibling.get("created_at") or "")
                )
            except (TypeError, ValueError):
                continue
            if abs((sibling_time - submit_time).total_seconds()) > 15 * 60:
                continue
            sibling_version_ids = payload_asset_version_ids(
                event_payload(sibling)
            )
            sibling_assets = [
                by_version_id.get(version_id)
                for version_id in sibling_version_ids
            ]
            if (
                not sibling_assets
                or any(asset is None for asset in sibling_assets)
                or any(
                    asset.get("asset_type") not in {"source", "delivery"}
                    or int(asset.get("task_id") or 0)
                    != int(scope["task_id"])
                    or not scope_matches(scope, asset)
                    or str(
                        asset.get("source_module_key") or ""
                    ).strip().lower()
                    not in expected_modules
                    for asset in sibling_assets
                )
            ):
                continue
            lower = min(
                str(sibling.get("created_at") or ""),
                str(submit_event.get("created_at") or ""),
            )
            upper = max(
                str(sibling.get("created_at") or ""),
                str(submit_event.get("created_at") or ""),
            )
            if any(
                boundary is not submit_event
                and event_kind(boundary) is not None
                and lower
                < str(boundary.get("created_at") or "")
                < upper
                for boundary in all_events
            ):
                continue
            completions.append(sibling)
            submit_event["_atomic_upload_batch"] = True
    selected, evidence = [], []
    referenced_version_ids: list[int] = []
    for completion in sorted(completions, key=lambda event: (event["created_at"], int(event.get("sequence") or 0), str(event["id"]))):
        version_ids = payload_asset_version_ids(event_payload(completion))
        if not version_ids:
            blockers.append(f"upload session completion {stable_event_id(completion)} lacks asset_version_id")
            continue
        referenced_version_ids.extend(version_ids)
        evidence.append(stable_event_id(completion))
        for version_id in version_ids:
            asset = by_version_id.get(version_id)
            if asset is None:
                blockers.append(f"asset_version_id {version_id} does not resolve to task_assets.id")
                continue
            if int(asset["task_id"]) != int(scope["task_id"]) or not scope_matches(scope, asset):
                blockers.append(f"asset_version_id {version_id} is outside the submitted task/scope")
                continue
            membership_time = boundary_membership_time(submit_event, completion)
            if not at_or_before(asset, membership_time):
                blockers.append(f"asset_version_id {version_id} was created after the submission boundary")
                continue
            eligible, reason = revision_asset_eligible(
                asset, expected_modules, membership_time
            )
            if not eligible:
                blockers.append(f"asset_version_id {version_id}: {reason}")
                continue
            if asset not in selected:
                selected.append(asset)
    if not referenced_version_ids:
        blockers.append(f"upload session {session_id} has no asset-version membership")
    return sorted(selected, key=lambda asset: (asset["created_at"], int(asset["id"]))), evidence, blockers


def is_legacy_retouch_terminal_submit(
    scope,
    submit_event,
    boundary,
    all_events,
    selected_assets,
    selection_blockers,
):
    """Recognize a legacy retouch submit that terminally completed its task."""
    if submit_event.get("_migration_policy") == RETOUCH_UNSCOPED_ATOMIC_BATCH_POLICY:
        membership = (
            submit_event.get("_retouch_scope_memberships") or {}
        ).get(str(scope.get("scope_ref_id") or ""))
        return bool(
            scope.get("scope_kind") == "retouch_requirement"
            and str(scope.get("task_status") or "") == "Completed"
            and isinstance(membership, dict)
            and membership.get("asset_version_ids")
            and [int(asset["id"]) for asset in selected_assets]
            == [int(value) for value in membership["asset_version_ids"]]
            and not selection_blockers
        )
    if (
        scope.get("scope_kind") != "retouch_requirement"
        or str(scope.get("task_status") or "") != "Completed"
        or event_kind(submit_event) != "submit"
        or selection_blockers
        or not int(submit_event.get("actor_id") or 0)
    ):
        return False

    final_assets = [
        asset for asset in selected_assets if asset.get("asset_type") == "delivery"
    ]
    if (
        not final_assets
        or any(
            asset.get("asset_type") not in {"source", "delivery"}
            for asset in selected_assets
        )
    ):
        return False
    for asset in selected_assets:
        requirement_id = int(asset.get("retouch_requirement_id") or 0)
        if requirement_id:
            if requirement_id != int(scope["scope_ref_id"]):
                return False
        elif not scope.get("_single_requirement"):
            # Legacy retouch rows omitted requirement scope. That omission is
            # unambiguous only when the task has exactly one requirement.
            return False

        asset_session_id = str(
            asset.get("upload_session_id")
            or asset.get("upload_request_id")
            or ""
        ).strip()
        if not asset_session_id:
            return False
        completions = []
        for event in all_events:
            if (
                str(event.get("event_type") or "").lower()
                not in UPLOAD_SESSION_COMPLETED_EVENTS
                or not event_precedes_boundary(event, submit_event)
            ):
                continue
            payload = event_payload(event)
            if (
                str(payload.get("upload_session_id") or "").strip()
                == asset_session_id
                and int(asset["id"])
                in payload_asset_version_ids(payload)
            ):
                completions.append(event)
        if (
            len(completions) != 1
            or int(completions[0].get("actor_id") or 0)
            != int(submit_event["actor_id"])
        ):
            return False

    submit_index = next(
        (
            index
            for index, event in enumerate(boundary)
            if stable_event_id(event) == stable_event_id(submit_event)
        ),
        -1,
    )
    if submit_index < 0:
        return False
    later_transitions = [
        event_kind(event)
        for event in boundary[submit_index + 1 :]
        if event_kind(event) in {"submit", "reject", "reopen"}
    ]
    # A proven reopen means the earlier retouch submit really did enter the
    # terminal state before its immutable snapshot was superseded.
    return not later_transitions or later_transitions[0] == "reopen"


def apply_explicit_audit_change(scope, event, assets, revision):
    payload = event_payload(event)
    source_ids = payload_ids(payload, "replace_source_asset_version_id", "source_asset_version_id")
    replace_final_ids = payload_ids(payload, "replace_final_asset_version_ids", "final_asset_version_ids")
    append_final_ids = payload_ids(payload, "append_asset_version_ids", "append_final_asset_version_ids")
    session_id = str(payload.get("upload_session_id") or payload.get("asset_upload_session_id") or "")
    before = nested_number(payload, "before")
    after = nested_number(payload, "after")
    if before is None:
        before = nested_number(payload, "audit_delivery_count_before")
    if after is None:
        after = nested_number(payload, "audit_delivery_count_after")
    direct_version_ids = payload_asset_version_ids(payload)
    if before is not None and after == before + 1:
        append_final_ids.extend(version_id for version_id in direct_version_ids if version_id not in append_final_ids)
    expected_modules = {"audit"}
    eligible = {}
    for asset in assets:
        if not scope_matches(scope, asset) or not at_or_before(asset, event["created_at"]):
            continue
        allowed, _ = revision_asset_eligible(asset, expected_modules, event["created_at"])
        if allowed:
            eligible[int(asset["id"])] = asset
    if session_id and not source_ids and not replace_final_ids and not append_final_ids:
        session_assets = [
            a for a in eligible.values()
            if str(a.get("upload_session_id") or a.get("upload_request_id") or "") == session_id
        ]
        final_mode = str(payload.get("final_mode") or payload.get("asset_operation") or "").lower()
        source_ids = [int(a["id"]) for a in session_assets if a["asset_type"] == "source"]
        session_finals = [int(a["id"]) for a in session_assets if a["asset_type"] == "delivery"]
        if final_mode == "replace":
            replace_final_ids = session_finals
        elif final_mode == "append":
            append_final_ids = session_finals
        elif session_finals:
            add_blocker(revision, "upload session has delivery assets but no explicit replace/append operation")
    explicit = bool(source_ids or replace_final_ids or append_final_ids)
    for asset_id in source_ids + replace_final_ids + append_final_ids:
        if asset_id not in eligible:
            add_blocker(revision, f"explicit audit asset {asset_id} is missing, inactive, late, or outside the resource scope")
    valid_sources = [asset_id for asset_id in source_ids if asset_id in eligible and eligible[asset_id]["asset_type"] == "source"]
    if len(valid_sources) > 1:
        revision.pop("source_task_asset_id", None)
        add_blocker(revision, "explicit audit replacement contains multiple sources and requires a reviewed deterministic ZIP bundle")
    elif len(valid_sources) == 1:
        revision["source_task_asset_id"] = valid_sources[0]
        revision.pop("source_alias_from_task_asset_id", None)
    if replace_final_ids:
        revision["final_task_asset_ids"] = [asset_id for asset_id in replace_final_ids if asset_id in eligible and eligible[asset_id]["asset_type"] == "delivery"]
        alias_id = revision.get("source_alias_from_task_asset_id")
        if alias_id and alias_id not in revision["final_task_asset_ids"]:
            revision.pop("source_alias_from_task_asset_id", None)
            add_blocker(revision, "replacing finals removed the delivery that backed the source alias; select a new source")
    if append_final_ids:
        for asset_id in append_final_ids:
            if asset_id in eligible and eligible[asset_id]["asset_type"] == "delivery" and asset_id not in revision["final_task_asset_ids"]:
                revision["final_task_asset_ids"].append(asset_id)
    recompute_revision_hash(revision)
    return explicit


def apply_proven_successor_audit_change(scope, event, events, assets, revision):
    """Apply an immutable version replacement chain proven by legacy links.

    Some reviewer uploads do not repeat replace/append semantics in the
    approval payload, but task_assets.superseded_by_version_id is an explicit
    version-root edge. A reviewer can upload more than one replacement before
    approving, so replay follows the complete acyclic chain to the version
    eligible at approval. Every hop must be same-scope, same-role, present
    before approval, and have exactly one completed upload-session membership
    event before approval.
    """
    asset_by_id = {int(asset["id"]): asset for asset in assets}
    member_ids = list(revision["final_task_asset_ids"])
    for key in ("source_task_asset_id", "source_alias_from_task_asset_id"):
        if revision.get(key) is not None:
            member_ids.append(int(revision[key]))
    replacements: dict[int, int] = {}
    completion_evidence: list[str] = []
    for old_id in dict.fromkeys(member_ids):
        old = asset_by_id.get(old_id)
        eligible, _ = revision_asset_eligible(old or {}, set(), event["created_at"])
        if eligible:
            continue
        if not old:
            add_blocker(
                revision,
                f"asset_version_id {old_id} is ineligible at approval without a resolvable successor",
            )
            return False
        current = old
        visited = {old_id}
        while True:
            successor_id = int(current.get("superseded_by_version_id") or 0)
            successor = asset_by_id.get(successor_id)
            if not successor or successor_id in visited:
                add_blocker(
                    revision,
                    f"asset_version_id {old_id} is ineligible at approval without an acyclic resolvable successor chain",
                )
                return False
            visited.add(successor_id)
            same_root = (
                not old.get("asset_id")
                or not successor.get("asset_id")
                or int(old["asset_id"]) == int(successor["asset_id"])
            )
            successor_ok, successor_reason = revision_asset_eligible(
                successor,
                {"audit", "customization", "design"},
                event["created_at"],
            )
            if (
                int(successor["task_id"]) != int(scope["task_id"])
                or not scope_matches(scope, successor)
                or successor.get("asset_type") != old.get("asset_type")
                or not same_root
                or not at_or_before(successor, event["created_at"])
            ):
                add_blocker(
                    revision,
                    f"asset_version_id {old_id} successor {successor_id} is not a same-root/scope/role approval-time replacement: {successor_reason}",
                )
                return False
            completions = [
                candidate
                for candidate in events
                if str(candidate.get("event_type") or "").lower()
                in UPLOAD_SESSION_COMPLETED_EVENTS
                and candidate.get("created_at", "") <= event["created_at"]
                and successor_id
                in payload_asset_version_ids(event_payload(candidate))
            ]
            if len(completions) != 1:
                add_blocker(
                    revision,
                    f"asset_version_id {old_id} successor {successor_id} has {len(completions)} completed upload membership events before approval",
                )
                return False
            completion_evidence.extend(event_evidence_ids(completions[0]))
            if successor_ok:
                replacements[old_id] = successor_id
                break
            current = successor
    if not replacements:
        return False
    if revision.get("source_task_asset_id") in replacements:
        revision["source_task_asset_id"] = replacements[int(revision["source_task_asset_id"])]
    if revision.get("source_alias_from_task_asset_id") in replacements:
        revision["source_alias_from_task_asset_id"] = replacements[int(revision["source_alias_from_task_asset_id"])]
    revision["final_task_asset_ids"] = list(dict.fromkeys(
        replacements.get(int(asset_id), int(asset_id))
        for asset_id in revision["final_task_asset_ids"]
    ))
    revision["evidence_event_ids"] = list(dict.fromkeys(
        list(revision.get("evidence_event_ids") or []) + completion_evidence
    ))
    recompute_revision_hash(revision)
    return True


def apply_proven_successor_audit_change_if_possible(
    scope, event, events, assets, revision
):
    """Apply optional current-snapshot repair without leaking failed blockers.

    The approval replay path is fail-closed and must retain exact successor
    errors.  The later current-snapshot pass is only an opportunistic repair:
    lifecycle pruning can still produce the correct active snapshot when an
    old member has no successor.  Run that pass on a deep copy so a failed
    probe cannot poison an otherwise valid pruned revision.
    """

    probe = copy.deepcopy(revision)
    if not apply_proven_successor_audit_change(
        scope, event, events, assets, probe
    ):
        return False
    revision.clear()
    revision.update(probe)
    return True


def parse_utc_timestamp(value: str) -> dt.datetime:
    return dt.datetime.fromisoformat(str(value).replace("Z", "+00:00"))


def apply_proven_legacy_audit_stage_snapshot(
    scope,
    event,
    events,
    assets,
    revision,
):
    """Apply a complete, event-linked legacy audit-stage snapshot.

    Delivery approval metadata is authoritative even when the auditor uploaded
    a batch long before clicking approve. Source-only batches inherit finals;
    a source uploaded by another actor must be paired with a delivery from the
    same short upload batch that the approval explicitly accepted.
    """
    submitted_at = str(revision.get("submitted_at") or "")
    approval_at = str(event.get("created_at") or "")
    actor_id = int(event.get("actor_id") or 0)
    if not submitted_at or not approval_at or actor_id <= 0:
        return False
    # Legacy review uploads were not consistently tagged with source_module_key
    # "audit". Membership is therefore proven by the approval actor/time,
    # completed upload sessions, and exact task/scope instead of that mutable
    # label. Unrelated late uploads are ignored unless they join the accepted
    # short review batch below.
    candidates = []
    completion_by_asset: dict[int, dict[str, Any]] = {}
    for asset in assets:
        stamp = str(asset.get("created_at") or "")
        eligible, _ = revision_asset_eligible(
            asset,
            {"audit", "customization", "design"},
            approval_at,
        )
        if not (
            eligible
            and scope_matches(scope, asset)
            and submitted_at < stamp <= approval_at
        ):
            continue
        asset_id = int(asset["id"])
        completions = [
            candidate
            for candidate in events
            if str(candidate.get("event_type") or "").lower()
            in UPLOAD_SESSION_COMPLETED_EVENTS
            and submitted_at < str(candidate.get("created_at") or "") <= approval_at
            and candidate.get("namespace") == "task_event_log"
            and asset_id in payload_asset_version_ids(event_payload(candidate))
        ]
        if len(completions) != 1:
            continue
        candidates.append(asset)
        completion_by_asset[asset_id] = completions[0]

    if not candidates:
        return False
    try:
        approval_time = parse_utc_timestamp(approval_at)
    except (TypeError, ValueError):
        return False

    approved_delivery_ids = set()
    for asset in candidates:
        if asset.get("asset_type") != "delivery":
            continue
        asset_id = int(asset["id"])
        completion = completion_by_asset[asset_id]
        approved_at = str(asset.get("approved_at") or "")
        approved_by = int(asset.get("approved_by") or 0)
        metadata_match = (
            approved_by == actor_id
            and approved_at
            and abs(
                (
                    parse_utc_timestamp(approved_at) - approval_time
                ).total_seconds()
            )
            <= 2
        )
        legacy_immediate_match = False
        try:
            legacy_immediate_match = (
                int(completion.get("actor_id") or 0) == actor_id
                and abs(
                    (
                        approval_time
                        - parse_utc_timestamp(
                            str(completion.get("created_at") or "")
                        )
                    ).total_seconds()
                )
                <= 15 * 60
            )
        except (TypeError, ValueError):
            pass
        if not metadata_match and not legacy_immediate_match:
            continue
        approved_delivery_ids.add(asset_id)

    source_ids = []
    for asset in candidates:
        if asset.get("asset_type") != "source":
            continue
        source_id = int(asset["id"])
        source_completion = completion_by_asset[source_id]
        source_actor = int(source_completion.get("actor_id") or 0)
        try:
            source_time = parse_utc_timestamp(
                str(source_completion.get("created_at") or "")
            )
        except (TypeError, ValueError):
            continue
        reviewer_upload = (
            source_actor == actor_id
            and (
                abs((approval_time - source_time).total_seconds()) <= 15 * 60
                or (
                    not approved_delivery_ids
                    and str(
                        asset.get("source_module_key") or ""
                    ).strip().lower()
                    == "audit"
                )
            )
        )
        paired_upload = any(
            int(completion_by_asset[final_id].get("actor_id") or 0)
            == source_actor
            and abs(
                (
                    parse_utc_timestamp(
                        str(
                            completion_by_asset[final_id].get("created_at")
                            or ""
                        )
                    )
                    - source_time
                ).total_seconds()
            )
            <= 15 * 60
            for final_id in approved_delivery_ids
        )
        if reviewer_upload or paired_upload:
            source_ids.append(source_id)

    final_ids = sorted(
        approved_delivery_ids,
        key=lambda asset_id: (
            str(next(
                asset["created_at"]
                for asset in candidates
                if int(asset["id"]) == asset_id
            )),
            asset_id,
        ),
    )
    if source_ids and not final_ids:
        # A source-only review inherits finals. When several historical
        # source-only batches exist between submit and approve, only the most
        # recent atomic batch can represent the approval-time working source;
        # older batches remain preserved assets but are not merged together.
        anchor_id = max(
            source_ids,
            key=lambda asset_id: (
                str(completion_by_asset[asset_id].get("created_at") or ""),
                asset_id,
            ),
        )
        anchor_event = completion_by_asset[anchor_id]
        anchor_actor = int(anchor_event.get("actor_id") or 0)
        anchor_time = parse_utc_timestamp(
            str(anchor_event.get("created_at") or "")
        )
        source_ids = [
            asset_id
            for asset_id in source_ids
            if int(
                completion_by_asset[asset_id].get("actor_id") or 0
            )
            == anchor_actor
            and abs(
                (
                    parse_utc_timestamp(
                        str(
                            completion_by_asset[asset_id].get("created_at")
                            or ""
                        )
                    )
                    - anchor_time
                ).total_seconds()
            )
            <= 15 * 60
        ]
    if not source_ids and not final_ids:
        return False

    completion_evidence = []
    for asset_id in source_ids + final_ids:
        completion_evidence.extend(
            event_evidence_ids(completion_by_asset[asset_id])
        )

    if len(source_ids) > 1:
        source_ids = sorted(
            source_ids,
            key=lambda asset_id: (
                str(
                    completion_by_asset[asset_id].get("created_at")
                    or ""
                ),
                asset_id,
            ),
        )
        revision.pop("source_task_asset_id", None)
        revision.pop("source_alias_from_task_asset_id", None)
        revision["source_bundle_candidate"] = {
            "ordered_member_task_asset_ids": list(source_ids),
            "ordering": "completion_time_then_task_asset_id",
        }
        add_blocker(
            revision,
            "multiple source assets require a reviewed deterministic ZIP bundle",
        )
    if source_ids:
        if len(source_ids) == 1:
            revision["source_task_asset_id"] = source_ids[0]
            revision.pop("source_alias_from_task_asset_id", None)
    elif (
        not revision.get("source_task_asset_id")
        and not revision.get("source_alias_from_task_asset_id")
        and len(final_ids) == 1
    ):
        revision["source_alias_from_task_asset_id"] = final_ids[0]
    if revision.get("source_task_asset_id") or revision.get(
        "source_alias_from_task_asset_id"
    ):
        revision["_blockers"] = [
            blocker
            for blocker in revision.get("_blockers", [])
            if blocker != "design revision has no uniquely evidenced source asset"
        ]
    if final_ids:
        revision["final_task_asset_ids"] = final_ids
        alias_id = revision.get("source_alias_from_task_asset_id")
        if (
            not revision.get("source_task_asset_id")
            and alias_id not in final_ids
        ):
            revision["source_alias_from_task_asset_id"] = final_ids[0]
    revision["evidence_event_ids"] = list(
        dict.fromkeys(
            list(revision.get("evidence_event_ids") or []) + completion_evidence
        )
    )
    revision.setdefault("_review_policy_ids", []).append(
        AUDIT_STAGE_FINAL_SNAPSHOT_POLICY
    )
    recompute_revision_hash(revision)
    return True


def assets_created_between(scope, assets, after, before):
    result = []
    for asset in assets:
        stamp = asset.get("created_at") or ""
        eligible, _ = revision_asset_eligible(asset, {"audit"}, before)
        if (
            eligible
            and scope_matches(scope, asset)
            and after < stamp <= before
        ):
            result.append(asset)
    return result


def direct_post_close_replacements(scope, events, assets):
    """Find exact same-root replacements uploaded after the last close boundary."""
    asset_by_id = {int(asset["id"]): asset for asset in assets}
    predecessor_by_successor: dict[int, list[dict[str, Any]]] = defaultdict(list)
    for asset in assets:
        successor_id = int(asset.get("superseded_by_version_id") or 0)
        if successor_id:
            predecessor_by_successor[successor_id].append(asset)

    def order_key(event):
        return (
            str(event.get("created_at") or ""),
            event.get("namespace") == "task_module_event",
            int(event.get("sequence") or 0),
            str(event.get("id") or ""),
        )

    boundaries = sorted(
        (event for event in events if event_kind(event)),
        key=order_key,
    )
    closes = [event for event in boundaries if event_kind(event) == "close"]
    terminal_retouch_submit = False
    if (
        scope.get("scope_kind") == "retouch_requirement"
        and str(scope.get("task_status") or "") == "Completed"
    ):
        retouch_submits = [
            event for event in boundaries if event_kind(event) == "submit"
        ]
        if retouch_submits and (
            not closes or order_key(retouch_submits[-1]) > order_key(closes[-1])
        ):
            closes.append(retouch_submits[-1])
            closes.sort(key=order_key)
            terminal_retouch_submit = True
    if not closes:
        return []
    last_close = closes[-1]
    last_close_key = order_key(last_close)
    replacements = []
    for completion in sorted(
        (
            event
            for event in events
            if str(event.get("event_type") or "").lower()
            in UPLOAD_SESSION_COMPLETED_EVENTS
        ),
        key=order_key,
    ):
        completion_key = order_key(completion)
        if completion_key <= last_close_key:
            continue
        if terminal_retouch_submit and not bool(
            event_payload(completion).get("post_close_replacement")
        ):
            continue
        if any(
            event_kind(event) in {"submit", "reject", "reopen"}
            and last_close_key < order_key(event) <= completion_key
            for event in boundaries
        ):
            continue
        completion_at = str(completion.get("created_at") or "")
        version_ids = payload_asset_version_ids(event_payload(completion))
        if len(version_ids) != 1:
            continue
        successor = asset_by_id.get(version_ids[0])
        predecessors = predecessor_by_successor.get(version_ids[0], [])
        if successor is None or len(predecessors) != 1:
            continue
        predecessor = predecessors[0]
        if (
            successor.get("asset_type") not in {"source", "delivery"}
            or predecessor.get("asset_type") != successor.get("asset_type")
            or int(predecessor.get("asset_id") or 0) <= 0
            or int(predecessor.get("asset_id") or 0)
            != int(successor.get("asset_id") or 0)
            or not scope_matches(scope, predecessor)
            or not scope_matches(scope, successor)
            or str(successor.get("created_at") or "") > completion_at
            or str(predecessor.get("superseded_at") or "") > completion_at
        ):
            continue
        session_id = str(
            event_payload(completion).get("upload_session_id") or ""
        ).strip()
        asset_session_id = str(
            successor.get("upload_session_id")
            or successor.get("upload_request_id")
            or ""
        ).strip()
        if not session_id or session_id != asset_session_id:
            continue
        replacements.append((completion, predecessor, successor))
    return replacements


def replace_revision_asset_root(revision, predecessor, successor, asset_by_id):
    predecessor_id = int(predecessor["id"])
    successor_id = int(successor["id"])
    root_id = int(successor.get("asset_id") or 0)
    changed = False

    for key in ("source_task_asset_id", "source_alias_from_task_asset_id"):
        member_id = int(revision.get(key) or 0)
        member = asset_by_id.get(member_id)
        if member_id == predecessor_id or (
            member is not None
            and root_id > 0
            and int(member.get("asset_id") or 0) == root_id
            and member.get("asset_type") == successor.get("asset_type")
        ):
            revision[key] = successor_id
            changed = True

    finals = []
    for member_id in revision.get("final_task_asset_ids") or []:
        member = asset_by_id.get(int(member_id))
        if int(member_id) == predecessor_id or (
            member is not None
            and root_id > 0
            and int(member.get("asset_id") or 0) == root_id
            and member.get("asset_type") == successor.get("asset_type")
        ):
            finals.append(successor_id)
            changed = True
        else:
            finals.append(int(member_id))
    revision["final_task_asset_ids"] = list(dict.fromkeys(finals))
    revision["mode"] = "set" if len(revision["final_task_asset_ids"]) > 1 else "single"
    return changed


def revision_contains_asset_root(revision, asset, asset_by_id):
    target_id = int(asset["id"])
    root_id = int(asset.get("asset_id") or 0)
    asset_type = asset.get("asset_type")
    member_ids = [
        int(revision.get("source_task_asset_id") or 0),
        int(revision.get("source_alias_from_task_asset_id") or 0),
        *[
            int(value)
            for value in revision.get("final_task_asset_ids") or []
        ],
    ]
    for member_id in member_ids:
        member = asset_by_id.get(member_id)
        if member_id == target_id or (
            member is not None
            and root_id > 0
            and int(member.get("asset_id") or 0) == root_id
            and member.get("asset_type") == asset_type
        ):
            return True
    return False


def completion_events_for_asset(
    events: list[dict[str, Any]],
    asset_id: int,
) -> list[dict[str, Any]]:
    return [
        event
        for event in events
        if str(event.get("event_type") or "").lower()
        in UPLOAD_SESSION_COMPLETED_EVENTS
        and asset_id in payload_asset_version_ids(event_payload(event))
    ]


def append_proven_missing_snapshot_member(
    scope,
    predecessor,
    events,
    assets,
    revisions,
) -> bool:
    """Repair an omitted member before applying its proven successor edge."""
    if predecessor.get("asset_type") != "delivery" or not scope_matches(
        scope, predecessor
    ):
        return False
    predecessor_id = int(predecessor["id"])
    completions = completion_events_for_asset(events, predecessor_id)
    if len(completions) != 1:
        return False
    completion = completions[0]
    origin_index = None

    approved_at = str(predecessor.get("approved_at") or "")
    approved_by = int(predecessor.get("approved_by") or 0)
    if approved_at and approved_by > 0:
        for index, revision in enumerate(revisions):
            finalized_at = str(revision.get("finalized_at") or "")
            if (
                finalized_at
                and timestamps_within_seconds(
                    {"created_at": approved_at},
                    {"created_at": finalized_at},
                    2,
                )
                and int(revision.get("created_by") or 0) == approved_by
                and str(completion.get("created_at") or "") <= finalized_at
            ):
                eligible, _ = revision_asset_eligible(
                    predecessor, set(), finalized_at
                )
                if eligible:
                    origin_index = index
                    break

    if origin_index is None and scope.get("scope_kind") == "retouch_requirement":
        if not scope.get("_single_requirement"):
            return False
        try:
            completion_time = parse_utc_timestamp(
                str(completion.get("created_at") or "")
            )
        except (TypeError, ValueError):
            return False
        asset_by_id = {int(asset["id"]): asset for asset in assets}
        for index, revision in enumerate(revisions):
            evidence = set(revision.get("evidence_event_ids") or [])
            submit_events = [
                event
                for event in events
                if event_kind(event) == "submit"
                and stable_event_id(event) in evidence
                and event_precedes_boundary(completion, event)
            ]
            if not submit_events:
                continue
            anchor_ids = [
                int(value)
                for value in revision.get("final_task_asset_ids") or []
            ]
            anchors = [
                asset_by_id[value]
                for value in anchor_ids
                if value in asset_by_id
                and str(
                    asset_by_id[value].get("source_module_key") or ""
                ).lower()
                == "retouch"
            ]
            paired = False
            for anchor in anchors:
                anchor_completions = completion_events_for_asset(
                    events, int(anchor["id"])
                )
                if len(anchor_completions) != 1:
                    continue
                try:
                    anchor_time = parse_utc_timestamp(
                        str(anchor_completions[0].get("created_at") or "")
                    )
                except (TypeError, ValueError):
                    continue
                if (
                    int(anchor_completions[0].get("actor_id") or 0)
                    == int(completion.get("actor_id") or 0)
                    and abs((anchor_time - completion_time).total_seconds())
                    <= 15 * 60
                ):
                    paired = True
                    break
            if paired:
                origin_index = index
                break

    if origin_index is None:
        return False

    asset_by_id = {int(asset["id"]): asset for asset in assets}
    root_id = int(predecessor.get("asset_id") or 0)
    evidence_ids = event_evidence_ids(completion)
    for revision in revisions[origin_index:]:
        finals = [int(value) for value in revision.get("final_task_asset_ids") or []]
        has_root = any(
            int((asset_by_id.get(value) or {}).get("asset_id") or 0)
            == root_id
            for value in finals
        )
        if not has_root:
            finals.append(predecessor_id)
            revision["final_task_asset_ids"] = finals
            revision["mode"] = "set" if len(finals) > 1 else "single"
        revision["evidence_event_ids"] = list(
            dict.fromkeys(
                list(revision.get("evidence_event_ids") or []) + evidence_ids
            )
        )
        recompute_revision_hash(revision)
    return True


def replay_post_close_replacements(
    scope, events, assets, references, revisions, working, finalized
):
    asset_by_id = {int(asset["id"]): asset for asset in assets}
    replacements = direct_post_close_replacements(scope, events, assets)
    for replacement_index, (completion, predecessor, successor) in enumerate(
        replacements
    ):
        later_predecessor_ids = {
            int(later_predecessor["id"])
            for _, later_predecessor, _ in replacements[
                replacement_index + 1 :
            ]
        }
        try:
            completion_time = parse_utc_timestamp(completion["created_at"])
            latest_revision_time = max(
                parse_utc_timestamp(revision["created_at"])
                for revision in revisions
            )
        except (KeyError, TypeError, ValueError):
            completion_time = None
            latest_revision_time = None

        # A replacement may be followed by later audit supplements. Insert it
        # at its real business boundary, then carry the replacement forward
        # through the later immutable snapshots. Appending it after those
        # supplements would both reverse time and make the replacement snapshot
        # contain files that did not exist yet.
        if (
            completion_time is not None
            and latest_revision_time is not None
            and completion_time < latest_revision_time
        ):
            anchor_index = None
            for index in range(len(revisions) - 1, -1, -1):
                try:
                    revision_time = parse_utc_timestamp(
                        revisions[index]["created_at"]
                    )
                except (KeyError, TypeError, ValueError):
                    continue
                if (
                    revision_time <= completion_time
                    and revision_contains_asset_root(
                        revisions[index], predecessor, asset_by_id
                    )
                ):
                    anchor_index = index
                    break
            if anchor_index is not None:
                current_working = (
                    revisions[working - 1] if working else None
                )
                current_finalized = (
                    revisions[finalized - 1] if finalized else None
                )
                inherited = revisions[anchor_index]
                revision = make_revision(
                    scope,
                    completion,
                    anchor_index + 2,
                    "finalized",
                    "reopen",
                    assets,
                    references,
                    inherited=inherited,
                    extra_evidence=list(
                        inherited.get("evidence_event_ids") or []
                    ),
                )
                revision["reason"] = (
                    f"policy {POST_CLOSE_REPLACEMENT_POLICY}: post-close "
                    "same-root replacement proven by an immutable "
                    "asset-version edge and exact completed upload session; "
                    "human confirmation remains required"
                )
                if not replace_revision_asset_root(
                    revision, predecessor, successor, asset_by_id
                ):
                    add_blocker(
                        revision,
                        f"post-close successor {successor['id']} asset root "
                        "is absent from the inherited snapshot",
                    )
                else:
                    prune_inherited_reopen_snapshot(
                        revision,
                        assets,
                        completion,
                        str(scope.get("scope_kind") or ""),
                        later_predecessor_ids,
                    )
                    revision["status"] = "superseded"
                    evidence_ids = event_evidence_ids(completion)
                    for later in revisions[anchor_index + 1 :]:
                        if replace_revision_asset_root(
                            later, predecessor, successor, asset_by_id
                        ) or revision_contains_asset_root(
                            later, successor, asset_by_id
                        ):
                            later["evidence_event_ids"] = list(
                                dict.fromkeys(
                                    list(
                                        later.get("evidence_event_ids") or []
                                    )
                                    + evidence_ids
                                )
                            )
                            later.setdefault(
                                "_review_policy_ids", []
                            ).append(POST_CLOSE_REPLACEMENT_POLICY)
                            recompute_revision_hash(later)
                    revisions.insert(anchor_index + 1, revision)
                    for index, item in enumerate(revisions, start=1):
                        item["revision_no"] = index
                        recompute_revision_hash(item)
                    working = (
                        revisions.index(current_working) + 1
                        if current_working is not None
                        else 0
                    )
                    finalized = (
                        revisions.index(current_finalized) + 1
                        if current_finalized is not None
                        else 0
                    )
                    continue

        inherited = revisions[finalized - 1] if finalized else None
        revision = make_revision(
            scope,
            completion,
            len(revisions) + 1,
            "finalized",
            "reopen",
            assets,
            references,
            inherited=inherited,
            extra_evidence=(
                list(inherited.get("evidence_event_ids") or [])
                if inherited is not None
                else None
            ),
        )
        revision["reason"] = (
            f"policy {POST_CLOSE_REPLACEMENT_POLICY}: post-close same-root "
            "replacement proven by an "
            "immutable asset-version edge and exact completed upload session; "
            "human confirmation remains required"
        )
        blocker = ""
        if inherited is None:
            blocker = "post-close replacement has no finalized revision to inherit"
        elif not replace_revision_asset_root(
            revision, predecessor, successor, asset_by_id
        ):
            if append_proven_missing_snapshot_member(
                scope,
                predecessor,
                events,
                assets,
                revisions,
            ):
                inherited = revisions[finalized - 1]
                revision = make_revision(
                    scope,
                    completion,
                    len(revisions) + 1,
                    "finalized",
                    "reopen",
                    assets,
                    references,
                    inherited=inherited,
                    extra_evidence=list(
                        inherited.get("evidence_event_ids") or []
                    ),
                )
                revision["reason"] = (
                    f"policy {POST_CLOSE_REPLACEMENT_POLICY}: post-close "
                    "same-root replacement proven by an immutable "
                    "asset-version edge and exact completed upload session; "
                    "human confirmation remains required"
                )
                if not replace_revision_asset_root(
                    revision, predecessor, successor, asset_by_id
                ):
                    blocker = (
                        f"post-close successor {successor['id']} asset root "
                        "is absent from the inherited snapshot"
                    )
            else:
                blocker = (
                    f"post-close successor {successor['id']} asset root is "
                    "absent from the inherited snapshot"
                )
        if not blocker:
            prune_inherited_reopen_snapshot(
                revision,
                assets,
                completion,
                str(scope.get("scope_kind") or ""),
                later_predecessor_ids,
            )
            revisions[finalized - 1]["status"] = "superseded"
            recompute_revision_hash(revisions[finalized - 1])
        if blocker:
            target = inherited
            if target is not None:
                add_blocker(target, blocker)
                target["evidence_event_ids"] = list(
                    target.get("evidence_event_ids") or []
                ) + event_evidence_ids(completion)
                recompute_revision_hash(target)
            continue
        recompute_revision_hash(revision)
        revisions.append(revision)
        working = revision["revision_no"]
        finalized = revision["revision_no"]
    return working, finalized


def effective_v8_status(status: str) -> str:
    if status in {"PendingAuditA", "PendingAuditB", "PendingCustomizationReview", "PendingOutsourceReview", "PendingEffectReview"}:
        return "PendingAudit"
    if status in {"RejectedByAuditA", "RejectedByAuditB", "PendingCustomizationProduction", "PendingEffectRevision", "PendingOutsource", "Outsourcing"}:
        return "InProgress"
    if status in {"PendingWarehouseQC", "PendingWarehouseReceive", "PendingProductionTransfer", "PendingClose"}:
        return "Completed"
    return status


def event_asset_ids(event: dict[str, Any]) -> list[int]:
    return payload_asset_version_ids(event_payload(event))


def module_event_applies(event: dict[str, Any], scope: dict[str, Any]) -> bool:
    if event.get("namespace") != "task_module_event":
        return True
    module_key = str(event.get("module_key") or "").strip().lower()
    kind = event_kind(event)
    if not module_key or not kind:
        return False
    if kind == "submit":
        expected = {"retouch"} if scope["scope_kind"] == "retouch_requirement" else {"design", "customization"}
    elif kind in {"approve", "reject", "supplement"}:
        expected = {"audit", "customization"}
    elif kind == "reopen":
        expected = {"warehouse", "audit", "design", "customization", "retouch"}
    else:
        expected = {"audit", "warehouse"}
    return module_key in expected


def event_applies_to_scope(event, scope, events, assets, sku_scope_count):
    if not module_event_applies(event, scope):
        return False
    payload = event_payload(event)
    target_sku = str(payload.get("target_sku_code") or "").strip()
    requirement_id = payload.get("retouch_requirement_id")
    asset_ids = event_asset_ids(event)
    identified_assets = [a for a in assets if int(a["id"]) in asset_ids]
    if target_sku:
        return scope["scope_kind"] == "sku" and target_sku == str(scope.get("sku_code") or "")
    if requirement_id not in (None, ""):
        try:
            return scope["scope_kind"] == "retouch_requirement" and int(requirement_id) == int(scope["scope_ref_id"])
        except (TypeError, ValueError):
            return False
    if identified_assets:
        return any(scope_matches(scope, asset) for asset in identified_assets)
    if event_kind(event) == "submit" and payload.get("upload_session_id"):
        session_id = str(payload.get("upload_session_id") or "").strip()
        version_ids = []
        for candidate in events:
            candidate_payload = event_payload(candidate)
            if (
                str(candidate.get("event_type") or "").lower() in UPLOAD_SESSION_COMPLETED_EVENTS
                and str(candidate_payload.get("upload_session_id") or "").strip() == session_id
                and candidate.get("created_at", "") <= event["created_at"]
            ):
                version_ids.extend(payload_asset_version_ids(candidate_payload))
        linked = [asset for asset in assets if int(asset["id"]) in version_ids]
        if linked:
            return any(scope_matches(scope, asset) for asset in linked)
        return scope["scope_kind"] != "sku" or sku_scope_count == 1
    if event_kind(event) == "submit" and scope["scope_kind"] == "sku" and sku_scope_count > 1:
        # A task-wide submit event cannot be copied into every SKU history. It
        # needs target_sku_code or a task asset whose scope proves the SKU.
        return False
    return True


def event_dedup_key(event: dict[str, Any]) -> tuple[Any, ...]:
    payload = event_payload(event)
    if str(event.get("event_type") or "").lower() in WAREHOUSE_REOPEN_EVENTS:
        receipt_no = str(payload.get("receipt_no") or "").strip()
        if receipt_no:
            return ("warehouse_rejected", receipt_no)
    return (
        event_kind(event),
        event.get("created_at"),
        int(event.get("actor_id") or 0),
        str(payload.get("target_sku_code") or ""),
        tuple(event_asset_ids(event)),
        str(payload.get("upload_session_id") or payload.get("asset_upload_session_id") or ""),
        str(payload.get("customization_review_decision") or payload.get("action") or payload.get("decision") or "").lower(),
    )


def is_nonfinal_customization_gate(event: dict[str, Any]) -> bool:
    payload = event_payload(event)
    return bool(
        str(event.get("event_type") or "").lower()
        == "task.customization.reviewed"
        and event_kind(event) == "approve"
        and str(
            payload.get("to_task_status")
            or payload.get("next_status")
            or event.get("to_state")
            or ""
        ).lower()
        in {
            "pendingcustomizationproduction",
            "pending_customization_production",
        }
    )


def is_customization_rejection_reopen_side_effect(
    event: dict[str, Any],
) -> bool:
    payload = event_payload(event)
    return bool(
        event.get("namespace") == "task_module_event"
        and event_kind(event) == "reopen"
        and str(payload.get("source") or "").lower()
        == "customization_review"
        and str(
            payload.get("customization_review_decision") or ""
        ).lower()
        == "return_to_designer"
    )


def rejection_semantic_key(event: dict[str, Any]) -> tuple[Any, ...] | None:
    if event_kind(event) != "reject":
        return None
    payload = event_payload(event)
    event_type = str(event.get("event_type") or "").lower()
    action = str(
        payload.get("customization_review_decision")
        or payload.get("action")
        or payload.get("decision")
        or ""
    ).lower()
    source = str(payload.get("source") or "").lower()
    if action == "return_to_designer" or (
        event_type == "rejected" and source == "customization_review"
    ):
        return (
            "customization_rejected",
            int(event.get("actor_id") or 0),
            str(payload.get("target_sku_code") or ""),
        )
    return (
        "rejected",
        int(event.get("actor_id") or 0),
        str(payload.get("stage") or ""),
        str(payload.get("comment") or ""),
        str(
            payload.get("from_task_status")
            or event.get("from_state")
            or ""
        ),
        str(
            payload.get("to_task_status")
            or event.get("to_state")
            or ""
        ),
    )


def timestamps_within_seconds(
    left: dict[str, Any],
    right: dict[str, Any],
    seconds: int,
) -> bool:
    try:
        return abs(
            (
                parse_utc_timestamp(str(left.get("created_at") or ""))
                - parse_utc_timestamp(str(right.get("created_at") or ""))
            ).total_seconds()
        ) <= seconds
    except (TypeError, ValueError):
        return False


def resolve_atomic_multi_sku_batch_submits(scopes, events, assets):
    """Rebuild each proven task-wide multi-SKU submission wave.

    A wave may contain several source/final files per SKU. Later resubmissions
    replace only roles with new completed uploads and inherit untouched SKU
    roles from the preceding immutable snapshot.
    """
    sku_scopes = [scope for scope in scopes if scope.get("scope_kind") == "sku"]
    if len(sku_scopes) <= 1 or len(sku_scopes) != len(scopes):
        return []
    sku_codes = [str(scope.get("sku_code") or "").strip() for scope in sku_scopes]
    if any(not code for code in sku_codes) or len(set(sku_codes)) != len(sku_codes):
        return []

    submits = sorted(
        (
            dict(event)
            for event in events
            if event_kind(event) == "submit"
            and event.get("namespace") == "task_event_log"
            and int(event.get("actor_id") or 0)
        ),
        key=lambda event: (
            str(event.get("created_at") or ""),
            int(event.get("sequence") or 0),
            str(event.get("id") or ""),
        ),
    )
    if not submits:
        return []

    completions = [
        dict(event)
        for event in events
        if str(event.get("event_type") or "").lower()
        in UPLOAD_SESSION_COMPLETED_EVENTS
        and event.get("namespace") == "task_event_log"
    ]
    asset_by_id = {int(asset["id"]): asset for asset in assets}
    state: dict[str, dict[str, list[dict[str, Any]]]] = {
        code: {"source": [], "delivery": []} for code in sku_codes
    }
    candidates = []
    previous_submit = None
    for submit in submits:
        payload = event_payload(submit)
        submit_session = str(payload.get("upload_session_id") or "").strip()
        submit_sku = str(payload.get("target_sku_code") or "").strip()
        submit_root_ids = payload_ids(payload, "asset_id")
        if (
            not submit_session
            or submit_sku not in state
            or str(payload.get("asset_type") or "").strip().lower()
            != "delivery"
            or len(submit_root_ids) != 1
        ):
            previous_submit = submit
            continue

        updates: dict[str, dict[str, list[dict[str, Any]]]] = defaultdict(
            lambda: {"source": [], "delivery": []}
        )
        seen_version_ids, seen_sessions = set(), set()
        invalid = False
        for completion in sorted(
            completions,
            key=lambda event: (
                str(event.get("created_at") or ""),
                int(event.get("sequence") or 0),
                str(event.get("id") or ""),
            ),
        ):
            if not event_precedes_boundary(completion, submit):
                continue
            if previous_submit and event_precedes_boundary(
                completion, previous_submit
            ):
                continue
            completion_payload = event_payload(completion)
            role = str(
                completion_payload.get("asset_type") or ""
            ).strip().lower()
            sku_code = str(
                completion_payload.get("target_sku_code") or ""
            ).strip()
            if role not in {"source", "delivery"} or sku_code not in state:
                continue
            version_ids = payload_asset_version_ids(completion_payload)
            root_ids = payload_ids(completion_payload, "asset_id")
            session_id = str(
                completion_payload.get("upload_session_id") or ""
            ).strip()
            if (
                len(version_ids) != 1
                or len(root_ids) != 1
                or not session_id
                or int(completion.get("actor_id") or 0)
                != int(submit.get("actor_id") or 0)
            ):
                invalid = True
                break
            version_id = version_ids[0]
            asset = asset_by_id.get(version_id)
            if (
                asset is None
                or version_id in seen_version_ids
                or session_id in seen_sessions
                or int(asset.get("task_id") or 0)
                != int(sku_scopes[0]["task_id"])
                or str(asset.get("scope_sku_code") or "").strip()
                != sku_code
                or asset.get("retouch_requirement_id")
                or str(asset.get("asset_type") or "").lower() != role
                or int(asset.get("asset_id") or 0) != root_ids[0]
                or str(
                    asset.get("upload_session_id")
                    or asset.get("upload_request_id")
                    or ""
                ).strip()
                != session_id
            ):
                invalid = True
                break
            membership_time = boundary_membership_time(submit, completion)
            module = str(
                asset.get("source_module_key")
                or completion.get("module_key")
                or ""
            ).strip().lower()
            if module not in {"design", "customization"}:
                invalid = True
                break
            eligible, _ = revision_asset_eligible(
                asset, {module}, membership_time
            )
            if not eligible or not at_or_before(asset, membership_time):
                invalid = True
                break
            seen_version_ids.add(version_id)
            seen_sessions.add(session_id)
            updates[sku_code][role].append(
                {
                    "asset_version_id": version_id,
                    "completion_event_id": stable_event_id(completion),
                    "upload_session_id": session_id,
                    "asset_root_id": root_ids[0],
                    "created_at": completion["created_at"],
                }
            )
        if invalid:
            previous_submit = submit
            continue

        for sku_code, role_updates in updates.items():
            for role in ("source", "delivery"):
                if role_updates[role]:
                    state[sku_code][role] = role_updates[role]

        trigger_members = (
            updates.get(submit_sku, {}).get("delivery", [])
            if submit_sku in updates
            else []
        )
        if not any(
            member["upload_session_id"] == submit_session
            and member["asset_root_id"] == submit_root_ids[0]
            for member in trigger_members
        ):
            previous_submit = submit
            continue
        if any(not state[code]["delivery"] for code in sku_codes):
            previous_submit = submit
            continue

        candidate = dict(submit)
        candidate["_batch_scope_memberships"] = {}
        for sku_code in sorted(sku_codes):
            sources = state[sku_code]["source"]
            finals = state[sku_code]["delivery"]
            members = sources + finals
            candidate["_batch_scope_memberships"][sku_code] = {
                "asset_version_ids": [
                    member["asset_version_id"] for member in members
                ],
                "completion_event_ids": [
                    member["completion_event_id"] for member in members
                ],
                **(
                    {
                        "source_alias_asset_version_id": finals[0][
                            "asset_version_id"
                        ]
                    }
                    if not sources
                    else {}
                ),
            }
        candidate["_migration_policy"] = BATCH_SUBMIT_POLICY
        candidates.append(candidate)
        previous_submit = submit
    return candidates


def resolve_atomic_multi_sku_batch_submit(scopes, events, assets):
    candidates = resolve_atomic_multi_sku_batch_submits(
        scopes, events, assets
    )
    return candidates[0] if len(candidates) == 1 else None


def _resolve_exact_retouch_terminal_submit(
    scopes,
    events,
    assets,
    expected_finals,
    policy,
):
    """Return a sole legacy retouch submit only when frozen facts still match."""
    if not scopes or any(
        scope.get("scope_kind") != "retouch_requirement"
        or str(scope.get("task_status") or "") != "Completed"
        for scope in scopes
    ):
        return None
    task_id = int(scopes[0]["task_id"])
    if any(int(scope["task_id"]) != task_id for scope in scopes):
        return None
    scope_ids = {int(scope["scope_ref_id"]) for scope in scopes}
    if scope_ids != set(expected_finals) or len(scope_ids) != len(scopes):
        return None

    task_deliveries = {
        int(asset["id"])
        for asset in assets
        if int(asset.get("task_id") or 0) == task_id
        and str(asset.get("asset_type") or "").lower() == "delivery"
    }
    expected_delivery_ids = {
        int(asset_id)
        for asset_ids in expected_finals.values()
        for asset_id in asset_ids
    }
    # Premature tasks can have one intentionally unassigned delivery; atomic
    # completed tasks must account for every delivery exactly.
    if task_deliveries != expected_delivery_ids:
        return None

    boundaries, seen = [], set()
    for event in sorted(
        (dict(event) for event in events if event_kind(event)),
        key=lambda event: (
            str(event.get("created_at") or ""),
            event.get("namespace") == "task_module_event",
            str(event.get("id") or ""),
        ),
    ):
        key = event_dedup_key(event)
        if key in seen:
            continue
        seen.add(key)
        boundaries.append(event)
    submits = [event for event in boundaries if event_kind(event) == "submit"]
    if len(submits) != 1:
        return None
    submit = submits[0]
    if (
        submit.get("namespace") != "task_event_log"
        or not int(submit.get("actor_id") or 0)
        or any(
            event_kind(event) in {"submit", "reject", "reopen"}
            and str(event.get("created_at") or "")
            > str(submit.get("created_at") or "")
            for event in boundaries
        )
    ):
        return None

    asset_by_id = {int(asset["id"]): asset for asset in assets}
    completions_by_asset: dict[int, list[dict[str, Any]]] = defaultdict(list)
    for event in events:
        if (
            event.get("namespace") != "task_event_log"
            or str(event.get("event_type") or "").lower()
            not in UPLOAD_SESSION_COMPLETED_EVENTS
            or str(event.get("created_at") or "")
            > str(submit.get("created_at") or "")
        ):
            continue
        for asset_id in payload_asset_version_ids(event_payload(event)):
            if asset_id in expected_delivery_ids:
                completions_by_asset[asset_id].append(event)

    memberships: dict[str, dict[str, Any]] = {}
    for scope_id, ordered_ids in expected_finals.items():
        evidence_ids = []
        for asset_id in ordered_ids:
            asset = asset_by_id.get(int(asset_id))
            completions = completions_by_asset.get(int(asset_id), [])
            if asset is None or len(completions) != 1:
                return None
            completion = completions[0]
            payload = event_payload(completion)
            session_id = str(payload.get("upload_session_id") or "").strip()
            asset_session = str(
                asset.get("upload_session_id")
                or asset.get("upload_request_id")
                or ""
            ).strip()
            eligible, _ = revision_asset_eligible(
                asset, {"retouch"}, str(submit["created_at"])
            )
            if (
                int(asset.get("task_id") or 0) != task_id
                or asset.get("retouch_requirement_id") not in (None, "", 0)
                or str(asset.get("asset_type") or "").lower() != "delivery"
                or not eligible
                or not at_or_before(asset, str(submit["created_at"]))
                or not session_id
                or session_id != asset_session
                or int(completion.get("actor_id") or 0)
                != int(submit.get("actor_id") or 0)
            ):
                return None
            evidence_ids.extend(event_evidence_ids(completion))
        memberships[str(scope_id)] = {
            "asset_version_ids": [int(value) for value in ordered_ids],
            "completion_event_ids": evidence_ids,
        }

    submit_session = str(
        event_payload(submit).get("upload_session_id") or ""
    ).strip()
    terminal_asset_id = max(
        expected_delivery_ids,
        key=lambda value: (
            str(asset_by_id[value].get("created_at") or ""),
            value,
        ),
        default=0,
    )
    terminal_asset = asset_by_id.get(terminal_asset_id)
    if (
        not submit_session
        or terminal_asset is None
        or submit_session
        != str(
            terminal_asset.get("upload_session_id")
            or terminal_asset.get("upload_request_id")
            or ""
        ).strip()
    ):
        return None

    candidate = dict(submit)
    candidate["_retouch_scope_memberships"] = memberships
    candidate["_migration_policy"] = policy
    return candidate


def resolve_legacy_retouch_unscoped_atomic_batch(scopes, events, assets):
    if not scopes:
        return None
    expected = LEGACY_RETOUCH_UNSCOPED_ATOMIC_FINALS.get(
        int(scopes[0]["task_id"])
    )
    if expected is None:
        return None
    return _resolve_exact_retouch_terminal_submit(
        scopes,
        events,
        assets,
        expected,
        RETOUCH_UNSCOPED_ATOMIC_BATCH_POLICY,
    )


def resolve_legacy_retouch_premature_partial(scopes, events, assets):
    if not scopes:
        return None
    task_id = int(scopes[0]["task_id"])
    expected = LEGACY_RETOUCH_PREMATURE_PARTIAL_FINALS.get(task_id)
    if expected is None:
        return None
    # The frozen task still has exactly one unscoped delivery even when that
    # delivery is deliberately not assigned to any requirement.
    actual_delivery_ids = sorted(
        int(asset["id"])
        for asset in assets
        if str(asset.get("asset_type") or "").lower() == "delivery"
    )
    if len(actual_delivery_ids) != 1:
        return None
    verification_finals = {
        scope_id: list(asset_ids)
        for scope_id, asset_ids in expected.items()
    }
    if not any(verification_finals.values()):
        verification_finals[min(verification_finals)] = actual_delivery_ids
    else:
        assigned = {
            asset_id
            for asset_ids in verification_finals.values()
            for asset_id in asset_ids
        }
        if assigned != set(actual_delivery_ids):
            return None
    candidate = _resolve_exact_retouch_terminal_submit(
        scopes,
        events,
        assets,
        verification_finals,
        RETOUCH_PREMATURE_TERMINAL_PARTIAL_POLICY,
    )
    if candidate is None:
        return None
    verification_evidence = [
        evidence_id
        for membership in candidate["_retouch_scope_memberships"].values()
        for evidence_id in membership.get("completion_event_ids", [])
    ]
    candidate["_retouch_scope_memberships"] = {
        str(scope_id): {
            "asset_version_ids": list(asset_ids),
            "completion_event_ids": (
                candidate["_retouch_scope_memberships"]
                .get(str(scope_id), {})
                .get("completion_event_ids", [])
            ),
        }
        for scope_id, asset_ids in expected.items()
    }
    candidate["_unassigned_delivery_ids"] = sorted(
        set(actual_delivery_ids)
        - {
            asset_id
            for asset_ids in expected.values()
            for asset_id in asset_ids
        }
    )
    candidate["_unassigned_completion_event_ids"] = verification_evidence
    return candidate


def resolve_legacy_retouch_unscoped_ambiguous_terminal(
    scopes, events, assets
):
    """Conservatively reopen an unscoped multi-requirement terminal batch.

    The upload and submit boundary prove that files were delivered, but no
    immutable fact assigns those files to individual requirements. V8 must not
    invent that membership. Preserve the legacy assets outside revision
    membership and move the task back to InProgress with empty requirement
    shells.
    """
    if (
        len(scopes) < 2
        or any(
            scope.get("scope_kind") != "retouch_requirement"
            or str(scope.get("task_status") or "") != "Completed"
            for scope in scopes
        )
    ):
        return None
    relevant = [
        asset
        for asset in assets
        if active_asset(asset)
        and str(asset.get("asset_type") or "") in {"source", "delivery"}
    ]
    if not relevant:
        return None
    scope_ids = {int(scope["scope_ref_id"]) for scope in scopes}
    if any(
        (
            asset.get("asset_type") == "delivery"
            and asset.get("retouch_requirement_id") not in (None, "", 0)
        )
        or (
            asset.get("asset_type") == "source"
            and int(asset.get("retouch_requirement_id") or 0)
            not in scope_ids
        )
        for asset in relevant
    ):
        return None
    submits = sorted(
        (
            event
            for event in events
            if event.get("namespace") == "task_event_log"
            and event_kind(event) == "submit"
        ),
        key=lambda event: (
            str(event.get("created_at") or ""),
            int(event.get("sequence") or 0),
            str(event.get("id") or ""),
        ),
    )
    if len(submits) != 1 or not int(submits[0].get("actor_id") or 0):
        return None
    submit = submits[0]
    if any(
        event_kind(event) in {"submit", "reject", "reopen"}
        and (
            str(event.get("created_at") or ""),
            int(event.get("sequence") or 0),
            str(event.get("id") or ""),
        )
        > (
            str(submit.get("created_at") or ""),
            int(submit.get("sequence") or 0),
            str(submit.get("id") or ""),
        )
        for event in events
    ):
        return None

    completion_by_asset = {}
    for asset in relevant:
        if not at_or_before(asset, submit["created_at"]):
            return None
        asset_session = str(
            asset.get("upload_session_id")
            or asset.get("upload_request_id")
            or ""
        ).strip()
        completions = [
            event
            for event in events
            if (
                event.get("namespace") == "task_event_log"
                and str(event.get("event_type") or "").lower()
                in UPLOAD_SESSION_COMPLETED_EVENTS
                and event_precedes_boundary(event, submit)
                and str(
                    event_payload(event).get("upload_session_id") or ""
                ).strip()
                == asset_session
                and int(asset["id"])
                in payload_asset_version_ids(event_payload(event))
            )
        ]
        if not asset_session or len(completions) != 1:
            return None
        completion_by_asset[int(asset["id"])] = event_evidence_ids(
            completions[0]
        )
    candidate = dict(submit)
    candidate["_migration_policy"] = (
        RETOUCH_PREMATURE_TERMINAL_PARTIAL_POLICY
    )
    candidate["_retouch_scope_memberships"] = {
        str(scope_id): {
            "asset_version_ids": [
                int(asset["id"])
                for asset in relevant
                if asset.get("asset_type") == "source"
                and int(asset.get("retouch_requirement_id") or 0)
                == scope_id
            ],
            "completion_event_ids": list(dict.fromkeys(
                evidence_id
                for asset in relevant
                if asset.get("asset_type") == "source"
                and int(asset.get("retouch_requirement_id") or 0)
                == scope_id
                for evidence_id in completion_by_asset[int(asset["id"])]
            )),
        }
        for scope_id in sorted(scope_ids)
    }
    candidate["_unassigned_delivery_ids"] = [
        int(asset["id"])
        for asset in relevant
        if asset.get("asset_type") == "delivery"
    ]
    candidate["_unassigned_completion_event_ids"] = list(dict.fromkeys(
        evidence_id
        for asset in relevant
        if asset.get("asset_type") == "delivery"
        for evidence_id in completion_by_asset[int(asset["id"])]
    ))
    return candidate


def resolve_legacy_retouch_visual_scope_task2533(
    scopes, events, assets, references
):
    """Resolve the one visually reviewed multi-requirement retouch task."""
    def order_key(event):
        return (
            str(event.get("created_at") or ""),
            event.get("namespace") == "task_module_event",
            int(event.get("sequence") or 0),
            str(event.get("id") or ""),
        )

    if (
        len(scopes) != len(LEGACY_RETOUCH_VISUAL_SCOPE_TASK2533)
        or {int(scope.get("task_id") or 0) for scope in scopes} != {2533}
        or {
            int(scope.get("scope_ref_id") or 0) for scope in scopes
        }
        != set(LEGACY_RETOUCH_VISUAL_SCOPE_TASK2533)
        or any(
            scope.get("scope_kind") != "retouch_requirement"
            or str(scope.get("task_status") or "") != "Completed"
            for scope in scopes
        )
    ):
        return None

    expected_sources = {
        int(contract["source"])
        for contract in LEGACY_RETOUCH_VISUAL_SCOPE_TASK2533.values()
    }
    expected_finals = {
        int(contract["final"])
        for contract in LEGACY_RETOUCH_VISUAL_SCOPE_TASK2533.values()
    }
    expected_unassigned = set(LEGACY_RETOUCH_VISUAL_UNASSIGNED_TASK2533)
    relevant_assets = {
        int(asset["id"]): asset
        for asset in assets
        if active_asset(asset)
        and str(asset.get("asset_type") or "") in {"source", "delivery"}
    }
    if set(relevant_assets) != (
        expected_sources | expected_finals | expected_unassigned
    ) or any(
        relevant_assets[asset_id].get("retouch_requirement_id")
        not in (None, "", 0)
        for asset_id in expected_finals | expected_unassigned
    ):
        return None

    expected_references = {
        int(reference_id)
        for contract in LEGACY_RETOUCH_VISUAL_SCOPE_TASK2533.values()
        for reference_id in contract["references"]
    }
    actual_references = {
        int(reference["id"])
        for reference in references
        if int(reference.get("task_id") or 0) == 2533
    }
    if actual_references != expected_references:
        return None

    completions_by_asset: dict[int, list[dict[str, Any]]] = defaultdict(list)
    for event in events:
        if (
            event.get("namespace") == "task_event_log"
            and str(event.get("event_type") or "").lower()
            in UPLOAD_SESSION_COMPLETED_EVENTS
        ):
            for asset_id in payload_asset_version_ids(event_payload(event)):
                if asset_id in relevant_assets:
                    completions_by_asset[asset_id].append(event)
    if any(
        len(completions_by_asset.get(asset_id, [])) != 1
        for asset_id in relevant_assets
    ):
        return None

    submits = [
        event
        for event in events
        if event.get("namespace") == "task_event_log"
        and event_kind(event) == "submit"
    ]
    if len(submits) != 1:
        return None
    submit = submits[0]
    if (
        int(submit.get("actor_id") or 0) != 228
        or str(event_payload(submit).get("upload_session_id") or "").strip()
        != str(
            relevant_assets[LEGACY_RETOUCH_VISUAL_UNASSIGNED_TASK2533[0]].get(
                "upload_session_id"
            )
            or relevant_assets[
                LEGACY_RETOUCH_VISUAL_UNASSIGNED_TASK2533[0]
            ].get("upload_request_id")
            or ""
        ).strip()
        or any(
            event_kind(event) in {"submit", "reject", "reopen"}
            and order_key(event) > order_key(submit)
            for event in events
        )
    ):
        return None

    scope_by_id = {
        int(scope["scope_ref_id"]): scope for scope in scopes
    }
    reference_by_id = {
        int(reference["id"]): reference for reference in references
    }
    by_scope = {}
    for scope_id, contract in LEGACY_RETOUCH_VISUAL_SCOPE_TASK2533.items():
        scope = scope_by_id[scope_id]
        source = relevant_assets[int(contract["source"])]
        final = relevant_assets[int(contract["final"])]
        if (
            str(source.get("asset_type") or "") != "source"
            or int(source.get("retouch_requirement_id") or 0) != scope_id
            or str(source.get("source_module_key") or "") != "retouch"
            or str(final.get("asset_type") or "") != "delivery"
            or final.get("retouch_requirement_id") not in (None, "", 0)
            or str(final.get("source_module_key") or "") != "retouch"
            or not at_or_before(source, submit["created_at"])
            or not at_or_before(final, submit["created_at"])
            or any(
                int(reference_by_id[reference_id].get(
                    "retouch_requirement_id"
                ) or 0)
                != scope_id
                for reference_id in contract["references"]
            )
        ):
            return None
        for asset in (source, final):
            completion = completions_by_asset[int(asset["id"])][0]
            asset_session = str(
                asset.get("upload_session_id")
                or asset.get("upload_request_id")
                or ""
            ).strip()
            if (
                not asset_session
                or asset_session
                != str(
                    event_payload(completion).get("upload_session_id") or ""
                ).strip()
                or not at_or_before(completion, submit["created_at"])
            ):
                return None
        by_scope[scope_id] = {
            "source": source,
            "final": final,
            "references": list(contract["references"]),
            "completion_events": [
                completions_by_asset[int(source["id"])][0],
                completions_by_asset[int(final["id"])][0],
            ],
        }
    return {"submit": submit, "by_scope": by_scope}


def scoped_boundary_events(scope, events, assets, sku_scope_count):
    applicable = [
        dict(event) for event in events
        if event_kind(event)
        and not is_nonfinal_customization_gate(event)
        and not is_customization_rejection_reopen_side_effect(event)
        and event_applies_to_scope(
            event, scope, events, assets, sku_scope_count
        )
    ]
    # Prefer task_event_logs when legacy dual-write produced an equivalent
    # task_module_event. Evidence remains stable without replaying the same
    # business boundary twice.
    applicable.sort(key=lambda event: (event["created_at"], event.get("namespace") == "task_module_event", str(event["id"])))
    deduped, seen = [], {}
    for event in applicable:
        rejection_key = rejection_semantic_key(event)
        if rejection_key and deduped:
            previous = deduped[-1]
            previous_key = rejection_semantic_key(previous)
            is_same_transition_without_resubmit = (
                rejection_key == previous_key
                and (
                    event.get("namespace") == previous.get("namespace")
                    or timestamps_within_seconds(event, previous, 2)
                )
            )
            if is_same_transition_without_resubmit:
                duplicate_ids = event_evidence_ids(previous)
                if (
                    previous.get("namespace") == "task_module_event"
                    and event.get("namespace") == "task_event_log"
                ):
                    event.setdefault("_duplicate_evidence_ids", []).extend(
                        duplicate_ids
                    )
                    deduped[-1] = event
                else:
                    previous.setdefault(
                        "_duplicate_evidence_ids", []
                    ).extend(event_evidence_ids(event))
                continue
        key = event_dedup_key(event)
        if key in seen:
            if key and key[0] == "warehouse_rejected":
                primary = deduped[seen[key]]
                primary.setdefault("_duplicate_evidence_ids", []).append(stable_event_id(event))
            continue
        seen[key] = len(deduped)
        deduped.append(event)
    return deduped


def review_row(scope, revision, confidence, reason):
    return {
        "task_id": scope["task_id"], "scope_kind": scope["scope_kind"], "scope_ref_id": scope["scope_ref_id"],
        "revision_no": revision["revision_no"] if revision else "", "confidence": confidence, "reason": reason,
        "evidence_event_ids": "|".join(revision["evidence_event_ids"]) if revision else "",
        "candidate_source_ids": str(revision.get("source_task_asset_id", "")) if revision else "",
        "candidate_final_ids": "|".join(str(v) for v in revision["final_task_asset_ids"]) if revision else "",
        "reviewer_id": "", "reviewed_at": "", "decision": "", "review_note": "",
    }


def resolve_customization_terminal_without_assets(
    task_scopes, events, assets
):
    if not task_scopes:
        return None
    task_id = int(task_scopes[0]["task_id"])
    contract = LEGACY_CUSTOMIZATION_TERMINAL_WITHOUT_ASSETS.get(task_id)
    if contract is None:
        return None
    actual_scopes = {
        (str(scope["scope_kind"]), int(scope["scope_ref_id"]))
        for scope in task_scopes
    }
    if (
        actual_scopes != set(contract)
        or any(
            str(scope.get("task_status") or "") != "PendingWarehouseReceive"
            for scope in task_scopes
        )
    ):
        return None
    approvals = []
    for event in events:
        payload = event_payload(event)
        if (
            str(event.get("event_type") or "").lower()
            == "task.customization.reviewed"
            and str(payload.get("customization_review_decision") or "").lower()
            == "approved"
            and str(payload.get("from_task_status") or "")
            == "PendingCustomizationReview"
            and str(payload.get("to_task_status") or "")
            == "PendingWarehouseReceive"
            and int(event.get("actor_id") or 0) > 0
        ):
            approvals.append(event)
    if len(approvals) != 1:
        return None
    approval = approvals[0]
    selected_by_scope = {}
    for scope in task_scopes:
        scope_key = (str(scope["scope_kind"]), int(scope["scope_ref_id"]))
        expected_ids = contract[scope_key]
        observed = sorted(
            (
                asset
                for asset in assets
                if scope_matches(scope, asset)
                and active_asset(asset)
                and at_or_before(asset, approval["created_at"])
                and str(asset.get("asset_type") or "") in {"source", "delivery"}
            ),
            key=lambda asset: (asset["created_at"], int(asset["id"])),
        )
        if [int(asset["id"]) for asset in observed] != expected_ids:
            return None
        if any(str(asset.get("asset_type") or "") != "source" for asset in observed):
            return None
        selected_by_scope[scope_key] = observed
    return {"event": approval, "selected_by_scope": selected_by_scope}


def build_customization_terminal_without_assets_resource(
    scope, candidate, assets, references
):
    event = candidate["event"]
    scope_key = (str(scope["scope_kind"]), int(scope["scope_ref_id"]))
    revision = make_revision(
        scope,
        event,
        1,
        "draft",
        "reopen",
        assets,
        references,
        selected_assets=candidate["selected_by_scope"][scope_key],
    )
    revision["reason"] = (
        f"policy {CUSTOMIZATION_TERMINAL_WITHOUT_ASSETS_POLICY}: retired "
        "customization approval reached PendingWarehouseReceive without a "
        "complete final snapshot; preserve only the exact allowlisted source "
        "membership and reopen an editable draft; human confirmation remains "
        "required"
    )
    blockers = list(revision.pop("_blockers", []))
    if blockers:
        revision["blockers"] = blockers
        revision["confidence"] = "hard_blocked"
    revision["review_policy_ids"] = revision_review_policy_ids(scope, revision)
    recompute_revision_hash(revision)
    return {
        "task_id": int(scope["task_id"]),
        "scope_kind": str(scope["scope_kind"]),
        "scope_ref_id": int(scope["scope_ref_id"]),
        "history": [revision],
        "working_revision_no": 1,
    }


def reopen_completed_customization_missing_final(
    scope: dict[str, Any],
    before_cleanup: dict[str, Any] | None,
    after_cleanup: dict[str, Any],
    assets: Iterable[dict[str, Any]],
) -> dict[str, Any] | None:
    """Reopen one frozen Completed customization row whose object is gone."""

    contract = LEGACY_COMPLETED_CUSTOMIZATION_MISSING_FINAL.get(
        int(scope["task_id"])
    )
    if (
        contract is None
        or before_cleanup is None
        or str(scope.get("task_status") or "") != "Completed"
        or str(scope.get("scope_kind") or "") != contract["scope_kind"]
        or int(scope.get("scope_ref_id") or 0) != contract["scope_ref_id"]
        or before_cleanup.get("status") != "finalized"
        or before_cleanup.get("final_task_asset_ids")
        not in ([], [contract["final_task_asset_id"]])
        or before_cleanup.get("source_task_asset_id") is not None
        or before_cleanup.get("source_alias_from_task_asset_id")
        not in (None, *contract["source_alias_candidates"])
        or after_cleanup.get("final_task_asset_ids")
        or after_cleanup.get("source_task_asset_id")
        or after_cleanup.get("source_alias_from_task_asset_id")
    ):
        return None
    asset = next(
        (
            candidate
            for candidate in assets
            if int(candidate.get("id") or 0)
            == contract["final_task_asset_id"]
        ),
        None,
    )
    if (
        asset is None
        or str(asset.get("asset_type") or "") != "delivery"
        or str(asset.get("deleted_at") or "") != contract["deleted_at"]
        or str(asset.get("storage_ref_id") or "")
        != contract["storage_ref_id"]
        or str(asset.get("approved_at") or "")
        != str(before_cleanup.get("created_at") or "")
        or str(asset.get("source_module_key") or "") != "customization"
        or asset.get("superseded_by_version_id")
    ):
        return None

    historical = copy.deepcopy(before_cleanup)
    historical["status"] = "superseded"
    historical["mode"] = "single"
    historical["final_task_asset_ids"] = [contract["final_task_asset_id"]]
    historical.pop("source_task_asset_id", None)
    historical["source_alias_from_task_asset_id"] = contract[
        "final_task_asset_id"
    ]
    historical["reason"] = (
        f"policy {HISTORICAL_ASSET_UNAVAILABLE_POLICY}: the approved final "
        "was valid at this revision boundary but its immutable object is no "
        "longer available; preserve the revision as read-only history"
    )
    historical.setdefault("_review_policy_ids", []).append(
        HISTORICAL_ASSET_UNAVAILABLE_POLICY
    )
    historical.pop("_blockers", None)
    historical.pop("blockers", None)
    recompute_revision_hash(historical)

    draft = copy.deepcopy(historical)
    draft["revision_no"] = int(historical["revision_no"]) + 1
    draft["status"] = "draft"
    draft["source_stage"] = "reopen"
    draft.pop("source_task_asset_id", None)
    draft.pop("source_alias_from_task_asset_id", None)
    draft["final_task_asset_ids"] = []
    draft["created_at"] = contract["deleted_at"]
    draft.pop("submitted_at", None)
    draft.pop("finalized_at", None)
    draft["reason"] = (
        f"policy {CUSTOMIZATION_TERMINAL_WITHOUT_ASSETS_POLICY}: the legacy "
        "Completed customization lost its only final object; reopen an "
        "editable draft without inventing a replacement"
    )
    draft["_review_policy_ids"] = ordered_review_policy_ids(
        [
            EXPLICIT_EVENT_REPLAY_POLICY,
            REOPEN_POLICY,
            CUSTOMIZATION_TERMINAL_WITHOUT_ASSETS_POLICY,
        ]
    )
    draft["confidence"] = "proposed_review"
    recompute_revision_hash(draft)
    return {
        "historical": historical,
        "draft": draft,
        "evidence_event_ids": list(
            dict.fromkeys(historical.get("evidence_event_ids") or [])
        ),
    }


def build_premature_retouch_resource(scope, candidate, assets, references):
    membership = candidate["_retouch_scope_memberships"][
        str(scope["scope_ref_id"])
    ]
    asset_by_id = {int(asset["id"]): asset for asset in assets}
    selected = [
        asset_by_id[int(asset_id)]
        for asset_id in membership.get("asset_version_ids") or []
    ]
    evidence = list(membership.get("completion_event_ids") or [])
    evidence.extend(event_evidence_ids(candidate))
    task_scope = (int(scope["task_id"]), int(scope["scope_ref_id"]))
    policy_reason = (
        f"policy {RETOUCH_PREMATURE_TERMINAL_PARTIAL_POLICY}: legacy "
        "Completed was premature; preserve only allowlisted partial membership "
        "and reopen an editable draft; human confirmation remains required"
    )

    revisions = []
    if task_scope in LEGACY_RETOUCH_PREMATURE_FINALIZED_SCOPES:
        finalized = make_revision(
            scope,
            candidate,
            1,
            "finalized",
            "retouch",
            assets,
            references,
            selected_assets=selected,
            extra_evidence=evidence,
        )
        finalized["reason"] = policy_reason
        recompute_revision_hash(finalized)
        draft = make_revision(
            scope,
            candidate,
            2,
            "draft",
            "reopen",
            assets,
            references,
            inherited=finalized,
            extra_evidence=evidence,
        )
        draft["reason"] = policy_reason
        recompute_revision_hash(draft)
        revisions = [finalized, draft]
        working, finalized_no = 2, 1
    else:
        if (
            not selected
            and int(scope["scope_ref_id"])
            == min(
                int(value)
                for value in candidate[
                    "_retouch_scope_memberships"
                ]
            )
        ):
            # Preserve exact event coverage without claiming that the unscoped
            # delivery belongs to this requirement.
            evidence = list(
                candidate.get("_unassigned_completion_event_ids") or []
            ) + event_evidence_ids(candidate)
        draft = make_revision(
            scope,
            candidate,
            1,
            "draft",
            "reopen",
            assets,
            references,
            selected_assets=selected,
            extra_evidence=evidence,
        )
        draft["reason"] = policy_reason
        # Task-level legacy references cannot be assigned across multiple
        # requirements without exact range evidence.
        draft["reference_file_ref_ids"] = []
        recompute_revision_hash(draft)
        revisions = [draft]
        working, finalized_no = 1, None

    for revision in revisions:
        blockers = list(revision.pop("_blockers", []))
        if blockers:
            revision["blockers"] = blockers
            revision["confidence"] = "hard_blocked"
        revision["evidence_event_ids"] = list(
            dict.fromkeys(revision.get("evidence_event_ids") or [])
        )
        revision["review_policy_ids"] = revision_review_policy_ids(
            scope, revision
        )
        recompute_revision_hash(revision)
    resource = {
        "task_id": int(scope["task_id"]),
        "scope_kind": "retouch_requirement",
        "scope_ref_id": int(scope["scope_ref_id"]),
        "history": revisions,
        "working_revision_no": working,
    }
    if finalized_no is not None:
        resource["finalized_revision_no"] = finalized_no
    return resource


def build_retouch_visual_scope_task2533_resource(
    scope, candidate, assets, references
):
    membership = candidate["by_scope"][int(scope["scope_ref_id"])]
    completion_evidence = sorted_evidence_ids(
        (
            evidence_id
            for event in membership["completion_events"]
            for evidence_id in event_evidence_ids(event)
        ),
        membership["completion_events"],
    )
    revision = make_revision(
        scope,
        candidate["submit"],
        1,
        "finalized",
        "retouch",
        assets,
        references,
        selected_assets=[membership["source"], membership["final"]],
        extra_evidence=completion_evidence,
    )
    revision["reference_file_ref_ids"] = list(membership["references"])
    revision["reason"] = (
        f"policy {RETOUCH_VISUAL_SCOPE_TASK2533_POLICY}: read-only "
        "production-page visual review binds this exact source/final/reference "
        "membership; delivery asset 19803 remains preserved but unassigned; "
        "human policy confirmation remains required"
    )
    blockers = list(revision.pop("_blockers", []))
    if blockers:
        revision["blockers"] = blockers
        revision["confidence"] = "hard_blocked"
    revision["review_policy_ids"] = revision_review_policy_ids(scope, revision)
    revision["evidence_event_ids"] = sorted_evidence_ids(
        revision["evidence_event_ids"],
        membership["completion_events"] + [candidate["submit"]],
    )
    recompute_revision_hash(revision)
    return {
        "task_id": 2533,
        "scope_kind": "retouch_requirement",
        "scope_ref_id": int(scope["scope_ref_id"]),
        "history": [revision],
        "working_revision_no": 1,
        "finalized_revision_no": 1,
    }


def build_object_manifest(rows):
    result = []
    for owner_kind, source in (("task_asset", rows["assets"]), ("reference_file_ref", rows["references"])):
        for row in source:
            result.append({
                "entity_key": f'{owner_kind}:{int(row["id"])}',
                "owner_kind": owner_kind, "owner_id": int(row["id"]), "task_id": int(row["task_id"]),
                "storage_ref_id": row.get("storage_ref_id") or row.get("ref_id") or "",
                "storage_adapter": row.get("storage_adapter") or "",
                "object_key": row.get("storage_key") or row.get("ref_key") or "",
                "size": row.get("file_size"), "mime_type": row.get("mime_type") or "",
                "sha256": row.get("whole_hash") or row.get("checksum_hint") or "",
                "status": row.get("storage_status") or "", "is_placeholder": bool(row.get("is_placeholder")),
            })
    return sorted(result, key=lambda x: (x["owner_kind"], x["owner_id"]))


ORG_DEPARTMENT_RENAMES = {
    "设计研发部": "视觉研创部",
    "设计部": "视觉研创部",
    "定制美工部": "定制中心",
    "人事部": "人力行政中心",
}

ORG_TEAM_RENAMES = {
    "淘系一组": "淘系运营一部",
    "淘系二组": "淘系运营二部",
    "淘系三组": "淘系运营三部",
    "天猫一组": "天猫运营一部（南京）",
    "天猫二组": "天猫运营一部（池州）",
    "拼多多南京组": "拼多多运营一部（南京）",
    "拼多多运营部（南京）": "拼多多运营一部（南京）",
    "拼多多运营部(南京)": "拼多多运营一部（南京）",
    "拼多多池州组": "拼多多运营二部（池州)",
    "拼多多运营二部（池州）": "拼多多运营二部（池州)",
    "拼多多运营二部(池州)": "拼多多运营二部（池州)",
}

KNOWN_ACCESS_ROLES = {
    "Member",
    "SuperAdmin",
    "Admin",
    "RoleAdmin",
    "Ops",
    "Designer",
    "CustomizationOperator",
    "Audit_A",
    "Audit_B",
    "DesignReviewer",
    "CustomizationReviewer",
    "AssetSubmitter",
    "AssetManager",
    "AssetTemplateAdmin",
    "AssetSettlement",
    "HRAdmin",
    "DepartmentAdmin",
    "TeamLead",
    "DesignDirector",
    "ERP",
}


def positive_or_none(value: Any) -> int | None:
    number = int(value or 0)
    return number if number > 0 else None


def organization_issue(row: dict[str, Any], subject_type: str, teams_by_id: dict[int, dict[str, Any]]) -> bool:
    department = str(row.get("legacy_department") or "").strip()
    team = str(row.get("legacy_team") or "").strip()
    department_id = positive_or_none(row.get("department_id"))
    team_id = positive_or_none(row.get("team_id"))
    if department and department_id is None:
        return True
    if team and team_id is None:
        return True
    if team_id is not None:
        target = teams_by_id.get(team_id)
        if department_id is None or target is None or int(target["department_id"]) != department_id:
            return True
    return False


def build_organization_mappings(rows: dict[str, Any]) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    departments = [row for row in rows.get("org_departments", []) if int(row.get("enabled") or 0) == 1]
    teams = [row for row in rows.get("org_teams", []) if int(row.get("enabled") or 0) == 1]
    all_teams_by_id = {int(row["id"]): row for row in rows.get("org_teams", [])}
    departments_by_id = {int(row["id"]): row for row in departments}
    departments_by_name: dict[str, list[dict[str, Any]]] = defaultdict(list)
    teams_by_id = {int(row["id"]): row for row in teams}
    teams_by_department_name: dict[tuple[int, str], list[dict[str, Any]]] = defaultdict(list)
    for row in departments:
        departments_by_name[str(row["name"]).strip()].append(row)
    for row in teams:
        teams_by_department_name[(int(row["department_id"]), str(row["name"]).strip())].append(row)

    candidates: list[tuple[str, dict[str, Any]]] = []
    for row in rows.get("users_org", []):
        candidates.append(("user", row))
    for row in rows.get("tasks_org", []):
        candidates.append(("task", row))

    mappings: list[dict[str, Any]] = []
    manual: list[dict[str, Any]] = []
    for subject_type, row in sorted(
        candidates, key=lambda value: (value[0], int(value[1]["id"]))
    ):
        if not organization_issue(row, subject_type, all_teams_by_id):
            continue
        subject_id = int(row["id"])
        legacy_department = str(row.get("legacy_department") or "").strip()
        legacy_team = str(row.get("legacy_team") or "").strip()
        from_department_id = positive_or_none(row.get("department_id"))
        from_team_id = positive_or_none(row.get("team_id"))
        used_alias = False
        uat_orphan_target = (
            UAT_ORPHAN_ORG_TARGETS.get(subject_id)
            if subject_type == "task"
            else None
        )

        target_department_id = from_department_id
        if target_department_id not in departments_by_id:
            target_department_name = ORG_DEPARTMENT_RENAMES.get(
                legacy_department, legacy_department
            )
            used_alias = target_department_name != legacy_department
            matches = departments_by_name.get(target_department_name, [])
            target_department_id = int(matches[0]["id"]) if len(matches) == 1 else None

        target_team_id = from_team_id
        if (
            target_team_id not in teams_by_id
            or target_department_id is None
            or int(teams_by_id[target_team_id]["department_id"]) != target_department_id
        ):
            target_team_name = ORG_TEAM_RENAMES.get(legacy_team, legacy_team)
            used_alias = used_alias or target_team_name != legacy_team
            matches = (
                teams_by_department_name.get((target_department_id, target_team_name), [])
                if target_department_id is not None
                else []
            )
            target_team_id = int(matches[0]["id"]) if len(matches) == 1 else None

        if uat_orphan_target is not None:
            expected_department_id, expected_team_id = uat_orphan_target
            target_team = teams_by_id.get(expected_team_id)
            if (
                expected_department_id not in departments_by_id
                or target_team is None
                or int(target_team["department_id"]) != expected_department_id
            ):
                raise ValueError(
                    "frozen unassigned organization target 3/14 is unavailable "
                    f"for UAT task {subject_id}"
                )
            target_department_id = expected_department_id
            target_team_id = expected_team_id

        blockers: list[str] = []
        if target_department_id is None:
            blockers.append(
                f"no unique enabled stable department matches {legacy_department or '<empty>'}"
            )
        if target_team_id is None:
            blockers.append(
                f"no unique enabled stable team matches {legacy_team or '<empty>'}"
            )
        confidence = "hard_blocked" if blockers else "proposed_review"
        policies = [
            UAT_ORPHAN_ORG_POLICY
            if uat_orphan_target is not None
            else ORG_MANUAL_TARGET_POLICY
            if blockers
            else ORG_ALIAS_LINEAGE_POLICY
            if used_alias
            else ORG_UNIQUE_STABLE_POLICY
        ]
        mapping = {
            "subject_type": subject_type,
            "subject_id": subject_id,
            "legacy_department": legacy_department,
            "legacy_team": legacy_team,
            "from_department_id": from_department_id,
            "from_team_id": from_team_id,
            "target_department_id": int(target_department_id or 0),
            "target_team_id": int(target_team_id or 0),
            "confidence": confidence,
            "review_policy_ids": ordered_review_policy_ids(policies),
            "confirmed_by": 0,
            "confirmed_at": ZERO_TIME,
            "confirmation_note": "",
            **({"blockers": blockers} if blockers else {}),
        }
        recompute_mapping_row_hash(mapping)
        mappings.append(mapping)
        manual.append(
            {
                "task_id": subject_id,
                "scope_kind": f"organization:{subject_type}",
                "scope_ref_id": 0,
                "revision_no": "",
                "confidence": confidence,
                "reason": "; ".join(blockers)
                if blockers
                else f"policy review required: {policies[0]}",
                "evidence_event_ids": "",
                "candidate_source_ids": "",
                "candidate_final_ids": "",
                "reviewer_id": "",
                "reviewed_at": "",
                "decision": "",
                "review_note": "",
            }
        )
    return mappings, manual


def assignment_evidence_key(row: dict[str, Any]) -> tuple[str, str, str, int]:
    return (
        str(row.get("role_code") or ""),
        str(row.get("scope_mode") or ""),
        str(row.get("source_type") or ""),
        int(row.get("source_ref_id") or 0),
    )


def build_access_decisions(
    rows: dict[str, Any],
    organization_mappings: list[dict[str, Any]] | None = None,
) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    active_users = {
        int(row["id"]): row
        for row in rows.get("users_org", [])
        if str(row.get("status") or "") == "active"
    }
    assignments_by_user: dict[int, list[dict[str, Any]]] = defaultdict(list)
    for row in rows.get("access_assignments", []):
        assignments_by_user[int(row["user_id"])].append(
            {
                "role_code": str(row.get("role_code") or ""),
                "scope_mode": str(row.get("scope_mode") or ""),
                "source_type": str(row.get("source_type") or ""),
                "source_ref_id": int(row.get("source_ref_id") or 0),
            }
        )
    for user_id in assignments_by_user:
        assignments_by_user[user_id].sort(key=assignment_evidence_key)
    resolved_user_org = {
        int(item["subject_id"]): (
            positive_or_none(item.get("target_department_id")),
            positive_or_none(item.get("target_team_id")),
        )
        for item in organization_mappings or []
        if item.get("subject_type") == "user"
        and item.get("confidence") != "hard_blocked"
    }

    decisions: list[dict[str, Any]] = []
    manual: list[dict[str, Any]] = []
    for role_row in sorted(
        rows.get("legacy_roles", []),
        key=lambda row: (int(row["user_id"]), str(row["role"])),
    ):
        user_id = int(role_row["user_id"])
        legacy_role = str(role_row["role"])
        user = active_users.get(user_id)
        if user is None:
            continue
        resolved_department_id, resolved_team_id = resolved_user_org.get(
            user_id, (None, None)
        )
        department_id = (
            positive_or_none(user.get("department_id"))
            or resolved_department_id
        )
        team_id = positive_or_none(user.get("team_id")) or resolved_team_id
        is_issue = (
            legacy_role not in KNOWN_ACCESS_ROLES
            or (
                legacy_role in {"DepartmentAdmin", "DesignDirector"}
                and positive_or_none(user.get("department_id")) is None
            )
            or (
                legacy_role == "TeamLead"
                and positive_or_none(user.get("team_id")) is None
            )
        )
        if not is_issue:
            continue

        evidence = assignments_by_user.get(user_id, [])
        blockers: list[str] = []
        action = ""
        policies: list[str]
        if legacy_role == "Warehouse":
            action = "no_new_grant"
            policies = [WAREHOUSE_NO_GRANT_POLICY]
        elif (
            legacy_role in {"DepartmentAdmin", "DesignDirector"}
            and department_id is not None
        ) or (legacy_role == "TeamLead" and team_id is not None):
            # The separately reviewed organization mapping supplies the
            # missing stable legacy scope. This access decision acknowledges
            # the pre-cutover raw issue but contributes no additional grant;
            # normal legacy-role migration remains responsible for the
            # scoped V8 assignment.
            action = "no_new_grant"
            policies = [EXISTING_ACCESS_PRESERVED_POLICY]
        elif legacy_role == "OrgAdmin" and any(
            row["role_code"] in {"super_admin", "access_admin"}
            and row["scope_mode"] == "global"
            for row in evidence
        ):
            action = "preserve_existing"
            policies = [EXISTING_ACCESS_PRESERVED_POLICY]
        elif legacy_role in {"Outsource", "OrgAdmin"} and evidence:
            # Neither legacy role has a stable V8 equivalent.  The owner has
            # approved preserving the independently-existing assignments and
            # adding no replacement grant.  This is deliberately not
            # ``preserve_existing``: the legacy role itself contributes no
            # authority after cutover.
            action = "no_new_grant"
            policies = [EXISTING_ACCESS_PRESERVED_POLICY]
        elif legacy_role == "Outsource":
            policies = [OUTSOURCE_ACCESS_DECISION_POLICY]
            blockers.append(
                "Outsource has no V8 role equivalent and the user has no "
                "independent V8 assignment evidence"
            )
        elif legacy_role == "OrgAdmin":
            policies = [ORG_ADMIN_ACCESS_DECISION_POLICY]
            blockers.append(
                "OrgAdmin has no independent V8 assignment evidence and "
                "cannot be widened to global access_admin"
            )
        else:
            policies = [EXISTING_ACCESS_PRESERVED_POLICY]
            blockers.append(
                "legacy role lacks a complete stable organization scope or explicit V8 replacement"
            )
        # A no-new-grant decision is complete precisely because it contributes
        # no V8 authority. Requiring an existing assignment here made newly
        # created retired Warehouse users impossible to migrate despite the
        # approved least-privilege policy.
        if not evidence and action != "no_new_grant":
            blockers.append("user has no explicit V8 assignment evidence")
        confidence = "hard_blocked" if blockers else "proposed_review"
        decision = {
            "user_id": user_id,
            "legacy_role": legacy_role,
            "action": action,
            "required_existing_assignments": evidence,
            "confidence": confidence,
            "review_policy_ids": ordered_review_policy_ids(policies),
            "confirmed_by": 0,
            "confirmed_at": ZERO_TIME,
            "confirmation_note": "",
            **({"blockers": blockers} if blockers else {}),
        }
        recompute_mapping_row_hash(decision)
        decisions.append(decision)
        manual.append(
            {
                "task_id": user_id,
                "scope_kind": f"access:{legacy_role}",
                "scope_ref_id": 0,
                "revision_no": "",
                "confidence": confidence,
                "reason": "; ".join(blockers)
                if blockers
                else f"policy review required: {policies[0]}",
                "evidence_event_ids": "",
                "candidate_source_ids": "",
                "candidate_final_ids": "",
                "reviewer_id": "",
                "reviewed_at": "",
                "decision": "",
                "review_note": "",
            }
        )
    return decisions, manual


def build_deleted_asset_recoveries(
    rows: dict[str, Any],
) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    assets_by_id = {
        int(row["id"]): row
        for row in rows.get("assets", [])
        if row.get("id") is not None
    }
    relevant_ids = set(LEGACY_DELETED_ASSET_RECOVERY_EVIDENCE)
    if relevant_ids.isdisjoint(assets_by_id):
        return [], []
    if not relevant_ids.issubset(assets_by_id):
        missing = sorted(relevant_ids - set(assets_by_id))
        raise ValueError(
            f"deleted-asset recovery evidence is incomplete; missing task_assets {missing}"
        )

    recoveries = []
    manual = []
    for missing_id, evidence in sorted(
        LEGACY_DELETED_ASSET_RECOVERY_EVIDENCE.items()
    ):
        missing = assets_by_id[missing_id]
        if (
            int(missing.get("task_id") or 0) != evidence["task_id"]
            or int(missing.get("file_size") or 0)
            != evidence["expected_file_size"]
            or str(missing.get("storage_ref_id") or "")
            != evidence["original_storage_ref_id"]
        ):
            raise ValueError(
                f"task_asset {missing_id} differs from the frozen recovery identity"
            )
        source_id = evidence["recovery_source_task_asset_id"]
        if source_id:
            source = assets_by_id.get(source_id)
            if source is None:
                raise ValueError(
                    f"task_asset {missing_id} recovery source {source_id} is absent"
                )
            if (
                int(source.get("file_size") or 0)
                != evidence["expected_file_size"]
                or str(source.get("storage_ref_id") or "")
                != evidence["recovery_source_storage_ref_id"]
            ):
                raise ValueError(
                    f"task_asset {missing_id} source {source_id} differs from the frozen size/storage identity"
                )
            source_task_assets = [
                row
                for row in assets_by_id.values()
                if int(row.get("task_id") or 0)
                == int(source.get("task_id") or 0)
            ]
            if not any(
                row.get("asset_type") == "preview"
                and int(row.get("source_asset_version_id") or 0) == source_id
                and row.get("whole_hash") == evidence["preview_whole_hash"]
                for row in source_task_assets
            ) or not any(
                row.get("asset_type") == "design_thumb"
                and int(row.get("source_asset_version_id") or 0) == source_id
                and row.get("whole_hash")
                == evidence["design_thumb_whole_hash"]
                for row in source_task_assets
            ):
                raise ValueError(
                    f"task_asset {missing_id} source {source_id} lacks pairwise-identical derivative hashes"
                )
        rejected_ids = evidence["rejected_source_task_asset_ids"]
        if rejected_ids:
            rejected_sizes = []
            for rejected_id in rejected_ids:
                rejected = assets_by_id.get(rejected_id)
                if rejected is None:
                    raise ValueError(
                        f"task_asset {missing_id} rejected source {rejected_id} is absent"
                    )
                rejected_sizes.append(int(rejected.get("file_size") or 0))
            if any(
                size == evidence["expected_file_size"]
                for size in rejected_sizes
            ):
                raise ValueError(
                    f"task_asset {missing_id} rejected source unexpectedly matches the original byte size"
                )

        task_assets = [
            row
            for row in assets_by_id.values()
            if int(row.get("task_id") or 0) == evidence["task_id"]
        ]
        if not any(
            row.get("asset_type") == "preview"
            and int(row.get("source_asset_version_id") or 0) == missing_id
            and row.get("whole_hash") == evidence["preview_whole_hash"]
            for row in task_assets
        ) or not any(
            row.get("asset_type") == "design_thumb"
            and int(row.get("source_asset_version_id") or 0) == missing_id
            and row.get("whole_hash") == evidence["design_thumb_whole_hash"]
            for row in task_assets
        ):
            raise ValueError(
                f"task_asset {missing_id} lacks the frozen preview/design-thumb evidence"
            )

        historical_unavailable = not source_id
        recovery = {
            "task_id": evidence["task_id"],
            "missing_task_asset_id": missing_id,
            "recovery_source_task_asset_id": source_id,
            **(
                {"rejected_source_task_asset_ids": rejected_ids}
                if rejected_ids
                else {}
            ),
            "strategy": (
                "historical_unavailable_tombstone_v1"
                if historical_unavailable
                else "verified_oss_recovery_v1"
            ),
            "original_storage_ref_id": evidence["original_storage_ref_id"],
            **(
                {
                    "recovery_source_storage_ref_id":
                    evidence["recovery_source_storage_ref_id"]
                }
                if evidence["recovery_source_storage_ref_id"]
                else {}
            ),
            "expected_file_size": evidence["expected_file_size"],
            "preview_whole_hash": evidence["preview_whole_hash"],
            "design_thumb_whole_hash": evidence["design_thumb_whole_hash"],
            **(
                {
                    "controlled_read_protocol":
                        evidence["controlled_read_protocol"],
                    "controlled_read_evidence_sha256":
                        evidence["controlled_read_evidence_sha256"],
                    "recovery_source_sha256":
                        evidence["recovery_source_sha256"],
                }
                if not historical_unavailable
                else {}
            ),
            **(
                {
                    "object_probe_result": evidence["object_probe_result"],
                    "object_probe_input_manifest_sha256":
                        evidence["object_probe_input_manifest_sha256"],
                    "object_probe_evidence_hash":
                        evidence["object_probe_evidence_hash"],
                    "object_probe_object_key_sha256":
                        evidence["object_probe_object_key_sha256"],
                    "object_probe_read_only_get_count":
                        evidence["object_probe_read_only_get_count"],
                }
                if historical_unavailable
                else {}
            ),
            "confidence": "proposed_review",
            "review_policy_ids": [
                HISTORICAL_ASSET_UNAVAILABLE_POLICY
                if historical_unavailable
                else DELETED_ASSET_RECOVERY_POLICY
            ],
            "confirmed_by": 0,
            "confirmed_at": ZERO_TIME,
            "confirmation_note": "",
        }
        recompute_mapping_row_hash(recovery)
        recoveries.append(recovery)
        manual.append(
            {
                "task_id": evidence["task_id"],
                "scope_kind": "asset_recovery",
                "scope_ref_id": missing_id,
                "revision_no": "",
                "confidence": recovery["confidence"],
                "reason": (
                    "same-root successors 14510/14514 have different sizes and "
                    "must never replace task_asset 12323"
                    if historical_unavailable
                    else (
                        f"policy review required: {DELETED_ASSET_RECOVERY_POLICY}; "
                        f"preserve task_asset {missing_id} identity and pre-materialize "
                        f"bytes from {source_id} only inside a run-scoped Clone B root"
                    )
                ),
                "evidence_event_ids": "",
                "candidate_source_ids": str(source_id) if source_id else "14510|14514",
                "candidate_final_ids": "",
                "reviewer_id": "",
                "reviewed_at": "",
                "decision": "",
                "review_note": "",
            }
        )
    return recoveries, manual


def generate(rows):
    assets_by_task, refs_by_task, events_by_task = defaultdict(list), defaultdict(list), defaultdict(list)
    for row in rows["assets"]:
        assets_by_task[int(row["task_id"])].append(row)
    for row in rows["references"]:
        refs_by_task[int(row["task_id"])].append(row)
    for row in rows["events"]:
        events_by_task[int(row["task_id"])].append(row)
    sku_scope_counts = defaultdict(int)
    retouch_scope_counts = defaultdict(int)
    scopes_by_task = defaultdict(list)
    for scope in rows["scopes"]:
        scopes_by_task[int(scope["task_id"])].append(scope)
        if scope["scope_kind"] == "sku":
            sku_scope_counts[int(scope["task_id"])] += 1
        elif scope["scope_kind"] == "retouch_requirement":
            retouch_scope_counts[int(scope["task_id"])] += 1
    batch_submits_by_task = {
        task_id: candidates
        for task_id, task_scopes in scopes_by_task.items()
        if (candidates := resolve_atomic_multi_sku_batch_submits(
            task_scopes,
            events_by_task[task_id],
            assets_by_task[task_id],
        ))
    }
    retouch_atomic_submit_by_task = {
        task_id: candidate
        for task_id, task_scopes in scopes_by_task.items()
        if (
            candidate := resolve_legacy_retouch_unscoped_atomic_batch(
                task_scopes,
                events_by_task[task_id],
                assets_by_task[task_id],
            )
        )
        is not None
    }
    retouch_partial_submit_by_task = {
        task_id: candidate
        for task_id, task_scopes in scopes_by_task.items()
        if (
            candidate := resolve_legacy_retouch_premature_partial(
                task_scopes,
                events_by_task[task_id],
                assets_by_task[task_id],
            )
        )
        is not None
    }
    retouch_unscoped_ambiguous_by_task = {
        task_id: candidate
        for task_id, task_scopes in scopes_by_task.items()
        if (
            task_id not in LEGACY_RETOUCH_UNSCOPED_ATOMIC_FINALS
            and task_id not in LEGACY_RETOUCH_PREMATURE_PARTIAL_FINALS
            and task_id != 2533
            and task_id not in retouch_atomic_submit_by_task
            and task_id not in retouch_partial_submit_by_task
            and (
                candidate :=
                resolve_legacy_retouch_unscoped_ambiguous_terminal(
                    task_scopes,
                    events_by_task[task_id],
                    assets_by_task[task_id],
                )
            )
            is not None
        )
    }
    retouch_visual_scope_by_task = {
        task_id: candidate
        for task_id, task_scopes in scopes_by_task.items()
        if (
            candidate := resolve_legacy_retouch_visual_scope_task2533(
                task_scopes,
                events_by_task[task_id],
                assets_by_task[task_id],
                refs_by_task[task_id],
            )
        )
        is not None
    }
    customization_terminal_without_assets_by_task = {
        task_id: candidate
        for task_id, task_scopes in scopes_by_task.items()
        if (
            candidate := resolve_customization_terminal_without_assets(
                task_scopes,
                events_by_task[task_id],
                assets_by_task[task_id],
            )
        )
        is not None
    }
    manual, resources = [], []
    completed_customization_missing_final_by_task = {}
    for scope in sorted(rows["scopes"], key=lambda x: (int(x["task_id"]), x["scope_kind"], int(x["scope_ref_id"]))):
        scope = dict(scope)
        task_id = int(scope["task_id"])
        scope["_single_sku"] = scope["scope_kind"] == "sku" and sku_scope_counts[task_id] == 1
        scope["_single_requirement"] = (
            scope["scope_kind"] == "retouch_requirement" and retouch_scope_counts[task_id] == 1
        )
        if (
            scope["scope_kind"] == "retouch_requirement"
            and task_id in retouch_visual_scope_by_task
        ):
            resource = build_retouch_visual_scope_task2533_resource(
                scope,
                retouch_visual_scope_by_task[task_id],
                assets_by_task[task_id],
                refs_by_task[task_id],
            )
            resources.append(resource)
            revision = resource["history"][0]
            manual.append(
                review_row(
                    scope,
                    revision,
                    revision["confidence"],
                    revision["reason"],
                )
            )
            continue
        if task_id in customization_terminal_without_assets_by_task:
            resource = build_customization_terminal_without_assets_resource(
                scope,
                customization_terminal_without_assets_by_task[task_id],
                assets_by_task[task_id],
                refs_by_task[task_id],
            )
            resources.append(resource)
            revision = resource["history"][0]
            manual.append(
                review_row(
                    scope,
                    revision,
                    revision["confidence"],
                    revision["reason"],
                )
            )
            continue
        if (
            scope["scope_kind"] == "retouch_requirement"
            and (
                task_id in retouch_partial_submit_by_task
                or task_id in retouch_unscoped_ambiguous_by_task
            )
        ):
            candidate = (
                retouch_partial_submit_by_task.get(task_id)
                or retouch_unscoped_ambiguous_by_task[task_id]
            )
            resource = build_premature_retouch_resource(
                scope,
                candidate,
                assets_by_task[task_id],
                refs_by_task[task_id],
            )
            resources.append(resource)
            for revision in resource["history"]:
                manual.append(
                    review_row(
                        scope,
                        revision,
                        revision["confidence"],
                        revision["reason"],
                    )
                )
            continue
        boundary = scoped_boundary_events(
            scope,
            events_by_task[task_id],
            assets_by_task[task_id],
            sku_scope_counts[task_id],
        )
        if scope["scope_kind"] == "sku" and task_id in batch_submits_by_task:
            atomic_submits = [
                dict(candidate)
                for candidate in batch_submits_by_task[task_id]
            ]
            boundary = [
                event for event in boundary
                if event_kind(event) != "submit"
            ] + atomic_submits
            boundary.sort(key=lambda event: (
                event.get("created_at", ""),
                event.get("namespace") == "task_module_event",
                str(event.get("id") or ""),
            ))
        if (
            scope["scope_kind"] == "retouch_requirement"
            and task_id in retouch_atomic_submit_by_task
        ):
            atomic_submit = dict(retouch_atomic_submit_by_task[task_id])
            boundary = [
                event for event in boundary if event_kind(event) != "submit"
            ] + [atomic_submit]
            boundary.sort(
                key=lambda event: (
                    event.get("created_at", ""),
                    event.get("namespace") == "task_module_event",
                    str(event.get("id") or ""),
                )
            )
        revisions = []
        working = None
        finalized = None
        if not boundary:
            status = effective_v8_status(str(scope.get("task_status") or ""))
            if task_id in retouch_unscoped_ambiguous_by_task:
                manual.append(
                    review_row(
                        scope,
                        None,
                        "proposed_review",
                        (
                            f"policy "
                            f"{RETOUCH_PREMATURE_TERMINAL_PARTIAL_POLICY}: "
                            "the task-level terminal upload is proven, but "
                            "multi-requirement membership is absent; preserve "
                            "unscoped assets and reopen an empty requirement "
                            "shell"
                        ),
                    )
                )
            elif status in {"PendingAudit", "Completed"}:
                manual.append(review_row(scope, None, "hard_blocked", "no revision boundary event exists for this scope"))
            else:
                manual.append(review_row(
                    scope,
                    None,
                    "proposed_review",
                    f"empty V8 shell candidate for task status {status or '<empty>'}; scope existence still requires review",
                ))
        for event in boundary:
            kind = event_kind(event)
            current = revisions[working - 1] if working else None
            if kind == "submit":
                selected_assets, completion_evidence, selection_blockers = resolve_submission_assets(
                    scope, event, events_by_task[task_id], assets_by_task[task_id]
                )
                if current is not None and current["status"] == "draft":
                    candidate = make_revision(
                        scope, event, current["revision_no"], "submitted", current["source_stage"],
                        assets_by_task[task_id], refs_by_task[task_id], selected_assets=selected_assets,
                        extra_evidence=completion_evidence, selection_blockers=selection_blockers,
                    )
                    prior_evidence = list(current["evidence_event_ids"])
                    prior_created_at = current["created_at"]
                    prior_created_by = current["created_by"]
                    prior_blockers = list(current.get("_blockers", []))
                    if not prior_created_by and candidate.get("created_by"):
                        # A repair-created legacy reopen can have no operator,
                        # while the subsequent submit has the exact business
                        # actor that materializes the replacement revision.
                        prior_created_by = candidate["created_by"]
                        prior_blockers = [
                            blocker
                            for blocker in prior_blockers
                            if blocker != "legacy event has no actor_id"
                        ]
                    current.update(candidate)
                    current["created_at"] = prior_created_at
                    current["created_by"] = prior_created_by
                    current["evidence_event_ids"] = list(dict.fromkeys(
                        prior_evidence + list(candidate["evidence_event_ids"])
                    ))
                    current["_blockers"] = prior_blockers + [b for b in candidate.get("_blockers", []) if b not in prior_blockers]
                    if current["_blockers"]:
                        current["confidence"] = "hard_blocked"
                    recompute_revision_hash(current)
                else:
                    if current is not None and current["status"] == "submitted":
                        current["status"] = "rejected"
                        add_blocker(current, "a later submission boundary appeared before an approval/rejection boundary")
                        recompute_revision_hash(current)
                    revision = make_revision(
                        scope, event, len(revisions) + 1, "submitted",
                        "retouch" if scope["scope_kind"] == "retouch_requirement" else "design",
                        assets_by_task[task_id], refs_by_task[task_id], selected_assets=selected_assets,
                        extra_evidence=completion_evidence, selection_blockers=selection_blockers,
                    )
                    revisions.append(revision)
                    working = revision["revision_no"]
                current = revisions[working - 1]
                if (
                    current.get("confidence") != "hard_blocked"
                    and is_legacy_retouch_terminal_submit(
                        scope,
                        event,
                        boundary,
                        events_by_task[task_id],
                        selected_assets,
                        selection_blockers,
                    )
                ):
                    if finalized and finalized != current["revision_no"]:
                        revisions[finalized - 1]["status"] = "superseded"
                        recompute_revision_hash(revisions[finalized - 1])
                    current["status"] = "finalized"
                    current["finalized_at"] = current["submitted_at"]
                    retouch_policy = str(
                        event.get("_migration_policy")
                        or RETOUCH_TERMINAL_SUBMIT_POLICY
                    )
                    if retouch_policy == RETOUCH_UNSCOPED_ATOMIC_BATCH_POLICY:
                        current["reason"] = (
                            f"policy {retouch_policy}: the sole task-level "
                            "retouch submit follows complete allowlisted "
                            "delivery coverage for every requirement; human "
                            "confirmation remains required"
                        )
                    else:
                        current["reason"] = (
                            f"policy {retouch_policy}: current retouch submit "
                            "semantics complete the task directly; the legacy "
                            "Completed task has one scope-proven final and a "
                            "matching completed upload session; human "
                            "confirmation remains required"
                        )
                    recompute_revision_hash(current)
                    working = finalized = current["revision_no"]
            elif kind in {"approve", "close"}:
                if current is not None and current["status"] == "submitted":
                    changed = False
                    probe = dict(current)
                    probe["final_task_asset_ids"] = list(current["final_task_asset_ids"])
                    probe["reference_file_ref_ids"] = list(current["reference_file_ref_ids"])
                    probe["evidence_event_ids"] = [stable_event_id(event)]
                    probe["_blockers"] = list(current.get("_blockers", []))
                    changed = apply_explicit_audit_change(scope, event, assets_by_task[task_id], probe)
                    if not changed:
                        changed = apply_proven_successor_audit_change(
                            scope, event, events_by_task[task_id], assets_by_task[task_id], probe,
                        )
                    if not changed:
                        changed = apply_proven_legacy_audit_stage_snapshot(
                            scope,
                            event,
                            events_by_task[task_id],
                            assets_by_task[task_id],
                            probe,
                        )
                    late_assets = assets_created_between(scope, assets_by_task[task_id], current.get("submitted_at", current["created_at"]), event["created_at"])
                    if not changed and late_assets:
                        add_blocker(current, "assets changed between submission and approval but the event does not prove replace/append semantics")
                    if not changed:
                        asset_by_id = {int(asset["id"]): asset for asset in assets_by_task[task_id]}
                        finalized_members = list(current["final_task_asset_ids"])
                        if current.get("source_task_asset_id") is not None:
                            finalized_members.append(int(current["source_task_asset_id"]))
                        if current.get("source_alias_from_task_asset_id") is not None:
                            finalized_members.append(int(current["source_alias_from_task_asset_id"]))
                        for member_id in dict.fromkeys(finalized_members):
                            member = asset_by_id.get(member_id)
                            eligible_at_approval, lifecycle_reason = revision_asset_eligible(member or {}, set(), event["created_at"])
                            if not eligible_at_approval:
                                add_blocker(
                                    current,
                                    f"asset_version_id {member_id} was not eligible at approval: {lifecycle_reason or 'missing asset'}",
                                )
                    if changed:
                        current["status"] = "superseded"
                        recompute_revision_hash(current)
                        revision = make_revision(scope, event, len(revisions) + 1, "finalized", "audit", assets_by_task[task_id], refs_by_task[task_id], inherited=current)
                        revision.update({
                            "source_task_asset_id": probe.get("source_task_asset_id"),
                            "final_task_asset_ids": list(probe["final_task_asset_ids"]),
                            "reference_file_ref_ids": list(probe["reference_file_ref_ids"]),
                            "evidence_event_ids": list(dict.fromkeys(probe["evidence_event_ids"])),
                            "_blockers": list(probe.get("_blockers", [])),
                            "_review_policy_ids": list(
                                probe.get("_review_policy_ids", [])
                            ),
                        })
                        if probe.get("source_bundle_candidate") is not None:
                            revision["source_bundle_candidate"] = dict(
                                probe["source_bundle_candidate"]
                            )
                        else:
                            revision.pop("source_bundle_candidate", None)
                        if revision.get("source_task_asset_id") is None:
                            revision.pop("source_task_asset_id", None)
                        if probe.get("source_alias_from_task_asset_id") is not None:
                            revision["source_alias_from_task_asset_id"] = probe["source_alias_from_task_asset_id"]
                        else:
                            revision.pop("source_alias_from_task_asset_id", None)
                        if revision["_blockers"]:
                            revision["confidence"] = "hard_blocked"
                        if not revision["final_task_asset_ids"]:
                            add_blocker(revision, "finalized audit revision has no delivery asset")
                        recompute_revision_hash(revision)
                        revisions.append(revision)
                        if finalized and finalized != current["revision_no"]:
                            revisions[finalized - 1]["status"] = "superseded"
                            recompute_revision_hash(revisions[finalized - 1])
                        working = finalized = revision["revision_no"]
                    else:
                        if finalized and finalized != current["revision_no"]:
                            revisions[finalized - 1]["status"] = "superseded"
                            recompute_revision_hash(revisions[finalized - 1])
                        current["status"] = "finalized"
                        current["finalized_at"] = event["created_at"]
                        current["evidence_event_ids"].append(stable_event_id(event))
                        if not current["final_task_asset_ids"]:
                            add_blocker(current, "finalized revision has no delivery asset")
                        recompute_revision_hash(current)
                        working = finalized = current["revision_no"]
                elif finalized:
                    # Close after approval is evidence for the same immutable snapshot, not a new revision.
                    final_revision = revisions[finalized - 1]
                    if stable_event_id(event) not in final_revision["evidence_event_ids"]:
                        final_revision["evidence_event_ids"].append(stable_event_id(event))
                        recompute_revision_hash(final_revision)
                else:
                    if current is not None and current["status"] in {"draft", "submitted"}:
                        current["status"] = "superseded"
                        add_blocker(current, "approval/close boundary arrived before a valid submitted revision")
                        recompute_revision_hash(current)
                    revision = make_revision(scope, event, len(revisions) + 1, "finalized", "audit", assets_by_task[task_id], refs_by_task[task_id])
                    add_blocker(revision, "approval/close boundary has no preceding submitted revision")
                    recompute_revision_hash(revision)
                    revisions.append(revision)
                    working = finalized = revision["revision_no"]
            elif kind == "reject":
                if current is None:
                    current = make_revision(scope, event, len(revisions) + 1, "rejected", "audit", assets_by_task[task_id], refs_by_task[task_id])
                    add_blocker(current, "rejection boundary has no preceding submitted revision")
                    revisions.append(current)
                elif current["status"] == "finalized":
                    draft = make_revision(
                        scope, event, len(revisions) + 1, "draft", "reopen",
                        assets_by_task[task_id], refs_by_task[task_id], inherited=current,
                    )
                    add_blocker(draft, "rejection boundary arrived after a finalized revision; confirm reopen semantics")
                    recompute_revision_hash(draft)
                    revisions.append(draft)
                    working = draft["revision_no"]
                    continue
                elif current["status"] != "submitted":
                    current["status"] = "rejected"
                    current.pop("finalized_at", None)
                    for evidence_id in event_evidence_ids(event):
                        if evidence_id not in current["evidence_event_ids"]:
                            current["evidence_event_ids"].append(evidence_id)
                    add_blocker(current, "rejection boundary has no preceding submitted revision")
                    recompute_revision_hash(current)
                else:
                    current["status"] = "rejected"
                    current.pop("finalized_at", None)
                    for evidence_id in event_evidence_ids(event):
                        if evidence_id not in current["evidence_event_ids"]:
                            current["evidence_event_ids"].append(evidence_id)
                    recompute_revision_hash(current)
                draft = make_revision(scope, event, len(revisions) + 1, "draft", "reopen", assets_by_task[task_id], refs_by_task[task_id], inherited=current)
                revisions.append(draft)
                working = draft["revision_no"]
            elif kind == "reopen":
                if current is not None and current["status"] == "draft":
                    for evidence_id in event_evidence_ids(event):
                        if evidence_id not in current["evidence_event_ids"]:
                            current["evidence_event_ids"].append(evidence_id)
                    recompute_revision_hash(current)
                else:
                    inherited = revisions[finalized - 1] if finalized else current
                    draft = make_revision(scope, event, len(revisions) + 1, "draft", "reopen", assets_by_task[task_id], refs_by_task[task_id], inherited=inherited)
                    if inherited is None:
                        add_blocker(draft, "reopen boundary has no finalized or working revision to clone")
                        recompute_revision_hash(draft)
                    revisions.append(draft)
                    working = draft["revision_no"]
            elif kind == "supplement":
                inherited = revisions[finalized - 1] if finalized else current
                revision = make_revision(
                    scope,
                    event,
                    len(revisions) + 1,
                    "finalized",
                    "reopen",
                    assets_by_task[task_id],
                    refs_by_task[task_id],
                    inherited=inherited,
                )
                payload = event_payload(event)
                session_id = str(payload.get("upload_session_id") or payload.get("asset_upload_session_id") or "")
                operation = str(payload.get("final_mode") or payload.get("asset_operation") or "").lower()
                before = nested_number(payload, "before")
                after = nested_number(payload, "after")
                if before is None:
                    before = nested_number(
                        payload, "audit_delivery_count_before"
                    )
                if after is None:
                    after = nested_number(
                        payload, "audit_delivery_count_after"
                    )
                inferred_append = before is not None and after == before + 1
                changed = apply_explicit_audit_change(scope, event, assets_by_task[task_id], revision)
                if inherited is None:
                    add_blocker(revision, "audit supplement has no finalized revision to reopen")
                if not session_id:
                    add_blocker(revision, "audit supplement lacks an explicit upload_session_id")
                if operation != "append" and not inferred_append:
                    add_blocker(revision, "audit supplement must prove append via asset_operation=append or before/after +1")
                if not changed:
                    add_blocker(revision, "audit supplement does not identify any scoped source/delivery asset")
                if finalized:
                    revisions[finalized - 1]["status"] = "superseded"
                    recompute_revision_hash(revisions[finalized - 1])
                recompute_revision_hash(revision)
                revisions.append(revision)
                working = finalized = revision["revision_no"]
        working, finalized = replay_post_close_replacements(
            scope,
            events_by_task[task_id],
            assets_by_task[task_id],
            refs_by_task[task_id],
            revisions,
            working,
            finalized,
        )
        current_pointer = working or finalized
        current_snapshot_before_cleanup = (
            copy.deepcopy(revisions[current_pointer - 1])
            if current_pointer
            else None
        )
        if current_pointer:
            current_revision = revisions[current_pointer - 1]
            completion_events = [
                event
                for event in events_by_task[task_id]
                if str(event.get("event_type") or "").lower()
                in UPLOAD_SESSION_COMPLETED_EVENTS
            ]
            if completion_events:
                latest_completion = max(
                    completion_events,
                    key=lambda event: (
                        str(event.get("created_at") or ""),
                        int(event.get("sequence") or 0),
                    ),
                )
                if apply_proven_successor_audit_change_if_possible(
                    scope,
                    latest_completion,
                    events_by_task[task_id],
                    assets_by_task[task_id],
                    current_revision,
                ):
                    current_revision["reason"] = (
                        f"policy {AUDIT_STAGE_FINAL_SNAPSHOT_POLICY}: "
                        "the current submitted/finalized snapshot follows the "
                        "complete same-root successor chain proven by exact "
                        "upload-completion events"
                    )
                    current_revision.setdefault(
                        "_review_policy_ids", []
                    ).append(AUDIT_STAGE_FINAL_SNAPSHOT_POLICY)
                    recompute_revision_hash(current_revision)
        event_by_evidence_id = {
            stable_event_id(event): event
            for event in events_by_task[task_id]
        }
        post_close_replacements = direct_post_close_replacements(
            scope,
            events_by_task[task_id],
            assets_by_task[task_id],
        )

        def cleanup_event_order(event: dict[str, Any]) -> tuple[Any, ...]:
            return (
                str(event.get("created_at") or ""),
                event.get("namespace") == "task_module_event",
                int(event.get("sequence") or 0),
                str(event.get("id") or ""),
            )

        for revision in revisions:
            cleanup_event = next(
                (
                    event_by_evidence_id[evidence_id]
                    for evidence_id in revision.get("evidence_event_ids") or []
                    if evidence_id in event_by_evidence_id
                    and str(
                        event_by_evidence_id[evidence_id].get("created_at")
                        or ""
                    )
                    == str(revision.get("created_at") or "")
                    and (
                        str(
                            event_by_evidence_id[evidence_id].get(
                                "event_type"
                            )
                            or ""
                        ).lower()
                        in UPLOAD_SESSION_COMPLETED_EVENTS
                        or str(
                            event_by_evidence_id[evidence_id].get(
                                "event_type"
                            )
                            or ""
                        ).lower()
                        == "task.audit.supplement_uploaded"
                    )
                ),
                None,
            )
            if cleanup_event is not None:
                protected_member_ids = {
                    int(predecessor["id"])
                    for completion, predecessor, _ in post_close_replacements
                    if cleanup_event_order(completion)
                    > cleanup_event_order(cleanup_event)
                }
                prune_inherited_reopen_snapshot(
                    revision,
                    assets_by_task[task_id],
                    cleanup_event,
                    str(scope.get("scope_kind") or ""),
                    protected_member_ids,
                )
                if (
                    revision.get("status") == "finalized"
                    and not revision.get("final_task_asset_ids")
                ):
                    add_blocker(
                        revision,
                        "finalized revision has no lifecycle-valid delivery asset",
                    )
                if (
                    revision.get("status") in {"submitted", "finalized"}
                    and str(scope.get("scope_kind") or "")
                    != "retouch_requirement"
                    and not revision.get("source_task_asset_id")
                    and not revision.get("source_alias_from_task_asset_id")
                ):
                    add_blocker(
                        revision,
                        "design revision has no lifecycle-valid source asset",
                    )
                recompute_revision_hash(revision)
        current_pointer = working or finalized
        if current_pointer:
            reopened_missing_final = reopen_completed_customization_missing_final(
                scope,
                current_snapshot_before_cleanup,
                revisions[current_pointer - 1],
                assets_by_task[task_id],
            )
            if reopened_missing_final is not None:
                revisions[current_pointer - 1] = reopened_missing_final[
                    "historical"
                ]
                revisions.append(reopened_missing_final["draft"])
                working = len(revisions)
                finalized = None
                completed_customization_missing_final_by_task[task_id] = (
                    reopened_missing_final
                )
        ambiguous_completed_retouch = (
            scope["scope_kind"] == "retouch_requirement"
            and str(scope.get("task_status") or "") == "Completed"
            and retouch_scope_counts[task_id] > 1
            and task_id not in retouch_atomic_submit_by_task
            and task_id not in retouch_visual_scope_by_task
            and any(
                active_asset(asset)
                and str(asset.get("asset_type") or "")
                in {"source", "delivery"}
                and asset.get("retouch_requirement_id") in (None, "", 0)
                for asset in assets_by_task[task_id]
            )
        )
        if ambiguous_completed_retouch:
            for revision in revisions:
                add_blocker(
                    revision,
                    "Completed multi-requirement retouch task has unscoped "
                    "source/delivery assets without an exact reviewed "
                    "membership policy",
                )
        for revision in revisions:
            clear_rejected_members_from_reopen_draft(
                revision,
                assets_by_task[task_id],
            )
            merge_member_completion_evidence(
                revision,
                events_by_task[task_id],
            )
            revision["evidence_event_ids"] = sorted_evidence_ids(
                revision.get("evidence_event_ids") or [],
                events_by_task[task_id],
            )
            blockers = list(revision.pop("_blockers", []))
            if blockers:
                revision["blockers"] = blockers
            revision["review_policy_ids"] = revision_review_policy_ids(scope, revision)
            revision.pop("_review_policy_ids", None)
            recompute_revision_hash(revision)
            reason = "; ".join(blockers) if blockers else "confirm event-to-revision semantics and ordered asset membership"
            if revision.get("reason", "").startswith(f"policy {BATCH_SUBMIT_POLICY}:"):
                reason = revision["reason"] + "; " + reason
            if revision.get("source_alias_from_task_asset_id") is not None:
                reason = "review delivery-to-source alias identity and exact upload-session evidence; " + reason
            if scope.get("deleted_at"):
                reason = "deleted retouch requirement retained as read-only history; " + reason
            manual.append(review_row(scope, revision, revision["confidence"], reason))
        resource = {"task_id": task_id, "scope_kind": scope["scope_kind"], "scope_ref_id": int(scope["scope_ref_id"]), "history": revisions}
        if working is not None:
            resource["working_revision_no"] = working
        if finalized is not None:
            resource["finalized_revision_no"] = finalized
        resources.append(resource)
    task_state_decisions = []
    for task_id, candidate in sorted(
        customization_terminal_without_assets_by_task.items()
    ):
        evidence_event_ids = event_evidence_ids(candidate["event"])
        decision = {
            "task_id": task_id,
            "from_status": "PendingWarehouseReceive",
            "target_status": "InProgress",
            "evidence_event_ids": evidence_event_ids,
            "confidence": "proposed_review",
            "review_policy_ids": [
                CUSTOMIZATION_TERMINAL_WITHOUT_ASSETS_POLICY
            ],
            "confirmed_by": 0,
            "confirmed_at": ZERO_TIME,
            "confirmation_note": "",
        }
        recompute_mapping_row_hash(decision)
        task_state_decisions.append(decision)
        manual.append(
            {
                "task_id": task_id,
                "scope_kind": "task_state_decision",
                "scope_ref_id": 0,
                "revision_no": "",
                "confidence": "proposed_review",
                "reason": (
                    f"policy review required: "
                    f"{CUSTOMIZATION_TERMINAL_WITHOUT_ASSETS_POLICY}; map "
                    "the retired incomplete customization terminal to "
                    "InProgress without inventing a final asset"
                ),
                "evidence_event_ids": "|".join(evidence_event_ids),
                "candidate_source_ids": "",
                "candidate_final_ids": "",
                "reviewer_id": "",
                "reviewed_at": "",
                "decision": "",
                "review_note": "",
            }
        )
    for task_id, candidate in sorted(
        completed_customization_missing_final_by_task.items()
    ):
        evidence_event_ids = candidate["evidence_event_ids"]
        decision = {
            "task_id": task_id,
            "from_status": "Completed",
            "target_status": "InProgress",
            "evidence_event_ids": evidence_event_ids,
            "confidence": "proposed_review",
            "review_policy_ids": [
                CUSTOMIZATION_TERMINAL_WITHOUT_ASSETS_POLICY,
                HISTORICAL_ASSET_UNAVAILABLE_POLICY,
            ],
            "confirmed_by": 0,
            "confirmed_at": ZERO_TIME,
            "confirmation_note": "",
        }
        recompute_mapping_row_hash(decision)
        task_state_decisions.append(decision)
        manual.append(
            {
                "task_id": task_id,
                "scope_kind": "task_state_decision",
                "scope_ref_id": 0,
                "revision_no": "",
                "confidence": "proposed_review",
                "reason": (
                    f"policy review required: "
                    f"{CUSTOMIZATION_TERMINAL_WITHOUT_ASSETS_POLICY} + "
                    f"{HISTORICAL_ASSET_UNAVAILABLE_POLICY}; map "
                    "legacy Completed to InProgress because the only approved "
                    "final object is no longer available"
                ),
                "evidence_event_ids": "|".join(evidence_event_ids),
                "candidate_source_ids": "",
                "candidate_final_ids": "",
                "reviewer_id": "",
                "reviewed_at": "",
                "decision": "",
                "review_note": "",
            }
        )
    for task_id, candidate in sorted(retouch_partial_submit_by_task.items()):
        evidence_event_ids = list(
            candidate.get("_unassigned_completion_event_ids") or []
        )
        for membership in candidate["_retouch_scope_memberships"].values():
            evidence_event_ids.extend(
                membership.get("completion_event_ids") or []
            )
        evidence_event_ids.extend(event_evidence_ids(candidate))
        decision = {
            "task_id": task_id,
            "from_status": "Completed",
            "target_status": "InProgress",
            "evidence_event_ids": sorted_evidence_ids(
                list(dict.fromkeys(evidence_event_ids)),
                events_by_task[task_id],
            ),
            "confidence": "proposed_review",
            "review_policy_ids": [
                RETOUCH_PREMATURE_TERMINAL_PARTIAL_POLICY
            ],
            "confirmed_by": 0,
            "confirmed_at": ZERO_TIME,
            "confirmation_note": "",
        }
        recompute_mapping_row_hash(decision)
        task_state_decisions.append(decision)
        manual.append(
            {
                "task_id": task_id,
                "scope_kind": "task_state_decision",
                "scope_ref_id": 0,
                "revision_no": "",
                "confidence": "proposed_review",
                "reason": (
                    f"policy review required: "
                    f"{RETOUCH_PREMATURE_TERMINAL_PARTIAL_POLICY}; map "
                    "premature legacy Completed to InProgress without "
                    "inventing missing requirement finals"
                ),
                "evidence_event_ids": "|".join(
                    decision["evidence_event_ids"]
                ),
                "candidate_source_ids": "",
                "candidate_final_ids": "",
                "reviewer_id": "",
                "reviewed_at": "",
                "decision": "",
                "review_note": "",
            }
        )
    for task_id, candidate in sorted(
        retouch_unscoped_ambiguous_by_task.items()
    ):
        evidence_event_ids = list(
            candidate.get("_unassigned_completion_event_ids") or []
        )
        for membership in candidate[
            "_retouch_scope_memberships"
        ].values():
            evidence_event_ids.extend(
                membership.get("completion_event_ids") or []
            )
        evidence_event_ids.extend(event_evidence_ids(candidate))
        decision = {
            "task_id": task_id,
            "from_status": "Completed",
            "target_status": "InProgress",
            "evidence_event_ids": sorted_evidence_ids(
                list(dict.fromkeys(evidence_event_ids)),
                events_by_task[task_id],
            ),
            "confidence": "proposed_review",
            "review_policy_ids": [
                RETOUCH_PREMATURE_TERMINAL_PARTIAL_POLICY
            ],
            "confirmed_by": 0,
            "confirmed_at": ZERO_TIME,
            "confirmation_note": "",
        }
        recompute_mapping_row_hash(decision)
        task_state_decisions.append(decision)
        manual.append(
            {
                "task_id": task_id,
                "scope_kind": "task_state_decision",
                "scope_ref_id": 0,
                "revision_no": "",
                "confidence": "proposed_review",
                "reason": (
                    f"policy review required: "
                    f"{RETOUCH_PREMATURE_TERMINAL_PARTIAL_POLICY}; "
                    "the task-level terminal upload is real, but assigning "
                    "unscoped files across multiple retouch requirements "
                    "would be fabricated; preserve files and map to "
                    "InProgress"
                ),
                "evidence_event_ids": "|".join(
                    decision["evidence_event_ids"]
                ),
                "candidate_source_ids": "",
                "candidate_final_ids": "",
                "reviewer_id": "",
                "reviewed_at": "",
                "decision": "",
                "review_note": "",
            }
        )
    planning_tasks = []
    planning_images = planning_image_candidates(rows)
    planning_by_task = defaultdict(list)
    for row in rows.get("planning_rows", []):
        planning_by_task[int(row["task_id"])].append(row)
    for task_id, items in sorted(planning_by_task.items()):
        task_status = str(items[0].get("task_status") or "")
        tombstone_contract = INCOMPLETE_UAT_PLANNING_TOMBSTONES.get(task_id)
        if tombstone_contract is not None:
            actual_item_ids = tuple(
                sorted(
                    int(item["task_sku_item_id"])
                    for item in items
                    if item.get("task_sku_item_id") is not None
                )
            )
            if actual_item_ids != tombstone_contract["task_sku_item_ids"]:
                raise ValueError(
                    f"UAT planning tombstone task {task_id} expected SKU items "
                    f"{tombstone_contract['task_sku_item_ids']}, got {actual_item_ids}"
                )
            planning = {
                "task_id": task_id,
                "target_task_status": tombstone_contract["target_task_status"],
                "code_rule_revision_id": LEGACY_PLANNING_RULE_REVISION_ID,
                "created_by": int(items[0].get("creator_id") or 0),
                "confidence": "proposed_review",
                "review_policy_ids": [
                    LEGACY_PURCHASE_TO_PLANNING_POLICY,
                    INCOMPLETE_UAT_PLANNING_TOMBSTONE_POLICY,
                    FROZEN_PLANNING_RULE_POLICY,
                ],
                "confirmed_by": 0,
                "confirmed_at": ZERO_TIME,
                "confirmation_note": "",
                "items": [
                    {
                        "task_sku_item_id": actual_item_ids[0],
                        "description_spec": "",
                        "quantity": 0,
                        "target_price": None,
                        "note": "",
                        "reference_url": "",
                        "erp_product_i_id": "",
                        "erp_product_name": "",
                        "image_storage_ref_id": "",
                    }
                ],
            }
            recompute_planning_hash(planning)
            planning_tasks.append(planning)
            manual.append(
                {
                    "task_id": task_id,
                    "scope_kind": "planning",
                    "scope_ref_id": 0,
                    "revision_no": "",
                    "confidence": "proposed_review",
                    "reason": (
                        f"policy review required: {INCOMPLETE_UAT_PLANNING_TOMBSTONE_POLICY}; "
                        "preserve the existing SKU identity but create no fabricated "
                        "planning detail or revision"
                    ),
                    "evidence_event_ids": "",
                    "candidate_source_ids": "",
                    "candidate_final_ids": "",
                    "reviewer_id": "",
                    "reviewed_at": "",
                    "decision": "",
                    "review_note": "",
                }
            )
            continue
        target_status = LEGACY_PLANNING_STATUS_MAP.get(
            task_status,
            task_status if task_status in CURRENT_PLANNING_STATUSES else "",
        )
        mapped_items = []
        # Planning images are optional in the V8 persistence contract.  An
        # empty image_storage_ref_id is therefore not a review blocker.
        #
        # Revision 9 is the frozen snapshot's sole immutable sku_planning rule
        # revision.  It is disabled for new allocation, but these rows already
        # own their SKU identities; binding them to revision 9 is therefore a
        # policy-review candidate rather than missing migration truth.
        blockers = []
        planning_policy_ids = [
            LEGACY_PURCHASE_TO_PLANNING_POLICY,
            FROZEN_PLANNING_RULE_POLICY,
        ]
        policy_notes = [
            "policy review required: bind existing SKU identities to frozen sku_planning code_rule_revision_id 9",
        ]
        if task_status in LEGACY_PLANNING_STATUS_MAP:
            planning_policy_ids.append(RETIRED_PLANNING_STATUS_POLICY)
            policy_notes.append(
                f"policy review required: map retired task_status {task_status} to {target_status}"
            )
        for item in sorted(items, key=lambda row: int(row.get("task_sku_item_id") or 0)):
            if item.get("task_sku_item_id") is None:
                blockers.append("purchase task has no task_sku_items")
                continue
            sku_code = str(item.get("sku_code") or "").strip(" ")
            image_candidates = planning_images.get((task_id, sku_code), [])
            image_storage_ref_id = ""
            if len(image_candidates) == 1:
                image_storage_ref_id = str(image_candidates[0]["storage_ref_id"])
            elif len(image_candidates) > 1:
                blockers.append(
                    f"SKU item {item['task_sku_item_id']} has multiple active "
                    "erp_product_image candidates: "
                    + ",".join(str(candidate["id"]) for candidate in image_candidates)
                )
            description = str(item.get("description_spec") or "").strip()
            if not description:
                description = str(item.get("erp_product_name") or "").strip()
                if description:
                    planning_policy_ids.append(PRODUCT_NAME_DESCRIPTION_FALLBACK_POLICY)
                    policy_notes.append(
                        f"policy review required: SKU item {item['task_sku_item_id']} uses task_sku_items.product_name_snapshot as description_spec"
                    )
            quantity = int(item.get("quantity") or 0)
            if not description:
                blockers.append(
                    f"SKU item {item['task_sku_item_id']} lacks both description_spec and ERP product_name snapshot"
                )
            if quantity <= 0:
                blockers.append(f"SKU item {item['task_sku_item_id']} lacks positive quantity")
            mapped_items.append({
                "task_sku_item_id": int(item["task_sku_item_id"]),
                "description_spec": description,
                "quantity": quantity,
                "target_price": str(item["target_price"]) if item.get("target_price") is not None else None,
                "note": "",
                "reference_url": "",
                "erp_product_i_id": str(item.get("erp_product_i_id") or ""),
                "erp_product_name": str(item.get("erp_product_name") or ""),
                "image_storage_ref_id": image_storage_ref_id,
            })
        if not target_status:
            blockers.append(f"legacy task_status {task_status or '<empty>'} requires an explicit V8 target")
        created_by = int(items[0].get("creator_id") or 0)
        if not created_by:
            blockers.append("created_by cannot be proven from task creator")
        blockers = list(dict.fromkeys(blockers))
        policy_notes = list(dict.fromkeys(policy_notes))
        confidence = "hard_blocked" if blockers else "proposed_review"
        review_reasons = policy_notes + blockers
        planning = {
            "task_id": task_id,
            "target_task_status": target_status,
            "code_rule_revision_id": LEGACY_PLANNING_RULE_REVISION_ID,
            "created_by": created_by,
            "confidence": confidence,
            "review_policy_ids": ordered_review_policy_ids(planning_policy_ids),
            "confirmed_by": 0,
            "confirmed_at": ZERO_TIME,
            "confirmation_note": "",
            "items": mapped_items,
            **({"blockers": blockers} if blockers else {}),
        }
        recompute_planning_hash(planning)
        planning_tasks.append(planning)
        manual.append({"task_id": task_id, "scope_kind": "planning", "scope_ref_id": 0, "revision_no": "", "confidence": confidence, "reason": "; ".join(review_reasons), "evidence_event_ids": "", "candidate_source_ids": "", "candidate_final_ids": "", "reviewer_id": "", "reviewed_at": "", "decision": "", "review_note": ""})
    for task in rows.get("warehouse_blockers", []):
        task_id = int(task["task_id"])
        warehouse_events = [
            event
            for event in events_by_task[task_id]
            if str(event.get("event_type") or "") == "task.warehouse.rejected"
        ]
        task_resources = [
            resource for resource in resources if int(resource["task_id"]) == task_id
        ]
        reopen_complete = bool(task_resources) and all(
            resource.get("working_revision_no") is not None
            and any(
                revision["revision_no"] == resource["working_revision_no"]
                and revision["status"] == "draft"
                and revision["source_stage"] == "reopen"
                for revision in resource["history"]
            )
            for resource in task_resources
        )
        if task_id == 2455 and len(warehouse_events) == 1 and reopen_complete:
            decision = {
                "task_id": task_id,
                "from_status": "RejectedByWarehouse",
                "target_status": "InProgress",
                "evidence_event_ids": [stable_event_id(warehouse_events[0])],
                "confidence": "proposed_review",
                "review_policy_ids": [WAREHOUSE_REOPEN_STATE_POLICY],
                "confirmed_by": 0,
                "confirmed_at": ZERO_TIME,
                "confirmation_note": "",
            }
            recompute_mapping_row_hash(decision)
            task_state_decisions.append(decision)
            confidence = "proposed_review"
            reason = (
                f"policy review required: {WAREHOUSE_REOPEN_STATE_POLICY}; "
                "the exact warehouse rejection has a complete reopen draft "
                "for every resource scope"
            )
            evidence = decision["evidence_event_ids"][0]
        else:
            confidence = "hard_blocked"
            reason = (
                "RejectedByWarehouse requires an exact warehouse event and "
                "a working reopen draft for every resource scope"
            )
            evidence = "|".join(stable_event_id(event) for event in warehouse_events)
        manual.append({"task_id": task_id, "scope_kind": "task_state_decision", "scope_ref_id": 0, "revision_no": "", "confidence": confidence, "reason": reason, "evidence_event_ids": evidence, "candidate_source_ids": "", "candidate_final_ids": "", "reviewer_id": "", "reviewed_at": "", "decision": "", "review_note": ""})
    organization_mappings, organization_manual = build_organization_mappings(rows)
    access_decisions, access_manual = build_access_decisions(
        rows, organization_mappings
    )
    asset_recoveries, asset_recovery_manual = build_deleted_asset_recoveries(
        rows
    )
    manual.extend(organization_manual)
    manual.extend(access_manual)
    manual.extend(asset_recovery_manual)
    return {
        "version": 2,
        "resources": resources,
        "planning_tasks": planning_tasks,
        "task_state_decisions": task_state_decisions,
        "asset_recoveries": asset_recoveries,
        "organization_mappings": organization_mappings,
        "access_decisions": access_decisions,
    }, manual, build_object_manifest(rows)


SQL = r"""
SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
SET SESSION TRANSACTION READ ONLY;
START TRANSACTION WITH CONSISTENT SNAPSHOT, READ ONLY;
SELECT CONCAT('timezone_truth\t',JSON_OBJECT(
 'system_time_zone',@@system_time_zone,
 'matched_asset_created_events',created_events.matched_count,
 'near_eight_hour_asset_created_events',created_events.near_eight_hour_count,
 'asset_created_min_delta_seconds',created_events.min_delta_seconds,
 'asset_created_max_delta_seconds',created_events.max_delta_seconds,
 'superseded_pairs',superseded_pairs.matched_count,
 'near_eight_hour_superseded_pairs',superseded_pairs.near_eight_hour_count,
 'superseded_min_delta_seconds',superseded_pairs.min_delta_seconds,
 'superseded_max_delta_seconds',superseded_pairs.max_delta_seconds,
 'rejected_at_count',(SELECT COUNT(*) FROM task_assets WHERE rejected_at IS NOT NULL),
 'deleted_at_count',(SELECT COUNT(*) FROM task_assets WHERE deleted_at IS NOT NULL)
))
FROM (
 SELECT COUNT(*) matched_count,
        SUM(TIMESTAMPDIFF(SECOND,ta.created_at,e.created_at) BETWEEN 28790 AND 28810) near_eight_hour_count,
        MIN(TIMESTAMPDIFF(SECOND,ta.created_at,e.created_at)) min_delta_seconds,
        MAX(TIMESTAMPDIFF(SECOND,ta.created_at,e.created_at)) max_delta_seconds
 FROM task_event_logs e
 JOIN task_assets ta ON ta.id=CAST(JSON_UNQUOTE(JSON_EXTRACT(e.payload,'$.asset_version_id')) AS UNSIGNED)
 WHERE e.event_type='task.asset.version.created'
) created_events
CROSS JOIN (
 SELECT COUNT(*) matched_count,
        SUM(TIMESTAMPDIFF(SECOND,successor.created_at,old_asset.superseded_at) BETWEEN 28797 AND 28803) near_eight_hour_count,
        MIN(TIMESTAMPDIFF(SECOND,successor.created_at,old_asset.superseded_at)) min_delta_seconds,
        MAX(TIMESTAMPDIFF(SECOND,successor.created_at,old_asset.superseded_at)) max_delta_seconds
 FROM task_assets old_asset
 JOIN task_assets successor ON successor.id=old_asset.superseded_by_version_id
 WHERE old_asset.superseded_at IS NOT NULL
) superseded_pairs;
SELECT CONCAT('planning_rule_truth\t',JSON_OBJECT(
 'rule_id',r.id,
 'rule_type',r.rule_type,
 'is_enabled',r.is_enabled,
 'active_revision_id',r.active_revision_id,
 'revision_id',rr.id,
 'version_no',rr.version_no
))
FROM code_rules r
JOIN code_rule_revisions rr ON rr.id=r.active_revision_id AND rr.rule_id=r.id
WHERE r.rule_type='sku_planning'
ORDER BY r.id;
SELECT CONCAT('scopes\t',JSON_OBJECT('task_id',task_id,'task_status',task_status,'scope_kind',scope_kind,'scope_ref_id',scope_ref_id,'sku_code',sku_code,'deleted_at',deleted_at)) FROM (
 SELECT t.id task_id,t.task_status,'retouch_requirement' scope_kind,trr.id scope_ref_id,'' sku_code,DATE_FORMAT(trr.deleted_at,'%Y-%m-%dT%H:%i:%sZ') deleted_at FROM tasks t JOIN task_retouch_requirements trr ON trr.task_id=t.id WHERE t.task_type='retouch_task'
 UNION ALL SELECT t.id,t.task_status,'sku',tsi.id,tsi.sku_code,NULL FROM tasks t JOIN task_sku_items tsi ON tsi.task_id=t.id WHERE t.task_type NOT IN ('retouch_task','purchase_task','sku_planning')
 UNION ALL SELECT t.id,t.task_status,'task',0,'',NULL FROM tasks t WHERE t.task_type NOT IN ('retouch_task','purchase_task','sku_planning') AND NOT EXISTS(SELECT 1 FROM task_sku_items si WHERE si.task_id=t.id)
) s ORDER BY task_id,scope_kind,scope_ref_id;
SELECT CONCAT('assets\t',JSON_OBJECT('id',ta.id,'asset_id',ta.asset_id,'task_id',ta.task_id,'asset_type',ta.asset_type,'scope_sku_code',COALESCE(ta.scope_sku_code,''),'retouch_requirement_id',ta.retouch_requirement_id,'upload_session_id',COALESCE(ta.upload_session_id,''),'upload_request_id',COALESCE(ta.upload_request_id,''),'source_module_key',COALESCE(ta.source_module_key,''),'source_asset_version_id',ta.source_asset_version_id,'flow_review_status',COALESCE(ta.flow_review_status,''),'approved_by',ta.approved_by,'approved_at',DATE_FORMAT(DATE_SUB(ta.approved_at,INTERVAL 8 HOUR),'%Y-%m-%dT%H:%i:%sZ'),'rejected_at',DATE_FORMAT(DATE_SUB(ta.rejected_at,INTERVAL 8 HOUR),'%Y-%m-%dT%H:%i:%sZ'),'superseded_by_version_id',ta.superseded_by_version_id,'superseded_at',DATE_FORMAT(DATE_SUB(ta.superseded_at,INTERVAL 8 HOUR),'%Y-%m-%dT%H:%i:%sZ'),'storage_ref_id',COALESCE(ta.storage_ref_id,''),'storage_key',COALESCE(ta.storage_key,''),'file_size',ta.file_size,'mime_type',COALESCE(ta.mime_type,''),'whole_hash',COALESCE(ta.whole_hash,''),'upload_status',COALESCE(ta.upload_status,''),'is_archived',COALESCE(ta.is_archived,0),'uploaded_at_legacy_raw',DATE_FORMAT(ta.uploaded_at,'%Y-%m-%d %H:%i:%s'),'created_at',DATE_FORMAT(ta.created_at,'%Y-%m-%dT%H:%i:%sZ'),'deleted_at',DATE_FORMAT(DATE_SUB(ta.deleted_at,INTERVAL 8 HOUR),'%Y-%m-%dT%H:%i:%sZ'),'cleaned_at',DATE_FORMAT(ta.cleaned_at,'%Y-%m-%dT%H:%i:%sZ'),'access_revoked_at',DATE_FORMAT(ta.access_revoked_at,'%Y-%m-%dT%H:%i:%sZ'),'object_deleted_at',DATE_FORMAT(ta.object_deleted_at,'%Y-%m-%dT%H:%i:%sZ'),'storage_owner_type',COALESCE(sr.owner_type,''),'storage_owner_id',sr.owner_id,'storage_adapter',COALESCE(sr.storage_adapter,''),'ref_key',COALESCE(sr.ref_key,''),'checksum_hint',COALESCE(sr.checksum_hint,''),'storage_status',COALESCE(sr.status,''),'is_placeholder',COALESCE(sr.is_placeholder,0))) FROM task_assets ta LEFT JOIN asset_storage_refs sr ON sr.ref_id=ta.storage_ref_id ORDER BY ta.task_id,ta.created_at,ta.id;
SELECT CONCAT('references\t',JSON_OBJECT('id',r.id,'task_id',r.task_id,'scope_sku_code',COALESCE(si.sku_code,''),'retouch_requirement_id',r.retouch_requirement_id,'attached_at',DATE_FORMAT(r.attached_at,'%Y-%m-%dT%H:%i:%sZ'),'ref_id',r.ref_id,'storage_adapter',COALESCE(sr.storage_adapter,''),'ref_key',COALESCE(sr.ref_key,''),'file_size',sr.file_size,'mime_type',COALESCE(sr.mime_type,''),'checksum_hint',COALESCE(sr.checksum_hint,''),'storage_status',COALESCE(sr.status,''),'is_placeholder',COALESCE(sr.is_placeholder,0))) FROM reference_file_refs r LEFT JOIN task_sku_items si ON si.id=r.sku_item_id LEFT JOIN asset_storage_refs sr ON sr.ref_id=r.ref_id ORDER BY r.task_id,r.attached_at,r.id;
SELECT CONCAT('events\t',JSON_OBJECT('namespace','task_event_log','id',e.id,'task_id',e.task_id,'sequence',e.sequence,'event_type',e.event_type,'actor_id',e.operator_id,'payload',e.payload,'module_key',COALESCE(JSON_UNQUOTE(JSON_EXTRACT(e.payload,'$.module_key')),''),'from_state',COALESCE(JSON_UNQUOTE(JSON_EXTRACT(e.payload,'$.from')),''),'to_state',COALESCE(JSON_UNQUOTE(JSON_EXTRACT(e.payload,'$.to')),''),'created_at',DATE_FORMAT(DATE_SUB(e.created_at,INTERVAL 8 HOUR),'%Y-%m-%dT%H:%i:%sZ'))) FROM task_event_logs e UNION ALL SELECT CONCAT('events\t',JSON_OBJECT('namespace','task_module_event','id',e.id,'task_id',m.task_id,'sequence',e.id,'event_type',e.event_type,'actor_id',e.actor_id,'payload',e.payload,'module_key',m.module_key,'from_state',COALESCE(e.from_state,''),'to_state',COALESCE(e.to_state,''),'created_at',DATE_FORMAT(e.created_at,'%Y-%m-%dT%H:%i:%sZ'))) FROM task_module_events e JOIN task_modules m ON m.id=e.task_module_id;
SELECT CONCAT('planning_rows\t',JSON_OBJECT('task_id',t.id,'task_status',t.task_status,'creator_id',t.creator_id,'task_sku_item_id',si.id,'sku_code',COALESCE(si.sku_code,''),'description_spec',COALESCE(si.design_requirement,''),'quantity',si.quantity,'target_price',si.base_sale_price,'erp_product_i_id',COALESCE(si.product_i_id,''),'erp_product_name',COALESCE(si.product_name_snapshot,''),'reference_file_refs_json',COALESCE(si.reference_file_refs_json,'[]'))) FROM tasks t LEFT JOIN task_sku_items si ON si.task_id=t.id WHERE t.task_type='purchase_task' ORDER BY t.id,si.id;
SELECT CONCAT('warehouse_blockers\t',JSON_OBJECT('task_id',id)) FROM tasks WHERE task_status='RejectedByWarehouse' ORDER BY id;
SELECT CONCAT('org_departments\t',JSON_OBJECT('id',id,'name',name,'enabled',enabled)) FROM org_departments ORDER BY id;
SELECT CONCAT('org_teams\t',JSON_OBJECT('id',id,'department_id',department_id,'name',name,'enabled',enabled)) FROM org_teams ORDER BY id;
SELECT CONCAT('users_org\t',JSON_OBJECT('id',id,'status',status,'legacy_department',COALESCE(department,''),'department_id',department_id,'legacy_team',COALESCE(team,''),'team_id',team_id)) FROM users ORDER BY id;
SELECT CONCAT('tasks_org\t',JSON_OBJECT('id',id,'legacy_department',COALESCE(owner_department,''),'department_id',owner_department_id,'legacy_team',COALESCE(NULLIF(TRIM(owner_org_team),''),NULLIF(TRIM(owner_team),''),''),'team_id',owner_team_id)) FROM tasks ORDER BY id;
SELECT CONCAT('legacy_roles\t',JSON_OBJECT('user_id',user_id,'role',role)) FROM user_roles ORDER BY user_id,role;
SELECT CONCAT('access_assignments\t',JSON_OBJECT('user_id',a.user_id,'role_code',r.code,'scope_mode',a.scope_mode,'source_type',a.source_type,'source_ref_id',a.source_ref_id)) FROM auth_user_role_assignments a JOIN auth_roles r ON r.id=a.role_id ORDER BY a.user_id,r.code,a.scope_mode,a.source_type,a.source_ref_id;
COMMIT;
"""


def validate_legacy_timezone_truth(result):
    rows = result.get("timezone_truth") or []
    if len(rows) != 1:
        raise RuntimeError("clone must emit exactly one legacy timezone truth row")
    truth = rows[0]
    if str(truth.get("system_time_zone") or "").upper() != "UTC":
        raise RuntimeError("clone MySQL system_time_zone must be UTC for legacy timestamp normalization")
    # The task asset row and its immutable event are committed by adjacent
    # statements, not by one database timestamp expression. Production request
    # latency can therefore add a few seconds while both values still prove the
    # same legacy UTC+8 wall-clock cohort. Keep the bound deliberately tight:
    # ten seconds accepts the observed transaction latency but still rejects a
    # mixed timezone population.
    for prefix, tolerance in (("asset_created", (28790, 28810)), ("superseded", (28790, 28810))):
        count_key = "matched_asset_created_events" if prefix == "asset_created" else "superseded_pairs"
        near_key = "near_eight_hour_asset_created_events" if prefix == "asset_created" else "near_eight_hour_superseded_pairs"
        matched = int(truth.get(count_key) or 0)
        near = int(truth.get(near_key) or 0)
        minimum = int(truth.get(f"{prefix}_min_delta_seconds") or 0)
        maximum = int(truth.get(f"{prefix}_max_delta_seconds") or 0)
        if matched <= 0 or near != matched or not (tolerance[0] <= minimum <= maximum <= tolerance[1]):
            raise RuntimeError(
                f"legacy {prefix} timestamps are not one uniform proven UTC+8 wall-clock cohort; "
                "refusing blanket normalization"
            )
    return truth


def validate_planning_rule_truth(result):
    rows = result.get("planning_rule_truth") or []
    if len(rows) != 1:
        raise RuntimeError("clone must contain exactly one active sku_planning rule revision")
    truth = rows[0]
    expected = {
        "rule_type": "sku_planning",
        "active_revision_id": LEGACY_PLANNING_RULE_REVISION_ID,
        "revision_id": LEGACY_PLANNING_RULE_REVISION_ID,
        "version_no": 1,
        "is_enabled": 0,
    }
    for key, value in expected.items():
        actual = str(truth.get(key) or "") if isinstance(value, str) else int(truth.get(key) or 0)
        if actual != value:
            raise RuntimeError(
                f"clone sku_planning rule truth drifted at {key}; refusing frozen revision 9 mapping"
            )
    if int(truth.get("rule_id") or 0) <= 0:
        raise RuntimeError("clone sku_planning rule_id is invalid")
    return truth


def load_clone(args):
    if not is_loopback(args.host):
        raise ValueError("--host must be loopback; only a local frozen clone is allowed")
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
    completed = subprocess.run(command, input=SQL, text=True, capture_output=True, check=False)
    if completed.returncode:
        raise RuntimeError(f"mysql read-only manifest query failed: {completed.stderr.strip()}")
    result = {
        name: []
        for name in (
            "timezone_truth",
            "planning_rule_truth",
            "scopes",
            "assets",
            "references",
            "events",
            "planning_rows",
            "warehouse_blockers",
            "org_departments",
            "org_teams",
            "users_org",
            "tasks_org",
            "legacy_roles",
            "access_assignments",
        )
    }
    for line_no, line in enumerate(completed.stdout.splitlines(), 1):
        if not line.strip():
            continue
        try:
            tag, payload = line.split("\t", 1)
            result[tag].append(json.loads(payload))
        except (ValueError, KeyError, json.JSONDecodeError) as exc:
            raise RuntimeError(f"unexpected mysql output at line {line_no}") from exc
    if not result["scopes"]:
        raise RuntimeError("clone produced no resource scopes; refusing empty output")
    validate_legacy_timezone_truth(result)
    validate_planning_rule_truth(result)
    return result


def write_outputs(output, mapping, manual, objects, timezone_truth=None, planning_rule_truth=None):
    output.mkdir(parents=True, exist_ok=False)
    (output / "migration_mapping_v2.candidate.json").write_text(json.dumps(mapping, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    with (output / "manual_review.csv").open("w", newline="", encoding="utf-8") as handle:
        fieldnames = [
            "task_id", "scope_kind", "scope_ref_id", "revision_no", "confidence", "reason",
            "evidence_event_ids", "candidate_source_ids", "candidate_final_ids", "reviewer_id",
            "reviewed_at", "decision", "review_note",
        ]
        writer = csv.DictWriter(handle, fieldnames=fieldnames)
        writer.writeheader(); writer.writerows(manual)
    with (output / "object_manifest.jsonl").open("w", encoding="utf-8") as handle:
        for row in objects:
            handle.write(canonical_json(row) + "\n")
    output_names = ["migration_mapping_v2.candidate.json", "manual_review.csv", "object_manifest.jsonl"]
    if timezone_truth is not None:
        (output / "timezone_truth.json").write_text(
            json.dumps(timezone_truth, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
        output_names.append("timezone_truth.json")
    if planning_rule_truth is not None:
        (output / "planning_rule_truth.json").write_text(
            json.dumps(planning_rule_truth, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
        output_names.append("planning_rule_truth.json")
    hashes = {name: hashlib.sha256((output / name).read_bytes()).hexdigest() for name in output_names}
    (output / "manifest_hashes.json").write_text(json.dumps(hashes, indent=2) + "\n", encoding="utf-8")


def parse_args(argv: Iterable[str] | None = None):
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="127.0.0.1"); parser.add_argument("--port", type=int, default=3306)
    parser.add_argument("--user", required=True); parser.add_argument("--database", required=True)
    parser.add_argument("--defaults-extra-file"); parser.add_argument("--mysql", default="mysql")
    parser.add_argument("--output-dir", required=True)
    return parser.parse_args(argv)


def main():
    args = parse_args()
    try:
        rows = load_clone(args); mapping, manual, objects = generate(rows)
        write_outputs(
            pathlib.Path(args.output_dir), mapping, manual, objects,
            rows["timezone_truth"][0], rows["planning_rule_truth"][0],
        )
    except Exception as exc:
        print(str(exc), file=sys.stderr); return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
