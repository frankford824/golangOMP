# 资产模块（Asset / `src/services/api/assetsApi.ts`）

> 回到索引：[`README.md`](./README.md) · 本模块共 **9 个在用接口**；1 个未使用接口（`getUploadSessionStatus`）见 [`00-unused-endpoints.md`](./00-unused-endpoints.md)。
>
> **特别说明**：OSS 直传（`PUT` 签名 URL）由 `src/services/upload/ossDirectUpload.ts` 直接调用 `axios.request`，不经 `http.ts` 封装；其契约由 create-session 的响应决定（见 3.E.2）。

---

## 3.E.1 任务作用域资产列表

- **请求路径**：`/v1/tasks/{id}/assets`
- **请求方法**：`GET`
- **定义**：`assetsApi.list`
- **调用位置**：
  - `src/views/TaskAssetsView.vue:336`
  - `src/views/AssetsIndexView.vue:696`
  - `src/components/warehouse/WarehouseReceiptPanel.vue:320`
  - `src/components/task-detail/DesignAssetBlock.vue:746`
  - `src/domain/asset-access.ts:302`
- **前端传参**：`id` Path。
- **期望返回**：`BackendAsset[]`（或 `{ data: [...] }` / `{ items: [...] }`），`BackendAsset` 结构见 `apiTypes.ts:293`：`id, task_id, file_role, previous_asset_id, current_asset_id, workflow_lane, source_department, versions[], approved_version, warehouse_ready_version`。

---

## 3.E.2 创建资产上传会话

- **请求路径**：`/v1/assets/upload-sessions`
- **请求方法**：`POST`
- **定义**：`assetsApi.createAssetUploadSession`
- **调用**：`src/services/upload/assetUploadFlow.ts:135` (`prepareTaskAssetUploadSession`)

**Body (`CreateAssetUploadSessionPayload`)**：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `task_id` | `string \| number` | 基本必填 | 纯数字字符串会被转成 `number` 传（`taskIdForSessionBody`）；无 task_id 仅走 `/v1/tasks/reference-upload` fallback |
| `asset_kind` | `'reference' \| 'source' \| 'delivery' \| 'preview' \| 'design_thumb'` | 是 | |
| `file_name` | `string` | 是 | 取 `file.name` |
| `expected_size` | `number` | 是 | `file.size` |
| `mime_type` | `string` | 是 | `resolveFileMimeType(file)`；**必须与后续 OSS 直传 Content-Type 一致**（已由前端做 `normalizeMimeType` 校验） |
| `file_hash` | `string` | 可选 | 当前未计算 |
| `remark` | `string` | 可选 | 默认 `file.name`（可追加多 SKU 后缀） |
| `source_asset_id` | `string \| number` | 可选 | 替换链路 |
| `target_sku_code` | `string` | 可选 | 批量交付必传，且必须属于该任务（否则后端返回 `INVALID_REQUEST: target_sku_code must belong to the task`，前端转换为友好提示） |

**期望返回** (`AssetUploadSessionCreateResponse`)：

```json
{
  "data": {
    "session": {
      "id": "string",
      "session_id": "string",
      "expected_size": "number | null",
      "session_status": "string | null",
      "upload_status": "string | null"
    },
    "oss_direct": {
      "mode": "single_part | multipart",
      "upload_strategy": "single_part | multipart",
      "upload_url": "string | null",
      "part_upload_url_template": "string | null",
      "method": "PUT",
      "headers": { "Content-Type": "..." },
      "required_upload_content_type": "string",
      "parts_total": "number | null",
      "part_size_hint": "number | null",
      "expected_size": "number | null",
      "bucket": "string | null",
      "endpoint": "string | null",
      "object_key": "string | null",
      "upload_id": "string | null",
      "parts": [
        { "part_number": "number", "upload_url": "string", "method": "PUT", "expires_at": "string | null" }
      ]
    },
    "complete_endpoint": "string | null",
    "cancel_endpoint": "string | null"
  }
}
```

**关键约束**：`oss_direct.required_upload_content_type` 必须与 create-session 提交的 `mime_type` 匹配，否则前端直接抛错阻止直传（避免 OSS `SignatureDoesNotMatch`）。

---

## 3.E.3 完成上传会话

- **请求路径**：`/v1/assets/upload-sessions/{session_id}/complete`
- **请求方法**：`POST`
- **定义**：`assetsApi.completeAssetUploadSession`
- **调用**：`assetUploadFlow.ts:210` (`completePreparedTaskAssetUploadSession`)

**Body (`CompleteAssetUploadSessionPayload`)**：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `remark` | `string` | 可选 | |
| `file_hash` | `string` | 可选 | |
| `oss_upload_id` | `string` | multipart 必填 | 前端 multipart 缺失时阻止请求 |
| `oss_object_key` | `string` | multipart 必填 | 同上 |
| `oss_parts` | `[{ part_number: number, etag: string }]` | multipart 必填 | 分片合并用 |
| `upload_content_type` | `string` | multipart 必填 | 与 direct 上传时一致 |

**期望返回**：

```json
{
  "data": {
    "session": { "id": "string", "session_id": "string", "session_status": "completed", "upload_status": "uploaded" },
    "asset": "BackendAsset",
    "version": "BackendAssetVersion"
  }
}
```

前端强校验：`session_status` 非 `completed` 或 `upload_status` 非 `uploaded` 立即抛错（`assertAssetCenterUploadCompleteOk`）。

**特例**：complete 返回 `error.code=PERMISSION_DENIED` + `deny_code=task_status_not_actionable` + `action=asset_upload_session_complete`（说明任务已切到非可上传状态）时，前端**不再调用 cancel**，让后端保留现场（见 `isTaskStatusNotActionableUploadError`）。

---

## 3.E.4 取消上传会话

- **请求路径**：`/v1/assets/upload-sessions/{session_id}/cancel`
- **请求方法**：`POST`
- **定义**：`assetsApi.cancelAssetUploadSession`
- **调用**：`assetUploadFlow.ts:232`（直传失败 / complete 失败时尽力取消，错误被吞）
- **Body**：`{}` 或 `Record<string, unknown>`。
- **期望返回**：前端不消费。

---

## 3.E.5 全局资产检索

- **请求路径**：`/v1/assets`
- **请求方法**：`GET`
- **定义**：`assetsApi.listAssets`
- **调用**：`src/views/AssetsIndexView.vue:686、711、717`

**前端传参 (Query, `AssetListQuery`)**：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `task_id` | `string \| number` | 显式任务过滤 |
| `asset_kind` | `string` | `reference / delivery / source / ...` |
| `source_asset_id` | `string \| number` | 查替换链 |
| `scope_sku_code` | `string` | 按 SKU 查（搜索兜底） |
| `page` / `page_size` | `number` | |

**期望返回**：`BackendAsset[]` 或 `{ data: [...] }` 或 `{ items: [...] }`，前端已做三态兼容。

---

## 3.E.6 单条资产详情

- **请求路径**：`/v1/assets/{id}`
- **请求方法**：`GET`
- **定义**：`assetsApi.getAsset`
- **调用**：`AssetDetailView.vue:256`、`TaskAssetsView.vue:260`、`AssetsIndexView.vue:625`
- **期望返回**：`{ data: BackendAsset }` 或裸 `BackendAsset`；前端通过 `normalizeAssetDetailFromApi` 规范化。

---

## 3.E.7 资产下载元信息

- **请求路径**：`/v1/assets/{id}/download`
- **请求方法**：`GET`
- **定义**：`assetsApi.getAssetDownloadMeta`
- **调用**：上述三处 + `src/domain/asset-access.ts:144`

**期望返回**：

```json
{
  "data": {
    "download_mode": "direct | proxy | public | private_network",
    "download_url": "string | null",
    "expires_at": "string | null",
    "access_hint": "string | null",
    "preview_available": "boolean",
    "filename": "string",
    "file_size": "number",
    "mime_type": "string"
  }
}
```

**关键决策依据**：UI 下载按钮显隐严格按 `download_mode` 与 `preview_available` 判定（见 `.cursorrules` v0.6 约定）；`download_url` 必须是规范业务入口（相对路径）。

---

## 3.E.8 资产预览元信息

- **请求路径**：`/v1/assets/{id}/preview`
- **请求方法**：`GET`
- **定义**：`assetsApi.getAssetPreviewMeta`
- **调用**：`src/domain/asset-access.ts:106`
- **期望返回**：同 3.E.7 的结构；预览不可用时 `preview_available=false`（前端分 `unavailable / not_found` 两态展示）。

---

## 3.E.9 创建前参考图 fallback 上传

- **请求路径**：`/v1/tasks/reference-upload`
- **请求方法**：`POST`（`multipart/form-data`）
- **定义**：`assetsApi.uploadReferenceForNewTask`
- **调用**：`src/services/upload/assetUploadFlow.ts:286` (`uploadReferenceFileRef` fallback 分支)
- **Body**：`FormData`，仅 `file` 一个字段；`timeout: 90s`。

**期望返回**：

```json
{
  "data": {
    "data": {
      "asset_id": "string",
      "ref_id": "string",
      "upload_request_id": "string",
      "filename": "string",
      "mime_type": "string",
      "file_size": "number",
      "download_url": "string | null",
      "source": "string",
      "status": "string"
    }
  }
}
```

兼容双层 `data.data` / 单层 `data`；前端将该对象直接塞进任务创建 payload 的 `reference_file_refs`。
