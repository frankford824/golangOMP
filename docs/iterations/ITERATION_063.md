# ITERATION_063

## Phase
- PHASE_AUTO_063 / frontend-ready minimal auth upgrade with fixed departments and explicit frontend access config

## Input Context
- Current CURRENT_STATE before execution: Step 60 complete plus Step 52/56 auth baseline
- Current OpenAPI version before execution: `0.62.0`
- Read latest iteration: `docs/iterations/ITERATION_062.md`
- Current execution phase file: `docs/phases/PHASE_AUTO_063.md`

## Goals
- Expand register/login/me to the new frontend payload shape without rebuilding the whole permission platform.
- Enforce fixed departments plus basic mobile/email validation.
- Move department-admin key and default super-admin bootstrap into config-backed behavior.
- Add current-user password change.
- Return explicit config-driven `frontend_access` so frontend can drive menu/page visibility.

## Files Changed
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

## DB / Migration Changes
- Added migration `030_v7_auth_frontend_minimal_upgrade.sql`.
- `users` now additionally persist:
  - `department`
  - `mobile`
  - `email`
  - `is_config_super_admin`
- Added `uq_users_mobile` for current phone-unique policy.

## API Changes
- OpenAPI version advanced from `0.62.0` to `0.63.0`.
- `POST /v1/auth/register` now accepts:
  - `account` / compatibility `username`
  - `name` / compatibility `display_name`
  - `department`
  - `mobile` / compatibility `phone`
  - optional `email`
  - `password`
  - optional `admin_key` / compatibility `secret_key`
- Added `PUT /v1/auth/password`.
- `WorkflowUser` now additionally exposes:
  - `account`
  - `name`
  - `department`
  - `mobile`
  - `phone`
  - `email`
- `frontend_access` now additionally exposes:
  - `is_department_admin`
  - `department`
  - `managed_departments`

## Design Decisions
- Replaced the old "first registered user becomes Admin" bootstrap with config-managed super-admin seeding.
- Kept department-admin capability as one explicit role marker (`DepartmentAdmin`) plus department-scoped frontend access, instead of introducing org-tree or row-level policy.
- Stored default auth/frontend visibility rules in explicit JSON files so later maintenance can happen by config edit without needing a UI first.
- Kept email optional and non-unique; kept mobile required and unique for the current minimal business contract.

## Verification
- Updated focused tests for:
  - extended register payload and department-admin promotion
  - invalid department / duplicate mobile rejection
  - change-password and relogin
  - config-managed default super-admin bootstrap
  - role add/remove permission logs
- `go test ./service ./transport ./transport/handler ./cmd/server ./cmd/api ./config ./workers`
- `go test ./...`
  - local policy note: `workflow/repo/mysql` and `workflow/tests` test binaries are blocked by host application-control policy during execution on this machine
