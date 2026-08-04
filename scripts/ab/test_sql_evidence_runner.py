from __future__ import annotations

import hashlib
import json
import os
import pathlib
import re
import subprocess
import tempfile
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[2]
RUNNER = ROOT / "scripts/ab/run-ab-audit.sh"
SQL_DIR = ROOT / "scripts/ab/sql"
DATABASE_GATES = {"G01", "G02", "G03", "G04", "G05", "G07", "G08", "G09"}
DERIVATION = {gate: ("immutable_a_truth" if gate == "G07" else "independent_projection" if gate == "G09" else "reviewed_mapping_a_truth") for gate in DATABASE_GATES}


def write_manifest(root: pathlib.Path, run_id: str) -> tuple[pathlib.Path, str]:
    rows = []
    inputs = {"mapping_sha256": "1" * 64}
    for gate in sorted(DATABASE_GATES):
        components = [gate, "entity"]
        rows.append({
            "run_id": run_id, "gate_name": gate, "entity_key": f"{gate}:entity",
            "expected_hash": hashlib.sha256("\x1f".join(components).encode()).hexdigest(),
            "expected_state": "approved", "review_state": "pass",
            "detail_json": {"derivation_method": DERIVATION[gate], "input_sha256": inputs, "components": components},
        })
    object_detail = {"derivation_method": "object_verifier", "input_sha256": inputs, "verdict": "PASS"}
    rows.append({
        "run_id": run_id, "gate_name": "G06", "entity_key": "objects",
        "expected_hash": hashlib.sha256(json.dumps(object_detail, sort_keys=True, separators=(",", ":")).encode()).hexdigest(),
        "expected_state": "verified", "review_state": "pass", "detail_json": object_detail,
    })
    detail = {"derivation_method": "human_decision", "input_sha256": inputs, "decision": "confirmed"}
    rows.append({
        "run_id": run_id, "gate_name": "G10", "entity_key": "release-decision",
        "expected_hash": hashlib.sha256(json.dumps(detail, sort_keys=True, separators=(",", ":")).encode()).hexdigest(),
        "expected_state": "confirmed", "review_state": "pass", "detail_json": detail,
    })
    path = root / "manifest.jsonl"
    path.write_text("".join(json.dumps(row) + "\n" for row in rows), encoding="utf-8")
    return path, hashlib.sha256(path.read_bytes()).hexdigest()


class SQLEvidenceRunnerTest(unittest.TestCase):
    def test_sql_files_are_select_only_and_side_scoped(self) -> None:
        files = sorted(SQL_DIR.glob("*.sql"))
        self.assertEqual([path.name[:2] for path in files], [f"{index:02d}" for index in range(13)])
        write_statement = re.compile(r"^\s*(INSERT|UPDATE|DELETE|REPLACE|ALTER|CREATE|DROP|TRUNCATE|CALL|LOAD)\b", re.I | re.M)
        for path in files:
            content = path.read_text(encoding="utf-8")
            self.assertIsNone(write_statement.search(content), path.name)
            if path.name != "00_snapshot_fingerprint.sql":
                self.assertIn("violation_code", content, path.name)
                self.assertIn("entity_key", content, path.name)
                self.assertIn("detail", content, path.name)
                self.assertIn("@ab_side", content, path.name)
        self.assertNotIn("manifest.full_gate_not_implemented", RUNNER.read_text(encoding="utf-8"))
        manifest_sql = (SQL_DIR / "11_manifest_state.sql").read_text(encoding="utf-8")
        self.assertEqual(manifest_sql.count("FROM ab_manifest_entities"), 1)
        self.assertIn("CAST('G01' AS BINARY)", manifest_sql)

    def test_corrected_gate_semantics_are_locked(self) -> None:
        task_state = (SQL_DIR / "01_task_state_parity.sql").read_text(encoding="utf-8")
        self.assertIn(
            "'completed', 'closed', 'forcibly_closed', 'closed_by_admin'",
            task_state,
        )
        self.assertNotIn("module_key = 'basic_info'", task_state)
        self.assertIn(
            "tm.state NOT IN ('completed', 'closed', 'forcibly_closed', 'closed_by_admin')",
            task_state,
        )

        asset_roles = (SQL_DIR / "04_asset_role_scope.sql").read_text(encoding="utf-8")
        self.assertIn("r.status <> 'draft'", asset_roles)

        storage = (SQL_DIR / "06_storage_integrity.sql").read_text(encoding="utf-8")
        self.assertIn("u.bound_asset_id = a.id", storage)
        self.assertIn("upload_request_projection_object_mismatch", storage)

        events = (SQL_DIR / "07_event_history_checksum.sql").read_text(encoding="utf-8")
        self.assertNotIn("workflow_trace_missing_task", events)

        planning = (SQL_DIR / "08_planning_retouch.sql").read_text(encoding="utf-8")
        for exact_tombstone_marker in (
            "t.id = 497",
            "s.id = 380",
            "p.code_rule_revision_id = 9",
            "p.client_create_id = 'migration-497'",
        ):
            self.assertIn(exact_tombstone_marker, planning)
        self.assertIn("$.components[12]", planning)

        negative = (SQL_DIR / "10_negative_assertions.sql").read_text(encoding="utf-8")
        self.assertIn("negative.legacy_asset_referenced_by_v8", negative)
        self.assertIn("negative.legacy_asset_with_bound_coordinates", negative)

        timestamp_contract = (
            SQL_DIR / "12_legacy_timestamp_contract.sql"
        ).read_text(encoding="utf-8")
        self.assertIn(
            "NOT BETWEEN 28798 AND 28805",
            timestamp_contract,
        )

    def test_fake_adapter_runs_two_single_sessions_and_builds_gate_report(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            run_id = "sql-runner-test"
            manifest, manifest_sha = write_manifest(root, run_id)
            adapter = root / "fake-mysql.py"
            adapter.write_text("""#!/usr/bin/env python3
import os, pathlib, re, sys
data=sys.stdin.read()
calls=pathlib.Path(os.environ['AB_FAKE_CALLS']); calls.mkdir(exist_ok=True)
path=calls/f'call-{len(list(calls.glob("call-*.sql")))+1}.sql'; path.write_text(data)
for gate in re.findall(r\"SELECT '__AB_GATE__([^']+)' AS ab_gate_marker\", data):
 print('ab_gate_marker'); print('__AB_GATE__'+gate)
 if gate.startswith('00_'): print('metric\\tvalue'); print('fake\\t1')
 else: print('violation_code\\tentity_key\\tdetail')
""", encoding="utf-8")
            adapter.chmod(0o700)
            calls = root / "calls"
            env = os.environ.copy(); env["AB_FAKE_CALLS"] = str(calls)
            command = [
                "bash", str(RUNNER), "--mode", "clone", "--run-id", run_id,
                "--source-db", "clone_a", "--target-db", "clone_b",
                "--evidence-root", str(root / "evidence"), "--mysql-bin", str(adapter),
                "--snapshot-sha256", "b" * 64, "--manifest-jsonl", str(manifest),
                "--manifest-sha256", manifest_sha, "--execute-readonly",
            ]
            result = subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True, check=False)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            call_files = sorted(calls.glob("call-*.sql"))
            self.assertEqual(len(call_files), 2)
            for path in call_files:
                session = path.read_text(encoding="utf-8")
                self.assertEqual(session.count("AS ab_gate_marker"), 13)
                self.assertLess(session.index("CREATE TEMPORARY TABLE"), session.index("START TRANSACTION READ ONLY"))
            source_session = call_files[0].read_text(encoding="utf-8")
            target_session = call_files[1].read_text(encoding="utf-8")
            self.assertNotIn("FROM task_asset_groups g", source_session)
            self.assertIn("FROM task_asset_groups g", target_session)
            run_dir = root / "evidence" / run_id
            report = json.loads((run_dir / "gate_report.json").read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "PASS")
            for side in ("source", "target"):
                self.assertEqual(len(list((run_dir / "sql" / side).glob("??_*.json"))), 13)
                self.assertEqual(len(list((run_dir / "sql" / side).glob("??_*.csv"))), 13)
                self.assertEqual(len(list((run_dir / "sql" / side).glob("??_*.sha256"))), 13)

    def test_adapter_failure_is_nonzero_and_fail_closed(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp); run_id = "sql-runner-fail"
            manifest, manifest_sha = write_manifest(root, run_id)
            adapter = root / "fail-mysql.sh"
            adapter.write_text("#!/bin/sh\nexit 9\n", encoding="utf-8"); adapter.chmod(0o700)
            result = subprocess.run([
                "bash", str(RUNNER), "--mode", "clone", "--run-id", run_id,
                "--source-db", "clone_a", "--target-db", "clone_b",
                "--evidence-root", str(root / "evidence"), "--mysql-bin", str(adapter),
                "--snapshot-sha256", "b" * 64, "--manifest-jsonl", str(manifest),
                "--manifest-sha256", manifest_sha, "--execute-readonly",
            ], cwd=ROOT, text=True, capture_output=True, check=False)
            self.assertNotEqual(result.returncode, 0)
            report = json.loads((root / "evidence" / run_id / "gate_report.json").read_text(encoding="utf-8"))
            self.assertEqual(report["violations"][0]["violation_code"], "runner.mysql_session_failed")


if __name__ == "__main__":
    unittest.main()
