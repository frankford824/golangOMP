from __future__ import annotations

import datetime as dt
import importlib.util
import json
import pathlib
import sys
import tempfile
import unittest


PATH = pathlib.Path(__file__).with_name("generate_api_session_fixtures.py")
SPEC = importlib.util.spec_from_file_location("api_session_fixtures", PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class GenerateAPISessionFixturesTest(unittest.TestCase):
    def test_builds_secret_separated_reversible_fixture(self) -> None:
        with tempfile.TemporaryDirectory() as raw:
            output = pathlib.Path(raw) / "fixture"
            counter = iter(("a" * 64, "b" * 64))
            manifest = MODULE.build(
                run_id="api-g06-001",
                clone_a_database="ab_formal20260723_01_a_ui",
                clone_b_database="ab_formal20260723_01_b_ui",
                identities=[
                    MODULE.Identity("admin", 1, "SuperAdmin"),
                    MODULE.Identity("designer", 306, "Designer"),
                ],
                output_dir=output,
                now=dt.datetime(
                    2026, 7, 25, 5, 30, tzinfo=dt.timezone.utc
                ),
                token_factory=lambda _size: next(counter),
            )
            self.assertEqual("PREPARED", manifest["status"])
            self.assertEqual(2, manifest["identity_count"])
            self.assertFalse(manifest["database_write_performed"])
            header = json.loads(
                (output / "admin.headers.json").read_text(encoding="utf-8")
            )
            self.assertEqual("Bearer " + "a" * 64, header["Authorization"])
            insert_a = (output / "insert-a.sql").read_text(encoding="utf-8")
            self.assertIn("ab_formal20260723_01_a_ui", insert_a)
            self.assertNotIn("a" * 64, insert_a)
            self.assertIn(manifest["identities"][0]["token_sha256"], insert_a)
            cleanup = (output / "cleanup-b.sql").read_text(encoding="utf-8")
            self.assertIn("DELETE FROM", cleanup)
            self.assertIn(manifest["identities"][0]["session_id"], cleanup)
            persisted = json.loads(
                (output / "fixture-manifest.json").read_text(encoding="utf-8")
            )
            self.assertEqual(manifest, persisted)

    def test_rejects_non_clone_database_and_duplicate_identity(self) -> None:
        with tempfile.TemporaryDirectory() as raw:
            with self.assertRaisesRegex(ValueError, "isolated"):
                MODULE.build(
                    run_id="api-g06-002",
                    clone_a_database="production",
                    clone_b_database="ab_formal20260723_01_b_ui",
                    identities=[MODULE.Identity("admin", 1, "Admin")],
                    output_dir=pathlib.Path(raw) / "bad",
                    now=dt.datetime.now(dt.timezone.utc),
                    token_factory=lambda _size: "x" * 64,
                )
            with self.assertRaisesRegex(ValueError, "unique"):
                MODULE.build(
                    run_id="api-g06-003",
                    clone_a_database="ab_formal20260723_01_a_ui",
                    clone_b_database="ab_formal20260723_01_b_ui",
                    identities=[
                        MODULE.Identity("admin", 1, "Admin"),
                        MODULE.Identity("admin", 2, "Member"),
                    ],
                    output_dir=pathlib.Path(raw) / "duplicate",
                    now=dt.datetime.now(dt.timezone.utc),
                    token_factory=lambda _size: "x" * 64,
                )

    def test_refuses_overwrite(self) -> None:
        with tempfile.TemporaryDirectory() as raw:
            output = pathlib.Path(raw) / "existing"
            output.mkdir()
            with self.assertRaises(FileExistsError):
                MODULE.build(
                    run_id="api-g06-004",
                    clone_a_database="ab_formal20260723_01_a_ui",
                    clone_b_database="ab_formal20260723_01_b_ui",
                    identities=[MODULE.Identity("admin", 1, "Admin")],
                    output_dir=output,
                    now=dt.datetime.now(dt.timezone.utc),
                    token_factory=lambda _size: "x" * 64,
                )


if __name__ == "__main__":
    unittest.main()
