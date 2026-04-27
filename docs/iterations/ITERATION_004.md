# ITERATION_004 - V7 Task Assets / Assign / Submit Design

**Date**: 2026-03-09  
**Scope**: STEP_04

## 1. Goals

- Add the V7 `task_assets` domain skeleton for frontend asset timeline and attachment areas
- Add task assignment action `assign`
- Add design submission action `submit-design`
- Complete the main task path `PendingAssign -> InProgress -> PendingAuditA`
- Keep Step 01-03 task/audit/warehouse/detail foundations intact
- Upgrade and sync OpenAPI at `v0.5.0`

## 2. Scope Boundary

- Implemented in this iteration:
  - `task_assets` domain and table
  - `POST /v1/tasks/{id}/assign`
  - `POST /v1/tasks/{id}/submit-design`
  - `GET /v1/tasks/{id}/assets`
  - `POST /v1/tasks/{id}/assets/mock-upload`
- Explicitly not implemented in this iteration:
  - real file upload
  - NAS integration
  - `whole_hash` strict validation
  - RBAC/auth
  - outsource lifecycle expansion

## 3. Changed Files

### New files

| File | Purpose |
|---|---|
| `domain/task_asset.go` | Task asset entity and asset-type enum |
| `repo/mysql/task_asset.go` | MySQL repo for `task_assets` |
| `service/task_assignment_service.go` | Task assign service |
| `service/task_asset_service.go` | Task asset timeline, mock upload, and submit-design service |
| `transport/handler/task_assignment.go` | Assign handler |
| `transport/handler/task_asset.go` | Asset timeline and mock upload handlers |
| `transport/handler/design_submission.go` | Submit-design handler |
| `db/migrations/004_v7_task_assets_assign_submit.sql` | Step 04 additive migration |
| `service/task_step04_service_test.go` | Service-level tests for Step 04 flows |
| `docs/iterations/ITERATION_004.md` | Step 04 iteration record |

### Modified files

| File | Change |
|---|---|
| `domain/audit.go` | Added Step 04 task event types |
| `repo/interfaces.go` | Added `TaskAssetRepo` and `TaskRepo.UpdateDesigner` |
| `repo/mysql/task.go` | Added designer update support |
| `service/task_service.go` | New tasks now start in `PendingAssign` |
| `transport/http.go` | Registered Step 04 task asset / assign / submit-design routes |
| `cmd/server/main.go` | Wired Step 04 repos, services, and handlers |
| `docs/api/openapi.yaml` | Documented Step 04 APIs and schemas |
| `CURRENT_STATE.md` | Synced repo state after Step 04 completion |

## 4. New / Modified Tables

### New tables

| Table | Notes |
|---|---|
| `task_assets` | Task-scoped asset timeline for reference, draft, revised, final, and outsource-return files |

### Version strategy

- `version_no` is a monotonic per-task sequence
- Unique constraint: `UNIQUE(task_id, version_no)`
- Ordering rule: `version_no ASC`
- Reason: frontend timeline needs one simple task-level sequence across all asset types, not separate counters per type

## 5. New / Modified APIs

| Method | Path | Notes |
|---|---|---|
| POST | `/v1/tasks/{id}/assign` | Assigns designer and moves task `PendingAssign -> InProgress` |
| POST | `/v1/tasks/{id}/submit-design` | Creates design asset and moves task to `PendingAuditA` |
| GET | `/v1/tasks/{id}/assets` | Returns task asset timeline ordered by `version_no ASC` |
| POST | `/v1/tasks/{id}/assets/mock-upload` | Creates task asset without changing task status |
| POST | `/v1/tasks` | Initial task status now starts at `PendingAssign` |

## 6. Business Rules Implemented

| Rule | Implementation |
|---|---|
| Assign only allowed in `PendingAssign` | Service-level state guard |
| Assign does not allow overwrite outside `PendingAssign` | Any non-`PendingAssign` task returns `409` |
| Assign updates both designer and current handler | `designer_id` and `current_handler_id` are both set to the assigned designer |
| Submit-design requires designer ownership first | `designer_id` must be present |
| Submit-design only allowed from active design states | `InProgress` and `RejectedByAuditA` only |
| Submit-design always creates a `task_assets` row | Asset metadata is persisted even when file storage is mocked |
| Submit-design pushes task into audit | Task moves to `PendingAuditA` |
| Mock upload must not move task state | Asset row created, task status unchanged |
| Assets are task-scoped, not V6 version-scoped | Separate `task_assets` table; no reuse of V6 `asset_versions` semantics |
| Assign / submit-design / mock-upload must be traceable | All three actions append `task_event_logs` with JSON payloads |

## 7. task_assets Model

`task_assets` fields:

- `id`
- `task_id`
- `asset_type`
- `version_no`
- `file_name`
- `file_path`
- `whole_hash`
- `uploaded_by`
- `remark`
- `created_at`

### Supported `asset_type`

- `reference`
- `draft`
- `revised`
- `final`
- `outsource_return`

## 8. State Transitions

- Task create: `PendingAssign`
- Assign: `PendingAssign -> InProgress`
- Submit-design first submission: `InProgress -> PendingAuditA`
- Submit-design resubmission: `RejectedByAuditA -> PendingAuditA`

## 9. Verification

- Added service-level tests for:
  - assign success
  - submit-design from `InProgress`
  - submit-design from `RejectedByAuditA`
  - mock-upload status preservation and version increment
- Ran `go test ./...`

## 10. Remaining Gaps

- No real file upload
- No NAS integration
- `whole_hash` is metadata only
- No auth/RBAC middleware
- Asset filtering/searching is still basic

## 11. Next Iteration Suggestion

- Query enhancement for asset timeline and task detail side panels
- Frontend-friendly aggregate shapes for integration
- Task list / asset list filter and sort enhancement
