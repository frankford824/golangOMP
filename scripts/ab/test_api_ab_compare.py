#!/usr/bin/env python3
from __future__ import annotations

import contextlib
import copy
import hashlib
import importlib.util
import io
import json
import os
import pathlib
import sys
import tempfile
import threading
import unittest
from concurrent.futures import ThreadPoolExecutor
from unittest import mock

from scripts.ab import finalize_release_gates as RELEASE

MODULE_PATH = pathlib.Path(__file__).with_name("api_ab_compare.py")
SPEC = importlib.util.spec_from_file_location("api_ab_compare", MODULE_PATH)
assert SPEC and SPEC.loader
api = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = api
SPEC.loader.exec_module(api)


def digest(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def result(status: int, body: object) -> api.HttpResult:
    raw = api.canonical(body)
    return api.HttpResult(status, body, digest(raw), len(raw))


class FakeHTTPResponse:
    def __init__(self, body: bytes) -> None:
        self.body = body

    def __enter__(self):
        return self

    def __exit__(self, *_args: object) -> None:
        return None

    def read(self, _limit: int) -> bytes:
        return self.body


class ComparatorTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = pathlib.Path(self.temp.name)
        self.run_id = "run-test"
        self.tasks = self.root / "tasks.jsonl"
        self.tasks.write_text('{"task_id":1}\n', encoding="utf-8")
        self.manifest = self.root / "manifest.jsonl"
        self.mapping_sha256 = "a" * 64
        self.baseline_attestation_sha256 = "b" * 64
        manifest_inputs = {
            "mapping_sha256": self.mapping_sha256,
            "baseline_attestation_sha256": (
                self.baseline_attestation_sha256
            ),
        }
        rows = [
            {
                "run_id": self.run_id,
                "gate_name": "G01",
                "entity_key": "task:1",
                "review_state": "pass",
                "expected_state": "matched",
                "detail_json": {
                    "components": ["1", "design", "PendingAudit", "", "1"],
                    "input_sha256": manifest_inputs,
                },
            },
            {
                "run_id": self.run_id,
                "gate_name": "G02",
                "entity_key": "group:1:task:0",
                "review_state": "pass",
                "expected_state": "matched",
                "detail_json": {
                    "components": [
                        "1",
                        "task",
                        "0",
                        "2",
                        "submitted",
                        "",
                        "",
                        "0",
                        "",
                    ],
                    "input_sha256": manifest_inputs,
                },
            },
        ]
        for revision_no in (1, 2):
            rows.append(
                {
                    "run_id": self.run_id,
                    "gate_name": "G03",
                    "entity_key": f"revision:1:task:0:{revision_no}",
                    "review_state": "pass",
                    "expected_state": "matched",
                    "detail_json": {
                        "components": [
                            "1",
                            "task",
                            "0",
                            str(revision_no),
                            "submitted",
                            "single",
                            "",
                            "design",
                            "1",
                            "",
                            "",
                            "",
                        ],
                        "input_sha256": manifest_inputs,
                    },
                }
            )
            rows.append(
                {
                    "run_id": self.run_id,
                    "gate_name": "G04",
                    "entity_key": f"revision-source:1:task:0:{revision_no}",
                    "review_state": "pass",
                    "expected_state": "matched",
                    "detail_json": {
                        "components": ["", "", "", "", "", "", ""],
                        "input_sha256": manifest_inputs,
                    },
                }
            )
        for row in rows:
            row["expected_hash"] = api.component_sha256(
                row["detail_json"]["components"]
            )
        self.manifest.write_text(
            "".join(json.dumps(row) + "\n" for row in rows),
            encoding="utf-8",
        )
        self.api_oracle = self.root / "api-oracle.json"
        oracle = {
            "schema_version": 2,
            "oracle_kind": "non_circular_g6_v2",
            "run_id": self.run_id,
            "inputs": {
                "reviewed_manifest_sha256": digest(
                    self.manifest.read_bytes()
                ),
                "reviewed_mapping_sha256": "a" * 64,
                "snapshot_verdict_sha256": (
                    self.baseline_attestation_sha256
                ),
            },
            "revision_reasons": [
                {
                    "revision_locator": f"1:task:0:{revision_no}",
                    "reason_sha256": digest(b""),
                }
                for revision_no in (1, 2)
            ],
            "tasks": [
                {
                    "task_id": 1,
                    "task_type": "design",
                    "task_status": "PendingAudit",
                    "current_handler_id": None,
                    "workflow_revision": 1,
                    "owner_department_id": None,
                    "owner_team_id": None,
                }
            ],
            "roots": [
                {
                    "root_asset_id": 101,
                    "task_id": 1,
                    "intrinsic_asset_type": "source",
                    "scope_sku_code": "",
                    "retouch_requirement_id": None,
                    "current_locator": "a:20:ref-1:key:content",
                    "approved_locator": None,
                    "provenance": {"kind": "a_preserved"},
                }
            ],
            "versions": [
                {
                    "task_asset_id": 20,
                    "task_id": 1,
                    "root_asset_id": 101,
                    "stable_locator": "a:20:ref-1:key:content",
                    "intrinsic_asset_type": "source",
                    "scope_sku_code": "",
                    "retouch_requirement_id": None,
                    "storage_ref_id": "ref-1",
                    "object_key_sha256": digest(
                        b"tasks/RW-1/source.psd"
                    ),
                    "content_sha256": "a" * 64,
                    "size": 10,
                    "mime_type": "image/png",
                    "upload_status": "uploaded",
                    "deleted_at": "",
                    "cleaned_at": "",
                    "object_deleted_at": "",
                    "asset_version_no": 1,
                    "flow_review_status": "",
                    "approved_at": "",
                    "approved_by": None,
                    "created_at": "2026-01-01T00:00:00Z",
                    "source_asset_version_id": None,
                    "content_availability": "available",
                    "expected_roles": [],
                    "provenance": {
                        "kind": "a_preserved",
                        "a_binding_state": "bound",
                        "a_bound_role": "source",
                    },
                }
            ],
            "revision_roles": [
                {
                    "revision_locator": f"1:task:0:{revision_no}",
                    "task_id": 1,
                    "scope_kind": "task",
                    "scope_ref_id": 0,
                    "revision_no": revision_no,
                    "status": "submitted",
                    "source_stage": "design",
                    "source_kind": "none",
                    "source_locator": None,
                    "final_locators": [],
                    "reference_file_ref_ids": [],
                    "reference_locators": [],
                    "is_working": revision_no == 2,
                    "is_finalized": False,
                }
                for revision_no in (1, 2)
            ],
            "route_expectations": {
                "detail_visible_locators": ["a:20:ref-1:key:content"],
                "list_root_ids": [101],
                "current_locators": ["a:20:ref-1:key:content"],
                "approved_locators": [],
                "historical_unavailable_locators": [],
            },
        }
        oracle["evidence_sha256"] = digest(api.canonical(oracle))
        self.api_oracle.write_text(json.dumps(oracle), encoding="utf-8")
        self.base_api_oracle = copy.deepcopy(oracle)
        os.environ["AB_ADMIN_HEADERS"] = json.dumps(
            {
                "Authorization": "Bearer TOP-SECRET",
                "X-Test-Identity": "admin",
            }
        )
        os.environ["AB_VIEW_INSIDE_HEADERS"] = json.dumps(
            {
                "Authorization": "Bearer TOP-SECRET",
                "X-Test-Identity": "view-inside",
            }
        )
        os.environ["AB_VIEW_OUTSIDE_HEADERS"] = json.dumps(
            {
                "Authorization": "Bearer TOP-SECRET",
                "X-Test-Identity": "view-outside",
            }
        )
        os.environ["AB_NO_VIEW_HEADERS"] = json.dumps(
            {
                "Authorization": "Bearer TOP-SECRET",
                "X-Test-Identity": "no-view",
            }
        )
        self.matrix = self.root / "matrix.json"
        self.matrix.write_text(
            json.dumps(
                {
                    "schema_version": 1,
                    "combinations": [
                        {
                            "id": "external_external_a",
                            "frontend": "external",
                            "backend": "external",
                            "data": "A",
                            "base_url": "http://127.0.0.1:8101",
                        },
                        {
                            "id": "dev_dev_b",
                            "frontend": "dev-plus",
                            "backend": "dev-plus",
                            "data": "B",
                            "base_url": "http://127.0.0.1:8102",
                        },
                        {
                            "id": "external_dev_b",
                            "frontend": "external",
                            "backend": "dev-plus",
                            "data": "B",
                            "base_url": "http://127.0.0.1:8103",
                        },
                        {
                            "id": "dev_external_a",
                            "frontend": "dev-plus",
                            "backend": "external",
                            "data": "A",
                            "base_url": "http://127.0.0.1:8104",
                        },
                    ],
                    "identities": [
                        {
                            "id": "admin",
                            "role": "admin",
                            "headers_json_env": "AB_ADMIN_HEADERS",
                        },
                        {
                            "id": "view-inside",
                            "role": "view_only",
                            "headers_json_env": "AB_VIEW_INSIDE_HEADERS",
                        },
                        {
                            "id": "view-outside",
                            "role": "view_only",
                            "headers_json_env": "AB_VIEW_OUTSIDE_HEADERS",
                        },
                        {
                            "id": "no-view",
                            "role": "no_view",
                            "headers_json_env": "AB_NO_VIEW_HEADERS",
                        },
                    ],
                    "retired_routes": ["/v1/tasks/{task_id}/warehouse-receive"],
                }
            ),
            encoding="utf-8",
        )
        self.rules = self.root / "rules.json"
        self.write_rules([])

    def tearDown(self) -> None:
        for name in (
            "AB_ADMIN_HEADERS",
            "AB_VIEW_INSIDE_HEADERS",
            "AB_VIEW_OUTSIDE_HEADERS",
            "AB_NO_VIEW_HEADERS",
        ):
            os.environ.pop(name, None)
        self.temp.cleanup()

    def write_rules(self, rules: list[dict]) -> None:
        self.rules.write_text(
            json.dumps({"schema_version": 1, "rules": rules}), encoding="utf-8"
        )

    def rebind_api_oracle(self) -> None:
        rows = [
            json.loads(line)
            for line in self.manifest.read_text(encoding="utf-8").splitlines()
            if line.strip()
        ]
        for row in rows:
            if (
                row.get("gate_name") in {"G01", "G02", "G03", "G04", "G05"}
                and isinstance(row.get("detail_json"), dict)
            ):
                row["detail_json"].setdefault(
                    "input_sha256",
                    {
                        "mapping_sha256": self.mapping_sha256,
                        "baseline_attestation_sha256": (
                            self.baseline_attestation_sha256
                        ),
                    },
                )
        self.manifest.write_text(
            "".join(json.dumps(row) + "\n" for row in rows),
            encoding="utf-8",
        )
        oracle = json.loads(self.api_oracle.read_text(encoding="utf-8"))
        oracle["inputs"]["reviewed_manifest_sha256"] = digest(
            self.manifest.read_bytes()
        )
        oracle.pop("evidence_sha256", None)
        oracle["evidence_sha256"] = digest(api.canonical(oracle))
        self.api_oracle.write_text(json.dumps(oracle), encoding="utf-8")

    def write_v3_alias_oracle(self) -> dict:
        oracle = copy.deepcopy(self.base_api_oracle)
        oracle["schema_version"] = 3
        oracle["oracle_kind"] = "non_circular_g6_v3"
        oracle["inputs"] = {
            field: "b" * 64
            for field in (
                api.ORACLE_V2_INPUT_FIELDS
                | api.ORACLE_V3_ALIAS_INPUT_FIELDS
            )
            if field != "clone_a_database"
        }
        oracle["inputs"]["reviewed_manifest_sha256"] = digest(
            self.manifest.read_bytes()
        )
        oracle["inputs"]["reviewed_mapping_sha256"] = "a" * 64
        oracle["inputs"]["clone_a_database"] = "clone_a"
        origin = oracle["versions"][0]
        origin.update(
            {
                "intrinsic_asset_type": "delivery",
                "binding_state": "bound",
                "bound_role": "final",
                "bound_resource_locator": "1:task:0",
                "expected_roles": ["final"],
                "provenance": {
                    "kind": "a_preserved",
                    "a_binding_state": "legacy",
                    "a_bound_role": "",
                },
            }
        )
        oracle["roots"][0]["intrinsic_asset_type"] = "delivery"
        alias = dict(origin)
        alias.update(
            {
                "task_asset_id": 21,
                "stable_locator": (
                    "alias:v1:1:task:0:origin-task-asset:20"
                ),
                "intrinsic_asset_type": "source",
                "flow_review_status": "not_applicable",
                "approved_at": "",
                "approved_by": None,
                "created_at": "",
                "source_asset_version_id": None,
                "binding_state": "bound",
                "bound_role": "source",
                "bound_resource_locator": "1:task:0",
                "expected_roles": ["source"],
                "provenance": {
                    "kind": "source_alias_apply_receipt",
                    "origin_task_asset_id": 20,
                    "origin_locator": origin["stable_locator"],
                    "group_id": 30,
                    "remark": "v8-source-alias:group=30:origin=20",
                },
            }
        )
        oracle["versions"].append(alias)
        oracle["route_expectations"]["detail_visible_locators"].append(
            alias["stable_locator"]
        )
        oracle["revision_roles"][0].update(
            {
                "source_kind": "delivery_source_alias",
                "source_locator": alias["stable_locator"],
                "final_locators": [origin["stable_locator"]],
            }
        )
        oracle.pop("evidence_sha256", None)
        oracle["evidence_sha256"] = digest(api.canonical(oracle))
        self.api_oracle.write_text(json.dumps(oracle), encoding="utf-8")
        return oracle

    def write_oracle_document(self, oracle: dict) -> None:
        oracle.pop("evidence_sha256", None)
        oracle["evidence_sha256"] = digest(api.canonical(oracle))
        self.api_oracle.write_text(json.dumps(oracle), encoding="utf-8")

    @staticmethod
    def append_legacy_oracle_asset(
        oracle: dict, legacy: dict
    ) -> None:
        root_id = legacy.get("root_asset_id")
        if not isinstance(root_id, int) or root_id <= 0:
            root_id = max(
                [row["root_asset_id"] for row in oracle["roots"]] + [0]
            ) + 1
        root = next(
            (
                row
                for row in oracle["roots"]
                if row["root_asset_id"] == root_id
            ),
            None,
        )
        if root is None:
            root = {
                "root_asset_id": root_id,
                "task_id": legacy["task_id"],
                "intrinsic_asset_type": (
                    legacy.get("root_asset_type")
                    or legacy.get("asset_type")
                    or "source"
                ),
                "scope_sku_code": legacy.get("root_scope_sku_code", ""),
                "retouch_requirement_id": legacy.get(
                    "root_retouch_requirement_id"
                ),
                "current_locator": None,
                "approved_locator": None,
                "provenance": {"kind": "test"},
            }
            oracle["roots"].append(root)
        else:
            if legacy.get("root_asset_type"):
                root["intrinsic_asset_type"] = legacy["root_asset_type"]
            root["scope_sku_code"] = legacy.get(
                "root_scope_sku_code", root["scope_sku_code"]
            )
        base = dict(oracle["versions"][0])
        base.update(
            {
                "task_asset_id": legacy["task_asset_id"],
                "task_id": legacy["task_id"],
                "root_asset_id": root_id,
                "stable_locator": legacy["stable_locator"],
                "intrinsic_asset_type": legacy.get("asset_type", "source"),
                "scope_sku_code": legacy.get("scope_sku_code", ""),
                "retouch_requirement_id": legacy.get(
                    "retouch_requirement_id"
                ),
                "content_sha256": legacy.get("whole_hash", ""),
                "expected_roles": [],
                "provenance": {
                    "kind": "test",
                    "a_binding_state": legacy.get(
                        "binding_state", "bound"
                    ),
                    "a_bound_role": legacy.get("bound_role", ""),
                },
            }
        )
        oracle["versions"].append(base)
        routes = oracle["route_expectations"]
        if legacy.get("detail_visible"):
            routes["detail_visible_locators"].append(base["stable_locator"])
        if legacy.get("list_current_version"):
            root["current_locator"] = base["stable_locator"]
            routes["current_locators"].append(base["stable_locator"])
            if root_id not in routes["list_root_ids"]:
                routes["list_root_ids"].append(root_id)
        if legacy.get("list_approved_version"):
            root["approved_locator"] = base["stable_locator"]
            routes["approved_locators"].append(base["stable_locator"])

    def set_manifest_task_contract(
        self, *, task_type: str | None = None, status: str | None = None
    ) -> None:
        rows = [
            json.loads(line)
            for line in self.manifest.read_text(encoding="utf-8").splitlines()
        ]
        for row in rows:
            if row["gate_name"] == "G01" and row["entity_key"] == "task:1":
                if task_type is not None:
                    row["detail_json"]["components"][1] = task_type
                if status is not None:
                    row["detail_json"]["components"][2] = status
                row["expected_hash"] = api.component_sha256(
                    row["detail_json"]["components"]
                )
        self.manifest.write_text(
            "".join(json.dumps(row) + "\n" for row in rows),
            encoding="utf-8",
        )
        self.rebind_api_oracle()

    def set_manifest_task_status(self, status: str) -> None:
        self.set_manifest_task_contract(status=status)

    def rule(
        self,
        *,
        rule_id: str,
        route: str,
        direction: str,
        from_status: int,
        to_status: int,
        operations: list[dict],
        reason: str = "approved test difference",
        identity: str | None = None,
    ) -> dict:
        value = {
            "rule_id": rule_id,
            "route": route,
            "direction": direction,
            "from_status": from_status,
            "to_status": to_status,
            "reason": reason,
            "reason_sha256": digest(reason.encode()),
            "operations": operations,
        }
        if identity is not None:
            value["identity"] = identity
        value["rule_sha256"] = digest(api.canonical(value))
        return value

    @staticmethod
    def requester(base: str, path: str, headers: dict[str, str]) -> api.HttpResult:
        assert headers["Authorization"] == "Bearer TOP-SECRET"
        if "warehouse-receive" in path:
            return result(404, {"code": "not_found"})
        if "/revisions" in path:
            return result(
                200,
                {
                    "data": {
                        "items": [
                            {
                                "id": 200,
                                "group_id": 10,
                                "revision_no": 2,
                                "status": "submitted",
                                "mode": "single",
                                "source_stage": "design",
                                "created_by": 1,
                                "reason": "",
                                "legacy_migration": False,
                                "items": [],
                                "references": [],
                                "created_at": "2026-01-01T00:00:00Z",
                            },
                            {
                                "id": 100,
                                "group_id": 10,
                                "revision_no": 1,
                                "status": "submitted",
                                "mode": "single",
                                "source_stage": "design",
                                "created_by": 1,
                                "reason": "",
                                "legacy_migration": False,
                                "items": [],
                                "references": [],
                                "created_at": "2026-01-01T00:00:00Z",
                            },
                        ],
                        "working_revision_id": 200,
                        "page": 1,
                        "page_size": 200,
                        "total": 2,
                    }
                },
            )
        if "/task-assets/" in path:
            return result(
                200,
                {
                    "data": {
                        "download_mode": "proxy",
                        "download_url": (
                            "/v1/assets/files/tasks/RW-1/source.psd"
                        ),
                        "access_hint": "authenticated_proxy",
                        "preview_available": True,
                        "filename": "source.psd",
                        "file_size": 10,
                        "mime_type": "image/png",
                    }
                },
            )
        if path == "/v1/resource-groups/10":
            return result(
                200,
                {
                    "id": 10,
                    "task_id": 1,
                    "scope_kind": "task",
                    "source_task_asset_id": 20,
                    "lock_version": 0,
                    "migration_incomplete": False,
                    "working_revision_id": 200,
                    "working_revision": {
                        "id": 200,
                        "group_id": 10,
                        "revision_no": 2,
                        "status": "submitted",
                        "mode": "single",
                        "source_stage": "design",
                        "created_by": 1,
                        "reason": "",
                        "legacy_migration": False,
                        "items": [],
                        "references": [],
                        "created_at": "2026-01-01T00:00:00Z",
                    },
                    "created_at": "2026-01-01T00:00:00Z",
                    "updated_at": "2026-01-01T00:00:00Z",
                },
            )
        if path.endswith("/resource-bundle"):
            return result(
                200,
                {
                    "task_id": 1,
                    "workflow_revision": 1,
                    "groups": [
                        {
                            "id": 10,
                            "task_id": 1,
                            "scope_kind": "task",
                            "source_task_asset_id": 20,
                            "lock_version": 0,
                            "migration_incomplete": False,
                            "working_revision_id": 200,
                            "working_revision": {
                                "id": 200,
                                "group_id": 10,
                                "revision_no": 2,
                                "status": "submitted",
                                "mode": "single",
                                "source_stage": "design",
                                "created_by": 1,
                                "reason": "",
                                "legacy_migration": False,
                                "items": [],
                                "references": [],
                                "created_at": "2026-01-01T00:00:00Z",
                            },
                            "created_at": "2026-01-01T00:00:00Z",
                            "updated_at": "2026-01-01T00:00:00Z",
                        }
                    ]
                },
            )
        if path.endswith("/assets"):
            return result(
                200,
                {
                    "data": [
                        {
                            "id": 101,
                            "task_id": 1,
                            "asset_type": "source",
                            "current_version_id": 20,
                            "current_version": {
                            "id": 20,
                            "task_id": 1,
                                "asset_id": 101,
                                "asset_type": "source",
                                "scope_sku_code": "",
                                "retouch_requirement_id": None,
                                "version_no": 1,
                                "storage_key": "tasks/RW-1/source.psd",
                                "file_hash": "a" * 64,
                                "file_size": 10,
                                "mime_type": "image/png",
                                "flow_review_status": "",
                                "usable_state": "not_applicable",
                                "approved_at": None,
                                "approved_by": None,
                                "current_version_role": "current_version",
                            },
                        }
                    ]
                },
            )
        task = {
            "id": 1,
            "task_type": "design",
            "task_status": "PendingAudit",
            "workflow_revision": 1,
            "workflow_contract_version": 2,
            "allowed_actions": ["view"],
        }
        if path.endswith("/detail"):
            return result(
                200,
                {
                    "task": task,
                    "design_sub_status": "not_required",
                    "asset_versions": [
                        {
                            "id": 20,
                            "task_id": 1,
                            "asset_id": 101,
                            "asset_type": "source",
                            "scope_sku_code": "",
                            "retouch_requirement_id": None,
                            "version_no": 1,
                            "storage_key": "tasks/RW-1/source.psd",
                            "file_hash": "a" * 64,
                            "file_size": 10,
                            "mime_type": "image/png",
                            "flow_review_status": "",
                            "usable_state": "not_applicable",
                            "approved_at": None,
                            "approved_by": None,
                        }
                    ],
                },
            )
        return result(200, task)

    def run_compare(
        self,
        requester=None,
        request_metrics=None,
        workers=16,
        downloader=None,
        simulate_scope=True,
    ) -> dict:
        provided_requester = requester or self.requester

        def scoped_requester(base, path, headers):
            if not simulate_scope:
                return provided_requester(base, path, headers)
            identity = headers.get("X-Test-Identity", "admin")
            if "warehouse-receive" in path and identity != "admin":
                return result(404, {"code": "not_found"})
            if identity in {"view-outside", "no-view"}:
                return result(403, {"code": "permission_denied"})
            if (
                identity == "view-inside"
                and "/v1/task-assets/" in path
                and path.endswith("/download")
            ):
                return result(403, {"code": "permission_denied"})
            response = provided_requester(base, path, headers)
            if (
                identity == "view-inside"
                and base.endswith(("8102", "8103"))
                and response.status == 200
                and "/v1/task-assets/" not in path
            ):
                # The formal view-only fixture can read its assigned task but
                # cannot download its files.  Keep the test matrix aligned
                # with the real B read model before exercising A/B semantics.
                return result(
                    response.status,
                    api.access_restricted_projection(response.body),
                )
            return response

        kwargs = dict(
            matrix_path=self.matrix,
            task_ids_path=self.tasks,
            rules_path=self.rules,
            manifest_path=self.manifest,
            api_oracle_path=self.api_oracle,
            run_id=self.run_id,
            requester=scoped_requester,
            workers=workers,
            request_metrics=request_metrics,
        )
        if downloader is not None:
            kwargs["downloader"] = downloader
        return api.compare(**kwargs)

    def make_runner(self) -> api.Runner:
        urls, _, resolved, retired = api.load_matrix(self.matrix)
        identities = [
            {"id": item["id"], "role": item["role"]}
            for item in json.loads(
                self.matrix.read_text(encoding="utf-8")
            )["identities"]
        ]
        asset_oracle = api.load_asset_identity_map(
            self.api_oracle,
            self.run_id,
            digest(self.manifest.read_bytes()),
        )
        expectations = api.load_manifest_expectations(
            self.manifest,
            self.run_id,
            expected_mapping_sha256=self.mapping_sha256,
            expected_baseline_attestation_sha256=(
                self.baseline_attestation_sha256
            ),
        )
        return api.Runner(
            urls,
            identities,
            resolved,
            [],
            retired,
            expectations,
            asset_oracle,
            self.requester,
        )

    def test_four_combo_get_only_pass_and_secret_not_in_evidence(self) -> None:
        calls: list[tuple[str, str]] = []

        def requester(base, path, headers):
            calls.append((base, path))
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        self.assertEqual("PASS", evidence["status"], evidence["violations"])
        self.assertEqual(4, len(evidence["combination_matrix"]))
        self.assertEqual(1, evidence["task_count"])
        self.assertEqual(1, evidence["group_count"])
        self.assertEqual(1, evidence["task_asset_count"])
        self.assertTrue(all(path.startswith("/") for _, path in calls))
        self.assertNotIn("TOP-SECRET", json.dumps(evidence))
        self.assertEqual(
            evidence["evidence_sha256"],
            digest(api.canonical({k: v for k, v in evidence.items() if k != "evidence_sha256"})),
        )

    def test_deterministic_evidence(self) -> None:
        first = self.run_compare()
        second = self.run_compare()
        self.assertEqual(api.canonical(first), api.canonical(second))

    def test_matrix_requires_all_three_permission_roles(self) -> None:
        document = json.loads(self.matrix.read_text(encoding="utf-8"))
        document["identities"] = [
            item
            for item in document["identities"]
            if item["role"] != "no_view"
        ]
        self.matrix.write_text(json.dumps(document), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "admin, view_only, and no_view"):
            api.load_matrix(self.matrix)

    def test_view_only_200_recursively_rejects_download_url(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if (
                headers.get("X-Test-Identity") == "view-inside"
                and base.endswith(("8102", "8103"))
                and path == "/v1/tasks/1"
            ):
                return result(
                    200,
                    {"data": {"nested": [{"download_url": ""}]}},
                )
            return value

        evidence = self.run_compare(requester, simulate_scope=False)
        self.assertIn(
            "api.view_only_download_url_exposed",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_view_only_url_check_is_candidate_b_only(self) -> None:
        runner = self.make_runner()
        runner.observation(
            "external_external_a",
            "view-inside",
            "/v1/tasks/{task_id}",
            "task:1:/v1/tasks/{task_id}",
            result(200, {"data": {"download_url": "/legacy/file"}}),
        )
        self.assertFalse(
            any(
                item["violation_code"]
                == "api.view_only_download_url_exposed"
                for item in runner.violations
            )
        )
        runner.observation(
            "dev_dev_b",
            "view-inside",
            "/v1/tasks/{task_id}",
            "task:1:/v1/tasks/{task_id}",
            result(
                200,
                {"data": {"download_url": "/v1/task-assets/20/download"}},
            ),
        )
        self.assertTrue(
            any(
                item["violation_code"]
                == "api.view_only_download_url_exposed"
                for item in runner.violations
            )
        )

    def test_view_only_rejects_locator_metadata_before_projection(self) -> None:
        runner = self.make_runner()
        runner.observation(
            "dev_dev_b",
            "view-inside",
            "/v1/tasks/{task_id}",
            "task:1:/v1/tasks/{task_id}",
            result(
                200,
                {
                    "data": {
                        "asset_id": "ref-1",
                        "storage_key": "",
                        "public_download_allowed": False,
                    }
                },
            ),
        )
        self.assertTrue(
            any(
                item["violation_code"]
                == "api.view_only_access_metadata_exposed"
                for item in runner.violations
            )
        )

    def test_view_only_rejects_all_removed_asset_locator_fields(self) -> None:
        for field in (
            "file_path",
            "object_key",
            "signed_url",
            "presigned_url",
        ):
            with self.subTest(field=field):
                runner = self.make_runner()
                runner.observation(
                    "dev_dev_b",
                    "view-inside",
                    "/v1/tasks/{task_id}/detail",
                    "task:1:/v1/tasks/{task_id}/detail",
                    result(200, {"data": {field: ""}}),
                )
                self.assertTrue(
                    any(
                        item["violation_code"]
                        == "api.view_only_access_metadata_exposed"
                        for item in runner.violations
                    )
                )
                self.assertNotIn(
                    field,
                    api.access_restricted_projection({"data": {field: ""}})[
                        "data"
                    ],
                )

    def test_view_only_rejects_legacy_reference_image_locators(self) -> None:
        locators = (
            "https://cdn.example.invalid/reference.png",
            "ftp://cdn.example.invalid/reference.png",
            "data:image/png;base64,AAAA",
            "blob:https://example.invalid/id",
            "file:///tmp/reference.png",
            "s3://bucket/reference.png",
            "oss://bucket/reference.png",
            "gs://bucket/reference.png",
            "/srv/references/reference.png",
            r"C:\references\reference.png",
            "../references/reference.png",
            "references/reference.png",
            "foo?bar",
            "https%3A%2F%2Fcdn.example.invalid%2Fencoded.png",
            "invalid%2",
        )
        for locator in locators:
            with self.subTest(locator=locator):
                issues = api.access_restricted_issues(
                    {"reference_images_json": json.dumps([locator])}
                )
                self.assertEqual(
                    [
                        (
                            "$.reference_images_json#json[0]",
                            "reference_image_locator",
                        )
                    ],
                    issues,
                )
                self.assertEqual(
                    [],
                    api.access_restricted_projection(
                        {"reference_images_json": json.dumps([locator])}
                    )["reference_images_json"],
                )
        runner = self.make_runner()
        runner.observation(
            "dev_dev_b",
            "view-inside",
            "/v1/tasks/{task_id}/detail",
            "task:1:/v1/tasks/{task_id}/detail",
            result(
                200,
                {"reference_images_json": json.dumps(list(locators))},
            ),
        )
        self.assertIn(
            "api.view_only_access_metadata_exposed",
            {item["violation_code"] for item in runner.violations},
        )

    def test_view_only_preserves_plain_legacy_reference_identifiers(self) -> None:
        values = [
            "reference-asset-123",
            "asset:123",
            "550e8400-e29b-41d4-a716-446655440000",
            "reference.final.png",
            "设计参考图 01.jpg",
            "  trimmed-identifier  ",
            "",
        ]
        encoded = json.dumps(values, ensure_ascii=False)
        projected_values = [
            "reference-asset-123",
            "asset:123",
            "550e8400-e29b-41d4-a716-446655440000",
            "reference.final.png",
            "设计参考图 01.jpg",
            "trimmed-identifier",
        ]
        self.assertEqual(
            [],
            api.access_restricted_issues({"reference_images_json": encoded}),
        )
        self.assertEqual(
            projected_values,
            api.access_restricted_projection(
                {"reference_images_json": encoded}
            )["reference_images_json"],
        )
        runner = self.make_runner()
        runner.observation(
            "dev_dev_b",
            "view-inside",
            "/v1/tasks/{task_id}/detail",
            "task:1:/v1/tasks/{task_id}/detail",
            result(200, {"reference_images_json": encoded}),
        )
        self.assertFalse(runner.violations)

    def test_view_only_invalid_legacy_reference_images_json_fails_closed(
        self,
    ) -> None:
        for value in (
            '{"not":"an-array"}',
            json.dumps([{"url": "hidden"}]),
            ["not-encoded-json"],
        ):
            with self.subTest(value=value):
                self.assertEqual(
                    [
                        (
                            "$.reference_images_json",
                            "invalid_reference_images_json",
                        )
                    ],
                    api.access_restricted_issues(
                        {"reference_images_json": value}
                    ),
                )
                self.assertEqual(
                    [],
                    api.access_restricted_projection(
                        {"reference_images_json": value}
                    )["reference_images_json"],
                )

    def test_view_only_invalid_embedded_reference_json_fails_closed(self) -> None:
        runner = self.make_runner()
        runner.observation(
            "dev_dev_b",
            "view-inside",
            "/v1/tasks/{task_id}/detail",
            "task:1:/v1/tasks/{task_id}/detail",
            result(
                200,
                {
                    "data": {
                        "task_detail": {
                            "reference_file_refs_json": '{"asset_id":"ref-1"'
                        }
                    }
                },
            ),
        )
        self.assertTrue(
            any(
                item["violation_code"]
                == "api.view_only_access_projection_invalid"
                for item in runner.violations
            )
        )

    def test_access_projection_preserves_business_identity_and_pointer(self) -> None:
        value = {
            "asset_id": 101,
            "task_id": 1,
            "current_version_id": 20,
            "scope_sku_code": "SKU-1",
            "file_hash": "a" * 64,
            "storage_key": "tasks/1/source.psd",
            "download_url": "/v1/task-assets/20/download",
            "public_download_allowed": True,
            "access_hint": "object_key=tasks/1/source.psd",
            "reference_file_refs_json": json.dumps(
                [
                    {
                        "asset_id": "ref-1",
                        "storage_key": "tasks/1/reference.png",
                        "download_url": "/v1/assets/files/reference.png",
                        "source": "task_reference_upload",
                    }
                ]
            ),
        }
        projected = api.access_restricted_projection(value)
        self.assertEqual(101, projected["asset_id"])
        self.assertEqual(20, projected["current_version_id"])
        self.assertEqual("SKU-1", projected["scope_sku_code"])
        self.assertEqual("a" * 64, projected["file_hash"])
        self.assertNotIn("storage_key", projected)
        self.assertNotIn("download_url", projected)
        self.assertNotIn("public_download_allowed", projected)
        self.assertNotIn("access_hint", projected)
        self.assertEqual(
            [
                {
                    "asset_id": "ref-1",
                    "source": "task_reference_upload",
                }
            ],
            projected["reference_file_refs_json"],
        )
        tampered = json.loads(json.dumps(value))
        tampered["current_version_id"] = 21
        self.assertNotEqual(
            projected,
            api.access_restricted_projection(tampered),
        )

    def test_view_only_asset_download_must_not_return_200(self) -> None:
        evidence = self.run_compare(simulate_scope=False)
        self.assertIn(
            "api.view_only_download_granted",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_no_view_b_routes_must_not_return_200(self) -> None:
        evidence = self.run_compare(simulate_scope=False)
        self.assertIn(
            "api.no_view_access_granted",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_view_only_requires_b_200_and_403_task_observations(self) -> None:
        document = json.loads(self.matrix.read_text(encoding="utf-8"))
        document["identities"] = [
            item
            for item in document["identities"]
            if item["id"] != "view-outside"
        ]
        self.matrix.write_text(json.dumps(document), encoding="utf-8")
        evidence = self.run_compare()
        violations = [
            item
            for item in evidence["violations"]
            if item["violation_code"] == "api.view_only_scope_unproven"
        ]
        self.assertEqual(2, len(violations))
        self.assertTrue(all("returned 403" in item["detail"] for item in violations))

    def test_view_only_denied_task_cannot_read_sibling_surfaces(self) -> None:
        runner = self.make_runner()
        task_entity = "task:1:/v1/tasks/{task_id}"
        detail_entity = "task:1:/v1/tasks/{task_id}/detail"
        runner.results[
            (
                "dev_dev_b",
                "view-inside",
                "/v1/tasks/{task_id}",
                task_entity,
            )
        ] = result(403, {"error": {"code": "forbidden"}})
        runner.results[
            (
                "dev_dev_b",
                "view-inside",
                "/v1/tasks/{task_id}/detail",
                detail_entity,
            )
        ] = result(200, {"data": {"id": 1}})
        runner.validate_identity_coverage()
        self.assertTrue(
            any(
                item["violation_code"] == "api.view_only_scope_bypass"
                and item["entity_key"] == detail_entity
                for item in runner.violations
            )
        )

    def test_schema_v1_oracle_is_rejected(self) -> None:
        document = json.loads(self.api_oracle.read_text(encoding="utf-8"))
        document["schema_version"] = 1
        document.pop("evidence_sha256")
        document["evidence_sha256"] = digest(api.canonical(document))
        self.api_oracle.write_text(json.dumps(document), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "binding or evidence"):
            self.run_compare()

    def test_api_oracle_must_bind_baseline_provenance(self) -> None:
        document = json.loads(self.api_oracle.read_text(encoding="utf-8"))
        document["inputs"].pop("snapshot_verdict_sha256")
        document.pop("evidence_sha256")
        document["evidence_sha256"] = digest(api.canonical(document))
        self.api_oracle.write_text(json.dumps(document), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "input bindings differ"):
            self.run_compare()

    def test_recovery_download_bytes_are_hash_verified(self) -> None:
        raw = b"recovered-object"
        content_hash = digest(raw)
        document = json.loads(self.api_oracle.read_text(encoding="utf-8"))
        version = document["versions"][0]
        version["content_sha256"] = content_hash
        version["size"] = len(raw)
        version["provenance"] = {"kind": "recovery_receipt"}
        document.pop("evidence_sha256")
        document["evidence_sha256"] = digest(api.canonical(document))
        self.api_oracle.write_text(json.dumps(document), encoding="utf-8")

        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if path in {"/v1/tasks/1/detail", "/v1/tasks/1/assets"}:
                body = json.loads(json.dumps(value.body))
                if path.endswith("/detail"):
                    target = body["asset_versions"][0]
                else:
                    target = body["data"][0]["current_version"]
                target["file_hash"] = content_hash
                target["file_size"] = len(raw)
                return result(200, body)
            if "/v1/task-assets/" in path:
                body = json.loads(json.dumps(value.body))
                body["data"]["file_size"] = len(raw)
                return result(200, body)
            return value

        calls: list[str] = []

        def downloader(base, path, headers):
            calls.append(base + path)
            return raw

        evidence = self.run_compare(requester, downloader=downloader)
        self.assertEqual("PASS", evidence["status"], evidence["violations"])
        self.assertEqual(2, len(calls))

        blocked = self.run_compare(
            requester,
            downloader=lambda base, path, headers: b"wrong",
        )
        self.assertIn(
            "api.recovery_download_hash_mismatch",
            {row["violation_code"] for row in blocked["violations"]},
        )

    def test_http_download_resolves_relative_controlled_metadata(self) -> None:
        metadata = api.canonical(
            {"data": {"download_url": "/v1/assets/files/recovered.bin"}}
        )
        metadata_opener = mock.Mock()
        metadata_opener.open.return_value = FakeHTTPResponse(metadata)
        download_opener = mock.Mock()
        download_opener.open.return_value = FakeHTTPResponse(b"recovered")
        headers = {"Authorization": "Bearer test"}
        with mock.patch.object(
            api.urllib.request,
            "build_opener",
            side_effect=[metadata_opener, download_opener],
        ):
            raw = api.http_download(
                "http://127.0.0.1:18202",
                "/v1/task-assets/1/download",
                headers,
            )
        self.assertEqual(b"recovered", raw)
        request = download_opener.open.call_args.args[0]
        self.assertEqual(
            "http://127.0.0.1:18202/v1/assets/files/recovered.bin",
            request.full_url,
        )
        self.assertEqual(
            "Bearer test", dict(request.header_items()).get("Authorization")
        )

    def test_http_download_does_not_forward_auth_to_absolute_url(self) -> None:
        metadata = api.canonical(
            {"data": {"download_url": "https://objects.test/recovered.bin"}}
        )
        metadata_opener = mock.Mock()
        metadata_opener.open.return_value = FakeHTTPResponse(metadata)
        download_opener = mock.Mock()
        download_opener.open.return_value = FakeHTTPResponse(b"recovered")
        with (
            mock.patch.object(
                api.urllib.request,
                "build_opener",
                side_effect=[metadata_opener, download_opener],
            ),
            mock.patch.dict(
                os.environ,
                {
                    "AB_DOWNLOAD_ALLOWED_HOSTS": (
                        "https://objects.test:443"
                    )
                },
            ),
        ):
            raw = api.http_download(
                "http://127.0.0.1:18202",
                "/v1/task-assets/1/download",
                {"Authorization": "Bearer secret"},
            )
        self.assertEqual(b"recovered", raw)
        request = download_opener.open.call_args.args[0]
        self.assertEqual(
            "https://objects.test/recovered.bin", request.full_url
        )
        self.assertNotIn("Authorization", dict(request.header_items()))

    def test_http_download_rejects_non_allowlisted_absolute_url(self) -> None:
        metadata = api.canonical(
            {"data": {"download_url": "https://objects.test/recovered.bin"}}
        )
        opener = mock.Mock()
        opener.open.return_value = FakeHTTPResponse(metadata)
        with (
            mock.patch.object(
                api.urllib.request, "build_opener", return_value=opener
            ),
            mock.patch.dict(os.environ, {}, clear=True),
            self.assertRaisesRegex(ValueError, "non-allowlisted origin"),
        ):
            api.http_download(
                "http://127.0.0.1:18202",
                "/v1/task-assets/1/download",
                {"Authorization": "Bearer secret"},
            )

    def test_download_allowlist_requires_scheme_and_explicit_port(self) -> None:
        with (
            mock.patch.dict(
                os.environ,
                {"AB_DOWNLOAD_ALLOWED_HOSTS": "objects.test"},
            ),
            self.assertRaisesRegex(ValueError, "scheme and explicit port"),
        ):
            api.download_allowed_hosts()

    def test_http_download_rejects_allowlisted_host_on_another_port(self) -> None:
        metadata = api.canonical(
            {
                "data": {
                    "download_url": (
                        "https://objects.test:444/recovered.bin"
                    )
                }
            }
        )
        opener = mock.Mock()
        opener.open.return_value = FakeHTTPResponse(metadata)
        with (
            mock.patch.object(
                api.urllib.request, "build_opener", return_value=opener
            ),
            mock.patch.dict(
                os.environ,
                {"AB_DOWNLOAD_ALLOWED_HOSTS": "https://objects.test:443"},
            ),
            self.assertRaisesRegex(ValueError, "non-allowlisted origin"),
        ):
            api.http_download(
                "http://127.0.0.1:18202",
                "/v1/task-assets/1/download",
                {"Authorization": "Bearer secret"},
            )

    def test_download_allowlist_is_canonical_and_auditable(self) -> None:
        with mock.patch.dict(
            os.environ,
            {
                "AB_DOWNLOAD_ALLOWED_HOSTS": (
                    " HTTPS://B.objects.test:443,"
                    "http://a.objects.test:8080,"
                    "https://b.objects.test:443 "
                )
            },
        ):
            self.assertEqual(
                (
                    "http://a.objects.test:8080",
                    "https://b.objects.test:443",
                ),
                api.download_allowed_hosts(),
            )

    def test_asset_list_is_bound_to_requested_task_and_exact_root_set(
        self,
    ) -> None:
        runner = self.make_runner()
        body = self.requester(
            "http://127.0.0.1:8102",
            "/v1/tasks/1/assets",
            {"Authorization": "Bearer TOP-SECRET"},
        ).body
        runner.validate_asset_list_oracle(
            "dev_dev_b", "admin", "2", body
        )
        runner.validate_asset_list_oracle(
            "dev_dev_b", "admin", "1", {"data": []}
        )
        details = [item["detail"] for item in runner.violations]
        self.assertTrue(
            any("requested task" in detail for detail in details),
            details,
        )
        self.assertTrue(
            any("root set differs" in detail for detail in details),
            details,
        )

    def test_group_detail_is_bound_to_requested_group_id(self) -> None:
        runner = self.make_runner()
        group = api.unwrap_data(
            self.requester(
                "http://127.0.0.1:8102",
                "/v1/resource-groups/10",
                {"Authorization": "Bearer TOP-SECRET"},
            ).body
        )
        runner.validate_group_oracle(
            "dev_dev_b",
            "admin",
            group,
            requested_group_id="999",
        )
        self.assertTrue(
            any(
                "requested group id" in item["detail"]
                for item in runner.violations
            ),
            runner.violations,
        )

    def test_task_asset_routes_reject_wrong_requested_asset_metadata(
        self,
    ) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if (
                base.endswith(("8102", "8103"))
                and headers.get("X-Test-Identity") == "admin"
                and path == "/v1/task-assets/20/download"
            ):
                return result(
                    200,
                    {
                        "data": {
                            "download_mode": "proxy",
                            "download_url": (
                                "/v1/assets/files/tasks/RW-1/other.psd"
                            ),
                            "access_hint": "authenticated_proxy",
                            "preview_available": True,
                            "filename": "other.psd",
                            "file_size": 999,
                            "mime_type": "application/octet-stream",
                        }
                    },
                )
            return value

        evidence = self.run_compare(requester)
        self.assertIn(
            "api.task_asset_metadata_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_preview_metadata_may_describe_exact_version_derivative(self) -> None:
        runner = self.make_runner()
        runner.task_asset_metadata[("dev_dev_b", "admin", "20")] = {
            "has_original_filename": True,
            "original_filename": "source.psd",
        }
        preview_body = {
            "data": {
                "download_mode": "proxy",
                "download_url": "/v1/assets/files/tasks/RW-1/preview.webp",
                "access_hint": "controlled preview",
                "preview_available": True,
                "filename": "preview.webp",
                "file_size": 2,
                "mime_type": "image/webp",
            }
        }
        runner.validate_task_asset_access_oracle(
            "dev_dev_b",
            "admin",
            "20",
            "/v1/task-assets/{task_asset_id}/preview",
            preview_body,
        )
        self.assertEqual([], runner.violations)

        runner.validate_task_asset_access_oracle(
            "dev_dev_b",
            "admin",
            "20",
            "/v1/task-assets/{task_asset_id}/download",
            preview_body,
        )
        self.assertTrue(
            any(
                item["violation_code"]
                == "api.task_asset_metadata_mismatch"
                for item in runner.violations
            )
        )

    def test_historical_unavailable_detail_suppresses_file_access(self) -> None:
        runner = self.make_runner()
        runner.asset_identity_oracle["20"][
            "content_availability"
        ] = "historical_unavailable"
        version = {
            "id": 20,
            "task_id": 1,
            "asset_id": 101,
            "asset_type": "source",
            "scope_sku_code": "",
            "retouch_requirement_id": None,
            "storage_key": "",
            "file_hash": "",
            "file_size": 10,
            "mime_type": "image/png",
            "flow_review_status": "",
            "usable_state": "not_applicable",
            "approved_at": None,
            "approved_by": None,
            "download_url": None,
            "public_download_allowed": False,
            "access_hint": "historical object is unavailable",
        }
        runner.validate_version_identity(
            "dev_dev_b",
            "admin",
            "task:1:/v1/tasks/{task_id}/detail",
            version,
            surface="detail",
        )
        self.assertEqual([], runner.violations)

        omitted = {
            key: child
            for key, child in version.items()
            if key
            not in {
                "download_url",
                "file_hash",
                "storage_key",
            }
        }
        runner.validate_version_identity(
            "dev_dev_b",
            "admin",
            "task:1:/v1/tasks/{task_id}/detail",
            omitted,
            surface="detail",
        )
        self.assertEqual([], runner.violations)

        exposed = dict(version)
        exposed["storage_key"] = "legacy/raw/object.psd"
        runner.validate_version_identity(
            "dev_dev_b",
            "admin",
            "task:1:/v1/tasks/{task_id}/detail",
            exposed,
            surface="detail",
        )
        self.assertTrue(
            any(
                "historical-unavailable metadata exposes file access"
                in item["detail"]
                for item in runner.violations
            )
        )
        runner.violations.clear()
        exposed_hint = dict(version)
        exposed_hint["access_hint"] = "historical path: legacy/raw/object.psd"
        runner.validate_version_identity(
            "dev_dev_b",
            "admin",
            "task:1:/v1/tasks/{task_id}/detail",
            exposed_hint,
            surface="detail",
        )
        self.assertTrue(
            any(
                "historical-unavailable metadata exposes file access"
                in item["detail"]
                for item in runner.violations
            )
        )

    def test_view_only_redacted_version_checks_visible_oracle_identity(self) -> None:
        runner = self.make_runner()
        version = {
            "id": 20,
            "task_id": 1,
            "asset_id": 101,
            "asset_type": "source",
            "scope_sku_code": "",
            "retouch_requirement_id": None,
            "file_hash": "a" * 64,
            "file_size": 10,
            "mime_type": "image/png",
            "flow_review_status": "",
            "usable_state": "not_applicable",
            "approved_at": None,
            "approved_by": None,
            "public_download_allowed": False,
        }
        resolved = runner.validate_version_identity(
            "dev_dev_b",
            "view-inside",
            "task:1:/v1/tasks/{task_id}/detail",
            version,
            surface="detail",
            coverage_kind="detail_asset",
        )
        self.assertIsNotNone(resolved)
        self.assertEqual([], runner.violations)
        self.assertEqual(
            {"20"},
            runner.oracle_coverage[
                ("dev_dev_b", "view-inside", "detail_asset")
            ],
        )

        tampered = dict(version)
        tampered["file_size"] = 11
        runner.validate_version_identity(
            "dev_dev_b",
            "view-inside",
            "task:1:/v1/tasks/{task_id}/detail",
            tampered,
            surface="detail",
        )
        self.assertTrue(
            any(
                "identity differs" in item["detail"]
                for item in runner.violations
            )
        )

    def test_admin_normal_asset_still_requires_storage_identity(self) -> None:
        runner = self.make_runner()
        version = {
            "id": 20,
            "task_id": 1,
            "asset_id": 101,
            "asset_type": "source",
            "scope_sku_code": "",
            "retouch_requirement_id": None,
            "file_hash": "a" * 64,
            "file_size": 10,
            "mime_type": "image/png",
            "flow_review_status": "",
            "usable_state": "not_applicable",
            "approved_at": None,
            "approved_by": None,
        }
        runner.validate_version_identity(
            "dev_dev_b",
            "admin",
            "task:1:/v1/tasks/{task_id}/detail",
            version,
            surface="detail",
        )
        self.assertTrue(
            any(
                "identity fields have invalid types" in item["detail"]
                for item in runner.violations
            )
        )

    def test_nested_retouch_asset_identity_is_exact(self) -> None:
        nested = {
            "id": 20,
            "task_id": 1,
            "asset_id": 101,
            "asset_type": "source",
            "scope_sku_code": "",
            "retouch_requirement_id": None,
            "version_no": 1,
            "storage_key": "tasks/RW-1/source.psd",
            "file_hash": "a" * 64,
            "file_size": 10,
            "mime_type": "image/png",
        }

        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if path != "/v1/tasks/1/detail":
                return value
            body = json.loads(json.dumps(value.body))
            value_copy = dict(nested)
            if base.endswith(("8102", "8103")):
                value_copy["mime_type"] = "image/jpeg"
            body["retouch_requirements"] = [
                {"id": 1, "source_assets": [{"current_version": value_copy}]}
            ]
            return result(200, body)

        evidence = self.run_compare(requester)
        self.assertIn(
            "api.manifest_oracle_mismatch",
            {row["violation_code"] for row in evidence["violations"]},
        )

    def test_any_5xx_is_hard_failure(self) -> None:
        def requester(base, path, headers):
            if base.endswith("8102") and path == "/v1/tasks/1/detail":
                return result(503, {"error": "down"})
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn("api.server_error", {v["violation_code"] for v in evidence["violations"]})

    def test_permission_widening_is_not_normalizable(self) -> None:
        def requester(base, path, headers):
            if path.endswith("/preview"):
                return result(403 if base.endswith("8101") else 200, {"ok": True})
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        self.assertIn(
            "api.permission_widened",
            {v["violation_code"] for v in evidence["violations"]},
        )

    def test_candidate_permission_tightening_is_not_mislabeled_as_widening(self) -> None:
        def requester(base, path, headers):
            if path.endswith("/preview"):
                # Both B combinations deny while both A combinations allow.
                return result(
                    403 if base.endswith(("8102", "8103")) else 200,
                    {"ok": True},
                )
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        codes = {v["violation_code"] for v in evidence["violations"]}
        self.assertNotIn("api.permission_widened", codes)
        # Tightening is still an unexplained compatibility difference until an
        # exact approved status rule exists.
        self.assertIn("api.asset_lost", codes)

    def test_exact_rule_accepts_tightening_but_never_reverse_widening(
        self,
    ) -> None:
        runner = self.make_runner()
        runner.rules = [
            self.rule(
                rule_id="reviewed-org-scope-tightening",
                route="/v1/tasks/{task_id}",
                direction="external_external_a->dev_dev_b",
                from_status=200,
                to_status=403,
                operations=[],
                reason=(
                    "reviewed mapping aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
                    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa approves stable org scope"
                ),
            )
        ]
        entity = "task:1:/v1/tasks/{task_id}"
        runner.results[
            (
                "external_external_a",
                "view-inside",
                "/v1/tasks/{task_id}",
                entity,
            )
        ] = result(200, {"data": {"id": 1}})
        runner.results[
            (
                "dev_dev_b",
                "view-inside",
                "/v1/tasks/{task_id}",
                entity,
            )
        ] = result(403, {"error": {"code": "forbidden"}})
        runner.compare_result(
            "/v1/tasks/{task_id}",
            entity,
            "view-inside",
            "external_external_a",
            "dev_dev_b",
        )
        self.assertEqual([], runner.violations)
        self.assertEqual(
            {"reviewed-org-scope-tightening"},
            runner.used_rules,
        )

        reverse = self.make_runner()
        reverse.rules = runner.rules
        reverse.results[
            (
                "external_external_a",
                "view-inside",
                "/v1/tasks/{task_id}",
                entity,
            )
        ] = result(403, {"error": {"code": "forbidden"}})
        reverse.results[
            (
                "dev_dev_b",
                "view-inside",
                "/v1/tasks/{task_id}",
                entity,
            )
        ] = result(200, {"data": {"id": 1}})
        reverse.compare_result(
            "/v1/tasks/{task_id}",
            entity,
            "view-inside",
            "external_external_a",
            "dev_dev_b",
        )
        self.assertTrue(
            any(
                item["violation_code"] == "api.permission_widened"
                for item in reverse.violations
            )
        )

    def test_denied_error_envelope_copy_is_not_a_business_contract(self) -> None:
        runner = self.make_runner()
        entity = "task:1:/v1/tasks/{task_id}"
        runner.results[
            (
                "external_external_a",
                "no-view",
                "/v1/tasks/{task_id}",
                entity,
            )
        ] = result(403, {"code": "permission_denied"})
        runner.results[
            (
                "dev_dev_b",
                "no-view",
                "/v1/tasks/{task_id}",
                entity,
            )
        ] = result(
            403,
            {"error": {"code": "FORBIDDEN", "message": "forbidden"}},
        )
        runner.compare_result(
            "/v1/tasks/{task_id}",
            entity,
            "no-view",
            "external_external_a",
            "dev_dev_b",
        )
        self.assertEqual([], runner.violations)

    def test_permission_widening_is_detected_when_a_is_right_in_pair_order(self) -> None:
        def requester(base, path, headers):
            if path.endswith("/preview"):
                if base.endswith("8104"):  # dev frontend + external backend + A
                    return result(403, {"ok": False})
                return result(200, {"ok": True})
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        widening = [
            violation
            for violation in evidence["violations"]
            if violation["violation_code"] == "api.permission_widened"
        ]
        self.assertTrue(
            any(
                "dev_external_a->dev_dev_b 403->200" in violation["detail"]
                for violation in widening
            )
        )

    def test_asset_404_is_hard_failure(self) -> None:
        def requester(base, path, headers):
            if base.endswith("8102") and path.endswith("/download"):
                return result(404, {"code": "missing"})
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        self.assertIn(
            "api.expected_asset_missing",
            {v["violation_code"] for v in evidence["violations"]},
        )

    def test_preview_409_is_contract_valid_for_governed_asset(self) -> None:
        def requester(base, path, headers):
            if path.endswith("/preview"):
                return result(
                    409,
                    {
                        "error": {
                            "code": "preview_unavailable",
                            "trace_id": f"random-{base}",
                        }
                    },
                )
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        self.assertEqual("PASS", evidence["status"], evidence["violations"])

    def test_unbound_legacy_task_assets_are_not_probed_as_governed(self) -> None:
        calls: list[str] = []

        def requester(base, path, headers):
            calls.append(path)
            if path == "/v1/resource-groups/10":
                return result(
                    200,
                    {
                        "id": 10,
                        "task_id": 1,
                        "scope_kind": "task",
                        "migration_incomplete": False,
                        "working_revision_id": 200,
                        "working_revision": {
                            "id": 200,
                            "group_id": 10,
                            "revision_no": 2,
                            "status": "submitted",
                        },
                    },
                )
            if path.endswith("/resource-bundle"):
                return result(
                    200,
                    {
                        "task_id": 1,
                        "workflow_revision": 1,
                        "groups": [
                            {
                                "id": 10,
                                "task_id": 1,
                                "scope_kind": "task",
                                "migration_incomplete": False,
                                "working_revision_id": 200,
                                "working_revision": {
                                    "id": 200,
                                    "group_id": 10,
                                    "revision_no": 2,
                                    "status": "submitted",
                                },
                            }
                        ],
                        "legacy_asset": {
                            "id": 999,
                            "version_no": 1,
                            "asset_type": "delivery",
                        },
                    },
                )
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        self.assertEqual(0, evidence["task_asset_count"])
        self.assertGreater(evidence["legacy_task_asset_count"], 0)
        self.assertFalse(any("/v1/task-assets/" in path for path in calls))

    def test_unbound_legacy_asset_scope_drift_is_not_ignored(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if path == "/v1/tasks/1/detail":
                body = json.loads(json.dumps(value.body))
                body["asset_versions"] = [
                    {
                        "id": 999,
                        "asset_type": "delivery",
                        "whole_hash": "f" * 64,
                        "scope_sku_code": (
                            "SKU-B"
                            if base.endswith(("8102", "8103"))
                            else "SKU-A"
                        ),
                    }
                ]
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.manifest_oracle_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_oracle_approved_scope_assignment_preserves_legacy_semantics(
        self,
    ) -> None:
        oracle = json.loads(self.api_oracle.read_text(encoding="utf-8"))
        oracle["versions"][0]["scope_sku_code"] = "SKU-B"
        oracle["roots"][0]["scope_sku_code"] = "SKU-B"
        oracle.pop("evidence_sha256")
        oracle["evidence_sha256"] = digest(api.canonical(oracle))
        self.api_oracle.write_text(json.dumps(oracle), encoding="utf-8")

        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if path == "/v1/tasks/1/detail":
                body = json.loads(json.dumps(value.body))
                body["asset_versions"][0]["scope_sku_code"] = (
                    "SKU-B" if base.endswith(("8102", "8103")) else ""
                )
                return result(200, body)
            if path == "/v1/tasks/1/assets":
                body = json.loads(json.dumps(value.body))
                body["data"][0]["scope_sku_code"] = (
                    "SKU-B" if base.endswith(("8102", "8103")) else ""
                )
                body["data"][0]["current_version"]["scope_sku_code"] = (
                    "SKU-B" if base.endswith(("8102", "8103")) else ""
                )
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertEqual("PASS", evidence["status"])

    def test_wrong_b_asset_scope_fails_even_when_intrinsic_asset_is_preserved(
        self,
    ) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if base.endswith(("8102", "8103")) and path == "/v1/tasks/1/detail":
                body = json.loads(json.dumps(value.body))
                body["asset_versions"][0]["scope_sku_code"] = "WRONG-SCOPE"
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.manifest_oracle_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_detail_retouch_scope_cannot_disagree_with_asset_list(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if base.endswith(("8102", "8103")) and path == "/v1/tasks/1/detail":
                body = json.loads(json.dumps(value.body))
                body["asset_versions"][0]["retouch_requirement_id"] = 999
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.manifest_oracle_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_historical_detail_retouch_scope_is_oracle_validated(self) -> None:
        oracle = json.loads(self.api_oracle.read_text(encoding="utf-8"))
        self.append_legacy_oracle_asset(
            oracle,
            {
                "task_asset_id": 18,
                "task_id": 1,
                "root_asset_id": 101,
                "stable_locator": "asset:101:historical-ref",
                "asset_type": "delivery",
                "whole_hash": "",
                "binding_state": "legacy",
                "bound_role": "NULL",
                "scope_sku_code": "",
                "retouch_requirement_id": None,
                "root_asset_type": "source",
                "root_scope_sku_code": "",
                "root_retouch_requirement_id": None,
                "detail_visible": True,
                "list_current_version": False,
                "list_approved_version": False,
            }
        )
        oracle.pop("evidence_sha256")
        oracle["evidence_sha256"] = digest(api.canonical(oracle))
        self.api_oracle.write_text(json.dumps(oracle), encoding="utf-8")

        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if path != "/v1/tasks/1/detail":
                return value
            body = json.loads(json.dumps(value.body))
            body["asset_versions"].append(
                {
                    "id": 18,
                    "task_id": 1,
                    "asset_id": 101,
                    "asset_type": "delivery",
                    "scope_sku_code": "",
                    "retouch_requirement_id": (
                        999 if base.endswith(("8102", "8103")) else None
                    ),
                }
            )
            return result(200, body)

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.manifest_oracle_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_asset_access_hint_difference_is_not_normalized(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if path not in {"/v1/tasks/1/detail", "/v1/tasks/1/assets"}:
                return value
            body = json.loads(json.dumps(value.body))
            access_hint = (
                "prepare_required"
                if base.endswith(("8102", "8103"))
                else "legacy_direct"
            )
            if path.endswith("/detail"):
                body["asset_versions"][0]["access_hint"] = access_hint
            else:
                body["data"][0]["current_version"]["access_hint"] = access_hint
            return result(200, body)

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.semantic_body_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_asset_notes_difference_is_not_normalized(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if path not in {"/v1/tasks/1/detail", "/v1/tasks/1/assets"}:
                return value
            body = json.loads(json.dumps(value.body))
            notes = (
                "candidate-notes"
                if base.endswith(("8102", "8103"))
                else "legacy-notes"
            )
            if path.endswith("/detail"):
                body["asset_versions"][0]["notes"] = notes
            else:
                body["data"][0]["current_version"]["notes"] = notes
            return result(200, body)

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.semantic_body_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_exact_v8_asset_explanatory_copy_is_normalized(self) -> None:
        cases = (
            (
                "source",
                "Use task_no=RW-1 asset_no=AST-1 version_no=2 "
                "object_key=tasks/RW-1/source.psd to fetch the OSS-backed "
                "source file.",
                "源文件属于任务 RW-1，文件编号 AST-1，第 2 版。",
                "Source files remain OSS-backed business assets; "
                "no NAS-only path is required.",
                "当前源文件由任务资源组统一管理。",
            ),
            (
                "delivery",
                "Delivery assets are the business flow truth for audit "
                "and warehouse after approval.",
                "成品图通过审核后，作为任务当前有效成品。",
                "Warehouse and audit should consume the "
                "warehouse_ready_version or approved_version based on "
                "current task status.",
                "当前成品图由任务资源组统一管理。",
            ),
            (
                "preview",
                "Preview assets are auxiliary only and must not replace "
                "delivery assets in business flow.",
                "该文件仅用于预览，不替代正式成品图。",
                "Preview artifacts are not the formal source of truth.",
                "预览文件不是正式业务文件。",
            ),
            (
                "design_thumb",
                "Design thumb assets are lightweight preview derivatives "
                "for list/detail rendering.",
                "该缩略图仅用于页面预览。",
                "Design thumb artifacts are backend-owned derivatives "
                "for preview rendering only.",
                "缩略图只用于页面预览。",
            ),
            (
                "reference",
                "Reference assets are task-scoped files for task creation, "
                "design reference, and business understanding only.",
                "参考图用于说明任务需求。",
                "Reference assets never enter the "
                "warehouse_ready_version path.",
                "参考图用于说明任务需求。",
            ),
            (
                "erp_product_image",
                "Reference assets are task-scoped files for task creation, "
                "design reference, and business understanding only.",
                "参考图用于说明任务需求。",
                "Reference assets never enter the "
                "warehouse_ready_version path.",
                "参考图用于说明任务需求。",
            ),
        )
        for asset_type, old_hint, new_hint, old_notes, new_notes in cases:
            with self.subTest(asset_type=asset_type):
                base = {
                    "asset_type": asset_type,
                    "task_no": "RW-1",
                    "asset_no": "AST-1",
                    "version_no": 2,
                    "storage_key": "tasks/RW-1/source.psd",
                }
                legacy = dict(
                    base, access_hint=old_hint, notes=old_notes
                )
                current = dict(
                    base, access_hint=new_hint, notes=new_notes
                )
                self.assertEqual(
                    api.project_asset_version(legacy),
                    api.project_asset_version(current),
                )
                near_miss = dict(legacy, access_hint=old_hint + " changed")
                self.assertNotEqual(
                    api.project_asset_version(near_miss),
                    api.project_asset_version(current),
                )

    def test_embedded_retouch_asset_copy_uses_exact_asset_projection(self) -> None:
        legacy = {
            "retouch_requirements": [
                {
                    "source_assets": [
                        {
                            "current_version": {
                                "id": 20,
                                "task_id": 1,
                                "asset_id": 101,
                                "asset_type": "source",
                                "version_no": 2,
                                "task_no": "RW-1",
                                "asset_no": "AST-1",
                                "storage_key": "tasks/RW-1/source.psd",
                                "warehouse_ready": False,
                                "access_hint": (
                                    "Use task_no=RW-1 asset_no=AST-1 version_no=2 "
                                    "object_key=tasks/RW-1/source.psd to fetch the "
                                    "OSS-backed source file."
                                ),
                                "notes": (
                                    "Source files remain OSS-backed business assets; "
                                    "no NAS-only path is required."
                                ),
                            }
                        }
                    ]
                }
            ]
        }
        current = json.loads(json.dumps(legacy))
        version = current["retouch_requirements"][0]["source_assets"][0][
            "current_version"
        ]
        version.pop("warehouse_ready")
        version["access_hint"] = "源文件属于任务 RW-1，文件编号 AST-1，第 2 版。"
        version["notes"] = "当前源文件由任务资源组统一管理。"
        self.assertEqual(api.project_task(legacy), api.project_task(current))
        version["notes"] = "unapproved copy drift"
        self.assertNotEqual(api.project_task(legacy), api.project_task(current))

    def test_sku_item_migration_timestamp_is_narrowly_normalized(self) -> None:
        left = {
            "id": 1,
            "sku_items": [
                {
                    "id": 10,
                    "sku_code": "SKU-1",
                    "product_name": "stable",
                    "updated_at": "2026-01-01T00:00:00Z",
                }
            ],
        }
        right = json.loads(json.dumps(left))
        right["sku_items"][0]["updated_at"] = "2026-01-02T00:00:00Z"
        self.assertEqual(api.project_task(left), api.project_task(right))
        right["sku_items"][0]["product_name"] = "drift"
        self.assertNotEqual(api.project_task(left), api.project_task(right))

    def test_candidate_sku_updated_at_must_be_rfc3339(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if path not in {"/v1/tasks/1", "/v1/tasks/1/detail"}:
                return value
            body = json.loads(json.dumps(value.body))
            sku_items = [
                {
                    "id": 10,
                    "sku_code": "SKU-1",
                    "updated_at": (
                        "not-a-date"
                        if base.endswith(("8102", "8103"))
                        else "2026-01-01T00:00:00Z"
                    ),
                }
            ]
            if path.endswith("/detail"):
                body["sku_items"] = sku_items
            else:
                body["sku_items"] = sku_items
            return result(200, body)

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.manifest_oracle_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_a_only_legacy_warehouse_ready_is_compatibly_retired(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if base.endswith(("8102", "8103")):
                return value
            body = json.loads(json.dumps(value.body))
            if path == "/v1/tasks/1/detail":
                body["asset_versions"][0]["warehouse_ready"] = True
                return result(200, body)
            if path == "/v1/tasks/1/assets":
                body["data"][0]["current_version"]["warehouse_ready"] = True
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertEqual("PASS", evidence["status"])

    def test_b_cannot_resurrect_retired_warehouse_ready(self) -> None:
        for location in ("detail", "current", "approved"):
            for value_to_inject in (True, False, None):
                with self.subTest(location=location, value=value_to_inject):
                    oracle = json.loads(
                        self.api_oracle.read_text(encoding="utf-8")
                    )
                    if location == "approved":
                        approved = {
                            "task_asset_id": 20,
                            "task_id": 1,
                            "root_asset_id": 101,
                            "stable_locator": "a:20:ref-1:key:content",
                            "asset_type": "source",
                            "whole_hash": "a" * 64,
                            "binding_state": "bound",
                            "bound_role": "source",
                            "scope_sku_code": "",
                            "retouch_requirement_id": None,
                            "root_asset_type": "source",
                            "root_scope_sku_code": "",
                            "root_retouch_requirement_id": None,
                            "detail_visible": True,
                            "list_current_version": True,
                            "list_approved_version": False,
                        }
                        approved.update(
                            {
                                "task_asset_id": 19,
                                "stable_locator": "asset:101:approved-ref",
                                "list_current_version": False,
                                "list_approved_version": True,
                            }
                        )
                        self.append_legacy_oracle_asset(oracle, approved)
                        oracle.pop("evidence_sha256")
                        oracle["evidence_sha256"] = digest(
                            api.canonical(oracle)
                        )
                        self.api_oracle.write_text(
                            json.dumps(oracle), encoding="utf-8"
                        )

                    def requester(base, path, headers):
                        result_value = self.requester(base, path, headers)
                        if not base.endswith(("8102", "8103")):
                            return result_value
                        body = json.loads(json.dumps(result_value.body))
                        if location == "detail" and path == "/v1/tasks/1/detail":
                            body["asset_versions"][0][
                                "warehouse_ready"
                            ] = value_to_inject
                            return result(200, body)
                        if path == "/v1/tasks/1/assets" and location == "current":
                            body["data"][0]["current_version"][
                                "warehouse_ready"
                            ] = value_to_inject
                            return result(200, body)
                        if path == "/v1/tasks/1/assets" and location == "approved":
                            body["data"][0]["approved_version_id"] = 19
                            body["data"][0]["approved_version"] = {
                                "id": 19,
                                "task_id": 1,
                                "asset_id": 101,
                                "asset_type": "source",
                                "scope_sku_code": "",
                                "warehouse_ready": value_to_inject,
                            }
                            return result(200, body)
                        return result_value

                    evidence = self.run_compare(requester)
                    self.assertEqual("BLOCKED", evidence["status"])
                    self.assertIn(
                        "api.manifest_oracle_mismatch",
                        {
                            item["violation_code"]
                            for item in evidence["violations"]
                        },
                    )
                    if location == "approved":
                        reset = json.loads(
                            self.api_oracle.read_text(encoding="utf-8")
                        )
                        reset["versions"] = [
                            row
                            for row in reset["versions"]
                            if row["task_asset_id"] != 19
                        ]
                        reset["roots"][0]["approved_locator"] = None
                        reset["route_expectations"][
                            "approved_locators"
                        ] = []
                        reset["route_expectations"][
                            "detail_visible_locators"
                        ] = [
                            locator
                            for locator in reset["route_expectations"][
                                "detail_visible_locators"
                            ]
                            if locator != "asset:101:approved-ref"
                        ]
                        reset.pop("evidence_sha256")
                        reset["evidence_sha256"] = digest(
                            api.canonical(reset)
                        )
                        self.api_oracle.write_text(
                            json.dumps(reset), encoding="utf-8"
                        )

    def test_a_only_root_warehouse_pointer_is_compatibly_retired(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if (
                base.endswith(("8102", "8103"))
                or path != "/v1/tasks/1/assets"
            ):
                return value
            body = json.loads(json.dumps(value.body))
            body["data"][0]["warehouse_ready_version_id"] = 20
            body["data"][0]["warehouse_ready_version"] = dict(
                body["data"][0]["current_version"]
            )
            return result(200, body)

        evidence = self.run_compare(requester)
        self.assertEqual("PASS", evidence["status"])

    def test_b_cannot_resurrect_root_warehouse_pointer(self) -> None:
        for field, injected in (
            ("warehouse_ready_version_id", None),
            ("warehouse_ready_version_id", 20),
            ("warehouse_ready_version", None),
            (
                "warehouse_ready_version",
                {
                    "id": 20,
                    "task_id": 1,
                    "asset_id": 101,
                    "asset_type": "source",
                },
            ),
        ):
            with self.subTest(field=field, injected=injected):
                def requester(base, path, headers):
                    value = self.requester(base, path, headers)
                    if (
                        not base.endswith(("8102", "8103"))
                        or path != "/v1/tasks/1/assets"
                    ):
                        return value
                    body = json.loads(json.dumps(value.body))
                    body["data"][0][field] = injected
                    return result(200, body)

                evidence = self.run_compare(requester)
                self.assertEqual("BLOCKED", evidence["status"])
                self.assertIn(
                    "api.manifest_oracle_mismatch",
                    {
                        item["violation_code"]
                        for item in evidence["violations"]
                    },
                )

    def test_current_version_role_must_match_oracle_pointers(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if (
                not base.endswith(("8102", "8103"))
                or path != "/v1/tasks/1/assets"
            ):
                return value
            body = json.loads(json.dumps(value.body))
            body["data"][0]["current_version"][
                "current_version_role"
            ] = "current_approved_version"
            return result(200, body)

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.manifest_oracle_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_detail_events_are_not_omitted_from_semantic_comparison(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if path != "/v1/tasks/1/detail":
                return value
            body = json.loads(json.dumps(value.body))
            body["events"] = [
                {
                    "id": (
                        999 if base.endswith(("8102", "8103")) else 1
                    ),
                    "event_type": (
                        "wrong-b"
                        if base.endswith(("8102", "8103"))
                        else "legacy-ok"
                    ),
                }
            ]
            return result(200, body)

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.semantic_body_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_detail_top_current_handler_must_match_nested_task(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if (
                not base.endswith(("8102", "8103"))
                or path != "/v1/tasks/1/detail"
            ):
                return value
            body = json.loads(json.dumps(value.body))
            body["current_handler_id"] = 999
            body["current_handler_name"] = "wrong"
            return result(200, body)

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.manifest_oracle_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_completed_module_terminal_migration_is_narrowly_normalized(
        self,
    ) -> None:
        self.set_manifest_task_status("Completed")

        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if path not in {"/v1/tasks/1", "/v1/tasks/1/detail"}:
                return value
            body = json.loads(json.dumps(value.body))
            if path in {"/v1/tasks/1", "/v1/tasks/1/detail"}:
                task = body["task"] if path.endswith("/detail") else body
                task["task_status"] = "Completed"
            if path == "/v1/tasks/1/detail":
                body["design_sub_status"] = "not_required"
                candidate = base.endswith(("8102", "8103"))
                body["modules"] = [
                    {
                        "id": 30,
                        "module_key": "design",
                        "state": "completed" if candidate else "submitted",
                        "claimed_by": None if candidate else 7,
                        "claimed_team_code": None if candidate else "design",
                        "claimed_at": "2026-01-01T00:00:00Z",
                        "terminal_at": (
                            "2026-01-02T00:00:00Z" if candidate else None
                        ),
                        "updated_at": (
                            "2026-01-02T00:00:00Z"
                            if candidate
                            else "2026-01-01T00:00:00Z"
                        ),
                    }
                ]
            return result(200, body)

        evidence = self.run_compare(requester)
        self.assertEqual("PASS", evidence["status"], evidence["violations"])

    def test_completed_module_must_satisfy_terminal_invariant(self) -> None:
        self.set_manifest_task_status("Completed")
        for mutation in (
            "state",
            "terminal_at",
            "terminal_at_format",
            "updated_at_format",
        ):
            with self.subTest(mutation=mutation):
                def requester(base, path, headers):
                    value = self.requester(base, path, headers)
                    if path not in {
                        "/v1/tasks/1",
                        "/v1/tasks/1/detail",
                    }:
                        return value
                    body = json.loads(json.dumps(value.body))
                    if path in {"/v1/tasks/1", "/v1/tasks/1/detail"}:
                        task = (
                            body["task"] if path.endswith("/detail") else body
                        )
                        task["task_status"] = "Completed"
                    if path == "/v1/tasks/1/detail":
                        body["design_sub_status"] = "not_required"
                        module = {
                            "id": 30,
                            "module_key": "design",
                            "state": "completed",
                            "claimed_by": None,
                            "claimed_team_code": None,
                            "claimed_at": "2026-01-01T00:00:00Z",
                            "terminal_at": "2026-01-02T00:00:00Z",
                            "updated_at": "2026-01-02T00:00:00Z",
                        }
                        if base.endswith(("8102", "8103")):
                            if mutation == "state":
                                module["state"] = "active"
                            elif mutation == "terminal_at":
                                module["terminal_at"] = None
                            elif mutation == "terminal_at_format":
                                module["terminal_at"] = "not-a-date"
                            else:
                                module["updated_at"] = "not-a-date"
                        body["modules"] = [module]
                    return result(200, body)

                evidence = self.run_compare(requester)
                self.assertEqual("BLOCKED", evidence["status"])
                self.assertIn(
                    "api.manifest_oracle_mismatch",
                    {
                        item["violation_code"]
                        for item in evidence["violations"]
                    },
                )

    def test_completed_legacy_terminal_module_may_retain_claim_history(
        self,
    ) -> None:
        self.set_manifest_task_status("Completed")

        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if path not in {"/v1/tasks/1", "/v1/tasks/1/detail"}:
                return value
            body = json.loads(json.dumps(value.body))
            task = body["task"] if path.endswith("/detail") else body
            task["task_status"] = "Completed"
            if path.endswith("/detail"):
                body["design_sub_status"] = "not_required"
                body["modules"] = [{
                    "id": 30,
                    "module_key": "design",
                    "state": "closed",
                    "claimed_by": 7,
                    "claimed_team_code": "design",
                    "claimed_at": "2026-01-01T00:00:00Z",
                    "terminal_at": "2026-01-02T00:00:00Z",
                    "updated_at": "2026-01-02T00:00:00Z",
                }]
            return result(200, body)

        evidence = self.run_compare(requester)
        self.assertEqual("PASS", evidence["status"], evidence["violations"])

    def test_completed_module_claimed_at_is_not_normalized(self) -> None:
        self.set_manifest_task_status("Completed")

        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if path not in {"/v1/tasks/1", "/v1/tasks/1/detail"}:
                return value
            body = json.loads(json.dumps(value.body))
            if path in {"/v1/tasks/1", "/v1/tasks/1/detail"}:
                task = body["task"] if path.endswith("/detail") else body
                task["task_status"] = "Completed"
            if path == "/v1/tasks/1/detail":
                body["design_sub_status"] = "not_required"
                body["modules"] = [
                    {
                        "id": 30,
                        "module_key": "design",
                        "state": "completed",
                        "claimed_by": None,
                        "claimed_team_code": None,
                        "claimed_at": (
                            "2026-01-03T00:00:00Z"
                            if base.endswith(("8102", "8103"))
                            else "2026-01-01T00:00:00Z"
                        ),
                        "terminal_at": "2026-01-02T00:00:00Z",
                        "updated_at": "2026-01-02T00:00:00Z",
                    }
                ]
            return result(200, body)

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.semantic_body_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_candidate_detail_design_sub_status_must_match_derivation(
        self,
    ) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if (
                base.endswith(("8102", "8103"))
                and path == "/v1/tasks/1/detail"
            ):
                body = json.loads(json.dumps(value.body))
                body["design_sub_status"] = "pending_audit"
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertTrue(
            any(
                item["violation_code"] == "api.manifest_oracle_mismatch"
                and "design_sub_status" in item["detail"]
                for item in evidence["violations"]
            )
        )

    def test_inprogress_retouch_requires_one_active_nonterminal_module(
        self,
    ) -> None:
        self.set_manifest_task_contract(
            task_type="retouch_task", status="InProgress"
        )

        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if path not in {"/v1/tasks/1", "/v1/tasks/1/detail"}:
                return value
            body = json.loads(json.dumps(value.body))
            task = body["task"] if path.endswith("/detail") else body
            task["task_type"] = "retouch_task"
            task["task_status"] = "InProgress"
            if path.endswith("/detail"):
                body["design_sub_status"] = "in_progress"
                body["modules"] = [
                    {
                        "id": 30,
                        "module_key": "retouch",
                        "state": (
                            "completed"
                            if base.endswith(("8102", "8103"))
                            else "in_progress"
                        ),
                        "claimed_by": 7,
                        "claimed_team_code": "retouch",
                        "claimed_at": "2026-01-01T00:00:00Z",
                        "terminal_at": (
                            "2026-01-02T00:00:00Z"
                            if base.endswith(("8102", "8103"))
                            else None
                        ),
                        "updated_at": "2026-01-02T00:00:00Z",
                    }
                ]
            return result(200, body)

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertTrue(
            any(
                item["violation_code"] == "api.manifest_oracle_mismatch"
                and "reopened retouch task" in item["detail"]
                for item in evidence["violations"]
            )
        )

    def test_non_retouch_module_must_satisfy_lifecycle_invariant(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if (
                base.endswith(("8102", "8103"))
                and path == "/v1/tasks/1/detail"
            ):
                body = json.loads(json.dumps(value.body))
                body["modules"] = [
                    {
                        "id": 30,
                        "module_key": "design",
                        "state": "BROKEN_TERMINAL_STATE",
                        "claimed_by": 999,
                        "claimed_team_code": "wrong-team",
                        "claimed_at": "2026-01-01T00:00:00Z",
                        "terminal_at": "2099-01-01T00:00:00Z",
                        "updated_at": "2026-01-02T00:00:00Z",
                    }
                ]
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertTrue(
            any(
                item["violation_code"] == "api.manifest_oracle_mismatch"
                and "lifecycle invariant" in item["detail"]
                for item in evidence["violations"]
            )
        )

    def test_reviewed_deleted_asset_recovery_projection_is_hash_bound(
        self,
    ) -> None:
        whole_hash = "d" * 64
        baseline = [
            {
                "id": 101,
                "current_version": {
                    "id": 20,
                    "file_hash": None,
                    "usable_state": "cleaned",
                    "storage_key": "",
                    "download_url": "",
                },
            }
        ]
        candidate = [
            {
                "id": 101,
                "current_version": {
                    "id": 20,
                    "file_hash": whole_hash,
                    "usable_state": "pending_review",
                    "storage_key": "controlled/object",
                    "download_url": "/v1/assets/20/download",
                },
            }
        ]
        left, right = api.project_asset_pair(
            baseline,
            candidate,
            frozenset(),
            {
                "20": {
                    "whole_hash": whole_hash,
                    "provenance": {"kind": "recovery_receipt"},
                }
            },
        )
        self.assertEqual(left, right)

        ordinary = json.loads(json.dumps(candidate))
        left, right = api.project_asset_pair(
            baseline,
            ordinary,
            frozenset(),
            {"20": {"whole_hash": whole_hash, "provenance": {"kind": "a_preserved"}}},
        )
        self.assertNotEqual(left, right)

    def test_reviewed_bundle_member_hash_projection_is_receipt_bound(
        self,
    ) -> None:
        baseline = [{"id": 101, "current_version": {"id": 20}}]
        candidate = [
            {
                "id": 101,
                "current_version": {
                    "id": 20,
                    "file_hash": "d" * 64,
                },
            }
        ]
        left, right = api.project_asset_pair(
            baseline,
            candidate,
            frozenset(),
            {
                "20": {
                    "provenance": {
                        "kind": "a_preserved",
                        "bundle_member_receipt": "task:sku:revision",
                    }
                }
            },
        )
        self.assertEqual(left, right)

        left, right = api.project_asset_pair(
            baseline,
            candidate,
            frozenset(),
            {"20": {"provenance": {"kind": "a_preserved"}}},
        )
        self.assertNotEqual(left, right)

    def test_reviewed_approval_pointer_is_normalized_after_oracle_check(
        self,
    ) -> None:
        baseline = [
            {
                "id": 101,
                "asset_type": "delivery",
                "current_version_id": 20,
                "current_version": {
                    "id": 20,
                    "asset_type": "delivery",
                    "approved_for_flow": False,
                },
            }
        ]
        candidate = json.loads(json.dumps(baseline))
        candidate[0]["current_version"]["approved_for_flow"] = True
        candidate[0]["approved_version_id"] = 20
        candidate[0]["approved_version"] = {
            "id": 20,
            "asset_type": "delivery",
            "approved_for_flow": True,
        }
        left, right = api.project_asset_pair(
            baseline,
            candidate,
            frozenset({"101"}),
            {},
        )
        self.assertEqual(left, right)

        left, right = api.project_asset_pair(
            baseline,
            candidate,
            frozenset(),
            {},
        )
        self.assertNotEqual(left, right)

    def test_receipt_governed_candidate_root_is_removed_only_when_allowlisted(
        self,
    ) -> None:
        baseline = [
            {
                "id": 101,
                "asset_type": "source",
                "current_version_id": 20,
                "current_version": {
                    "id": 20,
                    "asset_type": "source",
                },
            }
        ]
        candidate = [
            *json.loads(json.dumps(baseline)),
            {
                "id": 202,
                "asset_type": "source",
                "current_version_id": 30,
                "current_version": {
                    "id": 30,
                    "asset_type": "source",
                },
            },
        ]

        left, right = api.project_asset_pair(
            baseline,
            candidate,
            frozenset({"202"}),
            {"30": {"provenance": {"kind": "bundle_receipt"}}},
        )
        self.assertEqual(left, right)

        left, right = api.project_asset_pair(
            baseline,
            candidate,
            frozenset(),
            {"30": {"provenance": {"kind": "bundle_receipt"}}},
        )
        self.assertNotEqual(left, right)

    def test_retired_terminal_module_fields_are_normalized_after_oracle_check(
        self,
    ) -> None:
        baseline = {
            "task": {"id": 1, "task_status": "PendingClose"},
            "modules": [
                {
                    "id": 30,
                    "module_key": "warehouse",
                    "state": "submitted",
                    "claimed_by": 7,
                    "claimed_team_code": "warehouse",
                    "claimed_at": "2026-01-01T00:00:00Z",
                    "terminal_at": None,
                    "updated_at": "2026-01-01T00:00:00Z",
                }
            ],
        }
        candidate = {
            "task": {"id": 1, "task_status": "Completed"},
            "modules": [
                {
                    "id": 30,
                    "module_key": "warehouse",
                    "state": "completed",
                    "claimed_by": None,
                    "claimed_team_code": None,
                    "claimed_at": "2026-01-01T00:00:00Z",
                    "terminal_at": "2026-01-02T00:00:00Z",
                    "updated_at": "2026-01-02T00:00:00Z",
                }
            ],
        }
        left, right = api.project_detail_pair(baseline, candidate)
        self.assertEqual(left, right)

    def test_domain_terminal_module_states_are_not_treated_as_open(self) -> None:
        self.assertEqual(
            frozenset(
                {"completed", "closed", "forcibly_closed", "closed_by_admin"}
            ),
            api.TERMINAL_MODULE_STATES,
        )
        self.assertTrue(
            {
                "active",
                "pending",
                "pending_claim",
                "in_progress",
                "submitted",
                "approved",
                "preparing",
                "received",
                "review",
            }.issubset(api.ACTIVE_MODULE_STATES)
        )

    def test_effective_revision_scope_inherits_root_identity(self) -> None:
        row = {
            "scope_sku_code": "",
            "root_scope_sku_code": "SKU-ROOT",
            "retouch_requirement_id": None,
            "root_retouch_requirement_id": 9,
        }
        self.assertEqual(
            "SKU-ROOT", api.effective_oracle_scope_sku_code(row)
        )
        self.assertEqual(
            9, api.effective_oracle_retouch_requirement_id(row)
        )

    def test_approved_source_alias_uses_design_root_identity(self) -> None:
        oracle = json.loads(self.api_oracle.read_text(encoding="utf-8"))
        oracle["roots"][0]["intrinsic_asset_type"] = "delivery"
        oracle["versions"][0]["scope_sku_code"] = "SKU-REVIEWED"
        self.append_legacy_oracle_asset(
            oracle,
            {
                "task_asset_id": 19,
                "task_id": 1,
                "root_asset_id": 101,
                "stable_locator": "asset:101:approved-source-alias",
                "asset_type": "source",
                "whole_hash": "",
                "binding_state": "bound",
                "bound_role": "source",
                "scope_sku_code": "SKU-REVIEWED",
                "retouch_requirement_id": None,
                "root_asset_type": "delivery",
                "root_scope_sku_code": "",
                "root_retouch_requirement_id": None,
                "detail_visible": True,
                "list_current_version": False,
                "list_approved_version": True,
            }
        )
        oracle.pop("evidence_sha256")
        oracle["evidence_sha256"] = digest(api.canonical(oracle))
        self.api_oracle.write_text(json.dumps(oracle), encoding="utf-8")

        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            body = json.loads(json.dumps(value.body))
            if path == "/v1/tasks/1/detail":
                body["asset_versions"][0]["scope_sku_code"] = "SKU-REVIEWED"
                body["asset_versions"].append(
                    {
                        "id": 19,
                        "task_id": 1,
                        "asset_id": 101,
                        "asset_type": "source",
                        "scope_sku_code": "SKU-REVIEWED",
                    }
                )
                return result(200, body)
            if path == "/v1/tasks/1/assets":
                body["data"][0]["asset_type"] = "delivery"
                body["data"][0]["current_version"]["asset_type"] = "delivery"
                body["data"][0]["current_version"]["scope_sku_code"] = (
                    "SKU-REVIEWED"
                )
                body["data"][0]["approved_version_id"] = 19
                body["data"][0]["approved_version"] = {
                    "id": 19,
                    "task_id": 1,
                    "asset_id": 101,
                    "asset_type": "delivery",
                    "scope_sku_code": "SKU-REVIEWED",
                    "current_version_role": "approved_version",
                }
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])

    def test_detail_current_version_role_must_match_openapi_enum(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if (
                not base.endswith(("8102", "8103"))
                or path != "/v1/tasks/1/detail"
            ):
                return value
            body = json.loads(json.dumps(value.body))
            body["asset_versions"][0]["current_version_role"] = (
                "not-an-openapi-role"
            )
            return result(200, body)

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.manifest_oracle_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_oracle_bound_candidate_asset_root_addition_is_narrowly_projected(
        self,
    ) -> None:
        oracle = json.loads(self.api_oracle.read_text(encoding="utf-8"))
        self.append_legacy_oracle_asset(
            oracle,
            {
                "task_asset_id": 21,
                "task_id": 1,
                "root_asset_id": 102,
                "stable_locator": "asset:102:reviewed-bundle",
                "asset_type": "source",
                "whole_hash": "b" * 64,
                "binding_state": "bound",
                "bound_role": "source",
                "scope_sku_code": "",
                "retouch_requirement_id": None,
                "root_asset_type": "source",
                "root_scope_sku_code": "",
                "root_retouch_requirement_id": None,
                "detail_visible": False,
                "list_current_version": True,
                "list_approved_version": False,
            }
        )
        oracle.pop("evidence_sha256")
        oracle["evidence_sha256"] = digest(api.canonical(oracle))
        self.api_oracle.write_text(json.dumps(oracle), encoding="utf-8")

        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if not base.endswith(("8102", "8103")) or path != "/v1/tasks/1/assets":
                return value
            body = json.loads(json.dumps(value.body))
            body["data"].append(
                {
                    "id": 102,
                    "task_id": 1,
                    "asset_type": "source",
                    "current_version_id": 21,
                    "current_version": {
                        "id": 21,
                        "task_id": 1,
                        "asset_id": 102,
                        "asset_type": "source",
                        "scope_sku_code": "",
                        "current_version_role": "current_version",
                    },
                }
            )
            return result(200, body)

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])

    def test_unoracled_candidate_asset_root_addition_is_not_projected(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if not base.endswith(("8102", "8103")) or path != "/v1/tasks/1/assets":
                return value
            body = json.loads(json.dumps(value.body))
            body["data"].append(
                {
                    "id": 999,
                    "task_id": 1,
                    "asset_type": "source",
                    "current_version_id": 999,
                    "current_version": {
                        "id": 999,
                        "task_id": 1,
                        "asset_id": 999,
                        "asset_type": "source",
                        "scope_sku_code": "",
                        "current_version_role": "current_version",
                    },
                }
            )
            return result(200, body)

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertTrue(
            {
                "api.manifest_oracle_mismatch",
                "api.semantic_body_mismatch",
            }
            <= {
                item["violation_code"]
                for item in evidence["violations"]
            }
        )

    def test_b_asset_root_scope_is_not_hidden_by_version_projection(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if base.endswith(("8102", "8103")) and path == "/v1/tasks/1/assets":
                body = json.loads(json.dumps(value.body))
                body["data"][0]["scope_sku_code"] = "WRONG-ROOT-SCOPE"
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.manifest_oracle_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_empty_scope_rejects_non_string_falsy_value(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if not base.endswith(("8102", "8103")):
                return value
            body = json.loads(json.dumps(value.body))
            if path == "/v1/tasks/1/detail":
                body["asset_versions"][0]["scope_sku_code"] = 0
                return result(200, body)
            if path == "/v1/tasks/1/assets":
                body["data"][0]["current_version"]["scope_sku_code"] = False
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.manifest_oracle_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_asset_identity_rejects_boolean_and_invalid_contract_values(
        self,
    ) -> None:
        cases = (
            ("file_hash", False),
            ("retouch_requirement_id", True),
            ("usable_state", "BROKEN_NOT_ENUM"),
            ("approved_at", "2026-01-01T00:00:00"),
        )
        for field, invalid in cases:
            with self.subTest(field=field):
                def requester(base, path, headers):
                    value = self.requester(base, path, headers)
                    if (
                        base.endswith(("8102", "8103"))
                        and path == "/v1/tasks/1/detail"
                    ):
                        body = json.loads(json.dumps(value.body))
                        body["asset_versions"][0][field] = invalid
                        return result(200, body)
                    return value

                evidence = self.run_compare(requester)
                self.assertEqual("BLOCKED", evidence["status"])
                self.assertIn(
                    "api.manifest_oracle_mismatch",
                    {
                        item["violation_code"]
                        for item in evidence["violations"]
                    },
                )

    def test_duplicate_detail_asset_version_is_hard_failure(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if (
                base.endswith(("8102", "8103"))
                and path == "/v1/tasks/1/detail"
            ):
                body = json.loads(json.dumps(value.body))
                body["asset_versions"].append(
                    dict(body["asset_versions"][0])
                )
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertTrue(
            any(
                item["violation_code"] == "api.manifest_oracle_mismatch"
                and "duplicated" in item["detail"]
                for item in evidence["violations"]
            )
        )

    def test_nullable_identity_fields_and_root_scope_are_normalized(self) -> None:
        oracle = json.loads(self.api_oracle.read_text(encoding="utf-8"))
        oracle["roots"][0]["retouch_requirement_id"] = 9
        oracle["versions"][0]["content_sha256"] = ""
        oracle["versions"][0]["approved_at"] = "2026-01-01T00:00:00"
        oracle.pop("evidence_sha256")
        oracle["evidence_sha256"] = digest(api.canonical(oracle))
        self.api_oracle.write_text(json.dumps(oracle), encoding="utf-8")

        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            body = json.loads(json.dumps(value.body))
            if path == "/v1/tasks/1/detail":
                version = body["asset_versions"][0]
                version["scope_sku_code"] = None
                version["retouch_requirement_id"] = 9
                version["file_hash"] = None
                version["approved_at"] = "2026-01-01T00:00:00Z"
                return result(200, body)
            if path == "/v1/tasks/1/assets":
                body["data"][0]["retouch_requirement_id"] = 9
                version = body["data"][0]["current_version"]
                version["scope_sku_code"] = None
                version["retouch_requirement_id"] = 9
                version["file_hash"] = None
                version["approved_at"] = "2026-01-01T00:00:00Z"
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertEqual("PASS", evidence["status"], evidence["violations"])

    def test_approved_version_scope_is_oracle_validated(self) -> None:
        oracle = json.loads(self.api_oracle.read_text(encoding="utf-8"))
        self.append_legacy_oracle_asset(
            oracle,
            {
                "task_asset_id": 19,
                "task_id": 1,
                "root_asset_id": 101,
                "stable_locator": "asset:101:approved-ref",
                "asset_type": "source",
                "whole_hash": "",
                "binding_state": "legacy",
                "bound_role": "NULL",
                "scope_sku_code": "APPROVED-SCOPE",
                "retouch_requirement_id": None,
                "root_asset_type": "source",
                "root_scope_sku_code": "",
                "root_retouch_requirement_id": None,
                "detail_visible": True,
                "list_current_version": False,
                "list_approved_version": True,
            }
        )
        oracle.pop("evidence_sha256")
        oracle["evidence_sha256"] = digest(api.canonical(oracle))
        self.api_oracle.write_text(json.dumps(oracle), encoding="utf-8")

        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            body = json.loads(json.dumps(value.body))
            if path == "/v1/tasks/1/detail":
                body["asset_versions"].append(
                    {
                        "id": 19,
                        "task_id": 1,
                        "asset_id": 101,
                        "asset_type": "source",
                        "scope_sku_code": "APPROVED-SCOPE",
                    }
                )
                return result(200, body)
            if path == "/v1/tasks/1/assets":
                body["data"][0]["approved_version_id"] = 19
                body["data"][0]["approved_version"] = {
                    "id": 19,
                    "task_id": 1,
                    "asset_id": 101,
                    "asset_type": "source",
                    "scope_sku_code": (
                        "WRONG-APPROVED-SCOPE"
                        if base.endswith(("8102", "8103"))
                        else "APPROVED-SCOPE"
                    ),
                }
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.manifest_oracle_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_error_trace_id_is_only_normalized_at_exact_envelope_path(self) -> None:
        left = {
            "error": {"code": "gone", "trace_id": "one"},
            "data": {"trace_id": "business-one"},
        }
        right = {
            "error": {"code": "gone", "trace_id": "two"},
            "data": {"trace_id": "business-two"},
        }
        normalized_left = api.normalize_transport_noise(left)
        normalized_right = api.normalize_transport_noise(right)
        self.assertEqual({"code": "gone"}, normalized_left["error"])
        self.assertEqual({"code": "gone"}, normalized_right["error"])
        self.assertNotEqual(normalized_left, normalized_right)

    def test_manifest_time_normalizes_frozen_utc_datetime(self) -> None:
        self.assertEqual(
            "2026-07-04T05:50:38Z",
            api.normalize_manifest_time("2026-07-04T05:50:38"),
        )
        self.assertEqual(
            "2026-07-04T05:50:38Z",
            api.normalize_manifest_time("2026-07-04T05:50:38.000000+00:00"),
        )

    def test_compact_evidence_reason_is_bound_to_reviewed_mapping(self) -> None:
        reviewed_hash = digest("reviewed long reason".encode())
        marker = (
            "[migration_v2 manifest=" + "a" * 64
            + f" reason_sha256={reviewed_hash}"
            + " confidence=confirmed_auto confirmed_by=1"
            + " confirmed_at=2026-07-23T01:02:03Z"
            + " evidence_count=1 first_evidence=event-1]"
        )
        summary = api.expected_evidence_summary(marker, reviewed_hash)
        self.assertEqual(reviewed_hash, summary["business_reason_sha256"])
        with self.assertRaisesRegex(ValueError, "reviewed reason hash differs"):
            api.expected_evidence_summary(marker, "b" * 64)

    def test_manifest_duplicate_entity_or_component_hash_tamper_is_rejected(
        self,
    ) -> None:
        original = self.manifest.read_text(encoding="utf-8")
        first = original.splitlines()[0]
        self.manifest.write_text(original + first + "\n", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "duplicates"):
            api.load_manifest_expectations(
                self.manifest,
                self.run_id,
                expected_mapping_sha256=self.mapping_sha256,
                expected_baseline_attestation_sha256=(
                    self.baseline_attestation_sha256
                ),
            )
        rows = [json.loads(line) for line in original.splitlines()]
        rows[0]["expected_hash"] = "0" * 64
        self.manifest.write_text(
            "".join(json.dumps(row) + "\n" for row in rows),
            encoding="utf-8",
        )
        with self.assertRaisesRegex(ValueError, "component hash mismatch"):
            api.load_manifest_expectations(
                self.manifest,
                self.run_id,
                expected_mapping_sha256=self.mapping_sha256,
                expected_baseline_attestation_sha256=(
                    self.baseline_attestation_sha256
                ),
            )

    def test_manifest_requires_exact_mapping_and_baseline_provenance(self) -> None:
        original = self.manifest.read_text(encoding="utf-8")
        cases = (
            ("mapping_sha256", "mapping hash differs"),
            (
                "baseline_attestation_sha256",
                "baseline attestation hash differs",
            ),
        )
        for field, error in cases:
            with self.subTest(field=field):
                rows = [
                    json.loads(line)
                    for line in original.splitlines()
                    if line.strip()
                ]
                rows[0]["detail_json"]["input_sha256"].pop(field)
                self.manifest.write_text(
                    "".join(json.dumps(row) + "\n" for row in rows),
                    encoding="utf-8",
                )
                with self.assertRaisesRegex(ValueError, error):
                    api.load_manifest_expectations(
                        self.manifest,
                        self.run_id,
                        expected_mapping_sha256=self.mapping_sha256,
                        expected_baseline_attestation_sha256=(
                            self.baseline_attestation_sha256
                        ),
                    )
        self.manifest.write_text(original, encoding="utf-8")

    def test_all_b_missing_expected_group_fails_manifest_oracle(self) -> None:
        def requester(base, path, headers):
            if base.endswith(("8102", "8103")) and path.endswith(
                "/resource-bundle"
            ):
                return result(
                    200,
                    {"task_id": 1, "workflow_revision": 1, "groups": []},
                )
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        self.assertIn(
            "api.manifest_oracle_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_all_b_403_resource_bundle_cannot_skip_manifest_groups(self) -> None:
        def requester(base, path, headers):
            if base.endswith(("8102", "8103")) and path.endswith(
                "/resource-bundle"
            ):
                return result(403, {"error": {"code": "forbidden"}})
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.manifest_oracle_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_all_b_403_history_cannot_skip_manifest_revisions(self) -> None:
        def requester(base, path, headers):
            if base.endswith(("8102", "8103")) and "/revisions" in path:
                return result(403, {"error": {"code": "forbidden"}})
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertIn(
            "api.manifest_oracle_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_all_b_wrong_current_handler_fails_manifest_oracle(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if base.endswith(("8102", "8103")) and path in {
                "/v1/tasks/1",
                "/v1/tasks/1/detail",
            }:
                body = json.loads(json.dumps(value.body))
                task = body["task"] if path.endswith("/detail") else body
                task["current_handler_id"] = 99
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertIn(
            "api.manifest_oracle_mismatch",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_all_b_wrong_owner_org_fails_api_oracle(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if base.endswith(("8102", "8103")) and path in {
                "/v1/tasks/1",
                "/v1/tasks/1/detail",
            }:
                body = json.loads(json.dumps(value.body))
                task = body["task"] if path.endswith("/detail") else body
                task["owner_department_id"] = 99
                task["owner_team_id"] = 100
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertTrue(
            any(
                item["violation_code"] == "api.manifest_oracle_mismatch"
                and "organization projection" in item["detail"]
                for item in evidence["violations"]
            )
        )

    def test_allowed_actions_widening_inside_200_body_is_hard_failure(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if path == "/v1/tasks/1":
                body = dict(value.body)
                body["allowed_actions"] = (
                    ["view", "task.edit"]
                    if base.endswith(("8102", "8103"))
                    else ["view"]
                )
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertIn(
            "api.allowed_actions_widened",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_revision_pagination_total_mismatch_is_hard_failure(self) -> None:
        def requester(base, path, headers):
            if "/revisions" in path:
                value = self.requester(base, path, headers)
                body = json.loads(json.dumps(value.body))
                body["data"]["total"] = 3
                return result(200, body)
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        self.assertIn(
            "api.invalid_pagination",
            {item["violation_code"] for item in evidence["violations"]},
        )

    def test_revision_missing_openapi_required_field_is_hard_failure(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if base.endswith(("8102", "8103")) and "/revisions" in path:
                body = json.loads(json.dumps(value.body))
                body["data"]["items"][0].pop("created_at")
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertTrue(
            any(
                item["violation_code"] == "api.manifest_oracle_mismatch"
                and "created_at" in item["detail"]
                for item in evidence["violations"]
            )
        )

    def test_revision_file_access_contract_is_role_and_availability_bound(
        self,
    ) -> None:
        def available_file(task_asset_id: int) -> dict:
            return {
                "task_asset_id": task_asset_id,
                "file_name": f"{task_asset_id}.png",
                "availability": "available",
                "preview_url": f"/v1/task-assets/{task_asset_id}/preview",
                "download_url": f"/v1/task-assets/{task_asset_id}/download",
            }

        revision = {
            "id": 200,
            "group_id": 10,
            "revision_no": 2,
            "status": "submitted",
            "mode": "single",
            "source_task_asset_id": 20,
            "source_file": available_file(20),
            "source_stage": "design",
            "created_by": 1,
            "legacy_migration": False,
            "items": [
                {
                    "id": 300,
                    "revision_id": 200,
                    "task_asset_id": 21,
                    "sort_order": 0,
                    "file": {
                        **available_file(21),
                        "revision_item_id": 300,
                    },
                }
            ],
            "references": [
                {
                    "id": 400,
                    "revision_id": 200,
                    "reference_file_ref_id": 500,
                    "sort_order": 0,
                    "ref_id": "reference-500",
                    "formal_task_asset_id": 22,
                    "created_at": "2026-01-01T00:00:00Z",
                    **available_file(22),
                }
            ],
            "created_at": "2026-01-01T00:00:00Z",
        }

        admin_runner = self.make_runner()
        admin_runner.validate_revision_contract_shape(
            "dev_dev_b", "admin", "revision:test", revision
        )
        self.assertEqual([], admin_runner.violations)

        legacy_reference = copy.deepcopy(revision)
        legacy_reference["references"][0].pop("formal_task_asset_id")
        legacy_reference["references"][0].pop("preview_url")
        legacy_reference["references"][0].pop("download_url")
        legacy_reference_runner = self.make_runner()
        legacy_reference_runner.validate_revision_contract_shape(
            "dev_dev_b",
            "admin",
            "revision:test",
            legacy_reference,
            historical=True,
        )
        self.assertEqual([], legacy_reference_runner.violations)

        legacy_reference["references"][0]["preview_url"] = (
            "/v1/task-assets/22/preview"
        )
        exposed_reference_runner = self.make_runner()
        exposed_reference_runner.validate_revision_contract_shape(
            "dev_dev_b",
            "admin",
            "revision:test",
            legacy_reference,
            historical=True,
        )
        self.assertTrue(
            any(
                "without formal task asset exposes controlled URL"
                in item["detail"]
                for item in exposed_reference_runner.violations
            )
        )

        missing_admin_url = copy.deepcopy(revision)
        missing_admin_url["source_file"].pop("preview_url")
        missing_url_runner = self.make_runner()
        missing_url_runner.validate_revision_contract_shape(
            "dev_dev_b", "admin", "revision:test", missing_admin_url
        )
        self.assertTrue(
            any(
                "lacks preview or download URL" in item["detail"]
                for item in missing_url_runner.violations
            )
        )

        view_runner = self.make_runner()
        view_runner.validate_revision_contract_shape(
            "dev_dev_b", "view-inside", "revision:test", revision
        )
        self.assertEqual(
            3,
            sum(
                "view_only file exposes download URL" in item["detail"]
                for item in view_runner.violations
            ),
        )

        unavailable = copy.deepcopy(revision)
        unavailable["source_file"] = {
            "task_asset_id": 20,
            "file_name": "20.png",
            "availability": "historical_unavailable",
            "unavailable_reason": "legacy_object_missing",
        }
        unavailable_runner = self.make_runner()
        unavailable_runner.validate_revision_contract_shape(
            "dev_dev_b", "admin", "revision:test", unavailable
        )
        self.assertEqual([], unavailable_runner.violations)

        unavailable["source_file"]["preview_url"] = (
            "/v1/task-assets/20/preview"
        )
        bad_runner = self.make_runner()
        bad_runner.validate_revision_contract_shape(
            "dev_dev_b", "admin", "revision:test", unavailable
        )
        self.assertTrue(
            any(
                "unavailable file exposes URL" in item["detail"]
                for item in bad_runner.violations
            )
        )

    def test_null_revision_items_is_reported_without_comparator_crash(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if base.endswith(("8102", "8103")) and "/revisions" in path:
                body = json.loads(json.dumps(value.body))
                body["data"]["items"][0]["items"] = None
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertEqual("BLOCKED", evidence["status"])
        self.assertTrue(
            any(
                item["violation_code"] == "api.manifest_oracle_mismatch"
                and "revision items is invalid" in item["detail"]
                for item in evidence["violations"]
            )
        )

    def test_embedded_current_revision_must_equal_pointer_history_row(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if base.endswith(("8102", "8103")) and (
                path.endswith("/resource-bundle")
                or path == "/v1/resource-groups/10"
            ):
                body = json.loads(json.dumps(value.body))
                data = body.get("data", body)
                group = (
                    data["groups"][0]
                    if path.endswith("/resource-bundle")
                    else data
                )
                group["working_revision"]["reason"] = "wrong-current-snapshot"
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertTrue(
            any(
                item["violation_code"] == "api.manifest_oracle_mismatch"
                and "not identical to its history row" in item["detail"]
                for item in evidence["violations"]
            )
        )

    def test_nested_task_resource_file_identity_is_hard_failure(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if base.endswith(("8102", "8103")) and "/revisions" in path:
                body = json.loads(json.dumps(value.body))
                row = body["data"]["items"][0]
                row["source_task_asset_id"] = 20
                row["source_file"] = {
                    "task_asset_id": 999,
                    "file_name": "wrong.psd",
                }
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertTrue(
            any(
                item["violation_code"] == "api.manifest_oracle_mismatch"
                and "source_file task_asset_id differs" in item["detail"]
                for item in evidence["violations"]
            )
        )

    def test_reference_without_frozen_formal_asset_rejects_bundle_alias(self) -> None:
        rows = [
            json.loads(line)
            for line in self.manifest.read_text(encoding="utf-8").splitlines()
        ]
        reference_row = {
            "run_id": self.run_id,
            "gate_name": "G05",
            "entity_key": "revision-reference:1:task:0:2:0",
            "review_state": "pass",
            "expected_state": "matched",
            "detail_json": {
                "components": ["1", "", "0", "ref-1", "reference.jpg", "task"]
            },
        }
        reference_row["expected_hash"] = api.component_sha256(
            reference_row["detail_json"]["components"]
        )
        rows.append(reference_row)
        self.manifest.write_text(
            "".join(json.dumps(row) + "\n" for row in rows),
            encoding="utf-8",
        )
        oracle = json.loads(self.api_oracle.read_text(encoding="utf-8"))
        self.append_legacy_oracle_asset(
            oracle,
            {
                "task_asset_id": 99,
                "task_id": 1,
                "root_asset_id": None,
                "stable_locator": "bundle:" + "b" * 64,
                "asset_type": "source",
                "whole_hash": "b" * 64,
                "binding_state": "bound",
                "bound_role": "source",
                "scope_sku_code": "",
                "retouch_requirement_id": None,
                "root_asset_type": "",
                "root_scope_sku_code": "",
                "root_retouch_requirement_id": None,
                "detail_visible": False,
                "list_current_version": False,
                "list_approved_version": False,
            }
        )
        oracle.pop("evidence_sha256")
        self.api_oracle.write_text(json.dumps(oracle), encoding="utf-8")
        self.rebind_api_oracle()

        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if base.endswith(("8102", "8103")) and "/revisions" in path:
                body = json.loads(json.dumps(value.body))
                body["data"]["items"][0]["references"] = [
                    {
                        "id": 300,
                        "revision_id": 200,
                        "reference_file_ref_id": 1,
                        "formal_task_asset_id": 99,
                        "sort_order": 0,
                        "ref_id": "ref-1",
                        "file_name": "reference.jpg",
                        "scope": "task",
                        "created_at": "2026-01-01T00:00:00Z",
                    }
                ]
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertTrue(
            any(
                item["violation_code"] == "api.manifest_oracle_mismatch"
                and "unexpectedly has a formal asset" in item["detail"]
                for item in evidence["violations"]
            )
        )

    def test_formal_reference_rejects_source_role_alias(self) -> None:
        rows = [
            json.loads(line)
            for line in self.manifest.read_text(encoding="utf-8").splitlines()
        ]
        reference_row = {
            "run_id": self.run_id,
            "gate_name": "G05",
            "entity_key": "revision-reference:1:task:0:2:0",
            "review_state": "pass",
            "expected_state": "matched",
            "detail_json": {
                "components": [
                    "1",
                    "formal-ref",
                    "0",
                    "ref-1",
                    "reference.jpg",
                    "task",
                ]
            },
        }
        reference_row["expected_hash"] = api.component_sha256(
            reference_row["detail_json"]["components"]
        )
        rows.append(reference_row)
        self.manifest.write_text(
            "".join(json.dumps(row) + "\n" for row in rows),
            encoding="utf-8",
        )
        oracle = json.loads(self.api_oracle.read_text(encoding="utf-8"))
        self.append_legacy_oracle_asset(
            oracle,
            {
                "task_asset_id": 99,
                "task_id": 1,
                "root_asset_id": 999,
                "stable_locator": "asset:999:formal-ref",
                "asset_type": "source",
                "whole_hash": "b" * 64,
                "binding_state": "bound",
                "bound_role": "source",
                "scope_sku_code": "",
                "retouch_requirement_id": None,
                "root_asset_type": "source",
                "root_scope_sku_code": "",
                "root_retouch_requirement_id": None,
                "detail_visible": False,
                "list_current_version": False,
                "list_approved_version": False,
            }
        )
        oracle.pop("evidence_sha256")
        self.api_oracle.write_text(json.dumps(oracle), encoding="utf-8")
        self.rebind_api_oracle()

        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if base.endswith(("8102", "8103")) and "/revisions" in path:
                body = json.loads(json.dumps(value.body))
                body["data"]["items"][0]["references"] = [
                    {
                        "id": 300,
                        "revision_id": 200,
                        "reference_file_ref_id": 1,
                        "formal_task_asset_id": 99,
                        "sort_order": 0,
                        "ref_id": "ref-1",
                        "file_name": "reference.jpg",
                        "scope": "task",
                        "created_at": "2026-01-01T00:00:00Z",
                    }
                ]
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertTrue(
            any(
                item["violation_code"] == "api.manifest_oracle_mismatch"
                and "reference formal asset differs" in item["detail"]
                for item in evidence["violations"]
            )
        )

    def test_legacy_evidence_summary_must_match_frozen_reason_marker(self) -> None:
        marker = (
            "[migration_v2 "
            f"manifest={'a' * 64} "
            "confidence=confirmed_auto confirmed_by=1 "
            "confirmed_at=2026-01-01T00:00:00Z "
            "evidence_count=1 first_evidence=event-1]"
        )
        rows = [
            json.loads(line)
            for line in self.manifest.read_text(encoding="utf-8").splitlines()
        ]
        for row in rows:
            if row["entity_key"] == "revision:1:task:0:1":
                row["detail_json"]["components"][9] = marker
            row["expected_hash"] = api.component_sha256(
                row["detail_json"]["components"]
            )
        self.manifest.write_text(
            "".join(json.dumps(row) + "\n" for row in rows),
            encoding="utf-8",
        )
        self.rebind_api_oracle()

        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if base.endswith(("8102", "8103")) and "/revisions" in path:
                body = json.loads(json.dumps(value.body))
                row = body["data"]["items"][1]
                row["reason"] = marker
                row["legacy_migration"] = True
                row["evidence_summary"] = api.expected_evidence_summary(
                    marker, digest(b"")
                )
                row["evidence_summary"]["confidence"] = "proposed_review"
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertTrue(
            any(
                item["violation_code"] == "api.manifest_oracle_mismatch"
                and "legacy evidence differs" in item["detail"]
                for item in evidence["violations"]
            )
        )

    def test_wrong_delivery_source_alias_is_hard_failure(self) -> None:
        rows = [
            json.loads(line)
            for line in self.manifest.read_text(encoding="utf-8").splitlines()
        ]
        for row in rows:
            if row["entity_key"] == "revision:1:task:0:1":
                row["detail_json"]["components"][6] = "asset:100:stable-root"
            if row["entity_key"] == "revision-source:1:task:0:1":
                row["detail_json"]["components"] = [
                    "asset:100:stable-root",
                    "source",
                    "",
                    "bound",
                    "source",
                    "",
                    "",
                ]
            row["expected_hash"] = api.component_sha256(
                row["detail_json"]["components"]
            )
        self.manifest.write_text(
            "".join(json.dumps(row) + "\n" for row in rows),
            encoding="utf-8",
        )
        self.rebind_api_oracle()

        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if base.endswith(("8102", "8103")) and path == "/v1/tasks/1/detail":
                body = json.loads(json.dumps(value.body))
                body["asset_versions"] = [
                    {
                        "id": 20,
                        "asset_id": 101,
                        "asset_type": "source",
                    }
                ]
                return result(200, body)
            if base.endswith(("8102", "8103")) and "/revisions" in path:
                body = json.loads(json.dumps(value.body))
                body["data"]["items"][1]["source_task_asset_id"] = 20
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        source_failures = [
            item
            for item in evidence["violations"]
            if item["violation_code"] == "api.manifest_oracle_mismatch"
            and "source differs" in item["detail"]
        ]
        self.assertTrue(source_failures)

    def test_retired_route_must_be_404(self) -> None:
        def requester(base, path, headers):
            if base.endswith("8103") and "warehouse-receive" in path:
                return result(200, {"legacy": True})
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        self.assertIn(
            "api.retired_route_not_404",
            {v["violation_code"] for v in evidence["violations"]},
        )

    def test_revision_order_is_hard_failure(self) -> None:
        def requester(base, path, headers):
            if base.endswith("8102") and "/revisions" in path:
                return result(
                    200,
                    {
                        "data": {
                            "items": [
                                {"revision_no": 1, "items": [], "references": []},
                                {"revision_no": 2, "items": [], "references": []},
                            ],
                            "total": 2,
                        }
                    },
                )
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        self.assertIn(
            "api.revision_order_invalid",
            {v["violation_code"] for v in evidence["violations"]},
        )

    def test_revision_item_order_is_hard_failure_even_if_all_combos_agree(self) -> None:
        def requester(base, path, headers):
            if "/revisions" in path:
                return result(
                    200,
                    {
                        "data": {
                            "items": [
                                {
                                    "revision_no": 1,
                                    "items": [
                                        {"task_asset_id": 20, "sort_order": 2},
                                        {"task_asset_id": 21, "sort_order": 1},
                                    ],
                                    "references": [],
                                }
                            ],
                            "total": 1,
                        }
                    },
                )
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        self.assertIn(
            "api.asset_order_invalid",
            {v["violation_code"] for v in evidence["violations"]},
        )

    def test_exact_hashed_normalization_rule_can_map_value(self) -> None:
        rules = [
            self.rule(
                rule_id="status-map",
                route="/v1/tasks/{task_id}/detail",
                direction="external_external_a->dev_dev_b",
                from_status=200,
                to_status=200,
                operations=[
                    {
                        "op": "map",
                        "path": "/status",
                        "from": "PendingAuditA",
                        "to": "PendingAudit",
                    }
                ],
            )
        ]
        self.write_rules(rules)

        def requester(base, path, headers):
            if path == "/v1/tasks/1/detail":
                if base.endswith("8101"):
                    return result(200, {"status": "PendingAuditA"})
                if base.endswith("8102"):
                    return result(200, {"status": "PendingAudit"})
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        self.assertIn("status-map", evidence["used_rule_ids"])
        # The other four pairwise directions remain unapproved and therefore block.
        self.assertEqual("BLOCKED", evidence["status"])
        pair = [
            violation
            for violation in evidence["violations"]
            if "external_external_a->dev_dev_b" in violation["detail"]
            and "/detail" in violation["entity_key"]
        ]
        self.assertEqual([], pair)

    def test_identity_scoped_rule_records_exact_application(self) -> None:
        self.write_rules(
            [
                self.rule(
                    rule_id="view-inside-status-map",
                    identity="view-inside",
                    route="/v1/tasks/{task_id}/detail",
                    direction="external_external_a->dev_dev_b",
                    from_status=200,
                    to_status=200,
                    operations=[
                        {
                            "op": "map",
                            "path": "/status",
                            "from": "PendingAuditA",
                            "to": "PendingAudit",
                        }
                    ],
                )
            ]
        )

        def requester(base, path, headers):
            if (
                path == "/v1/tasks/1/detail"
                and headers["X-Test-Identity"] == "view-inside"
            ):
                if base.endswith(("8101", "8104")):
                    return result(200, {"status": "PendingAuditA"})
                return result(200, {"status": "PendingAudit"})
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        self.assertEqual(
            [
                {
                    "rule_id": "view-inside-status-map",
                    "rule_identity": "view-inside",
                    "identity": "view-inside",
                    "route": "/v1/tasks/{task_id}/detail",
                    "direction": "external_external_a->dev_dev_b",
                    "from_status": 200,
                    "to_status": 200,
                }
            ],
            evidence["used_rule_applications"],
        )

    def test_rule_rejects_unknown_identity(self) -> None:
        self.write_rules(
            [
                self.rule(
                    rule_id="unknown-identity",
                    identity="not-in-matrix",
                    route="/v1/tasks/{task_id}",
                    direction="external_external_a->dev_dev_b",
                    from_status=200,
                    to_status=403,
                    operations=[],
                )
            ]
        )
        with self.assertRaisesRegex(ValueError, "unknown identity"):
            self.run_compare()

    def test_generic_and_identity_rule_overlap_fails_closed(self) -> None:
        generic = self.rule(
            rule_id="generic",
            route="/v1/tasks/{task_id}/detail",
            direction="external_external_a->dev_dev_b",
            from_status=200,
            to_status=200,
            operations=[],
        )
        scoped = self.rule(
            rule_id="scoped",
            identity="view-inside",
            route="/v1/tasks/{task_id}/detail",
            direction="external_external_a->dev_dev_b",
            from_status=200,
            to_status=200,
            operations=[],
        )
        self.write_rules([generic, scoped])

        def requester(base, path, headers):
            if (
                path == "/v1/tasks/1/detail"
                and headers["X-Test-Identity"] == "view-inside"
                and base.endswith("8101")
            ):
                return result(200, {"different": True})
            return self.requester(base, path, headers)

        with self.assertRaisesRegex(ValueError, "multiple normalization rules"):
            self.run_compare(requester)

    def test_rule_reason_hash_is_mandatory(self) -> None:
        value = self.rule(
            rule_id="bad",
            route="/v1/tasks/{task_id}",
            direction="external_external_a->dev_dev_b",
            from_status=200,
            to_status=200,
            operations=[],
        )
        value["reason_sha256"] = "0" * 64
        self.write_rules([value])
        with self.assertRaisesRegex(ValueError, "reason hash mismatch"):
            self.run_compare()

    def test_rule_hash_is_mandatory(self) -> None:
        value = self.rule(
            rule_id="bad",
            route="/v1/tasks/{task_id}",
            direction="external_external_a->dev_dev_b",
            from_status=200,
            to_status=200,
            operations=[],
        )
        value["rule_sha256"] = "0" * 64
        self.write_rules([value])
        with self.assertRaisesRegex(ValueError, "rule hash mismatch"):
            self.run_compare()

    def test_rule_cannot_remove_allowed_actions_or_ordered_items(self) -> None:
        for path in ("/allowed_actions", "/data/items"):
            value = self.rule(
                rule_id=f"bad-{path.rsplit('/', 1)[-1]}",
                route="/v1/tasks/{task_id}",
                direction="external_external_a->dev_dev_b",
                from_status=200,
                to_status=200,
                operations=[{"op": "remove", "path": path}],
            )
            self.write_rules([value])
            with self.assertRaisesRegex(ValueError, "cannot remove"):
                self.run_compare()

    def test_rule_rejects_protected_map_and_structural_remove(self) -> None:
        protected_map = self.rule(
            rule_id="protected-map",
            route="/v1/tasks/{task_id}/detail",
            direction="external_external_a->dev_dev_b",
            from_status=200,
            to_status=200,
            operations=[
                {
                    "op": "map",
                    "path": "/data/allowed_actions",
                    "from": ["view"],
                    "to": ["view", "edit"],
                }
            ],
        )
        self.write_rules([protected_map])
        with self.assertRaisesRegex(ValueError, "cannot normalize"):
            self.run_compare()
        for index, path in enumerate(("/data/group", "/data/items/0")):
            structural = self.rule(
                rule_id=f"structural-{index}",
                route="/v1/tasks/{task_id}/detail",
                direction="external_external_a->dev_dev_b",
                from_status=200,
                to_status=200,
                operations=[{"op": "remove", "path": path}],
            )
            self.write_rules([structural])
            with self.assertRaisesRegex(ValueError, "cannot remove"):
                self.run_compare()

    def test_task_list_must_exactly_match_manifest(self) -> None:
        self.tasks.write_text("1\n2\n", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "exactly match"):
            self.run_compare()

    def test_design_asset_version_id_is_checked_as_task_asset(self) -> None:
        self.assertEqual(
            {"91"},
            api.task_asset_ids(
                {
                    "data": [
                        {
                            "id": 9,
                            "asset_type": "delivery",
                            "versions": [
                                {
                                    "id": 91,
                                    "asset_type": "delivery",
                                    "version_no": 2,
                                }
                            ],
                        }
                    ]
                }
            ),
        )

    def test_multiple_identities_are_executed_without_header_disclosure(self) -> None:
        os.environ["AB_REVIEWER_HEADERS"] = json.dumps({"Cookie": "session=SECOND"})
        value = json.loads(self.matrix.read_text(encoding="utf-8"))
        value["identities"].append(
            {
                "id": "reviewer",
                "role": "admin",
                "headers_json_env": "AB_REVIEWER_HEADERS",
            }
        )
        self.matrix.write_text(json.dumps(value), encoding="utf-8")

        def requester(base, path, headers):
            self.assertIn(next(iter(headers)), {"Authorization", "Cookie"})
            return self.requester(
                base, path, {"Authorization": "Bearer TOP-SECRET"}
            )

        try:
            evidence = self.run_compare(requester)
        finally:
            os.environ.pop("AB_REVIEWER_HEADERS", None)
        self.assertEqual(
            ["admin", "no-view", "reviewer", "view-inside", "view-outside"],
            [item["id"] for item in evidence["identities"]],
        )
        self.assertNotIn("SECOND", json.dumps(evidence))

    def test_visible_bundle_group_cannot_hide_detail_or_history_by_identity(
        self,
    ) -> None:
        os.environ["AB_VIEWER_HEADERS"] = json.dumps(
            {
                "Cookie": "session=VIEWER",
                "X-Test-Identity": "viewer",
            }
        )
        value = json.loads(self.matrix.read_text(encoding="utf-8"))
        value["identities"].append(
            {
                "id": "viewer",
                "role": "view_only",
                "headers_json_env": "AB_VIEWER_HEADERS",
            }
        )
        self.matrix.write_text(json.dumps(value), encoding="utf-8")

        def requester(base, path, headers):
            is_viewer = "Cookie" in headers
            is_b = base.endswith(("8102", "8103"))
            if is_viewer and is_b and (
                path == "/v1/resource-groups/10" or "/revisions" in path
            ):
                return result(403, {"error": {"code": "forbidden"}})
            return self.requester(
                base, path, {"Authorization": "Bearer TOP-SECRET"}
            )

        try:
            evidence = self.run_compare(requester)
        finally:
            os.environ.pop("AB_VIEWER_HEADERS", None)
        self.assertTrue(
            any(
                item["violation_code"] == "api.manifest_oracle_mismatch"
                and "visible bundle group" in item["detail"]
                for item in evidence["violations"]
            )
        )

    def test_v8_resource_contract_rejects_undeclared_allowed_actions(self) -> None:
        def requester(base, path, headers):
            value = self.requester(base, path, headers)
            if base.endswith(("8102", "8103")) and path.endswith(
                "/resource-bundle"
            ):
                body = json.loads(json.dumps(value.body))
                body["allowed_actions"] = ["resource-group.delete"]
                return result(200, body)
            return value

        evidence = self.run_compare(requester)
        self.assertTrue(
            any(
                item["violation_code"] == "api.manifest_oracle_mismatch"
                and "undeclared allowed_actions" in item["detail"]
                for item in evidence["violations"]
            )
        )

    def test_matrix_rejects_non_local_url(self) -> None:
        value = json.loads(self.matrix.read_text(encoding="utf-8"))
        value["combinations"][0]["base_url"] = "https://example.com"
        self.matrix.write_text(json.dumps(value), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "local URL"):
            self.run_compare()

    def test_matrix_may_reuse_backend_url_across_frontend_combinations(self) -> None:
        value = json.loads(self.matrix.read_text(encoding="utf-8"))
        value["combinations"][2]["base_url"] = value["combinations"][1]["base_url"]
        value["combinations"][3]["base_url"] = value["combinations"][0]["base_url"]
        self.matrix.write_text(json.dumps(value), encoding="utf-8")
        metrics: dict = {}
        evidence = self.run_compare(request_metrics=metrics)
        self.assertEqual("PASS", evidence["status"])
        self.assertEqual([], RELEASE.validate_g6(evidence))
        self.assertEqual(evidence["request_count"], metrics["logical_request_count"])
        self.assertEqual(
            metrics["logical_request_count"] // 2,
            metrics["physical_request_count"],
        )
        self.assertEqual(
            metrics["logical_request_count"] - metrics["physical_request_count"],
            metrics["deduplicated_request_count"],
        )
        self.assertEqual(
            api.REQUEST_CACHE_POLICY_VERSION,
            metrics["cache_policy_version"],
        )
        self.assertEqual(evidence["evidence_sha256"], metrics["api_evidence_sha256"])
        for field in (
            "task_ids_sha256",
            "matrix_sha256",
            "rules_sha256",
            "manifest_sha256",
        ):
            self.assertEqual(evidence[field], metrics[field])
        self.assertEqual(
            metrics["evidence_sha256"],
            digest(
                api.canonical(
                    {
                        key: value
                        for key, value in metrics.items()
                        if key != "evidence_sha256"
                    }
                )
            ),
        )

    def test_matrix_rejects_same_physical_origin_for_a_and_b(self) -> None:
        value = json.loads(self.matrix.read_text(encoding="utf-8"))
        shared = value["combinations"][0]["base_url"]
        for combination in value["combinations"]:
            combination["base_url"] = shared
        self.matrix.write_text(json.dumps(value), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "disjoint physical origins"):
            self.run_compare()

    def test_unique_urls_make_physical_and_logical_counts_equal(self) -> None:
        metrics: dict = {}
        evidence = self.run_compare(request_metrics=metrics)
        self.assertEqual(evidence["request_count"], metrics["logical_request_count"])
        self.assertEqual(
            metrics["logical_request_count"],
            metrics["physical_request_count"],
        )

    def test_request_metrics_are_deterministic(self) -> None:
        value = json.loads(self.matrix.read_text(encoding="utf-8"))
        value["combinations"][2]["base_url"] = value["combinations"][1]["base_url"]
        value["combinations"][3]["base_url"] = value["combinations"][0]["base_url"]
        self.matrix.write_text(json.dumps(value), encoding="utf-8")
        first: dict = {}
        second: dict = {}
        self.run_compare(request_metrics=first, workers=32)
        self.run_compare(request_metrics=second, workers=2)
        self.assertEqual(api.canonical(first), api.canonical(second))

    def test_cli_rejects_same_primary_and_metrics_output_before_requests(self) -> None:
        output = self.root / "same-output.json"
        argv = [
            str(api.MODULE_PATH) if hasattr(api, "MODULE_PATH") else "api_ab_compare.py",
            "--matrix",
            str(self.matrix),
            "--task-ids",
            str(self.tasks),
            "--rules",
            str(self.rules),
            "--manifest",
            str(self.manifest),
            "--api-oracle",
            str(self.api_oracle),
            "--run-id",
            self.run_id,
            "--output",
            str(output),
            "--request-metrics-output",
            str(output),
        ]
        original = sys.argv
        try:
            sys.argv = argv
            with contextlib.redirect_stderr(io.StringIO()):
                with self.assertRaises(SystemExit) as raised:
                    api.main()
        finally:
            sys.argv = original
        self.assertEqual(2, raised.exception.code)
        self.assertFalse(output.exists())

    def test_v3_loader_accepts_independent_source_alias_identity(self) -> None:
        document = self.write_v3_alias_oracle()
        loaded = api.load_asset_identity_map(
            self.api_oracle,
            self.run_id,
            digest(self.manifest.read_bytes()),
        )
        alias = loaded["assets"]["21"]
        self.assertEqual(3, loaded["schema_version"])
        self.assertEqual("non_circular_g6_v3", loaded["oracle_kind"])
        self.assertEqual(document["inputs"], loaded["inputs"])
        self.assertEqual("bound", alias["binding_state"])
        self.assertEqual("source", alias["bound_role"])
        self.assertEqual("1:task:0", alias["bound_resource_locator"])
        self.assertEqual("asset:101:ref-1", alias["manifest_locator"])
        self.assertEqual(
            "alias:v1:1:task:0:origin-task-asset:20",
            loaded["revision_roles"]["1:task:0:1"]["source_locator"],
        )

    def test_v3_loader_rejects_input_and_evidence_tamper(self) -> None:
        document = self.write_v3_alias_oracle()
        del document["inputs"]["source_alias_apply_receipt_sha256"]
        self.write_oracle_document(document)
        with self.assertRaisesRegex(ValueError, "input field contract"):
            api.load_asset_identity_map(
                self.api_oracle,
                self.run_id,
                digest(self.manifest.read_bytes()),
            )
        document = self.write_v3_alias_oracle()
        document["inputs"]["source_alias_apply_receipt_sha256"] = "wrong"
        self.write_oracle_document(document)
        with self.assertRaisesRegex(ValueError, "is not SHA-256"):
            api.load_asset_identity_map(
                self.api_oracle,
                self.run_id,
                digest(self.manifest.read_bytes()),
            )
        document = self.write_v3_alias_oracle()
        document["versions"][1]["content_sha256"] = "c" * 64
        self.api_oracle.write_text(json.dumps(document), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "evidence hash"):
            api.load_asset_identity_map(
                self.api_oracle,
                self.run_id,
                digest(self.manifest.read_bytes()),
            )

    def test_v3_loader_rejects_alias_lineage_binding_and_role_tamper(
        self,
    ) -> None:
        cases = (
            (
                lambda document: document["versions"][1]["provenance"].update(
                    {"origin_locator": "a:999:wrong"}
                ),
                "lineage differs",
            ),
            (
                lambda document: document["versions"][1].update(
                    {"bound_role": "final", "expected_roles": ["final"]}
                ),
                "lineage differs",
            ),
            (
                lambda document: document["versions"][1].update(
                    {"content_sha256": "c" * 64}
                ),
                "lineage differs",
            ),
            (
                lambda document: document["revision_roles"][0].update(
                    {
                        "source_locator": document["versions"][0][
                            "stable_locator"
                        ]
                    }
                ),
                "source binding differs",
            ),
        )
        for mutate, error in cases:
            with self.subTest(error=error):
                document = self.write_v3_alias_oracle()
                mutate(document)
                self.write_oracle_document(document)
                with self.assertRaisesRegex(ValueError, error):
                    api.load_asset_identity_map(
                        self.api_oracle,
                        self.run_id,
                        digest(self.manifest.read_bytes()),
                    )

    def test_cache_key_never_merges_different_url_identity_or_path(self) -> None:
        calls: list[tuple[str, str, tuple[tuple[str, str], ...]]] = []

        def requester(base, path, headers):
            calls.append((base, path, tuple(sorted(headers.items()))))
            return result(200, {"base": base, "path": path})

        urls = {
            "external_external_a": "http://127.0.0.1:8101",
            "dev_dev_b": "http://127.0.0.1:8101",
            "external_dev_b": "http://127.0.0.1:8102",
            "dev_external_a": "http://127.0.0.1:8101",
        }
        runner = api.Runner(
            urls,
            [
                {"id": "admin", "role": "admin"},
                {"id": "reviewer", "role": "reviewer"},
            ],
            {
                "admin": {"Authorization": "same"},
                "reviewer": {"Authorization": "same"},
            },
            [],
            [],
            api.load_manifest_expectations(
                self.manifest,
                self.run_id,
                expected_mapping_sha256=self.mapping_sha256,
                expected_baseline_attestation_sha256=(
                    self.baseline_attestation_sha256
                ),
            ),
            api.load_asset_identity_map(
                self.api_oracle, self.run_id, digest(self.manifest.read_bytes())
            ),
            requester,
        )
        first = runner.request("external_external_a", "admin", "/same")
        reused = runner.request("dev_dev_b", "admin", "/same")
        runner.request("external_dev_b", "admin", "/same")
        runner.request("external_external_a", "reviewer", "/same")
        runner.request("external_external_a", "admin", "/different")
        self.assertIs(first, reused)
        self.assertEqual(5, runner.logical_request_count)
        self.assertEqual(4, runner.physical_request_count)
        self.assertEqual(4, len(calls))

    def test_concurrent_duplicate_exception_is_single_flight_and_shared(self) -> None:
        calls = 0
        calls_lock = threading.Lock()
        entered = threading.Event()
        release = threading.Event()
        failure = RuntimeError("fixture requester failed")

        def requester(base, path, headers):
            nonlocal calls
            with calls_lock:
                calls += 1
            entered.set()
            self.assertTrue(release.wait(5))
            raise failure

        urls = {
            combination: "http://127.0.0.1:8101"
            for combination in api.COMBINATION_IDS
        }
        runner = api.Runner(
            urls,
            [{"id": "admin", "role": "admin"}],
            {"admin": {"Authorization": "fixture"}},
            [],
            [],
            api.load_manifest_expectations(
                self.manifest,
                self.run_id,
                expected_mapping_sha256=self.mapping_sha256,
                expected_baseline_attestation_sha256=(
                    self.baseline_attestation_sha256
                ),
            ),
            api.load_asset_identity_map(
                self.api_oracle, self.run_id, digest(self.manifest.read_bytes())
            ),
            requester,
        )

        def invoke(index):
            try:
                runner.request(
                    api.COMBINATION_IDS[index % len(api.COMBINATION_IDS)],
                    "admin",
                    "/same",
                )
            except RuntimeError as exc:
                return id(exc)
            self.fail("request unexpectedly succeeded")

        with ThreadPoolExecutor(max_workers=16) as executor:
            futures = [executor.submit(invoke, index) for index in range(64)]
            self.assertTrue(entered.wait(5))
            release.set()
            exception_ids = [future.result() for future in futures]
        self.assertEqual(1, calls)
        self.assertEqual(64, runner.logical_request_count)
        self.assertEqual(1, runner.physical_request_count)
        self.assertEqual({id(failure)}, set(exception_ids))

    def test_concurrent_shared_runner_records_every_logical_observation(self) -> None:
        calls = 0
        calls_lock = threading.Lock()
        shared_result = result(200, {"id": 1})

        def requester(base, path, headers):
            nonlocal calls
            with calls_lock:
                calls += 1
            return shared_result

        urls = {
            combination: "http://127.0.0.1:8101"
            for combination in api.COMBINATION_IDS
        }
        runner = api.Runner(
            urls,
            [{"id": "admin", "role": "admin"}],
            {"admin": {"Authorization": "fixture"}},
            [],
            [],
            api.load_manifest_expectations(
                self.manifest,
                self.run_id,
                expected_mapping_sha256=self.mapping_sha256,
                expected_baseline_attestation_sha256=(
                    self.baseline_attestation_sha256
                ),
            ),
            api.load_asset_identity_map(
                self.api_oracle, self.run_id, digest(self.manifest.read_bytes())
            ),
            requester,
        )

        def fetch(index):
            return runner.fetch(
                api.COMBINATION_IDS[index % len(api.COMBINATION_IDS)],
                "admin",
                api.CORE_ROUTES[0],
                "/v1/tasks/1",
                f"task:1:logical:{index}",
            )

        with ThreadPoolExecutor(max_workers=32) as executor:
            observed = list(executor.map(fetch, range(500)))
        self.assertEqual(1, calls)
        self.assertEqual(500, runner.logical_request_count)
        self.assertEqual(1, runner.physical_request_count)
        self.assertEqual(500, len(runner.observations))
        self.assertEqual(500, len(runner.results))
        self.assertTrue(all(item is shared_result for item in observed))


if __name__ == "__main__":
    unittest.main()
