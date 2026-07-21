# 组织架构

> Revision: V8 current contract (2026-07-20)
> Source: docs/api/openapi.yaml

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

部门、团队、组织选项与组织迁移申请。

## Family 约定

- 组织字段以前端选择器为主，候选值优先来自 `/v1/org/options`。
- 组织迁移申请是流程化操作，不要直接修改受管组织字段绕过审批。
- 本文件覆盖 `7` 个 `/v1` path；同一路径多 method 合并在同一节。

## GET /v1/org/options

### 简介
支持方法: GET。

- `GET`: Returns the backend org master source used by user-management, task org validation, owner-team compatibility bridging, and frontend org-assignment flows. The canonical response shape is top-level `departments[]`, where each department carries a nested `teams[]` array. That nested department tree is authoritative for `PATCH /v1/users/{id}` department/team updates and for org values accepted by task create. By default this endpoint returns only enabled departments and teams for assignment and validation. Authorized organization-master maintenance clients may pass `include_disabled=true` to return disabled departments/teams as well, so the frontend can display, restore, or keep maintaining rows that are no longer assignable. `teams_by_department` remains a deprecated compatibility mirror in v1.8 only; responses that still include it emit `Deprecation: version="v1.8"`. User responses return both `team` and compatibility alias `group` with the same value. Read access requires `access.view` or `access.manage`. Organization names are display values only; authorization is derived from explicit assignments and stable organization IDs. The v1.0 official baseline exposed here is exactly: `人事部` -> `人事管理组`; `运营部` -> `淘系一组`, `淘系二组`, `天猫一组`, `天猫二组`, `拼多多南京组`, `拼多多池州组`; `设计研发部` -> `默认组`; `定制美工部` -> `默认组`; `审核部` -> `普通审核组`, `定制审核组`; `云仓部` -> `默认组`; plus the `未分配` / `未分配池` system bucket. Legacy operations groups 1-7 and legacy compatibility departments are intentionally filtered out by the default `enabled=1` projection.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `include_disabled` | query | boolean | 否 | When `true`, returns disabled departments and teams. This expanded projection requires global `access.manage`. |

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
    "unassigned_pool_enabled": true,
    "configured_assignments": [
      "..."
    ]
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
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
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
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PUT /v1/org/departments/{id}

### 简介
支持方法: PUT, DELETE。

- `PUT`: Updates one backend org department. Supports renaming (`name`) and stopping use (`enabled=false`). When a department is stopped, the department and its child teams disappear from enabled org options, child teams are disabled, and existing assigned users are moved to the system `未分配/未分配池`. The system unassigned department cannot be deleted. Historical task snapshots are not rewritten.
- `DELETE`: Permanently removes one non-system department together with its child teams from backend org master. Existing assigned users are moved to the system `未分配/未分配池` before the org rows are deleted, and managed-department / managed-team scope references to the removed org are cleared. Historical task snapshots are not rewritten. The system `未分配` department is rejected with 400.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PUT` 允许角色: 已登录 / scope-aware。
- `DELETE` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### PUT 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `name` | string | 否 | Rename the department. Must be unique among org departments. Cannot be combined with stopping use of the same department. |
| `enabled` | boolean | 否 | Set to false to stop using the department. Existing assigned users are moved to the system unassigned pool, and child teams are disabled. System unassigned department cannot be deleted. |

##### 响应体 schema
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

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload, duplicate department name, or protected system department |
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |
| 404 | 见 `error.code` | 见 `deny_code` | Department not found |

##### curl 示例
```bash
curl -X PUT https://api.example.com/v1/org/departments/<id> \
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
| 400 | 见 `error.code` | 见 `deny_code` | Protected system department or unassigned pool unavailable |
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |
| 404 | 见 `error.code` | 见 `deny_code` | Department not found |

##### curl 示例
```bash
curl -X DELETE https://api.example.com/v1/org/departments/<id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 组织字段以前端选择器为主，候选值优先来自 `/v1/org/options`。
- 组织迁移申请是流程化操作，不要直接修改受管组织字段绕过审批。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/org/departments/{id}/merge

### 简介
支持方法: POST。

- `POST`: Moves every member (and managed-department scope reference) of the source department into the target department, disables the source department and its teams, then returns the target department. Members whose team does not exist in the target department keep an empty team. The system `未分配` department cannot be merged; the target must be enabled.

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
| `target_department_id` | integer | 是 | Enabled department that receives every member and managed-department scope of the source department. Must differ from the source department. |

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
| 400 | 见 `error.code` | 见 `deny_code` | Missing/identical target, disabled target, or protected system department |
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |
| 404 | 见 `error.code` | 见 `deny_code` | Source or target department not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/org/departments/<id>/merge \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 组织字段以前端选择器为主，候选值优先来自 `/v1/org/options`。
- 组织迁移申请是流程化操作，不要直接修改受管组织字段绕过审批。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
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
| `name` | string | 是 | Unique team name under the selected department in backend org master. |

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
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PUT /v1/org/teams/{id}

### 简介
支持方法: PUT, DELETE。

- `PUT`: Updates one backend org team. Supports renaming (`name`) and stopping use (`enabled=false`). When renaming into a name held only by a disabled, zero-member team in the same department, the stale disabled row is reclaimed and the active team takes that name. When a team is stopped, it disappears from enabled org options and existing assigned users are moved to the system `未分配/未分配池`. The system unassigned pool team cannot be deleted. Historical task snapshots are not rewritten.
- `DELETE`: Permanently removes one non-system team from backend org master. Existing assigned users are moved to the system `未分配/未分配池` before the org row is deleted, and managed-team scope references to the removed team are cleared. Historical task snapshots are not rewritten. The system unassigned pool team is rejected with 400.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PUT` 允许角色: 已登录 / scope-aware。
- `DELETE` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### PUT 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `name` | string | 否 | Rename the team inside its department. Cannot be combined with deleting the same team. |
| `enabled` | boolean | 否 | Set to false to stop using the team. Existing assigned users are moved to the system unassigned pool. System unassigned pool team cannot be deleted. |

##### 响应体 schema
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

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload, duplicate team name under department, or protected system team |
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |
| 404 | 见 `error.code` | 见 `deny_code` | Team not found |

##### curl 示例
```bash
curl -X PUT https://api.example.com/v1/org/teams/<id> \
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
| 400 | 见 `error.code` | 见 `deny_code` | Protected system team or unassigned pool unavailable |
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |
| 404 | 见 `error.code` | 见 `deny_code` | Team not found |

##### curl 示例
```bash
curl -X DELETE https://api.example.com/v1/org/teams/<id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 组织字段以前端选择器为主，候选值优先来自 `/v1/org/options`。
- 组织迁移申请是流程化操作，不要直接修改受管组织字段绕过审批。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/org/teams/{id}/merge

### 简介
支持方法: POST。

- `POST`: Moves every member of the source team into the target team (which may belong to a different department), updates managed-team scope references, disables the source team, then returns the target team. The system unassigned pool team cannot be merged; the target team and its department must be enabled.

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
| `target_team_id` | integer | 是 | Enabled team (under an enabled department) that receives every member of the source team. Must differ from the source team. |

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
| 400 | 见 `error.code` | 见 `deny_code` | Missing/identical target, disabled target, or protected system team |
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |
| 404 | 见 `error.code` | 见 `deny_code` | Source or target team not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/org/teams/<id>/merge \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 组织字段以前端选择器为主，候选值优先来自 `/v1/org/options`。
- 组织迁移申请是流程化操作，不要直接修改受管组织字段绕过审批。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

