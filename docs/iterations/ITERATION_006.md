# ITERATION_006 - V7 ERP Sync Placeholder / Contract

**Date**: 2026-03-09  
**Scope**: STEP_06

## 1. Goals

- Add a runnable ERP sync placeholder based on local stub data
- Persist ERP sync run history for visibility and troubleshooting
- Expose internal sync status and manual trigger APIs
- Keep all new ERP sync APIs clearly marked as internal placeholder and not ready for frontend
- Sync Step 06 code, OpenAPI, and project state documentation

## 2. Scope Boundary

- Implemented in this iteration:
  - ERP sync config block
  - stub ERP product provider
  - batch upsert into `products`
  - `erp_sync_runs` persistence
  - scheduled ERP sync worker
  - internal ERP sync status and manual trigger APIs
  - OpenAPI upgrade to `v0.7.0`
- Explicitly not implemented in this iteration:
  - real ERP HTTP / SDK integration
  - RBAC / auth enforcement
  - NAS / upload / asset storage work
  - Task / Audit / Warehouse flow changes

## 3. Changed Files

### New files

| File | Purpose |
|---|---|
| `domain/erp_sync.go` | ERP sync DTOs, run records, and status models |
| `repo/mysql/erp_sync_run.go` | MySQL repo for `erp_sync_runs` |
| `service/erp_sync_service.go` | ERP sync provider, service, and stub file reader |
| `service/erp_sync_service_test.go` | Service tests for success, update, noop, and failed cases |
| `transport/handler/erp_sync.go` | Internal ERP sync status and manual trigger handlers |
| `transport/handler/erp_sync_test.go` | Handler tests for status and trigger routes |
| `workers/erp_sync_worker.go` | Scheduled ERP sync placeholder worker |
| `db/migrations/005_v7_erp_sync_runs.sql` | Step 06 additive migration |
| `config/erp_products_stub.json` | Local ERP stub source file |
| `docs/phases/PHASE_AUTO_006.md` | Auto-generated phase contract |
| `docs/iterations/ITERATION_006.md` | This iteration record |

### Modified files

| File | Change |
|---|---|
| `config/config.go` | Added ERP sync config block and env parsing |
| `repo/interfaces.go` | Added `ProductRepo.UpsertBatch` and `ERPSyncRunRepo` |
| `repo/mysql/product.go` | Added batch upsert by `erp_product_id` |
| `transport/http.go` | Registered ERP sync routes under `/v1/products/sync/*` |
| `cmd/server/main.go` | Wired ERP sync repo, provider, service, handler, and worker |
| `workers/group.go` | Registered optional ERP sync worker |
| `docs/api/openapi.yaml` | Upgraded to `v0.7.0` and documented ERP placeholder APIs |
| `CURRENT_STATE.md` | Synced repo state after Step 06 completion |

## 4. Database Changes

### New tables

| Table | Notes |
|---|---|
| `erp_sync_runs` | Stores ERP sync placeholder execution history including trigger mode, counts, and status |

### Reused tables

| Table | Change |
|---|---|
| `products` | Reused as ERP master-data target; no schema change, now supports Step 06 batch upsert logic |

## 5. API Changes

| Method | Path | Notes |
|---|---|---|
| GET | `/v1/products/sync/status` | Internal placeholder status endpoint with latest sync run summary |
| POST | `/v1/products/sync/run` | Internal placeholder manual sync trigger |

### API contract rules

- Both Step 06 ERP sync APIs are internal placeholder APIs.
- They are documented in OpenAPI but are **not** ready for frontend.
- They currently have no auth / RBAC gate; RBAC remains a later step.
- Normal product read APIs remain unchanged:
  - `GET /v1/products/search`
  - `GET /v1/products/{id}`

## 6. Implementation Rules

- ERP sync source mode is currently fixed to `stub`.
- The stub provider reads `ERP_SYNC_STUB_FILE` as a JSON array.
- If the stub file is missing:
  - sync result status is `noop`
  - the run is still persisted into `erp_sync_runs`
- If the stub file is invalid JSON:
  - sync result status is `failed`
  - the error summary is persisted into `erp_sync_runs`
- Product upsert rules:
  - unique key source is `erp_product_id`
  - overwrite `sku_code`, `product_name`, `category`, `spec_json`, `status`, `source_updated_at`
  - update `sync_time` on every successful upsert run
  - do not delete historical products absent from the current source file

## 7. Verification

- Added service tests covering:
  - new ERP product insert from stub file
  - existing ERP product update by `erp_product_id`
  - missing stub file -> `noop`
  - invalid stub file -> `failed`
- Added handler tests covering:
  - initial status response with `latest_run = null`
  - manual trigger response payload
- Ran `go test ./...`

## 8. Correction Notes

- `CURRENT_STATE.md` previously still listed ERP sync worker as not implemented; this iteration resolves that gap and updates the state document accordingly.
- OpenAPI was previously at `v0.6.0` and had no Step 06 ERP placeholder APIs; it is now synchronized to the implementation at `v0.7.0`.

## 9. Remaining Gaps

- No real ERP system integration yet; only local stub-file placeholder
- No RBAC / auth enforcement yet
- ERP sync APIs are internal-only and must not be treated as ready for frontend
- No sync metrics, retry policy, or dead-letter handling beyond run history
- No delete/deactivate reconciliation policy for products missing from source

## 10. Next Iteration Suggestion

- Add RBAC placeholder and role matrix documentation
- Decide which internal APIs should gain auth middleware first
- Consolidate placeholder / ready-for-frontend markers across OpenAPI and state docs
