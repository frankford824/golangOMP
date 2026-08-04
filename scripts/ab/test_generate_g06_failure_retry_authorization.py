from __future__ import annotations

import hashlib
import pathlib
import tempfile
import unittest
from unittest import mock

from scripts.ab import generate_g06_failure_retry_authorization as MODULE


class GenerateG06FailureRetryAuthorizationTest(unittest.TestCase):
    def row(self, owner_id: int, object_key: str) -> dict:
        return {
            "entity_key": f"task_asset:{owner_id}",
            "owner_kind": "task_asset",
            "owner_id": owner_id,
            "task_id": 7,
            "storage_ref_id": f"ref-{owner_id}",
            "storage_adapter": "oss_upload_service",
            "object_key": object_key,
            "size": 0,
            "mime_type": "unknown/unknown",
            "sha256": "",
            "status": "recorded",
            "is_placeholder": False,
        }

    def write_json(self, path: pathlib.Path, value: dict) -> None:
        path.write_text(
            MODULE.verifier.canonical_json(value) + "\n",
            encoding="utf-8",
        )

    def write_manifest(self, root: pathlib.Path, rows: list[dict]) -> pathlib.Path:
        path = root / "input.jsonl"
        path.write_text(
            "".join(
                MODULE.verifier.canonical_json(row) + "\n"
                for row in rows
            ),
            encoding="utf-8",
        )
        return path

    def write_checkpoint(
        self,
        root: pathlib.Path,
        manifest: pathlib.Path,
        failures: list[tuple[dict, str, str]],
    ) -> pathlib.Path:
        failed = {}
        for row, code, detail in failures:
            key = MODULE.hydrator.checkpoint_key("upload", row["object_key"])
            failed[key] = MODULE.hydrator.checkpoint_failure_record(
                "upload", row["object_key"], code, detail
            )
        value = MODULE.hydrator.checkpoint_document(
            MODULE.verifier.sha256_file(manifest),
            {"upload": "a" * 64},
            {},
            failed,
        )
        path = root / "checkpoint.json"
        self.write_json(path, value)
        return path

    def write_reprobe_pair(
        self,
        root: pathlib.Path,
        label: str,
        source: dict,
        *,
        body: bytes,
        second_body: bytes | None = None,
    ) -> MODULE.ReprobePair:
        paths = []
        for index, content in (
            (1, body),
            (2, body if second_body is None else second_body),
        ):
            row = dict(source)
            row["size"] = len(content)
            row["mime_type"] = "application/octet-stream"
            row["sha256"] = hashlib.sha256(content).hexdigest()
            artifact = root / f"{label}-{index}.jsonl"
            self.write_json(artifact, row)
            artifact_sha = MODULE.verifier.sha256_file(artifact)
            evidence = {
                "status": "PASS",
                "failure_count": 0,
                "configured_target_row_count": 1,
                "unique_target_count": 1,
                "read_only_get_count": 1,
                "hydrated_row_count": 1,
                "hydrated_manifest_sha256": artifact_sha,
            }
            evidence["evidence_hash"] = hashlib.sha256(
                MODULE.verifier.canonical_json(evidence).encode("utf-8")
            ).hexdigest()
            evidence_path = root / f"{label}-{index}.evidence.json"
            self.write_json(evidence_path, evidence)
            paths.extend((evidence_path, artifact))
        return MODULE.ReprobePair(*paths)

    def fixture(self, root: pathlib.Path) -> dict:
        rows = [
            self.row(10, "tasks/7/a.bin"),
            self.row(11, "tasks/7/b.bin"),
            self.row(12, "tasks/7/transient.bin"),
        ]
        manifest = self.write_manifest(root, rows)
        checkpoint = self.write_checkpoint(
            root,
            manifest,
            [
                (
                    rows[0],
                    MODULE.TARGET_CODE,
                    MODULE.TARGET_DETAIL,
                ),
                (
                    rows[1],
                    MODULE.TARGET_CODE,
                    MODULE.TARGET_DETAIL,
                ),
                (
                    rows[2],
                    "object_manifest.unreadable",
                    "timeout",
                ),
            ],
        )
        return {
            "rows": rows,
            "manifest": manifest,
            "checkpoint": checkpoint,
            "pair_a": self.write_reprobe_pair(
                root, "a", rows[0], body=b"a-stable"
            ),
            "pair_b": self.write_reprobe_pair(
                root, "b", rows[1], body=b"b-stable"
            ),
            "pair_transient": self.write_reprobe_pair(
                root, "transient", rows[2], body=b"transient-stable"
            ),
        }

    def test_generates_reproducible_sorted_relative_self_validated_authorization(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            fixture = self.fixture(root)
            output1 = root / "authorization-1.json"
            output2 = root / "authorization-2.json"
            validator = MODULE.hydrator.load_failure_retry_authorization
            with mock.patch.object(
                MODULE.hydrator,
                "load_failure_retry_authorization",
                wraps=validator,
            ) as validation:
                first = MODULE.generate_authorization(
                    fixture["manifest"],
                    fixture["checkpoint"],
                    output1,
                    [fixture["pair_b"], fixture["pair_a"]],
                )
            second = MODULE.generate_authorization(
                fixture["manifest"],
                fixture["checkpoint"],
                output2,
                [fixture["pair_a"], fixture["pair_b"]],
            )
            first_bytes = output1.read_bytes()
            second_bytes = output2.read_bytes()

        self.assertTrue(validation.called)
        self.assertEqual(first, second)
        self.assertEqual(first_bytes, second_bytes)
        failure_hashes = [
            item["failure_record_sha256"]
            for item in first["failure_retries"]
        ]
        self.assertEqual(failure_hashes, sorted(failure_hashes))
        self.assertEqual(2, len(failure_hashes))
        for item in first["failure_retries"]:
            for reprobe in item["reprobes"]:
                self.assertFalse(pathlib.Path(reprobe["evidence_path"]).is_absolute())
                self.assertFalse(pathlib.Path(reprobe["artifact_path"]).is_absolute())
                self.assertNotIn("\\", reprobe["evidence_path"])
                self.assertNotIn("\\", reprobe["artifact_path"])
        unsigned = {
            key: value
            for key, value in first.items()
            if key != "authorization_sha256"
        }
        self.assertEqual(
            hashlib.sha256(
                MODULE.verifier.canonical_json(unsigned).encode("utf-8")
            ).hexdigest(),
            first["authorization_sha256"],
        )

    def test_rejects_missing_reprobe_pair(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            fixture = self.fixture(root)
            with self.assertRaisesRegex(ValueError, "does not cover all"):
                MODULE.generate_authorization(
                    fixture["manifest"],
                    fixture["checkpoint"],
                    root / "authorization.json",
                    [fixture["pair_a"]],
                )

    def test_rejects_duplicate_reprobe_pair(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            fixture = self.fixture(root)
            with self.assertRaisesRegex(ValueError, "paths are duplicated"):
                MODULE.generate_authorization(
                    fixture["manifest"],
                    fixture["checkpoint"],
                    root / "authorization.json",
                    [fixture["pair_a"], fixture["pair_a"]],
                )

    def test_rejects_extra_pair_for_non_target_failure(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            fixture = self.fixture(root)
            with self.assertRaisesRegex(ValueError, "non-authorizable"):
                MODULE.generate_authorization(
                    fixture["manifest"],
                    fixture["checkpoint"],
                    root / "authorization.json",
                    [fixture["pair_a"], fixture["pair_transient"]],
                )

    def test_rejects_reprobe_artifact_disagreement(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            fixture = self.fixture(root)
            disagreeing = self.write_reprobe_pair(
                root,
                "disagreeing",
                fixture["rows"][0],
                body=b"first",
                second_body=b"second",
            )
            with self.assertRaisesRegex(ValueError, "do not agree"):
                MODULE.generate_authorization(
                    fixture["manifest"],
                    fixture["checkpoint"],
                    root / "authorization.json",
                    [disagreeing, fixture["pair_b"]],
                )

    def test_cli_accepts_repeated_four_path_pairs(self):
        args = MODULE.parse_args([
            "input.jsonl",
            "checkpoint.json",
            "authorization.json",
            "--reprobe-pair",
            "e1.json",
            "a1.jsonl",
            "e2.json",
            "a2.jsonl",
            "--reprobe-pair",
            "e3.json",
            "a3.jsonl",
            "e4.json",
            "a4.jsonl",
        ])
        self.assertEqual(2, len(args.reprobe_pair))
        self.assertEqual(
            ["e1.json", "a1.jsonl", "e2.json", "a2.jsonl"],
            args.reprobe_pair[0],
        )


if __name__ == "__main__":
    unittest.main()
