#!/usr/bin/env python3
from __future__ import annotations

import hashlib
import json
import pathlib
import subprocess
import tempfile
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[2]
COMPOSE = ROOT / "deploy/ab-browser/compose.yaml"
VALIDATOR = ROOT / "scripts/ab/validate_browser_ab_stack.py"
TUNNELS = ROOT / "scripts/ab/browser_quick_tunnels.sh"
NGINX = ROOT / "deploy/ab-browser/nginx/edge.conf.template"
FIXTURE = ROOT / "scripts/ab/ab_upload_fixture.py"


def file_sha256(path: pathlib.Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def tree_sha256(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    for item in sorted(
        (candidate for candidate in path.rglob("*") if candidate.is_file()),
        key=lambda candidate: candidate.relative_to(path).as_posix().encode("utf-8"),
    ):
        relative = item.relative_to(path).as_posix().encode("utf-8")
        digest.update(len(relative).to_bytes(8, "big"))
        digest.update(relative)
        digest.update(item.stat().st_size.to_bytes(8, "big"))
        digest.update(item.read_bytes())
    return digest.hexdigest()


def write_fixture(root: pathlib.Path) -> pathlib.Path:
    external = root / "external-front"
    devplus = root / "devplus-front"
    external.mkdir()
    devplus.mkdir()
    fixture_base = root / "tmp" / "v8-ab" / "static-test"
    fixture_base.mkdir(parents=True)
    fixture_dirs = {}
    for name in (
        "upload-a-root", "upload-b-root", "upload-a-seed", "upload-b-seed",
        "object-a-root", "object-b-root", "object-a-seed", "object-b-seed",
    ):
        path = fixture_base / name
        path.mkdir()
        fixture_dirs[name] = path
    (external / "index.html").write_text('<div id="app"></div>\n', encoding="utf-8")
    (devplus / "index.html").write_text('<div id="app"></div>\n', encoding="utf-8")
    auth_a = root / "auth-a.json"
    auth_b = root / "auth-b.json"
    auth_a.write_text('{"identity":"ab-test"}\n', encoding="utf-8")
    auth_b.write_text(auth_a.read_text(encoding="utf-8"), encoding="utf-8")
    source_snapshot_hash = "3" * 64
    receipts = {}
    markers = {}
    for side, database in (("A", "ab_static_a"), ("B", "ab_static_b")):
        receipt = root / f"{side.lower()}-ui-import-receipt.json"
        receipt.write_text(json.dumps({
            "schema": 1,
            "side": f"{side}_ui",
            "database": database,
            "status": "ready",
            "task_count": 10,
            "source_snapshot_sha256": source_snapshot_hash,
        }, sort_keys=True) + "\n", encoding="utf-8")
        marker = root / f"{side.lower()}-ui-ready-marker.sha256"
        marker.write_text(file_sha256(receipt) + "\n", encoding="utf-8")
        receipts[side] = receipt
        markers[side] = marker
    external_backend_hash = "1" * 64
    devplus_backend_hash = "2" * 64
    env = root / "browser.env"
    values = {
        "AB_COMPOSE_PROJECT": "yb-v8-ab-static-test",
        "AB_MYSQL_IMAGE": f"mysql@sha256:{'4' * 64}",
        "AB_REDIS_IMAGE": f"redis@sha256:{'5' * 64}",
        "AB_NGINX_IMAGE": f"nginx@sha256:{'6' * 64}",
        "AB_PROBE_IMAGE": f"curlimages/curl@sha256:{'7' * 64}",
        "AB_FIXTURE_IMAGE": f"python@sha256:{'8' * 64}",
        "AB_EXTERNAL_BACKEND_IMAGE": f"local/external@sha256:{external_backend_hash}",
        "AB_DEVPLUS_BACKEND_IMAGE": f"local/devplus@sha256:{devplus_backend_hash}",
        "AB_EXTERNAL_BACKEND_SHA256": external_backend_hash,
        "AB_DEVPLUS_BACKEND_SHA256": devplus_backend_hash,
        "AB_MYSQL_ROOT_PASSWORD": "test-root",
        "AB_MYSQL_APP_USER": "ab_app",
        "AB_MYSQL_APP_PASSWORD": "test-app",
        "AB_MYSQL_A_READONLY_USER": "ab_a_reader",
        "AB_MYSQL_A_READONLY_PASSWORD": "test-reader",
        "AB_MYSQL_A_DATABASE": "ab_static_a",
        "AB_MYSQL_B_DATABASE": "ab_static_b",
        "AB_REDIS_PASSWORD": "test-redis",
        "AB_AUTH_SETTINGS_A_FILE": str(auth_a),
        "AB_AUTH_SETTINGS_B_FILE": str(auth_b),
        "AB_AUTH_SETTINGS_SHA256": file_sha256(auth_a),
        "AB_EXTERNAL_FRONT_DIR": str(external),
        "AB_DEVPLUS_FRONT_DIR": str(devplus),
        "AB_EXTERNAL_FRONTEND_SHA256": tree_sha256(external),
        "AB_DEVPLUS_FRONTEND_SHA256": tree_sha256(devplus),
        "AB_A_UI_IMPORT_RECEIPT_FILE": str(receipts["A"]),
        "AB_B_UI_IMPORT_RECEIPT_FILE": str(receipts["B"]),
        "AB_A_UI_IMPORT_RECEIPT_SHA256": file_sha256(receipts["A"]),
        "AB_B_UI_IMPORT_RECEIPT_SHA256": file_sha256(receipts["B"]),
        "AB_A_UI_READY_MARKER_FILE": str(markers["A"]),
        "AB_B_UI_READY_MARKER_FILE": str(markers["B"]),
        "AB_A_UI_EXPECTED_TASK_COUNT": "10",
        "AB_B_UI_EXPECTED_TASK_COUNT": "10",
        "AB_UPLOAD_A_ORIGIN": "http://fixture-upload-a:18091",
        "AB_UPLOAD_B_ORIGIN": "http://fixture-upload-b:18092",
        "AB_UPLOAD_A_PORT": "18091",
        "AB_UPLOAD_B_PORT": "18092",
        "AB_UPLOAD_A_IDENTITY": "test:A_ui:upload",
        "AB_UPLOAD_B_IDENTITY": "test:B_ui:upload",
        "AB_UPLOAD_A_ROOT": str(fixture_dirs["upload-a-root"]),
        "AB_UPLOAD_B_ROOT": str(fixture_dirs["upload-b-root"]),
        "AB_UPLOAD_A_SEED_ROOT": str(fixture_dirs["upload-a-seed"]),
        "AB_UPLOAD_B_SEED_ROOT": str(fixture_dirs["upload-b-seed"]),
        "AB_EXTERNAL_FILE_A_ORIGIN": "http://fixture-object-a:18093",
        "AB_EXTERNAL_FILE_B_ORIGIN": "http://fixture-object-b:18094",
        "AB_OBJECT_A_PORT": "18093",
        "AB_OBJECT_B_PORT": "18094",
        "AB_OBJECT_A_IDENTITY": "test:A_ui:object",
        "AB_OBJECT_B_IDENTITY": "test:B_ui:object",
        "AB_OBJECT_A_ROOT": str(fixture_dirs["object-a-root"]),
        "AB_OBJECT_B_ROOT": str(fixture_dirs["object-b-root"]),
        "AB_OBJECT_A_SEED_ROOT": str(fixture_dirs["object-a-seed"]),
        "AB_OBJECT_B_SEED_ROOT": str(fixture_dirs["object-b-seed"]),
        "AB_FIXTURE_SCRIPT_SHA256": file_sha256(FIXTURE),
        "AB_EDGE_EXTERNAL_EXTERNAL_PORT": "18101",
        "AB_EDGE_DEVPLUS_DEVPLUS_PORT": "18102",
        "AB_EDGE_EXTERNAL_DEVPLUS_PORT": "18103",
        "AB_EDGE_DEVPLUS_EXTERNAL_PORT": "18104",
    }
    env.write_text("".join(f"{key}={value}\n" for key, value in values.items()), encoding="utf-8")
    return env


def replace_env(env: pathlib.Path, key: str, value: str) -> None:
    lines = env.read_text(encoding="utf-8").splitlines()
    env.write_text("\n".join(f"{key}={value}" if line.startswith(f"{key}=") else line for line in lines) + "\n", encoding="utf-8")


class BrowserABStackTest(unittest.TestCase):
    def test_frontend_tree_hash_uses_posix_relative_byte_order(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tree = pathlib.Path(tmp)
            (tree / "z.txt").write_bytes(b"z")
            (tree / "A.txt").write_bytes(b"A")
            expected = hashlib.sha256()
            for relative in ("A.txt", "z.txt"):
                payload = relative.encode("utf-8")
                body = (tree / relative).read_bytes()
                expected.update(len(payload).to_bytes(8, "big"))
                expected.update(payload)
                expected.update(len(body).to_bytes(8, "big"))
                expected.update(body)
            self.assertEqual(expected.hexdigest(), tree_sha256(tree))

    def test_valid_stack_and_plan_only_tunnel(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            env = write_fixture(root)
            result = subprocess.run(
                ["python3", str(VALIDATOR), "--compose-file", str(COMPOSE), "--env-file", str(env)],
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            self.assertIn('"status": "PASS"', result.stdout)
            self.assertIn('"network_isolation"', result.stdout)
            self.assertIn('"edge_attestations"', result.stdout)

            plan = subprocess.run(
                ["bash", str(TUNNELS), "start", "--env-file", str(env), "--run-dir", str(root / "run")],
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertEqual(plan.returncode, 0, plan.stderr + plan.stdout)
            self.assertIn("PLAN only", plan.stdout)
            self.assertFalse((root / "run").exists())

    def test_validator_rejects_non_loopback_edge(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            env = write_fixture(root)
            unsafe = root / "compose.yaml"
            unsafe.write_text(
                COMPOSE.read_text(encoding="utf-8").replace(
                    "127.0.0.1:${AB_EDGE_EXTERNAL_EXTERNAL_PORT", "0.0.0.0:${AB_EDGE_EXTERNAL_EXTERNAL_PORT", 1
                ),
                encoding="utf-8",
            )
            result = subprocess.run(
                ["python3", str(VALIDATOR), "--compose-file", str(unsafe), "--env-file", str(env)],
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("must bind only 127.0.0.1", result.stdout)

    def test_validator_rejects_cross_side_network_reachability(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            env = write_fixture(root)
            unsafe = root / "compose.yaml"
            # Add B private reachability to backend-a without changing its DSN.
            content = COMPOSE.read_text(encoding="utf-8")
            marker = "  backend-a:\n"
            start = content.index(marker)
            network_marker = "    networks:\n      - ab_a_private\n      - ab_a_ingress\n"
            position = content.index(network_marker, start)
            content = content[:position] + network_marker.replace("      - ab_a_ingress\n", "      - ab_a_ingress\n      - ab_b_private\n") + content[position + len(network_marker):]
            unsafe.write_text(content, encoding="utf-8")
            result = subprocess.run(
                ["python3", str(VALIDATOR), "--compose-file", str(unsafe), "--env-file", str(env)],
                text=True, capture_output=True, check=False,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("network ownership mismatch", result.stdout)

    def test_validator_rejects_edge_origin_cross_wiring(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            env = write_fixture(root)
            unsafe = root / "compose.yaml"
            content = COMPOSE.read_text(encoding="utf-8")
            edge_start = content.index("  edge-external-external:\n")
            origin_at = content.index("      UPLOAD_ORIGIN: ${AB_UPLOAD_A_ORIGIN}", edge_start)
            content = content[:origin_at] + content[origin_at:].replace(
                "      UPLOAD_ORIGIN: ${AB_UPLOAD_A_ORIGIN}",
                "      UPLOAD_ORIGIN: ${AB_UPLOAD_B_ORIGIN}",
                1,
            )
            unsafe.write_text(content, encoding="utf-8")
            result = subprocess.run(
                ["python3", str(VALIDATOR), "--compose-file", str(unsafe), "--env-file", str(env)],
                text=True, capture_output=True, check=False,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("UPLOAD_ORIGIN ownership mismatch", result.stdout)

    def test_validator_rejects_auth_parity_and_receipt_marker_drift(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            env = write_fixture(root)
            auth_b = root / "auth-b.json"
            auth_b.write_text('{"identity":"wrong"}\n', encoding="utf-8")
            auth_result = subprocess.run(
                ["python3", str(VALIDATOR), "--compose-file", str(COMPOSE), "--env-file", str(env)],
                text=True, capture_output=True, check=False,
            )
            self.assertNotEqual(auth_result.returncode, 0)
            self.assertIn("auth settings content", auth_result.stdout)

            auth_a = root / "auth-a.json"
            auth_b.write_text(auth_a.read_text(encoding="utf-8"), encoding="utf-8")
            (root / "b-ui-ready-marker.sha256").write_text("0" * 64 + "\n", encoding="utf-8")
            receipt_result = subprocess.run(
                ["python3", str(VALIDATOR), "--compose-file", str(COMPOSE), "--env-file", str(env)],
                text=True, capture_output=True, check=False,
            )
            self.assertNotEqual(receipt_result.returncode, 0)
            self.assertIn("ready marker", receipt_result.stdout)

    def test_validator_rejects_writable_a_fixture_and_reused_roots(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            env = write_fixture(root)
            unsafe = root / "compose.yaml"
            content = COMPOSE.read_text(encoding="utf-8")
            service_start = content.index("  fixture-upload-a:\n")
            read_only_at = content.index("      - --read-only\n", service_start)
            unsafe.write_text(content[:read_only_at] + content[read_only_at + len("      - --read-only\n"):], encoding="utf-8")
            writable = subprocess.run(
                ["python3", str(VALIDATOR), "--compose-file", str(unsafe), "--env-file", str(env)],
                text=True, capture_output=True, check=False,
            )
            self.assertNotEqual(writable.returncode, 0)
            self.assertIn("read-only command contract mismatch", writable.stdout)

            replace_env(env, "AB_UPLOAD_B_ROOT", str(root / "tmp" / "v8-ab" / "static-test" / "upload-a-root"))
            reused = subprocess.run(
                ["python3", str(VALIDATOR), "--compose-file", str(COMPOSE), "--env-file", str(env)],
                text=True, capture_output=True, check=False,
            )
            self.assertNotEqual(reused.returncode, 0)
            self.assertIn("root and seed root must be distinct", reused.stdout)

    def test_nginx_has_complete_same_origin_routes(self) -> None:
        content = NGINX.read_text(encoding="utf-8")
        for expected in (
            "location /v1/",
            "location ^~ /ws/",
            "location ^~ /upload/",
            "location ^~ /_protected/external-alist/p/",
            "try_files $uri $uri/ /index.html",
        ):
            self.assertIn(expected, content)
        self.assertIn("internal;", content)
        self.assertIn("location = /__ab/identity", content)
        self.assertIn("proxy_ssl_server_name on;", content)
        self.assertIn("proxy_ssl_name $proxy_host;", content)
        ws_block = content.split("location ^~ /ws/", 1)[1].split(
            "location ^~ /upload/", 1
        )[0]
        self.assertIn("proxy_set_header Host $http_host;", ws_block)

    def test_compose_uses_internal_database_hosts_and_a_read_only_gate(self) -> None:
        content = COMPOSE.read_text(encoding="utf-8")
        self.assertNotIn("host.docker.internal:3308", content)
        self.assertIn("@tcp(mysql-a:3306)/${AB_MYSQL_A_DATABASE}", content)
        self.assertIn("@tcp(mysql-b:3306)/${AB_MYSQL_B_DATABASE}", content)
        self.assertIn("AB_MYSQL_A_READONLY_USER", content)
        self.assertIn("SHOW GRANTS FOR CURRENT_USER", content)
        self.assertIn('REQUIRE_READ_ONLY: "true"', content)
        for service in ("fixture-upload-a", "fixture-upload-b", "fixture-object-a", "fixture-object-b"):
            self.assertIn(f"  {service}:\n", content)
        self.assertNotIn("AB_UPLOAD_A_ORIGIN=http://host.docker.internal", (ROOT / "deploy/ab-browser/ab-browser.env.example").read_text(encoding="utf-8"))

    def test_tunnel_script_attests_public_identity_and_process_evidence(self) -> None:
        content = TUNNELS.read_text(encoding="utf-8")
        for expected in (
            "Quick Tunnel public URLs must be unique",
            "probe_edge \"$label\"",
            "public_health_status",
            "cmdline_sha256",
            "verify_evidence_hashes",
            "x-ab-backend-fingerprint",
            "browser-stack-preflight.json",
        ):
            self.assertIn(expected, content)


if __name__ == "__main__":
    unittest.main()
