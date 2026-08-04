#!/usr/bin/env python3
"""Fail-closed static validation for the Browser A/B Compose stack."""

from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import re
import subprocess
import sys
import urllib.parse


EDGE_MATRIX = {
    "edge-external-external": {
        "side": "a", "backend": "backend-a", "front_env": "AB_EXTERNAL_FRONT_DIR",
        "front_hash_env": "AB_EXTERNAL_FRONTEND_SHA256", "backend_hash_env": "AB_EXTERNAL_BACKEND_SHA256",
        "upload_env": "AB_UPLOAD_A_ORIGIN", "object_env": "AB_EXTERNAL_FILE_A_ORIGIN",
        "fixture_identity": ("AB_UPLOAD_A_IDENTITY", "AB_OBJECT_A_IDENTITY"),
    },
    "edge-devplus-devplus": {
        "side": "b", "backend": "backend-b", "front_env": "AB_DEVPLUS_FRONT_DIR",
        "front_hash_env": "AB_DEVPLUS_FRONTEND_SHA256", "backend_hash_env": "AB_DEVPLUS_BACKEND_SHA256",
        "upload_env": "AB_UPLOAD_B_ORIGIN", "object_env": "AB_EXTERNAL_FILE_B_ORIGIN",
        "fixture_identity": ("AB_UPLOAD_B_IDENTITY", "AB_OBJECT_B_IDENTITY"),
    },
    "edge-external-devplus": {
        "side": "b", "backend": "backend-b", "front_env": "AB_EXTERNAL_FRONT_DIR",
        "front_hash_env": "AB_EXTERNAL_FRONTEND_SHA256", "backend_hash_env": "AB_DEVPLUS_BACKEND_SHA256",
        "upload_env": "AB_UPLOAD_B_ORIGIN", "object_env": "AB_EXTERNAL_FILE_B_ORIGIN",
        "fixture_identity": ("AB_UPLOAD_B_IDENTITY", "AB_OBJECT_B_IDENTITY"),
    },
    "edge-devplus-external": {
        "side": "a", "backend": "backend-a", "front_env": "AB_DEVPLUS_FRONT_DIR",
        "front_hash_env": "AB_DEVPLUS_FRONTEND_SHA256", "backend_hash_env": "AB_EXTERNAL_BACKEND_SHA256",
        "upload_env": "AB_UPLOAD_A_ORIGIN", "object_env": "AB_EXTERNAL_FILE_A_ORIGIN",
        "fixture_identity": ("AB_UPLOAD_A_IDENTITY", "AB_OBJECT_A_IDENTITY"),
    },
}
EDGE_SERVICES = set(EDGE_MATRIX)
FIXTURE_SERVICES = {
    "fixture-upload-a": {"side": "a", "kind": "upload", "read_only": True},
    "fixture-object-a": {"side": "a", "kind": "object", "read_only": True},
    "fixture-upload-b": {"side": "b", "kind": "upload", "read_only": False},
    "fixture-object-b": {"side": "b", "kind": "object", "read_only": True},
}
PRIVATE_SERVICES = {
    "mysql-a", "mysql-b", "redis-a", "redis-b", "clone-ready-a", "clone-ready-b",
    "fixture-ready-a", "fixture-ready-b", "backend-a", "backend-b", *FIXTURE_SERVICES,
}
SERVICE_NETWORKS = {
    "mysql-a": {"ab_a_private"}, "redis-a": {"ab_a_private"}, "clone-ready-a": {"ab_a_private"},
    "backend-a": {"ab_a_private", "ab_a_ingress"}, "fixture-ready-a": {"ab_a_ingress"},
    "fixture-upload-a": {"ab_a_ingress"}, "fixture-object-a": {"ab_a_ingress"},
    "edge-external-external": {"ab_a_ingress"}, "edge-devplus-external": {"ab_a_ingress"},
    "mysql-b": {"ab_b_private"}, "redis-b": {"ab_b_private"}, "clone-ready-b": {"ab_b_private"},
    "backend-b": {"ab_b_private", "ab_b_ingress"}, "fixture-ready-b": {"ab_b_ingress"},
    "fixture-upload-b": {"ab_b_ingress"}, "fixture-object-b": {"ab_b_ingress"},
    "edge-devplus-devplus": {"ab_b_ingress"}, "edge-external-devplus": {"ab_b_ingress"},
}
REQUIRED_ENV = {
    "AB_COMPOSE_PROJECT",
    "AB_MYSQL_IMAGE",
    "AB_REDIS_IMAGE",
    "AB_NGINX_IMAGE",
    "AB_PROBE_IMAGE",
    "AB_FIXTURE_IMAGE",
    "AB_EXTERNAL_BACKEND_IMAGE",
    "AB_DEVPLUS_BACKEND_IMAGE",
    "AB_EXTERNAL_BACKEND_SHA256",
    "AB_DEVPLUS_BACKEND_SHA256",
    "AB_MYSQL_ROOT_PASSWORD",
    "AB_MYSQL_APP_USER",
    "AB_MYSQL_APP_PASSWORD",
    "AB_MYSQL_A_READONLY_USER",
    "AB_MYSQL_A_READONLY_PASSWORD",
    "AB_MYSQL_A_DATABASE",
    "AB_MYSQL_B_DATABASE",
    "AB_REDIS_PASSWORD",
    "AB_AUTH_SETTINGS_A_FILE",
    "AB_AUTH_SETTINGS_B_FILE",
    "AB_AUTH_SETTINGS_SHA256",
    "AB_EXTERNAL_FRONT_DIR",
    "AB_DEVPLUS_FRONT_DIR",
    "AB_EXTERNAL_FRONTEND_SHA256",
    "AB_DEVPLUS_FRONTEND_SHA256",
    "AB_A_UI_IMPORT_RECEIPT_FILE",
    "AB_B_UI_IMPORT_RECEIPT_FILE",
    "AB_A_UI_IMPORT_RECEIPT_SHA256",
    "AB_B_UI_IMPORT_RECEIPT_SHA256",
    "AB_A_UI_READY_MARKER_FILE",
    "AB_B_UI_READY_MARKER_FILE",
    "AB_A_UI_EXPECTED_TASK_COUNT",
    "AB_B_UI_EXPECTED_TASK_COUNT",
    "AB_UPLOAD_A_ORIGIN",
    "AB_UPLOAD_B_ORIGIN",
    "AB_UPLOAD_A_IDENTITY",
    "AB_UPLOAD_B_IDENTITY",
    "AB_UPLOAD_A_PORT",
    "AB_UPLOAD_B_PORT",
    "AB_UPLOAD_A_ROOT",
    "AB_UPLOAD_B_ROOT",
    "AB_UPLOAD_A_SEED_ROOT",
    "AB_UPLOAD_B_SEED_ROOT",
    "AB_EXTERNAL_FILE_A_ORIGIN",
    "AB_EXTERNAL_FILE_B_ORIGIN",
    "AB_OBJECT_A_IDENTITY",
    "AB_OBJECT_B_IDENTITY",
    "AB_OBJECT_A_PORT",
    "AB_OBJECT_B_PORT",
    "AB_OBJECT_A_ROOT",
    "AB_OBJECT_B_ROOT",
    "AB_OBJECT_A_SEED_ROOT",
    "AB_OBJECT_B_SEED_ROOT",
    "AB_FIXTURE_SCRIPT_SHA256",
    "AB_EDGE_EXTERNAL_EXTERNAL_PORT",
    "AB_EDGE_DEVPLUS_DEVPLUS_PORT",
    "AB_EDGE_EXTERNAL_DEVPLUS_PORT",
    "AB_EDGE_DEVPLUS_EXTERNAL_PORT",
}


def load_env(path: pathlib.Path) -> dict[str, str]:
    values: dict[str, str] = {}
    for line_no, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("export "):
            line = line[7:].strip()
        if "=" not in line:
            raise ValueError(f"invalid env line {line_no}: expected KEY=VALUE")
        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip()
        if not re.fullmatch(r"[A-Z][A-Z0-9_]*", key):
            raise ValueError(f"invalid env key on line {line_no}: {key!r}")
        if len(value) >= 2 and value[0] == value[-1] and value[0] in {'"', "'"}:
            value = value[1:-1]
        values[key] = value
    return values


def require_file(path_value: str, label: str) -> pathlib.Path:
    path = pathlib.Path(path_value)
    if not path.is_absolute():
        raise ValueError(f"{label} must be an absolute path: {path}")
    if not path.is_file():
        raise ValueError(f"{label} does not exist or is not a file: {path}")
    return path


def require_front(path_value: str, label: str) -> pathlib.Path:
    path = pathlib.Path(path_value)
    if not path.is_absolute():
        raise ValueError(f"{label} must be an absolute path: {path}")
    if not path.is_dir() or not (path / "index.html").is_file():
        raise ValueError(f"{label} must contain index.html: {path}")
    return path


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def sha256_tree(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    # Native pathlib ordering follows host path semantics (notably Windows
    # case folding). Sort exact POSIX relative-path bytes so identical frozen
    # frontend trees have one fingerprint in Windows and WSL.
    files = sorted(
        (item for item in path.rglob("*") if item.is_file()),
        key=lambda item: item.relative_to(path).as_posix().encode("utf-8"),
    )
    if not files:
        raise ValueError(f"frontend artifact directory is empty: {path}")
    for item in files:
        if item.is_symlink():
            raise ValueError(f"frontend artifact must not contain symlinked files: {item}")
        relative = item.relative_to(path).as_posix().encode("utf-8")
        digest.update(len(relative).to_bytes(8, "big"))
        digest.update(relative)
        digest.update(item.stat().st_size.to_bytes(8, "big"))
        with item.open("rb") as handle:
            for chunk in iter(lambda: handle.read(1024 * 1024), b""):
                digest.update(chunk)
    return digest.hexdigest()


def require_sha256(value: str, label: str) -> str:
    normalized = value.lower()
    if not re.fullmatch(r"[0-9a-f]{64}", normalized):
        raise ValueError(f"{label} must be a 64-character lowercase SHA-256")
    return normalized


def image_sha256(image: str, label: str) -> str:
    match = re.search(r"@sha256:([0-9a-fA-F]{64})$", image)
    if not match:
        raise ValueError(f"{label} must be pinned by @sha256 digest")
    return match.group(1).lower()


def validate_fixture_origin(value: str, label: str, expected_host: str, expected_port: int) -> None:
    parsed = urllib.parse.urlparse(value)
    if parsed.scheme not in {"http", "https"} or parsed.path != "" or parsed.query or parsed.fragment:
        raise ValueError(f"{label} must be a plain fixture http(s) origin without path/query/fragment or trailing slash")
    if parsed.hostname != expected_host or parsed.port != expected_port:
        raise ValueError(f"{label} must point to Compose fixture {expected_host}:{expected_port}")


def require_directory(path_value: str, label: str) -> pathlib.Path:
    path = pathlib.Path(path_value)
    if not path.is_absolute():
        raise ValueError(f"{label} must be an absolute path: {path}")
    if not path.is_dir():
        raise ValueError(f"{label} does not exist or is not a directory: {path}")
    resolved = path.resolve()
    if str(resolved).startswith("/root/ecommerce_ai/"):
        raise ValueError(f"{label} must not use the production release/config tree")
    normalized_parts = tuple(part.lower() for part in resolved.parts)
    if not any(normalized_parts[index:index + 2] == ("tmp", "v8-ab") for index in range(len(normalized_parts) - 1)):
        raise ValueError(f"{label} must be inside a run-scoped tmp/v8-ab directory")
    return resolved


def require_identity(value: str, label: str) -> str:
    if not re.fullmatch(r"[A-Za-z0-9._:-]{3,128}", value):
        raise ValueError(f"{label} must be a stable 3-128 character fixture identity")
    return value


def load_import_receipt(path: pathlib.Path, side: str, database: str, task_count: int) -> dict[str, object]:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        raise ValueError(f"invalid {side} import receipt JSON: {exc}") from exc
    if not isinstance(payload, dict):
        raise ValueError(f"{side} import receipt must be a JSON object")
    expected = {"schema": 1, "side": side, "database": database, "status": "ready", "task_count": task_count}
    for key, value in expected.items():
        if payload.get(key) != value:
            raise ValueError(f"{side} import receipt {key} mismatch: expected {value!r}")
    require_sha256(str(payload.get("source_snapshot_sha256") or ""), f"{side} source_snapshot_sha256")
    return payload


def service_networks(service: dict[str, object]) -> set[str]:
    raw = service.get("networks") or {}
    if isinstance(raw, dict):
        return set(raw)
    if isinstance(raw, list):
        return set(raw)
    raise ValueError("service networks must be a list or object")


def bind_mount(service: dict[str, object], target: str) -> dict[str, object] | None:
    for mount in service.get("volumes") or []:
        if isinstance(mount, dict) and mount.get("target") == target:
            return mount
    return None


def compose_json(compose_file: pathlib.Path, env_file: pathlib.Path) -> dict[str, object]:
    command = [
        "docker",
        "compose",
        "--env-file",
        str(env_file),
        "-f",
        str(compose_file),
        "config",
        "--format",
        "json",
    ]
    completed = subprocess.run(command, text=True, capture_output=True, check=False)
    if completed.returncode:
        raise ValueError(f"docker compose config failed: {completed.stderr.strip()}")
    return json.loads(completed.stdout)


def validate(compose_file: pathlib.Path, env_file: pathlib.Path) -> dict[str, object]:
    env = load_env(env_file)
    missing = sorted(key for key in REQUIRED_ENV if not env.get(key))
    if missing:
        raise ValueError(f"missing required env values: {', '.join(missing)}")
    for key in ("AB_MYSQL_A_DATABASE", "AB_MYSQL_B_DATABASE"):
        if not re.fullmatch(r"ab_[A-Za-z0-9_]+", env[key]):
            raise ValueError(f"{key} must start with ab_ and contain only letters, digits, underscore")
    if env["AB_MYSQL_A_DATABASE"] == env["AB_MYSQL_B_DATABASE"]:
        raise ValueError("A and B database names must differ")
    if not re.fullmatch(r"[A-Za-z0-9_]{1,32}", env["AB_MYSQL_A_READONLY_USER"]):
        raise ValueError("AB_MYSQL_A_READONLY_USER must be a simple MySQL account name")
    if env["AB_MYSQL_A_READONLY_USER"] == env["AB_MYSQL_APP_USER"]:
        raise ValueError("A read-only backend account must differ from the B/import app account")

    expected_auth_hash = require_sha256(env["AB_AUTH_SETTINGS_SHA256"], "AB_AUTH_SETTINGS_SHA256")
    auth_paths = {
        "a": require_file(env["AB_AUTH_SETTINGS_A_FILE"], "AB_AUTH_SETTINGS_A_FILE"),
        "b": require_file(env["AB_AUTH_SETTINGS_B_FILE"], "AB_AUTH_SETTINGS_B_FILE"),
    }
    auth_hashes = {side: sha256_file(path) for side, path in auth_paths.items()}
    if set(auth_hashes.values()) != {expected_auth_hash}:
        raise ValueError("A/B auth settings content must be byte-identical and match AB_AUTH_SETTINGS_SHA256")

    front_paths = {
        "external": require_front(env["AB_EXTERNAL_FRONT_DIR"], "AB_EXTERNAL_FRONT_DIR"),
        "devplus": require_front(env["AB_DEVPLUS_FRONT_DIR"], "AB_DEVPLUS_FRONT_DIR"),
    }
    frontend_hashes = {name: sha256_tree(path) for name, path in front_paths.items()}
    expected_frontend_hashes = {
        "external": require_sha256(env["AB_EXTERNAL_FRONTEND_SHA256"], "AB_EXTERNAL_FRONTEND_SHA256"),
        "devplus": require_sha256(env["AB_DEVPLUS_FRONTEND_SHA256"], "AB_DEVPLUS_FRONTEND_SHA256"),
    }
    if frontend_hashes != expected_frontend_hashes:
        raise ValueError(f"frontend artifact fingerprint mismatch: expected={expected_frontend_hashes}, actual={frontend_hashes}")

    backend_hashes = {
        "external": require_sha256(env["AB_EXTERNAL_BACKEND_SHA256"], "AB_EXTERNAL_BACKEND_SHA256"),
        "devplus": require_sha256(env["AB_DEVPLUS_BACKEND_SHA256"], "AB_DEVPLUS_BACKEND_SHA256"),
    }
    image_hashes = {
        "external": image_sha256(env["AB_EXTERNAL_BACKEND_IMAGE"], "AB_EXTERNAL_BACKEND_IMAGE"),
        "devplus": image_sha256(env["AB_DEVPLUS_BACKEND_IMAGE"], "AB_DEVPLUS_BACKEND_IMAGE"),
    }
    if backend_hashes != image_hashes:
        raise ValueError(f"backend image digest/fingerprint mismatch: images={image_hashes}, fingerprints={backend_hashes}")
    infrastructure_image_hashes = {
        key: image_sha256(env[key], key)
        for key in ("AB_MYSQL_IMAGE", "AB_REDIS_IMAGE", "AB_NGINX_IMAGE", "AB_PROBE_IMAGE", "AB_FIXTURE_IMAGE")
    }

    fixture_ports: dict[str, int] = {}
    for key in ("AB_UPLOAD_A_PORT", "AB_UPLOAD_B_PORT", "AB_OBJECT_A_PORT", "AB_OBJECT_B_PORT"):
        if not re.fullmatch(r"[0-9]{4,5}", env[key]) or not 1024 <= int(env[key]) <= 65535:
            raise ValueError(f"{key} must be an unprivileged TCP port")
        fixture_ports[key] = int(env[key])
    if len(set(fixture_ports.values())) != 4:
        raise ValueError("the four fixture ports must be unique")
    fixture_origins = {
        "AB_UPLOAD_A_ORIGIN": ("fixture-upload-a", fixture_ports["AB_UPLOAD_A_PORT"]),
        "AB_UPLOAD_B_ORIGIN": ("fixture-upload-b", fixture_ports["AB_UPLOAD_B_PORT"]),
        "AB_EXTERNAL_FILE_A_ORIGIN": ("fixture-object-a", fixture_ports["AB_OBJECT_A_PORT"]),
        "AB_EXTERNAL_FILE_B_ORIGIN": ("fixture-object-b", fixture_ports["AB_OBJECT_B_PORT"]),
    }
    for key, (host, port) in fixture_origins.items():
        validate_fixture_origin(env[key], key, host, port)
    if env["AB_UPLOAD_A_ORIGIN"] == env["AB_UPLOAD_B_ORIGIN"]:
        raise ValueError("A/B upload fixture origins must differ")
    if env["AB_EXTERNAL_FILE_A_ORIGIN"] == env["AB_EXTERNAL_FILE_B_ORIGIN"]:
        raise ValueError("A/B object fixture origins must differ")
    for key in ("AB_UPLOAD_A_IDENTITY", "AB_UPLOAD_B_IDENTITY", "AB_OBJECT_A_IDENTITY", "AB_OBJECT_B_IDENTITY"):
        require_identity(env[key], key)
    if env["AB_UPLOAD_A_IDENTITY"] == env["AB_UPLOAD_B_IDENTITY"]:
        raise ValueError("A/B upload fixture identities must differ")
    if env["AB_OBJECT_A_IDENTITY"] == env["AB_OBJECT_B_IDENTITY"]:
        raise ValueError("A/B object fixture identities must differ")
    fixture_path_keys = (
        "AB_UPLOAD_A_ROOT", "AB_UPLOAD_B_ROOT", "AB_UPLOAD_A_SEED_ROOT", "AB_UPLOAD_B_SEED_ROOT",
        "AB_OBJECT_A_ROOT", "AB_OBJECT_B_ROOT", "AB_OBJECT_A_SEED_ROOT", "AB_OBJECT_B_SEED_ROOT",
    )
    fixture_paths = {key: require_directory(env[key], key) for key in fixture_path_keys}
    if len(set(fixture_paths.values())) != len(fixture_paths):
        raise ValueError("every upload/object fixture root and seed root must be distinct")
    fixture_script = pathlib.Path(__file__).resolve().with_name("ab_upload_fixture.py")
    expected_fixture_script_hash = require_sha256(env["AB_FIXTURE_SCRIPT_SHA256"], "AB_FIXTURE_SCRIPT_SHA256")
    if not fixture_script.is_file() or sha256_file(fixture_script) != expected_fixture_script_hash:
        raise ValueError("fixture script is missing or does not match AB_FIXTURE_SCRIPT_SHA256")

    task_counts: dict[str, int] = {}
    for side in ("A", "B"):
        key = f"AB_{side}_UI_EXPECTED_TASK_COUNT"
        if not re.fullmatch(r"[1-9][0-9]*", env[key]):
            raise ValueError(f"{key} must be a positive integer")
        task_counts[side.lower()] = int(env[key])
    receipt_paths = {
        "a": require_file(env["AB_A_UI_IMPORT_RECEIPT_FILE"], "AB_A_UI_IMPORT_RECEIPT_FILE"),
        "b": require_file(env["AB_B_UI_IMPORT_RECEIPT_FILE"], "AB_B_UI_IMPORT_RECEIPT_FILE"),
    }
    marker_paths = {
        "a": require_file(env["AB_A_UI_READY_MARKER_FILE"], "AB_A_UI_READY_MARKER_FILE"),
        "b": require_file(env["AB_B_UI_READY_MARKER_FILE"], "AB_B_UI_READY_MARKER_FILE"),
    }
    receipt_hashes: dict[str, str] = {}
    receipts: dict[str, dict[str, object]] = {}
    for side, receipt_path in receipt_paths.items():
        upper = side.upper()
        expected_hash = require_sha256(env[f"AB_{upper}_UI_IMPORT_RECEIPT_SHA256"], f"AB_{upper}_UI_IMPORT_RECEIPT_SHA256")
        receipt_hashes[side] = sha256_file(receipt_path)
        if receipt_hashes[side] != expected_hash:
            raise ValueError(f"{upper}_ui import receipt hash mismatch")
        marker = marker_paths[side].read_text(encoding="utf-8").strip().lower()
        if marker != expected_hash:
            raise ValueError(f"{upper}_ui ready marker must contain the import receipt SHA-256")
        receipts[side] = load_import_receipt(
            receipt_path,
            f"{upper}_ui",
            env[f"AB_MYSQL_{upper}_DATABASE"],
            task_counts[side],
        )
    if receipts["a"]["source_snapshot_sha256"] != receipts["b"]["source_snapshot_sha256"]:
        raise ValueError("A_ui and B_ui import receipts must attest the same source snapshot")

    ports = [env[key] for key in REQUIRED_ENV if key.startswith("AB_EDGE_") and key.endswith("_PORT")]
    if any(not re.fullmatch(r"[0-9]{2,5}", port) or not 1024 <= int(port) <= 65535 for port in ports):
        raise ValueError("edge ports must be unique unprivileged TCP ports")
    if len(set(ports)) != 4:
        raise ValueError("the four edge ports must be unique")

    config = compose_json(compose_file, env_file)
    services = config.get("services")
    if not isinstance(services, dict):
        raise ValueError("compose output has no services map")
    expected = EDGE_SERVICES | PRIVATE_SERVICES
    if set(services) != expected:
        raise ValueError(f"compose services differ: expected={sorted(expected)}, actual={sorted(services)}")

    networks = config.get("networks")
    if not isinstance(networks, dict) or set(networks) != {"ab_a_private", "ab_b_private", "ab_a_ingress", "ab_b_ingress"}:
        raise ValueError("compose must define only independent A/B private and ingress networks")
    for private in ("ab_a_private", "ab_b_private"):
        if not isinstance(networks[private], dict) or networks[private].get("internal") is not True:
            raise ValueError(f"network {private} must be internal")

    for name, raw_service in services.items():
        if not isinstance(raw_service, dict):
            raise ValueError(f"service {name} is not an object")
        if raw_service.get("network_mode") == "host":
            raise ValueError(f"service {name} must not use host networking")
        actual_networks = service_networks(raw_service)
        if actual_networks != SERVICE_NETWORKS[name]:
            raise ValueError(f"service {name} network ownership mismatch: expected={sorted(SERVICE_NETWORKS[name])}, actual={sorted(actual_networks)}")
        published = raw_service.get("ports") or []
        if name in PRIVATE_SERVICES and published:
            raise ValueError(f"private service {name} must not publish ports")
        if name in EDGE_SERVICES:
            if len(published) != 1:
                raise ValueError(f"edge service {name} must publish exactly one port")
            port = published[0]
            if not isinstance(port, dict) or port.get("host_ip") != "127.0.0.1":
                raise ValueError(f"edge service {name} must bind only 127.0.0.1")
            if int(port.get("target", 0)) != 8080 or port.get("protocol") != "tcp":
                raise ValueError(f"edge service {name} must publish TCP target 8080")

    fixture_attestations: dict[str, dict[str, object]] = {}
    for name, contract in FIXTURE_SERVICES.items():
        service = services[name]
        side = str(contract["side"]).upper()
        kind = str(contract["kind"])
        if kind == "object":
            identity_key = f"AB_OBJECT_{side}_IDENTITY"
            port_key = f"AB_OBJECT_{side}_PORT"
            root_key = f"AB_OBJECT_{side}_ROOT"
            seed_key = f"AB_OBJECT_{side}_SEED_ROOT"
        else:
            identity_key = f"AB_UPLOAD_{side}_IDENTITY"
            port_key = f"AB_UPLOAD_{side}_PORT"
            root_key = f"AB_UPLOAD_{side}_ROOT"
            seed_key = f"AB_UPLOAD_{side}_SEED_ROOT"
        if service.get("image") != env["AB_FIXTURE_IMAGE"]:
            raise ValueError(f"{name} must use the attested AB_FIXTURE_IMAGE")
        command = [str(item) for item in service.get("command") or []]
        required_command = {
            f"--mode={kind}",
            f"--identity={env[identity_key]}",
            f"--port={env[port_key]}",
            "--root=/run/ab/root",
            "--seed-root=/run/ab/seed",
        }
        if not required_command.issubset(set(command)):
            raise ValueError(f"{name} command does not match its mode/identity/root/port contract")
        has_read_only = "--read-only" in command
        if has_read_only is not bool(contract["read_only"]):
            raise ValueError(f"{name} read-only command contract mismatch")
        root_mount = bind_mount(service, "/run/ab/root")
        seed_mount = bind_mount(service, "/run/ab/seed")
        if not root_mount or pathlib.Path(str(root_mount.get("source"))).resolve() != fixture_paths[root_key]:
            raise ValueError(f"{name} root mount does not match {root_key}")
        expected_root_read_only = bool(contract["read_only"])
        if bool(root_mount.get("read_only")) is not expected_root_read_only:
            raise ValueError(f"{name} root write policy mismatch")
        if not seed_mount or pathlib.Path(str(seed_mount.get("source"))).resolve() != fixture_paths[seed_key] or seed_mount.get("read_only") is not True:
            raise ValueError(f"{name} seed mount must be the declared read-only seed root")
        fixture_attestations[name] = {
            "identity": env[identity_key],
            "port": int(env[port_key]),
            "read_only": expected_root_read_only,
            "root": str(fixture_paths[root_key]),
            "seed_root": str(fixture_paths[seed_key]),
        }

    env_a = services["backend-a"].get("environment", {})
    env_b = services["backend-b"].get("environment", {})
    expected_a_dsn_prefix = f"{env['AB_MYSQL_A_READONLY_USER']}:{env['AB_MYSQL_A_READONLY_PASSWORD']}@tcp(mysql-a:3306)/{env['AB_MYSQL_A_DATABASE']}?"
    if not str(env_a.get("MYSQL_DSN", "")).startswith(expected_a_dsn_prefix):
        raise ValueError("backend-a DSN is not pinned to mysql-a with the dedicated A read-only account")
    if f"@tcp(mysql-b:3306)/{env['AB_MYSQL_B_DATABASE']}?" not in str(env_b.get("MYSQL_DSN", "")):
        raise ValueError("backend-b DSN is not pinned to mysql-b and the confirmed B database")
    if services["backend-a"].get("env_file") or services["backend-b"].get("env_file"):
        raise ValueError("backend services must not inherit an env_file")

    for side in ("a", "b"):
        backend = services[f"backend-{side}"]
        backend_env = backend.get("environment") or {}
        expected_upload = env[f"AB_UPLOAD_{side.upper()}_ORIGIN"]
        if backend_env.get("UPLOAD_SERVICE_BASE_URL") != expected_upload:
            raise ValueError(f"backend-{side} upload origin is not owned by side {side.upper()}")
        auth_mount = bind_mount(backend, "/run/ab/auth_identity.json")
        if not auth_mount or pathlib.Path(str(auth_mount.get("source"))) != auth_paths[side] or auth_mount.get("read_only") is not True:
            raise ValueError(f"backend-{side} auth mount must be the attested read-only {side.upper()} file")
        depends = backend.get("depends_on") or {}
        dependency_contract = {
            f"mysql-{side}": "service_healthy",
            f"redis-{side}": "service_healthy",
            f"clone-ready-{side}": "service_completed_successfully",
            f"fixture-ready-{side}": "service_completed_successfully",
        }
        for required, condition in dependency_contract.items():
            if not isinstance(depends.get(required), dict) or depends[required].get("condition") != condition:
                raise ValueError(f"backend-{side} must depend on {required} with condition={condition}")

        clone_gate = services[f"clone-ready-{side}"]
        clone_env = clone_gate.get("environment") or {}
        if clone_env.get("MYSQL_HOST") != f"mysql-{side}" or clone_env.get("MYSQL_DATABASE") != env[f"AB_MYSQL_{side.upper()}_DATABASE"]:
            raise ValueError(f"clone-ready-{side} database ownership mismatch")
        expected_gate_user = env["AB_MYSQL_A_READONLY_USER"] if side == "a" else env["AB_MYSQL_APP_USER"]
        expected_read_only = "true" if side == "a" else "false"
        if clone_env.get("MYSQL_USER") != expected_gate_user or str(clone_env.get("REQUIRE_READ_ONLY")).lower() != expected_read_only:
            raise ValueError(f"clone-ready-{side} account/read-only contract mismatch")
        if clone_env.get("EXPECTED_RECEIPT_SHA256") != receipt_hashes[side] or str(clone_env.get("EXPECTED_TASK_COUNT")) != str(task_counts[side]):
            raise ValueError(f"clone-ready-{side} receipt/task attestation mismatch")
        clone_command = json.dumps(clone_gate.get("command") or "")
        for required_token in ("import-receipt.json", "ready-marker.sha256", "information_schema.tables", "SELECT COUNT(*) FROM tasks", "EXPECTED_TASK_COUNT", "SHOW GRANTS FOR CURRENT_USER", "ALL PRIVILEGES"):
            if required_token not in clone_command:
                raise ValueError(f"clone-ready-{side} command is missing required attestation check {required_token!r}")
        for target, source in (("/run/ab/import-receipt.json", receipt_paths[side]), ("/run/ab/ready-marker.sha256", marker_paths[side])):
            mount = bind_mount(clone_gate, target)
            if not mount or pathlib.Path(str(mount.get("source"))) != source or mount.get("read_only") is not True:
                raise ValueError(f"clone-ready-{side} must mount the attested {target} read-only")

        fixture_env = services[f"fixture-ready-{side}"].get("environment") or {}
        fixture_expected = {
            "UPLOAD_ORIGIN": env[f"AB_UPLOAD_{side.upper()}_ORIGIN"],
            "UPLOAD_IDENTITY": env[f"AB_UPLOAD_{side.upper()}_IDENTITY"],
            "OBJECT_ORIGIN": env[f"AB_EXTERNAL_FILE_{side.upper()}_ORIGIN"],
            "OBJECT_IDENTITY": env[f"AB_OBJECT_{side.upper()}_IDENTITY"],
        }
        if any(fixture_env.get(key) != value for key, value in fixture_expected.items()):
            raise ValueError(f"fixture-ready-{side} origin/identity ownership mismatch")
        fixture_command = json.dumps(services[f"fixture-ready-{side}"].get("command") or "")
        for required_token in ("UPLOAD_ORIGIN/health", "UPLOAD_ORIGIN/identity", "OBJECT_ORIGIN/health", "OBJECT_ORIGIN/identity"):
            if required_token not in fixture_command:
                raise ValueError(f"fixture-ready-{side} command is missing health/identity check {required_token!r}")
        fixture_depends = services[f"fixture-ready-{side}"].get("depends_on") or {}
        for fixture_name in (f"fixture-upload-{side}", f"fixture-object-{side}"):
            if fixture_name not in fixture_depends:
                raise ValueError(f"fixture-ready-{side} must depend on {fixture_name}")

    edge_attestations: dict[str, dict[str, str]] = {}
    for edge_name, contract in EDGE_MATRIX.items():
        edge = services[edge_name]
        edge_env = edge.get("environment") or {}
        side = str(contract["side"])
        expected_edge_env = {
            "BACKEND_ORIGIN": f"http://{contract['backend']}:8080",
            "UPLOAD_ORIGIN": env[str(contract["upload_env"])],
            "EXTERNAL_FILE_ORIGIN": env[str(contract["object_env"])],
            "EDGE_IDENTITY": edge_name.removeprefix("edge-"),
            "FRONTEND_FINGERPRINT": env[str(contract["front_hash_env"])],
            "BACKEND_FINGERPRINT": env[str(contract["backend_hash_env"])],
            "FIXTURE_IDENTITY": ":".join(env[key] for key in contract["fixture_identity"]),
        }
        for key, value in expected_edge_env.items():
            if edge_env.get(key) != value:
                raise ValueError(f"{edge_name} {key} ownership mismatch: expected {value!r}")
        front_mount = bind_mount(edge, "/srv/front")
        expected_front = pathlib.Path(env[str(contract["front_env"])])
        if not front_mount or pathlib.Path(str(front_mount.get("source"))) != expected_front or front_mount.get("read_only") is not True:
            raise ValueError(f"{edge_name} frontend mount does not match its declared frontend artifact")
        depends = edge.get("depends_on") or {}
        if str(contract["backend"]) not in depends:
            raise ValueError(f"{edge_name} must depend on {contract['backend']}")
        edge_attestations[edge_name] = {
            "side": side,
            "frontend_sha256": expected_edge_env["FRONTEND_FINGERPRINT"],
            "backend_sha256": expected_edge_env["BACKEND_FINGERPRINT"],
            "fixture_identity": expected_edge_env["FIXTURE_IDENTITY"],
        }

    return {
        "status": "PASS",
        "compose_project": env["AB_COMPOSE_PROJECT"],
        "services": sorted(services),
        "published_services": sorted(EDGE_SERVICES),
        "published_host": "127.0.0.1",
        "edge_ports": sorted(int(value) for value in ports),
        "private_services_without_published_ports": sorted(PRIVATE_SERVICES),
        "network_isolation": {name: sorted(value) for name, value in sorted(SERVICE_NETWORKS.items())},
        "auth_settings_sha256": expected_auth_hash,
        "frontend_sha256": frontend_hashes,
        "backend_sha256": backend_hashes,
        "infrastructure_image_sha256": infrastructure_image_hashes,
        "import_receipt_sha256": receipt_hashes,
        "source_snapshot_sha256": receipts["a"]["source_snapshot_sha256"],
        "fixture_script_sha256": expected_fixture_script_hash,
        "fixture_attestations": fixture_attestations,
        "edge_attestations": edge_attestations,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--compose-file", type=pathlib.Path, required=True)
    parser.add_argument("--env-file", type=pathlib.Path, required=True)
    parser.add_argument("--output", type=pathlib.Path)
    args = parser.parse_args()
    try:
        result = validate(args.compose_file.resolve(), args.env_file.resolve())
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        result = {"status": "FAIL", "error": str(exc)}
        code = 1
    else:
        code = 0
    rendered = json.dumps(result, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
    if args.output:
        args.output.write_text(rendered, encoding="utf-8")
    sys.stdout.write(rendered)
    return code


if __name__ == "__main__":
    raise SystemExit(main())
