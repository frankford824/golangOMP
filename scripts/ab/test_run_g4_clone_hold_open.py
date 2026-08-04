import argparse
import hashlib
import importlib.util
import json
import pathlib
import sys
import tempfile
import unittest
from unittest import mock


PATH = pathlib.Path(__file__).with_name("run_g4_clone_hold_open.py")
SPEC = importlib.util.spec_from_file_location("run_g4_clone_hold_open", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class HoldOpenCoordinatorTest(unittest.TestCase):
    def make_inputs(self, raw: str) -> argparse.Namespace:
        root = pathlib.Path(raw)
        run_id = "formal-test-run"
        clone_root = root / run_id
        clone_root.mkdir()
        run_dir = clone_root / "runs" / "hold-001"
        auth = clone_root / "auth_identity.clone-b.seed.json"
        auth.write_text(
            json.dumps(
                {
                    "departments": MODULE.g4.CLONE_B_AUTH_DEPARTMENTS,
                    "department_teams": MODULE.g4.CLONE_B_AUTH_DEPARTMENT_TEAMS,
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
        auth.chmod(0o440)
        mapping = root / "mapping.json"
        mapping.write_text('{"version":2}\n', encoding="utf-8")
        recovery_source = root / "recovery-source.bin"
        recovery_source.write_bytes(b"frozen recovery bytes")
        recovery_value = {
            "version": 1,
            "run_id": run_id,
            "recoveries": [
                {
                    "source_task_asset_id": 24034,
                    "source_local_path": str(recovery_source.resolve()),
                    "source_size": recovery_source.stat().st_size,
                    "source_sha256": MODULE.g4.sha256_file(recovery_source),
                }
            ],
        }
        recovery_value[MODULE.EVIDENCE_HASH_FIELD] = MODULE.compact_canonical_hash(
            recovery_value
        )
        recovery_evidence = root / "recovery-evidence.json"
        recovery_evidence.write_bytes(MODULE.g4.canonical_bytes(recovery_value))
        baseline_tables = {"tasks": {"row_count": 1, "content_sha256": "b" * 64}}
        baseline = root / "expected-baseline.json"
        baseline.write_bytes(
            MODULE.g4.canonical_bytes(
                {
                    "schema_version": 1,
                    "kind": "clone-b-baseline-fingerprint",
                    "database": "ab_formal_b_ui",
                    "fingerprint_algorithm": "test-fingerprint-v1",
                    "tables": baseline_tables,
                    "fingerprint_sha256": MODULE.canonical_hash(baseline_tables),
                }
            )
        )
        dsn = root / "clone-b.dsn"
        dsn.write_text(
            "user:secret@tcp(127.0.0.1:3306)/ab_formal_b_ui"
            "?parseTime=true\n",
            encoding="utf-8",
        )
        hooks = {}
        expected = {
            "capture_baseline_fingerprint": ["{baseline_fingerprint}"],
            "recovery_apply": [
                "{run_dir}/recovery-file-write-ahead.json",
                "{run_dir}/recovery-materialization-plan.json",
                "{run_dir}/recovery-guard-before.json",
            ],
            "bundle_apply": [
                "{run_dir}/bundle-staging-write-ahead.json",
                "{run_dir}/bundle-file-write-ahead.json",
                "{run_dir}/bundle-guard-before.json",
            ],
            "search_snapshot": [
                "{search_snapshot}",
                "{search_snapshot_archive}",
            ],
            "search_reindex": ["{run_dir}/search-reindex.json"],
            "search_rollback": ["{search_rollback}"],
            "bundle_rollback": ["{run_dir}/bundle-rollback.json"],
            "recovery_rollback": ["{run_dir}/recovery-rollback.json"],
            "validate_after_rollback_fingerprint": [
                "{rollback_fingerprint}"
            ],
        }
        for name in MODULE.g4.HOOKS:
            argv = ["mock", name]
            if name == "validate_after_rollback_fingerprint":
                argv.append("{baseline_fingerprint}")
            hooks[name] = {
                "argv": argv,
                "expected_artifacts": expected[name],
            }
        plan = root / "command-plan.json"
        plan.write_text(
            json.dumps(
                {
                    "schema_version": 1,
                    "workflow_base_argv": ["mock", "workflow"],
                    "hooks": hooks,
                },
                sort_keys=True,
            )
            + "\n",
            encoding="utf-8",
        )
        return argparse.Namespace(
            phase="apply-and-hold",
            run_id=run_id,
            run_dir=run_dir,
            clone_root=clone_root.resolve(),
            confirm_clone_database="ab_formal_b_ui",
            dsn_file=dsn.resolve(),
            mapping_file=mapping.resolve(),
            recovery_evidence_file=recovery_evidence.resolve(),
            recovery_source_root=recovery_source.parent.resolve(),
            command_plan=plan.resolve(),
            auth_settings_file=auth.resolve(),
            expected_baseline_file=baseline.resolve(),
            g5_evidence_manifest=None,
            g6_evidence_manifest=None,
            execute_clone_writes=True,
            confirm_interrupted_step_quiescent=False,
            max_step_seconds=30.0,
        )

    def fake_execute(
        self,
        calls: list[str],
        *,
        fail_step: str | None = None,
    ):
        def execute(
            *,
            step,
            phase,
            argv,
            expected_artifacts,
            run_dir,
            clone_root,
            repo_root,
            env,
            timeout_seconds,
            require_quiescence=False,
        ):
            del repo_root, timeout_seconds
            self.assertTrue(require_quiescence)
            self.assertEqual(env["AB_CONFIRMED_CLONE_SIDE"], "B")
            self.assertEqual(
                env["AB_CONFIRMED_CLONE_DATABASE"], "ab_formal_b_ui"
            )
            calls.append(step)
            command = {"step": step, "argv": argv}
            command_sha = hashlib.sha256(
                MODULE.g4.canonical_bytes(command)
            ).hexdigest()
            with (run_dir / "commands.jsonl").open("ab") as handle:
                handle.write(
                    MODULE.g4.canonical_bytes(
                        {**command, "sha256": command_sha}
                    )
                )
            stdout = run_dir / f"{step}.stdout"
            stderr = run_dir / f"{step}.stderr"
            stdout.write_bytes(b"")
            stderr.write_bytes(b"")
            exit_code = 7 if step == fail_step else 0
            if exit_code == 0:
                for path in expected_artifacts:
                    path.parent.mkdir(parents=True, exist_ok=True)
                    if path.name == "baseline-fingerprint.json":
                        tables = {
                            "tasks": {
                                "row_count": 1,
                                "content_sha256": "b" * 64,
                            }
                        }
                        value = {
                            "schema_version": 1,
                            "kind": "clone-b-baseline-fingerprint",
                            "database": "ab_formal_b_ui",
                            "fingerprint_algorithm": "test-fingerprint-v1",
                            "tables": tables,
                            "fingerprint_sha256": MODULE.canonical_hash(tables),
                        }
                        path.write_bytes(MODULE.g4.canonical_bytes(value))
                    elif path.name == "rollback-fingerprint.json":
                        value = {
                            "schema_version": 1,
                            "status": "PASS",
                            "violation_count": 0,
                            "baseline_fingerprint_sha256": "a" * 64,
                            "rollback_fingerprint_sha256": "a" * 64,
                        }
                        path.write_bytes(MODULE.g4.canonical_bytes(value))
                    elif path.suffix == ".jsonl":
                        path.write_text('{"row":1}\n', encoding="utf-8")
                    else:
                        path.write_bytes(
                            MODULE.g4.canonical_bytes(
                                {
                                    "schema_version": 1,
                                    "status": "PASS",
                                    "step": step,
                                }
                            )
                        )
            evidence = [
                MODULE.g4.evidence_ref(stdout, run_dir, clone_root),
                MODULE.g4.evidence_ref(stderr, run_dir, clone_root),
            ]
            if exit_code == 0:
                evidence.extend(
                    MODULE.g4.evidence_ref(path, run_dir, clone_root)
                    for path in expected_artifacts
                )
            return {
                "step": step,
                "phase": phase,
                "exit_code": exit_code,
                "elapsed_seconds": 0.01,
                "command_sha256": command_sha,
                "evidence": evidence,
                "process_group_quiescent": True,
            }

        return execute

    def observed_manifest(
        self,
        args: argparse.Namespace,
        gate: str,
        ready_sha: str,
        *,
        status: str = "PASS",
        violation_count: int = 0,
    ) -> pathlib.Path:
        evidence_root = args.clone_root / "observed"
        evidence_root.mkdir(exist_ok=True)
        report = evidence_root / f"{gate.lower()}-report.json"
        source_status = (
            "BLOCKED" if gate == "G6" and status == "FAIL" else status
        )
        report_value = {
            "schema_version": 1,
            "gate": gate,
            "status": source_status,
            "violation_count": violation_count,
            "violations": [
                {"code": f"test-{index + 1}"}
                for index in range(
                    violation_count
                    if isinstance(violation_count, int)
                    and not isinstance(violation_count, bool)
                    and violation_count > 0
                    else 0
                )
            ],
        }
        if gate == "G6":
            report_value[MODULE.EVIDENCE_HASH_FIELD] = (
                MODULE.compact_canonical_hash(report_value)
            )
        report.write_bytes(
            MODULE.g4.canonical_bytes(report_value)
        )
        manifest = evidence_root / f"{gate.lower()}-manifest.json"
        MODULE.write_hashed_json(
            manifest,
            {
                "schema_version": 1,
                "gate": gate,
                "status": status,
                "violation_count": violation_count,
                "hold_open_ledger_sha256": ready_sha,
                "artifacts": [
                    MODULE.relative_file_identity(report, args.clone_root)
                ],
            },
            MODULE.EVIDENCE_HASH_FIELD,
        )
        return manifest.resolve()

    def test_apply_success_publishes_hash_bound_ready_without_rollback(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            calls: list[str] = []
            execute = self.fake_execute(calls)
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4, "execute_step", side_effect=execute
                ),
            ):
                result = MODULE.run(args)
            self.assertEqual(result["status"], "HOLD_OPEN_READY")
            self.assertEqual(calls, list(MODULE.APPLY_SEQUENCE))
            ready = MODULE.read_hashed_json(
                args.run_dir / "HOLD_OPEN_READY.json", "ready"
            )
            session = MODULE.read_hashed_json(
                args.run_dir / "state" / "session.json", "session"
            )
            self.assertTrue(ready["process_groups_quiescent"])
            self.assertFalse(ready["production_writes_executed"])
            self.assertTrue(
                session["source_inputs"]["git_worktree_clean"]
            )
            self.assertTrue((args.run_dir / "apply-commands.jsonl").is_file())
            self.assertFalse((args.run_dir / "commands.jsonl").exists())
            self.assertNotIn("search_rollback", calls)

    def test_resume_requires_bound_g5_g6_then_rolls_back_in_strict_order(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            apply_calls: list[str] = []
            apply_execute = self.fake_execute(apply_calls)
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4, "execute_step", side_effect=apply_execute
                ),
            ):
                ready_result = MODULE.run(args)
            args.phase = "resume-and-rollback"
            args.g5_evidence_manifest = self.observed_manifest(
                args, "G5", ready_result["ready_ledger_sha256"]
            )
            args.g6_evidence_manifest = self.observed_manifest(
                args, "G6", ready_result["ready_ledger_sha256"]
            )
            rollback_calls: list[str] = []
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute(rollback_calls),
                ),
            ):
                result = MODULE.run(args)
            self.assertEqual(result["status"], "ROLLED_BACK")
            self.assertEqual(
                rollback_calls,
                [
                    "search_rollback",
                    "workflow_rollback",
                    "bundle_rollback",
                    "recovery_rollback",
                    "validate_after_rollback_fingerprint",
                ],
            )
            self.assertTrue((args.run_dir / "ROLLBACK_COMPLETE.json").is_file())
            self.assertTrue(
                (args.run_dir / "rollback-commands.jsonl").is_file()
            )
            authorization = MODULE.read_hashed_json(
                args.run_dir / "state" / "rollback-authorized.json",
                "rollback authorization",
            )
            first_started = MODULE.read_hashed_json(
                args.run_dir
                / "state"
                / "rollback-01-search-rollback.started.json",
                "first rollback checkpoint",
            )
            self.assertEqual(
                first_started["prior_checkpoint_sha256"],
                authorization[MODULE.DOCUMENT_HASH_FIELD],
            )

    def test_abort_after_g5_failure_rolls_back_and_preserves_failure(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute([]),
                ),
            ):
                ready = MODULE.run(args)
            args.phase = "abort-and-rollback"
            args.g5_evidence_manifest = self.observed_manifest(
                args,
                "G5",
                ready["ready_ledger_sha256"],
                status="FAIL",
                violation_count=3,
            )
            rollback_calls: list[str] = []
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute(rollback_calls),
                ),
            ):
                result = MODULE.run(args)
            self.assertEqual(
                result["status"], "ROLLED_BACK_AFTER_OBSERVED_FAILURE"
            )
            self.assertEqual(result["exit_code"], 1)
            self.assertEqual(
                rollback_calls,
                [
                    "search_rollback",
                    "workflow_rollback",
                    "bundle_rollback",
                    "recovery_rollback",
                    "validate_after_rollback_fingerprint",
                ],
            )
            receipt = MODULE.read_hashed_json(
                args.run_dir / "ROLLBACK_COMPLETE.json", "terminal receipt"
            )
            self.assertEqual(
                receipt["trigger"],
                {"gate": "G5", "status": "FAIL", "violation_count": 3},
            )

    def test_abort_after_g6_failure_requires_prior_g5_pass(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute([]),
                ),
            ):
                ready = MODULE.run(args)
            args.phase = "abort-and-rollback"
            args.g5_evidence_manifest = self.observed_manifest(
                args, "G5", ready["ready_ledger_sha256"]
            )
            args.g6_evidence_manifest = self.observed_manifest(
                args,
                "G6",
                ready["ready_ledger_sha256"],
                status="FAIL",
                violation_count=2,
            )
            rollback_calls: list[str] = []
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute(rollback_calls),
                ),
            ):
                result = MODULE.run(args)
            self.assertEqual(
                result["status"], "ROLLED_BACK_AFTER_OBSERVED_FAILURE"
            )
            self.assertEqual(result["exit_code"], 1)
            receipt = MODULE.read_hashed_json(
                args.run_dir / "ROLLBACK_COMPLETE.json", "terminal receipt"
            )
            self.assertEqual(
                receipt["trigger"],
                {"gate": "G6", "status": "FAIL", "violation_count": 2},
            )

    def test_abort_rejects_invalid_status_count_before_rollback(self):
        invalid = (
            ("PASS", 1),
            ("PASS", 0.0),
            ("FAIL", 0),
            ("FAIL", True),
        )
        for status, count in invalid:
            with self.subTest(status=status, count=count):
                with tempfile.TemporaryDirectory() as raw:
                    args = self.make_inputs(raw)
                    with (
                        mock.patch.object(
                            MODULE, "repo_head", return_value="f" * 64
                        ),
                        mock.patch.object(
                            MODULE.shutil,
                            "which",
                            return_value="/usr/bin/go",
                        ),
                        mock.patch.object(
                            MODULE.g4,
                            "execute_step",
                            side_effect=self.fake_execute([]),
                        ),
                    ):
                        ready = MODULE.run(args)
                    args.phase = "abort-and-rollback"
                    args.g5_evidence_manifest = self.observed_manifest(
                        args,
                        "G5",
                        ready["ready_ledger_sha256"],
                        status=status,
                        violation_count=count,
                    )
                    calls: list[str] = []
                    with (
                        mock.patch.object(
                            MODULE, "repo_head", return_value="f" * 64
                        ),
                        mock.patch.object(
                            MODULE.shutil,
                            "which",
                            return_value="/usr/bin/go",
                        ),
                        mock.patch.object(
                            MODULE.g4,
                            "execute_step",
                            side_effect=self.fake_execute(calls),
                        ),
                    ):
                        with self.assertRaisesRegex(
                            ValueError, "envelope is invalid"
                        ):
                            MODULE.run(args)
                    self.assertEqual(calls, [])

    def test_forged_existing_completion_never_bypasses_rollback(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute([]),
                ),
            ):
                ready = MODULE.run(args)
            args.phase = "abort-and-rollback"
            args.g5_evidence_manifest = self.observed_manifest(
                args,
                "G5",
                ready["ready_ledger_sha256"],
                status="FAIL",
                violation_count=1,
            )
            MODULE.write_hashed_json(
                args.run_dir / "ROLLBACK_COMPLETE.json",
                {"status": "FORGED_TERMINAL"},
            )
            calls: list[str] = []
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute(calls),
                ),
            ):
                with self.assertRaisesRegex(
                    ValueError, "completion ledger is missing"
                ):
                    MODULE.run(args)
            self.assertEqual(calls, [])

    def test_complete_receipt_with_wrong_authorization_is_rejected(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute([]),
                ),
            ):
                ready = MODULE.run(args)
            args.phase = "abort-and-rollback"
            args.g5_evidence_manifest = self.observed_manifest(
                args,
                "G5",
                ready["ready_ledger_sha256"],
                status="FAIL",
                violation_count=1,
            )
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute([]),
                ),
            ):
                MODULE.run(args)
            receipt_path = args.run_dir / "ROLLBACK_COMPLETE.json"
            receipt = MODULE.read_hashed_json(receipt_path, "receipt")
            receipt.pop(MODULE.DOCUMENT_HASH_FIELD)
            receipt["authorization_sha256"] = "0" * 64
            receipt_path.unlink()
            MODULE.write_hashed_json(receipt_path, receipt)
            retry_calls: list[str] = []
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute(retry_calls),
                ),
            ):
                with self.assertRaisesRegex(
                    ValueError, "identity or hashes differ"
                ):
                    MODULE.run(args)
            self.assertEqual(retry_calls, [])

    def test_failed_apply_automatically_rolls_back_only_attempted_components(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            calls: list[str] = []
            base_execute = self.fake_execute(
                calls, fail_step="workflow_apply"
            )

            def execute(**kwargs):
                result = base_execute(**kwargs)
                if kwargs["step"] == "workflow_apply":
                    snapshot = (
                        kwargs["run_dir"]
                        / "workflow-snapshot"
                        / "workflow-groups-snapshot.json"
                    )
                    snapshot.parent.mkdir(parents=True, exist_ok=True)
                    snapshot.write_text('{"version":8}\n', encoding="utf-8")
                return result

            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4, "execute_step", side_effect=execute
                ),
            ):
                result = MODULE.run(args)
            self.assertEqual(
                result["status"], "ROLLED_BACK_AFTER_APPLY_FAILURE"
            )
            self.assertEqual(result["exit_code"], 1)
            self.assertEqual(
                result["terminal_receipt"], "ROLLBACK_COMPLETE.json"
            )
            receipt = MODULE.read_hashed_json(
                args.run_dir / "ROLLBACK_COMPLETE.json", "terminal receipt"
            )
            self.assertEqual(
                receipt["trigger"],
                {"step": "workflow_apply", "exit_code": 7},
            )
            self.assertEqual(
                calls[-4:],
                [
                    "workflow_rollback",
                    "bundle_rollback",
                    "recovery_rollback",
                    "validate_after_rollback_fingerprint",
                ],
            )
            self.assertNotIn("search_rollback", calls)
            self.assertNotIn("idempotent_apply", calls)

    def test_failed_recovery_before_mutation_only_validates_fingerprint(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            calls: list[str] = []
            execute = self.fake_execute(calls, fail_step="recovery_apply")
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4, "execute_step", side_effect=execute
                ),
            ):
                result = MODULE.run(args)
            self.assertEqual(
                result["status"], "ROLLED_BACK_AFTER_APPLY_FAILURE"
            )
            self.assertEqual(
                calls,
                [
                    "capture_baseline_fingerprint",
                    "recovery_apply",
                    "validate_after_rollback_fingerprint",
                ],
            )
            authorization = MODULE.read_hashed_json(
                args.run_dir / "state" / "rollback-authorized.json",
                "rollback authorization",
            )
            self.assertEqual(authorization["attempted_components"], [])
            self.assertEqual(
                authorization["rollback_steps"],
                ["validate_after_rollback_fingerprint"],
            )

    def test_interrupted_apply_requires_quiescence_confirmation_then_uses_seed(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            calls: list[str] = []
            normal = self.fake_execute(calls)

            def interrupt(**kwargs):
                if kwargs["step"] != "recovery_apply":
                    return normal(**kwargs)
                calls.append("recovery_apply")
                command = {"step": "recovery_apply", "argv": kwargs["argv"]}
                with (kwargs["run_dir"] / "commands.jsonl").open("ab") as handle:
                    handle.write(
                        MODULE.g4.canonical_bytes(
                            {
                                **command,
                                "sha256": hashlib.sha256(
                                    MODULE.g4.canonical_bytes(command)
                                ).hexdigest(),
                            }
                        )
                    )
                (
                    kwargs["run_dir"] / "recovery-file-write-ahead.json"
                ).write_text('{"status":"PREPARED"}\n', encoding="utf-8")
                raise RuntimeError("simulated coordinator interruption")

            common = (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
            )
            with common[0], common[1], mock.patch.object(
                MODULE.g4, "execute_step", side_effect=interrupt
            ):
                with self.assertRaisesRegex(RuntimeError, "interruption"):
                    MODULE.run(args)
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
            ):
                with self.assertRaisesRegex(
                    ValueError, "confirm-interrupted-step-quiescent"
                ):
                    MODULE.run(args)
            args.confirm_interrupted_step_quiescent = True
            rollback_calls: list[str] = []
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute(rollback_calls),
                ),
            ):
                result = MODULE.run(args)
            self.assertEqual(
                result["status"], "ROLLED_BACK_AFTER_APPLY_FAILURE"
            )
            self.assertEqual(
                rollback_calls,
                [
                    "recovery_rollback",
                    "validate_after_rollback_fingerprint",
                ],
            )
            authorization = MODULE.read_hashed_json(
                args.run_dir / "state" / "rollback-authorized.json",
                "rollback authorization",
            )
            self.assertEqual(authorization["reason"], "apply-interrupted")
            self.assertEqual(authorization["attempted_components"], ["recovery"])

    def test_failed_rollback_never_advances_on_retry(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute([]),
                ),
            ):
                ready = MODULE.run(args)
            args.phase = "resume-and-rollback"
            args.g5_evidence_manifest = self.observed_manifest(
                args, "G5", ready["ready_ledger_sha256"]
            )
            args.g6_evidence_manifest = self.observed_manifest(
                args, "G6", ready["ready_ledger_sha256"]
            )
            first_calls: list[str] = []
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute(
                        first_calls, fail_step="workflow_rollback"
                    ),
                ),
            ):
                first = MODULE.run(args)
            self.assertEqual(first["status"], "BLOCKED")
            self.assertEqual(
                first_calls, ["search_rollback", "workflow_rollback"]
            )
            retry_calls: list[str] = []
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute(retry_calls),
                ),
            ):
                retry = MODULE.run(args)
            self.assertEqual(retry["status"], "BLOCKED")
            self.assertEqual(retry_calls, [])
            self.assertIn("refusing to advance", retry["blocker"])

    def test_started_without_completion_is_never_repeated(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
            ):
                context = MODULE.build_context(args, allow_create=True)
            MODULE.write_hashed_json(
                MODULE.checkpoint_path(
                    context,
                    "apply",
                    1,
                    MODULE.APPLY_SEQUENCE[0],
                    "started",
                ),
                {
                    "schema_version": 1,
                    "kind": "clone-b-hold-open-step-start",
                    "run_id": context.run_id,
                    "database": context.database,
                    "phase": "apply",
                    "ordinal": 1,
                    "step": MODULE.APPLY_SEQUENCE[0],
                    "prior_checkpoint_sha256": context.session[
                        MODULE.DOCUMENT_HASH_FIELD
                    ],
                    "production_writes_executed": False,
                },
            )
            calls: list[str] = []
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute(calls),
                ),
            ):
                with self.assertRaisesRegex(
                    ValueError, "confirm-interrupted-step-quiescent"
                ):
                    MODULE.run(args)
            self.assertEqual(calls, [])

    def test_staging_receipts_are_hash_authorized_rollback_seeds(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
            ):
                context = MODULE.build_context(args, allow_create=True)
            names = (
                "recovery-ownership-23989.json",
                "recovery-staging-ownership-23989.json",
                "bundle-ownership-25557.json",
                "bundle-staging-ownership-25557.json",
            )
            for name in names:
                (context.run_dir / name).write_text(
                    '{"status":"owned"}\n',
                    encoding="utf-8",
                )
            seeds = MODULE.rollback_seed_paths(context)
            self.assertIn(
                "recovery-staging-ownership-23989.json",
                {path.name for path in seeds["recovery"]},
            )
            self.assertIn(
                "bundle-staging-ownership-25557.json",
                {path.name for path in seeds["bundle"]},
            )

    def test_repo_head_rejects_a_dirty_worktree(self):
        head = mock.Mock(returncode=0, stdout="f" * 40 + "\n")
        dirty = mock.Mock(
            returncode=0,
            stdout=" M scripts/ab/api_ab_compare.py\n",
        )
        with mock.patch.object(
            MODULE.subprocess, "run", side_effect=[head, dirty]
        ):
            with self.assertRaisesRegex(ValueError, "clean Git worktree"):
                MODULE.repo_head(pathlib.Path.cwd())

    def test_execute_step_fails_after_cleaning_lingering_process_group(self):
        with tempfile.TemporaryDirectory() as raw:
            clone_root = pathlib.Path(raw)
            run_dir = clone_root / "run"
            run_dir.mkdir()
            process = mock.Mock()
            process.pid = 43210
            process.wait.return_value = 0
            with (
                mock.patch.object(
                    MODULE.g4.subprocess, "Popen", return_value=process
                ),
                mock.patch.object(
                    MODULE.g4,
                    "linux_non_zombie_process_group_members",
                    return_value=[43211],
                ),
                mock.patch.object(
                    MODULE.g4,
                    "terminate_process_group",
                    return_value=(True, [], None),
                ) as terminate,
            ):
                record = MODULE.g4.execute_step(
                    step="mock_step",
                    phase="apply",
                    argv=["mock", "step"],
                    expected_artifacts=[],
                    run_dir=run_dir,
                    clone_root=clone_root,
                    repo_root=pathlib.Path.cwd(),
                    env={},
                    timeout_seconds=1.0,
                    require_quiescence=True,
                )
            self.assertEqual(record["exit_code"], 122)
            self.assertTrue(record["process_group_quiescent"])
            terminate.assert_called_once_with(process)

    def test_tampered_observed_evidence_blocks_before_any_rollback(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute([]),
                ),
            ):
                ready = MODULE.run(args)
            args.phase = "resume-and-rollback"
            args.g5_evidence_manifest = self.observed_manifest(
                args, "G5", ready["ready_ledger_sha256"]
            )
            args.g6_evidence_manifest = self.observed_manifest(
                args, "G6", ready["ready_ledger_sha256"]
            )
            g6 = MODULE.read_hashed_json(
                args.g6_evidence_manifest,
                "G6",
                MODULE.EVIDENCE_HASH_FIELD,
            )
            target = MODULE.validate_relative_artifact(
                args.clone_root, g6["artifacts"][0], "G6"
            )
            target.write_text("tampered\n", encoding="utf-8")
            calls: list[str] = []
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute(calls),
                ),
            ):
                with self.assertRaisesRegex(ValueError, "drifted"):
                    MODULE.run(args)
            self.assertEqual(calls, [])

    def test_tampered_apply_artifact_blocks_before_any_rollback(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute([]),
                ),
            ):
                ready = MODULE.run(args)
            args.phase = "resume-and-rollback"
            args.g5_evidence_manifest = self.observed_manifest(
                args, "G5", ready["ready_ledger_sha256"]
            )
            args.g6_evidence_manifest = self.observed_manifest(
                args, "G6", ready["ready_ledger_sha256"]
            )
            (args.run_dir / "baseline-fingerprint.json").write_text(
                "tampered\n", encoding="utf-8"
            )
            calls: list[str] = []
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute(calls),
                ),
            ):
                with self.assertRaisesRegex(ValueError, "drifted"):
                    MODULE.run(args)
            self.assertEqual(calls, [])

    def test_resume_rejects_git_head_drift_before_any_rollback(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            with (
                mock.patch.object(MODULE, "repo_head", return_value="f" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute([]),
                ),
            ):
                ready = MODULE.run(args)
            args.phase = "resume-and-rollback"
            args.g5_evidence_manifest = self.observed_manifest(
                args, "G5", ready["ready_ledger_sha256"]
            )
            args.g6_evidence_manifest = self.observed_manifest(
                args, "G6", ready["ready_ledger_sha256"]
            )
            calls: list[str] = []
            with (
                mock.patch.object(MODULE, "repo_head", return_value="e" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute(calls),
                ),
            ):
                with self.assertRaisesRegex(
                    ValueError, "session identity differs"
                ):
                    MODULE.run(args)
            self.assertEqual(calls, [])

    def test_expected_baseline_drift_blocks_before_clone_write(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.make_inputs(raw)
            expected = json.loads(
                args.expected_baseline_file.read_text(encoding="utf-8")
            )
            expected["tables"]["tasks"]["row_count"] = 2
            expected["fingerprint_sha256"] = MODULE.canonical_hash(
                expected["tables"]
            )
            args.expected_baseline_file.write_bytes(
                MODULE.g4.canonical_bytes(expected)
            )
            calls: list[str] = []
            with (
                mock.patch.object(MODULE, "repo_head", return_value="a" * 64),
                mock.patch.object(
                    MODULE.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(
                    MODULE.g4,
                    "execute_step",
                    side_effect=self.fake_execute(calls),
                ),
            ):
                with self.assertRaisesRegex(
                    ValueError, "differs from the frozen expected baseline"
                ):
                    MODULE.run(args)
            self.assertEqual(calls, ["capture_baseline_fingerprint"])


if __name__ == "__main__":
    unittest.main()
