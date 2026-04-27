# ITERATION_009 - PRD Pending-Close Explicitness / Standalone Close

**Date**: 2026-03-09  
**Scope**: STEP_09

## 1. Goals

- Make the PRD "pending close" stage explicit in the persisted task flow.
- Split close from warehouse completion with a standalone endpoint.
- Stabilize workflow query payloads around `workflow`, `closable`, and structured close-blocking reasons.
- Correct state/OpenAPI/docs drift caused by Step 08 still describing warehouse completion as final closure.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration still listed Step 09 as a candidate only.
- `docs/api/openapi.yaml` was at `v0.9.0`.
- Latest completed iteration before this round: `docs/iterations/ITERATION_008.md`.
- The latest PRD gap was:
  - PRD state machine not explicit enough
  - no standalone close endpoint
  - close still depended on warehouse completion

## 3. Files Changed

### Code

- `domain/enums_v7.go`
- `domain/query_views.go`
- `domain/audit.go`
- `service/task_workflow.go`
- `service/task_service.go`
- `service/task_detail_service.go`
- `service/warehouse_service.go`
- `service/task_prd_service_test.go`
- `service/task_detail_service_test.go`
- `transport/handler/task.go`
- `transport/http.go`

### Documents

- `docs/phases/PHASE_AUTO_009.md`
- `docs/iterations/ITERATION_009.md`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## 4. DB / Migration Changes

- No new migration.
- Reused the existing `tasks.task_status` string contract and introduced explicit `PendingClose` semantics in code and API docs.

## 5. API Changes

### New endpoint

- `POST /v1/tasks/{id}/close`

### Changed semantics

- `POST /v1/tasks/{id}/warehouse/complete`
  - before: could directly move task to `Completed`
  - now: moves task to `PendingClose`

### Query contract updates

- `GET /v1/tasks/{id}` now returns the lightweight read model with `workflow`
- workflow payload now exposes:
  - `workflow.closable`
  - `workflow.can_close` (compatibility alias)
  - `workflow.warehouse_blocking_reasons[] = { code, message }`
  - `workflow.cannot_close_reasons[] = { code, message }`
- detail `available_actions` now includes `close` when the task is closable in `PendingClose`

## 6. Design Decisions

- Chose explicit `PendingClose` over adding new workflow tables or columns in this round.
  - This keeps the phase focused while still making the PRD close stage persisted and observable.
- Kept `sub_status` derived for now.
  - The mainline gap was warehouse-complete-vs-close ambiguity; solving that first gives the biggest PRD gain with the smallest schema blast radius.
- Added structured reason objects instead of plain strings.
  - This turns "cannot close" into a machine-readable contract without requiring frontend string parsing.
- Upgraded `GET /v1/tasks/{id}` to include workflow.
  - This removes the inconsistency where list/detail exposed workflow but the single-task read path did not.

## 7. Correction Notes

- Corrected `CURRENT_STATE.md` and OpenAPI drift that still implied warehouse completion directly closed tasks.
- Corrected `/v1/tasks/{id}` contract drift: the code now returns workflow and the docs state that explicitly.
- Corrected close-readiness contract drift by replacing plain string reason arrays with structured reason objects.

## 8. Verification

- Added/updated tests for:
  - warehouse prepare blocking on missing design readiness
  - close readiness failure outside `PendingClose`
  - close success from `PendingClose`
  - warehouse complete moving task to `PendingClose`
  - `available_actions` exposing `close`
- Ran:
  - `go test ./...`

## 9. Ready for Frontend

- `POST /v1/tasks/{id}/close`
- Updated semantics for:
  - `GET /v1/tasks/{id}`
  - `POST /v1/tasks/{id}/warehouse/complete`
  - workflow reason arrays and `closable`

## 10. Risks / Known Gaps

- `sub_status` is still a derived snapshot, not a persisted state machine.
- Existing frontend consumers must stop assuming warehouse completion equals closure.
- No dedicated procurement persistence yet; purchase-task readiness still uses `task_details`.
- No real auth/RBAC, ERP, NAS, or real upload integration yet.

## 11. Suggested Next Step

- Tighten `sub_status` into a more explicit contract or typed enum set.
- Introduce procurement-specific persistence so purchase-task readiness stops overloading generic business-info fields.
