#!/usr/bin/env python3
"""Two-phase, fail-closed Clone B hold-open coordinator.

``apply-and-hold`` runs the existing G4 apply sequence but deliberately does
not summarize G4/G10 or roll back a successful apply.  It publishes
``HOLD_OPEN_READY.json`` only after every command has exited, every command
process group is proven quiescent, and every apply artifact is hash-bound.

``resume-and-rollback`` revalidates the frozen execution identity, the ready
ledger, all apply artifacts, and caller-supplied G5/G6 evidence manifests.  It
then performs the existing strict rollback order and final fingerprint check.

``abort-and-rollback`` preserves a hash-bound G5 failure, or a G5 pass followed
by a hash-bound G6 failure, while using the same rollback authorization and
fingerprint chain.  It returns a non-zero terminal result after safe recovery.

The coordinator never accepts a non-local DSN, never targets a non-Clone-B
database, and never repeats an interrupted apply step.  After the operator has
independently proved the old process group quiescent, an explicit recovery
confirmation can authorize seed-bound rollback and the final fingerprint.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import re
import shutil
import stat
import subprocess
import sys
from dataclasses import dataclass
from typing import Any

try:
    from scripts.ab import run_g4_clone_rehearsal as g4
except ModuleNotFoundError:
    import run_g4_clone_rehearsal as g4


SCHEMA_VERSION = 1
SESSION_PATH = pathlib.PurePosixPath("state/session.json")
READY_PATH = pathlib.PurePosixPath("HOLD_OPEN_READY.json")
ROLLBACK_AUTH_PATH = pathlib.PurePosixPath("state/rollback-authorized.json")
ROLLBACK_COMPLETE_PATH = pathlib.PurePosixPath("ROLLBACK_COMPLETE.json")
APPLY_STEPS_PATH = pathlib.PurePosixPath("apply-steps.jsonl")
ROLLBACK_STEPS_PATH = pathlib.PurePosixPath("rollback-steps.jsonl")
APPLY_COMMANDS_PATH = pathlib.PurePosixPath("apply-commands.jsonl")
ROLLBACK_COMMANDS_PATH = pathlib.PurePosixPath("rollback-commands.jsonl")
DOCUMENT_HASH_FIELD = "document_sha256"
EVIDENCE_HASH_FIELD = "evidence_sha256"
GIT_OBJECT_ID = re.compile(r"^(?:[0-9a-f]{40}|[0-9a-f]{64})$")
APPLY_SEQUENCE = tuple(g4.APPLY_SEQUENCE)
ROLLBACK_COMPONENT_ORDER = (
    ("search_rollback", "search"),
    ("workflow_rollback", "workflow"),
    ("bundle_rollback", "bundle"),
    ("recovery_rollback", "recovery"),
)
FINAL_FINGERPRINT_STEP = "validate_after_rollback_fingerprint"
FEATURE_FLAGS_DISABLED = (
    "WEB_PUSH_ENABLED",
    "AI_AGENT_ENABLED",
    "AI_CHAT_ENABLED",
    "AI_EMBEDDING_ENABLED",
    "VECTOR_SEARCH_ENABLED",
    "AI_RETRIEVAL_WORKER_ENABLED",
)


@dataclass(frozen=True)
class Context:
    run_id: str
    run_dir: pathlib.Path
    clone_root: pathlib.Path
    repo_root: pathlib.Path
    database: str
    dsn: str
    mapping: pathlib.Path
    plan: dict[str, Any]
    hooks: dict[str, tuple[list[str], list[pathlib.Path]]]
    workflow_base: list[str]
    env: dict[str, str]
    session: dict[str, Any]
    max_step_seconds: float


def canonical_hash(value: Any) -> str:
    return hashlib.sha256(g4.canonical_bytes(value)).hexdigest()


def compact_canonical_hash(value: Any) -> str:
    return hashlib.sha256(
        json.dumps(
            value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
        ).encode("utf-8")
    ).hexdigest()


def add_self_hash(value: dict[str, Any], field: str = DOCUMENT_HASH_FIELD) -> dict[str, Any]:
    output = dict(value)
    if field in output:
        raise ValueError(f"{field} must not be supplied before hashing")
    output[field] = canonical_hash(output)
    return output


def validate_self_hash(
    value: dict[str, Any], label: str, field: str = DOCUMENT_HASH_FIELD
) -> None:
    expected = str(value.get(field) or "")
    unhashed = dict(value)
    unhashed.pop(field, None)
    if not g4.SHA256.fullmatch(expected) or canonical_hash(unhashed) != expected:
        raise ValueError(f"{label} self hash is missing or stale")


def write_hashed_json(
    path: pathlib.Path,
    value: dict[str, Any],
    field: str = DOCUMENT_HASH_FIELD,
) -> dict[str, Any]:
    output = add_self_hash(value, field)
    g4.atomic_write(path, output)
    return output


def read_hashed_json(
    path: pathlib.Path,
    label: str,
    field: str = DOCUMENT_HASH_FIELD,
) -> dict[str, Any]:
    value = read_strict_json(path, label)
    validate_self_hash(value, label, field)
    return value


def read_regular_file_bytes(path: pathlib.Path, label: str) -> bytes:
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
    try:
        descriptor = os.open(path, flags)
    except OSError as exc:
        raise ValueError(
            f"{label} must be an existing non-symlink file"
        ) from exc
    try:
        before = os.fstat(descriptor)
        if not stat.S_ISREG(before.st_mode):
            raise ValueError(f"{label} must be a regular file")
        chunks: list[bytes] = []
        while True:
            chunk = os.read(descriptor, 1024 * 1024)
            if not chunk:
                break
            chunks.append(chunk)
        after = os.fstat(descriptor)
    finally:
        os.close(descriptor)
    try:
        current = path.lstat()
    except OSError as exc:
        raise ValueError(f"{label} changed while it was read") from exc
    if (
        before.st_dev != after.st_dev
        or before.st_ino != after.st_ino
        or before.st_size != after.st_size
        or before.st_mtime_ns != after.st_mtime_ns
        or current.st_dev != after.st_dev
        or current.st_ino != after.st_ino
        or not stat.S_ISREG(current.st_mode)
    ):
        raise ValueError(f"{label} changed while it was read")
    data = b"".join(chunks)
    if len(data) != after.st_size:
        raise ValueError(f"{label} size changed while it was read")
    return data


def read_strict_json_bytes(data: bytes, label: str) -> dict[str, Any]:
    def no_duplicates(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
        value: dict[str, Any] = {}
        for key, item in pairs:
            if key in value:
                raise ValueError(f"{label} contains duplicate JSON key: {key}")
            value[key] = item
        return value

    try:
        value = json.loads(
            data.decode("utf-8"),
            object_pairs_hook=no_duplicates,
        )
    except json.JSONDecodeError as exc:
        raise ValueError(f"{label} is not valid JSON: {exc}") from exc
    if not isinstance(value, dict):
        raise ValueError(f"{label} must be a JSON object")
    return value


def read_strict_json(path: pathlib.Path, label: str) -> dict[str, Any]:
    return read_strict_json_bytes(read_regular_file_bytes(path, label), label)


def normalize_observed_result(
    gate: str, result: dict[str, Any]
) -> tuple[str, int]:
    source_status = result.get("status")
    violation_count = result.get("violation_count")
    violations = result.get("violations")
    if (
        not isinstance(violation_count, int)
        or isinstance(violation_count, bool)
        or violation_count < 0
        or not isinstance(violations, list)
        or len(violations) != violation_count
    ):
        raise ValueError(f"{gate} primary result count is invalid")
    if gate == "G5":
        allowed = {"PASS": "PASS", "FAIL": "FAIL"}
    elif gate == "G6":
        allowed = {"PASS": "PASS", "BLOCKED": "FAIL"}
        signature = result.get(EVIDENCE_HASH_FIELD)
        unsigned = dict(result)
        unsigned.pop(EVIDENCE_HASH_FIELD, None)
        if (
            not g4.SHA256.fullmatch(str(signature or ""))
            or compact_canonical_hash(unsigned) != signature
        ):
            raise ValueError("G6 primary result self hash is missing or stale")
    else:
        raise ValueError("observed gate is unsupported")
    status = allowed.get(source_status)
    if status is None:
        raise ValueError(f"{gate} primary result status is invalid")
    if (status == "PASS" and violation_count != 0) or (
        status == "FAIL" and violation_count <= 0
    ):
        raise ValueError(f"{gate} primary result status/count is inconsistent")
    return status, violation_count


def repo_head(repo_root: pathlib.Path) -> str:
    result = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=repo_root,
        check=False,
        capture_output=True,
        text=True,
        timeout=30,
    )
    value = result.stdout.strip()
    if result.returncode != 0 or not GIT_OBJECT_ID.fullmatch(value):
        raise ValueError("unable to bind coordinator to a Git HEAD")
    status = subprocess.run(
        ["git", "status", "--porcelain=v1", "--untracked-files=all"],
        cwd=repo_root,
        check=False,
        capture_output=True,
        text=True,
        timeout=30,
    )
    if status.returncode != 0:
        raise ValueError("unable to verify coordinator worktree cleanliness")
    if status.stdout:
        raise ValueError(
            "hold-open execution requires a clean Git worktree at apply and resume"
        )
    return value


def file_identity(path: pathlib.Path) -> dict[str, Any]:
    if path.is_symlink() or not path.is_file():
        raise ValueError(f"input must be an existing non-symlink file: {path}")
    return {
        "path": str(path.resolve()),
        "sha256": g4.sha256_file(path),
        "size": path.stat().st_size,
    }


def relative_file_identity(path: pathlib.Path, root: pathlib.Path) -> dict[str, Any]:
    resolved = path.resolve()
    try:
        relative = resolved.relative_to(root.resolve())
    except ValueError:
        raise ValueError(f"artifact escapes its allowed root: {path}") from None
    if path.is_symlink() or not path.is_file():
        raise ValueError(f"artifact is missing or symlinked: {path}")
    return {
        "path": relative.as_posix(),
        "sha256": g4.sha256_file(path),
        "size": path.stat().st_size,
    }


def validate_relative_artifact(
    root: pathlib.Path, item: dict[str, Any], label: str
) -> pathlib.Path:
    if set(item) != {"path", "sha256", "size"}:
        raise ValueError(f"{label} artifact shape is invalid")
    relative = pathlib.PurePosixPath(str(item["path"] or ""))
    if (
        relative.is_absolute()
        or not relative.parts
        or any(part in {"", ".", ".."} for part in relative.parts)
    ):
        raise ValueError(f"{label} artifact path is unsafe")
    target = root.joinpath(*relative.parts)
    try:
        target.resolve().relative_to(root.resolve())
    except ValueError:
        raise ValueError(f"{label} artifact escapes clone-root") from None
    if (
        target.is_symlink()
        or not target.is_file()
        or isinstance(item["size"], bool)
        or not isinstance(item["size"], int)
        or item["size"] < 0
        or target.stat().st_size != item["size"]
        or not g4.SHA256.fullmatch(str(item["sha256"] or ""))
        or g4.sha256_file(target) != item["sha256"]
    ):
        raise ValueError(f"{label} artifact is missing or drifted: {relative}")
    return target


def ensure_run_location(args: argparse.Namespace, *, create: bool) -> tuple[pathlib.Path, pathlib.Path]:
    if not g4.RUN_ID.fullmatch(args.run_id):
        raise ValueError("run-id is invalid")
    clone_root = args.clone_root
    if (
        not clone_root.is_absolute()
        or clone_root.is_symlink()
        or not clone_root.is_dir()
        or clone_root.name != args.run_id
    ):
        raise ValueError(
            "clone-root must be an existing absolute non-symlink directory "
            "whose name equals run-id"
        )
    run_dir = args.run_dir.resolve()
    parent = run_dir.parent.resolve()
    if parent != clone_root.resolve() and clone_root.resolve() not in parent.parents:
        raise ValueError("run-dir must be a descendant of clone-root")
    if create and run_dir.exists():
        raise FileExistsError("new hold-open run-dir already exists")
    if not create and (run_dir.is_symlink() or not run_dir.is_dir()):
        raise ValueError("existing hold-open run-dir is missing or unsafe")
    return clone_root.resolve(), run_dir


def current_input_identity(
    *,
    args: argparse.Namespace,
    repo_root: pathlib.Path,
    frontend_access: pathlib.Path,
) -> dict[str, Any]:
    return {
        "git_head": repo_head(repo_root),
        "git_worktree_clean": True,
        "command_plan": file_identity(args.command_plan),
        "mapping": file_identity(args.mapping_file),
        "recovery_evidence": file_identity(args.recovery_evidence_file),
        "recovery_source_root": {
            "path": str(args.recovery_source_root.resolve()),
            "kind": "frozen-recovery-source-directory",
        },
        "dsn": file_identity(args.dsn_file),
        "auth_settings": file_identity(args.auth_settings_file),
        "frontend_access_settings": file_identity(frontend_access),
        "expected_baseline": file_identity(args.expected_baseline_file),
    }


def freeze_recovery_evidence(
    source: pathlib.Path,
    source_root: pathlib.Path,
    run_dir: pathlib.Path,
) -> dict[str, pathlib.Path]:
    if (
        not source_root.is_absolute()
        or source_root.is_symlink()
        or not source_root.is_dir()
    ):
        raise ValueError(
            "recovery source root must be an absolute non-symlink directory"
        )
    source_root = source_root.resolve()
    value = g4.read_object(source, "recovery evidence")
    expected_evidence_hash = str(value.get(EVIDENCE_HASH_FIELD) or "")
    unsigned_evidence = dict(value)
    unsigned_evidence.pop(EVIDENCE_HASH_FIELD, None)
    if (
        not g4.SHA256.fullmatch(expected_evidence_hash)
        or compact_canonical_hash(unsigned_evidence) != expected_evidence_hash
    ):
        raise ValueError("recovery evidence self hash is missing or stale")
    rows = value.get("recoveries")
    if not isinstance(rows, list) or not rows:
        raise ValueError("recovery evidence has no recovery rows")
    source_dir = run_dir / "inputs" / "recovery-sources"
    source_dir.mkdir()
    frozen: dict[str, pathlib.Path] = {}
    rewritten_rows: list[dict[str, Any]] = []
    seen: set[int] = set()
    for index, item in enumerate(rows):
        if not isinstance(item, dict):
            raise ValueError(f"recovery evidence row {index} is invalid")
        source_task_asset = item.get("source_task_asset")
        if not isinstance(source_task_asset, dict):
            source_task_asset = {}
        task_asset_id = item.get(
            "source_task_asset_id", source_task_asset.get("id")
        )
        size = item.get("source_size", source_task_asset.get("file_size"))
        digest = str(item.get("source_sha256") or "")
        if (
            isinstance(task_asset_id, bool)
            or not isinstance(task_asset_id, int)
            or task_asset_id <= 0
            or task_asset_id in seen
            or isinstance(size, bool)
            or not isinstance(size, int)
            or size < 0
            or not g4.SHA256.fullmatch(digest)
        ):
            raise ValueError(f"recovery evidence row {index} identity is invalid")
        source_path = pathlib.Path(str(item.get("source_local_path") or ""))
        if not source_path.is_file():
            source_path = source_root / source_path.name
        if (
            not source_path.is_absolute()
            or source_path.is_symlink()
            or not source_path.is_file()
            or source_path.stat().st_size != size
            or g4.sha256_file(source_path) != digest
        ):
            raise ValueError(
                f"recovery source {task_asset_id} is missing or drifted"
            )
        target = source_dir / f"task-asset-{task_asset_id}-{digest}.bin"
        target.write_bytes(source_path.read_bytes())
        target.chmod(0o440)
        if target.stat().st_size != size or g4.sha256_file(target) != digest:
            raise ValueError(f"frozen recovery source {task_asset_id} drifted")
        rewritten = dict(item)
        rewritten["source_local_path"] = str(target.resolve())
        rewritten_rows.append(rewritten)
        frozen[f"recovery_source_{task_asset_id}"] = target
        seen.add(task_asset_id)
    rewritten_evidence = dict(value)
    rewritten_evidence.pop(EVIDENCE_HASH_FIELD, None)
    rewritten_evidence["recoveries"] = rewritten_rows
    rewritten_evidence[EVIDENCE_HASH_FIELD] = compact_canonical_hash(
        rewritten_evidence
    )
    evidence_target = run_dir / "inputs" / "recovery-evidence.json"
    g4.atomic_write(evidence_target, rewritten_evidence)
    evidence_target.chmod(0o440)
    frozen["recovery_evidence"] = evidence_target
    return frozen


def initialize_session(
    *,
    args: argparse.Namespace,
    run_dir: pathlib.Path,
    repo_root: pathlib.Path,
    frontend_access: pathlib.Path,
    auth_raw: bytes,
    input_identity: dict[str, Any],
) -> dict[str, Any]:
    run_dir.mkdir(parents=True)
    (run_dir / "workflow-snapshot").mkdir()
    (run_dir / "inputs").mkdir()
    (run_dir / "state").mkdir()
    frozen = {
        "command_plan": run_dir / "inputs" / "command-plan.json",
        "mapping": run_dir / "inputs" / "mapping.json",
        "auth_settings": run_dir / "inputs" / "auth_identity.clone-b.json",
        "frontend_access_settings": run_dir / "inputs" / "frontend_access.json",
        "expected_baseline": run_dir / "inputs" / "expected-baseline.json",
    }
    frozen["command_plan"].write_bytes(args.command_plan.read_bytes())
    frozen["mapping"].write_bytes(args.mapping_file.read_bytes())
    frozen["auth_settings"].write_bytes(auth_raw)
    frozen["frontend_access_settings"].write_bytes(frontend_access.read_bytes())
    frozen["expected_baseline"].write_bytes(args.expected_baseline_file.read_bytes())
    frozen.update(
        freeze_recovery_evidence(
            args.recovery_evidence_file,
            args.recovery_source_root,
            run_dir,
        )
    )
    frozen["auth_settings"].chmod(0o440)
    frozen["frontend_access_settings"].chmod(0o440)
    frozen["expected_baseline"].chmod(0o440)
    session = write_hashed_json(
        run_dir.joinpath(*SESSION_PATH.parts),
        {
            "schema_version": SCHEMA_VERSION,
            "kind": "clone-b-hold-open-session",
            "run_id": args.run_id,
            "database": args.confirm_clone_database,
            "clone_side": "B",
            "database_host_class": "local",
            "production_writes_executed": False,
            "source_inputs": input_identity,
            "frozen_inputs": {
                name: relative_file_identity(path, run_dir)
                for name, path in frozen.items()
            },
        },
    )
    return session


def validate_session(
    *,
    args: argparse.Namespace,
    run_dir: pathlib.Path,
    current_identity: dict[str, Any],
) -> dict[str, Any]:
    session = read_hashed_json(
        run_dir.joinpath(*SESSION_PATH.parts), "hold-open session"
    )
    if (
        session.get("schema_version") != SCHEMA_VERSION
        or session.get("kind") != "clone-b-hold-open-session"
        or session.get("run_id") != args.run_id
        or session.get("database") != args.confirm_clone_database
        or session.get("clone_side") != "B"
        or session.get("database_host_class") != "local"
        or session.get("production_writes_executed") is not False
        or session.get("source_inputs") != current_identity
    ):
        raise ValueError("hold-open session identity differs from current inputs")
    frozen = session.get("frozen_inputs")
    if not isinstance(frozen, dict) or not {
        "command_plan",
        "mapping",
        "recovery_evidence",
        "auth_settings",
        "frontend_access_settings",
        "expected_baseline",
    }.issubset(frozen) or not all(
        name in {
            "command_plan",
            "mapping",
            "recovery_evidence",
            "auth_settings",
            "frontend_access_settings",
            "expected_baseline",
        }
        or name.startswith("recovery_source_")
        for name in frozen
    ):
        raise ValueError("hold-open frozen input inventory is invalid")
    for name, item in frozen.items():
        validate_relative_artifact(run_dir, item, f"frozen {name}")
    return session


def build_hooks(
    *,
    plan: dict[str, Any],
    run_id: str,
    run_dir: pathlib.Path,
    clone_root: pathlib.Path,
    repo_root: pathlib.Path,
    mapping: pathlib.Path,
    recovery_evidence: pathlib.Path,
    dsn_file: pathlib.Path,
    database: str,
) -> tuple[list[str], dict[str, tuple[list[str], list[pathlib.Path]]]]:
    placeholders = {
        "run_id": run_id,
        "run_dir": str(run_dir),
        "clone_root": str(clone_root),
        "repo_root": str(repo_root),
        "mapping_file": str(mapping),
        "recovery_evidence_file": str(recovery_evidence),
        "dsn_file": str(dsn_file.resolve()),
        "database": database,
        "rollback_fingerprint": str(run_dir / "rollback-fingerprint.json"),
        "baseline_fingerprint": str(run_dir / "baseline-fingerprint.json"),
        "search_snapshot": str(run_dir / "search-snapshot.json"),
        "search_snapshot_archive": str(
            run_dir / "search-documents-snapshot.jsonl"
        ),
        "search_rollback": str(run_dir / "search-rollback.json"),
    }
    workflow_base = [
        g4.expand(item, placeholders)
        for item in g4.validate_argv(
            plan["workflow_base_argv"], "workflow_base_argv"
        )
    ]
    hooks: dict[str, tuple[list[str], list[pathlib.Path]]] = {}
    for name, hook in plan["hooks"].items():
        hooks[name] = (
            [g4.expand(item, placeholders) for item in hook["argv"]],
            [
                pathlib.Path(g4.expand(item, placeholders))
                for item in hook["expected_artifacts"]
            ],
        )
    baseline = run_dir / "baseline-fingerprint.json"
    rollback = run_dir / "rollback-fingerprint.json"
    search_snapshot = run_dir / "search-snapshot.json"
    search_archive = run_dir / "search-documents-snapshot.jsonl"
    search_rollback = run_dir / "search-rollback.json"
    if baseline not in hooks["capture_baseline_fingerprint"][1]:
        raise ValueError("baseline capture hook must declare the run baseline")
    if str(baseline) not in hooks["validate_after_rollback_fingerprint"][0]:
        raise ValueError("rollback fingerprint must consume the run baseline")
    if rollback not in hooks["validate_after_rollback_fingerprint"][1]:
        raise ValueError("rollback fingerprint artifact is not declared")
    if (
        search_snapshot not in hooks["search_snapshot"][1]
        or search_archive not in hooks["search_snapshot"][1]
    ):
        raise ValueError("search snapshot hook is not fully bound")
    if search_rollback not in hooks["search_rollback"][1]:
        raise ValueError("search rollback artifact is not declared")
    return workflow_base, hooks


def build_context(args: argparse.Namespace, *, allow_create: bool) -> Context:
    if shutil.which("go") is None:
        raise ValueError("go executable is required")
    repo_root = pathlib.Path(__file__).resolve().parents[2]
    frontend_access = repo_root / "config" / "frontend_access.json"
    if frontend_access.is_symlink() or not frontend_access.is_file():
        raise ValueError("tracked frontend access settings are missing")
    clone_root, run_dir = ensure_run_location(
        args, create=allow_create and not args.run_dir.exists()
    )
    dsn = g4.parse_local_clone_dsn(
        args.dsn_file, args.confirm_clone_database
    )
    auth_raw, _ = g4.validate_clone_b_auth_settings(
        args.auth_settings_file, clone_root
    )
    input_identity = current_input_identity(
        args=args,
        repo_root=repo_root,
        frontend_access=frontend_access,
    )
    if run_dir.exists():
        session = validate_session(
            args=args,
            run_dir=run_dir,
            current_identity=input_identity,
        )
    else:
        if not allow_create:
            raise ValueError("resume phase requires an existing hold-open run")
        session = initialize_session(
            args=args,
            run_dir=run_dir,
            repo_root=repo_root,
            frontend_access=frontend_access,
            auth_raw=auth_raw,
            input_identity=input_identity,
        )
    frozen_plan = run_dir / "inputs" / "command-plan.json"
    frozen_mapping = run_dir / "inputs" / "mapping.json"
    frozen_recovery_evidence = run_dir / "inputs" / "recovery-evidence.json"
    plan = g4.validate_plan(frozen_plan)
    workflow_base, hooks = build_hooks(
        plan=plan,
        run_id=args.run_id,
        run_dir=run_dir,
        clone_root=clone_root,
        repo_root=repo_root,
        mapping=frozen_mapping,
        recovery_evidence=frozen_recovery_evidence,
        dsn_file=args.dsn_file,
        database=args.confirm_clone_database,
    )
    env = dict(os.environ)
    env["MYSQL_DSN"] = dsn
    env["AB_CONFIRMED_CLONE_DATABASE"] = args.confirm_clone_database
    env["AB_CONFIRMED_CLONE_SIDE"] = "B"
    env["AUTH_SETTINGS_FILE"] = str(
        (run_dir / "inputs" / "auth_identity.clone-b.json").resolve()
    )
    env["FRONTEND_ACCESS_SETTINGS_FILE"] = str(
        (run_dir / "inputs" / "frontend_access.json").resolve()
    )
    env["AUTH_ALLOW_EMBEDDED_SETTINGS"] = "false"
    env["AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS"] = "false"
    for name in FEATURE_FLAGS_DISABLED:
        env[name] = "false"
    return Context(
        run_id=args.run_id,
        run_dir=run_dir,
        clone_root=clone_root,
        repo_root=repo_root,
        database=args.confirm_clone_database,
        dsn=dsn,
        mapping=frozen_mapping,
        plan=plan,
        hooks=hooks,
        workflow_base=workflow_base,
        env=env,
        session=session,
        max_step_seconds=args.max_step_seconds,
    )


def checkpoint_path(
    context: Context,
    phase: str,
    ordinal: int,
    step: str,
    suffix: str,
) -> pathlib.Path:
    safe_step = step.replace("_", "-")
    return (
        context.run_dir
        / "state"
        / f"{phase}-{ordinal:02d}-{safe_step}.{suffix}.json"
    )


def recorded_evidence_path(context: Context, evidence: dict[str, Any]) -> pathlib.Path:
    if not isinstance(evidence, dict) or set(evidence) != {
        "root",
        "path",
        "sha256",
    }:
        raise ValueError("step evidence shape is invalid")
    roots = {
        "run_dir": context.run_dir,
        "clone_root": context.clone_root,
    }
    root = roots.get(str(evidence["root"]))
    relative = pathlib.PurePosixPath(str(evidence["path"] or ""))
    if (
        root is None
        or relative.is_absolute()
        or not relative.parts
        or any(part in {"", ".", ".."} for part in relative.parts)
    ):
        raise ValueError("step evidence path is unsafe")
    target = root.joinpath(*relative.parts)
    if (
        target.is_symlink()
        or not target.is_file()
        or not g4.SHA256.fullmatch(str(evidence["sha256"] or ""))
        or g4.sha256_file(target) != evidence["sha256"]
    ):
        raise ValueError("step evidence is missing or drifted")
    return target


def validate_step_record(context: Context, record: dict[str, Any], step: str) -> None:
    if (
        record.get("step") != step
        or isinstance(record.get("exit_code"), bool)
        or not isinstance(record.get("exit_code"), int)
        or record.get("process_group_quiescent") is not True
        or not isinstance(record.get("evidence"), list)
    ):
        raise ValueError(f"{step} completion record is invalid")
    for evidence in record["evidence"]:
        recorded_evidence_path(context, evidence)


def load_phase_progress(
    context: Context,
    phase: str,
    steps: list[str],
    *,
    initial_hash: str | None = None,
) -> tuple[list[dict[str, Any]], str | None, str]:
    records: list[dict[str, Any]] = []
    prior_hash = initial_hash or str(context.session[DOCUMENT_HASH_FIELD])
    missing_seen = False
    interrupted: str | None = None
    for ordinal, step in enumerate(steps, 1):
        started_path = checkpoint_path(
            context, phase, ordinal, step, "started"
        )
        completed_path = checkpoint_path(
            context, phase, ordinal, step, "completed"
        )
        if completed_path.exists() and not started_path.exists():
            raise ValueError(f"{phase}/{step} completed without start checkpoint")
        if started_path.exists():
            if missing_seen:
                raise ValueError(f"{phase} checkpoint order is not a prefix")
            started = read_hashed_json(
                started_path, f"{phase}/{step} start checkpoint"
            )
            if (
                started.get("phase") != phase
                or started.get("ordinal") != ordinal
                or started.get("step") != step
                or started.get("prior_checkpoint_sha256") != prior_hash
            ):
                raise ValueError(f"{phase}/{step} start checkpoint differs")
            if not completed_path.exists():
                interrupted = step
                missing_seen = True
                continue
            completed = read_hashed_json(
                completed_path, f"{phase}/{step} completion checkpoint"
            )
            if (
                completed.get("phase") != phase
                or completed.get("ordinal") != ordinal
                or completed.get("step") != step
                or completed.get("started_checkpoint_sha256")
                != started[DOCUMENT_HASH_FIELD]
                or not isinstance(completed.get("record"), dict)
            ):
                raise ValueError(f"{phase}/{step} completion checkpoint differs")
            validate_step_record(context, completed["record"], step)
            records.append(completed["record"])
            prior_hash = str(completed[DOCUMENT_HASH_FIELD])
        else:
            if completed_path.exists():
                raise ValueError(f"{phase}/{step} checkpoint order is invalid")
            missing_seen = True
    return records, interrupted, prior_hash


def step_command(
    context: Context, step: str
) -> tuple[list[str], list[pathlib.Path]]:
    if step in g4.WORKFLOW_STEPS:
        return g4.workflow_argv(
            context.workflow_base,
            step,
            context.mapping,
            context.database,
            context.run_dir,
        )
    return context.hooks[step]


def execute_checkpointed_step(
    context: Context,
    *,
    phase: str,
    ordinal: int,
    step: str,
    prior_hash: str,
) -> tuple[dict[str, Any], str]:
    started = write_hashed_json(
        checkpoint_path(context, phase, ordinal, step, "started"),
        {
            "schema_version": SCHEMA_VERSION,
            "kind": "clone-b-hold-open-step-start",
            "run_id": context.run_id,
            "database": context.database,
            "phase": phase,
            "ordinal": ordinal,
            "step": step,
            "prior_checkpoint_sha256": prior_hash,
            "production_writes_executed": False,
        },
    )
    argv, expected = step_command(context, step)
    record = g4.execute_step(
        step=step,
        phase="apply" if phase == "apply" else "rollback",
        argv=argv,
        expected_artifacts=expected,
        run_dir=context.run_dir,
        clone_root=context.clone_root,
        repo_root=context.repo_root,
        env=context.env,
        timeout_seconds=context.max_step_seconds,
        require_quiescence=True,
    )
    completed = write_hashed_json(
        checkpoint_path(context, phase, ordinal, step, "completed"),
        {
            "schema_version": SCHEMA_VERSION,
            "kind": "clone-b-hold-open-step-complete",
            "run_id": context.run_id,
            "database": context.database,
            "phase": phase,
            "ordinal": ordinal,
            "step": step,
            "started_checkpoint_sha256": started[DOCUMENT_HASH_FIELD],
            "record": record,
            "production_writes_executed": False,
        },
    )
    return record, str(completed[DOCUMENT_HASH_FIELD])


def write_jsonl(path: pathlib.Path, rows: list[dict[str, Any]]) -> None:
    expected = b"".join(g4.canonical_bytes(row) for row in rows)
    if path.exists():
        if path.is_symlink() or not path.is_file() or path.read_bytes() != expected:
            raise ValueError(f"existing ledger differs: {path}")
        return
    path.write_bytes(expected)


def rotate_commands(
    context: Context,
    destination: pathlib.PurePosixPath,
    *,
    allow_empty: bool = False,
) -> None:
    source = context.run_dir / "commands.jsonl"
    target = context.run_dir.joinpath(*destination.parts)
    if target.exists():
        if source.exists():
            raise ValueError("both active and frozen command ledgers exist")
        return
    if allow_empty and not source.exists():
        target.write_bytes(b"")
        return
    if not source.is_file() or source.is_symlink():
        raise ValueError("active command ledger is missing")
    os.replace(source, target)


def inventory_run_files(context: Context) -> list[dict[str, Any]]:
    excluded = {
        READY_PATH.as_posix(),
        ROLLBACK_AUTH_PATH.as_posix(),
        ROLLBACK_COMPLETE_PATH.as_posix(),
    }
    values: list[dict[str, Any]] = []
    for path in sorted(context.run_dir.rglob("*")):
        if path.is_symlink():
            raise ValueError(f"run evidence contains a symlink: {path}")
        if not path.is_file():
            continue
        relative = path.relative_to(context.run_dir).as_posix()
        if relative in excluded or relative.startswith("state/rollback-"):
            continue
        values.append(relative_file_identity(path, context.run_dir))
    return values


def verify_baseline_database(context: Context) -> None:
    baseline = g4.read_object(
        context.run_dir / "baseline-fingerprint.json", "baseline fingerprint"
    )
    expected = g4.read_object(
        context.run_dir / "inputs" / "expected-baseline.json",
        "expected baseline fingerprint",
    )
    for label, value in (("captured", baseline), ("expected", expected)):
        tables = value.get("tables")
        if (
            value.get("schema_version") != 1
            or value.get("kind") != "clone-b-baseline-fingerprint"
            or value.get("database") != context.database
            or not isinstance(value.get("fingerprint_algorithm"), str)
            or not value["fingerprint_algorithm"]
            or not isinstance(tables, dict)
            or value.get("fingerprint_sha256") != canonical_hash(tables)
        ):
            raise ValueError(f"{label} baseline fingerprint is invalid")
    if baseline != expected:
        raise ValueError(
            "captured Clone B baseline differs from the frozen expected baseline"
        )


def publish_ready(
    context: Context, records: list[dict[str, Any]]
) -> dict[str, Any]:
    if any(
        row.get("exit_code") != 0
        or row.get("process_group_quiescent") is not True
        for row in records
    ):
        raise ValueError("HOLD_OPEN_READY requires successful quiescent apply steps")
    verify_baseline_database(context)
    apply_steps = context.run_dir.joinpath(*APPLY_STEPS_PATH.parts)
    if not apply_steps.exists():
        write_jsonl(apply_steps, records)
    rotate_commands(context, APPLY_COMMANDS_PATH)
    inventory = inventory_run_files(context)
    ready = write_hashed_json(
        context.run_dir.joinpath(*READY_PATH.parts),
        {
            "schema_version": SCHEMA_VERSION,
            "kind": "clone-b-hold-open-ready",
            "status": "HOLD_OPEN_READY",
            "run_id": context.run_id,
            "database": context.database,
            "clone_side": "B",
            "database_host_class": "local",
            "git_head": context.session["source_inputs"]["git_head"],
            "session_sha256": context.session[DOCUMENT_HASH_FIELD],
            "input_sha256": {
                name: value["sha256"]
                for name, value in context.session["source_inputs"].items()
                if isinstance(value, dict) and "sha256" in value
            },
            "apply_sequence": list(APPLY_SEQUENCE),
            "apply_steps_sha256": g4.sha256_file(apply_steps),
            "process_groups_quiescent": True,
            "apply_artifacts": inventory,
            "production_writes_executed": False,
        },
    )
    return ready


def validate_ready(context: Context) -> dict[str, Any]:
    ready = read_hashed_json(
        context.run_dir.joinpath(*READY_PATH.parts), "HOLD_OPEN_READY ledger"
    )
    if (
        ready.get("schema_version") != SCHEMA_VERSION
        or ready.get("kind") != "clone-b-hold-open-ready"
        or ready.get("status") != "HOLD_OPEN_READY"
        or ready.get("run_id") != context.run_id
        or ready.get("database") != context.database
        or ready.get("clone_side") != "B"
        or ready.get("database_host_class") != "local"
        or ready.get("git_head") != context.session["source_inputs"]["git_head"]
        or ready.get("session_sha256") != context.session[DOCUMENT_HASH_FIELD]
        or ready.get("process_groups_quiescent") is not True
        or ready.get("production_writes_executed") is not False
        or ready.get("apply_sequence") != list(APPLY_SEQUENCE)
    ):
        raise ValueError("HOLD_OPEN_READY identity is invalid")
    artifacts = ready.get("apply_artifacts")
    if not isinstance(artifacts, list) or not artifacts:
        raise ValueError("HOLD_OPEN_READY apply artifact inventory is missing")
    seen: set[str] = set()
    for item in artifacts:
        if not isinstance(item, dict) or str(item.get("path") or "") in seen:
            raise ValueError("HOLD_OPEN_READY artifact inventory is invalid")
        seen.add(str(item["path"]))
        validate_relative_artifact(context.run_dir, item, "HOLD_OPEN_READY")
    apply_steps = context.run_dir.joinpath(*APPLY_STEPS_PATH.parts)
    if (
        ready.get("apply_steps_sha256") != g4.sha256_file(apply_steps)
        or ready.get("input_sha256")
        != {
            name: value["sha256"]
            for name, value in context.session["source_inputs"].items()
            if isinstance(value, dict) and "sha256" in value
        }
    ):
        raise ValueError("HOLD_OPEN_READY hashes differ")
    return ready


def validate_observed_manifest(
    context: Context,
    path: pathlib.Path,
    gate: str,
    ready_sha256: str,
    *,
    required_status: str = "PASS",
) -> dict[str, Any]:
    if not path.is_absolute():
        raise ValueError(f"{gate} evidence manifest must be absolute")
    try:
        relative = path.resolve().relative_to(context.clone_root)
    except ValueError:
        raise ValueError(f"{gate} evidence manifest must be inside clone-root") from None
    manifest = read_hashed_json(
        path, f"{gate} evidence manifest", EVIDENCE_HASH_FIELD
    )
    status = manifest.get("status")
    violation_count = manifest.get("violation_count")
    status_count_valid = (
        status == "PASS"
        and isinstance(violation_count, int)
        and not isinstance(violation_count, bool)
        and violation_count == 0
    ) or (
        status == "FAIL"
        and isinstance(violation_count, int)
        and not isinstance(violation_count, bool)
        and violation_count > 0
    )
    if (
        set(manifest)
        != {
            "schema_version",
            "gate",
            "status",
            "violation_count",
            "hold_open_ledger_sha256",
            "artifacts",
            EVIDENCE_HASH_FIELD,
        }
        or manifest.get("schema_version") != SCHEMA_VERSION
        or manifest.get("gate") != gate
        or status != required_status
        or not status_count_valid
        or manifest.get("hold_open_ledger_sha256") != ready_sha256
        or not isinstance(manifest.get("artifacts"), list)
        or not manifest["artifacts"]
    ):
        raise ValueError(f"{gate} evidence manifest envelope is invalid")
    seen: set[str] = set()
    primary_result: pathlib.Path | None = None
    primary_identity: dict[str, Any] | None = None
    for item in manifest["artifacts"]:
        if not isinstance(item, dict) or str(item.get("path") or "") in seen:
            raise ValueError(f"{gate} evidence artifact list is invalid")
        seen.add(str(item["path"]))
        target = validate_relative_artifact(context.clone_root, item, gate)
        if primary_result is None:
            primary_result = target
            primary_identity = item
    if primary_result is None or primary_identity is None:
        raise ValueError(f"{gate} primary result artifact is missing")
    primary_bytes = read_regular_file_bytes(
        primary_result, f"{gate} primary result"
    )
    if (
        len(primary_bytes) != primary_identity["size"]
        or hashlib.sha256(primary_bytes).hexdigest()
        != primary_identity["sha256"]
    ):
        raise ValueError(f"{gate} primary result changed while it was read")
    source_status, source_count = normalize_observed_result(
        gate,
        read_strict_json_bytes(primary_bytes, f"{gate} primary result"),
    )
    if source_status != status or source_count != violation_count:
        raise ValueError(f"{gate} evidence differs from its primary result")
    return {
        "gate": gate,
        "status": status,
        "violation_count": violation_count,
        "path": relative.as_posix(),
        "sha256": g4.sha256_file(path),
        "evidence_sha256": manifest[EVIDENCE_HASH_FIELD],
    }


def rollback_seed_paths(
    context: Context,
) -> dict[str, tuple[pathlib.Path, ...]]:
    recovery_ownership = tuple(
        sorted(context.run_dir.glob("recovery-ownership-*.json"))
    ) + tuple(
        sorted(
            context.run_dir.glob(
                "recovery-staging-ownership-*.json"
            )
        )
    )
    bundle_ownership = tuple(
        sorted(context.run_dir.glob("bundle-ownership-*.json"))
    ) + tuple(
        sorted(
            context.run_dir.glob(
                "bundle-staging-ownership-*.json"
            )
        )
    )
    return {
        "recovery": (
            context.run_dir / "recovery-file-write-ahead.json",
            context.run_dir / "recovery-materialization-plan.json",
            context.run_dir / "recovery-guard-before.json",
            context.run_dir / "recovery-db-apply.json",
            context.run_dir / "recovery-component-apply.json",
        )
        + recovery_ownership,
        "bundle": (
            context.run_dir / "bundle-staging-write-ahead.json",
            context.run_dir / "bundle-file-write-ahead.json",
            context.run_dir / "bundle-guard-before.json",
            context.run_dir / "bundle-materialize-report.json",
            context.run_dir / "bundle-db-rollback-journal.json",
            context.run_dir / "bundle-db-apply.json",
            context.run_dir / "bundle-component-apply.json",
        )
        + bundle_ownership,
        "workflow": (
            context.run_dir
            / "workflow-snapshot"
            / "workflow-groups-snapshot.json",
            context.run_dir / "workflow-apply.json",
        ),
        "search": (
            context.run_dir / "search-snapshot.json",
            context.run_dir / "search-documents-snapshot.jsonl",
        ),
    }


def attempted_components(context: Context) -> list[str]:
    starts = {
        path.name
        for path in (context.run_dir / "state").glob("apply-*.started.json")
    }
    successful_steps: set[str] = set()
    for path in (context.run_dir / "state").glob("apply-*.completed.json"):
        checkpoint = read_hashed_json(path, "apply completion checkpoint")
        record = checkpoint.get("record")
        if (
            isinstance(record, dict)
            and record.get("exit_code") == 0
            and isinstance(record.get("step"), str)
        ):
            successful_steps.add(str(record["step"]))

    # A started checkpoint alone does not prove that the component crossed its
    # mutation boundary.  Each component writes its rollback seed before its
    # first durable change.  Require either a successful apply completion or
    # one of those seeds before authorizing the matching rollback hook.
    rollback_seeds = rollback_seed_paths(context)
    attempted: list[str] = []
    bindings = (
        ("recovery", "recovery-apply", "recovery_apply"),
        ("bundle", "bundle-apply", "bundle_apply"),
        ("workflow", "workflow-apply", "workflow_apply"),
        ("search", "search-reindex", "search_reindex"),
    )
    for component, token, step in bindings:
        started = any(token in name for name in starts)
        crossed_boundary = step in successful_steps or any(
            path.is_file() and not path.is_symlink()
            for path in rollback_seeds[component]
        )
        if started and crossed_boundary:
            attempted.append(component)
    return attempted


def rollback_steps_for(components: list[str], baseline_exists: bool) -> list[str]:
    steps = [
        step
        for step, component in ROLLBACK_COMPONENT_ORDER
        if component in components
    ]
    if baseline_exists:
        steps.append(FINAL_FINGERPRINT_STEP)
    return steps


def authorize_rollback(
    context: Context,
    *,
    reason: str,
    ready: dict[str, Any] | None,
    observed: list[dict[str, Any]],
    trigger: dict[str, Any] | None = None,
) -> dict[str, Any]:
    path = context.run_dir.joinpath(*ROLLBACK_AUTH_PATH.parts)
    components = attempted_components(context)
    seed_paths = rollback_seed_paths(context)
    steps = rollback_steps_for(
        components, (context.run_dir / "baseline-fingerprint.json").is_file()
    )
    expected = {
        "schema_version": SCHEMA_VERSION,
        "kind": "clone-b-hold-open-rollback-authorization",
        "run_id": context.run_id,
        "database": context.database,
        "reason": reason,
        "trigger": trigger,
        "ready_ledger_sha256": (
            ready[DOCUMENT_HASH_FIELD] if ready is not None else None
        ),
        "observed_evidence": observed,
        "attempted_components": components,
        "rollback_seed_artifacts": {
            component: [
                relative_file_identity(path, context.run_dir)
                for path in seed_paths[component]
                if path.is_file() and not path.is_symlink()
            ]
            for component in components
        },
        "rollback_steps": steps,
        "production_writes_executed": False,
    }
    if path.exists():
        value = read_hashed_json(path, "rollback authorization")
        if {k: v for k, v in value.items() if k != DOCUMENT_HASH_FIELD} != expected:
            raise ValueError("rollback authorization cannot be changed")
        return value
    return write_hashed_json(path, expected)


def validate_authorized_rollback_seeds(
    context: Context, authorization: dict[str, Any]
) -> None:
    components = authorization.get("attempted_components")
    inventory = authorization.get("rollback_seed_artifacts")
    if (
        not isinstance(components, list)
        or not isinstance(inventory, dict)
        or set(inventory) != set(components)
    ):
        raise ValueError("rollback seed authorization is invalid")
    for component in components:
        items = inventory.get(component)
        if not isinstance(items, list) or not items:
            raise ValueError(f"{component} rollback seed inventory is empty")
        for item in items:
            validate_relative_artifact(
                context.run_dir, item, f"{component} rollback seed"
            )


def validate_final_fingerprint(context: Context) -> dict[str, Any]:
    value = g4.read_object(
        context.run_dir / "rollback-fingerprint.json",
        "rollback fingerprint",
    )
    if (
        value.get("status") != "PASS"
        or value.get("violation_count") != 0
        or value.get("baseline_fingerprint_sha256")
        != value.get("rollback_fingerprint_sha256")
    ):
        raise ValueError("final rollback fingerprint is not exact")
    return value


def continue_rollback(
    context: Context,
    authorization: dict[str, Any],
    *,
    failure_exit_code: int,
) -> dict[str, Any]:
    validate_authorized_rollback_seeds(context, authorization)
    status_by_reason = {
        "observed-evidence-complete": "ROLLED_BACK",
        "observed-evidence-failed": "ROLLED_BACK_AFTER_OBSERVED_FAILURE",
        "apply-failure": "ROLLED_BACK_AFTER_APPLY_FAILURE",
        "apply-interrupted": "ROLLED_BACK_AFTER_APPLY_FAILURE",
    }
    reason = authorization.get("reason")
    if reason not in status_by_reason:
        raise ValueError("rollback authorization reason is invalid")
    status = status_by_reason[reason]
    complete_path = context.run_dir.joinpath(*ROLLBACK_COMPLETE_PATH.parts)
    if complete_path.exists():
        complete = read_hashed_json(complete_path, "rollback completion")
        rollback_steps = context.run_dir.joinpath(*ROLLBACK_STEPS_PATH.parts)
        if not rollback_steps.is_file() or rollback_steps.is_symlink():
            raise ValueError("rollback completion ledger is missing")
        fingerprint = validate_final_fingerprint(context)
        if (
            set(complete)
            != {
                "schema_version",
                "kind",
                "status",
                "run_id",
                "database",
                "terminal_reason",
                "trigger",
                "authorization_sha256",
                "rollback_steps_sha256",
                "rollback_fingerprint_sha256",
                "baseline_fingerprint_sha256",
                "production_writes_executed",
                DOCUMENT_HASH_FIELD,
            }
            or complete.get("schema_version") != SCHEMA_VERSION
            or complete.get("kind")
            != "clone-b-hold-open-rollback-complete"
            or complete.get("status") != status
            or complete.get("run_id") != context.run_id
            or complete.get("database") != context.database
            or complete.get("terminal_reason") != reason
            or complete.get("trigger") != authorization.get("trigger")
            or complete.get("authorization_sha256")
            != authorization[DOCUMENT_HASH_FIELD]
            or complete.get("rollback_steps_sha256")
            != g4.sha256_file(rollback_steps)
            or complete.get("rollback_fingerprint_sha256")
            != g4.sha256_file(context.run_dir / "rollback-fingerprint.json")
            or complete.get("baseline_fingerprint_sha256")
            != fingerprint["baseline_fingerprint_sha256"]
            or complete.get("production_writes_executed") is not False
        ):
            raise ValueError("rollback completion identity or hashes differ")
        return {
            "status": status,
            "exit_code": failure_exit_code,
            "terminal_receipt": ROLLBACK_COMPLETE_PATH.as_posix(),
            "rollback_complete_sha256": complete[DOCUMENT_HASH_FIELD],
        }
    steps = list(authorization.get("rollback_steps") or [])
    if not steps:
        return {
            "status": "BLOCKED",
            "exit_code": failure_exit_code or 1,
            "blocker": "no safe rollback/fingerprint step is authorized",
        }
    records, interrupted, prior_hash = load_phase_progress(
        context,
        "rollback",
        steps,
        initial_hash=str(authorization[DOCUMENT_HASH_FIELD]),
    )
    if interrupted is not None:
        raise ValueError(
            f"rollback step {interrupted} started without a completion "
            "checkpoint; refusing to repeat or advance"
        )
    failed_prior = next(
        (row for row in records if row.get("exit_code") != 0), None
    )
    if failed_prior is not None:
        return {
            "status": "BLOCKED",
            "exit_code": failure_exit_code or 1,
            "blocker": (
                "a completed rollback step previously failed; refusing to "
                f"advance past {failed_prior['step']}"
            ),
            "step_exit_code": failed_prior["exit_code"],
        }
    for ordinal in range(len(records) + 1, len(steps) + 1):
        step = steps[ordinal - 1]
        record, prior_hash = execute_checkpointed_step(
            context,
            phase="rollback",
            ordinal=ordinal,
            step=step,
            prior_hash=prior_hash,
        )
        records.append(record)
        if (
            record["exit_code"] != 0
            or record.get("process_group_quiescent") is not True
        ):
            return {
                "status": "BLOCKED",
                "exit_code": failure_exit_code or 1,
                "blocker": f"rollback step failed: {step}",
                "step_exit_code": record["exit_code"],
            }
    fingerprint = validate_final_fingerprint(context)
    rollback_steps = context.run_dir.joinpath(*ROLLBACK_STEPS_PATH.parts)
    if not rollback_steps.exists():
        write_jsonl(rollback_steps, records)
    rotate_commands(context, ROLLBACK_COMMANDS_PATH)
    complete = write_hashed_json(
        complete_path,
        {
            "schema_version": SCHEMA_VERSION,
            "kind": "clone-b-hold-open-rollback-complete",
            "status": status,
            "run_id": context.run_id,
            "database": context.database,
            "terminal_reason": authorization["reason"],
            "trigger": authorization.get("trigger"),
            "authorization_sha256": authorization[DOCUMENT_HASH_FIELD],
            "rollback_steps_sha256": g4.sha256_file(rollback_steps),
            "rollback_fingerprint_sha256": g4.sha256_file(
                context.run_dir / "rollback-fingerprint.json"
            ),
            "baseline_fingerprint_sha256": fingerprint[
                "baseline_fingerprint_sha256"
            ],
            "production_writes_executed": False,
        },
    )
    return {
        "status": status,
        "exit_code": failure_exit_code,
        "terminal_receipt": ROLLBACK_COMPLETE_PATH.as_posix(),
        "rollback_complete_sha256": complete[DOCUMENT_HASH_FIELD],
    }


def run_apply_and_hold(args: argparse.Namespace) -> dict[str, Any]:
    context = build_context(args, allow_create=True)
    ready_path = context.run_dir.joinpath(*READY_PATH.parts)
    if ready_path.exists():
        ready = validate_ready(context)
        return {
            "status": "HOLD_OPEN_READY",
            "exit_code": 0,
            "ready_ledger_sha256": ready[DOCUMENT_HASH_FIELD],
            "already_ready": True,
        }
    rollback_auth_path = context.run_dir.joinpath(*ROLLBACK_AUTH_PATH.parts)
    if rollback_auth_path.exists():
        authorization = read_hashed_json(
            rollback_auth_path, "rollback authorization"
        )
        if authorization.get("reason") not in {
            "apply-failure",
            "apply-interrupted",
        }:
            raise ValueError("apply phase cannot resume an observed rollback")
        return continue_rollback(context, authorization, failure_exit_code=1)
    records, interrupted, prior_hash = load_phase_progress(
        context, "apply", list(APPLY_SEQUENCE)
    )
    if interrupted is not None:
        if not args.confirm_interrupted_step_quiescent:
            raise ValueError(
                f"apply step {interrupted} started without a completion "
                "checkpoint; pass --confirm-interrupted-step-quiescent only "
                "after independently proving the old process group stopped"
            )
        rotate_commands(context, APPLY_COMMANDS_PATH, allow_empty=True)
        authorization = authorize_rollback(
            context,
            reason="apply-interrupted",
            ready=None,
            observed=[],
            trigger={
                "step": interrupted,
                "operator_confirmed_process_group_quiescent": True,
            },
        )
        return continue_rollback(context, authorization, failure_exit_code=1)
    failed_prior = next(
        (row for row in records if row.get("exit_code") != 0), None
    )
    if failed_prior is not None:
        rotate_commands(context, APPLY_COMMANDS_PATH)
        authorization = authorize_rollback(
            context,
            reason="apply-failure",
            ready=None,
            observed=[],
            trigger={
                "step": failed_prior["step"],
                "exit_code": failed_prior["exit_code"],
            },
        )
        return continue_rollback(context, authorization, failure_exit_code=1)
    if records and records[0].get("step") == "capture_baseline_fingerprint":
        verify_baseline_database(context)
    for ordinal in range(len(records) + 1, len(APPLY_SEQUENCE) + 1):
        step = APPLY_SEQUENCE[ordinal - 1]
        record, prior_hash = execute_checkpointed_step(
            context,
            phase="apply",
            ordinal=ordinal,
            step=step,
            prior_hash=prior_hash,
        )
        records.append(record)
        if record.get("process_group_quiescent") is not True:
            return {
                "status": "BLOCKED",
                "exit_code": 1,
                "blocker": f"process group is not quiescent after {step}",
            }
        if step == "capture_baseline_fingerprint" and record["exit_code"] == 0:
            verify_baseline_database(context)
        if record["exit_code"] != 0:
            rotate_commands(context, APPLY_COMMANDS_PATH)
            authorization = authorize_rollback(
                context,
                reason="apply-failure",
                ready=None,
                observed=[],
                trigger={
                    "step": record["step"],
                    "exit_code": record["exit_code"],
                },
            )
            return continue_rollback(
                context, authorization, failure_exit_code=1
            )
    ready = publish_ready(context, records)
    return {
        "status": "HOLD_OPEN_READY",
        "exit_code": 0,
        "ready_ledger_sha256": ready[DOCUMENT_HASH_FIELD],
        "already_ready": False,
    }


def run_resume_and_rollback(args: argparse.Namespace) -> dict[str, Any]:
    context = build_context(args, allow_create=False)
    ready = validate_ready(context)
    ready_sha = str(ready[DOCUMENT_HASH_FIELD])
    observed = [
        validate_observed_manifest(
            context, args.g5_evidence_manifest, "G5", ready_sha
        ),
        validate_observed_manifest(
            context, args.g6_evidence_manifest, "G6", ready_sha
        ),
    ]
    authorization = authorize_rollback(
        context,
        reason="observed-evidence-complete",
        ready=ready,
        observed=observed,
    )
    return continue_rollback(context, authorization, failure_exit_code=0)


def run_abort_and_rollback(args: argparse.Namespace) -> dict[str, Any]:
    context = build_context(args, allow_create=False)
    ready = validate_ready(context)
    ready_sha = str(ready[DOCUMENT_HASH_FIELD])
    g5_status = (
        "FAIL" if args.g6_evidence_manifest is None else "PASS"
    )
    observed = [
        validate_observed_manifest(
            context,
            args.g5_evidence_manifest,
            "G5",
            ready_sha,
            required_status=g5_status,
        )
    ]
    failed = observed[0]
    if args.g6_evidence_manifest is not None:
        g6 = validate_observed_manifest(
            context,
            args.g6_evidence_manifest,
            "G6",
            ready_sha,
            required_status="FAIL",
        )
        observed.append(g6)
        failed = g6
    authorization = authorize_rollback(
        context,
        reason="observed-evidence-failed",
        ready=ready,
        observed=observed,
        trigger={
            "gate": failed["gate"],
            "status": failed["status"],
            "violation_count": failed["violation_count"],
        },
    )
    return continue_rollback(context, authorization, failure_exit_code=1)


def run(args: argparse.Namespace) -> dict[str, Any]:
    if not args.execute_clone_writes:
        raise ValueError("--execute-clone-writes is required")
    if args.max_step_seconds <= 0:
        raise ValueError("--max-step-seconds must be positive")
    if args.phase == "apply-and-hold":
        return run_apply_and_hold(args)
    if args.phase == "resume-and-rollback":
        if args.g5_evidence_manifest is None or args.g6_evidence_manifest is None:
            raise ValueError(
                "resume-and-rollback requires explicit G5 and G6 evidence manifests"
            )
        return run_resume_and_rollback(args)
    if args.phase == "abort-and-rollback":
        if args.g5_evidence_manifest is None:
            raise ValueError(
                "abort-and-rollback requires an explicit G5 evidence manifest"
            )
        return run_abort_and_rollback(args)
    raise ValueError("unknown coordinator phase")


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--phase",
        choices=(
            "apply-and-hold",
            "resume-and-rollback",
            "abort-and-rollback",
        ),
        required=True,
    )
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--run-dir", type=pathlib.Path, required=True)
    parser.add_argument("--clone-root", type=pathlib.Path, required=True)
    parser.add_argument("--confirm-clone-database", required=True)
    parser.add_argument("--dsn-file", type=pathlib.Path, required=True)
    parser.add_argument("--mapping-file", type=pathlib.Path, required=True)
    parser.add_argument(
        "--recovery-evidence-file", type=pathlib.Path, required=True
    )
    parser.add_argument(
        "--recovery-source-root", type=pathlib.Path, required=True
    )
    parser.add_argument("--command-plan", type=pathlib.Path, required=True)
    parser.add_argument("--auth-settings-file", type=pathlib.Path, required=True)
    parser.add_argument(
        "--expected-baseline-file", type=pathlib.Path, required=True
    )
    parser.add_argument("--g5-evidence-manifest", type=pathlib.Path)
    parser.add_argument("--g6-evidence-manifest", type=pathlib.Path)
    parser.add_argument("--execute-clone-writes", action="store_true")
    parser.add_argument(
        "--confirm-interrupted-step-quiescent", action="store_true"
    )
    parser.add_argument("--max-step-seconds", type=float, default=600)
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    try:
        report = run(parse_args(argv))
        print(json.dumps(report, ensure_ascii=False, sort_keys=True))
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
