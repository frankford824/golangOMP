#!/usr/bin/env python3
"""Bind a reviewed manifest to canonical observations from clone B."""
from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import re

OBS_RE = re.compile(r"^evidence\.manifest_state\.(G(?:01|02|03|04|05|07|08|09))$")
REQUIRED_GATES = {"G01", "G02", "G03", "G04", "G05", "G07", "G08", "G09"}


def load_manifest(path: pathlib.Path, run_id: str) -> dict[tuple[str, str], str]:
    expected: dict[tuple[str, str], str] = {}
    gates: set[str] = set()
    for line_no, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if not line:
            continue
        row = json.loads(line)
        if row.get("run_id") != run_id:
            raise ValueError(f"manifest row {line_no}: wrong run_id")
        review = row.get("review_state")
        if review != "pass":
            raise ValueError(f"manifest row {line_no}: {review} is a hard blocker")
        gate = row.get("gate_name")
        if gate == "G10":
            if row.get("expected_state") != "confirmed":
                raise ValueError(f"manifest row {line_no}: G10 decision is not confirmed")
            canonical = json.dumps(row.get("detail_json"), ensure_ascii=False, sort_keys=True, separators=(",", ":"))
            if hashlib.sha256(canonical.encode()).hexdigest() != row.get("expected_hash"):
                raise ValueError(f"manifest row {line_no}: G10 decision hash mismatch")
            continue
        if gate not in REQUIRED_GATES:
            # G06 belongs to the object verifier. Other names are never silently accepted.
            if gate == "G06":
                continue
            raise ValueError(f"manifest row {line_no}: unsupported gate {gate!r}")
        gates.add(gate)
        key = (gate, row.get("entity_key", ""))
        if not key[1] or key in expected:
            raise ValueError(f"manifest row {line_no}: empty or duplicate entity key")
        expected[key] = row.get("expected_hash", "")
    missing = REQUIRED_GATES - gates
    if missing:
        raise ValueError(f"manifest missing database gates: {sorted(missing)}")
    return expected


def load_observed(path: pathlib.Path) -> dict[tuple[str, str], str]:
    payload = json.loads(path.read_text(encoding="utf-8"))
    if payload.get("violation_count"):
        raise ValueError("manifest-state SQL contains structural violations")
    observed: dict[tuple[str, str], str] = {}
    for row in payload.get("evidence", []):
        match = OBS_RE.fullmatch(row.get("violation_code", ""))
        if not match:
            continue
        key = (match.group(1), row.get("entity_key", ""))
        if not key[1] or key in observed:
            raise ValueError(f"duplicate or empty observed entity: {key}")
        observed[key] = row.get("detail", "")
    if not observed:
        raise ValueError("manifest-state SQL emitted no observations")
    return observed


def verify(manifest: pathlib.Path, observations: pathlib.Path, run_id: str) -> dict:
    expected = load_manifest(manifest, run_id)
    observed = load_observed(observations)
    violations = []
    for key in sorted(expected.keys() - observed.keys()):
        violations.append({"violation_code": "manifest.expected_entity_missing", "entity_key": f"{key[0]}:{key[1]}", "detail": "missing from clone B"})
    for key in sorted(observed.keys() - expected.keys()):
        violations.append({"violation_code": "manifest.unreviewed_entity", "entity_key": f"{key[0]}:{key[1]}", "detail": "present in clone B but absent from reviewed manifest"})
    for key in sorted(expected.keys() & observed.keys()):
        if expected[key] != observed[key]:
            violations.append({"violation_code": "manifest.entity_hash_mismatch", "entity_key": f"{key[0]}:{key[1]}", "detail": f"expected={expected[key]},actual={observed[key]}"})
    return {"violation_count": len(violations), "violations": violations,
            "expected_entities": len(expected), "observed_entities": len(observed)}


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", required=True, type=pathlib.Path)
    parser.add_argument("--observations", required=True, type=pathlib.Path)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--output", required=True, type=pathlib.Path)
    args = parser.parse_args()
    try:
        result = verify(args.manifest, args.observations, args.run_id)
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        result = {"violation_count": 1, "violations": [{"violation_code": "manifest.verification_error", "entity_key": "*", "detail": str(exc)}]}
    args.output.write_text(json.dumps(result, ensure_ascii=False, sort_keys=True, separators=(",", ":")) + "\n", encoding="utf-8")
    raise SystemExit(0 if result["violation_count"] == 0 else 1)


if __name__ == "__main__":
    main()
