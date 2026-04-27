# PHASE_AUTO_026 - Export Center Skeleton

## Why This Phase Now
- Step 25 already stabilized `product_selection` as a first-class read-model contract across task list, task board, procurement summary, and task read/detail views.
- The next PRD-aligned gap is no longer query visibility; it is the lack of a unified export carrier that can persist "what the user wants to export" from already-stable result sets.
- Repository truth and current code both show multiple frontend-ready read models but no shared export-job boundary, so this phase should add a minimal export center skeleton before any future file storage or reporting expansion.

## Current Context
- `CURRENT_STATE.md` currently reports Step 25 complete and OpenAPI `v0.22.0`.
- Stable exportable sources already exist through:
  - `GET /v1/tasks`
  - `GET /v1/task-board/summary`
  - `GET /v1/task-board/queues`
  - purchase-facing `procurement_summary`
  - warehouse receipt list read model
- Current main gap:
  - there is no `export_jobs` persistence or export-center API skeleton
  - frontend cannot turn current list/board/filter state into a durable export task
  - there is no formal `result_ref` contract to represent placeholder export output references

## Goals
- Add an export-center skeleton that persists minimal export jobs without introducing real file generation or storage integration.
- Reuse existing stable task/list/board/procurement/warehouse filter contracts as export sources instead of inventing a parallel reporting query language.
- Expose minimal frontend-usable export job APIs so current result views can be converted into durable export requests.

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
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Forbidden Scope
- Real file generation or real file storage integration
- NAS, upload, download-agent, or object-storage integration
- Full export/report template engine
- Full async scheduling / orchestration platform
- BI, KPI, finance, or ERP reporting modules
- Real auth / permission trimming
- Contract regressions to existing task list / board / procurement / warehouse / `product_selection` read models

## Expected File Changes
- Add one export-center domain model family for:
  - export job
  - export result reference
  - source query metadata
  - optional static template catalog metadata
- Add one migration for `export_jobs`.
- Add repo/service/handler/router wiring for minimal export job create/list/get flows.
- Add tests validating source-filter normalization and persisted export-job read contracts.
- Sync phase, iteration, state, OpenAPI, and handover documents.

## Required API / DB Changes
- API:
  - `POST /v1/export-jobs`
  - `GET /v1/export-jobs`
  - `GET /v1/export-jobs/{id}`
  - optional minimal template/catalog read endpoint only if it stays clearly skeleton-scoped
- DB:
  - add `export_jobs`
  - `export_templates` table is not required in this phase if a static catalog is sufficient

## Success Criteria
- Frontend can persist an export job from stable list/board/procurement/warehouse query state without needing real file output.
- Export jobs clearly capture:
  - export type
  - source query type
  - source filters
  - normalized filters / query template when applicable
  - requested-by identity
  - status
  - placeholder `result_ref`
- Existing ready-for-frontend list/board/procurement/warehouse contracts stay backward-compatible.
- `docs/api/openapi.yaml`, `CURRENT_STATE.md`, and `docs/iterations/ITERATION_026.md` are synchronized.
- `go test ./...` passes.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_026.md`
- `docs/api/openapi.yaml`

Expected in this round:
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- Export-source reuse must stay aligned with existing task/board filter semantics; otherwise frontend may persist jobs that cannot be replayed consistently later.
- `result_ref` must be described carefully as a placeholder reference only, or consumers may mistake it for a real downloadable file integration.
- Adding export-center APIs must not imply that a full reporting platform, permission model, or async export runner already exists.

## Completion Output Format
1. Phase path
2. Changed files
3. DB / migration changes
4. API / OpenAPI changes
5. Auto-correction notes
6. Verification
7. Risks / remaining gaps
8. Next recommended phase
