#!/usr/bin/env python3
"""Create an explicit automatic policy decision from a frozen review ledger.

This file-only tool does not infer business facts. It approves exactly the
known policies required by rows that the review ledger already marked
eligible, records that no row-by-row human review occurred, and binds the
decision to the candidate, cohort, and ledger hashes.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
from typing import Any

try:
    from scripts.ab import review_migration_mapping as review
except ModuleNotFoundError:
    import review_migration_mapping as review


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


def read_object(path: pathlib.Path, label: str) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{label} must be a JSON object")
    return value


def build_decision(
    ledger: dict[str, Any],
    template: dict[str, Any],
    *,
    ledger_sha256: str,
    reviewer_id: int,
    reviewed_at: str,
    authorization_note: str,
) -> dict[str, Any]:
    if (
        ledger.get("schema_version") != 1
        or template.get("schema_version") != 1
        or template.get("decision") != "PENDING_REVIEW"
        or template.get("candidate_sha256")
        != ledger.get("candidate_sha256")
        or template.get("cohort_digest")
        != ledger.get("cohort_digest")
        or template.get("ledger_sha256") != ledger_sha256
    ):
        raise ValueError(
            "review template does not bind the frozen ledger"
        )
    if isinstance(reviewer_id, bool) or reviewer_id <= 0:
        raise ValueError("reviewer_id must be a positive integer")
    normalized_at = review.normalize_reviewed_at(reviewed_at)
    note = authorization_note.strip()
    if not note:
        raise ValueError("authorization_note must be non-empty")
    rows = ledger.get("rows")
    if not isinstance(rows, list):
        raise ValueError("ledger.rows must be an array")
    required: set[str] = set()
    eligible_count = 0
    for index, row in enumerate(rows):
        if not isinstance(row, dict):
            raise ValueError(f"ledger.rows[{index}] must be an object")
        if row.get("eligible") is not True:
            continue
        policies = row.get("required_policies")
        if (
            not isinstance(policies, list)
            or not policies
            or any(
                not isinstance(policy, str)
                or policy not in review.POLICIES
                for policy in policies
            )
        ):
            raise ValueError(
                f"eligible ledger row {index} has invalid policies"
            )
        eligible_count += 1
        required.update(policies)
    if eligible_count <= 0:
        raise ValueError("ledger has no eligible rows to approve")
    expected_count = (
        ledger.get("summary", {}).get("total_eligible_count")
        if isinstance(ledger.get("summary"), dict)
        else None
    )
    if expected_count != eligible_count:
        raise ValueError("ledger eligible row count is internally inconsistent")
    approved = [
        policy
        for policy in review.POLICIES
        if policy in required
    ]
    return {
        "schema_version": 1,
        "decision": "approve",
        "candidate_sha256": ledger["candidate_sha256"],
        "cohort_digest": ledger["cohort_digest"],
        "ledger_sha256": ledger_sha256,
        "reviewer_id": reviewer_id,
        "reviewed_at": normalized_at,
        "note": note,
        "approved_policies": approved,
        "decision_mode": "automatic_policy_engine",
        "manual_row_review_performed": False,
        "eligible_row_count": eligible_count,
        "authorization_boundary": (
            "Approval applies only to review-eligible policy rows in the "
            "frozen ledger. It does not waive SQL, API, object, permission, "
            "search, UI, rollback, or release gates."
        ),
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--ledger", type=pathlib.Path, required=True)
    parser.add_argument(
        "--decision-template",
        type=pathlib.Path,
        required=True,
    )
    parser.add_argument("--reviewer-id", type=int, required=True)
    parser.add_argument("--reviewed-at", required=True)
    parser.add_argument("--authorization-note", required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    args = parser.parse_args()
    if args.output.exists() or args.output.is_symlink():
        raise FileExistsError(
            "refusing to overwrite automatic review decision"
        )
    ledger_bytes = args.ledger.read_bytes()
    ledger = read_object(args.ledger, "ledger")
    template = read_object(
        args.decision_template,
        "decision template",
    )
    decision = build_decision(
        ledger,
        template,
        ledger_sha256=hashlib.sha256(ledger_bytes).hexdigest(),
        reviewer_id=args.reviewer_id,
        reviewed_at=args.reviewed_at,
        authorization_note=args.authorization_note,
    )
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_bytes(canonical_bytes(decision))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
