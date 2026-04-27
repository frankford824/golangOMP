# PHASE_AUTO_056

## Why This Phase Now
- Step A is materially hardened enough that session-backed identity can become the real mainline authorization base.
- The largest remaining Step B ambiguity was that ready-for-frontend routes still accepted debug-header roles as if they were normal users.
- The next bounded gain is therefore to tighten the route-auth contract, expose it for admin inspection, and improve access-decision auditability without building a larger RBAC platform.

## Current Context
- Current CURRENT_STATE before this phase: Step 55 complete
- Current OpenAPI version before this phase: `0.55.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_055.md`
- Mainline focus: Step B role/permission configuration, assignment consistency, and permission/access logs

## Goals
- Require session-backed actors on `ready_for_frontend` protected routes.
- Keep debug-header authorization only as narrow compatibility for internal/mock placeholder routes.
- Make route/role requirements inspectable through one admin-facing API.
- Persist richer permission-log context for who/what/why auditing.
- Keep role assignment behavior simple and consistent with request actor resolution.

## Allowed Scope
- Route-access policy hardening on top of existing auth/session behavior
- Narrow admin inspection API for protected route rules
- Permission-log persistence/readability improvements
- Focused workbench alignment where current route/service behavior would otherwise disagree
- OpenAPI/state/handover/phase/iteration synchronization
- Focused tests

## Forbidden Scope
- Broad RBAC or ABAC redesign
- Org hierarchy / department tree / row-level visibility
- SSO / external identity providers
- ERP writeback or Bridge mutation work
- Task-flow expansion unrelated to authorization hardening
- Large admin platform scaffolding

## Expected File Changes
- `domain/rbac_placeholder.go`
- `domain/auth_identity.go`
- `transport/auth_placeholder.go`
- `transport/http.go`
- `transport/route_access_catalog.go`
- `transport/handler/user_admin.go`
- `service/identity_service.go`
- `service/workbench_service.go`
- `repo/interfaces.go`
- `repo/mysql/identity.go`
- focused auth/identity/workbench tests
- `db/migrations/027_v7_permission_log_route_policy.sql`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/phases/PHASE_AUTO_056.md`
- `docs/iterations/ITERATION_056.md`

## Required API / DB Changes
- API:
  - add `GET /v1/access-rules`
  - extend `GET /v1/permission-logs` filter support with `actor_username` and `method`
  - document `ready_for_frontend` protected routes as session-backed
  - document workbench preferences as bearer-session-only
- DB:
  - extend `permission_logs` with route-policy context fields needed for Step B auditing

## Success Criteria
- Step A real-user-first behavior remains intact.
- Ready-for-frontend protected routes no longer use debug headers as the primary authorization path.
- Existing role assignments continue to drive request actor resolution and route checks consistently.
- Permission logs are more useful for backend debugging and later admin inspection.
- Route-role requirements are inspectable without introducing a broad policy platform.
- `go test ./...` passes.
