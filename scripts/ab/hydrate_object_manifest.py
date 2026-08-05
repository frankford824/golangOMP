#!/usr/bin/env python3
"""Hydrate missing object-manifest fingerprints through read-only streamed GETs.

The input manifest remains immutable.  Objects are deduplicated within their
configured adapter by object key, response bodies are streamed directly into a
SHA-256 digest, and resumable state is written through an atomic checkpoint.
Neither the checkpoint nor the evidence contains credentials, request URLs,
request/response headers, or response bodies.
"""
from __future__ import annotations

import argparse
import concurrent.futures
import hashlib
import http.client
import io
import json
import os
import pathlib
import re
import shlex
import struct
import subprocess
import sys
import tempfile
import threading
import urllib.error
import urllib.parse
from dataclasses import dataclass
from typing import Any, BinaryIO, Callable

try:
    from scripts.ab import object_manifest_verifier as verifier
    from scripts.ab import historical_unavailable_exception
except ModuleNotFoundError:  # Direct execution from scripts/ab.
    import object_manifest_verifier as verifier
    import historical_unavailable_exception


SCHEMA_VERSION = 1
CHECKPOINT_SCHEMA_VERSION = 2
FAILURE_RETRY_AUTHORIZATION_SCHEMA_VERSION = 1
FAILURE_RETRY_AUTHORIZATION_TYPE = "g06_checkpoint_failure_retry_v1"
DEFAULT_CHECKPOINT_EVERY = 50
DEFAULT_MAX_OBJECT_BYTES = 20 * 1024 * 1024 * 1024
ZERO_SHA256 = "0" * 64
MAX_PROTOCOL_HEADER = 64 * 1024
MAX_PROTOCOL_OBJECT_KEY = 64 * 1024
SSH_HOST = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,254}$")
REMOTE_PATH = re.compile(r"^/[A-Za-z0-9._/-]+$")
DIRECT_OSS_INTERNAL_ENDPOINT = re.compile(
    r"^oss-[a-z0-9]+(?:-[a-z0-9]+)*-internal\.aliyuncs\.com$"
)
SAFE_CHECKPOINT_FAILURE_DETAILS = {
    "timeout",
    "tls_error",
    "connection_error",
    "read_error",
    "invalid_content_length",
    "declared_size_exceeds_limit",
    "stream_size_exceeds_limit",
    "stored MIME is unavailable",
    "non_identity_content_encoding",
    "content_length_differs_from_stream",
}
RETRYABLE_TRANSIENT_FAILURE_DETAILS = {
    "timeout",
    "tls_error",
    "connection_error",
    "read_error",
}
SAFE_CHECKPOINT_FAILURE_CODE = re.compile(r"^object_manifest\.[a-z0-9_]+$")
SAFE_HTTP_STATUS_DETAIL = re.compile(r"^http_status=[1-5][0-9]{2}$")


REMOTE_UPLOAD_HELPER = r'''
import json
import pathlib
import re
import struct
import sys
import urllib.error
import urllib.parse
import urllib.request

MAX_KEY = 65536
CHUNK = 1024 * 1024

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

def send_header(status, size, mime):
    payload = json.dumps(
        {"status": status, "size": size, "mime": mime},
        sort_keys=True, separators=(",", ":"),
    ).encode("utf-8")
    sys.stdout.buffer.write(struct.pack("!I", len(payload)))
    sys.stdout.buffer.write(payload)
    sys.stdout.buffer.flush()

def valid_key(value):
    return (
        value
        and not value.startswith("/")
        and "\\" not in value
        and "\x00" not in value
        and all(part not in {"", ".", ".."} for part in value.split("/"))
    )

def load_upload_config(path_value):
    values = {}
    with pathlib.Path(path_value).open("r", encoding="utf-8") as handle:
        for raw in handle:
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            if line.startswith("export "):
                line = line[7:].lstrip()
            key, separator, value = line.partition("=")
            key = key.strip()
            if separator and key in {
                "UPLOAD_SERVICE_INTERNAL_TOKEN",
                "UPLOAD_STORAGE_PROVIDER",
            }:
                value = value.strip()
                if len(value) >= 2 and value[0] == value[-1] and value[0] in {"'", '"'}:
                    value = value[1:-1]
                values[key] = value
    token = values.get("UPLOAD_SERVICE_INTERNAL_TOKEN", "")
    if not token or "\r" in token or "\n" in token:
        raise ValueError("token unavailable")
    provider = values.get("UPLOAD_STORAGE_PROVIDER", "").strip() or "oss"
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]{0,63}", provider):
        raise ValueError("invalid storage provider")
    return token, provider

def validate_base(value):
    parsed = urllib.parse.urlsplit(value)
    if (
        parsed.scheme not in {"http", "https"}
        or not parsed.netloc
        or parsed.username
        or parsed.password
        or parsed.query
        or parsed.fragment
    ):
        raise ValueError("invalid base")
    return value.rstrip("/")

env_path = sys.argv[1]
base = validate_base(sys.argv[2])
timeout = float(sys.argv[3])
if timeout <= 0 or timeout > 3600:
    raise ValueError("invalid timeout")
token, storage_provider = load_upload_config(env_path)
opener = urllib.request.build_opener(NoRedirect())

while True:
    prefix = read_exact(sys.stdin.buffer, 4)
    if prefix is None:
        break
    length = struct.unpack("!I", prefix)[0]
    if length == 0:
        break
    if length > MAX_KEY:
        raise SystemExit(2)
    encoded_key = read_exact(sys.stdin.buffer, length)
    if encoded_key is None:
        raise SystemExit(2)
    try:
        object_key = encoded_key.decode("utf-8", "strict")
    except UnicodeDecodeError:
        send_header(400, 0, "")
        continue
    if not valid_key(object_key):
        send_header(400, 0, "")
        continue
    escaped = "/".join(urllib.parse.quote(part, safe="") for part in object_key.split("/"))
    request = urllib.request.Request(
        base + "/" + escaped,
        method="GET",
        headers={
            "X-Internal-Token": token,
            "X-Storage-Provider": storage_provider,
            "Accept-Encoding": "identity",
        },
    )
    try:
        response = opener.open(request, timeout=timeout)
    except urllib.error.HTTPError as exc:
        status = exc.code if isinstance(exc.code, int) else 599
        exc.close()
        send_header(status, 0, "")
        continue
    except Exception:
        send_header(599, 0, "")
        continue
    with response:
        status = response.getcode()
        content_length = response.headers.get("Content-Length")
        mime = (response.headers.get("Content-Type") or "").split(";", 1)[0].strip().lower()
        encoding = (response.headers.get("Content-Encoding") or "").strip().lower()
        try:
            size = int(content_length)
        except (TypeError, ValueError):
            size = -1
        if status != 200 or size < 0 or not mime or encoding not in {"", "identity"}:
            send_header(status if status != 200 else 502, 0, "")
            continue
        send_header(200, size, mime)
        decision = read_exact(sys.stdin.buffer, 1)
        if decision is None:
            raise SystemExit(2)
        if decision != b"\x01":
            continue
        remaining = size
        while remaining:
            chunk = response.read(min(CHUNK, remaining))
            if not chunk:
                raise SystemExit(3)
            sys.stdout.buffer.write(chunk)
            remaining -= len(chunk)
        sys.stdout.buffer.flush()
'''


REMOTE_DIRECT_OSS_HELPER = r'''
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

MAX_FRAME = 131072
MAX_KEY = 65536
CHUNK = 1024 * 1024
RANGE_CHUNK = 8 * 1024 * 1024
PROTOCOL = "direct-oss-sha256-v1"
SAFE_DETAILS = {
    "connection_error",
    "content_length_differs_from_stream",
    "declared_size_exceeds_limit",
    "invalid_content_length",
    "non_identity_content_encoding",
    "read_error",
    "stored MIME is unavailable",
    "stream_size_exceeds_limit",
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
    if len(payload) > MAX_FRAME:
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
        raise ValueError("direct OSS adapter requires OSS storage provider")
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

def endpoint_with_override(endpoint_value, override_value):
    if not override_value:
        return endpoint_value
    public = re.fullmatch(
        r"oss-([a-z0-9]+(?:-[a-z0-9]+)*)\.aliyuncs\.com",
        endpoint_value,
    )
    if public is None or "internal" in public.group(1).split("-"):
        raise ValueError("OSS endpoint is not an exact public regional endpoint")
    expected = "oss-" + public.group(1) + "-internal.aliyuncs.com"
    if override_value != expected:
        raise ValueError("OSS endpoint override is not the same-region internal endpoint")
    return override_value

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
        "sha256": "",
        "size": 0,
        "status": status,
    })

def signed_request(method, object_key, escaped_key, range_value=""):
    date_value = email.utils.formatdate(usegmt=True)
    canonical_resource = "/" + bucket + "/" + object_key
    string_to_sign = method + "\n\n\n" + date_value + "\n" + canonical_resource
    signature = base64.b64encode(hmac.new(
        access_key_secret.encode("utf-8"),
        string_to_sign.encode("utf-8"),
        hashlib.sha1,
    ).digest()).decode("ascii")
    headers = {
        "Accept-Encoding": "identity",
        "Authorization": "OSS " + access_key_id + ":" + signature,
        "Date": date_value,
    }
    if range_value:
        headers["Range"] = range_value
    return urllib.request.Request(
        origin + "/" + escaped_key,
        method=method,
        headers=headers,
    )

env_path = sys.argv[1]
timeout = float(sys.argv[2])
endpoint_override = sys.argv[3]
if timeout <= 0 or timeout > 3600:
    raise ValueError("invalid timeout")
config = load_env(env_path)
bucket = config["OSS_BUCKET"]
access_key_id = config["OSS_ACCESS_KEY_ID"]
access_key_secret = config["OSS_ACCESS_KEY_SECRET"]
route_endpoint = endpoint_with_override(
    config["OSS_ENDPOINT"], endpoint_override
)
origin = endpoint_origin(route_endpoint, bucket)
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
        request_frame = json.loads(encoded_request.decode("utf-8", "strict"))
    except (UnicodeDecodeError, json.JSONDecodeError):
        error_frame(400, "read_error")
        continue
    valid_request = (
        isinstance(request_frame, dict)
        and set(request_frame) == {"max_object_bytes", "object_key"}
        and isinstance(request_frame["max_object_bytes"], int)
        and not isinstance(request_frame["max_object_bytes"], bool)
        and request_frame["max_object_bytes"] > 0
        and isinstance(request_frame["object_key"], str)
        and valid_key(request_frame["object_key"])
    )
    if not valid_request:
        error_frame(400, "read_error")
        continue
    object_key = request_frame["object_key"]
    max_object_bytes = request_frame["max_object_bytes"]
    escaped_key = "/".join(
        urllib.parse.quote(part, safe="") for part in object_key.split("/")
    )
    request = signed_request("GET", object_key, escaped_key)
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
        if status != 200:
            error_frame(status if isinstance(status, int) else 599, "read_error")
            continue
        raw_length = response.headers.get("Content-Length")
        try:
            declared_size = int(raw_length)
        except (TypeError, ValueError):
            error_frame(502, "invalid_content_length")
            continue
        if declared_size < 0:
            error_frame(502, "invalid_content_length")
            continue
        if declared_size > max_object_bytes:
            error_frame(413, "declared_size_exceeds_limit")
            continue
        mime = (
            (response.headers.get("Content-Type") or "")
            .split(";", 1)[0].strip().lower()
        )
        if not mime:
            error_frame(502, "stored MIME is unavailable")
            continue
        encoding = (response.headers.get("Content-Encoding") or "").strip().lower()
        if encoding not in {"", "identity"}:
            error_frame(502, "non_identity_content_encoding")
            continue
        digest = hashlib.sha256()
        total = 0
        failed = ""
        try:
            while True:
                chunk = response.read(CHUNK)
                if not chunk:
                    break
                total += len(chunk)
                if total > max_object_bytes:
                    failed = "stream_size_exceeds_limit"
                    break
                digest.update(chunk)
        except Exception as exc:
            failed = safe_error_detail(exc)
        if failed:
            error_frame(413 if failed == "stream_size_exceeds_limit" else 599, failed)
            continue
        if total < declared_size:
            # A small set of large multipart objects can end a long-lived
            # urllib response cleanly before Content-Length is exhausted.
            # Resume only the missing suffix through bounded OSS Range GETs.
            # Each range is buffered before it mutates the digest, so a short
            # range response can be retried without double-hashing bytes.
            while total < declared_size:
                range_start = total
                range_end = min(
                    declared_size - 1,
                    range_start + RANGE_CHUNK - 1,
                )
                expected_range_size = range_end - range_start + 1
                range_bytes = None
                for _attempt in range(3):
                    range_request = signed_request(
                        "GET",
                        object_key,
                        escaped_key,
                        "bytes=" + str(range_start) + "-" + str(range_end),
                    )
                    try:
                        range_response = opener.open(
                            range_request, timeout=timeout
                        )
                    except Exception:
                        continue
                    with range_response:
                        content_range = (
                            range_response.headers.get("Content-Range") or ""
                        ).strip().lower()
                        expected_content_range = (
                            "bytes "
                            + str(range_start)
                            + "-"
                            + str(range_end)
                            + "/"
                            + str(declared_size)
                        )
                        if (
                            range_response.getcode() != 206
                            or content_range != expected_content_range
                        ):
                            continue
                        buffer = bytearray()
                        try:
                            while len(buffer) < expected_range_size:
                                chunk = range_response.read(
                                    min(
                                        CHUNK,
                                        expected_range_size - len(buffer),
                                    )
                                )
                                if not chunk:
                                    break
                                buffer.extend(chunk)
                        except Exception:
                            buffer.clear()
                        if len(buffer) == expected_range_size:
                            range_bytes = bytes(buffer)
                            break
                if range_bytes is None:
                    break
                digest.update(range_bytes)
                total += len(range_bytes)
        if total != declared_size:
            error_frame(502, "content_length_differs_from_stream")
            continue
        send_frame({
            "detail": "",
            "mime": mime,
            "sha256": digest.hexdigest(),
            "size": total,
            "status": 200,
        })
'''


class HydrationFailure(Exception):
    """Secret-free object hydration failure."""

    def __init__(self, code: str, detail: str):
        super().__init__(detail)
        self.code = code
        self.detail = detail


@dataclass(frozen=True)
class ObjectMetadata:
    size: int
    mime_type: str
    sha256: str


class SSHProtocolError(OSError):
    """A secret-free persistent SSH framing failure."""


def read_exact(stream: BinaryIO, size: int) -> bytes:
    parts: list[bytes] = []
    remaining = size
    while remaining:
        chunk = stream.read(remaining)
        if not chunk:
            raise SSHProtocolError("ssh framed transport ended unexpectedly")
        parts.append(chunk)
        remaining -= len(chunk)
    return b"".join(parts)


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


def validate_direct_oss_endpoint_override(value: str) -> str:
    if value and not DIRECT_OSS_INTERNAL_ENDPOINT.fullmatch(value):
        raise ValueError(
            "SSH direct OSS endpoint override must be a bare Aliyun internal "
            "regional endpoint"
        )
    return value


def remote_command(env_path: str, base_url: str, timeout_seconds: float) -> str:
    return " ".join([
        "python3", "-u", "-c", shlex.quote(REMOTE_UPLOAD_HELPER),
        shlex.quote(env_path), shlex.quote(base_url), shlex.quote(str(timeout_seconds)),
    ])


def start_ssh_process(
    host: str,
    env_path: str,
    base_url: str,
    timeout_seconds: float,
) -> subprocess.Popen:
    connect_timeout = max(1, min(3600, int(timeout_seconds)))
    return subprocess.Popen(
        [
            "ssh", "-T",
            "-o", "BatchMode=yes",
            "-o", "LogLevel=ERROR",
            "-o", f"ConnectTimeout={connect_timeout}",
            host,
            remote_command(env_path, base_url, timeout_seconds),
        ],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        # Never copy SSH/helper diagnostics into evidence; they may be affected
        # by remote configuration outside this tool's control.
        stderr=subprocess.DEVNULL,
        bufsize=0,
    )


def remote_direct_oss_command(
    env_path: str,
    timeout_seconds: float,
    endpoint_override: str = "",
) -> str:
    endpoint_override = validate_direct_oss_endpoint_override(
        endpoint_override
    )
    return " ".join([
        "python3", "-u", "-c", shlex.quote(REMOTE_DIRECT_OSS_HELPER),
        shlex.quote(env_path), shlex.quote(str(timeout_seconds)),
        shlex.quote(endpoint_override),
    ])


def start_direct_oss_ssh_process(
    host: str,
    env_path: str,
    timeout_seconds: float,
    endpoint_override: str,
) -> subprocess.Popen:
    connect_timeout = max(1, min(3600, int(timeout_seconds)))
    return subprocess.Popen(
        [
            "ssh", "-T",
            "-o", "BatchMode=yes",
            "-o", "LogLevel=ERROR",
            "-o", f"ConnectTimeout={connect_timeout}",
            host,
            remote_direct_oss_command(
                env_path, timeout_seconds, endpoint_override
            ),
        ],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        bufsize=0,
    )


def read_json_frame(stream: BinaryIO) -> dict[str, Any]:
    try:
        header_length = struct.unpack("!I", read_exact(stream, 4))[0]
        if header_length <= 0 or header_length > MAX_PROTOCOL_HEADER:
            raise SSHProtocolError("invalid ssh response header length")
        raw_header = read_exact(stream, header_length)
        value = json.loads(raw_header.decode("utf-8", "strict"))
    except (OSError, UnicodeDecodeError, json.JSONDecodeError, struct.error) as exc:
        if isinstance(exc, SSHProtocolError):
            raise
        raise SSHProtocolError("invalid ssh response frame") from exc
    if not isinstance(value, dict):
        raise SSHProtocolError("invalid ssh response metadata")
    return value


class SSHObjectResponse:
    def __init__(self, adapter: "PersistentSSHReadAdapter", size: int, mime_type: str):
        self.adapter = adapter
        self.remaining = size
        self.headers = {
            "Content-Length": str(size),
            "Content-Type": mime_type,
            "Content-Encoding": "identity",
        }
        self.closed = False
        self.started = False

    def read(self, size: int = -1) -> bytes:
        if self.closed or self.remaining == 0:
            return b""
        if not self.started:
            self.adapter.begin_response(self)
            self.started = True
        wanted = self.remaining if size is None or size < 0 else min(size, self.remaining)
        data = read_exact(self.adapter.stdout, wanted)
        self.remaining -= len(data)
        return data

    def close(self, *, preserve_exception: bool = False) -> None:
        if self.closed:
            return
        try:
            if not self.started:
                self.adapter.cancel_response(self)
                return
            while self.remaining:
                self.read(min(verifier.CHUNK_SIZE, self.remaining))
        except OSError:
            self.adapter.mark_broken()
            if not preserve_exception:
                raise
        finally:
            self.closed = True
            self.adapter.release_response(self)

    def __enter__(self) -> "SSHObjectResponse":
        return self

    def __exit__(self, exc_type, _exc, _traceback) -> None:
        self.close(preserve_exception=exc_type is not None)


class PersistentSSHReadAdapter:
    """One lazy, persistent, read-only SSH object transport per hydration run."""

    use_head = False
    headers: dict[str, str] = {}

    def __init__(
        self,
        host: str,
        env_path: str,
        upload_base_url: str,
        timeout_seconds: float,
        *,
        process_factory: Callable[[str, str, str, float], Any] = start_ssh_process,
    ):
        self.host = validate_ssh_host(host)
        self.env_path = validate_remote_env_path(env_path)
        self.upload_base_url = verifier.validate_base_url(upload_base_url)
        self.timeout_seconds = timeout_seconds
        self.process_factory = process_factory
        origin = "\x1f".join((self.host, self.env_path, self.upload_base_url))
        # adapter_fingerprints hashes this value again. It intentionally reveals
        # neither the internal URL nor the remote environment path.
        self.base_url = "ssh-origin:" + hashlib.sha256(origin.encode("utf-8")).hexdigest()
        self.process: Any | None = None
        self.stdin: BinaryIO | None = None
        self.stdout: BinaryIO | None = None
        self.active_response: SSHObjectResponse | None = None
        self.broken = False

    def ensure_process(self) -> None:
        if self.broken:
            raise SSHProtocolError("ssh framed transport is unavailable")
        if self.process is not None:
            if self.process.poll() is not None:
                self.mark_broken()
                raise SSHProtocolError("ssh framed transport exited")
            return
        process = self.process_factory(
            self.host, self.env_path, self.upload_base_url, self.timeout_seconds
        )
        if process.stdin is None or process.stdout is None:
            try:
                process.terminate()
            except OSError:
                pass
            self.broken = True
            raise SSHProtocolError("ssh framed transport has no pipes")
        self.process = process
        self.stdin = process.stdin
        self.stdout = process.stdout

    def request(self, method: str, object_key: str) -> SSHObjectResponse:
        if method != "GET":
            raise SSHProtocolError("ssh object adapter only permits GET")
        if not verifier.valid_object_key(object_key):
            raise SSHProtocolError("invalid object key")
        if self.active_response is not None:
            raise SSHProtocolError("previous ssh object response is still active")
        encoded = object_key.encode("utf-8")
        if len(encoded) > MAX_PROTOCOL_OBJECT_KEY:
            raise SSHProtocolError("object key exceeds protocol limit")
        self.ensure_process()
        assert self.stdin is not None and self.stdout is not None
        try:
            self.stdin.write(struct.pack("!I", len(encoded)) + encoded)
            self.stdin.flush()
            header_length = struct.unpack("!I", read_exact(self.stdout, 4))[0]
            if header_length <= 0 or header_length > MAX_PROTOCOL_HEADER:
                raise SSHProtocolError("invalid ssh response header length")
            raw_header = read_exact(self.stdout, header_length)
            header = json.loads(raw_header.decode("utf-8", "strict"))
        except (OSError, UnicodeDecodeError, json.JSONDecodeError, struct.error) as exc:
            self.mark_broken()
            if isinstance(exc, SSHProtocolError):
                raise
            raise SSHProtocolError("invalid ssh response frame") from exc
        valid = (
            isinstance(header, dict)
            and set(header) == {"status", "size", "mime"}
            and isinstance(header["status"], int) and not isinstance(header["status"], bool)
            and 100 <= header["status"] <= 599
            and isinstance(header["size"], int) and not isinstance(header["size"], bool)
            and header["size"] >= 0
            and isinstance(header["mime"], str)
        )
        if not valid:
            self.mark_broken()
            raise SSHProtocolError("invalid ssh response metadata")
        if header["status"] != 200:
            if header["size"] != 0:
                self.mark_broken()
                raise SSHProtocolError("invalid ssh error response")
            raise urllib.error.HTTPError(
                "ssh-object://redacted",
                header["status"],
                "remote read failed",
                {},
                io.BytesIO(),
            )
        if not verifier.normalize_mime(header["mime"]):
            self.mark_broken()
            raise SSHProtocolError("ssh response MIME is unavailable")
        response = SSHObjectResponse(self, header["size"], verifier.normalize_mime(header["mime"]))
        self.active_response = response
        return response

    def release_response(self, response: SSHObjectResponse) -> None:
        if self.active_response is response:
            self.active_response = None

    def begin_response(self, response: SSHObjectResponse) -> None:
        if self.active_response is not response or self.stdin is None:
            raise SSHProtocolError("ssh response is not active")
        try:
            self.stdin.write(b"\x01")
            self.stdin.flush()
        except OSError as exc:
            self.mark_broken()
            raise SSHProtocolError("ssh response acknowledgement failed") from exc

    def cancel_response(self, response: SSHObjectResponse) -> None:
        if self.active_response is not response or self.stdin is None:
            raise SSHProtocolError("ssh response is not active")
        try:
            self.stdin.write(b"\x00")
            self.stdin.flush()
        except OSError as exc:
            self.mark_broken()
            raise SSHProtocolError("ssh response cancellation failed") from exc

    def mark_broken(self) -> None:
        self.broken = True

    def close(self) -> None:
        if self.process is None:
            return
        if self.active_response is not None:
            try:
                self.active_response.close()
            except OSError:
                pass
        try:
            if not self.broken and self.process.poll() is None and self.stdin is not None:
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

    def __enter__(self) -> "PersistentSSHReadAdapter":
        return self

    def __exit__(self, *_args) -> None:
        self.close()


class PersistentSSHDirectOSSAdapter:
    """Hash OSS objects remotely while returning only secret-free metadata."""

    use_head = False
    headers: dict[str, str] = {}
    protocol = "direct-oss-sha256-v1"

    def __init__(
        self,
        host: str,
        env_path: str,
        timeout_seconds: float,
        endpoint_override: str = "",
        *,
        process_factory: Callable[
            [str, str, float, str], Any
        ] = start_direct_oss_ssh_process,
    ):
        self.host = validate_ssh_host(host)
        self.env_path = validate_remote_env_path(env_path)
        self.timeout_seconds = timeout_seconds
        self.endpoint_override = validate_direct_oss_endpoint_override(
            endpoint_override
        )
        self.process_factory = process_factory
        static_origin = "\x1f".join((self.protocol, self.host, self.env_path))
        self.base_url = (
            "ssh-direct-oss-origin:"
            + hashlib.sha256(static_origin.encode("utf-8")).hexdigest()
        )
        self.process: Any | None = None
        self.stdin: BinaryIO | None = None
        self.stdout: BinaryIO | None = None
        self.remote_config_fingerprint = ""
        self.broken = False

    def ensure_process(self) -> None:
        if self.broken:
            raise SSHProtocolError("ssh direct OSS transport is unavailable")
        if self.process is not None:
            if self.process.poll() is not None:
                self.mark_broken()
                raise SSHProtocolError("ssh direct OSS transport exited")
            return
        process = self.process_factory(
            self.host,
            self.env_path,
            self.timeout_seconds,
            self.endpoint_override,
        )
        if process.stdin is None or process.stdout is None:
            try:
                process.terminate()
            except OSError:
                pass
            self.broken = True
            raise SSHProtocolError("ssh direct OSS transport has no pipes")
        self.process = process
        self.stdin = process.stdin
        self.stdout = process.stdout
        try:
            hello = read_json_frame(self.stdout)
        except OSError:
            self.mark_broken()
            raise
        valid = (
            set(hello) == {"config_fingerprint_sha256", "protocol"}
            and hello["protocol"] == self.protocol
            and isinstance(hello["config_fingerprint_sha256"], str)
            and bool(verifier.SHA256.fullmatch(hello["config_fingerprint_sha256"]))
        )
        if not valid:
            self.mark_broken()
            raise SSHProtocolError("invalid ssh direct OSS handshake")
        self.remote_config_fingerprint = hello["config_fingerprint_sha256"]

    def origin_fingerprint(self) -> str:
        self.ensure_process()
        source = "\x1f".join((
            self.protocol,
            self.host,
            self.env_path,
            self.remote_config_fingerprint,
        ))
        return hashlib.sha256(source.encode("utf-8")).hexdigest()

    def clone(self) -> "PersistentSSHDirectOSSAdapter":
        """Create an independent framed SSH session for parallel hashing."""
        return PersistentSSHDirectOSSAdapter(
            self.host,
            self.env_path,
            self.timeout_seconds,
            self.endpoint_override,
            process_factory=self.process_factory,
        )

    def get_metadata(
        self,
        object_key: str,
        max_object_bytes: int,
    ) -> ObjectMetadata:
        if not verifier.valid_object_key(object_key):
            raise SSHProtocolError("invalid object key")
        if max_object_bytes <= 0:
            raise SSHProtocolError("invalid object size limit")
        request_frame = verifier.canonical_json({
            "max_object_bytes": max_object_bytes,
            "object_key": object_key,
        }).encode("utf-8")
        if len(request_frame) > MAX_PROTOCOL_HEADER:
            raise SSHProtocolError("ssh direct OSS request exceeds protocol limit")
        self.ensure_process()
        assert self.stdin is not None and self.stdout is not None
        try:
            self.stdin.write(struct.pack("!I", len(request_frame)) + request_frame)
            self.stdin.flush()
            response = read_json_frame(self.stdout)
        except OSError:
            self.mark_broken()
            raise
        valid_shape = (
            set(response) == {"detail", "mime", "sha256", "size", "status"}
            and isinstance(response["detail"], str)
            and isinstance(response["mime"], str)
            and isinstance(response["sha256"], str)
            and isinstance(response["size"], int)
            and not isinstance(response["size"], bool)
            and response["size"] >= 0
            and isinstance(response["status"], int)
            and not isinstance(response["status"], bool)
            and 100 <= response["status"] <= 599
        )
        if not valid_shape:
            self.mark_broken()
            raise SSHProtocolError("invalid ssh direct OSS metadata")
        status = response["status"]
        detail = response["detail"]
        if status == 200:
            valid_success = (
                detail == ""
                and bool(verifier.normalize_mime(response["mime"]))
                and bool(verifier.SHA256.fullmatch(response["sha256"]))
                and response["size"] <= max_object_bytes
            )
            if not valid_success:
                self.mark_broken()
                raise SSHProtocolError("invalid ssh direct OSS success metadata")
            return ObjectMetadata(
                size=response["size"],
                mime_type=verifier.normalize_mime(response["mime"]),
                sha256=response["sha256"],
            )
        valid_failure = (
            response["mime"] == ""
            and response["sha256"] == ""
            and response["size"] == 0
            and (
                detail in SAFE_CHECKPOINT_FAILURE_DETAILS
                or bool(SAFE_HTTP_STATUS_DETAIL.fullmatch(detail))
            )
        )
        if not valid_failure:
            self.mark_broken()
            raise SSHProtocolError("invalid ssh direct OSS failure metadata")
        if detail in {"declared_size_exceeds_limit", "stream_size_exceeds_limit"}:
            code = "object_manifest.object_too_large"
        elif detail == "invalid_content_length":
            code = "object_manifest.invalid_content_length"
        elif detail == "stored MIME is unavailable":
            code = "object_manifest.mime_unavailable"
        elif detail == "non_identity_content_encoding":
            code = "object_manifest.content_encoding_unsupported"
        elif detail == "content_length_differs_from_stream":
            code = "object_manifest.size_inconsistent"
        elif detail in {"http_status=404", "http_status=410"}:
            code = "object_manifest.missing"
        else:
            code = "object_manifest.unreadable"
        raise HydrationFailure(code, detail)

    def mark_broken(self) -> None:
        self.broken = True

    def close(self) -> None:
        if self.process is None:
            return
        try:
            if not self.broken and self.process.poll() is None and self.stdin is not None:
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

    def __enter__(self) -> "PersistentSSHDirectOSSAdapter":
        return self

    def __exit__(self, *_args) -> None:
        self.close()


def atomic_write_bytes(path: pathlib.Path, payload: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary: pathlib.Path | None = None
    try:
        with tempfile.NamedTemporaryFile(
            prefix=path.name + ".", suffix=".tmp", dir=path.parent, delete=False
        ) as handle:
            temporary = pathlib.Path(handle.name)
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
        temporary = None
        try:
            directory_fd = os.open(path.parent, os.O_RDONLY)
        except OSError:
            return
        try:
            os.fsync(directory_fd)
        finally:
            os.close(directory_fd)
    finally:
        if temporary is not None:
            try:
                temporary.unlink()
            except FileNotFoundError:
                pass


def atomic_write_json(path: pathlib.Path, value: Any) -> None:
    atomic_write_bytes(path, (verifier.canonical_json(value) + "\n").encode("utf-8"))


def adapter_kind(config: verifier.VerifierConfig, storage_adapter: str) -> tuple[str, verifier.HTTPReadAdapter] | None:
    adapter = config.adapter_for(storage_adapter)
    if adapter is None:
        return None
    if adapter is config.upload:
        return "upload", adapter
    if adapter is config.oss:
        return "oss", adapter
    # VerifierConfig currently exposes only these two adapters.  Fail closed if
    # a future implementation returns an unidentifiable adapter.
    return None


def adapter_fingerprints(config: verifier.VerifierConfig) -> dict[str, str]:
    result: dict[str, str] = {}
    for kind, adapter in (("upload", config.upload), ("oss", config.oss)):
        if adapter is not None:
            # Bind resume data to the object origin without recording its URL.
            # The direct-OSS adapter additionally binds to the remote endpoint,
            # bucket, and access-key-id through a one-way handshake fingerprint.
            fingerprint_reader = getattr(adapter, "origin_fingerprint", None)
            if callable(fingerprint_reader):
                result[kind] = fingerprint_reader()
            else:
                result[kind] = hashlib.sha256(
                    adapter.base_url.encode("utf-8")
                ).hexdigest()
    return result


def validate_hydration_row(row: Any, line_no: int) -> str | None:
    """Return ``missing`` or ``complete``; raise on an invalid input row."""
    if not isinstance(row, dict) or set(row) != verifier.REQUIRED_FIELDS:
        raise ValueError(f"invalid manifest row {line_no}: exact field contract mismatch")
    sha = row.get("sha256")
    if sha not in {"", None}:
        problems = verifier.validate_contract(row, line_no)
        if problems:
            raise ValueError(f"invalid manifest row {line_no}: {problems[0]['violation_code']}")
        return "complete"

    owner_kind = row.get("owner_kind")
    owner_id = row.get("owner_id")
    task_id = row.get("task_id")
    expected_entity = f"{owner_kind}:{owner_id}"
    valid = (
        owner_kind in {"task_asset", "reference_file_ref"}
        and isinstance(owner_id, int) and not isinstance(owner_id, bool) and owner_id > 0
        and isinstance(task_id, int) and not isinstance(task_id, bool) and task_id > 0
        and row.get("entity_key") == expected_entity
        and isinstance(row.get("storage_ref_id"), str) and bool(row["storage_ref_id"].strip())
        and isinstance(row.get("storage_adapter"), str) and bool(row["storage_adapter"].strip())
        and isinstance(row.get("object_key"), str) and verifier.valid_object_key(row["object_key"])
        and (row.get("size") is None or (
            isinstance(row["size"], int) and not isinstance(row["size"], bool) and row["size"] >= 0
        ))
        and (row.get("mime_type") is None or isinstance(row["mime_type"], str))
        and isinstance(row.get("status"), str) and bool(row["status"].strip())
        and row.get("is_placeholder") is False
    )
    if not valid:
        raise ValueError(f"invalid manifest row {line_no}: invalid field value")
    return "missing"


def read_manifest(path: pathlib.Path) -> tuple[list[dict[str, Any]], str]:
    if not path.is_file():
        raise ValueError("input manifest is missing")
    manifest_sha = verifier.sha256_file(path)
    rows: list[dict[str, Any]] = []
    seen_entities: set[str] = set()
    try:
        with path.open("r", encoding="utf-8") as handle:
            for line_no, line in enumerate(handle, 1):
                if not line.strip():
                    continue
                try:
                    row = json.loads(line)
                except json.JSONDecodeError as exc:
                    raise ValueError(f"invalid manifest row {line_no}: invalid JSON") from exc
                validate_hydration_row(row, line_no)
                entity = row["entity_key"]
                if entity in seen_entities:
                    raise ValueError(f"invalid manifest row {line_no}: duplicate entity")
                seen_entities.add(entity)
                rows.append(row)
    except UnicodeDecodeError as exc:
        raise ValueError("input manifest is not UTF-8") from exc
    if not rows:
        raise ValueError("input manifest contains no objects")
    return rows, manifest_sha


def checkpoint_key(kind: str, object_key: str) -> str:
    return kind + "\x1f" + object_key


def checkpoint_record(kind: str, object_key: str, metadata: ObjectMetadata) -> dict[str, Any]:
    return {
        "adapter_kind": kind,
        "object_key": object_key,
        "size": metadata.size,
        "mime_type": metadata.mime_type,
        "sha256": metadata.sha256,
    }


def checkpoint_failure_record(
    kind: str,
    object_key: str,
    code: str,
    detail: str,
) -> dict[str, Any]:
    if (
        not SAFE_CHECKPOINT_FAILURE_CODE.fullmatch(code)
        or not (
            detail in SAFE_CHECKPOINT_FAILURE_DETAILS
            or SAFE_HTTP_STATUS_DETAIL.fullmatch(detail)
        )
    ):
        raise ValueError("unsafe failure cannot be written to checkpoint")
    return {
        "adapter_kind": kind,
        "object_key": object_key,
        "violation_code": code,
        "detail": detail,
    }


def checkpoint_failure_record_sha256(record: dict[str, Any]) -> str:
    return hashlib.sha256(
        verifier.canonical_json(record).encode("utf-8")
    ).hexdigest()


def _authorization_artifact_path(
    authorization_path: pathlib.Path,
    value: str,
) -> pathlib.Path:
    candidate = pathlib.Path(value)
    if not candidate.is_absolute():
        candidate = authorization_path.parent / candidate
    return candidate.resolve()


def _load_json_without_duplicate_keys(path: pathlib.Path) -> Any:
    def build_object(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
        result: dict[str, Any] = {}
        for key, value in pairs:
            if key in result:
                raise ValueError("JSON contains a duplicate object key")
            result[key] = value
        return result

    return json.loads(
        path.read_text(encoding="utf-8"),
        object_pairs_hook=build_object,
    )


def _validate_reprobe(
    *,
    authorization_path: pathlib.Path,
    reprobe: Any,
    expected_key: str,
    config: verifier.VerifierConfig,
) -> tuple[tuple[int, str, str], pathlib.Path, pathlib.Path]:
    expected_fields = {
        "evidence_path",
        "evidence_sha256",
        "artifact_path",
        "artifact_sha256",
    }
    if (
        not isinstance(reprobe, dict)
        or set(reprobe) != expected_fields
        or not isinstance(reprobe["evidence_path"], str)
        or not reprobe["evidence_path"]
        or not isinstance(reprobe["artifact_path"], str)
        or not reprobe["artifact_path"]
        or not isinstance(reprobe["evidence_sha256"], str)
        or not verifier.SHA256.fullmatch(reprobe["evidence_sha256"])
        or not isinstance(reprobe["artifact_sha256"], str)
        or not verifier.SHA256.fullmatch(reprobe["artifact_sha256"])
    ):
        raise ValueError("invalid failure retry authorization reprobe")

    evidence_path = _authorization_artifact_path(
        authorization_path, reprobe["evidence_path"]
    )
    artifact_path = _authorization_artifact_path(
        authorization_path, reprobe["artifact_path"]
    )
    if evidence_path == artifact_path:
        raise ValueError("failure retry reprobe evidence and artifact paths must differ")
    if (
        not evidence_path.is_file()
        or verifier.sha256_file(evidence_path) != reprobe["evidence_sha256"]
        or not artifact_path.is_file()
        or verifier.sha256_file(artifact_path) != reprobe["artifact_sha256"]
    ):
        raise ValueError("failure retry reprobe artifact hash mismatch")

    evidence = _load_json_without_duplicate_keys(evidence_path)
    evidence_without_hash = (
        {key: value for key, value in evidence.items() if key != "evidence_hash"}
        if isinstance(evidence, dict)
        else {}
    )
    evidence_hash = hashlib.sha256(
        verifier.canonical_json(evidence_without_hash).encode("utf-8")
    ).hexdigest()
    if (
        not isinstance(evidence, dict)
        or evidence.get("status") != "PASS"
        or evidence.get("failure_count") != 0
        or evidence.get("configured_target_row_count") != 1
        or evidence.get("unique_target_count") != 1
        or evidence.get("read_only_get_count") != 1
        or evidence.get("hydrated_row_count") != 1
        or evidence.get("hydrated_manifest_sha256") != reprobe["artifact_sha256"]
        or evidence.get("evidence_hash") != evidence_hash
    ):
        raise ValueError("failure retry reprobe evidence is not a one-target PASS")

    try:
        artifact_lines = artifact_path.read_text(encoding="utf-8").splitlines()
    except UnicodeDecodeError as exc:
        raise ValueError("failure retry reprobe artifact is not UTF-8") from exc
    if len(artifact_lines) != 1:
        raise ValueError("failure retry reprobe artifact must contain one row")
    try:
        row = _load_json_without_duplicate_keys(artifact_path)
    except json.JSONDecodeError as exc:
        raise ValueError("failure retry reprobe artifact is invalid JSONL") from exc
    validate_hydration_row(row, 1)
    resolved = adapter_kind(config, row["storage_adapter"])
    if resolved is None:
        raise ValueError("failure retry reprobe adapter is not configured")
    kind, _adapter = resolved
    if checkpoint_key(kind, row["object_key"]) != expected_key:
        raise ValueError("failure retry reprobe targets a different object")
    if not row["sha256"]:
        raise ValueError("failure retry reprobe artifact is not hydrated")
    return (
        (row["size"], verifier.normalize_mime(row["mime_type"]), row["sha256"]),
        evidence_path,
        artifact_path,
    )


def load_failure_retry_authorization(
    path: pathlib.Path | None,
    *,
    input_sha: str,
    checkpoint_sha: str,
    failed_targets: dict[str, dict[str, Any]],
    config: verifier.VerifierConfig,
) -> tuple[set[str], str]:
    if path is None:
        return set(), ZERO_SHA256
    if checkpoint_sha == ZERO_SHA256:
        raise ValueError("failure retry authorization requires an existing checkpoint")
    raw = _load_json_without_duplicate_keys(path)
    expected_fields = {
        "schema_version",
        "authorization_type",
        "input_manifest_sha256",
        "checkpoint_sha256",
        "failure_retries",
        "authorization_sha256",
    }
    valid_header = (
        isinstance(raw, dict)
        and set(raw) == expected_fields
        and raw.get("schema_version") == FAILURE_RETRY_AUTHORIZATION_SCHEMA_VERSION
        and raw.get("authorization_type") == FAILURE_RETRY_AUTHORIZATION_TYPE
        and isinstance(raw.get("input_manifest_sha256"), str)
        and bool(verifier.SHA256.fullmatch(raw["input_manifest_sha256"]))
        and isinstance(raw.get("checkpoint_sha256"), str)
        and bool(verifier.SHA256.fullmatch(raw["checkpoint_sha256"]))
        and isinstance(raw.get("failure_retries"), list)
        and bool(raw["failure_retries"])
        and isinstance(raw.get("authorization_sha256"), str)
        and bool(verifier.SHA256.fullmatch(raw["authorization_sha256"]))
    )
    if not valid_header:
        raise ValueError("invalid failure retry authorization schema")
    payload = {
        key: value for key, value in raw.items() if key != "authorization_sha256"
    }
    authorization_sha = hashlib.sha256(
        verifier.canonical_json(payload).encode("utf-8")
    ).hexdigest()
    if raw["authorization_sha256"] != authorization_sha:
        raise ValueError("failure retry authorization self-hash mismatch")
    if raw["input_manifest_sha256"] != input_sha:
        raise ValueError("failure retry authorization input manifest mismatch")
    if raw["checkpoint_sha256"] != checkpoint_sha:
        raise ValueError("failure retry authorization checkpoint mismatch")

    failed_by_sha: dict[str, str] = {}
    for key, record in failed_targets.items():
        record_sha = checkpoint_failure_record_sha256(record)
        if record_sha in failed_by_sha:
            raise ValueError("checkpoint failure record hash collision")
        failed_by_sha[record_sha] = key

    authorized: set[str] = set()
    previous_failure_sha = ""
    for item in raw["failure_retries"]:
        if (
            not isinstance(item, dict)
            or set(item) != {"failure_record_sha256", "reprobes"}
            or not isinstance(item["failure_record_sha256"], str)
            or not verifier.SHA256.fullmatch(item["failure_record_sha256"])
            or not isinstance(item["reprobes"], list)
            or len(item["reprobes"]) != 2
        ):
            raise ValueError("invalid failure retry authorization entry")
        failure_sha = item["failure_record_sha256"]
        if failure_sha <= previous_failure_sha:
            raise ValueError(
                "failure retry authorization entries must be unique and sorted"
            )
        previous_failure_sha = failure_sha
        key = failed_by_sha.get(failure_sha)
        if key is None:
            raise ValueError(
                "failure retry authorization does not match a checkpoint failure"
            )
        validated_reprobes = [
            _validate_reprobe(
                authorization_path=path,
                reprobe=reprobe,
                expected_key=key,
                config=config,
            )
            for reprobe in item["reprobes"]
        ]
        metadata = [value[0] for value in validated_reprobes]
        reprobe_paths = [(value[1], value[2]) for value in validated_reprobes]
        if len(set(reprobe_paths)) != 2:
            raise ValueError("failure retry authorization requires two reprobe files")
        if metadata[0] != metadata[1]:
            raise ValueError("failure retry reprobes do not agree")
        authorized.add(key)
    return authorized, authorization_sha


def checkpoint_document(
    input_sha: str,
    fingerprints: dict[str, str],
    completed: dict[str, dict[str, Any]],
    failed: dict[str, dict[str, Any]],
) -> dict[str, Any]:
    return {
        "schema_version": CHECKPOINT_SCHEMA_VERSION,
        "input_manifest_sha256": input_sha,
        "adapter_fingerprints": fingerprints,
        "completed": [completed[key] for key in sorted(completed)],
        "failed": [failed[key] for key in sorted(failed)],
    }


def load_checkpoint(
    path: pathlib.Path,
    input_sha: str,
    fingerprints: dict[str, str],
) -> tuple[dict[str, dict[str, Any]], dict[str, dict[str, Any]]]:
    if not path.exists():
        return {}, {}
    raw = json.loads(path.read_text(encoding="utf-8"))
    schema_version = raw.get("schema_version") if isinstance(raw, dict) else None
    if (
        not isinstance(raw, dict)
        or schema_version not in {1, CHECKPOINT_SCHEMA_VERSION}
        or raw.get("input_manifest_sha256") != input_sha
        or raw.get("adapter_fingerprints") != fingerprints
        or not isinstance(raw.get("completed"), list)
        or (schema_version == CHECKPOINT_SCHEMA_VERSION and not isinstance(raw.get("failed"), list))
    ):
        raise ValueError("checkpoint does not match the input manifest and adapter origins")
    completed: dict[str, dict[str, Any]] = {}
    for index, record in enumerate(raw["completed"]):
        valid = (
            isinstance(record, dict)
            and set(record) == {"adapter_kind", "object_key", "size", "mime_type", "sha256"}
            and record["adapter_kind"] in {"upload", "oss"}
            and isinstance(record["object_key"], str) and verifier.valid_object_key(record["object_key"])
            and isinstance(record["size"], int) and not isinstance(record["size"], bool) and record["size"] >= 0
            and isinstance(record["mime_type"], str) and bool(verifier.normalize_mime(record["mime_type"]))
            and isinstance(record["sha256"], str) and bool(verifier.SHA256.fullmatch(record["sha256"]))
        )
        if not valid:
            raise ValueError(f"invalid checkpoint record {index}")
        key = checkpoint_key(record["adapter_kind"], record["object_key"])
        if key in completed:
            raise ValueError("checkpoint contains a duplicate object")
        completed[key] = record
    failed: dict[str, dict[str, Any]] = {}
    for index, record in enumerate(raw.get("failed", [])):
        valid = (
            isinstance(record, dict)
            and set(record) == {
                "adapter_kind", "object_key", "violation_code", "detail",
            }
            and record["adapter_kind"] in {"upload", "oss"}
            and isinstance(record["object_key"], str)
            and verifier.valid_object_key(record["object_key"])
            and isinstance(record["violation_code"], str)
            and bool(SAFE_CHECKPOINT_FAILURE_CODE.fullmatch(record["violation_code"]))
            and isinstance(record["detail"], str)
            and (
                record["detail"] in SAFE_CHECKPOINT_FAILURE_DETAILS
                or bool(SAFE_HTTP_STATUS_DETAIL.fullmatch(record["detail"]))
            )
        )
        if not valid:
            raise ValueError(f"invalid checkpoint failure record {index}")
        key = checkpoint_key(record["adapter_kind"], record["object_key"])
        if key in failed or key in completed:
            raise ValueError("checkpoint contains a duplicate object result")
        failed[key] = record
    return completed, failed


def stream_metadata(response: BinaryIO, max_object_bytes: int) -> ObjectMetadata:
    declared_size, mime_type = verifier.response_metadata(response)
    content_encoding = (response.headers.get("Content-Encoding") or "").strip().lower()
    if content_encoding not in {"", "identity"}:
        raise HydrationFailure(
            "object_manifest.content_encoding_unsupported",
            "non_identity_content_encoding",
        )
    if declared_size is not None and declared_size < 0:
        raise HydrationFailure("object_manifest.invalid_content_length", "invalid_content_length")
    if declared_size is not None and declared_size > max_object_bytes:
        raise HydrationFailure("object_manifest.object_too_large", "declared_size_exceeds_limit")
    if not mime_type:
        raise HydrationFailure("object_manifest.mime_unavailable", "stored MIME is unavailable")

    digest = hashlib.sha256()
    total = 0
    while True:
        chunk = response.read(verifier.CHUNK_SIZE)
        if not chunk:
            break
        total += len(chunk)
        if total > max_object_bytes:
            raise HydrationFailure("object_manifest.object_too_large", "stream_size_exceeds_limit")
        digest.update(chunk)
    if declared_size is not None and declared_size != total:
        raise HydrationFailure("object_manifest.size_inconsistent", "content_length_differs_from_stream")
    return ObjectMetadata(size=total, mime_type=mime_type, sha256=digest.hexdigest())


def get_object_metadata(
    adapter: verifier.HTTPReadAdapter,
    object_key: str,
    max_object_bytes: int,
) -> ObjectMetadata:
    metadata_reader = getattr(adapter, "get_metadata", None)
    if callable(metadata_reader):
        try:
            return metadata_reader(object_key, max_object_bytes)
        except HydrationFailure:
            raise
        except (OSError, TimeoutError, urllib.error.URLError) as exc:
            raise HydrationFailure(
                "object_manifest.unreadable",
                verifier.safe_http_error(exc),
            ) from exc
    try:
        with adapter.request("GET", object_key) as response:
            return stream_metadata(response, max_object_bytes)
    except HydrationFailure:
        raise
    except (OSError, TimeoutError, urllib.error.URLError, http.client.HTTPException) as exc:
        detail = verifier.safe_http_error(exc)
        if isinstance(exc, urllib.error.HTTPError):
            exc.close()
        code = "object_manifest.missing" if (
            isinstance(exc, urllib.error.HTTPError) and exc.code in {404, 410}
        ) else "object_manifest.unreadable"
        raise HydrationFailure(code, detail) from exc


def safe_failure(
    kind: str,
    object_key: str,
    entities: list[str],
    code: str,
    detail: str,
) -> dict[str, Any]:
    return {
        "adapter_kind": kind,
        "object_key_sha256": hashlib.sha256(object_key.encode("utf-8")).hexdigest(),
        "entity_keys": sorted(entities),
        "violation_code": code,
        "detail": detail,
    }


def write_hydrated_manifest(
    path: pathlib.Path,
    rows: list[dict[str, Any]],
    completed: dict[str, dict[str, Any]],
    row_targets: dict[int, str],
) -> str:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary: pathlib.Path | None = None
    try:
        with tempfile.NamedTemporaryFile(
            mode="w", encoding="utf-8", newline="\n",
            prefix=path.name + ".", suffix=".tmp", dir=path.parent, delete=False,
        ) as handle:
            temporary = pathlib.Path(handle.name)
            for index, source in enumerate(rows):
                row = dict(source)
                target = row_targets.get(index)
                if target is not None:
                    record = completed[target]
                    row["size"] = record["size"]
                    row["mime_type"] = record["mime_type"]
                    row["sha256"] = record["sha256"]
                handle.write(verifier.canonical_json(row) + "\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
        temporary = None
    finally:
        if temporary is not None:
            try:
                temporary.unlink()
            except FileNotFoundError:
                pass
    return verifier.sha256_file(path)


def result_document(
    *,
    status: str,
    input_sha: str,
    output_sha: str,
    checkpoint_sha: str,
    row_count: int,
    already_complete: int,
    missing_rows: int,
    target_row_count: int,
    unique_targets: int,
    resumed_targets: int,
    resumed_failure_targets: int,
    retried_transient_failure_targets: int,
    retried_authorized_failure_targets: int,
    failure_retry_authorization_sha: str,
    historical_unavailable_exception_attestation_sha: str,
    historical_unavailable_exception_mapping_sha: str,
    historical_unavailable_exception_mapping_row_hash: str,
    historical_unavailable_exception_count: int,
    get_count: int,
    hydrated_rows: int,
    failures: list[dict[str, Any]],
) -> dict[str, Any]:
    result = {
        "schema_version": SCHEMA_VERSION,
        "status": status,
        "input_manifest_sha256": input_sha,
        "hydrated_manifest_sha256": output_sha,
        "checkpoint_sha256": checkpoint_sha,
        "row_count": row_count,
        "already_complete_count": already_complete,
        "missing_sha256_count": missing_rows,
        "configured_target_row_count": target_row_count,
        "unique_target_count": unique_targets,
        "resumed_target_count": resumed_targets,
        "resumed_failure_target_count": resumed_failure_targets,
        "retried_transient_failure_target_count": (
            retried_transient_failure_targets
        ),
        "retried_authorized_failure_target_count": (
            retried_authorized_failure_targets
        ),
        "failure_retry_authorization_sha256": (
            failure_retry_authorization_sha
        ),
        "historical_unavailable_exception_attestation_sha256": (
            historical_unavailable_exception_attestation_sha
        ),
        "historical_unavailable_exception_mapping_sha256": (
            historical_unavailable_exception_mapping_sha
        ),
        "historical_unavailable_exception_mapping_row_hash": (
            historical_unavailable_exception_mapping_row_hash
        ),
        "historical_unavailable_exception_count": (
            historical_unavailable_exception_count
        ),
        "read_only_get_count": get_count,
        "hydrated_row_count": hydrated_rows,
        "deduplicated_get_count": max(0, target_row_count - unique_targets),
        "failure_count": len(failures),
        "failures": sorted(
            failures,
            key=lambda item: (
                item["adapter_kind"], item["object_key_sha256"],
                item["violation_code"], item["detail"],
            ),
        ),
    }
    result["evidence_hash"] = hashlib.sha256(
        verifier.canonical_json(result).encode("utf-8")
    ).hexdigest()
    return result


def hydrate_manifest(
    manifest_path: pathlib.Path,
    output_path: pathlib.Path,
    checkpoint_path: pathlib.Path,
    config: verifier.VerifierConfig,
    *,
    checkpoint_every: int = DEFAULT_CHECKPOINT_EVERY,
    max_object_bytes: int = DEFAULT_MAX_OBJECT_BYTES,
    workers: int = 1,
    retry_transient_failures: bool = False,
    failure_retry_authorization_path: pathlib.Path | None = None,
    historical_unavailable_exception_path: pathlib.Path | None = None,
) -> dict[str, Any]:
    if checkpoint_every <= 0:
        raise ValueError("checkpoint_every must be positive")
    if max_object_bytes <= 0:
        raise ValueError("max_object_bytes must be positive")
    if workers <= 0 or workers > 16:
        raise ValueError("workers must be in [1, 16]")

    rows, input_sha = read_manifest(manifest_path)
    historical_unavailable_attestation_sha = ZERO_SHA256
    historical_unavailable_mapping_sha = ZERO_SHA256
    historical_unavailable_mapping_row_hash = ZERO_SHA256
    historical_unavailable_count = 0
    historical_unavailable_entity = ""
    historical_unavailable_checkpoint_key = ""
    if historical_unavailable_exception_path is not None:
        attestation, exception, historical_unavailable_attestation_sha = (
            historical_unavailable_exception.load_attestation(
                historical_unavailable_exception_path,
                manifest_path=manifest_path,
            )
        )
        historical_unavailable_mapping_sha = attestation["mapping_sha256"]
        historical_unavailable_mapping_row_hash = attestation["mapping_row_hash"]
        historical_unavailable_count = attestation["exception_count"]
        historical_unavailable_entity = exception["entity_key"]
        exception_rows = [
            row for row in rows
            if row["entity_key"] == historical_unavailable_entity
        ]
        if len(exception_rows) != 1 or exception_rows[0]["sha256"] != "":
            raise ValueError(
                "historical-unavailable exception does not identify one empty-SHA row"
            )
        normalized_adapter = (
            exception_rows[0]["storage_adapter"].strip().lower()
        )
        if normalized_adapter in verifier.UPLOAD_ADAPTERS:
            exception_adapter_kind = "upload"
        elif normalized_adapter in verifier.OSS_ADAPTERS:
            exception_adapter_kind = "oss"
        else:
            raise ValueError(
                "historical-unavailable exception uses an unsupported adapter"
            )
        historical_unavailable_checkpoint_key = checkpoint_key(
            exception_adapter_kind, exception_rows[0]["object_key"]
        )
    fingerprints = adapter_fingerprints(config)
    checkpoint_input_sha = (
        verifier.sha256_file(checkpoint_path)
        if checkpoint_path.is_file()
        else ZERO_SHA256
    )
    completed, failed_targets = load_checkpoint(checkpoint_path, input_sha, fingerprints)
    targets: dict[str, dict[str, Any]] = {}
    row_targets: dict[int, str] = {}
    already_complete = 0
    failures: list[dict[str, Any]] = []

    for index, row in enumerate(rows):
        if row["sha256"]:
            already_complete += 1
            continue
        if row["entity_key"] == historical_unavailable_entity:
            continue
        resolved = adapter_kind(config, row["storage_adapter"])
        if resolved is None:
            failures.append(safe_failure(
                "unconfigured", row["object_key"], [row["entity_key"]],
                "object_manifest.adapter_not_configured",
                f'adapter class {row["storage_adapter"].strip().lower()} is not configured',
            ))
            continue
        kind, adapter = resolved
        key = checkpoint_key(kind, row["object_key"])
        row_targets[index] = key
        target = targets.setdefault(key, {
            "adapter_kind": kind,
            "adapter": adapter,
            "object_key": row["object_key"],
            "entities": [],
            "row_indexes": [],
        })
        target["entities"].append(row["entity_key"])
        target["row_indexes"].append(index)

    if historical_unavailable_checkpoint_key:
        completed.pop(historical_unavailable_checkpoint_key, None)
        failed_targets.pop(historical_unavailable_checkpoint_key, None)
    unknown_checkpoint_results = (set(completed) | set(failed_targets)) - set(targets)
    if unknown_checkpoint_results:
        raise ValueError("checkpoint contains an object outside the current hydration targets")
    authorized_retry_failed, failure_retry_authorization_sha = (
        load_failure_retry_authorization(
            failure_retry_authorization_path,
            input_sha=input_sha,
            checkpoint_sha=checkpoint_input_sha,
            failed_targets=failed_targets,
            config=config,
        )
    )
    retryable_failed = {
        key
        for key in targets
        if (
            key in failed_targets
            and failed_targets[key]["detail"]
            in RETRYABLE_TRANSIENT_FAILURE_DETAILS
            and key not in authorized_retry_failed
        )
    }
    if retry_transient_failures:
        for key in retryable_failed:
            del failed_targets[key]
    else:
        retryable_failed = set()
    for key in authorized_retry_failed:
        del failed_targets[key]
    applicable_completed = {key for key in targets if key in completed}
    applicable_failed = {key for key in targets if key in failed_targets}
    resumed_targets = len(applicable_completed)
    resumed_failure_targets = len(applicable_failed)
    retried_transient_failure_targets = len(retryable_failed)
    retried_authorized_failure_targets = len(authorized_retry_failed)
    target_row_count = sum(len(target["row_indexes"]) for target in targets.values())
    for key in sorted(applicable_failed):
        record = failed_targets[key]
        target = targets[key]
        failures.append(safe_failure(
            target["adapter_kind"], target["object_key"], target["entities"],
            record["violation_code"], record["detail"],
        ))
    pending_since_checkpoint = 0
    get_count = 0
    interrupted = False

    def checkpoint_if_needed() -> None:
        nonlocal pending_since_checkpoint
        if pending_since_checkpoint < checkpoint_every:
            return
        atomic_write_json(
            checkpoint_path,
            checkpoint_document(
                input_sha, fingerprints, completed, failed_targets
            ),
        )
        pending_since_checkpoint = 0

    def record_failure(key: str, exc: HydrationFailure) -> None:
        nonlocal pending_since_checkpoint
        target = targets[key]
        failed_targets[key] = checkpoint_failure_record(
            target["adapter_kind"], target["object_key"], exc.code, exc.detail,
        )
        failures.append(safe_failure(
            target["adapter_kind"], target["object_key"], target["entities"],
            exc.code, exc.detail,
        ))
        pending_since_checkpoint += 1
        checkpoint_if_needed()

    def record_success(key: str, metadata: ObjectMetadata) -> None:
        nonlocal pending_since_checkpoint
        target = targets[key]
        completed[key] = checkpoint_record(
            target["adapter_kind"], target["object_key"], metadata
        )
        pending_since_checkpoint += 1
        checkpoint_if_needed()

    pending_keys = [
        key
        for key in sorted(targets)
        if key not in completed and key not in failed_targets
    ]
    if workers == 1:
        for key in pending_keys:
            target = targets[key]
            get_count += 1
            try:
                metadata = get_object_metadata(
                    target["adapter"], target["object_key"], max_object_bytes
                )
            except KeyboardInterrupt:
                interrupted = True
                break
            except HydrationFailure as exc:
                record_failure(key, exc)
                continue
            record_success(key, metadata)
    elif pending_keys:
        adapters = {
            id(targets[key]["adapter"]): targets[key]["adapter"]
            for key in pending_keys
        }
        if len(adapters) != 1:
            raise ValueError(
                "parallel hydration requires one configured adapter origin"
            )
        base_adapter = next(iter(adapters.values()))
        clone_adapter = getattr(base_adapter, "clone", None)
        if not callable(clone_adapter):
            raise ValueError(
                "parallel hydration requires a cloneable read adapter"
            )
        expected_fingerprint = next(iter(fingerprints.values()))
        thread_state = threading.local()
        worker_adapters: list[Any] = []
        worker_adapters_lock = threading.Lock()
        attempted: set[str] = set()
        attempted_lock = threading.Lock()

        def initialize_worker() -> None:
            adapter = clone_adapter()
            fingerprint_reader = getattr(adapter, "origin_fingerprint", None)
            if (
                not callable(fingerprint_reader)
                or fingerprint_reader() != expected_fingerprint
            ):
                close = getattr(adapter, "close", None)
                if callable(close):
                    close()
                raise ValueError("parallel adapter origin fingerprint differs")
            thread_state.adapter = adapter
            with worker_adapters_lock:
                worker_adapters.append(adapter)

        def fetch(key: str) -> ObjectMetadata:
            with attempted_lock:
                attempted.add(key)
            target = targets[key]
            return get_object_metadata(
                thread_state.adapter, target["object_key"], max_object_bytes
            )

        executor = concurrent.futures.ThreadPoolExecutor(
            max_workers=workers,
            initializer=initialize_worker,
            thread_name_prefix="object-hydration",
        )
        futures = {executor.submit(fetch, key): key for key in pending_keys}
        try:
            for future in concurrent.futures.as_completed(futures):
                key = futures[future]
                try:
                    metadata = future.result()
                except concurrent.futures.CancelledError:
                    interrupted = True
                    for pending in futures:
                        pending.cancel()
                    break
                except HydrationFailure as exc:
                    record_failure(key, exc)
                except concurrent.futures.BrokenExecutor:
                    interrupted = True
                    for pending in futures:
                        pending.cancel()
                    break
                else:
                    record_success(key, metadata)
        except KeyboardInterrupt:
            interrupted = True
            for future in futures:
                future.cancel()
        finally:
            executor.shutdown(wait=True, cancel_futures=interrupted)
            for adapter in worker_adapters:
                close = getattr(adapter, "close", None)
                if callable(close):
                    close()
        get_count = len(attempted)

    # Flush all successful work on normal exit, failure, or Ctrl-C.
    atomic_write_json(
        checkpoint_path,
        checkpoint_document(input_sha, fingerprints, completed, failed_targets),
    )
    checkpoint_sha = verifier.sha256_file(checkpoint_path)
    hydrated_rows = sum(
        len(targets[key]["row_indexes"]) for key in targets if key in completed
    )
    complete = not failures and not interrupted and all(key in completed for key in targets)
    output_sha = ZERO_SHA256
    if complete:
        output_sha = write_hydrated_manifest(output_path, rows, completed, row_targets)
        status = "PASS"
    elif interrupted:
        status = "INTERRUPTED"
    else:
        status = "BLOCKED"
    return result_document(
        status=status,
        input_sha=input_sha,
        output_sha=output_sha,
        checkpoint_sha=checkpoint_sha,
        row_count=len(rows),
        already_complete=already_complete,
        missing_rows=len(rows) - already_complete,
        target_row_count=target_row_count,
        unique_targets=len(targets),
        resumed_targets=resumed_targets,
        resumed_failure_targets=resumed_failure_targets,
        retried_transient_failure_targets=(
            retried_transient_failure_targets
        ),
        retried_authorized_failure_targets=(
            retried_authorized_failure_targets
        ),
        failure_retry_authorization_sha=(
            failure_retry_authorization_sha
        ),
        historical_unavailable_exception_attestation_sha=(
            historical_unavailable_attestation_sha
        ),
        historical_unavailable_exception_mapping_sha=(
            historical_unavailable_mapping_sha
        ),
        historical_unavailable_exception_mapping_row_hash=(
            historical_unavailable_mapping_row_hash
        ),
        historical_unavailable_exception_count=(
            historical_unavailable_count
        ),
        get_count=get_count,
        hydrated_rows=hydrated_rows,
        failures=failures,
    )


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("manifest_jsonl")
    parser.add_argument("hydrated_manifest_jsonl")
    parser.add_argument("evidence_json")
    parser.add_argument("--checkpoint", required=True, help="run-scoped atomic resume checkpoint")
    parser.add_argument("--upload-base-url", help="upload-service /files URL prefix; may use AB_UPLOAD_READ_BASE_URL")
    parser.add_argument("--upload-headers-file", help="JSON headers file; may use AB_UPLOAD_READ_HEADERS_FILE")
    parser.add_argument("--oss-base-url", help="read-only OSS/gateway URL prefix; may use AB_OSS_READ_BASE_URL")
    parser.add_argument("--oss-headers-file", help="JSON headers file; may use AB_OSS_READ_HEADERS_FILE")
    parser.add_argument("--ssh-host", help="explicit SSH config host alias for a persistent remote adapter")
    parser.add_argument("--ssh-env-file", help="absolute remote env file containing upload-service or OSS configuration")
    parser.add_argument("--ssh-upload-base-url", help="explicit remote-internal upload-service /files URL prefix")
    parser.add_argument(
        "--ssh-direct-oss",
        action="store_true",
        help="stream OSS objects on the SSH host and return only size/MIME/SHA-256 metadata",
    )
    parser.add_argument(
        "--ssh-direct-oss-endpoint-override",
        help=(
            "explicit bare same-region Aliyun OSS internal endpoint for SSH "
            "direct reads; valid only with --ssh-direct-oss"
        ),
    )
    parser.add_argument("--timeout-seconds", type=float, default=30.0)
    parser.add_argument("--checkpoint-every", type=int, default=DEFAULT_CHECKPOINT_EVERY)
    parser.add_argument("--workers", type=int, default=1)
    parser.add_argument(
        "--retry-transient-failures",
        action="store_true",
        help=(
            "re-read only checkpointed timeout, TLS, connection, and transport "
            "read failures; HTTP and content-integrity failures remain sticky "
            "blockers"
        ),
    )
    parser.add_argument(
        "--failure-retry-authorization",
        help=(
            "exact self-hashed authorization JSON for individually identified "
            "sticky checkpoint failures with two matching one-target reprobes"
        ),
    )
    parser.add_argument(
        "--historical-unavailable-exception",
        help=(
            "exact historical-unavailable PASS attestation bound to the input "
            "manifest; excludes only task_asset:12323 from remote hydration"
        ),
    )
    parser.add_argument("--max-object-bytes", type=int, default=DEFAULT_MAX_OBJECT_BYTES)
    args = parser.parse_args(argv)
    if args.timeout_seconds <= 0 or args.timeout_seconds > 3600:
        parser.error("--timeout-seconds must be in (0, 3600]")
    if args.checkpoint_every <= 0:
        parser.error("--checkpoint-every must be positive")
    if args.workers <= 0 or args.workers > 16:
        parser.error("--workers must be in [1, 16]")
    if args.max_object_bytes <= 0:
        parser.error("--max-object-bytes must be positive")
    return args


def upload_adapter_from_args(
    args: argparse.Namespace,
) -> verifier.HTTPReadAdapter | PersistentSSHReadAdapter | PersistentSSHDirectOSSAdapter | None:
    if args.ssh_direct_oss:
        if not args.ssh_host or not args.ssh_env_file:
            raise ValueError("--ssh-direct-oss requires --ssh-host and --ssh-env-file")
        if args.ssh_upload_base_url:
            raise ValueError("--ssh-direct-oss cannot be combined with --ssh-upload-base-url")
        local_upload_configuration = (
            args.upload_base_url,
            args.upload_headers_file,
            os.environ.get("AB_UPLOAD_READ_BASE_URL", ""),
            os.environ.get("AB_UPLOAD_READ_HEADERS_FILE", ""),
            os.environ.get("AB_UPLOAD_READ_BEARER_TOKEN", ""),
        )
        if any(local_upload_configuration):
            raise ValueError(
                "local upload adapter configuration cannot be combined with "
                "the SSH direct OSS adapter"
            )
        return PersistentSSHDirectOSSAdapter(
            args.ssh_host,
            args.ssh_env_file,
            args.timeout_seconds,
            args.ssh_direct_oss_endpoint_override or "",
        )
    if args.ssh_direct_oss_endpoint_override:
        raise ValueError(
            "--ssh-direct-oss-endpoint-override requires --ssh-direct-oss"
        )
    ssh_values = (args.ssh_host, args.ssh_env_file, args.ssh_upload_base_url)
    if any(ssh_values) and not all(ssh_values):
        raise ValueError("--ssh-host, --ssh-env-file, and --ssh-upload-base-url are required together")
    if not any(ssh_values):
        return verifier.adapter_from_args("upload", args)
    local_upload_configuration = (
        args.upload_base_url,
        args.upload_headers_file,
        os.environ.get("AB_UPLOAD_READ_BASE_URL", ""),
        os.environ.get("AB_UPLOAD_READ_HEADERS_FILE", ""),
        os.environ.get("AB_UPLOAD_READ_BEARER_TOKEN", ""),
    )
    if any(local_upload_configuration):
        raise ValueError("local upload adapter configuration cannot be combined with the SSH adapter")
    return PersistentSSHReadAdapter(
        args.ssh_host,
        args.ssh_env_file,
        args.ssh_upload_base_url,
        args.timeout_seconds,
    )


def main(argv: list[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    manifest = pathlib.Path(args.manifest_jsonl)
    output = pathlib.Path(args.hydrated_manifest_jsonl)
    evidence_path = pathlib.Path(args.evidence_json)
    checkpoint = pathlib.Path(args.checkpoint)
    failure_retry_authorization = (
        pathlib.Path(args.failure_retry_authorization)
        if args.failure_retry_authorization
        else None
    )
    historical_unavailable_exception_path = (
        pathlib.Path(args.historical_unavailable_exception)
        if args.historical_unavailable_exception
        else None
    )
    protected_evidence_inputs = {manifest.resolve(), checkpoint.resolve()}
    if failure_retry_authorization is not None:
        protected_evidence_inputs.add(failure_retry_authorization.resolve())
    if historical_unavailable_exception_path is not None:
        protected_evidence_inputs.add(
            historical_unavailable_exception_path.resolve()
        )
    evidence_collides_with_input = (
        evidence_path.resolve() in protected_evidence_inputs
    )
    upload_adapter: (
        verifier.HTTPReadAdapter
        | PersistentSSHReadAdapter
        | PersistentSSHDirectOSSAdapter
        | None
    ) = None
    try:
        resolved = [
            item.resolve()
            for item in (manifest, output, evidence_path, checkpoint)
        ]
        if failure_retry_authorization is not None:
            resolved.append(failure_retry_authorization.resolve())
        if historical_unavailable_exception_path is not None:
            resolved.append(historical_unavailable_exception_path.resolve())
        if len(set(resolved)) != len(resolved):
            raise ValueError(
                "manifest, output, evidence, checkpoint, authorization, and "
                "historical-unavailable exception paths must differ"
            )
        upload_adapter = upload_adapter_from_args(args)
        config = verifier.VerifierConfig(
            upload=upload_adapter,
            oss=verifier.adapter_from_args("oss", args),
        )
        result = hydrate_manifest(
            manifest, output, checkpoint, config,
            checkpoint_every=args.checkpoint_every,
            max_object_bytes=args.max_object_bytes,
            workers=args.workers,
            retry_transient_failures=args.retry_transient_failures,
            failure_retry_authorization_path=failure_retry_authorization,
            historical_unavailable_exception_path=(
                historical_unavailable_exception_path
            ),
        )
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        input_sha = verifier.sha256_file(manifest) if manifest.is_file() else ZERO_SHA256
        failure = {
            "adapter_kind": "configuration",
            "object_key_sha256": ZERO_SHA256,
            "entity_keys": ["*"],
            "violation_code": "object_manifest.hydration_configuration",
            "detail": str(exc),
        }
        result = result_document(
            status="BLOCKED", input_sha=input_sha, output_sha=ZERO_SHA256,
            checkpoint_sha=verifier.sha256_file(checkpoint) if checkpoint.is_file() else ZERO_SHA256,
            row_count=0, already_complete=0, missing_rows=0, target_row_count=0, unique_targets=0,
            resumed_targets=0, resumed_failure_targets=0,
            retried_transient_failure_targets=0,
            retried_authorized_failure_targets=0,
            failure_retry_authorization_sha=ZERO_SHA256,
            historical_unavailable_exception_attestation_sha=ZERO_SHA256,
            historical_unavailable_exception_mapping_sha=ZERO_SHA256,
            historical_unavailable_exception_mapping_row_hash=ZERO_SHA256,
            historical_unavailable_exception_count=0,
            get_count=0, hydrated_rows=0, failures=[failure],
        )
    finally:
        if isinstance(
            upload_adapter,
            (PersistentSSHReadAdapter, PersistentSSHDirectOSSAdapter),
        ):
            upload_adapter.close()
    if evidence_collides_with_input:
        return 1
    atomic_write_json(evidence_path, result)
    return 0 if result["status"] == "PASS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
