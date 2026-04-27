> Revision: V1.2 authority purge + OpenAPI GC (2026-04-26)
> Source: docs/api/openapi.yaml (post V1.2 path-closure GC)
> V1 SoT: docs/V1_BACKEND_SOURCE_OF_TRUTH.md

# 认证与登录

> Revision: V1.1-A2 contract drift purge (2026-04-27)
> Source: docs/api/openapi.yaml (post V1.1-A2)
> 与 v1.21 生产实际响应对齐


> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

登录、注册、会话身份与密码变更。

## Family 约定

- 公开端点仅限注册、登录、注册选项；其余端点需要 Bearer token。
- 登录成功后，前端统一使用 `Authorization: Bearer <token>`。
- 本文件覆盖 `5` 个 `/v1` path；同一路径多 method 合并在同一节。

## POST /v1/auth/register

### 简介
支持方法: POST。

- `POST`: Frontend-ready registration endpoint for the current minimal auth mainline. Registration now accepts explicit profile fields (`name`, `department`, optional `team`, `mobile`, optional `email`, `account`, `password`, optional admin key). Department must match the backend org master exposed by `/v1/org/options`. If `team` is provided it must belong to the selected department. `mobile` is validated and kept unique. If the provided admin key matches the configured department rule, the new user is granted `DepartmentAdmin`. The response includes `user.frontend_access`.

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
| `username` | string | 否 | Compatibility alias of account |
| `account` | string | 是 | - |
| `display_name` | string | 否 | - |
| `name` | string | 是 | - |
| `department` | Department | 是 | Dynamic backend org-master department name. Values come from enabled `org_departments` rows exposed by `/v1/org/options`; this is no longer a fixed frontend enum. v1.0 business baseline is exactly `人事部`, `运营部`, `设计研发部`, `定制美工部`, `审核部`, `云仓部` (plus the system bucket `未分配`). Legacy names (`设计部`, `采购部`, `仓储部`, `烘焙仓储部`) remain only as disabled compatibility rows for historical integrity and are not returned by `/v1/org/options` nor accepted by registration / user admin / task create inputs. |
| `team` | string | 否 | - |
| `group` | string | 否 | Compatibility alias of team |
| `mobile` | string | 是 | - |
| `phone` | string | 否 | Compatibility alias of mobile |
| `email` | string | 否 | - |
| `password` | string | 是 | - |
| `admin_key` | string | 否 | Department admin registration key. If a valid department admin key is provided, the user is registered as a department admin (role: dept_admin) for the specified department. If omitted or invalid, the user is registered as a regular member. Super admin accounts are NOT created through self-registration; they are managed via auth_identity.json configuration (super_admins section). |
| `secret_key` | string | 否 | Compatibility alias of admin_key |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "user": {
      "id": "...",
      "username": "...",
      "account": "...",
      "display_name": "..."
    },
    "session": {
      "session_id": "...",
      "token": "...",
      "token_type": "...",
      "expires_at": "..."
    }
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AuthResult | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid registration payload, invalid department/team/mobile/email, or duplicate account/mobile |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/auth/register \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 公开端点仅限注册、登录、注册选项；其余端点需要 Bearer token。
- 登录成功后，前端统一使用 `Authorization: Bearer <token>`。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/auth/register-options

### 简介
支持方法: GET。

- `GET`: Returns the current enabled backend org-master department list plus enabled teams per department for frontend register-form initialization. Departments without active teams return an empty `teams` array.

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
    "departments": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | RegistrationOptions | 否 | - |

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
curl -X GET https://api.example.com/v1/auth/register-options \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 公开端点仅限注册、登录、注册选项；其余端点需要 Bearer token。
- 登录成功后，前端统一使用 `Authorization: Bearer <token>`。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/auth/login

### 简介
支持方法: POST。

- `POST`: Login endpoint for local session-token auth. Returns a bearer token backed by persisted `user_sessions` rows. This contract does not introduce SSO, org sync, or external identity providers. The response includes `session_id` plus the current frontend permission contract under `user.frontend_access`.

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
| `username` | string | 是 | Workflow user login name. Runtime handler binds this as the primary field. |
| `account` | string | 否 | Compatibility alias of `username`. |
| `password` | string | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "user": {
      "id": "...",
      "username": "...",
      "account": "...",
      "display_name": "..."
    },
    "session": {
      "session_id": "...",
      "token": "...",
      "token_type": "...",
      "expires_at": "..."
    }
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AuthResult | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Invalid account or password |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/auth/login \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 公开端点仅限注册、登录、注册选项；其余端点需要 Bearer token。
- 登录成功后，前端统一使用 `Authorization: Bearer <token>`。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/auth/me

### 简介
支持方法: GET。

- `GET`: Returns the current bearer-session user. Debug headers are not accepted as a substitute for login on this route. `data.frontend_access` is the explicit frontend-ready contract for page, menu, action, scope, role, department, and team gating.

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
    "id": 123,
    "username": "string",
    "account": "string",
    "display_name": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | WorkflowUser | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Authentication required |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/auth/me \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 公开端点仅限注册、登录、注册选项；其余端点需要 Bearer token。
- 登录成功后，前端统一使用 `Authorization: Bearer <token>`。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PUT /v1/auth/password

### 简介
支持方法: PUT。

- `PUT`: Session-backed current-user password change endpoint. Requires `old_password` verification and validates `new_password` with the same minimum password rules used during registration.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PUT` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `old_password` | string | 是 | - |
| `new_password` | string | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "message": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid new password |
| 401 | 见 `error.code` | 见 `deny_code` | Authentication required or old password incorrect |

### curl 示例
```bash
curl -X PUT https://api.example.com/v1/auth/password \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 公开端点仅限注册、登录、注册选项；其余端点需要 Bearer token。
- 登录成功后，前端统一使用 `Authorization: Bearer <token>`。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

