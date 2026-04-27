# PHASE_AUTO_054

## Why This Phase Now
- Step 53 established the real ERP Bridge query boundary, but live upstream behavior remained uneven and partially undiscoverable from this environment.
- The next bounded mainline gain was therefore to harden the main-project integration layer itself rather than expanding scope into ERP writeback or broader downstream integrations.
- Frontend integration also needed a more explicit normalized query contract for `/v1/erp/products` so keyword-first picking stays stable while bridge category coverage is still incomplete.

## Current Context
- Current CURRENT_STATE before this phase: Step 53 complete
- Current OpenAPI version before this phase: `0.53.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_053.md`
- Mainline focus: ERP Bridge query reliability / task-side original-product binding hardening

## Goals
- Stabilize the ERP Bridge integration layer without changing the upstream ERP Bridge service.
- Improve bridge product search reliability through broader response normalization and stricter handler/service query normalization.
- Add additive pagination/filter normalization for `/v1/erp/products`.
- Harden task-side ERP Bridge product binding and snapshot persistence while keeping compatibility with older partial snapshots.
- Add lightweight internal observability for ERP Bridge calls: timeout/error classification, retry hints, and request logging.

## Allowed Scope
- Main-project ERP Bridge client/service/handler/repo-binding hardening
- Additive task-side snapshot normalization/backfill logic
- Focused tests for bridge query normalization, duplicate-row handling, and snapshot merge/backfill behavior
- OpenAPI/state/handover/iteration synchronization

## Forbidden Scope
- ERP Bridge upstream service changes
- ERP writeback
- WMS / procurement / finance docking
- Broader integration-center redesign
- Upload / NAS / object-storage work
- Frontend redesign beyond contract clarification

## Expected File Changes
- `service/erp_bridge_client.go`
- `service/erp_bridge_service.go`
- `service/task_product_selection.go`
- `transport/handler/erp_bridge.go`
- `cmd/server/main.go`
- `domain/erp_bridge.go`
- `service/erp_bridge_client_test.go`
- `service/task_erp_bridge_test.go`
- `transport/handler/erp_bridge_test.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/phases/PHASE_AUTO_054.md`
- `docs/iterations/ITERATION_054.md`

## Required API / DB Changes
- API:
  - keep `/v1/erp/products`, `/v1/erp/products/{id}`, `/v1/erp/categories` stable
  - extend `/v1/erp/products` with additive query normalization and response `normalized_filters`
  - clarify timeout/retry-hint diagnostics on ERP Bridge failures
- DB:
  - no new migration in this phase

## Success Criteria
- `/v1/erp/products` stays backward-compatible while exposing a clearer normalized query contract.
- ERP Bridge product parsing tolerates broader upstream envelope/detail variants and duplicate rows more safely.
- Task-side `product_selection.erp_product` persistence remains additive and backward-compatible for older partial snapshots.
- ERP Bridge failures are easier to diagnose through internal timeout/retry-hint details and request logging.
- OpenAPI and handover/state docs clearly describe the stabilized Step 54 behavior and remaining deferred scope.
