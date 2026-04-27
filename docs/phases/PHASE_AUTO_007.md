# PHASE_AUTO_007 - RBAC Placeholder / Documentation Consolidation

## Why This Phase Now
- Step 06 already closed the ERP sync placeholder gap, so the next smallest unresolved infrastructure/documentation item is RBAC/auth placeholder scaffolding.
- `CURRENT_STATE.md` already identifies RBAC placeholder and ready/placeholder document consolidation as the Step 07 candidate.
- This phase improves handoff quality for frontend and future model iterations without expanding into real authentication, real permission enforcement, or real ERP integration.

## Current Context
- Current `CURRENT_STATE.md`: Step 06 complete and Step 07 candidate is RBAC placeholder plus document consolidation.
- Current OpenAPI version: `0.7.0`
- Latest iterations: `ITERATION_006.md`, `ITERATION_005.md`
- Stable business mainline already present:
  - task create / assign / submit-design
  - task detail aggregate
  - audit / outsource / warehouse flows
  - ERP sync stub worker and internal placeholder APIs
- Main missing gap for this phase:
  - no request actor / middleware placeholder
  - no route-level RBAC placeholder metadata
  - ready-for-frontend vs internal placeholder vs mock placeholder markers not yet consistently consolidated

## Goals
- Add role constants and route-level RBAC placeholder metadata for current V7 handlers.
- Add request context / middleware placeholder without enabling real authentication or permission checks.
- Consolidate frontend-readiness documentation across OpenAPI, current state, and new V7 handoff docs.

## Allowed Scope
- `domain/`
- `service/`
- `transport/`
- `docs/phases/`
- `docs/iterations/`
- `docs/api/openapi.yaml`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `CURRENT_STATE.md`

## Forbidden Scope
- Real authentication provider integration
- Real RBAC enforcement or request rejection based on roles
- New business workflows or task state changes
- Real ERP HTTP / SDK integration
- Reclassifying ERP placeholder APIs as frontend-ready

## Expected File Changes
- Add RBAC placeholder domain types and request actor context helpers.
- Add transport middleware for placeholder actor injection and route metadata headers.
- Annotate V7 routes with readiness and required-role placeholder metadata.
- Add Phase 07, iteration 007, and V7 documentation consolidation files.
- Update `CURRENT_STATE.md` and `docs/api/openapi.yaml`.

## Required API / DB Changes
- No new database tables or migrations in this phase.
- No new business endpoints in this phase.
- Add placeholder request/response header conventions for V7 routes:
  - request: `X-Debug-Actor-Id`, `X-Debug-Actor-Roles`
  - response: `X-Workflow-Auth-Mode`, `X-Workflow-API-Readiness`, `X-Workflow-Required-Roles`

## Success Criteria
- `go test ./...` passes.
- V7 routes expose placeholder readiness / required-role metadata without blocking requests.
- Request actor placeholder is available in `context.Context`.
- OpenAPI, iteration memory, current state, and V7 handoff docs are synchronized.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_007.md`
- `docs/api/openapi.yaml`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Risks
- Placeholder headers may be mistaken for real auth unless the docs remain explicit.
- Route-level required roles are advisory only in this phase and must not be treated as enforced policy.

## Completion Output Format
1. Changed files
2. DB / migration changes
3. API changes
4. Auto-correction notes
5. Risks / remaining gaps
6. Next iteration suggestion
