# PHASE_AUTO_019 - Lightweight Queue Ownership Hint / Saved Workbench Preferences

## Why This Phase Now
- Step 18 already classified candidate-scan cost and concluded that broad index or materialized-view work should stay deferred by default.
- `board/list/filter/query_template` contracts are now stable enough to support a workbench-usage layer without reopening queue semantics.
- The most useful next increment is frontend usability: let preset queues expose lightweight ownership guidance and let the workbench restore actor-scoped defaults without introducing real auth or heavy inbox persistence.

## Current Context
- `CURRENT_STATE.md` before this round reports Step 18 complete and OpenAPI `v0.15.4`.
- Task-board aggregation is already frontend-ready through:
  - `GET /v1/task-board/summary`
  - `GET /v1/task-board/queues`
- Task-board queue drill-down contracts are stable and must stay stable:
  - `queue_key`
  - `filters`
  - `normalized_filters`
  - `query_template`
- Placeholder request actor headers already exist:
  - `X-Debug-Actor-Id`
  - `X-Debug-Actor-Roles`
- Current gaps:
  - preset queues do not yet expose lightweight ownership guidance
  - frontend lacks one direct API to read/save actor-scoped workbench preferences
  - docs still describe saved preferences as not yet present

## Goals
- Add lightweight, non-enforced ownership hint metadata to preset task-board queues.
- Add actor-scoped saved workbench preferences APIs for frontend restore/bootstrap.
- Keep existing board/list/query semantics stable and do not introduce real permission trimming or queue ownership persistence.

## Allowed Scope
- `cmd/`
- `db/migrations/`
- `domain/`
- `repo/`
- `service/`
- `transport/`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/api/openapi.yaml`
- `docs/iterations/`
- `docs/phases/`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Forbidden Scope
- Real authentication or real permission enforcement
- Real ERP integration
- NAS / real upload integration
- Strict `whole_hash` verification
- Full queue ownership persistence or personal inbox system
- Heavy index or materialized-view redesign
- BI / KPI / finance / export-center real modules

## Expected File Changes
- Add one lightweight workbench-preferences persistence table and repo.
- Extend preset queue metadata with ownership hint fields while preserving existing queue/filter/query contracts.
- Add `GET /v1/workbench/preferences` and `PATCH /v1/workbench/preferences`.
- Add tests covering queue hints and placeholder-actor-scoped preference save/load behavior.
- Sync state, phase, iteration, OpenAPI, handover, and V7 appendix/spec docs to Step 19.

## Required API / DB Changes
- API:
  - extend task-board queue payloads with optional ownership hint metadata
  - add `GET /v1/workbench/preferences`
  - add `PATCH /v1/workbench/preferences`
- DB / migration:
  - add one lightweight `workbench_preferences` table keyed by placeholder actor scope
  - no real ownership or inbox table system

## Success Criteria
- Preset queues return lightweight ownership hint fields without changing existing board/list/query semantics.
- Frontend can read one workbench bootstrap payload containing:
  - actor placeholder context
  - saved preferences
  - workbench config / queue metadata
- Frontend can patch saved preferences by placeholder actor scope.
- OpenAPI explicitly marks queue ownership data as hint-only and documents placeholder actor scoping for preferences.
- `go test ./...` passes.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_019.md`
- `docs/api/openapi.yaml`

Optional but expected for this round:
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- Queue ownership hint metadata could be misread as real permission control if the docs are not explicit.
- Placeholder actor scoping can only provide lightweight saved preferences, not real user isolation.
- Workbench preferences must not drift into a parallel query contract separate from existing task-board and task-list filters.

## Completion Output Format
1. Changed Files
2. DB / Migration Changes
3. API / OpenAPI Changes
4. Auto-Correction Notes
5. Verification
6. Risks / Remaining Gaps
7. Next Step
