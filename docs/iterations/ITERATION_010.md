# ITERATION_010 - Structured Sub-Status / Procurement Persistence Boundary

**Date**: 2026-03-09  
**Scope**: STEP_10

## 1. Goals

- Replace loose `workflow.sub_status` strings with a stable structured contract.
- Separate `purchase_task` procurement preparation from generic `task_details`.
- Keep Step 09 close semantics intact while making purchase readiness explicit.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration still marked Step 10 as the next candidate only.
- `docs/api/openapi.yaml` was at `v0.10.0`.
- Latest completed iteration before this round: `docs/iterations/ITERATION_009.md`.
- The main gaps were:
  - `sub_status` lacked an explicit stable machine contract
  - procurement preparation still reused `task_details`

## 3. Files Changed

### Code

- `cmd/server/main.go`
- `domain/audit.go`
- `domain/enums_v7.go`
- `domain/procurement.go`
- `domain/query_views.go`
- `domain/task.go`
- `domain/task_detail_aggregate.go`
- `repo/procurement_interface.go`
- `repo/mysql/procurement.go`
- `repo/mysql/task.go`
- `service/task_detail_service.go`
- `service/task_detail_service_test.go`
- `service/task_prd_service_test.go`
- `service/task_service.go`
- `service/task_workflow.go`
- `transport/handler/task.go`
- `transport/http.go`

### Documents

- `db/migrations/007_v7_procurement_records.sql`
- `docs/phases/PHASE_AUTO_010.md`
- `docs/iterations/ITERATION_010.md`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## 4. DB / Migration Changes

- Added `db/migrations/007_v7_procurement_records.sql`.
- New table:
  - `procurement_records`
- Migration backfills existing purchase-task `task_details.procurement_price` into `procurement_records` as `ready`.

## 5. API Changes

### New endpoint

- `PATCH /v1/tasks/{id}/procurement`

### Query contract changes

- `workflow.sub_status.*` now returns:
  - `code`
  - `label`
  - `source`
- `GET /v1/tasks/{id}` now includes nullable `procurement`
- `GET /v1/tasks/{id}/detail` now includes nullable `procurement`

### Boundary changes

- `PATCH /v1/tasks/{id}/business-info` no longer owns procurement preparation fields.
- Purchase-task readiness now depends on:
  - generic business info in `task_details`
  - dedicated procurement record
  - procurement price
  - procurement status `ready|completed`

## 6. Design Decisions

- Kept `task_status` as the persisted operational state.
  - This round stabilizes the read contract instead of redesigning the entire persisted state model.
- Chose structured `sub_status` objects over adding a new persisted sub-status table.
  - This gives frontend/API stability now with much smaller schema impact.
- Added a dedicated procurement table and endpoint instead of continuing to overload `task_details`.
  - This establishes a clear ownership boundary without attempting a full procurement module in one round.
- Kept close / closable / cannot-close semantics intact.
  - The new procurement checks strengthen purchase-task readiness without regressing the Step 09 split.

## 7. Correction Notes

- Corrected repository drift where docs still described procurement preparation as living in `task_details` only.
- Corrected the public task-detail contract by removing procurement preparation from the public `business-info` API boundary.
- Corrected `workflow.sub_status` drift by making the OpenAPI and implementation agree on structured objects instead of loose strings.

## 8. Verification

- Added/updated tests for:
  - purchase-task warehouse prepare with dedicated procurement readiness
  - close failure when procurement record is missing
  - procurement update persistence
  - available actions under the new procurement boundary
- Ran:
  - `go test ./...`

## 9. Ready for Frontend

- `PATCH /v1/tasks/{id}/procurement`
- Updated contracts for:
  - `GET /v1/tasks`
  - `GET /v1/tasks/{id}`
  - `GET /v1/tasks/{id}/detail`
  - `PATCH /v1/tasks/{id}/business-info`

## 10. Risks / Known Gaps

- `sub_status` is now stable at the API layer, but still computed rather than persisted.
- Procurement persistence is still only a skeleton and does not yet model a full supplier / inbound lifecycle.
- Existing clients must stop assuming `workflow.sub_status` values are plain strings.

## 11. Suggested Next Step

- Extend list/query/filter capabilities around structured `sub_status` once frontend integration settles.
- Evolve `procurement_records` beyond readiness maintenance only after the current boundary is proven stable.
