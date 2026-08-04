import importlib.util
import hashlib
import json
import os
import pathlib
import socket
import sys
import tempfile
import time
import types
import unittest
from unittest import mock


PATH = pathlib.Path(__file__).with_name("run_g4_clone_rehearsal.py")
SPEC = importlib.util.spec_from_file_location("run_g4_clone_rehearsal", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)

FINALIZER_PATH = pathlib.Path(__file__).with_name("finalize_release_gates.py")
FINALIZER_SPEC = importlib.util.spec_from_file_location(
    "finalize_release_gates_for_g4_test", FINALIZER_PATH
)
FINALIZER = importlib.util.module_from_spec(FINALIZER_SPEC)
sys.modules[FINALIZER_SPEC.name] = FINALIZER
FINALIZER_SPEC.loader.exec_module(FINALIZER)


HELPER = r"""
import hashlib
import json
import os
import pathlib
import sys

if os.environ.get("AB_CONFIRMED_CLONE_SIDE") != "B":
    raise SystemExit(31)
if not os.environ.get("MYSQL_DSN", "").find("@tcp(127.0.0.1:") >= 0:
    raise SystemExit(32)
auth_path = pathlib.Path(os.environ.get("AUTH_SETTINGS_FILE", ""))
frontend_access_path = pathlib.Path(
    os.environ.get("FRONTEND_ACCESS_SETTINGS_FILE", "")
)
if (
    not auth_path.is_file()
    or auth_path.name != "auth_identity.clone-b.json"
    or not frontend_access_path.is_file()
    or frontend_access_path.name != "frontend_access.json"
):
    raise SystemExit(33)
for name in (
    "AUTH_ALLOW_EMBEDDED_SETTINGS",
    "AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS",
    "WEB_PUSH_ENABLED",
    "AI_AGENT_ENABLED",
    "AI_CHAT_ENABLED",
    "AI_EMBEDDING_ENABLED",
    "VECTOR_SEARCH_ENABLED",
    "AI_RETRIEVAL_WORKER_ENABLED",
):
    if os.environ.get(name) != "false":
        raise SystemExit(34)
mode = sys.argv[1]
if mode == "workflow":
    args = sys.argv[2:]
    if "--unsafe-workflow-rollback" in args and "--rollback" in args:
        raise SystemExit(121)
    if "--fail-workflow-rollback" in args and "--rollback" in args:
        raise SystemExit(7)
    if "--report-file" in args:
        report = pathlib.Path(args[args.index("--report-file") + 1])
        report.write_text(json.dumps({"status": "PASS"}) + "\n", encoding="utf-8")
    raise SystemExit(0)
name, output = sys.argv[2], pathlib.Path(sys.argv[3])
if "--fail" in sys.argv:
    raise SystemExit(7)
if "--unsafe-timeout" in sys.argv:
    raise SystemExit(121)
if name == "capture_baseline_fingerprint":
    tables = {
        "tasks": {
            "row_count": 2,
            "content_sha256": "4" * 64,
            "schema_sha256": "5" * 64,
            "auto_increment": 3,
            "content_fingerprint_algorithm": (
                "sha256(sorted(sha256(canonical-json-cells-v1)),"
                "duplicates-preserved)-v1"
            ),
        }
    }
    value = {
        "schema_version": 1,
        "kind": "clone-b-baseline-fingerprint",
        "database": "ab_formal_b_ui",
        "fingerprint_algorithm": (
            "sha256(sorted(sha256(canonical-json-cells-v1)),"
            "duplicates-preserved)-v1"
        ),
        "tables": tables,
        "fingerprint_sha256": hashlib.sha256(
            (json.dumps(tables, sort_keys=True, separators=(",", ":")) + "\n").encode()
        ).hexdigest(),
    }
elif name == "search_snapshot":
    archive_path = pathlib.Path(sys.argv[4])
    archive_path.write_bytes(b'{"table":"task_search_documents"}\n')
    archive_sha = hashlib.sha256(archive_path.read_bytes()).hexdigest()
    tables = {
        "task_search_documents": {"row_count": 2, "content_sha256": "1" * 64},
        "task_asset_group_search_documents": {"row_count": 3, "content_sha256": "2" * 64},
        "product_search_documents": {"row_count": 4, "content_sha256": "3" * 64},
    }
    digest = hashlib.sha256(
        (json.dumps(tables, sort_keys=True, separators=(",", ":")) + "\n").encode()
    ).hexdigest()
    value = {
        "schema_version": 1,
        "status": "CAPTURED",
        "violation_count": 0,
        "tables": tables,
        "snapshot_sha256": digest,
        "archive": {
            "format": "deterministic-jsonl-v1",
            "sha256": archive_sha,
            "size": archive_path.stat().st_size,
        },
    }
elif name == "search_rollback":
    snapshot = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
    value = {
        "schema_version": 1,
        "status": "PASS",
        "violation_count": 0,
        "snapshot_sha256": snapshot["snapshot_sha256"],
        "restored_snapshot_sha256": snapshot["snapshot_sha256"],
        "restored_tables": snapshot["tables"],
        "source_archive_sha256": snapshot["archive"]["sha256"],
    }
elif name == "validate_after_rollback_fingerprint":
    baseline_path = pathlib.Path(sys.argv[4])
    baseline = json.loads(baseline_path.read_text(encoding="utf-8"))
    value = {
        "schema_version": 1,
        "status": "PASS",
        "violation_count": 0,
        "baseline_artifact_sha256": hashlib.sha256(
            baseline_path.read_bytes()
        ).hexdigest(),
        "baseline_fingerprint_sha256": baseline["fingerprint_sha256"],
        "rollback_fingerprint_sha256": baseline["fingerprint_sha256"],
    }
elif name == "recovery_apply":
    sys.path.insert(0, os.getcwd())
    from scripts.ab.test_clone_b_materialization_component import (
        write_component_chain_fixture,
    )
    write_component_chain_fixture(
        output.parent,
        run_id="formal-test-run",
        database="ab_formal_b_ui",
    )
    value = {"schema_version": 1, "status": "PASS", "step": name}
else:
    value = {"schema_version": 1, "status": "PASS", "step": name}
output.write_bytes(
    (
        json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n"
    ).encode("utf-8")
)
"""


class RunG4CloneRehearsalTest(unittest.TestCase):
    @unittest.skipUnless(
        sys.platform.startswith("linux"),
        "process-group quiescence evidence requires Linux /proc",
    )
    def test_execute_step_timeout_releases_ignoring_descendant_socket(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            clone_root = root / "clone"
            run_dir = clone_root / "run"
            run_dir.mkdir(parents=True)
            child_port = root / "child.port"
            helper = root / "timeout_parent.py"
            helper.write_text(
                "import pathlib,subprocess,sys,time\n"
                "child=subprocess.Popen([sys.executable,'-c',"
                "'import pathlib,signal,socket,sys,time;"
                "signal.signal(signal.SIGTERM,signal.SIG_IGN);"
                "sock=socket.socket();sock.bind((\"127.0.0.1\",0));sock.listen();"
                "pathlib.Path(sys.argv[1]).write_text(str(sock.getsockname()[1]),encoding=\"utf-8\");"
                "time.sleep(60)',sys.argv[1]])\n"
                "deadline=time.monotonic()+2\n"
                "while not pathlib.Path(sys.argv[1]).exists():\n"
                "  if time.monotonic()>deadline: raise RuntimeError('child socket not ready')\n"
                "  time.sleep(.01)\n"
                "time.sleep(60)\n",
                encoding="utf-8",
            )
            with (
                mock.patch.object(
                    MODULE, "PROCESS_GROUP_TERM_GRACE_SECONDS", 0.1
                ),
                mock.patch.object(
                    MODULE, "PROCESS_GROUP_KILL_GRACE_SECONDS", 1.0
                ),
            ):
                record = MODULE.execute_step(
                    step="timeout_probe",
                    phase="apply",
                    argv=[sys.executable, str(helper), str(child_port)],
                    expected_artifacts=[],
                    run_dir=run_dir,
                    clone_root=clone_root,
                    repo_root=root,
                    env=dict(os.environ),
                    timeout_seconds=0.5,
                )
            self.assertEqual(record["exit_code"], 124)
            port = int(child_port.read_text(encoding="utf-8"))
            with socket.socket() as probe:
                probe.settimeout(0.2)
                self.assertNotEqual(probe.connect_ex(("127.0.0.1", port)), 0)
            self.assertIn(
                "verified zero non-zombie process-group members before rollback",
                (run_dir / "timeout_probe.stderr").read_text(encoding="utf-8"),
            )

    @unittest.skipUnless(
        sys.platform.startswith("linux"),
        "process-group quiescence evidence requires Linux /proc",
    )
    def test_execute_step_timeout_releases_ignoring_direct_child_socket(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            clone_root = root / "clone"
            run_dir = clone_root / "run"
            run_dir.mkdir(parents=True)
            port_file = root / "direct.port"
            helper = root / "timeout_direct.py"
            helper.write_text(
                "import pathlib,signal,socket,sys,time\n"
                "signal.signal(signal.SIGTERM,signal.SIG_IGN)\n"
                "sock=socket.socket();sock.bind(('127.0.0.1',0));sock.listen()\n"
                "pathlib.Path(sys.argv[1]).write_text(str(sock.getsockname()[1]),encoding='utf-8')\n"
                "time.sleep(60)\n",
                encoding="utf-8",
            )
            with (
                mock.patch.object(
                    MODULE, "PROCESS_GROUP_TERM_GRACE_SECONDS", 0.1
                ),
                mock.patch.object(
                    MODULE, "PROCESS_GROUP_KILL_GRACE_SECONDS", 1.0
                ),
            ):
                record = MODULE.execute_step(
                    step="timeout_direct_probe",
                    phase="apply",
                    argv=[sys.executable, str(helper), str(port_file)],
                    expected_artifacts=[],
                    run_dir=run_dir,
                    clone_root=clone_root,
                    repo_root=root,
                    env=dict(os.environ),
                    timeout_seconds=0.5,
                )
            self.assertEqual(record["exit_code"], 124)
            port = int(port_file.read_text(encoding="utf-8"))
            with socket.socket() as probe:
                probe.settimeout(0.2)
                self.assertNotEqual(probe.connect_ex(("127.0.0.1", port)), 0)

    def test_repository_example_plan_matches_full_hook_contract(self):
        example = pathlib.Path(__file__).with_name(
            "g4-command-plan.example.json"
        )
        plan = MODULE.validate_plan(example)
        self.assertEqual(set(plan["hooks"]), MODULE.HOOKS)
        self.assertIn(
            "./cmd/tools/workflow-groups-migrate",
            plan["workflow_base_argv"],
        )

    def make_inputs(
        self,
        root,
        *,
        fail_hook=None,
        fail_workflow_rollback=False,
        unsafe_hook=None,
        unsafe_workflow_rollback=False,
        dsn_host="127.0.0.1",
    ):
        root = pathlib.Path(root)
        clone_root = root / "formal-test-run"
        clone_root.mkdir()
        auth_settings = clone_root / "auth_identity.clone-b.seed.json"
        auth_settings.write_text(
            json.dumps(
                {
                    "departments": MODULE.CLONE_B_AUTH_DEPARTMENTS,
                    "department_teams": MODULE.CLONE_B_AUTH_DEPARTMENT_TEAMS,
                    "phone_unique": True,
                    "department_admin_keys": {},
                    "super_admins": [],
                    "unassigned_pool_enabled": True,
                    "configured_user_assignments": [],
                    "task_team_mappings": {},
                },
                ensure_ascii=False,
                sort_keys=True,
            )
            + "\n",
            encoding="utf-8",
        )
        auth_settings.chmod(0o440)
        helper = root / "helper.py"
        helper.write_text(HELPER, encoding="utf-8")
        mapping = root / "mapping.json"
        mapping.write_text('{"version":2}\n', encoding="utf-8")
        dsn = root / "dsn.txt"
        dsn.write_text(
            f"user:secret@tcp({dsn_host}:3307)/ab_formal_b_ui?parseTime=true\n",
            encoding="utf-8",
        )
        run_dir = clone_root / "g4"
        hooks = {}
        for name in sorted(MODULE.HOOKS):
            output = (
                "{rollback_fingerprint}"
                if name == "validate_after_rollback_fingerprint"
                else "{baseline_fingerprint}"
                if name == "capture_baseline_fingerprint"
                else "{search_snapshot}"
                if name == "search_snapshot"
                else "{search_rollback}"
                if name == "search_rollback"
                else f"{{run_dir}}/{name}.json"
            )
            argv = [
                sys.executable,
                str(helper),
                "hook",
                name,
                output,
            ]
            if name == "search_rollback":
                argv.extend(["{search_snapshot}", "{search_snapshot_archive}"])
            if name == "validate_after_rollback_fingerprint":
                argv.append("{baseline_fingerprint}")
            if name == "search_snapshot":
                argv.append("{search_snapshot_archive}")
            if name == fail_hook:
                argv.append("--fail")
            if name == unsafe_hook:
                argv.append("--unsafe-timeout")
            hooks[name] = {
                "argv": argv,
                "expected_artifacts": (
                    [output, "{search_snapshot_archive}"]
                    if name == "search_snapshot"
                    else [output]
                ),
            }
        plan = root / "command-plan.json"
        plan.write_text(
            json.dumps(
                {
                    "schema_version": 1,
                    "workflow_base_argv": [
                        sys.executable,
                        str(helper),
                        "workflow",
                    ]
                    + (
                        ["--fail-workflow-rollback"]
                        if fail_workflow_rollback
                        else []
                    )
                    + (
                        ["--unsafe-workflow-rollback"]
                        if unsafe_workflow_rollback
                        else []
                    ),
                    "hooks": hooks,
                }
            )
            + "\n",
            encoding="utf-8",
        )
        args = types.SimpleNamespace(
            run_id="formal-test-run",
            run_dir=run_dir,
            clone_root=clone_root.resolve(),
            confirm_clone_database="ab_formal_b_ui",
            dsn_file=dsn,
            mapping_file=mapping,
            command_plan=plan,
            auth_settings_file=auth_settings.resolve(),
            execute_clone_writes=True,
            max_step_seconds=30.0,
            max_phase_seconds=600.0,
            max_total_seconds=1800.0,
        )
        return args

    def test_full_chain_passes_and_hashes_raw_artifacts(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            report = MODULE.run(args)
            self.assertEqual(report["status"], "PASS")
            self.assertEqual(
                FINALIZER.validate_g4(report, args.clone_root), []
            )
            self.assertEqual(
                [row["step"] for row in report["steps"]],
                [name for name, _ in MODULE.summarize_g4.REQUIRED_STEPS],
            )
            names = [row["step"] for row in report["steps"]]
            self.assertLess(
                names.index("capture_baseline_fingerprint"),
                names.index("recovery_apply"),
            )
            self.assertLess(
                names.index("recovery_apply"), names.index("bundle_apply")
            )
            self.assertLess(
                names.index("bundle_apply"), names.index("dry_run_before")
            )
            self.assertLess(
                names.index("dry_run_before"), names.index("workflow_apply")
            )
            evidence = json.loads(
                (args.run_dir / "evidence.sha256.json").read_text(
                    encoding="utf-8"
                )
            )
            self.assertEqual(evidence["status"], "PASS")
            self.assertEqual(evidence["database_host_class"], "local")
            self.assertEqual(
                evidence["auth_settings_sha256"],
                report["input_sha256"]["auth_settings"],
            )
            self.assertEqual(
                evidence["frontend_access_settings_sha256"],
                report["input_sha256"]["frontend_access_settings"],
            )
            self.assertEqual(
                report["auth_settings_attestation"]["super_admin_count"], 0
            )
            self.assertTrue(report["auth_settings_attestation"]["read_only"])
            self.assertGreater(len(evidence["raw_evidence"]), 22)
            self.assertNotIn(
                "secret",
                (args.run_dir / "commands.jsonl").read_text(encoding="utf-8"),
            )

    def test_auth_settings_must_match_frozen_zero_secret_policy(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            payload = json.loads(
                args.auth_settings_file.read_text(encoding="utf-8")
            )
            payload["super_admins"] = ["unexpected"]
            args.auth_settings_file.chmod(0o640)
            args.auth_settings_file.write_text(
                json.dumps(payload, ensure_ascii=False) + "\n",
                encoding="utf-8",
            )
            args.auth_settings_file.chmod(0o440)
            with self.assertRaisesRegex(ValueError, "zero-secret"):
                MODULE.run(args)
            self.assertFalse(args.run_dir.exists())

    def test_auth_settings_rejects_numeric_boolean_confusion(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            payload = json.loads(
                args.auth_settings_file.read_text(encoding="utf-8")
            )
            payload["phone_unique"] = 1
            args.auth_settings_file.chmod(0o640)
            args.auth_settings_file.write_text(
                json.dumps(payload, ensure_ascii=False) + "\n",
                encoding="utf-8",
            )
            args.auth_settings_file.chmod(0o440)
            with self.assertRaisesRegex(ValueError, "zero-secret"):
                MODULE.run(args)
            self.assertFalse(args.run_dir.exists())

    def test_failed_apply_runs_reverse_cleanup_and_blocks(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw, fail_hook="bundle_apply")
            report = MODULE.run(args)
            self.assertEqual(report["status"], "BLOCKED")
            by_name = {row["step"]: row for row in report["steps"]}
            self.assertEqual(by_name["bundle_apply"]["exit_code"], 7)
            self.assertEqual(by_name["dry_run_before"]["exit_code"], 125)
            self.assertEqual(by_name["workflow_apply"]["exit_code"], 125)
            self.assertEqual(by_name["workflow_rollback"]["exit_code"], 125)
            self.assertEqual(by_name["bundle_rollback"]["exit_code"], 0)
            self.assertEqual(by_name["recovery_rollback"]["exit_code"], 0)
            self.assertEqual(
                by_name["validate_after_rollback_fingerprint"]["exit_code"], 0
            )

    def test_failed_reindex_restores_search_before_workflow_rollback(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw, fail_hook="search_reindex")
            report = MODULE.run(args)
            self.assertEqual(report["status"], "BLOCKED")
            by_name = {row["step"]: row for row in report["steps"]}
            self.assertEqual(by_name["search_snapshot"]["exit_code"], 0)
            self.assertEqual(by_name["search_reindex"]["exit_code"], 7)
            self.assertEqual(by_name["workflow_rollback"]["exit_code"], 0)
            self.assertEqual(by_name["search_rollback"]["exit_code"], 0)
            names = [row["step"] for row in report["steps"]]
            self.assertLess(
                names.index("search_rollback"), names.index("workflow_rollback")
            )
            self.assertLess(
                names.index("workflow_rollback"), names.index("bundle_rollback")
            )

    def test_failed_search_rollback_continues_independent_cleanup(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw, fail_hook="search_rollback")
            report = MODULE.run(args)
            by_name = {row["step"]: row for row in report["steps"]}
            self.assertEqual(by_name["search_rollback"]["exit_code"], 7)
            for step in (
                "workflow_rollback",
                "bundle_rollback",
                "recovery_rollback",
            ):
                self.assertEqual(by_name[step]["exit_code"], 0)
            self.assertEqual(
                by_name["validate_after_rollback_fingerprint"]["exit_code"], 0
            )

    def test_failed_workflow_rollback_continues_bundle_and_recovery(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw, fail_workflow_rollback=True)
            report = MODULE.run(args)
            by_name = {row["step"]: row for row in report["steps"]}
            self.assertEqual(by_name["search_rollback"]["exit_code"], 0)
            self.assertEqual(by_name["workflow_rollback"]["exit_code"], 7)
            self.assertEqual(by_name["bundle_rollback"]["exit_code"], 0)
            self.assertEqual(by_name["recovery_rollback"]["exit_code"], 0)
            self.assertEqual(
                by_name["validate_after_rollback_fingerprint"]["exit_code"], 0
            )

    def test_failed_bundle_rollback_still_runs_recovery(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw, fail_hook="bundle_rollback")
            report = MODULE.run(args)
            by_name = {row["step"]: row for row in report["steps"]}
            self.assertEqual(by_name["search_rollback"]["exit_code"], 0)
            self.assertEqual(by_name["workflow_rollback"]["exit_code"], 0)
            self.assertEqual(by_name["bundle_rollback"]["exit_code"], 7)
            self.assertEqual(by_name["recovery_rollback"]["exit_code"], 0)
            self.assertEqual(
                by_name["validate_after_rollback_fingerprint"]["exit_code"], 0
            )

    def test_unquiesced_apply_timeout_skips_all_database_cleanup(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw, unsafe_hook="search_reindex")
            report = MODULE.run(args)
            by_name = {row["step"]: row for row in report["steps"]}
            self.assertEqual(by_name["search_reindex"]["exit_code"], 121)
            for step in (
                "search_rollback",
                "workflow_rollback",
                "bundle_rollback",
                "recovery_rollback",
                "validate_after_rollback_fingerprint",
            ):
                self.assertEqual(by_name[step]["exit_code"], 125)

    def test_unquiesced_rollback_timeout_skips_remaining_database_work(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw, unsafe_workflow_rollback=True)
            report = MODULE.run(args)
            by_name = {row["step"]: row for row in report["steps"]}
            self.assertEqual(by_name["search_rollback"]["exit_code"], 0)
            self.assertEqual(by_name["workflow_rollback"]["exit_code"], 121)
            for step in (
                "bundle_rollback",
                "recovery_rollback",
                "validate_after_rollback_fingerprint",
            ):
                self.assertEqual(by_name[step]["exit_code"], 125)

    def test_nonlocal_dsn_is_rejected_before_run_directory_exists(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw, dsn_host="prod.example.com")
            with self.assertRaisesRegex(ValueError, "DSN must use"):
                MODULE.run(args)
            self.assertFalse(args.run_dir.exists())

    def test_dsn_without_parse_time_is_rejected_before_run_directory_exists(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            args.dsn_file.write_text(
                "user:secret@tcp(127.0.0.1:3307)/ab_formal_b_ui\n",
                encoding="utf-8",
            )
            with self.assertRaisesRegex(ValueError, "parseTime=true"):
                MODULE.run(args)
            self.assertFalse(args.run_dir.exists())

    def test_dsn_with_disabled_parse_time_is_rejected(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            args.dsn_file.write_text(
                "user:secret@tcp(127.0.0.1:3307)/ab_formal_b_ui"
                "?parseTime=false\n",
                encoding="utf-8",
            )
            with self.assertRaisesRegex(ValueError, "parseTime=true"):
                MODULE.run(args)
            self.assertFalse(args.run_dir.exists())

    def test_missing_go_fails_before_creating_run_evidence(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            with mock.patch.object(MODULE.shutil, "which", return_value=None):
                with self.assertRaisesRegex(ValueError, "go executable"):
                    MODULE.run(args)
            self.assertFalse(args.run_dir.exists())

    def test_command_plan_cannot_embed_a_dsn(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            plan = json.loads(args.command_plan.read_text(encoding="utf-8"))
            plan["hooks"]["search_reindex"]["argv"].extend(
                ["--dsn", "user:pass@tcp(prod:3306)/workflow"]
            )
            args.command_plan.write_text(json.dumps(plan), encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "MYSQL_DSN"):
                MODULE.run(args)
            self.assertFalse(args.run_dir.exists())

    def test_final_compare_must_consume_run_scoped_baseline(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            plan = json.loads(args.command_plan.read_text(encoding="utf-8"))
            plan["hooks"]["validate_after_rollback_fingerprint"]["argv"][-1] = (
                "{clone_root}/unbound-baseline.json"
            )
            args.command_plan.write_text(json.dumps(plan), encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "same.*baseline"):
                MODULE.run(args)
            self.assertFalse(args.run_dir.exists())

    def test_baseline_failure_prevents_every_clone_b_apply(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(
                raw, fail_hook="capture_baseline_fingerprint"
            )
            report = MODULE.run(args)
            self.assertEqual(report["status"], "BLOCKED")
            by_name = {row["step"]: row for row in report["steps"]}
            self.assertEqual(
                by_name["capture_baseline_fingerprint"]["exit_code"], 7
            )
            for name in (
                "recovery_apply",
                "bundle_apply",
                "dry_run_before",
                "workflow_apply",
                "idempotent_apply",
                "search_reindex",
            ):
                self.assertEqual(by_name[name]["exit_code"], 125)


if __name__ == "__main__":
    unittest.main()
