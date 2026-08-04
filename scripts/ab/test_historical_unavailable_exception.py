from __future__ import annotations

import json
import pathlib
import tempfile
import unittest

from scripts.ab import historical_unavailable_exception as MODULE


class HistoricalUnavailableExceptionTest(unittest.TestCase):
    def write_json(self, path: pathlib.Path, value: dict) -> None:
        path.write_text(MODULE.canonical_json(value) + "\n", encoding="utf-8")

    def fixture(self, root: pathlib.Path) -> dict[str, pathlib.Path]:
        mapping_row = {
            "task_id": 2199,
            "missing_task_asset_id": 12323,
            "strategy": MODULE.STRATEGY,
            "review_policy_ids": [MODULE.POLICY_ID],
            "confidence": "confirmed_auto",
            "confirmed_by": 1,
            "confirmed_at": "2026-07-23T12:00:00Z",
            "confirmation_note": "admin confirmed historical tombstone",
            "recovery_source_task_asset_id": 0,
            "original_storage_ref_id": MODULE.EXPECTED_STORAGE_REF_ID,
            "expected_file_size": MODULE.EXPECTED_SIZE,
            "object_probe_result": MODULE.EXPECTED_PROBE_RESULT,
            "object_probe_read_only_get_count": (
                MODULE.EXPECTED_PROBE_READ_ONLY_GET_COUNT
            ),
            "object_probe_evidence_hash": MODULE.EXPECTED_PROBE_EVIDENCE_HASH,
            "object_probe_input_manifest_sha256": (
                MODULE.EXPECTED_PROBE_INPUT_MANIFEST_SHA256
            ),
            "object_probe_object_key_sha256": (
                MODULE.EXPECTED_PROBE_OBJECT_KEY_SHA256
            ),
            "blockers": [],
        }
        mapping_row["manifest_row_hash"] = MODULE.canonical_hash(mapping_row)
        mapping = {"version": 2, "asset_recoveries": [mapping_row]}
        mapping_path = root / "mapping.json"
        self.write_json(mapping_path, mapping)

        object_row = {
            "entity_key": MODULE.ENTITY_KEY,
            "owner_kind": "task_asset",
            "owner_id": MODULE.TASK_ASSET_ID,
            "task_id": MODULE.TASK_ID,
            "storage_ref_id": MODULE.EXPECTED_STORAGE_REF_ID,
            "storage_adapter": MODULE.EXPECTED_STORAGE_ADAPTER,
            "object_key": MODULE.EXPECTED_OBJECT_KEY,
            "size": MODULE.EXPECTED_SIZE,
            "mime_type": MODULE.EXPECTED_MIME_TYPE,
            "sha256": "",
            "status": MODULE.EXPECTED_STATUS,
            "is_placeholder": False,
        }
        manifest_path = root / "objects.jsonl"
        manifest_path.write_text(
            MODULE.canonical_json(object_row) + "\n", encoding="utf-8"
        )

        mapping_sha = MODULE.sha256_file(mapping_path)
        row_hash = mapping_row["manifest_row_hash"]
        sql = {
            "schema_version": 1,
            "status": "PASS",
            "mapping_sha256": mapping_sha,
            "mapping_row_hash": row_hash,
            "database": "ab_20260723_b",
            "transaction": "consistent_read_only",
            "task_id": MODULE.TASK_ID,
            "missing_task_asset_id": MODULE.TASK_ASSET_ID,
            "working_reference_count": 0,
            "finalized_reference_count": 0,
            "query_sha256": "2" * 64,
        }
        sql["evidence_hash"] = MODULE.self_hash(sql)
        sql_path = root / "sql.json"
        self.write_json(sql_path, sql)

        api = {
            "schema_version": 1,
            "status": "PASS",
            "mapping_sha256": mapping_sha,
            "mapping_row_hash": row_hash,
            "task_id": MODULE.TASK_ID,
            "task_asset_id": MODULE.TASK_ASSET_ID,
            "method": "GET",
            "request_path": "/v1/task-assets/12323/preview",
            "http_status": 410,
            "error_code": "asset_historically_unavailable",
        }
        api["evidence_hash"] = MODULE.self_hash(api)
        api_path = root / "api.json"
        self.write_json(api_path, api)
        return {
            "mapping": mapping_path,
            "manifest": manifest_path,
            "sql": sql_path,
            "api": api_path,
        }

    def build(self, paths: dict[str, pathlib.Path]) -> dict:
        return MODULE.build(
            paths["mapping"],
            paths["manifest"],
            paths["sql"],
            paths["api"],
        )

    def test_builds_exact_single_hash_bound_exception(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            result = self.build(paths)
            output = pathlib.Path(raw) / "exception.json"
            self.write_json(output, result)
            attestation, exception, artifact_sha = MODULE.load_attestation(
                output, manifest_path=paths["manifest"]
            )
        self.assertEqual(result, attestation)
        self.assertEqual(1, result["exception_count"])
        self.assertEqual(MODULE.ENTITY_KEY, exception["entity_key"])
        self.assertEqual(result["mapping_row_hash"], exception["mapping_row_hash"])
        self.assertRegex(artifact_sha, MODULE.SHA256)

    def test_proposed_review_mapping_is_rejected_even_with_fresh_row_hash(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            mapping = json.loads(paths["mapping"].read_text(encoding="utf-8"))
            row = mapping["asset_recoveries"][0]
            row["confidence"] = "proposed_review"
            row["manifest_row_hash"] = MODULE.canonical_hash(
                {key: value for key, value in row.items() if key != "manifest_row_hash"}
            )
            self.write_json(paths["mapping"], mapping)
            with self.assertRaisesRegex(
                MODULE.ExceptionContractError, "not final-reviewed"
            ):
                self.build(paths)

    def test_wrong_size_or_probe_binding_is_rejected(self):
        for field, value in (
            ("expected_file_size", MODULE.EXPECTED_SIZE - 1),
            ("object_probe_result", "unknown"),
            ("object_probe_read_only_get_count", 0),
            ("object_probe_evidence_hash", "9" * 64),
            ("object_probe_input_manifest_sha256", "8" * 64),
            ("object_probe_object_key_sha256", "7" * 64),
        ):
            with self.subTest(field=field), tempfile.TemporaryDirectory() as raw:
                paths = self.fixture(pathlib.Path(raw))
                mapping = json.loads(paths["mapping"].read_text(encoding="utf-8"))
                row = mapping["asset_recoveries"][0]
                row[field] = value
                row["manifest_row_hash"] = MODULE.canonical_hash(
                    {
                        key: item
                        for key, item in row.items()
                        if key != "manifest_row_hash"
                    }
                )
                self.write_json(paths["mapping"], mapping)
                with self.assertRaisesRegex(
                    MODULE.ExceptionContractError, "not final-reviewed"
                ):
                    self.build(paths)

    def test_original_object_row_must_keep_exact_blank_digest_contract(self):
        for field, value in (
            ("size", MODULE.EXPECTED_SIZE - 1),
            ("object_key", "tasks/2199/thumbnail.png"),
            ("sha256", "5" * 64),
        ):
            with self.subTest(field=field), tempfile.TemporaryDirectory() as raw:
                paths = self.fixture(pathlib.Path(raw))
                row = json.loads(paths["manifest"].read_text(encoding="utf-8"))
                row[field] = value
                paths["manifest"].write_text(
                    MODULE.canonical_json(row) + "\n", encoding="utf-8"
                )
                with self.assertRaisesRegex(
                    MODULE.ExceptionContractError,
                    "historical-unavailable object contract",
                ):
                    self.build(paths)

    def test_stale_mapping_row_hash_is_rejected(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            mapping = json.loads(paths["mapping"].read_text(encoding="utf-8"))
            mapping["asset_recoveries"][0]["confirmation_note"] = "tampered"
            self.write_json(paths["mapping"], mapping)
            with self.assertRaisesRegex(MODULE.ExceptionContractError, "hash is stale"):
                self.build(paths)

    def test_current_working_or_finalized_reference_is_rejected(self):
        for field in ("working_reference_count", "finalized_reference_count"):
            with self.subTest(field=field), tempfile.TemporaryDirectory() as raw:
                paths = self.fixture(pathlib.Path(raw))
                sql = json.loads(paths["sql"].read_text(encoding="utf-8"))
                sql[field] = 1
                sql["evidence_hash"] = MODULE.self_hash(sql)
                self.write_json(paths["sql"], sql)
                with self.assertRaisesRegex(
                    MODULE.ExceptionContractError, "zero current references"
                ):
                    self.build(paths)

    def test_http_404_is_never_accepted(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            api = json.loads(paths["api"].read_text(encoding="utf-8"))
            api["http_status"] = 404
            api["evidence_hash"] = MODULE.self_hash(api)
            self.write_json(paths["api"], api)
            with self.assertRaisesRegex(MODULE.ExceptionContractError, "HTTP 410"):
                self.build(paths)

    def test_wrong_object_identity_is_rejected(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            row = json.loads(paths["manifest"].read_text(encoding="utf-8"))
            row["task_id"] = 2200
            paths["manifest"].write_text(
                MODULE.canonical_json(row) + "\n", encoding="utf-8"
            )
            with self.assertRaisesRegex(
                MODULE.ExceptionContractError, "object identity differs"
            ):
                self.build(paths)

    def test_empty_digest_is_allowed_only_for_exact_tombstone_contract(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            self.build(paths)
            row = json.loads(paths["manifest"].read_text(encoding="utf-8"))
            row["owner_id"] = 12324
            paths["manifest"].write_text(
                MODULE.canonical_json(row) + "\n", encoding="utf-8"
            )
            with self.assertRaisesRegex(
                MODULE.ExceptionContractError,
                "object identity differs",
            ):
                self.build(paths)

    def test_attestation_tampering_is_rejected(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.fixture(pathlib.Path(raw))
            result = self.build(paths)
            result["exceptions"][0]["observed_http_status"] = 410
            output = pathlib.Path(raw) / "exception.json"
            self.write_json(output, result)
            with self.assertRaisesRegex(
                MODULE.ExceptionContractError, "not a valid PASS"
            ):
                MODULE.load_attestation(output, manifest_path=paths["manifest"])


if __name__ == "__main__":
    unittest.main()
