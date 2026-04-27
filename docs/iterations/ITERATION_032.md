# ITERATION_032 - Export Execution-Attempt / Runner-Adapter Visibility

**Date**: 2026-03-10  
**Scope**: PHASE_AUTO_032

## 1. Goals

- Add a durable execution-attempt layer for export jobs.
- Make the placeholder runner-adapter boundary explicit without introducing a real async platform.
- Keep export job lifecycle, lifecycle events, download handoff, and start boundary backward compatible while separating attempt lifecycle from job lifecycle.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 31 complete and OpenAPI `v0.28.0`.
- Export center already had:
  - export job persistence
  - lifecycle statuses
  - structured placeholder `result_ref`
  - lifecycle audit trace
  - placeholder claim/read boundary
  - placeholder handoff refresh boundary
  - explicit placeholder `POST /v1/export-jobs/{id}/start`
  - runner-boundary events `export_job.runner_initiated` / `export_job.started`
- Main remaining gap before this round:
  - no durable record for one concrete export-job start attempt
  - no explicit placeholder runner-adapter identity beyond event wording
  - no read-model split between job lifecycle state and attempt lifecycle state
  - no internal/admin inspection route for attempt history
- This round stayed out of scope for:
  - real async runner / scheduler platform
  - real file generation
  - real byte-stream download
  - signed URLs
  - NAS / object storage
  - BI / KPI / finance export expansion

## 3. Files Changed

### Code

- `cmd/server/main.go`
- `db/migrations/017_v7_export_job_attempts.sql`
- `domain/export_center.go`
- `domain/export_job_attempt.go`
- `domain/export_job_event.go`
- `repo/interfaces.go`
- `repo/mysql/export_job_attempt.go`
- `service/export_center_service.go`
- `service/export_center_service_test.go`
- `transport/handler/export_center.go`
- `transport/http.go`

### Documents

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_032.md`
- `docs/phases/PHASE_AUTO_032.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- Added DB migration `017_v7_export_job_attempts.sql`.
- New table:
  - `export_job_attempts`
- Stored fields include:
  - `attempt_id`
  - `export_job_id`
  - `attempt_no`
  - `trigger_source`
  - `execution_mode`
  - `adapter_key`
  - `status`
  - `started_at`
  - `finished_at`
  - `error_message`
  - `adapter_note`
  - `created_at`
  - `updated_at`
- No scheduler queue, worker lease, dispatch table, NAS handle, or storage-delivery table was introduced.

## 5. API Changes

### Export-job read model

- `GET /v1/export-jobs`
- `GET /v1/export-jobs/{id}`

Both now additionally expose:
- `attempt_count`
- `latest_attempt`
- `can_retry`

Current meaning:
- `status` remains the export job lifecycle state.
- `latest_attempt.status` is the most recent execution-attempt state.
- `can_retry=true` currently means the job is back in `queued` and already has historical attempts.

### New internal/admin route

- `GET /v1/export-jobs/{id}/attempts`

Purpose:
- inspect attempt history independently of the shared event timeline
- expose placeholder runner-adapter boundary data
- keep this route internal/admin only

### Existing start / lifecycle routes

- `POST /v1/export-jobs/{id}/start`
- `POST /v1/export-jobs/{id}/advance`

Behavior changes:
- successful start now creates one durable attempt record
- `advance action=start` still reuses the same start helper
- `mark_ready` / `fail` / `cancel` from `running` now finalize the latest running attempt

### Event chain additions

- Added attempt-result events:
  - `export_job.attempt_succeeded`
  - `export_job.attempt_failed`
  - `export_job.attempt_cancelled`
- Existing lifecycle and download-handoff events remain in place.
- Attempt context is now also carried in runner-related/lifecycle event payloads.

### OpenAPI

- Version updated from `0.28.0` to `0.29.0`.
- Added attempt schemas and the new `GET /attempts` path.
- Synced export-job schema with:
  - `attempt_count`
  - `latest_attempt`
  - `can_retry`
- Clarified which attempt visibility is frontend-ready via job detail/list and which remains internal/admin only.

## 6. Design Decisions

- Chose a dedicated `export_job_attempts` table instead of overloading attempt state into `export_job_events` only.
- Kept attempt visibility separate from job lifecycle:
  - job lifecycle answers the business object state
  - attempt lifecycle answers one concrete start execution
- Kept the current placeholder runner-adapter explicit through:
  - `execution_mode`
  - `adapter_key`
  - `adapter_note`
- Reused the shared export-job event chain for attempt-result audit visibility instead of adding a second event log.
- Kept `GET /v1/export-jobs/{id}/attempts` internal/admin only while exposing `latest_attempt` on frontend-ready job read models.

## 7. Correction Notes

- `docs/V7_FRONTEND_INTEGRATION_ORDER.md` previously referenced `POST /v1/export-jobs/{id}/refresh-download` in guidance text but omitted it from the integration list; this round corrected that drift.
- Export-center repository-truth docs previously stopped at Step 31 and described Step 32 only as a candidate; this round advanced state, handover, and appendix docs to Step 32 complete.
- OpenAPI and document wording were aligned so attempt visibility is clearly placeholder-only and not described as a real scheduler or worker platform.

## 8. Verification

- Ran `gofmt -w` on touched Go files.
- Added/updated service tests covering:
  - start creating an attempt
  - ready/fail/requeue transitions updating attempt visibility
  - job list/detail hydration with `attempt_count` / `latest_attempt`
  - attempt history across retry
  - unchanged placeholder download-handoff semantics with the new attempt layer
- Ran:
  - `go test ./service/...`

## 9. Risks / Known Gaps

- Attempt visibility is still placeholder-only and does not dispatch real background work.
- There is still no scheduler queue, worker lease, heartbeat, or adapter callback protocol for export jobs.
- Attempt creation currently starts directly in `running`; there is no separate dispatch/claimed/acked phase yet.
- Real file generation, storage delivery, signed URLs, and runner telemetry remain out of scope.

## 10. Suggested Next Step

- Keep real async execution and storage deferred.
- The next narrow phase should add one small adapter-dispatch or scheduler-handoff seam on top of the new attempt boundary, without turning it into a real execution platform yet.
