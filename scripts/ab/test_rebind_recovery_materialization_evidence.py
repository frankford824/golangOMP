import copy
import hashlib
import tempfile
import unittest
from pathlib import Path

from scripts.ab import rebind_recovery_materialization_evidence as rebind


def recovery_row(missing_id: int) -> dict:
    return {
        "missing_task_asset_id": missing_id,
        "task_id": 2807,
        "recovery_source_task_asset_id": missing_id + 100,
        "expected_file_size": 4,
        "recovery_source_storage_ref_id": f"source-ref-{missing_id}",
    }


class RecoveryMaterializationRebindTests(unittest.TestCase):
    def test_recovery_subdocument_must_be_identical(self) -> None:
        old = {
            "asset_recoveries": [
                recovery_row(missing_id) for missing_id in rebind.RECOVERY_IDS
            ]
        }
        new = copy.deepcopy(old)
        rows, digest = rebind.validate_equal_recovery_scope(old, new)
        self.assertEqual(tuple(sorted(rows)), rebind.RECOVERY_IDS)
        self.assertEqual(
            digest,
            hashlib.sha256(
                rebind.canonical(
                    [rows[missing_id] for missing_id in rebind.RECOVERY_IDS]
                )
            ).hexdigest(),
        )

        new["asset_recoveries"][1]["expected_file_size"] = 5
        with self.assertRaisesRegex(
            ValueError, "recovery subdocuments differ"
        ):
            rebind.validate_equal_recovery_scope(old, new)

    def test_old_evidence_self_hash_and_mapping_are_hard_bound(self) -> None:
        unsigned = {
            "version": 1,
            "run_id": "old-run",
            "status": "PASS",
            "mapping_sha256": "a" * 64,
            "database_writes_executed": False,
            "production_connections_opened": False,
            "controlled_read_receipts_sha256": "b" * 64,
            "recoveries": [
                {
                    "missing_task_asset_id": missing_id,
                    "source_sha256": "c" * 64,
                }
                for missing_id in rebind.RECOVERY_IDS
            ],
        }
        document = rebind.signed(unsigned)
        rows, evidence = rebind.validate_old_evidence(
            document, old_mapping_sha256="a" * 64
        )
        self.assertEqual(tuple(sorted(rows)), rebind.RECOVERY_IDS)
        self.assertEqual(evidence, document["evidence_sha256"])

        document["recoveries"][0]["source_sha256"] = "d" * 64
        with self.assertRaisesRegex(ValueError, "self-hash differs"):
            rebind.validate_old_evidence(
                document, old_mapping_sha256="a" * 64
            )

    def test_current_physical_target_drift_is_a_hard_failure(self) -> None:
        with tempfile.TemporaryDirectory() as raw:
            source_path = Path(raw) / "source.bin"
            source_path.write_bytes(b"data")
            digest = rebind.sha256_file(source_path)
            mapping = {
                "task_id": 2807,
                "recovery_source_task_asset_id": 24034,
                "expected_file_size": 4,
                "recovery_source_storage_ref_id": "source-ref",
            }
            evidence = {
                "source_sha256": digest,
                "source_local_path": str(source_path),
                "source_fetch_receipt": {
                    "sha256": digest,
                    "size": 4,
                },
            }
            plan = {
                "target_object_key": "v8-production/recovered.bin",
                "target_storage_ref_id": "target-ref",
                "db_apply_plan": {
                    "insert_asset_storage_ref": {
                        "ref_id": "target-ref",
                        "ref_key": "v8-production/recovered.bin",
                        "checksum_hint": digest,
                    },
                    "update_task_asset": {
                        "set": {
                            "storage_ref_id": "target-ref",
                            "storage_key": "v8-production/recovered.bin",
                            "whole_hash": digest,
                        }
                    },
                },
            }
            task_assets = {
                23989: {
                    "task_id": 2807,
                    "storage_ref_id": "target-ref",
                    "storage_key": "v8-production/recovered.bin",
                    "whole_hash": digest,
                    "file_size": 4,
                    "mime_type": "image/jpeg",
                    "deleted_at": None,
                    "object_deleted_at": None,
                },
                24034: {
                    "storage_ref_id": "source-ref",
                    "file_size": 4,
                    "mime_type": "image/jpeg",
                },
            }
            objects = {
                "target-ref": {
                    "ref_key": "v8-production/recovered.bin",
                    "checksum_hint": digest,
                    "file_size": 4,
                    "mime_type": "image/jpeg",
                }
            }
            self.assertEqual(
                rebind.validate_current_target(
                    missing_id=23989,
                    mapping_row=mapping,
                    evidence_row=evidence,
                    plan_entry=plan,
                    task_assets=task_assets,
                    objects=objects,
                ),
                (digest, 4, "image/jpeg"),
            )

            task_assets[23989]["storage_key"] = "wrong"
            with self.assertRaisesRegex(
                ValueError, "physical A recovery target 23989 differs"
            ):
                rebind.validate_current_target(
                    missing_id=23989,
                    mapping_row=mapping,
                    evidence_row=evidence,
                    plan_entry=plan,
                    task_assets=task_assets,
                    objects=objects,
                )


if __name__ == "__main__":
    unittest.main()
