#!/usr/bin/env python3
"""Fail-closed G4/G10 full-chain rehearsal orchestrator for local Clone B.

No command is interpreted by a shell.  File/object recovery, bundle
materialization, search reindex, their cleanup operations, and rollback
fingerprint validation are explicit command hooks.  The workflow migration
commands are constructed here so their mapping, snapshot directory, report
paths, and exact clone database confirmation cannot drift.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import re
import subprocess
import sys
import time
from typing import Any

try:
    from scripts.ab import summarize_g4
except ModuleNotFoundError:
    import summarize_g4


SHA256 = re.compile(r"^[0-9a-f]{64}$")
RUN_ID = re.compile(r"^[a-z0-9][a-z0-9_-]{2,63}$")
CLONE_B_DB = re.compile(r"^ab_[A-Za-z0-9_]*_b(?:_|$)[A-Za-z0-9_]*$")
DSN = re.compile(
    r"^.+@tcp\((127\.0\.0\.1|localhost):([0-9]{1,5})\)"
    r"/([A-Za-z0-9_]+)(?:\?.*)?$"
)
HOOKS = {
    "capture_baseline_fingerprint",
    "recovery_apply",
    "bundle_apply",
    "search_snapshot",
    "search_reindex",
    "search_rollback",
    "bundle_rollback",
    "recovery_rollback",
    "validate_after_rollback_fingerprint",
}
WORKFLOW_STEPS = {
    "dry_run_before",
    "workflow_apply",
    "idempotent_apply",
    "validate_after_apply",
    "workflow_rollback",
}
APPLY_SEQUENCE = (
    "dry_run_before",
    "capture_baseline_fingerprint",
    "recovery_apply",
    "bundle_apply",
    "workflow_apply",
    "idempotent_apply",
    "validate_after_apply",
    "search_snapshot",
    "search_reindex",
)


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


def atomic_write(path: pathlib.Path, value: Any) -> None:
    if path.exists():
        raise FileExistsError(f"refusing to overwrite artifact: {path}")
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(f".{path.name}.{os.getpid()}.tmp")
    temporary.write_bytes(canonical_bytes(value))
    os.replace(temporary, path)


def read_object(path: pathlib.Path, label: str) -> dict[str, Any]:
    if path.is_symlink() or not path.is_file():
        raise ValueError(f"{label} must be an existing non-symlink file")
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{label} must be a JSON object")
    return value


def parse_local_clone_dsn(path: pathlib.Path, expected_database: str) -> str:
    if path.is_symlink() or not path.is_file():
        raise ValueError("dsn-file must be an existing non-symlink file")
    raw = path.read_text(encoding="utf-8").strip()
    match = DSN.fullmatch(raw)
    if not match:
        raise ValueError("DSN must use tcp(127.0.0.1|localhost:port)")
    port = int(match.group(2))
    database = match.group(3)
    if not 1 <= port <= 65535:
        raise ValueError("DSN port is invalid")
    if database != expected_database:
        raise ValueError("DSN database differs from --confirm-clone-database")
    if not CLONE_B_DB.fullmatch(database):
        raise ValueError("confirmed database must be an ab_*_b* Clone B name")
    return raw


def validate_argv(value: Any, label: str) -> list[str]:
    if (
        not isinstance(value, list)
        or not value
        or any(not isinstance(item, str) or not item for item in value)
    ):
        raise ValueError(f"{label} must be a non-empty string array")
    for item in value:
        lowered = item.lower()
        if "@tcp(" in lowered or lowered == "--dsn" or lowered.startswith("--dsn="):
            raise ValueError(f"{label} must receive MYSQL_DSN only from the runner")
    return list(value)


def validate_plan(path: pathlib.Path) -> dict[str, Any]:
    plan = read_object(path, "command-plan")
    if plan.get("schema_version") != 1:
        raise ValueError("command-plan.schema_version must be 1")
    validate_argv(plan.get("workflow_base_argv"), "workflow_base_argv")
    hooks = plan.get("hooks")
    if not isinstance(hooks, dict) or set(hooks) != HOOKS:
        raise ValueError(f"command-plan hooks must be exactly {sorted(HOOKS)}")
    for name, hook in hooks.items():
        if not isinstance(hook, dict) or set(hook) != {
            "argv",
            "expected_artifacts",
        }:
            raise ValueError(f"hooks.{name} has an invalid shape")
        validate_argv(hook["argv"], f"hooks.{name}.argv")
        artifacts = hook["expected_artifacts"]
        if not isinstance(artifacts, list) or any(
            not isinstance(item, str) or not item for item in artifacts
        ):
            raise ValueError(f"hooks.{name}.expected_artifacts is invalid")
    return plan


def expand(value: str, placeholders: dict[str, str]) -> str:
    result = value
    for name, replacement in placeholders.items():
        result = result.replace("{" + name + "}", replacement)
    if re.search(r"\{[A-Za-z0-9_]+\}", result):
        raise ValueError(f"unknown command-plan placeholder in {value!r}")
    return result


def evidence_ref(
    path: pathlib.Path,
    run_dir: pathlib.Path,
    clone_root: pathlib.Path,
) -> dict[str, str]:
    resolved = path.resolve()
    for root_name, root in (
        ("run_dir", run_dir.resolve()),
        ("clone_root", clone_root.resolve()),
    ):
        try:
            relative = resolved.relative_to(root)
        except ValueError:
            continue
        if path.is_symlink() or not path.is_file():
            raise ValueError(f"evidence is missing or symlinked: {path}")
        return {
            "root": root_name,
            "path": relative.as_posix(),
            "sha256": sha256_file(path),
        }
    raise ValueError(f"evidence is outside run-dir/clone-root: {path}")


def workflow_argv(
    base: list[str],
    step: str,
    mapping: pathlib.Path,
    database: str,
    run_dir: pathlib.Path,
) -> tuple[list[str], list[pathlib.Path]]:
    common = list(base) + [
        "--mapping-file",
        str(mapping),
        "--confirm-database",
        database,
    ]
    reports = {
        "dry_run_before": run_dir / "dry-run-before.json",
        "workflow_apply": run_dir / "workflow-apply.json",
        "idempotent_apply": run_dir / "idempotent-apply.json",
        "validate_after_apply": run_dir / "validate-after-apply.json",
    }
    if step in {"dry_run_before", "validate_after_apply"}:
        report = reports[step]
        argv = common + ["--dry-run=true", "--report-file", str(report)]
        expected = [report]
    elif step in {"workflow_apply", "idempotent_apply"}:
        report = reports[step]
        argv = common + [
            "--dry-run=false",
            "--apply",
            "--snapshot-dir",
            str(run_dir / "workflow-snapshot"),
            "--report-file",
            str(report),
        ]
        expected = [report]
    elif step == "workflow_rollback":
        argv = common + [
            "--dry-run=false",
            "--rollback",
            "--snapshot-dir",
            str(run_dir / "workflow-snapshot"),
        ]
        # The current workflow rollback CLI exits directly after rollback and
        # does not emit --report-file. stdout/stderr and the final independent
        # fingerprint are the bound evidence for this step.
        expected = []
    else:
        raise ValueError(f"unknown workflow step: {step}")
    return argv, expected


def skipped_record(
    step: str,
    phase: str,
    run_dir: pathlib.Path,
    reason: str,
) -> dict[str, Any]:
    stdout = run_dir / f"{step}.stdout"
    stderr = run_dir / f"{step}.stderr"
    stdout.write_bytes(b"")
    stderr.write_text(reason + "\n", encoding="utf-8")
    return {
        "step": step,
        "phase": phase,
        "exit_code": 125,
        "elapsed_seconds": 0.0,
        "command_sha256": hashlib.sha256(
            canonical_bytes({"step": step, "skipped": reason})
        ).hexdigest(),
        "evidence": [
            evidence_ref(stdout, run_dir, run_dir),
            evidence_ref(stderr, run_dir, run_dir),
        ],
    }


def execute_step(
    *,
    step: str,
    phase: str,
    argv: list[str],
    expected_artifacts: list[pathlib.Path],
    run_dir: pathlib.Path,
    clone_root: pathlib.Path,
    repo_root: pathlib.Path,
    env: dict[str, str],
    timeout_seconds: float,
) -> dict[str, Any]:
    for path in expected_artifacts:
        resolved = path.resolve(strict=False)
        if not (
            resolved == run_dir.resolve()
            or run_dir.resolve() in resolved.parents
            or clone_root.resolve() in resolved.parents
        ):
            raise ValueError(f"{step} expected artifact is outside allowed roots")
        if path.exists():
            raise FileExistsError(
                f"{step} expected artifact existed before command: {path}"
            )
    command_document = {"step": step, "argv": argv}
    command_sha = hashlib.sha256(canonical_bytes(command_document)).hexdigest()
    with (run_dir / "commands.jsonl").open("ab") as handle:
        handle.write(canonical_bytes({**command_document, "sha256": command_sha}))
    stdout = run_dir / f"{step}.stdout"
    stderr = run_dir / f"{step}.stderr"
    started = time.monotonic()
    code = 0
    with stdout.open("wb") as out_handle, stderr.open("wb") as err_handle:
        try:
            completed = subprocess.run(
                argv,
                cwd=repo_root,
                env=env,
                stdout=out_handle,
                stderr=err_handle,
                timeout=timeout_seconds,
                check=False,
            )
            code = completed.returncode
        except subprocess.TimeoutExpired:
            code = 124
    elapsed = time.monotonic() - started
    evidence = [
        evidence_ref(stdout, run_dir, clone_root),
        evidence_ref(stderr, run_dir, clone_root),
    ]
    if code == 0:
        try:
            evidence.extend(
                evidence_ref(path, run_dir, clone_root)
                for path in expected_artifacts
            )
        except (OSError, ValueError) as exc:
            code = 120
            with stderr.open("ab") as handle:
                handle.write(
                    f"post-command evidence validation failed: {exc}\n".encode()
                )
            evidence[1] = evidence_ref(stderr, run_dir, clone_root)
    return {
        "step": step,
        "phase": phase,
        "exit_code": int(code),
        "elapsed_seconds": elapsed,
        "command_sha256": command_sha,
        "evidence": evidence,
    }


def run(args: argparse.Namespace) -> dict[str, Any]:
    if not RUN_ID.fullmatch(args.run_id):
        raise ValueError("run-id is invalid")
    run_dir = args.run_dir.resolve()
    if run_dir.exists():
        raise FileExistsError("run-dir must not already exist")
    clone_root = args.clone_root
    if (
        not clone_root.is_absolute()
        or not clone_root.is_dir()
        or clone_root.is_symlink()
    ):
        raise ValueError("clone-root must be an existing absolute non-symlink directory")
    if clone_root.name != args.run_id:
        raise ValueError("clone-root directory name must equal run-id")
    run_parent = run_dir.parent.resolve()
    if run_parent != clone_root.resolve() and clone_root.resolve() not in run_parent.parents:
        raise ValueError("run-dir must be a new descendant of clone-root")
    mapping = args.mapping_file
    if mapping.is_symlink() or not mapping.is_file():
        raise ValueError("mapping-file must be an existing non-symlink file")
    repo_root = pathlib.Path(__file__).resolve().parents[2]
    plan = validate_plan(args.command_plan)
    dsn = parse_local_clone_dsn(args.dsn_file, args.confirm_clone_database)

    placeholders = {
        "run_dir": str(run_dir.resolve()),
        "clone_root": str(clone_root.resolve()),
        "repo_root": str(repo_root),
        "mapping_file": str(mapping.resolve()),
        "dsn_file": str(args.dsn_file.resolve()),
        "database": args.confirm_clone_database,
        "rollback_fingerprint": str(
            (run_dir / "rollback-fingerprint.json").resolve()
        ),
        "baseline_fingerprint": str(
            (run_dir / "baseline-fingerprint.json").resolve()
        ),
        "search_snapshot": str((run_dir / "search-snapshot.json").resolve()),
        "search_snapshot_archive": str(
            (run_dir / "search-documents-snapshot.jsonl").resolve()
        ),
        "search_rollback": str((run_dir / "search-rollback.json").resolve()),
    }
    workflow_base = [
        expand(item, placeholders)
        for item in validate_argv(
            plan["workflow_base_argv"], "workflow_base_argv"
        )
    ]
    hooks: dict[str, tuple[list[str], list[pathlib.Path]]] = {}
    for name, hook in plan["hooks"].items():
        hooks[name] = (
            [expand(item, placeholders) for item in hook["argv"]],
            [
                pathlib.Path(expand(item, placeholders))
                for item in hook["expected_artifacts"]
            ],
        )
    fingerprint_path = run_dir / "rollback-fingerprint.json"
    baseline_fingerprint_path = run_dir / "baseline-fingerprint.json"
    if baseline_fingerprint_path not in hooks[
        "capture_baseline_fingerprint"
    ][1]:
        raise ValueError(
            "baseline capture hook must declare {baseline_fingerprint} "
            "as an expected artifact"
        )
    if str(baseline_fingerprint_path) not in hooks[
        "validate_after_rollback_fingerprint"
    ][0]:
        raise ValueError(
            "rollback fingerprint hook must consume the same "
            "{baseline_fingerprint} artifact"
        )
    if fingerprint_path not in hooks["validate_after_rollback_fingerprint"][1]:
        raise ValueError(
            "fingerprint hook must declare {rollback_fingerprint} as an expected artifact"
        )
    search_snapshot_path = run_dir / "search-snapshot.json"
    if search_snapshot_path not in hooks["search_snapshot"][1]:
        raise ValueError(
            "search_snapshot hook must declare {search_snapshot} as an expected artifact"
        )
    search_snapshot_archive_path = (
        run_dir / "search-documents-snapshot.jsonl"
    )
    if search_snapshot_archive_path not in hooks["search_snapshot"][1]:
        raise ValueError(
            "search_snapshot hook must declare {search_snapshot_archive} as an expected artifact"
        )
    search_rollback_path = run_dir / "search-rollback.json"
    if search_rollback_path not in hooks["search_rollback"][1]:
        raise ValueError(
            "search_rollback hook must declare {search_rollback} as an expected artifact"
        )

    run_dir.mkdir(parents=True)
    (run_dir / "workflow-snapshot").mkdir()
    (run_dir / "inputs").mkdir()
    (run_dir / "inputs" / "command-plan.json").write_bytes(
        args.command_plan.read_bytes()
    )
    (run_dir / "inputs" / "mapping.json").write_bytes(mapping.read_bytes())

    env = dict(os.environ)
    env["MYSQL_DSN"] = dsn
    env["AB_CONFIRMED_CLONE_DATABASE"] = args.confirm_clone_database
    env["AB_CONFIRMED_CLONE_SIDE"] = "B"
    records: list[dict[str, Any]] = []
    attempted = {
        "recovery": False,
        "bundle": False,
        "workflow": False,
        "search": False,
    }
    failed = False
    started = time.monotonic()

    for step in APPLY_SEQUENCE:
        phase = dict(summarize_g4.REQUIRED_STEPS)[step]
        if failed:
            records.append(
                skipped_record(step, phase, run_dir, "skipped after prior failure")
            )
            continue
        if step in WORKFLOW_STEPS:
            argv, expected = workflow_argv(
                workflow_base,
                step,
                mapping.resolve(),
                args.confirm_clone_database,
                run_dir.resolve(),
            )
        else:
            argv, expected = hooks[step]
        if step == "recovery_apply":
            attempted["recovery"] = True
        elif step == "bundle_apply":
            attempted["bundle"] = True
        elif step == "workflow_apply":
            attempted["workflow"] = True
        elif step == "search_reindex":
            attempted["search"] = True
        record = execute_step(
            step=step,
            phase=phase,
            argv=argv,
            expected_artifacts=expected,
            run_dir=run_dir,
            clone_root=clone_root,
            repo_root=repo_root,
            env=env,
            timeout_seconds=args.max_step_seconds,
        )
        records.append(record)
        failed = record["exit_code"] != 0

    rollback_specs = (
        ("workflow_rollback", "workflow"),
        ("search_rollback", "search"),
        ("bundle_rollback", "bundle"),
        ("recovery_rollback", "recovery"),
    )
    for step, component in rollback_specs:
        phase = "rollback"
        if not attempted[component]:
            records.append(
                skipped_record(
                    step, phase, run_dir, f"{component} apply was not attempted"
                )
            )
            continue
        if step == "workflow_rollback":
            argv, expected = workflow_argv(
                workflow_base,
                step,
                mapping.resolve(),
                args.confirm_clone_database,
                run_dir.resolve(),
            )
        else:
            argv, expected = hooks[step]
        record = execute_step(
            step=step,
            phase=phase,
            argv=argv,
            expected_artifacts=expected,
            run_dir=run_dir,
            clone_root=clone_root,
            repo_root=repo_root,
            env=env,
            timeout_seconds=args.max_step_seconds,
        )
        records.append(record)
        failed = failed or record["exit_code"] != 0

    step = "validate_after_rollback_fingerprint"
    argv, expected = hooks[step]
    record = execute_step(
        step=step,
        phase="rollback",
        argv=argv,
        expected_artifacts=expected,
        run_dir=run_dir,
        clone_root=clone_root,
        repo_root=repo_root,
        env=env,
        timeout_seconds=args.max_step_seconds,
    )
    records.append(record)
    failed = failed or record["exit_code"] != 0
    total_seconds = time.monotonic() - started

    steps_path = run_dir / "steps.jsonl"
    with steps_path.open("wb") as handle:
        for row in records:
            handle.write(canonical_bytes(row))
    report = summarize_g4.summarize(
        run_id=args.run_id,
        run_dir=run_dir,
        clone_root=clone_root,
        steps_path=steps_path,
        rollback_fingerprint_path=fingerprint_path,
        baseline_fingerprint_path=baseline_fingerprint_path,
        search_snapshot_path=search_snapshot_path,
        search_snapshot_archive_path=search_snapshot_archive_path,
        search_rollback_path=search_rollback_path,
        total_seconds=total_seconds,
        max_step=args.max_step_seconds,
        max_phase=args.max_phase_seconds,
        max_total=args.max_total_seconds,
    )
    report["input_sha256"] = {
        "command_plan": sha256_file(args.command_plan),
        "mapping": sha256_file(mapping),
    }
    report["evidence_inventory"] = [
        {
            "path": path.resolve().relative_to(clone_root.resolve()).as_posix(),
            "sha256": sha256_file(path),
        }
        for path in sorted(run_dir.rglob("*"))
        if path.is_file()
        and not path.is_symlink()
        and path.name not in {"g4-report.json", "evidence.sha256.json"}
    ]
    atomic_write(run_dir / "g4-report.json", report)
    evidence = {
        "schema_version": 1,
        "run_id": args.run_id,
        "status": report["status"],
        "command_plan_sha256": sha256_file(args.command_plan),
        "mapping_sha256": sha256_file(mapping),
        "steps_sha256": sha256_file(steps_path),
        "g4_report_sha256": sha256_file(run_dir / "g4-report.json"),
        "commands_sha256": sha256_file(run_dir / "commands.jsonl"),
        "raw_evidence": [
            evidence
            for row in records
            for evidence in row.get("evidence", [])
        ],
        "database": args.confirm_clone_database,
        "database_host_class": "local",
        "clone_side": "B",
    }
    atomic_write(run_dir / "evidence.sha256.json", evidence)
    return report


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--run-dir", type=pathlib.Path, required=True)
    parser.add_argument("--clone-root", type=pathlib.Path, required=True)
    parser.add_argument("--confirm-clone-database", required=True)
    parser.add_argument("--dsn-file", type=pathlib.Path, required=True)
    parser.add_argument("--mapping-file", type=pathlib.Path, required=True)
    parser.add_argument("--command-plan", type=pathlib.Path, required=True)
    parser.add_argument("--execute-clone-writes", action="store_true")
    parser.add_argument("--max-step-seconds", type=float, default=600)
    parser.add_argument("--max-phase-seconds", type=float, default=600)
    parser.add_argument("--max-total-seconds", type=float, default=1800)
    args = parser.parse_args(argv)
    if not args.execute_clone_writes:
        parser.error("--execute-clone-writes is required")
    for name in (
        "max_step_seconds",
        "max_phase_seconds",
        "max_total_seconds",
    ):
        value = getattr(args, name)
        if value <= 0:
            parser.error(f"--{name.replace('_', '-')} must be positive")
    return args


def main(argv: list[str] | None = None) -> int:
    try:
        report = run(parse_args(argv))
        return int(report["exit_code"])
    except (
        OSError,
        UnicodeError,
        ValueError,
        json.JSONDecodeError,
        subprocess.SubprocessError,
    ) as exc:
        print(str(exc), file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
