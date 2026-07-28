import copy
import hashlib
import importlib.util
import json
import os
import shutil
import subprocess
import tempfile
import unittest
import urllib.parse
import zipfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
RUNNER = ROOT / "scripts" / "ab" / "run_g7_playwright.mjs"
CATALOG = ROOT / "scripts" / "ab" / "computer_use_scenarios.json"
COMBINATIONS = {
    "external_external": "http://127.0.0.1:18101",
    "devplus_devplus": "http://127.0.0.1:18102",
    "external_devplus": "http://127.0.0.1:18103",
    "devplus_external": "http://127.0.0.1:18104",
}


def load_validator():
    path = ROOT / "scripts" / "ab" / "validate_computer_use_evidence.py"
    spec = importlib.util.spec_from_file_location("g7_validator_for_runner", path)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


VALIDATOR = load_validator()


def file_sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write_json(path: Path, value: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )


def requirements_for(scenario: dict, combination: str) -> dict:
    return copy.deepcopy(
        scenario.get("requirements_by_combination", {}).get(
            combination,
            {
                "requires_task_id": scenario["requires_task_id"],
                "requires_revision_ids": scenario["requires_revision_ids"],
                "requires_history_drawer": scenario["requires_history_drawer"],
                "required_http_statuses": scenario["required_http_statuses"],
                "required_assertions": scenario["required_assertions"],
            },
        )
    )


class RunG7PlaywrightTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.temp = tempfile.TemporaryDirectory()
        cls.root = Path(cls.temp.name)
        cls.run_id = "fixture-g7-run"
        cls.secret_root = cls.root / "secrets" / cls.run_id
        cls.secret_root.mkdir(parents=True)
        os.chmod(cls.root / "secrets", 0o700)
        os.chmod(cls.secret_root, 0o700)
        cls.catalog = json.loads(CATALOG.read_text(encoding="utf-8"))
        cls.samples = cls.root / "samples.json"
        cls.api_oracle = cls.root / "api-oracle.json"
        cls.fixture_receipt = cls.root / "fixture-receipt.json"
        cls.auth_attestation = cls.root / "auth-attestation.json"
        cls.combined_receipt = cls.root / "edge-combined.json"
        cls.receipts: list[Path] = []
        cls.admin_states: dict[str, Path] = {}
        cls.denied_state = cls.secret_root / "denied.json"
        cls.build_inputs()

    @classmethod
    def tearDownClass(cls) -> None:
        cls.temp.cleanup()

    @classmethod
    def write_state(cls, path: Path, role: str, origin: str) -> None:
        write_json(
            path,
            {
                "cookies": [],
                "origins": [
                    {
                        "origin": origin,
                        "localStorage": [
                            {
                                "name": "access_token",
                                "value": f"super-secret-cookie-{role}",
                            }
                        ],
                    }
                ],
            },
        )
        os.chmod(path, 0o600)

    @classmethod
    def build_inputs(cls) -> None:
        catalog_hash = file_sha256(CATALOG)
        task_response_sha256 = hashlib.sha256(
            json.dumps(
                {
                    "data": {
                        "id": 1,
                        "task_type": "sku_planning",
                        "allowed_actions": ["preview"],
                        "planning": {"revision": 1},
                        "access_token": "response-secret-token-must-not-survive",
                    }
                },
                ensure_ascii=False,
                separators=(",", ":"),
            ).encode()
        ).hexdigest()
        edges = {}
        for index, (combination, origin) in enumerate(COMBINATIONS.items(), 1):
            edge = {
                "origin": origin,
                "edge": combination,
                "frontend_sha256": hashlib.sha256(
                    f"frontend-{index}".encode()
                ).hexdigest(),
                "backend_sha256": hashlib.sha256(
                    f"backend-{index}".encode()
                ).hexdigest(),
                "fixture_identity": "fixture-v2-test",
            }
            edges[combination] = edge
            receipt = {
                "schema_version": 1,
                "gate": "G7",
                "status": "PASS",
                "combination": combination,
                "edge_identity": edge,
            }
            receipt["receipt_sha256"] = VALIDATOR.canonical_sha256(receipt)
            receipt_path = cls.root / f"edge-{combination}.json"
            write_json(receipt_path, receipt)
            cls.receipts.append(receipt_path)
            state_path = cls.secret_root / f"admin-{combination}.json"
            cls.write_state(state_path, f"admin-{combination}", origin)
            cls.admin_states[combination] = state_path
        combined = {
            "schema_version": 1,
            "gate": "G7",
            "status": "PASS",
            "edges": edges,
        }
        combined["receipt_sha256"] = VALIDATOR.canonical_sha256(combined)
        write_json(cls.combined_receipt, combined)
        cls.write_state(
            cls.denied_state,
            "denied",
            COMBINATIONS["devplus_devplus"],
        )

        samples = []
        api_oracle_cases = []
        for scenario in cls.catalog["scenarios"]:
            matrix = []
            for combination in scenario["required_combinations"]:
                requirements = requirements_for(scenario, combination)
                probes = []
                for status in requirements["required_http_statuses"]:
                    probes.append(
                        {
                            "kind": f"status_{status}",
                            "method": "GET",
                            "path": f"/probe/{status}",
                            "expected_status": status,
                        }
                    )
                for viewport in scenario["required_viewports"]:
                    is_task1264 = scenario["id"] == "retouch_reopen_task1264"
                    resource_oracle = (
                        {
                            "kind": (
                                "v8_missing_resource_group"
                                if scenario["id"] == "missing_resource_group_negative"
                                else "v8_wrong_scope_rejected"
                                if scenario["id"] == "wrong_scope_negative"
                                else "v8_expected_no_resource_groups"
                                if scenario["id"]
                                in {
                                    "purchase_to_sku_planning",
                                }
                                else "v8_resource_groups"
                            )
                        }
                        if combination == "devplus_devplus"
                        else {
                            "kind": {
                                "external_external": "legacy_task_snapshot",
                                "external_devplus": "legacy_frontend_task_snapshot",
                                "devplus_external": (
                                    "frontend_rollback_compatibility"
                                ),
                            }[combination],
                            "task_response_sha256": task_response_sha256,
                            **(
                                {
                                    "approved_assertion": (
                                        "approved_compatibility_difference_only"
                                    )
                                }
                                if combination == "devplus_external"
                                else {}
                            ),
                        }
                    )
                    matrix.append(
                        {
                            "combination": combination,
                            "viewport": viewport,
                            "requirements": requirements,
                            "task_id": 1264 if is_task1264 else 1,
                            "resource_ids": (
                                (
                                    ["group:1264:retouch_requirement:45"]
                                    if is_task1264
                                    else ["task_asset_group:10"]
                                )
                                if resource_oracle["kind"]
                                in {
                                    "v8_resource_groups",
                                    "v8_wrong_scope_rejected",
                                }
                                else []
                            ),
                            "revision_ids": (
                                ([635, 636] if is_task1264 else [1])
                                if resource_oracle["kind"]
                                in {
                                    "v8_resource_groups",
                                    "v8_wrong_scope_rejected",
                                }
                                and requirements["requires_revision_ids"]
                                else []
                            ),
                            "resource_oracle": resource_oracle,
                            "allowed_actions": [
                                {
                                    "checkpoint": "task_detail",
                                    "expected": ["preview"],
                                }
                            ],
                            "http_probes": probes,
                            "oracle_sha256": hashlib.sha256(
                                f"{scenario['id']}/{combination}/{viewport}".encode()
                            ).hexdigest(),
                        }
                    )
                api_oracle_cases.append(
                    {
                        "scenario_id": scenario["id"],
                        "combination": combination,
                        "allowed_actions": [
                            {
                                "checkpoint": "task_detail",
                                "expected": ["preview"],
                            }
                        ],
                        "http_probes": probes,
                        "resource_oracle": resource_oracle,
                    }
                )
            sample = {
                "scenario_id": scenario["id"],
                "status": "READY",
                "required_combinations": scenario["required_combinations"],
                "required_viewports": scenario["required_viewports"],
                "coverage_matrix": matrix,
            }
            if scenario["id"] == "multi_source_zip_bundle":
                sample["revision_facts"] = [
                    {
                        "resource_key": "task_asset_group:10",
                        "predicted_revision_id": 1,
                        "revision_no": 1,
                        "source_bundle": {
                            "task_asset_id": 9001,
                            "bundle_sha256": (
                                "6afba5980e37a4798fe3c6f75638e585"
                                "606180002256850855df240037b61093"
                            ),
                            "ordered_member_task_asset_ids": [101, 102],
                        },
                    }
                ]
            sample["sample_sha256"] = VALIDATOR.canonical_sha256(sample)
            samples.append(sample)
        mapping_sha256 = hashlib.sha256(b"mapping").hexdigest()
        canonical_sha256 = hashlib.sha256(b"canonical").hexdigest()
        fixture_receipt = {
            "schema_version": 2,
            "gate": "G7",
            "status": "APPLIED_VERIFIED_PENDING_UI_AND_CLEANUP",
            "run_id": "fixture-receipt-v2",
            "scenarios": [scenario["id"] for scenario in cls.catalog["scenarios"]],
        }
        fixture_receipt["receipt_payload_sha256"] = VALIDATOR.canonical_sha256(
            fixture_receipt
        )
        write_json(cls.fixture_receipt, fixture_receipt)
        api_oracle = {
            "schema_version": 1,
            "gate": "G7",
            "status": "PASS",
            "source_kind": "reviewed_api_allowed_actions",
            "reviewed_by": 1,
            "reviewed_at": "2026-07-23T12:00:00Z",
            "review_note": "fixture reviewed oracle",
            "input_sha256": {
                "scenario_catalog_sha256": catalog_hash,
                "mapping_sha256": mapping_sha256,
                "canonical_entities_sha256": canonical_sha256,
                "edge_receipt_sha256": file_sha256(cls.combined_receipt),
                "fixture_receipt_sha256": file_sha256(cls.fixture_receipt),
            },
            "cases": api_oracle_cases,
        }
        api_oracle["manifest_sha256"] = VALIDATOR.canonical_sha256(api_oracle)
        write_json(cls.api_oracle, api_oracle)
        document = {
            "schema_version": 1,
            "gate": "G7",
            "status": "PASS",
            "mode": "final",
            "input_sha256": {
                "scenario_catalog_sha256": catalog_hash,
                "mapping_sha256": mapping_sha256,
                "canonical_entities_sha256": canonical_sha256,
                "edge_receipt_sha256": file_sha256(cls.combined_receipt),
                "fixture_receipt_sha256": file_sha256(cls.fixture_receipt),
                "api_oracle_sha256": file_sha256(cls.api_oracle),
            },
            "oracle_contract": {
                "kind": "reviewed_api_allowed_actions_v1",
                "edge_receipt_manifest_sha256": combined["receipt_sha256"],
                "api_oracle_manifest_sha256": api_oracle["manifest_sha256"],
                "fixture_receipt_payload_sha256": fixture_receipt[
                    "receipt_payload_sha256"
                ],
                "executor_supplied_oracle_forbidden": True,
            },
            "scenario_count": len(cls.catalog["scenarios"]),
            "sample_count": len(samples),
            "sealed_edges": edges,
            "samples": samples,
        }
        document["manifest_sha256"] = VALIDATOR.canonical_sha256(document)
        write_json(cls.samples, document)
        admin_attestations = {}
        for combination, state in cls.admin_states.items():
            admin_attestations[combination] = {
                "identity_id": "1",
                "role": "Admin",
                "origin": COMBINATIONS[combination],
                "storage_state_sha256": file_sha256(state),
            }
        attestation = {
            "schema_version": 1,
            "gate": "G7",
            "status": "PASS",
            "run_id": cls.run_id,
            "secret_root_sha256": hashlib.sha256(
                str(cls.secret_root.resolve()).encode()
            ).hexdigest(),
            "states": {
                "admin": admin_attestations,
                "denied": {
                    "identity_id": "339",
                    "role": "Member",
                    "origin": COMBINATIONS["devplus_devplus"],
                    "combination": "devplus_devplus",
                    "storage_state_sha256": file_sha256(cls.denied_state),
                },
            },
        }
        attestation["attestation_sha256"] = VALIDATOR.canonical_sha256(attestation)
        write_json(cls.auth_attestation, attestation)

    def command(
        self,
        mode: str,
        artifact_root: Path,
        output: Path,
        *,
        combined_receipt: bool = False,
    ) -> list[str]:
        entrypoint = (
            ROOT / "scripts" / "ab" / "test_run_g7_playwright_fixture.mjs"
            if mode == "execute"
            else RUNNER
        )
        command = [
            "node",
            str(entrypoint),
            f"--{mode}",
            "--scenarios",
            str(CATALOG),
            "--samples",
            str(self.samples),
            "--api-oracle",
            str(self.api_oracle),
            "--fixture-receipt",
            str(self.fixture_receipt),
            "--auth-attestation",
            str(self.auth_attestation),
        ]
        receipts = [self.combined_receipt]
        for receipt in receipts:
            command.extend(["--edge-receipt", str(receipt)])
        command.extend(["--secret-root", str(self.secret_root)])
        for combination, state in self.admin_states.items():
            command.extend(["--admin-state", f"{combination}={state}"])
        command.extend(
            [
                "--denied-state",
                str(self.denied_state),
                "--artifact-root",
                str(artifact_root),
                "--output",
                str(output),
                "--run-id",
                self.run_id,
                "--executor-id",
                "playwright-executor",
                "--reviewer-id",
                "playwright-reviewer",
            ]
        )
        return command

    def run_command(
        self,
        command: list[str],
        timeout: int = 180,
        env: dict[str, str] | None = None,
    ) -> subprocess.CompletedProcess:
        return subprocess.run(
            command,
            cwd=ROOT,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=timeout,
            check=False,
            env={**os.environ, **(env or {})},
        )

    def test_dry_run_validates_66_case_plan_without_disclosing_secret_paths(self) -> None:
        artifact_root = self.root / "dry-artifacts"
        output = self.root / "dry-plan.json"
        result = self.run_command(
            self.command(
                "dry-run",
                artifact_root,
                output,
                combined_receipt=True,
            )
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn(
            "super-secret-cookie",
            f"{result.stdout}\n{result.stderr}",
        )
        plan = json.loads(output.read_text(encoding="utf-8"))
        self.assertEqual(plan["status"], "PASS")
        self.assertEqual(plan["case_count"], 66)
        task1264_cases = [
            row
            for row in plan["cases"]
            if row["case_key"].startswith("retouch_reopen_task1264/")
        ]
        self.assertEqual(len(task1264_cases), 2)
        self.assertEqual(
            {
                (
                    row["task_id"],
                    tuple(row["resource_ids"]),
                    tuple(row["revision_ids"]),
                )
                for row in task1264_cases
            },
            {
                (
                    1264,
                    ("group:1264:retouch_requirement:45",),
                    (635, 636),
                )
            },
        )
        self.assertEqual(plan["auth_state_files_validated"], 5)
        self.assertFalse(plan["auth_state_paths_disclosed"])
        self.assertEqual(
            plan["auth_attestation_sha256"],
            json.loads(self.auth_attestation.read_text())["attestation_sha256"],
        )
        self.assertEqual(
            plan["upstream_bindings"]["apiOracleManifestSha256"],
            json.loads(self.api_oracle.read_text())["manifest_sha256"],
        )
        serialized = output.read_text(encoding="utf-8")
        self.assertNotIn(str(self.secret_root), serialized)
        self.assertNotIn("super-secret-cookie", serialized)

    def test_auth_attestation_requires_authority_admin_role_case(self) -> None:
        accepted_output = self.root / "admin-role-case-plan.json"
        accepted_result = self.run_command(
            self.command(
                "dry-run",
                self.root / "admin-role-case-artifacts",
                accepted_output,
            )
        )
        self.assertEqual(accepted_result.returncode, 0, accepted_result.stderr)

        lowercase_attestation_path = self.root / "lowercase-admin-attestation.json"
        lowercase_attestation = json.loads(self.auth_attestation.read_text())
        for row in lowercase_attestation["states"]["admin"].values():
            row["role"] = "admin"
        lowercase_attestation.pop("attestation_sha256")
        lowercase_attestation["attestation_sha256"] = VALIDATOR.canonical_sha256(
            lowercase_attestation
        )
        write_json(lowercase_attestation_path, lowercase_attestation)
        rejected_command = self.command(
            "dry-run",
            self.root / "lowercase-admin-artifacts",
            self.root / "lowercase-admin-plan.json",
        )
        rejected_command[rejected_command.index("--auth-attestation") + 1] = str(
            lowercase_attestation_path
        )
        rejected_result = self.run_command(rejected_command)
        self.assertNotEqual(rejected_result.returncode, 0)
        self.assertIn(
            "admin auth attestation mismatch",
            rejected_result.stderr,
        )

    def test_execute_writes_validator_compatible_sanitized_66_case_evidence(self) -> None:
        artifact_root = self.root / "execute-artifacts"
        output = self.root / "playwright-evidence.json"
        result = self.run_command(
            self.command("execute", artifact_root, output),
            timeout=300,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        evidence = json.loads(output.read_text(encoding="utf-8"))
        self.assertEqual(len(evidence["records"]), 66)
        self.assertEqual(
            {record["status"] for record in evidence["records"]},
            {"PASS"},
        )
        self.assertNotIn(str(self.secret_root), output.read_text(encoding="utf-8"))

        first_trace = next(
            artifact_root / artifact["path"]
            for artifact in evidence["records"][0]["artifacts"]
            if artifact["kind"] == "trace"
        )
        with zipfile.ZipFile(first_trace) as archive:
            self.assertIn("trace.trace", archive.namelist())
            self.assertIn("trace.network", archive.namelist())
            self.assertFalse(
                any(name == "resources" or name.startswith("resources/") for name in archive.namelist())
            )
            trace_text = "\n".join(
                archive.read(name).decode("utf-8", errors="ignore")
                for name in archive.namelist()
                if not name.endswith("/")
            )
        self.assertNotIn("super-secret-cookie", trace_text)
        self.assertNotIn("response-secret-token-must-not-survive", trace_text)
        self.assertNotRegex(trace_text.lower(), r'"authorization"\s*:')
        self.assertNotRegex(trace_text.lower(), r'"cookie"\s*:')

        browser_records = []
        for record in evidence["records"]:
            browser_record = copy.deepcopy(record)
            browser_record["source_kind"] = "browser_computer_use"
            browser_record["executor_id"] = "browser-executor"
            browser_record["reviewer_id"] = "browser-reviewer"
            browser_artifacts = []
            case_dir = artifact_root / "browser" / record["record_sha256"]
            case_dir.mkdir(parents=True)
            for artifact in record["artifacts"]:
                if artifact["kind"] == "trace":
                    continue
                source = artifact_root / artifact["path"]
                target = case_dir / source.name
                if artifact["kind"] == "screenshot":
                    shutil.copyfile(source, target)
                else:
                    document = json.loads(source.read_text(encoding="utf-8"))
                    document["source_kind"] = "browser_computer_use"
                    write_json(target, document)
                browser_artifacts.append(
                    {
                        "kind": artifact["kind"],
                        "path": target.relative_to(artifact_root).as_posix(),
                        "sha256": file_sha256(target),
                    }
                )
            browser_record["artifacts"] = browser_artifacts
            browser_record.pop("record_sha256", None)
            browser_record["record_sha256"] = VALIDATOR.record_sha256(browser_record)
            browser_records.append(browser_record)
        browser_evidence = {
            "schema_version": 1,
            "source_kind": "browser_computer_use",
            "scenario_catalog_sha256": file_sha256(CATALOG),
            "samples_sha256": file_sha256(self.samples),
            "run_id": evidence["run_id"],
            "records": browser_records,
        }
        browser_path = self.root / "browser-evidence.json"
        write_json(browser_path, browser_evidence)
        report = VALIDATOR.validate_evidence(
            catalog_path=CATALOG,
            samples_path=self.samples,
            browser_evidence_path=browser_path,
            playwright_evidence_path=output,
            artifact_root=artifact_root,
        )
        self.assertEqual(report["status"], "PASS", report["failures"][:5])
        self.assertEqual(report["passed_case_count"], 66)
        self.assertEqual(report["failed_case_count"], 0)

        unauthorized = next(
            record
            for record in evidence["records"]
            if record["scenario_id"] == "unauthorized_asset_403"
            and record["viewport"] == "desktop"
        )
        network_path = next(
            artifact_root / artifact["path"]
            for artifact in unauthorized["artifacts"]
            if artifact["kind"] == "network"
        )
        requests = json.loads(network_path.read_text())["requests"]
        controls = [
            row
            for row in requests
            if row["url"].endswith("/probe/403") and row["method"] == "GET"
        ]
        self.assertEqual({row["status"] for row in controls}, {200, 403})

    def test_execute_authenticates_local_storage_only_zero_assignment_denied(
        self,
    ) -> None:
        artifact_root = self.root / "zero-assignment-artifacts"
        output = self.root / "zero-assignment-evidence.json"
        result = self.run_command(
            self.command("execute", artifact_root, output),
            timeout=300,
            env={"G7_FIXTURE_ASSERT_ZERO_ASSIGNMENT_SHAPE": "1"},
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn(
            "super-secret-cookie",
            f"{result.stdout}\n{result.stderr}",
        )
        evidence = json.loads(output.read_text(encoding="utf-8"))
        self.assertEqual(len(evidence["records"]), 66)
        state_documents = [
            json.loads(path.read_text(encoding="utf-8"))
            for path in [*self.admin_states.values(), self.denied_state]
        ]
        self.assertTrue(
            all(
                document["cookies"] == []
                and len(document["origins"]) == 1
                and document["origins"][0]["localStorage"][0]["name"]
                == "access_token"
                for document in state_documents
            )
        )
        tokens = [
            document["origins"][0]["localStorage"][0]["value"].encode()
            for document in state_documents
        ]
        evidence_files = [output, *artifact_root.rglob("*")]
        for evidence_file in evidence_files:
            if not evidence_file.is_file():
                continue
            payload = evidence_file.read_bytes()
            self.assertNotIn(b'"Member"', payload)
            for token in tokens:
                self.assertNotIn(token, payload)

    def test_execute_aborts_non_read_request_before_fixture_route(self) -> None:
        attempts = [
            ("POST", "G7_FIXTURE_ATTEMPT_POST"),
            ("SERVICE_WORKER", "G7_FIXTURE_ATTEMPT_SERVICE_WORKER"),
            ("WEBSOCKET", "G7_FIXTURE_ATTEMPT_WEBSOCKET"),
        ]
        for method, environment_name in attempts:
            with self.subTest(method=method):
                artifact_root = self.root / f"blocked-{method.lower()}-artifacts"
                output = self.root / f"blocked-{method.lower()}-evidence.json"
                result = self.run_command(
                    self.command("execute", artifact_root, output),
                    timeout=60,
                    env={environment_name: "1"},
                )
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(
                    f"blocked_forbidden_requests={method}",
                    result.stderr,
                )
                self.assertFalse(output.exists())

    def test_execute_records_exact_blocked_infrastructure_as_guard_observation(
        self,
    ) -> None:
        artifact_root = self.root / "expected-guard-artifacts"
        output = self.root / "expected-guard-evidence.json"
        result = self.run_command(
            self.command("execute", artifact_root, output),
            timeout=300,
            env={"G7_FIXTURE_ATTEMPT_EXPECTED_INFRA": "1"},
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        evidence = json.loads(output.read_text(encoding="utf-8"))
        self.assertEqual(len(evidence["records"]), 66)
        for record in evidence["records"]:
            guard = record["assertions"]["read_only_guard"]
            self.assertEqual(guard["expected_guard_observation_count"], 4)
            self.assertEqual(guard["forbidden_attempt_count"], 0)
            self.assertEqual(guard["mutation_response_count"], 0)
            console_path = next(
                artifact_root / artifact["path"]
                for artifact in record["artifacts"]
                if artifact["kind"] == "console"
            )
            console = json.loads(console_path.read_text(encoding="utf-8"))
            self.assertEqual(console["unexpected_error_count"], 0)
            blocked_errors = [
                entry
                for entry in console["entries"]
                if entry["level"] == "error"
                and "ERR_BLOCKED_BY_CLIENT" in entry["text"]
            ]
            self.assertTrue(blocked_errors)
            self.assertTrue(
                all(
                    entry.get("expected_guard_observation") is True
                    for entry in blocked_errors
                )
            )

    def test_execute_rejects_guard_console_tamper_and_ordinary_error(self) -> None:
        cases = [
            (
                "guard-console-tamper",
                {"G7_FIXTURE_GUARD_CONSOLE_TAMPER": "1"},
                "console_errors=1",
            ),
            (
                "ordinary-console-error",
                {"G7_FIXTURE_ORDINARY_CONSOLE_ERROR": "1"},
                "console_errors=1",
            ),
        ]
        for label, environment, expected_error in cases:
            with self.subTest(label=label):
                artifact_root = self.root / f"{label}-artifacts"
                output = self.root / f"{label}-evidence.json"
                result = self.run_command(
                    self.command("execute", artifact_root, output),
                    timeout=60,
                    env=environment,
                )
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(expected_error, result.stderr)
                self.assertFalse(output.exists())

    def test_execute_rejects_expected_guard_attempt_over_limit(self) -> None:
        artifact_root = self.root / "guard-over-limit-artifacts"
        output = self.root / "guard-over-limit-evidence.json"
        result = self.run_command(
            self.command("execute", artifact_root, output),
            timeout=60,
            env={
                "G7_FIXTURE_ATTEMPT_EXPECTED_INFRA": "1",
                "G7_FIXTURE_ATTEMPT_EXPECTED_INFRA_OVER_LIMIT": "1",
            },
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(
            "blocked_forbidden_requests=POST "
            "http://127.0.0.1:18101/v1/auth/asset-cookie",
            result.stderr,
        )
        self.assertFalse(output.exists())

    def test_execute_rejects_visible_retired_dom_action(self) -> None:
        artifact_root = self.root / "retired-dom-action-artifacts"
        output = self.root / "retired-dom-action-evidence.json"
        result = self.run_command(
            self.command("execute", artifact_root, output),
            timeout=180,
            env={"G7_FIXTURE_RETIRED_DOM_ACTION": "1"},
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("assertions=retired_actions_absent", result.stderr)
        self.assertFalse(output.exists())

    def test_execute_deletes_raw_trace_and_fails_when_sanitizer_fails(self) -> None:
        artifact_root = self.root / "trace-failure-artifacts"
        output = self.root / "trace-failure-evidence.json"
        result = self.run_command(
            self.command("execute", artifact_root, output),
            timeout=60,
            env={"G7_FIXTURE_FAIL_TRACE_SANITIZER": "1"},
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Playwright trace sanitization failed", result.stderr)
        self.assertFalse(output.exists())
        self.assertEqual(list(artifact_root.rglob("trace.zip")), [])

    def test_execute_rejects_runtime_identity_drift(self) -> None:
        cases = [
            (
                "mismatch",
                "G7_FIXTURE_IDENTITY_MISMATCH",
                "/v1/auth/me identity or role differs from attestation",
            ),
            (
                "redirect",
                "G7_FIXTURE_AUTH_ME_REDIRECT",
                "/v1/auth/me did not return HTTP 200",
            ),
            (
                "html",
                "G7_FIXTURE_AUTH_ME_HTML",
                "returned empty, non-JSON, or HTML/login content",
            ),
            (
                "invalid-json",
                "G7_FIXTURE_AUTH_ME_INVALID_JSON",
                "returned invalid JSON",
            ),
        ]
        for label, environment_name, expected_error in cases:
            with self.subTest(label=label):
                artifact_root = self.root / f"identity-{label}-artifacts"
                output = self.root / f"identity-{label}-evidence.json"
                result = self.run_command(
                    self.command("execute", artifact_root, output),
                    timeout=60,
                    env={environment_name: "1"},
                )
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(expected_error, result.stderr)
                self.assertNotIn("super-secret-cookie", result.stderr)
                self.assertFalse(output.exists())

    def test_execute_rejects_redirect_or_html_admin_403_positive_control(self) -> None:
        cases = [
            (
                "html",
                "G7_FIXTURE_ADMIN_403_HTML",
                "returned empty, non-JSON, or HTML/login content",
            ),
            (
                "redirect",
                "G7_FIXTURE_ADMIN_403_REDIRECT",
                "admin positive control did not return 2xx",
            ),
        ]
        for label, environment_name, expected_error in cases:
            with self.subTest(label=label):
                artifact_root = self.root / f"{label}-positive-control-artifacts"
                output = self.root / f"{label}-positive-control-evidence.json"
                result = self.run_command(
                    self.command("execute", artifact_root, output),
                    timeout=180,
                    env={environment_name: "1"},
                )
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(expected_error, result.stderr)
                self.assertFalse(output.exists())

    def test_execute_fails_when_runtime_group_id_is_missing(self) -> None:
        bad_samples = self.root / "missing-runtime-group.json"
        samples = json.loads(self.samples.read_text())
        v8_case = next(
            row
            for row in samples["samples"][0]["coverage_matrix"]
            if row["combination"] == "devplus_devplus"
        )
        v8_case["resource_ids"] = ["task_asset_group:999"]
        samples["samples"][0].pop("sample_sha256")
        samples["samples"][0]["sample_sha256"] = VALIDATOR.canonical_sha256(
            samples["samples"][0]
        )
        samples.pop("manifest_sha256")
        samples["manifest_sha256"] = VALIDATOR.canonical_sha256(samples)
        write_json(bad_samples, samples)
        artifact_root = self.root / "missing-runtime-group-artifacts"
        output = self.root / "missing-runtime-group-evidence.json"
        command = self.command("execute", artifact_root, output)
        command[command.index("--samples") + 1] = str(bad_samples)
        result = self.run_command(command, timeout=60)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("assertions=assets_match", result.stderr)
        self.assertFalse(output.exists())

    def test_dry_run_rejects_unbound_oracle_and_auth_attestation(self) -> None:
        bad_oracle = self.root / "bad-api-oracle.json"
        oracle = json.loads(self.api_oracle.read_text())
        oracle["review_note"] = "tampered after review"
        write_json(bad_oracle, oracle)
        oracle_command = self.command(
            "dry-run",
            self.root / "bad-oracle-artifacts",
            self.root / "bad-oracle-plan.json",
        )
        oracle_command[oracle_command.index("--api-oracle") + 1] = str(bad_oracle)
        oracle_result = self.run_command(oracle_command)
        self.assertNotEqual(oracle_result.returncode, 0)
        self.assertIn("API oracle has an invalid manifest_sha256", oracle_result.stderr)

        three_hash_oracle_path = self.root / "three-hash-api-oracle.json"
        three_hash_samples_path = self.root / "three-hash-samples.json"
        three_hash_oracle = json.loads(self.api_oracle.read_text())
        three_hash_oracle["input_sha256"].pop("edge_receipt_sha256")
        three_hash_oracle["input_sha256"].pop("fixture_receipt_sha256")
        three_hash_oracle.pop("manifest_sha256")
        three_hash_oracle["manifest_sha256"] = VALIDATOR.canonical_sha256(
            three_hash_oracle
        )
        write_json(three_hash_oracle_path, three_hash_oracle)
        three_hash_samples = json.loads(self.samples.read_text())
        three_hash_samples["input_sha256"]["api_oracle_sha256"] = file_sha256(
            three_hash_oracle_path
        )
        three_hash_samples["oracle_contract"][
            "api_oracle_manifest_sha256"
        ] = three_hash_oracle["manifest_sha256"]
        three_hash_samples.pop("manifest_sha256")
        three_hash_samples["manifest_sha256"] = VALIDATOR.canonical_sha256(
            three_hash_samples
        )
        write_json(three_hash_samples_path, three_hash_samples)
        three_hash_command = self.command(
            "dry-run",
            self.root / "three-hash-artifacts",
            self.root / "three-hash-plan.json",
        )
        three_hash_command[three_hash_command.index("--samples") + 1] = str(
            three_hash_samples_path
        )
        three_hash_command[three_hash_command.index("--api-oracle") + 1] = str(
            three_hash_oracle_path
        )
        three_hash_result = self.run_command(three_hash_command)
        self.assertNotEqual(three_hash_result.returncode, 0)
        self.assertIn(
            "API oracle is not bound to the samples upstream inputs",
            three_hash_result.stderr,
        )

        bad_attestation = self.root / "bad-auth-attestation.json"
        attestation = json.loads(self.auth_attestation.read_text())
        attestation["run_id"] = "different-run"
        attestation.pop("attestation_sha256")
        attestation["attestation_sha256"] = VALIDATOR.canonical_sha256(attestation)
        write_json(bad_attestation, attestation)
        auth_command = self.command(
            "dry-run",
            self.root / "bad-auth-artifacts",
            self.root / "bad-auth-plan.json",
        )
        auth_command[auth_command.index("--auth-attestation") + 1] = str(
            bad_attestation
        )
        auth_result = self.run_command(auth_command)
        self.assertNotEqual(auth_result.returncode, 0)
        self.assertIn("auth attestation is not a PASS document", auth_result.stderr)


if __name__ == "__main__":
    unittest.main()
