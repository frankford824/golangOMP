# ITERATION_017 - Board Candidate Scan Read-Model Hardening

**Date**: 2026-03-09  
**Scope**: STEP_17

## 1. Goals

- Move broad task-board candidate scan narrowing onto an explicit repo/read-model path.
- Stop building the shared board candidate pool by paging through the generic task-list path.
- Keep board/list/filter/query-template contracts stable while classifying remaining fan-out debt.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 16 complete and OpenAPI `v0.15.2`.
- `docs/phases/PHASE_AUTO_017.md` fixed this round's theme as `board candidate scan read-model hardening`.
- Main gaps before implementation:
  - board candidate scans still depended on paged list/read-model collection
  - queue partitioning still happened mostly in service memory after a broad pool was loaded
  - remaining fan-out points were not yet clearly labeled as business-required versus later-optimizable

## 3. Files Changed

### Code

- `repo/interfaces.go`
- `repo/mysql/task.go`
- `service/task_board_service.go`
- `service/task_board_service_test.go`
- `service/task_query.go`
- `service/task_service.go`
- `service/task_prd_service_test.go`
- `service/task_step04_service_test.go`

### Documents

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_017.md`
- `docs/phases/PHASE_AUTO_017.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- No new table.
- No migration added in this round.
- Step 17 hardens query execution only; it does not add indexes, materialized views, or persistence for ownership/preferences.

## 5. API Changes

### External contract

- No public queue payload rename.
- No public filter-field rename for:
  - `GET /v1/tasks`
  - `GET /v1/task-board/summary`
  - `GET /v1/task-board/queues`
- `queue_key`, `filters`, `normalized_filters`, `query_template`, `count`, and sample-task/task-list payloads remain stable.

### Execution changes behind the stable contract

- Added an explicit repo/read-model-backed board candidate scan entry that accepts:
  - shared global board/list filters
  - the union of selected preset queue predicates
- Task-board summary and task-board queues now obtain their shared candidate pool from that direct candidate scan instead of paging through `/v1/tasks`.
- Remaining service-memory partitioning is now limited to business-required final queue shaping:
  - overlapping preset queue membership
  - stable per-queue counts
  - stable sample-task selection
  - per-queue pagination slicing

### Fan-out classification

- Business-required fan-out:
  - final per-queue partitioning after the shared candidate scan
  - overlapping queue membership resolution under stable preset semantics
  - sample-task and paginated queue slicing per preset
- Later-optimizable fan-out:
  - SQL expression cost for projected workflow/procurement predicates
  - broad candidate-scan execution cost when the selected board still covers many presets
  - any future index/materialized-view work

### OpenAPI

- Version updated from `0.15.2` to `0.15.3`.
- Documented that Step 17 hardens board candidate scan execution while keeping the public task-board contract unchanged.

## 6. Design Decisions

- Added repo-level board candidate scan support instead of introducing a second board-only state model.
- Used preset-union predicate pushdown to narrow board candidates before service partitioning.
- Kept final queue shaping in service memory because preset queues intentionally overlap and still need stable sample/list rendering semantics.
- Deferred real index/materialized-view work until the narrowed candidate-scan boundary is explicit and documented.

## 7. Correction Notes

- Corrected repository-truth docs that still described broad board scans as paged list-path collection after the code moved to a dedicated board candidate scan path.
- Advanced OpenAPI from `v0.15.2` to `v0.15.3` with implementation-note-only updates because the public contract did not change.

## 8. Verification

- Added service coverage proving task-board requests now call one board candidate scan path instead of the generic paged list collector.
- Added service coverage proving board candidate scans receive preset-union narrowing and still preserve global converged filters.
- Ran:
  - `gofmt -w repo/interfaces.go repo/mysql/task.go service/task_service.go service/task_query.go service/task_board_service.go service/task_board_service_test.go service/task_prd_service_test.go service/task_step04_service_test.go`
  - `go test ./service/...`
  - `go test ./...`

## 9. Ready for Frontend

- `GET /v1/tasks`
- `GET /v1/task-board/summary`
- `GET /v1/task-board/queues`

These remain frontend-ready. Step 17 only hardens the board candidate scan substrate and fan-out classification behind the existing contract.

## 10. Risks / Known Gaps

- Board candidate scans are now narrower and repo-backed, but still rely on derived SQL expressions rather than dedicated indexes or materialized views.
- Final queue partitioning is still performed in service memory by design because preset queues can overlap; this remains a business-required fan-out point.
- This round does not add real auth, ERP, NAS/upload, queue ownership persistence, saved preferences, or strict `whole_hash` validation.

## 11. Suggested Next Step

- Measure and classify whether any remaining board candidate scan hotspots justify dedicated indexes/materialized projections, while still keeping ownership persistence, saved preferences, and personal inbox storage out of scope until that evidence is clear.
