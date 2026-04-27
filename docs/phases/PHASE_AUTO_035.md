# PHASE_AUTO_035 - Integration Center / API Call Log Skeleton

## Why This Phase Now
- Export-center dispatch/attempt skeleton is now sufficiently explicit for this three-phase batch.
- The next bounded platform step is not real ERP execution, but one placeholder integration-center seam for external-call visibility.
- This phase stays on the skeleton side of the boundary: static connectors plus call-log persistence, without real retries, callbacks, or worker orchestration.

## Goals
- Add a narrow integration-center / API call log persistence layer.
- Provide static connector metadata and internal/admin list/detail/create/advance APIs.
- Keep all semantics placeholder-only.

## Required Changes
- Add `integration_call_logs` persistence.
- Add internal/admin APIs:
  - `GET /v1/integration/connectors`
  - `POST /v1/integration/call-logs`
  - `GET /v1/integration/call-logs`
  - `GET /v1/integration/call-logs/{id}`
  - `POST /v1/integration/call-logs/{id}/advance`
- Expose placeholder call-log lifecycle/read-model fields such as `progress_hint`, `latest_status_at`, and `can_replay`.

## Success Criteria
- Integration-center routes work end-to-end with persistence and tests.
- OpenAPI and docs keep these routes internal placeholder only.
- No real ERP/external execution, retry engine, or callback processing is introduced.
