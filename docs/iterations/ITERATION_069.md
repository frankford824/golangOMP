# Iteration 069 — v0.4 Closure: ERP Filing Non-Blocking Fix + 3-Round Blackbox Validation

**Date**: 2026-03-16

## Objective
Complete v0.4 closure: fix blocking bugs, run comprehensive 3-round blackbox end-to-end tests covering all three task types, and solidify documentation for model handover.

## Changes

### 1. ERP Bridge Filing Made Non-Blocking (`service/task_service.go`)
- **Problem**: `PATCH /v1/tasks/:id/business-info` with `filed_at` for `existing_product` source mode triggered `performERPBridgeFiling`, which called the ERP Bridge at port 8081. When Bridge was unavailable, the entire business-info update failed, preventing `filed_at` from being set and blocking task close.
- **Fix**: In `performERPBridgeFiling`, the `UpsertProduct` call error is now non-blocking. The call log records the failure (status=failed), but the function returns `nil` error so the `filed_at` and other business info fields are persisted. Validation errors (missing product_selection, missing product_id) still block correctly.
- **Impact**: Tasks can now complete the full lifecycle even when ERP Bridge is offline. Filing can be retried later via integration call logs.

### 2. TaskListItem Field Completeness (from prior session, verified)
- Added `owner_team`, `priority`, `created_at`, `is_outsource` to `TaskListItem` struct and SQL queries
- Extended keyword search to include `owner_team LIKE` and `CAST(t.id AS CHAR) = ?`
- Verified with unit tests (54-column scan match)

### 3. Timestamp Consistency Analysis
- **Finding**: Backend timestamps are consistent within `Asia/Shanghai` timezone. MySQL `CURRENT_TIMESTAMP` and Go driver `loc=Asia/Shanghai` (via `TZ` env) round-trip correctly. No backend fix needed; any display issues are frontend-layer.

### 4. Task Creation Rules PRD Alignment
- All three task types' validation rules verified aligned with `docs/TASK_CREATE_RULES.md`. No code changes needed.

## Blackbox Test Results

All three rounds executed on live deployment (PID 3253212, `223.4.249.11:8080`):

### Round 1: Original Product Development
- Create task (ERP product selection) -> Assign designer -> Submit design (delivery) -> Audit claim (A) -> Audit approve (A -> PendingWarehouseReceive) -> Business info (category, spec, cost, filed_at) -> Warehouse receive -> Warehouse complete -> Close
- **Result: PASSED** (task_status = Completed)

### Round 2: New Product Development
- Create task (new_product, preset material) -> Assign designer -> Submit design -> Audit claim -> Audit approve -> Business info -> Warehouse receive -> Warehouse complete -> Close
- **Result: PASSED** (task_status = Completed)

### Round 3: Purchase Task
- Create task (purchase_task, manual cost) -> Business info -> Procurement (draft -> prepare -> start -> complete) -> Prepare warehouse -> Warehouse receive -> Warehouse complete -> Close
- **Result: PASSED** (task_status = Completed)

## Files Modified
- `service/task_service.go` — ERP filing non-blocking fix in `performERPBridgeFiling`
- `domain/query_views.go` — `TaskListItem` field additions (prior session)
- `repo/mysql/task.go` — SQL query and scan updates (prior session)
- `repo/mysql/task_test.go` — Test column count update (prior session)

## Deployment
- Binary cross-compiled `GOOS=linux GOARCH=amd64`
- Deployed to `223.4.249.11` via SCP + `deploy-restart.sh`
- PID: 3253212, `/proc/PID/exe` → live binary, `/health` → `{"status":"ok"}`
