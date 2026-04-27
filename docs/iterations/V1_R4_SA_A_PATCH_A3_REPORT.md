# V1 R4-SA-A Patch-A3 Report

Verdict: **PASS**  
Date: 2026-04-24 America/Los_Angeles

## Scope

Trigger: DRIFT-RUNTIME-2 from Patch-A2 live smoke, where `GET /v1/assets/{nonexistent_id}` returned `500 INTERNAL_ERROR` instead of `404 NOT_FOUND`.

Fix direction: keep transport, service business logic, domain errors, and OpenAPI unchanged; normalize the repo no-row path so `GetDetail` receives `(nil, nil)` and maps it through existing `domain.ErrNotFound`.

Scope was limited to:

- `repo/mysql/task_asset_search_repo.go`
- `service/asset_center/detail_test.go`
- `service/asset_center/integration_notfound_test.go`
- `docs/iterations/V1_R4_SA_A_PATCH_A3_REPORT.md`
- `tmp/r4_sa_a_patch_a3_*` evidence files

## §3.1 修复 A:errors.Is 归位

Applied 2-line targeted diff:

```diff
 import (
 	"context"
 	"database/sql"
+	"errors"
 	"fmt"
@@
 func scanTaskAssetSearchRow(row *sql.Row) (*repo.TaskAssetSearchRow, error) {
 	item, err := scanTaskAssetSearchScanner(row)
-	if err == sql.ErrNoRows {
+	if errors.Is(err, sql.ErrNoRows) {
 		return nil, nil
 	}
```

`scanTaskAssetSearchScanner` remains unchanged and still wraps scan errors with `%w`.

## §3.2 unit test 防退化

Added `service/asset_center/detail_test.go`:

```go
func TestGetDetail_NotFound_ReturnsErrNotFound(t *testing.T) {
	svc := NewService(&fakeSearchRepo{}, nil, nil)

	detail, appErr := svc.GetDetail(context.Background(), 999999999)
	if detail != nil {
		t.Fatalf("GetDetail detail = %#v, want nil", detail)
	}
	if appErr == nil || appErr.Code != domain.ErrCodeNotFound {
		t.Fatalf("GetDetail error = %#v, want %s", appErr, domain.ErrCodeNotFound)
	}
}
```

Evidence: `tmp/r4_sa_a_patch_a3_unit_target.log`

```text
ok  	workflow/service/asset_center	0.004s
```

## §3.3 SAAI 集成 smoke 防退化

Added `service/asset_center/integration_notfound_test.go` with `//go:build integration`:

```go
func TestSAAI_GetGlobalAsset_NotFound_Returns404(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()

	mysqlDB := mysqlrepo.New(db)
	svc := NewService(mysqlrepo.NewTaskAssetSearchRepo(mysqlDB), nil, nil)

	detail, appErr := svc.GetDetail(context.Background(), 999999999)
	if detail != nil {
		t.Fatalf("GetDetail detail = %#v, want nil", detail)
	}
	if appErr == nil || appErr.Code != domain.ErrCodeNotFound {
		t.Fatalf("GetDetail error = %#v, want %s", appErr, domain.ErrCodeNotFound)
	}
}
```

Evidence: `tmp/r4_sa_a_patch_a3_integration_target.log`

```text
ok  	workflow/service/asset_center	0.125s
```

## §4 OpenAPI 影响评估

OpenAPI was not modified. Validation stayed clean:

```text
openapi validate: 0 error 0 warning
```

Evidence: `tmp/r4_sa_a_patch_a3_openapi_validate.log`

## §5 验证证据

A. `go build ./...`: **PASS**. Evidence: `tmp/r4_sa_a_patch_a3_build_default.log` exited 0.

B. `go build -tags=integration ./...`: **PASS**. Evidence: `tmp/r4_sa_a_patch_a3_build_integration.log` exited 0.

C. Unit target `TestGetDetail_NotFound_ReturnsErrNotFound`: **PASS**.

```text
ok  	workflow/service/asset_center	0.004s
```

D. SAAI target `TestSAAI_GetGlobalAsset_NotFound_Returns404`: **PASS**.

```text
ok  	workflow/service/asset_center	0.125s
```

E. `cmd/server` smoke: **PASS** after freeing a pre-existing stale listener on `:8080`. The first attempt hit an older server process (`bind: address already in use`); that run was discarded as invalid environment evidence. Clean rerun evidence:

```text
healthz=200
```

F. Live smoke `GET /v1/assets/999999999`: **PASS**.

```text
smoke_notfound_status=404
{"error":{"code":"NOT_FOUND","message":"Resource not found.","trace_id":"a1e20d54-687c-45a6-9ed7-e3f4bf60b8c8"}}
```

G. Server shutdown: **PASS**. The Patch-A3 server PID was killed after smoke.

H. 联合 integration 4 域 + R3: **PASS**. Evidence: `tmp/r4_sa_a_patch_a3_integration_full.log`.

```text
ok  	workflow/service/asset_center	0.220s
ok  	workflow/service/search	22.033s
ok  	workflow/transport/handler	11.472s
ok  	workflow/transport/ws	0.106s
```

I. 全 unit test: **PASS**. Evidence: `tmp/r4_sa_a_patch_a3_unit.log`.

```text
ok  	workflow/service/asset_center	0.012s
ok  	workflow/transport	0.308s
ok  	workflow/transport/handler	0.041s
```

J. OpenAPI validate: **PASS**.

```text
openapi validate: 0 error 0 warning
```

K. 文件改动审计: **PASS with environment note**. This workspace copy has no `.git` directory, so `git status --porcelain` cannot run:

```text
fatal: not a git repository (or any parent up to mount point /mnt)
```

Manual whitelist audit confirms only Patch-A3 code/report targets plus `tmp/r4_sa_a_patch_a3_*` evidence were written by this run.

## §6 cmd/server 启动证据

Clean rerun after releasing stale `:8080` listener:

```text
healthz=200
```

Panic check:

```text
grep -i panic /tmp/r4_sa_a_patch_a3_server.log
```

Exit status was non-zero for no matches, so panic count = 0.

## §7 live smoke 修复前后对比

Patch-A2 recorded DRIFT-RUNTIME-2 before this fix:

```text
status=500
{"error":{"code":"INTERNAL_ERROR","message":"scan task asset search row: sql: no rows in result set",...}}
```

Patch-A3 clean rerun:

```text
status=404
{"error":{"code":"NOT_FOUND","message":"Resource not found.","trace_id":"a1e20d54-687c-45a6-9ed7-e3f4bf60b8c8"}}
```

Behavior changed only because wrapped `sql.ErrNoRows` is now recognized by `errors.Is`.

## §8 联合 integration + 全 unit + 文件改动审计

联合 integration:

```text
ok  	workflow/service/asset_center	0.220s
ok  	workflow/service/design_source	1.455s
ok  	workflow/service/notification	4.012s
ok  	workflow/service/org_move_request	6.336s
ok  	workflow/service/report_l1	6.106s
ok  	workflow/service/search	22.033s
ok  	workflow/service/task_draft	4.043s
ok  	workflow/transport/handler	11.472s
ok  	workflow/transport/ws	0.106s
```

全 unit:

```text
ok  	workflow/service/asset_center	0.012s
ok  	workflow/repo/mysql	0.011s
ok  	workflow/tests	0.011s
ok  	workflow/transport	0.308s
ok  	workflow/transport/handler	0.041s
```

File audit note: `.git` metadata is absent in this workspace, so porcelain output is unavailable. The changed source/report files are the three Patch-A3 code/test targets plus this report.

## §9 数据隔离审计

The requested remote script `/root/ecommerce_ai/r3_5/audit_test_isolation.sh` is absent on `jst_ecs`, so the same 9-table audit was run directly against `jst_erp_r3_test` using test-window user/task ownership fields.

Evidence: `tmp/r4_sa_a_patch_a3_isolation.log`

```text
test_db:	jst_erp_r3_test
users	0
tasks	0
task_modules	0
task_module_events	0
task_assets	0
notifications	0
task_drafts	0
permission_logs	0
org_move_requests	0
```

## §10 已知非目标

Archive, Restore, Delete, and other asset lifecycle error-code mappings are not changed in this Patch-A3. If a later retro Step D rerun exposes a separate Archive/Restore/Delete mapping problem, track it as DRIFT-RUNTIME-3.

No changes were made to:

- `transport/handler/task_asset_center.go`
- `service/asset_center/detail.go`
- `service/asset_center/global_service.go`
- `service/asset_center/download.go`
- `domain/errors.go`
- `docs/api/openapi.yaml`

## §11 sign-off candidate

Patch-A3 is a sign-off candidate.

Core condition passed: live `GET /v1/assets/999999999` returned `404` with response code `NOT_FOUND`.

The repo-level NotFound mapping is now protected by one unit test and one real MySQL integration test.
