#!/usr/bin/env python3
"""Build the non-circular G6 API oracle from frozen Clone A evidence.

Clone B is deliberately not an input.  Runtime-created identities are accepted
only when they were allocated by an approved, hash-bound materialization
receipt.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import re
from collections.abc import Iterable
from typing import Any

if __package__:
    from scripts.ab import export_frozen_a_oracle as frozen_export
else:
    import export_frozen_a_oracle as frozen_export


SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
ELIGIBLE_APPROVED_TYPES = frozenset(
    {"delivery", "draft", "revised", "final", "outsource_return"}
)


def canonical(value: object) -> bytes:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")


def sha256(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def require_sha256(value: object, label: str, *, allow_empty: bool = False) -> str:
    text = str(value or "")
    if allow_empty and not text:
        return ""
    if not SHA256_RE.fullmatch(text):
        raise ValueError(f"{label} must be a lowercase SHA-256")
    return text


def nullable_int(value: object) -> int | None:
    if value is None:
        return None
    text = str(value or "")
    return None if text in {"", "NULL", "\\N"} else int(text)


def required_int(value: object, label: str, *, minimum: int = 1) -> int:
    number = int(str(value))
    if number < minimum:
        raise ValueError(f"{label} must be >= {minimum}")
    return number


def required_bool(value: object, label: str) -> bool:
    if isinstance(value, bool):
        return value
    if str(value) in {"1", "true"}:
        return True
    if str(value) in {"0", "false"}:
        return False
    raise ValueError(f"{label} must be boolean")


def load_a_snapshot_package(
    manifest_path: pathlib.Path,
) -> tuple[dict[str, list[dict[str, Any]]], dict[str, str]]:
    """Load and independently validate the formal frozen-A export package."""
    manifest_bytes = manifest_path.read_bytes()
    manifest = json.loads(manifest_bytes)
    expected_top_fields = {
        "database",
        "datasets",
        "evidence_sha256",
        "export_contract",
        "mysql_evidence",
        "schema_version",
        "transaction",
    }
    if not isinstance(manifest, dict) or set(manifest) != expected_top_fields:
        raise ValueError("frozen A exporter manifest field contract differs")
    unsigned = {
        key: value for key, value in manifest.items() if key != "evidence_sha256"
    }
    if (
        manifest["schema_version"] != frozen_export.SCHEMA_VERSION
        or manifest["export_contract"] != "frozen_a_oracle_v2"
        or manifest["evidence_sha256"]
        != sha256(frozen_export.canonical_json_bytes(unsigned))
        or manifest["transaction"]
        != {
            "access_mode": "READ ONLY",
            "consistent_snapshot": True,
            "isolation_level": "REPEATABLE READ",
            "session_time_zone": "+00:00",
            "single_connection": True,
        }
    ):
        raise ValueError("frozen A exporter manifest evidence differs")
    require_sha256(
        manifest["evidence_sha256"], "frozen A exporter manifest evidence"
    )
    mysql_evidence = manifest["mysql_evidence"]
    if (
        not isinstance(mysql_evidence, dict)
        or mysql_evidence.get("session_time_zone") != "+00:00"
    ):
        raise ValueError("frozen A exporter MySQL evidence differs")
    dataset_rows = manifest["datasets"]
    if not isinstance(dataset_rows, list) or len(dataset_rows) != len(
        frozen_export.DATASETS
    ):
        raise ValueError("frozen A exporter dataset manifest count differs")
    by_name: dict[str, dict[str, Any]] = {}
    for index, dataset_manifest in enumerate(dataset_rows):
        if not isinstance(dataset_manifest, dict) or set(dataset_manifest) != {
            "columns_sha256",
            "dataset",
            "dataset_sha256",
            "file",
            "file_sha256",
            "first_key",
            "key",
            "last_key",
            "row_count",
            "schema",
            "schema_sha256",
            "source_table",
        }:
            raise ValueError(
                f"frozen A dataset manifest {index} field contract differs"
            )
        name = str(dataset_manifest["dataset"])
        if name in by_name:
            raise ValueError(f"frozen A dataset manifest duplicates {name}")
        by_name[name] = dataset_manifest

    output: dict[str, list[dict[str, Any]]] = {}
    package_dir = manifest_path.resolve().parent
    for spec in frozen_export.DATASETS:
        dataset_manifest = by_name.get(spec.name)
        if dataset_manifest is None:
            raise ValueError(f"frozen A dataset manifest lacks {spec.name}")
        expected_columns_hash = sha256(
            frozen_export.canonical_json_bytes(
                {
                    "dataset": spec.name,
                    "excluded_schema_columns": list(
                        spec.excluded_schema_columns
                    ),
                    "selected_columns": list(spec.columns),
                }
            )
        )
        if (
            dataset_manifest["source_table"] != spec.table
            or dataset_manifest["key"] != spec.key
            or dataset_manifest["file"] != f"{spec.name}.ndjson"
            or dataset_manifest["columns_sha256"] != expected_columns_hash
        ):
            raise ValueError(f"frozen A dataset {spec.name} contract differs")
        for field in (
            "columns_sha256",
            "dataset_sha256",
            "file_sha256",
            "schema_sha256",
        ):
            require_sha256(
                dataset_manifest[field],
                f"frozen A dataset {spec.name} {field}",
            )
        schema = dataset_manifest["schema"]
        if (
            not isinstance(schema, list)
            or dataset_manifest["schema_sha256"]
            != sha256(frozen_export.canonical_json_bytes(schema))
        ):
            raise ValueError(
                f"frozen A dataset {spec.name} schema hash mismatch"
            )
        try:
            frozen_export._validate_schema(spec, schema)
        except frozen_export.OracleExportError as exc:
            raise ValueError(
                f"frozen A dataset {spec.name} schema differs: {exc}"
            ) from exc
        dataset_path = (package_dir / dataset_manifest["file"]).resolve()
        if dataset_path.parent != package_dir:
            raise ValueError(f"frozen A dataset {spec.name} path escapes package")
        file_bytes = dataset_path.read_bytes()
        if sha256(file_bytes) != dataset_manifest["file_sha256"]:
            raise ValueError(f"frozen A dataset {spec.name} file hash mismatch")
        parsed_rows: list[dict[str, Any]] = []
        row_hashes: list[str] = []
        keys: list[int | str] = []
        for line_no, line in enumerate(file_bytes.splitlines(), 1):
            if not line:
                raise ValueError(
                    f"frozen A dataset {spec.name} line {line_no} is empty"
                )
            envelope = json.loads(line)
            if not isinstance(envelope, dict) or set(envelope) != {
                "dataset",
                "row",
                "row_key",
                "row_sha256",
            }:
                raise ValueError(
                    f"frozen A dataset {spec.name} line {line_no} envelope differs"
                )
            row = envelope["row"]
            key = envelope["row_key"]
            if (
                envelope["dataset"] != spec.name
                or not isinstance(row, dict)
                or set(row) != set(spec.columns)
                or row[spec.key] != key
            ):
                raise ValueError(
                    f"frozen A dataset {spec.name} line {line_no} row contract differs"
                )
            if spec.key_kind == "integer" and (
                isinstance(key, bool) or not isinstance(key, int)
            ):
                raise ValueError(
                    f"frozen A dataset {spec.name} line {line_no} key type differs"
                )
            if spec.key_kind == "string" and not isinstance(key, str):
                raise ValueError(
                    f"frozen A dataset {spec.name} line {line_no} key type differs"
                )
            row_hash = sha256(
                frozen_export.canonical_json_bytes(
                    {
                        "dataset": spec.name,
                        "row": row,
                        "schema_version": frozen_export.SCHEMA_VERSION,
                    }
                )
            )
            if envelope["row_sha256"] != row_hash:
                raise ValueError(
                    f"frozen A dataset {spec.name} line {line_no} row hash mismatch"
                )
            if keys and key <= keys[-1]:
                raise ValueError(
                    f"frozen A dataset {spec.name} keys are duplicate or out of order"
                )
            keys.append(key)
            row_hashes.append(row_hash)
            parsed_rows.append(dict(row))
        dataset_hash = sha256(
            "".join(f"{item}\n" for item in row_hashes).encode("ascii")
        )
        if (
            len(parsed_rows) != dataset_manifest["row_count"]
            or (keys[0] if keys else None) != dataset_manifest["first_key"]
            or (keys[-1] if keys else None) != dataset_manifest["last_key"]
            or dataset_hash != dataset_manifest["dataset_sha256"]
        ):
            raise ValueError(f"frozen A dataset {spec.name} aggregate differs")
        output[spec.name] = parsed_rows
    if set(by_name) != {spec.name for spec in frozen_export.DATASETS}:
        raise ValueError("frozen A exporter dataset names differ")
    return output, {
        "manifest_sha256": sha256(manifest_bytes),
        "evidence_sha256": manifest["evidence_sha256"],
    }


def load_snapshot_verdict(
    path: pathlib.Path,
    *,
    run_id: str,
    expected_snapshot_verdict_sha256: str,
    expected_clone_a_attestation_sha256: str,
) -> dict[str, str]:
    actual_file_sha256 = sha256(path.read_bytes())
    if actual_file_sha256 != expected_snapshot_verdict_sha256:
        raise ValueError("snapshot verdict file hash differs")
    document = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(document, dict) or set(document) != {
        "baseline_fingerprint_sha256",
        "evidence_sha256",
        "run_id",
        "schema_version",
        "snapshot_sha256",
        "source_attestation_sha256",
        "status",
        "target_attestation_sha256",
        "violation_count",
        "violations",
    }:
        raise ValueError("snapshot verdict field contract differs")
    if (
        document["schema_version"] != 1
        or document["run_id"] != run_id
        or document["status"] != "PASS"
        or document["source_attestation_sha256"]
        != expected_clone_a_attestation_sha256
        or document["violation_count"] != 0
        or document["violations"] != []
    ):
        raise ValueError("snapshot verdict evidence differs")
    for field in (
        "baseline_fingerprint_sha256",
        "evidence_sha256",
        "snapshot_sha256",
        "source_attestation_sha256",
        "target_attestation_sha256",
    ):
        require_sha256(document[field], f"snapshot verdict {field}")
    return {
        "snapshot_sha256": document["snapshot_sha256"],
        "source_attestation_sha256": document["source_attestation_sha256"],
    }


def load_clone_a_attestation(
    path: pathlib.Path,
    *,
    run_id: str,
    expected_clone_a_attestation_sha256: str,
    expected_source_snapshot_sha256: str,
) -> dict[str, str]:
    actual_file_sha256 = sha256(path.read_bytes())
    if actual_file_sha256 != expected_clone_a_attestation_sha256:
        raise ValueError("Clone A attestation file hash differs")
    document = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(document, dict) or set(document) != {
        "baseline_fingerprint_sha256",
        "clone_database",
        "clone_label",
        "import_receipt_sha256",
        "run_id",
        "schema_version",
        "snapshot_sha256",
        "source_coordinates",
    }:
        raise ValueError("Clone A attestation field contract differs")
    coordinates = document["source_coordinates"]
    if (
        document["schema_version"] != 1
        or document["run_id"] != run_id
        or document["clone_label"] != "A"
        or not isinstance(document["clone_database"], str)
        or not document["clone_database"]
        or document["snapshot_sha256"] != expected_source_snapshot_sha256
        or not isinstance(coordinates, dict)
        or coordinates.get("snapshot_sha256") != expected_source_snapshot_sha256
        or not isinstance(coordinates.get("binlog_file"), str)
        or not coordinates.get("binlog_file")
        or not isinstance(coordinates.get("binlog_position"), int)
        or coordinates["binlog_position"] < 0
    ):
        raise ValueError("Clone A attestation evidence differs")
    for field in (
        "baseline_fingerprint_sha256",
        "import_receipt_sha256",
        "snapshot_sha256",
    ):
        require_sha256(document[field], f"Clone A attestation {field}")
    return {
        "clone_database": document["clone_database"],
        "snapshot_sha256": document["snapshot_sha256"],
    }


def canonical_asset_type(value: str) -> str:
    if value == "reference":
        return "reference"
    if value in {"source", "original"}:
        return "source"
    if value in ELIGIBLE_APPROVED_TYPES:
        return "delivery"
    if value in {"preview", "design_thumb", "erp_product_image"}:
        return value
    return ""


def load_manifest(
    path: pathlib.Path,
    run_id: str,
    mapping_sha256: str,
    snapshot_verdict_sha256: str,
) -> tuple[
    dict[int, dict[str, str]],
    dict[str, dict[str, str]],
    set[str],
]:
    tasks: dict[int, dict[str, str]] = {}
    sources: dict[str, dict[str, str]] = {}
    revisions: set[str] = set()
    seen_entities: set[tuple[str, str]] = set()
    with path.open(encoding="utf-8") as handle:
        for line_no, line in enumerate(handle, 1):
            if not line.strip():
                continue
            row = json.loads(line)
            if row.get("run_id") != run_id or row.get("review_state") != "pass":
                continue
            detail_json = row.get("detail_json")
            components = (
                detail_json.get("components")
                if isinstance(detail_json, dict)
                else None
            )
            if not isinstance(components, list):
                raise ValueError(f"manifest line {line_no} lacks components")
            input_hashes = detail_json.get("input_sha256", {})
            bound_mapping = (
                input_hashes.get("mapping_sha256")
                if isinstance(input_hashes, dict)
                else None
            )
            if bound_mapping is not None and bound_mapping != mapping_sha256:
                raise ValueError(f"manifest line {line_no} mapping hash differs")
            bound_baseline = (
                input_hashes.get("baseline_attestation_sha256")
                if isinstance(input_hashes, dict)
                else None
            )
            if (
                bound_baseline is not None
                and bound_baseline != snapshot_verdict_sha256
            ):
                raise ValueError(
                    f"manifest line {line_no} snapshot verdict binding differs"
                )
            entity = str(row.get("entity_key", ""))
            gate = str(row.get("gate_name", ""))
            gate_entity = (gate, entity)
            if gate_entity in seen_entities:
                raise ValueError(f"manifest line {line_no} duplicates {gate_entity}")
            seen_entities.add(gate_entity)
            if gate == "G01":
                if len(components) < 5:
                    raise ValueError(f"manifest line {line_no} has short G01 components")
                task_id = required_int(components[0], "manifest task_id")
                tasks[task_id] = {
                    "task_id": str(task_id),
                    "task_type": str(components[1]),
                    "task_status": str(components[2]),
                    "current_handler_id": str(components[3]),
                    "workflow_revision": str(components[4]),
                }
            elif gate == "G03" and entity.startswith("revision:"):
                revisions.add(":".join(entity.split(":")[1:5]))
            elif gate == "G04" and entity.startswith("revision-source:"):
                if len(components) < 7:
                    raise ValueError(f"manifest line {line_no} has short G04 components")
                locator = ":".join(entity.split(":")[1:5])
                sources[locator] = {
                    "stable_locator": str(components[0]),
                    "asset_type": str(components[1]),
                    "whole_hash": str(components[2]),
                    "binding_state": str(components[3]),
                    "bound_role": str(components[4]),
                    "scope_sku_code": str(components[5]),
                    "retouch_requirement_id": str(components[6]),
                }
    if not tasks or not revisions or not sources:
        raise ValueError("manifest lacks G01 tasks, G03 revisions, or G04 source rows")
    return tasks, sources, revisions


def load_receipts(
    paths: Iterable[pathlib.Path],
    *,
    expected_kind: str,
    mapping_sha256: str,
    manifest_sha256: str,
) -> list[dict[str, Any]]:
    entries: list[dict[str, Any]] = []
    for path in paths:
        document = json.loads(path.read_text(encoding="utf-8"))
        if not isinstance(document, dict) or set(document) != {
            "schema_version",
            "kind",
            "status",
            "mapping_sha256",
            "reviewed_manifest_sha256",
            "source_evidence_sha256",
            "entries",
            "evidence_sha256",
        }:
            raise ValueError(f"receipt {path} field contract differs")
        if (
            document["schema_version"] != 2
            or document["kind"] != expected_kind
            or document["status"] != "approved"
            or document["mapping_sha256"] != mapping_sha256
            or document["reviewed_manifest_sha256"] != manifest_sha256
        ):
            raise ValueError(f"receipt {path} approval binding differs")
        source_evidence = document["source_evidence_sha256"]
        if (
            not isinstance(source_evidence, list)
            or not source_evidence
            or source_evidence != sorted(set(source_evidence))
        ):
            raise ValueError(f"receipt {path} source evidence is invalid")
        for index, digest in enumerate(source_evidence):
            require_sha256(digest, f"receipt {path} source evidence {index}")
        unsigned = {key: value for key, value in document.items() if key != "evidence_sha256"}
        if document["evidence_sha256"] != sha256(canonical(unsigned)):
            raise ValueError(f"receipt {path} evidence hash mismatch")
        if not isinstance(document["entries"], list):
            raise ValueError(f"receipt {path} entries must be an array")
        for entry in document["entries"]:
            if not isinstance(entry, dict):
                raise ValueError(f"receipt {path} has a non-object entry")
            entries.append(dict(entry))
    return entries


def scope_values(
    task_id: int,
    scope_kind: str,
    scope_ref_id: int,
    skus: dict[int, dict[str, Any]],
    retouch_requirements: dict[int, dict[str, Any]],
) -> tuple[str, int | None]:
    if scope_kind == "task":
        if scope_ref_id != 0:
            raise ValueError("task scope_ref_id must be 0")
        return "", None
    if scope_kind == "sku":
        sku = skus.get(scope_ref_id)
        if sku is None or sku["task_id"] != task_id:
            raise ValueError(
                f"sku scope {task_id}:{scope_ref_id} is absent from frozen A"
            )
        return sku["sku_code"], None
    if scope_kind == "retouch_requirement":
        requirement_id = required_int(scope_ref_id, "retouch scope_ref_id")
        requirement = retouch_requirements.get(requirement_id)
        if requirement is None or requirement["task_id"] != task_id:
            raise ValueError(
                f"retouch scope {task_id}:{requirement_id} is absent from frozen A"
            )
        return str(requirement["sku_code"] or ""), requirement_id
    raise ValueError(f"unsupported scope_kind {scope_kind}")


def manifest_locator(version: dict[str, Any]) -> str:
    provenance = version["provenance"]["kind"]
    if provenance == "bundle_receipt":
        return f"bundle:{version['content_sha256']}"
    if version["root_asset_id"] is None or not version["storage_ref_id"]:
        return ""
    return f"asset:{version['root_asset_id']}:{version['storage_ref_id']}"


def version_sort_key(version: dict[str, Any]) -> tuple[int, str, int]:
    return (
        int(version["asset_version_no"]),
        str(version["created_at"]),
        int(version["task_asset_id"]),
    )


def build(
    *,
    run_id: str,
    manifest_path: pathlib.Path,
    reviewed_mapping_path: pathlib.Path,
    expected_mapping_sha256: str,
    snapshot_verdict_path: pathlib.Path,
    expected_snapshot_verdict_sha256: str,
    clone_a_attestation_path: pathlib.Path,
    expected_clone_a_attestation_sha256: str,
    a_snapshot_manifest_path: pathlib.Path,
    bundle_receipt_paths: Iterable[pathlib.Path] = (),
    recovery_receipt_paths: Iterable[pathlib.Path] = (),
) -> dict[str, Any]:
    require_sha256(expected_mapping_sha256, "expected_mapping_sha256")
    require_sha256(
        expected_snapshot_verdict_sha256,
        "expected_snapshot_verdict_sha256",
    )
    require_sha256(
        expected_clone_a_attestation_sha256,
        "expected_clone_a_attestation_sha256",
    )
    mapping_bytes = reviewed_mapping_path.read_bytes()
    mapping_sha256 = sha256(mapping_bytes)
    if mapping_sha256 != expected_mapping_sha256:
        raise ValueError("reviewed mapping hash differs")
    manifest_sha256 = sha256(manifest_path.read_bytes())
    snapshot_verdict = load_snapshot_verdict(
        snapshot_verdict_path,
        run_id=run_id,
        expected_snapshot_verdict_sha256=expected_snapshot_verdict_sha256,
        expected_clone_a_attestation_sha256=(
            expected_clone_a_attestation_sha256
        ),
    )
    clone_a_attestation = load_clone_a_attestation(
        clone_a_attestation_path,
        run_id=run_id,
        expected_clone_a_attestation_sha256=(
            expected_clone_a_attestation_sha256
        ),
        expected_source_snapshot_sha256=snapshot_verdict["snapshot_sha256"],
    )
    manifest_tasks, manifest_sources, expected_revisions = load_manifest(
        manifest_path,
        run_id,
        mapping_sha256,
        expected_snapshot_verdict_sha256,
    )
    bundle_receipt_paths = tuple(bundle_receipt_paths)
    recovery_receipt_paths = tuple(recovery_receipt_paths)
    mapping = json.loads(mapping_bytes)
    if not isinstance(mapping, dict):
        raise ValueError("reviewed mapping must be an object")

    snapshot, snapshot_package = load_a_snapshot_package(
        a_snapshot_manifest_path
    )
    task_rows = snapshot["tasks"]
    root_rows = snapshot["roots"]
    asset_rows = snapshot["task_assets"]
    object_rows = snapshot["objects"]
    sku_rows = snapshot["skus"]
    retouch_rows = snapshot["retouch_requirements"]
    reference_rows = snapshot["reference_file_refs"]

    objects: dict[str, dict[str, Any]] = {}
    for index, row in enumerate(object_rows, 1):
        storage_ref_id = row["ref_id"]
        if not storage_ref_id or storage_ref_id in objects:
            raise ValueError(f"A object row {index} storage_ref_id is invalid or duplicated")
        objects[storage_ref_id] = {
            "storage_ref_id": storage_ref_id,
            "asset_id": nullable_int(row["asset_id"]),
            "owner_type": str(row["owner_type"] or ""),
            "owner_id": nullable_int(row["owner_id"]),
            "object_key": str(row["ref_key"] or ""),
            "object_key_sha256": sha256(str(row["ref_key"] or "").encode()),
            "content_sha256": require_sha256(
                row["checksum_hint"],
                f"A object {storage_ref_id} content",
                allow_empty=True,
            ),
            "size": required_int(row["file_size"], "A object size", minimum=0),
            "mime_type": str(row["mime_type"] or ""),
            "object_status": str(row["status"] or ""),
            "is_placeholder": required_bool(
                row["is_placeholder"], "A object is_placeholder"
            ),
        }

    frozen_tasks: dict[int, dict[str, Any]] = {}
    for row in task_rows:
        task_id = required_int(row["id"], "A task_id")
        if task_id in frozen_tasks:
            raise ValueError(f"duplicate A task {task_id}")
        frozen_tasks[task_id] = {
            "task_id": task_id,
            "task_type": str(row["task_type"] or ""),
            "task_status": str(row["task_status"] or ""),
            "current_handler_id": nullable_int(row["current_handler_id"]),
            "workflow_revision": required_int(
                row["workflow_revision"], "A workflow_revision", minimum=0
            ),
            "owner_department_id": nullable_int(row["owner_department_id"]),
            "owner_team_id": nullable_int(row["owner_team_id"]),
        }
    if set(frozen_tasks) != set(manifest_tasks):
        raise ValueError("frozen A tasks do not exactly cover manifest G01 tasks")

    skus: dict[int, dict[str, Any]] = {}
    for row in sku_rows:
        sku_id = required_int(row["id"], "A sku_item_id")
        task_id = required_int(row["task_id"], "A sku task_id")
        if sku_id in skus or task_id not in frozen_tasks or not row["sku_code"]:
            raise ValueError(f"A sku {sku_id} is invalid or duplicated")
        skus[sku_id] = {
            "sku_item_id": sku_id,
            "task_id": task_id,
            "sku_code": row["sku_code"],
        }

    retouch_requirements: dict[int, dict[str, Any]] = {}
    for row in retouch_rows:
        requirement_id = required_int(row["id"], "A retouch requirement id")
        task_id = required_int(row["task_id"], "A retouch task_id")
        if (
            requirement_id in retouch_requirements
            or task_id not in frozen_tasks
            or row["deleted_at"] is not None
        ):
            raise ValueError(
                f"A retouch requirement {requirement_id} is invalid or duplicated"
            )
        retouch_requirements[requirement_id] = {
            "retouch_requirement_id": requirement_id,
            "task_id": task_id,
            "sku_code": str(row["sku_code"] or ""),
        }

    reference_file_refs: dict[int, dict[str, Any]] = {}
    for row in reference_rows:
        reference_id = required_int(row["id"], "A reference file ref id")
        task_id = required_int(row["task_id"], "A reference task_id")
        if reference_id in reference_file_refs or task_id not in frozen_tasks:
            raise ValueError(
                f"A reference file ref {reference_id} is invalid or duplicated"
            )
        reference_file_refs[reference_id] = {
            "reference_file_ref_id": reference_id,
            "task_id": task_id,
            "sku_item_id": nullable_int(row["sku_item_id"]),
            "retouch_requirement_id": nullable_int(
                row["retouch_requirement_id"]
            ),
            "ref_id": str(row["ref_id"] or ""),
        }

    roots: dict[int, dict[str, Any]] = {}
    for row in root_rows:
        root_id = required_int(row["id"], "A root_asset_id")
        task_id = required_int(row["task_id"], "A root task_id")
        if root_id in roots or task_id not in frozen_tasks:
            raise ValueError(f"A root {root_id} is invalid or duplicated")
        if not canonical_asset_type(str(row["asset_type"] or "")):
            raise ValueError(f"A root {root_id} has invalid intrinsic type")
        roots[root_id] = {
            "root_asset_id": root_id,
            "task_id": task_id,
            "intrinsic_asset_type": str(row["asset_type"] or ""),
            "scope_sku_code": str(row["scope_sku_code"] or ""),
            "retouch_requirement_id": nullable_int(row["retouch_requirement_id"]),
            "a_current_version_id": nullable_int(row["current_version_id"]),
            "current_locator": None,
            "approved_locator": None,
            "provenance": {"kind": "a_preserved"},
        }

    versions_by_id: dict[int, dict[str, Any]] = {}
    versions_by_locator: dict[str, dict[str, Any]] = {}
    for row in asset_rows:
        task_asset_id = required_int(row["id"], "A task_asset_id")
        task_id = required_int(row["task_id"], "A asset task_id")
        root_id = required_int(row["asset_id"], "A asset root_asset_id")
        root = roots.get(root_id)
        if (
            task_asset_id in versions_by_id
            or root is None
            or root["task_id"] != task_id
        ):
            raise ValueError(f"A task asset {task_asset_id} is invalid or duplicated")
        intrinsic_type = str(row["asset_type"] or "")
        if not canonical_asset_type(intrinsic_type):
            raise ValueError(f"A task asset {task_asset_id} has invalid intrinsic type")
        storage_ref_id = row["storage_ref_id"]
        object_row = objects.get(storage_ref_id) if storage_ref_id else None
        if object_row is not None:
            if (
                required_int(row["file_size"], "A asset file_size", minimum=0)
                != object_row["size"]
                or str(row["mime_type"] or "") != object_row["mime_type"]
                or object_row["owner_type"] != "task_asset"
                or object_row["owner_id"] != task_asset_id
                or object_row["asset_id"] != task_asset_id
            ):
                raise ValueError(f"A task asset {task_asset_id} object projection differs")
            whole_hash = require_sha256(
                row["whole_hash"],
                f"A task asset {task_asset_id} whole_hash",
                allow_empty=True,
            )
            if (
                whole_hash
                and object_row["content_sha256"]
                and whole_hash != object_row["content_sha256"]
            ):
                raise ValueError(f"A task asset {task_asset_id} content hash differs")
            content_hash = whole_hash or object_row["content_sha256"]
            effective_storage_key = str(
                row["storage_key"] or object_row["object_key"]
            )
            object_key_hash = sha256(effective_storage_key.encode())
        else:
            content_hash = require_sha256(
                row["whole_hash"],
                f"A task asset {task_asset_id} whole_hash",
                allow_empty=True,
            )
            object_key_hash = require_sha256(
                sha256(str(row["storage_key"]).encode())
                if row["storage_key"]
                else "",
                f"A task asset {task_asset_id} storage key",
                allow_empty=True,
            )
            if storage_ref_id and not row["deleted_at"] and not row["cleaned_at"]:
                raise ValueError(
                    f"A task asset {task_asset_id} lacks frozen object evidence"
                )
        locator = (
            f"a:{task_asset_id}:{storage_ref_id}:{object_key_hash}:{content_hash}"
        )
        version = {
            "stable_locator": locator,
            "task_asset_id": task_asset_id,
            "task_id": task_id,
            "root_asset_id": root_id,
            "intrinsic_asset_type": intrinsic_type,
            "scope_sku_code": str(row["scope_sku_code"] or ""),
            "retouch_requirement_id": nullable_int(row["retouch_requirement_id"]),
            "storage_ref_id": storage_ref_id,
            "object_key_sha256": object_key_hash,
            "content_sha256": content_hash,
            "size": required_int(row["file_size"], "A asset file_size", minimum=0),
            "mime_type": str(row["mime_type"] or ""),
            "upload_status": str(row["upload_status"] or ""),
            "deleted_at": row["deleted_at"] or "",
            "cleaned_at": row["cleaned_at"] or "",
            "object_deleted_at": row["object_deleted_at"] or "",
            "asset_version_no": required_int(
                row["asset_version_no"], "A asset_version_no", minimum=0
            ),
            "flow_review_status": str(row["flow_review_status"] or ""),
            "approved_at": str(row["approved_at"] or ""),
            "approved_by": nullable_int(row["approved_by"]),
            "created_at": str(row["created_at"] or ""),
            "source_asset_version_id": nullable_int(row["source_asset_version_id"]),
            "content_availability": (
                "available"
                if object_row is not None
                and object_row["object_status"] == "recorded"
                and not object_row["is_placeholder"]
                else "unverified"
            ),
            "expected_roles": set(),
            "provenance": {
                "kind": "a_preserved",
                "a_binding_state": str(row["binding_state"] or ""),
                "a_bound_role": str(row["bound_role"] or ""),
            },
        }
        versions_by_id[task_asset_id] = version
        versions_by_locator[locator] = version

    for root_id, root in roots.items():
        current_id = root.pop("a_current_version_id")
        if current_id is not None:
            current = versions_by_id.get(current_id)
            if current is None or current["root_asset_id"] != root_id:
                raise ValueError(f"A root {root_id} current pointer is broken")
            root["current_locator"] = current["stable_locator"]

    organization_targets: dict[int, tuple[int | None, int | None]] = {}
    org_rows = mapping.get("organization_mappings")
    if not isinstance(org_rows, list):
        raise ValueError("reviewed mapping lacks organization_mappings")
    for index, row in enumerate(org_rows):
        if not isinstance(row, dict) or row.get("subject_type") != "task":
            continue
        task_id = required_int(row.get("subject_id"), "organization subject_id")
        if task_id not in frozen_tasks:
            continue
        if (
            row.get("confidence") != "confirmed_auto"
            or not isinstance(row.get("confirmed_by"), int)
            or row["confirmed_by"] <= 0
            or task_id in organization_targets
        ):
            raise ValueError(f"organization mapping {index} is not uniquely confirmed")
        organization_targets[task_id] = (
            nullable_int(row.get("target_department_id")),
            nullable_int(row.get("target_team_id")),
        )

    tasks: list[dict[str, Any]] = []
    for task_id in sorted(frozen_tasks):
        frozen = frozen_tasks[task_id]
        expected = manifest_tasks[task_id]
        target_org = organization_targets.get(
            task_id,
            (frozen["owner_department_id"], frozen["owner_team_id"]),
        )
        tasks.append(
            {
                "task_id": task_id,
                "task_type": expected["task_type"],
                "task_status": expected["task_status"],
                "current_handler_id": nullable_int(expected["current_handler_id"]),
                "workflow_revision": required_int(
                    expected["workflow_revision"],
                    "manifest workflow_revision",
                    minimum=0,
                ),
                "owner_department_id": target_org[0],
                "owner_team_id": target_org[1],
            }
        )

    bundle_entries = load_receipts(
        bundle_receipt_paths,
        expected_kind="bundle_materialization_v2",
        mapping_sha256=mapping_sha256,
        manifest_sha256=manifest_sha256,
    )
    recovery_entries = load_receipts(
        recovery_receipt_paths,
        expected_kind="recovery_materialization_v2",
        mapping_sha256=mapping_sha256,
        manifest_sha256=manifest_sha256,
    )
    bundle_by_revision: dict[str, dict[str, Any]] = {}
    for index, entry in enumerate(bundle_entries):
        required = {
            "task_id",
            "scope_kind",
            "scope_ref_id",
            "revision_no",
            "bundle_task_asset_id",
            "bundle_root_asset_id",
            "bundle_storage_ref_id",
            "object_key_sha256",
            "bundle_sha256",
            "internal_manifest_sha256",
            "size",
            "mime_type",
            "members",
        }
        if set(entry) != required:
            raise ValueError(f"bundle receipt entry {index} field contract differs")
        locator = (
            f"{entry['task_id']}:{entry['scope_kind']}:{entry['scope_ref_id']}:"
            f"{entry['revision_no']}"
        )
        if locator in bundle_by_revision:
            raise ValueError(f"duplicate bundle receipt {locator}")
        require_sha256(entry["object_key_sha256"], f"bundle {locator} object key")
        require_sha256(entry["bundle_sha256"], f"bundle {locator} content")
        require_sha256(
            entry["internal_manifest_sha256"], f"bundle {locator} manifest"
        )
        if entry["mime_type"] != "application/zip" or not isinstance(
            entry["members"], list
        ):
            raise ValueError(f"bundle receipt {locator} payload is invalid")
        for member_index, member in enumerate(entry["members"]):
            if not isinstance(member, dict) or set(member) != {
                "task_asset_id",
                "storage_ref_id",
                "size",
                "mime_type",
                "sha256",
            }:
                raise ValueError(
                    f"bundle receipt {locator} member {member_index} "
                    "field contract differs"
                )
            required_int(
                member["task_asset_id"],
                f"bundle {locator} member {member_index} task_asset_id",
            )
            required_int(
                member["size"],
                f"bundle {locator} member {member_index} size",
                minimum=0,
            )
            require_sha256(
                member["sha256"],
                f"bundle {locator} member {member_index} content",
            )
            if not member["storage_ref_id"] or not member["mime_type"]:
                raise ValueError(
                    f"bundle receipt {locator} member {member_index} is incomplete"
                )
        bundle_by_revision[locator] = dict(entry)

    recovery_by_missing_id: dict[int, dict[str, Any]] = {}
    for index, entry in enumerate(recovery_entries):
        required = {
            "missing_task_asset_id",
            "target_root_asset_id",
            "target_task_id",
            "target_storage_ref_id",
            "target_object_key_sha256",
            "target_content_sha256",
            "target_size",
            "target_mime",
            "source_task_asset_id",
            "source_task_id",
            "source_storage_ref_id",
            "source_content_sha256",
            "source_size",
            "source_mime",
            "strategy",
            "source_receipt_sha256",
        }
        if set(entry) != required:
            raise ValueError(f"recovery receipt entry {index} field contract differs")
        asset_id = required_int(
            entry["missing_task_asset_id"], "recovery missing_task_asset_id"
        )
        if asset_id in recovery_by_missing_id:
            raise ValueError(f"duplicate recovery receipt {asset_id}")
        for field in (
            "target_object_key_sha256",
            "target_content_sha256",
            "source_content_sha256",
            "source_receipt_sha256",
        ):
            require_sha256(entry[field], f"recovery {asset_id} {field}")
        if not entry["target_storage_ref_id"] or not entry["target_mime"]:
            raise ValueError(f"recovery receipt {asset_id} target is incomplete")
        recovery_by_missing_id[asset_id] = dict(entry)

    recoveries = mapping.get("asset_recoveries", [])
    if not isinstance(recoveries, list):
        raise ValueError("reviewed mapping asset_recoveries is invalid")
    consumed_recoveries: set[int] = set()
    tombstones: set[int] = set()
    for index, recovery in enumerate(recoveries):
        if not isinstance(recovery, dict):
            raise ValueError(f"asset recovery {index} is invalid")
        missing_id = required_int(
            recovery.get("missing_task_asset_id"), "mapping missing_task_asset_id"
        )
        version = versions_by_id.get(missing_id)
        if version is None:
            raise ValueError(f"asset recovery {missing_id} is absent from frozen A")
        strategy = str(recovery.get("strategy", ""))
        if strategy == "historical_unavailable_tombstone_v1":
            if missing_id in recovery_by_missing_id:
                raise ValueError(f"tombstone {missing_id} unexpectedly has a receipt")
            old_locator = version["stable_locator"]
            root = roots[version["root_asset_id"]]
            if root["current_locator"] == old_locator:
                raise ValueError(f"tombstone {missing_id} remains the current pointer")
            tombstones.add(missing_id)
            version["content_availability"] = "historical_unavailable"
            version["provenance"] = {
                "kind": "approved_tombstone",
                "mapping_row_sha256": recovery.get("manifest_row_hash", ""),
            }
            continue
        receipt = recovery_by_missing_id.get(missing_id)
        if receipt is None:
            raise ValueError(f"asset recovery {missing_id} lacks approved receipt")
        source_id = required_int(
            receipt["source_task_asset_id"],
            f"asset recovery {missing_id} source_task_asset_id",
        )
        source = versions_by_id.get(source_id)
        if (
            receipt["strategy"] != strategy
            or int(receipt["target_root_asset_id"]) != version["root_asset_id"]
            or int(receipt["target_task_id"]) != version["task_id"]
            or int(recovery.get("task_id", -1)) != version["task_id"]
            or source is None
            or int(receipt["source_task_id"]) != source["task_id"]
            or source_id
            != int(recovery.get("recovery_source_task_asset_id", -1))
            or receipt["source_storage_ref_id"] != source["storage_ref_id"]
            or receipt["source_storage_ref_id"]
            != str(recovery.get("recovery_source_storage_ref_id", ""))
            or (
                source["content_sha256"]
                and receipt["source_content_sha256"]
                != source["content_sha256"]
            )
            or receipt["source_content_sha256"]
            != receipt["target_content_sha256"]
            or int(receipt["source_size"]) != source["size"]
            or int(receipt["source_size"]) != int(receipt["target_size"])
            or receipt["source_mime"] != source["mime_type"]
            or receipt["source_mime"] != receipt["target_mime"]
            or str(recovery.get("recovery_source_sha256", ""))
            != receipt["target_content_sha256"]
            or int(recovery.get("expected_file_size", -1))
            != int(receipt["target_size"])
        ):
            raise ValueError(f"asset recovery {missing_id} receipt differs")
        old_locator = version["stable_locator"]
        del versions_by_locator[old_locator]
        version.update(
            {
                "storage_ref_id": receipt["target_storage_ref_id"],
                "object_key_sha256": receipt["target_object_key_sha256"],
                "content_sha256": receipt["target_content_sha256"],
                "size": int(receipt["target_size"]),
                "mime_type": receipt["target_mime"],
                "deleted_at": "",
                "cleaned_at": "",
                "object_deleted_at": "",
                "content_availability": "available",
                "provenance": {
                    "kind": "recovery_receipt",
                    "mapping_row_sha256": recovery.get("manifest_row_hash", ""),
                    "source_receipt_sha256": receipt["source_receipt_sha256"],
                },
            }
        )
        version["stable_locator"] = (
            f"recovery:{missing_id}:{receipt['target_storage_ref_id']}:"
            f"{receipt['target_object_key_sha256']}:{receipt['target_content_sha256']}"
        )
        versions_by_locator[version["stable_locator"]] = version
        root = roots[version["root_asset_id"]]
        if root["current_locator"] == old_locator:
            root["current_locator"] = version["stable_locator"]
        consumed_recoveries.add(missing_id)
    if consumed_recoveries != set(recovery_by_missing_id):
        raise ValueError("one or more recovery receipts were not consumed")

    resources = mapping.get("resources")
    if not isinstance(resources, list):
        raise ValueError("reviewed mapping lacks resources")
    revision_roles: list[dict[str, Any]] = []
    revision_reasons: list[dict[str, str]] = []
    consumed_bundles: set[str] = set()
    seen_revisions: set[str] = set()
    active_final_ids: set[int] = set()
    for resource_index, resource in enumerate(resources):
        if not isinstance(resource, dict):
            raise ValueError(f"resource {resource_index} is invalid")
        task_id = required_int(resource.get("task_id"), "resource task_id")
        if task_id not in frozen_tasks:
            continue
        scope_kind = str(resource.get("scope_kind", ""))
        scope_ref_id = required_int(
            resource.get("scope_ref_id"), "resource scope_ref_id", minimum=0
        )
        scope_sku_code, retouch_requirement_id = scope_values(
            task_id,
            scope_kind,
            scope_ref_id,
            skus,
            retouch_requirements,
        )
        history = resource.get("history")
        if not isinstance(history, list):
            raise ValueError(f"resource {resource_index} lacks history")
        finalized_revision_no = nullable_int(resource.get("finalized_revision_no"))
        for revision_index, revision in enumerate(history):
            if not isinstance(revision, dict):
                raise ValueError(
                    f"resource {resource_index} revision {revision_index} is invalid"
                )
            revision_no = required_int(revision.get("revision_no"), "revision_no")
            locator = f"{task_id}:{scope_kind}:{scope_ref_id}:{revision_no}"
            if locator in seen_revisions:
                raise ValueError(f"duplicate reviewed revision {locator}")
            seen_revisions.add(locator)
            source_fields = [
                revision.get("source_task_asset_id") is not None,
                revision.get("source_alias_from_task_asset_id") is not None,
                revision.get("source_bundle") is not None,
            ]
            if sum(source_fields) > 1:
                raise ValueError(f"revision {locator} has multiple source selectors")
            source_locator: str | None = None
            source_kind = "none"
            if revision.get("source_task_asset_id") is not None:
                source_id = required_int(
                    revision["source_task_asset_id"], f"revision {locator} source"
                )
                source = versions_by_id.get(source_id)
                if source is None or source["task_id"] != task_id:
                    raise ValueError(f"revision {locator} source is absent from frozen A")
                source_locator = source["stable_locator"]
                source_kind = "a_source"
                source["expected_roles"].add("source")
            elif revision.get("source_alias_from_task_asset_id") is not None:
                source_id = required_int(
                    revision["source_alias_from_task_asset_id"],
                    f"revision {locator} source alias",
                )
                source = versions_by_id.get(source_id)
                if source is None or source["task_id"] != task_id:
                    raise ValueError(f"revision {locator} alias is absent from frozen A")
                source_locator = source["stable_locator"]
                source_kind = "delivery_source_alias"
                source["expected_roles"].add("source")
            elif revision.get("source_bundle") is not None:
                bundle = revision["source_bundle"]
                receipt = bundle_by_revision.get(locator)
                if not isinstance(bundle, dict) or receipt is None:
                    raise ValueError(f"revision {locator} bundle lacks approved receipt")
                expected_members = [
                    {
                        "task_asset_id": required_int(
                            member.get("task_asset_id"),
                            f"revision {locator} bundle member",
                        ),
                        "sha256": require_sha256(
                            member.get("sha256"),
                            f"revision {locator} bundle member hash",
                        ),
                    }
                    for member in bundle.get("members", [])
                    if isinstance(member, dict) and member.get("confirmed") is True
                ]
                receipt_members = receipt["members"]
                receipt_member_identity = [
                    {
                        "task_asset_id": required_int(
                            member["task_asset_id"],
                            f"revision {locator} receipt member",
                        ),
                        "sha256": require_sha256(
                            member["sha256"],
                            f"revision {locator} receipt member hash",
                        ),
                    }
                    for member in receipt_members
                ]
                if (
                    bundle.get("format") != "zip"
                    or bundle.get("bundle_sha256") != receipt["bundle_sha256"]
                    or bundle.get("manifest_sha256")
                    != receipt["internal_manifest_sha256"]
                    or bundle.get("task_asset_id")
                    != receipt["bundle_task_asset_id"]
                    or expected_members != receipt_member_identity
                ):
                    raise ValueError(f"revision {locator} bundle receipt differs")
                for member, receipt_member in zip(
                    expected_members, receipt_members, strict=True
                ):
                    a_member = versions_by_id.get(member["task_asset_id"])
                    if (
                        a_member is None
                        or a_member["task_id"] != task_id
                        or receipt_member["storage_ref_id"]
                        != a_member["storage_ref_id"]
                        or int(receipt_member["size"]) != a_member["size"]
                        or receipt_member["mime_type"] != a_member["mime_type"]
                        or (
                            a_member["content_sha256"]
                            and a_member["content_sha256"] != member["sha256"]
                        )
                    ):
                        raise ValueError(f"revision {locator} bundle member differs")
                new_id = required_int(
                    receipt["bundle_task_asset_id"], f"bundle {locator} task_asset_id"
                )
                new_root_id = required_int(
                    receipt["bundle_root_asset_id"], f"bundle {locator} root_asset_id"
                )
                if new_id in versions_by_id or new_root_id in roots:
                    raise ValueError(f"bundle {locator} allocation collides with frozen A")
                source_locator = (
                    f"bundle:{receipt['bundle_sha256']}:"
                    f"{receipt['internal_manifest_sha256']}"
                )
                bundle_version = {
                    "stable_locator": source_locator,
                    "task_asset_id": new_id,
                    "task_id": task_id,
                    "root_asset_id": new_root_id,
                    "intrinsic_asset_type": "source",
                    "scope_sku_code": scope_sku_code,
                    "retouch_requirement_id": retouch_requirement_id,
                    "storage_ref_id": receipt["bundle_storage_ref_id"],
                    "object_key_sha256": receipt["object_key_sha256"],
                    "content_sha256": receipt["bundle_sha256"],
                    "size": required_int(receipt["size"], f"bundle {locator} size"),
                    "mime_type": receipt["mime_type"],
                    "upload_status": "uploaded",
                    "deleted_at": "",
                    "cleaned_at": "",
                    "object_deleted_at": "",
                    "asset_version_no": 1,
                    "flow_review_status": "",
                    "approved_at": "",
                    "approved_by": None,
                    "created_at": "",
                    "source_asset_version_id": None,
                    "content_availability": "available",
                    "expected_roles": {"source"},
                    "provenance": {
                        "kind": "bundle_receipt",
                        "revision_locator": locator,
                    },
                }
                versions_by_id[new_id] = bundle_version
                versions_by_locator[source_locator] = bundle_version
                roots[new_root_id] = {
                    "root_asset_id": new_root_id,
                    "task_id": task_id,
                    "intrinsic_asset_type": "source",
                    "scope_sku_code": scope_sku_code,
                    "retouch_requirement_id": retouch_requirement_id,
                    "current_locator": source_locator,
                    "approved_locator": None,
                    "provenance": {
                        "kind": "bundle_receipt",
                        "revision_locator": locator,
                    },
                }
                source_kind = "bundle"
                consumed_bundles.add(locator)

            final_ids = revision.get("final_task_asset_ids")
            if not isinstance(final_ids, list):
                raise ValueError(f"revision {locator} finals must be an array")
            final_locators: list[str] = []
            for final_id_value in final_ids:
                final_id = required_int(final_id_value, f"revision {locator} final")
                final = versions_by_id.get(final_id)
                if final is None or final["task_id"] != task_id:
                    raise ValueError(f"revision {locator} final is absent from frozen A")
                final["expected_roles"].add("final")
                final_locators.append(final["stable_locator"])
                if revision.get("status") in {"finalized", "superseded"} and not (
                    final["flow_review_status"] == "approved"
                    and final["approved_at"]
                    and final["approved_by"] is not None
                ):
                    approved_at = (
                        revision.get("finalized_at")
                        or revision.get("submitted_at")
                        or revision.get("created_at")
                    )
                    if not isinstance(approved_at, str) or not approved_at:
                        raise ValueError(
                            f"revision {locator} lacks approval timestamp"
                        )
                    created_by = required_int(
                        revision.get("created_by"),
                        f"revision {locator} created_by",
                    )
                    final["flow_review_status"] = "approved"
                    if not final["approved_at"]:
                        final["approved_at"] = approved_at
                    if final["approved_by"] is None:
                        final["approved_by"] = created_by
                if finalized_revision_no == revision_no:
                    active_final_ids.add(final_id)

            expected_source = manifest_sources.get(locator)
            if expected_source is None:
                raise ValueError(f"revision {locator} lacks manifest G04 source")
            source = versions_by_locator.get(source_locator) if source_locator else None
            source_projection = {
                "stable_locator": manifest_locator(source) if source else "",
                "asset_type": "source" if source else "",
                "whole_hash": source["content_sha256"] if source else "",
                "binding_state": "bound" if source else "",
                "bound_role": "source" if source else "",
                "scope_sku_code": (
                    scope_sku_code if source else ""
                ),
                "retouch_requirement_id": (
                    str(retouch_requirement_id)
                    if source and retouch_requirement_id is not None
                    else ""
                ),
            }
            if source_projection != expected_source:
                raise ValueError(
                    f"revision {locator} source differs from reviewed manifest: "
                    f"expected={expected_source!r}, actual={source_projection!r}"
                )
            raw_reference_ids = revision.get("reference_file_ref_ids", [])
            if not isinstance(raw_reference_ids, list):
                raise ValueError(
                    f"revision {locator} reference_file_ref_ids must be an array"
                )
            reference_ids: list[int] = []
            reference_locators: list[str] = []
            for raw_reference_id in raw_reference_ids:
                reference_id = required_int(
                    raw_reference_id,
                    f"revision {locator} reference_file_ref_id",
                )
                reference = reference_file_refs.get(reference_id)
                if reference is None or reference["task_id"] != task_id:
                    raise ValueError(
                        f"revision {locator} reference {reference_id} "
                        "is absent from frozen A"
                    )
                if (
                    scope_kind == "sku"
                    and reference["sku_item_id"] is not None
                    and reference["sku_item_id"] != scope_ref_id
                ) or (
                    scope_kind == "retouch_requirement"
                    and reference["retouch_requirement_id"] is not None
                    and reference["retouch_requirement_id"] != scope_ref_id
                ):
                    raise ValueError(
                        f"revision {locator} reference {reference_id} "
                        "has a conflicting scope"
                    )
                if reference_id in reference_ids:
                    raise ValueError(
                        f"revision {locator} duplicates reference {reference_id}"
                    )
                reference_ids.append(reference_id)
                reference_locators.append(
                    f"reference:{reference_id}:{reference['ref_id']}"
                )
            revision_roles.append(
                {
                    "revision_locator": locator,
                    "task_id": task_id,
                    "scope_kind": scope_kind,
                    "scope_ref_id": scope_ref_id,
                    "revision_no": revision_no,
                    "status": revision.get("status"),
                    "source_stage": revision.get("source_stage"),
                    "source_kind": source_kind,
                    "source_locator": source_locator,
                    "final_locators": final_locators,
                    "reference_file_ref_ids": reference_ids,
                    "reference_locators": reference_locators,
                    "is_working": nullable_int(resource.get("working_revision_no"))
                    == revision_no,
                    "is_finalized": finalized_revision_no == revision_no,
                }
            )
            revision_reasons.append(
                {
                    "revision_locator": locator,
                    "reason_sha256": sha256(
                        str(revision.get("reason", "")).strip().encode("utf-8")
                    ),
                }
            )
    if seen_revisions != expected_revisions:
        raise ValueError("reviewed mapping revisions do not cover manifest G03")
    if consumed_bundles != set(bundle_by_revision):
        raise ValueError("one or more bundle receipts were not consumed")

    task_status = {task["task_id"]: task["task_status"] for task in tasks}
    by_root: dict[int, list[dict[str, Any]]] = {}
    for version in versions_by_id.values():
        by_root.setdefault(version["root_asset_id"], []).append(version)
    for root_id, root in roots.items():
        candidates = [
            version
            for version in by_root.get(root_id, [])
            if version["intrinsic_asset_type"] in ELIGIBLE_APPROVED_TYPES
            and version["provenance"]["kind"] != "approved_tombstone"
        ]
        approved = [
            version for version in candidates if version["flow_review_status"] == "approved"
        ]
        selected: dict[str, Any] | None = (
            max(approved, key=version_sort_key) if approved else None
        )
        if selected is None and task_status[root["task_id"]] == "Completed" and candidates:
            selected = max(candidates, key=version_sort_key)
        root["approved_locator"] = selected["stable_locator"] if selected else None
        if root["current_locator"]:
            current = versions_by_locator.get(root["current_locator"])
            if current is None or current["root_asset_id"] != root_id:
                raise ValueError(f"root {root_id} current locator is broken")
        if root["approved_locator"]:
            approved_version = versions_by_locator.get(root["approved_locator"])
            if approved_version is None or approved_version["root_asset_id"] != root_id:
                raise ValueError(f"root {root_id} approved locator is broken")

    for tombstone_id in tombstones:
        tombstone = versions_by_id[tombstone_id]
        root = roots[tombstone["root_asset_id"]]
        if (
            root["current_locator"] == tombstone["stable_locator"]
            or root["approved_locator"] == tombstone["stable_locator"]
            or tombstone_id in active_final_ids
        ):
            raise ValueError(f"tombstone {tombstone_id} remains an active pointer")

    detail_visible: list[str] = []
    for version in versions_by_id.values():
        normalized = canonical_asset_type(version["intrinsic_asset_type"])
        if (
            version["retouch_requirement_id"] is not None
            and normalized in {"reference", "source"}
        ):
            continue
        detail_visible.append(version["stable_locator"])

    serialized_versions: list[dict[str, Any]] = []
    for version in versions_by_locator.values():
        serialized = dict(version)
        serialized["expected_roles"] = sorted(version["expected_roles"])
        serialized_versions.append(serialized)
    serialized_versions.sort(key=lambda item: item["stable_locator"])

    unsigned: dict[str, Any] = {
        "schema_version": 2,
        "oracle_kind": "non_circular_g6_v2",
        "run_id": run_id,
        "inputs": {
            "reviewed_mapping_sha256": mapping_sha256,
            "reviewed_manifest_sha256": manifest_sha256,
            "snapshot_verdict_sha256": expected_snapshot_verdict_sha256,
            "clone_a_attestation_sha256": expected_clone_a_attestation_sha256,
            "a_snapshot_manifest_sha256": snapshot_package[
                "manifest_sha256"
            ],
            "a_snapshot_evidence_sha256": snapshot_package[
                "evidence_sha256"
            ],
            "source_snapshot_sha256": snapshot_verdict["snapshot_sha256"],
            "clone_a_database": clone_a_attestation["clone_database"],
            "bundle_receipts_sha256": sha256(
                canonical(
                    [sha256(path.read_bytes()) for path in bundle_receipt_paths]
                )
            ),
            "recovery_receipts_sha256": sha256(
                canonical(
                    [sha256(path.read_bytes()) for path in recovery_receipt_paths]
                )
            ),
        },
        "tasks": tasks,
        "roots": sorted(roots.values(), key=lambda item: item["root_asset_id"]),
        "versions": serialized_versions,
        "revision_roles": sorted(
            revision_roles, key=lambda item: item["revision_locator"]
        ),
        "revision_reasons": sorted(
            revision_reasons, key=lambda item: item["revision_locator"]
        ),
        "route_expectations": {
            "detail_visible_locators": sorted(detail_visible),
            "list_root_ids": sorted(
                root_id
                for root_id, root in roots.items()
                if root["current_locator"] is not None
            ),
            "current_locators": sorted(
                root["current_locator"]
                for root in roots.values()
                if root["current_locator"] is not None
            ),
            "approved_locators": sorted(
                root["approved_locator"]
                for root in roots.values()
                if root["approved_locator"] is not None
            ),
            "historical_unavailable_locators": sorted(
                version["stable_locator"]
                for version in versions_by_id.values()
                if version["content_availability"] == "historical_unavailable"
            ),
        },
    }
    return {**unsigned, "evidence_sha256": sha256(canonical(unsigned))}


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--manifest", type=pathlib.Path, required=True)
    parser.add_argument("--reviewed-mapping", type=pathlib.Path, required=True)
    parser.add_argument("--expected-mapping-sha256", required=True)
    parser.add_argument("--snapshot-verdict", type=pathlib.Path, required=True)
    parser.add_argument("--expected-snapshot-verdict-sha256", required=True)
    parser.add_argument(
        "--clone-a-attestation", type=pathlib.Path, required=True
    )
    parser.add_argument("--expected-clone-a-attestation-sha256", required=True)
    parser.add_argument(
        "--a-snapshot-manifest", type=pathlib.Path, required=True
    )
    parser.add_argument(
        "--bundle-receipt", type=pathlib.Path, action="append", default=[]
    )
    parser.add_argument(
        "--recovery-receipt", type=pathlib.Path, action="append", default=[]
    )
    parser.add_argument("--output", type=pathlib.Path, required=True)
    args = parser.parse_args(argv)
    try:
        result = build(
            run_id=args.run_id,
            manifest_path=args.manifest,
            reviewed_mapping_path=args.reviewed_mapping,
            expected_mapping_sha256=args.expected_mapping_sha256,
            snapshot_verdict_path=args.snapshot_verdict,
            expected_snapshot_verdict_sha256=(
                args.expected_snapshot_verdict_sha256
            ),
            clone_a_attestation_path=args.clone_a_attestation,
            expected_clone_a_attestation_sha256=(
                args.expected_clone_a_attestation_sha256
            ),
            a_snapshot_manifest_path=args.a_snapshot_manifest,
            bundle_receipt_paths=args.bundle_receipt,
            recovery_receipt_paths=args.recovery_receipt,
        )
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_bytes(canonical(result) + b"\n")
        return 0
    except (OSError, ValueError, KeyError, json.JSONDecodeError) as exc:
        print(str(exc))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
