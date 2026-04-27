# ITERATION_070 — ERP Bridge Production Restoration

**Date**: 2026-03-17
**Version**: v0.4 (in-place update, no version bump)

## Objective

Restore the ERP Bridge service (port 8081) to production and verify the real MAIN -> Bridge -> DB writeback chain.

## Root Cause Analysis

### Problem
- Bridge process was running on port 8081, but `POST /v1/erp/products/upsert` returned HTTP 404
- MAIN's `performERPBridgeFiling` called Bridge's upsert endpoint, got 404, logged it as failed in `integration_call_logs`, then returned nil error (non-blocking fallback)
- This created the illusion of a working filing flow while no actual writeback occurred

### Root Cause
- The route `POST /v1/erp/products/upsert` was **never registered** in `transport/http.go`
- The router only had read routes: `GET /products`, `GET /products/{id}`, `GET /categories`
- The `ERPBridgeHandler` had no `UpsertProduct` method — only SearchProducts, GetProductByID, ListCategories

### Evidence
Bridge logs before fix:
```
POST /v1/erp/products/upsert status=404 (multiple timestamps across 2026-03-14 to 2026-03-16)
```

## Fix

### Files Changed

| File | Change |
|---|---|
| `transport/handler/erp_bridge.go` | Added `UpsertProduct` handler method: parses JSON body, validates product_id/sku_id, calls `svc.UpsertProduct`, responds with result |
| `transport/http.go` | Registered `erpGroup.POST("/products/upsert", ...)` route in the `/erp` route group |

### Verification

| Layer | Evidence |
|---|---|
| Bridge process | PID 3416790, port 8081, `/proc/exe` -> `/root/ecommerce_ai/releases/v0.4/erp_bridge` (not deleted) |
| Bridge health | `GET /v1/auth/me` returns 401 (correct — auth required, not 404 or connection refused) |
| Bridge upsert | `POST /v1/erp/products/upsert` returns 200 with `{"status":"accepted","message":"stored locally"}` |
| MAIN -> Bridge | Main logs: `erp_bridge_request_completed url=http://127.0.0.1:8081/v1/erp/products/upsert method=POST status_code=200 duration=0.007s` |
| Bridge logs | `POST /v1/erp/products/upsert status=200` (was 404 before fix) |
| Blackbox flow | Task 63 created with ERP product selection -> business info updated with `filed_at` -> Bridge upsert succeeded -> task detail shows `source_mode=existing_product`, `product_id=517` |

## Non-blocking Fallback Status

- `performERPBridgeFiling` still swallows Bridge errors and returns nil (non-blocking)
- This is retained as a **safety net** for genuine Bridge failures
- Under normal operation (Bridge running), the upsert succeeds and the fallback does not trigger
- The fallback position: temporary safety net, NOT the primary writeback path

## Production State After Fix

| Component | Status |
|---|---|
| MAIN | PID 3416820, port 8080, binary v0.4 |
| Bridge | PID 3416790, port 8081, binary v0.4 |
| MAIN -> Bridge | Connected via `ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081` |
| Bridge -> DB | Uses `localERPBridgeClient` (same DB as MAIN) |
| External ERP | Not yet integrated (Bridge acts as DB adapter only) |
