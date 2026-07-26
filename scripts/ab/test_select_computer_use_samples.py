from __future__ import annotations

import copy
import hashlib
import json
import tempfile
import unittest
from pathlib import Path

import select_computer_use_samples as selector


def digest(label: str) -> str:
    return hashlib.sha256(label.encode("utf-8")).hexdigest()


def confirmed(label: str) -> dict:
    return {
        "confidence": "confirmed_auto",
        "confirmed_by": 1,
        "confirmed_at": "2026-07-23T12:00:00Z",
        "confirmation_note": f"confirmed {label}",
        "manifest_row_hash": digest(label),
    }


def revision(
    label: str,
    revision_no: int = 1,
    status: str = "finalized",
    stage: str = "design",
    policies: tuple[str, ...] = ("explicit_event_replay", "delivery_source_alias"),
    **extra,
) -> dict:
    value = {
        "revision_no": revision_no,
        "status": status,
        "mode": "single",
        "source_stage": stage,
        "source_alias_from_task_asset_id": int(f"{revision_no}01"),
        "final_task_asset_ids": [int(f"{revision_no}02")],
        "reference_file_ref_ids": [],
        "evidence_event_ids": [f"event:{label}:submit", f"event:{label}:audit"],
        "review_policy_ids": list(policies),
        "created_by": 1,
        **confirmed(label),
    }
    value.update(extra)
    return value


def group(
    task_id: int,
    label: str,
    history: list[dict] | None = None,
    scope_kind: str = "task",
    scope_ref_id: int = 0,
) -> dict:
    history = history if history is not None else [revision(label)]
    value = {
        "task_id": task_id,
        "scope_kind": scope_kind,
        "scope_ref_id": scope_ref_id,
        "history": history,
    }
    if history:
        working = max(int(row["revision_no"]) for row in history)
        finalized = max(
            (
                int(row["revision_no"])
                for row in history
                if row["status"] in {"finalized", "superseded"}
            ),
            default=None,
        )
        value["working_revision_no"] = working
        if finalized is not None:
            value["finalized_revision_no"] = finalized
    return value


def mapping_fixture() -> dict:
    resources = [
        group(1, "design-first"),
        group(
            2,
            "audit-replace",
            [
                revision(
                    "audit-replace",
                    stage="audit",
                    policies=(
                        "explicit_event_replay",
                        "legacy_audit_stage_final_snapshot_v1",
                    ),
                )
            ],
        ),
        group(
            3,
            "rejected",
            [
                revision(
                    "rejected-1",
                    status="rejected",
                    policies=("explicit_event_replay", "rejected_history"),
                ),
                revision(
                    "rejected-2",
                    revision_no=2,
                    policies=("explicit_event_replay", "rejected_history"),
                ),
            ],
        ),
        group(4, "customization"),
        group(5, "warehouse-receive"),
        group(6, "production-transfer"),
        group(7, "pending-close"),
        group(
            8,
            "warehouse-reject",
            [
                revision("warehouse-reject-1"),
                revision(
                    "warehouse-reject-2",
                    revision_no=2,
                    stage="reopen",
                    policies=("explicit_event_replay", "delivery_source_alias", "reopen"),
                ),
            ],
        ),
        group(
            9,
            "post-close",
            [
                revision("post-close-1"),
                revision(
                    "post-close-2",
                    revision_no=2,
                    stage="reopen",
                    policies=(
                        "explicit_event_replay",
                        "delivery_source_alias",
                        "reopen",
                        "legacy_post_close_replacement_v1",
                    ),
                ),
            ],
        ),
        group(
            10,
            "supplement",
            [
                revision(
                    "supplement",
                    stage="audit",
                    policies=("explicit_event_replay", "legacy_audit_stage_final_snapshot_v1"),
                )
            ],
        ),
        group(
            11,
            "bundle",
            [
                revision(
                    "bundle",
                    source_alias_from_task_asset_id=None,
                    source_bundle={
                        "task_asset_id": 50001,
                        "format": "zip",
                        "bundle_sha256": digest("bundle-bytes"),
                        "manifest_sha256": digest("bundle-manifest"),
                        "members": [
                            {"task_asset_id": 501, "sha256": digest("member-501"), "confirmed": True},
                            {"task_asset_id": 502, "sha256": digest("member-502"), "confirmed": True},
                        ],
                        "confirmed_by": 1,
                        "confirmed_at": "2026-07-23T12:00:00Z",
                        "confirmation_note": "confirmed bundle",
                    },
                )
            ],
        ),
        group(12, "single-sku", scope_kind="sku", scope_ref_id=1201),
        group(
            13,
            "multi-sku-a",
            [
                revision(
                    "multi-sku-a",
                    policies=(
                        "explicit_event_replay",
                        "delivery_source_alias",
                        "legacy_multi_sku_atomic_batch_submit_v1",
                    ),
                    evidence_event_ids=["event:multi-sku:atomic-submit"],
                )
            ],
            scope_kind="sku",
            scope_ref_id=1301,
        ),
        group(
            13,
            "multi-sku-b",
            [
                revision(
                    "multi-sku-b",
                    policies=(
                        "explicit_event_replay",
                        "delivery_source_alias",
                        "legacy_multi_sku_atomic_batch_submit_v1",
                    ),
                    evidence_event_ids=["event:multi-sku:atomic-submit"],
                )
            ],
            scope_kind="sku",
            scope_ref_id=1302,
        ),
        group(
            14,
            "retouch-single",
            [
                revision(
                    "retouch-single",
                    stage="retouch",
                    policies=(
                        "explicit_event_replay",
                        "retouch_source_optional",
                        "legacy_retouch_terminal_submit_v1",
                    ),
                    source_alias_from_task_asset_id=None,
                )
            ],
            scope_kind="retouch_requirement",
            scope_ref_id=1401,
        ),
        group(
            15,
            "retouch-multi-a",
            [
                revision(
                    "retouch-multi-a",
                    stage="retouch",
                    policies=("explicit_event_replay", "retouch_source_optional"),
                    source_alias_from_task_asset_id=None,
                )
            ],
            scope_kind="retouch_requirement",
            scope_ref_id=1501,
        ),
        group(
            15,
            "retouch-multi-b",
            [
                revision(
                    "retouch-multi-b",
                    stage="retouch",
                    policies=("explicit_event_replay", "retouch_source_optional"),
                    source_alias_from_task_asset_id=None,
                )
            ],
            scope_kind="retouch_requirement",
            scope_ref_id=1502,
        ),
        group(16, "planning-empty", [], scope_kind="task", scope_ref_id=0),
        group(17, "cancelled", [], scope_kind="task", scope_ref_id=0),
        group(18, "archived", [], scope_kind="task", scope_ref_id=0),
        group(
            19,
            "tombstone",
            [
                revision(
                    "tombstone",
                    source_alias_from_task_asset_id=1901,
                    final_task_asset_ids=[1901],
                )
            ],
        ),
    ]
    return {
        "version": 2,
        "resources": resources,
        "planning_tasks": [
            {
                "task_id": 16,
                "target_task_status": "Completed",
                "review_policy_ids": ["legacy_purchase_to_sku_planning_v1"],
                "items": [{"task_sku_item_id": 1601, "description_spec": "SKU"}],
                **confirmed("planning-16"),
            }
        ],
        "task_state_decisions": [],
        "asset_recoveries": [
            {
                "task_id": 19,
                "missing_task_asset_id": 1901,
                "strategy": "historical_unavailable_tombstone_v1",
                "review_policy_ids": ["legacy_historical_asset_unavailable_v1"],
                **confirmed("tombstone-recovery"),
            }
        ],
        "organization_mappings": [],
        "access_decisions": [
            {
                "user_id": 200,
                "action": "no_new_grant",
                "review_policy_ids": ["retired_warehouse_no_new_grant_v1"],
                **confirmed("no-new-grant"),
            }
        ],
    }


def canonical_entity(gate: str, key: str, components: list) -> dict:
    return {
        "gate_name": gate,
        "entity_key": key,
        "expected_state": "matched",
        "review_state": "pass",
        "derivation_method": "test",
        "components": ["" if value is None else str(value) for value in components],
        "detail": {},
    }


def canonical_fixture(mapping: dict, mapping_hash: str) -> dict:
    entities: list[dict] = []
    task_types = {
        4: "customization_task",
        14: "retouch_task",
        15: "retouch_task",
        16: "sku_planning",
    }
    statuses = {17: "Cancelled", 18: "Archived"}
    task_ids = sorted({int(row["task_id"]) for row in mapping["resources"]} | {16, 17, 18})
    for task_id in task_ids:
        entities.append(
            canonical_entity(
                "G01",
                f"task:{task_id}",
                [
                    task_id,
                    task_types.get(task_id, "design_task"),
                    statuses.get(task_id, "Completed"),
                    1,
                    1,
                ],
            )
        )
    for resource in mapping["resources"]:
        key = selector.group_locator(resource)
        history = resource["history"]
        entities.append(
            canonical_entity(
                "G02",
                key,
                [
                    resource["task_id"],
                    resource["scope_kind"],
                    resource["scope_ref_id"],
                    resource.get("working_revision_no", ""),
                    history[-1]["status"] if history else "",
                    resource.get("finalized_revision_no", ""),
                    history[-1]["status"] if history else "",
                    0,
                    "",
                ],
            )
        )
        for row in history:
            entities.append(
                canonical_entity(
                    "G03",
                    selector.revision_locator(resource, row),
                    [
                        resource["task_id"],
                        resource["scope_kind"],
                        resource["scope_ref_id"],
                        row["revision_no"],
                        row["status"],
                        row["mode"],
                        row.get("source_task_asset_id"),
                        row["source_stage"],
                    ],
                )
            )
    events = {
        4: "task.customization.reviewed",
        5: "PendingWarehouseReceive",
        6: "PendingProductionTransfer",
        7: "PendingClose",
        8: "RejectedByWarehouse",
        10: "audit_supplement_upload",
    }
    for task_id, event_type in events.items():
        entities.append(
            canonical_entity(
                "G07",
                f"task-event:{task_id}:1",
                [f"event-{task_id}", task_id, 1, event_type, 1, "{}", "2026-07-23"],
            )
        )
    for task_id, requirement_ids in {14: [1401], 15: [1501, 1502]}.items():
        for requirement_id in requirement_ids:
            entities.append(
                canonical_entity(
                    "G08",
                    f"retouch-requirement:{task_id}:{requirement_id}",
                    [task_id, requirement_id, "requirement", "", "", "", 0, 0],
                )
            )
    for gate in ("G04", "G05", "G06", "G09", "G10"):
        entity = canonical_entity(gate, f"{gate.lower()}-sentinel", [])
        entity["expected_state"] = {
            "G06": "verified",
            "G09": "approved",
            "G10": "confirmed",
        }.get(gate, "matched")
        entities.append(entity)
    return {
        "schema_version": 1,
        "input_sha256": {"mapping_sha256": mapping_hash},
        "entities": entities,
    }


class SelectComputerUseSamplesTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.catalog = (
            Path(__file__).resolve().parent / "computer_use_scenarios.json"
        )

    def tearDown(self) -> None:
        self.temp.cleanup()

    def test_canonical_positive_int_accepts_only_canonical_decimal_strings(self):
        self.assertEqual(selector.canonical_positive_int("1000"), 1000)
        for value in ("", "0", "01", "-1", "+1", " 1", 1, True, None):
            with self.subTest(value=value):
                self.assertIsNone(selector.canonical_positive_int(value))

    def write_inputs(self, mapping: dict | None = None, canonical_mutator=None):
        mapping = copy.deepcopy(mapping or mapping_fixture())
        mapping_path = self.root / "mapping.json"
        mapping_path.write_text(
            json.dumps(mapping, ensure_ascii=False, sort_keys=True),
            encoding="utf-8",
        )
        mapping_hash = selector.file_sha256(mapping_path)
        canonical = canonical_fixture(mapping, mapping_hash)
        if canonical_mutator:
            canonical_mutator(canonical)
        canonical_path = self.root / "canonical.json"
        canonical_path.write_text(
            json.dumps(canonical, ensure_ascii=False, sort_keys=True),
            encoding="utf-8",
        )
        return mapping_path, canonical_path

    def run_selector(
        self,
        mapping_path: Path,
        canonical_path: Path | None,
        mode: str,
        output_name: str = "samples.json",
    ):
        output = self.root / output_name
        argv = [
            "--scenarios",
            str(self.catalog),
            "--mapping",
            str(mapping_path),
            "--mode",
            mode,
            "--output",
            str(output),
        ]
        if canonical_path:
            argv += ["--canonical-entities", str(canonical_path)]
        result = selector.main(argv)
        document = json.loads(output.read_text(encoding="utf-8")) if output.exists() else None
        return result, document, output

    def test_final_selects_all_29_with_full_matrix_and_hashes(self):
        mapping_path, canonical_path = self.write_inputs()
        code, document, _ = self.run_selector(mapping_path, canonical_path, "final")
        self.assertEqual(code, 0)
        self.assertEqual(document["status"], "PASS")
        self.assertEqual(document["sample_count"], 29)
        self.assertEqual(document["blocker_count"], 0)
        self.assertEqual(
            set(document["coverage"]["combinations"]),
            selector.EXPECTED_COMBINATIONS,
        )
        self.assertEqual(set(document["coverage"]["viewports"]), {"desktop", "mobile"})
        self.assertRegex(document["manifest_sha256"], r"^[0-9a-f]{64}$")
        for sample in document["samples"]:
            self.assertGreater(sample["task_id"], 0)
            self.assertTrue(sample["coverage_matrix"])
            self.assertRegex(sample["sample_sha256"], r"^[0-9a-f]{64}$")
            self.assertEqual(sample["resource_ids"], sample["resource_keys"])
            self.assertEqual(
                sample["resource_identity_kind"], "canonical_task_scope_key"
            )
            if sample["revision_facts"]:
                self.assertTrue(sample["revision_ids"])

    def test_output_is_byte_for_byte_deterministic(self):
        mapping_path, canonical_path = self.write_inputs()
        first_code, first, first_path = self.run_selector(
            mapping_path, canonical_path, "final", "first.json"
        )
        second_code, second, second_path = self.run_selector(
            mapping_path, canonical_path, "final", "second.json"
        )
        self.assertEqual((first_code, second_code), (0, 0))
        self.assertEqual(first, second)
        self.assertEqual(first_path.read_bytes(), second_path.read_bytes())

    def test_negative_cases_are_clone_b_fixtures_not_production_damage(self):
        mapping_path, canonical_path = self.write_inputs()
        code, document, _ = self.run_selector(mapping_path, canonical_path, "final")
        self.assertEqual(code, 0)
        samples = {row["scenario_id"]: row for row in document["samples"]}
        for scenario_id in selector.NEGATIVE_FIXTURE_SCENARIOS:
            sample = samples[scenario_id]
            self.assertEqual(sample["target_kind"], "clone_b_fixture_derived")
            plan = sample["fixture_plan"]
            self.assertEqual(plan["fixture_class"], "negative_assertion")
            self.assertEqual(plan["environment"], "isolated_clone_b_only")
            self.assertIn("production_write", plan["forbidden"])
            self.assertIn("reuse_existing_malformed_production_row", plan["forbidden"])
            self.assertTrue(plan["fixture_receipt_required_before_browser_execution"])
        self.assertEqual(
            document["coverage"]["negative_fixture_scenarios"],
            sorted(selector.NEGATIVE_FIXTURE_SCENARIOS),
        )

    def test_baseline_manifest_scopes_revision_history_to_devplus_edge(self):
        mapping_path, canonical_path = self.write_inputs()
        code, document, _ = self.run_selector(mapping_path, canonical_path, "final")
        self.assertEqual(code, 0)
        baseline = next(
            row
            for row in document["samples"]
            if row["scenario_id"] == "baseline_four_edge_readonly"
        )
        self.assertTrue(baseline["revision_ids"])
        for case in baseline["coverage_matrix"]:
            with self.subTest(
                combination=case["combination"],
                viewport=case["viewport"],
            ):
                requirements = case["requirements"]
                self.assertTrue(requirements["requires_task_id"])
                self.assertEqual(
                    {
                        "page_matches_manifest",
                        "assets_match",
                        "allowed_actions_exact",
                    },
                    set(requirements["required_assertions"]),
                )
                if case["combination"] == "devplus_devplus":
                    self.assertTrue(requirements["requires_revision_ids"])
                    self.assertTrue(requirements["requires_history_drawer"])
                    self.assertEqual(baseline["revision_ids"], case["revision_ids"])
                else:
                    self.assertFalse(requirements["requires_revision_ids"])
                    self.assertFalse(requirements["requires_history_drawer"])
                    self.assertEqual([], case["revision_ids"])

    def test_selector_rejects_conditional_core_assertion_weakening(self):
        catalog = json.loads(self.catalog.read_text(encoding="utf-8"))
        baseline = catalog["scenarios"][0]
        baseline["requirements_by_combination"]["external_external"][
            "required_assertions"
        ].remove("assets_match")
        with self.assertRaisesRegex(selector.InputError, "cannot weaken assertions"):
            selector.validate_catalog(catalog)

    def test_missing_bundle_is_a_hard_final_blocker_without_fallback(self):
        mapping = mapping_fixture()
        bundle_group = next(row for row in mapping["resources"] if row["task_id"] == 11)
        bundle_group["history"][0].pop("source_bundle")
        bundle_group["history"][0]["source_alias_from_task_asset_id"] = 1101
        mapping_path, canonical_path = self.write_inputs(mapping)
        code, document, _ = self.run_selector(mapping_path, canonical_path, "final")
        self.assertEqual(code, 3)
        self.assertEqual(document["status"], "BLOCKED")
        blockers = {row["scenario_id"] for row in document["blockers"]}
        self.assertIn("multi_source_zip_bundle", blockers)
        selected = {row["scenario_id"] for row in document["samples"]}
        self.assertNotIn("multi_source_zip_bundle", selected)

    def test_malformed_bundle_contract_is_rejected_as_input(self):
        mapping = mapping_fixture()
        bundle_group = next(row for row in mapping["resources"] if row["task_id"] == 11)
        bundle_group["history"][0]["source_bundle"]["members"][0]["sha256"] = ""
        mapping_path, canonical_path = self.write_inputs(mapping)
        code, document, _ = self.run_selector(mapping_path, canonical_path, "final")
        self.assertEqual(code, 1)
        self.assertIsNone(document)

    def test_prepare_accepts_rebased_mapping_but_never_passes(self):
        mapping = mapping_fixture()
        mapping["resources"][0]["history"][0].update(
            {
                "confidence": "proposed_review",
                "confirmed_by": 0,
                "confirmed_at": "0001-01-01T00:00:00Z",
                "confirmation_note": "",
            }
        )
        mapping_path, canonical_path = self.write_inputs(mapping)
        code, document, _ = self.run_selector(mapping_path, canonical_path, "prepare")
        self.assertEqual(code, 2)
        self.assertEqual(document["status"], "PENDING")
        self.assertEqual(document["mapping_review"]["status"], "PENDING")
        self.assertGreater(document["mapping_review"]["unconfirmed_row_count"], 0)
        self.assertTrue(all(row["status"] == "PENDING" for row in document["samples"]))

    def test_final_rejects_unreviewed_mapping_before_selection(self):
        mapping = mapping_fixture()
        mapping["planning_tasks"][0]["confidence"] = "proposed_review"
        mapping_path, canonical_path = self.write_inputs(mapping)
        code, document, _ = self.run_selector(mapping_path, canonical_path, "final")
        self.assertEqual(code, 1)
        self.assertIsNone(document)

    def test_canonical_mapping_hash_mismatch_is_rejected(self):
        mapping_path, canonical_path = self.write_inputs()
        canonical = json.loads(canonical_path.read_text(encoding="utf-8"))
        canonical["input_sha256"]["mapping_sha256"] = digest("wrong")
        canonical_path.write_text(json.dumps(canonical), encoding="utf-8")
        code, document, _ = self.run_selector(mapping_path, canonical_path, "final")
        self.assertEqual(code, 1)
        self.assertIsNone(document)

    def test_final_rejects_approved_entity_without_pass_review(self):
        def make_g09_pending(canonical: dict) -> None:
            entity = next(
                row
                for row in canonical["entities"]
                if row["gate_name"] == "G09"
            )
            entity["expected_state"] = "approved"
            entity["review_state"] = "pending"

        mapping_path, canonical_path = self.write_inputs(
            canonical_mutator=make_g09_pending
        )
        code, document, _ = self.run_selector(mapping_path, canonical_path, "final")
        self.assertEqual(code, 1)
        self.assertIsNone(document)

    def test_final_rejects_wrong_gate_specific_expected_state(self):
        def make_g01_confirmed(canonical: dict) -> None:
            entity = next(
                row
                for row in canonical["entities"]
                if row["gate_name"] == "G01"
            )
            entity["expected_state"] = "confirmed"

        mapping_path, canonical_path = self.write_inputs(
            canonical_mutator=make_g01_confirmed
        )
        code, document, _ = self.run_selector(mapping_path, canonical_path, "final")
        self.assertEqual(code, 1)
        self.assertIsNone(document)

    def test_customization_event_from_another_task_cannot_select_group(self):
        def detach_customization_event(canonical: dict) -> None:
            event = next(
                row
                for row in canonical["entities"]
                if row["gate_name"] == "G07"
                and row["entity_key"] == "task-event:4:1"
            )
            event["components"][3] = "task.created"
            canonical["entities"].append(
                canonical_entity(
                    "G07",
                    "task-event:17:2",
                    [
                        "event-17-customization",
                        17,
                        2,
                        "task.customization.reviewed",
                        1,
                        "{}",
                        "2026-07-23",
                    ],
                )
            )

        mapping_path, canonical_path = self.write_inputs(
            canonical_mutator=detach_customization_event
        )
        code, document, _ = self.run_selector(mapping_path, canonical_path, "final")
        self.assertEqual(code, 3)
        self.assertIn(
            "customization_submit_audit",
            {row["scenario_id"] for row in document["blockers"]},
        )

    def test_zero_archived_population_uses_positive_contract_fixture(self):
        def remove_archived(canonical: dict) -> None:
            task = next(
                row
                for row in canonical["entities"]
                if row["gate_name"] == "G01" and row["entity_key"] == "task:18"
            )
            task["components"][2] = "Completed"

        mapping_path, canonical_path = self.write_inputs(
            canonical_mutator=remove_archived
        )
        code, document, _ = self.run_selector(mapping_path, canonical_path, "final")
        self.assertEqual(code, 0)
        sample = next(
            row
            for row in document["samples"]
            if row["scenario_id"] == "archived_readonly"
        )
        self.assertEqual(sample["target_kind"], "clone_b_fixture_derived")
        plan = sample["fixture_plan"]
        self.assertEqual(plan["fixture_kind"], "archived_terminal")
        self.assertEqual(plan["fixture_class"], "positive_contract")
        self.assertEqual(plan["canonical_archived_population"], 0)
        self.assertEqual(sample["resource_ids"], [])
        self.assertEqual(sample["revision_ids"], [])
        self.assertEqual(sample["revision_facts"], [])
        self.assertEqual(sample["task_facts"]["task_status"], "Archived")
        self.assertEqual(sample["task_facts"]["expected_asset_count"], 0)
        self.assertEqual(
            plan["expected_runtime"],
            {
                "task_status": "Archived",
                "current_handler_id": None,
                "resource_group_count": 0,
                "revision_count": 0,
                "asset_count": 0,
                "allowed_actions": [],
                "module_state": "closed",
            },
        )
        self.assertEqual(
            plan["canonical_entities_sha256"],
            selector.file_sha256(canonical_path),
        )
        self.assertEqual(
            plan["historical_migration_coverage"],
            "not_applicable_zero_frozen_population",
        )
        self.assertIn(
            "archived_readonly",
            document["coverage"]["positive_contract_fixture_scenarios"],
        )
        self.assertNotIn(
            "archived_readonly",
            document["coverage"]["negative_fixture_scenarios"],
        )

    def test_real_archived_candidate_never_uses_fixture(self):
        mapping_path, canonical_path = self.write_inputs()
        code, document, _ = self.run_selector(mapping_path, canonical_path, "final")
        self.assertEqual(code, 0)
        sample = next(
            row
            for row in document["samples"]
            if row["scenario_id"] == "archived_readonly"
        )
        self.assertEqual(sample["target_kind"], "reviewed_real_task")
        self.assertNotIn("fixture_plan", sample)

    def test_revision_ids_follow_mapping_apply_order(self):
        mapping_path, canonical_path = self.write_inputs()
        code, document, _ = self.run_selector(mapping_path, canonical_path, "final")
        self.assertEqual(code, 0)
        samples = {row["scenario_id"]: row for row in document["samples"]}
        self.assertEqual(samples["design_first_submit_audit"]["revision_ids"], [1])
        self.assertEqual(
            samples["audit_reject_redesign_resubmit"]["revision_ids"],
            [3, 4],
        )
        self.assertTrue(
            document["revision_id_precondition"]["runtime_receipt_must_reconfirm_ids"]
        )


if __name__ == "__main__":
    unittest.main()
