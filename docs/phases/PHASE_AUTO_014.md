# PHASE_AUTO_014 - Task List / Task-Board Filter Convergence

## Why This Phase Now
- Step 13 delivered frontend-ready task-board aggregate APIs, but board presets still describe queue conditions through a board-specific filter shape instead of the same contract consumed by `/v1/tasks`.
- The latest PRD direction expects task board and task list to act as two views over the same task pool, so preset queue conditions must be reusable for board-to-list drill-down without duplicating business logic in frontend code.
- Current workflow and `procurement_summary` projections are already stable enough to become the single source of truth for both list filtering and board queue definitions.

## Current Context
- `CURRENT_STATE.md` before this round reports Step 13 complete and OpenAPI `v0.14.0`.
- `GET /v1/task-board/summary` and `GET /v1/task-board/queues` are already ready for frontend usage.
- Main gaps before implementation:
  - `/v1/tasks` does not yet expose the full set of filter dimensions used by board presets
  - board queue `filters` cannot be consumed by `/v1/tasks` without endpoint-specific translation
  - preset queues and task list filtering risk drifting if `workflow` or `procurement_summary` semantics change

## Goals
- Converge task list and task-board onto one reusable task query filter contract.
- Make preset queue filters directly consumable by `/v1/tasks`.
- Add stable board-to-list jump metadata so frontend can reuse preset queue filters without rebuilding query logic.
- Keep existing `workflow`, `procurement_summary`, `close`, and `cannot_close_reasons` contracts backward compatible.

## Allowed Scope
- `domain/`
- `service/`
- `transport/`
- `repo/`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/iterations/`
- `docs/phases/`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Forbidden Scope
- Real authentication or permission enforcement
- Queue ownership persistence or personal inbox storage
- Real ERP integration
- NAS / real upload integration
- `whole_hash` strict verification
- Full BI / KPI implementation
- Supplier settlement or full inbound lifecycle expansion

## Expected File Changes
- Introduce a shared task query filter schema reused by `/v1/tasks` and task-board queue metadata.
- Update task list filtering to support board preset dimensions such as procurement coordination and warehouse readiness-related predicates.
- Simplify task-board queue definitions so they reuse the same normalized task query filters and board-to-list query templates.
- Add or update tests for unified filters, queue drill-down reuse, and filter parsing behavior.
- Sync OpenAPI, current state, iteration memory, and handover docs to Step 14.

## Required API / DB Changes
- API:
  - extend `GET /v1/tasks` filter contract to accept board-reusable filter dimensions
  - extend task-board queue responses with normalized filter metadata and task-list query templates
- DB / migration:
  - no new table
  - no migration required; all filtering remains derived from existing task, procurement, asset, and warehouse projections

## Success Criteria
- `/v1/tasks` supports the filter dimensions required by current preset queues without frontend-only translation glue.
- task-board preset responses expose stable normalized filters and task-list query templates for drill-down.
- Board queue semantics reuse the same workflow / procurement-summary filtering logic as task list queries.
- Existing read-model, workflow, procurement-summary, and close contracts do not regress.
- `go test ./...` passes.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_014.md`
- `docs/api/openapi.yaml`

Optional but expected for this round:
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- Some unified filters still require service-layer derived matching because not every dimension maps cleanly to a single SQL predicate.
- Broad multi-value task list queries may be less efficient than simple single-filter queries until dedicated read-model indexing is introduced.
- Future workflow or procurement-summary semantic changes must continue to update the shared filter matcher to preserve board/list convergence.

## Completion Output Format
1. Changed Files
2. DB / Migration Changes
3. API / OpenAPI Changes
4. Auto-Correction Notes
5. Verification
6. Risks / Remaining Gaps
7. Next Step
