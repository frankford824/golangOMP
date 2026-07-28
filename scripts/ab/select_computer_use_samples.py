#!/usr/bin/env python3
"""Select deterministic, evidence-bound samples for the G7 UI test catalog.

The selector is deliberately offline.  It reads the scenario catalog, mapping
v2, and the canonical entity document produced from frozen Clone A.  It never
connects to a database or a service and never mutates a test environment.

`final` mode is fail-closed: every reviewable mapping row must be confirmed and
the canonical document must be bound to the exact mapping bytes.  `prepare`
mode may inspect a rebased mapping, but its result is always PENDING.

Negative tests are represented by explicit, isolated Clone B fixture plans.
Existing malformed production rows are never selected as negative fixtures.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
import tempfile
from collections import defaultdict
from pathlib import Path
from typing import Any, Callable, Iterable


SCHEMA_VERSION = 1
SELECTOR_VERSION = "computer-use-sample-selector-v1"
EXPECTED_COMBINATIONS = {
    "external_external",
    "devplus_devplus",
    "external_devplus",
    "devplus_external",
}
EXPECTED_VIEWPORTS = {"desktop", "mobile"}
EXPECTED_SCENARIO_COUNT = 30
EXPECTED_EDGE_ORIGINS = {
    "external_external": "http://127.0.0.1:18101",
    "devplus_devplus": "http://127.0.0.1:18102",
    "external_devplus": "http://127.0.0.1:18103",
    "devplus_external": "http://127.0.0.1:18104",
}
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
REQUIREMENT_FIELDS = (
    "requires_task_id",
    "requires_revision_ids",
    "requires_history_drawer",
    "required_http_statuses",
    "required_assertions",
)
NEGATIVE_FIXTURE_SCENARIOS = {
    "unauthorized_asset_403": "permission_denied_identity",
    "missing_resource_group_negative": "missing_resource_group",
    "missing_current_pointer_negative": "missing_current_pointer",
    "wrong_scope_negative": "wrong_scope_asset",
}
RETOUCH_REOPEN_TASK_ID = 1264
RETOUCH_REOPEN_SCOPE_REF_ID = 45
RETOUCH_REOPEN_REVISION_IDS = [635, 636]
EXPECTED_NO_RESOURCE_GROUP_SCENARIOS = {
    "purchase_to_sku_planning",
}


class InputError(ValueError):
    """Raised when an input is malformed or cannot be safely correlated."""


def canonical_json_bytes(value: Any) -> bytes:
    return json.dumps(
        value,
        ensure_ascii=False,
        sort_keys=True,
        separators=(",", ":"),
    ).encode("utf-8")


def canonical_sha256(value: Any) -> str:
    return hashlib.sha256(canonical_json_bytes(value)).hexdigest()


def file_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def load_json(path: Path, label: str) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise InputError(f"cannot read {label} {path}: {exc}") from exc
    if not isinstance(value, dict):
        raise InputError(f"{label} must be a JSON object")
    return value


def atomic_write(path: Path, value: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(descriptor, "wb") as stream:
            stream.write(canonical_json_bytes(value) + b"\n")
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
    except BaseException:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise


def nonempty(value: Any) -> bool:
    return isinstance(value, str) and bool(value.strip())


def positive_int(value: Any) -> bool:
    return isinstance(value, int) and not isinstance(value, bool) and value > 0


def canonical_positive_int(value: Any) -> int | None:
    if not isinstance(value, str) or not re.fullmatch(r"[1-9][0-9]*", value):
        return None
    return int(value)


def confirmed_time(value: Any) -> bool:
    return nonempty(value) and not str(value).startswith("0001-01-01T00:00:00")


def validate_manifest_hash(
    document: dict[str, Any],
    field: str,
    label: str,
) -> str:
    declared = document.get(field)
    payload = dict(document)
    payload.pop(field, None)
    if (
        not SHA256_RE.fullmatch(str(declared or ""))
        or declared != canonical_sha256(payload)
    ):
        raise InputError(f"{label} has an invalid {field}")
    return str(declared)


def validate_edge_receipt(receipt: dict[str, Any]) -> dict[str, dict[str, Any]]:
    if (
        receipt.get("schema_version") != SCHEMA_VERSION
        or receipt.get("gate") != "G7"
        or receipt.get("status") != "PASS"
    ):
        raise InputError("edge receipt must be schema_version=1, gate=G7, status=PASS")
    validate_manifest_hash(receipt, "receipt_sha256", "edge receipt")
    edges = receipt.get("edges")
    if not isinstance(edges, dict) or set(edges) != EXPECTED_COMBINATIONS:
        raise InputError("edge receipt must contain exactly the four G7 combinations")
    normalized: dict[str, dict[str, Any]] = {}
    for combination, expected_origin in EXPECTED_EDGE_ORIGINS.items():
        edge = edges.get(combination)
        if not isinstance(edge, dict):
            raise InputError(f"edge receipt {combination} must be an object")
        normalized_edge = {
            "origin": edge.get("origin"),
            "edge": edge.get("edge"),
            "frontend_sha256": edge.get("frontend_sha256"),
            "backend_sha256": edge.get("backend_sha256"),
            "fixture_identity": edge.get("fixture_identity"),
        }
        if (
            normalized_edge["origin"] != expected_origin
            or normalized_edge["edge"] != combination
        ):
            raise InputError(
                f"edge receipt {combination} must use fixed origin {expected_origin}"
            )
        if (
            not SHA256_RE.fullmatch(str(normalized_edge["frontend_sha256"] or ""))
            or not SHA256_RE.fullmatch(str(normalized_edge["backend_sha256"] or ""))
            or not nonempty(normalized_edge["fixture_identity"])
        ):
            raise InputError(f"edge receipt {combination} has invalid fingerprints")
        normalized[combination] = normalized_edge
    return normalized


def normalize_allowed_actions(value: Any, label: str) -> list[dict[str, Any]]:
    if not isinstance(value, list) or not value:
        raise InputError(f"{label} allowed_actions must be a non-empty array")
    normalized: list[dict[str, Any]] = []
    seen: set[str] = set()
    for index, row in enumerate(value):
        if not isinstance(row, dict) or set(row) != {"checkpoint", "expected"}:
            raise InputError(f"{label} allowed_actions[{index}] has an invalid shape")
        checkpoint = row.get("checkpoint")
        expected = row.get("expected")
        if (
            not nonempty(checkpoint)
            or checkpoint in seen
            or not isinstance(expected, list)
            or any(not nonempty(action) for action in expected)
            or len(expected) != len(set(expected))
        ):
            raise InputError(f"{label} allowed_actions[{index}] is invalid")
        seen.add(str(checkpoint))
        normalized.append(
            {
                "checkpoint": str(checkpoint),
                "expected": sorted(str(action) for action in expected),
            }
        )
    return sorted(normalized, key=lambda row: row["checkpoint"])


def normalize_http_probes(
    value: Any,
    required_statuses: list[int],
    label: str,
) -> list[dict[str, Any]]:
    if not isinstance(value, list):
        raise InputError(f"{label} http_probes must be an array")
    normalized: list[dict[str, Any]] = []
    seen: set[str] = set()
    for index, row in enumerate(value):
        if not isinstance(row, dict) or set(row) != {
            "kind",
            "method",
            "path",
            "expected_status",
        }:
            raise InputError(f"{label} http_probes[{index}] has an invalid shape")
        kind = row.get("kind")
        path = row.get("path")
        status = row.get("expected_status")
        if (
            not nonempty(kind)
            or kind in seen
            or row.get("method") != "GET"
            or not nonempty(path)
            or not str(path).startswith("/")
            or str(path).startswith("//")
            or "#" in str(path)
            or ".." in Path(str(path)).parts
            or not isinstance(status, int)
            or isinstance(status, bool)
            or status < 100
            or status > 599
        ):
            raise InputError(f"{label} http_probes[{index}] is invalid or unsafe")
        seen.add(str(kind))
        normalized.append(
            {
                "kind": str(kind),
                "method": "GET",
                "path": str(path),
                "expected_status": status,
            }
        )
    if sorted(row["expected_status"] for row in normalized) != sorted(
        required_statuses
    ):
        raise InputError(f"{label} http_probes do not match required HTTP statuses")
    return sorted(normalized, key=lambda row: row["kind"])


def normalize_resource_oracle(
    value: Any,
    combination: str,
    scenario_id: str,
    label: str,
) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise InputError(f"{label} resource_oracle must be an object")
    if combination == "devplus_devplus":
        expected_kind = (
            "v8_missing_resource_group"
            if scenario_id == "missing_resource_group_negative"
            else "v8_expected_no_resource_groups"
            if scenario_id in EXPECTED_NO_RESOURCE_GROUP_SCENARIOS
            else "v8_resource_groups"
        )
        if value != {"kind": expected_kind}:
            raise InputError(f"{label} must use the V8 resource-group oracle")
        return dict(value)
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
        raise InputError(f"{label} has an invalid explicit edge resource oracle")
    return dict(value)


def validate_api_oracle(
    document: dict[str, Any],
    scenarios: list[dict[str, Any]],
    *,
    catalog_sha256: str,
    mapping_sha256: str,
    canonical_entities_sha256: str | None,
    edge_receipt_sha256: str,
    fixture_receipt_sha256: str,
) -> dict[tuple[str, str], dict[str, Any]]:
    if (
        document.get("schema_version") != SCHEMA_VERSION
        or document.get("gate") != "G7"
        or document.get("status") != "PASS"
        or document.get("source_kind") != "reviewed_api_allowed_actions"
    ):
        raise InputError(
            "API oracle must be a PASS reviewed_api_allowed_actions G7 document"
        )
    validate_manifest_hash(document, "manifest_sha256", "API oracle")
    if (
        not positive_int(document.get("reviewed_by"))
        or not confirmed_time(document.get("reviewed_at"))
        or not nonempty(document.get("review_note"))
    ):
        raise InputError("API oracle review identity, time, and note are required")
    expected_inputs = {
        "scenario_catalog_sha256": catalog_sha256,
        "mapping_sha256": mapping_sha256,
        "canonical_entities_sha256": canonical_entities_sha256,
        "edge_receipt_sha256": edge_receipt_sha256,
        "fixture_receipt_sha256": fixture_receipt_sha256,
    }
    if document.get("input_sha256") != expected_inputs:
        raise InputError("API oracle is not bound to the frozen selector inputs")
    rows = document.get("cases")
    if not isinstance(rows, list):
        raise InputError("API oracle cases must be an array")
    expected_keys = {
        (str(scenario["id"]), str(combination))
        for scenario in scenarios
        for combination in scenario["required_combinations"]
    }
    indexed: dict[tuple[str, str], dict[str, Any]] = {}
    for index, row in enumerate(rows):
        if not isinstance(row, dict):
            raise InputError(f"API oracle cases[{index}] must be an object")
        key = (str(row.get("scenario_id", "")), str(row.get("combination", "")))
        if key not in expected_keys or key in indexed:
            raise InputError("API oracle contains an unexpected or duplicate case")
        normalized = {
            "scenario_id": key[0],
            "combination": key[1],
            "allowed_actions": normalize_allowed_actions(
                row.get("allowed_actions"),
                f"API oracle {key[0]}/{key[1]}",
            ),
            "http_probes": normalize_http_probes(
                row.get("http_probes"),
                requirements_for(
                    next(
                        scenario
                        for scenario in scenarios
                        if scenario["id"] == key[0]
                    ),
                    key[1],
                )["required_http_statuses"],
                f"API oracle {key[0]}/{key[1]}",
            ),
            "resource_oracle": normalize_resource_oracle(
                row.get("resource_oracle"),
                key[1],
                key[0],
                f"API oracle {key[0]}/{key[1]}",
            ),
        }
        normalized["api_oracle_case_sha256"] = canonical_sha256(normalized)
        indexed[key] = normalized
    if set(indexed) != expected_keys:
        raise InputError("API oracle does not cover every scenario/combination")
    return indexed


def validate_fixture_receipt(
    document: dict[str, Any],
    *,
    catalog_sha256: str,
    mapping_sha256: str,
    canonical_entities_sha256: str | None,
) -> dict[str, dict[str, Any]]:
    if (
        document.get("schema_version") != 2
        or document.get("gate") != "G7"
        or document.get("status") != "APPLIED_VERIFIED_PENDING_UI_AND_CLEANUP"
        or document.get("production_write_performed") is not False
        or document.get("clone_a_write_performed") is not False
        or document.get("template_task_mutated") is not False
    ):
        raise InputError("fixture receipt is not a verified Clone B-only v2 receipt")
    validate_manifest_hash(
        document,
        "receipt_payload_sha256",
        "fixture receipt",
    )
    inputs = document.get("input_sha256")
    if (
        not isinstance(inputs, dict)
        or inputs.get("scenarios") != catalog_sha256
        or inputs.get("mapping") != mapping_sha256
        or inputs.get("canonical") != canonical_entities_sha256
    ):
        raise InputError("fixture receipt is not bound to frozen selector inputs")
    if (
        document.get("nonfixture_integrity", {}).get("status") != "PASS"
        or document.get("template_integrity", {}).get("status") != "PASS"
        or document.get("row_verification", {}).get("status") != "PASS"
        or document.get("api_verification", {}).get("status") != "PASS"
    ):
        raise InputError("fixture receipt integrity or API verification is not PASS")
    scenario_ids = document.get("scenario_ids")
    scenarios = document.get("created_rows", {}).get("scenarios")
    fixture_plans = document.get("fixture_plans")
    if (
        not isinstance(scenario_ids, list)
        or len(scenario_ids) != len(set(scenario_ids))
        or not isinstance(scenarios, dict)
        or set(scenarios) != set(scenario_ids)
        or not isinstance(fixture_plans, dict)
        or set(fixture_plans) != set(scenario_ids)
    ):
        raise InputError("fixture receipt scenario coverage is invalid")
    normalized: dict[str, dict[str, Any]] = {}
    for scenario_id in scenario_ids:
        row = scenarios.get(scenario_id)
        if not isinstance(row, dict):
            raise InputError(f"fixture receipt {scenario_id} row is invalid")
        normalized[str(scenario_id)] = {
            "created": row,
            "fixture_plan": fixture_plans[scenario_id],
        }
    return normalized


def resolve_fixture_selection(
    *,
    scenario_id: str,
    fixture_plan: dict[str, Any],
    fixture_receipt_row: dict[str, Any],
    fixture_receipt_sha256: str,
    task_id: int,
    resource_ids: list[str],
    revision_ids: list[int],
    revision_facts: list[dict[str, Any]],
) -> tuple[int, list[str], list[int], list[dict[str, Any]], dict[str, Any]]:
    if fixture_receipt_row.get("fixture_plan") != fixture_plan:
        raise InputError(f"fixture receipt plan drift for {scenario_id}")
    created = fixture_receipt_row["created"]
    kind = fixture_plan["fixture_kind"]
    runtime_task_id = task_id
    runtime_resource_ids = list(resource_ids)
    runtime_revision_ids = list(revision_ids)
    runtime_revision_facts = list(revision_facts)
    if kind == "permission_denied_identity":
        if (
            created.get("template_task_id") != task_id
            or not positive_int(created.get("template_task_asset_id"))
            or not positive_int(created.get("fixture_user_id"))
            or not nonempty(created.get("session_id"))
        ):
            raise InputError(f"fixture receipt permission identity is invalid for {scenario_id}")
    else:
        if not positive_int(created.get("task_id")):
            raise InputError(f"fixture receipt task_id is invalid for {scenario_id}")
        runtime_task_id = int(created["task_id"])
        runtime_resource_ids = [
            f"task_asset_group:{int(value)}"
            for value in created.get("group_ids", [])
            if positive_int(value)
        ]
        runtime_revision_ids = [
            int(value)
            for value in created.get("revision_ids", [])
            if positive_int(value)
        ]
        if len(runtime_resource_ids) != len(created.get("group_ids", [])) or len(
            runtime_revision_ids
        ) != len(created.get("revision_ids", [])):
            raise InputError(f"fixture receipt IDs are invalid for {scenario_id}")
        runtime_revision_facts = [
            {
                "runtime_revision_id": revision_id,
                "fixture_receipt_sha256": fixture_receipt_sha256,
            }
            for revision_id in runtime_revision_ids
        ]
    runtime_resolution = {
        "source": "verified_clone_b_fixture_receipt_v2",
        "fixture_receipt_sha256": fixture_receipt_sha256,
        "fixture_kind": kind,
        "created": created,
    }
    return (
        runtime_task_id,
        runtime_resource_ids,
        runtime_revision_ids,
        runtime_revision_facts,
        runtime_resolution,
    )


def validate_requirement_block(value: Any, label: str) -> dict[str, Any]:
    if not isinstance(value, dict) or set(value) != set(REQUIREMENT_FIELDS):
        raise InputError(f"{label} must declare the exact requirement fields")
    for field in (
        "requires_task_id",
        "requires_revision_ids",
        "requires_history_drawer",
    ):
        if not isinstance(value.get(field), bool):
            raise InputError(f"{label} has invalid {field}")
    if value["requires_history_drawer"] and not value["requires_revision_ids"]:
        raise InputError(f"{label} cannot require history without revision IDs")
    statuses = value.get("required_http_statuses")
    assertions = value.get("required_assertions")
    if (
        not isinstance(statuses, list)
        or len(statuses) != len(set(statuses))
        or any(
            not isinstance(status, int)
            or isinstance(status, bool)
            or status < 100
            or status > 599
            for status in statuses
        )
    ):
        raise InputError(f"{label} has invalid HTTP statuses")
    if (
        not isinstance(assertions, list)
        or not assertions
        or len(assertions) != len(set(assertions))
        or any(not nonempty(assertion) for assertion in assertions)
    ):
        raise InputError(f"{label} has invalid assertions")
    return value


def requirements_for(
    scenario: dict[str, Any],
    combination: str,
) -> dict[str, Any]:
    conditional = scenario.get("requirements_by_combination")
    if isinstance(conditional, dict) and combination in conditional:
        return conditional[combination]
    return {field: scenario[field] for field in REQUIREMENT_FIELDS}


def expected_assertions_for(
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


def validate_catalog(catalog: dict[str, Any]) -> list[dict[str, Any]]:
    if catalog.get("schema_version") != 1 or catalog.get("gate") != "G7":
        raise InputError("scenario catalog must be schema_version=1 and gate=G7")
    combinations = catalog.get("combinations")
    if (
        not isinstance(combinations, list)
        or len(combinations) != len(EXPECTED_COMBINATIONS)
        or len(combinations) != len(set(combinations))
        or set(combinations) != EXPECTED_COMBINATIONS
    ):
        raise InputError("scenario catalog must declare the exact four combinations")
    viewports = catalog.get("viewports")
    if (
        not isinstance(viewports, list)
        or len(viewports) != len(EXPECTED_VIEWPORTS)
        or len(viewports) != len(set(viewports))
        or set(viewports) != EXPECTED_VIEWPORTS
    ):
        raise InputError("scenario catalog must declare desktop and mobile")
    scenarios = catalog.get("scenarios")
    if (
        not isinstance(scenarios, list)
        or len(scenarios) != EXPECTED_SCENARIO_COUNT
    ):
        raise InputError(
            f"scenario catalog must contain the exact "
            f"{EXPECTED_SCENARIO_COUNT} G7 scenarios"
        )
    seen: set[str] = set()
    for index, scenario in enumerate(scenarios):
        if not isinstance(scenario, dict):
            raise InputError(f"scenario[{index}] must be an object")
        scenario_id = scenario.get("id")
        if not nonempty(scenario_id) or scenario_id in seen:
            raise InputError(f"scenario[{index}] has an invalid or duplicate id")
        seen.add(str(scenario_id))
        combinations = scenario.get("required_combinations")
        viewports = scenario.get("required_viewports")
        if (
            not isinstance(combinations, list)
            or not combinations
            or len(combinations) != len(set(combinations))
            or not set(combinations).issubset(EXPECTED_COMBINATIONS)
        ):
            raise InputError(f"scenario {scenario_id} has invalid combinations")
        if (
            not isinstance(viewports, list)
            or not viewports
            or len(viewports) != len(set(viewports))
            or not set(viewports).issubset(EXPECTED_VIEWPORTS)
        ):
            raise InputError(f"scenario {scenario_id} has invalid viewports")
        base = validate_requirement_block(
            {field: scenario.get(field) for field in REQUIREMENT_FIELDS},
            f"scenario {scenario_id}",
        )
        conditional = scenario.get("requirements_by_combination")
        if conditional is not None:
            if (
                not isinstance(conditional, dict)
                or set(conditional) != set(combinations)
            ):
                raise InputError(
                    f"scenario {scenario_id} conditional requirements must "
                    "declare every required combination"
                )
            for combination in combinations:
                requirements = validate_requirement_block(
                    conditional[combination],
                    f"scenario {scenario_id}/{combination}",
                )
                if requirements["requires_task_id"] != base["requires_task_id"]:
                    raise InputError(
                        f"scenario {scenario_id}/{combination} cannot change task requirement"
                    )
                if set(requirements["required_http_statuses"]) != set(
                    base["required_http_statuses"]
                ):
                    raise InputError(
                        f"scenario {scenario_id}/{combination} cannot weaken HTTP statuses"
                    )
                if set(requirements["required_assertions"]) != expected_assertions_for(
                    str(scenario_id),
                    combination,
                    base["required_assertions"],
                ):
                    raise InputError(
                        f"scenario {scenario_id}/{combination} cannot weaken assertions"
                    )
    if set(NEGATIVE_FIXTURE_SCENARIOS) - seen:
        raise InputError("scenario catalog is missing required negative fixture scenarios")
    return scenarios


def review_row_confirmed(row: dict[str, Any]) -> bool:
    return (
        row.get("confidence") == "confirmed_auto"
        and not row.get("blockers")
        and positive_int(row.get("confirmed_by"))
        and confirmed_time(row.get("confirmed_at"))
        and nonempty(row.get("confirmation_note"))
        and bool(SHA256_RE.fullmatch(str(row.get("manifest_row_hash", ""))))
    )


def valid_source_bundle(value: Any) -> bool:
    if not isinstance(value, dict):
        return False
    members = value.get("members")
    return (
        positive_int(value.get("task_asset_id"))
        and value.get("format") == "zip"
        and bool(SHA256_RE.fullmatch(str(value.get("bundle_sha256", ""))))
        and bool(SHA256_RE.fullmatch(str(value.get("manifest_sha256", ""))))
        and isinstance(members, list)
        and len(members) >= 2
        and all(
            isinstance(member, dict)
            and positive_int(member.get("task_asset_id"))
            and bool(SHA256_RE.fullmatch(str(member.get("sha256", ""))))
            and member.get("confirmed") is True
            for member in members
        )
        and len({int(member["task_asset_id"]) for member in members}) == len(members)
        and positive_int(value.get("confirmed_by"))
        and confirmed_time(value.get("confirmed_at"))
        and nonempty(value.get("confirmation_note"))
    )


def mapping_review_failures(mapping: dict[str, Any]) -> list[str]:
    failures: list[str] = []
    collections = (
        "planning_tasks",
        "task_state_decisions",
        "asset_recoveries",
        "organization_mappings",
        "access_decisions",
    )
    resources = mapping.get("resources")
    if mapping.get("version") != 2 or not isinstance(resources, list):
        raise InputError("mapping must be version 2 with resources[]")
    for group_index, group in enumerate(resources):
        if not isinstance(group, dict) or not isinstance(group.get("history"), list):
            raise InputError(f"resources[{group_index}] must contain history[]")
        for revision_index, revision in enumerate(group["history"]):
            if not isinstance(revision, dict):
                raise InputError(
                    f"resources[{group_index}].history[{revision_index}] must be an object"
                )
            if revision.get("source_bundle") is not None and not valid_source_bundle(
                revision["source_bundle"]
            ):
                raise InputError(
                    f"resources[{group_index}].history[{revision_index}].source_bundle "
                    "is not a confirmed deterministic ZIP contract"
                )
            if not review_row_confirmed(revision):
                failures.append(f"resources[{group_index}].history[{revision_index}]")
    for name in collections:
        rows = mapping.get(name, [])
        if not isinstance(rows, list):
            raise InputError(f"mapping {name} must be an array")
        for index, row in enumerate(rows):
            if not isinstance(row, dict):
                raise InputError(f"{name}[{index}] must be an object")
            if not review_row_confirmed(row):
                failures.append(f"{name}[{index}]")
    return failures


def index_canonical(
    document: dict[str, Any],
    mapping_sha256: str,
    final_mode: bool,
) -> dict[tuple[str, str], dict[str, Any]]:
    if document.get("schema_version") != 1 or not isinstance(document.get("entities"), list):
        raise InputError("canonical entities must be a schema_version=1 document")
    bound_hash = document.get("input_sha256", {}).get("mapping_sha256")
    if bound_hash != mapping_sha256:
        raise InputError("canonical entities are not bound to the exact mapping bytes")
    indexed: dict[tuple[str, str], dict[str, Any]] = {}
    for index, entity in enumerate(document["entities"]):
        if not isinstance(entity, dict):
            raise InputError(f"canonical entities[{index}] must be an object")
        gate, key = entity.get("gate_name"), entity.get("entity_key")
        if not nonempty(gate) or not nonempty(key):
            raise InputError(f"canonical entities[{index}] has no gate/entity key")
        natural = (str(gate), str(key))
        if natural in indexed:
            raise InputError(f"duplicate canonical entity {gate}/{key}")
        expected_state_by_gate = {
            "G01": "matched",
            "G02": "matched",
            "G03": "matched",
            "G04": "matched",
            "G05": "matched",
            "G06": "verified",
            "G07": "matched",
            "G08": "matched",
            "G09": "approved",
            "G10": "confirmed",
        }
        if final_mode and (
            entity.get("review_state") != "pass"
            or entity.get("expected_state") != expected_state_by_gate.get(str(gate))
        ):
            raise InputError(f"canonical entity {gate}/{key} is not a reviewed PASS")
        indexed[natural] = entity
    for gate in ("G01", "G02", "G03"):
        if not any(key[0] == gate for key in indexed):
            raise InputError(f"canonical entities are missing {gate}")
    return indexed


def entity_evidence(entity: dict[str, Any]) -> dict[str, Any]:
    return {
        "gate_name": entity["gate_name"],
        "entity_key": entity["entity_key"],
        "entity_sha256": canonical_sha256(entity),
    }


def policy_set(group: dict[str, Any]) -> set[str]:
    return {
        str(policy)
        for revision in group.get("history", [])
        for policy in revision.get("review_policy_ids", [])
    }


def revision_locator(group: dict[str, Any], revision: dict[str, Any]) -> str:
    return (
        f"revision:{int(group['task_id'])}:{group['scope_kind']}:"
        f"{int(group['scope_ref_id'])}:{int(revision['revision_no'])}"
    )


def group_locator(group: dict[str, Any]) -> str:
    return (
        f"group:{int(group['task_id'])}:{group['scope_kind']}:"
        f"{int(group['scope_ref_id'])}"
    )


def compact_event_text(entity: dict[str, Any]) -> str:
    text = json.dumps(entity.get("components", []), ensure_ascii=False).lower()
    return re.sub(r"[\s_\-]+", "", text)


def event_matches(entity: dict[str, Any], aliases: Iterable[str]) -> bool:
    text = compact_event_text(entity)
    return any(re.sub(r"[\s_\-]+", "", alias.lower()) in text for alias in aliases)


class Facts:
    def __init__(
        self,
        mapping: dict[str, Any],
        canonical: dict[tuple[str, str], dict[str, Any]],
        canonical_sha256: str | None = None,
    ) -> None:
        self.mapping = mapping
        self.canonical = canonical
        self.canonical_sha256 = canonical_sha256
        self.resources: list[dict[str, Any]] = list(mapping["resources"])
        self.by_task: dict[int, list[dict[str, Any]]] = defaultdict(list)
        self.planning_by_task: dict[int, dict[str, Any]] = {}
        self.recovery_by_task: dict[int, list[dict[str, Any]]] = defaultdict(list)
        self.task_state_by_task: dict[int, list[dict[str, Any]]] = defaultdict(list)
        self.access_decisions = list(mapping.get("access_decisions", []))
        self.tasks: dict[int, dict[str, Any]] = {}
        self.events_by_task: dict[int, list[dict[str, Any]]] = defaultdict(list)
        self.retouch_entities_by_task: dict[int, list[dict[str, Any]]] = defaultdict(list)
        self.revision_id_by_locator: dict[str, int] = {}

        next_revision_id = 1
        seen_scopes: set[tuple[int, str, int]] = set()
        for group in self.resources:
            task_id = int(group["task_id"])
            scope = (task_id, str(group["scope_kind"]), int(group["scope_ref_id"]))
            if scope in seen_scopes:
                raise InputError(f"duplicate mapping resource scope {scope}")
            seen_scopes.add(scope)
            self.by_task[task_id].append(group)
            for revision in group["history"]:
                locator = revision_locator(group, revision)
                if locator in self.revision_id_by_locator:
                    raise InputError(f"duplicate revision locator {locator}")
                self.revision_id_by_locator[locator] = next_revision_id
                next_revision_id += 1

        for planning in mapping.get("planning_tasks", []):
            task_id = int(planning["task_id"])
            if task_id in self.planning_by_task:
                raise InputError(f"duplicate planning task {task_id}")
            self.planning_by_task[task_id] = planning
        for recovery in mapping.get("asset_recoveries", []):
            self.recovery_by_task[int(recovery["task_id"])].append(recovery)
        for decision in mapping.get("task_state_decisions", []):
            self.task_state_by_task[int(decision["task_id"])].append(decision)

        for (gate, key), entity in canonical.items():
            components = entity.get("components")
            if not isinstance(components, list):
                raise InputError(f"canonical entity {gate}/{key} has invalid components")
            if gate == "G01" and key.startswith("task:"):
                task_id = (
                    canonical_positive_int(components[0])
                    if len(components) >= 5
                    else None
                )
                if task_id is None:
                    raise InputError(f"canonical task entity {key} is malformed")
                self.tasks[task_id] = {
                    "task_id": task_id,
                    "task_type": str(components[1]),
                    "task_status": str(components[2]),
                    "current_handler_id": components[3],
                    "workflow_revision": components[4],
                    "entity": entity,
                }
            elif gate == "G07" and key.startswith("task-event:"):
                task_id = (
                    canonical_positive_int(components[1])
                    if len(components) >= 2
                    else None
                )
                if task_id is None:
                    raise InputError(f"canonical task event {key} is malformed")
                self.events_by_task[task_id].append(entity)
            elif gate == "G08" and key.startswith("retouch-requirement:"):
                task_id = (
                    canonical_positive_int(components[0])
                    if len(components) >= 2
                    else None
                )
                if task_id is None:
                    raise InputError(
                        f"canonical retouch requirement {key} is malformed"
                    )
                self.retouch_entities_by_task[task_id].append(entity)

        # Correlate every mapped revision with a canonical expected entity.
        if canonical:
            for group in self.resources:
                group_key = group_locator(group)
                if ("G02", group_key) not in canonical:
                    raise InputError(f"canonical entities are missing {group_key}")
                for revision in group["history"]:
                    locator = revision_locator(group, revision)
                    entity = canonical.get(("G03", locator))
                    if entity is None:
                        raise InputError(f"canonical entities are missing {locator}")
                    components = entity.get("components", [])
                    if (
                        len(components) < 8
                        or str(components[4]) != str(revision.get("status"))
                        or str(components[7]) != str(revision.get("source_stage"))
                    ):
                        raise InputError(f"canonical entity {locator} contradicts mapping")

        for groups in self.by_task.values():
            groups.sort(key=lambda item: (str(item["scope_kind"]), int(item["scope_ref_id"])))
        for events in self.events_by_task.values():
            events.sort(key=lambda item: str(item["entity_key"]))

    def task_type(self, task_id: int) -> str:
        return str(self.tasks.get(task_id, {}).get("task_type", "")).lower()

    def task_status(self, task_id: int) -> str:
        return str(self.tasks.get(task_id, {}).get("task_status", ""))

    def task_event(
        self,
        task_id: int,
        aliases: Iterable[str],
    ) -> dict[str, Any] | None:
        return next(
            (
                entity
                for entity in self.events_by_task.get(task_id, [])
                if event_matches(entity, aliases)
            ),
            None,
        )

    def task_event_exact(
        self,
        task_id: int,
        event_type: str,
    ) -> dict[str, Any] | None:
        return next(
            (
                entity
                for entity in self.events_by_task.get(task_id, [])
                if len(entity.get("components", [])) >= 4
                and entity["components"][3] == event_type
            ),
            None,
        )

    def task_state(
        self,
        task_id: int,
        from_status: str,
    ) -> dict[str, Any] | None:
        return next(
            (
                row
                for row in self.task_state_by_task.get(task_id, [])
                if row.get("from_status") == from_status
            ),
            None,
        )


def has_policy(group: dict[str, Any], *names: str) -> bool:
    return bool(policy_set(group).intersection(names))


def group_is_design(facts: Facts, group: dict[str, Any]) -> bool:
    task_type = facts.task_type(int(group["task_id"]))
    return (
        group["scope_kind"] != "retouch_requirement"
        and "retouch" not in task_type
        and "planning" not in task_type
        and "purchase" not in task_type
        and "custom" not in task_type
    )


def finalized_revisions(group: dict[str, Any]) -> list[dict[str, Any]]:
    return [
        revision
        for revision in group["history"]
        if revision.get("status") in {"finalized", "superseded"}
    ]


def select_best(
    candidates: Iterable[dict[str, Any]],
    quality: Callable[[dict[str, Any]], tuple[Any, ...]] | None = None,
) -> dict[str, Any] | None:
    rows = list(candidates)
    if not rows:
        return None
    quality = quality or (
        lambda group: (
            int(group["task_id"]),
            str(group["scope_kind"]),
            int(group["scope_ref_id"]),
        )
    )
    return min(rows, key=quality)


def event_backed_group(
    facts: Facts,
    aliases: Iterable[str],
    from_status: str | None = None,
    require_reopen: bool = False,
) -> tuple[dict[str, Any], dict[str, Any] | None, dict[str, Any] | None] | None:
    candidates: list[tuple[dict[str, Any], dict[str, Any] | None, dict[str, Any] | None]] = []
    for group in facts.resources:
        if not group["history"]:
            continue
        task_id = int(group["task_id"])
        event = facts.task_event(task_id, aliases)
        decision = facts.task_state(task_id, from_status) if from_status else None
        if event is None and decision is None:
            continue
        if require_reopen and not any(
            revision.get("source_stage") == "reopen" for revision in group["history"]
        ):
            continue
        candidates.append((group, event, decision))
    return min(
        candidates,
        key=lambda item: (
            int(item[0]["task_id"]),
            str(item[0]["scope_kind"]),
            int(item[0]["scope_ref_id"]),
        ),
        default=None,
    )


def base_resource(facts: Facts) -> dict[str, Any] | None:
    return select_best(
        (
            group
            for group in facts.resources
            if group["history"]
            and group.get("working_revision_no") is not None
            and group.get("finalized_revision_no") is not None
        ),
        quality=lambda group: (
            -len(group["history"]),
            int(group["task_id"]),
            str(group["scope_kind"]),
            int(group["scope_ref_id"]),
        ),
    )


def choose_scenario(
    scenario_id: str,
    facts: Facts,
) -> tuple[list[dict[str, Any]], list[dict[str, Any]], str, dict[str, Any] | None]:
    """Return resources, extra evidence, rationale, and optional fixture plan."""

    selected: list[dict[str, Any]] = []
    extra: list[dict[str, Any]] = []
    rationale = ""
    fixture: dict[str, Any] | None = None

    if scenario_id in {
        "baseline_four_edge_readonly",
        "backend_first_compatibility",
        "frontend_rollback_compatibility",
    }:
        group = base_resource(facts)
        if group:
            selected = [group]
            rationale = (
                "blocker-free resource with both current pointers; maximizes revision-chain "
                "length, then uses the stable scope key"
            )
    elif scenario_id == "design_first_submit_audit":
        group = select_best(
            group
            for group in facts.resources
            if group_is_design(facts, group)
            and len(group["history"]) == 1
            and group["history"][0].get("status") == "finalized"
            and group["history"][0].get("source_stage") in {"design", "audit"}
            and has_policy(group, "explicit_event_replay")
            and len(group["history"][0].get("evidence_event_ids", [])) >= 2
        )
        if group:
            selected, rationale = [group], "single explicit-event design revision finalized after submit/audit evidence"
    elif scenario_id == "audit_without_source_replacement":
        group = select_best(
            group
            for group in facts.resources
            if group_is_design(facts, group)
            and any(
                revision.get("status") == "finalized"
                and revision.get("source_stage") == "design"
                and not revision.get("source_bundle")
                and revision.get("source_alias_from_task_asset_id") is not None
                for revision in group["history"]
            )
            and has_policy(group, "explicit_event_replay", "delivery_source_alias")
            and not has_policy(group, "legacy_audit_stage_final_snapshot_v1")
        )
        if group:
            selected, rationale = [group], "finalized design-stage snapshot retains its immutable source alias without an audit-stage replacement"
    elif scenario_id == "audit_with_source_replacement":
        group = select_best(
            group
            for group in facts.resources
            if group_is_design(facts, group)
            and finalized_revisions(group)
            and (
                has_policy(group, "legacy_audit_stage_final_snapshot_v1")
                or any(revision.get("source_stage") == "audit" for revision in group["history"])
            )
        )
        if group:
            selected, rationale = [group], "reviewed audit-stage final snapshot proves an audit replacement boundary"
    elif scenario_id == "audit_reject_redesign_resubmit":
        group = select_best(
            group
            for group in facts.resources
            if has_policy(group, "rejected_history")
            and any(revision.get("status") == "rejected" for revision in group["history"])
            and any(
                revision.get("revision_no", 0) > min(
                    int(row["revision_no"])
                    for row in group["history"]
                    if row.get("status") == "rejected"
                )
                and revision.get("status") in {"submitted", "finalized", "superseded"}
                for revision in group["history"]
            )
        )
        if group:
            selected, rationale = [group], "immutable rejected revision is followed by a distinct resubmitted/finalized revision"
    elif scenario_id == "customization_submit_audit":
        group = select_best(
            group
            for group in facts.resources
            if finalized_revisions(group)
            and has_policy(group, "explicit_event_replay")
            and facts.task_event_exact(
                int(group["task_id"]),
                "task.customization.reviewed",
            )
        )
        if group:
            event = facts.task_event_exact(
                int(group["task_id"]),
                "task.customization.reviewed",
            )
            selected = [group]
            extra = [entity_evidence(event)] if event else []
            rationale = "immutable customization-reviewed event and explicit-event history identify a finalized customization audit snapshot"
    elif scenario_id == "retired_warehouse_receive_history":
        match = event_backed_group(
            facts,
            ("PendingWarehouseReceive", "warehouse_receive", "warehouse receive"),
            "PendingWarehouseReceive",
        )
        if match:
            group, event, decision = match
            selected = [group]
            if event:
                extra.append(entity_evidence(event))
            if decision:
                extra.append({"mapping_row_sha256": decision.get("manifest_row_hash")})
            rationale = "canonical event or reviewed state decision proves the retired warehouse-receive boundary"
    elif scenario_id == "retired_production_transfer_history":
        match = event_backed_group(
            facts,
            ("PendingProductionTransfer", "production_transfer", "production transfer"),
            "PendingProductionTransfer",
        )
        if match:
            group, event, decision = match
            selected = [group]
            if event:
                extra.append(entity_evidence(event))
            if decision:
                extra.append({"mapping_row_sha256": decision.get("manifest_row_hash")})
            rationale = "canonical event or reviewed state decision proves the retired production-transfer boundary"
    elif scenario_id == "retired_pending_close_history":
        match = event_backed_group(
            facts,
            ("PendingClose", "pending_close", "pending close"),
            "PendingClose",
        )
        if match:
            group, event, decision = match
            selected = [group]
            if event:
                extra.append(entity_evidence(event))
            if decision:
                extra.append({"mapping_row_sha256": decision.get("manifest_row_hash")})
            rationale = "canonical event or reviewed state decision proves the retired pending-close boundary"
    elif scenario_id == "warehouse_rejection_reopen_history":
        match = event_backed_group(
            facts,
            ("RejectedByWarehouse", "warehouse_rejection", "warehouse rejection"),
            "RejectedByWarehouse",
            require_reopen=True,
        )
        if match:
            group, event, decision = match
            selected = [group]
            if event:
                extra.append(entity_evidence(event))
            if decision:
                extra.append({"mapping_row_sha256": decision.get("manifest_row_hash")})
            rationale = "warehouse rejection evidence is paired with a distinct source_stage=reopen revision"
    elif scenario_id == "multi_round_design_audit":
        group = select_best(
            (group for group in facts.resources if len(group["history"]) >= 2),
            quality=lambda group: (
                -len(group["history"]),
                int(group["task_id"]),
                str(group["scope_kind"]),
                int(group["scope_ref_id"]),
            ),
        )
        if group:
            selected, rationale = [group], "longest contiguous reviewed revision chain, then stable scope key"
    elif scenario_id == "post_close_replacement":
        group = select_best(
            group
            for group in facts.resources
            if has_policy(group, "legacy_post_close_replacement_v1", "reopen")
            and any(
                revision.get("source_stage") == "reopen"
                and revision.get("status") in {"finalized", "superseded"}
                for revision in group["history"]
            )
        )
        if group:
            selected, rationale = [group], "reviewed post-close policy and finalized/superseded reopen snapshot are both present"
    elif scenario_id == "audit_supplement_upload":
        candidates = []
        for group in facts.resources:
            task_id = int(group["task_id"])
            event = facts.task_event(
                task_id,
                ("audit_supplement", "supplement_upload", "audit supplement", "补传"),
            )
            if (
                event
                and any(revision.get("source_stage") in {"audit", "reopen"} for revision in group["history"])
            ):
                candidates.append((group, event))
        if candidates:
            group, event = min(
                candidates,
                key=lambda item: (
                    int(item[0]["task_id"]),
                    str(item[0]["scope_kind"]),
                    int(item[0]["scope_ref_id"]),
                ),
            )
            selected, extra = [group], [entity_evidence(event)]
            rationale = "canonical supplement-upload event and audit/reopen revision stage prove the boundary"
    elif scenario_id == "multi_source_zip_bundle":
        group = select_best(
            group
            for group in facts.resources
            if any(valid_source_bundle(revision.get("source_bundle")) for revision in group["history"])
        )
        if group:
            selected, rationale = [group], "reviewed revision carries a hash-bound source_bundle with frozen ordered members"
    elif scenario_id == "single_sku_scope":
        group = select_best(
            group
            for group in facts.resources
            if group["scope_kind"] == "sku"
            and group["history"]
            and sum(
                1
                for item in facts.by_task[int(group["task_id"])]
                if item["scope_kind"] == "sku"
            )
            == 1
        )
        if group:
            selected, rationale = [group], "task has exactly one reviewed SKU-scoped resource"
    elif scenario_id in {"multi_sku_atomic", "task_scope_asset_split"}:
        candidates: list[list[dict[str, Any]]] = []
        for task_id, groups in facts.by_task.items():
            sku_groups = [group for group in groups if group["scope_kind"] == "sku" and group["history"]]
            shared_evidence = set.intersection(
                *[
                    {
                        str(event_id)
                        for revision in group["history"]
                        for event_id in revision.get("evidence_event_ids", [])
                    }
                    for group in sku_groups
                ]
            ) if sku_groups else set()
            if (
                len(sku_groups) >= 2
                and all(has_policy(group, "legacy_multi_sku_atomic_batch_submit_v1") for group in sku_groups)
                and (
                    scenario_id == "multi_sku_atomic"
                    or bool(shared_evidence)
                )
            ):
                candidates.append(sku_groups)
        if candidates:
            selected = min(
                candidates,
                key=lambda groups: (
                    int(groups[0]["task_id"]),
                    tuple(int(group["scope_ref_id"]) for group in groups),
                ),
            )
            rationale = (
                "all SKU scopes are independently mapped and share the reviewed atomic-batch policy"
                if scenario_id == "multi_sku_atomic"
                else "multiple independently scoped SKU snapshots share one explicit task-level event, proving deterministic scope splitting"
            )
    elif scenario_id in {"retouch_single_requirement", "retouch_multiple_requirements"}:
        wanted = 1 if scenario_id == "retouch_single_requirement" else 2
        candidates = []
        for task_id, groups in facts.by_task.items():
            retouch = [
                group
                for group in groups
                if group["scope_kind"] == "retouch_requirement"
                and group["history"]
                and has_policy(group, "retouch_source_optional")
            ]
            if (wanted == 1 and len(retouch) == 1) or (wanted == 2 and len(retouch) >= 2):
                if len(facts.retouch_entities_by_task.get(task_id, [])) >= len(retouch):
                    candidates.append(retouch)
        if candidates:
            selected = min(
                candidates,
                key=lambda groups: (
                    int(groups[0]["task_id"]),
                    tuple(int(group["scope_ref_id"]) for group in groups),
                ),
            )
            rationale = (
                "one canonical retouch requirement has a reviewed optional-source revision"
                if wanted == 1
                else "multiple canonical retouch requirements have isolated reviewed resource scopes"
            )
            extra.extend(
                entity_evidence(entity)
                for entity in facts.retouch_entities_by_task[int(selected[0]["task_id"])]
            )
    elif scenario_id == "retouch_reopen_task1264":
        group = next(
            (
                candidate
                for candidate in facts.by_task.get(RETOUCH_REOPEN_TASK_ID, [])
                if candidate.get("scope_kind") == "retouch_requirement"
                and int(candidate.get("scope_ref_id", -1))
                == RETOUCH_REOPEN_SCOPE_REF_ID
                and [
                    (
                        int(revision.get("revision_no", 0)),
                        revision.get("status"),
                        revision.get("source_stage"),
                    )
                    for revision in candidate.get("history", [])
                ]
                == [
                    (1, "superseded", "retouch"),
                    (2, "finalized", "reopen"),
                ]
                and all(
                    all(
                        has_row_policy(revision, policy)
                        for policy in (
                            "explicit_event_replay",
                            "retouch_source_optional",
                            "legacy_retouch_terminal_submit_v1",
                        )
                    )
                    for revision in candidate["history"]
                )
                and has_row_policy(candidate["history"][1], "reopen")
                and candidate["history"][0].get("final_task_asset_ids") == [5501]
                and candidate["history"][1].get("final_task_asset_ids") == [6316]
                and all(
                    revision.get("reference_file_ref_ids") == [1312]
                    for revision in candidate["history"]
                )
                and [
                    facts.revision_id_by_locator[
                        revision_locator(candidate, revision)
                    ]
                    for revision in candidate["history"]
                ]
                == RETOUCH_REOPEN_REVISION_IDS
                and any(
                    entity.get("entity_key")
                    == "retouch-requirement:1264:45"
                    for entity in facts.retouch_entities_by_task[
                        RETOUCH_REOPEN_TASK_ID
                    ]
                )
            ),
            None,
        )
        if group:
            selected = [group]
            rationale = (
                "frozen task 1264 retouch_requirement scope 45 resolves to "
                "Clone B revisions 635 superseded/retouch and 636 finalized/reopen"
            )
            extra.extend(
                entity_evidence(entity)
                for entity in facts.retouch_entities_by_task[
                    RETOUCH_REOPEN_TASK_ID
                ]
                if entity.get("entity_key")
                == "retouch-requirement:1264:45"
            )
    elif scenario_id == "purchase_to_sku_planning":
        candidates = []
        for task_id, planning in facts.planning_by_task.items():
            task = facts.tasks.get(task_id)
            if (
                task
                and task["task_type"] == "sku_planning"
                and has_row_policy(planning, "legacy_purchase_to_sku_planning_v1")
                and planning.get("items")
            ):
                candidates.append((task_id, planning))
        if candidates:
            task_id, planning = min(candidates, key=lambda item: item[0])
            selected = facts.by_task.get(task_id, [])
            extra = [
                entity_evidence(facts.tasks[task_id]["entity"]),
                {"mapping_row_sha256": planning.get("manifest_row_hash")},
            ]
            rationale = "canonical task type is sku_planning and reviewed legacy purchase policy has concrete SKU items"
            if not selected:
                # Planning intentionally has no design group; retain a task-only selection.
                selected = [{"task_id": task_id, "scope_kind": "planning", "scope_ref_id": 0, "history": []}]
    elif scenario_id in {"cancelled_readonly", "archived_readonly"}:
        expected = "Cancelled" if scenario_id == "cancelled_readonly" else "Archived"
        task_ids = sorted(
            task_id
            for task_id, task in facts.tasks.items()
            if task["task_status"] == expected
        )
        if task_ids:
            task_id = task_ids[0]
            selected = facts.by_task.get(task_id, []) or [
                {"task_id": task_id, "scope_kind": "task_terminal", "scope_ref_id": 0, "history": []}
            ]
            extra = [entity_evidence(facts.tasks[task_id]["entity"])]
            rationale = f"canonical G01 task status is exactly {expected}"
        elif scenario_id == "archived_readonly":
            archived_population = sum(
                task["task_status"] == "Archived"
                for task in facts.tasks.values()
            )
            if archived_population != 0:
                raise InputError(
                    "Archived fixture fallback is allowed only when canonical "
                    "G01 Archived population is exactly zero"
                )
            group = base_resource(facts)
            if group:
                selected = [
                    {
                        "task_id": int(group["task_id"]),
                        "scope_kind": "task_terminal",
                        "scope_ref_id": 0,
                        "history": [],
                    }
                ]
                fixture = fixture_plan(
                    scenario_id,
                    "archived_terminal",
                    int(group["task_id"]),
                    group_locator(group),
                    fixture_class="positive_contract",
                    canonical_population=archived_population,
                    canonical_entities_sha256=facts.canonical_sha256,
                )
                rationale = (
                    "canonical G01 contains zero Archived tasks; an isolated "
                    "Clone B positive-contract fixture verifies the runtime/UI "
                    "terminal-state contract without claiming historical migration coverage"
                )
    elif scenario_id == "historical_asset_unavailable_410":
        candidates = []
        for task_id, recoveries in facts.recovery_by_task.items():
            for recovery in recoveries:
                if recovery.get("strategy") != "historical_unavailable_tombstone_v1":
                    continue
                missing_id = int(recovery["missing_task_asset_id"])
                for group in facts.by_task.get(task_id, []):
                    if any(
                        missing_id in [int(value) for value in revision.get("final_task_asset_ids", [])]
                        or revision.get("source_task_asset_id") == missing_id
                        or revision.get("source_alias_from_task_asset_id") == missing_id
                        for revision in group["history"]
                    ):
                        candidates.append((group, recovery))
        if candidates:
            group, recovery = min(
                candidates,
                key=lambda item: (
                    int(item[0]["task_id"]),
                    str(item[0]["scope_kind"]),
                    int(item[0]["scope_ref_id"]),
                ),
            )
            selected = [group]
            extra = [{"mapping_row_sha256": recovery.get("manifest_row_hash")}]
            rationale = "reviewed tombstone strategy and exact historical task_asset membership select the 410 case"
    elif scenario_id in NEGATIVE_FIXTURE_SCENARIOS:
        group = base_resource(facts)
        if group:
            selected = [group]
            fixture = fixture_plan(
                scenario_id,
                NEGATIVE_FIXTURE_SCENARIOS[scenario_id],
                int(group["task_id"]),
                group_locator(group),
            )
            rationale = "valid reviewed resource is a template for an isolated Clone B fixture; production anomalies are not used"
            if scenario_id == "unauthorized_asset_403":
                access = next(
                    (
                        row
                        for row in facts.access_decisions
                        if row.get("action") == "no_new_grant"
                        and has_row_policy(row, "retired_warehouse_no_new_grant_v1")
                    ),
                    None,
                )
                if access is None:
                    return [], [], "", None
                extra.append({"mapping_row_sha256": access.get("manifest_row_hash")})
    else:
        raise InputError(f"no selector is defined for scenario {scenario_id}")

    return selected, extra, rationale, fixture


def has_row_policy(row: dict[str, Any], name: str) -> bool:
    return name in set(row.get("review_policy_ids", []))


def fixture_plan(
    scenario_id: str,
    fixture_kind: str,
    template_task_id: int,
    template_resource_key: str,
    *,
    fixture_class: str = "negative_assertion",
    canonical_population: int | None = None,
    canonical_entities_sha256: str | None = None,
) -> dict[str, Any]:
    operations = {
        "permission_denied_identity": [
            "create_clone_b_fixture_identity_with_zero_assignments",
            "capture_fixture_user_id",
            "request_template_asset_with_fixture_identity",
        ],
        "missing_resource_group": [
            "clone_template_task_into_fixture_namespace",
            "omit_all_fixture_resource_groups",
            "capture_fixture_task_id",
        ],
        "missing_current_pointer": [
            "clone_template_task_and_resource_into_fixture_namespace",
            "retain_fixture_revision_history",
            "clear_fixture_working_and_finalized_pointers",
            "capture_fixture_task_group_revision_ids",
        ],
        "wrong_scope_asset": [
            "clone_template_task_with_two_fixture_scopes",
            "attach_fixture_asset_to_nonmatching_fixture_scope",
            "capture_fixture_task_group_revision_ids",
        ],
        "archived_terminal": [
            "clone_template_task_into_fixture_namespace",
            "set_only_fixture_task_status_to_archived",
            "close_only_fixture_task_modules",
            "create_empty_terminal_task_scope_group",
            "capture_fixture_task_group_and_module_ids",
        ],
    }[fixture_kind]
    plan = {
        "schema_version": 1,
        "fixture_kind": fixture_kind,
        "fixture_class": fixture_class,
        "environment": "isolated_clone_b_only",
        "template_task_id": template_task_id,
        "template_resource_key": template_resource_key,
        "operations": operations,
        "runtime_id_capture_required": True,
        "fixture_receipt_required_before_browser_execution": True,
        "cleanup": [
            "delete_only_rows_listed_in_fixture_receipt",
            "assert_no_non_fixture_row_changed",
            "record_cleanup_receipt_sha256",
        ],
        "forbidden": [
            "production_write",
            "clone_a_write",
            "reuse_existing_malformed_production_row",
            "mutate_template_task_or_resource",
        ],
        "expected_scenario_id": scenario_id,
    }
    if fixture_kind == "archived_terminal":
        if canonical_population != 0:
            raise InputError(
                "archived_terminal fixture requires canonical Archived population 0"
            )
        if not canonical_entities_sha256 or not SHA256_RE.fullmatch(
            canonical_entities_sha256
        ):
            raise InputError(
                "archived_terminal fixture requires canonical entity document hash"
            )
        plan.update(
            {
                "canonical_archived_population": 0,
                "canonical_entities_sha256": canonical_entities_sha256,
                "historical_migration_coverage": (
                    "not_applicable_zero_frozen_population"
                ),
                "expected_runtime": {
                    "task_status": "Archived",
                    "current_handler_id": None,
                    "resource_group_count": 1,
                    "revision_count": 0,
                    "asset_count": 0,
                    "allowed_actions": [],
                    "module_state": "closed",
                },
            }
        )
    return plan


def build_case_oracle(
    *,
    scenario_id: str,
    combination: str,
    viewport: str,
    task_id: int,
    resource_ids: list[str],
    revision_ids: list[int],
    revision_facts: list[dict[str, Any]],
    requirements: dict[str, Any],
    allowed_actions_oracle: dict[str, Any],
    oracle_context: dict[str, Any],
) -> str:
    return canonical_sha256(
        {
            "schema_version": SCHEMA_VERSION,
            "scenario_id": scenario_id,
            "combination": combination,
            "viewport": viewport,
            "reviewed_mapping_sha256": oracle_context["mapping_sha256"],
            "canonical_entities_sha256": oracle_context[
                "canonical_entities_sha256"
            ],
            "scenario_catalog_sha256": oracle_context[
                "scenario_catalog_sha256"
            ],
            "api_oracle_sha256": oracle_context["api_oracle_sha256"],
            "fixture_receipt_sha256": oracle_context[
                "fixture_receipt_sha256"
            ],
            "api_oracle_case_sha256": allowed_actions_oracle[
                "api_oracle_case_sha256"
            ],
            "task_id": task_id,
            "resource_ids": resource_ids,
            "revision_ids": revision_ids,
            "revision_facts_sha256": canonical_sha256(revision_facts),
            "requirements": requirements,
            "allowed_actions": allowed_actions_oracle["allowed_actions"],
            "http_probes": allowed_actions_oracle["http_probes"],
            "resource_oracle": allowed_actions_oracle["resource_oracle"],
        }
    )


def build_sample(
    scenario: dict[str, Any],
    facts: Facts,
    mode: str,
    allowed_actions_by_case: dict[tuple[str, str], dict[str, Any]],
    oracle_context: dict[str, Any],
    fixture_rows: dict[str, dict[str, Any]],
    fixture_receipt_sha256: str,
) -> tuple[dict[str, Any] | None, str | None]:
    resources, extra_evidence, rationale, fixture = choose_scenario(scenario["id"], facts)
    if not resources or not rationale:
        return None, (
            f"no candidate satisfies selector {scenario['id']}; "
            "arbitrary task fallback is prohibited"
        )

    task_ids = {int(group["task_id"]) for group in resources}
    if len(task_ids) != 1:
        raise InputError(f"selector {scenario['id']} returned resources from multiple tasks")
    task_id = next(iter(task_ids))
    resource_keys: list[str] = []
    revision_locators: list[str] = []
    revision_ids: list[int] = []
    evidence: list[dict[str, Any]] = []
    policies: set[str] = set()
    revision_facts: list[dict[str, Any]] = []

    for group in resources:
        if group["scope_kind"] in {"planning", "task_terminal"}:
            continue
        resource_key = group_locator(group)
        resource_keys.append(resource_key)
        canonical_group = facts.canonical.get(("G02", resource_key))
        if canonical_group:
            evidence.append(entity_evidence(canonical_group))
        for revision in group["history"]:
            locator = revision_locator(group, revision)
            revision_locators.append(locator)
            revision_ids.append(facts.revision_id_by_locator[locator])
            policies.update(str(value) for value in revision.get("review_policy_ids", []))
            canonical_revision = facts.canonical.get(("G03", locator))
            if canonical_revision:
                evidence.append(entity_evidence(canonical_revision))
            revision_facts.append(
                {
                    "resource_key": resource_key,
                    "revision_no": int(revision["revision_no"]),
                    "predicted_revision_id": facts.revision_id_by_locator[locator],
                    "status": revision.get("status"),
                    "source_stage": revision.get("source_stage"),
                    "mode": revision.get("mode"),
                    "source_bundle": (
                        {
                            "task_asset_id": revision["source_bundle"].get("task_asset_id"),
                            "bundle_sha256": revision["source_bundle"].get("bundle_sha256"),
                            "ordered_member_task_asset_ids": [
                                int(member["task_asset_id"])
                                for member in revision["source_bundle"].get("members", [])
                            ],
                        }
                        if revision.get("source_bundle")
                        else None
                    ),
                    "mapping_row_sha256": revision.get("manifest_row_hash"),
                    "confidence": revision.get("confidence"),
                }
            )
    task_entity = facts.tasks.get(task_id, {}).get("entity")
    if task_entity:
        evidence.append(entity_evidence(task_entity))
    evidence.extend(extra_evidence)
    # Deduplicate evidence without discarding deterministic order.
    deduplicated: list[dict[str, Any]] = []
    seen_evidence: set[str] = set()
    for row in evidence:
        digest = canonical_sha256(row)
        if digest not in seen_evidence:
            seen_evidence.add(digest)
            deduplicated.append(row)

    runtime_resolution: dict[str, Any] | None = None
    if fixture:
        receipt_row = fixture_rows.get(str(scenario["id"]))
        if receipt_row is None:
            raise InputError(
                f"fixture receipt does not resolve scenario {scenario['id']}"
            )
        (
            task_id,
            resource_keys,
            revision_ids,
            revision_facts,
            runtime_resolution,
        ) = resolve_fixture_selection(
            scenario_id=str(scenario["id"]),
            fixture_plan=fixture,
            fixture_receipt_row=receipt_row,
            fixture_receipt_sha256=fixture_receipt_sha256,
            task_id=task_id,
            resource_ids=resource_keys,
            revision_ids=revision_ids,
            revision_facts=revision_facts,
        )

    combination_requirements = [
        requirements_for(scenario, combination)
        for combination in scenario["required_combinations"]
    ]
    if (
        any(requirements["requires_revision_ids"] for requirements in combination_requirements)
        and not revision_ids
    ):
        return None, f"selector {scenario['id']} found no revision IDs"
    sample_status = "READY" if mode == "final" else "PENDING"
    matrix: list[dict[str, Any]] = []
    for combination in scenario["required_combinations"]:
        requirements = requirements_for(scenario, combination)
        actions_oracle = allowed_actions_by_case[
            (str(scenario["id"]), str(combination))
        ]
        resource_oracle = actions_oracle["resource_oracle"]
        uses_v8_groups = resource_oracle["kind"] == "v8_resource_groups"
        case_resource_ids = resource_keys if uses_v8_groups else []
        case_revision_ids = (
            revision_ids
            if uses_v8_groups and requirements["requires_revision_ids"]
            else []
        )
        for viewport in scenario["required_viewports"]:
            matrix.append(
                {
                    "combination": combination,
                    "viewport": viewport,
                    "requirements": requirements,
                    "task_id": task_id,
                    "resource_ids": case_resource_ids,
                    "revision_ids": case_revision_ids,
                    "resource_oracle": resource_oracle,
                    "allowed_actions": actions_oracle["allowed_actions"],
                    "http_probes": actions_oracle["http_probes"],
                    "oracle_sha256": build_case_oracle(
                        scenario_id=str(scenario["id"]),
                        combination=str(combination),
                        viewport=str(viewport),
                        task_id=task_id,
                        resource_ids=case_resource_ids,
                        revision_ids=case_revision_ids,
                        revision_facts=revision_facts,
                        requirements=requirements,
                        allowed_actions_oracle=actions_oracle,
                        oracle_context=oracle_context,
                    ),
                }
            )
    task_facts = {
        "task_type": facts.tasks.get(task_id, {}).get("task_type"),
        "task_status": facts.tasks.get(task_id, {}).get("task_status"),
    }
    if fixture and fixture.get("fixture_kind") == "archived_terminal":
        task_facts = {
            "task_type": facts.tasks.get(task_id, {}).get("task_type"),
            "task_status": "Archived",
            "template_task_status": facts.tasks.get(task_id, {}).get("task_status"),
            "expected_resource_group_count": 1,
            "expected_revision_count": 0,
            "expected_asset_count": 0,
        }
    sample: dict[str, Any] = {
        "scenario_id": scenario["id"],
        "status": sample_status,
        "selector_id": f"{SELECTOR_VERSION}:{scenario['id']}",
        "target_kind": "clone_b_fixture_derived" if fixture else "reviewed_real_task",
        "task_id": task_id,
        "resource_identity_kind": "canonical_task_scope_key",
        "resource_ids": resource_keys,
        "resource_keys": resource_keys,
        "revision_ids": revision_ids,
        "revision_locators": revision_locators,
        "revision_id_derivation": (
            "workflow_groups_migrate_mapping_order_from_empty_clone_b_revision_table_v1"
        ),
        "task_facts": task_facts,
        "policy_ids": sorted(policies),
        "revision_facts": revision_facts,
        "required_combinations": list(scenario["required_combinations"]),
        "required_viewports": list(scenario["required_viewports"]),
        "coverage_matrix": matrix,
        "rationale": rationale,
        "evidence": deduplicated,
    }
    if fixture:
        sample["fixture_plan"] = fixture
        sample["runtime_resolution"] = runtime_resolution
        if fixture["fixture_kind"] != "permission_denied_identity":
            sample["resource_identity_kind"] = "clone_b_runtime_group_id"
            sample["resource_keys"] = []
            sample["revision_locators"] = []
            sample["revision_id_derivation"] = "verified_clone_b_fixture_receipt_v2"
    sample["sample_sha256"] = canonical_sha256(sample)
    return sample, None


def build_manifest(args: argparse.Namespace) -> tuple[dict[str, Any], int]:
    catalog_path = Path(args.scenarios)
    mapping_path = Path(args.mapping)
    canonical_path = Path(args.canonical_entities) if args.canonical_entities else None
    edge_receipt_path = Path(args.edge_receipt)
    fixture_receipt_path = Path(args.fixture_receipt)
    api_oracle_path = Path(args.api_oracle)
    catalog = load_json(catalog_path, "scenario catalog")
    scenarios = validate_catalog(catalog)
    mapping = load_json(mapping_path, "mapping")
    review_failures = mapping_review_failures(mapping)
    if args.mode == "final" and review_failures:
        raise InputError(
            "final mode requires a fully reviewed mapping; unconfirmed rows: "
            + ", ".join(review_failures[:20])
            + (f" (+{len(review_failures) - 20} more)" if len(review_failures) > 20 else "")
        )
    if args.mode == "final" and canonical_path is None:
        raise InputError("final mode requires --canonical-entities")
    mapping_hash = file_sha256(mapping_path)
    catalog_hash = file_sha256(catalog_path)
    canonical: dict[tuple[str, str], dict[str, Any]] = {}
    canonical_hash: str | None = None
    if canonical_path is not None:
        canonical_hash = file_sha256(canonical_path)
        canonical = index_canonical(
            load_json(canonical_path, "canonical entities"),
            mapping_hash,
            args.mode == "final",
        )
    edge_receipt = load_json(edge_receipt_path, "edge receipt")
    sealed_edges = validate_edge_receipt(edge_receipt)
    edge_receipt_hash = file_sha256(edge_receipt_path)
    fixture_receipt = load_json(fixture_receipt_path, "fixture receipt")
    fixture_receipt_hash = file_sha256(fixture_receipt_path)
    fixture_rows = validate_fixture_receipt(
        fixture_receipt,
        catalog_sha256=catalog_hash,
        mapping_sha256=mapping_hash,
        canonical_entities_sha256=canonical_hash,
    )
    api_oracle = load_json(api_oracle_path, "API oracle")
    allowed_actions_by_case = validate_api_oracle(
        api_oracle,
        scenarios,
        catalog_sha256=catalog_hash,
        mapping_sha256=mapping_hash,
        canonical_entities_sha256=canonical_hash,
        edge_receipt_sha256=edge_receipt_hash,
        fixture_receipt_sha256=fixture_receipt_hash,
    )
    oracle_context = {
        "scenario_catalog_sha256": catalog_hash,
        "mapping_sha256": mapping_hash,
        "canonical_entities_sha256": canonical_hash,
        "api_oracle_sha256": api_oracle["manifest_sha256"],
        "fixture_receipt_sha256": fixture_receipt_hash,
    }
    facts = Facts(mapping, canonical, canonical_hash)

    samples: list[dict[str, Any]] = []
    blockers: list[dict[str, Any]] = []
    for scenario in scenarios:
        sample, failure = build_sample(
            scenario,
            facts,
            args.mode,
            allowed_actions_by_case,
            oracle_context,
            fixture_rows,
            fixture_receipt_hash,
        )
        if failure:
            blockers.append(
                {
                    "scenario_id": scenario["id"],
                    "code": "no_verified_candidate",
                    "detail": failure,
                }
            )
        else:
            samples.append(sample)

    selected_ids = {sample["scenario_id"] for sample in samples}
    expected_ids = {scenario["id"] for scenario in scenarios}
    if selected_ids | {row["scenario_id"] for row in blockers} != expected_ids:
        raise InputError(
            f"selector result does not account for all "
            f"{EXPECTED_SCENARIO_COUNT} scenarios"
        )

    if blockers:
        status = "PENDING" if args.mode == "prepare" else "BLOCKED"
        exit_code = 2 if args.mode == "prepare" else 3
    elif args.mode == "prepare":
        status, exit_code = "PENDING", 2
    else:
        status, exit_code = "PASS", 0

    manifest: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "selector_version": SELECTOR_VERSION,
        "gate": "G7",
        "mode": args.mode,
        "status": status,
        "input_sha256": {
            "scenario_catalog_sha256": catalog_hash,
            "mapping_sha256": mapping_hash,
            "canonical_entities_sha256": canonical_hash,
            "edge_receipt_sha256": edge_receipt_hash,
            "fixture_receipt_sha256": fixture_receipt_hash,
            "api_oracle_sha256": file_sha256(api_oracle_path),
        },
        "sealed_edges": sealed_edges,
        "oracle_contract": {
            "kind": "reviewed_api_allowed_actions_v1",
            "edge_receipt_manifest_sha256": edge_receipt["receipt_sha256"],
            "api_oracle_manifest_sha256": api_oracle["manifest_sha256"],
            "fixture_receipt_payload_sha256": fixture_receipt[
                "receipt_payload_sha256"
            ],
            "executor_supplied_oracle_forbidden": True,
        },
        "mapping_review": {
            "status": "PASS" if not review_failures else "PENDING",
            "unconfirmed_row_count": len(review_failures),
            "unconfirmed_rows": review_failures,
        },
        "revision_id_precondition": {
            "clone_b_revision_table_initial_count": 0,
            "migration_is_exclusive_and_serial": True,
            "runtime_receipt_must_reconfirm_ids": True,
        },
        "sample_count": len(samples),
        "scenario_count": len(scenarios),
        "blocker_count": len(blockers),
        "samples": samples,
        "blockers": blockers,
        "coverage": {
            "scenario_ids": sorted(selected_ids),
            "combinations": sorted(
                {
                    row["combination"]
                    for sample in samples
                    for row in sample["coverage_matrix"]
                }
            ),
            "viewports": sorted(
                {
                    row["viewport"]
                    for sample in samples
                    for row in sample["coverage_matrix"]
                }
            ),
            "fixture_scenarios": sorted(
                sample["scenario_id"]
                for sample in samples
                if sample["target_kind"] == "clone_b_fixture_derived"
            ),
            "positive_contract_fixture_scenarios": sorted(
                sample["scenario_id"]
                for sample in samples
                if sample.get("fixture_plan", {}).get("fixture_class")
                == "positive_contract"
            ),
            "negative_fixture_scenarios": sorted(
                sample["scenario_id"]
                for sample in samples
                if sample.get("fixture_plan", {}).get("fixture_class")
                == "negative_assertion"
            ),
        },
    }
    manifest["manifest_sha256"] = canonical_sha256(manifest)
    return manifest, exit_code


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--scenarios", required=True)
    parser.add_argument("--mapping", required=True)
    parser.add_argument("--canonical-entities")
    parser.add_argument("--edge-receipt", required=True)
    parser.add_argument("--fixture-receipt", required=True)
    parser.add_argument("--api-oracle", required=True)
    parser.add_argument("--mode", choices=("final", "prepare"), required=True)
    parser.add_argument("--output", required=True)
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    try:
        args = parse_args(argv)
        manifest, exit_code = build_manifest(args)
        atomic_write(Path(args.output), manifest)
        print(
            json.dumps(
                {
                    "status": manifest["status"],
                    "sample_count": manifest["sample_count"],
                    "blocker_count": manifest["blocker_count"],
                    "manifest_sha256": manifest["manifest_sha256"],
                },
                ensure_ascii=False,
                sort_keys=True,
            )
        )
        return exit_code
    except (InputError, OSError, UnicodeError, json.JSONDecodeError, KeyError, TypeError) as exc:
        print(str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
