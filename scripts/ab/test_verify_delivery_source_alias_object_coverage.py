#!/usr/bin/env python3

from __future__ import annotations

import copy
import hashlib
import unittest

from scripts.ab import verify_delivery_source_alias_object_coverage as module


def h(value: str) -> str:
    return hashlib.sha256(value.encode("utf-8")).hexdigest()


def mapping() -> dict:
    recovery = {
        "missing_task_asset_id": 100,
        "task_id": 44,
        "strategy": "historical_unavailable_tombstone_v1",
        "review_policy_ids": [
            "legacy_historical_asset_unavailable_v1"
        ],
    }
    recovery["manifest_row_hash"] = module.canonical_value_hash(recovery)
    return {
        "version": 2,
        "asset_recoveries": [recovery],
        "resources": [
            {
                "task_id": 44,
                "scope_kind": "sku",
                "scope_ref_id": 55,
                "working_revision_no": 2,
                "finalized_revision_no": 2,
                "history": [
                    {
                        "revision_no": 1,
                        "status": "superseded",
                        "source_alias_from_task_asset_id": 100,
                        "final_task_asset_ids": [100],
                        "review_policy_ids": [
                            "explicit_event_replay",
                            "delivery_source_alias",
                        ],
                        "manifest_row_hash": h("revision-1"),
                    },
                    {
                        "revision_no": 2,
                        "status": "finalized",
                        "source_alias_from_task_asset_id": 100,
                        "final_task_asset_ids": [100],
                        "review_policy_ids": [
                            "explicit_event_replay",
                            "delivery_source_alias",
                        ],
                        "manifest_row_hash": h("revision-2"),
                    },
                ],
            }
        ],
    }


def predecessor() -> dict:
    return {
        "id": 100,
        "task_id": 44,
        "asset_id": 900,
        "scope_sku_code": "SKU-55",
        "retouch_requirement_id": None,
        "asset_type": "delivery",
        "binding_state": "bound",
        "bound_group_id": 700,
        "bound_role": "final",
        "version_no": 3,
        "asset_version_no": 8,
        "upload_mode": "small",
        "upload_request_id": "request-1",
        "storage_ref_id": "ref-100",
        "file_name": "final.png",
        "original_filename": "final.png",
        "remote_file_id": "remote-1",
        "mime_type": "image/png",
        "file_size": 1234,
        "file_path": "",
        "storage_key": "tasks/44/final.png",
        "whole_hash": h("bytes"),
        "upload_status": "uploaded",
        "preview_status": "ready",
        "uploaded_by": 1,
        "uploaded_at": "2026-01-01T00:00:00.000000Z",
        "remark": "original",
        "source_module_key": "design",
        "source_task_module_id": 8,
        "is_archived": 0,
        "flow_review_status": "approved",
        "deleted_at": None,
        "cleaned_at": None,
        "access_revoked_at": None,
        "object_deleted_at": None,
    }


def alias() -> dict:
    value = copy.deepcopy(predecessor())
    value.update(
        {
            "id": 1000,
            "asset_type": "source",
            "binding_state": "bound",
            "bound_group_id": 700,
            "bound_role": "source",
            "version_no": 9,
            "remark": "v8-source-alias:group=700:origin=100",
            "source_module_key": "migration",
            "source_task_module_id": None,
            "is_archived": 0,
            "flow_review_status": "not_applicable",
        }
    )
    return value


def storage() -> dict:
    return {
        "ref_id": "ref-100",
        "asset_id": 900,
        "owner_type": "task_asset",
        "owner_id": 100,
        "upload_request_id": "request-1",
        "storage_adapter": "oss_upload_service",
        "ref_type": "task_asset_object",
        "ref_key": "tasks/44/final.png",
        "file_name": "final.png",
        "mime_type": "image/png",
        "file_size": 1234,
        "is_placeholder": 0,
        "checksum_hint": h("bytes"),
        "status": "recorded",
    }


def manifest_row(*, unavailable: bool = False) -> dict:
    return {
        "entity_key": "task_asset:100",
        "owner_kind": "task_asset",
        "owner_id": 100,
        "task_id": 44,
        "storage_ref_id": "ref-100",
        "storage_adapter": "oss_upload_service",
        "object_key": "tasks/44/final.png",
        "size": 1234,
        "mime_type": "image/png",
        "sha256": "" if unavailable else h("bytes"),
        "status": "recorded",
        "is_placeholder": False,
    }


def observed_alias() -> dict:
    return {
        "alias": alias(),
        "predecessor": predecessor(),
        "resource_group": {
            "id": 700,
            "task_id": 44,
            "scope_kind": "sku",
            "scope_ref_id": 55,
            "working_revision_id": 8002,
            "finalized_revision_id": 8002,
        },
        "storage_ref": storage(),
    }

def approved_bundle() -> dict:
    return {
        "task_id": 44,
        "scope_kind": "sku",
        "scope_ref_id": 55,
        "revision_no": 2,
        "status": "finalized",
        "is_working": True,
        "is_finalized": True,
        "bundle_sha256": h("bundle-bytes"),
        "manifest_sha256": h("bundle-manifest"),
        "materialization_manifest_sha256": h("bundle-manifest"),
        "bundle_asset_id": 23989,
        "bundle_storage_ref_id": "bundle-ref-25557",
        "confirmed_by": 1,
        "confirmed_at": "2026-07-24T01:57:49Z",
        "confirmation_note": "approved",
        "members": [
            {
                "task_asset_id": 100,
                "sha256": h("bytes"),
            }
        ],
    }


def observed_bundle() -> dict:
    value = observed_alias()
    value["alias"].update(
        {
            "id": 25557,
            "asset_id": 23989,
            "storage_ref_id": "bundle-ref-25557",
            "whole_hash": h("bundle-bytes"),
            "mime_type": "application/zip",
            "storage_key": "bundles/task-44/source-bundle.zip",
            "remark": (
                "v8-migration-source-bundle:"
                "bundles/task-44/source-bundle.zip:"
                + h("bundle-manifest")
            ),
        }
    )
    value["resource_group"].update(
        {"scope_kind": "sku", "scope_ref_id": 55}
    )
    value["storage_ref"].update(
        {
            "ref_id": value["alias"]["storage_ref_id"],
            "asset_id": 25557,
            "owner_id": 25557,
            "ref_key": value["alias"]["storage_key"],
            "mime_type": "application/zip",
            "file_size": value["alias"]["file_size"],
            "checksum_hint": h("bundle-bytes"),
            "status": "recorded",
            "is_placeholder": 0,
        }
    )
    return value


def bundle_manifest_row() -> dict:
    return {
        "entity_key": "task_asset:25557",
        "owner_kind": "task_asset",
        "owner_id": 25557,
        "task_id": 44,
        "storage_ref_id": "bundle-ref-25557",
        "storage_adapter": "clone_b_bundle",
        "object_key": "bundles/task-44/source-bundle.zip",
        "size": 1234,
        "mime_type": "application/zip",
        "sha256": h("bundle-bytes"),
        "status": "recorded",
        "is_placeholder": False,
    }


def usages() -> list[dict]:
    return [
        {
            "alias_id": 1000,
            "revision_id": 8001,
            "group_id": 700,
            "revision_no": 1,
            "status": "superseded",
            "is_working": False,
            "is_finalized": False,
        },
        {
            "alias_id": 1000,
            "revision_id": 8002,
            "group_id": 700,
            "revision_no": 2,
            "status": "finalized",
            "is_working": True,
            "is_finalized": True,
        },
    ]


def exception() -> dict:
    return {
        "entity_key": "task_asset:100",
        "expected_http_status": 410,
        "observed_http_status": 410,
        "working_reference_count": 0,
        "finalized_reference_count": 0,
        "missing_task_asset_id": 100,
        "task_id": 44,
        "object_row_sha256": module.canonical_value_hash(
            manifest_row(unavailable=True)
        ),
        "mapping_row_hash": mapping()["asset_recoveries"][0][
            "manifest_row_hash"
        ],
    }


class DeliverySourceAliasCoverageTests(unittest.TestCase):
    def verify(
        self,
        *,
        mapping_value: dict | None = None,
        aliases: list[dict] | None = None,
        usage_rows: list[dict] | None = None,
        manifest_value: dict | None = None,
        exceptions: dict[str, dict] | None = None,
    ) -> dict:
        mapping_value = mapping_value or mapping()
        expected = module.parse_mapping(mapping_value)
        manifest_value = manifest_value or {
            "task_asset:100": manifest_row()
        }
        return module.verify_coverage(
            run_id="formal-test",
            database="ab_formal_test_b",
            mapping_sha256=h("mapping"),
            manifest_sha256=h("manifest"),
            verdict_sha256=h("verdict"),
            expected=expected,
            manifest=manifest_value,
            exceptions=exceptions or {},
            observed_aliases=aliases
            if aliases is not None
            else [observed_alias()],
            observed_usages=usage_rows
            if usage_rows is not None
            else usages(),
        )

    def test_pass_binds_alias_predecessor_object_and_revisions(self):
        result = self.verify()
        self.assertEqual(result["status"], "PASS")
        self.assertEqual(result["violation_count"], 0)
        self.assertEqual(result["expected_alias_count"], 1)
        self.assertEqual(result["expected_alias_revision_count"], 2)
        self.assertEqual(result["verified_predecessor_count"], 1)
        self.assertEqual(result["entry_count"], 1)
        self.assertEqual(
            result["entries"][0]["coverage_mode"],
            "verified_predecessor_object",
        )
        body = dict(result)
        evidence_hash = body.pop("evidence_sha256")
        self.assertEqual(
            evidence_hash, module.sha256_bytes(module.canonical_bytes(body))
        )

    def test_bundle_confirmation_binds_template_allocation_and_confirmed_members(self):
        expected = {25557: approved_bundle()}
        confirmed = {
            "schema_version": 1,
            "status": "CONFIRMED",
            "bundle_count": 1,
            "confirmed_by": 1,
            "confirmed_at": "2026-07-24T01:57:49Z",
            "confirmation_note": "approved",
            "decision_template_sha256": h("decision-template"),
            "bundles": [
                {
                    "confirmed": True,
                    "bundle_task_asset_id": 25557,
                    "bundle_asset_id": 23989,
                    "bundle_storage_ref_id": "bundle-ref-25557",
                    "task_id": 44,
                    "scope_kind": "sku",
                    "scope_ref_id": 55,
                    "revision_no": 2,
                    "ordered_members": [
                        {
                            "confirmed": True,
                            "task_asset_id": 100,
                            "sha256": h("bytes"),
                        }
                    ],
                }
            ],
        }
        module.bind_confirmed_bundle_manifest(
            confirmed,
            h("confirmed-manifest"),
            h("decision-template"),
            expected,
        )
        self.assertEqual(
            expected[25557]["materialization_manifest_sha256"],
            h("confirmed-manifest"),
        )
        rejected = copy.deepcopy(confirmed)
        rejected["bundles"][0]["ordered_members"][0]["confirmed"] = False
        with self.assertRaisesRegex(ValueError, "metadata differs"):
            module.bind_confirmed_bundle_manifest(
                rejected,
                h("confirmed-manifest"),
                h("decision-template"),
                {25557: approved_bundle()},
            )
        with self.assertRaisesRegex(ValueError, "manifest is invalid"):
            module.bind_confirmed_bundle_manifest(
                confirmed,
                h("confirmed-manifest"),
                h("different-template"),
                {25557: approved_bundle()},
            )

    def test_copied_field_drift_fails(self):
        record = observed_alias()
        record["alias"]["storage_ref_id"] = "ref-other"
        result = self.verify(aliases=[record])
        codes = {row["violation_code"] for row in result["violations"]}
        self.assertIn("alias_coverage.predecessor_copy_drift", codes)
        self.assertIn("alias_coverage.object_identity_drift", codes)

    def test_unexpected_alias_fails(self):
        extra = observed_alias()
        extra["alias"]["id"] = 1001
        extra["alias"]["remark"] = (
            "v8-source-alias:group=700:origin=101"
        )
        extra["predecessor"]["id"] = 101
        result = self.verify(aliases=[observed_alias(), extra])
        self.assertIn(
            "alias_coverage.unexpected_alias",
            {row["violation_code"] for row in result["violations"]},
        )

    def test_malformed_migration_source_alias_is_observable_and_fails(self):
        malformed = observed_alias()
        malformed["alias"]["id"] = 1001
        malformed["alias"]["remark"] = None
        result = self.verify(aliases=[observed_alias(), malformed])
        self.assertIn(
            "alias_coverage.remark_invalid",
            {row["violation_code"] for row in result["violations"]},
        )

    def test_revision_usage_drift_fails(self):
        result = self.verify(usage_rows=usages()[:1])
        self.assertIn(
            "alias_coverage.revision_usage_drift",
            {row["violation_code"] for row in result["violations"]},
        )

    def test_manifest_object_identity_drift_fails(self):
        row = manifest_row()
        row["object_key"] = "tasks/44/other.png"
        result = self.verify(manifest_value={"task_asset:100": row})
        self.assertIn(
            "alias_coverage.object_identity_drift",
            {row["violation_code"] for row in result["violations"]},
        )

    def test_hydrated_byte_metadata_and_storage_ref_key_may_differ(self):
        record = observed_alias()
        record["storage_ref"]["ref_key"] = "legacy/original-name.png"
        record["storage_ref"]["mime_type"] = "image/jpeg"
        record["storage_ref"]["file_size"] = 999
        row = manifest_row()
        row["mime_type"] = "application/octet-stream"
        row["size"] = 1234
        result = self.verify(
            aliases=[record],
            manifest_value={"task_asset:100": row},
        )
        self.assertEqual(result["status"], "PASS")

    def test_manifest_object_key_must_follow_predecessor_storage_key(self):
        record = observed_alias()
        record["predecessor"]["storage_key"] = "tasks/44/predecessor.png"
        record["alias"]["storage_key"] = "tasks/44/predecessor.png"
        result = self.verify(aliases=[record])
        self.assertIn(
            "alias_coverage.object_identity_drift",
            {row["violation_code"] for row in result["violations"]},
        )

    def test_approved_bundle_is_excluded_only_after_shape_and_usage_validation(self):
        bundle_usage = {
            "alias_id": 25557,
            "revision_id": 8002,
            "group_id": 700,
            "revision_no": 2,
            "status": "finalized",
            "is_working": True,
            "is_finalized": True,
        }
        result = module.verify_coverage(
            run_id="formal-test",
            database="ab_formal_test_b",
            mapping_sha256=h("mapping"),
            manifest_sha256=h("manifest"),
            verdict_sha256=h("verdict"),
            expected=module.parse_mapping(mapping()),
            manifest={
                "task_asset:100": manifest_row(),
                "task_asset:25557": bundle_manifest_row(),
            },
            exceptions={},
            observed_aliases=[observed_alias(), observed_bundle()],
            observed_usages=[*usages(), bundle_usage],
            approved_bundles={25557: approved_bundle()},
        )
        self.assertEqual(result["status"], "PASS")
        wrong_group_usage = dict(bundle_usage)
        wrong_group_usage["group_id"] = 999
        result = module.verify_coverage(
            run_id="formal-test",
            database="ab_formal_test_b",
            mapping_sha256=h("mapping"),
            manifest_sha256=h("manifest"),
            verdict_sha256=h("verdict"),
            expected=module.parse_mapping(mapping()),
            manifest={
                "task_asset:100": manifest_row(),
                "task_asset:25557": bundle_manifest_row(),
            },
            exceptions={},
            observed_aliases=[observed_alias(), observed_bundle()],
            observed_usages=[*usages(), wrong_group_usage],
            approved_bundles={25557: approved_bundle()},
        )
        self.assertIn(
            "alias_coverage.bundle_usage_drift",
            {row["violation_code"] for row in result["violations"]},
        )
        malformed = observed_bundle()
        malformed["alias"]["remark"] = None
        result = module.verify_coverage(
            run_id="formal-test",
            database="ab_formal_test_b",
            mapping_sha256=h("mapping"),
            manifest_sha256=h("manifest"),
            verdict_sha256=h("verdict"),
            expected=module.parse_mapping(mapping()),
            manifest={
                "task_asset:100": manifest_row(),
                "task_asset:25557": bundle_manifest_row(),
            },
            exceptions={},
            observed_aliases=[observed_alias(), malformed],
            observed_usages=[*usages(), bundle_usage],
            approved_bundles={25557: approved_bundle()},
        )
        self.assertIn(
            "alias_coverage.bundle_shape_drift",
            {row["violation_code"] for row in result["violations"]},
        )
        wrong_manifest = observed_bundle()
        wrong_manifest["alias"]["remark"] = (
            "v8-migration-source-bundle:"
            "bundles/task-44/source-bundle.zip:"
            + h("different-bundle-manifest")
        )
        self.assertTrue(
            module.approved_bundle_problems(
                approved_bundle(),
                wrong_manifest,
                bundle_manifest_row(),
            )
        )
        wrong_allocation = observed_bundle()
        wrong_allocation["alias"]["asset_id"] = 999999
        wrong_allocation["alias"]["storage_ref_id"] = "different-ref"
        wrong_allocation["storage_ref"]["ref_id"] = "different-ref"
        self.assertTrue(
            module.approved_bundle_problems(
                approved_bundle(),
                wrong_allocation,
                bundle_manifest_row(),
            )
        )

    def test_historical_unavailable_exception_passes_when_not_current(self):
        value = mapping()
        value["resources"][0]["working_revision_no"] = None
        value["resources"][0]["finalized_revision_no"] = None
        value["resources"][0]["history"] = [
            value["resources"][0]["history"][0]
        ]
        exception_usage = usages()[:1]
        result = self.verify(
            mapping_value=value,
            usage_rows=exception_usage,
            manifest_value={"task_asset:100": manifest_row(unavailable=True)},
            exceptions={"task_asset:100": exception()},
        )
        self.assertEqual(result["status"], "PASS")
        self.assertEqual(
            result["entries"][0]["coverage_mode"],
            "historical_unavailable_exception",
        )
        self.assertEqual(result["historical_unavailable_exception_count"], 1)

    def test_historical_exception_does_not_require_current_storage_status(self):
        value = mapping()
        value["resources"][0]["working_revision_no"] = None
        value["resources"][0]["finalized_revision_no"] = None
        value["resources"][0]["history"] = [
            value["resources"][0]["history"][0]
        ]
        record = observed_alias()
        record["storage_ref"]["status"] = "deleted"
        result = self.verify(
            mapping_value=value,
            aliases=[record],
            usage_rows=usages()[:1],
            manifest_value={"task_asset:100": manifest_row(unavailable=True)},
            exceptions={"task_asset:100": exception()},
        )
        self.assertEqual(result["status"], "PASS")

    def test_historical_unavailable_exception_fails_when_current(self):
        result = self.verify(
            manifest_value={"task_asset:100": manifest_row(unavailable=True)},
            exceptions={"task_asset:100": exception()},
        )
        self.assertIn(
            "alias_coverage.unavailable_alias_is_current",
            {row["violation_code"] for row in result["violations"]},
        )

    def test_historical_unavailable_exception_requires_exact_row_hashes(self):
        value = mapping()
        value["resources"][0]["working_revision_no"] = None
        value["resources"][0]["finalized_revision_no"] = None
        value["resources"][0]["history"] = [
            value["resources"][0]["history"][0]
        ]
        attestation = exception()
        attestation["object_row_sha256"] = h("different-object-row")
        result = self.verify(
            mapping_value=value,
            usage_rows=usages()[:1],
            manifest_value={"task_asset:100": manifest_row(unavailable=True)},
            exceptions={"task_asset:100": attestation},
        )
        self.assertIn(
            "alias_coverage.unavailable_exception_drift",
            {row["violation_code"] for row in result["violations"]},
        )

    def test_unused_historical_exception_fails(self):
        unused = exception()
        unused["entity_key"] = "task_asset:999"
        unused["missing_task_asset_id"] = 999
        result = self.verify(
            exceptions={"task_asset:999": unused},
        )
        self.assertIn(
            "alias_coverage.unused_historical_exception",
            {row["violation_code"] for row in result["violations"]},
        )

    def test_exception_manifest_row_is_not_blanket_skipped(self):
        with self.assertRaisesRegex(ValueError, "failed its contract"):
            module.index_manifest(
                [{"entity_key": "task_asset:100", "garbage": True}],
                {"task_asset:100": exception()},
            )

    def test_mapping_requires_approved_policy_and_final_membership(self):
        value = mapping()
        revision = value["resources"][0]["history"][0]
        revision["review_policy_ids"] = ["explicit_event_replay"]
        with self.assertRaisesRegex(ValueError, "approved policy"):
            module.parse_mapping(value)

    def test_parse_json_rows_rejects_non_object(self):
        with self.assertRaisesRegex(ValueError, "not an object"):
            module.parse_json_rows("[]\n", "test")

    def test_object_verdict_requires_exact_contract_and_self_hash(self):
        value = {
            "schema_version": 1,
            "status": "PASS",
            "violation_count": 0,
            "violations": [],
            "checked_count": 1,
            "exception_count": 0,
            "exception_evidence_sha256": module.ZERO_SHA256,
            "exceptions": [],
            "manifest_sha256": h("manifest"),
            "mapping_row_hash": module.ZERO_SHA256,
            "mapping_sha256": h("mapping"),
        }
        value["evidence_hash"] = module.canonical_value_hash(value)
        self.assertEqual(
            module.validate_verdict(
                value,
                mapping_sha256=h("mapping"),
                manifest_sha256=h("manifest"),
            ),
            {},
        )
        value["checked_count"] = 2
        with self.assertRaisesRegex(ValueError, "self-hash"):
            module.validate_verdict(
                value,
                mapping_sha256=h("mapping"),
                manifest_sha256=h("manifest"),
            )

    def test_object_verdict_requires_exception_top_level_binding(self):
        accepted = exception()
        value = {
            "schema_version": 1,
            "status": "PASS",
            "violation_count": 0,
            "violations": [],
            "checked_count": 1,
            "exception_count": 1,
            "exception_evidence_sha256": h("exception-attestation"),
            "exceptions": [accepted],
            "manifest_sha256": h("manifest"),
            "mapping_row_hash": accepted["mapping_row_hash"],
            "mapping_sha256": h("mapping"),
        }
        value["evidence_hash"] = module.canonical_value_hash(value)
        self.assertEqual(
            module.validate_verdict(
                value,
                mapping_sha256=h("mapping"),
                manifest_sha256=h("manifest"),
                exception_attestation_sha256=h("exception-attestation"),
            ),
            {"task_asset:100": accepted},
        )
        with self.assertRaisesRegex(ValueError, "binding"):
            module.validate_verdict(
                value,
                mapping_sha256=h("mapping"),
                manifest_sha256=h("manifest"),
                exception_attestation_sha256=h("different-attestation"),
            )
        value["mapping_row_hash"] = h("unrelated")
        value.pop("evidence_hash")
        value["evidence_hash"] = module.canonical_value_hash(value)
        with self.assertRaisesRegex(ValueError, "binding"):
            module.validate_verdict(
                value,
                mapping_sha256=h("mapping"),
                manifest_sha256=h("manifest"),
                exception_attestation_sha256=h("exception-attestation"),
            )


if __name__ == "__main__":
    unittest.main()
