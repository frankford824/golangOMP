# 组织架构

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

部门、团队、组织选项与组织迁移申请。

## Family 约定

- 组织字段以前端选择器为主，候选值优先来自 `/v1/org/options`。
- 组织迁移申请是流程化操作，不要直接修改受管组织字段绕过审批。
- 本文件覆盖 `9` 个 `/v1` path；同一路径多 method 合并在同一节。

## GET /v1/org/options

### 简介
支持方法: GET。

- `GET`: Returns the backend org master source used by user-management, task org validation, owner-team compatibility bridging, and frontend org-assignment flows. The canonical response shape is top-level `departments[]`, where each department carries a nested `teams[]` array. That nested department tree is authoritative for `PATCH /v1/users/{id}` department/team updates and for org values accepted by task create. `teams_by_department` remains a deprecated compatibility mirror in v1.8 only; responses that still include it emit `Deprecation: version="v1.8"`. User responses return both `team` and compatibility alias `group` with the same value. Read access is available to company-level managers, department managers, and legacy org/role compatibility admins. The v1.0 official baseline exposed here is exactly: `人事部` -> `人事管理组`; `运营部` -> `淘系一组`, `淘系二组`, `天猫一组`, `天猫二组`, `拼多多南京组`, `拼多多池州组`; `设计研发部` -> `默认组`; `定制美工部` -> `默认组`; `审核部` -> `普通审核组`, `定制审核组`; `云仓部` -> `默认组`; plus the `未分配` / `未分配池` system bucket. Legacy operations groups 1-7 and legacy compatibility departments are intentionally filtered out by the `enabled=1` projection.

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
    ],
    "teams_by_department": {},
    "role_catalog_summary": [
      "..."
    ],
    "unassigned_pool_enabled": true
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | OrgOptions | 否 | Canonical org-options payload. `departments[].teams` is the authoritative shape as of v1.8. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/org/options \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 组织字段以前端选择器为主，候选值优先来自 `/v1/org/options`。
- 组织迁移申请是流程化操作，不要直接修改受管组织字段绕过审批。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/org/departments

### 简介
支持方法: POST。

- `POST`: Creates one enabled department in backend org master. Newly created departments appear in `/v1/org/options` and become valid user/task org values immediately after creation.

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
| `name` | string | 是 | Unique department name in backend org master. |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "name": "string",
    "enabled": true,
    "created_at": "2026-04-25T10:30:41Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | OrgDepartment | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload or department already exists |
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/org/departments \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 组织字段以前端选择器为主，候选值优先来自 `/v1/org/options`。
- 组织迁移申请是流程化操作，不要直接修改受管组织字段绕过审批。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PUT /v1/org/departments/{id}

### 简介
支持方法: PUT。

- `PUT`: Updates one backend org department. Current runtime semantic is enable/disable only; disabling is rejected while users are still assigned or enabled child teams still exist.

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
| `enabled` | boolean | 是 | Current backend update semantic is enable/disable only. |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "name": "string",
    "enabled": true,
    "created_at": "2026-04-25T10:30:41Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | OrgDepartment | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload or department cannot be disabled yet |
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |
| 404 | 见 `error.code` | 见 `deny_code` | Department not found |

### curl 示例
```bash
curl -X PUT https://api.example.com/v1/org/departments/<id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 组织字段以前端选择器为主，候选值优先来自 `/v1/org/options`。
- 组织迁移申请是流程化操作，不要直接修改受管组织字段绕过审批。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/org/teams

### 简介
支持方法: POST。

- `POST`: Creates one enabled team in backend org master under the specified department. Newly created teams appear in `/v1/org/options` and become valid user/task org values immediately after creation.

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
| `department_id` | integer | 否 | Backend org department id. Optional when `department` is provided. |
| `department` | string | 否 | Backend org department name. Optional when `department_id` is provided. |
| `name` | string | 是 | Globally unique team name in backend org master. |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "department_id": 123,
    "department": "string",
    "name": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | OrgTeam | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload, team already exists, or department is invalid |
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/org/teams \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 组织字段以前端选择器为主，候选值优先来自 `/v1/org/options`。
- 组织迁移申请是流程化操作，不要直接修改受管组织字段绕过审批。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PUT /v1/org/teams/{id}

### 简介
支持方法: PUT。

- `PUT`: Updates one backend org team. Current runtime semantic is enable/disable only; disabling is rejected while users are still assigned to that team.

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
| `enabled` | boolean | 是 | Current backend update semantic is enable/disable only. |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "department_id": 123,
    "department": "string",
    "name": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | OrgTeam | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload or team cannot be disabled yet |
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |
| 404 | 见 `error.code` | 见 `deny_code` | Team not found |

### curl 示例
```bash
curl -X PUT https://api.example.com/v1/org/teams/<id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 组织字段以前端选择器为主，候选值优先来自 `/v1/org/options`。
- 组织迁移申请是流程化操作，不要直接修改受管组织字段绕过审批。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/departments/{id}/org-move-requests

### 简介
支持方法: POST。

- `POST`: DeptAdmin in the source department initiates a cross-department user move. The request is created in state `pending_super_admin_confirm`. Source: V1_INFORMATION_ARCHITECTURE §5.2.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | Source department ID. Must match the user's current department and the caller's managed department (for DeptAdmin). |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `user_id` | integer | 是 | The user being moved. Must currently belong to the source department identified in the path. |
| `target_department_id` | integer | 是 | Destination department; nullable means unassigned-pool. |
| `reason` | string | 是 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "user_id": 123,
    "source_department_id": 123,
    "target_department_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | OrgMoveRequest | 否 | Source: V1_INFORMATION_ARCHITECTURE §5.2. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Validation failed |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden (caller is not DeptAdmin of source department, or the target user does not belong to the source department) |
| 404 | 见 `error.code` | 见 `deny_code` | Department or user not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/departments/<id>/org-move-requests \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 组织字段以前端选择器为主，候选值优先来自 `/v1/org/options`。
- 组织迁移申请是流程化操作，不要直接修改受管组织字段绕过审批。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/org-move-requests

### 简介
支持方法: GET。

- `GET`: Lists org-move-requests for the SuperAdmin review queue. Source: V1_INFORMATION_ARCHITECTURE §5.2.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `state` | query | enum(pending_super_admin_confirm/approved/rejected) | 否 | Filter by request state. Omit to list all states. |
| `user_id` | query | integer | 否 | Filter by target user. |
| `source_department_id` | query | integer | 否 | Filter by source department. DeptAdmin callers are server-side forced to own managed department; this parameter is a safe filter hint and not a bypass. |
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
      "user_id": "...",
      "source_department_id": "...",
      "target_department_id": "..."
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
| `data` | array<OrgMoveRequest> | 否 | - |
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
curl -X GET https://api.example.com/v1/org-move-requests \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 组织字段以前端选择器为主，候选值优先来自 `/v1/org/options`。
- 组织迁移申请是流程化操作，不要直接修改受管组织字段绕过审批。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/org-move-requests/{id}/approve

### 简介
支持方法: POST。

- `POST`: SuperAdmin confirms a pending org-move-request. On approval the target user's `department` / `team` are updated and audit event `user_department_changed_by_admin` is emitted. Source: V1_INFORMATION_ARCHITECTURE §5.2.

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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden (caller is not SuperAdmin) |
| 404 | 见 `error.code` | 见 `deny_code` | Request not found or not in `pending_super_admin_confirm` state |
| 409 | 见 `error.code` | 见 `deny_code` | Request already decided (approved or rejected) |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/org-move-requests/<id>/approve \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 组织字段以前端选择器为主，候选值优先来自 `/v1/org/options`。
- 组织迁移申请是流程化操作，不要直接修改受管组织字段绕过审批。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/org-move-requests/{id}/reject

### 简介
支持方法: POST。

- `POST`: SuperAdmin rejects a pending org-move-request with a required reason. The target user remains in the source department. Source: V1_INFORMATION_ARCHITECTURE §5.2.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `reason` | string | 是 | - |

### 响应体 schema
成功响应: `204`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Validation failed (reason missing) |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden (caller is not SuperAdmin) |
| 404 | 见 `error.code` | 见 `deny_code` | Request not found or not in `pending_super_admin_confirm` state |
| 409 | 见 `error.code` | 见 `deny_code` | Request already decided (approved or rejected) |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/org-move-requests/<id>/reject \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 组织字段以前端选择器为主，候选值优先来自 `/v1/org/options`。
- 组织迁移申请是流程化操作，不要直接修改受管组织字段绕过审批。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

