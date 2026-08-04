#!/usr/bin/env python3
"""Create a fail-closed, run-scoped V8 A/B evidence ledger."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import pathlib


def write_json(path: pathlib.Path, value: object) -> None:
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-dir", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--mode", required=True)
    parser.add_argument("--source-db", required=True)
    parser.add_argument("--target-db", required=True)
    parser.add_argument("--git-head", required=True)
    parser.add_argument("--worktree-hash", required=True)
    parser.add_argument("--openapi-hash", required=True)
    parser.add_argument("--snapshot-hash", default="")
    parser.add_argument("--review-manifest-hash", default="")
    parser.add_argument("--comparator-hash", default="")
    parser.add_argument("--build-api-oracle-hash", default="")
    args = parser.parse_args()

    run_dir = pathlib.Path(args.run_dir)
    run_dir.mkdir(parents=True, exist_ok=True)
    now = dt.datetime.now(dt.timezone.utc).isoformat()
    environment = {
        "run_id": args.run_id,
        "created_at": now,
        "mode": args.mode,
        "source_database": args.source_db,
        "target_database": args.target_db,
        "candidate": {"git_head": args.git_head, "worktree_diff_sha256": args.worktree_hash},
        "openapi_sha256": args.openapi_hash,
        "external_backend_image_digest": None,
        "dev_plus_backend_image_digest": None,
        "external_frontend_manifest_sha256": None,
        "dev_plus_frontend_manifest_sha256": None,
        "configuration_sha256": None,
        "migration_mapping_sha256": None,
        "snapshot_sha256": args.snapshot_hash or None,
        "review_manifest_sha256": args.review_manifest_hash or None,
        "api_oracle_sha256": None,
        "api_rules_sha256": None,
        "comparator_sha256": args.comparator_hash or None,
        "build_api_oracle_sha256": args.build_api_oracle_hash or None,
    }
    write_json(run_dir / "environment_manifest.json", environment)

    gates = {
        f"G{index}": {
            "status": "BLOCKED",
            "reason": "not evaluated from complete raw evidence",
            "evidence": [],
            "executor": None,
            "reviewer": None,
        }
        for index in range(11)
    }
    write_json(run_dir / "gate_report.json", {"run_id": args.run_id, "decision": "NO-GO", "gates": gates})
    ledger = {
        "timestamp": now,
        "gate": "G0",
        "claim": "candidate reproducibility is not yet proven",
        "status": "BLOCKED",
        "evidence": ["environment_manifest.json"],
        "executor": "Release Commander",
        "reviewer": None,
        "boundary": "initialization only; no database or network action",
        "uncertainty": "image, frontend, configuration and migration hashes are absent",
        "blockers": ["G0-MISSING-REPRODUCIBLE-ARTIFACTS"],
    }
    (run_dir / "decision_ledger.jsonl").write_text(
        json.dumps(ledger, ensure_ascii=False, sort_keys=True) + "\n", encoding="utf-8"
    )
    (run_dir / "go-no-go.md").write_text(
        f"# V8 A/B decision: NO-GO\n\nRun: `{args.run_id}`\n\n"
        "This run is fail-closed. All gates remain blocked until raw evidence and independent review are attached.\n",
        encoding="utf-8",
    )


if __name__ == "__main__":
    main()
