# PHASE_AUTO_063

## Why This Phase Now
- The repo already had minimal session-backed auth, but frontend registration and current-user payloads still lacked the new business fields and explicit department visibility contract.
- The narrowest high-value increment was to make registration, login, `/me`, change-password, and frontend menu/page gating directly usable.
- Config-backed super-admin bootstrap and department-admin keys were required now; a full permission-platform rebuild was not.

## Current Context
- Current CURRENT_STATE before this phase: Step 60 complete plus Step 52/56 auth baseline
- Current OpenAPI version before this phase: `0.62.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_062.md`
- Mainline focus: frontend-ready minimal auth upgrade, fixed departments, config-backed admin bootstrap, explicit frontend_access

## Goals
- Expand `POST /v1/auth/register` to the new frontend fields and validations.
- Keep `POST /v1/auth/login` and `GET /v1/auth/me` stable while returning a richer user profile and `frontend_access`.
- Add `PUT /v1/auth/password`.
- Seed default super admins from config and support config-managed add/remove behavior.
- Keep scope narrow and avoid org tree, ABAC, row-level permissions, or UI-heavy admin configuration work.

## Allowed Scope
- Identity config / domain / repo / service / handler changes required by the new auth payload and bootstrap rules
- Additive `users` schema expansion
- Explicit frontend access config file loading
- Focused tests and repository-truth doc synchronization

## Forbidden Scope
- Organization hierarchy platform
- Row-level / field-level permission trimming
- SSO / OAuth / external identity provider work
- Full admin configuration UI
- Broader workflow-center redesign

## Expected File Changes
- `config/auth_identity.json`
- `config/frontend_access.json`
- `config/config.go`
- `db/migrations/030_v7_auth_frontend_minimal_upgrade.sql`
- `domain/auth_identity.go`
- `domain/enums.go`
- `domain/frontend_access.go`
- `domain/rbac_placeholder.go`
- `repo/interfaces.go`
- `repo/mysql/identity.go`
- `service/identity_service.go`
- `service/identity_service_test.go`
- `transport/handler/auth.go`
- `transport/http.go`
- `cmd/server/main.go`
- `cmd/api/main.go`
- `docs/api/openapi.yaml`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/phases/PHASE_AUTO_063.md`
- `docs/iterations/ITERATION_063.md`
- `ITERATION_INDEX.md`

## Required API / DB Changes
- API:
  - extend `POST /v1/auth/register`
  - keep `POST /v1/auth/login`
  - keep `GET /v1/auth/me`
  - add `PUT /v1/auth/password`
- DB:
  - extend `users` with `department`, `mobile`, `email`, `is_config_super_admin`
  - add mobile uniqueness index

## Success Criteria
- Frontend can register, login, call `/me`, change password, and drive page/menu visibility from `frontend_access`.
- Fixed department enum is enforced.
- Department-admin key is config-backed and works at registration time.
- Default super admin comes from config-backed bootstrap, not first-user side effects.
- `go test ./...` is attempted, and blocking local policy issues are called out if they prevent execution.
