# ITERATION_072 — MAIN ERPSyncWorker Verification, Recovery, and Boundary Clarification

**Date**: 2026-03-17  
**Version**: v0.4 (in-place update, no version bump)

## Objective

Verify the real live state of MAIN(8080) `ERPSyncWorker`, recover it only if broken, and clarify the responsibility boundary between MAIN sync ownership, Bridge(8081), and future external ERP integration.

## Confirmed Baseline Before Change

- Historical `cmd/api/main.go` cron-based `*/10 * * * *` plus `syncSvc.IncrementalSync(10)` is legacy only and is not the live runtime owner.
- Live sync owner remained MAIN(8080) `ERPSyncWorker` plus `/v1/products/sync/*`.
- Live `GET /v1/products/sync/status` returned:
  - `scheduler_enabled=true`
  - `interval_seconds=300`
  - `source_mode=stub`
  - latest scheduled run `status=noop`
- Live authenticated `POST /v1/products/sync/run` also returned `status=noop`.
- Live MAIN process:
  - PID `3416820`
  - `/proc/3416820/exe -> /root/ecommerce_ai/releases/v0.4/ecommerce-api`
  - `/proc/3416820/cwd -> /root`
- Packaged stub file existed at `/root/ecommerce_ai/releases/v0.4/config/erp_products_stub.json`, but `/root/config/erp_products_stub.json` did not exist.

## Confirmed Root Cause

- This was not a scheduler-disabled problem and not a Bridge ownership problem.
- Root cause was deployment/runtime layer mismatch:
  - `ERP_SYNC_STUB_FILE` remained relative (`config/erp_products_stub.json`)
  - process working directory was `/root`
  - packaged config lived beside the binary under the release directory
- Result: `StubERPProductProvider` resolved the stub path under `/root/config/...`, hit `os.ErrNotExist`, and the sync service intentionally persisted `noop`.

## Code Changes

### 1) Runtime Recovery

- Updated `deploy/run-with-env.sh` to `cd` into the resolved binary directory before `exec`.
- This preserves packaged relative-path assumptions for:
  - `ERP_SYNC_STUB_FILE`
  - `AUTH_SETTINGS_FILE`
  - other release-local config assets

### 2) Observability Enhancement

- Extended `ERPSyncStatus` with:
  - `resolved_stub_file`
  - `stub_file_exists`
- `service/erp_sync_service.go` now resolves the effective stub path at runtime and reports whether that file currently exists.

### 3) Test Coverage

- Added service tests covering:
  - resolved existing stub path
  - missing stub path visibility

## Architecture Findings

### Current MAIN Responsibility

- MAIN(8080) remains the current scheduler/cache owner for:
  - `ERPSyncWorker`
  - `GET /v1/products/sync/status`
  - `POST /v1/products/sync/run`
  - local `products` cache
  - `erp_sync_runs`

### Current Bridge Responsibility

- Bridge(8081) owns:
  - ERP/JST adapter query semantics
  - mutation execution (`POST /v1/erp/products/upsert`)
- In current live `ERP_REMOTE_MODE=local`, Bridge uses `localERPBridgeClient`, so Bridge query/write behavior still reads/writes the shared local `products` table.

### Current Overlap Reality

- There are two confirmed write paths into MAIN `products`:
  - scheduled/manual sync (`ERPSyncWorker`)
  - Bridge-driven local binding/upsert (`EnsureLocalProduct`, `UpsertProduct`)
- This is a real dual-writer shape, but current live behavior before recovery did not produce sync-side writes because the worker was stuck in `noop`.
- After recovery, the sync writer remains `stub`-backed, so it is still compatibility/placeholder data, not formal external ERP truth.

## Recommended Future Alignment

- Keep MAIN as the scheduler/cache owner.
- Do not move generic sync ownership into Bridge.
- When formal external ERP query access is ready, replace `ERPSyncWorker` source from `stub` with a Bridge-owned query/export contract:
  - recommended flow: `MAIN ERPSyncWorker -> Bridge query/export client -> MAIN products upsert + erp_sync_runs`
- Rationale:
  - Bridge already owns adapter semantics and runtime mode switching (`local|remote|hybrid`)
  - MAIN should keep local cache authority and task/business ownership
  - MAIN should not independently encode external ERP adapter semantics once Bridge is the canonical adapter layer

## Verification

### Local Code

- `go test ./service` passed
- `go test ./transport/handler` passed
- `go test ./cmd/server` compile passed

### Live Runtime After Deploy

- MAIN process replaced in-place on `v0.4`
- new MAIN PID: `3450797`
- `/proc/3450797/exe -> /root/ecommerce_ai/releases/v0.4/ecommerce-api` and is not deleted
- `/proc/3450797/cwd -> /root/ecommerce_ai/releases/v0.4`
- `/health` remains `200`
- `GET /v1/products/sync/status` now reports resolved stub path metadata
- manual sync no longer returns `noop`; it reads the packaged stub source and performs local upsert (`success`, `2/2`)
- first post-deploy scheduled tick also recovered: latest run became `trigger_mode=scheduled`, `status=success`, `started_at=2026-03-17T10:58:38+08:00`
- `/v1/products/search` and `/v1/erp/products` both returned the two stub-backed rows after recovery

## Files Changed

- `domain/erp_sync.go`
- `service/erp_sync_service.go`
- `service/erp_sync_service_test.go`
- `deploy/run-with-env.sh`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_v0.4_memory.md`
- `MODEL_HANDOVER.md`
- `docs/iterations/ITERATION_072.md`
- `ITERATION_INDEX.md`
