from __future__ import annotations

import hashlib
import importlib.util
import json
import pathlib
import sys
import tempfile
import unittest


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
) -> dict:
    value = {
        "schema_version": 1,
        "status": "PASS",
        "input_manifest_sha256": PREPARE.sha256_file(force),
        "hydrated_manifest_sha256": PREPARE.sha256_file(hydrated),
        "checkpoint_sha256": hashlib.sha256(b"checkpoint").hexdigest(),
        "row_count": row_count,
        "already_complete_count": 0,
        "missing_sha256_count": row_count,
        "configured_target_row_count": row_count,
        "unique_target_count": unique_targets,
        "resumed_target_count": resumed_count,
        "resumed_failure_target_count": 0,
        "read_only_get_count": (
            unique_targets if get_count is None else get_count
        ),
        "hydrated_row_count": row_count,
        "deduplicated_get_count": row_count - unique_targets,
        "failure_count": 0,
        "failures": [],
    }
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


if __name__ == "__main__":
    unittest.main()
