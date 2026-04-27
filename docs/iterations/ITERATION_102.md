# ITERATION_102

## Phase
MAIN audit / warehouse / close task-action minimum org gating closure on live `v0.8`

## Input Context
- Previous rounds had already closed:
  - canonical task ownership (`owner_department`, `owner_org_team`, legacy `owner_team` compatibility)
  - minimum task read visibility
  - `/assign` / bounded `reassign` org gating
- This round needed to extend the same shared authorizer line into:
  - audit actions
  - warehouse actions
  - close actions
- Constraints stayed explicit:
  - no full ABAC
  - no role-only global allow
  - no status-bypass for `Admin`
  - no rollback of owner/team compatibility or existing list/detail behavior

## Real Routed Action Scope
- Audit routes actually present:
  - `POST /v1/tasks/:id/audit/claim`
  - `POST /v1/tasks/:id/audit/approve`
  - `POST /v1/tasks/:id/audit/reject`
  - `POST /v1/tasks/:id/audit/transfer`
  - `POST /v1/tasks/:id/audit/handover`
  - `POST /v1/tasks/:id/audit/takeover`
- Warehouse routes actually present:
  - `POST /v1/tasks/:id/warehouse/receive`
  - `POST /v1/tasks/:id/warehouse/reject`
  - `POST /v1/tasks/:id/warehouse/complete`
- Close routes actually present:
  - `POST /v1/tasks/:id/close`
- Not present in current MAIN route table:
  - audit `submit`
  - audit `return`
  - warehouse `reopen`
  - warehouse `return`
  - task `reopen`
  - pending-close confirm
  - reject-close

## Final Design
- Shared authorizer remains the only runtime decision point for the above actions.
- Role + scope + status/handler are combined as follows:
  - view-all roles may cross org scope but cannot cross invalid status
  - department management requires canonical `owner_department`
  - team management requires canonical `owner_org_team`
  - audit A and audit B are resolved by workflow stage and are no longer globally interchangeable
  - non-management audit/warehouse actors still need current-handler semantics where the action requires it
  - close stays status-gated to `PendingClose`, with closability still checked after permission
- Result:
  - minimum usable, explainable gating
  - still not full ABAC
  - still compatible with legacy `owner_team`

## Files Changed
- Runtime:
  - `service/task_action_rules.go`
  - `service/task_action_authorizer.go`
  - `service/audit_v7_service.go`
  - `service/warehouse_service.go`
- Tests:
  - `service/task_action_authorizer_test.go`
  - `transport/handler/task_action_authorization_test.go`
- Docs:
  - `docs/api/openapi.yaml`
  - `CURRENT_STATE.md`
  - `MODEL_HANDOVER.md`
  - `ITERATION_INDEX.md`

## Local Verification
- Passed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
  - `go test ./repo/mysql`

## Publish
- Overwrite publish stayed on the existing release line:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task action org gating for audit warehouse close"`
- Entrypoint remained:
  - `./cmd/server`

## Live Acceptance
- Runtime after overwrite:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/3532025/exe -> /root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - `/proc/3532047/exe -> /root/ecommerce_ai/releases/v0.8/erp_bridge`
  - `/proc/3532217/exe -> /root/ecommerce_ai/erp_bridge_sync`
  - active executables were not deleted
- Temporary scoped verification users were created through real register/login flows:
  - `iter102_probe_1775029501`
  - `iter102_audit_out_1775029502`
  - `iter102_ops_in_1775029503`
  - `iter102_ops_out_1775029504`
- Audit:
  - task `163`:
    - out-of-scope audit A actor returned `403`
    - `deny_code=task_out_of_team_scope`
  - task `165`:
    - in-scope audit A approve returned `200`
    - follow-up status `PendingAuditB`
    - out-of-scope audit B approve returned `403`
    - `deny_code=task_out_of_team_scope`
    - in-scope audit B approve returned `200`
    - follow-up status `PendingWarehouseReceive`
- Warehouse:
  - task `165`:
    - out-of-scope receive returned `403`
    - `deny_code=task_out_of_department_scope`
    - in-scope receive returned `201`
    - in-scope complete returned `200`
    - follow-up status `PendingClose`
  - task `163`:
    - wrong-stage receive returned `403`
    - `deny_code=warehouse_stage_mismatch`
- Close:
  - task `137`:
    - out-of-scope close returned `403`
    - `deny_code=task_out_of_department_scope`
    - in-scope close returned `200`
    - follow-up status `Completed`
  - task `163`:
    - wrong-status close returned `403`
    - `deny_code=task_not_closable`
- Regression:
  - task `172`:
    - `/assign` reassign `41 -> 42 -> 41` both returned `200`
    - status stayed `InProgress`
  - task `163`:
    - `/assign` still returned `403`
    - `deny_code=task_not_reassignable`
  - canonical ownership still returned on live list/detail:
    - `owner_team`
    - `owner_department`
    - `owner_org_team`

## Boundaries Left Explicit
- This is still minimum task-action gating, not full ABAC.
- Legacy `owner_team` still has not been retired.
- Historical tasks with empty canonical ownership still exist.
- Close readiness remains separate from permission.
- Actions without routed endpoints remain outside the unified runtime surface until those routes are added.
