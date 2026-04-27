# ITERATION_008 - PRD Task-Type Realignment / Warehouse Handoff Readiness

**Date**: 2026-03-09  
**Scope**: STEP_08

## 1. Goals

- Replace legacy V7 task types with PRD V2.0 task categories
- Add front-loaded business-info / cost-maintenance fields and API
- Add explicit warehouse handoff preparation so `purchase_task` can bypass design/audit
- Expose machine-readable workflow projection and close-blocking reasons in task list/detail
- Sync code, OpenAPI, current state, and handover docs to the new contract

## 2. Scope Boundary

- Implemented in this iteration:
  - PRD-aligned task-type validation on create
  - task business-info / cost-maintenance migration and update API
  - `POST /v1/tasks/{id}/warehouse/prepare`
  - purchase-task flow restrictions and direct warehouse path
  - workflow projection:
    - `main_status`
    - `sub_status`
    - `warehouse_blocking_reasons`
    - `cannot_close_reasons`
  - warehouse-complete close-readiness guard
  - OpenAPI upgrade to `v0.9.0`
- Explicitly not implemented in this iteration:
  - persisted PRD main/sub status columns
  - standalone close endpoint
  - dedicated procurement record table
  - real auth/RBAC enforcement
  - real ERP/NAS integration

## 3. Changed Files

### New files

| File | Purpose |
|---|---|
| `db/migrations/006_v7_task_business_info.sql` | Adds business-info / cost-maintenance fields to `task_details` |
| `service/task_workflow.go` | PRD-oriented workflow projection and blocking-reason evaluator |
| `service/task_prd_service_test.go` | Service tests for task-type validation and purchase-task warehouse handoff |
| `docs/phases/PHASE_AUTO_008.md` | Auto-generated phase contract |
| `docs/iterations/ITERATION_008.md` | This iteration record |

### Modified files

| File | Change |
|---|---|
| `domain/enums_v7.go` | Replaced public task-type contract with PRD V2.0 values |
| `domain/task.go` | Added business-info / cost-maintenance fields to `TaskDetail` |
| `domain/query_views.go` | Added workflow projection structures and `prepare_warehouse` action |
| `domain/task_detail_aggregate.go` | Added workflow projection to aggregate detail |
| `domain/audit.go` | Added task events for business-info update and warehouse prepare |
| `repo/interfaces.go` | Added task-detail business-info update repo method |
| `repo/mysql/db.go` | Added nullable float helpers |
| `repo/mysql/task.go` | Stores/selects business-info fields and list-time workflow inputs |
| `service/task_service.go` | Added task-type validation, business-info update, warehouse prepare, and list workflow enrichment |
| `service/task_assignment_service.go` | Blocks designer assignment for `purchase_task` |
| `service/task_asset_service.go` | Blocks design upload / submit for `purchase_task` |
| `service/task_detail_service.go` | Adds workflow projection and PRD-aware available actions |
| `service/warehouse_service.go` | Validates close-readiness before warehouse complete |
| `service/task_detail_service_test.go` | Updated available-action expectations, including purchase task |
| `service/task_step04_service_test.go` | Updated fake repo for new interface |
| `transport/handler/task.go` | Added business-info update and warehouse prepare handlers |
| `transport/http.go` | Registered Step 08 routes |
| `cmd/server/main.go` | Wired new service dependencies |
| `docs/api/openapi.yaml` | Upgraded to `v0.9.0` and synced Step 08 contract |
| `CURRENT_STATE.md` | Synced repo state after Step 08 completion |
| `MODEL_HANDOVER.md` | Updated architecture direction / non-negotiables |
| `docs/V7_API_READY.md` | Added Step 08 frontend-ready routes and task-flow notes |
| `docs/V7_FRONTEND_INTEGRATION_ORDER.md` | Reordered frontend integration around business-info + warehouse prepare |
| `docs/V7_MODEL_HANDOVER_APPENDIX.md` | Updated handover appendix for Step 08 |

## 4. Database Changes

### Additive migration

| Migration | Change |
|---|---|
| `006_v7_task_business_info.sql` | Adds `category`, `spec_text`, `material`, `size_text`, `craft_text`, `procurement_price`, `cost_price`, `filed_at` to `task_details` |

### No new tables

- This iteration extends `task_details`; no new table is introduced.

## 5. API Changes

| Method | Path | Notes |
|---|---|---|
| PATCH | `/v1/tasks/{id}/business-info` | Maintains PRD front-loaded business-info / cost fields |
| POST | `/v1/tasks/{id}/warehouse/prepare` | Evaluates PRD warehouse handoff readiness and moves task to `PendingWarehouseReceive` |

### Contract changes

- Task create/list/detail `task_type` now uses:
  - `original_product_development`
  - `new_product_development`
  - `purchase_task`
- Task list/detail now expose:
  - `workflow.main_status`
  - `workflow.sub_status`
  - `workflow.can_prepare_warehouse`
  - `workflow.warehouse_blocking_reasons`
  - `workflow.can_close`
  - `workflow.cannot_close_reasons`
- `purchase_task` cannot call:
  - `POST /v1/tasks/{id}/assign`
  - `POST /v1/tasks/{id}/submit-design`

## 6. Implementation Rules

- `original_product_development` must use `source_mode=existing_product`.
- `new_product_development` must use `source_mode=new_product`.
- `purchase_task` can use either SKU source mode, but must maintain procurement/cost info before warehouse handoff.
- Warehouse handoff is explicit; existing receive/reject/complete APIs do not replace readiness evaluation.
- Warehouse complete now checks PRD-aligned close-blocking reasons before moving task to `Completed`.
- PRD main/sub status is exposed as a derived projection for now; stored `task_status` is not yet fully normalized.

## 7. Verification

- Added service tests covering:
  - invalid task type + source-mode combination
  - purchase-task direct warehouse handoff
  - design-task warehouse handoff rejection without final asset / audit pass
- Ran:
  - `go test ./...`

## 8. Correction Notes

- `CURRENT_STATE.md` and OpenAPI previously still advertised legacy `regular/custom/outsource_preflight` task types; this iteration corrects the public contract to PRD V2.0 values.
- Warehouse completion previously depended on a single state gate plus SKU presence; this iteration adds close-blocking reasons so completion no longer ignores missing business-info / design prerequisites.
- Frontend docs previously implied all tasks go through designer assignment / submit-design; this iteration corrects that for `purchase_task`.

## 9. Remaining Gaps

- `task_status` persistence still mixes old Step-04/05 workflow names with new PRD projection
- No dedicated procurement record table or purchase-specific list panel yet
- No standalone close endpoint; closure remains warehouse-driven
- Cost rules, export center, KPI, finance, and open API center are still structural placeholders rather than runnable modules
- No real auth/RBAC enforcement, ERP integration, or NAS upload integration

## 10. Next Iteration Suggestion

- Make PRD main/sub status more explicit in persistence and decide whether to split close into its own endpoint
- Add dedicated procurement/cost-rule structures so purchase tasks stop overloading `task_details`
- Tighten frontend detail/list contracts around warehouse and procurement panels after integration feedback
