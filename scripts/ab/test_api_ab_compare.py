#!/usr/bin/env python3
from __future__ import annotations

import hashlib
import importlib.util
import json
import os
import pathlib
import sys
import tempfile
import unittest

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


class ComparatorTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = pathlib.Path(self.temp.name)
        self.run_id = "run-test"
        self.tasks = self.root / "tasks.jsonl"
        self.tasks.write_text('{"task_id":1}\n', encoding="utf-8")
        self.manifest = self.root / "manifest.jsonl"
        self.manifest.write_text(
            json.dumps(
                {
                    "run_id": self.run_id,
                    "gate_name": "G01",
                    "entity_key": "task:1",
                }
            )
            + "\n",
            encoding="utf-8",
        )
        os.environ["AB_ADMIN_HEADERS"] = json.dumps(
            {"Authorization": "Bearer TOP-SECRET"}
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
                        }
                    ],
                    "retired_routes": ["/v1/tasks/{task_id}/warehouse-receive"],
                }
            ),
            encoding="utf-8",
        )
        self.rules = self.root / "rules.json"
        self.write_rules([])

    def tearDown(self) -> None:
        os.environ.pop("AB_ADMIN_HEADERS", None)
        self.temp.cleanup()

    def write_rules(self, rules: list[dict]) -> None:
        self.rules.write_text(
            json.dumps({"schema_version": 1, "rules": rules}), encoding="utf-8"
        )

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
                            {"revision_no": 2, "items": [], "references": []},
                            {"revision_no": 1, "items": [], "references": []},
                        ],
                        "total": 2,
                    }
                },
            )
        if "/task-assets/" in path:
            return result(200, {"ok": True})
        if path == "/v1/resource-groups/10":
            return result(200, {"id": 10, "task_id": 1, "scope_kind": "task"})
        if path.endswith("/resource-bundle"):
            return result(
                200,
                {
                    "groups": [
                        {
                            "id": 10,
                            "task_id": 1,
                            "scope_kind": "task",
                            "source_task_asset_id": 20,
                        }
                    ]
                },
            )
        return result(200, {"id": 1, "allowed_actions": ["view"]})

    def run_compare(self, requester=None) -> dict:
        return api.compare(
            matrix_path=self.matrix,
            task_ids_path=self.tasks,
            rules_path=self.rules,
            manifest_path=self.manifest,
            run_id=self.run_id,
            requester=requester or self.requester,
        )

    def test_four_combo_get_only_pass_and_secret_not_in_evidence(self) -> None:
        calls: list[tuple[str, str]] = []

        def requester(base, path, headers):
            calls.append((base, path))
            return self.requester(base, path, headers)

        evidence = self.run_compare(requester)
        self.assertEqual("PASS", evidence["status"])
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
            "api.asset_status_invalid",
            {v["violation_code"] for v in evidence["violations"]},
        )

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
                "role": "reviewer",
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
        self.assertEqual(["admin", "reviewer"], [item["id"] for item in evidence["identities"]])
        self.assertNotIn("SECOND", json.dumps(evidence))

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
        evidence = self.run_compare()
        self.assertEqual("PASS", evidence["status"])


if __name__ == "__main__":
    unittest.main()
