#!/usr/bin/env python3
"""Fail-closed proof that every available reviewed object was re-fingerprinted.

The sole task-2199/asset-12323 historical-unavailable row is counted
transparently as an exception only when a valid attestation binds its mapping
row, object row, zero-current-reference SQL evidence, and exact HTTP 410 API
evidence.  Every other row remains subject to forced byte rehydration.
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
HYDRATION_FIELDS = {
    "schema_version",
    "status",
    "input_manifest_sha256",
    "hydrated_manifest_sha256",
    "checkpoint_sha256",
    "row_count",
    "already_complete_count",
    "missing_sha256_count",
    "configured_target_row_count",
    "unique_target_count",
    "resumed_target_count",
    "resumed_failure_target_count",
    "read_only_get_count",
    "hydrated_row_count",
    "deduplicated_get_count",
    "failure_count",
    "failures",
    "evidence_hash",
}


class VerificationError(ValueError):
    """A secret-free force-reverification failure."""

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


def checked_result(
    *,
    status: str,
    checked_count: int,
    manifest_sha: str,
    hydrated_sha: str,
    hydration_evidence_sha: str,
    violations: list[dict[str, str]],
    exception_count: int = 0,
    exception_evidence_sha256: str = ZERO_SHA256,
    mapping_sha256: str = ZERO_SHA256,
    mapping_row_hash: str = ZERO_SHA256,
    exceptions: list[dict[str, Any]] | None = None,
) -> dict[str, Any]:
    result: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "status": status,
        "violation_count": len(violations),
        "violations": violations,
        "checked_count": checked_count,
        "manifest_sha256": manifest_sha,
        "hydrated_manifest_sha256": hydrated_sha,
        "hydration_evidence_sha256": hydration_evidence_sha,
        "exception_count": exception_count,
        "exception_evidence_sha256": exception_evidence_sha256,
        "mapping_sha256": mapping_sha256,
        "mapping_row_hash": mapping_row_hash,
        "exceptions": exceptions or [],
    }
    result["evidence_hash"] = hashlib.sha256(
        canonical_json(result).encode("utf-8")
    ).hexdigest()
    return result


def read_jsonl(
    path: pathlib.Path,
    *,
    label: str,
    allow_empty_sha: bool,
    exception_entity: str | None = None,
) -> tuple[list[dict[str, Any]], str]:
    if path.is_symlink() or not path.is_file():
        raise VerificationError(
            f"force_reverify.{label}_missing", f"{label} manifest is missing"
        )
    digest = sha256_file(path)
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except UnicodeDecodeError as exc:
        raise VerificationError(
            f"force_reverify.{label}_invalid",
            f"{label} manifest is not UTF-8",
        ) from exc
    if not lines:
        raise VerificationError(
            f"force_reverify.{label}_empty", f"{label} manifest is empty"
        )
    rows: list[dict[str, Any]] = []
    seen: set[str] = set()
    for line_no, line in enumerate(lines, 1):
        if not line:
            raise VerificationError(
                f"force_reverify.{label}_invalid",
                f"{label} manifest row {line_no} is blank",
            )
        try:
            row = json.loads(line)
        except json.JSONDecodeError as exc:
            raise VerificationError(
                f"force_reverify.{label}_invalid",
                f"{label} manifest row {line_no} is invalid JSON",
            ) from exc
        is_exception = (
            isinstance(row, dict) and row.get("entity_key") == exception_entity
        )
        if is_exception:
            try:
                historical_exception.validate_exception_object_row(row)
                problems = []
            except historical_exception.ExceptionContractError:
                problems = [
                    {
                        "violation_code": "object_manifest.exception_row_invalid"
                    }
                ]
        elif allow_empty_sha and isinstance(row, dict) and row.get("sha256") == "":
            candidate = dict(row)
            candidate["sha256"] = "1" * 64
            problems = verifier.validate_contract(candidate, line_no)
        else:
            problems = verifier.validate_contract(row, line_no)
        if problems:
            code = problems[0]["violation_code"]
            if code == "object_manifest.placeholder":
                raise VerificationError(
                    "force_reverify.placeholder",
                    f"{label} manifest row {line_no} is a placeholder",
                )
            raise VerificationError(
                f"force_reverify.{label}_invalid",
                f"{label} manifest row {line_no} violates the object contract",
            )
        if allow_empty_sha and not is_exception and row["sha256"] != "":
            raise VerificationError(
                "force_reverify.sha_not_cleared",
                f"{label} manifest row {line_no} retains a fingerprint",
            )
        if is_exception and row["sha256"] != "":
            raise VerificationError(
                "force_reverify.exception_fingerprint_present",
                f"{label} manifest exception row {line_no} invented a fingerprint",
            )
        if (
            not allow_empty_sha
            and not is_exception
            and row["sha256"] in {"", ZERO_SHA256}
        ):
            raise VerificationError(
                "force_reverify.fingerprint_missing",
                f"{label} manifest row {line_no} has no usable fingerprint",
            )
        entity = row["entity_key"]
        if entity in seen:
            raise VerificationError(
                "force_reverify.duplicate_entity",
                f"{label} manifest row {line_no} duplicates an entity",
            )
        seen.add(entity)
        rows.append(row)
    return rows, digest


def target_kind(storage_adapter: str) -> str:
    normalized = storage_adapter.strip().lower()
    if normalized in verifier.UPLOAD_ADAPTERS:
        return "upload"
    if normalized in verifier.OSS_ADAPTERS:
        return "oss"
    raise VerificationError(
        "force_reverify.adapter_unsupported",
        "reviewed manifest contains an unsupported storage adapter",
    )


def load_hydration_evidence(path: pathlib.Path) -> tuple[dict[str, Any], str]:
    if path.is_symlink() or not path.is_file():
        raise VerificationError(
            "force_reverify.hydration_evidence_missing",
            "hydration evidence is missing",
        )
    digest = sha256_file(path)
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise VerificationError(
            "force_reverify.hydration_evidence_invalid",
            "hydration evidence is not valid UTF-8 JSON",
        ) from exc
    if not isinstance(payload, dict) or set(payload) != HYDRATION_FIELDS:
        raise VerificationError(
            "force_reverify.hydration_evidence_invalid",
            "hydration evidence field contract differs",
        )
    evidence_hash = payload.get("evidence_hash")
    unsigned = {key: value for key, value in payload.items() if key != "evidence_hash"}
    expected_hash = hashlib.sha256(
        canonical_json(unsigned).encode("utf-8")
    ).hexdigest()
    if evidence_hash != expected_hash:
        raise VerificationError(
            "force_reverify.hydration_evidence_tampered",
            "hydration evidence self-hash differs",
        )
    return payload, digest


def require_count(payload: dict[str, Any], field: str, expected: int) -> None:
    value = payload.get(field)
    if (
        not isinstance(value, int)
        or isinstance(value, bool)
        or value != expected
    ):
        raise VerificationError(
            "force_reverify.hydration_count_mismatch",
            f"hydration evidence {field} does not cover the reviewed manifest",
        )


def read_count(payload: dict[str, Any], field: str) -> int:
    value = payload.get(field)
    if not isinstance(value, int) or isinstance(value, bool) or value < 0:
        raise VerificationError(
            "force_reverify.hydration_count_mismatch",
            f"hydration evidence {field} is not a non-negative count",
        )
    return value


def verify(
    reviewed_path: pathlib.Path,
    force_path: pathlib.Path,
    hydrated_path: pathlib.Path,
    hydration_evidence_path: pathlib.Path,
    exception_path: pathlib.Path | None = None,
) -> dict[str, Any]:
    manifest_sha = ZERO_SHA256
    hydrated_sha = ZERO_SHA256
    hydration_evidence_sha = ZERO_SHA256
    exception_evidence_sha = ZERO_SHA256
    mapping_sha = ZERO_SHA256
    mapping_row_hash = ZERO_SHA256
    exception: dict[str, Any] | None = None
    try:
        if exception_path is not None:
            attestation, exception, exception_evidence_sha = (
                historical_exception.load_attestation(
                    exception_path, manifest_path=reviewed_path
                )
            )
            mapping_sha = attestation["mapping_sha256"]
            mapping_row_hash = attestation["mapping_row_hash"]
        reviewed, manifest_sha = read_jsonl(
            reviewed_path,
            label="reviewed",
            allow_empty_sha=False,
            exception_entity=exception["entity_key"] if exception else None,
        )
        forced, force_sha = read_jsonl(
            force_path,
            label="force",
            allow_empty_sha=True,
            exception_entity=exception["entity_key"] if exception else None,
        )
        hydrated, hydrated_sha = read_jsonl(
            hydrated_path,
            label="hydrated",
            allow_empty_sha=False,
            exception_entity=exception["entity_key"] if exception else None,
        )
        hydration, hydration_evidence_sha = load_hydration_evidence(
            hydration_evidence_path
        )
        if not (len(reviewed) == len(forced) == len(hydrated)):
            raise VerificationError(
                "force_reverify.row_count_mismatch",
                "reviewed, force, and hydrated row counts differ",
            )

        for index, (source, force, observed) in enumerate(
            zip(reviewed, forced, hydrated), 1
        ):
            expected_force = dict(source)
            if exception is None or source["entity_key"] != exception["entity_key"]:
                expected_force["sha256"] = ""
            if force != expected_force:
                raise VerificationError(
                    "force_reverify.force_manifest_tampered",
                    f"force manifest row {index} differs outside the cleared SHA-256",
                )
            if observed != source:
                raise VerificationError(
                    "force_reverify.hydrated_manifest_mismatch",
                    f"hydrated manifest row {index} differs from the reviewed fingerprint",
                )

        unique_targets = {
            (target_kind(row["storage_adapter"]), row["object_key"])
            for row in reviewed
            if exception is None or row["entity_key"] != exception["entity_key"]
        }
        row_count = len(reviewed)
        exception_count = 1 if exception is not None else 0
        available_count = row_count - exception_count
        target_count = len(unique_targets)
        if hydration.get("schema_version") != 1:
            raise VerificationError(
                "force_reverify.hydration_evidence_invalid",
                "hydration evidence schema_version differs",
            )
        if hydration.get("status") != "PASS":
            raise VerificationError(
                "force_reverify.hydration_not_pass",
                "hydration status is not PASS",
            )
        if hydration.get("input_manifest_sha256") != force_sha:
            raise VerificationError(
                "force_reverify.hydration_input_mismatch",
                "hydration evidence is not bound to the force manifest",
            )
        if hydration.get("hydrated_manifest_sha256") != hydrated_sha:
            raise VerificationError(
                "force_reverify.hydration_output_mismatch",
                "hydration evidence is not bound to the hydrated manifest",
            )
        checkpoint_sha = hydration.get("checkpoint_sha256")
        if (
            not isinstance(checkpoint_sha, str)
            or not verifier.SHA256.fullmatch(checkpoint_sha)
            or checkpoint_sha == ZERO_SHA256
        ):
            raise VerificationError(
                "force_reverify.hydration_evidence_invalid",
                "hydration checkpoint hash is invalid",
            )
        expected_counts = {
            "row_count": row_count,
            "already_complete_count": exception_count,
            "missing_sha256_count": available_count,
            "configured_target_row_count": available_count,
            "unique_target_count": target_count,
            "resumed_failure_target_count": 0,
            "hydrated_row_count": available_count,
            "deduplicated_get_count": available_count - target_count,
            "failure_count": 0,
        }
        for field, expected in expected_counts.items():
            require_count(hydration, field, expected)
        resumed_count = read_count(hydration, "resumed_target_count")
        get_count = read_count(hydration, "read_only_get_count")
        if resumed_count + get_count != target_count:
            raise VerificationError(
                "force_reverify.hydration_count_mismatch",
                "hydration GET and checkpoint-resume counts do not cover every unique object",
            )
        if hydration.get("failures") != []:
            raise VerificationError(
                "force_reverify.hydration_failures",
                "hydration evidence contains failures",
            )
        accepted_exceptions = []
        if exception is not None:
            accepted_exceptions.append(
                {
                    "entity_key": exception["entity_key"],
                    "task_id": exception["task_id"],
                    "missing_task_asset_id": exception["missing_task_asset_id"],
                    "expected_http_status": historical_exception.EXPECTED_HTTP_STATUS,
                    "observed_http_status": historical_exception.EXPECTED_HTTP_STATUS,
                    "mapping_row_hash": exception["mapping_row_hash"],
                    "object_row_sha256": exception["object_row_sha256"],
                    "working_reference_count": 0,
                    "finalized_reference_count": 0,
                }
            )
        return checked_result(
            status="PASS",
            checked_count=row_count,
            manifest_sha=manifest_sha,
            hydrated_sha=hydrated_sha,
            hydration_evidence_sha=hydration_evidence_sha,
            violations=[],
            exception_count=exception_count,
            exception_evidence_sha256=exception_evidence_sha,
            mapping_sha256=mapping_sha,
            mapping_row_hash=mapping_row_hash,
            exceptions=accepted_exceptions,
        )
    except (
        VerificationError,
        OSError,
        historical_exception.ExceptionContractError,
    ) as exc:
        if isinstance(exc, VerificationError):
            code, detail = exc.code, exc.detail
        elif isinstance(exc, historical_exception.ExceptionContractError):
            code, detail = "force_reverify.exception_invalid", str(exc)
        else:
            code, detail = "force_reverify.io_error", "verification I/O failed"
        return checked_result(
            status="BLOCKED",
            checked_count=0,
            manifest_sha=manifest_sha,
            hydrated_sha=hydrated_sha,
            hydration_evidence_sha=hydration_evidence_sha,
            violations=[
                {
                    "violation_code": code,
                    "entity_key": "*",
                    "detail": detail,
                }
            ],
            exception_evidence_sha256=exception_evidence_sha,
            mapping_sha256=mapping_sha,
            mapping_row_hash=mapping_row_hash,
        )


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("reviewed_manifest_jsonl", type=pathlib.Path)
    parser.add_argument("force_reverify_manifest_jsonl", type=pathlib.Path)
    parser.add_argument("hydrated_manifest_jsonl", type=pathlib.Path)
    parser.add_argument("hydration_evidence_json", type=pathlib.Path)
    parser.add_argument("output_evidence_json", type=pathlib.Path)
    parser.add_argument(
        "--historical-unavailable-exception",
        type=pathlib.Path,
        help="hash-bound task 2199 / asset 12323 exception attestation",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    paths = tuple(
        path
        for path in (
            args.reviewed_manifest_jsonl,
            args.force_reverify_manifest_jsonl,
            args.hydrated_manifest_jsonl,
            args.hydration_evidence_json,
            args.output_evidence_json,
            args.historical_unavailable_exception,
        )
        if path is not None
    )
    if len({path.resolve() for path in paths}) != len(paths):
        result = checked_result(
            status="BLOCKED",
            checked_count=0,
            manifest_sha=ZERO_SHA256,
            hydrated_sha=ZERO_SHA256,
            hydration_evidence_sha=ZERO_SHA256,
            violations=[
                {
                    "violation_code": "force_reverify.path_collision",
                    "entity_key": "*",
                    "detail": "all input and output paths must differ",
                }
            ],
        )
    else:
        result = verify(
            args.reviewed_manifest_jsonl,
            args.force_reverify_manifest_jsonl,
            args.hydrated_manifest_jsonl,
            args.hydration_evidence_json,
            args.historical_unavailable_exception,
        )
    atomic_write_bytes(
        args.output_evidence_json,
        (canonical_json(result) + "\n").encode("utf-8"),
    )
    return 0 if result["status"] == "PASS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
