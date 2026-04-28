# 搜索

> Revision: V1.2-D-2 residual drift triage (2026-04-26)
> Source: docs/api/openapi.yaml (post V1.2-D-2)

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

全局搜索、资产搜索与设计来源搜索。

## Family 约定

- 搜索接口是只读入口，低权限用户可能拿到空数组而不是错误。
- `GET /v1/search` 的任务搜索已覆盖任务号、任务类型、产品名、SKU、i_id、创建人、所属组、接单人/设计师、创建/截止日期，以及任务关联设计图/参考图文件名。
- 高频输入框应做前端 debounce，避免无意义请求。
- 本文件覆盖 `3` 个 `/v1` path；同一路径多 method 合并在同一节。

## GET /v1/assets/search

### 简介
支持方法: GET。

- `GET`: Cross-task asset search for the asset management center. Source V1_ASSET_OWNERSHIP §5.2 / §5.3.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `keyword` | query | string | 否 | Fuzzy match file_name / task_no / task title. |
| `module_key` | query | enum(basic_info/design/audit/warehouse/customization/procurement/retouch) | 否 | Restrict to one source module. |
| `owner_team_code` | query | string | 否 | Restrict to one owner team. |
| `is_archived` | query | enum(true/false/all) | 否 | Archive filter. Default `false`. `all` returns active + archived. |
| `task_status` | query | enum(open/closed/archived/all) | 否 | Task lifecycle filter. |
| `created_from` | query | string | 否 | Inclusive lower bound for asset created_at. |
| `created_to` | query | string | 否 | Inclusive upper bound for asset created_at. |
| `page` | query | integer | 否 | - |
| `size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {}
  ],
  "total": 123,
  "page": 123,
  "size": 123
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<Asset> | 否 | - |
| `total` | integer | 否 | - |
| `page` | integer | 否 | - |
| `size` | integer | 否 | - |

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
curl -X GET https://api.example.com/v1/assets/search \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 搜索接口是只读入口，低权限用户可能拿到空数组而不是错误。
- 高频输入框应做前端 debounce，避免无意义请求。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/design-sources/search

### 简介
支持方法: GET。

- `GET`: Source: V1_CUSTOMIZATION_WORKFLOW §3.2.2. v1 MVP — file_name / task_id keyword full-text search, ordered by created_at DESC. No advanced filters; SuperAdmin-maintained independent material repository deferred to R7+.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `keyword` | query | string | 否 | Full-text match against file_name and origin_task_id. |
| `page` | query | integer | 否 | - |
| `size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "id": "...",
      "file_name": "...",
      "owner_team_code": "...",
      "preview_url": "..."
    }
  ],
  "total": 123,
  "page": 123,
  "size": 123
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<DesignSourceEntry> | 否 | - |
| `total` | integer | 否 | - |
| `page` | integer | 否 | - |
| `size` | integer | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/design-sources/search \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 搜索接口是只读入口，低权限用户可能拿到空数组而不是错误。
- 高频输入框应做前端 debounce，避免无意义请求。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/search

### 简介
支持方法: GET。

- `GET`: Source: V1_INFORMATION_ARCHITECTURE §4.2. Global search across tasks / assets / products / users. Row-level policy (R1.7-D Q1=A1 + Q2=U1): tasks/assets/products return full matches regardless of caller scope; `users[]` is always `[]` unless caller role ∈ {super_admin, hr_admin}. Backend (R1.7-D Q3=B1): v1 uses MySQL LIKE; no ES / relevance scoring in v1.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `q` | query | string | 是 | - |
| `scope` | query | enum(all/tasks/assets/products/users) | 否 | - |
| `limit` | query | integer | 否 | Max items per result array. Default 20 (IA §4.2). |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "query": "string",
  "results": {
    "tasks": [
      {
        "id": 603,
        "task_no": "RW-20260427-A-000600",
        "title": "产品名称",
        "task_status": "PendingAssign",
        "priority": "normal",
        "task_type": "new_product_development",
        "sku_code": "HSC34009",
        "primary_sku_code": "HSC34009",
        "i_id": "常规KT板",
        "owner_department": "运营部",
        "owner_team": "淘系一组",
        "owner_org_team": "淘系一组",
        "creator_id": 1,
        "creator_name": "系统管理员",
        "designer_id": 12,
        "designer_name": "设计师A",
        "created_at": "2026-04-27T09:58:31Z",
        "deadline_at": "2026-04-30T09:26:00Z",
        "highlight": null
      }
    ],
    "assets": [
      {
        "asset_id": 1001,
        "file_name": "参考图.jpg",
        "source_module_key": "design",
        "task_id": 603
      }
    ],
    "products": [
      {
        "erp_code": "HSC34009",
        "product_name": "产品名称",
        "i_id": "常规KT板",
        "category": "常规KT板"
      }
    ],
    "users": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `query` | string | 是 | - |
| `results` | SearchResultGroup | 是 | Source: V1_INFORMATION_ARCHITECTURE §4.2. Decision (R1.7-D): all four arrays are item-schema fixed; `users[]` is always `[]` for callers other than super_admin / hr_admin regardless of match count (IA §4.3 row-level policy; R1.7-D Q1=A1 + Q2=U1). |

`results.tasks[]` 关键字段:

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` / `task_no` | integer / string | 任务主键与任务号。 |
| `title` | string/null | 当前为 `product_name_snapshot`。 |
| `task_type` | string/null | 任务类型，如 `new_product_development` / `original_product_development` / `purchase_task`。 |
| `sku_code` / `primary_sku_code` | string/null | 产品编码 / 主 SKU。 |
| `i_id` | string/null | 聚水潭款式/产品族 i_id，优先从任务明细和 ERP filing 快照解析。 |
| `owner_department` / `owner_team` / `owner_org_team` | string/null | 任务所属部门和组。 |
| `creator_id` / `creator_name` | integer/null / string/null | 创建人。 |
| `designer_id` / `designer_name` | integer/null / string/null | 接单人/设计师。 |
| `created_at` / `deadline_at` | datetime/null | 创建日期 / 截止日期，可用于前端展示。 |

后端匹配范围包括: 任务号、产品名称、SKU、i_id、任务类型、状态、优先级、所属部门/组、创建人/设计师用户名或展示名、创建/截止日期、需求/备注/设计要求/材质/规格/工艺/参考链接，以及任务关联设计图/参考图的文件名、原始文件名、OSS key、模块 key。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/search \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 搜索接口是只读入口，低权限用户可能拿到空数组而不是错误。
- 高频输入框应做前端 debounce，避免无意义请求。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。
