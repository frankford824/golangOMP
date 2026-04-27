# V1 · R4-SA-A · Patch-A3(NotFound 错误码映射 · DRIFT-RUNTIME-2 修复)

> 发布:2026-04-25
> 触发:Patch-A2 PASS-WITH-FOLLOWUP · live smoke 暴露 SA-A v1.0 旧 bug DRIFT-RUNTIME-2
> 性质:**最小补丁轮 · 1 行 errors.Is 修复 + 1 行 import + 1 条 service 单测防退化 + 1 条 handler smoke 集成测**
> 上游签字依赖:R4-SA-A v1.0 + Patch-A2 已签字
> 产出:补丁 + `docs/iterations/V1_R4_SA_A_PATCH_A3_REPORT.md`

---

## 0. 角色与目标

`GET /v1/assets/{nonexistent_id}` 返 500 而非 404。根因已定位为 `repo/mysql/task_asset_search_repo.go::scanTaskAssetSearchRow` 用 `==` 比较被 wrap 的 `sql.ErrNoRows`,导致 NotFound 漏接 → 走 `INTERNAL_ERROR` 500。本补丁**单一目标**:修这一处 errors.Is + 加 2 条防退化测试。

**严禁触达**(白名单之外一律 ABORT):

- ❌ 任何 `service/asset_center/**` 业务逻辑(本补丁不动 service)
- ❌ 任何 `service/asset_lifecycle/**` 业务逻辑
- ❌ 任何 `transport/**`(handler 已在 Patch-A2 修好 · 不动)
- ❌ 任何 `migrations/**`
- ❌ 任何 `docs/api/openapi.yaml`
- ❌ 任何 R4 子轮 / Patch-A2 报告
- ❌ 任何 `domain/**` 错误码定义

**允许写入**:

- ✅ `repo/mysql/task_asset_search_repo.go`(1 行 `==` → `errors.Is` + 1 行 `import "errors"`)
- ✅ `service/asset_center/detail_test.go`(新建 · 1 条 unit test)
- ✅ `service/asset_center/detail_integration_test.go` 或 handler 层(新建 · 1 条 SAAI smoke 集成测;名 `TestSAAI_GetGlobalAsset_NotFound_Returns404`)
- ✅ `docs/iterations/V1_R4_SA_A_PATCH_A3_REPORT.md`(新建)
- ✅ `tmp/r4_sa_a_patch_a3_*.{sh,log,txt,body,pid}`(过程产物)
- ✅ 测试库 `jst_erp_r3_test`(`t.Cleanup` 清完即可)

---

## 1. 必读输入(各读 1 次)

1. `docs/iterations/V1_R4_SA_A_PATCH_A2_REPORT.md` §11(架构师裁决 + DRIFT-RUNTIME-2 描述)
2. `repo/mysql/task_asset_search_repo.go` 第 130–185 行(`scanTaskAssetSearchRow` + `scanTaskAssetSearchScanner`)
3. `service/asset_center/detail.go` 全文(GetDetail 调用链)
4. `service/asset_center/download_test.go` 头 60 行(已有 `fakeSearchRepo` 模式 · 复用)
5. `transport/handler/task_asset_center.go` 第 730–750 行(`respondAssetCenterError` · 仅作只读对照)

**禁止读**:`docs/archive/*` / 任何 R0/R1/R2 老 prompt / `service/asset_lifecycle/**`(范围外)。

---

## 2. 故障根因(架构师已抓 · 你不必重发现)

**响应体证据**:

```
HTTP/1.1 500 Internal Server Error
{"error":{"code":"INTERNAL_ERROR","message":"scan task asset search row: sql: no rows in result set",...}}
```

**根因链**:

1. `repo/mysql/task_asset_search_repo.go::scanTaskAssetSearchScanner`(line 156–183):
   ```go
   if err := s.Scan(...); err != nil {
       return nil, fmt.Errorf("scan task asset search row: %w", err)  // ← 把 sql.ErrNoRows wrap 了
   }
   ```
2. `repo/mysql/task_asset_search_repo.go::scanTaskAssetSearchRow`(line 144–150):
   ```go
   func scanTaskAssetSearchRow(row *sql.Row) (*repo.TaskAssetSearchRow, error) {
       item, err := scanTaskAssetSearchScanner(row)
       if err == sql.ErrNoRows {  // ← 比较失败,因为 err 已被 fmt.Errorf %w wrap
           return nil, nil
       }
       return item, err  // ← 返了 wrapped sql.ErrNoRows 给 service
   }
   ```
3. `service/asset_center/detail.go::GetDetail`(line 14–20):
   ```go
   current, err := s.searchRepo.GetCurrentByAssetID(ctx, assetID)
   if err != nil {
       return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)  // ← 走 500
   }
   if current == nil || current.Asset == nil || current.Asset.DeletedAt != nil {
       return nil, domain.ErrNotFound  // ← 期望走这里 · 应映射 404
   }
   ```

**4 个 SA-A 触点共享这条 helper**(由 `GetCurrentByAssetID` 路径):

- `GET /v1/assets/{asset_id}` → `globalSvc.GetDetail`
- `GET /v1/assets/{asset_id}/download` → `globalSvc.DownloadLatest`(同样调 `GetCurrentByAssetID`)
- `GET /v1/assets/{asset_id}/versions/{version_id}/download` → `globalSvc.DownloadVersion`(待审,可能也走 `GetVersion`)
- `GET /v1/assets/{asset_id}/preview` → R3 老路径(待审)

**1 行 errors.Is 修复后 · GetCurrentByAssetID 这条 helper 立刻归位 · 上述 SA-A 触点的 NotFound 全部正确返 404**。

---

## 3. 修复方案(架构师定 · 不许 codex 自由扩范围)

### 3.1 repo/mysql/task_asset_search_repo.go(2 处微改)

**修复 A · scanTaskAssetSearchRow 改用 errors.Is**:

```diff
+import "errors"  // 在文件顶部 import 块加入(若已有则跳过)

 func scanTaskAssetSearchRow(row *sql.Row) (*repo.TaskAssetSearchRow, error) {
     item, err := scanTaskAssetSearchScanner(row)
-    if err == sql.ErrNoRows {
+    if errors.Is(err, sql.ErrNoRows) {
         return nil, nil
     }
     return item, err
 }
```

**注意**:
- 如果 `import` 块已经有 `"errors"`,**不再重复 import**
- 不要动 `scanTaskAssetSearchScanner`(wrap 行为本身没错 · 调用方修就够)
- 不要动 `scanTaskAssetSearchRows`(已用 rows.Next() 模式 · 不受此 bug 影响)

### 3.2 service/asset_center/detail_test.go(新建 · 1 条 unit test)

加 1 个 unit test 用 `fakeSearchRepo` 模式(参考 `download_test.go` line 50–66 的 fakeSearchRepo)。新 test 名:**`TestGetDetail_NotFound_ReturnsErrNotFound`**。

测试用例:
- 当 fake repo 的 `GetCurrentByAssetID` 返 `(nil, nil)` 时(模拟修复后的 NotFound 行为)
- 调用 `GetDetail(ctx, 999999999)` 应返 `(nil, domain.ErrNotFound)`
- 断言 `appErr.Code == domain.ErrCodeNotFound`

### 3.3 集成 smoke 测(新建 · 1 条)

加 1 条 SAAI 命名集成测 `TestSAAI_GetGlobalAsset_NotFound_Returns404`,放在与 SA-A 已有 SAAI 集成测同包下(若已有 SAAI 集成测包,加进去;否则新建文件 `service/asset_center/integration_notfound_test.go` 带 `//go:build integration` tag)。

测试用例:
- 用真实 `MYSQL_DSN` 起 service + repo(参考已有 SAAI 集成测样板)
- `GetDetail(ctx, 999999999)` 应返 `(nil, domain.ErrNotFound)`(repo 真查 · 走 sql.ErrNoRows · 经 errors.Is 修复后正确归位)
- 断言 `appErr.Code == "NOT_FOUND"`(或 `domain.ErrCodeNotFound` 字面值)

**绝不**调起 `cmd/server` 也**绝不**做端到端 HTTP smoke(那是 retro Step D 的职责);这条集成测只验 service + repo 这一层。

### 3.4 OpenAPI 不动

`docs/api/openapi.yaml` 完全不改。

---

## 4. 硬约束(命中即 ABORT)

- 修改 §3 白名单之外的任何文件 → ABORT
- 修改 OpenAPI 任意行 → ABORT
- 改 `service/asset_center/global_service.go` / `detail.go`(只许新建 detail_test.go · 不改原 detail.go)→ ABORT
- 改 `transport/handler/task_asset_center.go` 任何行 → ABORT
- 改 `domain/errors.go` 任何行 → ABORT
- 加额外修复(如改 ArchiveGlobalAsset / RestoreGlobalAsset 错误码 · 改其他 helper)→ ABORT
- 测试用绕开 errors.Is 直接对比 wrapped string → ABORT
- 测试库残留 `id ∈ [50000, 60000)` 未清(9 张表任 1 张 > 0)→ ABORT
- DSN guard 失效(连到生产)→ ABORT

---

## 5. 验证清单(全跑 · 任一 fail 即 ABORT)

```bash
cd /mnt/c/Users/wsfwk/Downloads/yongboWorkflow/go
DSN="$(ssh jst_ecs 'bash /root/ecommerce_ai/r3_5/build_test_dsn.sh')"
export MYSQL_DSN="$DSN"
export R35_MODE=1
export GOPATH=$HOME/.cache/go-path && mkdir -p $GOPATH

# A) build 双 tag 绿
/home/wsfwk/go/bin/go build ./... 2>&1                                        | tee tmp/r4_sa_a_patch_a3_build_default.log
/home/wsfwk/go/bin/go build -tags=integration ./... 2>&1                      | tee tmp/r4_sa_a_patch_a3_build_integration.log

# B) 新增 unit test 单跑(必跑)
/home/wsfwk/go/bin/go test -count=1 -run 'TestGetDetail_NotFound_ReturnsErrNotFound' \
  ./service/asset_center/... 2>&1 | tee tmp/r4_sa_a_patch_a3_unit_target.log

# C) 新增 SAAI 集成测单跑(必跑)
/home/wsfwk/go/bin/go test -tags=integration -count=1 -run 'TestSAAI_GetGlobalAsset_NotFound_Returns404' \
  ./service/asset_center/... 2>&1 | tee tmp/r4_sa_a_patch_a3_integration_target.log

# D) cmd/server 启动 smoke(确认 Patch-A2 没回退)
setsid bash -c '/home/wsfwk/go/bin/go run ./cmd/server' \
  >/tmp/r4_sa_a_patch_a3_server.log 2>&1 < /dev/null &
echo $! > tmp/r4_sa_a_patch_a3_server.pid
sleep 8
HEALTH=$(curl -o /dev/null -s -w '%{http_code}' http://127.0.0.1:8080/healthz)
echo "healthz=$HEALTH" | tee tmp/r4_sa_a_patch_a3_healthz.log
test "$HEALTH" = "200" || { echo SERVER_BAD; tail -50 /tmp/r4_sa_a_patch_a3_server.log; exit 1; }

# E) live smoke 重打 GET /v1/assets/999999999 期望 404(本轮修复目标)
# 用 Patch-A2 同款身份注入(users 49091 + r4-sa-a-patch-a3-emp-token);
# 抓响应 status + body。期望 status=404 · code="NOT_FOUND"。
# 落 tmp/r4_sa_a_patch_a3_smoke_notfound.json/.txt。

# F) 关后端
kill -9 $(cat tmp/r4_sa_a_patch_a3_server.pid) 2>/dev/null || true

# G) 联合 integration 4 域 + R3 不破坏
/home/wsfwk/go/bin/go test -tags=integration -count=1 -timeout 30m \
  -run 'SAAI|SABI|SACI|SADI|TestModuleAction|TestTaskPool|TestTaskCancel|TestTaskAggregator' \
  ./service/... ./transport/handler/... ./transport/ws/... 2>&1 \
  | tee tmp/r4_sa_a_patch_a3_integration_full.log

# H) 全 unit test
/home/wsfwk/go/bin/go test ./... -count=1 2>&1 \
  | tee tmp/r4_sa_a_patch_a3_unit.log

# I) openapi-validate 必 0/0
/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml 2>&1 \
  | tee tmp/r4_sa_a_patch_a3_openapi_validate.log

# J) 测试库 [50000,60000) 9 表残留 0(标准隔离审计)
# 复用 retro/Patch-A2 同款脚本

# K) 修改文件清单审计:仅 4 个文件被改/新建
git status --porcelain 2>&1 | tee tmp/r4_sa_a_patch_a3_files_changed.log
```

**`E` 是核心通过条件**:`GET /v1/assets/999999999` 必须返 **404 + NOT_FOUND**。

---

## 6. 报告输出(新建 `docs/iterations/V1_R4_SA_A_PATCH_A3_REPORT.md`)

章节:

1. `## Scope` — 触发(DRIFT-RUNTIME-2)+ 修复方向 + 范围
2. `## §3.1 修复 A:errors.Is 归位` — 2 行 diff(import + 比较)
3. `## §3.2 unit test 防退化` — `TestGetDetail_NotFound_ReturnsErrNotFound` 全文
4. `## §3.3 SAAI 集成 smoke 防退化` — `TestSAAI_GetGlobalAsset_NotFound_Returns404` 全文
5. `## §4 OpenAPI 影响评估` — 不动 + openapi-validate 0/0
6. `## §5 验证证据` — A~K 11 步逐个 PASS / 关键末尾输出
7. `## §6 cmd/server 启动证据` — `/healthz` 200 + 无 panic
8. `## §7 live smoke 修复前后对比` — 修复前(Patch-A2 报告 §7 已记录)500 + 修复后 404
9. `## §8 联合 integration + 全 unit + 文件改动审计`
10. `## §9 数据隔离审计` — 9 表残留 0
11. `## §10 已知非目标` — Archive/Restore/Delete 错误码映射不在本轮(若有问题留 DRIFT-RUNTIME-3 跟踪)
12. `## §11 sign-off candidate` — 声明可签字

---

## 7. 工作流程

1. 读 §1 必读
2. 直接按 §3 三处定点修改 + 2 条新测
3. 跑 §5 验证 A→K 11 步
4. 写报告
5. 任何步骤失败 → 停 · 按 §4 ABORT 规则记录原因

## 8. 最终回报内容

- 文件改动清单(4 个 · 详 §3)
- A~K 11 步验证结果(每步 PASS / FAIL)
- E 步 `GET /v1/assets/999999999` 实测 status + code(必须 404 + NOT_FOUND)
- B/C 新增测试是否通过
- G 联合 integration 是否仍全绿
- I openapi-validate 是否 0/0
- 是否有 ABORT(若有 · 哪一条)

完成后简洁回报 · 然后退出。
