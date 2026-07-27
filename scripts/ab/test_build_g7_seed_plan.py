import hashlib
import json
import sys
import tempfile
import unittest
from pathlib import Path


sys.path.insert(0, str(Path(__file__).parent))
import build_g7_seed_plan as planner  # noqa: E402


def digest(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def write_json(path: Path, value: object) -> None:
    path.write_text(
        json.dumps(value, ensure_ascii=False, sort_keys=True) + "\n",
        encoding="utf-8",
    )


class BuildG7SeedPlanTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.samples = self.root / "samples.json"
        self.objects = self.root / "objects.jsonl"
        self.inventory = self.root / "inventory.json"
        self.a_root = self.root / "a"
        self.b_root = self.root / "b"
        (self.a_root / "objects").mkdir(parents=True)
        (self.b_root / "objects").mkdir(parents=True)
        self.first_bytes = b"first-object"
        self.missing_bytes = b"historical-unavailable"
        self.first_key = "tasks/one/preview.webp"
        self.missing_key = "tasks/two/missing.psd"
        self._write_samples()
        self._write_object_manifest()
        self._write_inventory()

    def tearDown(self) -> None:
        self.temp.cleanup()

    def _write_samples(self) -> None:
        samples = {
            "schema_version": 1,
            "gate": "G7",
            "status": "PASS",
            "mode": "final",
            "samples": [
                {
                    "scenario_id": "baseline",
                    "status": "READY",
                    "coverage_matrix": [
                        {"combination": combination, "task_id": 1}
                        for combination in (
                            "external_external",
                            "devplus_devplus",
                            "external_devplus",
                            "devplus_external",
                        )
                    ],
                },
                {
                    "scenario_id": "historical_410",
                    "status": "READY",
                    "coverage_matrix": [
                        {"combination": "devplus_devplus", "task_id": 2}
                    ],
                },
            ],
        }
        samples["manifest_sha256"] = planner.canonical_sha256(samples)
        write_json(self.samples, samples)

    def _object_row(
        self,
        entity: str,
        task_id: int,
        key: str,
        payload: bytes,
    ) -> dict:
        owner_id = int(entity.split(":", 1)[1])
        return {
            "entity_key": entity,
            "owner_kind": "task_asset",
            "owner_id": owner_id,
            "task_id": task_id,
            "storage_ref_id": f"ref-{owner_id}",
            "storage_adapter": "oss_upload_service",
            "object_key": key,
            "size": len(payload),
            "mime_type": "application/octet-stream",
            "sha256": digest(payload),
            "status": "recorded",
            "is_placeholder": False,
        }

    def _write_object_manifest(self) -> None:
        rows = [
            self._object_row(
                "task_asset:1",
                1,
                self.first_key,
                self.first_bytes,
            ),
            self._object_row(
                "task_asset:12323",
                2,
                self.missing_key,
                self.missing_bytes,
            ),
        ]
        self.objects.write_text(
            "".join(planner.canonical_json(row) + "\n" for row in rows),
            encoding="utf-8",
        )

    def _inventory_row(
        self,
        namespace: str,
        scenarios: list[str],
        task_ids: list[int],
        entity: str,
        key: str,
        payload: bytes,
        status: int,
    ) -> dict:
        return {
            "namespace": namespace,
            "scenario_ids": scenarios,
            "task_ids": task_ids,
            "source_entity_key": entity,
            "object_key": key,
            "size": len(payload),
            "mime_type": "application/octet-stream",
            "sha256": digest(payload),
            "expected_http_status": status,
        }

    def _write_inventory(self, mutator=None) -> None:
        rows = [
            self._inventory_row(
                "A",
                ["baseline"],
                [1],
                "task_asset:1",
                self.first_key,
                self.first_bytes,
                200,
            ),
            self._inventory_row(
                "B",
                ["baseline"],
                [1],
                "task_asset:1",
                self.first_key,
                self.first_bytes,
                200,
            ),
            self._inventory_row(
                "B",
                ["historical_410"],
                [2],
                "task_asset:12323",
                self.missing_key,
                self.missing_bytes,
                410,
            ),
        ]
        inventory = {
            "schema_version": 1,
            "gate": "G7",
            "status": "REVIEWED",
            "run_id": "seed-test",
            "samples_sha256": planner.file_sha256(self.samples),
            "object_manifest_sha256": planner.file_sha256(self.objects),
            "rows": rows,
        }
        if mutator:
            mutator(inventory)
        inventory["manifest_sha256"] = planner.canonical_sha256(inventory)
        write_json(self.inventory, inventory)

    def test_builds_hash_bound_read_only_plan_without_fixture_writes(self) -> None:
        existing = self.b_root / "objects" / self.first_key
        existing.parent.mkdir(parents=True)
        existing.write_bytes(self.first_bytes)
        before = sorted(path.relative_to(self.root) for path in self.root.rglob("*"))
        plan = planner.build_plan(
            samples_path=self.samples,
            object_manifest_path=self.objects,
            inventory_path=self.inventory,
            roots={"A": [self.a_root], "B": [self.b_root]},
            historical_unavailable_entity="task_asset:12323",
        )
        after = sorted(path.relative_to(self.root) for path in self.root.rglob("*"))
        self.assertEqual(before, after)
        self.assertEqual("READY_FOR_LOCAL_SEED", plan["status"])
        self.assertEqual(
            {
                "plan_row_count": 3,
                "expected_present_count": 2,
                "expected_absent_count": 1,
                "reuse_count": 1,
                "fetch_count": 1,
                "fetch_bytes": len(self.first_bytes),
                "blocker_count": 0,
            },
            plan["summary"],
        )
        self.assertFalse(plan["constraints"]["production_write_authorized"])
        self.assertFalse(plan["constraints"]["fixture_download_performed"])
        self.assertFalse(plan["constraints"]["fixture_write_performed"])
        self.assertEqual(
            plan["manifest_sha256"],
            planner.canonical_sha256(
                {
                    key: value
                    for key, value in plan.items()
                    if key != "manifest_sha256"
                }
            ),
        )

    def test_wrong_existing_bytes_are_a_blocker(self) -> None:
        existing = self.b_root / "objects" / self.first_key
        existing.parent.mkdir(parents=True)
        existing.write_bytes(b"wrong")
        plan = planner.build_plan(
            samples_path=self.samples,
            object_manifest_path=self.objects,
            inventory_path=self.inventory,
            roots={"A": [self.a_root], "B": [self.b_root]},
            historical_unavailable_entity="task_asset:12323",
        )
        self.assertEqual("BLOCKED", plan["status"])
        self.assertEqual(1, plan["summary"]["blocker_count"])

    def test_rejects_unapproved_410_and_manifest_drift(self) -> None:
        def wrong_410(inventory: dict) -> None:
            inventory["rows"][0]["expected_http_status"] = 410

        self._write_inventory(wrong_410)
        with self.assertRaisesRegex(planner.InputError, "unapproved 410"):
            planner.build_plan(
                samples_path=self.samples,
                object_manifest_path=self.objects,
                inventory_path=self.inventory,
                roots={"A": [], "B": []},
                historical_unavailable_entity="task_asset:12323",
            )

        def drift(inventory: dict) -> None:
            inventory["rows"][0]["size"] += 1

        self._write_inventory(drift)
        with self.assertRaisesRegex(planner.InputError, "final object manifest"):
            planner.build_plan(
                samples_path=self.samples,
                object_manifest_path=self.objects,
                inventory_path=self.inventory,
                roots={"A": [], "B": []},
                historical_unavailable_entity="task_asset:12323",
            )

        def wrong_task_binding(inventory: dict) -> None:
            inventory["rows"][0]["task_ids"] = [2]

        self._write_inventory(wrong_task_binding)
        with self.assertRaisesRegex(planner.InputError, "selected A sample tasks"):
            planner.build_plan(
                samples_path=self.samples,
                object_manifest_path=self.objects,
                inventory_path=self.inventory,
                roots={"A": [], "B": []},
                historical_unavailable_entity="task_asset:12323",
            )


if __name__ == "__main__":
    unittest.main()
