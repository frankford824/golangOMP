# PHASE_AUTO_011 - Structured Sub-Status Query Enhancement / Procurement Flow Skeleton

## Why This Phase Now
- Step 10 stabilized the structured `workflow.sub_status` read contract, but `/v1/tasks` still cannot query by that structure.
- The latest PRD expects task list filtering to support mainline + sub-line visibility for operational boards and warehouse/procurement handoff.
- `purchase_task` procurement currently supports persistence only; it still lacks a minimal lifecycle action path and stable list-friendly summary.

## Current Context
- `CURRENT_STATE.md` before this round reports Step 10 complete and identifies structured sub-status query/filter support plus procurement-flow completion as the top gap.
- `docs/api/openapi.yaml` is currently `v0.11.0`.
- Latest completed iteration before this round: `docs/iterations/ITERATION_010.md`.
- Main blockers for this round:
  - no `main_status` / `sub_status_code` query contract on `GET /v1/tasks`
  - no stable procurement summary on task list items
  - procurement only has record maintenance, not a minimal status-action flow

## Goals
- Add `GET /v1/tasks` filtering by projected `workflow.main_status` and structured `sub_status_code`, with optional scope targeting.
- Keep `workflow.sub_status` fully consistent across list, single-task read, and aggregate detail.
- Evolve procurement from a readiness-only record into a minimal business-flow skeleton with stable status transitions and summary exposure.

## Allowed Scope
- `domain/`
- `repo/`
- `service/`
- `transport/`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/iterations/`
- `docs/phases/`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Forbidden Scope
- Real authentication or RBAC enforcement
- Real ERP integration
- NAS / real upload integration
- Whole-hash strict verification
- Full procurement module explosion
- KPI / finance / export-center module implementation

## Expected File Changes
- Extend task list filter models, handlers, and repo SQL for projected workflow filters.
- Add procurement summary read fields for task list / read model / aggregate detail.
- Add minimal procurement lifecycle status/action support.
- Add/update tests for projected workflow filtering and procurement lifecycle behavior.
- Sync PRD-facing docs, iteration memory, and OpenAPI.

## Required API / DB Changes
- API:
  - extend `GET /v1/tasks` query params with `main_status`, `sub_status_code`, and optional `sub_status_scope`
  - expose `procurement_summary` on `/v1/tasks`, `/v1/tasks/{id}`, and `/v1/tasks/{id}/detail`
  - add one minimal procurement lifecycle action endpoint beyond `PATCH /v1/tasks/{id}/procurement`
- DB / migration:
  - no new table required if the current `procurement_records` columns can support the minimal flow
  - if additional additive procurement fields are needed for minimal usability, keep them strictly additive

## Success Criteria
- `/v1/tasks` supports projected workflow filtering for `main_status` and structured `sub_status_code`.
- `workflow.sub_status` remains stable and aligned across list, single-task read, and detail aggregate.
- `purchase_task` has a minimal procurement lifecycle skeleton beyond raw field patching.
- `procurement_summary` is returned in a stable frontend-friendly shape.
- `go test ./...` passes.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_011.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md` (appendix / implementation sync if needed)

## Risks
- Projected workflow filtering uses derived logic rather than a persisted workflow table, so SQL/query logic must stay tightly aligned with service-layer derivation.
- Procurement status evolution may require frontend clients to stop assuming only `preparing|ready|completed`.
- List filtering on derived workflow fields may remain less index-friendly than raw column filters.

## Completion Output Format
1. Changed Files
2. DB / Migration Changes
3. API / OpenAPI Changes
4. Auto-Correction Notes
5. Verification
6. Risks / Remaining Gaps
7. Next Step
