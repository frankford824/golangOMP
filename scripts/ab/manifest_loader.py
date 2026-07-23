#!/usr/bin/env python3
"""Build, validate, and load the reviewed A/B entity manifest.

Database entity hashes are SHA-256 over UTF-8 string components joined by
ASCII unit separator (0x1f), matching SQL ``SHA2(CONCAT_WS(CHAR(31), ...),256)``.
The builder binds those components to the reviewed mapping, frozen-A
attestation, approved decisions, and object-verifier result by file hash.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import re
import sys
from typing import Any


RID = re.compile(r"^[a-z0-9][a-z0-9_-]{2,63}$")
SHA256 = re.compile(r"^[0-9a-f]{64}$")
FIELDS = {"run_id", "gate_name", "entity_key", "expected_hash", "expected_state", "review_state", "detail_json"}
DATABASE_GATES = {"G01", "G02", "G03", "G04", "G05", "G07", "G08", "G09"}
SUPPORTED_GATES = DATABASE_GATES | {"G06", "G10"}
GATE_DERIVATIONS = {
    "G01": "reviewed_mapping_a_truth",
    "G02": "reviewed_mapping_a_truth",
    "G03": "reviewed_mapping_a_truth",
    "G04": "reviewed_mapping_a_truth",
    "G05": "reviewed_mapping_a_truth",
    "G07": "immutable_a_truth",
    "G08": "reviewed_mapping_a_truth",
    "G09": "independent_projection",
}


def canonical_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def sha256_file(path: pathlib.Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def hash_components(components: list[str]) -> str:
    if not isinstance(components, list) or not components or not all(isinstance(item, str) for item in components):
        raise ValueError("components must be a non-empty array of strings")
    return hashlib.sha256("\x1f".join(components).encode("utf-8")).hexdigest()


def hex_text(value: str) -> str:
    return "CONVERT(0x" + value.encode("utf-8").hex() + " USING utf8mb4)"


def validate_pass_row(row: dict[str, Any], line_no: int) -> None:
    detail = row["detail_json"]
    if not isinstance(detail, dict):
        raise ValueError(f"invalid manifest row {line_no}: detail_json must be an object")
    method = detail.get("derivation_method")
    inputs = detail.get("input_sha256")
    if not isinstance(inputs, dict) or not inputs or not all(
        isinstance(key, str) and isinstance(value, str) and SHA256.fullmatch(value)
        for key, value in inputs.items()
    ):
        raise ValueError(f"invalid manifest row {line_no}: verified input_sha256 provenance is required")
    gate = row["gate_name"]
    if gate in DATABASE_GATES:
        if method != GATE_DERIVATIONS[gate]:
            raise ValueError(f"invalid manifest row {line_no}: {gate} requires derivation_method={GATE_DERIVATIONS[gate]}")
        expected = hash_components(detail.get("components"))
        if expected != row["expected_hash"]:
            raise ValueError(f"invalid manifest row {line_no}: component hash mismatch")
    elif gate == "G06":
        if method != "object_verifier" or detail.get("verdict") != "PASS" or row["expected_state"] != "verified":
            raise ValueError(f"invalid manifest row {line_no}: G06 pass requires a PASS object_verifier verdict")
        expected = hashlib.sha256(canonical_json(detail).encode("utf-8")).hexdigest()
        if expected != row["expected_hash"]:
            raise ValueError(f"invalid manifest row {line_no}: G06 verifier hash mismatch")
    elif gate == "G10":
        if method != "human_decision" or row["expected_state"] != "confirmed" or detail.get("decision") != "confirmed":
            raise ValueError(f"invalid manifest row {line_no}: G10 pass must be a confirmed human decision")
        expected = hashlib.sha256(canonical_json(detail).encode("utf-8")).hexdigest()
        if expected != row["expected_hash"]:
            raise ValueError(f"invalid manifest row {line_no}: G10 decision hash mismatch")


def load_rows(path: pathlib.Path, expected_sha: str, run_id: str) -> list[dict[str, Any]]:
    raw = path.read_bytes()
    got = hashlib.sha256(raw).hexdigest()
    if not SHA256.fullmatch(expected_sha) or got != expected_sha:
        raise ValueError("manifest sha256 mismatch")
    if not RID.fullmatch(run_id):
        raise ValueError("invalid run_id")
    rows: list[dict[str, Any]] = []
    seen: set[tuple[str, str]] = set()
    gates: set[str] = set()
    for line_no, line in enumerate(raw.decode("utf-8").splitlines(), 1):
        if not line:
            continue
        row = json.loads(line)
        if set(row) != FIELDS or row.get("run_id") != run_id:
            raise ValueError(f"invalid manifest row {line_no}")
        if not all(isinstance(row[key], str) for key in FIELDS - {"detail_json"}):
            raise ValueError(f"invalid manifest row {line_no}: scalar fields must be strings")
        gate = row["gate_name"]
        if gate not in SUPPORTED_GATES:
            raise ValueError(f"invalid manifest row {line_no}: unsupported gate {gate!r}")
        if not row["entity_key"]:
            raise ValueError(f"invalid manifest row {line_no}: entity_key is required")
        key = (gate, row["entity_key"])
        if key in seen:
            raise ValueError(f"invalid manifest row {line_no}: duplicate gate/entity {key}")
        seen.add(key)
        gates.add(gate)
        if row["review_state"] not in {"pass", "proposed_review", "hard_blocked"}:
            raise ValueError(f"invalid manifest row {line_no}: review_state")
        if not SHA256.fullmatch(row["expected_hash"]):
            raise ValueError(f"invalid manifest row {line_no}: expected_hash must be lowercase sha256")
        if row["review_state"] == "pass":
            validate_pass_row(row, line_no)
        row["detail_json"] = canonical_json(row["detail_json"])
        rows.append(row)
    if not rows:
        raise ValueError("manifest must contain at least one reviewed row")
    missing = SUPPORTED_GATES - gates
    if missing:
        raise ValueError(f"manifest missing gates: {sorted(missing)}")
    return rows


def emit_sql(rows: list[dict[str, Any]], output: pathlib.Path) -> None:
    sql = [
        "CREATE TEMPORARY TABLE ab_manifest_entities (",
        "  run_id VARCHAR(64) NOT NULL,",
        "  gate_name VARCHAR(64) NOT NULL,",
        "  entity_key VARCHAR(255) NOT NULL,",
        "  expected_hash CHAR(64) NOT NULL,",
        "  expected_state VARCHAR(64) NOT NULL,",
        "  review_state VARCHAR(32) NOT NULL,",
        "  detail_json JSON NOT NULL,",
        "  PRIMARY KEY (gate_name, entity_key)",
        ");",
    ]
    for row in rows:
        values = [hex_text(row[key]) for key in ("run_id", "gate_name", "entity_key", "expected_hash", "expected_state", "review_state")]
        values.append("CAST(" + hex_text(row["detail_json"]) + " AS JSON)")
        sql.append("INSERT INTO ab_manifest_entities VALUES (" + ",".join(values) + ");")
    output.write_text("\n".join(sql) + "\n", encoding="utf-8")


def validate_reviewed_mapping(mapping: Any) -> None:
    if not isinstance(mapping, dict) or mapping.get("version") != 2:
        raise ValueError("reviewed mapping must be version 2")
    for index, resource in enumerate(mapping.get("resources", [])):
        for revision_index, revision in enumerate(resource.get("history", [])):
            if revision.get("confidence") != "confirmed_auto":
                raise ValueError(f"resources[{index}].history[{revision_index}] is not confirmed_auto")
            if not SHA256.fullmatch(str(revision.get("manifest_row_hash", ""))):
                raise ValueError(f"resources[{index}].history[{revision_index}] has no valid manifest_row_hash")
            confirmed_at = revision.get("confirmed_at")
            if (not revision.get("confirmed_by") or not isinstance(confirmed_at, str)
                    or confirmed_at.startswith("0001-01-01T00:00:00") or not revision.get("confirmation_note")):
                raise ValueError(f"resources[{index}].history[{revision_index}] has incomplete confirmation metadata")
    for index, planning in enumerate(mapping.get("planning_tasks", [])):
        if planning.get("confidence") != "confirmed_auto":
            raise ValueError(f"planning_tasks[{index}] is not confirmed_auto")
        confirmed_at = planning.get("confirmed_at")
        if (not planning.get("confirmed_by") or not isinstance(confirmed_at, str)
                or confirmed_at.startswith("0001-01-01T00:00:00") or not planning.get("confirmation_note")):
            raise ValueError(f"planning_tasks[{index}] has incomplete confirmation metadata")
    for index, decision in enumerate(mapping.get("task_state_decisions", [])):
        confirmed_at = decision.get("confirmed_at")
        if (not decision.get("confirmed_by") or not isinstance(confirmed_at, str)
                or confirmed_at.startswith("0001-01-01T00:00:00") or not decision.get("confirmation_note")):
            raise ValueError(f"task_state_decisions[{index}] has incomplete confirmation metadata")
        if not SHA256.fullmatch(str(decision.get("manifest_row_hash", ""))):
            raise ValueError(f"task_state_decisions[{index}] has no valid manifest_row_hash")


def build_manifest(
    run_id: str,
    entity_input: pathlib.Path,
    mapping_path: pathlib.Path,
    baseline_path: pathlib.Path,
    decisions_path: pathlib.Path,
    object_verdict_path: pathlib.Path,
    output: pathlib.Path,
    projection_expected_path: pathlib.Path | None = None,
) -> None:
    if not RID.fullmatch(run_id):
        raise ValueError("invalid run_id")
    mapping = json.loads(mapping_path.read_text(encoding="utf-8"))
    baseline = json.loads(baseline_path.read_text(encoding="utf-8"))
    decisions = json.loads(decisions_path.read_text(encoding="utf-8"))
    object_verdict = json.loads(object_verdict_path.read_text(encoding="utf-8"))
    validate_reviewed_mapping(mapping)
    if not isinstance(baseline, dict) or not baseline.get("snapshot_sha256") or not baseline.get("baseline_fingerprint_sha256"):
        raise ValueError("baseline attestation is incomplete")
    if not isinstance(decisions, dict) or decisions.get("decision") != "confirmed":
        raise ValueError("approved decisions must contain decision=confirmed")
    actual_inputs = {
        "mapping_sha256": sha256_file(mapping_path),
        "baseline_attestation_sha256": sha256_file(baseline_path),
        "approved_decisions_sha256": sha256_file(decisions_path),
        "object_verdict_sha256": sha256_file(object_verdict_path),
    }
    if projection_expected_path is not None:
        actual_inputs["projection_expected_sha256"] = sha256_file(projection_expected_path)
    source = json.loads(entity_input.read_text(encoding="utf-8"))
    if not isinstance(source, dict) or source.get("schema_version") != 1 or source.get("input_sha256") != actual_inputs:
        raise ValueError("canonical entity input is not bound to the supplied source artifacts")
    entities = source.get("entities")
    if not isinstance(entities, list) or not entities:
        raise ValueError("canonical entity input has no entities")
    if projection_expected_path is not None:
        projection_entities = []
        for line_no, line in enumerate(projection_expected_path.read_text(encoding="utf-8").splitlines(), 1):
            if not line:
                continue
            entity = json.loads(line)
            if not isinstance(entity, dict) or entity.get("gate_name") != "G09" or entity.get("derivation_method") != "independent_projection":
                raise ValueError(f"invalid projection expected row {line_no}")
            projection_entities.append(entity)
        if not projection_entities:
            raise ValueError("projection expected JSONL has no G09 entities")
        entities = [entity for entity in entities if isinstance(entity, dict) and entity.get("gate_name") != "G09"]
        entities.extend(projection_entities)
    rows = []
    for index, entity in enumerate(entities):
        if not isinstance(entity, dict):
            raise ValueError(f"entities[{index}] must be an object")
        gate = entity.get("gate_name")
        review_state = entity.get("review_state", "hard_blocked")
        method = entity.get("derivation_method", "unproven")
        detail = {
            "derivation_method": method,
            "input_sha256": actual_inputs,
            "components": entity.get("components", []),
            "detail": entity.get("detail", {}),
        }
        if gate == "G06":
            detail["verdict"] = object_verdict.get("status")
            if object_verdict.get("status") != "PASS" or object_verdict.get("violation_count") != 0:
                review_state = "hard_blocked"
        if gate == "G10":
            detail["decision"] = decisions.get("decision")
        if gate in DATABASE_GATES and entity.get("components"):
            expected_hash = hash_components(entity["components"])
        else:
            expected_hash = hashlib.sha256(canonical_json(detail).encode("utf-8")).hexdigest()
        rows.append({
            "run_id": run_id,
            "gate_name": gate,
            "entity_key": entity.get("entity_key", ""),
            "expected_hash": expected_hash,
            "expected_state": entity.get("expected_state", "approved"),
            "review_state": review_state,
            "detail_json": detail,
        })
    encoded = "".join(canonical_json(row) + "\n" for row in rows)
    output.write_text(encoded, encoding="utf-8")
    load_rows(output, sha256_file(output), run_id)


def parse_args(argv: list[str]) -> argparse.Namespace:
    if len(argv) == 4 and argv[0] not in {"build", "load"}:
        return argparse.Namespace(command="load", file=argv[0], sha256=argv[1], run_id=argv[2], output=argv[3])
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)
    load = sub.add_parser("load")
    load.add_argument("--file", required=True)
    load.add_argument("--sha256", required=True)
    load.add_argument("--run-id", required=True)
    load.add_argument("--output", required=True)
    build = sub.add_parser("build")
    build.add_argument("--run-id", required=True)
    build.add_argument("--entity-input", required=True)
    build.add_argument("--mapping", required=True)
    build.add_argument("--baseline-attestation", required=True)
    build.add_argument("--approved-decisions", required=True)
    build.add_argument("--object-verdict", required=True)
    build.add_argument("--projection-expected")
    build.add_argument("--output", required=True)
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    try:
        args = parse_args(argv)
        if args.command == "build":
            build_manifest(args.run_id, pathlib.Path(args.entity_input), pathlib.Path(args.mapping), pathlib.Path(args.baseline_attestation), pathlib.Path(args.approved_decisions), pathlib.Path(args.object_verdict), pathlib.Path(args.output), pathlib.Path(args.projection_expected) if args.projection_expected else None)
        else:
            rows = load_rows(pathlib.Path(args.file), args.sha256, args.run_id)
            emit_sql(rows, pathlib.Path(args.output))
    except (OSError, UnicodeDecodeError, ValueError, json.JSONDecodeError) as exc:
        print(str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
