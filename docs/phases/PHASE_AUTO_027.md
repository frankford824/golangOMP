# PHASE_AUTO_027 - Export Job Lifecycle / Download Handoff Skeleton

## Why This Phase Now
- Step 26 already made export jobs durable, but the current skeleton stops at "created with a static placeholder `result_ref`".
- The next blocking frontend gap is not real file storage. It is the absence of a minimal lifecycle and a stable handoff contract that tells frontend whether an export is created, processing, ready, or failed.
- Repository truth already points to this as the next narrow phase, so this round should complete the minimal closed loop before any NAS, object storage, or runner work.

## Current Context
- `CURRENT_STATE.md` currently reports Step 26 complete and OpenAPI `v0.23.0`.
- Existing frontend-ready export-center APIs:
  - `GET /v1/export-templates`
  - `POST /v1/export-jobs`
  - `GET /v1/export-jobs`
  - `GET /v1/export-jobs/{id}`
- Current main gaps:
  - no minimal lifecycle advancement path for export jobs
  - `result_ref` is still a static placeholder and not a structured download handoff object
  - list/detail read models do not yet expose stable lifecycle hints such as `progress_hint`, `latest_status_at`, or `download_ready`

## Goals
- Add a minimal export job lifecycle with stable status semantics for created, processing, ready, and failed states.
- Add a structured placeholder download-handoff contract through `result_ref` without introducing real file generation or file delivery.
- Expose stable list/detail lifecycle fields so frontend can render one minimal export-center closed loop.

## Allowed Scope
- `db/migrations/`
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
- NAS, upload, download-agent, or object-storage integration
- Full async scheduling / orchestration platform
- Real export runner execution
- BI, KPI, finance, or ERP reporting expansion
- Full template engine or export-template CRUD/versioning
- Real auth / permission trimming

## Expected File Changes
- Add export lifecycle domain contracts and structured placeholder handoff metadata.
- Add one migration if explicit lifecycle timestamps need persistence beyond the Step 26 shape.
- Extend export repo/service/handler/router wiring for lifecycle advancement and enriched read models.
- Add tests covering lifecycle transitions, ready/fail handoff semantics, and list/detail projection fields.
- Sync phase, iteration, state, OpenAPI, and handover documents.

## Required API / DB Changes
- API:
  - keep `GET /v1/export-templates`
  - keep `POST /v1/export-jobs`
  - keep `GET /v1/export-jobs`
  - keep `GET /v1/export-jobs/{id}`
  - add minimal lifecycle advancement endpoint:
    - `POST /v1/export-jobs/{id}/advance`
    - this endpoint must be explicitly marked internal/admin skeleton, not frontend-ready
- DB:
  - if needed, add explicit lifecycle timestamp persistence such as `status_updated_at`
  - do not add real file-storage tables or template-management tables

## Success Criteria
- Export jobs expose a stable minimal lifecycle with statuses at least covering:
  - `queued`
  - `running`
  - `ready`
  - `failed`
- Frontend can read stable lifecycle hints from list/detail:
  - `progress_hint`
  - `latest_status_at`
  - `download_ready`
- Detail responses expose structured placeholder handoff metadata through `result_ref`.
- OpenAPI clearly distinguishes:
  - frontend-ready export query/create APIs
  - internal/admin lifecycle advancement API
  - placeholder handoff metadata versus real file delivery
- `go test ./...` passes.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_027.md`
- `docs/api/openapi.yaml`

Expected in this round:
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- Status renaming from the Step 26 placeholder model must be reconciled cleanly across code, docs, and OpenAPI to avoid contract drift.
- `result_ref` must remain clearly documented as handoff metadata only, or consumers may misread `ready` as proof of real storage integration.
- The internal advance endpoint must not be mistaken for a full async runner or scheduler.

## Completion Output Format
1. Phase path
2. Changed files
3. DB / migration changes
4. API / OpenAPI changes
5. Auto-correction notes
6. Verification
7. Risks / remaining gaps
8. Next recommended phase
