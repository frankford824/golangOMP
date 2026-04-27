# ITERATION_007 - V7 RBAC Placeholder / Documentation Consolidation

**Date**: 2026-03-09  
**Scope**: STEP_07

## 1. Goals

- Add V7 RBAC role constants and route-level placeholder metadata
- Add request actor / middleware placeholder without enabling real authentication or permission enforcement
- Consolidate ready-for-frontend vs internal placeholder vs mock placeholder documentation
- Sync Step 07 code, OpenAPI, and project state documentation

## 2. Scope Boundary

- Implemented in this iteration:
  - V7 role constant expansion
  - request actor placeholder context helpers
  - transport middleware for placeholder actor injection
  - route-level readiness and required-role placeholder metadata
  - V7 readiness / handover document consolidation
  - OpenAPI upgrade to `v0.8.0`
- Explicitly not implemented in this iteration:
  - real authentication
  - real RBAC enforcement
  - request rejection based on roles
  - real ERP integration
  - real upload / NAS integration

## 3. Changed Files

### New files

| File | Purpose |
|---|---|
| `domain/rbac_placeholder.go` | V7 readiness / auth placeholder enums, context helpers, and role normalization |
| `transport/auth_placeholder.go` | Request actor injection and route metadata middleware |
| `transport/auth_placeholder_test.go` | Middleware tests proving metadata injection without enforcement |
| `docs/phases/PHASE_AUTO_007.md` | Auto-generated phase contract |
| `docs/V7_API_READY.md` | Ready/internal/mock endpoint matrix |
| `docs/V7_FRONTEND_INTEGRATION_ORDER.md` | Recommended frontend integration sequence |
| `docs/V7_MODEL_HANDOVER_APPENDIX.md` | Compact Step 07 handover appendix |
| `docs/iterations/ITERATION_007.md` | This iteration record |

### Modified files

| File | Change |
|---|---|
| `domain/enums.go` | Added `Warehouse`, `Outsource`, and `ERP` role constants |
| `service/context.go` | Reads typed request actor placeholder from `context.Context` before legacy fallbacks |
| `transport/http.go` | Registered request actor middleware and V7 route metadata headers |
| `docs/api/openapi.yaml` | Upgraded to `v0.8.0` and normalized readiness / placeholder classification |
| `CURRENT_STATE.md` | Synced repo state after Step 07 completion |

## 4. Database Changes

- None
- No new migrations in this iteration

## 5. API Changes

### No new paths

- Step 07 adds no new business endpoints.

### Contract additions

- Placeholder request headers for V7 routes:
  - `X-Debug-Actor-Id`
  - `X-Debug-Actor-Roles`
- Placeholder response headers for V7 routes:
  - `X-Workflow-Auth-Mode`
  - `X-Workflow-API-Readiness`
  - `X-Workflow-Required-Roles`

### Readiness classification

- `ready_for_frontend`:
  - main V7 product/task/audit/outsource/warehouse/code-rule routes
- `internal_placeholder`:
  - `GET /v1/products/sync/status`
  - `POST /v1/products/sync/run`
- `mock_placeholder_only`:
  - `POST /v1/tasks/{id}/assets/mock-upload`

## 6. Implementation Rules

- Request actor context is injected from debug headers only.
- Missing headers fall back to actor ID `1` with `system_fallback` source.
- Route metadata is exposed for observability and handoff only.
- No route rejects requests based on role metadata in this iteration.
- Existing business semantics for task/audit/outsource/warehouse flows remain unchanged.

## 7. Verification

- Added transport tests covering:
  - request actor header parsing and context injection
  - route metadata headers without role enforcement
- Ran:
  - `go test ./transport/... ./service/...`

## 8. Correction Notes

- OpenAPI previously mixed readiness status in prose only; this iteration normalizes V7 routes with explicit `ReadyForFrontend`, `InternalPlaceholder`, and `MockPlaceholderOnly` markers.
- Repository docs previously stated RBAC/auth was missing but did not expose a stable placeholder contract; Step 07 now adds a documented placeholder contract without claiming real enforcement.

## 9. Remaining Gaps

- No real authentication provider or session/token handling
- No real RBAC enforcement
- ERP sync remains stub-file-driven internal placeholder only
- Mock upload route is still not a real upload contract
- No real NAS / file storage integration

## 10. Next Iteration Suggestion

- Choose one narrow next step:
  - auth/RBAC contract hardening plan before selective enforcement
  - real upload / NAS integration placeholder replacement
  - frontend-driven query/detail contract refinements discovered during integration
