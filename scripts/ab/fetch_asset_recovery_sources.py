#!/usr/bin/env python3
"""Fetch the three frozen task-2807 recovery sources through read-only OSS GETs.

This tool has an intentionally closed scope.  It never connects to a database,
never sends an OSS mutation, and can only request the three immutable object
keys frozen below.  Bytes are streamed through a framed SSH helper into
``<run-root>/source-assets`` and become visible only after size, MIME, and
SHA-256 checks pass.  The resulting secret-free receipts are suitable for the
``source_local_path`` and ``source_fetch_receipt`` fields consumed by
``prepare_asset_recovery.py``.
"""

from __future__ import annotations

import argparse
import datetime
import hashlib
import json
import os
import pathlib
import re
import shlex
import struct
import subprocess
import sys
import tempfile
from dataclasses import dataclass
from typing import Any, BinaryIO, Callable


PROTOCOL = "controlled-asset-read-v1"
SCHEMA_VERSION = 1
MAX_FRAME_BYTES = 128 * 1024
CHUNK_BYTES = 1024 * 1024
SSH_HOST = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,254}$")
REMOTE_PATH = re.compile(r"^/[A-Za-z0-9._/-]+$")
SHA256 = re.compile(r"^[0-9a-f]{64}$")
SAFE_FAILURE_DETAILS = {
    "connection_error",
    "content_length_differs_from_stream",
    "invalid_content_length",
    "mime_mismatch",
    "read_error",
    "size_mismatch",
    "timeout",
    "tls_error",
}
SAFE_HTTP_STATUS = re.compile(r"^http_status=[1-5][0-9]{2}$")


@dataclass(frozen=True)
class FrozenSource:
    missing_task_asset_id: int
    task_asset_id: int
    storage_ref_id: str
    object_key: str
    size: int
    mime_type: str
    sha256: str

    @property
    def file_name(self) -> str:
        return f"task-asset-{self.task_asset_id}-{self.sha256}.jpg"


FROZEN_SOURCES = (
    FrozenSource(
        missing_task_asset_id=23989,
        task_asset_id=24034,
        storage_ref_id="983a746c-c674-4f5c-8812-073be989b194",
        object_key=(
            "tasks/RW-20260706-A-002095/upload-sessions/"
            "cd1ee703-1ab1-47ac-8777-0318642ab43f/"
            "cd1ee703-1ab1-47ac-8777-0318642ab43f.jpg"
        ),
        size=683001,
        mime_type="image/jpeg",
        sha256="d0558b1a9d4a7afed5a03b6b97d4a765d34050866686e396ab0acf9f08f0dec5",
    ),
    FrozenSource(
        missing_task_asset_id=23990,
        task_asset_id=24033,
        storage_ref_id="85c01c4c-0e27-4df4-a851-4b888f54a837",
        object_key=(
            "tasks/RW-20260706-A-002095/upload-sessions/"
            "c635a00c-4dd8-475d-85bb-a6caf6c7cb98/"
            "c635a00c-4dd8-475d-85bb-a6caf6c7cb98.jpg"
        ),
        size=689291,
        mime_type="image/jpeg",
        sha256="64cdfed11adc778fb6ede7f03c49f7c70e8655870236bdcd92a8207e41a8dfb8",
    ),
    FrozenSource(
        missing_task_asset_id=23991,
        task_asset_id=24040,
        storage_ref_id="769e687f-fd71-4f37-930c-fd3f566350e6",
        object_key=(
            "tasks/RW-20260706-A-002095/upload-sessions/"
            "986b811f-eca2-4162-b952-57f781296355/"
            "986b811f-eca2-4162-b952-57f781296355.jpg"
        ),
        size=686447,
        mime_type="image/jpeg",
        sha256="ebfecf3407e05c576bcddf74673d2e7568207ecc27855aa0e08c453d5a0d119a",
    ),
)


REMOTE_HELPER = r'''
import base64
import email.utils
import hashlib
import hmac
import json
import pathlib
import re
import socket
import ssl
import struct
import sys
import urllib.error
import urllib.parse
import urllib.request

PROTOCOL = "controlled-asset-read-v1"
MAX_FRAME = 131072
MAX_KEY = 65536
CHUNK = 1048576
SAFE_DETAILS = {
    "connection_error",
    "content_length_differs_from_stream",
    "invalid_content_length",
    "read_error",
    "timeout",
    "tls_error",
}

class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None

def read_exact(stream, size):
    parts = []
    remaining = size
    while remaining:
        chunk = stream.read(remaining)
        if not chunk:
            return None
        parts.append(chunk)
        remaining -= len(chunk)
    return b"".join(parts)

def send_frame(value):
    payload = json.dumps(
        value, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")
    if not payload or len(payload) > MAX_FRAME:
        raise SystemExit(2)
    sys.stdout.buffer.write(struct.pack("!I", len(payload)))
    sys.stdout.buffer.write(payload)
    sys.stdout.buffer.flush()

def valid_key(value):
    return (
        value
        and len(value.encode("utf-8")) <= MAX_KEY
        and not value.startswith("/")
        and "\\" not in value
        and "\x00" not in value
        and all(part not in {"", ".", ".."} for part in value.split("/"))
    )

def load_env(path_value):
    values = {}
    wanted = {
        "OSS_ACCESS_KEY_ID",
        "OSS_ACCESS_KEY_SECRET",
        "OSS_BUCKET",
        "OSS_ENDPOINT",
        "UPLOAD_STORAGE_PROVIDER",
    }
    with pathlib.Path(path_value).open("r", encoding="utf-8") as handle:
        for raw in handle:
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            if line.startswith("export "):
                line = line[7:].lstrip()
            key, separator, value = line.partition("=")
            key = key.strip()
            if separator and key in wanted:
                value = value.strip()
                if len(value) >= 2 and value[0] == value[-1] and value[0] in {"'", '"'}:
                    value = value[1:-1]
                if "\r" in value or "\n" in value:
                    raise ValueError("invalid OSS configuration")
                values[key] = value
    required = {
        "OSS_ACCESS_KEY_ID",
        "OSS_ACCESS_KEY_SECRET",
        "OSS_BUCKET",
        "OSS_ENDPOINT",
    }
    if any(not values.get(key) for key in required):
        raise ValueError("OSS configuration unavailable")
    provider = values.get("UPLOAD_STORAGE_PROVIDER", "").strip().lower() or "oss"
    if provider != "oss":
        raise ValueError("direct OSS reader requires OSS storage provider")
    values["UPLOAD_STORAGE_PROVIDER"] = provider
    return values

def endpoint_origin(endpoint_value, bucket):
    candidate = endpoint_value
    if "://" not in candidate:
        candidate = "https://" + candidate
    parsed = urllib.parse.urlsplit(candidate)
    if (
        parsed.scheme not in {"http", "https"}
        or not parsed.netloc
        or parsed.username
        or parsed.password
        or parsed.path not in {"", "/"}
        or parsed.query
        or parsed.fragment
    ):
        raise ValueError("invalid OSS endpoint")
    host = parsed.netloc
    if not host.lower().startswith(bucket.lower() + "."):
        host = bucket + "." + host
    return parsed.scheme + "://" + host

def safe_error_detail(exc):
    if isinstance(exc, (TimeoutError, socket.timeout)):
        return "timeout"
    if isinstance(exc, ssl.SSLError):
        return "tls_error"
    if isinstance(exc, urllib.error.URLError):
        reason = exc.reason
        if isinstance(reason, (TimeoutError, socket.timeout)):
            return "timeout"
        if isinstance(reason, ssl.SSLError):
            return "tls_error"
        return "connection_error"
    return "read_error"

def error_frame(status, detail):
    if not (
        detail in SAFE_DETAILS
        or re.fullmatch(r"http_status=[1-5][0-9]{2}", detail)
    ):
        detail = "read_error"
    send_frame({
        "detail": detail,
        "mime": "",
        "size": 0,
        "status": status,
    })

env_path = sys.argv[1]
timeout = float(sys.argv[2])
if timeout <= 0 or timeout > 3600:
    raise ValueError("invalid timeout")
config = load_env(env_path)
bucket = config["OSS_BUCKET"]
access_key_id = config["OSS_ACCESS_KEY_ID"]
access_key_secret = config["OSS_ACCESS_KEY_SECRET"]
origin = endpoint_origin(config["OSS_ENDPOINT"], bucket)
fingerprint_source = "\n".join([
    "adapter=" + PROTOCOL,
    "provider=" + config["UPLOAD_STORAGE_PROVIDER"],
    "endpoint=" + config["OSS_ENDPOINT"],
    "bucket=" + bucket,
    "access_key_id=" + access_key_id,
]) + "\n"
send_frame({
    "config_fingerprint_sha256": hashlib.sha256(
        fingerprint_source.encode("utf-8")
    ).hexdigest(),
    "protocol": PROTOCOL,
})
opener = urllib.request.build_opener(NoRedirect())

while True:
    prefix = read_exact(sys.stdin.buffer, 4)
    if prefix is None:
        break
    length = struct.unpack("!I", prefix)[0]
    if length == 0:
        break
    if length > MAX_FRAME:
        raise SystemExit(2)
    encoded_request = read_exact(sys.stdin.buffer, length)
    if encoded_request is None:
        raise SystemExit(2)
    try:
        frame = json.loads(encoded_request.decode("utf-8", "strict"))
    except (UnicodeDecodeError, json.JSONDecodeError):
        error_frame(400, "read_error")
        continue
    valid = (
        isinstance(frame, dict)
        and set(frame) == {"max_object_bytes", "object_key"}
        and isinstance(frame["max_object_bytes"], int)
        and not isinstance(frame["max_object_bytes"], bool)
        and frame["max_object_bytes"] > 0
        and isinstance(frame["object_key"], str)
        and valid_key(frame["object_key"])
    )
    if not valid:
        error_frame(400, "read_error")
        continue
    object_key = frame["object_key"]
    escaped_key = "/".join(
        urllib.parse.quote(part, safe="") for part in object_key.split("/")
    )
    date_value = email.utils.formatdate(usegmt=True)
    canonical_resource = "/" + bucket + "/" + object_key
    string_to_sign = "GET\n\n\n" + date_value + "\n" + canonical_resource
    signature = base64.b64encode(hmac.new(
        access_key_secret.encode("utf-8"),
        string_to_sign.encode("utf-8"),
        hashlib.sha1,
    ).digest()).decode("ascii")
    request = urllib.request.Request(
        origin + "/" + escaped_key,
        method="GET",
        headers={
            "Accept-Encoding": "identity",
            "Authorization": "OSS " + access_key_id + ":" + signature,
            "Date": date_value,
        },
    )
    try:
        response = opener.open(request, timeout=timeout)
    except urllib.error.HTTPError as exc:
        status = exc.code if isinstance(exc.code, int) else 599
        exc.close()
        error_frame(status, "http_status=" + str(status))
        continue
    except Exception as exc:
        error_frame(599, safe_error_detail(exc))
        continue
    with response:
        status = response.getcode()
        raw_length = response.headers.get("Content-Length")
        try:
            declared_size = int(raw_length)
        except (TypeError, ValueError):
            declared_size = -1
        mime = (
            (response.headers.get("Content-Type") or "")
            .split(";", 1)[0].strip().lower()
        )
        encoding = (response.headers.get("Content-Encoding") or "").strip().lower()
        if status != 200:
            error_frame(status if isinstance(status, int) else 599, "read_error")
            continue
        if declared_size < 0 or declared_size > frame["max_object_bytes"]:
            error_frame(502, "invalid_content_length")
            continue
        if not mime or encoding not in {"", "identity"}:
            error_frame(502, "read_error")
            continue
        send_frame({
            "detail": "",
            "mime": mime,
            "size": declared_size,
            "status": 200,
        })
        decision = read_exact(sys.stdin.buffer, 1)
        if decision is None:
            raise SystemExit(2)
        if decision != b"\x01":
            continue
        remaining = declared_size
        while remaining:
            chunk = response.read(min(CHUNK, remaining))
            if not chunk:
                raise SystemExit(3)
            sys.stdout.buffer.write(chunk)
            remaining -= len(chunk)
        sys.stdout.buffer.flush()
'''


class ControlledReadError(OSError):
    """Secret-free controlled read failure."""


def canonical_bytes(value: Any) -> bytes:
    return json.dumps(
        value, ensure_ascii=False, separators=(",", ":"), sort_keys=True
    ).encode("utf-8")


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(CHUNK_BYTES), b""):
            digest.update(chunk)
    return digest.hexdigest()


def read_exact(stream: BinaryIO, size: int) -> bytes:
    parts: list[bytes] = []
    remaining = size
    while remaining:
        chunk = stream.read(remaining)
        if not chunk:
            raise ControlledReadError("controlled read transport ended unexpectedly")
        parts.append(chunk)
        remaining -= len(chunk)
    return b"".join(parts)


def read_frame(stream: BinaryIO) -> dict[str, Any]:
    try:
        frame_size = struct.unpack("!I", read_exact(stream, 4))[0]
        if frame_size <= 0 or frame_size > MAX_FRAME_BYTES:
            raise ControlledReadError("invalid controlled read frame length")
        value = json.loads(read_exact(stream, frame_size).decode("utf-8", "strict"))
    except (json.JSONDecodeError, OSError, UnicodeDecodeError, struct.error) as exc:
        if isinstance(exc, ControlledReadError):
            raise
        raise ControlledReadError("invalid controlled read frame") from exc
    if not isinstance(value, dict):
        raise ControlledReadError("invalid controlled read metadata")
    return value


def validate_ssh_host(value: str) -> str:
    if not SSH_HOST.fullmatch(value):
        raise ValueError("ssh host must be a simple configured host alias")
    return value


def validate_remote_env_path(value: str) -> str:
    path = pathlib.PurePosixPath(value)
    if (
        not REMOTE_PATH.fullmatch(value)
        or any(part in {"", ".", ".."} for part in path.parts[1:])
    ):
        raise ValueError("ssh env file must be a simple absolute POSIX path")
    return value


def remote_command(env_path: str, timeout_seconds: float) -> str:
    return " ".join(
        [
            "python3",
            "-u",
            "-c",
            shlex.quote(REMOTE_HELPER),
            shlex.quote(env_path),
            shlex.quote(str(timeout_seconds)),
        ]
    )


def start_ssh_process(
    host: str,
    env_path: str,
    timeout_seconds: float,
) -> subprocess.Popen:
    connect_timeout = max(1, min(3600, int(timeout_seconds)))
    return subprocess.Popen(
        [
            "ssh",
            "-T",
            "-o",
            "BatchMode=yes",
            "-o",
            "LogLevel=ERROR",
            "-o",
            f"ConnectTimeout={connect_timeout}",
            host,
            remote_command(env_path, timeout_seconds),
        ],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        bufsize=0,
    )


def normalize_mime(value: str) -> str:
    return value.split(";", 1)[0].strip().lower()


class SSHControlledReadAdapter:
    """Persistent remote OSS GET stream with a local exact-allowlist caller."""

    def __init__(
        self,
        host: str,
        env_path: str,
        timeout_seconds: float,
        *,
        process_factory: Callable[[str, str, float], Any] = start_ssh_process,
    ):
        self.host = validate_ssh_host(host)
        self.env_path = validate_remote_env_path(env_path)
        if timeout_seconds <= 0 or timeout_seconds > 3600:
            raise ValueError("timeout_seconds must be in (0, 3600]")
        self.timeout_seconds = timeout_seconds
        self.process_factory = process_factory
        self.process: Any | None = None
        self.stdin: BinaryIO | None = None
        self.stdout: BinaryIO | None = None
        self.config_fingerprint_sha256 = ""
        self.broken = False

    def ensure_process(self) -> None:
        if self.broken:
            raise ControlledReadError("controlled read transport is unavailable")
        if self.process is not None:
            if self.process.poll() is not None:
                self.broken = True
                raise ControlledReadError("controlled read transport exited")
            return
        process = self.process_factory(
            self.host, self.env_path, self.timeout_seconds
        )
        if process.stdin is None or process.stdout is None:
            try:
                process.terminate()
            except OSError:
                pass
            self.broken = True
            raise ControlledReadError("controlled read transport has no pipes")
        self.process = process
        self.stdin = process.stdin
        self.stdout = process.stdout
        hello = read_frame(self.stdout)
        valid = (
            set(hello) == {"config_fingerprint_sha256", "protocol"}
            and hello.get("protocol") == PROTOCOL
            and isinstance(hello.get("config_fingerprint_sha256"), str)
            and bool(SHA256.fullmatch(hello["config_fingerprint_sha256"]))
        )
        if not valid:
            self.broken = True
            raise ControlledReadError("invalid controlled read handshake")
        self.config_fingerprint_sha256 = hello["config_fingerprint_sha256"]

    def origin_fingerprint(self) -> str:
        self.ensure_process()
        source = "\x1f".join(
            (
                PROTOCOL,
                self.host,
                self.env_path,
                self.config_fingerprint_sha256,
            )
        )
        return hashlib.sha256(source.encode("utf-8")).hexdigest()

    def fetch_to_path(self, source: FrozenSource, temporary: pathlib.Path) -> None:
        self.ensure_process()
        assert self.stdin is not None and self.stdout is not None
        request = canonical_bytes(
            {"max_object_bytes": source.size, "object_key": source.object_key}
        )
        if len(request) > MAX_FRAME_BYTES:
            raise ControlledReadError("controlled read request exceeds protocol limit")
        try:
            self.stdin.write(struct.pack("!I", len(request)) + request)
            self.stdin.flush()
            header = read_frame(self.stdout)
        except OSError:
            self.broken = True
            raise
        valid_shape = (
            set(header) == {"detail", "mime", "size", "status"}
            and isinstance(header.get("detail"), str)
            and isinstance(header.get("mime"), str)
            and isinstance(header.get("size"), int)
            and not isinstance(header.get("size"), bool)
            and header["size"] >= 0
            and isinstance(header.get("status"), int)
            and not isinstance(header.get("status"), bool)
            and 100 <= header["status"] <= 599
        )
        if not valid_shape:
            self.broken = True
            raise ControlledReadError("invalid controlled read response metadata")
        if header["status"] != 200:
            detail = header["detail"]
            if not (
                detail in SAFE_FAILURE_DETAILS
                or SAFE_HTTP_STATUS.fullmatch(detail)
            ):
                self.broken = True
                raise ControlledReadError("invalid controlled read failure metadata")
            raise ControlledReadError(f"controlled read failed: {detail}")
        if (
            header["detail"] != ""
            or header["size"] != source.size
            or normalize_mime(header["mime"]) != source.mime_type
        ):
            try:
                self.stdin.write(b"\x00")
                self.stdin.flush()
            except OSError:
                self.broken = True
            raise ControlledReadError("remote metadata differs from frozen allowlist")
        digest = hashlib.sha256()
        total = 0
        try:
            self.stdin.write(b"\x01")
            self.stdin.flush()
            with temporary.open("wb") as handle:
                remaining = source.size
                while remaining:
                    chunk = read_exact(self.stdout, min(CHUNK_BYTES, remaining))
                    handle.write(chunk)
                    digest.update(chunk)
                    total += len(chunk)
                    remaining -= len(chunk)
                handle.flush()
                os.fsync(handle.fileno())
        except OSError:
            self.broken = True
            raise
        if total != source.size or digest.hexdigest() != source.sha256:
            raise ControlledReadError("stream bytes differ from frozen allowlist")

    def close(self) -> None:
        if self.process is None:
            return
        close_failure = ""
        try:
            if (
                not self.broken
                and self.process.poll() is None
                and self.stdin is not None
            ):
                self.stdin.write(struct.pack("!I", 0))
                self.stdin.flush()
        except OSError:
            pass
        try:
            if self.stdin is not None:
                self.stdin.close()
        except OSError:
            pass
        try:
            self.process.wait(timeout=max(1.0, min(10.0, self.timeout_seconds)))
        except subprocess.TimeoutExpired:
            self.process.terminate()
            try:
                self.process.wait(timeout=2)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait(timeout=2)
            close_failure = "controlled read transport did not close cleanly"
        except OSError:
            close_failure = "controlled read transport close failed"
        if not close_failure and self.process.poll() != 0:
            close_failure = "controlled read transport exited unsuccessfully"
        if close_failure:
            raise ControlledReadError(close_failure)


def validated_run_root(value: pathlib.Path) -> pathlib.Path:
    if not value.is_absolute():
        raise ValueError("--run-root must be an explicit absolute path")
    if not value.is_dir():
        raise ValueError("--run-root must already exist and be a directory")
    return value.resolve(strict=True)


def contained_source_directory(run_root: pathlib.Path) -> pathlib.Path:
    candidate = run_root / "source-assets"
    if candidate.exists():
        resolved = candidate.resolve(strict=True)
        if not resolved.is_dir() or run_root not in resolved.parents:
            raise ValueError("source-assets must be a contained real directory")
        return resolved
    candidate.mkdir(mode=0o700)
    resolved = candidate.resolve(strict=True)
    if run_root not in resolved.parents:
        raise ValueError("source-assets escaped run-root")
    return resolved


def validate_existing_evidence(
    evidence_path: pathlib.Path,
    source_dir: pathlib.Path,
) -> dict[str, Any] | None:
    if not evidence_path.exists():
        return None
    if evidence_path.is_symlink():
        raise ValueError("controlled-read evidence must not be a symlink")
    evidence = json.loads(evidence_path.read_text(encoding="utf-8"))
    expected_document_keys = {
        "version",
        "status",
        "protocol",
        "production_writes_executed",
        "database_connections_opened",
        "remote_operation",
        "origin_fingerprint_sha256",
        "recoveries",
        "evidence_sha256",
    }
    if (
        not isinstance(evidence, dict)
        or set(evidence) != expected_document_keys
        or evidence.get("version") != SCHEMA_VERSION
        or evidence.get("protocol") != PROTOCOL
        or evidence.get("status") != "PASS"
        or evidence.get("production_writes_executed") is not False
        or evidence.get("database_connections_opened") is not False
        or evidence.get("remote_operation") != "GET"
        or not SHA256.fullmatch(
            str(evidence.get("origin_fingerprint_sha256") or "")
        )
        or not isinstance(evidence.get("recoveries"), list)
        or len(evidence["recoveries"]) != len(FROZEN_SOURCES)
    ):
        raise ValueError("existing controlled-read evidence is invalid")
    unsigned = dict(evidence)
    evidence_sha256 = unsigned.pop("evidence_sha256")
    if (
        not isinstance(evidence_sha256, str)
        or hashlib.sha256(canonical_bytes(unsigned)).hexdigest() != evidence_sha256
    ):
        raise ValueError("existing controlled-read evidence hash drifted")
    by_id = {
        row.get("task_asset_id"): row
        for row in evidence["recoveries"]
        if isinstance(row, dict)
    }
    if set(by_id) != {source.task_asset_id for source in FROZEN_SOURCES}:
        raise ValueError("existing controlled-read evidence allowlist drifted")
    for source in FROZEN_SOURCES:
        row = by_id[source.task_asset_id]
        target = source_dir / source.file_name
        receipt = row.get("source_fetch_receipt")
        expected_fields = {
            "protocol": PROTOCOL,
            "task_asset_id": source.task_asset_id,
            "storage_ref_id": source.storage_ref_id,
            "object_key": source.object_key,
            "size": source.size,
            "sha256": source.sha256,
        }
        if (
            set(row)
            != {
                "missing_task_asset_id",
                "task_asset_id",
                "source_local_path",
                "source_sha256",
                "source_fetch_receipt",
            }
            or row.get("missing_task_asset_id") != source.missing_task_asset_id
            or row.get("task_asset_id") != source.task_asset_id
            or row.get("source_sha256") != source.sha256
            or pathlib.Path(str(row.get("source_local_path") or "")).resolve()
            != target.resolve()
            or not isinstance(receipt, dict)
            or set(receipt)
            != {
                "protocol",
                "task_asset_id",
                "storage_ref_id",
                "object_key",
                "size",
                "mime_type",
                "sha256",
                "fetched_at",
            }
            or any(receipt.get(key) != value for key, value in expected_fields.items())
            or receipt.get("mime_type") != source.mime_type
            or not str(receipt.get("fetched_at") or "").strip()
            or not target.is_file()
            or target.is_symlink()
            or target.resolve().parent != source_dir
            or target.stat().st_size != source.size
            or sha256_file(target) != source.sha256
        ):
            raise ValueError("existing controlled-read bytes or receipt drifted")
    return evidence


def atomic_write(path: pathlib.Path, data: bytes) -> None:
    descriptor, name = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    temporary = pathlib.Path(name)
    try:
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(data)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if temporary.exists():
            temporary.unlink()


def run(
    args: argparse.Namespace,
    *,
    adapter_factory: Callable[[str, str, float], Any] = SSHControlledReadAdapter,
    now: Callable[[], datetime.datetime] = lambda: datetime.datetime.now(
        datetime.timezone.utc
    ),
) -> dict[str, Any]:
    run_root = validated_run_root(args.run_root)
    source_dir = contained_source_directory(run_root)
    evidence_path = source_dir / "controlled-read-receipts.json"
    existing = validate_existing_evidence(evidence_path, source_dir)
    if existing is not None:
        return existing
    existing_targets = [
        source_dir / source.file_name
        for source in FROZEN_SOURCES
        if (source_dir / source.file_name).exists()
    ]
    if existing_targets:
        raise FileExistsError(
            "source files exist without their exact controlled-read evidence"
        )

    adapter = adapter_factory(args.ssh_host, args.ssh_env_file, args.timeout_seconds)
    adapter_closed = False
    created: list[pathlib.Path] = []
    recoveries: list[dict[str, Any]] = []
    try:
        origin_fingerprint = adapter.origin_fingerprint()
        if not SHA256.fullmatch(origin_fingerprint):
            raise ControlledReadError("invalid controlled read origin fingerprint")
        for source in FROZEN_SOURCES:
            target = source_dir / source.file_name
            descriptor, name = tempfile.mkstemp(
                prefix=f".{target.name}.", dir=source_dir
            )
            os.close(descriptor)
            temporary = pathlib.Path(name)
            try:
                adapter.fetch_to_path(source, temporary)
                if (
                    temporary.stat().st_size != source.size
                    or sha256_file(temporary) != source.sha256
                ):
                    raise ControlledReadError(
                        "post-fetch bytes differ from frozen allowlist"
                    )
                os.replace(temporary, target)
                created.append(target)
            finally:
                if temporary.exists():
                    temporary.unlink()
            fetched_at = now().astimezone(datetime.timezone.utc).isoformat().replace(
                "+00:00", "Z"
            )
            receipt = {
                "protocol": PROTOCOL,
                "task_asset_id": source.task_asset_id,
                "storage_ref_id": source.storage_ref_id,
                "object_key": source.object_key,
                "size": source.size,
                "mime_type": source.mime_type,
                "sha256": source.sha256,
                "fetched_at": fetched_at,
            }
            recoveries.append(
                {
                    "missing_task_asset_id": source.missing_task_asset_id,
                    "task_asset_id": source.task_asset_id,
                    "source_local_path": str(target),
                    "source_sha256": source.sha256,
                    "source_fetch_receipt": receipt,
                }
            )
        adapter.close()
        adapter_closed = True
        evidence = {
            "version": SCHEMA_VERSION,
            "status": "PASS",
            "protocol": PROTOCOL,
            "production_writes_executed": False,
            "database_connections_opened": False,
            "remote_operation": "GET",
            "origin_fingerprint_sha256": origin_fingerprint,
            "recoveries": recoveries,
        }
        unsigned = canonical_bytes(evidence)
        evidence["evidence_sha256"] = hashlib.sha256(unsigned).hexdigest()
        atomic_write(evidence_path, canonical_bytes(evidence) + b"\n")
        return evidence
    except Exception:
        for path in reversed(created):
            try:
                path.unlink()
            except FileNotFoundError:
                pass
        raise
    finally:
        if not adapter_closed:
            try:
                adapter.close()
            except OSError:
                pass


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-root", required=True, type=pathlib.Path)
    parser.add_argument("--ssh-host", required=True)
    parser.add_argument("--ssh-env-file", required=True)
    parser.add_argument("--timeout-seconds", type=float, default=30.0)
    args = parser.parse_args(sys.argv[1:] if argv is None else argv)
    try:
        args.ssh_host = validate_ssh_host(args.ssh_host)
        args.ssh_env_file = validate_remote_env_path(args.ssh_env_file)
    except ValueError as exc:
        parser.error(str(exc))
    if args.timeout_seconds <= 0 or args.timeout_seconds > 3600:
        parser.error("--timeout-seconds must be in (0, 3600]")
    return args


def main() -> int:
    try:
        run(parse_args())
    except (ControlledReadError, OSError, ValueError, json.JSONDecodeError) as exc:
        print(f"BLOCKED: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
