import hashlib
import json
import pathlib
import tempfile
import unittest
from unittest import mock

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
            run_root, source_root, b_root, manifest_path, manifest = self.fixture(root)
            prepared = materializer.validate_manifest(manifest, source_root)
            registry_path = run_root / "registry.json"
            write_ahead_path = run_root / "write-ahead.json"
            staging_write_ahead_path = run_root / "staging-write-ahead.json"
            first = materializer.materialize(
                manifest,
                manifest_path,
                prepared,
                b_root.resolve(),
                registry_path,
                write_ahead_path,
                staging_write_ahead_path,
            )
            second = materializer.materialize(
                manifest,
                manifest_path,
                prepared,
                b_root.resolve(),
                registry_path,
                write_ahead_path,
                staging_write_ahead_path,
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
            run_root, source_root, b_root, manifest_path, manifest = self.fixture(root)
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
                run_root / "registry.json",
                run_root / "write-ahead.json",
                run_root / "staging-write-ahead.json",
            )
            entry = registry["entries"][0]
            target = b_root / entry["relative_object_path"]
            target.write_bytes(b"tampered")
            with self.assertRaisesRegex(ValueError, "drifted"):
                materializer.rollback(registry, b_root.resolve())
            self.assertTrue(target.exists())

    def test_write_ahead_registry_rolls_back_partial_target_without_final_registry(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            run_root, source_root, b_root, manifest_path, manifest = self.fixture(root)
            prepared = materializer.validate_manifest(manifest, source_root)
            registry_path = run_root / "registry.json"
            write_ahead_path = run_root / "write-ahead.json"
            staging_write_ahead_path = run_root / "staging-write-ahead.json"
            registry = materializer.materialize(
                manifest,
                manifest_path,
                prepared,
                b_root.resolve(),
                registry_path,
                write_ahead_path,
                staging_write_ahead_path,
            )
            target = b_root / registry["entries"][0]["relative_object_path"]
            self.assertTrue(target.is_file())
            registry_path.unlink()
            write_ahead = json.loads(
                write_ahead_path.read_text(encoding="utf-8")
            )
            result = materializer.rollback(write_ahead, b_root.resolve())
            self.assertFalse(target.exists())
            self.assertEqual(result["status"], "ROLLED_BACK")

    def test_staging_write_ahead_preserves_unowned_racing_file(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            run_root, _, b_root, manifest_path, manifest = self.fixture(root)
            stage = run_root / ".bundle-stage-301.zip"
            receipt = run_root / "bundle-staging-ownership-301.json"
            seed = materializer.self_bound(
                {
                    "schema_version": 1,
                    "status": "STAGING_WRITE_AHEAD",
                    "run_id": manifest["run_id"],
                    "manifest_sha256": materializer.sha256_file(manifest_path),
                    "b_root": str(b_root.resolve()),
                    "database_write_performed": False,
                    "stage_specs": [
                        {
                            "path": str(stage.resolve()),
                            "ownership_receipt_path": str(receipt.resolve()),
                            "object_key": (
                                "fixture/run-1/migration-bundles/task-7/"
                                "sku-70/revision-1/source-bundle.zip"
                            ),
                        }
                    ],
                }
            )
            stage.write_bytes(b"partial staging bytes")
            with self.assertRaisesRegex(
                ValueError, "staging ownership cannot be proven"
            ):
                materializer.rollback(seed, b_root.resolve())
            self.assertEqual(stage.read_bytes(), b"partial staging bytes")

    def test_staging_write_ahead_removes_only_receipted_inode(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            run_root, _, b_root, manifest_path, manifest = self.fixture(root)
            stage = run_root / ".bundle-stage-301.zip"
            receipt = run_root / "bundle-staging-ownership-301.json"
            stage.write_bytes(b"owned staging bytes")
            materializer.atomic_write(
                receipt,
                materializer.staging_receipt(manifest["run_id"], stage),
            )
            seed = materializer.self_bound(
                {
                    "schema_version": 1,
                    "status": "STAGING_WRITE_AHEAD",
                    "run_id": manifest["run_id"],
                    "manifest_sha256": materializer.sha256_file(manifest_path),
                    "b_root": str(b_root.resolve()),
                    "database_write_performed": False,
                    "stage_specs": [
                        {
                            "path": str(stage.resolve()),
                            "ownership_receipt_path": str(receipt.resolve()),
                            "object_key": (
                                "fixture/run-1/migration-bundles/task-7/"
                                "sku-70/revision-1/source-bundle.zip"
                            ),
                        }
                    ],
                }
            )
            result = materializer.rollback(seed, b_root.resolve())
            self.assertFalse(stage.exists())
            self.assertEqual(result["removed_object_paths"], [])

    def test_staging_write_ahead_removes_receipted_private_before_publish(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            run_root, _, b_root, manifest_path, manifest = self.fixture(root)
            stage = run_root / ".bundle-stage-301.zip"
            private = run_root / ".bundle-stage-301.zip.private"
            receipt = run_root / "bundle-staging-ownership-301.json"
            private.write_bytes(b"private bundle bytes")
            materializer.atomic_write(
                receipt,
                materializer.staging_receipt(
                    manifest["run_id"],
                    stage,
                    private,
                    private,
                ),
            )
            seed = materializer.self_bound(
                {
                    "schema_version": 1,
                    "status": "STAGING_WRITE_AHEAD",
                    "run_id": manifest["run_id"],
                    "manifest_sha256": materializer.sha256_file(manifest_path),
                    "b_root": str(b_root.resolve()),
                    "database_write_performed": False,
                    "stage_specs": [
                        {
                            "path": str(stage.resolve()),
                            "ownership_receipt_path": str(receipt.resolve()),
                            "object_key": (
                                "fixture/run-1/migration-bundles/task-7/"
                                "sku-70/revision-1/source-bundle.zip"
                            ),
                        }
                    ],
                }
            )
            result = materializer.rollback(seed, b_root.resolve())
            self.assertFalse(private.exists())
            self.assertEqual(
                result["removed_private_staging_paths"],
                [str(private)],
            )

    @unittest.skipIf(
        materializer.os.name == "nt",
        "xattr reservation is Linux-only",
    )
    def test_staging_write_ahead_removes_reserved_private_before_receipt(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            run_root, _, b_root, manifest_path, manifest = self.fixture(root)
            stage = run_root / ".bundle-stage-301.zip"
            private = run_root / ".bundle-private-301.zip"
            receipt = run_root / "bundle-staging-ownership-301.json"
            token = hashlib.sha256(b"reserved-bundle-private").hexdigest()
            private.write_bytes(b"sensitive bundle bytes")
            materializer.os.setxattr(
                private,
                materializer.STAGE_TOKEN_XATTR,
                token.encode("ascii"),
            )
            seed = materializer.self_bound(
                {
                    "schema_version": 1,
                    "status": "STAGING_WRITE_AHEAD",
                    "run_id": manifest["run_id"],
                    "manifest_sha256": materializer.sha256_file(manifest_path),
                    "b_root": str(b_root.resolve()),
                    "database_write_performed": False,
                    "stage_specs": [
                        {
                            "path": str(stage.resolve()),
                            "private_path": str(private.resolve()),
                            "ownership_token": token,
                            "ownership_receipt_path": str(receipt.resolve()),
                            "object_key": (
                                "fixture/run-1/migration-bundles/task-7/"
                                "sku-70/revision-1/source-bundle.zip"
                            ),
                        }
                    ],
                }
            )
            result = materializer.rollback(seed, b_root.resolve())
            self.assertFalse(private.exists())
            self.assertIn(
                str(private),
                result["removed_private_staging_paths"],
            )

    def test_staging_write_ahead_preserves_same_bytes_replacement(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            run_root, _, b_root, manifest_path, manifest = self.fixture(root)
            stage = run_root / ".bundle-stage-301.zip"
            receipt = run_root / "bundle-staging-ownership-301.json"
            stage.write_bytes(b"same bytes")
            materializer.atomic_write(
                receipt,
                materializer.staging_receipt(manifest["run_id"], stage),
            )
            replacement = run_root / "replacement.zip"
            replacement.write_bytes(b"same bytes")
            materializer.os.replace(replacement, stage)
            seed = materializer.self_bound(
                {
                    "schema_version": 1,
                    "status": "STAGING_WRITE_AHEAD",
                    "run_id": manifest["run_id"],
                    "manifest_sha256": materializer.sha256_file(manifest_path),
                    "b_root": str(b_root.resolve()),
                    "database_write_performed": False,
                    "stage_specs": [
                        {
                            "path": str(stage.resolve()),
                            "ownership_receipt_path": str(receipt.resolve()),
                            "object_key": (
                                "fixture/run-1/migration-bundles/task-7/"
                                "sku-70/revision-1/source-bundle.zip"
                            ),
                        }
                    ],
                }
            )
            with self.assertRaisesRegex(
                ValueError, "staging ownership cannot be proven"
            ):
                materializer.rollback(seed, b_root.resolve())
            self.assertEqual(stage.read_bytes(), b"same bytes")

    def test_tampered_write_ahead_is_rejected_before_cleanup(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            run_root, source_root, b_root, manifest_path, manifest = self.fixture(root)
            prepared = materializer.validate_manifest(manifest, source_root)
            registry_path = run_root / "registry.json"
            write_ahead_path = run_root / "write-ahead.json"
            materializer.materialize(
                manifest,
                manifest_path,
                prepared,
                b_root.resolve(),
                registry_path,
                write_ahead_path,
                run_root / "staging-write-ahead.json",
            )
            seed = json.loads(write_ahead_path.read_text(encoding="utf-8"))
            seed["entries"] = []
            with self.assertRaisesRegex(ValueError, "self hash"):
                materializer.rollback(seed, b_root.resolve())

    def test_atomic_promote_never_overwrites_a_racing_target(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            run_root, source_root, b_root, manifest_path, manifest = self.fixture(root)
            prepared = materializer.validate_manifest(manifest, source_root)
            real_link = materializer.os.link
            raced_target = None

            def inject_target(source, target):
                nonlocal raced_target
                target_path = pathlib.Path(target)
                if (b_root / "objects").resolve() not in target_path.parents:
                    return real_link(source, target)
                raced_target = target_path
                raced_target.write_bytes(b"concurrent-unowned-target")
                return real_link(source, target)

            with mock.patch.object(
                materializer.os, "link", side_effect=inject_target
            ):
                with self.assertRaisesRegex(
                    RuntimeError, "exact compensation could not complete"
                ):
                    materializer.materialize(
                        manifest,
                        manifest_path,
                        prepared,
                        b_root.resolve(),
                        run_root / "registry.json",
                        run_root / "write-ahead.json",
                        run_root / "staging-write-ahead.json",
                    )
            self.assertIsNotNone(raced_target)
            self.assertEqual(
                raced_target.read_bytes(), b"concurrent-unowned-target"
            )

    def test_rollback_never_deletes_an_identical_racing_target(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            run_root, source_root, b_root, manifest_path, manifest = self.fixture(root)
            prepared = materializer.validate_manifest(manifest, source_root)
            real_link = materializer.os.link
            raced_target = None

            def inject_identical_target(source, target):
                nonlocal raced_target
                target_path = pathlib.Path(target)
                if (b_root / "objects").resolve() not in target_path.parents:
                    return real_link(source, target)
                raced_target = target_path
                raced_target.write_bytes(pathlib.Path(source).read_bytes())
                return real_link(source, target)

            write_ahead = run_root / "write-ahead.json"
            with mock.patch.object(
                materializer.os, "link", side_effect=inject_identical_target
            ):
                with self.assertRaisesRegex(
                    RuntimeError, "exact compensation could not complete"
                ):
                    materializer.materialize(
                        manifest,
                        manifest_path,
                        prepared,
                        b_root.resolve(),
                        run_root / "registry.json",
                        write_ahead,
                        run_root / "staging-write-ahead.json",
                    )
            seed = json.loads(write_ahead.read_text(encoding="utf-8"))
            with self.assertRaisesRegex(ValueError, "ownership cannot be proven"):
                materializer.rollback(seed, b_root.resolve())
            self.assertIsNotNone(raced_target)
            self.assertTrue(raced_target.exists())

    def test_unowned_hardlinked_stage_never_authorizes_target_delete(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            run_root, source_root, b_root, manifest_path, manifest = self.fixture(root)
            prepared = materializer.validate_manifest(manifest, source_root)
            write_ahead = run_root / "write-ahead.json"
            registry = materializer.materialize(
                manifest,
                manifest_path,
                prepared,
                b_root.resolve(),
                run_root / "registry.json",
                write_ahead,
                run_root / "staging-write-ahead.json",
            )
            entry = registry["entries"][0]
            target = b_root / entry["relative_object_path"]
            replacement = target.with_name("replacement.zip")
            replacement.write_bytes(target.read_bytes())
            materializer.os.replace(replacement, target)
            seed = json.loads(write_ahead.read_text(encoding="utf-8"))
            stage_item = seed["staging_files"][0]
            stage = pathlib.Path(stage_item["path"])
            stage.hardlink_to(target)
            with self.assertRaisesRegex(
                ValueError, "staging ownership cannot be proven"
            ):
                materializer.rollback(seed, b_root.resolve())
            self.assertTrue(target.exists())

    def test_internal_compensation_never_deletes_replaced_owned_target(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            run_root, source_root, b_root, manifest_path, manifest = self.fixture(root)
            second = json.loads(json.dumps(manifest["bundles"][0]))
            second["task_id"] = 8
            second["scope_ref_id"] = 80
            second["bundle_task_asset_id"] = 302
            second["bundle_asset_id"] = 402
            second["bundle_storage_ref_id"] = "ref-302"
            for index, member in enumerate(second["ordered_members"]):
                member["task_asset_id"] = 102 + index
                member["asset_id"] = 202 + index
                member["task_id"] = 8
                member["storage_ref_id"] = f"member-ref-{102 + index}"
                member["object_key"] = f"sources/two-{index}.bin"
                second_source = source_root / member["object_key"]
                second_source.parent.mkdir(parents=True, exist_ok=True)
                second_source.write_bytes(f"second-{index}".encode())
                member["size"] = second_source.stat().st_size
                member["sha256"] = hashlib.sha256(
                    second_source.read_bytes()
                ).hexdigest()
            manifest["bundles"].append(second)
            manifest_path.write_text(
                json.dumps(manifest), encoding="utf-8"
            )
            prepared = materializer.validate_manifest(manifest, source_root)
            real_link = materializer.os.link
            first_target = None
            link_count = 0

            def replace_first_then_fail(source, target):
                nonlocal first_target, link_count
                target_path = pathlib.Path(target)
                if (b_root / "objects").resolve() not in target_path.parents:
                    return real_link(source, target)
                link_count += 1
                if link_count == 1:
                    first_target = target_path
                    return real_link(source, target)
                first_target.unlink()
                first_target.write_bytes(b"concurrent-replacement")
                raise FileExistsError("injected second promote failure")

            with mock.patch.object(
                materializer.os,
                "link",
                side_effect=replace_first_then_fail,
            ):
                with self.assertRaisesRegex(
                    RuntimeError, "exact compensation could not complete"
                ):
                    materializer.materialize(
                        manifest,
                        manifest_path,
                        prepared,
                        b_root.resolve(),
                        run_root / "registry.json",
                        run_root / "write-ahead.json",
                        run_root / "staging-write-ahead.json",
                    )
            self.assertIsNotNone(first_target)
            self.assertEqual(
                first_target.read_bytes(), b"concurrent-replacement"
            )

    def test_rollback_preserves_reused_identical_object(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            run_root, source_root, b_root, manifest_path, manifest = self.fixture(root)
            prepared = materializer.validate_manifest(manifest, source_root)
            created_registry = materializer.materialize(
                manifest,
                manifest_path,
                prepared,
                b_root.resolve(),
                run_root / "created-registry.json",
                run_root / "created-write-ahead.json",
                run_root / "created-staging-write-ahead.json",
            )
            target = b_root / created_registry["entries"][0][
                "relative_object_path"
            ]
            reused_registry = materializer.materialize(
                manifest,
                manifest_path,
                prepared,
                b_root.resolve(),
                run_root / "reused" / "registry.json",
                run_root / "reused" / "write-ahead.json",
                run_root / "reused" / "staging-write-ahead.json",
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
