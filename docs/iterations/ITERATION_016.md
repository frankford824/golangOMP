# ITERATION_016 - Task-Board Aggregation Hardening

**Date**: 2026-03-09  
**Scope**: STEP_16

## 1. Goals

- Remove preset-by-preset list fan-out from task-board aggregation.
- Make task-board summary, task-board queues, and `/v1/tasks` share a more consistent aggregation substrate.
- Keep external board queue payloads and drill-down contracts stable.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 15 complete and OpenAPI `v0.15.1`.
- `docs/phases/PHASE_AUTO_016.md` fixed this round's theme as `task-board aggregation hardening`.
- Main gaps before implementation:
  - task-board preset queues still aggregated by queue fan-out over the list path
  - board summary and board queues shared filter language but not enough shared aggregation topology
  - index or materialized-view work would be premature while board aggregation still duplicated preset scans

## 3. Files Changed

### Code

- `service/task_board_service.go`
- `service/task_board_service_test.go`

### Documents

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_016.md`
- `docs/phases/PHASE_AUTO_016.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- No new table.
- No migration added in this round.
- Board aggregation hardening stays on top of the existing task read-model/list path and converged predicate substrate.

## 5. API Changes

### External contract

- No public queue payload rename.
- No public filter-field rename for:
  - `GET /v1/tasks`
  - `GET /v1/task-board/summary`
  - `GET /v1/task-board/queues`
- `filters`, `normalized_filters`, and `query_template` remain stable.
- Existing frontend-consumed queue structure remains stable:
  - `queue_key`
  - `queue_name`
  - `filters`
  - `count`
  - sample tasks / queue task lists

### Execution changes behind the stable contract

- Task-board summary and task-board queues now build from one shared board-level candidate task pool per request.
- Preset queues are no longer aggregated by calling the list path once per queue.
- Board aggregation now shares one implementation for:
  - preset filtering
  - counts
  - sample-task selection
  - per-queue paginated task slicing
- Board aggregation continues to rely on the `/v1/tasks`-aligned filter semantics:
  - repo/read-model predicates constrain the base candidate pool
  - preset queues reuse the same filter matcher semantics for final board partitioning

### OpenAPI

- Version updated from `0.15.1` to `0.15.2`.
- Documented that Step 16 keeps the public board contract stable while hardening board aggregation topology behind it.

## 6. Design Decisions

- Reused the already-hardened `/v1/tasks` list path as the shared board candidate source instead of adding a second board-only source of truth.
- Moved task-board summary and task-board queues onto one shared aggregation helper so counts, sample tasks, and queue task lists are partitioned from the same candidate set.
- Kept `filters` and `normalized_filters` preset-shaped to avoid breaking current board-to-list and frontend queue contracts.
- Deferred dedicated index/materialized-view work until after board aggregation topology is fully converged.

## 7. Correction Notes

- Repository-truth documents were aligned from Step 15 to Step 16 and OpenAPI was advanced to `v0.15.2`.
- Known-gap documentation was corrected to reflect that preset queue fan-out is removed, while broad board-wide candidate scans remain.

## 8. Verification

- Added service coverage proving task-board summary and queue aggregation now reuse one base list collection instead of preset-by-preset list calls.
- Added service coverage proving global converged filters still apply across board queues under the shared aggregation path.
- Ran:
  - `gofmt -w service/task_board_service.go service/task_board_service_test.go`
  - `go test ./service/...`
  - `go test ./...`

## 9. Ready for Frontend

- `GET /v1/tasks`
- `GET /v1/task-board/summary`
- `GET /v1/task-board/queues`

These remain frontend-ready. Step 16 changes internal board aggregation topology only; the external queue contract and drill-down semantics stay stable.

## 10. Risks / Known Gaps

- Task-board aggregation no longer fans out preset by preset, but broad board-wide candidate scans still depend on paged list/read-model collection and are not yet backed by dedicated indexes or materialized views.
- Queue partitioning still happens in service memory after the shared candidate pool is loaded; this is a deliberate hardening step, not the final performance shape.
- This round does not add real auth, ERP, NAS/upload, queue ownership persistence, saved preferences, or strict `whole_hash` validation.

## 11. Suggested Next Step

- Add dedicated index/read-model support for broad board-wide candidate scans, and explicitly classify which remaining aggregation work is business-required versus later-optimizable before any ownership or preference persistence.
