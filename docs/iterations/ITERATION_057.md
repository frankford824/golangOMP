# ITERATION_057

## Phase
- PHASE_AUTO_057 / Step C ERP filing boundary and existing-product selection hardening

## Input Context
- Current CURRENT_STATE before execution: Step 56 complete
- Current OpenAPI version before execution: `0.56.0`
- Read latest iteration: `docs/iterations/ITERATION_056.md`
- Current execution phase file: `docs/phases/PHASE_AUTO_057.md`

## Goals
- Complete the next narrow Step C increment.
- Keep ERP Bridge query routes stable while hardening ERP-backed existing-product selection.
- Add Bridge product upsert only at the business-info filing boundary.
- Improve error handling and internal traceability without expanding into a broader integration platform.

## Files Changed
- `cmd/server/main.go`
- `domain/erp_bridge.go`
- `domain/integration_center.go`
- `service/erp_bridge_client.go`
- `service/erp_bridge_client_test.go`
- `service/erp_bridge_service.go`
- `service/integration_center_service.go`
- `service/task_erp_bridge_test.go`
- `service/task_service.go`
- `transport/handler/erp_bridge_test.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/phases/PHASE_AUTO_057.md`
- `docs/iterations/ITERATION_057.md`

## DB / Migration Changes
- No new migration in this phase.

## API Changes
- OpenAPI version advanced from `0.56.0` to `0.57.0`.
- Main project now consumes Bridge write API:
  - `POST /erp/products/upsert`
- Internal connector catalog now includes:
  - `erp_bridge_product_upsert`
- `PATCH /v1/tasks/{id}/business-info` semantics are tighter:
  - `filed_at` is now the only ERP Bridge filing boundary
  - existing-product filing requires ERP-backed `product_selection.erp_product`
  - bridge failures surface structured error details plus additive `integration_call_log_id` when available

## Design Decisions
- Chose `PATCH /v1/tasks/{id}/business-info` as the only Bridge write boundary because it already owned `filed_at` and filing semantics.
- Kept Bridge identities additive by continuing to resolve selections into local `products.id` before task persistence.
- Hardened `product_selection` correctness by rejecting mismatched `selected_product_id` plus ERP snapshot input instead of trusting caller state.
- Reused the existing integration call-log model for filing traceability instead of introducing a new sync/outbox table.
- Deferred retries/outbox/callback reliability explicitly rather than implying a broader integration platform exists.

## Verification
- Added/updated focused tests for:
  - Bridge upsert request/response normalization
  - empty-body successful Bridge upsert fallback handling
  - task business-info filing calling Bridge upsert at `filed_at`
  - internal filing call-log trace creation
  - rejection of existing-product filing without ERP-backed selection
  - rejection of mismatched `selected_product_id` plus ERP snapshot
- `go test ./service/... ./transport/...`
- `go test ./...`

## Risks / Known Gaps
- The Bridge upsert payload/result handling is intentionally tolerant because live upstream contract discovery was not reliable from this environment.
- Filing still lacks outbox/retry/callback guarantees; the phase adds one synchronous boundary, not a resilient integration platform.
- Legacy local-only existing-product tasks must be reselected through the ERP-backed picker before filing.
- Broader ERP mutation coverage, sync-log drill-through, procurement/WMS/finance docking, and Step D entry redesign remain deferred.

## Suggested Next Step
- If Step C needs another increment, keep it narrow:
  - verify live Bridge upsert behavior against the deployed Bridge service
  - add only minimal robustness/observability around this filing boundary
- Otherwise move to Step D task creation / SKU generation mainline work.
