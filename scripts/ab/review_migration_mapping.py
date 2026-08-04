#!/usr/bin/env python3
"""Prepare and apply policy-bound review decisions for a V8 mapping candidate.

This tool is intentionally file-only.  It never connects to a database and
never mutates the candidate.  ``prepare`` derives a deterministic row ledger
and a decision template.  ``apply`` reconstructs that ledger from the exact
candidate bytes before it can confirm any revision.
"""

from __future__ import annotations

import argparse
import copy
import datetime as dt
import hashlib
import json
import os
import pathlib
import re
import sys
import tempfile
from collections import Counter
from typing import Any


SHA256 = re.compile(r"^[0-9a-f]{64}$")
POLICY_ID = re.compile(r"^[a-z][a-z0-9_]{2,127}$")
ZERO_TIME = "0001-01-01T00:00:00Z"
POLICIES = (
    "explicit_event_replay",
    "delivery_source_alias",
    "rejected_history",
    "reopen",
    "legacy_post_close_replacement_v1",
    "retouch_source_optional",
    "legacy_retouch_terminal_submit_v1",
    "legacy_retouch_unscoped_atomic_batch_v1",
    "legacy_retouch_premature_terminal_partial_v1",
    "legacy_retouch_visual_scope_task2533_v1",
    "legacy_multi_sku_atomic_batch_submit_v1",
    "legacy_atomic_upload_batch_submit_v1",
    "legacy_audit_stage_final_snapshot_v1",
    "legacy_purchase_to_sku_planning_v1",
    "legacy_incomplete_uat_planning_tombstone_v1",
    "frozen_sku_planning_rule_revision_9_v1",
    "product_name_snapshot_description_fallback_v1",
    "retired_planning_status_to_completed_v1",
    "legacy_org_unique_stable_match_v1",
    "legacy_org_alias_lineage_v1",
    "legacy_org_manual_target_required_v1",
    "retired_warehouse_no_new_grant_v1",
    "existing_access_assignment_preserved_v1",
    "legacy_outsource_access_decision_v1",
    "legacy_org_admin_access_decision_v1",
    "legacy_uat_orphan_org_to_unassigned_v1",
    "legacy_warehouse_reopen_state_v1",
    "legacy_customization_terminal_without_assets_to_inprogress_v1",
    "legacy_deleted_asset_recovery_v1",
    "legacy_historical_asset_unavailable_v1",
)
POLICY_CATALOG = {
    "explicit_event_replay": (
        "Approve replay from the exact ordered evidence_event_ids recorded on "
        "the candidate revision."
    ),
    "delivery_source_alias": (
        "Approve a delivery asset as the immutable source alias when the "
        "candidate has no distinct legacy source asset."
    ),
    "rejected_history": (
        "Approve retention of a rejected historical revision in the immutable "
        "revision chain."
    ),
    "reopen": (
        "Approve reconstruction of a reopen revision from the recorded legacy "
        "reopen or rejection boundary."
    ),
    "legacy_post_close_replacement_v1": (
        "Approve a finalized post-close reopen revision only when an immutable "
        "same-root predecessor-successor edge, the exact completed upload "
        "session, and the inherited full snapshot evidence are all present."
    ),
    "retouch_source_optional": (
        "Approve a retouch revision with no V8 source asset while preserving "
        "its references and finals."
    ),
    "legacy_retouch_terminal_submit_v1": (
        "Approve finalizing a legacy Completed retouch submit at submitted_at "
        "only when one scope-proven persisted final and its matching completed "
        "upload session exist and no later reject, reopen, or submit exists."
    ),
    "legacy_retouch_unscoped_atomic_batch_v1": (
        "Approve the exact allowlisted legacy retouch final memberships only "
        "when the frozen submit, session, actor, and scope facts all match."
    ),
    "legacy_retouch_premature_terminal_partial_v1": (
        "Approve preserving only proven partial retouch work and reopening the "
        "enumerated tasks as InProgress without inventing missing finals."
    ),
    "legacy_retouch_visual_scope_task2533_v1": (
        "Approve only task 2533 requirements 183..187 with the exact "
        "read-only visually reviewed source, final, and reference memberships; "
        "delivery asset 19803 remains preserved but unassigned."
    ),
    "legacy_multi_sku_atomic_batch_submit_v1": (
        "Approve the last scoped submit as the trigger for the task-level "
        "atomic transition only after full SKU coverage is independently "
        "proven."
    ),
    "legacy_atomic_upload_batch_submit_v1": (
        "Approve one deterministic submission snapshot from contiguous "
        "same-actor completed upload sessions within fifteen minutes of the "
        "submit boundary when no other workflow boundary intervenes."
    ),
    "legacy_audit_stage_final_snapshot_v1": (
        "Approve an audit-stage replacement snapshot only when every changed "
        "source/final belongs to one completed 15-minute audit batch by the "
        "approving auditor."
    ),
    "legacy_purchase_to_sku_planning_v1": (
        "Approve migration of a legacy purchase task into the sku_planning "
        "task model while preserving existing SKU identities."
    ),
    "frozen_sku_planning_rule_revision_9_v1": (
        "Approve binding migrated planning rows to the independently verified "
        "frozen sku_planning rule revision 9."
    ),
    "product_name_snapshot_description_fallback_v1": (
        "Approve use of the frozen product_name_snapshot when the legacy "
        "description_spec is empty."
    ),
    "retired_planning_status_to_completed_v1": (
        "Approve mapping a retired downstream planning status to Completed."
    ),
    "legacy_org_unique_stable_match_v1": (
        "Approve a unique exact legacy organization display-name match to the "
        "frozen stable department/team pair."
    ),
    "legacy_org_alias_lineage_v1": (
        "Approve a repository-declared rename lineage from legacy organization "
        "labels to one frozen stable department/team pair."
    ),
    "legacy_org_manual_target_required_v1": (
        "Marks an organization subject whose stable target is not proven; this "
        "policy is never review-eligible while the row is hard_blocked."
    ),
    "retired_warehouse_no_new_grant_v1": (
        "Approve retirement of the Warehouse role without adding any V8 grant, "
        "while binding the decision to the user's exact existing assignments."
    ),
    "existing_access_assignment_preserved_v1": (
        "Approve preservation of the exact existing V8 assignments as complete "
        "evidence for a redundant legacy role."
    ),
    "legacy_outsource_access_decision_v1": (
        "Marks an Outsource account requiring an explicit replacement or "
        "no-new-grant decision before review."
    ),
    "legacy_org_admin_access_decision_v1": (
        "Marks an OrgAdmin account requiring an explicit decision that cannot "
        "silently widen it to global access administration."
    ),
    "legacy_uat_orphan_org_to_unassigned_v1": (
        "Approve routing only the enumerated orphan UAT tasks to the frozen "
        "unassigned department/team sink without claiming historical lineage."
    ),
    "legacy_incomplete_uat_planning_tombstone_v1": (
        "Approve preserving the enumerated incomplete UAT SKU identity as a "
        "Cancelled planning tombstone with no fabricated detail or revision."
    ),
    "legacy_warehouse_reopen_state_v1": (
        "Approve mapping a proven warehouse rejection to InProgress when every "
        "resource scope retains its finalized history and has a reopen draft."
    ),
    "legacy_customization_terminal_without_assets_to_inprogress_v1": (
        "Approve mapping only the frozen incomplete customization terminal "
        "rows to InProgress while preserving the exact source allowlist in an "
        "editable draft and inventing no final asset."
    ),
    "legacy_deleted_asset_recovery_v1": (
        "Record the frozen size and pairwise preview/design-thumb evidence for "
        "a missing legacy object. Approval establishes only the semantic "
        "recovery decision; it does not prove target-environment byte "
        "materialization, database apply, object verification, or rollback, "
        "which remain mandatory later G4/G8 gates."
    ),
    "legacy_historical_asset_unavailable_v1": (
        "Record an irrecoverable superseded historical asset without claiming "
        "that its original bytes exist. Approval establishes only the semantic "
        "tombstone decision; it does not prove zero current references, exact "
        "HTTP 410 behavior, UI disclosure, or object-integrity G8, which remain "
        "mandatory later gates."
    ),
}
VERIFICATION_BOUNDARY = (
    "This review tool binds policy approval to the exact candidate bytes and "
    "canonical revision rows. It does not prove the candidate's timezone "
    "interpretation, legacy event semantics, asset membership, object "
    "integrity, or database truth; those require independent gates."
)
RETOUCH_VISUAL_TASK2533 = {
    183: (19299, 19789, [3211, 3212]),
    184: (19301, 19790, [3213]),
    185: (19304, 19791, [3214, 3215]),
    186: (19306, 19800, [3216]),
    187: (19308, 19802, [3217]),
}


def canonical_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def encoded_json(value: Any) -> bytes:
    return (canonical_json(value) + "\n").encode("utf-8")


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_json(value: Any) -> str:
    return sha256_bytes(canonical_json(value).encode("utf-8"))


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def atomic_write(path: pathlib.Path, content: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary_name = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    temporary = pathlib.Path(temporary_name)
    try:
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    except BaseException:
        temporary.unlink(missing_ok=True)
        raise


def require_distinct_paths(candidate: pathlib.Path, outputs: list[pathlib.Path]) -> None:
    resolved_candidate = candidate.resolve()
    resolved_outputs = [path.resolve() for path in outputs]
    if resolved_candidate in resolved_outputs:
        raise ValueError("output paths must not overwrite the candidate")
    if len(set(resolved_outputs)) != len(resolved_outputs):
        raise ValueError("output paths must be distinct")


def load_object(path: pathlib.Path, label: str) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{label} must be a JSON object")
    return value


def canonical_revision_hash(revision: dict[str, Any]) -> str:
    payload = {
        key: value
        for key, value in revision.items()
        if key not in {"manifest_row_hash", "_blockers"}
    }
    return sha256_json(payload)


def canonical_planning_hash(planning: dict[str, Any]) -> str:
    return sha256_json(
        {key: value for key, value in planning.items() if key != "manifest_row_hash"}
    )


def canonical_mapping_row_hash(row: dict[str, Any]) -> str:
    return sha256_json(
        {key: value for key, value in row.items() if key != "manifest_row_hash"}
    )


def validate_policy_ids(value: Any, path: str) -> list[str]:
    if not isinstance(value, list) or not value:
        raise ValueError(f"{path}.review_policy_ids must contain explicit policies")
    if not all(isinstance(policy, str) and POLICY_ID.fullmatch(policy) for policy in value):
        raise ValueError(
            f"{path}.review_policy_ids must use safe lowercase snake_case policy ids"
        )
    if len(value) != len(set(value)):
        raise ValueError(f"{path}.review_policy_ids must not contain duplicates")
    unknown = sorted(set(value) - set(POLICIES))
    if unknown:
        raise ValueError(f"{path}.review_policy_ids contains unknown policies: {unknown}")
    canonical = [policy for policy in POLICIES if policy in value]
    if value != canonical:
        raise ValueError(f"{path}.review_policy_ids must use canonical policy order")
    return list(value)


def row_key(resource: dict[str, Any], revision: dict[str, Any]) -> str:
    return (
        f"task:{resource['task_id']}/"
        f"{resource['scope_kind']}:{resource['scope_ref_id']}/"
        f"revision:{revision['revision_no']}"
    )


def planning_row_key(planning: dict[str, Any]) -> str:
    return f"task:{planning['task_id']}/planning"


def task_state_row_key(item: dict[str, Any]) -> str:
    return f"task:{item['task_id']}/state:{item['from_status']}->{item['target_status']}"


def organization_row_key(item: dict[str, Any]) -> str:
    return f"organization:{item['subject_type']}:{item['subject_id']}"


def access_row_key(item: dict[str, Any]) -> str:
    return f"access:user:{item['user_id']}/legacy-role:{item['legacy_role']}"

def asset_recovery_row_key(item: dict[str, Any]) -> str:
    return (
        f"task:{item['task_id']}/"
        f"asset-recovery:{item['missing_task_asset_id']}"
    )


def validate_candidate(candidate: dict[str, Any]) -> None:
    if candidate.get("version") != 2:
        raise ValueError("candidate mapping must be version 2")
    resources = candidate.get("resources")
    if not isinstance(resources, list):
        raise ValueError("candidate resources must be an array")
    seen_resources: set[tuple[int, str, int]] = set()
    seen_rows: set[str] = set()
    visual_task2533_scopes: set[int] = set()
    for resource_index, resource in enumerate(resources):
        if not isinstance(resource, dict):
            raise ValueError(f"resources[{resource_index}] must be an object")
        task_id = resource.get("task_id")
        scope_kind = resource.get("scope_kind")
        scope_ref_id = resource.get("scope_ref_id")
        if isinstance(task_id, bool) or not isinstance(task_id, int) or task_id <= 0:
            raise ValueError(f"resources[{resource_index}].task_id must be positive")
        if scope_kind not in {"task", "sku", "retouch_requirement"}:
            raise ValueError(f"resources[{resource_index}].scope_kind is invalid")
        if isinstance(scope_ref_id, bool) or not isinstance(scope_ref_id, int):
            raise ValueError(f"resources[{resource_index}].scope_ref_id must be an integer")
        if (scope_kind == "task" and scope_ref_id != 0) or (scope_kind != "task" and scope_ref_id <= 0):
            raise ValueError(f"resources[{resource_index}].scope_ref_id is invalid")
        resource_key = (task_id, scope_kind, scope_ref_id)
        if resource_key in seen_resources:
            raise ValueError(f"duplicate resource {resource_key}")
        seen_resources.add(resource_key)
        history = resource.get("history")
        if not isinstance(history, list):
            raise ValueError(f"resources[{resource_index}].history must be an array")
        for revision_index, revision in enumerate(history):
            path = f"resources[{resource_index}].history[{revision_index}]"
            if not isinstance(revision, dict):
                raise ValueError(f"{path} must be an object")
            if revision.get("revision_no") != revision_index + 1:
                raise ValueError(f"{path}.revision_no must be contiguous from 1")
            if revision.get("confidence") not in {
                "confirmed_auto",
                "proposed_review",
                "hard_blocked",
            }:
                raise ValueError(f"{path}.confidence is invalid")
            policy_ids = validate_policy_ids(revision.get("review_policy_ids"), path)
            if "legacy_retouch_visual_scope_task2533_v1" in policy_ids:
                expected = RETOUCH_VISUAL_TASK2533.get(scope_ref_id)
                if (
                    task_id != 2533
                    or scope_kind != "retouch_requirement"
                    or expected is None
                    or len(history) != 1
                    or resource.get("working_revision_no") != 1
                    or resource.get("finalized_revision_no") != 1
                    or revision.get("status") != "finalized"
                    or revision.get("source_stage") != "retouch"
                    or revision.get("mode") != "single"
                    or revision.get("source_task_asset_id") != expected[0]
                    or revision.get("final_task_asset_ids") != [expected[1]]
                    or revision.get("reference_file_ref_ids") != expected[2]
                    or policy_ids
                    != [
                        "explicit_event_replay",
                        "legacy_retouch_visual_scope_task2533_v1",
                    ]
                    or not str(revision.get("reason") or "").startswith(
                        "policy legacy_retouch_visual_scope_task2533_v1:"
                    )
                ):
                    raise ValueError(
                        f"{path} visual-scope policy does not match the exact "
                        "task 2533 frozen membership"
                    )
                visual_task2533_scopes.add(scope_ref_id)
            if (
                str(revision.get("reason") or "").startswith(
                    "policy legacy_multi_sku_atomic_batch_submit_v1:"
                )
                and "legacy_multi_sku_atomic_batch_submit_v1" not in policy_ids
            ):
                raise ValueError(
                    f"{path}.review_policy_ids omits "
                    "legacy_multi_sku_atomic_batch_submit_v1 required by reason"
                )
            if (
                str(revision.get("reason") or "").startswith(
                    "policy legacy_retouch_terminal_submit_v1:"
                )
                and "legacy_retouch_terminal_submit_v1" not in policy_ids
            ):
                raise ValueError(
                    f"{path}.review_policy_ids omits "
                    "legacy_retouch_terminal_submit_v1 required by reason"
                )
            if (
                str(revision.get("reason") or "").startswith(
                    "policy legacy_post_close_replacement_v1:"
                )
                and "legacy_post_close_replacement_v1" not in policy_ids
            ):
                raise ValueError(
                    f"{path}.review_policy_ids omits "
                    "legacy_post_close_replacement_v1 required by reason"
                )
            expected_hash = revision.get("manifest_row_hash")
            if not isinstance(expected_hash, str) or not SHA256.fullmatch(expected_hash):
                raise ValueError(f"{path}.manifest_row_hash must be lowercase SHA-256")
            if canonical_revision_hash(revision) != expected_hash:
                raise ValueError(f"{path}.manifest_row_hash does not match canonical revision content")
            if revision.get("confidence") == "confirmed_auto":
                if (
                    isinstance(revision.get("confirmed_by"), bool)
                    or not isinstance(revision.get("confirmed_by"), int)
                    or revision["confirmed_by"] <= 0
                    or not str(revision.get("confirmed_at") or "")
                    or revision.get("confirmed_at") == ZERO_TIME
                    or not str(revision.get("confirmation_note") or "").strip()
                    or revision.get("blockers")
                ):
                    raise ValueError(f"{path} has incomplete confirmed_auto metadata")
            if revision.get("confidence") == "proposed_review":
                evidence = revision.get("evidence_event_ids")
                if not isinstance(evidence, list) or not evidence or not all(
                    isinstance(item, str) and item.strip() for item in evidence
                ):
                    raise ValueError(f"{path} proposed_review requires evidence_event_ids")
            key = row_key(resource, revision)
            if key in seen_rows:
                raise ValueError(f"duplicate revision row {key}")
            seen_rows.add(key)
    if visual_task2533_scopes and visual_task2533_scopes != set(
        RETOUCH_VISUAL_TASK2533
    ):
        raise ValueError(
            "legacy_retouch_visual_scope_task2533_v1 requires all five exact "
            "requirements 183..187"
        )
    planning_tasks = candidate.get("planning_tasks")
    if not isinstance(planning_tasks, list):
        raise ValueError("candidate planning_tasks must be an array")
    seen_planning: set[int] = set()
    resource_task_ids = {resource["task_id"] for resource in resources}
    for planning_index, planning in enumerate(planning_tasks):
        path = f"planning_tasks[{planning_index}]"
        if not isinstance(planning, dict):
            raise ValueError(f"{path} must be an object")
        task_id = planning.get("task_id")
        if isinstance(task_id, bool) or not isinstance(task_id, int) or task_id <= 0:
            raise ValueError(f"{path}.task_id must be positive")
        if task_id in seen_planning:
            raise ValueError(f"duplicate planning task {task_id}")
        if task_id in resource_task_ids:
            raise ValueError(f"{path} overlaps a design resource task")
        seen_planning.add(task_id)
        confidence = planning.get("confidence")
        if confidence not in {"confirmed_auto", "proposed_review", "hard_blocked"}:
            raise ValueError(f"{path}.confidence is invalid")
        policy_ids = validate_policy_ids(planning.get("review_policy_ids"), path)
        is_uat_tombstone = (
            task_id == 497
            and "legacy_incomplete_uat_planning_tombstone_v1" in policy_ids
        )
        required_policies = (
            (
                "legacy_purchase_to_sku_planning_v1",
                "legacy_incomplete_uat_planning_tombstone_v1",
                "frozen_sku_planning_rule_revision_9_v1",
            )
            if is_uat_tombstone
            else (
                "legacy_purchase_to_sku_planning_v1",
                "frozen_sku_planning_rule_revision_9_v1",
            )
        )
        for required in required_policies:
            if required not in policy_ids:
                raise ValueError(f"{path}.review_policy_ids must include {required}")
        if is_uat_tombstone:
            if (
                planning.get("target_task_status") != "Cancelled"
                or planning.get("code_rule_revision_id") != 9
                or len(planning.get("items") or []) != 1
                or planning["items"][0]
                != {
                    "task_sku_item_id": 380,
                    "description_spec": "",
                    "quantity": 0,
                    "target_price": None,
                    "note": "",
                    "reference_url": "",
                    "erp_product_i_id": "",
                    "erp_product_name": "",
                    "image_storage_ref_id": "",
                }
            ):
                raise ValueError(
                    f"{path} UAT tombstone requires Cancelled, rule 9, exact "
                    "SKU item 380, and zero inferred fields"
                )
        elif planning.get("code_rule_revision_id") != 9:
            raise ValueError(
                f"{path}.frozen_sku_planning_rule_revision_9_v1 requires "
                "code_rule_revision_id=9"
            )
        expected_hash = planning.get("manifest_row_hash")
        if not isinstance(expected_hash, str) or not SHA256.fullmatch(expected_hash):
            raise ValueError(f"{path}.manifest_row_hash must be lowercase SHA-256")
        if canonical_planning_hash(planning) != expected_hash:
            raise ValueError(
                f"{path}.manifest_row_hash does not match canonical planning content"
            )
        if confidence == "confirmed_auto":
            if (
                isinstance(planning.get("confirmed_by"), bool)
                or not isinstance(planning.get("confirmed_by"), int)
                or planning["confirmed_by"] <= 0
                or not str(planning.get("confirmed_at") or "")
                or planning.get("confirmed_at") == ZERO_TIME
                or not str(planning.get("confirmation_note") or "").strip()
                or planning.get("blockers")
            ):
                raise ValueError(f"{path} has incomplete confirmed_auto metadata")
    task_state_decisions = candidate.get("task_state_decisions", [])
    if not isinstance(task_state_decisions, list):
        raise ValueError("candidate task_state_decisions must be an array")
    seen_task_decisions: set[int] = set()
    for index, item in enumerate(task_state_decisions):
        path = f"task_state_decisions[{index}]"
        if not isinstance(item, dict):
            raise ValueError(f"{path} must be an object")
        task_id = item.get("task_id")
        if isinstance(task_id, bool) or not isinstance(task_id, int) or task_id <= 0:
            raise ValueError(f"{path}.task_id must be positive")
        if task_id in seen_task_decisions:
            raise ValueError(f"duplicate task state decision {task_id}")
        seen_task_decisions.add(task_id)
        confidence = item.get("confidence")
        if confidence not in {"confirmed_auto", "proposed_review", "hard_blocked"}:
            raise ValueError(f"{path}.confidence is invalid")
        policies = validate_policy_ids(item.get("review_policy_ids"), path)
        retouch_decision = (
            item.get("from_status") == "Completed"
            and item.get("target_status") == "InProgress"
            and policies
            == ["legacy_retouch_premature_terminal_partial_v1"]
        )
        warehouse_decision = (
            item.get("from_status") == "RejectedByWarehouse"
            and item.get("target_status") in {"InProgress", "Completed"}
            and "legacy_warehouse_reopen_state_v1" in policies
        )
        customization_terminal_decision = (
            (
                task_id in {449, 450, 451, 452, 756, 757}
                and item.get("from_status") == "PendingWarehouseReceive"
                and item.get("target_status") == "InProgress"
                and policies
                == [
                    "legacy_customization_terminal_without_assets_to_inprogress_v1"
                ]
            )
            or (
                task_id == 3091
                and item.get("from_status") == "Completed"
                and item.get("target_status") == "InProgress"
                and policies
                == [
                    "legacy_customization_terminal_without_assets_to_inprogress_v1",
                    "legacy_historical_asset_unavailable_v1",
                ]
            )
        )
        if (
            not retouch_decision
            and not warehouse_decision
            and not customization_terminal_decision
        ):
            raise ValueError(f"{path} has an unsupported policy-bound transition")
        evidence = item.get("evidence_event_ids")
        if not isinstance(evidence, list) or not evidence or not all(
            isinstance(value, str) and value.strip() for value in evidence
        ):
            raise ValueError(f"{path}.evidence_event_ids is invalid")
        if confidence == "hard_blocked" and not item.get("blockers"):
            raise ValueError(f"{path} hard_blocked requires blockers")
        if confidence == "confirmed_auto" and (
            isinstance(item.get("confirmed_by"), bool)
            or not isinstance(item.get("confirmed_by"), int)
            or item["confirmed_by"] <= 0
            or item.get("confirmed_at") in {"", ZERO_TIME}
            or not str(item.get("confirmation_note") or "").strip()
            or item.get("blockers")
        ):
            raise ValueError(f"{path} has incomplete confirmed_auto metadata")
        expected_hash = item.get("manifest_row_hash")
        if not isinstance(expected_hash, str) or not SHA256.fullmatch(expected_hash):
            raise ValueError(f"{path}.manifest_row_hash must be lowercase SHA-256")
        if canonical_mapping_row_hash(item) != expected_hash:
            raise ValueError(f"{path}.manifest_row_hash does not match canonical content")
    asset_recoveries = candidate.get("asset_recoveries", [])
    if not isinstance(asset_recoveries, list):
        raise ValueError("candidate asset_recoveries must be an array")
    seen_recoveries: set[int] = set()
    allowed_pairs = {
        23989: 24034,
        23990: 24033,
        23991: 24040,
        12323: 0,
    }
    recovery_source_hashes = {
        23989: "d0558b1a9d4a7afed5a03b6b97d4a765d34050866686e396ab0acf9f08f0dec5",
        23990: "64cdfed11adc778fb6ede7f03c49f7c70e8655870236bdcd92a8207e41a8dfb8",
        23991: "ebfecf3407e05c576bcddf74673d2e7568207ecc27855aa0e08c453d5a0d119a",
    }
    for index, item in enumerate(asset_recoveries):
        path = f"asset_recoveries[{index}]"
        if not isinstance(item, dict):
            raise ValueError(f"{path} must be an object")
        missing_id = item.get("missing_task_asset_id")
        if missing_id not in allowed_pairs:
            raise ValueError(f"{path} is outside the frozen recovery evidence set")
        if missing_id in seen_recoveries:
            raise ValueError(f"{path} duplicates missing_task_asset_id")
        seen_recoveries.add(missing_id)
        expected_strategy = (
            "historical_unavailable_tombstone_v1"
            if missing_id == 12323
            else "verified_oss_recovery_v1"
        )
        expected_policy = (
            "legacy_historical_asset_unavailable_v1"
            if missing_id == 12323
            else "legacy_deleted_asset_recovery_v1"
        )
        if (
            item.get("recovery_source_task_asset_id") != allowed_pairs[missing_id]
            or item.get("strategy")
            != expected_strategy
            or validate_policy_ids(item.get("review_policy_ids"), path)
            != [expected_policy]
        ):
            raise ValueError(f"{path} differs from the frozen recovery contract")
        confidence = item.get("confidence")
        if missing_id == 12323:
            if (
                confidence not in {"proposed_review", "confirmed_auto"}
                or item.get("rejected_source_task_asset_ids") != [14510, 14514]
                or item.get("object_probe_result") != "not_found"
                or item.get("object_probe_input_manifest_sha256")
                != "3f17b37296d2670235ca9bfcfd4388823b81adecf8fbac0826e6f241923579c7"
                or item.get("object_probe_evidence_hash")
                != "f1c78819e1f3d5f4e7a4b25ff3d173368574a5639f4c6df45c8aae5482d047b8"
                or item.get("object_probe_object_key_sha256")
                != "e732f6cd269a93d6bac168b0852dbcf8480af8966847278cb073cd6905b0efdd"
                or item.get("object_probe_read_only_get_count") != 1
                or item.get("blockers")
            ):
                raise ValueError(
                    f"{path} must retain the exact missing-object probe and "
                    "size-mismatched successors as rejected evidence"
                )
            if confidence == "confirmed_auto" and (
                not isinstance(item.get("confirmed_by"), int)
                or item.get("confirmed_by", 0) <= 0
                or item.get("confirmed_at") in {"", ZERO_TIME}
                or not str(item.get("confirmation_note") or "").strip()
            ):
                raise ValueError(
                    f"{path} has incomplete confirmed_auto metadata"
                )
        elif (
            confidence not in {"proposed_review", "confirmed_auto"}
            or item.get("controlled_read_protocol")
            != "controlled-asset-read-v1"
            or item.get("controlled_read_evidence_sha256")
            != "b39e0d9d26e6fdd35941b195bdc413eb12dd6795e23276a48c9b9bd49f829b08"
            or item.get("recovery_source_sha256")
            != recovery_source_hashes[missing_id]
            or item.get("blockers")
        ):
            raise ValueError(
                f"{path} must bind the exact controlled source read evidence"
            )
        elif confidence == "confirmed_auto" and (
            not isinstance(item.get("confirmed_by"), int)
            or item.get("confirmed_by", 0) <= 0
            or item.get("confirmed_at") in {"", ZERO_TIME}
            or not str(item.get("confirmation_note") or "").strip()
        ):
            raise ValueError(f"{path} has incomplete confirmed_auto metadata")
        expected_hash = item.get("manifest_row_hash")
        if not isinstance(expected_hash, str) or not SHA256.fullmatch(expected_hash):
            raise ValueError(f"{path}.manifest_row_hash must be lowercase SHA-256")
        if canonical_mapping_row_hash(item) != expected_hash:
            raise ValueError(f"{path}.manifest_row_hash does not match canonical content")
    organization_mappings = candidate.get("organization_mappings", [])
    if not isinstance(organization_mappings, list):
        raise ValueError("candidate organization_mappings must be an array")
    seen_organizations: set[tuple[str, int]] = set()
    for index, item in enumerate(organization_mappings):
        path = f"organization_mappings[{index}]"
        if not isinstance(item, dict):
            raise ValueError(f"{path} must be an object")
        subject_type = item.get("subject_type")
        subject_id = item.get("subject_id")
        if subject_type not in {"task", "user"}:
            raise ValueError(f"{path}.subject_type must be task or user")
        if isinstance(subject_id, bool) or not isinstance(subject_id, int) or subject_id <= 0:
            raise ValueError(f"{path}.subject_id must be positive")
        key = (subject_type, subject_id)
        if key in seen_organizations:
            raise ValueError(f"duplicate organization subject {key}")
        seen_organizations.add(key)
        confidence = item.get("confidence")
        if confidence not in {"confirmed_auto", "proposed_review", "hard_blocked"}:
            raise ValueError(f"{path}.confidence is invalid")
        policies = validate_policy_ids(item.get("review_policy_ids"), path)
        if "legacy_uat_orphan_org_to_unassigned_v1" in policies and (
            item["subject_type"] != "task"
            or item["subject_id"] not in {463, 464}
            or item.get("target_department_id") != 3
            or item.get("target_team_id") != 14
        ):
            raise ValueError(
                f"{path} UAT orphan policy is restricted to tasks 463/464 and 3/14"
            )
        if confidence != "hard_blocked" and (
            int(item.get("target_department_id") or 0) <= 0
            or int(item.get("target_team_id") or 0) <= 0
        ):
            raise ValueError(f"{path} requires positive stable target ids")
        if confidence == "hard_blocked" and not item.get("blockers"):
            raise ValueError(f"{path} hard_blocked requires blockers")
        if confidence == "confirmed_auto" and (
            isinstance(item.get("confirmed_by"), bool)
            or not isinstance(item.get("confirmed_by"), int)
            or item["confirmed_by"] <= 0
            or item.get("confirmed_at") in {"", ZERO_TIME}
            or not str(item.get("confirmation_note") or "").strip()
            or item.get("blockers")
        ):
            raise ValueError(f"{path} has incomplete confirmed_auto metadata")
        expected_hash = item.get("manifest_row_hash")
        if not isinstance(expected_hash, str) or not SHA256.fullmatch(expected_hash):
            raise ValueError(f"{path}.manifest_row_hash must be lowercase SHA-256")
        if canonical_mapping_row_hash(item) != expected_hash:
            raise ValueError(f"{path}.manifest_row_hash does not match canonical content")

    access_decisions = candidate.get("access_decisions", [])
    if not isinstance(access_decisions, list):
        raise ValueError("candidate access_decisions must be an array")
    seen_access: set[tuple[int, str]] = set()
    for index, item in enumerate(access_decisions):
        path = f"access_decisions[{index}]"
        if not isinstance(item, dict):
            raise ValueError(f"{path} must be an object")
        user_id = item.get("user_id")
        legacy_role = item.get("legacy_role")
        if isinstance(user_id, bool) or not isinstance(user_id, int) or user_id <= 0:
            raise ValueError(f"{path}.user_id must be positive")
        if not isinstance(legacy_role, str) or not legacy_role.strip():
            raise ValueError(f"{path}.legacy_role is required")
        key = (user_id, legacy_role)
        if key in seen_access:
            raise ValueError(f"duplicate access decision {key}")
        seen_access.add(key)
        confidence = item.get("confidence")
        if confidence not in {"confirmed_auto", "proposed_review", "hard_blocked"}:
            raise ValueError(f"{path}.confidence is invalid")
        validate_policy_ids(item.get("review_policy_ids"), path)
        if confidence != "hard_blocked" and item.get("action") not in {
            "no_new_grant",
            "preserve_existing",
        }:
            raise ValueError(f"{path}.action is invalid")
        evidence = item.get("required_existing_assignments")
        if not isinstance(evidence, list) or (
            confidence != "hard_blocked"
            and item.get("action") == "preserve_existing"
            and not evidence
        ):
            raise ValueError(f"{path}.required_existing_assignments is invalid")
        evidence_keys: list[tuple[str, str, str, int]] = []
        for evidence_index, assignment in enumerate(evidence):
            if not isinstance(assignment, dict):
                raise ValueError(
                    f"{path}.required_existing_assignments[{evidence_index}] must be an object"
                )
            evidence_keys.append(
                (
                    str(assignment.get("role_code") or ""),
                    str(assignment.get("scope_mode") or ""),
                    str(assignment.get("source_type") or ""),
                    int(assignment.get("source_ref_id") or 0),
                )
            )
        if evidence_keys != sorted(set(evidence_keys)) or any(
            not role_code
            or scope_mode
            not in {"self", "own_department", "own_team", "selected_org", "global"}
            or source_type not in {"direct", "org_policy", "migration"}
            or source_ref_id < 0
            for role_code, scope_mode, source_type, source_ref_id in evidence_keys
        ):
            raise ValueError(f"{path}.required_existing_assignments is not canonical")
        if confidence == "hard_blocked" and not item.get("blockers"):
            raise ValueError(f"{path} hard_blocked requires blockers")
        if confidence == "confirmed_auto" and (
            isinstance(item.get("confirmed_by"), bool)
            or not isinstance(item.get("confirmed_by"), int)
            or item["confirmed_by"] <= 0
            or item.get("confirmed_at") in {"", ZERO_TIME}
            or not str(item.get("confirmation_note") or "").strip()
            or item.get("blockers")
        ):
            raise ValueError(f"{path} has incomplete confirmed_auto metadata")
        expected_hash = item.get("manifest_row_hash")
        if not isinstance(expected_hash, str) or not SHA256.fullmatch(expected_hash):
            raise ValueError(f"{path}.manifest_row_hash must be lowercase SHA-256")
        if canonical_mapping_row_hash(item) != expected_hash:
            raise ValueError(f"{path}.manifest_row_hash does not match canonical content")


def build_ledger(candidate: dict[str, Any], candidate_sha256: str) -> dict[str, Any]:
    validate_candidate(candidate)
    rows: list[dict[str, Any]] = []
    revision_exclusions: Counter[str] = Counter()
    planning_exclusions: Counter[str] = Counter()
    task_state_exclusions: Counter[str] = Counter()
    revision_policy_counts: Counter[str] = Counter()
    planning_policy_counts: Counter[str] = Counter()
    task_state_policy_counts: Counter[str] = Counter()
    asset_recovery_exclusions: Counter[str] = Counter()
    asset_recovery_policy_counts: Counter[str] = Counter()
    organization_exclusions: Counter[str] = Counter()
    access_exclusions: Counter[str] = Counter()
    organization_policy_counts: Counter[str] = Counter()
    access_policy_counts: Counter[str] = Counter()
    for resource_index, resource in enumerate(candidate["resources"]):
        history = resource["history"]
        has_hard_sibling = any(
            revision.get("confidence") == "hard_blocked" for revision in history
        )
        for revision_index, revision in enumerate(history):
            exclusions: list[str] = []
            confidence = revision["confidence"]
            if confidence != "proposed_review":
                exclusions.append(f"confidence_is_{confidence}")
            if has_hard_sibling:
                exclusions.append("resource_has_hard_blocked_sibling")
            policies = list(revision["review_policy_ids"])
            eligible = not exclusions
            for exclusion in exclusions:
                revision_exclusions[exclusion] += 1
            if eligible:
                for policy in policies:
                    revision_policy_counts[policy] += 1
            rows.append(
                {
                    "row_type": "revision",
                    "row_key": row_key(resource, revision),
                    "resource_index": resource_index,
                    "revision_index": revision_index,
                    "task_id": resource["task_id"],
                    "scope_kind": resource["scope_kind"],
                    "scope_ref_id": resource["scope_ref_id"],
                    "revision_no": revision["revision_no"],
                    "candidate_confidence": confidence,
                    "candidate_manifest_row_hash": revision["manifest_row_hash"],
                    "evidence_event_ids_sha256": sha256_json(
                        revision.get("evidence_event_ids", [])
                    ),
                    "required_policies": policies,
                    "eligible": eligible,
                    "exclusion_reasons": exclusions,
                }
            )
    for planning_index, planning in enumerate(candidate["planning_tasks"]):
        confidence = planning["confidence"]
        exclusions = (
            [] if confidence == "proposed_review" else [f"confidence_is_{confidence}"]
        )
        eligible = not exclusions
        for exclusion in exclusions:
            planning_exclusions[exclusion] += 1
        if eligible:
            for policy in planning["review_policy_ids"]:
                planning_policy_counts[policy] += 1
        rows.append(
            {
                "row_type": "planning",
                "row_key": planning_row_key(planning),
                "planning_index": planning_index,
                "task_id": planning["task_id"],
                "candidate_confidence": confidence,
                "candidate_manifest_row_hash": planning["manifest_row_hash"],
                "required_policies": list(planning["review_policy_ids"]),
                "eligible": eligible,
                "exclusion_reasons": exclusions,
            }
        )
    for task_state_index, item in enumerate(candidate.get("task_state_decisions", [])):
        confidence = item["confidence"]
        exclusions = (
            [] if confidence == "proposed_review" else [f"confidence_is_{confidence}"]
        )
        for exclusion in exclusions:
            task_state_exclusions[exclusion] += 1
        eligible = not exclusions
        if eligible:
            for policy in item["review_policy_ids"]:
                task_state_policy_counts[policy] += 1
        rows.append(
            {
                "row_type": "task_state",
                "row_key": task_state_row_key(item),
                "task_state_index": task_state_index,
                "task_id": item["task_id"],
                "candidate_confidence": confidence,
                "candidate_manifest_row_hash": item["manifest_row_hash"],
                "evidence_event_ids_sha256": sha256_json(
                    item["evidence_event_ids"]
                ),
                "required_policies": list(item["review_policy_ids"]),
                "eligible": eligible,
                "exclusion_reasons": exclusions,
            }
        )
    for asset_recovery_index, item in enumerate(
        candidate.get("asset_recoveries", [])
    ):
        exclusions = []
        if item["confidence"] != "proposed_review":
            exclusions.append(f"confidence_is_{item['confidence']}")
        eligible = not exclusions
        for exclusion in exclusions:
            asset_recovery_exclusions[exclusion] += 1
        if eligible:
            for policy in item["review_policy_ids"]:
                asset_recovery_policy_counts[policy] += 1
        rows.append(
            {
                "row_type": "asset_recovery",
                "row_key": asset_recovery_row_key(item),
                "asset_recovery_index": asset_recovery_index,
                "task_id": item["task_id"],
                "missing_task_asset_id": item["missing_task_asset_id"],
                "candidate_confidence": item["confidence"],
                "candidate_manifest_row_hash": item["manifest_row_hash"],
                "required_policies": list(item["review_policy_ids"]),
                "eligible": eligible,
                "exclusion_reasons": exclusions,
            }
        )
    for organization_index, item in enumerate(candidate.get("organization_mappings", [])):
        confidence = item["confidence"]
        exclusions = (
            [] if confidence == "proposed_review" else [f"confidence_is_{confidence}"]
        )
        eligible = not exclusions
        for exclusion in exclusions:
            organization_exclusions[exclusion] += 1
        if eligible:
            for policy in item["review_policy_ids"]:
                organization_policy_counts[policy] += 1
        rows.append(
            {
                "row_type": "organization",
                "row_key": organization_row_key(item),
                "organization_index": organization_index,
                "subject_type": item["subject_type"],
                "subject_id": item["subject_id"],
                "candidate_confidence": confidence,
                "candidate_manifest_row_hash": item["manifest_row_hash"],
                "required_policies": list(item["review_policy_ids"]),
                "eligible": eligible,
                "exclusion_reasons": exclusions,
            }
        )
    for access_index, item in enumerate(candidate.get("access_decisions", [])):
        confidence = item["confidence"]
        exclusions = (
            [] if confidence == "proposed_review" else [f"confidence_is_{confidence}"]
        )
        eligible = not exclusions
        for exclusion in exclusions:
            access_exclusions[exclusion] += 1
        if eligible:
            for policy in item["review_policy_ids"]:
                access_policy_counts[policy] += 1
        rows.append(
            {
                "row_type": "access",
                "row_key": access_row_key(item),
                "access_index": access_index,
                "user_id": item["user_id"],
                "legacy_role": item["legacy_role"],
                "candidate_confidence": confidence,
                "candidate_manifest_row_hash": item["manifest_row_hash"],
                "assignment_evidence_sha256": sha256_json(
                    item["required_existing_assignments"]
                ),
                "required_policies": list(item["review_policy_ids"]),
                "eligible": eligible,
                "exclusion_reasons": exclusions,
            }
        )
    cohort = [
        {
            "row_type": row["row_type"],
            "row_key": row["row_key"],
            "candidate_manifest_row_hash": row["candidate_manifest_row_hash"],
            "required_policies": row["required_policies"],
        }
        for row in rows
        if row["eligible"]
    ]
    revision_rows = [row for row in rows if row["row_type"] == "revision"]
    planning_rows = [row for row in rows if row["row_type"] == "planning"]
    task_state_rows = [row for row in rows if row["row_type"] == "task_state"]
    asset_recovery_rows = [
        row for row in rows if row["row_type"] == "asset_recovery"
    ]
    organization_rows = [row for row in rows if row["row_type"] == "organization"]
    access_rows = [row for row in rows if row["row_type"] == "access"]
    eligible_revision_count = sum(row["eligible"] for row in revision_rows)
    eligible_planning_count = sum(row["eligible"] for row in planning_rows)
    eligible_task_state_count = sum(row["eligible"] for row in task_state_rows)
    eligible_asset_recovery_count = sum(
        row["eligible"] for row in asset_recovery_rows
    )
    eligible_organization_count = sum(row["eligible"] for row in organization_rows)
    eligible_access_count = sum(row["eligible"] for row in access_rows)
    return {
        "schema_version": 1,
        "candidate_sha256": candidate_sha256,
        "verification_boundary": VERIFICATION_BOUNDARY,
        "policy_catalog": [
            {"policy": policy, "description": POLICY_CATALOG[policy]} for policy in POLICIES
        ],
        "cohort_digest": sha256_json(cohort),
        "summary": {
            "resource_count": len(candidate["resources"]),
            "revision": {
                "row_count": len(revision_rows),
                "eligible_count": eligible_revision_count,
                "excluded_count": len(revision_rows) - eligible_revision_count,
                "eligible_policy_counts": {
                    policy: revision_policy_counts.get(policy, 0)
                    for policy in POLICIES
                },
                "exclusion_counts": dict(sorted(revision_exclusions.items())),
            },
            "planning": {
                "row_count": len(planning_rows),
                "eligible_count": eligible_planning_count,
                "excluded_count": len(planning_rows) - eligible_planning_count,
                "eligible_policy_counts": {
                    policy: planning_policy_counts.get(policy, 0)
                    for policy in POLICIES
                },
                "exclusion_counts": dict(sorted(planning_exclusions.items())),
            },
            "task_state": {
                "row_count": len(task_state_rows),
                "eligible_count": eligible_task_state_count,
                "excluded_count": len(task_state_rows) - eligible_task_state_count,
                "eligible_policy_counts": {
                    policy: task_state_policy_counts.get(policy, 0)
                    for policy in POLICIES
                },
                "exclusion_counts": dict(sorted(task_state_exclusions.items())),
            },
            "asset_recovery": {
                "row_count": len(asset_recovery_rows),
                "eligible_count": eligible_asset_recovery_count,
                "excluded_count": len(asset_recovery_rows)
                - eligible_asset_recovery_count,
                "eligible_policy_counts": {
                    policy: asset_recovery_policy_counts.get(policy, 0)
                    for policy in POLICIES
                },
                "exclusion_counts": dict(
                    sorted(asset_recovery_exclusions.items())
                ),
            },
            "organization": {
                "row_count": len(organization_rows),
                "eligible_count": eligible_organization_count,
                "excluded_count": len(organization_rows) - eligible_organization_count,
                "eligible_policy_counts": {
                    policy: organization_policy_counts.get(policy, 0)
                    for policy in POLICIES
                },
                "exclusion_counts": dict(sorted(organization_exclusions.items())),
            },
            "access": {
                "row_count": len(access_rows),
                "eligible_count": eligible_access_count,
                "excluded_count": len(access_rows) - eligible_access_count,
                "eligible_policy_counts": {
                    policy: access_policy_counts.get(policy, 0)
                    for policy in POLICIES
                },
                "exclusion_counts": dict(sorted(access_exclusions.items())),
            },
            "total_eligible_count": len(cohort),
            "total_excluded_count": len(rows) - len(cohort),
        },
        "rows": rows,
    }


def decision_template(
    candidate_sha256: str, cohort_digest: str, ledger_sha256: str
) -> dict[str, Any]:
    return {
        "schema_version": 1,
        "decision": "PENDING_REVIEW",
        "candidate_sha256": candidate_sha256,
        "cohort_digest": cohort_digest,
        "ledger_sha256": ledger_sha256,
        "reviewer_id": 0,
        "reviewed_at": "",
        "note": "",
        "approved_policies": [],
    }


def normalize_reviewed_at(value: Any) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError("reviewed_at must be a timezone-aware RFC3339 timestamp")
    raw = value.strip()
    try:
        parsed = dt.datetime.fromisoformat(raw[:-1] + "+00:00" if raw.endswith("Z") else raw)
    except ValueError as exc:
        raise ValueError("reviewed_at must be a timezone-aware RFC3339 timestamp") from exc
    if parsed.tzinfo is None or parsed.utcoffset() is None:
        raise ValueError("reviewed_at must include a timezone")
    normalized = parsed.astimezone(dt.timezone.utc)
    base = normalized.strftime("%Y-%m-%dT%H:%M:%S")
    if normalized.microsecond:
        base += "." + f"{normalized.microsecond:06d}".rstrip("0")
    return base + "Z"


def validate_decision(
    decision: dict[str, Any],
    candidate_sha256: str,
    cohort_digest: str,
    ledger_sha256: str,
) -> tuple[int, str, str, list[str]]:
    if decision.get("schema_version") != 1 or decision.get("decision") != "approve":
        raise ValueError("decision must set schema_version=1 and decision=approve")
    bindings = {
        "candidate_sha256": candidate_sha256,
        "cohort_digest": cohort_digest,
        "ledger_sha256": ledger_sha256,
    }
    for field, expected in bindings.items():
        if decision.get(field) != expected:
            raise ValueError(f"decision {field} mismatch")
    reviewer_id = decision.get("reviewer_id")
    if isinstance(reviewer_id, bool) or not isinstance(reviewer_id, int) or reviewer_id <= 0:
        raise ValueError("reviewer_id must be a positive integer")
    reviewed_at = normalize_reviewed_at(decision.get("reviewed_at"))
    note = decision.get("note")
    if not isinstance(note, str) or not note.strip():
        raise ValueError("note must be non-empty")
    if "approved_policies" not in decision:
        raise ValueError("approved_policies must be explicitly provided")
    approved = decision["approved_policies"]
    if not isinstance(approved, list) or not all(isinstance(item, str) for item in approved):
        raise ValueError("approved_policies must be an array of policy names")
    if len(approved) != len(set(approved)):
        raise ValueError("approved_policies must not contain duplicates")
    unknown = sorted(set(approved) - set(POLICIES))
    if unknown:
        raise ValueError(f"approved_policies contains unknown policies: {unknown}")
    normalized_policies = [policy for policy in POLICIES if policy in approved]
    return reviewer_id, reviewed_at, note.strip(), normalized_policies


def prepare(
    candidate_path: pathlib.Path,
    ledger_path: pathlib.Path,
    decision_path: pathlib.Path,
    evidence_path: pathlib.Path | None,
) -> None:
    outputs = [ledger_path, decision_path] + ([evidence_path] if evidence_path else [])
    require_distinct_paths(candidate_path, outputs)
    candidate_before = sha256_file(candidate_path)
    candidate = load_object(candidate_path, "candidate")
    ledger = build_ledger(candidate, candidate_before)
    ledger_bytes = encoded_json(ledger)
    ledger_sha256 = sha256_bytes(ledger_bytes)
    template = decision_template(
        candidate_before, ledger["cohort_digest"], ledger_sha256
    )
    evidence = {
        "schema_version": 1,
        "phase": "prepare",
        "status": "PREPARED",
        "candidate_sha256": candidate_before,
        "cohort_digest": ledger["cohort_digest"],
        "ledger_sha256": ledger_sha256,
        "decision_template_sha256": sha256_bytes(encoded_json(template)),
        "verification_boundary": VERIFICATION_BOUNDARY,
        "summary": ledger["summary"],
    }
    atomic_write(ledger_path, ledger_bytes)
    atomic_write(decision_path, encoded_json(template))
    if evidence_path:
        atomic_write(evidence_path, encoded_json(evidence))
    if sha256_file(candidate_path) != candidate_before:
        raise RuntimeError("candidate changed during prepare")


def apply_review(
    candidate_path: pathlib.Path,
    ledger_path: pathlib.Path,
    decision_path: pathlib.Path,
    output_path: pathlib.Path,
    evidence_path: pathlib.Path,
) -> None:
    require_distinct_paths(
        candidate_path, [ledger_path, decision_path, output_path, evidence_path]
    )
    candidate_before = sha256_file(candidate_path)
    candidate = load_object(candidate_path, "candidate")
    provided_ledger = load_object(ledger_path, "ledger")
    expected_ledger = build_ledger(candidate, candidate_before)
    if canonical_json(provided_ledger) != canonical_json(expected_ledger):
        raise ValueError("ledger does not match the exact candidate-derived cohort")
    ledger_sha256 = sha256_file(ledger_path)
    canonical_ledger_sha256 = sha256_bytes(encoded_json(expected_ledger))
    if ledger_sha256 != canonical_ledger_sha256:
        raise ValueError("ledger is not in canonical deterministic form")
    if provided_ledger.get("candidate_sha256") != candidate_before:
        raise ValueError("ledger candidate_sha256 mismatch")
    cohort_digest = provided_ledger.get("cohort_digest")
    if not isinstance(cohort_digest, str) or not SHA256.fullmatch(cohort_digest):
        raise ValueError("ledger cohort_digest is invalid")
    decision_bytes = decision_path.read_bytes()
    decision = json.loads(decision_bytes.decode("utf-8"))
    if not isinstance(decision, dict):
        raise ValueError("decision must be a JSON object")
    reviewer_id, reviewed_at, note, approved_policies = validate_decision(
        decision, candidate_before, cohort_digest, ledger_sha256
    )
    approved_set = set(approved_policies)
    reviewed = copy.deepcopy(candidate)
    promoted: list[dict[str, Any]] = []
    remaining_revision = Counter()
    remaining_planning = Counter()
    remaining_task_state = Counter()
    remaining_asset_recovery = Counter()
    remaining_organization = Counter()
    remaining_access = Counter()
    revision_row_by_position = {
        (row["resource_index"], row["revision_index"]): row
        for row in provided_ledger["rows"]
        if row["row_type"] == "revision"
    }
    planning_row_by_position = {
        row["planning_index"]: row
        for row in provided_ledger["rows"]
        if row["row_type"] == "planning"
    }
    task_state_row_by_position = {
        row["task_state_index"]: row
        for row in provided_ledger["rows"]
        if row["row_type"] == "task_state"
    }
    asset_recovery_row_by_position = {
        row["asset_recovery_index"]: row
        for row in provided_ledger["rows"]
        if row["row_type"] == "asset_recovery"
    }
    organization_row_by_position = {
        row["organization_index"]: row
        for row in provided_ledger["rows"]
        if row["row_type"] == "organization"
    }
    access_row_by_position = {
        row["access_index"]: row
        for row in provided_ledger["rows"]
        if row["row_type"] == "access"
    }
    for resource_index, resource in enumerate(reviewed["resources"]):
        for revision_index, revision in enumerate(resource["history"]):
            row = revision_row_by_position[(resource_index, revision_index)]
            if (
                row["eligible"]
                and revision.get("confidence") == "proposed_review"
                and set(row["required_policies"]).issubset(approved_set)
            ):
                revision["confidence"] = "confirmed_auto"
                revision.pop("blockers", None)
                revision.pop("_blockers", None)
                revision["confirmed_by"] = reviewer_id
                revision["confirmed_at"] = reviewed_at
                revision["confirmation_note"] = note
                revision["manifest_row_hash"] = canonical_revision_hash(revision)
                promoted.append(
                    {
                        "row_type": "revision",
                        "row_key": row["row_key"],
                        "prior_manifest_row_hash": row["candidate_manifest_row_hash"],
                        "reviewed_manifest_row_hash": revision["manifest_row_hash"],
                        "required_policies": row["required_policies"],
                    }
                )
            else:
                remaining_revision[revision["confidence"]] += 1
    for planning_index, planning in enumerate(reviewed["planning_tasks"]):
        row = planning_row_by_position[planning_index]
        if (
            row["eligible"]
            and planning.get("confidence") == "proposed_review"
            and set(row["required_policies"]).issubset(approved_set)
        ):
            planning["confidence"] = "confirmed_auto"
            planning.pop("blockers", None)
            planning["confirmed_by"] = reviewer_id
            planning["confirmed_at"] = reviewed_at
            planning["confirmation_note"] = note
            planning["manifest_row_hash"] = canonical_planning_hash(planning)
            promoted.append(
                {
                    "row_type": "planning",
                    "row_key": row["row_key"],
                    "prior_manifest_row_hash": row["candidate_manifest_row_hash"],
                    "reviewed_manifest_row_hash": planning["manifest_row_hash"],
                    "required_policies": row["required_policies"],
                }
            )
        else:
            remaining_planning[planning["confidence"]] += 1
    for task_state_index, item in enumerate(reviewed.get("task_state_decisions", [])):
        row = task_state_row_by_position[task_state_index]
        if (
            row["eligible"]
            and item.get("confidence") == "proposed_review"
            and set(row["required_policies"]).issubset(approved_set)
        ):
            item["confidence"] = "confirmed_auto"
            item.pop("blockers", None)
            item["confirmed_by"] = reviewer_id
            item["confirmed_at"] = reviewed_at
            item["confirmation_note"] = note
            item["manifest_row_hash"] = canonical_mapping_row_hash(item)
            promoted.append(
                {
                    "row_type": "task_state",
                    "row_key": row["row_key"],
                    "prior_manifest_row_hash": row["candidate_manifest_row_hash"],
                    "reviewed_manifest_row_hash": item["manifest_row_hash"],
                    "required_policies": row["required_policies"],
                }
            )
        else:
            remaining_task_state[item["confidence"]] += 1
    for asset_recovery_index, item in enumerate(reviewed.get("asset_recoveries", [])):
        row = asset_recovery_row_by_position[asset_recovery_index]
        if (
            row["eligible"]
            and item.get("confidence") == "proposed_review"
            and set(row["required_policies"]).issubset(approved_set)
        ):
            item["confidence"] = "confirmed_auto"
            item.pop("blockers", None)
            item["confirmed_by"] = reviewer_id
            item["confirmed_at"] = reviewed_at
            item["confirmation_note"] = note
            item["manifest_row_hash"] = canonical_mapping_row_hash(item)
            promoted.append(
                {
                    "row_type": "asset_recovery",
                    "row_key": row["row_key"],
                    "prior_manifest_row_hash": row[
                        "candidate_manifest_row_hash"
                    ],
                    "reviewed_manifest_row_hash": item["manifest_row_hash"],
                    "required_policies": row["required_policies"],
                }
            )
        else:
            remaining_asset_recovery[item["confidence"]] += 1
    for organization_index, item in enumerate(reviewed.get("organization_mappings", [])):
        row = organization_row_by_position[organization_index]
        if (
            row["eligible"]
            and item.get("confidence") == "proposed_review"
            and set(row["required_policies"]).issubset(approved_set)
        ):
            item["confidence"] = "confirmed_auto"
            item.pop("blockers", None)
            item["confirmed_by"] = reviewer_id
            item["confirmed_at"] = reviewed_at
            item["confirmation_note"] = note
            item["manifest_row_hash"] = canonical_mapping_row_hash(item)
            promoted.append(
                {
                    "row_type": "organization",
                    "row_key": row["row_key"],
                    "prior_manifest_row_hash": row["candidate_manifest_row_hash"],
                    "reviewed_manifest_row_hash": item["manifest_row_hash"],
                    "required_policies": row["required_policies"],
                }
            )
        else:
            remaining_organization[item["confidence"]] += 1
    for access_index, item in enumerate(reviewed.get("access_decisions", [])):
        row = access_row_by_position[access_index]
        if (
            row["eligible"]
            and item.get("confidence") == "proposed_review"
            and set(row["required_policies"]).issubset(approved_set)
        ):
            item["confidence"] = "confirmed_auto"
            item.pop("blockers", None)
            item["confirmed_by"] = reviewer_id
            item["confirmed_at"] = reviewed_at
            item["confirmation_note"] = note
            item["manifest_row_hash"] = canonical_mapping_row_hash(item)
            promoted.append(
                {
                    "row_type": "access",
                    "row_key": row["row_key"],
                    "prior_manifest_row_hash": row["candidate_manifest_row_hash"],
                    "reviewed_manifest_row_hash": item["manifest_row_hash"],
                    "required_policies": row["required_policies"],
                }
            )
        else:
            remaining_access[item["confidence"]] += 1
    for resource_index, (before_resource, after_resource) in enumerate(
        zip(candidate["resources"], reviewed["resources"])
    ):
        for revision_index, (before_revision, after_revision) in enumerate(
            zip(before_resource["history"], after_resource["history"])
        ):
            if before_revision["confidence"] == "hard_blocked" and after_revision != before_revision:
                raise RuntimeError(
                    f"hard_blocked revision changed at resources[{resource_index}].history[{revision_index}]"
                )
    for planning_index, (before_planning, after_planning) in enumerate(
        zip(candidate["planning_tasks"], reviewed["planning_tasks"])
    ):
        if (
            before_planning["confidence"] == "hard_blocked"
            and after_planning != before_planning
        ):
            raise RuntimeError(
                f"hard_blocked planning row changed at planning_tasks[{planning_index}]"
            )
    for index, (before_item, after_item) in enumerate(
        zip(candidate.get("task_state_decisions", []), reviewed.get("task_state_decisions", []))
    ):
        if before_item["confidence"] == "hard_blocked" and after_item != before_item:
            raise RuntimeError(
                f"hard_blocked task state row changed at task_state_decisions[{index}]"
            )
    for index, (before_item, after_item) in enumerate(
        zip(candidate.get("asset_recoveries", []), reviewed.get("asset_recoveries", []))
    ):
        if before_item["confidence"] == "hard_blocked" and after_item != before_item:
            raise RuntimeError(
                f"hard_blocked asset recovery row changed at asset_recoveries[{index}]"
            )
    for index, (before_item, after_item) in enumerate(
        zip(candidate.get("organization_mappings", []), reviewed.get("organization_mappings", []))
    ):
        if before_item["confidence"] == "hard_blocked" and after_item != before_item:
            raise RuntimeError(
                f"hard_blocked organization row changed at organization_mappings[{index}]"
            )
    for index, (before_item, after_item) in enumerate(
        zip(candidate.get("access_decisions", []), reviewed.get("access_decisions", []))
    ):
        if before_item["confidence"] == "hard_blocked" and after_item != before_item:
            raise RuntimeError(
                f"hard_blocked access row changed at access_decisions[{index}]"
            )
    validate_candidate(reviewed)
    reviewed_bytes = encoded_json(reviewed)
    reviewed_sha256 = sha256_bytes(reviewed_bytes)
    evidence = {
        "schema_version": 1,
        "phase": "apply",
        "status": "APPLIED",
        "candidate_sha256": candidate_before,
        "ledger_sha256": ledger_sha256,
        "cohort_digest": cohort_digest,
        "decision_sha256": sha256_bytes(decision_bytes),
        "reviewed_mapping_sha256": reviewed_sha256,
        "verification_boundary": VERIFICATION_BOUNDARY,
        "reviewer_id": reviewer_id,
        "reviewed_at": reviewed_at,
        "approved_policies": approved_policies,
        "promoted_revision_count": sum(
            row["row_type"] == "revision" for row in promoted
        ),
        "promoted_planning_count": sum(
            row["row_type"] == "planning" for row in promoted
        ),
        "promoted_task_state_count": sum(
            row["row_type"] == "task_state" for row in promoted
        ),
        "promoted_asset_recovery_count": sum(
            row["row_type"] == "asset_recovery" for row in promoted
        ),
        "promoted_organization_count": sum(
            row["row_type"] == "organization" for row in promoted
        ),
        "promoted_access_count": sum(
            row["row_type"] == "access" for row in promoted
        ),
        "remaining_revision_confidence_counts": dict(
            sorted(remaining_revision.items())
        ),
        "remaining_planning_confidence_counts": dict(
            sorted(remaining_planning.items())
        ),
        "remaining_task_state_confidence_counts": dict(
            sorted(remaining_task_state.items())
        ),
        "remaining_asset_recovery_confidence_counts": dict(
            sorted(remaining_asset_recovery.items())
        ),
        "remaining_organization_confidence_counts": dict(
            sorted(remaining_organization.items())
        ),
        "remaining_access_confidence_counts": dict(
            sorted(remaining_access.items())
        ),
        "promoted_rows": promoted,
    }
    atomic_write(output_path, reviewed_bytes)
    atomic_write(evidence_path, encoded_json(evidence))
    if sha256_file(output_path) != reviewed_sha256:
        raise RuntimeError("reviewed mapping hash changed after atomic write")
    if sha256_file(candidate_path) != candidate_before:
        raise RuntimeError("candidate changed during apply")


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Prepare or apply a policy-bound review of a V8 mapping candidate."
    )
    subparsers = parser.add_subparsers(dest="command", required=True)
    prepare_parser = subparsers.add_parser("prepare")
    prepare_parser.add_argument("--candidate", required=True)
    prepare_parser.add_argument("--ledger", required=True)
    prepare_parser.add_argument("--decision-template", required=True)
    prepare_parser.add_argument("--evidence")
    apply_parser = subparsers.add_parser("apply")
    apply_parser.add_argument("--candidate", required=True)
    apply_parser.add_argument("--ledger", required=True)
    apply_parser.add_argument("--decision", required=True)
    apply_parser.add_argument("--output", required=True)
    apply_parser.add_argument("--evidence", required=True)
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    try:
        args = parse_args(argv)
        if args.command == "prepare":
            prepare(
                pathlib.Path(args.candidate),
                pathlib.Path(args.ledger),
                pathlib.Path(args.decision_template),
                pathlib.Path(args.evidence) if args.evidence else None,
            )
        else:
            apply_review(
                pathlib.Path(args.candidate),
                pathlib.Path(args.ledger),
                pathlib.Path(args.decision),
                pathlib.Path(args.output),
                pathlib.Path(args.evidence),
            )
    except (
        OSError,
        UnicodeDecodeError,
        ValueError,
        RuntimeError,
        json.JSONDecodeError,
    ) as exc:
        print(str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
