# ITERATION_038 - Integration Center Execution Boundary Hardening

**Date**: 2026-03-10  
**Scope**: `docs/phases/PHASE_AUTO_038.md`

## 1. Goals
- Execute exactly one new phase after Step 37:
  - integration center execution boundary hardening
- Keep existing connector catalog, call-log contract, and replay hint additive and non-regressive.
- Avoid real ERP/HTTP/SDK execution, callback processing, retry scheduling, signing/auth negotiation, and async infrastructure while making the future seam explicit.

## 2. Inputs
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_037.md`
- `docs/phases/PHASE_AUTO_038.md`
- latest PRD / V7 implementation spec
- `AGENT_PROTOCOL.md`
- `AUTO_PHASE_PROTOCOL.md`

## 3. Files Changed
- `docs/phases/PHASE_AUTO_038.md`
- `db/migrations/021_v7_integration_execution_boundary.sql`
- `domain/integration_center.go`
- `repo/interfaces.go`
- `repo/mysql/integration_call_execution.go`
- `service/integration_center_service.go`
- `service/integration_center_service_test.go`
- `transport/handler/integration_center.go`
- `transport/http.go`
- `cmd/server/main.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_038.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`

## 4. DB / Migration Changes
- Added migration `021_v7_integration_execution_boundary.sql`.
- Added `integration_call_executions` for placeholder execution-attempt persistence beneath `integration_call_logs`.
- Current execution records persist:
  - `execution_id`
  - `call_log_id`
  - `connector_key`
  - `execution_no`
  - `execution_mode`
  - `trigger_source`
  - `status`
  - `status_updated_at`
  - `started_at`
  - `finished_at`
  - `error_message`
  - `adapter_note`
  - `retryable`

## 5. API Changes
- Added internal placeholder APIs:
  - `GET /v1/integration/call-logs/{id}/executions`
  - `POST /v1/integration/call-logs/{id}/executions`
  - `POST /v1/integration/call-logs/{id}/executions/{execution_id}/advance`
- Extended existing call-log read model additively:
  - `execution_count`
  - `latest_execution`
  - `can_retry`
  - backward-compatible `can_replay` remains and is now aligned to `can_retry`
- Kept existing compatibility route:
  - `POST /v1/integration/call-logs/{id}/advance`
  - `queued` still requeues the parent call log directly
  - `sent|succeeded|failed|cancelled` now reuse execution semantics instead of remaining a separate lifecycle implementation

## 6. Design Decisions
- Kept call log as the request-envelope / business-intent record:
  - connector
  - operation
  - request payload
  - latest business-visible outcome summary
- Introduced execution as the placeholder concrete-attempt layer:
  - one call log can have zero or many executions
  - `latest_execution` is only a summary, not the call log itself
- Chose additive read-model hardening instead of renaming or removing `can_replay`.
- Reused the export-center attempt/dispatch separation pattern conceptually, but did not copy export scheduler semantics into integration center.
- Left real ERP/HTTP/SDK executors, callback processors, retry schedulers, signing/auth, and queueing outside this phase.

## 7. Layering Clarification
- `integration_call_logs`:
  - owns request envelope and latest lifecycle summary
  - remains the troubleshooting anchor for connector/operation/resource context
- `integration_call_executions`:
  - owns one concrete placeholder execution attempt
  - records execution status progression such as `prepared -> dispatched -> received -> completed|failed|cancelled`
- connector catalog:
  - remains static metadata only
  - does not imply a real external adapter implementation exists
- Future real ERP/HTTP/SDK execution should attach beneath the execution boundary, not inside call-log tables or route contracts.

## 8. Correction Notes
- `docs/V7_MODEL_HANDOVER_APPENDIX.md` header was stale before this round:
  - still showed Step 35 complete
  - still pointed to `ITERATION_035`
  - still claimed OpenAPI `0.32.0`
- This iteration corrected that drift while documenting the new Step 38 execution boundary and advancing OpenAPI from `0.33.0` to `0.34.0`.

## 9. Verification
- `gofmt -w` on all changed Go files
- `go test ./service -run IntegrationCenter`
- `go test ./...`
- OpenAPI YAML remained synchronized with implemented routes and response models

## 10. Risks / Known Gaps
- No real ERP or external HTTP/SDK execution exists yet.
- No callback processor exists yet.
- No retry scheduler exists yet.
- `can_retry` / `can_replay` are troubleshooting hints only; they are not proof that retry automation exists.
- Call-log and execution layering is explicit now, but idempotency policies, signature/auth negotiation, and delivery guarantees are still future work.

## 11. Next Batch Recommended Roadmap
1. Step 39: export runner / storage boundary planning hardening
2. Step 40: cost-rule governance / versioning / override hardening
3. Keep real external execution infrastructure deferred until these remaining boundary phases stay stable
