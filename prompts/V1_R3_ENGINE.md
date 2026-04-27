# V1 · R3 · Blueprint Engine + 三层权限 + 池/接单 + Task Cancel

> 版本:**v1**(2026-04-17 起草 · 基于 R2 v3 真生产落地结果)
> 状态:**待 Codex 执行**
> 依赖:R1(OpenAPI + 501 骨架)+ R2 v3(10 份迁移已在 `jst_erp` 生产落地)
> 执行环境:**本地 Docker MySQL + 从 R2 生产备份还原的 dump**(R3 是代码层,**不在生产执行**;生产验证推迟到 R6 UAT)
> 禁止前置:R2 v3 报告未签字 / 生产 `task_modules` 表为空时 · 不得启动

## 0. 本轮目标(一句话)

> **把 R1 冻结的 9 条 R3 handler 从 `501` 换成真业务实现**,落地 blueprint engine + 三层权限 + CAS 接单 + 取消语义 + 状态聚合;**不动 OpenAPI 合约**,**不改 R2 建的表结构**,所有动作事件走 `task_module_events`。

## 1. 必读输入

1. `docs/V1_MODULE_ARCHITECTURE.md` **v1.2**
   - §4.1(6 种 task_type 的 blueprint 编排)
   - §5(模块描述符 / ActionRegistry / StateMachine 定义)
   - §6(**三层权限矩阵 · 本轮唯一权威**)· §6.1 AuthorizeModuleAction 决策顺序 · §6.2 deny_code 枚举
   - §7(池与接单 · CAS SQL)
   - §10.1(derived_status 聚合规则)
   - §11.2(R2 backfill 已落地的 state 分布基线)
   - §13 A1~A10(验收表)
2. `docs/V1_ASSET_OWNERSHIP.md` v1.2
   - §3.4 参考图双写双读规则(R3 `update_reference_files` action 必须同时写展平表和 JSON 列)
   - §6.1 / §6.1a 真生产 source_module_key 分布(basic_info=9, customization=5, design=251)
3. `docs/V1_CUSTOMIZATION_WORKFLOW.md` v1.1
   - §3 定制模块内部节点 + audit 必过
   - §6.3 历史定制参考图路径
4. `docs/V1_INFORMATION_ARCHITECTURE.md` v1.1
   - §3(统一任务详情页对应字段)
5. `docs/iterations/V1_R2_REPORT.md` v1
   - Backfill Stats(source_module_key 真分布)
   - Implementation Notes(062 `ref_id` 列级 collation `utf8mb4_0900_ai_ci`)
6. R1 冻结的 `docs/api/openapi.yaml`
   - `GET /v1/tasks` · `POST /v1/tasks` · `GET /v1/tasks/{id}/detail` · `GET /v1/tasks/pool` · 5 条 `/modules/*` · `POST /v1/tasks/{id}/cancel`
   - 所有 R3 端点的 `x-owner-round: R3` · response schemas · deny_code 字段(严禁扩枚举)
7. `domain/enums_v7.go` · `domain/workflow_blueprints.go`(R1 已建的常量表)· `domain/module_*.go`(R1 scaffolding)
8. `db/migrations/059~068`(R2 真实 DDL · 列名为权威)
9. `cmd/tools/migrate_v1_backfill`(参考其 query.go 对新表的 SELECT 方式,保持一致)

禁止读:`docs/archive/*` · 任何 R1.5 之前的文档版本 · 已删除的 R2 v1 遗迹。

## 2. R3 真生产基线(从 R2 报告固化)

**Codex 实现任何读路径时必须认识到这些事实,否则 list/pool/detail 数据会失真**:

| 事实 | 影响 |
| --- | --- |
| R2 已在生产创建 `task_modules`(98 tasks × blueprint,实际 300 条模块行)、`task_module_events` 300 条、`reference_file_refs` 194 条 | 所有 R3 handler 假设这些表已有数据,不做初始化 |
| `task_modules.data.backfill_placeholder=true` 有 2 条占位模块 | **池查询和详情查询必须 `WHERE COALESCE(JSON_EXTRACT(data, '$.backfill_placeholder'), false) != true`**,避免把占位模块展示到前端 |
| `task_module_events` 的 origin 事件类型 `migrated_from_v0_9` / `backfill_placeholder` 由 R2 写入 | R3 时间线查询 / 详情页事件列表默认**包含**这两个类型,前端 UI 会把 `migrated_from_v0_9` 渲染为"历史迁移" |
| 生产 source_module_key 分布 `basic_info=9 / customization=5 / design=251` | 定制模块资产数 << customization_required task 数的 2×;R3 定制详情页不能预设"每个定制任务必有 2 张以上源文件" |
| `reference_file_refs.ref_id` 列级 collation 是 `utf8mb4_0900_ai_ci` | R3 写该表时 INSERT 的 `ref_id` 也必须保持该 collation;JOIN 到 `asset_storage_refs.ref_id` 时**不加** `COLLATE` 子句(自动匹配),加了反而报错 |
| `task_modules` / `task_module_events` 行 1:1 | R3 时间线查询可用 `INNER JOIN`,不用 `LEFT JOIN` |
| `tasks.priority` 4 值 `low / normal / high / critical` · 池排序用 `FIELD(priority,'critical','high','normal','low') ASC` | **严禁用 `priority DESC` 字典序排序**(会把 normal 排在 low 之前,正好相反) |

## 3. 交付范围

### 3.1 Service 层(新建 Go 文件)

| 包 | 文件 | 职责 |
| --- | --- | --- |
| `service/blueprint` | `registry.go` | 6 种 task_type → 模块序列的 BlueprintRegistry(硬编码常量,引自 `domain/workflow_blueprints.go`) |
|  | `rules.go` | inter-module 触发规则:`design.submit → audit.enter(pending_claim, pool=audit_standard)` / `audit.approve → warehouse.enter` / `audit.reject → customization.reopen(if customization blueprint) else design.reopen` 等(严格按主 §4.1) |
|  | `registry_test.go` | 每条 blueprint 至少 3 个事件的链式测试,覆盖率 100% 6 种 task_type |
| `service/module` | `descriptor.go` | ModuleDescriptor 结构(主 §5)· 聚合 ActionRegistry + StateMachine 到每个 module_key |
|  | `state_machine.go` | 按模块的 `(state, action) → next_state` 转移表 · 非法转移返回 `deny_code=module_state_mismatch` |
|  | `action_registry.go` | 每个模块的 action 列表 + 允许的 role filter(主 §6 Layer 3 示例为权威) |
|  | `state_machine_test.go` | 每模块每状态每 action 的转移断言 |
| `service/permission` | `authorize_module_action.go` | **`AuthorizeModuleAction(ctx, actor, task, module_key, action) Decision`** 统一入口 · Layer 1→2→3 串联 |
|  | `deny_code.go` | deny_code 常量 + 字符串 `"module_not_instantiated"`,`"module_out_of_scope"`,`"module_state_mismatch"`,`"module_action_role_denied"`,`"module_claim_conflict"`,`"module_blueprint_missing_team"`(与主 §6.2 一一对应,**严禁新增**) |
|  | `scope.go` | Layer 2 scope 判定(`SuperAdmin` / `DepartmentAdmin(D)` / `TeamLead(T)` / `Member(T1..Tn)` / 任务创建者 → basic_info);接单前后 scope 的收缩逻辑 |
|  | `authorize_test.go` | 6 角色 × 每模块每 action 的表驱动测试 |
| `service/task_pool` | `claim_cas.go` | CAS 接单 SQL(主 §7.2 原文)· `affected_rows == 0 → MODULE_CLAIM_CONFLICT`;成功后写 `task_module_events` `event_type=claimed` · 同事务 |
|  | `pool_query.go` | `GET /v1/tasks/pool` 查询 · 按 `FIELD(priority, ...) ASC, created_at ASC` 排序 · **必须过滤 backfill_placeholder** |
|  | `claim_cas_concurrent_test.go` | **100 goroutine 并发抢单** · 断言恰好 1 成功 99 返回 `module_claim_conflict` |
| `service/task_cancel` | `cancel_service.go` | `POST /v1/tasks/{id}/cancel` 双语义(主 §9.1.1):`reason=user_cancel` → 任务作废(写 `task_module_events.event_type=task_cancelled` + `tasks.task_status=Cancelled`);`reason=admin_close` → 强制关闭(所有未终态模块 → `state=closed_by_admin`) |
|  | `cancel_service_test.go` | 两条语义分支 + 权限断言(仅 creator 或 DepartmentAdmin 或 SuperAdmin) |
| `service/task_aggregator` | `status_aggregator.go` | `TaskStatusAggregator.Derive(taskID) DerivedStatus`,按主 §10.1 **v1.1 映射表**(24 值)从 `task_modules` 聚合 |
|  | `detail_aggregator.go` | `GET /v1/tasks/{id}/detail` 组装:tasks + task_details + task_modules + task_module_events(最近 50 条)+ task_customization_orders(LEFT JOIN,可空) |
|  | `list_query.go` | `GET /v1/tasks` · workflow_lane 按主 §4.1 分两栏;筛选参数完整覆盖 OpenAPI declared 字段;排序同 pool |

### 3.2 Transport 层(替换 R1 stub)

| handler | 路径 | 文件 |
| --- | --- | --- |
| `handleTasksList` | `GET /v1/tasks` | `transport/tasks_list.go` |
| `handleTasksCreate` | `POST /v1/tasks` | `transport/tasks_create.go`(调 blueprint.Init 为新任务实例化所有起始模块) |
| `handleTaskDetail` | `GET /v1/tasks/{id}/detail` | `transport/task_detail.go` |
| `handleTaskPool` | `GET /v1/tasks/pool` | `transport/task_pool.go` |
| `handleModuleClaim` | `POST /v1/tasks/{id}/modules/{module_key}/claim` | `transport/module_claim.go` |
| `handleModuleAction` | `POST /v1/tasks/{id}/modules/{module_key}/actions/{action}` | `transport/module_action.go`(走 blueprint.rules 触发下游模块;走 AuthorizeModuleAction) |
| `handleModuleReassign` | `POST /v1/tasks/{id}/modules/{module_key}/reassign` | `transport/module_reassign.go`(仅本组) |
| `handleModulePoolReassign` | `POST /v1/tasks/{id}/modules/{module_key}/pool-reassign` | `transport/module_pool_reassign.go`(仅部门管理员) |
| `handleTaskCancel` | `POST /v1/tasks/{id}/cancel` | `transport/task_cancel.go` |

**所有 handler 必须**:

- 从 `transport/http.go` 现有的 mux 复用路由注册位置(R1 已注册为 501,本轮把 handler 从 stub 切到真实现;不得新增 mux.Handle 行)
- 统一通过 `service/permission.AuthorizeModuleAction` 鉴权 · deny 响应码 `403` + body `{ "error": { "code": "<deny_code>", "message": "..." } }`
- `module_claim_conflict` 返回 **`409`**(不是 403,与主 §6.2 前端处理一致:toast)
- 失败响应 schema 严格匹配 OpenAPI 的 `ErrorResponse`

### 3.3 Repo 层(新建最小薄封装)

| 文件 | 作用 |
| --- | --- |
| `repo/task_module_repo.go` | GetByTaskAndKey / ListByTask / ClaimCAS / UpdateState / InsertPlaceholder |
| `repo/task_module_event_repo.go` | Insert / ListByTaskModule / ListRecentByTask |
| `repo/reference_file_ref_repo.go` | InsertFlat / ListByTask / DeleteByTaskAndRef(R3 仅写入,不删历史) |

**严禁**:修改任何现有 repo 文件(v0.9 业务表的 repo 保持不变);repo 里不得出现业务决策,决策全部在 service 层。

### 3.4 Domain 层(最小扩展)

| 文件 | 新增 |
| --- | --- |
| `domain/module_state.go` | 各模块 state 字符串常量(`ModuleStatePendingClaim` / `ModuleStateInProgress` / `ModuleStateSubmitted` / `ModuleStateClosed` / `ModuleStateClosedByAdmin` / ...) |
| `domain/module_event.go` | event_type 字符串常量(含 R2 backfill 已写入的 `migrated_from_v0_9`、`backfill_placeholder` 两个历史值 · 保留不删) |
| `domain/deny_code.go` | 与 `service/permission/deny_code.go` 对偶的纯字符串常量(供 transport 直接引用,不产生循环依赖) |

**严禁**:改 `domain/enums_v7.go` 的 `TaskStatus` 值域;改 `domain/reference_file_ref.go` 结构。

## 4. 三层权限实现细节

### 4.1 AuthorizeModuleAction(唯一入口)

```go
// 伪码 · 严格按主 §6.1 四步
func AuthorizeModuleAction(ctx, actor, taskID, moduleKey, action) Decision {
    task, err := repo.TaskByID(taskID)
    if task == nil { return Deny("not_found", "task not found") }

    module, err := repo.TaskModule.GetByTaskAndKey(taskID, moduleKey)
    if module == nil { return Deny(DenyModuleNotInstantiated, ...) }
    // Layer 1 Visibility 恒 allow(v1 无字段脱敏)
    // Layer 2 Scope
    if !ScopeAllows(actor, task, module) {
        return Deny(DenyModuleOutOfScope, ...)
    }
    // Layer 3 Action Gate
    desc := module.Descriptor()
    allowed, denyReason := desc.ActionRegistry.Check(action, module.State, actor)
    if !allowed {
        return Deny(denyReason, ...)  // module_state_mismatch 或 module_action_role_denied
    }
    return Allow()
}
```

### 4.2 Scope 判定细节(主 §6 + 边界规则)

- `basic_info` 模块的组长 scope:用 **tasks.creator_team_code** 匹配当前用户的 team_codes
- 非 basic_info 模块的组长 scope(接单后):用 **task_modules.claimed_team_code**
- 非 basic_info 模块的组长 scope(接单前):用 **task_modules.pool_team_code**
- 任务创建者对 `basic_info` **永久 in scope**(即使换了部门)

### 4.3 claim 特殊路径

claim 动作不走 `AuthorizeModuleAction` 的 Layer 3 `state → state` 转移(因为 CAS SQL 本身就是原子并发控制)。流程:

1. Layer 1 + Layer 2 通过(`pool_team_code ∈ actor.TeamCodes`)
2. 跳过 Layer 3 的 state 检查,直接跑 CAS SQL
3. affected_rows 决定成功 / `module_claim_conflict`(409)

## 5. 池查询实现要点

```sql
-- GET /v1/tasks/pool?module_key=design&pool_team_code=design_standard
SELECT tm.task_id, tm.module_key, tm.pool_team_code,
       t.priority, t.created_at, t.task_type, t.task_no,
       td.title, td.product_code
  FROM task_modules tm
  JOIN tasks t ON t.id = tm.task_id
  LEFT JOIN task_details td ON td.task_id = t.id
 WHERE tm.state = 'pending_claim'
   AND tm.pool_team_code IN (:actor_team_codes)
   AND (:module_key IS NULL OR tm.module_key = :module_key)
   AND COALESCE(JSON_EXTRACT(tm.data, '$.backfill_placeholder'), CAST('false' AS JSON)) != CAST('true' AS JSON)
 ORDER BY FIELD(t.priority, 'critical', 'high', 'normal', 'low') ASC,
          t.created_at ASC
 LIMIT :limit OFFSET :offset
```

## 6. 测试要求

### 6.1 单元测试(必须 100% 绿)

- `service/blueprint/*_test.go`:6 种 task_type blueprint + 每种链式 3+ 事件转移
- `service/module/state_machine_test.go`:覆盖每个 (module, state, action) 组合
- `service/permission/authorize_test.go`:**6 角色 × 每模块 × 每 action** 表驱动
- `service/task_cancel/cancel_service_test.go`:两分支 + 3 种角色组合
- `service/task_aggregator/status_aggregator_test.go`:24 种 task_status 推导都命中

### 6.2 100 线程 CAS 抢单测试(主 A5)

`service/task_pool/claim_cas_concurrent_test.go`:

```go
// 伪码
func TestClaimCAS_100Concurrent(t *testing.T) {
    ctx := setupDB(t)  // 在 Docker MySQL 中建一条 pending_claim module
    var wg sync.WaitGroup
    var successCount int64
    var conflictCount int64
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(actorID int64) {
            defer wg.Done()
            dec := svc.Claim(ctx, taskID, moduleKey, actorID)
            if dec.OK { atomic.AddInt64(&successCount, 1) }
            if dec.DenyCode == "module_claim_conflict" { atomic.AddInt64(&conflictCount, 1) }
        }(int64(i + 1))
    }
    wg.Wait()
    assert.Equal(t, int64(1), successCount)
    assert.Equal(t, int64(99), conflictCount)
}
```

### 6.3 Integration Test(build tag `integration`)

环境:**Docker MySQL 8.0 + 还原 R2 报告里的 `20260424T024501Z_r2_pre_backfill.sql.gz`**(从 `jst_ecs` scp 下来)· 然后再跑一次 R2 forward + backfill 把 R2 表灌上数据 · **不连生产**。

最低断言:

1. `GET /v1/tasks/pool?pool_team_code=design_standard` 返回的行**不含** `backfill_placeholder=true` 的模块
2. `POST /v1/tasks/{id}/modules/design/claim` 对同一模块两次调用:第 1 次 200,第 2 次 409 deny_code=`module_claim_conflict`
3. `POST /v1/tasks/{id}/modules/audit/actions/approve` 成功后:audit state 变 `closed`,warehouse 模块 state 变 `pending_claim`(blueprint rules 联动)+ 生成 `task_module_events.event_type=approved`(audit 侧)+ `entered`(warehouse 侧)共 2 条事件
4. `POST /v1/tasks/{id}/cancel` with `reason=user_cancel`:tasks.task_status → `Cancelled`,`task_module_events.event_type=task_cancelled`
5. `GET /v1/tasks/{id}/detail` 返回的 `modules[]` 数量 = blueprint 期望数量;每个 module 的 `visibility=visible`(Layer 1)
6. `GET /v1/tasks` 排序:插入两条优先级分别为 `low` 和 `high` 的任务,返回顺序是 high 在前 low 在后(验证 FIELD 排序)

缺失 `MYSQL_DSN` 时 `t.Skip`,退出码 0。

### 6.4 OpenAPI 契约一致性

- 所有 R3 handler 的 request / response body 必须能被 R1 冻结的 `docs/api/openapi.yaml` schema 校验通过
- `deny_code` 字符串值域与 OpenAPI `ErrorResponse.code` 的 enum 列表(如有)完全一致
- `go run ./cmd/tools/openapi-validate docs/api/openapi.yaml` 仍 0 error 0 warning(R3 不改 OpenAPI)

### 6.5 全量回归

```bash
go build ./...
go test ./... -count=1                      # 全绿(允许 config 包因 Windows AppControl skip)
go test ./... -count=1 -tags=integration    # 在 Docker MySQL 下跑
go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
```

## 7. 严禁触碰(DO NOT TOUCH)

- `docs/api/openapi.yaml`(R1 冻结;R3 如发现 schema 不足必须 abort 并上报,不得自行扩)
- `db/migrations/001~068`(R2 已上线;R3 不加新迁移)
- `service/**` 里任何现有 v0.9 业务文件(只新建 R3 新包,不改旧包)
- `repo/**` 现有文件(只新建 3 个 R3 repo)
- R2 落地的 7 张表的 DDL(只读 + 按主 §6/§7 定义写)
- 生产 `jst_erp`(**R3 代码层轮次,不执行生产 SSH 操作**;若开发者出于测试想连 jst_erp,必须**只读**,且 PR 不得包含任何生产连接凭证)
- `tasks.task_status` 的直写路径(仅 `TaskStatusAggregator` 可经由服务层钩子写;不得在 handler 里直接 UPDATE)
- R2 backfill 写入的 `migrated_from_v0_9` / `backfill_placeholder` 历史事件(保留不删)

## 8. 执行环境

### 8.1 本地 Docker MySQL(R3 integration test 唯一环境)

```bash
# 从生产拉 R2 pre-backfill dump(读权限足够)
scp jst_ecs:/root/ecommerce_ai/backups/20260424T024501Z_r2_pre_backfill.sql.gz ./tmp/

# 起 Docker MySQL 8.0
docker run -d --name r3-mysql \
  -e MYSQL_ROOT_PASSWORD=root \
  -e MYSQL_DATABASE=jst_erp \
  -p 3307:3306 mysql:8.0

# 等 MySQL ready
until docker exec r3-mysql mysqladmin ping -proot --silent; do sleep 1; done

# 灌 dump
gunzip -c ./tmp/20260424T024501Z_r2_pre_backfill.sql.gz | \
  docker exec -i r3-mysql mysql -uroot -proot jst_erp

# 再跑 R2 forward + backfill 把 R2 表状态追平生产
export MYSQL_DSN='root:root@tcp(127.0.0.1:3307)/jst_erp?parseTime=true&multiStatements=true'
go run ./cmd/tools/migrate_v1_forward --dsn="$MYSQL_DSN"
go run ./cmd/tools/migrate_v1_backfill --dsn="$MYSQL_DSN"
```

### 8.2 在 Docker 环境运行 R3 integration tests

```bash
MYSQL_DSN='root:root@tcp(127.0.0.1:3307)/jst_erp?parseTime=true&multiStatements=true' \
  go test ./... -tags=integration -count=1 -v
```

### 8.3 开发过程中 Docker 重置

```bash
docker rm -f r3-mysql && bash ./8.1 重来
```

Codex 不得使用 ephemeral in-memory DB 或 sqlite 桩绕过 MySQL 特性(如 JSON_EXTRACT / FIELD 函数)。

## 9. 交付物清单

1. `service/blueprint/{registry,rules,registry_test,rules_test}.go`
2. `service/module/{descriptor,state_machine,action_registry,state_machine_test}.go`
3. `service/permission/{authorize_module_action,deny_code,scope,authorize_test}.go`
4. `service/task_pool/{claim_cas,pool_query,claim_cas_concurrent_test}.go`
5. `service/task_cancel/{cancel_service,cancel_service_test}.go`
6. `service/task_aggregator/{status_aggregator,detail_aggregator,list_query,*_test}.go`
7. `repo/{task_module_repo,task_module_event_repo,reference_file_ref_repo}.go`
8. `domain/{module_state,module_event,deny_code}.go`
9. 9 个 `transport/*.go` handler(替换 R1 stub)
10. `transport/*_integration_test.go`(build tag integration,6 断言)
11. `docs/iterations/V1_R3_REPORT.md` · 强制章节:

   - `## Blueprint Coverage`:6 种 task_type 的链式转移测试覆盖证明
   - `## AuthorizeModuleAction Matrix`:6 角色 × 模块 × action 的测试覆盖率表
   - `## CAS Concurrency`:100 线程测试结果 (1 成功 / 99 conflict)
   - `## Integration Results`:6 断言 PASS/FAIL 证据(基于 Docker + 生产 dump 还原)
   - `## OpenAPI Conformance`:9 个 handler 的 request/response 对 OpenAPI schema 校验结果
   - `## Production Touch`:明确声明 **R3 未触碰生产 `jst_erp`**
   - `## Known Non-Goals`:R3 未实现的 R4/R5 范围清单(避免越界)

## 10. 失败终止条件(Codex 必须 abort)

- 无法从 `jst_ecs` 拉到 R2 pre-backfill dump(权限 / 文件不存在)
- Docker 环境不可用且无替代的真实 MySQL 8.0
- R1 OpenAPI 中 R3 端点的 request/response schema 与主 §6 / §7 / §9.1 定义不一致
- 某个 deny_code 需要新加值才能通过测试(说明主 §6.2 或 R1 OpenAPI 漏项,R3 无权扩)
- 100 线程 CAS 测试出现 2 个以上成功(意味着 CAS SQL 没生效)
- 测试发现 `pool_team_code` 字段在 `task_modules` 不存在或类型不匹配(意味着 R2 migration 059 有偏差,回给 R2)
- 有任何 UPDATE / INSERT 动作误发到 `jst_erp` 生产

## 11. 给下游的交接约束

### 11.1 给 R4(4 subagent)

- R4-SA-A(资产中心)依赖 R3 的 `task_module_events` 事件订阅方式(`UpdatedSince(ts)` 查询)· R3 报告必须枚举已落地的 event_type 列表
- R4-SA-B(组织 / 个人中心)依赖 R3 的 `AuthorizeModuleAction` 作为 scope 查询基础(`me/pool` 可转发)
- R4-SA-C(通知)订阅 `task_module_events`,事件载荷 payload 结构必须在 R3 报告附录定义
- R4-SA-D(搜索 / 报表)依赖 R3 的 `list_query.go` 作为主列表契约,不再自造 SQL

### 11.2 给 R5(前端)

- 所有 R3 handler 的真实响应 sample 必须写入 R3 报告,供前端 mock
- deny_code 的前端处理策略已在主 §6.2 固化,R5 不得重新定义

---

## 12. 给 Codex 的最后一句话

> R2 已经把**数据层**放稳了。R3 是**第一次业务逻辑真正上链**:blueprint + 权限 + CAS + 聚合。
> 口径对齐的压力不在 DDL 而在"**三层权限矩阵的一致性**"——deny_code、scope 判定、action gate 三处若任意一处偏离主 §6,前端会立刻显出错乱的按钮态。
> 出现任何本文未覆盖 / 与主 §6 或 §7 不一致的场景,**立即 abort**,写日志报告,不要自决。
> 不要在生产上跑任何 R3 代码。

---

## 变更记录

| 版本 | 日期 | 变更 |
| --- | --- | --- |
| v1 | 2026-04-17 | 基于 R2 v3 真生产落地结果起草:9 handler 清单锁定(R1 OpenAPI `x-owner-round: R3` 精确 9 条);三层权限统一入口 `AuthorizeModuleAction`;CAS 100 线程并发测试为硬门;池查询强制过滤 `backfill_placeholder`;集成测试环境限 Docker + 生产 dump 还原,不连生产;drafts 端点归 R4-SA-C(与本轮无关);deny_code 6 值严禁扩。配套文档 `V1_MODULE_ARCHITECTURE.md` v1.2 + `V1_ASSET_OWNERSHIP.md` v1.2 + R2 报告 |
