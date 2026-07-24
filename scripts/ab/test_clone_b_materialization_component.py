import argparse
import json
import os
import pathlib
import tempfile
import unittest
from unittest import mock

from scripts.ab import clone_b_materialization_component as component


class FakeConnection:
    def __init__(self):
        self.database = "ab_formal_b_ui"
        self.host = "127.0.0.1"
        self.exists = False
        self.rows = []
        self.executed = []
        self.fail_next_insert = False

    def execute(self, sql):
        self.executed.append(sql)
        if "information_schema.columns" in sql:
            rows = []
            for ordinal, (name, column_type) in enumerate(
                component.RECOVERY_GUARD.columns, 1
            ):
                rows.append(
                    {
                        "kind": "column",
                        "table": component.RECOVERY_GUARD.table,
                        "ordinal": ordinal,
                        "name": name,
                        "column_type": column_type,
                        "nullable": "NO",
                        "default_hex": None,
                        "extra": "",
                        "generation_expression": "",
                        "character_set": (
                            None if name == "singleton_id" else "utf8mb4"
                        ),
                        "collation": (
                            None
                            if name == "singleton_id"
                            else "utf8mb4_0900_ai_ci"
                        ),
                    }
                )
            return "".join(
                json.dumps(row, separators=(",", ":")) + "\n"
                for row in rows
            )
        if "information_schema.tables" in sql:
            return "1\n" if self.exists else "0\n"
        if "information_schema.statistics" in sql:
            return "1\n"
        if sql.startswith("SHOW CREATE TABLE"):
            return (
                component.RECOVERY_GUARD.table
                + "\t"
                + component.RECOVERY_GUARD.create_sql
                + "\n"
            )
        if sql.startswith("SELECT JSON_OBJECT"):
            return "".join(
                json.dumps(row, separators=(",", ":")) + "\n"
                for row in self.rows
            )
        if sql.startswith("CREATE TABLE"):
            self.exists = True
            return ""
        if sql.startswith("START TRANSACTION"):
            if "INSERT INTO" in sql:
                if self.fail_next_insert:
                    self.fail_next_insert = False
                    raise RuntimeError("injected guard insert failure")
                if "'clone_b'" in sql:
                    run_id = sql.split("'clone_b','", 1)[1].split("'", 1)[0]
                    plan_sha = sql.rsplit("'", 2)[1]
                    self.rows = [
                        {
                            "singleton_id": 1,
                            "environment": "clone_b",
                            "run_id": run_id,
                            "plan_sha256": plan_sha,
                        }
                    ]
                else:
                    raise AssertionError(sql)
            else:
                self.rows = []
            return ""
        if sql.startswith("DROP TABLE"):
            self.exists = False
            self.rows = []
            return ""
        raise AssertionError(f"unexpected SQL: {sql}")


def recovery_report(
    *,
    mode,
    run_id,
    plan_sha,
    changed,
    already,
):
    return {
        "version": 1,
        "mode": mode,
        "run_id": run_id,
        "database": "ab_formal_b_ui",
        "host": "127.0.0.1",
        "plan_sha256": plan_sha,
        "changed_entries": changed,
        "already_in_target_state_entries": already,
        "database_transaction_committed": True,
        "object_storage_writes_executed": False,
    }


def write_component_chain_fixture(
    run_dir,
    *,
    run_id="formal-test",
    database="ab_formal_b_ui",
    host="127.0.0.1",
):
    run_dir = pathlib.Path(run_dir)

    def write(name, value):
        path = run_dir / name
        path.write_bytes(component.canonical_bytes(value))
        return path

    def guard_before(name):
        return component.self_bound(
            {
                "schema_version": 1,
                "kind": "clone-b-guard-state",
                "component": name,
                "database": database,
                "table": (
                    component.RECOVERY_GUARD.table
                    if name == "recovery"
                    else component.BUNDLE_GUARD.table
                ),
                "table_existed": False,
                "create_table_sql": None,
                "schema": [],
                "rows": [],
            }
        )

    recovery_plan = write(
        "recovery-materialization-plan.json",
        {"version": 1, "status": "MATERIALIZED", "run_id": run_id},
    )
    recovery_plan_sha = component.sha256_file(recovery_plan)
    recovery_before = write(
        "recovery-guard-before.json", guard_before("recovery")
    )
    recovery_provision = write(
        "recovery-guard-provision.json",
        component.self_bound(
            {
                "schema_version": 1,
                "status": "PROVISIONED",
                "component": "recovery",
                "database": database,
                "table": component.RECOVERY_GUARD.table,
                "before_artifact_sha256": component.sha256_file(
                    recovery_before
                ),
                "before_state_sha256": json.loads(
                    recovery_before.read_text()
                )["evidence_sha256"],
                "binding": {
                    "singleton_id": 1,
                    "environment": "clone_b",
                    "run_id": run_id,
                    "plan_sha256": recovery_plan_sha,
                },
                "after_state_sha256": "a" * 64,
            }
        ),
    )
    recovery_apply = write(
        "recovery-db-apply.json",
        {
            "version": 1,
            "mode": "apply",
            "run_id": run_id,
            "database": database,
            "host": host,
            "plan_sha256": recovery_plan_sha,
            "changed_entries": 3,
            "already_in_target_state_entries": 0,
            "database_transaction_committed": True,
            "object_storage_writes_executed": False,
        },
    )
    recovery_idempotent = write(
        "recovery-db-idempotent.json",
        {
            **json.loads(recovery_apply.read_text()),
            "changed_entries": 0,
            "already_in_target_state_entries": 3,
        },
    )
    recovery_component_apply = component.self_bound(
        {
            "schema_version": 1,
            "status": "APPLIED",
            "component": "recovery",
            "action": "apply",
            "run_id": run_id,
            "database": database,
            "host": host,
            "database_writes_executed": True,
            "production_writes_executed": False,
            "guard_retained_for_rollback": True,
            "guard_exactly_restored": False,
            "artifacts": [
                component.artifact(path)
                for path in (
                    recovery_plan,
                    recovery_before,
                    recovery_provision,
                    recovery_apply,
                    recovery_idempotent,
                )
            ],
        }
    )
    write("recovery-component-apply.json", recovery_component_apply)
    recovery_db_rollback = write(
        "recovery-db-rollback.json",
        {
            **json.loads(recovery_apply.read_text()),
            "mode": "rollback",
        },
    )
    recovery_restore = write(
        "recovery-guard-restore.json",
        component.self_bound(
            {
                "schema_version": 1,
                "status": "RESTORED",
                "component": "recovery",
                "database": database,
                "table": component.RECOVERY_GUARD.table,
                "before_artifact_sha256": component.sha256_file(
                    recovery_before
                ),
                "restored_state_sha256": json.loads(
                    recovery_before.read_text()
                )["evidence_sha256"],
                "exact": True,
            }
        ),
    )
    recovery_file = write(
        "recovery-file-rollback.json",
        {
            "schema_version": 1,
            "status": "ROLLED_BACK",
            "database_write_performed": False,
            "production_write_performed": False,
        },
    )
    write(
        "recovery-component-rollback.json",
        component.self_bound(
            {
                "schema_version": 1,
                "status": "ROLLED_BACK",
                "component": "recovery",
                "action": "rollback",
                "run_id": run_id,
                "database": database,
                "host": host,
                "database_writes_executed": True,
                "production_writes_executed": False,
                "guard_retained_for_rollback": False,
                "guard_exactly_restored": True,
                "artifacts": [
                    component.artifact(path)
                    for path in (
                        recovery_db_rollback,
                        recovery_restore,
                        recovery_file,
                    )
                ],
            }
        ),
    )

    bundle_materialize = write(
        "bundle-materialize-report.json",
        {"schema_version": 1, "status": "MATERIALIZED", "run_id": run_id},
    )
    bundle_registry = write(
        "bundle-registry.json",
        {"schema_version": 1, "status": "MATERIALIZED", "run_id": run_id},
    )
    registry_sha = component.sha256_file(bundle_registry)
    candidate_sha = "b" * 64
    manifest_sha = "c" * 64
    bundle_before = write("bundle-guard-before.json", guard_before("bundle"))
    bundle_provision = write(
        "bundle-guard-provision.json",
        component.self_bound(
            {
                "schema_version": 1,
                "status": "PROVISIONED",
                "component": "bundle",
                "database": database,
                "table": component.BUNDLE_GUARD.table,
                "before_artifact_sha256": component.sha256_file(bundle_before),
                "before_state_sha256": json.loads(bundle_before.read_text())[
                    "evidence_sha256"
                ],
                "binding": {
                    "singleton_id": 1,
                    "environment": "clone_b",
                    "run_id": run_id,
                    "candidate_sha256": candidate_sha,
                    "registry_sha256": registry_sha,
                },
                "after_state_sha256": "d" * 64,
            }
        ),
    )

    def bundle_db(mode, changed, already):
        return {
            "schema_version": 1,
            "mode": mode,
            "status": "PASS",
            "run_id": run_id,
            "database": database,
            "host": host,
            "candidate_sha256": candidate_sha,
            "registry_sha256": registry_sha,
            "manifest_sha256": manifest_sha,
            "changed_bundle_count": changed,
            "already_applied_bundle_count": already,
            "database_transaction_committed": True,
        }

    bundle_apply = write(
        "bundle-db-apply.json", bundle_db("apply", 7, 0)
    )
    bundle_idempotent = write(
        "bundle-db-idempotent.json", bundle_db("apply", 0, 7)
    )
    write(
        "bundle-component-apply.json",
        component.self_bound(
            {
                "schema_version": 1,
                "status": "APPLIED",
                "component": "bundle",
                "action": "apply",
                "run_id": run_id,
                "database": database,
                "host": host,
                "database_writes_executed": True,
                "production_writes_executed": False,
                "guard_retained_for_rollback": True,
                "guard_exactly_restored": False,
                "artifacts": [
                    component.artifact(path)
                    for path in (
                        bundle_materialize,
                        bundle_registry,
                        bundle_before,
                        bundle_provision,
                        bundle_apply,
                        bundle_idempotent,
                    )
                ],
            }
        ),
    )
    bundle_db_rollback = write(
        "bundle-db-rollback.json", bundle_db("rollback", 7, 0)
    )
    bundle_restore = write(
        "bundle-guard-restore.json",
        component.self_bound(
            {
                "schema_version": 1,
                "status": "RESTORED",
                "component": "bundle",
                "database": database,
                "table": component.BUNDLE_GUARD.table,
                "before_artifact_sha256": component.sha256_file(bundle_before),
                "restored_state_sha256": json.loads(
                    bundle_before.read_text()
                )["evidence_sha256"],
                "exact": True,
            }
        ),
    )
    bundle_file = write(
        "bundle-file-rollback.json",
        {
            "schema_version": 1,
            "status": "ROLLED_BACK",
            "database_write_performed": False,
        },
    )
    write(
        "bundle-component-rollback.json",
        component.self_bound(
            {
                "schema_version": 1,
                "status": "ROLLED_BACK",
                "component": "bundle",
                "action": "rollback",
                "run_id": run_id,
                "database": database,
                "host": host,
                "database_writes_executed": True,
                "production_writes_executed": False,
                "guard_retained_for_rollback": False,
                "guard_exactly_restored": True,
                "artifacts": [
                    component.artifact(path)
                    for path in (
                        bundle_db_rollback,
                        bundle_restore,
                        bundle_file,
                    )
                ],
            }
        ),
    )


class CloneBMaterializationComponentTest(unittest.TestCase):
    def test_guard_absent_is_frozen_provisioned_and_exactly_removed(self):
        connection = FakeConnection()
        binding = component.guard_binding(
            component.RECOVERY_GUARD,
            run_id="formal-test",
            primary_sha256="a" * 64,
        )
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            before_path = root / "before.json"
            provision_path = root / "provision.json"
            restore_path = root / "restore.json"
            before = component.provision_guard(
                connection,
                component.RECOVERY_GUARD,
                binding,
                before_path,
                provision_path,
            )
            self.assertFalse(before["table_existed"])
            self.assertTrue(connection.exists)
            self.assertEqual(connection.rows, [binding])
            restored = component.restore_guard(
                connection,
                component.RECOVERY_GUARD,
                binding,
                before_path,
                restore_path,
            )
            self.assertEqual(restored["status"], "RESTORED")
            self.assertTrue(restored["exact"])
            self.assertFalse(connection.exists)
            self.assertEqual(connection.rows, [])
            self.assertNotIn("secret", before_path.read_text())

    def test_guard_provision_failure_restores_absent_before_state(self):
        connection = FakeConnection()
        connection.fail_next_insert = True
        binding = component.guard_binding(
            component.RECOVERY_GUARD,
            run_id="formal-test",
            primary_sha256="a" * 64,
        )
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            with self.assertRaisesRegex(
                component.ComponentError, "was compensated"
            ):
                component.provision_guard(
                    connection,
                    component.RECOVERY_GUARD,
                    binding,
                    root / "before.json",
                    root / "provision.json",
                )
        self.assertFalse(connection.exists)
        self.assertEqual(connection.rows, [])

    def test_go_argv_never_contains_the_dsn(self):
        connection = FakeConnection()
        recovery = component.go_recovery_argv(
            mode="apply",
            plan=pathlib.Path("/tmp/plan.json"),
            fixture_root=pathlib.Path("/tmp/fixture-upload-b"),
            report=pathlib.Path("/tmp/report.json"),
            connection=connection,
            run_id="formal-test",
        )
        bundle = component.go_bundle_argv(
            mode="apply",
            registry=pathlib.Path("/tmp/registry.json"),
            manifest=pathlib.Path("/tmp/manifest.json"),
            fixture_root=pathlib.Path("/tmp/fixture-upload-b"),
            report=pathlib.Path("/tmp/report.json"),
            connection=connection,
            run_id="formal-test",
            candidate_sha256="b" * 64,
        )
        for argv in (recovery, bundle):
            rendered = " ".join(argv)
            self.assertNotIn("@tcp(", rendered)
            self.assertNotIn("--dsn", argv)

    def test_recovery_apply_failure_runs_database_guard_and_file_compensation(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            run_root = root / "formal-test"
            component_dir = run_root / "g4"
            fixture = run_root / "fixture-upload-b"
            component_dir.mkdir(parents=True)
            fixture.mkdir()
            mapping = run_root / "mapping.json"
            evidence = run_root / "evidence.json"
            mapping.write_text("{}\n", encoding="utf-8")
            evidence.write_text("{}\n", encoding="utf-8")
            args = argparse.Namespace(
                run_id="formal-test",
                mapping=mapping,
                evidence=evidence,
                fixture_root=fixture,
            )
            connection = FakeConnection()
            labels = []

            def command(argv, *, repo_root, env, label):
                labels.append(label)
                if label == "recovery file materialization":
                    plan = component_dir / "recovery-materialization-plan.json"
                    plan.write_text(
                        json.dumps(
                            {
                                "version": 1,
                                "status": "MATERIALIZED",
                                "run_id": "formal-test",
                            }
                        )
                        + "\n",
                        encoding="utf-8",
                    )
                elif label == "recovery database apply":
                    plan = component_dir / "recovery-materialization-plan.json"
                    report = component_dir / "recovery-db-apply.json"
                    report.write_text(
                        json.dumps(
                            recovery_report(
                                mode="apply",
                                run_id="formal-test",
                                plan_sha=component.sha256_file(plan),
                                changed=3,
                                already=0,
                            )
                        )
                        + "\n",
                        encoding="utf-8",
                    )
                elif label == "recovery database idempotent apply":
                    raise component.ComponentError("injected failure")
                elif "compensation" in label:
                    pathlib.Path(argv[argv.index("--report-file") + 1]).write_text(
                        "{}\n", encoding="utf-8"
                    ) if "--report-file" in argv else pathlib.Path(
                        argv[argv.index("--report") + 1]
                    ).write_text("{}\n", encoding="utf-8")

            def provision(
                connection,
                spec,
                binding,
                before_path,
                provision_path,
            ):
                before = component.self_bound(
                    {
                        "schema_version": 1,
                        "kind": "clone-b-guard-state",
                        "component": "recovery",
                        "database": connection.database,
                        "table": spec.table,
                        "table_existed": False,
                        "create_table_sql": None,
                        "schema": [],
                        "rows": [],
                    }
                )
                component.write_document(before_path, before)
                component.write_document(
                    provision_path,
                    component.self_bound(
                        {
                            "schema_version": 1,
                            "status": "PROVISIONED",
                        }
                    ),
                )
                return before

            with (
                mock.patch.object(component, "run_command", side_effect=command),
                mock.patch.object(component, "provision_guard", side_effect=provision),
                mock.patch.object(component, "restore_guard") as restore,
                mock.patch.dict(
                    os.environ,
                    {"MYSQL_DSN": "u:p@tcp(127.0.0.1:3307)/ab_formal_b_ui"},
                    clear=False,
                ),
            ):
                with self.assertRaisesRegex(
                    component.ComponentError, "compensation="
                ):
                    component.recovery_apply(
                        args,
                        connection,
                        pathlib.Path("/repo"),
                        component_dir,
                    )
            restore.assert_called_once()
            self.assertEqual(
                labels,
                [
                    "recovery file materialization",
                    "recovery database apply",
                    "recovery database idempotent apply",
                    "recovery apply compensation database rollback",
                    "recovery apply compensation file cleanup",
                ],
            )

    def test_report_validation_requires_real_change_then_idempotence(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            report = root / "report.json"
            report.write_text(
                json.dumps(
                    recovery_report(
                        mode="apply",
                        run_id="formal-test",
                        plan_sha="a" * 64,
                        changed=0,
                        already=3,
                    )
                )
                + "\n",
                encoding="utf-8",
            )
            with self.assertRaisesRegex(
                component.ComponentError, "report contract"
            ):
                component.validate_recovery_report(
                    report,
                    mode="apply",
                    run_id="formal-test",
                    database="ab_formal_b_ui",
                    host="127.0.0.1",
                    plan_sha256="a" * 64,
                    changed=3,
                    already=0,
                )


if __name__ == "__main__":
    unittest.main()
