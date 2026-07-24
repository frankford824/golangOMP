import hashlib
import importlib.util
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
        key = "v8-ab/formal-test/recovered/task-1/asset.bin"
        target = root / "objects" / pathlib.Path(key)
        target.parent.mkdir(parents=True)
        target.write_bytes(data)
        plan = {
            "version": 1,
            "status": "MATERIALIZED",
            "run_id": "formal-test",
            "database_writes_executed": False,
            "production_writes_executed": False,
            "entries": [
                {
                    "target_object_key": key,
                    "source_sha256": hashlib.sha256(data).hexdigest(),
                    "source_size": len(data),
                }
            ],
        }
        return root, target, plan

    def test_exact_registered_object_is_removed(self):
        with tempfile.TemporaryDirectory() as raw:
            root, target, plan = self.make_plan(raw)
            result = MODULE.rollback(plan, root)
            self.assertEqual(result["status"], "ROLLED_BACK")
            self.assertFalse(target.exists())
            self.assertEqual(result["removed_object_keys"], [
                "v8-ab/formal-test/recovered/task-1/asset.bin"
            ])

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
            with self.assertRaisesRegex(ValueError, "outside the run recovery prefix"):
                MODULE.rollback(plan, root)
            self.assertTrue(target.exists())


if __name__ == "__main__":
    unittest.main()
