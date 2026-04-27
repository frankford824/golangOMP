# ITERATION_062

## Phase
- PHASE_AUTO_062 / frontend-ready auth mainline and permission-action audit completion

## Input Context
- Current CURRENT_STATE before execution: Step 60 complete
- Current OpenAPI version before execution: `0.60.0`
- Read latest iteration: `docs/iterations/ITERATION_061.md`
- Current execution phase file: `docs/phases/PHASE_AUTO_062.md`

## Goals
- Finish the narrow frontend联调 auth contract on top of the existing local-session baseline.
- Keep register/login/me stable while exposing one frontend-friendly permission visibility shape.
- Let admins add/remove roles and trace those actions in permission logs.
- Avoid drifting into broader auth-platform redesign.

## Files Changed
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

## DB / Migration Changes
- Added migration `029_v7_permission_log_action_audit.sql`.
- `permission_logs` now additionally persist:
  - `action_type`
  - `target_user_id`
  - `target_username`
  - `target_roles_json`

## API Changes
- OpenAPI version advanced from `0.60.0` to `0.62.0`.
- Auth/session responses now additionally expose:
  - `session_id`
  - register/login -> `data.user.frontend_access.*`
  - current-user -> `data.frontend_access.*`
  - `frontend_access.is_super_admin`
  - `frontend_access.permission_flags`
  - `frontend_access.page_keys`
  - `frontend_access.menu_keys`
  - `frontend_access.module_keys`
  - `frontend_access.access_scopes`
- Added admin role mutation APIs:
  - `POST /v1/users/{id}/roles`
  - `DELETE /v1/users/{id}/roles/{role}`
- `GET /v1/permission-logs` now additionally supports:
  - `action_type`
  - `target_user_id`
  - `target_username`

## Design Decisions
- Reused the existing local session-token model instead of switching to JWT/SSO so frontend联调 can proceed immediately.
- Kept the frontend visibility contract additive under `user.frontend_access` rather than introducing a separate permission service.
- Reused the existing `permission_logs` table for auth and role actions instead of creating a second audit stream.
- Added a guard that prevents removing the final remaining `Admin` role from the system; this is a safety constraint, not a platform redesign.

## Verification
- Added/updated focused tests for:
  - register success and duplicate rejection
  - login success and invalid-password failure
  - current-user session requirement and latest-role visibility
  - admin role add/remove behavior
  - permission log action writes for login failure, role assignment, and role removal
- `go test ./service/...`
- `go test ./transport/...`
- `go test ./...`

## Risks / Known Gaps
- This remains local username/password plus bearer-session auth only.
- `frontend_access` is a bounded visibility contract, not a generic policy engine.
- Permission logs are still append-only operational audit records, not a full governance workflow.
- Org hierarchy, row-level visibility, and SSO remain deferred.

## Suggested Next Step
- Keep the next iteration on mainline flow completion or frontend联调 issue fixing; avoid reopening auth/platform scope unless a concrete integration gap appears.
