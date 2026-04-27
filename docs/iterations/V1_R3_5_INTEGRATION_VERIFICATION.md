# V1 R3.5 Integration Verification

> Date: 2026-04-24
> Scope: R3.5 DSN guard, jst_ecs test DB restore, R2 forward/backfill on `jst_erp_r3_test`, real MySQL CAS and six R3 integration assertions.

## DSN Guard Evidence

- `go test ./cmd/tools/internal/v1migrate/... -run TestGuardR35DSN -v`: PASS.
- Production DSN attack through `migrate_v1_forward --r35-mode=true`: blocked before connection open.

Evidence:

```text
[R2-FORWARD] abort: R3.5 safety violation: DSN database "jst_erp" must end with '_r3_test'
exit=4
```

## Test DB Setup

Test database: `jst_erp_r3_test`.

Pre-test backup:

```text
/root/ecommerce_ai/backups/20260424T063242Z_r35_pre_test.sql.gz
```

Restored snapshot row counts:

| Table | Rows |
| --- | ---: |
| tasks | 98 |
| task_details | 98 |
| task_assets | 265 |
| asset_storage_refs | 451 |
| users | 95 |
| customization_jobs | 9 |

## R2 Forward on Test DB

`migrate_v1_forward --r35-mode=true` applied 059 through 068 on `jst_erp_r3_test`.

R2 tables present:

```text
notifications
org_move_requests
reference_file_refs
task_customization_orders
task_drafts
task_module_events
task_modules
```

## R2 Backfill on Test DB

Backfill finished with zero warnings and zero errors.

| Phase | processed | generated | warnings | errors | duration |
| --- | ---: | ---: | ---: | ---: | ---: |
| A | 98 | 298 | 0 | 0 | 2.105s |
| B | 265 | 267 | 0 | 0 | 1.029s |
| C | 154 | 194 | 0 | 0 | 774ms |
| D | 0 | 0 | 0 | 0 | 0s |
| E | 0 | 0 | 0 | 0 | 0s |

Final R2 row counts on test DB:

| Table | Rows |
| --- | ---: |
| task_modules | 300 |
| task_module_events | 300 |
| reference_file_refs | 194 |
| task_customization_orders | 0 |

`task_assets.source_module_key` distribution:

| source_module_key | Rows |
| --- | ---: |
| basic_info | 9 |
| customization | 5 |
| design | 251 |

## CAS 100-Thread Real MySQL

Command:

```bash
MYSQL_DSN=<jst_erp_r3_test> R35_MODE=1 go test ./service/task_pool/... -tags=integration -run TestClaimCAS_100Concurrent_MySQL -v -count=1
```

Result: PASS.

Evidence:

```text
=== RUN   TestClaimCAS_100Concurrent_MySQL
--- PASS: TestClaimCAS_100Concurrent_MySQL (4.69s)
PASS
```

The test asserts:

- `success=1`
- `conflict=99`
- final module `state='in_progress'`
- exactly one `claimed` event

## 6 Integration Assertions

Command:

```bash
MYSQL_DSN=<jst_erp_r3_test> R35_MODE=1 go test ./... -tags=integration -run "Integration|TestClaimCAS_TwoClaims_MySQL" -v -count=1
```

Result: PASS.

| Assertion | Test | Result |
| --- | --- | --- |
| Pool excludes `backfill_placeholder=true` | `TestPoolQueryFiltersBackfillPlaceholderIntegration` | PASS |
| Same module double claim returns conflict | `TestClaimCAS_TwoClaims_MySQLIntegration` | PASS |
| `audit.approve` closes audit and enters warehouse as `pending_claim` | `TestAuditApproveEntersWarehouseIntegration` | PASS |
| `user_cancel` sets task `Cancelled` and writes `task_cancelled` | `TestUserCancelUpdatesTaskAndEventsIntegration` | PASS |
| Detail modules match blueprint and all visibility is `visible` | `TestTaskDetailModulesVisibleIntegration` | PASS |
| List priority ordering places `high` before `low` | `TestTaskListPriorityOrderingIntegration` | PASS |

## OpenAPI Conformance

Command:

```bash
go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
```

Result:

```text
openapi validate: 0 error 0 warning
```

## Production Probe

Read-only probe against production R2 target tables after R3.5:

| Table | Rows |
| --- | ---: |
| task_modules | 300 |
| task_module_events | 300 |
| reference_file_refs | 194 |
| task_drafts | 0 |
| notifications | 0 |
| org_move_requests | 0 |
| task_customization_orders | 0 |

These match the R2 post-backfill baseline. No R3.5 write landed on production `jst_erp`.

## Fixes Found During Verification

Real MySQL verification exposed issues that the previous stub test could not catch:

- `ClaimService` pre-authorized claim against mutable module state, causing concurrent losers to return non-conflict denials. Claim now validates pool membership then lets CAS return `module_claim_conflict`.
- `ClaimService` performed a non-transactional module read from inside the claim transaction, which could exhaust the connection pool under 100-way concurrency. The event now uses the already loaded module ID.
- Pool query selected non-existent `task_details.product_name_snapshot`; it now uses `tasks.product_name_snapshot`.
- `audit.approve` now enters warehouse as `pending_claim` for the R3.5 contract.
- Task list default ordering now uses `FIELD(priority, 'critical', 'high', 'normal', 'low') ASC, created_at ASC`.

## Cleanup Instructions

Do not run this automatically. When R4 no longer needs the fixture DB:

```sql
DROP DATABASE IF EXISTS `jst_erp_r3_test`;
```

## Production Touch Statement

All R3.5 writes were directed to `jst_erp_r3_test`. Production `jst_erp` was used only for a final read-only R2 target-table probe. The intentional production DSN attack was blocked before opening a database connection with exit code 4.
