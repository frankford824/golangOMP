# ITERATION_052

## Phase
- PHASE_AUTO_052 / minimal real auth / role assignment / permission logs / actor-derived task writes

## Input Context
- Current CURRENT_STATE before execution: Step 51 complete
- Current OpenAPI version before execution: `0.47.0`
- Read latest iteration: `docs/iterations/ITERATION_051.md`
- Current execution phase file: `docs/phases/PHASE_AUTO_052.md`

## Goals
- Restore mainline priority to Step A and Step B.
- Add minimal real user registration/login/current-user support.
- Add minimal user/role management and permission decision logs.
- Remove the need for explicit actor ids on key task-flow write APIs when authenticated.

## Files Changed
- `db/migrations/025_v7_identity_auth_minimal.sql`
- `domain/auth_identity.go`
- `domain/errors.go`
- `domain/rbac_placeholder.go`
- `repo/interfaces.go`
- `repo/mysql/identity.go`
- `service/identity_service.go`
- `service/identity_service_test.go`
- `transport/auth_placeholder.go`
- `transport/auth_placeholder_test.go`
- `transport/handler/actor.go`
- `transport/handler/auth.go`
- `transport/handler/user_admin.go`
- `transport/handler/response.go`
- `transport/handler/task.go`
- `transport/handler/task_assignment.go`
- `transport/handler/task_asset.go`
- `transport/handler/design_submission.go`
- `transport/handler/warehouse.go`
- `transport/handler/audit_v7.go`
- `transport/http.go`
- `cmd/server/main.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/phases/PHASE_AUTO_052.md`
- `docs/iterations/ITERATION_052.md`

## DB / Migration Changes
- Added DB migration `025_v7_identity_auth_minimal.sql`.
- New tables:
  - `users`
  - `user_roles`
  - `user_sessions`
  - `permission_logs`

## API Changes
- OpenAPI version advanced from `0.47.0` to `0.48.0`.
- Added real auth APIs:
  - `POST /v1/auth/register`
  - `POST /v1/auth/login`
  - `GET /v1/auth/me`
- Added minimal admin APIs:
  - `GET /v1/roles`
  - `GET /v1/users`
  - `GET /v1/users/{id}`
  - `PATCH /v1/users/{id}`
  - `PUT /v1/users/{id}/roles`
  - `GET /v1/permission-logs`
- Auth middleware now prefers `Authorization: Bearer <token>` and still accepts debug-header fallback.
- Key task-flow write requests now treat these actor-id fields as optional when authenticated:
  - `creator_id`
  - `operator_id`
  - `assigned_by`
  - `uploaded_by`
  - `auditor_id`
  - `from_auditor_id`
  - `receiver_id`

## Design Decisions
- Chose local session-token auth rather than JWT/SSO to unblock the mainline without entering high-risk identity infrastructure.
- Kept first-user bootstrap as `Admin` so the repo gains a usable permission-management entry point immediately.
- Logged route-level permission decisions instead of designing a new approval/authorization event subsystem.
- Kept debug-header compatibility to avoid breaking existing tests and internal placeholder flows while shifting the preferred path to real bearer sessions.

## Correction Notes
- Corrected the post-Step-51 direction away from more placeholder platform deepening and back to the requested mainline Step A / Step B.
- Reconciled stale repository-truth docs:
  - `docs/V7_MODEL_HANDOVER_APPENDIX.md` still pointed to Step 50 / `v0.46.0`
  - current repo truth is now Step 52 / `v0.48.0`
- Reconciled OpenAPI with runtime behavior so authenticated actor-id fallback is documented instead of leaving request fields incorrectly marked as always required.

## Risks / Known Gaps
- This is still local session-token auth only:
  - no SSO
  - no org sync
  - no deep RBAC/ABAC
- Role management is minimal replacement-based assignment over the existing route-role model; it is not a full permission engine.
- Some non-core write routes still keep explicit actor-id payloads and should be aligned in a later bounded pass if they become mainline blockers.
- ERP service docking is still pending; current original-product search remains over already-synced local ERP product data.

## Suggested Next Step
- Stop automatic continuation in this round.
- If continuing later, prioritize one bounded mainline phase:
  - ERP existing-service docking for original-product search/selection
  - or remaining mainline write-route actor derivation cleanup if frontend integration hits those routes first
