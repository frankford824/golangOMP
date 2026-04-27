# 用户与管理审计

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

用户管理、角色、访问规则、权限日志、操作日志与后台日志。

## Family 约定

- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 本文件覆盖 `18` 个 `/v1` path；同一路径多 method 合并在同一节。

## GET /v1/roles

### 简介
支持方法: GET。

- `GET`: Minimal role/permission catalog for the current auth/org scope. Read access is available to management roles, including DepartmentAdmin, plus legacy org/role compatibility admins.

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
      "role": "...",
      "name": "...",
      "description": "...",
      "capabilities": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<RoleCatalogEntry> | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/roles \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/access-rules

### 简介
支持方法: GET。

- `GET`: Super-admin or HR inspection endpoint for the current route-to-role authorization contract, including whether a route is session-only or still debug-compatible placeholder scope.

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
      "method": "...",
      "path": "...",
      "readiness": "...",
      "required_roles": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<RouteAccessRule> | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/access-rules \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/users

### 简介
支持方法: GET, POST。

- `GET`: Management-scoped user-management read endpoint. Returns user department, team, role, and frontend access state for frontend integration, with server-side pagination and filtering. `DepartmentAdmin` reads are forced to own department. `TeamLead` reads are forced to own team inside own department.
- `POST`: Managed user creation endpoint. Validates org fields against `/v1/org/options`, validates roles against the workflow role catalog, sets the initial password hash, and returns the created user with `frontend_access`. If `status` is omitted the user is created as `active`. `DepartmentAdmin` can create users only inside own department and only with department-compatible business roles.

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
| `role` | query | enum(Member/SuperAdmin/HRAdmin/OrgAdmin/RoleAdmin/TeamLead/DesignDirector/DesignReviewer/Ops/Designer/Audit_A/Audit_B...) | 否 | - |
| `department` | query | string | 否 | Must match `/v1/org/options`. |
| `team` | query | string | 否 | Must match the selected department in `/v1/org/options`. |
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
| `account` | string | 否 | Compatibility alias of username. |
| `display_name` | string | 是 | - |
| `name` | string | 否 | Compatibility alias of display_name. |
| `department` | string | 是 | Must match one enabled department from backend org master exposed by `/v1/org/options`. |
| `team` | string | 是 | Must match one enabled team under the selected department in backend org master exposed by `/v1/org/options`. |
| `group` | string | 否 | Compatibility alias of team. |
| `mobile` | string | 是 | - |
| `phone` | string | 否 | Compatibility alias of mobile. |
| `email` | string | 否 | - |
| `password` | string | 是 | - |
| `roles` | array<V7Role> | 否 | - |
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
    "display_name": "string"
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
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/users/designers

### 简介
支持方法: GET。

- `GET`: Returns users with Designer role for task creation/assignment dropdowns. Prefer this over filtering the full user list. Round C (v1.5) widens the route guard so Ops task creators plus HR/SuperAdmin can look up designers cross-department. `DepartmentAdmin` is intentionally NOT in the guard list: cross-department designer lookup is an Ops-style capability only. DepartmentAdmin remains scoped by `authorizeUserListFilter` on the canonical `/v1/users` path. As of v1.6 (Round D), this endpoint uses a dedicated assignment-candidate-pool service path (`IdentityService.ListAssignableDesigners`) that bypasses the standard user-list authorization filter. By default it remains restricted to `role=Designer` + `status=active` and does NOT accept department/team/keyword or pagination parameters. Round N adds `workflow_lane` to select the candidate-pool lane while preserving no-parameter backward compatibility. The route guard listed in `x-rbac-placeholder.required_roles` is the sole access control for this method; the service path performs no additional department/team scoping. Response envelope is `{data, pagination}` where `pagination.page_size` and `pagination.total` both reflect the full returned list length (no server-side pagination).

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `workflow_lane` | query | enum(normal/customization/all) | 否 | Selects the assignment candidate pool lane. `normal` (default, back-compat) returns active users with role=Designer. `customization` returns active users with role=CustomizationOperator. `all` returns the union. |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "id": "...",
      "username": "...",
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
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/users/{id}

### 简介
支持方法: GET, PATCH, DELETE。

- `GET`: Management-scoped user detail read endpoint. `DepartmentAdmin` can read only users in own department. `TeamLead` can read only users in own team.
- `PATCH`: Partial update for user profile and org affiliation. Org-field contract: - `department` + `team` are the canonical write fields. - `group` is a compatibility alias of `team` (when both are provided they must be equal). - To remove a user from a formal group, use the unassigned-pool semantic: - set `department` to the unassigned department from `/v1/org/options` - set `team` (or `group`) to its unassigned pool team. - Compatibility alias: `team/group = "ungrouped"` is normalized by backend into the configured unassigned pool. `DepartmentAdmin` can move users across teams inside own department and assign unassigned users into own department, but cannot move users across departments or mutate managed scope.
- `DELETE`: Delete workflow user

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- `PATCH` 允许角色: 已登录 / scope-aware。
- `DELETE` 允许角色: 已登录 / scope-aware。
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
    "display_name": "string"
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
| `display_name` | string | 否 | - |
| `status` | enum(active/disabled) | 否 | - |
| `employment_type` | enum(full_time/part_time) | 否 | - |
| `department` | Department | 否 | Dynamic backend org-master department name. Values come from enabled `org_departments` rows exposed by `/v1/org/options`; this is no longer a fixed frontend enum. v1.0 business baseline is exactly `人事部`, `运营部`, `设计研发部`, `定制美工部`, `审核部`, `云仓部` (plus the system bucket `未分配`). Legacy names (`设计部`, `采购部`, `仓储部`, `烘焙仓储部`) remain only as disabled compatibility rows for historical integrity and are not returned by `/v1/org/options` nor accepted by registration / user admin / task create inputs. |
| `team` | string | 否 | - |
| `group` | string | 否 | Compatibility alias of team. |
| `email` | string | 否 | - |
| `mobile` | string | 否 | - |
| `avatar` | string | 否 | R1.7-B placeholder. Server currently ignores this field (no `users.avatar_url` column in v1 DDL). Full persistence is scheduled for R5+. |
| `roles` | array<V7Role> | 否 | Role assignment per V1_INFORMATION_ARCHITECTURE §5.4. DeptAdmin cannot grant SuperAdmin or DeptAdmin; HRAdmin cannot grant SuperAdmin. |
| `managed_departments` | array<string> | 否 | - |
| `managed_teams` | array<string> | 否 | - |
| `team_codes` | array<string> | 否 | R1.7-B placeholder. v1 persists only the single `team` field plus `managed_teams[]`; multi-team membership deferred to R5+. The server accepts but currently ignores this array (no-op). IA §5.4 reference maintained for forward-compat. |
| `primary_team_code` | string | 否 | R1.7-B placeholder. v1 semantically equals the `team` field; server accepts but currently ignores this field (writes must go through `team`). Full persistence of the multi-team model is scheduled for R5+. |

##### 响应体 schema
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

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden (field-level authorization per §5.4 denied, e.g., DeptAdmin granting DeptAdmin or SuperAdmin; TeamLead attempting role change; cross-department mutation without org-move-request) |
| 404 | 见 `error.code` | 见 `deny_code` | User not found |

##### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/users/<id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

#### DELETE 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `204`

无 JSON 响应体或响应体由文件流承载。

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | User not found |

##### curl 示例
```bash
curl -X DELETE https://api.example.com/v1/users/<id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PUT /v1/users/{id}/password

### 简介
支持方法: PUT。

- `PUT`: Managed password reset endpoint. Replaces the target user's local password hash and returns the user record. `DepartmentAdmin` can reset passwords only for users in own department. Existing session tokens are not revoked by this minimal reset operation.

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
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/users/{id}/roles

### 简介
支持方法: POST, PUT。

- `POST`: Add workflow user roles
- `PUT`: Replace workflow user roles

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- `PUT` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### POST 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `roles` | array<V7Role> | 否 | - |

##### 响应体 schema
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
curl -X POST https://api.example.com/v1/users/<id>/roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

#### PUT 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `roles` | array<V7Role> | 否 | - |

##### 响应体 schema
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
curl -X PUT https://api.example.com/v1/users/<id>/roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## DELETE /v1/users/{id}/roles/{role}

### 简介
支持方法: DELETE。

- `DELETE`: Remove one workflow user role

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `DELETE` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `role` | path | enum(Member/SuperAdmin/HRAdmin/OrgAdmin/RoleAdmin/TeamLead/DesignDirector/DesignReviewer/Ops/Designer/Audit_A/Audit_B...) | 是 | - |

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
| 401 | UNAUTHENTICATED | - | 未登录、token 缺失或 token 过期。 |
| 403 | PERMISSION_DENIED | 见接口返回 | 角色、组织范围、字段级授权或流程状态不允许。 |
| 404 | NOT_FOUND | - | 资源不存在或当前用户不可见。 |
| 409 | CONFLICT | 见接口返回 | 状态竞态、重复操作或版本冲突。 |
| 422 | VALIDATION_ERROR | - | 请求参数或业务字段校验失败。 |

### curl 示例
```bash
curl -X DELETE https://api.example.com/v1/users/<id>/roles/<role> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/permission-logs

### 简介
支持方法: GET。

- `GET`: Combined route-access and identity/role-change audit log. Besides route-level authorization decisions, this endpoint also records register, login, login failure, password change, role assignment, and role removal actions. Read access is available to super admins and HR users.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `actor_id` | query | integer | 否 | - |
| `actor_username` | query | string | 否 | - |
| `action_type` | query | string | 否 | - |
| `target_user_id` | query | integer | 否 | - |
| `target_username` | query | string | 否 | - |
| `granted` | query | boolean | 否 | - |
| `method` | query | string | 否 | - |
| `route_path` | query | string | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "id": "...",
      "actor_id": "...",
      "actor_username": "...",
      "actor_source": "..."
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
| `data` | array<PermissionLog> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |

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
curl -X GET https://api.example.com/v1/permission-logs \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/operation-logs

### 简介
支持方法: GET。

- `GET`: Aggregated operation-log query endpoint for frontend integration. Combines recent task events, export-job events, and integration call logs into one timeline-style read model. Product policy is `HRAdmin` and `SuperAdmin`; legacy `Admin` remains accepted as compatibility-only access.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `source` | query | enum(task_event/export_event/integration_call) | 否 | - |
| `event_type` | query | string | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "source": "...",
      "log_id": "...",
      "reference_type": "...",
      "reference_id": "..."
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
| `data` | array<OperationLogEntry> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |

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
curl -X GET https://api.example.com/v1/operation-logs \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/audit-logs

### 简介
支持方法: GET。

- `GET`: Cross-task audit record list for audit log view. Filters by task_no, auditor, action, start/end date. Returns audit_records enriched with task_no and auditor_name. Read access is limited to audit roles and management roles. `DepartmentAdmin` results are filtered to own department. `TeamLead` results are filtered to own team inside own department.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `taskNo` | query | string | 否 | Contains match on task_no |
| `auditor` | query | string | 否 | Contains match on auditor display_name |
| `action` | query | enum(claim/approve/reject/transfer/handover/takeover) | 否 | - |
| `start` | query | string | 否 | YYYY-MM-DD, records with created_at >= start 00:00:00 |
| `end` | query | string | 否 | YYYY-MM-DD, records with created_at <= end 23:59:59 |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "id": "...",
      "task_id": "...",
      "task_no": "...",
      "stage": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<object> | 否 | - |

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
curl -X GET https://api.example.com/v1/audit-logs \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/server-logs

### 简介
支持方法: GET。

- `GET`: Admin-only server log query with filtering (level, keyword, since, until) and masking of sensitive data. 5xx HTTP responses are automatically recorded. Use POST /v1/server-logs/clean to purge old entries.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `level` | query | enum(info/warn/error) | 否 | - |
| `keyword` | query | string | 否 | - |
| `since` | query | string | 否 | - |
| `until` | query | string | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "id": "...",
      "level": "...",
      "msg": "...",
      "details": "..."
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
| `data` | array<ServerLog> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |

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
curl -X GET https://api.example.com/v1/server-logs \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/server-logs/clean

### 简介
支持方法: POST。

- `POST`: Admin-only. Deletes server logs older than the specified hours. Reason is required.

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
| `older_than_hours` | integer | 否 | - |
| `reason` | string | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "deleted": 123
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `deleted` | integer | 否 | - |

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
curl -X POST https://api.example.com/v1/server-logs/clean \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/admin/jst-users

### 简介
支持方法: GET。

- `GET`: Pre-wiring. Query JST company users through Bridge. Admin only.

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

请求体: 无请求体。

### 响应体 schema
成功响应: `200`

无 JSON 响应体或响应体由文件流承载。

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
curl -X GET https://api.example.com/v1/admin/jst-users \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/admin/jst-users/import-preview

### 简介
支持方法: POST。

- `POST`: Pre-wiring. Preview import result without executing. Admin only.

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
| `current_page` | integer | 否 | - |
| `page_size` | integer | 否 | - |
| `write_roles` | boolean | 否 | - |

### 响应体 schema
成功响应: `200`

无 JSON 响应体或响应体由文件流承载。

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
curl -X POST https://api.example.com/v1/admin/jst-users/import-preview \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/admin/jst-users/import

### 简介
支持方法: POST。

- `POST`: Pre-wiring. Import JST users. dry_run=true for preview. Admin only.

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
| `current_page` | integer | 否 | - |
| `page_size` | integer | 否 | - |
| `write_roles` | boolean | 否 | - |
| `dry_run` | boolean | 否 | - |

### 响应体 schema
成功响应: `200`

无 JSON 响应体或响应体由文件流承载。

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
curl -X POST https://api.example.com/v1/admin/jst-users/import \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。
- 角色与访问规则主要供后台管理页使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/users/{id}/activate

### 简介
支持方法: POST。

- `POST`: Enable the target user's account. Source: V1_INFORMATION_ARCHITECTURE §5.3 / §5.4 (`is_active` field). `TeamLead` may activate only members of own team; `DeptAdmin` only users in own department; `HRAdmin` / `SuperAdmin` global.

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
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/users/{id}/deactivate

### 简介
支持方法: POST。

- `POST`: Disable the target user's account. Scope rules identical to `activate`. Source: V1_INFORMATION_ARCHITECTURE §5.3 / §5.4.

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
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

