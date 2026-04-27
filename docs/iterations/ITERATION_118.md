# ITERATION 118

Date: 2026-04-09
Model: GPT-5 Codex

## Goal

Perform a pre-v0.9 consolidation pass across MAIN backend runtime, OpenAPI, handover/current-state docs, and frontend integration docs so the official contract matches the actual runtime and compatibility/deprecated routes are no longer described as primary.

## Runtime findings

- `transport/http.go` is the route-registration truth source.
- Official product selection path is `/v1/erp/products*`.
- `/v1/products/search` and `/v1/products/{id}` remain compatibility-only local-cache reads.
- Official task-create reference upload path is `POST /v1/tasks/reference-upload`.
- `/v1/task-create/asset-center/upload-sessions*` remains compatibility-only.
- Official task asset namespace is `/v1/tasks/{id}/asset-center/*`.
- `/v1/tasks/{id}/assets*` remains compatibility-only.
- `POST /v1/tasks/{id}/assets/upload` is deprecated for the old browser form contract and already returns `410` for that legacy usage.
- `reference_images` is rejected on `POST /v1/tasks`; `reference_file_refs` is the official create contract.
- Canonical task ownership fields are `owner_department` and `owner_org_team`; `owner_team` remains compatibility-active.
- Task product-code generation is backend-owned; `rule_templates/product-code` is deprecated by explicit error.

## Code changes

- Added explicit compatibility headers for major alias routes in `transport/http.go`:
  - `Deprecation: true`
  - `X-Workflow-API-Status: compatibility`
  - `X-Workflow-Successor-Path`
- Added regression coverage in `transport/http_test.go` for the compatibility signaling middleware.

## Documentation changes

- Added `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md` as the authoritative v0.9 contract summary.
- Rewrote `docs/V7_API_READY.md` and `docs/V7_FRONTEND_INTEGRATION_ORDER.md` to point frontend work at the actual mainline route families.
- Rewrote `docs/API_USAGE_GUIDE.md` around official ERP/product, task-reference, task-ownership, and product-code usage.
- Updated `docs/ASSET_UPLOAD_INTEGRATION.md` to state canonical `/asset-center/*` versus compatibility `/assets*`.
- Added a v0.9 supplement banner to `docs/TASK_CREATE_RULES.md`.
- Added authority banners to `CURRENT_STATE.md` and `MODEL_HANDOVER.md`.
- Updated `docs/api/openapi.yaml` to mark compatibility routes as deprecated and to restore `/asset-center/*` as canonical in the spec.

## Main classification outcome

### Official mainline

- `/v1/erp/products*`
- `/v1/tasks/reference-upload`
- `/v1/tasks*`
- `/v1/tasks/{id}/asset-center/*`
- `/v1/auth/me`, `/v1/org/options`, `/v1/users*`, `/v1/roles`

### Compatibility-only

- `/v1/products/search`
- `/v1/products/{id}`
- `/v1/task-create/asset-center/upload-sessions*`
- `/v1/tasks/{id}/assets*`
- `owner_team`

### Deprecated

- `rule_templates/product-code`
- `reference_images` on task create
- historical browser form use of `POST /v1/tasks/{id}/assets/upload`

### Remove candidates

- `/v1/sku/*`
- `/v1/agent/*`
- `/v1/incidents*`
- `/v1/policies*`
- `/v1/tasks/{id}/assets/mock-upload`

## Validation

- `go test ./service ./transport/handler`
- `go build ./cmd/server`
- `go build ./repo/mysql ./service ./transport/handler`
- `go test ./repo/mysql`

See iteration result in the final session report for exact pass/fail status.
