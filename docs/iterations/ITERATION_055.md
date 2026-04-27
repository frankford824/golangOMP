# ITERATION_055

## Phase
- PHASE_AUTO_055 / Step A actor precedence hardening

## Input Context
- Current CURRENT_STATE before execution: Step 54 complete
- Current OpenAPI version before execution: `0.54.0`
- Read latest iteration: `docs/iterations/ITERATION_054.md`
- Current execution phase file: `docs/phases/PHASE_AUTO_055.md`

## Goals
- Make the mainline request actor path real-user-first.
- Keep register/login/current-user working while narrowing debug-header and placeholder fallback semantics.
- Tighten user-scoped behavior for workbench preferences.
- Synchronize tests and docs with the narrowed actor contract.

## Files Changed
- `domain/rbac_placeholder.go`
- `service/context.go`
- `service/identity_service.go`
- `service/identity_service_test.go`
- `service/workbench_service.go`
- `service/workbench_service_test.go`
- `transport/auth_placeholder.go`
- `transport/auth_placeholder_test.go`
- `transport/http.go`
- `transport/handler/actor.go`
- `transport/handler/actor_test.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/phases/PHASE_AUTO_055.md`
- `docs/iterations/ITERATION_055.md`

## DB / Migration Changes
- No new DB migration in this phase.

## API Changes
- OpenAPI version advanced from `0.54.0` to `0.55.0`.
- `GET /v1/auth/me` is now explicitly bearer-session-only.
- `GET /v1/workbench/preferences` and `PATCH /v1/workbench/preferences` are now documented as session-user-first:
  - bearer-session actor is primary
  - debug-header compatibility remains only when `X-Debug-Actor-Id` is explicit
  - requests without session or explicit debug actor id now return `401`
- Ready-for-frontend request-body actor-id fallback now derives automatically only from real session actors.

## Design Decisions
- Removed the normal implicit system-fallback actor from the main request middleware path instead of trying to reinterpret it as a real user.
- Kept debug-header role compatibility for existing placeholder/internal routes, but stopped treating debug actors as equivalent to logged-in users for authenticated-only flows.
- Added explicit user-scoped route enforcement for workbench preferences so route behavior matches service behavior and docs.
- Limited implicit actor-id derivation on ready-for-frontend routes to session actors, while preserving internal placeholder debug compatibility where abrupt removal would be riskier.

## Verification
- Added/updated focused tests for:
  - anonymous normal-request actor behavior
  - `/v1/auth/me` rejecting debug-header-only actors
  - workbench user-scope acceptance/rejection rules
  - actor-id derivation rules for ready vs internal routes
  - identity service current-user session requirement
- `go test ./transport/...`
- `go test ./service/...`
- `go test ./...`

## Risks / Known Gaps
- Step A is still not fully complete:
  - many ready-for-frontend routes still accept debug-header role access as compatibility
  - mainline route protection is not yet a full session-only/authz redesign
  - org/visibility/RBAC depth is still deferred
- Some internal or legacy service paths still keep placeholder/system fallback semantics for non-request contexts and explicit placeholder flows.

## Suggested Next Step
- If frontend/mainline behavior is now no longer blocked by placeholder-first actor handling, Step B can become the next priority.
- If new gaps appear during frontend integration, keep any further Step A work bounded to remaining authenticated-route precedence cleanup rather than broad auth redesign.
