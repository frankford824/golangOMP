#!/usr/bin/env python3
"""Strict hash-bound contract for the three approved Clone B recoveries."""
from __future__ import annotations

import hashlib
import json
import pathlib
import re
import uuid
from typing import Any


SHA256 = re.compile(r"^[0-9a-f]{64}$")
RECOVERY_IDS = (23989, 23990, 23991)
TASK_ID = 2807
POLICY = "legacy_deleted_asset_recovery_v1"
STRATEGY = "verified_oss_recovery_v1"
FINAL_STORAGE_ADAPTER = "clone_b_recovery"
APPROVED_MAPPING_SHA256 = (
    "b19d48eacbc6700536f7e3b3286d1b35f023763cebdd13329b9c8bf76f6b01f7"
)
APPROVED_PLAN_SHA256 = (
    "4fc60c49baa745c087872d46b98680b654e4a15c6cbdca4b7cf7c37593897c9f"
)
APPROVED_DB_APPLY_SHA256 = (
    "78956bb4eb00ece55a4ebacca9d6c5c39d3ac94487c6f7793e7b3a2ff1433a77"
)
APPROVED_DB_IDEMPOTENT_SHA256 = (
    "19d8b6fb7e4942e4b02be004bfe608bff03fc0d43dde27e233d688d6933d544d"
)
APPROVED_COMPONENT_APPLY_SHA256 = (
    "018cbc91f8dee4a7ba7b4e6c44b3d1e76d22967322ad9e928ef94475b8d2ea9b"
)
EXPECTED_DATABASE = "ab_r20260723_01_v9_formal_b"
EXPECTED_HOST = "127.0.0.1"
RECOVERY_NAMESPACE = uuid.UUID("881b0034-ec6d-4b9e-95bd-8e3427b3b650")
REQUIRED_SOURCES = {
    23989: (24034, 683001),
    23990: (24033, 689291),
    23991: (24040, 686447),
}


def canonical_json(value: Any) -> str:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    )


def canonical_hash(value: Any) -> str:
    return hashlib.sha256(canonical_json(value).encode("utf-8")).hexdigest()


def component_self_hash(value: dict[str, Any]) -> str:
    """Match clone_b_materialization_component.self_bound exactly."""
    unsigned = {
        key: item for key, item in value.items()
        if key != "evidence_sha256"
    }
    return hashlib.sha256(
        (canonical_json(unsigned) + "\n").encode("utf-8")
    ).hexdigest()


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def _reject_duplicate_keys(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate JSON object key: {key}")
        result[key] = value
    return result


def read_json(path: pathlib.Path, label: str) -> dict[str, Any]:
    if path.is_symlink() or not path.is_file():
        raise ValueError(f"{label} must be an existing non-symlink file")
    try:
        value = json.loads(
            path.read_text(encoding="utf-8"),
            object_pairs_hook=_reject_duplicate_keys,
        )
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ValueError(f"{label} must be valid UTF-8 JSON") from exc
    if not isinstance(value, dict):
        raise ValueError(f"{label} must contain an object")
    return value


def require_hash(path: pathlib.Path, expected: str, label: str) -> str:
    if not isinstance(expected, str) or not SHA256.fullmatch(expected):
        raise ValueError(f"expected {label} SHA-256 is invalid")
    actual = sha256_file(path)
    if actual != expected:
        raise ValueError(
            f"{label} SHA-256 mismatch: expected={expected} actual={actual}"
        )
    return actual


def _mapping_rows(mapping: dict[str, Any]) -> dict[int, dict[str, Any]]:
    if mapping.get("version") != 2 or not isinstance(
        mapping.get("asset_recoveries"), list
    ):
        raise ValueError("recovery mapping must be a V2 final-reviewed mapping")
    selected: dict[int, dict[str, Any]] = {}
    for row in mapping["asset_recoveries"]:
        if not isinstance(row, dict):
            raise ValueError("asset recovery mapping contains a non-object row")
        missing_id = row.get("missing_task_asset_id")
        if missing_id not in RECOVERY_IDS:
            continue
        if missing_id in selected:
            raise ValueError(f"mapping duplicates recovery {missing_id}")
        selected[missing_id] = row
    if set(selected) != set(RECOVERY_IDS):
        raise ValueError("mapping does not contain exactly the three recoveries")
    for missing_id, row in selected.items():
        source_id, size = REQUIRED_SOURCES[missing_id]
        row_hash = row.get("manifest_row_hash")
        unsigned = {
            key: value for key, value in row.items()
            if key != "manifest_row_hash"
        }
        if (
            row.get("task_id") != TASK_ID
            or row.get("recovery_source_task_asset_id") != source_id
            or row.get("expected_file_size") != size
            or row.get("strategy") != STRATEGY
            or row.get("review_policy_ids") != [POLICY]
            or row.get("confidence") != "confirmed_auto"
            or not isinstance(row.get("confirmed_by"), int)
            or isinstance(row.get("confirmed_by"), bool)
            or row["confirmed_by"] <= 0
            or not str(row.get("confirmed_at") or "").strip()
            or not str(row.get("confirmation_note") or "").strip()
            or row.get("blockers")
            or not isinstance(row.get("original_storage_ref_id"), str)
            or not row["original_storage_ref_id"]
            or not isinstance(row.get("recovery_source_sha256"), str)
            or not SHA256.fullmatch(row["recovery_source_sha256"])
            or row.get("controlled_read_protocol")
            != "controlled-asset-read-v1"
            or not isinstance(
                row.get("controlled_read_evidence_sha256"), str
            )
            or not SHA256.fullmatch(
                row["controlled_read_evidence_sha256"]
            )
            or not isinstance(
                row.get("recovery_source_storage_ref_id"), str
            )
            or not row["recovery_source_storage_ref_id"]
            or not isinstance(row_hash, str)
            or not SHA256.fullmatch(row_hash)
            or canonical_hash(unsigned) != row_hash
        ):
            raise ValueError(
                f"mapping recovery {missing_id} differs from the approved contract"
            )
    return selected


def _plan_entries(
    plan: dict[str, Any],
    mapping_sha256: str,
    mapping_rows: dict[int, dict[str, Any]],
) -> dict[int, dict[str, Any]]:
    base_fields = {
        "version",
        "status",
        "run_id",
        "mapping_sha256",
        "database_writes_executed",
        "production_writes_executed",
        "entries",
    }
    actual_fields = set(plan)
    if actual_fields == base_fields | {"evidence_sha256"}:
        evidence_sha256 = str(plan.get("evidence_sha256") or "")
        unsigned = dict(plan)
        unsigned.pop("evidence_sha256", None)
        if (
            not SHA256.fullmatch(evidence_sha256)
            or canonical_hash(unsigned) != evidence_sha256
        ):
            raise ValueError("recovery plan self hash is missing or stale")
    elif actual_fields != base_fields:
        raise ValueError("recovery plan header differs from the G4 contract")
    if (
        plan.get("version") != 1
        or plan.get("status") != "MATERIALIZED"
        or plan.get("mapping_sha256") != mapping_sha256
        or plan.get("database_writes_executed") is not False
        or plan.get("production_writes_executed") is not False
        or not isinstance(plan.get("run_id"), str)
        or not plan["run_id"]
        or not isinstance(plan.get("entries"), list)
    ):
        raise ValueError("recovery plan header differs from the G4 contract")
    selected: dict[int, dict[str, Any]] = {}
    for entry in plan["entries"]:
        if not isinstance(entry, dict):
            raise ValueError("recovery plan contains a non-object entry")
        missing_id = entry.get("missing_task_asset_id")
        if missing_id not in RECOVERY_IDS or missing_id in selected:
            raise ValueError("recovery plan must contain only three unique entries")
        selected[missing_id] = entry
    if set(selected) != set(RECOVERY_IDS):
        raise ValueError("recovery plan entries differ from the exact allowlist")

    for missing_id, entry in selected.items():
        row = mapping_rows[missing_id]
        source_id, size = REQUIRED_SOURCES[missing_id]
        db_plan = entry.get("db_apply_plan")
        rollback = entry.get("rollback_registry")
        if not isinstance(db_plan, dict) or not isinstance(rollback, dict):
            raise ValueError(f"recovery plan {missing_id} lacks DB/rollback binding")
        storage = db_plan.get("insert_asset_storage_ref")
        update = db_plan.get("update_task_asset")
        upload_update = db_plan.get("update_upload_request")
        original = rollback.get("original_storage_ref")
        restore = rollback.get("restore_task_asset")
        restore_upload = rollback.get("restore_upload_request")
        if not all(
            isinstance(value, dict)
            for value in (
                storage,
                update,
                upload_update,
                original,
                restore,
                restore_upload,
            )
        ):
            raise ValueError(f"recovery plan {missing_id} binding is invalid")
        target_ref = entry.get("target_storage_ref_id")
        target_key = entry.get("target_object_key")
        source_sha = entry.get("source_sha256")
        expected_target_ref = str(
            uuid.uuid5(
                RECOVERY_NAMESPACE,
                (
                    f"{plan['run_id']}:{mapping_sha256}:"
                    f"{missing_id}:{source_sha}"
                ),
            )
        )
        expected_target_key = (
            f"v8-ab/{plan['run_id']}/recovered/task-{TASK_ID}/"
            f"task-asset-{missing_id}/{source_sha}.bin"
        )
        if (
            entry.get("source_task_asset_id") != source_id
            or entry.get("source_size") != size
            or source_sha != row["recovery_source_sha256"]
            or not isinstance(source_sha, str)
            or not SHA256.fullmatch(source_sha)
            or not isinstance(target_ref, str)
            or target_ref != expected_target_ref
            or not isinstance(target_key, str)
            or target_key != expected_target_key
            or storage.get("ref_id") != target_ref
            or storage.get("ref_key") != target_key
            or storage.get("owner_type") != "task_asset"
            or storage.get("owner_id") != missing_id
            or storage.get("file_size") != size
            or storage.get("mime_type") != "image/jpeg"
            or storage.get("checksum_hint") != source_sha
            or storage.get("status") != "recorded"
            or storage.get("is_placeholder") != 0
            or update.get("where") != {"id": missing_id}
            or not isinstance(update.get("set"), dict)
            or update["set"].get("storage_ref_id") != target_ref
            or update["set"].get("storage_key") != target_key
            or update["set"].get("whole_hash") != source_sha
            or upload_update.get("where")
            != {"request_id": restore_upload.get("request_id")}
            or not isinstance(upload_update.get("set"), dict)
            or upload_update["set"].get("bound_ref_id") != target_ref
            or upload_update["set"].get("checksum_hint") != source_sha
            or upload_update["set"].get("file_size") != size
            or upload_update["set"].get("status") != "bound"
            or upload_update["set"].get("session_status") != "completed"
            or restore.get("id") != missing_id
            or restore.get("task_id") != TASK_ID
            or restore.get("file_size") != size
            or restore.get("storage_ref_id")
            != row["original_storage_ref_id"]
            or original.get("ref_id") != row["original_storage_ref_id"]
            or not isinstance(original.get("ref_key"), str)
            or not original["ref_key"]
            or original.get("file_size") != size
            or original.get("mime_type") != "image/jpeg"
            or restore.get("upload_request_id")
            != restore_upload.get("request_id")
        ):
            raise ValueError(
                f"recovery plan {missing_id} differs from mapping/target identity"
            )
    return selected


def load_contract(
    *,
    mapping_path: pathlib.Path,
    expected_mapping_sha256: str,
    plan_path: pathlib.Path,
    expected_plan_sha256: str,
) -> tuple[dict[int, dict[str, Any]], dict[int, dict[str, Any]], dict[str, str]]:
    mapping_sha = require_hash(
        mapping_path, expected_mapping_sha256, "recovery mapping"
    )
    plan_sha = require_hash(plan_path, expected_plan_sha256, "recovery plan")
    mapping = read_json(mapping_path, "recovery mapping")
    plan = read_json(plan_path, "recovery plan")
    mapping_rows = _mapping_rows(mapping)
    plan_entries = _plan_entries(plan, mapping_sha, mapping_rows)
    return mapping_rows, plan_entries, {
        "recovery_mapping_sha256": mapping_sha,
        "recovery_plan_sha256": plan_sha,
    }


def require_frozen_hashes(
    mapping_sha256: str,
    plan_sha256: str,
) -> None:
    if (
        mapping_sha256 != APPROVED_MAPPING_SHA256
        or plan_sha256 != APPROVED_PLAN_SHA256
    ):
        raise ValueError(
            "G06 recovery inputs differ from the frozen final-reviewed/G4 boundary"
        )


def validate_apply_receipts(
    *,
    plan_path: pathlib.Path,
    db_apply_path: pathlib.Path,
    db_idempotent_path: pathlib.Path,
    component_apply_path: pathlib.Path,
    require_frozen: bool,
) -> dict[str, str]:
    plan_value = read_json(plan_path, "recovery materialization plan")
    modern_receipt_contract = "evidence_sha256" in plan_value
    plan_sha = sha256_file(plan_path)
    apply_sha = sha256_file(db_apply_path)
    idempotent_sha = sha256_file(db_idempotent_path)
    component_sha = sha256_file(component_apply_path)
    if require_frozen and (
        plan_sha != APPROVED_PLAN_SHA256
        or apply_sha != APPROVED_DB_APPLY_SHA256
        or idempotent_sha != APPROVED_DB_IDEMPOTENT_SHA256
        or component_sha != APPROVED_COMPONENT_APPLY_SHA256
    ):
        raise ValueError(
            "Clone B recovery apply receipts differ from the authoritative "
            "b19 G4 v12 boundary"
        )

    apply = read_json(db_apply_path, "recovery DB apply receipt")
    idempotent = read_json(
        db_idempotent_path, "recovery DB idempotent receipt"
    )
    expected_receipt_fields = {
        "version",
        "mode",
        "run_id",
        "database",
        "host",
        "plan_sha256",
        "changed_entries",
        "already_in_target_state_entries",
        "database_transaction_committed",
        "object_storage_writes_executed",
        "executed_at",
    }
    for label, value, counts in (
        ("apply", apply, (3, 0)),
        ("idempotent", idempotent, (0, 3)),
    ):
        if (
            set(value) != expected_receipt_fields
            or value.get("version") != 1
            or value.get("mode") != "apply"
            or value.get("run_id") != "bundle-materialization-20260723-29"
            or value.get("database") != EXPECTED_DATABASE
            or value.get("host") != EXPECTED_HOST
            or value.get("plan_sha256") != plan_sha
            or (
                value.get("changed_entries"),
                value.get("already_in_target_state_entries"),
            )
            != counts
            or value.get("database_transaction_committed") is not True
            or value.get("object_storage_writes_executed") is not False
            or not str(value.get("executed_at") or "").strip()
        ):
            raise ValueError(
                f"Clone B recovery {label} receipt contract differs"
            )

    component = read_json(
        component_apply_path, "recovery component apply receipt"
    )
    artifacts = component.get("artifacts")
    expected_component_fields = {
            "schema_version",
            "status",
            "component",
            "action",
            "run_id",
            "database",
            "host",
            "database_writes_executed",
            "production_writes_executed",
            "guard_retained_for_rollback",
            "guard_exactly_restored",
            "artifacts",
            "evidence_sha256",
    }
    if modern_receipt_contract:
        expected_component_fields.add(
            "ownership_receipt_contract_version"
        )
    if (
        set(component) != expected_component_fields
        or component.get("schema_version") != 1
        or component.get("status") != "APPLIED"
        or component.get("component") != "recovery"
        or component.get("action") != "apply"
        or component.get("run_id") != "bundle-materialization-20260723-29"
        or component.get("database") != EXPECTED_DATABASE
        or component.get("host") != EXPECTED_HOST
        or component.get("database_writes_executed") is not True
        or component.get("production_writes_executed") is not False
        or component.get("guard_retained_for_rollback") is not True
        or component.get("guard_exactly_restored") is not False
        or (
            modern_receipt_contract
            and component.get("ownership_receipt_contract_version") != 1
        )
        or component.get("evidence_sha256") != component_self_hash(component)
        or not isinstance(artifacts, list)
    ):
        raise ValueError("Clone B recovery component apply receipt differs")
    artifact_hashes: dict[str, str] = {}
    for artifact in artifacts:
        if (
            not isinstance(artifact, dict)
            or set(artifact) != {"path", "sha256", "size"}
            or not isinstance(artifact.get("path"), str)
            or artifact["path"] in artifact_hashes
            or not isinstance(artifact.get("sha256"), str)
            or not SHA256.fullmatch(artifact["sha256"])
            or not isinstance(artifact.get("size"), int)
            or isinstance(artifact.get("size"), bool)
            or artifact["size"] <= 0
        ):
            raise ValueError("Clone B recovery component artifact is invalid")
        artifact_hashes[artifact["path"]] = artifact["sha256"]
    expected_component_artifacts = {
        "recovery-materialization-plan.json",
        "recovery-guard-before.json",
        "recovery-guard-provision.json",
        "recovery-db-apply.json",
        "recovery-db-idempotent.json",
    }
    if modern_receipt_contract:
        expected_component_artifacts |= {
            "recovery-file-write-ahead.json",
            *{
                f"recovery-ownership-{asset_id}.json"
                for asset_id in RECOVERY_IDS
            },
            *{
                f"recovery-staging-ownership-{asset_id}.json"
                for asset_id in RECOVERY_IDS
            },
        }
    if (
        artifact_hashes.get("recovery-materialization-plan.json") != plan_sha
        or artifact_hashes.get("recovery-db-apply.json") != apply_sha
        or artifact_hashes.get("recovery-db-idempotent.json") != idempotent_sha
        or set(artifact_hashes) != expected_component_artifacts
    ):
        raise ValueError("Clone B recovery component artifacts are not exact")
    return {
        "recovery_plan_sha256": plan_sha,
        "recovery_db_apply_sha256": apply_sha,
        "recovery_db_idempotent_sha256": idempotent_sha,
        "recovery_component_apply_sha256": component_sha,
    }


def original_manifest_rows(
    mapping_rows: dict[int, dict[str, Any]],
    plan_entries: dict[int, dict[str, Any]],
) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for missing_id in RECOVERY_IDS:
        mapping = mapping_rows[missing_id]
        entry = plan_entries[missing_id]
        original = entry["rollback_registry"]["original_storage_ref"]
        rows.append(
            {
                "entity_key": f"task_asset:{missing_id}",
                "owner_kind": "task_asset",
                "owner_id": missing_id,
                "task_id": TASK_ID,
                "storage_ref_id": mapping["original_storage_ref_id"],
                "storage_adapter": original["storage_adapter"],
                "object_key": original["ref_key"],
                "size": entry["source_size"],
                "mime_type": original["mime_type"],
                "sha256": "",
                "status": original["status"],
                "is_placeholder": bool(original["is_placeholder"]),
            }
        )
    return rows


def recovery_manifest_rows(
    mapping_rows: dict[int, dict[str, Any]],
    plan_entries: dict[int, dict[str, Any]],
) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for missing_id in RECOVERY_IDS:
        entry = plan_entries[missing_id]
        storage = entry["db_apply_plan"]["insert_asset_storage_ref"]
        rows.append(
            {
                "entity_key": f"task_asset:{missing_id}",
                "owner_kind": "task_asset",
                "owner_id": missing_id,
                "task_id": TASK_ID,
                "storage_ref_id": entry["target_storage_ref_id"],
                "storage_adapter": FINAL_STORAGE_ADAPTER,
                "object_key": entry["target_object_key"],
                "size": entry["source_size"],
                "mime_type": storage["mime_type"],
                "sha256": entry["source_sha256"],
                "status": storage["status"],
                "is_placeholder": bool(storage["is_placeholder"]),
            }
        )
    return rows
