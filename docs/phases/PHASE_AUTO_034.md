# PHASE_AUTO_034 - Export Dispatch / Attempt Read-Model Integration

## Why This Phase Now
- Step 33 added dispatch persistence and internal dispatch APIs.
- The next gap was read-model visibility: list/detail still exposed lifecycle plus attempt summaries, but not dispatch summaries.
- This phase hardens export job read models so frontend/admin troubleshooting can distinguish lifecycle, dispatch, and attempt state without reading internal-only routes first.

## Goals
- Add dispatch-side summaries to export-job list/detail.
- Tighten `can_start` to reflect blocking submitted dispatch state.
- Keep list/detail contracts stable while making the new dispatch layer visible.

## Required Changes
- Export-job read models now expose:
  - `dispatch_count`
  - `latest_dispatch`
  - `can_dispatch`
  - `can_redispatch`
  - `latest_dispatch_event`
- Hydrate dispatch summaries and dispatch-event summaries together with existing attempt/event summaries.
- Sync OpenAPI, state, readiness, and handover docs.

## Success Criteria
- `GET /v1/export-jobs` and `GET /v1/export-jobs/{id}` show dispatch summaries.
- `can_start=false` when the latest dispatch is still `submitted`.
- Dispatch visibility stays explicitly placeholder-only.
