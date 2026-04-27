# ITERATION_013 - Task Board / Inbox Aggregation for Role-Based Collaboration

**Date**: 2026-03-09  
**Scope**: STEP_13

## 1. Goals

- Add frontend-ready task-board / inbox aggregate APIs over projected workflow and procurement coordination state.
- Provide preset role queues, queue counts, queue conditions, and queue task pages for workbench construction.
- Keep the existing task list/detail/read-model, close flow, and procurement/warehouse coordination contracts stable.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration still marked Step 12 as the latest completed round.
- `docs/api/openapi.yaml` was at `v0.13.0`.
- Latest completed iteration before this round: `docs/iterations/ITERATION_012.md`.
- Main gaps before implementation:
  - no board / inbox aggregation endpoint over `workflow.main_status`, `workflow.sub_status`, and `procurement_summary.coordination_status`
  - no role-oriented preset queues or queue counters for operations, designer, audit, procurement, and warehouse workbenches
  - frontend still had to compose generic `/v1/tasks` list data into workbench task pools on its own

## 3. Files Changed

### Code

- `cmd/server/main.go`
- `domain/task_board.go`
- `service/task_board_service.go`
- `service/task_board_service_test.go`
- `transport/handler/task_board.go`
- `transport/http.go`

### Documents

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_013.md`
- `docs/phases/PHASE_AUTO_013.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- No new table.
- No migration added in this round.
- Task-board data stays derived from existing:
  - `tasks`
  - `task_details`
  - `procurement_records`
  - `warehouse_receipts`
  - projected `workflow` / `procurement_summary`

## 5. API Changes

### New frontend-ready aggregate APIs

- `GET /v1/task-board/summary`
- `GET /v1/task-board/queues`

### Board contract additions

- Added stable board queue metadata:
  - `queue_key`
  - `queue_name`
  - `queue_description`
  - `filters`
  - `count`
- Added stable board filter-definition fields:
  - `task_types`
  - `main_statuses`
  - `sub_status_scope`
  - `sub_status_codes`
  - `coordination_statuses`
  - `warehouse_blocking_reason_codes`
- `GET /v1/task-board/summary` now returns sample tasks per queue.
- `GET /v1/task-board/queues` now returns paginated tasks per queue.

### Preset queues added

- `ops_pending_material`
- `design_pending_submit`
- `audit_pending_review`
- `procurement_pending_followup`
- `awaiting_arrival`
- `warehouse_pending_prepare`
- `warehouse_pending_receive`
- `pending_close`

## 6. Design Decisions

- Implemented task-board aggregation on top of the existing `TaskService.List` contract.
  - This keeps `workflow` and `procurement_summary` as the only workflow source of truth instead of inventing a second derivation path.
- Kept task-board queues preset-derived rather than adding persisted inbox tables.
  - This matches the requested scope and avoids pretending real queue ownership or auth trimming already exists.
- Exposed queue filter definitions directly in the API.
  - This gives frontend workbenches both display metadata and a stable contract for queue semantics without reverse-engineering server rules.

## 7. Correction Notes

- Corrected state drift: `CURRENT_STATE.md` still reported Step 12 complete and only listed task-board work as a candidate.
- Corrected OpenAPI drift: `docs/api/openapi.yaml` did not yet describe the new task-board aggregate endpoints or Step 13 readiness.
- Corrected handover drift: `MODEL_HANDOVER.md` and the V7 appendix still pointed to task-board aggregation as the next step instead of current repository reality.

## 8. Verification

- Added service tests for:
  - procurement board queue counts
  - designer queue pagination
  - invalid board-view validation
- Ran:
  - `gofmt -w domain/task_board.go service/task_board_service.go service/task_board_service_test.go transport/handler/task_board.go transport/http.go cmd/server/main.go`
  - `go test ./...`

## 9. Ready for Frontend

- `GET /v1/task-board/summary`
- `GET /v1/task-board/queues`

These are frontend-facing aggregate APIs, not internal placeholders.

## 10. Risks / Known Gaps

- Queue semantics are preset-derived and intentionally minimal; they are not yet user-owned inboxes.
- Board aggregation currently favors stable workbench presets over high-volume analytical querying.
- There is still no real auth/RBAC trimming, ERP procurement integration, NAS upload, or inbound receipt expansion in this round.

## 11. Suggested Next Step

- Converge more list-side filters and role-oriented workbench semantics with the new task-board presets, or add lightweight queue ownership hints without expanding into real auth or ERP integration.
