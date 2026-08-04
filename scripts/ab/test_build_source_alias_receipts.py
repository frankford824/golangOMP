from __future__ import annotations

import copy
import csv
import hashlib
import json
import pathlib
import tempfile
import unittest

from scripts.ab import build_source_alias_receipts as receipts


class SourceAliasReceiptTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = pathlib.Path(self.temp.name)
        self.mapping_path = self.root / "mapping.json"
        self.snapshot_path = self.root / "workflow-snapshot.json"
        self.tsv_path = self.root / "aliases.tsv"
        self.mapping = {
            "resources": [
                {
                    "task_id": 10,
                    "scope_kind": "sku",
                    "scope_ref_id": 20,
                    "history": [
                        {
                            "revision_no": 1,
                            "source_alias_from_task_asset_id": 30,
                        },
                        {
                            "revision_no": 2,
                            "source_alias_from_task_asset_id": 30,
                        },
                    ],
                }
            ]
        }
        self.snapshot = {
            "version": 8,
            "tool_version": "workflow-groups-migrate/v8.8",
            "schema_version": "124-126",
            "database": "clone_b",
            "mapping_sha256": "b" * 64,
            "integrity_sha256": "",
            "apply_state": "applied",
            "auto_increments_before": [
                {"table": "task_assets", "next_value": 100}
            ],
            "inserted_alias_asset_ids": [100],
        }
        self.write_inputs()

    def write_inputs(self) -> None:
        self.mapping_path.write_text(
            json.dumps(self.mapping, ensure_ascii=False), encoding="utf-8"
        )
        snapshot = copy.deepcopy(self.snapshot)
        snapshot["integrity_sha256"] = ""
        snapshot["integrity_sha256"] = hashlib.sha256(
            receipts.go_canonical(snapshot)
        ).hexdigest()
        self.snapshot_path.write_text(
            json.dumps(snapshot, ensure_ascii=False), encoding="utf-8"
        )
        row = {
            "alias_task_asset_id": "100",
            "task_id": "10",
            "scope_kind": "sku",
            "scope_ref_id": "20",
            "group_id": "40",
            "origin_task_asset_id": "30",
            "root_asset_id": "50",
            "storage_ref_id": "ref-1",
            "object_key_sha256": "c" * 64,
            "content_sha256": "d" * 64,
            "file_size": "60",
            "mime_type": "image/png",
            "scope_sku_code": "SKU-20",
            "retouch_requirement_id": "",
            "asset_type": "source",
            "binding_state": "bound",
            "bound_role": "source",
            "flow_review_status": "not_applicable",
            "source_module_key": "migration",
            "remark": "v8-source-alias:group=40:origin=30",
            "origin_root_asset_id": "50",
            "origin_storage_ref_id": "ref-1",
            "origin_object_key_sha256": "c" * 64,
            "origin_content_sha256": "d" * 64,
            "origin_file_size": "60",
            "origin_mime_type": "image/png",
        }
        with self.tsv_path.open("w", encoding="utf-8", newline="") as handle:
            writer = csv.DictWriter(
                handle, fieldnames=receipts.ACTUAL_FIELDS, delimiter="\t"
            )
            writer.writeheader()
            writer.writerow(row)

    def build(self) -> tuple[dict, dict]:
        return receipts.build_receipts(
            run_id="run-1",
            mapping_path=self.mapping_path,
            expected_mapping_sha256=receipts.file_sha256(self.mapping_path),
            expected_mapping_canonical_sha256="b" * 64,
            workflow_snapshot_path=self.snapshot_path,
            expected_workflow_snapshot_sha256=receipts.file_sha256(
                self.snapshot_path
            ),
            actual_aliases_tsv_path=self.tsv_path,
        )

    def mutate_tsv(self, field: str, value: str) -> None:
        with self.tsv_path.open(encoding="utf-8", newline="") as handle:
            rows = list(csv.DictReader(handle, delimiter="\t"))
        rows[0][field] = value
        with self.tsv_path.open("w", encoding="utf-8", newline="") as handle:
            writer = csv.DictWriter(
                handle, fieldnames=receipts.ACTUAL_FIELDS, delimiter="\t"
            )
            writer.writeheader()
            writer.writerows(rows)

    def test_allocation_is_prestate_derived_and_alias_is_reused(self) -> None:
        allocation, apply = self.build()
        self.assertEqual(1, allocation["entry_count"])
        self.assertEqual(100, allocation["entries"][0]["expected_alias_task_asset_id"])
        self.assertEqual(
            "alias:v1:10:sku:20:origin-task-asset:30",
            allocation["entries"][0]["canonical_locator"],
        )
        self.assertEqual(
            receipts.sha256(receipts.canonical(allocation) + b"\n"),
            apply["allocation_receipt_sha256"],
        )
        self.assertEqual("verified", apply["status"])

    def test_stale_mapping_and_snapshot_hashes_fail(self) -> None:
        with self.assertRaisesRegex(ValueError, "mapping file hash differs"):
            receipts.build_receipts(
                run_id="run-1",
                mapping_path=self.mapping_path,
                expected_mapping_sha256="e" * 64,
                expected_mapping_canonical_sha256="b" * 64,
                workflow_snapshot_path=self.snapshot_path,
                expected_workflow_snapshot_sha256=receipts.file_sha256(
                    self.snapshot_path
                ),
                actual_aliases_tsv_path=self.tsv_path,
            )
        self.snapshot["inserted_alias_asset_ids"] = [101]
        self.write_inputs()
        with self.assertRaisesRegex(ValueError, "inserted alias IDs differ"):
            self.build()

    def test_snapshot_integrity_tamper_fails(self) -> None:
        document = json.loads(self.snapshot_path.read_text(encoding="utf-8"))
        document["auto_increments_before"][0]["next_value"] = 101
        self.snapshot_path.write_text(json.dumps(document), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "integrity hash mismatch"):
            self.build()

    def test_post_apply_identity_and_role_tamper_fail(self) -> None:
        for field, value, error in (
            ("alias_task_asset_id", "101", "deterministic allocation"),
            ("bound_role", "final", "role or lineage"),
            ("origin_content_sha256", "e" * 64, "immutable origin identity"),
            ("scope_sku_code", "", "invalid asset scope"),
        ):
            with self.subTest(field=field):
                self.write_inputs()
                self.mutate_tsv(field, value)
                with self.assertRaisesRegex(ValueError, error):
                    self.build()

    def test_receipt_self_hash_changes_after_tamper(self) -> None:
        allocation, _ = self.build()
        original = allocation["evidence_sha256"]
        allocation["entries"][0]["task_id"] = 11
        unsigned = {
            key: value
            for key, value in allocation.items()
            if key != "evidence_sha256"
        }
        self.assertNotEqual(original, receipts.sha256(receipts.canonical(unsigned)))


if __name__ == "__main__":
    unittest.main()
