#!/usr/bin/env python3
"""Create a force-reverification manifest from a fully reviewed object manifest.

The reviewed input must already contain a complete, non-placeholder fingerprint
for every unique entity.  The output preserves every field and row position
except that ``sha256`` is cleared, forcing ``hydrate_object_manifest.py`` to
stream every unique object instead of trusting the reviewed digest.  The sole
task-2199/asset-12323 historical-unavailable row may retain its frozen digest
only when a valid hash-bound exception attestation is supplied.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import tempfile
from typing import Any

try:
    from scripts.ab import object_manifest_verifier as verifier
    from scripts.ab import historical_unavailable_exception as historical_exception
except ModuleNotFoundError:  # Direct execution from scripts/ab.
    import object_manifest_verifier as verifier
    import historical_unavailable_exception as historical_exception


SCHEMA_VERSION = 1
ZERO_SHA256 = "0" * 64


class ManifestError(ValueError):
    """A secret-free manifest validation failure."""

    def __init__(self, code: str, detail: str) -> None:
        super().__init__(detail)
        self.code = code
        self.detail = detail


def canonical_json(value: Any) -> str:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    )


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def atomic_write_bytes(path: pathlib.Path, payload: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary: pathlib.Path | None = None
    try:
        with tempfile.NamedTemporaryFile(
            mode="wb",
            prefix=path.name + ".",
            suffix=".tmp",
            dir=path.parent,
            delete=False,
        ) as handle:
            temporary = pathlib.Path(handle.name)
            handle.write(payload)
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


def evidence_document(
    *,
    status: str,
    source_sha: str,
    output_sha: str,
    row_count: int,
    violations: list[dict[str, str]],
    exception_count: int = 0,
    exception_evidence_sha256: str = ZERO_SHA256,
    mapping_sha256: str = ZERO_SHA256,
    mapping_row_hash: str = ZERO_SHA256,
) -> dict[str, Any]:
    result: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "status": status,
        "violation_count": len(violations),
        "violations": violations,
        "row_count": row_count,
        "source_manifest_sha256": source_sha,
        "force_reverify_manifest_sha256": output_sha,
        "exception_count": exception_count,
        "exception_evidence_sha256": exception_evidence_sha256,
        "mapping_sha256": mapping_sha256,
        "mapping_row_hash": mapping_row_hash,
    }
    result["evidence_hash"] = hashlib.sha256(
        canonical_json(result).encode("utf-8")
    ).hexdigest()
    return result


def load_reviewed_rows(
    path: pathlib.Path,
    exception: dict[str, Any] | None = None,
) -> tuple[list[dict[str, Any]], str]:
    if path.is_symlink() or not path.is_file():
        raise ManifestError("force_reverify.source_missing", "source manifest is missing")
    source_sha = sha256_file(path)
    rows: list[dict[str, Any]] = []
    seen_entities: set[str] = set()
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except UnicodeDecodeError as exc:
        raise ManifestError(
            "force_reverify.source_invalid", "source manifest is not UTF-8"
        ) from exc
    if not lines:
        raise ManifestError("force_reverify.source_empty", "source manifest is empty")
    for line_no, line in enumerate(lines, 1):
        if not line:
            raise ManifestError(
                "force_reverify.source_invalid",
                f"source manifest row {line_no} is blank",
            )
        try:
            row = json.loads(line)
        except json.JSONDecodeError as exc:
            raise ManifestError(
                "force_reverify.source_invalid",
                f"source manifest row {line_no} is invalid JSON",
            ) from exc
        is_attested_exception = (
            exception is not None
            and isinstance(row, dict)
            and row.get("entity_key") == exception["entity_key"]
        )
        if is_attested_exception:
            try:
                historical_exception.validate_exception_object_row(row)
                problems = []
            except historical_exception.ExceptionContractError:
                problems = [
                    {
                        "violation_code": "object_manifest.exception_row_invalid"
                    }
                ]
        else:
            problems = verifier.validate_contract(row, line_no)
        if problems:
            code = problems[0]["violation_code"]
            if code == "object_manifest.placeholder":
                raise ManifestError(
                    "force_reverify.placeholder",
                    f"source manifest row {line_no} is a placeholder",
                )
            raise ManifestError(
                "force_reverify.source_invalid",
                f"source manifest row {line_no} violates the object contract",
            )
        if not is_attested_exception and row["sha256"] in {"", ZERO_SHA256}:
            raise ManifestError(
                "force_reverify.fingerprint_missing",
                f"source manifest row {line_no} has no reviewed fingerprint",
            )
        adapter = row["storage_adapter"].strip().lower()
        if adapter not in verifier.UPLOAD_ADAPTERS | verifier.OSS_ADAPTERS:
            raise ManifestError(
                "force_reverify.adapter_unsupported",
                f"source manifest row {line_no} uses an unsupported adapter",
            )
        entity = row["entity_key"]
        if entity in seen_entities:
            raise ManifestError(
                "force_reverify.duplicate_entity",
                f"source manifest row {line_no} duplicates an entity",
            )
        seen_entities.add(entity)
        rows.append(row)
    return rows, source_sha


def prepare(
    source_path: pathlib.Path,
    output_path: pathlib.Path,
    exception_path: pathlib.Path | None = None,
) -> dict[str, Any]:
    resolved_inputs = {source_path.resolve()}
    if exception_path is not None:
        resolved_inputs.add(exception_path.resolve())
    if output_path.resolve() in resolved_inputs:
        raise ManifestError(
            "force_reverify.path_collision",
            "source, exception, and force-reverify manifest paths must differ",
        )
    exception: dict[str, Any] | None = None
    exception_sha = ZERO_SHA256
    mapping_sha = ZERO_SHA256
    mapping_row_hash = ZERO_SHA256
    if exception_path is not None:
        attestation, exception, exception_sha = historical_exception.load_attestation(
            exception_path, manifest_path=source_path
        )
        mapping_sha = attestation["mapping_sha256"]
        mapping_row_hash = attestation["mapping_row_hash"]
    rows, source_sha = load_reviewed_rows(source_path, exception)
    forced_rows: list[dict[str, Any]] = []
    for row in rows:
        forced = dict(row)
        if exception is None or row["entity_key"] != exception["entity_key"]:
            forced["sha256"] = ""
        forced_rows.append(forced)
    payload = "".join(canonical_json(row) + "\n" for row in forced_rows).encode(
        "utf-8"
    )
    atomic_write_bytes(output_path, payload)
    output_sha = sha256_file(output_path)
    return evidence_document(
        status="PASS",
        source_sha=source_sha,
        output_sha=output_sha,
        row_count=len(rows),
        violations=[],
        exception_count=1 if exception is not None else 0,
        exception_evidence_sha256=exception_sha,
        mapping_sha256=mapping_sha,
        mapping_row_hash=mapping_row_hash,
    )


def blocked_result(
    source_path: pathlib.Path,
    error: ManifestError | historical_exception.ExceptionContractError | OSError,
) -> dict[str, Any]:
    source_sha = (
        sha256_file(source_path)
        if source_path.is_file() and not source_path.is_symlink()
        else ZERO_SHA256
    )
    if isinstance(error, ManifestError):
        code, detail = error.code, error.detail
    elif isinstance(error, historical_exception.ExceptionContractError):
        code, detail = "force_reverify.exception_invalid", str(error)
    else:
        code, detail = "force_reverify.io_error", "manifest I/O failed"
    return evidence_document(
        status="BLOCKED",
        source_sha=source_sha,
        output_sha=ZERO_SHA256,
        row_count=0,
        violations=[
            {
                "violation_code": code,
                "entity_key": "*",
                "detail": detail,
            }
        ],
    )


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("reviewed_manifest_jsonl", type=pathlib.Path)
    parser.add_argument("force_reverify_manifest_jsonl", type=pathlib.Path)
    parser.add_argument("evidence_json", type=pathlib.Path)
    parser.add_argument(
        "--historical-unavailable-exception",
        type=pathlib.Path,
        help="hash-bound task 2199 / asset 12323 exception attestation",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    try:
        resolved = [
            item.resolve()
            for item in (
                args.reviewed_manifest_jsonl,
                args.force_reverify_manifest_jsonl,
                args.evidence_json,
                args.historical_unavailable_exception,
            )
            if item is not None
        ]
        if len(set(resolved)) != len(resolved):
            raise ManifestError(
                "force_reverify.path_collision",
                "source, output, and evidence paths must differ",
            )
        result = prepare(
            args.reviewed_manifest_jsonl,
            args.force_reverify_manifest_jsonl,
            args.historical_unavailable_exception,
        )
    except (
        ManifestError,
        OSError,
        historical_exception.ExceptionContractError,
    ) as exc:
        result = blocked_result(args.reviewed_manifest_jsonl, exc)
    atomic_write_bytes(
        args.evidence_json, (canonical_json(result) + "\n").encode("utf-8")
    )
    return 0 if result["status"] == "PASS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
