import copy
import hashlib
import json
import pathlib
import tempfile
import unittest

import prepare_source_bundle_confirmation as confirmation
import run_scoped_bundle_materializer as materializer


class PrepareSourceBundleConfirmationTest(unittest.TestCase):
    def candidate(self):
        bundles = []
        member_id = 100
        member_counts = [2, 4, 4, 2, 5, 3, 2]
        for bundle_index, count in enumerate(member_counts):
            task_id = 500 + bundle_index
            members = []
            for member_index in range(count):
                member_id += 1
                content = f"member-{member_id}".encode()
                members.append(
                    {
                        "task_asset_id": member_id,
                        "asset_id": 1000 + member_id,
                        "task_id": task_id,
                        "storage_ref_id": f"legacy-ref-{member_id}",
                        "original_file_name": f"{member_id}.psd",
                        "object_key": f"legacy/{member_id}.psd",
                        "size": len(content),
                        "sha256": hashlib.sha256(content).hexdigest(),
                        "source_stage": "design",
                        "evidence_event_ids": [f"task_event_log:event-{member_id}"],
                        "confirmed": False,
                    }
                )
            bundles.append(
                {
                    "task_id": task_id,
                    "scope_kind": "sku",
                    "scope_ref_id": 700 + bundle_index,
                    "revision_no": 1 if bundle_index < 3 else 2,
                    "bundle_task_asset_id": None,
                    "confidence": "proposed_review",
                    "requires_human_member_confirmation": True,
                    "all_members_exist_and_hash_verified": True,
                    "materialization_status": "blocked_no_review",
                    "ordered_members": members,
                }
            )
        return {
            "schema_version": 1,
            "status": "PROPOSED_REVIEW",
            "source_candidate_sha256": "a" * 64,
            "bundle_count": 7,
            "member_count": 22,
            "bundles": bundles,
        }

    def allocation(self):
        return {
            "schema_version": 1,
            "status": "FROZEN",
            "database": "ab_r20260723_01_b",
            "run_id": "formal-20260723-01",
            "max_task_asset_id": 9000,
            "max_design_asset_id": 10000,
            "read_only": True,
            "query_evidence": {
                "query": "SELECT MAX(id) FROM task_assets",
                "result_sha256": "b" * 64,
            },
        }

    def write_inputs(self, root):
        candidate_path = root / "candidate.json"
        mapping_path = root / "mapping.json"
        allocation_path = root / "allocation.json"
        candidate_path.write_bytes(confirmation.canonical_bytes(self.candidate()))
        mapping_path.write_bytes(
            confirmation.canonical_bytes(
                {"version": 2, "resources": [{"task_id": 500}]}
            )
        )
        allocation_path.write_bytes(confirmation.canonical_bytes(self.allocation()))
        return candidate_path, mapping_path, allocation_path

    def prepared(self, candidate_path, mapping_path, allocation_path):
        candidate = confirmation.read_json_object(candidate_path, "candidate")
        bundles = confirmation.validate_candidate(candidate)
        mapping = confirmation.read_json_object(mapping_path, "mapping")
        confirmation.validate_mapping(mapping)
        raw_allocation = confirmation.read_json_object(
            allocation_path, "allocation"
        )
        allocation = confirmation.validate_allocation(raw_allocation, bundles)
        candidate_hash = confirmation.sha256_file(candidate_path)
        allocation_hash = confirmation.sha256_file(allocation_path)
        rows = confirmation.allocation_rows(
            bundles, allocation, candidate_hash
        )
        template = confirmation.decision_template(
            candidate,
            candidate_hash,
            confirmation.sha256_file(mapping_path),
            allocation,
            allocation_hash,
            rows,
        )
        return candidate, template

    def approved_decision(self, template):
        return {
            "schema_version": 1,
            "decision": "APPROVED",
            "reviewer_id": 1,
            "approved_at": "2026-07-23T12:34:56Z",
            "note": "approved exact seven scopes and ordered members",
            "database": template["database"],
            "run_id": template["run_id"],
            "candidate_file_sha256": template["candidate_file_sha256"],
            "source_candidate_sha256": template["source_candidate_sha256"],
            "mapping_sha256": template["mapping_sha256"],
            "allocation_evidence_sha256": template[
                "allocation_evidence_sha256"
            ],
            "decision_template_sha256": confirmation.sha256_bytes(
                confirmation.canonical_bytes(template)
            ),
            "bundle_count": 7,
            "member_count": 22,
        }

    def test_prepare_is_deterministic_and_allocates_exact_ids(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            candidate_path, mapping_path, allocation_path = self.write_inputs(root)
            candidate, template = self.prepared(
                candidate_path, mapping_path, allocation_path
            )
            self.assertEqual(
                [row["bundle_task_asset_id"] for row in template["allocations"]],
                list(range(9001, 9008)),
            )
            self.assertEqual(
                [row["bundle_asset_id"] for row in template["allocations"]],
                list(range(10001, 10008)),
            )
            first = confirmation.canonical_bytes(template)
            _, repeated = self.prepared(
                candidate_path, mapping_path, allocation_path
            )
            self.assertEqual(first, confirmation.canonical_bytes(repeated))
            self.assertEqual(len({r["bundle_storage_ref_id"] for r in template["allocations"]}), 7)
            self.assertEqual(candidate["bundles"][0]["ordered_members"][0]["confirmed"], False)

    def test_apply_requires_exact_approval_and_materializer_accepts_manifest(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            candidate_path, mapping_path, allocation_path = self.write_inputs(root)
            candidate, template = self.prepared(
                candidate_path, mapping_path, allocation_path
            )
            decision = self.approved_decision(template)
            confirmation.validate_decision(
                decision,
                template,
                decision["decision_template_sha256"],
            )
            decision_path = root / "decision.json"
            decision_path.write_bytes(confirmation.canonical_bytes(decision))
            manifest = confirmation.confirmed_manifest(
                candidate,
                template,
                decision,
                confirmation.sha256_file(decision_path),
                decision["decision_template_sha256"],
            )
            self.assertEqual(manifest["status"], "CONFIRMED")
            self.assertEqual(
                manifest["mapping_sha256"], template["mapping_sha256"]
            )
            self.assertEqual(len(manifest["bundles"]), 7)
            self.assertEqual(
                sum(len(bundle["ordered_members"]) for bundle in manifest["bundles"]),
                22,
            )
            self.assertTrue(
                all(
                    member["confirmed"]
                    for bundle in manifest["bundles"]
                    for member in bundle["ordered_members"]
                )
            )

            source_root = root / "source"
            source_root.mkdir()
            for bundle in manifest["bundles"]:
                for member in bundle["ordered_members"]:
                    path = source_root / member["object_key"]
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_bytes(f"member-{member['task_asset_id']}".encode())
            prepared = materializer.validate_manifest(manifest, source_root)
            self.assertEqual(len(prepared), 7)

    def test_non_exact_candidate_and_allocation_fail_closed(self):
        candidate = self.candidate()
        candidate["bundles"].pop()
        with self.assertRaisesRegex(ValueError, "exactly 7"):
            confirmation.validate_candidate(candidate)

        candidate = self.candidate()
        candidate["bundles"][0]["confidence"] = "confirmed_auto"
        with self.assertRaisesRegex(ValueError, "review state drifted"):
            confirmation.validate_candidate(candidate)

        candidate = self.candidate()
        bundles = confirmation.validate_candidate(candidate)
        allocation = self.allocation()
        allocation["max_task_asset_id"] = 101
        with self.assertRaisesRegex(ValueError, "behind candidate"):
            confirmation.validate_allocation(allocation, bundles)

        allocation = self.allocation()
        allocation["database"] = "production"
        with self.assertRaisesRegex(ValueError, "Clone B"):
            confirmation.validate_allocation(allocation, bundles)

        with self.assertRaisesRegex(ValueError, "mapping.version"):
            confirmation.validate_mapping({"version": 1, "resources": []})

    def test_decision_drift_rejection_and_unknown_fields_fail_closed(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            candidate_path, mapping_path, allocation_path = self.write_inputs(root)
            _, template = self.prepared(
                candidate_path, mapping_path, allocation_path
            )
            decision = self.approved_decision(template)
            decision["decision"] = "PENDING_REVIEW"
            with self.assertRaisesRegex(ValueError, "exactly APPROVED"):
                confirmation.validate_decision(
                    decision,
                    template,
                    decision["decision_template_sha256"],
                )
            decision = self.approved_decision(template)
            decision["candidate_file_sha256"] = "f" * 64
            with self.assertRaisesRegex(ValueError, "does not match"):
                confirmation.validate_decision(
                    decision,
                    template,
                    decision["decision_template_sha256"],
                )
            decision = self.approved_decision(template)
            decision["mapping_sha256"] = "e" * 64
            with self.assertRaisesRegex(ValueError, "mapping_sha256"):
                confirmation.validate_decision(
                    decision,
                    template,
                    decision["decision_template_sha256"],
                )
            decision = self.approved_decision(template)
            decision["auto_approve"] = True
            with self.assertRaisesRegex(ValueError, "fields must be exact"):
                confirmation.validate_decision(
                    decision,
                    template,
                    decision["decision_template_sha256"],
                )

    def test_atomic_pair_refuses_drift_and_reuses_identical_outputs(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            output = root / "output.json"
            evidence = root / "evidence.json"
            confirmation.write_pair(output, {"a": 1}, evidence, {"b": 2})
            confirmation.write_pair(output, {"a": 1}, evidence, {"b": 2})
            output.write_text("drift", encoding="utf-8")
            with self.assertRaisesRegex(FileExistsError, "overwrite"):
                confirmation.write_pair(output, {"a": 1}, evidence, {"b": 2})


if __name__ == "__main__":
    unittest.main()
