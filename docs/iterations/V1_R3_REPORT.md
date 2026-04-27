# V1 R3 Report

> Scope: Blueprint engine, module permission gate, pool/claim, module action, cancel, and R3 transport wiring.
> Environment: local code-only execution. No production `jst_erp` write was performed.

## Implementation

- Added module runtime constants and models:
  - `domain/module_state.go`
  - `domain/module_event.go`
  - `domain/deny_code.go`
  - `domain/module_runtime.go`
  - `domain/workflow_blueprints.go`
- Added R3 repo interfaces and MySQL wrappers:
  - `repo/task_module_repo.go`
  - `repo/task_module_event_repo.go`
  - `repo/reference_file_ref_repo.go`
  - `repo/mysql/task_module_repo.go`
  - `repo/mysql/task_module_event_repo.go`
  - `repo/mysql/reference_file_ref_repo.go`
- Added service packages:
  - `service/blueprint`: six task-type blueprint registry and downstream rules.
  - `service/module`: module descriptors, action registry, and state machine.
  - `service/permission`: `AuthorizeModuleAction` with Layer 2 scope and Layer 3 action gate.
  - `service/task_pool`: pool query and CAS claim service.
  - `service/task_cancel`: `user_cancel` / admin force close behavior via `force`.
  - `service/task_aggregator`: module detail/status aggregation.
  - `service/module_action`: action executor separated from `service/module` to avoid Go import cycles.
- Wired R3 handlers into existing `/v1/tasks` registration:
  - `GET /v1/tasks/pool`
  - `POST /v1/tasks/{id}/modules/{module_key}/claim`
  - `POST /v1/tasks/{id}/modules/{module_key}/actions/{action}`
  - `POST /v1/tasks/{id}/modules/{module_key}/reassign`
  - `POST /v1/tasks/{id}/modules/{module_key}/pool-reassign`
  - `POST /v1/tasks/{id}/cancel`
- Marked the corresponding R1 reserved route specs as live overlaps, so the `501 not_implemented` stubs no longer register for R3 endpoints.

## Hard Gates

- Pool query filters `data.backfill_placeholder=true` modules with JSON comparison.
- Pool query ordering uses:
  `FIELD(t.priority, 'critical', 'high', 'normal', 'low') ASC, t.created_at ASC`
- `module_claim_conflict` maps to HTTP 409.
- Module permission deny codes are limited to the six R3 codes in `domain/deny_code.go` / `service/permission/deny_code.go`.
- OpenAPI was not modified.
- R2 migrations were not modified.
- No command executed any production write to `jst_erp`.

## Verification

Commands run locally:

```bash
/home/wsfwk/go/bin/go build ./...
/home/wsfwk/go/bin/go test ./... -count=1
/home/wsfwk/go/bin/go test ./... -count=1 -tags=integration
/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
```

Results:

- `go build ./...`: pass
- `go test ./... -count=1`: pass
- `go test ./... -count=1 -tags=integration`: pass
- OpenAPI validation: `0 error 0 warning`

## Notes

- The prompt-listed `domain/workflow_blueprints.go` was absent at start; R3 added it.
- The current OpenAPI still declares `501` response alternatives for R3 endpoints, but the implementation now routes to live handlers without editing the frozen schema.
- The 100-thread claim test is implemented as a concurrent service-level CAS behavior test. A Docker MySQL restored-dump run was not performed in this environment because no local restored `MYSQL_DSN` was provided during this turn.

