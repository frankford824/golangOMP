# PHASE_AUTO_029 - Placeholder Download Claim / Read Boundary

## Why This Phase Now
- Step 28 completed export-job lifecycle audit trace, so export jobs now have a durable event chain that can carry later handoff semantics.
- The next blocking gap is no longer lifecycle visibility; it is the missing frontend-consumable boundary between a `ready` export job and a consumer that wants to receive placeholder download handoff metadata.
- This phase is the narrowest next step that improves frontend integration without expanding into real file generation, storage delivery, signed URLs, NAS, object storage, or a full async platform.

## Current Context
- `CURRENT_STATE.md` reports V7 Step 28 complete.
- OpenAPI version is `0.25.0`.
- Existing export-center APIs:
  - `GET /v1/export-templates`
  - `POST /v1/export-jobs`
  - `GET /v1/export-jobs`
  - `GET /v1/export-jobs/{id}`
  - `GET /v1/export-jobs/{id}/events`
  - `POST /v1/export-jobs/{id}/advance` (internal/admin lifecycle skeleton)
- Existing export-center capabilities:
  - export job persistence
  - lifecycle statuses
  - structured placeholder `result_ref`
  - lifecycle audit trace
- Current main gaps:
  - no dedicated claim/read boundary for `ready` export jobs
  - frontend can inspect `result_ref`, but cannot explicitly claim or read handoff metadata through stable actions
  - no stable contract yet for later runner / NAS / storage integration to plug into

## Goals
- Add a minimal placeholder download claim/read contract for ready export jobs.
- Let frontend explicitly:
  - determine whether a job is claimable/readable through the ready-state boundary
  - read structured handoff metadata from a dedicated response contract
- Reuse the existing export-job event trace for claim/read activity instead of introducing a second audit system.
- Keep lifecycle, `result_ref`, and event-summary contracts backward compatible.

## Allowed Scope
- `domain/`
- `repo/`
- `service/`
- `transport/`
- `cmd/server/main.go`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/`
- `docs/phases/`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Forbidden Scope
- Real file generation
- Real byte-stream download endpoints
- Signed URL delivery
- NAS or object-storage integration
- Full async scheduling / orchestration platform
- BI / KPI / finance export expansion
- Real permission trimming
- Real ERP or other external-system integration

## Expected File Changes
- Add export-job placeholder download handoff read models and event constants.
- Extend export-center service/handler/router with minimal claim/read actions for ready jobs.
- Append claim/read events into the existing `export_job_events` chain.
- Add or update tests covering ready-only claim/read behavior and non-ready rejection cases.
- Sync OpenAPI, current-state, iteration, handover, and V7 appendix/spec documents.

## Required API / DB Changes
- API:
  - add one minimal claim action for ready export jobs
  - add one minimal read action for ready export jobs
  - return a dedicated placeholder handoff object that includes at least:
    - `export_job_id`
    - `result_ref`
    - `file_name`
    - `mime_type`
    - `is_placeholder`
    - `expires_at`
    - `download_ready`
    - `note`
  - clearly document that these actions expose placeholder handoff metadata only, not real file delivery
- DB:
  - no new migration is expected in this round if claim/read can reuse existing export-job rows plus `export_job_events`
  - if any persistence addition becomes necessary, it must remain narrowly scoped to placeholder handoff metadata rather than real storage integration

## Success Criteria
- `ready` export jobs can be explicitly claimed through a stable API contract.
- `ready` export jobs can be explicitly read through a stable API contract.
- non-ready statuses (`queued`, `running`, `failed`, `cancelled`) return clear rejection semantics for claim/read.
- claim/read actions append durable events such as:
  - `export_job.download_claimed`
  - `export_job.download_read`
- claim/read continue to reuse the same `export_job_events` timeline and do not create a parallel audit mechanism.
- OpenAPI clearly marks the new boundary as placeholder-only and frontend-ready.
- `go test ./...` passes, or any environment-limited failures are explicitly recorded.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_029.md`
- `docs/api/openapi.yaml`

Expected in this round:
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- If claim/read semantics are too close to real download semantics, frontend or later integrators may misread placeholder handoff as a live file service.
- Latest-event summaries on export jobs may now reflect download handoff interactions after readiness, so docs must clarify that lifecycle and handoff events share one timeline.
- Without a new persisted claim record, any claim-actor/timestamp fields must be clearly framed as placeholder or event-derived context.

## Completion Output Format
1. Phase path
2. Changed files
3. DB / migration changes
4. API / OpenAPI changes
5. Auto-correction notes
6. Verification
7. Risks / remaining gaps
8. Next recommended phase
