#!/usr/bin/env python3
"""Read-only, fail-closed verifier for the V8 A/B object manifest.

Credentials are accepted only through environment variables or JSON header
files.  The verifier never emits request URLs, response bodies, or headers.
Every usable object is streamed through SHA-256; ETag is intentionally ignored.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import re
import socket
import ssl
import sys
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from typing import Any, BinaryIO

try:
    from scripts.ab import historical_unavailable_exception as historical_exception
except ModuleNotFoundError:
    import historical_unavailable_exception as historical_exception


SHA256 = re.compile(r"^[0-9a-f]{64}$")
REQUIRED_FIELDS = {
    "entity_key", "owner_kind", "owner_id", "task_id", "storage_ref_id",
    "storage_adapter", "object_key", "size", "mime_type", "sha256",
    "status", "is_placeholder",
}
UPLOAD_ADAPTERS = {"upload_service", "upload-service", "oss_upload_service"}
OSS_ADAPTERS = {"oss", "aliyun_oss"}
FORBIDDEN_HEADERS = {"host", "content-length", "transfer-encoding", "connection"}
SCHEMA_VERSION = 1
CHUNK_SIZE = 1024 * 1024


def canonical_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(CHUNK_SIZE):
            digest.update(chunk)
    return digest.hexdigest()


def violation(code: str, entity: str, detail: str, *, blocked: bool) -> dict[str, Any]:
    return {
        "violation_code": code,
        "entity_key": entity,
        "detail": detail,
        "hard_blocked": blocked,
    }


def validate_contract(row: Any, line_no: int) -> list[dict[str, Any]]:
    entity = str(row.get("entity_key", line_no)) if isinstance(row, dict) else str(line_no)
    if not isinstance(row, dict) or set(row) != REQUIRED_FIELDS:
        return [violation("object_manifest.invalid", entity, "exact field contract mismatch", blocked=True)]
    expected_entity = f'{row["owner_kind"]}:{row["owner_id"]}'
    valid = (
        row["owner_kind"] in {"task_asset", "reference_file_ref"}
        and isinstance(row["owner_id"], int) and not isinstance(row["owner_id"], bool) and row["owner_id"] > 0
        and isinstance(row["task_id"], int) and not isinstance(row["task_id"], bool) and row["task_id"] > 0
        and row["entity_key"] == expected_entity
        and isinstance(row["storage_ref_id"], str) and bool(row["storage_ref_id"].strip())
        and isinstance(row["storage_adapter"], str) and bool(row["storage_adapter"].strip())
        and isinstance(row["object_key"], str) and valid_object_key(row["object_key"])
        and isinstance(row["size"], int) and not isinstance(row["size"], bool) and row["size"] >= 0
        and isinstance(row["mime_type"], str) and bool(normalize_mime(row["mime_type"]))
        and isinstance(row["sha256"], str) and bool(SHA256.fullmatch(row["sha256"]))
        and isinstance(row["status"], str) and bool(row["status"].strip())
        and isinstance(row["is_placeholder"], bool)
    )
    if not valid:
        return [violation("object_manifest.invalid", entity, "invalid field value", blocked=True)]
    if row["is_placeholder"]:
        return [violation("object_manifest.placeholder", entity, "placeholder is not byte-verifiable", blocked=True)]
    return []


def valid_object_key(value: str) -> bool:
    if not value or value.startswith("/") or "\\" in value or "\x00" in value:
        return False
    return all(segment not in {"", ".", ".."} for segment in value.split("/"))


def normalize_mime(value: str | None) -> str:
    return (value or "").split(";", 1)[0].strip().lower()


def validate_base_url(value: str) -> str:
    parsed = urllib.parse.urlsplit(value.strip())
    if parsed.scheme not in {"http", "https"} or not parsed.netloc or parsed.username or parsed.password:
        raise ValueError("adapter base URL must be an http(s) URL without userinfo")
    if parsed.query or parsed.fragment:
        raise ValueError("adapter base URL must not contain query or fragment")
    return value.strip().rstrip("/")


def load_headers(path_value: str | None, token: str | None) -> dict[str, str]:
    headers: dict[str, str] = {}
    if path_value:
        path = pathlib.Path(path_value)
        raw = json.loads(path.read_text(encoding="utf-8"))
        if not isinstance(raw, dict) or not all(isinstance(k, str) and isinstance(v, str) for k, v in raw.items()):
            raise ValueError("headers file must be a JSON object of string values")
        headers.update(raw)
    if token:
        if any(name.lower() == "authorization" for name in headers):
            raise ValueError("authorization is configured twice")
        headers["Authorization"] = "Bearer " + token
    for name in headers:
        normalized = name.strip().lower()
        if not normalized or normalized in FORBIDDEN_HEADERS or "\r" in name or "\n" in name:
            raise ValueError("headers file contains a forbidden header name")
    if any("\r" in value or "\n" in value for value in headers.values()):
        raise ValueError("headers file contains an invalid header value")
    return headers


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):  # noqa: ANN001
        return None


@dataclass(frozen=True)
class HTTPReadAdapter:
    base_url: str
    headers: dict[str, str]
    timeout_seconds: float
    use_head: bool = False

    def object_url(self, object_key: str) -> str:
        encoded = "/".join(urllib.parse.quote(segment, safe="") for segment in object_key.split("/"))
        return f"{self.base_url}/{encoded}"

    def request(self, method: str, object_key: str):
        request = urllib.request.Request(self.object_url(object_key), method=method, headers=self.headers)
        opener = urllib.request.build_opener(NoRedirect())
        return opener.open(request, timeout=self.timeout_seconds)


@dataclass(frozen=True)
class VerifierConfig:
    upload: HTTPReadAdapter | None = None
    oss: HTTPReadAdapter | None = None

    def adapter_for(self, storage_adapter: str) -> HTTPReadAdapter | None:
        name = storage_adapter.strip().lower()
        if name in UPLOAD_ADAPTERS:
            return self.upload
        if name in OSS_ADAPTERS:
            return self.oss
        return None


def safe_http_error(exc: BaseException) -> str:
    if isinstance(exc, urllib.error.HTTPError):
        return f"http_status={exc.code}"
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


def http_failure_violation(exc: BaseException, entity: str) -> dict[str, Any]:
    if isinstance(exc, urllib.error.HTTPError) and exc.code in {404, 410}:
        return violation("object_manifest.missing", entity, f"http_status={exc.code}", blocked=False)
    return violation("object_manifest.unreadable", entity, safe_http_error(exc), blocked=True)


def response_metadata(response: Any) -> tuple[int | None, str]:
    length = response.headers.get("Content-Length")
    try:
        parsed_length = int(length) if length is not None else None
    except ValueError:
        parsed_length = None
    return parsed_length, normalize_mime(response.headers.get("Content-Type"))


def stream_sha256(body: BinaryIO, expected_size: int) -> tuple[int, str, bool]:
    digest = hashlib.sha256()
    total = 0
    exceeded = False
    while True:
        chunk = body.read(CHUNK_SIZE)
        if not chunk:
            break
        total += len(chunk)
        if total > expected_size:
            exceeded = True
            # The size has already failed; stop reading unbounded/malicious responses.
            break
        digest.update(chunk)
    return total, digest.hexdigest(), exceeded


def verify_object(row: dict[str, Any], adapter: HTTPReadAdapter) -> tuple[bool, list[dict[str, Any]]]:
    entity = row["entity_key"]
    head_size: int | None = None
    head_mime = ""
    if adapter.use_head:
        try:
            with adapter.request("HEAD", row["object_key"]) as response:
                head_size, head_mime = response_metadata(response)
        except urllib.error.HTTPError as exc:
            if exc.code != 405:
                problem = http_failure_violation(exc, entity)
                exc.close()
                return False, [problem]
            exc.close()
        except (OSError, TimeoutError, urllib.error.URLError) as exc:
            return False, [http_failure_violation(exc, entity)]

    try:
        with adapter.request("GET", row["object_key"]) as response:
            get_size, get_mime = response_metadata(response)
            actual_size, actual_sha, exceeded = stream_sha256(response, row["size"])
    except (OSError, TimeoutError, urllib.error.URLError) as exc:
        problem = http_failure_violation(exc, entity)
        if isinstance(exc, urllib.error.HTTPError):
            exc.close()
        return False, [problem]

    issues: list[dict[str, Any]] = []
    if exceeded or actual_size != row["size"] or (get_size is not None and get_size != row["size"]) or (head_size is not None and head_size != row["size"]):
        issues.append(violation("object_manifest.size_mismatch", entity, "stored size differs from manifest", blocked=False))
    expected_mime = normalize_mime(row["mime_type"])
    observed_mime = get_mime or head_mime
    if not observed_mime:
        issues.append(violation("object_manifest.mime_unavailable", entity, "stored MIME is unavailable", blocked=True))
    elif observed_mime != expected_mime or (head_mime and head_mime != expected_mime):
        issues.append(violation("object_manifest.mime_mismatch", entity, "stored MIME differs from manifest", blocked=False))
    # A size mismatch that stopped the stream cannot produce a whole-object digest.
    if exceeded:
        issues.append(violation("object_manifest.sha256_unverified", entity, "whole-object SHA-256 could not be completed", blocked=True))
    elif actual_sha != row["sha256"]:
        issues.append(violation("object_manifest.sha256_mismatch", entity, "stored SHA-256 differs from manifest", blocked=False))
    return not exceeded, issues


def validate_row(row: Any, line_no: int) -> list[dict[str, Any]]:
    """Compatibility helper: a row without explicit adapter config stays blocked."""
    problems = validate_contract(row, line_no)
    if problems:
        return problems
    return [violation(
        "object_manifest.adapter_not_configured", row["entity_key"],
        "storage read adapter is not configured", blocked=True,
    )]


def finalize_result(
    manifest_sha: str,
    checked: int,
    violations: list[dict[str, Any]],
    *,
    exception_evidence_sha256: str = "0" * 64,
    mapping_sha256: str = "0" * 64,
    mapping_row_hash: str = "0" * 64,
    exceptions: list[dict[str, Any]] | None = None,
) -> dict[str, Any]:
    ordered = sorted(violations, key=lambda item: (item["entity_key"], item["violation_code"], item["detail"]))
    exceptions = sorted(
        exceptions or [], key=lambda item: (item["entity_key"], item["observed_http_status"])
    )
    if any(item["hard_blocked"] for item in ordered):
        status = "BLOCKED"
    elif ordered:
        status = "FAIL"
    else:
        status = "PASS"
    evidence = {
        "schema_version": SCHEMA_VERSION,
        "status": status,
        "violation_count": len(ordered),
        "checked_count": checked,
        "manifest_sha256": manifest_sha,
        "exception_count": len(exceptions),
        "exception_evidence_sha256": exception_evidence_sha256,
        "mapping_sha256": mapping_sha256,
        "mapping_row_hash": mapping_row_hash,
        "exceptions": exceptions,
        "violations": ordered,
    }
    evidence["evidence_hash"] = hashlib.sha256(canonical_json(evidence).encode("utf-8")).hexdigest()
    return evidence


def expected_unavailable_record(
    row: dict[str, Any],
    adapter: HTTPReadAdapter,
    exception: dict[str, Any],
) -> tuple[bool, dict[str, Any] | None, list[dict[str, Any]]]:
    entity = row["entity_key"]
    try:
        with adapter.request("GET", row["object_key"]):
            pass
    except urllib.error.HTTPError as exc:
        status = exc.code
        exc.close()
        if status == historical_exception.EXPECTED_HTTP_STATUS:
            return True, {
                "entity_key": entity,
                "task_id": exception["task_id"],
                "missing_task_asset_id": exception["missing_task_asset_id"],
                "expected_http_status": historical_exception.EXPECTED_HTTP_STATUS,
                "observed_http_status": status,
                "mapping_row_hash": exception["mapping_row_hash"],
                "object_row_sha256": exception["object_row_sha256"],
                "working_reference_count": exception["working_reference_count"],
                "finalized_reference_count": exception[
                    "finalized_reference_count"
                ],
            }, []
        return False, None, [
            http_failure_violation(exc, entity)
        ]
    except (OSError, TimeoutError, urllib.error.URLError) as exc:
        return False, None, [http_failure_violation(exc, entity)]
    return False, None, [
        violation(
            "object_manifest.expected_unavailable_present",
            entity,
            "expected HTTP 410 but object read succeeded",
            blocked=False,
        )
    ]


def verify(
    path: pathlib.Path,
    config: VerifierConfig | None = None,
    exception_path: pathlib.Path | None = None,
) -> dict[str, Any]:
    config = config or VerifierConfig()
    if not path.is_file():
        return finalize_result("0" * 64, 0, [violation("object_manifest.missing", "*", "manifest file is missing", blocked=True)])
    manifest_sha = sha256_file(path)
    exception_evidence_sha = "0" * 64
    exception_mapping_sha = "0" * 64
    exception_mapping_row_hash = "0" * 64
    exception: dict[str, Any] | None = None
    if exception_path is not None:
        try:
            attestation, exception, exception_evidence_sha = (
                historical_exception.load_attestation(
                    exception_path, manifest_path=path
                )
            )
            exception_mapping_sha = attestation["mapping_sha256"]
            exception_mapping_row_hash = attestation["mapping_row_hash"]
        except (OSError, historical_exception.ExceptionContractError) as exc:
            return finalize_result(
                manifest_sha,
                0,
                [
                    violation(
                        "object_manifest.exception_invalid",
                        historical_exception.ENTITY_KEY,
                        str(exc),
                        blocked=True,
                    )
                ],
            )
    rows: list[dict[str, Any]] = []
    problems: list[dict[str, Any]] = []
    seen_entities: set[str] = set()
    nonempty = 0
    try:
        with path.open("r", encoding="utf-8") as handle:
            for line_no, line in enumerate(handle, 1):
                if not line.strip():
                    continue
                nonempty += 1
                try:
                    row = json.loads(line)
                except json.JSONDecodeError:
                    problems.append(violation("object_manifest.invalid", str(line_no), "invalid JSON", blocked=True))
                    continue
                row_problems = validate_contract(row, line_no)
                if not row_problems and row["entity_key"] in seen_entities:
                    row_problems.append(violation(
                        "object_manifest.duplicate", row["entity_key"],
                        "entity appears more than once", blocked=True,
                    ))
                elif not row_problems:
                    seen_entities.add(row["entity_key"])
                problems.extend(row_problems)
                if not row_problems:
                    rows.append(row)
    except UnicodeDecodeError:
        return finalize_result(manifest_sha, 0, [violation("object_manifest.invalid", "*", "manifest is not UTF-8", blocked=True)])
    if nonempty == 0:
        return finalize_result(manifest_sha, 0, [violation("object_manifest.empty", "*", "manifest contains no objects", blocked=True)])

    checked = 0
    accepted_exceptions: list[dict[str, Any]] = []
    exception_seen = False
    verified_cache: dict[tuple[Any, ...], list[tuple[str, str, bool]]] = {}
    for row in sorted(rows, key=lambda item: (item["entity_key"], item["object_key"])):
        adapter = config.adapter_for(row["storage_adapter"])
        if adapter is None:
            problems.append(violation(
                "object_manifest.adapter_not_configured", row["entity_key"],
                f'adapter class {row["storage_adapter"].strip().lower()} is not configured', blocked=True,
            ))
            continue
        if exception is not None and row["entity_key"] == exception["entity_key"]:
            exception_seen = True
            complete, accepted, row_problems = expected_unavailable_record(
                row, adapter, exception
            )
            if complete and accepted is not None:
                checked += 1
                accepted_exceptions.append(accepted)
            problems.extend(row_problems)
            continue
        cache_key = (
            id(adapter), row["object_key"], row["size"],
            normalize_mime(row["mime_type"]), row["sha256"],
        )
        cached = verified_cache.get(cache_key)
        if cached is None:
            complete, row_problems = verify_object(row, adapter)
            if complete:
                verified_cache[cache_key] = [
                    (item["violation_code"], item["detail"], item["hard_blocked"])
                    for item in row_problems
                ]
        else:
            complete = True
            row_problems = [
                violation(code, row["entity_key"], detail, blocked=blocked)
                for code, detail, blocked in cached
            ]
        if complete:
            checked += 1
        problems.extend(row_problems)
    if exception is not None and not exception_seen:
        problems.append(
            violation(
                "object_manifest.exception_entity_missing",
                exception["entity_key"],
                "attested exception entity is absent from the manifest",
                blocked=True,
            )
        )
    return finalize_result(
        manifest_sha,
        checked,
        problems,
        exception_evidence_sha256=exception_evidence_sha,
        mapping_sha256=exception_mapping_sha,
        mapping_row_hash=exception_mapping_row_hash,
        exceptions=accepted_exceptions,
    )


def adapter_from_args(kind: str, args: argparse.Namespace) -> HTTPReadAdapter | None:
    prefix = kind.upper()
    base_url = getattr(args, f"{kind}_base_url") or os.environ.get(f"AB_{prefix}_READ_BASE_URL", "")
    headers_file = getattr(args, f"{kind}_headers_file") or os.environ.get(f"AB_{prefix}_READ_HEADERS_FILE", "")
    token = os.environ.get(f"AB_{prefix}_READ_BEARER_TOKEN", "")
    if not base_url:
        if headers_file or token:
            raise ValueError(f"{kind} credentials were provided without a base URL")
        return None
    return HTTPReadAdapter(
        base_url=validate_base_url(base_url),
        headers=load_headers(headers_file or None, token or None),
        timeout_seconds=args.timeout_seconds,
        use_head=kind == "oss",
    )


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("manifest_jsonl")
    parser.add_argument("out_json")
    parser.add_argument("--upload-base-url", help="upload-service /files URL prefix; may use AB_UPLOAD_READ_BASE_URL")
    parser.add_argument("--upload-headers-file", help="JSON headers file; may use AB_UPLOAD_READ_HEADERS_FILE")
    parser.add_argument("--oss-base-url", help="read-only OSS/gateway URL prefix; may use AB_OSS_READ_BASE_URL")
    parser.add_argument("--oss-headers-file", help="JSON headers file; may use AB_OSS_READ_HEADERS_FILE")
    parser.add_argument("--timeout-seconds", type=float, default=30.0)
    parser.add_argument(
        "--historical-unavailable-exception",
        help="hash-bound task 2199 / asset 12323 exception attestation",
    )
    args = parser.parse_args(argv)
    if args.timeout_seconds <= 0 or args.timeout_seconds > 3600:
        parser.error("--timeout-seconds must be in (0, 3600]")
    return args


def main(argv: list[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    output = pathlib.Path(args.out_json)
    manifest = pathlib.Path(args.manifest_jsonl)
    exception_path = (
        pathlib.Path(args.historical_unavailable_exception)
        if args.historical_unavailable_exception
        else None
    )
    try:
        input_paths = {manifest.resolve()}
        if exception_path is not None:
            input_paths.add(exception_path.resolve())
        if output.resolve() in input_paths:
            print(
                "manifest, exception, and output paths must differ",
                file=sys.stderr,
            )
            return 2
    except OSError:
        pass
    try:
        config = VerifierConfig(upload=adapter_from_args("upload", args), oss=adapter_from_args("oss", args))
        result = verify(
            manifest,
            config,
            exception_path,
        )
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        # Configuration failures are intentionally secret-free and still produce a gate artifact.
        manifest_sha = sha256_file(manifest) if manifest.is_file() else "0" * 64
        result = finalize_result(manifest_sha, 0, [violation("object_manifest.configuration", "*", str(exc), blocked=True)])
    output.write_text(canonical_json(result) + "\n", encoding="utf-8")
    return 0 if result["status"] == "PASS" and result["violation_count"] == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
