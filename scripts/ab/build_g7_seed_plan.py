#!/usr/bin/env python3
"""Build a hash-bound, read-only G7 A/B object seed plan.

The tool reads frozen samples, the final object manifest, a reviewed inventory,
and existing local fixture roots.  It never connects to production, downloads
objects, or writes fixture bytes.  Its only write is the new plan JSON.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import tempfile
from typing import Any


SCHEMA_VERSION = 1
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
NAMESPACES = {"A", "B"}
BACKEND_COMBINATIONS = {
    "A": {"external_external", "devplus_external"},
    "B": {"devplus_devplus", "external_devplus"},
}
ROW_FIELDS = {
    "namespace",
    "scenario_ids",
    "task_ids",
    "source_entity_key",
    "object_key",
    "size",
    "mime_type",
    "sha256",
    "expected_http_status",
}
OBJECT_FIELDS = {
    "entity_key",
    "owner_kind",
    "owner_id",
    "task_id",
    "storage_ref_id",
    "storage_adapter",
    "object_key",
    "size",
    "mime_type",
    "sha256",
    "status",
    "is_placeholder",
}


class InputError(ValueError):
    pass


def canonical_json(value: Any) -> str:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    )


def canonical_sha256(value: Any) -> str:
    return hashlib.sha256(canonical_json(value).encode()).hexdigest()


def file_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def read_json(path: Path, label: str) -> dict[str, Any]:
    if path.is_symlink() or not path.is_file():
        raise InputError(f"{label} must be an existing non-symlink file")
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise InputError(f"{label} must be UTF-8 JSON") from exc
    if not isinstance(value, dict):
        raise InputError(f"{label} must contain an object")
    return value


def validate_self_hash(document: dict[str, Any], field: str, label: str) -> None:
    declared = document.get(field)
    payload = dict(document)
    payload.pop(field, None)
    if (
        not isinstance(declared, str)
        or not SHA256_RE.fullmatch(declared)
        or declared != canonical_sha256(payload)
    ):
        raise InputError(f"{label} has an invalid {field}")


def read_object_manifest(path: Path) -> dict[str, dict[str, Any]]:
    if path.is_symlink() or not path.is_file():
        raise InputError("object manifest must be an existing non-symlink file")
    indexed: dict[str, dict[str, Any]] = {}
    with path.open("r", encoding="utf-8") as handle:
        for line_no, line in enumerate(handle, 1):
            if not line.strip():
                continue
            try:
                row = json.loads(line)
            except json.JSONDecodeError as exc:
                raise InputError(
                    f"object manifest row {line_no} is invalid JSON"
                ) from exc
            if not isinstance(row, dict) or set(row) != OBJECT_FIELDS:
                raise InputError(
                    f"object manifest row {line_no} has an invalid shape"
                )
            entity = row.get("entity_key")
            if not isinstance(entity, str) or not entity or entity in indexed:
                raise InputError("object manifest has duplicate/invalid entity keys")
            indexed[entity] = row
    if not indexed:
        raise InputError("object manifest is empty")
    return indexed


def samples_by_namespace(
    samples: dict[str, Any],
) -> dict[str, dict[str, set[int]]]:
    if (
        samples.get("schema_version") != SCHEMA_VERSION
        or samples.get("gate") != "G7"
        or samples.get("status") != "PASS"
        or samples.get("mode") != "final"
    ):
        raise InputError("samples must be the final PASS G7 manifest")
    validate_self_hash(samples, "manifest_sha256", "samples")
    rows = samples.get("samples")
    if not isinstance(rows, list) or not rows:
        raise InputError("samples contain no READY rows")
    result = {"A": {}, "B": {}}
    for sample in rows:
        if not isinstance(sample, dict) or sample.get("status") != "READY":
            raise InputError("samples contain a non-READY row")
        scenario = sample.get("scenario_id")
        if not isinstance(scenario, str) or not scenario:
            raise InputError("sample scenario_id is invalid")
        coverage = sample.get("coverage_matrix")
        if not isinstance(coverage, list):
            raise InputError(f"sample {scenario} has no coverage matrix")
        for namespace, combinations in BACKEND_COMBINATIONS.items():
            task_ids = {
                row.get("task_id")
                for row in coverage
                if isinstance(row, dict)
                and row.get("combination") in combinations
                and isinstance(row.get("task_id"), int)
                and not isinstance(row.get("task_id"), bool)
                and row["task_id"] > 0
            }
            if task_ids:
                result[namespace][scenario] = task_ids
    return result


def safe_object_key(value: Any) -> bool:
    return (
        isinstance(value, str)
        and bool(value)
        and not value.startswith("/")
        and "\\" not in value
        and "\x00" not in value
        and all(segment not in {"", ".", ".."} for segment in value.split("/"))
    )


def parse_roots(values: list[str]) -> dict[str, list[Path]]:
    result = {"A": [], "B": []}
    for raw in values:
        namespace, separator, path_value = raw.partition("=")
        if separator != "=" or namespace not in NAMESPACES or not path_value:
            raise InputError("--existing-root must be A=/path or B=/path")
        root = Path(path_value)
        if root.is_symlink() or not root.is_dir():
            raise InputError("existing fixture root must be a non-symlink directory")
        result[namespace].append(root.resolve(strict=True))
    return result


def existing_object(
    roots: list[Path],
    object_key: str,
    expected_size: int,
    expected_sha256: str,
) -> tuple[str | None, str | None]:
    for root in roots:
        candidate = root / "objects"
        for segment in object_key.split("/"):
            candidate = candidate / segment
            if candidate.is_symlink():
                return None, "existing fixture path contains a symlink"
        if not candidate.exists():
            continue
        if not candidate.is_file():
            return None, "existing fixture object is not a regular file"
        resolved = candidate.resolve(strict=True)
        try:
            resolved.relative_to(root)
        except ValueError:
            return None, "existing fixture object escaped its root"
        if (
            resolved.stat().st_size != expected_size
            or file_sha256(resolved) != expected_sha256
        ):
            return None, "existing fixture object differs from the frozen bytes"
        return str(root), None
    return None, None


def build_plan(
    *,
    samples_path: Path,
    object_manifest_path: Path,
    inventory_path: Path,
    roots: dict[str, list[Path]],
    historical_unavailable_entity: str,
) -> dict[str, Any]:
    samples = read_json(samples_path, "samples")
    inventory = read_json(inventory_path, "inventory")
    sample_hash = file_sha256(samples_path)
    object_hash = file_sha256(object_manifest_path)
    if (
        inventory.get("schema_version") != SCHEMA_VERSION
        or inventory.get("gate") != "G7"
        or inventory.get("status") != "REVIEWED"
        or inventory.get("samples_sha256") != sample_hash
        or inventory.get("object_manifest_sha256") != object_hash
        or not isinstance(inventory.get("run_id"), str)
        or not inventory["run_id"]
    ):
        raise InputError("inventory is not bound to the frozen G7 inputs")
    validate_self_hash(inventory, "manifest_sha256", "inventory")
    sample_tasks = samples_by_namespace(samples)
    objects = read_object_manifest(object_manifest_path)
    rows = inventory.get("rows")
    if not isinstance(rows, list) or not rows:
        raise InputError("inventory rows must be a non-empty array")
    seen: set[tuple[str, str]] = set()
    plan_rows: list[dict[str, Any]] = []
    blockers: list[dict[str, str]] = []
    for index, row in enumerate(rows):
        label = f"inventory.rows[{index}]"
        if not isinstance(row, dict) or set(row) != ROW_FIELDS:
            raise InputError(f"{label} has an invalid shape")
        namespace = row["namespace"]
        scenarios = row["scenario_ids"]
        task_ids = row["task_ids"]
        entity = row["source_entity_key"]
        object_key = row["object_key"]
        size = row["size"]
        sha256 = row["sha256"]
        status = row["expected_http_status"]
        if (
            namespace not in NAMESPACES
            or not isinstance(scenarios, list)
            or not scenarios
            or any(not isinstance(value, str) or not value for value in scenarios)
            or len(scenarios) != len(set(scenarios))
            or not isinstance(task_ids, list)
            or not task_ids
            or any(
                not isinstance(value, int)
                or isinstance(value, bool)
                or value <= 0
                for value in task_ids
            )
            or len(task_ids) != len(set(task_ids))
            or not isinstance(entity, str)
            or not entity
            or not safe_object_key(object_key)
            or not isinstance(size, int)
            or isinstance(size, bool)
            or size < 0
            or not isinstance(row["mime_type"], str)
            or not row["mime_type"].strip()
            or not isinstance(sha256, str)
            or not SHA256_RE.fullmatch(sha256)
            or status not in {200, 410}
        ):
            raise InputError(f"{label} contains an invalid value")
        key = (namespace, object_key)
        if key in seen:
            raise InputError(f"{label} duplicates a namespace/object key")
        seen.add(key)
        for scenario in scenarios:
            allowed_tasks = sample_tasks[namespace].get(scenario)
            if not allowed_tasks or not set(task_ids).issubset(allowed_tasks):
                raise InputError(
                    f"{label} is outside the selected {namespace} sample tasks"
                )
        source = objects.get(entity)
        if source is None:
            raise InputError(f"{label} source entity is absent from object manifest")
        if (
            source["task_id"] not in task_ids
            or source["object_key"] != object_key
            or source["size"] != size
            or source["mime_type"] != row["mime_type"]
            or source["sha256"] != sha256
        ):
            raise InputError(f"{label} differs from the final object manifest")
        if status == 410 and entity != historical_unavailable_entity:
            raise InputError(f"{label} uses an unapproved 410 exception")
        if status == 200 and entity == historical_unavailable_entity:
            raise InputError(f"{label} must preserve the approved 410 absence")
        existing_root, problem = existing_object(
            roots[namespace], object_key, size, sha256
        )
        if problem:
            blockers.append(
                {
                    "namespace": namespace,
                    "object_key_sha256": hashlib.sha256(
                        object_key.encode()
                    ).hexdigest(),
                    "reason": problem,
                }
            )
        action = (
            "preserve_absent"
            if status == 410
            else "reuse_verified"
            if existing_root
            else "fetch_production_readonly"
        )
        plan_rows.append(
            {
                **row,
                "action": action,
                "existing_root_sha256": (
                    hashlib.sha256(existing_root.encode()).hexdigest()
                    if existing_root
                    else None
                ),
                "row_sha256": canonical_sha256(row),
            }
        )
    plan_rows.sort(key=lambda row: (row["namespace"], row["object_key"]))
    fetch_rows = [
        row for row in plan_rows if row["action"] == "fetch_production_readonly"
    ]
    output: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "gate": "G7",
        "status": "BLOCKED" if blockers else "READY_FOR_LOCAL_SEED",
        "run_id": inventory["run_id"],
        "input_sha256": {
            "samples": sample_hash,
            "samples_manifest": samples["manifest_sha256"],
            "object_manifest": object_hash,
            "inventory": file_sha256(inventory_path),
            "inventory_manifest": inventory["manifest_sha256"],
        },
        "constraints": {
            "production_methods_allowed": ["GET"],
            "production_write_authorized": False,
            "fixture_download_performed": False,
            "fixture_write_performed": False,
            "fixture_write_deferred": True,
        },
        "summary": {
            "plan_row_count": len(plan_rows),
            "expected_present_count": sum(
                row["expected_http_status"] == 200 for row in plan_rows
            ),
            "expected_absent_count": sum(
                row["expected_http_status"] == 410 for row in plan_rows
            ),
            "reuse_count": sum(
                row["action"] == "reuse_verified" for row in plan_rows
            ),
            "fetch_count": len(fetch_rows),
            "fetch_bytes": sum(row["size"] for row in fetch_rows),
            "blocker_count": len(blockers),
        },
        "rows": plan_rows,
        "blockers": blockers,
    }
    output["manifest_sha256"] = canonical_sha256(output)
    return output


def write_exclusive(path: Path, document: dict[str, Any]) -> None:
    if path.exists():
        raise InputError("refusing to overwrite an existing seed plan")
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = (json.dumps(
        document, ensure_ascii=False, indent=2, sort_keys=True
    ) + "\n").encode()
    with tempfile.NamedTemporaryFile(
        dir=path.parent, prefix=path.name + ".", suffix=".tmp", delete=False
    ) as handle:
        temporary = Path(handle.name)
        handle.write(payload)
        handle.flush()
        os.fsync(handle.fileno())
    try:
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--samples", required=True)
    parser.add_argument("--object-manifest", required=True)
    parser.add_argument("--inventory", required=True)
    parser.add_argument("--existing-root", action="append", default=[])
    parser.add_argument(
        "--historical-unavailable-entity",
        default="task_asset:12323",
    )
    parser.add_argument("--output", required=True)
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    try:
        plan = build_plan(
            samples_path=Path(args.samples),
            object_manifest_path=Path(args.object_manifest),
            inventory_path=Path(args.inventory),
            roots=parse_roots(args.existing_root),
            historical_unavailable_entity=args.historical_unavailable_entity,
        )
        write_exclusive(Path(args.output), plan)
    except (InputError, OSError) as exc:
        print(f"build_g7_seed_plan: {exc}", file=os.sys.stderr)
        return 2
    return 3 if plan["status"] == "BLOCKED" else 0


if __name__ == "__main__":
    raise SystemExit(main())
