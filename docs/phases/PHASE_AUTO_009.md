# PHASE_AUTO_009 - PRD State Machine Explicitness / Close Entry Split

## Why This Phase Now
- The latest PRD requires warehouse completion and task closure to be distinct actions.
- Step 08 exposed `workflow.main_status`, `workflow.sub_status`, and `cannot_close_reasons`, but they were still mostly derived from legacy `task_status`.
- The repository still allowed warehouse completion to close a task directly, which kept the PRD "pending close" stage implicit.

## Current Context
- `CURRENT_STATE.md` reported Step 08 complete and explicitly called out the lack of a standalone close entry as the main gap.
- `docs/api/openapi.yaml` was still on `v0.9.0` and did not expose a close endpoint or a stable `closable` field.
- `/v1/tasks/{id}` still returned the base task only, which left workflow consumers split across list and detail contracts.

## Goals
- Make the PRD close stage explicit by introducing persisted `PendingClose`.
- Split closure from warehouse completion with `POST /v1/tasks/{id}/close`.
- Stabilize workflow query contracts around:
  - `workflow.main_status`
  - `workflow.sub_status`
  - `workflow.closable`
  - `workflow.cannot_close_reasons`
- Upgrade blocking-reason payloads from free-form strings to structured `{ code, message }`.

## Allowed Scope
- `domain/`
- `service/`
- `transport/`
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
- Dedicated procurement-table redesign

## Expected File Changes
- Add explicit pending-close semantics in the V7 task workflow model.
- Add standalone close request handling and route registration.
- Update workflow query payloads to expose `closable` and structured reasons.
- Update task read model so `GET /v1/tasks/{id}` includes workflow.
- Add or update tests for pending-close and close transitions.
- Sync OpenAPI and project-state / handover docs.

## Required API / DB Changes
- API:
  - add `POST /v1/tasks/{id}/close`
  - update `POST /v1/tasks/{id}/warehouse/complete` semantics to stop auto-closing
  - update `GET /v1/tasks/{id}` response to include workflow snapshot
- DB / migration:
  - no new migration in this phase
  - reuse the existing string-backed `task_status` column with new explicit value `PendingClose`

## Success Criteria
- Warehouse completion transitions tasks to `PendingClose`, not `Completed`.
- `POST /v1/tasks/{id}/close` closes only when workflow readiness is satisfied.
- `purchase_task`, `original_product_development`, and `new_product_development` remain distinguishable by close conditions.
- Query endpoints expose stable workflow/close-readiness fields.
- `go test ./...` passes.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_009.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Risks
- `sub_status` remains a derived snapshot even after `PendingClose` becomes explicit.
- Existing clients that assumed `warehouse/complete` implied immediate closure must switch to the new close action.
- Structured reason objects are a contract change for clients that previously parsed plain strings.

## Completion Output Format
1. Phase
2. Changed Files
3. DB / Migration Changes
4. API / OpenAPI Changes
5. Auto-Correction Notes
6. Verification
7. Risks / Remaining Gaps
8. Next Step
