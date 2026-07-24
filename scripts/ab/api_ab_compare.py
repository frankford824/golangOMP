#!/usr/bin/env python3
"""Deterministic, GET-only, four-combination API acceptance comparator.

The comparator never emits resolved authentication headers or response bodies.
It records status/body hashes and requires every accepted difference to be
bound to an exact route, direction, status pair, reason hash, and rule hash.
"""
from __future__ import annotations

import argparse
import dataclasses
import hashlib
import json
import os
import pathlib
import re
import threading
import urllib.error
import urllib.parse
import urllib.request
from collections.abc import Callable, Iterable
from concurrent.futures import Future, ThreadPoolExecutor
from typing import Any

COMBINATION_IDS = (
    "external_external_a",
    "dev_dev_b",
    "external_dev_b",
    "dev_external_a",
)
CORE_ROUTES = (
    "/v1/tasks/{task_id}",
    "/v1/tasks/{task_id}/detail",
    "/v1/tasks/{task_id}/events",
    "/v1/tasks/{task_id}/resource-bundle",
    "/v1/tasks/{task_id}/assets",
)
GROUP_ROUTES = (
    "/v1/resource-groups/{group_id}",
    "/v1/resource-groups/{group_id}/revisions",
)
ASSET_ROUTES = (
    "/v1/task-assets/{task_asset_id}/preview",
    "/v1/task-assets/{task_asset_id}/download",
)
ALL_ROUTE_TEMPLATES = frozenset((*CORE_ROUTES, *GROUP_ROUTES, *ASSET_ROUTES))
MAX_BODY_BYTES = 4 * 1024 * 1024
ID_RE = re.compile(r"[1-9][0-9]*")
SHA256_RE = re.compile(r"[0-9a-f]{64}")
REQUEST_CACHE_POLICY_VERSION = "exact_base_url_identity_path_v1"


def canonical(value: object) -> bytes:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")


def sha256(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def file_sha256(path: pathlib.Path) -> str:
    return sha256(path.read_bytes())


def load_object(path: pathlib.Path, label: str) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{label} must be a JSON object")
    return value


def load_tasks(path: pathlib.Path) -> list[str]:
    ids: list[str] = []
    for line_no, raw_line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        line = raw_line.strip()
        if not line:
            continue
        try:
            value = json.loads(line)
            value = value.get("task_id") if isinstance(value, dict) else value
        except json.JSONDecodeError:
            value = line.split(",", 1)[0]
        task_id = str(value)
        if not ID_RE.fullmatch(task_id):
            raise ValueError(f"invalid task id on line {line_no}")
        ids.append(task_id)
    if not ids or len(ids) != len(set(ids)):
        raise ValueError("task list must be non-empty and unique")
    return sorted(ids, key=int)


def manifest_task_ids(path: pathlib.Path, run_id: str) -> set[str]:
    found: set[str] = set()
    for line_no, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if not line.strip():
            continue
        row = json.loads(line)
        if not isinstance(row, dict):
            raise ValueError(f"manifest line {line_no} must be an object")
        entity_key = str(row.get("entity_key", ""))
        if (
            row.get("run_id") == run_id
            and row.get("gate_name") == "G01"
            and entity_key.startswith("task:")
        ):
            task_id = entity_key.split(":", 1)[1]
            if not ID_RE.fullmatch(task_id):
                raise ValueError(f"invalid G01 task entity on manifest line {line_no}")
            found.add(task_id)
    if not found:
        raise ValueError("reviewed manifest has no G01 task entities")
    return found


def local_url(url: str) -> str:
    parsed = urllib.parse.urlparse(url)
    if (
        parsed.scheme not in {"http", "https"}
        or parsed.hostname not in {"127.0.0.1", "localhost", "host.docker.internal"}
        or parsed.username
        or parsed.password
        or parsed.query
        or parsed.fragment
    ):
        raise ValueError("API clone URL must be a credential-free local URL")
    return url.rstrip("/")


def _validate_headers(value: object) -> dict[str, str]:
    if not isinstance(value, dict) or not value:
        raise ValueError("identity headers must be a non-empty string map")
    headers: dict[str, str] = {}
    forbidden = {"host", "content-length", "transfer-encoding"}
    for key, item in value.items():
        if (
            not isinstance(key, str)
            or not isinstance(item, str)
            or not key.strip()
            or "\r" in key
            or "\n" in key
            or "\r" in item
            or "\n" in item
            or key.lower() in forbidden
        ):
            raise ValueError("identity headers contain an invalid entry")
        headers[key] = item
    return headers


def resolve_headers(identity: dict[str, Any]) -> dict[str, str]:
    sources = [
        key
        for key in ("headers_file", "headers_file_env", "headers_json_env")
        if key in identity
    ]
    if len(sources) != 1:
        raise ValueError(
            f"identity {identity.get('id')} must declare exactly one header source"
        )
    source = sources[0]
    if source == "headers_file":
        path = pathlib.Path(str(identity[source]))
        value = json.loads(path.read_text(encoding="utf-8"))
    elif source == "headers_file_env":
        env_name = str(identity[source])
        if env_name not in os.environ:
            raise ValueError(f"missing header-file environment variable {env_name}")
        value = json.loads(pathlib.Path(os.environ[env_name]).read_text(encoding="utf-8"))
    else:
        env_name = str(identity[source])
        if env_name not in os.environ:
            raise ValueError(f"missing header-json environment variable {env_name}")
        value = json.loads(os.environ[env_name])
    return _validate_headers(value)


@dataclasses.dataclass(frozen=True)
class HttpResult:
    status: int
    body: object
    raw_sha256: str
    body_bytes: int


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):  # noqa: ANN001
        return None


def http_get(base_url: str, path: str, headers: dict[str, str]) -> HttpResult:
    request = urllib.request.Request(base_url + path, headers=headers, method="GET")
    try:
        with urllib.request.build_opener(_NoRedirect()).open(request, timeout=20) as response:
            status = int(response.status)
            raw = response.read(MAX_BODY_BYTES + 1)
    except urllib.error.HTTPError as exc:
        status = int(exc.code)
        raw = exc.read(MAX_BODY_BYTES + 1)
    if len(raw) > MAX_BODY_BYTES:
        raise ValueError(f"GET {path} response exceeds {MAX_BODY_BYTES} bytes")
    try:
        body: object = json.loads(raw) if raw else None
    except json.JSONDecodeError:
        body = {
            "_non_json_sha256": sha256(raw),
            "_bytes": len(raw),
        }
    return HttpResult(status, body, sha256(raw), len(raw))


def _rule_without_hash(rule: dict[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in rule.items() if key != "rule_sha256"}


def load_rules(path: pathlib.Path, retired_routes: Iterable[str]) -> list[dict[str, Any]]:
    document = load_object(path, "rules")
    if set(document) != {"schema_version", "rules"} or document["schema_version"] != 1:
        raise ValueError("rules must contain only schema_version=1 and rules")
    rules = document["rules"]
    if not isinstance(rules, list):
        raise ValueError("rules.rules must be an array")
    allowed_routes = ALL_ROUTE_TEMPLATES | frozenset(retired_routes)
    seen: set[str] = set()
    output: list[dict[str, Any]] = []
    for index, value in enumerate(rules):
        if not isinstance(value, dict):
            raise ValueError(f"rule {index} must be an object")
        required = {
            "rule_id",
            "route",
            "direction",
            "from_status",
            "to_status",
            "reason",
            "reason_sha256",
            "operations",
            "rule_sha256",
        }
        if set(value) != required:
            raise ValueError(f"rule {index} has unexpected or missing fields")
        rule_id = value["rule_id"]
        if not isinstance(rule_id, str) or not rule_id or rule_id in seen:
            raise ValueError(f"rule {index} has invalid or duplicate rule_id")
        seen.add(rule_id)
        if value["route"] not in allowed_routes:
            raise ValueError(f"rule {rule_id} binds an unknown route")
        if value["direction"] not in {
            f"{left}->{right}"
            for offset, left in enumerate(COMBINATION_IDS)
            for right in COMBINATION_IDS[offset + 1 :]
        }:
            raise ValueError(f"rule {rule_id} has an invalid direction")
        if not all(isinstance(value[key], int) for key in ("from_status", "to_status")):
            raise ValueError(f"rule {rule_id} statuses must be integers")
        reason = value["reason"]
        if (
            not isinstance(reason, str)
            or not reason.strip()
            or value["reason_sha256"] != sha256(reason.encode("utf-8"))
        ):
            raise ValueError(f"rule {rule_id} reason hash mismatch")
        operations = value["operations"]
        if not isinstance(operations, list):
            raise ValueError(f"rule {rule_id} operations must be an array")
        for operation in operations:
            if not isinstance(operation, dict) or operation.get("op") not in {
                "remove",
                "map",
            }:
                raise ValueError(f"rule {rule_id} has an invalid operation")
            if operation["op"] == "remove" and set(operation) != {"op", "path"}:
                raise ValueError(f"rule {rule_id} remove operation is invalid")
            if operation["op"] == "map" and set(operation) != {
                "op",
                "path",
                "from",
                "to",
            }:
                raise ValueError(f"rule {rule_id} map operation is invalid")
            path_value = operation.get("path")
            if not isinstance(path_value, str) or not path_value.startswith("/"):
                raise ValueError(f"rule {rule_id} operation path is invalid")
            if operation["op"] == "remove":
                path_tokens = _tokens(path_value)
                protected = {
                    "allowed_actions",
                    "data",
                    "items",
                    "references",
                    "revision_no",
                    "sort_order",
                    "source_task_asset_id",
                    "task_asset_id",
                    "task_asset_ids",
                    "final_task_asset_id",
                    "final_task_asset_ids",
                    "formal_task_asset_id",
                    "formal_task_asset_ids",
                }
                if len(path_tokens) < 2 or path_tokens[-1] in protected | {"*"}:
                    raise ValueError(
                        f"rule {rule_id} cannot remove an ordered, permission, "
                        "identity, or top-level payload field"
                    )
        if (
            not isinstance(value["rule_sha256"], str)
            or not SHA256_RE.fullmatch(value["rule_sha256"])
            or value["rule_sha256"] != sha256(canonical(_rule_without_hash(value)))
        ):
            raise ValueError(f"rule {rule_id} rule hash mismatch")
        output.append(value)
    return output


def load_matrix(path: pathlib.Path) -> tuple[dict[str, str], list[dict[str, str]], dict[str, dict[str, str]], list[str]]:
    document = load_object(path, "matrix")
    if set(document) != {
        "schema_version",
        "combinations",
        "identities",
        "retired_routes",
    } or document["schema_version"] != 1:
        raise ValueError("matrix has unexpected fields or schema_version")
    combinations = document["combinations"]
    if not isinstance(combinations, list) or len(combinations) != 4:
        raise ValueError("matrix must contain exactly four combinations")
    urls: dict[str, str] = {}
    metadata: list[dict[str, str]] = []
    expected = {
        "external_external_a": ("external", "external", "A"),
        "dev_dev_b": ("dev-plus", "dev-plus", "B"),
        "external_dev_b": ("external", "dev-plus", "B"),
        "dev_external_a": ("dev-plus", "external", "A"),
    }
    for combination in combinations:
        if not isinstance(combination, dict) or set(combination) != {
            "id",
            "frontend",
            "backend",
            "data",
            "base_url",
        }:
            raise ValueError("combination has unexpected fields")
        combo_id = combination["id"]
        if combo_id not in expected or combo_id in urls:
            raise ValueError("combination id is invalid or duplicated")
        actual = (combination["frontend"], combination["backend"], combination["data"])
        if actual != expected[combo_id]:
            raise ValueError(f"combination {combo_id} does not match the fixed matrix")
        urls[combo_id] = local_url(str(combination["base_url"]))
        metadata.append(
            {
                "id": combo_id,
                "frontend": str(combination["frontend"]),
                "backend": str(combination["backend"]),
                "data": str(combination["data"]),
            }
        )
    if set(urls) != set(COMBINATION_IDS):
        raise ValueError("all four combination URLs must be present")
    identities_value = document["identities"]
    if not isinstance(identities_value, list) or not identities_value:
        raise ValueError("matrix must contain at least one identity")
    identities: list[dict[str, str]] = []
    resolved: dict[str, dict[str, str]] = {}
    for identity in identities_value:
        if not isinstance(identity, dict) or not {"id", "role"} <= set(identity):
            raise ValueError("identity must contain id and role")
        unexpected = set(identity) - {
            "id",
            "role",
            "headers_file",
            "headers_file_env",
            "headers_json_env",
        }
        identity_id = identity["id"]
        role = identity["role"]
        if (
            unexpected
            or not isinstance(identity_id, str)
            or not identity_id
            or not isinstance(role, str)
            or not role
            or identity_id in resolved
        ):
            raise ValueError("identity is invalid or duplicated")
        resolved[identity_id] = resolve_headers(identity)
        identities.append({"id": identity_id, "role": role})
    retired = document["retired_routes"]
    if not isinstance(retired, list) or not retired:
        raise ValueError("retired_routes must be a non-empty array")
    retired_routes: list[str] = []
    for route in retired:
        if (
            not isinstance(route, str)
            or not route.startswith("/v1/")
            or set(re.findall(r"{([^}]+)}", route)) - {"task_id"}
            or route in ALL_ROUTE_TEMPLATES
            or route in retired_routes
        ):
            raise ValueError("retired route is invalid or duplicated")
        retired_routes.append(route)
    return urls, sorted(metadata, key=lambda item: COMBINATION_IDS.index(item["id"])), resolved, sorted(retired_routes)


def group_ids(value: object) -> set[str]:
    found: set[str] = set()
    if isinstance(value, dict):
        group_id = value.get("group_id")
        if isinstance(group_id, int) and group_id > 0:
            found.add(str(group_id))
        if (
            "scope_kind" in value
            and isinstance(value.get("task_id"), int)
            and isinstance(value.get("id"), int)
            and value["id"] > 0
        ):
            found.add(str(value["id"]))
        for child in value.values():
            found.update(group_ids(child))
    elif isinstance(value, list):
        for child in value:
            found.update(group_ids(child))
    return found


def task_asset_ids(value: object) -> set[str]:
    found: set[str] = set()
    scalar_keys = {
        "task_asset_id",
        "source_task_asset_id",
        "final_task_asset_id",
        "formal_task_asset_id",
    }
    list_keys = {
        "task_asset_ids",
        "final_task_asset_ids",
        "formal_task_asset_ids",
    }
    if isinstance(value, dict):
        # /v1/tasks/{id}/assets returns DesignAssetVersion objects whose
        # immutable task_assets identity is the generic `id` field.
        if (
            isinstance(value.get("id"), int)
            and value["id"] > 0
            and isinstance(value.get("version_no"), int)
            and isinstance(value.get("asset_type"), str)
        ):
            found.add(str(value["id"]))
        for key, child in value.items():
            if key in scalar_keys and isinstance(child, int) and child > 0:
                found.add(str(child))
            elif key in list_keys and isinstance(child, list):
                found.update(str(item) for item in child if isinstance(item, int) and item > 0)
            found.update(task_asset_ids(child))
    elif isinstance(value, list):
        for child in value:
            found.update(task_asset_ids(child))
    return found


def _tokens(path: str) -> list[str]:
    return [
        token.replace("~1", "/").replace("~0", "~")
        for token in path.lstrip("/").split("/")
    ]


def _apply_at(value: object, tokens: list[str], transform: Callable[[object], object]) -> tuple[object, int]:
    if not tokens:
        return transform(value), 1
    head, tail = tokens[0], tokens[1:]
    count = 0
    if isinstance(value, dict):
        output = dict(value)
        keys = sorted(output) if head == "*" else [head]
        for key in keys:
            if key not in output:
                continue
            if not tail and transform is _DELETE:
                del output[key]
                count += 1
            else:
                output[key], changed = _apply_at(output[key], tail, transform)
                count += changed
        return output, count
    if isinstance(value, list):
        output = list(value)
        indexes = range(len(output)) if head == "*" else ([int(head)] if head.isdigit() else [])
        for index in indexes:
            if index >= len(output):
                continue
            if not tail and transform is _DELETE:
                output[index] = _REMOVED
                count += 1
            else:
                output[index], changed = _apply_at(output[index], tail, transform)
                count += changed
        if transform is _DELETE:
            output = [item for item in output if item is not _REMOVED]
        return output, count
    return value, 0


def _DELETE(value: object) -> object:
    return _REMOVED


_REMOVED = object()


def apply_rule(
    left: object, right: object, rule: dict[str, Any]
) -> tuple[object, object]:
    for operation in rule["operations"]:
        path = _tokens(operation["path"])
        if operation["op"] == "remove":
            left, left_count = _apply_at(left, path, _DELETE)
            right, right_count = _apply_at(right, path, _DELETE)
            if left_count == 0 and right_count == 0:
                raise ValueError(
                    f"normalization rule {rule['rule_id']} remove path did not match"
                )
        else:
            expected_from, expected_to = operation["from"], operation["to"]

            def map_left(value: object) -> object:
                if value != expected_from:
                    raise ValueError(
                        f"normalization rule {rule['rule_id']} from value mismatch"
                    )
                return expected_to

            def verify_right(value: object) -> object:
                if value != expected_to:
                    raise ValueError(
                        f"normalization rule {rule['rule_id']} to value mismatch"
                    )
                return value

            left, left_count = _apply_at(left, path, map_left)
            right, right_count = _apply_at(right, path, verify_right)
            if left_count == 0 or right_count == 0:
                raise ValueError(
                    f"normalization rule {rule['rule_id']} map path did not match both sides"
                )
    return left, right


def history_items(value: object) -> list[dict[str, Any]]:
    pages = value if isinstance(value, list) else [value]
    items: list[dict[str, Any]] = []
    for page in pages:
        data = page.get("data", page) if isinstance(page, dict) else {}
        rows = data.get("items", []) if isinstance(data, dict) else []
        if isinstance(rows, list):
            items.extend(row for row in rows if isinstance(row, dict))
    return items


def revision_order_errors(value: object) -> list[str]:
    errors: list[str] = []
    for revision in history_items(value):
        revision_no = revision.get("revision_no", "?")
        for field in ("items", "references"):
            rows = revision.get(field, [])
            if rows is None:
                continue
            if not isinstance(rows, list):
                errors.append(f"revision {revision_no} {field} is not an array")
                continue
            sort_orders = [
                row.get("sort_order") for row in rows if isinstance(row, dict)
            ]
            if len(sort_orders) != len(rows) or not all(
                isinstance(item, int) for item in sort_orders
            ):
                errors.append(f"revision {revision_no} {field} lacks sort_order")
                continue
            if sort_orders != sorted(sort_orders) or any(
                right != left + 1
                for left, right in zip(sort_orders, sort_orders[1:])
            ):
                errors.append(f"revision {revision_no} {field} order is invalid")
    return errors


class Runner:
    def __init__(
        self,
        urls: dict[str, str],
        identities: list[dict[str, str]],
        resolved_headers: dict[str, dict[str, str]],
        rules: list[dict[str, Any]],
        retired_routes: list[str],
        requester: Callable[[str, str, dict[str, str]], HttpResult],
    ):
        self.urls = dict(urls)
        self.identities = [dict(identity) for identity in identities]
        self.resolved_headers = {
            identity: dict(headers)
            for identity, headers in resolved_headers.items()
        }
        self.rules = rules
        self.retired_routes = frozenset(retired_routes)
        self.requester = requester
        self.observations: list[dict[str, Any]] = []
        self.results: dict[tuple[str, str, str, str], HttpResult] = {}
        self.violations: list[dict[str, str]] = []
        self.used_rules: set[str] = set()
        self._state_lock = threading.RLock()
        self._requests: dict[tuple[str, str, str], Future[HttpResult]] = {}
        self.logical_request_count = 0
        self.physical_request_count = 0

    def request(
        self,
        combination: str,
        identity: str,
        path: str,
    ) -> HttpResult:
        """Return one immutable result per exact origin/identity/path.

        The Future provides single-flight behavior: one worker owns the
        physical GET while every concurrent logical caller waits for and
        receives the same frozen HttpResult, or the same exception. Identity
        IDs deliberately remain in the key even when resolved headers happen
        to be byte-for-byte equal.
        """
        key = (self.urls[combination], identity, path)
        with self._state_lock:
            self.logical_request_count += 1
            future = self._requests.get(key)
            owner = future is None
            if owner:
                future = Future()
                self._requests[key] = future
                self.physical_request_count += 1
        assert future is not None
        if owner:
            try:
                result = self.requester(
                    self.urls[combination],
                    path,
                    dict(self.resolved_headers[identity]),
                )
                if not isinstance(result, HttpResult):
                    raise TypeError("requester must return HttpResult")
            except BaseException as exc:
                future.set_exception(exc)
            else:
                future.set_result(result)
        return future.result()

    def observation(
        self,
        combination: str,
        identity: str,
        route: str,
        entity: str,
        result: HttpResult,
    ) -> None:
        row = {
            "combination": combination,
            "identity": identity,
            "route": route,
            "entity_key": entity,
            "status": result.status,
            "body_sha256": sha256(canonical(result.body)),
            "raw_sha256": result.raw_sha256,
            "body_bytes": result.body_bytes,
        }
        with self._state_lock:
            self.observations.append(row)

    def violation(self, code: str, entity: str, detail: str) -> None:
        with self._state_lock:
            self.violations.append(
                {"violation_code": code, "entity_key": entity, "detail": detail}
            )

    def fetch(
        self,
        combination: str,
        identity: str,
        route: str,
        path: str,
        entity: str,
    ) -> HttpResult:
        result = self.request(combination, identity, path)
        with self._state_lock:
            self.results[(combination, identity, route, entity)] = result
        self.observation(combination, identity, route, entity, result)
        if result.status >= 500:
            self.violation(
                "api.server_error", entity, f"{combination}/{identity} returned {result.status}"
            )
        return result

    def fetch_history(
        self, combination: str, identity: str, group_id: str, entity: str
    ) -> HttpResult:
        pages: list[object] = []
        page = 1
        while True:
            path = (
                f"/v1/resource-groups/{group_id}/revisions"
                f"?page={page}&page_size=200"
            )
            result = self.request(combination, identity, path)
            self.observation(
                combination,
                identity,
                GROUP_ROUTES[1],
                f"{entity}:page:{page}",
                result,
            )
            if result.status >= 500:
                self.violation(
                    "api.server_error",
                    entity,
                    f"{combination}/{identity} returned {result.status}",
                )
            if result.status != 200:
                final = result
                break
            pages.append(result.body)
            data = (
                result.body.get("data", result.body)
                if isinstance(result.body, dict)
                else {}
            )
            total = data.get("total", 0) if isinstance(data, dict) else 0
            rows = data.get("items", []) if isinstance(data, dict) else []
            if not isinstance(total, int) or not isinstance(rows, list):
                self.violation(
                    "api.invalid_pagination", entity, f"{combination}/{identity}"
                )
                final = HttpResult(200, pages, sha256(canonical(pages)), len(canonical(pages)))
                break
            if page * 200 >= total or not rows:
                final = HttpResult(200, pages, sha256(canonical(pages)), len(canonical(pages)))
                break
            page += 1
            if page > 10000:
                raise ValueError(f"history pagination did not terminate for group {group_id}")
        with self._state_lock:
            self.results[(combination, identity, GROUP_ROUTES[1], entity)] = final
        if final.status == 200:
            numbers = [
                item.get("revision_no")
                for item in history_items(final.body)
                if isinstance(item.get("revision_no"), int)
            ]
            if len(numbers) != len(history_items(final.body)) or any(
                left <= right for left, right in zip(numbers, numbers[1:])
            ):
                self.violation(
                    "api.revision_order_invalid", entity, f"{combination}/{identity}"
                )
            for detail in revision_order_errors(final.body):
                self.violation(
                    "api.asset_order_invalid",
                    entity,
                    f"{combination}/{identity}: {detail}",
                )
        return final

    def find_rule(
        self, route: str, direction: str, from_status: int, to_status: int
    ) -> dict[str, Any] | None:
        matches = [
            rule
            for rule in self.rules
            if rule["route"] == route
            and rule["direction"] == direction
            and rule["from_status"] == from_status
            and rule["to_status"] == to_status
        ]
        if len(matches) > 1:
            raise ValueError(
                f"multiple normalization rules match {route} {direction} "
                f"{from_status}->{to_status}"
            )
        return matches[0] if matches else None

    def compare_result(
        self,
        route: str,
        entity: str,
        identity: str,
        left_combo: str,
        right_combo: str,
    ) -> None:
        left = self.results[(left_combo, identity, route, entity)]
        right = self.results[(right_combo, identity, route, entity)]
        direction = f"{left_combo}->{right_combo}"
        # Permission widening is directional from the frozen A baseline to the
        # migrated B candidate, regardless of the pair iteration order.  The
        # four-combination matrix also contains B->A and same-data comparisons;
        # treating those lexical directions as authorization transitions would
        # misclassify candidate permission tightening as widening.
        left_data = "A" if left_combo.endswith("_a") else "B"
        right_data = "A" if right_combo.endswith("_a") else "B"
        if left_data != right_data:
            if left_data == "A":
                baseline_combo, baseline = left_combo, left
                candidate_combo, candidate = right_combo, right
            else:
                baseline_combo, baseline = right_combo, right
                candidate_combo, candidate = left_combo, left
        else:
            baseline_combo = candidate_combo = ""
            baseline = candidate = None
        if (
            baseline is not None
            and candidate is not None
            and baseline.status in {401, 403}
            and candidate.status not in {401, 403}
        ):
            self.violation(
                "api.permission_widened",
                entity,
                f"{identity} {baseline_combo}->{candidate_combo} "
                f"{baseline.status}->{candidate.status}",
            )
            return
        if route in self.retired_routes:
            # Each retired route is already asserted to be exactly 404. Error
            # envelope wording is deliberately not a compatibility contract.
            return
        if route in ASSET_ROUTES:
            if left.status not in {200, 403, 410} or right.status not in {200, 403, 410}:
                self.violation(
                    "api.asset_status_invalid",
                    entity,
                    f"{identity} {direction} {left.status}->{right.status}",
                )
                return
            if left.status == 200 and right.status == 410:
                # A tombstone is an asset loss unless an exact status rule approves it.
                pass
        rule = self.find_rule(route, direction, left.status, right.status)
        left_body, right_body = left.body, right.body
        different = left.status != right.status or canonical(left_body) != canonical(right_body)
        if rule is not None and different:
            with self._state_lock:
                self.used_rules.add(rule["rule_id"])
            try:
                left_body, right_body = apply_rule(left_body, right_body, rule)
            except ValueError as exc:
                self.violation("api.normalization_failed", entity, str(exc))
                return
        if left.status != right.status:
            if rule is None:
                code = (
                    "api.asset_lost"
                    if route in ASSET_ROUTES and left.status == 200
                    else "api.status_mismatch"
                )
                self.violation(
                    code,
                    entity,
                    f"{identity} {direction} {left.status}->{right.status}",
                )
            return
        if canonical(left_body) != canonical(right_body):
            self.violation(
                "api.body_mismatch",
                entity,
                f"{identity} {direction} "
                f"{sha256(canonical(left_body))}!={sha256(canonical(right_body))}",
            )

    def compare_all(self) -> None:
        for identity in [item["id"] for item in self.identities]:
            for offset, left in enumerate(COMBINATION_IDS):
                for right in COMBINATION_IDS[offset + 1 :]:
                    shared = sorted(
                        {
                            (route, entity)
                            for combo, ident, route, entity in self.results
                            if combo == left and ident == identity
                        }
                        & {
                            (route, entity)
                            for combo, ident, route, entity in self.results
                            if combo == right and ident == identity
                        }
                    )
                    for route, entity in shared:
                        self.compare_result(route, entity, identity, left, right)


def compare(
    *,
    matrix_path: pathlib.Path,
    task_ids_path: pathlib.Path,
    rules_path: pathlib.Path,
    manifest_path: pathlib.Path,
    run_id: str,
    requester: Callable[[str, str, dict[str, str]], HttpResult] = http_get,
    workers: int = 16,
    request_metrics: dict[str, Any] | None = None,
) -> dict[str, Any]:
    if not isinstance(workers, int) or isinstance(workers, bool) or not 1 <= workers <= 64:
        raise ValueError("workers must be an integer between 1 and 64")
    if request_metrics is not None:
        request_metrics.clear()
    urls, combinations, resolved, retired_routes = load_matrix(matrix_path)
    identities = [
        {"id": identity_id, "role": next(
            item["role"]
            for item in load_object(matrix_path, "matrix")["identities"]
            if item["id"] == identity_id
        )}
        for identity_id in sorted(resolved)
    ]
    rules = load_rules(rules_path, retired_routes)
    tasks = load_tasks(task_ids_path)
    manifest_tasks = manifest_task_ids(manifest_path, run_id)
    if set(tasks) != manifest_tasks:
        raise ValueError(
            "task list does not exactly match reviewed manifest: "
            f"list={len(tasks)},manifest={len(manifest_tasks)}"
        )
    runner = Runner(urls, identities, resolved, rules, retired_routes, requester)
    dynamic_groups: dict[tuple[str, str, str], set[str]] = {}
    dynamic_assets: dict[tuple[str, str, str], set[str]] = {}

    def fetch_task(job: tuple[str, str, str]) -> tuple[tuple[str, str, str], set[str], set[str]]:
        combination, identity, task_id = job
        bodies: list[object] = []
        for route in CORE_ROUTES:
            path = route.format(task_id=task_id)
            entity = f"task:{task_id}:{route}"
            result = runner.fetch(combination, identity, route, path, entity)
            if result.status not in {200, 403}:
                runner.violation(
                    "api.task_status_invalid",
                    entity,
                    f"{combination}/{identity} returned {result.status}",
                )
            if result.status == 200:
                bodies.append(result.body)
        for route in retired_routes:
            entity = f"task:{task_id}:retired:{route}"
            result = runner.fetch(
                combination,
                identity,
                route,
                route.format(task_id=task_id),
                entity,
            )
            if result.status != 404:
                runner.violation(
                    "api.retired_route_not_404",
                    entity,
                    f"{combination}/{identity} returned {result.status}",
                )
        return (
            job,
            set().union(*(group_ids(body) for body in bodies)),
            set().union(*(task_asset_ids(body) for body in bodies)),
        )

    identity_ids = [item["id"] for item in identities]
    task_jobs = [
        (combination, identity, task_id)
        for combination in COMBINATION_IDS
        for identity in identity_ids
        for task_id in tasks
    ]
    with ThreadPoolExecutor(max_workers=workers) as executor:
        for key, groups, assets in executor.map(fetch_task, task_jobs):
            dynamic_groups[key] = groups
            dynamic_assets[key] = assets
    all_groups = sorted(
        set().union(*dynamic_groups.values()) if dynamic_groups else set(), key=int
    )
    def fetch_group(job: tuple[str, str, str]) -> tuple[str, str, set[str]]:
        combination, identity, group_id = job
        entity = f"group:{group_id}"
        detail = runner.fetch(
            combination,
            identity,
            GROUP_ROUTES[0],
            GROUP_ROUTES[0].format(group_id=group_id),
            entity,
        )
        assets = task_asset_ids(detail.body) if detail.status == 200 else set()
        history = runner.fetch_history(combination, identity, group_id, entity)
        if history.status == 200:
            assets.update(task_asset_ids(history.body))
        return combination, identity, assets

    group_jobs = [
        (combination, identity, group_id)
        for combination in COMBINATION_IDS
        for identity in identity_ids
        for group_id in all_groups
    ]
    group_assets: set[str] = set()
    with ThreadPoolExecutor(max_workers=workers) as executor:
        for _combination, _identity, assets in executor.map(fetch_group, group_jobs):
            group_assets.update(assets)
    all_assets = sorted(
        (set().union(*dynamic_assets.values()) if dynamic_assets else set())
        | group_assets,
        key=int,
    )
    def fetch_asset(job: tuple[str, str, str]) -> None:
        combination, identity, asset_id = job
        entity = f"task-asset:{asset_id}"
        for route in ASSET_ROUTES:
            result = runner.fetch(
                combination,
                identity,
                route,
                route.format(task_asset_id=asset_id),
                entity,
            )
            if result.status not in {200, 403, 410}:
                runner.violation(
                    "api.asset_status_invalid",
                    entity,
                    f"{combination}/{identity} {route} returned {result.status}",
                )

    asset_jobs = [
        (combination, identity, asset_id)
        for combination in COMBINATION_IDS
        for identity in identity_ids
        for asset_id in all_assets
    ]
    with ThreadPoolExecutor(max_workers=workers) as executor:
        for _ in executor.map(fetch_asset, asset_jobs):
            pass
    runner.compare_all()
    observations = sorted(
        runner.observations,
        key=lambda item: (
            COMBINATION_IDS.index(item["combination"]),
            item["identity"],
            item["route"],
            item["entity_key"],
        ),
    )
    violations = sorted(
        runner.violations,
        key=lambda item: (
            item["violation_code"],
            item["entity_key"],
            item["detail"],
        ),
    )
    result: dict[str, Any] = {
        "schema_version": 1,
        "run_id": run_id,
        "status": "PASS" if not violations else "BLOCKED",
        "task_count": len(tasks),
        "group_count": len(all_groups),
        "task_asset_count": len(all_assets),
        "request_count": len(observations),
        "combination_matrix": combinations,
        "identities": identities,
        "task_ids_sha256": sha256(canonical(tasks)),
        "matrix_sha256": file_sha256(matrix_path),
        "rules_sha256": file_sha256(rules_path),
        "manifest_sha256": file_sha256(manifest_path),
        "used_rule_ids": sorted(runner.used_rules),
        "unused_rule_ids": sorted(
            set(rule["rule_id"] for rule in rules) - runner.used_rules
        ),
        "observations": observations,
        "violation_count": len(violations),
        "violations": violations,
    }
    result["evidence_sha256"] = sha256(canonical(result))
    if request_metrics is not None:
        logical_count = runner.logical_request_count
        physical_count = runner.physical_request_count
        if (
            logical_count != result["request_count"]
            or physical_count < 0
            or physical_count > logical_count
        ):
            raise ValueError("request counters differ from logical observations")
        metrics: dict[str, Any] = {
            "schema_version": 1,
            "run_id": run_id,
            "cache_policy_version": REQUEST_CACHE_POLICY_VERSION,
            "logical_request_count": logical_count,
            "physical_request_count": physical_count,
            "deduplicated_request_count": logical_count - physical_count,
            "api_evidence_sha256": result["evidence_sha256"],
            "task_ids_sha256": result["task_ids_sha256"],
            "matrix_sha256": result["matrix_sha256"],
            "rules_sha256": result["rules_sha256"],
            "manifest_sha256": result["manifest_sha256"],
        }
        metrics["evidence_sha256"] = sha256(canonical(metrics))
        request_metrics.update(metrics)
    return result


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--matrix", type=pathlib.Path, required=True)
    parser.add_argument("--task-ids", type=pathlib.Path, required=True)
    parser.add_argument("--rules", type=pathlib.Path, required=True)
    parser.add_argument("--manifest", type=pathlib.Path, required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument(
        "--request-metrics-output",
        type=pathlib.Path,
        help=(
            "optional hash-bound request-count sidecar; the primary G6 "
            "evidence contract remains unchanged"
        ),
    )
    parser.add_argument("--workers", type=int, default=16)
    args = parser.parse_args()
    if (
        args.request_metrics_output
        and args.request_metrics_output.resolve() == args.output.resolve()
    ):
        parser.error("request metrics output must differ from API evidence output")
    try:
        request_metrics: dict[str, Any] = {}
        result = compare(
            matrix_path=args.matrix,
            task_ids_path=args.task_ids,
            rules_path=args.rules,
            manifest_path=args.manifest,
            run_id=args.run_id,
            workers=args.workers,
            request_metrics=request_metrics,
        )
    except (OSError, ValueError, json.JSONDecodeError, urllib.error.URLError) as exc:
        result = {
            "schema_version": 1,
            "run_id": args.run_id,
            "status": "BLOCKED",
            "violation_count": 1,
            "violations": [
                {
                    "violation_code": "api.comparison_error",
                    "entity_key": "*",
                    "detail": str(exc),
                }
            ],
        }
        result["evidence_sha256"] = sha256(canonical(result))
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_bytes(canonical(result) + b"\n")
    if args.request_metrics_output and request_metrics:
        args.request_metrics_output.parent.mkdir(parents=True, exist_ok=True)
        args.request_metrics_output.write_bytes(canonical(request_metrics) + b"\n")
    raise SystemExit(0 if result["violation_count"] == 0 else 1)


if __name__ == "__main__":
    main()
