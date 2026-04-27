# ITERATION_029 - Placeholder Download Claim / Read Boundary

**Date**: 2026-03-10  
**Scope**: PHASE_AUTO_029

## 1. Goals

- Add a minimal placeholder download claim/read contract for ready export jobs.
- Let frontend explicitly claim and read structured handoff metadata without introducing real file delivery.
- Reuse the existing export-job event trace for handoff-consumption audit instead of creating a separate log.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 28 complete and OpenAPI `v0.25.0`.
- Export center already had:
  - export job persistence
  - lifecycle statuses
  - structured placeholder `result_ref`
  - lifecycle audit trace through `GET /v1/export-jobs/{id}/events`
- The main remaining gap was:
  - no explicit claim/read boundary for `ready` export jobs
  - no dedicated frontend-facing handoff response contract
  - no event-trace coverage for placeholder download consumption
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
- `docs/iterations/ITERATION_029.md`
- `docs/phases/PHASE_AUTO_029.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- No DB migration in this round.
- Placeholder download claim/read reuses:
  - existing `export_jobs`
  - existing `export_job_events`
- No claim/read state was moved into a new table, and no real storage integration was introduced.

## 5. API Changes

### `POST /v1/export-jobs/{id}/claim-download`

- Added frontend-ready placeholder claim action for ready export jobs.
- Purpose:
  - formally claim placeholder download handoff
  - return structured handoff metadata
  - append `export_job.download_claimed` to the existing event chain
- This endpoint does not return file bytes and does not imply real storage integration.

### `GET /v1/export-jobs/{id}/download`

- Added frontend-ready placeholder read action for ready export jobs.
- Purpose:
  - read structured handoff metadata
  - append `export_job.download_read` to the existing event chain
- This endpoint is a handoff-read boundary only, not a real download service.

### Handoff response contract

- Added dedicated placeholder handoff response shape with at least:
  - `export_job_id`
  - `result_ref`
  - `file_name`
  - `mime_type`
  - `is_placeholder`
  - `expires_at`
  - `download_ready`
  - `note`
- Also exposes latest placeholder access audit context when available:
  - `claimed_at`
  - `claimed_by_actor_id`
  - `claimed_by_actor_type`
  - `last_read_at`
  - `last_read_by_actor_id`
  - `last_read_by_actor_type`

### Ready-state enforcement

- Claim/read are allowed only when:
  - `status=ready`
  - `download_ready=true`
- Non-ready jobs return a clear invalid-state error with current status context.

### OpenAPI

- Version updated from `0.25.0` to `0.26.0`.
- Added schemas for:
  - `ExportJobDownloadHandoff`
  - `ExportJobDownloadHandoffResponse`
- Clarified:
  - claim/read are placeholder handoff actions
  - they are frontend-ready
  - they are not real file-download endpoints

## 6. Design Decisions

- Kept claim/read out of the export-job row model and reused `export_job_events` as the durable handoff audit channel.
- Treated claim and read as separate semantics:
  - `claim` means explicit handoff takeover intent
  - `read` means explicit retrieval of structured handoff metadata
- Returned a dedicated handoff response object instead of forcing frontend to reconstruct handoff semantics from `result_ref` plus event timeline manually.
- Kept `GET /v1/export-jobs/{id}/download` as a placeholder metadata read boundary only and documented it aggressively to avoid confusion with real file delivery.

## 7. Correction Notes

- `docs/V7_MODEL_HANDOVER_APPENDIX.md` still reported outdated repository-truth metadata (`Step 28` / `ITERATION_028`) even though Step 28 outputs were already present.
- This round corrected repository-truth docs and synced:
  - `CURRENT_STATE.md`
  - `MODEL_HANDOVER.md`
  - `docs/V7_API_READY.md`
  - `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
  - `docs/V7_MODEL_HANDOVER_APPENDIX.md`
  - `docs/api/openapi.yaml`
- No code/document mismatch was found that required reverting Step 28 behavior.

## 8. Verification

- Ran `gofmt -w` on touched Go files.
- Added/updated service tests covering:
  - ready export-job claim handoff
  - ready export-job read handoff
  - event-chain reuse for claim/read
  - rejection of claim on non-ready jobs
- Ran:
  - `go test ./service/...`

## 9. Risks / Known Gaps

- `GET /v1/export-jobs/{id}/download` still returns metadata only; there is no real byte-stream delivery behind it.
- Claim/read audit context is event-derived; there is still no dedicated persisted download-claim projection or permission model.
- Real file generation, signed URLs, NAS, object storage, and async runners remain deferred.

## 10. Suggested Next Step

- Keep real storage and download delivery deferred.
- The next narrow phase should harden post-ready placeholder handoff behavior, such as expiry/refresh semantics or a later runner/storage adapter boundary, without expanding into actual file services.
