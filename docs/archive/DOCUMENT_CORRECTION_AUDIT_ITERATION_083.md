# 文档纠偏 + 真相源口径统一 — 专项输出（ITERATION_083）

**执行日期**：2026-03-18  
**参考**：`docs/ARCHITECTURE_FORENSIC_AUDIT_REPORT.md`、`docs/TRUTH_SOURCE_ALIGNMENT.md`

---

## 1. 旧口径审计清单

| 文件 | 原表述 | 问题类型 | 新表述 |
|------|--------|----------|--------|
| CURRENT_STATE.md | 品类维度数据来自 **categories 分类中心** | categories 被误写为真实分类中心 | 数据来自**本地可配置映射层**（当前含 31 行样例，非生产真实分类库）；业务分类主语义 = 款式编码（i_id） |
| CURRENT_STATE.md | 选品搜索唯一真相、创建任务硬前置（products 非职责） | 表述略模糊 | 选品搜索唯一真相源、原品创建唯一硬前置（允许 defer + ERP snapshot） |
| CURRENT_STATE.md | 前端实时搜索、任务创建强绑定（jst_inventory 非职责） | 表述略模糊 | 前台商品搜索主表、原品开发创建主表 |
| MODEL_HANDOVER.md | 全局品类用 `/v1/categories` | 未明确 categories 为样例 | 全局品类用 `/v1/categories`（本地可配置映射层，当前含样例数据）；业务分类主语义 = 款式编码（i_id） |
| 设计流转自动化管理系统_V7.0_重构版_技术实施规格.md | 品类维度主链 GET /v1/categories | 未明确 categories 为样例、未区分 i_id | 业务分类主语义 = 款式编码（i_id）；主链 GET /v1/categories 来自本地可配置映射层（当前 31 行为样例） |
| 设计流转自动化管理系统_V7.0_重构版_技术实施规格.md | 8082 jst_inventory 禁止：前台实时搜索入口；任务创建强依赖 | 表述略模糊 | 禁止：前台商品搜索主表；原品开发创建主表 |
| docs/FRONTEND_ALIGNMENT_v0.5.md | 全局品类来自分类中心映射 | 未明确 categories 为样例 | 来自本地可配置映射层（当前含样例数据）；业务分类主语义 = 款式编码（i_id） |
| docs/FRONTEND_ALIGNMENT_v0.5.md | 选品主链为 8081 经 OpenWeb | 未区分 live/code | 主链为 8081 经 OpenWeb；**未经服务器实证不得写成“已完全切主链”** |
| docs/ERP_REAL_LINK_VERIFICATION.md | 分类中心表，体量可控 | 未明确 categories 为样例 | 本地可配置映射层，当前含 31 行样例，非生产真实分类库 |
| docs/api/openapi.yaml | Source is category-center skeleton (categories table) | 未明确 categories 为样例、未区分 i_id | Source is local configurable mapping layer (categories table; current 31 rows are sample data); business classification primary semantic = i_id |
| docs/iterations/ITERATION_082.md | 分类中心表 | 未明确 categories 为样例 | 本地可配置映射层，当前含样例数据 |
| MODEL_v0.4_memory.md | （无显式错误，但缺少纠偏） | 其他真相源错位 | 新增纠偏说明，引用 TRUTH_SOURCE_ALIGNMENT |

---

## 2. 本轮修改文件列表

| 文件名 | 修改摘要 |
|--------|----------|
| `CURRENT_STATE.md` | 四层职责表：品类维度改为“业务分类主语义 = 款式编码（i_id）”；categories 改为“本地可配置映射层（当前含 31 行样例）”；products/jst_inventory 非职责表述收紧 |
| `MODEL_HANDOVER.md` | 架构一句话：补充 local/fallback 仅兜底、products 非搜索唯一真相源、i_id 为业务分类主语义、categories 为本地映射层；引用 TRUTH_SOURCE_ALIGNMENT |
| `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md` | 四层职责表：品类维度补充 i_id、categories 为样例；新增 2.1.2 商品主数据与分类真相源统一说明；4.4 ERP 字段补充 category 与 i_id 区分 |
| `docs/FRONTEND_ALIGNMENT_v0.5.md` | 选品：补充“未经服务器实证不得写成已完全切主链”；品类：补充 i_id、categories 为样例；原品开发：补充 defer 路径、前端不必把 product_id 当前提 |
| `docs/ERP_REAL_LINK_VERIFICATION.md` | D 节：补充业务分类主语义 = i_id；categories 改为本地可配置映射层；聚水潭 category 为 ERP 原始字段 |
| `docs/api/openapi.yaml` | `/v1/erp/categories` 描述：补充 categories 为样例、i_id 为业务分类主语义、引用 TRUTH_SOURCE_ALIGNMENT |
| `docs/iterations/ITERATION_082.md` | 全局品类：补充 i_id、categories 为本地可配置映射层 |
| `docs/iterations/ITERATION_083.md` | 新增：本轮执行记录 |
| `MODEL_v0.4_memory.md` | 新增纠偏说明：categories、products、jst_inventory、OpenWeb 主链、Code ≠ Live |
| `ITERATION_INDEX.md` | 新增 ITERATION_083 行 |
| `docs/TRUTH_SOURCE_ALIGNMENT.md` | 新增：商品主数据与分类真相源统一说明专章 |
| `docs/DOCUMENT_CORRECTION_AUDIT_ITERATION_083.md` | 新增：本专项输出文档 |

---

## 3. 统一后的核心口径

### 商品搜索真相源
- 主链：8081 OpenWeb（remote/hybrid 下）
- 兜底：local/fallback
- 禁止：将 products 当作唯一真相源；未经服务器实证不得写成“已完全切主链”

### 分类真相源
- 业务分类主语义：**款式编码（i_id）**
- 聚水潭 `category`：ERP 原始字段，不等于业务分类
- 本地 `categories`：可配置映射层，当前 31 行为样例，非生产真实分类库

### products 角色
- 是：副本、映射缓存、业务承接表
- 非：商品搜索唯一真相源、原品创建唯一硬前置（允许 defer + ERP snapshot）

### jst_inventory 角色
- 是：同步驻留原始层、证据、对账、品类索引抽取来源
- 非：前台商品搜索主表、原品开发创建主表

### original_product_development 语义
- 允许：defer 时 product_id 为空，基于 ERP snapshot 先创建
- 非 defer：仍可能 EnsureLocalProduct 绑定 products
- 禁止：再写成“必须先有本地 product 才能创建”

### live/code/design 状态分层
| 层级 | 含义 |
|------|------|
| Design Target | 目标架构 |
| Code Implemented | 代码已实现 |
| Server Verified | 服务器已验收 |
| Live Effective | 线上已生效 |

---

## 4. 明确作废的旧结论

1. categories 31 行是系统已有真实全局品类中心  
2. 聚水潭 category = 业务分类  
3. products 为选品搜索唯一真相源  
4. jst_inventory 可当前端品类 API / 前台搜索主表  
5. 原品开发必须先有本地 product 才能创建  
6. OpenWeb 主链已打通（未经验收时）  
7. 代码支持 = live 已生效  
8. fallback 能跑 = 真实 OpenWeb 主链已闭环  

---

## 5. 最终文档状态

- **已完成纠偏**：CURRENT_STATE、MODEL_HANDOVER、技术实施规格、FRONTEND_ALIGNMENT、ERP_REAL_LINK_VERIFICATION、openapi、ITERATION_082、MODEL_v0.4、ITERATION_INDEX
- **新增**：TRUTH_SOURCE_ALIGNMENT、ITERATION_083、DOCUMENT_CORRECTION_AUDIT_ITERATION_083
- **已足够作为当前项目统一口径**：是；后续开发、联调、验收、文档编写均须以 `docs/TRUTH_SOURCE_ALIGNMENT.md` 为准。
