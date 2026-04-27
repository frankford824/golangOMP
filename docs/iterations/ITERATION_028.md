# ITERATION_028 - Export Job Lifecycle Audit Trace

**Date**: 2026-03-10  
**Scope**: PHASE_AUTO_028

## 1. Goals

- Add a durable lifecycle audit trace for export jobs without changing the existing export-center placeholder boundaries.
- Record status progression, failure context, and placeholder handoff mutations in one stable event chain.
- Expose a timeline query plus lightweight event summaries so frontend/internal troubleshooting can inspect export-job history directly.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 27 complete and OpenAPI `v0.24.0`.
- Step 27 already completed:
  - `GET /v1/export-templates`
  - `POST /v1/export-jobs`
  - `GET /v1/export-jobs`
  - `GET /v1/export-jobs/{id}`
  - `POST /v1/export-jobs/{id}/advance`
  - export-job lifecycle statuses
  - placeholder `result_ref` handoff contract
- The main remaining product/backend gap was:
  - no dedicated lifecycle audit trace for export jobs
  - no timeline query for troubleshooting each lifecycle transition
  - no unified record for failure reasons or material placeholder handoff changes
- This round remained out of scope for:
  - real file generation
  - real download endpoints
  - signed URLs
  - NAS / object storage
  - full async runner platform
  - BI / KPI / finance export expansion

## 3. Files Changed

### Code

- `cmd/server/main.go`
- `db/migrations/016_v7_export_job_events.sql`
- `domain/export_center.go`
- `domain/export_job_event.go`
- `repo/interfaces.go`
- `repo/mysql/export_job_event.go`
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
- `docs/iterations/ITERATION_028.md`
- `docs/phases/PHASE_AUTO_028.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

### Migration 016

- Added dedicated export-job audit-trace persistence:
  - `export_job_events`
  - `export_job_event_sequences`
- `export_job_events` stores:
  - `event_id`
  - `export_job_id`
  - `sequence`
  - `event_type`
  - `from_status`
  - `to_status`
  - `actor_id`
  - `actor_type`
  - `note`
  - `payload`
  - `created_at`
- Added uniqueness and read indexes for:
  - `event_id`
  - per-job sequence
  - per-job created-at lookup
  - per-job event-type lookup

## 5. API Changes

### `GET /v1/export-jobs/{id}/events`

- Added export-job timeline query.
- Readiness level:
  - `ready_for_frontend`
- Purpose:
  - troubleshooting timeline display
  - lifecycle visibility
  - placeholder handoff/audit context lookup
- The response is ordered oldest to newest and returns audit context, not runner logs.

### `GET /v1/export-jobs` and `GET /v1/export-jobs/{id}`

- Export-job read models now also expose lightweight audit summaries:
  - `event_count`
  - `latest_event`
- Full event timeline is still kept out of list payloads.

### Lifecycle event coverage

- Export jobs now record:
  - `export_job.created`
  - `export_job.advanced_to_running`
  - `export_job.advanced_to_ready`
  - `export_job.advanced_to_failed`
  - `export_job.advanced_to_cancelled`
  - `export_job.advanced_to_queued`
  - `export_job.result_ref_updated`
- `result_ref_updated` is emitted only for material placeholder handoff changes, not every note-only lifecycle update.

### OpenAPI

- Version updated from `0.24.0` to `0.25.0`.
- Added schemas for:
  - `ExportJobEvent`
  - `ExportJobEventSummary`
  - `ExportJobEventListResponse`
- Clarified:
  - export audit-trace payload is freeform audit context only
  - it is not a full runner log stream
  - real file generation and download delivery are still deferred

## 6. Design Decisions

- Chose a dedicated `export_job_events` table instead of overloading `export_jobs.remark` or pretending row timestamps are enough.
- Reused the same per-aggregate sequence pattern already used by task events so export-job timelines have stable ordering.
- Kept event summaries lightweight on list/detail and exposed the full timeline through a separate endpoint.
- Recorded lifecycle transitions inside the same transaction as export-job state changes so status and audit trace do not drift apart.
- Treated `result_ref_updated` as a placeholder handoff event, not as evidence of real storage or download capability.

## 7. Correction Notes

- Repository-truth docs still pointed to Step 28 as a candidate choice between placeholder download claim/read and lifecycle audit trace.
- This round resolves that candidate decision by landing lifecycle audit trace as the actual Step 28 scope and syncing:
  - `CURRENT_STATE.md`
  - `MODEL_HANDOVER.md`
  - `docs/V7_API_READY.md`
  - `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
  - `docs/V7_MODEL_HANDOVER_APPENDIX.md`
  - `docs/api/openapi.yaml`
- OpenAPI version was incremented to `0.25.0` for the new timeline endpoint and summary fields.

## 8. Verification

- Ran `gofmt -w` on all touched Go files.
- Added/updated service tests covering:
  - create writes `export_job.created`
  - ready lifecycle emits timeline events in order
  - failure/requeue lifecycle summaries
  - list/detail event summaries
- Ran:
  - `go test ./service/...`
  - `go test ./repo/...`
  - `go test ./transport/...`
  - `go test ./...`
- Environment limitation:
  - handler package test binaries under `workflow/transport/handler` are blocked by local Application Control policy, so `go test ./transport/...` and `go test ./...` fail only at that execution step.

## 9. Risks / Known Gaps

- Export-job event timelines are still manual/admin-driven where lifecycle changes depend on `POST /v1/export-jobs/{id}/advance`; there is no real background runner yet.
- Event payload remains intentionally lightweight and should not be treated as a substitute for full runner logs or download-delivery telemetry.
- There is still no real download endpoint, signed URL, NAS, or object-storage integration behind `result_ref`.

## 10. Suggested Next Step

- Keep storage and real runner execution deferred.
- The next narrow phase should add a placeholder download-claim/read contract over ready export jobs, reusing the new event chain instead of inventing a separate lifecycle log.
