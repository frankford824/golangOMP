#!/usr/bin/env python3
"""Materialize a reviewed multi-source bundle with reproducible ZIP bytes.

The input is a JSON object with ``bundle_task_asset_id`` and at least two
ordered ``members``. Each member must contain task_asset_id, asset_id,
storage_ref_id, original_file_name, local_path, sha256, source_stage,
evidence_event_ids, and confirmed=true. Local paths are read-only inputs and
are deliberately omitted from the embedded manifest and result.

ZIP_STORED, a fixed DOS timestamp, fixed Unix mode, canonical JSON, and caller
order make the output byte-for-byte reproducible. The tool never uploads or
registers the resulting object; that remains an explicit reviewed step.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import re
import tempfile
import zipfile


SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
EVIDENCE_RE = re.compile(r"^(task_event_log|task_module_event):[^:]+$")
PROFILE = "zip-stored-fixed-1980-0644-v1"
FIXED_TIME = (1980, 1, 1, 0, 0, 0)
ALLOWED_STAGES = {"design", "audit", "customization", "reopen", "retouch", "migration"}


def canonical_json(value: object) -> bytes:
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"), sort_keys=True).encode("utf-8")


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def safe_name(value: str) -> str:
    name = pathlib.PurePath(value).name.strip()
    name = re.sub(r"[^0-9A-Za-z._-]+", "_", name).strip("._")
    return name or "unnamed"


def zip_info(name: str) -> zipfile.ZipInfo:
    info = zipfile.ZipInfo(name, FIXED_TIME)
    info.compress_type = zipfile.ZIP_STORED
    info.create_system = 3
    info.external_attr = (0o100644 << 16)
    info.flag_bits |= 0x800
    return info


def validate_plan(plan: dict) -> list[dict]:
    if plan.get("version") != 1:
        raise ValueError("plan.version must be 1")
    if not isinstance(plan.get("bundle_task_asset_id"), int) or plan["bundle_task_asset_id"] <= 0:
        raise ValueError("bundle_task_asset_id must be a positive integer")
    if not isinstance(plan.get("confirmed_by"), int) or plan["confirmed_by"] <= 0:
        raise ValueError("confirmed_by must be a positive reviewer id")
    if not str(plan.get("confirmed_at") or "").strip() or not str(plan.get("confirmation_note") or "").strip():
        raise ValueError("confirmed_at and confirmation_note are required")
    members = plan.get("members")
    if not isinstance(members, list) or len(members) < 2:
        raise ValueError("members must contain at least two reviewed source files")
    seen: set[int] = set()
    normalized = []
    for index, member in enumerate(members, 1):
        task_asset_id = member.get("task_asset_id")
        asset_id = member.get("asset_id")
        if not isinstance(task_asset_id, int) or task_asset_id <= 0 or task_asset_id in seen:
            raise ValueError(f"members[{index - 1}].task_asset_id must be unique and positive")
        if not isinstance(asset_id, int) or asset_id <= 0:
            raise ValueError(f"members[{index - 1}].asset_id must be positive")
        if member.get("confirmed") is not True:
            raise ValueError(f"members[{index - 1}].confirmed must be true")
        expected_sha = member.get("sha256")
        if not isinstance(expected_sha, str) or not SHA256_RE.fullmatch(expected_sha):
            raise ValueError(f"members[{index - 1}].sha256 must be lowercase SHA-256")
        local_path = pathlib.Path(str(member.get("local_path") or ""))
        if not local_path.is_file():
            raise ValueError(f"members[{index - 1}].local_path is not a regular file")
        actual_sha = sha256_file(local_path)
        if actual_sha != expected_sha:
            raise ValueError(f"members[{index - 1}] byte SHA-256 does not match the reviewed value")
        storage_ref_id = str(member.get("storage_ref_id") or "").strip()
        original_name = str(member.get("original_file_name") or "").strip()
        stage = str(member.get("source_stage") or "").strip()
        evidence = member.get("evidence_event_ids")
        if not storage_ref_id or not original_name:
            raise ValueError(f"members[{index - 1}] requires storage_ref_id and original_file_name")
        if stage not in ALLOWED_STAGES:
            raise ValueError(f"members[{index - 1}].source_stage is invalid")
        if not isinstance(evidence, list) or not evidence or any(not isinstance(v, str) or not EVIDENCE_RE.fullmatch(v) for v in evidence):
            raise ValueError(f"members[{index - 1}].evidence_event_ids must be non-empty namespaced IDs")
        archive_path = f"{index:03d}_{task_asset_id}_{safe_name(original_name)}"
        normalized.append({
            "task_asset_id": task_asset_id,
            "asset_id": asset_id,
            "storage_ref_id": storage_ref_id,
            "original_file_name": original_name,
            "archive_path": archive_path,
            "sha256": expected_sha,
            "source_stage": stage,
            "evidence_event_ids": evidence,
            "confirmed": True,
            "_local_path": local_path,
        })
        seen.add(task_asset_id)
    return normalized


def build(plan_path: pathlib.Path, output_path: pathlib.Path) -> dict:
    if output_path.exists():
        raise FileExistsError(f"refusing to overwrite existing bundle: {output_path}")
    plan = json.loads(plan_path.read_text(encoding="utf-8"))
    members = validate_plan(plan)
    manifest_members = [{k: v for k, v in member.items() if k != "_local_path"} for member in members]
    embedded_manifest = {
        "version": 1,
        "deterministic_profile": PROFILE,
        "confirmation": {
            "confirmed_by": plan["confirmed_by"],
            "confirmed_at": plan["confirmed_at"],
            "confirmation_note": plan["confirmation_note"],
        },
        "members": manifest_members,
    }
    manifest_bytes = canonical_json(embedded_manifest)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(prefix=output_path.name + ".", suffix=".tmp", dir=output_path.parent, delete=False) as handle:
        temporary = pathlib.Path(handle.name)
    try:
        with zipfile.ZipFile(temporary, "w", compression=zipfile.ZIP_STORED, allowZip64=True) as bundle:
            bundle.writestr(zip_info("manifest.json"), manifest_bytes)
            for member in members:
                bundle.writestr(zip_info(member["archive_path"]), member["_local_path"].read_bytes())
        os.replace(temporary, output_path)
    finally:
        if temporary.exists():
            temporary.unlink()

    mapping_members = [
        {"task_asset_id": member["task_asset_id"], "sha256": member["sha256"], "confirmed": True}
        for member in members
    ]
    # Match Go encoding/json field order in sourceBundleManifestHash.
    mapping_manifest = json.dumps(
        {"format": "zip", "members": mapping_members}, ensure_ascii=False, separators=(",", ":")
    ).encode("utf-8")
    return {
        "status": "PASS",
        "violation_count": 0,
        "deterministic_profile": PROFILE,
        "bundle_sha256": sha256_file(output_path),
        "embedded_manifest_sha256": hashlib.sha256(manifest_bytes).hexdigest(),
        "source_bundle": {
            "task_asset_id": plan["bundle_task_asset_id"],
            "format": "zip",
            "bundle_sha256": sha256_file(output_path),
            "manifest_sha256": hashlib.sha256(mapping_manifest).hexdigest(),
            "members": mapping_members,
            "confirmed_by": plan.get("confirmed_by"),
            "confirmed_at": plan.get("confirmed_at"),
            "confirmation_note": plan.get("confirmation_note"),
        },
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--plan", type=pathlib.Path, required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--result", type=pathlib.Path, required=True)
    args = parser.parse_args()
    if args.result.exists():
        raise FileExistsError(f"refusing to overwrite existing result: {args.result}")
    result = build(args.plan, args.output)
    args.result.parent.mkdir(parents=True, exist_ok=True)
    args.result.write_bytes(canonical_json(result) + b"\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
