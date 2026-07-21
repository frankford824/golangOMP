# 搜索

> Revision: V8 current contract (2026-07-20)
> Source: docs/api/openapi.yaml

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

全局搜索、资产搜索与设计来源搜索。

## Family 约定

- 搜索接口是只读入口，低权限用户可能拿到空数组而不是错误。
- `GET /v1/search` 的任务搜索覆盖任务号、产品名、SKU、i_id、任务类型、创建人、所属组、设计师、日期与任务关联设计图/参考图文件信息。
- 高频输入框应做前端 debounce，避免无意义请求。
- 本文件覆盖 `2` 个 `/v1` path；同一路径多 method 合并在同一节。

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
- `GET /v1/search` 的任务搜索覆盖任务号、产品名、SKU、i_id、任务类型、创建人、所属组、设计师、日期与任务关联设计图/参考图文件信息。
- 高频输入框应做前端 debounce，避免无意义请求。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/search

### 简介
支持方法: GET。

- `GET`: Source: V1_INFORMATION_ARCHITECTURE §4.2. Global search across tasks / assets / products / users. The route hydrates the caller's explicit access policy before searching. Internal task-resource groups require `asset.view` and are filtered by self / own department / own team / selected organization / global scope. AssetSubmitter results are limited to published finalized revisions. Resource-group results are pinned to `finalized_revision_id`; staged files and historical revisions are never returned. `users[]` remains limited to authorized people-management views.

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
| `mode` | query | enum(auto/exact/hybrid) | 否 | Auto keeps identifier-like input exact and uses hybrid retrieval only for natural language. |

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
  },
  "retrieval": {
    "requested_mode": "auto",
    "mode": "exact",
    "degraded": true,
    "candidates": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `query` | string | 是 | - |
| `results` | SearchResultGroup | 是 | All four arrays use fixed item schemas. Each branch is fail-closed by its explicit capability and stable-ID data scope. `users[]` is returned only when the caller has `access.view` or `access.manage`; legacy role names do not grant search visibility. |
| `retrieval` | SearchRetrievalMeta | 是 | - |

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
- `GET /v1/search` 的任务搜索覆盖任务号、产品名、SKU、i_id、任务类型、创建人、所属组、设计师、日期与任务关联设计图/参考图文件信息。
- 高频输入框应做前端 debounce，避免无意义请求。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

