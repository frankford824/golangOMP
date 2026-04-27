# ITERATION_066

## Phase
- PHASE_AUTO_066 / department-team auth mainline completion for frontend integration

## Input Context
- Current CURRENT_STATE before execution: Step 65 complete with Step 63 auth baseline already in place
- Current OpenAPI version before execution: `0.63.0`
- Read latest iteration: `docs/iterations/ITERATION_063.md`
- Current execution phase file: `docs/phases/PHASE_AUTO_066.md`

## Goals
- Keep the auth/org model flat and configurable while making frontend registration/login/current-user flow directly usable.
- Persist department-team membership and expose fixed registration options.
- Give HR a minimal readable org/log visibility surface.
- Keep ERP query visible to every authenticated user.

## Files Changed
- `cmd/api/main.go`
- `cmd/server/main.go`
- `config/auth_identity.json`
- `config/config.go`
- `config/config_test.go`
- `config/frontend_access.json`
- `CURRENT_STATE.md`
- `db/migrations/031_v7_org_team_auth_extension.sql`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_066.md`
- `docs/phases/PHASE_AUTO_066.md`
- `domain/auth_identity.go`
- `domain/frontend_access.go`
- `domain/operation_log.go`
- `MODEL_HANDOVER.md`
- `repo/interfaces.go`
- `repo/mysql/export_job_event.go`
- `repo/mysql/identity.go`
- `repo/mysql/task_event.go`
- `service/export_center_service_test.go`
- `service/identity_service.go`
- `service/identity_service_test.go`
- `service/operation_log_service.go`
- `service/task_prd_service_test.go`
- `service/task_step04_service_test.go`
- `transport/handler/auth.go`
- `transport/handler/user_admin.go`
- `transport/http.go`
- `ITERATION_INDEX.md`

## DB / Migration Changes
- Added migration `031_v7_org_team_auth_extension.sql`.
- `users` now additionally persist:
  - `team`
- Added `(department, team)` index to support current flat org lookup/read patterns.

## API Changes
- OpenAPI version advanced from `0.63.0` to `0.66.0`.
- Added `GET /v1/auth/register-options`.
- `POST /v1/auth/register` now additionally accepts:
  - `team`
  - compatibility `group`
- `WorkflowUser` now additionally exposes:
  - `team`
  - compatibility `group`
- `frontend_access` now additionally exposes:
  - `team`
  - `roles`
  - `scopes`
  - `menus`
  - `pages`
  - `actions`
  - `modules`
- Added `GET /v1/operation-logs`.
- `GET /v1/erp/products`
- `GET /v1/erp/products/{id}`
- `GET /v1/erp/categories`
  - now all require authentication only; no role subset is required.

## Design Decisions
- Kept the org model flat: fixed departments plus per-department team lists only.
- Reused the existing auth/identity/role stack instead of introducing a separate organization subsystem.
- Kept HR visibility read-only/minimal by lifting read access on org/log inspection endpoints instead of inventing a new audit platform.
- Used one aggregated operation-log read model to minimize frontend integration cost across task/export/integration process traces.

## Verification
- `gofmt -w service/identity_service.go service/operation_log_service.go`
- `go test ./...`
