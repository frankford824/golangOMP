#!/usr/bin/env python3
"""Generate conservative V8 migration candidates from a frozen local clone."""

from __future__ import annotations

import argparse
import csv
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
EXPLICIT_EVENT_REPLAY_POLICY = "explicit_event_replay"
DELIVERY_SOURCE_ALIAS_POLICY = "delivery_source_alias"
REJECTED_HISTORY_POLICY = "rejected_history"
REOPEN_POLICY = "reopen"
POST_CLOSE_REPLACEMENT_POLICY = "legacy_post_close_replacement_v1"
RETOUCH_SOURCE_OPTIONAL_POLICY = "retouch_source_optional"
RETOUCH_TERMINAL_SUBMIT_POLICY = "legacy_retouch_terminal_submit_v1"
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
REVIEW_POLICY_ORDER = (
    EXPLICIT_EVENT_REPLAY_POLICY,
    DELIVERY_SOURCE_ALIAS_POLICY,
    REJECTED_HISTORY_POLICY,
    REOPEN_POLICY,
    POST_CLOSE_REPLACEMENT_POLICY,
    RETOUCH_SOURCE_OPTIONAL_POLICY,
    RETOUCH_TERMINAL_SUBMIT_POLICY,
    BATCH_SUBMIT_POLICY,
    LEGACY_PURCHASE_TO_PLANNING_POLICY,
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
    if revision.get("reason", "").startswith(f"policy {BATCH_SUBMIT_POLICY}:"):
        policies.append(BATCH_SUBMIT_POLICY)
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
        final_ids = [int(a["id"]) for a in finals]
        reference_ids = [int(r["id"]) for r in scoped_references(scope, references, event["created_at"])]
    policy = str(event.get("_migration_policy") or "").strip()
    reason = "candidate reconstructed from explicit legacy workflow boundaries; human confirmation remains required"
    if policy:
        reason = (
            f"policy {policy}: the last scoped submit triggers the task-level "
            "atomic transition after independently proven full SKU coverage; "
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
        add_blocker(revision, "multiple source assets require a reviewed deterministic ZIP bundle")
    for blocker in selection_blockers or []:
        add_blocker(revision, blocker)
    if (
        inherited is None
        and scope["scope_kind"] != "retouch_requirement"
        and not revision.get("source_task_asset_id")
        and len(final_ids) == 1
    ):
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
    batch_memberships = submit_event.get("_batch_scope_memberships")
    if isinstance(batch_memberships, dict):
        membership = batch_memberships.get(str(scope.get("sku_code") or ""))
        if not isinstance(membership, dict):
            return [], [], [f"{BATCH_SUBMIT_POLICY}: no reviewed membership exists for this SKU scope"]
        asset_id = int(membership["asset_version_id"])
        asset = next((candidate for candidate in assets if int(candidate["id"]) == asset_id), None)
        if asset is None:
            return [], [], [f"{BATCH_SUBMIT_POLICY}: asset_version_id {asset_id} no longer resolves"]
        return [asset], [str(membership["completion_event_id"])], []

    payload = event_payload(submit_event)
    session_id = str(payload.get("upload_session_id") or "").strip()
    blockers: list[str] = []
    if not session_id:
        return [], [], ["design submission lacks an explicit upload_session_id"]
    completions = []
    for event in all_events:
        if str(event.get("event_type") or "").lower() not in UPLOAD_SESSION_COMPLETED_EVENTS:
            continue
        if event.get("created_at", "") > submit_event["created_at"]:
            continue
        completion_payload = event_payload(event)
        if str(completion_payload.get("upload_session_id") or "").strip() == session_id:
            completions.append(event)
    if not completions:
        return [], [], [f"upload session {session_id} has no completed event before submission"]
    by_version_id = {int(asset["id"]): asset for asset in assets}
    expected_modules = expected_submit_modules(scope, submit_event)
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
            if not at_or_before(asset, submit_event["created_at"]):
                blockers.append(f"asset_version_id {version_id} was created after the submission boundary")
                continue
            eligible, reason = revision_asset_eligible(asset, expected_modules, submit_event["created_at"])
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
    if len(selected_assets) != 1 or len(final_assets) != 1:
        return False
    final_asset = final_assets[0]
    requirement_id = int(final_asset.get("retouch_requirement_id") or 0)
    if requirement_id:
        if requirement_id != int(scope["scope_ref_id"]):
            return False
    elif not scope.get("_single_requirement"):
        # Legacy retouch delivery rows omitted requirement scope. That omission
        # is unambiguous only when the task has exactly one requirement.
        return False

    submit_payload = event_payload(submit_event)
    session_id = str(submit_payload.get("upload_session_id") or "").strip()
    asset_session_id = str(
        final_asset.get("upload_session_id")
        or final_asset.get("upload_request_id")
        or ""
    ).strip()
    if not session_id or asset_session_id != session_id:
        return False

    completions = []
    for event in all_events:
        if (
            str(event.get("event_type") or "").lower()
            not in UPLOAD_SESSION_COMPLETED_EVENTS
            or str(event.get("created_at") or "")
            > str(submit_event.get("created_at") or "")
        ):
            continue
        payload = event_payload(event)
        if str(payload.get("upload_session_id") or "").strip() == session_id:
            completions.append(event)
    if len(completions) != 1:
        return False
    completion = completions[0]
    if payload_asset_version_ids(event_payload(completion)) != [int(final_asset["id"])]:
        return False
    if int(completion.get("actor_id") or 0) != int(submit_event["actor_id"]):
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
    before, after = nested_number(payload, "before"), nested_number(payload, "after")
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
    """Apply an immutable one-hop version replacement proven by legacy links.

    Some reviewer uploads do not repeat replace/append semantics in the
    approval payload, but task_assets.superseded_by_version_id is an explicit
    version-root edge.  It is usable only when the successor is same-scope,
    same-role, present before approval, and has exactly one completed upload
    session membership event before approval.
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
        successor_id = int((old or {}).get("superseded_by_version_id") or 0)
        successor = asset_by_id.get(successor_id)
        if not old or not successor:
            add_blocker(revision, f"asset_version_id {old_id} is ineligible at approval without a resolvable successor")
            return False
        same_root = not old.get("asset_id") or not successor.get("asset_id") or int(old["asset_id"]) == int(successor["asset_id"])
        successor_ok, successor_reason = revision_asset_eligible(successor, {"audit", "customization"}, event["created_at"])
        if (
            int(successor["task_id"]) != int(scope["task_id"])
            or not scope_matches(scope, successor)
            or successor.get("asset_type") != old.get("asset_type")
            or not same_root
            or not at_or_before(successor, event["created_at"])
            or not successor_ok
        ):
            add_blocker(
                revision,
                f"asset_version_id {old_id} successor {successor_id} is not a same-root/scope/role approval-time replacement: {successor_reason}",
            )
            return False
        completions = [
            candidate for candidate in events
            if str(candidate.get("event_type") or "").lower() in UPLOAD_SESSION_COMPLETED_EVENTS
            and candidate.get("created_at", "") <= event["created_at"]
            and successor_id in payload_asset_version_ids(event_payload(candidate))
        ]
        if len(completions) != 1:
            add_blocker(
                revision,
                f"asset_version_id {old_id} successor {successor_id} has {len(completions)} completed upload membership events before approval",
            )
            return False
        replacements[old_id] = successor_id
        completion_evidence.extend(event_evidence_ids(completions[0]))
    if not replacements:
        return False
    if revision.get("source_task_asset_id") in replacements:
        revision["source_task_asset_id"] = replacements[int(revision["source_task_asset_id"])]
    if revision.get("source_alias_from_task_asset_id") in replacements:
        revision["source_alias_from_task_asset_id"] = replacements[int(revision["source_alias_from_task_asset_id"])]
    revision["final_task_asset_ids"] = [replacements.get(int(asset_id), int(asset_id)) for asset_id in revision["final_task_asset_ids"]]
    revision["evidence_event_ids"] = list(dict.fromkeys(
        list(revision.get("evidence_event_ids") or []) + completion_evidence
    ))
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


def replay_post_close_replacements(
    scope, events, assets, references, revisions, working, finalized
):
    asset_by_id = {int(asset["id"]): asset for asset in assets}
    for completion, predecessor, successor in direct_post_close_replacements(
        scope, events, assets
    ):
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
            blocker = (
                f"post-close successor {successor['id']} asset root is absent "
                "from the inherited snapshot"
            )
        else:
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


def resolve_atomic_multi_sku_batch_submit(scopes, events, assets):
    """Prove the legacy one-submit/many-SKU upload transaction shape.

    The legacy batch UI completed one explicitly SKU-scoped delivery session
    per SKU and then advanced the task once.  The single submit event only
    repeated the final upload session, so it cannot be copied to other SKU
    scopes unless the complete task-wide membership is independently proven.
    This detector deliberately returns no candidate on any ambiguity.
    """
    sku_scopes = [scope for scope in scopes if scope.get("scope_kind") == "sku"]
    if len(sku_scopes) <= 1 or len(sku_scopes) != len(scopes):
        return None
    if any(str(scope.get("task_status") or "") != "PendingAuditA" for scope in sku_scopes):
        return None
    sku_codes = [str(scope.get("sku_code") or "").strip() for scope in sku_scopes]
    if any(not code for code in sku_codes) or len(set(sku_codes)) != len(sku_codes):
        return None

    # One task state boundary only. Exact dual-write duplicates are collapsed,
    # but a second submit/approve/reject/reopen/close/supplement is a conflict.
    boundaries, seen = [], set()
    for event in sorted(
        (dict(event) for event in events if event_kind(event)),
        key=lambda event: (
            event.get("created_at", ""),
            event.get("namespace") == "task_module_event",
            str(event.get("id") or ""),
        ),
    ):
        key = event_dedup_key(event)
        if key in seen:
            continue
        seen.add(key)
        boundaries.append(event)
    if len(boundaries) != 1 or event_kind(boundaries[0]) != "submit":
        return None
    submit = boundaries[0]
    if submit.get("namespace") != "task_event_log" or not int(submit.get("actor_id") or 0):
        return None
    submit_payload = event_payload(submit)
    submit_session = str(submit_payload.get("upload_session_id") or "").strip()
    submit_sku = str(submit_payload.get("target_sku_code") or "").strip()
    submit_root_ids = payload_ids(submit_payload, "asset_id")
    if (
        not submit_session
        or submit_sku not in set(sku_codes)
        or str(submit_payload.get("asset_type") or "").strip().lower() != "delivery"
        or len(submit_root_ids) != 1
    ):
        return None

    asset_by_id = {int(asset["id"]): asset for asset in assets}
    memberships: dict[str, list[dict[str, Any]]] = defaultdict(list)
    seen_version_ids, seen_sessions = set(), set()
    for event in events:
        if str(event.get("event_type") or "").lower() not in UPLOAD_SESSION_COMPLETED_EVENTS:
            continue
        if str(event.get("created_at") or "") > str(submit.get("created_at") or ""):
            continue
        payload = event_payload(event)
        role = str(payload.get("asset_type") or "").strip().lower()
        if role not in {"source", "delivery"}:
            continue
        # The historical batch path submitted delivery files. A source
        # completion in the same window is a competing membership, not a
        # second permissible interpretation.
        if role != "delivery":
            return None
        sku_code = str(payload.get("target_sku_code") or "").strip()
        version_ids = payload_asset_version_ids(payload)
        root_ids = payload_ids(payload, "asset_id")
        session_id = str(payload.get("upload_session_id") or "").strip()
        if (
            event.get("namespace") != "task_event_log"
            or sku_code not in set(sku_codes)
            or len(version_ids) != 1
            or len(root_ids) != 1
            or not session_id
        ):
            return None
        version_id = version_ids[0]
        asset = asset_by_id.get(version_id)
        if asset is None:
            return None
        if version_id in seen_version_ids or session_id in seen_sessions:
            return None
        if (
            int(asset.get("task_id") or 0) != int(sku_scopes[0]["task_id"])
            or str(asset.get("scope_sku_code") or "").strip() != sku_code
            or asset.get("retouch_requirement_id")
            or str(asset.get("asset_type") or "").lower() != "delivery"
            or int(asset.get("asset_id") or 0) != root_ids[0]
            or str(asset.get("upload_session_id") or asset.get("upload_request_id") or "").strip() != session_id
            or str(asset.get("created_at") or "") > str(event.get("created_at") or "")
            or int(event.get("actor_id") or 0) != int(submit.get("actor_id") or 0)
        ):
            return None
        eligible, _ = revision_asset_eligible(asset, {"design"}, str(submit["created_at"]))
        if not eligible:
            return None
        seen_version_ids.add(version_id)
        seen_sessions.add(session_id)
        memberships[sku_code].append({
            "asset_version_id": version_id,
            "completion_event_id": stable_event_id(event),
            "upload_session_id": session_id,
            "asset_root_id": root_ids[0],
            "created_at": event["created_at"],
        })

    if any(len(memberships.get(code, [])) != 1 for code in sku_codes):
        return None
    final_membership = memberships[submit_sku][0]
    if (
        final_membership["upload_session_id"] != submit_session
        or final_membership["created_at"] > submit["created_at"]
        or final_membership["asset_root_id"] != submit_root_ids[0]
    ):
        return None

    candidate = dict(submit)
    candidate["_batch_scope_memberships"] = {
        code: memberships[code][0] for code in sorted(sku_codes)
    }
    candidate["_migration_policy"] = BATCH_SUBMIT_POLICY
    return candidate


def scoped_boundary_events(scope, events, assets, sku_scope_count):
    applicable = [
        dict(event) for event in events
        if event_kind(event) and event_applies_to_scope(event, scope, events, assets, sku_scope_count)
    ]
    # Prefer task_event_logs when legacy dual-write produced an equivalent
    # task_module_event. Evidence remains stable without replaying the same
    # business boundary twice.
    applicable.sort(key=lambda event: (event["created_at"], event.get("namespace") == "task_module_event", str(event["id"])))
    deduped, seen = [], {}
    for event in applicable:
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
            ORG_MANUAL_TARGET_POLICY
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


def build_access_decisions(rows: dict[str, Any]) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
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
        department_id = positive_or_none(user.get("department_id"))
        team_id = positive_or_none(user.get("team_id"))
        is_issue = (
            legacy_role not in KNOWN_ACCESS_ROLES
            or (legacy_role in {"DepartmentAdmin", "DesignDirector"} and department_id is None)
            or (legacy_role == "TeamLead" and team_id is None)
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
        elif legacy_role == "OrgAdmin" and any(
            row["role_code"] in {"super_admin", "access_admin"}
            and row["scope_mode"] == "global"
            for row in evidence
        ):
            action = "preserve_existing"
            policies = [EXISTING_ACCESS_PRESERVED_POLICY]
        elif legacy_role == "Outsource":
            policies = [OUTSOURCE_ACCESS_DECISION_POLICY]
            blockers.append(
                "Outsource has no V8 role equivalent; explicit no-grant or separately provisioned replacement is required"
            )
        elif legacy_role == "OrgAdmin":
            policies = [ORG_ADMIN_ACCESS_DECISION_POLICY]
            blockers.append(
                "OrgAdmin cannot be widened to global access_admin without an explicit administrator decision"
            )
        else:
            policies = [EXISTING_ACCESS_PRESERVED_POLICY]
            blockers.append(
                "legacy role lacks a complete stable organization scope or explicit V8 replacement"
            )
        if not evidence:
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
    batch_submit_by_task = {
        task_id: candidate
        for task_id, task_scopes in scopes_by_task.items()
        if (candidate := resolve_atomic_multi_sku_batch_submit(
            task_scopes,
            events_by_task[task_id],
            assets_by_task[task_id],
        )) is not None
    }
    manual, resources = [], []
    for scope in sorted(rows["scopes"], key=lambda x: (int(x["task_id"]), x["scope_kind"], int(x["scope_ref_id"]))):
        scope = dict(scope)
        task_id = int(scope["task_id"])
        scope["_single_sku"] = scope["scope_kind"] == "sku" and sku_scope_counts[task_id] == 1
        scope["_single_requirement"] = (
            scope["scope_kind"] == "retouch_requirement" and retouch_scope_counts[task_id] == 1
        )
        boundary = scoped_boundary_events(
            scope,
            events_by_task[task_id],
            assets_by_task[task_id],
            sku_scope_counts[task_id],
        )
        if scope["scope_kind"] == "sku" and task_id in batch_submit_by_task:
            boundary = [dict(batch_submit_by_task[task_id])]
        revisions = []
        working = None
        finalized = None
        if not boundary:
            status = effective_v8_status(str(scope.get("task_status") or ""))
            if status in {"PendingAudit", "Completed"}:
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
                    current["reason"] = (
                        f"policy {RETOUCH_TERMINAL_SUBMIT_POLICY}: current "
                        "retouch submit semantics complete the task directly; "
                        "the legacy Completed task has one scope-proven final "
                        "and a matching completed upload session; human "
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
                        })
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
                    if stable_event_id(event) not in current["evidence_event_ids"]:
                        current["evidence_event_ids"].append(stable_event_id(event))
                    add_blocker(current, "rejection boundary has no preceding submitted revision")
                    recompute_revision_hash(current)
                else:
                    current["status"] = "rejected"
                    current.pop("finalized_at", None)
                    current["evidence_event_ids"].append(stable_event_id(event))
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
                before, after = nested_number(payload, "before"), nested_number(payload, "after")
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
    planning_tasks = []
    planning_by_task = defaultdict(list)
    for row in rows.get("planning_rows", []):
        planning_by_task[int(row["task_id"])].append(row)
    for task_id, items in sorted(planning_by_task.items()):
        task_status = str(items[0].get("task_status") or "")
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
                "image_storage_ref_id": "",
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
        manual.append({"task_id": task["task_id"], "scope_kind": "task_state_decision", "scope_ref_id": 0, "revision_no": "", "confidence": "hard_blocked", "reason": "RejectedByWarehouse requires an explicit reviewed InProgress or Completed decision with warehouse evidence", "evidence_event_ids": "", "candidate_source_ids": "", "candidate_final_ids": "", "reviewer_id": "", "reviewed_at": "", "decision": "", "review_note": ""})
    organization_mappings, organization_manual = build_organization_mappings(rows)
    access_decisions, access_manual = build_access_decisions(rows)
    manual.extend(organization_manual)
    manual.extend(access_manual)
    return {
        "version": 2,
        "resources": resources,
        "planning_tasks": planning_tasks,
        "task_state_decisions": [],
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
        SUM(TIMESTAMPDIFF(SECOND,ta.created_at,e.created_at) BETWEEN 28798 AND 28802) near_eight_hour_count,
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
SELECT CONCAT('assets\t',JSON_OBJECT('id',ta.id,'asset_id',ta.asset_id,'task_id',ta.task_id,'asset_type',ta.asset_type,'scope_sku_code',COALESCE(ta.scope_sku_code,''),'retouch_requirement_id',ta.retouch_requirement_id,'upload_session_id',COALESCE(ta.upload_session_id,''),'upload_request_id',COALESCE(ta.upload_request_id,''),'source_module_key',COALESCE(ta.source_module_key,''),'flow_review_status',COALESCE(ta.flow_review_status,''),'rejected_at',DATE_FORMAT(DATE_SUB(ta.rejected_at,INTERVAL 8 HOUR),'%Y-%m-%dT%H:%i:%sZ'),'superseded_by_version_id',ta.superseded_by_version_id,'superseded_at',DATE_FORMAT(DATE_SUB(ta.superseded_at,INTERVAL 8 HOUR),'%Y-%m-%dT%H:%i:%sZ'),'storage_ref_id',COALESCE(ta.storage_ref_id,''),'storage_key',COALESCE(ta.storage_key,''),'file_size',ta.file_size,'mime_type',COALESCE(ta.mime_type,''),'whole_hash',COALESCE(ta.whole_hash,''),'upload_status',COALESCE(ta.upload_status,''),'uploaded_at_legacy_raw',DATE_FORMAT(ta.uploaded_at,'%Y-%m-%d %H:%i:%s'),'created_at',DATE_FORMAT(ta.created_at,'%Y-%m-%dT%H:%i:%sZ'),'deleted_at',DATE_FORMAT(DATE_SUB(ta.deleted_at,INTERVAL 8 HOUR),'%Y-%m-%dT%H:%i:%sZ'),'cleaned_at',DATE_FORMAT(ta.cleaned_at,'%Y-%m-%dT%H:%i:%sZ'),'access_revoked_at',DATE_FORMAT(ta.access_revoked_at,'%Y-%m-%dT%H:%i:%sZ'),'object_deleted_at',DATE_FORMAT(ta.object_deleted_at,'%Y-%m-%dT%H:%i:%sZ'),'storage_adapter',COALESCE(sr.storage_adapter,''),'ref_key',COALESCE(sr.ref_key,''),'checksum_hint',COALESCE(sr.checksum_hint,''),'storage_status',COALESCE(sr.status,''),'is_placeholder',COALESCE(sr.is_placeholder,0))) FROM task_assets ta LEFT JOIN asset_storage_refs sr ON sr.ref_id=ta.storage_ref_id ORDER BY ta.task_id,ta.created_at,ta.id;
SELECT CONCAT('references\t',JSON_OBJECT('id',r.id,'task_id',r.task_id,'scope_sku_code',COALESCE(si.sku_code,''),'retouch_requirement_id',r.retouch_requirement_id,'attached_at',DATE_FORMAT(r.attached_at,'%Y-%m-%dT%H:%i:%sZ'),'ref_id',r.ref_id,'storage_adapter',COALESCE(sr.storage_adapter,''),'ref_key',COALESCE(sr.ref_key,''),'file_size',sr.file_size,'mime_type',COALESCE(sr.mime_type,''),'checksum_hint',COALESCE(sr.checksum_hint,''),'storage_status',COALESCE(sr.status,''),'is_placeholder',COALESCE(sr.is_placeholder,0))) FROM reference_file_refs r LEFT JOIN task_sku_items si ON si.id=r.sku_item_id LEFT JOIN asset_storage_refs sr ON sr.ref_id=r.ref_id ORDER BY r.task_id,r.attached_at,r.id;
SELECT CONCAT('events\t',JSON_OBJECT('namespace','task_event_log','id',e.id,'task_id',e.task_id,'sequence',e.sequence,'event_type',e.event_type,'actor_id',e.operator_id,'payload',e.payload,'module_key',COALESCE(JSON_UNQUOTE(JSON_EXTRACT(e.payload,'$.module_key')),''),'from_state',COALESCE(JSON_UNQUOTE(JSON_EXTRACT(e.payload,'$.from')),''),'to_state',COALESCE(JSON_UNQUOTE(JSON_EXTRACT(e.payload,'$.to')),''),'created_at',DATE_FORMAT(DATE_SUB(e.created_at,INTERVAL 8 HOUR),'%Y-%m-%dT%H:%i:%sZ'))) FROM task_event_logs e UNION ALL SELECT CONCAT('events\t',JSON_OBJECT('namespace','task_module_event','id',e.id,'task_id',m.task_id,'sequence',e.id,'event_type',e.event_type,'actor_id',e.actor_id,'payload',e.payload,'module_key',m.module_key,'from_state',COALESCE(e.from_state,''),'to_state',COALESCE(e.to_state,''),'created_at',DATE_FORMAT(e.created_at,'%Y-%m-%dT%H:%i:%sZ'))) FROM task_module_events e JOIN task_modules m ON m.id=e.task_module_id;
SELECT CONCAT('planning_rows\t',JSON_OBJECT('task_id',t.id,'task_status',t.task_status,'creator_id',t.creator_id,'task_sku_item_id',si.id,'description_spec',COALESCE(si.design_requirement,''),'quantity',si.quantity,'target_price',si.base_sale_price,'erp_product_i_id',COALESCE(si.product_i_id,''),'erp_product_name',COALESCE(si.product_name_snapshot,''),'reference_file_refs_json',COALESCE(si.reference_file_refs_json,'[]'))) FROM tasks t LEFT JOIN task_sku_items si ON si.task_id=t.id WHERE t.task_type='purchase_task' ORDER BY t.id,si.id;
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
    for prefix, tolerance in (("asset_created", (28798, 28802)), ("superseded", (28797, 28803))):
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
