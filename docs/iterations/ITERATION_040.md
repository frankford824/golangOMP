# ITERATION_040 - Cost-Rule Governance / Versioning / Override Hardening

**Date**: 2026-03-10  
**Scope**: `docs/phases/PHASE_AUTO_040.md`

## 1. Goals
- Execute exactly one new phase after Step 39:
  - cost-rule governance / versioning / override hardening
- Keep existing category center, cost preview, task-side prefill, and procurement-summary contracts non-regressive while making the current cost-rule skeleton governable.
- Avoid full formula DSL, real finance integration, real ERP cost writeback, real approval flow, BI/KPI/report deepening, and deep auth expansion.

## 2. Inputs
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_039.md`
- `docs/phases/PHASE_AUTO_040.md`
- latest PRD / V7 implementation spec
- `AGENT_PROTOCOL.md`
- `AUTO_PHASE_PROTOCOL.md`

## 3. Files Changed
- `db/migrations/022_v7_cost_rule_governance_hardening.sql`
- `domain/cost_rule.go`
- `domain/task.go`
- `domain/procurement.go`
- `domain/query_views.go`
- `repo/mysql/db.go`
- `repo/mysql/cost_rule.go`
- `repo/mysql/task.go`
- `service/cost_rule_service.go`
- `service/cost_prefill.go`
- `service/task_service.go`
- `service/procurement_summary.go`
- `service/cost_rule_service_test.go`
- `service/task_prd_service_test.go`
- `transport/handler/cost_rule.go`
- `docs/phases/PHASE_AUTO_040.md`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_040.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## 4. DB / Migration Changes
- Added `db/migrations/022_v7_cost_rule_governance_hardening.sql`.
- Hardened `cost_rules` additively with:
  - `rule_version`
  - `supersedes_rule_id`
  - `governance_note`
- Hardened `task_details` additively with:
  - `matched_rule_version`
  - `prefill_source`
  - `prefill_at`
  - `override_actor`
  - `override_at`
- Kept the existing `cost_rules` table as the governing row model instead of introducing a separate `cost_rule_versions` subsystem in this phase.

## 5. API Changes
- No new endpoints were added.
- OpenAPI version advanced from `0.35.0` to `0.36.0`.
- Cost-rule list/detail schema now additively expose:
  - `rule_version`
  - `supersedes_rule_id`
  - `superseded_by_rule_id`
  - `governance_note`
  - `governance_status`
- Cost preview schema now additively exposes:
  - `matched_rule_id`
  - `matched_rule_version`
  - `governance_status`
  - version/source/status on `matched_rule` and `applied_rules`
- Task-detail / procurement-summary schema now additively expose:
  - `cost_rule_id` on procurement summary
  - `matched_rule_version`
  - `prefill_source`
  - `prefill_at`
  - `override_actor`
  - `override_at`

## 6. Design Decisions
- Kept governance hardening inside the existing `cost_rules` table because the current gap was traceability/governance, not missing rule-execution infrastructure.
- Used `rule_version` plus `supersedes_rule_id` as the minimum viable lineage model.
- Exposed `superseded_by_rule_id` as a derived read-model field rather than maintaining a second write path.
- Kept `governance_status` as a derived effective-window status:
  - `inactive`
  - `scheduled`
  - `effective`
  - `expired`
- Kept preview and task-side persistence separate:
  - preview explains the current governed match
  - task-side prefill stores the last persisted snapshot
- Kept manual override governance lightweight:
  - `manual_cost_override`
  - `manual_cost_override_reason`
  - `override_actor`
  - `override_at`
  - no approval flow, no auth dependency

## 7. History Policy
- New rule changes do not auto-recompute old task rows.
- Historical task-side cost results remain the last persisted snapshot.
- New rule changes affect:
  - future `POST /v1/cost-rules/preview`
  - future `PATCH /v1/tasks/{id}/business-info` prefill executions
- This iteration documents that policy explicitly in code-facing docs and OpenAPI.

## 8. Correction Notes
- Corrected `CURRENT_STATE.md` migration inventory, which had drifted behind the repository by omitting migrations `015` through `021`.
- Corrected code/doc contract drift where cost-rule and task-side cost docs still described the older pre-governance skeleton even though Step 40 required governed version/trace fields.

## 9. Verification
- `gofmt -w domain/cost_rule.go domain/task.go domain/procurement.go domain/query_views.go repo/mysql/cost_rule.go repo/mysql/task.go repo/mysql/db.go service/cost_rule_service.go service/cost_prefill.go service/task_service.go service/procurement_summary.go service/cost_rule_service_test.go service/task_prd_service_test.go transport/handler/cost_rule.go`
- `go test ./service -run CostRule`
- `go test ./service -run 'TaskServiceUpdateBusinessInfo|TaskServiceGetByIDReturnsProcurementSummaryCostSignals'`
- `go test ./...`

## 10. Risks / Known Gaps
- No full formula DSL exists yet.
- No approval workflow exists yet.
- No real finance system or ERP cost writeback exists yet.
- `override_actor` is lightweight placeholder trace only, not real identity governance.
- Rule lineage is intentionally minimal:
  - one row-level version number
  - one predecessor pointer
  - no separate version-history table

## 11. Suggested Next Step
1. Step 41: cost-governance audit/history read-model hardening
2. Keep approval flow, finance integration, ERP writeback, and formula DSL deferred until the now-governed skeleton semantics stay stable
