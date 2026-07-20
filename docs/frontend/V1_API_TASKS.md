# 任务主流程

> Revision: V1.3-A2 i_id-first task/ERP/search integration (2026-04-27)
> Source: docs/api/openapi.yaml (post V1.3-A2)

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

任务创建、列表、详情、模块动作、分派、取消、归档与工作流操作。

## Family 约定

- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 本文件覆盖 `238` 个 `/v1` path；同一路径多 method 合并在同一节。

## GET /v1/access/permissions

### 简介
支持方法: GET。

- `GET`: List the code-maintained capability catalog

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
      "code": "...",
      "module": "...",
      "name": "...",
      "description": "...",
      "risk_level": "...",
      "enabled": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<AccessPermission> | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/access/permissions \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/access/roles

### 简介
支持方法: GET, POST。

- `GET`: List administrator-managed business roles
- `POST`: Create a business role

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
| `include_archived` | query | boolean | 否 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "id": "...",
      "code": "...",
      "name": "...",
      "description": "...",
      "system_protected": "...",
      "version": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<AccessRole> | 是 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/access/roles \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `code` | string | 是 | - |
| `name` | string | 是 | - |
| `description` | string | 否 | - |
| `permissions` | array<AccessRolePermission> | 否 | - |
| `reason` | string | 是 | - |
| `expected_policy_revision` | integer | 是 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "policy_revision": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AccessPolicyMutationResult | 是 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 409 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/access/roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/access/roles/{id}

### 简介
支持方法: PATCH。

- `PATCH`: Update role display metadata

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `name` | string | 是 | - |
| `description` | string | 否 | - |
| `expected_version` | integer | 是 | - |
| `reason` | string | 是 | - |
| `expected_policy_revision` | integer | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "policy_revision": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AccessPolicyMutationResult | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 404 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 409 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/access/roles/<id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/access/roles/{id}/archive

### 简介
支持方法: POST。

- `POST`: Archive an in-use role without physical deletion

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
| `expected_version` | integer | 是 | - |
| `reason` | string | 是 | - |
| `expected_policy_revision` | integer | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "policy_revision": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AccessPolicyMutationResult | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 409 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/access/roles/<id>/archive \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PUT /v1/access/roles/{id}/permissions

### 简介
支持方法: PUT。

- `PUT`: Atomically replace a role's capability set

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
| `permissions` | array<AccessRolePermission> | 是 | - |
| `expected_role_version` | integer | 是 | - |
| `reason` | string | 是 | - |
| `expected_policy_revision` | integer | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "policy_revision": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AccessPolicyMutationResult | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 409 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X PUT https://api.example.com/v1/access/roles/<id>/permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/access/users

### 简介
支持方法: GET。

- `GET`: Returns only stable user and organization identifiers plus display labels. The route is governed by `access_policy.view` or `access_policy.manage` and never by legacy administrator roles.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `q` | query | string | 否 | Matches username or display name. |
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
      "username": "...",
      "display_name": "...",
      "department": "..."
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
| `data` | array<AccessPolicyUserOption> | 是 | - |
| `pagination` | PaginationMeta | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/access/users \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PUT /v1/access/users/{id}/assignments

### 简介
支持方法: PUT。

- `PUT`: Atomically replace a user's roles and stable organization-ID scopes

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
| `expected_policy_revision` | integer | 是 | - |
| `reason` | string | 是 | - |
| `assignments` | array<AccessAssignment> | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "policy_revision": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AccessPolicyMutationResult | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 409 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X PUT https://api.example.com/v1/access/users/<id>/assignments \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/access/users/{id}/effective

### 简介
支持方法: GET。

- `GET`: Resolve capabilities, scopes, sources and policy revision

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "user_id": 123,
    "policy_revision": 123,
    "permissions": [
      "..."
    ],
    "assignments": [
      "..."
    ],
    "sources": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | EffectiveAccess | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 404 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/access/users/<id>/effective \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/access/org-policies/{subject_type}/{subject_id}

### 简介
支持方法: GET, PUT。

- `GET`: Read explicitly enabled defaults for an organization ID
- `PUT`: Atomically replace policies for an organization ID

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- `PUT` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `subject_type` | path | enum(department/team) | 是 | - |
| `subject_id` | path | integer | 是 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "subject_type": "...",
      "subject_id": "...",
      "role_id": "...",
      "scope_mode": "...",
      "enabled": "...",
      "version": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<AccessOrgPolicy> | 是 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/access/org-policies/<subject_type>/<subject_id> \
  -H "Authorization: Bearer $TOKEN"
```

#### PUT 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `subject_type` | path | enum(department/team) | 是 | - |
| `subject_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `policies` | array<AccessOrgPolicy> | 是 | - |
| `reason` | string | 是 | - |
| `expected_policy_revision` | integer | 是 | - |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "policy_revision": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AccessPolicyMutationResult | 是 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 409 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

##### curl 示例
```bash
curl -X PUT https://api.example.com/v1/access/org-policies/<subject_type>/<subject_id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/access/preview

### 简介
支持方法: POST。

- `POST`: Preview the current effective-access projection for a user

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
| `user_id` | integer | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "user_id": 123,
    "policy_revision": 123,
    "permissions": [
      "..."
    ],
    "assignments": [
      "..."
    ],
    "sources": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | EffectiveAccess | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/access/preview \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/access/events

### 简介
支持方法: GET。

- `GET`: List access-policy audit events

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `limit` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "id": "...",
      "action": "...",
      "actor_id": "...",
      "reason": "...",
      "policy_revision": "...",
      "created_at": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<AccessPolicyEvent> | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/access/events \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/submit-design

### 简介
支持方法: POST。

- `POST`: Ordinary/customization design tasks submit one source per group plus the designer's single/set decision; final outputs are rejected at this stage. Retouch tasks submit final outputs here and complete directly. Upload-session completion never advances workflow state.

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
| `expected_workflow_revision` | integer | 是 | - |
| `idempotency_key` | string | 是 | - |
| `groups` | array<SubmitResourceGroupInput> | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "task_id": 123,
    "workflow_revision": 123,
    "groups": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ResourceBundle | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 409 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/submit-design \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/audit/decision

### 简介
支持方法: POST。

- `POST`: Approval uploads a complete final set for every resource group using the designer-selected mode. The auditor may replace the source file; otherwise the designer source remains effective. Approval finalizes resources and returns only after the task is `Completed`. Return requires a reason and restores the design stage without accepting a partial final set.

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
| `decision` | enum(approve/return_to_design) | 是 | - |
| `expected_workflow_revision` | integer | 是 | - |
| `idempotency_key` | string | 是 | - |
| `reason` | string | 否 | - |
| `groups` | array<SubmitResourceGroupInput> | 否 | Required and complete for approve; omitted or empty for return_to_design. |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "task_id": 123,
    "workflow_revision": 123,
    "groups": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ResourceBundle | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 409 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/audit/decision \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/audit/handover-candidates

### 简介
支持方法: GET。

- `GET`: List PendingAudit tasks currently handled by the caller and eligible for handover

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `keyword` | query | string | 否 | - |
| `status` | query | enum(PendingAudit) | 否 | - |
| `owner_org_team` | query | string | 否 | Display filter only; authorization uses stable organization IDs. |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "items": [
      "..."
    ],
    "pagination": {
      "page": "...",
      "page_size": "...",
      "total": "..."
    },
    "eligible_count": 123,
    "selected_limit": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AuditHandoverCandidateListResponse | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/audit/handover-candidates \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/audit/handover-batch

### 简介
支持方法: POST。

- `POST`: Hand over caller-owned PendingAudit tasks to an eligible auditor

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
| `mode` | enum(explicit/all_matching) | 是 | - |
| `task_ids` | array<integer> | 否 | Required when `mode=explicit`. |
| `filters` | object | 否 | Candidate filters reused by `mode=all_matching`; the backend recomputes candidates and ignores frontend list state. |
| `to_auditor_id` | integer | 是 | - |
| `reason` | string | 是 | - |
| `current_judgement` | string | 否 | - |
| `risk_remark` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "success_count": 123,
    "failure_count": 123,
    "results": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | BatchAuditHandoverResponse | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 409 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/audit/handover-batch \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/audit/handover

### 简介
支持方法: POST。

- `POST`: The task row is locked and its current handler and workflow revision are revalidated before any handover record or event is written.

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
| `to_auditor_id` | integer | 是 | - |
| `reason` | string | 是 | - |
| `current_judgement` | string | 否 | - |
| `risk_remark` | string | 否 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "handover_no": "string",
    "task_id": 123,
    "from_auditor_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AuditHandover | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 409 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/audit/handover \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/audit/handovers

### 简介
支持方法: GET。

- `GET`: List audit handovers visible in the caller's task scope

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "id": "...",
      "handover_no": "...",
      "task_id": "...",
      "from_auditor_id": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<AuditHandover> | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 404 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/<id>/audit/handovers \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/audit/takeover

### 简介
支持方法: POST。

- `POST`: The task and handover rows are locked and revalidated before the handler, audit record, and event are written.

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
| `handover_id` | integer | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "task_id": 123,
    "handover_id": 123,
    "action": "taken_over"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 409 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/audit/takeover \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/reopen

### 简介
支持方法: POST。

- `POST`: Reopen a completed design or retouch task under optimistic concurrency

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
| `target` | enum(design/audit/retouch) | 是 | - |
| `reason` | string | 是 | - |
| `expected_workflow_revision` | integer | 是 | - |
| `idempotency_key` | string | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "task_id": 123,
    "workflow_revision": 123,
    "groups": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ResourceBundle | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 409 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/reopen \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/resource-bundle

### 简介
支持方法: GET。

- `GET`: Read task, SKU and retouch resource groups

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "task_id": 123,
    "workflow_revision": 123,
    "groups": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ResourceBundle | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 404 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/<id>/resource-bundle \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/resource-groups

### 简介
支持方法: GET。

- `GET`: Default response uses `view_mode=group` (one SKU resource-group card). When `resource_role` or a non-all `format_category` is supplied, the service returns `view_mode=flat` with matching `flat_items`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `task_id` | query | integer | 否 | - |
| `sku_code` | query | string | 否 | - |
| `task_no` | query | string | 否 | - |
| `creator_id` | query | integer | 否 | - |
| `resource_role` | query | enum(reference/source/final) | 否 | - |
| `q` | query | string | 否 | Searches task number |
| `format_category` | query | enum(all/image/design/pdf/document/video/archive) | 否 | document is accepted as an alias of pdf. |
| `business_lane` | query | enum(normal/customization) | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "items": [
      "..."
    ],
    "page": 123,
    "page_size": 123,
    "total": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ResourceGroupListResult | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/resource-groups \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/resource-groups/{id}

### 简介
支持方法: GET。

- `GET`: Read one resource group and its current revisions

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_id": 123,
    "scope_kind": "task",
    "lock_version": 123,
    "migration_incomplete": true,
    "created_at": "2026-04-25T10:30:41Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskAssetGroup | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 404 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/resource-groups/<id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/resource-groups/batch-download

### 简介
支持方法: POST。

- `POST`: Requires asset.download and applies the actor's effective task data scope. This is the default single/set resource download contract, not an export operation.

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
| `group_ids` | array<integer> | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "items": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ResourceGroupBatchDownloadManifest | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/resource-groups/batch-download \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/sku-planning/image-upload-sessions

### 简介
支持方法: POST。

- `POST`: Stage one planning-SKU product image before task creation

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
| `client_create_id` | string | 是 | - |
| `client_item_id` | string | 是 | - |
| `filename` | string | 是 | - |
| `expected_size` | integer | 是 | - |
| `mime_type` | string | 否 | - |
| `file_hash` | string | 否 | - |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "session": {
      "id": "...",
      "task_id": "...",
      "asset_id": "...",
      "asset_type": "..."
    }
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | PlanningSKUImageUploadSessionResult | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/sku-planning/image-upload-sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/sku-planning/image-upload-sessions/{session_id}

### 简介
支持方法: GET。

- `GET`: Read a planning-SKU image staging session

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `session_id` | path | string | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": "string",
    "task_id": 123,
    "asset_id": 123,
    "asset_type": "reference"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | UploadSession | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 404 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/sku-planning/image-upload-sessions/<session_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/sku-planning/image-upload-sessions/{session_id}/complete

### 简介
支持方法: POST。

- `POST`: Complete staging and return an image_upload_ref

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `session_id` | path | string | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `body` | object | 视接口 | OpenAPI 声明的整体对象。 |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {}
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 409 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/sku-planning/image-upload-sessions/<session_id>/complete \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/sku-planning/image-upload-sessions/{session_id}/abort

### 简介
支持方法: POST。

- `POST`: Abort an unbound planning-SKU image staging session

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `session_id` | path | string | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `body` | object | 视接口 | OpenAPI 声明的整体对象。 |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": "string",
    "task_id": 123,
    "asset_id": 123,
    "asset_type": "reference"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | UploadSession | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/sku-planning/image-upload-sessions/<session_id>/abort \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/sku-planning/template.xlsx

### 简介
支持方法: GET。

- `GET`: Download the standard or ERP planning-SKU import template

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `erp` | query | boolean | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`

```json
"string"
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `body` | string | 视接口 | OpenAPI 声明的整体对象。 |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/sku-planning/template.xlsx \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/sku-planning/parse-excel

### 简介
支持方法: POST。

- `POST`: Parse and validate an import workbook without creating a task

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `multipart/form-data`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `file` | string | 是 | - |
| `erp` | boolean | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "planning_sku_items": [
      "..."
    ],
    "errors": [
      "..."
    ],
    "valid": true
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | PlanningSKUExcelParseResult | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/sku-planning/parse-excel \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@example.xlsx"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/tasks/{id}/planning-skus/{item_id}

### 简介
支持方法: PATCH。

- `PATCH`: Create an immutable correction revision for a completed planning SKU

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `item_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `expected_version` | integer | 是 | - |
| `reason` | string | 是 | - |
| `description_spec` | string | 是 | - |
| `quantity` | integer | 是 | - |
| `target_price` | string | 否 | - |
| `note` | string | 否 | - |
| `reference_url` | string | 否 | - |
| `image_upload_ref` | string | 否 | - |
| `remove_image` | boolean | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_sku_item_id": 123,
    "version_no": 123,
    "description_spec": "string",
    "quantity": 123,
    "currency": "CNY"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | PlanningSKURevision | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 409 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/tasks/<id>/planning-skus/<item_id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/planning-skus/export.xlsx

### 简介
支持方法: GET。

- `GET`: Export all planning SKUs for one task

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`

```json
"string"
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `body` | string | 视接口 | OpenAPI 声明的整体对象。 |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/<id>/planning-skus/export.xlsx \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/planning-skus/export.xlsx

### 简介
支持方法: POST。

- `POST`: Export up to 5000 selected planning-SKU rows

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
| `task_ids` | array<integer> | 否 | - |
| `task_sku_item_ids` | array<integer> | 否 | - |

### 响应体 schema
成功响应: `200 application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`

```json
"string"
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `body` | string | 视接口 | OpenAPI 声明的整体对象。 |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/planning-skus/export.xlsx \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/planning-skus/erp-retry

### 简介
支持方法: POST。

- `POST`: Queue retry for failed planning-SKU ERP projections

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
成功响应: `200 application/json`

```json
{
  "data": {
    "queued": 123,
    "resync": false
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/planning-skus/erp-retry \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/planning-skus/erp-resync

### 简介
支持方法: POST。

- `POST`: Explicitly queue ERP overwrite after a completed-SKU correction

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
成功响应: `200 application/json`

```json
{
  "data": {
    "queued": 123,
    "resync": true
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/planning-skus/erp-resync \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/trace-events

### 简介
支持方法: GET, POST。

- `GET`: Query the lightweight full-chain event ledger for business tracing and AI insight use cases. Supports filtering by people, department, task, SKU, asset, ERP/integration call, event source, outcome, trace ID, and occurred time range.
- `POST`: Authenticated frontend endpoint for recording page-view and user-action events. The server enriches the record with session actor, client IP, user agent, and request trace ID.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Admin, SuperAdmin, HRAdmin。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `trace_id` | query | string | 否 | - |
| `event_source` | query | enum(api/frontend/system/integration) | 否 | - |
| `event_type` | query | string | 否 | - |
| `action` | query | string | 否 | - |
| `actor_id` | query | integer | 否 | - |
| `actor_username` | query | string | 否 | Contains match on logged-in username/display name snapshot. |
| `actor_source` | query | enum(session_token/anonymous/header_placeholder/header_roles_placeholder/system_fallback) | 否 | Filter by actor source; business dashboards typically use session_token. |
| `actor_department` | query | string | 否 | - |
| `actor_team` | query | string | 否 | - |
| `route_path` | query | string | 否 | - |
| `task_id` | query | integer | 否 | - |
| `module_key` | query | string | 否 | - |
| `sku_code` | query | string | 否 | - |
| `asset_id` | query | integer | 否 | - |
| `design_asset_id` | query | integer | 否 | - |
| `task_asset_id` | query | integer | 否 | - |
| `integration_call_log_id` | query | integer | 否 | - |
| `resource_type` | query | string | 否 | - |
| `resource_id` | query | string | 否 | - |
| `outcome` | query | enum(succeeded/failed) | 否 | - |
| `business_only` | query | boolean | 否 | Excludes low-value technical traffic such as auth, polling, websocket, and log-center routes. |
| `from` | query | string | 否 | - |
| `since` | query | string | 否 | - |
| `to` | query | string | 否 | - |
| `until` | query | string | 否 | - |
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
      "event_id": "...",
      "trace_id": "...",
      "event_source": "..."
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
| `data` | array<WorkflowTraceEvent> | 否 | - |
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
curl -X GET https://api.example.com/v1/trace-events \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `event_type` | string | 是 | - |
| `action` | string | 否 | - |
| `page_url` | string | 否 | - |
| `page_name` | string | 否 | - |
| `component_id` | string | 否 | - |
| `task_id` | integer | 否 | - |
| `task_module_id` | integer | 否 | - |
| `module_key` | string | 否 | - |
| `sku_code` | string | 否 | - |
| `task_sku_item_id` | integer | 否 | - |
| `asset_id` | integer | 否 | - |
| `design_asset_id` | integer | 否 | - |
| `task_asset_id` | integer | 否 | - |
| `integration_call_log_id` | integer | 否 | - |
| `resource_type` | string | 否 | - |
| `resource_id` | string | 否 | - |
| `outcome` | string | 否 | - |
| `payload` | object | 否 | - |
| `occurred_at` | string | 否 | - |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "event_id": "string",
    "trace_id": "string",
    "event_source": "api"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | WorkflowTraceEvent | 否 | Lightweight business trace event used by the business tracing and AI insight page. |

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
curl -X POST https://api.example.com/v1/trace-events \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/product-management

### 简介
支持方法: GET。

- `GET`: Product-center read model for SKU-to-ERP maintenance. The record includes base data sync state, ERP image sync state, cost trace, and the server-derived area trace used by cost assessment.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, ERP, Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `keyword` | query | string | 否 | - |
| `display_scope` | query | enum(combo/single/all) | 否 | - |
| `image_source` | query | enum(manual/erp_product_image/delivery/derived_preview/task_reference/auto_on_close/missing) | 否 | - |
| `sync_status` | query | enum(pending_sync/queued/syncing/synced/failed/cooling_down/waiting_image) | 否 | - |
| `base_sync_status` | query | enum(pending_sync/queued/syncing/synced/failed/cooling_down/waiting_image) | 否 | - |
| `image_sync_status` | query | enum(pending_sync/queued/syncing/synced/failed/cooling_down/waiting_image) | 否 | - |
| `cost_status` | query | enum(missing/ready) | 否 | - |
| `issue_scope` | query | enum(attention/all) | 否 | - |
| `creator_id` | query | integer | 否 | - |
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
      "record_key": "...",
      "task_id": "...",
      "task_sku_item_id": "..."
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
| `data` | array<ProductManagementRecord> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/product-management \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/product-management/combo-tree

### 简介
支持方法: GET。

- `GET`: Product-center combo hierarchy. This endpoint does not call the ERP OpenWeb API directly; it reads local combo-cache tables and embeds the same product management record contract as `/v1/product-management`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, ERP, Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `keyword` | query | string | 否 | - |
| `display_scope` | query | enum(combo/single/all) | 否 | - |
| `image_source` | query | enum(manual/erp_product_image/delivery/derived_preview/task_reference/auto_on_close/missing) | 否 | - |
| `sync_status` | query | enum(pending_sync/queued/syncing/synced/failed/cooling_down/waiting_image) | 否 | - |
| `base_sync_status` | query | enum(pending_sync/queued/syncing/synced/failed/cooling_down/waiting_image) | 否 | - |
| `image_sync_status` | query | enum(pending_sync/queued/syncing/synced/failed/cooling_down/waiting_image) | 否 | - |
| `cost_status` | query | enum(missing/ready) | 否 | - |
| `issue_scope` | query | enum(attention/all) | 否 | - |
| `creator_id` | query | integer | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "groups": [
    {}
  ],
  "data": [
    {}
  ],
  "pagination": {
    "page": 123,
    "page_size": 123,
    "total": 123
  },
  "combo_sync_summary": {}
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `groups` | array<object> | 否 | Combo or single-SKU groups. Each group embeds its current product-management child records; use `/v1/product-management` for the authoritative child record field contract. |
| `data` | array<object> | 否 | Flattened current-page product-management records, using the same record shape as `/v1/product-management`. |
| `pagination` | PaginationMeta | 否 | - |
| `combo_sync_summary` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/product-management/combo-tree \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/product-management/cost-dashboard

### 简介
支持方法: GET。

- `GET`: Aggregates product-center cost issues into three operator-facing buckets and six fine-grained chips. The dashboard reads the local product-management read model, latest cost snapshots, and latest ERP cost verification trace; it does not mutate costs.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, ERP, Admin, SuperAdmin。
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
    "total_records": 123,
    "total_count": 123,
    "unbound_iid_count": 123,
    "legacy_fallback_ratio": 12.3
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ProductCostDashboardResponse | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/product-management/cost-dashboard \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/product-management/cost-recalculation-runs

### 简介
支持方法: GET, POST。

- `GET`: List product cost recalculation runs
- `POST`: Creates a persistent cost recalculation run. Single mode previews synchronously; bulk modes may start in `previewing` and are polled through the run detail endpoint. The server re-reads product records and enforces the 300 item batch limit.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, ERP, Admin, SuperAdmin。
- `POST` 允许角色: Ops, ERP, Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `status` | query | string | 否 | - |
| `mode` | query | enum(single/explicit/all_matching) | 否 | - |
| `created_by` | query | integer | 否 | - |
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
      "run_no": "...",
      "status": "...",
      "mode": "..."
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
| `data` | array<CostRecalculationRun> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/product-management/cost-recalculation-runs \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `mode` | enum(single/explicit/all_matching) | 是 | - |
| `product_management_record_id` | integer | 否 | Required for single mode unless the first `record_ids` entry is used. |
| `record_ids` | array<integer> | 否 | - |
| `filters` | object | 否 | Server-side selection filter used by all_matching mode. |
| `issue_group` | string | 否 | - |
| `issue_tag` | string | 否 | - |
| `sync_erp` | boolean | 否 | Frontend hint for the quick-fix flow; ERP queueing still requires calling sync-erp after apply. |
| `force_manual` | boolean | 否 | Reserved. Current apply flow skips manual override and manual-quote rows by default. |
| `reason` | string | 否 | - |
| `description` | string | 否 | - |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "run_no": "string",
    "status": "previewing",
    "mode": "single"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CostRecalculationRun | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid selection or batch limit exceeded |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/product-management/cost-recalculation-runs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/product-management/cost-recalculation-runs/{run_id}

### 简介
支持方法: GET。

- `GET`: Get a product cost recalculation run with preview items

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, ERP, Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `run_id` | path | integer | 是 | - |
| `item_status` | query | enum(previewed/applied/skipped/conflict/failed/erp_queued/erp_synced/erp_failed) | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "run_no": "string",
    "status": "previewing",
    "mode": "single"
  },
  "pagination": {
    "page": 123,
    "page_size": 123,
    "total": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CostRecalculationRun | 否 | - |
| `pagination` | PaginationMeta | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Run not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/product-management/cost-recalculation-runs/<run_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/product-management/cost-recalculation-runs/{run_id}/apply

### 简介
支持方法: POST。

- `POST`: Idempotently applies previewed run items. Rows are marked conflict when current cost drifted or another open run owns the same product record.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Ops, ERP, Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `run_id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "run": {
      "id": "...",
      "run_no": "...",
      "status": "...",
      "mode": "..."
    },
    "summary": {
      "total_count": "...",
      "previewed_count": "...",
      "applied_count": "...",
      "skipped_count": "..."
    }
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ApplyCostRecalculationRunResponse | 否 | - |

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
curl -X POST https://api.example.com/v1/product-management/cost-recalculation-runs/<run_id>/apply \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/product-management/cost-recalculation-runs/{run_id}/sync-erp

### 简介
支持方法: POST。

- `POST`: Reuses the existing product-management base sync queue; run items move from `applied` to `erp_queued` and are completed by the base-sync worker callback.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Ops, ERP, Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `run_id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "run": {
      "id": "...",
      "run_no": "...",
      "status": "...",
      "mode": "..."
    },
    "summary": {
      "total_count": "...",
      "previewed_count": "...",
      "applied_count": "...",
      "skipped_count": "..."
    }
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | SyncCostRecalculationRunERPResponse | 否 | - |

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
curl -X POST https://api.example.com/v1/product-management/cost-recalculation-runs/<run_id>/sync-erp \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/product-management/cost-recalculation-runs/{run_id}/cancel

### 简介
支持方法: POST。

- `POST`: Cancel an open product cost recalculation run

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Ops, ERP, Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `run_id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "run_no": "string",
    "status": "previewing",
    "mode": "single"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CostRecalculationRun | 否 | - |

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
curl -X POST https://api.example.com/v1/product-management/cost-recalculation-runs/<run_id>/cancel \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/product-management/{id}/reparse-image

### 简介
支持方法: POST。

- `POST`: Re-runs the backend product-image selection logic for a product-center record and returns the updated read-model row.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, ERP, Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "record_key": "string",
    "task_id": 123,
    "task_sku_item_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ProductManagementRecord | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid record id |
| 404 | 见 `error.code` | 见 `deny_code` | Product management record not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/product-management/<id>/reparse-image \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/product-management/{id}/image

### 简介
支持方法: POST。

- `POST`: Binds an existing task asset as the managed product image and returns the updated read-model row.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, ERP, Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `asset_id` | integer | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "record_key": "string",
    "task_id": 123,
    "task_sku_item_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ProductManagementRecord | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid image payload |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/product-management/<id>/image \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/product-management/{id}/sync-request

### 简介
支持方法: POST。

- `POST`: Queues both base data and image synchronization when applicable, honoring backend cooldown and force rules.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, ERP, Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `force` | boolean | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "record_key": "string",
    "task_id": 123,
    "task_sku_item_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ProductManagementRecord | 否 | - |

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
curl -X POST https://api.example.com/v1/product-management/<id>/sync-request \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/product-management/{id}/base-sync-request

### 简介
支持方法: POST。

- `POST`: Queues only product base data synchronization, including cost price when available.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, ERP, Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `force` | boolean | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "record_key": "string",
    "task_id": 123,
    "task_sku_item_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ProductManagementRecord | 否 | - |

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
curl -X POST https://api.example.com/v1/product-management/<id>/base-sync-request \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/product-management/{id}/image-sync-request

### 简介
支持方法: POST。

- `POST`: Queues only ERP image synchronization for the managed product image.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, ERP, Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `force` | boolean | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "record_key": "string",
    "task_id": 123,
    "task_sku_item_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ProductManagementRecord | 否 | - |

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
curl -X POST https://api.example.com/v1/product-management/<id>/image-sync-request \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/cost-rule-bindings

### 简介
支持方法: GET, POST。

- `GET`: Lists active or inactive mappings from normalized ERP style i_id values to internal cost rule groups. Matching order in the pricing path is ERP i_id, then task product_i_id, then legacy text alias fallback.
- `POST`: Creates one binding. The backend normalizes `i_id_raw`, requires the target rule group to exist as an active `cost_rules.category_code`, and enforces active normalized i_id uniqueness.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, ERP, Admin, SuperAdmin。
- `POST` 允许角色: Ops, ERP, Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `keyword` | query | string | 否 | - |
| `q` | query | string | 否 | - |
| `rule_group` | query | string | 否 | - |
| `is_active` | query | boolean | 否 | - |
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
      "i_id_raw": "...",
      "normalized_i_id": "...",
      "rule_group": "..."
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
| `data` | array<CostRuleBinding> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/cost-rule-bindings \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `i_id_raw` | string | 是 | - |
| `rule_group` | string | 是 | - |
| `display_name` | string | 否 | - |
| `source` | string | 否 | - |
| `is_active` | boolean | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "i_id_raw": "string",
    "normalized_i_id": "string",
    "rule_group": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CostRuleBinding | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid binding payload |
| 409 | 见 `error.code` | 见 `deny_code` | Active normalized i_id already has a binding |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/cost-rule-bindings \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/cost-rule-bindings/unbound-candidates

### 简介
支持方法: GET。

- `GET`: Returns normalized i_id values that currently have no active binding and whose latest cost snapshot was produced by legacy text alias fallback. This powers the "unassociated style" migration list.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, ERP, Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `keyword` | query | string | 否 | - |
| `q` | query | string | 否 | - |
| `limit` | query | integer | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "erp_i_id": "...",
      "product_i_id": "...",
      "normalized_i_id": "...",
      "suggested_rule_group": "..."
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
| `data` | array<UnboundCostRuleCandidate> | 否 | - |
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
curl -X GET https://api.example.com/v1/cost-rule-bindings/unbound-candidates \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/cost-rule-bindings/{id}

### 简介
支持方法: PATCH。

- `PATCH`: Updates a binding. If `i_id_raw` changes, the backend recomputes `normalized_i_id`; active uniqueness and active rule-group validation are enforced again.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: Ops, ERP, Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `i_id_raw` | string | 否 | - |
| `rule_group` | string | 否 | - |
| `display_name` | string | 否 | - |
| `source` | string | 否 | - |
| `is_active` | boolean | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "i_id_raw": "string",
    "normalized_i_id": "string",
    "rule_group": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CostRuleBinding | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid binding payload |
| 404 | 见 `error.code` | 见 `deny_code` | Binding not found |
| 409 | 见 `error.code` | 见 `deny_code` | Active normalized i_id already has a binding |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/cost-rule-bindings/<id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/reference-upload-sessions

### 简介
支持方法: POST。

- `POST`: Canonical pre-task reference upload entry. Returns an OSS direct single-part or multipart plan chosen from the configured part-size threshold; the remote plan is present only as fallback when OSS direct planning is unavailable.

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
| `created_by` | integer | 否 | Deprecated and ignored. The backend always uses the authenticated session actor. |
| `filename` | string | 是 | - |
| `expected_size` | integer | 是 | Required exact file size in bytes. Pre-task reference uploads reject values above 300 MB and verify the final OSS object length. |
| `mime_type` | string | 否 | - |
| `file_hash` | string | 否 | - |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "session": {
      "id": "...",
      "task_id": "...",
      "asset_id": "...",
      "asset_type": "..."
    },
    "remote": {
      "upload_id": "...",
      "file_id": "...",
      "base_url": "...",
      "upload_url": "..."
    },
    "oss_direct": {
      "mode": "...",
      "object_key": "...",
      "expires_at": "...",
      "method": "...",
      "required_upload_content_type": "..."
    },
    "complete_endpoint": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CreateTaskReferenceUploadSessionResponseData | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload |
| 401 | 见 `error.code` | 见 `deny_code` | Authentication required |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/reference-upload-sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/reference-upload-sessions/{session_id}

### 简介
支持方法: GET。

- `GET`: Get task reference upload session

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `session_id` | path | string | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": "string",
    "task_id": 123,
    "asset_id": 123,
    "asset_type": "reference"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | UploadSession | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Upload session not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/reference-upload-sessions/<session_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/reference-upload-sessions/{session_id}/complete

### 简介
支持方法: POST。

- `POST`: Finalizes the returned OSS plan, verifies that the expected object exists and matches the declared size, then returns the normalized `ref_object` for `POST /v1/tasks`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `session_id` | path | string | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `completed_by` | integer | 否 | Deprecated and ignored. The backend always uses the authenticated session actor. |
| `file_hash` | string | 否 | - |
| `upload_content_type` | string | 否 | - |
| `oss_upload_id` | string | 否 | - |
| `oss_object_key` | string | 否 | - |
| `oss_parts` | array<object> | 否 | - |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "session": {
      "id": "...",
      "task_id": "...",
      "asset_id": "...",
      "asset_type": "..."
    },
    "reference_file_ref": "string",
    "storage_ref": {
      "ref_id": "...",
      "asset_id": "...",
      "owner_type": "...",
      "owner_id": "..."
    },
    "ref_object": {
      "asset_id": "...",
      "source": "..."
    }
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CompleteTaskReferenceUploadSessionResponseData | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid upload completion |
| 404 | 见 `error.code` | 见 `deny_code` | Upload session not found |
| 409 | 见 `error.code` | 见 `deny_code` | Upload session already terminal |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/reference-upload-sessions/<session_id>/complete \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/reference-upload-sessions/{session_id}/abort

### 简介
支持方法: POST。

- `POST`: Abort task reference upload session

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `session_id` | path | string | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `cancelled_by` | integer | 否 | Deprecated and ignored. The backend always uses the authenticated session actor. |
| `remark` | string | 否 | - |
| `oss_object_key` | string | 否 | Direct-upload object key returned by the session plan. Used for validated cleanup. |
| `oss_upload_id` | string | 否 | Multipart upload id returned by the session plan. Used to abort unfinished multipart data. |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": "string",
    "task_id": 123,
    "asset_id": 123,
    "asset_type": "reference"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | UploadSession | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Upload session not found |
| 409 | 见 `error.code` | 见 `deny_code` | Upload session is terminal |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/reference-upload-sessions/<session_id>/abort \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/prepare-product-codes

### 简介
支持方法: POST。

- `POST`: Allocates unique default product codes for task-create UIs. Default format is selected by `sku_code_type`: `regular` allocates `CG + {CATEGORY_LETTER} + {6-digit sequence}` and `customization` allocates `DZ + {CATEGORY_LETTER} + {6-digit sequence}`. This endpoint does not require frontend code-rule/template selection and is available only for `new_product_development`. Planning-SKU codes use the dedicated versioned code-rule engine.

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
| `task_type` | enum(new_product_development) | 是 | - |
| `business_lane` | enum(normal/customization) | 否 | Canonical lane selector. Controls default SKU prefix (`CG` for `normal`, `DZ` for `customization`). |
| `workflow_lane` | enum(normal/customization) | 否 | Compatibility alias of `business_lane`. |
| `category_code` | string | 否 | Required when `batch_items` is omitted. |
| `sku_code_type` | enum(regular/customization) | 否 | Automatic SKU code type. `regular` allocates `CG` codes; `customization` allocates `DZ` codes. |
| `count` | integer | 否 | Defaults to 1 when omitted. Used only when `batch_items` is omitted. |
| `batch_items` | array<object> | 否 | If provided, backend allocates one code per item and ignores top-level `count`. |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "codes": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | PrepareTaskProductCodesResponse | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid task type/category/count payload |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/prepare-product-codes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks

### 简介
支持方法: GET, POST。

- `GET`: Returns V8 task rows, stable organization IDs, workflow revision, allowed actions and current resource summaries. Organization names are display-only.
- `POST`: Creates one task under the V8 contract. - `original_product_development` and `new_product_development` enter the unified design workflow. - `retouch_task` completes when all retouch requirements have final products. - `sku_planning` accepts 1-200 `planning_sku_items`, allocates one atomic SKU range and returns only after the task is `Completed`. - task ownership uses stable `owner_department_id` and `owner_team_id`; organization names are display-only. - planning-SKU product images must be staged through the dedicated image-upload-session API and never enter task resource groups.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / 主流程读全量可见。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `status` | query | array<enum(Draft/PendingAssign/Assigned/InProgress/PendingAudit/Completed/Archived/Cancelled/Blocked)> | 否 | - |
| `task_type` | query | array<enum(original_product_development/new_product_development/retouch_task/sku_planning)> | 否 | - |
| `creator_id` | query | integer | 否 | - |
| `designer_id` | query | integer | 否 | - |
| `owner_department_id` | query | integer | 否 | - |
| `owner_team_id` | query | integer | 否 | - |
| `priority` | query | enum(low/normal/high/critical) | 否 | - |
| `overdue` | query | boolean | 否 | - |
| `date_from` | query | string | 否 | - |
| `date_to` | query | string | 否 | - |
| `keyword` | query | string | 否 | - |
| `sort` | query | enum(created_at/-created_at/updated_at/-updated_at/due_at/-due_at/task_no/-task_no) | 否 | - |
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
      "task_no": "...",
      "task_type": "...",
      "task_status": "...",
      "workflow_revision": "...",
      "allowed_actions": "..."
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
| `data` | array<TaskListItem> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `body` | any | 视接口 | OpenAPI 声明的整体对象。 |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {}
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | any | 是 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 403 | 见 `error.code` | 见 `deny_code` | 错误响应。 |
| 409 | 见 `error.code` | 见 `deny_code` | 错误响应。 |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/filter-options

### 简介
支持方法: GET。

- `GET`: Returns task-derived creator and designer display options within the caller's explicit task-view scope.

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
    "creators": [
      "..."
    ],
    "designers": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskFilterOptions | 否 | - |

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
curl -X GET https://api.example.com/v1/tasks/filter-options \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}

### 简介
支持方法: GET。

- `GET`: Returns the V8 task read model with stable organization IDs, workflow revision, backend-derived allowed actions and the resource-group projection. Legacy file-version pointers are not business-resource authority.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / 主流程读全量可见。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {}
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskReadModel | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with `deny_code` such as `task_out_of_department_scope` or `task_out_of_team_scope`. |
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/<id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/predictions

### 简介
支持方法: GET。

- `GET`: Returns deterministic next-action suggestions for a task detail page based on current task status, task modules, task assets, cost, and ERP filing state. This endpoint does not call the AI provider.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `limit` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "suggestions": [
      "..."
    ],
    "generated_at": "2026-04-25T10:30:41Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | PredictionBundle | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid task id |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/<id>/predictions \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/product-info

### 简介
支持方法: GET, PATCH。

- `GET`: Returns task-scoped product/business fields used by frontend product panel.
- `PATCH`: Partial update of task-scoped product fields; omitted fields remain unchanged. This write path now also requires both an allowed role and a matching minimum org scope over canonical task ownership.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, Audit_A, Audit_B, Warehouse, Outsource, Admin。
- `PATCH` 允许角色: Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
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
    "product_id": 123,
    "sku_code": "string",
    "product_name": "string",
    "product_name_snapshot": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

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
curl -X GET https://api.example.com/v1/tasks/<id>/product-info \
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
| `operator_id` | integer | 否 | - |
| `product_name` | string | 否 | Clean product-name edit field. Updates task product snapshot and participates in ERP filing. |
| `product_name_snapshot` | string | 否 | Compatibility alias for `product_name`. |
| `i_id` | string | 否 | Canonical Jushuitan product style/family i_id. Prefer this over legacy category/category_code fields. |
| `product_i_id` | string | 否 | Compatibility alias for `i_id`. |
| `product_selection` | TaskProductSelectionContext | 否 | Full original-product provenance contract for task read and detail views. It extends the lightweight summary with the local matched mapping snapshot and an additive ERP Bridge product snapshot. |
| `category` | string | 否 | - |
| `category_id` | integer | 否 | - |
| `category_code` | string | 否 | - |
| `spec_text` | string | 否 | - |
| `design_requirement` | string | 否 | Editable demand text. For original-product tasks backend maps this as an alias to `change_request`; for new-product and retouch tasks it writes `design_requirement`. |
| `change_request` | string | 否 | Editable original-product change request. For new-product and retouch tasks backend accepts it as a compatibility alias of `design_requirement`. |
| `material` | string | 否 | - |
| `size_text` | string | 否 | - |
| `reference_link` | string | 否 | - |
| `reference_file_refs` | array<ReferenceFileRef> | 否 | - |
| `note` | string | 否 | - |
| `trigger_filing` | boolean | 否 | Optional active sync switch. When true, backend forces ERP filing evaluation immediately; otherwise new-product tasks auto-file on product-info/business-info patch per backend policy. |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_id": 123,
    "demand_text": "string",
    "copy_text": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskDetail | 否 | Task supplemental demand information. Field names follow `domain.TaskDetail` json tags in `domain/task.go`. |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with task-action `deny_code` details when the actor is outside the allowed org scope. |

##### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/tasks/<id>/product-info \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/cost-info

### 简介
支持方法: GET, PATCH。

- `GET`: Returns task-scoped cost fields and governance light metadata.
- `PATCH`: Partial update of task-scoped cost fields; omitted fields remain unchanged. This write path now also requires both an allowed role and a matching minimum org scope over canonical task ownership.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, Audit_A, Audit_B, Warehouse, Outsource, Admin。
- `PATCH` 允许角色: Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
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
    "cost_price": 12.3,
    "estimated_cost": 12.3,
    "cost_rule_id": 123,
    "cost_rule_name": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

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
curl -X GET https://api.example.com/v1/tasks/<id>/cost-info \
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
| `operator_id` | integer | 否 | - |
| `cost_price` | number | 否 | - |
| `cost_rule_id` | integer | 否 | - |
| `cost_rule_name` | string | 否 | - |
| `cost_rule_source` | string | 否 | - |
| `manual_cost_override` | boolean | 否 | - |
| `manual_cost_override_reason` | string | 否 | - |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_id": 123,
    "demand_text": "string",
    "copy_text": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskDetail | 否 | Task supplemental demand information. Field names follow `domain.TaskDetail` json tags in `domain/task.go`. |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with task-action `deny_code` details when the actor is outside the allowed org scope. |

##### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/tasks/<id>/cost-info \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/tasks/{id}/sku-items/{sku_item_id}

### 简介
支持方法: PATCH。

- `PATCH`: Updates row-scoped batch SKU fields such as product name, ERP product i_id, design requirement, and reference images. Supplying or changing `product_i_id` writes it into the row `variant_json` and triggers ERP filing evaluation.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `sku_item_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `operator_id` | integer | 否 | - |
| `product_name` | string | 否 | - |
| `i_id` | string | 否 | - |
| `product_i_id` | string | 否 | - |
| `design_requirement` | string | 否 | - |
| `reference_file_refs` | array<ReferenceFileRef> | 否 | - |
| `trigger_filing` | boolean | 否 | - |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_id": 123,
    "sequence_no": 123,
    "sku_code": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskSKUItem | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with task-action `deny_code` details when the actor is outside the allowed org scope. |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/tasks/<id>/sku-items/<sku_item_id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/tasks/{id}/sku-items/{sku_item_id}/cost-info

### 简介
支持方法: PATCH。

- `PATCH`: Updates one `task_sku_items` cost projection and forces ERP filing so the child SKU uses its own `cost_price` instead of the mother-task cost.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `sku_item_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `operator_id` | integer | 否 | - |
| `cost_price` | number | 否 | - |
| `manual_cost_override` | boolean | 否 | - |
| `manual_cost_override_reason` | string | 否 | - |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_id": 123,
    "sequence_no": 123,
    "sku_code": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskSKUItem | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with task-action `deny_code` details when the actor is outside the allowed org scope. |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/tasks/<id>/sku-items/<sku_item_id>/cost-info \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/cost-quote/preview

### 简介
支持方法: POST。

- `POST`: Runs cost-rule preview using task defaults plus optional request overrides.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Ops, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `operator_id` | integer | 否 | - |
| `category_id` | integer | 否 | - |
| `category_code` | string | 否 | - |
| `width` | number | 否 | - |
| `height` | number | 否 | - |
| `area` | number | 否 | - |
| `quantity` | integer | 否 | - |
| `process` | string | 否 | - |
| `notes` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "matched_rule": {},
    "matched_rule_id": 123,
    "matched_rule_version": 123,
    "applied_rules": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CostRulePreviewResponse | 否 | - |

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
curl -X POST https://api.example.com/v1/tasks/<id>/cost-quote/preview \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/tasks/{id}/business-info

### 简介
支持方法: PATCH。

- `PATCH`: Maintains PRD V2.0 front-loaded category/spec/cost/filed information used by warehouse handoff and close-readiness checks. When category plus minimal width/height/area/quantity/process inputs are present, the backend also triggers skeleton cost preview and persists `estimated_cost`, rule provenance, governed `matched_rule_version`, and manual-review state. Existing-product tasks may also persist or rebind `product_selection` here so the selected product stays traceable back to local mapped-search provenance and optional ERP Bridge external snapshot fields. Filing now uses backend state-machine auto triggers and idempotent payload comparison. Legacy `trigger_filing` and `filed_at` remain compatibility forced triggers. Bridge remains the ERP/JST adapter and mutation executor; MAIN decides business boundary and records filing traces/status. `cost_price` is the current effective internal cost, while `manual_cost_override` distinguishes business-side override from system prefill; `prefill_source`, `prefill_at`, `override_actor`, and `override_at` provide governance trace, and override state changes append a dedicated `cost_override_events` audit record. This remains a narrow filing/master-data boundary only, not a broad ERP docking, approval flow, finance system, procurement/WMS integration, or raw ERP mutation API family on MAIN. Historical tasks are not auto-recomputed by later rule changes; new rule changes affect future preview/prefill only. Procurement preparation is maintained separately via `/v1/tasks/{id}/procurement`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: Ops, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `operator_id` | integer | 否 | Optional override for compatibility. Defaults to the current authenticated actor. |
| `product_name` | string | 否 | Clean product-name edit field. For new-product tasks this updates `tasks.product_name_snapshot` and is included in the next ERP filing payload. |
| `product_name_snapshot` | string | 否 | Compatibility alias for `product_name`. |
| `i_id` | string | 否 | Canonical Jushuitan product style/family i_id used by ERP filing. Prefer this over legacy category fields. |
| `product_i_id` | string | 否 | Compatibility alias for `i_id`. |
| `category` | string | 否 | - |
| `category_id` | integer | 否 | - |
| `category_code` | string | 否 | - |
| `spec_text` | string | 否 | - |
| `material` | string | 否 | - |
| `size_text` | string | 否 | - |
| `design_requirement` | string | 否 | Editable demand text. For `original_product_development`, this is accepted as a compatibility alias and persisted to `change_request`; for `new_product_development` and `retouch_task`, it persists to `design_requirement`. |
| `change_request` | string | 否 | Editable original-product change request. For `new_product_development` and `retouch_task`, this is accepted as a compatibility alias of `design_requirement`. |
| `craft_text` | string | 否 | - |
| `width` | number | 否 | - |
| `height` | number | 否 | - |
| `area` | number | 否 | - |
| `quantity` | integer | 否 | - |
| `process` | string | 否 | - |
| `product_selection` | any | 否 | - |
| `cost_price` | number | 否 | Optional current effective cost. If `manual_cost_override=true`, this becomes the manual override value; otherwise the backend prefers system prefill when available. |
| `cost_rule_id` | integer | 否 | - |
| `cost_rule_name` | string | 否 | - |
| `cost_rule_source` | string | 否 | - |
| `manual_cost_override` | boolean | 否 | Business data flag only. It distinguishes user-entered override from system prefill and is not tied to auth/permissions. |
| `manual_cost_override_reason` | string | 否 | - |
| `trigger_filing` | boolean | 否 | Legacy compatibility switch. Prefer backend auto-policy; this flag forces one filing evaluation. |
| `filed_at` | string | 否 | Legacy compatibility trigger timestamp. Backend maps this to a forced filing evaluation source. |
| `priority` | enum(low/normal/high/critical) | 否 | Task priority. When provided, updates `tasks.priority` without requiring other business-info fields. |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_id": 123,
    "demand_text": "string",
    "copy_text": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskDetail | 否 | Task supplemental demand information. Field names follow `domain.TaskDetail` json tags in `domain/task.go`. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload |
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with task-action `deny_code` details when the actor is outside the allowed org scope. |
| 500 | 见 `error.code` | 见 `deny_code` | Internal error |
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/tasks/<id>/business-info \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/filing-status

### 简介
支持方法: GET。

- `GET`: Returns filing state-machine status, missing fields, and retry hints for frontend display.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Warehouse, Admin, Designer, Audit_A, Audit_B。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "task_id": 123,
    "task_type": "original_product_development",
    "task_status": "Draft",
    "filing_status": "not_filed"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskFilingStatusView | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/<id>/filing-status \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/filing/retry

### 简介
支持方法: POST。

- `POST`: Forces one filing retry attempt using current task payload snapshot and updates filing status fields. This write path now also requires both an allowed role and a matching minimum org scope over canonical task ownership.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `operator_id` | integer | 否 | - |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "task_id": 123,
    "task_type": "original_product_development",
    "task_status": "Draft",
    "filing_status": "not_filed"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskFilingStatusView | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with task-action `deny_code` details when the actor is outside the allowed org scope. |
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/filing/retry \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/detail

### 简介
支持方法: GET。

- `GET`: Returns the task aggregate produced by `task_aggregator.DetailService` fast path. Top-level `data` contains: `task`, nullable `task_detail`, `modules[]`, `events[]` (service caps recent events at 50), `reference_file_refs[]`, `sku_items[]`, and `asset_versions[]`. For batch tasks, `sku_items[]` is present on this detail endpoint so frontend can render per-SKU tabs without a second read. Design upload versions preserve batch scope through `asset_versions[].scope_sku_code`, copied from upload-session `target_sku_code`. Rich snapshot sections such as `procurement_summary`, full top-level `product_selection`, `matched_rule_governance`, `design_assets`, and `governance_audit_summary` are not returned by this endpoint in v1.21; use dedicated routes such as `/v1/tasks/{id}/procurement`, `/v1/tasks/{id}/asset-center/*`, and `/v1/tasks/{id}/cost-overrides` for those read models. Main task-flow aggregate detail is globally visible to task-facing authenticated roles; all mutating actions remain separately action-gated by role, status, handler/assignee, and organization scope.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / 主流程读全量可见。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "task": {
      "id": "...",
      "task_no": "...",
      "task_type": "...",
      "task_status": "...",
      "workflow_revision": "...",
      "allowed_actions": "..."
    },
    "task_detail": {},
    "modules": [
      "..."
    ],
    "events": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskAggregateDetailV2 | 否 | V1.1-A1 fast-path task aggregate detail. Batch tasks include `sku_items`; design uploads include `asset_versions` with `scope_sku_code` for per-SKU grouping. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/<id>/detail \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/cost-overrides

### 简介
支持方法: GET。

- `GET`: Returns the dedicated read-only cost override and governance audit timeline for one task. This timeline records override-specific audit facts such as previous estimated cost, override cost, matched rule and version context, actor and time, and release events. `override_governance_boundary` reuses the same boundary summary object exposed by task, detail, and procurement reads. `task_event_logs` remain the general task event stream and coexist with this governance-specific audit layer. This endpoint is not an approval flow, finance system, accounting contract, or ERP writeback contract.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "task_id": 123,
    "events": [
      "..."
    ],
    "governance_audit_summary": {},
    "override_governance_boundary": {}
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskCostOverrideAuditTimeline | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/<id>/cost-overrides \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/cost-overrides/{event_id}/review

### 简介
支持方法: POST。

- `POST`: Adds or updates the approval-side placeholder boundary for one dedicated `cost_override_events` row. This is a skeleton governance handoff only; it is not a real approval workflow, identity approval chain, or permission model.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Ops, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `event_id` | path | string | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `review_required` | boolean | 否 | - |
| `review_status` | any | 否 | - |
| `review_note` | string | 否 | - |
| `review_actor` | string | 否 | Optional explicit placeholder actor. When omitted, the debug-header actor placeholder may be used. |
| `reviewed_at` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "override_event_id": "string",
    "task_id": 123,
    "review_record_id": 123,
    "finance_record_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskCostOverrideGovernanceBoundary | 否 | Unified ready-for-frontend governance boundary layered above `cost_override_events`. It consolidates approval placeholder, finance placeholder, and latest-action summary reads without introducing a real approval workflow, finance system, or ERP writeback contract. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Task or override event not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/cost-overrides/<event_id>/review \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/cost-overrides/{event_id}/finance-mark

### 简介
支持方法: POST。

- `POST`: Adds or updates the finance-side placeholder boundary for one dedicated `cost_override_events` row. This is a future finance-handoff skeleton only; it is not a real finance system, ledger, reconciliation, settlement, invoice, or ERP writeback interface.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: ERP, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `event_id` | path | string | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `finance_required` | boolean | 否 | - |
| `finance_status` | any | 否 | - |
| `finance_note` | string | 否 | - |
| `finance_marked_by` | string | 否 | Optional explicit placeholder actor. When omitted, the debug-header actor placeholder may be used. |
| `finance_marked_at` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "override_event_id": "string",
    "task_id": 123,
    "review_record_id": 123,
    "finance_record_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskCostOverrideGovernanceBoundary | 否 | Unified ready-for-frontend governance boundary layered above `cost_override_events`. It consolidates approval placeholder, finance placeholder, and latest-action summary reads without introducing a real approval workflow, finance system, or ERP writeback contract. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Task or override event not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/cost-overrides/<event_id>/finance-mark \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/assign

### 简介
支持方法: POST。

- `POST`: Assigns an active Designer to a `PendingAssign` task or reassigns an `InProgress` task. Authorization requires explicit `task.assign` for first assignment or `task.reassign` for reassignment, intersected with the task's stable organization-ID scope; legacy roles and department/team names never grant access. The action is exposed to clients as `task.assign` in the task's `allowed_actions`. `PendingAudit`, `Completed`, `Archived`, `Cancelled`, and `Blocked` are rejected.

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
| `designer_id` | integer | 否 | Designer or customization-operator user id (same field for both lanes). Regular tasks expect an active `Designer` target; customization tasks in `PendingCustomizationProduction` expect an active `CustomizationOperator` target. Omit or send null on a single-task reassign to clear the assignee and return an InProgress task to PendingAssign. |
| `assigned_by` | integer | 否 | Optional override for compatibility. Defaults to the current authenticated actor. |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_no": "string",
    "task_type": "original_product_development",
    "task_status": "Draft",
    "workflow_revision": 123,
    "allowed_actions": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | Task | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` when the exact assign/reassign capability is missing or the task is outside the actor's stable organization-ID scope. |
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |
| 409 | 见 `error.code` | 见 `deny_code` | Task state or workflow revision conflict |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/assign \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/batch/assign

### 简介
支持方法: POST。

- `POST`: Batch assign tasks to designer

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
    "batch_request_id": "string",
    "total": 123,
    "succeeded": 123,
    "failed": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | BatchTaskActionResult | 否 | - |

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
curl -X POST https://api.example.com/v1/tasks/batch/assign \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/batch/remind

### 简介
支持方法: POST。

- `POST`: Batch remind task handlers

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
    "batch_request_id": "string",
    "total": 123,
    "succeeded": 123,
    "failed": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | BatchTaskActionResult | 否 | - |

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
curl -X POST https://api.example.com/v1/tasks/batch/remind \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/assets

### 简介
支持方法: GET。

- `GET`: Canonical task-scoped design-asset list path. Returns the same resource model as `GET /v1/assets?task_id={id}` and keeps task detail pages on one explicit task context route while `/v1/assets` remains the canonical cross-task resource namespace.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "id": "...",
      "task_id": "...",
      "asset_no": "...",
      "source_asset_id": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<DesignAsset> | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/<id>/assets \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/reference-assets/batch-download

### 简介
支持方法: POST。

- `POST`: Return a task-scoped direct-download manifest that aggregates all reference images visible on task detail: - formalized reference assets under task asset center - legacy reference_file_refs persisted on task detail or sku_items The backend does not build ZIP packages; frontend should use JSZip.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `body` | TaskReferenceBatchDownloadRequest | 视接口 | Reserved for future filters. Send `{}` for now. |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "items": [
      "..."
    ],
    "success_count": 123,
    "failure_count": 123,
    "total_size": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskReferenceBatchDownloadManifest | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |
| 500 | 见 `error.code` | 见 `deny_code` | Internal error while building manifest |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/reference-assets/batch-download \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/assets/timeline

### 简介
支持方法: GET。

- `GET`: Returns the append-only task-asset timeline ordered by `version_no ASC`. This is a compatibility-only standalone refresh view, obsolete for frontend rollout, and not the primary design-asset aggregation surface for new frontend upload integration.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "id": "...",
      "task_id": "...",
      "asset_id": "...",
      "asset_type": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<TaskAsset> | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/<id>/assets/timeline \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/assets/{asset_id}/versions

### 简介
支持方法: GET。

- `GET`: Compatibility-only alias for `GET /v1/tasks/{id}/asset-center/assets/{asset_id}/versions`. Obsolete for frontend rollout.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Designer, Ops, Audit_A, Audit_B。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `asset_id` | path | integer | 是 | - |

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
      "asset_id": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<DesignAssetVersion> | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Task or asset not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/<id>/assets/<asset_id>/versions \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/assets/{asset_id}/download

### 简介
支持方法: GET。

- `GET`: Compatibility-only alias for `GET /v1/tasks/{id}/asset-center/assets/{asset_id}/download`. Obsolete for frontend rollout.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Designer, Ops, Audit_A, Audit_B。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `asset_id` | path | integer | 是 | - |
| `X-Network-Probe-Reachable` | header | boolean | 否 | - |
| `X-Network-Probe-Method` | header | string | 否 | - |
| `X-Network-Probe-URL` | header | string | 否 | - |
| `X-Network-Probe-Checked-At` | header | string | 否 | - |
| `X-Network-Probe-Status-Code` | header | integer | 否 | - |
| `X-Network-Probe-Error` | header | string | 否 | - |
| `X-Network-Probe-Attestation` | header | string | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "download_mode": "string",
    "download_url": "string",
    "access_hint": "string",
    "preview_available": true
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AssetDownloadInfo | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Task or asset not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/<id>/assets/<asset_id>/download \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/assets/{asset_id}/versions/{version_id}/download

### 简介
支持方法: GET。

- `GET`: Compatibility-only alias for `GET /v1/tasks/{id}/asset-center/assets/{asset_id}/versions/{version_id}/download`. Obsolete for frontend rollout.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Designer, Ops, Audit_A, Audit_B。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `asset_id` | path | integer | 是 | - |
| `version_id` | path | integer | 是 | - |
| `X-Network-Probe-Reachable` | header | boolean | 否 | - |
| `X-Network-Probe-Method` | header | string | 否 | - |
| `X-Network-Probe-URL` | header | string | 否 | - |
| `X-Network-Probe-Checked-At` | header | string | 否 | - |
| `X-Network-Probe-Status-Code` | header | integer | 否 | - |
| `X-Network-Probe-Error` | header | string | 否 | - |
| `X-Network-Probe-Attestation` | header | string | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "download_mode": "string",
    "download_url": "string",
    "access_hint": "string",
    "preview_available": true
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AssetDownloadInfo | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Task, asset, or version not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/<id>/assets/<asset_id>/versions/<version_id>/download \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/assets/upload-sessions

### 简介
支持方法: POST。

- `POST`: Compatibility-only alias for `POST /v1/assets/upload-sessions`. Obsolete for frontend rollout; new integration must use the top-level asset session contract.

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
| `task_id` | integer | 否 | Required on `POST /v1/assets/upload-sessions`; ignored on task-scoped compatibility routes where task context comes from path. |
| `created_by` | integer | 否 | Deprecated and ignored. The backend always uses the authenticated session actor. |
| `asset_id` | integer | 否 | Existing technical design-asset root when adding another staged version. Completed tasks reject the request and require reopen. |
| `source_asset_id` | integer | 否 | Optional linkage to a source asset. Allowed for `preview` and `design_thumb` intents. |
| `asset_type` | enum(reference/source/delivery/preview/design_thumb) | 否 | Compatibility alias of `asset_kind` retained for migration safety. |
| `asset_kind` | enum(reference/source/delivery/preview/design_thumb) | 否 | Canonical upload intent field for new frontend integrations. |
| `upload_mode` | enum(small/multipart) | 否 | Compatibility-only input. New frontend integrations must not send this field. |
| `filename` | string | 否 | Compatibility alias of `file_name`. At least one of `file_name` or `filename` must be provided. |
| `file_name` | string | 否 | Canonical file name field for new frontend integrations. At least one of `file_name` or `filename` must be provided. |
| `expected_size` | integer | 否 | Required exact file size in bytes. Task asset upload sessions reject values above 1 GiB and completion verifies the OSS object length. |
| `file_size` | integer | 否 | Compatibility alias of `expected_size`; when used it must be the exact positive file size. |
| `mime_type` | string | 否 | Optional MIME hint. |
| `file_hash` | string | 否 | - |
| `remark` | string | 否 | - |
| `reason` | string | 否 | Business reason. Required by the audit post-close supplement upload route. |
| `target_sku_code` | string | 否 | Required for multi-SKU batch-task non-reference uploads. Backend validates that the SKU belongs to the task, returns it on the upload-session business view as `target_sku_code`, and persists the completed asset scope on `scope_sku_code` for the asset root and asset version. |
| `retouch_requirement_id` | integer | 否 | Optional P图需求明细 scope for `retouch_task`. Mutually exclusive with `target_sku_code`. Backend validates ownership and persists the scope on upload session, `design_assets`, and `task_assets`. |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "session": {
      "id": "...",
      "task_id": "...",
      "asset_id": "...",
      "asset_type": "..."
    },
    "remote": {
      "upload_id": "...",
      "file_id": "...",
      "base_url": "...",
      "upload_url": "..."
    },
    "oss_direct": {
      "mode": "...",
      "object_key": "...",
      "expires_at": "...",
      "method": "...",
      "required_upload_content_type": "..."
    },
    "upload_strategy": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CreateTaskAssetUploadSessionResponseData | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload |
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with task-action `deny_code` details when the actor is outside the allowed org scope. |
| 404 | 见 `error.code` | 见 `deny_code` | Task or asset not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/assets/upload-sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/assets/upload-sessions/{session_id}

### 简介
支持方法: GET。

- `GET`: Compatibility-only alias for `GET /v1/assets/upload-sessions/{session_id}`. Obsolete for frontend rollout. Completed and Archived tasks require reopen because the read may synchronize remote session state transactionally.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `session_id` | path | string | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": "string",
    "task_id": 123,
    "asset_id": 123,
    "asset_type": "reference"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | UploadSession | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Task or upload session not found |
| 409 | 见 `error.code` | 见 `deny_code` | Task is Completed/Archived and requires reopen |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/<id>/assets/upload-sessions/<session_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/assets/upload-sessions/{session_id}/complete

### 简介
支持方法: POST。

- `POST`: Compatibility-only alias for `POST /v1/assets/upload-sessions/{session_id}/complete`. Obsolete for frontend rollout.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `session_id` | path | string | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `completed_by` | integer | 否 | Deprecated and ignored. The backend always uses the authenticated session actor. |
| `file_hash` | string | 否 | - |
| `upload_content_type` | string | 否 | Exact `required_upload_content_type` echoed back by the client when finalizing an OSS direct upload. |
| `oss_object_key` | string | 否 | Required for every OSS direct completion. The backend validates that it belongs to this upload session. |
| `oss_upload_id` | string | 否 | Required together with `oss_parts` for multipart completion; omitted for single-part completion. |
| `oss_parts` | array<object> | 否 | Ordered multipart ETags; omitted for single-part completion. |
| `remark` | string | 否 | - |
| `reason` | string | 否 | Optional reason override for audit post-close supplement completion. When omitted, the reason captured during create-session is used. |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "session": {
      "id": "...",
      "task_id": "...",
      "asset_id": "...",
      "asset_type": "..."
    },
    "asset": {
      "id": "...",
      "task_id": "...",
      "asset_no": "...",
      "source_asset_id": "..."
    },
    "version": {
      "id": "...",
      "task_id": "...",
      "task_no": "...",
      "asset_id": "..."
    }
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CompleteTaskAssetUploadSessionResponseData | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload |
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with task-action `deny_code` details when the actor is outside the allowed org scope. |
| 404 | 见 `error.code` | 见 `deny_code` | Task or upload session not found |
| 409 | 见 `error.code` | 见 `deny_code` | Upload session already terminal, asset type mismatch, or Completed task requires reopen |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/assets/upload-sessions/<session_id>/complete \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/assets/upload-sessions/{session_id}/abort

### 简介
支持方法: POST。

- `POST`: Compatibility-only alias for `POST /v1/assets/upload-sessions/{session_id}/cancel`. Obsolete for frontend rollout.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `session_id` | path | string | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `cancelled_by` | integer | 否 | Deprecated and ignored. The backend always uses the authenticated session actor. |
| `remark` | string | 否 | - |
| `oss_object_key` | string | 否 | Direct-upload object key returned by the session plan. Used for validated cleanup. |
| `oss_upload_id` | string | 否 | Multipart upload id returned by the session plan. Used to abort unfinished multipart data. |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": "string",
    "task_id": 123,
    "asset_id": 123,
    "asset_type": "reference"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | UploadSession | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload |
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with task-action `deny_code` details when the actor is outside the allowed org scope. |
| 404 | 见 `error.code` | 见 `deny_code` | Task or upload session not found |
| 409 | 见 `error.code` | 见 `deny_code` | Upload session is terminal, or the task is Completed and requires reopen |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/assets/upload-sessions/<session_id>/abort \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/assets/mock-upload

### 简介
支持方法: POST。

- `POST`: Creates a `task_assets` record without changing task status. Intended for prototype reference or attachment areas. This route can optionally bind a placeholder `upload_request_id` and emit structured `storage_ref` metadata, but it remains mock or placeholder only and is not a stable real-upload contract.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Designer, Ops。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `uploaded_by` | integer | 否 | Optional override for compatibility. Defaults to the current authenticated actor. |
| `asset_type` | enum(reference/source/delivery/preview/design_thumb) | 是 | - |
| `upload_request_id` | string | 否 | - |
| `file_name` | string | 是 | - |
| `mime_type` | string | 否 | - |
| `file_size` | integer | 否 | - |
| `file_path` | string | 否 | - |
| `whole_hash` | string | 否 | - |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "task_id": 123,
    "asset_id": 123,
    "asset_type": "reference"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskAsset | 否 | Task-scoped asset timeline item. Asset semantics are now canonicalized to `reference/source/delivery/preview`, while legacy input aliases remain compatibility-only. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/assets/mock-upload \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/task-board/overview

### 简介
支持方法: GET。

- `GET`: Returns an uncached, database-aggregated snapshot for the main operations dashboard. All counts use the globally visible main task-flow scope. Calendar-day and current-week boundaries use `Asia/Shanghai`. Completion counts and duration prefer explicit `task.closed` or terminal retouch `task.design.submitted` events; legacy completed tasks without a completion event use `updated_at` only as an explicitly measured fallback. The response always contains seven trend days and all five mutually exclusive status buckets.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
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
    "generated_at": "2026-04-25T10:30:41Z",
    "time_zone": "Asia/Shanghai",
    "period_start": "2026-04-25T10:30:41Z",
    "period_end": "2026-04-25T10:30:41Z",
    "health_status": "ok",
    "counts": {
      "total_tasks": "...",
      "active_tasks": "...",
      "design_pending": "...",
      "pending_audit": "...",
      "handover": "...",
      "customization_in_progress": "..."
    }
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskOperationalOverview | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Caller is not a task-facing authenticated role |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/task-board/overview \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/task-board/summary

### 简介
支持方法: GET。

- `GET`: Frontend-ready aggregate entry for role-based workbenches. Returns preset queues with queue identifiers, queue conditions, counts, sample tasks, `normalized_filters`, `/v1/tasks`-ready `query_template` metadata, and lightweight ownership-hint fields built on top of projected `workflow`, task-item `product_selection` summary, and `procurement_summary.coordination_status`. Queue aggregation uses a shared board-level candidate pool and preserves the stable external queue contract. Ownership hints are advisory only and do not introduce enforced queue ownership persistence.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `board_view` | query | enum(all/ops/designer/audit/procurement/warehouse) | 否 | Restricts the response to one role-oriented board. Defaults to `all`. |
| `queue_key` | query | string | 否 | When present, returns only one preset queue inside the board summary. |
| `keyword` | query | string | 否 | - |
| `task_type` | query | array<enum(original_product_development/new_product_development/retouch_task/sku_planning)> | 否 | Applies the same task-list filter semantics as `/v1/tasks`. Supports comma-separated multi-value queries. |
| `source_mode` | query | array<enum(existing_product/new_product)> | 否 | Applies the same task-list filter semantics as `/v1/tasks`. Supports comma-separated multi-value queries. |
| `status` | query | array<string> | 否 | Applies the same task-list filter semantics as `/v1/tasks`. Supports comma-separated multi-value queries. |
| `main_status` | query | array<TaskMainStatus> | 否 | Applies the same task-list filter semantics as `/v1/tasks`. Supports comma-separated multi-value queries. |
| `sub_status_code` | query | array<TaskSubStatusCode> | 否 | Applies the same task-list filter semantics as `/v1/tasks`. Supports comma-separated multi-value queries. |
| `sub_status_scope` | query | enum(design/audit/procurement/warehouse/customization/outsource/production) | 否 | Applies the same task-list filter semantics as `/v1/tasks`. |
| `coordination_status` | query | array<ProcurementCoordinationStatus> | 否 | Applies the same task-list filter semantics as `/v1/tasks`. Supports comma-separated multi-value queries. |
| `creator_id` | query | integer | 否 | - |
| `designer_id` | query | integer | 否 | - |
| `need_outsource` | query | boolean | 否 | - |
| `overdue` | query | boolean | 否 | - |
| `warehouse_prepare_ready` | query | boolean | 否 | - |
| `warehouse_receive_ready` | query | boolean | 否 | - |
| `warehouse_blocking_reason_code` | query | array<string> | 否 | Applies the same task-list filter semantics as `/v1/tasks`. Supports comma-separated multi-value queries. |
| `preview_size` | query | integer | 否 | Number of sample tasks per queue. Defaults to `3`, max `10`. |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "board_view": "all",
    "board_name": "string",
    "generated_at": "2026-04-25T10:30:41Z",
    "filters_schema": {
      "board_views": "...",
      "supported_global_filters": "...",
      "queue_condition_fields": "...",
      "task_list_endpoint": "..."
    }
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskBoardSummary | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid board query |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/task-board/summary \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/task-board/queues

### 简介
支持方法: GET。

- `GET`: Frontend-ready aggregate queue endpoint. Returns preset queues with queue conditions, total counts, paginated task lists, `normalized_filters`, `/v1/tasks`-ready `query_template` metadata, and lightweight ownership-hint fields so workbenches can render inbox or task-board columns directly and drill into list view without rebuilding queue logic. Task items in these queues carry the same `product_selection` summary used by `/v1/tasks`, while detail endpoints keep the full provenance object. Queue aggregation uses a shared board-level candidate pool and preserves the stable external queue contract. Ownership hints are advisory only and do not introduce enforced queue ownership persistence.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `board_view` | query | enum(all/ops/designer/audit/procurement/warehouse) | 否 | Restricts the response to one role-oriented board. Defaults to `all`. |
| `queue_key` | query | string | 否 | When present, returns only one preset queue. |
| `keyword` | query | string | 否 | - |
| `task_type` | query | array<enum(original_product_development/new_product_development/retouch_task/sku_planning)> | 否 | Applies the same task-list filter semantics as `/v1/tasks`. Supports comma-separated multi-value queries. |
| `source_mode` | query | array<enum(existing_product/new_product)> | 否 | Applies the same task-list filter semantics as `/v1/tasks`. Supports comma-separated multi-value queries. |
| `status` | query | array<string> | 否 | Applies the same task-list filter semantics as `/v1/tasks`. Supports comma-separated multi-value queries. |
| `main_status` | query | array<TaskMainStatus> | 否 | Applies the same task-list filter semantics as `/v1/tasks`. Supports comma-separated multi-value queries. |
| `sub_status_code` | query | array<TaskSubStatusCode> | 否 | Applies the same task-list filter semantics as `/v1/tasks`. Supports comma-separated multi-value queries. |
| `sub_status_scope` | query | enum(design/audit/procurement/warehouse/customization/outsource/production) | 否 | Applies the same task-list filter semantics as `/v1/tasks`. |
| `coordination_status` | query | array<ProcurementCoordinationStatus> | 否 | Applies the same task-list filter semantics as `/v1/tasks`. Supports comma-separated multi-value queries. |
| `creator_id` | query | integer | 否 | - |
| `designer_id` | query | integer | 否 | - |
| `need_outsource` | query | boolean | 否 | - |
| `overdue` | query | boolean | 否 | - |
| `warehouse_prepare_ready` | query | boolean | 否 | - |
| `warehouse_receive_ready` | query | boolean | 否 | - |
| `warehouse_blocking_reason_code` | query | array<string> | 否 | Applies the same task-list filter semantics as `/v1/tasks`. Supports comma-separated multi-value queries. |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "board_view": "all",
    "board_name": "string",
    "generated_at": "2026-04-25T10:30:41Z",
    "filters_schema": {
      "board_views": "...",
      "supported_global_filters": "...",
      "queue_condition_fields": "...",
      "task_list_endpoint": "..."
    }
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskBoardQueuesResponse | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid board query |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/task-board/queues \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/workbench/preferences

### 简介
支持方法: GET, PATCH。

- `GET`: Returns user-scoped saved workbench preferences plus frontend bootstrap config for preset queues. This frontend-ready route now requires a bearer session.
- `PATCH`: Saves lightweight workbench preferences for the current session-backed user. This persists queue/default-filter/page-size/sort hints only and does not introduce full inbox ownership persistence.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Admin。
- `PATCH` 允许角色: Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Admin。
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
    "actor": {
      "id": "...",
      "username": "...",
      "roles": "...",
      "source": "..."
    },
    "preferences": {
      "default_queue_key": "...",
      "pinned_queue_keys": "...",
      "default_filters": "...",
      "default_page_size": "..."
    },
    "workbench_config": {
      "filters_schema": "...",
      "supported_sorts": "...",
      "supported_page_sizes": "...",
      "queues": "..."
    }
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | WorkbenchPreferencesEnvelope | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Session-backed user required |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/workbench/preferences \
  -H "Authorization: Bearer $TOKEN"
```

#### PATCH 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `default_queue_key` | string | 否 | - |
| `pinned_queue_keys` | array<string> | 否 | - |
| `default_filters` | TaskQueryTemplate | 否 | Direct board-to-list query template for `/v1/tasks`. Multi-value fields use comma-separated values. |
| `default_page_size` | enum(0/10/20/50/100) | 否 | - |
| `default_sort` | enum(/updated_at_desc) | 否 | - |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "actor": {
      "id": "...",
      "username": "...",
      "roles": "...",
      "source": "..."
    },
    "preferences": {
      "default_queue_key": "...",
      "pinned_queue_keys": "...",
      "default_filters": "...",
      "default_page_size": "..."
    },
    "workbench_config": {
      "filters_schema": "...",
      "supported_sorts": "...",
      "supported_page_sizes": "...",
      "queues": "..."
    }
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | WorkbenchPreferencesEnvelope | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid preference payload |
| 401 | 见 `error.code` | 见 `deny_code` | Session-backed user required |

##### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/workbench/preferences \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/export-templates

### 简介
支持方法: GET。

- `GET`: Returns the static export-template catalog for the current export-center skeleton. These templates only describe placeholder export intent over stable read models; they do not imply a real template engine or file-generation pipeline.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, Audit_A, Audit_B, Warehouse, Admin。
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
      "key": "...",
      "name": "...",
      "description": "...",
      "export_type": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<ExportTemplate> | 否 | - |

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
curl -X GET https://api.example.com/v1/export-templates \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/integration/external-assets/events

### 简介
支持方法: POST。

- `POST`: Dedicated machine-to-machine entry for the NAS watcher. The backend validates every path against configured NAS event roots, applies upsert/delete transitions idempotently by external origin identity and source fingerprint, queues required originals for OSS, and wakes the upload worker immediately. Duplicate `event_id` values inside one batch are ignored. Periodic full reconciliation remains the recovery path for missed filesystem events.

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
| `agent_id` | string | 是 | - |
| `events` | array<ExternalAssetFilesystemEvent> | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "received": 123,
    "applied": 123,
    "duplicates": 123,
    "upserted": 123,
    "deleted": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExternalAssetFilesystemEventResult | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid batch, event type, file metadata, mount, or origin root |
| 401 | 见 `error.code` | 见 `deny_code` | Missing or invalid dedicated machine token |
| 409 | 见 `error.code` | 见 `deny_code` | External asset synchronization is disabled |
| 500 | 见 `error.code` | 见 `deny_code` | Event application or OSS queueing failed |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/integration/external-assets/events \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/integration/connectors

### 简介
支持方法: GET。

- `GET`: Returns the static connector catalog for the current integration-center boundary. Most connectors remain placeholder-only. `erp_bridge_product_upsert` represents the narrow task business-info filing trace.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Admin, ERP。
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
      "key": "...",
      "name": "...",
      "description": "...",
      "direction": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<IntegrationConnector> | 否 | - |

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
curl -X GET https://api.example.com/v1/integration/connectors \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/integration/call-logs

### 简介
支持方法: GET, POST。

- `GET`: Returns internal integration call logs plus latest execution summaries for troubleshooting. The payload exposes `retry_count`, `replay_count`, latest retry or replay action summaries, and separate retryability or replayability reasons so retry and replay remain distinguishable on the same execution boundary. This route also serves narrow ERP filing traces; admins can filter task filing traces with `connector_key=erp_bridge_product_upsert` and `resource_type=task_erp_filing`.
- `POST`: Persists one internal integration call log as the business/request envelope above later execution attempts. This is still mainly a placeholder/internal troubleshooting surface; it does not provide a general ERP executor, retry queue, or callback platform.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Admin, ERP。
- `POST` 允许角色: Admin, ERP。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `connector_key` | query | enum(erp_product_stub/erp_bridge_product_upsert/export_adapter_bridge) | 否 | - |
| `status` | query | enum(queued/sent/succeeded/failed/cancelled) | 否 | - |
| `resource_type` | query | string | 否 | - |
| `resource_id` | query | integer | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "call_log_id": "...",
      "connector_key": "...",
      "operation_key": "...",
      "direction": "..."
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
| `data` | array<IntegrationCallLog> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid integration call log query |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/integration/call-logs \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `connector_key` | enum(erp_product_stub/erp_bridge_product_upsert/export_adapter_bridge) | 是 | - |
| `operation_key` | string | 是 | - |
| `direction` | enum(outbound/inbound) | 是 | - |
| `resource_type` | string | 否 | - |
| `resource_id` | integer | 否 | - |
| `request_payload` | any | 否 | - |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "call_log_id": 123,
    "connector_key": "erp_product_stub",
    "operation_key": "string",
    "direction": "outbound"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | IntegrationCallLog | 否 | Internal integration call log. It records the request envelope above execution attempts, exposes retry/replay admission hints, and is also used for the narrow ERP Bridge product-filing trace under connector `erp_bridge_product_upsert`. |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid integration call log payload |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/integration/call-logs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/integration/call-logs/{id}

### 简介
支持方法: GET。

- `GET`: Returns one internal integration call log record with request/response payload snapshots, layered lifecycle timestamps, latest execution summary, separate retry/replay admission hints, latest retry/replay action summaries, and shared adapter/handoff summaries. This remains an internal trace surface, not a general integration execution platform.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Admin, ERP。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "call_log_id": 123,
    "connector_key": "erp_product_stub",
    "operation_key": "string",
    "direction": "outbound"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | IntegrationCallLog | 否 | Internal integration call log. It records the request envelope above execution attempts, exposes retry/replay admission hints, and is also used for the narrow ERP Bridge product-filing trace under connector `erp_bridge_product_upsert`. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Integration call log not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/integration/call-logs/<id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/integration/call-logs/{id}/executions

### 简介
支持方法: GET, POST。

- `GET`: Internal or admin placeholder execution inspection route. Returns execution attempts beneath one call log so request-envelope lifecycle and execution lifecycle stay visibly separate. Each execution record includes the shared adapter and handoff summaries used in export and storage. This is not a real external worker timeline, callback stream, or retry queue.
- `POST`: Internal/admin placeholder execution-start boundary beneath one call log. This formalizes a manual execution attempt without introducing a real ERP/HTTP/SDK executor, callback processor, or async platform.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Admin, ERP。
- `POST` 允许角色: Admin, ERP。
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
  "data": [
    {
      "execution_id": "...",
      "call_log_id": "...",
      "connector_key": "...",
      "execution_no": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<IntegrationExecution> | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Integration call log not found |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/integration/call-logs/<id>/executions \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `execution_mode` | enum(manual_placeholder_adapter) | 否 | - |
| `trigger_source` | string | 否 | - |
| `adapter_note` | string | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "execution_id": "string",
    "call_log_id": 123,
    "connector_key": "erp_product_stub",
    "execution_no": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | IntegrationExecution | 否 | One placeholder integration execution attempt beneath one call log. `action_type` distinguishes manual start, retry, replay, and compatibility actions on the same execution boundary. This is not an external worker, callback stream, or retry platform. |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid integration execution payload or invalid call-log state |
| 404 | 见 `error.code` | 见 `deny_code` | Integration call log not found |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/integration/call-logs/<id>/executions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/integration/call-logs/{id}/retry

### 简介
支持方法: POST。

- `POST`: Internal/admin placeholder retry route. `retry` is allowed only when the latest visible outcome is a retryable failed execution and creates a new execution attempt beneath the same call log. It does not introduce a real retry scheduler, queue, callback, or external executor.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Admin, ERP。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `execution_mode` | enum(manual_placeholder_adapter) | 否 | - |
| `adapter_note` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "call_log_id": 123,
    "connector_key": "erp_product_stub",
    "operation_key": "string",
    "direction": "outbound"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | IntegrationCallLog | 否 | Internal integration call log. It records the request envelope above execution attempts, exposes retry/replay admission hints, and is also used for the narrow ERP Bridge product-filing trace under connector `erp_bridge_product_upsert`. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid integration retry payload or invalid call-log state |
| 404 | 见 `error.code` | 见 `deny_code` | Integration call log not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/integration/call-logs/<id>/retry \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/integration/call-logs/{id}/replay

### 简介
支持方法: POST。

- `POST`: Internal or admin placeholder replay route. `replay` re-drives the existing call-log envelope through a new execution attempt for troubleshooting or controlled redelivery semantics, including previously succeeded or cancelled logs. This is not a real external replay engine.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Admin, ERP。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `execution_mode` | enum(manual_placeholder_adapter) | 否 | - |
| `adapter_note` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "call_log_id": 123,
    "connector_key": "erp_product_stub",
    "operation_key": "string",
    "direction": "outbound"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | IntegrationCallLog | 否 | Internal integration call log. It records the request envelope above execution attempts, exposes retry/replay admission hints, and is also used for the narrow ERP Bridge product-filing trace under connector `erp_bridge_product_upsert`. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid integration replay payload or invalid call-log state |
| 404 | 见 `error.code` | 见 `deny_code` | Integration call log not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/integration/call-logs/<id>/replay \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/integration/call-logs/{id}/executions/{execution_id}/advance

### 简介
支持方法: POST。

- `POST`: Internal/admin placeholder execution-state advancement route. This advances one persisted execution through `prepared|dispatched|received|completed|failed|cancelled` while synchronizing the parent call-log lifecycle summary. It still does not introduce a real external executor, callback processor, or retry scheduler.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Admin, ERP。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `execution_id` | path | string | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `status` | enum(prepared/dispatched/received/completed/failed/cancelled) | 是 | - |
| `response_payload` | any | 否 | - |
| `error_message` | string | 否 | - |
| `adapter_note` | string | 否 | - |
| `retryable` | boolean | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "execution_id": "string",
    "call_log_id": 123,
    "connector_key": "erp_product_stub",
    "execution_no": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | IntegrationExecution | 否 | One placeholder integration execution attempt beneath one call log. `action_type` distinguishes manual start, retry, replay, and compatibility actions on the same execution boundary. This is not an external worker, callback stream, or retry platform. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid integration execution payload or transition |
| 404 | 见 `error.code` | 见 `deny_code` | Integration execution not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/integration/call-logs/<id>/executions/<execution_id>/advance \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/integration/call-logs/{id}/advance

### 简介
支持方法: POST。

- `POST`: Backward-compatible internal or admin call-log lifecycle advancement route. `queued` requeues the parent call log directly, while `sent`, `succeeded`, `failed`, and `cancelled` reuse the explicit execution boundary so call-log lifecycle and execution lifecycle remain layered. This route does not introduce a real integration worker, callback, or retry engine. It is compatibility-only and should not be treated as the preferred execution API.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Admin, ERP。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `status` | enum(queued/sent/succeeded/failed/cancelled) | 是 | - |
| `response_payload` | any | 否 | - |
| `error_message` | string | 否 | - |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "call_log_id": 123,
    "connector_key": "erp_product_stub",
    "operation_key": "string",
    "direction": "outbound"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | IntegrationCallLog | 否 | Internal integration call log. It records the request envelope above execution attempts, exposes retry/replay admission hints, and is also used for the narrow ERP Bridge product-filing trace under connector `erp_bridge_product_upsert`. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid integration call log payload or transition |
| 404 | 见 `error.code` | 见 `deny_code` | Integration call log not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/integration/call-logs/<id>/advance \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/export-jobs

### 简介
支持方法: GET, POST。

- `GET`: Returns persisted export jobs for the current export-center skeleton. List items expose lifecycle read fields such as `progress_hint`, `latest_status_at`, `download_ready`, `can_start`, `can_attempt`, `can_retry`, `can_dispatch`, `can_redispatch`, admission reason fields (`can_*_reason`, `dispatchability_reason`, `attemptability_reason`, `latest_admission_decision`), `start_mode`, `execution_mode`, `adapter_mode`, `dispatch_mode`, `storage_mode`, `delivery_mode`, `execution_boundary`, `storage_boundary`, `delivery_boundary`, `is_expired`, and `can_refresh`, plus shared `adapter_ref_summary`, `resource_ref_summary`, and `handoff_ref_summary`, placeholder dispatch visibility through `dispatch_count` and `latest_dispatch`, placeholder execution-attempt visibility through `attempt_count` and `latest_attempt`, and lightweight audit summaries through `event_count`, `latest_event`, `latest_dispatch_event`, and `latest_runner_event`. `result_ref` remains placeholder handoff metadata only.
- `POST`: Persists a minimal export job over an existing stable read model. This endpoint does not generate a real file yet; it only records export intent, source filters, initial `queued` status, and structured placeholder download-handoff metadata in `result_ref`. For task-query-derived exports, frontend should pass the current `query_template` and can optionally include `normalized_filters` from task-board handoff payloads.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, Audit_A, Audit_B, Warehouse, Admin。
- `POST` 允许角色: Ops, Designer, Audit_A, Audit_B, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `status` | query | enum(queued/running/ready/failed/cancelled) | 否 | - |
| `source_query_type` | query | enum(task_query/task_board_queue/procurement_summary/warehouse_receipts) | 否 | - |
| `requested_by_id` | query | integer | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "export_job_id": "...",
      "template_key": "...",
      "export_type": "...",
      "source_query_type": "..."
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
| `data` | array<ExportJob> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid export job query |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/export-jobs \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `export_type` | enum(task_list/task_board_queue/procurement_summary/warehouse_receipts) | 是 | - |
| `template_key` | string | 否 | Optional static template key. When omitted, the backend chooses the default skeleton template for the selected `export_type`. |
| `source_query_type` | enum(task_query/task_board_queue/procurement_summary/warehouse_receipts) | 是 | - |
| `source_filters` | ExportSourceFilters | 否 | - |
| `normalized_filters` | TaskQueryFilterDefinition | 否 | Shared board/list filter contract. Queue `normalized_filters` map directly to `/v1/tasks` query semantics. |
| `query_template` | TaskQueryTemplate | 否 | Direct board-to-list query template for `/v1/tasks`. Multi-value fields use comma-separated values. |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "export_job_id": 123,
    "template_key": "string",
    "export_type": "task_list",
    "source_query_type": "task_query"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExportJob | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid export job payload |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/export-jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/export-jobs/{id}

### 简介
支持方法: GET。

- `GET`: Returns one persisted export job skeleton with full placeholder download-handoff metadata plus lightweight lifecycle-audit summaries. Detail payloads also expose `can_start`, `can_attempt`, `can_retry`, `can_dispatch`, `can_redispatch`, admission reason fields (`can_*_reason`, `dispatchability_reason`, `attemptability_reason`, `latest_admission_decision`), `start_mode`, `execution_mode`, `adapter_mode`, `dispatch_mode`, `storage_mode`, `delivery_mode`, `adapter_ref_summary`, `resource_ref_summary`, `handoff_ref_summary`, `execution_boundary`, `storage_boundary`, `delivery_boundary`, `dispatch_count`, `latest_dispatch`, `attempt_count`, `latest_attempt`, `latest_dispatch_event`, `latest_runner_event`, `is_expired`, and `can_refresh` so frontend or internal tools can distinguish export-job lifecycle from dispatch handoff state, placeholder execution-attempt state, placeholder storage representation, and placeholder delivery handoff state. `result_ref` is not a real file location, signed URL, or storage integration.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, Audit_A, Audit_B, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "export_job_id": 123,
    "template_key": "string",
    "export_type": "task_list",
    "source_query_type": "task_query"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExportJob | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Export job not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/export-jobs/<id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/export-jobs/{id}/dispatches

### 简介
支持方法: GET, POST。

- `GET`: Internal/admin placeholder adapter-dispatch inspection route for export jobs. This endpoint returns persisted dispatch handoff records such as trigger source, adapter key, submitted / received / rejected / expired / not-executed status, additive dispatch-level start-admission hints (`start_admissible`, `start_admission_reason`), and placeholder notes so the dispatch boundary is explicit without pretending a real scheduler queue or worker platform exists.
- `POST`: Internal/admin placeholder adapter-dispatch submit boundary for queued export jobs. This route persists one explicit dispatch handoff and appends `export_job.dispatch_submitted` audit context without creating a real scheduler queue item, worker lease, or background execution. Submission admission is now explicitly surfaced on export-job read models through `can_dispatch` and `can_dispatch_reason`; only one unresolved submitted/received dispatch is allowed at a time.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Admin。
- `POST` 允许角色: Admin。
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
  "data": [
    {
      "dispatch_id": "...",
      "export_job_id": "...",
      "dispatch_no": "...",
      "trigger_source": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<ExportJobDispatch> | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Export job not found |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/export-jobs/<id>/dispatches \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `trigger_source` | string | 否 | Optional placeholder handoff source. Defaults to a manual internal dispatch source. |
| `expires_at` | string | 否 | Optional placeholder dispatch expiry timestamp. |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "dispatch_id": "string",
    "export_job_id": 123,
    "dispatch_no": 123,
    "trigger_source": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExportJobDispatch | 否 | One placeholder adapter-dispatch handoff for an export job. This is not a real scheduler queue item, lease, worker callback, or distributed delivery contract; it only makes the dispatch boundary explicit ahead of any future real runner platform. |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid dispatch payload or invalid dispatch state |
| 404 | 见 `error.code` | 见 `deny_code` | Export job not found |
| 409 | 见 `error.code` | 见 `deny_code` | Export job is not in a dispatchable queued state |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/export-jobs/<id>/dispatches \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/export-jobs/{id}/dispatches/{dispatch_id}/advance

### 简介
支持方法: POST。

- `POST`: Internal/admin placeholder dispatch-state advancement route. This endpoint advances one persisted dispatch handoff to `received`, `rejected`, `expired`, or `not_executed` without introducing a real scheduler callback or worker lifecycle. Dispatch state stays separate from both export-job lifecycle and execution-attempt lifecycle.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `dispatch_id` | path | string | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `action` | enum(receive/reject/expire/mark_not_executed) | 是 | - |
| `reason` | string | 否 | - |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "dispatch_id": "string",
    "export_job_id": 123,
    "dispatch_no": 123,
    "trigger_source": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExportJobDispatch | 否 | One placeholder adapter-dispatch handoff for an export job. This is not a real scheduler queue item, lease, worker callback, or distributed delivery contract; it only makes the dispatch boundary explicit ahead of any future real runner platform. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid dispatch payload or invalid dispatch transition |
| 404 | 见 `error.code` | 见 `deny_code` | Export job or dispatch not found |
| 409 | 见 `error.code` | 见 `deny_code` | Export job dispatch is not in an advanceable placeholder state |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/export-jobs/<id>/dispatches/<dispatch_id>/advance \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/export-jobs/{id}/attempts

### 简介
支持方法: GET。

- `GET`: Internal/admin placeholder execution-attempt inspection route for export jobs. This endpoint returns persisted attempt records such as trigger source, execution mode, adapter key, and terminal attempt status, plus additive attempt-level admission hints (`blocks_new_attempt`, `next_attempt_admission_reason`) so current placeholder runner-adapter boundary behavior is visible without pretending a real scheduler, worker lease, or distributed runner platform exists.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "attempt_id": "...",
      "export_job_id": "...",
      "dispatch_id": "...",
      "attempt_no": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<ExportJobAttempt> | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Export job not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/export-jobs/<id>/attempts \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/export-jobs/{id}/events

### 简介
支持方法: GET。

- `GET`: Returns the export-job lifecycle audit timeline ordered oldest to newest. Event payload is audit context only and must not be interpreted as a full runner log stream or proof of real file generation/download delivery.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, Audit_A, Audit_B, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {}
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<ExportJobEvent> | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Export job not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/export-jobs/<id>/events \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/export-jobs/{id}/claim-download

### 简介
支持方法: POST。

- `POST`: Claims placeholder download handoff for a ready export job. This does not start a real file transfer and does not return file bytes; it records a handoff-claim audit event and returns structured placeholder handoff metadata for frontend consumption. This action is allowed only when the export job is `ready` and the current placeholder handoff is not expired. Expired ready handoff returns a placeholder-expired invalid-state response and requires refresh.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Ops, Designer, Audit_A, Audit_B, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "export_job_id": 123,
    "status": "queued",
    "download_ready": true,
    "claim_available": true
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExportJobDownloadHandoff | 否 | Structured placeholder download-handoff response for ready export jobs. This is not a real file-download service and does not return bytes, signed URLs, NAS paths, or object-storage handles. `is_expired` and `can_refresh` describe placeholder handoff lifecycle only. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Export job not found |
| 409 | 见 `error.code` | 见 `deny_code` | Export job is not in a claimable placeholder-download state, including expired ready handoff |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/export-jobs/<id>/claim-download \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/export-jobs/{id}/download

### 简介
支持方法: GET。

- `GET`: Reads structured placeholder download handoff metadata for a ready export job. This endpoint is the current read boundary only: it does not return real file bytes, signed URLs, NAS paths, or object-storage references. A `download_read` audit event is appended to the existing export-job event chain each time this handoff metadata is read. This action is allowed only when the export job is `ready` and the current placeholder handoff is not expired. Expired ready handoff requires refresh.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, Audit_A, Audit_B, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "export_job_id": 123,
    "status": "queued",
    "download_ready": true,
    "claim_available": true
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExportJobDownloadHandoff | 否 | Structured placeholder download-handoff response for ready export jobs. This is not a real file-download service and does not return bytes, signed URLs, NAS paths, or object-storage handles. `is_expired` and `can_refresh` describe placeholder handoff lifecycle only. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Export job not found |
| 409 | 见 `error.code` | 见 `deny_code` | Export job is not in a readable placeholder-download state, including expired ready handoff |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/export-jobs/<id>/download \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/export-jobs/{id}/refresh-download

### 简介
支持方法: POST。

- `POST`: Refreshes expired placeholder download handoff for a ready export job. Refresh rotates the placeholder `result_ref.ref_key`, extends `expires_at`, appends `result_ref_updated` and `download_refreshed` audit events, and returns refreshed handoff metadata. This endpoint is placeholder-only and does not mint signed URLs, return file bytes, re-run export generation, or connect to NAS/object storage.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Ops, Designer, Audit_A, Audit_B, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "export_job_id": 123,
    "status": "queued",
    "download_ready": true,
    "claim_available": true
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExportJobDownloadHandoff | 否 | Structured placeholder download-handoff response for ready export jobs. This is not a real file-download service and does not return bytes, signed URLs, NAS paths, or object-storage handles. `is_expired` and `can_refresh` describe placeholder handoff lifecycle only. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Export job not found |
| 409 | 见 `error.code` | 见 `deny_code` | Export job is not in a refreshable placeholder-download state |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/export-jobs/<id>/refresh-download \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/export-jobs/{id}/start

### 简介
支持方法: POST。

- `POST`: Internal/admin placeholder runner-initiation boundary for export jobs. This route formalizes the `queued -> running` start contract without introducing a real async runner, scheduler, file generator, NAS integration, or object storage. It is allowed only when the current export job status is `queued`, and a latest `submitted` dispatch blocks start until it is received or otherwise resolved. Admission reasons are exposed through `can_start_reason` and `can_attempt_reason`. Successful start creates or consumes one placeholder dispatch handoff: if latest dispatch is `received`, start consumes it; if no startable dispatch exists, start may auto-create one placeholder submitted and received dispatch when no startable dispatch exists before creating the new attempt. This remains a skeleton only and does not imply a real scheduler or asynchronous dispatch platform.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "export_job_id": 123,
    "template_key": "string",
    "export_type": "task_list",
    "source_query_type": "task_query"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExportJob | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Export job not found |
| 409 | 见 `error.code` | 见 `deny_code` | Export job is not in a startable queued state |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/export-jobs/<id>/start \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/export-jobs/{id}/advance

### 简介
支持方法: POST。

- `POST`: Internal or admin skeleton endpoint for manually advancing export-job lifecycle state. This endpoint updates placeholder lifecycle, execution-attempt visibility, and download-handoff metadata while writing audit-trace events. `action=start` remains available for compatibility, but `POST /v1/export-jobs/{id}/start` is the preferred explicit placeholder runner-initiation boundary.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `action` | enum(start/mark_ready/fail/cancel/requeue) | 是 | - |
| `result_file_name` | string | 否 | Optional placeholder handoff file name override used when `action=mark_ready`. |
| `result_mime_type` | string | 否 | Optional placeholder MIME type override used when `action=mark_ready`. |
| `expires_at` | string | 否 | Optional placeholder download-handoff expiry used when `action=mark_ready`. |
| `failure_reason` | string | 否 | Optional failure note used when `action=fail`. |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "export_job_id": 123,
    "template_key": "string",
    "export_type": "task_list",
    "source_query_type": "task_query"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExportJob | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid advance payload or lifecycle transition |
| 404 | 见 `error.code` | 见 `deny_code` | Export job not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/export-jobs/<id>/advance \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/events

### 简介
支持方法: GET。

- `GET`: List task events

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "id": "...",
      "task_id": "...",
      "sequence": "...",
      "event_type": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<TaskEvent> | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/<id>/events \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/code-rules

### 简介
支持方法: GET。

- `GET`: List code rules

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Admin。
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
      "id": "...",
      "rule_type": "...",
      "rule_name": "...",
      "prefix": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<CodeRule> | 否 | - |

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
curl -X GET https://api.example.com/v1/code-rules \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/code-rules/{id}/preview

### 简介
支持方法: GET。

- `GET`: Preview generated code

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "rule_id": 123,
    "preview": "string",
    "is_preview": true
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CodePreview | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Rule not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/code-rules/<id>/preview \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/code-rules/generate-sku

### 简介
支持方法: POST。

- `POST`: Archived. Legacy CodeRule new_sku generation is disabled. Use POST /v1/tasks/prepare-product-codes or task creation default product-code allocation instead.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Ops, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `rule_id` | integer | 是 | - |

### 响应体 schema
成功响应: `见 OpenAPI responses`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Legacy new_sku CodeRule is archived |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/code-rules/generate-sku \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/sku/preview_code

### 简介
支持方法: POST。

- `POST`: [V6] Preview SKU code

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

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
curl -X POST https://api.example.com/v1/sku/preview_code \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/sku/list

### 简介
支持方法: GET。

- `GET`: [V6] List SKUs

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
      "id": "...",
      "sku_code": "...",
      "name": "...",
      "workflow_status": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<SKU> | 否 | - |

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
curl -X GET https://api.example.com/v1/sku/list \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/sku

### 简介
支持方法: POST。

- `POST`: [V6] Create SKU

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

请求体: 无请求体。

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "sku_code": "string",
    "name": "string",
    "workflow_status": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | SKU | 否 | - |

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
curl -X POST https://api.example.com/v1/sku \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/sku/{id}

### 简介
支持方法: GET。

- `GET`: [V6] Get SKU by ID

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "sku_code": "string",
    "name": "string",
    "workflow_status": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | SKU | 否 | - |

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
curl -X GET https://api.example.com/v1/sku/<id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/sku/{id}/sync_status

### 简介
支持方法: GET。

- `GET`: [V6] Frontend sequence-gap recovery

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `since_sequence` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "sku": {
      "id": "...",
      "sku_code": "...",
      "name": "...",
      "workflow_status": "..."
    },
    "latest_sequence": 123,
    "events": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | SKUSyncStatusResult | 否 | - |

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
curl -X GET https://api.example.com/v1/sku/<id>/sync_status \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/agent/sync

### 简介
支持方法: POST。

- `POST`: [V6] NAS agent sync

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
    "asset_version_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AgentSyncResult | 否 | - |

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
curl -X POST https://api.example.com/v1/agent/sync \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/agent/pull_job

### 简介
支持方法: POST。

- `POST`: [V6] Agent pull job

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
    "attempt_id": 123,
    "job": {},
    "lease_expires_at": "2026-04-25T10:30:41Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | PullJobResult | 否 | - |

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
curl -X POST https://api.example.com/v1/agent/pull_job \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/agent/heartbeat

### 简介
支持方法: POST。

- `POST`: [V6] Agent heartbeat

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
    "lease_expires_at": "2026-04-25T10:30:41Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | HeartbeatResult | 否 | - |

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
curl -X POST https://api.example.com/v1/agent/heartbeat \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/agent/ack_job

### 简介
支持方法: POST。

- `POST`: [V6] Agent ack job

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

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
curl -X POST https://api.example.com/v1/agent/ack_job \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/incidents

### 简介
支持方法: GET。

- `GET`: [V6] List incidents

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
      "id": "...",
      "sku_id": "...",
      "job_id": "...",
      "status": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<Incident> | 否 | - |

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
curl -X GET https://api.example.com/v1/incidents \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/incidents/{id}/assign

### 简介
支持方法: POST。

- `POST`: [V6] Assign incident

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
| `assignee_id` | integer | 是 | - |
| `reason` | string | 是 | - |

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
curl -X POST https://api.example.com/v1/incidents/<id>/assign \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/incidents/{id}/resolve

### 简介
支持方法: POST。

- `POST`: [V6] Resolve incident

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
curl -X POST https://api.example.com/v1/incidents/<id>/resolve \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/policies

### 简介
支持方法: GET。

- `GET`: [V6] List policies

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
      "id": "...",
      "key": "...",
      "value": "...",
      "version": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<SystemPolicy> | 否 | - |

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
curl -X GET https://api.example.com/v1/policies \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PUT /v1/policies/{id}

### 简介
支持方法: PUT。

- `PUT`: [V6] Update policy

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
| `value` | string | 是 | - |
| `reason` | string | 是 | - |

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
curl -X PUT https://api.example.com/v1/policies/<id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/rule-templates

### 简介
支持方法: GET。

- `GET`: [V6] List rule templates

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
      "id": "...",
      "template_type": "...",
      "config_json": "...",
      "created_at": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<RuleTemplate> | 否 | - |

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
curl -X GET https://api.example.com/v1/rule-templates \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/rule-templates/{type}

### 简介
支持方法: GET, PUT。

- `GET`: Compatibility endpoint. `type=cost-pricing` is deprecated; product cost governance uses `/v1/cost-rule-bindings` and `/v1/cost-rules` instead.
- `PUT`: Compatibility endpoint. `type=cost-pricing` is deprecated; product cost governance uses `/v1/cost-rule-bindings` and `/v1/cost-rules` instead.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- `PUT` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `type` | path | enum(cost-pricing/product-code/short-name) | 是 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "template_type": "cost-pricing",
    "config_json": "string",
    "created_at": "2026-04-25T10:30:41Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | RuleTemplate | 否 | - |

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
curl -X GET https://api.example.com/v1/rule-templates/<type> \
  -H "Authorization: Bearer $TOKEN"
```

#### PUT 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `type` | path | enum(cost-pricing/product-code/short-name) | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `body` | string | 视接口 | OpenAPI 声明的整体对象。 |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "template_type": "cost-pricing",
    "config_json": "string",
    "created_at": "2026-04-25T10:30:41Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | RuleTemplate | 否 | - |

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
curl -X PUT https://api.example.com/v1/rule-templates/<type> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/pool

### 简介
支持方法: GET。

- `GET`: Lists R3 module-pool entries generated from `task_modules` rows in `pending_claim` state. This is a module claim pool, not the generic assignment/unassigned task list; use `GET /v1/tasks` filters for `PendingAssign` / unassigned-pool task assignment views. Response `data` is always an array; empty pools return `[]`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `module_key` | query | string | 否 | - |
| `pool_team_code` | query | string | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |
| `limit` | query | integer | 否 | Compatibility offset-pagination size. Prefer `page_size`. |
| `offset` | query | integer | 否 | Compatibility offset. Prefer `page`. |
| `sort` | query | enum(created_at/-created_at/updated_at/-updated_at) | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "task_id": "...",
      "module_key": "...",
      "pool_team_code": "...",
      "priority": "...",
      "created_at": "...",
      "updated_at": "..."
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
| `data` | array<object> | 是 | - |
| `pagination` | PaginationMeta | 是 | - |

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
curl -X GET https://api.example.com/v1/tasks/pool \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/modules/{module_key}/claim

### 简介
支持方法: POST。

- `POST`: Claim a task module

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `module_key` | path | string | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `见 OpenAPI responses`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 4XX | 见 `error.code` | 见 `deny_code` | Module action denied |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/modules/<module_key>/claim \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/modules/{module_key}/actions/{action}

### 简介
支持方法: POST。

- `POST`: Requires one of the code-owned capabilities `task.upload_source` or `task.audit`; the service then applies the exact module/state/scope rule. For a customization `submit`, the caller must have `task.upload_source` in the task's stable organization scope. The action marks the internal customization job `ready_for_submit` but does not advance the main task; only `/submit-design` can enter `PendingAudit`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `module_key` | path | string | 是 | - |
| `action` | path | enum(claim/submit/approve/reject/reassign/pool_reassign/asset_upload_session_create/update_reference_files/update_basic_info/update_deadline/update_priority/close_task...) | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `见 OpenAPI responses`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | Missing effective capability or outside the stable task scope |
| 409 | 见 `error.code` | 见 `deny_code` | Module, task, or customization readiness state changed |
| 4XX | 见 `error.code` | 见 `deny_code` | Module action denied |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/modules/<module_key>/actions/<action> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/modules/{module_key}/reassign

### 简介
支持方法: POST。

- `POST`: Reassign a task module within team scope

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `module_key` | path | string | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `见 OpenAPI responses`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 4XX | 见 `error.code` | 见 `deny_code` | Reassign denied |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/modules/<module_key>/reassign \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/modules/{module_key}/pool-reassign

### 简介
支持方法: POST。

- `POST`: Reassign a task module between pools

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `module_key` | path | string | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `见 OpenAPI responses`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 4XX | 见 `error.code` | 见 `deny_code` | Pool reassign denied |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/modules/<module_key>/pool-reassign \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/cancel

### 简介
支持方法: POST。

- `POST`: Cancel or close a task

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
| `force` | boolean | 否 | - |

### 响应体 schema
成功响应: `见 OpenAPI responses`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 4XX | 见 `error.code` | 见 `deny_code` | Cancel denied |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/cancel \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/excel-assist/template.xlsx

### 简介
支持方法: GET。

- `GET`: Downloads the Excel assist workbook for creating one task at a time with `mode=single`. `task_type=new_product_development` columns: `产品款式编码`, `产品名称`, `设计要求` (required); optional `规格尺寸`, `材质`, `材质备注`, `备注`. `task_type=original_product_development` columns: `SKU编码`, `修改要求` (required); optional `规格尺寸`, `备注`. Product name and category are enriched from ERP during `parse-excel`, not collected in the template. The workbook has no sample data rows; `parse-excel` rejects more than one non-empty data row.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `task_type` | query | enum(new_product_development/original_product_development) | 是 | - |
| `mode` | query | enum(single) | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`

```json
"string"
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `body` | string | 视接口 | OpenAPI 声明的整体对象。 |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid task_type or mode |
| 401 | 见 `error.code` | 见 `deny_code` | Authentication required |
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/excel-assist/template.xlsx \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/excel-assist/parse-excel

### 简介
支持方法: POST。

- `POST`: Parses a single-task Excel assist upload into a `draft` plus row-level `violations`. Does not create tasks. `mode` must be `single`. For `new_product_development`, required columns: `产品款式编码`, `产品名称`, `设计要求`. For `original_product_development`, required: `SKU编码`, `修改要求`; optional `规格尺寸`, `备注`. Parsed `sku_code` values are resolved through ERP product search; unknown SKU returns `product_not_found`. More than one non-empty data row returns `multiple_rows_not_allowed`. Invalid quantity returns `invalid_quantity`. Parsed `product_i_id` values for new-product development are validated against ERP i_id options when configured.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `multipart/form-data`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `task_type` | enum(new_product_development/original_product_development) | 是 | - |
| `mode` | enum(single) | 是 | - |
| `file` | string | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "task_type": "new_product_development",
    "mode": "single",
    "draft": {
      "product_i_id": "...",
      "product_name": "...",
      "design_requirement": "...",
      "sku_code": "..."
    },
    "violations": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExcelAssistParseResult | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid upload or parse error |
| 401 | 见 `error.code` | 见 `deny_code` | Authentication required |
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |
| 413 | 见 `error.code` | 见 `deny_code` | File too large |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/excel-assist/parse-excel \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@example.xlsx"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/ai/chat/config

### 简介
支持方法: GET。

- `GET`: Get the current data-assistant capability contract

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
    "hybrid_search_enabled": true,
    "max_input_chars": 123,
    "retention_days": 123,
    "max_concurrent_user": 123,
    "can_review_all": true
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AIChatConfig | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Missing report.view |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/ai/chat/config \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/ai/chat/conversations

### 简介
支持方法: GET, POST。

- `GET`: List the caller's active conversations
- `POST`: Create an owner-scoped conversation retained for 90 days

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
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "items": [
      "..."
    ],
    "total": 123,
    "page": 123,
    "page_size": 123,
    "total_pages": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AIConversationList | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Missing report.view |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/ai/chat/conversations \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `title` | string | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": "string",
    "owner_user_id": 123,
    "title": "string",
    "status": "active",
    "lock_version": 123,
    "expires_at": "2026-04-25T10:30:41Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AIConversation | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | Missing report.view |
| 503 | 见 `error.code` | 见 `deny_code` | Data assistant disabled |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/ai/chat/conversations \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/ai/chat/conversations/{conversation_id}

### 简介
支持方法: GET, DELETE。

- `GET`: Read one owner-scoped conversation and its evidence citations
- `DELETE`: Hide a conversation immediately and hard-delete its body within 24 hours

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- `DELETE` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `conversation_id` | path | string | 是 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": "string",
    "owner_user_id": 123,
    "title": "string",
    "status": "active",
    "lock_version": 123,
    "expires_at": "2026-04-25T10:30:41Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AIConversation | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Not found or not owned by caller |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/ai/chat/conversations/<conversation_id> \
  -H "Authorization: Bearer $TOKEN"
```

#### DELETE 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `conversation_id` | path | string | 是 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `204`

无 JSON 响应体或响应体由文件流承载。

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Not found or not owned by caller |

##### curl 示例
```bash
curl -X DELETE https://api.example.com/v1/ai/chat/conversations/<conversation_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/ai/chat/conversations/{conversation_id}/messages:stream

### 简介
支持方法: POST。

- `POST`: Emits `meta`, `status`, `retrieval`, `delta`, `done`, and `error` SSE events with a heartbeat at least every 15 seconds. The client_message_id is an idempotency key within the conversation. Cancellation persists any partial answer as cancelled.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `conversation_id` | path | string | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `client_message_id` | string | 是 | - |
| `content` | string | 是 | - |

### 响应体 schema
成功响应: `200 text/event-stream`

```json
"string"
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `body` | string | 视接口 | OpenAPI 声明的整体对象。 |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 409 | 见 `error.code` | 见 `deny_code` | Identical client message is still streaming |
| 429 | 见 `error.code` | 见 `deny_code` | Global or per-user stream concurrency limit reached |
| 503 | 见 `error.code` | 见 `deny_code` | Provider |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/ai/chat/conversations/<conversation_id>/messages:stream \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/ai/chat/admin/conversations

### 简介
支持方法: GET。

- `GET`: List conversation metadata across users

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `owner_user_id` | query | integer | 否 | - |
| `status` | query | enum(active/deleted) | 否 | - |
| `from` | query | string | 否 | - |
| `to` | query | string | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "items": [
      "..."
    ],
    "total": 123,
    "page": 123,
    "page_size": 123,
    "total_pages": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AIConversationList | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | Only a protected SuperAdmin may review all conversations |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/ai/chat/admin/conversations \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/ai/chat/admin/conversations/{conversation_id}

### 简介
支持方法: GET。

- `GET`: Review one cross-user conversation and write a metadata-only audit event

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `conversation_id` | path | string | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": "string",
    "owner_user_id": 123,
    "title": "string",
    "status": "active",
    "lock_version": 123,
    "expires_at": "2026-04-25T10:30:41Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AIConversation | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | Only a protected SuperAdmin may review all conversations |
| 404 | 见 `error.code` | 见 `deny_code` | Conversation not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/ai/chat/admin/conversations/<conversation_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/predictions/search

### 简介
支持方法: GET。

- `GET`: Returns deterministic suggestions for the global search overlay. Empty `q` returns recent personal workflow trace suggestions; non-empty `q` returns task / asset / product suggestions. This endpoint uses existing OMP data only and does not call the AI provider.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `q` | query | string | 否 | Optional keyword. When omitted, returns recent personal suggestions. |
| `scope` | query | enum(all/tasks/assets/products/users) | 否 | - |
| `limit` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "suggestions": [
      "..."
    ],
    "generated_at": "2026-04-25T10:30:41Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | PredictionBundle | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/predictions/search \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/predictions/task-create

### 简介
支持方法: GET。

- `GET`: Returns deterministic form-fill suggestions from historical task detail fields. This endpoint is lightweight and does not call the AI provider.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `keyword` | query | string | 否 | Optional form context keyword. Alias `q` is also accepted by the backend. |
| `q` | query | string | 否 | Compatibility alias for `keyword`. |
| `task_type` | query | string | 否 | Optional task type filter. |
| `limit` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "suggestions": [
      "..."
    ],
    "generated_at": "2026-04-25T10:30:41Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | PredictionBundle | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/predictions/task-create \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/predictions/assets

### 简介
支持方法: GET。

- `GET`: Returns deterministic asset suggestions sorted by asset usable state and recency. This endpoint uses task asset records only and does not call the AI provider.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `q` | query | string | 否 | Optional keyword. Alias `keyword` is also accepted by the backend. |
| `keyword` | query | string | 否 | Compatibility alias for `q`. |
| `limit` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "suggestions": [
      "..."
    ],
    "generated_at": "2026-04-25T10:30:41Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | PredictionBundle | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/predictions/assets \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/predictions/management

### 简介
支持方法: GET。

- `GET`: Returns deterministic management attention points for the KPI/data-center page. This endpoint does not call the AI provider; it aggregates tasks, task details, and task assets.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Admin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `from` | query | string | 否 | Start date, inclusive. Defaults to seven days before now. |
| `to` | query | string | 否 | End date, inclusive. Defaults to now. |
| `limit` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "suggestions": [
      "..."
    ],
    "generated_at": "2026-04-25T10:30:41Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | PredictionBundle | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid date range |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/predictions/management \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/experience/config

### 简介
支持方法: GET。

- `GET`: Returns the current feature flags for the stable-first experience learning half-loop.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: SuperAdmin。
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
    "ui_enabled": true,
    "capture_enabled": true,
    "ai_feedback_enabled": true,
    "worker_enabled": true,
    "behavior_capture_enabled": true,
    "micro_question_enabled": true
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExperienceRuntimeFlags | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/experience/config \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/experience/client-config

### 简介
支持方法: GET。

- `GET`: Login-user-readable configuration for lightweight feedback, behavior capture, and micro-question UI. This response is a separate DTO from the SuperAdmin runtime config.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
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
    "ai_feedback_enabled": true,
    "behavior_capture_enabled": true,
    "micro_question_enabled": true,
    "behavior_sample_rate": 12.3,
    "enabled_surfaces": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExperienceClientConfig | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/experience/client-config \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/experience/reason-tags

### 简介
支持方法: GET。

- `GET`: Returns enabled reason tags for the client whitelist scenes only. The response omits management metadata such as severity, version, enabled, deleted_at, and timestamps.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `scene` | query | enum(ai_suggestion_feedback/ai_suggestion_micro_question) | 否 | Optional client tag scene filter. Unknown scenes return an empty list. |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "scene": "...",
      "code": "...",
      "name": "...",
      "group": "...",
      "sort_order": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<ExperienceClientReasonTag> | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/experience/reason-tags \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/experience/behavior-events:batch

### 简介
支持方法: POST。

- `POST`: Best-effort side-channel capture for suggestion impressions, clicks, refreshes, and related actions. Events are idempotent by actor plus client_event_id and never block business workflows.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `events` | array<ExperienceBehaviorEventRequest> | 是 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "received": 123,
    "inserted": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExperienceBehaviorBatchResult | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/experience/behavior-events:batch \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/experience/micro-question-eligibility

### 简介
支持方法: GET。

- `GET`: Non-consuming eligibility check for low-interruption experience micro-questions. This side-channel never mutates task, asset, ERP, audit, cost, or permission state.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `suggestion_event_id` | query | string | 否 | Single-display suggestion id. Required for server-side suggestion lookup. |
| `suggestion_stable_key` | query | string | 否 | - |
| `surface` | query | string | 否 | - |
| `target_type` | query | string | 否 | - |
| `target_id` | query | string | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "eligible": true,
    "remaining_daily": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExperienceMicroQuestionEligibility | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/experience/micro-question-eligibility \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/experience/micro-question-answers

### 简介
支持方法: POST。

- `POST`: Records a side-channel answer for a low-interruption micro-question. Answers are separate from formal AI suggestion feedback and do not affect adoption-rate metrics directly.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `answer_event_key` | string | 否 | Optional idempotency key returned by eligibility. If omitted, the backend derives one. |
| `suggestion_event_id` | string | 是 | - |
| `suggestion_stable_key` | string | 否 | - |
| `surface` | string | 是 | - |
| `target_type` | string | 是 | - |
| `target_id` | string | 是 | - |
| `answer_value` | enum(answered/dismissed) | 是 | - |
| `reason_code` | enum(temporarily_not_needed/will_handle_later/already_handled/not_relevant/missing_context/stage_not_applicable/customer_special_case/suggestion_outdated) | 否 | Required when `answer_value` is `answered`; optional when dismissed. |
| `payload` | object | 否 | For approve, clients must include review_confirmation: true; this only materializes side-channel experience candidates and never mutates core business state. |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "answer_event_key": "string",
    "suggestion_event_id": "string",
    "suggestion_stable_key": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExperienceMicroQuestionAnswer | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request or daily micro-question quota exhausted |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/experience/micro-question-answers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/ai-suggestions/{suggestion_event_id}/feedback

### 简介
支持方法: POST。

- `POST`: Records human feedback for an AI or rule suggestion into the side-channel feedback table. This endpoint never executes a business action and returns 403 when the AI feedback flag is off.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `suggestion_event_id` | path | string | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `suggestion_event_id` | string | 否 | Optional when supplied by the path parameter. |
| `feedback_value` | enum(accepted/rejected/partially_accepted) | 是 | - |
| `reason_code` | string | 否 | - |
| `reason_note` | string | 否 | - |
| `outcome_source_type` | string | 否 | - |
| `outcome_source_id` | string | 否 | - |
| `payload` | object | 否 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "suggestion_event_id": "string",
    "feedback_value": "accepted",
    "reason_code": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AISuggestionFeedback | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden or experience AI feedback disabled |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/ai-suggestions/<suggestion_event_id>/feedback \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/events

### 简介
支持方法: GET。

- `GET`: Returns the asset-workbench audit timeline, including upload, whole-work file move/delete, repricing, quality import, settlement, and settings changes. List responses omit large before/after snapshots and include the operator username and display name.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `event_type` | query | string | 否 | - |
| `entity_type` | query | string | 否 | - |
| `entity_id` | query | integer | 否 | - |
| `actor_user_id` | query | integer | 否 | - |
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
      "event_type": "...",
      "entity_type": "...",
      "reason": "...",
      "created_at": "..."
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
| `data` | array<object> | 是 | - |
| `pagination` | PaginationMeta | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/events \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/notifications

### 简介
支持方法: GET。

- `GET`: Returns only notification types prefixed with `asset_workbench_`. Main-operations task notifications are never returned by this route.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `is_read` | query | boolean | 否 | - |
| `limit` | query | integer | 否 | - |
| `cursor` | query | string | 否 | - |

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
| 403 | 见 `error.code` | 见 `deny_code` | Asset-workbench membership or role required |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/notifications \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/notifications/{id}/read

### 简介
支持方法: POST。

- `POST`: Only the owner may update the row, and the notification type must belong to the `asset_workbench_` scope.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin。
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
| 404 | 见 `error.code` | 见 `deny_code` | Notification does not exist in the asset-workbench scope |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/notifications/<id>/read \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/notifications/read-all

### 简介
支持方法: POST。

- `POST`: Updates only the authenticated user's unread `asset_workbench_*` notifications.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin。
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
| 403 | 见 `error.code` | 见 `deny_code` | Asset-workbench membership or role required |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/notifications/read-all \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/notifications/unread-count

### 简介
支持方法: GET。

- `GET`: Counts only the authenticated user's unread `asset_workbench_*` notifications.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin。
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
| 403 | 见 `error.code` | 见 `deny_code` | Asset-workbench membership or role required |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/notifications/unread-count \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/notifications/broadcast

### 简介
支持方法: POST。

- `POST`: Creates persistent `system_broadcast` notifications for active recipients. Selected-user broadcasts are available to Admin, SuperAdmin, HRAdmin, and DepartmentAdmin. Full-system broadcasts are restricted to Admin, SuperAdmin, and HRAdmin.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Admin, SuperAdmin, HRAdmin, DepartmentAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `audience` | enum(all/users) | 是 | - |
| `user_ids` | array<integer> | 否 | Required when `audience = users`. |
| `title` | string | 是 | - |
| `content` | string | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "notification_count": 123,
    "recipient_count": 123,
    "recipient_ids": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid audience, content, or inactive recipient |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Actor is not allowed to broadcast to the requested audience |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/notifications/broadcast \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/entry

### 简介
支持方法: GET。

- `GET`: Returns not_member, pending, disabled, merged, or ready with inline bootstrap. This route is intentionally outside the active-membership gate.

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
    "state": "ready",
    "message": "string",
    "access": {
      "membership_status": "...",
      "is_enabled": "...",
      "is_admin_shell": "...",
      "asset_roles": "..."
    },
    "bootstrap": {}
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
curl -X GET https://api.example.com/v1/asset-workbench/entry \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/batch-jobs

### 简介
支持方法: GET。

- `GET`: Manager-only task center for long-running asset workbench operations, including recursive client-material publish/disable/remove jobs.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `status` | query | enum(queued/running/succeeded/failed/cancelled) | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "items": [
      "..."
    ],
    "total": 123,
    "page": 123,
    "size": 123
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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/batch-jobs \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/batch-jobs/{job_id}

### 简介
支持方法: GET。

- `GET`: Get one asset workbench batch job

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `job_id` | path | string | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "job_id": "string",
    "job_type": "client_material_batch_update",
    "status": "queued"
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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/batch-jobs/<job_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/register

### 简介
支持方法: POST。

- `POST`: Creates a workbench account only after all required contact, location, identity and payment profile fields pass validation.

### 鉴权与 RBAC
- 本节为公开资源接口，不需要 Bearer token。
- `POST` 允许角色: 公开。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `account` | string | 是 | - |
| `name` | string | 是 | - |
| `phone` | string | 是 | - |
| `email` | string | 否 | - |
| `password` | string | 是 | - |
| `worker_type` | enum(fulltime/parttime) | 是 | - |
| `province` | string | 是 | - |
| `city` | string | 是 | - |
| `id_card` | string | 是 | - |
| `gender` | enum(female/male) | 是 | - |
| `alipay_account` | string | 是 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "auth": {},
    "profile": {
      "id": "...",
      "user_id": "...",
      "worker_type": "...",
      "job_grade": "...",
      "real_name": "...",
      "province": "..."
    }
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AssetWorkbenchRegisterResponse | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid registration data |
| 409 | 见 `error.code` | 见 `deny_code` | Account already exists |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/register \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/access/request

### 简介
支持方法: POST。

- `POST`: Request asset workbench access

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
| `identity_type` | enum(staff/external/contractor) | 否 | - |
| `reason` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "app_code": "string",
    "user_id": 123,
    "status": "string"
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
curl -X POST https://api.example.com/v1/asset-workbench/access/request \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/access/open

### 简介
支持方法: POST。

- `POST`: AssetManager may first-open submitter-only access; SuperAdmin is required for restore or management roles.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `user_id` | integer | 是 | - |
| `roles` | array<enum(AssetSubmitter/AssetManager/AssetTemplateAdmin/AssetSettlement)> | 否 | - |
| `identity_type` | enum(staff/external/contractor) | 否 | - |
| `reason` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "app_code": "string",
    "user_id": 123,
    "status": "string"
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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/access/open \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/access/disable

### 简介
支持方法: POST。

- `POST`: Disable asset workbench access

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `user_id` | integer | 是 | - |
| `reason` | string | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "app_code": "string",
    "user_id": 123,
    "status": "string"
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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/access/disable \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/asset-workbench/profile

### 简介
支持方法: PATCH。

- `PATCH`: Updates the authenticated user's own contact, identity and payment profile. Worker grade and employment governance fields remain server-controlled.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, HRAdmin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `worker_type` | enum(fulltime/parttime) | 否 | - |
| `real_name` | string | 是 | - |
| `phone` | string | 是 | - |
| `province` | string | 是 | - |
| `city` | string | 是 | - |
| `id_card` | string | 是 | - |
| `gender` | enum(female/male) | 是 | - |
| `alipay_account` | string | 是 | - |
| `reason` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "user_id": 123,
    "worker_type": "fulltime",
    "job_grade": "string",
    "real_name": "string",
    "province": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AssetWorkbenchProfile | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid profile data |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/asset-workbench/profile \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/profiles

### 简介
支持方法: GET。

- `GET`: Lists workbench profiles for HR and settlement management. Phone, ID card and Alipay values are masked; use the single-profile endpoint to view an authorized full record with an audit event.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: HRAdmin, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `q` | query | string | 否 | - |
| `worker_type` | query | enum(fulltime/parttime) | 否 | - |
| `job_grade` | query | string | 否 | - |
| `status` | query | enum(pending/active/suspended) | 否 | - |
| `user_id` | query | integer | 否 | - |
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
      "worker_type": "...",
      "job_grade": "...",
      "real_name": "...",
      "province": "..."
    }
  ],
  "pagination": {
    "total": 123,
    "page": 123,
    "page_size": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<AssetWorkbenchProfile> | 否 | - |
| `pagination` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/profiles \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/profiles/{user_id}

### 简介
支持方法: GET, PATCH。

- `GET`: Returns full phone, ID card and Alipay fields to HR, settlement or SuperAdmin roles. Every successful read appends a profile PII access audit event without copying sensitive values into the event snapshot.
- `PATCH`: Updates HR-governed grade fields and personal contact, identity and payment fields for one workbench user.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: HRAdmin, AssetSettlement, SuperAdmin。
- `PATCH` 允许角色: HRAdmin, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `user_id` | path | integer | 是 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "user_id": 123,
    "worker_type": "fulltime",
    "job_grade": "string",
    "real_name": "string",
    "province": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AssetWorkbenchProfile | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Profile not found |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/profiles/<user_id> \
  -H "Authorization: Bearer $TOKEN"
```

#### PATCH 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `user_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `worker_type` | enum(fulltime/parttime) | 否 | - |
| `job_grade` | string | 否 | - |
| `real_name` | string | 否 | - |
| `phone` | string | 否 | - |
| `province` | string | 否 | - |
| `city` | string | 否 | - |
| `id_card` | string | 否 | - |
| `gender` | enum(/female/male) | 否 | - |
| `alipay_account` | string | 否 | - |
| `onboarded_at` | string | 否 | - |
| `grade_hidden` | boolean | 否 | - |
| `status` | enum(pending/active/suspended) | 否 | - |
| `reason` | string | 否 | - |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "user_id": 123,
    "worker_type": "fulltime",
    "job_grade": "string",
    "real_name": "string",
    "province": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AssetWorkbenchProfile | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid profile data |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Profile not found |

##### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/asset-workbench/profiles/<user_id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/members

### 简介
支持方法: GET。

- `GET`: Returns members from app_memberships, including pending, active, disabled, and merged states.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `q` | query | string | 否 | - |
| `identity` | query | enum(admin/normal) | 否 | - |
| `status` | query | enum(pending/active/disabled/merged) | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "user_id": "...",
      "username": "...",
      "display_name": "...",
      "real_name": "..."
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
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/members \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/asset-workbench/members/{user_id}/identity

### 简介
支持方法: PATCH。

- `PATCH`: Deprecated. Returns 410. Use PATCH /v1/asset-workbench/members/{user_id}/roles.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `user_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `identity` | enum(admin/normal) | 是 | - |
| `reason` | string | 否 | - |

### 响应体 schema
成功响应: `见 OpenAPI responses`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 410 | 见 `error.code` | 见 `deny_code` | Deprecated endpoint |
| 400 | 见 `error.code` | 见 `deny_code` | Invalid identity |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/asset-workbench/members/<user_id>/identity \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/asset-workbench/members/{user_id}/roles

### 简介
支持方法: PATCH。

- `PATCH`: SuperAdmin-only endpoint for composable business capabilities.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `user_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `roles` | array<enum(AssetSubmitter/AssetManager/AssetTemplateAdmin/AssetSettlement)> | 是 | - |
| `reason` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "user_id": 123,
    "username": "string",
    "display_name": "string",
    "real_name": "string"
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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 409 | 见 `error.code` | 见 `deny_code` | Membership is not active |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/asset-workbench/members/<user_id>/roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/accounts/merge/preview

### 简介
支持方法: POST。

- `POST`: Preview asset workbench account merge

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `source_user_id` | integer | 是 | - |
| `canonical_user_id` | integer | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "source_user_id": 123,
    "canonical_user_id": 123,
    "conflicts": {},
    "counts": {}
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/accounts/merge/preview \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/accounts/merge

### 简介
支持方法: POST。

- `POST`: Rewrites workbench ownership to canonical user. paid_to_user_id and payout_snapshot_json are never rewritten.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `source_user_id` | integer | 是 | - |
| `canonical_user_id` | integer | 是 | - |
| `profile_choices` | object | 是 | - |
| `reason` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "source_user_id": 123,
    "canonical_user_id": 123,
    "conflicts": {},
    "counts": {}
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Missing conflict choices |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 409 | 见 `error.code` | 见 `deny_code` | Merge conflict |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/accounts/merge \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/people-lookup

### 简介
支持方法: GET。

- `GET`: Name-based lookup for opening workbench access and member selectors. Returns masked, non-PII workbench member summaries.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetManager, AssetTemplateAdmin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `q` | query | string | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "user_id": "...",
      "username": "...",
      "display_name": "...",
      "real_name": "..."
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
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/people-lookup \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/groups/{group_id}/members

### 简介
支持方法: GET。

- `GET`: List asset workbench group members

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetManager, AssetTemplateAdmin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `group_id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "group_id": "...",
      "user_id": "...",
      "username": "...",
      "display_name": "..."
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
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/groups/<group_id>/members \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/difficulty-classes

### 简介
支持方法: GET, POST。

- `GET`: List enabled asset workbench difficulty classes
- `POST`: Create asset workbench difficulty class

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, HRAdmin, SuperAdmin。
- `POST` 允许角色: AssetTemplateAdmin, SuperAdmin。
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
  "data": [
    {
      "id": "...",
      "code": "...",
      "name": "...",
      "description": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<object> | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/difficulty-classes \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `code` | string | 是 | Stable pricing code used by rules and historical snapshots. |
| `name` | string | 是 | - |
| `description` | string | 否 | - |
| `enabled` | boolean | 否 | - |
| `sort_order` | integer | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "code": "string",
    "name": "string",
    "description": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/difficulty-classes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/difficulty-classes/admin

### 简介
支持方法: GET。

- `GET`: List all asset workbench difficulty classes for administration

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetManager, AssetTemplateAdmin, AssetSettlement, SuperAdmin。
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
      "id": "...",
      "code": "...",
      "name": "...",
      "description": "..."
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
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/difficulty-classes/admin \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/asset-workbench/difficulty-classes/{difficulty_code}

### 简介
支持方法: PATCH。

- `PATCH`: The difficulty code is stable and cannot be changed after creation; update the display name, description, enabled state, or sort order.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: AssetTemplateAdmin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `difficulty_code` | path | string | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `name` | string | 否 | - |
| `description` | string | 否 | - |
| `enabled` | boolean | 否 | - |
| `sort_order` | integer | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "code": "string",
    "name": "string",
    "description": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Difficulty class not found |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/asset-workbench/difficulty-classes/<difficulty_code> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/price-matrix

### 简介
支持方法: GET, POST。

- `GET`: List asset workbench price matrix rules
- `POST`: Create asset workbench price matrix rule

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetTemplateAdmin, AssetSettlement, SuperAdmin。
- `POST` 允许角色: AssetTemplateAdmin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `worker_type` | query | enum(all/fulltime/parttime) | 否 | - |
| `job_grade` | query | string | 否 | - |
| `difficulty_class` | query | string | 否 | - |
| `enabled` | query | boolean | 否 | - |
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
      "worker_type": "...",
      "job_grade": "...",
      "difficulty_class": "..."
    }
  ],
  "pagination": {
    "total": 123,
    "page": 123,
    "page_size": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<object> | 否 | - |
| `pagination` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/price-matrix \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `worker_type` | enum(fulltime/parttime) | 是 | - |
| `job_grade` | string | 是 | - |
| `difficulty_class` | string | 是 | Must match an enabled difficulty class code. |
| `unit_price` | number | 是 | - |
| `effective_from` | string | 是 | - |
| `effective_to` | string | 否 | - |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "worker_type": "string",
    "job_grade": "string",
    "difficulty_class": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 409 | 见 `error.code` | 见 `deny_code` | Overlapping effective range |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/price-matrix \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/asset-workbench/price-matrix/{rule_id}

### 简介
支持方法: PATCH。

- `PATCH`: Enable or disable a price matrix rule

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: AssetTemplateAdmin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `rule_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `enabled` | boolean | 是 | - |
| `reason` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "worker_type": "string",
    "job_grade": "string",
    "difficulty_class": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Rule not found |
| 409 | 见 `error.code` | 见 `deny_code` | Overlapping effective range |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/asset-workbench/price-matrix/<rule_id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/price-matrix/{rule_id}/supersede

### 简介
支持方法: POST。

- `POST`: Closes the selected rule at the day before the new effective_from and creates a new price rule for the same worker type, grade, and difficulty. Historical rows stay auditable through events, revision numbers, and stored pricing snapshots.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetTemplateAdmin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `rule_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `worker_type` | enum(fulltime/parttime) | 是 | - |
| `job_grade` | string | 是 | - |
| `difficulty_class` | string | 是 | Must match an enabled difficulty class code. |
| `unit_price` | number | 是 | - |
| `effective_from` | string | 是 | - |
| `effective_to` | string | 否 | - |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "worker_type": "string",
    "job_grade": "string",
    "difficulty_class": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Rule not found |
| 409 | 见 `error.code` | 见 `deny_code` | Overlapping effective range |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/price-matrix/<rule_id>/supersede \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/deduction-rules

### 简介
支持方法: GET, POST。

- `GET`: List asset workbench deduction rules
- `POST`: Create asset workbench deduction rule

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetTemplateAdmin, AssetSettlement, SuperAdmin。
- `POST` 允许角色: AssetTemplateAdmin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `worker_type` | query | enum(all/fulltime/parttime) | 否 | - |
| `job_grade` | query | string | 否 | - |
| `difficulty_class` | query | string | 否 | - |
| `enabled` | query | boolean | 否 | - |
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
      "worker_type": "...",
      "job_grade": "...",
      "difficulty_class": "..."
    }
  ],
  "pagination": {
    "total": 123,
    "page": 123,
    "page_size": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<object> | 否 | - |
| `pagination` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/deduction-rules \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `worker_type` | enum(all/fulltime/parttime) | 是 | - |
| `job_grade` | string | 是 | - |
| `difficulty_class` | string | 是 | Use all for wildcard |
| `deduction_amount` | number | 是 | - |
| `effective_from` | string | 是 | - |
| `effective_to` | string | 否 | - |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "worker_type": "string",
    "job_grade": "string",
    "difficulty_class": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 409 | 见 `error.code` | 见 `deny_code` | Overlapping effective range |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/deduction-rules \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/asset-workbench/deduction-rules/{rule_id}

### 简介
支持方法: PATCH。

- `PATCH`: Enable or disable a deduction rule

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: AssetTemplateAdmin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `rule_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `enabled` | boolean | 是 | - |
| `reason` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "worker_type": "string",
    "job_grade": "string",
    "difficulty_class": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Rule not found |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/asset-workbench/deduction-rules/<rule_id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/deduction-rules/{rule_id}/supersede

### 简介
支持方法: POST。

- `POST`: Supersede a deduction rule with a new revision

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetTemplateAdmin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `rule_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `worker_type` | enum(all/fulltime/parttime) | 是 | - |
| `job_grade` | string | 是 | - |
| `difficulty_class` | string | 是 | Use all for wildcard |
| `deduction_amount` | number | 是 | - |
| `effective_from` | string | 是 | - |
| `effective_to` | string | 否 | - |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "worker_type": "string",
    "job_grade": "string",
    "difficulty_class": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Rule not found |
| 409 | 见 `error.code` | 见 `deny_code` | Overlapping effective range |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/deduction-rules/<rule_id>/supersede \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/welfare-rules

### 简介
支持方法: GET, POST。

- `GET`: List asset workbench welfare rules
- `POST`: Create asset workbench welfare rule

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetTemplateAdmin, AssetSettlement, SuperAdmin。
- `POST` 允许角色: AssetTemplateAdmin, SuperAdmin。
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
  "data": [
    {
      "id": "...",
      "rule_name": "...",
      "worker_type": "...",
      "job_grade": "..."
    }
  ],
  "pagination": {
    "total": 123,
    "page": 123,
    "page_size": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<object> | 否 | - |
| `pagination` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/welfare-rules \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `rule_name` | string | 是 | - |
| `worker_type` | enum(all/fulltime/parttime) | 否 | - |
| `job_grade` | string | 否 | - |
| `rule_type` | string | 否 | - |
| `amount` | number | 是 | - |
| `config_json` | object | 否 | - |
| `effective_from` | string | 是 | - |
| `effective_to` | string | 否 | - |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "rule_name": "string",
    "worker_type": "string",
    "job_grade": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/welfare-rules \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/asset-workbench/welfare-rules/{rule_id}

### 简介
支持方法: PATCH。

- `PATCH`: Enable or disable a welfare rule

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: AssetTemplateAdmin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `rule_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `enabled` | boolean | 是 | - |
| `reason` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "rule_name": "string",
    "worker_type": "string",
    "job_grade": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Rule not found |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/asset-workbench/welfare-rules/<rule_id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/welfare-rules/{rule_id}/supersede

### 简介
支持方法: POST。

- `POST`: Supersede a welfare rule with a new row

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetTemplateAdmin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `rule_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `rule_name` | string | 是 | - |
| `worker_type` | enum(all/fulltime/parttime) | 否 | - |
| `job_grade` | string | 否 | - |
| `rule_type` | string | 否 | - |
| `amount` | number | 是 | - |
| `config_json` | object | 否 | - |
| `effective_from` | string | 是 | - |
| `effective_to` | string | 否 | - |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "rule_name": "string",
    "worker_type": "string",
    "job_grade": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Rule not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/welfare-rules/<rule_id>/supersede \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/promo-coupons

### 简介
支持方法: GET, POST。

- `GET`: List asset workbench promo coupons
- `POST`: Create asset workbench promo coupon

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetTemplateAdmin, AssetSettlement, SuperAdmin。
- `POST` 允许角色: AssetTemplateAdmin, SuperAdmin。
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
  "data": [
    {
      "id": "...",
      "coupon_code": "...",
      "coupon_name": "...",
      "mode": "..."
    }
  ],
  "pagination": {
    "total": 123,
    "page": 123,
    "page_size": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<object> | 否 | - |
| `pagination` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/promo-coupons \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `coupon_code` | string | 是 | - |
| `coupon_name` | string | 是 | - |
| `mode` | enum(fixed_price/markup_amount/markup_rate) | 是 | - |
| `amount` | number | 否 | - |
| `percent` | number | 否 | - |
| `priority` | integer | 否 | - |
| `worker_type` | enum(all/fulltime/parttime) | 否 | - |
| `job_grade` | string | 否 | - |
| `difficulty_class` | string | 否 | Use all for wildcard |
| `eligible_user_ids_json` | array<integer> | 否 | - |
| `eligible_codes_json` | array<string> | 否 | - |
| `effective_from` | string | 是 | - |
| `effective_to` | string | 否 | - |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "coupon_code": "string",
    "coupon_name": "string",
    "mode": "fixed_price"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/promo-coupons \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/asset-workbench/promo-coupons/{rule_id}

### 简介
支持方法: PATCH。

- `PATCH`: Enable or disable a promo coupon

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: AssetTemplateAdmin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `rule_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `enabled` | boolean | 是 | - |
| `reason` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "coupon_code": "string",
    "coupon_name": "string",
    "mode": "fixed_price"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Rule not found |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/asset-workbench/promo-coupons/<rule_id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/promo-coupons/{rule_id}/supersede

### 简介
支持方法: POST。

- `POST`: Supersede a promo coupon with a new row

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetTemplateAdmin, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `rule_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `coupon_code` | string | 是 | - |
| `coupon_name` | string | 是 | - |
| `mode` | enum(fixed_price/markup_amount/markup_rate) | 是 | - |
| `amount` | number | 否 | - |
| `percent` | number | 否 | - |
| `priority` | integer | 否 | - |
| `worker_type` | enum(all/fulltime/parttime) | 否 | - |
| `job_grade` | string | 否 | - |
| `difficulty_class` | string | 否 | Use all for wildcard |
| `eligible_user_ids_json` | array<integer> | 否 | - |
| `eligible_codes_json` | array<string> | 否 | - |
| `effective_from` | string | 是 | - |
| `effective_to` | string | 否 | - |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "coupon_code": "string",
    "coupon_name": "string",
    "mode": "fixed_price"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Rule not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/promo-coupons/<rule_id>/supersede \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/overview-search

### 简介
支持方法: GET。

- `GET`: Unified search across client-visible operational materials, system assets for managers, uploaded files, submissions, and piecework items. Date filters use Beijing business-day input when YYYY-MM-DD is supplied.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `q` | query | string | 否 | Search by code, order number, filename, template, submitter, or material keyword. |
| `scope` | query | enum(all/operational/files/orders) | 否 | Limits search to operational materials, uploaded files, order/piecework rows, or all sources. |
| `creator` | query | string | 否 | - |
| `date_from` | query | string | 否 | RFC3339 timestamp or YYYY-MM-DD in Beijing time. |
| `date_to` | query | string | 否 | RFC3339 timestamp or YYYY-MM-DD in Beijing time. |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "items": [
      "..."
    ],
    "total": 123,
    "page": 123,
    "size": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/overview-search \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/drive/directories

### 简介
支持方法: GET。

- `GET`: Lists virtual upload folders derived from submitted files. Non-manager users are scoped to their own uploads.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin。
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
      "directory_id": "...",
      "name": "...",
      "prefix": "...",
      "difficulty_class": "..."
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
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/drive/directories \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/drive/orders

### 简介
支持方法: GET。

- `GET`: Lists virtual order folders within a drive directory. Non-manager users are scoped to their own uploads.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `dir_id` | query | integer | 否 | - |
| `unassigned` | query | boolean | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "order_no": "...",
      "submission_item_id": "...",
      "submission_item_ids": "...",
      "file_count": "..."
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
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/drive/orders \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/drive/files

### 简介
支持方法: GET。

- `GET`: Lists files under a drive directory ordered by upload time. `order_no` is optional legacy/internal trace filtering; normal asset-workbench clients should browse by directory. Non-manager users are scoped to their own uploads.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `dir_id` | query | integer | 否 | - |
| `unassigned` | query | boolean | 否 | - |
| `order_no` | query | string | 否 | - |
| `q` | query | string | 否 | Keyword filter across filename, relative path, format, upload directory, uploader, order number, and submission number. |
| `owner` | query | string | 否 | Uploader name/account filter. |
| `created_from` | query | string | 否 | Upload time lower bound. Accepts RFC3339 or YYYY-MM-DD. |
| `created_to` | query | string | 否 | Upload time upper bound. Accepts RFC3339 or YYYY-MM-DD. |
| `sort_by` | query | enum(created_at/owner/creator/directory/category/name/display_name/format/file_type) | 否 | Sort field for upload ledger views. |
| `sort_dir` | query | enum(asc/desc) | 否 | Sort direction. |
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
      "submission_id": "...",
      "submission_item_id": "...",
      "submission_no": "..."
    }
  ],
  "pagination": {
    "total": 123,
    "page": 123,
    "page_size": 123
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
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/drive/files \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/drive/folder

### 简介
支持方法: GET。

- `GET`: Returns immediate child folders derived from uploaded relative_path values plus direct files under the requested virtual folder. Non-manager users are scoped to their own uploads.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `dir_id` | query | integer | 否 | - |
| `unassigned` | query | boolean | 否 | - |
| `path` | query | string | 否 | Relative virtual folder path inside the selected upload directory. |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "path": "string",
    "folders": [
      "..."
    ],
    "files": [
      "..."
    ],
    "total": 123
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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/drive/folder \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/drive/search

### 简介
支持方法: GET。

- `GET`: Searches uploaded file names, order numbers, and submission numbers. Non-manager users are scoped to their own uploads.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `q` | query | string | 否 | - |
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
      "submission_id": "...",
      "submission_item_id": "...",
      "submission_no": "..."
    }
  ],
  "pagination": {
    "total": 123,
    "page": 123,
    "page_size": 123
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
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/drive/search \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/drive/locate

### 简介
支持方法: GET。

- `GET`: Returns one drive file row with locate_page metadata for reveal-in-folder behavior. Non-manager users are scoped to their own uploads.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `file_id` | query | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "submission_id": 123,
    "submission_item_id": 123,
    "submission_no": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | File not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/drive/locate \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/submissions

### 简介
支持方法: GET, POST。

- `GET`: Lists submitted asset workbench batches. Managers can sort by creation time, first file type, or first filename; submitters are scoped to their own submissions.
- `POST`: Creates uploaded work records. Client uploads no longer need a frontend "template/type" selection: when an item omits `difficulty_class`, the backend derives it from the uploaded session's upload-directory difficulty snapshot. New integrations should submit by upload directory and difficulty snapshot, not by legacy template assignment.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, HRAdmin, SuperAdmin。
- `POST` 允许角色: AssetSubmitter, AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |
| `submitter_user_id` | query | integer | 否 | - |
| `payee_user_id` | query | integer | 否 | - |
| `business_month` | query | string | 否 | - |
| `status` | query | string | 否 | - |
| `settlement_status` | query | string | 否 | - |
| `order_by` | query | enum(submitted_at/created_at/file_type/file_name) | 否 | - |
| `order_dir` | query | enum(asc/desc) | 否 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "id": "...",
      "submission_no": "...",
      "submitter_user_id": "...",
      "submitter_name": "..."
    }
  ],
  "pagination": {
    "total": 123,
    "page": 123,
    "page_size": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<object> | 否 | - |
| `pagination` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/submissions \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `notes` | string | 否 | - |
| `expected_business_month` | string | 否 | Business month displayed to the client when the upload page was rendered. If it differs from server current month, backend returns MONTH_ROLLOVER_REQUIRED unless acknowledged. |
| `month_rollover_ack` | boolean | 否 | Explicit confirmation that a cross-month submission should count to the current server business month. |
| `business_month_override` | string | 否 | Manager-only manual backfill month. Rejected when the target month already has a non-cancelled settlement batch. |
| `items` | array<object> | 是 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "submission": {
      "id": "...",
      "submission_no": "...",
      "submitter_user_id": "...",
      "submitter_name": "..."
    },
    "items": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/submissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/submissions/{submission_id}/void

### 简介
支持方法: POST。

- `POST`: Voids an entire submission batch before settlement by marking all unsettled items as voided and setting the submission status to `voided`. Source files are retained for audit and download history; this is not a physical delete.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `submission_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `reason` | string | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "submission_no": "string",
    "submitter_user_id": 123,
    "submitter_name": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Submission not found |
| 409 | 见 `error.code` | 见 `deny_code` | Submission contains settled or in-batch items |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/submissions/<submission_id>/void \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/asset-workbench/items/{item_id}

### 简介
支持方法: PATCH。

- `PATCH`: Manager or settlement operator can correct order number, difficulty, finalized state, and page count before settlement confirmation. Pricing is recalculated with the item's worker type and job grade snapshot.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `item_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `order_no` | string | 否 | - |
| `difficulty_class` | string | 否 | Must match an enabled difficulty class code. |
| `finalized` | boolean | 否 | - |
| `page_count` | integer | 否 | - |
| `reason` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "submission_id": 123,
    "payee_user_id": 123,
    "entry_kind": "normal"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Submission item not found |
| 409 | 见 `error.code` | 见 `deny_code` | Item is locked by settlement state |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/asset-workbench/items/<item_id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/items/qc/excel

### 简介
支持方法: POST。

- `POST`: Updates item QC statuses in batch. Excel rows may identify items by `item_id`, or by `order_no` when `business_month` is supplied and the order number is unique in that month.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `multipart/form-data`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `business_month` | string | 否 | - |
| `file` | string | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "updated": [
      "..."
    ],
    "failures": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/items/qc/excel \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@example.xlsx"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/error-imports

### 简介
支持方法: POST。

- `POST`: Imports quality error deduction records. Deduction amount is not provided by the client; settlement preview and batch generation calculate it from the matched payee profile, difficulty class, error count, and active deduction rules. `order_no` is optional trace data for uploaded file naming and does not drive deduction matching.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `business_month` | string | 是 | - |
| `original_filename` | string | 否 | - |
| `records` | array<object> | 是 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "import_no": "string",
    "business_month": "string",
    "uploaded_by": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/error-imports \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/error-imports/excel

### 简介
支持方法: POST。

- `POST`: Accepts the formal quality error template with Chinese headers such as `线上订单号`, `分类`, `出错人`, `问题描述`, `抽查/售后`, `处理方法`, `登记人`, `备注`, and optional hidden `出错数`. Missing `出错数` defaults to 1. `线上订单号` is retained for traceability only; deduction calculation matches by payee and difficulty class.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `multipart/form-data`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `business_month` | string | 是 | - |
| `file` | string | 是 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "import_no": "string",
    "business_month": "string",
    "uploaded_by": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/error-imports/excel \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@example.xlsx"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/asset-workbench/files/{file_id}

### 简介
支持方法: PATCH。

- `PATCH`: Updates the editable display name for one uploaded work file. The original filename and object key remain unchanged.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `file_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `display_name` | string | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "submission_id": 123,
    "submission_item_id": 123,
    "upload_directory_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | File not found |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/asset-workbench/files/<file_id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/files/{file_id}/preview

### 简介
支持方法: GET。

- `GET`: Returns preview metadata for one uploaded work file. Submitters can access only their own files; managers and settlement roles can access visible workbench files.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `file_id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "file_id": 123,
    "status": "pending",
    "preparing": true,
    "preview_url": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | File not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/files/<file_id>/preview \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/files/{file_id}/download

### 简介
支持方法: GET。

- `GET`: Get uploaded work file download info

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `file_id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "file_id": 123,
    "filename": "string",
    "mime_type": "string",
    "file_size": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | File not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/files/<file_id>/download \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/files/{file_id}/archive

### 简介
支持方法: GET。

- `GET`: Lists folders and files inside an uploaded archive without extracting it into the workbench drive. Supports ZIP and RAR virtual browsing.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `file_id` | path | integer | 是 | - |
| `path` | query | string | 否 | Virtual folder path inside the archive. |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "file_id": 123,
    "path": "string",
    "format": "string",
    "folders": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request or unsupported archive format |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | File not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/files/<file_id>/archive \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/files/{file_id}/archive/entry

### 简介
支持方法: GET。

- `GET`: Streams a single file entry from a ZIP or RAR archive for inline preview or download. This endpoint returns file bytes rather than the standard JSON envelope.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `file_id` | path | integer | 是 | - |
| `path` | query | string | 是 | - |
| `disposition` | query | enum(inline/attachment) | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/octet-stream`

```json
"string"
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `body` | string | 视接口 | OpenAPI 声明的整体对象。 |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | File or entry not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/files/<file_id>/archive/entry \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/files/batch-move

### 简介
支持方法: POST。

- `POST`: Manager-only priced-work maintenance operation. All active files belonging to the same submission item must be selected together. The backend rejects settlement-locked items, copies the complete work into the target directory, updates directory snapshots, reprices the item using the target directory difficulty while preserving its item/page count, refreshes submission totals, records events, and best-effort removes old objects.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `file_ids` | array<integer> | 是 | - |
| `upload_directory_id` | integer | 是 | - |
| `reason` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "files": [
      "..."
    ],
    "deleted": [
      "..."
    ],
    "failures": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Upload directory not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/files/batch-move \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/files/batch-delete

### 简介
支持方法: POST。

- `POST`: Soft-deletes complete unsettled priced works. All active files belonging to the same submission item must be selected together; partial folder-work deletion and settlement-locked items are rejected. The related item is voided once and submission totals are refreshed. Submitters may delete only their own files; managers may delete any unlocked work.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetSubmitter, AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `file_ids` | array<integer> | 是 | - |
| `reason` | string | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "files": [
      "..."
    ],
    "deleted": [
      "..."
    ],
    "failures": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/files/batch-delete \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/system-search

### 简介
支持方法: GET。

- `GET`: Manager-only read-only material source search. Defaults to system + external assets, supports page/page_size pagination, and accepts legacy `limit` as page_size.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `q` | query | string | 否 | - |
| `source` | query | enum(all/system/external) | 否 | Source bucket for publishable material search. Defaults to `all`. |
| `format_category` | query | enum(all/image/design/pdf/video/archive) | 否 | Optional coarse file format category for narrowing batchable material search results. |
| `business_lane` | query | enum(all/customization/normal) | 否 | Optional system-resource business category. `customization` means 定制; `normal` means 常规. External resources are excluded when this filter is set. |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |
| `limit` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "items": [
      "..."
    ],
    "total": 123,
    "page": 123,
    "size": 123
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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/system-search \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/materials/groups

### 简介
支持方法: GET。

- `GET`: Groups materials by SKU namespace or external directory fallback. Group keys are namespaced, for example `sku:ABC123`, `ext-sku:ABC123`, `ext-dir:/p3/path`, or `system-asset:123`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `q` | query | string | 否 | - |
| `source` | query | enum(all/system/external) | 否 | - |
| `format_category` | query | enum(all/image/design/pdf/video/archive) | 否 | Optional coarse file format category applied before grouping. |
| `business_lane` | query | enum(all/customization/normal) | 否 | Optional system-resource business category. `customization` means 定制; `normal` means 常规. |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "items": [
      "..."
    ],
    "total": 123,
    "page": 123,
    "size": 123
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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/materials/groups \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/materials/group-files

### 简介
支持方法: GET。

- `GET`: List files inside one material group

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `group_key` | query | string | 是 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "group_key": "string",
    "items": [
      "..."
    ],
    "total": 123,
    "page": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/materials/group-files \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/materials/browse

### 简介
支持方法: GET。

- `GET`: Manager-only virtual material browser. Root returns system and external top-level folders; a folder path returns direct child folders and direct files without changing preview/download semantics.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `path` | query | string | 否 | Virtual folder path. Empty string means root; examples include `/系统资源`, `/quark`, and `/p3/仓库素材区`. |
| `source` | query | enum(all/system/external) | 否 | Source bucket to browse. Defaults to `all`. |
| `format_category` | query | enum(all/image/design/pdf/video/archive) | 否 | Optional coarse file format category for direct files returned from the current folder. |
| `business_lane` | query | enum(all/customization/normal) | 否 | Optional system-resource business category. `customization` means 定制; `normal` means 常规. External folders are hidden when this filter is set. |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |
| `limit` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "path": "string",
    "folders": [
      "..."
    ],
    "files": [
      "..."
    ],
    "total": 123
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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/materials/browse \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/settlement/supplement-permissions

### 简介
支持方法: GET, PUT。

- `GET`: Lists per-person supplement switches. An enabled permission is scoped to the current Asia/Shanghai natural month and can be closed at any time.
- `PUT`: Opens supplement entry only for the current Asia/Shanghai natural month, or closes an existing month permission. The target supplement date does not control the payroll month.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSettlement, SuperAdmin。
- `PUT` 允许角色: AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `payee_user_id` | query | integer | 否 | - |
| `business_month` | query | string | 否 | - |
| `enabled` | query | boolean | 否 | - |
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
      "submission_item_id": "...",
      "payee_user_id": "...",
      "business_month": "..."
    }
  ],
  "pagination": {
    "total": 123,
    "page": 123,
    "page_size": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<object> | 否 | - |
| `pagination` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/settlement/supplement-permissions \
  -H "Authorization: Bearer $TOKEN"
```

#### PUT 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `payee_user_id` | integer | 是 | - |
| `business_month` | string | 是 | - |
| `enabled` | boolean | 是 | - |
| `reason` | string | 否 | - |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "submission_item_id": 123,
    "payee_user_id": 123,
    "business_month": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid or non-current month |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X PUT https://api.example.com/v1/asset-workbench/settlement/supplement-permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/settlement/supplement-eligible-months

### 简介
支持方法: GET。

- `GET`: Returns the current Asia/Shanghai natural month for the selected person. Historical target dates are allowed, but their month never replaces the returned payroll month.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `payee_user_id` | query | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "months": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid payee_user_id |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/settlement/supplement-eligible-months \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/settlement/report

### 简介
支持方法: GET。

- `GET`: Read-only piecework report for one business month. Rows are split into normal piecework and supplement piecework, with distinct non-empty `order_no` counts and difficulty-level metrics.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `business_month` | query | string | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "business_month": "string",
    "difficulty_classes": [
      "..."
    ],
    "rows": [
      "..."
    ],
    "totals": {
      "payee_user_id": "...",
      "business_month": "...",
      "row_type": "...",
      "creator_name": "..."
    }
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/settlement/report \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/settlement/supplements

### 简介
支持方法: GET, POST。

- `GET`: Lists manual settlement supplement rows. Settlement roles can page, sort, and exact-filter by payee, month, file/work name, status, and supplement date.
- `POST`: Creates one supplement in the current Asia/Shanghai natural month. Settlement roles may create a manual row. Asset submitters may create only their own row, must supply uploaded `upload_session_ids`, and must still have an enabled current-month permission when the transaction locks the permission row. Uploaded supplements reuse normal upload directories, pricing, file query, and preview, but their linked submission item has `entry_kind=supplement` and is excluded from normal piecework settlement.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSettlement, SuperAdmin。
- `POST` 允许角色: AssetSubmitter, AssetManager, AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `business_month` | query | string | 否 | - |
| `payee_user_id` | query | integer | 否 | - |
| `order_no` | query | string | 否 | - |
| `status` | query | enum(draft/approved/in_batch/settled/voided) | 否 | - |
| `supplement_date` | query | string | 否 | Exact supplement date filter. |
| `supplement_date_from` | query | string | 否 | Inclusive supplement date lower bound. |
| `supplement_date_to` | query | string | 否 | Inclusive supplement date upper bound. |
| `sort_by` | query | enum(id/business_month/payee_user_id/order_no/supplement_date/status/gross_amount/created_at/updated_at) | 否 | - |
| `sort_dir` | query | enum(asc/desc) | 否 | - |
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
      "payee_user_id": "...",
      "business_month": "...",
      "linked_batch_id": "..."
    }
  ],
  "pagination": {
    "total": 123,
    "page": 123,
    "page_size": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<object> | 否 | - |
| `pagination` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/settlement/supplements \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `payee_user_id` | integer | 是 | - |
| `business_month` | string | 是 | - |
| `order_no` | string | 是 | File/work display name used for duplicate hints. |
| `supplement_date` | string | 否 | Historical target date for display and duplicate checks; it may be outside business_month. |
| `difficulty_class` | string | 是 | - |
| `finalized` | boolean | 否 | - |
| `page_count` | integer | 是 | - |
| `gross_amount` | number | 是 | - |
| `status` | enum(draft/approved) | 否 | - |
| `upload_session_ids` | array<string> | 否 | Uploaded asset-workbench sessions. Required for a non-settlement actor; all sessions must belong to the actor and use one upload-directory difficulty. |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "payee_user_id": 123,
    "business_month": "string",
    "linked_batch_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/settlement/supplements \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/settlement/my

### 简介
支持方法: GET。

- `GET`: Returns only the authenticated user's income rows, current-month supplement permission, and current-month supplement records with linked files for query and preview.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, AssetTemplateAdmin, AssetSettlement, HRAdmin, SuperAdmin。
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
    "current_month": "string",
    "estimated_net_amount": 12.3,
    "months": [
      "..."
    ],
    "supplement_permission": {
      "id": "...",
      "submission_item_id": "...",
      "payee_user_id": "...",
      "business_month": "..."
    }
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
| 403 | 见 `error.code` | 见 `deny_code` | Inactive membership or unsupported role |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/settlement/my \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/settlement/supplements/excel

### 简介
支持方法: POST。

- `POST`: Batch-creates approved supplement rows in the current Asia/Shanghai natural month. Optional `supplement_date` cells may point to historical days. Each row still checks payee/month supplement permission and duplicate hints.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `multipart/form-data`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `business_month` | string | 是 | - |
| `file` | string | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "created": [
      "..."
    ],
    "failures": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/settlement/supplements/excel \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@example.xlsx"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## DELETE /v1/asset-workbench/settlement/supplements/{supplement_id}

### 简介
支持方法: DELETE。

- `DELETE`: Deletes by marking the supplement as `voided`; in-batch and settled supplement rows are protected.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `DELETE` 允许角色: AssetSettlement, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `supplement_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `reason` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "payee_user_id": 123,
    "business_month": "string",
    "linked_batch_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Supplement not found |
| 409 | 见 `error.code` | 见 `deny_code` | Supplement is already in batch or settled |

### curl 示例
```bash
curl -X DELETE https://api.example.com/v1/asset-workbench/settlement/supplements/<supplement_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/upload-sessions

### 简介
支持方法: POST。

- `POST`: Creates a direct-upload session. Files no larger than the configured OSS part size use a single PUT; larger files use multipart upload. When upload directories are configured, `upload_directory_id` is required and the session stores the directory name, prefix, and difficulty snapshot. `file_hash` is optional metadata and is not required before upload starts.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetSubmitter, AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `original_filename` | string | 是 | - |
| `file_size` | integer | 是 | - |
| `mime_type` | string | 是 | - |
| `file_hash` | string | 否 | - |
| `upload_directory_id` | integer | 否 | - |
| `upload_batch_id` | string | 否 | - |
| `relative_path` | string | 否 | Browser folder-upload path such as folder/sub/file.jpg. Backend normalizes and rejects unsafe segments. |
| `is_folder_upload` | boolean | 否 | - |
| `expected_business_month` | string | 否 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "session": {
      "id": "...",
      "session_id": "...",
      "status": "...",
      "object_key": "..."
    },
    "plan": {}
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/upload-sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/upload-sessions/{session_id}/complete

### 简介
支持方法: POST。

- `POST`: Completes multipart uploads with ordered ETags. For a single-part session, the body may be empty and the backend verifies that the signed PUT created the expected OSS object.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetSubmitter, AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `session_id` | path | string | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `parts` | array<object> | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "session_id": "string",
    "status": "string",
    "object_key": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Upload session not found |
| 409 | 见 `error.code` | 见 `deny_code` | Upload session cannot be completed from current state |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/upload-sessions/<session_id>/complete \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/upload-directories

### 简介
支持方法: GET, POST。

- `GET`: Client-visible upload destinations. When this list is non-empty, upload session creation requires `upload_directory_id`; each directory also carries the pricing/difficulty class used by client uploads.
- `POST`: Create asset workbench upload directory

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, SuperAdmin。
- `POST` 允许角色: AssetManager, SuperAdmin。
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
  "data": [
    {
      "id": "...",
      "name": "...",
      "oss_prefix": "...",
      "description": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<object> | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/upload-directories \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `name` | string | 是 | - |
| `oss_prefix` | string | 是 | Relative OSS prefix under asset-workbench/uploads. |
| `description` | string | 否 | - |
| `difficulty_class` | string | 是 | Must match an enabled difficulty class code. |
| `allowed_file_types` | array<string> | 否 | Empty or omitted means all formats are allowed. Values may be file extensions without dots, MIME types, or wildcard MIME groups such as image/*. |
| `enabled` | boolean | 否 | - |
| `sort_order` | integer | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "name": "string",
    "oss_prefix": "string",
    "description": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/upload-directories \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/upload-directories/admin

### 简介
支持方法: GET。

- `GET`: List all asset workbench upload directories for administration

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetManager, SuperAdmin。
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
      "id": "...",
      "name": "...",
      "oss_prefix": "...",
      "description": "..."
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
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/upload-directories/admin \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/asset-workbench/upload-directories/{directory_id}

### 简介
支持方法: PATCH。

- `PATCH`: Update asset workbench upload directory

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `directory_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `name` | string | 否 | - |
| `oss_prefix` | string | 否 | - |
| `description` | string | 否 | - |
| `difficulty_class` | string | 否 | Must match an enabled difficulty class code. |
| `allowed_file_types` | array<string> | 否 | Empty or omitted means all formats are allowed. Values may be file extensions without dots, MIME types, or wildcard MIME groups such as image/*. |
| `enabled` | boolean | 否 | - |
| `sort_order` | integer | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "name": "string",
    "oss_prefix": "string",
    "description": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Upload directory not found |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/asset-workbench/upload-directories/<directory_id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/client-materials

### 简介
支持方法: GET, POST。

- `GET`: Returns enabled materials for clients. AssetManager/SuperAdmin may pass `admin=1` to include disabled materials.
- `POST`: Publishes an external asset or pins a task resource group's current finalized revision. Resource-group publications remain fixed until an explicit update.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, SuperAdmin。
- `POST` 允许角色: AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `admin` | query | boolean | 否 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "id": "...",
      "asset_id": "...",
      "source_type": "...",
      "source_ref": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<object> | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/client-materials \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `asset_id` | integer | 否 | - |
| `source_type` | enum(external_asset/task_resource_group) | 否 | - |
| `source_ref` | string | 否 | - |
| `resource_id` | string | 否 | - |
| `resource_group_id` | integer | 否 | - |
| `finalized_revision_id` | integer | 否 | Must equal the resource group's current finalized revision when publishing. |
| `cover_revision_item_id` | integer | 否 | - |
| `title` | string | 否 | - |
| `description` | string | 否 | - |
| `enabled` | boolean | 否 | - |
| `sort_order` | integer | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "asset_id": 123,
    "source_type": "external_asset",
    "source_ref": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/client-materials \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/client-materials/batch-update

### 简介
支持方法: POST。

- `POST`: Batch publishes selected system/external assets to client materials, or enables, disables, or removes existing client material publications. This synchronous endpoint is capped for selected-item operations; large folder-recursive operations should be promoted to an async task.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `action` | enum(publish/enable/disable/remove) | 是 | - |
| `items` | array<object> | 否 | - |
| `folders` | array<object> | 否 | Folder scopes resolved by backend. `include_children=true` recursively includes visible child folders up to the synchronous limit. |
| `query` | string | 否 | Optional current search keyword scope. |
| `source` | enum(all/system/external) | 否 | - |
| `format_category` | enum(all/image/design/pdf/video/archive) | 否 | - |
| `business_lane` | enum(all/customization/normal) | 否 | - |
| `selection_scope` | enum(selected/current_page/current_folder/current_folder_recursive/current_filter) | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "requested": 123,
    "created": 123,
    "updated": 123,
    "enabled": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/client-materials/batch-update \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/client-materials/search

### 简介
支持方法: GET。

- `GET`: Paginated search over materials published to clients. Non-manager users only receive enabled materials.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `q` | query | string | 否 | - |
| `sku` | query | string | 否 | - |
| `creator` | query | string | 否 | - |
| `admin` | query | boolean | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "items": [
      "..."
    ],
    "total": 123,
    "page": 123,
    "size": 123
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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/client-materials/search \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/asset-workbench/client-materials/{material_id}

### 简介
支持方法: PATCH, DELETE。

- `PATCH`: Update client material publication
- `DELETE`: Delete client material publication

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: AssetManager, SuperAdmin。
- `DELETE` 允许角色: AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### PATCH 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `material_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `asset_id` | integer | 否 | - |
| `source_type` | enum(external_asset/task_resource_group) | 否 | - |
| `source_ref` | string | 否 | - |
| `resource_id` | string | 否 | - |
| `resource_group_id` | integer | 否 | - |
| `finalized_revision_id` | integer | 否 | Must equal the resource group's current finalized revision when republishing. |
| `cover_revision_item_id` | integer | 否 | - |
| `title` | string | 否 | - |
| `description` | string | 否 | - |
| `enabled` | boolean | 否 | - |
| `sort_order` | integer | 否 | - |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "asset_id": 123,
    "source_type": "external_asset",
    "source_ref": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Client material not found |

##### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/asset-workbench/client-materials/<material_id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

#### DELETE 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `material_id` | path | integer | 是 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200`

无 JSON 响应体或响应体由文件流承载。

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Client material not found |

##### curl 示例
```bash
curl -X DELETE https://api.example.com/v1/asset-workbench/client-materials/<material_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/client-materials/{material_id}/download

### 简介
支持方法: GET。

- `GET`: Returns a direct download manifest only when the material is published and enabled for clients. External netdisk files without a public source URL are queued for OSS preparation and temporarily return an empty `download_url` with `access_hint=external_netdisk_prepare_required`; clients should poll this endpoint until the signed OSS URL is ready.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `material_id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "download_mode": "string",
    "download_url": "string",
    "access_hint": "string",
    "preview_available": true
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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Client material not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/client-materials/<material_id>/download \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/client-materials/{material_id}/preview

### 简介
支持方法: GET。

- `GET`: Returns preview metadata for a published client material without opening system asset search to submitters and without recording a download event. External materials return OSS/public preview URLs when ready, or `pending` while backend preview generation is queued.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetSubmitter, AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `material_id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "asset_id": 123,
    "source_type": "external_asset",
    "source_ref": "string",
    "status": "ready"
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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Client material not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/client-materials/<material_id>/preview \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/asset-workbench/client-materials/batch-download

### 简介
支持方法: POST。

- `POST`: Builds a direct-download manifest for published client material IDs. The backend does not open system search to clients.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: AssetSubmitter, AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `material_ids` | array<integer> | 是 | - |
| `naming_mode` | enum(business/original) | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "items": [
      "..."
    ],
    "failures": [
      "..."
    ],
    "success_count": 123,
    "failure_count": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/asset-workbench/client-materials/batch-download \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/asset-workbench/system-assets/{asset_id}/preview

### 简介
支持方法: GET。

- `GET`: Returns preview metadata for a read-only system asset in the asset workbench. The endpoint reuses the main asset-center preview semantics, including OSS image transforms and backend-derived preview/design-thumb assets for source formats such as PSD/AI/PDF/TIFF when available.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: AssetManager, SuperAdmin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `asset_id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "asset_id": 123,
    "status": "ready",
    "preparing": true,
    "preview_url": "string"
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
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/asset-workbench/system-assets/<asset_id>/preview \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。
- 创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。
- `sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

