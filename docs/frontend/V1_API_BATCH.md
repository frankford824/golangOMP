# Excel 批量创建

> Revision: V1.3-A2 i_id-first task/ERP/search integration (2026-04-27)
> Source: docs/api/openapi.yaml (post V1.3-A2)

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

批量创建模板下载、Excel 解析与前端预览校验。

## Family 约定

- 批量创建只做模板下载和解析预览，不直接写任务表。
- Excel 字段与枚举以模板中的 Schema/EnumDict sheet 和接口 violations 为准。
- 本文件覆盖 `2` 个 `/v1` path；同一路径多 method 合并在同一节。

## GET /v1/tasks/batch-create/template.xlsx

### 简介
支持方法: GET。

- `GET`: Downloads the batch-create workbook. For `new_product_development`, the Items sheet intentionally contains only `产品名称` and `设计要求`; category/product-code details are resolved by the backend during task creation. For `purchase_task`, the purchase-specific template remains unchanged.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `task_type` | query | enum(new_product_development/purchase_task) | 否 | - |

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
| 400 | 见 `error.code` | 见 `deny_code` | Invalid task_type |
| 401 | 见 `error.code` | 见 `deny_code` | Authentication required |
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/batch-create/template.xlsx \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- 批量创建只做模板下载和解析预览，不直接写任务表。
- Excel 字段与枚举以模板中的 Schema/EnumDict sheet 和接口 violations 为准。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/batch-create/parse-excel

### 简介
支持方法: POST。

- `POST`: Parses the batch-create workbook into `batch_items`. For `new_product_development`, only `产品名称` and `设计要求` are required and returned by the template parser.

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
| `task_type` | enum(new_product_development/purchase_task) | 是 | - |
| `file` | string | 是 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "task_type": "new_product_development",
    "preview": [
      "..."
    ],
    "violations": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | BatchCreateParseResult | 否 | Source: V1_INFORMATION_ARCHITECTURE §3.5.4. |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid upload or parse error |
| 401 | 见 `error.code` | 见 `deny_code` | Authentication required |
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |
| 413 | 见 `error.code` | 见 `deny_code` | File too large |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/batch-create/parse-excel \
  -H "Authorization: Bearer $TOKEN"
  -F "file=@example.xlsx"
```

### 前端最佳实践
- 批量创建只做模板下载和解析预览，不直接写任务表。
- Excel 字段与枚举以模板中的 Schema/EnumDict sheet 和接口 violations 为准。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

