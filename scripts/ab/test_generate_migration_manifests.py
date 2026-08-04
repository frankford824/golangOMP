import importlib.util
import pathlib
import tempfile
import unittest

PATH = pathlib.Path(__file__).with_name("generate_migration_manifests.py")
SPEC = importlib.util.spec_from_file_location("manifest_generator", PATH)
MODULE = importlib.util.module_from_spec(SPEC); SPEC.loader.exec_module(MODULE)


class GeneratorTest(unittest.TestCase):
    def timezone_truth(self):
        return {"timezone_truth": [{
            "system_time_zone": "UTC",
            "matched_asset_created_events": 21305,
            "near_eight_hour_asset_created_events": 21305,
            "asset_created_min_delta_seconds": 28800,
            "asset_created_max_delta_seconds": 28802,
            "superseded_pairs": 1718,
            "near_eight_hour_superseded_pairs": 1718,
            "superseded_min_delta_seconds": 28799,
            "superseded_max_delta_seconds": 28801,
            "rejected_at_count": 26,
            "deleted_at_count": 9,
        }]}

    def test_timezone_truth_requires_one_uniform_proven_legacy_cohort(self):
        truth = self.timezone_truth()
        self.assertEqual(MODULE.validate_legacy_timezone_truth(truth)["matched_asset_created_events"], 21305)
        bounded_latency = self.timezone_truth()
        bounded_latency["timezone_truth"][0]["asset_created_max_delta_seconds"] = 28805
        self.assertEqual(
            MODULE.validate_legacy_timezone_truth(bounded_latency)[
                "asset_created_max_delta_seconds"
            ],
            28805,
        )
        mixed = self.timezone_truth()
        mixed["timezone_truth"][0]["near_eight_hour_asset_created_events"] -= 1
        with self.assertRaisesRegex(RuntimeError, "refusing blanket normalization"):
            MODULE.validate_legacy_timezone_truth(mixed)
        out_of_bound = self.timezone_truth()
        out_of_bound["timezone_truth"][0]["asset_created_max_delta_seconds"] = 28811
        with self.assertRaisesRegex(RuntimeError, "refusing blanket normalization"):
            MODULE.validate_legacy_timezone_truth(out_of_bound)
        non_utc = self.timezone_truth()
        non_utc["timezone_truth"][0]["system_time_zone"] = "CST"
        with self.assertRaisesRegex(RuntimeError, "system_time_zone"):
            MODULE.validate_legacy_timezone_truth(non_utc)

    def test_planning_rule_truth_is_frozen_to_the_reviewed_revision(self):
        truth = {"planning_rule_truth": [{
            "rule_id": 6, "rule_type": "sku_planning", "is_enabled": 0,
            "active_revision_id": 9, "revision_id": 9, "version_no": 1,
        }]}
        self.assertEqual(MODULE.validate_planning_rule_truth(truth)["revision_id"], 9)
        drift = {"planning_rule_truth": [{**truth["planning_rule_truth"][0], "active_revision_id": 10}]}
        with self.assertRaisesRegex(RuntimeError, "refusing frozen revision 9"):
            MODULE.validate_planning_rule_truth(drift)

    def test_clone_query_normalizes_only_proven_legacy_wall_clock_fields(self):
        sql = MODULE.SQL
        self.assertIn("'timezone_truth\\t'", sql)
        self.assertIn("'planning_rule_truth\\t'", sql)
        self.assertIn("DATE_SUB(e.created_at,INTERVAL 8 HOUR)", sql)
        self.assertIn("DATE_SUB(ta.rejected_at,INTERVAL 8 HOUR)", sql)
        self.assertIn("DATE_SUB(ta.superseded_at,INTERVAL 8 HOUR)", sql)
        self.assertIn("DATE_SUB(ta.deleted_at,INTERVAL 8 HOUR)", sql)
        self.assertIn("'uploaded_at_legacy_raw'", sql)
        self.assertNotIn("'uploaded_at',DATE_FORMAT(ta.uploaded_at", sql)
        self.assertIn("'created_at',DATE_FORMAT(ta.created_at", sql)
        self.assertIn("'created_at',DATE_FORMAT(e.created_at", sql)
        self.assertNotIn("DATE_SUB(ta.created_at", sql)
        self.assertNotIn("DATE_SUB(ta.cleaned_at", sql)
        self.assertNotIn("DATE_SUB(ta.access_revoked_at", sql)
        self.assertNotIn("DATE_SUB(ta.object_deleted_at", sql)

    def test_organization_mapping_uses_stable_alias_and_hard_blocks_unknown_org(self):
        rows = {
            "org_departments": [
                {"id": 6, "name": "运营部", "enabled": 1},
                {"id": 14, "name": "视觉研创部", "enabled": 1},
            ],
            "org_teams": [
                {
                    "id": 30,
                    "department_id": 6,
                    "name": "拼多多运营二部（池州)",
                    "enabled": 1,
                },
                {"id": 31, "department_id": 14, "name": "默认组", "enabled": 1},
            ],
            "tasks_org": [
                {
                    "id": 7,
                    "legacy_department": "运营部",
                    "legacy_team": "拼多多池州组",
                    "department_id": None,
                    "team_id": None,
                },
                {
                    "id": 8,
                    "legacy_department": "总经办",
                    "legacy_team": "总经办组",
                    "department_id": None,
                    "team_id": None,
                },
            ],
            "users_org": [
                {
                    "id": 9,
                    "legacy_department": "设计研发部",
                    "legacy_team": "默认组",
                    "department_id": None,
                    "team_id": None,
                    "status": "active",
                }
            ],
        }
        mappings, manual = MODULE.build_organization_mappings(rows)
        by_subject = {
            (item["subject_type"], item["subject_id"]): item for item in mappings
        }

        pdd = by_subject[("task", 7)]
        self.assertEqual(
            (pdd["target_department_id"], pdd["target_team_id"]), (6, 30)
        )
        self.assertEqual(pdd["confidence"], "proposed_review")
        self.assertEqual(
            pdd["review_policy_ids"], [MODULE.ORG_ALIAS_LINEAGE_POLICY]
        )
        self.assertEqual(
            pdd["manifest_row_hash"],
            MODULE.sha256_json(
                {key: value for key, value in pdd.items() if key != "manifest_row_hash"}
            ),
        )

        design = by_subject[("user", 9)]
        self.assertEqual(
            (design["target_department_id"], design["target_team_id"]), (14, 31)
        )
        self.assertEqual(design["confidence"], "proposed_review")

        unknown = by_subject[("task", 8)]
        self.assertEqual(unknown["confidence"], "hard_blocked")
        self.assertEqual(
            (unknown["target_department_id"], unknown["target_team_id"]), (0, 0)
        )
        self.assertTrue(unknown["blockers"])
        self.assertEqual(len(manual), 3)

    def test_access_decisions_preserve_exact_assignments_and_never_guess_roles(self):
        rows = {
            "users_org": [
                {
                    "id": user_id,
                    "status": "active",
                    "department_id": 6,
                    "team_id": 30,
                }
                for user_id in (31, 226, 233, 234)
            ],
            "legacy_roles": [
                {"user_id": 31, "role": "Warehouse"},
                {"user_id": 226, "role": "OrgAdmin"},
                {"user_id": 233, "role": "Outsource"},
                {"user_id": 234, "role": "OrgAdmin"},
            ],
            "access_assignments": [
                {
                    "user_id": 31,
                    "role_code": "member",
                    "scope_mode": "self",
                    "source_type": "direct",
                    "source_ref_id": 0,
                },
                {
                    "user_id": 226,
                    "role_code": "super_admin",
                    "scope_mode": "global",
                    "source_type": "direct",
                    "source_ref_id": 0,
                },
                {
                    "user_id": 233,
                    "role_code": "member",
                    "scope_mode": "self",
                    "source_type": "direct",
                    "source_ref_id": 0,
                },
            ],
        }
        decisions, manual = MODULE.build_access_decisions(rows)
        by_role = {
            (item["user_id"], item["legacy_role"]): item for item in decisions
        }

        warehouse = by_role[(31, "Warehouse")]
        self.assertEqual(warehouse["action"], "no_new_grant")
        self.assertEqual(warehouse["confidence"], "proposed_review")
        self.assertEqual(
            warehouse["review_policy_ids"], [MODULE.WAREHOUSE_NO_GRANT_POLICY]
        )

        org_admin = by_role[(226, "OrgAdmin")]
        self.assertEqual(org_admin["action"], "preserve_existing")
        self.assertEqual(org_admin["confidence"], "proposed_review")
        self.assertEqual(
            org_admin["required_existing_assignments"],
            [
                {
                    "role_code": "super_admin",
                    "scope_mode": "global",
                    "source_type": "direct",
                    "source_ref_id": 0,
                }
            ],
        )

        outsource = by_role[(233, "Outsource")]
        self.assertEqual(outsource["action"], "no_new_grant")
        self.assertEqual(outsource["confidence"], "proposed_review")
        self.assertEqual(
            outsource["review_policy_ids"],
            [MODULE.EXISTING_ACCESS_PRESERVED_POLICY],
        )

        unassigned_admin = by_role[(234, "OrgAdmin")]
        self.assertEqual(unassigned_admin["confidence"], "hard_blocked")
        self.assertIn(
            "user has no explicit V8 assignment evidence",
            unassigned_admin["blockers"],
        )
        self.assertEqual(len(manual), 4)
        for decision in decisions:
            self.assertEqual(
                decision["manifest_row_hash"],
                MODULE.sha256_json(
                    {
                        key: value
                        for key, value in decision.items()
                        if key != "manifest_row_hash"
                    }
                ),
            )

    def test_access_decision_accepts_separately_reviewed_org_scope(self):
        rows = {
            "users_org": [{
                "id": 340,
                "status": "active",
                "department_id": None,
                "team_id": None,
            }],
            "legacy_roles": [{
                "user_id": 340,
                "role": "DepartmentAdmin",
            }],
            "access_assignments": [],
        }
        organization_mappings = [{
            "subject_type": "user",
            "subject_id": 340,
            "target_department_id": 12,
            "target_team_id": 38,
            "confidence": "confirmed_auto",
        }]

        decisions, manual = MODULE.build_access_decisions(
            rows,
            organization_mappings,
        )

        self.assertEqual(len(decisions), 1)
        self.assertEqual(decisions[0]["action"], "no_new_grant")
        self.assertEqual(
            decisions[0]["review_policy_ids"],
            [MODULE.EXISTING_ACCESS_PRESERVED_POLICY],
        )
        self.assertEqual(decisions[0]["required_existing_assignments"], [])
        self.assertEqual(decisions[0]["confidence"], "proposed_review")
        self.assertEqual(len(manual), 1)

    def test_access_decisions_use_reviewable_stable_org_and_allow_empty_no_grant(self):
        rows = {
            "users_org": [
                {
                    "id": 340,
                    "status": "active",
                    "department_id": None,
                    "team_id": None,
                },
                {
                    "id": 341,
                    "status": "active",
                    "department_id": None,
                    "team_id": None,
                },
            ],
            "legacy_roles": [
                {"user_id": 340, "role": "DepartmentAdmin"},
                {"user_id": 341, "role": "Warehouse"},
            ],
            "access_assignments": [],
        }
        organization_mappings = [
            {
                "subject_type": "user",
                "subject_id": 340,
                "target_department_id": 12,
                "target_team_id": 38,
                "confidence": "proposed_review",
            },
            {
                "subject_type": "user",
                "subject_id": 341,
                "target_department_id": 11,
                "target_team_id": 32,
                "confidence": "proposed_review",
            },
        ]

        decisions, _ = MODULE.build_access_decisions(
            rows, organization_mappings
        )

        self.assertEqual(len(decisions), 2)
        by_role = {
            (decision["user_id"], decision["legacy_role"]): decision
            for decision in decisions
        }
        department_admin = by_role[(340, "DepartmentAdmin")]
        self.assertEqual(department_admin["action"], "no_new_grant")
        self.assertEqual(department_admin["confidence"], "proposed_review")
        self.assertNotIn("blockers", department_admin)
        warehouse = by_role[(341, "Warehouse")]
        self.assertEqual(
            (warehouse["user_id"], warehouse["legacy_role"]),
            (341, "Warehouse"),
        )
        self.assertEqual(warehouse["action"], "no_new_grant")
        self.assertEqual(warehouse["confidence"], "proposed_review")
        self.assertNotIn("blockers", warehouse)

    def test_enumerated_uat_orphan_org_uses_unassigned_sink_not_alias(self):
        rows = {
            "org_departments": [
                {"id": 3, "name": "未分配", "enabled": 1},
            ],
            "org_teams": [
                {
                    "id": 14,
                    "department_id": 3,
                    "name": "未分配池",
                    "enabled": 1,
                },
            ],
            "tasks_org": [
                {
                    "id": 463,
                    "legacy_department": "总经办",
                    "legacy_team": "总经办组",
                    "department_id": None,
                    "team_id": None,
                },
            ],
            "users_org": [],
        }
        mappings, _ = MODULE.build_organization_mappings(rows)
        self.assertEqual(len(mappings), 1)
        mapping = mappings[0]
        self.assertEqual(
            (mapping["target_department_id"], mapping["target_team_id"]),
            (3, 14),
        )
        self.assertEqual(mapping["confidence"], "proposed_review")
        self.assertEqual(
            mapping["review_policy_ids"], [MODULE.UAT_ORPHAN_ORG_POLICY]
        )

    def sample(self):
        return {"scopes": [{"task_id": 7, "scope_kind": "task", "scope_ref_id": 0, "sku_code": ""}], "assets": [
            {"id": 10, "asset_id": 100, "task_id": 7, "asset_type": "source", "scope_sku_code": "", "retouch_requirement_id": None, "upload_status": "uploaded", "source_module_key": "design", "flow_review_status": "not_applicable", "superseded_by_version_id": None, "uploaded_at": "2026-01-01T00:00:00Z", "created_at": "2026-01-01T00:00:00Z", "whole_hash": "a" * 64},
            {"id": 11, "asset_id": 101, "task_id": 7, "asset_type": "delivery", "scope_sku_code": "", "retouch_requirement_id": None, "upload_status": "uploaded", "source_module_key": "design", "flow_review_status": "pending_review", "superseded_by_version_id": None, "uploaded_at": "2026-01-01T06:00:00Z", "created_at": "2026-01-01T06:00:00Z", "whole_hash": "b" * 64}], "references": [], "events": [
            {"namespace": "task_event_log", "id": "one", "task_id": 7, "sequence": 2, "event_type": "task.design.submitted", "actor_id": 3, "module_key": "design", "payload": {"upload_session_id": "design-1"}, "created_at": "2026-01-01T12:00:00Z"},
            {"namespace": "task_event_log", "id": "two", "task_id": 7, "sequence": 3, "event_type": "task.audit.approved", "actor_id": 4, "module_key": "audit", "created_at": "2026-01-03T00:00:00Z"},
            {"namespace": "task_event_log", "id": "upload", "task_id": 7, "sequence": 1, "event_type": "task.asset.upload_session.completed", "actor_id": 3, "module_key": "design", "payload": {"upload_session_id": "design-1", "asset_version_ids": [10, 11]}, "created_at": "2026-01-01T08:00:00Z"}], "planning_blockers": []}

    def test_no_change_approval_finalizes_original_submission(self):
        mapping, manual, _ = MODULE.generate(self.sample()); history = mapping["resources"][0]["history"]
        self.assertEqual([r["status"] for r in history], ["finalized"])
        self.assertEqual(history[0]["evidence_event_ids"], ["task_event_log:upload", "task_event_log:one", "task_event_log:two"])
        self.assertEqual(mapping["resources"][0]["working_revision_no"], 1)
        self.assertEqual(mapping["resources"][0]["finalized_revision_no"], 1)
        self.assertTrue(all(r["confidence"] == "proposed_review" for r in history)); self.assertEqual(len(manual), 1)

    def customization_terminal_rows(self, task_id=452):
        scopes = [{
            "task_id": task_id,
            "task_status": "PendingWarehouseReceive",
            "scope_kind": "task",
            "scope_ref_id": 0,
            "sku_code": "",
        }]
        assets = []
        if task_id == 452:
            assets = [{
                "id": 207,
                "asset_id": 207,
                "task_id": task_id,
                "asset_type": "source",
                "scope_sku_code": "",
                "retouch_requirement_id": None,
                "upload_status": "uploaded",
                "source_module_key": "customization",
                "flow_review_status": "not_applicable",
                "superseded_by_version_id": None,
                "created_at": "2026-04-18T10:19:01Z",
                "whole_hash": "a" * 64,
            }]
        return {
            "scopes": scopes,
            "assets": assets,
            "references": [],
            "events": [{
                "namespace": "task_event_log",
                "id": f"customization-{task_id}",
                "task_id": task_id,
                "sequence": 7,
                "event_type": "task.customization.reviewed",
                "actor_id": 303,
                "module_key": "customization",
                "payload": {
                    "customization_review_decision": "approved",
                    "from_task_status": "PendingCustomizationReview",
                    "to_task_status": "PendingWarehouseReceive",
                },
                "created_at": "2026-06-02T03:47:00Z",
            }],
        }

    def test_incomplete_customization_terminal_reopens_exact_allowlisted_draft(self):
        mapping, manual, _ = MODULE.generate(
            self.customization_terminal_rows()
        )
        resource = mapping["resources"][0]
        revision = resource["history"][0]
        self.assertEqual(resource["working_revision_no"], 1)
        self.assertNotIn("finalized_revision_no", resource)
        self.assertEqual((revision["status"], revision["source_stage"]), ("draft", "reopen"))
        self.assertEqual(revision["source_task_asset_id"], 207)
        self.assertEqual(revision["final_task_asset_ids"], [])
        self.assertEqual(revision["confidence"], "proposed_review")
        self.assertEqual(
            revision["review_policy_ids"],
            [
                MODULE.EXPLICIT_EVENT_REPLAY_POLICY,
                MODULE.REOPEN_POLICY,
                MODULE.CUSTOMIZATION_TERMINAL_WITHOUT_ASSETS_POLICY,
            ],
        )
        decision = mapping["task_state_decisions"][0]
        self.assertEqual(
            (
                decision["task_id"],
                decision["from_status"],
                decision["target_status"],
                decision["confidence"],
            ),
            (452, "PendingWarehouseReceive", "InProgress", "proposed_review"),
        )
        self.assertEqual(
            decision["review_policy_ids"],
            [MODULE.CUSTOMIZATION_TERMINAL_WITHOUT_ASSETS_POLICY],
        )
        self.assertEqual(
            len([row for row in manual if row["task_id"] == 452]),
            2,
        )

    def test_incomplete_customization_terminal_policy_fails_closed_on_asset_drift(self):
        rows = self.customization_terminal_rows()
        rows["assets"].append({
            **rows["assets"][0],
            "id": 208,
            "asset_type": "delivery",
            "created_at": "2026-04-18T10:20:01Z",
        })
        mapping, _, _ = MODULE.generate(rows)
        self.assertEqual(mapping["task_state_decisions"], [])
        self.assertTrue(
            any(
                revision["confidence"] == "hard_blocked"
                for revision in mapping["resources"][0]["history"]
            )
        )

    def test_multiple_sources_are_hard_blocked(self):
        rows = self.sample(); rows["assets"].append({**rows["assets"][0], "id": 12})
        rows["events"][2]["payload"]["asset_version_ids"].append(12)
        mapping, manual, _ = MODULE.generate(rows)
        self.assertEqual(mapping["resources"][0]["history"][0]["confidence"], "hard_blocked"); self.assertIn("ZIP", manual[0]["reason"])

    def test_atomic_submit_never_pulls_a_future_upload_into_the_snapshot(self):
        rows = self.sample()
        rows["assets"].append({
            **rows["assets"][1],
            "id": 12,
            "asset_id": 102,
            "created_at": "2026-01-01T12:10:00Z",
            "uploaded_at": "2026-01-01T12:10:00Z",
        })
        rows["events"].append({
            "namespace": "task_event_log",
            "id": "future-upload",
            "task_id": 7,
            "sequence": 4,
            "event_type": "task.asset.upload_session.completed",
            "actor_id": 3,
            "module_key": "design",
            "payload": {
                "upload_session_id": "design-2",
                "asset_version_id": 12,
            },
            "created_at": "2026-01-01T12:10:00Z",
        })

        selected, evidence, blockers = MODULE.resolve_submission_assets(
            rows["scopes"][0],
            rows["events"][0],
            rows["events"],
            rows["assets"],
        )

        self.assertEqual([asset["id"] for asset in selected], [10, 11])
        self.assertEqual(evidence, ["task_event_log:upload"])
        self.assertEqual(blockers, [])
        self.assertNotIn("_atomic_upload_batch", rows["events"][0])

    def test_successor_replay_deduplicates_same_root_final_members(self):
        scope = self.sample()["scopes"][0]
        old = {
            **self.sample()["assets"][1],
            "id": 11,
            "asset_id": 101,
            "flow_review_status": "superseded",
            "superseded_by_version_id": 12,
            "superseded_at": "2026-01-02T00:00:00Z",
        }
        successor = {
            **old,
            "id": 12,
            "flow_review_status": "approved",
            "superseded_by_version_id": None,
            "superseded_at": "",
            "created_at": "2026-01-02T00:00:00Z",
        }
        completion = {
            "namespace": "task_event_log",
            "id": "replacement-upload",
            "task_id": 7,
            "sequence": 4,
            "event_type": "task.asset.upload_session.completed",
            "actor_id": 4,
            "payload": {"asset_version_id": 12},
            "created_at": "2026-01-02T00:00:00Z",
        }
        approval = {
            "namespace": "task_event_log",
            "id": "approval",
            "task_id": 7,
            "sequence": 5,
            "event_type": "task.audit.approved",
            "actor_id": 4,
            "created_at": "2026-01-03T00:00:00Z",
        }
        revision = {
            "source_alias_from_task_asset_id": 11,
            "final_task_asset_ids": [11, 12],
            "evidence_event_ids": [],
            "mode": "set",
            "manifest_row_hash": "",
            "_blockers": [],
        }

        changed = MODULE.apply_proven_successor_audit_change(
            scope,
            approval,
            [completion, approval],
            [old, successor],
            revision,
        )

        self.assertTrue(changed)
        self.assertEqual(revision["source_alias_from_task_asset_id"], 12)
        self.assertEqual(revision["final_task_asset_ids"], [12])
        self.assertEqual(revision["mode"], "single")

    def test_optional_successor_probe_does_not_leak_failure_blocker(self):
        scope = self.sample()["scopes"][0]
        old = {
            **self.sample()["assets"][1],
            "id": 11,
            "asset_id": 101,
            "flow_review_status": "approved",
            "deleted_at": "2026-01-02T00:00:00Z",
            "superseded_by_version_id": None,
        }
        completion = {
            "namespace": "task_event_log",
            "id": "cleanup",
            "task_id": 7,
            "sequence": 4,
            "event_type": "task.asset.upload_session.completed",
            "actor_id": 3,
            "module_key": "design",
            "payload": {"asset_version_ids": [11]},
            "created_at": "2026-01-02T00:00:00Z",
        }
        revision = {
            "revision_no": 2,
            "status": "finalized",
            "source_stage": "reopen",
            "source_alias_from_task_asset_id": 11,
            "final_task_asset_ids": [11],
            "reference_file_ref_ids": [],
            "evidence_event_ids": [],
            "mode": "single",
            "manifest_row_hash": "",
            "_blockers": [],
        }
        original = dict(revision)

        changed = MODULE.apply_proven_successor_audit_change_if_possible(
            scope,
            completion,
            [completion],
            [old],
            revision,
        )

        self.assertFalse(changed)
        self.assertEqual(revision, original)

    def test_completed_customization_missing_final_reopens_empty_draft(self):
        scope = {
            "task_id": 3091,
            "task_status": "Completed",
            "scope_kind": "sku",
            "scope_ref_id": 3311,
            "sku_code": "SKU-3091",
        }
        before = {
            "revision_no": 3,
            "status": "finalized",
            "mode": "single",
            "source_stage": "reopen",
            "source_alias_from_task_asset_id": 29144,
            "final_task_asset_ids": [29144],
            "reference_file_ref_ids": [1],
            "evidence_event_ids": ["task_event_log:upload"],
            "confidence": "proposed_review",
            "created_by": 7,
            "created_at": "2026-07-28T03:29:46Z",
            "finalized_at": "2026-07-28T03:29:46Z",
            "manifest_row_hash": "",
        }
        after = {
            **before,
            "source_alias_from_task_asset_id": None,
            "final_task_asset_ids": [],
            "_blockers": [
                "finalized revision has no lifecycle-valid delivery asset"
            ],
        }
        after.pop("source_alias_from_task_asset_id")
        assets = [
            {
                "id": 29144,
                "task_id": 3091,
                "asset_type": "delivery",
                "deleted_at": "2026-07-28T03:30:26Z",
                "storage_ref_id": "a40e151c-7fb0-4dfa-a8a2-624992c2832c",
                "approved_at": "2026-07-28T03:29:46Z",
                "source_module_key": "customization",
                "superseded_by_version_id": None,
            }
        ]

        result = MODULE.reopen_completed_customization_missing_final(
            scope, before, after, assets
        )

        self.assertIsNotNone(result)
        self.assertEqual(result["historical"]["status"], "superseded")
        self.assertEqual(
            result["historical"]["final_task_asset_ids"], [29144]
        )
        self.assertEqual(result["draft"]["revision_no"], 4)
        self.assertEqual(result["draft"]["status"], "draft")
        self.assertEqual(result["draft"]["source_stage"], "reopen")
        self.assertNotIn("source_alias_from_task_asset_id", result["draft"])
        self.assertEqual(result["draft"]["final_task_asset_ids"], [])

    def test_rejection_clones_draft_and_resubmit_mutates_that_draft(self):
        rows = self.sample()
        rows["events"] = [rows["events"][0],
            {"namespace": "task_event_log", "id": "reject", "task_id": 7, "event_type": "task.audit.returned_to_design", "actor_id": 4, "created_at": "2026-01-02T00:00:00Z"},
            {"namespace": "task_event_log", "id": "resubmit", "task_id": 7, "event_type": "task.design.submitted", "actor_id": 3, "created_at": "2026-01-03T00:00:00Z"}]
        mapping, _, _ = MODULE.generate(rows); resource = mapping["resources"][0]
        self.assertEqual([r["status"] for r in resource["history"]], ["rejected", "submitted"])
        self.assertEqual(resource["working_revision_no"], 2)
        self.assertNotIn("finalized_revision_no", resource)
        self.assertEqual(resource["history"][1]["source_stage"], "reopen")

    def test_reopen_resubmit_keeps_upload_completion_evidence(self):
        rows = self.sample()
        rows["events"] = [
            rows["events"][2],
            rows["events"][0],
            {"namespace": "task_event_log", "id": "reject", "task_id": 7,
             "event_type": "task.audit.rejected", "actor_id": 4,
             "created_at": "2026-01-02T00:00:00Z"},
            {"namespace": "task_event_log", "id": "upload-2", "task_id": 7,
             "event_type": "task.asset.upload_session.completed", "actor_id": 3,
             "module_key": "design", "payload": {"upload_session_id": "design-2", "asset_version_ids": [10, 11]},
             "created_at": "2026-01-03T08:00:00Z"},
            {"namespace": "task_event_log", "id": "resubmit", "task_id": 7,
             "event_type": "task.design.submitted", "actor_id": 3,
             "module_key": "design", "payload": {"upload_session_id": "design-2"},
             "created_at": "2026-01-03T12:00:00Z"},
        ]
        mapping, _, _ = MODULE.generate(rows)
        evidence = mapping["resources"][0]["history"][1]["evidence_event_ids"]
        self.assertIn("task_event_log:upload-2", evidence)
        self.assertIn("task_event_log:resubmit", evidence)

    def test_reopen_preserves_old_finalized_pointer(self):
        rows = self.sample(); rows["events"].append(
            {"namespace": "task_event_log", "id": "reopen", "task_id": 7, "event_type": "task.reopened", "actor_id": 5, "created_at": "2026-01-04T00:00:00Z"})
        mapping, _, _ = MODULE.generate(rows); resource = mapping["resources"][0]
        self.assertEqual([r["status"] for r in resource["history"]], ["finalized", "draft"])
        self.assertEqual(resource["finalized_revision_no"], 1); self.assertEqual(resource["working_revision_no"], 2)

    def test_explicit_audit_replacement_creates_audit_revision(self):
        rows = self.sample()
        rows["assets"] += [
            {**rows["assets"][0], "id": 12, "source_module_key": "audit", "uploaded_at": "2026-01-02T00:00:00Z", "created_at": "2026-01-02T00:00:00Z"},
            {**rows["assets"][1], "id": 13, "source_module_key": "audit", "uploaded_at": "2026-01-02T00:00:00Z", "created_at": "2026-01-02T00:00:00Z"},
        ]
        rows["events"][1]["payload"] = {"replace_source_asset_version_id": 12, "replace_final_asset_version_ids": [13]}
        mapping, _, _ = MODULE.generate(rows); history = mapping["resources"][0]["history"]
        self.assertEqual([r["status"] for r in history], ["superseded", "finalized"])
        self.assertEqual(history[1]["source_stage"], "audit")
        self.assertEqual(history[1]["source_task_asset_id"], 12); self.assertEqual(history[1]["final_task_asset_ids"], [13])

    def test_unproven_late_asset_change_is_hard_blocked_not_guessed(self):
        rows = self.sample(); rows["assets"].append({**rows["assets"][1], "id": 14, "source_module_key": "audit", "uploaded_at": "2026-01-02T00:00:00Z", "created_at": "2026-01-02T00:00:00Z"})
        mapping, manual, _ = MODULE.generate(rows)
        self.assertEqual(mapping["resources"][0]["history"][0]["confidence"], "hard_blocked")
        self.assertIn("does not prove replace/append", manual[0]["reason"])

    def test_proven_audit_stage_batch_replaces_design_finals(self):
        rows = self.sample()
        rows["assets"].append({
            **rows["assets"][1],
            "id": 14,
            "asset_id": 104,
            "source_module_key": "audit",
            "upload_session_id": "audit-1",
            "upload_request_id": "audit-1",
            "uploaded_at": "2026-01-02T00:00:00Z",
            "created_at": "2026-01-02T00:00:00Z",
        })
        rows["events"][1]["created_at"] = "2026-01-02T00:05:00Z"
        rows["events"].append({
            "namespace": "task_event_log",
            "id": "audit-upload",
            "task_id": 7,
            "sequence": 4,
            "event_type": "task.asset.upload_session.completed",
            "actor_id": 4,
            "module_key": "audit",
            "payload": {
                "upload_session_id": "audit-1",
                "asset_type": "delivery",
                "asset_version_id": 14,
            },
            "created_at": "2026-01-02T00:00:01Z",
        })

        mapping, _, _ = MODULE.generate(rows)
        history = mapping["resources"][0]["history"]

        self.assertEqual([revision["status"] for revision in history], ["superseded", "finalized"])
        self.assertEqual(history[1]["source_stage"], "audit")
        self.assertEqual(history[1]["source_task_asset_id"], 10)
        self.assertEqual(history[1]["final_task_asset_ids"], [14])
        self.assertIn("task_event_log:audit-upload", history[1]["evidence_event_ids"])
        self.assertIn(
            MODULE.AUDIT_STAGE_FINAL_SNAPSHOT_POLICY,
            history[1]["review_policy_ids"],
        )

    def test_source_only_audit_batch_inherits_submitted_finals(self):
        rows = self.sample()
        rows["assets"].append({
            **rows["assets"][0],
            "id": 14,
            "asset_id": 104,
            "source_module_key": "audit",
            "created_at": "2026-01-02T00:00:00Z",
        })
        rows["events"].append({
            "namespace": "task_event_log",
            "id": "audit-source",
            "task_id": 7,
            "sequence": 4,
            "event_type": "task.asset.upload_session.completed",
            "actor_id": 4,
            "module_key": "audit",
            "payload": {"asset_version_id": 14, "asset_type": "source"},
            "created_at": "2026-01-02T00:00:00Z",
        })

        mapping, _, _ = MODULE.generate(rows)
        history = mapping["resources"][0]["history"]

        self.assertEqual(history[1]["source_task_asset_id"], 14)
        self.assertEqual(history[1]["final_task_asset_ids"], [11])

    def test_delayed_audit_batch_uses_delivery_approval_metadata(self):
        rows = self.sample()
        rows["events"][1]["created_at"] = "2026-01-03T00:00:00Z"
        rows["assets"] += [
            {
                **rows["assets"][0],
                "id": 14,
                "asset_id": 104,
                "source_module_key": "audit",
                "created_at": "2026-01-02T00:00:00Z",
            },
            {
                **rows["assets"][1],
                "id": 15,
                "asset_id": 105,
                "source_module_key": "audit",
                "approved_by": 4,
                "approved_at": "2026-01-03T00:00:00Z",
                "created_at": "2026-01-02T00:00:01Z",
            },
        ]
        rows["events"] += [
            {
                "namespace": "task_event_log",
                "id": "audit-source",
                "task_id": 7,
                "sequence": 4,
                "event_type": "task.asset.upload_session.completed",
                "actor_id": 9,
                "module_key": "audit",
                "payload": {"asset_version_id": 14, "asset_type": "source"},
                "created_at": "2026-01-02T00:00:00Z",
            },
            {
                "namespace": "task_event_log",
                "id": "audit-final",
                "task_id": 7,
                "sequence": 5,
                "event_type": "task.asset.upload_session.completed",
                "actor_id": 9,
                "module_key": "audit",
                "payload": {"asset_version_id": 15, "asset_type": "delivery"},
                "created_at": "2026-01-02T00:00:01Z",
            },
        ]

        mapping, _, _ = MODULE.generate(rows)
        revision = mapping["resources"][0]["history"][1]

        self.assertEqual(revision["source_task_asset_id"], 14)
        self.assertEqual(revision["final_task_asset_ids"], [15])
        self.assertEqual(revision["confidence"], "proposed_review")

    def test_asset_boundaries_use_created_at_not_uploaded_at(self):
        rows = self.sample()
        rows["assets"].append({
            **rows["assets"][1], "id": 14,
            "source_module_key": "audit",
            "created_at": "2026-01-02T00:00:00Z",
            "uploaded_at": "2026-01-01T00:00:00Z",
        })
        mapping, manual, _ = MODULE.generate(rows)
        self.assertEqual(mapping["resources"][0]["history"][0]["confidence"], "hard_blocked")
        self.assertIn("does not prove replace/append", manual[0]["reason"])

    def test_non_source_delivery_asset_does_not_trigger_change_blocker(self):
        rows = self.sample()
        rows["assets"].append({
            **rows["assets"][1], "id": 14, "asset_type": "preview",
            "created_at": "2026-01-02T00:00:00Z",
        })
        mapping, _, _ = MODULE.generate(rows)
        self.assertEqual(mapping["resources"][0]["history"][0]["confidence"], "proposed_review")

    def test_multi_sku_submit_uses_payload_target_or_asset_scope(self):
        rows = self.sample()
        rows["scopes"] = [
            {"task_id": 7, "task_status": "PendingAuditA", "scope_kind": "sku", "scope_ref_id": 21, "sku_code": "SKU-A"},
            {"task_id": 7, "task_status": "PendingAuditA", "scope_kind": "sku", "scope_ref_id": 22, "sku_code": "SKU-B"},
        ]
        rows["assets"] = [
            {**rows["assets"][0], "scope_sku_code": "SKU-A"},
            {**rows["assets"][1], "scope_sku_code": "SKU-A"},
            {**rows["assets"][0], "id": 12, "scope_sku_code": "SKU-B"},
            {**rows["assets"][1], "id": 13, "scope_sku_code": "SKU-B"},
        ]
        rows["events"][0]["payload"] = {"target_sku_code": "SKU-B", "asset_id": 12}
        rows["events"][2]["payload"]["asset_version_ids"] = [12, 13]
        mapping, manual, _ = MODULE.generate(rows)
        self.assertNotIn("task_event_log:one", mapping["resources"][0]["history"][0]["evidence_event_ids"])
        self.assertEqual(mapping["resources"][0]["history"][0]["confidence"], "hard_blocked")
        self.assertEqual(len(mapping["resources"][1]["history"]), 1)
        self.assertTrue(any(row["task_id"] == 7 and row["scope_ref_id"] == 21 and row["confidence"] == "hard_blocked" for row in manual))

    def atomic_batch_sample(self):
        scopes = [
            {"task_id": 7, "task_status": "PendingAuditA", "scope_kind": "sku", "scope_ref_id": 21, "sku_code": "SKU-A"},
            {"task_id": 7, "task_status": "PendingAuditA", "scope_kind": "sku", "scope_ref_id": 22, "sku_code": "SKU-B"},
        ]
        assets = [
            {
                "id": 10, "asset_id": 100, "task_id": 7, "asset_type": "delivery",
                "scope_sku_code": "SKU-A", "retouch_requirement_id": None,
                "upload_session_id": "session-a", "upload_request_id": "session-a",
                "upload_status": "uploaded", "source_module_key": "design",
                "flow_review_status": "pending_review", "superseded_by_version_id": None,
                "created_at": "2026-01-01T08:00:00Z", "whole_hash": "a" * 64,
            },
            {
                "id": 11, "asset_id": 101, "task_id": 7, "asset_type": "delivery",
                "scope_sku_code": "SKU-B", "retouch_requirement_id": None,
                "upload_session_id": "session-b", "upload_request_id": "session-b",
                "upload_status": "uploaded", "source_module_key": "design",
                "flow_review_status": "pending_review", "superseded_by_version_id": None,
                "created_at": "2026-01-01T09:00:00Z", "whole_hash": "b" * 64,
            },
        ]
        events = [
            {
                "namespace": "task_event_log", "id": "upload-a", "task_id": 7,
                "sequence": 1, "event_type": "task.asset.upload_session.completed",
                "actor_id": 3, "module_key": "design",
                "payload": {
                    "upload_session_id": "session-a", "asset_id": 100,
                    "asset_type": "delivery", "asset_version_id": 10,
                    "target_sku_code": "SKU-A",
                },
                "created_at": "2026-01-01T08:00:00Z",
            },
            {
                "namespace": "task_event_log", "id": "upload-b", "task_id": 7,
                "sequence": 2, "event_type": "task.asset.upload_session.completed",
                "actor_id": 3, "module_key": "design",
                "payload": {
                    "upload_session_id": "session-b", "asset_id": 101,
                    "asset_type": "delivery", "asset_version_id": 11,
                    "target_sku_code": "SKU-B",
                },
                "created_at": "2026-01-01T09:00:00Z",
            },
            {
                "namespace": "task_event_log", "id": "submit", "task_id": 7,
                "sequence": 3, "event_type": "task.design.submitted",
                "actor_id": 3, "module_key": "design",
                "payload": {
                    "upload_session_id": "session-b", "asset_id": 101,
                    "asset_type": "delivery", "target_sku_code": "SKU-B",
                },
                "created_at": "2026-01-01T09:00:01Z",
            },
        ]
        return {
            "scopes": scopes, "assets": assets, "references": [], "events": events,
            "planning_rows": [], "warehouse_blockers": [],
        }

    def test_atomic_multi_sku_batch_submit_replays_one_submitted_revision_per_sku(self):
        mapping, manual, _ = MODULE.generate(self.atomic_batch_sample())
        self.assertEqual(len(mapping["resources"]), 2)
        revisions = [resource["history"][0] for resource in mapping["resources"]]
        self.assertEqual([revision["status"] for revision in revisions], ["submitted", "submitted"])
        self.assertEqual([revision["final_task_asset_ids"] for revision in revisions], [[10], [11]])
        self.assertEqual(
            [revision["evidence_event_ids"] for revision in revisions],
            [
                ["task_event_log:upload-a", "task_event_log:submit"],
                ["task_event_log:upload-b", "task_event_log:submit"],
            ],
        )
        self.assertTrue(all(revision["confidence"] == "proposed_review" for revision in revisions))
        self.assertTrue(all(MODULE.BATCH_SUBMIT_POLICY in revision["reason"] for revision in revisions))
        self.assertTrue(all(
            MODULE.BATCH_SUBMIT_POLICY in revision["review_policy_ids"]
            for revision in revisions
        ))
        self.assertTrue(all(
            "the last scoped submit triggers the task-level atomic transition"
            in revision["reason"]
            for revision in revisions
        ))
        self.assertTrue(all(
            "one task-wide submit" not in revision["reason"]
            for revision in revisions
        ))
        self.assertTrue(all(
            revision["manifest_row_hash"] == MODULE.sha256_json({
                key: value
                for key, value in revision.items()
                if key != "manifest_row_hash"
            })
            for revision in revisions
        ))
        self.assertTrue(all(MODULE.BATCH_SUBMIT_POLICY in row["reason"] for row in manual))

    def test_atomic_multi_sku_batch_submit_fails_closed_on_ambiguous_truth(self):
        mutations = {}

        missing = self.atomic_batch_sample()
        missing["events"] = [event for event in missing["events"] if event["id"] != "upload-a"]
        mutations["missing SKU membership"] = missing

        duplicate = self.atomic_batch_sample()
        duplicate["events"].insert(1, {
            **duplicate["events"][0], "id": "upload-a-duplicate",
        })
        mutations["duplicate completion observation"] = duplicate

        actor_conflict = self.atomic_batch_sample()
        actor_conflict["events"][0]["actor_id"] = 4
        mutations["different completion actor"] = actor_conflict

        session_conflict = self.atomic_batch_sample()
        session_conflict["events"][-1]["payload"]["upload_session_id"] = "not-the-final-session"
        mutations["submit session mismatch"] = session_conflict

        for label, rows in mutations.items():
            with self.subTest(label):
                candidate = MODULE.resolve_atomic_multi_sku_batch_submit(
                    rows["scopes"], rows["events"], rows["assets"],
                )
                self.assertIsNone(candidate)
                mapping, manual, _ = MODULE.generate(rows)
                first = mapping["resources"][0]
                self.assertTrue(
                    not first["history"]
                    or all(revision["confidence"] == "hard_blocked" for revision in first["history"])
                )
                self.assertTrue(all(
                    MODULE.BATCH_SUBMIT_POLICY not in revision.get("reason", "")
                    for revision in first["history"]
                ))
                self.assertTrue(any(
                    row["scope_ref_id"] == 21
                    and row["confidence"] == "hard_blocked"
                    for row in manual
                ))

    def test_atomic_multi_sku_batch_accepts_multi_member_final_set(self):
        rows = self.atomic_batch_sample()
        rows["assets"].append({
            **rows["assets"][0],
            "id": 12,
            "asset_id": 102,
            "upload_session_id": "session-a-2",
            "upload_request_id": "session-a-2",
            "created_at": "2026-01-01T08:30:00Z",
        })
        rows["events"].insert(1, {
            **rows["events"][0],
            "id": "upload-a-2",
            "sequence": 2,
            "payload": {
                "upload_session_id": "session-a-2",
                "asset_id": 102,
                "asset_type": "delivery",
                "asset_version_id": 12,
                "target_sku_code": "SKU-A",
            },
            "created_at": "2026-01-01T08:30:00Z",
        })

        mapping, _, _ = MODULE.generate(rows)

        first = mapping["resources"][0]["history"][0]
        self.assertEqual(first["final_task_asset_ids"], [10, 12])
        self.assertEqual(first["source_alias_from_task_asset_id"], 10)
        self.assertEqual(first["confidence"], "proposed_review")

    def test_atomic_multi_sku_batch_multiple_sources_stays_bundle_blocked(self):
        rows = self.atomic_batch_sample()
        for offset, asset_id in enumerate((12, 13), start=1):
            rows["assets"].append({
                **rows["assets"][0],
                "id": asset_id,
                "asset_id": 100 + asset_id,
                "asset_type": "source",
                "upload_session_id": f"source-{asset_id}",
                "upload_request_id": f"source-{asset_id}",
                "created_at": f"2026-01-01T08:0{offset}:00Z",
            })
            rows["events"].insert(offset, {
                **rows["events"][0],
                "id": f"source-{asset_id}",
                "sequence": offset + 1,
                "payload": {
                    "upload_session_id": f"source-{asset_id}",
                    "asset_id": 100 + asset_id,
                    "asset_type": "source",
                    "asset_version_id": asset_id,
                    "target_sku_code": "SKU-A",
                },
                "created_at": f"2026-01-01T08:0{offset}:00Z",
            })

        mapping, _, _ = MODULE.generate(rows)

        first = mapping["resources"][0]["history"][0]
        self.assertEqual(first["confidence"], "hard_blocked")
        self.assertTrue(any(
            "deterministic ZIP bundle" in blocker
            for blocker in first["blockers"]
        ))

    def test_atomic_multi_sku_batch_replays_second_wave_with_inheritance(self):
        rows = self.atomic_batch_sample()
        rows["events"].append({
            "namespace": "task_event_log",
            "id": "reject",
            "task_id": 7,
            "sequence": 4,
            "event_type": "task.audit.rejected",
            "actor_id": 5,
            "module_key": "audit",
            "payload": {},
            "created_at": "2026-01-01T10:00:00Z",
        })
        rows["assets"].append({
            **rows["assets"][0],
            "id": 12,
            "asset_id": 102,
            "upload_session_id": "session-a-2",
            "upload_request_id": "session-a-2",
            "created_at": "2026-01-01T11:00:00Z",
        })
        rows["events"] += [
            {
                **rows["events"][0],
                "id": "upload-a-2",
                "sequence": 5,
                "payload": {
                    "upload_session_id": "session-a-2",
                    "asset_id": 102,
                    "asset_type": "delivery",
                    "asset_version_id": 12,
                    "target_sku_code": "SKU-A",
                },
                "created_at": "2026-01-01T11:00:00Z",
            },
            {
                **rows["events"][2],
                "id": "submit-2",
                "sequence": 6,
                "payload": {
                    "upload_session_id": "session-a-2",
                    "asset_id": 102,
                    "asset_type": "delivery",
                    "target_sku_code": "SKU-A",
                },
                "created_at": "2026-01-01T11:00:01Z",
            },
        ]

        mapping, _, _ = MODULE.generate(rows)

        first, second = mapping["resources"]
        self.assertEqual(
            [revision["final_task_asset_ids"] for revision in first["history"]],
            [[10], [12]],
        )
        self.assertEqual(
            [revision["final_task_asset_ids"] for revision in second["history"]],
            [[11], [11]],
        )

    def test_atomic_multi_sku_batch_submit_replays_later_approval_for_every_sku(self):
        rows = self.atomic_batch_sample()
        for scope in rows["scopes"]:
            scope["task_status"] = "Completed"
        rows["events"].append({
            "namespace": "task_event_log", "id": "approve", "task_id": 7,
            "sequence": 4, "event_type": "task.audit.approved", "actor_id": 5,
            "module_key": "audit", "payload": {},
            "created_at": "2026-01-02T00:00:00Z",
        })
        rows["events"].append({
            "namespace": "task_event_log", "id": "close", "task_id": 7,
            "sequence": 5, "event_type": "task.closed", "actor_id": 5,
            "module_key": "", "payload": {},
            "created_at": "2026-01-02T00:01:00Z",
        })

        mapping, _, _ = MODULE.generate(rows)

        self.assertEqual(len(mapping["resources"]), 2)
        for resource, expected_final in zip(mapping["resources"], ([10], [11])):
            self.assertEqual(len(resource["history"]), 1)
            revision = resource["history"][0]
            self.assertEqual(revision["status"], "finalized")
            self.assertEqual(revision["final_task_asset_ids"], expected_final)
            self.assertEqual(
                revision["evidence_event_ids"][-2:],
                ["task_event_log:approve", "task_event_log:close"],
            )
            self.assertIn(MODULE.BATCH_SUBMIT_POLICY, revision["review_policy_ids"])

    def test_module_events_are_module_filtered_and_deduplicated_against_task_events(self):
        rows = self.sample()
        rows["events"].insert(1, {
            **rows["events"][0], "namespace": "task_module_event", "id": 99, "module_key": "design",
        })
        rows["events"].insert(2, {
            **rows["events"][0], "namespace": "task_module_event", "id": 100, "module_key": "warehouse",
        })
        mapping, _, _ = MODULE.generate(rows)
        self.assertIn("task_event_log:one", mapping["resources"][0]["history"][0]["evidence_event_ids"])
        self.assertNotIn("task_module_event:99", mapping["resources"][0]["history"][0]["evidence_event_ids"])
        self.assertEqual(len(mapping["resources"][0]["history"]), 1)

    def test_customization_review_decision_is_used(self):
        rows = self.sample()
        rows["events"][1].update({
            "event_type": "task.customization.reviewed",
            "payload": {"customization_review_decision": "approved"},
        })
        mapping, _, _ = MODULE.generate(rows)
        self.assertEqual(mapping["resources"][0]["history"][0]["status"], "finalized")

    def test_customization_return_to_designer_is_rejection(self):
        rows = self.sample()
        rows["events"][1].update({
            "event_type": "task.customization.reviewed",
            "payload": {"customization_review_decision": "return_to_designer"},
        })
        mapping, _, _ = MODULE.generate(rows)
        self.assertEqual([row["status"] for row in mapping["resources"][0]["history"]], ["rejected", "draft"])

    def test_customization_production_gate_is_not_resource_approval(self):
        rows = self.sample()
        rows["scopes"][0]["task_status"] = "PendingCustomizationProduction"
        rows["events"] = [{
            "namespace": "task_event_log",
            "id": "gate",
            "task_id": 7,
            "sequence": 1,
            "event_type": "task.customization.reviewed",
            "actor_id": 4,
            "payload": {
                "customization_review_decision": "approved",
                "to_task_status": "PendingCustomizationProduction",
            },
            "created_at": "2026-01-01T00:00:00Z",
        }]

        mapping, manual, _ = MODULE.generate(rows)

        self.assertEqual(mapping["resources"][0]["history"], [])
        self.assertEqual(manual[0]["confidence"], "proposed_review")

    def test_customization_rejection_dual_write_is_one_boundary(self):
        rows = self.sample()
        rows["events"] = [
            rows["events"][2],
            rows["events"][0],
            {
                "namespace": "task_module_event",
                "id": 90,
                "task_id": 7,
                "sequence": 90,
                "event_type": "rejected",
                "actor_id": 4,
                "module_key": "customization",
                "payload": {
                    "source": "customization_review",
                    "customization_review_decision": "return_to_designer",
                },
                "created_at": "2026-01-02T00:00:00Z",
            },
            {
                "namespace": "task_module_event",
                "id": 91,
                "task_id": 7,
                "sequence": 91,
                "event_type": "reopened",
                "actor_id": 4,
                "module_key": "customization",
                "payload": {
                    "source": "customization_review",
                    "customization_review_decision": "return_to_designer",
                },
                "created_at": "2026-01-02T00:00:00Z",
            },
            {
                "namespace": "task_event_log",
                "id": "return",
                "task_id": 7,
                "sequence": 4,
                "event_type": "task.customization.reviewed",
                "actor_id": 4,
                "payload": {
                    "customization_review_decision": "return_to_designer",
                },
                "created_at": "2026-01-02T00:00:01Z",
            },
        ]

        mapping, _, _ = MODULE.generate(rows)
        resource = mapping["resources"][0]

        self.assertEqual(
            [revision["status"] for revision in resource["history"]],
            ["rejected", "draft"],
        )
        self.assertIn(
            "task_module_event:90",
            resource["history"][0]["evidence_event_ids"],
        )
        self.assertIn(
            "task_event_log:return",
            resource["history"][0]["evidence_event_ids"],
        )

    def test_repaired_submit_uses_task_event_sequence_for_membership(self):
        rows = self.sample()
        rows["events"][0]["payload"]["repair_reason"] = "contract repair"
        rows["events"][0]["created_at"] = "2026-01-01T07:00:00Z"
        rows["assets"][0]["created_at"] = "2026-01-01T08:00:00Z"
        rows["assets"][1]["created_at"] = "2026-01-01T08:00:00Z"

        mapping, _, _ = MODULE.generate(rows)

        revision = mapping["resources"][0]["history"][0]
        self.assertEqual(revision["source_task_asset_id"], 10)
        self.assertEqual(revision["final_task_asset_ids"], [11])
        self.assertEqual(revision["confidence"], "proposed_review")

    def test_payload_asset_id_is_not_treated_as_task_asset_version(self):
        rows = self.sample()
        rows["events"] = [{
            **rows["events"][0], "payload": {"asset_id": 10},
        }]
        mapping, manual, _ = MODULE.generate(rows)
        revision = mapping["resources"][0]["history"][0]
        self.assertNotIn("source_task_asset_id", revision)
        self.assertEqual(revision["final_task_asset_ids"], [])
        self.assertIn("upload_session_id", manual[0]["reason"])

    def test_delivery_only_submission_uses_explicit_source_alias(self):
        rows = self.sample()
        rows["assets"] = [rows["assets"][1]]
        rows["events"][2]["payload"]["asset_version_ids"] = [11]
        mapping, manual, _ = MODULE.generate(rows)
        revision = mapping["resources"][0]["history"][0]
        self.assertNotIn("source_task_asset_id", revision)
        self.assertEqual(revision["source_alias_from_task_asset_id"], 11)
        self.assertEqual(revision["final_task_asset_ids"], [11])
        self.assertEqual(revision["confidence"], "proposed_review")
        self.assertTrue(any("alias" in row["reason"].lower() for row in manual))

    def test_submission_membership_is_only_completed_session_versions(self):
        rows = self.sample()
        rows["assets"].insert(0, {
            **rows["assets"][1], "id": 9, "asset_id": 99,
            "created_at": "2025-12-31T00:00:00Z", "uploaded_at": "2025-12-31T08:00:00Z",
        })
        mapping, _, _ = MODULE.generate(rows)
        self.assertEqual(mapping["resources"][0]["history"][0]["final_task_asset_ids"], [11])

    def test_superseded_and_wrong_stage_session_assets_are_excluded(self):
        rows = self.sample()
        rows["assets"][0]["superseded_by_version_id"] = 20
        rows["assets"][1]["source_module_key"] = "warehouse"
        mapping, manual, _ = MODULE.generate(rows)
        revision = mapping["resources"][0]["history"][0]
        self.assertNotIn("source_task_asset_id", revision)
        self.assertEqual(revision["final_task_asset_ids"], [])
        self.assertEqual(revision["confidence"], "hard_blocked")
        self.assertIn("superseded", manual[0]["reason"])
        self.assertIn("source_module_key", manual[0]["reason"])

    def test_asset_superseded_after_revision_boundary_remains_in_history(self):
        rows = self.sample()
        rows["assets"][0].update({
            "superseded_by_version_id": 20,
            "superseded_at": "2026-01-04T00:00:00Z",
            "flow_review_status": "superseded",
        })
        mapping, _, _ = MODULE.generate(rows)
        revision = mapping["resources"][0]["history"][0]
        self.assertEqual(revision["source_task_asset_id"], 10)

    def test_asset_superseded_between_submission_and_approval_blocks_finalization(self):
        rows = self.sample()
        rows["assets"][0].update({
            "superseded_by_version_id": 20,
            "superseded_at": "2026-01-02T00:00:00Z",
            "flow_review_status": "superseded",
        })
        mapping, _, _ = MODULE.generate(rows)
        revision = mapping["resources"][0]["history"][0]
        self.assertEqual(revision["confidence"], "hard_blocked")
        self.assertTrue(any("not eligible at approval" in value for value in revision["blockers"]))

    def test_proven_successor_and_completion_create_audit_revision(self):
        rows = self.sample()
        rows["assets"][1].update({
            "superseded_by_version_id": 13,
            "superseded_at": "2026-01-02T00:00:00Z",
            "flow_review_status": "superseded",
        })
        rows["assets"].append({
            **rows["assets"][1], "id": 13, "asset_id": rows["assets"][1]["asset_id"],
            "superseded_by_version_id": None, "superseded_at": "", "flow_review_status": "approved",
            "source_module_key": "audit", "created_at": "2026-01-02T00:00:00Z",
            "uploaded_at": "2026-01-02T00:00:00Z",
        })
        rows["events"].append({
            "namespace": "task_event_log", "id": "audit-upload", "task_id": 7,
            "event_type": "task.asset.upload_session.completed", "actor_id": 4,
            "module_key": "audit", "payload": {"upload_session_id": "audit-1", "asset_version_ids": [13]},
            "created_at": "2026-01-02T00:00:01Z",
        })
        mapping, _, _ = MODULE.generate(rows)
        history = mapping["resources"][0]["history"]
        self.assertEqual([revision["status"] for revision in history], ["superseded", "finalized"])
        self.assertEqual(history[1]["final_task_asset_ids"], [13])
        self.assertIn("task_event_log:audit-upload", history[1]["evidence_event_ids"])

    def test_asset_rejected_after_submission_remains_in_rejected_history(self):
        rows = self.sample()
        rows["assets"][1].update({
            "flow_review_status": "rejected",
            "rejected_at": "2026-01-02T00:00:00Z",
        })
        rows["events"] = [rows["events"][2], rows["events"][0], {
            "namespace": "task_event_log", "id": "reject", "task_id": 7,
            "event_type": "task.audit.rejected", "actor_id": 4,
            "created_at": "2026-01-02T00:00:00Z",
        }]
        mapping, _, _ = MODULE.generate(rows)
        revision = mapping["resources"][0]["history"][0]
        self.assertEqual(revision["final_task_asset_ids"], [11])
        self.assertEqual(revision["status"], "rejected")

    def test_single_sku_accepts_unscoped_legacy_assets_without_crossing_multi_sku(self):
        rows = self.sample()
        rows["scopes"] = [{
            "task_id": 7, "task_status": "PendingAuditA", "scope_kind": "sku",
            "scope_ref_id": 21, "sku_code": "SKU-A",
        }]
        mapping, _, _ = MODULE.generate(rows)
        revision = mapping["resources"][0]["history"][0]
        self.assertEqual(revision["source_task_asset_id"], 10)
        self.assertEqual(revision["final_task_asset_ids"], [11])

    def test_single_retouch_requirement_accepts_unscoped_legacy_assets(self):
        rows = self.sample()
        rows["scopes"] = [{
            "task_id": 7, "task_status": "Completed", "scope_kind": "retouch_requirement",
            "scope_ref_id": 31, "sku_code": "",
        }]
        for asset in rows["assets"]:
            asset.update({"source_module_key": "retouch", "retouch_requirement_id": None})
        rows["events"][0]["module_key"] = "retouch"
        mapping, _, _ = MODULE.generate(rows)
        revision = mapping["resources"][0]["history"][0]
        self.assertEqual(revision["final_task_asset_ids"], [11])

    def terminal_retouch_sample(self):
        rows = self.sample()
        rows["scopes"] = [{
            "task_id": 7,
            "task_status": "Completed",
            "scope_kind": "retouch_requirement",
            "scope_ref_id": 31,
            "sku_code": "",
        }]
        rows["assets"][1].update({
            "source_module_key": "retouch",
            "retouch_requirement_id": None,
            "upload_request_id": "retouch-1",
        })
        rows["events"] = [
            {
                "namespace": "task_event_log",
                "id": "retouch-upload",
                "task_id": 7,
                "sequence": 1,
                "event_type": "task.asset.upload_session.completed",
                "actor_id": 3,
                "module_key": "retouch",
                "payload": {
                    "upload_session_id": "retouch-1",
                    "asset_version_ids": [11],
                },
                "created_at": "2026-01-01T08:00:00Z",
            },
            {
                "namespace": "task_event_log",
                "id": "retouch-submit",
                "task_id": 7,
                "sequence": 2,
                "event_type": "task.design.submitted",
                "actor_id": 3,
                "module_key": "retouch",
                "payload": {"upload_session_id": "retouch-1"},
                "created_at": "2026-01-01T12:00:00Z",
            },
        ]
        return rows

    def exact_retouch_atomic_sample(self):
        task_id = 1562
        final_ids = {
            69: [7900, 7901, 7902],
            70: [7903, 7904, 7905],
        }
        assets = []
        events = []
        for index, asset_id in enumerate(
            value for values in final_ids.values() for value in values
        ):
            session_id = f"retouch-{asset_id}"
            created_at = f"2026-06-21T02:02:{49 + index:02d}Z"
            assets.append(
                {
                    "id": asset_id,
                    "asset_id": asset_id + 10000,
                    "task_id": task_id,
                    "asset_type": "delivery",
                    "scope_sku_code": "",
                    "retouch_requirement_id": None,
                    "upload_status": "uploaded",
                    "source_module_key": "retouch",
                    "flow_review_status": "not_applicable",
                    "superseded_by_version_id": None,
                    "upload_request_id": session_id,
                    "created_at": created_at,
                    "whole_hash": f"{asset_id:064x}",
                }
            )
            events.append(
                {
                    "namespace": "task_event_log",
                    "id": f"complete-{asset_id}",
                    "task_id": task_id,
                    "sequence": index + 1,
                    "event_type": "task.asset.upload_session.completed",
                    "actor_id": 228,
                    "module_key": "retouch",
                    "payload": {
                        "asset_type": "delivery",
                        "asset_version_id": asset_id,
                        "upload_session_id": session_id,
                    },
                    "created_at": created_at,
                }
            )
        events.append(
            {
                "namespace": "task_event_log",
                "id": "retouch-batch-submit",
                "task_id": task_id,
                "sequence": 7,
                "event_type": "task.design.submitted",
                "actor_id": 228,
                "module_key": "retouch",
                "payload": {"upload_session_id": "retouch-7905"},
                "created_at": "2026-06-21T02:03:00Z",
            }
        )
        return {
            "scopes": [
                {
                    "task_id": task_id,
                    "task_status": "Completed",
                    "scope_kind": "retouch_requirement",
                    "scope_ref_id": scope_id,
                    "sku_code": "",
                }
                for scope_id in final_ids
            ],
            "assets": assets,
            "references": [],
            "events": events,
            "planning_blockers": [],
        }

    def test_allowlisted_unscoped_retouch_batch_creates_independent_finalized_sets(self):
        mapping, manual, _ = MODULE.generate(self.exact_retouch_atomic_sample())
        resources = {
            resource["scope_ref_id"]: resource
            for resource in mapping["resources"]
        }
        self.assertEqual(resources[69]["history"][0]["final_task_asset_ids"], [7900, 7901, 7902])
        self.assertEqual(resources[70]["history"][0]["final_task_asset_ids"], [7903, 7904, 7905])
        for resource in resources.values():
            revision = resource["history"][0]
            self.assertEqual(revision["status"], "finalized")
            self.assertEqual(resource["working_revision_no"], 1)
            self.assertEqual(resource["finalized_revision_no"], 1)
            self.assertIn(
                MODULE.RETOUCH_UNSCOPED_ATOMIC_BATCH_POLICY,
                revision["review_policy_ids"],
            )
            self.assertEqual(revision["confidence"], "proposed_review")
        self.assertTrue(
            all(row["confidence"] == "proposed_review" for row in manual)
        )

    def test_allowlisted_retouch_batch_drift_fails_closed(self):
        rows = self.exact_retouch_atomic_sample()
        rows["assets"].pop()
        rows["events"].pop(-2)
        mapping, manual, _ = MODULE.generate(rows)
        self.assertTrue(
            all(
                all(
                    MODULE.RETOUCH_UNSCOPED_ATOMIC_BATCH_POLICY
                    not in revision["review_policy_ids"]
                    for revision in resource["history"]
                )
                for resource in mapping["resources"]
            )
        )
        self.assertTrue(
            all(row["confidence"] == "hard_blocked" for row in manual)
        )

    def retouch_visual_task2533_sample(self):
        contracts = MODULE.LEGACY_RETOUCH_VISUAL_SCOPE_TASK2533
        assets = []
        events = []
        sequence = 1
        for scope_id, contract in contracts.items():
            for role, asset_id, actor_id in (
                ("source", contract["source"], 240),
                ("delivery", contract["final"], 228),
            ):
                session_id = f"visual-{asset_id}"
                created_at = f"2026-07-17T08:2{scope_id - 183}:00Z"
                assets.append({
                    "id": asset_id,
                    "asset_id": asset_id + 1000,
                    "task_id": 2533,
                    "asset_type": role,
                    "scope_sku_code": "",
                    "retouch_requirement_id": (
                        scope_id if role == "source" else None
                    ),
                    "upload_status": "uploaded",
                    "source_module_key": "retouch",
                    "flow_review_status": (
                        "not_applicable" if role == "source"
                        else "pending_review"
                    ),
                    "superseded_by_version_id": None,
                    "upload_request_id": session_id,
                    "created_at": created_at,
                    "whole_hash": f"{asset_id:064x}",
                })
                events.append({
                    "namespace": "task_event_log",
                    "id": f"visual-complete-{asset_id}",
                    "task_id": 2533,
                    "sequence": sequence,
                    "event_type": "task.asset.upload_session.completed",
                    "actor_id": actor_id,
                    "module_key": "retouch",
                    "payload": {
                        "asset_version_id": asset_id,
                        "upload_session_id": session_id,
                    },
                    "created_at": created_at,
                })
                sequence += 1
        extra_id = MODULE.LEGACY_RETOUCH_VISUAL_UNASSIGNED_TASK2533[0]
        extra_session = f"visual-{extra_id}"
        assets.append({
            **assets[-1],
            "id": extra_id,
            "asset_id": extra_id + 1000,
            "retouch_requirement_id": None,
            "upload_request_id": extra_session,
            "created_at": "2026-07-17T08:26:00Z",
            "whole_hash": f"{extra_id:064x}",
        })
        events.append({
            "namespace": "task_event_log",
            "id": f"visual-complete-{extra_id}",
            "task_id": 2533,
            "sequence": sequence,
            "event_type": "task.asset.upload_session.completed",
            "actor_id": 228,
            "module_key": "retouch",
            "payload": {
                "asset_version_id": extra_id,
                "upload_session_id": extra_session,
            },
            "created_at": "2026-07-17T08:26:00Z",
        })
        events.append({
            "namespace": "task_event_log",
            "id": "visual-submit",
            "task_id": 2533,
            "sequence": sequence + 1,
            "event_type": "task.design.submitted",
            "actor_id": 228,
            "module_key": "retouch",
            "payload": {"upload_session_id": extra_session},
            "created_at": "2026-07-17T08:27:53Z",
        })
        references = [
            {
                "id": reference_id,
                "task_id": 2533,
                "scope_sku_code": "",
                "retouch_requirement_id": scope_id,
                "attached_at": "2026-07-17T03:16:00Z",
            }
            for scope_id, contract in contracts.items()
            for reference_id in contract["references"]
        ]
        return {
            "scopes": [
                {
                    "task_id": 2533,
                    "task_status": "Completed",
                    "scope_kind": "retouch_requirement",
                    "scope_ref_id": scope_id,
                    "sku_code": "",
                }
                for scope_id in contracts
            ],
            "assets": assets,
            "references": references,
            "events": events,
            "planning_blockers": [],
        }

    def test_task2533_visual_policy_binds_exact_five_and_excludes_sixth(self):
        mapping, manual, _ = MODULE.generate(
            self.retouch_visual_task2533_sample()
        )
        resources = {
            resource["scope_ref_id"]: resource
            for resource in mapping["resources"]
        }
        self.assertEqual(set(resources), {183, 184, 185, 186, 187})
        for scope_id, contract in (
            MODULE.LEGACY_RETOUCH_VISUAL_SCOPE_TASK2533.items()
        ):
            resource = resources[scope_id]
            revision = resource["history"][0]
            self.assertEqual(revision["status"], "finalized")
            self.assertEqual(
                revision["source_task_asset_id"], contract["source"]
            )
            self.assertEqual(
                revision["final_task_asset_ids"], [contract["final"]]
            )
            self.assertEqual(
                revision["reference_file_ref_ids"], contract["references"]
            )
            self.assertNotIn(
                MODULE.LEGACY_RETOUCH_VISUAL_UNASSIGNED_TASK2533[0],
                revision["final_task_asset_ids"],
            )
            self.assertEqual(
                revision["review_policy_ids"],
                [
                    MODULE.EXPLICIT_EVENT_REPLAY_POLICY,
                    MODULE.RETOUCH_VISUAL_SCOPE_TASK2533_POLICY,
                ],
            )
            self.assertEqual(revision["confidence"], "proposed_review")
        self.assertEqual(len(manual), 5)

    def test_task2533_visual_policy_drift_fails_closed(self):
        rows = self.retouch_visual_task2533_sample()
        rows["assets"][-1]["retouch_requirement_id"] = 187
        mapping, manual, _ = MODULE.generate(rows)
        self.assertTrue(
            all(
                MODULE.RETOUCH_VISUAL_SCOPE_TASK2533_POLICY
                not in revision.get("review_policy_ids", [])
                for resource in mapping["resources"]
                for revision in resource["history"]
            )
        )
        self.assertTrue(
            any(row["confidence"] == "hard_blocked" for row in manual)
        )

    def premature_retouch_sample(self):
        task_id = 981
        asset_id = 2763
        session_id = "partial-2763"
        return {
            "scopes": [
                {
                    "task_id": task_id,
                    "task_status": "Completed",
                    "scope_kind": "retouch_requirement",
                    "scope_ref_id": scope_id,
                    "sku_code": "",
                }
                for scope_id in range(8, 18)
            ],
            "assets": [
                {
                    "id": asset_id,
                    "asset_id": 9000,
                    "task_id": task_id,
                    "asset_type": "delivery",
                    "scope_sku_code": "",
                    "retouch_requirement_id": None,
                    "upload_status": "uploaded",
                    "source_module_key": "retouch",
                    "flow_review_status": "not_applicable",
                    "superseded_by_version_id": None,
                    "upload_request_id": session_id,
                    "created_at": "2026-05-29T05:23:52Z",
                    "whole_hash": "d" * 64,
                }
            ],
            "references": [],
            "events": [
                {
                    "namespace": "task_event_log",
                    "id": "partial-completion",
                    "task_id": task_id,
                    "sequence": 1,
                    "event_type": "task.asset.upload_session.completed",
                    "actor_id": 263,
                    "module_key": "retouch",
                    "payload": {
                        "asset_type": "delivery",
                        "asset_version_id": asset_id,
                        "upload_session_id": session_id,
                    },
                    "created_at": "2026-05-29T05:23:53Z",
                },
                {
                    "namespace": "task_event_log",
                    "id": "partial-submit",
                    "task_id": task_id,
                    "sequence": 2,
                    "event_type": "task.design.submitted",
                    "actor_id": 263,
                    "module_key": "retouch",
                    "payload": {"upload_session_id": session_id},
                    "created_at": "2026-05-29T05:23:53Z",
                },
            ],
            "planning_blockers": [],
        }

    def test_premature_retouch_completion_preserves_one_final_and_reopens_all_scopes(self):
        mapping, manual, _ = MODULE.generate(self.premature_retouch_sample())
        resources = {
            resource["scope_ref_id"]: resource
            for resource in mapping["resources"]
        }
        completed = resources[8]
        self.assertEqual(
            [revision["status"] for revision in completed["history"]],
            ["finalized", "draft"],
        )
        self.assertEqual(completed["finalized_revision_no"], 1)
        self.assertEqual(completed["working_revision_no"], 2)
        self.assertEqual(
            completed["history"][0]["final_task_asset_ids"], [2763]
        )
        self.assertEqual(
            completed["history"][1]["final_task_asset_ids"], [2763]
        )
        for scope_id in range(9, 18):
            resource = resources[scope_id]
            self.assertNotIn("finalized_revision_no", resource)
            self.assertEqual(resource["working_revision_no"], 1)
            self.assertEqual(resource["history"][0]["status"], "draft")
            self.assertEqual(resource["history"][0]["final_task_asset_ids"], [])
        decision = mapping["task_state_decisions"][0]
        self.assertEqual(
            (decision["from_status"], decision["target_status"]),
            ("Completed", "InProgress"),
        )
        self.assertEqual(decision["confidence"], "proposed_review")
        self.assertEqual(
            decision["review_policy_ids"],
            [MODULE.RETOUCH_PREMATURE_TERMINAL_PARTIAL_POLICY],
        )
        self.assertTrue(
            any(
                row["scope_kind"] == "task_state_decision"
                and row["confidence"] == "proposed_review"
                for row in manual
            )
        )

    def test_premature_retouch_allowlist_drift_does_not_emit_state_decision(self):
        rows = self.premature_retouch_sample()
        rows["scopes"].pop()
        mapping, _, _ = MODULE.generate(rows)
        self.assertEqual(mapping["task_state_decisions"], [])

    def test_unscoped_multi_requirement_terminal_reopens_without_guessing(self):
        rows = self.terminal_retouch_sample()
        rows["scopes"] = [
            {
                **rows["scopes"][0],
                "scope_ref_id": requirement_id,
            }
            for requirement_id in (31, 32)
        ]
        rows["assets"] = [rows["assets"][1]]

        mapping, manual, _ = MODULE.generate(rows)

        self.assertEqual(
            [
                (
                    resource["scope_ref_id"],
                    resource["history"][0]["status"],
                    resource["history"][0]["final_task_asset_ids"],
                )
                for resource in mapping["resources"]
            ],
            [(31, "draft", []), (32, "draft", [])],
        )
        decision = mapping["task_state_decisions"][0]
        self.assertEqual(
            (decision["from_status"], decision["target_status"]),
            ("Completed", "InProgress"),
        )
        self.assertEqual(
            decision["review_policy_ids"],
            [MODULE.RETOUCH_PREMATURE_TERMINAL_PARTIAL_POLICY],
        )
        self.assertNotIn(
            "hard_blocked",
            {
                row["confidence"]
                for row in manual
                if row["task_id"] == 7
            },
        )

    def test_completed_retouch_terminal_submit_is_a_finalized_policy_candidate(self):
        rows = self.terminal_retouch_sample()
        self.assertNotIn("group_id", rows["scopes"][0])
        mapping, manual, _ = MODULE.generate(rows)
        resource = mapping["resources"][0]
        revision = resource["history"][0]
        self.assertEqual(revision["status"], "finalized")
        self.assertEqual(revision["confidence"], "proposed_review")
        self.assertEqual(revision["submitted_at"], "2026-01-01T12:00:00Z")
        self.assertEqual(revision["finalized_at"], revision["submitted_at"])
        self.assertEqual(resource["working_revision_no"], 1)
        self.assertEqual(resource["finalized_revision_no"], 1)
        self.assertEqual(
            revision["review_policy_ids"],
            [
                MODULE.EXPLICIT_EVENT_REPLAY_POLICY,
                MODULE.RETOUCH_SOURCE_OPTIONAL_POLICY,
                MODULE.RETOUCH_TERMINAL_SUBMIT_POLICY,
            ],
        )
        self.assertEqual(manual[0]["confidence"], "proposed_review")

    def test_completed_retouch_atomic_multi_file_submit_is_finalized(self):
        rows = self.terminal_retouch_sample()
        rows["assets"].append({
            **rows["assets"][1],
            "id": 12,
            "asset_id": 102,
            "upload_request_id": "retouch-2",
            "created_at": "2026-01-01T07:59:00Z",
            "uploaded_at": "2026-01-01T07:59:00Z",
        })
        rows["events"].insert(0, {
            "namespace": "task_event_log",
            "id": "retouch-upload-2",
            "task_id": 7,
            "sequence": 0,
            "event_type": "task.asset.upload_session.completed",
            "actor_id": 3,
            "module_key": "retouch",
            "payload": {
                "upload_session_id": "retouch-2",
                "asset_version_ids": [12],
            },
            "created_at": "2026-01-01T07:59:00Z",
        })
        rows["events"][2]["created_at"] = "2026-01-01T08:01:00Z"

        mapping, _, _ = MODULE.generate(rows)
        resource = mapping["resources"][0]
        revision = resource["history"][0]

        self.assertEqual(revision["status"], "finalized")
        self.assertEqual(revision["final_task_asset_ids"], [11, 12])
        self.assertEqual(resource["working_revision_no"], 1)
        self.assertEqual(resource["finalized_revision_no"], 1)
        self.assertIn(
            MODULE.RETOUCH_TERMINAL_SUBMIT_POLICY,
            revision["review_policy_ids"],
        )

    def test_retouch_terminal_submit_with_later_reject_is_not_finalized(self):
        rows = self.terminal_retouch_sample()
        rows["events"].append({
            "namespace": "task_event_log",
            "id": "retouch-reject",
            "task_id": 7,
            "sequence": 3,
            "event_type": "task.audit.rejected",
            "actor_id": 4,
            "module_key": "audit",
            "created_at": "2026-01-02T00:00:00Z",
        })
        mapping, _, _ = MODULE.generate(rows)
        resource = mapping["resources"][0]
        self.assertNotIn("finalized_revision_no", resource)
        self.assertTrue(all(
            MODULE.RETOUCH_TERMINAL_SUBMIT_POLICY
            not in revision["review_policy_ids"]
            for revision in resource["history"]
        ))

    def test_retouch_terminal_policy_does_not_clear_hard_sibling(self):
        rows = self.terminal_retouch_sample()
        rows["events"][1]["payload"]["upload_session_id"] = "missing-session"
        rows["events"].extend([
            {
                "namespace": "task_event_log",
                "id": "retouch-upload-2",
                "task_id": 7,
                "sequence": 3,
                "event_type": "task.asset.upload_session.completed",
                "actor_id": 3,
                "module_key": "retouch",
                "payload": {
                    "upload_session_id": "retouch-2",
                    "asset_version_ids": [11],
                },
                "created_at": "2026-01-02T08:00:00Z",
            },
            {
                "namespace": "task_event_log",
                "id": "retouch-submit-2",
                "task_id": 7,
                "sequence": 4,
                "event_type": "task.design.submitted",
                "actor_id": 3,
                "module_key": "retouch",
                "payload": {"upload_session_id": "retouch-2"},
                "created_at": "2026-01-02T12:00:00Z",
            },
        ])
        rows["assets"][1]["upload_request_id"] = "retouch-2"
        rows["assets"][1]["created_at"] = "2026-01-02T06:00:00Z"
        mapping, _, _ = MODULE.generate(rows)
        history = mapping["resources"][0]["history"]
        self.assertEqual(history[0]["confidence"], "hard_blocked")
        self.assertTrue(history[0]["blockers"])
        self.assertEqual(history[1]["status"], "finalized")
        self.assertEqual(history[1]["confidence"], "proposed_review")
        self.assertIn(
            MODULE.RETOUCH_TERMINAL_SUBMIT_POLICY,
            history[1]["review_policy_ids"],
        )

    def test_completed_retouch_repair_reopen_supersedes_then_refinalizes(self):
        rows = self.terminal_retouch_sample()
        rows["assets"].append({
            **rows["assets"][1],
            "id": 12,
            "asset_id": 102,
            "upload_request_id": "retouch-2",
            "created_at": "2026-01-03T08:00:00Z",
        })
        rows["events"].extend([
            {
                "namespace": "task_event_log",
                "id": "repair-reopen",
                "task_id": 7,
                "sequence": 3,
                "event_type": "task.status.changed",
                "actor_id": 1,
                "payload": {
                    "from_task_status": "Completed",
                    "to_task_status": "InProgress",
                    "source": "codex_repair_v1_126",
                    "reason": "prematurely completed; reopened for re-upload",
                },
                "created_at": "2026-01-02T00:00:00Z",
            },
            {
                "namespace": "task_event_log",
                "id": "retouch-upload-2",
                "task_id": 7,
                "sequence": 4,
                "event_type": "task.asset.upload_session.completed",
                "actor_id": 3,
                "module_key": "retouch",
                "payload": {
                    "upload_session_id": "retouch-2",
                    "asset_version_ids": [12],
                },
                "created_at": "2026-01-03T08:00:00Z",
            },
            {
                "namespace": "task_event_log",
                "id": "retouch-submit-2",
                "task_id": 7,
                "sequence": 5,
                "event_type": "task.design.submitted",
                "actor_id": 3,
                "module_key": "retouch",
                "payload": {"upload_session_id": "retouch-2"},
                "created_at": "2026-01-03T12:00:00Z",
            },
        ])
        mapping, _, _ = MODULE.generate(rows)
        resource = mapping["resources"][0]
        self.assertEqual(
            [revision["status"] for revision in resource["history"]],
            ["superseded", "finalized"],
        )
        self.assertEqual(resource["history"][1]["source_stage"], "reopen")
        self.assertEqual(resource["history"][1]["final_task_asset_ids"], [12])
        self.assertEqual(resource["working_revision_no"], 2)
        self.assertEqual(resource["finalized_revision_no"], 2)
        self.assertIn(
            "task_event_log:repair-reopen",
            resource["history"][1]["evidence_event_ids"],
        )

    def test_post_close_same_root_replacements_replay_multi_hop(self):
        rows = self.sample()
        rows["events"].append({
            "namespace": "task_event_log",
            "id": "close",
            "task_id": 7,
            "sequence": 4,
            "event_type": "task.closed",
            "actor_id": 4,
            "payload": {},
            "created_at": "2026-01-04T00:00:00Z",
        })
        rows["assets"][1].update({
            "flow_review_status": "superseded",
            "superseded_by_version_id": 13,
            "superseded_at": "2026-01-05T08:00:00Z",
        })
        rows["assets"].extend([
            {
                **rows["assets"][1],
                "id": 13,
                "asset_id": 101,
                "flow_review_status": "superseded",
                "superseded_by_version_id": 14,
                "superseded_at": "2026-01-06T08:00:00Z",
                "upload_request_id": "post-close-1",
                "created_at": "2026-01-05T00:00:00Z",
            },
            {
                **rows["assets"][1],
                "id": 14,
                "asset_id": 101,
                "flow_review_status": "approved",
                "superseded_by_version_id": None,
                "superseded_at": "",
                "upload_request_id": "post-close-2",
                "created_at": "2026-01-06T00:00:00Z",
            },
        ])
        rows["events"].extend([
            {
                "namespace": "task_event_log",
                "id": "post-close-1",
                "task_id": 7,
                "sequence": 5,
                "event_type": "task.asset.upload_session.completed",
                "actor_id": 4,
                "payload": {
                    "upload_session_id": "post-close-1",
                    "asset_version_id": 13,
                },
                "created_at": "2026-01-05T08:00:00Z",
            },
            {
                "namespace": "task_event_log",
                "id": "post-close-2",
                "task_id": 7,
                "sequence": 6,
                "event_type": "task.asset.upload_session.completed",
                "actor_id": 4,
                "payload": {
                    "upload_session_id": "post-close-2",
                    "asset_version_id": 14,
                },
                "created_at": "2026-01-06T08:00:00Z",
            },
        ])
        mapping, _, _ = MODULE.generate(rows)
        resource = mapping["resources"][0]
        self.assertEqual(
            [revision["status"] for revision in resource["history"]],
            ["superseded", "superseded", "finalized"],
        )
        self.assertEqual(
            [revision["final_task_asset_ids"] for revision in resource["history"]],
            [[11], [13], [14]],
        )
        self.assertEqual(resource["finalized_revision_no"], 3)
        self.assertTrue(all(
            revision["source_stage"] == "reopen"
            for revision in resource["history"][1:]
        ))
        self.assertTrue(all(
            MODULE.REOPEN_POLICY in revision["review_policy_ids"]
            for revision in resource["history"][1:]
        ))
        self.assertTrue(all(
            MODULE.POST_CLOSE_REPLACEMENT_POLICY in revision["review_policy_ids"]
            for revision in resource["history"][1:]
        ))
        self.assertTrue(all(
            "task_event_log:upload" in revision["evidence_event_ids"]
            for revision in resource["history"][1:]
        ))

    def test_distinct_post_close_sessions_do_not_prune_later_predecessor(self):
        rows = self.sample()
        rows["events"].append({
            "namespace": "task_event_log",
            "id": "close",
            "task_id": 7,
            "sequence": 4,
            "event_type": "task.closed",
            "actor_id": 4,
            "payload": {},
            "created_at": "2026-01-04T00:00:00Z",
        })
        rows["assets"][0].update({
            "flow_review_status": "superseded",
            "superseded_by_version_id": 14,
            "superseded_at": "2026-01-05T00:00:42Z",
        })
        rows["assets"][1].update({
            "flow_review_status": "superseded",
            "superseded_by_version_id": 13,
            "superseded_at": "2026-01-05T00:00:00Z",
        })
        rows["assets"].extend([
            {
                **rows["assets"][1],
                "id": 13,
                "asset_id": 101,
                "flow_review_status": "approved",
                "superseded_by_version_id": None,
                "superseded_at": "",
                "upload_request_id": "replace-final",
                "created_at": "2026-01-05T00:00:00Z",
            },
            {
                **rows["assets"][0],
                "id": 14,
                "asset_id": 100,
                "flow_review_status": "approved",
                "superseded_by_version_id": None,
                "superseded_at": "",
                "upload_request_id": "replace-source",
                "created_at": "2026-01-05T00:00:42Z",
            },
        ])
        rows["events"].extend([
            {
                "namespace": "task_event_log",
                "id": "replace-final",
                "task_id": 7,
                "sequence": 5,
                "event_type": "task.asset.upload_session.completed",
                "actor_id": 4,
                "payload": {
                    "upload_session_id": "replace-final",
                    "asset_version_id": 13,
                    "post_close_replacement": True,
                },
                "created_at": "2026-01-05T00:00:00Z",
            },
            {
                "namespace": "task_event_log",
                "id": "replace-source",
                "task_id": 7,
                "sequence": 6,
                "event_type": "task.asset.upload_session.completed",
                "actor_id": 4,
                "payload": {
                    "upload_session_id": "replace-source",
                    "asset_version_id": 14,
                    "post_close_replacement": True,
                },
                "created_at": "2026-01-05T00:00:42Z",
            },
        ])

        mapping, _, _ = MODULE.generate(rows)
        resource = mapping["resources"][0]
        self.assertEqual(
            [revision["status"] for revision in resource["history"]],
            ["superseded", "superseded", "finalized"],
        )
        self.assertEqual(
            [
                revision.get("source_task_asset_id")
                or revision.get("source_alias_from_task_asset_id")
                for revision in resource["history"]
            ],
            [10, 10, 14],
        )
        self.assertEqual(
            [
                revision["final_task_asset_ids"]
                for revision in resource["history"]
            ],
            [[11], [13], [13]],
        )
        self.assertFalse(
            any(
                revision["confidence"] == "hard_blocked"
                for revision in resource["history"]
            )
        )

    def test_post_close_replacement_is_inserted_before_later_supplements(self):
        rows = self.sample()
        rows["events"].append({
            "namespace": "task_event_log", "id": "close", "task_id": 7,
            "sequence": 4, "event_type": "task.closed", "actor_id": 4,
            "payload": {}, "created_at": "2026-01-04T00:00:00Z",
        })
        rows["assets"][0].update({
            "flow_review_status": "superseded",
            "superseded_by_version_id": 13,
            "superseded_at": "2026-01-05T12:00:00Z",
        })
        rows["assets"].extend([
            {
                **rows["assets"][1], "id": 12, "asset_id": 102,
                "source_module_key": "audit",
                "created_at": "2026-01-05T00:00:00Z",
            },
            {
                **rows["assets"][0], "id": 13, "asset_id": 100,
                "flow_review_status": "approved",
                "superseded_by_version_id": None, "superseded_at": "",
                "upload_request_id": "replace-source",
                "created_at": "2026-01-05T12:00:00Z",
            },
            {
                **rows["assets"][1], "id": 14, "asset_id": 103,
                "source_module_key": "audit",
                "created_at": "2026-01-06T00:00:00Z",
            },
        ])
        rows["events"].extend([
            {
                "namespace": "task_event_log", "id": "supplement-1",
                "task_id": 7, "sequence": 5,
                "event_type": "task.audit.supplement_uploaded",
                "actor_id": 4,
                "payload": {
                    "append_asset_version_ids": [12],
                    "upload_session_id": "supplement-1",
                    "asset_operation": "append",
                },
                "created_at": "2026-01-05T00:00:00Z",
            },
            {
                "namespace": "task_event_log", "id": "replace-source",
                "task_id": 7, "sequence": 6,
                "event_type": "task.asset.upload_session.completed",
                "actor_id": 4,
                "payload": {
                    "upload_session_id": "replace-source",
                    "asset_version_id": 13,
                },
                "created_at": "2026-01-05T12:00:00Z",
            },
            {
                "namespace": "task_event_log", "id": "supplement-2",
                "task_id": 7, "sequence": 7,
                "event_type": "task.audit.supplement_uploaded",
                "actor_id": 4,
                "payload": {
                    "append_asset_version_ids": [14],
                    "upload_session_id": "supplement-2",
                    "asset_operation": "append",
                },
                "created_at": "2026-01-06T00:00:00Z",
            },
        ])

        mapping, _, _ = MODULE.generate(rows)
        resource = mapping["resources"][0]
        self.assertEqual(
            [revision["created_at"] for revision in resource["history"]],
            [
                "2026-01-01T12:00:00Z",
                "2026-01-05T00:00:00Z",
                "2026-01-05T12:00:00Z",
                "2026-01-06T00:00:00Z",
            ],
        )
        self.assertEqual(
            [revision.get("source_task_asset_id") for revision in resource["history"]],
            [10, 10, 13, 13],
        )
        self.assertEqual(
            [revision["final_task_asset_ids"] for revision in resource["history"]],
            [[11], [11, 12], [11, 12], [11, 12, 14]],
        )
        self.assertEqual(resource["working_revision_no"], 4)
        self.assertEqual(resource["finalized_revision_no"], 4)
        self.assertEqual(
            [revision["status"] for revision in resource["history"]],
            ["superseded", "superseded", "superseded", "finalized"],
        )
        self.assertIn(
            MODULE.POST_CLOSE_REPLACEMENT_POLICY,
            resource["history"][3]["review_policy_ids"],
        )
        self.assertIn(
            "task_event_log:replace-source",
            resource["history"][3]["evidence_event_ids"],
        )

    def test_resubmit_completion_before_final_close_is_not_post_close(self):
        rows = self.sample()
        rows["assets"][1].update({
            "flow_review_status": "superseded",
            "superseded_by_version_id": 13,
            "superseded_at": "2026-01-05T08:00:00Z",
        })
        rows["assets"].append({
            **rows["assets"][1],
            "id": 13,
            "asset_id": 101,
            "flow_review_status": "approved",
            "superseded_by_version_id": None,
            "superseded_at": "",
            "upload_request_id": "resubmit-session",
            "created_at": "2026-01-05T08:00:00Z",
        })
        rows["events"].extend([
            {
                "namespace": "task_event_log",
                "id": "rejected-close",
                "task_id": 7,
                "sequence": 4,
                "event_type": "task.closed",
                "actor_id": 4,
                "payload": {},
                "created_at": "2026-01-04T00:00:00Z",
            },
            {
                "namespace": "task_event_log",
                "id": "resubmit-completion",
                "task_id": 7,
                "sequence": 5,
                "event_type": "task.asset.upload_session.completed",
                "actor_id": 4,
                "payload": {
                    "upload_session_id": "resubmit-session",
                    "asset_version_id": 13,
                },
                "created_at": "2026-01-05T08:00:00Z",
            },
            {
                "namespace": "task_event_log",
                "id": "resubmit",
                "task_id": 7,
                "sequence": 6,
                "event_type": "task.design.submitted",
                "actor_id": 4,
                "payload": {"upload_session_id": "resubmit-session"},
                "created_at": "2026-01-05T08:00:00Z",
            },
            {
                "namespace": "task_event_log",
                "id": "final-close",
                "task_id": 7,
                "sequence": 7,
                "event_type": "task.closed",
                "actor_id": 4,
                "payload": {},
                "created_at": "2026-01-06T00:00:00Z",
            },
        ])

        self.assertEqual(
            MODULE.direct_post_close_replacements(
                rows["scopes"][0], rows["events"], rows["assets"]
            ),
            [],
        )

    def test_completed_retouch_submit_is_post_close_boundary_for_flagged_replacement(self):
        rows = self.sample()
        scope = {
            "task_id": 7,
            "task_status": "Completed",
            "scope_kind": "retouch_requirement",
            "scope_ref_id": 31,
            "_single_requirement": True,
        }
        for asset in rows["assets"]:
            asset["retouch_requirement_id"] = 31
        rows["assets"][1].update({
            "flow_review_status": "superseded",
            "superseded_by_version_id": 13,
            "superseded_at": "2026-01-05T08:00:00Z",
        })
        rows["assets"].append({
            **rows["assets"][1],
            "id": 13,
            "flow_review_status": "approved",
            "superseded_by_version_id": None,
            "superseded_at": "",
            "upload_request_id": "retouch-post-close",
            "created_at": "2026-01-05T00:00:00Z",
        })
        completion = {
            "namespace": "task_event_log",
            "id": "retouch-post-close",
            "task_id": 7,
            "sequence": 4,
            "event_type": "task.asset.upload_session.completed",
            "actor_id": 4,
            "payload": {
                "upload_session_id": "retouch-post-close",
                "asset_version_id": 13,
                "post_close_replacement": True,
            },
            "created_at": "2026-01-05T08:00:00Z",
        }
        events = [rows["events"][2], rows["events"][0], completion]

        replacements = MODULE.direct_post_close_replacements(
            scope, events, rows["assets"]
        )
        self.assertEqual(len(replacements), 1)
        self.assertEqual(replacements[0][2]["id"], 13)

    def test_post_close_edge_missing_from_snapshot_blocks_existing_revision(self):
        rows = self.sample()
        rows["events"].append({
            "namespace": "task_event_log",
            "id": "close",
            "task_id": 7,
            "sequence": 4,
            "event_type": "task.closed",
            "actor_id": 4,
            "payload": {},
            "created_at": "2026-01-04T00:00:00Z",
        })
        rows["assets"].extend([
            {
                **rows["assets"][1],
                "id": 12,
                "asset_id": 999,
                "flow_review_status": "superseded",
                "superseded_by_version_id": 13,
                "superseded_at": "2026-01-05T08:00:00Z",
                "created_at": "2026-01-02T00:00:00Z",
            },
            {
                **rows["assets"][1],
                "id": 13,
                "asset_id": 999,
                "flow_review_status": "approved",
                "superseded_by_version_id": None,
                "superseded_at": "",
                "upload_request_id": "post-close-missing-root",
                "created_at": "2026-01-05T00:00:00Z",
            },
        ])
        rows["events"].append({
            "namespace": "task_event_log",
            "id": "post-close-missing-root",
            "task_id": 7,
            "sequence": 5,
            "event_type": "task.asset.upload_session.completed",
            "actor_id": 4,
            "payload": {
                "upload_session_id": "post-close-missing-root",
                "asset_version_id": 13,
            },
            "created_at": "2026-01-05T08:00:00Z",
        })

        mapping, _, _ = MODULE.generate(rows)
        resource = mapping["resources"][0]
        self.assertEqual(
            [revision["status"] for revision in resource["history"]],
            ["finalized"],
        )
        self.assertEqual(resource["working_revision_no"], 1)
        self.assertEqual(resource["finalized_revision_no"], 1)
        self.assertEqual(resource["history"][0]["confidence"], "hard_blocked")
        self.assertIn(
            "post-close successor 13 asset root is absent",
            "; ".join(resource["history"][0]["blockers"]),
        )
        self.assertIn(
            "task_event_log:post-close-missing-root",
            resource["history"][0]["evidence_event_ids"],
        )

    def test_later_post_close_revision_preserves_prior_hard_evidence(self):
        rows = self.sample()
        rows["events"].append({
            "namespace": "task_event_log", "id": "close", "task_id": 7,
            "sequence": 4, "event_type": "task.closed", "actor_id": 4,
            "payload": {}, "created_at": "2026-01-04T00:00:00Z",
        })
        rows["assets"][1].update({
            "flow_review_status": "superseded",
            "superseded_by_version_id": 14,
            "superseded_at": "2026-01-06T08:00:00Z",
        })
        rows["assets"].extend([
            {
                **rows["assets"][1], "id": 12, "asset_id": 999,
                "flow_review_status": "superseded",
                "superseded_by_version_id": 13,
                "superseded_at": "2026-01-05T08:00:00Z",
                "created_at": "2026-01-02T00:00:00Z",
            },
            {
                **rows["assets"][1], "id": 13, "asset_id": 999,
                "flow_review_status": "approved",
                "superseded_by_version_id": None, "superseded_at": "",
                "upload_request_id": "missing-root",
                "created_at": "2026-01-05T00:00:00Z",
            },
            {
                **rows["assets"][1], "id": 14, "asset_id": 101,
                "flow_review_status": "approved",
                "superseded_by_version_id": None, "superseded_at": "",
                "upload_request_id": "known-root",
                "created_at": "2026-01-06T00:00:00Z",
            },
        ])
        rows["events"].extend([
            {
                "namespace": "task_event_log", "id": "missing-root",
                "task_id": 7, "sequence": 5,
                "event_type": "task.asset.upload_session.completed",
                "actor_id": 4,
                "payload": {"upload_session_id": "missing-root", "asset_version_id": 13},
                "created_at": "2026-01-05T08:00:00Z",
            },
            {
                "namespace": "task_event_log", "id": "known-root",
                "task_id": 7, "sequence": 6,
                "event_type": "task.asset.upload_session.completed",
                "actor_id": 4,
                "payload": {"upload_session_id": "known-root", "asset_version_id": 14},
                "created_at": "2026-01-06T08:00:00Z",
            },
        ])

        mapping, _, _ = MODULE.generate(rows)
        resource = mapping["resources"][0]
        self.assertEqual(
            [revision["status"] for revision in resource["history"]],
            ["superseded", "finalized"],
        )
        self.assertEqual(resource["working_revision_no"], 2)
        self.assertEqual(resource["finalized_revision_no"], 2)
        self.assertEqual(resource["history"][0]["confidence"], "hard_blocked")
        self.assertIn(
            "task_event_log:missing-root",
            resource["history"][1]["evidence_event_ids"],
        )

    def test_evidence_ids_are_sorted_by_real_event_order(self):
        events = [
            {
                "namespace": "task_event_log", "id": "later", "sequence": 9,
                "created_at": "2026-01-02T00:00:00Z",
            },
            {
                "namespace": "task_event_log", "id": "earlier", "sequence": 3,
                "created_at": "2026-01-01T00:00:00Z",
            },
        ]
        self.assertEqual(
            MODULE.sorted_evidence_ids(
                [
                    "task_event_log:later",
                    "task_event_log:earlier",
                    "task_event_log:later",
                ],
                events,
            ),
            ["task_event_log:earlier", "task_event_log:later"],
        )

    def test_inherited_snapshot_merges_exact_member_completion_evidence(self):
        rows = self.sample()
        revision = {
            "source_alias_from_task_asset_id": 11,
            "final_task_asset_ids": [11],
            "created_at": "2026-01-02T00:00:00Z",
            "evidence_event_ids": ["task_event_log:one"],
        }
        MODULE.merge_member_completion_evidence(revision, rows["events"])
        self.assertEqual(
            revision["evidence_event_ids"],
            ["task_event_log:one", "task_event_log:upload"],
        )

    def test_rejection_draft_drops_rejected_current_members(self):
        revision = {
            "status": "draft",
            "source_stage": "reopen",
            "created_at": "2026-01-03T00:00:00Z",
            "source_alias_from_task_asset_id": 11,
            "final_task_asset_ids": [11, 12],
            "mode": "set",
        }
        assets = [
            {
                "id": 11,
                "flow_review_status": "rejected",
                "rejected_at": "2026-01-02T00:00:00Z",
            },
            {
                "id": 12,
                "flow_review_status": "approved",
                "rejected_at": "",
            },
        ]
        MODULE.clear_rejected_members_from_reopen_draft(revision, assets)
        self.assertNotIn("source_alias_from_task_asset_id", revision)
        self.assertEqual(revision["final_task_asset_ids"], [12])
        self.assertEqual(revision["mode"], "single")

    def test_upload_reopen_prunes_bounded_lifecycle_cleanup(self):
        revision = {
            "status": "finalized",
            "source_stage": "reopen",
            "created_at": "2026-07-27T09:33:17Z",
            "source_alias_from_task_asset_id": 28735,
            "final_task_asset_ids": [28735, 28756],
            "mode": "set",
        }
        assets = [
            {
                "id": 28735,
                "asset_type": "delivery",
                "upload_status": "uploaded",
                "deleted_at": "2026-07-27T09:33:34Z",
            },
            {
                "id": 28756,
                "asset_type": "delivery",
                "upload_status": "uploaded",
                "deleted_at": None,
            },
        ]
        event = {
            "event_type": "task.asset.upload_session.completed",
            "created_at": "2026-07-27T09:33:17Z",
        }
        MODULE.prune_inherited_reopen_snapshot(
            revision, assets, event, "sku"
        )
        self.assertEqual(revision["final_task_asset_ids"], [28756])
        self.assertEqual(revision["source_alias_from_task_asset_id"], 28756)
        self.assertEqual(revision["mode"], "single")

    def test_non_upload_reopen_does_not_prune_future_lifecycle(self):
        revision = {
            "status": "draft",
            "source_stage": "reopen",
            "created_at": "2026-07-27T09:33:17Z",
            "source_alias_from_task_asset_id": 28735,
            "final_task_asset_ids": [28735],
            "mode": "single",
        }
        assets = [
            {
                "id": 28735,
                "asset_type": "delivery",
                "upload_status": "uploaded",
                "deleted_at": "2026-07-27T09:33:34Z",
            }
        ]
        event = {
            "event_type": "task.reopened",
            "created_at": "2026-07-27T09:33:17Z",
        }
        MODULE.prune_inherited_reopen_snapshot(
            revision, assets, event, "sku"
        )
        self.assertEqual(revision["final_task_asset_ids"], [28735])
        self.assertEqual(revision["source_alias_from_task_asset_id"], 28735)

    def test_completed_retouch_without_boundary_remains_hard_blocked(self):
        rows = self.terminal_retouch_sample()
        rows["events"] = []
        mapping, manual, _ = MODULE.generate(rows)
        self.assertEqual(mapping["resources"][0]["history"], [])
        self.assertEqual(manual[0]["confidence"], "hard_blocked")

    def test_warehouse_rejection_creates_reopen_draft(self):
        rows = self.sample()
        rows["events"].append({
            "namespace": "task_event_log", "id": "warehouse", "task_id": 7,
            "event_type": "task.warehouse.rejected", "actor_id": 5,
            "created_at": "2026-01-04T00:00:00Z",
        })
        mapping, _, _ = MODULE.generate(rows)
        resource = mapping["resources"][0]
        self.assertEqual([revision["status"] for revision in resource["history"]], ["finalized", "draft"])
        self.assertEqual(resource["history"][1]["source_stage"], "reopen")

    def test_duplicate_warehouse_rejections_are_grouped_by_receipt(self):
        rows = self.sample()
        for event_id, created_at in (("warehouse-1", "2026-01-04T00:00:00Z"), ("warehouse-2", "2026-01-04T00:00:01Z")):
            rows["events"].append({
                "namespace": "task_event_log", "id": event_id, "task_id": 7,
                "event_type": "task.warehouse.rejected", "actor_id": 5,
                "payload": {"receipt_no": "WR-7"}, "created_at": created_at,
            })
        mapping, _, _ = MODULE.generate(rows)
        resource = mapping["resources"][0]
        self.assertEqual(len(resource["history"]), 2)
        self.assertEqual(
            resource["history"][1]["evidence_event_ids"],
            [
                "task_event_log:upload",
                "task_event_log:warehouse-1",
                "task_event_log:warehouse-2",
            ],
        )

    def test_audit_supplement_creates_explicit_append_reopen_revision(self):
        rows = self.sample()
        rows["assets"].append({
            **rows["assets"][1], "id": 14, "upload_session_id": "supp-1", "source_module_key": "audit",
            "created_at": "2026-01-04T00:00:00Z", "uploaded_at": "2026-01-04T08:00:00Z",
        })
        rows["events"].append({
            "namespace": "task_event_log", "id": "supplement", "task_id": 7,
            "event_type": "task.audit.supplement_uploaded", "actor_id": 5,
            "payload": {"upload_session_id": "supp-1", "asset_version_id": 14, "before": 1, "after": 2},
            "created_at": "2026-01-05T00:00:00Z",
        })
        mapping, _, _ = MODULE.generate(rows)
        resource = mapping["resources"][0]
        self.assertEqual([revision["status"] for revision in resource["history"]], ["superseded", "finalized"])
        self.assertEqual(resource["history"][1]["source_stage"], "reopen")
        self.assertEqual(resource["history"][1]["final_task_asset_ids"], [11, 14])

    def test_audit_supplement_accepts_named_delivery_counts(self):
        rows = self.sample()
        rows["assets"].append({
            **rows["assets"][1],
            "id": 14,
            "upload_session_id": "supp-1",
            "source_module_key": "audit",
            "created_at": "2026-01-04T00:00:00Z",
        })
        rows["events"].append({
            "namespace": "task_event_log",
            "id": "supplement",
            "task_id": 7,
            "event_type": "task.audit.supplement_uploaded",
            "actor_id": 5,
            "payload": {
                "upload_session_id": "supp-1",
                "asset_version_id": 14,
                "audit_delivery_count_before": 1,
                "audit_delivery_count_after": 2,
            },
            "created_at": "2026-01-05T00:00:00Z",
        })

        mapping, _, _ = MODULE.generate(rows)
        revision = mapping["resources"][0]["history"][1]

        self.assertEqual(revision["final_task_asset_ids"], [11, 14])
        self.assertEqual(revision["confidence"], "proposed_review")

    def test_non_terminal_scope_without_boundary_is_an_empty_shell(self):
        rows = self.sample()
        rows["scopes"][0]["task_status"] = "InProgress"
        rows["events"] = []
        mapping, manual, _ = MODULE.generate(rows)
        self.assertEqual(mapping["resources"][0]["history"], [])
        self.assertEqual(len(manual), 1)
        self.assertEqual(manual[0]["confidence"], "proposed_review")

    def test_read_query_exports_real_replay_linkage_fields(self):
        for field in (
            "'asset_id',ta.asset_id", "'upload_request_id'", "'source_module_key'",
            "'flow_review_status'", "'approved_by'", "'approved_at'",
            "'rejected_at'", "'superseded_by_version_id'", "'superseded_at'",
            "DATE_FORMAT(DATE_SUB(ta.superseded_at", "'sequence',e.sequence",
            "'module_key'", "'from_state'", "'to_state'",
            "'sku_code',COALESCE(si.sku_code,'')",
        ):
            self.assertIn(field, MODULE.SQL)

    def test_planning_candidates_preserve_legal_status_and_export_sku_truth(self):
        for task_status in ("Cancelled", "PendingAssign", "Completed"):
            with self.subTest(task_status=task_status):
                rows = self.sample()
                rows["planning_rows"] = [{
                    "task_id": 70, "task_status": task_status, "creator_id": 9,
                    "task_sku_item_id": 701, "sku_code": "SKU-70",
                    "description_spec": "Blue / XL", "quantity": 12,
                    "target_price": "9.90", "erp_product_i_id": "IID-7", "erp_product_name": "Blue shirt",
                }]
                mapping, manual, _ = MODULE.generate(rows)
                planning = mapping["planning_tasks"][0]
                self.assertEqual(planning["target_task_status"], task_status)
                self.assertEqual(planning["items"][0]["description_spec"], "Blue / XL")
                self.assertEqual(planning["items"][0]["erp_product_i_id"], "IID-7")
                self.assertEqual(planning["code_rule_revision_id"], 9)
                self.assertEqual(planning["confidence"], "proposed_review")
                self.assertNotIn("blockers", planning)
                self.assertEqual(
                    planning["review_policy_ids"],
                    [
                        MODULE.LEGACY_PURCHASE_TO_PLANNING_POLICY,
                        MODULE.FROZEN_PLANNING_RULE_POLICY,
                    ],
                )
                self.assertEqual(
                    planning["manifest_row_hash"],
                    MODULE.sha256_json({
                        key: value
                        for key, value in planning.items()
                        if key != "manifest_row_hash"
                    }),
                )
                planning_review = next(row for row in manual if row["scope_kind"] == "planning")
                self.assertIn("code_rule_revision_id", planning_review["reason"])
                self.assertNotIn("image selection", planning_review["reason"])

    def test_planning_candidate_preserves_unique_exact_sku_product_image(self):
        rows = self.sample()
        rows["planning_rows"] = [{
            "task_id": 70, "task_status": "PendingAssign", "creator_id": 9,
            "task_sku_item_id": 701, "sku_code": "SKU-70",
            "description_spec": "Blue / XL", "quantity": 12,
            "target_price": None, "erp_product_i_id": "IID-7",
            "erp_product_name": "Blue shirt",
        }]
        rows["assets"].append({
            "id": 900, "asset_id": 901, "task_id": 70,
            "asset_type": "erp_product_image", "scope_sku_code": "SKU-70",
            "storage_ref_id": "ref-product-70", "upload_status": "uploaded",
            "is_archived": False, "superseded_by_version_id": None,
            "storage_owner_type": "task_asset", "storage_owner_id": 900,
            "ref_key": "planning/SKU-70.png",
            "storage_status": "recorded", "is_placeholder": False,
            "deleted_at": None, "cleaned_at": None,
            "access_revoked_at": None, "object_deleted_at": None,
        })

        mapping, _, _ = MODULE.generate(rows)

        planning = mapping["planning_tasks"][0]
        self.assertEqual(
            planning["items"][0]["image_storage_ref_id"],
            "ref-product-70",
        )
        self.assertEqual(planning["confidence"], "proposed_review")

    def test_planning_candidate_blocks_ambiguous_exact_sku_product_images(self):
        rows = self.sample()
        rows["planning_rows"] = [{
            "task_id": 70, "task_status": "PendingAssign", "creator_id": 9,
            "task_sku_item_id": 701, "sku_code": "SKU-70",
            "description_spec": "Blue / XL", "quantity": 12,
            "target_price": None, "erp_product_i_id": "IID-7",
            "erp_product_name": "Blue shirt",
        }]
        for asset_id, ref_id in ((900, "ref-product-a"), (901, "ref-product-b")):
            rows["assets"].append({
                "id": asset_id, "asset_id": asset_id + 100,
                "task_id": 70, "asset_type": "erp_product_image",
                "scope_sku_code": "SKU-70", "storage_ref_id": ref_id,
                "is_archived": False, "superseded_by_version_id": None,
                "storage_owner_type": "task_asset",
                "storage_owner_id": asset_id,
                "ref_key": f"planning/{asset_id}.png",
                "upload_status": "uploaded", "storage_status": "recorded",
                "is_placeholder": False, "deleted_at": None,
                "cleaned_at": None, "access_revoked_at": None,
                "object_deleted_at": None,
            })

        mapping, manual, _ = MODULE.generate(rows)

        planning = mapping["planning_tasks"][0]
        self.assertEqual(planning["confidence"], "hard_blocked")
        self.assertEqual(planning["items"][0]["image_storage_ref_id"], "")
        self.assertIn("900,901", planning["blockers"][0])
        planning_review = next(row for row in manual if row["scope_kind"] == "planning")
        self.assertEqual(planning_review["confidence"], "hard_blocked")

    def test_planning_candidate_rejects_archived_superseded_or_wrong_owner_images(self):
        base = {
            "id": 900, "asset_id": 901, "task_id": 70,
            "asset_type": "erp_product_image", "scope_sku_code": "SKU-70",
            "storage_ref_id": "ref-product-70", "upload_status": "uploaded",
            "is_archived": False, "superseded_by_version_id": None,
            "storage_owner_type": "task_asset", "storage_owner_id": 900,
            "ref_key": "planning/SKU-70.png",
            "storage_status": "recorded", "is_placeholder": False,
            "deleted_at": None, "cleaned_at": None,
            "access_revoked_at": None, "object_deleted_at": None,
        }
        cases = {
            "archived": {"is_archived": True},
            "superseded": {"superseded_by_version_id": 901},
            "wrong_owner_type": {"storage_owner_type": "reference"},
            "wrong_owner_id": {"storage_owner_id": 899},
        }
        for name, changes in cases.items():
            with self.subTest(name=name):
                rows = self.sample()
                rows["planning_rows"] = [{
                    "task_id": 70, "task_status": "PendingAssign",
                    "creator_id": 9, "task_sku_item_id": 701,
                    "sku_code": "SKU-70", "description_spec": "Blue / XL",
                    "quantity": 12, "target_price": None,
                    "erp_product_i_id": "IID-7",
                    "erp_product_name": "Blue shirt",
                }]
                rows["assets"].append({**base, **changes})
                mapping, _, _ = MODULE.generate(rows)
                self.assertEqual(
                    mapping["planning_tasks"][0]["items"][0]["image_storage_ref_id"],
                    "",
                )

    def test_planning_candidate_uses_space_trimmed_but_case_sensitive_sku_identity(self):
        planning_row = {
            "task_id": 70, "task_status": "PendingAssign",
            "creator_id": 9, "task_sku_item_id": 701,
            "sku_code": " SKU-70 ", "description_spec": "Blue / XL",
            "quantity": 12, "target_price": None,
            "erp_product_i_id": "IID-7", "erp_product_name": "Blue shirt",
        }
        asset = {
            "id": 900, "asset_id": 901, "task_id": 70,
            "asset_type": "erp_product_image",
            "scope_sku_code": "SKU-70",
            "storage_ref_id": "ref-product-70", "upload_status": "uploaded",
            "is_archived": False, "superseded_by_version_id": None,
            "storage_owner_type": "task_asset", "storage_owner_id": 900,
            "ref_key": " planning/SKU-70.png ",
            "storage_status": "recorded", "is_placeholder": False,
            "deleted_at": None, "cleaned_at": None,
            "access_revoked_at": None, "object_deleted_at": None,
        }
        rows = self.sample()
        rows["planning_rows"] = [planning_row]
        rows["assets"].append(asset)
        mapping, _, _ = MODULE.generate(rows)
        self.assertEqual(
            mapping["planning_tasks"][0]["items"][0]["image_storage_ref_id"],
            "ref-product-70",
        )

        rows["assets"][-1]["scope_sku_code"] = "sku-70"
        mapping, _, _ = MODULE.generate(rows)
        self.assertEqual(
            mapping["planning_tasks"][0]["items"][0]["image_storage_ref_id"],
            "",
        )

    def test_planning_candidate_rejects_case_variant_enum_values(self):
        rows = self.sample()
        rows["planning_rows"] = [{
            "task_id": 70, "task_status": "PendingAssign",
            "creator_id": 9, "task_sku_item_id": 701,
            "sku_code": "SKU-70", "description_spec": "Blue / XL",
            "quantity": 12, "target_price": None,
            "erp_product_i_id": "IID-7", "erp_product_name": "Blue shirt",
        }]
        rows["assets"].append({
            "id": 900, "asset_id": 901, "task_id": 70,
            "asset_type": "ERP_PRODUCT_IMAGE",
            "scope_sku_code": "SKU-70",
            "storage_ref_id": "ref-product-70", "upload_status": "uploaded",
            "is_archived": False, "superseded_by_version_id": None,
            "storage_owner_type": "task_asset", "storage_owner_id": 900,
            "ref_key": "planning/SKU-70.png",
            "storage_status": "recorded", "is_placeholder": False,
            "deleted_at": None, "cleaned_at": None,
            "access_revoked_at": None, "object_deleted_at": None,
        })
        mapping, _, _ = MODULE.generate(rows)
        self.assertEqual(
            mapping["planning_tasks"][0]["items"][0]["image_storage_ref_id"],
            "",
        )

    def test_planning_candidate_uses_erp_product_snapshot_when_legacy_description_is_empty(self):
        rows = self.sample()
        rows["planning_rows"] = [{
            "task_id": 70, "task_status": "PendingAssign", "creator_id": 9,
            "task_sku_item_id": 701, "description_spec": "", "quantity": 12,
            "target_price": None, "erp_product_i_id": "IID-7",
            "erp_product_name": "ERP blue shirt",
        }]
        mapping, manual, _ = MODULE.generate(rows)
        planning = mapping["planning_tasks"][0]
        self.assertEqual(planning["items"][0]["description_spec"], "ERP blue shirt")
        self.assertEqual(planning["confidence"], "proposed_review")
        self.assertNotIn("blockers", planning)
        self.assertIn(
            MODULE.PRODUCT_NAME_DESCRIPTION_FALLBACK_POLICY,
            planning["review_policy_ids"],
        )
        planning_review = next(row for row in manual if row["scope_kind"] == "planning")
        self.assertIn(
            "task_sku_items.product_name_snapshot as description_spec",
            planning_review["reason"],
        )

    def test_planning_candidate_maps_retired_downstream_statuses_to_completed(self):
        for task_status in ("PendingClose", "PendingProductionTransfer"):
            with self.subTest(task_status=task_status):
                rows = self.sample()
                rows["planning_rows"] = [{
                    "task_id": 70, "task_status": task_status, "creator_id": 9,
                    "task_sku_item_id": 701, "description_spec": "",
                    "quantity": 12, "target_price": None,
                    "erp_product_i_id": "", "erp_product_name": "ERP blue shirt",
                }]
                mapping, manual, _ = MODULE.generate(rows)
                planning = mapping["planning_tasks"][0]
                self.assertEqual(planning["target_task_status"], "Completed")
                self.assertEqual(planning["confidence"], "proposed_review")
                self.assertIn(
                    MODULE.RETIRED_PLANNING_STATUS_POLICY,
                    planning["review_policy_ids"],
                )
                planning_review = next(row for row in manual if row["scope_kind"] == "planning")
                self.assertIn(
                    f"map retired task_status {task_status} to Completed",
                    planning_review["reason"],
                )

    def test_incomplete_uat_planning_candidate_becomes_cancelled_tombstone(self):
        rows = self.sample()
        rows["planning_rows"] = [{
            "task_id": 497, "task_status": "PendingAssign", "creator_id": 1,
            "task_sku_item_id": 380, "description_spec": "", "quantity": None,
            "target_price": None, "erp_product_i_id": "", "erp_product_name": "",
        }]
        mapping, manual, _ = MODULE.generate(rows)
        planning = mapping["planning_tasks"][0]
        self.assertEqual(planning["target_task_status"], "Cancelled")
        self.assertEqual(planning["code_rule_revision_id"], 9)
        self.assertEqual(planning["confidence"], "proposed_review")
        self.assertEqual(
            planning["items"],
            [{
                "task_sku_item_id": 380,
                "description_spec": "",
                "quantity": 0,
                "target_price": None,
                "note": "",
                "reference_url": "",
                "erp_product_i_id": "",
                "erp_product_name": "",
                "image_storage_ref_id": "",
            }],
        )
        self.assertEqual(
            planning["review_policy_ids"],
            [
                MODULE.LEGACY_PURCHASE_TO_PLANNING_POLICY,
                MODULE.INCOMPLETE_UAT_PLANNING_TOMBSTONE_POLICY,
                MODULE.FROZEN_PLANNING_RULE_POLICY,
            ],
        )
        planning_review = next(row for row in manual if row["scope_kind"] == "planning")
        self.assertEqual(planning_review["confidence"], "proposed_review")
        self.assertIn("no fabricated", planning_review["reason"])

    def test_task_reference_inherits_into_child_scope(self):
        scope = {"task_id": 7, "scope_kind": "sku", "scope_ref_id": 2, "sku_code": "SKU-B"}
        task_reference = {"task_id": 7, "scope_sku_code": "", "retouch_requirement_id": None}
        self.assertTrue(MODULE.reference_scope_matches(scope, task_reference))

    def test_deleted_retouch_scope_is_retained_for_read_only_history(self):
        rows = self.sample(); rows["scopes"][0].update({"scope_kind": "retouch_requirement", "scope_ref_id": 9, "deleted_at": "2026-01-05T00:00:00Z"})
        for asset in rows["assets"]: asset["retouch_requirement_id"] = 9
        mapping, manual, _ = MODULE.generate(rows)
        self.assertEqual(mapping["resources"][0]["scope_ref_id"], 9)
        self.assertIn("read-only history", manual[0]["reason"])

    def test_object_manifest_uses_verifier_contract(self):
        rows = self.sample(); rows["assets"][0].update({"storage_ref_id": "ref-10", "storage_adapter": "oss", "storage_key": "k/10", "file_size": 12, "mime_type": "application/octet-stream", "storage_status": "active", "is_placeholder": False})
        manifest = MODULE.build_object_manifest(rows); row = next(r for r in manifest if r["owner_id"] == 10)
        self.assertEqual(row["entity_key"], "task_asset:10"); self.assertIn("object_key", row); self.assertNotIn("storage_key", row)

    def test_deleted_asset_recovery_candidates_are_exact_and_12323_is_honest_tombstone(self):
        assets = []
        for missing_id, evidence in MODULE.LEGACY_DELETED_ASSET_RECOVERY_EVIDENCE.items():
            assets.append({
                "id": missing_id,
                "task_id": evidence["task_id"],
                "file_size": evidence["expected_file_size"],
                "storage_ref_id": evidence["original_storage_ref_id"],
                "asset_type": "delivery",
            })
            source_id = evidence["recovery_source_task_asset_id"]
            if source_id:
                assets.append({
                    "id": source_id,
                    "task_id": 2098,
                    "file_size": evidence["expected_file_size"],
                    "storage_ref_id": evidence["recovery_source_storage_ref_id"],
                    "asset_type": "delivery",
                })
                for asset_type, key in (
                    ("preview", "preview_whole_hash"),
                    ("design_thumb", "design_thumb_whole_hash"),
                ):
                    assets.append({
                        "id": source_id * 10 + (1 if asset_type == "preview" else 2),
                        "task_id": 2098,
                        "asset_type": asset_type,
                        "source_asset_version_id": source_id,
                        "whole_hash": evidence[key],
                    })
            for rejected_id, size in zip(
                evidence["rejected_source_task_asset_ids"],
                (17595421, 11275123),
            ):
                assets.append({
                    "id": rejected_id,
                    "task_id": evidence["task_id"],
                    "file_size": size,
                    "storage_ref_id": f"rejected-{rejected_id}",
                    "asset_type": "delivery",
                })
            for asset_type, key in (
                ("preview", "preview_whole_hash"),
                ("design_thumb", "design_thumb_whole_hash"),
            ):
                assets.append({
                    "id": missing_id * 10 + (1 if asset_type == "preview" else 2),
                    "task_id": evidence["task_id"],
                    "asset_type": asset_type,
                    "source_asset_version_id": missing_id,
                    "whole_hash": evidence[key],
                })
        recoveries, manual = MODULE.build_deleted_asset_recoveries(
            {"assets": assets}
        )
        self.assertEqual(
            [row["missing_task_asset_id"] for row in recoveries],
            [12323, 23989, 23990, 23991],
        )
        unresolved = recoveries[0]
        self.assertEqual(unresolved["confidence"], "proposed_review")
        self.assertEqual(
            unresolved["rejected_source_task_asset_ids"], [14510, 14514]
        )
        self.assertEqual(unresolved["recovery_source_task_asset_id"], 0)
        self.assertEqual(
            unresolved["strategy"],
            "historical_unavailable_tombstone_v1",
        )
        self.assertEqual(
            unresolved["review_policy_ids"],
            [MODULE.HISTORICAL_ASSET_UNAVAILABLE_POLICY],
        )
        self.assertNotIn("blockers", unresolved)
        self.assertEqual(
            [row["confidence"] for row in recoveries[1:]],
            ["proposed_review"] * 3,
        )
        self.assertEqual(len(manual), 4)

        source_preview = next(
            row
            for row in assets
            if row.get("id") == 24034 * 10 + 1
        )
        source_preview["source_asset_version_id"] = 999
        with self.assertRaisesRegex(
            ValueError, "lacks pairwise-identical derivative hashes"
        ):
            MODULE.build_deleted_asset_recoveries({"assets": assets})

    def test_deleted_asset_recovery_rejects_size_drift(self):
        rows = {"assets": [
            {
                "id": missing_id,
                "task_id": evidence["task_id"],
                "file_size": (
                    evidence["expected_file_size"] + 1
                    if missing_id == 12323
                    else evidence["expected_file_size"]
                ),
                "storage_ref_id": evidence["original_storage_ref_id"],
            }
            for missing_id, evidence in
            MODULE.LEGACY_DELETED_ASSET_RECOVERY_EVIDENCE.items()
        ]}
        with self.assertRaisesRegex(ValueError, "differs from the frozen"):
            MODULE.build_deleted_asset_recoveries(rows)

    def test_scope_does_not_cross_sku(self):
        scope = {"task_id": 7, "scope_kind": "sku", "scope_ref_id": 2, "sku_code": "SKU-B"}
        self.assertFalse(MODULE.scope_matches(scope, {"task_id": 7, "scope_sku_code": "SKU-A", "retouch_requirement_id": None}))
        self.assertTrue(MODULE.scope_matches(scope, {"task_id": 7, "scope_sku_code": "SKU-B", "retouch_requirement_id": None}))

    def test_remote_host_is_rejected(self):
        self.assertFalse(MODULE.is_loopback("yongbo.cloud")); self.assertTrue(MODULE.is_loopback("127.0.0.1"))

    def test_outputs_are_hashed_and_not_reused(self):
        mapping, manual, objects = MODULE.generate(self.sample())
        with tempfile.TemporaryDirectory() as root:
            out = pathlib.Path(root) / "run"; MODULE.write_outputs(out, mapping, manual, objects)
            self.assertTrue((out / "manifest_hashes.json").is_file())
            with self.assertRaises(FileExistsError): MODULE.write_outputs(out, mapping, manual, objects)


if __name__ == "__main__":
    unittest.main()
