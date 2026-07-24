#!/usr/bin/env python3
from __future__ import annotations

import copy
import hashlib
import importlib.util
import json
import pathlib
import subprocess
import sys
import tempfile
import unittest


MODULE_PATH = pathlib.Path(__file__).with_name("validate_computer_use_evidence.py")
CATALOG_PATH = pathlib.Path(__file__).with_name("computer_use_scenarios.json")
SPEC = importlib.util.spec_from_file_location(
    "validate_computer_use_evidence",
    MODULE_PATH,
)
assert SPEC and SPEC.loader
validator = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = validator
SPEC.loader.exec_module(validator)


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


class ComputerUseEvidenceValidatorTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = pathlib.Path(self.temp.name)
        self.artifact_root = self.root / "artifacts"
        self.artifact_root.mkdir()
        self.catalog = json.loads(CATALOG_PATH.read_text(encoding="utf-8"))
        self.catalog_hash = validator.file_sha256(CATALOG_PATH)
        self.browser_document, self.playwright_document = self._make_documents()
        self.browser_path = self.root / "browser.json"
        self.playwright_path = self.root / "playwright.json"
        self._write_documents(self.browser_document, self.playwright_document)

    def tearDown(self) -> None:
        self.temp.cleanup()

    def _write_artifact(
        self,
        source_kind: str,
        case_slug: str,
        kind: str,
    ) -> dict[str, str]:
        suffix = "png" if kind == "screenshot" else "json"
        if kind == "trace":
            suffix = "zip"
        relative_path = f"{source_kind}/{case_slug}/{kind}.{suffix}"
        target = self.artifact_root / relative_path
        target.parent.mkdir(parents=True, exist_ok=True)
        content = f"{source_kind}|{case_slug}|{kind}\n".encode()
        target.write_bytes(content)
        return {
            "kind": kind,
            "path": relative_path,
            "sha256": sha256_bytes(content),
        }

    def _make_record(
        self,
        scenario: dict[str, object],
        combination: str,
        viewport: str,
        source_kind: str,
        task_id: int,
    ) -> dict[str, object]:
        case_slug = f"{scenario['id']}__{combination}__{viewport}"
        requirements = validator._requirements_for(scenario, combination)
        revision_ids = (
            [task_id * 10 + 1, task_id * 10 + 2]
            if requirements["requires_revision_ids"]
            else []
        )
        assertions: dict[str, object] = {
            "allowed_actions": [
                {
                    "checkpoint": "task_detail",
                    "expected": ["preview", "download"],
                    "observed": ["download", "preview"],
                }
            ],
            "console_unexpected_error_count": 0,
            "network_5xx_count": 0,
            "http_statuses": [
                {
                    "name": f"expected_{status}",
                    "expected_status": status,
                    "actual_status": status,
                }
                for status in requirements["required_http_statuses"]
            ],
        }
        for assertion_name in requirements["required_assertions"]:
            if assertion_name != "allowed_actions_exact":
                assertions[assertion_name] = True
        if requirements["requires_history_drawer"]:
            assertions["history_drawer"] = {
                "opened": True,
                "revision_ids": revision_ids,
                "stage_status_actor_file_time_checked": True,
            }
        artifact_kinds = ["screenshot", "console", "network"]
        if source_kind == validator.SOURCE_PLAYWRIGHT:
            artifact_kinds.append("trace")
        record: dict[str, object] = {
            "schema_version": 1,
            "scenario_id": scenario["id"],
            "combination": combination,
            "viewport": viewport,
            "status": "PASS",
            "executor_id": (
                "browser-operator"
                if source_kind == validator.SOURCE_BROWSER
                else "playwright-operator"
            ),
            "reviewer_id": "independent-reviewer",
            "started_at": "2026-07-23T10:00:00Z",
            "finished_at": "2026-07-23T10:01:00Z",
            "url": f"http://127.0.0.1:18102/tasks/{task_id}",
            "task_id": task_id,
            "revision_ids": revision_ids,
            "assertions": assertions,
            "artifacts": [
                self._write_artifact(source_kind, case_slug, kind)
                for kind in artifact_kinds
            ],
        }
        record["record_sha256"] = validator.record_sha256(record)
        return record

    def _make_documents(self) -> tuple[dict[str, object], dict[str, object]]:
        browser_records: list[dict[str, object]] = []
        playwright_records: list[dict[str, object]] = []
        task_id = 1000
        for scenario in self.catalog["scenarios"]:
            for combination in scenario["required_combinations"]:
                for viewport in scenario["required_viewports"]:
                    task_id += 1
                    browser_records.append(
                        self._make_record(
                            scenario,
                            combination,
                            viewport,
                            validator.SOURCE_BROWSER,
                            task_id,
                        )
                    )
                    playwright_records.append(
                        self._make_record(
                            scenario,
                            combination,
                            viewport,
                            validator.SOURCE_PLAYWRIGHT,
                            task_id,
                        )
                    )
        common = {
            "schema_version": 1,
            "run_id": "g7-test-run",
            "scenario_catalog_sha256": self.catalog_hash,
        }
        return (
            {
                **common,
                "source_kind": validator.SOURCE_BROWSER,
                "records": browser_records,
            },
            {
                **common,
                "source_kind": validator.SOURCE_PLAYWRIGHT,
                "records": playwright_records,
            },
        )

    def _write_documents(
        self,
        browser_document: dict[str, object],
        playwright_document: dict[str, object],
    ) -> None:
        self.browser_path.write_text(
            json.dumps(browser_document, ensure_ascii=False),
            encoding="utf-8",
        )
        self.playwright_path.write_text(
            json.dumps(playwright_document, ensure_ascii=False),
            encoding="utf-8",
        )

    def _validate(
        self,
        browser_document: dict[str, object] | None = None,
        playwright_document: dict[str, object] | None = None,
    ) -> dict[str, object]:
        self._write_documents(
            browser_document or self.browser_document,
            playwright_document or self.playwright_document,
        )
        return validator.validate_evidence(
            catalog_path=CATALOG_PATH,
            browser_evidence_path=self.browser_path,
            playwright_evidence_path=self.playwright_path,
            artifact_root=self.artifact_root,
        )

    @staticmethod
    def _rehash(record: dict[str, object]) -> None:
        record["record_sha256"] = validator.record_sha256(record)

    def test_full_catalog_passes_with_two_independent_sources(self) -> None:
        report = self._validate()

        expected_count = sum(
            len(scenario["required_combinations"])
            * len(scenario["required_viewports"])
            for scenario in self.catalog["scenarios"]
        )
        self.assertEqual("PASS", report["status"])
        self.assertEqual(expected_count, report["required_case_count"])
        self.assertEqual(expected_count, report["passed_case_count"])
        self.assertEqual(0, report["failed_case_count"])
        self.assertEqual(1.0, report["critical_pass_rate"])
        self.assertEqual([], report["failures"])
        self.assertTrue(all(case["pair_sha256"] for case in report["cases"]))

    def test_missing_source_record_fails_critical_coverage(self) -> None:
        playwright = copy.deepcopy(self.playwright_document)
        playwright["records"].pop()

        report = self._validate(playwright_document=playwright)

        self.assertEqual("FAIL", report["status"])
        self.assertEqual(1, report["failed_case_count"])
        self.assertIn(
            "missing_playwright_record",
            {failure["code"] for failure in report["failures"]},
        )

    def test_actor_independence_is_enforced(self) -> None:
        browser = copy.deepcopy(self.browser_document)
        playwright = copy.deepcopy(self.playwright_document)
        browser_record = browser["records"][0]
        playwright_record = playwright["records"][0]
        browser_record["reviewer_id"] = browser_record["executor_id"]
        playwright_record["executor_id"] = browser_record["executor_id"]
        self._rehash(browser_record)
        self._rehash(playwright_record)

        report = self._validate(browser, playwright)
        codes = {failure["code"] for failure in report["failures"]}

        self.assertEqual("FAIL", report["status"])
        self.assertIn("self_review", codes)
        self.assertIn("executor_independence", codes)
        self.assertIn("review_independence", codes)

    def test_runtime_failures_and_hash_tampering_are_rejected(self) -> None:
        cases = (
            ("console_unexpected_error_count", 1, "console_errors"),
            ("network_5xx_count", 1, "network_5xx"),
        )
        for field, value, expected_code in cases:
            with self.subTest(field=field):
                browser = copy.deepcopy(self.browser_document)
                record = browser["records"][0]
                record["assertions"][field] = value
                self._rehash(record)
                report = self._validate(browser_document=browser)
                self.assertEqual("FAIL", report["status"])
                self.assertIn(
                    expected_code,
                    {failure["code"] for failure in report["failures"]},
                )

        browser = copy.deepcopy(self.browser_document)
        browser["records"][0]["url"] = "http://127.0.0.1:19999/tampered"
        report = self._validate(browser_document=browser)
        self.assertEqual("FAIL", report["status"])
        self.assertIn(
            "record_hash",
            {failure["code"] for failure in report["failures"]},
        )

    def test_required_403_and_410_are_enforced(self) -> None:
        for scenario_id, expected_status in (
            ("unauthorized_asset_403", 403),
            ("historical_asset_unavailable_410", 410),
        ):
            with self.subTest(scenario_id=scenario_id):
                browser = copy.deepcopy(self.browser_document)
                record = next(
                    item
                    for item in browser["records"]
                    if item["scenario_id"] == scenario_id
                )
                record["assertions"]["http_statuses"] = []
                self._rehash(record)
                report = self._validate(browser_document=browser)
                matching = [
                    failure
                    for failure in report["failures"]
                    if failure["code"] == "required_http_status"
                ]
                self.assertTrue(matching)
                self.assertIn(str(expected_status), matching[0]["detail"])

    def test_history_and_allowed_actions_must_match_both_sources(self) -> None:
        browser = copy.deepcopy(self.browser_document)
        record = next(
            item
            for item in browser["records"]
            if item["scenario_id"] == "baseline_four_edge_readonly"
            and item["combination"] == "devplus_devplus"
        )
        record["assertions"]["history_drawer"]["opened"] = False
        record["assertions"]["allowed_actions"][0]["observed"] = ["preview"]
        self._rehash(record)

        report = self._validate(browser_document=browser)
        codes = {failure["code"] for failure in report["failures"]}

        self.assertEqual("FAIL", report["status"])
        self.assertIn("history_drawer", codes)
        self.assertIn("allowed_actions", codes)
        self.assertIn("pair_history", codes)
        self.assertIn("pair_allowed_actions", codes)

    def test_baseline_requirements_are_conditional_per_combination(self) -> None:
        baseline = [
            record
            for record in self.browser_document["records"]
            if record["scenario_id"] == "baseline_four_edge_readonly"
        ]
        self.assertEqual(8, len(baseline))
        for record in baseline:
            with self.subTest(
                combination=record["combination"],
                viewport=record["viewport"],
            ):
                assertions = record["assertions"]
                self.assertTrue(assertions["page_matches_manifest"])
                self.assertTrue(assertions["assets_match"])
                self.assertTrue(assertions["allowed_actions"])
                if record["combination"] == "devplus_devplus":
                    self.assertTrue(record["revision_ids"])
                    self.assertIn("history_drawer", assertions)
                else:
                    self.assertEqual([], record["revision_ids"])
                    self.assertNotIn("history_drawer", assertions)
        self.assertEqual("PASS", self._validate()["status"])

    def test_non_v8_baseline_still_requires_page_assets_and_actions(self) -> None:
        browser = copy.deepcopy(self.browser_document)
        record = next(
            item
            for item in browser["records"]
            if item["scenario_id"] == "baseline_four_edge_readonly"
            and item["combination"] == "external_external"
        )
        record["assertions"]["assets_match"] = False
        self._rehash(record)

        report = self._validate(browser_document=browser)

        self.assertEqual("FAIL", report["status"])
        self.assertIn(
            "required_assertion",
            {failure["code"] for failure in report["failures"]},
        )

    def test_devplus_baseline_still_requires_revision_history(self) -> None:
        browser = copy.deepcopy(self.browser_document)
        record = next(
            item
            for item in browser["records"]
            if item["scenario_id"] == "baseline_four_edge_readonly"
            and item["combination"] == "devplus_devplus"
        )
        record["revision_ids"] = []
        record["assertions"].pop("history_drawer")
        self._rehash(record)

        report = self._validate(browser_document=browser)
        codes = {failure["code"] for failure in report["failures"]}

        self.assertEqual("FAIL", report["status"])
        self.assertIn("revision_ids", codes)
        self.assertIn("history_drawer", codes)

    def test_other_scenarios_keep_their_revision_history_contract(self) -> None:
        browser = copy.deepcopy(self.browser_document)
        record = next(
            item
            for item in browser["records"]
            if item["scenario_id"] == "design_first_submit_audit"
        )
        record["revision_ids"] = []
        record["assertions"].pop("history_drawer")
        self._rehash(record)

        report = self._validate(browser_document=browser)
        codes = {failure["code"] for failure in report["failures"]}

        self.assertEqual("FAIL", report["status"])
        self.assertIn("revision_ids", codes)
        self.assertIn("history_drawer", codes)

    def test_catalog_conditional_contract_cannot_weaken_core_assertions(self) -> None:
        catalog = copy.deepcopy(self.catalog)
        baseline = catalog["scenarios"][0]
        baseline["requirements_by_combination"]["external_external"][
            "required_assertions"
        ] = ["page_matches_manifest", "allowed_actions_exact"]

        with self.assertRaisesRegex(
            validator.ValidationInputError,
            "cannot weaken assertions",
        ):
            validator._validate_catalog(catalog)

        catalog = copy.deepcopy(self.catalog)
        del catalog["scenarios"][0]["requirements_by_combination"][
            "devplus_external"
        ]
        with self.assertRaisesRegex(
            validator.ValidationInputError,
            "every and only required combination",
        ):
            validator._validate_catalog(catalog)

    def test_artifact_bytes_and_source_paths_are_bound(self) -> None:
        browser = copy.deepcopy(self.browser_document)
        playwright = copy.deepcopy(self.playwright_document)
        browser_artifact = browser["records"][0]["artifacts"][0]
        target = self.artifact_root / browser_artifact["path"]
        original = target.read_bytes()
        target.write_bytes(original + b"tampered")
        try:
            report = self._validate(browser, playwright)
        finally:
            target.write_bytes(original)
        self.assertEqual("FAIL", report["status"])
        self.assertIn(
            "artifact_hash",
            {failure["code"] for failure in report["failures"]},
        )

        browser = copy.deepcopy(self.browser_document)
        playwright = copy.deepcopy(self.playwright_document)
        browser_artifact = browser["records"][0]["artifacts"][0]
        playwright_artifact = playwright["records"][0]["artifacts"][0]
        playwright_artifact.update(browser_artifact)
        self._rehash(playwright["records"][0])
        report = self._validate(browser, playwright)
        self.assertEqual("FAIL", report["status"])
        self.assertIn(
            "source_artifact_independence",
            {failure["code"] for failure in report["failures"]},
        )

    def test_invalid_remote_http_url_is_rejected(self) -> None:
        browser = copy.deepcopy(self.browser_document)
        record = browser["records"][0]
        record["url"] = "http://example.test/tasks/1"
        self._rehash(record)

        report = self._validate(browser_document=browser)

        self.assertEqual("FAIL", report["status"])
        self.assertIn("url", {failure["code"] for failure in report["failures"]})

    def test_cli_writes_pass_report_and_refuses_overwrite(self) -> None:
        output = self.root / "g7-report.json"
        command = [
            sys.executable,
            str(MODULE_PATH),
            "--scenarios",
            str(CATALOG_PATH),
            "--browser-evidence",
            str(self.browser_path),
            "--playwright-evidence",
            str(self.playwright_path),
            "--artifact-root",
            str(self.artifact_root),
            "--output",
            str(output),
        ]

        first = subprocess.run(command, capture_output=True, text=True, check=False)
        second = subprocess.run(command, capture_output=True, text=True, check=False)

        self.assertEqual(0, first.returncode, first.stderr)
        self.assertEqual("PASS", json.loads(output.read_text())["status"])
        self.assertEqual(2, second.returncode)
        self.assertIn("refusing to overwrite", second.stderr)


if __name__ == "__main__":
    unittest.main()
