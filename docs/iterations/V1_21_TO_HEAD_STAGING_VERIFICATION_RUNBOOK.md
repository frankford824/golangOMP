# v1.21-prod -> HEAD Staging Verification Runbook

Date: 2026-04-30
Scope: external verification required after local predeploy PASS.

## Rule

Do not run this against production first. Use a staging clone with
production-like data volume.

This runbook permits writes only to the staging clone and to an isolated
side-by-side candidate release. Production cutover remains blocked until every
gate below is recorded as PASS.

## Inputs

To be confirmed before execution:

- Staging clone host, port, database, and credentials.
- Candidate backend base URL.
- Test user credentials for SuperAdmin, task creator, designer, audit role,
  and pool claimant.
- ERP/JST bridge behavior for staging: real upstream, mocked upstream, or
  read-only sandbox.
- Frontend build/version that will be tested against this backend contract.

## Gate 0 - Local Baseline

Run from repository root:

```bash
./scripts/agent-check.sh
git status --short --untracked-files=all
git log --oneline -5
```

Required result:

- `agent-check` PASS.
- Working tree clean.
- Target commit recorded.

Current local evidence before this runbook:

- `cfcc5bf docs(governance): record v1.21 predeploy local verification`
- `contract_audit`: `drift=0 missing_in_openapi=0 missing_in_code=0`

## Gate 1 - Staging Clone Read-Only Precheck

Run read-only SQL against the staging clone before applying migrations:

```sql
SELECT VERSION() AS mysql_version, @@version_comment AS version_comment, @@sql_mode AS sql_mode;
SHOW VARIABLES LIKE 'collation_server';
SHOW VARIABLES LIKE 'character_set_server';

SELECT COUNT(*) AS task_rows FROM tasks;
SELECT COUNT(*) AS task_sku_item_rows FROM task_sku_items;
SELECT COUNT(*) AS urgent_priority_rows FROM tasks WHERE priority = 'urgent';

SELECT COUNT(*) AS existing_search_document_table
FROM information_schema.tables
WHERE table_schema = DATABASE()
  AND table_name = 'task_search_documents';

SELECT column_name
FROM information_schema.columns
WHERE table_schema = DATABASE()
  AND table_name = 'task_sku_items'
  AND column_name IN (
    'filing_status',
    'erp_sync_status',
    'erp_sync_required',
    'erp_sync_version',
    'last_filed_at',
    'filing_error_message'
  )
ORDER BY column_name;
```

Required result:

- MySQL supports `utf8mb4_0900_ai_ci`.
- `urgent_priority_rows = 0`.
- If the six `task_sku_items` projection columns are absent, migration 070 is
  mandatory before candidate traffic.
- If `task_search_documents` is absent, migration 069 should create and
  backfill it.

Abort if:

- The environment is production, not a staging clone.
- `urgent_priority_rows > 0`.
- MySQL does not support the collation used by migration 069.
- The table size makes migration 070 unsafe without an agreed maintenance
  window or online DDL plan.

## Gate 2 - Migration Dry-Run On Staging Clone

Apply only to the staging clone:

```bash
mysql "$STAGING_CLONE_DSN" < db/migrations/069_v1_1_task_search_documents.sql
mysql "$STAGING_CLONE_DSN" < db/migrations/070_v1_1_task_sku_item_filing_projection.sql
```

If the local `mysql` client does not accept a DSN string, use the equivalent
host/user/password/database flags from the staging clone credentials. Do not
print secrets into logs.

Record:

- Start and end timestamp for each migration.
- Exit status.
- Row count of `tasks` and `task_sku_items`.
- Any lock wait, timeout, or replication-lag observation.

Postcheck SQL:

```sql
SELECT COUNT(*) AS task_rows FROM tasks;
SELECT COUNT(*) AS search_document_rows FROM task_search_documents;

SELECT column_name
FROM information_schema.columns
WHERE table_schema = DATABASE()
  AND table_name = 'task_sku_items'
  AND column_name IN (
    'filing_status',
    'erp_sync_status',
    'erp_sync_required',
    'erp_sync_version',
    'last_filed_at',
    'filing_error_message'
  )
ORDER BY column_name;

SELECT index_name
FROM information_schema.statistics
WHERE table_schema = DATABASE()
  AND table_name = 'task_sku_items'
  AND index_name IN (
    'idx_task_sku_items_filing_status',
    'idx_task_sku_items_erp_sync_required'
  )
ORDER BY index_name;

SELECT index_name
FROM information_schema.statistics
WHERE table_schema = DATABASE()
  AND table_name = 'task_search_documents'
  AND index_name = 'ft_task_search_text';
```

Required result:

- `search_document_rows = task_rows`.
- Six projection columns exist on `task_sku_items`.
- Both SKU projection indexes exist.
- `ft_task_search_text` exists.

Abort if any migration fails, hangs, or produces partial schema state that has
not been explicitly analyzed and remediated.

## Gate 3 - Side-By-Side Candidate Deploy

Use side-by-side validation only. Do not cut over live MAIN at this gate.

Reference command shape:

```bash
bash ./deploy/deploy.sh \
  --version <candidate-version> \
  --parallel \
  --release-note "v1.21-to-head staging validation"
```

Required result:

- Live MAIN remains untouched.
- Candidate MAIN starts on the isolated parallel port.
- Candidate points to the staging clone whose migrations passed Gate 2.
- Runtime verify script passes:

```bash
bash /root/ecommerce_ai/releases/<candidate-version>/deploy/verify-runtime.sh \
  --base-url http://127.0.0.1:<candidate-port>
```

Abort if candidate points to production DB, stops live MAIN, or cannot start
cleanly.

## Gate 4 - Backend Smoke

Run against the candidate base URL and record HTTP status plus response snippets
without printing tokens.

Required smoke set:

1. Auth and identity:
   - `POST /v1/auth/login`
   - `GET /v1/me`
2. Task list/detail:
   - `GET /v1/tasks?page=1&page_size=20`
   - `GET /v1/tasks/{id}/detail`
3. ERP i_id selector:
   - `GET /v1/erp/iids?page=1&page_size=20`
4. Task creation/product info:
   - create or update a new-product task with `i_id` / `product_i_id`
   - verify returned detail preserves the selected i_id
5. Batch Excel:
   - download `GET /v1/tasks/batch-create/template.xlsx?task_type=new_product_development`
   - parse a workbook containing row `商品编码`
   - parse a workbook containing an embedded row image when staging storage is available
6. ERP filing:
   - verify success path updates task-level and per-SKU filing projection
   - verify failed/retry visibility when staging bridge can safely simulate failure
7. Retouch:
   - submit retouch design work
   - verify the retouch task completes without entering audit
8. Pool and assignment:
   - `GET /v1/tasks/pool?page=1&page_size=20`
   - verify response envelope contains `data` and `pagination`
   - attempt a concurrent/double claim and verify conflict behavior
9. Notifications:
   - trigger an audit-pending transition
   - verify `task_pending_audit` appears for the expected user
10. Assets:
   - `GET /v1/assets/{asset_id}`
   - `GET /v1/assets/{asset_id}/download`
   - `GET /v1/assets/{asset_id}/preview`
11. Search:
   - `GET /v1/search?q=<task_no-or-product>`
   - verify search works after `task_search_documents` exists

Required result:

- No unexpected 5xx.
- Expected 4xx responses expose meaningful `error.code` / `deny_code`.
- Frontend-visible response shapes match `docs/frontend/*.md`.

## Gate 5 - Frontend Contract Acknowledgement

Frontend owner must explicitly acknowledge these contract deltas before
production cutover:

- Task priority enum is `low`, `normal`, `high`, `critical`; `urgent` is not
  accepted.
- New-product and purchase flows use `i_id` / `product_i_id`.
- `GET /v1/erp/iids` is the canonical selector for ERP i_id.
- `/v1/tasks/pool` returns pagination envelope, not a raw list.
- `task_pending_audit` is a valid notification type.
- Global asset docs use `{asset_id}` as path parameter name.
- New-product batch Excel only requires `产品名称` and `设计要求`; `商品编码`
  maps to row `product_i_id`.

Required result:

- Frontend build under test is named and recorded.
- Any required frontend patch is complete before backend cutover.

## Gate 6 - Cutover Decision

Production cutover is allowed only if all previous gates are PASS.

Before cutover, record:

- Candidate commit.
- Migration 069/070 staging duration and row counts.
- Smoke result table.
- Frontend acknowledgement.
- Backup plan and rollback owner.

If any item is Unknown or To be confirmed, do not cut over.

## Evidence Template

```text
Target commit:
Candidate version:
Staging DB:
Gate 0 local baseline: PASS/FAIL
Gate 1 read-only DB precheck: PASS/FAIL
Gate 2 migration dry-run: PASS/FAIL
Gate 3 side-by-side deploy: PASS/FAIL
Gate 4 backend smoke: PASS/FAIL
Gate 5 frontend acknowledgement: PASS/FAIL
Cutover decision: HOLD/GO
Notes:
```
