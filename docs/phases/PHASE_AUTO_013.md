# PHASE_AUTO_013 - Task Board / Inbox Aggregation for Role-Based Collaboration

## Why This Phase Now
- Step 12 stabilized projected workflow and procurement-to-warehouse coordination fields, but frontend consumers still need to compose generic task lists into role-specific workbenches by themselves.
- The latest PRD direction expects role-oriented task pools for operations, design, audit, procurement, and warehouse instead of relying only on `/v1/tasks`.
- The current repository already has enough derived read-model fields (`workflow.*` and `procurement_summary.*`) to build a minimal board/inbox layer without expanding into real auth, ERP, NAS, or upload scope.

## Current Context
- `CURRENT_STATE.md` before this round reports Step 12 complete.
- `docs/api/openapi.yaml` is currently `v0.13.0`.
- Latest completed iteration before this round: `docs/iterations/ITERATION_012.md`.
- Main blockers for this round:
  - no board / inbox level aggregation API over projected workflow and procurement coordination
  - no preset queue counts for role-oriented workbenches
  - warehouse, procurement, operations, and audit still depend on generic task-list composition in frontend code

## Goals
- Add frontend-ready task-board aggregation APIs over `workflow.main_status`, `workflow.sub_status`, and `procurement_summary.coordination_status`.
- Provide preset queues, queue counts, queue conditions, and sample or first-page tasks for role-oriented workbenches.
- Cover at least these minimal preset pools:
  - ops pending materials
  - designer pending submit
  - audit pending review
  - procurement pending follow-up
  - awaiting arrival
  - pending warehouse prepare
  - warehouse pending receive
  - pending close
- Keep `/v1/tasks`, `/v1/tasks/{id}`, `/v1/tasks/{id}/detail`, close flow, and procurement/warehouse coordination contracts backward compatible.

## Allowed Scope
- `domain/`
- `service/`
- `transport/`
- `cmd/server/main.go`
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
- Real authentication or permission enforcement
- Real ERP integration
- NAS / real upload integration
- `whole_hash` strict verification
- Full BI / KPI module
- Finance / export-center module implementation
- Supplier settlement or full inbound receipt lifecycle expansion

## Expected File Changes
- Add task-board / inbox domain response contracts.
- Add a task-board aggregation service that reuses current task read-model semantics instead of inventing a second workflow source of truth.
- Add task-board handlers and route registration.
- Add tests for preset queues, queue counts, and queue pagination behavior.
- Sync OpenAPI, current state, iteration memory, and handover documents with the new board endpoints.

## Required API / DB Changes
- API:
  - add `GET /v1/task-board/summary`
  - add `GET /v1/task-board/queues`
  - document board-view presets, queue filter definitions, counts, and sample/task-page shapes as frontend-ready aggregate APIs
- DB / migration:
  - no new table
  - no migration required; board data is derived from existing task, procurement, asset, and warehouse projections

## Success Criteria
- Board endpoints expose stable preset queues with queue identifiers, names, condition definitions, counts, and sample or paginated tasks.
- Procurement / warehouse queues consume the existing derived coordination contract instead of re-deriving raw procurement state client-side.
- Existing task list/detail/read-model and close / procurement / warehouse contracts do not regress.
- `go test ./...` passes.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_013.md`
- `docs/api/openapi.yaml`

Optional but expected for this round:
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- Board presets are derived from current read-model semantics, so future workflow changes must keep the board layer aligned with `/v1/tasks`.
- Some role pools are intentionally minimal and heuristic-based because this round does not add real auth or persisted queue ownership.
- Aggregation currently favors frontend-ready contract stability over BI-style analytical flexibility.

## Completion Output Format
1. Changed Files
2. DB / Migration Changes
3. API / OpenAPI Changes
4. Auto-Correction Notes
5. Verification
6. Risks / Remaining Gaps
7. Next Step
