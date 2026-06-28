# L1 报表

> Revision: V1.3-A2 i_id-first task/ERP/search integration (2026-04-27)
> Source: docs/api/openapi.yaml (post V1.3-A2)

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

L1 卡片、吞吐与模块停留报表。

## Family 约定

- L1 报表仅 super_admin 可用。
- 403 时重点展示 `reports_super_admin_only`。
- 本文件覆盖 `6` 个 `/v1` path；同一路径多 method 合并在同一节。

## GET /v1/reports/experience/stats

### 简介
支持方法: GET。

- `GET`: SuperAdmin-only read model for capture success, outbox backlog, dead-letter count, tag coverage, AI feedback rate, profile generation, and asset quality labels.

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
    "flags": {
      "ui_enabled": "...",
      "capture_enabled": "...",
      "ai_feedback_enabled": "...",
      "worker_enabled": "..."
    },
    "total_events": 123,
    "outbox_queued": 123,
    "outbox_processing": 123,
    "outbox_dead_letter": 123,
    "generated_at": "2026-04-25T10:30:41Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ExperienceStats | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/reports/experience/stats \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- L1 报表仅 super_admin 可用。
- 403 时重点展示 `reports_super_admin_only`。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/reports/experience/samples

### 简介
支持方法: GET。

- `GET`: SuperAdmin-only sample pool. The query reads only `experience_events` and never joins core task or asset tables.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `source_type` | query | string | 否 | - |
| `source_id` | query | string | 否 | - |
| `task_id` | query | integer | 否 | - |
| `action` | query | string | 否 | - |
| `outcome` | query | string | 否 | - |
| `from` | query | string | 否 | ISO date or RFC3339 timestamp, inclusive. |
| `to` | query | string | 否 | ISO date or RFC3339 timestamp, inclusive. |
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
      "event_key": "...",
      "schema_version": "...",
      "event_time": "..."
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
| `data` | array<ExperienceEvent> | 是 | - |
| `pagination` | PaginationMeta | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid query |
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/reports/experience/samples \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- L1 报表仅 super_admin 可用。
- 403 时重点展示 `reports_super_admin_only`。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/reports/l1/cards

### 简介
支持方法: GET。

- `GET`: Source: V1_INFORMATION_ARCHITECTURE §1 一级菜单「报表」. Returns the top-row report cards (task counts, throughput delta, etc.). RBAC (R1.7-D Q5=E1): super_admin only.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: super_admin。
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
      "title": "...",
      "value": "...",
      "unit": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<L1Card> | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden. `deny_code=reports_super_admin_only` when the caller role is not `super_admin`. |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/reports/l1/cards \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- L1 报表仅 super_admin 可用。
- 403 时重点展示 `reports_super_admin_only`。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/reports/l1/throughput

### 简介
支持方法: GET。

- `GET`: Source: V1_INFORMATION_ARCHITECTURE §1 一级菜单「报表」 + V1_MODULE_ARCHITECTURE §12. Daily task throughput (created / completed / archived counts) within a [from, to] window. RBAC (R1.7-D Q5=E1): super_admin only. Backend (R1.7-D Q6=C1): v1 直查 `task_module_events` + `tasks`,不建物化表。

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: super_admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `from` | query | string | 是 | Start of the report window (inclusive, ISO 8601 date). |
| `to` | query | string | 是 | End of the report window (inclusive, ISO 8601 date). |
| `department_id` | query | integer | 否 | Optional filter by owning department. |
| `task_type` | query | string | 否 | Optional filter by task type key. |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "date": "...",
      "created": "...",
      "completed": "...",
      "archived": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<object> | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden. `deny_code=reports_super_admin_only`. |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/reports/l1/throughput \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- L1 报表仅 super_admin 可用。
- 403 时重点展示 `reports_super_admin_only`。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/reports/l1/module-dwell

### 简介
支持方法: GET。

- `GET`: Source: V1_INFORMATION_ARCHITECTURE §1 一级菜单「报表」 + V1_MODULE_ARCHITECTURE §12. Average and P95 dwell time per module (computed from `task_module_events`) within [from, to]. RBAC (R1.7-D Q5=E1): super_admin only.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: super_admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `from` | query | string | 是 | - |
| `to` | query | string | 是 | - |
| `department_id` | query | integer | 否 | Optional filter by owning department. |
| `task_type` | query | string | 否 | Optional filter by task type key. |

请求体: 无请求体。

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": [
    {
      "module_key": "...",
      "avg_dwell_seconds": "...",
      "p95_dwell_seconds": "...",
      "samples": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<object> | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden. `deny_code=reports_super_admin_only`. |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/reports/l1/module-dwell \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- L1 报表仅 super_admin 可用。
- 403 时重点展示 `reports_super_admin_only`。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/reports/l1/kpi-events

### 简介
支持方法: GET。

- `GET`: Returns task workflow events enriched with task priority, status and deadline for the KPI/data-center page. RBAC: super_admin only. This endpoint is read-only and does not perform AI analysis.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: super_admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `from` | query | string | 是 | Start of the report window (inclusive, ISO 8601 date). |
| `to` | query | string | 是 | End of the report window (inclusive, ISO 8601 date). |
| `limit` | query | integer | 否 | Maximum number of events to return. Defaults to 2000. |

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
      "sku_code": "..."
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | array<KPIAnalysisEvent> | 是 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthenticated |
| 403 | 见 `error.code` | 见 `deny_code` | Forbidden. `deny_code=reports_super_admin_only`. |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/reports/l1/kpi-events \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- L1 报表仅 super_admin 可用。
- 403 时重点展示 `reports_super_admin_only`。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

