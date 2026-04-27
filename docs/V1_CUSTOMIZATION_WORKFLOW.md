# V1 定制任务工作流细则

> 状态: **v1.1 · DDL 对齐已签字生效**(2026-04-17,依据 `docs/iterations/V1_R1_5_DDL_ALIGNMENT.md` v1.0);前置版本 v1.0 · 已签字生效(2026-04-17)
> 最终签字版本:Draft v2(含通用字段口径对齐 + `design_source_lookup_id` v1 单选约束)
> v1.1 变更:§3.1.1 明确五个新字段的落盘位置(方案 Y · 新建 `task_customization_orders` 子表,R2 migration 068 建表);§6.3 校正"customization_review_reference" 引用语义(`asset_storage_refs.owner_type` 记录层,不是表)。依据 `docs/iterations/V1_R1_5_DDL_ALIGNMENT.md`。
> 主次关系: 本文与 `V1_MODULE_ARCHITECTURE.md` 冲突时以主文档为准;主文档未覆盖的定制领域细节由本文补齐。
> 涵盖任务类型: `customer_customization`(客户定制)、`regular_customization`(常规定制)

---

## 1. 核心约定(与主文档一致)

- **不启用** `design` 模块(定制美工直接产出终稿)。
- **不经过系统内"客户确认"** 阻塞动作(定制美工上传终稿即进 audit)。
- audit 驳回**永远回 `customization`**,不回 `design`。
- blueprint:`basic_info → customization → audit → warehouse`。
- 池组映射:`customization.pool_team_code = customization_art`;`audit.pool_team_code = audit_customization`。
- 客户定制 vs 常规定制 **流程完全一致**,差异仅在:
  - 任务编号前缀(由任务创建服务决定,不入 blueprint)
  - 创建时的**元数据来源**(本文 §3 详述)

---

## 2. `customization` 模块状态机

### 2.1 状态集

| state | 含义 | 进入方式 | 离开方式 |
| --- | --- | --- | --- |
| `pending_claim` | 刚入池,等待定制美工组成员接单 | blueprint `basic_info.completed → customization.enter` 时 | 成员 `customization.claim` 成功 |
| `in_progress` | 已接单,定制美工制作中(沟通稿 → 终稿) | `customization.claim` 成功 | 成员 `customization.submit` 提交终稿 |
| `submitted` | 终稿已提交,下游 audit 已触发 | `customization.submit` | 1) audit 驳回 → 回 `in_progress`;2) audit 通过 → `closed` |
| `closed` | 定制模块完结(audit 通过或任务取消) | audit 通过 / 任务管理员取消 | 终态,不可逆 |

**无独立"内部自审"状态**:沟通稿、草图、终稿的迭代全部发生在 `in_progress` 内,由美工自行掌握,资产以版本号累加。

### 2.2 合法 action × state 矩阵

| action | 允许 state | 允许角色(Layer 3) |
| --- | --- | --- |
| `customization.claim` | `pending_claim` | 定制美工组成员 / 组长 / 定制美工部部门管理员 |
| `customization.reassign` | `pending_claim` / `in_progress` | 组长(本组) / 部门管理员(`定制美工部`) |
| `customization.pool_reassign` | `pending_claim` | 仅部门管理员(`定制美工部`)—— 改到其他美工组(若将来扩展) |
| `customization.asset_upload_session_create` | `in_progress` | `claimed_by` 本人 / 本组组长 |
| `customization.asset_upload_session_complete` | `in_progress` | `claimed_by` 本人 / 本组组长 |
| `customization.asset_upload_session_cancel` | `in_progress` | `claimed_by` 本人 / 本组组长 |
| `customization.submit` | `in_progress` | `claimed_by` 本人(强制 self) |
| `customization.reopen` | `submitted` | 由 `audit.rejected` 事件**系统自动触发**,不开放人工端点 |

### 2.3 事件流(task_module_events)

定制模块必写事件:

| event_type | 触发时机 | payload 关键字段 |
| --- | --- | --- |
| `entered` | blueprint 实例化该模块 | `{ "pool_team_code": "customization_art" }` |
| `claimed` | 接单成功 | `{ "claimed_team_code": "customization_art" }` |
| `reassigned` | 组内改派 | `{ "from_user_id": .., "to_user_id": .. }` |
| `submitted` | 提交终稿 | `{ "latest_asset_version_id": .., "asset_count": .. }` |
| `reopened` | audit 驳回触发回流 | `{ "reject_reason": "...", "reject_audit_event_id": .. }` |
| `closed` | audit 通过 | `{ "approved_at": .., "approved_by": .. }` |
| `pool_reassigned_by_admin` | 部门管理员跨组调度 | `{ "from_pool": .., "to_pool": .. }` |

---

## 3. 创建链路

### 3.1 客户定制(`customer_customization`)

#### 3.1.1 输入字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `online_order_no` | 是 | 线上订单号(运营手填) |
| `requirement_note` | 是 | 客户需求信息文本 |
| `attachments[]` | 可选 | 附件文件(走 OSS 直传 → `asset_storage_refs.owner_type='customization_reference'` → `reference_file_refs` 展平表,`owner_module_key='customization'`)。详见 `V1_ASSET_OWNERSHIP.md` §3 |
| `ordered_at` | 是 | 下单时间 |
| `erp_product_code` | 是 | ERP 产品编码(通过 §3.1.2 查询后写入) |
| **通用字段**(`task_deadline` / `priority` / `remark` 等) | 同主文档 G1 | 所有任务类型共享同一套通用字段;优先级**复用既有 `tasks.priority` 列**,v1 将枚举扩为 `normal / urgent / critical`(R2 加 CHECK + 复合索引,不新增列,见主 §17 Q7.5 + §8.2)。前端对 DeptAdmin / SuperAdmin 保留 `critical` 选项,Member 视角仅展示"是否加急"(off=normal / on=urgent)。 |

##### 3.1.1.1 新字段的落盘(v1.1 追加)

v0.9 并无持久化位置承载"客户定制订单"业务数据。R2 按**方案 Y** 新建子表:

```sql
-- R2 migration 068 (v1.1)
CREATE TABLE task_customization_orders (
  task_id          BIGINT      NOT NULL COMMENT 'FK tasks.id · PK 保证一对一',
  online_order_no  VARCHAR(64) NOT NULL DEFAULT '',
  requirement_note TEXT        NOT NULL,
  ordered_at       DATETIME    NULL,
  erp_product_code VARCHAR(64) NOT NULL DEFAULT '',
  created_at       DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at       DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (task_id),
  KEY idx_task_customization_orders_order_no (online_order_no),
  KEY idx_task_customization_orders_erp_code (erp_product_code),
  CONSTRAINT fk_task_customization_orders_task FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v1 客户定制订单业务字段,PK=task_id 与 tasks 一对一';
```

- **不扩 `customization_jobs`**:该表承载定价 / 审核流转状态,避免与业务数据耦合。
- **`attachments[]` 不入本表**:附件走 `reference_file_refs` 展平表,统一模型。
- 常规定制(`regular_customization`)**不写本表**,其 `design_source_lookup_id` / `design_source_snapshot` 存于 `task_details` 或 `task_sku_items`(沿用 v0.9 路径,R2 不改)。

#### 3.1.2 ERP 产品编码查询(R2 新端点)

- **上游接口**:8081 ERP Bridge 原始路径 `/open/combine/sku/query`(bridge owns,非本后端)
- **本后端包装**:`GET /v1/erp/products/by-code?code={erp_product_code}` → 内部调用 `service.ERPBridgeClient` 转发上游,返回 `ERPBridgeProductSnapshot`
- **绑定规则**:
  - 查询结果作为 `task.source_snapshot`(沿用现有 `erp_bridge_product_upsert` 连接器语义)存档
  - 任务与 ERP 产品通过 `erp_product_code` 锚定;ERP 侧后续变更**不回流**至任务快照(与 v0.9 一致)
- **失败策略**:
  - 上游 5xx / timeout:**拒绝创建任务**,前端提示"ERP 查询失败,请稍后重试"
  - 上游返回"未找到":**拒绝创建任务**,前端提示"该产品编码不存在于 ERP",由运营核实

#### 3.1.3 编号前缀

- 由 `service.TaskCreator` 按 blueprint 配置生成,建议前缀:`CC-YYYYMM-NNNNNN`(Customer Customization)

### 3.2 常规定制(`regular_customization`)

#### 3.2.1 输入字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `design_source_lookup_id` | 是 | 通过 §3.2.2 查询返回的设计源文件唯一 ID |
| `design_source_snapshot` | 自动回填 | 由后端根据 `design_source_lookup_id` 回填(文件名、创建者、版本号、OSS key 指针等) |
| `requirement_note` | 是 | 需求说明 |
| **通用字段**(`task_deadline` / `task_priority` / `remark` 等) | 同主文档 G1 | 参见 §3.1.1 通用字段行;口径与客户定制完全一致 |

> **v1 约束**:`design_source_lookup_id` **单选**(不支持多选)。未来若需"多源拼图定制"再迭代,v1 不开口子。

#### 3.2.2 设计源文件查询(v1 做基础版,不留空)

- **新端点**:`GET /v1/design-sources/search?keyword=&page=&size=`
- **数据源**:
  - **首选**:现有 `task_assets` 表(设计部产出的 `source_module_key IN ('design', 'retouch')` 且已关联归档任务的资产)
  - **扩展**:运营后台手动入库的独立"设计源文件库"(如有独立物料库则接入;v1 可先仅覆盖首选来源)
- **返回字段**:`{ id, file_name, preview_url(预签名), owner_team_code, created_at, version_no, origin_task_id(可空) }`
- **V1 MVP**:仅按文件名 / 任务号关键字全文搜索 + 按创建时间倒序,不做高级过滤;SuperAdmin 可维护独立物料库的未来迭代在 R7+ 考虑。

#### 3.2.3 编号前缀

- 建议:`RC-YYYYMM-NNNNNN`(Regular Customization)

---

## 4. 驳回回流(audit.rejected → customization.reopen)

### 4.1 触发规则(blueprint)

```
on event: audit.rejected (任何子审核阶段)
  if task.type in { customer_customization, regular_customization }:
    customization.reopen(
      to_state = in_progress,
      reopen_reason = audit_event.reject_reason,
      reopen_by = audit_event.actor_id,
      trace = audit_event.id
    )
```

### 4.2 回流后的作业人

- 默认 = 原 `claimed_by`(继续由原定制美工处理)
- 若原 `claimed_by` 已离职 / 停用 → 降级为 `pending_claim` 重新入池(`customization_art`),并写事件 `reopen_fallback_to_pool`
- 组长可在回流后使用 `customization.reassign` 换人

### 4.3 资产语义

- 驳回**不清空**历史资产,定制美工在原版本基础上**追加新版本**
- audit 的 `rejected` 事件 `payload.rejected_asset_version_id` 冻结**被驳回时的资产版本快照**,用于审计追溯(参见 `V1_ASSET_OWNERSHIP.md` §审核锚定)

---

## 5. 终态与流转

### 5.1 audit 通过

```
audit.approved → customization.close + warehouse.enter
```

- `customization` 进入 `closed`
- `warehouse` 进入 `pending`
- `customization` 的 `closed` 不再接受任何 action

### 5.2 任务取消(部门管理员)

- 入口:`POST /v1/tasks/{id}/cancel`(通用,非定制独有)
- 需理由必填;写事件 `task_cancelled`,同时级联给每个未终态模块写 `forcibly_closed`
- `customization` 的 `forcibly_closed` 事件 payload:`{ "by_task_cancel": true, "reason": "..." }`

---

## 6. 旧数据迁移映射

### 6.1 旧任务 → 新定制模块

v0.9 的定制任务数据分布在多张表(沿用 migration `055` / `056` / `054` 的 schema):

| 旧字段 / 状态 | 新模块 state | claim 快照 | 备注 |
| --- | --- | --- | --- |
| `WorkflowLane=customization` ∧ `CustomizationFlow.step=draft` | `customization.in_progress` | 从 `customization_flow` 取当前负责人 | — |
| `customization_review.state=pending` | `customization.submitted` + `audit.pending_claim` | audit 侧 `pool_team_code=audit_customization` | 同一原子事务写两条 |
| `customization_review.state=approved` | `customization.closed` + `audit.closed` + `warehouse.pending` | — | — |
| `customization_review.state=rejected_latest` | `customization.in_progress`(已 reopen 过) | 事件流回填 `reopened` 事件 | payload 可丢失,仅做一条标记 |

### 6.2 若旧数据存在 `design` 模块被定制任务使用

- 后端判定:`task.type ∈ {customer_customization, regular_customization}` ∧ 存在 `design_*` 字段非空 / 残留 `design` 历史任务状态
- 处理:**不创建** `task_modules.module_key=design` 实例,但原资产 `task_assets` 记录的 `source_module_key` 在 backfill 中批量改写为 `customization`
- 留痕:写一条 `customization.migrated_from_legacy_design` 事件,payload 含原 `design_asset_ids`

### 6.3 历史沟通稿 / 附件(v1.1 校正)

> Draft v2 写的"`reference_file_refs.context=customization_review_reference`"是误传 —— v0.9 `reference_file_refs` 不是表(它是 JSON TEXT 列,见 `V1_ASSET_OWNERSHIP.md` §3.1),`context` 不存在。真正承载"定制参考图"分类的字段是 `asset_storage_refs.owner_type='customization_reference'`(由 service 层在上传时打标)。

backfill 规则:

- 扫 `asset_storage_refs` 里 `owner_type IN ('customization_reference')` 的记录,在 `reference_file_refs` 展平表插入对应行,`owner_module_key='customization'`,`context='customization_reference'`(保留审计追溯)
- 客户定制任务创建时的附件 upload 在未来走新路径(R3 前端 → `/v1/tasks/reference-upload` → 写 `asset_storage_refs.owner_type='customization_reference'` + 写展平表 `owner_module_key='customization'`),无需额外迁移

---

## 7. 前端呈现(任务详情页 customization 模块)

> 非架构性强要求,但作为 R3 验收参考。

```
┌── customization 模块卡片 ──────────────────────────┐
│ 状态徽章: in_progress                              │
│ 接单人: 李四 · customization_art 组 · 2026-04-10  │
│                                                   │
│ 需求信息(只读,来自 basic_info 的输入字段)         │
│ ├ 订单号 / ERP 产品编码 / 下单时间 / 客户附件       │
│ └ 或:设计源文件快照(常规定制)                     │
│                                                   │
│ 资产版本列表(按 version_no 降序)                  │
│ v3  终稿 v3.psd · 李四 · 2026-04-12                │
│ v2  终稿 v2.psd · 李四 · 2026-04-11 (已被驳回)     │
│ v1  沟通稿.jpg · 李四 · 2026-04-10                 │
│                                                   │
│ 动作按钮(按 Layer 3 呈现)                          │
│ [上传新版本] [提交审核] [改派(组长)]                │
└───────────────────────────────────────────────────┘
```

---

## 8. 验收标准(本子文档追加)

| 编号 | 验收项 | 关联章节 |
| --- | --- | --- |
| C-A1 | 定制任务创建时 blueprint 不实例化 `design` 模块 | §1 / §3 |
| C-A2 | 客户定制创建时,`erp_product_code` 必须通过 `/v1/erp/products/by-code` 校验成功 | §3.1.2 |
| C-A3 | 常规定制创建时,`design_source_lookup_id` 必须命中 `/v1/design-sources/search` 结果 | §3.2.2 |
| C-A4 | audit 驳回后,`customization` 自动回流至 `in_progress`,`reopened` 事件写入 | §4 |
| C-A5 | 原 `claimed_by` 停用时,回流降级为 `pending_claim` 并写 `reopen_fallback_to_pool` | §4.2 |
| C-A6 | backfill 后,定制任务不存在 `task_modules.module_key=design` 实例 | §6.2 |

---

## 9. 变更记录

| 版本 | 日期 | 变更 | 签字 |
| --- | --- | --- | --- |
| Draft v1 | 2026-04-17 | 初稿;合并 Q2 + O1 + O2 + O3 决策 | — |
| Draft v2 | 2026-04-17 | 合并 FE Plan 评审:§3.1.1 / §3.2.1 末尾补"通用字段同主文档 G1"行(解决前端 F-U3 议题);§3.2.1 明确 `design_source_lookup_id` v1 单选;口径对齐主文档 Draft v4 | **已签字**(2026-04-17) |
| v1.1 | 2026-04-17 | **R1.5 DDL 对齐**:§3.1.1 通用字段行里 `task_priority` 更正为 `priority` 复用;新增 §3.1.1.1 客户定制五个新字段落盘方案 Y(R2 建 `task_customization_orders`);§6.3 校正 `reference_file_refs.context` 幻觉,改为 `asset_storage_refs.owner_type='customization_reference'` 为真源。依据 `docs/iterations/V1_R1_5_DDL_ALIGNMENT.md` | **已签字**(2026-04-17) |

---

**签字**:

- 产品: **已确认**(2026-04-17)
- 后端: **已确认**(2026-04-17)
- 前端: **已确认**(2026-04-17)

> 本文于 2026-04-17 随主文档 v4 一并签字生效。定制流程的任何字段 / 状态机 / 端点变更必须先改本文再改代码。
