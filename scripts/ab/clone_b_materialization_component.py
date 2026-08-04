#!/usr/bin/env python3
"""Execute one hash-bound recovery or bundle component in local Clone B.

The file-only materializers and the Go database executors intentionally have
different responsibilities.  This wrapper joins them without weakening either
boundary:

* MYSQL_DSN is accepted only through ``g4_clone_db.Connection`` and is never
  placed in an argv or evidence document.
* both database executors receive the DSN through their private environment;
* the non-migrated guard table's exact prior DDL and rows are frozen before it
  is provisioned;
* apply proves an initial database change and an idempotent second apply;
* rollback restores the database first, then the guard, then fixture bytes.

The wrapper is deliberately restricted to the existing task-2807 recovery and
seven reviewed source-bundle executors.  It is not a general migration shell.
"""

from __future__ import annotations

import argparse
import dataclasses
import hashlib
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
from typing import Any

try:
    from scripts.ab import g4_clone_db as clone_db
except ModuleNotFoundError:
    import g4_clone_db as clone_db


SHA256 = re.compile(r"^[0-9a-f]{64}$")
RUN_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{2,80}$")


@dataclasses.dataclass(frozen=True)
class GuardSpec:
    component: str
    table: str
    columns: tuple[tuple[str, str], ...]
    create_sql: str


RECOVERY_GUARD = GuardSpec(
    component="recovery",
    table="v8_ab_clone_guard",
    columns=(
        ("singleton_id", "tinyint"),
        ("environment", "varchar(32)"),
        ("run_id", "varchar(81)"),
        ("plan_sha256", "char(64)"),
    ),
    create_sql=(
        "CREATE TABLE `v8_ab_clone_guard` ("
        "`singleton_id` TINYINT NOT NULL,"
        "`environment` VARCHAR(32) NOT NULL,"
        "`run_id` VARCHAR(81) NOT NULL,"
        "`plan_sha256` CHAR(64) NOT NULL,"
        "PRIMARY KEY (`singleton_id`)"
        ") ENGINE=InnoDB"
    ),
)

BUNDLE_GUARD = GuardSpec(
    component="bundle",
    table="v8_ab_source_bundle_guard",
    columns=(
        ("singleton_id", "tinyint"),
        ("environment", "varchar(32)"),
        ("run_id", "varchar(128)"),
        ("candidate_sha256", "char(64)"),
        ("registry_sha256", "char(64)"),
    ),
    create_sql=(
        "CREATE TABLE `v8_ab_source_bundle_guard` ("
        "`singleton_id` TINYINT NOT NULL,"
        "`environment` VARCHAR(32) NOT NULL,"
        "`run_id` VARCHAR(128) NOT NULL,"
        "`candidate_sha256` CHAR(64) NOT NULL,"
        "`registry_sha256` CHAR(64) NOT NULL,"
        "PRIMARY KEY (`singleton_id`)"
        ") ENGINE=InnoDB"
    ),
)


class ComponentError(RuntimeError):
    pass


def canonical_bytes(value: Any) -> bytes:
    return clone_db.canonical_bytes(value)


def sha256_file(path: pathlib.Path) -> str:
    return clone_db.sha256_file(path)


def artifact(path: pathlib.Path) -> dict[str, Any]:
    if path.is_symlink() or not path.is_file():
        raise ComponentError(f"artifact is missing or symlinked: {path}")
    return {
        "path": path.name,
        "sha256": sha256_file(path),
        "size": path.stat().st_size,
    }


def read_object(path: pathlib.Path, label: str) -> dict[str, Any]:
    if path.is_symlink() or not path.is_file():
        raise ComponentError(f"{label} must be an existing non-symlink file")
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise ComponentError(f"{label} is unreadable: {exc}") from exc
    if not isinstance(value, dict):
        raise ComponentError(f"{label} must be a JSON object")
    return value


def write_document(path: pathlib.Path, value: dict[str, Any]) -> None:
    clone_db.atomic_write(path, canonical_bytes(value))


def self_bound(value: dict[str, Any]) -> dict[str, Any]:
    payload = dict(value)
    payload["evidence_sha256"] = hashlib.sha256(
        canonical_bytes(payload)
    ).hexdigest()
    return payload


def verify_self_bound(value: dict[str, Any], label: str) -> None:
    expected = str(value.get("evidence_sha256") or "")
    unhashed = dict(value)
    unhashed.pop("evidence_sha256", None)
    if (
        not SHA256.fullmatch(expected)
        or hashlib.sha256(canonical_bytes(unhashed)).hexdigest() != expected
    ):
        raise ComponentError(f"{label} self hash is missing or stale")


def verify_compact_self_bound(value: dict[str, Any], label: str) -> None:
    expected = str(value.get("evidence_sha256") or "")
    unhashed = dict(value)
    unhashed.pop("evidence_sha256", None)
    actual = hashlib.sha256(
        json.dumps(
            unhashed,
            ensure_ascii=False,
            sort_keys=True,
            separators=(",", ":"),
        ).encode("utf-8")
    ).hexdigest()
    if not SHA256.fullmatch(expected) or actual != expected:
        raise ComponentError(f"{label} self hash is missing or stale")


def _parse_json_lines(raw: str) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for line_number, line in enumerate(raw.splitlines(), 1):
        if not line.strip():
            continue
        try:
            value = json.loads(line)
        except json.JSONDecodeError as exc:
            raise ComponentError(
                f"guard query returned invalid JSON at line {line_number}"
            ) from exc
        if not isinstance(value, dict):
            raise ComponentError("guard query returned a non-object row")
        rows.append(value)
    return rows


def capture_guard_state(
    connection: clone_db.Connection, spec: GuardSpec
) -> dict[str, Any]:
    exists_raw = connection.execute(
        "SELECT COUNT(*) FROM information_schema.tables "
        "WHERE table_schema=DATABASE() "
        f"AND table_name='{spec.table}' AND table_type='BASE TABLE';\n"
    ).strip()
    if exists_raw not in {"0", "1"}:
        raise ComponentError("guard table existence query was not scalar")
    exists = exists_raw == "1"
    create_sql: str | None = None
    schema: list[dict[str, Any]] = []
    rows: list[dict[str, Any]] = []
    if exists:
        discovered = clone_db.discover_schema(connection, [spec.table])
        schema = discovered[spec.table]
        actual_columns = tuple(
            (str(row["name"]), str(row["column_type"]).lower())
            for row in schema
        )
        if actual_columns != spec.columns:
            raise ComponentError(
                f"{spec.table} has an unexpected column contract"
            )
        primary = connection.execute(
            "SELECT COUNT(*) FROM information_schema.statistics "
            "WHERE table_schema=DATABASE() "
            f"AND table_name='{spec.table}' AND index_name='PRIMARY' "
            "AND seq_in_index=1 AND column_name='singleton_id' "
            "AND non_unique=0;\n"
        ).strip()
        if primary != "1":
            raise ComponentError(f"{spec.table} primary key contract drifted")
        shown = connection.execute(f"SHOW CREATE TABLE `{spec.table}`;\n")
        parts = shown.rstrip("\r\n").split("\t", 1)
        if len(parts) != 2 or parts[0] != spec.table:
            raise ComponentError("SHOW CREATE TABLE output is invalid")
        create_sql = parts[1]
        json_fields = ",".join(
            f"'{name}',`{name}`" for name, _ in spec.columns
        )
        rows = _parse_json_lines(
            connection.execute(
                f"SELECT JSON_OBJECT({json_fields}) "
                f"FROM `{spec.table}` ORDER BY `singleton_id`;\n"
            )
        )
        if len(rows) > 1 or any(
            row.get("singleton_id") != 1
            or set(row) != {name for name, _ in spec.columns}
            or any(
                not isinstance(row[name], str)
                or not re.fullmatch(r"[A-Za-z0-9._-]+", row[name])
                for name, _ in spec.columns
                if name != "singleton_id"
            )
            for row in rows
        ):
            raise ComponentError(
                f"{spec.table} contains unexpected non-singleton guard rows"
            )
    payload = {
        "schema_version": 1,
        "kind": "clone-b-guard-state",
        "component": spec.component,
        "database": connection.database,
        "table": spec.table,
        "table_existed": exists,
        "create_table_sql": create_sql,
        "schema": schema,
        "rows": rows,
    }
    return self_bound(payload)


def _safe_value(value: str, label: str) -> str:
    if label == "run_id":
        valid = RUN_ID.fullmatch(value)
    else:
        valid = SHA256.fullmatch(value)
    if not valid:
        raise ComponentError(f"{label} is invalid")
    return value


def guard_binding(
    spec: GuardSpec,
    *,
    run_id: str,
    primary_sha256: str,
    secondary_sha256: str | None = None,
) -> dict[str, Any]:
    _safe_value(run_id, "run_id")
    _safe_value(primary_sha256, "sha256")
    if spec is RECOVERY_GUARD:
        if secondary_sha256 is not None:
            raise ComponentError("recovery guard accepts one hash")
        return {
            "singleton_id": 1,
            "environment": "clone_b",
            "run_id": run_id,
            "plan_sha256": primary_sha256,
        }
    if secondary_sha256 is None:
        raise ComponentError("bundle guard requires candidate and registry hashes")
    _safe_value(secondary_sha256, "sha256")
    return {
        "singleton_id": 1,
        "environment": "clone_b",
        "run_id": run_id,
        "candidate_sha256": primary_sha256,
        "registry_sha256": secondary_sha256,
    }


def expected_provisioned_state(
    before: dict[str, Any], binding: dict[str, Any]
) -> list[dict[str, Any]]:
    rows = [
        dict(row)
        for row in before.get("rows", [])
        if row.get("singleton_id") != 1
    ]
    rows.append(dict(binding))
    return sorted(rows, key=lambda row: int(row["singleton_id"]))


def provision_guard(
    connection: clone_db.Connection,
    spec: GuardSpec,
    binding: dict[str, Any],
    before_path: pathlib.Path,
    provision_path: pathlib.Path,
) -> dict[str, Any]:
    before = capture_guard_state(connection, spec)
    write_document(before_path, before)
    try:
        if not before["table_existed"]:
            connection.execute(spec.create_sql + ";\n")
        columns = [name for name, _ in spec.columns]
        values = []
        for name in columns:
            value = binding[name]
            if isinstance(value, int) and not isinstance(value, bool):
                values.append(str(value))
            elif isinstance(value, str) and re.fullmatch(
                r"[A-Za-z0-9._-]+", value
            ):
                values.append("'" + value + "'")
            else:
                raise ComponentError(f"guard binding {name} is unsafe")
        connection.execute(
            "START TRANSACTION;\n"
            f"DELETE FROM `{spec.table}` WHERE `singleton_id`=1;\n"
            f"INSERT INTO `{spec.table}` ("
            + ",".join(f"`{name}`" for name in columns)
            + ") VALUES ("
            + ",".join(values)
            + ");\nCOMMIT;\n"
        )
        after = capture_guard_state(connection, spec)
        if (
            not after["table_existed"]
            or after["rows"] != expected_provisioned_state(before, binding)
        ):
            raise ComponentError(
                "guard did not reach the exact provisioned state"
            )
        receipt = self_bound(
            {
                "schema_version": 1,
                "status": "PROVISIONED",
                "component": spec.component,
                "database": connection.database,
                "table": spec.table,
                "before_artifact_sha256": sha256_file(before_path),
                "before_state_sha256": before["evidence_sha256"],
                "binding": binding,
                "after_state_sha256": after["evidence_sha256"],
            }
        )
        write_document(provision_path, receipt)
        return before
    except Exception as original:
        try:
            _restore_guard_to_before(connection, spec, before)
        except Exception as compensation:
            raise ComponentError(
                "guard provision failed and exact compensation failed: "
                f"{original}; compensation={compensation}"
            ) from original
        raise ComponentError(
            f"guard provision failed and was compensated: {original}"
        ) from original


def _row_insert_sql(spec: GuardSpec, row: dict[str, Any]) -> str:
    values: list[str] = []
    for name, _ in spec.columns:
        value = row[name]
        if isinstance(value, int) and not isinstance(value, bool):
            values.append(str(value))
        elif isinstance(value, str) and re.fullmatch(
            r"[A-Za-z0-9._-]+", value
        ):
            values.append("'" + value + "'")
        else:
            raise ComponentError(f"guard before value {name} is unsafe")
    return (
        f"INSERT INTO `{spec.table}` ("
        + ",".join(f"`{name}`" for name, _ in spec.columns)
        + ") VALUES ("
        + ",".join(values)
        + ");\n"
    )


def _restore_guard_to_before(
    connection: clone_db.Connection,
    spec: GuardSpec,
    before: dict[str, Any],
) -> None:
    if before["table_existed"]:
        current = capture_guard_state(connection, spec)
        if not current["table_existed"]:
            raise ComponentError("pre-existing guard table disappeared")
        original = [
            row for row in before["rows"] if row.get("singleton_id") == 1
        ]
        connection.execute(
            "START TRANSACTION;\n"
            f"DELETE FROM `{spec.table}` WHERE `singleton_id`=1;\n"
            + (_row_insert_sql(spec, original[0]) if original else "")
            + "COMMIT;\n"
        )
    else:
        exists_raw = connection.execute(
            "SELECT COUNT(*) FROM information_schema.tables "
            "WHERE table_schema=DATABASE() "
            f"AND table_name='{spec.table}' AND table_type='BASE TABLE';\n"
        ).strip()
        if exists_raw == "1":
            connection.execute(f"DROP TABLE `{spec.table}`;\n")
        elif exists_raw != "0":
            raise ComponentError("guard existence query was not scalar")
    restored = capture_guard_state(connection, spec)
    restored_without_hash = dict(restored)
    restored_without_hash.pop("evidence_sha256", None)
    before_without_hash = dict(before)
    before_without_hash.pop("evidence_sha256", None)
    if restored_without_hash != before_without_hash:
        raise ComponentError("guard compensation is not exact")


def restore_guard(
    connection: clone_db.Connection,
    spec: GuardSpec,
    binding: dict[str, Any],
    before_path: pathlib.Path,
    restore_path: pathlib.Path,
) -> dict[str, Any]:
    before = read_object(before_path, "guard-before")
    verify_self_bound(before, "guard-before")
    if (
        before.get("component") != spec.component
        or before.get("database") != connection.database
        or before.get("table") != spec.table
    ):
        raise ComponentError("guard-before belongs to another component/database")
    current = capture_guard_state(connection, spec)
    current_without_hash = dict(current)
    current_without_hash.pop("evidence_sha256", None)
    before_without_hash = dict(before)
    before_without_hash.pop("evidence_sha256", None)
    already_restored = current_without_hash == before_without_hash
    if not already_restored:
        if (
            not current["table_existed"]
            or current["rows"] != expected_provisioned_state(before, binding)
        ):
            raise ComponentError("guard state drifted before restoration")
        _restore_guard_to_before(connection, spec, before)
        restored = capture_guard_state(connection, spec)
    else:
        restored = current
    receipt = self_bound(
        {
            "schema_version": 1,
            "status": "RESTORED",
            "component": spec.component,
            "database": connection.database,
            "table": spec.table,
            "before_artifact_sha256": sha256_file(before_path),
            "restored_state_sha256": restored["evidence_sha256"],
            "exact": True,
            "already_restored": already_restored,
        }
    )
    write_document(restore_path, receipt)
    return receipt


def run_command(
    argv: list[str],
    *,
    repo_root: pathlib.Path,
    env: dict[str, str],
    label: str,
) -> None:
    completed = subprocess.run(
        argv,
        cwd=repo_root,
        env=env,
        text=True,
        encoding="utf-8",
        capture_output=True,
        check=False,
    )
    if completed.returncode != 0:
        secret = os.environ.get("MYSQL_DSN", "")
        detail = (completed.stderr or completed.stdout).strip()
        if secret:
            detail = detail.replace(secret, "[REDACTED_DSN]")
        raise ComponentError(
            f"{label} failed with exit {completed.returncode}: {detail}"
        )


def go_env() -> dict[str, str]:
    raw = os.environ.get("MYSQL_DSN", "")
    if not raw:
        raise ComponentError("MYSQL_DSN is required")
    env = dict(os.environ)
    env["CLONE_B_MYSQL_DSN"] = raw
    return env


def validate_root(args: argparse.Namespace) -> tuple[pathlib.Path, pathlib.Path]:
    if not RUN_ID.fullmatch(args.run_id):
        raise ComponentError("--run-id is invalid")
    run_root = args.run_root
    component_dir = args.component_dir
    fixture_root = args.fixture_root
    if (
        not run_root.is_absolute()
        or not run_root.is_dir()
        or run_root.is_symlink()
        or run_root.name != args.run_id
    ):
        raise ComponentError("--run-root must be an exact run-id directory")
    run_root = run_root.resolve()
    if (
        not component_dir.is_absolute()
        or not component_dir.is_dir()
        or component_dir.is_symlink()
    ):
        raise ComponentError("--component-dir must be an existing directory")
    component_dir = component_dir.resolve()
    try:
        component_dir.relative_to(run_root)
    except ValueError:
        raise ComponentError("--component-dir must be below --run-root") from None
    if (
        not fixture_root.is_absolute()
        or not fixture_root.is_dir()
        or fixture_root.is_symlink()
        or fixture_root.resolve() != run_root / "fixture-upload-b"
    ):
        raise ComponentError(
            "--fixture-root must be <run-root>/fixture-upload-b"
        )
    return run_root, component_dir


def validate_recovery_report(
    path: pathlib.Path,
    *,
    mode: str,
    run_id: str,
    database: str,
    host: str,
    plan_sha256: str,
    changed: int,
    already: int,
    allowed_counts: set[tuple[int, int]] | None = None,
) -> dict[str, Any]:
    value = read_object(path, f"recovery-db-{mode}")
    counts = (
        value.get("changed_entries"),
        value.get("already_in_target_state_entries"),
    )
    expected_counts = allowed_counts or {(changed, already)}
    if (
        value.get("version") != 1
        or value.get("mode") != mode
        or value.get("run_id") != run_id
        or value.get("database") != database
        or value.get("host") != host
        or value.get("plan_sha256") != plan_sha256
        or counts not in expected_counts
        or value.get("database_transaction_committed") is not True
        or value.get("object_storage_writes_executed") is not False
    ):
        raise ComponentError(f"recovery {mode} report contract failed")
    return value


def validate_bundle_report(
    path: pathlib.Path,
    *,
    mode: str,
    run_id: str,
    database: str,
    host: str,
    candidate_sha256: str,
    registry_sha256: str,
    manifest_sha256: str,
    rollback_journal_sha256: str,
    rollback_journal_evidence_sha256: str,
    changed: int,
    already: int,
    allowed_counts: set[tuple[int, int]] | None = None,
) -> dict[str, Any]:
    value = read_object(path, f"bundle-db-{mode}")
    counts = (
        value.get("changed_bundle_count"),
        value.get("already_applied_bundle_count"),
    )
    expected_counts = allowed_counts or {(changed, already)}
    if (
        value.get("schema_version") != 1
        or value.get("mode") != mode
        or value.get("status") != "PASS"
        or value.get("run_id") != run_id
        or value.get("database") != database
        or value.get("host") != host
        or value.get("candidate_sha256") != candidate_sha256
        or value.get("registry_sha256") != registry_sha256
        or value.get("manifest_sha256") != manifest_sha256
        or value.get("rollback_journal_sha256")
        != rollback_journal_sha256
        or value.get("rollback_journal_evidence_sha256")
        != rollback_journal_evidence_sha256
        or counts not in expected_counts
        or value.get("database_transaction_committed") is not True
    ):
        raise ComponentError(f"bundle {mode} report contract failed")
    return value


def validate_bundle_journal(
    path: pathlib.Path,
    *,
    manifest: dict[str, Any],
    run_id: str,
    database: str,
    host: str,
    candidate_sha256: str,
    registry_sha256: str,
    manifest_sha256: str,
) -> dict[str, Any]:
    value = read_object(path, "bundle rollback journal")
    verify_compact_self_bound(value, "bundle rollback journal")
    members = [
        member
        for bundle in manifest.get("bundles") or []
        if isinstance(bundle, dict)
        for member in bundle.get("ordered_members") or []
        if isinstance(member, dict)
    ]
    before = value.get("member_before")
    auto_before = value.get("auto_increment_before")
    auto_ceilings = value.get("auto_increment_ceilings")
    expected = {
        int(member["task_asset_id"]): str(member["sha256"])
        for member in members
    }
    if (
        value.get("schema_version") != 1
        or value.get("kind")
        != "source-bundle-clone-b-rollback-journal"
        or value.get("status") != "PREPARED"
        or value.get("run_id") != run_id
        or value.get("database") != database
        or value.get("host") != host
        or value.get("candidate_sha256") != candidate_sha256
        or value.get("registry_sha256") != registry_sha256
        or value.get("manifest_sha256") != manifest_sha256
        or value.get("prepared_before_first_database_mutation") is not True
        or value.get("database_commit_state") != "unknown"
        or value.get("expected_bundle_count") != 7
        or value.get("expected_member_count") != 22
        or value.get("changed_bundle_count") != 7
        or value.get("already_applied_bundle_count") != 0
        or value.get("production_writes_executed") is not False
        or not isinstance(before, list)
        or len(before) != 22
        or len(expected) != 22
        or not isinstance(auto_before, list)
        or not isinstance(auto_ceilings, list)
        or [item.get("table") for item in auto_before]
        != ["design_assets", "task_assets"]
        or [item.get("table") for item in auto_ceilings]
        != ["design_assets", "task_assets"]
    ):
        raise ComponentError("bundle rollback journal contract failed")
    for before_state, ceiling in zip(auto_before, auto_ceilings):
        before_value = before_state.get("next_value")
        ceiling_value = ceiling.get("next_value")
        if (
            isinstance(before_value, bool)
            or not isinstance(before_value, int)
            or before_value <= 0
            or isinstance(ceiling_value, bool)
            or not isinstance(ceiling_value, int)
            or ceiling_value < before_value
        ):
            raise ComponentError(
                "bundle rollback journal auto-increment state is invalid"
            )
    ids = [item.get("task_asset_id") for item in before if isinstance(item, dict)]
    if ids != sorted(expected) or len(ids) != len(set(ids)):
        raise ComponentError("bundle rollback journal member order is invalid")
    for item in before:
        if (
            not isinstance(item, dict)
            or item.get("recovered_whole_hash")
            != expected.get(item.get("task_asset_id"))
            or item.get("original_whole_hash") not in {None, ""}
        ):
            raise ComponentError(
                "bundle rollback journal before-images are invalid"
            )
    return value


def go_recovery_argv(
    *,
    mode: str,
    plan: pathlib.Path,
    fixture_root: pathlib.Path,
    report: pathlib.Path,
    connection: clone_db.Connection,
    run_id: str,
) -> list[str]:
    return [
        "go",
        "run",
        "./cmd/tools/asset-recovery-clone-b",
        "--mode",
        mode,
        "--plan",
        str(plan),
        "--fixture-root",
        str(fixture_root),
        "--report-file",
        str(report),
        "--confirm-database",
        connection.database,
        "--confirm-host",
        connection.host,
        "--confirm-run-id",
        run_id,
    ]


def go_bundle_argv(
    *,
    mode: str,
    registry: pathlib.Path,
    manifest: pathlib.Path,
    fixture_root: pathlib.Path,
    report: pathlib.Path,
    connection: clone_db.Connection,
    run_id: str,
    candidate_sha256: str,
    rollback_journal: pathlib.Path,
    apply_report: pathlib.Path | None = None,
) -> list[str]:
    argv = [
        "go",
        "run",
        "./cmd/tools/source-bundle-clone-b",
        "--mode",
        mode,
        "--registry",
        str(registry),
        "--manifest",
        str(manifest),
        "--fixture-root",
        str(fixture_root),
        "--report-file",
        str(report),
        "--rollback-journal",
        str(rollback_journal),
        "--confirm-database",
        connection.database,
        "--confirm-host",
        connection.host,
        "--confirm-run-id",
        run_id,
        "--confirm-candidate-sha256",
        candidate_sha256,
    ]
    if apply_report is not None:
        argv.extend(["--apply-report", str(apply_report)])
    return argv


def component_report(
    *,
    component: str,
    action: str,
    args: argparse.Namespace,
    connection: clone_db.Connection,
    files: list[pathlib.Path],
    database_writes_executed: bool = True,
) -> dict[str, Any]:
    return self_bound(
        {
            "schema_version": 1,
            "status": "APPLIED" if action == "apply" else "ROLLED_BACK",
            "component": component,
            "action": action,
            "run_id": args.run_id,
            "database": connection.database,
            "host": connection.host,
            "database_writes_executed": database_writes_executed,
            "production_writes_executed": False,
            "guard_retained_for_rollback": action == "apply",
            "guard_exactly_restored": action == "rollback",
            "ownership_receipt_contract_version": 1,
            "artifacts": [artifact(path) for path in files],
        }
    )


def ownership_receipt_artifacts(
    component_dir: pathlib.Path,
    component: str,
    *,
    require_complete: bool,
) -> list[pathlib.Path]:
    if component == "recovery":
        expected = {
            *{
                f"recovery-ownership-{asset_id}.json"
                for asset_id in (23989, 23990, 23991)
            },
            *{
                f"recovery-staging-ownership-{asset_id}.json"
                for asset_id in (23989, 23990, 23991)
            },
        }
        patterns = (
            "recovery-ownership-*.json",
            "recovery-staging-ownership-*.json",
        )
    elif component == "bundle":
        expected = {
            *{
                f"bundle-ownership-{asset_id}.json"
                for asset_id in range(25557, 25564)
            },
            *{
                f"bundle-staging-ownership-{asset_id}.json"
                for asset_id in range(25557, 25564)
            },
        }
        patterns = (
            "bundle-ownership-*.json",
            "bundle-staging-ownership-*.json",
        )
    else:
        raise ComponentError("ownership receipt component is invalid")
    paths = sorted(
        {
            path
            for pattern in patterns
            for path in component_dir.glob(pattern)
            if path.is_file() and not path.is_symlink()
        },
        key=lambda path: path.name,
    )
    if require_complete and {path.name for path in paths} != expected:
        raise ComponentError(
            f"{component} ownership receipt artifact set differs"
        )
    return paths


def validate_bundle_registry_for_apply(
    value: dict[str, Any],
    *,
    registry: pathlib.Path,
    write_ahead: pathlib.Path,
    component_dir: pathlib.Path,
    run_id: str,
) -> list[pathlib.Path]:
    verify_self_bound(value, "bundle registry")
    if (
        value.get("schema_version") != 1
        or value.get("status") != "MATERIALIZED"
        or value.get("run_id") != run_id
        or value.get("database_write_performed") is not False
        or value.get("write_ahead_sha256") != sha256_file(write_ahead)
    ):
        raise ComponentError("bundle registry apply contract differs")
    if not registry.is_file() or registry.is_symlink():
        raise ComponentError("bundle registry must be a regular non-symlink file")
    entries = value.get("entries")
    if not isinstance(entries, list) or len(entries) != 7:
        raise ComponentError("bundle registry entries differ")
    b_root = pathlib.Path(str(value.get("b_root") or ""))
    if (
        not b_root.is_absolute()
        or not b_root.is_dir()
        or b_root.is_symlink()
    ):
        raise ComponentError("bundle registry B root is invalid")
    resolved_b_root = b_root.resolve()
    expected_receipt_names: set[str] = set()
    entry_by_asset_id: dict[int, dict[str, Any]] = {}
    for entry in entries:
        if not isinstance(entry, dict):
            raise ComponentError("bundle registry entry is invalid")
        disposition = entry.get("disposition")
        rollback = entry.get("rollback_candidate")
        candidate = entry.get("task_asset_candidate")
        if not isinstance(rollback, dict) or not isinstance(candidate, dict):
            raise ComponentError("bundle rollback candidate is invalid")
        asset_id = candidate.get("id")
        if (
            not isinstance(asset_id, int)
            or rollback.get("task_asset_id") != asset_id
            or asset_id in entry_by_asset_id
        ):
            raise ComponentError("bundle rollback task asset differs")
        entry_by_asset_id[asset_id] = entry
        expected_receipt_names.add(
            f"bundle-staging-ownership-{asset_id}.json"
        )
        expected_path = (
            component_dir / f"bundle-ownership-{asset_id}.json"
        ).resolve()
        recorded_path = str(rollback.get("ownership_receipt_path") or "")
        if disposition == "created":
            expected_receipt_names.add(expected_path.name)
            if recorded_path != str(expected_path):
                raise ComponentError(
                    "created bundle ownership receipt path differs"
                )
        elif disposition == "reused_identical":
            if recorded_path and recorded_path != str(expected_path):
                raise ComponentError(
                    "reused bundle ownership receipt path differs"
                )
        else:
            raise ComponentError("bundle disposition is invalid")
    receipts = sorted(
        {
            path
            for pattern in (
                "bundle-ownership-*.json",
                "bundle-staging-ownership-*.json",
            )
            for path in component_dir.glob(pattern)
            if path.is_file() and not path.is_symlink()
        },
        key=lambda path: path.name,
    )
    if {path.name for path in receipts} != expected_receipt_names:
        raise ComponentError("bundle ownership receipt artifact set differs")
    receipt_by_name: dict[str, tuple[pathlib.Path, dict[str, Any]]] = {}
    for path in receipts:
        receipt = read_object(path, f"bundle receipt {path.name}")
        verify_self_bound(receipt, f"bundle receipt {path.name}")
        if (
            receipt.get("schema_version") != 1
            or receipt.get("run_id") != run_id
        ):
            raise ComponentError(f"bundle receipt {path.name} differs")
        receipt_by_name[path.name] = (path, receipt)
    for asset_id, entry in entry_by_asset_id.items():
        relative = pathlib.PurePosixPath(
            str(entry.get("relative_object_path") or "")
        )
        if (
            relative.is_absolute()
            or not relative.parts
            or any(part in {"", ".", ".."} for part in relative.parts)
        ):
            raise ComponentError("bundle target path is invalid")
        target_candidate = resolved_b_root.joinpath(*relative.parts)
        if target_candidate.is_symlink():
            raise ComponentError("bundle target path is invalid")
        target = target_candidate.resolve()
        try:
            target.relative_to(resolved_b_root)
        except ValueError:
            raise ComponentError("bundle target escapes B root") from None
        if (
            not target.is_file()
            or target.is_symlink()
            or sha256_file(target) != entry.get("bundle_sha256")
            or target.stat().st_size != entry.get("size")
        ):
            raise ComponentError("bundle target identity differs")
        staging_name = f"bundle-staging-ownership-{asset_id}.json"
        staging_pair = receipt_by_name.get(staging_name)
        if staging_pair is None:
            raise ComponentError("bundle staging ownership receipt is missing")
        staging = staging_pair[1]
        expected_stage = (
            component_dir / f".bundle-stage-{asset_id}.zip"
        ).resolve()
        expected_private = (
            component_dir / f".bundle-private-{asset_id}.zip"
        ).resolve()
        if (
            staging.get("status") != "STAGING_OWNED"
            or staging.get("staging_path") != str(expected_stage)
            or staging.get("private_path") != str(expected_private)
            or staging.get("sha256") != entry.get("bundle_sha256")
            or staging.get("size") != entry.get("size")
        ):
            raise ComponentError(
                "bundle staging ownership receipt identity differs"
            )
        if entry.get("disposition") != "created":
            continue
        ownership_name = f"bundle-ownership-{asset_id}.json"
        pair = receipt_by_name.get(ownership_name)
        if pair is None or pair[0].resolve() != (
            component_dir / ownership_name
        ).resolve():
            raise ComponentError("created bundle ownership receipt is missing")
        receipt = pair[1]
        stat = target.stat()
        if (
            receipt.get("status") != "OWNED_LINK"
            or receipt.get("target_path") != str(target)
            or receipt.get("staging_path") != str(expected_stage)
            or receipt.get("device") != stat.st_dev
            or receipt.get("inode") != stat.st_ino
            or receipt.get("sha256") != entry.get("bundle_sha256")
            or receipt.get("size") != stat.st_size
        ):
            raise ComponentError(
                "created bundle ownership receipt identity differs"
            )
    return receipts


def recovery_apply(
    args: argparse.Namespace,
    connection: clone_db.Connection,
    repo_root: pathlib.Path,
    component_dir: pathlib.Path,
) -> dict[str, Any]:
    plan = component_dir / "recovery-materialization-plan.json"
    write_ahead = component_dir / "recovery-file-write-ahead.json"
    guard_before = component_dir / "recovery-guard-before.json"
    guard_provision = component_dir / "recovery-guard-provision.json"
    db_apply = component_dir / "recovery-db-apply.json"
    db_idempotent = component_dir / "recovery-db-idempotent.json"
    report = component_dir / "recovery-component-apply.json"
    cleanup = component_dir / "recovery-apply-compensation-files.json"
    guard_restore = component_dir / "recovery-apply-compensation-guard.json"
    db_rollback = component_dir / "recovery-apply-compensation-db.json"
    materialized = False
    guard_ready = False
    binding: dict[str, Any] | None = None
    try:
        run_command(
            [
                sys.executable,
                str(repo_root / "scripts/ab/prepare_asset_recovery.py"),
                "--mapping",
                str(args.mapping),
                "--evidence",
                str(args.evidence),
                "--output",
                str(write_ahead),
                "--fixture-root",
                str(args.fixture_root),
            ],
            repo_root=repo_root,
            env=dict(os.environ),
            label="recovery file write-ahead plan",
        )
        run_command(
            [
                sys.executable,
                str(repo_root / "scripts/ab/prepare_asset_recovery.py"),
                "--mapping",
                str(args.mapping),
                "--evidence",
                str(args.evidence),
                "--output",
                str(plan),
                "--materialize",
                "--fixture-root",
                str(args.fixture_root),
                "--expected-write-ahead",
                str(write_ahead),
            ],
            repo_root=repo_root,
            env=dict(os.environ),
            label="recovery file materialization",
        )
        materialized = True
        plan_value = read_object(plan, "recovery-plan")
        verify_compact_self_bound(plan_value, "recovery-plan")
        if plan_value.get("run_id") != args.run_id:
            raise ComponentError("recovery plan run_id differs")
        plan_sha = sha256_file(plan)
        binding = guard_binding(
            RECOVERY_GUARD,
            run_id=args.run_id,
            primary_sha256=plan_sha,
        )
        provision_guard(
            connection,
            RECOVERY_GUARD,
            binding,
            guard_before,
            guard_provision,
        )
        guard_ready = True
        env = go_env()
        run_command(
            go_recovery_argv(
                mode="apply",
                plan=plan,
                fixture_root=args.fixture_root,
                report=db_apply,
                connection=connection,
                run_id=args.run_id,
            ),
            repo_root=repo_root,
            env=env,
            label="recovery database apply",
        )
        validate_recovery_report(
            db_apply,
            mode="apply",
            run_id=args.run_id,
            database=connection.database,
            host=connection.host,
            plan_sha256=plan_sha,
            changed=3,
            already=0,
        )
        run_command(
            go_recovery_argv(
                mode="apply",
                plan=plan,
                fixture_root=args.fixture_root,
                report=db_idempotent,
                connection=connection,
                run_id=args.run_id,
            ),
            repo_root=repo_root,
            env=env,
            label="recovery database idempotent apply",
        )
        validate_recovery_report(
            db_idempotent,
            mode="apply",
            run_id=args.run_id,
            database=connection.database,
            host=connection.host,
            plan_sha256=plan_sha,
            changed=0,
            already=3,
        )
        payload = component_report(
            component="recovery",
            action="apply",
            args=args,
            connection=connection,
            files=[
                write_ahead,
                plan,
                guard_before,
                guard_provision,
                db_apply,
                db_idempotent,
                *ownership_receipt_artifacts(
                    component_dir,
                    "recovery",
                    require_complete=True,
                ),
            ],
        )
        write_document(report, payload)
        return payload
    except Exception as original:
        compensation: list[str] = []
        database_safe = True
        guard_safe = not guard_ready
        files_safe = not materialized
        if guard_ready and binding is not None:
            try:
                run_command(
                    go_recovery_argv(
                        mode="rollback",
                        plan=plan,
                        fixture_root=args.fixture_root,
                        report=db_rollback,
                        connection=connection,
                        run_id=args.run_id,
                    ),
                    repo_root=repo_root,
                    env=go_env(),
                    label="recovery apply compensation database rollback",
                )
                validate_recovery_report(
                    db_rollback,
                    mode="rollback",
                    run_id=args.run_id,
                    database=connection.database,
                    host=connection.host,
                    plan_sha256=plan_sha,
                    changed=3,
                    already=0,
                    allowed_counts={(3, 0), (0, 3)},
                )
                compensation.append("database_restored")
            except Exception as exc:
                database_safe = False
                compensation.append(f"database_compensation_failed:{exc}")
            if database_safe:
                try:
                    restore_guard(
                        connection,
                        RECOVERY_GUARD,
                        binding,
                        guard_before,
                        guard_restore,
                    )
                    compensation.append("guard_restored")
                    guard_safe = True
                except Exception as exc:
                    database_safe = False
                    compensation.append(f"guard_compensation_failed:{exc}")
        if materialized and database_safe:
            try:
                run_command(
                    [
                        sys.executable,
                        str(
                            repo_root
                            / "scripts/ab/rollback_asset_recovery_materialization.py"
                        ),
                        "--plan",
                        str(plan),
                        "--fixture-root",
                        str(args.fixture_root),
                        "--report",
                        str(cleanup),
                        "--execute",
                    ],
                    repo_root=repo_root,
                    env=dict(os.environ),
                    label="recovery apply compensation file cleanup",
                )
                compensation.append("files_restored")
                files_safe = True
            except Exception as exc:
                compensation.append(f"file_compensation_failed:{exc}")
        compensation_state = self_bound(
            {
                "schema_version": 1,
                "status": (
                    "COMPENSATION_COMPLETE"
                    if database_safe and guard_safe and files_safe
                    else "ROLLBACK_REQUIRED"
                ),
                "component": "recovery",
                "run_id": args.run_id,
                "database": connection.database,
                "database_safe": database_safe,
                "guard_safe": guard_safe,
                "files_safe": files_safe,
                "details": compensation,
            }
        )
        write_document(
            component_dir / "recovery-apply-compensation-state.json",
            compensation_state,
        )
        raise ComponentError(
            f"recovery apply failed: {original}; compensation={compensation}"
        ) from original


def recovery_rollback(
    args: argparse.Namespace,
    connection: clone_db.Connection,
    repo_root: pathlib.Path,
    component_dir: pathlib.Path,
) -> dict[str, Any]:
    plan = component_dir / "recovery-materialization-plan.json"
    write_ahead = component_dir / "recovery-file-write-ahead.json"
    guard_before = component_dir / "recovery-guard-before.json"
    db_rollback = component_dir / "recovery-db-rollback.json"
    db_rollback_recheck = (
        component_dir / "recovery-db-rollback-recheck.json"
    )
    guard_restore = component_dir / "recovery-guard-restore.json"
    file_rollback = component_dir / "recovery-file-rollback.json"
    report = component_dir / "recovery-component-rollback.json"
    rollback_plan = plan if plan.is_file() else write_ahead
    plan_value = read_object(rollback_plan, "recovery rollback plan")
    verify_compact_self_bound(plan_value, "recovery rollback plan")
    plan_sha = sha256_file(plan) if plan.is_file() else ""
    entries = plan_value.get("entries")
    if (
        args.mapping is None
        or plan_value.get("run_id") != args.run_id
        or plan_value.get("mapping_sha256") != sha256_file(args.mapping)
        or not isinstance(entries, list)
        or {
            entry.get("missing_task_asset_id")
            for entry in entries
            if isinstance(entry, dict)
        }
        != {23989, 23990, 23991}
    ):
        raise ComponentError("recovery plan run_id differs")
    rollback_artifacts: list[pathlib.Path] = []
    if guard_before.is_file():
        binding = guard_binding(
            RECOVERY_GUARD,
            run_id=args.run_id,
            primary_sha256=plan_sha,
        )
        rollback_artifacts.append(db_rollback)
    else:
        binding = None
    if binding is not None and db_rollback.is_file():
        validate_recovery_report(
            db_rollback,
            mode="rollback",
            run_id=args.run_id,
            database=connection.database,
            host=connection.host,
            plan_sha256=plan_sha,
            changed=3,
            already=0,
            allowed_counts={(3, 0), (0, 3)},
        )
        rollback_target = db_rollback_recheck
        rollback_artifacts.append(db_rollback_recheck)
    elif binding is not None:
        rollback_target = db_rollback
    if binding is not None:
        run_command(
            go_recovery_argv(
                mode="rollback",
                plan=plan,
                fixture_root=args.fixture_root,
                report=rollback_target,
                connection=connection,
                run_id=args.run_id,
            ),
            repo_root=repo_root,
            env=go_env(),
            label="recovery database rollback",
        )
        validate_recovery_report(
            rollback_target,
            mode="rollback",
            run_id=args.run_id,
            database=connection.database,
            host=connection.host,
            plan_sha256=plan_sha,
            changed=0 if rollback_target == db_rollback_recheck else 3,
            already=3 if rollback_target == db_rollback_recheck else 0,
            allowed_counts=(
                None
                if rollback_target == db_rollback_recheck
                else {(3, 0), (0, 3)}
            ),
        )
        restore_guard(
            connection,
            RECOVERY_GUARD,
            binding,
            guard_before,
            guard_restore,
        )
        rollback_artifacts.append(guard_restore)
    run_command(
        [
            sys.executable,
            str(
                repo_root
                / "scripts/ab/rollback_asset_recovery_materialization.py"
            ),
            "--plan",
            str(rollback_plan),
            "--fixture-root",
            str(args.fixture_root),
            "--report",
            str(file_rollback),
            "--execute",
        ],
        repo_root=repo_root,
        env=dict(os.environ),
        label="recovery file rollback",
    )
    file_value = read_object(file_rollback, "recovery-file-rollback")
    if (
        file_value.get("status") != "ROLLED_BACK"
        or file_value.get("database_write_performed") is not False
        or file_value.get("production_write_performed") is not False
    ):
        raise ComponentError("recovery file rollback report contract failed")
    payload = component_report(
        component="recovery",
        action="rollback",
        args=args,
        connection=connection,
        files=[
            *rollback_artifacts,
            file_rollback,
            *ownership_receipt_artifacts(
                component_dir,
                "recovery",
                require_complete=False,
            ),
        ],
        database_writes_executed=binding is not None,
    )
    write_document(report, payload)
    return payload


def bundle_apply(
    args: argparse.Namespace,
    connection: clone_db.Connection,
    repo_root: pathlib.Path,
    run_root: pathlib.Path,
    component_dir: pathlib.Path,
) -> dict[str, Any]:
    materialize_report = component_dir / "bundle-materialize-report.json"
    registry = component_dir / "bundle-registry.json"
    write_ahead = component_dir / "bundle-file-write-ahead.json"
    staging_write_ahead = (
        component_dir / "bundle-staging-write-ahead.json"
    )
    guard_before = component_dir / "bundle-guard-before.json"
    guard_provision = component_dir / "bundle-guard-provision.json"
    db_apply = component_dir / "bundle-db-apply.json"
    db_idempotent = component_dir / "bundle-db-idempotent.json"
    db_journal = component_dir / "bundle-db-rollback-journal.json"
    report = component_dir / "bundle-component-apply.json"
    manifest_value = read_object(args.manifest, "bundle-manifest")
    candidate = str(manifest_value.get("source_candidate_sha256") or "")
    if (
        manifest_value.get("run_id") != args.run_id
        or not SHA256.fullmatch(candidate)
    ):
        raise ComponentError("bundle manifest run/candidate binding is invalid")
    materialized = False
    guard_ready = False
    database_apply_started = False
    binding: dict[str, Any] | None = None
    try:
        run_command(
            [
                sys.executable,
                str(repo_root / "scripts/ab/run_scoped_bundle_materializer.py"),
                "materialize",
                "--run-root",
                str(run_root),
                "--source-root",
                str(args.source_root),
                "--b-root",
                str(args.fixture_root),
                "--manifest",
                str(args.manifest),
                "--report",
                str(materialize_report),
                "--registry",
                str(registry),
                "--write-ahead-registry",
                str(write_ahead),
                "--staging-write-ahead-registry",
                str(staging_write_ahead),
                "--execute",
            ],
            repo_root=repo_root,
            env=dict(os.environ),
            label="bundle file materialization",
        )
        materialized = True
        registry_value = read_object(registry, "bundle-registry")
        receipt_artifacts = validate_bundle_registry_for_apply(
            registry_value,
            registry=registry,
            write_ahead=write_ahead,
            component_dir=component_dir,
            run_id=args.run_id,
        )
        registry_sha = sha256_file(registry)
        manifest_sha = sha256_file(args.manifest)
        binding = guard_binding(
            BUNDLE_GUARD,
            run_id=args.run_id,
            primary_sha256=candidate,
            secondary_sha256=registry_sha,
        )
        provision_guard(
            connection,
            BUNDLE_GUARD,
            binding,
            guard_before,
            guard_provision,
        )
        guard_ready = True
        env = go_env()
        database_apply_started = True
        run_command(
            go_bundle_argv(
                mode="apply",
                registry=registry,
                manifest=args.manifest,
                fixture_root=args.fixture_root,
                report=db_apply,
                connection=connection,
                run_id=args.run_id,
                candidate_sha256=candidate,
                rollback_journal=db_journal,
            ),
            repo_root=repo_root,
            env=env,
            label="bundle database apply",
        )
        journal_value = validate_bundle_journal(
            db_journal,
            manifest=manifest_value,
            run_id=args.run_id,
            database=connection.database,
            host=connection.host,
            candidate_sha256=candidate,
            registry_sha256=registry_sha,
            manifest_sha256=manifest_sha,
        )
        journal_sha = sha256_file(db_journal)
        validate_bundle_report(
            db_apply,
            mode="apply",
            run_id=args.run_id,
            database=connection.database,
            host=connection.host,
            candidate_sha256=candidate,
            registry_sha256=registry_sha,
            manifest_sha256=manifest_sha,
            rollback_journal_sha256=journal_sha,
            rollback_journal_evidence_sha256=journal_value[
                "evidence_sha256"
            ],
            changed=7,
            already=0,
        )
        run_command(
            go_bundle_argv(
                mode="apply",
                registry=registry,
                manifest=args.manifest,
                fixture_root=args.fixture_root,
                report=db_idempotent,
                connection=connection,
                run_id=args.run_id,
                candidate_sha256=candidate,
                rollback_journal=db_journal,
            ),
            repo_root=repo_root,
            env=env,
            label="bundle database idempotent apply",
        )
        validate_bundle_report(
            db_idempotent,
            mode="apply",
            run_id=args.run_id,
            database=connection.database,
            host=connection.host,
            candidate_sha256=candidate,
            registry_sha256=registry_sha,
            manifest_sha256=manifest_sha,
            rollback_journal_sha256=journal_sha,
            rollback_journal_evidence_sha256=journal_value[
                "evidence_sha256"
            ],
            changed=0,
            already=7,
        )
        second_journal = validate_bundle_journal(
            db_journal,
            manifest=manifest_value,
            run_id=args.run_id,
            database=connection.database,
            host=connection.host,
            candidate_sha256=candidate,
            registry_sha256=registry_sha,
            manifest_sha256=manifest_sha,
        )
        if sha256_file(db_journal) != journal_sha:
            raise ComponentError(
                "idempotent bundle apply changed the rollback journal"
            )
        if second_journal["evidence_sha256"] != journal_value[
            "evidence_sha256"
        ]:
            raise ComponentError(
                "idempotent bundle apply changed journal evidence"
            )
        payload = component_report(
            component="bundle",
            action="apply",
            args=args,
            connection=connection,
            files=[
                staging_write_ahead,
                write_ahead,
                materialize_report,
                registry,
                guard_before,
                guard_provision,
                db_journal,
                db_apply,
                db_idempotent,
                *receipt_artifacts,
            ],
        )
        write_document(report, payload)
        return payload
    except Exception as original:
        compensation: list[str] = []
        database_safe = True
        guard_safe = not guard_ready
        files_safe = not materialized
        if guard_ready and binding is not None:
            if db_journal.is_file():
                try:
                    compensation_report = (
                        component_dir / "bundle-apply-compensation-db.json"
                    )
                    run_command(
                        go_bundle_argv(
                            mode="rollback",
                            registry=registry,
                            manifest=args.manifest,
                            fixture_root=args.fixture_root,
                            report=compensation_report,
                            connection=connection,
                            run_id=args.run_id,
                            candidate_sha256=candidate,
                            rollback_journal=db_journal,
                            apply_report=None,
                        ),
                        repo_root=repo_root,
                        env=go_env(),
                        label="bundle apply compensation database rollback",
                    )
                    validate_bundle_report(
                        compensation_report,
                        mode="rollback",
                        run_id=args.run_id,
                        database=connection.database,
                        host=connection.host,
                        candidate_sha256=candidate,
                        registry_sha256=registry_sha,
                        manifest_sha256=manifest_sha,
                        rollback_journal_sha256=sha256_file(
                            db_journal
                        ),
                        rollback_journal_evidence_sha256=read_object(
                            db_journal, "bundle rollback journal"
                        )["evidence_sha256"],
                        changed=7,
                        already=0,
                        allowed_counts={(7, 0), (0, 7)},
                    )
                    compensation.append("database_restored")
                except Exception as exc:
                    database_safe = False
                    compensation.append(
                        f"database_compensation_failed:{exc}"
                    )
            else:
                if database_apply_started:
                    database_safe = False
                    compensation.append(
                        "rollback_journal_missing_or_invalid_guard_retained"
                    )
                else:
                    compensation.append(
                        "database_apply_not_started"
                    )
            if database_safe:
                try:
                    restore_guard(
                        connection,
                        BUNDLE_GUARD,
                        binding,
                        guard_before,
                        component_dir
                        / "bundle-apply-compensation-guard.json",
                    )
                    compensation.append("guard_restored")
                    guard_safe = True
                except Exception as exc:
                    database_safe = False
                    compensation.append(f"guard_compensation_failed:{exc}")
        if materialized and database_safe:
            try:
                run_command(
                    [
                        sys.executable,
                        str(
                            repo_root
                            / "scripts/ab/run_scoped_bundle_materializer.py"
                        ),
                        "rollback",
                        "--run-root",
                        str(run_root),
                        "--b-root",
                        str(args.fixture_root),
                        "--registry",
                        str(registry),
                        "--report",
                        str(
                            component_dir
                            / "bundle-apply-compensation-files.json"
                        ),
                        "--execute",
                    ],
                    repo_root=repo_root,
                    env=dict(os.environ),
                    label="bundle apply compensation file cleanup",
                )
                compensation.append("files_restored")
                files_safe = True
            except Exception as exc:
                compensation.append(f"file_compensation_failed:{exc}")
        compensation_state = self_bound(
            {
                "schema_version": 1,
                "status": (
                    "COMPENSATION_COMPLETE"
                    if database_safe and guard_safe and files_safe
                    else "ROLLBACK_REQUIRED"
                ),
                "component": "bundle",
                "run_id": args.run_id,
                "database": connection.database,
                "database_safe": database_safe,
                "guard_safe": guard_safe,
                "files_safe": files_safe,
                "details": compensation,
            }
        )
        write_document(
            component_dir / "bundle-apply-compensation-state.json",
            compensation_state,
        )
        raise ComponentError(
            f"bundle apply failed: {original}; compensation={compensation}"
        ) from original


def bundle_rollback(
    args: argparse.Namespace,
    connection: clone_db.Connection,
    repo_root: pathlib.Path,
    run_root: pathlib.Path,
    component_dir: pathlib.Path,
) -> dict[str, Any]:
    registry = component_dir / "bundle-registry.json"
    write_ahead = component_dir / "bundle-file-write-ahead.json"
    staging_write_ahead = (
        component_dir / "bundle-staging-write-ahead.json"
    )
    guard_before = component_dir / "bundle-guard-before.json"
    db_apply = component_dir / "bundle-db-apply.json"
    db_journal = component_dir / "bundle-db-rollback-journal.json"
    db_rollback = component_dir / "bundle-db-rollback.json"
    db_rollback_recheck = component_dir / "bundle-db-rollback-recheck.json"
    guard_restore = component_dir / "bundle-guard-restore.json"
    file_rollback = component_dir / "bundle-file-rollback.json"
    report = component_dir / "bundle-component-rollback.json"
    compensation_state_path = (
        component_dir / "bundle-apply-compensation-state.json"
    )
    manifest_value = read_object(args.manifest, "bundle-manifest")
    candidate = str(manifest_value.get("source_candidate_sha256") or "")
    rollback_registry = (
        registry
        if registry.is_file()
        else write_ahead
        if write_ahead.is_file()
        else staging_write_ahead
    )
    registry_value = read_object(rollback_registry, "bundle rollback registry")
    verify_self_bound(registry_value, "bundle rollback registry")
    if (
        manifest_value.get("run_id") != args.run_id
        or registry_value.get("run_id") != args.run_id
        or registry_value.get("manifest_sha256") != sha256_file(args.manifest)
        or not SHA256.fullmatch(candidate)
    ):
        raise ComponentError("bundle rollback input binding is invalid")
    if registry_value.get("status") in {"WRITE_AHEAD", "MATERIALIZED"}:
        entries = registry_value.get("entries")
        if not isinstance(entries, list) or len(entries) != 7:
            raise ComponentError("bundle rollback registry must contain seven entries")
    registry_sha = sha256_file(registry) if registry.is_file() else ""
    manifest_sha = sha256_file(args.manifest)
    compensation_complete = False
    if compensation_state_path.is_file():
        compensation_state = read_object(
            compensation_state_path, "bundle compensation state"
        )
        verify_self_bound(compensation_state, "bundle compensation state")
        compensation_complete = (
            compensation_state.get("status") == "COMPENSATION_COMPLETE"
            and compensation_state.get("component") == "bundle"
            and compensation_state.get("run_id") == args.run_id
            and compensation_state.get("database") == connection.database
            and compensation_state.get("database_safe") is True
            and compensation_state.get("guard_safe") is True
            and compensation_state.get("files_safe") is True
        )
    rollback_artifacts: list[pathlib.Path] = []
    if db_journal.is_file():
        rollback_artifacts.append(db_journal)
    if guard_before.is_file():
        if not registry.is_file():
            raise ComponentError("bundle guard exists without final registry")
        binding = guard_binding(
            BUNDLE_GUARD,
            run_id=args.run_id,
            primary_sha256=candidate,
            secondary_sha256=registry_sha,
        )
    else:
        binding = None
    if (
        binding is not None
        and db_apply.is_file()
        and not db_journal.is_file()
        and not compensation_complete
    ):
        raise ComponentError(
            "bundle apply report exists without rollback journal"
        )
    if binding is not None and db_journal.is_file() and db_rollback.is_file():
        rollback_artifacts.append(db_rollback)
        validate_bundle_report(
            db_rollback,
            mode="rollback",
            run_id=args.run_id,
            database=connection.database,
            host=connection.host,
            candidate_sha256=candidate,
            registry_sha256=registry_sha,
            manifest_sha256=manifest_sha,
            rollback_journal_sha256=sha256_file(db_journal),
            rollback_journal_evidence_sha256=read_object(
                db_journal, "bundle rollback journal"
            )["evidence_sha256"],
            changed=7,
            already=0,
            allowed_counts={(7, 0), (0, 7)},
        )
        rollback_target = db_rollback_recheck
        rollback_artifacts.append(db_rollback_recheck)
    elif binding is not None and db_journal.is_file():
        rollback_target = db_rollback
        rollback_artifacts.append(db_rollback)
    if binding is not None and db_journal.is_file():
        journal_value = validate_bundle_journal(
            db_journal,
            manifest=manifest_value,
            run_id=args.run_id,
            database=connection.database,
            host=connection.host,
            candidate_sha256=candidate,
            registry_sha256=registry_sha,
            manifest_sha256=manifest_sha,
        )
        apply_crosscheck: pathlib.Path | None = None
        if db_apply.is_file():
            validate_bundle_report(
                db_apply,
                mode="apply",
                run_id=args.run_id,
                database=connection.database,
                host=connection.host,
                candidate_sha256=candidate,
                registry_sha256=registry_sha,
                manifest_sha256=manifest_sha,
                rollback_journal_sha256=sha256_file(db_journal),
                rollback_journal_evidence_sha256=journal_value[
                    "evidence_sha256"
                ],
                changed=7,
                already=0,
            )
            apply_crosscheck = db_apply
        run_command(
            go_bundle_argv(
                mode="rollback",
                registry=registry,
                manifest=args.manifest,
                fixture_root=args.fixture_root,
                report=rollback_target,
                connection=connection,
                run_id=args.run_id,
                candidate_sha256=candidate,
                rollback_journal=db_journal,
                apply_report=apply_crosscheck,
            ),
            repo_root=repo_root,
            env=go_env(),
            label="bundle database rollback",
        )
        validate_bundle_report(
            rollback_target,
            mode="rollback",
            run_id=args.run_id,
            database=connection.database,
            host=connection.host,
            candidate_sha256=candidate,
            registry_sha256=registry_sha,
            manifest_sha256=manifest_sha,
            rollback_journal_sha256=sha256_file(db_journal),
            rollback_journal_evidence_sha256=read_object(
                db_journal, "bundle rollback journal"
            )["evidence_sha256"],
            changed=0 if rollback_target == db_rollback_recheck else 7,
            already=7 if rollback_target == db_rollback_recheck else 0,
            allowed_counts=(
                None
                if rollback_target == db_rollback_recheck
                else {(7, 0), (0, 7)}
            ),
        )
    if binding is not None:
        restore_guard(
            connection,
            BUNDLE_GUARD,
            binding,
            guard_before,
            guard_restore,
        )
        rollback_artifacts.append(guard_restore)
    run_command(
        [
            sys.executable,
            str(repo_root / "scripts/ab/run_scoped_bundle_materializer.py"),
            "rollback",
            "--run-root",
            str(run_root),
            "--b-root",
            str(args.fixture_root),
            "--registry",
            str(rollback_registry),
            "--report",
            str(file_rollback),
            "--execute",
        ],
        repo_root=repo_root,
        env=dict(os.environ),
        label="bundle file rollback",
    )
    file_value = read_object(file_rollback, "bundle-file-rollback")
    if (
        file_value.get("status") != "ROLLED_BACK"
        or file_value.get("database_write_performed") is not False
    ):
        raise ComponentError("bundle file rollback report contract failed")
    payload = component_report(
        component="bundle",
        action="rollback",
        args=args,
        connection=connection,
        files=[
            *rollback_artifacts,
            file_rollback,
            *ownership_receipt_artifacts(
                component_dir,
                "bundle",
                require_complete=False,
            ),
        ],
        database_writes_executed=(
            binding is not None and db_journal.is_file()
        ),
    )
    write_document(report, payload)
    return payload


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "action",
        choices=(
            "recovery-apply",
            "recovery-rollback",
            "bundle-apply",
            "bundle-rollback",
        ),
    )
    parser.add_argument("--database", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--run-root", type=pathlib.Path, required=True)
    parser.add_argument("--component-dir", type=pathlib.Path, required=True)
    parser.add_argument("--fixture-root", type=pathlib.Path, required=True)
    parser.add_argument("--mapping", type=pathlib.Path)
    parser.add_argument("--evidence", type=pathlib.Path)
    parser.add_argument("--source-root", type=pathlib.Path)
    parser.add_argument("--manifest", type=pathlib.Path)
    parser.add_argument("--mysql", default="mysql")
    return parser.parse_args()


def run(args: argparse.Namespace) -> dict[str, Any]:
    run_root, component_dir = validate_root(args)
    if shutil.which("go") is None:
        raise ComponentError("go executable is required before clone writes")
    connection = clone_db.Connection.confirmed_clone_b(
        args.database, args.mysql
    )
    repo_root = pathlib.Path(__file__).resolve().parents[2]
    if args.action.startswith("recovery-"):
        if args.mapping is None or (
            args.action == "recovery-apply" and args.evidence is None
        ):
            raise ComponentError(
                "recovery actions require mapping; apply also requires evidence"
            )
        if args.action == "recovery-apply":
            return recovery_apply(args, connection, repo_root, component_dir)
        return recovery_rollback(args, connection, repo_root, component_dir)
    if args.manifest is None:
        raise ComponentError("bundle actions require --manifest")
    if args.action == "bundle-apply":
        if args.source_root is None:
            raise ComponentError("bundle-apply requires --source-root")
        return bundle_apply(
            args, connection, repo_root, run_root, component_dir
        )
    return bundle_rollback(
        args, connection, repo_root, run_root, component_dir
    )


def main() -> int:
    parser = argparse.ArgumentParser(add_help=False)
    try:
        args = parse_args()
        run(args)
    except Exception as exc:
        parser.error(str(exc))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
