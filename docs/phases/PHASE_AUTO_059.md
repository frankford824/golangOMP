# PHASE_AUTO_059

## Why This Phase Now
- Step D already made task entry minimally usable across all three task types.
- The next mainline business gap was Step E: post-entry audit, warehouse, and traceability closure.
- The narrowest correct increment was to harden the existing audit and warehouse paths instead of inventing new centers.

## Current Context
- Current CURRENT_STATE before this phase: Step 58 complete
- Current OpenAPI version before this phase: `0.58.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_058.md`
- Mainline focus: audit ownership, warehouse rejection/re-entry, and task-event closure

## Goals
- Make audit claim/approve/reject/handover/takeover paths consistent and business-safe.
- Make warehouse prepare/receive/reject/complete truthful and reusable after rejection.
- Ensure task events can answer who acted, on which task, when, and with what state/result across the mainline.
- Keep Step A-D intact.

## Allowed Scope
- Audit handler/service/domain/repo hardening strictly required by the Step E mainline
- Warehouse handler/service/domain/repo hardening strictly required by the Step E mainline
- Task event payload closure on existing task business events
- Narrow read-model truthfulness fixes for workflow, coordination, and available actions
- Focused docs and tests

## Forbidden Scope
- Warehouse subsystem redesign
- Procurement redesign beyond warehouse re-entry truthfulness
- New generic audit/event/observability frameworks
- Finance/order/aftersale/cross-border expansion
- Broad platform-center rewrites

## Expected File Changes
- `service/task_service.go`
- `service/audit_v7_service.go`
- `service/warehouse_service.go`
- `service/task_asset_service.go`
- `service/task_assignment_service.go`
- `service/task_detail_service.go`
- `service/task_workflow.go`
- `service/procurement_summary.go`
- focused Step E tests
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/phases/PHASE_AUTO_059.md`
- `docs/iterations/ITERATION_059.md`

## Required API / DB Changes
- API:
  - keep existing task/audit/warehouse routes
  - document truthful warehouse reject routing and rejected-receipt receive reuse
  - document audit handover/takeover ownership semantics
  - document the richer task-event stream semantics
- DB:
  - no new migration in this phase
  - continue reusing `audit_records`, `audit_handovers`, `warehouse_receipts`, and `task_event_logs`

## Success Criteria
- Audit approve/reject/handover/takeover no longer leave stale ownership behind.
- Warehouse reject no longer dead-ends the task in a generic blocked state.
- Re-prepared tasks can be received again after a rejected warehouse receipt.
- Task list/detail/read-model workflow and coordination fields stay truthful after audit/warehouse exceptions.
- Key task events across create/assign/submit-design/audit/procurement/warehouse/close provide usable business/debug traceability.
- `go test ./...` passes and Step A-D behavior remains intact.
