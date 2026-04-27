# PHASE_AUTO_043

## Why This Phase Now
- Step 42 established the dedicated override-governance audit stream, but the repo still could not express whether one override needs later review, what its current placeholder review state is, whether it may enter a finance-facing layer, or what its placeholder finance state is.
- The latest PRD-aligned gap is therefore not a real approval flow or finance system, but a stable placeholder boundary above `cost_override_events`.

## Current Context
- Current CURRENT_STATE: Step 42 complete before this phase
- Current OpenAPI version: `0.38.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_042.md`
- Current completed mainline:
  - governed cost-rule versioning / history
  - task-side matched-rule snapshot + override summary
  - dedicated `cost_override_events` audit stream
  - read-only `GET /v1/tasks/{id}/cost-overrides`
- Current main gap:
  - no approval placeholder boundary
  - no finance placeholder boundary
  - no way to express whether one override is review-required / finance-required

## Goals
- Add approval placeholder and finance placeholder persistence above `cost_override_events`
- Expose additive `override_governance_boundary` read models on task read/detail, procurement summary, and override timeline
- Add minimal internal placeholder actions for review / finance marking without expanding into real approval or finance systems

## Allowed Scope
- Domain models for override placeholder boundary
- Repo interfaces and MySQL repos for placeholder tables
- Task read/detail/procurement/override-timeline read-model wiring
- Minimal HTTP skeleton routes:
  - `POST /v1/tasks/{id}/cost-overrides/{event_id}/review`
  - `POST /v1/tasks/{id}/cost-overrides/{event_id}/finance-mark`
- OpenAPI / CURRENT_STATE / iteration / handover sync
- One new migration for placeholder tables

## Forbidden Scope
- Real approval workflow or real reviewer assignment/state machine
- Real finance / accounting / reconciliation / settlement / invoice capabilities
- ERP cost writeback
- Real identity / permission approval chain
- Formula DSL expansion, BI/report deepening, or unrelated modules

## Expected File Changes
- Add `db/migrations/024_v7_cost_override_placeholder_boundaries.sql`
- Add domain / repo / service / handler wiring for placeholder boundary
- Update `docs/api/openapi.yaml`
- Update `CURRENT_STATE.md`, `MODEL_HANDOVER.md`, `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- Add `docs/iterations/ITERATION_043.md`

## Required API / DB Changes
- New tables:
  - `cost_override_reviews`
  - `cost_override_finance_flags`
- Additive read-model field:
  - `override_governance_boundary`
- New internal placeholder APIs:
  - `POST /v1/tasks/{id}/cost-overrides/{event_id}/review`
  - `POST /v1/tasks/{id}/cost-overrides/{event_id}/finance-mark`

## Success Criteria
- Go code compiles and focused tests pass
- System can express review-required / review-status / finance-required / finance-status / finance-view-ready for one override
- Existing rule history, matched snapshot, override summary, and override audit contracts remain additive and non-regressive
- Docs and OpenAPI are synchronized to Step 43

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_043.md`
- `docs/api/openapi.yaml`

## Risks
- Older override audit rows still have no persisted placeholder rows until explicitly marked; read models therefore must keep sensible fallback semantics
- Placeholder status names may later need refinement when a real approval/finance subsystem is connected

## Completion Output Format
1. Changed files
2. DB / migration changes
3. API changes
4. Correction notes
5. Risks / known gaps
6. Suggested next step
