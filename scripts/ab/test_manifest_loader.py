from __future__ import annotations

import hashlib
import json
import pathlib
import tempfile
import unittest

from manifest_loader import DATABASE_GATES, build_manifest, canonical_json, emit_sql, hash_components, load_rows


DERIVATION = {
    "G01": "reviewed_mapping_a_truth", "G02": "reviewed_mapping_a_truth",
    "G03": "reviewed_mapping_a_truth", "G04": "reviewed_mapping_a_truth",
    "G05": "reviewed_mapping_a_truth", "G07": "immutable_a_truth",
    "G08": "reviewed_mapping_a_truth", "G09": "independent_projection",
}


class ManifestLoaderTest(unittest.TestCase):
    def make_manifest(self, root: pathlib.Path, mutate=None) -> tuple[pathlib.Path, str]:
        run_id = "audit-run"
        inputs = {"mapping_sha256": "1" * 64}
        rows = []
        for gate in sorted(DATABASE_GATES):
            components = [gate, "entity"]
            rows.append({
                "run_id": run_id, "gate_name": gate, "entity_key": f"{gate}:entity",
                "expected_hash": hash_components(components), "expected_state": "approved", "review_state": "pass",
                "detail_json": {"derivation_method": DERIVATION[gate], "input_sha256": inputs, "components": components},
            })
        object_detail = {"derivation_method": "object_verifier", "input_sha256": inputs, "verdict": "PASS"}
        rows.append({
            "run_id": run_id, "gate_name": "G06", "entity_key": "object-verdict",
            "expected_hash": hashlib.sha256(canonical_json(object_detail).encode()).hexdigest(),
            "expected_state": "verified", "review_state": "pass", "detail_json": object_detail,
        })
        decision = {"derivation_method": "human_decision", "input_sha256": inputs, "decision": "confirmed"}
        rows.append({
            "run_id": run_id, "gate_name": "G10", "entity_key": "release-decision",
            "expected_hash": hashlib.sha256(canonical_json(decision).encode()).hexdigest(),
            "expected_state": "confirmed", "review_state": "pass", "detail_json": decision,
        })
        if mutate:
            mutate(rows)
        path = root / "manifest.jsonl"
        path.write_text("".join(json.dumps(row, ensure_ascii=False) + "\n" for row in rows), encoding="utf-8")
        return path, hashlib.sha256(path.read_bytes()).hexdigest()

    def test_valid_manifest_emits_injection_safe_temp_sql(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            path, digest = self.make_manifest(root)
            rows = load_rows(path, digest, "audit-run")
            output = root / "manifest.sql"
            emit_sql(rows, output)
            sql = output.read_text(encoding="utf-8")
            self.assertIn("CREATE TEMPORARY TABLE ab_manifest_entities", sql)
            self.assertIn(
                "DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
                sql,
            )
            self.assertIn("PRIMARY KEY (gate_name, entity_key)", sql)
            self.assertIn("CAST(CONVERT(0x", sql)

    def test_duplicate_gate_entity_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            path, digest = self.make_manifest(root, lambda rows: rows.append(dict(rows[0])))
            with self.assertRaisesRegex(ValueError, "duplicate gate/entity"):
                load_rows(path, digest, "audit-run")

    def test_component_hash_is_verified_before_sql(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            path, digest = self.make_manifest(root, lambda rows: rows[0].update(expected_hash="f" * 64))
            with self.assertRaisesRegex(ValueError, "component hash mismatch"):
                load_rows(path, digest, "audit-run")

    def test_build_binds_all_source_artifact_hashes(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            mapping = root / "mapping.json"
            mapping.write_text(json.dumps({"version": 2, "resources": [], "planning_tasks": []}), encoding="utf-8")
            baseline = root / "baseline.json"
            baseline.write_text(json.dumps({"snapshot_sha256": "a" * 64, "baseline_fingerprint_sha256": "b" * 64}), encoding="utf-8")
            decisions = root / "decisions.json"
            decisions.write_text(json.dumps({"decision": "confirmed"}), encoding="utf-8")
            objects = root / "objects.json"
            objects.write_text(json.dumps({"status": "PASS", "violation_count": 0}), encoding="utf-8")
            input_hashes = {
                "mapping_sha256": hashlib.sha256(mapping.read_bytes()).hexdigest(),
                "baseline_attestation_sha256": hashlib.sha256(baseline.read_bytes()).hexdigest(),
                "approved_decisions_sha256": hashlib.sha256(decisions.read_bytes()).hexdigest(),
                "object_verdict_sha256": hashlib.sha256(objects.read_bytes()).hexdigest(),
            }
            entities = []
            for gate in sorted(DATABASE_GATES):
                entities.append({"gate_name": gate, "entity_key": f"{gate}:entity", "review_state": "pass", "derivation_method": DERIVATION[gate], "components": [gate]})
            entities.extend([
                {"gate_name": "G06", "entity_key": "objects", "expected_state": "verified", "review_state": "pass", "derivation_method": "object_verifier"},
                {"gate_name": "G10", "entity_key": "decision", "expected_state": "confirmed", "review_state": "pass", "derivation_method": "human_decision"},
            ])
            entity_input = root / "entities.json"
            entity_input.write_text(json.dumps({"schema_version": 1, "input_sha256": input_hashes, "entities": entities}), encoding="utf-8")
            output = root / "reviewed.jsonl"
            build_manifest("audit-run", entity_input, mapping, baseline, decisions, objects, output)
            rows = load_rows(output, hashlib.sha256(output.read_bytes()).hexdigest(), "audit-run")
            self.assertEqual(len(rows), 10)

    def test_build_refuses_unconfirmed_mapping(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            mapping = root / "mapping.json"
            mapping.write_text(json.dumps({"version": 2, "resources": [{"history": [{"confidence": "proposed_review"}]}]}), encoding="utf-8")
            baseline = root / "baseline.json"; baseline.write_text(json.dumps({"snapshot_sha256": "a", "baseline_fingerprint_sha256": "b"}))
            decisions = root / "decisions.json"; decisions.write_text(json.dumps({"decision": "confirmed"}))
            objects = root / "objects.json"; objects.write_text(json.dumps({"status": "PASS", "violation_count": 0}))
            source = root / "entities.json"; source.write_text(json.dumps({"schema_version": 1, "input_sha256": {}, "entities": [{}]}))
            with self.assertRaisesRegex(ValueError, "not confirmed_auto"):
                build_manifest("audit-run", source, mapping, baseline, decisions, objects, root / "out")

    def test_build_consumes_hash_bound_projection_jsonl(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            mapping = root / "mapping.json"
            mapping.write_text(json.dumps({"version": 2, "resources": [], "planning_tasks": []}), encoding="utf-8")
            baseline = root / "baseline.json"
            baseline.write_text(json.dumps({"snapshot_sha256": "a" * 64, "baseline_fingerprint_sha256": "b" * 64}), encoding="utf-8")
            decisions = root / "decisions.json"; decisions.write_text(json.dumps({"decision": "confirmed"}), encoding="utf-8")
            objects = root / "objects.json"; objects.write_text(json.dumps({"status": "PASS", "violation_count": 0}), encoding="utf-8")
            projection = root / "projection.jsonl"
            projection.write_text(json.dumps({"gate_name": "G09", "entity_key": "task-search:1",
                "expected_state": "approved", "review_state": "pass", "derivation_method": "independent_projection",
                "components": ["1", "design_task", "Completed", "", "c" * 64], "detail": {"algorithm_sha256": "d" * 64}}) + "\n", encoding="utf-8")
            input_hashes = {
                "mapping_sha256": hashlib.sha256(mapping.read_bytes()).hexdigest(),
                "baseline_attestation_sha256": hashlib.sha256(baseline.read_bytes()).hexdigest(),
                "approved_decisions_sha256": hashlib.sha256(decisions.read_bytes()).hexdigest(),
                "object_verdict_sha256": hashlib.sha256(objects.read_bytes()).hexdigest(),
                "projection_expected_sha256": hashlib.sha256(projection.read_bytes()).hexdigest(),
            }
            entities = []
            for gate in sorted(DATABASE_GATES - {"G09"}):
                entities.append({"gate_name": gate, "entity_key": f"{gate}:entity", "review_state": "pass",
                    "derivation_method": DERIVATION[gate], "components": [gate]})
            entities.extend([
                {"gate_name": "G06", "entity_key": "objects", "expected_state": "verified", "review_state": "pass", "derivation_method": "object_verifier"},
                {"gate_name": "G10", "entity_key": "decision", "expected_state": "confirmed", "review_state": "pass", "derivation_method": "human_decision"},
            ])
            source = root / "entities.json"
            source.write_text(json.dumps({"schema_version": 1, "input_sha256": input_hashes, "entities": entities}), encoding="utf-8")
            output = root / "reviewed.jsonl"
            build_manifest("audit-run", source, mapping, baseline, decisions, objects, output, projection)
            rows = load_rows(output, hashlib.sha256(output.read_bytes()).hexdigest(), "audit-run")
            g09 = next(row for row in rows if row["gate_name"] == "G09")
            self.assertEqual(g09["entity_key"], "task-search:1")
            self.assertEqual(json.loads(g09["detail_json"])["input_sha256"]["projection_expected_sha256"], input_hashes["projection_expected_sha256"])


if __name__ == "__main__":
    unittest.main()
