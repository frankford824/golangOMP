from __future__ import annotations

import hashlib
import json
import pathlib
import tempfile
import unittest
from unittest import mock

from scripts.ab import apply_bundle_registry_to_mapping as bundle_validator
from scripts.ab import g06_recovery_contract as recovery_contract
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

    def recovery_fixture(self, root: pathlib.Path):
        recoveries = []
        entries = []
        original_rows = []
        for missing_id in recovery_contract.RECOVERY_IDS:
            source_id, size = recovery_contract.REQUIRED_SOURCES[missing_id]
            source_sha = hashlib.sha256(
                f"source-{missing_id}".encode()
            ).hexdigest()
            original_ref = f"original-ref-{missing_id}"
            original_key = f"tasks/2807/original-{missing_id}.jpg"
            target_ref = f"target-ref-{missing_id}"
            target_key = f"fixture/recovered/{missing_id}.jpg"
            request_id = f"request-{missing_id}"
            mapping_row = {
                "task_id": recovery_contract.TASK_ID,
                "missing_task_asset_id": missing_id,
                "recovery_source_task_asset_id": source_id,
                "expected_file_size": size,
                "strategy": recovery_contract.STRATEGY,
                "review_policy_ids": [recovery_contract.POLICY],
                "confidence": "confirmed_auto",
                "confirmed_by": 1,
                "confirmed_at": "2026-07-24T00:00:00Z",
                "confirmation_note": "approved fixture",
                "blockers": [],
                "original_storage_ref_id": original_ref,
                "recovery_source_sha256": source_sha,
                "recovery_source_storage_ref_id": f"source-ref-{missing_id}",
                "controlled_read_protocol": "controlled-asset-read-v1",
                "controlled_read_evidence_sha256": hashlib.sha256(
                    b"controlled-read"
                ).hexdigest(),
            }
            mapping_row["manifest_row_hash"] = recovery_contract.canonical_hash(
                mapping_row
            )
            recoveries.append(mapping_row)
            storage = {
                "ref_id": target_ref,
                "ref_key": target_key,
                "owner_type": "task_asset",
                "owner_id": missing_id,
                "file_size": size,
                "mime_type": "image/jpeg",
                "checksum_hint": source_sha,
                "status": "recorded",
                "is_placeholder": 0,
            }
            entries.append(
                {
                    "missing_task_asset_id": missing_id,
                    "source_task_asset_id": source_id,
                    "source_size": size,
                    "source_sha256": source_sha,
                    "target_storage_ref_id": target_ref,
                    "target_object_key": target_key,
                    "db_apply_plan": {
                        "insert_asset_storage_ref": storage,
                        "update_task_asset": {
                            "where": {"id": missing_id},
                            "set": {
                                "storage_ref_id": target_ref,
                                "storage_key": target_key,
                                "whole_hash": source_sha,
                            },
                        },
                        "update_upload_request": {
                            "where": {"request_id": request_id},
                            "set": {
                                "bound_ref_id": target_ref,
                                "checksum_hint": source_sha,
                                "file_size": size,
                                "status": "bound",
                                "session_status": "completed",
                            },
                        },
                    },
                    "rollback_registry": {
                        "original_storage_ref": {
                            "ref_id": original_ref,
                            "ref_key": original_key,
                            "storage_adapter": "oss_upload_service",
                            "file_size": size,
                            "mime_type": "image/jpeg",
                            "status": "recorded",
                            "is_placeholder": 0,
                        },
                        "restore_task_asset": {
                            "id": missing_id,
                            "task_id": recovery_contract.TASK_ID,
                            "file_size": size,
                            "storage_ref_id": original_ref,
                            "upload_request_id": request_id,
                        },
                        "restore_upload_request": {
                            "request_id": request_id,
                        },
                    },
                }
            )
            original_rows.append(
                {
                    "entity_key": f"task_asset:{missing_id}",
                    "owner_kind": "task_asset",
                    "owner_id": missing_id,
                    "task_id": recovery_contract.TASK_ID,
                    "storage_ref_id": original_ref,
                    "storage_adapter": "oss_upload_service",
                    "object_key": original_key,
                    "size": size,
                    "mime_type": "image/jpeg",
                    "sha256": "",
                    "status": "recorded",
                    "is_placeholder": False,
                }
            )
        mapping = self.write_json(
            root,
            "recovery-mapping.json",
            {"version": 2, "asset_recoveries": recoveries, "resources": []},
        )
        mapping_sha = digest(mapping)
        for entry in entries:
            missing_id = entry["missing_task_asset_id"]
            source_sha = entry["source_sha256"]
            target_ref = str(
                __import__("uuid").uuid5(
                    recovery_contract.RECOVERY_NAMESPACE,
                    (
                        f"fixture-run:{mapping_sha}:"
                        f"{missing_id}:{source_sha}"
                    ),
                )
            )
            target_key = (
                f"v8-ab/fixture-run/recovered/task-2807/"
                f"task-asset-{missing_id}/{source_sha}.bin"
            )
            entry["target_storage_ref_id"] = target_ref
            entry["target_object_key"] = target_key
            storage = entry["db_apply_plan"]["insert_asset_storage_ref"]
            storage["ref_id"] = target_ref
            storage["ref_key"] = target_key
            update = entry["db_apply_plan"]["update_task_asset"]["set"]
            update["storage_ref_id"] = target_ref
            update["storage_key"] = target_key
            entry["db_apply_plan"]["update_upload_request"]["set"][
                "bound_ref_id"
            ] = target_ref
        plan = self.write_json(
            root,
            "recovery-plan.json",
            {
                "version": 1,
                "status": "MATERIALIZED",
                "run_id": "fixture-run",
                "mapping_sha256": mapping_sha,
                "database_writes_executed": False,
                "production_writes_executed": False,
                "entries": entries,
            },
        )
        return mapping, plan, original_rows

    def test_prepare_removes_only_exact_tombstone_and_sorts(self):
        complete_sha = hashlib.sha256(b"complete").hexdigest()
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            mapping, plan, recoveries = self.recovery_fixture(root)
            source = self.write_jsonl(
                root,
                "source.jsonl",
                [
                    self.row(20, sha256=""),
                    dict(module.TOMBSTONE_CONTRACT),
                    *recoveries,
                    self.row(3, sha256=complete_sha),
                ],
            )
            output, summary = module.prepare_manifest(
                source,
                digest(source),
                mapping,
                digest(mapping),
                plan,
                digest(plan),
            )
        rows = [json.loads(line) for line in output.decode().splitlines()]
        self.assertEqual(
            [row["entity_key"] for row in rows],
            ["task_asset:3", "task_asset:20"],
        )
        self.assertEqual(summary["excluded_row_count"], 4)
        self.assertEqual(summary["hydration_row_count"], 2)
        self.assertEqual(
            summary["hydration_manifest_sha256"], hashlib.sha256(output).hexdigest()
        )

    def test_prepare_rejects_tombstone_drift_and_duplicate_entity(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            mapping, plan, recoveries = self.recovery_fixture(root)
            drifted = dict(module.TOMBSTONE_CONTRACT, size=1)
            source = self.write_jsonl(
                root, "drifted.jsonl", [drifted, *recoveries, self.row(2)]
            )
            with self.assertRaisesRegex(ValueError, "one exact"):
                module.prepare_manifest(
                    source, digest(source), mapping, digest(mapping),
                    plan, digest(plan)
                )
            duplicate = self.write_jsonl(
                root,
                "duplicate.jsonl",
                [
                    dict(module.TOMBSTONE_CONTRACT),
                    *recoveries,
                    self.row(2),
                    self.row(2),
                ],
            )
            with self.assertRaisesRegex(ValueError, "duplicate entity_key"):
                module.prepare_manifest(
                    duplicate, digest(duplicate), mapping, digest(mapping),
                    plan, digest(plan)
                )

    def test_hydrated_rows_may_fill_missing_metadata_but_not_change_identity(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            _mapping, _plan, recoveries = self.recovery_fixture(root)
            source_rows = [
                dict(module.TOMBSTONE_CONTRACT),
                *recoveries,
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
            verified = module.verify_hydrated_rows(
                source_rows, hydrated, recoveries
            )
            self.assertEqual(len(verified), 2)
            changed_identity = [dict(row) for row in hydrated]
            changed_identity[0]["storage_ref_id"] = "different"
            with self.assertRaisesRegex(ValueError, "immutable field"):
                module.verify_hydrated_rows(
                    source_rows, changed_identity, recoveries
                )
            changed_complete = [dict(row) for row in hydrated]
            changed_complete[1]["size"] = 99
            with self.assertRaisesRegex(ValueError, "complete field"):
                module.verify_hydrated_rows(
                    source_rows, changed_complete, recoveries
                )

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
            recovery_mapping, recovery_plan, recoveries = self.recovery_fixture(root)
            source = self.write_jsonl(
                root,
                "source.jsonl",
                [
                    dict(module.TOMBSTONE_CONTRACT),
                    *recoveries,
                    self.row(2, sha256=""),
                ],
            )
            hydrated = self.write_jsonl(
                root, "hydrated.jsonl", [self.row(2, sha256=filled_sha)]
            )
            bundle_mapping, manifest, registry = self.bundle_fixture(root)
            recovery_mapping_sha = digest(recovery_mapping)
            bundle_mapping_sha = digest(bundle_mapping)
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
                mock.patch.object(
                    module,
                    "validate_reviewed_bundle_semantics",
                    return_value=(7, "7" * 64),
                ),
            ):
                output, summary = module.finalize_manifest(
                    source_path=source,
                    expected_source_sha256=digest(source),
                    hydrated_path=hydrated,
                    expected_hydrated_sha256=digest(hydrated),
                    recovery_mapping_path=recovery_mapping,
                    expected_recovery_mapping_sha256=recovery_mapping_sha,
                    bundle_mapping_path=bundle_mapping,
                    expected_bundle_mapping_sha256=bundle_mapping_sha,
                    bundle_manifest_path=manifest,
                    expected_bundle_manifest_sha256=digest(manifest),
                    registry_path=registry,
                    expected_registry_sha256=digest(registry),
                    recovery_plan_path=recovery_plan,
                    expected_recovery_plan_sha256=digest(recovery_plan),
                )
        rows = [json.loads(line) for line in output.decode().splitlines()]
        self.assertEqual(len(rows), 12)
        self.assertEqual(
            next(row for row in rows if row["entity_key"] == module.TOMBSTONE_ENTITY),
            module.TOMBSTONE_CONTRACT,
        )
        bundles = [row for row in rows if row["storage_adapter"] == "clone_b_bundle"]
        self.assertEqual(len(bundles), 7)
        recovery_rows = [
            row for row in rows
            if row["storage_adapter"]
            == recovery_contract.FINAL_STORAGE_ADAPTER
        ]
        self.assertEqual(len(recovery_rows), 3)
        self.assertEqual(summary["bundle_row_count"], 7)
        self.assertEqual(summary["recovery_row_count"], 3)
        self.assertEqual(summary["final_row_count"], 12)
        self.assertEqual(
            summary["recovery_mapping_sha256"], recovery_mapping_sha
        )
        self.assertEqual(
            summary["bundle_mapping_sha256"], bundle_mapping_sha
        )
        self.assertEqual(summary["reviewed_bundle_scope_count"], 7)
        self.assertEqual(
            summary["reviewed_bundle_semantics_sha256"], "7" * 64
        )

    def reviewed_bundle_fixture(self):
        normalized = {}
        resources = []
        for index, key in enumerate(sorted(bundle_validator.EXACT_SCOPES), 1):
            source_bundle = {
                "task_asset_id": 30000 + index,
                "format": "zip",
                "bundle_sha256": hashlib.sha256(
                    f"bundle-{index}".encode()
                ).hexdigest(),
                "manifest_sha256": hashlib.sha256(
                    f"manifest-{index}".encode()
                ).hexdigest(),
                "members": list(bundle_validator.EXACT_SCOPES[key]),
                "confirmed_by": 1,
                "confirmed_at": "2026-07-24T00:00:00Z",
                "confirmation_note": "approved fixture",
            }
            normalized[key] = source_bundle
            resources.append(
                {
                    "task_id": key[0],
                    "scope_kind": key[1],
                    "scope_ref_id": key[2],
                    "history": [
                        {
                            "revision_no": key[3],
                            "source_bundle": source_bundle,
                        }
                    ],
                }
            )
        return {"version": 2, "resources": resources}, normalized

    def test_reviewed_bundle_semantics_bind_exact_scopes_and_member_order(self):
        mapping, normalized = self.reviewed_bundle_fixture()
        count, semantic_sha = module.validate_reviewed_bundle_semantics(
            mapping, normalized
        )
        self.assertEqual(count, 7)
        self.assertRegex(semantic_sha, r"^[0-9a-f]{64}$")

        tampered = json.loads(json.dumps(mapping))
        tampered["resources"][0]["history"][0]["source_bundle"][
            "bundle_sha256"
        ] = "f" * 64
        with self.assertRaisesRegex(ValueError, "source_bundle drifted"):
            module.validate_reviewed_bundle_semantics(tampered, normalized)

        missing = json.loads(json.dumps(mapping))
        missing["resources"].pop()
        with self.assertRaisesRegex(ValueError, "scopes differ"):
            module.validate_reviewed_bundle_semantics(missing, normalized)

        reordered = json.loads(json.dumps(mapping))
        multi_member = next(
            resource
            for resource in reordered["resources"]
            if len(
                resource["history"][0]["source_bundle"]["members"]
            )
            > 1
        )
        multi_member["history"][0]["source_bundle"]["members"].reverse()
        with self.assertRaisesRegex(ValueError, "source_bundle drifted"):
            module.validate_reviewed_bundle_semantics(reordered, normalized)

        registry_missing = dict(normalized)
        registry_missing.pop(next(iter(registry_missing)))
        with self.assertRaisesRegex(ValueError, "exact seven scopes"):
            module.validate_reviewed_bundle_semantics(mapping, registry_missing)

    def test_finalize_rejects_bundle_mapping_hash_substitution(self):
        filled_sha = hashlib.sha256(b"filled").hexdigest()
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            recovery_mapping, recovery_plan, recoveries = self.recovery_fixture(root)
            source = self.write_jsonl(
                root,
                "source.jsonl",
                [dict(module.TOMBSTONE_CONTRACT), *recoveries, self.row(2, sha256="")],
            )
            hydrated = self.write_jsonl(
                root, "hydrated.jsonl", [self.row(2, sha256=filled_sha)]
            )
            bundle_mapping, manifest, registry = self.bundle_fixture(root)
            self.assertNotEqual(digest(recovery_mapping), digest(bundle_mapping))
            with self.assertRaisesRegex(ValueError, "bundle_mapping SHA-256"):
                module.finalize_manifest(
                    source_path=source,
                    expected_source_sha256=digest(source),
                    hydrated_path=hydrated,
                    expected_hydrated_sha256=digest(hydrated),
                    recovery_mapping_path=recovery_mapping,
                    expected_recovery_mapping_sha256=digest(recovery_mapping),
                    bundle_mapping_path=bundle_mapping,
                    expected_bundle_mapping_sha256=digest(recovery_mapping),
                    bundle_manifest_path=manifest,
                    expected_bundle_manifest_sha256=digest(manifest),
                    registry_path=registry,
                    expected_registry_sha256=digest(registry),
                    recovery_plan_path=recovery_plan,
                    expected_recovery_plan_sha256=digest(recovery_plan),
                )

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

    def test_cli_accepts_current_hash_bound_inputs_for_prepare_and_finalize(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            mapping_sha = "a" * 64
            bundle_mapping_sha = "1" * 64
            plan_sha = "b" * 64
            common_outputs = [
                "--output", str(root / "out.jsonl"),
                "--summary", str(root / "summary.json"),
            ]
            prepare_argv = [
                "prepare",
                "--source-manifest", str(root / "source.jsonl"),
                "--expected-source-sha256", "c" * 64,
                "--mapping", str(root / "mapping.json"),
                "--expected-mapping-sha256", mapping_sha,
                "--recovery-plan", str(root / "plan.json"),
                "--expected-recovery-plan-sha256", plan_sha,
                *common_outputs,
            ]
            finalize_argv = [
                "finalize",
                "--source-manifest", str(root / "source.jsonl"),
                "--expected-source-sha256", "c" * 64,
                "--hydrated-manifest", str(root / "hydrated.jsonl"),
                "--expected-hydrated-sha256", "d" * 64,
                "--recovery-mapping", str(root / "recovery-mapping.json"),
                "--expected-recovery-mapping-sha256", mapping_sha,
                "--bundle-mapping", str(root / "bundle-mapping.json"),
                "--expected-bundle-mapping-sha256", bundle_mapping_sha,
                "--bundle-manifest", str(root / "bundle-manifest.json"),
                "--expected-bundle-manifest-sha256", "e" * 64,
                "--bundle-registry", str(root / "registry.json"),
                "--expected-bundle-registry-sha256", "f" * 64,
                "--recovery-plan", str(root / "plan.json"),
                "--expected-recovery-plan-sha256", plan_sha,
                *common_outputs,
            ]
            for command, argv in (
                ("prepare", prepare_argv),
                ("finalize", finalize_argv),
            ):
                with (
                    self.subTest(command=command),
                    mock.patch.object(
                        module,
                        "prepare_manifest",
                        return_value=(b"{}\n", {"status": "PASS"}),
                    ),
                    mock.patch.object(
                        module,
                        "finalize_manifest",
                        return_value=(b"{}\n", {"status": "PASS"}),
                    ),
                    mock.patch.object(module, "atomic_write_many"),
                ):
                    self.assertEqual(0, module.main(argv))
                    if command == "finalize":
                        _, kwargs = module.finalize_manifest.call_args
                        self.assertEqual(
                            root / "recovery-mapping.json",
                            kwargs["recovery_mapping_path"],
                        )
                        self.assertEqual(
                            root / "bundle-mapping.json",
                            kwargs["bundle_mapping_path"],
                        )
                        self.assertEqual(
                            bundle_mapping_sha,
                            kwargs["expected_bundle_mapping_sha256"],
                        )


if __name__ == "__main__":
    unittest.main()
