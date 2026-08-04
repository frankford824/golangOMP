from __future__ import annotations

import hashlib
import json
import re
import tempfile
import unittest
from pathlib import Path
from typing import Any

from scripts.ab import export_frozen_a_oracle as oracle


def _hex(value: str) -> str:
    return value.encode("utf-8").hex().upper()


def _mysql_type(column: str, key: str) -> str:
    if column == key and key != "ref_id":
        return "bigint"
    if column in oracle._DECIMAL_FIELDS:
        return "decimal(12,3)"
    if column.endswith("_at") or column == "deadline_at":
        return "datetime(6)"
    if column in oracle._NATIVE_JSON_FIELDS:
        return "json"
    return "varchar(255)"


def _sample_value(column: str, mysql_type: str, key: str) -> Any:
    if column == key:
        return 1 if key != "ref_id" else "ref-001"
    if column in oracle._DECIMAL_FIELDS:
        return "12.300"
    if column.endswith("_at") or column == "deadline_at":
        return "2026-07-25T01:02:03.000000Z"
    if column in oracle._NATIVE_JSON_FIELDS:
        return {"nested": [2, 1], "name": "样本"}
    if column in oracle._JSON_TEXT_FIELDS:
        return '[{"ref_id":"r-2"},{"ref_id":"r-1"}]'
    if mysql_type == "bigint":
        return 1
    return f"value:{column}"


def _fixture_output(
    *,
    missing_column: tuple[str, str] | None = None,
    duplicate_task_row: bool = False,
) -> str:
    marker = oracle._MARKER
    metadata = {
        "mysql_version": "8.0.46",
        "server_uuid": "test-server",
        "session_time_zone": "+00:00",
        "system_time_zone": "UTC",
    }
    lines = [
        f"{marker}\tmeta\tall",
        _hex(json.dumps(metadata, ensure_ascii=False)),
    ]
    for spec in oracle.DATASETS:
        lines.append(f"{marker}\tschema\t{spec.name}")
        schema_columns = list(spec.columns)
        if spec.excluded_schema_columns:
            schema_columns.extend(spec.excluded_schema_columns)
        if missing_column and missing_column[0] == spec.name:
            schema_columns.remove(missing_column[1])
        type_by_column: dict[str, str] = {}
        for column in schema_columns:
            mysql_type = _mysql_type(column, spec.key)
            type_by_column[column] = mysql_type
            nullable = "NO" if column == spec.key else "YES"
            key_flag = "PRI" if column == spec.key else ""
            lines.append(
                "\t".join((column, mysql_type, nullable, key_flag, "NULL", ""))
            )
        lines.append(f"{marker}\tdata\t{spec.name}")
        payload = {
            column: _sample_value(
                column, type_by_column.get(column, "varchar(255)"), spec.key
            )
            for column in spec.columns
        }
        key = payload[spec.key]
        encoded = json.dumps(payload, ensure_ascii=False, separators=(",", ":"))
        lines.append(f"{_hex(str(key))}\t{_hex(encoded)}")
        if duplicate_task_row and spec.name == "tasks":
            lines.append(f"{_hex(str(key))}\t{_hex(encoded)}")
    lines.append(f"{marker}\tfinal\tall")
    return "\n".join(lines) + "\n"


class FrozenAOracleTests(unittest.TestCase):
    def test_sql_is_one_read_only_consistent_snapshot(self) -> None:
        sql = oracle.build_snapshot_sql("ab_r20260723_01_a")
        self.assertEqual(
            sql.count("START TRANSACTION READ ONLY, WITH CONSISTENT SNAPSHOT;"),
            1,
        )
        self.assertEqual(sql.count("COMMIT;"), 1)
        self.assertEqual(sql.count("SHOW COLUMNS FROM"), 7)
        self.assertEqual(sql.count("ORDER BY"), 7)
        self.assertIn("SET SESSION time_zone = '+00:00';", sql)
        self.assertIn("CAST(`base_sale_price` AS CHAR)", sql)
        self.assertIn(
            "DATE_FORMAT(`created_at`,'%Y-%m-%dT%H:%i:%s.%fZ')", sql
        )
        self.assertIn("JSON_EXTRACT(`variant_json`,'$')", sql)
        for forbidden in (
            "INSERT", "UPDATE", "DELETE", "REPLACE", "CREATE", "DROP", "ALTER",
            "TRUNCATE", "LOCK TABLES", "UNLOCK TABLES",
        ):
            self.assertIsNone(
                re.search(rf"\b{re.escape(forbidden)}\b", sql, re.IGNORECASE),
                forbidden,
            )
        self.assertNotRegex(sql, r"(?i)password|passwd|identified\s+by")

    def test_export_writes_seven_canonical_ndjson_files_and_manifest(self) -> None:
        captured: dict[str, Any] = {}

        def runner(command: list[str], sql: str) -> str:
            captured["command"] = list(command)
            captured["sql"] = sql
            return _fixture_output()

        with tempfile.TemporaryDirectory() as directory:
            output_dir = Path(directory)
            manifest = oracle.export_frozen_a_oracle(
                database="ab_r20260723_01_a",
                output_dir=output_dir,
                mysql_bin="mysql",
                host="127.0.0.1",
                port=3306,
                user="readonly",
                runner=runner,
            )
            self.assertEqual(len(manifest["datasets"]), 7)
            self.assertEqual(
                manifest["transaction"],
                {
                    "access_mode": "READ ONLY",
                    "consistent_snapshot": True,
                    "isolation_level": "REPEATABLE READ",
                    "session_time_zone": "+00:00",
                    "single_connection": True,
                },
            )
            for dataset_manifest in manifest["datasets"]:
                self.assertEqual(
                    set(dataset_manifest),
                    {
                        "columns_sha256",
                        "dataset",
                        "dataset_sha256",
                        "file",
                        "file_sha256",
                        "first_key",
                        "key",
                        "last_key",
                        "row_count",
                        "schema",
                        "schema_sha256",
                        "source_table",
                    },
                )
                self.assertIsInstance(dataset_manifest["schema"], list)
                self.assertGreater(len(dataset_manifest["schema"]), 0)
                self.assertEqual(
                    dataset_manifest["schema_sha256"],
                    hashlib.sha256(
                        oracle.canonical_json_bytes(dataset_manifest["schema"])
                    ).hexdigest(),
                )
                for column in dataset_manifest["schema"]:
                    self.assertEqual(
                        set(column),
                        {"default", "extra", "field", "key", "null", "type"},
                    )
            for spec in oracle.DATASETS:
                path = output_dir / f"{spec.name}.ndjson"
                self.assertTrue(path.is_file())
                raw = path.read_bytes()
                self.assertTrue(raw.endswith(b"\n"))
                line = json.loads(raw)
                self.assertEqual(line["dataset"], spec.name)
                expected_hash = hashlib.sha256(
                    oracle.canonical_json_bytes(
                        {
                            "dataset": spec.name,
                            "row": line["row"],
                            "schema_version": oracle.SCHEMA_VERSION,
                        }
                    )
                ).hexdigest()
                self.assertEqual(line["row_sha256"], expected_hash)
            sku_line = json.loads((output_dir / "skus.ndjson").read_bytes())
            self.assertEqual(sku_line["row"]["base_sale_price"], "12.300")
            self.assertEqual(
                sku_line["row"]["set_mode_hint"],
                "value:set_mode_hint",
            )
            self.assertEqual(
                sku_line["row"]["reference_file_refs_json"],
                [{"ref_id": "r-2"}, {"ref_id": "r-1"}],
            )
            self.assertEqual(
                sku_line["row"]["created_at"],
                "2026-07-25T01:02:03.000000Z",
            )
            manifest_on_disk = json.loads(
                (output_dir / "manifest.json").read_bytes()
            )
            evidence_hash = manifest_on_disk.pop("evidence_sha256")
            self.assertEqual(
                evidence_hash,
                hashlib.sha256(
                    oracle.canonical_json_bytes(manifest_on_disk)
                ).hexdigest(),
            )
            self.assertNotIn("password", json.dumps(manifest).lower())
            self.assertNotIn("password", " ".join(captured["command"]).lower())
            self.assertEqual(captured["sql"].count("START TRANSACTION"), 1)

    def test_docker_mode_still_invokes_exactly_one_mysql_client(self) -> None:
        captured: dict[str, Any] = {}

        def runner(command: list[str], sql: str) -> str:
            captured["command"] = list(command)
            return _fixture_output()

        with tempfile.TemporaryDirectory() as directory:
            oracle.export_frozen_a_oracle(
                database="ab_r20260723_01_a",
                output_dir=Path(directory),
                docker_container="codex-yongbo-ab-a-compare-20260724",
                user="root",
                runner=runner,
            )
        self.assertEqual(
            captured["command"][:6],
            [
                "docker",
                "exec",
                "-i",
                "codex-yongbo-ab-a-compare-20260724",
                "mysql",
                "--batch",
            ],
        )
        self.assertEqual(captured["command"].count("mysql"), 1)
        with self.assertRaisesRegex(oracle.OracleExportError, "unsafe Docker"):
            oracle._mysql_command(
                "mysql",
                "container;whoami",
                None,
                None,
                None,
                "root",
                None,
            )

    def test_embedded_schema_is_bound_by_dataset_and_evidence_hashes(self) -> None:
        metadata, schemas, rows = oracle.parse_mysql_output(_fixture_output())
        manifest, _ = oracle.build_evidence(
            "ab_r20260723_01_a", metadata, schemas, rows
        )
        tasks = manifest["datasets"][0]
        self.assertEqual(
            tasks["schema_sha256"],
            hashlib.sha256(
                oracle.canonical_json_bytes(tasks["schema"])
            ).hexdigest(),
        )
        original_schema_hash = tasks["schema_sha256"]
        original_evidence_hash = manifest["evidence_sha256"]
        tasks["schema"][0]["type"] = "varchar(999)"
        self.assertNotEqual(
            original_schema_hash,
            hashlib.sha256(
                oracle.canonical_json_bytes(tasks["schema"])
            ).hexdigest(),
        )
        unsigned = {
            key: value
            for key, value in manifest.items()
            if key != "evidence_sha256"
        }
        self.assertNotEqual(
            original_evidence_hash,
            hashlib.sha256(oracle.canonical_json_bytes(unsigned)).hexdigest(),
        )

    def test_schema_drift_is_a_hard_failure(self) -> None:
        output = _fixture_output(missing_column=("tasks", "task_no"))
        metadata, schemas, rows = oracle.parse_mysql_output(output)
        with self.assertRaisesRegex(oracle.OracleExportError, "column drift"):
            oracle.build_evidence(
                "ab_r20260723_01_a", metadata, schemas, rows
            )

    def test_duplicate_or_unsorted_key_is_a_hard_failure(self) -> None:
        output = _fixture_output(duplicate_task_row=True)
        metadata, schemas, rows = oracle.parse_mysql_output(output)
        with self.assertRaisesRegex(
            oracle.OracleExportError, "duplicate or out of order"
        ):
            oracle.build_evidence(
                "ab_r20260723_01_a", metadata, schemas, rows
            )

    def test_noncanonical_decimal_and_datetime_are_rejected(self) -> None:
        output = _fixture_output()
        metadata, schemas, rows = oracle.parse_mysql_output(output)
        sku_payload = rows["skus"][0][1]
        sku_payload["base_sale_price"] = 12.3
        with self.assertRaisesRegex(oracle.OracleExportError, "DECIMAL"):
            oracle.build_evidence(
                "ab_r20260723_01_a", metadata, schemas, rows
            )

        metadata, schemas, rows = oracle.parse_mysql_output(output)
        task_payload = rows["tasks"][0][1]
        task_payload["created_at"] = "2026-07-25 01:02:03"
        with self.assertRaisesRegex(oracle.OracleExportError, "DATETIME"):
            oracle.build_evidence(
                "ab_r20260723_01_a", metadata, schemas, rows
            )

    def test_null_is_preserved_but_not_null_violation_fails(self) -> None:
        output = _fixture_output()
        metadata, schemas, rows = oracle.parse_mysql_output(output)
        rows["tasks"][0][1]["task_no"] = None
        _, files = oracle.build_evidence(
            "ab_r20260723_01_a", metadata, schemas, rows
        )
        task_line = json.loads(files["tasks.ndjson"])
        self.assertIsNone(task_line["row"]["task_no"])

        metadata, schemas, rows = oracle.parse_mysql_output(output)
        rows["tasks"][0][1]["id"] = None
        with self.assertRaisesRegex(oracle.OracleExportError, "NOT NULL"):
            oracle.build_evidence(
                "ab_r20260723_01_a", metadata, schemas, rows
            )

    def test_unsafe_database_identifier_is_rejected(self) -> None:
        with self.assertRaisesRegex(oracle.OracleExportError, "unsafe"):
            oracle.build_snapshot_sql("db; DROP DATABASE prod")


if __name__ == "__main__":
    unittest.main()
