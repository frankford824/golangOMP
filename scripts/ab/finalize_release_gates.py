#!/usr/bin/env python3
"""Fail-closed G0-G10 finalizer for one formal V8 A/B run.

The finalizer never executes a migration, database query, HTTP request, or UI
action.  It verifies hashes and the small, stable result envelopes produced by
those independent executors.  A release can be ``GO`` only when every required
artifact is present, hash-bound, reports ``PASS``, and the three required
review roles sign the exact evidence index.
"""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import pathlib
import re
from typing import Any


SHA256 = re.compile(r"^[0-9a-f]{64}$")
RUN_ID = re.compile(r"^[a-z0-9][a-z0-9_-]{2,63}$")
GATES = tuple(f"G{index}" for index in range(11))
REQUIRED_ROLES = {
    "independent_sql_verifier",
    "adversarial_reviewer",
    "release_commander",
}
EMPTY_SHA256 = hashlib.sha256(b"").hexdigest()


def canonical_bytes(value: Any) -> bytes:
    return (
        json.dumps(
            value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
        )
        + "\n"
    ).encode("utf-8")


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def read_json(path: pathlib.Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{path}: expected a JSON object")
    return value


def safe_artifact_path(run_dir: pathlib.Path, value: str) -> pathlib.Path:
    candidate = pathlib.PurePosixPath(value)
    if (
        candidate.is_absolute()
        or not candidate.parts
        or any(part in {"", ".", ".."} for part in candidate.parts)
    ):
        raise ValueError(f"unsafe artifact path: {value!r}")
    path = run_dir.joinpath(*candidate.parts)
    resolved_parent = path.parent.resolve()
    try:
        resolved_parent.relative_to(run_dir)
    except ValueError:
        raise ValueError(f"artifact escapes run directory: {value!r}") from None
    if path.is_symlink() or not path.is_file():
        raise ValueError(f"artifact is missing or symlinked: {value!r}")
    return path


def is_pass(payload: dict[str, Any]) -> bool:
    return (
        payload.get("status") == "PASS"
        and payload.get("violation_count", 0) == 0
    )


def require_hash(value: Any, label: str) -> str:
    text = str(value or "")
    if not SHA256.fullmatch(text):
        raise ValueError(f"{label} must be a lowercase SHA-256")
    return text


def validate_environment(payload: dict[str, Any]) -> list[str]:
    violations: list[str] = []
    candidate = payload.get("candidate")
    if not isinstance(candidate, dict):
        return ["environment candidate metadata is missing"]
    if not re.fullmatch(r"[0-9a-f]{40}", str(candidate.get("git_head") or "")):
        violations.append("candidate git_head is not an exact commit")
    if candidate.get("worktree_diff_sha256") != EMPTY_SHA256:
        violations.append("candidate worktree is not clean")
    required_hashes = (
        "openapi_sha256",
        "external_backend_image_digest",
        "dev_plus_backend_image_digest",
        "external_frontend_manifest_sha256",
        "dev_plus_frontend_manifest_sha256",
        "configuration_sha256",
        "migration_mapping_sha256",
        "snapshot_sha256",
        "review_manifest_sha256",
    )
    for field in required_hashes:
        value = str(payload.get(field) or "")
        if field.endswith("_image_digest"):
            if not re.fullmatch(r"sha256:[0-9a-f]{64}", value):
                violations.append(f"{field} is missing or mutable")
        elif not SHA256.fullmatch(value):
            violations.append(f"{field} is missing")
    return violations


def validate_g3(payload: dict[str, Any]) -> list[str]:
    violations: list[str] = []
    if payload.get("decision") not in {"APPROVED", "CONFIRMED", "confirmed"}:
        violations.append("review decision is not approved")
    summary = payload.get("summary")
    if not isinstance(summary, dict):
        violations.append("review summary is missing")
        return violations
    for field in ("proposed_review_count", "hard_blocked_count"):
        if summary.get(field) != 0:
            violations.append(f"{field} is not zero")
    require_hash(payload.get("candidate_sha256"), "candidate_sha256")
    require_hash(payload.get("cohort_digest"), "cohort_digest")
    return violations


def validate_g4(payload: dict[str, Any]) -> list[str]:
    violations: list[str] = []
    required_steps = {
        "dry_run_before",
        "apply",
        "idempotent_apply",
        "validate_after_apply",
        "rollback",
        "validate_after_rollback",
    }
    steps = payload.get("steps")
    if not isinstance(steps, list):
        return ["G4 steps are missing"]
    names = {
        str(row.get("step") or "")
        for row in steps
        if isinstance(row, dict) and row.get("exit_code") == 0
    }
    if names != required_steps:
        violations.append(
            f"G4 successful step set differs: {sorted(names)}"
        )
    return violations


def validate_g7(payload: dict[str, Any]) -> list[str]:
    violations: list[str] = []
    if payload.get("critical_scenario_pass_rate") not in {1, 1.0, "100%"}:
        violations.append("critical Computer Use scenario pass rate is not 100%")
    if payload.get("browser_surface") not in {"in_app_browser", "computer_use"}:
        violations.append("real browser surface is not recorded")
    if not payload.get("screenshot_evidence_sha256"):
        violations.append("screenshot evidence hash is missing")
    return violations


def validate_g9(payload: dict[str, Any]) -> list[str]:
    if payload.get("unresolved_p0_count") != 0:
        return ["adversarial review has unresolved P0"]
    if payload.get("unresolved_p1_count") != 0:
        return ["adversarial review has unresolved P1"]
    return []


def validate_g10(payload: dict[str, Any]) -> list[str]:
    violations: list[str] = []
    timings = payload.get("timings_seconds")
    if not isinstance(timings, dict):
        return ["G10 timing evidence is missing"]
    for name in ("apply", "validate", "rollback"):
        value = timings.get(name)
        if (
            isinstance(value, bool)
            or not isinstance(value, (int, float))
            or value < 0
            or value > 600
        ):
            violations.append(f"{name} exceeds 600 seconds or is invalid")
    total = timings.get("total")
    if (
        isinstance(total, bool)
        or not isinstance(total, (int, float))
        or total < 0
        or total > 1800
    ):
        violations.append("total maintenance rehearsal exceeds 1800 seconds")
    return violations


VALIDATORS = {
    "G0": validate_environment,
    "G3": validate_g3,
    "G4": validate_g4,
    "G7": validate_g7,
    "G9": validate_g9,
    "G10": validate_g10,
}


def validate_index(
    run_dir: pathlib.Path, index_path: pathlib.Path
) -> tuple[dict[str, Any], dict[str, Any]]:
    index = read_json(index_path)
    if index.get("schema_version") != 1:
        raise ValueError("evidence index schema_version must be 1")
    run_id = str(index.get("run_id") or "")
    if not RUN_ID.fullmatch(run_id) or run_dir.name != run_id:
        raise ValueError("run_id must match the run directory")
    artifacts = index.get("gates")
    if not isinstance(artifacts, dict) or set(artifacts) != set(GATES):
        raise ValueError("evidence index must contain exactly G0-G10")

    gate_results: dict[str, Any] = {}
    for gate in GATES:
        record = artifacts[gate]
        if not isinstance(record, dict) or set(record) != {
            "path",
            "sha256",
            "executor",
            "reviewer",
        }:
            raise ValueError(f"{gate} evidence record has an invalid shape")
        path = safe_artifact_path(run_dir, str(record["path"]))
        expected = require_hash(record["sha256"], f"{gate}.sha256")
        actual = sha256_file(path)
        violations: list[str] = []
        if actual != expected:
            violations.append(
                f"artifact hash mismatch expected={expected} actual={actual}"
            )
            payload: dict[str, Any] = {}
        else:
            payload = read_json(path)
            if payload.get("run_id") not in {None, run_id}:
                violations.append("artifact belongs to another run")
            if not is_pass(payload):
                violations.append("artifact status is not PASS/zero violations")
            validator = VALIDATORS.get(gate)
            if validator is not None:
                try:
                    violations.extend(validator(payload))
                except ValueError as exc:
                    violations.append(str(exc))
        executor = str(record.get("executor") or "").strip()
        reviewer = str(record.get("reviewer") or "").strip()
        if not executor or not reviewer or executor == reviewer:
            violations.append("executor/reviewer independence is not proven")
        gate_results[gate] = {
            "status": "PASS" if not violations else "BLOCKED",
            "violations": violations,
            "evidence": str(record["path"]),
            "evidence_sha256": actual,
            "executor": executor or None,
            "reviewer": reviewer or None,
        }

    signatures = index.get("signatures")
    signature_violations: list[str] = []
    if not isinstance(signatures, list):
        signature_violations.append("signatures must be an array")
        signatures = []
    index_unsigned = dict(index)
    index_unsigned["signatures"] = []
    index_digest = hashlib.sha256(canonical_bytes(index_unsigned)).hexdigest()
    seen_roles: set[str] = set()
    seen_signers: set[str] = set()
    for position, signature in enumerate(signatures):
        if not isinstance(signature, dict):
            signature_violations.append(f"signature[{position}] is invalid")
            continue
        role = str(signature.get("role") or "")
        signer = str(signature.get("signer") or "").strip()
        if (
            role not in REQUIRED_ROLES
            or role in seen_roles
            or not signer
            or signer in seen_signers
            or signature.get("decision") != "GO"
            or signature.get("evidence_index_sha256") != index_digest
            or not str(signature.get("signed_at") or "").strip()
        ):
            signature_violations.append(
                f"signature[{position}] is incomplete, duplicate, or hash-drifted"
            )
            continue
        seen_roles.add(role)
        seen_signers.add(signer)
    missing_roles = REQUIRED_ROLES - seen_roles
    if missing_roles:
        signature_violations.append(
            f"missing signature roles: {sorted(missing_roles)}"
        )

    passed = all(item["status"] == "PASS" for item in gate_results.values())
    passed = passed and not signature_violations
    report = {
        "schema_version": 1,
        "run_id": run_id,
        "decision": "GO" if passed else "NO-GO",
        "status": "PASS" if passed else "BLOCKED",
        "gate_count": len(GATES),
        "passed_gate_count": sum(
            item["status"] == "PASS" for item in gate_results.values()
        ),
        "gates": gate_results,
        "signature_violations": signature_violations,
        "evidence_index_sha256": sha256_file(index_path),
        "unsigned_evidence_index_sha256": index_digest,
        "finalized_at": dt.datetime.now(dt.timezone.utc)
        .isoformat()
        .replace("+00:00", "Z"),
    }
    return index, report


def write_outputs(
    run_dir: pathlib.Path,
    report: dict[str, Any],
    gate_report_path: pathlib.Path,
    decision_path: pathlib.Path,
    ledger_path: pathlib.Path,
) -> None:
    for path in (gate_report_path, decision_path, ledger_path):
        if path.exists():
            raise FileExistsError(f"refusing to overwrite final artifact: {path}")
        if path.parent.resolve() != run_dir:
            raise ValueError("final artifacts must be direct children of run_dir")
    gate_report_path.write_bytes(canonical_bytes(report))
    decision = (
        f"# V8 A/B decision: {report['decision']}\n\n"
        f"Run: `{report['run_id']}`\n\n"
        f"Passed gates: {report['passed_gate_count']}/{report['gate_count']}\n\n"
    )
    blocked = [
        (gate, row)
        for gate, row in report["gates"].items()
        if row["status"] != "PASS"
    ]
    if blocked:
        decision += "## Blocking gates\n\n"
        for gate, row in blocked:
            decision += f"- `{gate}`: {'; '.join(row['violations'])}\n"
    if report["signature_violations"]:
        decision += "\n## Signature blockers\n\n"
        for item in report["signature_violations"]:
            decision += f"- {item}\n"
    decision_path.write_text(decision, encoding="utf-8")
    ledger = {
        "timestamp": report["finalized_at"],
        "gate": "FINAL",
        "claim": f"formal V8 A/B decision is {report['decision']}",
        "status": report["status"],
        "evidence": [
            gate_report_path.name,
            decision_path.name,
        ],
        "executor": "release_finalizer",
        "reviewer": (
            "three_role_signature_set"
            if not report["signature_violations"]
            else None
        ),
        "boundary": "hash-only aggregation; no database, object, HTTP, or UI action",
        "uncertainty": "",
        "blockers": [
            gate
            for gate, row in report["gates"].items()
            if row["status"] != "PASS"
        ]
        + list(report["signature_violations"]),
    }
    ledger_path.write_bytes(canonical_bytes(ledger))


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-dir", type=pathlib.Path, required=True)
    parser.add_argument("--evidence-index", type=pathlib.Path, required=True)
    parser.add_argument("--gate-report", default="final-gate-report.json")
    parser.add_argument("--decision", default="go-no-go.md")
    parser.add_argument("--ledger", default="final-decision-ledger.json")
    args = parser.parse_args()
    try:
        run_dir = args.run_dir.resolve(strict=True)
        if not run_dir.is_dir() or run_dir.is_symlink():
            raise ValueError("run-dir must be an existing non-symlink directory")
        index_path = args.evidence_index.resolve(strict=True)
        if index_path.parent != run_dir or index_path.is_symlink():
            raise ValueError("evidence-index must be a direct run-dir file")
        _, report = validate_index(run_dir, index_path)
        write_outputs(
            run_dir,
            report,
            run_dir / args.gate_report,
            run_dir / args.decision,
            run_dir / args.ledger,
        )
        return 0 if report["decision"] == "GO" else 1
    except (OSError, UnicodeDecodeError, ValueError, json.JSONDecodeError) as exc:
        print(str(exc))
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
