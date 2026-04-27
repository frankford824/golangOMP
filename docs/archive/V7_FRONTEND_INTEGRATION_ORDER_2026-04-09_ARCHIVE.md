# V7 Frontend Integration Order

Authoritative v0.9 contract: `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`

This file lists the order frontend work should integrate the current MAIN backend. It intentionally omits compatibility-only routes as primary targets.

## 1. Auth and access bootstrap

- `POST /v1/auth/login`
- `GET /v1/auth/me`
- `PUT /v1/auth/password`
- `GET /v1/org/options`
- `GET /v1/roles`

Rules:

- Protected frontend-ready routes use `Authorization: Bearer <token>`.
- `/v1/auth/me` returns the current profile plus computed `frontend_access`.
- For new frontend work, prefer `frontend_access.roles/scopes/menus/pages/actions/modules`.
- Compatibility alias keys may still appear in responses, but they are not the naming target for new docs.

## 2. Product selection and task-create prerequisites

- `GET /v1/erp/products`
- `GET /v1/erp/products/{id}`
- `GET /v1/erp/categories`
- `POST /v1/tasks/reference-upload`
- `POST /v1/tasks/prepare-product-codes`

Rules:

- Use `/v1/erp/products*` for original-product selection.
- Do not start new integrations on `/v1/products/search` or `/v1/products/{id}`.
- Task-create reference uploads should use `/v1/tasks/reference-upload`, not the old task-create asset-center routes.

## 3. Task create, list, and detail

- `POST /v1/tasks`
- `GET /v1/tasks`
- `GET /v1/tasks/{id}`
- `GET /v1/tasks/{id}/detail`
- `GET /v1/tasks/{id}/product-info`
- `PATCH /v1/tasks/{id}/product-info`
- `GET /v1/tasks/{id}/cost-info`
- `PATCH /v1/tasks/{id}/cost-info`
- `POST /v1/tasks/{id}/cost-quote/preview`
- `PATCH /v1/tasks/{id}/business-info`
- `PATCH /v1/tasks/{id}/procurement`
- `POST /v1/tasks/{id}/procurement/advance`

Rules:

- Task-create reference input is `reference_file_refs`.
- `reference_images` is no longer accepted on create.
- Canonical task ownership fields are `owner_department` and `owner_org_team`.
- `owner_team` is still returned and still accepted for compatibility, but should not be treated as the only source of truth.

## 4. Canonical task asset center

- `GET /v1/tasks/{id}/asset-center/assets`
- `GET /v1/tasks/{id}/asset-center/assets/{asset_id}/versions`
- `GET /v1/tasks/{id}/asset-center/assets/{asset_id}/download`
- `GET /v1/tasks/{id}/asset-center/assets/{asset_id}/versions/{version_id}/download`
- `POST /v1/tasks/{id}/asset-center/upload-sessions`
- `POST /v1/tasks/{id}/asset-center/upload-sessions/small`
- `POST /v1/tasks/{id}/asset-center/upload-sessions/multipart`
- `GET /v1/tasks/{id}/asset-center/upload-sessions/{session_id}`
- `POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/complete`
- `POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/cancel`

Rules:

- Treat `/v1/tasks/{id}/asset-center/*` as the canonical asset API namespace.
- Do not start new UI work on `/v1/tasks/{id}/assets*`.
- Large-file multipart lanes require NAS probe evidence and attestation.
- Returned download metadata may require the same probe evidence for private-network download.

## 5. Workflow actions

- `POST /v1/tasks/{id}/assign`
- `POST /v1/tasks/batch/assign`
- `POST /v1/tasks/batch/remind`
- `POST /v1/tasks/{id}/submit-design`
- `POST /v1/tasks/{id}/audit/claim`
- `POST /v1/tasks/{id}/audit/approve`
- `POST /v1/tasks/{id}/audit/reject`
- `POST /v1/tasks/{id}/audit/transfer`
- `POST /v1/tasks/{id}/audit/handover`
- `GET /v1/tasks/{id}/audit/handovers`
- `POST /v1/tasks/{id}/audit/takeover`
- `POST /v1/tasks/{id}/outsource`
- `POST /v1/tasks/{id}/warehouse/prepare`
- `POST /v1/tasks/{id}/warehouse/receive`
- `POST /v1/tasks/{id}/warehouse/reject`
- `POST /v1/tasks/{id}/warehouse/complete`
- `POST /v1/tasks/{id}/close`
- `GET /v1/tasks/{id}/events`

## 6. Read-model utilities

- `GET /v1/task-board/summary`
- `GET /v1/task-board/queues`
- `GET /v1/workbench/preferences`
- `PATCH /v1/workbench/preferences`
- `GET /v1/export-templates`
- `POST /v1/export-jobs`
- `GET /v1/export-jobs`
- `GET /v1/export-jobs/{id}`
- `GET /v1/export-jobs/{id}/events`
- `POST /v1/export-jobs/{id}/claim-download`
- `GET /v1/export-jobs/{id}/download`
- `POST /v1/export-jobs/{id}/refresh-download`
- `GET /v1/code-rules`
- `GET /v1/code-rules/{id}/preview`
- `POST /v1/code-rules/generate-sku`

Rules:

- `code-rules` remains a numbering utility module, but task create no longer depends on frontend-selected code rules for default task product-code generation.
- Export download routes expose structured handoff state; they are not proof of a real file-generation platform.

## Do not treat as mainline frontend targets

- `/v1/products/search`
- `/v1/products/{id}`
- `/v1/task-create/asset-center/upload-sessions*`
- `/v1/tasks/{id}/assets*`
- `/v1/tasks/{id}/assets/upload`
- `/v1/products/sync/*`
- `/v1/assets/upload-requests*`
- `/v1/integration/*`
- `/v1/export-jobs/{id}/dispatches*`
- `/v1/export-jobs/{id}/attempts`
- `/v1/export-jobs/{id}/start`
- `/v1/export-jobs/{id}/advance`
- `/v1/admin/jst-users*`
- `/v1/tasks/{id}/assets/mock-upload`
