# 任务资产中心

> Revision: V8 current contract (2026-07-20)
> Source: docs/api/openapi.yaml

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

任务内资产中心、创建前参考文件上传与任务参考文件。

## Family 约定

- 已有任务使用 `/v1/tasks/{id}/asset-center/*`；创建前参考图使用 `/v1/tasks/reference-upload-sessions*`。
- 本文件覆盖 `1` 个 `/v1` path；同一路径多 method 合并在同一节。

## POST /v1/tasks/reference-upload

### 简介
支持方法: POST。

- `POST`: Deprecated rollback-compatible multipart proxy. New clients must use `POST /v1/tasks/reference-upload-sessions` so file bytes travel directly from the browser to OSS instead of through the application server. Accepts one `multipart/form-data` file field named `file`, writes it to OSS through backend-controlled direct storage flow when available (and uses upload-service proxy only as compatibility fallback), records a completed legal reference source, and returns one normalized `reference_file_ref` object. The returned object should be appended directly into `POST /v1/tasks.reference_file_refs`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `Idempotency-Key` | header | string | 否 | Optional create-task idempotency key. Equivalent to request body `client_create_id`. |

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
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@example.xlsx"
```

### 前端最佳实践
- 已有任务使用 `/v1/tasks/{id}/asset-center/*`；创建前参考图使用 `/v1/tasks/reference-upload-sessions*`。
- 只使用本文列出的当前 V8 路径；已退役路径不再提供兼容入口。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

