# PHASE_AUTO_038 - Integration Center Execution Boundary Hardening

## Why This Phase Now
- Step 37 completed the task-asset storage/upload boundary and left integration center as the next highest-value platform gap.
- Current integration center already has static connectors plus placeholder call logs, but it still mixes request-envelope lifecycle and execution lifecycle too closely.
- Future ERP/HTTP/SDK executors, callback processors, and retry schedulers need one stable execution seam before any real external integration lands.

## Current Context
- Current state: `V7 Migration Step 37 complete`
- Current OpenAPI version before this phase: `0.33.0`
- Latest iteration: `docs/iterations/ITERATION_037.md`
- Stable skeletons already present:
  - task/workflow mainline
  - audit/handover/outsource/warehouse loop
  - board/workbench/filter convergence
  - category center + mapped product search + `product_selection`
  - export placeholder platform
  - integration connector catalog + call-log skeleton
  - task asset storage/upload placeholder boundary
- Current main gap:
  - integration center execution boundary hardening

## Goals
- Add an explicit placeholder execution-attempt layer beneath integration call logs.
- Keep call-log lifecycle and execution lifecycle visibly separate but connected.
- Preserve the existing connector catalog, call-log contract, and `can_replay` hint while adding clearer execution summaries such as `latest_execution`, `execution_count`, and `can_retry`.
- Keep all semantics placeholder-only.

## Allowed Scope
- additive integration-center schema evolution only
- new placeholder execution persistence beneath `integration_call_logs`
- new domain / repo / service / handler code for execution list/create/advance
- call-log read-model hardening and backward-compatible `/advance` reuse
- OpenAPI / state / iteration / handover synchronization

## Forbidden Scope
- real ERP / HTTP / SDK execution
- real callback processor
- real retry scheduler
- real external signing / auth
- real message queue or async platform
- export real runner / storage implementation
- cost-rule governance / versioning work

## Expected File Changes
- add one migration for integration execution persistence
- add integration execution domain / repo / service / handler code
- update integration-center routes with:
  - `GET /v1/integration/call-logs/{id}/executions`
  - `POST /v1/integration/call-logs/{id}/executions`
  - `POST /v1/integration/call-logs/{id}/executions/{execution_id}/advance`
- harden `GET /v1/integration/call-logs` and `GET /v1/integration/call-logs/{id}` with execution summaries
- update OpenAPI, `CURRENT_STATE.md`, `ITERATION_038.md`, and handover docs

## Required API / DB Changes
- DB:
  - add `integration_call_executions`
- API:
  - add internal placeholder execution list/create/advance routes
  - extend call-log read models with:
    - `latest_execution`
    - `execution_count`
    - `can_retry`
  - keep `POST /v1/integration/call-logs/{id}/advance` as backward-compatible compatibility route that now reuses execution semantics

## Success Criteria
- one call log can expose zero-to-many execution attempts without collapsing both lifecycles into one status machine
- execution records express at least:
  - `execution_id`
  - `call_log_id`
  - `connector_key`
  - `execution_mode`
  - `trigger_source`
  - `status`
  - `started_at`
  - `finished_at`
  - `error_message`
  - `adapter_note`
  - `retryable`
- call-log detail and list now surface stable execution summaries
- OpenAPI and docs clearly state this is still an integration execution skeleton, not a real external-call platform
- tests pass without introducing real external execution infrastructure

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_038.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`

## Risks
- call-log compatibility must not regress existing internal placeholder consumers
- new execution states must stay clearly separate from proof of real external delivery
- `can_replay` and `can_retry` must stay aligned to avoid contradictory troubleshooting signals

## Completion Output Format
1. changed files
2. DB / migration changes
3. API changes
4. correction notes
5. risks / remaining gaps
6. next recommended single phase
