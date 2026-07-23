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
        mixed = self.timezone_truth()
        mixed["timezone_truth"][0]["near_eight_hour_asset_created_events"] -= 1
        with self.assertRaisesRegex(RuntimeError, "refusing blanket normalization"):
            MODULE.validate_legacy_timezone_truth(mixed)
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
        self.assertEqual(outsource["action"], "")
        self.assertEqual(outsource["confidence"], "hard_blocked")
        self.assertTrue(outsource["blockers"])

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

    def test_multiple_sources_are_hard_blocked(self):
        rows = self.sample(); rows["assets"].append({**rows["assets"][0], "id": 12})
        rows["events"][2]["payload"]["asset_version_ids"].append(12)
        mapping, manual, _ = MODULE.generate(rows)
        self.assertEqual(mapping["resources"][0]["history"][0]["confidence"], "hard_blocked"); self.assertIn("ZIP", manual[0]["reason"])

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
        duplicate["assets"].append({
            **duplicate["assets"][0], "id": 12, "asset_id": 102,
            "upload_session_id": "session-a-2", "upload_request_id": "session-a-2",
            "created_at": "2026-01-01T08:30:00Z",
        })
        duplicate["events"].insert(1, {
            **duplicate["events"][0], "id": "upload-a-2", "sequence": 2,
            "payload": {
                "upload_session_id": "session-a-2", "asset_id": 102,
                "asset_type": "delivery", "asset_version_id": 12,
                "target_sku_code": "SKU-A",
            },
            "created_at": "2026-01-01T08:30:00Z",
        })
        mutations["duplicate SKU membership"] = duplicate

        actor_conflict = self.atomic_batch_sample()
        actor_conflict["events"][0]["actor_id"] = 4
        mutations["different completion actor"] = actor_conflict

        boundary_conflict = self.atomic_batch_sample()
        boundary_conflict["events"].append({
            "namespace": "task_event_log", "id": "approve", "task_id": 7,
            "sequence": 4, "event_type": "task.audit.approved", "actor_id": 5,
            "module_key": "audit", "payload": {},
            "created_at": "2026-01-02T00:00:00Z",
        })
        mutations["competing workflow boundary"] = boundary_conflict

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
            "'flow_review_status'", "'rejected_at'", "'superseded_by_version_id'", "'superseded_at'", "DATE_FORMAT(DATE_SUB(ta.superseded_at", "'sequence',e.sequence",
            "'module_key'", "'from_state'", "'to_state'",
        ):
            self.assertIn(field, MODULE.SQL)

    def test_planning_candidates_preserve_legal_status_and_export_sku_truth(self):
        for task_status in ("Cancelled", "PendingAssign", "Completed"):
            with self.subTest(task_status=task_status):
                rows = self.sample()
                rows["planning_rows"] = [{
                    "task_id": 70, "task_status": task_status, "creator_id": 9,
                    "task_sku_item_id": 701, "description_spec": "Blue / XL", "quantity": 12,
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

    def test_planning_candidate_remains_hard_when_name_and_quantity_are_unprovable(self):
        rows = self.sample()
        rows["planning_rows"] = [{
            "task_id": 497, "task_status": "PendingAssign", "creator_id": 1,
            "task_sku_item_id": 380, "description_spec": "", "quantity": None,
            "target_price": None, "erp_product_i_id": "", "erp_product_name": "",
        }]
        mapping, manual, _ = MODULE.generate(rows)
        planning = mapping["planning_tasks"][0]
        self.assertEqual(planning["code_rule_revision_id"], 9)
        self.assertEqual(planning["confidence"], "hard_blocked")
        self.assertEqual(
            planning["review_policy_ids"],
            [
                MODULE.LEGACY_PURCHASE_TO_PLANNING_POLICY,
                MODULE.FROZEN_PLANNING_RULE_POLICY,
            ],
        )
        self.assertIn(
            "SKU item 380 lacks both description_spec and ERP product_name snapshot",
            planning["blockers"],
        )
        self.assertIn("SKU item 380 lacks positive quantity", planning["blockers"])
        planning_review = next(row for row in manual if row["scope_kind"] == "planning")
        self.assertEqual(planning_review["confidence"], "hard_blocked")

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
