# ITERATION_005 - V7 Query Enhancement / Detail Aggregate Upgrade / Unified Pagination

**Date**: 2026-03-09  
**Scope**: STEP_05

## 1. Goals

- Enhance `GET /v1/tasks` for real frontend list replacement
- Upgrade `GET /v1/tasks/{id}/detail` into a more frontend-ready aggregate
- Enhance `GET /v1/products/search`, `GET /v1/outsource-orders`, and `GET /v1/warehouse/receipts`
- Standardize the main V7 list query response shape to `{ data, pagination }`
- Keep V6 compatibility and preserve the existing Step 01-04 business mainline

## 2. Scope Boundary

- Implemented in this iteration:
  - task list filter expansion and list-item projection enhancement
  - detail aggregate `assets` embedding
  - detail aggregate `available_actions` generation
  - product / outsource / warehouse list pagination and filter enhancement
  - OpenAPI upgrade to `v0.6.0`
- Explicitly not implemented in this iteration:
  - real file upload
  - NAS integration
  - RBAC / auth
  - ERP worker
  - audit dashboard statistics
  - WebSocket

## 3. Changed Files

### New files

| File | Purpose |
|---|---|
| `domain/query_views.go` | V7 pagination meta, task list item projection, available action enum |
| `service/pagination.go` | Shared pagination normalization helper |
| `service/task_detail_service_test.go` | Unit tests for `available_actions` generation |
| `docs/iterations/ITERATION_005.md` | Step 05 iteration record |

### Modified files

| File | Change |
|---|---|
| `domain/task_detail_aggregate.go` | Added `assets` and `available_actions` to aggregate response |
| `repo/interfaces.go` | Expanded V7 query filters and list repo contracts with `total` |
| `repo/mysql/db.go` | Added shared page/page_size normalization |
| `repo/mysql/task.go` | Added Step 05 task list filters, count query, warehouse join, latest asset projection |
| `repo/mysql/product.go` | Added product total count and category fuzzy match |
| `repo/mysql/outsource.go` | Added vendor filter and total count |
| `repo/mysql/warehouse.go` | Added receiver filter and total count |
| `service/task_service.go` | Added task filter expansion and paginated task list response |
| `service/task_detail_service.go` | Aggregated `task_assets` and generated `available_actions` |
| `service/product_service.go` | Added paginated product search response |
| `service/outsource_service.go` | Added vendor filter and paginated outsource list response |
| `service/warehouse_service.go` | Added receiver filter and paginated warehouse list response |
| `transport/handler/response.go` | Added V7 pagination response helper |
| `transport/handler/sku.go` | Added shared bool query parser helper |
| `transport/handler/task.go` | Bound Step 05 task list query parameters and paginated response |
| `transport/handler/product.go` | Returned paginated product search response |
| `transport/handler/outsource.go` | Bound vendor filter and paginated response |
| `transport/handler/warehouse.go` | Bound receiver filter and paginated response |
| `service/task_step04_service_test.go` | Updated repo mock signature for paginated task list contract |
| `cmd/server/main.go` | Wired task detail aggregate service with `taskAssetRepo` |
| `docs/api/openapi.yaml` | Upgraded to `v0.6.0` and documented Step 05 query/detail changes |
| `CURRENT_STATE.md` | Synced repo state after Step 05 completion |

## 4. New / Modified APIs

| Method | Path | Change |
|---|---|---|
| GET | `/v1/tasks` | Added Step 05 filters, pagination, and frontend-oriented list item projection |
| GET | `/v1/tasks/{id}/detail` | Added `assets` and `available_actions` |
| GET | `/v1/products/search` | Added pagination standardization |
| GET | `/v1/outsource-orders` | Added `vendor` filter and pagination |
| GET | `/v1/warehouse/receipts` | Added `receiver_id` filter and pagination |

## 5. Unified Response Rules

### Standardized in Step 05

- List queries now return:

```json
{
  "data": [],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 0
  }
}
```

- Applied to:
  - `GET /v1/tasks`
  - `GET /v1/products/search`
  - `GET /v1/outsource-orders`
  - `GET /v1/warehouse/receipts`

### Still intentionally unchanged in Step 05

- Single-object responses remain `{ "data": {...} }`
- The following V7 query endpoints keep their pre-Step-05 `data`-only list structure:
  - `GET /v1/tasks/{id}/assets`
  - `GET /v1/tasks/{id}/audit/handovers`
  - `GET /v1/tasks/{id}/events`
  - `GET /v1/code-rules`

Reason:
- they were already usable by the frontend
- they are not part of the Step 05 query replacement target
- changing them here would expand scope beyond the requested iteration boundary

## 6. Task List Enhancement

### New supported query params

- `status`
- `task_type`
- `source_mode`
- `creator_id`
- `designer_id`
- `need_outsource`
- `overdue`
- `keyword`
- `page`
- `page_size`

### `overdue` rule

- `true` means `deadline_at < now` and task status is not `Completed`, `Archived`, or `Cancelled`
- `false` means the complement set

### `keyword` match scope

- `task_no`
- `sku_code`
- `product_name_snapshot`

### Added task list fields

- `warehouse_status`
- `latest_asset_type`

### `latest_asset_type` source rule

- Derived from the task's latest `task_assets` record by highest `version_no`
- Tie-breaker is later `created_at`
- If the task has no assets, the field is `null`

## 7. Detail Aggregate Enhancement

`GET /v1/tasks/{id}/detail` now returns:

```json
{
  "data": {
    "task": {},
    "task_detail": {},
    "product": {},
    "assets": [],
    "audit_records": [],
    "audit_handovers": [],
    "outsource_orders": [],
    "warehouse_receipt": null,
    "event_logs": [],
    "available_actions": []
  }
}
```

### Rules

- `/v1/tasks/{id}` is still the base entity endpoint only
- `/v1/tasks/{id}/detail` is the frontend aggregate detail endpoint
- `/v1/tasks/{id}/assets` is still retained for standalone timeline refresh
- `assets` are ordered by `version_no ASC`
- `warehouse_receipt` returns `null` when absent
- `audit_records`, `audit_handovers`, `outsource_orders`, `event_logs`, and `assets` remain empty arrays when no rows exist

## 8. `available_actions` Implementation

### Status-based generation rules

- `PendingAssign` -> `assign`
- `InProgress` / `RejectedByAuditA` -> `submit_design`
- `PendingAuditA` / `PendingAuditB` -> `claim_audit`, `approve_audit`, `reject_audit`, `handover`
- `PendingOutsource` -> `create_outsource`
- `PendingOutsourceReview` -> `claim_audit`, `approve_audit`, `handover`
- `PendingWarehouseReceive` with no receipt -> `warehouse_receive`, `warehouse_reject`
- `PendingWarehouseReceive` with `warehouse_receipt.status=received` -> `warehouse_reject`, `warehouse_complete`

### Important constraint

- `available_actions` is frontend guidance only
- all real transition legality is still enforced by service-layer guards

## 9. Query Enhancement Notes

### Product search

- `category` is matched against the existing `products.category` column
- current implementation uses SQL `LIKE`

### Outsource list

- `vendor` is matched with SQL `LIKE` on `vendor_name`

### Warehouse list

- `receiver_id` filters the current receipt owner

### Known implementation limit

- current filters are implemented with straightforward SQL predicates and `LIKE`
- no dedicated Step 05 index tuning was added in this iteration

## 10. Verification

- Added unit tests for `available_actions`
- Updated Step 04 service test doubles to satisfy new repo contracts
- Ran `go test ./...`

## 11. Frontend Replacement Readiness

The following endpoints are now suitable for direct frontend mock replacement in normal page flows:

- `GET /v1/tasks`
- `GET /v1/tasks/{id}/detail`
- `GET /v1/products/search`
- `GET /v1/outsource-orders`
- `GET /v1/warehouse/receipts`

Already available from earlier steps and still usable:

- `POST /v1/tasks/{id}/assign`
- `POST /v1/tasks/{id}/submit-design`
- `GET /v1/tasks/{id}/assets`
- `POST /v1/tasks/{id}/assets/mock-upload`

## 12. Remaining Gaps

- No real file upload
- No NAS integration
- No RBAC/auth
- No ERP worker
- Asset list independent filtering is still basic
- Some Step 05 filters are functional but not index-optimized yet

## 13. Next Iteration Suggestion

- Add ERP worker placeholder and sync contract documentation
- Add RBAC/auth placeholder and role matrix documentation
- Consolidate V7 query response standards across the remaining `data`-only list endpoints
