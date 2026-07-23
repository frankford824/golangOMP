import datetime
import hashlib
import importlib.util
import json
import pathlib
import sys
import tempfile
import unittest


PATH = pathlib.Path(__file__).with_name("fetch_bundle_sources.py")
SPEC = importlib.util.spec_from_file_location("fetch_bundle_sources", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class FakeAdapter:
    def __init__(self, _host, _env_path, _timeout, bodies, fail_id=None):
        self.bodies = bodies
        self.fail_id = fail_id
        self.calls = []
        self.closed = False

    def origin_fingerprint(self):
        return "a" * 64

    def fetch_to_path(self, source, target):
        self.calls.append(source.task_asset_id)
        target.write_bytes(self.bodies[source.task_asset_id])
        if source.task_asset_id == self.fail_id:
            raise MODULE.controlled.ControlledReadError("synthetic failure")

    def close(self):
        self.closed = True


class FetchBundleSourcesTest(unittest.TestCase):
    def setUp(self):
        self.original = {
            "candidate_file_sha": MODULE.FROZEN_CANDIDATE_FILE_SHA256,
            "candidate_digest": MODULE.FROZEN_CANDIDATE_DIGEST,
            "hydrated_sha": MODULE.FROZEN_HYDRATED_MANIFEST_SHA256,
            "scope_order": MODULE.FROZEN_SCOPE_ORDER,
            "member_count": MODULE.FROZEN_MEMBER_COUNT,
        }
        self.scope_order = (
            (7, "sku", 70, 1, (31, 32)),
            (8, "sku", 80, 2, (41,)),
        )
        MODULE.FROZEN_SCOPE_ORDER = self.scope_order
        MODULE.FROZEN_MEMBER_COUNT = 3
        MODULE.FROZEN_CANDIDATE_DIGEST = "d" * 64
        self.bodies = {
            31: b"first source",
            32: b"second source",
            41: b"third source",
        }

    def tearDown(self):
        MODULE.FROZEN_CANDIDATE_FILE_SHA256 = self.original[
            "candidate_file_sha"
        ]
        MODULE.FROZEN_CANDIDATE_DIGEST = self.original["candidate_digest"]
        MODULE.FROZEN_HYDRATED_MANIFEST_SHA256 = self.original[
            "hydrated_sha"
        ]
        MODULE.FROZEN_SCOPE_ORDER = self.original["scope_order"]
        MODULE.FROZEN_MEMBER_COUNT = self.original["member_count"]

    def write_inputs(self, root, *, mutate_candidate=None, mutate_rows=None):
        candidate = {
            "schema_version": 1,
            "status": "PROPOSED_REVIEW",
            "bundle_count": 2,
            "member_count": 3,
            "source_candidate_sha256": "d" * 64,
            "bundles": [],
        }
        hydrated = []
        for task_id, scope_kind, scope_ref_id, revision_no, ids in self.scope_order:
            members = []
            for index, task_asset_id in enumerate(ids):
                body = self.bodies[task_asset_id]
                object_key = f"tasks/{task_id}/{task_asset_id}.psd"
                digest = hashlib.sha256(body).hexdigest()
                member = {
                    "task_id": task_id,
                    "task_asset_id": task_asset_id,
                    "asset_id": 1000 + task_asset_id,
                    "asset_type": "source",
                    "storage_ref_id": f"ref-{task_asset_id}",
                    "object_key": object_key,
                    "size": len(body),
                    "sha256": digest,
                    "mime_type_from_object": "application/octet-stream",
                    "object_status": "recorded",
                    "upload_status": "uploaded",
                    "confirmed": False,
                    "original_file_name": f"{task_asset_id}.psd",
                    "source_stage": "design",
                    "evidence_event_ids": [
                        f"task_event_log:event-{task_asset_id}"
                    ],
                    "event_sequence": index + 1,
                }
                members.append(member)
                hydrated.append(
                    {
                        "entity_key": f"task_asset:{task_asset_id}",
                        "owner_kind": "task_asset",
                        "owner_id": task_asset_id,
                        "task_id": task_id,
                        "storage_ref_id": f"ref-{task_asset_id}",
                        "object_key": object_key,
                        "size": len(body),
                        "sha256": digest,
                        "mime_type": "application/octet-stream",
                        "status": "recorded",
                    }
                )
            candidate["bundles"].append(
                {
                    "task_id": task_id,
                    "scope_kind": scope_kind,
                    "scope_ref_id": scope_ref_id,
                    "revision_no": revision_no,
                    "confidence": "proposed_review",
                    "requires_human_member_confirmation": True,
                    "all_members_exist_and_hash_verified": True,
                    "ordered_members": members,
                }
            )
        if mutate_candidate:
            mutate_candidate(candidate)
        if mutate_rows:
            mutate_rows(hydrated)
        candidate_path = root / "candidate.json"
        hydrated_path = root / "hydrated.jsonl"
        candidate_path.write_text(
            json.dumps(candidate, ensure_ascii=False), encoding="utf-8"
        )
        hydrated_path.write_text(
            "".join(
                json.dumps(row, ensure_ascii=False) + "\n"
                for row in hydrated
            ),
            encoding="utf-8",
        )
        MODULE.FROZEN_CANDIDATE_FILE_SHA256 = MODULE.sha256_file(
            candidate_path
        )
        MODULE.FROZEN_HYDRATED_MANIFEST_SHA256 = MODULE.sha256_file(
            hydrated_path
        )
        return candidate_path.resolve(), hydrated_path.resolve()

    def args(self, root, candidate, hydrated):
        return type(
            "Args",
            (),
            {
                "run_root": pathlib.Path(root),
                "candidate": candidate,
                "hydrated_manifest": hydrated,
                "ssh_host": "safe-host",
                "ssh_env_file": "/safe/main.env",
                "timeout_seconds": 30.0,
            },
        )()

    def adapter_factory(self, calls, fail_id=None):
        bodies = self.bodies

        def factory(host, env_path, timeout):
            adapter = FakeAdapter(
                host, env_path, timeout, bodies, fail_id=fail_id
            )
            calls.append(adapter)
            return adapter

        return factory

    def test_cross_manifest_builds_only_exact_ordered_allowlist(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            candidate, hydrated = self.write_inputs(root)
            sources, hashes = MODULE.load_frozen_sources(
                candidate, hydrated
            )
            self.assertEqual(
                [source.task_asset_id for source in sources],
                [31, 32, 41],
            )
            self.assertEqual(
                hashes["candidate_digest"],
                "d" * 64,
            )

    def test_dynamic_scope_expansion_is_rejected_even_with_new_file_hashes(self):
        def expand(candidate):
            candidate["bundles"].append(candidate["bundles"][0])
            candidate["bundle_count"] = 3
            candidate["member_count"] = 5

        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            candidate, hydrated = self.write_inputs(
                root, mutate_candidate=expand
            )
            with self.assertRaisesRegex(ValueError, "header|scope count"):
                MODULE.load_frozen_sources(candidate, hydrated)

    def test_candidate_hydrated_identity_drift_is_rejected(self):
        def drift(rows):
            rows[0]["storage_ref_id"] = "other-ref"

        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            candidate, hydrated = self.write_inputs(
                root, mutate_rows=drift
            )
            with self.assertRaisesRegex(ValueError, "evidence drifted"):
                MODULE.load_frozen_sources(candidate, hydrated)

    def test_fetch_is_atomic_receipted_and_rerun_is_network_free(self):
        fixed = datetime.datetime(
            2026, 7, 23, 12, 0, tzinfo=datetime.timezone.utc
        )
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            candidate, hydrated = self.write_inputs(root)
            calls = []
            first = MODULE.run(
                self.args(root, candidate, hydrated),
                adapter_factory=self.adapter_factory(calls),
                now=lambda: fixed,
            )
            self.assertEqual(first["status"], "PASS")
            self.assertEqual(first["member_count"], 3)
            self.assertEqual(
                first["total_bytes"], sum(map(len, self.bodies.values()))
            )
            self.assertEqual(calls[0].calls, [31, 32, 41])
            self.assertFalse(first["production_writes_executed"])
            self.assertFalse(first["database_connections_opened"])
            for row in first["sources"]:
                target = pathlib.Path(row["source_local_path"])
                self.assertEqual(
                    target.read_bytes(), self.bodies[row["task_asset_id"]]
                )
                self.assertIn(
                    MODULE.SOURCE_DIRECTORY, target.parts
                )

            def forbidden_adapter(*_args):
                raise AssertionError("idempotent rerun must not open SSH")

            second = MODULE.run(
                self.args(root, candidate, hydrated),
                adapter_factory=forbidden_adapter,
            )
            self.assertEqual(first, second)

    def test_failure_cleans_staging_targets_and_receipt(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            candidate, hydrated = self.write_inputs(root)
            with self.assertRaisesRegex(
                MODULE.controlled.ControlledReadError,
                "synthetic failure",
            ):
                MODULE.run(
                    self.args(root, candidate, hydrated),
                    adapter_factory=self.adapter_factory([], fail_id=32),
                )
            source_root = root / MODULE.SOURCE_DIRECTORY
            self.assertEqual(
                [path for path in source_root.rglob("*") if path.is_file()],
                [],
            )
            self.assertEqual(
                list(root.glob(MODULE.STAGING_PREFIX + "*")),
                [],
            )

    def test_existing_target_without_receipt_is_rejected(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            candidate, hydrated = self.write_inputs(root)
            sources, _ = MODULE.load_frozen_sources(candidate, hydrated)
            source_root = root / MODULE.SOURCE_DIRECTORY
            source_root.mkdir()
            target = MODULE.target_path(source_root.resolve(), sources[0])
            target.parent.mkdir(parents=True)
            target.write_bytes(self.bodies[31])
            with self.assertRaisesRegex(FileExistsError, "without their exact"):
                MODULE.run(
                    self.args(root, candidate, hydrated),
                    adapter_factory=self.adapter_factory([]),
                )

    def test_symlinked_source_root_and_receipt_drift_fail_closed(self):
        with tempfile.TemporaryDirectory() as raw, tempfile.TemporaryDirectory() as outside:
            root = pathlib.Path(raw)
            candidate, hydrated = self.write_inputs(root)
            (root / MODULE.SOURCE_DIRECTORY).symlink_to(
                outside, target_is_directory=True
            )
            with self.assertRaisesRegex(ValueError, "contained real"):
                MODULE.run(
                    self.args(root, candidate, hydrated),
                    adapter_factory=self.adapter_factory([]),
                )

        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            candidate, hydrated = self.write_inputs(root)
            result = MODULE.run(
                self.args(root, candidate, hydrated),
                adapter_factory=self.adapter_factory([]),
            )
            receipt = root / MODULE.SOURCE_DIRECTORY / MODULE.RECEIPT_NAME
            value = json.loads(receipt.read_text(encoding="utf-8"))
            value["total_bytes"] += 1
            receipt.write_text(json.dumps(value), encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "invalid|hash drifted"):
                MODULE.run(
                    self.args(root, candidate, hydrated),
                    adapter_factory=self.adapter_factory([]),
                )
            self.assertEqual(result["member_count"], 3)


if __name__ == "__main__":
    unittest.main()
