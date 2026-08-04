import copy
import hashlib
import importlib.util
import pathlib
import unittest


PATH = pathlib.Path(__file__).with_name(
    "prepare_auto_source_bundle_manifest.py"
)
SPEC = importlib.util.spec_from_file_location(
    "prepare_auto_source_bundle_manifest",
    PATH,
)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class PrepareAutoSourceBundleManifestTest(unittest.TestCase):
    def fixture(self):
        member_ids = [101, 103, 109]
        mapping = {
            "version": 2,
            "resources": [
                {
                    "task_id": 480,
                    "scope_kind": "task",
                    "scope_ref_id": 0,
                    "history": [
                        {
                            "revision_no": 1,
                            "source_stage": "design",
                            "confidence": "hard_blocked",
                            "blockers": [MODULE.BUNDLE_BLOCKER],
                            "evidence_event_ids": [
                                "task_event_log:7001",
                                "task_event_log:7002",
                            ],
                            "source_bundle_candidate": {
                                "ordering": (
                                    "completion_time_then_task_asset_id"
                                ),
                                "ordered_member_task_asset_ids": member_ids,
                            },
                        }
                    ],
                }
            ],
        }
        objects = {
            member_id: {
                "entity_key": f"task_asset:{member_id}",
                "owner_kind": "task_asset",
                "owner_id": member_id,
                "task_id": 480,
                "storage_ref_id": f"ref-{member_id}",
                "object_key": f"tasks/480/{member_id}.psd",
                "mime_type": "application/octet-stream",
                "size": 1000 + member_id,
                "sha256": hashlib.sha256(
                    str(member_id).encode()
                ).hexdigest(),
                "status": "recorded",
                "is_placeholder": False,
            }
            for member_id in member_ids
        }
        assets = {
            member_id: {
                "id": member_id,
                "task_id": 480,
                "asset_id": 9000 + member_id,
                "asset_type": "source",
                "upload_status": "uploaded",
                "storage_ref_id": f"ref-{member_id}",
                "file_name": f"{member_id}.psd",
                "original_filename": f"source-{member_id}.psd",
            }
            for member_id in member_ids
        }
        events = [
            {
                "id": 7001,
                "task_id": 480,
                "payload": {
                    "task_asset_ids": [101, 103],
                },
            },
            {
                "id": 7002,
                "task_id": 480,
                "payload": {
                    "task_asset_id": 109,
                },
            },
        ]
        return mapping, objects, assets, events

    def build(self):
        mapping, objects, assets, events = self.fixture()
        return MODULE.build_manifest(
            mapping=mapping,
            mapping_sha256="a" * 64,
            object_rows=objects,
            task_asset_rows=assets,
            completion_events=events,
            max_task_asset_id=25000,
            max_asset_id=18000,
            run_id="v1295-auto-bundles",
            confirmed_by=1,
            confirmed_at="2026-08-04T11:15:30Z",
        )

    def test_manifest_preserves_order_and_allocates_after_frozen_maxima(self):
        first = self.build()
        second = self.build()
        self.assertEqual(first, second)
        self.assertEqual(first["confirmation_mode"], "automatic_policy_engine")
        self.assertIn(
            "no row-by-row human review",
            first["confirmation_note"],
        )
        self.assertEqual(first["bundle_count"], 1)
        self.assertEqual(first["member_count"], 3)
        bundle = first["bundles"][0]
        self.assertEqual(bundle["bundle_task_asset_id"], 25001)
        self.assertEqual(bundle["bundle_asset_id"], 18001)
        self.assertEqual(
            [
                member["task_asset_id"]
                for member in bundle["ordered_members"]
            ],
            [101, 103, 109],
        )

    def test_rejects_missing_hash_event_and_cross_task_member(self):
        mapping, objects, assets, events = self.fixture()
        objects[101]["sha256"] = ""
        with self.assertRaisesRegex(ValueError, "failed frozen"):
            MODULE.build_manifest(
                mapping=mapping,
                mapping_sha256="a" * 64,
                object_rows=objects,
                task_asset_rows=assets,
                completion_events=events,
                max_task_asset_id=25000,
                max_asset_id=18000,
                run_id="v1295-auto-bundles",
                confirmed_by=1,
                confirmed_at="2026-08-04T11:15:30Z",
            )
        mapping, objects, assets, events = self.fixture()
        events = copy.deepcopy(events)
        events[0]["payload"] = {}
        with self.assertRaisesRegex(ValueError, "failed frozen"):
            MODULE.build_manifest(
                mapping=mapping,
                mapping_sha256="a" * 64,
                object_rows=objects,
                task_asset_rows=assets,
                completion_events=events,
                max_task_asset_id=25000,
                max_asset_id=18000,
                run_id="v1295-auto-bundles",
                confirmed_by=1,
                confirmed_at="2026-08-04T11:15:30Z",
            )
        mapping, objects, assets, events = self.fixture()
        assets[109]["task_id"] = 999
        with self.assertRaisesRegex(ValueError, "failed frozen"):
            MODULE.build_manifest(
                mapping=mapping,
                mapping_sha256="a" * 64,
                object_rows=objects,
                task_asset_rows=assets,
                completion_events=events,
                max_task_asset_id=25000,
                max_asset_id=18000,
                run_id="v1295-auto-bundles",
                confirmed_by=1,
                confirmed_at="2026-08-04T11:15:30Z",
            )


if __name__ == "__main__":
    unittest.main()
