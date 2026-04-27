# V1.3-A1.1 Priority Drift Fix Report

Date: 2026-04-27 PT

Verdict: fixed pending architect verify. This report does not self-sign PASS.

## Root Cause

Production trace `83ba7d26-385b-4bea-99b7-db0925be2975` failed in `POST /v1/tasks` with:

```text
create_task_tx_failed err=create task: insert task: Error 3819 (HY000): Check constraint 'chk_tasks_priority_v1' is violated.
```

The request sent `priority: "urgent"`. Go enum/service validation/OpenAPI/frontend docs allowed `urgent`, while production DB CHECK `chk_tasks_priority_v1` allows only `low`, `normal`, `high`, `critical`. The fix follows option B: DB/migrations remain authoritative and unchanged; code and contracts drop `urgent`.

## Fix Points

| Phase | Scope | Result |
| --- | --- | --- |
| P0 | SHA baseline | `tmp/v1_3_a1_1_baseline_sha.log` recorded. `transport/http.go` equals expected SHA `9a6d194b54aa8d49dbff3d10f6d91283e07d68f21e117d2ffb9c2f99a72eb396`. Protected SoT/migration/tool anchors unchanged at end. |
| P1 | Go enum | Removed `TaskPriorityUrgent`; removed remaining service reference so compile surface stays 4-value. |
| P2 | Backend hardening | `mapTaskCreateTxError` maps MySQL 3819 to `INVALID_REQUEST`; handler validates priority before service call and rejects invalid values with `task_priority_invalid`. |
| P3 | OpenAPI | Priority enums/descriptions now list only `low`, `normal`, `high`, `critical`; `openapi-validate` returned `0 error 0 warning`. |
| P4 | Frontend docs | Regenerated via `scripts/docs/generate_frontend_docs.py`; `docs/frontend/V1_API_TASKS.md` now shows `enum(low/normal/high/critical)`. |
| P5 | Governance | ROADMAP v57 and RETRO debt/status updated; audit regenerated to `tmp/v1_3_a1_1_audit.{json,md}` with `summary.drift == 0`. |

## Regression Coverage

- `service/task_service_priority_test.go`: MySQL 3819 maps to `ErrCodeInvalidRequest` and surfaces the constraint message without SQL/stack.
- `transport/handler/task_priority_validate_test.go`: accepts `""`, `low`, `normal`, `high`, `critical`; rejects `urgent`, `random`, `LOW`.

## Validation

| Command | Result |
| --- | --- |
| `go vet ./...` | PASS |
| `go build ./...` | PASS |
| `go test ./domain/... -count=1` | PASS |
| `go test ./service/... -count=1` | PASS |
| `go test ./transport/handler/... -count=1` | PASS |
| `go test ./tools/contract_audit/... -count=1` | PASS |
| `go test ./... -count=1` | PASS |
| `go run ./cmd/tools/openapi-validate docs/api/openapi.yaml` | PASS, `0 error 0 warning` |
| `go run ./tools/contract_audit ... --fail-on-drift true` | PASS, exit 0 |

Go commands were run with Windows Go at `/mnt/c/Program Files/Go/bin/go.exe` because `go` was not on the WSL shell `PATH`.

## Audit Regression

`tmp/v1_3_a1_1_audit.json` summary:

```json
{
  "total_paths": 233,
  "clean": 179,
  "drift": 0,
  "unmapped": 0,
  "known_gap": 54,
  "missing_in_openapi": 0,
  "missing_in_code": 0
}
```

Known caveat: current contract audit still compares field names, not enum value sets. Q-V1.3-T4 is registered in RETRO for enum-value drift detection across OpenAPI, Go const blocks, and DB CHECK constraints.

## SHA Verify

| File | Final SHA |
| --- | --- |
| `transport/http.go` | `9a6d194b54aa8d49dbff3d10f6d91283e07d68f21e117d2ffb9c2f99a72eb396` |
| `docs/V1_MODULE_ARCHITECTURE.md` | `08ed1b849fe3ca4ed8dc4fda9fffe8396500e2a1c0bf5ee00986ce66ae2fc51a` |
| `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` | `0fdc6a9b786e495a786017d148cdbda1dd8f8188e6c2754d4ffd038b40558ee5` |
| `db/migrations/067_v1_0_tasks_priority_constraint.sql` | `ed68e2161ba4ece3bd56228e6a7c368a3275816069f1aafd2c1cc1e0ca4d7224` |
| `cmd/tools/migrate_v1_forward/main.go` | `ffabcef629cf1cf1d8512f09a37425736472aa0c78cf570701df9a5100e6aea8` |
| `cmd/tools/migrate_v1_backfill/phases.go` | `ea5588560bb5a8a5eb883cfe2c2f28d7d8314f384457bd467d3823f595df59e3` |

## Pending Verify

Waiting for architect verify. No PASS is self-signed here.
