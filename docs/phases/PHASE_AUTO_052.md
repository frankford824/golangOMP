# PHASE_AUTO_052

## Why This Phase Now
- The repository drifted into export/upload/policy/platform-entry scaffolding while the requested mainline still lacked real user registration/login and authenticated actor usage.
- Step A and Step B are upstream blockers for the whole P0 chain because task creation and workflow writes were still tied to debug headers plus explicit request-body actor ids.
- This phase restores mainline priority with the smallest safe real-auth slice: local users, bearer sessions, role assignment, permission logs, and actor-derived task writes.

## Current Context
- Current CURRENT_STATE before this phase: Step 51 complete
- Current OpenAPI version before this phase: `0.47.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_051.md`
- Correction focus: stop deepening placeholder platform seams and return to Step A / Step B

## Goals
- Add minimal real user registration / login / current-user APIs.
- Add minimal user management, role assignment, and permission-log query APIs.
- Persist real auth/session state with one bounded local DB model.
- Let key task-flow write APIs default actor ids from the authenticated request actor.
- Keep debug-header compatibility for existing internal flows while preferring bearer session tokens.

## Allowed Scope
- Additive identity/auth DB tables, domain models, repos, services, handlers, and router wiring
- Route-level permission decision logging
- Authenticated-actor defaults for mainline task-flow write routes
- Focused tests for auth/session middleware and identity service
- OpenAPI/state/iteration/handover synchronization

## Forbidden Scope
- SSO / OAuth / external identity providers
- Org hierarchy sync
- Deep RBAC / ABAC / row-level visibility engine
- Real ERP executor platform
- Real upload/NAS/object-storage work
- Finance / KPI / report platform deepening

## Expected File Changes
- `db/migrations/025_v7_identity_auth_minimal.sql`
- `domain/auth_identity.go`
- `repo/interfaces.go`
- `repo/mysql/identity.go`
- `service/identity_service.go`
- `transport/auth_placeholder.go`
- `transport/handler/auth.go`
- `transport/handler/user_admin.go`
- `transport/handler/task*.go`
- `transport/handler/design_submission.go`
- `transport/handler/warehouse.go`
- `transport/handler/audit_v7.go`
- `transport/http.go`
- `cmd/server/main.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/iterations/ITERATION_052.md`

## Required API / DB Changes
- DB:
  - add `users`
  - add `user_roles`
  - add `user_sessions`
  - add `permission_logs`
- API:
  - add `/v1/auth/register`
  - add `/v1/auth/login`
  - add `/v1/auth/me`
  - add `/v1/roles`
  - add `/v1/users`
  - add `/v1/users/{id}`
  - add `/v1/users/{id}/roles`
  - add `/v1/permission-logs`
  - make key task-flow actor-id request fields optional with authenticated fallback

## Success Criteria
- A real local user can register, log in, and call authenticated APIs with a bearer token.
- Admin can view users, assign roles, and inspect permission decision logs.
- Protected route access decisions are persisted into `permission_logs`.
- Core task-flow writes no longer require explicit actor ids when the caller is authenticated.
- OpenAPI + CURRENT_STATE + iteration/handover docs are synchronized to Step 52 and `v0.48.0`.
