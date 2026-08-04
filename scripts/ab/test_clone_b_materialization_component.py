import argparse
import hashlib
import json
import os
import pathlib
import tempfile
import unittest
from unittest import mock

from scripts.ab import clone_b_materialization_component as component


def compact_self(value):
    result = dict(value)
    result["evidence_sha256"] = hashlib.sha256(
        json.dumps(
            result,
            ensure_ascii=False,
            sort_keys=True,
            separators=(",", ":"),
        ).encode("utf-8")
    ).hexdigest()
    return result


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


def bundle_report(*, changed, already):
    return {
        "schema_version": 1,
        "mode": "rollback",
        "status": "PASS",
        "run_id": "formal-test",
        "database": "ab_formal_b_ui",
        "host": "127.0.0.1",
        "candidate_sha256": "a" * 64,
        "registry_sha256": "b" * 64,
        "manifest_sha256": "c" * 64,
        "rollback_journal_sha256": "d" * 64,
        "rollback_journal_evidence_sha256": "e" * 64,
        "changed_bundle_count": changed,
        "already_applied_bundle_count": already,
        "database_transaction_committed": True,
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
        compact_self(
            {"version": 1, "status": "MATERIALIZED", "run_id": run_id}
        ),
    )
    recovery_plan_sha = component.sha256_file(recovery_plan)
    recovery_write_ahead = write(
        "recovery-file-write-ahead.json",
        component.self_bound(
            {"version": 1, "status": "PREPARED", "run_id": run_id}
        ),
    )
    recovery_receipts = [
        write(
            name,
            component.self_bound(
                {
                    "schema_version": 1,
                    "status": status,
                    "run_id": run_id,
                }
            ),
        )
        for asset_id in (23989, 23990, 23991)
        for name, status in (
            (
                f"recovery-ownership-{asset_id}.json",
                "OWNED_LINK",
            ),
            (
                f"recovery-staging-ownership-{asset_id}.json",
                "STAGING_OWNED",
            ),
        )
    ]
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
            "ownership_receipt_contract_version": 1,
            "artifacts": [
                component.artifact(path)
                for path in (
                    recovery_write_ahead,
                    recovery_plan,
                    recovery_before,
                    recovery_provision,
                    recovery_apply,
                    recovery_idempotent,
                    *recovery_receipts,
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
                "ownership_receipt_contract_version": 1,
                "artifacts": [
                    component.artifact(path)
                    for path in (
                        recovery_db_rollback,
                        recovery_restore,
                        recovery_file,
                        *recovery_receipts,
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
    bundle_staging_write_ahead = write(
        "bundle-staging-write-ahead.json",
        component.self_bound(
            {
                "schema_version": 1,
                "status": "STAGING_WRITE_AHEAD",
                "run_id": run_id,
            }
        ),
    )
    bundle_write_ahead = write(
        "bundle-file-write-ahead.json",
        component.self_bound(
            {
                "schema_version": 1,
                "status": "WRITE_AHEAD",
                "run_id": run_id,
            }
        ),
    )
    bundle_receipts = [
        write(
            name,
            component.self_bound(
                {
                    "schema_version": 1,
                    "status": status,
                    "run_id": run_id,
                }
            ),
        )
        for asset_id in range(25557, 25564)
        for name, status in (
            (f"bundle-ownership-{asset_id}.json", "OWNED_LINK"),
            (
                f"bundle-staging-ownership-{asset_id}.json",
                "STAGING_OWNED",
            ),
        )
    ]
    registry_sha = component.sha256_file(bundle_registry)
    candidate_sha = "b" * 64
    manifest_sha = "c" * 64
    bundle_journal_value = {
        "schema_version": 1,
        "kind": "source-bundle-clone-b-rollback-journal",
        "status": "PREPARED",
        "run_id": run_id,
        "database": database,
        "host": host,
        "candidate_sha256": candidate_sha,
        "registry_sha256": registry_sha,
        "manifest_sha256": manifest_sha,
        "prepared_before_first_database_mutation": True,
        "database_commit_state": "unknown",
        "expected_bundle_count": 7,
        "expected_member_count": 22,
        "changed_bundle_count": 7,
        "already_applied_bundle_count": 0,
        "member_before": [
            {
                "task_asset_id": index,
                "original_whole_hash": None,
                "recovered_whole_hash": f"{index:064x}",
            }
            for index in range(1, 23)
        ],
        "auto_increment_before": [
            {"table": "design_assets", "next_value": 24000},
            {"table": "task_assets", "next_value": 25000},
        ],
        "auto_increment_ceilings": [
            {"table": "design_assets", "next_value": 24000},
            {"table": "task_assets", "next_value": 26000},
        ],
        "production_writes_executed": False,
    }
    bundle_journal_value["evidence_sha256"] = hashlib.sha256(
        json.dumps(
            bundle_journal_value,
            ensure_ascii=False,
            sort_keys=True,
            separators=(",", ":"),
        ).encode("utf-8")
    ).hexdigest()
    bundle_journal = write(
        "bundle-db-rollback-journal.json", bundle_journal_value
    )
    bundle_journal_sha = component.sha256_file(bundle_journal)
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
            "rollback_journal_sha256": bundle_journal_sha,
            "rollback_journal_evidence_sha256": bundle_journal_value[
                "evidence_sha256"
            ],
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
                "ownership_receipt_contract_version": 1,
                "artifacts": [
                    component.artifact(path)
                    for path in (
                        bundle_staging_write_ahead,
                        bundle_write_ahead,
                        bundle_materialize,
                        bundle_registry,
                        bundle_before,
                        bundle_provision,
                        bundle_journal,
                        bundle_apply,
                        bundle_idempotent,
                        *bundle_receipts,
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
                "ownership_receipt_contract_version": 1,
                "artifacts": [
                    component.artifact(path)
                    for path in (
                        bundle_journal,
                        bundle_db_rollback,
                        bundle_restore,
                        bundle_file,
                        *bundle_receipts,
                    )
                ],
            }
        ),
    )


class CloneBMaterializationComponentTest(unittest.TestCase):
    def test_manifest_bundle_count_supports_current_dynamic_cohort(self):
        manifest = {
            "bundles": [
                {
                    "task_id": 1000 + index,
                    "scope_kind": "sku",
                    "scope_ref_id": 2000 + index,
                    "revision_no": 1,
                }
                for index in range(70)
            ]
        }
        self.assertEqual(component.manifest_bundle_count(manifest), 70)
        manifest["bundles"].append(dict(manifest["bundles"][0]))
        with self.assertRaisesRegex(
            component.ComponentError, "scope is invalid"
        ):
            component.manifest_bundle_count(manifest)

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
            rollback_journal=pathlib.Path("/tmp/rollback-journal.json"),
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
                if label == "recovery file write-ahead plan":
                    write_ahead = (
                        component_dir / "recovery-file-write-ahead.json"
                    )
                    write_ahead.write_text(
                        json.dumps(
                            {
                                "version": 1,
                                "status": "PREPARED",
                                "run_id": "formal-test",
                            }
                        )
                        + "\n",
                        encoding="utf-8",
                    )
                elif label == "recovery file materialization":
                    plan = component_dir / "recovery-materialization-plan.json"
                    plan.write_text(
                        json.dumps(
                            compact_self({
                                "version": 1,
                                "status": "MATERIALIZED",
                                "run_id": "formal-test",
                            })
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
                elif label == "recovery apply compensation database rollback":
                    plan = component_dir / "recovery-materialization-plan.json"
                    pathlib.Path(
                        argv[argv.index("--report-file") + 1]
                    ).write_text(
                        json.dumps(
                            recovery_report(
                                mode="rollback",
                                run_id="formal-test",
                                plan_sha=component.sha256_file(plan),
                                changed=3,
                                already=0,
                            )
                        )
                        + "\n",
                        encoding="utf-8",
                    )
                elif "compensation" in label:
                    pathlib.Path(
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
                    "recovery file write-ahead plan",
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

    def test_restore_guard_is_idempotent_after_internal_compensation(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            before_path = root / "guard-before.json"
            restore_path = root / "guard-restore.json"
            connection = FakeConnection()
            before = component.self_bound(
                {
                    "schema_version": 1,
                    "kind": "clone-b-guard-state",
                    "component": "recovery",
                    "database": connection.database,
                    "table": component.RECOVERY_GUARD.table,
                    "table_existed": False,
                    "create_table_sql": None,
                    "schema": [],
                    "rows": [],
                }
            )
            component.write_document(before_path, before)
            binding = component.guard_binding(
                component.RECOVERY_GUARD,
                run_id="formal-test",
                primary_sha256="a" * 64,
            )
            with (
                mock.patch.object(
                    component, "capture_guard_state", return_value=before
                ),
                mock.patch.object(
                    component, "_restore_guard_to_before"
                ) as restore,
            ):
                receipt = component.restore_guard(
                    connection,
                    component.RECOVERY_GUARD,
                    binding,
                    before_path,
                    restore_path,
                )
            restore.assert_not_called()
            self.assertTrue(receipt["already_restored"])
            self.assertTrue(receipt["exact"])

    def test_bundle_outer_rollback_accepts_complete_internal_compensation(self):
        with tempfile.TemporaryDirectory() as temporary:
            run_root = pathlib.Path(temporary) / "formal-test"
            component_dir = run_root / "g4"
            fixture = run_root / "fixture-upload-b"
            component_dir.mkdir(parents=True)
            fixture.mkdir()
            manifest = run_root / "manifest.json"
            manifest.write_text(
                json.dumps(
                    {
                        "schema_version": 1,
                        "status": "CONFIRMED",
                        "run_id": "formal-test",
                        "source_candidate_sha256": "b" * 64,
                        "bundles": [
                            {
                                "task_id": 1000 + index,
                                "scope_kind": "sku",
                                "scope_ref_id": 2000 + index,
                                "revision_no": 1,
                            }
                            for index in range(7)
                        ],
                    }
                )
                + "\n",
                encoding="utf-8",
            )
            component.write_document(
                component_dir / "bundle-registry.json",
                component.self_bound(
                    {
                        "schema_version": 1,
                        "status": "MATERIALIZED",
                        "run_id": "formal-test",
                        "manifest_sha256": component.sha256_file(manifest),
                        "entries": [{"index": index} for index in range(7)],
                    }
                ),
            )
            (component_dir / "bundle-guard-before.json").write_text(
                "{}\n", encoding="utf-8"
            )
            component.write_document(
                component_dir / "bundle-apply-compensation-state.json",
                component.self_bound(
                    {
                        "schema_version": 1,
                        "status": "COMPENSATION_COMPLETE",
                        "component": "bundle",
                        "run_id": "formal-test",
                        "database": "ab_formal_b_ui",
                        "database_safe": True,
                        "guard_safe": True,
                        "files_safe": True,
                        "details": [],
                    }
                ),
            )
            args = argparse.Namespace(
                run_id="formal-test",
                manifest=manifest,
                fixture_root=fixture,
            )
            labels: list[str] = []

            def command(argv, *, repo_root, env, label):
                labels.append(label)
                if label == "bundle file rollback":
                    pathlib.Path(
                        argv[argv.index("--report") + 1]
                    ).write_text(
                        json.dumps(
                            {
                                "schema_version": 1,
                                "status": "ROLLED_BACK",
                                "database_write_performed": False,
                            }
                        )
                        + "\n",
                        encoding="utf-8",
                    )

            def restore(*_args):
                path = _args[-1]
                component.write_document(
                    path,
                    component.self_bound(
                        {"schema_version": 1, "status": "RESTORED"}
                    ),
                )

            with (
                mock.patch.object(component, "run_command", side_effect=command),
                mock.patch.object(
                    component, "restore_guard", side_effect=restore
                ),
            ):
                result = component.bundle_rollback(
                    args,
                    FakeConnection(),
                    pathlib.Path("/repo"),
                    run_root,
                    component_dir,
                )
            self.assertEqual(result["status"], "ROLLED_BACK")
            self.assertEqual(labels, ["bundle file rollback"])

    def test_rollback_report_accepts_only_atomic_extremes_when_resuming(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            report = root / "report.json"
            for changed, already in ((3, 0), (0, 3)):
                report.write_text(
                    json.dumps(
                        recovery_report(
                            mode="rollback",
                            run_id="formal-test",
                            plan_sha="a" * 64,
                            changed=changed,
                            already=already,
                        )
                    )
                    + "\n",
                    encoding="utf-8",
                )
                component.validate_recovery_report(
                    report,
                    mode="rollback",
                    run_id="formal-test",
                    database="ab_formal_b_ui",
                    host="127.0.0.1",
                    plan_sha256="a" * 64,
                    changed=3,
                    already=0,
                    allowed_counts={(3, 0), (0, 3)},
                )

            report.write_text(
                json.dumps(
                    recovery_report(
                        mode="rollback",
                        run_id="formal-test",
                        plan_sha="a" * 64,
                        changed=1,
                        already=2,
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
                    mode="rollback",
                    run_id="formal-test",
                    database="ab_formal_b_ui",
                    host="127.0.0.1",
                    plan_sha256="a" * 64,
                    changed=3,
                    already=0,
                    allowed_counts={(3, 0), (0, 3)},
                )

    def test_bundle_rollback_accepts_only_atomic_extremes_when_resuming(self):
        with tempfile.TemporaryDirectory() as temporary:
            report = pathlib.Path(temporary) / "report.json"
            for changed, already in ((7, 0), (0, 7)):
                report.write_text(
                    json.dumps(
                        bundle_report(changed=changed, already=already)
                    )
                    + "\n",
                    encoding="utf-8",
                )
                component.validate_bundle_report(
                    report,
                    mode="rollback",
                    run_id="formal-test",
                    database="ab_formal_b_ui",
                    host="127.0.0.1",
                    candidate_sha256="a" * 64,
                    registry_sha256="b" * 64,
                    manifest_sha256="c" * 64,
                    rollback_journal_sha256="d" * 64,
                    rollback_journal_evidence_sha256="e" * 64,
                    changed=7,
                    already=0,
                    allowed_counts={(7, 0), (0, 7)},
                )

            report.write_text(
                json.dumps(bundle_report(changed=2, already=5)) + "\n",
                encoding="utf-8",
            )
            with self.assertRaisesRegex(
                component.ComponentError, "report contract"
            ):
                component.validate_bundle_report(
                    report,
                    mode="rollback",
                    run_id="formal-test",
                    database="ab_formal_b_ui",
                    host="127.0.0.1",
                    candidate_sha256="a" * 64,
                    registry_sha256="b" * 64,
                    manifest_sha256="c" * 64,
                    rollback_journal_sha256="d" * 64,
                    rollback_journal_evidence_sha256="e" * 64,
                    changed=7,
                    already=0,
                    allowed_counts={(7, 0), (0, 7)},
                )

    def test_bundle_registry_apply_contract_is_verified_before_database(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary).resolve()
            component_dir = root / "runs" / "hold-open"
            component_dir.mkdir(parents=True)
            b_root = root / "fixture-upload-b"
            (b_root / "objects").mkdir(parents=True)
            run_id = "bundle-materialization-formal-test"
            write_ahead = component_dir / "bundle-file-write-ahead.json"
            write_ahead.write_bytes(
                component.canonical_bytes(
                    component.self_bound(
                        {
                            "schema_version": 1,
                            "status": "WRITE_AHEAD",
                            "run_id": run_id,
                        }
                    )
                )
            )
            entries = []
            target_by_asset_id = {}
            content_by_asset_id = {}
            ownership_bytes = {}
            for asset_id in range(25557, 25564):
                content = f"bundle-{asset_id}".encode()
                digest = hashlib.sha256(content).hexdigest()
                relative = pathlib.PurePosixPath(
                    "objects", f"bundle-{asset_id}.zip"
                )
                target = b_root.joinpath(*relative.parts)
                target.write_bytes(content)
                target_by_asset_id[asset_id] = target
                content_by_asset_id[asset_id] = content
                stat = target.stat()
                expected_stage = (
                    component_dir / f".bundle-stage-{asset_id}.zip"
                ).resolve()
                expected_private = (
                    component_dir / f".bundle-private-{asset_id}.zip"
                ).resolve()
                ownership = component_dir / (
                    f"bundle-ownership-{asset_id}.json"
                )
                ownership_value = component.self_bound(
                    {
                        "schema_version": 1,
                        "status": "OWNED_LINK",
                        "run_id": run_id,
                        "target_path": str(target.resolve()),
                        "staging_path": str(expected_stage),
                        "device": stat.st_dev,
                        "inode": stat.st_ino,
                        "sha256": digest,
                        "size": len(content),
                    }
                )
                ownership_bytes[asset_id] = component.canonical_bytes(
                    ownership_value
                )
                ownership.write_bytes(ownership_bytes[asset_id])
                staging = component_dir / (
                    f"bundle-staging-ownership-{asset_id}.json"
                )
                staging.write_bytes(
                    component.canonical_bytes(
                        component.self_bound(
                            {
                                "schema_version": 1,
                                "status": "STAGING_OWNED",
                                "run_id": run_id,
                                "staging_path": str(expected_stage),
                                "private_path": str(expected_private),
                                "sha256": digest,
                                "size": len(content),
                            }
                        )
                    )
                )
                entries.append(
                    {
                        "disposition": "created",
                        "bundle_sha256": digest,
                        "size": len(content),
                        "relative_object_path": relative.as_posix(),
                        "task_asset_candidate": {"id": asset_id},
                        "rollback_candidate": {
                            "task_asset_id": asset_id,
                            "ownership_receipt_path": str(
                                ownership.resolve()
                            ),
                        },
                    }
                )
            unsigned = {
                "schema_version": 1,
                "status": "MATERIALIZED",
                "run_id": run_id,
                "database_write_performed": False,
                "b_root": str(b_root),
                "write_ahead_sha256": component.sha256_file(write_ahead),
                "entries": entries,
            }
            registry_value = component.self_bound(unsigned)
            registry = component_dir / "bundle-registry.json"
            registry.write_bytes(component.canonical_bytes(registry_value))

            receipts = component.validate_bundle_registry_for_apply(
                registry_value,
                registry=registry,
                write_ahead=write_ahead,
                component_dir=component_dir,
                run_id=run_id,
                expected_bundle_count=7,
            )
            self.assertEqual(14, len(receipts))

            stale = dict(registry_value)
            stale["status"] = "MATERIALIZEd"
            with self.assertRaisesRegex(component.ComponentError, "self hash"):
                component.validate_bundle_registry_for_apply(
                    stale,
                    registry=registry,
                    write_ahead=write_ahead,
                    component_dir=component_dir,
                    run_id=run_id,
                    expected_bundle_count=7,
                )

            for recorded in ("", str(component_dir / "outside.json")):
                missing = json.loads(json.dumps(unsigned))
                missing["entries"][0]["rollback_candidate"][
                    "ownership_receipt_path"
                ] = recorded
                with self.assertRaisesRegex(
                    component.ComponentError,
                    "ownership receipt path differs",
                ):
                    component.validate_bundle_registry_for_apply(
                        component.self_bound(missing),
                        registry=registry,
                        write_ahead=write_ahead,
                        component_dir=component_dir,
                        run_id=run_id,
                        expected_bundle_count=7,
                    )

            reused = json.loads(json.dumps(unsigned))
            reused["entries"][0]["disposition"] = "reused_identical"
            reused["entries"][0]["rollback_candidate"][
                "ownership_receipt_path"
            ] = ""
            first_asset_id = 25557
            first_ownership = (
                component_dir
                / f"bundle-ownership-{first_asset_id}.json"
            )
            first_ownership.unlink()
            component.validate_bundle_registry_for_apply(
                component.self_bound(reused),
                registry=registry,
                write_ahead=write_ahead,
                component_dir=component_dir,
                run_id=run_id,
                expected_bundle_count=7,
            )
            first_ownership.write_bytes(ownership_bytes[first_asset_id])

            first_target = target_by_asset_id[first_asset_id]
            replacement = first_target.with_suffix(".replacement")
            replacement.write_bytes(content_by_asset_id[first_asset_id])
            self.assertNotEqual(
                first_target.stat().st_ino,
                replacement.stat().st_ino,
            )
            os.replace(replacement, first_target)
            with self.assertRaisesRegex(
                component.ComponentError,
                "ownership receipt identity differs",
            ):
                component.validate_bundle_registry_for_apply(
                    registry_value,
                    registry=registry,
                    write_ahead=write_ahead,
                    component_dir=component_dir,
                    run_id=run_id,
                    expected_bundle_count=7,
                )


if __name__ == "__main__":
    unittest.main()
