# ERP 与业务字典

> Revision: V1.3-A2 i_id-first task/ERP/search integration (2026-04-27)
> Source: docs/api/openapi.yaml (post V1.3-A2)

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

ERP 商品、分类、仓库、同步、类目、成本规则与兼容商品目录。

## Family 约定

- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 本文件覆盖 `28` 个 `/v1` path；同一路径多 method 合并在同一节。

## GET /v1/erp/products

### 简介
支持方法: GET。

- `GET`: Frontend-ready MAIN facade over Bridge(8081). **Primary chain**: Bridge `remote`/`hybrid` calls Jushuitan OpenWeb POST `ERP_REMOTE_SKU_QUERY_PATH` (live ERP). Local `products` is **not** the search source of truth; hybrid may fall back to local `products` only on transient upstream failure (5xx/timeout), never on OpenWeb business errors or auth misconfiguration. Server logs: `erp_bridge_product_search` (`remote_ok` vs `fallback_local_products`). **8080 `products` table**: sync replica / mapping cache / business read-model (ERP_SYNC_SOURCE_MODE=jst via OpenWeb provider), not the picker primary index. Current search boundary: - keyword / q: supported; implemented as the current bridge's contains-match style lookup rather than semantic/global search - sku_code: supported as an exact filter after MAIN normalization - category_id / category_name: supported as exact filters after MAIN category validation and local refinement - browse: supported - sorting: not supported - search suggestion / association: not supported - global search: not supported Contract guarantees for result-page integration: - product_id is the stable lookup key emitted by this list and accepted by /v1/erp/products/{id} - sku_code, category_name, category_code, product_short_name, image_url may be empty strings when upstream data is absent - empty image_url means frontend should render its own placeholder image - when category_id / category_name is invalid or does not exist, MAIN returns an empty list instead of falling back to browse - when a result is chosen, frontend should pass the returned object into task product_selection.erp_product; backend will cache/bind it to a local products.id

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `q` | query | string | 否 | Keyword search string. This is the primary original-product selection entry and should be preferred over bridge category-only browsing. |
| `keyword` | query | string | 否 | Additive compatibility alias for `q`. When `q` is empty, backend normalizes this into the effective keyword query. |
| `sku_code` | query | string | 否 | Exact sku-code filter. MAIN may reuse a code-like upstream id as sku_code when bridge does not emit a dedicated field. |
| `category_id` | query | string | 否 | Exact category filter using the normalized category id from /v1/erp/categories. |
| `category_name` | query | string | 否 | Exact category-name filter. MAIN validates the value against /v1/erp/categories; unknown values return an empty list. |
| `category` | query | string | 否 | Compatibility alias for `category_name`. |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "product_id": "...",
      "sku_id": "...",
      "i_id": "...",
      "sku_code": "..."
    }
  ],
  "pagination": {
    "page": 123,
    "page_size": 123,
    "total": 123
  },
  "normalized_filters": {
    "q": "string",
    "keyword": "string",
    "sku_code": "string",
    "category_id": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<ERPBridgeProduct> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |
| `normalized_filters` | ERPBridgeProductSearchFilter | 否 | Normalized `/v1/erp/products` query echo. Supported filters are keyword contains-match, exact `sku_code`, exact `category_id` / `category_name`, and browse. Global search, suggestion, and sorting are not supported. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | ERP Bridge upstream product path not found |
| 500 | 见 `error.code` | 见 `deny_code` | ERP Bridge unavailable or upstream returned a non-200 response. Error details may now include timeout/retry-hint diagnostics for internal observability. |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/erp/products \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/erp/iids

### 简介
支持方法: GET。

- `GET`: Returns distinct Jushuitan product style/family `i_id` values from the local ERP sync replica for frontend selection. New task creation should use this endpoint to select `i_id`; `category_code` is backend-owned compatibility metadata and should not be a required frontend input.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `q` | query | string | 否 | Optional fuzzy search against i_id, category, category_name, product_name, or sku_code. |
| `keyword` | query | string | 否 | Compatibility alias for `q`. |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "i_id": "...",
      "label": "...",
      "category": "...",
      "category_name": "..."
    }
  ],
  "pagination": {
    "page": 123,
    "page_size": 123,
    "total": 123
  },
  "normalized_filters": {
    "q": "string",
    "page": 123,
    "page_size": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<ERPIIDOption> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |
| `normalized_filters` | ERPIIDListFilter | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | UNAUTHENTICATED | - | 未登录、token 缺失或 token 过期。 |
| 403 | PERMISSION_DENIED | 见接口返回 | 角色、组织范围、字段级授权或流程状态不允许。 |
| 404 | NOT_FOUND | - | 资源不存在或当前用户不可见。 |
| 409 | CONFLICT | 见接口返回 | 状态竞态、重复操作或版本冲突。 |
| 422 | VALIDATION_ERROR | - | 请求参数或业务字段校验失败。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/erp/iids \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/erp/products/{id}

### 简介
支持方法: GET。

- `GET`: Returns one normalized ERP product detail through the MAIN facade. Detail lookup behavior: - accepts the product_id emitted by GET /v1/erp/products - first tries direct bridge detail lookup - when direct bridge detail returns 404, MAIN performs a compatible search-based fallback so list -> detail -> task binding remains stable for result-page integration - returns 404 only when MAIN cannot resolve the list-emitted lookup key anymore

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | string | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "product_id": "string",
    "sku_id": "string",
    "i_id": "string",
    "sku_code": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ERPBridgeProduct | 否 | Normalized ERP product contract returned by the MAIN facade. `product_id` is the stable lookup key emitted by `GET /v1/erp/products` and accepted by `GET /v1/erp/products/{id}`. Empty strings mean the backend could not obtain that field from bridge/category normalization; frontend should render its own placeholder for empty `image_url`. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | ERP Bridge product not found |
| 500 | 见 `error.code` | 见 `deny_code` | ERP Bridge unavailable or upstream returned a non-200 response |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/erp/products/<id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/erp/categories

### 简介
支持方法: GET。

- `GET`: Normalized categories for **exact** product filters on /v1/erp/products. Source is local configurable mapping layer (categories table; current 31 rows are sample data, not production category center). Business classification primary semantic = style code (i_id); JST category field is ERP raw field. Do NOT use jst_inventory or any large sync table scan. For **global** category pickers (rules, filters, dropdowns), prefer **GET /v1/categories** or **GET /v1/categories/search** as the primary API. See docs/TRUTH_SOURCE_ALIGNMENT.md.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "category_id": "...",
      "category_name": "...",
      "parent_id": "...",
      "level": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<ERPBridgeCategory> | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 500 | 见 `error.code` | 见 `deny_code` | ERP Bridge unavailable or upstream returned a non-200 response |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/erp/categories \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/erp/warehouses

### 简介
支持方法: GET。

- `GET`: Returns Bridge-aligned warehouse dimension catalog for MAIN filtering and warehouse-scoped ERP writes.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "name": "...",
      "wms_co_id": "...",
      "warehouse_type": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<ERPWarehouse> | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 500 | 见 `error.code` | 见 `deny_code` | ERP Bridge unavailable |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/erp/warehouses \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/erp/users

### 简介
支持方法: GET。

- `GET`: Bridge-side query for JST getcompanyusers. Maps to `/open/webapi/userapi/company/getcompanyusers`. Pre-wiring only: does NOT change main auth/permission logic. Admin/ERP role required.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `current_page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |
| `page_action` | query | integer | 否 | - |
| `enabled` | query | boolean | 否 | - |
| `version` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "current_page": "string",
    "page_size": "string",
    "count": "string",
    "pages": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | JSTUserListResponse | 否 | ERP-style JST getcompanyusers response. Source: domain.JSTUserListResponse. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 500 | 见 `error.code` | 见 `deny_code` | Bridge or upstream unavailable |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/erp/users \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/erp/products/upsert

### 简介
支持方法: POST。

- `POST`: Bridge-side write endpoint that accepts ordinary product-profile upsert payloads from MAIN. MAIN calls this internally via `ERP_BRIDGE_BASE_URL` when `filed_at` is set on an existing-product task. In the current v0.4 architecture, Bridge uses a `localERPBridgeClient` that persists directly to the shared MySQL database rather than calling an external ERP API. As of 2026-03-17, Bridge also supports a config-driven remote mode: `ERP_REMOTE_MODE=local|remote|hybrid`. - `local`: keep local DB writeback - `remote`: upsert is sent to configured external ERP API - `hybrid`: remote first, optional local fallback by env switch OpenWeb official mapping used by Bridge remote mode: - `POST /v1/erp/products/upsert` -> `/open/webapi/itemapi/itemsku/itemskubatchupload` - `POST /v1/erp/products/style/update` -> `/open/webapi/itemapi/itemskuim/itemupload` - OpenWeb signature parameters are: `app_key`, `access_token`, `timestamp`, `charset`, `version`, `biz` (`sign` excluded from signing), sorted by key and signed as: `md5(app_secret + key1value1key2value2...)` (lowercase hex) Combined-SKU upload endpoint `/open/item/combinesku/upload` is intentionally not auto-selected in current Bridge contract because this payload currently has no explicit "combined SKU" discriminator; Bridge keeps ordinary-SKU mapping deterministic to avoid accidental wrong-endpoint writes. Current live production mode is `hybrid`; do not assume every write endpoint has remote success without upstream response evidence for that endpoint. This route was added in ITERATION_070 to fix a 404 gap that prevented all MAIN -> Bridge filing calls from succeeding. Requires session authentication (Bearer token forwarded from MAIN).

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `product_id` | string | 否 | - |
| `sku_id` | string | 否 | - |
| `i_id` | string | 否 | - |
| `sku_code` | string | 否 | 鍘嗗彶鍏煎瀛楁 |
| `name` | string | 否 | - |
| `product_name` | string | 否 | 鍘嗗彶鍏煎瀛楁 |
| `short_name` | string | 否 | - |
| `category_id` | string | 否 | - |
| `category_code` | string | 否 | - |
| `category_name` | string | 否 | - |
| `product_short_name` | string | 否 | 鍘嗗彶鍏煎瀛楁 |
| `image_url` | string | 否 | - |
| `price` | number | 否 | - |
| `s_price` | number | 否 | - |
| `remark` | string | 否 | - |
| `cost_price` | number | 否 | - |
| `supplier_name` | string | 否 | - |
| `wms_co_id` | string | 否 | - |
| `brand` | string | 否 | - |
| `vc_name` | string | 否 | - |
| `item_type` | string | 否 | - |
| `pic` | string | 否 | - |
| `pic_big` | string | 否 | - |
| `sku_pic` | string | 否 | - |
| `properties_value` | string | 否 | - |
| `weight` | number | 否 | - |
| `l` | number | 否 | - |
| `w` | number | 否 | - |
| `h` | number | 否 | - |
| `enabled` | boolean | 否 | - |
| `supplier_sku_id` | string | 否 | - |
| `supplier_i_id` | string | 否 | - |
| `market_price` | number | 否 | - |
| `other_price_1` | number | 否 | - |
| `other_price_2` | number | 否 | - |
| `other_price_3` | number | 否 | - |
| `other_price_4` | number | 否 | - |
| `other_price_5` | number | 否 | - |
| `other_1` | string | 否 | - |
| `other_2` | string | 否 | - |
| `other_3` | string | 否 | - |
| `other_4` | string | 否 | - |
| `other_5` | string | 否 | - |
| `stock_disabled` | boolean | 否 | - |
| `operation` | string | 否 | product_profile_upsert or original_product_update |
| `sku_immutable` | boolean | 否 | Whether SKU binding stays immutable for original-product updates. |
| `auto_generate_short_name` | boolean | 否 | - |
| `short_name_template_key` | string | 否 | - |
| `currency` | string | 否 | - |
| `source` | string | 否 | Origin context, e.g. task_business_info_filing |
| `product` | object | 否 | Full ERP product snapshot from product_selection.erp_product binding |
| `task_context` | object | 否 | - |
| `business_info` | object | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "product_id": "string",
    "sku_id": "string",
    "i_id": "string",
    "sku_code": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ERPProductUpsertResult | 否 | Normalized Bridge write result. Tolerant schema because upstream bridge may expose several response envelopes. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid payload or missing required fields (sku_id required) |
| 401 | 见 `error.code` | 见 `deny_code` | Authentication required |
| 500 | 见 `error.code` | 见 `deny_code` | Internal error during upsert |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/erp/products/upsert \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/erp/products/style/update

### 简介
支持方法: POST。

- `POST`: Bridge-side style update endpoint for i_id-centered changes, especially for original-product flows where SKU remains unchanged but style-level data (picture/style fields) needs update. OpenWeb remote mapping: `POST /v1/erp/products/style/update` -> `/open/webapi/itemapi/itemskuim/itemupload`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `sku_id` | string | 否 | - |
| `i_id` | string | 是 | - |
| `name` | string | 否 | - |
| `short_name` | string | 否 | - |
| `category_name` | string | 否 | - |
| `pic` | string | 否 | - |
| `pic_big` | string | 否 | - |
| `sku_pic` | string | 否 | - |
| `properties_value` | string | 否 | - |
| `brand` | string | 否 | - |
| `vc_name` | string | 否 | - |
| `supplier_i_id` | string | 否 | - |
| `enabled` | boolean | 否 | - |
| `operation` | string | 否 | - |
| `source` | string | 否 | - |
| `auto_generate_short_name` | boolean | 否 | - |
| `short_name_template_key` | string | 否 | - |
| `task_context` | object | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "sku_id": "string",
    "i_id": "string",
    "name": "string",
    "short_name": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ERPItemStyleUpdateResult | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid payload (`i_id` is required) |
| 401 | 见 `error.code` | 见 `deny_code` | Authentication required |
| 500 | 见 `error.code` | 见 `deny_code` | Internal error during style update |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/erp/products/style/update \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/erp/sync-logs

### 简介
支持方法: GET。

- `GET`: Lists Bridge-side ERP write sync logs used for mutation observability. This route is served by 8081 Bridge and primarily backed by integration call logs for: - `erp_bridge_product_upsert` - `erp_bridge_item_style_update` - `erp_bridge_product_shelve_batch` - `erp_bridge_product_unshelve_batch` - `erp_bridge_inventory_virtual_qty`

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |
| `status` | query | string | 否 | - |
| `connector` | query | string | 否 | - |
| `operation` | query | string | 否 | - |
| `resource_type` | query | string | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "sync_log_id": "...",
      "connector": "...",
      "operation": "...",
      "status": "..."
    }
  ],
  "pagination": {
    "page": 123,
    "page_size": 123,
    "total": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<ERPSyncLog> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 500 | 见 `error.code` | 见 `deny_code` | Internal error while reading sync logs |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/erp/sync-logs \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/erp/sync-logs/{id}

### 简介
支持方法: GET。

- `GET`: Get ERP Bridge sync log detail

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | string | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "sync_log_id": "string",
    "connector": "string",
    "operation": "string",
    "status": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ERPSyncLog | 否 | Bridge-side ERP write sync log record. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Sync log not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/erp/sync-logs/<id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/erp/products/shelve/batch

### 简介
支持方法: POST。

- `POST`: Bridge-owned ERP batch shelve mutation boundary for MAIN integration. OpenWeb official mapping in remote/hybrid mode: `POST /v1/erp/products/shelve/batch` -> `/open/webapi/wmsapi/openshelve/skubatchshelve`. Current live boundary (`v0.4`, ITERATION_077): remote request is sent, but upstream currently rejects with business response `code=100, msg=涓婃灦浠撲綅涓嶈兘涓虹┖` for tested payloads; in `hybrid` mode Bridge falls back to local write path.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `items` | array<ERPProductBatchMutationItem> | 是 | - |
| `reason` | string | 否 | - |
| `source` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "action": "string",
    "total": 123,
    "accepted": 123,
    "rejected": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ERPProductBatchMutationResult | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid batch payload |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/erp/products/shelve/batch \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/erp/products/unshelve/batch

### 简介
支持方法: POST。

- `POST`: Bridge-owned ERP batch unshelve mutation boundary for MAIN integration. OpenWeb official mapping in remote/hybrid mode: `POST /v1/erp/products/unshelve/batch` -> `/open/webapi/wmsapi/openoffshelve/skubatchoffshelve`. Current live boundary (`v0.4`, ITERATION_077): remote request is sent, but upstream currently rejects with business response `code=100, msg=鎸囧畾绠变笉瀛樺湪` for tested payloads; in `hybrid` mode Bridge falls back to local write path.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `items` | array<ERPProductBatchMutationItem> | 是 | - |
| `reason` | string | 否 | - |
| `source` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "action": "string",
    "total": 123,
    "accepted": 123,
    "rejected": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ERPProductBatchMutationResult | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid batch payload |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/erp/products/unshelve/batch \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/erp/inventory/virtual-qty

### 简介
支持方法: POST。

- `POST`: Bridge-owned ERP virtual inventory mutation boundary for MAIN integration. OpenWeb official mapping in remote/hybrid mode: `POST /v1/erp/inventory/virtual-qty` -> `/open/webapi/itemapi/iteminventory/batchupdatewmsvirtualqtys`. Current live boundary (`v0.4`, ITERATION_077): remote request is sent, but upstream raw body in tested cases returns `code=0, msg=鏈幏鍙栧埌鏈夋晥鐨勪紶鍏ユ暟鎹? data=null`. Bridge classifies this as business rejection and in `hybrid` mode falls back to local write path.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `items` | array<ERPVirtualInventoryUpdateItem> | 是 | - |
| `reason` | string | 否 | - |
| `source` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "total": 123,
    "accepted": 123,
    "rejected": 123,
    "sync_log_id": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ERPVirtualInventoryUpdateResult | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid payload |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/erp/inventory/virtual-qty \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/products/search

### 简介
支持方法: GET。

- `GET`: Compatibility-only local-cache product search. New integrations must use `GET /v1/erp/products`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `keyword` | query | string | 否 | - |
| `category` | query | string | 否 | Legacy fuzzy filter against `products.category`. Kept for compatibility. |
| `category_id` | query | integer | 否 | Resolve the selected category to its local first-level `search_entry_code`, then apply active local category-to-ERP mappings. |
| `category_code` | query | string | 否 | Resolve the selected category to its local first-level `search_entry_code`, then apply active local category-to-ERP mappings. |
| `search_entry_code` | query | string | 否 | Explicit first-level search-entry code for local ERP positioning. When omitted but `category_id` or `category_code` is provided, it is derived from the category center. |
| `mapping_match` | query | enum(primary/all) | 否 | Controls which active local mapping rules are consumed. `primary` is the default when mapped search is used; `all` allows non-primary active rules too. |
| `secondary_key` | query | string | 否 | Optional lightweight reserved second-level mapping key. Must be paired with `secondary_value`. |
| `secondary_value` | query | string | 否 | Optional lightweight reserved second-level mapping value. Must be paired with `secondary_key`. |
| `tertiary_key` | query | string | 否 | Optional lightweight reserved third-level mapping key. Must be paired with `tertiary_value`. |
| `tertiary_value` | query | string | 否 | Optional lightweight reserved third-level mapping value. Must be paired with `tertiary_key`. |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {}
  ],
  "pagination": {
    "page": 123,
    "page_size": 123,
    "total": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<ProductSearchResult> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid mapped-search query |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/products/search \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/products/sync/status

### 简介
支持方法: GET。

- `GET`: Internal placeholder endpoint for ERP sync visibility. This remains a MAIN-owned sync and runtime control surface and does not imply Bridge ownership. `source_mode=stub` means MAIN reads a local stub JSON source. The response reports the runtime-resolved stub path plus existence state so noop-vs-source-path diagnosis is explicit. Not ready for frontend. Uses current debug-header role enforcement.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "placeholder": true,
    "scheduler_enabled": true,
    "interval_seconds": 123,
    "source_mode": "stub"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ERPSyncStatus | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | UNAUTHENTICATED | - | 未登录、token 缺失或 token 过期。 |
| 403 | PERMISSION_DENIED | 见接口返回 | 角色、组织范围、字段级授权或流程状态不允许。 |
| 404 | NOT_FOUND | - | 资源不存在或当前用户不可见。 |
| 409 | CONFLICT | 见接口返回 | 状态竞态、重复操作或版本冲突。 |
| 422 | VALIDATION_ERROR | - | 请求参数或业务字段校验失败。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/products/sync/status \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/products/sync/run

### 简介
支持方法: POST。

- `POST`: Internal placeholder endpoint that synchronously reads the stub ERP source and upserts products. This remains a MAIN-owned sync and runtime control surface and does not imply Bridge ownership. Not ready for frontend. Uses current debug-header role enforcement.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "trigger_mode": "manual",
    "source_mode": "stub",
    "status": "success",
    "total_received": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ERPSyncRunResult | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | UNAUTHENTICATED | - | 未登录、token 缺失或 token 过期。 |
| 403 | PERMISSION_DENIED | 见接口返回 | 角色、组织范围、字段级授权或流程状态不允许。 |
| 404 | NOT_FOUND | - | 资源不存在或当前用户不可见。 |
| 409 | CONFLICT | 见接口返回 | 状态竞态、重复操作或版本冲突。 |
| 422 | VALIDATION_ERROR | - | 请求参数或业务字段校验失败。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/products/sync/run \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/products/{id}

### 简介
支持方法: GET。

- `GET`: Compatibility-only local-cache product detail. New integrations must use `GET /v1/erp/products/{id}`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "erp_product_id": "string",
    "sku_code": "string",
    "product_name": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | Product | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Product not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/products/<id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/categories

### 简介
支持方法: GET, POST。

- `GET`: Returns paginated category-center skeleton entries. These records now explicitly model first-level ERP search-entry semantics through `search_entry_code` and `is_search_entry`. Coded-style total category codes such as `HBJ` remain valid first-level search-entry categories.
- `POST`: Creates one category-center skeleton entry. Top-level categories must keep `search_entry_code == category_code` and `is_search_entry=true`, making the total category code the explicit first-level ERP search entry.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `keyword` | query | string | 否 | - |
| `category_type` | query | enum(coded_style/board/paper/print/cloth/material/custom/manual_quote/other) | 否 | - |
| `parent_id` | query | integer | 否 | - |
| `level` | query | integer | 否 | - |
| `is_active` | query | boolean | 否 | - |
| `source` | query | string | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "category_id": "...",
      "category_code": "...",
      "category_name": "...",
      "display_name": "..."
    }
  ],
  "pagination": {
    "page": 123,
    "page_size": 123,
    "total": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<Category> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | UNAUTHENTICATED | - | 未登录、token 缺失或 token 过期。 |
| 403 | PERMISSION_DENIED | 见接口返回 | 角色、组织范围、字段级授权或流程状态不允许。 |
| 404 | NOT_FOUND | - | 资源不存在或当前用户不可见。 |
| 409 | CONFLICT | 见接口返回 | 状态竞态、重复操作或版本冲突。 |
| 422 | VALIDATION_ERROR | - | 请求参数或业务字段校验失败。 |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/categories \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `category_code` | string | 是 | - |
| `category_name` | string | 是 | - |
| `display_name` | string | 否 | - |
| `parent_id` | integer | 否 | - |
| `level` | integer | 否 | - |
| `search_entry_code` | string | 否 | Explicit first-level ERP search-entry code. Top-level categories must keep this equal to `category_code`. |
| `is_search_entry` | boolean | 否 | - |
| `category_type` | enum(coded_style/board/paper/print/cloth/material/custom/manual_quote/other) | 是 | - |
| `is_active` | boolean | 否 | - |
| `sort_order` | integer | 否 | - |
| `source` | string | 否 | - |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "category_id": 123,
    "category_code": "string",
    "category_name": "string",
    "display_name": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | Category | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid category payload |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/categories \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/categories/search

### 简介
支持方法: GET。

- `GET`: Lightweight selection/search endpoint for category-center skeleton entries. Suitable for task filing and for locating first-level ERP search-entry categories before later second/third-level refinement is introduced.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `keyword` | query | string | 否 | - |
| `category_type` | query | enum(coded_style/board/paper/print/cloth/material/custom/manual_quote/other) | 否 | - |
| `is_active` | query | boolean | 否 | - |
| `limit` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "category_id": "...",
      "category_code": "...",
      "category_name": "...",
      "display_name": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<Category> | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | UNAUTHENTICATED | - | 未登录、token 缺失或 token 过期。 |
| 403 | PERMISSION_DENIED | 见接口返回 | 角色、组织范围、字段级授权或流程状态不允许。 |
| 404 | NOT_FOUND | - | 资源不存在或当前用户不可见。 |
| 409 | CONFLICT | 见接口返回 | 状态竞态、重复操作或版本冲突。 |
| 422 | VALIDATION_ERROR | - | 请求参数或业务字段校验失败。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/categories/search \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/categories/{id}

### 简介
支持方法: GET, PATCH。

- `GET`: Get category by ID
- `PATCH`: Updates one category-center skeleton entry while preserving explicit first-level ERP search-entry semantics.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- `PATCH` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "category_id": 123,
    "category_code": "string",
    "category_name": "string",
    "display_name": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | Category | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Category not found |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/categories/<id> \
  -H "Authorization: Bearer $TOKEN"
```

#### PATCH 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `category_code` | string | 否 | - |
| `category_name` | string | 否 | - |
| `display_name` | string | 否 | - |
| `parent_id` | integer | 否 | - |
| `level` | integer | 否 | - |
| `search_entry_code` | string | 否 | - |
| `is_search_entry` | boolean | 否 | - |
| `category_type` | enum(coded_style/board/paper/print/cloth/material/custom/manual_quote/other) | 否 | - |
| `is_active` | boolean | 否 | - |
| `sort_order` | integer | 否 | - |
| `source` | string | 否 | - |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "category_id": 123,
    "category_code": "string",
    "category_name": "string",
    "display_name": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | Category | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid category payload |
| 404 | 见 `error.code` | 见 `deny_code` | Category not found |

##### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/categories/<id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/category-mappings

### 简介
支持方法: GET, POST。

- `GET`: Returns paginated category-to-ERP positioning skeleton records. These mappings define how one category or first-level search entry should later be used to narrow ERP product lookup, without performing real ERP lookup in this phase.
- `POST`: Creates one category-to-ERP positioning skeleton record. `search_entry_code` represents the explicit first-level ERP lookup entry, while secondary and tertiary condition fields are reserved for later refinement.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `keyword` | query | string | 否 | - |
| `category_id` | query | integer | 否 | - |
| `category_code` | query | string | 否 | - |
| `search_entry_code` | query | string | 否 | - |
| `erp_match_type` | query | enum(category_code/product_family/sku_prefix/keyword/external_id) | 否 | - |
| `is_active` | query | boolean | 否 | - |
| `is_primary` | query | boolean | 否 | - |
| `source` | query | string | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "mapping_id": "...",
      "category_id": "...",
      "category_code": "...",
      "search_entry_code": "..."
    }
  ],
  "pagination": {
    "page": 123,
    "page_size": 123,
    "total": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<CategoryERPMapping> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | UNAUTHENTICATED | - | 未登录、token 缺失或 token 过期。 |
| 403 | PERMISSION_DENIED | 见接口返回 | 角色、组织范围、字段级授权或流程状态不允许。 |
| 404 | NOT_FOUND | - | 资源不存在或当前用户不可见。 |
| 409 | CONFLICT | 见接口返回 | 状态竞态、重复操作或版本冲突。 |
| 422 | VALIDATION_ERROR | - | 请求参数或业务字段校验失败。 |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/category-mappings \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `category_id` | integer | 否 | - |
| `category_code` | string | 否 | - |
| `search_entry_code` | string | 否 | - |
| `erp_match_type` | enum(category_code/product_family/sku_prefix/keyword/external_id) | 是 | - |
| `erp_match_value` | string | 是 | - |
| `secondary_condition_key` | string | 否 | - |
| `secondary_condition_value` | string | 否 | - |
| `tertiary_condition_key` | string | 否 | - |
| `tertiary_condition_value` | string | 否 | - |
| `is_primary` | boolean | 否 | - |
| `is_active` | boolean | 否 | - |
| `priority` | integer | 否 | - |
| `source` | string | 否 | - |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "mapping_id": 123,
    "category_id": 123,
    "category_code": "string",
    "search_entry_code": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CategoryERPMapping | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid category mapping payload |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/category-mappings \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/category-mappings/search

### 简介
支持方法: GET。

- `GET`: Lightweight selection/search endpoint for category-to-ERP positioning skeleton records. Suitable for admin lookup and for later task/product-search orchestration.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `keyword` | query | string | 否 | - |
| `category_code` | query | string | 否 | - |
| `search_entry_code` | query | string | 否 | - |
| `erp_match_type` | query | enum(category_code/product_family/sku_prefix/keyword/external_id) | 否 | - |
| `is_active` | query | boolean | 否 | - |
| `limit` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "mapping_id": "...",
      "category_id": "...",
      "category_code": "...",
      "search_entry_code": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<CategoryERPMapping> | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | UNAUTHENTICATED | - | 未登录、token 缺失或 token 过期。 |
| 403 | PERMISSION_DENIED | 见接口返回 | 角色、组织范围、字段级授权或流程状态不允许。 |
| 404 | NOT_FOUND | - | 资源不存在或当前用户不可见。 |
| 409 | CONFLICT | 见接口返回 | 状态竞态、重复操作或版本冲突。 |
| 422 | VALIDATION_ERROR | - | 请求参数或业务字段校验失败。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/category-mappings/search \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/category-mappings/{id}

### 简介
支持方法: GET, PATCH。

- `GET`: Get category-to-ERP mapping by ID
- `PATCH`: Updates one category-to-ERP positioning skeleton record while preserving the explicit first-level ERP search-entry contract.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- `PATCH` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "mapping_id": 123,
    "category_id": 123,
    "category_code": "string",
    "search_entry_code": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CategoryERPMapping | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Category-to-ERP mapping not found |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/category-mappings/<id> \
  -H "Authorization: Bearer $TOKEN"
```

#### PATCH 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `category_id` | integer | 否 | - |
| `category_code` | string | 否 | - |
| `search_entry_code` | string | 否 | - |
| `erp_match_type` | enum(category_code/product_family/sku_prefix/keyword/external_id) | 否 | - |
| `erp_match_value` | string | 否 | - |
| `secondary_condition_key` | string | 否 | - |
| `secondary_condition_value` | string | 否 | - |
| `tertiary_condition_key` | string | 否 | - |
| `tertiary_condition_value` | string | 否 | - |
| `is_primary` | boolean | 否 | - |
| `is_active` | boolean | 否 | - |
| `priority` | integer | 否 | - |
| `source` | string | 否 | - |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "mapping_id": 123,
    "category_id": 123,
    "category_code": "string",
    "search_entry_code": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CategoryERPMapping | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid category mapping payload |
| 404 | 见 `error.code` | 见 `deny_code` | Category-to-ERP mapping not found |

##### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/category-mappings/<id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/cost-rules

### 简介
支持方法: GET, POST。

- `GET`: Returns paginated governed cost-rule entries with effective-window metadata and lineage fields such as `version_chain_summary`, `previous_version`, `next_version`, and `supersession_depth`. This module is a governed rule skeleton, not a full formula engine or approval platform.
- `POST`: Creates one governed cost-rule skeleton entry. Use `manual_quote` for categories or formulas that are not machine-calculable yet. `rule_version` / `supersedes_rule_id` / `governance_note` harden governance only; they do not introduce a separate rule-version subsystem or approval flow.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `category_id` | query | integer | 否 | - |
| `category_code` | query | string | 否 | - |
| `product_family` | query | string | 否 | - |
| `rule_type` | query | enum(fixed_unit_price/area_threshold_surcharge/minimum_billable_area/size_based_formula/manual_quote/special_process_surcharge) | 否 | - |
| `is_active` | query | boolean | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "rule_id": "...",
      "rule_name": "...",
      "rule_version": "...",
      "category_id": "..."
    }
  ],
  "pagination": {
    "page": 123,
    "page_size": 123,
    "total": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<CostRule> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | UNAUTHENTICATED | - | 未登录、token 缺失或 token 过期。 |
| 403 | PERMISSION_DENIED | 见接口返回 | 角色、组织范围、字段级授权或流程状态不允许。 |
| 404 | NOT_FOUND | - | 资源不存在或当前用户不可见。 |
| 409 | CONFLICT | 见接口返回 | 状态竞态、重复操作或版本冲突。 |
| 422 | VALIDATION_ERROR | - | 请求参数或业务字段校验失败。 |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/cost-rules \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `rule_name` | string | 是 | - |
| `rule_version` | integer | 否 | Optional explicit version for this row. If omitted while `supersedes_rule_id` is provided, the backend auto-increments from the referenced rule. |
| `category_id` | integer | 否 | - |
| `category_code` | string | 否 | - |
| `product_family` | string | 否 | - |
| `rule_type` | enum(fixed_unit_price/area_threshold_surcharge/minimum_billable_area/size_based_formula/manual_quote/special_process_surcharge) | 是 | - |
| `base_price` | number | 否 | - |
| `tax_multiplier` | number | 否 | - |
| `min_area` | number | 否 | - |
| `area_threshold` | number | 否 | - |
| `surcharge_amount` | number | 否 | - |
| `special_process_keyword` | string | 否 | - |
| `special_process_price` | number | 否 | - |
| `formula_expression` | string | 否 | - |
| `priority` | integer | 否 | - |
| `is_active` | boolean | 否 | - |
| `effective_from` | string | 否 | - |
| `effective_to` | string | 否 | - |
| `supersedes_rule_id` | integer | 否 | - |
| `governance_note` | string | 否 | Lightweight governance note only. This is not an approval workflow comment. |
| `source` | string | 否 | - |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "rule_id": 123,
    "rule_name": "string",
    "rule_version": 123,
    "category_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CostRule | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid cost rule payload |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/cost-rules \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/cost-rules/{id}

### 简介
支持方法: GET, PATCH。

- `GET`: Returns one governed cost-rule row with lineage read-model fields such as `previous_version`, `next_version`, `version_chain_summary`, and `supersession_depth`. This is configuration governance, not a separate approval workflow or formula platform.
- `PATCH`: Updates one governed cost-rule skeleton entry while keeping the model extensible for later rule growth, later deactivation, and later successor-version planning. This remains configuration governance, not a formula DSL editor or approval system.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- `PATCH` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "rule_id": 123,
    "rule_name": "string",
    "rule_version": 123,
    "category_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CostRule | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Cost rule not found |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/cost-rules/<id> \
  -H "Authorization: Bearer $TOKEN"
```

#### PATCH 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `rule_name` | string | 否 | - |
| `rule_version` | integer | 否 | - |
| `category_id` | integer | 否 | - |
| `category_code` | string | 否 | - |
| `product_family` | string | 否 | - |
| `rule_type` | enum(fixed_unit_price/area_threshold_surcharge/minimum_billable_area/size_based_formula/manual_quote/special_process_surcharge) | 否 | - |
| `base_price` | number | 否 | - |
| `tax_multiplier` | number | 否 | - |
| `min_area` | number | 否 | - |
| `area_threshold` | number | 否 | - |
| `surcharge_amount` | number | 否 | - |
| `special_process_keyword` | string | 否 | - |
| `special_process_price` | number | 否 | - |
| `formula_expression` | string | 否 | - |
| `priority` | integer | 否 | - |
| `is_active` | boolean | 否 | - |
| `effective_from` | string | 否 | - |
| `effective_to` | string | 否 | - |
| `supersedes_rule_id` | integer | 否 | - |
| `governance_note` | string | 否 | - |
| `source` | string | 否 | - |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "rule_id": 123,
    "rule_name": "string",
    "rule_version": 123,
    "category_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CostRule | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid cost rule payload |
| 404 | 见 `error.code` | 见 `deny_code` | Cost rule not found |

##### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/cost-rules/<id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/cost-rules/{id}/history

### 简介
支持方法: GET。

- `GET`: Returns the selected governed rule row together with its lineage chain so consumers can distinguish the current row from predecessor and successor versions. This endpoint is read-only and does not introduce approval flow, formula authoring, or finance integration.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "rule": {
      "rule_id": "...",
      "rule_name": "...",
      "rule_version": "...",
      "category_id": "..."
    },
    "version_chain": [
      "..."
    ],
    "current_rule": {}
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CostRuleHistoryReadModel | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Cost rule not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/cost-rules/<id>/history \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/cost-rules/preview

### 简介
支持方法: POST。

- `POST`: Minimal preview contract for the governed cost-rule skeleton. Fixed price, area-threshold surcharge, minimum billable area, and special-process surcharge can return estimates. `manual_quote`, missing required size or area inputs, and unsupported size-based formulas return `requires_manual_review=true`. The response includes `matched_rule_id`, `matched_rule_version`, `rule_source`, and `governance_status`. `PATCH /v1/tasks/{id}/business-info` reuses the same pricing semantics for persisted task-side prefill snapshots.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `category_id` | integer | 否 | - |
| `category_code` | string | 否 | - |
| `width` | number | 否 | - |
| `height` | number | 否 | - |
| `area` | number | 否 | - |
| `quantity` | integer | 否 | - |
| `process` | string | 否 | - |
| `notes` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "matched_rule": {},
    "matched_rule_id": 123,
    "matched_rule_version": 123,
    "applied_rules": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CostRulePreviewResponse | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid preview request |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/cost-rules/preview \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/erp/products/by-code

### 简介
支持方法: GET。

- `GET`: Source: V1_CUSTOMIZATION_WORKFLOW §3.1.2. Backend-side wrapper of upstream `/open/combine/sku/query` (ERP 8081). Failure policy (§3.1.2): upstream 5xx / timeout → 502; upstream 'not found' → 404. Task creation callers MUST block on 404/502 and surface a retry or correction UX.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `code` | query | string | 是 | ERP product code (matches `tasks.erp_product_code`) |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "code": "string",
    "product_name": "string",
    "snapshot": {}
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ERPProductSnapshot | 否 | Source: V1_CUSTOMIZATION_WORKFLOW §3.1.2. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 404 | 见 `error.code` | 见 `deny_code` | ERP upstream reports code not found |
| 502 | 见 `error.code` | 见 `deny_code` | ERP upstream 5xx / timeout |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/erp/products/by-code \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。
- `/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。
- `/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。
