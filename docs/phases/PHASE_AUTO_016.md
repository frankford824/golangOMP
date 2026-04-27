# PHASE_AUTO_016 - Task-Board Aggregation Hardening

## Why This Phase Now
- Step 15 pushed converged `/v1/tasks` filtering into repo/read-model predicates, but task-board preset aggregation still fans out queue by queue on top of that stabilized list path.
- The latest PRD direction treats task board and task list as two views over the same task pool, so board summary, board queues, and `/v1/tasks` should share more of the same aggregation substrate before any index or materialized-view work.
- Queue ownership persistence, saved preferences, or personal inbox storage would be premature while board aggregation still duplicates queue-by-queue list collection.

## Current Context
- `CURRENT_STATE.md` before this round reports Step 15 complete and OpenAPI `v0.15.1`.
- `GET /v1/task-board/summary` and `GET /v1/task-board/queues` are already frontend-ready and expose stable queue metadata plus `/v1/tasks` drill-down metadata.
- Main gaps before implementation:
  - preset queues still aggregate through service-layer queue fan-out
  - board summary and board queues share filter language but not enough shared aggregation topology
  - index/read-model optimization would be premature while board aggregation still duplicates queue scans

## Goals
- Collapse task-board preset aggregation from per-queue list fan-out into one shared base candidate pool per board request.
- Make board summary, board queues, and `/v1/tasks` rely on the same filter semantics and read-model-backed task pool as much as possible in the current architecture.
- Keep `query_template`, `normalized_filters`, `filters`, `queue_key`, `queue_name`, `count`, and sample-task/task-list contracts stable.
- Explicitly document any remaining aggregation fan-out that is still business-required or later-optimizable.

## Allowed Scope
- `service/`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/`
- `docs/phases/`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Forbidden Scope
- Real authentication or permission enforcement
- Queue ownership persistence or personal inbox storage
- Saved workbench preferences
- Real ERP integration
- NAS / real upload integration
- Strict `whole_hash` verification
- Large index or materialized-view redesign
- Full BI / KPI / finance / export-center modules

## Expected File Changes
- Refactor task-board aggregation so summary and queues are built from one shared base candidate set instead of preset-by-preset list fan-out.
- Add or update tests proving board aggregation stays contract-stable while reducing duplicate list calls.
- Sync current state, iteration memory, OpenAPI, handover, and V7 appendix/spec docs to Step 16.

## Required API / DB Changes
- API:
  - no external queue payload rename
  - no filter-field rename for `/v1/tasks`, `/v1/task-board/summary`, or `/v1/task-board/queues`
  - OpenAPI patch-version update plus implementation notes only
- DB / migration:
  - no new table
  - no migration required in this round

## Success Criteria
- Task-board summary and queues no longer collect tasks by calling the list path once per preset queue.
- Board summary, board queues, and `/v1/tasks` continue to share the same filter semantics and drill-down contract.
- `normalized_filters`, `filters`, and `query_template` remain stable for current frontend consumers.
- Remaining aggregation debt is explicitly documented as board-wide candidate scan or later index/read-model work.
- `go test ./...` passes.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_016.md`
- `docs/api/openapi.yaml`

Optional but expected for this round:
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- Board aggregation is more consistent after removing preset fan-out, but broad board-wide candidate scans can still be expensive before dedicated index/read-model optimization.
- Queue counts and sample tasks now share one aggregation substrate, so any drift in the shared matcher affects both summary and queues together.

## Completion Output Format
1. Changed Files
2. DB / Migration Changes
3. API / OpenAPI Changes
4. Auto-Correction Notes
5. Verification
6. Risks / Remaining Gaps
7. Next Step
