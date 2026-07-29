#!/usr/bin/env python3
from __future__ import annotations

import pathlib
import unittest

import yaml


ROOT = pathlib.Path(__file__).resolve().parents[2]
COMPOSE_PATH = ROOT / "deploy" / "ab-browser" / "nas" / "compose.yaml"
BOOTSTRAP_PATH = ROOT / "deploy" / "ab-browser" / "nas" / "bootstrap.sh"


class NASCloneBComposeTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.compose = yaml.safe_load(COMPOSE_PATH.read_text(encoding="utf-8"))
        cls.services = cls.compose["services"]

    def test_erp_bridge_is_internal_and_defaults_to_local_mode(self) -> None:
        bridge = self.services["erp-bridge"]
        backend = self.services["backend"]

        self.assertEqual(bridge["image"], backend["image"])
        self.assertNotIn("ports", bridge)
        self.assertEqual(set(bridge["networks"]), {"private"})
        self.assertEqual(bridge["environment"]["SERVER_PORT"], "8081")
        self.assertEqual(
            bridge["environment"]["ERP_REMOTE_MODE"],
            "${ERP_REMOTE_MODE:-local}",
        )
        self.assertEqual(
            bridge["environment"]["ERP_REMOTE_FALLBACK_LOCAL_ON_ERROR"],
            "${ERP_REMOTE_FALLBACK_LOCAL_ON_ERROR:-false}",
        )
        self.assertEqual(bridge["environment"]["ERP_SYNC_ENABLED"], "false")
        self.assertEqual(bridge["environment"]["WEB_PUSH_ENABLED"], "false")

    def test_real_integrations_require_explicit_env_overrides(self) -> None:
        bridge = self.services["erp-bridge"]["environment"]
        backend = self.services["backend"]["environment"]
        edge = self.services["edge"]["environment"]

        self.assertEqual(
            bridge["ERP_REMOTE_BASE_URL"],
            "${ERP_REMOTE_BASE_URL:-}",
        )
        self.assertEqual(
            bridge["ERP_REMOTE_APP_SECRET"],
            "${ERP_REMOTE_APP_SECRET:-}",
        )
        self.assertEqual(
            backend["OSS_DIRECT_ENABLED"],
            "${OSS_DIRECT_ENABLED:-false}",
        )
        self.assertEqual(backend["OSS_BUCKET"], "${OSS_BUCKET:-}")
        self.assertEqual(
            backend["OSS_ACCESS_KEY_SECRET"],
            "${OSS_ACCESS_KEY_SECRET:-}",
        )
        self.assertEqual(
            backend["UPLOAD_SERVICE_ENABLED"],
            "${UPLOAD_SERVICE_ENABLED:-true}",
        )
        self.assertEqual(
            edge["UPLOAD_ORIGIN"],
            "${UPLOAD_ORIGIN:-http://fixture-upload:18092}",
        )

    def test_main_backend_uses_healthy_compose_bridge(self) -> None:
        backend = self.services["backend"]

        self.assertEqual(
            backend["environment"]["ERP_BRIDGE_BASE_URL"],
            "http://erp-bridge:8081",
        )
        self.assertEqual(
            backend["depends_on"]["erp-bridge"]["condition"],
            "service_healthy",
        )

    def test_bootstrap_starts_and_waits_for_bridge(self) -> None:
        bootstrap = BOOTSTRAP_PATH.read_text(encoding="utf-8")

        self.assertIn(
            '"${COMPOSE[@]}" up -d erp-bridge backend edge',
            bootstrap,
        )
        self.assertIn("wait_healthy erp-bridge", bootstrap)


if __name__ == "__main__":
    unittest.main()
