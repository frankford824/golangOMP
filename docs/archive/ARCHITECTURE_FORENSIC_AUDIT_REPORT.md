# 架构法医 / 真相源总审计报告

**审计日期**：2026-03-18  
**审计范围**：商品主数据、分类语义、ERP/JST 真实链路  
**审计模式**：真相源总审计（非补丁式修复）

---

## 1. 结论先行（不超过 10 句话）

1. **改了这么多版问题还在的根本原因**：真相源从未一次性钉死，商品搜索、分类、本地副本、原始同步层各自为政，且本地兜底（fallback、EnsureLocalProduct、products upsert、defer 最终仍落本地）让系统“看起来能跑”，掩盖了主链未统一的事实。

2. **`categories` 31 行是假中心**：来自 migration 010 + category_seed.json，全部为 `phase_020_sample` / `phase_022_sample` 开发样例数据，不是生产真实分类库；历史文档多次把它当成“系统已有真实全局品类中心”，这是误导根因之一。

3. **款式编码（i_id）才是业务真正使用的“分类”语义**：聚水潭 `i_id` = 款式编码 = 分类/款式维度；工程中 `erp_bridge_jst_sku`、`erp_sync_jst_provider`、`ERPProduct.IID`、`erp_short_name_template` 均已按此语义实现；聚水潭 `category` 字段与 `i_id` 不是同一概念，但项目长期混用。

4. **下一轮必须统一**：商品搜索真相源 = 8081 OpenWeb（remote/hybrid 下）；分类真相源 = 款式编码（i_id）为主、categories 为可配置本地映射层；products = 副本/缓存/承接表；jst_inventory = 同步驻留/证据层，不当前台搜索源。

5. **live 与文档长期错位**：代码支持 remote/hybrid/openweb，但历史上 live 多次停留在 local；`ERP_SYNC_SOURCE_MODE` 默认 stub，JST 同步需显式配置；文档常把“能力已具备”写成“线上已生效”。

---

## 2. 术语权威表

| 术语 | 业务含义 | 工程落点字段 | 当前是否权威 | 是否曾被误用 |
|------|----------|--------------|--------------|--------------|
| **商品搜索** | 原品选品时按关键词/SKU/品类检索 ERP 商品 | 8081 `GET /v1/erp/products`，Bridge SearchProducts | 是（主链应为 OpenWeb） | 曾误以 products 为唯一真相源 |
| **原品开发绑定** | 原品任务创建时选定已有 ERP 商品并绑定到任务 | `product_selection`、`EnsureLocalProduct`、`tasks.product_id` | 是 | 曾误以 product_id 硬前置 |
| **分类** | 业务上的商品归类维度 | 多义：见冲突矩阵 | 否（语义漂移） | 是 |
| **品类** | 与“分类”混用，常指一级总分类 | `categories`、`category_code`、`category_name` | 部分 | 是 |
| **款式编码** | 聚水潭款式/分类维度，业务真正使用的分类语义 | `i_id`（JST/OpenWeb）、`ERPProduct.IID`、`spec_json.i_id` | **是** | 曾被与 category 混用 |
| **category_code** | 本地 categories 表编码 | `categories.category_code`、`search_entry_code` | 是（本地映射层） | 曾被当作 ERP 主分类 |
| **category_name** | 分类显示名 | `categories.category_name`、`products.category`、ERP `CategoryName` | 部分 | 是 |
| **i_id** | 聚水潭款式编码（分类/款式维度） | `domain.ERPProduct.IID`、`spec_json.i_id`、OpenWeb 响应 | **是** | 曾被与 sku_id 混淆 |
| **sku_id** | 聚水潭商品唯一编码 | `ERPProduct.SKUID`、`spec_json.sku_id` | 是 | 否 |
| **sku_code** | 商品 SKU 编码（可来自 ERP 或本地生成） | `products.sku_code`、`tasks.sku_code` | 是 | 否 |
| **erp_product_id** | ERP 侧商品主键 | `products.erp_product_id`、`ERPProduct.ProductID` | 是 | 否 |
| **products** | 8080 本地商品表 | `products` 表 | 是（副本角色） | 曾误为选品搜索唯一真相源 |
| **jst_inventory** | JST 同步驻留原始表 | **本仓库无表实现**，见部署侧 | 是（职责已定） | 曾误当前台搜索源 |
| **categories** | 本地可配置分类中心表 | `categories` 表，31 行样例 | 是（本地映射层） | **曾误为真实分类中心** |
| **OpenWeb** | 聚水潭开放 API | `ERP_REMOTE_BASE_URL`、`ERP_REMOTE_SKU_QUERY_PATH` | 是（主链） | 否 |
| **sync 副本** | 从 JST/OpenWeb 同步到 products 的数据 | `products`、`erp_sync_runs`、`JSTOpenWebProductProvider` | 是 | 否 |
| **snapshot 创建** | 任务创建时保存的 ERP 商品快照 | `product_selection.erp_product`、`task_details` | 是 | 否 |
| **local binding** | 任务绑定本地 products.id | `tasks.product_id`、`EnsureLocalProduct` | 是 | 否 |
| **defer_local_product_binding** | 允许 product_id=null，快照落 task_details | `product_selection.defer_local_product_binding`、`syntheticProductFromDeferredERPSelection` | 是 | 否 |

---

## 3. 分类语义冲突矩阵

| 概念 | 实际字段 | 出现模块 | 是否当前 live 使用 | 是否权威 | 是否误用 |
|------|---------|---------|-------------------|---------|---------|
| 聚水潭 category | `CategoryName`、`category`（products 表） | erp_bridge_jst_sku、erp_sync_jst_provider、products | 是（来自 JST 响应） | 否 | 是（非业务主分类） |
| 款式编码 | `i_id` | domain.erp_bridge、erp_bridge_jst_sku、erp_sync_jst_provider、erp_short_name_template、style/update | 是 | **是** | 否 |
| 本地 categories | `categories.category_code`、`category_name`、`search_entry_code` | migration 010/012、category_seed、categoryRepo、localERPBridgeClient.ListCategories | 是 | 是（本地映射） | 是（曾当真实分类中心） |
| search_entry_code | `categories.search_entry_code`、`category_erp_mappings` | migration 012、product search、category_erp_mappings | 是 | 是 | 否 |
| matched_category_code | `product_selection`、search result | openapi、task read model | 是 | 是 | 否 |
| products.category | `products.category` | products 表、ProductRepo | 是 | 否（来自 JST CategoryName） | 是（与 i_id 混用） |
| task_details.category_code | `task_details.category_code` | task repo、business-info | 是 | 是 | 否 |

**结论**：业务真正使用的“分类”语义是 **款式编码（i_id）**；聚水潭 `category` 字段是 ERP 原始字段，不等同于业务分类；本地 `categories` 是可配置映射层，31 行是样例，非生产真实分类库。

---

## 4. `categories` 表法医结论

### 4.1 来源

- **Migration**：`db/migrations/010_v7_category_cost_rule_skeleton.sql` 创建表并 INSERT 31 行
- **Seed**：`config/category_seed.json` 与 migration 内嵌 INSERT 一致
- **来源标注**：全部 `source='phase_020_sample'` 或 `phase_022_sample`
- **类型**：开发阶段人工提供/样例性质数据

### 4.2 误导原因

1. 文档多次将 `GET /v1/categories` 描述为“分类中心”“全局品类主链”，未明确 31 行为样例
2. `category_erp_mappings` 从 categories 表 seed，形成“categories 是分类中心”的错觉
3. Bridge `ListCategories` 在 local/hybrid 下均读 `categories` 表，hybrid 下 remote 返回空、必走 local，强化了“categories 即品类”的认知

### 4.3 真实定位

- **当前**：可配置的一级分类映射骨架，用于 cost_rule、category_erp_mappings、ERP 搜索定位
- **非**：生产真实全局品类中心、JST 分类主数据、jst_inventory 的 GROUP BY 来源

### 4.4 必须作废的旧结论

| 旧结论 | 正确表述 |
|--------|----------|
| categories 是系统已有真实全局品类中心 | categories 是开发样例 + 可配置映射骨架，非生产真实分类库 |
| 前端品类下拉应来自 categories | 前端品类主链应为款式编码（i_id）映射或业务配置，categories 仅作本地映射层 |
| 31 行 categories 即完整品类树 | 31 行为 phase_020_sample 样例，生产需另行维护或对接真实分类源 |

---

## 5. 商品主数据四条链法医分析

### 链 1：8081 商品查询链

| 环节 | 实现 |
|------|------|
| Handler | `ERPBridgeHandler.SearchProducts`、`GetProductByID` |
| Service | `erpBridgeService.SearchProducts`、`GetProductByID` |
| Client | `localERPBridgeClient` / `remoteERPBridgeClient` / `hybridERPBridgeClient` |
| Remote | `remoteERPBridgeClient` 调用 OpenWeb `ERP_REMOTE_SKU_QUERY_PATH` |
| Hybrid | 优先 remote，仅网络超时/5xx 等可恢复错误时 fallback 本地 products |
| Fallback | `localERPBridgeClient.SearchProducts` 读 `productRepo.Search` |
| 最终数据源 | remote/hybrid + openweb 配置正确时 = OpenWeb；local 或 fallback 时 = products |

**问题**：live 历史上多次为 local，hybrid 下 `ListCategories` 始终走 local（remote 返回空），品类过滤依赖本地 categories。

### 链 2：8080 `products` 链

| 环节 | 实现 |
|------|------|
| 写入 | `ERPSyncService`（StubERPProductProvider 或 JSTOpenWebProductProvider）、`EnsureLocalProduct`、Bridge upsert（local 模式） |
| 读取 | `ProductRepo.Search`、`GetByID`、`GetByERPProductID` |
| 依赖 | 任务绑定 `tasks.product_id`、成本规则、商品维护 |
| 是否副本 | 是（JST/OpenWeb 同步副本、ERP 映射缓存） |
| 是否绑定锚点 | 是（任务创建时 EnsureLocalProduct 写入并绑定） |

**问题**：`ERP_SYNC_SOURCE_MODE` 默认 stub，JST 同步需显式 `jst`；products 体量小，因 stub 或 JST 分页拉取未全量覆盖。

### 链 3：8082 `jst_inventory` 链

| 环节 | 实现 |
|------|------|
| 写入 | 8082 sync 服务（本仓库无实现） |
| 读取 | 本仓库无直接读取 |
| 真正使用 | 文档定义为证据、对账、品类索引抽取来源 |
| 定位 | 同步驻留原始表，**不当前台搜索入口** |

**问题**：**本仓库无 jst_inventory 表实现**，职责仅在文档与架构中固定；部署侧可能有独立实现，与 8080 products 未形成稳定副本关系。

### 链 4：original_product_development 创建链

| 环节 | 实现 |
|------|------|
| Request | `product_selection`、`defer_local_product_binding`、`erp_product` |
| product_selection | `task_product_selection.go`、handler 归一化 |
| EnsureLocalProduct | `erp_bridge_service.EnsureLocalProduct`，绑定键优先级：product_id -> sku_id -> sku_code |
| defer_local_product_binding | `product_id=null` 时跳过 EnsureLocalProduct，快照落 task_details |
| create tx | `task_service.CreateTask`、`repo/mysql/task.go` |
| detail/read model | `task_detail_service.syntheticProductFromDeferredERPSelection` 合成 `product`（status=erp_snapshot） |
| 最终是否落回 products | defer 时可不落；非 defer 时 EnsureLocalProduct 必写 products |

**问题**：defer 时详情仍返回 product，但为合成对象；非 defer 时最终仍依赖 products 有对应行，否则 EnsureLocalProduct 会 upsert 新行。

### 为什么四条链长期没有收成统一真相源

1. **商品搜索**：设计上 8081 OpenWeb 为主链，但 live 长期 local，或 hybrid 下 fallback 频繁，主链未真正生效
2. **products**：设计为副本，但 sync 默认 stub，JST 需额外配置，且与 jst_inventory 无同步关系
3. **jst_inventory**：本仓库无实现，无法参与统一架构
4. **分类**：款式编码（i_id）与 categories、聚水潭 category 多源并存，未明确唯一真相源

---

## 6. 历史假收口清单

| 结论 | 为什么当时会被认为成立 | 实际为什么不成立 | 应该改成什么表述 |
|------|------------------------|------------------|------------------|
| 选品搜索主链在 8081 OpenWeb | 代码支持 remote/hybrid + OpenWeb | live 历史上多次为 local；hybrid 下需正确配置且 fallback 条件收紧 | 设计上主链为 OpenWeb；live 需验证 `erp_bridge_product_search` 日志 `result=remote_ok` |
| products 是 JST/OpenWeb 同步副本 | 有 JSTOpenWebProductProvider | `ERP_SYNC_SOURCE_MODE` 默认 stub，live 多为 stub 或 noop | 代码支持 JST 同步；live 需 `ERP_SYNC_SOURCE_MODE=jst` 且凭证正确 |
| 全局品类用 GET /v1/categories | 有 categories 表与 API | categories 31 行为样例，非生产真实分类库 | 品类主链应为款式编码映射；categories 为可配置本地映射层 |
| 外部 ERP 已接通 | Bridge 有 remote client、hybrid 模式 | ITERATION_075 明确 live `ERP_REMOTE_MODE=local`，凭证空 | 能力已具备；live 接通需配置并复验 |
| jst_inventory 是同步驻留层 | 文档定义 | 本仓库无表实现 | 职责在文档中固定；实现见部署侧 |
| 原品创建不要求本地 products.id | defer_local_product_binding 支持 | 非 defer 时仍须 EnsureLocalProduct，最终落 products | defer 时可不要求；非 defer 时需 products 有行或能 upsert |

---

## 7. 为什么改了这么多版问题还在（正式根因报告）

### A. 真相源没有一次性统一

- **商品搜索真相源**：设计为 OpenWeb，但未强制 live 配置与验收，长期以 products 为实际数据源
- **分类真相源**：款式编码（i_id）与 categories、聚水潭 category 并存，未明确“业务分类 = i_id”
- **本地副本真相源**：products 角色清晰，但 sync 来源（stub vs JST）未统一，体量长期偏小
- **原始同步层**：jst_inventory 本仓库无实现，与 products 无稳定同步关系

### B. 本地兜底掩盖了主链未打通

- **local fallback**：Bridge local 模式、hybrid fallback 让搜索“总能返回结果”
- **EnsureLocalProduct**：选品后自动 upsert products，任务能创建
- **products upsert**：Bridge 写入、sync 写入，products 有数据
- **defer 最终仍落本地**：defer 时详情合成 product，但 filing 等仍可能依赖 products

系统“看起来能跑”，但主链（OpenWeb 搜索、JST 同步、款式编码分类）未真正贯通。

### C. 代码、配置、部署、数据各自为政

- **代码**：支持 remote/hybrid、JST sync、defer
- **配置**：`ERP_REMOTE_MODE`、`ERP_SYNC_SOURCE_MODE` 等散落环境变量，live 常未切
- **部署**：cutover 不自动恢复 8082；bridge.env 与 main.env 分离
- **数据**：products 体量小；categories 为样例；jst_inventory 无实现

### D. 分类语义长期被错误简化

- **聚水潭 category ≠ 业务分类**：category 为 ERP 原始字段，业务主分类为款式编码（i_id）
- **款式编码 ≠ categories 样例表**：HBJ/HBZ 等 coded_style 在 categories 中，但真实款式来自 JST
- **前端“全局品类” ≠ 原始 ERP category**：前端需要的是业务可配置分类，不是 ERP 原始字段

项目长期混用，导致筛选、规则、任务创建时分类语义不一致。

### E. 文档领先于真实系统

- **能力存在 ≠ 已生效**：remote client、JST provider 已实现，但 live 未切
- **设计正确 ≠ live 已落地**：四层职责文档正确，但线上仍以 local/stub 为主
- 历史常把“应该如此”写成“已经如此”，导致联调、排查时预期与事实不符。

---

## 8. 唯一正确的收口方向（唯一方案）

### 商品搜索真相源

**8081 OpenWeb**（`ERP_REMOTE_SKU_QUERY_PATH`）。  
`ERP_REMOTE_MODE=remote` 或 `hybrid` 且 `ERP_REMOTE_AUTH_MODE=openweb` 时，主链为 OpenWeb；hybrid 仅在可恢复上游故障时回退本地 products。

### 分类真相源

**款式编码（i_id）** 为业务主分类语义；**categories** 为可配置本地映射层（含 search_entry_code、category_erp_mappings），用于 ERP 搜索定位与成本规则，非生产真实分类库。  
禁止将聚水潭 `category` 字段默认当作业务分类。

### `products` 最终角色

**副本 / 映射缓存 / 业务承接表**。  
职责：JST/OpenWeb 同步副本、ERP 映射缓存、任务/成本/商品维护的本地承接。  
非职责：选品搜索唯一真相源、原品创建硬前置（允许 defer + ERP snapshot）。

### `jst_inventory` 最终角色

**同步驻留原始表 / 证据层 / 对账层 / 品类索引抽取来源**。  
非职责：前台实时搜索入口、任务创建强依赖。  
本仓库无表实现，职责在文档与架构中固定；实现见部署侧。

### original_product_development 最终语义

- **允许 ERP snapshot 先创建**：`defer_local_product_binding=true` 时，`product_id` 可为 null，快照落 `task_details`
- **本地 product 绑定时机**：非 defer 时在 create tx 中 `EnsureLocalProduct`；defer 时可不绑定
- **异步绑定**：当前为同步；如需异步，需单独设计
- **detail/read model**：`product_id` 非空时读 products；为空且 defer 时合成 `product`（status=erp_snapshot）

---

## 9. 唯一主张架构图（文字版）

```
用户搜索商品
  -> 8081 Bridge GET /v1/erp/products
  -> remote/hybrid: OpenWeb SKU 查询（主链）
  -> local 或 fallback: 读 8080 products

用户查全局品类
  -> GET /v1/categories（主，来自 categories 表，可配置映射）
  -> GET /v1/erp/categories（辅，Bridge 转发，hybrid 下仍来自 categories）
  -> 禁止对 jst_inventory 全表 GROUP BY

原品开发创建
  -> 选品: 8081 搜索 -> product_selection
  -> 非 defer: EnsureLocalProduct -> products upsert -> tasks.product_id
  -> defer: product_id=null，快照落 task_details，详情合成 product

本地 products 生成/更新
  -> ERPSyncWorker: stub 或 JSTOpenWebProductProvider -> products upsert
  -> EnsureLocalProduct: 选品后 upsert
  -> Bridge upsert（local 模式）: 写入 products

jst_inventory 参与但不干扰主链
  -> 8082 sync 写入（部署侧）
  -> 证据、对账、品类索引抽取
  -> 不当前台搜索、不参与任务创建主链
```

---

## 10. 必须纠正/废弃的旧文档与旧口径

| 文件/位置 | 旧结论/口径 | 纠正后 |
|-----------|-------------|--------|
| 任意文档 | categories 31 行为真实分类中心 | 样例数据，非生产真实分类库 |
| 任意文档 | 聚水潭 category = 业务分类 | 业务分类 = 款式编码（i_id）；category 为 ERP 原始字段 |
| 任意文档 | 选品搜索主链已切 OpenWeb | 需按 ERP_REAL_LINK_VERIFICATION 验收后确认 |
| 任意文档 | products 为选品搜索唯一真相源 | products 为副本；选品主链为 8081 OpenWeb |
| 任意文档 | jst_inventory 可当前端品类 API | 禁止；品类用 /v1/categories |
| CURRENT_STATE / MODEL_HANDOVER | 品类数据来自 categories 分类中心 | 品类主链为款式编码；categories 为可配置映射层 |

---

## 11. 下一轮最小改造清单

1. **文档收口**：在 `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`、`CURRENT_STATE.md`、`MODEL_HANDOVER.md` 中明确：categories 31 行为样例；业务分类 = 款式编码（i_id）。
2. **配置校验**：8081 启动时，remote/hybrid 强制校验 `ERP_REMOTE_BASE_URL` + `ERP_REMOTE_AUTH_MODE=openweb`（已部分实现）。
3. **验收执行**：按 `docs/ERP_REAL_LINK_VERIFICATION.md` 完成 A/B/C/D 四项，并贴日志证据。
4. **ERP_SYNC_SOURCE_MODE**：若需 JST 同步，显式配置 `jst` 并验证 products 增长。
5. **废弃口径**：禁止在文档中把 categories 说成“真实分类中心”；禁止把聚水潭 category 默认说成业务分类。
