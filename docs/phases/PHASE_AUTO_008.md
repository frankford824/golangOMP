# PHASE_AUTO_008 - PRD Task-Type Realignment / Warehouse Handoff Readiness

## Why This Phase Now
- The latest PRD V2.0 replaces the old "design -> audit -> warehouse" default path with a task-centric mainline driven by task type.
- The current repository still models task types as `regular/custom/outsource_preflight`, which blocks procurement tasks from becoming first-class workflow variants.
- Frontend and downstream handoff are currently blocked by missing PRD concepts: business-info maintenance before warehouse, procurement-task direct warehouse entry, and machine-readable close-blocking reasons.

## Current Context
- `CURRENT_STATE.md` reports Step 07 complete, but the business flow is still centered on legacy design/audit statuses.
- `docs/api/openapi.yaml` still exposes the old V7 task-type contract and does not yet reflect PRD V2.0's three task categories.
- Warehouse APIs exist, but there is no explicit "prepare/handoff to warehouse" step and purchase tasks cannot bypass designer assignment / audit.

## Goals
- Replace the public V7 task-type contract with PRD-aligned task categories:
  - `original_product_development`
  - `new_product_development`
  - `purchase_task`
- Add task business-info / cost-maintenance entry points needed before warehouse handoff.
- Add a warehouse handoff action that allows purchase tasks to enter the warehouse path without design/audit while still gating design tasks on required subflows.
- Expose PRD-oriented workflow projections:
  - main status
  - sub-statuses
  - warehouse blocking reasons
  - close blocking reasons

## Allowed Scope
- `domain/`
- `repo/`
- `service/`
- `transport/`
- `cmd/server/main.go`
- `db/migrations/`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/iterations/`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Forbidden Scope
- Real authentication or RBAC enforcement
- Real ERP integration beyond the existing placeholder
- Real upload / NAS integration
- Production subflow implementation
- KPI / finance / export center full business implementation
- Large refactors unrelated to PRD mainline correction

## Expected File Changes
- Update task enums and task/business-info models for PRD V2.0 task categories.
- Add additive migration for business-info / cost-maintenance fields.
- Add task business-info update API and warehouse handoff API.
- Update task list/detail projections with PRD-oriented main status, sub-statuses, and close-readiness reasons.
- Add service tests for purchase-task direct warehouse handoff and close/readiness evaluation.
- Sync OpenAPI and state / iteration handoff documents.

## Required API / DB Changes
- Add one additive migration for task business-info / cost-maintenance fields.
- Add `PATCH /v1/tasks/{id}/business-info`.
- Add `POST /v1/tasks/{id}/warehouse/prepare`.
- Update task create/list/detail schemas and enums to the new task types and workflow projections.

## Success Criteria
- PRD-aligned task types are enforced by the V7 task API.
- Purchase tasks can reach warehouse preparation without designer assignment or audit.
- Design task types still require design/audit prerequisites before warehouse preparation.
- Task list/detail responses expose main status, sub-statuses, and blocking reasons.
- `go test ./...` or `go build ./...` passes after document sync.

## Required Document Updates
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/iterations/ITERATION_008.md`
- `docs/api/openapi.yaml`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Risks
- Existing local data using legacy task-type values may need manual normalization outside this phase.
- The first PRD-aligned readiness model will be correct functionally but may still use straightforward list-time derivation instead of optimized SQL projections.

## Completion Output Format
1. Phase
2. Why this phase now
3. PRD alignment changes
4. Changed Files
5. DB / Migration Changes
6. API / OpenAPI Changes
7. Auto-Correction / Verification
8. Risks / Remaining Gaps
9. Next
