from __future__ import annotations

import json
import hashlib
import pathlib
import tempfile
import unittest
import uuid

from scripts.ab import finalize_g06_verdict as MODULE
from scripts.ab import finalize_release_gates as RELEASE
from scripts.ab import historical_unavailable_exception as HISTORICAL
from scripts.ab import g06_recovery_contract as RECOVERY
from scripts.ab import hydrate_object_manifest as HYDRATOR
from scripts.ab import manifest_loader as LOADER
from scripts.ab import object_manifest_verifier as OBJECTS
from scripts.ab import verify_g06_clone_b_recoveries as RECOVERY_VERIFIER


class FinalizeG06VerdictTest(unittest.TestCase):
    def write_json(self, path: pathlib.Path, value: dict) -> None:
        path.write_text(MODULE.canonical_json(value) + "\n", encoding="utf-8")

    def remote_row(self, owner_id: int, sha256: str) -> dict:
        return {
            "entity_key": f"task_asset:{owner_id}",
            "owner_kind": "task_asset",
            "owner_id": owner_id,
            "task_id": 10,
            "storage_ref_id": f"ref-{owner_id}",
            "storage_adapter": "oss_upload_service",
            "object_key": f"tasks/10/{owner_id}.bin",
            "size": owner_id,
            "mime_type": "application/octet-stream",
            "sha256": sha256,
            "status": "recorded",
            "is_placeholder": False,
        }

    def exception_row(self) -> dict:
        return {
            "entity_key": HISTORICAL.ENTITY_KEY,
            "owner_kind": "task_asset",
            "owner_id": HISTORICAL.TASK_ASSET_ID,
            "task_id": HISTORICAL.TASK_ID,
            "storage_ref_id": HISTORICAL.EXPECTED_STORAGE_REF_ID,
            "storage_adapter": HISTORICAL.EXPECTED_STORAGE_ADAPTER,
            "object_key": HISTORICAL.EXPECTED_OBJECT_KEY,
            "size": HISTORICAL.EXPECTED_SIZE,
            "mime_type": HISTORICAL.EXPECTED_MIME_TYPE,
            "sha256": "",
            "status": HISTORICAL.EXPECTED_STATUS,
            "is_placeholder": False,
        }

    def bundle_rows(self) -> list[dict]:
        rows = []
        for owner_id, task_id in MODULE.BUNDLE_TASKS.items():
            rows.append(
                {
                    "entity_key": f"task_asset:{owner_id}",
                    "owner_kind": "task_asset",
                    "owner_id": owner_id,
                    "task_id": task_id,
                    "storage_ref_id": f"bundle-ref-{owner_id}",
                    "storage_adapter": "clone_b_bundle",
                    "object_key": (
                        f"fixture/run/migration-bundles/task-{task_id}/"
                        f"asset-{owner_id}/source-bundle.zip"
                    ),
                    "size": owner_id,
                    "mime_type": "application/zip",
                    "sha256": f"{owner_id:064x}",
                    "status": "recorded",
                    "is_placeholder": False,
                }
            )
        return rows

    def mapping(self) -> dict:
        row = {
            "task_id": HISTORICAL.TASK_ID,
            "missing_task_asset_id": HISTORICAL.TASK_ASSET_ID,
            "strategy": HISTORICAL.STRATEGY,
            "review_policy_ids": [HISTORICAL.POLICY_ID],
            "confidence": "confirmed_auto",
            "confirmed_by": 1,
            "confirmed_at": "2026-07-24T02:43:42Z",
            "confirmation_note": "admin confirmed exact historical tombstone",
            "recovery_source_task_asset_id": 0,
            "original_storage_ref_id": HISTORICAL.EXPECTED_STORAGE_REF_ID,
            "expected_file_size": HISTORICAL.EXPECTED_SIZE,
            "object_probe_result": HISTORICAL.EXPECTED_PROBE_RESULT,
            "object_probe_read_only_get_count": (
                HISTORICAL.EXPECTED_PROBE_READ_ONLY_GET_COUNT
            ),
            "object_probe_evidence_hash": (
                HISTORICAL.EXPECTED_PROBE_EVIDENCE_HASH
            ),
            "object_probe_input_manifest_sha256": (
                HISTORICAL.EXPECTED_PROBE_INPUT_MANIFEST_SHA256
            ),
            "object_probe_object_key_sha256": (
                HISTORICAL.EXPECTED_PROBE_OBJECT_KEY_SHA256
            ),
            "blockers": [],
        }
        row["manifest_row_hash"] = HISTORICAL.canonical_hash(row)
        recoveries = [row]
        for missing_id in RECOVERY.RECOVERY_IDS:
            source_id, size = RECOVERY.REQUIRED_SOURCES[missing_id]
            recovery = {
                "task_id": RECOVERY.TASK_ID,
                "missing_task_asset_id": missing_id,
                "recovery_source_task_asset_id": source_id,
                "expected_file_size": size,
                "strategy": RECOVERY.STRATEGY,
                "review_policy_ids": [RECOVERY.POLICY],
                "confidence": "confirmed_auto",
                "confirmed_by": 1,
                "confirmed_at": "2026-07-24T02:43:42Z",
                "confirmation_note": "approved recovery fixture",
                "blockers": [],
                "original_storage_ref_id": f"original-ref-{missing_id}",
                "recovery_source_sha256": hashlib.sha256(
                    bytes([missing_id % 251]) * size
                ).hexdigest(),
                "recovery_source_storage_ref_id": f"source-ref-{missing_id}",
                "controlled_read_protocol": "controlled-asset-read-v1",
                "controlled_read_evidence_sha256": "e" * 64,
            }
            recovery["manifest_row_hash"] = RECOVERY.canonical_hash(recovery)
            recoveries.append(recovery)
        return {"version": 2, "asset_recoveries": recoveries}

    def recovery_material(
        self, root: pathlib.Path, mapping_path: pathlib.Path
    ) -> dict[str, pathlib.Path | list[dict]]:
        mapping = json.loads(mapping_path.read_text(encoding="utf-8"))
        mapping_sha = MODULE.sha256_file(mapping_path)
        by_id = {
            row["missing_task_asset_id"]: row
            for row in mapping["asset_recoveries"]
            if row["missing_task_asset_id"] in RECOVERY.RECOVERY_IDS
        }
        entries = []
        rows = []
        run_id = "fixture-run"
        for missing_id in RECOVERY.RECOVERY_IDS:
            row = by_id[missing_id]
            source_id, size = RECOVERY.REQUIRED_SOURCES[missing_id]
            source_sha = row["recovery_source_sha256"]
            target_ref = str(
                uuid.uuid5(
                    RECOVERY.RECOVERY_NAMESPACE,
                    f"{run_id}:{mapping_sha}:{missing_id}:{source_sha}",
                )
            )
            target_key = (
                f"v8-ab/{run_id}/recovered/task-2807/"
                f"task-asset-{missing_id}/{source_sha}.bin"
            )
            request_id = f"request-{missing_id}"
            storage = {
                "ref_id": target_ref,
                "ref_key": target_key,
                "owner_type": "task_asset",
                "owner_id": missing_id,
                "file_size": size,
                "mime_type": "image/jpeg",
                "checksum_hint": source_sha,
                "status": "recorded",
                "is_placeholder": 0,
            }
            entries.append(
                {
                    "missing_task_asset_id": missing_id,
                    "source_task_asset_id": source_id,
                    "source_size": size,
                    "source_sha256": source_sha,
                    "target_storage_ref_id": target_ref,
                    "target_object_key": target_key,
                    "db_apply_plan": {
                        "insert_asset_storage_ref": storage,
                        "update_task_asset": {
                            "where": {"id": missing_id},
                            "set": {
                                "storage_ref_id": target_ref,
                                "storage_key": target_key,
                                "whole_hash": source_sha,
                            },
                        },
                        "update_upload_request": {
                            "where": {"request_id": request_id},
                            "set": {
                                "bound_ref_id": target_ref,
                                "checksum_hint": source_sha,
                                "file_size": size,
                                "status": "bound",
                                "session_status": "completed",
                            },
                        },
                    },
                    "rollback_registry": {
                        "original_storage_ref": {
                            "ref_id": row["original_storage_ref_id"],
                            "ref_key": f"tasks/2807/original-{missing_id}.jpg",
                            "storage_adapter": "oss_upload_service",
                            "file_size": size,
                            "mime_type": "image/jpeg",
                            "status": "recorded",
                            "is_placeholder": 0,
                        },
                        "restore_task_asset": {
                            "id": missing_id,
                            "task_id": RECOVERY.TASK_ID,
                            "file_size": size,
                            "storage_ref_id": row["original_storage_ref_id"],
                            "upload_request_id": request_id,
                        },
                        "restore_upload_request": {
                            "request_id": request_id,
                        },
                    },
                }
            )
            rows.append(
                {
                    "entity_key": f"task_asset:{missing_id}",
                    "owner_kind": "task_asset",
                    "owner_id": missing_id,
                    "task_id": RECOVERY.TASK_ID,
                    "storage_ref_id": target_ref,
                    "storage_adapter": RECOVERY.FINAL_STORAGE_ADAPTER,
                    "object_key": target_key,
                    "size": size,
                    "mime_type": "image/jpeg",
                    "sha256": source_sha,
                    "status": "recorded",
                    "is_placeholder": False,
                }
            )
        plan_path = root / "recovery-plan.json"
        self.write_json(
            plan_path,
            {
                "version": 1,
                "status": "MATERIALIZED",
                "run_id": run_id,
                "mapping_sha256": mapping_sha,
                "database_writes_executed": False,
                "production_writes_executed": False,
                "entries": entries,
            },
        )
        plan_sha = MODULE.sha256_file(plan_path)
        receipt_fields = {
            "version": 1,
            "mode": "apply",
            "run_id": "bundle-materialization-20260723-29",
            "database": RECOVERY.EXPECTED_DATABASE,
            "host": RECOVERY.EXPECTED_HOST,
            "plan_sha256": plan_sha,
            "database_transaction_committed": True,
            "object_storage_writes_executed": False,
            "executed_at": "2026-07-24T00:00:00Z",
        }
        apply_path = root / "recovery-db-apply.json"
        self.write_json(
            apply_path,
            {
                **receipt_fields,
                "changed_entries": 3,
                "already_in_target_state_entries": 0,
            },
        )
        idempotent_path = root / "recovery-db-idempotent.json"
        self.write_json(
            idempotent_path,
            {
                **receipt_fields,
                "changed_entries": 0,
                "already_in_target_state_entries": 3,
            },
        )
        artifacts = []
        for path in (plan_path, apply_path, idempotent_path):
            artifacts.append(
                {
                    "path": {
                        plan_path: "recovery-materialization-plan.json",
                        apply_path: "recovery-db-apply.json",
                        idempotent_path: "recovery-db-idempotent.json",
                    }[path],
                    "sha256": MODULE.sha256_file(path),
                    "size": path.stat().st_size,
                }
            )
        for name in ("recovery-guard-before.json", "recovery-guard-provision.json"):
            artifacts.append({"path": name, "sha256": "f" * 64, "size": 1})
        component = {
            "schema_version": 1,
            "status": "APPLIED",
            "component": "recovery",
            "action": "apply",
            "run_id": "bundle-materialization-20260723-29",
            "database": RECOVERY.EXPECTED_DATABASE,
            "host": RECOVERY.EXPECTED_HOST,
            "database_writes_executed": True,
            "production_writes_executed": False,
            "guard_retained_for_rollback": True,
            "guard_exactly_restored": False,
            "artifacts": artifacts,
        }
        component["evidence_sha256"] = hashlib.sha256(
            (RECOVERY.canonical_json(component) + "\n").encode("utf-8")
        ).hexdigest()
        component_path = root / "recovery-component-apply.json"
        self.write_json(component_path, component)
        subset_sha = MODULE.sha256_bytes(MODULE.canonical_jsonl(rows))
        verdict = {
            "schema_version": RECOVERY_VERIFIER.SCHEMA_VERSION,
            "verdict_type": RECOVERY_VERIFIER.VERDICT_TYPE,
            "status": "PASS",
            "violation_count": 0,
            "checked_count": 3,
            "recovery_manifest_sha256": subset_sha,
            "mapping_sha256": mapping_sha,
            "recovery_plan_sha256": plan_sha,
            "recovery_db_apply_sha256": MODULE.sha256_file(apply_path),
            "recovery_db_idempotent_sha256": MODULE.sha256_file(idempotent_path),
            "recovery_component_apply_sha256": MODULE.sha256_file(component_path),
            "database": RECOVERY.EXPECTED_DATABASE,
            "read_only_local_get_count": 3,
            "database_write_performed": False,
            "production_write_performed": False,
            "violations": [],
        }
        verdict["evidence_hash"] = MODULE.sha256_bytes(
            MODULE.canonical_json(verdict).encode()
        )
        verdict_path = root / "recovery-verdict.json"
        self.write_json(verdict_path, verdict)
        return {
            "rows": rows,
            "plan": plan_path,
            "plan_sha": plan_sha,
            "apply": apply_path,
            "idempotent": idempotent_path,
            "component": component_path,
            "verdict": verdict_path,
        }

    def fixture(self, root: pathlib.Path) -> dict[str, pathlib.Path | str]:
        mapping_path = root / "mapping.json"
        self.write_json(mapping_path, self.mapping())
        mapping_sha, mapping_row_hash, _row = HISTORICAL.validate_mapping(
            mapping_path
        )
        recovery = self.recovery_material(root, mapping_path)
        hydration_input_rows = sorted(
            [self.remote_row(1, "a" * 64), self.remote_row(2, "")],
            key=MODULE.row_sort_key,
        )
        final_remote_rows = [dict(row) for row in hydration_input_rows]
        final_remote_rows[1]["sha256"] = "b" * 64
        final_rows = sorted(
            final_remote_rows
            + self.bundle_rows()
            + recovery["rows"]
            + [self.exception_row()],
            key=MODULE.row_sort_key,
        )
        hydration_input = root / "hydration-input.jsonl"
        final_manifest = root / "final-manifest.jsonl"
        hydration_input.write_bytes(MODULE.canonical_jsonl(hydration_input_rows))
        final_manifest.write_bytes(MODULE.canonical_jsonl(final_rows))
        input_sha = MODULE.sha256_file(hydration_input)
        final_sha = MODULE.sha256_file(final_manifest)
        remote_sha = MODULE.sha256_bytes(
            MODULE.canonical_jsonl(final_remote_rows)
        )
        completed = HYDRATOR.checkpoint_record(
            "upload",
            hydration_input_rows[1]["object_key"],
            HYDRATOR.ObjectMetadata(
                size=final_remote_rows[1]["size"],
                mime_type=final_remote_rows[1]["mime_type"],
                sha256=final_remote_rows[1]["sha256"],
            ),
        )
        checkpoint_doc = HYDRATOR.checkpoint_document(
            input_sha,
            {"upload": "a" * 64, "oss": "b" * 64},
            {
                HYDRATOR.checkpoint_key(
                    "upload", hydration_input_rows[1]["object_key"]
                ): completed
            },
            {},
        )
        checkpoint_path = root / "hydration-checkpoint.json"
        self.write_json(checkpoint_path, checkpoint_doc)
        checkpoint_sha = MODULE.sha256_file(checkpoint_path)
        hydration = {
            "schema_version": 1,
            "status": "PASS",
            "input_manifest_sha256": input_sha,
            "hydrated_manifest_sha256": remote_sha,
            "checkpoint_sha256": checkpoint_sha,
            "row_count": 2,
            "already_complete_count": 1,
            "missing_sha256_count": 1,
            "configured_target_row_count": 1,
            "unique_target_count": 1,
            "resumed_target_count": 0,
            "resumed_failure_target_count": 0,
            "retried_transient_failure_target_count": 0,
            "retried_authorized_failure_target_count": 0,
            "failure_retry_authorization_sha256": MODULE.ZERO_SHA256,
            "read_only_get_count": 1,
            "hydrated_row_count": 1,
            "deduplicated_get_count": 0,
            "failure_count": 0,
            "failures": [],
        }
        hydration["evidence_hash"] = MODULE.sha256_bytes(
            MODULE.canonical_json(hydration).encode("utf-8")
        )
        hydration_path = root / "hydration-evidence.json"
        self.write_json(hydration_path, hydration)

        bundles = [
            row for row in final_rows
            if row["storage_adapter"] == "clone_b_bundle"
        ]
        bundle_sha = MODULE.sha256_bytes(MODULE.canonical_jsonl(bundles))
        bundle_verdict = OBJECTS.finalize_result(bundle_sha, 7, [])
        bundle_verdict_path = root / "bundle-verdict.json"
        self.write_json(bundle_verdict_path, bundle_verdict)

        sql = {
            "schema_version": 1,
            "status": "PASS",
            "mapping_sha256": mapping_sha,
            "mapping_row_hash": mapping_row_hash,
            "database": "ab_20260723_b",
            "transaction": "consistent_read_only",
            "task_id": HISTORICAL.TASK_ID,
            "missing_task_asset_id": HISTORICAL.TASK_ASSET_ID,
            "working_reference_count": 0,
            "finalized_reference_count": 0,
            "query_sha256": "d" * 64,
        }
        sql["evidence_hash"] = HISTORICAL.self_hash(sql)
        sql_path = root / "sql.json"
        self.write_json(sql_path, sql)
        api = {
            "schema_version": 1,
            "status": "PASS",
            "mapping_sha256": mapping_sha,
            "mapping_row_hash": mapping_row_hash,
            "task_id": HISTORICAL.TASK_ID,
            "task_asset_id": HISTORICAL.TASK_ASSET_ID,
            "method": "GET",
            "request_path": "/v1/task-assets/12323/preview",
            "http_status": 410,
            "error_code": "asset_historically_unavailable",
        }
        api["evidence_hash"] = HISTORICAL.self_hash(api)
        api_path = root / "api.json"
        self.write_json(api_path, api)
        attestation = HISTORICAL.build(
            mapping_path, final_manifest, sql_path, api_path
        )
        exception_path = root / "exception.json"
        self.write_json(exception_path, attestation)
        return {
            "mapping": mapping_path,
            "mapping_sha": mapping_sha,
            "hydration_input": hydration_input,
            "hydration_input_sha": input_sha,
            "hydration_evidence": hydration_path,
            "hydration_checkpoint": checkpoint_path,
            "final_manifest": final_manifest,
            "final_manifest_sha": final_sha,
            "bundle_verdict": bundle_verdict_path,
            "recovery_plan": recovery["plan"],
            "recovery_plan_sha": recovery["plan_sha"],
            "recovery_verdict": recovery["verdict"],
            "recovery_apply": recovery["apply"],
            "recovery_idempotent": recovery["idempotent"],
            "recovery_component": recovery["component"],
            "exception": exception_path,
            "sql": sql_path,
            "api": api_path,
        }

    def adjudicate(self, paths: dict[str, pathlib.Path | str]) -> dict:
        return MODULE.adjudicate(
            mapping_path=paths["mapping"],
            expected_mapping_sha256=paths["mapping_sha"],
            hydration_input_path=paths["hydration_input"],
            expected_hydration_input_sha256=paths["hydration_input_sha"],
            hydration_evidence_path=paths["hydration_evidence"],
            hydration_checkpoint_path=paths["hydration_checkpoint"],
            final_manifest_path=paths["final_manifest"],
            expected_final_manifest_sha256=paths["final_manifest_sha"],
            bundle_verdict_path=paths["bundle_verdict"],
            recovery_plan_path=paths["recovery_plan"],
            expected_recovery_plan_sha256=paths["recovery_plan_sha"],
            recovery_verdict_path=paths["recovery_verdict"],
            recovery_db_apply_path=paths["recovery_apply"],
            recovery_db_idempotent_path=paths["recovery_idempotent"],
            recovery_component_apply_path=paths["recovery_component"],
            exception_path=paths["exception"],
            sql_path=paths["sql"],
            api_path=paths["api"],
        )

    def test_passes_only_when_all_five_evidence_domains_are_bound(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            result = self.adjudicate(paths)
        self.assertEqual("PASS", result["status"])
        self.assertEqual(0, result["violation_count"])
        self.assertEqual(13, result["checked_count"])
        self.assertEqual(2, result["remote_row_count"])
        self.assertEqual(7, result["bundle_row_count"])
        self.assertEqual(3, result["recovery_row_count"])
        self.assertEqual(1, result["exception_count"])
        self.assertRegex(result["evidence_hash"], MODULE.SHA256)

    def test_pass_standard_verdict_matches_existing_g8_contract(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            composition = self.adjudicate(paths)
            verdict = MODULE.object_verdict_from_composition(
                composition, paths["exception"]
            )
        self.assertEqual(MODULE.OBJECT_VERDICT_FIELDS, set(verdict))
        self.assertEqual("PASS", verdict["status"])
        self.assertEqual(paths["final_manifest_sha"], verdict["manifest_sha256"])
        self.assertEqual(13, verdict["checked_count"])
        self.assertEqual(1, verdict["exception_count"])
        self.assertEqual(paths["mapping_sha"], verdict["mapping_sha256"])
        self.assertEqual([], RELEASE.validate_g8(verdict))

    def test_pass_standard_verdict_is_accepted_by_g06_manifest_contract(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            composition = self.adjudicate(paths)
            verdict = MODULE.object_verdict_from_composition(
                composition, paths["exception"]
            )
            verdict_payload = (
                MODULE.canonical_json(verdict) + "\n"
            ).encode("utf-8")
        detail = {
            "derivation_method": "object_verifier",
            "input_sha256": {
                "object_verdict_sha256": MODULE.sha256_bytes(verdict_payload)
            },
            "verdict": "PASS",
        }
        row = {
            "gate_name": "G06",
            "expected_state": "verified",
            "expected_hash": MODULE.sha256_bytes(
                LOADER.canonical_json(detail).encode("utf-8")
            ),
            "detail_json": detail,
        }
        LOADER.validate_pass_row(row, 1)

    def test_blocked_composition_cannot_be_mistaken_for_g8_pass(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            composition = self.adjudicate(paths)
            composition["status"] = "BLOCKED"
            composition["violations"] = [
                {
                    "violation_code": "g06.test_block",
                    "detail": "fixture failure",
                }
            ]
            verdict = MODULE.object_verdict_from_composition(
                composition, paths["exception"]
            )
        self.assertEqual(MODULE.OBJECT_VERDICT_FIELDS, set(verdict))
        self.assertEqual("BLOCKED", verdict["status"])
        self.assertNotEqual([], RELEASE.validate_g8(verdict))

    def test_ledger_binds_exact_standard_verdict_bytes(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            composition = self.adjudicate(paths)
            verdict = MODULE.object_verdict_from_composition(
                composition, paths["exception"]
            )
            ledger = MODULE.bind_standard_verdict(composition, verdict)
        expected_verdict_sha = MODULE.sha256_bytes(
            (MODULE.canonical_json(verdict) + "\n").encode("utf-8")
        )
        self.assertEqual(expected_verdict_sha, ledger["object_verdict_sha256"])
        unsigned = {
            key: value for key, value in ledger.items()
            if key != "evidence_hash"
        }
        self.assertEqual(
            MODULE.sha256_bytes(
                MODULE.canonical_json(unsigned).encode("utf-8")
            ),
            ledger["evidence_hash"],
        )

    def test_ledger_rejects_status_mismatch_with_standard_verdict(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            composition = self.adjudicate(paths)
            verdict = MODULE.object_verdict_from_composition(
                composition, paths["exception"]
            )
            composition["status"] = "BLOCKED"
            with self.assertRaisesRegex(
                MODULE.AdjudicationError,
                "status differs",
            ):
                MODULE.bind_standard_verdict(composition, verdict)

    def test_ordered_write_rejects_verdict_without_existing_ledger(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            ledger_path = root / "ledger.json"
            verdict_path = root / "verdict.json"
            verdict_path.write_bytes(b"verdict\n")
            with self.assertRaises(FileExistsError):
                MODULE.atomic_write_ordered(
                    [
                        (ledger_path, b"ledger\n"),
                        (verdict_path, b"verdict\n"),
                    ]
                )
            self.assertFalse(ledger_path.exists())

    def test_hydration_count_drift_blocks_without_remote_reads(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            evidence = json.loads(
                paths["hydration_evidence"].read_text(encoding="utf-8")
            )
            evidence["read_only_get_count"] = 0
            evidence["evidence_hash"] = MODULE.sha256_bytes(
                MODULE.canonical_json(
                    {
                        key: value
                        for key, value in evidence.items()
                        if key != "evidence_hash"
                    }
                ).encode("utf-8")
            )
            self.write_json(paths["hydration_evidence"], evidence)
            result = self.adjudicate(paths)
        self.assertEqual("BLOCKED", result["status"])
        self.assertEqual(
            "g06.hydration_coverage",
            result["violations"][0]["violation_code"],
        )

    def test_zero_authorization_hash_requires_zero_authorized_retries(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            evidence = json.loads(
                paths["hydration_evidence"].read_text(encoding="utf-8")
            )
            evidence["retried_authorized_failure_target_count"] = 1
            evidence["evidence_hash"] = MODULE.sha256_bytes(
                MODULE.canonical_json(
                    {
                        key: value
                        for key, value in evidence.items()
                        if key != "evidence_hash"
                    }
                ).encode("utf-8")
            )
            self.write_json(paths["hydration_evidence"], evidence)
            result = self.adjudicate(paths)
        self.assertEqual("BLOCKED", result["status"])
        self.assertEqual(
            "g06.hydration_authorization",
            result["violations"][0]["violation_code"],
        )

    def test_nonzero_authorization_hash_requires_authorized_retry(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            evidence = json.loads(
                paths["hydration_evidence"].read_text(encoding="utf-8")
            )
            evidence["failure_retry_authorization_sha256"] = "f" * 64
            evidence["evidence_hash"] = MODULE.sha256_bytes(
                MODULE.canonical_json(
                    {
                        key: value
                        for key, value in evidence.items()
                        if key != "evidence_hash"
                    }
                ).encode("utf-8")
            )
            self.write_json(paths["hydration_evidence"], evidence)
            result = self.adjudicate(paths)
        self.assertEqual("BLOCKED", result["status"])
        self.assertEqual(
            "g06.hydration_authorization",
            result["violations"][0]["violation_code"],
        )

    def test_nonzero_authorization_hash_with_retry_is_accepted(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            evidence = json.loads(
                paths["hydration_evidence"].read_text(encoding="utf-8")
            )
            evidence["retried_authorized_failure_target_count"] = 1
            evidence["failure_retry_authorization_sha256"] = "f" * 64
            evidence["evidence_hash"] = MODULE.sha256_bytes(
                MODULE.canonical_json(
                    {
                        key: value
                        for key, value in evidence.items()
                        if key != "evidence_hash"
                    }
                ).encode("utf-8")
            )
            self.write_json(paths["hydration_evidence"], evidence)
            result = self.adjudicate(paths)
        self.assertEqual("PASS", result["status"])
        self.assertEqual(0, result["violation_count"])

    def test_bundle_verdict_cannot_be_rebound_to_another_manifest(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            verdict = json.loads(
                paths["bundle_verdict"].read_text(encoding="utf-8")
            )
            verdict["manifest_sha256"] = "e" * 64
            verdict["evidence_hash"] = MODULE.sha256_bytes(
                MODULE.canonical_json(
                    {
                        key: value
                        for key, value in verdict.items()
                        if key != "evidence_hash"
                    }
                ).encode("utf-8")
            )
            self.write_json(paths["bundle_verdict"], verdict)
            result = self.adjudicate(paths)
        self.assertEqual("BLOCKED", result["status"])
        self.assertEqual(
            "g06.bundle_verdict",
            result["violations"][0]["violation_code"],
        )

    def test_exact_sql_evidence_semantics_are_revalidated(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            sql = json.loads(paths["sql"].read_text(encoding="utf-8"))
            sql["working_reference_count"] = 1
            sql["evidence_hash"] = HISTORICAL.self_hash(sql)
            self.write_json(paths["sql"], sql)
            attestation = json.loads(
                paths["exception"].read_text(encoding="utf-8")
            )
            attestation["sql_evidence_sha256"] = MODULE.sha256_file(paths["sql"])
            attestation["evidence_hash"] = HISTORICAL.self_hash(attestation)
            self.write_json(paths["exception"], attestation)
            result = self.adjudicate(paths)
        self.assertEqual("BLOCKED", result["status"])
        self.assertEqual(
            "g06.historical_exception",
            result["violations"][0]["violation_code"],
        )

    def test_exact_api_evidence_semantics_are_revalidated(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            api = json.loads(paths["api"].read_text(encoding="utf-8"))
            api["http_status"] = 200
            api["error_code"] = ""
            api["evidence_hash"] = HISTORICAL.self_hash(api)
            self.write_json(paths["api"], api)
            attestation = json.loads(
                paths["exception"].read_text(encoding="utf-8")
            )
            attestation["api_evidence_sha256"] = MODULE.sha256_file(paths["api"])
            attestation["evidence_hash"] = HISTORICAL.self_hash(attestation)
            self.write_json(paths["exception"], attestation)
            result = self.adjudicate(paths)
        self.assertEqual("BLOCKED", result["status"])
        self.assertEqual(
            "g06.historical_exception",
            result["violations"][0]["violation_code"],
        )

    def test_extract_bundles_is_exactly_the_verifier_subset(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            extracted = MODULE.extract_bundles(
                paths["final_manifest"], paths["final_manifest_sha"]
            )
            verdict = json.loads(
                paths["bundle_verdict"].read_text(encoding="utf-8")
            )
        self.assertEqual(
            verdict["manifest_sha256"], MODULE.sha256_bytes(extracted)
        )

    def test_rebased_checkpoint_hash_coverage_and_metadata_are_bound(self):
        for mode, expected_code in (
            ("hash", "g06.checkpoint_hash"),
            ("coverage", "g06.checkpoint_coverage"),
            ("metadata", "g06.checkpoint_metadata"),
        ):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as raw:
                paths = self.fixture(pathlib.Path(raw))
                checkpoint = json.loads(
                    paths["hydration_checkpoint"].read_text(encoding="utf-8")
                )
                if mode == "hash":
                    checkpoint["adapter_fingerprints"]["upload"] = "f" * 64
                elif mode == "coverage":
                    checkpoint["completed"] = []
                else:
                    checkpoint["completed"][0]["sha256"] = "f" * 64
                self.write_json(paths["hydration_checkpoint"], checkpoint)
                if mode != "hash":
                    evidence = json.loads(
                        paths["hydration_evidence"].read_text(encoding="utf-8")
                    )
                    evidence["checkpoint_sha256"] = MODULE.sha256_file(
                        paths["hydration_checkpoint"]
                    )
                    evidence["evidence_hash"] = MODULE.sha256_bytes(
                        MODULE.canonical_json(
                            {
                                key: value
                                for key, value in evidence.items()
                                if key != "evidence_hash"
                            }
                        ).encode("utf-8")
                    )
                    self.write_json(paths["hydration_evidence"], evidence)
                result = self.adjudicate(paths)
            self.assertEqual("BLOCKED", result["status"])
            self.assertEqual(
                expected_code, result["violations"][0]["violation_code"]
            )

    def test_recovery_rows_and_verdict_are_independently_bound(self):
        for mode, expected_code in (
            ("row", "g06.recovery_rows"),
            ("verdict", "g06.recovery_verdict"),
        ):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as raw:
                paths = self.fixture(pathlib.Path(raw))
                if mode == "row":
                    rows = [
                        json.loads(line)
                        for line in paths["final_manifest"].read_text(
                            encoding="utf-8"
                        ).splitlines()
                    ]
                    recovery = next(
                        row for row in rows
                        if row["storage_adapter"]
                        == RECOVERY.FINAL_STORAGE_ADAPTER
                    )
                    recovery["storage_ref_id"] = "drifted-ref"
                    paths["final_manifest"].write_bytes(
                        MODULE.canonical_jsonl(rows)
                    )
                    paths["final_manifest_sha"] = MODULE.sha256_file(
                        paths["final_manifest"]
                    )
                else:
                    verdict = json.loads(
                        paths["recovery_verdict"].read_text(encoding="utf-8")
                    )
                    verdict["checked_count"] = 2
                    verdict["evidence_hash"] = MODULE.sha256_bytes(
                        MODULE.canonical_json(
                            {
                                key: value
                                for key, value in verdict.items()
                                if key != "evidence_hash"
                            }
                        ).encode("utf-8")
                    )
                    self.write_json(paths["recovery_verdict"], verdict)
                result = self.adjudicate(paths)
            self.assertEqual("BLOCKED", result["status"])
            self.assertEqual(
                expected_code, result["violations"][0]["violation_code"]
            )

    def test_recovery_receipt_commit_drift_blocks_composition(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            receipt = json.loads(
                paths["recovery_apply"].read_text(encoding="utf-8")
            )
            receipt["database_transaction_committed"] = False
            self.write_json(paths["recovery_apply"], receipt)
            result = self.adjudicate(paths)
        self.assertEqual("BLOCKED", result["status"])
        self.assertEqual(
            "g06.contract_error",
            result["violations"][0]["violation_code"],
        )

    def test_frozen_composition_requires_29046_plus_3_plus_7_plus_1(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            result = MODULE.adjudicate(
                mapping_path=paths["mapping"],
                expected_mapping_sha256=paths["mapping_sha"],
                hydration_input_path=paths["hydration_input"],
                expected_hydration_input_sha256=paths["hydration_input_sha"],
                hydration_evidence_path=paths["hydration_evidence"],
                hydration_checkpoint_path=paths["hydration_checkpoint"],
                final_manifest_path=paths["final_manifest"],
                expected_final_manifest_sha256=paths["final_manifest_sha"],
                bundle_verdict_path=paths["bundle_verdict"],
                recovery_plan_path=paths["recovery_plan"],
                expected_recovery_plan_sha256=paths["recovery_plan_sha"],
                recovery_verdict_path=paths["recovery_verdict"],
                recovery_db_apply_path=paths["recovery_apply"],
                recovery_db_idempotent_path=paths["recovery_idempotent"],
                recovery_component_apply_path=paths["recovery_component"],
                exception_path=paths["exception"],
                sql_path=paths["sql"],
                api_path=paths["api"],
                require_frozen=True,
            )
        self.assertEqual("BLOCKED", result["status"])
        self.assertEqual(
            "g06.frozen_composition_count",
            result["violations"][0]["violation_code"],
        )


if __name__ == "__main__":
    unittest.main()
