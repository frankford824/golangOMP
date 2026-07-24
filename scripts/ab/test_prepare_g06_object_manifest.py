from __future__ import annotations

import hashlib
import json
import pathlib
import tempfile
import unittest
from unittest import mock

from scripts.ab import apply_bundle_registry_to_mapping as bundle_validator
from scripts.ab import prepare_g06_object_manifest as module


def digest(path: pathlib.Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


class PrepareG06ObjectManifestTests(unittest.TestCase):
    def row(
        self,
        owner_id: int,
        *,
        sha256: str = "",
        storage_ref: str | None = None,
        object_key: str | None = None,
    ) -> dict:
        return {
            "entity_key": f"task_asset:{owner_id}",
            "owner_kind": "task_asset",
            "owner_id": owner_id,
            "task_id": 7,
            "storage_ref_id": storage_ref or f"ref-{owner_id}",
            "storage_adapter": "oss_upload_service",
            "object_key": object_key or f"tasks/7/{owner_id}.bin",
            "size": 3,
            "mime_type": "application/octet-stream",
            "sha256": sha256,
            "status": "recorded",
            "is_placeholder": False,
        }

    def write_jsonl(
        self, root: pathlib.Path, name: str, rows: list[dict]
    ) -> pathlib.Path:
        path = root / name
        path.write_text(
            "".join(module.canonical_json(row) + "\n" for row in rows),
            encoding="utf-8",
        )
        return path

    def write_json(
        self, root: pathlib.Path, name: str, value: dict
    ) -> pathlib.Path:
        path = root / name
        path.write_text(module.canonical_json(value) + "\n", encoding="utf-8")
        return path

    def test_prepare_removes_only_exact_tombstone_and_sorts(self):
        complete_sha = hashlib.sha256(b"complete").hexdigest()
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            source = self.write_jsonl(
                root,
                "source.jsonl",
                [
                    self.row(20, sha256=""),
                    dict(module.TOMBSTONE_CONTRACT),
                    self.row(3, sha256=complete_sha),
                ],
            )
            output, summary = module.prepare_manifest(source, digest(source))
        rows = [json.loads(line) for line in output.decode().splitlines()]
        self.assertEqual(
            [row["entity_key"] for row in rows],
            ["task_asset:3", "task_asset:20"],
        )
        self.assertEqual(summary["excluded_row_count"], 1)
        self.assertEqual(summary["hydration_row_count"], 2)
        self.assertEqual(
            summary["hydration_manifest_sha256"], hashlib.sha256(output).hexdigest()
        )

    def test_prepare_rejects_tombstone_drift_and_duplicate_entity(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            drifted = dict(module.TOMBSTONE_CONTRACT, size=1)
            source = self.write_jsonl(
                root, "drifted.jsonl", [drifted, self.row(2)]
            )
            with self.assertRaisesRegex(ValueError, "one exact"):
                module.prepare_manifest(source, digest(source))
            duplicate = self.write_jsonl(
                root,
                "duplicate.jsonl",
                [
                    dict(module.TOMBSTONE_CONTRACT),
                    self.row(2),
                    self.row(2),
                ],
            )
            with self.assertRaisesRegex(ValueError, "duplicate entity_key"):
                module.prepare_manifest(duplicate, digest(duplicate))

    def test_hydrated_rows_may_fill_missing_metadata_but_not_change_identity(self):
        source_rows = [
            dict(module.TOMBSTONE_CONTRACT),
            self.row(2, sha256=""),
            self.row(3, sha256=hashlib.sha256(b"old").hexdigest()),
        ]
        hydrated = [
            dict(
                self.row(2),
                size=5,
                mime_type="image/png",
                sha256=hashlib.sha256(b"new").hexdigest(),
            ),
            self.row(3, sha256=hashlib.sha256(b"old").hexdigest()),
        ]
        verified = module.verify_hydrated_rows(source_rows, hydrated)
        self.assertEqual(len(verified), 2)
        changed_identity = [dict(row) for row in hydrated]
        changed_identity[0]["storage_ref_id"] = "different"
        with self.assertRaisesRegex(ValueError, "immutable field"):
            module.verify_hydrated_rows(source_rows, changed_identity)
        changed_complete = [dict(row) for row in hydrated]
        changed_complete[1]["size"] = 99
        with self.assertRaisesRegex(ValueError, "complete field"):
            module.verify_hydrated_rows(source_rows, changed_complete)

    def bundle_fixture(self, root: pathlib.Path):
        mapping = self.write_json(root, "mapping.json", {"version": 2, "resources": []})
        manifest = self.write_json(root, "manifest.json", {"status": "CONFIRMED"})
        entries = []
        for index, key in enumerate(sorted(bundle_validator.EXACT_SCOPES), 1):
            task_id, scope_kind, scope_ref_id, revision_no = key
            bundle_sha = hashlib.sha256(f"bundle-{index}".encode()).hexdigest()
            object_key = (
                f"fixture/run/migration-bundles/task-{task_id}/"
                f"{scope_kind}-{scope_ref_id}/revision-{revision_no}/"
                "source-bundle.zip"
            )
            entries.append(
                {
                    "task_id": task_id,
                    "scope_kind": scope_kind,
                    "scope_ref_id": scope_ref_id,
                    "revision_no": revision_no,
                    "object_key": object_key,
                    "size": 100 + index,
                    "bundle_sha256": bundle_sha,
                    "task_asset_candidate": {
                        "id": 30000 + index,
                        "task_id": task_id,
                    },
                    "asset_storage_ref_candidate": {
                        "ref_id": f"bundle-ref-{index}",
                    },
                }
            )
        registry = self.write_json(root, "registry.json", {"entries": entries})
        return mapping, manifest, registry

    def test_finalize_restores_tombstone_and_adds_seven_bundle_rows(self):
        filled_sha = hashlib.sha256(b"filled").hexdigest()
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            source = self.write_jsonl(
                root,
                "source.jsonl",
                [dict(module.TOMBSTONE_CONTRACT), self.row(2, sha256="")],
            )
            hydrated = self.write_jsonl(
                root, "hydrated.jsonl", [self.row(2, sha256=filled_sha)]
            )
            mapping, manifest, registry = self.bundle_fixture(root)
            scopes = {
                key: {"validated": True}
                for key in bundle_validator.EXACT_SCOPES
            }
            with (
                mock.patch.object(
                    module.bundle_validator,
                    "validate_manifest",
                    return_value=(scopes, "run"),
                ),
                mock.patch.object(
                    module.bundle_validator,
                    "validate_registry",
                    return_value=scopes,
                ),
            ):
                output, summary = module.finalize_manifest(
                    source_path=source,
                    expected_source_sha256=digest(source),
                    hydrated_path=hydrated,
                    expected_hydrated_sha256=digest(hydrated),
                    mapping_path=mapping,
                    expected_mapping_sha256=digest(mapping),
                    bundle_manifest_path=manifest,
                    expected_bundle_manifest_sha256=digest(manifest),
                    registry_path=registry,
                    expected_registry_sha256=digest(registry),
                )
        rows = [json.loads(line) for line in output.decode().splitlines()]
        self.assertEqual(len(rows), 9)
        self.assertEqual(
            next(row for row in rows if row["entity_key"] == module.TOMBSTONE_ENTITY),
            module.TOMBSTONE_CONTRACT,
        )
        bundles = [row for row in rows if row["storage_adapter"] == "clone_b_bundle"]
        self.assertEqual(len(bundles), 7)
        self.assertEqual(summary["bundle_row_count"], 7)
        self.assertEqual(summary["final_row_count"], 9)

    def test_bundle_collisions_are_rejected_but_historical_sharing_is_preserved(self):
        existing = [
            self.row(1, storage_ref="shared", object_key="tasks/shared.bin"),
            self.row(2, storage_ref="shared", object_key="tasks/shared.bin"),
        ]
        addition = self.row(
            3, storage_ref="bundle-ref", object_key="tasks/bundle.zip"
        )
        module.ensure_new_bundle_identifiers(existing, [addition])
        with self.assertRaisesRegex(ValueError, "storage_ref_id"):
            module.ensure_new_bundle_identifiers(
                existing,
                [self.row(3, storage_ref="shared", object_key="tasks/bundle.zip")],
            )
        with self.assertRaisesRegex(ValueError, "object_key"):
            module.ensure_new_bundle_identifiers(
                existing,
                [self.row(3, storage_ref="new", object_key="tasks/shared.bin")],
            )

    def test_atomic_outputs_refuse_different_overwrite(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = pathlib.Path(temporary) / "out.jsonl"
            module.atomic_write_many([(path, b"one\n")])
            module.atomic_write_many([(path, b"one\n")])
            with self.assertRaises(FileExistsError):
                module.atomic_write_many([(path, b"two\n")])


if __name__ == "__main__":
    unittest.main()
