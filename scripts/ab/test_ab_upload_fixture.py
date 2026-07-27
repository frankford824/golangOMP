#!/usr/bin/env python3
from __future__ import annotations

import hashlib
import http.client
import json
import pathlib
import tempfile
import threading
import unittest
import urllib.parse

from ab_upload_fixture import FixtureHandler, FixtureHTTPServer, FixtureStore


class RunningFixture:
    def __init__(self, root: pathlib.Path, *, read_only: bool = False, mode: str = "upload", seed: pathlib.Path | None = None):
        store = FixtureStore(root, f"test:{mode}:{'ro' if read_only else 'rw'}", mode, read_only, seed, 1024 * 1024, "")
        self.server = FixtureHTTPServer(("127.0.0.1", 0), FixtureHandler)
        self.port = self.server.server_address[1]
        store.public_base_url = f"http://127.0.0.1:{self.port}"
        self.server.store = store
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def close(self) -> None:
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)

    def request(self, method: str, path: str, body: bytes = b"", headers: dict[str, str] | None = None):
        conn = http.client.HTTPConnection("127.0.0.1", self.port, timeout=3)
        conn.request(method, path, body=body, headers=headers or {})
        response = conn.getresponse()
        data = response.read()
        conn.close()
        return response.status, dict(response.getheaders()), data


class ABUploadFixtureTest(unittest.TestCase):
    @staticmethod
    def multipart_body(fields: dict[str, str], filename: str, mime_type: str, data: bytes) -> tuple[str, bytes]:
        boundary = "ab-fixture-boundary"
        chunks: list[bytes] = []
        for key, value in fields.items():
            chunks.append(
                f"--{boundary}\r\nContent-Disposition: form-data; name=\"{key}\"\r\n\r\n{value}\r\n".encode()
            )
        chunks.append(
            f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; filename=\"{filename}\"\r\nContent-Type: {mime_type}\r\n\r\n".encode()
            + data
            + b"\r\n"
        )
        chunks.append(f"--{boundary}--\r\n".encode())
        return f"multipart/form-data; boundary={boundary}", b"".join(chunks)

    def test_small_raw_session_complete_and_escaped_read(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            fixture = RunningFixture(pathlib.Path(tmp))
            self.addCleanup(fixture.close)
            body = "你好-v8".encode()
            create = json.dumps({
                "task_id": 7, "task_ref": "RW-7", "asset_type": "delivery", "file_role": "delivery",
                "version_no": 1, "upload_mode": "small", "filename": "成 品.png",
                "expected_size": len(body), "mime_type": "image/png", "created_by": 1,
            }).encode()
            status, _, raw = fixture.request("POST", "/upload/sessions", create, {"Content-Type": "application/json"})
            self.assertEqual(status, 201, raw)
            plan = json.loads(raw)
            upload_id = plan["upload_id"]
            status, _, raw = fixture.request("PUT", f"/upload/sessions/{upload_id}/file", body, {"Content-Type": "image/png"})
            self.assertEqual(status, 200, raw)
            checksum = hashlib.sha256(body).hexdigest()
            complete = json.dumps({"expected_size": len(body), "mime_type": "image/png", "checksum_hint": checksum}).encode()
            status, _, raw = fixture.request("POST", f"/upload/sessions/{upload_id}/complete", complete, {"Content-Type": "application/json"})
            self.assertEqual(status, 200, raw)
            meta = json.loads(raw)
            self.assertEqual(meta["file_hash"], checksum)
            escaped = "/".join(urllib.parse.quote(part, safe="") for part in meta["storage_key"].split("/"))
            status, headers, returned = fixture.request("GET", f"/files/{escaped}")
            self.assertEqual((status, returned), (200, body))
            self.assertEqual(headers["ETag"], f'"{checksum}"')
            log = (pathlib.Path(tmp) / "events.jsonl").read_text(encoding="utf-8")
            self.assertNotIn("token", log.lower())

    def test_size_mime_and_checksum_are_fail_closed(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            fixture = RunningFixture(pathlib.Path(tmp))
            self.addCleanup(fixture.close)
            create = json.dumps({"upload_mode": "small", "filename": "x.bin", "expected_size": 3, "mime_type": "application/octet-stream"}).encode()
            _, _, raw = fixture.request("POST", "/upload/sessions", create, {"Content-Type": "application/json"})
            upload_id = json.loads(raw)["upload_id"]
            status, _, _ = fixture.request("PUT", f"/upload/sessions/{upload_id}/file", b"xx", {"Content-Type": "application/octet-stream"})
            self.assertEqual(status, 422)
            status, _, _ = fixture.request("PUT", f"/upload/sessions/{upload_id}/file", b"xxx", {"Content-Type": "text/plain"})
            self.assertEqual(status, 422)
            status, _, _ = fixture.request("PUT", f"/upload/sessions/{upload_id}/file", b"xxx", {"Content-Type": "application/octet-stream"})
            self.assertEqual(status, 200)
            complete = json.dumps({"checksum_hint": "0" * 64}).encode()
            status, _, _ = fixture.request("POST", f"/upload/sessions/{upload_id}/complete", complete, {"Content-Type": "application/json"})
            self.assertEqual(status, 422)

    def test_multipart_parts_are_combined_in_order(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            fixture = RunningFixture(pathlib.Path(tmp))
            self.addCleanup(fixture.close)
            create = json.dumps({"upload_mode": "multipart", "filename": "x.psd", "expected_size": 6, "mime_type": "application/octet-stream"}).encode()
            _, _, raw = fixture.request("POST", "/upload/sessions", create, {"Content-Type": "application/json"})
            upload_id = json.loads(raw)["upload_id"]
            for number, body in ((1, b"abc"), (2, b"def")):
                prepare = json.dumps({"part_number": number, "content_length": len(body)}).encode()
                status, _, _ = fixture.request("POST", f"/upload/sessions/{upload_id}/parts", prepare, {"Content-Type": "application/json"})
                self.assertEqual(status, 200)
                status, _, _ = fixture.request("PUT", f"/upload/sessions/{upload_id}/parts/{number}", body, {"Content-Type": "application/octet-stream"})
                self.assertEqual(status, 200)
            status, _, raw = fixture.request("POST", f"/upload/sessions/{upload_id}/complete", b"{}", {"Content-Type": "application/json"})
            self.assertEqual(status, 200, raw)
            self.assertEqual(json.loads(raw)["file_hash"], hashlib.sha256(b"abcdef").hexdigest())

    def test_legacy_small_multipart_upload_contract(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            fixture = RunningFixture(pathlib.Path(tmp))
            self.addCleanup(fixture.close)
            data = b"small-file"
            content_type, body = self.multipart_body(
                {"task_id": "7", "asset_type": "reference", "filename": "small.txt", "mime_type": "text/plain", "expected_size": str(len(data))},
                "small.txt",
                "text/plain",
                data,
            )
            status, _, raw = fixture.request("POST", "/upload/files", body, {"Content-Type": content_type})
            self.assertEqual(status, 201, raw)
            meta = json.loads(raw)
            self.assertEqual(meta["file_size"], len(data))
            self.assertEqual(meta["file_hash"], hashlib.sha256(data).hexdigest())

    def test_read_only_upload_and_object_fixtures_reject_all_writes(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            seed = root / "seed"
            seed.mkdir()
            (seed / "frozen.txt").write_bytes(b"frozen")
            for mode in ("upload", "object"):
                readonly_root = root / mode
                readonly_root.mkdir()
                fixture = RunningFixture(readonly_root, read_only=True, mode=mode, seed=seed)
                try:
                    status, _, identity = fixture.request("GET", "/identity")
                    self.assertEqual(status, 200)
                    self.assertIn(mode.encode(), identity)
                    status, _, data = fixture.request("GET", "/files/frozen.txt")
                    self.assertEqual((status, data), (200, b"frozen"))
                    status, _, data = fixture.request("GET", "/p/frozen.txt")
                    self.assertEqual((status, data), (200, b"frozen"))
                    for method, path in (("POST", "/upload/sessions"), ("PUT", "/upload/sessions/upl-" + "0" * 24 + "/file")):
                        status, _, _ = fixture.request(method, path, b"{}", {"Content-Type": "application/json"})
                        self.assertEqual(status, 403)
                    status, _, _ = fixture.request("DELETE", "/files/frozen.txt")
                    self.assertEqual(status, 403)
                finally:
                    fixture.close()

    def test_seed_root_accepts_materialized_objects_directory(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            seed = root / "seed"
            (seed / "objects" / "v8-ab").mkdir(parents=True)
            (seed / "objects" / "v8-ab" / "recovered.bin").write_bytes(
                b"recovered"
            )
            fixture = RunningFixture(
                root / "runtime", mode="upload", seed=seed
            )
            self.addCleanup(fixture.close)
            status, _, data = fixture.request(
                "GET", "/files/v8-ab/recovered.bin"
            )
            self.assertEqual((status, data), (200, b"recovered"))

    def test_encoded_path_traversal_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            fixture = RunningFixture(root)
            self.addCleanup(fixture.close)
            status, _, _ = fixture.request("GET", "/files/%2e%2e/secret")
            self.assertEqual(status, 400)
            self.assertFalse((root.parent / "secret").exists())


if __name__ == "__main__":
    unittest.main()
