from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import tempfile
import unittest

import build_approved_decisions as approved
import review_migration_mapping as review


def digest(path: pathlib.Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def proposed_mapping() -> dict:
    revision = {
        "revision_no": 1,
        "status": "finalized",
        "mode": "single",
        "source_stage": "design",
        "source_alias_from_task_asset_id": 101,
        "final_task_asset_ids": [101],
        "reference_file_ref_ids": [],
        "evidence_event_ids": ["task_event_log:event-1"],
        "confidence": "proposed_review",
        "review_policy_ids": ["explicit_event_replay", "delivery_source_alias"],
        "confirmed_by": 0,
        "confirmed_at": review.ZERO_TIME,
        "confirmation_note": "",
        "reason": "candidate",
        "created_by": 9,
        "created_at": "2026-07-22T10:00:00Z",
        "submitted_at": "2026-07-22T10:00:00Z",
        "finalized_at": "2026-07-22T10:01:00Z",
    }
    revision["manifest_row_hash"] = review.canonical_revision_hash(revision)
    return {
        "version": 2,
        "resources": [
            {
                "task_id": 7,
                "scope_kind": "task",
                "scope_ref_id": 0,
                "history": [revision],
                "working_revision_no": 1,
                "finalized_revision_no": 1,
            }
        ],
        "planning_tasks": [],
        "task_state_decisions": [],
        "organization_mappings": [],
        "access_decisions": [],
        "asset_recoveries": [],
    }


class BuildApprovedDecisionsTest(unittest.TestCase):
    def setUp(self) -> None:
        self.directory = tempfile.TemporaryDirectory()
        self.root = pathlib.Path(self.directory.name)
        self.candidate = self.root / "candidate.json"
        self.ledger = self.root / "ledger.json"
        self.template = self.root / "template.json"
        self.prepare_evidence = self.root / "prepare.json"
        self.admin = self.root / "admin.json"
        self.mapping = self.root / "reviewed.json"
        self.apply_evidence = self.root / "apply.json"
        self.output = self.root / "confirmed.json"

        self.candidate.write_text(
            json.dumps(proposed_mapping(), ensure_ascii=False, separators=(",", ":")),
            encoding="utf-8",
        )
        review.prepare(
            self.candidate,
            self.ledger,
            self.template,
            self.prepare_evidence,
        )
        decision = json.loads(self.template.read_text(encoding="utf-8"))
        decision.update(
            {
                "decision": "approve",
                "reviewer_id": 1,
                "reviewed_at": "2026-07-24T02:43:42Z",
                "note": "Approved only for the exact frozen candidate and Clone B.",
                "approved_policies": [
                    "delivery_source_alias",
                    "explicit_event_replay",
                ],
            }
        )
        self.admin.write_text(
            json.dumps(decision, ensure_ascii=False, indent=2) + "\n",
            encoding="utf-8",
        )
        review.apply_review(
            self.candidate,
            self.ledger,
            self.admin,
            self.mapping,
            self.apply_evidence,
        )

    def tearDown(self) -> None:
        self.directory.cleanup()

    def args(self, **overrides) -> argparse.Namespace:
        values = {
            "review_template": str(self.template),
            "template_sha256": digest(self.template),
            "admin_decision": str(self.admin),
            "admin_decision_sha256": digest(self.admin),
            "apply_evidence": str(self.apply_evidence),
            "apply_evidence_sha256": digest(self.apply_evidence),
            "reviewed_mapping": str(self.mapping),
            "mapping_sha256": digest(self.mapping),
            "candidate_sha256": digest(self.candidate),
            "cohort_digest": json.loads(self.template.read_text(encoding="utf-8"))[
                "cohort_digest"
            ],
            "output": str(self.output),
        }
        values.update(overrides)
        return argparse.Namespace(**values)

    def test_builds_canonical_clone_b_only_confirmation(self) -> None:
        document = approved.build(self.args())
        written = self.output.read_bytes()
        self.assertEqual(
            written,
            (approved.canonical_json(document) + "\n").encode("utf-8"),
        )
        self.assertEqual(document["decision"], "confirmed")
        self.assertEqual(document["authorized_scope"], "clone_b_validation_only")
        self.assertIs(document["production_write_authorized"], False)
        self.assertEqual(document["reviewer_id"], 1)
        self.assertEqual(document["promoted_review_count"], 1)
        self.assertEqual(document["confirmed_mapping_row_count"], 1)
        self.assertEqual(
            document["approved_policies"],
            ["explicit_event_replay", "delivery_source_alias"],
        )
        self.assertEqual(document["review_template_sha256"], digest(self.template))
        self.assertEqual(document["admin_decision_sha256"], digest(self.admin))
        self.assertEqual(
            document["review_apply_evidence_sha256"],
            digest(self.apply_evidence),
        )
        self.assertEqual(document["reviewed_mapping_sha256"], digest(self.mapping))

    def test_refuses_to_overwrite_existing_output(self) -> None:
        self.output.write_text("owner data", encoding="utf-8")
        with self.assertRaisesRegex(FileExistsError, "refusing to overwrite"):
            approved.build(self.args())
        self.assertEqual(self.output.read_text(encoding="utf-8"), "owner data")

    def test_rejects_wrong_expected_input_hash(self) -> None:
        with self.assertRaisesRegex(ValueError, "admin decision sha256 mismatch"):
            approved.build(self.args(admin_decision_sha256="0" * 64))
        self.assertFalse(self.output.exists())

    def test_rejects_unconfirmed_mapping_even_when_hashes_are_updated(self) -> None:
        value = json.loads(self.mapping.read_text(encoding="utf-8"))
        revision = value["resources"][0]["history"][0]
        revision["confidence"] = "proposed_review"
        revision["confirmed_by"] = 0
        revision["confirmed_at"] = review.ZERO_TIME
        revision["confirmation_note"] = ""
        revision["manifest_row_hash"] = review.canonical_revision_hash(revision)
        self.mapping.write_text(
            json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
            + "\n",
            encoding="utf-8",
        )
        evidence = json.loads(self.apply_evidence.read_text(encoding="utf-8"))
        evidence["reviewed_mapping_sha256"] = digest(self.mapping)
        self.apply_evidence.write_text(
            json.dumps(evidence, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
            + "\n",
            encoding="utf-8",
        )
        with self.assertRaisesRegex(ValueError, "reviewed mapping|confirmed_auto"):
            approved.build(
                self.args(
                    mapping_sha256=digest(self.mapping),
                    apply_evidence_sha256=digest(self.apply_evidence),
                )
            )
        self.assertFalse(self.output.exists())

    def test_rejects_apply_evidence_with_remaining_blocker(self) -> None:
        evidence = json.loads(self.apply_evidence.read_text(encoding="utf-8"))
        evidence["remaining_revision_confidence_counts"] = {"hard_blocked": 1}
        self.apply_evidence.write_text(
            json.dumps(evidence, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
            + "\n",
            encoding="utf-8",
        )
        with self.assertRaisesRegex(ValueError, "unconfirmed confidence states"):
            approved.build(
                self.args(apply_evidence_sha256=digest(self.apply_evidence))
            )
        self.assertFalse(self.output.exists())


if __name__ == "__main__":
    unittest.main()
