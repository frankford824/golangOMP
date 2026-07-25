import hashlib
import importlib.util
import json
import pathlib
import tempfile
import unittest


PATH = pathlib.Path(__file__).with_name("prepare_asset_recovery.py")
SPEC = importlib.util.spec_from_file_location("prepare_asset_recovery", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class PrepareAssetRecoveryTest(unittest.TestCase):
    def fixture(self, root: pathlib.Path):
        rows = []
        evidence_rows = []
        for missing_id, (task_id, source_id, size) in MODULE.ALLOWED.items():
            body = bytes([missing_id % 251]) * size
            source_path = root / f"{source_id}.bin"
            source_path.write_bytes(body)
            source_sha = hashlib.sha256(body).hexdigest()
            preview = hashlib.sha256(f"preview-{missing_id}".encode()).hexdigest()
            thumb = hashlib.sha256(f"thumb-{missing_id}".encode()).hexdigest()
            row = {
                "task_id": task_id,
                "missing_task_asset_id": missing_id,
                "recovery_source_task_asset_id": source_id,
                "strategy": MODULE.STRATEGY,
                "original_storage_ref_id": f"missing-ref-{missing_id}",
                "recovery_source_storage_ref_id": f"source-ref-{source_id}",
                "expected_file_size": size,
                "preview_whole_hash": preview,
                "design_thumb_whole_hash": thumb,
                "confidence": "confirmed_auto",
                "review_policy_ids": [MODULE.POLICY],
                "confirmed_by": 1,
                "confirmed_at": "2026-07-23T12:00:00Z",
                "confirmation_note": "reviewed exact source bytes and lineage",
                "manifest_row_hash": "",
            }
            row["manifest_row_hash"] = MODULE.sha256_bytes(
                MODULE.canonical_bytes(
                    {
                        key: value
                        for key, value in row.items()
                        if key != "manifest_row_hash"
                    }
                )
            )
            rows.append(row)
            missing_before = {
                "id": missing_id,
                "task_id": task_id,
                "asset_id": missing_id + 1000,
                "upload_request_id": f"request-{missing_id}",
                "storage_ref_id": f"missing-ref-{missing_id}",
                "storage_key": f"deleted/{missing_id}",
                "whole_hash": None,
                "upload_status": "uploaded",
                "deleted_at": "2026-07-22T00:00:00Z",
                "cleaned_at": None,
                "object_deleted_at": None,
                "access_revoked_at": None,
                "access_revoked_reason": "",
                "file_size": size,
                "file_name": f"{missing_id}.jpg",
                "mime_type": "image/jpeg",
            }
            evidence_rows.append({
                "missing_task_asset_id": missing_id,
                "source_local_path": str(source_path),
                "source_sha256": source_sha,
                "missing_task_asset_before": missing_before,
                "source_task_asset": {
                    "id": source_id,
                    "task_id": MODULE.SOURCE_TASK_ID,
                    "asset_type": "delivery",
                    "file_size": size,
                    "storage_ref_id": f"source-ref-{source_id}",
                    "storage_key": f"surviving/{source_id}.jpg",
                    "upload_status": "uploaded",
                    "deleted_at": None,
                    "object_deleted_at": None,
                },
                "source_fetch_receipt": {
                    "protocol": "controlled-asset-read-v1",
                    "task_asset_id": source_id,
                    "storage_ref_id": f"source-ref-{source_id}",
                    "object_key": f"surviving/{source_id}.jpg",
                    "size": size,
                    "sha256": source_sha,
                    "fetched_at": "2026-07-23T12:00:00Z",
                },
                "upload_request_before": {
                    "request_id": f"request-{missing_id}",
                    "bound_ref_id": f"missing-ref-{missing_id}",
                    "checksum_hint": "",
                    "file_size": size,
                    "status": "bound",
                    "session_status": "completed",
                },
                "original_storage_ref_before": {
                    "ref_id": f"missing-ref-{missing_id}",
                    "status": "recorded",
                },
                "missing_derivatives": [
                    {"asset_type": "preview", "source_asset_version_id": missing_id, "whole_hash": preview},
                    {"asset_type": "design_thumb", "source_asset_version_id": missing_id, "whole_hash": thumb},
                ],
                "source_derivatives": [
                    {"asset_type": "preview", "source_asset_version_id": source_id, "whole_hash": preview},
                    {"asset_type": "design_thumb", "source_asset_version_id": source_id, "whole_hash": thumb},
                ],
            })
        mapping = {"version": 2, "asset_recoveries": rows}
        mapping_path = root / "mapping.json"
        mapping_path.write_bytes(MODULE.canonical_bytes(mapping))
        evidence = {
            "version": 1,
            "run_id": "r20260723-test",
            "mapping_sha256": MODULE.sha256_file(mapping_path),
            "recoveries": evidence_rows,
        }
        evidence_path = root / "evidence.json"
        evidence_path.write_bytes(MODULE.canonical_bytes(evidence))
        return mapping_path, evidence_path

    def args(self, mapping, evidence, output, materialize=False, fixture_root=None):
        return type("Args", (), {
            "mapping": mapping,
            "evidence": evidence,
            "output": output,
            "materialize": materialize,
            "fixture_root": fixture_root,
        })()

    def test_prepare_is_write_free_and_contains_complete_rollback(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            mapping, evidence = self.fixture(root)
            output = root / "plan.json"
            plan = MODULE.run(self.args(mapping, evidence, output))
            self.assertEqual(plan["status"], "PREPARED")
            self.assertFalse(plan["database_writes_executed"])
            self.assertEqual(len(plan["entries"]), 3)
            for entry in plan["entries"]:
                self.assertIn("restore_task_asset", entry["rollback_registry"])
                self.assertIn("restore_upload_request", entry["rollback_registry"])
                self.assertIn(
                    "db_rollback_plan", entry["rollback_registry"]
                )
                self.assertFalse((root / "objects").exists())

    def test_materialize_is_contained_verified_and_idempotent(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            mapping, evidence = self.fixture(root)
            fixture = root / "fixture"
            output = root / "plan.json"
            args = self.args(mapping, evidence, output, True, fixture)
            first = MODULE.run(args)
            second = MODULE.run(args)
            self.assertEqual(first, second)
            for entry in first["entries"]:
                path = fixture / "objects" / entry["target_object_key"]
                self.assertEqual(MODULE.sha256_file(path), entry["source_sha256"])

    def test_two_phase_write_ahead_materialization_is_idempotent(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            mapping, evidence = self.fixture(root)
            fixture = root / "fixture-upload-b"
            fixture.mkdir()
            write_ahead = root / "recovery-write-ahead.json"
            wal_args = self.args(
                mapping, evidence, write_ahead, False, fixture
            )
            prepared = MODULE.run(wal_args)
            final = root / "recovery-materialized.json"
            apply_args = self.args(
                mapping, evidence, final, True, fixture
            )
            apply_args.expected_write_ahead = write_ahead
            first = MODULE.run(apply_args)
            second = MODULE.run(apply_args)
            self.assertEqual(prepared["entries"], first["entries"])
            self.assertEqual(first, second)

    def test_rejects_unconfirmed_and_byte_drift(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            mapping_path, evidence_path = self.fixture(root)
            mapping = json.loads(mapping_path.read_text())
            mapping["asset_recoveries"][0]["confidence"] = "proposed_review"
            mapping_path.write_bytes(MODULE.canonical_bytes(mapping))
            evidence = json.loads(evidence_path.read_text())
            evidence["mapping_sha256"] = MODULE.sha256_file(mapping_path)
            evidence_path.write_bytes(MODULE.canonical_bytes(evidence))
            with self.assertRaisesRegex(ValueError, "not an exact confirmed"):
                MODULE.run(self.args(mapping_path, evidence_path, root / "plan.json"))

            mapping_path, evidence_path = self.fixture(root)
            evidence = json.loads(evidence_path.read_text())
            pathlib.Path(evidence["recoveries"][0]["source_local_path"]).write_bytes(b"drift")
            with self.assertRaisesRegex(ValueError, "byte size drifted"):
                MODULE.run(self.args(mapping_path, evidence_path, root / "plan2.json"))


if __name__ == "__main__":
    unittest.main()
