# PHASE_AUTO_025 - Product Selection Read-Model Integration

## Why This Phase Now
- Step 24 already completed the mapped local ERP product-search handoff into task-side persistence.
- The main remaining frontend gap is no longer task-side write support; it is inconsistent read-model consumption of `product_selection` across list, board, procurement summary, and detail views.
- Repository truth already identifies frontend/detail consumption hardening as the next reasonable phase while real ERP integration remains deferred.

## Current Context
- `CURRENT_STATE.md` currently reports Step 24 complete and OpenAPI `v0.21.0`.
- `POST /v1/tasks` and `PATCH /v1/tasks/{id}/business-info` already accept additive `product_selection`.
- `GET /v1/tasks/{id}` and `GET /v1/tasks/{id}/detail` already expose full task-side `product_selection` provenance.
- Current main gap:
  - `GET /v1/tasks` / task-board list consumers still do not get a stable `product_selection` summary contract.
  - `procurement_summary` does not yet explicitly carry stable product-selection provenance.
  - OpenAPI does not yet clearly separate summary read models from full provenance read models.

## Goals
- Make `product_selection` a first-class read-model object across task list, task board, task detail, and procurement summary.
- Keep naming stable by reusing one `product_selection` field family while distinguishing summary vs full provenance schemas.
- Let frontend consumers directly render original-product provenance without reconstructing it from scattered `matched_*` / `source_*` fields.

## Allowed Scope
- `domain/`
- `repo/`
- `service/`
- `transport/`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/`
- `docs/phases/`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Forbidden Scope
- Real ERP API integration or real-time ERP validation
- ERP sync enhancement beyond current local synced-data usage
- NAS / real upload implementation
- Strict `whole_hash` verification
- Full second/third-level category tree implementation
- Full search engine or indexing redesign
- Real auth / RBAC enforcement
- Finance / BI / export-center real modules

## Expected File Changes
- Add one explicit `product_selection` summary read-model type alongside the existing full provenance object.
- Extend task list / board candidate projections to hydrate `product_selection` summary from persisted task-detail provenance.
- Extend `procurement_summary` to expose a stable nested `product_selection` summary.
- Update tests covering list, board, read, and procurement-summary product-selection exposure.
- Sync phase, iteration, state, OpenAPI, and handover documents.

## Required API / DB Changes
- API:
  - `GET /v1/tasks` should return stable `product_selection` summary fields on each task item.
  - `GET /v1/task-board/summary` and `GET /v1/task-board/queues` should inherit the same task-item `product_selection` summary contract.
  - `GET /v1/tasks/{id}` and `GET /v1/tasks/{id}/detail` should keep full `product_selection` provenance.
  - `procurement_summary` should explicitly expose product-selection summary when task-side provenance exists.
- DB:
  - No new migration in this phase; reuse Step 24 persisted task-detail provenance fields.

## Success Criteria
- `product_selection` is exposed consistently across list, board, procurement summary, and detail/read-model layers.
- Summary read models keep stable naming and avoid forcing frontend-side provenance reconstruction.
- Full detail/read-model provenance remains backward-compatible and does not regress mapped-search traceability.
- `docs/api/openapi.yaml`, `CURRENT_STATE.md`, and `docs/iterations/ITERATION_025.md` are synchronized.
- `go test ./...` passes.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_025.md`
- `docs/api/openapi.yaml`

Expected in this round:
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- Summary/full layering must stay clear enough that frontend does not mistake local provenance for real ERP validation.
- List/board queries must remain additive and should not regress existing filter/read-model behavior.
- Procurement summary is purchase-focused, so product-selection exposure there must stay lightweight rather than duplicate full detail provenance.

## Completion Output Format
1. Phase path
2. Changed files
3. DB / migration changes
4. API / OpenAPI changes
5. Auto-correction notes
6. Verification
7. Risks / remaining gaps
8. Next recommended phase
