# ITERATION_003 - V7 Warehouse / Task Detail Aggregate / Audit Handover Query

**Date**: 2026-03-09  
**Scope**: STEP_03

## 1. Goals

- Add warehouse receipt domain: `WarehouseReceipt`
- Expose frontend-ready task aggregate detail API
- Expose audit handover list query API
- Give `TaskDetailView` and `WarehouseView` real backend APIs for direct integration
- Keep `task_assets` out of this iteration; defer to STEP_04

## 2. Changed Files

### New files

| File | Purpose |
|---|---|
| `domain/warehouse.go` | Warehouse receipt entity |
| `domain/task_detail_aggregate.go` | Task aggregate detail response model |
| `repo/mysql/warehouse.go` | Warehouse receipt repo implementation |
| `service/warehouse_service.go` | Warehouse receive / reject / complete logic |
| `service/task_detail_service.go` | Task aggregate detail read service |
| `transport/handler/warehouse.go` | Warehouse query and action handlers |
| `transport/handler/task_detail.go` | Task aggregate detail handler |
| `db/migrations/003_v7_warehouse_detail.sql` | Step 03 additive migration |
| `docs/iterations/ITERATION_003.md` | Step 03 iteration record |

### Modified files

| File | Change |
|---|---|
| `domain/enums_v7.go` | Added `WarehouseReceiptStatus` |
| `domain/audit.go` | Added warehouse task event type constants |
| `repo/interfaces.go` | Added task detail read, handover list, and warehouse repo interfaces |
| `repo/mysql/task.go` | Added `GetDetailByTaskID` |
| `repo/mysql/audit_v7.go` | Added handover list by task query |
| `service/audit_service.go` | Added handover list service contract |
| `service/audit_v7_service.go` | Added handover list implementation |
| `transport/handler/audit_v7.go` | Added `/audit/handovers` query handler |
| `transport/http.go` | Registered Step 03 routes |
| `cmd/server/main.go` | Wired Step 03 repos, services, and handlers |
| `docs/api/openapi.yaml` | Upgraded to `v0.4.0` and documented Step 03 APIs |
| `CURRENT_STATE.md` | Synced repo state after Step 03 completion |

## 3. New / Modified Tables

### New tables

| Table | Notes |
|---|---|
| `warehouse_receipts` | One warehouse receipt record per task; unique by `task_id` |

### Existing tables used by aggregate detail

- `tasks`
- `task_details`
- `audit_records`
- `audit_handovers`
- `outsource_orders`
- `task_event_logs`
- `products`

## 4. New / Modified APIs

| Method | Path | Notes |
|---|---|---|
| GET | `/v1/tasks/{id}/detail` | New aggregate detail API for frontend detail page |
| GET | `/v1/tasks/{id}/audit/handovers` | New audit handover query API |
| GET | `/v1/warehouse/receipts` | New warehouse receipt list API |
| POST | `/v1/tasks/{id}/warehouse/receive` | New warehouse receive action |
| POST | `/v1/tasks/{id}/warehouse/reject` | New warehouse reject action |
| POST | `/v1/tasks/{id}/warehouse/complete` | New warehouse complete action |
| GET | `/v1/tasks/{id}` | Preserved as base task entity only |

## 5. Aggregate Detail Structure

`GET /v1/tasks/{id}/detail` returns:

```json
{
  "data": {
    "task": {},
    "task_detail": {},
    "product": {},
    "audit_records": [],
    "audit_handovers": [],
    "outsource_orders": [],
    "warehouse_receipt": null,
    "event_logs": []
  }
}
```

### Rules

- `GET /v1/tasks/{id}` is not changed and still returns only the root `task`
- `GET /v1/tasks/{id}/detail` is the frontend-oriented aggregate endpoint
- `event_logs` are ordered by `sequence ASC`
- Empty children remain in the payload:
  - `audit_records`: `[]`
  - `audit_handovers`: `[]`
  - `outsource_orders`: `[]`
  - `event_logs`: `[]`
- `warehouse_receipt` returns `null` if it does not exist
- `product` may be `null` for `new_product` tasks or missing source data

## 6. Warehouse Status Flow

### Warehouse receipt status

- `received`
- `rejected`
- `completed`

### Task status rules

- Only tasks in `PendingWarehouseReceive` may call `receive`, `reject`, or `complete`
- `receive`:
  - creates `warehouse_receipts`
  - keeps task status at `PendingWarehouseReceive`
- `reject`:
  - requires `reject_reason` or `remark`
  - sets warehouse receipt status to `rejected`
  - moves task status to `Blocked`
- `complete`:
  - requires an existing `received` warehouse receipt
  - requires non-empty `task.sku_code`
  - sets warehouse receipt status to `completed`
  - moves task status to `Completed`
- All warehouse actions write `task_event_logs`

## 7. Handover List API

### Endpoint

- `GET /v1/tasks/{id}/audit/handovers`

### Behavior

- Returns `404` when task does not exist
- Returns handovers ordered by `created_at DESC`
- This ordering is documented in OpenAPI and kept consistent in repo implementation

## 8. Business Rules Implemented

| Rule | Implementation |
|---|---|
| Warehouse actions only allowed in `PendingWarehouseReceive` | Service-level guard before action |
| Warehouse reject must explain why | `reject_reason` or `remark` required |
| Warehouse complete needs SKU binding | `task.sku_code` must be non-empty |
| Receive and complete are separate | `receive` does not move task to `Completed` |
| Reject routes task back for manual handling | `reject -> Blocked` |
| All warehouse actions must emit task events | `task_event_logs` append inside same transaction |
| Aggregate detail should minimize frontend round-trips | One query service composes task/detail/product/audit/outsource/warehouse/events |
| Handover list must be task-scoped | Service checks task existence before querying |

## 9. Unfinished Items

- `task_assets` not implemented in this iteration
- `assign` not implemented
- `submit-design` not implemented
- Real file upload not implemented
- RBAC/auth middleware not implemented
- Outsource lifecycle transitions beyond create/list are still missing

## 10. Next Iteration Suggestion

- Add `task_assets` domain, table, and upload metadata API
- Add assign APIs and status transitions around `PendingAssign` / `Assigned`
- Add submit-design workflow entry and related event model
