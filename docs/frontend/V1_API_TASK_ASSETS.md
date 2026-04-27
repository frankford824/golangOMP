# 任务资产中心

> Revision: V1.2-D-2 residual drift triage (2026-04-26)
> Source: docs/api/openapi.yaml (post V1.2-D-2)

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

任务内资产中心、创建前参考文件上传与任务参考文件。

## Family 约定

- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 本文件覆盖 `16` 个 `/v1` path；同一路径多 method 合并在同一节。

## POST /v1/task-create/asset-center/upload-sessions

### 简介
支持方法: POST。

- `POST`: Compatibility-only pre-task reference upload flow retained for rollback-safe migration. Obsolete for frontend rollout. New frontend integration must use `POST /v1/tasks/reference-upload`.

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
| `created_by` | integer | 否 | - |
| `filename` | string | 是 | - |
| `expected_size` | integer | 否 | - |
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
    }
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

### curl 示例
```bash
curl -X POST https://api.example.com/v1/task-create/asset-center/upload-sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/task-create/asset-center/upload-sessions/{session_id}

### 简介
支持方法: GET。

- `GET`: Compatibility-only status read for the old task-create upload-session flow. Obsolete for frontend rollout; new integration must use `POST /v1/tasks/reference-upload`.

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
curl -X GET https://api.example.com/v1/task-create/asset-center/upload-sessions/<session_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/task-create/asset-center/upload-sessions/{session_id}/complete

### 简介
支持方法: POST。

- `POST`: Compatibility-only completion for the old task-create upload-session flow. Obsolete for frontend rollout. Finalizes one old pre-task reference upload session and returns both the legacy `reference_file_ref` id and the normalized `ref_object` that can be passed into `POST /v1/tasks`.

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
| `completed_by` | integer | 否 | - |
| `file_hash` | string | 否 | - |
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
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload |
| 404 | 见 `error.code` | 见 `deny_code` | Upload session not found |
| 409 | 见 `error.code` | 见 `deny_code` | Upload session already terminal |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/task-create/asset-center/upload-sessions/<session_id>/complete \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/task-create/asset-center/upload-sessions/{session_id}/abort

### 简介
支持方法: POST。

- `POST`: Compatibility-only abort for the old task-create upload-session flow. Obsolete for frontend rollout; new integration must use `POST /v1/tasks/reference-upload`.

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
| `cancelled_by` | integer | 否 | - |
| `remark` | string | 否 | - |

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
| 404 | 见 `error.code` | 见 `deny_code` | Upload session not found |
| 409 | 见 `error.code` | 见 `deny_code` | Completed upload session cannot be aborted |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/task-create/asset-center/upload-sessions/<session_id>/abort \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/reference-upload

### 简介
支持方法: POST。

- `POST`: Canonical frontend entry for task-create reference upload. Accepts one `multipart/form-data` file field named `file`, writes it to OSS through backend-controlled direct storage flow when available (and uses upload-service proxy only as compatibility fallback), records a completed legal reference source, and returns one normalized `reference_file_ref` object. The returned object should be appended directly into `POST /v1/tasks.reference_file_refs`.

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
| `created_by` | integer | 否 | - |
| `remark` | string | 否 | - |
| `file_hash` | string | 否 | - |
| `file` | string | 是 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "asset_id": "string",
    "source": "task_reference_upload"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ReferenceFileRef | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid upload request |
| 401 | 见 `error.code` | 见 `deny_code` | Authentication required |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/reference-upload \
  -H "Authorization: Bearer $TOKEN"
  -F "file=@example.xlsx"
```

### 前端最佳实践
- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/asset-center/assets

### 简介
支持方法: GET。

- `GET`: Compatibility-only alias for `GET /v1/tasks/{id}/assets`. Keep for migration safety only; new frontend integration must use the canonical task-linked lookup path `GET /v1/tasks/{id}/assets`.

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
curl -X GET https://api.example.com/v1/tasks/<id>/asset-center/assets \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/asset-center/assets/{asset_id}/versions

### 简介
支持方法: GET。

- `GET`: Compatibility-only alias for `GET /v1/tasks/{id}/assets/{asset_id}/versions`. Obsolete for frontend rollout.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
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
curl -X GET https://api.example.com/v1/tasks/<id>/asset-center/assets/<asset_id>/versions \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/asset-center/assets/{asset_id}/download

### 简介
支持方法: GET。

- `GET`: Compatibility-only alias for `GET /v1/tasks/{id}/assets/{asset_id}/download`. Obsolete for frontend rollout.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
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
curl -X GET https://api.example.com/v1/tasks/<id>/asset-center/assets/<asset_id>/download \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/asset-center/assets/{asset_id}/versions/{version_id}/download

### 简介
支持方法: GET。

- `GET`: Compatibility-only alias for `GET /v1/tasks/{id}/assets/{asset_id}/versions/{version_id}/download`. Obsolete for frontend rollout.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |
| `asset_id` | path | integer | 是 | - |
| `version_id` | path | integer | 是 | - |

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
curl -X GET https://api.example.com/v1/tasks/<id>/asset-center/assets/<asset_id>/versions/<version_id>/download \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/asset-center/upload-sessions

### 简介
支持方法: POST。

- `POST`: Compatibility-only alias for `POST /v1/assets/upload-sessions`. Use the top-level asset session route for new frontend work.

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
| `created_by` | integer | 否 | - |
| `asset_id` | integer | 否 | - |
| `source_asset_id` | integer | 否 | Optional linkage to a source asset. Allowed for `preview` and `design_thumb` intents. |
| `asset_type` | enum(reference/source/delivery/preview/design_thumb) | 否 | Compatibility alias of `asset_kind` retained for migration safety. |
| `asset_kind` | enum(reference/source/delivery/preview/design_thumb) | 否 | Canonical upload intent field for new frontend integrations. |
| `upload_mode` | enum(small/multipart) | 否 | Compatibility-only input. New frontend integrations must not send this field. |
| `filename` | string | 否 | Compatibility alias of `file_name`. At least one of `file_name` or `filename` must be provided. |
| `file_name` | string | 否 | Canonical file name field for new frontend integrations. At least one of `file_name` or `filename` must be provided. |
| `expected_size` | integer | 否 | Optional size hint in bytes. |
| `file_size` | integer | 否 | Optional compatibility alias of `expected_size`. |
| `mime_type` | string | 否 | Optional MIME hint. |
| `file_hash` | string | 否 | - |
| `remark` | string | 否 | - |
| `target_sku_code` | string | 否 | Required for multi-SKU batch-task non-reference uploads. Backend validates that the SKU belongs to the task, returns it on the upload-session business view as `target_sku_code`, and persists the completed asset scope on `scope_sku_code` for the asset root and asset version. |

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
    "upload_strategy": "string",
    "required_upload_content_type": "string"
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
curl -X POST https://api.example.com/v1/tasks/<id>/asset-center/upload-sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/asset-center/upload-sessions/small

### 简介
支持方法: POST。

- `POST`: Compatibility-only alias for `POST /v1/assets/upload-sessions`. Use the top-level asset session route for new frontend work.

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
| `created_by` | integer | 否 | - |
| `asset_id` | integer | 否 | - |
| `source_asset_id` | integer | 否 | Optional linkage to a source asset. Allowed for `preview` and `design_thumb` intents. |
| `asset_type` | enum(reference/source/delivery/preview/design_thumb) | 否 | Compatibility alias of `asset_kind` retained for migration safety. |
| `asset_kind` | enum(reference/source/delivery/preview/design_thumb) | 否 | Canonical upload intent field for new frontend integrations. |
| `upload_mode` | enum(small/multipart) | 否 | Compatibility-only input. New frontend integrations must not send this field. |
| `filename` | string | 否 | Compatibility alias of `file_name`. At least one of `file_name` or `filename` must be provided. |
| `file_name` | string | 否 | Canonical file name field for new frontend integrations. At least one of `file_name` or `filename` must be provided. |
| `expected_size` | integer | 否 | Optional size hint in bytes. |
| `file_size` | integer | 否 | Optional compatibility alias of `expected_size`. |
| `mime_type` | string | 否 | Optional MIME hint. |
| `file_hash` | string | 否 | - |
| `remark` | string | 否 | - |
| `target_sku_code` | string | 否 | Required for multi-SKU batch-task non-reference uploads. Backend validates that the SKU belongs to the task, returns it on the upload-session business view as `target_sku_code`, and persists the completed asset scope on `scope_sku_code` for the asset root and asset version. |

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
    "upload_strategy": "string",
    "required_upload_content_type": "string"
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
curl -X POST https://api.example.com/v1/tasks/<id>/asset-center/upload-sessions/small \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/asset-center/upload-sessions/multipart

### 简介
支持方法: POST。

- `POST`: Compatibility-only alias for `POST /v1/assets/upload-sessions`. Use the top-level asset session route for new frontend work.

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
| `created_by` | integer | 否 | - |
| `asset_id` | integer | 否 | - |
| `source_asset_id` | integer | 否 | Optional linkage to a source asset. Allowed for `preview` and `design_thumb` intents. |
| `asset_type` | enum(reference/source/delivery/preview/design_thumb) | 否 | Compatibility alias of `asset_kind` retained for migration safety. |
| `asset_kind` | enum(reference/source/delivery/preview/design_thumb) | 否 | Canonical upload intent field for new frontend integrations. |
| `upload_mode` | enum(small/multipart) | 否 | Compatibility-only input. New frontend integrations must not send this field. |
| `filename` | string | 否 | Compatibility alias of `file_name`. At least one of `file_name` or `filename` must be provided. |
| `file_name` | string | 否 | Canonical file name field for new frontend integrations. At least one of `file_name` or `filename` must be provided. |
| `expected_size` | integer | 否 | Optional size hint in bytes. |
| `file_size` | integer | 否 | Optional compatibility alias of `expected_size`. |
| `mime_type` | string | 否 | Optional MIME hint. |
| `file_hash` | string | 否 | - |
| `remark` | string | 否 | - |
| `target_sku_code` | string | 否 | Required for multi-SKU batch-task non-reference uploads. Backend validates that the SKU belongs to the task, returns it on the upload-session business view as `target_sku_code`, and persists the completed asset scope on `scope_sku_code` for the asset root and asset version. |

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
    "upload_strategy": "string",
    "required_upload_content_type": "string"
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
curl -X POST https://api.example.com/v1/tasks/<id>/asset-center/upload-sessions/multipart \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/asset-center/upload-sessions/{session_id}

### 简介
支持方法: GET。

- `GET`: Compatibility-only alias for `GET /v1/assets/upload-sessions/{session_id}`. Returns the MAIN-side upload-session business view.

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

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/<id>/asset-center/upload-sessions/<session_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/complete

### 简介
支持方法: POST。

- `POST`: Compatibility-only alias for `POST /v1/assets/upload-sessions/{session_id}/complete`. For multi-SKU batch delivery submissions, whole-task status does not advance on the first successful bucket. `PendingAuditA` is entered only after all SKU items required by the batch have a completed delivery asset.

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
| `completed_by` | integer | 否 | - |
| `file_hash` | string | 否 | - |
| `upload_content_type` | string | 否 | Exact `required_upload_content_type` echoed back by the client when finalizing an OSS direct upload. |
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
| 409 | 见 `error.code` | 见 `deny_code` | Upload session already terminal or asset type mismatch |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/asset-center/upload-sessions/<session_id>/complete \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/cancel

### 简介
支持方法: POST。

- `POST`: Compatibility-only alias for `POST /v1/assets/upload-sessions/{session_id}/cancel`. Cancels the MAIN business session and aborts the remote OSS upload session when needed.

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
| `cancelled_by` | integer | 否 | - |
| `remark` | string | 否 | - |

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
| 409 | 见 `error.code` | 见 `deny_code` | Completed upload session cannot be cancelled |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/asset-center/upload-sessions/<session_id>/cancel \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/abort

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
| `cancelled_by` | integer | 否 | - |
| `remark` | string | 否 | - |

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
| 409 | 见 `error.code` | 见 `deny_code` | Completed upload session cannot be aborted |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/asset-center/upload-sessions/<session_id>/abort \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 新联调优先使用 `/v1/tasks/{id}/asset-center/*`。
- `/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

