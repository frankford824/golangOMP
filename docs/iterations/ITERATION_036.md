# ITERATION_036 - Phase Audit / Placeholder Auth RBAC Hardening

**Date**: 2026-03-10  
**Scope**: `docs/phases/PHASE_AUDIT_036.md`

## 1. Goals
- Run a repository-truth phase audit before starting any new feature work.
- Re-rank Step 36 to Step 40 from actual code and document state.
- Execute exactly one new phase:
  - placeholder auth / RBAC route enforcement hardening

## 2. Inputs
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_035.md`
- `docs/iterations/ITERATION_034.md`
- latest PRD / V7 implementation spec

## 3. Audit Outcome
- Stable skeletons confirmed:
  - task/workflow mainline
  - audit/handover/outsource/warehouse loop
  - board/workbench/filter convergence
  - category center + category mappings + mapped product search + `product_selection`
  - export center placeholder platform skeleton
  - integration center placeholder call-log skeleton
- Placeholder-heavy seams confirmed:
  - auth / org / visibility boundary
  - task asset storage/upload boundary
  - real integration execution boundary
  - export runner/storage boundary
  - deeper cost-rule governance
  - KPI / finance / deeper reporting layers
- Highest-impact current gap:
  - auth / org / visibility / role boundary drift
- Recommended Step 36 to Step 40 order:
  1. route-level placeholder auth / RBAC enforcement
  2. task asset storage / upload adapter boundary
  3. integration center execution boundary hardening
  4. export runner / storage boundary planning
  5. cost-rule governance / versioning / override hardening

## 4. Files Changed
- `docs/phases/PHASE_AUDIT_036.md`
- `domain/rbac_placeholder.go`
- `transport/auth_placeholder.go`
- `transport/auth_placeholder_test.go`
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_036.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`

## 5. DB / Migration Changes
- None.

## 6. API / OpenAPI Changes
- No new endpoints were added.
- V7 routes carrying `withAccessMeta(...)` now enforce declared `required_roles` using current debug actor headers.
- Current placeholder auth mode is now `debug_header_role_enforced`.
- Requests that do not satisfy route role requirements now return `403 PERMISSION_DENIED`.
- `Admin` is currently accepted as a placeholder route-level override.
- OpenAPI advanced to `v0.32.1`.

## 7. Design Decisions
- Kept scope narrow:
  - no login/session system
  - no org hierarchy
  - no team/department visibility trimming
  - no field-level permission model
- Reused existing route metadata instead of inventing a second permission declaration layer.
- Preserved placeholder identity source:
  - `X-Debug-Actor-Id`
  - `X-Debug-Actor-Roles`

## 8. Correction Notes
- Repository docs previously stated that placeholder role metadata was advisory only and no requests were rejected.
- Step 36 corrects that drift by making route-level role checks real for V7 routes already declaring `required_roles`.

## 9. Verification
- `gofmt -w domain/rbac_placeholder.go transport/auth_placeholder.go transport/auth_placeholder_test.go`
- `go test ./...`
- OpenAPI YAML parse validation via `python` + `yaml.safe_load`

## 10. Risks / Known Gaps
- This is still not a real auth platform.
- Role checks still depend on debug headers rather than login identity.
- No org/account/session model exists yet.
- No task-level visibility or per-field permission trimming exists yet.
- Task asset write flow is still `mock-upload` only and remains the next major PRD gap.

## 11. Next Batch Recommended Roadmap
1. Step 37: task asset storage / upload adapter boundary hardening
2. Step 38: integration center execution boundary hardening
3. Stop for confirmation before export runner/storage planning
4. Stop for confirmation before cost-rule governance/versioning work
5. Do not enter KPI / finance / deep reporting layers without user confirmation
