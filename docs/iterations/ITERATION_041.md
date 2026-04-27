# ITERATION_041 - Cost-Governance Audit / History Read-Model Hardening

**Date**: 2026-03-10  
**Scope**: `docs/phases/PHASE_AUTO_041.md`

## 1. Goals
- Execute exactly one new phase after Step 40:
  - cost-governance audit / history read-model hardening
- Turn existing governed cost-rule rows and task-side snapshots into stable read models without changing Step 40 write semantics.
- Avoid formula DSL, real approval flow, finance integration, ERP cost writeback, BI/report deepening, and deep auth expansion.

## 2. Inputs
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_040.md`
- `docs/phases/PHASE_AUTO_041.md`
- latest PRD / V7 implementation spec
- `AGENT_PROTOCOL.md`
- `AUTO_PHASE_PROTOCOL.md`

## 3. Files Changed
- `domain/cost_rule.go`
- `domain/query_views.go`
- `domain/task_detail_aggregate.go`
- `domain/procurement.go`
- `service/cost_governance_read_model.go`
- `service/cost_rule_service.go`
- `service/task_service.go`
- `service/task_detail_service.go`
- `service/procurement_summary.go`
- `service/cost_rule_service_test.go`
- `service/task_prd_service_test.go`
- `transport/handler/cost_rule.go`
- `transport/http.go`
- `cmd/server/main.go`
- `docs/phases/PHASE_AUTO_041.md`
- `docs/iterations/ITERATION_041.md`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## 4. DB / Migration Changes
- No new migration in this phase.
- Reused the existing Step 40 data model:
  - `cost_rules`
  - `task_details`
  - `task_event_logs`
- Kept write-side history policy unchanged:
  - new rule changes affect future preview / future prefill only
  - old task rows are not auto-recomputed

## 5. API Changes
- OpenAPI version advanced from `0.36.0` to `0.37.0`.
- Added one read-only endpoint:
  - `GET /v1/cost-rules/{id}/history`
- `CostRule` responses now additively expose:
  - `version_chain_summary`
  - `previous_version`
  - `next_version`
  - `supersession_depth`
- `GET /v1/tasks/{id}`, `GET /v1/tasks/{id}/detail`, and purchase-task `procurement_summary` now additively expose:
  - `matched_rule_governance`
  - `override_summary`

## 6. Design Decisions
- Kept rule history read-side and derived:
  - lineage is rebuilt from `supersedes_rule_id` plus derived `superseded_by_rule_id`
  - no separate `cost_rule_versions` table was introduced
- Made task-side governance explicitly layered:
  - historical matched rule snapshot explains what the task last hit/persisted
  - current rule explains what the latest reachable governed rule is now
  - override summary explains lightweight business adjustments derived from task events
- Kept override history at summary grade in this phase:
  - current snapshot remains on `task_details`
  - event-derived summary comes from existing `task_event_logs`
  - no dedicated approval/audit subsystem was introduced

## 7. Correction Notes
- Corrected code/doc drift where Step 40 documentation still described governed cost-rule rows mostly as isolated records and did not expose the actual Step 41 lineage and task-side governance read models.
- Corrected API/doc drift by adding the new read-only `GET /v1/cost-rules/{id}/history` contract to OpenAPI and state documents.

## 8. Verification
- `gofmt -w domain/cost_rule.go domain/query_views.go domain/task_detail_aggregate.go domain/procurement.go service/cost_governance_read_model.go service/cost_rule_service.go service/task_detail_service.go service/procurement_summary.go service/task_service.go service/cost_rule_service_test.go service/task_prd_service_test.go transport/handler/cost_rule.go transport/http.go cmd/server/main.go`
- `go test ./service -run 'CostRule|TaskServiceGetByIDReturnsProcurementSummaryCostSignals'`
- `go test ./...`

## 9. Risks / Known Gaps
- Override history is still summary-grade and derived from business-info task events rather than a dedicated override-history table.
- Rule lineage depends on continuity of `supersedes_rule_id`; broken links will reduce lineage visibility.
- This phase is still not:
  - a formula DSL
  - a real approval workflow
  - a real finance integration
  - a real ERP cost writeback layer

## 10. Suggested Next Step
1. Keep the new governance read models stable before attempting any approval or finance-adjacent expansion.
2. If governance depth needs to grow later, prefer an explicit audit/approval subsystem over adding more ambiguous snapshot fields to current write models.
