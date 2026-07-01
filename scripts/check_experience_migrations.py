#!/usr/bin/env python3
"""Static guard for experience Phase 2 migrations.

The guard intentionally checks objective rules only:
- no foreign keys / references in experience side-channel migrations
- high-growth tables define primary keys and idempotency unique keys where needed
- explicitly required indexes from the Phase 2 plan are present
"""

from __future__ import annotations

import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
MIGRATION = ROOT / "db" / "migrations" / "100_v1_14_experience_phase2_closed_loop.sql"


REQUIRED_SNIPPETS = [
    "idx_experience_outbox_target_time",
    "idx_experience_outbox_observed",
    "idx_experience_events_target_time",
    "idx_experience_events_source_action_time",
    "idx_experience_events_observed",
    "idx_ai_suggestion_events_stable_time",
    "idx_ai_suggestion_events_attribution_time",
    "idx_tasks_experience_observer_updated",
    "idx_audit_records_experience_observer",
    "idx_task_module_events_experience_observer",
    "idx_task_assets_experience_observer_created",
    "idx_task_assets_experience_observer_approved",
    "idx_task_assets_experience_observer_rejected",
    "idx_task_assets_experience_observer_archived",
    "idx_task_assets_experience_observer_cleaned",
    "idx_task_details_experience_observer_updated",
    "idx_task_sku_items_experience_observer_updated",
    "uq_experience_behavior_event_key",
    "uq_experience_observed_entity",
    "uq_experience_attribution_candidate",
    "uq_experience_micro_question_answer",
    "PRIMARY KEY (worker_name, source_name)",
    "PRIMARY KEY (limit_key)",
]


def strip_sql_comments(sql: str) -> str:
    sql = re.sub(r"/\*.*?\*/", "", sql, flags=re.S)
    return "\n".join(line for line in sql.splitlines() if not line.strip().startswith("--"))


def main() -> int:
    if not MIGRATION.exists():
        print(f"missing migration: {MIGRATION}", file=sys.stderr)
        return 1

    raw = MIGRATION.read_text(encoding="utf-8")
    sql = strip_sql_comments(raw)
    lowered = sql.lower()

    failures: list[str] = []
    if re.search(r"\bforeign\s+key\b|\breferences\b", lowered):
        failures.append("experience migration must not declare foreign keys or REFERENCES")
    if "add column if not exists" in lowered:
        failures.append("experience migration must not use ADD COLUMN IF NOT EXISTS; deployment MySQL rejects it")
    if "create index if not exists" in lowered:
        failures.append("experience migration must not use CREATE INDEX IF NOT EXISTS; use ALTER TABLE ADD KEY for deployment")

    for snippet in REQUIRED_SNIPPETS:
        if snippet.lower() not in lowered:
            failures.append(f"missing required migration guard snippet: {snippet}")

    for table in [
        "experience_behavior_events",
        "experience_observed_entity_states",
        "experience_attributions",
        "experience_micro_question_answers",
        "experience_rate_limits",
    ]:
        pattern = rf"create\s+table\s+if\s+not\s+exists\s+{table}\s*\((.*?)\);"
        match = re.search(pattern, lowered, flags=re.S)
        if not match:
            failures.append(f"missing table definition: {table}")
            continue
        body = match.group(1)
        if "primary key" not in body:
            failures.append(f"{table} must define a primary key")

    if failures:
        for failure in failures:
            print(f"FAIL: {failure}", file=sys.stderr)
        return 1

    print("PASS experience migration guard")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
