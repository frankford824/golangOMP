#!/usr/bin/env python3
"""Summarize one complete Clone B G4/G10 rehearsal.

The summarizer is intentionally read-only.  It verifies the ordered step
ledger, every recorded stdout/stderr/declared-artifact hash, the rollback
fingerprint envelope, and the G4/G10 timing limits.  Its output is directly
consumable by ``finalize_release_gates.py``.
"""

from __future__ import annotations

import argparse
import csv
import hashlib
import json
import pathlib
import re
from typing import Any


SHA256 = re.compile(r"^[0-9a-f]{64}$")
REQUIRED_STEPS = (
    ("dry_run_before", "validate"),
    ("capture_baseline_fingerprint", "validate"),
    ("recovery_apply", "apply"),
    ("bundle_apply", "apply"),
    ("workflow_apply", "apply"),
    ("idempotent_apply", "apply"),
    ("validate_after_apply", "validate"),
    ("search_snapshot", "apply"),
    ("search_reindex", "apply"),
    ("workflow_rollback", "rollback"),
    ("search_rollback", "rollback"),
    ("bundle_rollback", "rollback"),
    ("recovery_rollback", "rollback"),
    ("validate_after_rollback_fingerprint", "rollback"),
)
SEARCH_TABLES = {
    "task_search_documents",
    "task_asset_group_search_documents",
    "product_search_documents",
}


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


def read_steps(path: pathlib.Path) -> list[dict[str, Any]]:
    if path.suffix == ".jsonl":
        rows: list[dict[str, Any]] = []
        for line_number, line in enumerate(
            path.read_text(encoding="utf-8").splitlines(), 1
        ):
            if not line.strip():
                continue
            value = json.loads(line)
            if not isinstance(value, dict):
                raise ValueError(f"steps line {line_number} is not an object")
            rows.append(value)
        return rows
    with path.open(encoding="utf-8", newline="") as handle:
        return list(csv.DictReader(handle, delimiter="\t"))


def safe_recorded_path(
    roots: dict[str, pathlib.Path], record: dict[str, Any]
) -> pathlib.Path:
    root_name = str(record.get("root") or "")
    relative = pathlib.PurePosixPath(str(record.get("path") or ""))
    if (
        root_name not in roots
        or relative.is_absolute()
        or not relative.parts
        or any(part in {"", ".", ".."} for part in relative.parts)
    ):
        raise ValueError("recorded artifact path is unsafe")
    root = roots[root_name]
    target = root.joinpath(*relative.parts)
    if target.is_symlink() or not target.is_file():
        raise ValueError("recorded artifact is missing or symlinked")
    try:
        target.resolve().relative_to(root)
    except ValueError:
        raise ValueError("recorded artifact escapes its allowed root") from None
    return target


def numeric(value: Any, label: str) -> float:
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise ValueError(f"{label} must be numeric")
    if value < 0:
        raise ValueError(f"{label} must be non-negative")
    return float(value)


def validate_fingerprint(
    baseline_path: pathlib.Path,
    rollback_path: pathlib.Path,
) -> tuple[dict[str, Any], dict[str, Any], list[dict[str, str]]]:
    violations: list[dict[str, str]] = []
    try:
        baseline_payload = json.loads(
            baseline_path.read_text(encoding="utf-8")
        )
        payload = json.loads(rollback_path.read_text(encoding="utf-8"))
        if not isinstance(baseline_payload, dict):
            raise ValueError("baseline fingerprint document is not an object")
        if not isinstance(payload, dict):
            raise ValueError("fingerprint document is not an object")
        baseline_tables = baseline_payload.get("tables")
        baseline_internal = str(
            baseline_payload.get("fingerprint_sha256") or ""
        )
        baseline_artifact = sha256_file(baseline_path)
        if (
            baseline_payload.get("schema_version") != 1
            or baseline_payload.get("kind")
            != "clone-b-baseline-fingerprint"
            or not isinstance(baseline_tables, dict)
            or not baseline_tables
            or baseline_internal
            != hashlib.sha256(canonical_bytes(baseline_tables)).hexdigest()
            or baseline_artifact
            != hashlib.sha256(canonical_bytes(baseline_payload)).hexdigest()
        ):
            raise ValueError("baseline fingerprint envelope is invalid")
        baseline = str(payload.get("baseline_fingerprint_sha256") or "")
        rollback = str(payload.get("rollback_fingerprint_sha256") or "")
        if (
            payload.get("schema_version") != 1
            or payload.get("status") != "PASS"
            or payload.get("violation_count") != 0
            or not SHA256.fullmatch(baseline)
            or rollback != baseline
            or baseline != baseline_internal
            or payload.get("baseline_artifact_sha256")
            != baseline_artifact
        ):
            violations.append(
                {
                    "violation_code": "g4.rollback_fingerprint_mismatch",
                    "entity_key": "rollback_fingerprint",
                    "detail": "fingerprint envelope is not PASS/equal/zero-violation",
                }
            )
    except (OSError, UnicodeError, json.JSONDecodeError, ValueError) as exc:
        baseline_payload = {}
        payload = {}
        violations.append(
            {
                "violation_code": "g4.rollback_fingerprint_unreadable",
                "entity_key": "rollback_fingerprint",
                "detail": str(exc),
            }
        )
    return baseline_payload, payload, violations


def validate_search_restore(
    snapshot_path: pathlib.Path,
    snapshot_archive_path: pathlib.Path,
    rollback_path: pathlib.Path,
) -> tuple[dict[str, Any], dict[str, Any], list[dict[str, str]]]:
    violations: list[dict[str, str]] = []
    try:
        snapshot = json.loads(snapshot_path.read_text(encoding="utf-8"))
        rollback = json.loads(rollback_path.read_text(encoding="utf-8"))
        if not isinstance(snapshot, dict) or not isinstance(rollback, dict):
            raise ValueError("search restore documents must be objects")
        tables = snapshot.get("tables")
        if (
            snapshot.get("schema_version") != 1
            or snapshot.get("status") != "CAPTURED"
            or snapshot.get("violation_count") != 0
            or not isinstance(tables, dict)
            or set(tables) != SEARCH_TABLES
        ):
            raise ValueError("search snapshot envelope is invalid")
        archive = snapshot.get("archive")
        if (
            not isinstance(archive, dict)
            or set(archive) != {"format", "sha256", "size"}
            or archive.get("format") != "deterministic-jsonl-v1"
            or not SHA256.fullmatch(str(archive.get("sha256") or ""))
            or isinstance(archive.get("size"), bool)
            or not isinstance(archive.get("size"), int)
            or archive["size"] < 0
            or not snapshot_archive_path.is_file()
            or snapshot_archive_path.is_symlink()
            or snapshot_archive_path.stat().st_size != archive["size"]
            or sha256_file(snapshot_archive_path) != archive["sha256"]
        ):
            raise ValueError("search snapshot archive is missing or drifted")
        for name, value in tables.items():
            if (
                not isinstance(value, dict)
                or set(value) != {"row_count", "content_sha256"}
                or isinstance(value["row_count"], bool)
                or not isinstance(value["row_count"], int)
                or value["row_count"] < 0
                or not SHA256.fullmatch(str(value["content_sha256"] or ""))
            ):
                raise ValueError(f"search snapshot table {name} is invalid")
        snapshot_sha = hashlib.sha256(canonical_bytes(tables)).hexdigest()
        if snapshot.get("snapshot_sha256") != snapshot_sha:
            raise ValueError("search snapshot SHA-256 is stale")
        if (
            rollback.get("schema_version") != 1
            or rollback.get("status") != "PASS"
            or rollback.get("violation_count") != 0
            or rollback.get("snapshot_sha256") != snapshot_sha
            or rollback.get("restored_snapshot_sha256") != snapshot_sha
            or rollback.get("restored_tables") != tables
            or rollback.get("source_archive_sha256") != archive["sha256"]
        ):
            raise ValueError("search rollback is not an exact snapshot restore")
    except (OSError, UnicodeError, json.JSONDecodeError, ValueError) as exc:
        snapshot = {}
        rollback = {}
        violations.append(
            {
                "violation_code": "g4.search_restore_mismatch",
                "entity_key": "search_documents",
                "detail": str(exc),
            }
        )
    return snapshot, rollback, violations


def summarize(
    *,
    run_id: str,
    run_dir: pathlib.Path,
    clone_root: pathlib.Path,
    steps_path: pathlib.Path,
    baseline_fingerprint_path: pathlib.Path,
    rollback_fingerprint_path: pathlib.Path,
    search_snapshot_path: pathlib.Path,
    search_snapshot_archive_path: pathlib.Path,
    search_rollback_path: pathlib.Path,
    total_seconds: float,
    max_step: float,
    max_phase: float,
    max_total: float,
) -> dict[str, Any]:
    violations: list[dict[str, str]] = []
    rows = read_steps(steps_path)
    actual_sequence = [
        (str(row.get("step") or ""), str(row.get("phase") or ""))
        for row in rows
    ]
    if actual_sequence != list(REQUIRED_STEPS):
        violations.append(
            {
                "violation_code": "g4.step_sequence",
                "entity_key": "steps",
                "detail": repr(actual_sequence),
            }
        )

    roots = {"run_dir": run_dir.resolve(), "clone_root": clone_root.resolve()}
    timings = {"apply": 0.0, "validate": 0.0, "rollback": 0.0}
    normalized_steps: list[dict[str, Any]] = []
    for index, row in enumerate(rows):
        step = str(row.get("step") or f"index-{index}")
        phase = str(row.get("phase") or "")
        try:
            exit_code = row.get("exit_code")
            if isinstance(exit_code, bool) or not isinstance(exit_code, int):
                raise ValueError("exit_code must be an integer")
            elapsed = numeric(row.get("elapsed_seconds"), "elapsed_seconds")
        except ValueError as exc:
            violations.append(
                {
                    "violation_code": "g4.step_numeric_invalid",
                    "entity_key": step,
                    "detail": str(exc),
                }
            )
            exit_code, elapsed = 125, 0.0
        if phase in timings:
            timings[phase] += elapsed
        else:
            violations.append(
                {
                    "violation_code": "g4.phase_invalid",
                    "entity_key": step,
                    "detail": phase,
                }
            )
        if exit_code != 0:
            violations.append(
                {
                    "violation_code": "g4.step_failed",
                    "entity_key": step,
                    "detail": str(exit_code),
                }
            )
        if elapsed > max_step:
            violations.append(
                {
                    "violation_code": "g4.step_timeout",
                    "entity_key": step,
                    "detail": str(elapsed),
                }
            )
        for evidence in row.get("evidence", []):
            try:
                if not isinstance(evidence, dict):
                    raise ValueError("evidence entry is not an object")
                expected = str(evidence.get("sha256") or "")
                if not SHA256.fullmatch(expected):
                    raise ValueError("evidence SHA-256 is invalid")
                target = safe_recorded_path(roots, evidence)
                actual = sha256_file(target)
                if actual != expected:
                    raise ValueError(
                        f"hash mismatch expected={expected} actual={actual}"
                    )
            except (OSError, ValueError) as exc:
                violations.append(
                    {
                        "violation_code": "g4.evidence_drift",
                        "entity_key": step,
                        "detail": str(exc),
                    }
                )
        normalized_steps.append(
            {
                "step": step,
                "phase": phase,
                "status": "PASS" if exit_code == 0 else "BLOCKED",
                "exit_code": int(exit_code),
                "elapsed_seconds": elapsed,
                "command_sha256": str(row.get("command_sha256") or ""),
                "evidence": row.get("evidence", []),
            }
        )

    for phase, elapsed in timings.items():
        if elapsed > max_phase:
            violations.append(
                {
                    "violation_code": "g10.phase_timeout",
                    "entity_key": phase,
                    "detail": str(elapsed),
                }
            )
    total = numeric(total_seconds, "total_seconds")
    timings["total"] = total
    if total > max_total:
        violations.append(
            {
                "violation_code": "g10.total_timeout",
                "entity_key": "total",
                "detail": str(total),
            }
        )

    baseline_fingerprint, fingerprint, fingerprint_violations = (
        validate_fingerprint(
            baseline_fingerprint_path,
            rollback_fingerprint_path,
        )
    )
    violations.extend(fingerprint_violations)
    search_snapshot, search_rollback, search_violations = validate_search_restore(
        search_snapshot_path,
        search_snapshot_archive_path,
        search_rollback_path,
    )
    violations.extend(search_violations)
    status = "PASS" if not violations else "BLOCKED"
    return {
        "schema_version": 2,
        "run_id": run_id,
        "status": status,
        "violation_count": len(violations),
        "violations": violations,
        "exit_code": 0 if status == "PASS" else 1,
        "elapsed_seconds": total,
        "total_seconds": total,
        "timings_seconds": timings,
        "steps": normalized_steps,
        "baseline_fingerprint": baseline_fingerprint,
        "baseline_fingerprint_artifact_sha256": (
            sha256_file(baseline_fingerprint_path)
            if baseline_fingerprint_path.is_file()
            else None
        ),
        "rollback_fingerprint": fingerprint,
        "rollback_fingerprint_sha256": (
            sha256_file(rollback_fingerprint_path)
            if rollback_fingerprint_path.is_file()
            else None
        ),
        "search_restore": {
            "snapshot": search_snapshot,
            "rollback": search_rollback,
        },
        "search_snapshot_sha256": (
            sha256_file(search_snapshot_path)
            if search_snapshot_path.is_file()
            else None
        ),
        "search_rollback_sha256": (
            sha256_file(search_rollback_path)
            if search_rollback_path.is_file()
            else None
        ),
        "search_snapshot_archive_sha256": (
            sha256_file(search_snapshot_archive_path)
            if search_snapshot_archive_path.is_file()
            else None
        ),
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--run-dir", type=pathlib.Path, required=True)
    parser.add_argument("--clone-root", type=pathlib.Path, required=True)
    parser.add_argument("--steps", type=pathlib.Path, required=True)
    parser.add_argument(
        "--baseline-fingerprint", type=pathlib.Path, required=True
    )
    parser.add_argument("--rollback-fingerprint", type=pathlib.Path, required=True)
    parser.add_argument("--search-snapshot", type=pathlib.Path, required=True)
    parser.add_argument(
        "--search-snapshot-archive", type=pathlib.Path, required=True
    )
    parser.add_argument("--search-rollback", type=pathlib.Path, required=True)
    parser.add_argument("--total-seconds", type=float, required=True)
    parser.add_argument("--max-step", type=float, default=600)
    parser.add_argument("--max-phase", type=float, default=600)
    parser.add_argument("--max-total", type=float, default=1800)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    args = parser.parse_args()
    result = summarize(
        run_id=args.run_id,
        run_dir=args.run_dir,
        clone_root=args.clone_root,
        steps_path=args.steps,
        baseline_fingerprint_path=args.baseline_fingerprint,
        rollback_fingerprint_path=args.rollback_fingerprint,
        search_snapshot_path=args.search_snapshot,
        search_snapshot_archive_path=args.search_snapshot_archive,
        search_rollback_path=args.search_rollback,
        total_seconds=args.total_seconds,
        max_step=args.max_step,
        max_phase=args.max_phase,
        max_total=args.max_total,
    )
    args.output.write_bytes(canonical_bytes(result))
    return result["exit_code"]


if __name__ == "__main__":
    raise SystemExit(main())
