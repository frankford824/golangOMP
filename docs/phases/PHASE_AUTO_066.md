# PHASE_AUTO_066

## Goal
- Continue the real auth/org mainline to the frontend-linkable minimum usable slice.
- Keep the organization model flat and explicit: department + team + user.
- Expose HR-readable permission/process logs without widening into a full audit platform.

## Required Scope
- Extend register/login/me with `team` and richer `frontend_access`.
- Add `GET /v1/auth/register-options`.
- Persist `users.team` and enforce department-team relation on register.
- Keep department-admin key and default super-admin bootstrap config-driven.
- Add aggregated `GET /v1/operation-logs`.
- Make `/v1/erp/*` visible to all authenticated users.
- Sync OpenAPI, CURRENT_STATE, MODEL_HANDOVER, and iteration memory.

## Explicit Non-Goals
- Organization tree.
- Team-admin role.
- Full permission-management UI.
- ABAC / row-level / field-level data-visibility platform.
- Full audit-analysis platform.

## Verification Target
- `go test ./...`
- Confirm register/login/me and change-password still pass service/transport coverage.
- Confirm OpenAPI and state docs reflect the Step 66 contracts.
