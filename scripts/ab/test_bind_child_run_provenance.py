from __future__ import annotations

import importlib.util
import pathlib
import sys
import tempfile
import unittest


PATH = pathlib.Path(__file__).with_name("bind_child_run_provenance.py")
SPEC = importlib.util.spec_from_file_location("bind_child_run_provenance", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC and SPEC.loader
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class BindChildRunProvenanceTest(unittest.TestCase):
    def make_tree(self, root: pathlib.Path):
        base = root / "tmp" / "v8-ab"
        formal = base / "formal-run-1"
        child = base / "bundle-run-1"
        formal.mkdir(parents=True)
        child.mkdir()
        (child / "receipt.json").write_text("{}\n", encoding="utf-8")
        (child / "objects").mkdir()
        (child / "objects" / "asset.bin").write_bytes(b"asset")
        return formal, child

    def test_exact_child_inventory_is_deterministic(self) -> None:
        with tempfile.TemporaryDirectory() as raw:
            formal, child = self.make_tree(pathlib.Path(raw))
            first = MODULE.bind(formal, [("bundle", child)])
            second = MODULE.bind(formal, [("bundle", child)])
            self.assertEqual(first, second)
            self.assertEqual("PASS", first["status"])
            self.assertEqual(2, first["children"][0]["file_count"])
            self.assertEqual(
                {"objects/asset.bin", "receipt.json"},
                {row["path"] for row in first["children"][0]["files"]},
            )

    def test_content_change_changes_binding(self) -> None:
        with tempfile.TemporaryDirectory() as raw:
            formal, child = self.make_tree(pathlib.Path(raw))
            first = MODULE.bind(formal, [("bundle", child)])
            (child / "objects" / "asset.bin").write_bytes(b"changed")
            second = MODULE.bind(formal, [("bundle", child)])
            self.assertNotEqual(first["binding_sha256"], second["binding_sha256"])

    def test_duplicate_or_self_binding_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as raw:
            formal, child = self.make_tree(pathlib.Path(raw))
            with self.assertRaisesRegex(ValueError, "unique"):
                MODULE.bind(
                    formal,
                    [("bundle", child), ("bundle", child)],
                )
            with self.assertRaisesRegex(ValueError, "itself"):
                MODULE.bind(formal, [("formal", formal)])

    def test_child_outside_tmp_v8_ab_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            formal, _ = self.make_tree(root)
            outside = root / "elsewhere"
            outside.mkdir()
            (outside / "x").write_text("x", encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "tmp/v8-ab"):
                MODULE.bind(formal, [("outside", outside)])


if __name__ == "__main__":
    unittest.main()
