# 用户与管理审计

> Revision: V8 current contract (2026-07-20)
> Source: docs/api/openapi.yaml

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

用户管理、角色、访问规则、权限日志、操作日志与后台日志。

## Family 约定

- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 本文件覆盖 `6` 个 `/v1` path；同一路径多 method 合并在同一节。

## GET /v1/users

### 简介
支持方法: GET, POST。

- `GET`: Explicit-scope user-management read endpoint. Returns employee number, department, team, and effective frontend access state with server-side pagination and filtering. `keyword` matches username, display name, and employee number. `access.view` or `access.manage` scope is applied in SQL using stable user, department, and team IDs; list count and rows use the same scope predicate. Legacy roles and organization names never widen visibility. Stale or disabled department/team filter values on this read endpoint are treated as no-match filters and return an empty page instead of failing the whole user-management shell; user create/update inputs still reject invalid org values.
- `POST`: Creates one managed user after validating the selected department and team against `/v1/org/options`. `employee_no` must be globally unique and in range 0-9999. If `status` is omitted, the account is active. New users receive only the protected `Member` base role. Business roles and organization scopes are configured separately through the explicit access-policy endpoints after creation. Requires `access.manage`; the target department/team stable IDs must be inside the caller's effective scope.

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
| `status` | query | enum(active/disabled) | 否 | - |
| `department` | query | string | 否 | Must match an enabled department from `/v1/org/options`; stale or disabled values return an empty page. |
| `team` | query | string | 否 | Must match an enabled team under the selected department in `/v1/org/options`; stale or disabled values return an empty page. |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "id": "...",
      "username": "...",
      "account": "...",
      "employee_no": "..."
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
| `data` | array<WorkflowUser> | 否 | - |
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
curl -X GET https://api.example.com/v1/users \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `username` | string | 是 | - |
| `employee_no` | integer | 是 | 管理员维护的员工工号；纯数字，范围 0-9999，全局唯一。重复时服务端返回中文业务提示。 |
| `display_name` | string | 是 | - |
| `department` | string | 是 | Must match one enabled department from backend org master exposed by `/v1/org/options`. |
| `team` | string | 是 | Must match one enabled team under the selected department in backend org master exposed by `/v1/org/options`. |
| `mobile` | string | 是 | - |
| `email` | string | 否 | - |
| `password` | string | 是 | - |
| `status` | enum(active/disabled) | 否 | - |
| `employment_type` | enum(full_time/part_time) | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

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
curl -X POST https://api.example.com/v1/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/users/designers

### 简介
支持方法: GET。

- `GET`: Returns the active candidate pool used by task assignment, design handling, and audit handover controls. `workflow_lane=normal` selects designers, `customization` selects customization designers, `audit` selects unified auditors, and `all` returns design candidates across the active design lanes. Candidate membership comes exclusively from active `auth_*` role assignments and capabilities; legacy `user_roles` values and organization display names are not authorization inputs. The endpoint does not accept keyword, organization, or pagination filters. The response pagination block reports the returned candidate count.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `workflow_lane` | query | enum(normal/customization/audit/all) | 否 | Selects the active assignment candidate pool. |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "id": "...",
      "username": "...",
      "display_name": "...",
      "name": "..."
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
| `data` | array<object> | 否 | - |
| `pagination` | object | 否 | - |

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
curl -X GET https://api.example.com/v1/users/designers \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/users/{id}

### 简介
支持方法: GET, PATCH。

- `GET`: User detail read endpoint governed by `access.view` or `access.manage` and the same stable-ID scope model as the list endpoint.
- `PATCH`: Partial update for user profile and org affiliation. `employee_no` can be maintained by company-level user managers. It must be a pure numeric value in range 0-9999 and globally unique. Org-field contract: - `department` + `team` are the canonical write fields. - To remove a user from a formal group, use the unassigned-pool semantic: - set `department` to the unassigned department from `/v1/org/options` - set `team` to its unassigned pool team, or use the explicit `team = "ungrouped"` command supported by the current management UI. Requires `access.manage`. Both the current user organization and the requested new stable department/team IDs must be inside the caller's effective scope.

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
| 404 | 见 `error.code` | 见 `deny_code` | User not found |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/users/<id> \
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
| `employee_no` | integer | 否 | 管理员维护的员工工号；纯数字，范围 0-9999，全局唯一。传 `null` 会被拒绝，省略表示不修改。 |
| `display_name` | string | 否 | - |
| `status` | enum(active/disabled) | 否 | - |
| `employment_type` | enum(full_time/part_time) | 否 | - |
| `department` | Department | 否 | Display name from the organization master. Authorization and data scope use stable department/team IDs only; names are never permission conditions. |
| `team` | string | 否 | - |
| `email` | string | 否 | - |
| `mobile` | string | 否 | - |
| `managed_departments` | array<string> | 否 | - |
| `managed_teams` | array<string> | 否 | - |

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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden because `access.manage` is missing or the current/new stable organization is outside the effective scope. |
| 404 | 见 `error.code` | 见 `deny_code` | User not found |

##### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/users/<id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PUT /v1/users/{id}/password

### 简介
支持方法: PUT。

- `PUT`: Managed password reset endpoint. Replaces the target user's local password hash and returns the user record. Requires `access.manage`; the target user must be inside the caller's effective stable-ID scope. Existing session tokens are not revoked by this minimal reset operation.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PUT` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `password` | string | 是 | - |

### 响应体 schema
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
curl -X PUT https://api.example.com/v1/users/<id>/password \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/users/{id}/activate

### 简介
支持方法: POST。

- `POST`: Enables the target account. Requires `access.manage`; the target user must be inside the caller's effective stable-ID scope.

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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden (scope check failed) |
| 404 | 见 `error.code` | 见 `deny_code` | User not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/users/<id>/activate \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/users/{id}/deactivate

### 简介
支持方法: POST。

- `POST`: Disables the target account. Scope rules are identical to `activate`; self-deactivation is rejected.

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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden (scope check failed) |
| 404 | 见 `error.code` | 见 `deny_code` | User not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/users/<id>/deactivate \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

