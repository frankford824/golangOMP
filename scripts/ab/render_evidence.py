#!/usr/bin/env python3
"""Normalize deterministic SQL evidence and assemble the final gate report."""

from __future__ import annotations

import csv
import hashlib
import json
import pathlib
import re
import sys
from typing import Any


VIOLATION_FIELDS = ["violation_code", "entity_key", "detail"]
FINGERPRINT_FIELDS = ["metric", "value"]
GATE_RE = re.compile(r"^(\d{2}_[a-z0-9_]+)\.json$")
SQL_GATE_FIELDS = {
    "00_snapshot_fingerprint": FINGERPRINT_FIELDS,
    "01_task_state_parity": VIOLATION_FIELDS,
    "02_group_coverage": VIOLATION_FIELDS,
    "03_revision_chain": VIOLATION_FIELDS,
    "04_asset_role_scope": VIOLATION_FIELDS,
    "05_reference_integrity": VIOLATION_FIELDS,
    "06_storage_integrity": VIOLATION_FIELDS,
    "07_event_history_checksum": VIOLATION_FIELDS,
    "08_planning_retouch": VIOLATION_FIELDS,
    "09_search_publish_outbox": VIOLATION_FIELDS,
    "10_negative_assertions": VIOLATION_FIELDS,
    "11_manifest_state": VIOLATION_FIELDS,
    "12_legacy_timestamp_contract": VIOLATION_FIELDS,
}


def digest(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def canonical_bytes(value: Any) -> bytes:
    return (json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")) + "\n").encode()


def write_json(path: pathlib.Path, payload: dict[str, Any]) -> str:
    encoded = canonical_bytes(payload)
    path.write_bytes(encoded)
    return digest(encoded)


def render(
    tsv: pathlib.Path,
    csv_path: pathlib.Path,
    json_path: pathlib.Path,
    side: str = "",
    gate: str = "",
) -> None:
    with tsv.open("r", encoding="utf-8", newline="") as fh:
        reader = csv.DictReader(fh, delimiter="\t")
        rows = list(reader)
        fields = reader.fieldnames or []
    if not fields:
        raise SystemExit(f"{tsv}: empty MySQL output has no header and cannot pass")
    is_fingerprint = fields == FINGERPRINT_FIELDS
    if not is_fingerprint and fields != VIOLATION_FIELDS:
        raise SystemExit(f"{tsv}: unexpected columns {fields!r}")
    rows.sort(key=lambda row: tuple(row.get(key, "") for key in fields))
    with csv_path.open("w", encoding="utf-8", newline="") as fh:
        writer = csv.DictWriter(fh, fieldnames=fields, lineterminator="\n")
        writer.writeheader()
        writer.writerows(rows)
    evidence = rows if is_fingerprint else [row for row in rows if row.get("violation_code", "").startswith("evidence.")]
    violations = [] if is_fingerprint else [row for row in rows if not row.get("violation_code", "").startswith("evidence.")]
    payload: dict[str, Any] = {
        "schema_version": 1,
        "status": "PASS" if not violations else "FAIL",
        "violation_count": len(violations),
        "violations": violations,
        "evidence": evidence,
    }
    if side:
        payload["side"] = side
    if gate:
        payload["gate"] = gate
    encoded_hash = write_json(json_path, payload)
    print(f"sha256={encoded_hash} rows={len(rows)} violations={len(violations)}")


def mark_blocked(path: pathlib.Path, code: str, detail: str) -> None:
    payload = json.loads(path.read_text(encoding="utf-8"))
    blocker = {"violation_code": code, "entity_key": "*", "detail": detail}
    payload.setdefault("violations", []).append(blocker)
    payload["violations"].sort(key=lambda row: (row["violation_code"], row["entity_key"], row["detail"]))
    payload["violation_count"] = len(payload["violations"])
    payload["status"] = "FAIL"
    write_json(path, payload)


def compare(left: pathlib.Path, right: pathlib.Path, out: pathlib.Path) -> None:
    left_payload = json.loads(left.read_text(encoding="utf-8"))
    right_payload = json.loads(right.read_text(encoding="utf-8"))
    left_evidence = left_payload.get("evidence", [])
    right_evidence = right_payload.get("evidence", [])
    left_hash = digest(canonical_bytes(left_evidence))
    right_hash = digest(canonical_bytes(right_evidence))
    payload: dict[str, Any] = {
        "schema_version": 1,
        "gate": left.stem,
        "status": "PASS",
        "violation_count": 0,
        "violations": [],
        "source_evidence_sha256": left_hash,
        "target_evidence_sha256": right_hash,
    }
    if left_evidence != right_evidence:
        payload["status"] = "FAIL"
        payload["violation_count"] = 1
        payload["violations"] = [{
            "violation_code": "ab_parity.immutable_evidence_mismatch",
            "entity_key": left.stem,
            "detail": f"source_sha256={left_hash},target_sha256={right_hash}",
        }]
    encoded_hash = write_json(out, payload)
    print(f"sha256={encoded_hash} rows={payload['violation_count']}")


def split_markers(combined: pathlib.Path, output_dir: pathlib.Path) -> None:
    """Split one MySQL session while preserving each query's header row."""
    output_dir.mkdir(parents=True, exist_ok=True)
    sections: dict[str, list[str]] = {}
    current: str | None = None
    skip_marker_header = False
    for raw_line in combined.read_text(encoding="utf-8").splitlines():
        if raw_line == "ab_gate_marker":
            skip_marker_header = True
            continue
        if raw_line.startswith("__AB_GATE__"):
            current = raw_line.removeprefix("__AB_GATE__")
            if current not in SQL_GATE_FIELDS or current in sections:
                raise SystemExit(f"invalid or duplicate gate marker: {raw_line}")
            sections[current] = []
            skip_marker_header = False
            continue
        if skip_marker_header:
            raise SystemExit("gate marker header was not followed by a marker value")
        if current is not None:
            sections[current].append(raw_line)
    if not sections:
        raise SystemExit("combined MySQL output contained no gate markers")
    for name, lines in sections.items():
        if not lines:
            # mysql emits no header for a valid zero-row SELECT. Synthesize
            # only the fixed schema of an explicitly known audit gate.
            lines = ["\t".join(SQL_GATE_FIELDS[name])]
        (output_dir / f"{name}.tsv").write_text("\n".join(lines) + "\n", encoding="utf-8")


def load_gate_files(directory: pathlib.Path) -> dict[str, pathlib.Path]:
    files: dict[str, pathlib.Path] = {}
    for path in directory.glob("*.json"):
        match = GATE_RE.fullmatch(path.name)
        if match:
            files[match.group(1)] = path
    return files


def gate_report(
    run_id: str,
    source_dir: pathlib.Path,
    target_dir: pathlib.Path,
    parity_dir: pathlib.Path,
    out: pathlib.Path,
) -> dict[str, Any]:
    source_files = load_gate_files(source_dir)
    target_files = load_gate_files(target_dir)
    expected = {f"{index:02d}_" for index in range(13)}
    actual_names = set(source_files) | set(target_files)
    missing_prefixes = sorted(prefix for prefix in expected if not any(name.startswith(prefix) for name in actual_names))
    violations: list[dict[str, str]] = []
    if set(source_files) != set(target_files) or missing_prefixes or len(source_files) != 13:
        violations.append({
            "violation_code": "runner.gate_file_set_incomplete",
            "entity_key": "00-12",
            "detail": f"source={sorted(source_files)},target={sorted(target_files)},missing_prefixes={missing_prefixes}",
        })
    gates: list[dict[str, Any]] = []
    for name in sorted(set(source_files) & set(target_files)):
        source_payload = json.loads(source_files[name].read_text(encoding="utf-8"))
        target_payload = json.loads(target_files[name].read_text(encoding="utf-8"))
        for side, payload in (("A", source_payload), ("B", target_payload)):
            for row in payload.get("violations", []):
                violations.append({
                    "violation_code": row.get("violation_code", "runner.invalid_violation"),
                    "entity_key": f"{side}:{row.get('entity_key', '*')}",
                    "detail": row.get("detail", ""),
                })
        gates.append({
            "gate": name,
            "a_assessment": "baseline_or_immutable_parity",
            "b_assessment": "approved_manifest_and_v8_invariants",
            "a_violation_count": source_payload.get("violation_count", 0),
            "b_violation_count": target_payload.get("violation_count", 0),
            "a_json_sha256": digest(source_files[name].read_bytes()),
            "b_json_sha256": digest(target_files[name].read_bytes()),
        })
    parity_file = parity_dir / "07_event_history_checksum.json"
    parity_payload: dict[str, Any] | None = None
    if not parity_file.is_file():
        violations.append({
            "violation_code": "runner.immutable_parity_missing",
            "entity_key": "07_event_history_checksum",
            "detail": "A/B immutable event parity evidence was not produced",
        })
    else:
        parity_payload = json.loads(parity_file.read_text(encoding="utf-8"))
        violations.extend(parity_payload.get("violations", []))
    violations.sort(key=lambda row: (row["violation_code"], row["entity_key"], row["detail"]))
    payload = {
        "schema_version": 1,
        "run_id": run_id,
        "status": "PASS" if not violations else "FAIL",
        "violation_count": len(violations),
        "violations": violations,
        "gates": gates,
        "immutable_event_parity": parity_payload,
    }
    write_json(out, payload)
    return payload


def execution_failure(run_id: str, side: str, out: pathlib.Path) -> None:
    payload = {
        "schema_version": 1,
        "run_id": run_id,
        "status": "FAIL",
        "violation_count": 1,
        "violations": [{
            "violation_code": "runner.mysql_session_failed",
            "entity_key": side,
            "detail": f"MySQL {side} read-only session failed; inspect the run-scoped stderr evidence",
        }],
        "gates": [],
        "immutable_event_parity": None,
    }
    write_json(out, payload)


def main(argv: list[str]) -> int:
    if len(argv) == 4 and argv[1] == "split-markers":
        split_markers(pathlib.Path(argv[2]), pathlib.Path(argv[3]))
        return 0
    if len(argv) == 5 and argv[1] == "mark-blocked":
        mark_blocked(pathlib.Path(argv[2]), argv[3], argv[4])
        return 0
    if len(argv) in {5, 7} and argv[1] == "render":
        side = argv[5] if len(argv) == 7 else ""
        gate = argv[6] if len(argv) == 7 else ""
        render(pathlib.Path(argv[2]), pathlib.Path(argv[3]), pathlib.Path(argv[4]), side, gate)
        return 0
    if len(argv) == 5 and argv[1] == "compare":
        compare(pathlib.Path(argv[2]), pathlib.Path(argv[3]), pathlib.Path(argv[4]))
        return 0
    if len(argv) == 7 and argv[1] == "gate-report":
        payload = gate_report(argv[2], pathlib.Path(argv[3]), pathlib.Path(argv[4]), pathlib.Path(argv[5]), pathlib.Path(argv[6]))
        return 0 if payload["violation_count"] == 0 else 1
    if len(argv) == 5 and argv[1] == "execution-failure":
        execution_failure(argv[2], argv[3], pathlib.Path(argv[4]))
        return 1
    raise SystemExit(
        "usage: render_evidence.py render TSV CSV JSON [SIDE GATE] | compare SOURCE_JSON TARGET_JSON OUT_JSON | "
        "split-markers COMBINED_TSV OUT_DIR | mark-blocked JSON CODE DETAIL | "
        "gate-report RUN_ID A_DIR B_DIR PARITY_DIR OUT_JSON | execution-failure RUN_ID SIDE OUT_JSON"
    )


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
