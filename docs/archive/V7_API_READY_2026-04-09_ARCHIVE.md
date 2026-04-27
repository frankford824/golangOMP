# V7 API Ready Matrix

Authoritative v0.9 contract: `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`

OpenAPI source: `docs/api/openapi.yaml`

This file is now a concise readiness index, not a historical architecture narrative. If this file conflicts with route registration in `transport/http.go`, prefer the runtime.

## Official mainline route families

- Auth and access:
  - `/v1/auth/login`
  - `/v1/auth/me`
  - `/v1/auth/password`
  - `/v1/org/options`
  - `/v1/roles`
  - `/v1/users*`
  - `/v1/access-rules`
- ERP/product selection:
  - `/v1/erp/products*`
  - `/v1/erp/categories`
  - `/v1/erp/warehouses`
  - `/v1/erp/sync-logs*`
- Task create and task lifecycle:
  - `/v1/tasks/reference-upload`
  - `/v1/tasks/prepare-product-codes`
  - `/v1/tasks*`
  - `/v1/tasks/{id}/product-info`
  - `/v1/tasks/{id}/cost-info`
  - `/v1/tasks/{id}/business-info`
  - `/v1/tasks/{id}/procurement*`
  - `/v1/tasks/{id}/detail`
  - `/v1/tasks/{id}/cost-overrides`
  - `/v1/tasks/{id}/events`
- Canonical asset center:
  - `/v1/tasks/{id}/asset-center/assets*`
  - `/v1/tasks/{id}/asset-center/upload-sessions*`
- Workflow actions:
  - `/v1/tasks/{id}/audit/*`
  - `/v1/tasks/{id}/outsource`
  - `/v1/tasks/{id}/warehouse/*`
  - `/v1/outsource-orders`
  - `/v1/warehouse/receipts`
- Frontend utilities:
  - `/v1/task-board/*`
  - `/v1/workbench/preferences`
  - `/v1/export-templates`
  - `/v1/export-jobs*`
  - `/v1/code-rules*`

## Compatibility-only route families

- `/v1/products/search`
- `/v1/products/{id}`
- `/v1/task-create/asset-center/upload-sessions*`
- `/v1/tasks/{id}/assets`
- `/v1/tasks/{id}/assets/timeline`
- `/v1/tasks/{id}/assets/{asset_id}/versions`
- `/v1/tasks/{id}/assets/{asset_id}/download`
- `/v1/tasks/{id}/assets/{asset_id}/versions/{version_id}/download`
- `/v1/tasks/{id}/assets/upload-sessions*`
- `/v1/tasks/{id}/assets/upload`
- `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}/abort`

Rules:

- Keep callable for v0.9 continuity.
- Do not document them as preferred frontend entrypoints.
- Runtime now emits explicit compatibility headers on the major alias families.

## Deprecated or explicit-error surfaces

- `GET /v1/rule-templates/product-code`
- `PUT /v1/rule-templates/product-code`
- `POST /v1/tasks/{id}/assets/upload` for historical browser form upload
- `reference_images` on `POST /v1/tasks`
- `POST /v1/audit`

## Internal or placeholder surfaces

- `/health`
- `/ping`
- `/v1/assets/files/{path}`
- `/v1/assets/upload-requests*`
- `/v1/products/sync/*`
- `/v1/erp/users`
- `/v1/export-jobs/{id}/dispatches*`
- `/v1/export-jobs/{id}/attempts`
- `/v1/export-jobs/{id}/start`
- `/v1/export-jobs/{id}/advance`
- `/v1/integration/*`
- `/v1/admin/jst-users*`
- `/v1/tasks/{id}/cost-overrides/{event_id}/review`
- `/v1/tasks/{id}/cost-overrides/{event_id}/finance-mark`
- `/v1/tasks/{id}/assets/mock-upload`

## Naming and field truth rules

- Product selection:
  - official read path is `/v1/erp/products*`
  - `/v1/products*` is compatibility cache access only
- Task references:
  - official create field is `reference_file_refs`
  - `reference_images` is no longer accepted on create
- Task ownership:
  - canonical fields are `owner_department` and `owner_org_team`
  - `owner_team` remains compatibility-active
- Task asset center:
  - canonical API namespace is `/asset-center/*`
  - `/assets*` remains compatibility-active only
- Product codes:
  - task create uses backend-owned default product-code allocation
  - `POST /v1/tasks/prepare-product-codes` is the official preview path
  - `rule_templates/product-code` is deprecated

## Auth and permission truth

- `GET /v1/auth/me` is bearer/session-backed and frontend-ready.
- `GET /v1/org/options` is the backend org source for account/user-management writes.
- `frontend_access` is computed from current roles plus canonical user `department/team`.
- Debug headers are compatibility-only for placeholder/internal routes and are not the mainline auth contract.
