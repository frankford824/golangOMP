from __future__ import annotations

import copy
import unittest

import apply_bundle_registry_to_mapping as bundle
import reapply_approved_source_bundles as module


class ReapplyApprovedSourceBundlesTest(unittest.TestCase):
    def fixture(self):
        baseline_resources = []
        candidate_resources = []
        confirmed_bundles = []
        for index, (key, member_ids) in enumerate(bundle.EXACT_SCOPES.items(), 1):
            members = [
                {
                    "task_asset_id": member_id,
                    "sha256": f"{member_id:064x}"[-64:],
                    "confirmed": True,
                }
                for member_id in member_ids
            ]
            source_bundle = {
                "format": "zip",
                "task_asset_id": 50000 + index,
                "bundle_sha256": f"{60000 + index:064x}"[-64:],
                "manifest_sha256": f"{70000 + index:064x}"[-64:],
                "members": copy.deepcopy(members),
                "confirmed_by": 1,
                "confirmed_at": "2026-07-24T01:57:49Z",
                "confirmation_note": "approved",
            }
            baseline_revision = {
                "revision_no": key[3],
                "status": "finalized",
                "mode": "single",
                "source_stage": "migration",
                "source_bundle": source_bundle,
                "final_task_asset_ids": [80000 + index],
                "reference_file_ref_ids": [],
                "created_by": 1,
                "reason": "reviewed",
                "confidence": "confirmed_auto",
                "confirmed_by": 1,
                "confirmed_at": "2026-07-24T01:57:49Z",
                "confirmation_note": "approved",
            }
            baseline_revision["manifest_row_hash"] = bundle.revision_hash(
                baseline_revision
            )
            candidate_revision = {
                "revision_no": key[3],
                "status": "finalized",
                "mode": "single",
                "source_stage": "migration",
                "final_task_asset_ids": [80000 + index],
                "reference_file_ref_ids": [],
                "created_by": 1,
                "reason": "reviewed",
                "confidence": "hard_blocked",
                "confirmed_by": 0,
                "confirmed_at": bundle.ZERO_TIME,
                "confirmation_note": "",
                "blockers": [
                    "multiple source assets require a reviewed deterministic ZIP bundle"
                ],
            }
            candidate_revision["manifest_row_hash"] = bundle.revision_hash(
                candidate_revision
            )
            common = {
                "task_id": key[0],
                "scope_kind": key[1],
                "scope_ref_id": key[2],
            }
            baseline_resources.append(
                {**common, "history": [baseline_revision]}
            )
            candidate_resources.append(
                {**common, "history": [candidate_revision]}
            )
            confirmed_bundles.append(
                {
                    **common,
                    "revision_no": key[3],
                    "confirmed": True,
                    "bundle_task_asset_id": source_bundle["task_asset_id"],
                    "ordered_members": copy.deepcopy(members),
                }
            )
        baseline = {"version": 2, "resources": baseline_resources}
        candidate = {"version": 2, "resources": candidate_resources}
        manifest = {
            "schema_version": 1,
            "status": "CONFIRMED",
            "decision_template_sha256": "9" * 64,
            "confirmed_by": 1,
            "confirmed_at": "2026-07-24T01:57:49Z",
            "confirmation_note": "approved",
            "bundles": confirmed_bundles,
        }
        return baseline, candidate, manifest

    def test_reapplies_only_exact_frozen_bundle_rows_as_proposed(self):
        baseline, candidate, manifest = self.fixture()
        output, evidence = module.reapply(
            baseline,
            candidate,
            manifest,
            "9" * 64,
        )
        self.assertEqual(len(evidence), 7)
        for resource in output["resources"]:
            revision = resource["history"][0]
            self.assertEqual(revision["confidence"], "proposed_review")
            self.assertNotIn("blockers", revision)
            self.assertEqual(revision["confirmed_by"], 0)
            self.assertEqual(
                revision["manifest_row_hash"],
                bundle.revision_hash(revision),
            )

    def test_member_order_drift_fails_closed(self):
        baseline, candidate, manifest = self.fixture()
        manifest["bundles"][0]["ordered_members"].reverse()
        with self.assertRaisesRegex(ValueError, "member order drifted"):
            module.reapply(baseline, candidate, manifest, "9" * 64)


if __name__ == "__main__":
    unittest.main()
