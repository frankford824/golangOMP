#!/usr/bin/env python3
"""Derive a canonical confirmed-decision artifact from the exact review chain.

The tool is deliberately file-only.  It verifies caller-supplied SHA-256
bindings for the review template, administrator decision, apply evidence, and
final reviewed mapping before deriving a narrower Clone-B-only confirmation.
It never changes any input and refuses to overwrite its output.
"""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
import pathlib
import re
import sys
from collections import Counter
from typing import Any

import review_migration_mapping as review


SHA256 = re.compile(r"^[0-9a-f]{64}$")
TEMPLATE_FIELDS = {
    "schema_version",
    "decision",
    "candidate_sha256",
    "cohort_digest",
    "ledger_sha256",
    "reviewer_id",
    "reviewed_at",
    "note",
    "approved_policies",
}
AUTOMATIC_DECISION_FIELDS = TEMPLATE_FIELDS | {
    "decision_mode",
    "manual_row_review_performed",
    "eligible_row_count",
    "authorization_boundary",
}
CATEGORY_FIELDS = {
    "revision": ("resources", "promoted_revision_count", "remaining_revision_confidence_counts"),
    "planning": ("planning_tasks", "promoted_planning_count", "remaining_planning_confidence_counts"),
    "task_state": (
        "task_state_decisions",
        "promoted_task_state_count",
        "remaining_task_state_confidence_counts",
    ),
    "asset_recovery": (
        "asset_recoveries",
        "promoted_asset_recovery_count",
        "remaining_asset_recovery_confidence_counts",
    ),
    "organization": (
        "organization_mappings",
        "promoted_organization_count",
        "remaining_organization_confidence_counts",
    ),
    "access": ("access_decisions", "promoted_access_count", "remaining_access_confidence_counts"),
}


def canonical_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def require_sha256(value: str, label: str) -> str:
    if not isinstance(value, str) or not SHA256.fullmatch(value):
        raise ValueError(f"{label} must be a lowercase SHA-256")
    return value


def reject_duplicate_keys(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate JSON key {key!r}")
        result[key] = value
    return result


def load_object(path: pathlib.Path, expected_sha256: str, label: str) -> dict[str, Any]:
    expected = require_sha256(expected_sha256, f"{label} expected sha256")
    raw = path.read_bytes()
    actual = hashlib.sha256(raw).hexdigest()
    if actual != expected:
        raise ValueError(f"{label} sha256 mismatch: expected {expected}, got {actual}")
    value = json.loads(
        raw.decode("utf-8"),
        object_pairs_hook=reject_duplicate_keys,
    )
    if not isinstance(value, dict):
        raise ValueError(f"{label} must be a JSON object")
    return value


def normalize_timestamp(value: Any, label: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{label} must be a timezone-aware RFC3339 timestamp")
    raw = value.strip()
    try:
        parsed = dt.datetime.fromisoformat(raw[:-1] + "+00:00" if raw.endswith("Z") else raw)
    except ValueError as exc:
        raise ValueError(f"{label} must be a timezone-aware RFC3339 timestamp") from exc
    if parsed.tzinfo is None or parsed.utcoffset() is None:
        raise ValueError(f"{label} must include a timezone")
    normalized = parsed.astimezone(dt.timezone.utc)
    result = normalized.strftime("%Y-%m-%dT%H:%M:%S")
    if normalized.microsecond:
        result += "." + f"{normalized.microsecond:06d}".rstrip("0")
    return result + "Z"


def validate_template(
    template: dict[str, Any],
    candidate_sha256: str,
    cohort_digest: str,
) -> str:
    if set(template) != TEMPLATE_FIELDS:
        raise ValueError("review template has an unexpected field set")
    if (
        template.get("schema_version") != 1
        or template.get("decision") != "PENDING_REVIEW"
        or template.get("candidate_sha256") != candidate_sha256
        or template.get("cohort_digest") != cohort_digest
        or template.get("reviewer_id") != 0
        or template.get("reviewed_at") != ""
        or template.get("note") != ""
        or template.get("approved_policies") != []
    ):
        raise ValueError("review template does not match the frozen pending-review contract")
    return require_sha256(template.get("ledger_sha256"), "review template ledger_sha256")


def validate_admin_decision(
    decision: dict[str, Any],
    candidate_sha256: str,
    cohort_digest: str,
    ledger_sha256: str,
) -> tuple[int, str, list[str]]:
    fields = set(decision)
    frozen_fields = frozenset(fields)
    if frozen_fields not in {
        frozenset(TEMPLATE_FIELDS),
        frozenset(AUTOMATIC_DECISION_FIELDS),
    }:
        raise ValueError("admin decision has an unexpected field set")
    if (
        decision.get("schema_version") != 1
        or decision.get("decision") != "approve"
        or decision.get("candidate_sha256") != candidate_sha256
        or decision.get("cohort_digest") != cohort_digest
        or decision.get("ledger_sha256") != ledger_sha256
    ):
        raise ValueError("admin decision does not match the exact frozen review template")
    reviewer_id = decision.get("reviewer_id")
    if isinstance(reviewer_id, bool) or not isinstance(reviewer_id, int) or reviewer_id <= 0:
        raise ValueError("admin decision reviewer_id must be a positive integer")
    confirmed_at = normalize_timestamp(decision.get("reviewed_at"), "admin decision reviewed_at")
    if not isinstance(decision.get("note"), str) or not decision["note"].strip():
        raise ValueError("admin decision note must be non-empty")
    policies = decision.get("approved_policies")
    if not isinstance(policies, list) or not all(isinstance(item, str) for item in policies):
        raise ValueError("admin decision approved_policies must be an array of strings")
    if len(policies) != len(set(policies)):
        raise ValueError("admin decision approved_policies must not contain duplicates")
    unknown = sorted(set(policies) - set(review.POLICIES))
    if unknown:
        raise ValueError(f"admin decision contains unknown policies: {unknown}")
    if not policies:
        raise ValueError("admin decision must approve at least one policy")
    if fields == AUTOMATIC_DECISION_FIELDS:
        if (
            decision.get("decision_mode") != "automatic_policy_engine"
            or decision.get("manual_row_review_performed") is not False
            or isinstance(decision.get("eligible_row_count"), bool)
            or not isinstance(decision.get("eligible_row_count"), int)
            or decision["eligible_row_count"] <= 0
            or not isinstance(decision.get("authorization_boundary"), str)
            or not decision["authorization_boundary"].strip()
        ):
            raise ValueError(
                "automatic admin decision metadata is incomplete"
            )
    canonical_policies = [policy for policy in review.POLICIES if policy in set(policies)]
    return reviewer_id, confirmed_at, canonical_policies


def confidence_counts(mapping: dict[str, Any]) -> dict[str, Counter[str]]:
    counts = {category: Counter() for category in CATEGORY_FIELDS}
    resources = mapping.get("resources")
    if not isinstance(resources, list):
        raise ValueError("reviewed mapping resources must be an array")
    for resource_index, resource in enumerate(resources):
        if not isinstance(resource, dict) or not isinstance(resource.get("history"), list):
            raise ValueError(f"reviewed mapping resources[{resource_index}].history must be an array")
        for revision in resource["history"]:
            counts["revision"][str(revision.get("confidence"))] += 1
    for category, (mapping_field, _, _) in CATEGORY_FIELDS.items():
        if category == "revision":
            continue
        rows = mapping.get(mapping_field)
        if not isinstance(rows, list):
            raise ValueError(f"reviewed mapping {mapping_field} must be an array")
        for row in rows:
            counts[category][str(row.get("confidence"))] += 1
    return counts


def nonnegative_integer(value: Any, label: str) -> int:
    if isinstance(value, bool) or not isinstance(value, int) or value < 0:
        raise ValueError(f"{label} must be a non-negative integer")
    return value


def validate_apply_evidence(
    evidence: dict[str, Any],
    *,
    candidate_sha256: str,
    cohort_digest: str,
    ledger_sha256: str,
    admin_decision_sha256: str,
    reviewed_mapping_sha256: str,
    reviewer_id: int,
    confirmed_at: str,
    approved_policies: list[str],
    mapping_counts: dict[str, Counter[str]],
) -> tuple[int, int]:
    expected = {
        "schema_version": 1,
        "phase": "apply",
        "status": "APPLIED",
        "candidate_sha256": candidate_sha256,
        "cohort_digest": cohort_digest,
        "ledger_sha256": ledger_sha256,
        "decision_sha256": admin_decision_sha256,
        "reviewed_mapping_sha256": reviewed_mapping_sha256,
        "reviewer_id": reviewer_id,
        "reviewed_at": confirmed_at,
        "approved_policies": approved_policies,
    }
    for field, value in expected.items():
        if evidence.get(field) != value:
            raise ValueError(f"review apply evidence {field} mismatch")

    promoted_rows = evidence.get("promoted_rows")
    if not isinstance(promoted_rows, list):
        raise ValueError("review apply evidence promoted_rows must be an array")
    promoted_total = 0
    confirmed_total = 0
    for category, (_, promoted_field, remaining_field) in CATEGORY_FIELDS.items():
        promoted = nonnegative_integer(evidence.get(promoted_field), promoted_field)
        remaining = evidence.get(remaining_field)
        if not isinstance(remaining, dict):
            raise ValueError(f"{remaining_field} must be an object")
        if set(remaining) - {"confirmed_auto"}:
            raise ValueError(f"{remaining_field} still contains unconfirmed confidence states")
        remaining_confirmed = nonnegative_integer(
            remaining.get("confirmed_auto", 0),
            f"{remaining_field}.confirmed_auto",
        )
        counts = mapping_counts[category]
        if set(counts) - {"confirmed_auto"}:
            raise ValueError(f"reviewed mapping {category} rows are not all confirmed_auto")
        mapping_confirmed = counts["confirmed_auto"]
        if promoted + remaining_confirmed != mapping_confirmed:
            raise ValueError(
                f"review apply evidence {category} counts do not cover the reviewed mapping"
            )
        promoted_total += promoted
        confirmed_total += mapping_confirmed
    if len(promoted_rows) != promoted_total:
        raise ValueError("review apply evidence promoted_rows count mismatch")
    if promoted_total <= 0:
        raise ValueError("review apply evidence did not promote any proposed_review rows")
    return promoted_total, confirmed_total


def write_no_overwrite(path: pathlib.Path, content: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor: int | None = None
    created = False
    try:
        descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o440)
        created = True
        with os.fdopen(descriptor, "wb") as handle:
            descriptor = None
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
    except BaseException:
        if descriptor is not None:
            os.close(descriptor)
        if created:
            try:
                path.unlink()
            except FileNotFoundError:
                pass
        raise


def build(args: argparse.Namespace) -> dict[str, Any]:
    candidate_sha256 = require_sha256(args.candidate_sha256, "candidate_sha256")
    cohort_digest = require_sha256(args.cohort_digest, "cohort_digest")
    paths = {
        "review_template": pathlib.Path(args.review_template),
        "admin_decision": pathlib.Path(args.admin_decision),
        "review_apply_evidence": pathlib.Path(args.apply_evidence),
        "reviewed_mapping": pathlib.Path(args.reviewed_mapping),
    }
    output = pathlib.Path(args.output)
    resolved_inputs = {path.resolve() for path in paths.values()}
    if output.resolve() in resolved_inputs:
        raise ValueError("output must be distinct from every input")
    if output.exists():
        raise FileExistsError(f"refusing to overwrite existing output: {output}")

    template = load_object(paths["review_template"], args.template_sha256, "review template")
    admin = load_object(paths["admin_decision"], args.admin_decision_sha256, "admin decision")
    evidence = load_object(
        paths["review_apply_evidence"],
        args.apply_evidence_sha256,
        "review apply evidence",
    )
    mapping = load_object(paths["reviewed_mapping"], args.mapping_sha256, "reviewed mapping")
    template_sha256 = args.template_sha256
    admin_sha256 = args.admin_decision_sha256
    evidence_sha256 = args.apply_evidence_sha256
    mapping_sha256 = args.mapping_sha256

    ledger_sha256 = validate_template(template, candidate_sha256, cohort_digest)
    reviewer_id, confirmed_at, approved_policies = validate_admin_decision(
        admin,
        candidate_sha256,
        cohort_digest,
        ledger_sha256,
    )
    review.validate_candidate(mapping)
    counts = confidence_counts(mapping)
    promoted_total, confirmed_total = validate_apply_evidence(
        evidence,
        candidate_sha256=candidate_sha256,
        cohort_digest=cohort_digest,
        ledger_sha256=ledger_sha256,
        admin_decision_sha256=admin_sha256,
        reviewed_mapping_sha256=mapping_sha256,
        reviewer_id=reviewer_id,
        confirmed_at=confirmed_at,
        approved_policies=approved_policies,
        mapping_counts=counts,
    )

    document = {
        "schema_version": 1,
        "decision": "confirmed",
        "authorized_scope": "clone_b_validation_only",
        "production_write_authorized": False,
        "candidate_sha256": candidate_sha256,
        "cohort_digest": cohort_digest,
        "ledger_sha256": ledger_sha256,
        "review_template_sha256": template_sha256,
        "admin_decision_sha256": admin_sha256,
        "review_apply_evidence_sha256": evidence_sha256,
        "reviewed_mapping_sha256": mapping_sha256,
        "reviewer_id": reviewer_id,
        "confirmed_at": confirmed_at,
        "approved_policies": approved_policies,
        "promoted_review_count": promoted_total,
        "confirmed_mapping_row_count": confirmed_total,
    }
    write_no_overwrite(output, (canonical_json(document) + "\n").encode("utf-8"))
    return document


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Build a hash-bound Clone-B-only confirmed review decision."
    )
    parser.add_argument("--review-template", required=True)
    parser.add_argument("--template-sha256", required=True)
    parser.add_argument("--admin-decision", required=True)
    parser.add_argument("--admin-decision-sha256", required=True)
    parser.add_argument("--apply-evidence", required=True)
    parser.add_argument("--apply-evidence-sha256", required=True)
    parser.add_argument("--reviewed-mapping", required=True)
    parser.add_argument("--mapping-sha256", required=True)
    parser.add_argument("--candidate-sha256", required=True)
    parser.add_argument("--cohort-digest", required=True)
    parser.add_argument("--output", required=True)
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    try:
        build(parse_args(argv))
    except (
        FileExistsError,
        OSError,
        UnicodeDecodeError,
        ValueError,
        json.JSONDecodeError,
    ) as exc:
        print(str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
