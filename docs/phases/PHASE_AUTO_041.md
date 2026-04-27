# PHASE_AUTO_041 - Cost-Governance Audit / History Read-Model Hardening

## Why This Phase Now
- Step 40 made cost-rule governance writable and snapshot-friendly, but it still left read-side auditability too weak for operators and frontend consumers.
- The repository already has enough source facts to explain rule lineage, task-side historical hits, and lightweight override trace without introducing a new approval system or finance platform.
- The highest-value next move is to turn existing governed rows and task snapshots into stable read models before expanding into any deeper rule engine or workflow domain.

## Current Context
- Current state before this phase: `V7 Migration Step 40 complete`
- Current OpenAPI version before this phase: `0.36.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_040.md`
- Stable cost-governance facts already exist:
  - `rule_version`
  - `supersedes_rule_id` / `superseded_by_rule_id`
  - `governance_note` / `governance_status`
  - preview hit trace
  - task-side prefill snapshot trace
  - lightweight manual override trace
- Current main gap:
  - no strong unified read model for rule history, task-side historical hit vs current rule state, and override audit summary

## Goals
- Add additive cost-rule lineage read models that expose:
  - `version_chain_summary`
  - `previous_version`
  - `next_version`
  - `supersession_depth`
- Add one read-only cost-rule history interface:
  - `GET /v1/cost-rules/{id}/history`
- Add task-side governance read models that clearly separate:
  - the historical matched rule snapshot
  - the current latest rule in that lineage
  - lightweight override history summary
- Keep Step 40 write semantics unchanged:
  - future preview / future prefill are affected by new rules
  - old tasks are not auto-recomputed

## Allowed Scope
- additive cost-rule lineage read models on existing rule responses
- one additive read-only cost-rule history endpoint
- additive task read/detail/procurement-summary governance projections
- lightweight override-history summary derived from existing task events
- additive tests for lineage and task-side governance read models
- additive OpenAPI / state / iteration / handover synchronization
- create `docs/phases/PHASE_AUTO_041.md`

## Forbidden Scope
- complete formula DSL or expression authoring platform
- real approval workflow or approval routing engine
- real finance system or ERP cost writeback
- BI / KPI / export-report deepening
- deep auth / RBAC / org visibility work
- NAS / object storage / unrelated upload or export expansion
- automatic backfill or recomputation of historical task cost rows

## Expected File Changes
- update cost-rule domain/service/handler/router code for lineage and history reads
- add shared cost-governance read-model builder logic
- update task read/detail/procurement-summary shaping with additive governance objects
- update focused tests
- add `docs/phases/PHASE_AUTO_041.md`
- add `docs/iterations/ITERATION_041.md`
- update `docs/api/openapi.yaml`
- update `CURRENT_STATE.md`
- update `MODEL_HANDOVER.md`
- update `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Required API / DB Changes
- DB:
  - no new migration in this phase
  - reuse existing governed `cost_rules`, `task_details`, and `task_event_logs`
- API:
  - add `GET /v1/cost-rules/{id}/history`
  - extend `CostRule` additively with:
    - `version_chain_summary`
    - `previous_version`
    - `next_version`
    - `supersession_depth`
  - extend task read/detail/procurement-summary additively with:
    - `matched_rule_governance`
    - `override_summary`

## Success Criteria
- one cost rule can now be read as part of a version lineage instead of only as an isolated governed row
- task read models now distinguish historical matched rule snapshot from current latest rule state
- override history is now readable as a lightweight summary derived from existing task events
- docs and OpenAPI explicitly state this phase is read-model hardening, not approval flow or finance integration
- tests pass without changing Step 40 write behavior

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_041.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Risks
- current override history remains summary-grade because the write model still stores only the latest override snapshot plus task events
- task-side governance read models depend on lineage continuity through existing `supersedes_rule_id` links
- additive governance fields must still not be misread as real approval, auth, or finance workflow semantics

## Completion Output Format
1. changed files
2. DB / migration changes
3. API changes
4. correction notes
5. risks / remaining gaps
6. next recommended single phase
