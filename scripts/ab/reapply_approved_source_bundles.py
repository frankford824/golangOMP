#!/usr/bin/env python3
"""Reapply an unchanged, already-approved source-bundle decision to a new candidate.

This is deliberately narrower than a new bundle review.  It accepts only the
seven frozen scopes, proves that the prior final-reviewed mapping and confirmed
bundle manifest describe the same ordered members and Clone B IDs, and changes
only those seven still-hard-blocked candidate revisions back to
``proposed_review`` bundle rows.  A subsequent reviewed-mapping rebase decides
whether the prior confirmations can be inherited.
"""

from __future__ import annotations

import argparse
import copy
import hashlib
import json
import pathlib
from typing import Any

from apply_bundle_registry_to_mapping import (
    EXACT_SCOPES,
    MULTI_SOURCE_BLOCKERS,
    ZERO_TIME,
    atomic_write_many,
    canonical_bytes,
    load_object,
    require_sha256,
    revision_hash,
    sha256_file,
)


def scope_key(resource: dict[str, Any], revision: dict[str, Any]) -> tuple[int, str, int, int]:
    return (
        int(resource["task_id"]),
        str(resource["scope_kind"]),
        int(resource["scope_ref_id"]),
        int(revision["revision_no"]),
    )


def index_revisions(mapping: dict[str, Any], label: str) -> dict[tuple[int, str, int, int], dict[str, Any]]:
    result: dict[tuple[int, str, int, int], dict[str, Any]] = {}
    if mapping.get("version") != 2 or not isinstance(mapping.get("resources"), list):
        raise ValueError(f"{label} must be a V2 mapping")
    for resource_index, resource in enumerate(mapping["resources"]):
        if not isinstance(resource, dict) or not isinstance(resource.get("history"), list):
            raise ValueError(f"{label}.resources[{resource_index}] is invalid")
        for revision_index, revision in enumerate(resource["history"]):
            if not isinstance(revision, dict):
                raise ValueError(
                    f"{label}.resources[{resource_index}].history[{revision_index}] is invalid"
                )
            key = scope_key(resource, revision)
            if key in result:
                raise ValueError(f"{label} duplicates revision {key}")
            expected_hash = require_sha256(
                revision.get("manifest_row_hash"),
                f"{label} revision {key}.manifest_row_hash",
            )
            if revision_hash(revision) != expected_hash:
                raise ValueError(f"{label} revision {key} row hash is stale")
            result[key] = revision
    return result


def manifest_bundle_index(
    manifest: dict[str, Any],
    expected_template_sha256: str,
) -> dict[tuple[int, str, int, int], dict[str, Any]]:
    if (
        manifest.get("schema_version") != 1
        or manifest.get("status") != "CONFIRMED"
        or manifest.get("decision_template_sha256")
        != require_sha256(expected_template_sha256, "expected decision template sha256")
        or int(manifest.get("confirmed_by") or 0) <= 0
        or not str(manifest.get("confirmed_at") or "").strip()
        or not str(manifest.get("confirmation_note") or "").strip()
    ):
        raise ValueError("confirmed bundle manifest approval boundary is invalid")
    bundles = manifest.get("bundles")
    if not isinstance(bundles, list):
        raise ValueError("confirmed bundle manifest bundles must be an array")
    result: dict[tuple[int, str, int, int], dict[str, Any]] = {}
    for index, bundle in enumerate(bundles):
        if not isinstance(bundle, dict):
            raise ValueError(f"confirmed bundle manifest bundles[{index}] is invalid")
        key = (
            int(bundle["task_id"]),
            str(bundle["scope_kind"]),
            int(bundle["scope_ref_id"]),
            int(bundle["revision_no"]),
        )
        if key not in EXACT_SCOPES or key in result or bundle.get("confirmed") is not True:
            raise ValueError(f"confirmed bundle manifest has unexpected scope {key}")
        members = bundle.get("ordered_members")
        if not isinstance(members, list):
            raise ValueError(f"confirmed bundle manifest scope {key} has no ordered members")
        member_ids = tuple(int(member["task_asset_id"]) for member in members)
        if member_ids != EXACT_SCOPES[key]:
            raise ValueError(f"confirmed bundle manifest scope {key} member order drifted")
        for member in members:
            if member.get("confirmed") is not True:
                raise ValueError(f"confirmed bundle manifest scope {key} has unconfirmed member")
            require_sha256(member.get("sha256"), f"confirmed bundle manifest scope {key} member sha256")
        result[key] = bundle
    if set(result) != set(EXACT_SCOPES):
        raise ValueError("confirmed bundle manifest does not cover the exact seven scopes")
    return result


def reapply(
    baseline: dict[str, Any],
    candidate: dict[str, Any],
    confirmed_manifest: dict[str, Any],
    expected_template_sha256: str,
) -> tuple[dict[str, Any], list[dict[str, Any]]]:
    output = copy.deepcopy(candidate)
    baseline_revisions = index_revisions(baseline, "baseline")
    candidate_revisions = index_revisions(output, "candidate")
    manifest_bundles = manifest_bundle_index(
        confirmed_manifest,
        expected_template_sha256,
    )
    evidence_rows: list[dict[str, Any]] = []
    for key in EXACT_SCOPES:
        baseline_revision = baseline_revisions.get(key)
        candidate_revision = candidate_revisions.get(key)
        if baseline_revision is None or candidate_revision is None:
            raise ValueError(f"bundle scope {key} is absent from a mapping")
        source_bundle = baseline_revision.get("source_bundle")
        if (
            baseline_revision.get("confidence") != "confirmed_auto"
            or not isinstance(source_bundle, dict)
            or int(baseline_revision.get("confirmed_by") or 0) <= 0
        ):
            raise ValueError(f"baseline bundle scope {key} is not final-reviewed")
        manifest_bundle = manifest_bundles[key]
        manifest_members = [
            (int(member["task_asset_id"]), require_sha256(member["sha256"], "manifest member sha256"))
            for member in manifest_bundle["ordered_members"]
        ]
        reviewed_members = [
            (int(member["task_asset_id"]), require_sha256(member["sha256"], "reviewed member sha256"))
            for member in source_bundle.get("members", [])
        ]
        if (
            reviewed_members != manifest_members
            or int(source_bundle.get("task_asset_id") or 0)
            != int(manifest_bundle["bundle_task_asset_id"])
            or source_bundle.get("confirmed_by") != confirmed_manifest["confirmed_by"]
            or source_bundle.get("confirmed_at") != confirmed_manifest["confirmed_at"]
            or source_bundle.get("confirmation_note")
            != confirmed_manifest["confirmation_note"]
        ):
            raise ValueError(f"baseline bundle scope {key} differs from the approved manifest")
        blockers = candidate_revision.get("blockers")
        if (
            candidate_revision.get("confidence") != "hard_blocked"
            or not isinstance(blockers, list)
            or "multiple source assets require a reviewed deterministic ZIP bundle"
            not in blockers
            or any(blocker not in MULTI_SOURCE_BLOCKERS for blocker in blockers)
            or candidate_revision.get("source_bundle") is not None
            or candidate_revision.get("source_task_asset_id") is not None
        ):
            raise ValueError(f"candidate bundle scope {key} is not the exact frozen hard blocker")
        prior_hash = candidate_revision["manifest_row_hash"]
        displaced_alias = candidate_revision.pop("source_alias_from_task_asset_id", None)
        candidate_revision["source_bundle"] = copy.deepcopy(source_bundle)
        candidate_revision.pop("blockers", None)
        candidate_revision["confidence"] = "proposed_review"
        candidate_revision["confirmed_by"] = 0
        candidate_revision["confirmed_at"] = ZERO_TIME
        candidate_revision["confirmation_note"] = ""
        candidate_revision["manifest_row_hash"] = revision_hash(candidate_revision)
        evidence_rows.append(
            {
                "task_id": key[0],
                "scope_kind": key[1],
                "scope_ref_id": key[2],
                "revision_no": key[3],
                "ordered_member_task_asset_ids": [member[0] for member in manifest_members],
                "bundle_task_asset_id": source_bundle["task_asset_id"],
                "bundle_sha256": require_sha256(
                    source_bundle["bundle_sha256"],
                    f"bundle scope {key} sha256",
                ),
                "candidate_manifest_row_hash": prior_hash,
                "output_manifest_row_hash": candidate_revision["manifest_row_hash"],
                "displaced_source_alias_from_task_asset_id": displaced_alias,
            }
        )
    return output, evidence_rows


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--baseline", type=pathlib.Path, required=True)
    parser.add_argument("--candidate", type=pathlib.Path, required=True)
    parser.add_argument("--confirmed-manifest", type=pathlib.Path, required=True)
    parser.add_argument("--expected-baseline-sha256", required=True)
    parser.add_argument("--expected-candidate-sha256", required=True)
    parser.add_argument("--expected-confirmed-manifest-sha256", required=True)
    parser.add_argument("--expected-decision-template-sha256", required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--evidence-output", type=pathlib.Path, required=True)
    args = parser.parse_args()
    expected = {
        "baseline_mapping_sha256": require_sha256(
            args.expected_baseline_sha256,
            "expected baseline sha256",
        ),
        "candidate_mapping_sha256": require_sha256(
            args.expected_candidate_sha256,
            "expected candidate sha256",
        ),
        "confirmed_manifest_sha256": require_sha256(
            args.expected_confirmed_manifest_sha256,
            "expected confirmed manifest sha256",
        ),
    }
    actual = {
        "baseline_mapping_sha256": sha256_file(args.baseline),
        "candidate_mapping_sha256": sha256_file(args.candidate),
        "confirmed_manifest_sha256": sha256_file(args.confirmed_manifest),
    }
    if actual != expected:
        raise ValueError(f"input SHA-256 mismatch: expected={expected} actual={actual}")
    baseline = load_object(args.baseline, "baseline")
    candidate = load_object(args.candidate, "candidate")
    confirmed_manifest = load_object(args.confirmed_manifest, "confirmed manifest")
    output, rows = reapply(
        baseline,
        candidate,
        confirmed_manifest,
        args.expected_decision_template_sha256,
    )
    output_bytes = canonical_bytes(output)
    evidence = {
        "schema_version": 1,
        "status": "PASS",
        "operation": "reapply_unchanged_approved_source_bundles",
        "input_sha256": actual,
        "decision_template_sha256": args.expected_decision_template_sha256,
        "output_mapping_sha256": hashlib.sha256(output_bytes).hexdigest(),
        "target_count": len(rows),
        "targets": rows,
        "database_write_performed": False,
        "business_policy_review_performed": False,
    }
    atomic_write_many(
        [
            (args.output, output_bytes),
            (args.evidence_output, canonical_bytes(evidence)),
        ]
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
