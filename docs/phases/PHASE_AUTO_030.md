# PHASE_AUTO_030 - Export Handoff Expiry / Refresh Semantics

## Why This Phase Now
- Step 29 already delivered a frontend-ready placeholder claim/read boundary for ready export jobs.
- The main remaining contract gap is no longer how to access handoff metadata, but how that handoff behaves after `expires_at`.
- This is the narrowest next step that keeps export lifecycle and audit semantics coherent before any later runner, NAS, object-storage, or signed-URL work appears.

## Current Context
- `CURRENT_STATE.md` before this phase reported Step 29 complete.
- OpenAPI version before this phase was `0.26.0`.
- Existing export-center APIs:
  - `GET /v1/export-templates`
  - `POST /v1/export-jobs`
  - `GET /v1/export-jobs`
  - `GET /v1/export-jobs/{id}`
  - `GET /v1/export-jobs/{id}/events`
  - `POST /v1/export-jobs/{id}/claim-download`
  - `GET /v1/export-jobs/{id}/download`
  - `POST /v1/export-jobs/{id}/advance` (internal/admin skeleton)
- Existing export-center capabilities:
  - export job persistence
  - lifecycle statuses
  - structured placeholder `result_ref`
  - lifecycle audit trace
  - placeholder claim/read boundary for ready jobs
- Current main gaps:
  - no enforced `expires_at` behavior on claim/read
  - no stable refresh action after expiry
  - no explicit event semantics for expiry/refresh
  - no list/detail read-model hints for expired vs refreshable handoff

## Goals
- Turn `expires_at` into an enforced placeholder handoff lifecycle rule.
- Define stable refresh behavior for expired ready export jobs.
- Keep claim/read/result-ref/event contracts backward compatible while extending them with expiry/refresh semantics.
- Reuse `export_job_events` for expiry and refresh instead of creating any parallel audit record.

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
- Extend export-job read models with explicit expiry/refresh hints.
- Add one minimal refresh endpoint for expired placeholder handoff.
- Enforce ready-and-not-expired access rules on claim/read.
- Append expiry/refresh events into the existing `export_job_events` chain.
- Add tests covering expired rejection, expiry-event behavior, and refresh result-ref rotation.
- Sync phase, iteration, OpenAPI, state, handover, and appendix/spec docs.

## Required API / DB Changes
- API:
  - keep `POST /v1/export-jobs/{id}/claim-download`
  - keep `GET /v1/export-jobs/{id}/download`
  - add `POST /v1/export-jobs/{id}/refresh-download`
  - clarify:
    - ready + not expired => claim/read allowed
    - ready + expired => claim/read rejected with clear placeholder-expired semantics
    - refresh is allowed only for expired ready handoff
    - refresh updates `expires_at`
    - refresh rotates `result_ref.ref_key`
- DB:
  - no new migration is expected in this round
  - reuse existing `export_jobs`
  - reuse existing `export_job_events`

## Success Criteria
- Claim/read are allowed only when placeholder handoff is ready and not expired.
- Expired ready handoff returns a clear invalid-state error and does not silently behave like active handoff.
- Expired ready handoff can be refreshed through one stable API action.
- Refresh writes durable events such as:
  - `export_job.download_expired`
  - `export_job.download_refreshed`
- Material handoff changes from refresh still flow through `export_job.result_ref_updated`.
- List/detail contracts expose enough state to show expiry and refresh affordance without reconstructing it client-side.
- OpenAPI clearly marks the whole boundary as placeholder-only and frontend-ready.
- `go test ./...` passes, or any environment-limited failures are explicitly recorded.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_030.md`
- `docs/api/openapi.yaml`

Expected in this round:
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- If `download_ready` is reinterpreted as a live-access flag instead of a lifecycle readiness flag, frontend could mis-handle expired jobs.
- Refresh that rotates `ref_key` must remain clearly documented as placeholder-handoff renewal, not signed-URL token minting.
- Without a dedicated persisted handoff projection, `claimed_at` / `last_read_at` remain event-derived and now need current-ref scoping after refresh.

## Completion Output Format
1. Phase path
2. Changed files
3. DB / migration changes
4. API / OpenAPI changes
5. Auto-correction notes
6. Verification
7. Risks / remaining gaps
8. Next recommended phase
