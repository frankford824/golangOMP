#!/usr/bin/env python3
"""Validate the independent Browser/Computer Use and Playwright evidence for G7.

This validator is intentionally offline. It reads a versioned scenario catalog,
two evidence documents, and immutable local artifacts. It never controls a
browser, calls an API, or connects to a database.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import struct
import sys
import zipfile
import zlib
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit


SCHEMA_VERSION = 1
GATE = "G7"
SOURCE_BROWSER = "browser_computer_use"
SOURCE_PLAYWRIGHT = "playwright"
EXPECTED_COMBINATIONS = {
    "external_external",
    "devplus_devplus",
    "external_devplus",
    "devplus_external",
}
EXPECTED_VIEWPORTS = {"desktop", "mobile"}
EXPECTED_EDGE_ORIGINS = {
    "external_external": "http://127.0.0.1:18101",
    "devplus_devplus": "http://127.0.0.1:18102",
    "external_devplus": "http://127.0.0.1:18103",
    "devplus_external": "http://127.0.0.1:18104",
}
EXPECTED_VIEWPORT_SPECS = {
    "desktop": {"width": 1440, "height": 900, "device_scale_factor": 1},
    "mobile": {"width": 390, "height": 844, "device_scale_factor": 1},
}
EXPECTED_NO_RESOURCE_GROUP_SCENARIOS = {
    "purchase_to_sku_planning",
}
REQUIRED_COVERAGE_TAGS = {
    "four_combinations",
    "history_drawer",
    "url_task_revision",
    "design_submit",
    "audit",
    "reopen",
    "multi_source_zip",
    "single_sku",
    "multi_sku",
    "retouch_single",
    "retouch_multiple",
    "sku_planning",
    "permissions",
    "http_403",
    "http_410",
    "negative",
    "cross_generation",
}
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
REQUIREMENT_FIELDS = (
    "requires_task_id",
    "requires_revision_ids",
    "requires_history_drawer",
    "required_http_statuses",
    "required_assertions",
)


class ValidationInputError(ValueError):
    """Raised when a top-level input cannot be interpreted safely."""


def canonical_json_bytes(value: Any) -> bytes:
    return json.dumps(
        value,
        ensure_ascii=False,
        sort_keys=True,
        separators=(",", ":"),
    ).encode("utf-8")


def canonical_sha256(value: Any) -> str:
    return hashlib.sha256(canonical_json_bytes(value)).hexdigest()


def record_sha256(record: dict[str, Any]) -> str:
    payload = dict(record)
    payload.pop("record_sha256", None)
    return canonical_sha256(payload)


def file_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def load_json_object(path: Path) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise ValidationInputError(f"cannot read JSON object {path}: {exc}") from exc
    if not isinstance(value, dict):
        raise ValidationInputError(f"{path} must contain a JSON object")
    return value


def _is_nonempty_string(value: Any) -> bool:
    return isinstance(value, str) and bool(value.strip())


def _is_positive_int(value: Any) -> bool:
    return isinstance(value, int) and not isinstance(value, bool) and value > 0


def _is_nonnegative_int(value: Any) -> bool:
    return isinstance(value, int) and not isinstance(value, bool) and value >= 0


def _parse_rfc3339(value: Any) -> datetime | None:
    if not _is_nonempty_string(value):
        return None
    text = value.strip()
    if text.endswith("Z"):
        text = text[:-1] + "+00:00"
    try:
        parsed = datetime.fromisoformat(text)
    except ValueError:
        return None
    return parsed if parsed.tzinfo is not None else None


def _valid_url(value: Any) -> bool:
    if not _is_nonempty_string(value):
        return False
    try:
        parsed = urlsplit(value)
    except ValueError:
        return False
    if parsed.scheme not in {"http", "https"}:
        return False
    if not parsed.hostname or parsed.username or parsed.password or parsed.fragment:
        return False
    if parsed.scheme == "http" and parsed.hostname not in {
        "127.0.0.1",
        "::1",
        "localhost",
    }:
        return False
    return True


def _url_origin(value: Any) -> str | None:
    if not _valid_url(value):
        return None
    try:
        parsed = urlsplit(value)
        port = parsed.port
    except ValueError:
        return None
    if parsed.scheme != "http" or parsed.hostname != "127.0.0.1" or port is None:
        return None
    return f"http://127.0.0.1:{port}"


def _case_key(
    scenario_id: str,
    combination: str,
    viewport: str,
) -> tuple[str, str, str]:
    return scenario_id, combination, viewport


def _case_key_text(key: tuple[str, str, str]) -> str:
    return "/".join(key)


def _normalized_expected_allowed_actions(
    rows: Any,
) -> tuple[tuple[str, tuple[str, ...]], ...] | None:
    if not isinstance(rows, list) or not rows:
        return None
    normalized: list[tuple[str, tuple[str, ...]]] = []
    seen: set[str] = set()
    for row in rows:
        if (
            not isinstance(row, dict)
            or set(row) != {"checkpoint", "expected"}
            or not _is_nonempty_string(row.get("checkpoint"))
            or row["checkpoint"] in seen
            or not isinstance(row.get("expected"), list)
            or any(not _is_nonempty_string(value) for value in row["expected"])
            or len(row["expected"]) != len(set(row["expected"]))
        ):
            return None
        seen.add(row["checkpoint"])
        normalized.append((row["checkpoint"], tuple(sorted(row["expected"]))))
    return tuple(sorted(normalized))


def _normalized_http_probes(
    rows: Any,
) -> tuple[tuple[str, str, str, int], ...] | None:
    if not isinstance(rows, list):
        return None
    normalized: list[tuple[str, str, str, int]] = []
    seen: set[str] = set()
    for row in rows:
        if not isinstance(row, dict) or set(row) != {
            "kind",
            "method",
            "path",
            "expected_status",
        }:
            return None
        kind = row.get("kind")
        path = row.get("path")
        status = row.get("expected_status")
        try:
            parsed = urlsplit(path) if isinstance(path, str) else None
        except ValueError:
            return None
        if (
            not _is_nonempty_string(kind)
            or kind in seen
            or row.get("method") != "GET"
            or parsed is None
            or parsed.scheme
            or parsed.netloc
            or parsed.fragment
            or not parsed.path.startswith("/")
            or parsed.path.startswith("//")
            or ".." in Path(parsed.path).parts
            or not _is_nonnegative_int(status)
            or status < 100
            or status > 599
        ):
            return None
        seen.add(kind)
        normalized.append((kind, "GET", path, status))
    return tuple(sorted(normalized))


def _validate_requirement_block(value: Any, label: str) -> dict[str, Any]:
    if not isinstance(value, dict) or set(value) != set(REQUIREMENT_FIELDS):
        raise ValidationInputError(
            f"{label} must declare the exact conditional requirement fields"
        )
    for flag in (
        "requires_task_id",
        "requires_revision_ids",
        "requires_history_drawer",
    ):
        if not isinstance(value.get(flag), bool):
            raise ValidationInputError(f"{label} has invalid {flag}")
    if value["requires_history_drawer"] and not value["requires_revision_ids"]:
        raise ValidationInputError(
            f"{label} cannot require history_drawer without revision_ids"
        )
    statuses = value.get("required_http_statuses")
    assertions = value.get("required_assertions")
    if (
        not isinstance(statuses, list)
        or any(
            not _is_nonnegative_int(status) or status < 100 or status > 599
            for status in statuses
        )
        or len(statuses) != len(set(statuses))
    ):
        raise ValidationInputError(f"{label} has invalid required_http_statuses")
    if (
        not isinstance(assertions, list)
        or not assertions
        or any(not _is_nonempty_string(name) for name in assertions)
        or len(assertions) != len(set(assertions))
    ):
        raise ValidationInputError(f"{label} has invalid required_assertions")
    return value


def _requirements_for(
    scenario: dict[str, Any],
    combination: str,
) -> dict[str, Any]:
    conditional = scenario.get("requirements_by_combination")
    if isinstance(conditional, dict) and combination in conditional:
        return conditional[combination]
    return {field: scenario[field] for field in REQUIREMENT_FIELDS}


def _expected_assertions_for(
    scenario_id: str,
    combination: str,
    base_assertions: list[str],
) -> set[str]:
    expected = set(base_assertions)
    if (
        scenario_id == "baseline_four_edge_readonly"
        and combination == "devplus_external"
    ):
        expected.add("approved_compatibility_difference_only")
    return expected


def _normalized_resource_oracle(
    value: Any,
    combination: str,
    scenario_id: str,
) -> dict[str, Any] | None:
    if not isinstance(value, dict):
        return None
    if combination == "devplus_devplus":
        expected_kind = (
            "v8_missing_resource_group"
            if scenario_id == "missing_resource_group_negative"
            else "v8_wrong_scope_rejected"
            if scenario_id == "wrong_scope_negative"
            else "v8_expected_no_resource_groups"
            if scenario_id in EXPECTED_NO_RESOURCE_GROUP_SCENARIOS
            else "v8_resource_groups"
        )
        return value if value == {"kind": expected_kind} else None
    expected_kind = {
        "external_external": "legacy_task_snapshot",
        "external_devplus": "legacy_frontend_task_snapshot",
        "devplus_external": "frontend_rollback_compatibility",
    }.get(combination)
    expected_keys = {"kind", "task_response_sha256"}
    if combination == "devplus_external":
        expected_keys.add("approved_assertion")
    if (
        expected_kind is None
        or set(value) != expected_keys
        or value.get("kind") != expected_kind
        or not SHA256_RE.fullmatch(str(value.get("task_response_sha256") or ""))
        or (
            combination == "devplus_external"
            and value.get("approved_assertion")
            != "approved_compatibility_difference_only"
        )
    ):
        return None
    return dict(value)


def _validate_catalog(catalog: dict[str, Any]) -> list[dict[str, Any]]:
    if catalog.get("schema_version") != SCHEMA_VERSION:
        raise ValidationInputError("scenario catalog schema_version must be 1")
    if catalog.get("gate") != GATE:
        raise ValidationInputError("scenario catalog gate must be G7")

    combinations = catalog.get("combinations")
    viewports = catalog.get("viewports")
    scenarios = catalog.get("scenarios")
    if (
        not isinstance(combinations, list)
        or any(not _is_nonempty_string(value) for value in combinations)
        or set(combinations) != EXPECTED_COMBINATIONS
    ):
        raise ValidationInputError("scenario catalog must declare the exact four A/B combinations")
    if len(combinations) != len(set(combinations)):
        raise ValidationInputError("scenario catalog combinations contain duplicates")
    if (
        not isinstance(viewports, list)
        or any(not _is_nonempty_string(value) for value in viewports)
        or set(viewports) != EXPECTED_VIEWPORTS
    ):
        raise ValidationInputError("scenario catalog must declare desktop and mobile")
    if len(viewports) != len(set(viewports)):
        raise ValidationInputError("scenario catalog viewports contain duplicates")
    if not isinstance(scenarios, list) or not scenarios:
        raise ValidationInputError("scenario catalog scenarios must be a non-empty array")

    seen_ids: set[str] = set()
    coverage_tags: set[str] = set()
    normalized: list[dict[str, Any]] = []
    for index, scenario in enumerate(scenarios):
        if not isinstance(scenario, dict):
            raise ValidationInputError(f"scenario[{index}] must be an object")
        scenario_id = scenario.get("id")
        if not _is_nonempty_string(scenario_id) or scenario_id in seen_ids:
            raise ValidationInputError(f"scenario[{index}] has an invalid or duplicate id")
        seen_ids.add(scenario_id)
        if scenario.get("critical") is not True:
            raise ValidationInputError(f"scenario {scenario_id} must be critical")
        required_combinations = scenario.get("required_combinations")
        required_viewports = scenario.get("required_viewports")
        if (
            not isinstance(required_combinations, list)
            or not required_combinations
            or any(not _is_nonempty_string(value) for value in required_combinations)
            or len(required_combinations) != len(set(required_combinations))
            or not set(required_combinations).issubset(EXPECTED_COMBINATIONS)
        ):
            raise ValidationInputError(
                f"scenario {scenario_id} has invalid required_combinations"
            )
        if (
            not isinstance(required_viewports, list)
            or not required_viewports
            or any(not _is_nonempty_string(value) for value in required_viewports)
            or len(required_viewports) != len(set(required_viewports))
            or not set(required_viewports).issubset(EXPECTED_VIEWPORTS)
        ):
            raise ValidationInputError(
                f"scenario {scenario_id} has invalid required_viewports"
            )
        base_requirements = _validate_requirement_block(
            {field: scenario.get(field) for field in REQUIREMENT_FIELDS},
            f"scenario {scenario_id}",
        )
        statuses = base_requirements["required_http_statuses"]
        assertions = base_requirements["required_assertions"]
        tags = scenario.get("coverage_tags")
        conditional = scenario.get("requirements_by_combination")
        if conditional is not None:
            if (
                not isinstance(conditional, dict)
                or set(conditional) != set(required_combinations)
            ):
                raise ValidationInputError(
                    f"scenario {scenario_id} requirements_by_combination must "
                    "declare every and only required combination"
                )
            for combination in required_combinations:
                requirements = _validate_requirement_block(
                    conditional[combination],
                    f"scenario {scenario_id}/{combination}",
                )
                # Conditional contracts may vary only the V8 revision/history
                # requirements.  Core page, task, asset/action assertions and
                # HTTP status checks cannot be weakened per edge.
                if requirements["requires_task_id"] != base_requirements["requires_task_id"]:
                    raise ValidationInputError(
                        f"scenario {scenario_id}/{combination} cannot change requires_task_id"
                    )
                if set(requirements["required_http_statuses"]) != set(statuses):
                    raise ValidationInputError(
                        f"scenario {scenario_id}/{combination} cannot weaken HTTP statuses"
                    )
                if set(requirements["required_assertions"]) != _expected_assertions_for(
                    scenario_id,
                    combination,
                    assertions,
                ):
                    raise ValidationInputError(
                        f"scenario {scenario_id}/{combination} cannot weaken assertions"
                    )
        if (
            not isinstance(tags, list)
            or not tags
            or any(not _is_nonempty_string(tag) for tag in tags)
            or len(tags) != len(set(tags))
        ):
            raise ValidationInputError(f"scenario {scenario_id} has invalid coverage_tags")
        coverage_tags.update(tags)
        normalized.append(scenario)

    missing_tags = sorted(REQUIRED_COVERAGE_TAGS - coverage_tags)
    if missing_tags:
        raise ValidationInputError(
            "scenario catalog is missing required coverage tags: "
            + ", ".join(missing_tags)
        )
    return normalized


def _validate_samples_manifest(
    manifest: dict[str, Any],
    scenarios: list[dict[str, Any]],
    catalog_sha256: str,
) -> tuple[
    dict[tuple[str, str, str], tuple[dict[str, Any], dict[str, Any]]],
    dict[str, dict[str, Any]],
]:
    if manifest.get("schema_version") != SCHEMA_VERSION:
        raise ValidationInputError("samples manifest schema_version must be 1")
    if manifest.get("gate") != GATE:
        raise ValidationInputError("samples manifest gate must be G7")
    if manifest.get("status") != "PASS" or manifest.get("mode") != "final":
        raise ValidationInputError("samples manifest must be final and PASS")
    input_hashes = manifest.get("input_sha256")
    if (
        not isinstance(input_hashes, dict)
        or input_hashes.get("scenario_catalog_sha256") != catalog_sha256
    ):
        raise ValidationInputError("samples manifest is not bound to this scenario catalog")
    declared_manifest_hash = manifest.get("manifest_sha256")
    hash_payload = dict(manifest)
    hash_payload.pop("manifest_sha256", None)
    if (
        not SHA256_RE.fullmatch(declared_manifest_hash or "")
        or declared_manifest_hash != canonical_sha256(hash_payload)
    ):
        raise ValidationInputError("samples manifest_sha256 is invalid")

    sealed_edges = manifest.get("sealed_edges")
    if not isinstance(sealed_edges, dict) or set(sealed_edges) != EXPECTED_COMBINATIONS:
        raise ValidationInputError("samples sealed_edges must contain exactly four combinations")
    normalized_edges: dict[str, dict[str, Any]] = {}
    for combination, expected_origin in EXPECTED_EDGE_ORIGINS.items():
        edge = sealed_edges.get(combination)
        if not isinstance(edge, dict) or set(edge) != {
            "origin",
            "edge",
            "frontend_sha256",
            "backend_sha256",
            "fixture_identity",
        }:
            raise ValidationInputError(
                f"samples sealed_edges.{combination} has an invalid shape"
            )
        if edge.get("origin") != expected_origin or edge.get("edge") != combination:
            raise ValidationInputError(
                f"samples sealed_edges.{combination} does not match the fixed origin"
            )
        if (
            not SHA256_RE.fullmatch(edge.get("frontend_sha256") or "")
            or not SHA256_RE.fullmatch(edge.get("backend_sha256") or "")
            or not _is_nonempty_string(edge.get("fixture_identity"))
        ):
            raise ValidationInputError(
                f"samples sealed_edges.{combination} has invalid fingerprints"
            )
        normalized_edges[combination] = edge

    samples = manifest.get("samples")
    if not isinstance(samples, list):
        raise ValidationInputError("samples manifest samples must be an array")
    if (
        manifest.get("scenario_count") != len(scenarios)
        or manifest.get("sample_count") != len(samples)
        or len(samples) != len(scenarios)
    ):
        raise ValidationInputError("samples manifest counts do not match the catalog")

    scenarios_by_id = {scenario["id"]: scenario for scenario in scenarios}
    samples_by_id: dict[str, dict[str, Any]] = {}
    expected_cases: dict[
        tuple[str, str, str], tuple[dict[str, Any], dict[str, Any]]
    ] = {}
    for index, sample in enumerate(samples):
        if not isinstance(sample, dict):
            raise ValidationInputError(f"samples[{index}] must be an object")
        scenario_id = sample.get("scenario_id")
        scenario = scenarios_by_id.get(scenario_id)
        if scenario is None or scenario_id in samples_by_id:
            raise ValidationInputError(
                f"samples[{index}] has an unknown or duplicate scenario_id"
            )
        if (
            sample.get("status") != "READY"
            or not SHA256_RE.fullmatch(sample.get("sample_sha256") or "")
            or sample.get("required_combinations")
            != scenario["required_combinations"]
            or sample.get("required_viewports") != scenario["required_viewports"]
        ):
            raise ValidationInputError(
                f"samples[{index}] does not match scenario {scenario_id}"
            )
        coverage = sample.get("coverage_matrix")
        if not isinstance(coverage, list):
            raise ValidationInputError(
                f"samples[{index}].coverage_matrix must be an array"
            )
        expected_sample_keys = {
            _case_key(scenario_id, combination, viewport)
            for combination in scenario["required_combinations"]
            for viewport in scenario["required_viewports"]
        }
        actual_sample_keys: set[tuple[str, str, str]] = set()
        for coverage_index, row in enumerate(coverage):
            if not isinstance(row, dict):
                raise ValidationInputError(
                    f"samples[{index}].coverage_matrix[{coverage_index}] is invalid"
                )
            combination = row.get("combination")
            viewport = row.get("viewport")
            if not _is_nonempty_string(combination) or not _is_nonempty_string(viewport):
                raise ValidationInputError(
                    f"samples[{index}].coverage_matrix[{coverage_index}] has no case key"
                )
            key = _case_key(scenario_id, combination, viewport)
            if key not in expected_sample_keys or key in actual_sample_keys:
                raise ValidationInputError(
                    f"samples[{index}].coverage_matrix has unexpected or duplicate cases"
                )
            actual_sample_keys.add(key)
            requirements = _requirements_for(scenario, combination)
            task_id = row.get("task_id")
            revision_ids = row.get("revision_ids")
            resource_ids = row.get("resource_ids")
            resource_oracle = _normalized_resource_oracle(
                row.get("resource_oracle"),
                str(combination),
                str(scenario_id),
            )
            probes = _normalized_http_probes(row.get("http_probes"))
            if (
                row.get("requirements") != requirements
                or (requirements["requires_task_id"] and not _is_positive_int(task_id))
                or (
                    task_id is not None
                    and not _is_positive_int(task_id)
                )
                or not isinstance(revision_ids, list)
                or any(not _is_positive_int(value) for value in revision_ids)
                or len(revision_ids) != len(set(revision_ids))
                or (requirements["requires_revision_ids"] and not revision_ids)
                or not isinstance(resource_ids, list)
                or any(not _is_nonempty_string(value) for value in resource_ids)
                or len(resource_ids) != len(set(resource_ids))
                or resource_oracle is None
                or (
                    resource_oracle["kind"]
                    in {"v8_resource_groups", "v8_wrong_scope_rejected"}
                    and not resource_ids
                )
                or (
                    resource_oracle["kind"]
                    not in {
                        "v8_resource_groups",
                        "v8_wrong_scope_rejected",
                        "v8_missing_resource_group",
                        "v8_expected_no_resource_groups",
                    }
                    and (resource_ids or revision_ids)
                )
                or (
                    resource_oracle["kind"]
                    in {
                        "v8_missing_resource_group",
                        "v8_expected_no_resource_groups",
                    }
                    and (resource_ids or revision_ids)
                )
                or _normalized_expected_allowed_actions(
                    row.get("allowed_actions")
                )
                is None
                or probes is None
                or sorted(probe[3] for probe in probes)
                != sorted(requirements["required_http_statuses"])
                or not SHA256_RE.fullmatch(row.get("oracle_sha256") or "")
            ):
                raise ValidationInputError(
                    f"samples[{index}].coverage_matrix[{coverage_index}] "
                    "has invalid sealed identities or oracle"
                )
            expected_cases[key] = (sample, row)
        if actual_sample_keys != expected_sample_keys:
            raise ValidationInputError(
                f"samples[{index}].coverage_matrix does not cover its scenario"
            )
        samples_by_id[scenario_id] = sample
    if set(samples_by_id) != set(scenarios_by_id):
        raise ValidationInputError("samples manifest does not cover every scenario")
    return expected_cases, normalized_edges


def _add_failure(
    failures: list[dict[str, str]],
    code: str,
    detail: str,
    key: tuple[str, str, str] | None = None,
) -> None:
    failures.append(
        {
            "case_key": _case_key_text(key) if key else "document",
            "code": code,
            "detail": detail,
        }
    )


def _validate_document_header(
    document: dict[str, Any],
    expected_source: str,
    catalog_sha256: str,
    samples_sha256: str,
    failures: list[dict[str, str]],
) -> None:
    if document.get("schema_version") != SCHEMA_VERSION:
        _add_failure(failures, "document_schema", f"{expected_source}: schema_version != 1")
    if document.get("source_kind") != expected_source:
        _add_failure(
            failures,
            "source_kind",
            f"expected {expected_source}, got {document.get('source_kind')!r}",
        )
    if document.get("scenario_catalog_sha256") != catalog_sha256:
        _add_failure(
            failures,
            "catalog_hash",
            f"{expected_source}: scenario catalog hash does not match",
        )
    if document.get("samples_sha256") != samples_sha256:
        _add_failure(
            failures,
            "samples_hash",
            f"{expected_source}: runtime samples file hash does not match",
        )
    if not _is_nonempty_string(document.get("run_id")):
        _add_failure(failures, "run_id", f"{expected_source}: run_id is missing")
    if not isinstance(document.get("records"), list):
        _add_failure(failures, "records", f"{expected_source}: records must be an array")


def _index_records(
    document: dict[str, Any],
    expected_keys: set[tuple[str, str, str]],
    source_kind: str,
    failures: list[dict[str, str]],
) -> dict[tuple[str, str, str], dict[str, Any]]:
    records = document.get("records")
    if not isinstance(records, list):
        return {}
    indexed: dict[tuple[str, str, str], dict[str, Any]] = {}
    for index, record in enumerate(records):
        if not isinstance(record, dict):
            _add_failure(
                failures,
                "record_shape",
                f"{source_kind}: record[{index}] must be an object",
            )
            continue
        values = (
            record.get("scenario_id"),
            record.get("combination"),
            record.get("viewport"),
        )
        if not all(_is_nonempty_string(value) for value in values):
            _add_failure(
                failures,
                "record_key",
                f"{source_kind}: record[{index}] has an invalid key",
            )
            continue
        key = _case_key(values[0], values[1], values[2])
        if key not in expected_keys:
            _add_failure(
                failures,
                "unexpected_record",
                f"{source_kind}: record is not declared by the catalog",
                key,
            )
            continue
        if key in indexed:
            _add_failure(
                failures,
                "duplicate_record",
                f"{source_kind}: duplicate record",
                key,
            )
            continue
        indexed[key] = record
    return indexed


def _validate_png(path: Path, expected_spec: dict[str, int]) -> bool:
    try:
        payload = path.read_bytes()
    except OSError:
        return False
    if not payload.startswith(b"\x89PNG\r\n\x1a\n"):
        return False
    cursor = 8
    dimensions: tuple[int, int] | None = None
    compressed = bytearray()
    saw_iend = False
    try:
        while cursor + 12 <= len(payload):
            length = struct.unpack(">I", payload[cursor : cursor + 4])[0]
            kind = payload[cursor + 4 : cursor + 8]
            end = cursor + 12 + length
            if end > len(payload):
                return False
            data = payload[cursor + 8 : cursor + 8 + length]
            declared_crc = struct.unpack(">I", payload[cursor + 8 + length : end])[0]
            if zlib.crc32(kind + data) & 0xFFFFFFFF != declared_crc:
                return False
            if kind == b"IHDR":
                if dimensions is not None or length != 13:
                    return False
                dimensions = struct.unpack(">II", data[:8])
            elif kind == b"IDAT":
                compressed.extend(data)
            elif kind == b"IEND":
                if length != 0:
                    return False
                saw_iend = True
                cursor = end
                break
            cursor = end
        if (
            not saw_iend
            or cursor != len(payload)
            or dimensions
            != (expected_spec["width"], expected_spec["height"])
            or not compressed
        ):
            return False
        zlib.decompress(bytes(compressed))
    except (KeyError, struct.error, zlib.error):
        return False
    return True


def _load_artifact_json(path: Path) -> dict[str, Any] | None:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError):
        return None
    return value if isinstance(value, dict) else None


def _validate_console_artifact(
    path: Path,
    key: tuple[str, str, str],
    source_kind: str,
) -> bool:
    value = _load_artifact_json(path)
    if (
        not isinstance(value, dict)
        or set(value) != {
            "schema_version",
            "case_key",
            "source_kind",
            "unexpected_error_count",
            "entries",
        }
        or value.get("schema_version") != SCHEMA_VERSION
        or value.get("case_key") != _case_key_text(key)
        or value.get("source_kind") != source_kind
        or value.get("unexpected_error_count") != 0
        or not isinstance(value.get("entries"), list)
    ):
        return False
    for entry in value["entries"]:
        if (
            not isinstance(entry, dict)
            or entry.get("level") not in {"log", "info", "warning", "error"}
            or not isinstance(entry.get("text"), str)
        ):
            return False
    return True


def _validate_network_artifact(
    path: Path,
    key: tuple[str, str, str],
    source_kind: str,
    expected_origin: str,
    expected_probes: tuple[tuple[str, str, str, int], ...],
) -> bool:
    value = _load_artifact_json(path)
    if (
        not isinstance(value, dict)
        or set(value) != {
            "schema_version",
            "case_key",
            "source_kind",
            "five_xx_count",
            "requests",
        }
        or value.get("schema_version") != SCHEMA_VERSION
        or value.get("case_key") != _case_key_text(key)
        or value.get("source_kind") != source_kind
        or value.get("five_xx_count") != 0
        or not isinstance(value.get("requests"), list)
        or not value["requests"]
    ):
        return False
    calculated_5xx = 0
    observed_requests: set[tuple[str, str, int]] = set()
    for request in value["requests"]:
        if (
            not isinstance(request, dict)
            or set(request) != {"method", "url", "status"}
            or not _is_nonempty_string(request.get("method"))
            or request["method"] != request["method"].upper()
            or _url_origin(request.get("url")) != expected_origin
            or not _is_nonnegative_int(request.get("status"))
            or request["status"] < 100
            or request["status"] > 599
        ):
            return False
        calculated_5xx += int(request["status"] >= 500)
        observed_requests.add(
            (
                request["method"],
                request["url"][len(expected_origin) :],
                request["status"],
            )
        )
    required_requests = {
        (method, path, status)
        for _, method, path, status in expected_probes
    }
    return (
        calculated_5xx == value["five_xx_count"] == 0
        and required_requests.issubset(observed_requests)
    )


def _validate_playwright_trace(path: Path) -> bool:
    try:
        with zipfile.ZipFile(path) as archive:
            names = set(archive.namelist())
            if (
                "trace.trace" not in names
                or "trace.network" not in names
                or len(names) > 10000
                or any(
                    Path(name).is_absolute() or ".." in Path(name).parts
                    for name in names
                )
                or archive.testzip() is not None
            ):
                return False
            first_trace_line = archive.read("trace.trace").splitlines()[0]
            if not isinstance(json.loads(first_trace_line), dict):
                return False
    except (OSError, IndexError, KeyError, UnicodeError, ValueError, zipfile.BadZipFile):
        return False
    return True


def _validate_artifacts(
    record: dict[str, Any],
    key: tuple[str, str, str],
    source_kind: str,
    artifact_root: Path,
    failures: list[dict[str, str]],
    seen_paths: set[tuple[str, str]],
    expected_viewport_spec: dict[str, int],
    expected_origin: str,
    expected_http_probes: tuple[tuple[str, str, str, int], ...],
) -> None:
    artifacts = record.get("artifacts")
    if not isinstance(artifacts, list):
        _add_failure(failures, "artifacts", f"{source_kind}: artifacts must be an array", key)
        return
    required_kinds = {"screenshot", "console", "network"}
    if source_kind == SOURCE_PLAYWRIGHT:
        required_kinds.add("trace")
    actual_kinds: set[str] = set()
    root = artifact_root.resolve()
    for index, artifact in enumerate(artifacts):
        if not isinstance(artifact, dict):
            _add_failure(
                failures,
                "artifact_shape",
                f"{source_kind}: artifact[{index}] must be an object",
                key,
            )
            continue
        kind = artifact.get("kind")
        relative_path = artifact.get("path")
        expected_sha = artifact.get("sha256")
        if kind not in {"screenshot", "console", "network", "trace"}:
            _add_failure(
                failures,
                "artifact_kind",
                f"{source_kind}: artifact[{index}] has invalid kind",
                key,
            )
            continue
        actual_kinds.add(kind)
        if (
            not _is_nonempty_string(relative_path)
            or Path(relative_path).is_absolute()
            or not SHA256_RE.fullmatch(expected_sha or "")
        ):
            _add_failure(
                failures,
                "artifact_metadata",
                f"{source_kind}: artifact[{index}] has invalid path or sha256",
                key,
            )
            continue
        raw_path = artifact_root / relative_path
        try:
            resolved_path = raw_path.resolve(strict=True)
            resolved_path.relative_to(root)
        except (OSError, RuntimeError, ValueError):
            _add_failure(
                failures,
                "artifact_boundary",
                f"{source_kind}: artifact path escapes the artifact root or is missing",
                key,
            )
            continue
        cursor = artifact_root
        has_symlink = False
        for part in Path(relative_path).parts:
            cursor = cursor / part
            if cursor.is_symlink():
                has_symlink = True
                break
        if has_symlink or not resolved_path.is_file():
            _add_failure(
                failures,
                "artifact_file",
                f"{source_kind}: artifact must be a regular non-symlink file",
                key,
            )
            continue
        path_key = (source_kind, relative_path)
        if path_key in seen_paths:
            _add_failure(
                failures,
                "artifact_reuse",
                f"{source_kind}: artifact path is reused by another record",
                key,
            )
        seen_paths.add(path_key)
        if file_sha256(resolved_path) != expected_sha:
            _add_failure(
                failures,
                "artifact_hash",
                f"{source_kind}: artifact[{index}] sha256 does not match file bytes",
                key,
            )
            continue
        format_ok = True
        format_code = ""
        if kind == "screenshot":
            format_ok = _validate_png(resolved_path, expected_viewport_spec)
            format_code = "screenshot_png"
        elif kind == "console":
            format_ok = _validate_console_artifact(resolved_path, key, source_kind)
            format_code = "console_schema"
        elif kind == "network":
            format_ok = _validate_network_artifact(
                resolved_path,
                key,
                source_kind,
                expected_origin,
                expected_http_probes,
            )
            format_code = "network_schema"
        elif kind == "trace":
            format_ok = (
                source_kind == SOURCE_PLAYWRIGHT
                and _validate_playwright_trace(resolved_path)
            )
            format_code = "trace_zip"
        if not format_ok:
            _add_failure(
                failures,
                format_code,
                f"{source_kind}: {kind} artifact content is invalid",
                key,
            )
    missing = sorted(required_kinds - actual_kinds)
    if missing:
        _add_failure(
            failures,
            "artifact_coverage",
            f"{source_kind}: missing artifact kinds: {', '.join(missing)}",
            key,
        )


def _normalized_allowed_actions(
    assertions: dict[str, Any],
) -> tuple[tuple[str, tuple[str, ...], tuple[str, ...]], ...] | None:
    rows = assertions.get("allowed_actions")
    if not isinstance(rows, list) or not rows:
        return None
    normalized: list[tuple[str, tuple[str, ...], tuple[str, ...]]] = []
    seen_checkpoints: set[str] = set()
    for row in rows:
        if not isinstance(row, dict) or not _is_nonempty_string(row.get("checkpoint")):
            return None
        checkpoint = row["checkpoint"]
        expected = row.get("expected")
        observed = row.get("observed")
        if (
            checkpoint in seen_checkpoints
            or not isinstance(expected, list)
            or not isinstance(observed, list)
            or any(not _is_nonempty_string(item) for item in expected + observed)
            or len(expected) != len(set(expected))
            or len(observed) != len(set(observed))
            or set(expected) != set(observed)
        ):
            return None
        seen_checkpoints.add(checkpoint)
        normalized.append(
            (checkpoint, tuple(sorted(expected)), tuple(sorted(observed)))
        )
    return tuple(sorted(normalized))


def _validate_record(
    record: dict[str, Any],
    scenario: dict[str, Any],
    sample: dict[str, Any],
    coverage: dict[str, Any],
    edge: dict[str, Any],
    samples_sha256: str,
    key: tuple[str, str, str],
    source_kind: str,
    artifact_root: Path,
    failures: list[dict[str, str]],
    seen_paths: set[tuple[str, str]],
) -> None:
    requirements = _requirements_for(scenario, key[1])
    if record.get("schema_version") != SCHEMA_VERSION:
        _add_failure(failures, "record_schema", f"{source_kind}: schema_version != 1", key)
    if record.get("status") != "PASS":
        _add_failure(failures, "record_status", f"{source_kind}: status must be PASS", key)
    executor = record.get("executor_id")
    reviewer = record.get("reviewer_id")
    if not _is_nonempty_string(executor) or not _is_nonempty_string(reviewer):
        _add_failure(
            failures,
            "actor_identity",
            f"{source_kind}: executor_id and reviewer_id are required",
            key,
        )
    elif executor == reviewer:
        _add_failure(
            failures,
            "self_review",
            f"{source_kind}: executor and reviewer must differ",
            key,
        )
    started_at = _parse_rfc3339(record.get("started_at"))
    finished_at = _parse_rfc3339(record.get("finished_at"))
    if started_at is None or finished_at is None:
        _add_failure(
            failures,
            "record_time",
            f"{source_kind}: started_at and finished_at must be RFC3339 timestamps",
            key,
        )
    elif finished_at < started_at:
        _add_failure(
            failures,
            "record_time_order",
            f"{source_kind}: finished_at is before started_at",
            key,
        )
    expected_origin = EXPECTED_EDGE_ORIGINS[key[1]]
    actual_origin = _url_origin(record.get("url"))
    if actual_origin is None:
        _add_failure(failures, "url", f"{source_kind}: URL is invalid or unsafe", key)
    elif actual_origin != expected_origin:
        _add_failure(
            failures,
            "edge_origin",
            f"{source_kind}: URL origin must be {expected_origin}",
            key,
        )

    if record.get("samples_sha256") != samples_sha256:
        _add_failure(
            failures,
            "samples_hash",
            f"{source_kind}: record is not bound to the runtime samples file",
            key,
        )
    if record.get("sample_sha256") != sample["sample_sha256"]:
        _add_failure(
            failures,
            "sample_hash",
            f"{source_kind}: sample_sha256 does not match the sealed sample",
            key,
        )
    expected_edge_identity = {
        name: edge[name]
        for name in (
            "edge",
            "frontend_sha256",
            "backend_sha256",
            "fixture_identity",
        )
    }
    if record.get("edge_identity") != expected_edge_identity:
        _add_failure(
            failures,
            "edge_identity",
            f"{source_kind}: edge_identity does not match sealed fingerprints",
            key,
        )
    expected_viewport_spec = EXPECTED_VIEWPORT_SPECS[key[2]]
    if record.get("viewport_spec") != expected_viewport_spec:
        _add_failure(
            failures,
            "viewport_spec",
            f"{source_kind}: viewport dimensions or DPR are not fixed",
            key,
        )

    task_id = record.get("task_id")
    if requirements["requires_task_id"] and not _is_positive_int(task_id):
        _add_failure(
            failures,
            "task_id",
            f"{source_kind}: a positive task_id is required",
            key,
        )
    elif task_id is not None and not _is_positive_int(task_id):
        _add_failure(
            failures,
            "task_id",
            f"{source_kind}: task_id must be null or a positive integer",
            key,
        )
    if task_id != coverage["task_id"]:
        _add_failure(
            failures,
            "sample_task_id",
            f"{source_kind}: task_id differs from the sealed runtime sample",
            key,
        )
    revision_ids = record.get("revision_ids")
    if not isinstance(revision_ids, list) or any(
        not _is_positive_int(revision_id) for revision_id in revision_ids
    ):
        _add_failure(
            failures,
            "revision_ids",
            f"{source_kind}: revision_ids must be positive integers",
            key,
        )
        revision_ids = []
    elif len(revision_ids) != len(set(revision_ids)):
        _add_failure(
            failures,
            "revision_ids",
            f"{source_kind}: revision_ids contain duplicates",
            key,
        )
    if requirements["requires_revision_ids"] and not revision_ids:
        _add_failure(
            failures,
            "revision_ids",
            f"{source_kind}: at least one revision_id is required",
            key,
        )
    if revision_ids != coverage["revision_ids"]:
        _add_failure(
            failures,
            "sample_revision_ids",
            f"{source_kind}: revision_ids differ from the sealed runtime sample",
            key,
        )
    resource_ids = record.get("resource_ids")
    if resource_ids != coverage["resource_ids"]:
        _add_failure(
            failures,
            "sample_resource_ids",
            f"{source_kind}: resource_ids differ from the sealed runtime sample",
            key,
        )

    assertions = record.get("assertions")
    if not isinstance(assertions, dict):
        _add_failure(
            failures,
            "assertions",
            f"{source_kind}: assertions must be an object",
            key,
        )
        assertions = {}
    for assertion_name in requirements["required_assertions"]:
        if assertion_name == "allowed_actions_exact":
            if _normalized_allowed_actions(assertions) is None:
                _add_failure(
                    failures,
                    "allowed_actions",
                    f"{source_kind}: allowed_actions are missing or differ",
                    key,
                )
        elif assertions.get(assertion_name) is not True:
            _add_failure(
                failures,
                "required_assertion",
                f"{source_kind}: assertion {assertion_name} is not true",
                key,
            )
    observed_actions = _normalized_allowed_actions(assertions)
    normalized_observed_expectations = (
        tuple((checkpoint, expected) for checkpoint, expected, _ in observed_actions)
        if observed_actions is not None
        else None
    )
    if normalized_observed_expectations != _normalized_expected_allowed_actions(
        coverage["allowed_actions"]
    ):
        _add_failure(
            failures,
            "allowed_actions_oracle",
            f"{source_kind}: allowed_actions differ from the sealed API oracle",
            key,
        )
    if assertions.get("console_unexpected_error_count") != 0:
        _add_failure(
            failures,
            "console_errors",
            f"{source_kind}: unexpected console error count must be zero",
            key,
        )
    if assertions.get("network_5xx_count") != 0:
        _add_failure(
            failures,
            "network_5xx",
            f"{source_kind}: network 5xx count must be zero",
            key,
        )
    if assertions.get("oracle_sha256") != coverage["oracle_sha256"]:
        _add_failure(
            failures,
            "oracle_hash",
            f"{source_kind}: assertions are not bound to the sealed oracle",
            key,
        )

    history = assertions.get("history_drawer")
    if requirements["requires_history_drawer"]:
        if not isinstance(history, dict):
            _add_failure(
                failures,
                "history_drawer",
                f"{source_kind}: history_drawer evidence is required",
                key,
            )
        else:
            history_revision_ids = history.get("revision_ids")
            if (
                history.get("opened") is not True
                or history.get("stage_status_actor_file_time_checked") is not True
                or not isinstance(history_revision_ids, list)
                or any(
                    not _is_positive_int(revision_id)
                    for revision_id in history_revision_ids
                )
                or set(history_revision_ids) != set(revision_ids)
            ):
                _add_failure(
                    failures,
                    "history_drawer",
                    f"{source_kind}: history drawer does not prove the record revisions",
                    key,
                )

    http_rows = assertions.get("http_statuses")
    if not isinstance(http_rows, list):
        _add_failure(
            failures,
            "http_statuses",
            f"{source_kind}: http_statuses must be an array",
            key,
        )
        http_rows = []
    matched_statuses: set[int] = set()
    matched_probes: set[tuple[str, int]] = set()
    seen_probe_names: set[str] = set()
    for row in http_rows:
        if not isinstance(row, dict):
            _add_failure(
                failures,
                "http_status",
                f"{source_kind}: HTTP status evidence is malformed",
                key,
            )
            continue
        expected = row.get("expected_status")
        actual = row.get("actual_status")
        name = row.get("name")
        if (
            not _is_nonempty_string(name)
            or name in seen_probe_names
            or not _is_nonnegative_int(expected)
            or not _is_nonnegative_int(actual)
            or expected < 100
            or expected > 599
            or actual < 100
            or actual > 599
            or expected != actual
        ):
            _add_failure(
                failures,
                "http_status",
                f"{source_kind}: expected and actual HTTP statuses must match",
                key,
            )
            continue
        seen_probe_names.add(name)
        if actual >= 500:
            _add_failure(
                failures,
                "http_5xx",
                f"{source_kind}: HTTP evidence contains a 5xx response",
                key,
            )
        matched_statuses.add(actual)
        matched_probes.add((name, actual))
    for required_status in requirements["required_http_statuses"]:
        if required_status not in matched_statuses:
            _add_failure(
                failures,
                "required_http_status",
                f"{source_kind}: required HTTP {required_status} evidence is missing",
                key,
            )
    expected_probe_results = {
        (kind, status)
        for kind, _, _, status in (
            _normalized_http_probes(coverage["http_probes"]) or ()
        )
    }
    if matched_probes != expected_probe_results:
        _add_failure(
            failures,
            "http_probe_oracle",
            f"{source_kind}: HTTP results differ from the sealed probe oracle",
            key,
        )

    declared_hash = record.get("record_sha256")
    calculated_hash = record_sha256(record)
    if not SHA256_RE.fullmatch(declared_hash or "") or declared_hash != calculated_hash:
        _add_failure(
            failures,
            "record_hash",
            f"{source_kind}: record_sha256 does not match canonical record content",
            key,
        )
    _validate_artifacts(
        record,
        key,
        source_kind,
        artifact_root,
        failures,
        seen_paths,
        expected_viewport_spec,
        expected_origin,
        _normalized_http_probes(coverage["http_probes"]) or (),
    )


def _validate_pair(
    browser_record: dict[str, Any],
    playwright_record: dict[str, Any],
    key: tuple[str, str, str],
    failures: list[dict[str, str]],
) -> None:
    browser_executor = browser_record.get("executor_id")
    playwright_executor = playwright_record.get("executor_id")
    browser_reviewer = browser_record.get("reviewer_id")
    playwright_reviewer = playwright_record.get("reviewer_id")
    if (
        _is_nonempty_string(browser_executor)
        and _is_nonempty_string(playwright_executor)
        and browser_executor == playwright_executor
    ):
        _add_failure(
            failures,
            "executor_independence",
            "Browser/Computer Use and Playwright executors must differ",
            key,
        )
    executors = {
        actor
        for actor in (browser_executor, playwright_executor)
        if _is_nonempty_string(actor)
    }
    for reviewer in (browser_reviewer, playwright_reviewer):
        if _is_nonempty_string(reviewer) and reviewer in executors:
            _add_failure(
                failures,
                "review_independence",
                "neither reviewer may be either source executor",
                key,
            )
    for field in (
        "url",
        "task_id",
        "revision_ids",
        "resource_ids",
        "sample_sha256",
        "samples_sha256",
        "edge_identity",
        "viewport_spec",
    ):
        if browser_record.get(field) != playwright_record.get(field):
            _add_failure(
                failures,
                "pair_mismatch",
                f"Browser/Computer Use and Playwright {field} values differ",
                key,
            )
    browser_assertions = browser_record.get("assertions")
    playwright_assertions = playwright_record.get("assertions")
    if isinstance(browser_assertions, dict) and isinstance(playwright_assertions, dict):
        if browser_assertions.get("oracle_sha256") != playwright_assertions.get(
            "oracle_sha256"
        ):
            _add_failure(
                failures,
                "pair_oracle",
                "Browser/Computer Use and Playwright oracle hashes differ",
                key,
            )
        if _normalized_allowed_actions(browser_assertions) != _normalized_allowed_actions(
            playwright_assertions
        ):
            _add_failure(
                failures,
                "pair_allowed_actions",
                "Browser/Computer Use and Playwright allowed_actions evidence differs",
                key,
            )
        if browser_assertions.get("history_drawer") != playwright_assertions.get(
            "history_drawer"
        ):
            _add_failure(
                failures,
                "pair_history",
                "Browser/Computer Use and Playwright history drawer evidence differs",
                key,
            )
    browser_artifacts = browser_record.get("artifacts")
    playwright_artifacts = playwright_record.get("artifacts")
    browser_paths = {
        row.get("path")
        for row in browser_artifacts
        if isinstance(row, dict)
    } if isinstance(browser_artifacts, list) else set()
    playwright_paths = {
        row.get("path")
        for row in playwright_artifacts
        if isinstance(row, dict)
    } if isinstance(playwright_artifacts, list) else set()
    shared_paths = sorted(
        path for path in browser_paths & playwright_paths if _is_nonempty_string(path)
    )
    if shared_paths:
        _add_failure(
            failures,
            "source_artifact_independence",
            "Browser/Computer Use and Playwright reuse artifact paths",
            key,
        )


def validate_evidence(
    *,
    catalog_path: Path,
    samples_path: Path,
    browser_evidence_path: Path,
    playwright_evidence_path: Path,
    artifact_root: Path,
) -> dict[str, Any]:
    catalog_path = catalog_path.resolve()
    samples_path = samples_path.resolve()
    browser_evidence_path = browser_evidence_path.resolve()
    playwright_evidence_path = playwright_evidence_path.resolve()
    if browser_evidence_path == playwright_evidence_path:
        raise ValidationInputError("the two evidence inputs must be different files")
    if not artifact_root.exists() or not artifact_root.is_dir():
        raise ValidationInputError("artifact_root must be an existing directory")

    catalog = load_json_object(catalog_path)
    scenarios = _validate_catalog(catalog)
    samples_manifest = load_json_object(samples_path)
    browser_document = load_json_object(browser_evidence_path)
    playwright_document = load_json_object(playwright_evidence_path)
    catalog_hash = file_sha256(catalog_path)
    samples_hash = file_sha256(samples_path)
    expected_samples, sealed_edges = _validate_samples_manifest(
        samples_manifest,
        scenarios,
        catalog_hash,
    )
    browser_hash = file_sha256(browser_evidence_path)
    playwright_hash = file_sha256(playwright_evidence_path)
    failures: list[dict[str, str]] = []

    _validate_document_header(
        browser_document,
        SOURCE_BROWSER,
        catalog_hash,
        samples_hash,
        failures,
    )
    _validate_document_header(
        playwright_document,
        SOURCE_PLAYWRIGHT,
        catalog_hash,
        samples_hash,
        failures,
    )
    browser_run_id = browser_document.get("run_id")
    playwright_run_id = playwright_document.get("run_id")
    if (
        _is_nonempty_string(browser_run_id)
        and _is_nonempty_string(playwright_run_id)
        and browser_run_id != playwright_run_id
    ):
        _add_failure(failures, "run_id_mismatch", "evidence run_id values differ")
    if browser_hash == playwright_hash:
        _add_failure(failures, "source_document_independence", "evidence files are identical")

    expected_keys: set[tuple[str, str, str]] = set()
    scenarios_by_id: dict[str, dict[str, Any]] = {}
    for scenario in scenarios:
        scenarios_by_id[scenario["id"]] = scenario
        for combination in scenario["required_combinations"]:
            for viewport in scenario["required_viewports"]:
                expected_keys.add(_case_key(scenario["id"], combination, viewport))
    if set(expected_samples) != expected_keys:
        raise ValidationInputError(
            "runtime samples cases do not exactly match the scenario catalog"
        )

    browser_records = _index_records(
        browser_document,
        expected_keys,
        SOURCE_BROWSER,
        failures,
    )
    playwright_records = _index_records(
        playwright_document,
        expected_keys,
        SOURCE_PLAYWRIGHT,
        failures,
    )
    documents_are_valid = not failures
    seen_artifact_paths: set[tuple[str, str]] = set()
    cases: list[dict[str, Any]] = []
    passed_case_count = 0
    for key in sorted(expected_keys):
        failure_count_before = len(failures)
        browser_record = browser_records.get(key)
        playwright_record = playwright_records.get(key)
        if browser_record is None:
            _add_failure(
                failures,
                "missing_browser_record",
                "Browser/Computer Use evidence record is missing",
                key,
            )
        if playwright_record is None:
            _add_failure(
                failures,
                "missing_playwright_record",
                "Playwright evidence record is missing",
                key,
            )
        scenario = scenarios_by_id[key[0]]
        sample, coverage = expected_samples[key]
        edge = sealed_edges[key[1]]
        if browser_record is not None:
            _validate_record(
                browser_record,
                scenario,
                sample,
                coverage,
                edge,
                samples_hash,
                key,
                SOURCE_BROWSER,
                artifact_root,
                failures,
                seen_artifact_paths,
            )
        if playwright_record is not None:
            _validate_record(
                playwright_record,
                scenario,
                sample,
                coverage,
                edge,
                samples_hash,
                key,
                SOURCE_PLAYWRIGHT,
                artifact_root,
                failures,
                seen_artifact_paths,
            )
        if browser_record is not None and playwright_record is not None:
            _validate_pair(browser_record, playwright_record, key, failures)
        case_passed = documents_are_valid and len(failures) == failure_count_before
        if case_passed:
            passed_case_count += 1
        browser_record_hash = (
            browser_record.get("record_sha256")
            if isinstance(browser_record, dict)
            else None
        )
        playwright_record_hash = (
            playwright_record.get("record_sha256")
            if isinstance(playwright_record, dict)
            else None
        )
        pair_hash = None
        if SHA256_RE.fullmatch(browser_record_hash or "") and SHA256_RE.fullmatch(
            playwright_record_hash or ""
        ):
            pair_hash = canonical_sha256(
                {
                    "browser_record_sha256": browser_record_hash,
                    "case_key": list(key),
                    "playwright_record_sha256": playwright_record_hash,
                    "scenario_catalog_sha256": catalog_hash,
                    "samples_sha256": samples_hash,
                }
            )
        cases.append(
            {
                "scenario_id": key[0],
                "combination": key[1],
                "viewport": key[2],
                "status": "PASS" if case_passed else "FAIL",
                "browser_record_sha256": browser_record_hash,
                "playwright_record_sha256": playwright_record_hash,
                "pair_sha256": pair_hash,
            }
        )

    required_case_count = len(expected_keys)
    failed_case_count = required_case_count - passed_case_count
    status = (
        "PASS"
        if not failures
        and required_case_count > 0
        and passed_case_count == required_case_count
        else "FAIL"
    )
    return {
        "schema_version": SCHEMA_VERSION,
        "gate": GATE,
        "status": status,
        "run_id": browser_run_id if browser_run_id == playwright_run_id else None,
        "scenario_catalog_sha256": catalog_hash,
        "samples_sha256": samples_hash,
        "runtime_manifest_sha256": samples_manifest["manifest_sha256"],
        "browser_evidence_sha256": browser_hash,
        "playwright_evidence_sha256": playwright_hash,
        "required_case_count": required_case_count,
        "passed_case_count": passed_case_count,
        "failed_case_count": failed_case_count,
        "critical_pass_rate": (
            passed_case_count / required_case_count if required_case_count else 0.0
        ),
        "source_kinds": [SOURCE_BROWSER, SOURCE_PLAYWRIGHT],
        "failures": failures,
        "cases": cases,
        "generated_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
    }


def _failure_report(detail: str) -> dict[str, Any]:
    return {
        "schema_version": SCHEMA_VERSION,
        "gate": GATE,
        "status": "FAIL",
        "run_id": None,
        "required_case_count": 0,
        "passed_case_count": 0,
        "failed_case_count": 0,
        "critical_pass_rate": 0.0,
        "source_kinds": [SOURCE_BROWSER, SOURCE_PLAYWRIGHT],
        "failures": [
            {
                "case_key": "document",
                "code": "input_error",
                "detail": detail,
            }
        ],
        "cases": [],
        "generated_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
    }


def _write_new_json(path: Path, value: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("x", encoding="utf-8", newline="\n") as stream:
        json.dump(value, stream, ensure_ascii=False, sort_keys=True, indent=2)
        stream.write("\n")


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--scenarios", required=True, type=Path)
    parser.add_argument("--samples", required=True, type=Path)
    parser.add_argument("--browser-evidence", required=True, type=Path)
    parser.add_argument("--playwright-evidence", required=True, type=Path)
    parser.add_argument("--artifact-root", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    if args.output.exists():
        print(f"refusing to overwrite existing output: {args.output}", file=sys.stderr)
        return 2
    try:
        report = validate_evidence(
            catalog_path=args.scenarios,
            samples_path=args.samples,
            browser_evidence_path=args.browser_evidence,
            playwright_evidence_path=args.playwright_evidence,
            artifact_root=args.artifact_root,
        )
    except ValidationInputError as exc:
        report = _failure_report(str(exc))
    try:
        _write_new_json(args.output, report)
    except FileExistsError:
        print(f"refusing to overwrite existing output: {args.output}", file=sys.stderr)
        return 2
    print(
        f"{report['gate']} {report['status']}: "
        f"{report['passed_case_count']}/{report['required_case_count']} critical cases"
    )
    return 0 if report["status"] == "PASS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
