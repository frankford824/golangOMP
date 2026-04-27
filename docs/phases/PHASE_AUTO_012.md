# PHASE_AUTO_012 - Procurement Coordination Deepening / Arrival-to-Warehouse Handoff

## Why This Phase Now
- Step 11 introduced a minimal procurement lifecycle, but purchase-task warehouse handoff still treats multiple procurement states as equally handoff-ready.
- The latest PRD direction requires a clearer purchase-task closed loop around "awaiting arrival -> ready for warehouse -> handed to warehouse".
- Task list/read/detail queries expose `procurement_summary`, but they still do not provide a stable frontend-friendly procurement-to-warehouse coordination contract.

## Current Context
- `CURRENT_STATE.md` before this round reports Step 11 complete and still describes the next step as a generic candidate rather than the now-selected procurement/warehouse coordination phase.
- `docs/api/openapi.yaml` is currently `v0.12.0`.
- Latest completed iteration before this round: `docs/iterations/ITERATION_011.md`.
- Main blockers for this round:
  - purchase-task warehouse prepare readiness is too loose for the intended arrival/handoff semantics
  - procurement and warehouse coordination is not expressed through one stable summary shape
  - `/v1/tasks`, `/v1/tasks/{id}`, and `/v1/tasks/{id}/detail` do not yet expose explicit procurement-to-warehouse readiness/handoff semantics

## Goals
- Tighten purchase-task warehouse handoff so that procurement completion, not just procurement activity, unlocks warehouse prepare.
- Add a stable derived procurement coordination summary that clearly expresses:
  - `awaiting_arrival`
  - `ready_for_warehouse`
  - `handed_to_warehouse`
- Keep procurement + warehouse coordination aligned across task list, single-task read, and aggregate detail queries.
- Preserve `close` / `closable` / `cannot_close_reasons` compatibility while improving warehouse-readiness semantics.

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
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Forbidden Scope
- Real authentication or RBAC enforcement
- Real ERP integration
- NAS / real upload integration
- Whole-hash strict verification
- Full procurement module explosion
- KPI / finance / export-center module implementation

## Expected File Changes
- Extend procurement summary domain contract with derived coordination fields.
- Tighten purchase-task warehouse-prepare gating and keep close-readiness contract unchanged.
- Align repo list-query projection logic with service-layer procurement/warehouse derivation.
- Add/update tests for arrival, warehouse readiness, and warehouse handoff coordination states.
- Sync OpenAPI, current state, iteration memory, and model handover / appendix docs.

## Required API / DB Changes
- API:
  - keep existing procurement endpoints
  - extend `procurement_summary` on `/v1/tasks`, `/v1/tasks/{id}`, and `/v1/tasks/{id}/detail` with derived coordination/readiness fields
  - document the stricter purchase-task warehouse-prepare gate and frontend-ready coordination semantics
- DB / migration:
  - no new table
  - no required migration if coordination stays derived from existing task / procurement / warehouse records

## Success Criteria
- Purchase-task warehouse prepare is only allowed once procurement has reached the arrival-complete state needed for warehouse handoff.
- `procurement_summary` exposes a stable coordination contract across list/read/detail.
- Query-layer and service-layer procurement/warehouse semantics stay aligned.
- `close` / `closable` / `cannot_close_reasons` contracts do not regress.
- `go test ./...` passes.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_012.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- Tightening warehouse prepare semantics changes purchase-task behavior from Step 11 and may affect clients that assumed `prepared|in_progress` was already warehouse-ready.
- Derived SQL projections must remain tightly aligned with service-layer workflow derivation to avoid filter/read-model drift.
- Frontend clients must consume the new coordination summary instead of inferring procurement-to-warehouse handoff purely from raw procurement status.

## Completion Output Format
1. Changed Files
2. DB / Migration Changes
3. API / OpenAPI Changes
4. Auto-Correction Notes
5. Verification
6. Risks / Remaining Gaps
7. Next Step
