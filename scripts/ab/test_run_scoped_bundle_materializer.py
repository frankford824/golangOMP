import hashlib
import json
import pathlib
import tempfile
import unittest

import run_scoped_bundle_materializer as materializer


class RunScopedBundleMaterializerTest(unittest.TestCase):
    def fixture(self, root: pathlib.Path):
        run_root = root / "run-1"
        source_root = run_root / "frozen-upload-seed-b"
        b_root = run_root / "fixture-upload-b"
        source_root.mkdir(parents=True)
        b_root.mkdir()
        members = []
        for task_asset_id, name, content in (
            (31, "first.psd", b"first"),
            (32, "second.ai", b"second"),
        ):
            object_key = f"legacy/{task_asset_id}/{name}"
            path = source_root / object_key
            path.parent.mkdir(parents=True)
            path.write_bytes(content)
            members.append(
                {
                    "task_asset_id": task_asset_id,
                    "asset_id": 100 + task_asset_id,
                    "task_id": 7,
                    "storage_ref_id": f"legacy-ref-{task_asset_id}",
                    "original_file_name": name,
                    "object_key": object_key,
                    "size": len(content),
                    "sha256": hashlib.sha256(content).hexdigest(),
                    "source_stage": "design",
                    "evidence_event_ids": [
                        f"task_event_log:event-{task_asset_id}"
                    ],
                    "confirmed": True,
                }
            )
        manifest = {
            "schema_version": 1,
            "status": "CONFIRMED",
            "run_id": "run-1",
            "source_candidate_sha256": "a" * 64,
            "confirmed_by": 1,
            "confirmed_at": "2026-07-23T00:00:00Z",
            "confirmation_note": "administrator confirmed exact ordered members",
            "bundles": [
                {
                    "task_id": 7,
                    "scope_kind": "sku",
                    "scope_ref_id": 70,
                    "revision_no": 1,
                    "bundle_task_asset_id": 9001,
                    "bundle_asset_id": 9002,
                    "bundle_storage_ref_id": "bundle-ref-7-70-1",
                    "confirmed": True,
                    "ordered_members": members,
                }
            ],
        }
        manifest_path = run_root / "confirmed.json"
        manifest_path.write_text(json.dumps(manifest), encoding="utf-8")
        return run_root, source_root, b_root, manifest_path, manifest

    def test_prepare_is_read_only_and_unconfirmed_fails_closed(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            run_root, source_root, b_root, manifest_path, manifest = self.fixture(
                root
            )
            prepared = materializer.validate_manifest(manifest, source_root)
            report = materializer.prepare_document(
                manifest_path, source_root, prepared
            )
            self.assertEqual(report["status"], "PREPARED")
            self.assertFalse(report["object_write_performed"])
            self.assertFalse((b_root / "objects").exists())

            manifest["status"] = "PROPOSED_REVIEW"
            with self.assertRaisesRegex(ValueError, "CONFIRMED"):
                materializer.validate_manifest(manifest, source_root)
            self.assertEqual(
                materializer.exact_b_root(run_root.resolve(), b_root),
                b_root.resolve(),
            )

    def test_materialize_is_deterministic_idempotent_and_rollback_is_exact(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            _, source_root, b_root, manifest_path, manifest = self.fixture(root)
            prepared = materializer.validate_manifest(manifest, source_root)
            registry_path = root / "registry.json"
            first = materializer.materialize(
                manifest, manifest_path, prepared, b_root.resolve(), registry_path
            )
            second = materializer.materialize(
                manifest, manifest_path, prepared, b_root.resolve(), registry_path
            )
            self.assertEqual(first, second)
            self.assertFalse(first["database_write_performed"])
            entry = first["entries"][0]
            target = b_root / entry["relative_object_path"]
            self.assertTrue(target.is_file())
            self.assertEqual(
                hashlib.sha256(target.read_bytes()).hexdigest(),
                entry["bundle_sha256"],
            )
            rolled_back = materializer.rollback(first, b_root.resolve())
            self.assertEqual(rolled_back["status"], "ROLLED_BACK")
            self.assertFalse(target.exists())
            repeated = materializer.rollback(first, b_root.resolve())
            self.assertEqual(repeated["already_absent_count"], 1)
            self.assertFalse(repeated["database_write_performed"])

    def test_hash_drift_and_output_drift_fail_before_cleanup(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            _, source_root, b_root, manifest_path, manifest = self.fixture(root)
            source = source_root / manifest["bundles"][0]["ordered_members"][0][
                "object_key"
            ]
            source.write_bytes(b"drift")
            with self.assertRaisesRegex(ValueError, "drifted"):
                materializer.validate_manifest(manifest, source_root)

            source.write_bytes(b"first")
            prepared = materializer.validate_manifest(manifest, source_root)
            registry = materializer.materialize(
                manifest,
                manifest_path,
                prepared,
                b_root.resolve(),
                root / "registry.json",
            )
            entry = registry["entries"][0]
            target = b_root / entry["relative_object_path"]
            target.write_bytes(b"tampered")
            with self.assertRaisesRegex(ValueError, "drifted"):
                materializer.rollback(registry, b_root.resolve())
            self.assertTrue(target.exists())

    def test_rollback_preserves_reused_identical_object(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            _, source_root, b_root, manifest_path, manifest = self.fixture(root)
            prepared = materializer.validate_manifest(manifest, source_root)
            created_registry = materializer.materialize(
                manifest,
                manifest_path,
                prepared,
                b_root.resolve(),
                root / "created-registry.json",
            )
            target = b_root / created_registry["entries"][0][
                "relative_object_path"
            ]
            reused_registry = materializer.materialize(
                manifest,
                manifest_path,
                prepared,
                b_root.resolve(),
                root / "reused-registry.json",
            )
            self.assertEqual(
                reused_registry["entries"][0]["disposition"],
                "reused_identical",
            )
            rolled_back = materializer.rollback(
                reused_registry, b_root.resolve()
            )
            self.assertTrue(target.exists())
            self.assertEqual(rolled_back["removed_object_paths"], [])
            self.assertEqual(
                rolled_back["retained_reused_object_paths"],
                [reused_registry["entries"][0]["relative_object_path"]],
            )
            target.unlink()
            with self.assertRaisesRegex(ValueError, "reused bundle is missing"):
                materializer.rollback(reused_registry, b_root.resolve())


if __name__ == "__main__":
    unittest.main()
