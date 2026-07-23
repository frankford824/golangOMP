import hashlib
import importlib.util
import json
import os
import pathlib
import sys
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from unittest import mock


PATH = pathlib.Path(__file__).with_name("object_manifest_verifier.py")
SPEC = importlib.util.spec_from_file_location("object_manifest_verifier", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class ObjectServer:
    def __init__(self, body=b"hello object", mime="application/octet-stream", status=200, redirect=False):
        self.body = body
        self.mime = mime
        self.status = status
        self.redirect = redirect
        self.requests = []
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def _respond(self, include_body):
                owner.requests.append((self.command, self.path, dict(self.headers)))
                if owner.redirect:
                    self.send_response(302); self.send_header("Location", "/elsewhere"); self.end_headers(); return
                self.send_response(owner.status)
                self.send_header("Content-Type", owner.mime)
                self.send_header("Content-Length", str(len(owner.body)))
                self.send_header("ETag", '"not-a-sha256"')
                self.end_headers()
                if include_body and owner.status < 300:
                    self.wfile.write(owner.body)

            def do_GET(self):
                self._respond(True)

            def do_HEAD(self):
                self._respond(False)

            def log_message(self, _format, *_args):
                pass

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)

    @property
    def url(self):
        return f"http://127.0.0.1:{self.server.server_port}/files"

    def __enter__(self):
        self.thread.start(); return self

    def __exit__(self, *_args):
        self.server.shutdown(); self.thread.join(); self.server.server_close()


class ObjectManifestVerifierTest(unittest.TestCase):
    def valid_row(self, body=b"hello object", adapter="oss_upload_service"):
        return {
            "entity_key": "task_asset:10", "owner_kind": "task_asset", "owner_id": 10,
            "task_id": 7, "storage_ref_id": "ref-10", "storage_adapter": adapter,
            "object_key": "tasks/7/a b.psd", "size": len(body),
            "mime_type": "application/octet-stream", "sha256": hashlib.sha256(body).hexdigest(),
            "status": "active", "is_placeholder": False,
        }

    def write_manifest(self, root, *rows):
        path = pathlib.Path(root) / "objects.jsonl"
        path.write_text("".join(json.dumps(row) + "\n" for row in rows), encoding="utf-8")
        return path

    def test_valid_contract_remains_adapter_blocked(self):
        with tempfile.TemporaryDirectory() as root:
            result = MODULE.verify(self.write_manifest(root, self.valid_row()))
        self.assertEqual(result["status"], "BLOCKED")
        self.assertEqual(result["checked_count"], 0)
        self.assertEqual(result["violations"][0]["violation_code"], "object_manifest.adapter_not_configured")

    def test_upload_get_stream_passes_and_does_not_use_etag(self):
        body = b"hello object"
        with ObjectServer(body=body) as server, tempfile.TemporaryDirectory() as root:
            path = self.write_manifest(root, self.valid_row(body))
            config = MODULE.VerifierConfig(upload=MODULE.HTTPReadAdapter(server.url, {}, 5))
            result = MODULE.verify(path, config)
        self.assertEqual(result["status"], "PASS")
        self.assertEqual(result["violation_count"], 0)
        self.assertEqual(result["checked_count"], 1)
        self.assertRegex(result["manifest_sha256"], r"^[0-9a-f]{64}$")
        self.assertRegex(result["evidence_hash"], r"^[0-9a-f]{64}$")
        self.assertEqual(server.requests[0][0:2], ("GET", "/files/tasks/7/a%20b.psd"))

    def test_oss_uses_head_then_get(self):
        body = b"oss"
        with ObjectServer(body=body) as server, tempfile.TemporaryDirectory() as root:
            path = self.write_manifest(root, self.valid_row(body, "oss"))
            config = MODULE.VerifierConfig(oss=MODULE.HTTPReadAdapter(server.url, {}, 5, use_head=True))
            result = MODULE.verify(path, config)
        self.assertEqual(result["status"], "PASS")
        self.assertEqual([item[0] for item in server.requests], ["HEAD", "GET"])

    def test_sha_mismatch_is_fail_even_when_etag_is_present(self):
        body = b"actual"
        row = self.valid_row(body); row["sha256"] = "a" * 64
        with ObjectServer(body=body) as server, tempfile.TemporaryDirectory() as root:
            config = MODULE.VerifierConfig(upload=MODULE.HTTPReadAdapter(server.url, {}, 5))
            result = MODULE.verify(self.write_manifest(root, row), config)
        self.assertEqual(result["status"], "FAIL")
        self.assertEqual(result["violations"][0]["violation_code"], "object_manifest.sha256_mismatch")

    def test_oversize_stream_is_blocked_and_not_fully_hashed(self):
        body = b"too large"
        row = self.valid_row(body); row["size"] = 2
        with ObjectServer(body=body) as server, tempfile.TemporaryDirectory() as root:
            config = MODULE.VerifierConfig(upload=MODULE.HTTPReadAdapter(server.url, {}, 5))
            result = MODULE.verify(self.write_manifest(root, row), config)
        self.assertEqual(result["status"], "BLOCKED")
        self.assertEqual(result["checked_count"], 0)
        self.assertEqual({v["violation_code"] for v in result["violations"]}, {
            "object_manifest.size_mismatch", "object_manifest.sha256_unverified",
        })

    def test_confirmed_missing_object_is_secret_free_failure(self):
        secret = "do-not-leak"
        with ObjectServer(status=404) as server, tempfile.TemporaryDirectory() as root:
            adapter = MODULE.HTTPReadAdapter(server.url, {"Authorization": f"Bearer {secret}"}, 5)
            result = MODULE.verify(self.write_manifest(root, self.valid_row()), MODULE.VerifierConfig(upload=adapter))
        encoded = json.dumps(result)
        self.assertEqual(result["status"], "FAIL")
        self.assertNotIn(secret, encoded)
        self.assertNotIn(server.url, encoded)
        self.assertEqual(result["violations"][0]["violation_code"], "object_manifest.missing")
        self.assertEqual(result["violations"][0]["detail"], "http_status=404")

    def test_redirect_is_not_followed(self):
        with ObjectServer(redirect=True) as server, tempfile.TemporaryDirectory() as root:
            adapter = MODULE.HTTPReadAdapter(server.url, {}, 5)
            result = MODULE.verify(self.write_manifest(root, self.valid_row()), MODULE.VerifierConfig(upload=adapter))
        self.assertEqual(result["status"], "BLOCKED")
        self.assertEqual(len(server.requests), 1)
        self.assertEqual(result["violations"][0]["detail"], "http_status=302")

    def test_headers_file_and_environment_token_are_applied_without_output_leak(self):
        with ObjectServer() as server, tempfile.TemporaryDirectory() as root:
            headers_path = pathlib.Path(root) / "headers.json"
            headers_path.write_text(json.dumps({"X-Internal-ID": "header-secret"}), encoding="utf-8")
            args = MODULE.parse_args([
                str(self.write_manifest(root, self.valid_row())), str(pathlib.Path(root) / "out.json"),
                "--upload-base-url", server.url, "--upload-headers-file", str(headers_path),
            ])
            with mock.patch.dict(os.environ, {"AB_UPLOAD_READ_BEARER_TOKEN": "token-secret"}, clear=False):
                adapter = MODULE.adapter_from_args("upload", args)
                result = MODULE.verify(pathlib.Path(args.manifest_jsonl), MODULE.VerifierConfig(upload=adapter))
        self.assertEqual(result["status"], "PASS")
        request_headers = server.requests[0][2]
        self.assertEqual(request_headers["X-Internal-Id"], "header-secret")
        self.assertEqual(request_headers["Authorization"], "Bearer token-secret")
        encoded = json.dumps(result)
        self.assertNotIn("header-secret", encoded)
        self.assertNotIn("token-secret", encoded)

    def test_old_storage_key_contract_is_rejected(self):
        row = self.valid_row(); row["storage_key"] = row.pop("object_key")
        self.assertEqual(MODULE.validate_row(row, 1)[0]["violation_code"], "object_manifest.invalid")

    def test_placeholder_never_passes(self):
        row = self.valid_row(); row["is_placeholder"] = True
        self.assertEqual(MODULE.validate_row(row, 1)[0]["violation_code"], "object_manifest.placeholder")

    def test_duplicate_entity_is_blocked_before_network_read(self):
        with ObjectServer() as server, tempfile.TemporaryDirectory() as root:
            row = self.valid_row()
            config = MODULE.VerifierConfig(upload=MODULE.HTTPReadAdapter(server.url, {}, 5))
            result = MODULE.verify(self.write_manifest(root, row, row), config)
        self.assertEqual(result["status"], "BLOCKED")
        self.assertEqual(result["checked_count"], 1)
        self.assertEqual(len(server.requests), 1)
        self.assertIn("object_manifest.duplicate", {v["violation_code"] for v in result["violations"]})

    def test_distinct_manifest_entities_reuse_identical_verified_object(self):
        with ObjectServer() as server, tempfile.TemporaryDirectory() as root:
            first = self.valid_row()
            second = dict(first, entity_key="task_asset:11", owner_id=11, storage_ref_id="ref-11")
            config = MODULE.VerifierConfig(upload=MODULE.HTTPReadAdapter(server.url, {}, 5))
            result = MODULE.verify(self.write_manifest(root, first, second), config)
        self.assertEqual(result["status"], "PASS")
        self.assertEqual(result["checked_count"], 2)
        self.assertEqual(len(server.requests), 1)

    def test_evidence_is_deterministic(self):
        with ObjectServer() as server, tempfile.TemporaryDirectory() as root:
            path = self.write_manifest(root, self.valid_row())
            config = MODULE.VerifierConfig(upload=MODULE.HTTPReadAdapter(server.url, {}, 5))
            first = MODULE.verify(path, config); second = MODULE.verify(path, config)
        self.assertEqual(first, second)


if __name__ == "__main__":
    unittest.main()
