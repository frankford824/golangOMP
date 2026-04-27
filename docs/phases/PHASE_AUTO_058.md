# PHASE_AUTO_058

## Why This Phase Now
- Step C already made ERP-backed original-product selection and the filing boundary minimally usable.
- The next mainline gain had to be Step D task entry rather than more platform scaffolding.
- The narrowest correct Step D increment was to harden `/v1/tasks` creation so all three task types enter with explicit SKU behavior and truthful initial read state.

## Current Context
- Current CURRENT_STATE before this phase: Step 57 complete
- Current OpenAPI version before this phase: `0.57.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_057.md`
- Mainline focus: task creation, SKU generation/binding, and three task-type entry closure

## Goals
- Keep task creation explicit, stable, and safe by task type.
- Preserve Step C boundaries for ERP-backed original-product selection.
- Make SKU behavior narrow and understandable:
  - existing product binds existing SKU
  - new product generates SKU at create when omitted
- Ensure `purchase_task` enters without design/audit assumptions and exposes an immediate procurement entry state.
- Keep docs/tests aligned with the new Step D entry behavior.

## Allowed Scope
- Task create handler/service/domain/repo read-model alignment
- SKU generation/binding behavior inside task create only
- Task-type-specific validation improvements
- Narrow procurement/read-model initialization required for purchase-task entry
- Focused tests and documentation sync

## Forbidden Scope
- Procurement flow redesign beyond draft entry initialization
- Warehouse redesign
- Audit redesign
- Broad ERP mutation expansion
- Generic workflow-engine or platform-center rewrites

## Expected File Changes
- `service/task_service.go`
- `service/task_prd_service_test.go`
- `service/task_detail_service_test.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/phases/PHASE_AUTO_058.md`
- `docs/iterations/ITERATION_058.md`

## Required API / DB Changes
- API:
  - keep `POST /v1/tasks` as the single entry route
  - clarify per-task-type create semantics and source-mode-driven SKU behavior
  - document machine-readable create validation details
- DB:
  - no new migration in this phase
  - `purchase_task` create should reuse existing `procurement_records` persistence by initializing a draft row

## Success Criteria
- `original_product_development` create remains ERP-backed/query-first for existing-product selection.
- `new_product_development` create works cleanly with explicit new-product entry and SKU generation.
- `purchase_task` create works without design/audit assumptions and exposes draft procurement state immediately.
- Read/list/detail models show truthful initial workflow and procurement shape after entry.
- Docs and tests describe the new Step D behavior honestly without implying broader downstream completion.
