#!/usr/bin/env python3
"""Prepare or materialize run-scoped Clone B legacy asset recovery objects.

The tool is deliberately file-only. It never connects to MySQL and never
writes production/object-storage state. A caller must first export reviewed
mapping rows, read-only DB evidence, and surviving source bytes. ``prepare``
validates that evidence and emits a deterministic DB apply plan plus a
rollback registry. ``materialize`` additionally copies bytes into a contained
fixture/object root; it still does not execute the DB plan.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import re
import shutil
import tempfile
import uuid
from typing import Any


RUN_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{2,80}$")
SHA256 = re.compile(r"^[0-9a-f]{64}$")
POLICY = "legacy_deleted_asset_recovery_v1"
STRATEGY = "clone_b_prematerialized_storage_ref_v1"
NAMESPACE = uuid.UUID("881b0034-ec6d-4b9e-95bd-8e3427b3b650")
ALLOWED = {
    23989: (2807, 24034, 683001),
    23990: (2807, 24033, 689291),
    23991: (2807, 24040, 686447),
}
SOURCE_TASK_ID = 2098
CHANGED_TASK_ASSET_FIELDS = (
    "storage_ref_id",
    "storage_key",
    "whole_hash",
    "upload_status",
    "deleted_at",
    "cleaned_at",
    "object_deleted_at",
    "access_revoked_at",
    "access_revoked_reason",
)
CHANGED_UPLOAD_REQUEST_FIELDS = (
    "bound_ref_id",
    "checksum_hint",
    "file_size",
    "status",
    "session_status",
)
STAGE_TOKEN_XATTR = "user.codex_v8_stage_token"


def canonical_bytes(value: Any) -> bytes:
    return json.dumps(
        value, ensure_ascii=False, separators=(",", ":"), sort_keys=True
    ).encode("utf-8")


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def self_bound(value: dict[str, Any]) -> dict[str, Any]:
    result = dict(value)
    result["evidence_sha256"] = sha256_bytes(canonical_bytes(result))
    return result


def verify_self_bound(value: dict[str, Any], label: str) -> None:
    expected = str(value.get("evidence_sha256") or "")
    unsigned = dict(value)
    unsigned.pop("evidence_sha256", None)
    if (
        not SHA256.fullmatch(expected)
        or sha256_bytes(canonical_bytes(unsigned)) != expected
    ):
        raise ValueError(f"{label} self hash is missing or stale")


def read_json(path: pathlib.Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def require_dict(value: Any, path: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise ValueError(f"{path} must be an object")
    return value


def contained(root: pathlib.Path, relative: pathlib.PurePosixPath) -> pathlib.Path:
    if relative.is_absolute() or ".." in relative.parts:
        raise ValueError("object path must be contained and relative")
    root = root.resolve()
    target = (root / pathlib.Path(*relative.parts)).resolve()
    if target != root and root not in target.parents:
        raise ValueError("object path escapes fixture root")
    return target


def validate_confirmed_row(row: dict[str, Any]) -> tuple[int, int, int]:
    missing_id = row.get("missing_task_asset_id")
    if missing_id not in ALLOWED:
        raise ValueError("recovery row is outside the exact frozen allowlist")
    task_id, source_id, size = ALLOWED[missing_id]
    if (
        row.get("task_id") != task_id
        or row.get("recovery_source_task_asset_id") != source_id
        or row.get("expected_file_size") != size
        or row.get("strategy") != STRATEGY
        or row.get("review_policy_ids") != [POLICY]
        or row.get("confidence") != "confirmed_auto"
        or not isinstance(row.get("confirmed_by"), int)
        or isinstance(row.get("confirmed_by"), bool)
        or row["confirmed_by"] <= 0
        or not str(row.get("confirmed_at") or "").strip()
        or str(row.get("confirmed_at")) == "0001-01-01T00:00:00Z"
        or not str(row.get("confirmation_note") or "").strip()
        or row.get("blockers")
    ):
        raise ValueError(
            f"task_asset {missing_id} is not an exact confirmed recovery row"
        )
    expected_hash = row.get("manifest_row_hash")
    if not isinstance(expected_hash, str) or not SHA256.fullmatch(expected_hash):
        raise ValueError("confirmed row manifest hash is invalid")
    unhashed = {
        key: value
        for key, value in row.items()
        if key != "manifest_row_hash"
    }
    if sha256_bytes(canonical_bytes(unhashed)) != expected_hash:
        raise ValueError("confirmed row manifest hash does not match content")
    return task_id, source_id, size


def derivative_hashes(rows: Any, source_id: int) -> dict[str, str]:
    if not isinstance(rows, list):
        raise ValueError("derivatives must be an array")
    result: dict[str, str] = {}
    for index, item in enumerate(rows):
        item = require_dict(item, f"derivatives[{index}]")
        kind = item.get("asset_type")
        whole_hash = item.get("whole_hash")
        if (
            item.get("source_asset_version_id") != source_id
            or kind not in {"preview", "design_thumb"}
            or not isinstance(whole_hash, str)
            or not SHA256.fullmatch(whole_hash)
            or kind in result
        ):
            raise ValueError("derivative lineage must contain one linked preview and design_thumb")
        result[kind] = whole_hash
    if set(result) != {"preview", "design_thumb"}:
        raise ValueError("derivative lineage is incomplete")
    return result


def require_before_fields(row: dict[str, Any], fields: tuple[str, ...], path: str) -> None:
    missing = [field for field in fields if field not in row]
    if missing:
        raise ValueError(f"{path} lacks rollback fields {missing}")


def build_entry(
    run_id: str,
    mapping_sha256: str,
    row: dict[str, Any],
    evidence: dict[str, Any],
) -> dict[str, Any]:
    task_id, source_id, size = validate_confirmed_row(row)
    missing_id = row["missing_task_asset_id"]
    if evidence.get("missing_task_asset_id") != missing_id:
        raise ValueError("evidence missing_task_asset_id differs from mapping")
    source_path = pathlib.Path(str(evidence.get("source_local_path") or ""))
    if not source_path.is_file():
        raise ValueError(f"task_asset {source_id} source_local_path is not a file")
    actual_size = source_path.stat().st_size
    actual_sha256 = sha256_file(source_path)
    if actual_size != size:
        raise ValueError(f"task_asset {source_id} source byte size drifted")
    declared_sha256 = evidence.get("source_sha256")
    if declared_sha256 != actual_sha256 or not SHA256.fullmatch(str(declared_sha256 or "")):
        raise ValueError(f"task_asset {source_id} source SHA-256 drifted")

    missing_before = require_dict(
        evidence.get("missing_task_asset_before"), "missing_task_asset_before"
    )
    source_before = require_dict(
        evidence.get("source_task_asset"), "source_task_asset"
    )
    fetch_receipt = require_dict(
        evidence.get("source_fetch_receipt"), "source_fetch_receipt"
    )
    upload_before = require_dict(
        evidence.get("upload_request_before"), "upload_request_before"
    )
    storage_before = require_dict(
        evidence.get("original_storage_ref_before"),
        "original_storage_ref_before",
    )
    require_before_fields(
        missing_before, CHANGED_TASK_ASSET_FIELDS, "missing_task_asset_before"
    )
    require_before_fields(
        upload_before, CHANGED_UPLOAD_REQUEST_FIELDS, "upload_request_before"
    )
    if (
        missing_before.get("id") != missing_id
        or missing_before.get("task_id") != task_id
        or missing_before.get("file_size") != size
        or missing_before.get("storage_ref_id")
        != row.get("original_storage_ref_id")
        or source_before.get("id") != source_id
        or source_before.get("task_id") != SOURCE_TASK_ID
        or source_before.get("asset_type") != "delivery"
        or source_before.get("file_size") != size
        or source_before.get("storage_ref_id")
        != row.get("recovery_source_storage_ref_id")
        or not str(source_before.get("storage_key") or "").strip()
        or source_before.get("upload_status") != "uploaded"
        or source_before.get("deleted_at") is not None
        or source_before.get("object_deleted_at") is not None
        or storage_before.get("ref_id") != row.get("original_storage_ref_id")
        or upload_before.get("request_id")
        != missing_before.get("upload_request_id")
    ):
        raise ValueError("read-only DB evidence differs from the confirmed recovery row")
    if (
        fetch_receipt.get("protocol") != "controlled-asset-read-v1"
        or fetch_receipt.get("task_asset_id") != source_id
        or fetch_receipt.get("storage_ref_id")
        != row.get("recovery_source_storage_ref_id")
        or fetch_receipt.get("object_key") != source_before.get("storage_key")
        or fetch_receipt.get("size") != size
        or fetch_receipt.get("sha256") != actual_sha256
        or not str(fetch_receipt.get("fetched_at") or "").strip()
    ):
        raise ValueError("source fetch receipt does not bind local bytes to the surviving object")

    missing_derivatives = derivative_hashes(
        evidence.get("missing_derivatives"), missing_id
    )
    source_derivatives = derivative_hashes(
        evidence.get("source_derivatives"), source_id
    )
    expected_derivatives = {
        "preview": row.get("preview_whole_hash"),
        "design_thumb": row.get("design_thumb_whole_hash"),
    }
    if (
        missing_derivatives != expected_derivatives
        or source_derivatives != expected_derivatives
    ):
        raise ValueError("linked derivative hashes do not prove the frozen content pair")

    target_ref = str(
        uuid.uuid5(NAMESPACE, f"{run_id}:{mapping_sha256}:{missing_id}:{actual_sha256}")
    )
    object_key = (
        f"v8-ab/{run_id}/recovered/task-{task_id}/"
        f"task-asset-{missing_id}/{actual_sha256}.bin"
    )
    target_values = {
        "storage_ref_id": target_ref,
        "storage_key": object_key,
        "whole_hash": actual_sha256,
        "upload_status": "uploaded",
        "deleted_at": None,
        "cleaned_at": None,
        "object_deleted_at": None,
        "access_revoked_at": None,
        "access_revoked_reason": "",
    }
    upload_values = {
        "bound_ref_id": target_ref,
        "checksum_hint": actual_sha256,
        "file_size": size,
        "status": "bound",
        "session_status": "completed",
    }
    storage_ref_values = {
        "ref_id": target_ref,
        "asset_id": missing_before.get("asset_id"),
        "owner_type": "task_asset",
        "owner_id": missing_id,
        "upload_request_id": missing_before.get("upload_request_id"),
        "storage_adapter": "local",
        "ref_type": "task_asset_object",
        "ref_key": object_key,
        "file_name": missing_before.get("file_name"),
        "mime_type": missing_before.get("mime_type"),
        "file_size": size,
        "is_placeholder": 0,
        "checksum_hint": actual_sha256,
        "status": "recorded",
    }
    return {
        "missing_task_asset_id": missing_id,
        "source_task_asset_id": source_id,
        "source_local_path": str(source_path.resolve()),
        "source_sha256": actual_sha256,
        "source_size": size,
        "target_storage_ref_id": target_ref,
        "target_object_key": object_key,
        "derivative_lineage": expected_derivatives,
        "db_apply_plan": {
            "insert_asset_storage_ref": storage_ref_values,
            "update_task_asset": {
                "where": {"id": missing_id},
                "set": target_values,
            },
            "update_upload_request": {
                "where": {"request_id": upload_before["request_id"]},
                "set": upload_values,
            },
        },
        "rollback_registry": {
            "restore_task_asset": missing_before,
            "restore_upload_request": upload_before,
            "original_storage_ref": storage_before,
            "delete_created_storage_ref_id": target_ref,
            "delete_fixture_object_key": object_key,
            "expected_fixture_sha256": actual_sha256,
            "db_rollback_plan": {
                "restore_task_asset": {
                    "where": {"id": missing_id},
                    "set": {
                        field: missing_before[field]
                        for field in CHANGED_TASK_ASSET_FIELDS
                    },
                },
                "restore_upload_request": {
                    "where": {"request_id": upload_before["request_id"]},
                    "set": {
                        field: upload_before[field]
                        for field in CHANGED_UPLOAD_REQUEST_FIELDS
                    },
                },
                "delete_asset_storage_ref": {
                    "where": {"ref_id": target_ref},
                },
            },
        },
    }


def write_atomic_idempotent(path: pathlib.Path, data: bytes) -> None:
    if path.exists() or path.is_symlink():
        if (
            not path.is_file()
            or path.is_symlink()
            or path.read_bytes() != data
        ):
            raise FileExistsError(f"refusing to overwrite drifted artifact: {path}")
        with path.open("r+b") as handle:
            os.fsync(handle.fileno())
        if os.name != "nt":
            directory = os.open(
                path.parent,
                os.O_RDONLY | getattr(os, "O_DIRECTORY", 0),
            )
            try:
                os.fsync(directory)
            finally:
                os.close(directory)
        return
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, name = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    temporary = pathlib.Path(name)
    try:
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(data)
            handle.flush()
            os.fsync(handle.fileno())
        try:
            os.link(temporary, path)
        except FileExistsError:
            if (
                path.is_file()
                and not path.is_symlink()
                and path.read_bytes() == data
            ):
                with path.open("r+b") as handle:
                    os.fsync(handle.fileno())
                if os.name != "nt":
                    directory = os.open(
                        path.parent,
                        os.O_RDONLY | getattr(os, "O_DIRECTORY", 0),
                    )
                    try:
                        os.fsync(directory)
                    finally:
                        os.close(directory)
                return
            raise FileExistsError(
                f"refusing to overwrite racing artifact: {path}"
            ) from None
        if os.name != "nt":
            directory = os.open(
                path.parent,
                os.O_RDONLY | getattr(os, "O_DIRECTORY", 0),
            )
            try:
                os.fsync(directory)
            finally:
                os.close(directory)
    finally:
        if temporary.exists():
            temporary.unlink()


def fsync_directory(path: pathlib.Path) -> None:
    if os.name == "nt":
        return
    descriptor = os.open(
        path,
        os.O_RDONLY | getattr(os, "O_DIRECTORY", 0),
    )
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def fsync_directory_chain(
    leaf: pathlib.Path,
    root: pathlib.Path,
) -> None:
    current = leaf.resolve()
    boundary = root.resolve()
    try:
        current.relative_to(boundary)
    except ValueError:
        raise ValueError("durability path escapes the fixture root") from None
    while True:
        fsync_directory(current)
        if current == boundary:
            break
        current = current.parent


def durable_unlink(path: pathlib.Path) -> None:
    parent = path.parent
    path.unlink()
    fsync_directory(parent)


def create_reserved_private(path: pathlib.Path, token: str) -> None:
    descriptor = os.open(
        path,
        os.O_WRONLY | os.O_CREAT | os.O_EXCL,
        0o600,
    )
    try:
        if os.name != "nt":
            os.setxattr(
                descriptor,
                STAGE_TOKEN_XATTR,
                token.encode("ascii"),
            )
        os.fsync(descriptor)
    finally:
        os.close(descriptor)
    fsync_directory(path.parent)


def retire_reservation_token(path: pathlib.Path, token: str) -> None:
    if os.name == "nt":
        return
    try:
        actual = os.getxattr(path, STAGE_TOKEN_XATTR).decode("ascii")
    except (OSError, UnicodeError):
        raise ValueError("staging reservation token is unavailable") from None
    if actual != token:
        raise ValueError("staging reservation token drifted")
    os.removexattr(path, STAGE_TOKEN_XATTR)
    with path.open("r+b") as handle:
        os.fsync(handle.fileno())


def materialize_entry(
    root: pathlib.Path,
    entry: dict[str, Any],
    *,
    allow_existing_created: bool = False,
) -> None:
    relative = pathlib.PurePosixPath("objects") / pathlib.PurePosixPath(
        entry["target_object_key"]
    )
    target = contained(root, relative)
    disposition = entry.get("rollback_registry", {}).get(
        "fixture_disposition"
    )
    if disposition not in {"created", "reused_identical"}:
        raise ValueError("recovery fixture disposition is missing")
    if target.exists():
        if (
            target.stat().st_size != entry["source_size"]
            or sha256_file(target) != entry["source_sha256"]
        ):
                raise FileExistsError(f"existing fixture object drifted: {target}")
        if disposition != "reused_identical" and not (
            disposition == "created" and allow_existing_created
        ):
            raise FileExistsError("recovery fixture appeared after write-ahead")
        return
    if disposition != "created":
        raise FileNotFoundError("reused recovery fixture disappeared")
    target.parent.mkdir(parents=True, exist_ok=True)
    stage = pathlib.Path(
        str(entry.get("rollback_registry", {}).get("staging_local_path") or "")
    )
    try:
        stage.resolve(strict=False).relative_to(root.parent.resolve())
    except ValueError:
        raise ValueError("recovery staging path escapes the run root") from None
    if not stage.is_absolute() or stage.exists() or stage.is_symlink():
        raise FileExistsError("recovery staging path is unavailable")
    private_temporary = pathlib.Path(
        str(
            entry.get("rollback_registry", {}).get(
                "staging_private_path"
            )
            or ""
        )
    )
    ownership_token = str(
        entry.get("rollback_registry", {}).get(
            "staging_ownership_token"
        )
        or ""
    )
    try:
        private_temporary.resolve(strict=False).relative_to(
            root.parent.resolve()
        )
    except ValueError:
        raise ValueError("recovery private path escapes the run root") from None
    if (
        not private_temporary.is_absolute()
        or not SHA256.fullmatch(ownership_token)
        or private_temporary.exists()
        or private_temporary.is_symlink()
    ):
        raise FileExistsError("recovery private staging path is unavailable")
    private_identity: tuple[int, int] | None = None
    try:
        create_reserved_private(
            private_temporary,
            ownership_token,
        )
        created_stat = private_temporary.stat()
        private_identity = (created_stat.st_dev, created_stat.st_ino)
        with pathlib.Path(entry["source_local_path"]).open("rb") as source:
            with private_temporary.open("r+b") as destination:
                shutil.copyfileobj(source, destination)
                destination.flush()
                os.fsync(destination.fileno())
        if (
            private_temporary.stat().st_size != entry["source_size"]
            or sha256_file(private_temporary) != entry["source_sha256"]
        ):
            raise ValueError("fixture copy failed post-write verification")
        staging_receipt_path = pathlib.Path(
            str(
                entry.get("rollback_registry", {}).get(
                    "staging_ownership_receipt_path"
                )
                or ""
            )
        )
        try:
            staging_receipt_path.resolve(strict=False).relative_to(
                root.parent.resolve()
            )
        except ValueError:
            raise ValueError(
                "recovery staging ownership receipt escapes the run root"
            ) from None
        temporary_stat = private_temporary.stat()
        staging_receipt_fields = {
                "schema_version": 1,
                "status": "STAGING_OWNED",
                "run_id": entry["target_object_key"].split("/")[1],
                "staging_path": str(stage.resolve()),
                "device": temporary_stat.st_dev,
                "inode": temporary_stat.st_ino,
                "size": temporary_stat.st_size,
                "sha256": sha256_file(private_temporary),
        }
        staging_receipt_fields["private_path"] = str(
            private_temporary.resolve()
        )
        staging_receipt = self_bound(staging_receipt_fields)
        write_atomic_idempotent(
            staging_receipt_path,
            canonical_bytes(staging_receipt) + b"\n",
        )
        os.link(private_temporary, stage)
        fsync_directory(stage.parent)
        if not os.path.samestat(private_temporary.stat(), stage.stat()):
            raise RuntimeError("recovery staging ownership proof failed")
        retire_reservation_token(stage, ownership_token)
        durable_unlink(private_temporary)
        private_identity = None
        os.link(stage, target)
        fsync_directory_chain(target.parent, root)
        target_stat = target.stat()
        source_stat = stage.stat()
        if not os.path.samestat(source_stat, target_stat):
            raise RuntimeError("recovery hard-link ownership proof failed")
        receipt_path = pathlib.Path(
            str(
                entry.get("rollback_registry", {}).get(
                    "ownership_receipt_path"
                )
                or ""
            )
        )
        try:
            receipt_path.resolve(strict=False).relative_to(
                root.parent.resolve()
            )
        except ValueError:
            raise ValueError(
                "recovery ownership receipt escapes the run root"
            ) from None
        receipt = self_bound(
            {
                "schema_version": 1,
                "status": "OWNED_LINK",
                "run_id": entry["target_object_key"].split("/")[1],
                "target_path": str(target.resolve()),
                "staging_path": str(stage.resolve()),
                "device": target_stat.st_dev,
                "inode": target_stat.st_ino,
                "size": target_stat.st_size,
                "sha256": sha256_file(target),
            }
        )
        write_atomic_idempotent(
            receipt_path, canonical_bytes(receipt) + b"\n"
        )
        durable_unlink(stage)
    finally:
        if private_temporary.exists():
            stat = private_temporary.stat()
            if private_identity == (stat.st_dev, stat.st_ino):
                durable_unlink(private_temporary)
        if stage.exists() and target.exists():
            staging_receipt_path = pathlib.Path(
                str(
                    entry.get("rollback_registry", {}).get(
                        "staging_ownership_receipt_path"
                    )
                    or ""
                )
            )
            if staging_receipt_path.is_file():
                receipt = read_json(staging_receipt_path)
                verify_self_bound(receipt, "recovery staging ownership receipt")
                stat = stage.stat()
                if (
                    receipt.get("status") == "STAGING_OWNED"
                    and receipt.get("run_id")
                    == entry["target_object_key"].split("/")[1]
                    and receipt.get("staging_path") == str(stage.resolve())
                    and receipt.get("device") == stat.st_dev
                    and receipt.get("inode") == stat.st_ino
                    and receipt.get("size") == stat.st_size
                    and receipt.get("sha256") == sha256_file(stage)
                ):
                    durable_unlink(stage)


def run(args: argparse.Namespace) -> dict[str, Any]:
    existing_output = args.output.is_file() and not args.output.is_symlink()
    mapping_bytes = args.mapping.read_bytes()
    mapping_sha256 = sha256_bytes(mapping_bytes)
    mapping = json.loads(mapping_bytes)
    evidence = read_json(args.evidence)
    if not isinstance(evidence, dict) or evidence.get("version") != 1:
        raise ValueError("evidence.version must be 1")
    run_id = str(evidence.get("run_id") or "")
    if not RUN_ID.fullmatch(run_id):
        raise ValueError("evidence.run_id is invalid")
    if evidence.get("mapping_sha256") != mapping_sha256:
        raise ValueError("evidence mapping_sha256 mismatch")
    recovery_rows = {
        row.get("missing_task_asset_id"): row
        for row in mapping.get("asset_recoveries", [])
        if row.get("missing_task_asset_id") in ALLOWED
    }
    evidence_rows = evidence.get("recoveries")
    if set(recovery_rows) != set(ALLOWED) or not isinstance(evidence_rows, list):
        raise ValueError("mapping/evidence must contain all three exact recoveries")
    by_missing = {
        row.get("missing_task_asset_id"): row
        for row in evidence_rows
        if isinstance(row, dict)
    }
    if set(by_missing) != set(ALLOWED):
        raise ValueError("evidence must contain all three exact recoveries")
    entries = [
        build_entry(run_id, mapping_sha256, recovery_rows[missing_id], by_missing[missing_id])
        for missing_id in sorted(ALLOWED)
    ]
    expected_write_ahead = getattr(args, "expected_write_ahead", None)
    resumed_materialization = existing_output or expected_write_ahead is not None
    for entry in entries:
        staging_path = (
            args.output.parent.resolve()
            / (
                ".recovery-stage-"
                f"{entry['missing_task_asset_id']}-"
                f"{entry['source_sha256']}.bin"
            )
        )
        if (
            (staging_path.exists() or staging_path.is_symlink())
            and not resumed_materialization
        ):
            raise FileExistsError(
                f"recovery staging path existed before write-ahead: {staging_path}"
            )
        entry["rollback_registry"]["staging_local_path"] = str(staging_path)
        private_path = (
            args.output.parent.resolve()
            / (
                ".recovery-private-"
                f"{entry['missing_task_asset_id']}-"
                f"{entry['source_sha256']}.bin"
            )
        )
        ownership_token = sha256_bytes(
            (
                f"{run_id}\0{mapping_sha256}\0{private_path}\0"
                f"{entry['target_object_key']}"
            ).encode("utf-8")
        )
        if (
            (private_path.exists() or private_path.is_symlink())
            and not resumed_materialization
        ):
            raise FileExistsError(
                "recovery private staging path existed before write-ahead"
            )
        entry["rollback_registry"]["staging_private_path"] = str(
            private_path
        )
        entry["rollback_registry"]["staging_ownership_token"] = (
            ownership_token
        )
        staging_receipt = (
            args.output.parent.resolve()
            / (
                "recovery-staging-ownership-"
                f"{entry['missing_task_asset_id']}.json"
            )
        )
        if (
            (staging_receipt.exists() or staging_receipt.is_symlink())
            and not resumed_materialization
        ):
            raise FileExistsError(
                "recovery staging ownership receipt existed before write-ahead"
            )
        entry["rollback_registry"][
            "staging_ownership_receipt_path"
        ] = str(staging_receipt)
        ownership_receipt = (
            args.output.parent.resolve()
            / (
                "recovery-ownership-"
                f"{entry['missing_task_asset_id']}.json"
            )
        )
        if (
            (ownership_receipt.exists() or ownership_receipt.is_symlink())
            and not resumed_materialization
        ):
            raise FileExistsError(
                "recovery ownership receipt existed before write-ahead"
            )
        entry["rollback_registry"]["ownership_receipt_path"] = str(
            ownership_receipt
        )
    if args.fixture_root is not None:
        for entry in entries:
            target = contained(
                args.fixture_root,
                pathlib.PurePosixPath("objects")
                / pathlib.PurePosixPath(entry["target_object_key"]),
            )
            if target.exists():
                if (
                    not target.is_file()
                    or target.stat().st_size != entry["source_size"]
                    or sha256_file(target) != entry["source_sha256"]
                ):
                    raise FileExistsError(
                        f"existing fixture object drifted: {target}"
                    )
                disposition = "reused_identical"
            else:
                disposition = "created"
            entry["rollback_registry"][
                "fixture_disposition"
            ] = disposition
    def validated_plan(path: pathlib.Path, status: str) -> dict[str, Any]:
        expected = read_json(path)
        unsigned = dict(expected)
        expected_hash = str(unsigned.pop("evidence_sha256", ""))
        if (
            expected.get("version") != 1
            or expected.get("status") != status
            or expected.get("run_id") != run_id
            or expected.get("mapping_sha256") != mapping_sha256
            or not SHA256.fullmatch(expected_hash)
            or sha256_bytes(canonical_bytes(unsigned)) != expected_hash
            or not isinstance(expected.get("entries"), list)
            or len(expected["entries"]) != len(entries)
        ):
            raise ValueError("recovery write-ahead contract is invalid")
        return expected

    write_ahead_plan = (
        validated_plan(expected_write_ahead, "PREPARED")
        if expected_write_ahead is not None
        else None
    )
    existing_plan = (
        validated_plan(args.output, "MATERIALIZED")
        if existing_output
        else None
    )
    if existing_plan is not None:
        if (
            write_ahead_plan is not None
            and existing_plan["entries"] != write_ahead_plan["entries"]
        ):
            raise ValueError("materialized recovery plan differs from write-ahead")
        for entry, prior_entry in zip(entries, existing_plan["entries"]):
            entry["rollback_registry"] = prior_entry["rollback_registry"]
        if entries != existing_plan["entries"]:
            raise ValueError("recovery state drifted after prior materialization")
    elif write_ahead_plan is not None:
        if entries != write_ahead_plan["entries"]:
            raise ValueError("recovery state drifted after write-ahead")
    plan = self_bound({
        "version": 1,
        "status": "MATERIALIZED" if args.materialize else "PREPARED",
        "run_id": run_id,
        "mapping_sha256": mapping_sha256,
        "database_writes_executed": False,
        "production_writes_executed": False,
        "entries": entries,
    })
    if args.materialize:
        if args.fixture_root is None:
            raise ValueError("--materialize requires --fixture-root")
        for entry in entries:
            materialize_entry(
                args.fixture_root,
                entry,
                allow_existing_created=existing_output,
            )
    write_atomic_idempotent(args.output, canonical_bytes(plan) + b"\n")
    return plan


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mapping", type=pathlib.Path, required=True)
    parser.add_argument("--evidence", type=pathlib.Path, required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--materialize", action="store_true")
    parser.add_argument("--fixture-root", type=pathlib.Path)
    parser.add_argument("--expected-write-ahead", type=pathlib.Path)
    return parser.parse_args()


def main() -> int:
    run(parse_args())
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
