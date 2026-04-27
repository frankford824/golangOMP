Title: MAIN task action minimum org-scoped authorization over canonical task ownership

Date: 2026-03-31
Model: GPT-5 Codex

## Goal
- Continue the MAIN task/org closure after canonical task ownership and minimum list/detail visibility.
- Organize key task write actions around one minimum authorization model:
  - role threshold
  - canonical task ownership scope
  - workflow handler/status semantics
- Keep this round explicitly below full ABAC complexity.

## Delivered
- Added a shared task action authorization layer:
  - `service/task_action_scope.go`
  - `service/task_action_rules.go`
  - `service/task_action_authorizer.go`
- Extended request-actor context so task authorization can read:
  - `department`
  - `team`
  - `managed_departments`
  - `managed_teams`
  - `frontend_access`
- Wired unified authorization into key task actions:
  - `POST /v1/tasks`
  - `GET /v1/tasks/{id}`
  - task business-info update
  - task assignment / reassignment
  - design submit
  - task asset upload-session create / complete / cancel
  - audit claim / approve / reject / transfer / handover / takeover
  - warehouse prepare / receive / reject / complete
  - task close
  - procurement update / advance
- Expanded the route role metadata/gates where needed so department/team management roles can reach the shared authorizer.
- Added structured `PERMISSION_DENIED` details:
  - `missing_required_role`
  - `task_out_of_department_scope`
  - `task_out_of_team_scope`
  - `task_not_assigned_to_actor`
  - `task_status_not_actionable`
- Added bounded authorization logging for task actions.

## Authorization model
- Global view-all:
  - `Admin`
  - `SuperAdmin`
  - `RoleAdmin`
  - `HRAdmin`
- Department-scoped management:
  - `DepartmentAdmin`
  - `DesignDirector`
  - bounded by canonical `owner_department`
- Team-scoped management:
  - `TeamLead`
  - bounded by canonical `owner_org_team`
- Workflow roles:
  - `Designer`
  - `Audit_A`
  - `Audit_B`
  - `Warehouse`
  - `Ops`
  - these roles still require action-specific handler/designer/creator or status alignment unless an allowed management scope matches

## Tests and verification
- Added unit coverage in `service/task_action_authorizer_test.go` for:
  - view-all allow
  - department scope allow/deny
  - team scope allow
  - handler allow/deny
  - missing-role deny
  - status deny
- Added handler/route regression coverage in `transport/handler/task_action_authorization_test.go` for:
  - detail read
  - assign
  - submit-design
  - audit approve
  - warehouse complete
  - close
- Local commands executed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
  - `go test ./repo/mysql`

## Boundaries kept explicit
- This is not full ABAC.
- This is not a generic rule engine or policy DSL.
- Legacy `owner_team` remains for compatibility.
- Ordinary members still keep the smallest practical self/handler-related path.
- Not every task-adjacent endpoint is unified yet:
  - batch remind remains outside the new shared task action authorizer
  - audit handover listing remains read-path behavior
  - compatibility/mock asset aliases are not the primary authorization source

## Publish status
- Overwrite publish was executed onto existing `v0.8` through:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task action minimum org authorization"`
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task action minimum org authorization fix"`
- First live verification exposed a real defect:
  - `TeamLead(运营三组)` could still assign a task owned by `运营一组` because generic department scope was being accepted
  - non-management workflow-role denial ordering could return org-scope denial before handler mismatch
- After the follow-up code fix and second overwrite publish, live verification passed for:
  - `8080 /health`
  - `8081 /health`
  - `8082 /health`
  - `/proc/<pid>/exe`
  - detail read allow/deny
  - department-scoped business-info allow/deny
  - team-scoped assign allow/deny
  - handler/self-related submit-design allow/deny
