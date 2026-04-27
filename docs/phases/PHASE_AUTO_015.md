# PHASE_AUTO_015 - Converged Task Filtering Read-Model Hardening

## Why This Phase Now
- Step 14 converged task-list and task-board filters at the public contract layer, but several converged filters still depend on service-layer fan-out and post-hydration matching.
- The latest PRD direction treats task list and task board as two views over one stable task pool, so the shared filter contract should sit on top of more direct read-model predicates instead of duplicate service-side derivation.
- Ownership, inbox persistence, or saved preferences would be premature while converged filter execution still carries drift and complexity risk.

## Current Context
- `CURRENT_STATE.md` before this round reports Step 14 complete and OpenAPI `v0.15.0`.
- `GET /v1/tasks`, `GET /v1/task-board/summary`, and `GET /v1/task-board/queues` already expose the converged board/list filter contract.
- Main gaps before implementation:
  - some converged filters still require service-layer fan-out and derived matching
  - board/list filtering shares field names but not enough direct read-model predicate reuse
  - future ownership or preference features would otherwise build on unstable filter execution

## Goals
- Push the converged task filter contract further down into repo/read-model predicates.
- Reduce or remove service-layer fan-out for `GET /v1/tasks`, especially around projected workflow, procurement coordination, and warehouse readiness-related filters.
- Keep existing `normalized_filters` and `query_template` contracts stable.
- Preserve existing frontend-consumed filter field names and baseline semantics.

## Allowed Scope
- `domain/`
- `repo/`
- `service/`
- `transport/`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/`
- `docs/phases/`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Forbidden Scope
- Real authentication or permission enforcement
- Queue ownership persistence or personal inbox storage
- Saved workbench preferences
- Real ERP integration
- NAS / real upload integration
- Strict `whole_hash` verification
- Full BI / KPI / finance / export-center modules

## Expected File Changes
- Harden task repo filter structs and SQL predicates so converged filters can execute closer to the read model.
- Simplify task service list filtering to rely on normalized direct repo predicates instead of segmented fan-out where possible.
- Add or update tests that prove converged filters can execute through one repo query path while keeping board/list semantics aligned.
- Sync OpenAPI, current state, iteration memory, handover, and V7 spec appendix/doc text to Step 15.

## Required API / DB Changes
- API:
  - no external filter-field rename
  - no external semantic rollback for `/v1/tasks`, `/v1/task-board/summary`, or `/v1/task-board/queues`
  - if external contract stays stable, OpenAPI only needs a patch-version update plus implementation notes
- DB / migration:
  - no new table
  - no migration required in this round

## Success Criteria
- `/v1/tasks` executes converged workflow/procurement/warehouse filters through more direct repo/read-model predicates.
- Service-layer segmented fan-out is reduced or removed for the stabilized converged filters in scope.
- Task-board queue drill-down contracts keep existing `normalized_filters` and `query_template` behavior.
- Remaining fan-out points, if any, are explicitly documented in the iteration record.
- `go test ./...` passes.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_015.md`
- `docs/api/openapi.yaml`

Optional but expected for this round:
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- SQL predicate derivation can drift from service-side workflow/procurement semantics if not kept tightly aligned.
- Some queue aggregation work still requires preset-level iteration even after list filter fan-out is reduced.
- Predicate pushdown improves correctness and complexity first, not full indexing or large-scale query optimization.

## Completion Output Format
1. Changed Files
2. DB / Migration Changes
3. API / OpenAPI Changes
4. Auto-Correction Notes
5. Verification
6. Risks / Remaining Gaps
7. Next Step
