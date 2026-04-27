# API Usage Guide

Last purified: 2026-04-16

Current authority:

1. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
2. `docs/api/openapi.yaml`

Use this guide for current integration behavior only.

## Official Mainline Usage

### Product lookup

- use `GET /v1/erp/products`
- use `GET /v1/erp/products/{id}`
- use `GET /v1/erp/categories` as the supporting category source

Compatibility-only product routes:

- `GET /v1/products/search`
- `GET /v1/products/{id}`

New integrations must not start on `/v1/products*`.

### Task-create references

Official flow:

1. `POST /v1/tasks/reference-upload`
2. receive one normalized `reference_file_ref`
3. place that object into `POST /v1/tasks.reference_file_refs`

Compatibility-only flow:

- `/v1/task-create/asset-center/upload-sessions*`

### Asset download and preview

Official flow:

- read `download_mode` and `download_url`
- treat `download_url` as the only supported business file URL
- only treat `download_mode=direct` as valid direct mode when `download_url` is a real browser-direct URL (not `/v1/assets/files/{path}` or `/files/{path}`)
- use the returned `download_url` directly in browser for bytes
- treat `/v1/assets/files/{path}` as compatibility-only proxy fallback
- treat `/v1/assets/{id}/preview` as backend-owned preview metadata lookup
- source preview is two-tier:
  - direct OSS IMG preview for source formats `jpg/png/bmp/gif/webp/tiff/heic/avif` (signed URL may include `x-oss-process`)
  - async backend-derived preview for non-direct source formats (e.g. `psd/psb`)
- frontend must not parse raw PSD/PSB
- handle preview non-success states explicitly:
  - `404`: asset not found
  - `409`: asset exists but preview metadata not available yet (`INVALID_STATE_TRANSITION`)

Compatibility-only fields:

- `ReferenceFileRef.url`
- `public_download_allowed`
- `preview_public_allowed`

### Canonical OSS asset routes for frontend rollout

- `POST /v1/tasks/reference-upload`
- `POST /v1/assets/upload-sessions`
- `POST /v1/assets/upload-sessions/{session_id}/complete`
- `POST /v1/assets/upload-sessions/{session_id}/cancel`
- `GET /v1/assets`
- `GET /v1/assets/{id}`
- `GET /v1/assets/{id}/download`
- `GET /v1/assets/{id}/preview`
- `GET /v1/assets/files/{path}`

Canonical upload-session request notes:

- top-level `POST /v1/assets/upload-sessions` requires `task_id`
- send `asset_kind` as canonical intent field (`asset_type` is compatibility alias)
- do not send `upload_mode` for new integration (`upload_mode` is compatibility-only input)
- always provide file name (`file_name`; compatibility alias `filename` remains accepted)
- use canonical `asset_kind` values: `reference`, `source`, `delivery`, `preview`, `design_thumb`
- for preview/thumb derivatives, pass `source_asset_id` to keep source linkage explicit
- follow returned `upload_strategy` and remote upload plan
- execute returned `remote` URLs directly; do not inject internal auth headers; do not hardcode `/upload/*` proxy paths as canonical path
- call `POST /v1/assets/upload-sessions/{session_id}/complete` after browser upload success
- multipart completion confirmation is backend-owned at MAIN complete time; if browser-side remote complete was not confirmed yet, MAIN performs fallback remote complete through service-to-service path before persisting final asset/version metadata

`POST /v1/tasks/{id}/submit-design` batch submit notes:

- compatibility mode: send `asset_type + file_name` for single-asset submit
- batch mode: send `assets[]` and complete multiple upload sessions in one submit action
- each `assets[]` item must include `upload_session_id`
- each batch delivery item must include `target_sku_code`
- `assets[].target_sku_code` must belong to current task and match upload-session captured `target_sku_code`
- backend response in batch mode returns `data.submitted_assets[]` (each item includes completed session + persisted asset/version)

Canonical resource query notes:

- use `GET /v1/assets?task_id={id}` for cross-page resource list views
- use `GET /v1/tasks/{id}/assets` for canonical task-scoped list views
- use `source_asset_id` filter when querying source-linked preview/thumb derivatives
- treat `/v1/tasks/{id}/asset-center/assets*` as compatibility aliases only

Compatibility-only asset routes:

- `/v1/task-create/asset-center/upload-sessions*`
- `/v1/tasks/{id}/asset-center/upload-sessions*`
- `/v1/tasks/{id}/asset-center/assets*`
- `/v1/tasks/{id}/assets*` except `GET /v1/tasks/{id}/assets`

### Task ownership

- canonical fields: `owner_department`, `owner_org_team`
- compatibility field: `owner_team`
- compatibility department names remain accepted for migration safety, but backend normalizes them back to the canonical current departments in task create/read flows

Do not write new frontend logic that treats `owner_team` as the only ownership truth.

### Lane split and warehouse intake

- `customization_required` is the canonical lane selector at create-time.
- `workflow_lane` is the canonical read projection for lane distinction (`normal` / `customization`).
- use `GET /v1/tasks?workflow_lane=normal|customization` when list/workbench pages need an explicit lane split.
- Warehouse remains one unified intake surface:
  - use `GET /v1/warehouse/receipts?workflow_lane=normal|customization` for lane-focused workbench reads
  - each warehouse receipt read now carries `workflow_lane` and canonical upstream `source_department`
- Reassign policy on `POST /v1/tasks/{id}/assign` in `InProgress`:
  - allowed: requester/initiator, current owning-group `TeamLead`, scoped management roles
  - denied: ordinary Ops without those conditions

### Product-code generation

- canonical create behavior: backend default allocator in `POST /v1/tasks`
- canonical preview route: `POST /v1/tasks/prepare-product-codes`
- deprecated product-code config route:
  - `GET /v1/rule-templates/product-code`
  - `PUT /v1/rule-templates/product-code`

`/v1/code-rules*` remains a numbering utility module, not the task-create product-code authority.

### Customization piece-rate pricing (MVP)

- canonical customization routes:
  - `POST /v1/tasks/{id}/customization/review`
  - `GET /v1/customization-jobs`
  - `GET /v1/customization-jobs/{id}`
  - `POST /v1/customization-jobs/{id}/effect-preview`
  - `POST /v1/customization-jobs/{id}/effect-review`
  - `POST /v1/customization-jobs/{id}/production-transfer`
- pricing identity is user `employment_type` (`full_time` / `part_time`), not role split
- workflow permissions stay on existing customization roles; no full-time/part-time role explosion
- workflow operator role for this lane is `CustomizationOperator`; review role remains `CustomizationReviewer`
- pricing rules are keyed by `customization_level_code + employment_type`
- reviewer-stage reference pricing is persisted separately on customization job:
  - `review_reference_unit_price`
  - `review_reference_weight_factor`
- execution-stage frozen settlement snapshot is persisted separately on customization job:
  - `pricing_worker_type`
  - `unit_price`
  - `weight_factor`
- customization job also persists workflow trace fields:
  - `order_no`
  - `source_asset_id`
  - `current_asset_id`
  - `assigned_operator_id`
  - `last_operator_id`
- pricing snapshot is frozen when operator assignment becomes explicit in workflow (first effect-preview submit)
- pricing freeze must resolve by `(employment_type + customization_level_code)` and both full-time/part-time paths are first-class
- reviewer reference pricing must not be treated as the settlement snapshot; settlement freeze happens later at the canonical effect-preview freeze point
- if no enabled pricing rule matches, effect-preview returns a clear 4xx error and task/job states remain unchanged
- customization level stays business-configurable (`customization_level_code`, `customization_level_name`), not hardcoded A/B/C
- ERP order-detail remains out of customization pricing path
- `return_to_designer` in customization review is a successful state transition branch (no `customization_job` is created in this branch)
- warehouse rejection in customization branch routes back to `last_customization_operator_id`, which is maintained by effect-preview and production-transfer operations
- effect-preview submit now has two explicit branches:
  - `decision_type=effect_preview`: enters second-review loop
  - `decision_type=final`: skips second-review loop and enters production-transfer directly
- `order_no` is a stored workflow trace field on effect-preview submit; backend does not call ERP order-detail APIs to validate it in this round
- reviewer-fixed replacement uses `POST /v1/customization-jobs/{id}/effect-review` with a new `current_asset_id`, so second稿 replacement stays traceable
- reviewer-fixed replacement events now explicitly include both `previous_asset_id` and replacement `current_asset_id`, plus `workflow_lane` and canonical upstream `source_department`
- production transfer accepts placeholder trace fields `transfer_channel` and `transfer_reference`; this is event trace only, not a new external integration contract
- warehouse reject accepts `reject_category` so later error-reason statistics can aggregate from persisted task/job data

## Blocked For New Usage

- `/v1/products*`
- `/v1/task-create/asset-center/upload-sessions*`
- `/v1/tasks/{id}/asset-center/upload-sessions*`
- `/v1/tasks/{id}/asset-center/assets*`
- `/v1/tasks/{id}/assets*` except `GET /v1/tasks/{id}/assets`
- `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}/abort`
- `GET/PUT /v1/rule-templates/product-code`
- `POST /v1/audit`

## Archive

- Previous long-form version: `docs/archive/API_USAGE_GUIDE_2026-04-09_ARCHIVE.md`
