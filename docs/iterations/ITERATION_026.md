# ITERATION_026 - Export Center Skeleton

**Date**: 2026-03-10  
**Scope**: PHASE_AUTO_026

## 1. Goals

- Add a minimal export-center skeleton so stable query results can be persisted as export jobs.
- Reuse existing task list / task-board / procurement-summary / warehouse read-model query state instead of introducing a parallel reporting query language.
- Define clear export-job, template, status, and placeholder `result_ref` contracts without expanding into real file generation or storage.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 25 complete and OpenAPI `v0.22.0`.
- Stable frontend-ready query/read-model sources already existed through:
  - `GET /v1/tasks`
  - `GET /v1/task-board/summary`
  - `GET /v1/task-board/queues`
  - purchase-facing `procurement_summary`
  - `GET /v1/warehouse/receipts`
- The main remaining PRD/backend gap was no unified export carrier:
  - frontend could query and filter stable views
  - frontend could not persist those current results as durable export tasks
  - there was no formal `result_ref` placeholder contract for future storage/generation handoff
- Correction scope discovered during bootstrap:
  - `docs/V7_API_READY.md` was still pointing at `v0.19.0`
  - repository-truth docs had not yet documented export-center readiness because the capability did not exist
- This round remained out of scope for:
  - real file generation
  - NAS / upload / storage integration
  - full export template engine
  - full async scheduling platform
  - BI / KPI / finance reporting
  - real permission trimming

## 3. Files Changed

### Code

- `cmd/server/main.go`
- `db/migrations/014_v7_export_center_skeleton.sql`
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
- `docs/iterations/ITERATION_026.md`
- `docs/phases/PHASE_AUTO_026.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

### Migration 014

- Added `export_jobs` with minimal export-center persistence for:
  - `template_key`
  - `export_type`
  - `source_query_type`
  - `source_filters_json`
  - `normalized_filters_json`
  - `query_template_json`
  - `requested_by_*`
  - `status`
  - `result_ref_json`
  - `remark`
  - `finished_at`
  - timestamps
- Added supporting indexes for:
  - `status + created_at`
  - `source_query_type + created_at`
  - `requested_by_actor_id + created_at`

## 5. API Changes

### `GET /v1/export-templates`

- Added a minimal static template catalog for the current export-center skeleton.
- Current catalog is code-defined only; no `export_templates` table is introduced yet.
- Current templates are placeholder-only and cover:
  - task list
  - task-board queue
  - procurement summary
  - warehouse receipts

### `POST /v1/export-jobs`

- Added minimal export-job creation over stable read-model sources.
- Current supported `export_type` values:
  - `task_list`
  - `task_board_queue`
  - `procurement_summary`
  - `warehouse_receipts`
- Current supported `source_query_type` values:
  - `task_query`
  - `task_board_queue`
  - `procurement_summary`
  - `warehouse_receipts`
- Task-query-derived exports now accept persisted:
  - `query_template`
  - optional `normalized_filters`
- Task-board queue exports now persist:
  - `source_filters.queue_key`
  - optional `source_filters.board_view`
  - current `query_template`
  - optional `normalized_filters`
- Warehouse export jobs now persist current list filter fields through:
  - `source_filters.task_id`
  - `source_filters.receiver_id`
  - `source_filters.status`

### `GET /v1/export-jobs`

- Added paginated export-job list query with filters:
  - `status`
  - `source_query_type`
  - `requested_by_id`

### `GET /v1/export-jobs/{id}`

- Added single export-job read query.

### `result_ref`

- Formalized a placeholder-only result-reference structure:
  - `kind`
  - `locator`
  - `file_name`
  - `media_type`
  - `status`
  - `note`
- This is explicitly not:
  - a real NAS path
  - a signed URL
  - proof that a real file has been generated

### OpenAPI

- Version updated from `0.22.0` to `0.23.0`.
- Added export-center tags, schemas, and path docs.
- Explicitly documented export-center as a skeleton and clarified that `result_ref` is placeholder metadata only.

## 6. Design Decisions

- Chose one persisted `export_jobs` table instead of jumping to a full report/export platform.
- Kept template metadata static in code for now; this makes template intent explicit without prematurely adding `export_templates` CRUD or versioning.
- Reused existing stable `query_template` / `normalized_filters` contracts so frontend can convert current task list or task-board state directly into export jobs.
- Kept warehouse exports on source-specific `source_filters` rather than forcing warehouse list state into the task-query contract.
- Generated placeholder `result_ref` metadata immediately on create so downstream layers already have a stable handoff object even before real storage exists.

## 7. Correction Notes

- `docs/V7_API_READY.md` was stale before this round and still referenced OpenAPI `v0.19.0`; it has been corrected while adding export-center readiness.
- Repository-truth docs were updated to reflect that the new export-center endpoints are frontend-ready skeleton APIs, not internal placeholders.

## 8. Verification

- Ran `gofmt -w` on all touched Go files.
- Added service tests covering:
  - task-board export job creation
  - missing query-template rejection for task-query-derived exports
  - warehouse export validation
  - static export-template catalog exposure
- Ran:
  - `go test ./service/...`
  - `go test ./repo/...`
  - `go test ./transport/...`
  - `go test ./cmd/...`
  - `go test ./...`

## 9. Risks / Known Gaps

- Export jobs are persisted, but there is still no runner that advances them from placeholder persistence into real generated files.
- Static template metadata is enough for the current skeleton, but later template versioning or business-owned template configuration will need a dedicated boundary.
- `result_ref` is intentionally synthetic metadata; frontend and downstream docs must not misread it as a live download contract.
- Task-query-derived export replay depends on the continued stability of current `query_template` and `normalized_filters` semantics.

## 10. Suggested Next Step

- Keep real file storage and async execution deferred.
- The next reasonable phase is to decide whether export jobs need a minimal lifecycle advancement boundary, such as explicit placeholder completion/failure updates or download-handoff metadata, while still avoiding NAS/storage/BI expansion.
