# V7 API Ready Index

Last purified: 2026-04-13

This file is a route-classification index only. It is not the primary authority.

Primary authority:

1. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
2. `docs/api/openapi.yaml`

## Official Mainline Families

- `/v1/auth/*`
- `/v1/access-rules`
- `/v1/org/options`
- `/v1/roles`
- `/v1/users*`
- `/v1/erp/products*`
- `/v1/erp/categories`
- `/v1/tasks/reference-upload`
- `/v1/tasks/prepare-product-codes`
- `/v1/tasks*`
- `/v1/tasks/{id}/asset-center/*`
- `/v1/assets/files/{path}`
- `/v1/tasks/{id}/audit/*`
- `/v1/tasks/{id}/warehouse/*`
- `/v1/outsource-orders`
- `/v1/warehouse/receipts`
- `/v1/task-board/*`
- `/v1/workbench/preferences`
- `/v1/export-jobs*`
- `/v1/code-rules*`

Frontend rollout must use the canonical OSS asset routes under:

- `POST /v1/tasks/reference-upload`
- `POST /v1/tasks/{id}/asset-center/upload-sessions/small`
- `POST /v1/tasks/{id}/asset-center/upload-sessions/multipart`
- `POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/complete`
- `POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/cancel`
- `GET /v1/assets/files/{path}`

## Compatibility-Only Families

- `/v1/products/search`
- `/v1/products/{id}`
- `/v1/task-create/asset-center/upload-sessions*`
- `/v1/tasks/{id}/assets*`
- `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}/abort`

## Deprecated Or Explicit-Error Surfaces

- `GET/PUT /v1/rule-templates/product-code`
- `POST /v1/tasks/{id}/assets/upload`
- `reference_images` on `POST /v1/tasks`
- `POST /v1/audit`

Compatibility-only fields not for new frontend work:

- `ReferenceFileRef.url`
- `public_download_allowed`
- `preview_public_allowed`

## Internal Or Legacy-Only Families

 - `/health`
 - `/ping`
 - `/v1/assets/upload-requests*`
- `/v1/products/sync/*`
- `/v1/erp/users`
- `/v1/export-jobs/{id}/dispatches*`
- `/v1/export-jobs/{id}/attempts`
- `/v1/export-jobs/{id}/start`
- `/v1/export-jobs/{id}/advance`
- `/v1/integration/*`
- `/v1/admin/jst-users*`
- V6 legacy families such as `/v1/sku/*`, `/v1/agent/*`, `/v1/incidents*`, `/v1/policies*`

For successor routes and removal timing, use the compatibility exit matrix in `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`.

## Archive

- Previous long-form version: `docs/archive/V7_API_READY_2026-04-09_ARCHIVE.md`
