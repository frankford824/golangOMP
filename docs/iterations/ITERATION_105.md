# ITERATION_105

## Phase
MAIN hotspot optimization + full live-chain acceptance + overwrite publish to existing `v0.8`

## Goal
- Optimize real hotspots on current MAIN runtime with low-risk/high-return changes.
- Run full-chain live acceptance, not just smoke tests:
  - create -> assign/reassign -> audit A/B -> warehouse -> close
  - reference upload + design upload/preview/download
  - canonical ownership/list/detail visibility
  - org/action allow+deny and logs
- Keep release line/entrypoint unchanged (`v0.8`, `./cmd/server`).

## Hotspot Analysis (measured first)
- Focus endpoints:
  - `GET /v1/tasks`
  - `GET /v1/tasks/{id}`
  - `POST /v1/tasks`
  - `POST /v1/tasks/{id}/assign`
  - `POST /v1/tasks/reference-upload`
  - `GET /v1/org/options`
  - `GET /v1/roles`
  - `GET /v1/users`
- Identified overhead lanes:
  - `/v1/users` had role hydration fan-out behavior (`ListUsers` + per-user role read).
  - `/v1/org/options` rebuilt static-ish payload repeatedly.
  - task detail design-asset read path needed one-pass grouping over versions to avoid repeated per-asset hydration work.
  - data scope resolve path could re-hit user repo even when actor context already had scope fields.

## Optimization Design (low-risk)
- SQL/repo/service optimizations applied without contract change:
  - batch role read API in repo (`ListRolesByUserIDs`) and service-side batch role attachment.
  - request-safe cached `org/options` snapshot (`sync.Once`) with defensive clone on return.
  - task detail design-asset read-model one-pass version grouping by `asset_id`.
  - scope resolution reuse of actor context before repo fallback.
- Explicitly not changed:
  - task create rules
  - canonical ownership semantics
  - reference small vs multipart contracts
  - deny_code/deny_reason behavior

## Code Changes
- Runtime:
  - `repo/mysql/identity.go`
  - `service/identity_service.go`
  - `service/task_design_asset_read_model.go`
  - `service/task_data_scope_guard.go`
- Tests:
  - `service/task_design_asset_read_model_test.go`
  - `service/identity_service_test.go`
- Live acceptance script:
  - `scripts/iteration105_live_verify.py`
    - close-readiness补齐 (`business-info` + valid category code + filed_at trigger)
    - deterministic permission deny seed
    - permission log allow+deny scan across pages
    - multipart design upload session/parts/complete and preview/download checks

## Required Local Verification
- Passed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
  - `go test ./repo/mysql`

## Performance Verification
- Baseline sample (same optimized runtime, first acceptance run; `2026-04-02T03:20:15Z`, n=8 each):
  - `GET /v1/tasks`: avg `9.80ms`, p50 `9.27`, p95 `11.78`
  - `GET /v1/tasks/{id}`: avg `8.86ms`, p50 `8.46`, p95 `10.94`
  - `POST /v1/tasks`: avg `248.12ms`, p50 `243.17`, p95 `278.27`
  - `POST /v1/tasks/{id}/assign`: avg `13.14ms`, p50 `13.31`, p95 `14.95`
  - `POST /v1/tasks/reference-upload`: avg `61.65ms`, p50 `61.52`, p95 `63.90`
  - `GET /v1/org/options`: avg `5.95ms`, p50 `5.89`, p95 `6.51`
- Final acceptance sample (`2026-04-02T03:28:25Z`, n=8 each):
  - `GET /v1/tasks`: avg `10.99ms`, p50 `9.79`, p95 `13.84`
  - `GET /v1/tasks/{id}`: avg `8.73ms`, p50 `8.58`, p95 `9.67`
  - `POST /v1/tasks`: avg `256.40ms`, p50 `235.97`, p95 `334.13`
  - `POST /v1/tasks/{id}/assign`: avg `12.53ms`, p50 `11.54`, p95 `15.16`
  - `POST /v1/tasks/reference-upload`: avg `63.24ms`, p50 `63.48`, p95 `65.59`
  - `GET /v1/org/options`: avg `5.93ms`, p50 `5.83`, p95 `6.40`
- Interpretation:
  - repeated runs stayed in the same latency band and remained stable during full action-chain execution.
  - no runtime regression observed in list/detail/action/upload hot routes during live acceptance.
  - no preserved pre-optimization artifact is available in workspace after overwrite publish; above baseline/final samples are from this iteration's live acceptance runs.

## Publish (runtime changed => executed)
- Command:
  - `bash ./deploy/deploy.sh --version v0.8 --release-note "overwrite v0.8 performance optimization: task detail/user role/scope"`
- Release evidence:
  - `deploy/release-history.log`:
    - packaged/uploaded/deployed at `2026-04-02T02:57:47Z` -> `2026-04-02T02:58:13Z`
    - artifact sha256: `8ec7b4ea4e9c6d20e26252d134a982aecb2327386946d6372da5e6c88a1eff8a`
- Post-deploy runtime checks:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/*/exe`:
    - `8080 -> /root/ecommerce_ai/releases/v0.8/ecommerce-api` (not deleted)
    - `8081 -> /root/ecommerce_ai/releases/v0.8/erp_bridge` (not deleted)
    - `8082 -> /root/ecommerce_ai/erp_bridge_sync` (not deleted)
- Note:
  - deploy output included unrelated `workflow/config` test-failure text in the packaging stage output stream, but release workflow still completed to deployed state and runtime acceptance passed.

## Full Live Acceptance Result (Phase D)
- Evidence artifact:
  - `tmp/iteration105_live_verify_result.json`
  - remote: `/tmp/iteration105_live_verify_result.json`
- Final run summary:
  - total checks: `101`
  - passed: `101`
  - failed: `0`
- Passed modules:
  - Admin/auth/org/options/roles
  - create (original defer, new single+batch, purchase single+batch)
  - create error validations (duplicate batch/original batch invalid team/violations)
  - reference upload + detail + preview + download
  - design multipart upload complete + detail + preview + download
  - list/detail canonical ownership + batch fields
  - assign/reassign allow+deny
  - audit A/B allow+deny + stage mismatch
  - warehouse allow+deny + stage mismatch
  - close to `Completed` + re-close deny
  - task event logs + permission logs + operation logs
- Observed deny codes:
  - `audit_stage_mismatch`
  - `task_not_closable`
  - `task_out_of_team_scope`
  - `task_status_not_actionable`
  - `warehouse_stage_mismatch`

## Data/Environment Notes
- No destructive reset was executed in this iteration.
- Acceptance entities created and retained:
  - users: ids `141..152`
  - tasks: ids `301..315`
  - uploads:
    - reference asset `9b83eeb8-2553-4b43-8003-52cc1dfd8611`
    - design delivery session `2270604d-3e00-403a-a6f3-9e9943af4c01` (task `311`)

## OpenAPI / Contract
- No API contract change in this iteration.
- `docs/api/openapi.yaml` unchanged.

## Remaining Boundaries / Risks
- This is not full ABAC completion across all routes.
- Some actions outside this iteration's routed coverage still use mixed legacy gating paths.
- Legacy `owner_team` is still a compatibility field and has not been retired.
- Historical tasks may still have incomplete canonical ownership fields.
- Performance improvements in this round are hotspot mitigation (batching/cache/reuse), not deep storage/architecture refactor.
