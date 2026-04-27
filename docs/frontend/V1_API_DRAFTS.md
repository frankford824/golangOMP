# 任务草稿

> Revision: V1.2-D-2 residual drift triage (2026-04-26)
> Source: docs/api/openapi.yaml (post V1.2-D-2)

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

草稿创建、读取、删除、7 天过期与 20 条上限。

## Family 约定

- 草稿有 7 天过期与 20 条上限，前端保存失败时应提示用户清理旧草稿。
- 草稿 payload 由后端持久化，前端不要假设旧草稿一定符合最新创建表单。
- 本文件覆盖 `2` 个 `/v1` path；同一路径多 method 合并在同一节。

## POST /v1/task-drafts

### 简介
支持方法: POST。

- `POST`: Source: V1_INFORMATION_ARCHITECTURE §3.5.9. Body shape mirrors `POST /v1/tasks` with an optional `draft_id` for update semantics; no strict validation (draft is a work-in-progress snapshot). Per-owner cap 20 rows (IA-A16); 7-day expiry; deleted in-transaction on successful `POST /v1/tasks` with `source_draft_id` (IA-A15).

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
| `body` | TaskDraftPayload | 视接口 | Draft payload mirrors `POST /v1/tasks`. Source: V1_INFORMATION_ARCHITECTURE §3.5.9. |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "draft_id": 123,
    "owner_user_id": 123,
    "task_type": "string",
    "payload": {}
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskDraft | 否 | Source: V1_INFORMATION_ARCHITECTURE §3.5.9. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Malformed draft payload |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/task-drafts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 草稿有 7 天过期与 20 条上限，前端保存失败时应提示用户清理旧草稿。
- 草稿 payload 由后端持久化，前端不要假设旧草稿一定符合最新创建表单。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/task-drafts/{draft_id}

### 简介
支持方法: GET, DELETE。

- `GET`: Source: V1_INFORMATION_ARCHITECTURE §3.5.9. Only the draft creator may read; DeptAdmin / SuperAdmin are NOT authorized (privacy). Non-owner → 403.
- `DELETE`: Source: V1_INFORMATION_ARCHITECTURE §3.5.9. Only the draft creator may delete; non-owner → 403.

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
| `draft_id` | path | integer | 是 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "draft_id": 123,
    "owner_user_id": 123,
    "task_type": "string",
    "payload": {}
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskDraft | 否 | Source: V1_INFORMATION_ARCHITECTURE §3.5.9. |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Not the draft owner |
| 404 | 见 `error.code` | 见 `deny_code` | Draft not found |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/task-drafts/<draft_id> \
  -H "Authorization: Bearer $TOKEN"
```

#### DELETE 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `draft_id` | path | integer | 是 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `204`

无 JSON 响应体或响应体由文件流承载。

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Not the draft owner |
| 404 | 见 `error.code` | 见 `deny_code` | Draft not found |

##### curl 示例
```bash
curl -X DELETE https://api.example.com/v1/task-drafts/<draft_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 草稿有 7 天过期与 20 条上限，前端保存失败时应提示用户清理旧草稿。
- 草稿 payload 由后端持久化，前端不要假设旧草稿一定符合最新创建表单。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

