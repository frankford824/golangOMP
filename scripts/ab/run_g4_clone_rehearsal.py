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
import shutil
import signal
import subprocess
import sys
import time
import urllib.parse
from typing import Any

try:
    from scripts.ab import clone_b_auth_policy, summarize_g4
except ModuleNotFoundError:
    import clone_b_auth_policy
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
PROCESS_GROUP_TERM_GRACE_SECONDS = 5.0
PROCESS_GROUP_KILL_GRACE_SECONDS = 5.0
PROCESS_GROUP_POLL_SECONDS = 0.05
WORKFLOW_STEPS = {
    "dry_run_before",
    "workflow_apply",
    "idempotent_apply",
    "validate_after_apply",
    "workflow_rollback",
}
APPLY_SEQUENCE = (
    "capture_baseline_fingerprint",
    "recovery_apply",
    "bundle_apply",
    "dry_run_before",
    "workflow_apply",
    "idempotent_apply",
    "validate_after_apply",
    "search_snapshot",
    "search_reindex",
)

CLONE_B_AUTH_DEPARTMENTS = clone_b_auth_policy.POLICY["departments"]
CLONE_B_AUTH_DEPARTMENT_TEAMS = clone_b_auth_policy.POLICY["department_teams"]


def validate_clone_b_auth_settings(
    path: pathlib.Path, clone_root: pathlib.Path
) -> tuple[bytes, dict[str, Any]]:
    if not path.is_absolute() or path.is_symlink() or not path.is_file():
        raise ValueError(
            "auth-settings-file must be an existing absolute regular non-symlink file"
        )
    resolved = path.resolve()
    resolved_root = clone_root.resolve()
    if resolved_root not in resolved.parents:
        raise ValueError("auth-settings-file must be contained by clone-root")
    if resolved.stat().st_mode & 0o222:
        raise ValueError("auth-settings-file must be read-only")
    raw = resolved.read_bytes()
    clone_b_auth_policy.validate(raw)
    return raw, {
        "byte_count": len(raw),
        "sha256": hashlib.sha256(raw).hexdigest(),
        "read_only": True,
        "super_admin_count": 0,
        "department_admin_key_count": 0,
        "configured_user_assignment_count": 0,
    }


def canonical_bytes(value: Any) -> bytes:
    return (
        json.dumps(
            value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
        )
        + "\n"
    ).encode("utf-8")


def linux_non_zombie_process_group_members(process_group_id: int) -> list[int]:
    if not sys.platform.startswith("linux"):
        raise RuntimeError(
            "G4 timeout cleanup requires Linux /proc process-group evidence"
        )
    members: list[int] = []
    for candidate in pathlib.Path("/proc").iterdir():
        if not candidate.name.isdigit():
            continue
        try:
            raw = (candidate / "stat").read_text(encoding="utf-8")
            fields = raw[raw.rfind(")") + 2 :].split()
            state = fields[0]
            process_group = int(fields[2])
        except (FileNotFoundError, IndexError, PermissionError, ValueError):
            continue
        if process_group == process_group_id and state != "Z":
            members.append(int(candidate.name))
    return sorted(members)


def terminate_process_group(
    process: subprocess.Popen[bytes],
    *,
    term_grace_seconds: float | None = None,
    kill_grace_seconds: float | None = None,
) -> tuple[bool, list[int], str | None]:
    if term_grace_seconds is None:
        term_grace_seconds = PROCESS_GROUP_TERM_GRACE_SECONDS
    if kill_grace_seconds is None:
        kill_grace_seconds = PROCESS_GROUP_KILL_GRACE_SECONDS
    process_group_id = process.pid
    last_members: list[int] = []
    for signal_value, grace_seconds in (
        (signal.SIGTERM, term_grace_seconds),
        (signal.SIGKILL, kill_grace_seconds),
    ):
        try:
            os.killpg(process_group_id, signal_value)
        except ProcessLookupError:
            pass
        except OSError as exc:
            return False, last_members, f"signal {signal_value.name}: {exc}"
        deadline = time.monotonic() + grace_seconds
        while True:
            process.poll()
            try:
                last_members = linux_non_zombie_process_group_members(
                    process_group_id
                )
            except RuntimeError as exc:
                return False, last_members, str(exc)
            if not last_members:
                if process.poll() is None:
                    try:
                        process.wait(timeout=PROCESS_GROUP_POLL_SECONDS)
                    except subprocess.TimeoutExpired:
                        last_members = [process.pid]
                    else:
                        return True, [], None
                else:
                    return True, [], None
            if time.monotonic() >= deadline:
                break
            time.sleep(PROCESS_GROUP_POLL_SECONDS)
    return False, last_members, "non-zombie process-group members remain"


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
    query_suffix = raw[match.end(3):]
    query = urllib.parse.parse_qs(
        urllib.parse.urlsplit(
            f"mysql://placeholder/{database}{query_suffix}"
        ).query,
        keep_blank_values=True,
    )
    parse_time_values = query.get("parseTime", [])
    if len(parse_time_values) != 1 or parse_time_values[0].lower() != "true":
        raise ValueError(
            "DSN must set parseTime=true for deterministic timestamp scans"
        )
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
    require_quiescence: bool = False,
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
    process_group_quiescent: bool | None = None
    with stdout.open("wb") as out_handle, stderr.open("wb") as err_handle:
        process = subprocess.Popen(
            argv,
            cwd=repo_root,
            env=env,
            stdout=out_handle,
            stderr=err_handle,
            start_new_session=True,
        )
        try:
            code = process.wait(timeout=timeout_seconds)
        except subprocess.TimeoutExpired:
            code = 124
            quiescent, remaining, cleanup_error = terminate_process_group(process)
            process_group_quiescent = quiescent
            if quiescent:
                message = (
                    f"\nstep timed out after {timeout_seconds:.3f}s; "
                    "verified zero non-zombie process-group members "
                    "before rollback\n"
                )
            else:
                code = 121
                message = (
                    f"\nstep timed out after {timeout_seconds:.3f}s; "
                    "process-group cleanup could not prove quiescence; "
                    f"remaining={remaining!r}; error={cleanup_error}\n"
                )
            err_handle.write(message.encode())
        else:
            if require_quiescence:
                members = linux_non_zombie_process_group_members(process.pid)
                if not members:
                    process_group_quiescent = True
                else:
                    quiescent, remaining, cleanup_error = terminate_process_group(
                        process
                    )
                    process_group_quiescent = quiescent
                    if quiescent:
                        if code == 0:
                            code = 122
                        message = (
                            "\nstep leader exited while non-zombie process-group "
                            "members remained; descendants were terminated before "
                            "continuation\n"
                        )
                    else:
                        code = 121
                        message = (
                            "\nstep leader exited while process-group members "
                            "remained and cleanup could not prove quiescence; "
                            f"remaining={remaining!r}; error={cleanup_error}\n"
                        )
                    err_handle.write(message.encode())
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
    record = {
        "step": step,
        "phase": phase,
        "exit_code": int(code),
        "elapsed_seconds": elapsed,
        "command_sha256": command_sha,
        "evidence": evidence,
    }
    if require_quiescence:
        record["process_group_quiescent"] = process_group_quiescent is True
    return record


def run(args: argparse.Namespace) -> dict[str, Any]:
    if not RUN_ID.fullmatch(args.run_id):
        raise ValueError("run-id is invalid")
    if shutil.which("go") is None:
        raise ValueError("go executable is required before creating run evidence")
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
    auth_settings_raw, auth_settings_attestation = (
        validate_clone_b_auth_settings(args.auth_settings_file, clone_root)
    )
    run_parent = run_dir.parent.resolve()
    if run_parent != clone_root.resolve() and clone_root.resolve() not in run_parent.parents:
        raise ValueError("run-dir must be a new descendant of clone-root")
    mapping = args.mapping_file
    if mapping.is_symlink() or not mapping.is_file():
        raise ValueError("mapping-file must be an existing non-symlink file")
    repo_root = pathlib.Path(__file__).resolve().parents[2]
    frontend_access_settings = repo_root / "config" / "frontend_access.json"
    if (
        frontend_access_settings.is_symlink()
        or not frontend_access_settings.is_file()
    ):
        raise ValueError(
            "tracked config/frontend_access.json must be an existing non-symlink file"
        )
    plan = validate_plan(args.command_plan)
    dsn = parse_local_clone_dsn(args.dsn_file, args.confirm_clone_database)

    placeholders = {
        "run_id": args.run_id,
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
    frozen_auth_settings = run_dir / "inputs" / "auth_identity.clone-b.json"
    frozen_auth_settings.write_bytes(auth_settings_raw)
    frozen_auth_settings.chmod(0o440)
    frozen_frontend_access_settings = (
        run_dir / "inputs" / "frontend_access.json"
    )
    frozen_frontend_access_settings.write_bytes(
        frontend_access_settings.read_bytes()
    )
    frozen_frontend_access_settings.chmod(0o440)

    env = dict(os.environ)
    env["MYSQL_DSN"] = dsn
    env["AB_CONFIRMED_CLONE_DATABASE"] = args.confirm_clone_database
    env["AB_CONFIRMED_CLONE_SIDE"] = "B"
    env["AUTH_SETTINGS_FILE"] = str(frozen_auth_settings.resolve())
    env["FRONTEND_ACCESS_SETTINGS_FILE"] = str(
        frozen_frontend_access_settings.resolve()
    )
    env["AUTH_ALLOW_EMBEDDED_SETTINGS"] = "false"
    env["AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS"] = "false"
    for feature_flag in (
        "WEB_PUSH_ENABLED",
        "AI_AGENT_ENABLED",
        "AI_CHAT_ENABLED",
        "AI_EMBEDDING_ENABLED",
        "VECTOR_SEARCH_ENABLED",
        "AI_RETRIEVAL_WORKER_ENABLED",
    ):
        env[feature_flag] = "false"
    records: list[dict[str, Any]] = []
    attempted = {
        "recovery": False,
        "bundle": False,
        "workflow": False,
        "search": False,
    }
    failed = False
    clone_db_quiescent = True
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
        if record["exit_code"] == 121:
            clone_db_quiescent = False
        failed = record["exit_code"] != 0

    rollback_specs = (
        ("search_rollback", "search"),
        ("workflow_rollback", "workflow"),
        ("bundle_rollback", "bundle"),
        ("recovery_rollback", "recovery"),
    )
    failed_rollback_prerequisite: str | None = None
    for step, component in rollback_specs:
        phase = "rollback"
        if not clone_db_quiescent:
            records.append(
                skipped_record(
                    step,
                    phase,
                    run_dir,
                    "blocked because a timed-out process group could not be "
                    "proven quiescent",
                )
            )
            continue
        if failed_rollback_prerequisite is not None:
            records.append(
                skipped_record(
                    step,
                    phase,
                    run_dir,
                    "blocked because rollback prerequisite "
                    f"{failed_rollback_prerequisite} failed",
                )
            )
            continue
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
        if record["exit_code"] == 121:
            clone_db_quiescent = False
        if record["exit_code"] != 0:
            failed_rollback_prerequisite = step

    step = "validate_after_rollback_fingerprint"
    if clone_db_quiescent:
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
    else:
        record = skipped_record(
            step,
            "rollback",
            run_dir,
            "blocked because a timed-out process group could not be proven "
            "quiescent",
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
        "auth_settings": sha256_file(frozen_auth_settings),
        "frontend_access_settings": sha256_file(
            frozen_frontend_access_settings
        ),
    }
    report["auth_settings_attestation"] = {
        **auth_settings_attestation,
        "frozen_input_path": frozen_auth_settings.resolve()
        .relative_to(clone_root.resolve())
        .as_posix(),
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
        "auth_settings_sha256": sha256_file(frozen_auth_settings),
        "frontend_access_settings_sha256": sha256_file(
            frozen_frontend_access_settings
        ),
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
    parser.add_argument("--auth-settings-file", type=pathlib.Path, required=True)
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
