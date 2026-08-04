#!/usr/bin/env python3
"""Build an auditable automatic source-bundle manifest from frozen evidence.

This tool is intentionally read-only with respect to MySQL. It accepts only
mapping rows whose sole blocker is the deterministic ZIP requirement, resolves
the exact ordered member rows and their upload-completion events, allocates
consecutive IDs from the frozen clone maxima, and emits the CONFIRMED manifest
consumed by ``run_scoped_bundle_materializer.py``.

``confirmed_by`` identifies the administrator who authorized automatic
adjudication. The confirmation note explicitly records that no row-by-row
human review was performed.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import subprocess
import uuid
from typing import Any


BUNDLE_BLOCKER = "multiple source assets require a reviewed deterministic ZIP bundle"
BUNDLE_RESOLVABLE_BLOCKERS = {
    BUNDLE_BLOCKER,
    "design revision has no lifecycle-valid source asset",
}
STORAGE_REF_NAMESPACE = uuid.UUID("07212a18-6e57-54aa-bad2-f7e79673ffdc")
UPLOAD_COMPLETED = "task.asset.upload_session.completed"


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


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def read_json_object(path: pathlib.Path, label: str) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{label} must contain a JSON object")
    return value


def read_object_manifest(path: pathlib.Path) -> dict[int, dict[str, Any]]:
    rows: dict[int, dict[str, Any]] = {}
    with path.open("r", encoding="utf-8") as handle:
        for line_number, raw in enumerate(handle, 1):
            value = json.loads(raw)
            if not isinstance(value, dict):
                raise ValueError(
                    f"object manifest line {line_number} must be an object"
                )
            key = str(value.get("entity_key") or "")
            if not key.startswith("task_asset:"):
                continue
            task_asset_id = int(key.removeprefix("task_asset:"))
            if task_asset_id in rows:
                raise ValueError(
                    f"object manifest duplicates task_asset:{task_asset_id}"
                )
            rows[task_asset_id] = value
    return rows


def mysql_json_rows(
    mysql: str,
    database: str,
    query: str,
) -> list[dict[str, Any]]:
    completed = subprocess.run(
        [
            mysql,
            "--default-character-set=utf8mb4",
            "--batch",
            "--raw",
            "--skip-column-names",
            "-e",
            query,
            database,
        ],
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    rows = []
    for line in completed.stdout.splitlines():
        if not line.strip():
            continue
        value = json.loads(line)
        if not isinstance(value, dict):
            raise ValueError("MySQL JSON query returned a non-object row")
        rows.append(value)
    return rows


def payload_asset_version_ids(payload: Any) -> list[int]:
    if isinstance(payload, str):
        payload = json.loads(payload)
    if not isinstance(payload, dict):
        return []
    result: list[int] = []
    for key in (
        "asset_version_id",
        "asset_version_ids",
        "task_asset_id",
        "task_asset_ids",
    ):
        raw = payload.get(key)
        values = raw if isinstance(raw, list) else [raw]
        for value in values:
            try:
                parsed = int(value)
            except (TypeError, ValueError):
                continue
            if parsed > 0 and parsed not in result:
                result.append(parsed)
    return result


def bundle_rows(mapping: dict[str, Any]) -> list[dict[str, Any]]:
    if mapping.get("version") != 2:
        raise ValueError("mapping.version must be 2")
    rows = []
    for resource in mapping.get("resources") or []:
        for revision in resource.get("history") or []:
            if revision.get("confidence") != "hard_blocked":
                continue
            blockers = revision.get("blockers")
            candidate = revision.get("source_bundle_candidate")
            if (
                not isinstance(blockers, list)
                or BUNDLE_BLOCKER not in blockers
                or any(
                    blocker not in BUNDLE_RESOLVABLE_BLOCKERS
                    for blocker in blockers
                )
            ):
                raise ValueError(
                    "automatic bundle preparation found a non-bundle hard blocker"
                )
            if (
                not isinstance(candidate, dict)
                or candidate.get("ordering")
                != "completion_time_then_task_asset_id"
                or not isinstance(
                    candidate.get("ordered_member_task_asset_ids"),
                    list,
                )
                or len(candidate["ordered_member_task_asset_ids"]) < 2
            ):
                raise ValueError("bundle candidate membership is incomplete")
            rows.append(
                {
                    "task_id": int(resource["task_id"]),
                    "scope_kind": str(resource["scope_kind"]),
                    "scope_ref_id": int(resource["scope_ref_id"]),
                    "revision_no": int(revision["revision_no"]),
                    "source_stage": str(revision["source_stage"]),
                    "evidence_event_ids": list(
                        revision.get("evidence_event_ids") or []
                    ),
                    "member_ids": [
                        int(value)
                        for value in candidate[
                            "ordered_member_task_asset_ids"
                        ]
                    ],
                }
            )
    if not rows:
        raise ValueError("mapping contains no deterministic source bundles")
    return rows


def build_manifest(
    *,
    mapping: dict[str, Any],
    mapping_sha256: str,
    object_rows: dict[int, dict[str, Any]],
    task_asset_rows: dict[int, dict[str, Any]],
    completion_events: list[dict[str, Any]],
    max_task_asset_id: int,
    max_asset_id: int,
    run_id: str,
    confirmed_by: int,
    confirmed_at: str,
) -> dict[str, Any]:
    rows = bundle_rows(mapping)
    event_by_member: dict[int, list[str]] = {}
    for event in completion_events:
        event_id = f"task_event_log:{event['id']}"
        for task_asset_id in payload_asset_version_ids(event.get("payload")):
            event_by_member.setdefault(task_asset_id, []).append(event_id)

    bundles = []
    seen_members: set[int] = set()
    for index, row in enumerate(rows, 1):
        ordered_members = []
        allowed_evidence = set(row["evidence_event_ids"])
        for task_asset_id in row["member_ids"]:
            if task_asset_id in seen_members:
                raise ValueError(
                    f"task asset {task_asset_id} appears in multiple bundles"
                )
            asset = task_asset_rows.get(task_asset_id)
            obj = object_rows.get(task_asset_id)
            if asset is None or obj is None:
                raise ValueError(
                    f"task asset {task_asset_id} lacks frozen DB/object evidence"
                )
            evidence = [
                value
                for value in event_by_member.get(task_asset_id, [])
                if value in allowed_evidence
            ]
            if (
                int(asset.get("task_id") or 0) != row["task_id"]
                or asset.get("asset_type") != "source"
                or str(asset.get("upload_status") or "") != "uploaded"
                or str(asset.get("storage_ref_id") or "")
                != str(obj.get("storage_ref_id") or "")
                or int(obj.get("task_id") or 0) != row["task_id"]
                or int(obj.get("owner_id") or 0) != task_asset_id
                or obj.get("owner_kind") != "task_asset"
                or obj.get("status") not in {"active", "recorded"}
                or bool(obj.get("is_placeholder"))
                or not str(obj.get("sha256") or "")
                or int(obj.get("size") or 0) <= 0
                or not evidence
            ):
                raise ValueError(
                    f"task asset {task_asset_id} failed frozen bundle evidence validation"
                )
            ordered_members.append(
                {
                    "task_id": row["task_id"],
                    "task_asset_id": task_asset_id,
                    "asset_id": int(asset["asset_id"]),
                    "asset_type": "source",
                    "storage_ref_id": str(obj["storage_ref_id"]),
                    "object_key": str(obj["object_key"]),
                    "size": int(obj["size"]),
                    "mime_type_from_object": str(obj["mime_type"]),
                    "sha256": str(obj["sha256"]),
                    "original_file_name": str(
                        asset.get("original_filename")
                        or asset.get("file_name")
                        or f"task-asset-{task_asset_id}"
                    ),
                    "source_stage": row["source_stage"],
                    "evidence_event_ids": evidence,
                    "upload_status": "uploaded",
                    "object_status": str(obj["status"]),
                    "confirmed": True,
                }
            )
            seen_members.add(task_asset_id)
        bundle_task_asset_id = max_task_asset_id + index
        bundle_asset_id = max_asset_id + index
        storage_ref_name = "|".join(
            (
                run_id,
                mapping_sha256,
                str(row["task_id"]),
                row["scope_kind"],
                str(row["scope_ref_id"]),
                str(row["revision_no"]),
                str(bundle_task_asset_id),
                str(bundle_asset_id),
            )
        )
        bundles.append(
            {
                "task_id": row["task_id"],
                "scope_kind": row["scope_kind"],
                "scope_ref_id": row["scope_ref_id"],
                "revision_no": row["revision_no"],
                "bundle_task_asset_id": bundle_task_asset_id,
                "bundle_asset_id": bundle_asset_id,
                "bundle_storage_ref_id": str(
                    uuid.uuid5(STORAGE_REF_NAMESPACE, storage_ref_name)
                ),
                "ordered_members": ordered_members,
                "confirmed": True,
            }
        )
    return {
        "schema_version": 1,
        "status": "CONFIRMED",
        "run_id": run_id,
        "source_candidate_sha256": mapping_sha256,
        "mapping_sha256": mapping_sha256,
        "bundle_count": len(bundles),
        "member_count": len(seen_members),
        "confirmed_by": confirmed_by,
        "confirmed_at": confirmed_at,
        "confirmation_mode": "automatic_policy_engine",
        "confirmation_note": (
            "Admin authorized deterministic automatic adjudication; no "
            "row-by-row human review was performed. Member order is frozen "
            "from completed upload events."
        ),
        "bundles": bundles,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mapping", type=pathlib.Path, required=True)
    parser.add_argument(
        "--source-candidate",
        type=pathlib.Path,
        required=True,
    )
    parser.add_argument("--object-manifest", type=pathlib.Path, required=True)
    parser.add_argument(
        "--download-receipt",
        type=pathlib.Path,
        required=True,
    )
    parser.add_argument("--mysql", required=True)
    parser.add_argument("--database", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--confirmed-by", type=int, required=True)
    parser.add_argument("--confirmed-at", required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--evidence", type=pathlib.Path, required=True)
    args = parser.parse_args()
    if args.output.exists() or args.evidence.exists():
        raise FileExistsError("refusing to overwrite bundle manifest outputs")
    if args.confirmed_by <= 0:
        raise ValueError("confirmed-by must identify the authorizing admin")

    mapping = read_json_object(args.mapping, "mapping")
    rows = bundle_rows(mapping)
    source_candidate = read_json_object(
        args.source_candidate,
        "source candidate",
    )
    source_rows = bundle_rows(source_candidate)
    if source_rows != rows:
        raise ValueError(
            "rebased mapping bundle candidates drifted from the downloaded source candidate"
        )
    download_receipt = read_json_object(
        args.download_receipt,
        "download receipt",
    )
    if (
        download_receipt.get("schema_version") != 1
        or download_receipt.get("status") != "PASS"
        or download_receipt.get("remote_operation") != "GET"
        or download_receipt.get("remote_write_performed") is not False
        or download_receipt.get("mapping_sha256")
        != sha256_file(args.source_candidate)
        or download_receipt.get("hydrated_manifest_sha256")
        != sha256_file(args.object_manifest)
    ):
        raise ValueError(
            "download receipt does not bind the source candidate and hydrated objects"
        )
    member_ids = sorted(
        {
            task_asset_id
            for row in rows
            for task_asset_id in row["member_ids"]
        }
    )
    task_ids = sorted({row["task_id"] for row in rows})
    asset_query = (
        "SELECT JSON_OBJECT("
        "'id',id,'task_id',task_id,'asset_id',asset_id,"
        "'asset_type',asset_type,'upload_status',upload_status,"
        "'storage_ref_id',storage_ref_id,'file_name',file_name,"
        "'original_filename',original_filename"
        ") FROM task_assets WHERE id IN ("
        + ",".join(map(str, member_ids))
        + ") ORDER BY id"
    )
    event_query = (
        "SELECT JSON_OBJECT("
        "'id',id,'task_id',task_id,'payload',payload"
        ") FROM task_event_logs WHERE event_type='"
        + UPLOAD_COMPLETED
        + "' AND task_id IN ("
        + ",".join(map(str, task_ids))
        + ") ORDER BY task_id,sequence"
    )
    maxima_query = (
        "SELECT JSON_OBJECT("
        "'max_task_asset_id',COALESCE((SELECT MAX(id) FROM task_assets),0),"
        "'max_asset_id',COALESCE((SELECT MAX(id) FROM design_assets),0)"
        ")"
    )
    task_asset_rows = {
        int(row["id"]): row
        for row in mysql_json_rows(args.mysql, args.database, asset_query)
    }
    maxima = mysql_json_rows(
        args.mysql,
        args.database,
        maxima_query,
    )
    if len(maxima) != 1:
        raise ValueError("failed to freeze allocation maxima")
    mapping_sha256 = sha256_file(args.mapping)
    manifest = build_manifest(
        mapping=mapping,
        mapping_sha256=mapping_sha256,
        object_rows=read_object_manifest(args.object_manifest),
        task_asset_rows=task_asset_rows,
        completion_events=mysql_json_rows(
            args.mysql,
            args.database,
            event_query,
        ),
        max_task_asset_id=int(maxima[0]["max_task_asset_id"]),
        max_asset_id=int(maxima[0]["max_asset_id"]),
        run_id=args.run_id,
        confirmed_by=args.confirmed_by,
        confirmed_at=args.confirmed_at,
    )
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_bytes(canonical_bytes(manifest))
    evidence = {
        "schema_version": 1,
        "status": "PASS",
        "database_write_performed": False,
        "automatic_adjudication": True,
        "mapping_sha256": mapping_sha256,
        "object_manifest_sha256": sha256_file(args.object_manifest),
        "source_candidate_sha256": sha256_file(
            args.source_candidate
        ),
        "download_receipt_sha256": sha256_file(
            args.download_receipt
        ),
        "bundle_candidate_continuity": "PASS",
        "output_manifest_sha256": sha256_file(args.output),
        "bundle_count": manifest["bundle_count"],
        "member_count": manifest["member_count"],
        "max_task_asset_id": int(maxima[0]["max_task_asset_id"]),
        "max_asset_id": int(maxima[0]["max_asset_id"]),
    }
    args.evidence.write_bytes(canonical_bytes(evidence))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
