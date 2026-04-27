# 商品主数据与分类真相源统一说明

> **权威说明**：本文档为项目内商品主数据、分类语义、products、jst_inventory、8081 OpenWeb、original_product_development 的**唯一真相源口径**。后续开发、联调、验收、文档编写均须以此为准。  
> 来源：架构法医 / 真相源总审计报告（2026-03-18）。

---

## 1. 为什么以前会混乱

1. **真相源从未一次性钉死**：商品搜索、分类、本地副本、原始同步层各自为政。
2. **本地兜底掩盖主链未打通**：fallback、EnsureLocalProduct、products upsert 让系统“看起来能跑”，掩盖了主链未统一的事实。
3. **分类语义长期混用**：聚水潭 `category`、款式编码（`i_id`）、本地 `categories` 表被当作同一概念。
4. **categories 31 行被误当真实分类中心**：实为 migration + seed 的开发样例，非生产真实分类库。
5. **文档领先于真实系统**：常把“能力已具备”写成“线上已生效”。

---

## 2. 统一后的唯一口径

### 2.1 商品搜索真相源
- **主链**：8081 OpenWeb（`ERP_REMOTE_SKU_QUERY_PATH`），在 `ERP_REMOTE_MODE=remote` 或 `hybrid` 且 `ERP_REMOTE_AUTH_MODE=openweb` 时生效。
- **兜底**：local 模式或 hybrid 下可恢复上游故障时，回退本地 `products`。
- **禁止**：将本地 `products` 当作商品搜索唯一真相源。

### 2.2 分类真相源
- **业务分类主语义**：**款式编码（`i_id`）**，即聚水潭款式/分类维度。
- **聚水潭 `category` 字段**：ERP 原始字段，**不等于**业务分类真相源。
- **本地 `categories` 表**：可配置映射骨架，当前 31 行为开发样例（`phase_020_sample`），**不是**生产真实分类中心；用于 cost_rule、category_erp_mappings、ERP 搜索定位。

### 2.3 products 角色
- **是**：JST/OpenWeb 同步副本、ERP 映射缓存、任务/成本/商品维护的本地承接表。
- **非**：商品搜索唯一真相源、原品创建唯一硬前置（允许 defer + ERP snapshot）。

### 2.4 jst_inventory 角色
- **是**：8082 JST 同步驻留原始表、证据层、对账层、品类索引抽取来源（本仓库无表实现，见部署侧）。
- **非**：前台商品搜索主表、原品开发创建主表。

### 2.5 original_product_development 语义
- **允许**：`defer_local_product_binding=true` 时 `product_id` 为空，基于 ERP snapshot 创建任务。
- **非 defer**：仍可能 `EnsureLocalProduct` 绑定到本地 `products`。
- **禁止**：再写成“原品开发必须先有本地 product 才能创建”。

### 2.6 状态分层（必须严格区分）
| 层级 | 含义 |
|------|------|
| **Design Target** | 目标架构/正确方向 |
| **Code Implemented** | 代码已实现 |
| **Server Verified** | 已在服务器上验收 |
| **Live Effective** | 线上已生效 |

未经服务器实证，不得将 Code Implemented 写成 Live Effective。

---

## 3. 必须作废的旧结论

| 旧结论 | 正确表述 |
|--------|----------|
| categories 31 行是系统已有真实全局品类中心 | categories 是开发样例 + 可配置映射骨架，非生产真实分类库 |
| 聚水潭 category = 业务分类 | 业务分类主语义 = 款式编码（i_id）；聚水潭 category 为 ERP 原始字段 |
| products 为选品搜索唯一真相源 | products 为副本；选品主链为 8081 OpenWeb |
| jst_inventory 可当前端品类 API / 前台搜索主表 | 禁止；jst_inventory 为同步驻留层，不当前台搜索 |
| 原品开发必须先有本地 product 才能创建 | defer 时允许 product_id 为空，基于 ERP snapshot 创建 |
| OpenWeb 主链已打通 | **v0.8 已实证**：8081 remote_ok、fallback_used=false；8080 JST sync 已驱动 products 从 20 增至 7470 |

---

## 4. 对后续开发/联调/验收的约束

1. **文档**：凡涉及分类/品类，必须区分业务分类（i_id）、聚水潭 category、本地 categories。
2. **前端**：品类下拉优先 `GET /v1/categories`；禁止用库存/jst 全表接口当品类树；defer 路径下理解 detail 中 `product.status=erp_snapshot`。
3. **验收**：按 `docs/archive/ERP_REAL_LINK_VERIFICATION.md` 完成 A/B/C/D 四项，并贴日志证据，方可宣称 OpenWeb 主链已生效。**v0.8 已通过 A/B 组验收**。
4. **配置**：`ERP_SYNC_SOURCE_MODE=jst` 需显式配置；`ERP_REMOTE_MODE` 与 `ERP_REMOTE_AUTH_MODE` 决定 8081 主链行为。

---

## 5. 长期保护（可观测性约束，不得删除）

主链打通后，必须保留以下可观测性，否则会回到「到底有没有真打通」的不确定状态。

### 5.1 Bridge 8081 远程/回退
- `ERP_REMOTE_MODE=hybrid` 或 `remote` 时，**必须保留**：
  - `erp_bridge_product_search` / `erp_bridge_product_by_id`（含 `result`、`fallback_used`、`fallback_reason`）
  - `remote_erp_openweb_request_started` / `remote_erp_openweb_request_completed`

### 5.2 JST 同步 8080
- `ERP_SYNC_SOURCE_MODE=jst` 时，**必须保留**：
  - `erp_sync_run_start` / `erp_sync_run_finish`（含 `provider`、`source_mode`、`status`、`total_received`、`total_upserted`、`sample_sku`）
  - 分页日志：`page`、`upsert_count`
  - 限流/重试：`rate_limit`、`retry`、`code=199` 等错误码的日志
