# ITERATION_048

## Phase
- PHASE_AUTO_048 / export dispatch / attempt admission hardening

## Input Context
- Current CURRENT_STATE before execution: Step 47 complete
- Current OpenAPI version before execution: `0.43.0`
- Read latest iteration: `docs/iterations/ITERATION_047.md`
- Current execution phase file: `docs/phases/PHASE_AUTO_048.md`

## Goals
- Harden export dispatch/start/attempt/redispatch admission from implicit booleans into explicit `allowed + reason` semantics.
- Keep existing lifecycle/dispatch/attempt/start/download contracts stable while clarifying compatibility auto-dispatch behavior on `/start`.
- Add lightweight admission summaries to export-job read models and dispatch/attempt list records for troubleshooting.

## Files Changed
- `docs/phases/PHASE_AUTO_048.md`
- `domain/export_center.go`
- `domain/export_job_dispatch.go`
- `domain/export_job_attempt.go`
- `repo/mysql/export_job_dispatch.go`
- `repo/mysql/export_job_attempt.go`
- `service/export_center_service.go`
- `service/export_center_service_test.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/iterations/ITERATION_048.md`

## DB / Migration Changes
- None
- Step 48 reuses existing Step 17/18 tables:
  - `export_job_attempts`
  - `export_job_dispatches`
- No schema migration required; all admission hardening fields are derived read-model fields.

## API Changes
- OpenAPI version advanced from `0.43.0` to `0.44.0`.
- No new endpoints.
- `ExportJob` now additively exposes:
  - `can_start_reason`
  - `can_attempt`
  - `can_attempt_reason`
  - `can_dispatch_reason`
  - `can_redispatch_reason`
  - `dispatchability_reason`
  - `attemptability_reason`
  - `latest_admission_decision`
- `ExportJobDispatch` now additively exposes:
  - `start_admissible`
  - `start_admission_reason`
- `ExportJobAttempt` now additively exposes:
  - `blocks_new_attempt`
  - `next_attempt_admission_reason`
- Export route descriptions now explicitly document:
  - admission hardening is skeleton-only
  - `/start` compatibility auto-placeholder dispatch policy
  - internal/admin placeholder boundary positioning

## Design Decisions
- Kept admission hardening on existing layered model:
  - lifecycle = business-visible export object state
  - dispatch = placeholder adapter handoff state
  - attempt = one concrete placeholder execution try
- Implemented admission as deterministic read-model derivation:
  - no new persistence table
  - no extra event stream
  - no scheduler platform coupling
- Preserved backward-compatible `/start` behavior:
  - consume latest `received` dispatch when available
  - otherwise allow compatibility auto-placeholder dispatch creation for start admission paths that are explicitly marked as compatible
- Aligned invalid-state error details with read-model admission reasons so troubleshooting paths stay consistent between command and query responses.

## Correction Notes
- Corrected stale repository-handover appendix state:
  - `docs/V7_MODEL_HANDOVER_APPENDIX.md` previously still reported Step 45 / `v0.41.0`
  - now reconciled to Step 48 / `v0.44.0` and updated with Step 46-48 sections
- This correction was applied before finalizing Step 48 state outputs to keep CURRENT_STATE / OpenAPI / handover docs consistent.

## Risks / Known Gaps
- Admission reasons are current skeleton-level reason codes and may need extension once real scheduler/runner policy dimensions (priority/quota/tenant) are introduced.
- Admission rejection still does not append a dedicated rejected-action event stream; rejection context currently lives in API error details plus current read-model reason fields.
- Export infrastructure remains placeholder-only:
  - no real async runner/scheduler
  - no real file generation
  - no storage/NAS integration
  - no signed URL / real download delivery

## Suggested Next Step
- Stop automatic continuation here.
- If next round remains boundary-safe, Step 49 should target auth / org / visibility policy scaffolding without entering real identity provider integration.
