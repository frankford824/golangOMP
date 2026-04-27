# ITERATION_049

## Phase
- PHASE_AUTO_049 / auth / org / visibility policy scaffolding

## Input Context
- Current CURRENT_STATE before execution: Step 48 complete
- Current OpenAPI version before execution: `0.44.0`
- Read latest iteration: `docs/iterations/ITERATION_048.md`
- Current execution phase file: `docs/phases/PHASE_AUTO_049.md`

## Goals
- Move from route-local placeholder enforcement to one shared cross-center policy scaffolding language.
- Keep existing placeholder auth contract stable:
  - `X-Debug-Actor-Id`
  - `X-Debug-Actor-Roles`
  - `withAccessMeta(...)`
- Add additive read-model policy summaries for task/export/integration/cost/upload without introducing real identity/org/SSO/RBAC/ABAC systems.

## Files Changed
- `docs/phases/PHASE_AUTO_049.md`
- `domain/access_policy_scaffolding.go`
- `domain/query_views.go`
- `domain/task_detail_aggregate.go`
- `domain/task_board.go`
- `domain/procurement.go`
- `domain/cost_override_boundary.go`
- `domain/asset_storage.go`
- `domain/export_center.go`
- `domain/integration_center.go`
- `service/task_query.go`
- `service/task_service.go`
- `service/task_detail_service.go`
- `service/task_board_service.go`
- `service/procurement_summary.go`
- `service/cost_governance_read_model.go`
- `service/export_center_service_test.go`
- `service/integration_center_service_test.go`
- `service/asset_upload_service_test.go`
- `service/task_board_service_test.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/iterations/ITERATION_049.md`

## DB / Migration Changes
- None
- Step 49 is schema-preserving and read-model additive only.

## API Changes
- OpenAPI version advanced from `0.44.0` to `0.45.0`.
- No new endpoints.
- Added reusable policy scaffolding schemas:
  - `PolicyMode`
  - `PolicyAPISurface`
  - `ActionPolicySummary`
  - `ResourceAccessPolicy`
  - `PolicyScopeSummary`
- Added additive policy fields to cross-center read models:
  - `policy_mode`
  - `visible_to_roles`
  - `action_roles`
  - `policy_scope_summary`
- Coverage includes:
  - task list/read/detail/board
  - procurement summary
  - export job
  - integration call log/execution
  - cost override governance boundary
  - upload request boundary

## Design Decisions
- Policy scaffolding is implemented as one shared domain language with center-specific defaults:
  - task center
  - export center
  - integration center
  - cost governance boundary
  - upload/storage boundary
- Service hydration path was updated additively so existing business/lifecycle logic is unchanged.
- Action-level summaries now carry API surface intent:
  - `frontend_ready`
  - `internal`
  - `admin`
  - `mock_placeholder`
- Policy scaffolding is explicitly descriptive and not runtime row-level evaluation.

## Correction Notes
- Reconciled repository state references to Step 49 and OpenAPI `v0.45.0` in handover/state docs.
- Added missing `docs/iterations/ITERATION_049.md` so `CURRENT_STATE.md` latest-iteration pointer is no longer dangling.

## Risks / Known Gaps
- Current policy role defaults are broad scaffolding snapshots and may need tightening when real org/data-scope policy arrives.
- Runtime enforcement is still route-level placeholder role checks; policy summary fields are not final RBAC/ABAC decisions.
- This phase intentionally does not include:
  - real login/session/SSO
  - real org hierarchy sync
  - final fine-grained RBAC/ABAC
  - full approval permission redesign

## Suggested Next Step
- Stop automatic continuation in this round.
- If continuing later, prioritize one bounded phase such as KPI/finance/report entry boundary or policy-runtime integration planning without introducing real infra/auth rollouts in one jump.
