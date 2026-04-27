# ITERATION_053

## Phase
- PHASE_AUTO_053 / ERP Bridge query integration / original-product picker mainline

## Input Context
- Current CURRENT_STATE before execution: Step 52 complete
- Current OpenAPI version before execution: `0.48.0`
- Read latest iteration: `docs/iterations/ITERATION_052.md`
- Current execution phase file: `docs/phases/PHASE_AUTO_053.md`

## Goals
- Connect the main project to ERP Bridge query APIs without modifying the ERP Bridge service.
- Make keyword ERP product search the primary original-product picker entry.
- Let selected ERP Bridge products enter task create and business-info mainline via additive `product_selection`.
- Keep bridge categories auxiliary and non-blocking because current category coverage is incomplete.

## Files Changed
- `config/config.go`
- `db/migrations/026_v7_erp_bridge_selection_snapshot.sql`
- `domain/erp_bridge.go`
- `domain/query_views.go`
- `domain/task.go`
- `domain/task_product_selection.go`
- `repo/interfaces.go`
- `repo/mysql/product.go`
- `repo/mysql/task.go`
- `service/erp_bridge_client.go`
- `service/erp_bridge_service.go`
- `service/erp_bridge_client_test.go`
- `service/erp_sync_service_test.go`
- `service/product_service_test.go`
- `service/task_erp_bridge_test.go`
- `service/task_product_selection.go`
- `service/task_service.go`
- `transport/handler/erp_bridge.go`
- `transport/handler/task.go`
- `transport/http.go`
- `cmd/server/main.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/phases/PHASE_AUTO_053.md`
- `docs/iterations/ITERATION_053.md`

## DB / Migration Changes
- Added DB migration `026_v7_erp_bridge_selection_snapshot.sql`.
- Added `task_details.product_selection_snapshot_json` so task-side `product_selection` can persist ERP Bridge external snapshot fields additively.

## API Changes
- OpenAPI version advanced from `0.48.0` to `0.53.0`.
- Added frontend-ready ERP Bridge query APIs:
  - `GET /v1/erp/products`
  - `GET /v1/erp/products/{id}`
  - `GET /v1/erp/categories`
- Clarified that `GET /v1/products/search` is local cached search, not real ERP Bridge lookup.
- `product_selection` now supports additive `erp_product` snapshot fields for task create/business-info flows.

## Design Decisions
- Chose a dedicated `/v1/erp/*` query surface instead of overloading the existing local `/v1/products/*` routes, so bridge search and local cache search stay explicit.
- Preserved current task mainline by auto-caching/binding selected ERP Bridge products into local `products` before task persistence rather than redesigning task identity around external-only ids.
- Stored ERP Bridge selection extras in an additive task-side snapshot JSON column instead of scattering more single-purpose columns across the task schema.
- Kept bridge category integration auxiliary only because current upstream category coverage is known incomplete.

## Verification
- Added focused tests for:
  - ERP Bridge client response normalization
  - ERP Bridge local binding/caching
  - task create/business-info ERP Bridge mainline binding
- Attempted live checks against `http://223.4.249.11:8081` for:
  - `/health`
  - `/erp/products?q=定制车缝&page=1&page_size=2`
  - `/erp/categories`
- Current live result from this environment: upstream accepted the TCP connection but returned `Empty reply from server` / connection closed unexpectedly, so live query verification did not complete.

## Risks / Known Gaps
- Live ERP Bridge connectivity from the current environment still needs follow-up because the configured host returned an empty reply rather than JSON.
- ERP Bridge response normalization is intentionally tolerant because the exact upstream JSON contract was not fully discoverable from live probes in this run.
- This phase does not add ERP writeback, SKU writeback, WMS/procurement integration docking, upload/storage docking, or finance/report work.
- Bridge categories remain auxiliary only; frontend must not make them a required picker prerequisite.

## Suggested Next Step
- Use the current Step 53 surface to complete frontend联调 against `/v1/erp/*`.
- If live bridge access still fails, verify upstream protocol/port/ingress on the deployed ERP Bridge before changing main-project mapping logic.
