# ITERATION_067

## Phase
- PHASE_AUTO_067 / design asset upload-center integration-preparation layer

## Input Context
- Current CURRENT_STATE before execution: Step 66 complete with upload/storage still limited to placeholder upload-request boundaries
- Current OpenAPI version before execution: `0.66.0`
- Read latest iteration: `docs/iterations/ITERATION_066.md`
- Current execution phase file: `docs/phases/PHASE_AUTO_067.md`

## Goals
- Keep MAIN as the business orchestration side of the design asset upload center.
- Prepare `asset` / `asset_version` / `upload_session` boundaries without introducing real NAS byte transfer.
- Expose a stable frontend asset-center API contract and a stable remote upload-service client seam.

## Files Changed
- `cmd/api/main.go`
- `cmd/server/main.go`
- `config/config.go`
- `config/config_test.go`
- `CURRENT_STATE.md`
- `db/migrations/034_v7_design_asset_center_boundary.sql`
- `deploy/main.env.example`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_067.md`
- `docs/phases/PHASE_AUTO_067.md`
- `domain/asset_storage.go`
- `domain/audit.go`
- `domain/task_asset.go`
- `domain/task_asset_center.go`
- `MODEL_HANDOVER.md`
- `repo/interfaces.go`
- `repo/mysql/design_asset.go`
- `repo/mysql/task_asset.go`
- `repo/mysql/upload_request.go`
- `service/asset_upload_service.go`
- `service/task_asset_center_service.go`
- `service/task_asset_center_service_test.go`
- `service/task_asset_service.go`
- `service/task_prd_service_test.go`
- `service/task_step04_service_test.go`
- `service/upload_service_client.go`
- `transport/handler/task_asset_center.go`
- `transport/http.go`
- `ITERATION_INDEX.md`

## DB / Migration Changes
- Added migration `034_v7_design_asset_center_boundary.sql`.
- Added `design_assets` as task-scoped asset roots.
- `task_assets` now additionally persist asset-version-oriented fields:
  - `asset_id`
  - `asset_version_no`
  - `original_filename`
  - `storage_key`
  - `upload_status`
  - `preview_status`
  - `uploaded_at`
- `upload_requests` now additionally persist upload-session-oriented fields:
  - `task_id`
  - `asset_id`
  - `upload_mode`
  - `expected_size`
  - `storage_provider`
  - `session_status`
  - `remote_upload_id`
  - `created_by`
  - `expires_at`

## API Changes
- OpenAPI version advanced from `0.66.0` to `0.67.0`.
- Added asset-center routes:
  - `GET /v1/tasks/{id}/asset-center/assets`
  - `GET /v1/tasks/{id}/asset-center/assets/{asset_id}/versions`
  - `POST /v1/tasks/{id}/asset-center/upload-sessions/small`
  - `POST /v1/tasks/{id}/asset-center/upload-sessions/multipart`
  - `GET /v1/tasks/{id}/asset-center/upload-sessions/{session_id}`
  - `POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/complete`
  - `POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/cancel`
- `TaskAsset` and `UploadRequest` schemas now carry additive asset-center fields for compatibility.

## Design Decisions
- Reused the existing `task_assets` and `upload_requests` persistence layers additively instead of replacing them.
- Added a dedicated `design_assets` root so business asset identity is separated from append-only version records.
- Kept the upload-service seam behind a client abstraction so the later NAS-side Go service can attach without refactoring task logic.
- Treated remote upload behavior as stubbed contract planning only; MAIN still records metadata and audit traces, not bytes.

## Verification
- `gofmt -w` on changed Go sources
- `go test ./...`
