import hashlib
import importlib.util
import pathlib
import unittest


PATH = pathlib.Path(__file__).with_name(
    "authorize_automatic_mapping_review.py"
)
SPEC = importlib.util.spec_from_file_location(
    "authorize_automatic_mapping_review",
    PATH,
)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class AuthorizeAutomaticMappingReviewTest(unittest.TestCase):
    def fixture(self):
        ledger = {
            "schema_version": 1,
            "candidate_sha256": "a" * 64,
            "cohort_digest": "b" * 64,
            "summary": {"total_eligible_count": 2},
            "rows": [
                {
                    "eligible": True,
                    "required_policies": [
                        "explicit_event_replay",
                        "delivery_source_alias",
                    ],
                },
                {
                    "eligible": True,
                    "required_policies": [
                        "legacy_atomic_upload_batch_submit_v1",
                    ],
                },
                {
                    "eligible": False,
                    "required_policies": [
                        "legacy_org_manual_target_required_v1",
                    ],
                },
            ],
        }
        ledger_sha = hashlib.sha256(
            MODULE.canonical_bytes(ledger)
        ).hexdigest()
        template = {
            "schema_version": 1,
            "decision": "PENDING_REVIEW",
            "candidate_sha256": ledger["candidate_sha256"],
            "cohort_digest": ledger["cohort_digest"],
            "ledger_sha256": ledger_sha,
        }
        return ledger, template, ledger_sha

    def test_approves_only_policies_required_by_eligible_rows(self):
        ledger, template, ledger_sha = self.fixture()
        decision = MODULE.build_decision(
            ledger,
            template,
            ledger_sha256=ledger_sha,
            reviewer_id=1,
            reviewed_at="2026-08-04T11:15:30Z",
            authorization_note=(
                "Admin authorized automatic adjudication for the frozen run"
            ),
        )
        self.assertEqual(decision["decision"], "approve")
        self.assertFalse(decision["manual_row_review_performed"])
        self.assertEqual(decision["eligible_row_count"], 2)
        self.assertEqual(
            decision["approved_policies"],
            [
                "explicit_event_replay",
                "delivery_source_alias",
                "legacy_atomic_upload_batch_submit_v1",
            ],
        )

    def test_rejects_stale_template_and_unknown_policy(self):
        ledger, template, ledger_sha = self.fixture()
        template["ledger_sha256"] = "0" * 64
        with self.assertRaisesRegex(ValueError, "does not bind"):
            MODULE.build_decision(
                ledger,
                template,
                ledger_sha256=ledger_sha,
                reviewer_id=1,
                reviewed_at="2026-08-04T11:15:30Z",
                authorization_note="authorized",
            )
        ledger, template, ledger_sha = self.fixture()
        ledger["rows"][0]["required_policies"] = ["unknown_policy"]
        with self.assertRaisesRegex(ValueError, "invalid policies"):
            MODULE.build_decision(
                ledger,
                template,
                ledger_sha256=ledger_sha,
                reviewer_id=1,
                reviewed_at="2026-08-04T11:15:30Z",
                authorization_note="authorized",
            )


if __name__ == "__main__":
    unittest.main()
