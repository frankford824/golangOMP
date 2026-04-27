# Task Create Rules

Last purified: 2026-04-16

Current authority:

1. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
2. `docs/api/openapi.yaml`
3. `transport/http.go`

Use this file as a current task-create guide only. Historical notes were moved to archive.

## Current Entry Points

- Create task: `POST /v1/tasks`
- Pre-task reference upload: `POST /v1/tasks/reference-upload`
- Product-code preview: `POST /v1/tasks/prepare-product-codes`
- Original-product lookup: `GET /v1/erp/products` and `GET /v1/erp/products/{id}`

Compatibility-only entry points:

- `/v1/task-create/asset-center/upload-sessions*`

Rejected legacy input:

- `reference_images` on `POST /v1/tasks`

## Shared Rules

- `reference_file_refs` is the only supported task-create reference input.
- `owner_department` and `owner_org_team` are the canonical ownership fields.
- `owner_team` is still accepted and still returned for compatibility, but it is not the canonical ownership target.
- compatibility department names are still accepted for migration safety, but backend normalizes them back to the canonical current departments before persisting/returning task ownership.
- `due_at` remains the task deadline field.
- `customization_required` is the canonical creation-time selector for the customization lane.
- task read models expose `workflow_lane` as the explicit lane tag (`normal` / `customization`) derived from `customization_required`.
- `customization_source_type` keeps only the source distinction (`new_product` vs `existing_product`) inside that lane.
- `need_outsource` / `is_outsource` are compatibility-only create inputs for old callers; when true, backend normalizes them into `customization_required=true`.
- A customization task now creates its primary `customization_job` immediately and enters the customization workbench directly instead of the normal design lane.
- Frontend "2 in 1" create entry is still modeled on the existing task types plus customization selector:
  - `original_product_development + customization_required=true` covers existing-product customization entry
  - `new_product_development + customization_required=true` covers new-product / 来图定制 entry
- This round does not add a third task type or a parallel task-create contract for customization.
- Full payload schema and error response structure live in `docs/api/openapi.yaml`.

## Task Types

### `original_product_development`

Current rules:

- bind an existing product through the ERP mainline
- prefer `product_selection.erp_product`
- compatibility `product_id` input is still accepted
- `change_request` is required

Do not build new original-product flows on `/v1/products/search` or `/v1/products/{id}`.

### `new_product_development`

Current rules:

- requires `category_code`
- requires `material_mode`
- requires `product_name`
- requires `product_short_name`
- requires `design_requirement`
- backend owns default product-code generation during create
- `new_sku` remains optional

### `purchase_task`

Current rules:

- requires `purchase_sku`
- requires `product_name`
- requires `cost_price_mode`
- requires `quantity`
- requires `base_sale_price`
- `cost_price` is required when `cost_price_mode=manual`

## Batch SKU Rules

- `original_product_development` does not support `batch_sku_mode=multiple`.
- `new_product_development` and `purchase_task` may use `batch_sku_mode=multiple` with `batch_items[]`.
- In batch mode, per-SKU truth is under `sku_items[]`.
- Top-level batch `reference_file_refs` is only a compatibility mother-task summary and must not be treated as the per-SKU source of truth.

## Ownership Compatibility Note

- `owner_team` remains a legacy task-side compatibility field.
- The backend may normalize configured org-team aliases into `owner_team`.
- Unsupported `owner_team` values still fail validation.
- This is a compatibility bridge only, not a full org-model unification.

## Archive

- Previous long-form version: `docs/archive/TASK_CREATE_RULES_2026-04-09_ARCHIVE.md`
