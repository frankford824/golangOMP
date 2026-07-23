import hashlib
import importlib.util
import json
import pathlib
import sys
import tempfile
import unittest


PATH = pathlib.Path(__file__).with_name("finalize_release_gates.py")
SPEC = importlib.util.spec_from_file_location("finalize_release_gates", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


def write_json(path, value):
    path.write_bytes(MODULE.canonical_bytes(value))
    return MODULE.sha256_file(path)


class FinalizeReleaseGatesTest(unittest.TestCase):
    def make_run(self, root, *, blocked_gate=None, signed=True):
        run_id = "formal-run-1"
        run_dir = pathlib.Path(root) / run_id
        run_dir.mkdir()
        gates = {}
        for gate in MODULE.GATES:
            payload = {
                "schema_version": 1,
                "run_id": run_id,
                "status": "PASS",
                "violation_count": 0,
            }
            if gate == "G0":
                payload.update(
                    {
                        "candidate": {
                            "git_head": "a" * 40,
                            "worktree_diff_sha256": MODULE.EMPTY_SHA256,
                        },
                        "openapi_sha256": "b" * 64,
                        "external_backend_image_digest": "sha256:" + "c" * 64,
                        "dev_plus_backend_image_digest": "sha256:" + "d" * 64,
                        "external_frontend_manifest_sha256": "e" * 64,
                        "dev_plus_frontend_manifest_sha256": "f" * 64,
                        "configuration_sha256": "1" * 64,
                        "migration_mapping_sha256": "2" * 64,
                        "snapshot_sha256": "3" * 64,
                        "review_manifest_sha256": "4" * 64,
                    }
                )
            elif gate == "G3":
                payload.update(
                    {
                        "decision": "APPROVED",
                        "candidate_sha256": "5" * 64,
                        "cohort_digest": "6" * 64,
                        "summary": {
                            "proposed_review_count": 0,
                            "hard_blocked_count": 0,
                        },
                    }
                )
            elif gate == "G4":
                payload["steps"] = [
                    {"step": name, "exit_code": 0}
                    for name in (
                        "dry_run_before",
                        "apply",
                        "idempotent_apply",
                        "validate_after_apply",
                        "rollback",
                        "validate_after_rollback",
                    )
                ]
            elif gate == "G7":
                payload.update(
                    {
                        "critical_scenario_pass_rate": 1.0,
                        "browser_surface": "in_app_browser",
                        "screenshot_evidence_sha256": "7" * 64,
                    }
                )
            elif gate == "G9":
                payload.update(
                    {"unresolved_p0_count": 0, "unresolved_p1_count": 0}
                )
            elif gate == "G10":
                payload["timings_seconds"] = {
                    "apply": 100,
                    "validate": 200,
                    "rollback": 100,
                    "total": 400,
                }
            if gate == blocked_gate:
                payload["status"] = "FAIL"
                payload["violation_count"] = 1
            path = run_dir / f"{gate.lower()}.json"
            digest = write_json(path, payload)
            gates[gate] = {
                "path": path.name,
                "sha256": digest,
                "executor": f"{gate}-executor",
                "reviewer": f"{gate}-reviewer",
            }
        index = {
            "schema_version": 1,
            "run_id": run_id,
            "gates": gates,
            "signatures": [],
        }
        unsigned_digest = hashlib.sha256(MODULE.canonical_bytes(index)).hexdigest()
        if signed:
            index["signatures"] = [
                {
                    "role": role,
                    "signer": f"signer-{position}",
                    "decision": "GO",
                    "evidence_index_sha256": unsigned_digest,
                    "signed_at": "2026-07-23T12:00:00Z",
                }
                for position, role in enumerate(sorted(MODULE.REQUIRED_ROLES), 1)
            ]
        index_path = run_dir / "evidence-index.json"
        write_json(index_path, index)
        return run_dir, index_path

    def test_all_exact_evidence_and_independent_signatures_can_go(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw)
            _, report = MODULE.validate_index(run_dir, index_path)
            self.assertEqual(report["decision"], "GO")
            self.assertEqual(report["passed_gate_count"], 11)
            MODULE.write_outputs(
                run_dir,
                report,
                run_dir / "final-gate-report.json",
                run_dir / "go-no-go.md",
                run_dir / "final-ledger.json",
            )
            self.assertIn(
                "decision: GO",
                (run_dir / "go-no-go.md").read_text(encoding="utf-8"),
            )

    def test_one_failed_gate_is_no_go(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw, blocked_gate="G6")
            _, report = MODULE.validate_index(run_dir, index_path)
            self.assertEqual(report["decision"], "NO-GO")
            self.assertEqual(report["gates"]["G6"]["status"], "BLOCKED")

    def test_missing_signature_is_no_go(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw, signed=False)
            _, report = MODULE.validate_index(run_dir, index_path)
            self.assertEqual(report["decision"], "NO-GO")
            self.assertTrue(report["signature_violations"])

    def test_hash_drift_is_blocked(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw)
            (run_dir / "g5.json").write_text("{}\n", encoding="utf-8")
            _, report = MODULE.validate_index(run_dir, index_path)
            self.assertEqual(report["decision"], "NO-GO")
            self.assertIn(
                "artifact hash mismatch",
                report["gates"]["G5"]["violations"][0],
            )

    def test_dirty_candidate_is_blocked(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw)
            index = json.loads(index_path.read_text(encoding="utf-8"))
            env_path = run_dir / index["gates"]["G0"]["path"]
            env = json.loads(env_path.read_text(encoding="utf-8"))
            env["candidate"]["worktree_diff_sha256"] = "9" * 64
            index["gates"]["G0"]["sha256"] = write_json(env_path, env)
            index["signatures"] = []
            write_json(index_path, index)
            _, report = MODULE.validate_index(run_dir, index_path)
            self.assertEqual(report["decision"], "NO-GO")
            self.assertIn(
                "candidate worktree is not clean",
                report["gates"]["G0"]["violations"],
            )


if __name__ == "__main__":
    unittest.main()
