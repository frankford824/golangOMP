# ITERATION_011 - Structured Sub-Status Query Enhancement / Procurement Flow Skeleton

**Date**: 2026-03-09  
**Scope**: STEP_11

## 1. Goals

- Add projected workflow filtering to `GET /v1/tasks` for `main_status` and structured `sub_status_code`.
- Keep `workflow.sub_status` and procurement-facing task contracts aligned across list/read/detail queries.
- Upgrade `purchase_task` procurement from a readiness-only skeleton into a minimal lifecycle flow with a stable action endpoint.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration still listed Step 11 as the next candidate only.
- `docs/api/openapi.yaml` was at `v0.11.0`.
- Latest completed iteration before this round: `docs/iterations/ITERATION_010.md`.
- Main gaps before implementation:
  - no `main_status` / `sub_status_code` filtering on `/v1/tasks`
  - procurement exposed persistence but no minimal lifecycle action flow
  - list query did not expose a stable procurement summary

## 3. Files Changed

### Code

- `db/migrations/008_v7_procurement_flow_skeleton.sql`
- `domain/audit.go`
- `domain/enums_v7.go`
- `domain/procurement.go`
- `domain/query_views.go`
- `domain/task_detail_aggregate.go`
- `repo/interfaces.go`
- `repo/mysql/procurement.go`
- `repo/mysql/task.go`
- `service/procurement_summary.go`
- `service/task_detail_service.go`
- `service/task_detail_service_test.go`
- `service/task_prd_service_test.go`
- `service/task_service.go`
- `service/task_workflow.go`
- `transport/handler/task.go`
- `transport/http.go`

### Documents

- `docs/phases/PHASE_AUTO_011.md`
- `docs/iterations/ITERATION_011.md`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## 4. DB / Migration Changes

- Added `db/migrations/008_v7_procurement_flow_skeleton.sql`.
- Added additive `procurement_records.quantity`.
- Normalized persisted procurement statuses:
  - `preparing -> draft`
  - `ready -> prepared`
- Procurement lifecycle now persists as:
  - `draft`
  - `prepared`
  - `in_progress`
  - `completed`

## 5. API Changes

### Query contract changes

- `GET /v1/tasks` now supports:
  - `main_status`
  - `sub_status_code`
  - `sub_status_scope`
- `GET /v1/tasks`, `GET /v1/tasks/{id}`, and `GET /v1/tasks/{id}/detail` now expose nullable `procurement_summary`.

### Procurement contract changes

- `PATCH /v1/tasks/{id}/procurement` now supports `quantity` and the Step-11 status model.
- Added `POST /v1/tasks/{id}/procurement/advance`.
- Procurement lifecycle actions:
  - `prepare`
  - `start`
  - `complete`
  - `reopen`

### Readiness changes

- Purchase-task warehouse readiness now requires:
  - procurement record
  - procurement price
  - procurement quantity
  - procurement status in `prepared|in_progress|completed`
- Purchase-task close readiness now requires procurement status `completed`.

## 6. Design Decisions

- Kept projected workflow filtering derived instead of introducing a persisted workflow projection table.
  - The repo SQL now mirrors the service-layer workflow derivation closely enough to unlock frontend filtering without a larger schema redesign.
- Added `procurement_summary` instead of exposing full procurement detail everywhere.
  - List pages need a stable compact shape; detail/read paths still keep nullable `procurement`.
- Chose a minimal procurement action endpoint rather than a full procurement module.
  - This adds forward movement and explicit transitions while staying inside the allowed scope.
- Made close stricter than warehouse handoff for purchase tasks.
  - Warehouse handoff allows `prepared|in_progress|completed`; final close requires procurement `completed`.

## 7. Correction Notes

- Corrected OpenAPI drift: task list filtering and procurement status semantics were not documented before this round.
- Corrected state drift: `CURRENT_STATE.md` previously still described procurement as a readiness-only skeleton.
- Corrected contract drift by standardizing `procurement_summary` across list/read/detail instead of leaving the list path without a stable procurement overview.

## 8. Verification

- Added/updated tests for:
  - purchase-task direct warehouse handoff under the new procurement status model
  - procurement draft persistence with quantity
  - procurement lifecycle transitions through `prepare -> start -> complete`
  - task-list filter propagation and procurement summary shaping
  - task detail available actions under the stricter procurement readiness rules
- Ran:
  - `go test ./...`

## 9. Ready for Frontend

- `POST /v1/tasks/{id}/procurement/advance`
- Updated contracts for:
  - `GET /v1/tasks`
  - `GET /v1/tasks/{id}`
  - `GET /v1/tasks/{id}/detail`
  - `PATCH /v1/tasks/{id}/procurement`

## 10. Risks / Known Gaps

- Projected workflow filters are still derived SQL expressions and are not index-optimized like raw persisted columns.
- Procurement still does not model supplier settlement, inbound receipt linkage, or ERP procurement integration.
- Existing clients must stop assuming procurement statuses are only `preparing|ready|completed`.

## 11. Suggested Next Step

- Decide whether the next round should add task-board presets / aggregate counters around projected workflow filters, or deepen procurement into inbound/receipt-side checkpoints without crossing into real ERP integration.
