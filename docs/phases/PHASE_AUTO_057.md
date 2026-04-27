# PHASE_AUTO_057

## Why This Phase Now
- Step 53 and Step 54 already made ERP Bridge query consumption usable for original-product selection.
- The next bounded Step C gain was not broader ERP docking; it was to make the original-product filing boundary explicit and safe.
- `PATCH /v1/tasks/{id}/business-info` already owned `filed_at`, so it was the narrowest correct place to add Bridge upsert rather than spreading writes across task create or generic product edits.

## Current Context
- Current CURRENT_STATE before this phase: Step 56 complete
- Current OpenAPI version before this phase: `0.56.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_056.md`
- Mainline focus: ERP-backed existing-product selection correctness plus one filing boundary

## Goals
- Keep `/v1/erp/*` query-first and unchanged for frontend original-product picking.
- Harden ERP-backed `product_selection` binding so Bridge snapshots and local product ids cannot silently drift apart.
- Add one narrow Bridge `POST /erp/products/upsert` call at the business-info filing boundary only.
- Reuse internal integration call logs for filing traceability instead of building a second tracking system.
- Keep broad ERP/WMS/procurement/finance docking explicitly deferred.

## Allowed Scope
- Main-project Bridge client/service extensions for product upsert
- Task business-info filing-boundary integration
- ERP-backed `product_selection` validation hardening
- Additive internal traceability via existing integration call logs
- Focused tests and doc synchronization

## Forbidden Scope
- Broad ERP mutation coverage beyond the filing boundary
- Procurement/WMS/finance integration
- Replacing local task/product authority with Bridge identities
- Task-entry redesign or Step D work
- Retry workers, outbox infrastructure, or callback platforms

## Expected File Changes
- `domain/erp_bridge.go`
- `domain/integration_center.go`
- `service/erp_bridge_client.go`
- `service/erp_bridge_service.go`
- `service/integration_center_service.go`
- `service/task_service.go`
- `service/erp_bridge_client_test.go`
- `service/task_erp_bridge_test.go`
- `transport/handler/erp_bridge_test.go`
- `cmd/server/main.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/phases/PHASE_AUTO_057.md`
- `docs/iterations/ITERATION_057.md`

## Required API / DB Changes
- API:
  - keep `/v1/erp/products`, `/v1/erp/products/{id}`, and `/v1/erp/categories` stable
  - add main-project consumption of Bridge `POST /erp/products/upsert` at the filing boundary only
  - clarify internal integration trace usage through connector `erp_bridge_product_upsert`
- DB:
  - no new migration in this phase

## Success Criteria
- Existing-product original-product picking remains query-first and Bridge-backed.
- ERP-backed `product_selection` is stricter and safer when local ids and Bridge snapshots are both supplied.
- `PATCH /v1/tasks/{id}/business-info` now acts as the only ERP filing/upsert boundary for existing-product tasks.
- Bridge failures are returned clearly and traced internally without pretending a broader integration platform exists.
- Docs and tests describe the new boundary honestly, including what remains deferred.
