import hashlib
import importlib.util
import json
import os
import pathlib
import sys
import tempfile
import types
import unittest
from unittest import mock


PATH = pathlib.Path(__file__).with_name(
    "collect_clone_a_recovery_evidence.py"
)
SPEC = importlib.util.spec_from_file_location(
    "collect_clone_a_recovery_evidence", PATH
)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)

PREPARE_PATH = pathlib.Path(__file__).with_name("prepare_asset_recovery.py")
PREPARE_SPEC = importlib.util.spec_from_file_location(
    "prepare_asset_recovery_for_collector_test", PREPARE_PATH
)
PREPARE = importlib.util.module_from_spec(PREPARE_SPEC)
sys.modules[PREPARE_SPEC.name] = PREPARE
PREPARE_SPEC.loader.exec_module(PREPARE)


FAKE_MYSQL = r"""#!/usr/bin/env python3
import json
import os
import pathlib
import sys

sql = sys.stdin.read()
trace = {
    "argv": sys.argv[1:],
    "sql": sql,
    "password_present": bool(os.environ.get("MYSQL_PWD")),
}
pathlib.Path(os.environ["FAKE_MYSQL_TRACE"]).write_text(
    json.dumps(trace), encoding="utf-8"
)
rows = json.loads(pathlib.Path(os.environ["FAKE_MYSQL_ROWS"]).read_text())
for row in rows:
    print(json.dumps(row, separators=(",", ":")))
"""


class CloneARecoveryEvidenceCollectorTest(unittest.TestCase):
    def make_inputs(self, root):
        root = pathlib.Path(root)
        mapping_rows = []
        receipt_rows = []
        rows = [
            {
                "kind": "database",
                "database": "ab_formal_a_ui",
                "transaction_read_only": 1,
            }
        ]
        for missing_id, (task_id, source_id, size) in MODULE.ALLOWLIST.items():
            source_path = root / f"source-{source_id}.bin"
            source_path.write_bytes(bytes([source_id % 251]) * size)
            source_sha = MODULE.sha256_file(source_path)
            preview = hashlib.sha256(f"preview-{missing_id}".encode()).hexdigest()
            thumb = hashlib.sha256(f"thumb-{missing_id}".encode()).hexdigest()
            missing_ref = f"missing-ref-{missing_id}"
            source_ref = f"source-ref-{source_id}"
            row = {
                "task_id": task_id,
                "missing_task_asset_id": missing_id,
                "recovery_source_task_asset_id": source_id,
                "strategy": MODULE.STRATEGY,
                "original_storage_ref_id": missing_ref,
                "recovery_source_storage_ref_id": source_ref,
                "expected_file_size": size,
                "preview_whole_hash": preview,
                "design_thumb_whole_hash": thumb,
                "confidence": "confirmed_auto",
                "review_policy_ids": [MODULE.POLICY],
                "confirmed_by": 1,
                "confirmed_at": "2026-07-23T12:00:00Z",
                "confirmation_note": "exact task 2807 recovery approved",
                "blockers": [],
                "manifest_row_hash": "",
            }
            row["manifest_row_hash"] = hashlib.sha256(
                MODULE.canonical_bytes(row)
            ).hexdigest()
            mapping_rows.append(row)
            object_key = f"surviving/{source_id}.jpg"
            receipt_rows.append(
                {
                    "missing_task_asset_id": missing_id,
                    "task_asset_id": source_id,
                    "source_local_path": str(source_path),
                    "source_sha256": source_sha,
                    "source_fetch_receipt": {
                        "protocol": "controlled-asset-read-v1",
                        "task_asset_id": source_id,
                        "storage_ref_id": source_ref,
                        "object_key": object_key,
                        "size": size,
                        "mime_type": "image/jpeg",
                        "sha256": source_sha,
                        "fetched_at": "2026-07-23T12:00:00Z",
                    },
                }
            )
            common = {
                "whole_hash": None,
                "cleaned_at": None,
                "object_deleted_at": None,
                "access_revoked_at": None,
                "access_revoked_reason": "",
                "mime_type": "image/jpeg",
                "source_asset_version_id": None,
            }
            rows.extend(
                [
                    {
                        "kind": "task_asset",
                        "id": missing_id,
                        "task_id": task_id,
                        "asset_id": missing_id + 1000,
                        "upload_request_id": f"request-{missing_id}",
                        "storage_ref_id": missing_ref,
                        "storage_key": f"deleted/{missing_id}.jpg",
                        "upload_status": "uploaded",
                        "deleted_at": "2026-07-22 00:00:00",
                        "file_size": size,
                        "file_name": f"{missing_id}.jpg",
                        "asset_type": "delivery",
                        **common,
                    },
                    {
                        "kind": "task_asset",
                        "id": source_id,
                        "task_id": MODULE.SOURCE_TASK_ID,
                        "asset_id": source_id + 1000,
                        "upload_request_id": f"request-{source_id}",
                        "storage_ref_id": source_ref,
                        "storage_key": object_key,
                        "upload_status": "uploaded",
                        "deleted_at": None,
                        "file_size": size,
                        "file_name": f"{source_id}.jpg",
                        "asset_type": "delivery",
                        **common,
                    },
                    {
                        "kind": "upload_request",
                        "missing_task_asset_id": missing_id,
                        "request_id": f"request-{missing_id}",
                        "bound_ref_id": missing_ref,
                        "checksum_hint": "",
                        "file_size": size,
                        "status": "bound",
                        "session_status": "completed",
                    },
                    {
                        "kind": "storage_ref",
                        "missing_task_asset_id": missing_id,
                        "ref_id": missing_ref,
                        "asset_id": missing_id + 1000,
                        "owner_type": "task_asset",
                        "owner_id": missing_id,
                        "upload_request_id": f"request-{missing_id}",
                        "storage_adapter": "local",
                        "ref_type": "task_asset_object",
                        "ref_key": f"deleted/{missing_id}.jpg",
                        "file_name": f"{missing_id}.jpg",
                        "mime_type": "image/jpeg",
                        "file_size": size,
                        "is_placeholder": 0,
                        "checksum_hint": "",
                        "status": "recorded",
                    },
                ]
            )
            for parent_id in (missing_id, source_id):
                rows.extend(
                    [
                        {
                            "kind": "derivative",
                            "id": parent_id * 10 + 1,
                            "asset_type": "preview",
                            "source_asset_version_id": parent_id,
                            "whole_hash": preview,
                        },
                        {
                            "kind": "derivative",
                            "id": parent_id * 10 + 2,
                            "asset_type": "design_thumb",
                            "source_asset_version_id": parent_id,
                            "whole_hash": thumb,
                        },
                    ]
                )

        mapping = {"version": 2, "asset_recoveries": mapping_rows}
        mapping_path = root / "mapping.json"
        mapping_path.write_bytes(MODULE.canonical_bytes(mapping) + b"\n")
        receipts = {
            "version": 1,
            "status": "PASS",
            "protocol": "controlled-asset-read-v1",
            "production_writes_executed": False,
            "database_connections_opened": False,
            "remote_operation": "GET",
            "origin_fingerprint_sha256": "a" * 64,
            "recoveries": receipt_rows,
        }
        receipts["evidence_sha256"] = hashlib.sha256(
            MODULE.canonical_bytes(receipts)
        ).hexdigest()
        receipts_path = root / "receipts.json"
        receipts_path.write_bytes(MODULE.canonical_bytes(receipts) + b"\n")
        rows_path = root / "rows.json"
        rows_path.write_text(json.dumps(rows), encoding="utf-8")
        dsn = root / "clone-a.dsn"
        dsn.write_text(
            "reader:top-secret@tcp(127.0.0.1:3307)/ab_formal_a_ui?parseTime=true\n",
            encoding="utf-8",
        )
        fake = root / "fake-mysql.py"
        fake.write_text(FAKE_MYSQL, encoding="utf-8")
        fake.chmod(0o700)
        trace = root / "trace.json"
        args = types.SimpleNamespace(
            run_id="formal-test",
            clone_a_dsn_file=dsn,
            confirm_clone_a_database="ab_formal_a_ui",
            mapping=mapping_path,
            controlled_read_receipts=receipts_path,
            output=root / "evidence.json",
            mysql_bin=str(fake),
            timeout_seconds=30,
        )
        return args, rows_path, trace

    def test_collects_prepare_compatible_exact_evidence_read_only(self):
        with tempfile.TemporaryDirectory() as raw:
            args, rows_path, trace = self.make_inputs(raw)
            with mock.patch.dict(
                os.environ,
                {
                    "FAKE_MYSQL_ROWS": str(rows_path),
                    "FAKE_MYSQL_TRACE": str(trace),
                },
            ):
                evidence = MODULE.run(args)
            self.assertEqual(evidence["status"], "PASS")
            self.assertEqual(len(evidence["recoveries"]), 3)
            unsigned = dict(evidence)
            evidence_sha = unsigned.pop("evidence_sha256")
            self.assertEqual(
                evidence_sha,
                hashlib.sha256(MODULE.canonical_bytes(unsigned)).hexdigest(),
            )
            self.assertNotIn(
                "top-secret", args.output.read_text(encoding="utf-8")
            )
            query = json.loads(trace.read_text(encoding="utf-8"))
            self.assertTrue(query["password_present"])
            self.assertIn(
                "START TRANSACTION WITH CONSISTENT SNAPSHOT, READ ONLY",
                query["sql"],
            )
            self.assertIn("SET SESSION TRANSACTION READ ONLY", query["sql"])
            self.assertIn("@@session.transaction_read_only", query["sql"])
            self.assertIn("COMMIT", query["sql"])
            self.assertNotIn("SELECT *", query["sql"].upper())
            self.assertNotRegex(
                query["sql"],
                r"(?im)^\s*(INSERT|UPDATE|DELETE|REPLACE|ALTER|CREATE|DROP|TRUNCATE)\b",
            )
            prepare_output = pathlib.Path(raw) / "prepare-plan.json"
            prepare_args = types.SimpleNamespace(
                mapping=args.mapping,
                evidence=args.output,
                output=prepare_output,
                materialize=False,
                fixture_root=None,
            )
            plan = PREPARE.run(prepare_args)
            self.assertEqual(plan["status"], "PREPARED")
            self.assertFalse(plan["database_writes_executed"])

    def test_non_loopback_or_wrong_side_dsn_fails_before_query(self):
        with tempfile.TemporaryDirectory() as raw:
            args, _, trace = self.make_inputs(raw)
            args.clone_a_dsn_file.write_text(
                "reader:secret@tcp(prod.example.com:3306)/workflow\n",
                encoding="utf-8",
            )
            with self.assertRaisesRegex(ValueError, "loopback"):
                MODULE.run(args)
            self.assertFalse(trace.exists())

    def test_allowlist_or_receipt_hash_drift_fails_before_query(self):
        with tempfile.TemporaryDirectory() as raw:
            args, _, trace = self.make_inputs(raw)
            mapping = json.loads(args.mapping.read_text(encoding="utf-8"))
            mapping["asset_recoveries"][0]["expected_file_size"] += 1
            args.mapping.write_text(json.dumps(mapping), encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "drifted from allowlist"):
                MODULE.run(args)
            self.assertFalse(trace.exists())

    def test_missing_derivative_blocks_and_leaves_no_output(self):
        with tempfile.TemporaryDirectory() as raw:
            args, rows_path, trace = self.make_inputs(raw)
            rows = json.loads(rows_path.read_text(encoding="utf-8"))
            rows = [
                row
                for row in rows
                if not (
                    row.get("kind") == "derivative"
                    and row.get("source_asset_version_id") == 23989
                    and row.get("asset_type") == "preview"
                )
            ]
            rows_path.write_text(json.dumps(rows), encoding="utf-8")
            with mock.patch.dict(
                os.environ,
                {
                    "FAKE_MYSQL_ROWS": str(rows_path),
                    "FAKE_MYSQL_TRACE": str(trace),
                },
            ):
                with self.assertRaisesRegex(ValueError, "lineage 23989 drifted"):
                    MODULE.run(args)
            self.assertFalse(args.output.exists())


if __name__ == "__main__":
    unittest.main()
