#!/usr/bin/env python3
"""Prepare and apply an exact administrator decision for source bundles.

This tool is deliberately offline and file-only.  ``prepare`` validates the
frozen seven-bundle/twenty-two-member candidate plus read-only Clone B ID
allocation evidence and emits a hash-bound decision template.  ``apply``
reconstructs that exact template and accepts only an explicit administrator
``APPROVED`` decision before emitting the CONFIRMED manifest consumed by
``run_scoped_bundle_materializer.py``.

It never connects to a database, allocates IDs by writing state, downloads
objects, materializes ZIPs, or turns a proposed review into an approval.
"""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
import pathlib
import re
import tempfile
import uuid
from typing import Any


SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
RUN_ID_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{2,127}$")
DATABASE_RE = re.compile(r"^ab_[A-Za-z0-9_]*_b$")
RFC3339_UTC_RE = re.compile(
    r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,6})?Z$"
)
ALLOWED_SCOPE_KINDS = {"task", "sku", "retouch_requirement"}
EXPECTED_BUNDLE_COUNT = 7
EXPECTED_MEMBER_COUNT = 22
STORAGE_REF_NAMESPACE = uuid.UUID("91fb514a-5ec6-5eaa-8d99-4635bd8bd1d4")

DECISION_KEYS = {
    "schema_version",
    "decision",
    "reviewer_id",
    "approved_at",
    "note",
    "database",
    "run_id",
    "candidate_file_sha256",
    "source_candidate_sha256",
    "mapping_sha256",
    "allocation_evidence_sha256",
    "decision_template_sha256",
    "bundle_count",
    "member_count",
}


def canonical_bytes(value: object) -> bytes:
    return (
        json.dumps(
            value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
        )
        + "\n"
    ).encode("utf-8")


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def read_json_object(path: pathlib.Path, label: str) -> dict[str, Any]:
    if not path.is_file() or path.is_symlink():
        raise ValueError(f"{label} must be an existing non-symlink file")
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ValueError(f"{label} must contain valid UTF-8 JSON") from exc
    if not isinstance(value, dict):
        raise ValueError(f"{label} must contain a JSON object")
    return value


def validate_mapping(mapping: dict[str, Any]) -> None:
    if mapping.get("version") != 2:
        raise ValueError("mapping.version must be 2")
    resources = mapping.get("resources")
    if not isinstance(resources, list):
        raise ValueError("mapping.resources must be an array")


def positive_integer(value: object, label: str, *, allow_zero: bool = False) -> int:
    if isinstance(value, bool) or not isinstance(value, int):
        raise ValueError(f"{label} must be an integer")
    minimum = 0 if allow_zero else 1
    if value < minimum:
        raise ValueError(f"{label} must be >= {minimum}")
    return value


def validate_candidate(candidate: dict[str, Any]) -> list[dict[str, Any]]:
    if candidate.get("schema_version") != 1:
        raise ValueError("candidate.schema_version must be 1")
    if candidate.get("status") != "PROPOSED_REVIEW":
        raise ValueError("candidate.status must remain PROPOSED_REVIEW")
    source_hash = str(candidate.get("source_candidate_sha256") or "")
    if not SHA256_RE.fullmatch(source_hash):
        raise ValueError("candidate.source_candidate_sha256 must be SHA-256")
    bundles = candidate.get("bundles")
    if (
        candidate.get("bundle_count") != EXPECTED_BUNDLE_COUNT
        or not isinstance(bundles, list)
        or len(bundles) != EXPECTED_BUNDLE_COUNT
    ):
        raise ValueError("candidate must contain exactly 7 bundles")
    if candidate.get("member_count") != EXPECTED_MEMBER_COUNT:
        raise ValueError("candidate.member_count must be exactly 22")

    seen_scopes: set[tuple[int, str, int, int]] = set()
    seen_task_asset_ids: set[int] = set()
    member_count = 0
    for bundle_index, bundle in enumerate(bundles):
        label = f"candidate.bundles[{bundle_index}]"
        if not isinstance(bundle, dict):
            raise ValueError(f"{label} must be an object")
        if (
            bundle.get("confidence") != "proposed_review"
            or bundle.get("requires_human_member_confirmation") is not True
            or bundle.get("all_members_exist_and_hash_verified") is not True
            or bundle.get("bundle_task_asset_id") is not None
        ):
            raise ValueError(
                f"{label} review state drifted or is not an unallocated proposal"
            )
        task_id = positive_integer(bundle.get("task_id"), f"{label}.task_id")
        scope_kind = str(bundle.get("scope_kind") or "")
        scope_ref_id = positive_integer(
            bundle.get("scope_ref_id"),
            f"{label}.scope_ref_id",
            allow_zero=scope_kind == "task",
        )
        revision_no = positive_integer(
            bundle.get("revision_no"), f"{label}.revision_no"
        )
        if scope_kind not in ALLOWED_SCOPE_KINDS:
            raise ValueError(f"{label}.scope_kind is invalid")
        if (scope_kind == "task" and scope_ref_id != 0) or (
            scope_kind != "task" and scope_ref_id <= 0
        ):
            raise ValueError(f"{label} scope is invalid")
        scope = (task_id, scope_kind, scope_ref_id, revision_no)
        if scope in seen_scopes:
            raise ValueError(f"{label} duplicates a task/scope/revision")
        seen_scopes.add(scope)

        members = bundle.get("ordered_members")
        if not isinstance(members, list) or len(members) < 2:
            raise ValueError(f"{label}.ordered_members must contain at least 2 members")
        seen_in_bundle: set[int] = set()
        for member_index, member in enumerate(members):
            member_label = f"{label}.ordered_members[{member_index}]"
            if not isinstance(member, dict):
                raise ValueError(f"{member_label} must be an object")
            if member.get("confirmed") is not False:
                raise ValueError(f"{member_label}.confirmed must remain false")
            if member.get("task_id") != task_id:
                raise ValueError(f"{member_label} belongs to another task")
            task_asset_id = positive_integer(
                member.get("task_asset_id"), f"{member_label}.task_asset_id"
            )
            positive_integer(member.get("asset_id"), f"{member_label}.asset_id")
            positive_integer(
                member.get("size"), f"{member_label}.size", allow_zero=True
            )
            if task_asset_id in seen_in_bundle or task_asset_id in seen_task_asset_ids:
                raise ValueError(f"{member_label}.task_asset_id is duplicated")
            digest = str(member.get("sha256") or "")
            if not SHA256_RE.fullmatch(digest):
                raise ValueError(f"{member_label}.sha256 is invalid")
            for field in (
                "object_key",
                "original_file_name",
                "storage_ref_id",
                "source_stage",
            ):
                if not str(member.get(field) or "").strip():
                    raise ValueError(f"{member_label}.{field} is empty")
            evidence = member.get("evidence_event_ids")
            if (
                not isinstance(evidence, list)
                or not evidence
                or any(not isinstance(item, str) or not item.strip() for item in evidence)
            ):
                raise ValueError(f"{member_label}.evidence_event_ids is invalid")
            seen_in_bundle.add(task_asset_id)
            seen_task_asset_ids.add(task_asset_id)
            member_count += 1
    if member_count != EXPECTED_MEMBER_COUNT:
        raise ValueError("candidate must contain exactly 22 ordered members")
    return bundles


def validate_allocation(
    allocation: dict[str, Any],
    bundles: list[dict[str, Any]],
) -> dict[str, Any]:
    allowed = {
        "schema_version",
        "status",
        "database",
        "run_id",
        "max_task_asset_id",
        "max_design_asset_id",
        "read_only",
        "query_evidence",
    }
    unknown = set(allocation) - allowed
    if unknown:
        raise ValueError(
            "allocation evidence has unsupported fields: " + ", ".join(sorted(unknown))
        )
    if allocation.get("schema_version") != 1:
        raise ValueError("allocation.schema_version must be 1")
    if allocation.get("status") != "FROZEN":
        raise ValueError("allocation.status must be FROZEN")
    if allocation.get("read_only") is not True:
        raise ValueError("allocation.read_only must be true")
    database = str(allocation.get("database") or "")
    if not DATABASE_RE.fullmatch(database):
        raise ValueError("allocation.database must be an exact ab_*_b Clone B name")
    run_id = str(allocation.get("run_id") or "")
    if not RUN_ID_RE.fullmatch(run_id):
        raise ValueError("allocation.run_id is invalid")
    max_task_asset_id = positive_integer(
        allocation.get("max_task_asset_id"),
        "allocation.max_task_asset_id",
        allow_zero=True,
    )
    max_design_asset_id = positive_integer(
        allocation.get("max_design_asset_id"),
        "allocation.max_design_asset_id",
        allow_zero=True,
    )
    query_evidence = allocation.get("query_evidence")
    if not isinstance(query_evidence, dict) or not query_evidence:
        raise ValueError("allocation.query_evidence must be a non-empty object")

    existing_task_ids = {
        member["task_asset_id"]
        for bundle in bundles
        for member in bundle["ordered_members"]
    }
    existing_asset_ids = {
        member["asset_id"]
        for bundle in bundles
        for member in bundle["ordered_members"]
    }
    if existing_task_ids and max(existing_task_ids) > max_task_asset_id:
        raise ValueError("allocation max_task_asset_id is behind candidate members")
    if existing_asset_ids and max(existing_asset_ids) > max_design_asset_id:
        raise ValueError("allocation max_design_asset_id is behind candidate members")

    task_ids = list(
        range(
            max_task_asset_id + 1,
            max_task_asset_id + EXPECTED_BUNDLE_COUNT + 1,
        )
    )
    asset_ids = list(
        range(
            max_design_asset_id + 1,
            max_design_asset_id + EXPECTED_BUNDLE_COUNT + 1,
        )
    )
    if set(task_ids) & existing_task_ids or set(asset_ids) & existing_asset_ids:
        raise ValueError("allocated IDs conflict with candidate member IDs")
    return {
        "database": database,
        "run_id": run_id,
        "max_task_asset_id": max_task_asset_id,
        "max_design_asset_id": max_design_asset_id,
        "bundle_task_asset_ids": task_ids,
        "bundle_asset_ids": asset_ids,
    }


def storage_ref_id(
    allocation: dict[str, Any],
    candidate_file_sha256: str,
    bundle: dict[str, Any],
    bundle_task_asset_id: int,
    bundle_asset_id: int,
) -> str:
    name = "|".join(
        (
            allocation["database"],
            allocation["run_id"],
            candidate_file_sha256,
            str(bundle["task_id"]),
            str(bundle["scope_kind"]),
            str(bundle["scope_ref_id"]),
            str(bundle["revision_no"]),
            str(bundle_task_asset_id),
            str(bundle_asset_id),
        )
    )
    return str(uuid.uuid5(STORAGE_REF_NAMESPACE, name))


def allocation_rows(
    bundles: list[dict[str, Any]],
    allocation: dict[str, Any],
    candidate_file_sha256: str,
) -> list[dict[str, Any]]:
    rows = []
    for index, bundle in enumerate(bundles):
        task_asset_id = allocation["bundle_task_asset_ids"][index]
        asset_id = allocation["bundle_asset_ids"][index]
        rows.append(
            {
                "task_id": bundle["task_id"],
                "scope_kind": bundle["scope_kind"],
                "scope_ref_id": bundle["scope_ref_id"],
                "revision_no": bundle["revision_no"],
                "bundle_task_asset_id": task_asset_id,
                "bundle_asset_id": asset_id,
                "bundle_storage_ref_id": storage_ref_id(
                    allocation,
                    candidate_file_sha256,
                    bundle,
                    task_asset_id,
                    asset_id,
                ),
            }
        )
    expected_task_ids = list(
        range(
            allocation["max_task_asset_id"] + 1,
            allocation["max_task_asset_id"] + EXPECTED_BUNDLE_COUNT + 1,
        )
    )
    expected_asset_ids = list(
        range(
            allocation["max_design_asset_id"] + 1,
            allocation["max_design_asset_id"] + EXPECTED_BUNDLE_COUNT + 1,
        )
    )
    if [row["bundle_task_asset_id"] for row in rows] != expected_task_ids:
        raise ValueError("bundle task asset allocation is not exactly consecutive")
    if [row["bundle_asset_id"] for row in rows] != expected_asset_ids:
        raise ValueError("bundle design asset allocation is not exactly consecutive")
    if len({row["bundle_storage_ref_id"] for row in rows}) != len(rows):
        raise ValueError("deterministic bundle storage references collided")
    return rows


def decision_template(
    candidate: dict[str, Any],
    candidate_file_sha256: str,
    mapping_sha256: str,
    allocation: dict[str, Any],
    allocation_evidence_sha256: str,
    rows: list[dict[str, Any]],
) -> dict[str, Any]:
    return {
        "schema_version": 1,
        "status": "PENDING_ADMIN_REVIEW",
        "decision_instructions": {
            "required_decision": "APPROVED",
            "reviewer_id": "positive integer",
            "approved_at": "RFC3339 UTC timestamp ending in Z",
            "note": "non-empty human rationale",
            "automatic_confirmation_forbidden": True,
        },
        "database": allocation["database"],
        "run_id": allocation["run_id"],
        "candidate_file_sha256": candidate_file_sha256,
        "source_candidate_sha256": candidate["source_candidate_sha256"],
        "mapping_sha256": mapping_sha256,
        "allocation_evidence_sha256": allocation_evidence_sha256,
        "bundle_count": EXPECTED_BUNDLE_COUNT,
        "member_count": EXPECTED_MEMBER_COUNT,
        "allocations": rows,
        "decision": "PENDING_REVIEW",
        "reviewer_id": None,
        "approved_at": None,
        "note": None,
    }


def validate_decision(
    decision: dict[str, Any],
    template: dict[str, Any],
    template_sha256: str,
) -> None:
    if set(decision) != DECISION_KEYS:
        missing = sorted(DECISION_KEYS - set(decision))
        extra = sorted(set(decision) - DECISION_KEYS)
        raise ValueError(
            f"decision fields must be exact; missing={missing}, extra={extra}"
        )
    expected = {
        "schema_version": 1,
        "database": template["database"],
        "run_id": template["run_id"],
        "candidate_file_sha256": template["candidate_file_sha256"],
        "source_candidate_sha256": template["source_candidate_sha256"],
        "mapping_sha256": template["mapping_sha256"],
        "allocation_evidence_sha256": template["allocation_evidence_sha256"],
        "decision_template_sha256": template_sha256,
        "bundle_count": EXPECTED_BUNDLE_COUNT,
        "member_count": EXPECTED_MEMBER_COUNT,
    }
    for key, value in expected.items():
        if decision.get(key) != value:
            raise ValueError(f"decision.{key} does not match the prepared template")
    if decision.get("decision") != "APPROVED":
        raise ValueError("decision.decision must be exactly APPROVED")
    positive_integer(decision.get("reviewer_id"), "decision.reviewer_id")
    approved_at = str(decision.get("approved_at") or "")
    if not RFC3339_UTC_RE.fullmatch(approved_at):
        raise ValueError("decision.approved_at must be an RFC3339 UTC timestamp")
    try:
        dt.datetime.fromisoformat(approved_at[:-1] + "+00:00")
    except ValueError as exc:
        raise ValueError("decision.approved_at is not a real timestamp") from exc
    if not str(decision.get("note") or "").strip():
        raise ValueError("decision.note must be non-empty")


def confirmed_manifest(
    candidate: dict[str, Any],
    template: dict[str, Any],
    decision: dict[str, Any],
    decision_sha256: str,
    template_sha256: str,
) -> dict[str, Any]:
    bundles = []
    for candidate_bundle, allocation in zip(
        candidate["bundles"], template["allocations"], strict=True
    ):
        members = []
        for candidate_member in candidate_bundle["ordered_members"]:
            member = dict(candidate_member)
            member["confirmed"] = True
            members.append(member)
        bundles.append(
            {
                "task_id": candidate_bundle["task_id"],
                "scope_kind": candidate_bundle["scope_kind"],
                "scope_ref_id": candidate_bundle["scope_ref_id"],
                "revision_no": candidate_bundle["revision_no"],
                "bundle_task_asset_id": allocation["bundle_task_asset_id"],
                "bundle_asset_id": allocation["bundle_asset_id"],
                "bundle_storage_ref_id": allocation["bundle_storage_ref_id"],
                "confirmed": True,
                "ordered_members": members,
            }
        )
    return {
        "schema_version": 1,
        "status": "CONFIRMED",
        "run_id": template["run_id"],
        "source_candidate_sha256": candidate["source_candidate_sha256"],
        "candidate_file_sha256": template["candidate_file_sha256"],
        "mapping_sha256": template["mapping_sha256"],
        "allocation_evidence_sha256": template["allocation_evidence_sha256"],
        "decision_template_sha256": template_sha256,
        "decision_sha256": decision_sha256,
        "confirmed_by": decision["reviewer_id"],
        "confirmed_at": decision["approved_at"],
        "confirmation_note": decision["note"].strip(),
        "bundle_count": EXPECTED_BUNDLE_COUNT,
        "member_count": EXPECTED_MEMBER_COUNT,
        "bundles": bundles,
    }


def evidence_document(
    action: str,
    output: dict[str, Any],
    output_sha256: str,
    candidate_file_sha256: str,
    mapping_sha256: str,
    allocation_evidence_sha256: str,
    template_sha256: str,
    decision_sha256: str | None = None,
) -> dict[str, Any]:
    value = {
        "schema_version": 1,
        "status": "PREPARED" if action == "prepare" else "CONFIRMED",
        "action": action,
        "candidate_file_sha256": candidate_file_sha256,
        "source_candidate_sha256": output["source_candidate_sha256"],
        "mapping_sha256": mapping_sha256,
        "allocation_evidence_sha256": allocation_evidence_sha256,
        "decision_template_sha256": template_sha256,
        "output_sha256": output_sha256,
        "bundle_count": EXPECTED_BUNDLE_COUNT,
        "member_count": EXPECTED_MEMBER_COUNT,
        "database_connection_performed": False,
        "database_write_performed": False,
        "automatic_confirmation_performed": False,
    }
    if decision_sha256 is not None:
        value["decision_sha256"] = decision_sha256
        value["reviewer_id"] = output["confirmed_by"]
        value["approved_at"] = output["confirmed_at"]
    return value


def write_pair(
    output_path: pathlib.Path,
    output: dict[str, Any],
    evidence_path: pathlib.Path,
    evidence: dict[str, Any],
) -> None:
    if output_path.resolve() == evidence_path.resolve():
        raise ValueError("--output and --evidence must be different paths")
    payloads = (
        (output_path, canonical_bytes(output)),
        (evidence_path, canonical_bytes(evidence)),
    )
    for path, encoded in payloads:
        if path.exists():
            if not path.is_file() or path.is_symlink() or path.read_bytes() != encoded:
                raise FileExistsError(f"refusing to overwrite different artifact: {path}")
    pending = [(path, encoded) for path, encoded in payloads if not path.exists()]
    temporaries: list[tuple[pathlib.Path, pathlib.Path]] = []
    try:
        for path, encoded in pending:
            path.parent.mkdir(parents=True, exist_ok=True)
            with tempfile.NamedTemporaryFile(
                dir=path.parent,
                prefix=path.name + ".",
                suffix=".tmp",
                delete=False,
            ) as handle:
                temporary = pathlib.Path(handle.name)
                handle.write(encoded)
                handle.flush()
                os.fsync(handle.fileno())
            temporaries.append((temporary, path))
        for temporary, path in temporaries:
            os.replace(temporary, path)
    finally:
        for temporary, _ in temporaries:
            temporary.unlink(missing_ok=True)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=("prepare", "apply"))
    parser.add_argument("--candidate", type=pathlib.Path, required=True)
    parser.add_argument("--mapping", type=pathlib.Path, required=True)
    parser.add_argument(
        "--allocation-evidence", type=pathlib.Path, required=True
    )
    parser.add_argument("--decision", type=pathlib.Path)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--evidence", type=pathlib.Path, required=True)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    candidate = read_json_object(args.candidate, "candidate")
    bundles = validate_candidate(candidate)
    mapping = read_json_object(args.mapping, "mapping")
    validate_mapping(mapping)
    allocation_raw = read_json_object(
        args.allocation_evidence, "allocation evidence"
    )
    allocation = validate_allocation(allocation_raw, bundles)
    candidate_hash = sha256_file(args.candidate)
    mapping_hash = sha256_file(args.mapping)
    allocation_hash = sha256_file(args.allocation_evidence)
    rows = allocation_rows(bundles, allocation, candidate_hash)
    template = decision_template(
        candidate,
        candidate_hash,
        mapping_hash,
        allocation,
        allocation_hash,
        rows,
    )
    template_hash = sha256_bytes(canonical_bytes(template))

    if args.action == "prepare":
        if args.decision is not None:
            raise ValueError("--decision is forbidden for prepare")
        evidence = evidence_document(
            "prepare",
            {
                **template,
                "source_candidate_sha256": candidate[
                    "source_candidate_sha256"
                ],
            },
            template_hash,
            candidate_hash,
            mapping_hash,
            allocation_hash,
            template_hash,
        )
        write_pair(args.output, template, args.evidence, evidence)
        return 0

    if args.decision is None:
        raise ValueError("--decision is required for apply")
    decision = read_json_object(args.decision, "decision")
    validate_decision(decision, template, template_hash)
    decision_hash = sha256_file(args.decision)
    manifest = confirmed_manifest(
        candidate, template, decision, decision_hash, template_hash
    )
    manifest_hash = sha256_bytes(canonical_bytes(manifest))
    evidence = evidence_document(
        "apply",
        manifest,
        manifest_hash,
        candidate_hash,
        mapping_hash,
        allocation_hash,
        template_hash,
        decision_hash,
    )
    write_pair(args.output, manifest, args.evidence, evidence)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
