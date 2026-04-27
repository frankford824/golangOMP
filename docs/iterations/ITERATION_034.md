# ITERATION_034 - Export Dispatch / Attempt Read-Model Integration

**Date**: 2026-03-10  
**Scope**: PHASE_AUTO_034

## 1. Goals
- Surface dispatch summaries directly on export-job list/detail.
- Tighten read-model affordances around dispatch-aware start semantics.

## 2. Files Changed
- `domain/export_center.go`
- `service/export_center_service.go`
- `service/export_center_service_test.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## 3. API / OpenAPI Changes
- Export-job list/detail now expose:
  - `dispatch_count`
  - `latest_dispatch`
  - `can_dispatch`
  - `can_redispatch`
  - `latest_dispatch_event`
- `can_start` is now dispatch-aware.
- OpenAPI advanced to `v0.31.0` in this iteration.

## 4. Auto-Correction Notes
- Previous export-job read models could imply `queued => can_start`; this iteration corrected that drift by respecting blocking submitted dispatch state.

## 5. Verification
- `gofmt -w` on touched Go files.
- `go test ./...`

## 6. Risks / Known Gaps
- Dispatch summaries are still placeholder-only and not a real scheduler surface.
- Internal dispatch routes remain the authoritative inspection path for full dispatch history.

## 7. Next Step
- If export-center skeleton stays stable, add one narrow integration-center / API call log skeleton.
