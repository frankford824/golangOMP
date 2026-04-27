# ITERATION_071 — Bridge External ERP Remote-Ready Upgrade

**Date**: 2026-03-17  
**Version**: v0.4 (in-place update, no version bump)

## Objective

Only advance Bridge(8081) from local-write-only toward external-ERP-connectable state, while preserving current v0.4 MAIN flow safety.

## Confirmed Baseline Before Change

- Live Bridge process existed on `8081` and MAIN->Bridge was online.
- `POST /v1/erp/products/upsert` returned `200`, response `message=stored locally`.
- Bridge env used `ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081`, so runtime selected local write path.
- `/proc/<bridge_pid>/exe` pointed to `/root/ecommerce_ai/releases/v0.4/erp_bridge` (not deleted).

## What Was Implemented

### 1) Config and Mode Switch

- Added `ERPRemote` config block in `config/config.go`.
- Added env-driven mode:
  - `ERP_REMOTE_MODE=local|remote|hybrid`
- Added remote upsert connection/auth/retry config keys:
  - `ERP_REMOTE_BASE_URL`
  - `ERP_REMOTE_UPSERT_PATH`
  - `ERP_REMOTE_TIMEOUT`
  - `ERP_REMOTE_RETRY_MAX`
  - `ERP_REMOTE_RETRY_BACKOFF`
  - `ERP_REMOTE_AUTH_MODE` (`none|bearer|app`)
  - `ERP_REMOTE_*` auth/sign header/token fields
  - `ERP_REMOTE_FALLBACK_LOCAL_ON_ERROR`

### 2) Remote ERP Client

- Added `service/erp_bridge_remote_client.go`:
  - remote upsert request封装
  - timeout + retry(backoff)
  - auth modes: none/bearer/app-sign
  - structured logging for request/status/retry
  - response decode reuse for `ERPProductUpsertResult`

### 3) Runtime Wiring

- Updated `cmd/server/main.go`:
  - MAIN (`8080`) keeps existing HTTP Bridge client behavior.
  - Bridge (`8081`) now switches by `ERP_REMOTE_MODE`:
    - local -> local client
    - remote -> remote client
    - hybrid -> remote with optional local fallback

### 4) Deployment/Runtime Env

- Updated `deploy/bridge.env.example` with remote-mode keys.
- Rebuilt and replaced cloud Bridge binary only (`/root/ecommerce_ai/releases/v0.4/erp_bridge`), then restarted Bridge.
- Updated live `/root/ecommerce_ai/shared/bridge.env` to include remote-mode keys, with live mode pinned to `local`.

## Verification

### Code-Level

- `go test ./config ./service` passed.
- `go test ./cmd/server` compile passed.

### Process-Level (Cloud)

- Bridge PID after deploy: `3423714`.
- `/proc/3423714/exe -> /root/ecommerce_ai/releases/v0.4/erp_bridge` (not deleted).
- `GET http://127.0.0.1:8081/health` => `200`.

### MAIN -> Bridge

- Authenticated `MAIN -> Bridge` upsert path still returns `200`.
- Bridge upsert response remains `stored locally` under live `ERP_REMOTE_MODE=local`.

### Bridge -> External ERP

- Not verified end-to-end with real external formal ERP response in this iteration.
- Blocking dependencies remain: external base URL/path final contract, auth/sign details, credentials, whitelist/network readiness.

## Risk and Fallback Position

- Local mode retained as production safety valve.
- Hybrid mode provides controlled degradation path when remote upsert fails.
- Non-blocking behavior at MAIN filing boundary remains unchanged.

## Files Changed

- `config/config.go`
- `cmd/server/main.go`
- `service/erp_bridge_remote_client.go` (new)
- `deploy/bridge.env.example`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_v0.4_memory.md`
- `MODEL_HANDOVER.md`
