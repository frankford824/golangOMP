# V1 R2 Report

> Scope: production-aligned R2 data layer on `jst_erp` via `ssh jst_ecs`.
> Docker: not used.
> Rollback: dry-run only; no production rollback executed.

## Backfill Stats

First production run:

| Phase | processed | generated | warnings | errors | duration |
| --- | ---: | ---: | ---: | ---: | ---: |
| Phase A | 98 | 298 | 0 | 0 | 2.118s |
| Phase B | 265 | 267 | 0 | 0 | 1.022s |
| Phase C | 154 | 194 | 0 | 0 | 758ms |
| Phase D | 0 | 0 | 0 | 0 | 0s |
| Phase E | 0 | 0 | 0 | 0 | 0s |

Second idempotency run:

| Phase | processed | generated | warnings | errors | duration |
| --- | ---: | ---: | ---: | ---: | ---: |
| Phase A | 98 | 0 | 0 | 0 | 601ms |
| Phase B | 265 | 0 | 0 | 0 | 122ms |
| Phase C | 154 | 0 | 0 | 0 | 95ms |
| Phase D | 0 | 0 | 0 | 0 | 0s |
| Phase E | 0 | 0 | 0 | 0 | 0s |

Final production checks:

| Check | Result |
| --- | --- |
| `task_modules` | 300 |
| `task_module_events` | 300 |
| `reference_file_refs` | 194 |
| `task_drafts` / `notifications` / `org_move_requests` / `task_customization_orders` | 0 / 0 / 0 / 0 |
| unknown `task_assets.asset_type` | 0 |
| invalid `tasks.priority` | 0 |
| empty `task_assets.source_module_key` | 0 |
| source module distribution | `basic_info=9`, `customization=5`, `design=251` |
| `basic_info` coverage | 98 / 98 tasks |
| `PendingAuditA` audit coverage | 7 / 7 tasks |

## Performance

Current production size during execution was 98 tasks / 265 task assets. Phase A~E total duration was 3.899s on the first run and 818ms on the idempotency run, under the 10s production threshold.

## Rollback Verification

Dry-run only. Output saved to `docs/iterations/r2_rollback_dry_run.log`.

Rollback order printed as expected: 068, 067, 066, 065, 064, 063, 062, 061, 060, 059. The dry-run listed non-empty SQL for every migration, including `DROP TABLE` for the seven new tables and `DROP CHECK chk_tasks_priority_v1` / `DROP INDEX idx_tasks_priority_created` for 067.

## Smoke Results

Integration smoke was compiled with `-tags=integration` and executed on `jst_ecs` with `MYSQL_DSN` built from `/root/ecommerce_ai/shared/main.env`.

Result: PASS.

Assertions covered:

1. `basic_info` module exists for every task.
2. `PendingAuditA` tasks have an `audit` module.
3. `task_assets.source_module_key` is non-empty for all assets.
4. `reference_file_refs` count is at least JSON coverage threshold.
5. Re-running backfill does not change `task_modules`, `task_module_events`, `reference_file_refs`, or `task_customization_orders` row counts.

## Probe Diff

Pre-probe saved to `docs/iterations/r2_probe_pre.log`; post-probe saved to `docs/iterations/r2_probe_post.log`.

Expected R2 change:

- R2 target tables changed from absent to present: `task_modules`, `task_module_events`, `reference_file_refs`, `task_drafts`, `notifications`, `org_move_requests`, `task_customization_orders`.

Enum hard gates remained valid:

- `asset_type` stayed within `{reference, source, delivery, design_thumb, preview}`.
- `priority` stayed within `{low, normal, high, critical}`.
- Phantom columns stayed absent: `task_assets.flow_stage`, `tasks.is_urgent`, `tasks.task_priority`.

Live production traffic drift occurred during the execution window, so this probe cannot be used as evidence of zero non-target table activity:

| Metric | Pre | Post |
| --- | ---: | ---: |
| tasks | 97 | 98 |
| task_details | 97 | 98 |
| task_assets | 265 | 265 |
| task_sku_items | 80 | 84 |
| asset_storage_refs | 446 | 451 |
| users | 95 | 95 |
| `priority=high` | 21 | 22 |
| `new_product_development` tasks | 59 | 60 |

R2 backfill itself did not update historical `priority` or `asset_type` fields; both hard-gate validation queries returned 0 invalid rows.

## Backup Evidence

| Backup | Path | Size bytes | sha256 |
| --- | --- | ---: | --- |
| pre-forward | `/root/ecommerce_ai/backups/20260424T024243Z_r2_pre_forward.sql.gz` | 182834716 | `97e4465aeb071a543f8c0dbf7a4a4094ebc842c976de3a94030301543ae836bb` |
| pre-backfill | `/root/ecommerce_ai/backups/20260424T024501Z_r2_pre_backfill.sql.gz` | 182836873 | `3abe1ca62fd9190d1f2ad5d3ca02d563883dd626523cc1459480bf670d7ee561` |

Both backup files were validated with `gzip -t` and are greater than 1MB.

## Implementation Notes

- 059~068 were added with `-- ROLLBACK-BEGIN` / `-- ROLLBACK-END` blocks.
- 062 uses table default `utf8mb4_unicode_ci`; its `ref_id` column has column-level `utf8mb4_0900_ai_ci` to match production `asset_storage_refs.ref_id` and satisfy the required FK.
- `go build ./...` passed locally using `/home/wsfwk/go/bin/go`.
- Tool package tests passed; local integration smoke skips without `MYSQL_DSN`, while the server execution passed with production DSN from `main.env`.
