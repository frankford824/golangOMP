#!/usr/bin/env python3
"""Rebind immutable recovery evidence after the reviewed mapping envelope changes.

The three approved task-2807 recovery rows must be byte-for-byte identical in
the old and new mappings.  Historical controlled-read evidence and the old
production plan remain immutable.  A schema-2 physical Clone A snapshot proves
that the planned recovered identity is now present.  Local ownership receipts
prove the recovery content bytes; this tool never connects to a database or
object store and never rewrites an input artifact.
"""

from __future__ import annotations

import argparse
import copy
import hashlib
import json
import os
import pathlib
import tempfile
from typing import Any

try:
    from scripts.ab import build_api_oracle as api_oracle
    from scripts.ab import build_materialization_oracle_receipts as material
except ModuleNotFoundError:
    import build_api_oracle as api_oracle
    import build_materialization_oracle_receipts as material


RECOVERY_IDS = (23989, 23990, 23991)
KIND = "recovery_materialization_rebind_v1"


def canonical(value: Any) -> bytes:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def load_json(path: pathlib.Path, label: str) -> dict[str, Any]:
    if path.is_symlink() or not path.is_file():
        raise ValueError(f"{label} must be an existing non-symlink file")
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{label} must be a JSON object")
    return value


def require_hash(value: Any, label: str) -> str:
    text = str(value or "")
    if not material.SHA256_RE.fullmatch(text):
        raise ValueError(f"{label} must be a lowercase SHA-256")
    return text


def require_file_hash(path: pathlib.Path, expected: str, label: str) -> str:
    expected = require_hash(expected, f"{label} expected SHA-256")
    actual = sha256_file(path)
    if actual != expected:
        raise ValueError(f"{label} file hash differs")
    return actual


def self_hash(document: dict[str, Any], label: str) -> str:
    evidence = require_hash(document.get("evidence_sha256"), f"{label} evidence")
    unsigned = {
        key: value
        for key, value in document.items()
        if key != "evidence_sha256"
    }
    if evidence != sha256_bytes(canonical(unsigned)):
        raise ValueError(f"{label} self-hash differs")
    return evidence


def signed(document: dict[str, Any]) -> dict[str, Any]:
    if "evidence_sha256" in document:
        raise ValueError("unsigned document already has evidence_sha256")
    return {
        **document,
        "evidence_sha256": sha256_bytes(canonical(document)),
    }


def recovery_rows(
    mapping: dict[str, Any], label: str
) -> tuple[dict[int, dict[str, Any]], str]:
    rows = mapping.get("asset_recoveries")
    if not isinstance(rows, list):
        raise ValueError(f"{label} lacks asset_recoveries")
    selected: dict[int, dict[str, Any]] = {}
    for raw in rows:
        if not isinstance(raw, dict):
            raise ValueError(f"{label} has a non-object recovery")
        missing_id = raw.get("missing_task_asset_id")
        if missing_id not in RECOVERY_IDS:
            continue
        if missing_id in selected:
            raise ValueError(f"{label} duplicates recovery {missing_id}")
        selected[missing_id] = copy.deepcopy(raw)
    if tuple(sorted(selected)) != RECOVERY_IDS:
        raise ValueError(f"{label} does not cover the exact recovery set")
    ordered = [selected[item] for item in RECOVERY_IDS]
    return selected, sha256_bytes(canonical(ordered))


def validate_equal_recovery_scope(
    old_mapping: dict[str, Any],
    new_mapping: dict[str, Any],
) -> tuple[dict[int, dict[str, Any]], str]:
    old_rows, old_hash = recovery_rows(old_mapping, "old mapping")
    new_rows, new_hash = recovery_rows(new_mapping, "new mapping")
    if old_hash != new_hash or old_rows != new_rows:
        raise ValueError("old/new recovery subdocuments differ")
    return new_rows, new_hash


def validate_old_evidence(
    document: dict[str, Any],
    *,
    old_mapping_sha256: str,
) -> tuple[dict[int, dict[str, Any]], str]:
    evidence_hash = self_hash(document, "old recovery evidence")
    if (
        document.get("version") != 1
        or document.get("status") != "PASS"
        or document.get("mapping_sha256") != old_mapping_sha256
        or document.get("database_writes_executed") is not False
        or document.get("production_connections_opened") is not False
    ):
        raise ValueError("old recovery evidence contract differs")
    rows = document.get("recoveries")
    if not isinstance(rows, list):
        raise ValueError("old recovery evidence lacks recoveries")
    selected = {
        row.get("missing_task_asset_id"): row
        for row in rows
        if isinstance(row, dict)
        and row.get("missing_task_asset_id") in RECOVERY_IDS
    }
    if tuple(sorted(selected)) != RECOVERY_IDS or len(selected) != len(rows):
        raise ValueError("old recovery evidence scope differs")
    return selected, evidence_hash


def validate_old_plan(
    document: dict[str, Any],
    *,
    old_mapping_sha256: str,
) -> tuple[dict[int, dict[str, Any]], str]:
    evidence_hash = self_hash(document, "old recovery plan")
    if (
        document.get("version") != 1
        or document.get("status") != "PREPARED"
        or document.get("target_environment") != "production"
        or document.get("production_release") != "v1.295"
        or document.get("mapping_sha256") != old_mapping_sha256
        or document.get("database_writes_executed") is not False
        or document.get("production_writes_executed") is not False
    ):
        raise ValueError("old recovery plan contract differs")
    entries = document.get("entries")
    if not isinstance(entries, list):
        raise ValueError("old recovery plan lacks entries")
    selected = {
        entry.get("missing_task_asset_id"): entry
        for entry in entries
        if isinstance(entry, dict)
        and entry.get("missing_task_asset_id") in RECOVERY_IDS
    }
    if tuple(sorted(selected)) != RECOVERY_IDS or len(selected) != len(entries):
        raise ValueError("old recovery plan scope differs")
    return selected, evidence_hash


def validate_current_target(
    *,
    missing_id: int,
    mapping_row: dict[str, Any],
    evidence_row: dict[str, Any],
    plan_entry: dict[str, Any],
    task_assets: dict[int, dict[str, Any]],
    objects: dict[str, dict[str, Any]],
) -> tuple[str, int, str]:
    target = task_assets.get(missing_id)
    source_id = int(mapping_row["recovery_source_task_asset_id"])
    source = task_assets.get(source_id)
    if target is None or source is None:
        raise ValueError(f"physical A recovery {missing_id} lacks task assets")
    content_hash = require_hash(
        evidence_row.get("source_sha256"),
        f"recovery {missing_id} source SHA-256",
    )
    source_fetch = evidence_row.get("source_fetch_receipt")
    if not isinstance(source_fetch, dict):
        raise ValueError(f"recovery {missing_id} lacks source fetch receipt")
    target_key = str(plan_entry.get("target_object_key") or "")
    target_ref = str(plan_entry.get("target_storage_ref_id") or "")
    db_plan = plan_entry.get("db_apply_plan")
    if not isinstance(db_plan, dict):
        raise ValueError(f"recovery {missing_id} lacks db plan")
    insert_ref = db_plan.get("insert_asset_storage_ref")
    update_asset = db_plan.get("update_task_asset")
    if not isinstance(insert_ref, dict) or not isinstance(update_asset, dict):
        raise ValueError(f"recovery {missing_id} db plan differs")
    update_set = update_asset.get("set")
    if not isinstance(update_set, dict):
        raise ValueError(f"recovery {missing_id} task update differs")
    object_row = objects.get(target_ref)
    source_path = pathlib.Path(str(evidence_row.get("source_local_path") or ""))
    if (
        not source_path.is_file()
        or source_path.is_symlink()
        or sha256_file(source_path) != content_hash
    ):
        raise ValueError(f"recovery {missing_id} source bytes differ")
    expected_size = int(mapping_row["expected_file_size"])
    expected_mime = str(source.get("mime_type") or "")
    if (
        target.get("task_id") != mapping_row.get("task_id")
        or target.get("storage_ref_id") != target_ref
        or target.get("storage_key") != target_key
        or target.get("whole_hash") != content_hash
        or int(target.get("file_size") or 0) != expected_size
        or target.get("mime_type") != expected_mime
        or target.get("deleted_at") is not None
        or target.get("object_deleted_at") is not None
        or source.get("storage_ref_id")
        != mapping_row.get("recovery_source_storage_ref_id")
        or int(source.get("file_size") or 0) != expected_size
        or source_fetch.get("sha256") != content_hash
        or int(source_fetch.get("size") or 0) != expected_size
        or insert_ref.get("ref_id") != target_ref
        or insert_ref.get("ref_key") != target_key
        or insert_ref.get("checksum_hint") != content_hash
        or update_set.get("storage_ref_id") != target_ref
        or update_set.get("storage_key") != target_key
        or update_set.get("whole_hash") != content_hash
        or not isinstance(object_row, dict)
        or object_row.get("ref_key") != target_key
        or object_row.get("checksum_hint") != content_hash
        or int(object_row.get("file_size") or 0) != expected_size
        or object_row.get("mime_type") != expected_mime
    ):
        raise ValueError(f"physical A recovery target {missing_id} differs")
    return content_hash, expected_size, expected_mime


def atomic_write(path: pathlib.Path, content: bytes) -> None:
    if path.exists():
        if path.is_file() and not path.is_symlink() and path.read_bytes() == content:
            return
        raise FileExistsError(f"refusing to overwrite non-identical output {path}")
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(
        mode="wb", dir=path.parent, prefix=f".{path.name}.", delete=False
    ) as handle:
        temporary = pathlib.Path(handle.name)
        handle.write(content)
        handle.flush()
        os.fsync(handle.fileno())
    try:
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)


def materialize_evidence_link(
    *,
    source: pathlib.Path,
    target: pathlib.Path,
    expected_sha256: str,
    expected_size: int,
) -> os.stat_result:
    if target.exists() or target.is_symlink():
        if (
            not target.is_file()
            or target.is_symlink()
            or target.stat().st_size != expected_size
            or sha256_file(target) != expected_sha256
        ):
            raise FileExistsError(f"existing recovery evidence object differs: {target}")
        return target.stat()
    target.parent.mkdir(parents=True, exist_ok=True)
    os.link(source, target)
    stat = target.stat()
    if (
        stat.st_size != expected_size
        or sha256_file(target) != expected_sha256
    ):
        target.unlink(missing_ok=True)
        raise ValueError(f"recovery evidence object validation failed: {target}")
    return stat


def build(args: argparse.Namespace) -> dict[str, Any]:
    old_mapping_sha = require_file_hash(
        args.old_mapping,
        args.expected_old_mapping_sha256,
        "old mapping",
    )
    new_mapping_sha = require_file_hash(
        args.new_mapping,
        args.expected_new_mapping_sha256,
        "new mapping",
    )
    old_mapping = load_json(args.old_mapping, "old mapping")
    new_mapping = load_json(args.new_mapping, "new mapping")
    mapping_rows, recovery_scope_sha = validate_equal_recovery_scope(
        old_mapping, new_mapping
    )

    old_evidence_file_sha = require_file_hash(
        args.old_recovery_evidence,
        args.expected_old_recovery_evidence_sha256,
        "old recovery evidence",
    )
    old_plan_file_sha = require_file_hash(
        args.old_recovery_plan,
        args.expected_old_recovery_plan_sha256,
        "old recovery plan",
    )
    old_evidence = load_json(args.old_recovery_evidence, "old recovery evidence")
    old_plan = load_json(args.old_recovery_plan, "old recovery plan")
    evidence_rows, old_evidence_hash = validate_old_evidence(
        old_evidence, old_mapping_sha256=old_mapping_sha
    )
    plan_rows, old_plan_hash = validate_old_plan(
        old_plan, old_mapping_sha256=old_mapping_sha
    )
    if old_evidence.get("run_id") != old_plan.get("run_id"):
        raise ValueError("old recovery evidence/plan run_id differs")

    snapshot_verdict = api_oracle.load_snapshot_verdict(
        args.snapshot_verdict,
        run_id=args.run_id,
        expected_snapshot_verdict_sha256=(
            args.expected_snapshot_verdict_sha256
        ),
        expected_clone_a_attestation_sha256=(
            args.expected_clone_a_attestation_sha256
        ),
    )
    attestation = api_oracle.load_clone_a_attestation(
        args.clone_a_attestation,
        run_id=args.run_id,
        expected_clone_a_attestation_sha256=(
            args.expected_clone_a_attestation_sha256
        ),
        expected_source_snapshot_sha256=snapshot_verdict["snapshot_sha256"],
        expected_baseline_fingerprint_sha256=(
            snapshot_verdict["baseline_fingerprint_sha256"]
        ),
        expected_schema_version=2,
    )
    expected_a_manifest_sha = require_file_hash(
        args.a_snapshot_manifest,
        args.expected_a_snapshot_manifest_sha256,
        "physical A manifest",
    )
    snapshot, package = api_oracle.load_a_snapshot_package(
        args.a_snapshot_manifest
    )
    if package["database"] != attestation["clone_database"]:
        raise ValueError("physical A manifest database differs from attestation")
    task_assets = {int(row["id"]): row for row in snapshot["task_assets"]}
    objects = {str(row["ref_id"]): row for row in snapshot["objects"]}

    ownerships = material._load_ownership_receipts(
        tuple(args.old_ownership_receipt),
        str(old_plan["run_id"]),
        "old recovery ownership receipt",
        canonical_newline=False,
    )
    if len(ownerships) != len(RECOVERY_IDS):
        raise ValueError("old recovery ownership receipt count differs")
    ownership_by_hash: dict[str, tuple[pathlib.Path, dict[str, Any], str]] = {}
    for path, document, evidence_hash in ownerships:
        content_hash = require_hash(document.get("sha256"), f"ownership {path}")
        target = pathlib.Path(str(document["target_path"]))
        stat = target.stat()
        if (
            target.is_symlink()
            or not target.is_file()
            or stat.st_dev != int(document["device"])
            or stat.st_ino != int(document["inode"])
            or stat.st_size != int(document["size"])
            or sha256_file(target) != content_hash
            or content_hash in ownership_by_hash
        ):
            raise ValueError(f"old recovery ownership/object differs: {path}")
        ownership_by_hash[content_hash] = (path, document, evidence_hash)

    output_root = args.output_dir.resolve()
    new_plan_entries: list[dict[str, Any]] = []
    new_ownership_hashes: list[str] = []
    for missing_id in RECOVERY_IDS:
        mapping_row = mapping_rows[missing_id]
        evidence_row = evidence_rows[missing_id]
        plan_entry = copy.deepcopy(plan_rows[missing_id])
        content_hash, size, _ = validate_current_target(
            missing_id=missing_id,
            mapping_row=mapping_row,
            evidence_row=evidence_row,
            plan_entry=plan_entry,
            task_assets=task_assets,
            objects=objects,
        )
        prior_ownership = ownership_by_hash.get(content_hash)
        if prior_ownership is None:
            raise ValueError(f"recovery {missing_id} lacks old byte ownership")
        if int(prior_ownership[1]["size"]) != size:
            raise ValueError(f"recovery {missing_id} old ownership size differs")

        source_path = pathlib.Path(str(evidence_row["source_local_path"]))
        target_key = str(plan_entry["target_object_key"])
        mirror = output_root / "object-mirror" / "objects" / target_key
        stat = materialize_evidence_link(
            source=source_path,
            target=mirror,
            expected_sha256=content_hash,
            expected_size=size,
        )
        ownership_path = output_root / f"recovery-ownership-{missing_id}.json"
        ownership = signed(
            {
                "schema_version": 1,
                "run_id": old_plan["run_id"],
                "status": "OWNED_LINK",
                "staging_path": str(source_path.resolve()),
                "target_path": str(mirror),
                "device": stat.st_dev,
                "inode": stat.st_ino,
                "size": stat.st_size,
                "sha256": content_hash,
            }
        )
        atomic_write(ownership_path, canonical(ownership))
        new_ownership_hashes.append(sha256_file(ownership_path))
        rollback = plan_entry.get("rollback_registry")
        if not isinstance(rollback, dict):
            raise ValueError(f"recovery {missing_id} lacks rollback registry")
        rollback["ownership_receipt_path"] = str(ownership_path)
        new_plan_entries.append(plan_entry)

    rebound_evidence_unsigned = {
        key: copy.deepcopy(value)
        for key, value in old_evidence.items()
        if key != "evidence_sha256"
    }
    rebound_evidence_unsigned.update(
        {
            "mapping_sha256": new_mapping_sha,
            "rebind_contract": KIND,
            "rebound_from_mapping_sha256": old_mapping_sha,
            "rebound_from_recovery_evidence_file_sha256": old_evidence_file_sha,
            "rebound_from_recovery_evidence_sha256": old_evidence_hash,
            "rebound_from_recovery_plan_file_sha256": old_plan_file_sha,
            "rebound_from_recovery_plan_sha256": old_plan_hash,
            "recovery_subdocument_sha256": recovery_scope_sha,
            "physical_a_snapshot_manifest_sha256": expected_a_manifest_sha,
            "physical_a_snapshot_evidence_sha256": package["evidence_sha256"],
            "physical_a_attestation_sha256": (
                args.expected_clone_a_attestation_sha256
            ),
            "snapshot_verdict_sha256": (
                args.expected_snapshot_verdict_sha256
            ),
            "old_ownership_receipts_file_sha256": sorted(
                sha256_file(item[0]) for item in ownerships
            ),
            "old_ownership_receipts_evidence_sha256": sorted(
                item[2] for item in ownerships
            ),
        }
    )
    rebound_evidence = signed(rebound_evidence_unsigned)
    evidence_output = output_root / "recovery-evidence-rebound.json"
    atomic_write(evidence_output, canonical(rebound_evidence) + b"\n")

    rebound_plan = signed(
        {
            "version": 1,
            "status": "MATERIALIZED",
            "run_id": old_plan["run_id"],
            "mapping_sha256": new_mapping_sha,
            "database_writes_executed": False,
            "production_writes_executed": False,
            "entries": new_plan_entries,
        }
    )
    plan_output = output_root / "recovery-plan-rebound.json"
    atomic_write(plan_output, canonical(rebound_plan) + b"\n")

    receipt = signed(
        {
            "schema_version": 1,
            "kind": KIND,
            "status": "verified",
            "run_id": args.run_id,
            "old_mapping_sha256": old_mapping_sha,
            "new_mapping_sha256": new_mapping_sha,
            "recovery_subdocument_sha256": recovery_scope_sha,
            "old_recovery_evidence_file_sha256": old_evidence_file_sha,
            "old_recovery_evidence_sha256": old_evidence_hash,
            "old_recovery_plan_file_sha256": old_plan_file_sha,
            "old_recovery_plan_sha256": old_plan_hash,
            "physical_a_snapshot_manifest_sha256": expected_a_manifest_sha,
            "physical_a_snapshot_evidence_sha256": package["evidence_sha256"],
            "physical_a_attestation_sha256": (
                args.expected_clone_a_attestation_sha256
            ),
            "snapshot_verdict_sha256": (
                args.expected_snapshot_verdict_sha256
            ),
            "old_ownership_receipts_file_sha256": sorted(
                sha256_file(item[0]) for item in ownerships
            ),
            "old_ownership_receipts_evidence_sha256": sorted(
                item[2] for item in ownerships
            ),
            "new_ownership_receipts_file_sha256": sorted(
                new_ownership_hashes
            ),
            "rebound_recovery_evidence_file_sha256": (
                sha256_file(evidence_output)
            ),
            "rebound_recovery_evidence_sha256": (
                rebound_evidence["evidence_sha256"]
            ),
            "rebound_recovery_plan_file_sha256": sha256_file(plan_output),
            "rebound_recovery_plan_sha256": rebound_plan["evidence_sha256"],
        }
    )
    receipt_output = output_root / "recovery-rebind-receipt.json"
    atomic_write(receipt_output, canonical(receipt) + b"\n")
    return receipt


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--old-mapping", type=pathlib.Path, required=True)
    parser.add_argument("--expected-old-mapping-sha256", required=True)
    parser.add_argument("--new-mapping", type=pathlib.Path, required=True)
    parser.add_argument("--expected-new-mapping-sha256", required=True)
    parser.add_argument(
        "--old-recovery-evidence", type=pathlib.Path, required=True
    )
    parser.add_argument(
        "--expected-old-recovery-evidence-sha256", required=True
    )
    parser.add_argument("--old-recovery-plan", type=pathlib.Path, required=True)
    parser.add_argument("--expected-old-recovery-plan-sha256", required=True)
    parser.add_argument(
        "--old-ownership-receipt",
        type=pathlib.Path,
        action="append",
        required=True,
    )
    parser.add_argument("--snapshot-verdict", type=pathlib.Path, required=True)
    parser.add_argument(
        "--expected-snapshot-verdict-sha256", required=True
    )
    parser.add_argument(
        "--clone-a-attestation", type=pathlib.Path, required=True
    )
    parser.add_argument(
        "--expected-clone-a-attestation-sha256", required=True
    )
    parser.add_argument(
        "--a-snapshot-manifest", type=pathlib.Path, required=True
    )
    parser.add_argument(
        "--expected-a-snapshot-manifest-sha256", required=True
    )
    parser.add_argument("--output-dir", type=pathlib.Path, required=True)
    return parser.parse_args()


def main() -> int:
    receipt = build(parse_args())
    print(
        json.dumps(
            {
                "status": receipt["status"],
                "evidence_sha256": receipt["evidence_sha256"],
            },
            sort_keys=True,
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
