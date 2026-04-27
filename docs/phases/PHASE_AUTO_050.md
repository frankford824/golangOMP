# PHASE_AUTO_050

## Why This Phase Now
- Step 49 completed cross-center auth/org/visibility policy scaffolding, and the next highest-impact PRD gap is still the missing unified KPI/finance/report platform entry boundary.
- Task/procurement/cost governance/export already have stable read-model contracts, so Step 50 can add one shared entry language without reopening lifecycle writes.
- This is the smallest safe move that enables future KPI/finance/report docking while keeping real BI/finance/report engines deferred.

## Current Context
- Current CURRENT_STATE before this phase: Step 49 complete
- Current OpenAPI version before this phase: `0.45.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_049.md`
- Current main remaining safe gap: KPI / finance / report platform entry boundary

## Goals
- Introduce one reusable cross-center platform-entry boundary language:
  - `platform_entry_boundary`
  - `kpi_entry_summary`
  - `finance_entry_summary`
  - `report_entry_summary`
- Expose additive platform-entry summaries on key read models:
  - task list/read/detail
  - procurement summary
  - cost override governance boundary
  - export job
- Keep all current boundaries explicit:
  - placeholder entry scaffolding only
  - not a real KPI engine
  - not a real finance/accounting/reconciliation/settlement/invoice system
  - not a real report engine / data warehouse / analytics engine

## Allowed Scope
- Additive domain models and hydration helpers for unified KPI/finance/report entry boundaries
- Additive read-model fields in existing task/procurement/cost/export contracts
- Focused service/test updates for new boundary hydration coverage
- OpenAPI/state/iteration/handover synchronization

## Forbidden Scope
- Real KPI computation or BI analytics workflows
- Real finance posting/ledger/reconciliation/settlement/invoice workflows
- Real report generation platform or report scheduler implementation
- Real data warehouse or analytics engine integration
- Real ERP integration expansion
- Runtime permission redesign beyond existing Step 49 policy scaffolding

## Expected File Changes
- Add one domain file for shared platform-entry boundary language
- Update task/procurement/cost/export read-model structs and hydration wiring
- Update focused service tests
- Add `docs/phases/PHASE_AUTO_050.md`
- Add `docs/iterations/ITERATION_050.md`
- Update `docs/api/openapi.yaml`
- Update `CURRENT_STATE.md`
- Update `MODEL_HANDOVER.md`
- Update `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Required API / DB Changes
- DB:
  - no new tables or migrations
- API:
  - no new endpoints
  - additive `platform_entry_boundary` summaries on existing read-model schemas
  - OpenAPI clarifies this is entry scaffolding only, not real BI/finance/report platforms

## Success Criteria
- Task/procurement/cost-governance/export read models expose one shared platform-entry language.
- KPI/finance/report future docking intent is machine-readable on these read models.
- Existing task/procurement/cost/export lifecycle semantics remain unchanged.
- OpenAPI + CURRENT_STATE + iteration/handover docs are fully synchronized to Step 50 and `v0.46.0`.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_050.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Risks
- Entry summaries are still static scaffold decisions and may need remapping when real KPI/finance/report platform contracts are finalized.
- `eligible_now` and source-field hints are read-model guidance only; they are not execution guarantees for downstream systems.

## Completion Output Format
1. Changed files
2. DB / migration changes
3. API changes
4. Correction notes
5. Risks / known gaps
6. Suggested next step
