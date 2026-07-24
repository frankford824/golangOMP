import hashlib
import importlib.util
import json
import pathlib
import sys
import tempfile
import unittest

from scripts.ab.test_clone_b_materialization_component import (
    write_component_chain_fixture,
)

PATH = pathlib.Path(__file__).with_name("summarize_g4.py")
SPEC = importlib.util.spec_from_file_location("summarize_g4", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class SummarizeG4Test(unittest.TestCase):
    def make_evidence(self, root):
        root = pathlib.Path(root)
        run_dir = root / "g4"
        clone_root = root / "clone"
        run_dir.mkdir()
        clone_root.mkdir()
        rows = []
        for index, (step, phase) in enumerate(MODULE.REQUIRED_STEPS):
            stdout = run_dir / f"{step}.stdout"
            stderr = run_dir / f"{step}.stderr"
            stdout.write_text(f"{step}\n", encoding="utf-8")
            stderr.write_bytes(b"")
            rows.append(
                {
                    "step": step,
                    "phase": phase,
                    "exit_code": 0,
                    "elapsed_seconds": float(index + 1) / 10,
                    "command_sha256": f"{index + 1:064x}",
                    "evidence": [
                        {
                            "root": "run_dir",
                            "path": stdout.name,
                            "sha256": MODULE.sha256_file(stdout),
                        },
                        {
                            "root": "run_dir",
                            "path": stderr.name,
                            "sha256": MODULE.sha256_file(stderr),
                        },
                    ],
                }
            )
        steps = run_dir / "steps.jsonl"
        steps.write_bytes(b"".join(MODULE.canonical_bytes(row) for row in rows))
        baseline_tables = {
            "tasks": {
                "row_count": 2,
                "content_sha256": "4" * 64,
                "schema_sha256": "5" * 64,
                "content_fingerprint_algorithm": (
                    MODULE.ROW_FINGERPRINT_ALGORITHM
                ),
            }
        }
        baseline = run_dir / "baseline-fingerprint.json"
        baseline.write_bytes(
            MODULE.canonical_bytes(
                {
                    "schema_version": 1,
                    "kind": "clone-b-baseline-fingerprint",
                    "database": "ab_formal_b_ui",
                    "fingerprint_algorithm": (
                        MODULE.ROW_FINGERPRINT_ALGORITHM
                    ),
                    "tables": baseline_tables,
                    "fingerprint_sha256": hashlib.sha256(
                        MODULE.canonical_bytes(baseline_tables)
                    ).hexdigest(),
                }
            )
        )
        baseline_payload = json.loads(baseline.read_text(encoding="utf-8"))
        fingerprint = run_dir / "rollback-fingerprint.json"
        fingerprint.write_text(
            json.dumps(
                {
                    "schema_version": 1,
                    "status": "PASS",
                    "violation_count": 0,
                    "baseline_artifact_sha256": MODULE.sha256_file(baseline),
                    "baseline_fingerprint_sha256": baseline_payload[
                        "fingerprint_sha256"
                    ],
                    "rollback_fingerprint_sha256": baseline_payload[
                        "fingerprint_sha256"
                    ],
                }
            )
            + "\n",
            encoding="utf-8",
        )
        tables = {
            "task_search_documents": {
                "row_count": 2,
                "content_sha256": "1" * 64,
            },
            "task_asset_group_search_documents": {
                "row_count": 3,
                "content_sha256": "2" * 64,
            },
            "product_search_documents": {
                "row_count": 4,
                "content_sha256": "3" * 64,
            },
        }
        snapshot_sha = hashlib.sha256(MODULE.canonical_bytes(tables)).hexdigest()
        search_archive = run_dir / "search-documents-snapshot.jsonl"
        search_archive.write_bytes(b'{"table":"task_search_documents"}\n')
        archive_sha = MODULE.sha256_file(search_archive)
        search_snapshot = run_dir / "search-snapshot.json"
        search_snapshot.write_text(
            json.dumps(
                {
                    "schema_version": 1,
                    "status": "CAPTURED",
                    "violation_count": 0,
                    "tables": tables,
                    "snapshot_sha256": snapshot_sha,
                    "archive": {
                        "format": "deterministic-jsonl-v1",
                        "sha256": archive_sha,
                        "size": search_archive.stat().st_size,
                    },
                }
            )
            + "\n",
            encoding="utf-8",
        )
        search_rollback = run_dir / "search-rollback.json"
        search_rollback.write_text(
            json.dumps(
                {
                    "schema_version": 1,
                    "status": "PASS",
                    "violation_count": 0,
                    "snapshot_sha256": snapshot_sha,
                    "restored_snapshot_sha256": snapshot_sha,
                    "restored_tables": tables,
                    "source_archive_sha256": archive_sha,
                }
            )
            + "\n",
            encoding="utf-8",
        )
        write_component_chain_fixture(run_dir, run_id="formal-test")
        return run_dir, clone_root, steps, fingerprint, rows

    def summarize(self, root):
        run_dir, clone_root, steps, fingerprint, _ = self.make_evidence(root)
        return MODULE.summarize(
            run_id="formal-test",
            run_dir=run_dir,
            clone_root=clone_root,
            steps_path=steps,
            baseline_fingerprint_path=run_dir / "baseline-fingerprint.json",
            rollback_fingerprint_path=fingerprint,
            search_snapshot_path=run_dir / "search-snapshot.json",
            search_snapshot_archive_path=run_dir
            / "search-documents-snapshot.jsonl",
            search_rollback_path=run_dir / "search-rollback.json",
            total_seconds=10.5,
            max_step=600,
            max_phase=600,
            max_total=1800,
        )

    def test_complete_chain_is_directly_finalizable(self):
        with tempfile.TemporaryDirectory() as root:
            result = self.summarize(root)
            self.assertEqual(result["status"], "PASS")
            self.assertEqual(result["exit_code"], 0)
            self.assertIsInstance(result["elapsed_seconds"], float)
            self.assertEqual(
                list(result["timings_seconds"]),
                ["apply", "validate", "rollback", "total"],
            )
            self.assertTrue(
                all(isinstance(row["exit_code"], int) for row in result["steps"])
            )
            phases = {row["step"]: row["phase"] for row in result["steps"]}
            self.assertEqual(phases["idempotent_apply"], "apply")

    def test_step_failure_is_blocked(self):
        with tempfile.TemporaryDirectory() as root:
            run_dir, clone_root, steps, fingerprint, rows = self.make_evidence(root)
            rows[3]["exit_code"] = 7
            steps.write_bytes(
                b"".join(MODULE.canonical_bytes(row) for row in rows)
            )
            result = MODULE.summarize(
                run_id="formal-test",
                run_dir=run_dir,
                clone_root=clone_root,
                steps_path=steps,
                baseline_fingerprint_path=run_dir
                / "baseline-fingerprint.json",
                rollback_fingerprint_path=fingerprint,
                search_snapshot_path=run_dir / "search-snapshot.json",
                search_snapshot_archive_path=run_dir
                / "search-documents-snapshot.jsonl",
                search_rollback_path=run_dir / "search-rollback.json",
                total_seconds=10,
                max_step=600,
                max_phase=600,
                max_total=1800,
            )
            self.assertEqual(result["status"], "BLOCKED")
            self.assertEqual(result["exit_code"], 1)
            self.assertTrue(
                any(
                    item["violation_code"] == "g4.step_failed"
                    for item in result["violations"]
                )
            )

    def test_component_database_report_tamper_is_blocked(self):
        with tempfile.TemporaryDirectory() as root:
            run_dir, clone_root, steps, fingerprint, _ = self.make_evidence(
                root
            )
            target = run_dir / "bundle-db-idempotent.json"
            payload = json.loads(target.read_text(encoding="utf-8"))
            payload["already_applied_bundle_count"] = 6
            target.write_text(json.dumps(payload) + "\n", encoding="utf-8")
            result = MODULE.summarize(
                run_id="formal-test",
                run_dir=run_dir,
                clone_root=clone_root,
                steps_path=steps,
                baseline_fingerprint_path=run_dir
                / "baseline-fingerprint.json",
                rollback_fingerprint_path=fingerprint,
                search_snapshot_path=run_dir / "search-snapshot.json",
                search_snapshot_archive_path=run_dir
                / "search-documents-snapshot.jsonl",
                search_rollback_path=run_dir / "search-rollback.json",
                total_seconds=10,
                max_step=600,
                max_phase=600,
                max_total=1800,
            )
            self.assertEqual(result["status"], "BLOCKED")
            self.assertTrue(
                any(
                    item["violation_code"] == "g4.component_chain_invalid"
                    for item in result["violations"]
                )
            )

    def test_raw_evidence_drift_is_blocked(self):
        with tempfile.TemporaryDirectory() as root:
            run_dir, clone_root, steps, fingerprint, _ = self.make_evidence(root)
            (run_dir / "dry_run_before.stdout").write_text(
                "tampered\n", encoding="utf-8"
            )
            result = MODULE.summarize(
                run_id="formal-test",
                run_dir=run_dir,
                clone_root=clone_root,
                steps_path=steps,
                baseline_fingerprint_path=run_dir
                / "baseline-fingerprint.json",
                rollback_fingerprint_path=fingerprint,
                search_snapshot_path=run_dir / "search-snapshot.json",
                search_snapshot_archive_path=run_dir
                / "search-documents-snapshot.jsonl",
                search_rollback_path=run_dir / "search-rollback.json",
                total_seconds=10,
                max_step=600,
                max_phase=600,
                max_total=1800,
            )
            self.assertEqual(result["status"], "BLOCKED")
            self.assertTrue(
                any(
                    item["violation_code"] == "g4.evidence_drift"
                    for item in result["violations"]
                )
            )

    def test_rollback_fingerprint_must_equal_baseline(self):
        with tempfile.TemporaryDirectory() as root:
            run_dir, clone_root, steps, fingerprint, _ = self.make_evidence(root)
            payload = json.loads(fingerprint.read_text(encoding="utf-8"))
            payload["rollback_fingerprint_sha256"] = "b" * 64
            fingerprint.write_text(json.dumps(payload), encoding="utf-8")
            result = MODULE.summarize(
                run_id="formal-test",
                run_dir=run_dir,
                clone_root=clone_root,
                steps_path=steps,
                baseline_fingerprint_path=run_dir
                / "baseline-fingerprint.json",
                rollback_fingerprint_path=fingerprint,
                search_snapshot_path=run_dir / "search-snapshot.json",
                search_snapshot_archive_path=run_dir
                / "search-documents-snapshot.jsonl",
                search_rollback_path=run_dir / "search-rollback.json",
                total_seconds=10,
                max_step=600,
                max_phase=600,
                max_total=1800,
            )
            self.assertEqual(result["status"], "BLOCKED")
            self.assertTrue(
                any(
                    item["violation_code"] == "g4.rollback_fingerprint_mismatch"
                    for item in result["violations"]
                )
            )

    def test_baseline_artifact_tamper_is_blocked(self):
        with tempfile.TemporaryDirectory() as root:
            run_dir, clone_root, steps, fingerprint, _ = self.make_evidence(root)
            baseline = run_dir / "baseline-fingerprint.json"
            payload = json.loads(baseline.read_text(encoding="utf-8"))
            payload["tables"]["tasks"]["row_count"] += 1
            baseline.write_text(json.dumps(payload), encoding="utf-8")
            result = MODULE.summarize(
                run_id="formal-test",
                run_dir=run_dir,
                clone_root=clone_root,
                steps_path=steps,
                baseline_fingerprint_path=baseline,
                rollback_fingerprint_path=fingerprint,
                search_snapshot_path=run_dir / "search-snapshot.json",
                search_snapshot_archive_path=run_dir
                / "search-documents-snapshot.jsonl",
                search_rollback_path=run_dir / "search-rollback.json",
                total_seconds=10,
                max_step=600,
                max_phase=600,
                max_total=1800,
            )
            self.assertEqual(result["status"], "BLOCKED")
            self.assertTrue(
                any(
                    item["violation_code"]
                    == "g4.rollback_fingerprint_unreadable"
                    for item in result["violations"]
                )
            )

    def test_search_restore_must_exactly_match_snapshot(self):
        with tempfile.TemporaryDirectory() as root:
            run_dir, clone_root, steps, fingerprint, _ = self.make_evidence(root)
            path = run_dir / "search-rollback.json"
            payload = json.loads(path.read_text(encoding="utf-8"))
            payload["restored_tables"]["task_search_documents"]["row_count"] += 1
            path.write_text(json.dumps(payload), encoding="utf-8")
            result = MODULE.summarize(
                run_id="formal-test",
                run_dir=run_dir,
                clone_root=clone_root,
                steps_path=steps,
                baseline_fingerprint_path=run_dir
                / "baseline-fingerprint.json",
                rollback_fingerprint_path=fingerprint,
                search_snapshot_path=run_dir / "search-snapshot.json",
                search_snapshot_archive_path=run_dir
                / "search-documents-snapshot.jsonl",
                search_rollback_path=path,
                total_seconds=10,
                max_step=600,
                max_phase=600,
                max_total=1800,
            )
            self.assertEqual(result["status"], "BLOCKED")
            self.assertTrue(
                any(
                    item["violation_code"] == "g4.search_restore_mismatch"
                    for item in result["violations"]
                )
            )


if __name__ == "__main__":
    unittest.main()
