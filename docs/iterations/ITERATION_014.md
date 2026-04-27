# ITERATION_014 - Task List / Task-Board Filter Convergence

**Date**: 2026-03-09  
**Scope**: STEP_14

## 1. Goals

- Converge task list and task-board onto one shared task query filter contract.
- Make preset queue filters directly reusable by `/v1/tasks`.
- Add stable board-to-list handoff metadata while keeping existing workflow and procurement contracts backward compatible.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 13 complete and marked filter convergence as the next candidate.
- `docs/api/openapi.yaml` was at `v0.14.0`.
- Main gaps before implementation:
  - task list and task-board filters still used separate shapes
  - preset queue filters were not directly consumable by `/v1/tasks`
  - board/list filtering risked drifting if `workflow` or `procurement_summary` semantics changed

## 3. Files Changed

### Code

- `domain/task_board.go`
- `service/task_service.go`
- `service/task_query.go`
- `service/task_board_service.go`
- `service/task_board_service_test.go`
- `service/task_prd_service_test.go`
- `transport/handler/task.go`
- `transport/handler/task_board.go`
- `transport/handler/task_filters.go`

### Documents

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_014.md`
- `docs/phases/PHASE_AUTO_014.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- No new table.
- No migration added in this round.
- Filter convergence stays on top of existing derived task, workflow, procurement, and warehouse projections.

## 5. API Changes

### `/v1/tasks`

- Converged board/list query dimensions now include:
  - `coordination_status`
  - `warehouse_prepare_ready`
  - `warehouse_receive_ready`
  - `warehouse_blocking_reason_code`
- The following query params now support multi-value semantics for board/list convergence:
  - `status`
  - `task_type`
  - `source_mode`
  - `main_status`
  - `sub_status_code`
  - `coordination_status`
  - `warehouse_blocking_reason_code`

### `/v1/task-board/summary` and `/v1/task-board/queues`

- Queue metadata now additionally exposes:
  - `normalized_filters`
  - `query_template`
- Queue filters now reuse the same task query field names and semantics as `/v1/tasks`.
- Board request parsing now reuses the task-list query parser instead of maintaining a second filter entrypoint.

## 6. Design Decisions

- Kept task-board queue presets as derived definitions, but moved them onto the same filter contract consumed by `/v1/tasks`.
- Used one service-layer matcher for board/list convergence over:
  - `workflow.main_status`
  - `workflow.sub_status`
  - `procurement_summary` coordination semantics
  - warehouse readiness and blocking reasons
- Preserved the existing raw repo query path for simple list filters, and only fan out into derived matching when convergence filters require it.

## 7. Correction Notes

- Corrected contract drift by replacing board-specific queue filter metadata with a shared task query filter schema.
- Corrected board-to-list drift by adding explicit `query_template` payloads instead of requiring frontend to reverse-engineer queue conditions.
- Corrected parsing drift by making task-board endpoints reuse the same list query parsing helper used by `/v1/tasks`.

## 8. Verification

- Added service coverage for:
  - board queue `query_template` output
  - derived board/list filter matching on task list queries
- Ran:
  - `gofmt -w domain/task_board.go service/task_board_service.go service/task_query.go service/task_service.go service/task_board_service_test.go service/task_prd_service_test.go transport/handler/task.go transport/handler/task_board.go transport/handler/task_filters.go`
  - `go test ./...`

## 9. Ready for Frontend

- `GET /v1/tasks`
- `GET /v1/task-board/summary`
- `GET /v1/task-board/queues`

These remain frontend-ready and now share the converged task query contract for board/list drill-down.

## 10. Risks / Known Gaps

- Some Step 14 filters still rely on service-layer fan-out plus derived matching instead of dedicated indexed SQL predicates.
- Queue presets are still preset-derived only; there is still no real queue ownership, permission trimming, or per-user inbox persistence.
- This round does not add real auth, ERP, NAS, upload, or strict `whole_hash` validation.

## 11. Suggested Next Step

- Add lightweight queue ownership hints or saved workbench preferences, or optimize converged task filtering with more direct read-model predicates without expanding into real auth or ERP integration.
