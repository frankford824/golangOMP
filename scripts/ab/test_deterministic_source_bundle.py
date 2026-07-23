import hashlib
import importlib.util
import json
import pathlib
import tempfile
import unittest
import zipfile


PATH = pathlib.Path(__file__).with_name("deterministic_source_bundle.py")
SPEC = importlib.util.spec_from_file_location("deterministic_source_bundle", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class DeterministicSourceBundleTest(unittest.TestCase):
    def make_plan(self, root):
        members = []
        for task_asset_id, name, content in ((31, "first source.psd", b"first"), (32, "../second.ai", b"second")):
            path = root / f"{task_asset_id}.bin"
            path.write_bytes(content)
            members.append({
                "task_asset_id": task_asset_id,
                "asset_id": task_asset_id + 100,
                "storage_ref_id": f"ref-{task_asset_id}",
                "original_file_name": name,
                "local_path": str(path),
                "sha256": hashlib.sha256(content).hexdigest(),
                "source_stage": "design",
                "evidence_event_ids": [f"task_event_log:event-{task_asset_id}"],
                "confirmed": True,
            })
        plan = {
            "version": 1,
            "bundle_task_asset_id": 30,
            "confirmed_by": 21,
            "confirmed_at": "2026-07-22T00:00:00Z",
            "confirmation_note": "reviewed ordered source members",
            "members": members,
        }
        plan_path = root / "plan.json"
        plan_path.write_text(json.dumps(plan), encoding="utf-8")
        return plan_path, plan

    def test_repeated_builds_are_byte_identical_and_manifest_is_safe(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            plan_path, _ = self.make_plan(root)
            first = MODULE.build(plan_path, root / "one.zip")
            second = MODULE.build(plan_path, root / "two.zip")
            self.assertEqual(first["bundle_sha256"], second["bundle_sha256"])
            self.assertEqual((root / "one.zip").read_bytes(), (root / "two.zip").read_bytes())
            self.assertEqual(first["status"], "PASS")
            self.assertEqual(first["violation_count"], 0)
            with zipfile.ZipFile(root / "one.zip") as bundle:
                self.assertEqual(bundle.namelist(), ["manifest.json", "001_31_first_source.psd", "002_32_second.ai"])
                manifest = json.loads(bundle.read("manifest.json"))
                self.assertNotIn("local_path", manifest["members"][0])
                self.assertTrue(all(info.date_time == MODULE.FIXED_TIME for info in bundle.infolist()))

    def test_hash_mismatch_and_unreviewed_member_fail_closed(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            plan_path, plan = self.make_plan(root)
            plan["members"][0]["sha256"] = "0" * 64
            plan_path.write_text(json.dumps(plan), encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "does not match"):
                MODULE.build(plan_path, root / "bad.zip")
            self.assertFalse((root / "bad.zip").exists())

            plan_path, plan = self.make_plan(root)
            plan["members"][0]["confirmed"] = False
            plan_path.write_text(json.dumps(plan), encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "confirmed must be true"):
                MODULE.build(plan_path, root / "unreviewed.zip")


if __name__ == "__main__":
    unittest.main()
