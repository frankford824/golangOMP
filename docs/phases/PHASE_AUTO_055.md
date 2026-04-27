# PHASE_AUTO_055

## Why This Phase Now
- Step 52 added real register/login/me support, but the main request path still treated debug actors and fallback actors too much like normal users.
- That left Step A only partially complete because authenticated-user semantics and user-scoped behavior were still too placeholder-first.
- The next bounded gain was therefore to harden actor precedence and fallback semantics without jumping into a full auth/RBAC redesign.

## Current Context
- Current CURRENT_STATE before this phase: Step 54 complete
- Current OpenAPI version before this phase: `0.54.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_054.md`
- Mainline focus: Step A request-actor hardening, not Step B/C/D/E expansion

## Goals
- Make bearer/session-backed identity the primary actor path for mainline authenticated behavior.
- Narrow debug-header semantics so they are explicit compatibility behavior instead of silent real-user substitutes.
- Remove the normal implicit `system_fallback` actor from the main request path.
- Make user-scoped workbench preference behavior session-first.
- Keep register/login/me working and update docs/tests honestly.

## Allowed Scope
- Request-actor middleware precedence tightening
- Authenticated helper tightening
- Ready-route actor-id derivation narrowing
- User-scoped workbench preference hardening
- Focused tests
- OpenAPI/state/handover/phase/iteration synchronization

## Forbidden Scope
- Broad RBAC redesign
- SSO / OAuth / external identity providers
- Org hierarchy or row-level visibility
- ERP writeback or deeper ERP docking
- Export/upload/integration platform expansion unrelated to actor hardening
- Broad Step B/C/D/E work

## Expected File Changes
- `transport/auth_placeholder.go`
- `transport/http.go`
- `transport/handler/actor.go`
- `service/identity_service.go`
- `service/context.go`
- `service/workbench_service.go`
- `domain/rbac_placeholder.go`
- focused auth/workbench/actor tests
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/phases/PHASE_AUTO_055.md`
- `docs/iterations/ITERATION_055.md`

## Required API / DB Changes
- API:
  - keep `/v1/auth/register`, `/v1/auth/login`, `/v1/auth/me` working
  - make `/v1/auth/me` explicitly session-token-only
  - clarify session-first plus explicit-debug compatibility semantics for `/v1/workbench/preferences`
  - narrow implicit actor-id fallback semantics for ready-for-frontend write routes
- DB:
  - no new migration in this phase

## Success Criteria
- Register/login/current-user still work.
- Bearer/session actor clearly wins and defines authenticated-user behavior.
- Normal no-header requests no longer get a silent mainline actor `1`.
- Debug-header fallback is narrower than before and explicit in docs.
- Workbench preferences are more real-user-first.
- `go test ./...` passes.
