#!/usr/bin/env python3
"""Run-scoped upload/object fixture for the isolated Browser A/B rehearsal.

This intentionally implements only the upload-service routes used by
service/upload_service_client.go.  It never proxies an upstream service.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import mimetypes
import os
import pathlib
import re
import threading
import urllib.parse
from email.parser import BytesParser
from email.policy import default as email_policy
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any


HEX_SHA256 = re.compile(r"(?:sha256:)?([0-9a-fA-F]{64})\Z")
SAFE_ID = re.compile(r"[A-Za-z0-9._:-]{3,128}\Z")


class FixtureError(Exception):
    def __init__(self, status: int, message: str):
        super().__init__(message)
        self.status = status
        self.message = message


def json_bytes(value: object) -> bytes:
    return (json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")) + "\n").encode()


def strict_int(value: object, label: str, *, minimum: int = 0) -> int:
    if isinstance(value, bool):
        raise FixtureError(HTTPStatus.BAD_REQUEST, f"{label} must be an integer")
    try:
        parsed = int(value)  # type: ignore[arg-type]
    except (TypeError, ValueError):
        raise FixtureError(HTTPStatus.BAD_REQUEST, f"{label} must be an integer") from None
    if str(parsed) != str(value).strip() and not isinstance(value, int):
        raise FixtureError(HTTPStatus.BAD_REQUEST, f"{label} must be an integer")
    if parsed < minimum:
        raise FixtureError(HTTPStatus.BAD_REQUEST, f"{label} must be >= {minimum}")
    return parsed


def normalize_checksum(value: object) -> str:
    raw = str(value or "").strip()
    if not raw:
        return ""
    match = HEX_SHA256.fullmatch(raw)
    if not match:
        raise FixtureError(HTTPStatus.BAD_REQUEST, "checksum_hint must be a SHA-256 hex digest")
    return match.group(1).lower()


def safe_filename(value: object) -> str:
    raw = str(value or "").strip()
    if not raw or raw in {".", ".."} or "\x00" in raw or "/" in raw or "\\" in raw:
        raise FixtureError(HTTPStatus.BAD_REQUEST, "filename must be a non-empty basename")
    return raw


def safe_storage_key(raw: str) -> pathlib.PurePosixPath:
    try:
        decoded = urllib.parse.unquote(raw, errors="strict")
    except UnicodeDecodeError:
        raise FixtureError(HTTPStatus.BAD_REQUEST, "invalid UTF-8 storage key") from None
    if not decoded or "\x00" in decoded or "\\" in decoded or decoded.startswith("/"):
        raise FixtureError(HTTPStatus.BAD_REQUEST, "invalid storage key")
    path = pathlib.PurePosixPath(decoded)
    if any(part in {"", ".", ".."} for part in path.parts):
        raise FixtureError(HTTPStatus.BAD_REQUEST, "storage key traversal is forbidden")
    return path


def contained_path(root: pathlib.Path, relative: pathlib.PurePosixPath) -> pathlib.Path:
    candidate = root.joinpath(*relative.parts).resolve()
    try:
        candidate.relative_to(root.resolve())
    except ValueError:
        raise FixtureError(HTTPStatus.BAD_REQUEST, "storage key escapes fixture root") from None
    return candidate


def parse_multipart(content_type: str, body: bytes) -> tuple[dict[str, str], str, str, bytes]:
    if not content_type.lower().startswith("multipart/form-data;"):
        raise FixtureError(HTTPStatus.UNSUPPORTED_MEDIA_TYPE, "multipart/form-data is required")
    try:
        encoded_content_type = content_type.encode("ascii")
    except UnicodeEncodeError:
        raise FixtureError(HTTPStatus.BAD_REQUEST, "invalid multipart Content-Type") from None
    message = BytesParser(policy=email_policy).parsebytes(
        b"MIME-Version: 1.0\r\nContent-Type: " + encoded_content_type + b"\r\n\r\n" + body
    )
    if not message.is_multipart():
        raise FixtureError(HTTPStatus.BAD_REQUEST, "invalid multipart body")
    fields: dict[str, str] = {}
    file_name = ""
    file_mime = ""
    file_bytes: bytes | None = None
    for part in message.iter_parts():
        field_name = part.get_param("name", header="content-disposition")
        if not field_name:
            continue
        payload = part.get_payload(decode=True) or b""
        filename = part.get_filename()
        if filename is not None:
            if file_bytes is not None:
                raise FixtureError(HTTPStatus.BAD_REQUEST, "exactly one file field is required")
            file_name = safe_filename(filename)
            file_mime = str(part.get_content_type() or "application/octet-stream")
            file_bytes = payload
        else:
            fields[str(field_name)] = payload.decode(part.get_content_charset() or "utf-8")
    if file_bytes is None:
        raise FixtureError(HTTPStatus.BAD_REQUEST, "multipart body is missing file field")
    return fields, file_name, file_mime, file_bytes


class FixtureStore:
    def __init__(
        self,
        root: pathlib.Path,
        identity: str,
        mode: str,
        read_only: bool,
        seed_root: pathlib.Path | None,
        max_upload_bytes: int,
        public_base_url: str,
    ) -> None:
        if not root.is_absolute():
            raise ValueError("fixture root must be absolute")
        if not SAFE_ID.fullmatch(identity):
            raise ValueError("identity must contain only stable safe characters")
        if mode == "object" and not read_only:
            raise ValueError("object fixture must be read-only")
        self.root = root.resolve()
        self.identity = identity
        self.mode = mode
        self.read_only = read_only
        self.seed_root = seed_root.resolve() if seed_root else None
        self.max_upload_bytes = max_upload_bytes
        self.public_base_url = public_base_url.rstrip("/")
        self.lock = threading.Lock()
        if read_only:
            if not self.root.is_dir():
                raise ValueError("read-only fixture root must already exist")
        else:
            (self.root / "objects").mkdir(parents=True, exist_ok=True)
            (self.root / "sessions").mkdir(parents=True, exist_ok=True)
            (self.root / "parts").mkdir(parents=True, exist_ok=True)
        if self.seed_root and not self.seed_root.is_dir():
            raise ValueError("seed root must be an existing directory")

    def _require_writable(self) -> None:
        if self.read_only or self.mode != "upload":
            raise FixtureError(HTTPStatus.FORBIDDEN, "fixture is read-only")

    def _session_path(self, upload_id: str) -> pathlib.Path:
        if not re.fullmatch(r"upl-[0-9a-f]{24}", upload_id):
            raise FixtureError(HTTPStatus.BAD_REQUEST, "invalid upload_id")
        return self.root / "sessions" / f"{upload_id}.json"

    def _load_session(self, upload_id: str) -> dict[str, Any]:
        path = self._session_path(upload_id)
        try:
            value = json.loads(path.read_text(encoding="utf-8"))
        except FileNotFoundError:
            raise FixtureError(HTTPStatus.NOT_FOUND, "upload session not found") from None
        if not isinstance(value, dict):
            raise FixtureError(HTTPStatus.INTERNAL_SERVER_ERROR, "invalid stored session")
        return value

    def _save_session(self, session: dict[str, Any]) -> None:
        path = self._session_path(str(session["upload_id"]))
        temp = path.with_suffix(".tmp")
        temp.write_bytes(json_bytes(session))
        os.replace(temp, path)

    def _event(self, action: str, **facts: object) -> None:
        self._require_writable()
        log_path = self.root / "events.jsonl"
        with self.lock:
            sequence = 1
            if log_path.exists():
                with log_path.open("rb") as handle:
                    sequence += sum(1 for _ in handle)
            event = {"sequence": sequence, "identity": self.identity, "action": action, **facts}
            with log_path.open("ab") as handle:
                handle.write(json_bytes(event))

    def create_session(self, payload: dict[str, Any]) -> dict[str, Any]:
        self._require_writable()
        filename = safe_filename(payload.get("filename"))
        upload_mode = str(payload.get("upload_mode") or "small").strip().lower()
        if upload_mode not in {"small", "multipart"}:
            raise FixtureError(HTTPStatus.BAD_REQUEST, "upload_mode must be small or multipart")
        expected_raw = payload.get("expected_size", payload.get("file_size"))
        expected_size = strict_int(expected_raw, "expected_size", minimum=0)
        if expected_size > self.max_upload_bytes:
            raise FixtureError(HTTPStatus.REQUEST_ENTITY_TOO_LARGE, "expected_size exceeds fixture byte limit")
        mime_type = str(payload.get("mime_type") or "application/octet-stream").strip().lower()
        if "/" not in mime_type or "\r" in mime_type or "\n" in mime_type:
            raise FixtureError(HTTPStatus.BAD_REQUEST, "invalid mime_type")
        canonical = json_bytes({
            "identity": self.identity,
            "task_id": payload.get("task_id"),
            "task_ref": str(payload.get("task_ref") or ""),
            "asset_no": str(payload.get("asset_no") or ""),
            "asset_type": str(payload.get("asset_type") or payload.get("file_role") or ""),
            "version_no": payload.get("version_no"),
            "upload_mode": upload_mode,
            "filename": filename,
            "expected_size": expected_size,
            "mime_type": mime_type,
            "created_by": payload.get("created_by"),
        })
        with self.lock:
            ordinal = 1 + sum(1 for _ in (self.root / "sessions").glob("upl-*.json"))
            upload_id = "upl-" + hashlib.sha256(canonical + str(ordinal).encode()).hexdigest()[:24]
            session = {
                "upload_id": upload_id,
                "session_status": "created",
                "upload_mode": upload_mode,
                "filename": filename,
                "expected_size": expected_size,
                "mime_type": mime_type,
                "task_ref": str(payload.get("task_ref") or ""),
                "asset_no": str(payload.get("asset_no") or ""),
                "version_no": strict_int(payload.get("version_no") or 1, "version_no", minimum=1),
                "file_role": str(payload.get("file_role") or payload.get("asset_type") or ""),
                "parts": {},
            }
            self._save_session(session)
        self._event("session_created", upload_id=upload_id, expected_size=expected_size, mime_type=mime_type)
        return self.session_plan(session)

    def session_plan(self, session: dict[str, Any]) -> dict[str, Any]:
        upload_id = str(session["upload_id"])
        base = self.public_base_url
        return {
            **session,
            "base_url": base,
            "upload_url": f"{base}/upload/sessions/{upload_id}/file",
            "part_upload_url_template": f"{base}/upload/sessions/{upload_id}/parts/{{part_number}}",
            "complete_url": f"{base}/upload/sessions/{upload_id}/complete",
            "abort_url": f"{base}/upload/sessions/{upload_id}/abort",
            "method": "PUT",
            "headers": {"Content-Type": session["mime_type"]},
            "part_size_hint": 8 * 1024 * 1024,
        }

    def _validate_bytes(self, session: dict[str, Any], data: bytes, mime_type: str) -> str:
        if len(data) > self.max_upload_bytes:
            raise FixtureError(HTTPStatus.REQUEST_ENTITY_TOO_LARGE, "upload exceeds fixture byte limit")
        if len(data) != int(session["expected_size"]):
            raise FixtureError(HTTPStatus.UNPROCESSABLE_ENTITY, "uploaded byte size does not match expected_size")
        actual_mime = mime_type.split(";", 1)[0].strip().lower()
        if actual_mime != str(session["mime_type"]).lower():
            raise FixtureError(HTTPStatus.UNPROCESSABLE_ENTITY, "uploaded MIME type does not match session")
        return hashlib.sha256(data).hexdigest()

    def put_session_file(self, upload_id: str, data: bytes, mime_type: str) -> dict[str, Any]:
        self._require_writable()
        session = self._load_session(upload_id)
        if session["session_status"] in {"completed", "cancelled"}:
            raise FixtureError(HTTPStatus.CONFLICT, "upload session is terminal")
        digest = self._validate_bytes(session, data, mime_type)
        staging = self.root / "parts" / upload_id
        staging.mkdir(parents=True, exist_ok=True)
        (staging / "whole.bin").write_bytes(data)
        session.update({"session_status": "uploaded", "file_hash": digest, "file_size": len(data)})
        self._save_session(session)
        self._event("session_file_uploaded", upload_id=upload_id, file_size=len(data), file_hash=digest)
        return self.file_meta(session)

    def prepare_part(self, upload_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        self._require_writable()
        session = self._load_session(upload_id)
        part_number = strict_int(payload.get("part_number"), "part_number", minimum=1)
        content_length = payload.get("content_length")
        if content_length is not None:
            content_length = strict_int(content_length, "content_length", minimum=0)
        session.setdefault("prepared_parts", {})[str(part_number)] = content_length
        self._save_session(session)
        return {
            "upload_id": upload_id,
            "part_number": part_number,
            "method": "PUT",
            "upload_url": f"{self.public_base_url}/upload/sessions/{upload_id}/parts/{part_number}",
            "headers": {"Content-Type": "application/octet-stream"},
            "part_size_hint": 8 * 1024 * 1024,
        }

    def put_part(self, upload_id: str, part_number: int, data: bytes) -> dict[str, Any]:
        self._require_writable()
        session = self._load_session(upload_id)
        if session["session_status"] in {"completed", "cancelled"}:
            raise FixtureError(HTTPStatus.CONFLICT, "upload session is terminal")
        if len(data) > self.max_upload_bytes:
            raise FixtureError(HTTPStatus.REQUEST_ENTITY_TOO_LARGE, "part exceeds fixture byte limit")
        prepared = session.get("prepared_parts", {}).get(str(part_number))
        if prepared is not None and int(prepared) != len(data):
            raise FixtureError(HTTPStatus.UNPROCESSABLE_ENTITY, "part size does not match prepared content_length")
        existing_parts = session.get("parts", {})
        stored_other_bytes = sum(int(meta.get("size") or 0) for number, meta in existing_parts.items() if number != str(part_number))
        if stored_other_bytes + len(data) > int(session["expected_size"]):
            raise FixtureError(HTTPStatus.UNPROCESSABLE_ENTITY, "multipart bytes exceed expected_size")
        part_dir = self.root / "parts" / upload_id
        part_dir.mkdir(parents=True, exist_ok=True)
        (part_dir / f"{part_number:06d}.part").write_bytes(data)
        digest = hashlib.sha256(data).hexdigest()
        session.setdefault("parts", {})[str(part_number)] = {"size": len(data), "sha256": digest}
        session["session_status"] = "uploaded"
        self._save_session(session)
        self._event("session_part_uploaded", upload_id=upload_id, part_number=part_number, file_size=len(data), file_hash=digest)
        return {"upload_id": upload_id, "part_number": part_number, "etag": digest}

    def complete(self, upload_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        self._require_writable()
        session = self._load_session(upload_id)
        if session["session_status"] == "cancelled":
            raise FixtureError(HTTPStatus.CONFLICT, "cancelled session cannot complete")
        if session["session_status"] == "completed":
            return self.file_meta(session)
        staging = self.root / "parts" / upload_id
        whole = staging / "whole.bin"
        if whole.is_file():
            data = whole.read_bytes()
        else:
            part_files = sorted(staging.glob("*.part")) if staging.is_dir() else []
            numbers = [int(item.stem) for item in part_files]
            if not part_files or numbers != list(range(1, len(numbers) + 1)):
                raise FixtureError(HTTPStatus.CONFLICT, "multipart upload is incomplete")
            data = b"".join(item.read_bytes() for item in part_files)
        digest = self._validate_bytes(session, data, str(session["mime_type"]))
        declared_size = payload.get("expected_size", payload.get("file_size"))
        if declared_size is not None and strict_int(declared_size, "expected_size") != len(data):
            raise FixtureError(HTTPStatus.UNPROCESSABLE_ENTITY, "complete expected_size mismatch")
        declared_mime = str(payload.get("mime_type") or "").strip().lower()
        if declared_mime and declared_mime != str(session["mime_type"]).lower():
            raise FixtureError(HTTPStatus.UNPROCESSABLE_ENTITY, "complete mime_type mismatch")
        checksum = normalize_checksum(payload.get("checksum_hint"))
        if checksum and checksum != digest:
            raise FixtureError(HTTPStatus.UNPROCESSABLE_ENTITY, "complete checksum mismatch")
        identity_key = hashlib.sha256(self.identity.encode()).hexdigest()[:12]
        storage_key = pathlib.PurePosixPath("fixture", identity_key, "uploads", upload_id, str(session["filename"]))
        target = contained_path(self.root / "objects", storage_key)
        target.parent.mkdir(parents=True, exist_ok=True)
        target_temp = target.with_name(target.name + ".tmp")
        target_temp.write_bytes(data)
        os.replace(target_temp, target)
        session.update({
            "session_status": "completed",
            "file_id": "file-" + hashlib.sha256((upload_id + digest).encode()).hexdigest()[:24],
            "storage_key": storage_key.as_posix(),
            "file_size": len(data),
            "file_hash": digest,
        })
        self._save_session(session)
        self._event("session_completed", upload_id=upload_id, storage_key=storage_key.as_posix(), file_size=len(data), file_hash=digest)
        return self.file_meta(session)

    def cancel(self, upload_id: str) -> dict[str, Any]:
        self._require_writable()
        session = self._load_session(upload_id)
        if session["session_status"] == "completed":
            raise FixtureError(HTTPStatus.CONFLICT, "completed session cannot cancel")
        session["session_status"] = "cancelled"
        self._save_session(session)
        self._event("session_cancelled", upload_id=upload_id)
        return {"upload_id": upload_id, "session_status": "cancelled"}

    def small_upload(self, fields: dict[str, str], filename: str, mime_type: str, data: bytes) -> dict[str, Any]:
        self._require_writable()
        expected = fields.get("expected_size")
        expected_size = strict_int(expected, "expected_size") if expected is not None else len(data)
        requested_mime = str(fields.get("mime_type") or mime_type).split(";", 1)[0].strip().lower()
        payload = {
            "task_id": fields.get("task_id"),
            "asset_type": fields.get("asset_type"),
            "filename": fields.get("filename") or filename,
            "expected_size": expected_size,
            "mime_type": requested_mime,
            "created_by": fields.get("created_by"),
            "upload_mode": "small",
        }
        plan = self.create_session(payload)
        upload_id = str(plan["upload_id"])
        self.put_session_file(upload_id, data, mime_type)
        return self.complete(upload_id, {})

    @staticmethod
    def file_meta(session: dict[str, Any]) -> dict[str, Any]:
        return {
            key: session[key]
            for key in ("file_id", "storage_key", "file_size", "file_hash", "mime_type")
            if session.get(key) not in (None, "")
        }

    def resolve_file(self, raw_key: str) -> pathlib.Path:
        relative = safe_storage_key(raw_key)
        candidates = [contained_path(self.root / "objects", relative)]
        if self.seed_root:
            candidates.append(contained_path(self.seed_root, relative))
        for candidate in candidates:
            if candidate.is_file() and not candidate.is_symlink():
                return candidate
        raise FixtureError(HTTPStatus.NOT_FOUND, "file not found")


class FixtureHandler(BaseHTTPRequestHandler):
    server_version = "ABFixture/1"
    sys_version = ""

    @property
    def store(self) -> FixtureStore:
        return self.server.store  # type: ignore[attr-defined]

    def log_message(self, _format: str, *_args: object) -> None:
        return

    def _reply(self, status: int, body: bytes = b"", content_type: str = "application/json") -> None:
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(body)

    def _json_reply(self, status: int, value: object) -> None:
        self._reply(status, json_bytes(value))

    def _body(self) -> bytes:
        raw_length = self.headers.get("Content-Length")
        if raw_length is None:
            raise FixtureError(HTTPStatus.LENGTH_REQUIRED, "Content-Length is required")
        length = strict_int(raw_length, "Content-Length")
        if length > self.store.max_upload_bytes + 1024 * 1024:
            raise FixtureError(HTTPStatus.REQUEST_ENTITY_TOO_LARGE, "request exceeds fixture byte limit")
        data = self.rfile.read(length)
        if len(data) != length:
            raise FixtureError(HTTPStatus.BAD_REQUEST, "incomplete request body")
        return data

    def _json_body(self) -> dict[str, Any]:
        content_type = self.headers.get("Content-Type", "").split(";", 1)[0].strip().lower()
        if content_type != "application/json":
            raise FixtureError(HTTPStatus.UNSUPPORTED_MEDIA_TYPE, "application/json is required")
        try:
            value = json.loads(self._body())
        except json.JSONDecodeError:
            raise FixtureError(HTTPStatus.BAD_REQUEST, "invalid JSON body") from None
        if not isinstance(value, dict):
            raise FixtureError(HTTPStatus.BAD_REQUEST, "JSON body must be an object")
        return value

    def _guard(self, action: Any) -> None:
        try:
            action()
        except FixtureError as exc:
            self._json_reply(exc.status, {"error": exc.message})
        except (BrokenPipeError, ConnectionResetError):
            return

    def do_GET(self) -> None:  # noqa: N802
        self._guard(self._do_get)

    def do_HEAD(self) -> None:  # noqa: N802
        self._guard(self._do_get)

    def _do_get(self) -> None:
        path = urllib.parse.urlsplit(self.path).path
        if path == "/health":
            self._reply(HTTPStatus.OK, b"ok\n", "text/plain; charset=utf-8")
            return
        if path == "/identity":
            self._reply(HTTPStatus.OK, self.store.identity.encode(), "text/plain; charset=utf-8")
            return
        if path.startswith("/files/") or path.startswith("/p/"):
            prefix = "/files/" if path.startswith("/files/") else "/p/"
            file_path = self.store.resolve_file(path[len(prefix):])
            data = file_path.read_bytes()
            mime_type = mimetypes.guess_type(file_path.name)[0] or "application/octet-stream"
            self.send_response(HTTPStatus.OK)
            self.send_header("Content-Type", mime_type)
            self.send_header("Content-Length", str(len(data)))
            self.send_header("ETag", '"' + hashlib.sha256(data).hexdigest() + '"')
            self.send_header("Cache-Control", "no-store")
            self.end_headers()
            if self.command != "HEAD":
                self.wfile.write(data)
            return
        match = re.fullmatch(r"/upload/sessions/(upl-[0-9a-f]{24})", path)
        if match and self.store.mode == "upload":
            self._json_reply(HTTPStatus.OK, self.store.session_plan(self.store._load_session(match.group(1))))
            return
        meta = re.fullmatch(r"/upload/sessions/(upl-[0-9a-f]{24})/(?:file-meta|file)", path)
        if meta and self.store.mode == "upload":
            session = self.store._load_session(meta.group(1))
            if session.get("session_status") != "completed":
                raise FixtureError(HTTPStatus.CONFLICT, "upload session is not completed")
            self._json_reply(HTTPStatus.OK, self.store.file_meta(session))
            return
        raise FixtureError(HTTPStatus.NOT_FOUND, "route not found")

    def do_POST(self) -> None:  # noqa: N802
        self._guard(self._do_post)

    def _do_post(self) -> None:
        path = urllib.parse.urlsplit(self.path).path
        if self.store.mode != "upload" or self.store.read_only:
            raise FixtureError(HTTPStatus.FORBIDDEN, "fixture is read-only")
        if path == "/upload/sessions":
            self._json_reply(HTTPStatus.CREATED, self.store.create_session(self._json_body()))
            return
        if path == "/upload/files":
            body = self._body()
            fields, filename, mime_type, data = parse_multipart(self.headers.get("Content-Type", ""), body)
            self._json_reply(HTTPStatus.CREATED, self.store.small_upload(fields, filename, mime_type, data))
            return
        match = re.fullmatch(r"/upload/sessions/(upl-[0-9a-f]{24})/(complete|abort|cancel|parts|file)", path)
        if not match:
            raise FixtureError(HTTPStatus.NOT_FOUND, "route not found")
        upload_id, action = match.groups()
        if action == "complete":
            self._json_reply(HTTPStatus.OK, self.store.complete(upload_id, self._json_body()))
        elif action in {"abort", "cancel"}:
            self._json_reply(HTTPStatus.OK, self.store.cancel(upload_id))
        elif action == "parts":
            self._json_reply(HTTPStatus.OK, self.store.prepare_part(upload_id, self._json_body()))
        else:
            body = self._body()
            fields, _filename, mime_type, data = parse_multipart(self.headers.get("Content-Type", ""), body)
            form_upload_id = fields.get("upload_id") or fields.get("remote_upload_id")
            if form_upload_id and form_upload_id != upload_id:
                raise FixtureError(HTTPStatus.BAD_REQUEST, "multipart upload_id mismatch")
            self._json_reply(HTTPStatus.OK, self.store.put_session_file(upload_id, data, mime_type))

    def do_PUT(self) -> None:  # noqa: N802
        self._guard(self._do_put)

    def do_PATCH(self) -> None:  # noqa: N802
        self._guard(self._unsupported_write)

    def do_DELETE(self) -> None:  # noqa: N802
        self._guard(self._unsupported_write)

    def _unsupported_write(self) -> None:
        if self.store.read_only or self.store.mode != "upload":
            raise FixtureError(HTTPStatus.FORBIDDEN, "fixture is read-only")
        raise FixtureError(HTTPStatus.METHOD_NOT_ALLOWED, "write route not implemented")

    def _do_put(self) -> None:
        if self.store.mode != "upload" or self.store.read_only:
            raise FixtureError(HTTPStatus.FORBIDDEN, "fixture is read-only")
        path = urllib.parse.urlsplit(self.path).path
        file_match = re.fullmatch(r"/upload/sessions/(upl-[0-9a-f]{24})/file", path)
        part_match = re.fullmatch(r"/upload/sessions/(upl-[0-9a-f]{24})/parts/([1-9][0-9]*)", path)
        if file_match:
            raw = self._body()
            content_type = self.headers.get("Content-Type", "application/octet-stream")
            if content_type.lower().startswith("multipart/form-data"):
                fields, _filename, content_type, raw = parse_multipart(content_type, raw)
                form_upload_id = fields.get("upload_id") or fields.get("remote_upload_id")
                if form_upload_id and form_upload_id != file_match.group(1):
                    raise FixtureError(HTTPStatus.BAD_REQUEST, "multipart upload_id mismatch")
            self._json_reply(HTTPStatus.OK, self.store.put_session_file(file_match.group(1), raw, content_type))
            return
        if part_match:
            result = self.store.put_part(part_match.group(1), int(part_match.group(2)), self._body())
            self._json_reply(HTTPStatus.OK, result)
            return
        raise FixtureError(HTTPStatus.NOT_FOUND, "route not found")


class FixtureHTTPServer(ThreadingHTTPServer):
    daemon_threads = True
    allow_reuse_address = True


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", choices=("upload", "object"), required=True)
    parser.add_argument("--root", type=pathlib.Path, required=True)
    parser.add_argument("--seed-root", type=pathlib.Path)
    parser.add_argument("--identity", required=True)
    parser.add_argument("--host", default="0.0.0.0")
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--read-only", action="store_true")
    parser.add_argument("--max-upload-bytes", type=int, default=1024 * 1024 * 1024)
    parser.add_argument("--public-base-url", default="")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if not 1024 <= args.port <= 65535:
        raise SystemExit("port must be an unprivileged TCP port")
    if args.max_upload_bytes <= 0:
        raise SystemExit("max-upload-bytes must be positive")
    public_base_url = args.public_base_url or f"http://127.0.0.1:{args.port}"
    store = FixtureStore(
        args.root,
        args.identity,
        args.mode,
        args.read_only,
        args.seed_root,
        args.max_upload_bytes,
        public_base_url,
    )
    server = FixtureHTTPServer((args.host, args.port), FixtureHandler)
    server.store = store  # type: ignore[attr-defined]
    server.serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
