# V1 信息架构(菜单 / 个人中心 / 用户管理 / 全局搜索 / 通知中心)

> 状态: **v1.1 · DDL 对齐已签字生效(审计通过 · 无结构变更)**(2026-04-17,依据 `docs/iterations/V1_R1_5_DDL_ALIGNMENT.md` v1.0);前置版本 v1.0 · 已签字生效(2026-04-17)
> 最终签字版本:Draft v3(含任务草稿端点 + task_mentioned 推后)
> v1.1 变更:R1.5 审计本文所有列引用,**未发现字段幻觉**(`tasks.priority` 真实存在、`derived_status` 为 v1 新派生字段、`task_drafts / notifications / org_move_requests` 均为 v1 新表无历史冲突)。仅加注本说明,表结构与端点定义**维持 v1.0 不变**。依据 `docs/iterations/V1_R1_5_DDL_ALIGNMENT.md`。
> 主次关系: 本文与 `V1_MODULE_ARCHITECTURE.md` 冲突时以主文档为准;主文档未覆盖的信息架构细节由本文补齐。
> 读者: 前端研发、后端研发、产品

---

## 1. 菜单树(登录后全局)

```
顶部导航
├── 任务中心          ← 一级菜单,所有登录用户可见
│    ├── tab: 全任务           (默认,Layer 1 全量)
│    ├── tab: 任务池           (仅显示用户所属组匹配的 pending_claim 模块)
│    ├── tab: 我的任务         (已接单 + 已创建)
│    └── tab: 已归档           (按 closed + 90 天归档)
│
├── 组织              ← 一级菜单,按角色可见(见 §5 用户管理)
│    ├── 用户
│    ├── 部门
│    └── 组
│
├── 资产管理中心      ← 一级菜单,所有登录用户可见(下载开放,删除仅 SuperAdmin)
│    └── 资产列表 + 搜索 + 过滤 + 归档区
│
├── 报表              ← 一级菜单,仅 SuperAdmin 可见
│    ├── 任务吞吐看板
│    ├── 模块驻留时长
│    └── 个人/组效能(R5+)
│
└── [头像下拉]        ← 所有登录用户;无一级菜单
     ├── 个人中心
     │    ├── 账户信息
     │    ├── 安全(改密)
     │    ├── 我的组织(只读)
     │    ├── 我的任务(快捷入口,点击跳"任务中心 → 我的任务")
     │    ├── 我的待接单(快捷入口,点击跳"任务中心 → 任务池")
     │    └── 通知中心
     └── 退出登录
```

### 1.1 全局搜索框(顶部导航右侧)

- 位置:顶部导航栏右侧,与头像同行
- 入口:常驻放大镜图标 + `Ctrl+K`(或 `Cmd+K`)快捷键
- 覆盖范围:见 §4 全局搜索

---

## 2. 菜单可见性矩阵

| 菜单 | SuperAdmin | DeptAdmin | TeamLead | Member | 备注 |
| --- | --- | --- | --- | --- | --- |
| 任务中心 | ✓ | ✓ | ✓ | ✓ | 全员可见 |
| 组织 | ✓ | ✓ | ✓(只读组内) | ✗ | Member 不可见整个"组织"入口 |
| 资产管理中心 | ✓ | ✓ | ✓ | ✓ | 下载开放;归档/删除仅 SuperAdmin |
| 报表 | ✓ | ✗ | ✗ | ✗ | v1 仅 SuperAdmin;后续 U2 扩展 |
| 个人中心(头像下拉) | ✓ | ✓ | ✓ | ✓ | — |

菜单开关仍由 `config/frontend_access.json` 控制(v1 决策:保留瘦身版)。

### 2.1 报表 L1 实装规则快照(SA-D v1.0 签字固化 · 2026-04-25)

v1 报表只做 L1(3 个 endpoint:`cards` / `throughput` / `module-dwell`);以下 5 条实装规则来自 `docs/iterations/V1_R4_SA_D_REPORT.md` §Architect Adjudication · 与 OpenAPI `x-rbac-placeholder` 一起是前端/后续轮次的合同:

1. **数据源**:直查 `task_module_events e JOIN task_modules m ON m.id = e.task_module_id JOIN tasks t ON t.id = m.task_id`;**不建物化表 · 不引 ES / Redis / Kafka**(R6+ 再评估)。
2. **dwell 语义**:`module_key` / `task_id` 从 `task_modules` 读;**`task_module_events` 本身无 `module_key`/`task_id`/`state_enter_at`/`state_exit_at` 列**。dwell 在 CTE 内把 `enter-like` event 配到同一 `task_module_id` 的下一 `exit-like` event(按 `created_at` 升序),`AVG(TIMESTAMPDIFF(SECOND, enter.created_at, exit.created_at))`;P95 用 MySQL 8 window rank 近似。5 个 `module_key`(`task_detail/design/audit/customization/warehouse`)**固定返回行** · 样本为空时 `samples=0` 不得缺行。
3. **throughput 语义(v1 简化)**:`created` = event_type='created' 去重 `task_id` 日分组;`completed` = event_type ∈ {`closed`, `archived`, `approved`} 或 task.task_status 已 closed/archived;`archived` 在 v1 **与 completed 同集合** · v2+ 视需要再拆分。`backfill_placeholder` 事件**必须显式排除**(它不是真实业务事件)。
4. **RBAC**:3 个报表 endpoint **仅 SuperAdmin** 可见可调;其余角色返 `403 PERMISSION_DENIED deny_code=reports_super_admin_only`,必须写 `permission_logs.action_type='report_access_denied'` 审计。
5. **搜索 `users[]` 可见角色**(R1.7-D Q1=A1 补白):**仅 SuperAdmin + HRAdmin** 两个角色的 `users[]` 返匹配行;其余角色(含 DeptAdmin / TeamLead / Member / Operator / 各业务角色)`users[]` **永远返 `[]`**;搜索 tasks/assets/products 三数组不受此限制 · 按任务可见性规则(§6.1 Layer 1 全员可见未删除任务)返。

> v2 / R5+ 规划若引入 L2(跨部门宽表)/ L3(经营看板)/ 导出 CSV / ES 高亮 · 以上 §2.1 规则作为 v1 基线保留;R6+ 视压力评估物化表替代直查。

---

## 3. 任务中心(一级菜单)

### 3.1 4 个 tab 的默认过滤

| tab | 后端 query(示意) | 默认排序 |
| --- | --- | --- |
| 全任务 | `GET /v1/tasks?status=all` | `priority DESC, created_at DESC` |
| 任务池 | `GET /v1/tasks/pool?module_key=any&pool_team_code=<actor的所有teams>` | `priority DESC, created_at ASC`(FIFO 内部按优先级顶部) |
| 我的任务 | `GET /v1/tasks?filter=mine`(后端语义:任一模块 `claimed_by=actor_id` OR task.creator_id=actor_id) | `updated_at DESC` |
| 已归档 | `GET /v1/tasks?status=archived` | `archived_at DESC` |

### 3.2 可复合筛选

- 按任务类型(原款/新款/采购/精修/客户定制/常规定制)
- 按优先级(normal / urgent / critical)
- 按 `derived_status`
- 按创建者 / 接单人(支持按组过滤)
- 按日期范围
- 关键字(任务号、产品编码、标题模糊)

### 3.3 列表项字段

```
任务号 | 类型 | 优先级 | derived_status | 创建者(组) | 当前处理人(组) | 创建时间 | 更新时间 | 操作(查看详情)
```

### 3.4 详情页(所有点击任务都去同一个详情页)

见主文档 §9.2 TaskDetail 响应契约。

### 3.5 新品开发批量 SKU · Excel 唯一入口(v1 简化)

**背景**:运营线下本就用 Excel 组织 SKU 数据。当前系统在"新品开发 → 批量 SKU"下再要求在页面上逐行填写一遍,属于二次手工劳动。本节收敛为 **Excel 唯一入口**,消除一套冗余的并行字段维护链路。

#### 3.5.1 范围

- **仅限任务类型 `new_product_development`** 的"批量 SKU"子入口
- 单品 SKU 入口保留不变
- 其他任务类型(原款 / 精修 / 客户定制 / 常规定制 / 采购)**不提供** Excel 批量入口

#### 3.5.2 交互四步

1. 运营在新品开发创建页,选择 `批量 SKU` 模式
2. 点击 `下载 SKU 模板`(`GET /v1/tasks/batch-create/template.xlsx`)→ 后端根据当下数据库字典**动态生成** Excel
3. 运营本地填写后,上传文件(`POST /v1/tasks/batch-create/parse-excel`)
4. 前端展示**预览表 + 逐行错误定位**;运营就地修改 / 重新上传 → 确认 → 走现有 `POST /v1/tasks`(`batch_sku_mode=multiple`)创建

任务级字段(类目 / 材料 / 成本模式 / 基础价 / 任务级设计要求 / **任务级参考图**)仍在创建页顶部**统一填写一次**,不进 Excel(与运营原话"不用每个款式都放图片参考"一致)。

#### 3.5.3 模板结构(多 sheet)

| Sheet | 内容 | 生成策略 |
| --- | --- | --- |
| `SKU 数据` | 列 = **新品开发单品 SKU 必填字段 1:1 映射**(字段源:`domain/task_sku_item.go` + 单品创建校验清单,唯一真源) | 动态 |
| `填写说明` | 每列含义 / 是否必填 / 取值规则 | 静态模板文本 |
| `字典·类目` | 从 `category_*` 数据查询 | 动态 |
| `字典·材料模式` | 从当前材料模式枚举 | 动态 |
| `字典·成本模式` | 从当前成本模式枚举 | 动态 |

Sheet 1 中需枚举校验的列使用 Excel 原生的"数据校验 → 序列"引用对应字典 sheet,**离线填写时亦有下拉**。

#### 3.5.4 后端契约(R3 前端一屏化时落地;不依赖 v1 主架构签字顺序)

| 端点 | 说明 | 权限 |
| --- | --- | --- |
| `GET /v1/tasks/batch-create/template.xlsx?task_type=new_product_development` | 动态生成 Excel 模板(multi-sheet) | 登录用户 |
| `POST /v1/tasks/batch-create/parse-excel`(multipart) | 解析并返回 `{ preview: BatchItem[], violations: [{row, column, code, message}] }` | 登录用户 |

解析端点**不创建任务**,仅返回预览;前端拿预览后仍走**现有** `POST /v1/tasks`(`batch_sku_mode=multiple`)。

#### 3.5.5 前端改造

- **移除** 批量 SKU 的多行内联编辑器(含行新增/删除、行内校验、跨行参考图挂载等组件)
- 换成"下载 / 上传 / 预览 / 确认"四步组件
- 预览表只读 + 错误行标红 + 错误浮窗提示;运营改错回到本地 Excel 重新上传

#### 3.5.6 字段口径唯一源

- 字段清单、必填性、校验规则 = **单品 SKU 创建时的必填清单**(以 `service/task_batch_create.go` + `domain/task_sku_item.go` 中单品路径的校验逻辑为唯一真源)
- 模板列数 / 顺序 / 枚举字典随代码演进**自动跟随**,不做独立维护
- 若未来单品必填字段变化,模板会随下一次 `GET .../template.xlsx` 自动反映

#### 3.5.7 本节明确不做(防止未来需求混入)

| 被排除的延展 | 替代路径 |
| --- | --- |
| 竞品链接引用模式(`benchmark_referenced`) | 推迟到后续版本迭代 |
| 变量轴 + 矩阵编辑器 | 推迟到后续版本迭代 |
| 混合模式(`hybrid`) | 推迟到后续版本迭代 |
| 竞品商品信息自动抓取 | 推迟到后续版本迭代 |
| 其他任务类型的 Excel 批量入口 | 本轮不开口子,避免入口复杂度发散 |

#### 3.5.8 验收

| 编号 | 验收项 |
| --- | --- |
| IA-A9 | 新品开发创建页在"批量 SKU"模式下**不再**展示旧多行内联编辑器 |
| IA-A10 | `GET /v1/tasks/batch-create/template.xlsx` 返回的 Excel 列集合与单品 SKU 必填字段 100% 一致(单测校验) |
| IA-A11 | 上传 100 行 Excel 的解析响应 P95 < 2s,错误定位精确到行列 |
| IA-A12 | 解析端点**不创建任务**;仅 `POST /v1/tasks` 创建 |

#### 3.5.9 任务草稿(`task-drafts`) — v3 新增

**动机**:批量 SKU Excel 流程涉及"任务级字段(类目 / 材料 / 成本 / 任务级设计要求 / 任务级参考图) + 本地 Excel 多步填写",过程较长。误关弹窗、登出会话或临时切换任务时若不保留进度,运营需重填一遍,体验不可接受。因此 **v1 将"草稿"作为创建弹窗的必备能力**,且其用途不限于批量 SKU — 所有创建弹窗的"保存草稿"按钮共用此契约。

**范围**:
- 所有 7 种任务类型的创建弹窗(原款 / 新款单 SKU / 新款批量 SKU / 采购 / P 图 / 客户定制 / 常规定制)顶部都提供 `[保存草稿]` 按钮
- 批量 SKU 模式下,任务级字段在 Step 2 之前即可保存为草稿,Excel 解析结果也随之存储
- 草稿**不入池、不分配模块、不产生任务号**

**端点**:

| 端点 | 说明 | 权限 |
| --- | --- | --- |
| `POST /v1/task-drafts` | 新建或更新草稿;body shape 与 `POST /v1/tasks` 完全一致,额外可带 `draft_id`(有则更新) | 登录用户 |
| `GET /v1/me/task-drafts?task_type=&limit=&cursor=` | 我的草稿列表 | 登录用户 |
| `GET /v1/task-drafts/{draft_id}` | 读单条(用于回填创建弹窗) | 仅草稿创建者 |
| `DELETE /v1/task-drafts/{draft_id}` | 删除草稿 | 仅草稿创建者 |

**表结构**:

```sql
task_drafts (
  id BIGINT PK,
  owner_user_id BIGINT NOT NULL,
  task_type VARCHAR(64) NOT NULL,
  payload JSON NOT NULL,
  expires_at DATETIME NOT NULL,  -- 7 天后过期
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  INDEX (owner_user_id, task_type, expires_at)
)
```

**生命周期**:
- 默认 **7 天** 过期,过期由定时 job 硬删
- 用户手动 `DELETE` 立即生效
- 成功 `POST /v1/tasks` 后,若 body 携带 `source_draft_id`,后端在事务内同时删除该草稿
- 单用户同 `task_type` 最多 **20 条活跃草稿**,超出先删最老

**不做**(防止发散):
- 草稿**不跨用户共享**、**不支持多人协同**
- 草稿**不参与**全局搜索 / 统计报表
- 草稿**不挂载真实 OSS 资产**(已上传的参考图若草稿过期/删除,OSS 对象交由"孤儿文件清理 job"处理,不在本节定义)

**个人中心入口**:详见 §7.2"我的任务"下新增的"草稿" tab。

**验收**:

| 编号 | 验收项 |
| --- | --- |
| IA-A13 | 创建弹窗关闭前若字段非空,提示"保存为草稿 / 丢弃 / 取消"三选一 |
| IA-A14 | 7 天过期 job 每日触发,过期草稿从表中硬删 |
| IA-A15 | `POST /v1/tasks` body 带 `source_draft_id` 且创建成功 → 对应草稿在同事务内删除 |
| IA-A16 | 同 `owner_user_id + task_type` 超 20 条时,`POST /v1/task-drafts` 先删最老后插入 |

---

## 4. 全局搜索

### 4.1 覆盖对象(v1 MVP)

| 对象 | 搜索字段 | 权重建议 |
| --- | --- | --- |
| 任务 | task_no、title、requirement_note、erp_product_code、online_order_no | 高 |
| 资产 | file_name、source_task_module_id 上的 task_no | 中 |
| 产品(ERP 快照) | erp_product_code、product_name | 中 |
| 用户 | nickname、username、email | 低(仅 DeptAdmin+ 可见) |

### 4.2 端点

- `GET /v1/search?q=&scope=&limit=20`
- `scope` 可选 `all / tasks / assets / products / users`(默认 all)
- 返回统一 envelope,按对象类型分组:
  ```json
  {
    "query": "YB-2026",
    "results": {
      "tasks":    [ { "id": 484, "task_no": "YB-2026-00123", "highlight": "..." } ],
      "assets":   [ { "asset_id": 9911, "file_name": "CC-2026-main.psd" } ],
      "products": [ { "erp_code": "SKU-xxx", "product_name": "..." } ],
      "users":    []
    }
  }
  ```

### 4.3 权限

- `tasks`、`assets`、`products` 对登录用户全量可见(Layer 1)
- `users` 对象仅 `DeptAdmin` 及以上可见;低权限返回空数组,不报错

### 4.4 技术选型(R2)

- v1 MVP:MySQL LIKE + 多 union + 每张表主键索引,性能够 10w 任务规模
- 后续 R7+ 视 P95 再决定是否引入 Elasticsearch / Meilisearch(进入 U 表)

---

## 5. 用户管理(组织菜单下)

### 5.1 三级授权模型

| 角色 | 可见范围 | 可写能力 | 备注 |
| --- | --- | --- | --- |
| **SuperAdmin** | 全公司 | 增删用户、跨部门调配、组织架构调整(部门/组增删改) | 仅系统超管 |
| **HRAdmin** | 全公司 | 同 SuperAdmin(除删部门),管理员工基础资料 | 已存在账号 `HRAdmin` |
| **DeptAdmin** | 本部门 | 1) 调整本部门组 / 成员分布;2) 改本部门成员角色(不可升 SuperAdmin);3) 启停本部门成员账号;4) **可把本部门成员移出到其他部门**(需 SuperAdmin 最终确认);5) **不能把其他部门成员拉入** | |
| **TeamLead** | 本组 | **仅启停本组成员账号**(其他操作一律不给) | 不可改角色、不可调组 |
| **Member** | 本组(只读) | 无 | 见自己的部门/组信息只读 |

### 5.2 跨部门移动的工作流

1. DeptAdmin(源部门)在"组织 → 用户"中选择本部门成员,点"移出部门"
2. 系统生成一条 `org_move_request`,状态 `pending_super_admin_confirm`
3. SuperAdmin 在"组织 → 跨部门调配"待办中看到请求,批准 / 驳回
4. 批准后:成员 `department` / `team_codes` 更新,写审计事件 `user_department_changed_by_admin`
5. 驳回后:成员保持原部门,通知发起人

> 注:**DeptAdmin 不能主动把其他部门成员拉入**,必须由源部门 DeptAdmin 发起 + SuperAdmin 审批,或 SuperAdmin 直接操作。

### 5.3 API(R3)

| 端点 | 权限 |
| --- | --- |
| `GET /v1/users?department=&team=&keyword=` | DeptAdmin+ |
| `POST /v1/users` | HRAdmin / SuperAdmin |
| `PATCH /v1/users/{id}` | 按字段分级(见 §5.4) |
| `DELETE /v1/users/{id}` | SuperAdmin |
| `POST /v1/users/{id}/activate` / `deactivate` | TeamLead+(本组)/ DeptAdmin(本部门)/ 以上 |
| `POST /v1/departments/{id}/org-move-requests` | DeptAdmin(源部门) |
| `POST /v1/org-move-requests/{id}/approve` | SuperAdmin |
| `GET /v1/teams?department=` | 登录用户 |
| `POST /v1/teams` | SuperAdmin / HRAdmin / DeptAdmin(本部门) |
| `PATCH /v1/teams/{id}` | 同上 |
| `DELETE /v1/teams/{id}` | SuperAdmin / HRAdmin(组内无活跃任务时) |

### 5.4 字段级授权矩阵(PATCH /v1/users/{id})

| 字段 | SuperAdmin | HRAdmin | DeptAdmin(本部门) | TeamLead(本组) |
| --- | --- | --- | --- | --- |
| `nickname`/`phone`/`email` | ✓ | ✓ | ✓ | ✗ |
| `department` / `team_codes` | ✓(直改) | ✓(直改) | 仅本部门内调组;跨部门需走 org-move-request | ✗ |
| `roles`(DeptAdmin/TeamLead/Member) | ✓ | ✓(不可授 SuperAdmin) | 本部门内(不可授 DeptAdmin)| ✗ |
| `is_active`(启停) | ✓ | ✓ | ✓ | ✓(仅本组) |
| `primary_team_code` | ✓ | ✓ | ✓ | ✗ |

### 5.5 不走审批

按 Q6.4 决策:SuperAdmin / HRAdmin / DeptAdmin 在各自授权范围内**直接生效**,无审批流。唯一例外是 §5.2 的跨部门移出需 SuperAdmin 二次确认(不是"审批流",而是"DeptAdmin 发起 + SuperAdmin 点确认"的单步交互)。

---

## 6. 组织架构调整(部门 / 组)

### 6.1 部门

- 仅 SuperAdmin 可增删部门
- 重命名可由 HRAdmin
- 删除部门前需**清空成员**,否则返回 409 CONFLICT

### 6.2 组(`org_teams`)

- SuperAdmin / HRAdmin / **DeptAdmin(本部门)** 可增删改
- 删除组前需**清空成员 + 无活跃任务占用该组作为 pool_team_code**,否则 409

---

## 7. 个人中心(头像下拉)

### 7.1 不做一级菜单

按 Q5.4 决策,个人中心**仅在头像下拉中展开**,点击任一板块跳到对应子页。

### 7.2 子页清单

| 板块 | 功能 | API |
| --- | --- | --- |
| 账户信息 | 查看 + 编辑 昵称/头像/手机/邮箱 | `GET /v1/me` / `PATCH /v1/me` |
| 安全 | 改密(旧密码 + 新密码 + 确认) | `POST /v1/me/change-password` |
| 我的组织 | 只读展示:部门 / 组列表 / 当前角色 / 管理范围(若是 DeptAdmin/TeamLead 列出 `managed_departments` / `managed_teams`) | `GET /v1/me/org` |
| 我的任务 | 跳转"任务中心 → 我的任务" tab;下辖子 tab **"进行中 / 已完成 / 草稿"**,"草稿"子 tab 读 `GET /v1/me/task-drafts`(v3 新增,详见 §3.5.9) | `GET /v1/me/task-drafts` |
| 我的待接单 | 跳转"任务中心 → 任务池" tab | — |
| 通知中心 | 见 §8 | `GET /v1/me/notifications` |

### 7.3 不做的项

按 Q5.1 / Q5.5 / C1 / C2:

- 偏好(工作台排序 / 主题 / 默认过滤)**不做**
- 我的数据 / 报表 **不做**(个人中心不放任何报表)
- 登录历史 / Token 管理 **不做**

---

## 8. 通知中心

### 8.1 v1 必做范围

数据源 = `task_module_events` + 新建 `notifications` 外表。

| 通知类型 | 触发事件 | payload |
| --- | --- | --- |
| `task_assigned_to_me` | 我被组长 / 部门管理员 reassign 接入任务 | `{ task_id, module_key, assigned_by, reason }` |
| `task_rejected` | 我作为 claimed_by 的模块被 audit 驳回 | `{ task_id, reject_reason }` |
| `claim_conflict` | 我尝试接单但抢先(占位,Q5.3 必做) | `{ task_id, module_key }` |
| `pool_reassigned` | 我所在组新增一条 pool 任务(可配置开关) | `{ task_id, module_key }` |
| `task_cancelled` | 我参与的任务被关闭 | `{ task_id, cancel_reason, cancelled_by }` |

> **`task_mentioned`(@评论触发通知)暂不进 v1 枚举**(v3 评审追加):FE Plan 评审中前端曾提议新增一类 `task_mentioned` 用于评论 `@成员` 触发的通知。经评估:通知 type 扩展会同时牵动 `notifications` 表、WebSocket 推送(§9)、通知中心筛选 UI、前端通知 iconography 多处改动,与 v1 交付目标冲突。**v1 仅前端在评论区做 `@` 命中的 UI 高亮**(悬停显示被 @ 用户卡片),**不触发通知**;`task_mentioned` 作为 `notification_type` 延后到 v1.x 再评估,由届时统一扩展决策。

### 8.2 表结构

```sql
notifications (
  id BIGINT PK,
  user_id BIGINT NOT NULL,
  notification_type VARCHAR(64) NOT NULL,
  payload JSON NOT NULL,
  is_read TINYINT(1) NOT NULL DEFAULT 0,
  read_at DATETIME NULL,
  created_at DATETIME NOT NULL,
  INDEX (user_id, is_read, created_at)
)
```

### 8.3 端点

| 端点 | 说明 |
| --- | --- |
| `GET /v1/me/notifications?is_read=&limit=&cursor=` | 列表 |
| `POST /v1/me/notifications/{id}/read` | 标记已读 |
| `POST /v1/me/notifications/read-all` | 全部已读 |
| `GET /v1/me/notifications/unread-count` | 右上角 badge 用 |

### 8.4 渠道

- **仅站内**(Q7.1)
- 头像右上角 badge 显示未读数,点击进通知中心
- **不做** 企微 / 钉钉 / 邮件推送(v1 限定)

### 8.5 生成策略

- 后端在 `task_module_events` 写入时同事务触发"通知生成器",按规则决定是否给哪些 user_id 生成 `notifications` 行
- 生成器 = 一组规则(`domain/notification_rules.go`,R3 落地)
- 失败不回滚主事务(fire-and-forget,写日志)

---

## 9. WebSocket 实时推送

### 9.1 触发范围(Q4.4 + C1 限定)

v1 仅推送以下事件到在线客户端:

| 事件 | 推送给 | 用途 |
| --- | --- | --- |
| `task_pool_count_changed` | 池组成员(按 team_code 订阅) | 任务中心 → 任务池 tab 数字实时变 |
| `my_task_updated` | 具体 user_id | 我的任务列表实时变 |
| `notification_arrived` | 具体 user_id | 头像 badge 实时 +1 |

### 9.2 不做

- 所有任务列表全员广播(成本高,意义低)
- 报表卡片广播(R5 单独评估)

### 9.3 实现

- WebSocket 入口:`/ws/v1`(Bearer token 鉴权)
- 消息格式:`{ "type": "...", "payload": {...} }`
- 客户端无连接时依赖轮询补偿(前端 15s 回退轮询)

---

## 10. 前端菜单控制文件(瘦身后)

`config/frontend_access.json` 在 v1 仅保留:

```jsonc
{
  "roles": {
    "SuperAdmin": { "menus": ["tasks", "org", "assets", "reports", "profile"] },
    "HRAdmin":    { "menus": ["tasks", "org", "assets", "profile"] },
    "DeptAdmin":  { "menus": ["tasks", "org", "assets", "profile"] },
    "TeamLead":   { "menus": ["tasks", "org(readonly)", "assets", "profile"] },
    "Member":     { "menus": ["tasks", "assets", "profile"] }
  }
}
```

- `actions` 字段在 R4 前保留兼容,R4 清理到 deprecated shim

---

## 11. 验收标准(本子文档追加)

| 编号 | 验收项 | 关联章节 |
| --- | --- | --- |
| IA-A1 | 顶部导航呈现 4 个一级菜单(任务中心 / 组织 / 资产管理中心 / 报表),其中"报表"仅 SuperAdmin 可见 | §1 / §2 |
| IA-A2 | 全局搜索对任务号、产品编码、资产文件名关键字均能命中 | §4 |
| IA-A3 | `GET /v1/search?q=` 在 10w 任务规模下 P95 < 300ms | §4.4 |
| IA-A4 | DeptAdmin 操作本部门用户 PATCH 时,`department`/`team_codes` 的跨部门修改返回 409 + `cross_department_requires_super_admin` | §5.2 / §5.4 |
| IA-A5 | TeamLead 调用 PATCH /v1/users/{id} 除 `is_active` 外任何字段返回 403 | §5.4 |
| IA-A6 | 个人中心头像下拉包含 6 个板块(账户 / 安全 / 我的组织 / 我的任务 / 我的待接单 / 通知中心) | §7.2 |
| IA-A7 | 通知中心在 audit 驳回事件 5s 内生成一条 `task_rejected` 通知 | §8 |
| IA-A8 | WebSocket 仅推送 §9.1 的 3 类事件;池 tab 数字在接单 1s 内更新 | §9 |

---

## 12. 变更记录

| 版本 | 日期 | 变更 | 签字 |
| --- | --- | --- | --- |
| Draft v1 | 2026-04-17 | 初稿;合并 Q1 / Q5 / Q6 / Q7.1 / C1 / C3 / 全局搜索追加 / 资产管理中心菜单 | — |
| Draft v2 | 2026-04-17 | 新增 §3.5 新品开发"Excel 唯一入口"批量 SKU 方案(移除多行内联编辑,字段与单品必填 1:1 对齐,延展能力延后),对齐主文档 Draft v3 §17 追加行 | — |
| Draft v3 | 2026-04-17 | 合并 FE Plan 评审:§3.5.9 新增任务草稿端点(7 天过期、20 条上限、创建成功级联删除)、IA-A13~A16 验收;§7.2 我的任务新增"草稿"子 tab;§8.1 `task_mentioned` 暂不进 v1 明确标注 | **已签字**(2026-04-17) |
| v1.1 | 2026-04-17 | **R1.5 DDL 对齐 · 仅审计**:核查全文所有列引用,未发现字段幻觉(`priority` 真实列、`task_drafts` / `notifications` / `org_move_requests` 为 v1 新表)。不修改表结构或端点定义。依据 `docs/iterations/V1_R1_5_DDL_ALIGNMENT.md` | **已签字**(2026-04-17,审计 only,无结构变更) |
| v1.2 | 2026-04-25 | **R4-SA-D 签字生效 · §2.1 报表 L1 实装规则快照新增**:5 条合同级规则固化(数据源走 task_module_events JOIN task_modules JOIN tasks · dwell 语义校正:module_key/task_id 在 task_modules 侧 · task_module_events 无 state_enter_at/state_exit_at 列 · CTE 配对 enter-like/exit-like event + window rank P95 · 5 module_key 固定返行;throughput.archived v1 与 completed 同集合;SuperAdmin-only 报表 + `reports_super_admin_only` deny + `report_access_denied` 审计;搜索 `users[]` 仅 SuperAdmin+HRAdmin 可见 · 其余角色永返 [])。依据 `docs/iterations/V1_R4_SA_D_REPORT.md` §Architect Adjudication | **已签字**(2026-04-25) |

---

**签字**:

- 产品: **已确认**(2026-04-17)
- 后端: **已确认**(2026-04-17)
- 前端: **已确认**(2026-04-17)

> 本文于 2026-04-17 随主文档 v4 一并签字生效,进入 v1.0。后续修订走 ADR / 变更记录,不再静默修改。
