# ITERATION_068

## Phase
- Step 68 / formal MAIN-to-NAS upload-service integration

## Input Context
- Current CURRENT_STATE before execution: Step 67 complete with asset center present but remote upload client still stubbed
- Current OpenAPI version before execution: `0.67.0`
- Read latest iteration: `docs/iterations/ITERATION_067.md`

## Goals
- Replace the stubbed upload-service seam with a real HTTP client boundary.
- Keep MAIN as business metadata only and bind the asset center to the NAS upload service.
- Promote `/v1/tasks/{id}/assets/*` into the primary frontend asset-center contract.

## Files Changed
- `cmd/api/main.go`
- `cmd/server/main.go`
- `config/config.go`
- `config/config_test.go`
- `CURRENT_STATE.md`
- `db/migrations/035_v7_asset_upload_nas_integration.sql`
- `deploy/DEPLOYMENT_WORKFLOW.md`
- `deploy/main.env.example`
- `docs/api/openapi.yaml`
- `docs/ASSET_UPLOAD_INTEGRATION.md`
- `docs/iterations/ITERATION_068.md`
- `domain/asset_storage.go`
- `domain/audit.go`
- `domain/task_asset.go`
- `domain/task_asset_center.go`
- `MODEL_HANDOVER.md`
- `repo/interfaces.go`
- `repo/mysql/task_asset.go`
- `repo/mysql/upload_request.go`
- `service/task_asset_center_service.go`
- `service/task_asset_center_service_test.go`
- `service/task_step04_service_test.go`
- `service/upload_service_client.go`
- `service/upload_service_client_test.go`
- `transport/handler/task_asset_center.go`
- `transport/http.go`
- `ITERATION_INDEX.md`

## DB / Migration Changes
- Added migration `035_v7_asset_upload_nas_integration.sql`.
- `task_assets` now additionally persist `remote_file_id`.
- `upload_requests` now additionally persist:
  - `remote_file_id`
  - `last_synced_at`

## API / Contract Changes
- OpenAPI version advanced from `0.67.0` to `0.68.0`.
- Promoted `/v1/tasks/{id}/assets/*` to the primary asset-center surface:
  - `GET /v1/tasks/{id}/assets`
  - `GET /v1/tasks/{id}/assets/{asset_id}/versions`
  - `POST /v1/tasks/{id}/assets/upload-sessions`
  - `GET /v1/tasks/{id}/assets/upload-sessions/{session_id}`
  - `POST /v1/tasks/{id}/assets/upload-sessions/{session_id}/complete`
  - `POST /v1/tasks/{id}/assets/upload-sessions/{session_id}/abort`
  - `POST /v1/tasks/{id}/assets/upload`
- Kept `/v1/tasks/{id}/asset-center/*` as compatibility aliases.
- Added `GET /v1/tasks/{id}/assets/timeline` as the legacy task-asset timeline view.

## Verification
- `go test ./...` passed after setting `GOTMPDIR` to a workspace-local directory because the default temp executable location was blocked by local Windows application-control policy.
