from __future__ import annotations

import argparse
import copy
import hashlib
import json
import pathlib
import tempfile
import unittest
from unittest import mock

from build_canonical_entities import (
    FREEZE_SQL,
    build,
    build_entities,
    canonical_json,
    mysql_time,
    persisted_reason,
    run_freeze,
    validate_inputs,
)
from manifest_loader import build_manifest, load_rows, sha256_file


class CanonicalEntitiesTest(unittest.TestCase):
    def mapping(self):
        revision = {
            "revision_no": 1, "status": "finalized", "mode": "single", "source_stage": "design",
            "source_alias_from_task_asset_id": 100, "final_task_asset_ids": [100],
            "reference_file_ref_ids": [300], "evidence_event_ids": ["event:1"],
            "confidence": "confirmed_auto", "review_policy_ids": ["delivery_source_alias"],
            "confirmed_by": 1, "confirmed_at": "2026-07-23T01:02:03Z",
            "confirmation_note": "reviewed", "manifest_row_hash": "b" * 64,
            "reason": "legacy replay", "created_by": 1, "created_at": "2026-01-01T00:00:00Z",
            "submitted_at": "2026-01-01T00:00:01Z", "finalized_at": "2026-01-01T00:00:02.123Z",
        }
        return {
            "version": 2,
            "resources": [{
                "task_id": 10, "scope_kind": "task", "scope_ref_id": 0,
                "history": [revision], "working_revision_no": 1, "finalized_revision_no": 1,
            }],
            "planning_tasks": [{
                "task_id": 20, "target_task_status": "Completed", "code_rule_revision_id": 9,
                "created_by": 1, "confidence": "confirmed_auto", "review_policy_ids": [],
                "confirmed_by": 1, "confirmed_at": "2026-07-23T00:00:00Z",
                "confirmation_note": "reviewed", "items": [{
                    "task_sku_item_id": 200, "description_spec": "spec", "quantity": 2,
                    "target_price": "12.30", "note": "", "reference_url": "",
                    "erp_product_i_id": "ERP", "erp_product_name": "Product",
                    "image_storage_ref_id": "img-ref",
                }],
            }],
            "task_state_decisions": [],
        }

    def rows(self):
        return {
            "meta": [{"task_count": 2, "group_count": 1, "revision_count": 0, "max_task_asset_id": 100}],
            "task": [
                {"id": 10, "task_type": "design_task", "task_status": "PendingAuditA", "current_handler_id": 7, "workflow_revision": 3},
                {"id": 20, "task_type": "purchase_task", "task_status": "InProgress", "current_handler_id": None, "workflow_revision": 0},
            ],
            "group": [{"id": 1, "task_id": 10, "scope_kind": "task", "scope_ref_id": 0, "migration_incomplete": 1, "migration_issue": "legacy"}],
            "asset": [{"id": 100, "asset_id": 900, "task_id": 10, "asset_type": "delivery",
                       "storage_ref_id": "ref-100", "whole_hash": "c" * 64,
                       "binding_state": "staged", "bound_role": None, "scope_sku_code": "", "retouch_requirement_id": None}],
            "sku": [{"id": 200, "task_id": 20, "sku_code": "SKU-20"}],
            "reference": [{"id": 300, "task_id": 10, "ref_id": "ref-300", "sku_item_id": None,
                           "retouch_requirement_id": None, "file_name": "reference.png"}],
            "task_event": [{"id": "event:1", "task_id": 10, "sequence": 1, "event_type": "submitted",
                            "operator_id": 1, "payload_text": '{"a": 1}', "created_at_text": "2026-01-01T00:00:01.000000"}],
            "module_event": [{"id": 9, "task_module_id": 8, "event_type": "state", "from_state": None,
                              "to_state": "done", "actor_id": None, "actor_snapshot_text": None,
                              "payload_text": "{}", "created_at_text": "2026-01-01T00:00:02.000000"}],
            "planning_revision": [],
            "planning_setting": [],
            "retouch": [{"id": 400, "task_id": 10, "description": "retouch", "sku_code": None,
                         "spec": None, "remark": None, "sort_order": 0, "deleted": 0}],
        }

    def projection(self):
        return [{
            "gate_name": "G09", "entity_key": "task-search:10", "expected_state": "matched",
            "review_state": "pass", "derivation_method": "independent_projection",
            "components": ["10", "design_task", "PendingAudit"], "detail": {"projection_kind": "task"},
        }]

    def test_builds_complete_deterministic_contract(self):
        first = build_entities(self.mapping(), self.rows(), "a" * 64, self.projection(),
                               {"decision": "confirmed", "reviewer_id": 1}, {"status": "PASS", "violation_count": 0})
        second = build_entities(self.mapping(), self.rows(), "a" * 64, self.projection(),
                                {"decision": "confirmed", "reviewer_id": 1}, {"status": "PASS", "violation_count": 0})
        self.assertEqual(canonical_json(first), canonical_json(second))
        self.assertEqual({row["gate_name"] for row in first}, {f"G{i:02d}" for i in range(1, 11)} - {"G11"})
        task = next(row for row in first if row["entity_key"] == "task:10")
        self.assertEqual(task["components"], ["10", "design_task", "PendingAudit", "7", "4"])
        planning_task = next(row for row in first if row["entity_key"] == "task:20")
        self.assertEqual(planning_task["components"], ["20", "sku_planning", "Completed", "", "1"])
        revision = next(row for row in first if row["entity_key"] == "revision:10:task:0:1")
        self.assertEqual(revision["components"][6], "asset:900:ref-100")
        self.assertTrue(revision["components"][9].endswith("first_evidence=event:1]"))
        source = next(row for row in first if row["entity_key"] == "revision-source:10:task:0:1")
        self.assertEqual(source["components"], ["asset:900:ref-100", "source", "c" * 64, "bound", "source", "", ""])
        final = next(row for row in first if row["entity_key"] == "revision-final:10:task:0:1:0")
        self.assertEqual(final["components"], ["100", "0", "", "delivery", "c" * 64, "bound", "final", "", ""])
        reference = next(row for row in first if row["entity_key"] == "revision-reference:10:task:0:1:0")
        self.assertEqual(reference["components"], ["300", "", "0", "ref-300", "reference.png", "task"])
        planning = next(row for row in first if row["entity_key"] == "planning-revision:200:1")
        self.assertEqual(planning["components"][4:6], ["12.30", "CNY"])

    def test_source_bundle_moves_alias_sequence_and_uses_bundle_hash(self):
        mapping = self.mapping()
        bundle_revision = copy.deepcopy(mapping["resources"][0]["history"][0])
        bundle_revision.pop("source_alias_from_task_asset_id")
        bundle_revision["source_bundle"] = {
            "task_asset_id": 500, "format": "zip", "bundle_sha256": "d" * 64,
            "manifest_sha256": "e" * 64, "members": [], "confirmed_by": 1,
            "confirmed_at": "2026-07-23T00:00:00Z", "confirmation_note": "reviewed",
        }
        mapping["resources"][0]["history"] = [bundle_revision]
        entities = build_entities(mapping, self.rows(), "a" * 64, self.projection(),
                                  {"decision": "confirmed"}, {"status": "PASS", "violation_count": 0})
        source = next(row for row in entities if row["entity_key"].startswith("revision-source:"))
        self.assertEqual(source["components"][:3], ["bundle:" + "d" * 64, "source", "d" * 64])

    def test_recovery_hash_is_applied_to_expected_final_asset(self):
        mapping = self.mapping()
        mapping["asset_recoveries"] = [{
            "task_id": 10,
            "missing_task_asset_id": 100,
            "strategy": "verified_oss_recovery_v1",
            "recovery_source_sha256": "e" * 64,
        }]
        entities = build_entities(
            mapping,
            self.rows(),
            "a" * 64,
            self.projection(),
            {"decision": "confirmed"},
            {"status": "PASS", "violation_count": 0},
        )
        final = next(
            row for row in entities
            if row["entity_key"] == "revision-final:10:task:0:1:0"
        )
        self.assertEqual(final["components"][4], "e" * 64)

    def test_reviewed_state_decision_precedes_generic_legacy_mapping(self):
        mapping = self.mapping()
        mapping["task_state_decisions"] = [{
            "task_id": 10,
            "from_status": "PendingAuditA",
            "target_status": "InProgress",
            "confidence": "confirmed_auto",
            "confirmed_by": 1,
            "confirmed_at": "2026-07-24T00:00:00Z",
            "confirmation_note": "reviewed exception",
            "manifest_row_hash": "d" * 64,
        }]
        entities = build_entities(
            mapping,
            self.rows(),
            "a" * 64,
            self.projection(),
            {"decision": "confirmed"},
            {"status": "PASS", "violation_count": 0},
        )
        task = next(row for row in entities if row["entity_key"] == "task:10")
        self.assertEqual(task["components"], ["10", "design_task", "InProgress", "7", "4"])

    def test_existing_revision_and_coverage_drift_fail_closed(self):
        rows = self.rows()
        rows["meta"][0]["revision_count"] = 1
        with self.assertRaisesRegex(ValueError, "already contains resource revisions"):
            build_entities(self.mapping(), rows, "a" * 64, self.projection(),
                           {"decision": "confirmed"}, {"status": "PASS", "violation_count": 0})
        rows = self.rows()
        rows["group"] = []
        rows["meta"][0]["group_count"] = 0
        created = build_entities(
            self.mapping(),
            rows,
            "a" * 64,
            self.projection(),
            {"decision": "confirmed"},
            {"status": "PASS", "violation_count": 0},
        )
        self.assertTrue(
            any(row["entity_key"] == "group:10:task:0" for row in created)
        )
        rows = self.rows()
        rows["group"].append(
            {
                "id": 2,
                "task_id": 20,
                "scope_kind": "task",
                "scope_ref_id": 0,
                "migration_incomplete": 1,
                "migration_issue": "unmapped",
            }
        )
        rows["meta"][0]["group_count"] = 2
        with self.assertRaisesRegex(ValueError, "unmapped"):
            build_entities(
                self.mapping(),
                rows,
                "a" * 64,
                self.projection(),
                {"decision": "confirmed"},
                {"status": "PASS", "violation_count": 0},
            )

    def test_unreviewed_and_nonpass_inputs_fail_closed(self):
        baseline = {"snapshot_sha256": "a" * 64, "baseline_fingerprint_sha256": "b" * 64}
        decisions = {"decision": "confirmed"}
        verdict = {"status": "PASS", "violation_count": 0}
        validate_inputs(self.mapping(), baseline, decisions, verdict)
        bad = self.mapping()
        bad["resources"][0]["history"][0]["confidence"] = "proposed_review"
        with self.assertRaisesRegex(ValueError, "confirmed_auto"):
            validate_inputs(bad, baseline, decisions, verdict)
        with self.assertRaisesRegex(ValueError, "PASS"):
            validate_inputs(self.mapping(), baseline, decisions, {"status": "FAIL", "violation_count": 1})

    def test_time_and_reason_match_go_contract(self):
        revision = self.mapping()["resources"][0]["history"][0]
        self.assertEqual(mysql_time("2026-01-01T00:00:02.123Z"), "2026-01-01T00:00:02.123000")
        self.assertEqual(
            mysql_time("2026-07-23T05:58:03.45506Z"),
            "2026-07-23T05:58:03.455060",
        )
        with self.assertRaisesRegex(ValueError, "RFC3339"):
            mysql_time("2026-07-23T05:58:03.1234567Z")
        self.assertIn("confirmed_at=2026-07-23T01:02:03Z", persisted_reason(revision))
        revision["reason"] = "审阅历史证据" * 200
        compact = persisted_reason(revision)
        self.assertLessEqual(len(compact), 512)
        self.assertNotIn(revision["reason"], compact)
        self.assertIn(
            "reason_sha256="
            + hashlib.sha256(revision["reason"].encode("utf-8")).hexdigest(),
            compact,
        )

    @mock.patch("build_canonical_entities.subprocess.run")
    def test_freeze_is_loopback_and_read_only(self, subprocess_run):
        subprocess_run.return_value = argparse.Namespace(
            returncode=0, stderr="",
            stdout='meta\t{"task_count":1,"group_count":0,"revision_count":0,"max_task_asset_id":0}\n'
                   'task\t{"id":1,"task_type":"x","task_status":"InProgress","current_handler_id":null,"workflow_revision":0}\n',
        )
        args = argparse.Namespace(host="127.0.0.1", port=3306, user="reader", database="clone_a",
                                  defaults_extra_file=None, mysql="mysql")
        rows, digest = run_freeze(args)
        self.assertEqual(len(rows["task"]), 1)
        self.assertEqual(len(digest), 64)
        sql = subprocess_run.call_args.kwargs["input"]
        self.assertIn("SET SESSION TRANSACTION READ ONLY", sql)
        self.assertIn("START TRANSACTION WITH CONSISTENT SNAPSHOT, READ ONLY", sql)
        self.assertNotRegex(sql, r"\b(INSERT|UPDATE|DELETE|REPLACE)\b")
        args.host = "yongbo.cloud"
        with self.assertRaisesRegex(ValueError, "loopback"):
            run_freeze(args)

    @mock.patch("build_canonical_entities.run_freeze")
    def test_file_output_is_manifest_loader_bound_and_atomic(self, freeze):
        freeze.return_value = (self.rows(), "f" * 64)
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            files = {
                "mapping": self.mapping(),
                "baseline": {"snapshot_sha256": "a" * 64, "baseline_fingerprint_sha256": "b" * 64},
                "decisions": {"decision": "confirmed", "reviewer_id": 1},
                "verdict": {"status": "PASS", "violation_count": 0},
            }
            for name, value in files.items():
                (root / f"{name}.json").write_text(canonical_json(value), encoding="utf-8")
            projection = root / "projection.jsonl"
            projection.write_text(canonical_json(self.projection()[0]) + "\n", encoding="utf-8")
            output = root / "canonical.json"
            args = argparse.Namespace(
                mapping=str(root / "mapping.json"), baseline_attestation=str(root / "baseline.json"),
                approved_decisions=str(root / "decisions.json"), object_verdict=str(root / "verdict.json"),
                projection_expected=str(projection), host="127.0.0.1", port=3306, user="reader",
                database="clone_a", defaults_extra_file=None, mysql="mysql", output=str(output),
            )
            build(args)
            document = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(document["schema_version"], 1)
            self.assertEqual(document["input_sha256"]["mapping_sha256"],
                             hashlib.sha256((root / "mapping.json").read_bytes()).hexdigest())
            self.assertEqual(document["input_sha256"]["projection_expected_sha256"],
                             hashlib.sha256(projection.read_bytes()).hexdigest())
            self.assertFalse(any(path.name.startswith(".canonical.json.") for path in root.iterdir()))
            manifest = root / "reviewed-manifest.jsonl"
            build_manifest(
                "canonical-test", output, root / "mapping.json", root / "baseline.json",
                root / "decisions.json", root / "verdict.json", manifest, projection,
            )
            loaded = load_rows(manifest, sha256_file(manifest), "canonical-test")
            self.assertTrue(loaded)
            self.assertEqual({row["gate_name"] for row in loaded},
                             {f"G{i:02d}" for i in range(1, 11)})

    def test_sql_contract_text_has_no_mutation(self):
        self.assertIn("SET SESSION TRANSACTION READ ONLY", FREEZE_SQL)
        self.assertNotRegex(FREEZE_SQL, r"\b(INSERT|UPDATE|DELETE|REPLACE)\b")
        self.assertNotIn("i.id", FREEZE_SQL)
        self.assertIn(
            "ORDER BY r.task_sku_item_id,r.version_no,i.storage_ref_id",
            FREEZE_SQL,
        )


if __name__ == "__main__":
    unittest.main()
