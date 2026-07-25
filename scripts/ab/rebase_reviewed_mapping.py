#!/usr/bin/env python3
"""Safely carry unchanged V8 review decisions onto a regenerated candidate."""

from __future__ import annotations

import argparse
import copy
import hashlib
import json
import os
import pathlib
import sys
import tempfile
from collections import Counter
from typing import Any, Iterable


REVIEW_FIELDS = {
    "confidence",
    "confirmed_by",
    "confirmed_at",
    "confirmation_note",
    "manifest_row_hash",
}
ROW_COLLECTIONS = (
    (
        "asset_recovery",
        "asset_recoveries",
        lambda row: (int(row["missing_task_asset_id"]),),
    ),
    ("planning", "planning_tasks", lambda row: (int(row["task_id"]),)),
    (
        "organization",
        "organization_mappings",
        lambda row: (str(row["subject_type"]), int(row["subject_id"])),
    ),
    (
        "access",
        "access_decisions",
        lambda row: (int(row["user_id"]), str(row["legacy_role"])),
    ),
    (
        "task_state",
        "task_state_decisions",
        lambda row: (int(row["task_id"]),),
    ),
)


def canonical_json(value: Any) -> str:
    return json.dumps(
        value,
        ensure_ascii=False,
        separators=(",", ":"),
        sort_keys=True,
    )


def sha256_json(value: Any) -> str:
    return hashlib.sha256(canonical_json(value).encode("utf-8")).hexdigest()


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def business_content(row: dict[str, Any]) -> dict[str, Any]:
    return {
        key: value
        for key, value in row.items()
        if key not in REVIEW_FIELDS
    }


def canonical_row_hash(row: dict[str, Any]) -> str:
    return sha256_json(
        {
            key: value
            for key, value in row.items()
            if key != "manifest_row_hash"
        }
    )


def validate_row_hash(path: str, row: dict[str, Any]) -> None:
    expected = row.get("manifest_row_hash")
    if not isinstance(expected, str) or len(expected) != 64:
        raise ValueError(f"{path}.manifest_row_hash must be SHA-256")
    if canonical_row_hash(row) != expected:
        raise ValueError(f"{path}.manifest_row_hash is stale or invalid")


def complete_confirmation(row: dict[str, Any]) -> bool:
    return bool(
        row.get("confidence") == "confirmed_auto"
        and isinstance(row.get("confirmed_by"), int)
        and not isinstance(row.get("confirmed_by"), bool)
        and row["confirmed_by"] > 0
        and str(row.get("confirmed_at") or "").strip()
        and str(row.get("confirmed_at")) != "0001-01-01T00:00:00Z"
        and str(row.get("confirmation_note") or "").strip()
        and not row.get("blockers")
    )


def carry_confirmation(
    baseline: dict[str, Any],
    candidate: dict[str, Any],
) -> None:
    candidate["confidence"] = "confirmed_auto"
    candidate["confirmed_by"] = baseline["confirmed_by"]
    candidate["confirmed_at"] = baseline["confirmed_at"]
    candidate["confirmation_note"] = baseline["confirmation_note"]
    candidate["manifest_row_hash"] = canonical_row_hash(candidate)


def index_rows(
    rows: Any,
    key_fn,
    path: str,
) -> dict[tuple[Any, ...], dict[str, Any]]:
    if not isinstance(rows, list):
        raise ValueError(f"{path} must be an array")
    result = {}
    for index, row in enumerate(rows):
        if not isinstance(row, dict):
            raise ValueError(f"{path}[{index}] must be an object")
        key = key_fn(row)
        if key in result:
            raise ValueError(f"{path} contains duplicate key {key}")
        validate_row_hash(f"{path}[{index}]", row)
        result[key] = row
    return result


def revision_key(
    resource: dict[str, Any],
    revision: dict[str, Any],
) -> tuple[Any, ...]:
    return (
        int(resource["task_id"]),
        str(resource["scope_kind"]),
        int(resource["scope_ref_id"]),
        int(revision["revision_no"]),
    )


def index_revisions(
    mapping: dict[str, Any],
    label: str,
) -> tuple[
    dict[tuple[Any, ...], dict[str, Any]],
    dict[tuple[Any, ...], dict[str, Any]],
]:
    resources = mapping.get("resources")
    if not isinstance(resources, list):
        raise ValueError(f"{label}.resources must be an array")
    revisions = {}
    contexts = {}
    for resource_index, resource in enumerate(resources):
        if not isinstance(resource, dict):
            raise ValueError(
                f"{label}.resources[{resource_index}] must be an object"
            )
        resource_key = (
            int(resource["task_id"]),
            str(resource["scope_kind"]),
            int(resource["scope_ref_id"]),
        )
        if resource_key in contexts:
            raise ValueError(
                f"{label}.resources contains duplicate key {resource_key}"
            )
        history = resource.get("history")
        if not isinstance(history, list):
            raise ValueError(
                f"{label}.resources[{resource_index}].history must be an array"
            )
        contexts[resource_key] = resource
        for revision_index, revision in enumerate(history):
            if not isinstance(revision, dict):
                raise ValueError(
                    f"{label}.resources[{resource_index}].history"
                    f"[{revision_index}] must be an object"
                )
            key = revision_key(resource, revision)
            if key in revisions:
                raise ValueError(
                    f"{label}.resources contains duplicate revision key {key}"
                )
            validate_row_hash(
                f"{label}.resources[{resource_index}].history[{revision_index}]",
                revision,
            )
            revisions[key] = revision
    return revisions, contexts


def rebase_mapping(
    baseline: dict[str, Any],
    candidate: dict[str, Any],
) -> tuple[dict[str, Any], dict[str, Any]]:
    if not isinstance(baseline, dict) or not isinstance(candidate, dict):
        raise ValueError("baseline and candidate must be JSON objects")
    reviewed = copy.deepcopy(candidate)
    counts: Counter[str] = Counter()

    baseline_revisions, baseline_contexts = index_revisions(
        baseline, "baseline"
    )
    candidate_revisions, candidate_contexts = index_revisions(
        reviewed, "candidate"
    )
    for key, row in candidate_revisions.items():
        counts["revision.considered"] += 1
        baseline_row = baseline_revisions.get(key)
        if baseline_row is None:
            counts["revision.missing_baseline"] += 1
            continue
        resource_key = key[:3]
        candidate_resource = candidate_contexts[resource_key]
        baseline_resource = baseline_contexts.get(resource_key)
        if any(
            revision.get("confidence") == "hard_blocked"
            for revision in candidate_resource["history"]
        ):
            counts["revision.hard_sibling"] += 1
            continue
        if not complete_confirmation(baseline_row):
            counts["revision.baseline_not_confirmed"] += 1
            continue
        if row.get("confidence") != "proposed_review":
            counts["revision.candidate_not_proposed"] += 1
            continue
        if business_content(baseline_row) != business_content(row):
            counts["revision.business_changed"] += 1
            continue
        # Pointer context is checked only after the immutable row itself is
        # proven identical and previously confirmed.  This preserves the same
        # fail-closed behavior while preventing a changed pointer from hiding
        # a more fundamental non-confirmed or business-changed reason in the
        # evidence counts.
        if (
            baseline_resource is None
            or baseline_resource.get("working_revision_no")
            != candidate_resource.get("working_revision_no")
            or baseline_resource.get("finalized_revision_no")
            != candidate_resource.get("finalized_revision_no")
        ):
            counts["revision.pointer_context_changed"] += 1
            continue
        carry_confirmation(baseline_row, row)
        counts["revision.inherited"] += 1

    for kind, field, key_fn in ROW_COLLECTIONS:
        baseline_rows = index_rows(
            baseline.get(field, []),
            key_fn,
            f"baseline.{field}",
        )
        candidate_rows = index_rows(
            reviewed.get(field, []),
            key_fn,
            f"candidate.{field}",
        )
        for key, row in candidate_rows.items():
            counts[f"{kind}.considered"] += 1
            baseline_row = baseline_rows.get(key)
            if baseline_row is None:
                counts[f"{kind}.missing_baseline"] += 1
                continue
            if not complete_confirmation(baseline_row):
                counts[f"{kind}.baseline_not_confirmed"] += 1
                continue
            if row.get("confidence") != "proposed_review":
                counts[f"{kind}.candidate_not_proposed"] += 1
                continue
            if business_content(baseline_row) != business_content(row):
                counts[f"{kind}.business_changed"] += 1
                continue
            carry_confirmation(baseline_row, row)
            counts[f"{kind}.inherited"] += 1

    evidence = {
        "version": 1,
        "baseline_mapping_sha256": sha256_json(baseline),
        "candidate_mapping_sha256": sha256_json(candidate),
        "reviewed_mapping_sha256": sha256_json(reviewed),
        "counts": dict(sorted(counts.items())),
    }
    return reviewed, evidence


def read_json(path: pathlib.Path) -> tuple[dict[str, Any], bytes]:
    raw = path.read_bytes()
    try:
        value = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise ValueError(f"{path} is not valid JSON") from exc
    if not isinstance(value, dict):
        raise ValueError(f"{path} must contain a JSON object")
    return value, raw


def atomic_write(path: pathlib.Path, payload: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary_name = tempfile.mkstemp(
        prefix=f".{path.name}.",
        suffix=".tmp",
        dir=path.parent,
    )
    temporary = pathlib.Path(temporary_name)
    try:
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if temporary.exists():
            temporary.unlink()


def parse_args(argv: Iterable[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--baseline", required=True)
    parser.add_argument("--candidate", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--evidence-output", required=True)
    return parser.parse_args(argv)


def main(argv: Iterable[str] | None = None) -> int:
    args = parse_args(argv)
    baseline_path = pathlib.Path(args.baseline).resolve()
    candidate_path = pathlib.Path(args.candidate).resolve()
    output_path = pathlib.Path(args.output).resolve()
    evidence_path = pathlib.Path(args.evidence_output).resolve()
    if baseline_path == candidate_path:
        raise ValueError("baseline and candidate must be distinct files")
    if output_path in {baseline_path, candidate_path}:
        raise ValueError("output must not overwrite an input")
    if evidence_path in {baseline_path, candidate_path, output_path}:
        raise ValueError(
            "evidence output must be distinct from inputs and mapping output"
        )

    baseline, baseline_raw = read_json(baseline_path)
    candidate, candidate_raw = read_json(candidate_path)
    reviewed, evidence = rebase_mapping(baseline, candidate)
    evidence["baseline_file_sha256"] = sha256_bytes(baseline_raw)
    evidence["candidate_file_sha256"] = sha256_bytes(candidate_raw)
    mapping_payload = (
        json.dumps(reviewed, ensure_ascii=False, indent=2) + "\n"
    ).encode("utf-8")
    evidence["reviewed_file_sha256"] = sha256_bytes(mapping_payload)
    evidence_payload = (
        json.dumps(evidence, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
    ).encode("utf-8")
    atomic_write(output_path, mapping_payload)
    atomic_write(evidence_path, evidence_payload)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError) as exc:
        print(str(exc), file=sys.stderr)
        raise SystemExit(2)
