from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import tempfile
import unittest
from unittest import mock

from projection_expected import (
    ALGORITHM_SPEC,
    apply_recovery_plan,
    build_expected,
    canonical_json,
    materialized_bundle_assets,
    mysql_concat_ws,
    require_reviewed_mapping,
    sha256_text,
)


class ProjectionExpectedTest(unittest.TestCase):
    snapshot = "a" * 64

    def write_inputs(self, root: pathlib.Path, second_asset: bool = False, missing_source: bool = False):
        task = {
            "id": 10, "task_no": "RW-10", "product_name_snapshot": "Product", "sku_code": "SKU",
            "primary_sku_code": "P-SKU", "task_type": "new_product_development", "task_status": "PendingAuditA",
            "current_handler_id": 7, "priority": "normal", "owner_department": "D", "owner_team": "T",
            "owner_org_team": "O", "detail_category": "C", "category_code": None, "product_short_name": None,
            "demand_text": None, "copy_text": None, "remark": None, "change_request": None,
            "design_requirement": None, "material": None, "spec_text": None, "size_text": None,
            "craft_text": None, "process": None, "reference_link": None, "creator_name": "Creator",
            "requester_name": "Requester", "designer_name": "Designer", "handler_name": "Handler",
            "created_date": "2026-01-01", "created_compact": "20260101", "deadline_date": None,
        }
        asset1 = {"id": 100, "task_id": 10, "asset_type": "delivery", "file_name": "final.psd",
                  "original_filename": "final.psd", "storage_key": "task/10/final.psd", "source_module_key": "design"}
        rows = [
            {"kind": "header", "value": {"schema_version": 1, "snapshot_sha256": self.snapshot,
                "algorithm_sha256": sha256_text(canonical_json(ALGORITHM_SPEC))}},
            {"kind": "task", "value": task}, {"kind": "asset", "value": asset1},
            {"kind": "client_pin", "value": {"id": 1, "asset_id": 999, "source_type": "external",
                "source_ref": "external:999", "enabled": 1, "task_id": None, "scope_kind": None,
                "scope_ref_id": None, "finalized_revision_no": None, "cover_sort_order": None}},
        ]
        if second_asset:
            rows.append({"kind": "asset", "value": {**asset1, "id": 101, "file_name": "other.png", "original_filename": "other.png"}})
        frozen = root / "frozen.jsonl"
        frozen.write_text("".join(canonical_json(row) + "\n" for row in rows), encoding="utf-8")
        source_id = 9999 if missing_source else 100
        mapping = {"version": 2, "resources": [{"task_id": 10, "scope_kind": "task", "scope_ref_id": 0,
            "history": [{"revision_no": 1, "status": "finalized", "mode": "single", "source_stage": "design",
                "source_task_asset_id": source_id, "final_task_asset_ids": [100], "reference_file_ref_ids": [],
                "confidence": "confirmed_auto", "confirmed_by": 9, "confirmed_at": "2026-01-02T00:00:00Z",
                "confirmation_note": "reviewed", "manifest_row_hash": "b" * 64}],
            "working_revision_no": 1, "finalized_revision_no": 1}], "planning_tasks": [], "task_state_decisions": []}
        mapping_path = root / "mapping.json"
        mapping_path.write_text(canonical_json(mapping), encoding="utf-8")
        return frozen, mapping_path

    def run_build(self, root: pathlib.Path, **kwargs):
        frozen, mapping = self.write_inputs(root, **kwargs)
        output = root / "expected.jsonl"
        build_expected(argparse.Namespace(mapping=str(mapping), frozen_a=str(frozen), snapshot_sha256=self.snapshot, output=str(output)))
        return [json.loads(line) for line in output.read_text(encoding="utf-8").splitlines()]

    def test_build_is_deterministic_and_maps_task_status(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            rows = self.run_build(root)
            first = (root / "expected.jsonl").read_bytes()
            build_expected(argparse.Namespace(mapping=str(root / "mapping.json"), frozen_a=str(root / "frozen.jsonl"),
                                                snapshot_sha256=self.snapshot, output=str(root / "expected2.jsonl")))
            self.assertEqual(first, (root / "expected2.jsonl").read_bytes())
            task = next(row for row in rows if row["entity_key"] == "task-search:10")
            self.assertEqual(task["review_state"], "pass")
            self.assertEqual(task["components"][2], "PendingAudit")
            self.assertEqual(task["components"][3], "7")
            self.assertEqual(len(task["components"][4]), 64)
            self.assertEqual(next(row for row in rows if row["entity_key"] == "group-search:10:task:0")["review_state"], "pass")
            self.assertEqual(next(row for row in rows if row["entity_key"] == "client-pin:1")["review_state"], "pass")

    def test_multi_asset_task_uses_stable_task_asset_order(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            rows = self.run_build(pathlib.Path(tmp), second_asset=True)
            task = next(row for row in rows if row["entity_key"] == "task-search:10")
            self.assertEqual(task["review_state"], "pass")
            self.assertEqual(task["detail"]["projection_kind"], "task_search")

    def test_reviewed_state_decision_precedes_generic_legacy_mapping(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            frozen, mapping_path = self.write_inputs(root)
            frozen_rows = [
                json.loads(line)
                for line in frozen.read_text(encoding="utf-8").splitlines()
            ]
            task = next(row["value"] for row in frozen_rows if row["kind"] == "task")
            task["task_status"] = "PendingWarehouseReceive"
            frozen.write_text(
                "".join(canonical_json(row) + "\n" for row in frozen_rows),
                encoding="utf-8",
            )
            mapping = json.loads(mapping_path.read_text(encoding="utf-8"))
            mapping["task_state_decisions"] = [{
                "task_id": 10,
                "from_status": "PendingWarehouseReceive",
                "target_status": "InProgress",
                "confirmed_by": 1,
                "confirmed_at": "2026-07-24T00:00:00Z",
                "confirmation_note": "reviewed exception",
                "manifest_row_hash": "c" * 64,
            }]
            mapping_path.write_text(canonical_json(mapping), encoding="utf-8")
            output = root / "expected.jsonl"
            build_expected(argparse.Namespace(
                mapping=str(mapping_path),
                frozen_a=str(frozen),
                snapshot_sha256=self.snapshot,
                output=str(output),
            ))
            task_projection = next(
                json.loads(line)
                for line in output.read_text(encoding="utf-8").splitlines()
                if '"task-search:10"' in line
            )
            self.assertEqual(task_projection["components"][2], "InProgress")

    def test_missing_group_projection_input_is_entity_blocker(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            rows = self.run_build(pathlib.Path(tmp), missing_source=True)
            group = next(row for row in rows if row["entity_key"] == "group-search:10:task:0")
            self.assertEqual(group["review_state"], "hard_blocked")
            self.assertIn("lacks a frozen filename", group["detail"]["blockers"][0])

    def test_historical_superseded_publication_pin_is_valid_but_draft_is_blocked(self) -> None:
        for historical_status, expected_state in (("superseded", "pass"), ("draft", "hard_blocked")):
            with self.subTest(status=historical_status), tempfile.TemporaryDirectory() as tmp:
                root = pathlib.Path(tmp)
                frozen, mapping_path = self.write_inputs(root)
                mapping = json.loads(mapping_path.read_text(encoding="utf-8"))
                current = mapping["resources"][0]["history"][0]
                historical = {**current, "revision_no": 1, "status": historical_status}
                current = {**current, "revision_no": 2}
                mapping["resources"][0].update({"history": [historical, current], "working_revision_no": 2, "finalized_revision_no": 2})
                mapping_path.write_text(canonical_json(mapping), encoding="utf-8")
                rows = [json.loads(line) for line in frozen.read_text(encoding="utf-8").splitlines()]
                rows[-1] = {"kind": "client_pin", "value": {
                    "id": 2, "asset_id": None, "source_type": "task_resource_group", "source_ref": "group:10",
                    "enabled": 1, "task_id": 10, "scope_kind": "task", "scope_ref_id": 0,
                    "finalized_revision_no": 1, "cover_sort_order": 0,
                }}
                frozen.write_text("".join(canonical_json(row) + "\n" for row in rows), encoding="utf-8")
                output = root / "expected.jsonl"
                build_expected(argparse.Namespace(mapping=str(mapping_path), frozen_a=str(frozen), snapshot_sha256=self.snapshot, output=str(output)))
                pin = next(json.loads(line) for line in output.read_text(encoding="utf-8").splitlines() if '"client-pin:2"' in line)
                self.assertEqual(pin["review_state"], expected_state)

    def test_mysql_concat_ws_skips_null_but_keeps_empty(self) -> None:
        self.assertEqual(mysql_concat_ws(["a", None, "", 2]), "a  2")

    def test_recovery_plan_activates_only_hash_bound_deleted_asset(self) -> None:
        mapping = {
            "asset_recoveries": [
                {
                    "task_id": 10,
                    "missing_task_asset_id": 200,
                    "strategy": "verified_oss_recovery_v1",
                    "recovery_source_sha256": "1" * 64,
                    "expected_file_size": 4,
                }
            ]
        }
        mapping_sha = "2" * 64
        assets = {
            200: {
                "id": 200,
                "task_id": 10,
                "storage_key": None,
                "deleted_at": "2026-01-01",
                "cleaned_at": None,
            }
        }
        plan = {
            "version": 1,
            "status": "MATERIALIZED",
            "mapping_sha256": mapping_sha,
            "production_writes_executed": False,
            "database_writes_executed": False,
            "run_id": "test-run",
            "entries": [
                {
                    "missing_task_asset_id": 200,
                    "source_sha256": "1" * 64,
                    "source_size": 4,
                    "target_object_key": "recovered/200.bin",
                    "db_apply_plan": {
                        "update_task_asset": {
                            "where": {"id": 200},
                            "set": {
                                "storage_key": "recovered/200.bin",
                                "whole_hash": "1" * 64,
                                "deleted_at": None,
                                "cleaned_at": None,
                            },
                        }
                    },
                }
            ],
        }
        with tempfile.TemporaryDirectory() as raw:
            path = pathlib.Path(raw) / "plan.json"
            path.write_text(canonical_json(plan), encoding="utf-8")
            provenance = apply_recovery_plan(
                mapping, mapping_sha, assets, str(path)
            )
        self.assertRegex(provenance["recovery_plan_sha256"], r"^[0-9a-f]{64}$")
        self.assertIsNone(assets[200]["deleted_at"])
        self.assertEqual("recovered/200.bin", assets[200]["storage_key"])

    def test_bundle_projection_uses_only_validated_registry_candidate(self) -> None:
        key = (10, "task", 0, 1)
        normalized = {
            "task_asset_id": 300,
            "format": "zip",
            "bundle_sha256": "3" * 64,
            "manifest_sha256": "4" * 64,
            "members": [],
            "confirmed_by": 1,
            "confirmed_at": "2026-01-01T00:00:00Z",
            "confirmation_note": "reviewed",
        }
        mapping = {
            "resources": [
                {
                    "task_id": 10,
                    "scope_kind": "task",
                    "scope_ref_id": 0,
                    "history": [
                        {"revision_no": 1, "source_bundle": normalized}
                    ],
                }
            ]
        }
        manifest = {"status": "CONFIRMED"}
        registry = {
            "entries": [
                {
                    "task_asset_candidate": {
                        "id": 300,
                        "task_id": 10,
                        "storage_key": "fixture/run/source-bundle.zip",
                    }
                }
            ]
        }
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            manifest_path = root / "manifest.json"
            registry_path = root / "registry.json"
            bundle_mapping_path = root / "bundle-mapping.json"
            manifest_path.write_text(canonical_json(manifest), encoding="utf-8")
            registry_path.write_text(canonical_json(registry), encoding="utf-8")
            bundle_mapping_path.write_text(
                canonical_json({"version": 2, "resources": []}),
                encoding="utf-8",
            )
            with mock.patch(
                "projection_expected.bundle_registry.validate_manifest",
                return_value=({key: {}}, "run"),
            ), mock.patch(
                "projection_expected.bundle_registry.validate_registry",
                return_value={key: normalized},
            ):
                rows, provenance = materialized_bundle_assets(
                    mapping,
                    str(bundle_mapping_path),
                    str(manifest_path),
                    str(registry_path),
                )
        self.assertEqual("source-bundle.zip", rows[0]["original_filename"])
        self.assertEqual("migration", rows[0]["source_module_key"])
        self.assertRegex(
            provenance["source_bundle_registry_sha256"], r"^[0-9a-f]{64}$"
        )

    def test_unreviewed_mapping_is_rejected(self) -> None:
        with self.assertRaisesRegex(ValueError, "confirmed_auto"):
            require_reviewed_mapping({"version": 2, "resources": [{"history": [{"confidence": "proposed_review"}]}]})

    def test_sql_gates_cover_pin_and_outbox_failure_identity(self) -> None:
        sql_root = pathlib.Path(__file__).with_name("sql")
        gate09 = (sql_root / "09_search_publish_outbox.sql").read_text(encoding="utf-8")
        manifest = (sql_root / "11_manifest_state.sql").read_text(encoding="utf-8")
        for fragment in ("client_pin_invalid_shape", "erp_outbox_permanent_failure",
                         "erp_outbox_duplicate_dedupe_key", "erp_outbox_duplicate_business_key",
                         "reindex_outbox_permanent_failure", "reindex_outbox_duplicate_dedupe_key"):
            self.assertIn(fragment, gate09)
        self.assertIn("CONCAT('client-pin:', c.id)", manifest)


if __name__ == "__main__":
    unittest.main()
