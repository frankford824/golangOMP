# ITERATION_033 - Export Adapter-Dispatch Handoff Skeleton

**Date**: 2026-03-10  
**Scope**: PHASE_AUTO_033

## 1. Goals
- Add a placeholder dispatch layer between queued export jobs and execution attempts.
- Make adapter handoff state explicit without adding a real scheduler.

## 2. Files Changed
- `db/migrations/018_v7_export_job_dispatches.sql`
- `domain/export_job_dispatch.go`
- `domain/export_job_attempt.go`
- `domain/export_job_event.go`
- `repo/interfaces.go`
- `repo/mysql/export_job_attempt.go`
- `repo/mysql/export_job_dispatch.go`
- `service/export_center_service.go`
- `service/export_center_service_test.go`
- `transport/handler/export_center.go`
- `transport/http.go`
- `cmd/server/main.go`

## 3. DB / Migration Changes
- Added `export_job_dispatches` through DB migration `018`.
- Added nullable `dispatch_id` linkage on `export_job_attempts`.

## 4. API Changes
- Added internal/admin dispatch routes:
  - `GET /v1/export-jobs/{id}/dispatches`
  - `POST /v1/export-jobs/{id}/dispatches`
  - `POST /v1/export-jobs/{id}/dispatches/{dispatch_id}/advance`
- Start now consumes a received dispatch or auto-creates a placeholder submitted/received dispatch for backward compatibility.

## 5. Correction Notes
- Repository truth after Step 32 still described dispatch as future work only; this iteration makes dispatch persistence and events real repository truth.

## 6. Verification
- `gofmt -w` on touched Go files.
- `go test ./...`

## 7. Risks / Known Gaps
- Dispatch is still placeholder-only and not a real scheduler queue.
- No callback, heartbeat, lease, or real worker ownership exists.

## 8. Next Step
- Add dispatch summaries into export-job read models.
