# PHASE_AUTO_017 - Board Candidate Scan Read-Model Hardening

## Why This Phase Now
- Step 16 removed preset-by-preset task-board list fan-out, but the shared board candidate pool still came from a broad paged list/read-model collection path.
- The latest PRD direction treats board and list as two views over one task-query substrate, so the next safe step is to harden board candidate scans closer to repo/read-model predicates before any ownership, preferences, or inbox persistence.
- Queue ownership persistence, saved preferences, or personal inbox storage would still sit on top of an unnecessarily wide board candidate scan if introduced now.

## Current Context
- `CURRENT_STATE.md` before this round reports Step 16 complete and OpenAPI `v0.15.2`.
- `GET /v1/task-board/summary` and `GET /v1/task-board/queues` are already frontend-ready and must keep their queue/filter/drill-down payloads stable.
- Main gaps before implementation:
  - broad board-wide candidate scans still rely on the shared list/read-model path with paging collection
  - queue partitioning is still mostly performed in service memory after collecting a broad candidate pool
  - remaining fan-out debt is not yet explicitly classified as business-required versus later-optimizable

## Goals
- Push board candidate scan narrowing into a clearer repo/read-model entrypoint instead of collecting the board pool through paged task-list traversal.
- Keep task-board summary and queue rendering on one shared candidate scan entrance while preserving existing board/list query semantics and queue payload contracts.
- Explicitly document which remaining board fan-out is business-required and which parts are later-optimizable.

## Allowed Scope
- `repo/`
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
- Add a repo/read-model-backed board candidate scan entry that accepts shared global filters plus the union of preset queue predicates.
- Refactor task-board aggregation to call the new candidate scan path instead of paged `/v1/tasks` collection.
- Update tests to prove the new path keeps contracts stable and pushes preset-union narrowing down before service partitioning.
- Sync state, phase, iteration, OpenAPI, handover, and V7 appendix/spec docs to Step 17.

## Required API / DB Changes
- API:
  - no public queue payload rename
  - no public filter-field rename for `/v1/tasks`, `/v1/task-board/summary`, or `/v1/task-board/queues`
  - OpenAPI patch-version update plus implementation notes only
- DB / migration:
  - no new table
  - no migration required in this round

## Success Criteria
- Task-board summary and queues no longer build their board candidate pool by paging through the generic task-list path.
- Board candidate narrowing is supported by an explicit repo/read-model entry that applies shared global filters plus preset-union predicates.
- `queue_key`, `filters`, `normalized_filters`, `query_template`, `count`, and sample-task/task-list payloads remain stable.
- Remaining board fan-out is documented as business-required or later-optimizable.
- `go test ./...` passes.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_017.md`
- `docs/api/openapi.yaml`

Optional but expected for this round:
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- The new repo/read-model candidate scan narrows broad board scans, but it still is not a dedicated index or materialized-view solution.
- Final queue partitioning remains in service memory for overlapping queue semantics and stable sample/list shaping; this is deliberate and should not be confused with the removed broad-scan path.

## Completion Output Format
1. Changed Files
2. DB / Migration Changes
3. API / OpenAPI Changes
4. Auto-Correction Notes
5. Verification
6. Risks / Remaining Gaps
7. Next Step
