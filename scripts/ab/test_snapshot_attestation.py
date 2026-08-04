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
        coordinates.write_text(
            json.dumps(
                {
                    "binlog_file": "binlog.000001",
                    "binlog_position": 42,
                    "snapshot_sha256": MODULE.sha(snapshot),
                },
                sort_keys=True,
            )
            + "\n",
            encoding="utf-8",
        )
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

    def create_physical_attestation(
        self,
        root: pathlib.Path,
        label: str,
        *,
        database_port: int,
        container_suffix: str,
    ) -> pathlib.Path:
        snapshot, coordinates, fingerprint = self.make_inputs(root)
        receipt = root / f"{label.lower()}-physical-receipt.json"
        inspect_file = root / f"{label.lower()}-inspect.json"
        output = root / f"{label.lower()}-physical-attestation.json"
        receipt.write_text(
            json.dumps({"side": label}, sort_keys=True) + "\n",
            encoding="utf-8",
        )
        inspect_file.write_text(
            json.dumps({"container": container_suffix}) + "\n",
            encoding="utf-8",
        )
        code = MODULE.create(
            argparse.Namespace(
                run_id="formal-run-1",
                clone_label=label,
                clone_database="jst_erp",
                snapshot_file=snapshot,
                source_coordinates=coordinates,
                baseline_fingerprint=fingerprint,
                import_receipt=receipt,
                output=output,
                physical_docker_isolation=True,
                database_host="127.0.0.1",
                database_port=database_port,
                container_port=3306,
                container_name=f"yongbo-clone-{label.lower()}-{container_suffix}",
                container_id=(
                    ("a" if label == "A" else "b") * 63
                    + container_suffix
                )[:64],
                container_image_digest="sha256:" + "c" * 64,
                container_inspect_file=inspect_file,
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

    def test_physical_clone_same_database_passes_with_distinct_isolation(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            source = self.create_physical_attestation(
                root, "A", database_port=3332, container_suffix="1"
            )
            target = self.create_physical_attestation(
                root, "B", database_port=3331, container_suffix="2"
            )
            output = root / "verdict.json"
            code = MODULE.verify(
                argparse.Namespace(
                    run_id="formal-run-1",
                    source=source,
                    target=target,
                    expected_snapshot_sha256=MODULE.sha(
                        root / "snapshot.sql.gz"
                    ),
                    output=output,
                )
            )
            verdict = json.loads(output.read_text(encoding="utf-8"))
        self.assertEqual(0, code)
        self.assertEqual("PASS", verdict["status"])
        self.assertEqual(2, verdict["schema_version"])

    def test_physical_clone_requires_distinct_port_container_and_inspect(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            source = self.create_physical_attestation(
                root, "A", database_port=3332, container_suffix="1"
            )
            target = self.create_physical_attestation(
                root, "B", database_port=3331, container_suffix="2"
            )
            a = json.loads(source.read_text(encoding="utf-8"))
            b = json.loads(target.read_text(encoding="utf-8"))
            for field in (
                "database_port",
                "container_name",
                "container_id",
                "container_inspect_sha256",
            ):
                changed = dict(b)
                changed[field] = a[field]
                target.write_bytes(MODULE.canonical_bytes(changed))
                output = root / f"{field}.json"
                code = MODULE.verify(
                    argparse.Namespace(
                        run_id="formal-run-1",
                        source=source,
                        target=target,
                        expected_snapshot_sha256="",
                        output=output,
                    )
                )
                verdict = json.loads(output.read_text(encoding="utf-8"))
                self.assertEqual(1, code, field)
                self.assertIn(
                    "snapshot.clone_not_distinct",
                    {
                        row["violation_code"]
                        for row in verdict["violations"]
                    },
                    field,
                )
                target.write_bytes(MODULE.canonical_bytes(b))

    def test_physical_clone_requires_same_snapshot_baseline_and_coordinates(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            source = self.create_physical_attestation(
                root, "A", database_port=3332, container_suffix="1"
            )
            target = self.create_physical_attestation(
                root, "B", database_port=3331, container_suffix="2"
            )
            original = json.loads(target.read_text(encoding="utf-8"))
            mutations = []
            baseline = json.loads(json.dumps(original))
            baseline["baseline_fingerprint_sha256"] = "e" * 64
            mutations.append(baseline)
            coordinates = json.loads(json.dumps(original))
            coordinates["source_coordinates"]["binlog_position"] = 43
            mutations.append(coordinates)
            snapshot = json.loads(json.dumps(original))
            snapshot["snapshot_sha256"] = "f" * 64
            snapshot["source_compound_snapshot_sha256"] = "f" * 64
            snapshot["source_coordinates"]["snapshot_sha256"] = "f" * 64
            mutations.append(snapshot)
            for index, changed in enumerate(mutations):
                target.write_bytes(MODULE.canonical_bytes(changed))
                output = root / f"identity-{index}.json"
                code = MODULE.verify(
                    argparse.Namespace(
                        run_id="formal-run-1",
                        source=source,
                        target=target,
                        expected_snapshot_sha256="",
                        output=output,
                    )
                )
                verdict = json.loads(output.read_text(encoding="utf-8"))
                self.assertEqual(1, code)
                self.assertIn(
                    "snapshot.identity_mismatch",
                    {
                        row["violation_code"]
                        for row in verdict["violations"]
                    },
                )

    def test_physical_attestation_rejects_non_loopback_or_arbitrary_jst_erp(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            physical = self.create_physical_attestation(
                root, "A", database_port=3332, container_suffix="1"
            )
            value = json.loads(physical.read_text(encoding="utf-8"))
            value["database_host"] = "192.168.0.10"
            violations = MODULE.validate_attestation(
                value,
                label="A",
                expected_run_id="formal-run-1",
                expected_clone_label="A",
            )
            self.assertIn(
                "snapshot.attestation_physical_isolation",
                {row["violation_code"] for row in violations},
            )
            logical = dict(value)
            for field in MODULE.PHYSICAL_ISOLATION_FIELDS:
                logical.pop(field)
            logical["schema_version"] = 1
            violations = MODULE.validate_attestation(
                logical,
                label="A",
                expected_run_id="formal-run-1",
                expected_clone_label="A",
            )
            self.assertIn(
                "snapshot.attestation_database",
                {row["violation_code"] for row in violations},
            )

    def test_physical_attestation_rejects_bad_port_id_digest_and_snapshot(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            physical = self.create_physical_attestation(
                root, "A", database_port=3332, container_suffix="1"
            )
            original = json.loads(physical.read_text(encoding="utf-8"))
            mutations = {
                "database_port": 3306,
                "container_port": 3307,
                "container_id": "bad",
                "container_image_digest": "c" * 64,
                "source_compound_snapshot_sha256": "d" * 64,
                "production_write_performed": True,
            }
            for field, bad in mutations.items():
                with self.subTest(field=field):
                    value = dict(original)
                    value[field] = bad
                    violations = MODULE.validate_attestation(
                        value,
                        label="A",
                        expected_run_id="formal-run-1",
                        expected_clone_label="A",
                    )
                    self.assertTrue(violations)


if __name__ == "__main__":
    unittest.main()
