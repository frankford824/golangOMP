# 资产资源库

> Revision: V8 current contract (2026-07-20)
> Source: docs/api/openapi.yaml

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

资产检索、详情、下载、预览、上传会话、归档与恢复。

## Family 约定

- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 本文件覆盖 `10` 个 `/v1` path；同一路径多 method 合并在同一节。

## POST /v1/assets/batch-download

### 简介
支持方法: POST。

- `POST`: Return direct download URLs for requested system and external asset-center resources. System assets use presigned OSS URLs for current versions; external resources are returned only when an OSS-ready URL is available. The backend does not proxy file bytes or build ZIP packages.

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
| `asset_ids` | array<integer> | 否 | System asset IDs retained for compatibility with earlier clients. |
| `resource_ids` | array<string> | 否 | Mixed resource IDs accepted by the asset center. System resources may use a numeric string; external resources use `ext-{id}` or `external:{id}`. |
| `naming_mode` | enum(original/business) | 否 | Download filename mode. original keeps original upload/file_name; business uses SKU plus task product name for batch business downloads. |

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
| `data` | AssetBatchDownloadManifest | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request or all assets unavailable |
| 500 | 见 `error.code` | 见 `deny_code` | Internal error while building direct download manifest |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/assets/batch-download \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/assets/{asset_id}

### 简介
支持方法: GET, DELETE。

- `GET`: Returns one asset resource by id including full version list. Source V1_ASSET_OWNERSHIP §5.2.
- `DELETE`: Discards one staged, unbound resource and its backend-derived preview/design-thumb resources; reason is required. Authorization is the explicit `asset.manage` capability intersected with the task's stable organization-ID scope. The same transaction locks the complete resource set, rejects any resource referenced by a working/finalized/historical resource-group revision or client publication, immediately revokes access, soft-deletes metadata, and writes durable adapter-aware object-deletion outbox rows. Physical deletion is asynchronous: only `oss_upload_service` rows reach OSS; placeholder/mock/export-placeholder rows complete without a physical call; unknown adapters fail closed, alert, and retry indefinitely. Object-not-found is success. Completed and Archived tasks require reopen; reopening never permits deletion of files retained by an earlier finalized revision.

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
| `asset_id` | path | string | 是 | Numeric system asset id or external resource id such as `ext-123`. |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "resource_id": "string",
    "source_type": "system",
    "source_label": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AssetDetail | 否 | Source: service/asset_center.AssetDetail — detail endpoint returns asset + version list. |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Asset not found |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/assets/<asset_id> \
  -H "Authorization: Bearer $TOKEN"
```

#### DELETE 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `asset_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `reason` | string | 是 | - |

##### 响应体 schema
成功响应: `204`

无 JSON 响应体或响应体由文件流承载。

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | Missing `asset.manage` capability or outside the task's stable organization-ID scope |
| 404 | 见 `error.code` | 见 `deny_code` | Asset not found |
| 409 | 见 `error.code` | 见 `deny_code` | Task requires reopen, or the resource is bound, finalized, historical, or publication-pinned |

##### curl 示例
```bash
curl -X DELETE https://api.example.com/v1/assets/<asset_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/assets/{asset_id}/download

### 简介
支持方法: GET。

- `GET`: Returns backend-authorized download metadata for one asset resource. Canonical runtime prefers browser-direct byte access.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `asset_id` | path | string | 是 | Numeric system asset id or external resource id such as `ext-123`. |

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
| 404 | 见 `error.code` | 见 `deny_code` | Asset not found |
| 410 | 见 `error.code` | 见 `deny_code` | Asset metadata exists but the object is no longer available |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/assets/<asset_id>/download \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/assets/{asset_id}/content

### 简介
支持方法: GET。

- `GET`: Authenticated byte-stream endpoint for external netdisk resources such as `/quark`. The backend authorizes and resolves the resource, then Nginx internally streams the signed AList `/p` source with HTTP Range support. Original bytes are not copied to OSS; derived thumbnails and previews remain OSS-backed.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `asset_id` | path | string | 是 | External resource id such as `ext-123`. |

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
| 400 | 见 `error.code` | 见 `deny_code` | Resource id is not an external netdisk asset |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |
| 404 | 见 `error.code` | 见 `deny_code` | Asset not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/assets/<asset_id>/content \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/assets/{asset_id}/preview

### 简介
支持方法: GET。

- `GET`: Returns preview metadata for one asset resource. For source formats that OSS IMG can process directly (`jpg/png/bmp/gif/webp/tiff/heic/avif`), this endpoint returns a signed private-bucket URL with `x-oss-process` preview transform. For source formats that are not directly previewable (such as PSD/PSB), this endpoint resolves backend-derived `preview/design_thumb` assets linked by `source_asset_id` when available. External resources prefer OSS-backed derived preview/original URLs or already-public provider URLs; browser-facing BFF proxy URLs are not returned as the default preview surface. A staged, unbound upload is visible only to its uploader with `asset.view`, or to an auditor whose explicit `task.audit` scope includes the task. Bound resources require `asset.view` within the task's stable organization-ID scope. Legacy roles and organization names do not authorize preview. When preview metadata is not currently available for the asset, runtime returns HTTP 409 with `error.code=INVALID_STATE_TRANSITION` and message `asset preview is not available`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `asset_id` | path | string | 是 | Numeric system asset id or external resource id such as `ext-123`. |

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
| 404 | 见 `error.code` | 见 `deny_code` | Asset not found |
| 403 | 见 `error.code` | 见 `deny_code` | Actor lacks preview capability or stable task scope |
| 409 | 见 `error.code` | 见 `deny_code` | Preview metadata not available for current asset state |
| 410 | 见 `error.code` | 见 `deny_code` | Asset metadata exists but the object is no longer available |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/assets/<asset_id>/preview \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/assets/upload-sessions

### 简介
支持方法: POST。

- `POST`: Creates a staged task-asset upload session and lets the backend choose single-part or multipart OSS upload. The task must be in an editable design or audit state. Authorization is `task.upload_source`, `task.audit`, or `asset.manage`, intersected with the task's stable organization-ID scope. `task.create` may create, complete, and cancel only `reference` uploads; it never authorizes source or final-product uploads. Upload completion never advances workflow state. Completed and Archived tasks reject upload-session access/mutations and must be reopened first. Task state is locked and checked again in every transaction that writes upload-session state.

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
| `body` | CreateAssetUploadSessionRequestCanonical | 视接口 | OpenAPI 声明的整体对象。 |

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
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |
| 409 | 见 `error.code` | 见 `deny_code` | Task state changed concurrently, or the task is Completed/Archived and requires reopen |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/assets/upload-sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/assets/upload-sessions/{session_id}

### 简介
支持方法: GET。

- `GET`: Returns the current upload-session state by session id. Because polling may synchronize remote session state, Completed and Archived tasks reject this operation and require reopen; the task is locked and checked again before any synchronized state is written.

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
| 409 | 见 `error.code` | 见 `deny_code` | Task is Completed/Archived and requires reopen |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/assets/upload-sessions/<session_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/assets/upload-sessions/{session_id}/complete

### 简介
支持方法: POST。

- `POST`: Completes one staged upload after OSS bytes are verified. It never advances workflow state. Completed and Archived tasks reject the mutation and require reopen; task state is locked and checked again inside the transaction.

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
| 404 | 见 `error.code` | 见 `deny_code` | Upload session not found |
| 409 | 见 `error.code` | 见 `deny_code` | Upload session already terminal, asset type mismatch, or Completed/Archived task requires reopen |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/assets/upload-sessions/<session_id>/complete \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/assets/upload-sessions/{session_id}/cancel

### 简介
支持方法: POST。

- `POST`: Cancels one staged upload session and aborts the remote OSS session when needed. Completed and Archived tasks reject the mutation and require reopen; task state is locked and checked again inside the transaction.

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
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload |
| 404 | 见 `error.code` | 见 `deny_code` | Upload session not found |
| 409 | 见 `error.code` | 见 `deny_code` | Upload session is terminal, or the task is Completed/Archived and requires reopen |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/assets/upload-sessions/<session_id>/cancel \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/assets/files/{path}

### 简介
支持方法: GET。

- `GET`: Compatibility-only authorization route for OSS-backed business files. When OSS direct storage is configured, it signs the authorized object and responds with `302` without first probing the legacy upload service. Non-OSS deployments retain the upstream proxy fallback. Canonical browser download should use the URL returned by `/v1/assets/{asset_id}/download` or `/v1/assets/{asset_id}/preview`. Path is the storage_key (e.g. tasks/task-create-reference/assets/.../filename.png).

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `path` | path | string | 是 | Storage key (may contain slashes) |

请求体: 无请求体。

### 响应体 schema
成功响应: `200`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | File not found |
| 502 | 见 `error.code` | 见 `deny_code` | Upstream file request failed |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/assets/files/<path> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

