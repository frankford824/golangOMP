# ITERATION_015 - Converged Task Filtering Read-Model Hardening

**Date**: 2026-03-09  
**Scope**: STEP_15

## 1. Goals

- Push converged task filtering closer to direct repo/read-model predicates.
- Remove service-layer segmented fan-out from `/v1/tasks` for the stabilized converged filters.
- Keep external board/list filter fields, `normalized_filters`, and `query_template` contracts stable.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 14 complete and OpenAPI `v0.15.0`.
- `docs/phases/PHASE_AUTO_015.md` fixed this round's theme as `converged task filtering read-model hardening`.
- Main gaps before implementation:
  - converged filters such as `coordination_status` and warehouse-readiness predicates still depended on service-layer fan-out or post-hydration matching
  - board/list filter names had converged, but repo/read-model predicate execution had not
  - future ownership or preference work would otherwise build on unstable filtering internals

## 3. Files Changed

### Code

- `repo/interfaces.go`
- `repo/mysql/task.go`
- `service/task_query.go`
- `service/task_prd_service_test.go`

### Documents

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_015.md`
- `docs/phases/PHASE_AUTO_015.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- No new table.
- No migration added in this round.
- Predicate hardening stays on top of existing task, task-detail, procurement, asset, and warehouse projections.

## 5. API Changes

### External contract

- No public filter-field rename.
- No public filter semantic rollback for:
  - `GET /v1/tasks`
  - `GET /v1/task-board/summary`
  - `GET /v1/task-board/queues`
- `normalized_filters` and `query_template` stay stable.

### Execution changes behind the stable contract

- `/v1/tasks` now pushes converged multi-value filtering directly into repo/read-model predicates for:
  - `status`
  - `task_type`
  - `source_mode`
  - `main_status`
  - `sub_status_code`
  - `coordination_status`
  - `warehouse_blocking_reason_code`
- `/v1/tasks` now pushes derived predicate execution directly into repo/read-model predicates for:
  - `coordination_status`
  - `warehouse_prepare_ready`
  - `warehouse_receive_ready`
  - `warehouse_blocking_reason_code`

### OpenAPI

- Version updated from `0.15.0` to `0.15.1`.
- Documented that Step 15 keeps the public converged filter contract stable while hardening internal execution.

## 6. Design Decisions

- Replaced repo single-value task filters with a shared multi-value filter definition so `/v1/tasks` can execute one normalized repo query instead of segmented service fan-out.
- Mirrored the existing workflow/procurement/warehouse derivation logic in repo SQL expressions for:
  - projected main/sub-status filtering
  - procurement coordination filtering
  - warehouse prepare/receive readiness
  - warehouse blocking-reason-code matching
- Kept service-layer hydration for `workflow` and `procurement_summary`, but removed the previous `/v1/tasks` segmented fetch-and-merge path.
- Left task-board preset aggregation unchanged in topology; this round hardens predicate consistency first, not board aggregation count/index strategy.

## 7. Correction Notes

- Corrected repository-truth documents from Step 14 to Step 15 and updated OpenAPI to `v0.15.1`.
- Corrected handover/spec docs to state that converged task filtering is now repo/read-model-driven for the supported predicates, while public board/list filter contracts remain unchanged.

## 8. Verification

- Added service coverage proving converged list filtering now uses one repo call for derived board/list filters.
- Ran:
  - `gofmt -w repo/interfaces.go repo/mysql/task.go service/task_query.go service/task_prd_service_test.go`
  - `go test ./...`

## 9. Ready for Frontend

- `GET /v1/tasks`
- `GET /v1/task-board/summary`
- `GET /v1/task-board/queues`

These remain frontend-ready. Step 15 changes internal filter execution only; external filter names and baseline semantics stay stable.

## 10. Risks / Known Gaps

- Derived SQL predicates are now closer to the read model, but they are not yet backed by dedicated indexes or materialized views.
- Task-board summary/queue aggregation still iterates preset queues and paged list reads by design.
- This round does not add real auth, ERP, NAS/upload, queue ownership persistence, saved preferences, or strict `whole_hash` validation.

## 11. Suggested Next Step

- Improve task-board aggregation efficiency or introduce dedicated index/read-model support for the converged predicates before layering ownership or preference persistence on top.
