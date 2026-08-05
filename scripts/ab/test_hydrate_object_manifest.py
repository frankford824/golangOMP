import hashlib
import importlib.util
import io
import json
import pathlib
import struct
import sys
import tempfile
import threading
import unittest
import urllib.error
import urllib.request
from concurrent.futures.thread import BrokenThreadPool
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


class FakeParallelAdapter:
    def __init__(self, objects, state=None):
        self.objects = objects
        self.base_url = "parallel-origin"
        self.use_head = False
        self.state = state or {
            "barrier": threading.Barrier(len(objects)),
            "clones": 0,
            "threads": set(),
            "closed": 0,
            "lock": threading.Lock(),
        }

    def origin_fingerprint(self):
        return hashlib.sha256(b"parallel-origin").hexdigest()

    def clone(self):
        with self.state["lock"]:
            self.state["clones"] += 1
        return FakeParallelAdapter(self.objects, self.state)

    def get_metadata(self, object_key, _max_object_bytes):
        with self.state["lock"]:
            self.state["threads"].add(threading.current_thread().name)
        self.state["barrier"].wait(timeout=5)
        body, mime = self.objects[object_key]
        return MODULE.ObjectMetadata(
            size=len(body),
            mime_type=mime,
            sha256=hashlib.sha256(body).hexdigest(),
        )

    def close(self):
        with self.state["lock"]:
            self.state["closed"] += 1


class BrokenParallelAdapter:
    def __init__(self, *, clone=False):
        self.base_url = "broken-parallel-origin"
        self.use_head = False
        self.is_clone = clone

    def origin_fingerprint(self):
        if self.is_clone:
            raise BrokenThreadPool("worker adapter could not initialize")
        return hashlib.sha256(b"broken-parallel-origin").hexdigest()

    def clone(self):
        return BrokenParallelAdapter(clone=True)

    def close(self):
        pass


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

    def __call__(self, host, env_path, timeout, endpoint_override):
        self.calls.append((host, env_path, timeout, endpoint_override))
        return self.process


class CloningFakeDirectOSSFactory:
    def __init__(self, responses, config_fingerprint=None):
        self.responses = responses
        self.config_fingerprint = config_fingerprint
        self.calls = []
        self.processes = []

    def __call__(self, host, env_path, timeout, endpoint_override):
        self.calls.append((host, env_path, timeout, endpoint_override))
        process = FakeDirectOSSProcess(
            self.responses,
            config_fingerprint=self.config_fingerprint,
        )
        self.processes.append(process)
        return process


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


class FakeRangeResponse(FakeResponse):
    def __init__(self, body, mime, *, code, content_range=""):
        super().__init__(body, mime)
        self.code = code
        if content_range:
            self.headers["Content-Range"] = content_range

    def getcode(self):
        return self.code


class TruncatedRemoteOpener:
    def __init__(self, body, truncate_at, mime="application/octet-stream"):
        self.body = body
        self.truncate_at = truncate_at
        self.mime = mime
        self.requests = []

    def open(self, request, timeout):
        self.requests.append((request, timeout))
        range_header = request.headers.get("Range", "")
        if not range_header:
            return FakeResponse(
                self.body[:self.truncate_at],
                self.mime,
                declared_size=len(self.body),
            )
        prefix = "bytes="
        start_raw, end_raw = range_header.removeprefix(prefix).split("-", 1)
        start = int(start_raw)
        end = min(int(end_raw), len(self.body) - 1)
        return FakeRangeResponse(
            self.body[start:end + 1],
            self.mime,
            code=206,
            content_range=f"bytes {start}-{end}/{len(self.body)}",
        )


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

    def historical_unavailable_row(self):
        contract = MODULE.historical_unavailable_exception
        return {
            "entity_key": contract.ENTITY_KEY,
            "owner_kind": "task_asset",
            "owner_id": contract.TASK_ASSET_ID,
            "task_id": contract.TASK_ID,
            "storage_ref_id": contract.EXPECTED_STORAGE_REF_ID,
            "storage_adapter": contract.EXPECTED_STORAGE_ADAPTER,
            "object_key": contract.EXPECTED_OBJECT_KEY,
            "size": contract.EXPECTED_SIZE,
            "mime_type": contract.EXPECTED_MIME_TYPE,
            "sha256": "",
            "status": contract.EXPECTED_STATUS,
            "is_placeholder": False,
        }

    def write_historical_unavailable_attestation(self, root, manifest):
        contract = MODULE.historical_unavailable_exception
        row = self.historical_unavailable_row()
        mapping_sha = "1" * 64
        mapping_row_hash = "2" * 64
        exception = {
            "entity_key": contract.ENTITY_KEY,
            "owner_kind": "task_asset",
            "owner_id": contract.TASK_ASSET_ID,
            "task_id": contract.TASK_ID,
            "missing_task_asset_id": contract.TASK_ASSET_ID,
            "strategy": contract.STRATEGY,
            "policy_id": contract.POLICY_ID,
            "expected_http_status": contract.EXPECTED_HTTP_STATUS,
            "storage_ref_id": contract.EXPECTED_STORAGE_REF_ID,
            "object_row_sha256": contract.canonical_hash(row),
            "mapping_row_hash": mapping_row_hash,
            "expected_file_size": contract.EXPECTED_SIZE,
            "object_probe_result": contract.EXPECTED_PROBE_RESULT,
            "object_probe_read_only_get_count": (
                contract.EXPECTED_PROBE_READ_ONLY_GET_COUNT
            ),
            "object_probe_evidence_hash": contract.EXPECTED_PROBE_EVIDENCE_HASH,
            "object_probe_input_manifest_sha256": (
                contract.EXPECTED_PROBE_INPUT_MANIFEST_SHA256
            ),
            "object_probe_object_key_sha256": (
                contract.EXPECTED_PROBE_OBJECT_KEY_SHA256
            ),
            "working_reference_count": 0,
            "finalized_reference_count": 0,
        }
        attestation = {
            "schema_version": contract.SCHEMA_VERSION,
            "status": "PASS",
            "exception_count": 1,
            "mapping_sha256": mapping_sha,
            "mapping_row_hash": mapping_row_hash,
            "object_manifest_sha256": VERIFIER.sha256_file(manifest),
            "sql_evidence_sha256": "3" * 64,
            "api_evidence_sha256": "4" * 64,
            "exceptions": [exception],
        }
        attestation["evidence_hash"] = contract.self_hash(attestation)
        path = pathlib.Path(root) / "historical-unavailable-exception.json"
        path.write_text(
            contract.canonical_json(attestation) + "\n",
            encoding="utf-8",
        )
        return path, attestation

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

    def write_retry_authorization(
        self,
        root,
        manifest,
        checkpoint,
        failure_record,
        row,
        body=b"reprobe-stable",
    ):
        root_path = pathlib.Path(root)
        reprobes = []
        for index in (1, 2):
            output = root_path / f"reprobe-{index}.jsonl"
            evidence_path = root_path / f"reprobe-{index}.evidence.json"
            result = MODULE.hydrate_manifest(
                manifest,
                output,
                root_path / f"reprobe-{index}.checkpoint.json",
                VERIFIER.VerifierConfig(
                    upload=FakeAdapter({
                        row["object_key"]: (body, "application/octet-stream"),
                    })
                ),
            )
            self.assertEqual(result["status"], "PASS")
            MODULE.atomic_write_json(evidence_path, result)
            reprobes.append({
                "evidence_path": evidence_path.name,
                "evidence_sha256": VERIFIER.sha256_file(evidence_path),
                "artifact_path": output.name,
                "artifact_sha256": VERIFIER.sha256_file(output),
            })
        payload = {
            "schema_version": MODULE.FAILURE_RETRY_AUTHORIZATION_SCHEMA_VERSION,
            "authorization_type": MODULE.FAILURE_RETRY_AUTHORIZATION_TYPE,
            "input_manifest_sha256": VERIFIER.sha256_file(manifest),
            "checkpoint_sha256": VERIFIER.sha256_file(checkpoint),
            "failure_retries": [{
                "failure_record_sha256": (
                    MODULE.checkpoint_failure_record_sha256(failure_record)
                ),
                "reprobes": reprobes,
            }],
        }
        authorization_sha = hashlib.sha256(
            VERIFIER.canonical_json(payload).encode("utf-8")
        ).hexdigest()
        authorization = dict(payload)
        authorization["authorization_sha256"] = authorization_sha
        path = root_path / "failure-retry-authorization.json"
        MODULE.atomic_write_json(path, authorization)
        return path, authorization_sha

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

    def test_exact_historical_unavailable_exception_skips_only_attested_row(self):
        historical = self.historical_unavailable_row()
        ordinary = self.row(10, "tasks/7/a.bin")
        body = b"ordinary"
        adapter = FakeAdapter({
            ordinary["object_key"]: (body, "application/octet-stream"),
        })
        with tempfile.TemporaryDirectory() as root:
            rows = [historical, ordinary]
            manifest = self.write_manifest(root, rows)
            attestation_path, attestation = (
                self.write_historical_unavailable_attestation(root, manifest)
            )
            output = pathlib.Path(root) / "hydrated.jsonl"
            result = MODULE.hydrate_manifest(
                manifest,
                output,
                pathlib.Path(root) / "checkpoint.json",
                VERIFIER.VerifierConfig(upload=adapter),
                historical_unavailable_exception_path=attestation_path,
            )
            hydrated = [
                json.loads(line)
                for line in output.read_text(encoding="utf-8").splitlines()
            ]
            attestation_sha = VERIFIER.sha256_file(attestation_path)

        self.assertEqual(result["status"], "PASS")
        self.assertEqual(adapter.requests, [("GET", ordinary["object_key"])])
        self.assertEqual(hydrated[0], historical)
        self.assertEqual(hydrated[1]["sha256"], hashlib.sha256(body).hexdigest())
        self.assertEqual(
            result["historical_unavailable_exception_attestation_sha256"],
            attestation_sha,
        )
        self.assertEqual(
            result["historical_unavailable_exception_mapping_sha256"],
            attestation["mapping_sha256"],
        )
        self.assertEqual(
            result["historical_unavailable_exception_mapping_row_hash"],
            attestation["mapping_row_hash"],
        )
        self.assertEqual(
            result["historical_unavailable_exception_count"],
            1,
        )
        self.assertEqual(result["configured_target_row_count"], 1)
        self.assertEqual(result["read_only_get_count"], 1)
        self.assertEqual(result["hydrated_row_count"], 1)

    def test_exception_prunes_only_its_sticky_404_from_existing_checkpoint(self):
        historical = self.historical_unavailable_row()
        ordinary = self.row(10, "tasks/7/a.bin")
        missing = urllib.error.HTTPError(
            "https://secret.invalid", 404, "missing", {}, io.BytesIO()
        )
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            rows = [historical, ordinary]
            manifest = self.write_manifest(root, rows)
            checkpoint = root_path / "checkpoint.json"
            first_adapter = FakeAdapter({
                historical["object_key"]: missing,
                ordinary["object_key"]: (b"ordinary", "application/octet-stream"),
            })
            first = MODULE.hydrate_manifest(
                manifest,
                root_path / "first.jsonl",
                checkpoint,
                VERIFIER.VerifierConfig(upload=first_adapter),
                checkpoint_every=1,
            )
            self.assertEqual(first["status"], "BLOCKED")
            attestation_path, _attestation = (
                self.write_historical_unavailable_attestation(root, manifest)
            )
            resumed_adapter = FakeAdapter({})
            output = root_path / "hydrated.jsonl"
            resumed = MODULE.hydrate_manifest(
                manifest,
                output,
                checkpoint,
                VERIFIER.VerifierConfig(upload=resumed_adapter),
                historical_unavailable_exception_path=attestation_path,
            )
            hydrated = [
                json.loads(line)
                for line in output.read_text(encoding="utf-8").splitlines()
            ]
            checkpoint_value = json.loads(checkpoint.read_text(encoding="utf-8"))

        self.assertEqual(resumed["status"], "PASS")
        self.assertEqual(resumed["resumed_target_count"], 1)
        self.assertEqual(resumed["resumed_failure_target_count"], 0)
        self.assertEqual(resumed_adapter.requests, [])
        self.assertEqual(hydrated[0], historical)
        self.assertEqual(checkpoint_value["failed"], [])
        self.assertEqual(len(checkpoint_value["completed"]), 1)

    def test_exception_does_not_exempt_any_other_empty_sha_row(self):
        historical = self.historical_unavailable_row()
        ordinary = self.row(10, "tasks/7/a.bin")
        missing = urllib.error.HTTPError(
            "https://secret.invalid", 404, "missing", {}, io.BytesIO()
        )
        adapter = FakeAdapter({ordinary["object_key"]: missing})
        with tempfile.TemporaryDirectory() as root:
            manifest = self.write_manifest(root, [historical, ordinary])
            attestation_path, _attestation = (
                self.write_historical_unavailable_attestation(root, manifest)
            )
            result = MODULE.hydrate_manifest(
                manifest,
                pathlib.Path(root) / "hydrated.jsonl",
                pathlib.Path(root) / "checkpoint.json",
                VERIFIER.VerifierConfig(upload=adapter),
                historical_unavailable_exception_path=attestation_path,
            )

        self.assertEqual(result["status"], "BLOCKED")
        self.assertEqual(adapter.requests, [("GET", ordinary["object_key"])])
        self.assertEqual(result["failure_count"], 1)

    def test_invalid_exception_fails_before_any_remote_read(self):
        historical = self.historical_unavailable_row()
        adapter = FakeAdapter({
            historical["object_key"]: (b"unexpected", "application/octet-stream"),
        })
        with tempfile.TemporaryDirectory() as root:
            manifest = self.write_manifest(root, [historical])
            attestation_path, attestation = (
                self.write_historical_unavailable_attestation(root, manifest)
            )
            attestation["mapping_row_hash"] = "9" * 64
            attestation_path.write_text(json.dumps(attestation), encoding="utf-8")
            with self.assertRaisesRegex(
                ValueError, "historical-unavailable attestation"
            ):
                MODULE.hydrate_manifest(
                    manifest,
                    pathlib.Path(root) / "hydrated.jsonl",
                    pathlib.Path(root) / "checkpoint.json",
                    VERIFIER.VerifierConfig(upload=adapter),
                    historical_unavailable_exception_path=attestation_path,
                )
        self.assertEqual(adapter.requests, [])

    def test_cli_accepts_exact_historical_unavailable_exception(self):
        historical = self.historical_unavailable_row()
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, [historical])
            attestation_path, attestation = (
                self.write_historical_unavailable_attestation(root, manifest)
            )
            output = root_path / "hydrated.jsonl"
            evidence = root_path / "evidence.json"
            checkpoint = root_path / "checkpoint.json"
            exit_code = MODULE.main([
                str(manifest),
                str(output),
                str(evidence),
                "--checkpoint", str(checkpoint),
                "--historical-unavailable-exception", str(attestation_path),
            ])
            result = json.loads(evidence.read_text(encoding="utf-8"))
            hydrated = json.loads(output.read_text(encoding="utf-8"))

        self.assertEqual(exit_code, 0)
        self.assertEqual(result["status"], "PASS")
        self.assertEqual(result["read_only_get_count"], 0)
        self.assertEqual(result["historical_unavailable_exception_count"], 1)
        self.assertEqual(
            result["historical_unavailable_exception_mapping_sha256"],
            attestation["mapping_sha256"],
        )
        self.assertEqual(hydrated, historical)

    def test_cli_rejects_exception_path_collision_without_mutating_manifest(self):
        historical = self.historical_unavailable_row()
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, [historical])
            original = manifest.read_bytes()
            evidence = root_path / "evidence.json"
            exit_code = MODULE.main([
                str(manifest),
                str(root_path / "hydrated.jsonl"),
                str(evidence),
                "--checkpoint", str(root_path / "checkpoint.json"),
                "--historical-unavailable-exception", str(manifest),
            ])
            result = json.loads(evidence.read_text(encoding="utf-8"))
            persisted = manifest.read_bytes()

        self.assertEqual(exit_code, 1)
        self.assertEqual(result["status"], "BLOCKED")
        self.assertIn("paths must differ", result["failures"][0]["detail"])
        self.assertEqual(persisted, original)

    def test_cli_does_not_overwrite_exception_when_it_collides_with_evidence(self):
        historical = self.historical_unavailable_row()
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, [historical])
            attestation_path, _attestation = (
                self.write_historical_unavailable_attestation(root, manifest)
            )
            original = attestation_path.read_bytes()
            exit_code = MODULE.main([
                str(manifest),
                str(root_path / "hydrated.jsonl"),
                str(attestation_path),
                "--checkpoint", str(root_path / "checkpoint.json"),
                "--historical-unavailable-exception", str(attestation_path),
            ])
            persisted = attestation_path.read_bytes()

        self.assertEqual(exit_code, 1)
        self.assertEqual(persisted, original)

    def test_parallel_cloneable_adapter_hashes_with_independent_workers(self):
        objects = {
            f"tasks/7/{name}.bin": (name.encode(), "application/octet-stream")
            for name in ("a", "b", "c", "d")
        }
        adapter = FakeParallelAdapter(objects)
        rows = [
            self.row(index + 10, object_key)
            for index, object_key in enumerate(objects)
        ]
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, rows)
            output = root_path / "hydrated.jsonl"
            checkpoint = root_path / "checkpoint.json"
            result = MODULE.hydrate_manifest(
                manifest,
                output,
                checkpoint,
                VERIFIER.VerifierConfig(upload=adapter),
                checkpoint_every=1,
                workers=4,
            )
        self.assertEqual("PASS", result["status"])
        self.assertEqual(4, result["read_only_get_count"])
        self.assertEqual(4, adapter.state["clones"])
        self.assertEqual(4, adapter.state["closed"])
        self.assertEqual(4, len(adapter.state["threads"]))

    def test_broken_parallel_initializer_finishes_as_interrupted(self):
        rows = [self.row(10, "tasks/7/a.bin")]
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            result = MODULE.hydrate_manifest(
                self.write_manifest(root, rows),
                root_path / "hydrated.jsonl",
                root_path / "checkpoint.json",
                VERIFIER.VerifierConfig(upload=BrokenParallelAdapter()),
                workers=2,
            )

        self.assertEqual(result["status"], "INTERRUPTED")
        self.assertEqual(result["failure_count"], 0)
        self.assertEqual(result["read_only_get_count"], 0)

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
            "https://secret.invalid/body-secret", 404, "body-secret", {},
            io.BytesIO(),
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
        missing = urllib.error.HTTPError(
            "https://secret.invalid", 404, "missing", {}, io.BytesIO()
        )
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

    def test_explicit_retry_rehydrates_checkpointed_timeout(self):
        rows = [self.row(10, "tasks/7/a.bin")]
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, rows)
            output = root_path / "hydrated.jsonl"
            checkpoint = root_path / "checkpoint.json"
            first = MODULE.hydrate_manifest(
                manifest,
                output,
                checkpoint,
                VERIFIER.VerifierConfig(
                    upload=FakeAdapter({"tasks/7/a.bin": TimeoutError()})
                ),
            )
            second_adapter = FakeAdapter({
                "tasks/7/a.bin": (b"a", "application/octet-stream"),
            })
            second = MODULE.hydrate_manifest(
                manifest,
                output,
                checkpoint,
                VERIFIER.VerifierConfig(upload=second_adapter),
                retry_transient_failures=True,
            )

        self.assertEqual(first["status"], "BLOCKED")
        self.assertEqual(first["failures"][0]["detail"], "timeout")
        self.assertEqual(second["status"], "PASS")
        self.assertEqual(second["retried_transient_failure_target_count"], 1)
        self.assertEqual(second["resumed_failure_target_count"], 0)
        self.assertEqual(second_adapter.requests, [("GET", "tasks/7/a.bin")])

    def test_explicit_retry_keeps_http_failure_sticky(self):
        missing = urllib.error.HTTPError(
            "https://secret.invalid", 404, "missing", {}, io.BytesIO()
        )
        rows = [self.row(10, "tasks/7/a.bin")]
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, rows)
            output = root_path / "hydrated.jsonl"
            checkpoint = root_path / "checkpoint.json"
            MODULE.hydrate_manifest(
                manifest,
                output,
                checkpoint,
                VERIFIER.VerifierConfig(
                    upload=FakeAdapter({"tasks/7/a.bin": missing})
                ),
            )
            second_adapter = FakeAdapter({
                "tasks/7/a.bin": (b"a", "application/octet-stream"),
            })
            second = MODULE.hydrate_manifest(
                manifest,
                output,
                checkpoint,
                VERIFIER.VerifierConfig(upload=second_adapter),
                retry_transient_failures=True,
            )

        self.assertEqual(second["status"], "BLOCKED")
        self.assertEqual(second["retried_transient_failure_target_count"], 0)
        self.assertEqual(second["resumed_failure_target_count"], 1)
        self.assertEqual(second_adapter.requests, [])
        self.assertEqual(second["failures"][0]["detail"], "http_status=404")

    def test_content_failure_stays_sticky_without_exact_authorization(self):
        rows = [self.row(10, "tasks/7/a.bin")]
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, rows)
            output = root_path / "hydrated.jsonl"
            checkpoint = root_path / "checkpoint.json"
            first = MODULE.hydrate_manifest(
                manifest,
                output,
                checkpoint,
                VERIFIER.VerifierConfig(
                    upload=FakeAdapter({
                        "tasks/7/a.bin": (
                            b"short",
                            "application/octet-stream",
                            100,
                        ),
                    })
                ),
            )
            second_adapter = FakeAdapter({
                "tasks/7/a.bin": (b"stable", "application/octet-stream"),
            })
            second = MODULE.hydrate_manifest(
                manifest,
                output,
                checkpoint,
                VERIFIER.VerifierConfig(upload=second_adapter),
                retry_transient_failures=True,
            )

        self.assertEqual(first["status"], "BLOCKED")
        self.assertEqual(
            first["failures"][0]["detail"],
            "content_length_differs_from_stream",
        )
        self.assertEqual(second["status"], "BLOCKED")
        self.assertEqual(second["retried_authorized_failure_target_count"], 0)
        self.assertEqual(
            second["failure_retry_authorization_sha256"],
            MODULE.ZERO_SHA256,
        )
        self.assertEqual(second_adapter.requests, [])

    def test_exact_authorization_retries_one_content_failure_after_two_reprobes(self):
        rows = [self.row(10, "tasks/7/a.bin")]
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, rows)
            output = root_path / "hydrated.jsonl"
            checkpoint = root_path / "checkpoint.json"
            MODULE.hydrate_manifest(
                manifest,
                output,
                checkpoint,
                VERIFIER.VerifierConfig(
                    upload=FakeAdapter({
                        "tasks/7/a.bin": (
                            b"short",
                            "application/octet-stream",
                            100,
                        ),
                    })
                ),
            )
            failure_record = json.loads(
                checkpoint.read_text(encoding="utf-8")
            )["failed"][0]
            authorization, authorization_sha = self.write_retry_authorization(
                root, manifest, checkpoint, failure_record, rows[0]
            )
            second_adapter = FakeAdapter({
                "tasks/7/a.bin": (
                    b"reprobe-stable",
                    "application/octet-stream",
                ),
            })
            second = MODULE.hydrate_manifest(
                manifest,
                output,
                checkpoint,
                VERIFIER.VerifierConfig(upload=second_adapter),
                failure_retry_authorization_path=authorization,
            )

        self.assertEqual(second["status"], "PASS")
        self.assertEqual(second["retried_authorized_failure_target_count"], 1)
        self.assertEqual(
            second["failure_retry_authorization_sha256"],
            authorization_sha,
        )
        self.assertEqual(second["resumed_failure_target_count"], 0)
        self.assertEqual(second_adapter.requests, [("GET", "tasks/7/a.bin")])

    def test_authorization_rejects_wrong_self_hash_before_any_get(self):
        rows = [self.row(10, "tasks/7/a.bin")]
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, rows)
            output = root_path / "hydrated.jsonl"
            checkpoint = root_path / "checkpoint.json"
            MODULE.hydrate_manifest(
                manifest,
                output,
                checkpoint,
                VERIFIER.VerifierConfig(
                    upload=FakeAdapter({
                        "tasks/7/a.bin": (
                            b"short",
                            "application/octet-stream",
                            100,
                        ),
                    })
                ),
            )
            failure_record = json.loads(
                checkpoint.read_text(encoding="utf-8")
            )["failed"][0]
            authorization, _authorization_sha = self.write_retry_authorization(
                root, manifest, checkpoint, failure_record, rows[0]
            )
            value = json.loads(authorization.read_text(encoding="utf-8"))
            value["authorization_sha256"] = "f" * 64
            MODULE.atomic_write_json(authorization, value)
            adapter = FakeAdapter({
                "tasks/7/a.bin": (b"reprobe-stable", "application/octet-stream"),
            })
            with self.assertRaisesRegex(ValueError, "self-hash mismatch"):
                MODULE.hydrate_manifest(
                    manifest,
                    output,
                    checkpoint,
                    VERIFIER.VerifierConfig(upload=adapter),
                    failure_retry_authorization_path=authorization,
                )

        self.assertEqual(adapter.requests, [])

    def test_authorization_is_bound_to_exact_checkpoint_bytes(self):
        rows = [self.row(10, "tasks/7/a.bin")]
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, rows)
            output = root_path / "hydrated.jsonl"
            checkpoint = root_path / "checkpoint.json"
            MODULE.hydrate_manifest(
                manifest,
                output,
                checkpoint,
                VERIFIER.VerifierConfig(
                    upload=FakeAdapter({
                        "tasks/7/a.bin": (
                            b"short",
                            "application/octet-stream",
                            100,
                        ),
                    })
                ),
            )
            failure_record = json.loads(
                checkpoint.read_text(encoding="utf-8")
            )["failed"][0]
            authorization, _authorization_sha = self.write_retry_authorization(
                root, manifest, checkpoint, failure_record, rows[0]
            )
            checkpoint.write_text(
                checkpoint.read_text(encoding="utf-8") + " ",
                encoding="utf-8",
            )
            adapter = FakeAdapter({
                "tasks/7/a.bin": (b"reprobe-stable", "application/octet-stream"),
            })
            with self.assertRaisesRegex(ValueError, "checkpoint mismatch"):
                MODULE.hydrate_manifest(
                    manifest,
                    output,
                    checkpoint,
                    VERIFIER.VerifierConfig(upload=adapter),
                    failure_retry_authorization_path=authorization,
                )

        self.assertEqual(adapter.requests, [])

    def test_authorization_rejects_changed_reprobe_artifact(self):
        rows = [self.row(10, "tasks/7/a.bin")]
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, rows)
            output = root_path / "hydrated.jsonl"
            checkpoint = root_path / "checkpoint.json"
            MODULE.hydrate_manifest(
                manifest,
                output,
                checkpoint,
                VERIFIER.VerifierConfig(
                    upload=FakeAdapter({
                        "tasks/7/a.bin": (
                            b"short",
                            "application/octet-stream",
                            100,
                        ),
                    })
                ),
            )
            failure_record = json.loads(
                checkpoint.read_text(encoding="utf-8")
            )["failed"][0]
            authorization, _authorization_sha = self.write_retry_authorization(
                root, manifest, checkpoint, failure_record, rows[0]
            )
            (root_path / "reprobe-2.jsonl").write_text(
                "{}\n", encoding="utf-8"
            )
            adapter = FakeAdapter({
                "tasks/7/a.bin": (b"reprobe-stable", "application/octet-stream"),
            })
            with self.assertRaisesRegex(ValueError, "artifact hash mismatch"):
                MODULE.hydrate_manifest(
                    manifest,
                    output,
                    checkpoint,
                    VERIFIER.VerifierConfig(upload=adapter),
                    failure_retry_authorization_path=authorization,
                )

        self.assertEqual(adapter.requests, [])

    def test_authorized_retry_failure_replaces_checkpoint_failure(self):
        rows = [self.row(10, "tasks/7/a.bin")]
        with tempfile.TemporaryDirectory() as root:
            root_path = pathlib.Path(root)
            manifest = self.write_manifest(root, rows)
            output = root_path / "hydrated.jsonl"
            checkpoint = root_path / "checkpoint.json"
            MODULE.hydrate_manifest(
                manifest,
                output,
                checkpoint,
                VERIFIER.VerifierConfig(
                    upload=FakeAdapter({
                        "tasks/7/a.bin": (
                            b"short",
                            "application/octet-stream",
                            100,
                        ),
                    })
                ),
            )
            old_failure = json.loads(
                checkpoint.read_text(encoding="utf-8")
            )["failed"][0]
            authorization, authorization_sha = self.write_retry_authorization(
                root, manifest, checkpoint, old_failure, rows[0]
            )
            adapter = FakeAdapter({
                "tasks/7/a.bin": TimeoutError(),
            })
            result = MODULE.hydrate_manifest(
                manifest,
                output,
                checkpoint,
                VERIFIER.VerifierConfig(upload=adapter),
                failure_retry_authorization_path=authorization,
            )
            new_failure = json.loads(
                checkpoint.read_text(encoding="utf-8")
            )["failed"][0]

        self.assertEqual(result["status"], "BLOCKED")
        self.assertEqual(result["retried_authorized_failure_target_count"], 1)
        self.assertEqual(
            result["failure_retry_authorization_sha256"],
            authorization_sha,
        )
        self.assertEqual(adapter.requests, [("GET", "tasks/7/a.bin")])
        self.assertEqual(new_failure["detail"], "timeout")
        self.assertNotEqual(new_failure, old_failure)

    def test_checkpoint_every_counts_failures_before_ungraceful_abort(self):
        missing = urllib.error.HTTPError(
            "https://secret.invalid", 410, "gone", {}, io.BytesIO()
        )
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
        self,
        responses,
        interrupt_on=None,
        config_fingerprint=None,
        endpoint_override="",
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
            endpoint_override,
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

    def test_direct_oss_endpoint_override_reuses_existing_checkpoint(self):
        objects = {
            "tasks/7/a.bin": (200, b"a", "application/octet-stream"),
        }
        with tempfile.TemporaryDirectory() as root:
            original, _original_factory = self.direct_oss_adapter(objects)
            first, output, checkpoint = self.run_hydrate(
                root,
                [self.row(10, "tasks/7/a.bin")],
                original,
            )
            original.close()
            internal, internal_factory = self.direct_oss_adapter(
                objects,
                endpoint_override=(
                    "oss-cn-hangzhou-internal.aliyuncs.com"
                ),
            )
            second = MODULE.hydrate_manifest(
                self.write_manifest(root, [self.row(10, "tasks/7/a.bin")]),
                output,
                checkpoint,
                VERIFIER.VerifierConfig(upload=internal),
            )
            internal.close()
            persisted = (
                json.dumps(second)
                + checkpoint.read_text(encoding="utf-8")
            )
        self.assertEqual(first["status"], "PASS")
        self.assertEqual(second["status"], "PASS")
        self.assertEqual(second["resumed_target_count"], 1)
        self.assertEqual(internal_factory.process.frames, [])
        self.assertNotIn(
            "oss-cn-hangzhou-internal.aliyuncs.com", persisted
        )

    def test_direct_oss_clone_and_parallel_workers_preserve_override(self):
        objects = {
            "tasks/7/a.bin": (200, b"a", "application/octet-stream"),
            "tasks/7/b.bin": (200, b"b", "application/octet-stream"),
        }
        endpoint_override = "oss-cn-hangzhou-internal.aliyuncs.com"
        factory = CloningFakeDirectOSSFactory(objects)
        adapter = MODULE.PersistentSSHDirectOSSAdapter(
            "jst_ecs",
            "/root/ecommerce_ai/shared/main.env",
            5,
            endpoint_override,
            process_factory=factory,
        )
        clone = adapter.clone()
        self.assertEqual(clone.endpoint_override, endpoint_override)
        clone.close()
        with tempfile.TemporaryDirectory() as root:
            result, _output, checkpoint = self.run_hydrate(
                root,
                [
                    self.row(10, "tasks/7/a.bin"),
                    self.row(11, "tasks/7/b.bin"),
                ],
                adapter,
                workers=2,
            )
            adapter.close()
            persisted = (
                json.dumps(result)
                + checkpoint.read_text(encoding="utf-8")
            )
        self.assertEqual(result["status"], "PASS")
        self.assertGreaterEqual(len(factory.calls), 2)
        self.assertTrue(
            all(call[3] == endpoint_override for call in factory.calls)
        )
        self.assertNotIn(endpoint_override, persisted)

    def test_direct_oss_helper_and_command_contain_no_config_values(self):
        compile(MODULE.REMOTE_DIRECT_OSS_HELPER, "<remote-direct-oss-helper>", "exec")
        command = MODULE.remote_direct_oss_command(
            "/root/ecommerce_ai/shared/main.env", 30
        )
        self.assertNotIn("OSS_ACCESS_KEY_ID=", command)
        self.assertNotIn("OSS_ACCESS_KEY_SECRET=", command)
        self.assertNotIn("OSS_ENDPOINT=", command)
        self.assertNotIn("OSS_BUCKET=", command)

    def test_direct_oss_cli_requires_explicit_valid_override(self):
        args = MODULE.parse_args([
            "input.jsonl",
            "output.jsonl",
            "evidence.json",
            "--checkpoint", "checkpoint.json",
            "--ssh-direct-oss",
            "--ssh-host", "jst_ecs",
            "--ssh-env-file", "/root/main.env",
            "--ssh-direct-oss-endpoint-override",
            "oss-cn-hangzhou-internal.aliyuncs.com",
        ])
        adapter = MODULE.upload_adapter_from_args(args)
        self.assertIsInstance(
            adapter, MODULE.PersistentSSHDirectOSSAdapter
        )
        self.assertEqual(
            adapter.endpoint_override,
            "oss-cn-hangzhou-internal.aliyuncs.com",
        )
        with self.assertRaisesRegex(ValueError, "requires --ssh-direct-oss"):
            MODULE.upload_adapter_from_args(
                MODULE.parse_args([
                    "input.jsonl",
                    "output.jsonl",
                    "evidence.json",
                    "--checkpoint", "checkpoint.json",
                    "--ssh-direct-oss-endpoint-override",
                    "oss-cn-hangzhou-internal.aliyuncs.com",
                ])
            )

    def test_direct_oss_override_rejects_non_bare_or_arbitrary_hosts_locally(self):
        unsafe = (
            "https://oss-cn-hangzhou-internal.aliyuncs.com",
            "user@oss-cn-hangzhou-internal.aliyuncs.com",
            "oss-cn-hangzhou-internal.aliyuncs.com/path",
            "oss-cn-hangzhou-internal.aliyuncs.com?x=1",
            "example.com",
            "oss-cn-hangzhou.aliyuncs.com",
        )
        for value in unsafe:
            with self.subTest(value=value):
                with self.assertRaisesRegex(
                    ValueError, "bare Aliyun internal regional endpoint"
                ):
                    MODULE.PersistentSSHDirectOSSAdapter(
                        "jst_ecs", "/root/main.env", 5, value
                    )

    def execute_direct_oss_helper(self, env_text, endpoint_override, opener=None):
        request = json.dumps(
            {
                "max_object_bytes": 1024,
                "object_key": "tasks/7/a.bin",
            },
            sort_keys=True,
            separators=(",", ":"),
        ).encode("utf-8")
        stdin = io.BytesIO(
            struct.pack("!I", len(request))
            + request
            + struct.pack("!I", 0)
        )
        stdout = io.BytesIO()
        if opener is None:
            opener = FakeRemoteOpener(
                body=b"direct-body",
                mime="application/octet-stream",
            )
        with (
            mock.patch.object(
                pathlib.Path,
                "open",
                return_value=io.StringIO(env_text),
            ),
            mock.patch.object(
                urllib.request, "build_opener", return_value=opener
            ),
            mock.patch.object(sys, "stdin", BinaryWrapper(stdin)),
            mock.patch.object(sys, "stdout", BinaryWrapper(stdout)),
            mock.patch.object(
                sys,
                "argv",
                [
                    "helper",
                    "/root/ecommerce_ai/shared/main.env",
                    "5",
                    endpoint_override,
                ],
            ),
        ):
            exec(
                MODULE.REMOTE_DIRECT_OSS_HELPER,
                {"__name__": "__main__"},
            )
        output = stdout.getvalue()
        frames = []
        offset = 0
        while offset < len(output):
            length = struct.unpack("!I", output[offset:offset + 4])[0]
            offset += 4
            frames.append(
                json.loads(output[offset:offset + length].decode("utf-8"))
            )
            offset += length
        return opener, frames, output

    def test_direct_oss_helper_resumes_cleanly_truncated_get_with_ranges(self):
        env_text = (
            "OSS_ACCESS_KEY_ID=id\n"
            "OSS_ACCESS_KEY_SECRET=secret\n"
            "OSS_BUCKET=bucket\n"
            "OSS_ENDPOINT=oss-cn-hangzhou.aliyuncs.com\n"
            "UPLOAD_STORAGE_PROVIDER=oss\n"
        )
        body = b"direct-body-with-a-truncated-prefix"
        opener = TruncatedRemoteOpener(body, truncate_at=7)
        used_opener, frames, _output = self.execute_direct_oss_helper(
            env_text,
            "",
            opener,
        )
        self.assertIs(used_opener, opener)
        self.assertEqual(frames[1]["status"], 200)
        self.assertEqual(frames[1]["size"], len(body))
        self.assertEqual(
            frames[1]["sha256"],
            hashlib.sha256(body).hexdigest(),
        )
        self.assertGreaterEqual(len(opener.requests), 2)
        self.assertEqual(
            opener.requests[1][0].headers.get("Range"),
            f"bytes=7-{len(body) - 1}",
        )

    def test_direct_oss_helper_uses_only_same_region_internal_route(self):
        env_text = (
            "OSS_ACCESS_KEY_ID=access-key-secret\n"
            "OSS_ACCESS_KEY_SECRET=signing-secret\n"
            "OSS_BUCKET=test-bucket\n"
            "OSS_ENDPOINT=oss-cn-hangzhou.aliyuncs.com\n"
            "UPLOAD_STORAGE_PROVIDER=oss\n"
        )
        public_opener, public_frames, _public_output = (
            self.execute_direct_oss_helper(env_text, "")
        )
        internal_opener, internal_frames, internal_output = (
            self.execute_direct_oss_helper(
                env_text,
                "oss-cn-hangzhou-internal.aliyuncs.com",
            )
        )
        self.assertEqual(
            public_frames[0]["config_fingerprint_sha256"],
            internal_frames[0]["config_fingerprint_sha256"],
        )
        self.assertEqual(
            urllib.parse.urlsplit(
                public_opener.requests[0][0].full_url
            ).hostname,
            "test-bucket.oss-cn-hangzhou.aliyuncs.com",
        )
        self.assertEqual(
            urllib.parse.urlsplit(
                internal_opener.requests[0][0].full_url
            ).hostname,
            "test-bucket.oss-cn-hangzhou-internal.aliyuncs.com",
        )
        self.assertNotIn(b"access-key-secret", internal_output)
        self.assertNotIn(b"signing-secret", internal_output)
        self.assertNotIn(b"oss-cn-hangzhou", internal_output)

    def test_direct_oss_helper_rejects_cross_region_and_non_exact_source(self):
        base = (
            "OSS_ACCESS_KEY_ID=id\n"
            "OSS_ACCESS_KEY_SECRET=secret\n"
            "OSS_BUCKET=bucket\n"
            "UPLOAD_STORAGE_PROVIDER=oss\n"
        )
        with self.assertRaisesRegex(ValueError, "same-region"):
            self.execute_direct_oss_helper(
                base + "OSS_ENDPOINT=oss-cn-hangzhou.aliyuncs.com\n",
                "oss-cn-shanghai-internal.aliyuncs.com",
            )
        for source in (
            "https://oss-cn-hangzhou.aliyuncs.com",
            "oss-cn-hangzhou.aliyuncs.com/path",
            "oss-cn-hangzhou-internal.aliyuncs.com",
        ):
            with self.subTest(source=source):
                with self.assertRaisesRegex(
                    ValueError, "exact public regional endpoint"
                ):
                    self.execute_direct_oss_helper(
                        base + f"OSS_ENDPOINT={source}\n",
                        "oss-cn-hangzhou-internal.aliyuncs.com",
                    )

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
