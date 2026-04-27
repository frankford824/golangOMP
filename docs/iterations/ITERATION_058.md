# ITERATION_058

## Phase
- PHASE_AUTO_058 / Step D task entry and SKU mainline hardening

## Input Context
- Current CURRENT_STATE before execution: Step 57 complete
- Current OpenAPI version before execution: `0.57.0`
- Read latest iteration: `docs/iterations/ITERATION_057.md`
- Current execution phase file: `docs/phases/PHASE_AUTO_058.md`

## Goals
- Implement the next minimal usable Step D increment around task creation.
- Make SKU generation/binding consistent with task entry semantics.
- Close the three task-type entry path without redesigning later procurement/audit/warehouse flows.

## Files Changed
- `service/task_detail_service_test.go`
- `service/task_prd_service_test.go`
- `service/task_service.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/phases/PHASE_AUTO_058.md`
- `docs/iterations/ITERATION_058.md`

## DB / Migration Changes
- No new migration in this phase.

## API Changes
- OpenAPI version advanced from `0.57.0` to `0.58.0`.
- `POST /v1/tasks` create semantics are now explicit:
  - `original_product_development` must use `source_mode=existing_product`
  - `new_product_development` must use `source_mode=new_product`
  - `purchase_task` may use either source mode
- SKU behavior is now source-mode-driven during task create:
  - existing-product entry must bind SKU from the selected product
  - new-product entry auto-generates SKU from the enabled `new_sku` rule when omitted
- create validation now documents additive machine-readable `error.details.violations`

## Design Decisions
- Extended the existing create-task flow instead of adding new task-type-specific endpoints.
- Kept Step C unchanged by leaving ERP-backed original-product binding query-first and additive.
- Initialized draft procurement only for `purchase_task` so entry read models are immediately useful without pretending procurement is already prepared.
- Scoped SKU generation to `source_mode=new_product` to keep the rule narrow and understandable.
- Deferred downstream procurement, audit, and warehouse redesign rather than mixing them into entry hardening.

## Verification
- Added/updated focused tests for:
  - new-product task create auto-generating SKU
  - purchase-task create initializing draft procurement read state
  - machine-readable validation errors for invalid new-product entry
  - purchase-task entry actions remaining free of design actions at draft entry
- `go test ./service/...`
- `go test ./...`

## Risks / Known Gaps
- Existing-product legacy create calls that omit bound SKU now fail fast instead of silently creating an unusable entry.
- New-product purchase tasks now enter with a generated SKU, but downstream procurement/warehouse detail completion is still handled by existing endpoints.
- Purchase-task create initializes only draft procurement state; it does not expand procurement lifecycle automation.
- Task create still returns the root `Task` entity; richer entry workflow shape is consumed through existing read/detail endpoints.

## Suggested Next Step
- Continue Step D on the narrow business path:
  - refine frontend consumption around entry validation/read models if needed
  - add only the next required task-entry or SKU-binding seam
- Do not jump into Step E closure work unless a concrete Step D blocker requires it.
