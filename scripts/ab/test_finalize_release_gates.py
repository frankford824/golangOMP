import hashlib
import importlib.util
import json
import pathlib
import sys
import tempfile
import unittest

from scripts.ab.test_clone_b_materialization_component import (
    write_component_chain_fixture,
)

PATH = pathlib.Path(__file__).with_name("finalize_release_gates.py")
SPEC = importlib.util.spec_from_file_location("finalize_release_gates", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


def write_json(path, value):
    path.write_bytes(MODULE.canonical_bytes(value))
    return MODULE.sha256_file(path)


def g7_pair_hash(combination, viewport):
    return hashlib.sha256(
        json.dumps(
            {
                "browser_record_sha256": "4" * 64,
                "case_key": [
                    "baseline_four_edge_readonly",
                    combination,
                    viewport,
                ],
                "playwright_record_sha256": "5" * 64,
                "scenario_catalog_sha256": "1" * 64,
            },
            ensure_ascii=False,
            sort_keys=True,
            separators=(",", ":"),
        ).encode("utf-8")
    ).hexdigest()


class FinalizeReleaseGatesTest(unittest.TestCase):
    def make_run(self, root, *, blocked_gate=None, signed=True):
        run_id = "formal-run-1"
        run_dir = pathlib.Path(root) / run_id
        run_dir.mkdir()
        gates = {}
        for gate in MODULE.GATES:
            payload = {
                "schema_version": 1,
                "run_id": run_id,
                "status": "PASS",
                "violation_count": 0,
            }
            if gate == "G0":
                payload.update(
                    {
                        "candidate": {
                            "git_head": "a" * 40,
                            "worktree_diff_sha256": MODULE.EMPTY_SHA256,
                        },
                        "openapi_sha256": "b" * 64,
                        "external_backend_image_digest": "sha256:" + "c" * 64,
                        "dev_plus_backend_image_digest": "sha256:" + "d" * 64,
                        "external_frontend_manifest_sha256": "e" * 64,
                        "dev_plus_frontend_manifest_sha256": "f" * 64,
                        "configuration_sha256": "1" * 64,
                        "migration_mapping_sha256": "2" * 64,
                        "snapshot_sha256": "3" * 64,
                        "review_manifest_sha256": "4" * 64,
                    }
                )
            elif gate == "G1":
                payload.update(
                    {
                        "violations": [],
                        "snapshot_sha256": "1" * 64,
                        "baseline_fingerprint_sha256": "2" * 64,
                        "source_attestation_sha256": "3" * 64,
                        "target_attestation_sha256": "4" * 64,
                    }
                )
                payload["evidence_sha256"] = hashlib.sha256(
                    MODULE.canonical_bytes(payload)
                ).hexdigest()
            elif gate == "G2":
                payload.update(
                    {
                        "violations": [],
                        "expected_entities": 8,
                        "observed_entities": 8,
                        "manifest_sha256": "5" * 64,
                        "observations_sha256": "6" * 64,
                        "required_gates": sorted(
                            MODULE.MANIFEST_DATABASE_GATES
                        ),
                    }
                )
                payload["evidence_sha256"] = hashlib.sha256(
                    MODULE.canonical_bytes(payload)
                ).hexdigest()
            elif gate == "G3":
                payload.update(
                    {
                        "decision": "APPROVED",
                        "candidate_sha256": "5" * 64,
                        "cohort_digest": "6" * 64,
                        "summary": {
                            "proposed_review_count": 0,
                            "hard_blocked_count": 0,
                        },
                    }
                )
            elif gate == "G4":
                raw_g4_evidence = run_dir / "g4-raw.txt"
                raw_g4_evidence.write_text("raw G4 evidence\n", encoding="utf-8")
                g4_command_plan = run_dir / "g4-command-plan.json"
                g4_command_plan.write_bytes(b"command plan\n")
                g4_mapping = run_dir / "g4-mapping.json"
                g4_mapping.write_bytes(b"mapping\n")
                g4_inputs = run_dir / "g4-run" / "inputs"
                g4_inputs.mkdir(parents=True)
                g4_auth_settings = (
                    g4_inputs / "auth_identity.clone-b.json"
                )
                g4_auth_settings.write_text(
                    json.dumps(
                        MODULE.clone_b_auth_policy.POLICY,
                        ensure_ascii=False,
                        sort_keys=True,
                    )
                    + "\n",
                    encoding="utf-8",
                )
                g4_auth_settings.chmod(0o440)
                g4_frontend_access = g4_inputs / "frontend_access.json"
                g4_frontend_access.write_bytes(b'{"roles":[]}\n')
                g4_frontend_access.chmod(0o440)
                baseline_tables = {
                    "tasks": {
                        "row_count": 2,
                        "content_sha256": "7" * 64,
                        "schema_sha256": "8" * 64,
                        "auto_increment": 3,
                        "content_fingerprint_algorithm": (
                            "sha256(sorted(sha256(canonical-json-cells-v1)),"
                            "duplicates-preserved)-v1"
                        ),
                    }
                }
                baseline_payload = {
                    "schema_version": 1,
                    "kind": "clone-b-baseline-fingerprint",
                    "database": "ab_formal_b_ui",
                    "fingerprint_algorithm": (
                        "sha256(sorted(sha256(canonical-json-cells-v1)),"
                        "duplicates-preserved)-v1"
                    ),
                    "tables": baseline_tables,
                    "fingerprint_sha256": hashlib.sha256(
                        MODULE.canonical_bytes(baseline_tables)
                    ).hexdigest(),
                }
                baseline_path = run_dir / "g4-baseline-fingerprint.json"
                baseline_file_sha = write_json(
                    baseline_path, baseline_payload
                )
                fingerprint_payload = {
                    "schema_version": 1,
                    "status": "PASS",
                    "violation_count": 0,
                    "baseline_artifact_sha256": baseline_file_sha,
                    "baseline_fingerprint_sha256": baseline_payload[
                        "fingerprint_sha256"
                    ],
                    "rollback_fingerprint_sha256": baseline_payload[
                        "fingerprint_sha256"
                    ],
                }
                fingerprint_path = run_dir / "g4-rollback-fingerprint.json"
                fingerprint_file_sha = write_json(
                    fingerprint_path, fingerprint_payload
                )
                search_tables = {
                    "task_search_documents": {
                        "row_count": 2,
                        "content_sha256": "1" * 64,
                    },
                    "task_asset_group_search_documents": {
                        "row_count": 3,
                        "content_sha256": "2" * 64,
                    },
                    "product_search_documents": {
                        "row_count": 4,
                        "content_sha256": "3" * 64,
                    },
                }
                search_snapshot_digest = hashlib.sha256(
                    MODULE.canonical_bytes(search_tables)
                ).hexdigest()
                search_archive_path = run_dir / "g4-search-snapshot.jsonl"
                search_archive_path.write_bytes(
                    b'{"table":"task_search_documents"}\n'
                )
                search_archive_sha = MODULE.sha256_file(search_archive_path)
                search_snapshot = {
                    "schema_version": 1,
                    "status": "CAPTURED",
                    "violation_count": 0,
                    "tables": search_tables,
                    "snapshot_sha256": search_snapshot_digest,
                    "archive": {
                        "format": "deterministic-jsonl-v1",
                        "sha256": search_archive_sha,
                        "size": search_archive_path.stat().st_size,
                    },
                }
                search_rollback = {
                    "schema_version": 1,
                    "status": "PASS",
                    "violation_count": 0,
                    "snapshot_sha256": search_snapshot_digest,
                    "restored_snapshot_sha256": search_snapshot_digest,
                    "restored_tables": search_tables,
                    "source_archive_sha256": search_archive_sha,
                }
                search_snapshot_path = run_dir / "g4-search-snapshot.json"
                search_rollback_path = run_dir / "g4-search-rollback.json"
                search_snapshot_file_sha = write_json(
                    search_snapshot_path, search_snapshot
                )
                search_rollback_file_sha = write_json(
                    search_rollback_path, search_rollback
                )
                step_phases = (
                    ("capture_baseline_fingerprint", "validate"),
                    ("recovery_apply", "apply"),
                    ("bundle_apply", "apply"),
                    ("dry_run_before", "validate"),
                    ("workflow_apply", "apply"),
                    ("idempotent_apply", "apply"),
                    ("validate_after_apply", "validate"),
                    ("search_snapshot", "apply"),
                    ("search_reindex", "apply"),
                    ("workflow_rollback", "rollback"),
                    ("search_rollback", "rollback"),
                    ("bundle_rollback", "rollback"),
                    ("recovery_rollback", "rollback"),
                    ("validate_after_rollback_fingerprint", "rollback"),
                )
                write_component_chain_fixture(
                    run_dir,
                    run_id=run_id,
                    database="ab_formal_b_ui",
                )
                component_chain = {}
                for component_name in ("recovery", "bundle"):
                    component_chain[component_name] = {}
                    for action in ("apply", "rollback"):
                        component_path = (
                            run_dir
                            / f"{component_name}-component-{action}.json"
                        )
                        component_value = json.loads(
                            component_path.read_text(encoding="utf-8")
                        )
                        component_value["artifact_sha256"] = (
                            MODULE.sha256_file(component_path)
                        )
                        component_chain[component_name][action] = (
                            component_value
                        )
                component_files = sorted(
                    path
                    for path in run_dir.iterdir()
                    if path.name.startswith(("recovery-", "bundle-"))
                )
                payload.update(
                    {
                        "exit_code": 0,
                        "elapsed_seconds": 8.5,
                        "timings_seconds": {
                            "apply": 3.0,
                            "validate": 2.0,
                            "rollback": 3.5,
                            "total": 8.5,
                        },
                        "steps": [
                            {
                                "step": name,
                                "phase": phase,
                                "status": "PASS",
                                "exit_code": 0,
                                "elapsed_seconds": 0.5,
                                "command_sha256": "8" * 64,
                            }
                            for name, phase in step_phases
                        ],
                        "rollback_fingerprint": fingerprint_payload,
                        "baseline_fingerprint": baseline_payload,
                        "baseline_fingerprint_artifact_sha256": (
                            baseline_file_sha
                        ),
                        "rollback_fingerprint_sha256": fingerprint_file_sha,
                        "search_restore": {
                            "snapshot": search_snapshot,
                            "rollback": search_rollback,
                        },
                        "component_chain": component_chain,
                        "search_snapshot_sha256": search_snapshot_file_sha,
                        "search_snapshot_archive_sha256": search_archive_sha,
                        "search_rollback_sha256": search_rollback_file_sha,
                        "input_sha256": {
                            "command_plan": MODULE.sha256_file(g4_command_plan),
                            "mapping": MODULE.sha256_file(g4_mapping),
                            "auth_settings": MODULE.sha256_file(g4_auth_settings),
                            "frontend_access_settings": MODULE.sha256_file(
                                g4_frontend_access
                            ),
                        },
                        "auth_settings_attestation": {
                            "frozen_input_path": g4_auth_settings.relative_to(
                                run_dir
                            ).as_posix(),
                            "byte_count": g4_auth_settings.stat().st_size,
                            "sha256": MODULE.sha256_file(g4_auth_settings),
                            "read_only": True,
                            "super_admin_count": 0,
                            "department_admin_key_count": 0,
                            "configured_user_assignment_count": 0,
                        },
                        "evidence_inventory": [
                            {
                                "path": raw_g4_evidence.name,
                                "sha256": MODULE.sha256_file(raw_g4_evidence),
                            },
                            {
                                "path": g4_command_plan.name,
                                "sha256": MODULE.sha256_file(g4_command_plan),
                            },
                            {
                                "path": g4_mapping.name,
                                "sha256": MODULE.sha256_file(g4_mapping),
                            },
                            {
                                "path": g4_auth_settings.relative_to(
                                    run_dir
                                ).as_posix(),
                                "sha256": MODULE.sha256_file(g4_auth_settings),
                            },
                            {
                                "path": g4_frontend_access.relative_to(
                                    run_dir
                                ).as_posix(),
                                "sha256": MODULE.sha256_file(
                                    g4_frontend_access
                                ),
                            },
                            {
                                "path": baseline_path.name,
                                "sha256": baseline_file_sha,
                            },
                            {
                                "path": fingerprint_path.name,
                                "sha256": fingerprint_file_sha,
                            },
                            {
                                "path": search_snapshot_path.name,
                                "sha256": search_snapshot_file_sha,
                            },
                            {
                                "path": search_rollback_path.name,
                                "sha256": search_rollback_file_sha,
                            },
                            {
                                "path": search_archive_path.name,
                                "sha256": search_archive_sha,
                            },
                        ]
                        + [
                            {
                                "path": path.name,
                                "sha256": MODULE.sha256_file(path),
                            }
                            for path in component_files
                        ],
                    }
                )
            elif gate == "G5":
                payload.update(
                    {
                        "violations": [],
                        "gates": [
                            {
                                "gate": name,
                                "a_assessment": "baseline_or_immutable_parity",
                                "b_assessment": (
                                    "approved_manifest_and_v8_invariants"
                                ),
                                "a_violation_count": 0,
                                "b_violation_count": 0,
                                "a_json_sha256": "2" * 64,
                                "b_json_sha256": "3" * 64,
                            }
                            for name in MODULE.SQL_GATE_NAMES
                        ],
                        "immutable_event_parity": {
                            "schema_version": 1,
                            "gate": "07_event_history_checksum",
                            "status": "PASS",
                            "violation_count": 0,
                            "violations": [],
                            "source_evidence_sha256": "1" * 64,
                            "target_evidence_sha256": "1" * 64,
                        },
                    }
                )
            elif gate == "G6":
                payload.update(
                    {
                        "task_count": 1,
                        "group_count": 0,
                        "task_asset_count": 0,
                        "request_count": 4,
                        "combination_matrix": [
                            {
                                "id": combo_id,
                                "frontend": values[0],
                                "backend": values[1],
                                "data": values[2],
                            }
                            for combo_id, values in MODULE.API_COMBINATIONS.items()
                        ],
                        "identities": [{"id": "admin", "role": "admin"}],
                        "task_ids_sha256": "3" * 64,
                        "matrix_sha256": "4" * 64,
                        "rules_sha256": "5" * 64,
                        "manifest_sha256": "6" * 64,
                        "used_rule_ids": [],
                        "unused_rule_ids": ["approved-difference"],
                        "observations": [
                            {
                                "combination": combo_id,
                                "identity": "admin",
                                "route": "/v1/tasks/{task_id}",
                                "entity_key": "task:1",
                                "status": 200,
                                "body_sha256": "1" * 64,
                                "raw_sha256": "2" * 64,
                                "body_bytes": 10,
                            }
                            for combo_id in MODULE.API_COMBINATIONS
                        ],
                        "violations": [],
                    }
                )
                payload["evidence_sha256"] = hashlib.sha256(
                    json.dumps(
                        payload,
                        ensure_ascii=False,
                        sort_keys=True,
                        separators=(",", ":"),
                    ).encode("utf-8")
                ).hexdigest()
            elif gate == "G7":
                payload.pop("violation_count")
                payload.update(
                    {
                        "gate": "G7",
                        "scenario_catalog_sha256": "1" * 64,
                        "browser_evidence_sha256": "2" * 64,
                        "playwright_evidence_sha256": "3" * 64,
                        "required_case_count": 8,
                        "passed_case_count": 8,
                        "failed_case_count": 0,
                        "critical_pass_rate": 1.0,
                        "source_kinds": [
                            "browser_computer_use",
                            "playwright",
                        ],
                        "failures": [],
                        "cases": [
                            {
                                "scenario_id": "baseline_four_edge_readonly",
                                "combination": combination,
                                "viewport": viewport,
                                "status": "PASS",
                                "browser_record_sha256": "4" * 64,
                                "playwright_record_sha256": "5" * 64,
                                "pair_sha256": g7_pair_hash(
                                    combination, viewport
                                ),
                            }
                            for combination in (
                                "external_external",
                                "devplus_devplus",
                                "external_devplus",
                                "devplus_external",
                            )
                            for viewport in ("desktop", "mobile")
                        ],
                        "generated_at": "2026-07-23T12:00:00Z",
                    }
                )
            elif gate == "G8":
                payload = {
                    "schema_version": 1,
                    "status": "PASS",
                    "violation_count": 0,
                    "checked_count": 1,
                    "manifest_sha256": "7" * 64,
                    "exception_count": 1,
                    "exception_evidence_sha256": "8" * 64,
                    "mapping_sha256": "9" * 64,
                    "mapping_row_hash": "a" * 64,
                    "exceptions": [
                        {
                            "entity_key": "task_asset:12323",
                            "task_id": 2199,
                            "missing_task_asset_id": 12323,
                            "expected_http_status": 410,
                            "observed_http_status": 410,
                            "mapping_row_hash": "a" * 64,
                            "object_row_sha256": "b" * 64,
                            "working_reference_count": 0,
                            "finalized_reference_count": 0,
                        }
                    ],
                    "violations": [],
                }
                payload["evidence_hash"] = hashlib.sha256(
                    json.dumps(
                        payload,
                        ensure_ascii=False,
                        sort_keys=True,
                        separators=(",", ":"),
                    ).encode("utf-8")
                ).hexdigest()
            elif gate == "G9":
                payload.update(
                    {"unresolved_p0_count": 0, "unresolved_p1_count": 0}
                )
            elif gate == "G10":
                payload["timings_seconds"] = {
                    "apply": 100,
                    "validate": 200,
                    "rollback": 100,
                    "total": 400,
                }
            if gate == blocked_gate:
                payload["status"] = "FAIL"
                payload["violation_count"] = 1
            path = run_dir / f"{gate.lower()}.json"
            digest = write_json(path, payload)
            gates[gate] = {
                "path": path.name,
                "sha256": digest,
                "executor": f"{gate}-executor",
                "reviewer": f"{gate}-reviewer",
            }
        index = {
            "schema_version": 1,
            "run_id": run_id,
            "gates": gates,
            "signatures": [],
        }
        unsigned_digest = hashlib.sha256(MODULE.canonical_bytes(index)).hexdigest()
        if signed:
            index["signatures"] = [
                {
                    "role": role,
                    "signer": f"signer-{position}",
                    "decision": "GO",
                    "evidence_index_sha256": unsigned_digest,
                    "signed_at": "2026-07-23T12:00:00Z",
                }
                for position, role in enumerate(sorted(MODULE.REQUIRED_ROLES), 1)
            ]
        index_path = run_dir / "evidence-index.json"
        write_json(index_path, index)
        return run_dir, index_path

    def test_all_exact_evidence_and_independent_signatures_can_go(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw)
            _, report = MODULE.validate_index(run_dir, index_path)
            self.assertEqual(report["decision"], "GO")
            self.assertEqual(report["passed_gate_count"], 11)
            MODULE.write_outputs(
                run_dir,
                report,
                run_dir / "final-gate-report.json",
                run_dir / "go-no-go.md",
                run_dir / "final-ledger.json",
            )
            self.assertIn(
                "decision: GO",
                (run_dir / "go-no-go.md").read_text(encoding="utf-8"),
            )

    def test_one_failed_gate_is_no_go(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw, blocked_gate="G6")
            _, report = MODULE.validate_index(run_dir, index_path)
            self.assertEqual(report["decision"], "NO-GO")
            self.assertEqual(report["gates"]["G6"]["status"], "BLOCKED")

    def test_missing_signature_is_no_go(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw, signed=False)
            _, report = MODULE.validate_index(run_dir, index_path)
            self.assertEqual(report["decision"], "NO-GO")
            self.assertTrue(report["signature_violations"])

    def test_hash_drift_is_blocked(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw)
            (run_dir / "g5.json").write_text("{}\n", encoding="utf-8")
            _, report = MODULE.validate_index(run_dir, index_path)
            self.assertEqual(report["decision"], "NO-GO")
            self.assertIn(
                "artifact hash mismatch",
                report["gates"]["G5"]["violations"][0],
            )

    def test_dirty_candidate_is_blocked(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw)
            index = json.loads(index_path.read_text(encoding="utf-8"))
            env_path = run_dir / index["gates"]["G0"]["path"]
            env = json.loads(env_path.read_text(encoding="utf-8"))
            env["candidate"]["worktree_diff_sha256"] = "9" * 64
            index["gates"]["G0"]["sha256"] = write_json(env_path, env)
            index["signatures"] = []
            write_json(index_path, index)
            _, report = MODULE.validate_index(run_dir, index_path)
            self.assertEqual(report["decision"], "NO-GO")
            self.assertIn(
                "candidate worktree is not clean",
                report["gates"]["G0"]["violations"],
            )

    def test_g4_rejects_string_exit_code_and_incomplete_chain(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw)
            index = json.loads(index_path.read_text(encoding="utf-8"))
            g4_path = run_dir / index["gates"]["G4"]["path"]
            g4 = json.loads(g4_path.read_text(encoding="utf-8"))
            g4["steps"][0]["exit_code"] = "0"
            g4["steps"].pop(2)
            index["gates"]["G4"]["sha256"] = write_json(g4_path, g4)
            index["signatures"] = []
            write_json(index_path, index)
            _, report = MODULE.validate_index(run_dir, index_path)
            self.assertEqual(report["decision"], "NO-GO")
            violations = report["gates"]["G4"]["violations"]
            self.assertTrue(
                any("numeric zero" in item for item in violations), violations
            )
            self.assertTrue(
                any("ordered step sequence differs" in item for item in violations),
                violations,
            )

    def test_g4_requires_hash_bound_zero_secret_clone_b_auth_settings(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw)
            index = json.loads(index_path.read_text(encoding="utf-8"))
            g4_path = run_dir / index["gates"]["G4"]["path"]
            g4 = json.loads(g4_path.read_text(encoding="utf-8"))
            del g4["input_sha256"]["auth_settings"]
            g4["auth_settings_attestation"]["super_admin_count"] = 1
            index["gates"]["G4"]["sha256"] = write_json(g4_path, g4)
            index["signatures"] = []
            write_json(index_path, index)

            _, report = MODULE.validate_index(run_dir, index_path)

            self.assertEqual(report["decision"], "NO-GO")
            violations = report["gates"]["G4"]["violations"]
            self.assertIn("G4 input hash binding is incomplete", violations)
            self.assertIn(
                "G4 Clone B auth settings attestation is invalid",
                violations,
            )

    def test_g4_rejects_hash_consistent_nonzero_auth_payload(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw)
            index = json.loads(index_path.read_text(encoding="utf-8"))
            g4_path = run_dir / index["gates"]["G4"]["path"]
            g4 = json.loads(g4_path.read_text(encoding="utf-8"))
            auth_relative = g4["auth_settings_attestation"][
                "frozen_input_path"
            ]
            auth_path = run_dir / auth_relative
            payload = json.loads(auth_path.read_text(encoding="utf-8"))
            payload["super_admins"] = [{"username": "forged"}]
            auth_path.chmod(0o640)
            auth_path.write_text(
                json.dumps(payload, ensure_ascii=False, sort_keys=True) + "\n",
                encoding="utf-8",
            )
            auth_path.chmod(0o440)
            forged_sha = MODULE.sha256_file(auth_path)
            g4["input_sha256"]["auth_settings"] = forged_sha
            g4["auth_settings_attestation"]["sha256"] = forged_sha
            g4["auth_settings_attestation"]["byte_count"] = (
                auth_path.stat().st_size
            )
            for row in g4["evidence_inventory"]:
                if row["path"] == auth_relative:
                    row["sha256"] = forged_sha
            index["gates"]["G4"]["sha256"] = write_json(g4_path, g4)
            index["signatures"] = []
            write_json(index_path, index)

            _, report = MODULE.validate_index(run_dir, index_path)

            self.assertEqual(report["decision"], "NO-GO")
            self.assertTrue(
                any(
                    "auth settings evidence is invalid" in item
                    for item in report["gates"]["G4"]["violations"]
                ),
                report["gates"]["G4"]["violations"],
            )

    def test_g4_raw_evidence_hash_drift_is_blocked(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw)
            (run_dir / "g4-raw.txt").write_text("tampered\n", encoding="utf-8")
            _, report = MODULE.validate_index(run_dir, index_path)
            self.assertEqual(report["decision"], "NO-GO")
            self.assertTrue(
                any(
                    "G4 evidence[0] is invalid" in item
                    for item in report["gates"]["G4"]["violations"]
                ),
                report["gates"]["G4"]["violations"],
            )

    def test_g4_baseline_artifact_tamper_is_blocked(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw)
            (run_dir / "g4-baseline-fingerprint.json").write_text(
                '{"tampered":true}\n', encoding="utf-8"
            )
            _, report = MODULE.validate_index(run_dir, index_path)
            self.assertEqual(report["decision"], "NO-GO")
            violations = report["gates"]["G4"]["violations"]
            self.assertTrue(
                any("G4 evidence" in item and "invalid" in item for item in violations),
                violations,
            )

    def test_g4_baseline_payload_omission_is_blocked(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw)
            index = json.loads(index_path.read_text(encoding="utf-8"))
            g4_path = run_dir / index["gates"]["G4"]["path"]
            g4 = json.loads(g4_path.read_text(encoding="utf-8"))
            del g4["baseline_fingerprint"]
            index["gates"]["G4"]["sha256"] = write_json(g4_path, g4)
            index["signatures"] = []
            write_json(index_path, index)
            _, report = MODULE.validate_index(run_dir, index_path)
            self.assertEqual(report["decision"], "NO-GO")
            self.assertIn(
                "G4 baseline fingerprint is missing",
                report["gates"]["G4"]["violations"],
            )

    def test_g4_rejects_unbound_full_fingerprint_algorithm(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw)
            index = json.loads(index_path.read_text(encoding="utf-8"))
            g4_path = run_dir / index["gates"]["G4"]["path"]
            g4 = json.loads(g4_path.read_text(encoding="utf-8"))
            g4["baseline_fingerprint"]["fingerprint_algorithm"] = "legacy"
            violations = MODULE.validate_g4(g4, run_dir)
            self.assertIn(
                "G4 baseline fingerprint envelope is invalid",
                violations,
            )

    def test_g4_search_restore_mismatch_is_blocked(self):
        with tempfile.TemporaryDirectory() as raw:
            run_dir, index_path = self.make_run(raw)
            index = json.loads(index_path.read_text(encoding="utf-8"))
            g4_path = run_dir / index["gates"]["G4"]["path"]
            g4 = json.loads(g4_path.read_text(encoding="utf-8"))
            g4["search_restore"]["rollback"]["restored_tables"][
                "task_search_documents"
            ]["row_count"] += 1
            index["gates"]["G4"]["sha256"] = write_json(g4_path, g4)
            index["signatures"] = []
            write_json(index_path, index)
            _, report = MODULE.validate_index(run_dir, index_path)
            self.assertEqual(report["decision"], "NO-GO")
            self.assertIn(
                "G4 search rollback is not an exact snapshot restore",
                report["gates"]["G4"]["violations"],
            )

    def test_real_executor_gate_envelopes_fail_closed_on_content_tampering(self):
        cases = (
            (
                "G1",
                lambda payload: payload.__setitem__("snapshot_sha256", "9" * 64),
                "does not bind",
            ),
            (
                "G2",
                lambda payload: payload.__setitem__("observed_entities", 7),
                "entity counts",
            ),
            (
                "G5",
                lambda payload: payload["gates"].pop(),
                "exactly the 00-12",
            ),
            (
                "G6",
                lambda payload: payload.__setitem__("request_count", 2),
                "observations length",
            ),
            (
                "G7",
                lambda payload: payload.__setitem__("failed_case_count", 1),
                "case counts",
            ),
            (
                "G7",
                lambda payload: payload["cases"][0].__setitem__(
                    "pair_sha256", "9" * 64
                ),
                "does not bind the pair",
            ),
            (
                "G8",
                lambda payload: payload["exceptions"][0].__setitem__(
                    "observed_http_status", 404
                ),
                "exact HTTP 410",
            ),
        )
        for gate, mutate, expected in cases:
            with self.subTest(gate=gate), tempfile.TemporaryDirectory() as raw:
                run_dir, index_path = self.make_run(raw)
                index = json.loads(index_path.read_text(encoding="utf-8"))
                artifact = run_dir / index["gates"][gate]["path"]
                payload = json.loads(artifact.read_text(encoding="utf-8"))
                mutate(payload)
                index["gates"][gate]["sha256"] = write_json(artifact, payload)
                index["signatures"] = []
                write_json(index_path, index)

                _, report = MODULE.validate_index(run_dir, index_path)

                self.assertEqual("NO-GO", report["decision"])
                self.assertTrue(
                    any(
                        expected in item
                        for item in report["gates"][gate]["violations"]
                    ),
                    report["gates"][gate]["violations"],
                )

    def test_g8_semantic_exception_mutations_fail_even_with_fresh_self_hash(self):
        cases = (
            (
                lambda payload: payload.__setitem__("exception_count", 0),
                "exception_count must be exactly 1",
            ),
            (
                lambda payload: payload["exceptions"][0].__setitem__(
                    "entity_key", "task_asset:12324"
                ),
                "entity must be task 2199 asset 12323",
            ),
            (
                lambda payload: payload["exceptions"][0].__setitem__(
                    "observed_http_status", 404
                ),
                "exact HTTP 410",
            ),
            (
                lambda payload: payload["exceptions"][0].__setitem__(
                    "working_reference_count", 1
                ),
                "current revision reference",
            ),
            (
                lambda payload: payload.__setitem__("mapping_row_hash", "c" * 64),
                "mapping row hash differs",
            ),
        )
        for mutate, expected in cases:
            with self.subTest(expected=expected), tempfile.TemporaryDirectory() as raw:
                run_dir, index_path = self.make_run(raw)
                index = json.loads(index_path.read_text(encoding="utf-8"))
                artifact = run_dir / index["gates"]["G8"]["path"]
                payload = json.loads(artifact.read_text(encoding="utf-8"))
                mutate(payload)
                unsigned = {
                    key: value
                    for key, value in payload.items()
                    if key != "evidence_hash"
                }
                payload["evidence_hash"] = hashlib.sha256(
                    json.dumps(
                        unsigned,
                        ensure_ascii=False,
                        sort_keys=True,
                        separators=(",", ":"),
                    ).encode("utf-8")
                ).hexdigest()
                index["gates"]["G8"]["sha256"] = write_json(artifact, payload)
                index["signatures"] = []
                write_json(index_path, index)

                _, report = MODULE.validate_index(run_dir, index_path)

                self.assertEqual("NO-GO", report["decision"])
                self.assertTrue(
                    any(
                        expected in item
                        for item in report["gates"]["G8"]["violations"]
                    ),
                    report["gates"]["G8"]["violations"],
                )

    def test_real_executor_gate_envelopes_reject_unexpected_fields(self):
        for gate in ("G1", "G2", "G5", "G6", "G7", "G8"):
            with self.subTest(gate=gate), tempfile.TemporaryDirectory() as raw:
                run_dir, index_path = self.make_run(raw)
                index = json.loads(index_path.read_text(encoding="utf-8"))
                artifact = run_dir / index["gates"][gate]["path"]
                payload = json.loads(artifact.read_text(encoding="utf-8"))
                payload["invented_summary"] = "not produced by the executor"
                index["gates"][gate]["sha256"] = write_json(artifact, payload)
                index["signatures"] = []
                write_json(index_path, index)

                _, report = MODULE.validate_index(run_dir, index_path)

                self.assertEqual("NO-GO", report["decision"])
                self.assertTrue(
                    any(
                        "field contract differs" in item
                        for item in report["gates"][gate]["violations"]
                    ),
                    report["gates"][gate]["violations"],
                )


if __name__ == "__main__":
    unittest.main()
