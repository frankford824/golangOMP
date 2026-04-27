> Revision: V1.2 authority purge + OpenAPI GC (2026-04-26)
> Source: docs/api/openapi.yaml (post V1.2 path-closure GC)
> V1 SoT: docs/V1_BACKEND_SOURCE_OF_TRUTH.md

# 搜索

> Revision: V1.1-A2 contract drift purge (2026-04-27)
> Source: docs/api/openapi.yaml (post V1.1-A2)
> 与 v1.21 生产实际响应对齐


> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

全局搜索、资产搜索与设计来源搜索。

## Family 约定

- 搜索接口是只读入口，低权限用户可能拿到空数组而不是错误。
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
      "..."
    ],
    "assets": [
      "..."
    ],
    "products": [
      "..."
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

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 501 | 见 `error.code` | 见 `deny_code` | Reserved for R4-SA-D |

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

