# 当前用户

> Revision: V8 current contract (2026-07-20)
> Source: docs/api/openapi.yaml

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

当前登录用户、个人资料、组织视图与个人偏好。

## Family 约定

- 当前用户 family 只面向当前 token，不应用于管理其他用户。
- `GET /v1/me/avatar-files/{filename}` 是公开随机头像资源，浏览器可直接作为图片地址加载；头像变更仍走登录后的上传/删除接口。
- 通知路径拆到 `V1_API_NOTIFICATIONS.md`，避免重复接入。
- 本文件覆盖 `6` 个 `/v1` path；同一路径多 method 合并在同一节。

## GET /v1/me/task-drafts

### 简介
支持方法: GET。

- `GET`: Source: V1_INFORMATION_ARCHITECTURE §3.5.9. Returns only drafts owned by the authenticated user (cursor-based pagination).

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `task_type` | query | string | 否 | Optional task_type filter (for example `new_product_development` or `retouch_task`). |
| `limit` | query | integer | 否 | - |
| `cursor` | query | string | 否 | Opaque cursor returned by a previous page; omit for first page. |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "draft_id": "...",
      "owner_user_id": "...",
      "task_type": "...",
      "payload": "..."
    }
  ],
  "next_cursor": "string"
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<TaskDraft> | 否 | - |
| `next_cursor` | string | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/me/task-drafts \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 当前用户 family 只面向当前 token，不应用于管理其他用户。
- `GET /v1/me/avatar-files/{filename}` 是公开随机头像资源，浏览器可直接作为图片地址加载；头像变更仍走登录后的上传/删除接口。
- 通知路径拆到 `V1_API_NOTIFICATIONS.md`，避免重复接入。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/me

### 简介
支持方法: GET, PATCH。

- `GET`: Returns the currently authenticated user's full profile. Source: V1_INFORMATION_ARCHITECTURE §7.2 (账户信息).
- `PATCH`: Partial self-update for the authenticated user's display profile. Source: V1_INFORMATION_ARCHITECTURE §7.2 (账户信息 · 编辑 昵称/头像/手机/邮箱). This endpoint updates display_name/mobile/email only. Avatar fields are rejected here; use POST /v1/me/avatar to upload and DELETE /v1/me/avatar to clear the avatar.

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
    "id": 123,
    "username": "string",
    "account": "string",
    "employee_no": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | WorkflowUser | 否 | - |

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
curl -X GET https://api.example.com/v1/me \
  -H "Authorization: Bearer $TOKEN"
```

#### PATCH 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `display_name` | string | 否 | - |
| `mobile` | string | 否 | - |
| `email` | string | 否 | - |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "username": "string",
    "account": "string",
    "employee_no": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | WorkflowUser | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Validation failed |

##### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/me \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 当前用户 family 只面向当前 token，不应用于管理其他用户。
- `GET /v1/me/avatar-files/{filename}` 是公开随机头像资源，浏览器可直接作为图片地址加载；头像变更仍走登录后的上传/删除接口。
- 通知路径拆到 `V1_API_NOTIFICATIONS.md`，避免重复接入。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/me/avatar

### 简介
支持方法: POST, DELETE。

- `POST`: Uploads a small JPG, PNG, or WebP avatar for the authenticated user and stores the resulting avatar URL on the user profile.
- `DELETE`: Clears the authenticated user's avatar URL and removes the stored avatar file when it is managed by this service.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- `DELETE` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `multipart/form-data`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `file` | string | 是 | JPG, PNG, or WebP avatar image, max 2MB. |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "username": "string",
    "account": "string",
    "employee_no": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | WorkflowUser | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Validation failed |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/me/avatar \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@avatar.png"
```

#### DELETE 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "username": "string",
    "account": "string",
    "employee_no": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | WorkflowUser | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |

##### curl 示例
```bash
curl -X DELETE https://api.example.com/v1/me/avatar \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 当前用户 family 只面向当前 token，不应用于管理其他用户。
- `GET /v1/me/avatar-files/{filename}` 是公开随机头像资源，浏览器可直接作为图片地址加载；头像变更仍走登录后的上传/删除接口。
- 通知路径拆到 `V1_API_NOTIFICATIONS.md`，避免重复接入。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/me/avatar-files/{filename}

### 简介
支持方法: GET。

- `GET`: Publicly serves an uploaded avatar file by its generated random filename so browsers can load it directly in `<img>`. Filenames are generated by the server, do not contain user IDs, and should be treated as opaque.

### 鉴权与 RBAC
- 本节为公开资源接口，不需要 Bearer token。
- `GET` 允许角色: 公开。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `filename` | path | string | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 image/jpeg`

```json
"string"
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `body` | string | 视接口 | OpenAPI 声明的整体对象。 |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Avatar not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/me/avatar-files/<filename>
```

### 前端最佳实践
- 当前用户 family 只面向当前 token，不应用于管理其他用户。
- `GET /v1/me/avatar-files/{filename}` 是公开随机头像资源，浏览器可直接作为图片地址加载；头像变更仍走登录后的上传/删除接口。
- 通知路径拆到 `V1_API_NOTIFICATIONS.md`，避免重复接入。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/me/change-password

### 简介
支持方法: POST。

- `POST`: Self password change. Server enforces `new_password == confirm` and that `old_password` matches the current stored hash. Source: V1_INFORMATION_ARCHITECTURE §7.2 (安全 · 改密).

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
| `old_password` | string | 是 | - |
| `new_password` | string | 是 | - |
| `confirm` | string | 是 | Must equal `new_password`; server enforces equality. |
| `password_confirmation` | string | 否 | Backward-compatible alias for `confirm`. |

### 响应体 schema
成功响应: `204`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Validation failed (old_password mismatch or new_password != confirm) |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/me/change-password \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 当前用户 family 只面向当前 token，不应用于管理其他用户。
- `GET /v1/me/avatar-files/{filename}` 是公开随机头像资源，浏览器可直接作为图片地址加载；头像变更仍走登录后的上传/删除接口。
- 通知路径拆到 `V1_API_NOTIFICATIONS.md`，避免重复接入。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/me/org

### 简介
支持方法: GET。

- `GET`: Read-only org snapshot for the authenticated user: department, primary team, managed scope (for DeptAdmin / TeamLead), and role set. Source: V1_INFORMATION_ARCHITECTURE §7.2 (我的组织 · 只读).

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
    "department": "string",
    "team": "string",
    "managed_departments": [
      "..."
    ],
    "managed_teams": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | MyOrgProfile | 否 | Source: V1_INFORMATION_ARCHITECTURE §7.2 (我的组织 · 只读). R4-SA-B. |

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
curl -X GET https://api.example.com/v1/me/org \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 当前用户 family 只面向当前 token，不应用于管理其他用户。
- `GET /v1/me/avatar-files/{filename}` 是公开随机头像资源，浏览器可直接作为图片地址加载；头像变更仍走登录后的上传/删除接口。
- 通知路径拆到 `V1_API_NOTIFICATIONS.md`，避免重复接入。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

