# ITERATION_019 - Lightweight Queue Ownership Hint / Saved Workbench Preferences

**Date**: 2026-03-09  
**Scope**: STEP_19

## 1. Goals

- Add lightweight, non-enforced ownership hints to preset task-board queues.
- Add placeholder-actor-scoped saved workbench preferences for frontend restore/bootstrap.
- Keep existing `queue_key / filters / normalized_filters / query_template` semantics stable.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 18 complete and OpenAPI `v0.15.4`.
- Candidate-scan hotspot assessment was already complete, with a clear decision to defer broad index/materialized-view work by default.
- This round remained explicitly out of scope for:
  - real auth / RBAC enforcement
  - real ERP / NAS / upload work
  - strict `whole_hash` verification
  - full queue ownership persistence
  - full personal inbox persistence

## 3. Files Changed

### Code

- `cmd/server/main.go`
- `db/migrations/009_v7_workbench_preferences.sql`
- `domain/task_board.go`
- `domain/workbench.go`
- `repo/interfaces.go`
- `repo/mysql/workbench.go`
- `service/task_board_service.go`
- `service/task_board_service_test.go`
- `service/workbench_service.go`
- `service/workbench_service_test.go`
- `transport/http.go`
- `transport/handler/workbench.go`

### Documents

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_019.md`
- `docs/phases/PHASE_AUTO_019.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- Added `db/migrations/009_v7_workbench_preferences.sql`.
- New table: `workbench_preferences`
  - keyed by `actor_id + actor_roles_key + auth_mode`
  - stores:
    - `default_queue_key`
    - `pinned_queue_keys`
    - `default_filters`
    - `default_page_size`
    - `default_sort`
- No real queue-ownership or inbox-persistence tables were introduced.

## 5. API Changes

### External contract

- Extended task-board queue payloads with optional lightweight hint fields:
  - `suggested_roles`
  - `suggested_actor_type`
  - `default_visibility`
  - `ownership_hint`
- Added:
  - `GET /v1/workbench/preferences`
  - `PATCH /v1/workbench/preferences`
- Existing task-board drill-down fields remain stable:
  - `queue_key`
  - `filters`
  - `normalized_filters`
  - `query_template`

### Workbench preferences behavior

- Preferences are scoped by the current placeholder request actor:
  - `X-Debug-Actor-Id`
  - `X-Debug-Actor-Roles`
- The response returns:
  - placeholder actor context
  - effective saved preferences
  - workbench config with filters schema, supported page sizes/sorts, and preset queue metadata
- Supported saved fields:
  - `default_queue_key`
  - `pinned_queue_keys`
  - `default_filters`
  - `default_page_size`
  - `default_sort`

### OpenAPI

- Version updated from `0.15.4` to `0.16.0`.
- Documented queue ownership fields as hint-only.
- Documented workbench preferences as placeholder-actor-scoped saved settings, not real auth.

## 6. Design Decisions

- Keep queue ownership lightweight and advisory instead of introducing permission or assignment enforcement.
- Use one small persistence table so preferences survive process restarts without turning into a full inbox system.
- Reuse the stable task-board preset metadata and existing `/v1/tasks` query-template contract for workbench bootstrap.
- Keep supported backend sort options intentionally narrow (`updated_at_desc`) to avoid implying server behavior that does not yet exist.

## 7. Correction Notes

- Before implementation, repository-truth docs still correctly reflected Step 18 / OpenAPI `v0.15.4`; no contradictory code drift had to be rolled back first.
- After implementation, repository-truth docs were advanced together to Step 19 / OpenAPI `v0.16.0`.

## 8. Verification

- Ran `gofmt -w` on all touched Go files.
- Added service tests covering:
  - task-board ownership hint exposure
  - placeholder-actor-scoped workbench preference persistence
  - preference validation
- Ran:
  - `go test ./service/...`
  - `go test ./transport/...`
  - `go test ./repo/mysql/...`
  - `go test ./...`

## 9. Risks / Known Gaps

- Queue ownership hints remain advisory only; frontend must not treat them as permission or assignment enforcement.
- Placeholder actor scoping is only a temporary persistence boundary and is not equivalent to real user identity.
- `default_sort` is intentionally limited to the current backend-safe default and does not imply broader server-side sort support.
- Real auth, ERP, NAS/upload, full ownership persistence, and full inbox persistence remain out of scope.

## 10. Suggested Next Step

- Keep real auth and inbox-persistence work deferred.
- If another workbench iteration is needed, prefer small frontend-usage improvements only, such as lightweight recent-context restore or queue presentation refinements, while continuing to reuse the stable board/list/query contract.
