import hashlib
import importlib.util
import json
import pathlib
import sys
import tempfile
import unittest


PATH = pathlib.Path(__file__).with_name(
    "rollback_asset_recovery_materialization.py"
)
SPEC = importlib.util.spec_from_file_location(
    "rollback_asset_recovery_materialization", PATH
)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class RollbackAssetRecoveryMaterializationTest(unittest.TestCase):
    def make_plan(self, raw):
        root = pathlib.Path(raw) / "fixture-upload-b"
        root.mkdir()
        data = b"recovered"
        entries = []
        targets = []
        for missing_id in (23989, 23990, 23991):
            key = (
                "v8-ab/formal-test/recovered/task-1/"
                f"task-asset-{missing_id}/asset.bin"
            )
            target = root / "objects" / pathlib.Path(key)
            target.parent.mkdir(parents=True)
            target.write_bytes(data)
            staging = (
                root.parent
                / f".recovery-stage-{missing_id}-{hashlib.sha256(data).hexdigest()}.bin"
            )
            ownership_receipt = (
                root.parent / f"recovery-ownership-{missing_id}.json"
            )
            staging_ownership_receipt = (
                root.parent
                / f"recovery-staging-ownership-{missing_id}.json"
            )
            staging_private = (
                root.parent / f".recovery-private-{missing_id}.bin"
            )
            staging_token = hashlib.sha256(
                f"formal-test:{missing_id}".encode()
            ).hexdigest()
            stat = target.stat()
            receipt = {
                "schema_version": 1,
                "status": "OWNED_LINK",
                "run_id": "formal-test",
                "target_path": str(target.resolve()),
                "staging_path": str(staging.resolve()),
                "device": stat.st_dev,
                "inode": stat.st_ino,
                "size": stat.st_size,
                "sha256": hashlib.sha256(data).hexdigest(),
            }
            receipt["evidence_sha256"] = hashlib.sha256(
                json.dumps(
                    receipt,
                    ensure_ascii=False,
                    sort_keys=True,
                    separators=(",", ":"),
                ).encode("utf-8")
            ).hexdigest()
            ownership_receipt.write_text(
                json.dumps(receipt, sort_keys=True), encoding="utf-8"
            )
            entries.append(
                {
                    "missing_task_asset_id": missing_id,
                    "target_object_key": key,
                    "source_sha256": hashlib.sha256(data).hexdigest(),
                    "source_size": len(data),
                    "rollback_registry": {
                        "fixture_disposition": "created",
                        "staging_local_path": str(staging),
                        "ownership_receipt_path": str(ownership_receipt),
                        "staging_ownership_receipt_path": str(
                            staging_ownership_receipt
                        ),
                        "staging_private_path": str(staging_private),
                        "staging_ownership_token": staging_token,
                    },
                }
            )
            targets.append(target)
        plan = {
            "version": 1,
            "status": "MATERIALIZED",
            "run_id": "formal-test",
            "database_writes_executed": False,
            "production_writes_executed": False,
            "entries": entries,
        }
        plan["evidence_sha256"] = hashlib.sha256(
            json.dumps(
                plan,
                ensure_ascii=False,
                sort_keys=True,
                separators=(",", ":"),
            ).encode("utf-8")
        ).hexdigest()
        return root, targets[0], plan

    def test_exact_registered_object_is_removed(self):
        with tempfile.TemporaryDirectory() as raw:
            root, target, plan = self.make_plan(raw)
            result = MODULE.rollback(plan, root)
            self.assertEqual(result["status"], "ROLLED_BACK")
            self.assertFalse(target.exists())
            self.assertEqual(len(result["removed_object_keys"]), 3)

    def test_prepared_write_ahead_preserves_unowned_partial_staging(self):
        with tempfile.TemporaryDirectory() as raw:
            root, target, plan = self.make_plan(raw)
            plan["status"] = "PREPARED"
            stage = pathlib.Path(
                plan["entries"][0]["rollback_registry"][
                    "staging_local_path"
                ]
            )
            stage.write_bytes(b"partial")
            unsigned = dict(plan)
            unsigned.pop("evidence_sha256")
            plan["evidence_sha256"] = hashlib.sha256(
                json.dumps(
                    unsigned,
                    ensure_ascii=False,
                    sort_keys=True,
                    separators=(",", ":"),
                ).encode("utf-8")
            ).hexdigest()
            with self.assertRaisesRegex(
                ValueError, "staging ownership cannot be proven"
            ):
                MODULE.rollback(plan, root)
            self.assertTrue(stage.exists())

    def test_reused_identical_object_is_retained(self):
        with tempfile.TemporaryDirectory() as raw:
            root, target, plan = self.make_plan(raw)
            for entry in plan["entries"]:
                entry["rollback_registry"][
                    "fixture_disposition"
                ] = "reused_identical"
            unsigned = dict(plan)
            unsigned.pop("evidence_sha256")
            plan["evidence_sha256"] = hashlib.sha256(
                json.dumps(
                    unsigned,
                    ensure_ascii=False,
                    sort_keys=True,
                    separators=(",", ":"),
                ).encode("utf-8")
            ).hexdigest()
            result = MODULE.rollback(plan, root)
            self.assertTrue(target.exists())
            self.assertEqual(result["removed_object_keys"], [])
            self.assertEqual(
                result["retained_reused_object_keys"],
                [
                    entry["target_object_key"]
                    for entry in plan["entries"]
                ],
            )
            self.assertEqual(result["already_absent_count"], 0)

    def test_unowned_identical_target_is_never_removed(self):
        with tempfile.TemporaryDirectory() as raw:
            root, target, plan = self.make_plan(raw)
            receipt = pathlib.Path(
                plan["entries"][0]["rollback_registry"][
                    "ownership_receipt_path"
                ]
            )
            receipt.unlink()
            replacement = target.read_bytes()
            target.unlink()
            target.write_bytes(replacement)
            with self.assertRaisesRegex(ValueError, "ownership cannot be proven"):
                MODULE.rollback(plan, root)
            self.assertTrue(target.exists())

    def test_stage_hard_link_proves_ownership_without_receipt(self):
        with tempfile.TemporaryDirectory() as raw:
            root, target, plan = self.make_plan(raw)
            entry = plan["entries"][0]
            receipt = pathlib.Path(
                entry["rollback_registry"]["ownership_receipt_path"]
            )
            receipt.unlink()
            staging = pathlib.Path(
                entry["rollback_registry"]["staging_local_path"]
            )
            staging.hardlink_to(target)
            staging_receipt = pathlib.Path(
                entry["rollback_registry"][
                    "staging_ownership_receipt_path"
                ]
            )
            stat = staging.stat()
            receipt = {
                "schema_version": 1,
                "status": "STAGING_OWNED",
                "run_id": "formal-test",
                "staging_path": str(staging.resolve()),
                "device": stat.st_dev,
                "inode": stat.st_ino,
                "size": stat.st_size,
                "sha256": hashlib.sha256(staging.read_bytes()).hexdigest(),
            }
            receipt["evidence_sha256"] = hashlib.sha256(
                json.dumps(
                    receipt,
                    ensure_ascii=False,
                    sort_keys=True,
                    separators=(",", ":"),
                ).encode("utf-8")
            ).hexdigest()
            staging_receipt.write_text(
                json.dumps(receipt, sort_keys=True), encoding="utf-8"
            )
            result = MODULE.rollback(plan, root)
            self.assertFalse(target.exists())
            self.assertFalse(staging.exists())
            self.assertIn(entry["target_object_key"], result["removed_object_keys"])

    def test_same_bytes_replacement_staging_is_preserved(self):
        with tempfile.TemporaryDirectory() as raw:
            root, _, plan = self.make_plan(raw)
            entry = plan["entries"][0]
            staging = pathlib.Path(
                entry["rollback_registry"]["staging_local_path"]
            )
            staging.write_bytes(b"recovered")
            receipt_path = pathlib.Path(
                entry["rollback_registry"][
                    "staging_ownership_receipt_path"
                ]
            )
            stat = staging.stat()
            receipt = {
                "schema_version": 1,
                "status": "STAGING_OWNED",
                "run_id": "formal-test",
                "staging_path": str(staging.resolve()),
                "device": stat.st_dev,
                "inode": stat.st_ino,
                "size": stat.st_size,
                "sha256": hashlib.sha256(staging.read_bytes()).hexdigest(),
            }
            receipt["evidence_sha256"] = hashlib.sha256(
                json.dumps(
                    receipt,
                    ensure_ascii=False,
                    sort_keys=True,
                    separators=(",", ":"),
                ).encode("utf-8")
            ).hexdigest()
            receipt_path.write_text(
                json.dumps(receipt, sort_keys=True), encoding="utf-8"
            )
            replacement = staging.with_name("replacement.bin")
            replacement.write_bytes(b"recovered")
            MODULE.os.replace(replacement, staging)
            unsigned = dict(plan)
            unsigned.pop("evidence_sha256")
            plan["evidence_sha256"] = hashlib.sha256(
                json.dumps(
                    unsigned,
                    ensure_ascii=False,
                    sort_keys=True,
                    separators=(",", ":"),
                ).encode("utf-8")
            ).hexdigest()
            with self.assertRaisesRegex(
                ValueError, "staging ownership cannot be proven"
            ):
                MODULE.rollback(plan, root)
            self.assertEqual(staging.read_bytes(), b"recovered")

    def test_receipted_private_is_removed_when_stage_was_not_published(self):
        with tempfile.TemporaryDirectory() as raw:
            root, _, plan = self.make_plan(raw)
            entry = plan["entries"][0]
            staging = pathlib.Path(
                entry["rollback_registry"]["staging_local_path"]
            )
            private = pathlib.Path(
                entry["rollback_registry"]["staging_private_path"]
            )
            private.write_bytes(b"recovered")
            receipt_path = pathlib.Path(
                entry["rollback_registry"][
                    "staging_ownership_receipt_path"
                ]
            )
            stat = private.stat()
            receipt = {
                "schema_version": 1,
                "status": "STAGING_OWNED",
                "run_id": "formal-test",
                "staging_path": str(staging.resolve()),
                "private_path": str(private.resolve()),
                "device": stat.st_dev,
                "inode": stat.st_ino,
                "size": stat.st_size,
                "sha256": hashlib.sha256(private.read_bytes()).hexdigest(),
            }
            receipt["evidence_sha256"] = hashlib.sha256(
                json.dumps(
                    receipt,
                    ensure_ascii=False,
                    sort_keys=True,
                    separators=(",", ":"),
                ).encode("utf-8")
            ).hexdigest()
            receipt_path.write_text(
                json.dumps(receipt, sort_keys=True), encoding="utf-8"
            )
            unsigned = dict(plan)
            unsigned.pop("evidence_sha256")
            plan["evidence_sha256"] = hashlib.sha256(
                json.dumps(
                    unsigned,
                    ensure_ascii=False,
                    sort_keys=True,
                    separators=(",", ":"),
                ).encode("utf-8")
            ).hexdigest()
            result = MODULE.rollback(plan, root)
            self.assertFalse(private.exists())
            self.assertIn(
                str(private),
                result["removed_private_staging_paths"],
            )

    @unittest.skipIf(MODULE.os.name == "nt", "xattr reservation is Linux-only")
    def test_wal_reserved_private_is_removed_before_receipt_exists(self):
        with tempfile.TemporaryDirectory() as raw:
            root, _, plan = self.make_plan(raw)
            entry = plan["entries"][0]
            private = pathlib.Path(
                entry["rollback_registry"]["staging_private_path"]
            )
            token = entry["rollback_registry"]["staging_ownership_token"]
            private.write_bytes(b"sensitive-source")
            MODULE.os.setxattr(
                private,
                MODULE.STAGE_TOKEN_XATTR,
                token.encode("ascii"),
            )
            result = MODULE.rollback(plan, root)
            self.assertFalse(private.exists())
            self.assertIn(
                str(private),
                result["removed_private_staging_paths"],
            )

    def test_drifted_object_is_never_removed(self):
        with tempfile.TemporaryDirectory() as raw:
            root, target, plan = self.make_plan(raw)
            target.write_bytes(b"drifted")
            with self.assertRaisesRegex(ValueError, "drifted recovery object"):
                MODULE.rollback(plan, root)
            self.assertTrue(target.exists())

    def test_prefix_escape_is_rejected(self):
        with tempfile.TemporaryDirectory() as raw:
            root, target, plan = self.make_plan(raw)
            plan["entries"][0]["target_object_key"] = "../outside"
            unsigned = dict(plan)
            unsigned.pop("evidence_sha256")
            plan["evidence_sha256"] = hashlib.sha256(
                json.dumps(
                    unsigned,
                    ensure_ascii=False,
                    sort_keys=True,
                    separators=(",", ":"),
                ).encode("utf-8")
            ).hexdigest()
            with self.assertRaisesRegex(ValueError, "outside the run recovery prefix"):
                MODULE.rollback(plan, root)
            self.assertTrue(target.exists())


if __name__ == "__main__":
    unittest.main()
