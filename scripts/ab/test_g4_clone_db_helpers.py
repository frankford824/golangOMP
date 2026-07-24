import argparse
import hashlib
import json
import os
import pathlib
import tempfile
import unittest
from unittest import mock

from scripts.ab import capture_and_compare_rollback_fingerprint as fingerprint
from scripts.ab import capture_search_documents_snapshot as capture_search
from scripts.ab import g4_clone_db as db
from scripts.ab import restore_search_documents_snapshot as restore_search


def schema_for(columns_by_table):
    result = {}
    for table, names in columns_by_table.items():
        result[table] = [
            {
                "kind": "column",
                "table": table,
                "ordinal": index,
                "name": name,
                "column_type": "varchar(255)",
                "nullable": "NO",
                "default_hex": None,
                "extra": "",
                "generation_expression": "",
                "character_set": "utf8mb4",
                "collation": "utf8mb4_0900_ai_ci",
            }
            for index, name in enumerate(names, 1)
        ]
    return result


class FakeConnection:
    def __init__(self, schema, rows, auto_increments=None):
        self.database = "ab_formal_b_ui"
        self.schema = schema
        self.rows = rows
        self.auto_increments = auto_increments or {
            table: None for table in schema
        }
        self.executed = []

    def execute(self, sql):
        self.executed.append(sql)
        return ""

    def iter_output_lines(self, sql):
        self.executed.append(sql)
        for table in sorted(self.schema):
            for column in self.schema[table]:
                yield db.canonical_bytes(column)
            yield db.canonical_bytes(
                {
                    "kind": "table_metadata",
                    "table": table,
                    "auto_increment": self.auto_increments[table],
                }
            )
            for cells in self.rows[table]:
                yield db.canonical_bytes(
                    {"kind": "row", "table": table, "cells": cells}
                )


class G4CloneDBHelpersTest(unittest.TestCase):
    def test_connection_requires_all_three_clone_b_bindings(self):
        env = {
            "AB_CONFIRMED_CLONE_SIDE": "B",
            "AB_CONFIRMED_CLONE_DATABASE": "ab_formal_b_ui",
            "MYSQL_DSN": "u:p@tcp(127.0.0.1:3322)/ab_formal_b_ui?parseTime=true",
        }
        with mock.patch.dict(os.environ, env, clear=True):
            value = db.Connection.confirmed_clone_b("ab_formal_b_ui")
        self.assertEqual(value.host, "127.0.0.1")
        self.assertEqual(value.port, 3322)
        with mock.patch.dict(
            os.environ, {**env, "AB_CONFIRMED_CLONE_SIDE": "A"}, clear=True
        ):
            with self.assertRaisesRegex(ValueError, "must be B"):
                db.Connection.confirmed_clone_b("ab_formal_b_ui")
        with mock.patch.dict(os.environ, env, clear=True):
            with self.assertRaisesRegex(ValueError, "ab_.+_b"):
                db.Connection.confirmed_clone_b("production")

    def test_search_schema_is_explicit_and_asset_search_is_not_reindexed(self):
        valid = schema_for(db.SEARCH_COLUMNS)
        db.validate_search_schema(valid)
        invalid = dict(valid)
        invalid["asset_search_documents"] = schema_for(
            {"asset_search_documents": ("asset_id",)}
        )["asset_search_documents"]
        with self.assertRaisesRegex(RuntimeError, "table set"):
            db.validate_search_schema(invalid)
        source = (
            pathlib.Path(__file__).parents[2]
            / "cmd/tools/search-reindex/main.go"
        ).read_text(encoding="utf-8")
        self.assertIn("DELETE FROM task_asset_group_search_documents", source)
        self.assertIn("DELETE FROM product_search_documents", source)
        self.assertNotIn("DELETE FROM asset_search_documents", source)

    def test_archive_roundtrip_and_restore_sql_are_exact(self):
        schema = schema_for(db.SEARCH_COLUMNS)
        rows = {
            table: [["41" for _ in columns], ["42" for _ in columns]]
            for table, columns in db.SEARCH_COLUMNS.items()
        }
        archive = db.make_search_archive(schema, rows)
        parsed_schema, parsed_rows = db.parse_search_archive(archive)
        self.assertEqual(parsed_schema, schema)
        self.assertEqual(parsed_rows, rows)
        sql = db.restore_sql(parsed_schema, parsed_rows)
        self.assertIn("START TRANSACTION", sql)
        self.assertIn("COMMIT", sql)
        for table in db.SEARCH_COLUMNS:
            self.assertIn(f"DELETE FROM `{table}`", sql)
            self.assertIn(f"INSERT INTO `{table}`", sql)
        self.assertNotIn("asset_search_documents", sql)

    def test_full_fingerprint_is_order_independent_and_preserves_duplicates(self):
        self.assertIn(
            "SET SESSION information_schema_stats_expiry=0;",
            db._capture_sql(
                schema_for({"probe": ("id",)}),
                include_table_metadata=True,
            ),
        )
        schema = schema_for(
            {
                "blob_without_primary_key": ("payload", "label"),
                "empty_table": ("value",),
            }
        )
        blob = "AB" * (1024 * 1024)
        rows = {
            "blob_without_primary_key": [
                [blob, "42"],
                ["00FF", None],
                [blob, "42"],
                ["", "41"],
            ],
            "empty_table": [],
        }
        reversed_rows = {
            table: list(reversed(values)) for table, values in rows.items()
        }
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            _, first = db.capture_fingerprint(
                FakeConnection(schema, rows),
                schema,
                temporary_directory=root,
                chunk_rows=2,
                merge_fan_in=2,
            )
            _, second = db.capture_fingerprint(
                FakeConnection(schema, reversed_rows),
                schema,
                temporary_directory=root,
                chunk_rows=2,
                merge_fan_in=2,
            )
        self.assertEqual(first, second)
        self.assertEqual(
            first["blob_without_primary_key"]["row_count"], 4
        )
        self.assertEqual(first["empty_table"]["row_count"], 0)
        self.assertIsNone(first["empty_table"]["auto_increment"])
        self.assertEqual(
            first["blob_without_primary_key"][
                "content_fingerprint_algorithm"
            ],
            db.ROW_FINGERPRINT_ALGORITHM,
        )

        without_duplicate = {
            **rows,
            "blob_without_primary_key": rows["blob_without_primary_key"][:-1],
        }
        _, third = db.capture_fingerprint(
            FakeConnection(schema, without_duplicate),
            schema,
            chunk_rows=2,
            merge_fan_in=2,
        )
        self.assertNotEqual(
            first["blob_without_primary_key"]["content_sha256"],
            third["blob_without_primary_key"]["content_sha256"],
        )

    def test_row_digest_spool_has_strict_buffer_and_merge_fan_in_bounds(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            spool = db._RowDigestSpool(
                root,
                "large_fixture",
                chunk_rows=7,
                merge_fan_in=3,
            )
            for value in range(10_003):
                spool.add([f"{value:08X}"])
            summary = spool.finish()
            self.assertEqual(summary["row_count"], 10_003)
            self.assertLessEqual(spool.max_buffered_digests, 7)
            self.assertEqual(list(root.iterdir()), [])

    def test_full_fingerprint_rejects_schema_drift_and_missing_table(self):
        expected = schema_for({"one": ("id",), "two": ("id",)})
        drifted = schema_for({"one": ("changed",), "two": ("id",)})
        with self.assertRaisesRegex(RuntimeError, "schema drifted"):
            db.capture_fingerprint(
                FakeConnection(drifted, {"one": [], "two": []}),
                expected,
                chunk_rows=2,
            )

        incomplete = FakeConnection(
            {"one": expected["one"]},
            {"one": []},
        )
        with self.assertRaisesRegex(RuntimeError, "omitted"):
            db.capture_fingerprint(incomplete, expected, chunk_rows=2)

    @mock.patch.object(db, "capture")
    @mock.patch.object(db, "discover_schema")
    @mock.patch.object(db.Connection, "confirmed_clone_b")
    def test_search_capture_writes_bound_snapshot(
        self, confirmed, discover, capture
    ):
        schema = schema_for(db.SEARCH_COLUMNS)
        rows = {
            table: [["41" for _ in columns]]
            for table, columns in db.SEARCH_COLUMNS.items()
        }
        confirmed.return_value = FakeConnection(schema, rows)
        discover.return_value = schema
        capture.return_value = (schema, rows)
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            result = capture_search.run(
                argparse.Namespace(
                    database="ab_formal_b_ui",
                    mysql="mysql",
                    archive=root / "rows.jsonl",
                    output=root / "snapshot.json",
                )
            )
            self.assertEqual(result["status"], "CAPTURED")
            self.assertEqual(set(result["tables"]), set(db.SEARCH_COLUMNS))
            self.assertEqual(
                result["archive"]["sha256"],
                hashlib.sha256((root / "rows.jsonl").read_bytes()).hexdigest(),
            )

    @mock.patch.object(db, "capture")
    @mock.patch.object(db, "discover_schema")
    @mock.patch.object(db.Connection, "confirmed_clone_b")
    def test_restore_rejects_schema_drift_before_write(
        self, confirmed, discover, capture
    ):
        schema = schema_for(db.SEARCH_COLUMNS)
        rows = {
            table: [["41" for _ in columns]]
            for table, columns in db.SEARCH_COLUMNS.items()
        }
        connection = FakeConnection(schema, rows)
        confirmed.return_value = connection
        discover.return_value = {
            **schema,
            "task_search_documents": schema["task_search_documents"][:-1],
        }
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            archive = db.make_search_archive(schema, rows)
            (root / "rows.jsonl").write_bytes(archive)
            tables = db.table_summaries(schema, rows, include_schema=False)
            snapshot = {
                "schema_version": 1,
                "status": "CAPTURED",
                "violation_count": 0,
                "database": "ab_formal_b_ui",
                "tables": tables,
                "snapshot_sha256": db.sha256_bytes(
                    db.canonical_bytes(tables)
                ),
                "archive": {
                    "format": "deterministic-jsonl-v1",
                    "sha256": db.sha256_bytes(archive),
                    "size": len(archive),
                },
            }
            (root / "snapshot.json").write_bytes(db.canonical_bytes(snapshot))
            with self.assertRaisesRegex(RuntimeError, "explicit columns"):
                restore_search.run(
                    argparse.Namespace(
                        database="ab_formal_b_ui",
                        mysql="mysql",
                        archive=root / "rows.jsonl",
                        snapshot=root / "snapshot.json",
                        output=root / "restore.json",
                    )
                )
        self.assertEqual(connection.executed, [])

    @mock.patch.object(fingerprint, "capture_document")
    @mock.patch.object(db.Connection, "confirmed_clone_b")
    def test_fingerprint_binds_explicit_baseline_and_reports_drift(
        self, confirmed, capture_document
    ):
        confirmed.return_value = FakeConnection({}, {})
        baseline_tables = {
            "tasks": {
                "row_count": 1,
                "content_sha256": "1" * 64,
                "schema_sha256": "2" * 64,
            }
        }
        baseline = {
            "schema_version": 1,
            "kind": "clone-b-baseline-fingerprint",
            "database": "ab_formal_b_ui",
            "fingerprint_algorithm": db.ROW_FINGERPRINT_ALGORITHM,
            "tables": baseline_tables,
            "fingerprint_sha256": db.sha256_bytes(
                db.canonical_bytes(baseline_tables)
            ),
        }
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            (root / "baseline.json").write_bytes(db.canonical_bytes(baseline))
            capture_document.return_value = dict(baseline)
            passed = fingerprint.run(
                argparse.Namespace(
                    database="ab_formal_b_ui",
                    mysql="mysql",
                    capture_baseline=False,
                    baseline=root / "baseline.json",
                    output=root / "pass.json",
                )
            )
            self.assertEqual(passed["status"], "PASS")
            capture_document.return_value = {
                **baseline,
                "tables": {
                    "tasks": {**baseline_tables["tasks"], "row_count": 2}
                },
                "fingerprint_sha256": "f" * 64,
            }
            with self.assertRaisesRegex(RuntimeError, "differs"):
                fingerprint.run(
                    argparse.Namespace(
                        database="ab_formal_b_ui",
                        mysql="mysql",
                        capture_baseline=False,
                        baseline=root / "baseline.json",
                        output=root / "fail.json",
                    )
                )
            failed = json.loads((root / "fail.json").read_text())
            self.assertEqual(failed["status"], "FAIL")
            self.assertEqual(failed["changed_tables"], ["tasks"])


if __name__ == "__main__":
    unittest.main()
