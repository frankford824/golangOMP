# 通知

> Revision: V8 current contract (2026-07-20)
> Source: docs/api/openapi.yaml

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

站内通知列表、已读、全部已读、未读数与 5 类通知事件。

## Family 约定

- 当前通知类型：task_assigned_to_me、task_rejected、task_pending_audit、claim_conflict、pool_reassigned、task_cancelled。
- `payload` 按 `notification_type` 分型。前端应穷举已知 type；未知 type 使用兜底文案。
- `task_assigned_to_me` payload：必有 `task_id`；可能有 `task_no`、`task_type`、`module_key`、`action`、`assigned_by`、`assigned_by_name`、`designer_id`、`previous_designer_id`、`previous_handler_id`、`reason`、`remark`、`batch_request_id`。
- `task_rejected` payload：必有 `task_id`、`reject_reason`；可能有 `task_no`、`module_key`、`rejected_by`、`rejected_by_name`。
- `task_pending_audit` payload：必有 `task_id`、`module_key`、`pool_team_code`；可能有 `task_no`、`task_type`、`submitted_by`、`submitted_by_name`。
- `claim_conflict` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`winner_user_id`、`winner_user_name`。
- `pool_reassigned` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`team_code`、`team_name`、`reassigned_by`、`reassigned_by_name`。
- `task_cancelled` payload：必有 `task_id`、`cancel_reason`、`cancelled_by`；可能有 `task_no`、`cancelled_by_name`、`module_key`。
- payload 允许后端携带人名、团队名等展示冗余字段，例如 `assigned_by_name`；这些字段是展示快照，前端跳转和业务定位仍以 `task_id`、用户 id、`module_key` 等稳定字段为准。
- 未读数用于 badge，列表分页以接口返回 cursor/limit 字段为准。
- 本文件覆盖 `9` 个 `/v1` path；同一路径多 method 合并在同一节。

## GET /v1/me/notifications

### 简介
支持方法: GET。

- `GET`: Source: V1_INFORMATION_ARCHITECTURE §8.3. Cursor-based pagination; optional `is_read` filter (omit for 'all'). Notifications are always scoped to the authenticated user; no cross-user reads regardless of role. This main-operations route excludes all `asset_workbench_*` notification types; asset-workbench clients use `/v1/asset-workbench/notifications`.

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
- 当前通知类型：task_assigned_to_me、task_rejected、task_pending_audit、claim_conflict、pool_reassigned、task_cancelled。
- `payload` 按 `notification_type` 分型。前端应穷举已知 type；未知 type 使用兜底文案。
- `task_assigned_to_me` payload：必有 `task_id`；可能有 `task_no`、`task_type`、`module_key`、`action`、`assigned_by`、`assigned_by_name`、`designer_id`、`previous_designer_id`、`previous_handler_id`、`reason`、`remark`、`batch_request_id`。
- `task_rejected` payload：必有 `task_id`、`reject_reason`；可能有 `task_no`、`module_key`、`rejected_by`、`rejected_by_name`。
- `task_pending_audit` payload：必有 `task_id`、`module_key`、`pool_team_code`；可能有 `task_no`、`task_type`、`submitted_by`、`submitted_by_name`。
- `claim_conflict` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`winner_user_id`、`winner_user_name`。
- `pool_reassigned` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`team_code`、`team_name`、`reassigned_by`、`reassigned_by_name`。
- `task_cancelled` payload：必有 `task_id`、`cancel_reason`、`cancelled_by`；可能有 `task_no`、`cancelled_by_name`、`module_key`。
- payload 允许后端携带人名、团队名等展示冗余字段，例如 `assigned_by_name`；这些字段是展示快照，前端跳转和业务定位仍以 `task_id`、用户 id、`module_key` 等稳定字段为准。
- 未读数用于 badge，列表分页以接口返回 cursor/limit 字段为准。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
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
- 当前通知类型：task_assigned_to_me、task_rejected、task_pending_audit、claim_conflict、pool_reassigned、task_cancelled。
- `payload` 按 `notification_type` 分型。前端应穷举已知 type；未知 type 使用兜底文案。
- `task_assigned_to_me` payload：必有 `task_id`；可能有 `task_no`、`task_type`、`module_key`、`action`、`assigned_by`、`assigned_by_name`、`designer_id`、`previous_designer_id`、`previous_handler_id`、`reason`、`remark`、`batch_request_id`。
- `task_rejected` payload：必有 `task_id`、`reject_reason`；可能有 `task_no`、`module_key`、`rejected_by`、`rejected_by_name`。
- `task_pending_audit` payload：必有 `task_id`、`module_key`、`pool_team_code`；可能有 `task_no`、`task_type`、`submitted_by`、`submitted_by_name`。
- `claim_conflict` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`winner_user_id`、`winner_user_name`。
- `pool_reassigned` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`team_code`、`team_name`、`reassigned_by`、`reassigned_by_name`。
- `task_cancelled` payload：必有 `task_id`、`cancel_reason`、`cancelled_by`；可能有 `task_no`、`cancelled_by_name`、`module_key`。
- payload 允许后端携带人名、团队名等展示冗余字段，例如 `assigned_by_name`；这些字段是展示快照，前端跳转和业务定位仍以 `task_id`、用户 id、`module_key` 等稳定字段为准。
- 未读数用于 badge，列表分页以接口返回 cursor/limit 字段为准。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
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
- 当前通知类型：task_assigned_to_me、task_rejected、task_pending_audit、claim_conflict、pool_reassigned、task_cancelled。
- `payload` 按 `notification_type` 分型。前端应穷举已知 type；未知 type 使用兜底文案。
- `task_assigned_to_me` payload：必有 `task_id`；可能有 `task_no`、`task_type`、`module_key`、`action`、`assigned_by`、`assigned_by_name`、`designer_id`、`previous_designer_id`、`previous_handler_id`、`reason`、`remark`、`batch_request_id`。
- `task_rejected` payload：必有 `task_id`、`reject_reason`；可能有 `task_no`、`module_key`、`rejected_by`、`rejected_by_name`。
- `task_pending_audit` payload：必有 `task_id`、`module_key`、`pool_team_code`；可能有 `task_no`、`task_type`、`submitted_by`、`submitted_by_name`。
- `claim_conflict` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`winner_user_id`、`winner_user_name`。
- `pool_reassigned` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`team_code`、`team_name`、`reassigned_by`、`reassigned_by_name`。
- `task_cancelled` payload：必有 `task_id`、`cancel_reason`、`cancelled_by`；可能有 `task_no`、`cancelled_by_name`、`module_key`。
- payload 允许后端携带人名、团队名等展示冗余字段，例如 `assigned_by_name`；这些字段是展示快照，前端跳转和业务定位仍以 `task_id`、用户 id、`module_key` 等稳定字段为准。
- 未读数用于 badge，列表分页以接口返回 cursor/limit 字段为准。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
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
- 当前通知类型：task_assigned_to_me、task_rejected、task_pending_audit、claim_conflict、pool_reassigned、task_cancelled。
- `payload` 按 `notification_type` 分型。前端应穷举已知 type；未知 type 使用兜底文案。
- `task_assigned_to_me` payload：必有 `task_id`；可能有 `task_no`、`task_type`、`module_key`、`action`、`assigned_by`、`assigned_by_name`、`designer_id`、`previous_designer_id`、`previous_handler_id`、`reason`、`remark`、`batch_request_id`。
- `task_rejected` payload：必有 `task_id`、`reject_reason`；可能有 `task_no`、`module_key`、`rejected_by`、`rejected_by_name`。
- `task_pending_audit` payload：必有 `task_id`、`module_key`、`pool_team_code`；可能有 `task_no`、`task_type`、`submitted_by`、`submitted_by_name`。
- `claim_conflict` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`winner_user_id`、`winner_user_name`。
- `pool_reassigned` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`team_code`、`team_name`、`reassigned_by`、`reassigned_by_name`。
- `task_cancelled` payload：必有 `task_id`、`cancel_reason`、`cancelled_by`；可能有 `task_no`、`cancelled_by_name`、`module_key`。
- payload 允许后端携带人名、团队名等展示冗余字段，例如 `assigned_by_name`；这些字段是展示快照，前端跳转和业务定位仍以 `task_id`、用户 id、`module_key` 等稳定字段为准。
- 未读数用于 badge，列表分页以接口返回 cursor/limit 字段为准。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/me/notifications/web-push/config

### 简介
支持方法: GET。

- `GET`: Returns whether Web Push is enabled and the current VAPID public key for the authenticated user session.

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
    "enabled": true,
    "public_key": "string",
    "vapid_key_hash": "string",
    "subject": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | WebPushConfig | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/me/notifications/web-push/config \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 当前通知类型：task_assigned_to_me、task_rejected、task_pending_audit、claim_conflict、pool_reassigned、task_cancelled。
- `payload` 按 `notification_type` 分型。前端应穷举已知 type；未知 type 使用兜底文案。
- `task_assigned_to_me` payload：必有 `task_id`；可能有 `task_no`、`task_type`、`module_key`、`action`、`assigned_by`、`assigned_by_name`、`designer_id`、`previous_designer_id`、`previous_handler_id`、`reason`、`remark`、`batch_request_id`。
- `task_rejected` payload：必有 `task_id`、`reject_reason`；可能有 `task_no`、`module_key`、`rejected_by`、`rejected_by_name`。
- `task_pending_audit` payload：必有 `task_id`、`module_key`、`pool_team_code`；可能有 `task_no`、`task_type`、`submitted_by`、`submitted_by_name`。
- `claim_conflict` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`winner_user_id`、`winner_user_name`。
- `pool_reassigned` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`team_code`、`team_name`、`reassigned_by`、`reassigned_by_name`。
- `task_cancelled` payload：必有 `task_id`、`cancel_reason`、`cancelled_by`；可能有 `task_no`、`cancelled_by_name`、`module_key`。
- payload 允许后端携带人名、团队名等展示冗余字段，例如 `assigned_by_name`；这些字段是展示快照，前端跳转和业务定位仍以 `task_id`、用户 id、`module_key` 等稳定字段为准。
- 未读数用于 badge，列表分页以接口返回 cursor/limit 字段为准。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/me/notifications/web-push/subscriptions

### 简介
支持方法: POST。

- `POST`: Upserts by global endpoint hash and reassigns ownership to the current user, preventing shared-browser leakage after account switch.

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
| `endpoint` | string | 是 | - |
| `keys` | object | 是 | - |
| `user_agent` | string | 否 | - |
| `platform` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "user_id": 123,
    "endpoint_hash": "string",
    "status": "active"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | WebPushSubscription | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Web Push disabled or invalid subscription |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/me/notifications/web-push/subscriptions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 当前通知类型：task_assigned_to_me、task_rejected、task_pending_audit、claim_conflict、pool_reassigned、task_cancelled。
- `payload` 按 `notification_type` 分型。前端应穷举已知 type；未知 type 使用兜底文案。
- `task_assigned_to_me` payload：必有 `task_id`；可能有 `task_no`、`task_type`、`module_key`、`action`、`assigned_by`、`assigned_by_name`、`designer_id`、`previous_designer_id`、`previous_handler_id`、`reason`、`remark`、`batch_request_id`。
- `task_rejected` payload：必有 `task_id`、`reject_reason`；可能有 `task_no`、`module_key`、`rejected_by`、`rejected_by_name`。
- `task_pending_audit` payload：必有 `task_id`、`module_key`、`pool_team_code`；可能有 `task_no`、`task_type`、`submitted_by`、`submitted_by_name`。
- `claim_conflict` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`winner_user_id`、`winner_user_name`。
- `pool_reassigned` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`team_code`、`team_name`、`reassigned_by`、`reassigned_by_name`。
- `task_cancelled` payload：必有 `task_id`、`cancel_reason`、`cancelled_by`；可能有 `task_no`、`cancelled_by_name`、`module_key`。
- payload 允许后端携带人名、团队名等展示冗余字段，例如 `assigned_by_name`；这些字段是展示快照，前端跳转和业务定位仍以 `task_id`、用户 id、`module_key` 等稳定字段为准。
- 未读数用于 badge，列表分页以接口返回 cursor/limit 字段为准。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## DELETE /v1/me/notifications/web-push/subscriptions/current

### 简介
支持方法: DELETE。

- `DELETE`: Disables the provided endpoint only when it belongs to the authenticated user. Missing endpoints are treated as idempotent no-ops.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `DELETE` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `endpoint` | string | 否 | - |

### 响应体 schema
成功响应: `204`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |

### curl 示例
```bash
curl -X DELETE https://api.example.com/v1/me/notifications/web-push/subscriptions/current \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 当前通知类型：task_assigned_to_me、task_rejected、task_pending_audit、claim_conflict、pool_reassigned、task_cancelled。
- `payload` 按 `notification_type` 分型。前端应穷举已知 type；未知 type 使用兜底文案。
- `task_assigned_to_me` payload：必有 `task_id`；可能有 `task_no`、`task_type`、`module_key`、`action`、`assigned_by`、`assigned_by_name`、`designer_id`、`previous_designer_id`、`previous_handler_id`、`reason`、`remark`、`batch_request_id`。
- `task_rejected` payload：必有 `task_id`、`reject_reason`；可能有 `task_no`、`module_key`、`rejected_by`、`rejected_by_name`。
- `task_pending_audit` payload：必有 `task_id`、`module_key`、`pool_team_code`；可能有 `task_no`、`task_type`、`submitted_by`、`submitted_by_name`。
- `claim_conflict` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`winner_user_id`、`winner_user_name`。
- `pool_reassigned` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`team_code`、`team_name`、`reassigned_by`、`reassigned_by_name`。
- `task_cancelled` payload：必有 `task_id`、`cancel_reason`、`cancelled_by`；可能有 `task_no`、`cancelled_by_name`、`module_key`。
- payload 允许后端携带人名、团队名等展示冗余字段，例如 `assigned_by_name`；这些字段是展示快照，前端跳转和业务定位仍以 `task_id`、用户 id、`module_key` 等稳定字段为准。
- 未读数用于 badge，列表分页以接口返回 cursor/limit 字段为准。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/me/notifications/web-push/test

### 简介
支持方法: POST。

- `POST`: Queues a lightweight test notification for the current user and rate-limits repeated test sends.

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
    "notification_id": 123,
    "queued": true,
    "message": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | WebPushTestResult | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Web Push disabled or test is rate-limited |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/me/notifications/web-push/test \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 当前通知类型：task_assigned_to_me、task_rejected、task_pending_audit、claim_conflict、pool_reassigned、task_cancelled。
- `payload` 按 `notification_type` 分型。前端应穷举已知 type；未知 type 使用兜底文案。
- `task_assigned_to_me` payload：必有 `task_id`；可能有 `task_no`、`task_type`、`module_key`、`action`、`assigned_by`、`assigned_by_name`、`designer_id`、`previous_designer_id`、`previous_handler_id`、`reason`、`remark`、`batch_request_id`。
- `task_rejected` payload：必有 `task_id`、`reject_reason`；可能有 `task_no`、`module_key`、`rejected_by`、`rejected_by_name`。
- `task_pending_audit` payload：必有 `task_id`、`module_key`、`pool_team_code`；可能有 `task_no`、`task_type`、`submitted_by`、`submitted_by_name`。
- `claim_conflict` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`winner_user_id`、`winner_user_name`。
- `pool_reassigned` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`team_code`、`team_name`、`reassigned_by`、`reassigned_by_name`。
- `task_cancelled` payload：必有 `task_id`、`cancel_reason`、`cancelled_by`；可能有 `task_no`、`cancelled_by_name`、`module_key`。
- payload 允许后端携带人名、团队名等展示冗余字段，例如 `assigned_by_name`；这些字段是展示快照，前端跳转和业务定位仍以 `task_id`、用户 id、`module_key` 等稳定字段为准。
- 未读数用于 badge，列表分页以接口返回 cursor/limit 字段为准。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/me/notifications/preferences

### 简介
支持方法: GET, PATCH。

- `GET`: Get my notification preferences
- `PATCH`: Update my notification preferences

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- `PATCH` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "user_id": 123,
    "web_push_enabled": true,
    "last_test_sent_at": "2026-04-25T10:30:41Z",
    "vapid_key_hash": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | NotificationPreferences | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/me/notifications/preferences \
  -H "Authorization: Bearer $TOKEN"
```

#### PATCH 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `web_push_enabled` | boolean | 否 | - |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "user_id": 123,
    "web_push_enabled": true,
    "last_test_sent_at": "2026-04-25T10:30:41Z",
    "vapid_key_hash": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | NotificationPreferences | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |

##### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/me/notifications/preferences \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 当前通知类型：task_assigned_to_me、task_rejected、task_pending_audit、claim_conflict、pool_reassigned、task_cancelled。
- `payload` 按 `notification_type` 分型。前端应穷举已知 type；未知 type 使用兜底文案。
- `task_assigned_to_me` payload：必有 `task_id`；可能有 `task_no`、`task_type`、`module_key`、`action`、`assigned_by`、`assigned_by_name`、`designer_id`、`previous_designer_id`、`previous_handler_id`、`reason`、`remark`、`batch_request_id`。
- `task_rejected` payload：必有 `task_id`、`reject_reason`；可能有 `task_no`、`module_key`、`rejected_by`、`rejected_by_name`。
- `task_pending_audit` payload：必有 `task_id`、`module_key`、`pool_team_code`；可能有 `task_no`、`task_type`、`submitted_by`、`submitted_by_name`。
- `claim_conflict` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`winner_user_id`、`winner_user_name`。
- `pool_reassigned` payload：必有 `task_id`、`module_key`；可能有 `task_no`、`team_code`、`team_name`、`reassigned_by`、`reassigned_by_name`。
- `task_cancelled` payload：必有 `task_id`、`cancel_reason`、`cancelled_by`；可能有 `task_no`、`cancelled_by_name`、`module_key`。
- payload 允许后端携带人名、团队名等展示冗余字段，例如 `assigned_by_name`；这些字段是展示快照，前端跳转和业务定位仍以 `task_id`、用户 id、`module_key` 等稳定字段为准。
- 未读数用于 badge，列表分页以接口返回 cursor/limit 字段为准。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

