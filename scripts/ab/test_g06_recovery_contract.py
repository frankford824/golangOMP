from __future__ import annotations

import json
import pathlib
import tempfile
import unittest

from scripts.ab import g06_recovery_contract as CONTRACT
from scripts.ab import hydrate_object_manifest as HYDRATOR
from scripts.ab import rebase_g06_recovery_checkpoint as REBASE
from scripts.ab import test_finalize_g06_verdict as FINAL_FIXTURES
from scripts.ab import verify_g06_clone_b_recoveries as VERIFIER


class G06RecoveryContractTest(unittest.TestCase):
    def fixture(self, root: pathlib.Path):
        helper = FINAL_FIXTURES.FinalizeG06VerdictTest()
        paths = helper.fixture(root)
        mapping_rows, plan_entries, _hashes = CONTRACT.load_contract(
            mapping_path=paths["mapping"],
            expected_mapping_sha256=paths["mapping_sha"],
            plan_path=paths["recovery_plan"],
            expected_plan_sha256=paths["recovery_plan_sha"],
        )
        return paths, mapping_rows, plan_entries

    def write_jsonl(self, path: pathlib.Path, rows: list[dict]) -> None:
        path.write_bytes(
            "".join(
                CONTRACT.canonical_json(row) + "\n" for row in rows
            ).encode("utf-8")
        )

    def test_real_hold_open_component_self_hash_regression(self):
        component = {
            "action": "apply",
            "artifacts": [
                {
                    "path": "recovery-materialization-plan.json",
                    "sha256": CONTRACT.APPROVED_PLAN_SHA256,
                    "size": 13592,
                },
                {
                    "path": "recovery-guard-before.json",
                    "sha256": "eeeb01de15c4e6361cbd5d386b7efe4d96cc6394cb6ba3bb640905909810c677",
                    "size": 285,
                },
                {
                    "path": "recovery-guard-provision.json",
                    "sha256": "6c9a7da4ed6f4235f2954258a7164eae9ab167dd7e7973cf65c0a0e32069c3e1",
                    "size": 660,
                },
                {
                    "path": "recovery-db-apply.json",
                    "sha256": CONTRACT.APPROVED_DB_APPLY_SHA256,
                    "size": 431,
                },
                {
                    "path": "recovery-db-idempotent.json",
                    "sha256": CONTRACT.APPROVED_DB_IDEMPOTENT_SHA256,
                    "size": 431,
                },
            ],
            "component": "recovery",
            "database": CONTRACT.EXPECTED_DATABASE,
            "database_writes_executed": True,
            "guard_exactly_restored": False,
            "guard_retained_for_rollback": True,
            "host": CONTRACT.EXPECTED_HOST,
            "production_writes_executed": False,
            "run_id": "bundle-materialization-20260723-29",
            "schema_version": 1,
            "status": "APPLIED",
        }
        self.assertEqual(
            "cea060c0d1856ede5f8ddf94701975c3c32a9b64063467b9cc71a848da65d431",
            CONTRACT.component_self_hash(component),
        )

    def test_frozen_boundary_rejects_mapping_or_plan_sha_drift(self):
        with self.assertRaisesRegex(ValueError, "frozen"):
            CONTRACT.require_frozen_hashes(
                "0" * 64, CONTRACT.APPROVED_PLAN_SHA256
            )
        with self.assertRaisesRegex(ValueError, "frozen"):
            CONTRACT.require_frozen_hashes(
                CONTRACT.APPROVED_MAPPING_SHA256, "0" * 64
            )

    def test_local_verifier_reads_exact_three_targets_and_binds_receipts(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            paths, mapping_rows, plan_entries = self.fixture(root)
            object_root = root / "objects"
            for missing_id, entry in plan_entries.items():
                target = object_root / entry["target_object_key"]
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(
                    bytes([missing_id % 251]) * entry["source_size"]
                )
            verdict = VERIFIER.verify(
                object_root=object_root,
                mapping_path=paths["mapping"],
                expected_mapping_sha256=paths["mapping_sha"],
                plan_path=paths["recovery_plan"],
                expected_plan_sha256=paths["recovery_plan_sha"],
                db_apply_path=paths["recovery_apply"],
                db_idempotent_path=paths["recovery_idempotent"],
                component_apply_path=paths["recovery_component"],
                require_frozen=False,
            )
        self.assertEqual("PASS", verdict["status"])
        self.assertEqual(3, verdict["checked_count"])
        self.assertEqual(3, verdict["read_only_local_get_count"])
        self.assertFalse(verdict["production_write_performed"])
        self.assertEqual(CONTRACT.EXPECTED_DATABASE, verdict["database"])

    def test_local_verifier_rejects_symlink_and_receipt_drift(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            paths, _mapping_rows, plan_entries = self.fixture(root)
            object_root = root / "objects"
            first_id = CONTRACT.RECOVERY_IDS[0]
            for missing_id, entry in plan_entries.items():
                target = object_root / entry["target_object_key"]
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(
                    bytes([missing_id % 251]) * entry["source_size"]
                )
            target = object_root / plan_entries[first_id]["target_object_key"]
            outside = root / "outside.jpg"
            outside.write_bytes(target.read_bytes())
            target.unlink()
            target.symlink_to(outside)
            verdict = VERIFIER.verify(
                object_root=object_root,
                mapping_path=paths["mapping"],
                expected_mapping_sha256=paths["mapping_sha"],
                plan_path=paths["recovery_plan"],
                expected_plan_sha256=paths["recovery_plan_sha"],
                db_apply_path=paths["recovery_apply"],
                db_idempotent_path=paths["recovery_idempotent"],
                component_apply_path=paths["recovery_component"],
                require_frozen=False,
            )
            self.assertEqual("BLOCKED", verdict["status"])
            self.assertIn("symlink", verdict["violations"][0]["detail"])

            receipt = json.loads(
                paths["recovery_apply"].read_text(encoding="utf-8")
            )
            receipt["database_transaction_committed"] = False
            paths["recovery_apply"].write_text(
                CONTRACT.canonical_json(receipt) + "\n", encoding="utf-8"
            )
            verdict = VERIFIER.verify(
                object_root=object_root,
                mapping_path=paths["mapping"],
                expected_mapping_sha256=paths["mapping_sha"],
                plan_path=paths["recovery_plan"],
                expected_plan_sha256=paths["recovery_plan_sha"],
                db_apply_path=paths["recovery_apply"],
                db_idempotent_path=paths["recovery_idempotent"],
                component_apply_path=paths["recovery_component"],
                require_frozen=False,
            )
        self.assertEqual("BLOCKED", verdict["status"])

    def test_local_verifier_rejects_size_and_sha_mismatch_separately(self):
        for mode in ("size", "sha256"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as raw:
                root = pathlib.Path(raw)
                paths, _mapping_rows, plan_entries = self.fixture(root)
                object_root = root / "objects"
                for missing_id, entry in plan_entries.items():
                    target = object_root / entry["target_object_key"]
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_bytes(
                        bytes([missing_id % 251]) * entry["source_size"]
                    )
                first = plan_entries[CONTRACT.RECOVERY_IDS[0]]
                target = object_root / first["target_object_key"]
                data = bytearray(target.read_bytes())
                if mode == "size":
                    target.write_bytes(data[:-1])
                else:
                    data[0] ^= 1
                    target.write_bytes(data)
                verdict = VERIFIER.verify(
                    object_root=object_root,
                    mapping_path=paths["mapping"],
                    expected_mapping_sha256=paths["mapping_sha"],
                    plan_path=paths["recovery_plan"],
                    expected_plan_sha256=paths["recovery_plan_sha"],
                    db_apply_path=paths["recovery_apply"],
                    db_idempotent_path=paths["recovery_idempotent"],
                    component_apply_path=paths["recovery_component"],
                    require_frozen=False,
                )
                self.assertEqual("BLOCKED", verdict["status"])
                self.assertEqual(
                    f"g06.recovery_{mode}_mismatch",
                    verdict["violations"][0]["violation_code"],
                )

    def test_apply_receipt_semantic_tampering_is_fail_closed(self):
        mutations = {
            "database": "wrong",
            "host": "remote",
            "run_id": "wrong",
            "plan_sha256": "0" * 64,
            "changed_entries": 2,
            "already_in_target_state_entries": 1,
            "database_transaction_committed": False,
            "object_storage_writes_executed": True,
        }
        for field, value in mutations.items():
            with self.subTest(field=field), tempfile.TemporaryDirectory() as raw:
                root = pathlib.Path(raw)
                paths, _mapping_rows, _plan_entries = self.fixture(root)
                receipt = json.loads(
                    paths["recovery_apply"].read_text(encoding="utf-8")
                )
                receipt[field] = value
                paths["recovery_apply"].write_text(
                    CONTRACT.canonical_json(receipt) + "\n",
                    encoding="utf-8",
                )
                with self.assertRaisesRegex(ValueError, "apply receipt"):
                    CONTRACT.validate_apply_receipts(
                        plan_path=paths["recovery_plan"],
                        db_apply_path=paths["recovery_apply"],
                        db_idempotent_path=paths["recovery_idempotent"],
                        component_apply_path=paths["recovery_component"],
                        require_frozen=False,
                    )

    def test_component_receipt_semantic_tampering_is_fail_closed(self):
        mutations = {
            "production_writes_executed": True,
            "database": "wrong",
            "host": "remote",
            "run_id": "wrong",
            "action": "rollback",
            "status": "ROLLED_BACK",
            "guard_retained_for_rollback": False,
            "guard_exactly_restored": True,
        }
        for field, value in mutations.items():
            with self.subTest(field=field), tempfile.TemporaryDirectory() as raw:
                root = pathlib.Path(raw)
                paths, _mapping_rows, _plan_entries = self.fixture(root)
                component = json.loads(
                    paths["recovery_component"].read_text(encoding="utf-8")
                )
                component[field] = value
                component["evidence_sha256"] = CONTRACT.component_self_hash(
                    component
                )
                paths["recovery_component"].write_text(
                    CONTRACT.canonical_json(component) + "\n",
                    encoding="utf-8",
                )
                with self.assertRaisesRegex(ValueError, "component apply"):
                    CONTRACT.validate_apply_receipts(
                        plan_path=paths["recovery_plan"],
                        db_apply_path=paths["recovery_apply"],
                        db_idempotent_path=paths["recovery_idempotent"],
                        component_apply_path=paths["recovery_component"],
                        require_frozen=False,
                    )

    def test_idempotent_receipt_semantic_tampering_is_fail_closed(self):
        for field, value in {
            "changed_entries": 1,
            "already_in_target_state_entries": 2,
            "database_transaction_committed": False,
            "object_storage_writes_executed": True,
        }.items():
            with self.subTest(field=field), tempfile.TemporaryDirectory() as raw:
                root = pathlib.Path(raw)
                paths, _mapping_rows, _plan_entries = self.fixture(root)
                receipt = json.loads(
                    paths["recovery_idempotent"].read_text(encoding="utf-8")
                )
                receipt[field] = value
                paths["recovery_idempotent"].write_text(
                    CONTRACT.canonical_json(receipt) + "\n",
                    encoding="utf-8",
                )
                with self.assertRaisesRegex(ValueError, "idempotent receipt"):
                    CONTRACT.validate_apply_receipts(
                        plan_path=paths["recovery_plan"],
                        db_apply_path=paths["recovery_apply"],
                        db_idempotent_path=paths["recovery_idempotent"],
                        component_apply_path=paths["recovery_component"],
                        require_frozen=False,
                    )

    def test_component_evidence_and_artifact_hash_tampering_is_fail_closed(self):
        for mode in ("evidence", "artifact"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as raw:
                root = pathlib.Path(raw)
                paths, _mapping_rows, _plan_entries = self.fixture(root)
                component = json.loads(
                    paths["recovery_component"].read_text(encoding="utf-8")
                )
                if mode == "evidence":
                    component["evidence_sha256"] = "0" * 64
                else:
                    component["artifacts"][0]["sha256"] = "0" * 64
                    component["evidence_sha256"] = (
                        CONTRACT.component_self_hash(component)
                    )
                paths["recovery_component"].write_text(
                    CONTRACT.canonical_json(component) + "\n",
                    encoding="utf-8",
                )
                with self.assertRaises(ValueError):
                    CONTRACT.validate_apply_receipts(
                        plan_path=paths["recovery_plan"],
                        db_apply_path=paths["recovery_apply"],
                        db_idempotent_path=paths["recovery_idempotent"],
                        component_apply_path=paths["recovery_component"],
                        require_frozen=False,
                    )

    def checkpoint_fixture(self, root: pathlib.Path):
        paths, mapping_rows, plan_entries = self.fixture(root)
        removed = CONTRACT.original_manifest_rows(mapping_rows, plan_entries)
        retained = {
            "entity_key": "task_asset:7",
            "owner_kind": "task_asset",
            "owner_id": 7,
            "task_id": 8,
            "storage_ref_id": "retained-ref",
            "storage_adapter": "oss_upload_service",
            "object_key": "tasks/8/retained.bin",
            "size": 4,
            "mime_type": "application/octet-stream",
            "sha256": "",
            "status": "recorded",
            "is_placeholder": False,
        }
        old_rows = sorted([retained, *removed], key=lambda row: (
            row["owner_kind"], row["owner_id"], row["task_id"], row["entity_key"]
        ))
        new_rows = [retained]
        old_input = root / "old.jsonl"
        new_input = root / "new.jsonl"
        self.write_jsonl(old_input, old_rows)
        self.write_jsonl(new_input, new_rows)
        old_sha = CONTRACT.sha256_file(old_input)
        new_sha = CONTRACT.sha256_file(new_input)
        completed_record = HYDRATOR.checkpoint_record(
            "upload",
            retained["object_key"],
            HYDRATOR.ObjectMetadata(
                size=4,
                mime_type="application/octet-stream",
                sha256="a" * 64,
            ),
        )
        completed = {
            HYDRATOR.checkpoint_key("upload", retained["object_key"]):
            completed_record
        }
        failed = {}
        for row in removed:
            key = HYDRATOR.checkpoint_key("upload", row["object_key"])
            failed[key] = HYDRATOR.checkpoint_failure_record(
                "upload",
                row["object_key"],
                "object_manifest.missing",
                "http_status=404",
            )
        checkpoint = root / "old-checkpoint.json"
        checkpoint.write_text(
            CONTRACT.canonical_json(
                HYDRATOR.checkpoint_document(
                    old_sha,
                    {"upload": "b" * 64, "oss": "c" * 64},
                    completed,
                    failed,
                )
            )
            + "\n",
            encoding="utf-8",
        )
        return paths, old_input, old_sha, new_input, new_sha, checkpoint

    def test_checkpoint_rebase_removes_only_three_404_and_preserves_records(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            (
                paths,
                old_input,
                old_sha,
                new_input,
                new_sha,
                checkpoint,
            ) = self.checkpoint_fixture(root)
            old_doc = json.loads(checkpoint.read_text(encoding="utf-8"))
            payload, summary = REBASE.rebase(
                old_input_path=old_input,
                expected_old_input_sha256=old_sha,
                new_input_path=new_input,
                expected_new_input_sha256=new_sha,
                old_checkpoint_path=checkpoint,
                expected_old_checkpoint_sha256=CONTRACT.sha256_file(checkpoint),
                mapping_path=paths["mapping"],
                expected_mapping_sha256=paths["mapping_sha"],
                plan_path=paths["recovery_plan"],
                expected_plan_sha256=paths["recovery_plan_sha"],
            )
            new_doc = json.loads(payload)
        self.assertEqual(new_sha, new_doc["input_manifest_sha256"])
        self.assertEqual(old_doc["completed"], new_doc["completed"])
        self.assertEqual([], new_doc["failed"])
        self.assertEqual(3, summary["removed_failed_record_count"])

    def test_checkpoint_rebase_rejects_any_fourth_exception_or_non404(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            (
                paths,
                old_input,
                old_sha,
                new_input,
                new_sha,
                checkpoint,
            ) = self.checkpoint_fixture(root)
            doc = json.loads(checkpoint.read_text(encoding="utf-8"))
            doc["failed"][0]["detail"] = "http_status=410"
            checkpoint.write_text(
                CONTRACT.canonical_json(doc) + "\n", encoding="utf-8"
            )
            with self.assertRaisesRegex(ValueError, "exact original 404"):
                REBASE.rebase(
                    old_input_path=old_input,
                    expected_old_input_sha256=old_sha,
                    new_input_path=new_input,
                    expected_new_input_sha256=new_sha,
                    old_checkpoint_path=checkpoint,
                    expected_old_checkpoint_sha256=CONTRACT.sha256_file(checkpoint),
                    mapping_path=paths["mapping"],
                    expected_mapping_sha256=paths["mapping_sha"],
                    plan_path=paths["recovery_plan"],
                    expected_plan_sha256=paths["recovery_plan_sha"],
                )

    def test_checkpoint_rebase_rejects_input_drift_added_or_extra_removed(self):
        for mode in ("row_drift", "added", "extra_removed"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as raw:
                root = pathlib.Path(raw)
                (
                    paths,
                    old_input,
                    old_sha,
                    new_input,
                    _new_sha,
                    checkpoint,
                ) = self.checkpoint_fixture(root)
                new_rows = [
                    json.loads(line)
                    for line in new_input.read_text(encoding="utf-8").splitlines()
                ]
                if mode == "row_drift":
                    new_rows[0]["storage_ref_id"] = "drifted"
                elif mode == "added":
                    extra = dict(new_rows[0])
                    extra["entity_key"] = "task_asset:999"
                    extra["owner_id"] = 999
                    extra["object_key"] = "tasks/8/added.bin"
                    new_rows.append(extra)
                else:
                    old_rows = [
                        json.loads(line)
                        for line in old_input.read_text(
                            encoding="utf-8"
                        ).splitlines()
                    ]
                    old_rows.append(
                        {
                            **new_rows[0],
                            "entity_key": "task_asset:998",
                            "owner_id": 998,
                            "object_key": "tasks/8/extra-removed.bin",
                        }
                    )
                    old_rows.sort(
                        key=lambda row: (
                            row["owner_kind"],
                            row["owner_id"],
                            row["task_id"],
                            row["entity_key"],
                        )
                    )
                    self.write_jsonl(old_input, old_rows)
                    old_sha = CONTRACT.sha256_file(old_input)
                    checkpoint_doc = json.loads(
                        checkpoint.read_text(encoding="utf-8")
                    )
                    checkpoint_doc["input_manifest_sha256"] = old_sha
                    checkpoint.write_text(
                        CONTRACT.canonical_json(checkpoint_doc) + "\n",
                        encoding="utf-8",
                    )
                self.write_jsonl(new_input, new_rows)
                with self.assertRaises(ValueError):
                    REBASE.rebase(
                        old_input_path=old_input,
                        expected_old_input_sha256=old_sha,
                        new_input_path=new_input,
                        expected_new_input_sha256=CONTRACT.sha256_file(new_input),
                        old_checkpoint_path=checkpoint,
                        expected_old_checkpoint_sha256=CONTRACT.sha256_file(checkpoint),
                        mapping_path=paths["mapping"],
                        expected_mapping_sha256=paths["mapping_sha"],
                        plan_path=paths["recovery_plan"],
                        expected_plan_sha256=paths["recovery_plan_sha"],
                    )

    def test_checkpoint_rebase_rejects_checkpoint_binding_or_extra_record(self):
        for mode in ("input_hash", "extra_record"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as raw:
                root = pathlib.Path(raw)
                (
                    paths,
                    old_input,
                    old_sha,
                    new_input,
                    new_sha,
                    checkpoint,
                ) = self.checkpoint_fixture(root)
                doc = json.loads(checkpoint.read_text(encoding="utf-8"))
                if mode == "input_hash":
                    doc["input_manifest_sha256"] = "0" * 64
                else:
                    doc["failed"].append(
                        HYDRATOR.checkpoint_failure_record(
                            "upload",
                            "tasks/8/unknown.bin",
                            "object_manifest.missing",
                            "http_status=404",
                        )
                    )
                checkpoint.write_text(
                    CONTRACT.canonical_json(doc) + "\n", encoding="utf-8"
                )
                with self.assertRaises(ValueError):
                    REBASE.rebase(
                        old_input_path=old_input,
                        expected_old_input_sha256=old_sha,
                        new_input_path=new_input,
                        expected_new_input_sha256=new_sha,
                        old_checkpoint_path=checkpoint,
                        expected_old_checkpoint_sha256=CONTRACT.sha256_file(checkpoint),
                        mapping_path=paths["mapping"],
                        expected_mapping_sha256=paths["mapping_sha"],
                        plan_path=paths["recovery_plan"],
                        expected_plan_sha256=paths["recovery_plan_sha"],
                    )


if __name__ == "__main__":
    unittest.main()
