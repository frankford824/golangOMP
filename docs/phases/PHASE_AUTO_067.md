# PHASE_AUTO_067

## Goal
- Prepare MAIN for the design asset upload center without implementing real NAS byte transfer.
- Lock the business boundary around task asset roots, asset versions, upload sessions, and the remote upload-service seam.
- Give frontend a stable asset-center API contract that can be linked before the NAS-side service is ready.

## Required Scope
- Add task-scoped `design_assets` plus additive `task_assets` version fields and additive `upload_requests` session fields.
- Introduce a dedicated upload-service client abstraction in MAIN with config for base URL / timeout / auth / storage provider.
- Add frontend-facing asset-center routes for:
  - asset list
  - asset version list
  - upload-session create/read/complete/cancel
- Keep audit/task-event traceability for upload-session create / complete / cancel.
- Sync OpenAPI, CURRENT_STATE, MODEL_HANDOVER, and iteration memory.

## Explicit Non-Goals
- Real NAS upload service implementation.
- Byte receive, multipart chunk merge, or local large-file persistence in MAIN.
- PSD/PSB/AI preview rendering or thumbnail pipeline.
- General-purpose file platform or unrelated permission expansion.

## Verification Target
- `go test ./...`
- Confirm migration/model/repo/service/handler boundaries stay self-consistent.
- Confirm OpenAPI and docs describe MAIN as metadata/orchestration only, with NAS transfer deferred to the remote upload service.
