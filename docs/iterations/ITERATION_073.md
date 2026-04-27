# ITERATION_073 — Bridge 8081 ERP Adapter Surface Completion (v0.4)

**Date**: 2026-03-17  
**Version**: v0.4 (in-place update, no version bump)

## Objective

Close the confirmed live 8081 Bridge capability gap so future MAIN(8080) can rely on Bridge as the unified ERP/JST business adapter entry, while explicitly preserving 8082 as the resident JST sync service.

## Confirmed Input Baseline

- Current production entry remains `cmd/server/main.go` with role split by `SERVER_PORT`.
- Live role split:
  - `8080 = MAIN`
  - `8081 = Bridge`
- Confirmed existing Bridge endpoints before this iteration:
  - `GET /health`
  - `GET /v1/erp/products`
  - `GET /v1/erp/products/{id}`
  - `GET /v1/erp/categories`
  - `POST /v1/erp/products/upsert`
- Confirmed missing (live 404 before this iteration):
  - `GET /v1/erp/sync-logs`
  - `POST /v1/erp/products/shelve/batch`
  - `POST /v1/erp/products/unshelve/batch`
  - `POST /v1/erp/inventory/virtual-qty`

## Code Changes

### 1) Bridge route/handler/service/client chain completion

- Added Bridge routes in `transport/http.go`:
  - `GET /v1/erp/sync-logs`
  - `GET /v1/erp/sync-logs/{id}`
  - `POST /v1/erp/products/shelve/batch`
  - `POST /v1/erp/products/unshelve/batch`
  - `POST /v1/erp/inventory/virtual-qty`
- Added handler methods in `transport/handler/erp_bridge.go` with payload/query validation and consistent error mapping.
- Extended `service.ERPBridgeService` with:
  - sync-log list/detail methods
  - shelve/unshelve batch mutation methods
  - virtual-inventory mutation method

### 2) Adapter/client layer expansion

- Extended `service.ERPBridgeClient` with the same new capabilities.
- Extended `service/erp_bridge_client.go` HTTP facade client to call the new `/v1/erp/*` paths and normalize responses.
- Extended `service/erp_bridge_remote_client.go`:
  - added configurable remote paths for new endpoints
  - added retry/auth-aware remote calls for new mutation and sync-log operations
  - kept hybrid fallback behavior to local client
- Extended `service/erp_bridge_local_client.go`:
  - implemented local mutation contracts for shelve/unshelve/virtual-qty
  - implemented local sync-log list/detail by reading `integration_call_logs`
  - added local mutation call-log persistence so `sync_log_id` is available

### 3) Integration connector extension (observability)

- Added connector keys in `domain/integration_center.go`:
  - `erp_bridge_product_shelve_batch`
  - `erp_bridge_product_unshelve_batch`
  - `erp_bridge_inventory_virtual_qty`
- Exposed these connectors in `service/integration_center_service.go`.

### 4) Config/bootstrap updates

- Extended `config.ERPRemoteConfig` with remote path envs for shelve/unshelve/virtual-qty/sync-logs.
- Updated `cmd/server/main.go` remote client wiring to pass the new path settings.
- Updated local client construction (server/api entrypoints) to inject integration call-log repo.

### 5) Test updates

- Extended ERP bridge handler/service/client stubs for new interfaces.
- Added/updated tests in:
  - `service/erp_bridge_client_test.go`
  - `service/erp_bridge_local_client_test.go`
  - `service/task_erp_bridge_test.go`
  - `transport/handler/erp_bridge_test.go`

## OpenAPI / Contract Updates

- Updated `docs/api/openapi.yaml`:
  - Added 8081 Bridge contracts for:
    - `/v1/erp/sync-logs`
    - `/v1/erp/sync-logs/{id}`
    - `/v1/erp/products/shelve/batch`
    - `/v1/erp/products/unshelve/batch`
    - `/v1/erp/inventory/virtual-qty`
  - Added related schemas for sync-log and mutation payload/result DTOs.
  - Added explicit 8082 JST sync contract documentation:
    - `/internal/jst/ping`
    - `/jst/sync/inc`
    - tagged as `JSTSync8082` and marked as non-8080/8081 runtime ownership.

## Architecture Boundary Clarification

- 8081 Bridge remains the unified ERP/JST business adapter boundary for MAIN-facing query/write semantics.
- 8082 remains the resident JST sync executor (official JST pull/full/inc/callback/cache-refresh domain).
- No service merge was done; only minimal shared contract and adapter expansion was applied.

## Verification

### Local code verification

- `go test ./...` passed after this change set.

### Runtime verification

- Live remote host: `223.4.249.11` (localhost probing over SSH).
- 8081 Bridge verification:
  - `GET /health` returned `200`.
  - `GET /v1/erp/products`, `GET /v1/erp/products/{id}`, `GET /v1/erp/categories`, `POST /v1/erp/products/upsert` returned `401` (session-backed auth enforced), confirming route chain is mounted and no longer `404`.
  - New routes `GET /v1/erp/sync-logs`, `GET /v1/erp/sync-logs/{id}`, `POST /v1/erp/products/shelve/batch`, `POST /v1/erp/products/unshelve/batch`, `POST /v1/erp/inventory/virtual-qty` also returned `401` instead of historical `404`.
- 8082 JST sync verification:
  - Initial check showed `8082` not listening (`curl HTTP:000`), with stale `run/erp_sync.pid`.
  - Recovered by running `/root/ecommerce_ai/scripts/start-sync.sh --base-dir /root/ecommerce_ai`.
  - After recovery: `GET /health` = `200`, `GET /internal/jst/ping` = `200`, `POST /jst/sync/inc` = `200`.
- Coexistence verification:
  - `ss -ltnp` confirms concurrent listeners: `8080` (`ecommerce-api`), `8081` (`erp_bridge`), `8082` (`erp_bridge_sync`).
  - `/proc/<pid>/exe` links point to existing binaries and do not contain `(deleted)`.

## Files Changed (high-level)

- `domain/erp_bridge.go`
- `domain/integration_center.go`
- `config/config.go`
- `cmd/server/main.go`
- `cmd/api/main.go`
- `transport/http.go`
- `transport/handler/erp_bridge.go`
- `service/erp_bridge_service.go`
- `service/erp_bridge_client.go`
- `service/erp_bridge_local_client.go`
- `service/erp_bridge_remote_client.go`
- `service/integration_center_service.go`
- tests under `service/` and `transport/handler/`
- docs:
  - `docs/api/openapi.yaml`
  - `CURRENT_STATE.md`
  - `MODEL_v0.4_memory.md`
  - `MODEL_HANDOVER.md`
  - `docs/iterations/ITERATION_073.md`
