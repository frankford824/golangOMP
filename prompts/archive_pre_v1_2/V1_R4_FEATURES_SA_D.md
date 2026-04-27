# V1 · R4-SA-D · 全局搜索 + 报表 L1

> 版本:**v1.0**(2026-04-24)
> 状态:**起草完成 · 待 Codex 执行**
> 依赖:R1(OpenAPI 冻结)+ R2 v3(`task_module_events` 已生产落地)+ R3 v1 + R3.5 验证 + SA-A v2.1 签字 + SA-B v1.1 签字 + SA-B.1 + SA-B.2 + SA-C v1 签字 + SA-C.1 签字 + **R1.7-D OpenAPI SA-D 前置补丁签字**(2026-04-24)
> 执行环境:**本地代码 + `jst_ecs` 上的 `jst_erp_r3_test` 测试库**(R3.5/SA-A/SA-B/SA-C 已搭好 · 直接复用);生产 `jst_erp` 零写入
> 禁止前置:R1.7-D 或 SA-C 整体(v1+.1)任一未签字时不得启动

---

## 0. 本轮目标(一句话)

> **把 OpenAPI 冻结的 4 条 R4-SA-D 路径(`/v1/search` + `/v1/reports/l1/cards` + `/v1/reports/l1/throughput` + `/v1/reports/l1/module-dwell`)从 `501` 补齐到 `V1_INFORMATION_ARCHITECTURE.md` v1.1 §4.2 + `V1_MODULE_ARCHITECTURE.md` v1.2 §12 + `V1_R1_7_D_OPENAPI_SA_D_PATCH.md` 合同**,落地 MySQL LIKE 全局搜索(users[] 低权空数组)+ 3 条 L1 报表直查 `task_module_events` + `tasks`(仅 SuperAdmin);**不改 OpenAPI**、**不动 R2 DDL**、**不新建物化表**、**不引 ES/Redis**、**不做导出**。

---

## 1. 必读输入

1. `docs/V1_INFORMATION_ARCHITECTURE.md` **v1.1** · **本轮权威之一**
   - §1 一级菜单「报表」(仅 SuperAdmin 可见)
   - §4 全局搜索(**本轮主轴之一**)· §4.1 MVP 覆盖对象(tasks/assets/products/users)· §4.2 端点 `GET /v1/search?q=&scope=&limit=20` · §4.3 行级权限(users 低权空数组)· §4.4 技术(MySQL LIKE · v1 不上 ES)· IA-A2/A3 验收
   - §11 验收:IA-A2(搜索命中)/IA-A3(搜索 P95<300ms 告警 · 非 abort 门)
2. `docs/V1_MODULE_ARCHITECTURE.md` **v1.2** · **本轮权威之二**
   - §12 实施路线 U 表:报表 L1 v1 直查 `task_module_events` + `tasks` · R6+ 视情况物化
   - §6.2 deny_code 枚举(SA-D **不得新增** · 403 仅用本轮新确认的 `reports_super_admin_only`;该 code 在 R1.7-D 补丁中已入 OpenAPI ErrorResponse · 实装需在 `domain/errors.go` 或等价位置登记)
   - §13 验收 A12(事件流可独立消费 · SA-D 读 `task_module_events` 满足)
3. `docs/iterations/V1_R1_7_D_OPENAPI_SA_D_PATCH.md` **v1.0**(**本轮核心 · 10 问裁决明文化**)
   - §2 架构师 10 问裁决表(Q1=A1 / Q2=U1 / Q3=B1 / Q4=S1 / Q5=E1 / Q6=C1 / Q7=T1 / Q8=F1 / Q9=R3 / Q10=AGREE)
   - §3 OpenAPI Diff 实际落地(4 路径 + 3 schema + RBAC + ErrorResponse)
   - §5 路径对齐清单
4. `docs/iterations/V1_R2_REPORT.md` v1 + `V1_R3_REPORT.md` v1 + `V1_R3_5_INTEGRATION_VERIFICATION.md` v1 + `V1_R4_SA_A_REPORT.md` v1.1 + `V1_R4_SA_B_REPORT.md` v1.1 + `V1_R4_SA_C_REPORT.md`
   - 真生产基线 + R3.5 测试库 + SA-A/B/C 已追加的 integration 数据段隔离约定(SA-A task_id ≥ 20000 · SA-B user_id ≥ 30000 · SA-C user_id ≥ 40000 · **SA-D 用 user_id ≥ 50000 + task_id ≥ 50000**)
5. `docs/api/openapi.yaml` · 仅阅读以下 4 条路径 + 3 schema(**禁止修改**):
   - `GET /v1/search` · `GET /v1/reports/l1/cards` · `GET /v1/reports/l1/throughput` · `GET /v1/reports/l1/module-dwell`
   - `components/schemas/SearchResultGroup` · `components/schemas/L1Card` · `components/schemas/ErrorResponse`
6. 现有代码(**必读 · 本轮是扩展而非重建**):
   - `domain/auth_identity.go`(`User` / `UserSession` / `PermissionAction` 常量)· 只追加 · 不改现有
   - `domain/errors.go`(或 `service/*/errors.go` 等价位置)· SA-D 新增 `deny_code = reports_super_admin_only` 常量
   - `service/task_module/*` · `service/module_action/*`(R3 已实现)· SA-D 只读 `task_module_events` · **零改动**
   - `repo/mysql/task_*.go` · SA-D 新建 `repo/search_repo.go` + `repo/report_l1_repo.go` · 不碰 R3 repo
   - `testsupport/r35`(R3.5 测试 DSN 守卫 · 直接用)
   - `domain/identity.go` / `service/identity_service.go`(SA-B)· SA-D 只读取 `actor.Role` 判定 SuperAdmin

禁止引用:R1.5 之前的文档;`docs/archive/*`;任何 R4-SA-A / SA-B / SA-C owner 的代码目录(`service/asset_center` / `service/asset_lifecycle` / `service/org_move_request` / `service/notification` / `service/websocket` / `service/task_draft` / `service/erp_product` / `service/design_source` 等)

---

## 2. 真生产基线(从 R2/R3.5/SA-A/SA-B/SA-C 报告 + 2026-04-24 pre-probe 固化)

Codex 实现读路径时必须认到这些事实:

| 事实 | 影响 |
| --- | --- |
| `task_module_events` 表生产 300 行 · event_type 分布:`migrated_from_v0_9`×298 / `backfill_placeholder`×2(来自 SA-C pre-probe)| SA-D L1 报表直查此表 · `throughput` / `module-dwell` 聚合必须能识别并排除 `backfill_placeholder`(它不是真实业务事件)· 生产 300 行不足以产出有意义的 L1 数据,Codex 不得在报告中"编造"throughput;生产 probe 只验证 0 污染 |
| `tasks` 表生产行数见 R3.5 报告 · SA-D 只读 · 不写 | `reports/l1/cards` 的 `tasks_in_progress` 等卡片查 `tasks.task_status`;任何对 `tasks` 的写入即 SA-D 违反白名单 |
| `users` 表生产用户名单见 SA-B 报告 · SA-D 只读 | `/v1/search` `users[]` 分支只在 `actor.Role ∈ {super_admin, hr_admin}` 时查询;其他角色直接返回 `users: []`(R1.7-D Q1=A1 + Q2=U1);**无论 `scope=all` 还是 `scope=users`,低权一律空** |
| `task_assets` 表生产行数见 SA-A 报告 · SA-D 只读 | `/v1/search` `assets[]` LIKE `file_name` |
| `erp_product_snapshots`(若有)或上游 ERP 不在生产 MySQL · SA-C 已走 `service/erp_product` 桥接 | SA-D 搜索 products 分支可查现有 `erp_product_snapshots` 表(R2 已建 · 若存在)或退回查 `tasks.erp_code` 的 distinct(R6+ 可接 ES);**绝不**走 ERP 上游实时查询(SA-C 已独占此路径 · SA-D 只查已缓存/已落盘数据 · 搜索必须 <300ms) |
| SA-A/B/C 控制字段(`is_archived` / `org_move_requests.*` / `notifications.*` / `task_drafts.*`)生产全 0(SA-C pre-probe H 段)| SA-D post probe 必须保持跨域字段仍全 0 · 任一非 0 即 SA-D 跨域污染 |
| `task_module_events.created_at` 为 `DATETIME` · 时区 UTC | `throughput` 按 `DATE(created_at)` 分组时必须显式指定时区或假设 UTC;前端传 `from=2026-04-01` 应被解释为 UTC 日期边界(`[2026-04-01 00:00:00, 2026-04-02 00:00:00)`) |
| `module_key` 当前实装可能值(见 R3 `task_modules.module_key`):`task_detail` / `design` / `audit` / `customization` / `warehouse`(5 值 · 与 OpenAPI `module-dwell` enum 对齐)| SA-D `module-dwell` 聚合 GROUP BY 必须输出这 5 类中的每一类 · 某类无样本时 `samples=0, avg=0, p95=0`(不得缺行)· 保证前端固定卡片数量 |

---

## 3. 交付范围

### 3.1 Domain / 模型层

| 文件 | 作用 |
| --- | --- |
| `domain/search_result.go`(**新建**)| `SearchResultGroup` + 4 子结构 `SearchTask` / `SearchAsset` / `SearchProduct` / `SearchUser` · 字段严格对齐 OpenAPI §3.1(R1.7-D) |
| `domain/report_l1.go`(**新建**)| `L1Card` + `L1ThroughputPoint` + `L1ModuleDwellPoint` · 字段严格对齐 OpenAPI §3.3-§3.5(R1.7-D) |
| `domain/errors.go`(**追加 · 不改现有 code**)| 新增 `ErrDenyCodeReportsSuperAdminOnly = "reports_super_admin_only"` 常量 |

**严禁**:扩展 `NotificationType`(SA-C 冻结 5 值 · SA-D 不得碰);加 `task_module_events.event_type` 枚举值(R3 冻结);新建 `deny_code` 除本节列出的 1 个之外的任何值。

### 3.2 Service 层

| 包 | 文件 | 职责 |
| --- | --- | --- |
| `service/search`(**新建**)| `service.go` | `Search(ctx, actor, q string, scope string, limit int) (*domain.SearchResultGroup, *AppError)` · 按 `scope` 分支(`all` 执行全部 4 类;具体类仅执行该类)· **users 分支仅在 actor.Role ∈ {super_admin, hr_admin} 时执行**,否则 `users=[]` · `limit` 应用到每类数组(非总和)· 空 `q` → 400 `invalid_query` |
|  | `service.go` | `searchTasks(ctx, q string, limit int)` → `SELECT id, task_no, title, task_status, priority FROM tasks WHERE (task_no LIKE ? OR title LIKE ? OR CAST(id AS CHAR) LIKE ?) ORDER BY id DESC LIMIT ?` · `highlight` 字段 v1 为 `null`(B1 MySQL LIKE 不计相关性) |
|  | `service.go` | `searchAssets(ctx, q string, limit int)` → `SELECT asset_id, file_name, source_module_key, task_id FROM task_assets WHERE file_name LIKE ? AND lifecycle_state IN ('active','closed_retained','archived') ORDER BY asset_id DESC LIMIT ?`(`lifecycle_state='deleted'/'auto_cleaned'` 必须排除 · 对齐 SA-A 5 态机可见性) |
|  | `service.go` | `searchProducts(ctx, q string, limit int)` → 优先查 `erp_product_snapshots` 表(若存在);若不存在 fallback 到 `SELECT DISTINCT erp_code, '' AS product_name, NULL AS category FROM tasks WHERE erp_code LIKE ? LIMIT ?`;**不**实时调 ERP 上游 |
|  | `service.go` | `searchUsers(ctx, actor, q string, limit int)` → 首先判 `actor.Role ∈ {super_admin, hr_admin}`,否则直接 return `[]`;否则 `SELECT user_id, username, ... FROM users LEFT JOIN org_departments ON ... WHERE username LIKE ? OR nickname LIKE ? AND status='active' ORDER BY user_id DESC LIMIT ?` |
| `service/report_l1`(**新建**)| `service.go` | `Cards(ctx, actor) ([]domain.L1Card, *AppError)` · **进入前断言 `actor.Role = super_admin`**;非 SuperAdmin → `AppError{HTTP:403, Code:"reports_super_admin_only"}` |
|  | `service.go` | `Throughput(ctx, actor, from, to time.Time, deptID *int64, taskType *string) ([]domain.L1ThroughputPoint, *AppError)` · `from > to` → 400;同 SuperAdmin 门控;查 `task_module_events` + `tasks` JOIN 按 `DATE(created_at UTC)` 分组;`created` = 事件 type='created' 计数;`completed` = 事件 type='archived' 或 `task.task_status='archived'` 去重;`archived` = 同 completed(v1 简化 · IA §4 未区分)· 在报告中明示语义 |
|  | `service.go` | `ModuleDwell(ctx, actor, from, to time.Time, deptID *int64, taskType *string) ([]domain.L1ModuleDwellPoint, *AppError)` · SuperAdmin 门控;查 `task_module_events` 按 `module_key` 分组 · `avg_dwell_seconds = AVG(TIMESTAMPDIFF(SECOND, state_enter_at, state_exit_at))`;`p95` 用 `PERCENT_RANK` 或 `ROW_NUMBER`(MySQL 8)或 `LIMIT ... OFFSET` 近似;若 `state_exit_at IS NULL`(仍在该模块)不计入 · 报告明示 |
|  | `service.go` | 所有报表方法必须**只读** · 绝不写任何表 |

**严禁**:新建 `service/report_l2` / `service/report_l3` / `service/export_*` / `service/metrics_materialized_*` 任何包;改 R3 `service/task_module/*` 或 `service/module_action/*` 任何方法;查询 `task_module_events` 时写入同表。

### 3.3 Repo 层

| 文件 | 作用 |
| --- | --- |
| `repo/search_repo.go`(**新建**)| `SearchRepo` interface:`SearchTasks/SearchAssets/SearchProducts/SearchUsers` · 每方法签名含 `limit int` |
| `repo/mysql/search_repo.go`(**新建**)| MySQL 实现 · 显式列名 SELECT · `LIKE` 使用 `CONCAT('%',?,'%')` · 不 `SELECT *` · **每条查询必须附 `ORDER BY ... LIMIT ?` 保证稳定性 + P95** |
| `repo/report_l1_repo.go`(**新建**)| `ReportL1Repo` interface:`GetCards/GetThroughput/GetModuleDwell` |
| `repo/mysql/report_l1_repo.go`(**新建**)| MySQL 实现 · **禁止** `SELECT * FROM task_module_events`(300 行小 · 但生产数据增长后会爆);所有聚合必须在 SQL 里完成 · 不拉全表到 Go 内存 |

**严禁**:改 v0.9 / R3 / SA-A / SA-B / SA-C 任何已有 repo。

### 3.4 Transport 层

#### 3.4.1 替换 501 stub(4 条)

| handler | 路径 | 角色门控 |
| --- | --- | --- |
| `handleGlobalSearch` | `GET /v1/search` | 登录用户(`session_token_authenticated`)· users[] 低权空数组由 service 层处理 |
| `handleReportL1Cards` | `GET /v1/reports/l1/cards` | SuperAdmin only |
| `handleReportL1Throughput` | `GET /v1/reports/l1/throughput` | SuperAdmin only |
| `handleReportL1ModuleDwell` | `GET /v1/reports/l1/module-dwell` | SuperAdmin only |

#### 3.4.2 新建文件

- `transport/handler/search.go`(1 handler)
- `transport/handler/report_l1.go`(3 handler)

#### 3.4.3 路由挂载(`transport/http.go` 内追加 · 替换现有 501 stub)

```go
v1.GET("/search", withAuthenticated(...), searchH.Search)

v1.GET("/reports/l1/cards", withAuthenticated(...), reportL1H.Cards)
v1.GET("/reports/l1/throughput", withAuthenticated(...), reportL1H.Throughput)
v1.GET("/reports/l1/module-dwell", withAuthenticated(...), reportL1H.ModuleDwell)
```

**同时**:从 `transport/http.go` 的 `v1R1ReservedHandler` 列表中移除 `/v1/search` + 3 条 `/v1/reports/l1/*`(清理 dangling 501)· 这是 R1.7-D 补丁保留给本轮清的 4 条 501。

---

## 4. 审计事件清单(SA-D 必须落 / 不必落的写入)

SA-D 几乎**不写** `permission_logs`(全部是只读查询 · 没有跨租户敏感操作)。**但以下一种必须写**:

| 触发端点 | `permission_logs.action_type` | payload keys |
| --- | --- | --- |
| `GET /v1/reports/l1/*`(非 SuperAdmin 尝试 · 3 条报表各自记录)| `report_access_denied` | `{actor, path, reason: "not_super_admin"}` |

(搜索 `/v1/search` 任何登录用户都能调 · 不写审计 · users[] 权限边界由 service 层空数组处理 · 不上审计)

---

## 5. 执行环境

**复用 R3.5 + SA-A + SA-B + SA-C 的 `jst_erp_r3_test`**(在 `jst_ecs`):

```bash
ssh jst_ecs 'mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -p"$DB_PASS" -e "SHOW DATABASES LIKE '\''jst_erp_r3_test'\'';"'

DSN=$(ssh jst_ecs 'bash /root/ecommerce_ai/r3_5/build_test_dsn.sh')
MYSQL_DSN="$DSN" R35_MODE=1 /home/wsfwk/go/bin/go test ./service/search/... ./service/report_l1/... -tags=integration -count=1 -v
```

**严格守则**:
- 所有 integration test 必须走 `testsupport/r35.MustOpenTestDB(t)`,DSN 不以 `_r3_test` 结尾即 `t.Fatalf`
- SA-D 追加的测试样本:`user_id ≥ 50000` + `task_id ≥ 50000` + 新建 `task_module_events` 测试行的 `id` 段无特殊隔离但测试结束必须 `t.Cleanup` 清理
- 搜索 integration 允许 LIKE 命中其他段数据(历史样本)· 但**断言**必须针对本测试插入的 `task_id ≥ 50000` 行

---

## 6. 测试要求

### 6.1 单元测试(100% 绿)

- `service/search/service_test.go`:
  - 空 `q` → 400
  - 各 `scope` 分支(all/tasks/assets/products/users)路由正确
  - `limit` 生效(传 3 · 返回 ≤ 3)
  - **actor.Role=pb_normal → users=[]**(即使 `scope=users`)
  - `actor.Role=super_admin → users` 实际查询
  - `actor.Role=hr_admin → users` 实际查询
- `service/report_l1/service_test.go`:
  - 非 SuperAdmin → 403 `reports_super_admin_only` · 三 handler 各测
  - `from > to` → 400
  - Throughput 聚合:给定 10 条 `task_module_events`(3 天 · 各 3-3-4 条 created)· 断言返回 3 个 point · `created` 计数正确
  - ModuleDwell:`state_exit_at IS NULL` 不计入 · `avg/p95/samples` 正确

### 6.2 Integration Tests(build tag `integration`)

| # | 断言 | 对应验收 |
| --- | --- | --- |
| SA-D-I1 | `GET /v1/search?q=TASK50001&scope=all` 返回 tasks[] 含刚插入的 task_id=50001 · assets[]/products[]/users[] 按 actor 权限分支(actor=operator → users=[]) | IA §4.2 / R1.7-D Q1=A1 Q2=U1 |
| SA-D-I2 | 同一 q · actor=super_admin → users[] 非空(若命中);actor=hr_admin → 同;actor=operator → users=[] | R1.7-D Q2=U1 |
| SA-D-I3 | `limit=3` → 每数组最多 3 条(即使生产有更多命中) | R1.7-D §3.2 |
| SA-D-I4 | `scope=tasks` → assets/products/users 全返空数组;`scope=users` 低权 → users=[] 且其他数组也空 | R1.7-D §3.2 |
| SA-D-I5 | `q=""` → 400 `invalid_query`;`q=x`(1 字符)→ 200(v1 不限最短) | IA §4.2 |
| SA-D-I6 | `GET /v1/reports/l1/cards` 以 operator 调 → 403 `reports_super_admin_only` · 审计 `report_access_denied` 已写 | R1.7-D Q5=E1 |
| SA-D-I7 | SuperAdmin 调 `/cards` → 200 · `data` 是 `L1Card[]` · 每项含 `key/title/value` · 至少 3 张卡(tasks_in_progress / tasks_completed_today / archived_total) | R1.7-D §3.3 |
| SA-D-I8 | SuperAdmin 调 `/throughput?from=2026-04-20&to=2026-04-22` → 200 · `data[]` 每项含 `date/created/completed/archived` · 日期在 `[from, to]` 范围内(闭区间) | R1.7-D §3.4 |
| SA-D-I9 | SuperAdmin 调 `/module-dwell?from=...&to=...` → 200 · `data[]` 5 条(5 个 module_key 各 1 行 · 无样本时 samples=0)· `avg/p95` 非负数字 | R1.7-D §3.5 |
| SA-D-I10 | `from > to` → 400 `invalid_date_range` · 三个报表 handler 各测 | R1.7-D §3.4-§3.5 |
| SA-D-I11 | 搜索 P95:本地跑 100 次 `/v1/search?q=xxx&scope=all` · P95 < 1s(真生产可能 < 300ms · 测试库放宽);报告日志记录平均/中位 | IA-A3 |

### 6.3 全量回归

```bash
/home/wsfwk/go/bin/go build ./...
/home/wsfwk/go/bin/go test ./... -count=1

DSN=$(ssh jst_ecs 'bash /root/ecommerce_ai/r3_5/build_test_dsn.sh')
MYSQL_DSN="$DSN" R35_MODE=1 /home/wsfwk/go/bin/go test ./... -tags=integration -count=1

/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml  # 0 error 0 warning
```

### 6.4 生产 Probe(零污染证据 · 沿用 SA-A/B/C 模板)

Probe 脚本由主对话打包交付:`tmp/sa_d_probe_readonly.sh`(主对话在交付时已与本 prompt 校对列名);**本轮 Codex 不自行写 probe SQL** · 如脚本缺字段回报主对话。

执行两阶段:

```bash
# Pre(主对话在启动 Codex 前执行)· 固化 mysql_server_time_utc
# Post(SA-D 跑完后 · 传 pre 的时间作参数 · 硬门聚合期望全 0)
ssh jst_ecs "bash /tmp/sa_d_probe.sh '<pre_time>'" | tee docs/iterations/r4_sa_d_probe_post.log
```

Post 阶段 SA-D 控制字段聚合硬门(4 条 · 全 0):

| 脚本段 | 聚合 | 期望值 | 含义 |
| --- | --- | --- | --- |
| D1 | `SELECT COUNT(*) FROM task_module_events WHERE created_at >= :sa_d_start_time AND event_type NOT IN ('<R3 冻结枚举>')` | **0** | SA-D 未扩 event_type |
| D2 | `SELECT COUNT(*) FROM permission_logs WHERE created_at >= :sa_d_start_time AND action_type='report_access_denied'` | **0** | SA-D 未写生产审计(本轮生产不上 SA-D handler · 审计应为 0)|
| D3 | `SELECT COUNT(*) FROM tasks WHERE updated_at >= :sa_d_start_time AND task_status IS NULL` | **0**(示例 · 由 probe 脚本定稿)| SA-D 未误写 tasks · drift 允许但非空字段不变 null |
| D4 · 跨域回归 | SA-A 控制字段(`is_archived`) + SA-B 控制字段(`org_move_requests` 计数)+ SA-C 控制字段(`notifications` / `task_drafts` 在生产计数)与 SA-C pre-probe 对比 drift | **全部为 0**(SA-D 不跨域) | SA-D 不触碰其他域 |

**行数 drift 非 abort**(沿用 SA-A/B/C 裁决):生产表行数变化均为 live traffic · 不触发 abort;只要 D1~D4 聚合符合期望即通过。

---

## 7. 交付物清单

1. Domain 层 3 新文件(§3.1)
2. Service 层 2 新包(§3.2) · **包含 users[] 低权空数组闸 · 包含 SuperAdmin 门控 · 不含物化表 · 不含 ES / 不含导出**
3. Repo 层 4 新文件(§3.3)
4. Transport 层 2 新 handler 文件 + `transport/http.go` 4 路由挂载(§3.4) · **同时清理 4 条 dangling 501 映射**
5. 11 条 integration test(build tag `integration` · §6.2)+ 单测 2 组(§6.1)
6. `docs/iterations/V1_R4_SA_D_REPORT.md`:

**报告强制章节**:

- `## Scope` · 4 条 handler 清单 + 每条的 deny 路径 + 返回码矩阵
- `## §3 Search 实装` · 4 类 source 的 SQL/LIKE 策略 + users 低权空数组代码锚点 + `highlight` v1=null 声明
- `## §4 Report L1 实装` · 3 handler 的 SQL + `task_module_events` 聚合语义 + `created/completed/archived` 语义说明(v1 简化)
- `## §5 SuperAdmin 门控证据` · `actor.Role` 判定代码锚点 + 非 SuperAdmin 返回 403 + `reports_super_admin_only` 的 integration 证据
- `## 11 Integration Assertions` · SA-D-I1~I11 的实际 SQL / HTTP 数据证据
- `## Test DB Touch` · `jst_erp_r3_test` 中 SA-D 测试数据范围(`user_id ≥ 50000` + `task_id ≥ 50000`)· `t.Cleanup` 清理证据
- `## Production Probe Diff` · §6.4 四条控制字段聚合全 0 的 SQL + 输出
- `## Dangling 501 Cleanup` · `transport/http.go` 的 4 条 501 stub 移除 diff
- `## OpenAPI Conformance` · 0/0
- `## Performance` · `/v1/search` 100 次调用的 P95 / 平均 / 中位(IA-A3 证据)· 非 abort 门但必须报
- `## Known Non-Goals` · L2/L3 报表(R7+)· ES 切换(R6+)· 物化表(R6+)· 导出 CSV/Excel(R5+ 或之后)· `task_timeout` 相关指标 · 跨实例聚合(不做)

---

## 8. 失败终止条件

- 4 条 R4-SA-D 路径在 OpenAPI 里 schema 字段与 R1.7-D §3 对不上 → **回报主对话**(而非 abort 重起一轮;R1.7-D 已签字,残漏由主对话裁决)
- 任何 SA-D 代码触发写入(**只读轮 · 允许写的表只有 `permission_logs` 且只限 `report_access_denied`**)→ abort
- 任何 DSN 守卫失效(测试跑出 `jst_erp` 写入)→ abort
- integration 断言任一 FAIL
- 生产 probe **控制字段聚合**任一 ≠ 期望值(见 §6.4 表)→ abort;**表行数 drift 不算违反**
- 搜索服务写入 `task_module_events` 新 event_type → abort
- 任何报表 handler 未做 SuperAdmin 门控即 return 200 → abort
- 搜索 users 分支未做低权 scope 闸(即使 `actor.Role=operator` 也返回非空 users[])→ abort
- 引入 ES / Redis / Kafka / gRPC / 任何分布式依赖 → abort(v1 单 MySQL)
- 新建物化表(`task_metrics_l1` 等)或 migration → abort(Q6=C1 直查决议)
- 新增 `deny_code` 除 `reports_super_admin_only` 之外的值 → abort

---

## 9. 控制字段白名单(SA-D 允许写入的表/列)

| 表 | 允许写入的列 | 备注 |
| --- | --- | --- |
| `permission_logs` | 全部列(insert only) | 仅限 `report_access_denied` action_type |
| 其他所有表 | **零改动** | SA-D 是纯只读查询轮 |

**R3 / SA-A / SA-B / SA-C 代码触点**:
- SA-D 不得修改任何上述轮的文件(除了 `transport/http.go` 路由挂载区 · 允许 4 条 501 stub 替换为真 handler)
- SA-D 不得改 `domain/auth_identity.go` 现有字段 · 不得改 `service/identity_service.go` 方法

---

## 10. 给 Codex 的最后一句话

> SA-D 是 R4 P3 的最后一轮,前面 SA-A/B/C 已全部签字。
> 你的范围只有"**全局搜索 + 3 条 L1 报表**",用户管理 / 资产 / 任务 / 草稿 / 通知 / WebSocket / ERP / 设计源 全不归你。
> 这一轮是**纯只读**——除了 `permission_logs` 的 `report_access_denied` 审计,你不写任何表。
> 搜索用 MySQL LIKE,不引 ES;报表直查 `task_module_events` + `tasks`,不建物化表;users[] 在非 SuperAdmin/HRAdmin 角色下**永远空数组**,不可例外。
> 3 条报表 API **仅 SuperAdmin**——进方法第一行就断言,否则 403 `reports_super_admin_only`。
> 遇到需要改 OpenAPI / 改 DDL / 新建物化表 / 引 ES / 动 SA-A/B/C 目录的冲动,**立即 abort** 并报告,让主对话裁决。
> 不要在生产 `jst_erp` 上跑任何东西,测试库是 `jst_erp_r3_test`;控制字段聚合 4 条全 0 + 搜索 P95 数据记录才是零污染 + 可签证据,表行数漂移是正常 live traffic。

---

## 变更记录

| 版本 | 日期 | 变更 |
| --- | --- | --- |
| v1.0 | 2026-04-24 | 初稿 · 基于 R1.7-D v1.0 / IA v1.1 §4 / Module v1.2 §12 / SA-A/B/C 签字模板固化。4 handler 清单锁定(`x-owner-round: R4-SA-D`);搜索 users[] 低权空数组(Q1=A1 + Q2=U1);MySQL LIKE 技术栈(Q3=B1);报表直查 `task_module_events` + `tasks`(Q6=C1 · 不建物化表);L1Card description 修正(Q10);仅 SuperAdmin(Q5=E1);v1 不做导出(Q8=F1);Module §12 命名张力保持现状(Q9=R3);测试数据段 `user_id ≥ 50000` + `task_id ≥ 50000`;4 条 dangling 501 本轮清除。 |
