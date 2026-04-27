# ITERATION_002 - V7 Audit / Handover / Outsource / Task Event Log

**Date**: 2026-03-09  
**Scope**: STEP_02

## 1. Goals

- Add V7 audit skeleton: `AuditRecord`, `AuditHandover`
- Add V7 outsource skeleton: `OutsourceOrder`
- Land task-scoped event logging without breaking V6 `event_logs`
- Expose minimal frontend-facing APIs for audit, handover, takeover, outsource, and task events
- Upgrade OpenAPI to `v0.3.0`

## 2. Changed Files

### New files

| File | Purpose |
|---|---|
| `domain/audit.go` | AuditRecord / AuditHandover domain entities |
| `domain/outsource.go` | OutsourceOrder domain entity |
| `domain/task_event.go` | TaskEvent entity and task event log design rationale |
| `repo/mysql/audit_v7.go` | V7 audit repo implementation |
| `repo/mysql/outsource.go` | V7 outsource repo implementation |
| `repo/mysql/task_event.go` | Task event repo implementation |
| `service/audit_service.go` | V7 audit service contract and request params |
| `service/audit_v7_service.go` | V7 audit service implementation |
| `service/outsource_service.go` | V7 outsource service |
| `service/task_event_service.go` | Task event read service |
| `transport/handler/audit_v7.go` | Audit claim/approve/reject/transfer/handover/takeover handlers |
| `transport/handler/outsource.go` | Outsource create/list handlers |
| `db/migrations/002_v7_audit_outsource.sql` | Step 02 additive migration |

### Modified files

| File | Change |
|---|---|
| `domain/enums_v7.go` | Added Step 02 enums |
| `repo/interfaces.go` | Added TaskRepo status/handler updates and Step 02 repo interfaces |
| `repo/mysql/task.go` | Added task status and handler update methods |
| `domain/task_event.go` | `TaskEvent.Payload` now returns raw JSON instead of base64 bytes |
| `service/task_event_service.go` | Event listing now checks task existence and returns 404 semantics |
| `transport/handler/audit_v7.go` | `takeover` now passes `task_id` through to the service for task-scoped validation |
| `transport/http.go` | Registered 9 Step 02 routes |
| `cmd/server/main.go` | Wired Step 02 repos, services, and handlers |
| `docs/api/openapi.yaml` | Upgraded to `v0.3.0` and documented Step 02 APIs |
| `CURRENT_STATE.md` | Synced actual repo state after Step 02 completion |

## 3. Database Changes

### New tables

| Table | Notes |
|---|---|
| `audit_records` | Task-scoped audit action records |
| `audit_handovers` | Task-scoped audit shift handovers |
| `outsource_orders` | Task-scoped outsource orders |
| `task_event_logs` | Task-scoped event log |
| `task_event_sequences` | Per-task sequence counter for task events |

### Preserved V6 tables

- `event_logs` unchanged
- All V6 tables and routes remain intact

## 4. API Changes

### New/updated Step 02 APIs

| Method | Path |
|---|---|
| POST | `/v1/tasks/:id/audit/claim` |
| POST | `/v1/tasks/:id/audit/approve` |
| POST | `/v1/tasks/:id/audit/reject` |
| POST | `/v1/tasks/:id/audit/transfer` |
| POST | `/v1/tasks/:id/audit/handover` |
| POST | `/v1/tasks/:id/audit/takeover` |
| POST | `/v1/tasks/:id/outsource` |
| GET | `/v1/outsource-orders` |
| GET | `/v1/tasks/:id/events` |

## 5. task_event_logs Design

### Decision

Use an independent `task_event_logs` table instead of extending V6 `event_logs`.

### Reason

1. V6 `event_logs` is SKU-scoped and keyed by `(sku_id, sequence)`.
2. Step 02 events are task-scoped and should not force `sku_id` coupling.
3. Extending V6 `event_logs` would change existing repo contracts and recovery logic.
4. A separate table keeps V6 compatibility and gives Task detail pages a clean event source.

### Compatibility

- V6 `event_logs` is untouched
- V7 writes task business events into `task_event_logs`
- Future unification can be done with a view or fan-out worker if needed

## 6. Business Rules Implemented

| Rule | Implementation |
|---|---|
| Audit actions are task-scoped | All Step 02 handlers and services are keyed by `task_id` |
| Claim sets current auditor | `tasks.current_handler_id` updated on claim |
| Approve / reject write `audit_records` | Persisted inside the same transaction as task changes |
| Handover / takeover write `audit_handovers` | Handover creates record, takeover updates status |
| Outsource create writes `outsource_orders` | Persisted before task moves to `Outsourcing` |
| Critical changes write `task_event_logs` | All Step 02 stateful actions append task events |
| Handover does not finish a task | No terminal status change on handover |
| Transfer / handover only work in active audit stages | Current `task_status` must map to the requested/derived stage |
| Handover / takeover stage is inferred from task status | Avoids hard-coding stage `A` |
| Takeover must match both task and designated auditor | Validates `handover.task_id` and `handover.to_auditor_id` |
| Task event payloads are frontend-friendly | `payload` returns raw JSON |
| Missing task events return 404 semantics | Task existence is checked before listing events |

## 7. Unfinished Items

- No warehouse receipt skeleton yet
- No task assets skeleton yet
- No task aggregate detail API yet
- No handover list-by-task API yet
- No outsource lifecycle transitions beyond create/list yet
- No RBAC/auth middleware yet
- No ERP sync worker yet

## 8. Next Iteration Suggestion

- Add `warehouse_receipts`
- Add `task_assets`
- Add task aggregate detail query
- Add task handover list endpoint
- Extend outsource lifecycle operations
