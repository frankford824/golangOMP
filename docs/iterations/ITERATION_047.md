# ITERATION_047

## Phase
- PHASE_AUTO_047 / integration execution replay / retry hardening

## Input Context
- Current CURRENT_STATE before execution: Step 46 complete
- Current OpenAPI version before execution: `0.42.0`
- Read latest iteration: `docs/iterations/ITERATION_046.md`
- Current execution phase file: `docs/phases/PHASE_AUTO_047.md`

## Goals
- Separate replay and retry semantics on top of the existing integration execution boundary
- Add internal/admin placeholder replay/retry routes without introducing real external execution infrastructure
- Expose retry/replay counts, latest action summaries, and admission reasons on integration call-log read models

## Files Changed
- `docs/phases/PHASE_AUTO_047.md`
- `domain/integration_center.go`
- `repo/interfaces.go`
- `repo/mysql/integration_call_execution.go`
- `service/integration_center_service.go`
- `service/integration_center_service_test.go`
- `transport/handler/integration_center.go`
- `transport/http.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/iterations/ITERATION_047.md`

## DB / Migration Changes
- None
- Step 47 reuses the existing Step 38 persistence:
  - `integration_call_logs`
  - `integration_call_executions`

## API Changes
- OpenAPI version advanced from `0.42.0` to `0.43.0`
- Added internal/admin placeholder routes:
  - `POST /v1/integration/call-logs/{id}/retry`
  - `POST /v1/integration/call-logs/{id}/replay`
- Integration executions now additionally expose:
  - `action_type`
- Integration call logs now additionally expose:
  - `retry_count`
  - `replay_count`
  - `latest_retry_action`
  - `latest_replay_action`
  - `retryability_reason`
  - `replayability_reason`

## Design Decisions
- Chose to keep retry and replay on the existing execution boundary so every new action is still represented by one persisted execution attempt.
- Defined `retry` as a narrower admission path for retryable failed outcomes only.
- Defined `replay` as a broader redrive path for an already recorded call-log envelope, including succeeded and cancelled outcomes.
- Kept history traceability execution-centric by deriving latest retry/replay summaries from `integration_call_executions` rather than adding a second event stream.
- Kept `POST /v1/integration/call-logs/{id}/advance` as a compatibility facade and did not expand into a real external executor, callback processor, retry scheduler, or signature/auth layer.

## Correction Notes
- Corrected repository docs that still described `can_replay` as aligned to `can_retry`; Step 47 now documents and implements them as separate admission hints.
- Updated the V7 handover appendix so future turns do not inherit the older collapsed retry/replay wording.

## Risks / Known Gaps
- Replay/retry are still placeholder actions only:
  - no real ERP / HTTP / SDK execution
  - no callback processor
  - no retry scheduler
  - no signature/auth negotiation
  - no queue or async worker platform
- `retryability_reason` and `replayability_reason` are lightweight admission hints only; future real connector infrastructure may require richer policy metadata.

## Suggested Next Step
- Stop automatic continuation here.
- If the next round stays boundary-safe, Step 48 should target export dispatch / attempt admission hardening without entering real scheduler or storage infrastructure.
