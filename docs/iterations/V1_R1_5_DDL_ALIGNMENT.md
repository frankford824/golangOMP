# V1 R1.5 · DDL 对齐与 R2 回退报告

> 状态:**v1.0 · 已签字生效**(2026-04-17)
> 目标:修复 R2 执行暴露的权威文档字段幻觉 → 把 4 份权威文档对齐真实仓库 DDL → 重写 R2 prompt v2 → 重新 R2。
> 阻塞范围:**R2 / R3 / R4 全部无法启动**,直到本轮签字。**已解锁**。
> 本文是方案文档,不直接改代码或权威文档。§6 清单已全部落地(4 份 v1.1 文档 + R2 v2 prompt)。

---

## 1. 触发原因

R2 prompt v1 已被 Codex 执行(9 migration SQL + 3 tool + smoke_test 全部写完),但:

1. **形式上完成,实证上零**:无 `MYSQL_DSN` + 无种子 dump,forward / backfill / rollback / 10w 性能门槛全部未执行;smoke test 在无 DB 情况下 `go test` 返回假绿(`ok`)。
2. **Codex 的 schema probe 掩盖了真正的上游缺陷**:`hasColumn("task_assets","flow_stage")`、`SELECT status FROM tasks` 等代码在假设字段存在;真上线时会静默失败或报 1054 / 1146。

经对全量 `db/migrations/` 的回放式 DDL 审计(共 59 份迁移,001~058 + `039_v8_identity_org_scope_extension`),确认**权威文档有 7 处字段幻觉**,R2 是在错误前提上构建的。

---

## 2. DDL 真值快照(R1.5 基线)

### 2.1 `tasks` 表真实列(001 + 037 + 046 + 047 + 051 + 054 叠加后)

```text
id BIGINT PK
task_no VARCHAR(64)           -- UNIQUE
source_mode VARCHAR(32)       -- existing_product | new_product
product_id BIGINT NULL
sku_code VARCHAR(64)
product_name_snapshot VARCHAR(255)
task_type VARCHAR(32)         -- regular | ... (v0.9 取值枚举见 domain/task.go)
operator_group_id BIGINT NULL
creator_id BIGINT
designer_id BIGINT NULL
current_handler_id BIGINT NULL
task_status VARCHAR(64)       -- ✱ NOT `status`,默认 'Draft'
priority VARCHAR(16)          -- ✱ 默认 'normal',已存在
deadline_at DATETIME NULL
need_outsource TINYINT(1)
owner_team VARCHAR(128)       -- 037
is_outsource TINYINT(1)       -- 037
is_batch_task TINYINT(1)      -- 046
batch_item_count INT          -- 046
batch_mode VARCHAR(32)        -- 046
primary_sku_code VARCHAR(64)  -- 046
sku_generation_status VARCHAR(32) -- 046
owner_department VARCHAR(64)  -- 047
owner_org_team VARCHAR(64)    -- 047
requester_id BIGINT NULL      -- 051
customization_required TINYINT(1)        -- 054
customization_source_type VARCHAR(32)    -- 054
last_customization_operator_id BIGINT    -- 054
warehouse_reject_reason VARCHAR(255)     -- 054
warehouse_reject_category VARCHAR(64)    -- 054
created_at / updated_at DATETIME

-- ⚠ 不存在:is_urgent / status / task_priority / workflow_lane(独立列)
```

### 2.2 `task_assets` 表真实列(004 + 020 + 034 + 035 + 036 + 049 叠加后)

```text
id BIGINT PK
task_id BIGINT                 -- FK tasks.id
asset_type VARCHAR(32)         -- ✱ 036 改为 {reference, source, delivery} 三值(原四值已归并)
version_no INT
upload_request_id VARCHAR(64)  -- 020
storage_ref_id VARCHAR(64)     -- 020 → FK asset_storage_refs.ref_id
asset_id BIGINT NULL           -- 034
asset_version_no INT NULL      -- 034
file_name VARCHAR(255)
original_filename VARCHAR(255) -- 034
mime_type VARCHAR(255) NULL    -- 020
file_size BIGINT NULL          -- 020
file_path VARCHAR(1024) NULL
storage_key VARCHAR(255) NULL  -- 034
whole_hash VARCHAR(255) NULL
upload_status VARCHAR(32) NULL -- 034
preview_status VARCHAR(32) NULL -- 034
upload_mode VARCHAR(32) NULL   -- 036:small | multipart
uploaded_by BIGINT
uploaded_at DATETIME NULL      -- 034
remote_file_id VARCHAR(128) NULL -- 035
scope_sku_code VARCHAR(64) NULL  -- 049
remark TEXT
created_at DATETIME

-- ⚠ 不存在:flow_stage / source_module_key(本列由 R2 新增) / source_task_module_id / is_archived / cleaned_at / deleted_at
```

**关键**:`flow_stage` 是**纯幻觉字段**。判别"哪个模块产生了此资产"的真实信号组合是:

```text
task_assets.asset_type + tasks.task_type + tasks.customization_required
```

### 2.3 `reference_file_refs` 根本**不是表**

- 真身是两个 JSON TEXT 列:
  - `task_details.reference_file_refs_json`(042 引入,存 `domain.ReferenceFileRef[]`)
  - `task_sku_items.reference_file_refs_json`(`tmp/release_v09_schema_guarded_patch.sql` 引入,SKU 级参考图)
- 每个 `ReferenceFileRef.ref_id` 指向 `asset_storage_refs.ref_id`(020 表,有 `owner_type / ref_key / status / file_name / mime_type / file_size` 等)。
- `asset_storage_refs.owner_type` 已经有 `task_create_reference / customization_reference / audit_reference` 等取值 —— **它才是 `owner_module_key` 的真实承载点的最近邻**。

### 2.4 `customization_jobs` 表实际列(054 + 055 + 056 + 053 叠加后)

```text
id / task_id / source_asset_id / current_asset_id
customization_level_code / customization_level_name
unit_price / weight_factor / note TEXT
customization_review_decision / decision_type
assigned_operator_id / last_operator_id
pricing_worker_type
status VARCHAR(64)             -- 默认 'pending_customization_production'
warehouse_reject_reason / warehouse_reject_category
order_no VARCHAR(64)           -- 055 添加
review_reference_unit_price / review_reference_weight_factor  -- 056 添加
created_at / updated_at

-- ⚠ 不存在:online_order_no / requirement_note / ordered_at / erp_product_code / attachments_json
```

定制文档 §3.1.1 的 5 个客户定制输入字段**全部是 v1 新字段**,v0.9 无持久化位置。R2 需要决定落盘到:

- 方案 X:扩 `customization_jobs` 表 +5 列
- 方案 Y:新建 `task_customization_orders(task_id PK, online_order_no, requirement_note, ordered_at, erp_product_code, attachments_ref_ids_json)` 子表
- 方案 Z:塞 `task_details.demand_text` / `task_details.note` 为 JSON 文本

### 2.5 `notifications / task_drafts / org_move_requests` 都不存在

这三张表在仓库内没有任何 DDL,是 v1 纯新增表。不属于幻觉,属于 R2 需新建。

---

## 3. 幻觉清单(总览)

| # | 权威文档写的 | 真实仓库 | 位于文档 | 严重度 |
|---|---|---|---|---|
| 1 | `tasks.status` | `tasks.task_status`(001#38) | 主 §11.2 | 低(改名) |
| 2 | `tasks.is_urgent` | 不存在 | 主 §11.2 + 资产 §6 | 中(backfill 空转) |
| 3 | 新增 `tasks.task_priority` | `tasks.priority` 001#39 已有 VARCHAR(16) DEFAULT 'normal' | 主 §17 Q7.5 | **高**(列重复) |
| 4 | `task_assets.flow_stage` | 不存在;真信号是 `asset_type + tasks.task_type + customization_required` | 资产 §6.1 + 主 §11.2 | **严重**(整章 backfill 规则错) |
| 5 | `reference_file_refs` 表 | 不是表;是 `task_details.reference_file_refs_json` 和 `task_sku_items.reference_file_refs_json` 两个 JSON TEXT 列,+ `asset_storage_refs` 子实体 | 资产 §3 / §6.3 | **严重**(整章基于错误模型) |
| 6 | `audit_reference`(v0.9 056 引入) | 056 是定制评审参考价 DECIMAL,与审核参考图无关;审核参考图真实挂在 `asset_storage_refs.owner_type='audit_reference'` 的 JSON ref 上 | 资产 §3.2 | 中(出处错) |
| 7 | 定制 §3.1.1 五个新字段 | 任何表都没有 | 定制 §3.1.1 | 中(未说明落盘位置) |

---

## 4. 处理决策(已询问用户并签字的选项)

- **参考图展平方案**:B —— 新建 `reference_file_refs` 表,从两个 JSON 列展平,加 `owner_module_key`;两个 JSON 列保留到 R6-slim。
- **R2 回退范围**:A —— 物理删除全部 9 份 SQL + 3 tool 包 + R2 报告。
- **优先级字段**:复用 `tasks.priority`,扩枚举为 `normal | urgent | critical`;**不新增 `task_priority` 列**。
- **flow_stage 推断**:改为 `asset_type (reference|source|delivery) × tasks.task_type × tasks.customization_required` 组合规则。
- **定制 §3.1.1 五个新字段落盘**:**方案 Y · 已签字**(2026-04-17)。新建 `task_customization_orders` 子表:
  ```
  task_customization_orders
    task_id BIGINT PK FK tasks.id
    online_order_no VARCHAR(64) NOT NULL DEFAULT ''
    requirement_note TEXT NOT NULL
    ordered_at DATETIME NULL
    erp_product_code VARCHAR(64) NOT NULL DEFAULT ''
    created_at / updated_at DATETIME
    KEY idx_task_customization_orders_order_no (online_order_no)
  ```
  附件(`attachments[]`)**不存本表**,走 `asset_storage_refs.owner_type='customization_reference'` +`reference_file_refs` 展平表(owner_module_key='customization'),与审核参考图、基础参考图统一模型。

---

## 5. R2 物理回退记录(已执行 · 2026-04-17)

已删除:

```text
db/migrations/059_v1_0_task_modules.sql
db/migrations/060_v1_0_task_module_events.sql
db/migrations/061_v1_0_task_assets_source_module_key.sql
db/migrations/062_v1_0_reference_file_refs_owner_module_key.sql
db/migrations/063_v1_0_task_drafts.sql
db/migrations/064_v1_0_notifications.sql
db/migrations/065_v1_0_org_move_requests.sql
db/migrations/066_v1_0_task_priority.sql
db/migrations/067_v1_0_asset_lifecycle_state.sql
cmd/tools/migrate_v1_forward/                (全目录)
cmd/tools/migrate_v1_backfill/                (全目录,含 smoke_test.go / phases.go / query.go / mapping.go / main.go / helpers.go)
cmd/tools/migrate_v1_rollback/                (全目录)
cmd/tools/internal/v1migrate/                 (全目录)
docs/iterations/V1_R2_REPORT.md
```

保留:

- `docs/api/openapi.yaml` 里的 `Error` / `TaskDetailAggregate` 孤儿 schema 清理(-146 行,R2 唯一应保留的成果)。

验证:

- `go build ./...` → 退出 0(本地 Windows)。
- `db/migrations/` 文件数回到 59(无 06x_v1_0 系列)。

---

## 6. R1.5 签字后要做的事(按顺序)

### 6.1 修 `V1_MODULE_ARCHITECTURE.md` → v1.1

- **§11.2 backfill 表**:把所有 `status` 字段改为 `task_status`;删除 `is_urgent` 行;`task_priority` 改为 `priority`。
- **§17 Q7.5 任务优先级**:明确"复用 `tasks.priority`,v1 将 enum 从 `normal` 单值扩展为 `normal | urgent | critical`,由 R2 加 CHECK + `(priority, created_at)` 复合索引,**不新增列**"。
- **§10.1 task_status 聚合映射表**:核对每行左侧旧 status 是否是真实的 v0.9 `task_status` 取值(`Draft / PendingAssignment / InDesign / PendingAuditA / InAuditA / PendingAuditB / InAuditB / InWarehouse / Completed / Archived / Cancelled` —— 以 `domain/task.go` 枚举为权威)。如发现新值,补;如发现幻觉,删。
- 末尾追加"v1.1 变更记录"表格;签字行保留原签字 + 追加 R1.5 签字。

### 6.2 修 `V1_ASSET_OWNERSHIP.md` → v1.1(改动最大)

- **§2.1 task_assets 扩列**:明确"R2 新增 `source_module_key / source_task_module_id / is_archived / archived_at / archived_by / cleaned_at / deleted_at` 共 7 列"。保留。
- **§3 整章重写**(**核心**):
  - §3.1 现状:`reference_file_refs` 不是表,是 2 个 JSON TEXT 列 + `asset_storage_refs` 作为附属元数据;所有权分散。
  - §3.2 v1 方案:**新建 `reference_file_refs` 展平表**,字段:
    ```
    id BIGINT PK
    task_id BIGINT NOT NULL FK tasks.id
    sku_item_id BIGINT NULL FK task_sku_items.id
    ref_id VARCHAR(64) NOT NULL FK asset_storage_refs.ref_id
    owner_module_key VARCHAR(32) NOT NULL  -- basic_info | audit | customization
    context VARCHAR(64) NULL               -- 保留原 JSON 对象里 source 字段语义(task_create_reference / customization_reference / audit_reference)
    attached_at DATETIME NOT NULL
    UNIQUE (task_id, ref_id, sku_item_id)
    KEY (owner_module_key, task_id)
    ```
  - §3.3 归属规则:
    - `asset_storage_refs.owner_type='task_create_reference'` → `basic_info`
    - `asset_storage_refs.owner_type='audit_reference'` → `audit`
    - `asset_storage_refs.owner_type='customization_reference'` → `customization`
    - `asset_storage_refs.owner_type` 其他取值:按 task 主 lane 兜底。
  - §3.4 并存策略:两个 JSON 列保留到 R6-slim,期间 API 读取以展平表为准,写入双写(R3 handler 负责)。
- **§6.1 backfill 规则**(**核心**):删除所有 `flow_stage` 引用,替换为:
  ```
  # 推断 source_module_key:
  IF asset_type = 'reference':
     source_module_key = 'basic_info'   # 任务创建参考图
  ELIF asset_type IN ('source','delivery'):
     IF tasks.customization_required = 1:
        source_module_key = 'customization'
     ELIF tasks.task_type LIKE '%retouch%':
        source_module_key = 'retouch'
     ELSE:
        source_module_key = 'design'
  ```
  保留 `backfill_warning` 事件用于兜底路径审计。
- **§6.3 新增子章节**:reference_file_refs 展平 backfill 算法(解析 JSON → insert 展平表 → 命中 `asset_storage_refs.owner_type` 映射 `owner_module_key`)。

### 6.3 修 `V1_CUSTOMIZATION_WORKFLOW.md` → v1.1

- **§3.1.1 末尾追加**:"字段落盘位置:R2 新建 `task_customization_orders(task_id PK, online_order_no VARCHAR(64), requirement_note TEXT, ordered_at DATETIME, erp_product_code VARCHAR(64), ...)` 子表;附件通过 `/v1/tasks/reference-upload` 走 `asset_storage_refs` + `reference_file_refs` 展平表(owner_module_key='customization')。"
- **§6.3 历史沟通稿 / 附件**:明确"v0.9 `asset_storage_refs.owner_type='customization_reference'` 记录 → backfill 时插入 `reference_file_refs` 展平表,`owner_module_key='customization'`"。
- 变更表追加 v1.1 行。

### 6.4 修 `V1_INFORMATION_ARCHITECTURE.md` → v1.1(若有幻觉)

- 核查 §5.2 `org_move_requests` 字段定义:由于该表 v0.9 不存在,属 v1 纯新增,无需对齐,只需与主文档 §11.2 命名一致。
- 核查 §8.2 `notifications` 字段定义:同上,v1 新增表,保持命名一致即可。
- 若无字段幻觉,仅追加"v1.1 已对齐真实 DDL 口径"说明;否则按实际修正。

### 6.5 重写 `prompts/V1_R2_DATA_LAYER.md` → v2

核心变化:

| R2 v1 (已废) | R2 v2 (新) |
|---|---|
| 9 份 migration 059~067 | 9 份 migration 059~067(号段不变,语义重写) |
| 062 `ALTER TABLE reference_file_refs ADD COLUMN owner_module_key` | 062 `CREATE TABLE reference_file_refs(...)` 展平表 |
| 066 `ADD COLUMN tasks.task_priority` | 066 `ALTER TABLE tasks ADD CHECK priority IN (...) + ADD INDEX (priority, created_at)`,不加新列 |
| (无) | **068(新增)**`CREATE TABLE task_customization_orders`(若定制 §3.1.1 选方案 Y) |
| backfill Phase A `SELECT status FROM tasks` | `SELECT task_status FROM tasks` |
| backfill Phase B 基于 flow_stage | 基于 asset_type × task_type × customization_required |
| backfill Phase D 基于 is_urgent | 无(priority 已默认 'normal',仅需校验枚举 + 可选升级) |
| Phase C UPDATE reference_file_refs.owner_module_key | Phase C 解析两个 JSON 列 → INSERT 展平表 |

验收门槛加强:

- 必须提供 Docker MySQL(`docker compose up mysql`)+ 最小种子 dump(至少 100 行 tasks / 50 行 task_details / 10 行 customization_jobs / 30 行 asset_storage_refs 覆盖 3 种 owner_type)作为验收输入;Codex 不得以"无 DSN"为由跳过验收。

### 6.6 写 `docs/iterations/V1_R1_5_DDL_ALIGNMENT.md`(本文最终版)

本文件签字后,转为"已生效"版本,保留在 iterations 目录作为历史凭证。

### 6.7 更新 `prompts/V1_ROADMAP.md`

- 在 R1 和 R2 之间插入 R1.5 行:"**已执行 · DDL 对齐 + R2 回退 + 4 份权威文档 v1.1 + R2 v2 prompt**"。
- R2 状态回到 "**待 Codex 执行(v2 prompt)**"。
- R1 状态补注:"发现 P0 字段幻觉,已通过 R1.5 修复,R1 不需重做(OpenAPI 层无字段问题)。"

---

## 7. 风险与未覆盖

- **种子 dump 依赖**:R2 v2 的 10w 任务性能门槛仍需真实数据;本地 / WSL 起 Docker MySQL 是最低要求,需用户提供或确认用空种子 + 造数脚本。
- **服务层双写**:R3 handler 必须对两个 JSON 列和 `reference_file_refs` 展平表双写(读仅展平表);R3 prompt 需明确该约束,否则 R2 backfill 结果 R3 可能遗忘维护。
- **task_customization_orders 方案**:等本文 §4 签字确认"方案 Y"(推荐)后,§6.3 / §6.5 才定稿;若选 X / Z,§6.3 和 §6.5 068 需改写。
- **`task_assets.asset_type` 历史数据分布**:036 迁移已批量把 `draft/revised/final/outsource_return` 统一为 `delivery`,`original` 统一为 `source`。backfill Phase B 可依赖此真值;但如果生产库尚未跑过 036,需在 R2 前置检查。

---

## 8. 签字门槛

| 签字项 | 人 | 状态 |
|---|---|---|
| §3 幻觉清单认可 | 产品 / 架构 | **已签字**(2026-04-17) |
| §4 5 项决策认可 | 产品 / 架构 | **已签字**(B/A/priority/flow/方案 Y 全部 2026-04-17 签字) |
| §6.1 主文档修改点 | 产品 / 后端 | **已签字**(2026-04-17,V1_MODULE_ARCHITECTURE.md v1.1 落盘) |
| §6.2 资产文档重写点 | 产品 / 后端 | **已签字**(2026-04-17,V1_ASSET_OWNERSHIP.md v1.1 落盘) |
| §6.3 定制文档修改点 | 产品 / 后端 | **已签字**(2026-04-17,V1_CUSTOMIZATION_WORKFLOW.md v1.1 落盘) |
| §6.4 IA 文档核查 | 产品 / 后端 | **已签字**(2026-04-17,V1_INFORMATION_ARCHITECTURE.md v1.1 · 审计通过,无结构变更) |
| §6.5 R2 v2 prompt 范围 | 后端 / 研发 | **已签字**(2026-04-17,prompts/V1_R2_DATA_LAYER.md v2 落盘) |

全部签字项 2026-04-17 一并盖章,R1.5 进入 v1.0 生效状态,R2 v2 解锁可开工。

---

## 9. 变更记录

| 版本 | 日期 | 变更 | 签字 |
|---|---|---|---|
| Draft v1 | 2026-04-17 | 初稿;R2 回退已执行;4 份文档修改提案 + R2 v2 prompt 范围 | — |
| v1.0 | 2026-04-17 | §6 全部落盘:主文档 v1.1 / 资产文档 v1.1 / 定制文档 v1.1 / IA 文档 v1.1(审计) / R2 prompt v2 · 全部签字项盖章 | **已签字**(产品 + 后端 + 架构 2026-04-17) |
