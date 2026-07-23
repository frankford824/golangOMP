import datetime
import hashlib
import importlib.util
import io
import json
import pathlib
import struct
import sys
import tempfile
import unittest


PATH = pathlib.Path(__file__).with_name("fetch_asset_recovery_sources.py")
SPEC = importlib.util.spec_from_file_location("fetch_asset_recovery_sources", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


def frame(value):
    encoded = MODULE.canonical_bytes(value)
    return struct.pack("!I", len(encoded)) + encoded


class FakeProcess:
    def __init__(self, stdout):
        self.stdin = io.BytesIO()
        self.stdout = io.BytesIO(stdout)
        self.returncode = None

    def poll(self):
        return self.returncode

    def wait(self, timeout=None):
        self.returncode = 0
        return 0

    def terminate(self):
        self.returncode = -15

    def kill(self):
        self.returncode = -9


class FakeAdapter:
    def __init__(self, _host, _env_path, _timeout, fail_id=None):
        self.fail_id = fail_id
        self.calls = []
        self.closed = False

    def origin_fingerprint(self):
        return "a" * 64

    def fetch_to_path(self, source, target):
        self.calls.append(source.task_asset_id)
        if source.task_asset_id == self.fail_id:
            target.write_bytes(b"partial")
            raise MODULE.ControlledReadError("synthetic read failure")
        target.write_bytes(bytes.fromhex(source.sha256) + b"x")
        body = target.read_bytes()
        if len(body) != source.size or hashlib.sha256(body).hexdigest() != source.sha256:
            # Unit tests do not have the production bytes. Replace the frozen
            # source tuple in the individual test with matching synthetic data.
            raise AssertionError("test source was not replaced with synthetic data")

    def close(self):
        self.closed = True


class FetchAssetRecoverySourcesTest(unittest.TestCase):
    def setUp(self):
        self.original_sources = MODULE.FROZEN_SOURCES
        synthetic = []
        for index, source in enumerate(self.original_sources):
            body = bytes([index + 11]) * (100 + index)
            synthetic.append(
                MODULE.FrozenSource(
                    missing_task_asset_id=source.missing_task_asset_id,
                    task_asset_id=source.task_asset_id,
                    storage_ref_id=source.storage_ref_id,
                    object_key=source.object_key,
                    size=len(body),
                    mime_type=source.mime_type,
                    sha256=hashlib.sha256(body).hexdigest(),
                )
            )
        MODULE.FROZEN_SOURCES = tuple(synthetic)
        self.bodies = {
            source.task_asset_id: bytes([index + 11]) * source.size
            for index, source in enumerate(MODULE.FROZEN_SOURCES)
        }

    def tearDown(self):
        MODULE.FROZEN_SOURCES = self.original_sources

    def args(self, root):
        return type(
            "Args",
            (),
            {
                "run_root": pathlib.Path(root),
                "ssh_host": "223.4.249.11",
                "ssh_env_file": "/root/ecommerce_ai/shared/main.env",
                "timeout_seconds": 30.0,
            },
        )()

    def adapter_factory(self, fail_id=None, close_failure=False):
        bodies = self.bodies

        class Adapter(FakeAdapter):
            def __init__(self, host, env_path, timeout):
                super().__init__(host, env_path, timeout, fail_id)

            def fetch_to_path(self, source, target):
                self.calls.append(source.task_asset_id)
                if source.task_asset_id == self.fail_id:
                    target.write_bytes(b"partial")
                    raise MODULE.ControlledReadError("synthetic read failure")
                target.write_bytes(bodies[source.task_asset_id])

            def close(self):
                if close_failure:
                    raise MODULE.ControlledReadError("synthetic close failure")
                super().close()

        return Adapter

    def test_fetches_exact_allowlist_and_emits_prepare_compatible_receipts(self):
        fixed_now = datetime.datetime(2026, 7, 23, 12, 0, tzinfo=datetime.timezone.utc)
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            result = MODULE.run(
                self.args(root),
                adapter_factory=self.adapter_factory(),
                now=lambda: fixed_now,
            )
            self.assertEqual(result["status"], "PASS")
            self.assertFalse(result["production_writes_executed"])
            self.assertFalse(result["database_connections_opened"])
            self.assertEqual(len(result["recoveries"]), 3)
            encoded = json.dumps(result)
            self.assertNotIn("ecommerce_ai", encoded)
            self.assertNotIn("223.4.249.11", encoded)
            for source, row in zip(MODULE.FROZEN_SOURCES, result["recoveries"]):
                path = pathlib.Path(row["source_local_path"])
                self.assertEqual(path.parent, root / "source-assets")
                self.assertEqual(path.read_bytes(), self.bodies[source.task_asset_id])
                receipt = row["source_fetch_receipt"]
                self.assertEqual(receipt["protocol"], MODULE.PROTOCOL)
                self.assertEqual(receipt["storage_ref_id"], source.storage_ref_id)
                self.assertEqual(receipt["object_key"], source.object_key)
                self.assertEqual(receipt["fetched_at"], "2026-07-23T12:00:00Z")

    def test_failure_removes_all_files_created_by_this_invocation(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            failing_id = MODULE.FROZEN_SOURCES[1].task_asset_id
            with self.assertRaisesRegex(
                MODULE.ControlledReadError, "synthetic read failure"
            ):
                MODULE.run(
                    self.args(root),
                    adapter_factory=self.adapter_factory(failing_id),
                )
            source_dir = root / "source-assets"
            self.assertEqual(list(source_dir.iterdir()), [])

    def test_transport_close_failure_removes_verified_files_and_evidence(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            with self.assertRaisesRegex(
                MODULE.ControlledReadError, "synthetic close failure"
            ):
                MODULE.run(
                    self.args(root),
                    adapter_factory=self.adapter_factory(close_failure=True),
                )
            self.assertEqual(list((root / "source-assets").iterdir()), [])

    def test_exact_evidence_makes_rerun_network_free(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            first = MODULE.run(
                self.args(root), adapter_factory=self.adapter_factory()
            )

            def forbidden_adapter(*_args):
                raise AssertionError("idempotent rerun must not open SSH")

            second = MODULE.run(self.args(root), adapter_factory=forbidden_adapter)
            self.assertEqual(first, second)

    def test_existing_file_without_receipt_is_rejected(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw)
            source_dir = root / "source-assets"
            source_dir.mkdir()
            source = MODULE.FROZEN_SOURCES[0]
            (source_dir / source.file_name).write_bytes(self.bodies[source.task_asset_id])
            with self.assertRaisesRegex(FileExistsError, "without their exact"):
                MODULE.run(
                    self.args(root), adapter_factory=self.adapter_factory()
                )

    def test_source_assets_symlink_escape_is_rejected(self):
        with tempfile.TemporaryDirectory() as raw, tempfile.TemporaryDirectory() as outside:
            root = pathlib.Path(raw)
            (root / "source-assets").symlink_to(outside, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "contained real directory"):
                MODULE.run(
                    self.args(root), adapter_factory=self.adapter_factory()
                )

    def test_transport_streams_only_after_exact_metadata_ack(self):
        source = MODULE.FROZEN_SOURCES[0]
        hello = frame(
            {
                "config_fingerprint_sha256": "b" * 64,
                "protocol": MODULE.PROTOCOL,
            }
        )
        header = frame(
            {
                "detail": "",
                "mime": source.mime_type,
                "size": source.size,
                "status": 200,
            }
        )
        process = FakeProcess(hello + header + self.bodies[source.task_asset_id])
        adapter = MODULE.SSHControlledReadAdapter(
            "223.4.249.11",
            "/root/ecommerce_ai/shared/main.env",
            30,
            process_factory=lambda *_args: process,
        )
        with tempfile.TemporaryDirectory() as raw:
            target = pathlib.Path(raw) / "object.tmp"
            adapter.fetch_to_path(source, target)
            self.assertEqual(target.read_bytes(), self.bodies[source.task_asset_id])
        sent = process.stdin.getvalue()
        request_size = struct.unpack("!I", sent[:4])[0]
        request = json.loads(sent[4 : 4 + request_size])
        self.assertEqual(
            request,
            {"max_object_bytes": source.size, "object_key": source.object_key},
        )
        self.assertEqual(sent[4 + request_size :], b"\x01")

    def test_transport_rejects_header_drift_without_stream_ack(self):
        source = MODULE.FROZEN_SOURCES[0]
        hello = frame(
            {
                "config_fingerprint_sha256": "b" * 64,
                "protocol": MODULE.PROTOCOL,
            }
        )
        header = frame(
            {
                "detail": "",
                "mime": source.mime_type,
                "size": source.size + 1,
                "status": 200,
            }
        )
        process = FakeProcess(hello + header)
        adapter = MODULE.SSHControlledReadAdapter(
            "safe-host",
            "/safe/env.file",
            30,
            process_factory=lambda *_args: process,
        )
        with tempfile.TemporaryDirectory() as raw:
            with self.assertRaisesRegex(
                MODULE.ControlledReadError, "metadata differs"
            ):
                adapter.fetch_to_path(source, pathlib.Path(raw) / "object.tmp")
        self.assertTrue(process.stdin.getvalue().endswith(b"\x00"))

    def test_remote_helper_compiles_and_ssh_inputs_reject_injection(self):
        compile(MODULE.REMOTE_HELPER, "<controlled-read-helper>", "exec")
        with self.assertRaisesRegex(ValueError, "simple configured host"):
            MODULE.validate_ssh_host("host;touch /tmp/pwn")
        with self.assertRaisesRegex(ValueError, "simple absolute POSIX"):
            MODULE.validate_remote_env_path("/root/../secret")


if __name__ == "__main__":
    unittest.main()
