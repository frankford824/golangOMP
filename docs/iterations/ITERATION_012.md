# ITERATION_012 - Procurement-Warehouse Coordination Deepening / Arrival-to-Warehouse Handoff

**Date**: 2026-03-09  
**Scope**: STEP_12

## 1. Goals

- Tighten `purchase_task` warehouse handoff so procurement completion, not just procurement activity, unlocks warehouse prepare.
- Add a stable procurement-to-warehouse coordination summary across task list, read model, and aggregate detail queries.
- Keep SQL-projected procurement semantics aligned with the service-layer workflow contract.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration still marked Step 11 as the latest completed round.
- `docs/api/openapi.yaml` was at `v0.12.0`.
- Latest completed iteration before this round: `docs/iterations/ITERATION_011.md`.
- Main gaps before implementation:
  - purchase-task warehouse handoff was still effectively open to `prepared|in_progress|completed`
  - `procurement_summary` did not express arrival, warehouse readiness, or handoff state explicitly
  - query-layer procurement projection and service-layer coordination semantics were not explicit enough for frontend consumption

## 3. Files Changed

### Code

- `domain/enums_v7.go`
- `domain/procurement.go`
- `repo/mysql/task.go`
- `service/procurement_summary.go`
- `service/task_detail_service.go`
- `service/task_detail_service_test.go`
- `service/task_prd_service_test.go`
- `service/task_service.go`
- `service/task_workflow.go`

### Documents

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_012.md`
- `docs/phases/PHASE_AUTO_012.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- No new table.
- No migration added in this round.
- Procurement-to-warehouse coordination stays derived from existing:
  - `tasks`
  - `task_details`
  - `procurement_records`
  - `warehouse_receipts`

## 5. API Changes

### Query contract changes

- `procurement_summary` on `GET /v1/tasks`, `GET /v1/tasks/{id}`, and `GET /v1/tasks/{id}/detail` now additionally exposes:
  - `coordination_status`
  - `coordination_label`
  - `warehouse_status`
  - `warehouse_prepare_ready`
  - `warehouse_receive_ready`

### Procurement / warehouse coordination changes

- Purchase-task coordination summary now explicitly distinguishes:
  - `awaiting_arrival`
  - `ready_for_warehouse`
  - `handed_to_warehouse`
- Purchase-task procurement sub-status semantics are tightened to:
  - `in_progress -> pending_inbound / Awaiting arrival`
  - `completed -> ready / Ready for warehouse`
- `POST /v1/tasks/{id}/warehouse/prepare` for `purchase_task` now requires procurement completion before handoff.

### Compatibility

- `close` / `closable` / `cannot_close_reasons` contracts remain intact.
- Procurement lifecycle endpoint surface remains unchanged:
  - `PATCH /v1/tasks/{id}/procurement`
  - `POST /v1/tasks/{id}/procurement/advance`

## 6. Design Decisions

- Kept procurement-to-warehouse coordination derived instead of adding a new persisted coordination table or status column.
  - This keeps scope small and avoids inflating the procurement module before the repository needs a fuller inbound model.
- Tightened warehouse prepare to procurement completion only.
  - `prepared` and `in_progress` are now treated as upstream procurement stages, not warehouse-ready stages.
- Extended `procurement_summary` instead of inventing a second summary object.
  - The existing task list/read/detail contract already carried procurement context; extending it is the smallest stable frontend contract.

## 7. Correction Notes

- Corrected state drift: `CURRENT_STATE.md` and handover docs still implied the Step 11 warehouse gate was the current intended contract.
- Corrected OpenAPI drift: `procurement_summary` was documented without the new coordination/readiness fields.
- Corrected query drift: MySQL projected procurement sub-status now aligns with the tightened service-layer coordination semantics.

## 8. Verification

- Updated tests for:
  - purchase-task warehouse prepare only after procurement completion
  - purchase-task awaiting-arrival blocking reasons
  - task list procurement coordination summary shaping
  - task-detail available actions under the tighter warehouse gate
- Ran:
  - `gofmt -w ...` on touched Go files
  - `go test ./...`

## 9. Ready for Frontend

- Updated `procurement_summary` contract on:
  - `GET /v1/tasks`
  - `GET /v1/tasks/{id}`
  - `GET /v1/tasks/{id}/detail`
- Updated purchase-task coordination semantics on:
  - `POST /v1/tasks/{id}/warehouse/prepare`

## 10. Risks / Known Gaps

- Frontend clients that assumed procurement `prepared|in_progress` already meant warehouse-ready must switch to the new summary/readiness fields.
- Coordination is still derived, so future SQL or workflow changes must keep repo and service logic aligned.
- The system still does not model a full inbound receipt lifecycle, supplier settlement, or ERP procurement integration.

## 11. Suggested Next Step

- Add task-board presets / counters around projected workflow and procurement coordination states, or add a minimal warehouse-side inbox view keyed by derived coordination status without expanding into real ERP integration.
