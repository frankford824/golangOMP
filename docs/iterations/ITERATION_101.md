# ITERATION_101

## Phase
MAIN `/v1/tasks/{id}/assign` status-gating repair with bounded reassign support on live `v0.8`

## Input Context
- Frontend repro:
  - `POST /v1/tasks/170/assign`
  - body: `{"designer_id":42,"designer_name":"iter098_designer_b_74931810"}`
  - response: `403 PERMISSION_DENIED`
  - details showed:
    - `action=assign`
    - `deny_code=task_status_not_actionable`
    - `matched_rule=role_plus_assignment_scope`
    - actor roles already included `Admin/Designer/Member/Ops/SuperAdmin`
- Goal:
  - verify the real status and authorizer path first
  - decide whether `/assign` was correctly strict or missing reassign semantics
  - fix without weakening canonical ownership / org-scoped task action gating

## Root Cause
- Live task `170` was not a pending assignment:
  - `task_status=InProgress`
  - `designer_id=41`
  - `current_handler_id=41`
  - `owner_department=运营部`
  - `owner_org_team=运营一组`
- The task had been created with `designer_id`, so its initial event log already showed:
  - `task.created`
  - `assigned_at_creation=true`
  - `initial_task_status=InProgress`
- Old implementation had no reassign branch:
  - `service/task_action_rules.go` only allowed `TaskActionAssign` on `PendingAssign`
  - `service/task_assignment_service.go` hard-coded `PendingAssign -> InProgress`
  - any `InProgress` call to `/assign` was denied before scope evaluation with `task_status_not_actionable`
- Therefore the live failure was real rule narrowness, not actor-role shortage and not org-scope shortage.

## Final Rule
- Route kept unchanged:
  - `POST /v1/tasks/{id}/assign`
- Internal semantics are now split by current task status:
  - `PendingAssign`:
    - semantic action = `assign`
    - allowed for existing assign path (`Ops` or management role) with existing org-scope rules
    - success sets `designer_id` + `current_handler_id`
    - status transitions to `InProgress`
    - event = `task.assigned`
  - `InProgress`:
    - semantic action = `reassign`
    - allowed only for management roles:
      - `Admin`
      - `SuperAdmin`
      - `RoleAdmin`
      - `HRAdmin`
      - `DepartmentAdmin`
      - `TeamLead`
      - `DesignDirector`
    - allowed scopes:
      - `view_all`
      - department / managed-department scope
      - team / managed-team scope
    - success updates `designer_id` + `current_handler_id`
    - status remains `InProgress`
    - event = `task.reassigned`
  - audit / warehouse / close style states:
    - denied
    - machine-readable deny code = `task_not_reassignable`
- This round intentionally does not open reassign on:
  - `PendingAuditA`
  - `PendingAuditB`
  - `PendingOutsourceReview`
  - `PendingWarehouseReceive`
  - `PendingClose`
  - `Completed`

## Files Changed
- `domain/audit.go`
- `service/task_action_rules.go`
- `service/task_action_authorizer.go`
- `service/task_assignment_service.go`
- `service/task_action_authorizer_test.go`
- `service/task_step04_service_test.go`
- `transport/handler/task_action_authorization_test.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `ITERATION_INDEX.md`

## Local Verification
- Passed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
- Additional focused pass:
  - `go test ./service -run "TaskActionAuthorizer|TaskAssignmentService"`
- Blocked by host execution policy:
  - `go test ./repo/mysql`
  - real error: `fork/exec ...\\mysql.test.exe: An Application Control policy has blocked this file.`

## Publish
- Overwrite publish stayed on the existing release line:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task assign reassign status gating fix"`
- Release history records:
  - packaged at `2026-04-01T07:16:29Z`
  - deployed at `2026-04-01T07:16:42Z`
- Entrypoint remained:
  - `./cmd/server`

## Live Acceptance
- Runtime after overwrite:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/3519769/exe -> /root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - `/proc/3519812/exe -> /root/ecommerce_ai/releases/v0.8/erp_bridge`
  - `/proc/3519962/exe -> /root/ecommerce_ai/erp_bridge_sync`
  - active executables were not deleted
- Current repro sample, task `170`:
  - before fix verification:
    - detail read showed `InProgress`, `assignee_id=41`, `current_handler_id=41`
  - out-of-scope manager denial:
    - actor: temporary `TeamLead` in `运营三组`
    - response: `403 PERMISSION_DENIED`
    - deny details:
      - `action=reassign`
      - `deny_code=task_out_of_team_scope`
      - `matched_rule=manager_role_plus_reassignment_scope`
  - in-scope manager reassign success:
    - actor: temporary `TeamLead` in `运营一组`
    - `POST /v1/tasks/170/assign` to designer `42` -> `200`
    - follow-up detail read showed:
      - `task_status=InProgress`
      - `assignee_id=42`
      - `assignee_name=iter098_designer_b_74931810`
      - `current_handler_id=42`
  - restore step:
    - same in-scope manager reassigned task `170` back to designer `41`
    - follow-up detail read again showed `assignee_id=41`, `current_handler_id=41`
- Assign success sample:
  - task `169` before:
    - `task_status=PendingAssign`
    - `assignee_id=null`
    - `current_handler_id=null`
  - admin `POST /v1/tasks/169/assign` to designer `42` -> `200`
  - follow-up detail read showed:
    - `task_status=InProgress`
    - `assignee_id=42`
    - `assignee_name=iter098_designer_b_74931810`
    - `current_handler_id=42`
- Forbidden status sample:
  - task `165`:
    - current status `PendingAuditA`
  - admin `POST /v1/tasks/165/assign` -> `403 PERMISSION_DENIED`
  - deny details:
    - `action=reassign`
    - `deny_code=task_not_reassignable`
- Event/log proof:
  - `task_event_logs`:
    - task `169` sequence `3` -> `task.assigned`, `action=assign`, `designer_id=42`
    - task `170` sequence `4` -> `task.reassigned`, `previous_designer_id=41`, `designer_id=42`
    - task `170` sequence `5` -> `task.reassigned`, `previous_designer_id=42`, `designer_id=41`
  - live server log now records:
    - `task_action_auth action=reassign ...`
    - `task_assignment trace_id=... task_id=... action=assign|reassign ... previous_designer_id=... new_designer_id=... previous_status=... resulting_status=... allow=... deny_reason=...`
- Temporary live verification users:
  - created users `50` and `51`
  - both were disabled after acceptance

## Remaining Boundaries
- This is still not full task-action ABAC.
- `/assign` does not support reopening or reassigning tasks already in audit / warehouse / close phases.
- No standalone `/reassign` route was added; the distinction is internal semantic branching only.
- `go test ./repo/mysql` remains blocked by host policy, so repo/mysql runtime tests could not be executed on this workstation in this round.
