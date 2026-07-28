from __future__ import annotations

import copy
import hashlib
import json
import tempfile
import unittest
from pathlib import Path

import select_computer_use_samples as selector
import validate_computer_use_evidence as evidence_validator


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
        group(
            20,
            "revision-id-padding",
            [
                revision(
                    f"revision-id-padding-{revision_no}",
                    revision_no=revision_no,
                )
                for revision_no in range(1, 614)
            ],
        ),
        group(
            1264,
            "retouch-reopen-task1264",
            [
                revision(
                    "retouch-reopen-task1264-1",
                    status="superseded",
                    stage="retouch",
                    policies=(
                        "explicit_event_replay",
                        "retouch_source_optional",
                        "legacy_retouch_terminal_submit_v1",
                    ),
                    source_alias_from_task_asset_id=None,
                    final_task_asset_ids=[5501],
                    reference_file_ref_ids=[1312],
                ),
                revision(
                    "retouch-reopen-task1264-2",
                    revision_no=2,
                    status="finalized",
                    stage="reopen",
                    policies=(
                        "explicit_event_replay",
                        "reopen",
                        "retouch_source_optional",
                        "legacy_retouch_terminal_submit_v1",
                    ),
                    source_alias_from_task_asset_id=None,
                    final_task_asset_ids=[6316],
                    reference_file_ref_ids=[1312],
                ),
            ],
            scope_kind="retouch_requirement",
            scope_ref_id=45,
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
        1264: "retouch_task",
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
    for task_id, requirement_ids in {
        14: [1401],
        15: [1501, 1502],
        1264: [45],
    }.items():
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

    def write_edge_receipt(self, mutator=None) -> Path:
        receipt = {
            "schema_version": 1,
            "gate": "G7",
            "status": "PASS",
            "edges": {
                combination: {
                    "origin": origin,
                    "edge": combination,
                    "frontend_sha256": digest(f"frontend:{combination}"),
                    "backend_sha256": digest(f"backend:{combination}"),
                    "fixture_identity": f"clone-b-{combination}",
                }
                for combination, origin in selector.EXPECTED_EDGE_ORIGINS.items()
            },
        }
        if mutator:
            mutator(receipt)
        receipt["receipt_sha256"] = selector.canonical_sha256(receipt)
        path = self.root / f"edge-receipt-{digest(json.dumps(receipt))[:8]}.json"
        path.write_text(
            json.dumps(receipt, ensure_ascii=False, sort_keys=True),
            encoding="utf-8",
        )
        return path

    def write_api_oracle(
        self,
        mapping_path: Path,
        canonical_path: Path | None,
        edge_receipt_path: Path,
        fixture_receipt_path: Path,
        mutator=None,
    ) -> Path:
        catalog = json.loads(self.catalog.read_text(encoding="utf-8"))
        oracle = {
            "schema_version": 1,
            "gate": "G7",
            "status": "PASS",
            "source_kind": "reviewed_api_allowed_actions",
            "reviewed_by": 1,
            "reviewed_at": "2026-07-23T12:00:00Z",
            "review_note": "frozen API allowed-actions oracle",
            "input_sha256": {
                "scenario_catalog_sha256": selector.file_sha256(self.catalog),
                "mapping_sha256": selector.file_sha256(mapping_path),
                "canonical_entities_sha256": (
                    selector.file_sha256(canonical_path) if canonical_path else None
                ),
                "edge_receipt_sha256": selector.file_sha256(edge_receipt_path),
                "fixture_receipt_sha256": selector.file_sha256(
                    fixture_receipt_path
                ),
            },
            "cases": [
                {
                    "scenario_id": scenario["id"],
                    "combination": combination,
                    "allowed_actions": [
                        {
                            "checkpoint": "task_detail",
                            "expected": ["download", "preview"],
                        }
                    ],
                    "http_probes": [
                        {
                            "kind": f"expected_{status}",
                            "method": "GET",
                            "path": (
                                f"/oracle/{scenario['id']}/{combination}/{status}"
                            ),
                            "expected_status": status,
                        }
                        for status in selector.requirements_for(
                            scenario,
                            combination,
                        )["required_http_statuses"]
                    ],
                    "resource_oracle": (
                        {
                            "kind": (
                                "v8_missing_resource_group"
                                if scenario["id"] == "missing_resource_group_negative"
                                else "v8_resource_groups"
                            )
                        }
                        if combination == "devplus_devplus"
                        else {
                            "kind": {
                                "external_external": "legacy_task_snapshot",
                                "external_devplus": "legacy_frontend_task_snapshot",
                                "devplus_external": "frontend_rollback_compatibility",
                            }[combination],
                            "task_response_sha256": digest(
                                f"task-response:{scenario['id']}:{combination}"
                            ),
                            **(
                                {
                                    "approved_assertion": (
                                        "approved_compatibility_difference_only"
                                    )
                                }
                                if combination == "devplus_external"
                                else {}
                            ),
                        }
                    ),
                }
                for scenario in catalog["scenarios"]
                for combination in scenario["required_combinations"]
            ],
        }
        if mutator:
            mutator(oracle)
        oracle["manifest_sha256"] = selector.canonical_sha256(oracle)
        path = self.root / f"api-oracle-{digest(json.dumps(oracle))[:8]}.json"
        path.write_text(
            json.dumps(oracle, ensure_ascii=False, sort_keys=True),
            encoding="utf-8",
        )
        return path

    def write_fixture_receipt(
        self,
        mapping_path: Path,
        canonical_path: Path,
    ) -> Path:
        mapping = json.loads(mapping_path.read_text(encoding="utf-8"))
        mapping_hash = selector.file_sha256(mapping_path)
        canonical_hash = selector.file_sha256(canonical_path)
        canonical_document = canonical_fixture(mapping, mapping_hash)
        canonical = selector.index_canonical(
            canonical_document,
            mapping_hash,
            False,
        )
        facts = selector.Facts(mapping, canonical, canonical_hash)
        catalog = json.loads(self.catalog.read_text(encoding="utf-8"))
        plans: dict[str, dict] = {}
        created: dict[str, dict] = {}
        next_task = 9000
        next_group = 9100
        next_revision = 9200
        for scenario in catalog["scenarios"]:
            resources, _, _, fixture = selector.choose_scenario(
                scenario["id"],
                facts,
            )
            if not fixture:
                continue
            plans[scenario["id"]] = fixture
            template_task_id = int(resources[0]["task_id"])
            kind = fixture["fixture_kind"]
            if kind == "permission_denied_identity":
                created[scenario["id"]] = {
                    "template_task_id": template_task_id,
                    "template_task_asset_id": 9301,
                    "fixture_user_id": 9401,
                    "session_id": "fixture-session",
                }
                continue
            next_task += 1
            row: dict[str, object] = {
                "task_id": next_task,
                "group_ids": [],
                "revision_ids": [],
            }
            if kind in {"missing_current_pointer", "wrong_scope_asset"}:
                next_group += 1
                next_revision += 1
                row["group_ids"] = [next_group]
                row["revision_ids"] = [next_revision]
                if kind == "wrong_scope_asset":
                    next_group += 1
                    row["group_ids"].append(next_group)
            created[scenario["id"]] = row
        archived_entity = next(
            row
            for row in canonical_document["entities"]
            if row["gate_name"] == "G01" and row["entity_key"] == "task:18"
        )
        archived_entity["components"][2] = "Completed"
        no_archived = selector.index_canonical(
            canonical_document,
            mapping_hash,
            False,
        )
        archived_facts = selector.Facts(mapping, no_archived, canonical_hash)
        archived_scenario = next(
            row
            for row in catalog["scenarios"]
            if row["id"] == "archived_readonly"
        )
        archived_resources, _, _, archived_fixture = selector.choose_scenario(
            archived_scenario["id"],
            archived_facts,
        )
        if archived_fixture:
            plans["archived_readonly"] = archived_fixture
            next_task += 1
            created["archived_readonly"] = {
                "task_id": next_task,
                "group_ids": [],
                "revision_ids": [],
                "template_task_id": int(archived_resources[0]["task_id"]),
            }
        receipt: dict[str, object] = {
            "schema_version": 2,
            "gate": "G7",
            "status": "APPLIED_VERIFIED_PENDING_UI_AND_CLEANUP",
            "production_write_performed": False,
            "clone_a_write_performed": False,
            "template_task_mutated": False,
            "input_sha256": {
                "scenarios": selector.file_sha256(self.catalog),
                "mapping": mapping_hash,
                "canonical": canonical_hash,
            },
            "scenario_ids": sorted(created),
            "created_rows": {"scenarios": created},
            "fixture_plans": plans,
            "nonfixture_integrity": {"status": "PASS"},
            "template_integrity": {"status": "PASS"},
            "row_verification": {"status": "PASS"},
            "api_verification": {"status": "PASS"},
        }
        receipt["receipt_payload_sha256"] = selector.canonical_sha256(receipt)
        path = self.root / "fixture-receipt.json"
        path.write_text(
            json.dumps(receipt, ensure_ascii=False, sort_keys=True),
            encoding="utf-8",
        )
        return path

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
        edge_receipt_path: Path | None = None,
        fixture_receipt_path: Path | None = None,
        api_oracle_path: Path | None = None,
    ):
        output = self.root / output_name
        edge_receipt_path = edge_receipt_path or self.write_edge_receipt()
        fixture_receipt_path = fixture_receipt_path or self.write_fixture_receipt(
            mapping_path,
            canonical_path,
        )
        api_oracle_path = api_oracle_path or self.write_api_oracle(
            mapping_path,
            canonical_path,
            edge_receipt_path,
            fixture_receipt_path,
        )
        argv = [
            "--scenarios",
            str(self.catalog),
            "--mapping",
            str(mapping_path),
            "--mode",
            mode,
            "--edge-receipt",
            str(edge_receipt_path),
            "--fixture-receipt",
            str(fixture_receipt_path),
            "--api-oracle",
            str(api_oracle_path),
            "--output",
            str(output),
        ]
        if canonical_path:
            argv += ["--canonical-entities", str(canonical_path)]
        result = selector.main(argv)
        document = json.loads(output.read_text(encoding="utf-8")) if output.exists() else None
        return result, document, output

    def test_final_selects_all_30_with_full_matrix_and_hashes(self):
        mapping_path, canonical_path = self.write_inputs()
        code, document, _ = self.run_selector(mapping_path, canonical_path, "final")
        self.assertEqual(code, 0)
        self.assertEqual(document["status"], "PASS")
        self.assertEqual(document["sample_count"], 30)
        self.assertEqual(document["blocker_count"], 0)
        self.assertEqual(
            set(document["coverage"]["combinations"]),
            selector.EXPECTED_COMBINATIONS,
        )
        self.assertEqual(set(document["coverage"]["viewports"]), {"desktop", "mobile"})
        self.assertRegex(document["manifest_sha256"], r"^[0-9a-f]{64}$")
        self.assertEqual(
            set(document["sealed_edges"]),
            selector.EXPECTED_COMBINATIONS,
        )
        for sample in document["samples"]:
            self.assertGreater(sample["task_id"], 0)
            self.assertTrue(sample["coverage_matrix"])
            self.assertRegex(sample["sample_sha256"], r"^[0-9a-f]{64}$")
            if sample["resource_identity_kind"] == "canonical_task_scope_key":
                self.assertEqual(sample["resource_ids"], sample["resource_keys"])
            else:
                self.assertEqual(
                    sample["resource_identity_kind"],
                    "clone_b_runtime_group_id",
                )
                self.assertEqual(sample["resource_keys"], [])
            if sample["revision_facts"]:
                self.assertTrue(sample["revision_ids"])
            for case in sample["coverage_matrix"]:
                self.assertRegex(case["oracle_sha256"], r"^[0-9a-f]{64}$")
                self.assertEqual(
                    case["allowed_actions"],
                    [
                        {
                            "checkpoint": "task_detail",
                            "expected": ["download", "preview"],
                        }
                    ],
                )
                self.assertEqual(
                    [row["expected_status"] for row in case["http_probes"]],
                    case["requirements"]["required_http_statuses"],
                )
        validated_cases, validated_edges = (
            evidence_validator._validate_samples_manifest(
                document,
                evidence_validator._validate_catalog(
                    json.loads(self.catalog.read_text(encoding="utf-8"))
                ),
                selector.file_sha256(self.catalog),
            )
        )
        self.assertEqual(len(validated_cases), 66)
        self.assertEqual(set(validated_edges), selector.EXPECTED_COMBINATIONS)

    def test_edge_receipt_is_required_and_rejects_port_or_fingerprint_drift(self):
        mapping_path, canonical_path = self.write_inputs()
        edge_path = self.write_edge_receipt()
        fixture_path = self.write_fixture_receipt(mapping_path, canonical_path)
        api_oracle_path = self.write_api_oracle(
            mapping_path,
            canonical_path,
            edge_path,
            fixture_path,
        )
        with self.assertRaises(SystemExit):
            selector.parse_args(
                [
                    "--scenarios",
                    str(self.catalog),
                    "--mapping",
                    str(mapping_path),
                    "--canonical-entities",
                    str(canonical_path),
                    "--api-oracle",
                    str(api_oracle_path),
                    "--fixture-receipt",
                    str(fixture_path),
                    "--mode",
                    "final",
                    "--output",
                    str(self.root / "missing-edge.json"),
                ]
            )

        def wrong_port(receipt: dict) -> None:
            receipt["edges"]["external_external"]["origin"] = (
                "http://127.0.0.1:18104"
            )

        wrong_port_path = self.write_edge_receipt(wrong_port)
        code, document, _ = self.run_selector(
            mapping_path,
            canonical_path,
            "final",
            "wrong-port.json",
            edge_receipt_path=wrong_port_path,
            fixture_receipt_path=fixture_path,
            api_oracle_path=api_oracle_path,
        )
        self.assertEqual(code, 1)
        self.assertIsNone(document)

        def missing_fingerprint(receipt: dict) -> None:
            receipt["edges"]["devplus_devplus"]["backend_sha256"] = ""

        missing_fingerprint_path = self.write_edge_receipt(missing_fingerprint)
        code, document, _ = self.run_selector(
            mapping_path,
            canonical_path,
            "final",
            "missing-fingerprint.json",
            edge_receipt_path=missing_fingerprint_path,
            fixture_receipt_path=fixture_path,
            api_oracle_path=api_oracle_path,
        )
        self.assertEqual(code, 1)
        self.assertIsNone(document)

    def test_case_oracle_is_deterministic_and_changes_with_reviewed_actions(self):
        mapping_path, canonical_path = self.write_inputs()
        edge_path = self.write_edge_receipt()
        fixture_path = self.write_fixture_receipt(mapping_path, canonical_path)
        first_oracle = self.write_api_oracle(
            mapping_path,
            canonical_path,
            edge_path,
            fixture_path,
        )
        first_code, first, _ = self.run_selector(
            mapping_path,
            canonical_path,
            "final",
            "oracle-first.json",
            edge_receipt_path=edge_path,
            fixture_receipt_path=fixture_path,
            api_oracle_path=first_oracle,
        )

        def change_actions(oracle: dict) -> None:
            oracle["cases"][0]["allowed_actions"][0]["expected"] = ["preview"]

        second_oracle = self.write_api_oracle(
            mapping_path,
            canonical_path,
            edge_path,
            fixture_path,
            change_actions,
        )
        second_code, second, _ = self.run_selector(
            mapping_path,
            canonical_path,
            "final",
            "oracle-second.json",
            edge_receipt_path=edge_path,
            fixture_receipt_path=fixture_path,
            api_oracle_path=second_oracle,
        )
        self.assertEqual((first_code, second_code), (0, 0))
        first_case = first["samples"][0]["coverage_matrix"][0]
        second_case = second["samples"][0]["coverage_matrix"][0]
        self.assertNotEqual(first_case["oracle_sha256"], second_case["oracle_sha256"])
        self.assertNotEqual(first["manifest_sha256"], second["manifest_sha256"])

    def test_api_oracle_rejects_missing_or_unsafe_http_probe(self):
        mapping_path, canonical_path = self.write_inputs()
        edge_path = self.write_edge_receipt()
        fixture_path = self.write_fixture_receipt(mapping_path, canonical_path)

        def remove_probe(oracle: dict) -> None:
            row = next(case for case in oracle["cases"] if case["http_probes"])
            row["http_probes"] = []

        missing_probe = self.write_api_oracle(
            mapping_path,
            canonical_path,
            edge_path,
            fixture_path,
            remove_probe,
        )
        code, document, _ = self.run_selector(
            mapping_path,
            canonical_path,
            "final",
            "missing-probe.json",
            edge_receipt_path=edge_path,
            fixture_receipt_path=fixture_path,
            api_oracle_path=missing_probe,
        )
        self.assertEqual(code, 1)
        self.assertIsNone(document)

        def unsafe_probe(oracle: dict) -> None:
            row = next(case for case in oracle["cases"] if case["http_probes"])
            row["http_probes"][0]["path"] = "//evil.example/asset"

        unsafe = self.write_api_oracle(
            mapping_path,
            canonical_path,
            edge_path,
            fixture_path,
            unsafe_probe,
        )
        code, document, _ = self.run_selector(
            mapping_path,
            canonical_path,
            "final",
            "unsafe-probe.json",
            edge_receipt_path=edge_path,
            fixture_receipt_path=fixture_path,
            api_oracle_path=unsafe,
        )
        self.assertEqual(code, 1)
        self.assertIsNone(document)

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
                expected_assertions = {
                    "page_matches_manifest",
                    "assets_match",
                    "allowed_actions_exact",
                }
                if case["combination"] == "devplus_external":
                    expected_assertions.add(
                        "approved_compatibility_difference_only"
                    )
                self.assertEqual(
                    expected_assertions,
                    set(requirements["required_assertions"]),
                )
                if case["combination"] == "devplus_devplus":
                    self.assertTrue(requirements["requires_revision_ids"])
                    self.assertTrue(requirements["requires_history_drawer"])
                    self.assertEqual(baseline["revision_ids"], case["revision_ids"])
                    self.assertEqual(
                        {"kind": "v8_resource_groups"},
                        case["resource_oracle"],
                    )
                    self.assertEqual(baseline["resource_ids"], case["resource_ids"])
                else:
                    self.assertFalse(requirements["requires_revision_ids"])
                    self.assertFalse(requirements["requires_history_drawer"])
                    self.assertEqual([], case["revision_ids"])
                    self.assertEqual([], case["resource_ids"])
                    self.assertRegex(
                        case["resource_oracle"]["task_response_sha256"],
                        r"^[0-9a-f]{64}$",
                    )
                    if case["combination"] == "devplus_external":
                        self.assertEqual(
                            "approved_compatibility_difference_only",
                            case["resource_oracle"]["approved_assertion"],
                        )

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
        retouch_reopen = samples["retouch_reopen_task1264"]
        self.assertEqual(retouch_reopen["task_id"], 1264)
        self.assertEqual(
            retouch_reopen["resource_ids"],
            ["group:1264:retouch_requirement:45"],
        )
        self.assertEqual(retouch_reopen["revision_ids"], [635, 636])
        self.assertEqual(
            [
                (
                    row["predicted_revision_id"],
                    row["status"],
                    row["source_stage"],
                )
                for row in retouch_reopen["revision_facts"]
            ],
            [
                (635, "superseded", "retouch"),
                (636, "finalized", "reopen"),
            ],
        )
        self.assertTrue(
            document["revision_id_precondition"]["runtime_receipt_must_reconfirm_ids"]
        )


if __name__ == "__main__":
    unittest.main()
