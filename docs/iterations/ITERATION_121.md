# ITERATION 121

Date: 2026-04-11
Model: GPT-5 Codex

## Goal

Audit `/v1/tasks` list/detail actor-source fields against runtime truth, fill missing requester/current-handler/name projections through the shared read-model path, overwrite-publish to live `v0.9`, and complete live acceptance.

## Runtime truth before change

- `GET /v1/tasks`
  - returned `creator_id`, `designer_id`, `current_handler_id`, `owner_team`, `owner_department`, `owner_org_team`
  - did not return `requester_id`
  - did not return actor display-name fields such as `requester_name`, `creator_name`, `designer_name`, `current_handler_name`
- `GET /v1/tasks/{id}`
  - returned `creator_id`, `designer_id`, `current_handler_id`, `owner_*`
  - returned compatibility `assignee_id/assignee_name` and `creator_name`
  - did not return canonical `requester_id/requester_name`
  - did not return canonical `designer_name` or `current_handler_name`
- `GET /v1/tasks/{id}/detail`
  - nested `task` carried persisted ids only
  - no root-level actor display-name projection
- Create-path gap:
  - request payload accepted optional `requester_id`
  - runtime create path did not persist it anywhere
  - event payload also did not record it

## Code change

- Added persistent requester storage:
  - `domain/task.go`
  - `repo/mysql/task.go`
  - `db/migrations/051_v7_task_requester_projection.sql`
- Added shared actor/name read projections:
  - `domain/query_views.go`
  - `service/task_service.go`
  - `service/task_detail_service.go`
  - `repo/mysql/task.go`
- Added `/detail` aggregate actor parity:
  - `domain/task_detail_aggregate.go`
  - `cmd/server/main.go`
  - `cmd/api/main.go`
- Added test coverage:
  - `service/task_actor_read_model_test.go`
  - `repo/mysql/task_test.go`

## Source-of-truth mapping

- `requester_id`
  - persisted on `tasks.requester_id`
  - new writes default to `creator_id` when create payload omits `requester_id`
  - historical rows are backfilled by migration `051`
- `requester_name`
  - resolved from `users.display_name`
  - falls back to `users.username`
- `creator_id`
  - persisted on `tasks.creator_id`
- `creator_name`
  - resolved from `users.display_name` with `username` fallback
- `designer_id`
  - persisted on `tasks.designer_id`
- `designer_name`
  - resolved from `users.display_name` with `username` fallback
- `current_handler_id`
  - persisted on `tasks.current_handler_id`
  - updated by assign / audit / warehouse / close workflow services
  - may be null on unclaimed or shared-state lanes
- `current_handler_name`
  - resolved from `users.display_name` with `username` fallback
- `assignee_id/assignee_name`
  - retained as compatibility alias on `GET /v1/tasks/{id}`
  - mirrors `designer_id/designer_name`
- `owner_team`
  - legacy compatibility field on `tasks.owner_team`
- `owner_department`, `owner_org_team`
  - canonical org-source fields on `tasks.owner_department`, `tasks.owner_org_team`

## Local verification

- Passed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
  - `go test ./repo/mysql`
- Note:
  - first local test attempt hit Windows host restrictions on the default Go build cache path
  - rerun succeeded after pinning `GOCACHE` and `GOMODCACHE` to repo-local directories

## Migration and deploy

- Applied live migration before deploy:
  - `db/migrations/051_v7_task_requester_projection.sql`
  - backup dir: `/root/ecommerce_ai/backups/pre-051-v0.9-20260411T030809Z`
  - result:
    - `REQUESTER_COLUMN_PRESENT=1`
    - `REQUESTER_NULL_COUNT=0`
- Overwrite deploy command:
  - `bash ./deploy/deploy.sh --version v0.9 --skip-tests --release-note "overwrite v0.9 task actor/source projection closure"`
- Deploy result:
  - success on first attempt
  - runtime verify reported:
    - `8080 /health = 200`
    - `8081 /health = 200`
    - `8082 /health = 200`
    - `/proc/<pid>/exe`:
      - main -> `/root/ecommerce_ai/releases/v0.9/ecommerce-api`
      - bridge -> `/root/ecommerce_ai/releases/v0.9/erp_bridge`
      - sync -> `/root/ecommerce_ai/erp_bridge_sync`
    - active executables were not deleted

## Live acceptance

- Real login:
  - `POST /v1/auth/login` with `admin / <ADMIN_PASSWORD>` -> `200`
- Required list probe:
  - `GET /v1/tasks?page=1&page_size=5` -> `200`
  - top returned tasks `388, 387, 386, 385, 384` all carried:
    - `requester_id/requester_name`
    - `creator_id/creator_name`
    - `designer_id/designer_name`
    - `current_handler_id/current_handler_name`
    - `owner_team/owner_department/owner_org_team`
- Status-specific list probes:
  - `GET /v1/tasks?status=PendingAssign&page=1&page_size=5` -> ids `365, 364, 363, 362, 360`
  - `GET /v1/tasks?status=InProgress&page=1&page_size=5` -> ids `385, 380, 378, 375, 374`
  - `GET /v1/tasks?status=PendingAuditA&page=1&page_size=5` -> ids `388, 387, 386, 382, 379`
  - `GET /v1/tasks?status=Completed&page=1&page_size=5` -> id `377`
- Sampled real task details:
  - task `365` (`PendingAssign`)
    - `requester_id=1`, `requester_name=系统管理员`
    - `designer_id=null`, `designer_name=null`
    - `current_handler_id=null`, `current_handler_name=null`
    - semantics matched “待分配 / 未认领”
  - task `385` (`InProgress`)
    - `requester_id=1`, `requester_name=系统管理员`
    - `designer_id=5`, `designer_name=Candidate Test`
    - `current_handler_id=5`, `current_handler_name=Candidate Test`
    - `assignee_id/assignee_name` matched designer fields
  - task `388` (`PendingAuditA`)
    - `requester_id=1`, `requester_name=系统管理员`
    - `designer_id=5`, `designer_name=Candidate Test`
    - `current_handler_id=1`, `current_handler_name=系统管理员`
    - showed the claimed-audit lane where current handler is not null even in audit state
  - task `377` (`Completed`)
    - `requester_id=1`, `requester_name=系统管理员`
    - `current_handler_id=null`, `current_handler_name=null`
    - semantics matched completed/unclaimed terminal state
- Aggregate parity probe:
  - `GET /v1/tasks/385/detail` -> `200`
  - root returned:
    - `creator_id=1`
    - `requester_id=1`
    - `designer_id=5`
    - `current_handler_id=5`
    - `creator_name=系统管理员`
    - `requester_name=系统管理员`
    - `designer_name=Candidate Test`
    - `current_handler_name=Candidate Test`
    - `assignee_id=5`
    - `assignee_name=Candidate Test`
  - nested `task.requester_id=1`, `task.current_handler_id=5` remained aligned

## Residual note

- `PendingAuditA` does not have a single universal `current_handler_id` rule:
  - tasks `386` and `387` returned `null`
  - task `388` returned the claimed auditor/operator `1`
  - this is consistent with “shared queue when unclaimed, concrete handler when claimed” rather than a bug in the new projection itself
