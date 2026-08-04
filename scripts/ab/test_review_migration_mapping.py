import hashlib
import json
import pathlib
import tempfile
import unittest

import review_migration_mapping as review


def revision(
    revision_no=1,
    *,
    confidence="proposed_review",
    status="finalized",
    stage="design",
    alias=101,
    blockers=None,
    policies=None,
    reason="candidate",
):
    if policies is None:
        policies = ["explicit_event_replay"]
        if alias is not None:
            policies.append("delivery_source_alias")
        if status == "rejected":
            policies.append("rejected_history")
        if stage == "reopen":
            policies.append("reopen")
    value = {
        "revision_no": revision_no,
        "status": status,
        "mode": "single",
        "source_stage": stage,
        "final_task_asset_ids": [101],
        "reference_file_ref_ids": [],
        "evidence_event_ids": [f"task_event_log:event-{revision_no}"],
        "confidence": confidence,
        "review_policy_ids": policies,
        "confirmed_by": 0,
        "confirmed_at": review.ZERO_TIME,
        "confirmation_note": "",
        "reason": reason,
        "created_by": 9,
        "created_at": "2026-07-22T10:00:00Z",
        "submitted_at": "2026-07-22T10:00:00Z",
        "finalized_at": "2026-07-22T10:01:00Z",
    }
    if alias is not None:
        value["source_alias_from_task_asset_id"] = alias
    if blockers is not None:
        value["blockers"] = blockers
    value["manifest_row_hash"] = review.canonical_revision_hash(value)
    return value


def planning(*, confidence="proposed_review", blockers=None, policies=None):
    value = {
        "task_id": 70,
        "target_task_status": "PendingAssign",
        "code_rule_revision_id": 9,
        "created_by": 9,
        "confidence": confidence,
        "review_policy_ids": policies
        or [
            "legacy_purchase_to_sku_planning_v1",
            "frozen_sku_planning_rule_revision_9_v1",
        ],
        "confirmed_by": 0,
        "confirmed_at": review.ZERO_TIME,
        "confirmation_note": "",
        "items": [
            {
                "task_sku_item_id": 701,
                "description_spec": "Blue / XL",
                "quantity": 2,
                "target_price": None,
                "note": "",
                "reference_url": "",
                "erp_product_i_id": "IID-7",
                "erp_product_name": "Blue shirt",
                "image_storage_ref_id": "",
            }
        ],
    }
    if blockers is not None:
        value["blockers"] = blockers
    value["manifest_row_hash"] = review.canonical_planning_hash(value)
    return value


def organization(
    *,
    subject_id=7,
    confidence="proposed_review",
    blockers=None,
):
    value = {
        "subject_type": "task",
        "subject_id": subject_id,
        "legacy_department": "运营部",
        "legacy_team": "拼多多池州组",
        "from_department_id": None,
        "from_team_id": None,
        "target_department_id": 6 if confidence != "hard_blocked" else 0,
        "target_team_id": 30 if confidence != "hard_blocked" else 0,
        "confidence": confidence,
        "review_policy_ids": [
            "legacy_org_alias_lineage_v1"
            if confidence != "hard_blocked"
            else "legacy_org_manual_target_required_v1"
        ],
        "confirmed_by": 0,
        "confirmed_at": review.ZERO_TIME,
        "confirmation_note": "",
    }
    if blockers is not None:
        value["blockers"] = blockers
    value["manifest_row_hash"] = review.canonical_mapping_row_hash(value)
    return value


def access(
    *,
    user_id=31,
    legacy_role="Warehouse",
    confidence="proposed_review",
    blockers=None,
):
    value = {
        "user_id": user_id,
        "legacy_role": legacy_role,
        "action": "no_new_grant" if confidence != "hard_blocked" else "",
        "required_existing_assignments": [
            {
                "role_code": "member",
                "scope_mode": "self",
                "source_type": "direct",
                "source_ref_id": 0,
            }
        ],
        "confidence": confidence,
        "review_policy_ids": [
            "retired_warehouse_no_new_grant_v1"
            if confidence != "hard_blocked"
            else "legacy_outsource_access_decision_v1"
        ],
        "confirmed_by": 0,
        "confirmed_at": review.ZERO_TIME,
        "confirmation_note": "",
    }
    if blockers is not None:
        value["blockers"] = blockers
    value["manifest_row_hash"] = review.canonical_mapping_row_hash(value)
    return value


def task_state(*, task_id=981, confidence="proposed_review", blockers=None):
    value = {
        "task_id": task_id,
        "from_status": "Completed",
        "target_status": "InProgress",
        "evidence_event_ids": ["task_event_log:submit-981"],
        "confidence": confidence,
        "review_policy_ids": [
            "legacy_retouch_premature_terminal_partial_v1"
        ],
        "confirmed_by": 0,
        "confirmed_at": review.ZERO_TIME,
        "confirmation_note": "",
    }
    if blockers is not None:
        value["blockers"] = blockers
    value["manifest_row_hash"] = review.canonical_mapping_row_hash(value)
    return value


def asset_recovery(
    *,
    missing_task_asset_id=12323,
    confidence="proposed_review",
    blockers=None,
):
    if missing_task_asset_id == 12323:
        value = {
            "task_id": 2199,
            "missing_task_asset_id": 12323,
            "recovery_source_task_asset_id": 0,
            "rejected_source_task_asset_ids": [14510, 14514],
            "strategy": "historical_unavailable_tombstone_v1",
            "original_storage_ref_id": (
                "c0a135a1-080f-46a0-a41a-461aef0ea0fb"
            ),
            "expected_file_size": 17755216,
            "preview_whole_hash": (
                "82b35a045540d27f9656d6d02c99eb2814a62e9d048d33b20823fb8c0017aa4c"
            ),
            "design_thumb_whole_hash": (
                "54dbf569874243a212c11c3e83e80f19944c2581f12c9473a793bc273ec666a3"
            ),
            "object_probe_result": "not_found",
            "object_probe_input_manifest_sha256": (
                "3f17b37296d2670235ca9bfcfd4388823b81adecf8fbac0826e6f241923579c7"
            ),
            "object_probe_evidence_hash": (
                "f1c78819e1f3d5f4e7a4b25ff3d173368574a5639f4c6df45c8aae5482d047b8"
            ),
            "object_probe_object_key_sha256": (
                "e732f6cd269a93d6bac168b0852dbcf8480af8966847278cb073cd6905b0efdd"
            ),
            "object_probe_read_only_get_count": 1,
            "review_policy_ids": [
                "legacy_historical_asset_unavailable_v1"
            ],
        }
    else:
        source_sha256 = {
            23989: (
                "d0558b1a9d4a7afed5a03b6b97d4a765d34050866686e396ab0acf9f08f0dec5"
            ),
            23990: (
                "64cdfed11adc778fb6ede7f03c49f7c70e8655870236bdcd92a8207e41a8dfb8"
            ),
            23991: (
                "ebfecf3407e05c576bcddf74673d2e7568207ecc27855aa0e08c453d5a0d119a"
            ),
        }[missing_task_asset_id]
        value = {
            "task_id": 2807,
            "missing_task_asset_id": missing_task_asset_id,
            "recovery_source_task_asset_id": {
                23989: 24034,
                23990: 24033,
                23991: 24040,
            }[missing_task_asset_id],
            "rejected_source_task_asset_ids": [],
            "strategy": "verified_oss_recovery_v1",
            "original_storage_ref_id": f"missing-{missing_task_asset_id}",
            "expected_file_size": 683000,
            "preview_whole_hash": "",
            "design_thumb_whole_hash": "",
            "review_policy_ids": ["legacy_deleted_asset_recovery_v1"],
            "controlled_read_protocol": "controlled-asset-read-v1",
            "controlled_read_evidence_sha256": (
                "b39e0d9d26e6fdd35941b195bdc413eb12dd6795e23276a48c9b9bd49f829b08"
            ),
            "recovery_source_sha256": source_sha256,
        }
    value.update(
        {
            "confidence": confidence,
            "confirmed_by": 0,
            "confirmed_at": review.ZERO_TIME,
            "confirmation_note": "",
        }
    )
    if blockers is not None:
        value["blockers"] = blockers
    value["manifest_row_hash"] = review.canonical_mapping_row_hash(value)
    return value


def mapping(
    resources=None,
    planning_tasks=None,
    task_state_decisions=None,
    organization_mappings=None,
    access_decisions=None,
    asset_recoveries=None,
):
    return {
        "version": 2,
        "resources": resources
        if resources is not None
        else [
            {
                "task_id": 7,
                "scope_kind": "task",
                "scope_ref_id": 0,
                "history": [revision()],
                "working_revision_no": 1,
                "finalized_revision_no": 1,
            }
        ],
        "planning_tasks": planning_tasks or [],
        "task_state_decisions": task_state_decisions or [],
        "organization_mappings": organization_mappings or [],
        "access_decisions": access_decisions or [],
        "asset_recoveries": asset_recoveries or [],
    }


class ReviewMigrationMappingTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.root = pathlib.Path(self.directory.name)
        self.candidate = self.root / "candidate.json"
        self.ledger = self.root / "ledger.json"
        self.template = self.root / "decision-template.json"
        self.prepare_evidence = self.root / "prepare-evidence.json"
        self.decision = self.root / "decision.json"
        self.output = self.root / "reviewed.json"
        self.apply_evidence = self.root / "apply-evidence.json"

    def tearDown(self):
        self.directory.cleanup()

    def write_candidate(self, value=None):
        self.candidate.write_text(
            json.dumps(value or mapping(), ensure_ascii=False, separators=(",", ":")),
            encoding="utf-8",
        )

    def prepare(self):
        review.prepare(
            self.candidate, self.ledger, self.template, self.prepare_evidence
        )

    def approve(self, policies, **overrides):
        decision = json.loads(self.template.read_text(encoding="utf-8"))
        decision.update(
            {
                "decision": "approve",
                "reviewer_id": 88,
                "reviewed_at": "2026-07-22T18:30:00+08:00",
                "note": "Approved against the frozen candidate evidence.",
                "approved_policies": policies,
            }
        )
        decision.update(overrides)
        self.decision.write_text(
            json.dumps(decision, ensure_ascii=False, separators=(",", ":")),
            encoding="utf-8",
        )

    def apply(self):
        review.apply_review(
            self.candidate,
            self.ledger,
            self.decision,
            self.output,
            self.apply_evidence,
        )

    def test_prepare_classifies_required_policies_and_is_deterministic(self):
        retouch = revision(
            alias=None,
            status="rejected",
            stage="reopen",
            policies=[
                "explicit_event_replay",
                "rejected_history",
                "reopen",
                "retouch_source_optional",
            ],
        )
        resource = {
            "task_id": 9,
            "scope_kind": "retouch_requirement",
            "scope_ref_id": 31,
            "history": [retouch],
            "working_revision_no": 1,
        }
        self.write_candidate(mapping([resource]))
        candidate_before = self.candidate.read_bytes()
        self.prepare()
        ledger = json.loads(self.ledger.read_text(encoding="utf-8"))
        self.assertEqual(
            ledger["rows"][0]["required_policies"],
            [
                "explicit_event_replay",
                "rejected_history",
                "reopen",
                "retouch_source_optional",
            ],
        )
        first = (self.ledger.read_bytes(), self.template.read_bytes())
        self.prepare()
        self.assertEqual(first, (self.ledger.read_bytes(), self.template.read_bytes()))
        self.assertEqual(candidate_before, self.candidate.read_bytes())

    def test_no_new_grant_allows_proven_empty_existing_assignments(self):
        decision = access()
        decision["required_existing_assignments"] = []
        decision["manifest_row_hash"] = review.canonical_mapping_row_hash(
            decision
        )
        self.write_candidate(mapping(access_decisions=[decision]))
        self.prepare()
        ledger = json.loads(self.ledger.read_text(encoding="utf-8"))
        access_row = next(
            row for row in ledger["rows"] if row["row_type"] == "access"
        )
        self.assertTrue(access_row["eligible"])

    def test_review_promotes_org_and_access_but_keeps_hard_rows_byte_identical(self):
        hard_org = organization(
            subject_id=8,
            confidence="hard_blocked",
            blockers=["manual target required"],
        )
        hard_access = access(
            user_id=233,
            legacy_role="Outsource",
            confidence="hard_blocked",
            blockers=["explicit replacement decision required"],
        )
        candidate = mapping(
            organization_mappings=[organization(), hard_org],
            access_decisions=[access(), hard_access],
        )
        self.write_candidate(candidate)
        self.prepare()
        ledger = json.loads(self.ledger.read_text(encoding="utf-8"))
        self.assertEqual(ledger["summary"]["organization"]["eligible_count"], 1)
        self.assertEqual(ledger["summary"]["organization"]["excluded_count"], 1)
        self.assertEqual(ledger["summary"]["access"]["eligible_count"], 1)
        self.assertEqual(ledger["summary"]["access"]["excluded_count"], 1)

        self.approve(
            [
                "explicit_event_replay",
                "delivery_source_alias",
                "legacy_org_alias_lineage_v1",
                "retired_warehouse_no_new_grant_v1",
            ]
        )
        self.apply()
        reviewed = json.loads(self.output.read_text(encoding="utf-8"))
        promoted_org = reviewed["organization_mappings"][0]
        promoted_access = reviewed["access_decisions"][0]
        self.assertEqual(promoted_org["confidence"], "confirmed_auto")
        self.assertEqual(promoted_access["confidence"], "confirmed_auto")
        self.assertEqual(promoted_org["confirmed_by"], 88)
        self.assertEqual(promoted_access["confirmed_by"], 88)
        self.assertEqual(
            promoted_org["manifest_row_hash"],
            review.canonical_mapping_row_hash(promoted_org),
        )
        self.assertEqual(
            promoted_access["manifest_row_hash"],
            review.canonical_mapping_row_hash(promoted_access),
        )
        self.assertEqual(reviewed["organization_mappings"][1], hard_org)
        self.assertEqual(reviewed["access_decisions"][1], hard_access)

    def test_batch_reason_requires_explicit_batch_policy(self):
        reason = (
            "policy legacy_multi_sku_atomic_batch_submit_v1: "
            "exact per-SKU membership"
        )
        candidate = mapping(
            [
                {
                    "task_id": 7,
                    "scope_kind": "task",
                    "scope_ref_id": 0,
                    "history": [revision(reason=reason)],
                    "working_revision_no": 1,
                    "finalized_revision_no": 1,
                }
            ]
        )
        self.write_candidate(candidate)
        with self.assertRaisesRegex(ValueError, "omits"):
            self.prepare()
        candidate["resources"][0]["history"][0][
            "review_policy_ids"
        ].append("legacy_multi_sku_atomic_batch_submit_v1")
        candidate["resources"][0]["history"][0][
            "manifest_row_hash"
        ] = review.canonical_revision_hash(candidate["resources"][0]["history"][0])
        self.write_candidate(candidate)
        self.prepare()
        ledger = json.loads(self.ledger.read_text(encoding="utf-8"))
        self.assertIn(
            "legacy_multi_sku_atomic_batch_submit_v1",
            ledger["rows"][0]["required_policies"],
        )
        catalog = {
            row["policy"]: row["description"]
            for row in ledger["policy_catalog"]
        }
        self.assertIn(
            "the last scoped submit",
            catalog["legacy_multi_sku_atomic_batch_submit_v1"],
        )
        self.assertNotIn(
            "one task-wide submit",
            catalog["legacy_multi_sku_atomic_batch_submit_v1"],
        )

    def test_retouch_terminal_reason_requires_explicit_policy(self):
        reason = (
            "policy legacy_retouch_terminal_submit_v1: "
            "one scope-proven final"
        )
        candidate = mapping(
            [
                {
                    "task_id": 9,
                    "scope_kind": "retouch_requirement",
                    "scope_ref_id": 31,
                    "history": [
                        revision(
                            alias=None,
                            reason=reason,
                            policies=[
                                "explicit_event_replay",
                                "retouch_source_optional",
                            ],
                        )
                    ],
                    "working_revision_no": 1,
                    "finalized_revision_no": 1,
                }
            ]
        )
        self.write_candidate(candidate)
        with self.assertRaisesRegex(ValueError, "omits"):
            self.prepare()
        candidate["resources"][0]["history"][0][
            "review_policy_ids"
        ].append("legacy_retouch_terminal_submit_v1")
        candidate["resources"][0]["history"][0][
            "manifest_row_hash"
        ] = review.canonical_revision_hash(candidate["resources"][0]["history"][0])
        self.write_candidate(candidate)
        self.prepare()
        ledger = json.loads(self.ledger.read_text(encoding="utf-8"))
        self.assertIn(
            "legacy_retouch_terminal_submit_v1",
            ledger["rows"][0]["required_policies"],
        )

    def test_post_close_reason_requires_explicit_policy(self):
        candidate = mapping(
            [
                {
                    "task_id": 9,
                    "scope_kind": "task",
                    "scope_ref_id": 0,
                    "history": [
                        revision(
                            reason=(
                                "policy legacy_post_close_replacement_v1: "
                                "exact same-root successor"
                            ),
                            policies=["explicit_event_replay", "reopen"],
                        )
                    ],
                    "working_revision_no": 1,
                    "finalized_revision_no": 1,
                }
            ]
        )
        self.write_candidate(candidate)
        with self.assertRaisesRegex(ValueError, "omits"):
            self.prepare()
        candidate["resources"][0]["history"][0][
            "review_policy_ids"
        ].append("legacy_post_close_replacement_v1")
        candidate["resources"][0]["history"][0][
            "manifest_row_hash"
        ] = review.canonical_revision_hash(candidate["resources"][0]["history"][0])
        self.write_candidate(candidate)
        self.prepare()

    def test_hard_sibling_excludes_entire_resource(self):
        proposed = revision()
        hard = revision(
            2,
            confidence="hard_blocked",
            status="rejected",
            stage="reopen",
            blockers=["ambiguous boundary"],
        )
        resource = {
            "task_id": 7,
            "scope_kind": "task",
            "scope_ref_id": 0,
            "history": [proposed, hard],
            "working_revision_no": 2,
            "finalized_revision_no": 1,
        }
        self.write_candidate(mapping([resource]))
        self.prepare()
        ledger = json.loads(self.ledger.read_text(encoding="utf-8"))
        self.assertFalse(ledger["rows"][0]["eligible"])
        self.assertIn(
            "resource_has_hard_blocked_sibling",
            ledger["rows"][0]["exclusion_reasons"],
        )
        self.assertFalse(ledger["rows"][1]["eligible"])

    def test_apply_promotes_only_fully_approved_eligible_rows(self):
        self.write_candidate()
        candidate_before = self.candidate.read_bytes()
        self.prepare()
        self.approve(["explicit_event_replay", "delivery_source_alias"])
        self.apply()
        reviewed = json.loads(self.output.read_text(encoding="utf-8"))
        confirmed = reviewed["resources"][0]["history"][0]
        self.assertEqual(confirmed["confidence"], "confirmed_auto")
        self.assertEqual(confirmed["confirmed_by"], 88)
        self.assertEqual(confirmed["confirmed_at"], "2026-07-22T10:30:00Z")
        self.assertNotIn("blockers", confirmed)
        self.assertEqual(
            confirmed["manifest_row_hash"],
            review.canonical_revision_hash(confirmed),
        )
        self.assertEqual(candidate_before, self.candidate.read_bytes())
        evidence = json.loads(self.apply_evidence.read_text(encoding="utf-8"))
        self.assertEqual(evidence["promoted_revision_count"], 1)
        self.assertEqual(
            evidence["reviewed_mapping_sha256"],
            hashlib.sha256(self.output.read_bytes()).hexdigest(),
        )

    def test_apply_with_partial_policy_approval_keeps_candidate_row(self):
        self.write_candidate()
        self.prepare()
        self.approve(["explicit_event_replay"])
        self.apply()
        reviewed = json.loads(self.output.read_text(encoding="utf-8"))
        self.assertEqual(
            reviewed["resources"][0]["history"][0]["confidence"],
            "proposed_review",
        )
        evidence = json.loads(self.apply_evidence.read_text(encoding="utf-8"))
        self.assertEqual(evidence["promoted_revision_count"], 0)

    def test_planning_is_separately_counted_and_requires_every_policy(self):
        policies = [
            "legacy_purchase_to_sku_planning_v1",
            "frozen_sku_planning_rule_revision_9_v1",
            "product_name_snapshot_description_fallback_v1",
            "retired_planning_status_to_completed_v1",
        ]
        self.write_candidate(mapping([], [planning(policies=policies)]))
        self.prepare()
        ledger = json.loads(self.ledger.read_text(encoding="utf-8"))
        self.assertEqual(ledger["summary"]["revision"]["row_count"], 0)
        self.assertEqual(ledger["summary"]["planning"]["row_count"], 1)
        self.assertEqual(ledger["summary"]["planning"]["eligible_count"], 1)
        self.assertEqual(ledger["rows"][0]["row_type"], "planning")
        self.approve(policies[:-1])
        self.apply()
        reviewed = json.loads(self.output.read_text(encoding="utf-8"))
        self.assertEqual(
            reviewed["planning_tasks"][0]["confidence"], "proposed_review"
        )
        self.assertEqual(
            json.loads(self.apply_evidence.read_text(encoding="utf-8"))[
                "promoted_planning_count"
            ],
            0,
        )

        self.approve(policies)
        self.apply()
        reviewed = json.loads(self.output.read_text(encoding="utf-8"))
        confirmed = reviewed["planning_tasks"][0]
        self.assertEqual(confirmed["confidence"], "confirmed_auto")
        self.assertEqual(
            confirmed["manifest_row_hash"],
            review.canonical_planning_hash(confirmed),
        )
        self.assertEqual(
            json.loads(self.apply_evidence.read_text(encoding="utf-8"))[
                "promoted_planning_count"
            ],
            1,
        )

    def test_hard_planning_is_never_upgraded(self):
        hard = planning(
            confidence="hard_blocked", blockers=["quantity cannot be proven"]
        )
        self.write_candidate(mapping([], [hard]))
        self.prepare()
        self.approve(list(review.POLICIES))
        self.apply()
        reviewed = json.loads(self.output.read_text(encoding="utf-8"))
        self.assertEqual(reviewed["planning_tasks"][0], hard)

    def test_hard_blocked_revision_is_byte_equivalent_inside_output(self):
        hard = revision(
            confidence="hard_blocked", blockers=["cannot prove asset membership"]
        )
        resource = {
            "task_id": 7,
            "scope_kind": "task",
            "scope_ref_id": 0,
            "history": [hard],
            "working_revision_no": 1,
            "finalized_revision_no": 1,
        }
        candidate = mapping([resource])
        self.write_candidate(candidate)
        self.prepare()
        self.approve(list(review.POLICIES))
        self.apply()
        reviewed = json.loads(self.output.read_text(encoding="utf-8"))
        self.assertEqual(reviewed["resources"][0]["history"][0], hard)

    def test_apply_rejects_candidate_changed_after_prepare(self):
        self.write_candidate()
        self.prepare()
        changed = json.loads(self.candidate.read_text(encoding="utf-8"))
        changed["resources"][0]["task_id"] = 999
        self.write_candidate(changed)
        self.approve(list(review.POLICIES))
        with self.assertRaisesRegex(ValueError, "ledger does not match"):
            self.apply()

    def test_apply_rejects_tampered_cohort_ledger(self):
        self.write_candidate()
        self.prepare()
        ledger = json.loads(self.ledger.read_text(encoding="utf-8"))
        ledger["cohort_digest"] = "0" * 64
        self.ledger.write_text(review.canonical_json(ledger) + "\n", encoding="utf-8")
        self.approve(list(review.POLICIES))
        with self.assertRaisesRegex(ValueError, "ledger does not match"):
            self.apply()

    def test_apply_rejects_invalid_human_confirmation_fields(self):
        invalid = [
            ({"reviewer_id": 0}, "reviewer_id"),
            ({"reviewed_at": "2026-07-22T10:00:00"}, "timezone"),
            ({"note": "   "}, "note"),
        ]
        for index, (override, message) in enumerate(invalid):
            with self.subTest(index=index):
                self.write_candidate()
                self.prepare()
                self.approve(list(review.POLICIES), **override)
                with self.assertRaisesRegex(ValueError, message):
                    self.apply()

    def test_apply_requires_explicit_known_unique_policy_list(self):
        for policies, message in [
            (["not-a-policy"], "unknown"),
            (["explicit_event_replay", "explicit_event_replay"], "duplicates"),
        ]:
            with self.subTest(policies=policies):
                self.write_candidate()
                self.prepare()
                self.approve(policies)
                with self.assertRaisesRegex(ValueError, message):
                    self.apply()
        decision = json.loads(self.template.read_text(encoding="utf-8"))
        decision.update(
            {
                "decision": "approve",
                "reviewer_id": 88,
                "reviewed_at": "2026-07-22T10:00:00Z",
                "note": "reviewed",
            }
        )
        decision.pop("approved_policies")
        self.decision.write_text(json.dumps(decision), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "explicitly"):
            self.apply()

    def test_task_state_decision_is_policy_bound_and_promoted(self):
        self.write_candidate(mapping(task_state_decisions=[task_state()]))
        self.prepare()
        self.approve(["legacy_retouch_premature_terminal_partial_v1"])
        self.apply()
        reviewed = json.loads(self.output.read_text(encoding="utf-8"))
        decision = reviewed["task_state_decisions"][0]
        self.assertEqual(decision["confidence"], "confirmed_auto")
        self.assertEqual(decision["confirmed_by"], 88)
        self.assertEqual(
            decision["manifest_row_hash"],
            review.canonical_mapping_row_hash(decision),
        )
        evidence = json.loads(self.apply_evidence.read_text(encoding="utf-8"))
        self.assertEqual(evidence["promoted_task_state_count"], 1)

    def test_historical_unavailable_asset_recovery_is_policy_bound_and_promoted(self):
        candidate = asset_recovery()
        self.write_candidate(mapping(resources=[], asset_recoveries=[candidate]))
        self.prepare()
        ledger = json.loads(self.ledger.read_text(encoding="utf-8"))
        self.assertEqual(ledger["summary"]["asset_recovery"]["row_count"], 1)
        self.assertEqual(
            ledger["summary"]["asset_recovery"]["eligible_count"], 1
        )
        self.assertEqual(
            ledger["summary"]["asset_recovery"]["eligible_policy_counts"][
                "legacy_historical_asset_unavailable_v1"
            ],
            1,
        )
        self.assertEqual(ledger["rows"][0]["row_type"], "asset_recovery")

        self.approve(["legacy_historical_asset_unavailable_v1"])
        self.apply()
        reviewed = json.loads(self.output.read_text(encoding="utf-8"))
        confirmed = reviewed["asset_recoveries"][0]
        self.assertEqual(confirmed["confidence"], "confirmed_auto")
        self.assertEqual(confirmed["confirmed_by"], 88)
        self.assertEqual(
            confirmed["manifest_row_hash"],
            review.canonical_mapping_row_hash(confirmed),
        )
        evidence = json.loads(self.apply_evidence.read_text(encoding="utf-8"))
        self.assertEqual(evidence["promoted_asset_recovery_count"], 1)

    def test_clone_b_asset_recovery_is_eligible_after_controlled_read(self):
        candidate = asset_recovery(missing_task_asset_id=23989)
        self.write_candidate(mapping(resources=[], asset_recoveries=[candidate]))
        self.prepare()
        ledger = json.loads(self.ledger.read_text(encoding="utf-8"))
        self.assertEqual(
            ledger["summary"]["asset_recovery"]["eligible_count"], 1
        )
        self.assertEqual(
            ledger["summary"]["asset_recovery"]["excluded_count"], 0
        )
        self.approve(["legacy_deleted_asset_recovery_v1"])
        self.apply()
        reviewed = json.loads(self.output.read_text(encoding="utf-8"))
        self.assertEqual(
            reviewed["asset_recoveries"][0]["confidence"], "confirmed_auto"
        )

    def test_task2533_visual_scope_policy_is_exact_and_promotable(self):
        resources = []
        for scope_id, (source_id, final_id, reference_ids) in (
            review.RETOUCH_VISUAL_TASK2533.items()
        ):
            value = revision(
                stage="retouch",
                alias=None,
                policies=[
                    "explicit_event_replay",
                    "legacy_retouch_visual_scope_task2533_v1",
                ],
                reason=(
                    "policy legacy_retouch_visual_scope_task2533_v1: "
                    "exact visually reviewed membership"
                ),
            )
            value["source_task_asset_id"] = source_id
            value["final_task_asset_ids"] = [final_id]
            value["reference_file_ref_ids"] = reference_ids
            value["manifest_row_hash"] = review.canonical_revision_hash(value)
            resources.append({
                "task_id": 2533,
                "scope_kind": "retouch_requirement",
                "scope_ref_id": scope_id,
                "history": [value],
                "working_revision_no": 1,
                "finalized_revision_no": 1,
            })
        self.write_candidate(mapping(resources=resources))
        self.prepare()
        self.approve(
            [
                "explicit_event_replay",
                "legacy_retouch_visual_scope_task2533_v1",
            ]
        )
        self.apply()
        reviewed = json.loads(self.output.read_text(encoding="utf-8"))
        self.assertTrue(
            all(
                resource["history"][0]["confidence"] == "confirmed_auto"
                for resource in reviewed["resources"]
            )
        )

        resources[0]["history"][0]["final_task_asset_ids"] = [19803]
        resources[0]["history"][0]["manifest_row_hash"] = (
            review.canonical_revision_hash(resources[0]["history"][0])
        )
        self.write_candidate(mapping(resources=resources))
        with self.assertRaisesRegex(ValueError, "exact task 2533"):
            self.prepare()

    def test_incomplete_customization_terminal_state_is_exactly_allowlisted(self):
        candidate = task_state(task_id=452)
        candidate.update(
            {
                "from_status": "PendingWarehouseReceive",
                "target_status": "InProgress",
                "evidence_event_ids": [
                    "task_event_log:customization-approved"
                ],
                "review_policy_ids": [
                    "legacy_customization_terminal_without_assets_to_inprogress_v1"
                ],
            }
        )
        candidate["manifest_row_hash"] = review.canonical_mapping_row_hash(
            candidate
        )
        self.write_candidate(
            mapping(resources=[], task_state_decisions=[candidate])
        )
        self.prepare()
        ledger = json.loads(self.ledger.read_text(encoding="utf-8"))
        self.assertEqual(
            ledger["rows"][0]["required_policies"],
            [
                "legacy_customization_terminal_without_assets_to_inprogress_v1"
            ],
        )
        candidate["task_id"] = 453
        candidate["manifest_row_hash"] = review.canonical_mapping_row_hash(
            candidate
        )
        self.write_candidate(
            mapping(resources=[], task_state_decisions=[candidate])
        )
        with self.assertRaisesRegex(ValueError, "unsupported policy-bound"):
            self.prepare()

    def test_invalid_candidate_manifest_hash_fails_closed(self):
        candidate = mapping()
        candidate["resources"][0]["history"][0]["manifest_row_hash"] = "0" * 64
        self.write_candidate(candidate)
        with self.assertRaisesRegex(ValueError, "does not match"):
            self.prepare()

    def test_unknown_candidate_policy_fails_closed(self):
        candidate = mapping()
        revision_row = candidate["resources"][0]["history"][0]
        revision_row["review_policy_ids"].append("unknown_policy_v1")
        revision_row["manifest_row_hash"] = review.canonical_revision_hash(revision_row)
        self.write_candidate(candidate)
        with self.assertRaisesRegex(ValueError, "unknown"):
            self.prepare()

    def test_output_paths_cannot_overwrite_candidate(self):
        self.write_candidate()
        with self.assertRaisesRegex(ValueError, "must not overwrite"):
            review.prepare(self.candidate, self.candidate, self.template, None)


if __name__ == "__main__":
    unittest.main()
