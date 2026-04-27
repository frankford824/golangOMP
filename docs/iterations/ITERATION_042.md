# ITERATION_042 - Override / Governance Audit Stream Hardening

**Date**: 2026-03-10  
**Scope**: `docs/phases/PHASE_AUTO_042.md`

## 1. Goals
- Execute exactly one new phase after Step 41:
  - override / governance audit stream hardening
- Turn task-side manual cost override from a summary-only read model into a dedicated governance audit skeleton while preserving existing rule lineage, preview, prefill, and override snapshot contracts.
- Avoid real approval flow, finance integration, ERP cost writeback, formula DSL expansion, BI/report deepening, and deep auth work.

## 2. Inputs
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_041.md`
- `docs/phases/PHASE_AUTO_042.md`
- latest PRD / V7 implementation spec
- `AGENT_PROTOCOL.md`
- `AUTO_PHASE_PROTOCOL.md`

## 3. Files Changed
- `db/migrations/023_v7_cost_override_audit_stream.sql`
- `domain/cost_override_audit.go`
- `domain/query_views.go`
- `domain/procurement.go`
- `domain/task_detail_aggregate.go`
- `repo/interfaces.go`
- `repo/mysql/cost_override_event.go`
- `service/cost_governance_read_model.go`
- `service/procurement_summary.go`
- `service/task_cost_override_service.go`
- `service/task_detail_service.go`
- `service/task_prd_service_test.go`
- `service/task_service.go`
- `transport/handler/task_cost_override.go`
- `transport/http.go`
- `cmd/server/main.go`
- `docs/phases/PHASE_AUTO_042.md`
- `docs/iterations/ITERATION_042.md`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## 4. DB / Migration Changes
- Added `db/migrations/023_v7_cost_override_audit_stream.sql`.
- New tables:
  - `cost_override_events`
  - `cost_override_event_sequences`
- Dedicated override audit rows now persist:
  - event identity and per-task sequence
  - task/task-detail linkage
  - matched rule/version/source/governance snapshot
  - previous estimated cost / previous cost / override cost / result cost
  - lightweight actor / time / note / source

## 5. API Changes
- OpenAPI version advanced from `0.37.0` to `0.38.0`.
- Added one read-only endpoint:
  - `GET /v1/tasks/{id}/cost-overrides`
- `GET /v1/tasks/{id}`, `GET /v1/tasks/{id}/detail`, and purchase-task `procurement_summary` now additively expose:
  - `governance_audit_summary`
- `override_summary` now additively exposes richer `latest_audit_event` context while keeping the existing summary contract stable.

## 6. Design Decisions
- Kept override governance layered instead of collapsing everything into one stream:
  - `task_event_logs` remains the general task event stream
  - `cost_override_events` is the governance-specific override audit layer
  - `override_summary` remains the stable lightweight consumer summary
- Kept backward compatibility for older tasks:
  - if dedicated override audit rows exist, read models prefer them
  - otherwise override summary falls back to older `task_event_logs`-derived behavior
- Kept write-side rule history policy unchanged:
  - later rule changes affect future preview/prefill only
  - existing task snapshots are not auto-recomputed
  - dedicated override audit explains human cost changes, not retroactive rule recomputation

## 7. Correction Notes
- Corrected code/doc drift from Step 41 where override governance was still described only as a `task_event_logs`-derived summary even though the next required PRD-aligned move was a dedicated governance audit layer.
- Corrected API/doc drift by adding the explicit `GET /v1/tasks/{id}/cost-overrides` contract and documenting the coexistence boundary between `task_event_logs` and the new governance audit stream.

## 8. Verification
- `gofmt -w domain/cost_override_audit.go domain/query_views.go domain/procurement.go domain/task_detail_aggregate.go repo/interfaces.go repo/mysql/cost_override_event.go service/cost_governance_read_model.go service/task_service.go service/procurement_summary.go service/task_detail_service.go service/task_cost_override_service.go service/task_prd_service_test.go transport/handler/task_cost_override.go transport/http.go cmd/server/main.go`
- `go test ./service/...`
- `go test ./...`

## 9. Risks / Known Gaps
- Older tasks still have no dedicated override audit rows until a later write touches those tasks; read models therefore still rely on fallback summary logic for those rows.
- This phase is still not:
  - a real approval workflow
  - a finance ledger
  - an ERP cost writeback layer
  - a historical backfill job
- The new audit stream is intentionally task-scoped and governance-scoped only; it does not yet model approvers, approval decisions, or accounting states.

## 10. Suggested Next Step
1. Keep the new override governance audit stream stable before expanding into any approval-like workflow.
2. If governance depth grows later, attach approval or finance concerns above this audit layer instead of collapsing rule lineage, task snapshots, and override audit back into one stream.
