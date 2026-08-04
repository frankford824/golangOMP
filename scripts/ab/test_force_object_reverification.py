from __future__ import annotations

import hashlib
import importlib.util
import json
import pathlib
import sys
import tempfile
import unittest

from scripts.ab import historical_unavailable_exception as EXCEPTION


ROOT = pathlib.Path(__file__).parent


def load_module(name: str):
    path = ROOT / f"{name}.py"
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    assert spec and spec.loader
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


PREPARE = load_module("prepare_force_reverify_manifest")
VERIFY = load_module("verify_force_rehydration")


def row(
    owner_id: int,
    *,
    object_key: str | None = None,
    placeholder: bool = False,
) -> dict:
    body = f"bytes-{owner_id}".encode()
    return {
        "entity_key": f"task_asset:{owner_id}",
        "owner_kind": "task_asset",
        "owner_id": owner_id,
        "task_id": 42,
        "storage_ref_id": f"ref-{owner_id}",
        "storage_adapter": "upload_service",
        "object_key": object_key or f"task/{owner_id}.bin",
        "size": len(body),
        "mime_type": "application/octet-stream",
        "sha256": hashlib.sha256(body).hexdigest(),
        "status": "active",
        "is_placeholder": placeholder,
    }


def write_jsonl(path: pathlib.Path, rows: list[dict]) -> None:
    path.write_text(
        "".join(PREPARE.canonical_json(item) + "\n" for item in rows),
        encoding="utf-8",
    )


def hydration_evidence(
    force: pathlib.Path,
    hydrated: pathlib.Path,
    *,
    row_count: int,
    unique_targets: int,
    get_count: int | None = None,
    resumed_count: int = 0,
    exception_count: int = 0,
    exception_path: pathlib.Path | None = None,
) -> dict:
    available_count = row_count - exception_count
    value = {
        "schema_version": 1,
        "status": "PASS",
        "input_manifest_sha256": PREPARE.sha256_file(force),
        "hydrated_manifest_sha256": PREPARE.sha256_file(hydrated),
        "checkpoint_sha256": hashlib.sha256(b"checkpoint").hexdigest(),
        "row_count": row_count,
        "already_complete_count": 0,
        "missing_sha256_count": row_count,
        "configured_target_row_count": available_count,
        "unique_target_count": unique_targets,
        "resumed_target_count": resumed_count,
        "resumed_failure_target_count": 0,
        "read_only_get_count": (
            unique_targets if get_count is None else get_count
        ),
        "hydrated_row_count": available_count,
        "deduplicated_get_count": available_count - unique_targets,
        "failure_count": 0,
        "failures": [],
    }
    if exception_path is not None:
        attestation, _exception, attestation_sha = EXCEPTION.load_attestation(
            exception_path
        )
        value.update(
            {
                "retried_transient_failure_target_count": 0,
                "retried_authorized_failure_target_count": 0,
                "failure_retry_authorization_sha256": VERIFY.ZERO_SHA256,
                "historical_unavailable_exception_attestation_sha256": (
                    attestation_sha
                ),
                "historical_unavailable_exception_mapping_sha256": (
                    attestation["mapping_sha256"]
                ),
                "historical_unavailable_exception_mapping_row_hash": (
                    attestation["mapping_row_hash"]
                ),
                "historical_unavailable_exception_count": exception_count,
            }
        )
    value["evidence_hash"] = hashlib.sha256(
        VERIFY.canonical_json(value).encode("utf-8")
    ).hexdigest()
    return value


class ForceObjectReverificationTest(unittest.TestCase):
    def make_pass_documents(self, root: pathlib.Path):
        reviewed = root / "reviewed.jsonl"
        force = root / "force.jsonl"
        hydrated = root / "hydrated.jsonl"
        evidence = root / "hydration.json"
        rows = [
            row(1, object_key="shared/content.bin"),
            row(2, object_key="shared/content.bin"),
            row(3),
        ]
        write_jsonl(reviewed, rows)
        prepared = PREPARE.prepare(reviewed, force)
        self.assertEqual("PASS", prepared["status"])
        write_jsonl(hydrated, rows)
        payload = hydration_evidence(
            force, hydrated, row_count=3, unique_targets=2
        )
        evidence.write_text(
            VERIFY.canonical_json(payload) + "\n", encoding="utf-8"
        )
        return reviewed, force, hydrated, evidence, rows

    def make_exception_documents(self, root: pathlib.Path):
        reviewed = root / "reviewed.jsonl"
        force = root / "force.jsonl"
        hydrated = root / "hydrated.jsonl"
        evidence = root / "hydration.json"
        exception_row = row(EXCEPTION.TASK_ASSET_ID)
        exception_row.update(
            {
                "entity_key": EXCEPTION.ENTITY_KEY,
                "owner_id": EXCEPTION.TASK_ASSET_ID,
                "task_id": EXCEPTION.TASK_ID,
                "storage_ref_id": EXCEPTION.EXPECTED_STORAGE_REF_ID,
                "storage_adapter": EXCEPTION.EXPECTED_STORAGE_ADAPTER,
                "object_key": EXCEPTION.EXPECTED_OBJECT_KEY,
                "size": EXCEPTION.EXPECTED_SIZE,
                "mime_type": EXCEPTION.EXPECTED_MIME_TYPE,
                "sha256": "",
                "status": EXCEPTION.EXPECTED_STATUS,
            }
        )
        rows = [exception_row, row(3)]
        write_jsonl(reviewed, rows)

        mapping_row = {
            "task_id": EXCEPTION.TASK_ID,
            "missing_task_asset_id": EXCEPTION.TASK_ASSET_ID,
            "strategy": EXCEPTION.STRATEGY,
            "review_policy_ids": [EXCEPTION.POLICY_ID],
            "confidence": "confirmed_auto",
            "confirmed_by": 1,
            "confirmed_at": "2026-07-23T12:00:00Z",
            "confirmation_note": "confirmed historical tombstone",
            "recovery_source_task_asset_id": 0,
            "original_storage_ref_id": EXCEPTION.EXPECTED_STORAGE_REF_ID,
            "expected_file_size": EXCEPTION.EXPECTED_SIZE,
            "object_probe_result": EXCEPTION.EXPECTED_PROBE_RESULT,
            "object_probe_read_only_get_count": (
                EXCEPTION.EXPECTED_PROBE_READ_ONLY_GET_COUNT
            ),
            "object_probe_evidence_hash": EXCEPTION.EXPECTED_PROBE_EVIDENCE_HASH,
            "object_probe_input_manifest_sha256": (
                EXCEPTION.EXPECTED_PROBE_INPUT_MANIFEST_SHA256
            ),
            "object_probe_object_key_sha256": (
                EXCEPTION.EXPECTED_PROBE_OBJECT_KEY_SHA256
            ),
            "blockers": [],
        }
        mapping_row["manifest_row_hash"] = EXCEPTION.canonical_hash(mapping_row)
        mapping = {"version": 2, "asset_recoveries": [mapping_row]}
        mapping_path = root / "mapping.json"
        mapping_path.write_text(EXCEPTION.canonical_json(mapping) + "\n", encoding="utf-8")
        mapping_sha = EXCEPTION.sha256_file(mapping_path)
        sql = {
            "schema_version": 1,
            "status": "PASS",
            "mapping_sha256": mapping_sha,
            "mapping_row_hash": mapping_row["manifest_row_hash"],
            "database": "ab_test_b",
            "transaction": "consistent_read_only",
            "task_id": EXCEPTION.TASK_ID,
            "missing_task_asset_id": EXCEPTION.TASK_ASSET_ID,
            "working_reference_count": 0,
            "finalized_reference_count": 0,
            "query_sha256": "2" * 64,
        }
        sql["evidence_hash"] = EXCEPTION.self_hash(sql)
        sql_path = root / "sql.json"
        sql_path.write_text(EXCEPTION.canonical_json(sql) + "\n", encoding="utf-8")
        api = {
            "schema_version": 1,
            "status": "PASS",
            "mapping_sha256": mapping_sha,
            "mapping_row_hash": mapping_row["manifest_row_hash"],
            "task_id": EXCEPTION.TASK_ID,
            "task_asset_id": EXCEPTION.TASK_ASSET_ID,
            "method": "GET",
            "request_path": "/v1/task-assets/12323/preview",
            "http_status": 410,
            "error_code": "asset_historically_unavailable",
        }
        api["evidence_hash"] = EXCEPTION.self_hash(api)
        api_path = root / "api.json"
        api_path.write_text(EXCEPTION.canonical_json(api) + "\n", encoding="utf-8")
        exception_path = root / "exception.json"
        exception_path.write_text(
            EXCEPTION.canonical_json(
                EXCEPTION.build(mapping_path, reviewed, sql_path, api_path)
            )
            + "\n",
            encoding="utf-8",
        )
        prepared = PREPARE.prepare(reviewed, force, exception_path)
        self.assertEqual("PASS", prepared["status"])
        self.assertEqual(1, prepared["exception_count"])
        write_jsonl(hydrated, rows)
        payload = hydration_evidence(
            force,
            hydrated,
            row_count=2,
            unique_targets=1,
            exception_count=1,
            exception_path=exception_path,
        )
        evidence.write_text(
            VERIFY.canonical_json(payload) + "\n", encoding="utf-8"
        )
        return reviewed, force, hydrated, evidence, exception_path, rows

    def test_pass_requires_every_unique_object_to_be_fetched(self):
        with tempfile.TemporaryDirectory() as raw:
            paths = self.make_pass_documents(pathlib.Path(raw))
            result = VERIFY.verify(*paths[:4])
            self.assertEqual("PASS", result["status"])
            self.assertEqual(3, result["checked_count"])
            self.assertEqual(0, result["violation_count"])
            unsigned = {
                key: value
                for key, value in result.items()
                if key != "evidence_hash"
            }
            self.assertEqual(
                result["evidence_hash"],
                hashlib.sha256(
                    VERIFY.canonical_json(unsigned).encode("utf-8")
                ).hexdigest(),
            )

    def test_current_zero_exception_fields_keep_no_exception_evidence_compatible(self):
        with tempfile.TemporaryDirectory() as raw:
            reviewed, force, hydrated, evidence, _rows = (
                self.make_pass_documents(pathlib.Path(raw))
            )
            payload = json.loads(evidence.read_text(encoding="utf-8"))
            payload.update(
                {
                    "retried_transient_failure_target_count": 0,
                    "retried_authorized_failure_target_count": 0,
                    "failure_retry_authorization_sha256": VERIFY.ZERO_SHA256,
                    "historical_unavailable_exception_attestation_sha256": (
                        VERIFY.ZERO_SHA256
                    ),
                    "historical_unavailable_exception_mapping_sha256": (
                        VERIFY.ZERO_SHA256
                    ),
                    "historical_unavailable_exception_mapping_row_hash": (
                        VERIFY.ZERO_SHA256
                    ),
                    "historical_unavailable_exception_count": 0,
                }
            )
            payload["evidence_hash"] = hashlib.sha256(
                VERIFY.canonical_json(
                    {
                        key: value
                        for key, value in payload.items()
                        if key != "evidence_hash"
                    }
                ).encode("utf-8")
            ).hexdigest()
            evidence.write_text(
                VERIFY.canonical_json(payload) + "\n", encoding="utf-8"
            )
            result = VERIFY.verify(reviewed, force, hydrated, evidence)
        self.assertEqual("PASS", result["status"])

    def test_hydrated_row_tampering_blocks(self):
        with tempfile.TemporaryDirectory() as raw:
            reviewed, force, hydrated, evidence, rows = (
                self.make_pass_documents(pathlib.Path(raw))
            )
            rows[1]["mime_type"] = "image/png"
            write_jsonl(hydrated, rows)
            payload = hydration_evidence(
                force, hydrated, row_count=3, unique_targets=2
            )
            evidence.write_text(
                VERIFY.canonical_json(payload) + "\n", encoding="utf-8"
            )
            result = VERIFY.verify(reviewed, force, hydrated, evidence)
            self.assertEqual("BLOCKED", result["status"])
            self.assertEqual(
                "force_reverify.hydrated_manifest_mismatch",
                result["violations"][0]["violation_code"],
            )

    def test_fewer_gets_than_unique_objects_blocks(self):
        with tempfile.TemporaryDirectory() as raw:
            reviewed, force, hydrated, evidence, _rows = (
                self.make_pass_documents(pathlib.Path(raw))
            )
            payload = hydration_evidence(
                force,
                hydrated,
                row_count=3,
                unique_targets=2,
                get_count=1,
            )
            evidence.write_text(
                VERIFY.canonical_json(payload) + "\n", encoding="utf-8"
            )
            result = VERIFY.verify(reviewed, force, hydrated, evidence)
            self.assertEqual("BLOCKED", result["status"])
            self.assertEqual(
                "force_reverify.hydration_count_mismatch",
                result["violations"][0]["violation_code"],
            )

    def test_checkpoint_resumed_gets_count_as_reverification(self):
        with tempfile.TemporaryDirectory() as raw:
            reviewed, force, hydrated, evidence, _rows = (
                self.make_pass_documents(pathlib.Path(raw))
            )
            payload = hydration_evidence(
                force,
                hydrated,
                row_count=3,
                unique_targets=2,
                get_count=1,
                resumed_count=1,
            )
            evidence.write_text(
                VERIFY.canonical_json(payload) + "\n", encoding="utf-8"
            )
            result = VERIFY.verify(reviewed, force, hydrated, evidence)
            self.assertEqual("PASS", result["status"])
            self.assertEqual(3, result["checked_count"])

    def test_duplicate_entity_blocks_without_writing_force_manifest(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            reviewed = root / "reviewed.jsonl"
            force = root / "force.jsonl"
            duplicate = row(1)
            write_jsonl(reviewed, [duplicate, duplicate])
            with self.assertRaises(PREPARE.ManifestError) as caught:
                PREPARE.prepare(reviewed, force)
            self.assertEqual("force_reverify.duplicate_entity", caught.exception.code)
            self.assertFalse(force.exists())

    def test_placeholder_blocks_without_writing_force_manifest(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            reviewed = root / "reviewed.jsonl"
            force = root / "force.jsonl"
            write_jsonl(reviewed, [row(1, placeholder=True)])
            with self.assertRaises(PREPARE.ManifestError) as caught:
                PREPARE.prepare(reviewed, force)
            self.assertEqual("force_reverify.placeholder", caught.exception.code)
            self.assertFalse(force.exists())

    def test_force_manifest_tampering_blocks(self):
        with tempfile.TemporaryDirectory() as raw:
            reviewed, force, hydrated, evidence, rows = (
                self.make_pass_documents(pathlib.Path(raw))
            )
            forced_rows = [dict(item, sha256="") for item in rows]
            forced_rows[0]["size"] += 1
            write_jsonl(force, forced_rows)
            payload = hydration_evidence(
                force, hydrated, row_count=3, unique_targets=2
            )
            evidence.write_text(
                VERIFY.canonical_json(payload) + "\n", encoding="utf-8"
            )
            result = VERIFY.verify(reviewed, force, hydrated, evidence)
            self.assertEqual("BLOCKED", result["status"])
            self.assertEqual(
                "force_reverify.force_manifest_tampered",
                result["violations"][0]["violation_code"],
            )

    def test_exact_exception_is_preserved_and_consumed_transparently(self):
        with tempfile.TemporaryDirectory() as raw:
            reviewed, force, hydrated, evidence, exception, rows = (
                self.make_exception_documents(pathlib.Path(raw))
            )
            forced_rows = [
                json.loads(line)
                for line in force.read_text(encoding="utf-8").splitlines()
            ]
            result = VERIFY.verify(
                reviewed, force, hydrated, evidence, exception
            )
        self.assertEqual(rows[0]["sha256"], forced_rows[0]["sha256"])
        self.assertEqual("", forced_rows[1]["sha256"])
        self.assertEqual("PASS", result["status"])
        self.assertEqual(2, result["checked_count"])
        self.assertEqual(1, result["exception_count"])
        self.assertEqual(EXCEPTION.ENTITY_KEY, result["exceptions"][0]["entity_key"])

    def test_inventing_exception_fingerprint_blocks(self):
        with tempfile.TemporaryDirectory() as raw:
            reviewed, force, hydrated, evidence, exception, _rows = (
                self.make_exception_documents(pathlib.Path(raw))
            )
            forced_rows = [
                json.loads(line)
                for line in force.read_text(encoding="utf-8").splitlines()
            ]
            forced_rows[0]["sha256"] = "6" * 64
            write_jsonl(force, forced_rows)
            result = VERIFY.verify(
                reviewed, force, hydrated, evidence, exception
            )
        self.assertEqual("BLOCKED", result["status"])
        self.assertEqual(
            "force_reverify.force_invalid",
            result["violations"][0]["violation_code"],
        )

    def test_tampered_exception_attestation_blocks(self):
        with tempfile.TemporaryDirectory() as raw:
            reviewed, force, hydrated, evidence, exception, _rows = (
                self.make_exception_documents(pathlib.Path(raw))
            )
            payload = json.loads(exception.read_text(encoding="utf-8"))
            payload["mapping_sha256"] = "9" * 64
            exception.write_text(
                VERIFY.canonical_json(payload) + "\n", encoding="utf-8"
            )
            result = VERIFY.verify(
                reviewed, force, hydrated, evidence, exception
            )
        self.assertEqual("BLOCKED", result["status"])
        self.assertEqual(
            "force_reverify.exception_invalid",
            result["violations"][0]["violation_code"],
        )

    def test_hydration_exception_binding_must_match_exact_attestation(self):
        with tempfile.TemporaryDirectory() as raw:
            reviewed, force, hydrated, evidence, exception, _rows = (
                self.make_exception_documents(pathlib.Path(raw))
            )
            payload = json.loads(evidence.read_text(encoding="utf-8"))
            payload[
                "historical_unavailable_exception_mapping_row_hash"
            ] = "9" * 64
            payload["evidence_hash"] = hashlib.sha256(
                VERIFY.canonical_json(
                    {
                        key: value
                        for key, value in payload.items()
                        if key != "evidence_hash"
                    }
                ).encode("utf-8")
            ).hexdigest()
            evidence.write_text(
                VERIFY.canonical_json(payload) + "\n", encoding="utf-8"
            )
            result = VERIFY.verify(
                reviewed, force, hydrated, evidence, exception
            )
        self.assertEqual("BLOCKED", result["status"])
        self.assertEqual(
            "force_reverify.hydration_exception_binding",
            result["violations"][0]["violation_code"],
        )

    def test_legacy_hydration_evidence_cannot_authorize_an_exception(self):
        with tempfile.TemporaryDirectory() as raw:
            reviewed, force, hydrated, evidence, exception, _rows = (
                self.make_exception_documents(pathlib.Path(raw))
            )
            payload = json.loads(evidence.read_text(encoding="utf-8"))
            for field in (
                VERIFY.HYDRATION_RETRY_FIELDS
                | VERIFY.HYDRATION_EXCEPTION_FIELDS
            ):
                payload.pop(field)
            payload["evidence_hash"] = hashlib.sha256(
                VERIFY.canonical_json(
                    {
                        key: value
                        for key, value in payload.items()
                        if key != "evidence_hash"
                    }
                ).encode("utf-8")
            ).hexdigest()
            evidence.write_text(
                VERIFY.canonical_json(payload) + "\n", encoding="utf-8"
            )
            result = VERIFY.verify(
                reviewed, force, hydrated, evidence, exception
            )
        self.assertEqual("BLOCKED", result["status"])
        self.assertEqual(
            "force_reverify.hydration_exception_binding",
            result["violations"][0]["violation_code"],
        )

    def test_prepare_never_overwrites_exception_attestation(self):
        with tempfile.TemporaryDirectory() as raw:
            reviewed, _force, _hydrated, _evidence, exception, _rows = (
                self.make_exception_documents(pathlib.Path(raw))
            )
            original = exception.read_bytes()
            with self.assertRaises(PREPARE.ManifestError) as caught:
                PREPARE.prepare(reviewed, exception, exception)
            self.assertEqual("force_reverify.path_collision", caught.exception.code)
            self.assertEqual(original, exception.read_bytes())


if __name__ == "__main__":
    unittest.main()
