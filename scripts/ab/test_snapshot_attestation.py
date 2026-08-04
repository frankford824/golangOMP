from __future__ import annotations

import argparse
import importlib.util
import json
import pathlib
import sys
import tempfile
import unittest


PATH = pathlib.Path(__file__).with_name("snapshot_attestation.py")
SPEC = importlib.util.spec_from_file_location("snapshot_attestation", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC and SPEC.loader
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class SnapshotAttestationTest(unittest.TestCase):
    def make_inputs(self, root: pathlib.Path):
        snapshot = root / "snapshot.sql.gz"
        coordinates = root / "coordinates.json"
        fingerprint = root / "fingerprint.json"
        snapshot.write_bytes(b"snapshot")
        coordinates.write_text('{"binlog":"file:1"}\n', encoding="utf-8")
        fingerprint.write_text('{"tasks":2}\n', encoding="utf-8")
        return snapshot, coordinates, fingerprint

    def create_attestation(
        self, root: pathlib.Path, label: str, database: str
    ) -> pathlib.Path:
        snapshot, coordinates, fingerprint = self.make_inputs(root)
        receipt = root / f"{label.lower()}-receipt.json"
        output = root / f"{label.lower()}-attestation.json"
        receipt.write_text(
            json.dumps({"side": label}, sort_keys=True) + "\n",
            encoding="utf-8",
        )
        code = MODULE.create(
            argparse.Namespace(
                run_id="formal-run-1",
                clone_label=label,
                clone_database=database,
                snapshot_file=snapshot,
                source_coordinates=coordinates,
                baseline_fingerprint=fingerprint,
                import_receipt=receipt,
                output=output,
            )
        )
        self.assertEqual(0, code)
        return output

    def test_exact_a_b_attestations_pass_with_direct_finalizer_envelope(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            source = self.create_attestation(root, "A", "ab_formal_a")
            target = self.create_attestation(root, "B", "ab_formal_b")
            output = root / "verdict.json"
            code = MODULE.verify(
                argparse.Namespace(
                    run_id="formal-run-1",
                    source=source,
                    target=target,
                    expected_snapshot_sha256=MODULE.sha(root / "snapshot.sql.gz"),
                    output=output,
                )
            )
            verdict = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(0, code)
            self.assertEqual("PASS", verdict["status"])
            self.assertEqual(0, verdict["violation_count"])
            evidence = verdict.pop("evidence_sha256")
            self.assertEqual(
                evidence,
                __import__("hashlib").sha256(MODULE.canonical_bytes(verdict)).hexdigest(),
            )

    def test_run_or_snapshot_drift_fails(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            source = self.create_attestation(root, "A", "ab_formal_a")
            target = self.create_attestation(root, "B", "ab_formal_b")
            value = json.loads(target.read_text(encoding="utf-8"))
            value["run_id"] = "other-run"
            target.write_bytes(MODULE.canonical_bytes(value))
            output = root / "verdict.json"
            code = MODULE.verify(
                argparse.Namespace(
                    run_id="formal-run-1",
                    source=source,
                    target=target,
                    expected_snapshot_sha256="f" * 64,
                    output=output,
                )
            )
            verdict = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(1, code)
            self.assertEqual("FAIL", verdict["status"])
            self.assertGreaterEqual(verdict["violation_count"], 2)

    def test_duplicate_clone_database_fails(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            source = self.create_attestation(root, "A", "ab_same")
            target = self.create_attestation(root, "B", "ab_same")
            output = root / "verdict.json"
            code = MODULE.verify(
                argparse.Namespace(
                    run_id="formal-run-1",
                    source=source,
                    target=target,
                    expected_snapshot_sha256="",
                    output=output,
                )
            )
            self.assertEqual(1, code)
            verdict = json.loads(output.read_text(encoding="utf-8"))
            self.assertIn(
                "snapshot.clone_not_distinct",
                {row["violation_code"] for row in verdict["violations"]},
            )


if __name__ == "__main__":
    unittest.main()
