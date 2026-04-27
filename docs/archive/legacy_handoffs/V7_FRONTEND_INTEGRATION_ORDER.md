# V7 Frontend Integration Order

Last purified: 2026-04-13

This file is a current frontend sequence guide. It is not the primary authority.

Primary authority:

1. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
2. `docs/api/openapi.yaml`

## Integration Order

1. Auth and access bootstrap
   - `POST /v1/auth/login`
   - `GET /v1/auth/me`
   - `PUT /v1/auth/password`
   - `GET /v1/org/options`
   - `GET /v1/roles`
2. Product selection and task-create prerequisites
   - `GET /v1/erp/products`
   - `GET /v1/erp/products/{id}`
   - `GET /v1/erp/categories`
   - `POST /v1/tasks/reference-upload`
   - `POST /v1/tasks/prepare-product-codes`
3. Task create, list, and detail
   - `POST /v1/tasks`
   - `GET /v1/tasks`
   - `GET /v1/tasks/{id}`
   - `GET /v1/tasks/{id}/detail`
4. Canonical asset registry and upload sessions
   - `POST /v1/assets/upload-sessions`
   - `POST /v1/assets/upload-sessions/{session_id}/complete`
   - `POST /v1/assets/upload-sessions/{session_id}/cancel`
   - `GET /v1/assets`
   - `GET /v1/assets/{id}`
   - `GET /v1/assets/{id}/download`
   - `GET /v1/assets/{id}/preview`
   - `GET /v1/assets/files/{path}`
   - returned file links must be consumed through `download_url` (runtime may return signed OSS direct URL; compatibility may still return `/v1/assets/files/{path}`)
5. Workflow actions and utilities
   - `POST /v1/tasks/{id}/submit-design` (batch mode supports `assets[]` for multi-file submit)
   - `/v1/tasks/{id}/audit/*`
   - `POST /v1/tasks/{id}/customization/review`
   - `/v1/customization-jobs*`
   - `/v1/tasks/{id}/warehouse/*`
   - `/v1/task-board/*`
   - `/v1/workbench/preferences`
   - `/v1/export-jobs*`

## Rules

- Use bearer token auth on frontend-ready protected routes.
- Use `reference_file_refs`, not `reference_images`.
- Use `owner_department` and `owner_org_team` as canonical ownership fields.
- Treat `owner_team` as compatibility output/input only.
- Use `customization_required` to create customization-lane tasks.
- Use `customization_source_type` only to distinguish `new_product` vs `existing_product` inside the customization lane.
- Treat `need_outsource` and `is_outsource` as compatibility-only create fields. New frontend logic must not use them as workflow selectors.
- A task created with `customization_required=true` now appears in `/v1/customization-jobs` immediately and must not be routed through the normal design workbench first.
- Use `/v1/assets*` as the canonical asset resource contract.
- Use `GET /v1/tasks/{id}/assets` as the canonical task-linked asset lookup route.
- Treat `/v1/tasks/{id}/asset-center/assets*` as compatibility-only read aliases.
- Treat `/v1/tasks/{id}/asset-center/upload-sessions*` as compatibility-only upload-session aliases.
- Treat `upload_mode` as compatibility-only input/output and let backend-returned `upload_strategy` drive upload execution.
- For batch-SKU delivery submit on `/v1/tasks/{id}/submit-design`, pass `assets[]` and provide per-item `target_sku_code`.
- Use `download_url` as the only supported business file URL field.
- Use `GET /v1/assets/{id}/preview` as the only preview metadata route and handle source preview in two tiers:
  - OSS IMG direct preview for source formats `jpg/png/bmp/gif/webp/tiff/heic/avif` (signed URL may include `x-oss-process`).
  - Async backend-derived preview for non-direct source formats like `psd/psb`; before derived preview is ready, API returns `409 INVALID_STATE_TRANSITION`.
- Do not attempt frontend-side parsing/rendering of raw PSD/PSB source files.
- Treat `ReferenceFileRef.url`, `public_download_allowed`, and `preview_public_allowed` as compatibility-only output. Do not start new frontend logic on them.

## Do Not Start New Work On

- `/v1/products*`
- `/v1/task-create/asset-center/upload-sessions*`
- `/v1/tasks/{id}/asset-center/assets*`
- `/v1/tasks/{id}/assets*` except `GET /v1/tasks/{id}/assets`
- `/v1/tasks/{id}/asset-center/upload-sessions/small`
- `/v1/tasks/{id}/asset-center/upload-sessions/multipart`
- `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}/abort`
- `/v1/tasks/{id}/outsource`
- `/v1/outsource-orders`
- `GET/PUT /v1/rule-templates/product-code`
- `POST /v1/audit`
- `/v1/assets/upload-requests*`
- `/v1/products/sync/*`
- `/v1/integration/*`
- `docs/archive/obsolete_alignment/FRONTEND_ALIGNMENT_v0.5.md`

## Archive

- Previous long-form version: `docs/archive/V7_FRONTEND_INTEGRATION_ORDER_2026-04-09_ARCHIVE.md`
