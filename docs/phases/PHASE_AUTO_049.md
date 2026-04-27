# PHASE_AUTO_049

## Why This Phase Now
- Step 48 completed export admission hardening and left auth/org/visibility as the highest-impact cross-center PRD gap.
- Route-level placeholder auth enforcement already exists (`withAccessMeta(...)` + `X-Debug-Actor-*`), but policy semantics are still scattered and mostly route-local.
- The smallest safe next move is to add unified policy scaffolding language across task/export/integration/cost/upload read models without entering real identity, org sync, SSO, or deep RBAC/ABAC implementation.

## Current Context
- Current CURRENT_STATE before this phase: Step 48 complete
- Current OpenAPI version before this phase: `0.44.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_048.md`
- Current main remaining safe gap: auth / org / visibility policy scaffolding

## Goals
- Introduce one reusable policy scaffolding model:
  - `policy_scope_summary`
  - `resource_access_policy`
  - `action_policy_summary`
- Expose additive `policy_mode` / `visible_to_roles` / `action_roles` on key cross-center read models.
- Keep existing route-level placeholder auth contract stable:
  - request headers: `X-Debug-Actor-Id`, `X-Debug-Actor-Roles`
  - route middleware: `withAccessMeta(...)`
- Align OpenAPI + CURRENT_STATE + handover docs on access semantics and placeholder boundaries.

## Allowed Scope
- Additive policy scaffolding domain models and hydrators
- Additive read-model fields for task/detail/board, export, integration, cost-governance boundary, and upload-request boundary
- Focused service-layer hydration wiring and tests
- OpenAPI / state / iteration / handover synchronization

## Forbidden Scope
- Real identity provider integration
- Real login/session/SSO implementation
- Real org hierarchy synchronization
- Final fine-grained RBAC/ABAC engine
- Full approval-permission system redesign
- Real ERP / NAS / object storage / runner platform integration

## Expected File Changes
- Add policy scaffolding domain model helpers
- Update task/export/integration/cost/upload read-model structs and hydration paths
- Update focused service tests
- Add `docs/phases/PHASE_AUTO_049.md`
- Add `docs/iterations/ITERATION_049.md`
- Update `docs/api/openapi.yaml`
- Update `CURRENT_STATE.md`
- Update `MODEL_HANDOVER.md`
- Update `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Required API / DB Changes
- DB:
  - no new tables or migrations
- API:
  - no new endpoints
  - additive policy scaffolding fields on existing read-model schemas
  - OpenAPI clarifies this is policy scaffolding, not a real auth/org system

## Success Criteria
- Task/export/integration/cost/upload read models can express default visibility/action roles with one shared policy language.
- Frontend/internal/admin exposure intent is machine-readable in action-level policy summaries.
- Existing route-level placeholder auth enforcement remains unchanged and non-regressed.
- OpenAPI and handover docs clearly state this phase is scaffolding-only and not final auth/org/RBAC/ABAC.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_049.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Risks
- Current policy role defaults are still broad scaffolding snapshots and may need tightening when real org/data-scope policies land.
- Action policy surfaces (`frontend_ready/internal/admin/mock_placeholder`) are descriptive summaries, not runtime policy evaluation.

## Completion Output Format
1. Changed files
2. DB / migration changes
3. API changes
4. Correction notes
5. Risks / known gaps
6. Suggested next step
