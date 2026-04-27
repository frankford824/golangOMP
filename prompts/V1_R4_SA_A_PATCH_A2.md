# V1 · R4-SA-A · Patch-A2(wildcard 参数名归一 + reserved 表清残)

> 发布:2026-04-25
> 触发:R4-Retro 联合 live smoke 0/36 · `cmd/server` 启动 panic
> 性质:**最小补丁轮 · 仅修 wildcard 命名分裂 + 清 reserved 表 7 条遗存 · 不改任何业务语义**
> 上游签字依赖:R4-SA-A v1.0 已签字(报告留底);本轮在其基础上做**生产部署修复补丁**
> 产出:补丁 + `docs/iterations/V1_R4_SA_A_PATCH_A2_REPORT.md`

---

## 0. 角色与目标

R4-SA-A 实装时遗留两处 wildcard 命名分裂,导致 `cmd/server` 启动期 Gin router panic。本补丁**单一目标**:修复 panic · 让 `cmd/server` 正常启动 · 让联合 live smoke 可执行。

**严禁触达**(白名单之外一律 ABORT):

- ❌ 任何 `service/**`、`repo/**`、`domain/**` 业务逻辑(本补丁不动业务)
- ❌ 任何 `migrations/**`(无 schema 改动)
- ❌ 任何 `docs/api/openapi.yaml` 的 path / schema 改动(本补丁不改契约)
- ❌ 任何 R4 子轮报告(SA-A v1.0 已签字 · 不可改)

**允许写入**:

- ✅ `transport/http.go`(2 处:GET wildcard 归一 + reserved 表清残)
- ✅ `transport/handler/task_asset_center.go`(4 处:`c.Param("id")` → `c.Param("asset_id")`)
- ✅ `docs/iterations/V1_R4_SA_A_PATCH_A2_REPORT.md`(新建)
- ✅ `tmp/r4_sa_a_patch_a2_*.{sh,log}`(过程产物)
- ✅ 测试库 `jst_erp_r3_test`(`t.Cleanup` 清完即可)

---

## 1. 必读输入(各读 1 次 · 不要反复读)

1. `docs/iterations/V1_R4_RETRO_REPORT.md` §4 + §7(BLOCKER 描述 + DRIFT-RUNTIME-1)
2. `docs/iterations/V1_R4_SA_A_REPORT.md`(SA-A 7 触点 + 设计意图)
3. `transport/http.go` 第 372–394 行(`assetGroup` 真 handler 挂载段)+ 第 540–596 行(`v1R1ReservedHandler` 残留描述符)+ 第 510–536 行(reserved 表如何 mount)
4. `transport/handler/task_asset_center.go` 第 140–340 行(SA-A 4 个 handler 实现)
5. `docs/api/openapi.yaml` 9200–9320 行(R3 接管的 `/v1/assets/{id}` 系) + 14120–14210 行(SA-A 新 `/v1/assets/{asset_id}` 系)— 仅作只读对照 · **不修改**

**禁止读**:`docs/archive/*` / 任何 R0/R1/R2 老 prompt / 任何与 wildcard 命名无关的 SA-B/SA-C/SA-D 代码段。

---

## 2. 故障根因(架构师已定位 · 你不必重新发现)

### 2.1 真 handler 内 GET wildcard 名分裂

`transport/http.go` 第 375–393 行 `assetGroup`(`/v1/assets`)下 GET 同 prefix 用了两个不同 wildcard 名:

```go
assetGroup.GET("/:id", ...)                                        // SA-A GetGlobalAsset · 用 :id
assetGroup.DELETE("/:id", ...)                                     // SA-A DeleteGlobalAsset · 用 :id
assetGroup.GET("/:id/download", ...)                               // SA-A DownloadGlobalAsset · 用 :id
assetGroup.GET("/:asset_id/versions/:version_id/download", ...)    // SA-A DownloadGlobalAssetVersion · 用 :asset_id ← 冲突源
assetGroup.POST("/:asset_id/archive", ...)                         // SA-A ArchiveGlobalAsset · 用 :asset_id (POST 不冲突 GET)
assetGroup.POST("/:asset_id/restore", ...)                         // SA-A RestoreGlobalAsset · 用 :asset_id (POST 不冲突 GET)
assetGroup.GET("/:id/preview", ...)                                // R3 PreviewAssetResource · 用 :id
```

Gin radix tree:同一 method 下,同一 prefix 的第一段 wildcard 必须用同一名字。GET 上 `:id` 和 `:asset_id` 共存 → panic at `transport/http.go:380`。

### 2.2 v1R1ReservedHandler 表残留 7 条 SA-A 描述符

`transport/http.go` 第 568–574 行有 7 条 SA-A 描述符,在第 510–530 行 `mountV1R1ReservedDescriptors` 流程里**会再被** `group.GET/POST/DELETE(spec.RelativePath, ...)` mount 一次,产生 501 占位 handler。但 SA-A 已实装 · 它们应当**从 reserved 表移除**。

这 7 条 reserved 描述符全部用 `:asset_id`,与真 handler GET `/:id` 也冲突;即使 §2.1 修了,它们也会重新引发 panic。

---

## 3. 修复方案(架构师定 · 不许 codex 自由选)

### 3.1 transport/http.go(2 处)

**修复 A · 真 handler GET wildcard 全归一为 `:asset_id`**(与 SA-A 已用的 archive/restore/version 路由 + handler 内变量名 `assetID` 一致):

| 行 | 旧 | 新 |
| --- | --- | --- |
| 377 | `assetGroup.GET("/:id", access(group, http.MethodGet, "/:id", ...), taskAssetCenterH.GetGlobalAsset)` | `assetGroup.GET("/:asset_id", access(group, http.MethodGet, "/:asset_id", ...), taskAssetCenterH.GetGlobalAsset)` |
| 378 | `assetGroup.DELETE("/:id", ...)` | `assetGroup.DELETE("/:asset_id", ...)` |
| 379 | `assetGroup.GET("/:id/download", ...)` | `assetGroup.GET("/:asset_id/download", ...)` |
| 383 | `assetGroup.GET("/:id/preview", ...)` | `assetGroup.GET("/:asset_id/preview", ...)` |

**注**:`access(...)` 第 4 个参数(relativePath 字符串)也必须随之改名,与 router 路径一致。
**注**:第 380/381/382 行(已用 `:asset_id`)**不动**。

**修复 B · 删除 v1R1ReservedHandler 表内 SA-A 7 条遗存**:

`transport/http.go` 第 568–574 行(连续 7 行 · 全部 `OwnerRound: "R4-SA-A"`):

```go
{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/assets/search", OwnerRound: "R4-SA-A", ...},
{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/assets/:asset_id", OwnerRound: "R4-SA-A", ...},
{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/assets/:asset_id/download", OwnerRound: "R4-SA-A", ...},
{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/assets/:asset_id/versions/:version_id/download", OwnerRound: "R4-SA-A", ...},
{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/assets/:asset_id/archive", OwnerRound: "R4-SA-A", ...},
{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/assets/:asset_id/restore", OwnerRound: "R4-SA-A", ...},
{GroupBase: "/v1", Method: http.MethodDelete, RelativePath: "/assets/:asset_id", OwnerRound: "R4-SA-A", ...},
```

**全部删除**(7 行)。理由:SA-A 已在 line 376–382 真实挂载 · reserved 表保留它们就是双重 mount 残留。

### 3.2 transport/handler/task_asset_center.go(4 处 c.Param 重命名)

| 行 | handler | 旧 | 新 |
| --- | --- | --- | --- |
| 148 | `GetGlobalAsset` | `c.Param("id")` | `c.Param("asset_id")` |
| 166 | `DownloadGlobalAsset` | `c.Param("id")` | `c.Param("asset_id")` |
| 249 | `DeleteGlobalAsset` | `c.Param("id")` | `c.Param("asset_id")` |
| 331 | `PreviewAssetResource` | `c.Param("id")` | `c.Param("asset_id")` |

**注意**:同文件还有 `GetAssetResource`(line 265)、`DownloadAssetResource`(line 317)是 dead handler(未在 http.go 任何地方挂载)· **不动**。

### 3.3 OpenAPI 不动

`docs/api/openapi.yaml` 内 `/v1/assets/{id}` 与 `/v1/assets/{asset_id}` 混用是 v0.9 → v1 演进的设计遗产(R3 接管路径用 `{id}` · SA-A 新增路径用 `{asset_id}`)。**这一轮不动 OpenAPI**。Gin wildcard 名是框架内变量名 · 与 URL 模板占位符是分离的;router 全部改 `:asset_id` 后,URL 字符串(`/v1/assets/42`)仍能匹配 OpenAPI 任一占位符。

**例外**:如果 `openapi-validate` 工具对 wildcard 变量名做严格匹配并报错 → 在补丁报告里详细记录;**仍不改 OpenAPI**;改为请求架构师裁决是否单开 R1.7-A.1 OpenAPI 微调补丁。

---

## 4. 硬约束(命中即 ABORT)

- 修改 §3 白名单之外的任何文件 → ABORT
- 修改 OpenAPI 任意行 → ABORT
- 改 SA-A handler 内任何**业务字段**(只许改 `c.Param("id")` → `c.Param("asset_id")` 这 4 处字符串)→ ABORT
- 改 `service/asset_center/**`、`service/asset_lifecycle/**`、`repo/**` 任何文件 → ABORT
- 改 R4 子轮报告(SA-A/B/C/D 已签字)→ ABORT
- `cmd/server` 启动后仍 panic → ABORT(改不到根因)
- 联合 integration 出 FAIL → ABORT(本补丁应保持 100% 测试通过)
- 测试库残留 `id ∈ [50000, 60000)` 未清 → ABORT

---

## 5. 验证清单(全跑 · 任一 fail 即 ABORT)

```bash
cd /mnt/c/Users/wsfwk/Downloads/yongboWorkflow/go
DSN="$(ssh jst_ecs 'bash /root/ecommerce_ai/r3_5/build_test_dsn.sh')"
export MYSQL_DSN="$DSN"
export R35_MODE=1
export GOPATH=$HOME/.cache/go-path && mkdir -p $GOPATH

# A) build 双 tag 绿
/home/wsfwk/go/bin/go build ./... 2>&1                                        | tee tmp/r4_sa_a_patch_a2_build_default.log
/home/wsfwk/go/bin/go build -tags=integration ./... 2>&1                      | tee tmp/r4_sa_a_patch_a2_build_integration.log

# B) cmd/server 启动 smoke(此前 panic 的关键证据)
setsid bash -c '/home/wsfwk/go/bin/go run ./cmd/server' \
  >/tmp/r4_sa_a_patch_a2_server.log 2>&1 < /dev/null &
echo $! > tmp/r4_sa_a_patch_a2_server.pid
sleep 8
curl -fsS http://127.0.0.1:8080/healthz                                       | tee tmp/r4_sa_a_patch_a2_healthz.log
test -s tmp/r4_sa_a_patch_a2_healthz.log || { echo SERVER_PANIC; tail -50 /tmp/r4_sa_a_patch_a2_server.log; exit 1; }

# C) 启动后再打 SA-A 7 触点(所有 GET/DELETE 用 :asset_id 真值 · POST 同)· 期望 200/401/403/404 · 严禁 5xx
# (按合理身份探针;参考 retro Step D)

# D) 关后端
kill -9 $(cat tmp/r4_sa_a_patch_a2_server.pid) 2>/dev/null || true

# E) SA-A integration 全跑(SAAI · 不破坏)
/home/wsfwk/go/bin/go test -tags=integration -count=1 -run 'SAAI' \
  ./service/asset_center/... ./service/asset_lifecycle/... ./transport/handler/... 2>&1 \
  | tee tmp/r4_sa_a_patch_a2_integration.log

# F) 联合 integration 4 域 + R3 不破坏
/home/wsfwk/go/bin/go test -tags=integration -count=1 -timeout 30m \
  -run 'SAAI|SABI|SACI|SADI|TestModuleAction|TestTaskPool|TestTaskCancel|TestTaskAggregator' \
  ./service/... ./transport/handler/... ./transport/ws/... 2>&1 \
  | tee tmp/r4_sa_a_patch_a2_integration_full.log

# G) 全 unit test
/home/wsfwk/go/bin/go test ./... -count=1 2>&1 \
  | tee tmp/r4_sa_a_patch_a2_unit.log

# H) openapi-validate 必 0/0(不动 OpenAPI · 应仍绿)
/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml 2>&1 \
  | tee tmp/r4_sa_a_patch_a2_openapi_validate.log

# I) reserved 表残留扫描:确认 SA-A 7 条已清
grep -nE 'OwnerRound: *"R4-SA-A"' transport/http.go 2>&1 \
  | tee tmp/r4_sa_a_patch_a2_reserved_scan.log
# 期望:0 行(7 条全删)
```

---

## 6. 报告输出(新建 `docs/iterations/V1_R4_SA_A_PATCH_A2_REPORT.md`)

章节顺序:

1. `## Scope` — 触发原因 · BLOCKER 描述 · 修复方向
2. `## §3.1 修复 A:GET wildcard 归一` — 4 行 diff(用代码块)
3. `## §3.2 修复 B:reserved 表清残` — 7 行删除 diff
4. `## §3.3 c.Param 重命名` — 4 行 diff
5. `## §4 OpenAPI 影响评估` — 不动的理由 + openapi-validate 0/0 实跑
6. `## §5 验证证据` — A~I 9 步逐个 PASS / 关键末尾输出
7. `## §6 cmd/server 启动证据` — `/healthz` 200 实跑 + 启动日志关键行
8. `## §7 联合 integration 证据` — full log 末尾 PASS 计数
9. `## §8 数据隔离审计` — 测试库 `[50000,60000)` 9 表残留 0
10. `## §9 已知非目标` — 不动 OpenAPI · 不改业务 · 不并发其他补丁
11. `## §10 sign-off candidate` — 声明可签字 · 待主对话架构师裁决

---

## 7. 工作流程

1. 读 §1 必读 · 一次读齐
2. 直接按 §3 三处定点修改 · 不做任何超出范围的改动
3. 跑 §5 验证 A→I 9 步 · 任一 fail 立即抓 stack 写报告 · ABORT
4. 写报告 · 结束

## 8. 最终回报内容

- 修改文件清单(2 个 · 共 11 处 · 详 §3)
- A~I 9 步验证结果(每步 PASS / FAIL)
- `cmd/server` 启动是否成功 + healthz 状态
- SA-A 7 触点 live smoke 是否全 200/4xx 不 5xx
- openapi-validate 是否仍 0/0
- 是否有 ABORT(若有 · 哪一条)

完成后简洁回报 · 然后退出。
