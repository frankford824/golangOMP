# ITERATION_027 - Export Job Lifecycle / Download Handoff Skeleton

**Date**: 2026-03-10  
**Scope**: PHASE_AUTO_027

## 1. Goals

- Add a minimal export-job lifecycle so export center no longer stops at one static created record.
- Turn `result_ref` into a structured placeholder download-handoff contract instead of a loose static placeholder object.
- Let frontend/admin read one minimal closed loop through stable lifecycle fields without introducing real file generation or storage.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 26 complete and OpenAPI `v0.23.0`.
- Step 26 already completed:
  - `GET /v1/export-templates`
  - `POST /v1/export-jobs`
  - `GET /v1/export-jobs`
  - `GET /v1/export-jobs/{id}`
  - `export_jobs` persistence
- The main remaining product/backend gap was:
  - no lifecycle progression path after create
  - `result_ref` still using a static placeholder shape without clear download-handoff semantics
  - list/detail views missing explicit lifecycle read fields such as `progress_hint`, `latest_status_at`, and `download_ready`
- This round remained out of scope for:
  - real file generation
  - NAS / upload / object storage
  - full async scheduling platform
  - real export runner
  - BI / KPI / finance export modules

## 3. Files Changed

### Code

- `db/migrations/015_v7_export_job_lifecycle_status_timestamp.sql`
- `domain/export_center.go`
- `repo/interfaces.go`
- `repo/mysql/export_job.go`
- `service/export_center_service.go`
- `service/export_center_service_test.go`
- `transport/http.go`
- `transport/handler/export_center.go`

### Documents

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_027.md`
- `docs/phases/PHASE_AUTO_027.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

### Migration 015

- Added explicit lifecycle timestamp persistence on `export_jobs`:
  - `status_updated_at`
- Backfilled existing export jobs from:
  - `finished_at`
  - otherwise `updated_at`
  - otherwise `created_at`
- Added supporting index:
  - `idx_export_jobs_status_updated_at`

## 5. API Changes

### `POST /v1/export-jobs`

- Create now returns export jobs in initial:
  - `queued`
- Create responses now expose:
  - `progress_hint`
  - `latest_status_at`
  - `download_ready`
- Create still does not generate a real file; it only creates export intent plus placeholder handoff metadata.

### `GET /v1/export-jobs`

- List responses now expose stable lifecycle read fields:
  - `status`
  - `progress_hint`
  - `latest_status_at`
  - `download_ready`
- `result_ref` now uses the structured placeholder handoff shape.

### `GET /v1/export-jobs/{id}`

- Detail responses now expose the same lifecycle fields plus full structured placeholder handoff metadata through `result_ref`.

### `POST /v1/export-jobs/{id}/advance`

- Added internal/admin lifecycle-advance skeleton endpoint.
- Supported actions:
  - `start`
  - `mark_ready`
  - `fail`
  - `cancel`
  - `requeue`
- Current lifecycle transitions:
  - `queued -> running`
  - `running -> ready`
  - `queued|running -> failed`
  - `queued|running -> cancelled`
  - `failed|cancelled -> queued`
- This endpoint is explicitly:
  - internal/admin skeleton only
  - not frontend-ready
  - not a real runner or scheduler

### `result_ref`

- Replaced the old loose placeholder shape with structured placeholder handoff metadata:
  - `ref_type`
  - `ref_key`
  - `file_name`
  - `mime_type`
  - `expires_at`
  - `is_placeholder`
  - `note`
- `ready` export jobs now expose placeholder handoff metadata that looks downloadable to frontend, while still not being real storage integration.

### OpenAPI

- Version updated from `0.23.0` to `0.24.0`.
- Added:
  - `ExportJobProgressHint`
  - `ExportJobAdvanceAction`
  - `AdvanceExportJobRequest`
  - `POST /v1/export-jobs/{id}/advance`
- Clarified that:
  - export lifecycle is still skeleton-only
  - `result_ref` is placeholder handoff metadata
  - `ready` is not proof of real file generation or signed download delivery

## 6. Design Decisions

- Chose one narrow internal lifecycle-advance endpoint instead of pretending a full async export runner exists.
- Kept lifecycle states explicit and small:
  - `queued`
  - `running`
  - `ready`
  - `failed`
  - optional `cancelled`
- Added `status_updated_at` so `latest_status_at` does not depend on unrelated row updates.
- Kept `result_ref` as the handoff boundary rather than introducing a separate placeholder download service in the same round.
- Preserved the Step 26 source-query model:
  - task query
  - task-board queue handoff
  - procurement summary
  - warehouse receipts

## 7. Correction Notes

- Step 26 repository-truth docs and OpenAPI still documented export-job statuses as:
  - `pending`
  - `processing`
  - `completed`
- This round reconciled code, OpenAPI, `CURRENT_STATE.md`, and handover docs to the new lifecycle contract:
  - `queued`
  - `running`
  - `ready`
  - `failed`
  - `cancelled`
- `result_ref` documentation was also tightened from loose placeholder wording to explicit placeholder download-handoff semantics.

## 8. Verification

- Ran `gofmt -w` on all touched Go files.
- Added/updated service tests covering:
  - queued export-job creation
  - lifecycle progression to `running`
  - lifecycle progression to `ready`
  - failure and requeue transitions
  - structured placeholder handoff fields
- Ran:
  - `go test ./service/...`
  - `go test ./transport/...`
  - `go test ./repo/...`
  - `go test ./...`

## 9. Risks / Known Gaps

- `POST /v1/export-jobs/{id}/advance` is still manual lifecycle progression; no real background runner updates jobs automatically.
- `ready` now gives frontend a stable handoff object, but there is still no actual file retrieval endpoint, signed URL, or storage integration behind it.
- `result_ref.expires_at` is placeholder semantics only in this round.
- There is still no separate lifecycle event stream or audit trail for export jobs beyond row-level timestamps and remarks.

## 10. Suggested Next Step

- Keep real file generation and storage deferred.
- The next reasonable phase is to decide whether ready export jobs need a narrow placeholder download-claim/read endpoint or lightweight lifecycle audit trace, while continuing to avoid NAS, object storage, and full async platform scope.
