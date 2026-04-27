# PHASE_AUTO_010 - Structured Sub-Status Contract / Procurement Persistence Split

## Why This Phase Now
- Step 09 made close and pending-close explicit, but `workflow.sub_status` still exposed loose derived strings.
- The latest PRD requires `main_status`, `sub_status`, and completion readiness to be machine-readable and stable across list/read/detail queries.
- `purchase_task` still overloaded generic `task_details` with procurement preparation data, which kept the purchase mainline boundary unclear.

## Current Context
- `CURRENT_STATE.md` before this round reported Step 09 complete and named structured sub-status plus procurement persistence as the top gap.
- `docs/api/openapi.yaml` was at `v0.10.0`.
- Latest completed iteration before this round: `docs/iterations/ITERATION_009.md`.
- Current mainline blockers were:
  - `sub_status` was still a loose snapshot
  - procurement preparation was not persisted as its own boundary

## Goals
- Turn `workflow.sub_status` into a stable structured `{ code, label, source }` contract.
- Clarify the relationship between persisted `task_status`, projected `main_status`, and structured `sub_status`.
- Add a dedicated procurement persistence skeleton and expose it through task queries / update API.

## Allowed Scope
- `domain/`
- `repo/`
- `service/`
- `transport/`
- `db/migrations/`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/iterations/`
- `docs/phases/`

## Forbidden Scope
- Real authentication or RBAC enforcement
- Real ERP integration
- NAS / real upload integration
- Whole-hash strict verification
- KPI / finance / export center module delivery
- Full procurement lifecycle or supplier/ERP receipt integration

## Expected File Changes
- Introduce structured sub-status types in the workflow read model.
- Add procurement entity/repo/migration and wire it into task read/detail/workflow logic.
- Add `PATCH /v1/tasks/{id}/procurement`.
- Remove procurement preparation from the public `business-info` contract.
- Add/update tests for structured sub-status and purchase-task readiness.

## Required API / DB Changes
- API:
  - add `PATCH /v1/tasks/{id}/procurement`
  - update `workflow.sub_status` from strings to structured objects
  - expose nullable `procurement` on `GET /v1/tasks/{id}` and `GET /v1/tasks/{id}/detail`
  - keep `close` / `closable` / `cannot_close_reasons` stable
- DB / migration:
  - add `procurement_records` via migration 007
  - keep legacy `task_details.procurement_price` column intact, but stop using it as the new public source of truth

## Success Criteria
- `workflow.sub_status` is stable and structured across `/v1/tasks`, `/v1/tasks/{id}`, and `/v1/tasks/{id}/detail`.
- `purchase_task` readiness uses dedicated procurement persistence.
- `PATCH /v1/tasks/{id}/procurement` works and is documented.
- `go test ./...` passes.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_010.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Risks
- `sub_status` is now a stronger API contract; clients must stop treating it as arbitrary display text.
- Procurement persistence is still only a readiness skeleton and not yet a full operational module.
- Legacy rows that depended on `task_details.procurement_price` rely on migration/backfill to align with the new boundary.

## Completion Output Format
1. Changed Files
2. DB / Migration Changes
3. API / OpenAPI Changes
4. Auto-Correction Notes
5. Verification
6. Risks / Remaining Gaps
7. Next Step
