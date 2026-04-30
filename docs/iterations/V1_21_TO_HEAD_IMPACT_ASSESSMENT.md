# v1.21-prod → HEAD Functional Impact Assessment

Date: 2026-04-30
Base: `v1.21-prod`
Target: `3171d5f` (`docs(governance): refresh bootstrap after V1.4 phase 1`)

## Executive Verdict

Current HEAD must be treated as a functional release candidate, not as a
governance-only patch over online v1.21. The V1.4 governance commits are
non-business, but `v1.21-prod..HEAD` includes schema changes, OpenAPI contract
changes, task workflow behavior changes, permission/scope changes, new
notification behavior, search behavior changes, batch Excel changes, and ERP
filing changes.

Do not deploy HEAD over v1.21 without a migration plan, frontend contract
acknowledgement, and targeted smoke/regression verification.

## Diff Scale

- Commit window: 90 commits after `v1.21-prod`.
- Files changed: 271.
- Shortstat: 53980 insertions, 2295 deletions.
- Tests in the window: 11 added `_test.go` files and 25 modified `_test.go`
  files.
- Current full gate: green.
- Current contract audit: `total_paths=234 clean=169 drift=0 unmapped=0
  known_gap=65 missing_in_openapi=0 missing_in_code=0`.

## Contract / Route Impact

OpenAPI operations changed materially:

- Old OpenAPI operations: 228.
- New OpenAPI operations: 236.
- Added operations: 12.
- Removed operations: 4.
- Changed common operations: 73.

Added OpenAPI operations:

- `GET /v1/erp/iids`
- `GET /v1/assets/{asset_id}`
- `GET /v1/assets/{asset_id}/download`
- `GET /v1/assets/{asset_id}/preview`
- `DELETE /v1/assets/{asset_id}`
- `GET /v1/rule-templates`
- `GET /v1/rule-templates/{type}`
- `PUT /v1/rule-templates/{type}`
- `POST /v1/incidents/{id}/assign`
- `POST /v1/incidents/{id}/resolve`
- `POST /v1/sku/preview_code`
- `PUT /v1/policies/{id}`

Removed OpenAPI operations:

- `GET /v1/assets/{id}`
- `GET /v1/assets/{id}/download`
- `GET /v1/assets/{id}/preview`
- `DELETE /v1/assets/{id}`

The removed asset operations are path-parameter-name replacements
(`id` → `asset_id`) for the same URL shape; generated clients that key on
parameter names will see this as a contract change.

Runtime route comparison with the current `contract_audit` shows one mounted
path added: `GET /v1/erp/iids`. No mounted runtime path was removed. The same
audit also shows that the current branch closes the old v1.21 audit drift
window: old snapshot under the current audit engine reports `drift=57
unmapped=1 known_gap=60`, while HEAD reports `drift=0 unmapped=0
known_gap=65`.

## Database / Migration Impact

Two migrations were added:

- `069_v1_1_task_search_documents.sql`
  - Creates `task_search_documents`.
  - Adds a FULLTEXT search read model with backfill.
  - Uses `utf8mb4_0900_ai_ci`, so MySQL 8 compatibility is assumed.
  - Search code falls back when the table does not exist, but once the table
    exists search ordering and matching use the read model.

- `070_v1_1_task_sku_item_filing_projection.sql`
  - Alters `task_sku_items`.
  - Adds `filing_status`, `erp_sync_status`, `erp_sync_required`,
    `erp_sync_version`, `last_filed_at`, and `filing_error_message`.
  - Adds two indexes.
  - This is mandatory before running HEAD for code paths that read SKU items;
    `repo/mysql/task.go` selects these columns in SKU item reads.

Release risk: deploying HEAD without migration 070 can break batch SKU reads
with unknown-column SQL errors. Migration 070 is an `ALTER TABLE` and should be
timed or tested against production-size `task_sku_items`.

## Functional Impact By Area

### Task Priority

`urgent` was removed from the backend enum and OpenAPI. The current accepted
values are `low`, `normal`, `high`, and `critical`. Clients still sending
`urgent` to task creation now receive `400 task_priority_invalid`.

Production data risk: whether online v1.21 contains rows or clients still using
`urgent` is Unknown from local repository state. Confirm before deployment.

### Task Creation / Product Info / ERP i_id

Task create and product-info APIs now accept/return `i_id` /
`product_i_id`. Product info patch can update product name, i_id,
`design_requirement`, `change_request`, and can trigger filing.

`GET /v1/erp/iids` was added as the canonical selector for product family/style
i_id values. New-product and purchase flows can use it before task creation or
batch Excel parsing.

Impact: frontend create/edit forms need to stop treating product code/category
fields as the only ERP identity; i_id now participates in validation and ERP
filing.

### Batch Excel

New-product batch Excel changed materially:

- Required columns were reduced to `产品名称` and `设计要求`.
- `商品编码` maps to `batch_items[].product_i_id`.
- Row-anchored embedded images are extracted, uploaded, and returned as
  row-level `reference_file_refs`.
- Parse validates supplied i_id values against ERP i_id options.
- Purchase-task Excel also supports `商品编码` and row reference images.

Impact: existing frontend expectations for the template columns and parse
payload must align with regenerated docs. Older clients expecting mandatory
`产品简称` / `类目编码` / `材料模式` for new-product Excel will see changed
validation semantics.

### ERP Filing

ERP filing changed from one task-level payload to potentially multiple payloads
for batch new-product tasks. Per-SKU filing projections were added and synced
back to `task_sku_items`.

Other filing changes:

- New-product and purchase missing-field rules were relaxed.
- `i_id` is preferred when building ERP payloads.
- Retouch tasks are marked `not_filed` / `erp_sync_required=false`.
- `sync_erp_on_create` can trigger create-time sync.

Impact: ERP filing behavior and retry semantics differ from v1.21. Batch
new-product filing now depends on per-row `product_i_id`.

### Retouch Workflow

`retouch_task` now requires design/retouch work but does not require audit.
Submitting retouch design completes the retouch module and completes the task
instead of entering audit/warehouse.

Impact: this is a behavior change for task lifecycle, detail aggregation, and
filing. Any frontend or operations workflow that expected retouch tasks to
enter audit after design submit must be updated.

### Task Pool / Claim / Assignment

Pool and claim behavior changed:

- `/v1/tasks/pool` now responds with pagination metadata instead of a raw list.
- Pool entries include `updated_at`.
- Supports `page`, `page_size`, and `sort`.
- Pool visibility maps canonical pool codes to real team names/departments.
- Claim and assignment use CAS-style conflict protection.
- Claiming a module can update task designer/current handler/status.
- Assignment syncs design/retouch module state and emits assignment
  notifications.

Impact: frontend pool consumers must accept the new envelope and pagination.
Operationally, duplicate/self-claim conflict behavior is stricter and should be
smoked under concurrent claim scenarios.

### Permissions / Visibility

Permission behavior changed:

- DesignDirector, Designer, and CustomizationOperator now get `task_list` menu
  and page access.
- Audit roles now get `task.asset_upload`.
- Main-flow task detail read visibility was widened in the action authorizer.
- Pool team scope now recognizes canonical pool codes through real
  department/team mappings.
- Audit actions are blocked for task types that do not support audit.

Impact: some users gain task-list or upload abilities they did not have in
v1.21, while unsupported audit operations are more explicitly rejected.

### Notifications

`task_pending_audit` was added. Assignment and module-enter transitions can now
create notifications through the notification generator/service.

Impact: notification volume and payload types change. Frontend notification
rendering must handle `task_pending_audit`.

### Search

Global search now searches a broader task/document surface:

- Optional `task_search_documents` read model.
- Fallback legacy search is broader than v1.21.
- Task search results include task type, SKU, i_id, owner org fields, creator,
  designer, created time, and deadline.
- Asset/product search fields broadened.

Impact: search result ordering and match coverage will change. This is likely
positive, but it is user-visible.

### Assets / Files

Asset behavior changed:

- Global asset OpenAPI path parameter renamed to `asset_id`.
- Asset file serving can fall back to OSS direct URLs.
- Task asset creation now writes `source_module_key`.
- Audit-stage uploads are restricted to source/delivery assets.
- Retouch/customization/design source module attribution was tightened.
- Detail aggregation now enriches reference file refs and asset version access
  fields.

Impact: asset upload/download/detail surfaces are user-visible and need smoke
coverage for source, delivery, audit-stage, retouch, and OSS fallback paths.

## Governance / Tooling Impact

The branch also adds governance and guardrails:

- `AGENTS.md` / `CLAUDE.md` unified agent contract.
- `scripts/agent-check.sh` and `.ps1` full gate.
- `tools/contract_audit` contract drift gate.
- `.cursor/hooks/contract-guard.json`.
- V1 authority cleanup, archive moves, and generated frontend docs.

These are not runtime business behavior, but they materially improve future
change control.

## Deployment Recommendation

Treat HEAD as a new release candidate after v1.21, not a patch with no impact.

Minimum pre-deploy checklist:

1. Confirm production MySQL version supports `utf8mb4_0900_ai_ci`.
2. Dry-run migrations 069 and 070 against a production-size staging clone.
3. Confirm no active clients still send `priority=urgent`.
4. Confirm frontend is updated for:
   - `/v1/tasks/pool` pagination envelope.
   - `product_i_id` / `i_id` fields.
   - batch Excel new template/parse semantics.
   - `task_pending_audit` notification type.
   - asset path parameter name `asset_id` if generated clients care.
5. Run full backend gate: `./scripts/agent-check.sh`.
6. Run focused smoke:
   - auth/me/session.
   - task create for original/new/purchase/retouch.
   - new-product batch Excel template + parse + confirm.
   - `/v1/erp/iids`.
   - task detail for original/new/purchase/retouch/customization.
   - task pool list + claim conflict.
   - design submit for normal and retouch tasks.
   - audit approve/reject and audit-stage source/delivery upload.
   - asset upload/download/global asset detail.
   - global search.
   - notification list/unread for assignment and pending audit.
   - ERP filing retry for single and batch new-product tasks.

## Final Classification

- Functional impact: Yes.
- DB migration required: Yes, especially migration 070.
- Frontend compatibility impact: Yes.
- Runtime mounted route removal: No evidence of mounted route removal.
- OpenAPI/client contract impact: Yes.
- Safe to deploy as governance-only: No.
- Safe to deploy after migrations + frontend alignment + smoke: To be
  confirmed by staging verification.
