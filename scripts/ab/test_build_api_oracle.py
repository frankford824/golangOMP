from __future__ import annotations

import contextlib
import copy
import io
import json
import pathlib
import tempfile
import unittest

from scripts.ab import build_api_oracle as oracle
from scripts.ab import build_source_alias_receipts as alias_receipts
from scripts.ab import export_frozen_a_oracle as frozen_export
from scripts.ab.test_export_frozen_a_oracle import _fixture_output


class BuildAPIOracleV3Test(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = pathlib.Path(self.temp.name)
        self.run_id = "run-1"
        self.manifest = self.root / "reviewed-manifest.jsonl"
        self.mapping = self.root / "mapping.json"
        self.snapshot_dir = self.root / "frozen-a"
        self.snapshot_manifest = self.snapshot_dir / "manifest.json"
        self.snapshot_verdict = self.root / "snapshot-verdict.json"
        self.clone_a_attestation = self.root / "clone-a-attestation.json"
        self.alias_allocation_receipt = self.root / "alias-allocation.json"
        self.alias_apply_receipt = self.root / "alias-apply.json"
        self.source_snapshot_sha256 = "7" * 64
        self.mapping_document = {
            "organization_mappings": [
                {
                    "subject_type": "task",
                    "subject_id": 1,
                    "confidence": "confirmed_auto",
                    "confirmed_by": 1,
                    "target_department_id": None,
                    "target_team_id": None,
                }
            ],
            "asset_recoveries": [],
            "resources": [
                {
                    "task_id": 1,
                    "scope_kind": "task",
                    "scope_ref_id": 0,
                    "working_revision_no": 1,
                    "finalized_revision_no": 1,
                    "history": [
                        {
                            "revision_no": 1,
                            "status": "finalized",
                            "source_stage": "design",
                            "created_by": 1,
                            "created_at": "2026-01-01T00:00:00Z",
                            "submitted_at": None,
                            "finalized_at": "2026-01-02T00:00:00Z",
                            "source_alias_from_task_asset_id": 1,
                            "final_task_asset_ids": [1],
                            "reference_file_ref_ids": [],
                            "reason": "reviewed reason",
                        }
                    ],
                }
            ],
        }
        self.metadata, self.schemas, self.snapshot_rows = (
            frozen_export.parse_mysql_output(_fixture_output())
        )
        self.prepare_snapshot_rows()
        self.write_mapping()
        self.write_clone_a_attestation()
        self.write_snapshot_verdict()
        self.write_manifest()
        self.write_snapshot_package()

    def tearDown(self) -> None:
        self.temp.cleanup()

    def row(self, dataset: str) -> dict:
        return self.snapshot_rows[dataset][0][1]

    def prepare_snapshot_rows(self) -> None:
        task = self.row("tasks")
        task.update(
            {
                "id": 1,
                "task_type": "new_product_development",
                "task_status": "Completed",
                "current_handler_id": None,
                "workflow_revision": "0",
                "owner_department_id": "6",
                "owner_team_id": None,
            }
        )
        root = self.row("roots")
        root.update(
            {
                "id": 1,
                "task_id": "1",
                "asset_type": "delivery",
                "scope_sku_code": None,
                "retouch_requirement_id": None,
                "current_version_id": "1",
            }
        )
        asset = self.row("task_assets")
        asset.update(
            {
                "id": 1,
                "task_id": "1",
                "asset_id": "1",
                "scope_sku_code": None,
                "retouch_requirement_id": None,
                "asset_type": "delivery",
                "binding_state": "legacy",
                "bound_role": None,
                "storage_ref_id": "ref-001",
                "file_size": "10",
                "storage_key": "objects/task-1.png",
                "whole_hash": "a" * 64,
                "upload_status": "uploaded",
                "mime_type": "image/png",
                "deleted_at": None,
                "cleaned_at": None,
                "object_deleted_at": None,
                "asset_version_no": "1",
                "flow_review_status": None,
                "approved_at": None,
                "approved_by": None,
                "created_at": "2026-07-25T01:02:03.000000Z",
                "source_asset_version_id": None,
            }
        )
        storage = self.row("objects")
        storage.update(
            {
                "ref_id": "ref-001",
                "asset_id": "1",
                "owner_type": "task_asset",
                "owner_id": "1",
                "ref_key": "objects/task-1.png",
                "file_size": "10",
                "mime_type": "image/png",
                "is_placeholder": "0",
                "checksum_hint": "a" * 64,
                "status": "recorded",
            }
        )
        sku = self.row("skus")
        sku.update({"id": 1, "task_id": "1", "sku_code": "SKU-1"})
        requirement = self.row("retouch_requirements")
        requirement.update(
            {
                "id": 1,
                "task_id": "1",
                "sku_code": "SKU-1",
                "deleted_at": None,
            }
        )
        reference = self.row("reference_file_refs")
        reference.update(
            {
                "id": 1,
                "task_id": "1",
                "sku_item_id": None,
                "retouch_requirement_id": None,
                "ref_id": "ref-001",
            }
        )

    @property
    def mapping_sha256(self) -> str:
        return oracle.sha256(self.mapping.read_bytes())

    @property
    def clone_a_attestation_sha256(self) -> str:
        return oracle.sha256(self.clone_a_attestation.read_bytes())

    @property
    def snapshot_verdict_sha256(self) -> str:
        return oracle.sha256(self.snapshot_verdict.read_bytes())

    def write_mapping(self) -> None:
        self.mapping.write_bytes(oracle.canonical(self.mapping_document))

    def write_clone_a_attestation(
        self, *, source_snapshot_sha256: str | None = None
    ) -> None:
        snapshot_hash = source_snapshot_sha256 or self.source_snapshot_sha256
        self.clone_a_attestation.write_bytes(
            oracle.canonical(
                {
                    "baseline_fingerprint_sha256": "6" * 64,
                    "clone_database": "clone_a",
                    "clone_label": "A",
                    "import_receipt_sha256": "4" * 64,
                    "run_id": self.run_id,
                    "schema_version": 1,
                    "snapshot_sha256": snapshot_hash,
                    "source_coordinates": {
                        "binlog_file": "binlog.000001",
                        "binlog_position": 42,
                        "snapshot_sha256": snapshot_hash,
                    },
                }
            )
        )

    def write_snapshot_verdict(self) -> None:
        self.snapshot_verdict.write_bytes(
            oracle.canonical(
                {
                    "baseline_fingerprint_sha256": "6" * 64,
                    "evidence_sha256": "3" * 64,
                    "run_id": self.run_id,
                    "schema_version": 1,
                    "snapshot_sha256": self.source_snapshot_sha256,
                    "source_attestation_sha256": (
                        self.clone_a_attestation_sha256
                    ),
                    "status": "PASS",
                    "target_attestation_sha256": "5" * 64,
                    "violation_count": 0,
                    "violations": [],
                }
            )
        )

    def write_manifest(
        self,
        *,
        task_status: str = "Completed",
        source_ref: str = "ref-001",
        source_hash: str | None = None,
        scope_sku_code: str = "",
        retouch_requirement_id: str = "",
    ) -> None:
        common_hashes = {
            "mapping_sha256": self.mapping_sha256,
            "baseline_attestation_sha256": self.snapshot_verdict_sha256,
        }
        rows = [
            {
                "run_id": self.run_id,
                "gate_name": "G01",
                "entity_key": "task:1",
                "review_state": "pass",
                "detail_json": {
                    "components": [
                        "1",
                        "new_product_development",
                        task_status,
                        "",
                        "0",
                    ],
                    "input_sha256": common_hashes,
                },
            },
            {
                "run_id": self.run_id,
                "gate_name": "G03",
                "entity_key": "revision:1:task:0:1",
                "review_state": "pass",
                "detail_json": {
                    "components": ["1"],
                    "input_sha256": common_hashes,
                },
            },
            {
                "run_id": self.run_id,
                "gate_name": "G04",
                "entity_key": "revision-source:1:task:0:1",
                "review_state": "pass",
                "detail_json": {
                    "components": [
                        f"asset:1:{source_ref}",
                        "source",
                        source_hash or "a" * 64,
                        "bound",
                        "source",
                        scope_sku_code,
                        retouch_requirement_id,
                    ],
                    "input_sha256": common_hashes,
                },
            },
        ]
        self.manifest.write_text(
            "".join(json.dumps(row) + "\n" for row in rows),
            encoding="utf-8",
        )

    def write_snapshot_package(self) -> None:
        _, files = frozen_export.build_evidence(
            "clone_a", self.metadata, self.schemas, self.snapshot_rows
        )
        self.snapshot_dir.mkdir(exist_ok=True)
        for filename, content in files.items():
            (self.snapshot_dir / filename).write_bytes(content)

    def rewrite_manifest_revision_scope(
        self,
        scope_kind: str,
        scope_ref_id: int,
        source_components: list[str],
    ) -> None:
        rows = [
            json.loads(line)
            for line in self.manifest.read_text(encoding="utf-8").splitlines()
        ]
        for row in rows:
            if row["gate_name"] == "G03":
                row["entity_key"] = (
                    f"revision:1:{scope_kind}:{scope_ref_id}:1"
                )
            elif row["gate_name"] == "G04":
                row["entity_key"] = (
                    f"revision-source:1:{scope_kind}:{scope_ref_id}:1"
                )
                row["detail_json"]["components"] = source_components
        self.manifest.write_text(
            "".join(json.dumps(row) + "\n" for row in rows),
            encoding="utf-8",
        )

    def write_receipt(
        self, name: str, kind: str, entries: list[dict]
    ) -> pathlib.Path:
        path = self.root / name
        unsigned = {
            "schema_version": 2,
            "kind": kind,
            "status": "approved",
            "mapping_sha256": self.mapping_sha256,
            "reviewed_manifest_sha256": oracle.sha256(
                self.manifest.read_bytes()
            ),
            "source_evidence_sha256": ["8" * 64],
            "entries": entries,
        }
        path.write_bytes(
            oracle.canonical(
                {
                    **unsigned,
                    "evidence_sha256": oracle.sha256(
                        oracle.canonical(unsigned)
                    ),
                }
            )
        )
        return path

    def write_source_alias_receipts(
        self, recovery_receipts: tuple[pathlib.Path, ...]
    ) -> tuple[pathlib.Path | None, pathlib.Path | None]:
        entries = alias_receipts.allocation_entries(
            self.mapping_document, first_alias_id=101
        )
        if not entries:
            return None, None
        allocation_unsigned = {
            "schema_version": 1,
            "kind": "source_alias_allocation_v1",
            "status": "planned",
            "run_id": self.run_id,
            "database": "clone_b",
            "mapping_file_sha256": self.mapping_sha256,
            "mapping_canonical_sha256": "b" * 64,
            "workflow_snapshot_file_sha256": "c" * 64,
            "workflow_snapshot_integrity_sha256": "d" * 64,
            "task_assets_auto_increment_before": 101,
            "entry_count": len(entries),
            "entries": entries,
        }
        allocation = {
            **allocation_unsigned,
            "evidence_sha256": oracle.sha256(
                oracle.canonical(allocation_unsigned)
            ),
        }
        self.alias_allocation_receipt.write_bytes(
            oracle.canonical(allocation) + b"\n"
        )
        asset = self.row("task_assets")
        storage = self.row("objects")
        identity = {
            "root_asset_id": int(asset["asset_id"]),
            "storage_ref_id": str(asset["storage_ref_id"] or ""),
            "object_key_sha256": oracle.sha256(
                str(
                    asset["storage_key"]
                    or (
                        storage["ref_key"]
                        if storage["ref_id"] == asset["storage_ref_id"]
                        else ""
                    )
                ).encode()
            )
            if asset["storage_key"]
            or (
                storage["ref_id"] == asset["storage_ref_id"]
                and storage["ref_key"]
            )
            else "",
            "content_sha256": str(asset["whole_hash"] or ""),
            "file_size": int(asset["file_size"]),
            "mime_type": str(asset["mime_type"]),
        }
        for recovery_path in recovery_receipts:
            document = json.loads(recovery_path.read_bytes())
            for recovery in document.get("entries", []):
                if recovery.get("missing_task_asset_id") == int(asset["id"]):
                    identity = {
                        "root_asset_id": int(
                            recovery["target_root_asset_id"]
                        ),
                        "storage_ref_id": recovery["target_storage_ref_id"],
                        "object_key_sha256": recovery[
                            "target_object_key_sha256"
                        ],
                        "content_sha256": recovery[
                            "target_content_sha256"
                        ],
                        "file_size": int(recovery["target_size"]),
                        "mime_type": recovery["target_mime"],
                    }
        applied_entries = []
        for entry in entries:
            scope_kind = entry["scope_kind"]
            scope_ref_id = entry["scope_ref_id"]
            scope_sku_code = ""
            retouch_requirement_id = None
            if scope_kind == "sku":
                scope_sku_code = str(self.row("skus")["sku_code"])
            elif scope_kind == "retouch_requirement":
                scope_sku_code = str(
                    self.row("retouch_requirements")["sku_code"] or ""
                )
                retouch_requirement_id = scope_ref_id
            alias_id = entry["expected_alias_task_asset_id"]
            group_id = 200 + entry["sequence"]
            applied_entries.append(
                {
                    **entry,
                    "alias_task_asset_id": alias_id,
                    "group_id": group_id,
                    **identity,
                    "scope_sku_code": scope_sku_code,
                    "retouch_requirement_id": retouch_requirement_id,
                    "asset_type": "source",
                    "binding_state": "bound",
                    "bound_role": "source",
                    "flow_review_status": "not_applicable",
                    "source_module_key": "migration",
                    "remark": (
                        f"v8-source-alias:group={group_id}:"
                        f"origin={entry['origin_task_asset_id']}"
                    ),
                }
            )
        apply_unsigned = {
            "schema_version": 1,
            "kind": "source_alias_apply_v1",
            "status": "verified",
            "run_id": self.run_id,
            "database": "clone_b",
            "mapping_file_sha256": self.mapping_sha256,
            "mapping_canonical_sha256": "b" * 64,
            "workflow_snapshot_file_sha256": "c" * 64,
            "workflow_snapshot_integrity_sha256": "d" * 64,
            "allocation_receipt_sha256": oracle.sha256(
                self.alias_allocation_receipt.read_bytes()
            ),
            "actual_aliases_tsv_sha256": "e" * 64,
            "entry_count": len(applied_entries),
            "entries": applied_entries,
        }
        apply = {
            **apply_unsigned,
            "evidence_sha256": oracle.sha256(oracle.canonical(apply_unsigned)),
        }
        self.alias_apply_receipt.write_bytes(oracle.canonical(apply) + b"\n")
        return self.alias_allocation_receipt, self.alias_apply_receipt

    def build(
        self,
        *,
        expected_verdict_sha256: str | None = None,
        expected_attestation_sha256: str | None = None,
        bundle_receipts: tuple[pathlib.Path, ...] = (),
        recovery_receipts: tuple[pathlib.Path, ...] = (),
    ) -> dict:
        allocation_receipt, apply_receipt = self.write_source_alias_receipts(
            recovery_receipts
        )
        return oracle.build(
            run_id=self.run_id,
            manifest_path=self.manifest,
            reviewed_mapping_path=self.mapping,
            expected_mapping_sha256=self.mapping_sha256,
            snapshot_verdict_path=self.snapshot_verdict,
            expected_snapshot_verdict_sha256=(
                expected_verdict_sha256 or self.snapshot_verdict_sha256
            ),
            clone_a_attestation_path=self.clone_a_attestation,
            expected_clone_a_attestation_sha256=(
                expected_attestation_sha256
                or self.clone_a_attestation_sha256
            ),
            a_snapshot_manifest_path=self.snapshot_manifest,
            bundle_receipt_paths=bundle_receipts,
            recovery_receipt_paths=recovery_receipts,
            source_alias_allocation_receipt_path=allocation_receipt,
            expected_source_alias_allocation_receipt_sha256=(
                oracle.sha256(allocation_receipt.read_bytes())
                if allocation_receipt is not None
                else ""
            ),
            source_alias_apply_receipt_path=apply_receipt,
            expected_source_alias_apply_receipt_sha256=(
                oracle.sha256(apply_receipt.read_bytes())
                if apply_receipt is not None
                else ""
            ),
        )

    def test_alias_preserves_delivery_and_creates_independent_source(self) -> None:
        result = self.build()
        self.assertEqual("non_circular_g6_v3", result["oracle_kind"])
        delivery = next(
            row
            for row in result["versions"]
            if row["task_asset_id"] == 1
        )
        alias = next(
            row
            for row in result["versions"]
            if row["provenance"]["kind"] == "source_alias_apply_receipt"
        )
        self.assertEqual("delivery", delivery["intrinsic_asset_type"])
        self.assertEqual(["final"], delivery["expected_roles"])
        self.assertEqual("source", alias["intrinsic_asset_type"])
        self.assertEqual(["source"], alias["expected_roles"])
        self.assertEqual("bound", alias["binding_state"])
        self.assertEqual("source", alias["bound_role"])
        self.assertEqual("not_applicable", alias["flow_review_status"])
        self.assertNotEqual(delivery["stable_locator"], alias["stable_locator"])
        self.assertEqual(
            "delivery_source_alias",
            result["revision_roles"][0]["source_kind"],
        )
        self.assertEqual(
            alias["stable_locator"],
            result["revision_roles"][0]["source_locator"],
        )
        self.assertEqual("approved", delivery["flow_review_status"])
        self.assertEqual(
            "2026-01-02T00:00:00Z", delivery["approved_at"]
        )
        self.assertEqual(1, delivery["approved_by"])
        self.assertEqual(
            result["inputs"]["snapshot_verdict_sha256"],
            self.snapshot_verdict_sha256,
        )
        self.assertEqual(
            result["inputs"]["clone_a_attestation_sha256"],
            self.clone_a_attestation_sha256,
        )
        self.assertNotEqual(
            self.snapshot_verdict_sha256, self.clone_a_attestation_sha256
        )

    def test_source_alias_receipts_reject_stale_and_tampered_evidence(
        self,
    ) -> None:
        allocation_path, apply_path = self.write_source_alias_receipts(())
        assert allocation_path is not None
        assert apply_path is not None
        allocation_hash = oracle.sha256(allocation_path.read_bytes())
        apply_hash = oracle.sha256(apply_path.read_bytes())
        oracle.load_source_alias_receipts(
            allocation_path=allocation_path,
            expected_allocation_sha256=allocation_hash,
            apply_path=apply_path,
            expected_apply_sha256=apply_hash,
            run_id=self.run_id,
            mapping_sha256=self.mapping_sha256,
        )
        allocation = json.loads(allocation_path.read_bytes())
        allocation["entries"][0]["task_id"] = 2
        allocation_path.write_bytes(oracle.canonical(allocation) + b"\n")
        with self.assertRaisesRegex(ValueError, "file hash differs"):
            oracle.load_source_alias_receipts(
                allocation_path=allocation_path,
                expected_allocation_sha256=allocation_hash,
                apply_path=apply_path,
                expected_apply_sha256=apply_hash,
                run_id=self.run_id,
                mapping_sha256=self.mapping_sha256,
            )
        with self.assertRaisesRegex(ValueError, "evidence hash mismatch"):
            oracle.load_source_alias_receipts(
                allocation_path=allocation_path,
                expected_allocation_sha256=oracle.sha256(
                    allocation_path.read_bytes()
                ),
                apply_path=apply_path,
                expected_apply_sha256=apply_hash,
                run_id=self.run_id,
                mapping_sha256=self.mapping_sha256,
            )

    def test_complete_a_approval_metadata_is_preserved(self) -> None:
        asset = self.row("task_assets")
        asset["flow_review_status"] = "approved"
        asset["approved_at"] = "2025-12-01T00:00:00.000000Z"
        asset["approved_by"] = "9"
        self.write_snapshot_package()
        version = self.build()["versions"][0]
        self.assertEqual("approved", version["flow_review_status"])
        self.assertEqual(
            "2025-12-01T00:00:00.000000Z", version["approved_at"]
        )
        self.assertEqual(9, version["approved_by"])

    def test_formal_exporter_transaction_contract_is_mandatory(self) -> None:
        manifest = json.loads(self.snapshot_manifest.read_bytes())
        manifest["transaction"]["single_connection"] = False
        unsigned = {
            key: value
            for key, value in manifest.items()
            if key != "evidence_sha256"
        }
        manifest["evidence_sha256"] = oracle.sha256(
            frozen_export.canonical_json_bytes(unsigned)
        )
        self.snapshot_manifest.write_bytes(
            frozen_export.canonical_json_bytes(manifest) + b"\n"
        )
        with self.assertRaisesRegex(ValueError, "manifest evidence differs"):
            self.build()

    def test_embedded_exporter_schema_is_independently_rehashed(self) -> None:
        manifest = json.loads(self.snapshot_manifest.read_bytes())
        tasks = next(
            row for row in manifest["datasets"] if row["dataset"] == "tasks"
        )
        tasks["schema"][0]["type"] = "varchar(999)"
        unsigned = {
            key: value
            for key, value in manifest.items()
            if key != "evidence_sha256"
        }
        manifest["evidence_sha256"] = oracle.sha256(
            frozen_export.canonical_json_bytes(unsigned)
        )
        self.snapshot_manifest.write_bytes(
            frozen_export.canonical_json_bytes(manifest) + b"\n"
        )
        with self.assertRaisesRegex(ValueError, "schema hash mismatch"):
            self.build()

    def test_dataset_file_and_row_hashes_are_independently_validated(self) -> None:
        tasks_path = self.snapshot_dir / "tasks.ndjson"
        envelope = json.loads(tasks_path.read_bytes())
        envelope["row"]["task_status"] = "InProgress"
        tasks_path.write_bytes(oracle.canonical(envelope) + b"\n")
        with self.assertRaisesRegex(ValueError, "file hash mismatch"):
            self.build()

        self.write_snapshot_package()
        manifest = json.loads(self.snapshot_manifest.read_bytes())
        envelope = json.loads(tasks_path.read_bytes())
        envelope["row"]["task_status"] = "InProgress"
        tampered = oracle.canonical(envelope) + b"\n"
        tasks_path.write_bytes(tampered)
        tasks_manifest = next(
            row for row in manifest["datasets"] if row["dataset"] == "tasks"
        )
        tasks_manifest["file_sha256"] = oracle.sha256(tampered)
        unsigned = {
            key: value
            for key, value in manifest.items()
            if key != "evidence_sha256"
        }
        manifest["evidence_sha256"] = oracle.sha256(
            frozen_export.canonical_json_bytes(unsigned)
        )
        self.snapshot_manifest.write_bytes(
            frozen_export.canonical_json_bytes(manifest) + b"\n"
        )
        with self.assertRaisesRegex(ValueError, "row hash mismatch"):
            self.build()

    def test_legacy_divergent_storage_key_prefers_task_asset_key(self) -> None:
        asset = self.row("task_assets")
        storage = self.row("objects")
        asset["storage_key"] = "legacy/migrated_e7d.psd"
        storage["ref_key"] = "legacy/中文名.psd"
        self.write_snapshot_package()
        version = self.build()["versions"][0]
        self.assertEqual(
            digest := oracle.sha256(b"legacy/migrated_e7d.psd"),
            version["object_key_sha256"],
        )
        self.assertNotEqual(
            digest, oracle.sha256(b"legacy/\xe4\xb8\xad\xe6\x96\x87\xe5\x90\x8d.psd")
        )

    def test_empty_task_asset_storage_key_falls_back_to_ref_key(self) -> None:
        self.row("task_assets")["storage_key"] = ""
        self.row("objects")["ref_key"] = "legacy/object-ref.psd"
        self.write_snapshot_package()
        version = self.build()["versions"][0]
        self.assertEqual(
            oracle.sha256(b"legacy/object-ref.psd"),
            version["object_key_sha256"],
        )

    def test_divergent_key_does_not_weaken_object_owner_constraints(self) -> None:
        self.row("task_assets")["storage_key"] = "legacy/migrated.psd"
        self.row("objects")["ref_key"] = "legacy/original.psd"
        self.row("objects")["owner_id"] = "99"
        self.write_snapshot_package()
        with self.assertRaisesRegex(ValueError, "object projection differs"):
            self.build()

    def test_mapping_hash_is_mandatory(self) -> None:
        with self.assertRaisesRegex(ValueError, "reviewed mapping hash differs"):
            oracle.build(
                run_id=self.run_id,
                manifest_path=self.manifest,
                reviewed_mapping_path=self.mapping,
                expected_mapping_sha256="f" * 64,
                snapshot_verdict_path=self.snapshot_verdict,
                expected_snapshot_verdict_sha256=(
                    self.snapshot_verdict_sha256
                ),
                clone_a_attestation_path=self.clone_a_attestation,
                expected_clone_a_attestation_sha256=(
                    self.clone_a_attestation_sha256
                ),
                a_snapshot_manifest_path=self.snapshot_manifest,
            )

    def test_all_passed_g01_g05_rows_require_exact_input_provenance(
        self,
    ) -> None:
        original_rows = [
            json.loads(line)
            for line in self.manifest.read_text(encoding="utf-8").splitlines()
            if line.strip()
        ]
        common_hashes = {
            "mapping_sha256": self.mapping_sha256,
            "baseline_attestation_sha256": self.snapshot_verdict_sha256,
        }
        original_rows.extend(
            [
                {
                    "run_id": self.run_id,
                    "gate_name": "G02",
                    "entity_key": "group:1:task:0",
                    "review_state": "pass",
                    "detail_json": {
                        "components": ["ignored"],
                        "input_sha256": dict(common_hashes),
                    },
                },
                {
                    "run_id": self.run_id,
                    "gate_name": "G05",
                    "entity_key": "revision-reference:1:task:0:1:1",
                    "review_state": "pass",
                    "detail_json": {
                        "components": ["ignored"],
                        "input_sha256": dict(common_hashes),
                    },
                },
            ]
        )
        for gate in ("G01", "G02", "G03", "G04", "G05"):
            for field, error in (
                ("mapping_sha256", "mapping hash differs"),
                (
                    "baseline_attestation_sha256",
                    "snapshot verdict binding differs",
                ),
            ):
                with self.subTest(gate=gate, field=field):
                    rows = copy.deepcopy(original_rows)
                    target = next(
                        row for row in rows if row["gate_name"] == gate
                    )
                    target["detail_json"]["input_sha256"].pop(field)
                    self.manifest.write_text(
                        "".join(json.dumps(row) + "\n" for row in rows),
                        encoding="utf-8",
                    )
                    with self.assertRaisesRegex(ValueError, error):
                        oracle.load_manifest(
                            self.manifest,
                            self.run_id,
                            self.mapping_sha256,
                            self.snapshot_verdict_sha256,
                        )

    def test_verdict_and_attestation_hashes_are_separate_hard_bindings(
        self,
    ) -> None:
        with self.assertRaisesRegex(ValueError, "snapshot verdict file hash"):
            self.build(expected_verdict_sha256="f" * 64)
        with self.assertRaisesRegex(ValueError, "snapshot verdict evidence"):
            self.build(expected_attestation_sha256="e" * 64)

    def test_verdict_and_attestation_must_point_to_same_source_snapshot(
        self,
    ) -> None:
        self.write_clone_a_attestation(source_snapshot_sha256="2" * 64)
        self.write_snapshot_verdict()
        self.write_manifest()
        with self.assertRaisesRegex(ValueError, "attestation evidence differs"):
            self.build()

    def test_approved_pointer_follows_replayed_finalized_revision(self) -> None:
        result = self.build()
        self.assertEqual(
            result["versions"][0]["stable_locator"],
            result["route_expectations"]["approved_locators"][0],
        )
        self.row("tasks")["task_status"] = "InProgress"
        self.write_snapshot_package()
        self.write_manifest(task_status="InProgress")
        result = self.build()
        self.assertEqual(
            [result["versions"][0]["stable_locator"]],
            result["route_expectations"]["approved_locators"],
        )

    def test_retouch_scope_and_reference_file_refs_are_validated(self) -> None:
        resource = self.mapping_document["resources"][0]
        resource["scope_kind"] = "retouch_requirement"
        resource["scope_ref_id"] = 1
        resource["history"][0]["reference_file_ref_ids"] = [1]
        self.row("task_assets")["scope_sku_code"] = "SKU-1"
        self.row("task_assets")["retouch_requirement_id"] = "1"
        self.write_snapshot_package()
        self.write_mapping()
        self.write_manifest(
            scope_sku_code="SKU-1", retouch_requirement_id="1"
        )
        # Rewrite gate entity keys for the retouch revision.
        text = self.manifest.read_text(encoding="utf-8")
        text = text.replace(
            "revision:1:task:0:1", "revision:1:retouch_requirement:1:1"
        ).replace(
            "revision-source:1:task:0:1",
            "revision-source:1:retouch_requirement:1:1",
        )
        self.manifest.write_text(text, encoding="utf-8")
        result = self.build()
        role = result["revision_roles"][0]
        self.assertEqual([1], role["reference_file_ref_ids"])
        self.assertEqual(["reference:1:ref-001"], role["reference_locators"])

        self.row("reference_file_refs")["retouch_requirement_id"] = "2"
        self.write_snapshot_package()
        with self.assertRaisesRegex(ValueError, "conflicting scope"):
            self.build()

    def test_scoped_sku_draft_without_source_projects_all_empty(self) -> None:
        resource = self.mapping_document["resources"][0]
        resource["scope_kind"] = "sku"
        resource["scope_ref_id"] = 1
        resource["finalized_revision_no"] = None
        revision = resource["history"][0]
        revision.pop("source_alias_from_task_asset_id")
        revision["status"] = "draft"
        revision["final_task_asset_ids"] = []
        self.write_mapping()
        self.write_manifest()
        self.rewrite_manifest_revision_scope(
            "sku", 1, ["", "", "", "", "", "", ""]
        )
        role = self.build()["revision_roles"][0]
        self.assertIsNone(role["source_locator"])
        self.assertEqual("none", role["source_kind"])

    def test_scoped_retouch_draft_without_source_projects_all_empty(
        self,
    ) -> None:
        resource = self.mapping_document["resources"][0]
        resource["scope_kind"] = "retouch_requirement"
        resource["scope_ref_id"] = 1
        resource["finalized_revision_no"] = None
        revision = resource["history"][0]
        revision.pop("source_alias_from_task_asset_id")
        revision["status"] = "draft"
        revision["final_task_asset_ids"] = []
        self.write_mapping()
        self.write_manifest()
        self.rewrite_manifest_revision_scope(
            "retouch_requirement",
            1,
            ["", "", "", "", "", "", ""],
        )
        role = self.build()["revision_roles"][0]
        self.assertIsNone(role["source_locator"])
        self.assertEqual("none", role["source_kind"])

    def test_task_scoped_source_bound_to_sku_uses_group_scope(self) -> None:
        resource = self.mapping_document["resources"][0]
        resource["scope_kind"] = "sku"
        resource["scope_ref_id"] = 1
        self.write_mapping()
        self.write_manifest(scope_sku_code="SKU-1")
        self.rewrite_manifest_revision_scope(
            "sku",
            1,
            [
                "asset:1:ref-001",
                "source",
                "a" * 64,
                "bound",
                "source",
                "SKU-1",
                "",
            ],
        )
        role = self.build()["revision_roles"][0]
        self.assertIsNotNone(role["source_locator"])
        result = self.build()
        alias = next(
            row
            for row in result["versions"]
            if row["provenance"]["kind"] == "source_alias_apply_receipt"
        )
        final = next(
            row for row in result["versions"] if row["task_asset_id"] == 1
        )
        self.assertEqual("SKU-1", alias["scope_sku_code"])
        self.assertEqual("source", alias["bound_role"])
        self.assertEqual("SKU-1", final["scope_sku_code"])
        self.assertEqual("final", final["bound_role"])
        rows = [
            json.loads(line)
            for line in self.manifest.read_text(encoding="utf-8").splitlines()
        ]
        for row in rows:
            if row["gate_name"] == "G04":
                row["detail_json"]["components"][5] = "WRONG-SKU"
        self.manifest.write_text(
            "".join(json.dumps(row) + "\n" for row in rows),
            encoding="utf-8",
        )
        with self.assertRaisesRegex(ValueError, "source differs"):
            self.build()

    def test_bundle_requires_exact_cross_bound_receipt(self) -> None:
        history = self.mapping_document["resources"][0]["history"][0]
        history.pop("source_alias_from_task_asset_id")
        history["source_bundle"] = {
            "format": "zip",
            "bundle_sha256": "c" * 64,
            "manifest_sha256": "d" * 64,
            "task_asset_id": 30,
            "members": [
                {"confirmed": True, "task_asset_id": 1, "sha256": "a" * 64}
            ],
        }
        self.write_mapping()
        self.write_manifest(
            source_ref="ignored", source_hash="c" * 64
        )
        rows = [
            json.loads(line)
            for line in self.manifest.read_text(encoding="utf-8").splitlines()
        ]
        rows[2]["detail_json"]["components"][0] = "bundle:" + "c" * 64
        self.manifest.write_text(
            "".join(json.dumps(row) + "\n" for row in rows),
            encoding="utf-8",
        )
        entry = {
            "task_id": 1,
            "scope_kind": "task",
            "scope_ref_id": 0,
            "revision_no": 1,
            "bundle_task_asset_id": 30,
            "bundle_root_asset_id": 200,
            "bundle_storage_ref_id": "bundle-ref",
            "object_key_sha256": "e" * 64,
            "bundle_sha256": "c" * 64,
            "internal_manifest_sha256": "d" * 64,
            "size": 20,
            "mime_type": "application/zip",
            "members": [
                {
                    "task_asset_id": 1,
                    "storage_ref_id": "ref-001",
                    "size": 10,
                    "mime_type": "image/png",
                    "sha256": "a" * 64,
                }
            ],
        }
        receipt = self.write_receipt(
            "bundle.json", "bundle_materialization_v2", [entry]
        )
        result = self.build(bundle_receipts=(receipt,))
        bundle = next(
            version
            for version in result["versions"]
            if version["provenance"]["kind"] == "bundle_receipt"
        )
        self.assertEqual(30, bundle["task_asset_id"])

        self.row("task_assets")["whole_hash"] = None
        self.row("objects")["checksum_hint"] = ""
        self.write_snapshot_package()
        fallback_result = self.build(bundle_receipts=(receipt,))
        fallback_bundle = next(
            version
            for version in fallback_result["versions"]
            if version["provenance"]["kind"] == "bundle_receipt"
        )
        self.assertEqual(30, fallback_bundle["task_asset_id"])
        self.assertEqual(
            "not_applicable", fallback_bundle["flow_review_status"]
        )
        member = next(
            row
            for row in fallback_result["versions"]
            if row["task_asset_id"] == 1
        )
        self.assertEqual("a" * 64, member["content_sha256"])

        for field, value in {
            "task_asset_id": 2,
            "storage_ref_id": "wrong-ref",
            "size": 11,
            "mime_type": "application/octet-stream",
            "sha256": "b" * 64,
        }.items():
            with self.subTest(bundle_member_field=field):
                wrong_entry = copy.deepcopy(entry)
                wrong_entry["members"][0][field] = value
                wrong_receipt = self.write_receipt(
                    f"wrong-bundle-member-{field}.json",
                    "bundle_materialization_v2",
                    [wrong_entry],
                )
                with self.assertRaisesRegex(
                    ValueError, "bundle (receipt|member)"
                ):
                    self.build(bundle_receipts=(wrong_receipt,))

        self.row("task_assets")["whole_hash"] = "b" * 64
        self.row("objects")["checksum_hint"] = "b" * 64
        self.write_snapshot_package()
        with self.assertRaisesRegex(ValueError, "bundle member differs"):
            self.build(bundle_receipts=(receipt,))

        old = self.root / "old-bundle.json"
        old.write_text('{"schema_version":1,"status":"confirmed"}')
        with self.assertRaisesRegex(ValueError, "field contract differs"):
            self.build(bundle_receipts=(old,))

    def test_recovery_locator_is_fully_receipt_bound(self) -> None:
        self.mapping_document["asset_recoveries"] = [
            {
                "missing_task_asset_id": 1,
                "task_id": 1,
                "strategy": "verified_oss_recovery_v1",
                "recovery_source_task_asset_id": 1,
                "recovery_source_storage_ref_id": "ref-001",
                "recovery_source_sha256": "a" * 64,
                "expected_file_size": 10,
                "manifest_row_hash": "d" * 64,
            }
        ]
        self.write_mapping()
        self.write_manifest(source_ref="target-ref")
        entry = {
            "missing_task_asset_id": 1,
            "target_root_asset_id": 1,
            "target_task_id": 1,
            "target_storage_ref_id": "target-ref",
            "target_object_key_sha256": "e" * 64,
            "target_content_sha256": "a" * 64,
            "target_size": 10,
            "target_mime": "image/png",
            "source_task_asset_id": 1,
            "source_task_id": 1,
            "source_storage_ref_id": "ref-001",
            "source_content_sha256": "a" * 64,
            "source_size": 10,
            "source_mime": "image/png",
            "strategy": "verified_oss_recovery_v1",
            "source_receipt_sha256": "f" * 64,
        }
        receipt = self.write_receipt(
            "recovery.json", "recovery_materialization_v2", [entry]
        )
        result = self.build(recovery_receipts=(receipt,))
        recovered = next(
            row for row in result["versions"] if row["task_asset_id"] == 1
        )
        self.assertEqual("target-ref", recovered["storage_ref_id"])

        entry["source_storage_ref_id"] = "wrong-owner"
        wrong = self.write_receipt(
            "wrong-recovery.json", "recovery_materialization_v2", [entry]
        )
        with self.assertRaisesRegex(ValueError, "receipt differs"):
            self.build(recovery_receipts=(wrong,))

    def test_recovery_receipt_supplies_only_a_missing_content_hash(self) -> None:
        self.mapping_document["asset_recoveries"] = [
            {
                "missing_task_asset_id": 1,
                "task_id": 1,
                "strategy": "verified_oss_recovery_v1",
                "recovery_source_task_asset_id": 1,
                "recovery_source_storage_ref_id": "ref-001",
                "recovery_source_sha256": "a" * 64,
                "expected_file_size": 10,
                "manifest_row_hash": "d" * 64,
            }
        ]
        self.write_mapping()
        self.write_manifest(source_ref="target-ref")
        entry = {
            "missing_task_asset_id": 1,
            "target_root_asset_id": 1,
            "target_task_id": 1,
            "target_storage_ref_id": "target-ref",
            "target_object_key_sha256": "e" * 64,
            "target_content_sha256": "a" * 64,
            "target_size": 10,
            "target_mime": "image/png",
            "source_task_asset_id": 1,
            "source_task_id": 1,
            "source_storage_ref_id": "ref-001",
            "source_content_sha256": "a" * 64,
            "source_size": 10,
            "source_mime": "image/png",
            "strategy": "verified_oss_recovery_v1",
            "source_receipt_sha256": "f" * 64,
        }

        self.row("task_assets")["whole_hash"] = None
        self.row("objects")["checksum_hint"] = ""
        self.write_snapshot_package()
        receipt = self.write_receipt(
            "missing-a-hash-recovery.json",
            "recovery_materialization_v2",
            [entry],
        )
        result = self.build(recovery_receipts=(receipt,))
        recovered = next(
            row for row in result["versions"] if row["task_asset_id"] == 1
        )
        self.assertEqual("a" * 64, recovered["content_sha256"])
        self.assertEqual(
            "recovery_receipt",
            recovered["provenance"]["kind"],
        )

        self.row("task_assets")["whole_hash"] = "b" * 64
        self.row("objects")["checksum_hint"] = "b" * 64
        self.write_snapshot_package()
        with self.assertRaisesRegex(ValueError, "receipt differs"):
            self.build(recovery_receipts=(receipt,))

    def test_recovery_missing_a_hash_keeps_non_hash_cross_bindings(self) -> None:
        self.mapping_document["asset_recoveries"] = [
            {
                "missing_task_asset_id": 1,
                "task_id": 1,
                "strategy": "verified_oss_recovery_v1",
                "recovery_source_task_asset_id": 1,
                "recovery_source_storage_ref_id": "ref-001",
                "recovery_source_sha256": "a" * 64,
                "expected_file_size": 10,
                "manifest_row_hash": "d" * 64,
            }
        ]
        self.write_mapping()
        self.write_manifest(source_ref="target-ref")
        self.row("task_assets")["whole_hash"] = None
        self.row("objects")["checksum_hint"] = ""
        self.write_snapshot_package()
        base = {
            "missing_task_asset_id": 1,
            "target_root_asset_id": 1,
            "target_task_id": 1,
            "target_storage_ref_id": "target-ref",
            "target_object_key_sha256": "e" * 64,
            "target_content_sha256": "a" * 64,
            "target_size": 10,
            "target_mime": "image/png",
            "source_task_asset_id": 1,
            "source_task_id": 1,
            "source_storage_ref_id": "ref-001",
            "source_content_sha256": "a" * 64,
            "source_size": 10,
            "source_mime": "image/png",
            "strategy": "verified_oss_recovery_v1",
            "source_receipt_sha256": "f" * 64,
        }
        mutations = {
            "source_task_id": 2,
            "source_storage_ref_id": "wrong-ref",
            "source_size": 11,
            "source_mime": "image/jpeg",
            "target_content_sha256": "b" * 64,
        }
        for field, value in mutations.items():
            with self.subTest(field=field):
                entry = dict(base)
                entry[field] = value
                receipt = self.write_receipt(
                    f"wrong-{field}.json",
                    "recovery_materialization_v2",
                    [entry],
                )
                with self.assertRaisesRegex(ValueError, "receipt differs"):
                    self.build(recovery_receipts=(receipt,))

    def test_cli_has_no_legacy_snapshot_or_clone_b_arguments(self) -> None:
        with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(
            SystemExit
        ):
            oracle.main(["--a-snapshot-ndjson", "old.ndjson"])
        with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(
            SystemExit
        ):
            oracle.main(["--tasks-tsv", "b.tsv"])


if __name__ == "__main__":
    unittest.main()
