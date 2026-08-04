import argparse
import json
import pathlib
import tempfile
import unittest
from unittest import mock

from scripts.ab import build_observed_gate_manifest as builder


class ObservedGateManifestTest(unittest.TestCase):
    def inputs(self, raw: str, gate: str = "G5") -> argparse.Namespace:
        root = pathlib.Path(raw)
        clone_root = root / "clone"
        clone_root.mkdir()
        run_dir = clone_root / "runs" / "hold"
        run_dir.mkdir(parents=True)
        ready_path = run_dir / "HOLD_OPEN_READY.json"
        ready = builder.hold.write_hashed_json(
            ready_path,
            {
                "schema_version": 1,
                "kind": "clone-b-hold-open-ready",
                "status": "HOLD_OPEN_READY",
                "clone_side": "B",
                "database_host_class": "local",
                "process_groups_quiescent": True,
                "production_writes_executed": False,
            },
        )
        result_path = root / "result.json"
        result = {
            "schema_version": 1,
            "status": "PASS",
            "violation_count": 0,
            "violations": [],
        }
        if gate == "G6":
            result[builder.hold.EVIDENCE_HASH_FIELD] = (
                builder.hold.compact_canonical_hash(result)
            )
        result_path.write_bytes(builder.hold.g4.canonical_bytes(result))
        return argparse.Namespace(
            gate=gate,
            result_json=result_path,
            hold_open_ready=ready_path,
            clone_root=clone_root.resolve(),
            output=(clone_root / "observed" / f"{gate.lower()}-manifest.json").resolve(),
            artifact=[],
            ready=ready,
        )

    def test_builds_g5_pass_manifest_and_is_idempotent(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.inputs(raw)
            first = builder.build(args)
            second = builder.build(args)
            self.assertEqual(first, second)
            manifest = builder.hold.read_hashed_json(
                args.output, "manifest", builder.hold.EVIDENCE_HASH_FIELD
            )
            self.assertEqual(manifest["status"], "PASS")
            self.assertEqual(manifest["violation_count"], 0)
            self.assertEqual(
                manifest["hold_open_ledger_sha256"],
                args.ready[builder.hold.DOCUMENT_HASH_FIELD],
            )
            self.assertEqual(len(manifest["artifacts"]), 1)

    def test_maps_hash_bound_g6_blocked_to_fail(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.inputs(raw, "G6")
            result = {
                "schema_version": 1,
                "status": "BLOCKED",
                "violation_count": 1,
                "violations": [{"code": "api-difference"}],
            }
            result[builder.hold.EVIDENCE_HASH_FIELD] = (
                builder.hold.compact_canonical_hash(result)
            )
            args.result_json.write_bytes(builder.hold.g4.canonical_bytes(result))
            built = builder.build(args)
            self.assertEqual(built["gate_status"], "FAIL")
            self.assertEqual(built["violation_count"], 1)

    def test_rejects_tampered_g6_self_hash(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.inputs(raw, "G6")
            result = json.loads(args.result_json.read_text(encoding="utf-8"))
            result["evidence_sha256"] = "0" * 64
            args.result_json.write_text(json.dumps(result), encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "self hash"):
                builder.build(args)

    def test_rejects_count_mismatch_and_duplicate_json_key(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.inputs(raw)
            args.result_json.write_text(
                '{"status":"PASS","status":"FAIL",'
                '"violation_count":0,"violations":[]}',
                encoding="utf-8",
            )
            with self.assertRaisesRegex(ValueError, "duplicate JSON key"):
                builder.build(args)
            args.result_json.write_text(
                '{"status":"FAIL","violation_count":2,'
                '"violations":[{"code":"one"}]}',
                encoding="utf-8",
            )
            with self.assertRaisesRegex(ValueError, "count is invalid"):
                builder.build(args)

    def test_refuses_to_overwrite_a_different_manifest(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.inputs(raw)
            args.output.parent.mkdir(parents=True)
            args.output.write_text('{"different":true}\n', encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "refusing to overwrite"):
                builder.build(args)

    def test_same_named_same_content_artifacts_get_unique_frozen_paths(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.inputs(raw)
            first = pathlib.Path(raw) / "one" / "gate_report.json"
            second = pathlib.Path(raw) / "two" / "gate_report.json"
            first.parent.mkdir()
            second.parent.mkdir()
            first.write_bytes(b"same evidence\n")
            second.write_bytes(b"same evidence\n")
            args.artifact = [first, second]
            builder.build(args)
            manifest = builder.hold.read_hashed_json(
                args.output, "manifest", builder.hold.EVIDENCE_HASH_FIELD
            )
            paths = [item["path"] for item in manifest["artifacts"]]
            self.assertEqual(len(paths), len(set(paths)))

    def test_rejects_symlink_in_output_path(self):
        with tempfile.TemporaryDirectory() as raw:
            args = self.inputs(raw)
            real = args.clone_root / "real"
            real.mkdir()
            alias = args.clone_root / "alias"
            alias.symlink_to(real, target_is_directory=True)
            args.output = (alias / "nested" / "manifest.json").absolute()
            with self.assertRaisesRegex(ValueError, "traverse a symlink"):
                builder.build(args)

    def test_hashed_json_rejects_duplicate_keys(self):
        with tempfile.TemporaryDirectory() as raw:
            path = pathlib.Path(raw) / "duplicate.json"
            path.write_text(
                '{"status":"FAIL","status":"PASS",'
                '"document_sha256":"' + "0" * 64 + '"}',
                encoding="utf-8",
            )
            with self.assertRaisesRegex(ValueError, "duplicate JSON key"):
                builder.hold.read_hashed_json(path, "duplicate")

    def test_atomic_publish_never_clobbers_a_concurrent_owner(self):
        with tempfile.TemporaryDirectory() as raw:
            target = pathlib.Path(raw) / "evidence.json"
            real_link = builder.os.link

            def concurrent_owner(source, destination):
                pathlib.Path(destination).write_bytes(b"concurrent-owner")
                return real_link(source, destination)

            with mock.patch.object(
                builder.os, "link", side_effect=concurrent_owner
            ):
                with self.assertRaisesRegex(
                    ValueError, "concurrent output differs"
                ):
                    builder.atomic_write_bytes(target, b"our-evidence")
            self.assertEqual(target.read_bytes(), b"concurrent-owner")


if __name__ == "__main__":
    unittest.main()
