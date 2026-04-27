# PHASE_AUTO_042 - Override / Governance Audit Stream Hardening

## Why This Phase Now
- Step 41 made override history readable only at summary grade, but the repository still lacked a dedicated audit layer for task-side cost override governance.
- The biggest remaining governance gap is no longer rule lineage itself; it is the inability to express override apply/update/release as first-class audit events with before/after values and rule-version context.
- The next highest-value move is therefore to add a dedicated override audit skeleton while keeping existing cost-rule preview/prefill/history contracts stable and explicitly avoiding approval-flow or finance-system expansion.

## Current Context
- Current state before this phase: `V7 Migration Step 41 complete`
- Current OpenAPI version before this phase: `0.37.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_041.md`
- Stable governed cost facts already exist:
  - rule lineage on `cost_rules`
  - preview hit trace
  - task-side prefill snapshot trace
  - lightweight override snapshot fields on `task_details`
  - task/detail/procurement-summary governance read models
- Current main gap:
  - override governance still relies on `task_event_logs`-derived summary instead of a dedicated governance audit stream

## Goals
- Add a dedicated override/governance audit persistence skeleton through:
  - `cost_override_events`
  - `cost_override_event_sequences`
- Append dedicated override audit events from `PATCH /v1/tasks/{id}/business-info` when override state changes while preserving `task_event_logs` as the general event stream.
- Expose additive governance audit read models through:
  - `governance_audit_summary`
  - read-only task timeline endpoint `GET /v1/tasks/{id}/cost-overrides`
- Keep existing rule history, preview, prefill, and task-side snapshot semantics stable:
  - future preview / future prefill only
  - no old-task auto-recompute

## Allowed Scope
- one dedicated override audit table family plus repo/service/handler wiring
- additive task/detail/procurement-summary governance audit read models
- one read-only task override audit timeline endpoint
- fallback compatibility from dedicated audit rows back to older `task_event_logs`-derived summary when no dedicated rows exist yet
- additive tests and document/OpenAPI synchronization
- create `docs/phases/PHASE_AUTO_042.md`

## Forbidden Scope
- real approval workflow or approval routing engine
- real finance system, ledger, or ERP cost writeback
- formula DSL or broader rule-engine expansion
- deep auth / RBAC / org-visibility expansion
- BI / KPI / export-report deepening
- NAS / object storage / unrelated infrastructure work
- historical backfill of old task events into the new override audit table

## Expected File Changes
- add dedicated override audit domain/repo/mysql migration code
- update task business-info write flow to append dedicated override audit events
- update task read/detail/procurement-summary governance shaping
- add read-only override audit timeline service + handler + route
- update focused tests
- add `docs/phases/PHASE_AUTO_042.md`
- add `docs/iterations/ITERATION_042.md`
- update `docs/api/openapi.yaml`
- update `CURRENT_STATE.md`
- update `MODEL_HANDOVER.md`
- update `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Required API / DB Changes
- DB:
  - add `db/migrations/023_v7_cost_override_audit_stream.sql`
  - create:
    - `cost_override_events`
    - `cost_override_event_sequences`
- API:
  - add `GET /v1/tasks/{id}/cost-overrides`
  - extend `GET /v1/tasks/{id}`, `GET /v1/tasks/{id}/detail`, and purchase-task `procurement_summary` additively with:
    - `governance_audit_summary`
  - extend `override_summary` additively with richer latest-audit-event fields while keeping the contract lightweight

## Success Criteria
- override apply/update/release is now traceable through a dedicated governance audit stream
- task/detail/procurement-summary reads expose both lightweight override summary and additive governance audit summary
- one task-scoped read-only timeline endpoint returns dedicated override audit events with before/after cost context and matched rule/version trace
- `task_event_logs` remains clearly documented as the general event layer and is not collapsed into the governance-specific audit layer
- docs and OpenAPI explicitly state this is an audit skeleton, not approval flow or finance integration
- tests pass without regressing Step 40 / Step 41 contracts

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_042.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Risks
- older tasks still rely on fallback summary from `task_event_logs` until new override audit rows are created by later writes
- the new audit stream is governance-only and intentionally does not answer approval ownership or finance posting semantics
- rule/version context on audit rows is snapshot-grade and depends on current write-side task business-info inputs being correct at the time of change

## Completion Output Format
1. changed files
2. DB / migration changes
3. API changes
4. correction notes
5. risks / remaining gaps
6. next recommended single phase
