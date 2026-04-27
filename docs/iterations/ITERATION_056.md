# ITERATION_056

## Phase
- PHASE_AUTO_056 / Step B route-access hardening and auditability

## Input Context
- Current CURRENT_STATE before execution: Step 55 complete
- Current OpenAPI version before execution: `0.55.0`
- Read latest iteration: `docs/iterations/ITERATION_055.md`
- Current execution phase file: `docs/phases/PHASE_AUTO_056.md`

## Goals
- Start Step B with the smallest usable authorization increment on top of the Step A real-user-first auth base.
- Make frontend-ready route authorization explicitly session-backed.
- Keep debug-header role compatibility narrow and limited to internal/mock placeholder routes.
- Make permission decisions easier to audit and expose the current route-role contract to admins.

## Files Changed
- `cmd/server/main.go`
- `db/migrations/027_v7_permission_log_route_policy.sql`
- `domain/auth_identity.go`
- `domain/rbac_placeholder.go`
- `repo/interfaces.go`
- `repo/mysql/identity.go`
- `service/identity_service.go`
- `service/identity_service_test.go`
- `service/workbench_service.go`
- `service/workbench_service_test.go`
- `transport/auth_placeholder.go`
- `transport/auth_placeholder_test.go`
- `transport/http.go`
- `transport/route_access_catalog.go`
- `transport/handler/user_admin.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/phases/PHASE_AUTO_056.md`
- `docs/iterations/ITERATION_056.md`

## DB / Migration Changes
- Added migration `027_v7_permission_log_route_policy.sql`.
- `permission_logs` now persist:
  - `readiness`
  - `session_required`
  - `debug_compatible`
- Added read indexes for:
  - `actor_username`
  - `method`

## API Changes
- OpenAPI version advanced from `0.55.0` to `0.56.0`.
- Added admin inspection API:
  - `GET /v1/access-rules`
- `GET /v1/permission-logs` now additionally supports:
  - `actor_username`
  - `method`
- `GET /v1/workbench/preferences` and `PATCH /v1/workbench/preferences` now require a bearer session.
- All `ready_for_frontend` role-gated routes now use `session_token_role_enforced` semantics.

## Design Decisions
- Treated `ready_for_frontend` as the current mainline authorization boundary:
  - session-backed actor required first
  - role match evaluated second
- Kept debug-header route authorization only on `internal_placeholder` and `mock_placeholder_only` paths to preserve narrow compatibility where placeholder flows still exist.
- Avoided a broader RBAC redesign by using existing role assignments, existing route-role metadata, and richer logging instead of introducing permission objects, org hierarchy, or ABAC.
- Kept role inspection narrow by exposing the effective protected-route catalog instead of building a new admin platform.

## Verification
- Added/updated focused tests for:
  - ready-for-frontend routes rejecting debug-header authorization
  - ready-for-frontend routes accepting session-backed actors
  - permission logs carrying actor username and route-policy context
  - existing session tokens resolving the latest assigned roles
  - workbench preferences rejecting debug actors and requiring session scope
- `go test ./transport/...`
- `go test ./service/...`
- `go test ./...`

## Risks / Known Gaps
- Step B is still intentionally narrow:
  - no fine-grained permission object model
  - no role-assignment audit stream beyond route-level access logging
  - no org hierarchy, team scoping, or row-level visibility trimming
- Route-access rule inspection is read-only and mirrors current route registration; it is not a dynamic policy authoring system.
- Internal/mock placeholder routes still allow debug-header compatibility by design.

## Suggested Next Step
- Continue Step B with the next narrow increment only if needed:
  - role-assignment change audit trail
  - small admin inspection improvements over current access logs/rules
- If current authorization behavior is sufficient for the mainline, Step C can become the next priority.
