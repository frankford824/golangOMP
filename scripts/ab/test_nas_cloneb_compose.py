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

    def test_erp_bridge_is_internal_local_mode_and_read_only(self) -> None:
        bridge = self.services["erp-bridge"]
        backend = self.services["backend"]

        self.assertEqual(bridge["image"], backend["image"])
        self.assertNotIn("ports", bridge)
        self.assertEqual(set(bridge["networks"]), {"private"})
        self.assertEqual(bridge["environment"]["SERVER_PORT"], "8081")
        self.assertEqual(bridge["environment"]["ERP_REMOTE_MODE"], "local")
        self.assertEqual(bridge["environment"]["ERP_SYNC_ENABLED"], "false")
        self.assertEqual(bridge["environment"]["WEB_PUSH_ENABLED"], "false")

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
