# ITERATION_030 - Export Handoff Expiry / Refresh Semantics

**Date**: 2026-03-10  
**Scope**: PHASE_AUTO_030

## 1. Goals

- Enforce `expires_at` as active placeholder handoff behavior instead of passive metadata.
- Add a minimal refresh action for expired ready export jobs.
- Keep export lifecycle, `result_ref`, claim/read, and audit-trace contracts backward compatible while extending them with expiry/refresh rules.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 29 complete and OpenAPI `v0.26.0`.
- Export center already had:
  - export job persistence
  - lifecycle statuses
  - structured placeholder `result_ref`
  - lifecycle audit trace through `GET /v1/export-jobs/{id}/events`
  - placeholder claim/read boundary through `POST /v1/export-jobs/{id}/claim-download` and `GET /v1/export-jobs/{id}/download`
- The main remaining gap was:
  - `expires_at` existed but did not yet control claim/read behavior
  - no stable refresh boundary after expiry
  - no explicit expiry/refresh events in `export_job_events`
- This round remained out of scope for:
  - real file generation
  - real file-byte download
  - signed URLs
  - NAS / object storage
  - full async runner / scheduler platform
  - BI / KPI / finance export expansion

## 3. Files Changed

### Code

- `domain/export_center.go`
- `domain/export_job_event.go`
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
- `docs/iterations/ITERATION_030.md`
- `docs/phases/PHASE_AUTO_030.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- No DB migration in this round.
- Placeholder handoff expiry/refresh continues to reuse:
  - existing `export_jobs`
  - existing `export_job_events`
- No new download-claim, token, or storage table was introduced.

## 5. API Changes

### `POST /v1/export-jobs/{id}/refresh-download`

- Added a frontend-ready placeholder refresh action for expired ready export jobs.
- Purpose:
  - renew expired placeholder handoff
  - rotate `result_ref.ref_key`
  - extend `expires_at`
  - return the refreshed structured handoff metadata
- This endpoint does not generate files, return file bytes, mint signed URLs, or connect to NAS/object storage.

### `POST /v1/export-jobs/{id}/claim-download`

- Kept the existing ready-job claim boundary.
- Tightened semantics:
  - `ready` + not expired => allowed
  - `ready` + expired => rejected with placeholder-expired invalid-state semantics
  - non-ready => still rejected

### `GET /v1/export-jobs/{id}/download`

- Kept the existing placeholder handoff read boundary.
- Tightened semantics:
  - `ready` + not expired => allowed
  - `ready` + expired => rejected with placeholder-expired invalid-state semantics
  - non-ready => still rejected

### Export-job list/detail read model

- Added lightweight expiry/refresh hints:
  - `is_expired`
  - `can_refresh`
- Kept existing lifecycle fields such as:
  - `status`
  - `progress_hint`
  - `latest_status_at`
  - `download_ready`

### Handoff response contract

- Extended the existing placeholder handoff response with:
  - `is_expired`
  - `can_refresh`
- Existing claim/read availability flags now reflect expiry:
  - `claim_available`
  - `read_available`

### Event chain

- Added placeholder handoff expiry/refresh events:
  - `export_job.download_expired`
  - `export_job.download_refreshed`
- Refresh still writes `export_job.result_ref_updated` because the handoff reference materially changes.

### OpenAPI

- Version updated from `0.26.0` to `0.27.0`.
- Added the `refresh-download` path.
- Synced `ExportJob` and `ExportJobDownloadHandoff` schemas with expiry/refresh read fields.
- Clarified that expiry/refresh are placeholder handoff lifecycle semantics only.

## 6. Design Decisions

- Kept `download_ready` as lifecycle readiness, not active-access readiness, so expired ready jobs still remain distinguishable from non-ready jobs.
- Chose refresh to be allowed only for expired ready handoff, which keeps churn down and makes refresh semantics explicit.
- Rotated `result_ref.ref_key` on refresh so a refreshed handoff is a new placeholder reference, not a silent TTL extension on the old one.
- Scoped event-derived claim/read audit context to the current `result_ref.ref_key`, so old claim/read activity does not leak into refreshed handoff responses.
- Reused `export_job_events` for expiry and refresh instead of adding a second audit or token table.

## 7. Correction Notes

- No pre-existing code/OpenAPI mismatch was found that required reverting Step 29 behavior.
- Repository-truth docs were advanced from Step 29 / `ITERATION_029` / OpenAPI `v0.26.0` to Step 30 / `ITERATION_030` / OpenAPI `v0.27.0`.
- Export-center docs that previously described `expires_at` as metadata-only were corrected to describe the new enforced placeholder lifecycle semantics.

## 8. Verification

- Ran `gofmt -w` on touched Go files.
- Added/updated service tests covering:
  - non-expired ready claim/read
  - expired handoff rejection
  - single-current-ref `download_expired` emission
  - refresh rejection before expiry
  - refresh success after expiry with `ref_key` rotation and new expiry
  - post-refresh claim using the new handoff reference
- Ran:
  - `go test ./service/...`

## 9. Risks / Known Gaps

- Placeholder handoff expiry is still detected lazily during access/refresh attempts; there is no background expirer or scheduler in this phase.
- `GET /v1/export-jobs/{id}/download` still returns metadata only; there is no real byte-stream delivery behind it.
- Refresh remains placeholder-only:
  - not signed URL renewal
  - not NAS/object-storage reissue
  - not runner re-execution
- Claim/read audit context is still event-derived; there is still no dedicated handoff projection table.

## 10. Suggested Next Step

- Keep real storage and download delivery deferred.
- The next narrow phase should either harden export-runner initiation boundaries over the existing export job lifecycle or add one small placeholder projection for richer handoff/list audit context, without jumping into real file services.
