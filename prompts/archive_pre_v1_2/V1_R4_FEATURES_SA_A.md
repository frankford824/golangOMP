# V1 · R4-SA-A · 资产管理中心 + OSS 生命周期

> 版本:**v2**(2026-04-17 · R1.7 OpenAPI 补丁落地后重启)
> 状态:**待 Codex 执行**(R4 顺序 4 轮第 1 轮)
> 依赖:R1(OpenAPI 冻结)+ R2 v3(资产表 5 个新列已生产落地)+ R3 v1 + R3.5 验证 + **R1.7 SA-A OpenAPI 补丁**(2026-04-17 签字)
> 执行环境:**本地代码 + `jst_ecs` 上的 `jst_erp_r3_test` 测试库**(R3.5 已搭好,直接复用);生产 `jst_erp` 零写入
> 禁止前置:R3.5 未签字时不得启动

---

## 0. 本轮目标(一句话)

> **把 OpenAPI 冻结的 7 个跨任务资产 handler 从 `501` 换成真实现**,落地资产生命周期 5 态机 + 归档/恢复/删除 + 410 GONE + 清理 job 的代码骨架(但**不启用 cron**,R6 再开);**不改 OpenAPI**、**不动 R2 DDL**、**不碰现有任务级资产端点**。

---

## 1. 必读输入

1. `docs/V1_ASSET_OWNERSHIP.md` **v1.2** · **本轮唯一权威**
   - §1.5 核心约定
   - §2.1 `task_assets` 新增 7 列(R2 已落)
   - §4 audit 的 asset_versions_snapshot(SA-A **只读取**,审核本身属 R3 action executor;SA-A 要保证归档后这些 snapshot 还可读)
   - §5 资产管理中心(**最权威的功能规格**)
   - §7.1 5 态机 + §7.2 365 天清理 job(注:R6 落地 · 本轮只写代码骨架 + disable schedule)
   - §7.3 手动归档 / 删除
   - §7.4 事件 payload 清理兼容
   - §9 AS-A4 / AS-A5 / AS-A6 / AS-A7 验收项
2. `docs/V1_MODULE_ARCHITECTURE.md` v1.2
   - §6.2 deny_code 枚举(SA-A 不得新增;`403` 情况全部用 `module_action_role_denied`,`410` 不走 deny_code)
3. `docs/V1_INFORMATION_ARCHITECTURE.md` v1.1
   - §3 资产管理中心菜单位置(只读参考,SA-A 不写前端)
4. `docs/iterations/V1_R2_REPORT.md` v1 + `docs/iterations/V1_R3_REPORT.md` v1 + `docs/iterations/V1_R3_5_INTEGRATION_VERIFICATION.md` v1 + **`docs/iterations/V1_R1_7_OPENAPI_SA_A_PATCH.md` v1.0**
   - 真生产基线 · 本轮 integration test 用的测试库状态
   - R1.7 补的 3 个新 schema(`AssetReasonRequest` / `AssetVersion` / `AssetDetail`)· 所有 SA-A 路径的 query/body/response 已对齐权威文档 · 本轮无需再扩 OpenAPI
5. `docs/api/openapi.yaml` · 仅阅读以下 7 条路径的 schema:
   - `GET /v1/assets/{id}` · `DELETE /v1/assets/{id}` · `GET /v1/assets/{id}/download`
   - `GET /v1/assets/search` · `GET /v1/assets/{asset_id}/versions/{version_id}/download`
   - `POST /v1/assets/{asset_id}/archive` · `POST /v1/assets/{asset_id}/restore`
6. `domain/asset*.go`(现有资产模型)· `repo/` 下现有 asset 相关 repo(**仅读**,不改)
7. `service/permission/authorize_module_action.go`(R3 已落,SA-A 的写动作要走它)
8. `testsupport/r35`(R3.5 加的测试辅助,本轮直接用)

禁止引用:R1.5 之前的文档;`docs/archive/*`;任何"assets preview"相关代码(**不属 SA-A**)

---

## 2. 真生产基线(从 R2/R3.5 报告固化)

Codex 实现读路径时必须认到这些事实:

| 事实 | 影响 |
| --- | --- |
| `task_assets` 共 265 条(`jst_erp_r3_test` 与生产一致);`asset_storage_refs` 451 条 | SA-A search 的索引设计必须针对这个量级 + 可扩展;不要对全表扫描放任 |
| `source_module_key` 真分布 `basic_info=9 / customization=5 / design=251` | `GET /v1/assets/search?module_key=customization` 期望返回 ≤ 5 条(生产当下);实现不能假设某模块资产量级 |
| `task_assets.is_archived` 全部为 0,`cleaned_at` 全为 NULL,`deleted_at` 全为 NULL | 生产当下 5 态机所有资产都落 `active` / `closed_retained`;**`archived` / `auto_cleaned` / `deleted` 目前生产 0 例**,integration test 必须自己造数据覆盖三态 |
| `tasks.task_status` 有 `Completed` 2 条 + `PendingClose` 5 条 · 共 7 条任务已进入终态域 | `closed_retained` 状态推导在本数据集上有样本;测试数据无需再造任务 |
| R2 / R3 已用掉 059~068 迁移号段 | SA-A **不新增 migration**(5 态机可以从现有列推导) |
| `reference_file_refs.ref_id` 列级 collation `utf8mb4_0900_ai_ci` | 若 SA-A search 要 JOIN 到 `reference_file_refs`,不加 COLLATE 子句 |

---

## 3. 交付范围

### 3.1 Domain / 模型层

| 文件 | 作用 |
| --- | --- |
| `domain/asset_lifecycle_state.go` | 5 态常量 + `DeriveLifecycleState(asset TaskAsset, task Task) LifecycleState` 纯函数;输入 `is_archived / cleaned_at / deleted_at / task.task_status / task.terminal_at(用 tasks.updated_at 近似或主 §10.1 推导)`,输出 `active / closed_retained / archived / auto_cleaned / deleted` |
| `domain/asset_search_query.go` | search 参数结构体(§5.3 精确字段:keyword / module_key / owner_team_code / created_from / created_to / page / size / is_archived(default 0)/ task_status(open/closed/archived/all)) |

严禁:改 `domain/asset.go` 现有字段(现有 DDL 已是权威);改 `domain/enums_v7.go`。

### 3.2 Service 层

| 包 | 文件 | 职责 |
| --- | --- | --- |
| `service/asset_center` | `search.go` | 跨任务 search · 分页 · Layer 1 全可见 · 默认过滤 `is_archived=0` · 可选 `is_archived=true/false/all` · 排序 `task_assets.created_at DESC` |
|  | `detail.go` | `GET /v1/assets/{id}` 聚合:资产基本信息 + 版本列表 + `lifecycle_state`(调 DeriveLifecycleState)+ 当前 storage_key + 归档元数据 |
|  | `download.go` | `GET /v1/assets/{id}/download`(最新版本)+ `/versions/{version_id}/download`(特定版本)· 预签名 URL · 遇 `auto_cleaned` → 410 GONE · 遇 `deleted` → 404 |
|  | `search_test.go` / `detail_test.go` / `download_test.go` | 单元测试 |
| `service/asset_lifecycle` | `state_machine.go` | 5 态机转移表 + `CanArchive(state)` / `CanRestore(state)` / `CanDelete(state)` 守卫函数 |
|  | `archive_service.go` | SuperAdmin 归档:`UPDATE task_assets SET is_archived=1, archived_at=NOW(), archived_by=?` + 写 `task_module_events.event_type=asset_archived_by_admin`(事件 payload 含 reason / actor)· 事务 |
|  | `restore_service.go` | SuperAdmin 取消归档:`UPDATE task_assets SET is_archived=0, archived_at=NULL, archived_by=NULL` + 事件 `asset_unarchived_by_admin` |
|  | `delete_service.go` | SuperAdmin 硬删除:先调 OSS DeleteObject(通过 `infra/oss` 现有 client) · 再 `UPDATE task_assets SET deleted_at=NOW(), storage_key=NULL` + 事件 `asset_deleted_by_admin`(必填 reason) |
|  | `cleanup_job.go` | 365 天自动清理 job 的**函数实现**(核心逻辑)· 批处理 · 幂等 · 日志前缀 `[ASSET-CLEANUP]` · **接受 `--dry-run`** 参数 · 写事件 `asset_auto_cleaned`(payload 含原 storage_key 便于审计) |
|  | `cleanup_job_test.go` | 单元测试 + 故意跑 dry-run 不写任何数据 |
| `service/asset_lifecycle/scheduler` | `register.go` | cron 注册点 · 但**默认 `enabled=false`**(通过 `infra/config` 里的 `AssetCleanup.Enabled` 布尔,默认 false);R6 翻开 | 本轮仅暴露接口,不注册到任何 runtime cron |
|  | `register_test.go` | 断言默认 disabled;启用后仅调用 job function 一次 |

### 3.3 Repo 层

| 文件 | 作用 |
| --- | --- |
| `repo/task_asset_search_repo.go` | 定义 search interface(SearchQuery → []TaskAsset + total) |
| `repo/mysql/task_asset_search_repo.go` | MySQL 实现 · SQL 里显式引用 R2 新列 `is_archived / cleaned_at / deleted_at / archived_at / archived_by / source_module_key / source_task_module_id`;JOIN `tasks` 为 `task_status / priority`;必要时 LEFT JOIN `task_modules` 为 `owner_team_code`(通过 `claimed_team_code` 或 `pool_team_code`) |
| `repo/task_asset_lifecycle_repo.go` | Archive / Restore / Delete / ListEligibleForCleanup 接口 |
| `repo/mysql/task_asset_lifecycle_repo.go` | 上面接口的 MySQL 实现 · 所有 UPDATE 用事务 · 硬删除走软删(`deleted_at IS NOT NULL`),严禁 DELETE FROM |

严禁:改现有 `repo/task_asset*.go` 现有文件(v0.9 任务内资产 repo)

### 3.4 Transport 层

替换 `transport/` 里现有 R4-SA-A 的 7 个 501 stub:

| handler | 路径 | 角色门控 |
| --- | --- | --- |
| `handleAssetSearch` | `GET /v1/assets/search` | 登录用户(Layer 1) |
| `handleAssetGet` | `GET /v1/assets/{id}` | 登录用户 |
| `handleAssetDownload` | `GET /v1/assets/{id}/download` | 登录用户 · 态机判断 410/404 |
| `handleAssetVersionDownload` | `GET /v1/assets/{asset_id}/versions/{version_id}/download` | 登录用户 · 态机判断 410/404 · 即使资产已 `auto_cleaned`,**历史事件的 `asset_versions_snapshot` 读取仍能命中原 version 元数据**(§7.4) |
| `handleAssetArchive` | `POST /v1/assets/{asset_id}/archive` | **仅 SuperAdmin** · 必填 `reason` |
| `handleAssetRestore` | `POST /v1/assets/{asset_id}/restore` | **仅 SuperAdmin** |
| `handleAssetDelete` | `DELETE /v1/assets/{id}` | **仅 SuperAdmin** · 必填 `reason` · 返回 204 |

**权限实现**:

- 登录用户门控:直接判断 actor 非空
- SuperAdmin 门控:`actor.HasRole(domain.RoleSuperAdmin)`;拒绝 → 返回 `403` + body `{ "error": { "code": "module_action_role_denied", ... } }`(**复用 R3 deny_code · 不新增**)

### 3.5 OSS 基础设施复用

不新建 OSS 客户端。**复用** `infra/oss`(或等价路径)下现有 client:

- `PresignGet(key, ttl)` · 下载预签名
- `DeleteObject(key)` · 硬删除 + 清理 job 调用

若现有 client 缺 `DeleteObject`,**在原文件内追加方法**(单行追加,不改包结构)+ 单测。

---

## 4. 5 态机推导规则(权威)

**只基于 DB 字段推导,不加新列**。输入:`task_assets` 行 + 其任务的 `tasks` 行。

```go
func DeriveLifecycleState(a TaskAsset, t Task) LifecycleState {
    if a.DeletedAt != nil           { return StateDeleted }
    if a.CleanedAt != nil           { return StateAutoCleaned }
    if a.IsArchived                 { return StateArchived }
    // 以下基于任务终态 + 365 天阈值(本轮仅态机 · 清理 job 用同公式)
    if taskIsTerminal(t)            { return StateClosedRetained }
    return StateActive
}

func taskIsTerminal(t Task) bool {
    // 对齐 domain/enums_v7.go 终态集合:Completed / Cancelled / Archived
    return t.TaskStatus == TaskStatusCompleted ||
           t.TaskStatus == TaskStatusCancelled ||
           t.TaskStatus == TaskStatusArchived
}
```

- 365 天阈值**只由清理 job 用**(见 §3.2 `cleanup_job.go`),态机本身不判断时间
- `closed_retained` 状态下 `download` 仍返回预签名 URL(§7.1 "可下载")

---

## 5. DON'T TOUCH

- `docs/api/openapi.yaml`(若发现 7 条路径 schema 不够,**立即 abort**,不自行扩)
- `db/migrations/001~068`(SA-A 不加迁移)
- R3 落地的 `service/blueprint / service/module / service/permission / service/task_pool / service/task_cancel / service/task_aggregator / service/module_action`(只允许调用其公开 API,不改文件)
- 现有 `service/` / `repo/` 下除新建包外所有文件
- `transport/http.go` 里 R4-SA-B/C/D 的 501 stub(保持 501 不动)
- 现有所有 `/v1/tasks/{id}/assets/*` 和 `/v1/tasks/{id}/asset-center/*` 任务级端点(保持现状)
- `/v1/assets/{id}/preview`(不属 SA-A,owner 另定)
- 任何触碰 `jst_erp` 生产的写入;本轮所有 integration 走 `jst_erp_r3_test`
- R3.5 的 `testsupport/r35` 里的 DSN 守卫(只用,不改)

---

## 6. 执行环境

**复用 R3.5 的 `jst_erp_r3_test`**(在 `jst_ecs`):

```bash
# 确认测试库仍在
ssh jst_ecs 'mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -p"$DB_PASS" -e "SHOW DATABASES LIKE '\''jst_erp_r3_test'\'';"'

# 若被人 DROP 了,重建:
ssh jst_ecs 'bash /root/ecommerce_ai/r3_5/setup_test_db.sh'

# Integration test DSN
DSN=$(ssh jst_ecs 'bash /root/ecommerce_ai/r3_5/build_test_dsn.sh')
MYSQL_DSN="$DSN" R35_MODE=1 /home/wsfwk/go/bin/go test ./service/asset_center/... ./service/asset_lifecycle/... -tags=integration -count=1 -v
```

**严格守则**:

- 所有 integration test 必须走 `testsupport/r35.MustOpenTestDB(t)`,DSN 不以 `_r3_test` 结尾即 `t.Fatalf`
- SA-A 追加的 integration test 必须**创造自己的测试样本**(新插入 task_assets 行,`task_id >= 20000` 段 + `source_task_module_id = NULL 可空`),用 defer 清理,**不污染** R3.5 已有测试数据

---

## 7. 测试要求

### 7.1 单元测试(100% 绿)

- `domain/asset_lifecycle_state_test.go`:5 态 × 关键输入组合表驱动,覆盖所有分支
- `service/asset_center/*_test.go`:search / detail / download 行为(用 fake repo)
- `service/asset_lifecycle/*_test.go`:archive/restore/delete 权限 + 态机守卫 + 事件 payload 格式
- `service/asset_lifecycle/cleanup_job_test.go`:dry-run 不写 DB / 真跑 mock DB 写入符合预期 / 幂等重跑
- `service/asset_lifecycle/scheduler/register_test.go`:默认 disabled

### 7.2 Integration Tests(build tag `integration`)

| # | 断言 | 对应验收 |
| --- | --- | --- |
| SA-A-I1 | `GET /v1/assets/search?is_archived=false` 返回所有 lifecycle_state ∈ {active, closed_retained} 资产 | AS-A4 |
| SA-A-I2 | `GET /v1/assets/search?is_archived=all` 可看到自建 archived 资产(测试 defer 清理) | §5.3 |
| SA-A-I3 | `POST /v1/assets/{id}/archive` 以非 SuperAdmin → 403 + deny_code=`module_action_role_denied`;以 SuperAdmin → 202 + 事件 `asset_archived_by_admin` 已写入 | AS-A4 / §5.4 |
| SA-A-I4 | `POST /v1/assets/{id}/restore` 以 SuperAdmin 成功 → `is_archived=0, archived_at=NULL` + 事件 `asset_unarchived_by_admin` 已写入 | §7.3 |
| SA-A-I5 | `DELETE /v1/assets/{id}` 以 SuperAdmin + reason → 204 + `deleted_at IS NOT NULL` + `storage_key IS NULL` + 事件 `asset_deleted_by_admin` 已写入 | §5.4 |
| SA-A-I6 | `GET /v1/assets/{id}/download` 对 `auto_cleaned` 资产(自建:插入 `cleaned_at=NOW(), storage_key=NULL`)→ 410 GONE;对 `deleted` 资产 → 404 | AS-A6 |
| SA-A-I7 | `GET /v1/assets/{asset_id}/versions/{version_id}/download` 对清理资产返回 410,但事件表 `task_module_events` 中旧 `asset_versions_snapshot` payload 能读到原 version 元数据(证明 §7.4)| AS-A7 |
| SA-A-I8 | 清理 job dry-run 扫描 365 天前终态任务,列表非空(测试前插入 `task_status=Completed, updated_at = NOW() - INTERVAL 400 DAY`),不写任何数据;真跑一次,资产 `cleaned_at=NOW()` + `storage_key=NULL` + 事件 `asset_auto_cleaned` 写入;再跑一次幂等,无新数据 | AS-A5(staging 演练 1000 条本轮不做,10 条代表性即可) |

### 7.3 全量回归

```bash
/home/wsfwk/go/bin/go build ./...
/home/wsfwk/go/bin/go test ./... -count=1
MYSQL_DSN=... R35_MODE=1 /home/wsfwk/go/bin/go test ./... -tags=integration -count=1
/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
```

### 7.4 生产 Probe(零污染证据)

R4-SA-A 跑完后必须最后跑一次生产只读 probe:

```bash
scp tmp/r2_probe_readonly.sh jst_ecs:/tmp/sa_a_probe.sh
ssh jst_ecs 'bash /tmp/sa_a_probe.sh' > docs/iterations/r4_sa_a_probe.log
```

断言:
- 生产 `task_assets.is_archived` 全 0(SA-A 没写生产)
- 生产 R2 目标表行数与 R2 post-run 基线相同
- 不与 R3.5 post-run 发生漂移

---

## 8. 交付物清单

1. `domain/asset_lifecycle_state.go` · `asset_search_query.go`
2. `service/asset_center/*`(search / detail / download + 测试)
3. `service/asset_lifecycle/*`(state_machine / archive / restore / delete / cleanup_job + 测试)
4. `service/asset_lifecycle/scheduler/*`(默认 disabled 的注册点)
5. `repo/task_asset_search_repo.go` + `repo/mysql/task_asset_search_repo.go`
6. `repo/task_asset_lifecycle_repo.go` + `repo/mysql/task_asset_lifecycle_repo.go`
7. 7 个 `transport/*.go` handler(替换 501)
8. 8 条 integration test(build tag `integration`)
9. `docs/iterations/V1_R4_SA_A_REPORT.md`:

**报告强制章节**:

- `## Scope` · 7 条 handler 清单 + 每条的 deny 路径 + 返回码矩阵
- `## 5-State Machine` · `DeriveLifecycleState` 的 5 输入组合单测结果
- `## 8 Integration Assertions` · SA-A-I1 ~ SA-A-I8 的实际 SQL / HTTP 数据证据
- `## Cleanup Job` · dry-run / 真跑 / 幂等重跑三次结果 · 插入的 10 条测试数据清单
- `## Scheduler Posture` · 证明 cron 未启动(`infra/config` 里 `AssetCleanup.Enabled=false`,grep 全仓无注册点)
- `## Test DB Touch` · `jst_erp_r3_test` 中 SA-A 测试数据范围(`task_id >= 20000` 段);执行后的 defer 清理证据
- `## Production Probe Diff` · 生产 R2 表行数与 R2 post 一致 ± 0 行;`is_archived` 分布仍全 0
- `## OpenAPI Conformance` · 0/0
- `## Known Non-Goals` · 清理 job 的 cron 触发(R6)+ 资产 preview 端点(不属 SA-A)+ audit snapshot 写入端(R3 action executor)

---

## 9. 失败终止条件

- ~~7 条 R4-SA-A 路径在 OpenAPI 里 schema 字段与 §5.2/§5.3 对不上~~ **R1.7 v1.0 已补齐,本条失效**;如 Codex 仍认为 schema 不一致,说明 R1.7 执行不完整,**直接回报主对话**(而非 abort 重起一轮)
- 清理 job cron 被误注册进 runtime(有任何 `AddFunc / cron.Schedule` 指向 `cleanup_job`)→ abort
- 任何 DSN 守卫失效(测试跑出 `jst_erp` 写入)→ abort
- integration 断言任一 FAIL
- 生产 probe 显示 SA-A **控制字段**(`is_archived` / `archived_at` / `archived_by` / `cleaned_at` / `deleted_at`)有任何写入痕迹 → abort。**注**:表行数 drift 若由生产 live traffic 产生(例如资产上传线新增 task_assets 行)**不算违反**,只要 SA-A 控制字段零写入即可。对照证据见 §7.4 探测的 `task_assets_is_archived` 与 `task_assets_lifecycle_dirty` 两个聚合计数
- 某处需要新 deny_code 才能通过测试 → abort(改主 §6.2 + OpenAPI 是下一轮决策)

---

## 10. 给 Codex 的最后一句话

> SA-A 是 R4 的第 1 轮,后面还有 B/C/D 3 轮,每轮 30~90 分钟。
> 你的范围只有"**跨任务资产中心 + 5 态机 + 清理 job 骨架**",任务内资产端点不归你 · audit snapshot 写入不归你 · preview 不归你 · cron 不归你。
> 遇到需要改 OpenAPI / 改 DDL / 加 deny_code / 启动 cron 的冲动,**立即 abort** 并报告,让主对话裁决。
> 不要在生产 `jst_erp` 上跑任何东西,测试库是 `jst_erp_r3_test`。

---

## 变更记录

| 版本 | 日期 | 变更 |
| --- | --- | --- |
| v1 | 2026-04-17 | 初稿 · 基于资产 v1.2 § 5-9 + R3.5 测试库固化。7 handler 清单锁定(`x-owner-round: R4-SA-A`);5 态机纯派生(无新列);清理 job 代码落地但 cron disabled 推 R6;integration 走 `jst_erp_r3_test` · 测试数据 `task_id >= 20000` 段隔离 |
| v2 | 2026-04-17 | **R1.7 OpenAPI 补丁落地后重启**。Codex v1 前置校验正确 abort(search 缺 query / archive+delete 缺 reason body / 4 条路径残留 501);主对话按 §5.2/§5.3/§5.4/§7.3 补 9 点 OpenAPI(3 新 schema `AssetReasonRequest`/`AssetVersion`/`AssetDetail` + 6 path 编辑);`openapi-validate` 0/0。本 prompt §1 新增必读输入 R1.7 报告;§9 失败终止条件里"OpenAPI schema 对不上"一条失效(改为"若仍不一致 → 回报主对话")|
| v2.1 | 2026-04-17 | **SA-A 落地后教训固化**。Codex v2 正确执行并产出报告,但按 v2 §9 "生产 probe 行数变化即 abort" 字面条件触发签字阻塞;架构裁决 drift 100% 来自 live traffic 业务增长(`is_archived` 全 0、`cleaned_at/deleted_at` 全 0、SA-A 代码路径不含 `INSERT INTO task_assets/asset_storage_refs`)。§9 规则修订为"**控制字段零写入**"而非"表行数 ± 0",为 SA-B/C/D prompt 提供正确模板。SA-A 签字通过,report 升 v1.1 |
