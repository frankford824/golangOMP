# PHASE_AUTO_044

## Why This Phase Now
- Step 43 established approval / finance placeholder write boundaries, but the read side still exposed only flat status fields and did not yet provide one stable frontend-ready governance-boundary summary.
- The latest PRD-aligned gap is therefore read-model consolidation, not more write APIs.

## Current Context
- Current CURRENT_STATE before this phase: Step 43 complete
- Current OpenAPI version before this phase: `0.39.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_043.md`
- Current completed mainline:
  - governed cost-rule versioning / history
  - task-side matched-rule snapshot + prefill trace
  - dedicated `cost_override_events` audit stream
  - approval / finance placeholder write boundary
- Current main gap:
  - no unified governance-boundary summary across task/detail/procurement/timeline
  - no stable latest-action summary for review / finance placeholder layers
  - future real approval / finance adapters still lack one consolidated read seam

## Goals
- Consolidate governance boundary reads into one stable `override_governance_boundary` contract
- Add unified summary structures:
  - `governance_boundary_summary`
  - `approval_placeholder_summary`
  - `finance_placeholder_summary`
- Add stable latest-action fields:
  - `latest_review_action`
  - `latest_finance_action`
  - `latest_boundary_actor`
  - `latest_boundary_at`
- Reuse the same boundary aggregation across task read/detail, purchase-task `procurement_summary`, and cost-override timeline reads

## Allowed Scope
- Domain read-model structures for governance-boundary consolidation
- Service-layer read-model builders and tests
- OpenAPI / CURRENT_STATE / iteration / handover sync
- One new phase-plan document and one new iteration document

## Forbidden Scope
- Real approval workflow
- Real finance / accounting / settlement / invoice system
- ERP cost writeback
- New placeholder write endpoints beyond the existing Step 43 surface
- Full formula DSL, BI/report deepening, auth / RBAC deep work, or unrelated modules

## Expected File Changes
- Update domain governance-boundary structs
- Update governance read-model builder logic
- Update focused service tests
- Update `docs/api/openapi.yaml`
- Update `CURRENT_STATE.md`, `MODEL_HANDOVER.md`, `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- Add `docs/iterations/ITERATION_044.md`

## Required API / DB Changes
- No new DB tables or migrations
- Additive read-model fields under `override_governance_boundary`:
  - `governance_boundary_summary`
  - `approval_placeholder_summary`
  - `finance_placeholder_summary`
  - `latest_review_action`
  - `latest_finance_action`
  - `latest_boundary_actor`
  - `latest_boundary_at`

## Success Criteria
- Go code compiles and focused tests pass
- Task/detail/procurement/timeline read models reuse the same governance-boundary object
- Boundary summaries clearly separate override audit from approval placeholder and finance placeholder semantics
- Docs and OpenAPI are synchronized to Step 44

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_044.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Risks
- Older tasks without persisted placeholder rows still rely on derived fallback summary semantics
- The current summary field set is intentionally placeholder-oriented and may need extension when a real approval or finance system is connected later

## Completion Output Format
1. Changed files
2. DB / migration changes
3. API changes
4. Correction notes
5. Risks / known gaps
6. Suggested next step
