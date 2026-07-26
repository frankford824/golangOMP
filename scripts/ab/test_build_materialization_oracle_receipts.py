from __future__ import annotations

import copy
import json
import pathlib
import tempfile
import unittest

from scripts.ab import build_materialization_oracle_receipts as receipts
from scripts.ab import deterministic_source_bundle


class BuildMaterializationOracleReceiptsTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = pathlib.Path(self.temp.name)
        self.mapping_path = self.root / "mapping.json"
        self.manifest_path = self.root / "reviewed-manifest.jsonl"
        self.snapshot_dir = self.root / "frozen-a"
        self.snapshot_dir.mkdir()
        self.snapshot_manifest_path = self.snapshot_dir / "manifest.json"
        self.bundle_registry_path = self.root / "bundle-registry.json"
        self.recovery_plan_path = self.root / "recovery-plan.json"
        self.recovery_evidence_path = self.root / "recovery-evidence.json"
        self.bundle_ownership_paths: list[pathlib.Path] = []
        self.recovery_ownership_paths: list[pathlib.Path] = []
        self.run_id = "materialization-run-1"
        self.controlled_digest = "c" * 64
        self.content_hashes = {
            101: "1" * 64,
            102: "2" * 64,
            103: "3" * 64,
        }
        self.sources = {101: 201, 102: 202, 103: 203}
        self.sizes = {101: 1001, 102: 1002, 103: 1003}
        self.storage_refs = {
            201: "source-ref-201",
            202: "source-ref-202",
            203: "source-ref-203",
        }
        self.bundle_member_rows: dict[int, dict] = {}
        self.bundle_payloads: dict[int, bytes] = {}
        self.mapping = self.make_mapping()
        self.write_json(self.mapping_path, self.mapping)
        self.mapping_sha256 = receipts.sha256(self.mapping_path.read_bytes())
        self.write_reviewed_manifest()
        self.write_snapshot()
        self.write_bundle_evidence()
        self.write_recovery_evidence()

    def tearDown(self) -> None:
        self.temp.cleanup()

    @staticmethod
    def signed(document: dict, *, newline: bool = False) -> dict:
        result = copy.deepcopy(document)
        payload = receipts.canonical(result) + (b"\n" if newline else b"")
        result["evidence_sha256"] = receipts.sha256(payload)
        return result

    @staticmethod
    def write_json(path: pathlib.Path, document: dict) -> None:
        path.write_bytes(receipts.canonical(document))

    def rewrite_signed(self, path: pathlib.Path, mutate) -> dict:
        document = json.loads(path.read_text())
        document.pop("evidence_sha256", None)
        mutate(document)
        signed = self.signed(
            document,
            newline=path.name.startswith("bundle-ownership-")
            or path.name == "bundle-registry.json",
        )
        self.write_json(path, signed)
        return signed

    def make_mapping(self) -> dict:
        resources = []
        for offset, (task_id, scope_ref_id) in enumerate(((10, 100), (11, 101))):
            bundle_id = 500 + offset
            plan_members = []
            for member_offset in range(2):
                member_id = 300 + offset * 2 + member_offset
                content = f"bundle-{offset}-member-{member_offset}".encode()
                local_path = self.root / f"member-{member_id}.bin"
                local_path.write_bytes(content)
                storage_ref = f"bundle-member-ref-{member_id}"
                asset_id = 9000 + member_id
                plan_members.append(
                    {
                        "task_asset_id": member_id,
                        "asset_id": asset_id,
                        "storage_ref_id": storage_ref,
                        "original_file_name": f"member-{member_id}.bin",
                        "local_path": str(local_path),
                        "sha256": receipts.sha256(content),
                        "source_stage": "design",
                        "evidence_event_ids": [f"task_event_log:event-{member_id}"],
                        "confirmed": True,
                    }
                )
                self.bundle_member_rows[member_id] = {
                    "id": member_id,
                    "task_id": task_id,
                    "asset_id": asset_id,
                    "storage_ref_id": storage_ref,
                    "file_size": len(content),
                    "mime_type": "application/octet-stream",
                    "whole_hash": None,
                    "deleted_at": None,
                    "object_deleted_at": None,
                }
            plan = {
                "version": 1,
                "bundle_task_asset_id": bundle_id,
                "confirmed_by": 1,
                "confirmed_at": "2026-07-25T00:00:00Z",
                "confirmation_note": "approved frozen order",
                "members": plan_members,
            }
            plan_path = self.root / f"bundle-plan-{bundle_id}.json"
            bundle_path = self.root / f"bundle-{bundle_id}.zip"
            self.write_json(plan_path, plan)
            build_result = deterministic_source_bundle.build(
                plan_path, bundle_path
            )
            source_bundle = build_result["source_bundle"]
            self.bundle_payloads[bundle_id] = bundle_path.read_bytes()
            resources.append(
                {
                    "task_id": task_id,
                    "scope_kind": "sku",
                    "scope_ref_id": scope_ref_id,
                    "working_revision_no": 1,
                    "finalized_revision_no": 1,
                    "history": [
                        {
                            "revision_no": 1,
                            "status": "finalized",
                            "source_stage": "design",
                            "source_bundle": source_bundle,
                        }
                    ],
                }
            )
        recoveries = []
        for missing_id, source_id in self.sources.items():
            recoveries.append(
                {
                    "confidence": "confirmed_auto",
                    "confirmed_at": "2026-07-25T00:00:00Z",
                    "confirmed_by": 1,
                    "controlled_read_evidence_sha256": self.controlled_digest,
                    "expected_file_size": self.sizes[missing_id],
                    "manifest_row_hash": "a" * 64,
                    "missing_task_asset_id": missing_id,
                    "recovery_source_sha256": self.content_hashes[missing_id],
                    "recovery_source_storage_ref_id": self.storage_refs[source_id],
                    "recovery_source_task_asset_id": source_id,
                    "strategy": receipts.BUNDLE_STRATEGY,
                    "task_id": 900 + missing_id,
                }
            )
        recoveries.append(
            {
                "missing_task_asset_id": 999,
                "strategy": "historical_unavailable_tombstone_v1",
            }
        )
        return {
            "version": 2,
            "access_decisions": [],
            "asset_recoveries": recoveries,
            "organization_mappings": [],
            "planning_tasks": [],
            "resources": resources,
            "task_state_decisions": [],
        }

    def write_reviewed_manifest(self) -> None:
        lines = []
        for resource in self.mapping["resources"]:
            locator = (
                f"{resource['task_id']}:{resource['scope_kind']}:"
                f"{resource['scope_ref_id']}:1"
            )
            lines.append(
                {
                    "run_id": "review-run",
                    "review_state": "pass",
                    "gate_name": "G04",
                    "entity_key": f"revision-source:{locator}",
                    "detail_json": {
                        "components": [
                            locator,
                            "source",
                            "irrelevant",
                            "bound",
                            "source",
                            "",
                            "",
                        ],
                        "input_sha256": {
                            "mapping_sha256": self.mapping_sha256,
                        },
                    },
                }
            )
        self.manifest_path.write_bytes(
            b"".join(receipts.canonical(line) + b"\n" for line in lines)
        )

    def snapshot_rows(self) -> list[dict]:
        rows = []
        for missing_id, source_id in self.sources.items():
            rows.append(
                {
                    "id": missing_id,
                    "task_id": 900 + missing_id,
                    "asset_id": 700 + missing_id,
                    "storage_ref_id": f"deleted-ref-{missing_id}",
                    "file_size": self.sizes[missing_id],
                    "mime_type": "image/jpeg",
                    "whole_hash": None,
                    "deleted_at": "2026-07-24T00:00:00.000000Z",
                    "object_deleted_at": None,
                }
            )
            rows.append(
                {
                    "id": source_id,
                    "task_id": 2098,
                    "asset_id": 800 + source_id,
                    "storage_ref_id": self.storage_refs[source_id],
                    "file_size": self.sizes[missing_id],
                    "mime_type": "image/jpeg",
                    "whole_hash": None,
                    "deleted_at": None,
                    "object_deleted_at": None,
                }
            )
        rows.extend(copy.deepcopy(list(self.bundle_member_rows.values())))
        return sorted(rows, key=lambda row: row["id"])

    def write_snapshot(self, rows: list[dict] | None = None) -> None:
        rows = self.snapshot_rows() if rows is None else rows
        row_hashes = []
        lines = []
        for row in rows:
            row_hash = receipts.sha256(
                receipts.canonical(
                    {
                        "dataset": "task_assets",
                        "row": row,
                        "schema_version": 2,
                    }
                )
            )
            row_hashes.append(row_hash)
            lines.append(
                receipts.canonical(
                    {
                        "dataset": "task_assets",
                        "row": row,
                        "row_key": row["id"],
                        "row_sha256": row_hash,
                    }
                )
                + b"\n"
            )
        dataset_bytes = b"".join(lines)
        dataset_path = self.snapshot_dir / "task_assets.ndjson"
        dataset_path.write_bytes(dataset_bytes)
        schema: list[dict] = []
        descriptor = {
            "columns_sha256": "b" * 64,
            "dataset": "task_assets",
            "dataset_sha256": receipts.sha256(
                "".join(f"{item}\n" for item in row_hashes).encode("ascii")
            ),
            "file": "task_assets.ndjson",
            "file_sha256": receipts.sha256(dataset_bytes),
            "first_key": rows[0]["id"],
            "key": "id",
            "last_key": rows[-1]["id"],
            "row_count": len(rows),
            "schema": schema,
            "schema_sha256": receipts.sha256(receipts.canonical(schema)),
            "source_table": "task_assets",
        }
        manifest = self.signed(
            {
                "database": "frozen-a",
                "datasets": [descriptor],
                "export_contract": "frozen_a_oracle_v2",
                "mysql_evidence": {"snapshot": "fixture"},
                "schema_version": 2,
                "transaction": {
                    "access_mode": "READ ONLY",
                    "consistent_snapshot": True,
                },
            }
        )
        self.write_json(self.snapshot_manifest_path, manifest)

    def make_ownership(
        self,
        path: pathlib.Path,
        *,
        target_path: pathlib.Path,
        content_sha256: str,
        size: int,
    ) -> None:
        document = self.signed(
            {
                "device": 1,
                "inode": 2,
                "run_id": self.run_id,
                "schema_version": 1,
                "sha256": content_sha256,
                "size": size,
                "staging_path": str(target_path) + ".staging",
                "status": "OWNED_LINK",
                "target_path": str(target_path),
            },
            newline=path.name.startswith("bundle-ownership-"),
        )
        self.write_json(path, document)

    def write_bundle_evidence(self) -> None:
        b_root = self.root / "bundle-objects"
        entries = []
        for resource in self.mapping["resources"]:
            source_bundle = resource["history"][0]["source_bundle"]
            bundle_id = source_bundle["task_asset_id"]
            object_key = f"bundle/task-{resource['task_id']}.zip"
            relative = f"objects/{object_key}"
            target_path = b_root / "objects" / object_key
            target_path.parent.mkdir(parents=True, exist_ok=True)
            target_path.write_bytes(self.bundle_payloads[bundle_id])
            ownership_path = self.root / f"bundle-ownership-{bundle_id}.json"
            size = len(self.bundle_payloads[bundle_id])
            self.make_ownership(
                ownership_path,
                target_path=target_path,
                content_sha256=source_bundle["bundle_sha256"],
                size=size,
            )
            self.bundle_ownership_paths.append(ownership_path)
            storage_ref = f"bundle-ref-{bundle_id}"
            root_asset_id = 600 + bundle_id
            entries.append(
                {
                    "asset_storage_ref_candidate": {
                        "asset_id": root_asset_id,
                        "checksum_hint": source_bundle["bundle_sha256"],
                        "file_size": size,
                        "mime_type": "application/zip",
                        "ref_id": storage_ref,
                        "ref_key": object_key,
                    },
                    "bundle_sha256": source_bundle["bundle_sha256"],
                    "disposition": "create",
                    "object_key": object_key,
                    "relative_object_path": relative,
                    "revision_no": 1,
                    "rollback_candidate": {
                        "expected_sha256": source_bundle["bundle_sha256"],
                        "ownership_receipt_path": str(ownership_path),
                        "relative_object_path": relative,
                        "storage_ref_id": storage_ref,
                        "task_asset_id": bundle_id,
                    },
                    "scope_kind": resource["scope_kind"],
                    "scope_ref_id": resource["scope_ref_id"],
                    "size": size,
                    "source_bundle": source_bundle,
                    "task_asset_candidate": {
                        "asset_id": root_asset_id,
                        "asset_type": "source",
                        "file_size": size,
                        "id": bundle_id,
                        "mime_type": "application/zip",
                        "storage_key": object_key,
                        "storage_ref_id": storage_ref,
                        "whole_hash": source_bundle["bundle_sha256"],
                    },
                    "task_id": resource["task_id"],
                }
            )
        registry = self.signed(
            {
                "b_root": str(b_root),
                "database_write_performed": False,
                "entries": entries,
                "manifest_sha256": "d" * 64,
                "run_id": self.run_id,
                "schema_version": 1,
                "status": "MATERIALIZED",
                "write_ahead_sha256": "e" * 64,
            },
            newline=True,
        )
        self.write_json(self.bundle_registry_path, registry)

    def write_recovery_evidence(self) -> None:
        controlled = []
        plan_entries = []
        for missing_id, source_id in self.sources.items():
            size = self.sizes[missing_id]
            content_hash = self.content_hashes[missing_id]
            storage_ref = self.storage_refs[source_id]
            controlled.append(
                {
                    "missing_task_asset_id": missing_id,
                    "source_fetch_receipt": {
                        "mime_type": "image/jpeg",
                        "sha256": content_hash,
                        "size": size,
                        "storage_ref_id": storage_ref,
                        "task_asset_id": source_id,
                    },
                    "source_sha256": content_hash,
                    "source_task_asset": {
                        "file_size": size,
                        "id": source_id,
                        "mime_type": "image/jpeg",
                        "storage_ref_id": storage_ref,
                        "task_id": 2098,
                    },
                }
            )
            target_object_key = f"recovered/{missing_id}.jpg"
            target_path = (
                self.root / "recovery-fixture" / "objects" / target_object_key
            )
            ownership_path = self.root / f"recovery-ownership-{missing_id}.json"
            self.make_ownership(
                ownership_path,
                target_path=target_path,
                content_sha256=content_hash,
                size=size,
            )
            self.recovery_ownership_paths.append(ownership_path)
            target_ref = f"target-ref-{missing_id}"
            plan_entries.append(
                {
                    "db_apply_plan": {
                        "insert_asset_storage_ref": {
                            "asset_id": 700 + missing_id,
                            "checksum_hint": content_hash,
                            "file_size": size,
                            "mime_type": "image/jpeg",
                            "ref_id": target_ref,
                            "ref_key": target_object_key,
                        },
                        "update_task_asset": {
                            "set": {
                                "file_size": size,
                                "mime_type": "image/jpeg",
                                "storage_key": target_object_key,
                                "storage_ref_id": target_ref,
                                "whole_hash": content_hash,
                            },
                            "where": {"id": missing_id},
                        },
                        "update_upload_request": {},
                    },
                    "derivative_lineage": {"kind": "exact-copy"},
                    "missing_task_asset_id": missing_id,
                    "rollback_registry": {
                        "expected_fixture_sha256": content_hash,
                        "ownership_receipt_path": str(ownership_path),
                    },
                    "source_local_path": f"/evidence/{source_id}.jpg",
                    "source_sha256": content_hash,
                    "source_size": size,
                    "source_task_asset_id": source_id,
                    "target_object_key": target_object_key,
                    "target_storage_ref_id": target_ref,
                }
            )
        evidence = self.signed(
            {
                "controlled_read_receipts_sha256": self.controlled_digest,
                "database_writes_executed": False,
                "mapping_sha256": self.mapping_sha256,
                "production_connections_opened": False,
                "recoveries": controlled,
                "run_id": "controlled-read-run",
                "status": "PASS",
                "version": 1,
            }
        )
        self.write_json(self.recovery_evidence_path, evidence)
        plan = self.signed(
            {
                "database_writes_executed": False,
                "entries": plan_entries,
                "mapping_sha256": self.mapping_sha256,
                "production_writes_executed": False,
                "run_id": self.run_id,
                "status": "MATERIALIZED",
                "version": 1,
            }
        )
        self.write_json(self.recovery_plan_path, plan)

    def build(self) -> tuple[dict, dict]:
        return receipts.build_receipts(
            reviewed_mapping_path=self.mapping_path,
            expected_mapping_sha256=self.mapping_sha256,
            reviewed_manifest_path=self.manifest_path,
            a_snapshot_manifest_path=self.snapshot_manifest_path,
            bundle_registry_path=self.bundle_registry_path,
            bundle_ownership_paths=self.bundle_ownership_paths,
            recovery_plan_path=self.recovery_plan_path,
            recovery_evidence_path=self.recovery_evidence_path,
            recovery_ownership_paths=self.recovery_ownership_paths,
        )

    def assert_self_hash(self, document: dict) -> None:
        unsigned = {
            key: value
            for key, value in document.items()
            if key != "evidence_sha256"
        }
        self.assertEqual(
            document["evidence_sha256"],
            receipts.sha256(receipts.canonical(unsigned)),
        )

    def test_builds_direct_schema_v2_receipts_and_consumes_all_evidence(self) -> None:
        bundle, recovery = self.build()
        self.assertEqual(bundle["schema_version"], 2)
        self.assertEqual(bundle["kind"], receipts.BUNDLE_KIND)
        self.assertEqual(bundle["status"], "approved")
        self.assertEqual(bundle["mapping_sha256"], self.mapping_sha256)
        self.assertEqual(
            bundle["reviewed_manifest_sha256"],
            receipts.sha256(self.manifest_path.read_bytes()),
        )
        self.assertEqual(len(bundle["entries"]), 2)
        self.assertEqual(len(recovery["entries"]), 3)
        self.assertEqual(
            bundle["entries"][0]["members"][0],
            {
                "task_asset_id": 300,
                "storage_ref_id": "bundle-member-ref-300",
                "size": len(b"bundle-0-member-0"),
                "mime_type": "application/octet-stream",
                "sha256": receipts.sha256(b"bundle-0-member-0"),
            },
        )
        self.assertEqual(
            recovery["entries"][0],
            {
                "missing_task_asset_id": 101,
                "target_root_asset_id": 801,
                "target_task_id": 1001,
                "target_storage_ref_id": "target-ref-101",
                "target_object_key_sha256": receipts.object_key_sha256(
                    "recovered/101.jpg"
                ),
                "target_content_sha256": "1" * 64,
                "target_size": 1001,
                "target_mime": "image/jpeg",
                "source_task_asset_id": 201,
                "source_task_id": 2098,
                "source_storage_ref_id": "source-ref-201",
                "source_content_sha256": "1" * 64,
                "source_size": 1001,
                "source_mime": "image/jpeg",
                "strategy": receipts.BUNDLE_STRATEGY,
                "source_receipt_sha256": self.controlled_digest,
            },
        )
        self.assertEqual(
            bundle["source_evidence_sha256"],
            sorted(set(bundle["source_evidence_sha256"])),
        )
        self.assertEqual(
            recovery["source_evidence_sha256"],
            sorted(set(recovery["source_evidence_sha256"])),
        )
        self.assert_self_hash(bundle)
        self.assert_self_hash(recovery)

    def test_rejects_bundle_member_or_order_drift(self) -> None:
        self.rewrite_signed(
            self.bundle_registry_path,
            lambda document: document["entries"][0]["source_bundle"]["members"].reverse(),
        )
        with self.assertRaisesRegex(
            receipts.ReceiptError, "source_bundle"
        ):
            self.build()

    def test_rejects_frozen_a_bundle_member_metadata_or_hash_drift(self) -> None:
        for field, value in {
            "storage_ref_id": "wrong-member-ref",
            "file_size": 999,
            "whole_hash": "f" * 64,
        }.items():
            with self.subTest(field=field):
                rows = self.snapshot_rows()
                next(row for row in rows if row["id"] == 300)[field] = value
                self.write_snapshot(rows)
                with self.assertRaisesRegex(receipts.ReceiptError, "member 300"):
                    self.build()
                self.write_snapshot()

    def test_rejects_extra_ownership_receipt(self) -> None:
        extra_path = self.root / "bundle-ownership-extra.json"
        self.make_ownership(
            extra_path,
            target_path=self.root / "bundle-objects" / "objects" / "extra.zip",
            content_sha256="f" * 64,
            size=42,
        )
        self.bundle_ownership_paths.append(extra_path)
        with self.assertRaisesRegex(
            receipts.ReceiptError, "unconsumed bundle ownership"
        ):
            self.build()

    def test_rejects_frozen_a_package_or_row_hash_tamper(self) -> None:
        dataset_path = self.snapshot_dir / "task_assets.ndjson"
        dataset_path.write_bytes(dataset_path.read_bytes().replace(b"2098", b"2099", 1))
        with self.assertRaisesRegex(receipts.ReceiptError, "file_sha256"):
            self.build()

    def test_rejects_source_identity_drift_even_if_snapshot_is_rehashed(self) -> None:
        rows = self.snapshot_rows()
        next(row for row in rows if row["id"] == 201)["task_id"] = 2099
        self.write_snapshot(rows)
        with self.assertRaisesRegex(receipts.ReceiptError, "source_task_asset.task_id"):
            self.build()

    def test_rejects_recovery_ownership_content_drift(self) -> None:
        path = self.recovery_ownership_paths[0]
        self.rewrite_signed(path, lambda document: document.update(sha256="f" * 64))
        with self.assertRaisesRegex(receipts.ReceiptError, "ownership.sha256"):
            self.build()

    def test_rejects_review_manifest_not_bound_to_mapping(self) -> None:
        rows = [
            json.loads(line)
            for line in self.manifest_path.read_text().splitlines()
        ]
        rows[0]["detail_json"]["input_sha256"]["mapping_sha256"] = "0" * 64
        self.manifest_path.write_bytes(
            b"".join(receipts.canonical(row) + b"\n" for row in rows)
        )
        with self.assertRaisesRegex(receipts.ReceiptError, "mapping_sha256"):
            self.build()

    def test_immutable_writer_allows_identical_rerun_only(self) -> None:
        bundle, _ = self.build()
        output = self.root / "bundle-receipt.json"
        receipts.write_immutable(output, bundle)
        receipts.write_immutable(output, bundle)
        changed = copy.deepcopy(bundle)
        changed["status"] = "changed"
        with self.assertRaisesRegex(receipts.ReceiptError, "refusing to replace"):
            receipts.write_immutable(output, changed)


if __name__ == "__main__":
    unittest.main()
