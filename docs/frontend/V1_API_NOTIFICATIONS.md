> Revision: V1.2 authority purge + OpenAPI GC (2026-04-26)
> Source: docs/api/openapi.yaml (post V1.2 path-closure GC)
> V1 SoT: docs/V1_BACKEND_SOURCE_OF_TRUTH.md

# 通知

> Revision: V1.1-A2 contract drift purge (2026-04-27)
> Source: docs/api/openapi.yaml (post V1.1-A2)
> 与 v1.21 生产实际响应对齐


> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

站内通知列表、已读、全部已读、未读数与 5 类通知事件。

## Family 约定

- V1 冻结 5 类通知类型：task_assigned_to_me、task_rejected、claim_conflict、pool_reassigned、task_cancelled。
- 未读数用于 badge，列表分页以接口返回 cursor/limit 字段为准。
- 本文件覆盖 `4` 个 `/v1` path；同一路径多 method 合并在同一节。

## GET /v1/me/notifications

### 简介
支持方法: GET。

- `GET`: Source: V1_INFORMATION_ARCHITECTURE §8.3. Cursor-based pagination; optional `is_read` filter (omit for 'all'). Notifications are always scoped to the authenticated user; no cross-user reads regardless of role.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `is_read` | query | boolean | 否 | Optional filter; omit to return both read and unread. |
| `limit` | query | integer | 否 | - |
| `cursor` | query | string | 否 | Opaque cursor returned by a previous page; omit for first page. |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "id": "...",
      "notification_type": "...",
      "payload": "...",
      "is_read": "..."
    }
  ],
  "next_cursor": "string"
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<Notification> | 否 | - |
| `next_cursor` | string | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/me/notifications \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- V1 冻结 5 类通知类型：task_assigned_to_me、task_rejected、claim_conflict、pool_reassigned、task_cancelled。
- 未读数用于 badge，列表分页以接口返回 cursor/limit 字段为准。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/me/notifications/{id}/read

### 简介
支持方法: POST。

- `POST`: Source: V1_INFORMATION_ARCHITECTURE §8.3. Only the notification owner (`user_id = current_user`) may mark it read; other users → 403. Repeated marking on already-read rows is idempotent.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `204`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Not the notification owner |
| 404 | 见 `error.code` | 见 `deny_code` | Notification not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/me/notifications/<id>/read \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- V1 冻结 5 类通知类型：task_assigned_to_me、task_rejected、claim_conflict、pool_reassigned、task_cancelled。
- 未读数用于 badge，列表分页以接口返回 cursor/limit 字段为准。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/me/notifications/read-all

### 简介
支持方法: POST。

- `POST`: Source: V1_INFORMATION_ARCHITECTURE §8.3. Scoped to the authenticated user; UPDATE targets `notifications WHERE user_id = current_user AND is_read = 0`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

请求体: 无请求体。

### 响应体 schema
成功响应: `204`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/me/notifications/read-all \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- V1 冻结 5 类通知类型：task_assigned_to_me、task_rejected、claim_conflict、pool_reassigned、task_cancelled。
- 未读数用于 badge，列表分页以接口返回 cursor/limit 字段为准。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/me/notifications/unread-count

### 简介
支持方法: GET。

- `GET`: Source: V1_INFORMATION_ARCHITECTURE §8.3 (右上角 badge 用). Counts `notifications WHERE user_id = current_user AND is_read = 0`.

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
    "unread_count": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/me/notifications/unread-count \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- V1 冻结 5 类通知类型：task_assigned_to_me、task_rejected、claim_conflict、pool_reassigned、task_cancelled。
- 未读数用于 badge，列表分页以接口返回 cursor/limit 字段为准。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

