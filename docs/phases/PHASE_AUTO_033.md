# PHASE_AUTO_033 - Export Adapter-Dispatch Handoff Skeleton

## Why This Phase Now
- Step 32 already split export-job lifecycle from execution-attempt lifecycle.
- The next blocking gap was the missing dispatch/adaptor handoff boundary between `queued` and one concrete attempt.
- This phase keeps real scheduler/runner/storage deferred while making dispatch semantics explicit enough for later replacement.

## Goals
- Add placeholder export dispatch persistence on top of export attempts.
- Make the system explicitly express:
  - dispatch submitted
  - dispatch received
  - dispatch rejected
  - dispatch expired
  - dispatch not executed
- Link attempts to dispatches without introducing a real scheduler queue.

## Allowed Scope
- `db/migrations/`
- `domain/`
- `repo/`
- `service/`
- `transport/`
- `cmd/server/main.go`
- `docs/api/openapi.yaml`
- state / handover / readiness / iteration docs

## Forbidden Scope
- Real async runner or scheduler
- Real file generation
- Real download bytes
- Signed URLs
- NAS / object storage
- Real ERP or auth integration

## Required Changes
- Add `export_job_dispatches` persistence and one nullable dispatch link on attempts.
- Add internal/admin dispatch APIs:
  - `GET /v1/export-jobs/{id}/dispatches`
  - `POST /v1/export-jobs/{id}/dispatches`
  - `POST /v1/export-jobs/{id}/dispatches/{dispatch_id}/advance`
- Extend start semantics so attempts consume a dispatch handoff instead of skipping directly from queued lifecycle to attempt visibility.
- Append dispatch events into the shared export-job timeline.

## Success Criteria
- Dispatch records are persisted independently of attempts.
- Attempts can point to the dispatch they consumed.
- Start rejects while the latest dispatch is still blocking in `submitted`.
- OpenAPI and docs describe dispatch as placeholder-only.
