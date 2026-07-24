from __future__ import annotations

import hashlib
import importlib.util
import json
import pathlib
import sys
import tempfile
import unittest


PATH = pathlib.Path(__file__).with_name("manifest_verifier.py")
SPEC = importlib.util.spec_from_file_location("manifest_verifier", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC and SPEC.loader
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class ManifestVerifierTest(unittest.TestCase):
    def make_documents(self, root: pathlib.Path):
        run_id = "formal-run-1"
        manifest = root / "manifest.jsonl"
        observations = root / "observations.json"
        rows = []
        evidence = []
        for gate in sorted(MODULE.REQUIRED_GATES):
            expected = hashlib.sha256(gate.encode()).hexdigest()
            rows.append(
                {
                    "run_id": run_id,
                    "gate_name": gate,
                    "entity_key": "entity",
                    "expected_hash": expected,
                    "expected_state": "approved",
                    "review_state": "pass",
                }
            )
            evidence.append(
                {
                    "violation_code": f"evidence.manifest_state.{gate}",
                    "entity_key": "entity",
                    "detail": expected,
                }
            )
        decision = {"decision": "confirmed"}
        rows.append(
            {
                "run_id": run_id,
                "gate_name": "G10",
                "entity_key": "release-decision",
                "expected_hash": hashlib.sha256(
                    json.dumps(
                        decision,
                        sort_keys=True,
                        separators=(",", ":"),
                    ).encode()
                ).hexdigest(),
                "expected_state": "confirmed",
                "review_state": "pass",
                "detail_json": decision,
            }
        )
        manifest.write_text(
            "".join(json.dumps(row, sort_keys=True) + "\n" for row in rows),
            encoding="utf-8",
        )
        observations.write_text(
            json.dumps({"violation_count": 0, "evidence": evidence}) + "\n",
            encoding="utf-8",
        )
        return run_id, manifest, observations

    def test_exact_manifest_observations_emit_direct_pass_envelope(self):
        with tempfile.TemporaryDirectory() as raw:
            run_id, manifest, observations = self.make_documents(
                pathlib.Path(raw)
            )
            result = MODULE.verify(manifest, observations, run_id)
            self.assertEqual("PASS", result["status"])
            self.assertEqual(result["expected_entities"], result["observed_entities"])
            self.assertEqual(
                sorted(MODULE.REQUIRED_GATES), result["required_gates"]
            )
            evidence = result.pop("evidence_sha256")
            self.assertEqual(
                evidence,
                hashlib.sha256(MODULE.canonical_bytes(result)).hexdigest(),
            )

    def test_unreviewed_observation_fails(self):
        with tempfile.TemporaryDirectory() as raw:
            run_id, manifest, observations = self.make_documents(
                pathlib.Path(raw)
            )
            payload = json.loads(observations.read_text(encoding="utf-8"))
            payload["evidence"].append(
                {
                    "violation_code": "evidence.manifest_state.G01",
                    "entity_key": "unreviewed",
                    "detail": "f" * 64,
                }
            )
            observations.write_text(json.dumps(payload), encoding="utf-8")
            result = MODULE.verify(manifest, observations, run_id)
            self.assertEqual("FAIL", result["status"])
            self.assertIn(
                "manifest.unreviewed_entity",
                {row["violation_code"] for row in result["violations"]},
            )


if __name__ == "__main__":
    unittest.main()
