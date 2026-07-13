# 资产资源库

> Revision: V1.3-A2 i_id-first task/ERP/search integration (2026-04-27)
> Source: docs/api/openapi.yaml (post V1.3-A2)

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

资产检索、详情、下载、预览、上传会话、归档与恢复。

## Family 约定

- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 本文件覆盖 `20` 个 `/v1` path；同一路径多 method 合并在同一节。

## GET /v1/assets

### 简介
支持方法: GET。

- `GET`: Canonical resource catalog for asset lookup. Supports task-linked lookup plus minimal resource filters for task, SKU scope, archive state, and upload state. For PSD preview flows, callers can filter by `source_asset_id` to find backend-owned preview/thumb derivatives.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `task_id` | query | integer | 否 | - |
| `asset_kind` | query | enum(reference/source/delivery/preview/design_thumb) | 否 | - |
| `source_asset_id` | query | integer | 否 | - |
| `scope_sku_code` | query | string | 否 | - |
| `archive_status` | query | string | 否 | - |
| `upload_status` | query | string | 否 | - |

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
| 401 | UNAUTHENTICATED | - | 未登录、token 缺失或 token 过期。 |
| 403 | PERMISSION_DENIED | 见接口返回 | 角色、组织范围、字段级授权或流程状态不允许。 |
| 404 | NOT_FOUND | - | 资源不存在或当前用户不可见。 |
| 409 | CONFLICT | 见接口返回 | 状态竞态、重复操作或版本冲突。 |
| 422 | VALIDATION_ERROR | - | 请求参数或业务字段校验失败。 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/assets \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/assets/batch-download

### 简介
支持方法: POST。

- `POST`: Return direct download URLs for requested system and external asset-center resources. System assets use presigned OSS URLs for current versions; external resources are returned only when an OSS-ready URL is available. The backend does not proxy file bytes or build ZIP packages.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin。
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
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/assets/excel-package/preview

### 简介
支持方法: POST。

- `POST`: Matches uploaded Excel rows to current JPG/PNG system assets and OSS-ready external assets, returns direct download URLs and per-row failures. Single images remain flat in the ZIP. Under `/p3/仓库素材区/徐凯`, an exact SKU-prefixed source directory with at least two indexed image files is returned as a complete set; the frontend restores that source directory in the ZIP and keeps all components together. Address summaries, missing-SKU reports, source filenames, and quantity-based copies are preserved.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `rows` | array<AssetExcelPackageRow> | 是 | - |

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
    "total_files": 123,
    "total_size": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AssetExcelPackageManifest | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 500 | 见 `error.code` | 见 `deny_code` | Internal error while matching assets |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/assets/excel-package/preview \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/assets/excel-package/preview-file

### 简介
支持方法: POST。

- `POST`: Uploads an Eve-compatible .xlsx or .xls template, parses order number, SKU, quantity, address, and keyword columns on the backend, then returns the same flat-single-image and complete-multi-image-set manifest as /v1/assets/excel-package/preview.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `multipart/form-data`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `file` | string | 是 | Eve-compatible .xlsx or .xls template, max 10 MiB. |

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
    "total_files": 123,
    "total_size": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AssetExcelPackageManifest | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid file or unsupported template |
| 500 | 见 `error.code` | 见 `deny_code` | Internal error while matching assets |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/assets/excel-package/preview-file \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@example.xlsx"
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/assets/{asset_id}

### 简介
支持方法: GET, DELETE。

- `GET`: Returns one asset resource by id including full version list. Source V1_ASSET_OWNERSHIP §5.2.
- `DELETE`: Hard-delete all stored versions (OSS DeleteObject + soft-delete DB rows); reason is required. SuperAdmin may delete assets in supported lifecycle states. For a Completed task, CustomizationReviewer, Audit_A, Audit_B, and AssetManager may delete only the current system reference/source/delivery resource. Active-task workflow permissions are unchanged. Source V1_ASSET_OWNERSHIP §5.4 plus completed-resource maintenance policy.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin。
- `DELETE` 允许角色: SuperAdmin, CustomizationReviewer, Audit_A, Audit_B, AssetManager。
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
| 403 | 见 `error.code` | 见 `deny_code` | Role or completed-resource maintenance scope denied |
| 404 | 见 `error.code` | 见 `deny_code` | Asset not found |

##### curl 示例
```bash
curl -X DELETE https://api.example.com/v1/assets/<asset_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/assets/{asset_id}/download

### 简介
支持方法: GET。

- `GET`: Returns backend-authorized download metadata for one asset resource. Canonical runtime prefers browser-direct byte access.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin。
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

### curl 示例
```bash
curl -X GET https://api.example.com/v1/assets/<asset_id>/download \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/assets/{asset_id}/content

### 简介
支持方法: GET。

- `GET`: Authenticated byte-stream endpoint for external netdisk resources such as `/quark`. The backend authorizes and resolves the resource, then Nginx internally streams the signed AList `/p` source with HTTP Range support. Original bytes are not copied to OSS; derived thumbnails and previews remain OSS-backed.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin。
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
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/assets/{asset_id}/preview

### 简介
支持方法: GET。

- `GET`: Returns preview metadata for one asset resource. For source formats that OSS IMG can process directly (`jpg/png/bmp/gif/webp/tiff/heic/avif`), this endpoint returns a signed private-bucket URL with `x-oss-process` preview transform. For source formats that are not directly previewable (such as PSD/PSB), this endpoint resolves backend-derived `preview/design_thumb` assets linked by `source_asset_id` when available. External resources prefer OSS-backed derived preview/original URLs or already-public provider URLs; browser-facing BFF proxy URLs are not returned as the default preview surface. When preview metadata is not currently available for the asset, runtime returns HTTP 409 with `error.code=INVALID_STATE_TRANSITION` and message `asset preview is not available`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, Admin。
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
| 409 | 见 `error.code` | 见 `deny_code` | Preview metadata not available for current asset state |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/assets/<asset_id>/preview \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/assets/upload-sessions

### 简介
支持方法: POST。

- `POST`: Canonical frontend entry for asset upload session creation. Backend decides whether to use single-part or multipart upload and returns the upload strategy plus completion/cancel endpoints. Reference uploads are allowed while a task is still `PendingAssign`, so operations users can fill in reference material before a designer self-claims or is assigned. `CustomizationReviewer` is not a generic asset uploader; it may use this route only through `task.customization.review.asset_upload` for uploaded `source` assets while the task is in `PendingCustomizationReview` or `PendingEffectReview`. When the task is `Completed`, this route only replaces an active, non-archived current `reference`, `source`, or `delivery` resource supplied through `asset_id`; it never creates an unrelated new resource. `CustomizationReviewer`, regular audit roles (`Audit_A`/`Audit_B`), and `AssetManager` may perform this completed-task replacement across task department/team scope; active workflow tasks retain their normal workflow and organization-scope authorization. During `PendingCustomizationReview` or `PendingEffectReview`, an existing `delivery` may be replaced only when its `asset_id` is supplied; creating a second delivery root from those review states is rejected.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, AssetManager, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
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
| 409 | 见 `error.code` | 见 `deny_code` | Completed task request is not an existing-current-resource replacement |

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
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/assets/upload-sessions/{session_id}

### 简介
支持方法: GET。

- `GET`: Returns the current upload-session state by session id.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, Warehouse, AssetManager, Admin。
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
curl -X GET https://api.example.com/v1/assets/upload-sessions/<session_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/assets/upload-sessions/{session_id}/complete

### 简介
支持方法: POST。

- `POST`: Completes one asset upload session after the frontend uploads bytes to OSS using the returned plan. `CustomizationReviewer` completion is allowed only for the same restricted customization-review source upload flow documented on create. A delivery replacement created while the task is `Completed` remains approved and does not reopen the task workflow.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, AssetManager, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
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
| 409 | 见 `error.code` | 见 `deny_code` | Upload session already terminal or asset type mismatch |

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
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/assets/upload-sessions/{session_id}/cancel

### 简介
支持方法: POST。

- `POST`: Cancels one asset upload session and aborts the remote OSS session when needed. `CustomizationReviewer` cancellation is allowed only for the same restricted customization-review source upload flow documented on create. Completed tasks may cancel only upload sessions that were created as an existing-resource replacement.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Designer, CustomizationOperator, CustomizationReviewer, Ops, Audit_A, Audit_B, AssetManager, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
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
| 409 | 见 `error.code` | 见 `deny_code` | Completed upload session cannot be cancelled |

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
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
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
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/assets/upload-requests

### 简介
支持方法: GET, POST。

- `GET`: Internal placeholder management view for upload requests. Supports paginated filtering by owner boundary, task-asset type, and lifecycle status so upload/storage placeholder records can be inspected without introducing a real upload allocator, signed-URL session, or object storage/NAS integration.
- `POST`: Internal placeholder upload-intent boundary for task assets. This endpoint does not upload file bytes, does not return signed URLs, and does not connect to NAS/object storage. It only records one placeholder upload request that later task-asset write actions may bind into a `storage_ref`. Shared `adapter_mode`, `dispatch_mode`, `storage_mode`, `adapter_ref_summary`, and `handoff_ref_summary` fields keep storage and upload language aligned with export and integration. Lifecycle advancement remains a separate internal placeholder route.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, Audit_A, Audit_B, Warehouse, Outsource, Admin。
- `POST` 允许角色: Ops, Designer, Audit_A, Audit_B, Warehouse, Outsource, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `owner_type` | query | enum(task/task_asset/export_job/outsource_order/warehouse_receipt) | 否 | - |
| `owner_id` | query | integer | 否 | - |
| `task_asset_type` | query | enum(reference/source/delivery/preview/design_thumb) | 否 | - |
| `status` | query | enum(requested/bound/expired/cancelled) | 否 | - |
| `page` | query | integer | 否 | - |
| `page_size` | query | integer | 否 | - |

请求体: 无请求体。

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "request_id": "...",
      "owner_type": "...",
      "owner_id": "...",
      "task_id": "..."
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
| `data` | array<UploadRequest> | 否 | - |
| `pagination` | PaginationMeta | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid filter values |

##### curl 示例
```bash
curl -X GET https://api.example.com/v1/assets/upload-requests \
  -H "Authorization: Bearer $TOKEN"
```

#### POST 细节

##### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `owner_type` | enum(task/task_asset/export_job/outsource_order/warehouse_receipt) | 是 | - |
| `owner_id` | integer | 是 | - |
| `task_asset_type` | enum(reference/source/delivery/preview/design_thumb) | 否 | - |
| `storage_adapter` | enum(mock_upload/placeholder_storage/export_placeholder/oss_upload_service) | 否 | - |
| `ref_type` | enum(task_asset_object/export_result/generic_object) | 否 | - |
| `file_name` | string | 是 | - |
| `mime_type` | string | 否 | - |
| `file_size` | integer | 否 | - |
| `checksum_hint` | string | 否 | - |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "request_id": "string",
    "owner_type": "task",
    "owner_id": 123,
    "task_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | UploadRequest | 否 | Placeholder upload-intent contract only. Upload requests are storage/upload handoff records aligned with shared export and integration boundary language, with lifecycle readiness hints for `requested -> bound|expired|cancelled`. Creating or advancing an upload request does not upload bytes and does not allocate a real object-storage or NAS location. |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload or unsupported owner boundary |
| 404 | 见 `error.code` | 见 `deny_code` | Owner not found |

##### curl 示例
```bash
curl -X POST https://api.example.com/v1/assets/upload-requests \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/assets/upload-requests/{id}

### 简介
支持方法: GET。

- `GET`: Returns one persisted placeholder upload request. `can_bind`, `can_cancel`, and `can_expire` expose internal placeholder lifecycle readiness while the payload remains storage and upload metadata only.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Ops, Designer, Audit_A, Audit_B, Warehouse, Outsource, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | string | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "request_id": "string",
    "owner_type": "task",
    "owner_id": 123,
    "task_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | UploadRequest | 否 | Placeholder upload-intent contract only. Upload requests are storage/upload handoff records aligned with shared export and integration boundary language, with lifecycle readiness hints for `requested -> bound|expired|cancelled`. Creating or advancing an upload request does not upload bytes and does not allocate a real object-storage or NAS location. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Upload request not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/assets/upload-requests/<id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/assets/upload-requests/{id}/advance

### 简介
支持方法: POST。

- `POST`: Internal placeholder lifecycle action for upload requests. Supports `cancel` and `expire` while keeping `bound` reserved for later task-asset binding. This endpoint does not upload bytes, does not allocate storage, and does not replace task-asset writes as the actual binding path.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Ops, Designer, Audit_A, Audit_B, Warehouse, Outsource, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | string | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `action` | enum(cancel/expire) | 是 | - |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "request_id": "string",
    "owner_type": "task",
    "owner_id": 123,
    "task_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | UploadRequest | 否 | Placeholder upload-intent contract only. Upload requests are storage/upload handoff records aligned with shared export and integration boundary language, with lifecycle readiness hints for `requested -> bound|expired|cancelled`. Creating or advancing an upload request does not upload bytes and does not allocate a real object-storage or NAS location. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload or invalid upload-request lifecycle transition |
| 404 | 见 `error.code` | 见 `deny_code` | Upload request not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/assets/upload-requests/<id>/advance \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/assets/search/batch

### 简介
支持方法: POST。

- `POST`: Batch asset search for the asset management bulk-download dialog. It returns ranked system and external resource candidates per input term and avoids issuing one HTTP request per SKU/task number.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

无 path/query/header 参数。

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `terms` | array<string> | 是 | SKU codes, task numbers, or external-resource filename/path keywords, one logical search term per item. |
| `format_filter` | enum(jpg_png/jpg/png/webp/image/design/pdf/archive/all) | 否 | UI-facing format filter. The server maps this to a coarse DB format category first, then applies exact extension/MIME filtering. |
| `asset_kind` | enum(auto/all/delivery/reference/source/preview/other) | 否 | UI-facing asset role filter. System resources use task asset roles; external resources do not have system roles and are included for auto/all/delivery/source searches by filename/path format. |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "results": [
      "..."
    ],
    "matched_count": 123,
    "failed_count": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AssetBatchSearchResponse | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request |
| 401 | 见 `error.code` | 见 `deny_code` | Authentication required |
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/assets/search/batch \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/assets/{asset_id}/versions/{version_id}/download

### 简介
支持方法: GET。

- `GET`: Returns presigned download metadata for a specific historical version. Returns 410 GONE when the version has been auto-cleaned (V1_ASSET_OWNERSHIP §7.4); 404 when the asset itself is soft-deleted.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
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
| 404 | 见 `error.code` | 见 `deny_code` | Asset deleted or not found |
| 410 | 见 `error.code` | 见 `deny_code` | Asset version auto-cleaned (V1_ASSET_OWNERSHIP §7.4) |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/assets/<asset_id>/versions/<version_id>/download \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/assets/{asset_id}/archive

### 简介
支持方法: POST。

- `POST`: Archive asset (is_archived=1, OSS retained). SuperAdmin only; reason required. Source V1_ASSET_OWNERSHIP §7.3.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `asset_id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `reason` | string | 是 | - |

### 响应体 schema
成功响应: `见 OpenAPI responses`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | Not SuperAdmin |
| 404 | 见 `error.code` | 见 `deny_code` | Asset not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/assets/<asset_id>/archive \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/assets/{asset_id}/restore

### 简介
支持方法: POST。

- `POST`: Un-archive asset (is_archived=0). SuperAdmin only. Source V1_ASSET_OWNERSHIP §7.3.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `asset_id` | path | integer | 是 | - |

请求体: 无请求体。

### 响应体 schema
成功响应: `见 OpenAPI responses`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | Not SuperAdmin |
| 404 | 见 `error.code` | 见 `deny_code` | Asset not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/assets/<asset_id>/restore \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 资产上传建议走 upload session；下载与预览 URL 以接口返回为准。
- 删除、归档、恢复动作需按返回错误处理竞态和权限失败。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

