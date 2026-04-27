# V1 模块化架构设计文档

> 状态: **v1.3 · R4-SA-D 实装校正已签字生效**(2026-04-25,依据 `docs/iterations/V1_R4_SA_D_REPORT.md` §Architect Adjudication);前置版本 v1.2 / v1.1 / Draft v4 已签字生效
> v1.3 变更(R4-SA-D):§17 追加 `报表数据源实装校正(SA-D)` 行 · 锁定 L1 报表 dwell 数据源为 `task_module_events JOIN task_modules JOIN tasks`(module_key/task_id 在 task_modules 侧);生产 `task_module_events` 无 `state_enter_at`/`state_exit_at` 列 · dwell 用 CTE 配对 enter-like/exit-like event + window rank P95;throughput.archived 在 v1 与 completed 同集合(v2+ 再拆分);`users[]` 除 SuperAdmin+HRAdmin 外返空。
> v1.2 变更(R1.6):§8.2 + §17 Q7.5 行的 priority 枚举从 `normal | urgent | critical`(3 值)修订为 **`low | normal | high | critical`**(4 值);真生产 `jst_erp` `tasks.priority` 实测分布 `low(56) / high(20) / normal(19)`,3 值枚举会让 067 CHECK 约束直接拒 76 行。v1.2 保留现状 4 值,不做 `low→normal` / `high→urgent` 语义归一化;排序推荐用 `FIELD(priority,'critical','high','normal','low') ASC` 显式权重。依据 `docs/iterations/V1_R1_6_PROD_ALIGN.md`。
> 最终签字版本:Draft v4(含 FE Plan 评审回写:task-cancel / audit 参考图 / 草稿端点口径)
> 作者: 后端架构组
> 适用版本: v1.1(本轮重构)→ v1.x 持续迭代
> 权威级别: 本文一经签字即成为 **与 `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`、`docs/api/openapi.yaml` 同级** 的架构权威。
> 三者冲突时:`openapi.yaml` 决定 HTTP 契约;`V0_9_BACKEND_SOURCE_OF_TRUTH.md` 决定兼容策略;本文决定模块结构、权限矩阵、状态推导规则。
>
> 本文不描述具体 SQL / Go 代码,只规定**结构、契约、边界、迁移策略**。R1 ~ R6 的落地 prompt 会逐轮引用本文的章节号。
>
> **关联子文档(v2 新增,本文为骨架,细节见子文档)**:
> - `docs/V1_CUSTOMIZATION_WORKFLOW.md` — 定制任务模块内部流程细节
> - `docs/V1_ASSET_OWNERSHIP.md` — 资产归属、版本锚定与生命周期
> - `docs/V1_INFORMATION_ARCHITECTURE.md` — 菜单树、个人中心、用户管理、全局搜索

---

## 0. 文档目的与读者

| 读者 | 需要读的章节 |
| --- | --- |
| 后端研发 | 全部 |
| 前端研发 | 1, 2, 3, 4, 6, 7, 9, 10 |
| 产品 / 运营 | 1, 2, 3, 4, 6, 7 |
| DBA / 运维 | 8, 11, 14 |

---

## 1. 背景与目标

### 1.1 现状痛点(v0.9 / v1.0)

1. **权限横切无法收敛**:`TaskAction` 已膨胀到 50+,每新增一种任务类型或阶段,需同步改动 `service/task_action_rules.go`、`service/task_action_authorizer.go`、`service/data_scope.go`、`service/task_data_scope_guard.go`、`config/frontend_access.json` 等 5+ 文件,极易漏改导致 403 / 幽灵可见性。
2. **工作台割裂**:审核工作台、设计工作台、云仓工作台分别持有一套独立的读模型、入口、上传入口,数据口径不统一(如 `uploader_name`、`reference_file_refs` 的呈现曾三次出现回归)。
3. **任务类型与流程强耦合**:`WorkflowLane` 目前只有 `normal` / `customization` 两档,但实际业务需要 **原款开发、新款开发、采购、精修、客户定制、常规定制** 至少 6 条不同的流程路径,用 lane 枚举扩展会再次回到"每加一种就改一圈"的泥潭。
4. **"所有人默认可见任务" 与 "只能在自己组操作" 无法在现有 DataScope 中同时表达**。现模型是"可见 ≈ 可操作",违背组长只对接单后的任务管理的业务语义。
5. **组长口径不清**:运营组长按**创建者**划分,设计组长 / 审核组长按**接单者**划分,旧代码里没有这个概念,导致组长看板数据不稳。

### 1.2 目标(本轮必须达到)

| # | 目标 | 验收指标 |
| --- | --- | --- |
| G1 | **Task = 容器,Module = 工作单元**,新增任务类型只需组合现有模块 | 新增一条工作流无需改 `task_action_rules.go`,仅配置 |
| G2 | 前端**任务详情页**一屏呈现基本信息 / 设计 / 审核 / 云仓 / 定制 / 采购 6 个模块,模块内部按角色可见/可操作 | 详情页只调 1 个 `GET /v1/tasks/{id}/detail` |
| G3 | 所有用户**默认看到所有任务**(列表 / 详情),无字段脱敏;**模块内操作**按三层权限矩阵门控 | 审计日志不再出现 `deny_code=task_out_of_team_scope`(改为 `module_out_of_scope`) |
| G4 | 引入**任务池 + 接单**模型,设计 / 审核按**组-任务类型严格绑定**领取 | 常规任务只进"常规设计组池",定制任务只进"定制美工组池 / 定制审核组池";跨组仅部门管理员可调度 |
| G5 | **一次性切换**完成数据迁移(允许 5 ~ 10 min 停服窗口);不保留双写 | 停服窗口内 migration + backfill + smoke 全绿 |
| G6 | 所有核心决策**在本文档可溯源**,后续每次需求变更先改本文再改代码 | 本文版本号与 migration 版本一一对应 |

### 1.3 非目标(本轮不做,但预留接口)

- 超时升级(escalation)**只预留接口**,不实现调度器。
- 报表 / 统计模块**只预留事件流**,不实现聚合视图。
- 移动端 / OpenAPI 外部开放。
- 多租户 / 多站点。

---

## 2. 核心概念模型

### 2.1 三层对象

```
+------------------------------------------------------+
|                      Task (容器)                      |
|  - 身份: id, 任务编号, 任务类型 (TaskType)              |
|  - 业务: 商品 SKU 锚、客户信息(仅客户定制)              |
|  - 元数据: 创建者、创建者所在部门/组、workflow_blueprint |
|  - 派生状态: status (= 所有 Module 状态聚合,只读)        |
+----------+------------------+--------------------+---+
           |                  |                    |
           v                  v                    v
   +---------------+  +---------------+    +---------------+
   | Module: Design|  | Module: Audit |    | Module: ...    |
   |  - module_key |  |  - module_key |    |                |
   |  - state      |  |  - state      |    |                |
   |  - claimed_by |  |  - claimed_by |    |                |
   |  - org_snap   |  |  - org_snap   |    |                |
   +-------+-------+  +-------+-------+    +-------+-------+
           |                  |                    |
           v                  v                    v
   +---------------+  +---------------+    +---------------+
   | ModuleEvent   |  | ModuleEvent   |    | ModuleEvent   |
   | (不可变事件流) |  | (不可变事件流) |    | (不可变事件流) |
   +---------------+  +---------------+    +---------------+
```

### 2.2 关键转变

| 旧模型 | 新模型 |
| --- | --- |
| `task.status` 是一等字段,代码直接 UPDATE | `task.status` **派生只读**,由视图 `v_task_status_derived` 或服务层 `TaskStatusAggregator` 聚合 |
| `TaskAction` 50+ 枚举,平铺在一个全局列表 | **`Module × ModuleAction × ModuleState`** 三元组,每个 Module 维护自己的 ActionRegistry |
| 权限 = `frontend_access.actions` ∋ `task.<action>` | 权限 = (Layer 1 可见性) ∧ (Layer 2 模块作用域) ∧ (Layer 3 模块门控) |
| 工作流 = `WorkflowLane` 枚举 | 工作流 = `WorkflowBlueprint`:任务类型 → 模块列表 + 模块间触发规则 |
| 任务分配 = `reassign` 直改字段 | 任务 = 进入**池**(`pending_claim` 模块)+ **接单**(CAS 原子领取) |

---

## 3. 模块目录(Module Registry)

本轮落地 **7 个模块**。每个模块都是**独立的有限状态机**,拥有自己的 `state`、`actions`、`events`、`data_projection`。

| module_key | 中文名 | 归属部门 | 生命周期 | 说明 |
| --- | --- | --- | --- | --- |
| `basic_info` | 基础信息 | 运营部 | 任务全生命周期 | 承载任务编号、SKU 锚、业务字段;始终存在;由创建者 / 运营组长维护 |
| `design` | 设计 | 设计研发部 | 任务创建时由 blueprint 决定是否实例化 | 状态:`pending_claim → in_progress → submitted → closed`(可被驳回重入) |
| `audit` | 审核 | 审核部 | 同上 | 状态:`pending_claim → in_progress → approved / rejected → closed`;支持一审/二审子状态 |
| `warehouse` | 云仓 | 云仓部 | 产品类任务的终态模块 | 状态:`pending → preparing → received → completed / rejected` |
| `customization` | 定制 | 定制美工部 | 仅客户定制 / 常规定制任务 | 负责效果图、客户沟通稿确认;与 `design` 可并行或替代 |
| `procurement` | 采购 | 采购部 | 仅采购任务 | 独立闭环,不经过设计/审核 |
| `retouch` | 精修 | 设计研发部(精修组) | 仅精修任务 | 纯设计,不进审核 |

### 3.1 模块的标准结构

每个 Module 在代码层必须满足如下接口(R1 需落地):

```
ModuleDescriptor {
  Key           string                // e.g. "design"
  Department    Department            // 归属部门(决定默认作用域)
  States        []ModuleState         // 合法状态集
  InitialState  ModuleState           // blueprint 启用该模块时的起始态
  TerminalStates []ModuleState        // 进入该态即冻结,不再触发事件
  Actions       []ModuleActionSpec    // 该模块可能触发的 action
  Projections   []ProjectionKey       // 详情页该模块呈现的数据块 key
}
```

### 3.2 模块间的触发(Blueprint Rules)

模块之间不直接相互 UPDATE,而是通过 `WorkflowBlueprint` 声明**事件 → 触发动作**:

示例(原款开发):

```
design.submitted          → audit.enter(initial_state=pending_claim)
audit.approved            → warehouse.enter(initial_state=pending)
audit.rejected            → design.reopen(reset_state=in_progress)
warehouse.received        → task.close
```

R1 阶段 blueprint **落在代码里的常量**(`domain/workflow_blueprints.go`),R3 阶段再考虑是否外置成配置。

---

## 4. 任务类型 × 模块编排

本轮明确 **6 种任务类型**,全部通过 blueprint 组合现有模块实现,未来新增只需加一条 blueprint。

| TaskType | 中文名 | Blueprint(模块流转) | 备注 |
| --- | --- | --- | --- |
| `original_product_development` | 原款开发 | `basic_info` → `design` → `audit` → `warehouse` | 当前默认流程 |
| `new_product_development` | 新款开发 | `basic_info` → `design` → `audit` → `warehouse` | 与原款同 blueprint,差异仅在元数据/分类/编号规则 |
| `purchase_task` | 采购任务 | `basic_info` → `procurement` → `warehouse` | **跳过设计 / 审核** |
| `retouch_task` | 精修任务 | `basic_info` → `retouch` → `warehouse` | **只经过设计,不经审核**;`retouch` 模块的设计者来自精修组 |
| `customer_customization` | 客户定制 | `basic_info` → `customization` → `audit` → `warehouse` | 元数据含线上订单号、ERP 产品编码、下单时间、客户需求附件;**不经过 `design`**,**不经过客户确认**(定制美工直接出终稿,交审核) |
| `regular_customization` | 常规定制 | `basic_info` → `customization` → `audit` → `warehouse` | 与客户定制同 blueprint,元数据含"设计源文件查询结果 + 需求说明";**不经过 `design`**;编号前缀不同 |

> **说明 4.1**:`retouch_task` 选择**不走 audit**,业务语义为精修是对成品图的二次润色,按"谁精修谁负责"的惯例直接进仓。若后续要补审核,只需往该 blueprint 加一条 `retouch.submitted → audit.enter` 即可,无需改枚举。
>
> **说明 4.2(v2 修订)**:`customer_customization` 和 `regular_customization` **不启用 `design` 模块**。定制美工组直接负责从沟通稿到终稿的全部产出,审核环节由定制审核组承担。两类任务的 blueprint 完全一致,差异仅在:(a) 任务编号前缀;(b) 创建时元数据来源(客户定制从 ERP Bridge `/open/combine/sku/query` 拉取产品编码;常规定制走"设计源文件查询")。详见 `docs/V1_CUSTOMIZATION_WORKFLOW.md`。

### 4.1 任务类型 → 接单池 → 组 映射

| TaskType | `design`/`retouch` 池归属 | `audit` 池归属 | `customization` 池归属 |
| --- | --- | --- | --- |
| `original_product_development` | 常规设计组 | 常规审核组 | — |
| `new_product_development` | 常规设计组 | 常规审核组 | — |
| `purchase_task` | — | — | — |
| `retouch_task` | 精修组 | — | — |
| `customer_customization` | — | 定制审核组 | 定制美工组 |
| `regular_customization` | — | 定制审核组 | 定制美工组 |

**严格性约束**:

- 任务进入池时,`pool_team_code` 依据上表写死,**不接受运行时选组**。
- 跨组领取 **禁止**,唯一例外:部门管理员可通过"部门管理员调度"手动把一条任务从 A 组池挪到 B 组池(视作"重新入池",记审计事件 `pool_reassigned_by_admin`)。

---

## 5. 组织结构与身份模型

### 5.1 部门与组(canonical 命名)

沿用 `domain/auth_identity.go` 已有枚举,本文落地如下**组(team_code)**:

| Department | 组 team_code | 组中文名 | 接管的模块 |
| --- | --- | --- | --- |
| `设计研发部` | `design_standard` | 常规设计组 | `design`(原款/新款) |
| `设计研发部` | `design_retouch` | 精修组 | `retouch` |
| `审核部` | `audit_standard` | 常规审核组 | `audit`(原款/新款) |
| `审核部` | `audit_customization` | 定制审核组 | `audit`(定制) |
| `定制美工部` | `customization_art` | 定制美工组 | `customization` |
| `云仓部` | `warehouse_main` | 云仓默认组 | `warehouse` |
| `采购部` | `procurement_main` | 采购默认组 | `procurement` |
| `运营部` | `operations_*`(按业务线) | 运营组 | `basic_info`(创建者) |

> **v2 修订**:原先草拟的 `design_customization`(定制设计组)**取消**。定制任务不经过 `design` 模块,无需独立的定制设计组。

> **注 5.1**:R1 阶段通过 migration 建立上述 team_code 的种子数据(`org_teams` 表),缺失则任务 `enter module` 失败并返回 `MODULE_BLUEPRINT_MISSING_TEAM`。

### 5.2 身份画像(Actor Profile)

在 R1 之后,每个请求的 `actor` 在服务层应持有以下**完整画像**:

```
ActorProfile {
  UserID            int64
  Department        Department              // 主属部门
  TeamCodes         []string                // 可能属于多个组
  Roles             []Role                  // DepartmentAdmin/TeamLead/Member/...
  ManagedDepartments []Department           // 仅部门管理员
  ManagedTeamCodes  []string                // 仅组长(自己带的组)
  FrontendActions   map[string]struct{}     // 沿用 frontend_access.json 计算结果
}
```

> **注 5.2**:`TeamCodes` 可以是多个。一个用户同时在"常规设计组"和"精修组"是合法的,他能看到两个组的待接单池并都可接单。

### 5.3 角色层级(简化)

| Role | 说明 | 跨部门/跨组能力 |
| --- | --- | --- |
| `SuperAdmin` | 平台超管 | 全部 |
| `DepartmentAdmin` | 部门管理员 | 限定在 `ManagedDepartments` 内,可跨组调度 |
| `TeamLead` | 组长 | 限定在 `ManagedTeamCodes` 内 |
| `Member` | 成员 | 仅自己承接/创建的任务模块 |

---

## 6. 三层权限矩阵

> 本章是整个 v1 权限系统的**唯一权威**。`frontend_access.json` 仅用于**前端菜单开关**,不再承担细粒度授权。

### Layer 1 · 可见性(Visibility)

**所有登录用户对所有任务可见**,无字段脱敏,无列表过滤。

- `GET /v1/tasks` 默认返回全量(筛选依赖参数,而非 scope)。
- `GET /v1/tasks/{id}/detail` 对登录用户**一律 200**(除非任务已硬删除)。
- 详情中每个 **Module 的 projection** 携带 `visibility: visible`(本层永远 visible)。

### Layer 2 · 模块作用域(Module Scope)

判断"**当前用户对这个 Module 是否属于作业范围**"。不属于作业范围 ≠ 不可见,而是**只读/不可操作**。

| 角色 | 在什么情况下对该 Module "in scope" |
| --- | --- |
| `SuperAdmin` | 恒 in scope |
| `DepartmentAdmin(D)` | 该 Module 的 `department == D` 且 Module 所属组 ∈ D |
| `TeamLead(T)` | 1) Module 已被领取且 `module.claim.team_code == T` **或** 2) Module 处于 `pending_claim` 且 `module.pool_team_code == T`(看板能看到待接单池) |
| `Member(T1…Tn)` | 1) Module 已被该用户领取(`claimed_by == user_id`) **或** 2) Module 处于 `pending_claim` 且 `module.pool_team_code ∈ {T1..Tn}`(可接单) |
| 任务创建者 | 对 `basic_info` 模块永久 in scope |

**特殊规则**:

- `basic_info` 模块的 scope 判定独立:**创建者所在组的组长** = 永远 in scope(这就是"运营组长按创建者组分"的落地点)。
- `design`/`audit`/`customization` 等模块的组长 scope 判定按**接单人所在组**(这就是"设计/审核组长按接单者组分"的落地点)。
- 接单前(`pending_claim`),模块在**池的组(`pool_team_code`)**下的组长 / 成员都 in scope。接单后,scope **收缩到接单人所在组**。

### Layer 3 · 模块动作门(Module Action Gate)

当 Layer 2 通过后,具体能执行什么 action 由 Module 的 `ActionRegistry` 决定,组成 `(action, allowed_states, allowed_role_filter)` 三元组。

示例(design 模块):

```
design.claim:
  allowed_states = [pending_claim]
  allowed_roles  = [Member(team=design_standard|design_retouch), TeamLead, DepartmentAdmin]

design.submit:
  allowed_states = [in_progress]
  allowed_roles  = [self_only]   // 只有 claimed_by == actor 才行

design.reassign:
  allowed_states = [pending_claim, in_progress]
  allowed_roles  = [TeamLead(same team), DepartmentAdmin(design_rd)]

design.asset_upload_session_create:
  allowed_states = [in_progress]
  allowed_roles  = [self_only, TeamLead(same team)]
```

> **重要**:`audit` 模块明确**不暴露** `asset_upload_session_create`(沿用 v0.9 现行约定,审核阶段**不上传资产**)。前端需据此隐藏入口。
>
> **但 `audit` 可挂"审核参考图"**(非资产,归属 `reference_file_refs`,`owner_module_key = audit`,详见 `V1_ASSET_OWNERSHIP.md` §3.2)。审核人在审核过程中补充说明性图片、标注稿走参考图通道;真正的成品资产只能由上游模块(design / retouch / customization)产出。前端在 audit Panel 仍渲染"审核参考"上传入口,调用 `update_reference_files` 动作(不走 `asset_upload_session_create`)。

### 6.1 决策顺序(服务层统一入口)

所有 `/v1/tasks/*` 端点在 R1 之后**必须**走同一个授权函数:

```
AuthorizeModuleAction(actor, task, module_key, action) → Decision{ Allow | Deny(code, reason) }
  1. 验证 task 存在 ∧ module 已实例化  → 否则 NOT_FOUND
  2. Layer 1 可见性                   → 登录即通过(保留钩子,未来做组织级隔离)
  3. Layer 2 模块作用域               → deny_code = module_out_of_scope
  4. Layer 3 action 门                → deny_code = action_not_allowed_in_state | action_role_denied
```

### 6.2 deny_code 枚举(前端需识别)

| code | 含义 | 前端处理 |
| --- | --- | --- |
| `module_not_instantiated` | blueprint 未开启该模块 | 不渲染该模块 |
| `module_out_of_scope` | 属于只读呈现 | 显示模块内容,禁用所有操作按钮 |
| `module_state_mismatch` | 状态不支持该动作 | 隐藏按钮 |
| `module_action_role_denied` | 角色不够 | 隐藏按钮,不弹 toast |
| `module_claim_conflict` | 接单时被他人抢先 | toast `该任务已被他人领取` |
| `module_blueprint_missing_team` | 池组缺配置 | 展示错误横幅,上报告警 |

---

## 7. 任务池与接单

### 7.1 池的定义

每个**非 basic_info**模块在 `initial_state == pending_claim` 时,记录 `pool_team_code`。所有 `TeamCodes ∋ pool_team_code` 的用户都能在"待接单池"列表看到这条 Module。

- 池入口:`GET /v1/tasks/pool?module_key=design&pool_team_code=design_standard`
  - 参数省略时返回当前用户**所有所属组**的池
  - 列表已按优先级 / 创建时间排序
- 接单动作:`POST /v1/tasks/{task_id}/modules/{module_key}/claim`
  - 请求体可选 `{ "confirm_pool_team_code": "design_standard" }` 做客户端校验

### 7.2 原子接单(CAS)

后端通过如下 SQL 保证并发安全:

```sql
UPDATE task_modules
   SET state = 'in_progress',
       claimed_by = :actor_id,
       claimed_team_code = :actor_primary_team_for_pool,
       claimed_at = NOW()
 WHERE task_id    = :task_id
   AND module_key = :module_key
   AND state      = 'pending_claim'
   AND pool_team_code = :confirm_pool_team_code;
-- affected rows == 1 → 成功
-- affected rows == 0 → MODULE_CLAIM_CONFLICT
```

- 接单成功会写一条 `task_module_events: event_type=claimed`
- `claimed_team_code` 固定为"actor 的多组中与 pool 匹配的那个",用于 Layer 2 组长 scope 判定

### 7.3 不做接单上限

- 无数量限制(产品决策)
- 组长 / 部门管理员可通过 `POST /v1/tasks/{id}/modules/{key}/reassign` 将已接单任务改派到本组其他成员(跨组不允许,除非部门管理员)

### 7.4 跨组调度(仅部门管理员)

- 入口:`POST /v1/tasks/{id}/modules/{key}/pool-reassign`
- 动作:将 Module 重置为 `pending_claim`,修改 `pool_team_code` 为目标组,清空 `claimed_by`
- 审计事件:`pool_reassigned_by_admin`

---

## 8. 数据模型

### 8.1 新表(R2 落地)

#### `task_modules`

| 列 | 类型 | 说明 |
| --- | --- | --- |
| `id` | bigint PK | |
| `task_id` | bigint FK | |
| `module_key` | varchar(32) | `design`/`audit`/`warehouse`/... |
| `state` | varchar(48) | 模块内部状态 |
| `pool_team_code` | varchar(64) NULL | pending_claim 时必填 |
| `claimed_by` | bigint NULL | |
| `claimed_team_code` | varchar(64) NULL | |
| `claimed_at` | datetime NULL | |
| `actor_org_snapshot` | json NULL | 接单时冻结的身份快照(部门、组、姓名) |
| `entered_at` | datetime NOT NULL | blueprint 触发 enter 的时刻 |
| `terminal_at` | datetime NULL | 进入终态的时刻 |
| `data` | json NOT NULL DEFAULT '{}' | 模块私有数据(轻量,大数据走外表) |
| `updated_at` | datetime NOT NULL | |

唯一约束:`UNIQUE(task_id, module_key)`

#### `task_module_events`

| 列 | 类型 | 说明 |
| --- | --- | --- |
| `id` | bigint PK | |
| `task_module_id` | bigint FK | |
| `event_type` | varchar(48) | `entered`/`claimed`/`submitted`/`approved`/`rejected`/`reassigned`/`closed`/`pool_reassigned_by_admin`/... |
| `from_state` | varchar(48) NULL | |
| `to_state` | varchar(48) NULL | |
| `actor_id` | bigint NULL | |
| `actor_snapshot` | json NULL | |
| `payload` | json NOT NULL DEFAULT '{}' | 事件载荷(如驳回原因) |
| `created_at` | datetime NOT NULL | |

索引:`INDEX (task_module_id, created_at)`,`INDEX (event_type, created_at)`(为未来统计预留)

#### `v_task_status_derived`(视图)

用 SQL CASE 聚合 `task_modules.state` 得到任务级 `status`。R1 阶段先以**服务层 `TaskStatusAggregator`** 实现,R3 阶段视性能决定是否物化成视图或冗余列。

### 8.2 现存表的兼容处理

- `tasks.task_status`(真实列名,非 `status`) **保留物理列**,但写入**仅由 `TaskStatusAggregator`** 完成(触发器或服务层钩子,R1 用服务层钩子,R3 再评估是否加 DB 触发器)。
- `tasks.assignee_id`、`audit_claim_*` 等**冗余字段**转为**由 aggregator 回填**的只读镜像,供旧查询沿用。新代码禁止直接 UPDATE 这些字段。
- **任务优先级**:复用既有列 `tasks.priority VARCHAR(16) DEFAULT 'normal'`(由 `001_v7_tables.sql` 创建),v1.2 将 CHECK 枚举定为 **`low | normal | high | critical`** 4 值(R1.6 真生产分布实测 `low 58.9% / high 21.1% / normal 20.0%`,保留 `critical` 作为未来预留位),并增加 `(priority, created_at)` 复合索引支持池排序(由 R2 迁移 067 完成)。排序**必须**用 `FIELD(priority,'critical','high','normal','low') ASC` 显式权重,**不要**依赖字典序。**不新增 `task_priority` 列**。

### 8.3 业务大数据外挂表

| 表 | 归属模块 | 说明 |
| --- | --- | --- |
| `task_assets` | `design` / `retouch` / `customization` | 继续沿用;`source_module_key` 新增列指向模块 |
| `task_audit_events` | `audit` | 仅存审核批注等业务细节;状态事件走 `task_module_events` |
| `task_warehouse_*` | `warehouse` | |
| `task_procurement_*` | `procurement` | |
| `task_customization_*` | `customization` | |

---

## 9. API 模块化重构

### 9.1 URL 命名

新的**首选** URL:

```
GET    /v1/tasks                                           # 任务列表(全量可见)
GET    /v1/tasks/{id}/detail                               # 任务详情(含全部模块 projection)
GET    /v1/tasks/pool?module_key=&pool_team_code=          # 待接单池
POST   /v1/tasks/{id}/modules/{module_key}/claim           # 接单
POST   /v1/tasks/{id}/modules/{module_key}/actions/{action}# 触发模块内任意动作
POST   /v1/tasks/{id}/modules/{module_key}/reassign        # 组内改派
POST   /v1/tasks/{id}/modules/{module_key}/pool-reassign   # 跨组调度(部门管理员)
POST   /v1/tasks/{id}/cancel                               # 任务级作废 / 关闭(v4 新增)
```

#### 9.1.1 `POST /v1/tasks/{id}/cancel` 语义

一条端点承载产品侧"作废"与"关闭"两个按钮。

| 请求体字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `reason` | string | 是 | 运营侧必须填写的原因,全长 ≤ 500 |
| `force` | boolean | 否(默认 false) | `false` 对应"作废"(未接单前);`true` 对应"关闭"(任意节点,DeptAdmin) |

**后端分流规则**:

| `force` | 前置要求 | 级联行为 | 写入事件 |
| --- | --- | --- | --- |
| `false` | 所有**非 `basic_info`** 模块处于 `pending_claim` 或未实例化 | 全部未终态模块置 `forcibly_closed`;task 置 `cancelled` | `task_module_events.event_type = task_cancelled` |
| `true` | 仅 `DeptAdmin / SuperAdmin` 可触发 | 所有仍在运行的模块置 `forcibly_closed`;task 置 `closed` | `task_module_events.event_type = forcibly_closed`(每个被级联的模块一条) |

`force=false` 条件不满足时返回 `409 task_already_claimed` → 前端提示用户改走"关闭(force=true)"流程。

前端侧两个产品按钮(作废 / 关闭)共调此端点,无独立 task 级取消 API。

### 9.2 TaskDetail 响应契约(契约稳定点)

```json
{
  "task": {
    "id": 484,
    "task_no": "YB-2026-00123",
    "task_type": "original_product_development",
    "workflow_blueprint_key": "original_product_development_v1",
    "creator": { "user_id": 12, "name": "...", "team_code": "operations_daily" },
    "derived_status": "audit_in_progress"
  },
  "modules": [
    {
      "module_key": "basic_info",
      "state": "active",
      "scope": { "in_scope": true, "reason": null },
      "allowed_actions": ["update_business_info"],
      "projection": { /* 业务字段 */ }
    },
    {
      "module_key": "design",
      "state": "submitted",
      "pool_team_code": null,
      "claimed_by": { "user_id": 66, "name": "张三", "team_code": "design_standard" },
      "scope": { "in_scope": false, "reason": "module_out_of_scope" },
      "allowed_actions": [],
      "projection": { "asset_versions": [...], "reference_file_refs": [...] }
    },
    {
      "module_key": "audit",
      "state": "pending_claim",
      "pool_team_code": "audit_standard",
      "claimed_by": null,
      "scope": { "in_scope": true },
      "allowed_actions": ["claim"],
      "projection": { "design_version_snapshot": {...} }
    }
  ]
}
```

### 9.3 兼容路由(Migration-only)

旧路由 `/v1/tasks/{id}/audit_a_claim` 等 **保留 3 个迭代周期**,在控制器层内部转译到新路由,响应结构收敛到上文契约。过渡期结束后进 `docs/COMPATIBILITY_ROUTES_INVENTORY.md` 的"已下线"段落。

---

## 10. 状态推导与兼容层

### 10.1 `derived_status` 推导规则

从 `task_modules` 聚合,按 blueprint 顺序扫描,返回**第一个非终态模块的复合状态**。若全部终态则返回 `closed`。

> **v1.1 · 旧 status 映射对齐真实枚举**:v0.9 的 `tasks.task_status` 取值范围以 `domain/enums_v7.go` 为权威(共 24 个值,含 Draft / PendingAssign / Assigned / InProgress / PendingAuditA / RejectedByAuditA / PendingAuditB / RejectedByAuditB / PendingOutsource / Outsourcing / PendingOutsourceReview / PendingCustomizationReview / PendingCustomizationProduction / PendingEffectReview / PendingEffectRevision / PendingProductionTransfer / PendingWarehouseQC / RejectedByWarehouse / PendingWarehouseReceive / PendingClose / Completed / Archived / Blocked / Cancelled)。**早期 Draft v1 使用的 `InDesign / InAuditA / InWarehouse / InCustomization / InProcurement` 等值不在真实枚举中**,本表按真实枚举重写。

| Blueprint 扫描结果 | derived_status | 旧 `task_status` 映射(兼容) |
| --- | --- | --- |
| `design.pending_claim` | `design_pending_claim` | `PendingAssign` |
| `design.in_progress` | `design_in_progress` | `InProgress` / `Assigned` |
| `design.submitted` ∧ 下游 audit 未 enter | `design_submitted` | `InProgress`(提交瞬间) / 下一条 |
| `audit.pending_claim` | `audit_pending_claim` | `PendingAuditA` |
| `audit.in_progress` | `audit_in_progress` | `PendingAuditA`(持续态,v0.9 不区分"在审") |
| `audit.rejected` ∧ 回流中 | `audit_rejected` | `RejectedByAuditA` / `RejectedByAuditB` |
| `audit.approved` ∧ warehouse 未 enter | `audit_approved` | `PendingProductionTransfer` / `PendingWarehouseReceive` |
| `warehouse.preparing` | `warehouse_preparing` | `PendingWarehouseQC` / `PendingWarehouseReceive` |
| `warehouse.rejected` | `warehouse_rejected` | `RejectedByWarehouse` |
| `warehouse.completed` | `closed` | `Completed` / `PendingClose` |
| `customization.pending_claim` | `customization_pending_claim` | `PendingCustomizationProduction`(定制任务)|
| `customization.in_progress` | `customization_in_progress` | `PendingCustomizationProduction` |
| `customization.submitted` | `customization_submitted` | `PendingCustomizationReview` |
| `procurement.in_progress` | `procurement_in_progress` | `Outsourcing` / `PendingOutsource` |
| `procurement.review` | `procurement_review` | `PendingOutsourceReview` |
| 任何模块 `cancelled` | `cancelled` | `Cancelled` / `Blocked` |
| 全部模块终态 + 归档 job 跑过 | `archived` | `Archived` |

### 10.2 `TaskStatus` 旧枚举的处置

- 不删
- 标注 `// Deprecated: derived in v1; kept for read-side compatibility only.`
- 所有**写入者**迁移到 `TaskModuleStateTransitioner`

### 10.3 `TaskAction` 旧枚举的处置

- 不删
- 标注 Deprecated
- R1 新建 `domain/module_action.go`,按模块切割成:

```
DesignAction   = Claim | Submit | Reassign | AssetUploadSessionCreate | ...
AuditAction    = Claim | Approve | Reject | Transfer | Takeover | Handover | ...
...
```

- 旧 `TaskAction` 在控制器入口处映射到 `(module_key, ModuleAction)` 二元组

---

## 11. 迁移方案(一次性切换)

### 11.1 停服窗口

允许 **5 ~ 10 min** 的停服窗口,公告提前 24h。窗口内步骤:

| 步骤 | 耗时预算 | 失败可回滚? |
| --- | --- | --- |
| 1. 关闭入口流量(前置 LB 404) | 30s | 是(直接放流量) |
| 2. 等待在飞事务收敛(观察 10s 无活跃连接) | 30s | 是 |
| 3. 执行 R2 迁移 `059` ~ `06x`(建表) | 1 min | 是(回滚脚本随行) |
| 4. 执行 R2 backfill:遍历现存 task → 按 `task_type` 实例化 `task_modules` → 根据旧字段推断各模块 state/claimed_by → 写事件 `migrated_from_v0_9` | 3 ~ 5 min(10w 任务内) | 部分是(backfill 失败回滚迁移 + 旧代码即可) |
| 5. 切换服务二进制到 v1 | 30s | 是(切回旧二进制 + 回滚迁移) |
| 6. smoke 关键链路(列表/详情/接单/提交/上传一把) | 2 min | 是(完整回滚) |
| 7. 放流量 | 30s | — |

### 11.2 Backfill 规则(核心)

> **v1.1 字段对齐**:SELECT 源列统一为 `tasks.task_status`(非 `status`);旧值 `PendingAssignment` / `InDesign` / `InAuditA` 是 Draft v1 误写,已对齐真实枚举 `PendingAssign` / `InProgress` / `PendingAuditA`(`domain/enums_v7.go`)。`is_urgent` 列在仓库中不存在,从 backfill 规则中移除。

| 旧任务字段 / 状态 | 实例化的模块 | state | claimed_by | pool_team_code |
| --- | --- | --- | --- | --- |
| 任意 task | `basic_info` | `active` | — | — |
| `task_status = PendingAssign` | `design` | `pending_claim` | — | 依任务类型查 4.1 表 |
| `task_status IN (Assigned, InProgress)` ∧ `designer_id != null` | `design` | `in_progress` | `designer_id` | — |
| `task_status = PendingAuditA` | `audit`(A 轮) | `pending_claim` / `in_progress`(看有无审核员) | 审核员 ID(若存在) | 依任务类型查 4.1 表 |
| `task_status = RejectedByAuditA` | `audit` | `rejected` → 回流 design | — | — |
| `task_status = PendingAuditB` | `audit`(B 轮) | `pending_claim` / `in_progress` | — | — |
| `task_status = PendingCustomizationProduction` | `customization` | `pending_claim` / `in_progress` | `last_customization_operator_id`(若有) | `customization_art` |
| `task_status = PendingCustomizationReview` | `customization` | `submitted` + `audit` `pending_claim` | — | `audit_customization` |
| `task_status IN (PendingOutsource, Outsourcing)` | `procurement` | 按采购单状态映射 | — | `procurement_main` |
| `task_status = PendingOutsourceReview` | `procurement` | `review` | — | — |
| `task_status IN (PendingProductionTransfer, PendingWarehouseQC, PendingWarehouseReceive)` | `warehouse` | `preparing` | — | `warehouse_main` |
| `task_status = RejectedByWarehouse` | `warehouse` | `rejected` | — | — |
| `task_status IN (PendingClose, Completed)` | 全部已进入模块 | 终态(closed) | — | — |
| `task_status IN (Archived)` | 全部 | 终态(closed);task 打 `archived_at` | — | — |
| `task_status IN (Cancelled, Blocked)` | 全部未终态模块 | `forcibly_closed` | — | — |
| `customization_jobs.status` 非空 | 交叉校验 `customization` 模块 state(`customization_jobs.status=pending_customization_production` → `pending_claim` / `in_progress`) | 按子状态映射 | `assigned_operator_id` / `last_operator_id` | — |

> **v2 注解**:v0.9 数据中若存在 `design` 模块被定制任务使用的记录,backfill 时按"已完成"直接收敛到 `customization` 模块的对应子状态,不再创建独立 `design` 实例。详见 `docs/V1_CUSTOMIZATION_WORKFLOW.md` §迁移部分。

> Backfill 脚本独立 ADR:`docs/workorders/round_r2_backfill.md`(R2 产出)。

### 11.3 无双写期

产品已确认**不保留双写**。切换后旧字段仅作为 aggregator 回填的副产品存在。

### 11.4 回滚剧本

- DB 侧:`059` ~ `06x` 迁移都必须随带 `DROP TABLE IF EXISTS task_modules; DROP TABLE IF EXISTS task_module_events;` 回滚 SQL。
- 服务侧:保留 `v1.0` 的二进制镜像,切换失败 2 min 内可原封放回。
- 数据侧:backfill 完成后**不删旧字段**,回滚时旧字段依旧是真源。

---

## 12. 实施路线(R1 ~ R6)

| 轮次 | 产物 | 验收门 | 估算 |
| --- | --- | --- | --- |
| **R1** | 骨架与契约:`domain/module_*.go`、`ModuleDescriptor` 注册表、`WorkflowBlueprint` 常量、`AuthorizeModuleAction` 统一授权入口、`TaskDetail` 新契约 scaffolding;**不接数据库**(全部走现有表,用 aggregator 派生) | 单测:blueprint 覆盖率 100%;`AuthorizeModuleAction` 覆盖 6 种角色 × 每模块 3 action;openapi.yaml 新增 `/modules/*` 端点定义;老路径不变 | 1 个迭代 |
| **R2** | 数据层:`task_modules` / `task_module_events` 迁移 + backfill 脚本 + 回滚脚本;切到真实读写;下线 aggregator 的 fallback 路径;**全局资产搜索 API `GET /v1/assets/*`**;**全局搜索 `GET /v1/search`**(任务号 / 产品编码 / 资产文件名);**ERP Bridge 按产品编码查询 `/v1/erp/products/by-code`**(转发 8081 上游 `/open/combine/sku/query`) | 停服演练在 staging 成功;staging 任务集抽样 200 条数据一致性比对 100%;全局搜索在 10w 任务规模 P95 < 300ms | 1 个迭代 |
| **R3** | 前端详情页一屏化改造;池 / 接单 UI;旧工作台视图切到新 detail aggregate;个人中心(头像下拉)+ 通知中心;组织菜单(用户 / 部门 / 组);资产管理中心跨任务视图(所有人可见,删除仅 SuperAdmin) | UAT 登录三种角色,列表 / 详情 / 接单 / 提交 / 驳回 / 上传全部走新路径无 403;通知中心 v1 必做(Q5.3) | 1 ~ 2 个迭代 |
| **R4** | 清理:旧 `TaskAction` 入口收敛到 `(module, action)` 映射;旧 URL 标 Deprecated;`frontend_access.json` 瘦身(只留菜单开关) | `grep -r TaskActionAudit` 仅出现在 deprecated shim 里 | 1 个迭代 |
| **R5** | 观测与报表:`task_module_events` 的 Kafka / 本地 consumer 出口;**L1 报表卡片**(工作台顶部实时 count,WebSocket 推送);**报表一级菜单(SuperAdmin 专属)**;超时升级钩子接口(不实现调度) | L1 卡片 P95 < 200ms;运维面板能看到事件流 QPS / 模块驻留时间 | 1 个迭代 |
| **R6** | 性能与治理:视图物化 / 索引优化;`derived_status` 物化列评估;任务自动归档 job(closed 满 90 天);OSS 对象生命周期(终态 + 365 天) | P99 `/v1/tasks/{id}/detail` < 150ms;归档 job 回归 | 1 个迭代 |

每轮 Codex prompt 在本文签字后按此顺序产出,并且每个 prompt 必须**引用本文章节号**作为验收依据(如 "R1 的 Auth 实现必须满足 §6.1 的四步决策顺序")。

---

## 13. 验收标准(总表)

| 编号 | 验收项 | 关联章节 | 生效轮次 |
| --- | --- | --- | --- |
| A1 | 任一登录用户 `GET /v1/tasks` 可见所有未删除任务 | §6 Layer 1 | R1 |
| A2 | 任一登录用户 `GET /v1/tasks/{id}/detail` 返回 200(除硬删除) | §6 Layer 1 | R1 |
| A3 | Module.scope.in_scope 的判定与 §6 Layer 2 表格 100% 一致,单测覆盖 | §6.Layer 2 | R1 |
| A4 | 每个 module action 在非允许状态被触发时返回 `module_state_mismatch`;非允许角色返回 `module_action_role_denied` | §6 Layer 3 | R1 |
| A5 | 接单并发 100 线程抢同一 `pending_claim`,恰好 1 人成功,其余 `MODULE_CLAIM_CONFLICT` | §7.2 | R1/R2 |
| A6 | 常规任务池仅出现在 `design_standard`/`audit_standard` 组成员面前;定制任务仅出现在定制组面前 | §4.1 | R1/R2 |
| A7 | 部门管理员可执行 `pool-reassign` 改池组;其他角色返回 `module_action_role_denied` | §7.4 | R1 |
| A8 | `tasks.status` 列与 `v_task_status_derived` / aggregator 计算结果在 staging 200 条样本比对完全一致 | §8.1/§10 | R2 |
| A9 | 回滚脚本在 staging 演练可恢复到 v1.0 基线 | §11.4 | R2 |
| A10 | 详情页一屏呈现 6 模块,无二次请求;创建者可写 `basic_info`,非接单者在 `design` 模块只读 | §9.2 | R3 |
| A11 | `grep -r "TaskAction.*=\s*TaskAction\s*\"audit_"` 在 R4 后仅出现在 deprecated 映射文件 | §10.3 | R4 |
| A12 | 事件流可被独立消费,消费端断连不影响主库写入 | §12 R5 | R5 |

---

## 14. 风险与回滚

| 风险 | 等级 | 缓解 |
| --- | --- | --- |
| Backfill 推断状态错误 | 高 | Backfill 逐条写审计事件 `migrated_from_v0_9`,保留旧字段,错误可重跑 |
| 多组用户的 Layer 2 scope 判定死循环 | 中 | R1 强制 `TeamCodes` 最大长度 8,单测覆盖 1~8 组场景 |
| `derived_status` 聚合开销过大 | 中 | R1 先走 aggregator;R3 评估物化视图 / 冗余列;P99 > 150ms 即触发 R6 |
| 旧前端在 R3 前只能用兼容路由 | 低 | 兼容路由保留 3 个迭代,R4 才下线 |
| `frontend_access.json` 瘦身破坏菜单 | 低 | R4 前不动,仅标注未来会删的 key |
| 部门管理员错误调度跨组任务 | 中 | `pool-reassign` 需二次确认 + 审计事件 + 组长抄送 |
| 停服窗口超时 | 中 | 预发演练必须 ≤ 7 min,否则延迟切换 |

---

## 15. 未决项(本轮不解决,留 ADR)

| # | 议题 | 预期轮次 |
| --- | --- | --- |
| U1 | 超时升级(TimeoutEscalation)调度器实现 | R7+ |
| U2 | L2 / L3 报表(跨部门经营看板、效能分析) | R7+ |
| U3 | `WorkflowBlueprint` 外置成 DB 配置 / 配置中心 | R8+ |
| U4 | `actor_org_snapshot` 冻结点是接单时还是每次触发动作时 | 本轮采用"接单时";若出现历史追溯需求改为"每次动作快照" |
| U5 | ~~客户定制是否在 `design` 模块开启时自动联动 `customization` 结束~~ | **v2 决策**:定制任务永不启用 `design`,议题取消(见 §4 说明 4.2) |
| U6 | 任务优先级是否按 SLA 自动升级(low → normal → high → critical) | R7+;v1 仅支持手动设置与池排序 |

---

## 16. 关联子文档索引(v2 新增)

本文保持为**架构骨架**,以下三份子文档承担**领域细节**,与本文同等生效:

| 子文档 | 覆盖范围 | 状态 |
| --- | --- | --- |
| `docs/V1_CUSTOMIZATION_WORKFLOW.md` | 定制模块(`customer_customization` / `regular_customization`)内部状态机、创建链路、ERP Bridge 产品编码查询、驳回回流、迁移映射 | Draft v1 |
| `docs/V1_ASSET_OWNERSHIP.md` | 资产归属表、`source_module_key`、版本锚定语义(审核 always-latest + 事件快照)、参考图挂载点、OSS 生命周期、资产管理中心跨任务 API、删除权限 | Draft v1 |
| `docs/V1_INFORMATION_ARCHITECTURE.md` | 菜单树(任务中心 / 组织 / 资产管理中心 / 报表 / 个人中心)、个人中心头像下拉、用户管理三级授权(SuperAdmin + DeptAdmin + TeamLead)、全局搜索、通知中心 | Draft v1 |

> **冲突裁决**:三份子文档的某个章节与本文冲突时,以本文为准;本文未覆盖的空白由子文档补齐。主次关系写入每份子文档的文档头。

---

## 17. 已合并的边界决策(v2 会议纪要)

| 领域 | 决策 | 来源问题 |
| --- | --- | --- |
| 菜单 | 合并为一级菜单 **"任务中心"**,内含 tabs:全任务 / 任务池 / 我的任务 / 已归档 | Q1.1 / Q1.2 / Q1.3 |
| `frontend_access.json` | 保留,瘦身,仅作菜单开关 | Q1.4 |
| 定制 | `design` 模块**永不启用**;`basic_info → customization → audit → warehouse`;audit 驳回永远回 `customization` | Q2.2 / Q2.4 / Q2.5 |
| 客户确认 | **不经过系统内客户确认**;定制美工上传终稿即进 audit | Q2.3 / O3 |
| 客户定制元数据来源 | ERP Bridge `/open/combine/sku/query`(上游 8081),后端封装 `/v1/erp/products/by-code` | Q2.1 / O1 |
| 常规定制元数据来源 | **v1 直接做"设计源文件查询"功能**(不留空) | Q2.1 / O2 |
| 客户定制 vs 常规定制差异 | 仅任务编号前缀 + 创建时元数据来源,流程一致 | Q2.6 |
| 资产模型 | 全局 `task_assets` + `source_module_key` | Q3.1 |
| 审核引用版本 | always latest + `task_module_events.payload` 冻结 `asset_version_id` 快照 | Q3.2 / C6 |
| 云仓资产 | 无独立资产,仅引用上游 | Q3.3 |
| OSS 生命周期 | 终态后 365 天自动清理 + 资产管理中心管理员可手动归档删除 | Q3.4 / C5 |
| 参考图归属 | 按挂载点:task/SKU 归 `basic_info`,审核归 `audit`,定制归 `customization` | Q3.6 |
| 报表范围 | v1 到 L1(实时卡片);仅 SuperAdmin 可见"报表"一级菜单 | Q4.1 / Q4.2 / C1 |
| 报表跨模块指标 | 支持,需 task 级宽表(R5) | Q4.3 |
| 报表实时性 | WebSocket 推送(限定范围:任务池计数 + 我的任务变更) | Q4.4 |
| 报表数据源 | v1 直接 query `task_module_events`,R6+ 视情况上聚合 | Q4.5 |
| 个人中心位置 | **头像下拉,无一级菜单** | Q5.4 |
| 个人中心板块 | 账户信息 / 安全(改密)/ 我的组织 / 我的任务 / 我的待接单 / 通知中心 | Q5.1 |
| 个人中心不含 | 偏好 / 我的数据(报表)/ 登录历史 | Q5.1 / Q5.5 / C2 |
| 通知中心 | v1 必做,仅站内,数据源 `task_module_events` + `notifications` 外表 | Q5.3 / Q7.1 |
| 用户管理授权 | 三级:SuperAdmin(全局)+ DeptAdmin(本部门)+ TeamLead(本组**仅启停账号**) | Q6.1 / O4 |
| 用户管理菜单 | 一级菜单 **"组织"** → 二级(用户 / 部门 / 组) | Q6.2 |
| 组增删改 | SuperAdmin + HR + DeptAdmin(本部门) | Q6.3 |
| 部门 / 组 / 成员调整 | DeptAdmin 部门内直接生效;跨部门需 SuperAdmin 确认;不走审批 | Q6.4 |
| DeptAdmin 跨部门 | 可把本部门成员**移出**,**不能拉入**其他部门成员;跨部门移动需 SuperAdmin 确认 | Q6.5 / C3 |
| HR 管理员 | 现已存在 `HRAdmin` 用户,R1 沿用 | Q6.6 |
| 任务优先级 | **复用既有 `tasks.priority` 列**(001 已定义 VARCHAR(16) DEFAULT 'normal');v1.2 定枚举为 `low / normal / high / critical` 4 值(R1.6 真生产实测分布 `low 58.9% / high 21.1% / normal 20.0%`,`critical` 作为未来预留)。R2 067 加 CHECK + `(priority, created_at)` 复合索引;池列表按 `FIELD(priority,'critical','high','normal','low') ASC, created_at ASC` 排(避免字典序误解)。**不新增 `task_priority` 列**;不做 `low→normal / high→urgent` 归一化 UPDATE | Q7.5 + R1.5 对齐 + R1.6 真生产对齐 |
| 任务关闭 | 部门管理员任意节点可关闭,需理由必填 | Q7.2 |
| 任务作废 / 关闭端点 | 统一 `POST /v1/tasks/{id}/cancel` body `{ reason, force }`:`force=false` = 作废(未接单前);`force=true` = 关闭(任意节点 DeptAdmin)。前端两个按钮共用此端点,详见 §9.1.1 | FE Plan Review v4 |
| 任务归档 | 自动:closed 满 90 天;手动:部门管理员可提前归档 | Q7.3 / C4 |
| 多组用户接单 | `claimed_team_code` 固定为接单时匹配到池组的组 | Q7.4 |
| 全局搜索 | **v1 必做**,涵盖任务号 / 产品编码 / 资产文件名 | 会议追加 |
| 资产管理中心 | **所有人可见**(跨任务浏览/下载);删除权限仅 SuperAdmin | C5 |
| 报表数据源实装校正(SA-D) | **L1 module-dwell / throughput 的 dwell 语义数据源是 `task_module_events e JOIN task_modules m ON m.id = e.task_module_id`**(module_key / task_id 从 task_modules 读,不来自 task_module_events);dwell 用 CTE 在 task_module_events 内部把 `enter-like` event 配到下一个 `exit-like` event(以 `created_at` 排序),P95 用 MySQL 8 window rank 近似;`backfill_placeholder` 事件必须显式排除;5 个 module_key 固定返回行(样本为空时 `samples=0` 不得缺行)。**本条覆盖并校正 V1_R1_7_D_OPENAPI_SA_D_PATCH.md §3.5 中"state_enter_at / state_exit_at"文档幻觉表述** — 生产 `task_module_events` 无此两列 · SA-D 实装已按此校正。throughput.archived 在 v1 与 completed 同集合(基于 event_type `closed/archived/approved`),v2+ 视需要再拆分。仅 SuperAdmin 可见报表;MySQL LIKE 全局搜索(不引 ES);`users[]` 在 SuperAdmin+HRAdmin 之外全部返 `[]` | SA-D 架构裁决(2026-04-25) |
| 新品开发批量 SKU | **入口统一为 Excel** — 下载模板 → 上传 → 预览 → 创建;**移除现有"多行内联编辑" UI**;单品 SKU 入口保留不变;模板字段 1:1 对齐单品必填清单;后端 `batch_sku_mode=multiple` 契约不变。延展能力(竞品引用、矩阵编辑、变量轴)统一推迟到后续版本迭代。详见 `V1_INFORMATION_ARCHITECTURE.md` §3.5 | 会议追加(运营对齐) |

---

## 18. 变更记录

| 版本 | 日期 | 变更 | 签字 |
| --- | --- | --- | --- |
| Draft v1 | 2026-04-17 | 初稿 | — |
| Draft v2 | 2026-04-17 | 合并 Q1~Q7 / C1~C6 / O1~O4 决策;收敛定制 blueprint(移除 design);新增子文档索引(§16)与决策清单(§17);R2/R3/R5/R6 路线扩容;U5 取消 / 新增 U6 | — |
| Draft v3 | 2026-04-17 | 追加"新品开发批量 SKU = Excel 唯一入口"决策(§17 追加行);移除旧"多行内联编辑" UI;延展能力统一延后;在 `V1_INFORMATION_ARCHITECTURE.md` 新增 §3.5 | — |
| Draft v4 | 2026-04-17 | 合并 FE Plan 评审:§9.1 新增 `POST /v1/tasks/{id}/cancel`(§9.1.1 双语义端点);§6 Layer 3 补"audit 可挂审核参考图"注;§17 追加任务作废/关闭端点行;本文配套 IA Draft v3(草稿端点 + task_mentioned 延后)与定制工作流 Draft v2(通用字段注) | **已签字**(2026-04-17) |
| v1.1 | 2026-04-17 | **R1.5 DDL 对齐**:§8.2 `tasks.status` 更正为 `tasks.task_status` 并补"优先级复用 `tasks.priority` 列,不新增 `task_priority`";§10.1 旧 `task_status` 映射表按 `domain/enums_v7.go` 真实枚举重写(24 个值);§11.2 backfill 表 SELECT 源列统一为 `task_status`,移除 `is_urgent`,旧值 Draft v1 误写项更正;§17 Q7.5 追加"复用 priority 列,不新增 task_priority"注。依据 `docs/iterations/V1_R1_5_DDL_ALIGNMENT.md` | **已签字**(2026-04-17) |
| v1.2 | 2026-04-17 | **R1.6 真生产对齐**:§8.2 + §17 Q7.5 的 `tasks.priority` CHECK 枚举从 v1.1 的 `normal | urgent | critical`(3 值)修订为 **`low | normal | high | critical`**(4 值),对齐 `jst_erp` 真生产分布(`low 58.9%` / `high 21.1%` / `normal 20.0%`);排序语义改为 `FIELD(priority,'critical','high','normal','low') ASC` 显式权重,避免字典序误用;§U6 联动更新。依据 `docs/iterations/V1_R1_6_PROD_ALIGN.md` | **已签字**(2026-04-17) |
| v1.3 | 2026-04-25 | **R4-SA-D 实装校正**:§17 追加 `报表数据源实装校正(SA-D)` 行,锁定 L1 报表 dwell 数据源为 `task_module_events JOIN task_modules JOIN tasks`(module_key/task_id 在 task_modules 而非 task_module_events;后者无 state_enter_at/state_exit_at 列),dwell 用 CTE 内配对 enter-like/exit-like event + window rank P95,5 module_key 固定返行;throughput.archived 在 v1 与 completed 同集合(v2+ 再拆分);`users[]` 除 SuperAdmin+HRAdmin 外返空。依据 `docs/iterations/V1_R4_SA_D_REPORT.md` §Architect Adjudication | **已签字**(2026-04-25) |

---

**签字门槛**:产品、后端、前端负责人在本段各自签注 "确认" 或提出章节级别改动后,本文(含三份子文档)进入 v1 生效,R1 Codex prompt 开工。

- 产品: **已确认**(2026-04-17)
- 后端: **已确认**(2026-04-17)
- 前端: **已确认**(2026-04-17,基于 FE Plan Draft v1 评审回写并同步修订)

> **v1.0 生效说明**:本文与三份子文档(IA v3 / 定制工作流 v2 / 资产归属 v1)已于 2026-04-17 一并签字生效。后续对本文及子文档的任何修订必须先提 ADR 或变更记录,严禁静默改动。R1 Codex prompt 即日起可基于 v4 骨架开工。
