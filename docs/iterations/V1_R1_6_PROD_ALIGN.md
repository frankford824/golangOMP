# V1 R1.6 · 真生产 DB 对齐报告

> 状态:**v1.0 · 已签字生效**(2026-04-17)
> 目标:R1.5 签字版在"纸面"上对齐 v0.9 仓库 DDL;R1.6 在**真生产 `jst_erp` 数据库**上做一次只读探测,发现两项 R1.5 未覆盖的偏差 → 合入权威文档 v1.2 → 重写 R2 prompt v3 → Codex 开工。
> 范围:不涉及 R2 回退;不改表结构;仅追加决策与文档口径修订。
> 依据:2026-04-17 `ssh jst_ecs` + `tmp/r2_probe_readonly.sh`(只读)探测结果。

---

## 1. 真生产快照(2026-04-17)

### 1.1 规模

| 表 | 行数 | 备注 |
| --- | --- | --- |
| `tasks` | **95** | 比 R1.5 §7 风险项预估的 10w 小 3 个数量级 |
| `task_details` | 95 | 与 tasks 1:1 |
| `task_assets` | 264 | |
| `task_sku_items` | 78 | |
| `asset_storage_refs` | 443 | |
| `customization_jobs` | 9 | 全部为 `original_product_development + customization_required=1` 类(常规定制) |
| `users` | 95 | |

### 1.2 `task_assets.asset_type` 分布

| asset_type | 行数 | 占比 | R1.5 §6.1 规则覆盖 |
| --- | --- | --- | --- |
| `delivery` | 138 | 52.3% | ✓ |
| `source` | 47 | 17.8% | ✓ |
| **`design_thumb`** | 35 | 13.3% | **✗ 未覆盖** |
| **`preview`** | 35 | 13.3% | **✗ 未覆盖** |
| `reference` | 9 | 3.4% | ✓ |

**偏差**:`design_thumb + preview = 70 条(26.5%)`在 R1.5 资产 §6.1 规则里**没有映射**。若 Codex 直接跑,Phase B backfill 对这 70 条资产无法推断 `source_module_key`。

### 1.3 `tasks.priority` 分布

| priority | 行数 | 占比 |
| --- | --- | --- |
| `low` | 56 | 58.9% |
| `high` | 20 | 21.1% |
| `normal` | 19 | 20.0% |

**偏差**:主 §17 v1.1 签字版将 CHECK 约束定为 `normal | urgent | critical`(3 值)。若直接跑 R2 067 迁移,**会拒 76 行(low + high)**,整个 ALTER 失败。

### 1.4 `tasks.task_type` 分布

| task_type | customization_required | 行数 |
| --- | --- | --- |
| `new_product_development` | 0 | 57 |
| `original_product_development` | 0 | 28 |
| `original_product_development` | 1 | 9 |
| `purchase_task` | 0 | 1 |

**观察**:生产至今无 `customer_customization` 任务。v1 新客户定制入口尚未使用,符合预期。

### 1.5 `tasks.task_status` TOP 分布

```
PendingWarehouseReceive    29
InProgress                 20
PendingAssign              14
PendingAuditA               9
PendingProductionTransfer   6
PendingClose                5
PendingCustomizationProduction  5
PendingCustomizationReview  4
Completed                   2
RejectedByAuditB            1
```

全部落入 `domain/enums_v7.go` 24 值枚举 ✓(主 §10.1 v1.1 映射表全部命中)。

### 1.6 字段幻觉实测验证

| 列 | 存在性 | R1.5 §3 预期 |
| --- | --- | --- |
| `task_assets.flow_stage` | **0**(不存在) | ✓ 幻觉已确认 |
| `tasks.is_urgent` | **0**(不存在) | ✓ 幻觉已确认 |
| `tasks.task_priority` | **0**(不存在) | ✓ 幻觉已确认 |
| `tasks.task_status` | 1(存在) | ✓ 真实列 |
| `tasks.priority` | 1(存在) | ✓ 真实列 |

R1.5 §3 的 5 处后端列幻觉 · **全部实测闭环**。

### 1.7 R2 目标表状态

7 张待建表 `task_modules / task_module_events / reference_file_refs / task_drafts / notifications / org_move_requests / task_customization_orders` · **全部不存在**,059~068 号段可用。

### 1.8 参考图 JSON 覆盖

- `task_details.reference_file_refs_json` 非空:**80 / 95** 行(Phase C 展平预计 ~80+ 条)
- `task_sku_items.reference_file_refs_json` 非空:**1 / 78** 行
- `asset_storage_refs` 总行数:443(真权威源)

### 1.9 部署状态

当前生产部署 migration 最大号段:**058_v1_0_org_team_department_scoped_uniqueness.sql**。
059 起对 R2 可用,号段不冲突。

### 1.10 MySQL 版本

`8.0.45-0ubuntu0.24.04.1` · 满足 067 CHECK 约束所需的 ≥ 8.0.16。

---

## 2. 三项 v1.1 → v1.2 决策(2026-04-17 签字)

### 2.1 Y1 · `asset_type` 规则扩展

**决策**:资产 §6.1 `source_module_key` 推断规则新增两条:

```
asset_type = 'design_thumb' → source_module_key = 'design'
asset_type = 'preview'      → source_module_key = 'design'
```

同时补一条兜底规则:

```
asset_type ∉ {reference, source, delivery, design_thumb, preview} → abort backfill + 写 backfill_error 事件
```

**不对历史数据做归一化**(即不做 Y2 的 UPDATE)· 保留现状语义。

### 2.2 Z1 · `tasks.priority` 枚举使用现状 4 值

**决策**:CHECK 约束值域从 `normal | urgent | critical` **改为 `low | normal | high | critical`**(4 值,critical 作为未来加急预留)。
- 067 迁移的 CHECK 约束必须改为 4 值
- 主 §17 v1.1 表格改写
- **不做 UPDATE 归一化**(即不做 Z2 的 `low→normal / high→urgent` 转译)
- 前端任务池排序仍可用 `priority DESC`(按字典序排序天然 low < normal < high < critical)— 但**主文档 §17 必须明示推荐用 `FIELD(priority,'critical','high','normal','low')` 显式权重排序**,避免字典序误解

### 2.3 P1 · 性能门槛降级

**决策**:R2 v3 不再要求 10w 级性能门槛;实测生产规模 95 条任务,Codex **在生产上跑一次实测**,R2 报告记录 `total_duration`(预期 ≤ 1s,若 > 10s 才 fail)。合成数据压测(P2)**不做**。

---

## 3. 对权威文档的影响

| 文档 | v1.1 状态 | v1.2 改动 |
| --- | --- | --- |
| `V1_MODULE_ARCHITECTURE.md` | 已签字 | §17 Q7.5 行改为 4 值 · §18 新增 v1.2 变更行 |
| `V1_ASSET_OWNERSHIP.md` | 已签字 | §6.1 映射表追加两行(design_thumb / preview)+ 兜底规则 · §10 新增 v1.2 |
| `V1_CUSTOMIZATION_WORKFLOW.md` | 已签字 | **无变更**(真生产无 customer_customization 数据,不影响 §3.1.1.1) |
| `V1_INFORMATION_ARCHITECTURE.md` | 已签字 v1.1(审计 only) | **无变更** |

---

## 4. 对 R2 prompt v2 的改动(→ v3)

| 改动点 | v2 | v3 |
| --- | --- | --- |
| 库名 | `workflow_v09` 占位 | **`jst_erp`**(真生产) |
| 执行方式 | 本地 Docker MySQL + `MYSQL_DSN` | **SSH `jst_ecs`** + 服务器本地 `mysql` 客户端(对齐 `deploy/run-org-master-convergence.sh` 模板) |
| 性能门槛 | 10w ≤ 5 min | 95 行 ≤ 10s;合成压测**不做** |
| 067 CHECK | `normal | urgent | critical` | `low | normal | high | critical` |
| Phase B 规则 | 3 个 asset_type 映射 | 5 个(加 design_thumb + preview → design)+ 未知兜底 abort |
| 9 步验收脚本 | 本地 docker 链 | SSH 链:scp → ssh 跑 forward → ssh 跑 backfill → SSH 跑 rollback(dry-run,**不在真生产跑 rollback**)+ 单独的 staging 回滚验证 |
| 备份前置 | 无 | **强制**:每次 forward / backfill 前 `mysqldump --single-transaction` 到 `/root/ecommerce_ai/backups/<ts>_r2/` |

---

## 5. 风险与后续

- **生产真跑 rollback 风险**:rollback 脚本会 DROP TABLE 7 张新表 + DROP CHECK + DROP 索引,不会触及老表数据,但 DROP TABLE 不可逆。v3 默认策略:**生产只验证 forward + backfill,rollback 只在 staging 或 Docker 验证语法正确**;真生产若要回退,必须手工 + 全量 dump 前置。
- **backfill 异常终止的残留清理**:v3 必须提供 `--cleanup-partial` 子命令,清空 `task_modules / task_module_events / reference_file_refs / task_customization_orders` 四张表,便于失败后人工干预重跑。
- **模块化工作台上线 gating**:R2 跑完只是数据层到位;HTTP handler 仍是 R1 的 501。前端切换工作台视角必须等 R3 落地 blueprint engine 后才能走通 · 这意味着 R2 跑完到 R3 上线之间有一段"数据已 ready · 用户看不到"的窗口,**R5 前端不应在 R2 后立刻改**。

---

## 6. 签字门槛

| 签字项 | 人 | 状态 |
| --- | --- | --- |
| §1 真生产快照准确性(只读,无副作用) | 产品 / 架构 | **已签字**(2026-04-17) |
| §2.1 Y1 决策 | 产品 / 架构 | **已签字**(2026-04-17) |
| §2.2 Z1 决策 | 产品 / 架构 | **已签字**(2026-04-17) |
| §2.3 P1 决策 | 产品 / 研发 | **已签字**(2026-04-17) |
| §3 v1.2 文档改动范围 | 产品 / 后端 | **已签字**(2026-04-17) |
| §4 R2 prompt v3 改动范围 | 后端 / 研发 | **已签字**(2026-04-17) |

---

## 7. 变更记录

| 版本 | 日期 | 变更 | 签字 |
| --- | --- | --- | --- |
| v1.0 | 2026-04-17 | 真生产探测 · 三项决策 · v1.2 文档改动范围 · R2 prompt v3 改动范围 | **已签字**(产品 + 后端 + 架构 2026-04-17) |
