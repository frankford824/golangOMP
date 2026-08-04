import copy
import hashlib
import importlib.util
import json
import pathlib
import tempfile
import unittest


MODULE_PATH = pathlib.Path(__file__).with_name(
    "apply_bundle_registry_to_mapping.py"
)
SPEC = importlib.util.spec_from_file_location("apply_bundle_registry", MODULE_PATH)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


def write_json(path: pathlib.Path, value: object) -> None:
    path.write_bytes(MODULE.canonical_bytes(value))


def fake_sha(value: int) -> str:
    return f"{value:064x}"


TEST_SCOPES = {
    (480, "task", 0, 1): (242, 243, 246, 249, 254),
    (523, "sku", 398, 1): (402, 403, 404, 405),
    (1264, "retouch_requirement", 44, 2): (5501, 6316),
}


class ApplyBundleRegistryTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = pathlib.Path(self.temp.name)
        self.mapping_path = self.root / "mapping.json"
        self.manifest_path = self.root / "manifest.json"
        self.registry_path = self.root / "registry.json"
        self.output_path = self.root / "output.json"
        self.evidence_path = self.root / "evidence.json"
        self.mapping = self.make_mapping()
        write_json(self.mapping_path, self.mapping)
        self.mapping_sha = MODULE.sha256_file(self.mapping_path)
        self.manifest = self.make_manifest()
        write_json(self.manifest_path, self.manifest)
        self.manifest_sha = MODULE.sha256_file(self.manifest_path)
        self.registry = self.make_registry()
        write_json(self.registry_path, self.registry)
        self.registry_sha = MODULE.sha256_file(self.registry_path)

    def tearDown(self):
        self.temp.cleanup()

    def make_mapping(self):
        resources = []
        for index, (key, members) in enumerate(TEST_SCOPES.items(), 1):
            task_id, scope_kind, scope_ref_id, revision_no = key
            revision = {
                "revision_no": revision_no,
                "status": "finalized",
                "mode": "single",
                "source_stage": "audit" if revision_no == 2 else "design",
                "final_task_asset_ids": [90000 + index],
                "reference_file_ref_ids": [],
                "evidence_event_ids": [f"task_event_log:event-{index}"],
                "confidence": "hard_blocked",
                "confirmed_by": 0,
                "confirmed_at": MODULE.ZERO_TIME,
                "confirmation_note": "",
                "manifest_row_hash": "",
                "reason": "candidate requires policy review",
                "created_by": 1,
                "created_at": "2026-07-23T00:00:00Z",
                "submitted_at": "2026-07-23T00:00:00Z",
                "finalized_at": "2026-07-23T00:00:00Z",
                "blockers": [
                    "multiple source assets require a reviewed deterministic ZIP bundle"
                ],
                "review_policy_ids": ["explicit_event_replay"],
                "source_bundle_candidate": {
                    "ordering": "completion_time_then_task_asset_id",
                    "ordered_member_task_asset_ids": list(members),
                },
            }
            if index <= 3:
                revision["blockers"].append(
                    "design revision has no lifecycle-valid source asset"
                    if index == 0
                    else "design revision has no uniquely evidenced source asset"
                )
            if revision_no == 2:
                revision["source_alias_from_task_asset_id"] = 80000 + index
            revision["manifest_row_hash"] = MODULE.revision_hash(revision)
            resources.append(
                {
                    "task_id": task_id,
                    "scope_kind": scope_kind,
                    "scope_ref_id": scope_ref_id,
                    "working_revision_no": revision_no,
                    "finalized_revision_no": revision_no,
                    "history": [revision],
                }
            )
        return {
            "version": 2,
            "resources": resources,
            "planning_tasks": [],
            "task_state_decisions": [],
            "asset_recoveries": [],
            "organization_mappings": [],
            "access_decisions": [],
        }

    def make_manifest(self):
        bundles = []
        for index, (key, members) in enumerate(TEST_SCOPES.items(), 1):
            task_id, scope_kind, scope_ref_id, revision_no = key
            bundles.append(
                {
                    "task_id": task_id,
                    "scope_kind": scope_kind,
                    "scope_ref_id": scope_ref_id,
                    "revision_no": revision_no,
                    "bundle_task_asset_id": 30000 + index,
                    "bundle_asset_id": 40000 + index,
                    "bundle_storage_ref_id": f"bundle-ref-{index}",
                    "confirmed": True,
                    "ordered_members": [
                        {
                            "task_asset_id": member,
                            "asset_id": 50000 + member,
                            "task_id": task_id,
                            "storage_ref_id": f"source-ref-{member}",
                            "sha256": fake_sha(member),
                            "confirmed": True,
                        }
                        for member in members
                    ],
                }
            )
        return {
            "schema_version": 1,
            "status": "CONFIRMED",
            "run_id": "formal-20260723-01",
            "mapping_sha256": self.mapping_sha,
            "source_candidate_sha256": fake_sha(999),
            "confirmed_by": 1,
            "confirmed_at": "2026-07-23T12:00:00Z",
            "confirmation_note": "exact source members reviewed",
            "bundles": bundles,
        }

    def make_registry(self):
        entries = []
        for index, (key, members) in enumerate(TEST_SCOPES.items(), 1):
            task_id, scope_kind, scope_ref_id, revision_no = key
            manifest_bundle = self.manifest["bundles"][index - 1]
            object_key = (
                f"fixture/formal-20260723-01/migration-bundles/task-{task_id}/"
                f"{scope_kind}-{scope_ref_id}/revision-{revision_no}/source-bundle.zip"
            )
            bundle_sha = fake_sha(60000 + index)
            source_bundle = {
                "task_asset_id": 30000 + index,
                "format": "zip",
                "bundle_sha256": bundle_sha,
                "manifest_sha256": "",
                "members": [
                    {
                        "task_asset_id": member,
                        "sha256": fake_sha(member),
                        "confirmed": True,
                    }
                    for member in members
                ],
                "confirmed_by": 1,
                "confirmed_at": "2026-07-23T12:00:00Z",
                "confirmation_note": "exact source members reviewed",
            }
            source_bundle["manifest_sha256"] = (
                MODULE.manifest_hash_for_source_bundle(source_bundle)
            )
            relative = f"objects/{object_key}"
            storage_ref = manifest_bundle["bundle_storage_ref_id"]
            size = 70000 + index
            entries.append(
                {
                    "task_id": task_id,
                    "scope_kind": scope_kind,
                    "scope_ref_id": scope_ref_id,
                    "revision_no": revision_no,
                    "relative_object_path": relative,
                    "object_key": object_key,
                    "bundle_sha256": bundle_sha,
                    "size": size,
                    "disposition": "created",
                    "source_bundle": source_bundle,
                    "asset_storage_ref_candidate": {
                        "ref_id": storage_ref,
                        "storage_adapter": "upload_service",
                        "ref_key": object_key,
                        "file_name": "source-bundle.zip",
                        "file_size": size,
                        "mime_type": "application/zip",
                        "checksum_hint": bundle_sha,
                        "status": "recorded",
                        "is_placeholder": False,
                    },
                    "task_asset_candidate": {
                        "id": 30000 + index,
                        "task_id": task_id,
                        "asset_id": 40000 + index,
                        "asset_type": "source",
                        "scope_kind": scope_kind,
                        "scope_ref_id": scope_ref_id,
                        "storage_ref_id": storage_ref,
                        "file_name": "source-bundle.zip",
                        "mime_type": "application/zip",
                        "file_size": size,
                        "storage_key": object_key,
                        "whole_hash": bundle_sha,
                        "upload_status": "uploaded",
                        "source_module_key": "migration",
                    },
                    "rollback_candidate": {
                        "task_asset_id": 30000 + index,
                        "storage_ref_id": storage_ref,
                        "relative_object_path": relative,
                        "expected_sha256": bundle_sha,
                    },
                }
            )
        return {
            "schema_version": 1,
            "status": "MATERIALIZED",
            "run_id": "formal-20260723-01",
            "manifest_sha256": self.manifest_sha,
            "b_root": str(self.root / "fixture-upload-b"),
            "database_write_performed": False,
            "entries": entries,
        }

    def prepare(self):
        return MODULE.prepare_outputs(
            self.mapping_path,
            self.manifest_path,
            self.registry_path,
            self.mapping_sha,
            self.manifest_sha,
            self.registry_sha,
        )

    def rewrite_manifest_and_registry(self):
        write_json(self.manifest_path, self.manifest)
        self.manifest_sha = MODULE.sha256_file(self.manifest_path)
        self.registry["manifest_sha256"] = self.manifest_sha
        write_json(self.registry_path, self.registry)
        self.registry_sha = MODULE.sha256_file(self.registry_path)

    def rewrite_registry(self):
        write_json(self.registry_path, self.registry)
        self.registry_sha = MODULE.sha256_file(self.registry_path)

    def test_exact_merge_remains_unreviewed_and_hash_bound(self):
        output, evidence = self.prepare()
        self.assertEqual(evidence["target_count"], len(TEST_SCOPES))
        self.assertFalse(evidence["business_policy_review_performed"])
        self.assertEqual(
            evidence["input_sha256"],
            {
                "mapping_sha256": self.mapping_sha,
                "manifest_sha256": self.manifest_sha,
                "registry_sha256": self.registry_sha,
            },
        )
        for index, resource in enumerate(output["resources"]):
            revision = resource["history"][0]
            self.assertEqual(revision["confidence"], "proposed_review")
            self.assertEqual(revision["confirmed_by"], 0)
            self.assertEqual(revision["confirmed_at"], MODULE.ZERO_TIME)
            self.assertEqual(revision["confirmation_note"], "")
            self.assertNotIn("blockers", revision)
            self.assertNotIn("source_bundle_candidate", revision)
            self.assertNotIn("source_alias_from_task_asset_id", revision)
            self.assertEqual(
                revision["source_bundle"]["task_asset_id"], 30001 + index
            )
            self.assertEqual(
                revision["manifest_row_hash"], MODULE.revision_hash(revision)
            )
        self.assertEqual(
            evidence["output_mapping_sha256"],
            hashlib.sha256(MODULE.canonical_bytes(output)).hexdigest(),
        )

    def test_deterministic_atomic_outputs_are_idempotent(self):
        output, evidence = self.prepare()
        MODULE.atomic_write_many(
            [
                (self.output_path, MODULE.canonical_bytes(output)),
                (self.evidence_path, MODULE.canonical_bytes(evidence)),
            ]
        )
        before = (self.output_path.read_bytes(), self.evidence_path.read_bytes())
        MODULE.atomic_write_many(
            [
                (self.output_path, MODULE.canonical_bytes(output)),
                (self.evidence_path, MODULE.canonical_bytes(evidence)),
            ]
        )
        self.assertEqual(
            before, (self.output_path.read_bytes(), self.evidence_path.read_bytes())
        )
        with self.assertRaises(FileExistsError):
            MODULE.atomic_write_many([(self.output_path, b"different\n")])

    def test_requires_explicit_exact_input_hashes(self):
        with self.assertRaisesRegex(ValueError, "input SHA-256 mismatch"):
            MODULE.prepare_outputs(
                self.mapping_path,
                self.manifest_path,
                self.registry_path,
                "0" * 64,
                self.manifest_sha,
                self.registry_sha,
            )
        self.manifest["mapping_sha256"] = "0" * 64
        self.rewrite_manifest_and_registry()
        with self.assertRaisesRegex(ValueError, "does not bind"):
            self.prepare()

    def test_rejects_scope_and_member_identity_drift(self):
        self.registry["entries"][0]["scope_ref_id"] = 999
        self.rewrite_registry()
        with self.assertRaisesRegex(ValueError, "scope_ref_id is invalid"):
            self.prepare()
        self.setUp_fresh()
        self.registry["entries"][0]["source_bundle"]["members"].reverse()
        self.rewrite_registry()
        with self.assertRaisesRegex(ValueError, "order/identity drifted"):
            self.prepare()

    def setUp_fresh(self):
        self.temp.cleanup()
        self.setUp()

    def test_rejects_member_hash_bundle_id_and_status_drift(self):
        self.registry["entries"][0]["source_bundle"]["members"][0]["sha256"] = (
            fake_sha(42)
        )
        self.rewrite_registry()
        with self.assertRaisesRegex(ValueError, "SHA-256 drifted"):
            self.prepare()
        self.setUp_fresh()
        self.registry["entries"][0]["task_asset_candidate"]["id"] = 99999
        self.rewrite_registry()
        with self.assertRaisesRegex(ValueError, "task_asset_candidate drifted"):
            self.prepare()
        self.setUp_fresh()
        self.registry["status"] = "PREPARED"
        self.rewrite_registry()
        with self.assertRaisesRegex(ValueError, "status=MATERIALIZED"):
            self.prepare()
        self.setUp_fresh()
        first_member = TEST_SCOPES[next(iter(TEST_SCOPES))][0]
        self.manifest["bundles"][0]["bundle_task_asset_id"] = first_member
        self.registry["entries"][0]["source_bundle"][
            "task_asset_id"
        ] = first_member
        self.registry["entries"][0]["task_asset_candidate"][
            "id"
        ] = first_member
        self.registry["entries"][0]["rollback_candidate"][
            "task_asset_id"
        ] = first_member
        self.rewrite_manifest_and_registry()
        with self.assertRaisesRegex(ValueError, "reuses a source member id"):
            self.prepare()

    def test_rejects_mapping_blocker_confidence_hash_and_existing_source_drift(self):
        revision = self.mapping["resources"][0]["history"][0]
        revision["blockers"].append("unrelated blocker")
        revision["manifest_row_hash"] = MODULE.revision_hash(revision)
        write_json(self.mapping_path, self.mapping)
        self.mapping_sha = MODULE.sha256_file(self.mapping_path)
        self.manifest["mapping_sha256"] = self.mapping_sha
        self.rewrite_manifest_and_registry()
        with self.assertRaisesRegex(ValueError, "invalid bundle candidate"):
            self.prepare()
        self.setUp_fresh()
        self.mapping["resources"][0]["history"][0]["manifest_row_hash"] = "0" * 64
        write_json(self.mapping_path, self.mapping)
        self.mapping_sha = MODULE.sha256_file(self.mapping_path)
        self.manifest["mapping_sha256"] = self.mapping_sha
        self.rewrite_manifest_and_registry()
        with self.assertRaisesRegex(ValueError, "manifest_row_hash is stale"):
            self.prepare()
        self.setUp_fresh()
        revision = self.mapping["resources"][0]["history"][0]
        revision["source_bundle"] = copy.deepcopy(
            self.registry["entries"][0]["source_bundle"]
        )
        revision["manifest_row_hash"] = MODULE.revision_hash(revision)
        write_json(self.mapping_path, self.mapping)
        self.mapping_sha = MODULE.sha256_file(self.mapping_path)
        self.manifest["mapping_sha256"] = self.mapping_sha
        self.rewrite_manifest_and_registry()
        with self.assertRaisesRegex(ValueError, "already has source_bundle"):
            self.prepare()


if __name__ == "__main__":
    unittest.main()
