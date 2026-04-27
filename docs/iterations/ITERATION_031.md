# ITERATION_031 - Export Runner-Initiation Semantics

**Date**: 2026-03-10  
**Scope**: PHASE_AUTO_031

## 1. Goals

- Add an explicit placeholder runner-initiation boundary for export jobs.
- Formalize `queued -> running` as a start contract instead of leaving it implicit inside generic lifecycle advance semantics.
- Keep existing export lifecycle, `result_ref`, claim/read, refresh, and audit-trace contracts backward compatible.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 30 complete and OpenAPI `v0.27.0`.
- Export center already had:
  - export job persistence
  - lifecycle statuses
  - structured placeholder `result_ref`
  - lifecycle audit trace through `GET /v1/export-jobs/{id}/events`
  - placeholder claim/read boundary through `POST /v1/export-jobs/{id}/claim-download` and `GET /v1/export-jobs/{id}/download`
  - enforced placeholder handoff expiry / refresh through `POST /v1/export-jobs/{id}/refresh-download`
- The main remaining gap was:
  - no explicit runner-initiation endpoint
  - `queued -> running` still mostly depended on generic `advance` behavior
  - no stable read-model hints for start affordance or latest runner-side placeholder activity
- This round remained out of scope for:
  - real async runner / scheduler platform
  - real file generation
  - real byte-stream download
  - signed URLs
  - NAS / object storage
  - BI / KPI / finance export expansion

## 3. Files Changed

### Code

- `domain/export_center.go`
- `domain/export_job_event.go`
- `repo/interfaces.go`
- `repo/mysql/export_job_event.go`
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
- `docs/iterations/ITERATION_031.md`
- `docs/phases/PHASE_AUTO_031.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- No DB migration in this round.
- Placeholder runner-initiation semantics continue to reuse:
  - existing `export_jobs`
  - existing `export_job_events`
- No runner queue, dispatch table, worker lease table, or delivery table was introduced.

## 5. API Changes

### `POST /v1/export-jobs/{id}/start`

- Added an internal/admin placeholder runner-initiation endpoint.
- Purpose:
  - formalize the `queued -> running` boundary
  - give future real runner integration one explicit replacement seam
  - append explicit start-boundary events without pretending a real async platform exists
- This endpoint is allowed only when export job status is `queued`.

### `POST /v1/export-jobs/{id}/advance`

- Kept for generic lifecycle changes and backward compatibility.
- `action=start` is still accepted, but now reuses the same service-layer start helper as `POST /v1/export-jobs/{id}/start`.
- `advance` remains internal/admin only and is still not frontend-ready.

### Export-job list/detail read model

- Added lightweight runner-initiation hints:
  - `can_start`
  - `start_mode`
  - `execution_mode`
  - `latest_runner_event`
- Kept existing lifecycle and handoff fields such as:
  - `status`
  - `progress_hint`
  - `latest_status_at`
  - `download_ready`
  - `is_expired`
  - `can_refresh`

### Event chain

- Added explicit runner-boundary audit events:
  - `export_job.runner_initiated`
  - `export_job.started`
- Kept generic lifecycle events such as:
  - `export_job.advanced_to_running`
  - `export_job.advanced_to_ready`
  - `export_job.advanced_to_failed`
  - `export_job.advanced_to_cancelled`
  - `export_job.advanced_to_queued`
- Start now appends explicit runner-boundary events first, then the backward-compatible lifecycle transition event.

### OpenAPI

- Version updated from `0.27.0` to `0.28.0`.
- Added the internal placeholder `start` path.
- Synced `ExportJob` schema with start/read-model hints.
- Clarified that start semantics are placeholder runner-initiation only, not a real async execution platform.

## 6. Design Decisions

- Chose a dedicated internal `POST /start` boundary instead of overloading all runner semantics into generic `advance`.
- Kept `advance action=start` for backward compatibility, but normalized it through the same service helper so semantics do not diverge.
- Added `export_job.runner_initiated` and `export_job.started` as explicit execution-boundary events while retaining `export_job.advanced_to_running` for existing lifecycle consumers.
- Kept `can_start` fully derived from lifecycle state so tooling can reason about start affordance without extra persistence.
- Exposed `start_mode` and `execution_mode` as lightweight contract metadata so future real runner replacement can be discussed at the API/document layer before any scheduler lands.

## 7. Correction Notes

- `CURRENT_STATE.md` previously still listed Step 31 as only a candidate; this round advanced repository-truth docs to Step 31 complete.
- Export-center docs that previously implied `queued -> running` was only a generic manual advance were corrected to document the new explicit placeholder runner-initiation boundary.
- OpenAPI and readiness docs were updated so `/start` is marked internal placeholder and not presented as a frontend-ready or real runner platform API.

## 8. Verification

- Ran `gofmt -w` on touched Go files.
- Added/updated service tests covering:
  - explicit `StartJob()` placeholder runner-initiation semantics
  - duplicate-start rejection
  - backward-compatible `advance start` behavior
  - runner-event summary hydration on list/detail
  - unchanged claim/read/refresh lifecycle compatibility after the new start contract
- Ran:
  - `go test ./service/...`

## 9. Risks / Known Gaps

- Placeholder runner-initiation still does not dispatch real background work; it only formalizes the boundary.
- `latest_runner_event` is still derived from the event chain; there is no dedicated runner projection table.
- `POST /v1/export-jobs/{id}/start` is not a job lease, worker claim, scheduler callback, or distributed execution API.
- Real file generation, storage delivery, signed URL issuance, and runner telemetry remain out of scope.

## 10. Suggested Next Step

- Keep real storage and delivery deferred.
- The next narrow phase should either add one small placeholder runner-adapter projection around execution ownership/attempt context or enrich export handoff/list projections, without jumping into real async infrastructure.
