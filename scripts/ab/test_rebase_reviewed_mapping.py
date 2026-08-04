import copy
import importlib.util
import json
import pathlib
import tempfile
import unittest


PATH = pathlib.Path(__file__).with_name("rebase_reviewed_mapping.py")
SPEC = importlib.util.spec_from_file_location("review_rebase", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def proposed(row):
    value = {
        **row,
        "confidence": "proposed_review",
        "confirmed_by": 0,
        "confirmed_at": "0001-01-01T00:00:00Z",
        "confirmation_note": "",
    }
    value["manifest_row_hash"] = MODULE.canonical_row_hash(value)
    return value


def confirmed(row):
    value = copy.deepcopy(row)
    value.update(
        {
            "confidence": "confirmed_auto",
            "confirmed_by": 1,
            "confirmed_at": "2026-07-23T00:00:00Z",
            "confirmation_note": "reviewed unchanged business truth",
        }
    )
    value["manifest_row_hash"] = MODULE.canonical_row_hash(value)
    return value


def candidate_mapping():
    revision = proposed(
        {
            "revision_no": 1,
            "status": "finalized",
            "mode": "single",
            "source_stage": "retouch",
            "final_task_asset_ids": [11],
            "reference_file_ref_ids": [21],
            "evidence_event_ids": ["task_event_log:event-1"],
            "review_policy_ids": ["explicit_event_replay"],
            "reason": "unchanged revision",
            "created_by": 3,
            "created_at": "2026-01-01T00:00:00Z",
            "submitted_at": "2026-01-01T00:00:00Z",
            "finalized_at": "2026-01-01T00:00:00Z",
        }
    )
    return {
        "version": 2,
        "resources": [
            {
                "task_id": 7,
                "scope_kind": "retouch_requirement",
                "scope_ref_id": 31,
                "history": [revision],
                "working_revision_no": 1,
                "finalized_revision_no": 1,
            }
        ],
        "planning_tasks": [
            proposed(
                {
                    "task_id": 8,
                    "target_task_status": "Completed",
                    "code_rule_revision_id": 9,
                    "created_by": 3,
                    "review_policy_ids": [
                        "legacy_purchase_to_sku_planning_v1"
                    ],
                    "items": [{"task_sku_item_id": 81}],
                }
            )
        ],
        "asset_recoveries": [
            proposed(
                {
                    "task_id": 12,
                    "missing_task_asset_id": 120,
                    "recovery_source_task_asset_id": 121,
                    "strategy": "clone_b_prematerialized_storage_ref_v1",
                    "review_policy_ids": [
                        "legacy_deleted_asset_recovery_v1"
                    ],
                }
            )
        ],
        "organization_mappings": [
            proposed(
                {
                    "subject_type": "task",
                    "subject_id": 9,
                    "target_department_id": 3,
                    "target_team_id": 14,
                    "review_policy_ids": [
                        "legacy_uat_orphan_org_to_unassigned_v1"
                    ],
                }
            )
        ],
        "access_decisions": [
            proposed(
                {
                    "user_id": 10,
                    "legacy_role": "Warehouse",
                    "action": "no_new_grant",
                    "required_existing_assignments": [
                        {
                            "role_code": "member",
                            "scope_mode": "self",
                            "source_type": "direct",
                            "source_ref_id": 0,
                        }
                    ],
                    "review_policy_ids": [
                        "retired_warehouse_no_new_grant_v1"
                    ],
                }
            )
        ],
        "task_state_decisions": [
            proposed(
                {
                    "task_id": 11,
                    "from_status": "Completed",
                    "target_status": "InProgress",
                    "evidence_event_ids": ["task_event_log:event-11"],
                    "review_policy_ids": [
                        "legacy_retouch_premature_terminal_partial_v1"
                    ],
                }
            )
        ],
    }


def reviewed_baseline(candidate):
    baseline = copy.deepcopy(candidate)
    for resource in baseline["resources"]:
        resource["history"] = [
            confirmed(revision) for revision in resource["history"]
        ]
    for field in (
        "planning_tasks",
        "asset_recoveries",
        "organization_mappings",
        "access_decisions",
        "task_state_decisions",
    ):
        baseline[field] = [confirmed(row) for row in baseline[field]]
    return baseline


class ReviewRebaseTest(unittest.TestCase):
    def test_inherits_all_supported_unchanged_confirmed_rows(self):
        candidate = candidate_mapping()
        baseline = reviewed_baseline(candidate)
        reviewed, evidence = MODULE.rebase_mapping(baseline, candidate)

        inherited = [
            reviewed["resources"][0]["history"][0],
            reviewed["planning_tasks"][0],
            reviewed["asset_recoveries"][0],
            reviewed["organization_mappings"][0],
            reviewed["access_decisions"][0],
            reviewed["task_state_decisions"][0],
        ]
        self.assertTrue(
            all(row["confidence"] == "confirmed_auto" for row in inherited)
        )
        self.assertTrue(all(row["confirmed_by"] == 1 for row in inherited))
        self.assertTrue(
            all(
                row["manifest_row_hash"] == MODULE.canonical_row_hash(row)
                for row in inherited
            )
        )
        self.assertEqual(evidence["counts"]["revision.inherited"], 1)
        self.assertEqual(evidence["counts"]["planning.inherited"], 1)
        self.assertEqual(evidence["counts"]["asset_recovery.inherited"], 1)
        self.assertEqual(evidence["counts"]["organization.inherited"], 1)
        self.assertEqual(evidence["counts"]["access.inherited"], 1)
        self.assertEqual(evidence["counts"]["task_state.inherited"], 1)
        self.assertEqual(
            evidence["candidate_mapping_sha256"],
            MODULE.sha256_json(candidate),
        )

    def test_business_change_and_nonconfirmed_baseline_do_not_inherit(self):
        candidate = candidate_mapping()
        baseline = reviewed_baseline(candidate)
        candidate["planning_tasks"][0]["target_task_status"] = "Cancelled"
        candidate["planning_tasks"][0]["manifest_row_hash"] = (
            MODULE.canonical_row_hash(candidate["planning_tasks"][0])
        )
        baseline["organization_mappings"][0] = proposed(
            MODULE.business_content(baseline["organization_mappings"][0])
        )

        reviewed, evidence = MODULE.rebase_mapping(baseline, candidate)
        self.assertEqual(
            reviewed["planning_tasks"][0]["confidence"], "proposed_review"
        )
        self.assertEqual(
            reviewed["organization_mappings"][0]["confidence"],
            "proposed_review",
        )
        self.assertEqual(evidence["counts"]["planning.business_changed"], 1)
        self.assertEqual(
            evidence["counts"]["organization.baseline_not_confirmed"], 1
        )

    def test_hard_sibling_blocks_entire_resource(self):
        candidate = candidate_mapping()
        baseline = reviewed_baseline(candidate)
        hard_sibling = proposed(
            {
                **MODULE.business_content(
                    candidate["resources"][0]["history"][0]
                ),
                "revision_no": 2,
                "reason": "unresolved sibling",
                "blockers": ["unresolved asset"],
            }
        )
        hard_sibling["confidence"] = "hard_blocked"
        hard_sibling["manifest_row_hash"] = MODULE.canonical_row_hash(
            hard_sibling
        )
        candidate["resources"][0]["history"].append(hard_sibling)

        reviewed, evidence = MODULE.rebase_mapping(baseline, candidate)
        self.assertEqual(
            reviewed["resources"][0]["history"][0]["confidence"],
            "proposed_review",
        )
        self.assertEqual(evidence["counts"]["revision.hard_sibling"], 1)

    def test_pointer_context_change_blocks_revision_inheritance(self):
        candidate = candidate_mapping()
        baseline = reviewed_baseline(candidate)
        candidate["resources"][0]["working_revision_no"] = None

        reviewed, evidence = MODULE.rebase_mapping(baseline, candidate)
        self.assertEqual(
            reviewed["resources"][0]["history"][0]["confidence"],
            "proposed_review",
        )
        self.assertEqual(
            evidence["counts"]["revision.pointer_context_changed"], 1
        )

    def test_pointer_diagnostic_does_not_hide_unconfirmed_baseline(self):
        candidate = candidate_mapping()
        baseline = candidate_mapping()
        candidate["resources"][0]["working_revision_no"] = None

        reviewed, evidence = MODULE.rebase_mapping(baseline, candidate)
        self.assertEqual(
            reviewed["resources"][0]["history"][0]["confidence"],
            "proposed_review",
        )
        self.assertEqual(
            evidence["counts"]["revision.baseline_not_confirmed"], 1
        )
        self.assertNotIn(
            "revision.pointer_context_changed", evidence["counts"]
        )

    def test_cli_writes_mapping_and_evidence_without_overwriting_inputs(self):
        candidate = candidate_mapping()
        baseline = reviewed_baseline(candidate)
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            baseline_path = root / "baseline.json"
            candidate_path = root / "candidate.json"
            output_path = root / "reviewed.json"
            evidence_path = root / "evidence.json"
            baseline_path.write_text(json.dumps(baseline), encoding="utf-8")
            candidate_path.write_text(json.dumps(candidate), encoding="utf-8")

            result = MODULE.main(
                [
                    "--baseline",
                    str(baseline_path),
                    "--candidate",
                    str(candidate_path),
                    "--output",
                    str(output_path),
                    "--evidence-output",
                    str(evidence_path),
                ]
            )
            self.assertEqual(result, 0)
            reviewed = json.loads(output_path.read_text(encoding="utf-8"))
            evidence = json.loads(evidence_path.read_text(encoding="utf-8"))
            self.assertEqual(
                reviewed["resources"][0]["history"][0]["confidence"],
                "confirmed_auto",
            )
            self.assertEqual(
                evidence["reviewed_file_sha256"],
                MODULE.sha256_bytes(output_path.read_bytes()),
            )
            self.assertEqual(
                json.loads(candidate_path.read_text(encoding="utf-8")),
                candidate,
            )
            with self.assertRaisesRegex(
                ValueError, "output must not overwrite an input"
            ):
                MODULE.main(
                    [
                        "--baseline",
                        str(baseline_path),
                        "--candidate",
                        str(candidate_path),
                        "--output",
                        str(candidate_path),
                        "--evidence-output",
                        str(evidence_path),
                    ]
                )

    def test_rejects_stale_input_hash(self):
        candidate = candidate_mapping()
        baseline = reviewed_baseline(candidate)
        candidate["access_decisions"][0]["action"] = "preserve_existing"
        with self.assertRaisesRegex(ValueError, "stale or invalid"):
            MODULE.rebase_mapping(baseline, candidate)


if __name__ == "__main__":
    unittest.main()
