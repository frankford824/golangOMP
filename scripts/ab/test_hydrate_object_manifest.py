import hashlib
import importlib.util
import io
import json
import pathlib
import struct
import sys
import tempfile
import unittest
import urllib.error
import urllib.request
from unittest import mock


VERIFIER_PATH = pathlib.Path(__file__).with_name("object_manifest_verifier.py")
VERIFIER_SPEC = importlib.util.spec_from_file_location("object_manifest_verifier", VERIFIER_PATH)
VERIFIER = importlib.util.module_from_spec(VERIFIER_SPEC)
sys.modules[VERIFIER_SPEC.name] = VERIFIER
VERIFIER_SPEC.loader.exec_module(VERIFIER)

PATH = pathlib.Path(__file__).with_name("hydrate_object_manifest.py")
SPEC = importlib.util.spec_from_file_location("hydrate_object_manifest", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class FakeResponse(io.BytesIO):
    def __init__(self, body, mime="application/octet-stream", declared_size=None):
        super().__init__(body)
        self.headers = {
            "Content-Type": mime,
            "Content-Length": str(len(body) if declared_size is None else declared_size),
        }

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        self.close()

    def getcode(self):
        return 200


class FakeAdapter:
    def __init__(self, objects, base_url="https://secret.invalid/files"):
        self.objects = objects
        self.base_url = base_url
        self.use_head = False
        self.requests = []
        self.interrupt_on = None

    def request(self, method, object_key):
        self.requests.append((method, object_key))
        if self.interrupt_on == object_key:
            raise KeyboardInterrupt()
        value = self.objects[object_key]
        if isinstance(value, BaseException):
            raise value
        if len(value) == 2:
            body, mime = value
            return FakeResponse(body, mime)
        body, mime, declared_size = value
        return FakeResponse(body, mime, declared_size)


class FeedStream:
    def __init__(self):
        self.buffer = bytearray()
        self.closed = False

    def feed(self, payload):
        self.buffer.extend(payload)

    def read(self, size=-1):
        if not self.buffer:
            return b""
        count = len(self.buffer) if size is None or size < 0 else min(size, len(self.buffer))
        payload = bytes(self.buffer[:count])
        del self.buffer[:count]
        return payload

    def close(self):
        self.closed = True


class FramedInput:
    def __init__(self, process):
        self.process = process
        self.buffer = bytearray()
        self.closed = False

    def write(self, payload):
        self.buffer.extend(payload)
        self.process.consume_requests(self.buffer)
        return len(payload)

    def flush(self):
        pass

    def close(self):
        self.closed = True


class FakeSSHProcess:
    def __init__(self, responses):
        self.responses = responses
        self.stdout = FeedStream()
        self.stdin = FramedInput(self)
        self.frames = []
        self.decisions = []
        self.returncode = None
        self.pending_body = None

    def consume_requests(self, buffer):
        while True:
            if self.pending_body is not None:
                if not buffer:
                    return
                decision = buffer.pop(0)
                self.decisions.append(decision)
                if decision == 1:
                    self.stdout.feed(self.pending_body)
                self.pending_body = None
                continue
            if len(buffer) < 4:
                return
            length = struct.unpack("!I", bytes(buffer[:4]))[0]
            if length == 0:
                del buffer[:4]
                self.returncode = 0
                return
            if len(buffer) < 4 + length:
                return
            payload = bytes(buffer[4:4 + length])
            del buffer[:4 + length]
            key = payload.decode("utf-8")
            self.frames.append(key)
            response = self.responses[key]
            if isinstance(response, bytes):
                self.stdout.feed(response)
                continue
            status, body, mime = response
            header = json.dumps(
                {"status": status, "size": len(body) if status == 200 else 0, "mime": mime},
                sort_keys=True, separators=(",", ":"),
            ).encode()
            framed = struct.pack("!I", len(header)) + header
            if status == 200:
                self.pending_body = body
            self.stdout.feed(framed)

    def poll(self):
        return self.returncode

    def wait(self, timeout=None):
        self.returncode = 0
        return 0

    def terminate(self):
        self.returncode = -15

    def kill(self):
        self.returncode = -9


class FakeSSHFactory:
    def __init__(self, responses):
        self.process = FakeSSHProcess(responses)
        self.calls = []

    def __call__(self, host, env_path, base_url, timeout):
        self.calls.append((host, env_path, base_url, timeout))
        return self.process


class FakeDirectOSSProcess:
    DEFAULT_CONFIG_FINGERPRINT = hashlib.sha256(
        b"direct-oss-test-config"
    ).hexdigest()

    def __init__(self, responses, interrupt_on=None, config_fingerprint=None):
        self.responses = responses
        self.interrupt_on = interrupt_on
        self.stdout = FeedStream()
        self.stdin = FramedInput(self)
        self.frames = []
        self.emitted = bytearray()
        self.returncode = None
        self.feed_frame({
            "config_fingerprint_sha256": (
                config_fingerprint or self.DEFAULT_CONFIG_FINGERPRINT
            ),
            "protocol": "direct-oss-sha256-v1",
        })

    def feed_frame(self, value):
        payload = json.dumps(
            value, sort_keys=True, separators=(",", ":")
        ).encode()
        framed = struct.pack("!I", len(payload)) + payload
        self.emitted.extend(framed)
        self.stdout.feed(framed)

    def consume_requests(self, buffer):
        while True:
            if len(buffer) < 4:
                return
            length = struct.unpack("!I", bytes(buffer[:4]))[0]
            if length == 0:
                del buffer[:4]
                self.returncode = 0
                return
            if len(buffer) < 4 + length:
                return
            payload = bytes(buffer[4:4 + length])
            del buffer[:4 + length]
            request = json.loads(payload)
            key = request["object_key"]
            self.frames.append((key, request["max_object_bytes"]))
            if self.interrupt_on == key:
                raise KeyboardInterrupt()
            response = self.responses[key]
            if isinstance(response, bytes):
                self.stdout.feed(response)
                self.emitted.extend(response)
                continue
            status, body, mime = response
            if status == 200 and len(body) > request["max_object_bytes"]:
                value = {
                    "detail": "declared_size_exceeds_limit",
                    "mime": "",
                    "sha256": "",
                    "size": 0,
                    "status": 413,
                }
            elif status == 200:
                value = {
                    "detail": "",
                    "mime": mime,
                    "sha256": hashlib.sha256(body).hexdigest(),
                    "size": len(body),
                    "status": 200,
                }
            else:
                value = {
                    "detail": f"http_status={status}",
                    "mime": "",
                    "sha256": "",
                    "size": 0,
                    "status": status,
                }
            self.feed_frame(value)

    def poll(self):
        return self.returncode

    def wait(self, timeout=None):
        self.returncode = 0
        return 0

    def terminate(self):
        self.returncode = -15

    def kill(self):
        self.returncode = -9


class FakeDirectOSSFactory:
    def __init__(self, responses, interrupt_on=None, config_fingerprint=None):
        self.process = FakeDirectOSSProcess(
            responses,
            interrupt_on=interrupt_on,
            config_fingerprint=config_fingerprint,
        )
        self.calls = []

    def __call__(self, host, env_path, timeout):
        self.calls.append((host, env_path, timeout))
        return self.process


class BinaryWrapper:
    def __init__(self, buffer):
        self.buffer = buffer


class FakeRemoteOpener:
    def __init__(self, body=b"x", mime="application/octet-stream"):
        self.body = body
        self.mime = mime
        self.requests = []

    def open(self, request, timeout):
        self.requests.append((request, timeout))
        return FakeResponse(self.body, self.mime)


class HydrateObjectManifestTest(unittest.TestCase):
    def row(self, owner_id, object_key, *, sha256="", size=0, mime="unknown/unknown"):
        return {
            "entity_key": f"task_asset:{owner_id}",
            "owner_kind": "task_asset",
            "owner_id": owner_id,
            "task_id": 7,
            "storage_ref_id": f"ref-{owner_id}",
            "storage_adapter": "oss_upload_service",
            "object_key": object_key,
            "size": size,
            "mime_type": mime,
            "sha256": sha256,
            "status": "recorded",
            "is_placeholder": False,
        }

    def write_manifest(self, root, rows):
        path = pathlib.Path(root) / "input.jsonl"
        path.write_text(
            "".join(json.dumps(row, sort_keys=True) + "\n" for row in rows),
            encoding="utf-8",
        )
        return path

    def run_hydrate(self, root, rows, adapter, **kwargs):
        root_path = pathlib.Path(root)
        manifest = self.write_manifest(root, rows)
        output = root_path / "hydrated.jsonl"
        checkpoint = root_path / "checkpoint.json"
        config = VERIFIER.VerifierConfig(upload=adapter)
        result = MODULE.hydrate_manifest(
            manifest, output, checkpoint, config, checkpoint_every=1, **kwargs
        )
        return result, output, checkpoint

    def test_streams_missing_hash_and_deduplicates_object_key(self):
        body = b"real bytes"
        adapter = FakeAdapter({"tasks/7/a.psd": (body, "image/vnd.adobe.photoshop")})
        existing_body = b"existing"
        rows = [
            self.row(10, "tasks/7/a.psd"),
            self.row(11, "tasks/7/a.psd"),
            self.row(
                12, "tasks/7/existing.png",
                sha256=hashlib.sha256(existing_body).hexdigest(),
                size=len(existing_body), mime="image/png",
            ),
        ]
        with tempfile.TemporaryDirectory() as root:
            result, output, checkpoint = self.run_hydrate(root, rows, adapter)
            hydrated = [
                json.loads(line)
                for line in output.read_text(encoding="utf-8").splitlines()
            ]
            checkpoint_value = json.loads(checkpoint.read_text(encoding="utf-8"))

        self.assertEqual(result["status"], "PASS")
        self.assertEqual(adapter.requests, [("GET", "tasks/7/a.psd")])
        self.assertEqual(result["missing_sha256_count"], 2)
        self.assertEqual(result["unique_target_count"], 1)
        self.assertEqual(result["deduplicated_get_count"], 1)
        for row in hydrated[:2]:
            self.assertEqual(row["sha256"], hashlib.sha256(body).hexdigest())
            self.assertEqual(row["size"], len(body))
            self.assertEqual(row["mime_type"], "image/vnd.adobe.photoshop")
        self.assertEqual(hydrated[2], rows[2])
        self.assertEqual(len(checkpoint_value["completed"]), 1)

    def test_interrupt_flushes_checkpoint_and_resume_skips_completed_get(self):
        objects = {
            "tasks/7/a.bin": (b"a", "application/octet-stream"),
            "tasks/7/b.bin": (b"b", "application/octet-stream"),
        }
        rows = [self.row(10, "tasks/7/a.bin"), self.row(11, "tasks/7/b.bin")]
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, rows)
            output = root_path / "hydrated.jsonl"
            checkpoint = root_path / "checkpoint.json"
            first_adapter = FakeAdapter(objects)
            first_adapter.interrupt_on = "tasks/7/b.bin"
            first = MODULE.hydrate_manifest(
                manifest, output, checkpoint,
                VERIFIER.VerifierConfig(upload=first_adapter), checkpoint_every=20,
            )
            self.assertEqual(first["status"], "INTERRUPTED")
            self.assertFalse(output.exists())
            second_adapter = FakeAdapter(objects)
            second = MODULE.hydrate_manifest(
                manifest, output, checkpoint,
                VERIFIER.VerifierConfig(upload=second_adapter), checkpoint_every=20,
            )

        self.assertEqual(second["status"], "PASS")
        self.assertEqual(second["resumed_target_count"], 1)
        self.assertEqual(second_adapter.requests, [("GET", "tasks/7/b.bin")])

    def test_evidence_and_checkpoint_never_record_url_token_or_body(self):
        secret_url = "https://token-user:token-pass@secret.invalid/files"
        secret_body = b"body-secret"
        # The fake adapter permits a sentinel URL so the test can prove it is
        # hashed rather than copied. Real CLI adapters reject URL userinfo.
        adapter = FakeAdapter(
            {"tasks/7/a.bin": (secret_body, "application/octet-stream")},
            base_url=secret_url,
        )
        with tempfile.TemporaryDirectory() as root:
            result, _output, checkpoint = self.run_hydrate(
                root, [self.row(10, "tasks/7/a.bin")], adapter
            )
            encoded = json.dumps(result) + checkpoint.read_text(encoding="utf-8")
        self.assertNotIn(secret_url, encoded)
        self.assertNotIn("token-user", encoded)
        self.assertNotIn("token-pass", encoded)
        self.assertNotIn(secret_body.decode(), encoded)

    def test_http_failure_is_secret_free_and_does_not_emit_partial_manifest(self):
        error = urllib.error.HTTPError(
            "https://secret.invalid/body-secret", 404, "body-secret", {}, None
        )
        adapter = FakeAdapter({"tasks/7/missing.bin": error})
        with tempfile.TemporaryDirectory() as root:
            result, output, checkpoint = self.run_hydrate(
                root, [self.row(10, "tasks/7/missing.bin")], adapter
            )
            checkpoint_text = checkpoint.read_text(encoding="utf-8")
        self.assertEqual(result["status"], "BLOCKED")
        self.assertEqual(result["failures"][0]["violation_code"], "object_manifest.missing")
        self.assertEqual(result["failures"][0]["detail"], "http_status=404")
        self.assertNotIn("secret.invalid", json.dumps(result))
        self.assertNotIn("body-secret", json.dumps(result))
        self.assertNotIn("secret.invalid", checkpoint_text)
        self.assertNotIn("body-secret", checkpoint_text)
        self.assertEqual(len(json.loads(checkpoint_text)["failed"]), 1)
        self.assertFalse(output.exists())

    def test_failure_then_interrupt_is_checkpointed_and_skipped_on_resume(self):
        missing = urllib.error.HTTPError("https://secret.invalid", 404, "missing", {}, None)
        rows = [self.row(10, "tasks/7/a.bin"), self.row(11, "tasks/7/b.bin")]
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, rows)
            output = root_path / "hydrated.jsonl"
            checkpoint = root_path / "checkpoint.json"
            first_adapter = FakeAdapter({
                "tasks/7/a.bin": missing,
                "tasks/7/b.bin": (b"b", "application/octet-stream"),
            })
            first_adapter.interrupt_on = "tasks/7/b.bin"
            first = MODULE.hydrate_manifest(
                manifest, output, checkpoint,
                VERIFIER.VerifierConfig(upload=first_adapter), checkpoint_every=20,
            )
            second_adapter = FakeAdapter({
                "tasks/7/a.bin": (b"a", "application/octet-stream"),
                "tasks/7/b.bin": (b"b", "application/octet-stream"),
            })
            second = MODULE.hydrate_manifest(
                manifest, output, checkpoint,
                VERIFIER.VerifierConfig(upload=second_adapter), checkpoint_every=20,
            )
        self.assertEqual(first["status"], "INTERRUPTED")
        self.assertEqual(second["status"], "BLOCKED")
        self.assertEqual(second["resumed_failure_target_count"], 1)
        self.assertEqual(second["read_only_get_count"], 1)
        self.assertEqual(second_adapter.requests, [("GET", "tasks/7/b.bin")])
        self.assertEqual(second["failures"][0]["detail"], "http_status=404")

    def test_checkpoint_every_counts_failures_before_ungraceful_abort(self):
        missing = urllib.error.HTTPError("https://secret.invalid", 410, "gone", {}, None)
        rows = [self.row(10, "tasks/7/a.bin"), self.row(11, "tasks/7/b.bin")]
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, rows)
            checkpoint = root_path / "checkpoint.json"
            adapter = FakeAdapter({
                "tasks/7/a.bin": missing,
                "tasks/7/b.bin": SystemExit(9),
            })
            with self.assertRaises(SystemExit):
                MODULE.hydrate_manifest(
                    manifest, root_path / "hydrated.jsonl", checkpoint,
                    VERIFIER.VerifierConfig(upload=adapter), checkpoint_every=1,
                )
            checkpoint_value = json.loads(checkpoint.read_text(encoding="utf-8"))
        self.assertEqual(checkpoint_value["schema_version"], 2)
        self.assertEqual(len(checkpoint_value["failed"]), 1)
        self.assertEqual(checkpoint_value["failed"][0]["detail"], "http_status=410")

    def test_checkpoint_is_bound_to_manifest_hash(self):
        adapter = FakeAdapter({"tasks/7/a.bin": (b"a", "application/octet-stream")})
        with tempfile.TemporaryDirectory() as root:
            result, _output, checkpoint = self.run_hydrate(
                root, [self.row(10, "tasks/7/a.bin")], adapter
            )
            self.assertEqual(result["status"], "PASS")
            manifest = self.write_manifest(root, [self.row(11, "tasks/7/a.bin")])
            with self.assertRaisesRegex(ValueError, "checkpoint does not match"):
                MODULE.hydrate_manifest(
                    manifest, pathlib.Path(root) / "second.jsonl", checkpoint,
                    VERIFIER.VerifierConfig(upload=FakeAdapter(adapter.objects)),
                )

    def test_legacy_v1_success_checkpoint_remains_resumable(self):
        objects = {"tasks/7/a.bin": (b"a", "application/octet-stream")}
        rows = [self.row(10, "tasks/7/a.bin")]
        with tempfile.TemporaryDirectory() as root:
            result, output, checkpoint = self.run_hydrate(
                root, rows, FakeAdapter(objects)
            )
            self.assertEqual(result["status"], "PASS")
            legacy = json.loads(checkpoint.read_text(encoding="utf-8"))
            legacy["schema_version"] = 1
            legacy.pop("failed")
            checkpoint.write_text(json.dumps(legacy), encoding="utf-8")
            resumed_adapter = FakeAdapter(objects)
            resumed = MODULE.hydrate_manifest(
                self.write_manifest(root, rows), output, checkpoint,
                VERIFIER.VerifierConfig(upload=resumed_adapter),
            )
        self.assertEqual(resumed["status"], "PASS")
        self.assertEqual(resumed["resumed_target_count"], 1)
        self.assertEqual(resumed_adapter.requests, [])

    def test_hydrated_output_satisfies_verifier_contract(self):
        body = b"verified bytes"
        objects = {"tasks/7/a.bin": (body, "application/octet-stream")}
        with tempfile.TemporaryDirectory() as root:
            result, output, _checkpoint = self.run_hydrate(
                root, [self.row(10, "tasks/7/a.bin")], FakeAdapter(objects)
            )
            verdict = VERIFIER.verify(
                output, VERIFIER.VerifierConfig(upload=FakeAdapter(objects))
            )
        self.assertEqual(result["status"], "PASS")
        self.assertEqual(verdict["status"], "PASS")

    def test_size_limit_blocks_without_recording_body(self):
        adapter = FakeAdapter({
            "tasks/7/large.bin": (b"body-secret", "application/octet-stream"),
        })
        with tempfile.TemporaryDirectory() as root:
            result, output, _checkpoint = self.run_hydrate(
                root, [self.row(10, "tasks/7/large.bin")], adapter,
                max_object_bytes=2,
            )
        self.assertEqual(result["status"], "BLOCKED")
        self.assertEqual(
            result["failures"][0]["violation_code"],
            "object_manifest.object_too_large",
        )
        self.assertNotIn("body-secret", json.dumps(result))
        self.assertFalse(output.exists())

    def ssh_adapter(self, responses):
        factory = FakeSSHFactory(responses)
        adapter = MODULE.PersistentSSHReadAdapter(
            "jst_ecs",
            "/root/ecommerce_ai/shared/main.env",
            "http://upload-service.internal/files",
            5,
            process_factory=factory,
        )
        return adapter, factory

    def test_persistent_ssh_adapter_uses_length_prefixed_frames_once(self):
        adapter, factory = self.ssh_adapter({
            "tasks/7/a b.bin": (200, b"a", "application/octet-stream"),
            "tasks/7/b.bin": (200, b"bb", "application/octet-stream"),
        })
        rows = [self.row(10, "tasks/7/a b.bin"), self.row(11, "tasks/7/b.bin")]
        with tempfile.TemporaryDirectory() as root:
            result, output, _checkpoint = self.run_hydrate(root, rows, adapter)
            adapter.close()
            hydrated = [
                json.loads(line)
                for line in output.read_text(encoding="utf-8").splitlines()
            ]
        self.assertEqual(result["status"], "PASS")
        self.assertEqual(len(factory.calls), 1)
        self.assertEqual(factory.process.frames, ["tasks/7/a b.bin", "tasks/7/b.bin"])
        self.assertEqual([row["size"] for row in hydrated], [1, 2])

    def test_ssh_http_error_is_mapped_without_remote_details(self):
        adapter, _factory = self.ssh_adapter({
            "tasks/7/missing.bin": (404, b"remote-body-secret", ""),
        })
        with tempfile.TemporaryDirectory() as root:
            result, output, _checkpoint = self.run_hydrate(
                root, [self.row(10, "tasks/7/missing.bin")], adapter
            )
            adapter.close()
        encoded = json.dumps(result)
        self.assertEqual(result["status"], "BLOCKED")
        self.assertEqual(result["failures"][0]["violation_code"], "object_manifest.missing")
        self.assertEqual(result["failures"][0]["detail"], "http_status=404")
        self.assertNotIn("remote-body-secret", encoded)
        self.assertNotIn("upload-service.internal", encoded)
        self.assertFalse(output.exists())

    def test_ssh_framing_failure_is_rebuilt_from_checkpoint_without_retry(self):
        malformed = struct.pack("!I", 1) + b"{"
        first_adapter, first_factory = self.ssh_adapter({
            "tasks/7/a.bin": (200, b"a", "application/octet-stream"),
            "tasks/7/b.bin": malformed,
        })
        rows = [self.row(10, "tasks/7/a.bin"), self.row(11, "tasks/7/b.bin")]
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, rows)
            output = root_path / "hydrated.jsonl"
            checkpoint = root_path / "checkpoint.json"
            first = MODULE.hydrate_manifest(
                manifest, output, checkpoint,
                VERIFIER.VerifierConfig(upload=first_adapter), checkpoint_every=20,
            )
            first_adapter.close()
            second_adapter, second_factory = self.ssh_adapter({
                "tasks/7/b.bin": (200, b"b", "application/octet-stream"),
            })
            second = MODULE.hydrate_manifest(
                manifest, output, checkpoint,
                VERIFIER.VerifierConfig(upload=second_adapter), checkpoint_every=20,
            )
            second_adapter.close()
        self.assertEqual(first["status"], "BLOCKED")
        self.assertEqual(first["failures"][0]["detail"], "read_error")
        self.assertEqual(first_factory.process.frames, ["tasks/7/a.bin", "tasks/7/b.bin"])
        self.assertEqual(second["status"], "BLOCKED")
        self.assertEqual(second["resumed_target_count"], 1)
        self.assertEqual(second["resumed_failure_target_count"], 1)
        self.assertEqual(second["failures"][0]["detail"], "read_error")
        self.assertEqual(second_factory.process.frames, [])

    def test_ssh_oversize_header_is_cancelled_before_remote_body_stream(self):
        adapter, factory = self.ssh_adapter({
            "tasks/7/large.bin": (200, b"remote-body-secret", "application/octet-stream"),
        })
        with tempfile.TemporaryDirectory() as root:
            result, output, _checkpoint = self.run_hydrate(
                root, [self.row(10, "tasks/7/large.bin")], adapter,
                max_object_bytes=2,
            )
            adapter.close()
        self.assertEqual(result["status"], "BLOCKED")
        self.assertEqual(factory.process.decisions, [0])
        self.assertEqual(bytes(factory.process.stdout.buffer), b"")
        self.assertFalse(output.exists())

    def direct_oss_adapter(
        self, responses, interrupt_on=None, config_fingerprint=None
    ):
        factory = FakeDirectOSSFactory(
            responses,
            interrupt_on=interrupt_on,
            config_fingerprint=config_fingerprint,
        )
        adapter = MODULE.PersistentSSHDirectOSSAdapter(
            "jst_ecs",
            "/root/ecommerce_ai/shared/main.env",
            5,
            process_factory=factory,
        )
        return adapter, factory

    def test_direct_oss_adapter_returns_remote_hash_without_object_body(self):
        body = b"remote-object-body-secret"
        adapter, factory = self.direct_oss_adapter({
            "tasks/7/a.bin": (200, body, "application/octet-stream"),
        })
        with tempfile.TemporaryDirectory() as root:
            result, output, checkpoint = self.run_hydrate(
                root, [self.row(10, "tasks/7/a.bin")], adapter
            )
            adapter.close()
            hydrated = json.loads(output.read_text(encoding="utf-8"))
            persisted = json.dumps(result) + checkpoint.read_text(encoding="utf-8")
        self.assertEqual(result["status"], "PASS")
        self.assertEqual(hydrated["sha256"], hashlib.sha256(body).hexdigest())
        self.assertEqual(hydrated["size"], len(body))
        self.assertEqual(hydrated["mime_type"], "application/octet-stream")
        self.assertEqual(len(factory.calls), 1)
        self.assertEqual(factory.process.frames, [("tasks/7/a.bin", MODULE.DEFAULT_MAX_OBJECT_BYTES)])
        self.assertNotIn(body, bytes(factory.process.emitted))
        self.assertNotIn(body.decode(), persisted)

    def test_direct_oss_404_and_secret_material_are_redacted(self):
        body = b"remote-error-body-secret"
        adapter, factory = self.direct_oss_adapter({
            "tasks/7/missing.bin": (404, body, "text/plain"),
        })
        with tempfile.TemporaryDirectory() as root:
            result, output, checkpoint = self.run_hydrate(
                root, [self.row(10, "tasks/7/missing.bin")], adapter
            )
            adapter.close()
            persisted = json.dumps(result) + checkpoint.read_text(encoding="utf-8")
        self.assertEqual(result["status"], "BLOCKED")
        self.assertEqual(result["failures"][0]["violation_code"], "object_manifest.missing")
        self.assertEqual(result["failures"][0]["detail"], "http_status=404")
        self.assertFalse(output.exists())
        self.assertNotIn(body, bytes(factory.process.emitted))
        self.assertNotIn(body.decode(), persisted)
        self.assertNotIn("/root/ecommerce_ai/shared/main.env", persisted)

    def test_direct_oss_framing_failure_is_checkpointed_safely(self):
        malformed = struct.pack("!I", 1) + b"{"
        adapter, _factory = self.direct_oss_adapter({
            "tasks/7/a.bin": malformed,
        })
        with tempfile.TemporaryDirectory() as root:
            result, output, checkpoint = self.run_hydrate(
                root, [self.row(10, "tasks/7/a.bin")], adapter
            )
            adapter.close()
            persisted = json.dumps(result) + checkpoint.read_text(encoding="utf-8")
        self.assertEqual(result["status"], "BLOCKED")
        self.assertEqual(result["failures"][0]["detail"], "read_error")
        self.assertFalse(output.exists())
        self.assertNotIn("invalid ssh", persisted)

    def test_direct_oss_max_bytes_blocks_before_remote_hash_result(self):
        body = b"remote-object-body-secret"
        adapter, factory = self.direct_oss_adapter({
            "tasks/7/large.bin": (200, body, "application/octet-stream"),
        })
        with tempfile.TemporaryDirectory() as root:
            result, output, _checkpoint = self.run_hydrate(
                root, [self.row(10, "tasks/7/large.bin")], adapter,
                max_object_bytes=2,
            )
            adapter.close()
        self.assertEqual(result["status"], "BLOCKED")
        self.assertEqual(
            result["failures"][0]["violation_code"],
            "object_manifest.object_too_large",
        )
        self.assertEqual(
            result["failures"][0]["detail"],
            "declared_size_exceeds_limit",
        )
        self.assertEqual(factory.process.frames, [("tasks/7/large.bin", 2)])
        self.assertNotIn(body, bytes(factory.process.emitted))
        self.assertFalse(output.exists())

    def test_direct_oss_resume_reuses_checkpoint_without_rehashing_completed(self):
        objects = {
            "tasks/7/a.bin": (200, b"a", "application/octet-stream"),
            "tasks/7/b.bin": (200, b"b", "application/octet-stream"),
        }
        rows = [self.row(10, "tasks/7/a.bin"), self.row(11, "tasks/7/b.bin")]
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, rows)
            output = root_path / "hydrated.jsonl"
            checkpoint = root_path / "checkpoint.json"
            first_adapter, first_factory = self.direct_oss_adapter(
                objects, interrupt_on="tasks/7/b.bin"
            )
            first = MODULE.hydrate_manifest(
                manifest, output, checkpoint,
                VERIFIER.VerifierConfig(upload=first_adapter),
                checkpoint_every=20,
            )
            first_adapter.close()
            second_adapter, second_factory = self.direct_oss_adapter(objects)
            second = MODULE.hydrate_manifest(
                manifest, output, checkpoint,
                VERIFIER.VerifierConfig(upload=second_adapter),
                checkpoint_every=20,
            )
            second_adapter.close()
        self.assertEqual(first["status"], "INTERRUPTED")
        self.assertEqual(first_factory.process.frames, [
            ("tasks/7/a.bin", MODULE.DEFAULT_MAX_OBJECT_BYTES),
            ("tasks/7/b.bin", MODULE.DEFAULT_MAX_OBJECT_BYTES),
        ])
        self.assertEqual(second["status"], "PASS")
        self.assertEqual(second["resumed_target_count"], 1)
        self.assertEqual(second_factory.process.frames, [
            ("tasks/7/b.bin", MODULE.DEFAULT_MAX_OBJECT_BYTES),
        ])

    def test_direct_oss_resume_rejects_remote_config_fingerprint_change(self):
        objects = {
            "tasks/7/a.bin": (200, b"a", "application/octet-stream"),
        }
        with tempfile.TemporaryDirectory() as root:
            original, _factory = self.direct_oss_adapter(objects)
            result, output, checkpoint = self.run_hydrate(
                root,
                [self.row(10, "tasks/7/a.bin")],
                original,
            )
            original.close()
            self.assertEqual(result["status"], "PASS")
            changed, _factory = self.direct_oss_adapter(
                objects,
                config_fingerprint=hashlib.sha256(
                    b"changed-direct-oss-config"
                ).hexdigest(),
            )
            with self.assertRaisesRegex(ValueError, "checkpoint does not match"):
                MODULE.hydrate_manifest(
                    self.write_manifest(root, [self.row(10, "tasks/7/a.bin")]),
                    output,
                    checkpoint,
                    VERIFIER.VerifierConfig(upload=changed),
                )
            changed.close()

    def test_direct_oss_helper_and_command_contain_no_config_values(self):
        compile(MODULE.REMOTE_DIRECT_OSS_HELPER, "<remote-direct-oss-helper>", "exec")
        command = MODULE.remote_direct_oss_command(
            "/root/ecommerce_ai/shared/main.env", 30
        )
        self.assertNotIn("OSS_ACCESS_KEY_ID=", command)
        self.assertNotIn("OSS_ACCESS_KEY_SECRET=", command)
        self.assertNotIn("OSS_ENDPOINT=", command)
        self.assertNotIn("OSS_BUCKET=", command)

    def test_ssh_configuration_rejects_shell_injection_shapes(self):
        with self.assertRaisesRegex(ValueError, "host alias"):
            MODULE.PersistentSSHReadAdapter(
                "jst_ecs;id", "/root/main.env", "http://upload/files", 5
            )
        with self.assertRaisesRegex(ValueError, "POSIX path"):
            MODULE.PersistentSSHReadAdapter(
                "jst_ecs", "/root/../main.env", "http://upload/files", 5
            )
        with self.assertRaisesRegex(ValueError, "query"):
            MODULE.PersistentSSHReadAdapter(
                "jst_ecs", "/root/main.env", "http://upload/files?token=x", 5
            )
        with self.assertRaisesRegex(ValueError, "host alias"):
            MODULE.PersistentSSHDirectOSSAdapter(
                "jst_ecs;id", "/root/main.env", 5
            )
        with self.assertRaisesRegex(ValueError, "POSIX path"):
            MODULE.PersistentSSHDirectOSSAdapter(
                "jst_ecs", "/root/../main.env", 5
            )

    def test_remote_helper_compiles_and_command_contains_no_token_value(self):
        compile(MODULE.REMOTE_UPLOAD_HELPER, "<remote-upload-helper>", "exec")
        command = MODULE.remote_command(
            "/root/ecommerce_ai/shared/main.env",
            "http://upload-service.internal/files",
            30,
        )
        self.assertNotIn("UPLOAD_SERVICE_INTERNAL_TOKEN=", command)

    def execute_remote_helper(self, env_text):
        key = b"tasks/7/a.bin"
        stdin = io.BytesIO(
            struct.pack("!I", len(key)) + key + b"\x01" + struct.pack("!I", 0)
        )
        stdout = io.BytesIO()
        opener = FakeRemoteOpener()
        with (
            mock.patch.object(pathlib.Path, "open", return_value=io.StringIO(env_text)),
            mock.patch.object(urllib.request, "build_opener", return_value=opener),
            mock.patch.object(sys, "stdin", BinaryWrapper(stdin)),
            mock.patch.object(sys, "stdout", BinaryWrapper(stdout)),
            mock.patch.object(
                sys, "argv",
                ["helper", "/root/ecommerce_ai/shared/main.env", "http://upload/files", "5"],
            ),
        ):
            exec(MODULE.REMOTE_UPLOAD_HELPER, {"__name__": "__main__"})
        return opener, stdout.getvalue()

    def test_remote_helper_sends_internal_token_and_explicit_storage_provider(self):
        opener, output = self.execute_remote_helper(
            "UPLOAD_SERVICE_INTERNAL_TOKEN=token-secret\n"
            "UPLOAD_STORAGE_PROVIDER=nas\n"
        )
        request, timeout = opener.requests[0]
        headers = {key.lower(): value for key, value in request.header_items()}
        self.assertEqual(timeout, 5)
        self.assertEqual(headers["x-internal-token"], "token-secret")
        self.assertEqual(headers["x-storage-provider"], "nas")
        self.assertNotIn("authorization", headers)
        self.assertNotIn(b"token-secret", output)
        self.assertNotIn(b"nas", output)

    def test_remote_helper_defaults_storage_provider_to_oss(self):
        opener, output = self.execute_remote_helper(
            "UPLOAD_SERVICE_INTERNAL_TOKEN=token-secret\n"
        )
        request, _timeout = opener.requests[0]
        headers = {key.lower(): value for key, value in request.header_items()}
        self.assertEqual(headers["x-storage-provider"], "oss")
        self.assertNotIn(b"token-secret", output)
        self.assertNotIn(b"oss", output)

    def test_remote_helper_rejects_unsafe_storage_provider(self):
        with self.assertRaisesRegex(ValueError, "invalid storage provider"):
            self.execute_remote_helper(
                "UPLOAD_SERVICE_INTERNAL_TOKEN=token-secret\n"
                "UPLOAD_STORAGE_PROVIDER=oss;unsafe\n"
            )


if __name__ == "__main__":
    unittest.main()
