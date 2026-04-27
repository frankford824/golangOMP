# PHASE_AUTO_047

## Why This Phase Now
- Step 46 finished hardening the storage/upload placeholder lifecycle and the post-audit priority already identified integration replay/retry as the next safe gap.
- The integration center already has connector catalog, call logs, execution boundary, `latest_execution`, `execution_count`, and `can_retry`, but replay/retry semantics are still collapsed together.
- This is the smallest bounded follow-up that deepens the integration skeleton without entering real ERP/HTTP/SDK execution, callbacks, schedulers, signatures, or async infrastructure.

## Current Context
- Current CURRENT_STATE before this phase: Step 46 complete
- Current OpenAPI version before this phase: `0.42.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_046.md`
- Current main remaining safe gap: integration execution replay / retry hardening

## Goals
- Add explicit internal/admin replay and retry action boundaries above the existing integration execution layer
- Keep replay and retry semantically distinct while reusing `integration_call_executions` as the only persisted execution history
- Expose additive retry/replay counts, latest-action summaries, and retryability/replayability reasons on call-log read models

## Allowed Scope
- Additive integration-center domain/read-model fields
- Additive integration-center service / repo aggregate / handler / route logic
- Internal/admin placeholder routes for replay/retry
- Focused service and transport verification
- Required document synchronization

## Forbidden Scope
- Real ERP / HTTP / SDK execution
- Real callback processor
- Real retry scheduler
- Real signature/auth negotiation
- Message queue or full async platform
- Permission-depth expansion beyond existing internal/admin placeholder boundary

## Expected File Changes
- Update integration-center domain semantics and derived fields
- Update execution aggregate summarization in repo layer
- Add retry/replay service methods and handlers
- Register internal placeholder replay/retry routes
- Extend focused integration-center tests
- Add `docs/phases/PHASE_AUTO_047.md`
- Add `docs/iterations/ITERATION_047.md`
- Update `docs/api/openapi.yaml`
- Update `CURRENT_STATE.md`
- Update `MODEL_HANDOVER.md`
- Update `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Required API / DB Changes
- No new DB tables or migrations
- Add internal/admin placeholder routes:
  - `POST /v1/integration/call-logs/{id}/retry`
  - `POST /v1/integration/call-logs/{id}/replay`
- Add additive integration call-log read-model fields:
  - `retry_count`
  - `replay_count`
  - `latest_retry_action`
  - `latest_replay_action`
  - `retryability_reason`
  - `replayability_reason`
- Add additive integration execution read-model field:
  - `action_type`

## Success Criteria
- Retry and replay are both expressible as internal/admin placeholder actions without introducing a real external execution platform
- Retry admission is narrower than replay admission and both return clear machine-readable reasons
- Retry/replay traceability reuses execution history and latest action summaries rather than adding a second event stream
- OpenAPI and state docs clearly describe this as replay/retry hardening skeleton only

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_047.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Risks
- Later real executor/callback/scheduler work may need richer retry policy inputs than the current placeholder reasons
- Replay currently reuses execution creation semantics; future real connector infrastructure may split “redelivery” from “simulation” more finely

## Completion Output Format
1. Changed files
2. DB / migration changes
3. API changes
4. Correction notes
5. Risks / known gaps
6. Suggested next step
