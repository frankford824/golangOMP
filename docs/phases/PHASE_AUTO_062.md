# PHASE_AUTO_062

## Why This Phase Now
- The repo already had minimal local register/login/session support, but frontend联调 still lacked one stable, frontend-friendly permission contract.
- The highest-value narrow increment was not a broader permission-platform redesign; it was to make register/login/me, admin role changes, and permission logs truly usable end-to-end.
- The existing `permission_logs` table and role-based route metadata were sufficient foundations, so this phase focuses on contract completion rather than platform reconstruction.

## Current Context
- Current CURRENT_STATE before this phase: Step 60 complete
- Current OpenAPI version before this phase: `0.60.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_061.md`
- Mainline focus: frontend-ready auth mainline, role assignment, and permission-action auditability

## Goals
- Keep local username/password + bearer-session auth as the bounded mainline.
- Make `POST /v1/auth/register`, `POST /v1/auth/login`, and `GET /v1/auth/me` return a stable frontend-facing visibility contract.
- Let admins add, replace, and remove user roles with immediate `/me` reflection.
- Extend permission logs from pure route-access decisions to key auth/role actions.
- Keep all changes additive and avoid org tree, ABAC, SSO, or permission-platform rewrites.

## Allowed Scope
- Identity domain/service/repo/handler changes required by auth mainline completion
- Additive permission-log schema/read-model changes
- Admin role-add / role-remove HTTP APIs
- Frontend visibility contract fields on auth/current-user responses
- Focused tests and repository-truth doc synchronization

## Forbidden Scope
- Org hierarchy / department tree
- Row-level or field-level data permissions
- SSO / OAuth / third-party login
- Large RBAC/ABAC engine redesign
- Unrelated workflow-center or platform-center expansion

## Expected File Changes
- `db/migrations/029_v7_permission_log_action_audit.sql`
- `domain/auth_identity.go`
- `domain/frontend_access.go`
- `repo/interfaces.go`
- `repo/mysql/identity.go`
- `service/identity_service.go`
- `service/identity_service_test.go`
- `transport/auth_placeholder.go`
- `transport/http.go`
- `transport/handler/user_admin.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/phases/PHASE_AUTO_062.md`
- `docs/iterations/ITERATION_062.md`
- `ITERATION_INDEX.md`

## Required API / DB Changes
- API:
  - keep `POST /v1/auth/register`, `POST /v1/auth/login`, `GET /v1/auth/me`
  - add `POST /v1/users/{id}/roles`
  - add `DELETE /v1/users/{id}/roles/{role}`
  - keep `PUT /v1/users/{id}/roles` as full replacement
  - extend `GET /v1/permission-logs` with action and target-user filters
- DB:
  - extend `permission_logs` with `action_type`, `target_user_id`, `target_username`, and `target_roles_json`

## Success Criteria
- Register/login/me are stable for frontend integration.
- `/me` returns role plus page/menu/module/permission visibility fields.
- Admin role changes are immediately reflected in the effective current-user contract.
- Permission logs can answer who registered, logged in, failed login, assigned roles, or removed roles.
- `go test ./...` passes.
