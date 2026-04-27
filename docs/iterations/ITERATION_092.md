# ITERATION_092

Title: Historical task read-model `500` audit, DB backup, retained-task repair, explicit test-data cleanup, and live re-verification on `v0.8`

Date: 2026-03-23
Model: GPT-5 Codex

## 1. Background and goal
- The org/permission line was explicitly frozen at the minimum usable closure.
- The next battlefront for this round was fixed before execution:
  - historical task read-model `500`
  - database dirty-data cleanup
  - live backup before any mutation
  - live re-verification after cleanup
- This round was explicitly not allowed to reopen:
  - upload-chain fixes
  - multipart/reference fixes
  - org tree / ABAC / row-level work

## 2. System-wide audit instead of single-task guessing
- Live current-state scan iterated across the full live `tasks` set and called:
  - `GET /v1/tasks/{id}`
  - `GET /v1/tasks/{id}/product-info`
  - `GET /v1/tasks/{id}/cost-info`
- Current active live result before cleanup:
  - `86` tasks scanned
  - no current active `500` on those three read endpoints
- Historical log audit still confirmed earlier `500` clusters from live logs:
  - `GET /v1/tasks/84`
  - `GET /v1/tasks/84/product-info`
  - `GET /v1/tasks/84/cost-info`
  - repeated `GET /v1/tasks/136~141/cost-info`
- Conclusion:
  - the live server code had already removed the active read-path crash
  - but the live DB still contained historical compatibility residue and explicit test/demo residue that needed to be cleaned deliberately

## 3. Dirty-data pattern classification
- Pattern A: retained historical compatibility tasks
  - `source_mode=existing_product`
  - `product_id IS NULL`
  - `product_selection_snapshot_json` still present
  - these are not automatically bad data; they are historical compatibility state
- Pattern B: explicit test/demo/acceptance/case tasks
  - task content itself contained clear markers in `sku_code` or `product_name_snapshot`
  - examples included `accept`, `demo`, `case`, `test`
  - these are safe delete candidates when backed up first
- Pattern C: structural integrity
  - no missing `task_details`
  - no `task_details` orphaned from `tasks`
  - no `task_assets` orphaned from `tasks`
  - no `design_assets` orphaned from `tasks`
- Pattern D: JSON payload integrity
  - audited JSON fields were valid in the current live snapshot:
    - `reference_images_json`
    - `reference_file_refs_json`
    - `matched_mapping_rule_json`
    - `product_selection_snapshot_json`
    - `risk_flags_json`
    - `last_filing_payload_json`

## 4. Keep / fix / delete boundary
- Keep boundary:
  - business-like historical tasks with no explicit test/demo/case marker
  - snapshot-based existing-product tasks that still represent valid historical context
- Fix boundary:
  - retained existing-product tasks where exact `products.sku_code = tasks.sku_code` now exists
  - these should be normalized by backfilling `tasks.product_id`
- Delete boundary:
  - only explicit marker tasks whose task content itself showed demo/acceptance/case/test semantics
  - test-account ownership alone was not treated as enough evidence to delete

## 5. Backup before mutation
- Backup path created before mutation:
  - `/root/ecommerce_ai/backups/20260323T120120Z_historical_task_cleanup_v091`
- Backup files:
  - `jst_erp_full_before.sql.gz`
  - `key_tables_before.sql.gz`
  - `candidate_boundaries_before.tsv`
- This round therefore kept a rollback boundary for:
  - the full database
  - the main task-related tables
  - the exact candidate task subset

## 6. Actual repair and cleanup actions
- Local traceability script added:
  - `scripts/historical_task_cleanup_v091.sql`
- Live retained-task repair:
  - `12` historical existing-product tasks had `tasks.product_id` backfilled from exact `products.sku_code` match
  - repaired examples:
    - `106`
    - `114`
    - `115`
    - `122`
    - `128`
    - `130`
    - `131`
    - `134`
    - `137`
    - `139`
    - `142`
    - `144`
- Live explicit test-data cleanup:
  - `20` explicit marker tasks were deleted with their task-scoped dependent rows
  - deleted IDs:
    - `51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136`

## 7. Consistency checks after cleanup
- Live counts after mutation:
  - total tasks: `86 -> 66`
  - explicit marker task count: `20 -> 0`
  - `existing_product_missing_product_id -> 0`
- Structural checks after mutation:
  - `missing_detail = 0`
  - `orphan_detail = 0`
  - `orphan_task_assets = 0`
  - `orphan_design_assets = 0`
  - `asset_storage_refs(owner_type=task_asset)` rechecked against `task_assets` also stayed `0` orphan

## 8. Live regression verification
- Full remaining task-set live scan after cleanup:
  - `GET /v1/tasks/{id}` = no `500`
  - `GET /v1/tasks/{id}/product-info` = no `500`
  - `GET /v1/tasks/{id}/cost-info` = no `500`
- Retained repaired sample verification:
  - tasks `106, 114, 137, 139, 142, 144`
  - `GET /v1/tasks/{id}` and `GET /v1/tasks/{id}/product-info` now agree on `product_id`
- Deleted sample verification:
  - deleted explicit marker task IDs now return `404`

## 9. Local test result
- Since no live service code path was changed in this round, there was no new server binary release.
- The round still re-ran local task-related regression checks:
  - `go test ./service -run "TaskDetail|TaskReadModel|TaskPRD|Filing"` passed
  - `go test ./transport/... -run "Task"` passed

## 10. Current boundary after this round
- The current historical task `500` lane is closed by:
  - historical log evidence
  - DB-wide audit
  - backup evidence
  - retained-task repair
  - explicit marker cleanup
  - full remaining task-set live verification
- This was a constrained cleanup round, not a general-purpose data purge.
- Upload-chain and org/permission closures remain unchanged and were not modified.

## 11. Next TODO
- Broader regression around non-explicit legacy test users
- Additional historical normalization only when the business/trace evidence is strong enough
- Continued watch on new task reads so future dirty data does not reintroduce `500`
