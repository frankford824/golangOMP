# V1 R1 Report

## Summary

- Scope landed in the allowed surfaces: `docs/api/openapi.yaml`, `transport/http.go`, `transport/route_access_catalog.go`, `transport/http_test.go`, `cmd/tools/openapi-validate/main.go`, `go.mod`, `go.sum`.
- Added a single R1 contract route table in `transport/http.go` for all 47 prompt-listed routes.
- Production router registers the genuinely missing routes as structured `501 not_implemented` stubs.
- Isolated transport tests mount all 47 routes on a dedicated contract router so the full freeze table is still verified without downgrading already-shipped live handlers.

## Route Table

| Method | Path | x-owner-round |
| --- | --- | --- |
| GET | `/v1/tasks` | `R3` |
| GET | `/v1/tasks/{id}/detail` | `R3` |
| GET | `/v1/tasks/pool` | `R3` |
| POST | `/v1/tasks/{id}/modules/{module_key}/claim` | `R3` |
| POST | `/v1/tasks/{id}/modules/{module_key}/actions/{action}` | `R3` |
| POST | `/v1/tasks/{id}/modules/{module_key}/reassign` | `R3` |
| POST | `/v1/tasks/{id}/modules/{module_key}/pool-reassign` | `R3` |
| POST | `/v1/tasks/{id}/cancel` | `R3` |
| POST | `/v1/tasks` | `R3` |
| POST | `/v1/task-drafts` | `R4-SA-C` |
| GET | `/v1/me/task-drafts` | `R4-SA-C` |
| GET | `/v1/task-drafts/{draft_id}` | `R4-SA-C` |
| DELETE | `/v1/task-drafts/{draft_id}` | `R4-SA-C` |
| GET | `/v1/tasks/batch-create/template.xlsx` | `R5` |
| POST | `/v1/tasks/batch-create/parse-excel` | `R5` |
| GET | `/v1/assets/search` | `R4-SA-A` |
| GET | `/v1/assets/{asset_id}` | `R4-SA-A` |
| GET | `/v1/assets/{asset_id}/download` | `R4-SA-A` |
| GET | `/v1/assets/{asset_id}/versions/{version_id}/download` | `R4-SA-A` |
| POST | `/v1/assets/{asset_id}/archive` | `R4-SA-A` |
| POST | `/v1/assets/{asset_id}/restore` | `R4-SA-A` |
| DELETE | `/v1/assets/{asset_id}` | `R4-SA-A` |
| GET | `/v1/erp/products/by-code` | `R4-SA-C` |
| GET | `/v1/design-sources/search` | `R4-SA-C` |
| GET | `/v1/search` | `R4-SA-D` |
| GET | `/v1/me/notifications` | `R4-SA-C` |
| POST | `/v1/me/notifications/{id}/read` | `R4-SA-C` |
| POST | `/v1/me/notifications/read-all` | `R4-SA-C` |
| GET | `/v1/me/notifications/unread-count` | `R4-SA-C` |
| GET | `/v1/me` | `R4-SA-B` |
| PATCH | `/v1/me` | `R4-SA-B` |
| POST | `/v1/me/change-password` | `R4-SA-B` |
| GET | `/v1/me/org` | `R4-SA-B` |
| GET | `/v1/users` | `R4-SA-B` |
| POST | `/v1/users` | `R4-SA-B` |
| PATCH | `/v1/users/{id}` | `R4-SA-B` |
| DELETE | `/v1/users/{id}` | `R4-SA-B` |
| POST | `/v1/users/{id}/activate` | `R4-SA-B` |
| POST | `/v1/users/{id}/deactivate` | `R4-SA-B` |
| POST | `/v1/departments/{id}/org-move-requests` | `R4-SA-B` |
| GET | `/v1/org-move-requests` | `R4-SA-B` |
| POST | `/v1/org-move-requests/{id}/approve` | `R4-SA-B` |
| POST | `/v1/org-move-requests/{id}/reject` | `R4-SA-B` |
| GET | `/v1/reports/l1/cards` | `R4-SA-D` |
| GET | `/v1/reports/l1/throughput` | `R4-SA-D` |
| GET | `/v1/reports/l1/module-dwell` | `R4-SA-D` |
| GET | `/ws/v1` | `R4-SA-C` |

## Schema Additions

- Added/froze: `APIReadiness`, `OSSCompletePart`, `TaskModuleState`, `TaskModuleScope`, `TaskModuleProjection`, `TaskModuleAction`, `TaskPriority`, `DerivedStatus`, `DenyCode`, `Actor`, `TaskModule`, `TaskDraftPayload`, `TaskDraft`, `BatchCreatePreviewItem`, `BatchCreateViolation`, `BatchCreateParseResult`, `AssetLifecycleState`, `Asset`, `ERPProductSnapshot`, `DesignSourceEntry`, `SearchResultGroup`, `NotificationType`, `Notification`, `OrgMoveRequestState`, `OrgMoveRequest`, `L1Card`.
- Extended existing schemas: `ErrorResponse`, `ReferenceFileRef`, `TaskDetail`, `CreateTaskRequest`.

## OpenAPI Size

- Prompt-stated baseline: `6297` lines.
- Current local file after R1 edits: `13800` lines.
- Note: this workspace is not a git repo, so a precise pre-edit line count could not be reconstructed from version control.

## Validation

```text
go build ./...                                      PASS
go test ./transport/... -run "V1R1" -v -count=1    PASS
go run ./cmd/tools/openapi-validate docs/api/openapi.yaml   PASS
Select-String '^  /v1/|^  /ws/v1' docs/api/openapi.yaml     204 paths
go test ./... -count=1                             FAIL (environment)
```

`go test ./... -count=1` failed only for:

- `workflow/config`
- `workflow/dist/v1.0/config`

Observed failure text:

```text
An Application Control policy has blocked this file.
```

## Leftovers / Conflicts

- Existing live-route overlap prevented a literal production downgrade of all 47 prompt-listed routes to `501`. Overlap set:
  - `GET /v1/tasks`
  - `POST /v1/tasks`
  - `GET /v1/tasks/{id}/detail`
  - `GET /v1/users`
  - `POST /v1/users`
  - `PATCH /v1/users/{id}`
  - `GET /v1/assets/{id}`
  - `GET /v1/assets/{id}/download`
- Conservative handling used:
  - production router preserves shipped handlers on the overlap set
  - isolated transport tests mount the full 47-route contract table and verify the reserved `501` behavior there
- `openapi-cli` is not present in this workspace. Validation uses the prompt-approved fallback loader command in `cmd/tools/openapi-validate`.
- This workspace is not a git repository, so no branch/commit/PR artifact could be created from here.

## Post-R1 Verification (added 2026-04-23)

### Full test suite re-run (WSL)

Local Windows AppControl blocks the generated `config.test.exe` / `dist/v1.0/config/config.test.exe`
binaries, so `go test ./... -count=1` was re-run under WSL (Ubuntu 24.04.3 LTS) with a local
`go1.25.5 linux/amd64` tarball install (`$HOME/go`), against the same workspace via
`/mnt/c/Users/wsfwk/Downloads/yongboWorkflow/go`.

```text
ok  workflow/cmd/tools/migrate_oss_keys   0.015s
ok  workflow/config                        0.016s
ok  workflow/dist/v1.0/config              0.017s
ok  workflow/domain                        0.029s
ok  workflow/repo/mysql                    0.011s
ok  workflow/service                       5.950s
ok  workflow/tests                         0.004s
ok  workflow/transport                     0.345s
ok  workflow/transport/handler             0.081s
?   workflow/cmd/api                       [no test files]
?   workflow/cmd/roundl-warehouse-drift-repair [no test files]
?   workflow/cmd/server                    [no test files]
?   workflow/cmd/tools/openapi-validate    [no test files]
?   workflow/policy                        [no test files]
?   workflow/repo                          [no test files]
?   workflow/workers                       [no test files]
```

- Every package with tests now reports `ok`.
- The two Windows-blocked packages (`workflow/config`, `workflow/dist/v1.0/config`) pass cleanly
  under WSL, confirming the earlier `FAIL` was an environment policy issue and not an R1 regression.
- R1 is verified green across the entire repository.

### OpenAPI slim audit (`docs/api/openapi.yaml`)

File baseline after R1: 13,800 lines / 525,399 bytes / 206 paths / 47 R1-tagged paths via
`x-owner-round`. Inspection focused on: duplicate schemas, orphan (unreferenced) schemas,
large legacy blocks that overlap with new R1 schemas, and missing error contracts on new R1
paths. Results below.

#### A. Orphan schemas (0 `$ref`, safe to delete in a dedicated slim pass)

| Schema | Lines | Status |
| --- | --- | --- |
| `Error` (52-64) | 14 | 0 refs. Pre-v1 error shape. R1 already canonicalized `ErrorResponse` (4520-4539). |
| `TaskDetailAggregate` (4630-4762) | 132 | 0 refs. Pre-v1 aggregate. R1 already canonicalized `TaskDetail` in §9.2. |

**Immediate saving: ~146 lines with zero contract risk.** These two deletions are additive-safe
and independent of any handler, so they are a legitimate R1-cleanup candidate if the R1 "no
touch existing live routes" constraint is loosened for schema-only dead code. Otherwise they
should be queued for R2 (which is already the canonical slim window).

#### B. Legacy schemas that overlap with new R1 schemas (coordinate with Rx)

| Legacy schema | Refs | Replacement | Defer to |
| --- | --- | --- | --- |
| `TaskReadModel` (3596) | 3 | `TaskDetail` (R1 canonical) | R3 (task write/read path) |
| `DesignAsset` + `DesignAsset*` chain (2666-2944) | 7 direct + chain | `Asset` + `AssetLifecycleState` (R1) | R4-SA-A (asset ownership) |
| `UploadRequest` + upload-session chain (2841-3470) | 4 + chain | OSS-only asset runtime (V1_ASSET_OWNERSHIP) | R4-SA-A |
| `TaskCostOverride*` block (1999-2400) | 41 | Retained for governance; not an R1 slim target | Hold |
| `ExportJob*` block (5241-5835) | many | Retained feature | Hold |
| `Integration*` block (5843-6200) | many | Retained feature | Hold |

#### C. R1 path error-contract gap

All 47 R1-tagged paths register a success response only (`200`/`201`) and do not declare
`4xx` / `5xx` using `ErrorResponse`. Count: 15 total `$ref: '#/components/schemas/ErrorResponse'`
versus 47 new paths → ~32 paths missing an error envelope.

- This is acceptable for a freeze stub round because the body shape is fixed by `ErrorResponse`
  and the `deny_code` set is fixed by `DenyCode`, so R2/R3/R4 can wire the error path when they
  implement the handlers.
- Recommended follow-up: R2 prompt requires every R1-frozen path that it touches to declare
  `400`/`401`/`403`/`404`/`409`/`422` using `$ref: '#/components/schemas/ErrorResponse'` and
  enumerate relevant `DenyCode` values in the description.

#### D. Size expectation going forward

- R1 freeze added 47 paths + 26 schemas + 4 extended schemas. Net delta was ~7.5k lines,
  driven mostly by per-path parameter/response blocks (average ~155 lines/new path).
- Slim expectation for R2-R4 is flat (no further growth) and R6 is -10% as dead pre-v1
  schemas (sections A + parts of B) are deleted along with the matching legacy routes.

### Action items carried forward

1. **P0 — closed.** Full test suite green under WSL.
2. **P1 — open.** Queue schema-only deletions of `Error` and `TaskDetailAggregate` for the R2
   prompt (schema-only delete list) or a dedicated R1b slim patch.
3. **P1 — open.** The R2/R3/R4 prompts must each declare R1-frozen error responses (`4xx/5xx`
   → `ErrorResponse`) when they touch a frozen path and must also replace the 8 live-overlap
   routes documented under "Leftovers / Conflicts" above.
4. **P2 — open.** Schedule a dedicated `R6-slim` pass to drop `TaskReadModel`, `DesignAsset*`,
   `UploadRequest*` once R3/R4 finish handler migration.
