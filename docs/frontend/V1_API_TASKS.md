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
- 本文件覆盖 `188` 个 `/v1` path；同一路径多 method 合并在同一节。

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

## POST /v1/tasks/prepare-product-codes

### 简介
支持方法: POST。

- `POST`: Allocates unique default product codes for task-create UIs. Default format is selected by `sku_code_type`: `regular` allocates `CG + {CATEGORY_LETTER} + {6-digit sequence}` and `customization` allocates `DZ + {CATEGORY_LETTER} + {6-digit sequence}`. This endpoint does not require frontend code-rule/template selection and is available for `new_product_development` and `purchase_task`.

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
| `task_type` | enum(new_product_development/purchase_task) | 是 | - |
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

- `GET`: Returns the frontend-oriented task list with projected `workflow`, aggregated `warehouse_status`, stable `product_selection` summary, `procurement_summary`, canonical actor/source fields `requester_id/requester_name`, `creator_id/creator_name`, `designer_id/designer_name`, `current_handler_id/current_handler_name`, and task org ownership fields `owner_team`, `owner_department`, and `owner_org_team`. Default ordering is latest updated first (`updated_at DESC, id DESC`). For `purchase_task`, `procurement_summary` carries procurement-to-warehouse coordination state plus lightweight product-selection provenance. Board queue `query_template` payloads are designed to be consumed directly by this endpoint. `workflow_lane` is the canonical list/workbench split selector for distinguishing the normal lane from the customization lane. Main task-flow list reads are globally visible to task-facing authenticated roles; use query filters for workbench/pool/tab slicing. Mutating actions such as assign/reassign, upload, submit, audit, filing, procurement, warehouse, close, and cancel remain action-gated by role, status, handler/assignee, and organization scope.
- `POST`: Creates one task. For `original_product_development`, narrow by category or `search_entry_code`, call `GET /v1/erp/products`, choose one product, and submit that result through `product_selection`. Legacy `product_id`, `sku_code`, and `product_name_snapshot` fields remain accepted for compatibility. Current create rules: - `original_product_development` is existing-product only. - when `product_id` is null, backend resolves ERP/local binding before create-tx using this priority: `product_id` -> `product_selection.erp_product.product_id` -> `product_selection.erp_product.sku_code` -> top-level `sku_code`. - ERP-side codes are treated as bridge binding keys and are normalized to a local `products.id`; they are not used as local primary keys directly. - frontend should not send `source_mode`; backend infers it from `task_type`. - `new_product_development` infers `source_mode=new_product` and auto-generates `sku_code` when omitted. - `purchase_task` no longer depends on design/audit assumptions at entry; creation initializes a draft procurement record so read models expose procurement state immediately. - `retouch_task` is a design-only image retouch task. It infers `source_mode=new_product`, does not require product binding, does not enter audit, and is completed immediately after the retouch/design worker submits the retouched image. - customization workflow is decoupled from ERP order-detail APIs; no ERP order-info matching/sync dependency is required at runtime. - `customization_required=true` is the canonical way to create a customization-lane task; that task enters `PendingCustomizationProduction` first as the compatible "waiting for customization operator design submission" state and does not pass through the normal design workbench. - legacy `is_outsource` / `need_outsource` create intent is folded into the same customization lane for compatibility, but new integrations must not use those fields as workflow selectors. - customization-lane create now also creates one primary `customization_job` immediately so `/v1/customization-jobs` visibility exists before review approval. - customization classification is business-configurable through `customization_level_code` and `customization_level_name`; do not assume fixed `A/B/C` levels. - default task product-code rule is backend-only: `sku_code_type=regular` generates `CG + category_short_code(1 uppercase letter) + 6-digit sequence`, while `sku_code_type=customization` generates `DZ + category_short_code(1 uppercase letter) + 6-digit sequence`; frontend no longer configures code-rules/rule-templates for task `sku_code` generation. - category short code generation priority is backend-owned: explicit map first (e.g. `KT_STANDARD -> K`), otherwise first alphabet letter from `category_code` (uppercased), then deterministic fallback to one letter. - sequence allocation for default task product-code uses `(prefix, category_short_code)` scope so different `category_code` values that collapse to one short code still remain unique. - `batch_sku_mode=multiple` is supported only for `new_product_development` and `purchase_task`; `original_product_development` returns `400 INVALID_REQUEST` with machine-readable `error.details.violations`. - batch Excel for `new_product_development` only requires each row's `产品名称` and `设计要求`; SKU/category internals are backend-owned. - batch mode writes one mother task plus multiple `task_sku_items` in one transaction and keeps `sku_code` / `primary_sku_code` aligned to the first child SKU for compatibility. - create now also appends `task.created`, and multi-SKU creates additionally append `task.batch_items_created`. - `reference_images` is no longer accepted. If present, backend returns `400 INVALID_REQUEST` and requires the reference-upload flow. - `reference_file_refs` must be objects returned by `POST /v1/tasks/reference-upload` or the compatibility task-create asset-center flow; forged, missing, incomplete, or unauthorized refs return `400 INVALID_REQUEST` with `invalid_reference_file_refs`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / 主流程读全量可见。
- `POST` 允许角色: Ops。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

#### GET 细节

##### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `status` | query | array<string> | 否 | Raw `task_status` filter. Supports comma-separated multi-value queries. |
| `task_type` | query | array<enum(original_product_development/new_product_development/purchase_task)> | 否 | Supports comma-separated multi-value queries. |
| `source_mode` | query | array<enum(existing_product/new_product)> | 否 | Supports comma-separated multi-value queries. |
| `workflow_lane` | query | array<enum(normal/customization)> | 否 | Filters list/workbench reads by canonical workflow lane. Supports comma-separated multi-value queries. |
| `main_status` | query | array<TaskMainStatus> | 否 | Filters by projected `workflow.main_status`. Supports comma-separated multi-value queries. |
| `sub_status_code` | query | array<TaskSubStatusCode> | 否 | Filters by projected `workflow.sub_status.*.code`. If `sub_status_scope` is omitted, the code is matched against all sub-status lanes. Supports comma-separated multi-value queries. |
| `sub_status_scope` | query | enum(design/audit/procurement/warehouse/customization/outsource/production) | 否 | Narrows `sub_status_code` matching to one sub-status lane. |
| `coordination_status` | query | array<ProcurementCoordinationStatus> | 否 | Filters by derived `procurement_summary.coordination_status`. Supports comma-separated multi-value queries. |
| `warehouse_prepare_ready` | query | boolean | 否 | Filters by derived warehouse handoff readiness. |
| `warehouse_receive_ready` | query | boolean | 否 | Filters by derived warehouse receive readiness. |
| `warehouse_blocking_reason_code` | query | array<string> | 否 | Filters tasks that currently contain any of the given `workflow.warehouse_blocking_reasons.code` values. Supports comma-separated multi-value queries. |
| `creator_id` | query | integer | 否 | - |
| `designer_id` | query | integer | 否 | - |
| `priority` | query | array<enum(low/normal/high/critical)> | 否 | Filters by task priority (`t.priority`). Supports comma-separated multi-value queries such as `priority=critical,high`. |
| `designer_empty` | query | boolean | 否 | When `true`, returns only tasks with no designer assignment (`designer_id` IS NULL or `0`). Use with `workflow_lane=customization` for customization-lane unassigned-artwork filtering; do not combine with `status=PendingAssign` for that case. |
| `need_outsource` | query | boolean | 否 | - |
| `overdue` | query | boolean | 否 | When `true`, filters `deadline_at < now` and excludes `Completed`/`Archived`/`Cancelled`; when `false`, returns the complement set. |
| `keyword` | query | string | 否 | Matches task no, task SKU, batch SKU, product name, owner department/team, task id, and related actor display names/usernames (creator, requester, designer, current handler). |
| `owner_department` | query | array<string> | 否 | Filters by canonical task owner department. Supports comma-separated multi-value queries. |
| `owner_org_team` | query | array<string> | 否 | Filters by canonical task owner org team. Supports comma-separated multi-value queries. |
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
      "sku_code": "...",
      "primary_sku_code": "..."
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
| 401 | UNAUTHENTICATED | - | 未登录、token 缺失或 token 过期。 |
| 403 | PERMISSION_DENIED | 见接口返回 | 角色、组织范围、字段级授权或流程状态不允许。 |
| 404 | NOT_FOUND | - | 资源不存在或当前用户不可见。 |
| 409 | CONFLICT | 见接口返回 | 状态竞态、重复操作或版本冲突。 |
| 422 | VALIDATION_ERROR | - | 请求参数或业务字段校验失败。 |

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
| `client_create_id` | string | 否 | Client-generated task-create idempotency key. Reusing the same key with the same payload returns the existing created task instead of allocating a new task number or SKU range. |
| `task_type` | enum(original_product_development/new_product_development/purchase_task/retouch_task) | 是 | - |
| `business_lane` | enum(normal/customization) | 否 | Canonical task lane selector (`normal` or `customization`). Drives audit-domain routing. |
| `workflow_lane` | enum(normal/customization) | 否 | Compatibility alias of `business_lane`. |
| `source_mode` | enum(existing_product/new_product) | 否 | - |
| `owner_team` | string | 是 | Required compatibility owner-team input. Supported `/v1/org/options` org-team values with deterministic task mappings may be normalized before validation and persisted into canonical ownership fields. Unsupported values return `invalid_owner_team`. |
| `owner_department` | string | 否 | Optional canonical task owner department hint. When provided with `owner_org_team` or a compatible org-team `owner_team`, backend validates consistency before create. |
| `owner_org_team` | string | 否 | Optional canonical task owner org-team hint. When omitted, backend may resolve it from `owner_team` when the mapping is deterministic. |
| `due_at` | string | 否 | - |
| `deadline_at` | string | 否 | - |
| `creator_id` | integer | 否 | - |
| `operator_group_id` | integer | 否 | - |
| `designer_id` | integer | 否 | - |
| `priority` | enum(low/normal/high/critical) | 否 | - |
| `is_outsource` | boolean | 否 | Compatibility-only legacy create flag. When true, backend normalizes the request into `customization_required=true`. |
| `customization_required` | boolean | 否 | Canonical creation-time customization lane selector. When true, task enters customization review directly, bypasses the normal design workbench, and immediately gets one primary `customization_job`. |
| `customization_source_type` | enum(new_product/existing_product) | 否 | Business source classification inside the customization lane; it does not select the lane by itself. |
| `reference_file_refs` | array<ReferenceFileRef> | 否 | Reference file ref objects returned by `POST /v1/tasks/reference-upload` or the compatibility upload flow. Object arrays are the formal contract. `POST /v1/tasks` rejects direct `reference_images` payloads with `400 INVALID_REQUEST`. |
| `remark` | string | 否 | - |
| `note` | string | 否 | - |
| `batch_sku_mode` | enum(single/multiple) | 否 | - |
| `sku_code_type` | enum(regular/customization) | 否 | Automatic SKU code type for generated new-product/purchase task SKUs. `regular` generates `CG` + one category letter + six digits; `customization` generates `DZ` + one category letter + six digits. Historical SKU strings are not rewritten. |
| `source_draft_id` | integer | 否 | Optional task draft source linkage. Source: V1_INFORMATION_ARCHITECTURE §3.5.9. |
| `batch_items` | array<CreateTaskBatchItem> | 否 | - |
| `product_id` | integer | 否 | - |
| `sku_code` | string | 否 | - |
| `product_name_snapshot` | string | 否 | - |
| `product_selection` | any | 否 | - |
| `change_request` | string | 否 | - |
| `category_code` | string | 否 | Backend-owned compatibility category/cost/SKU-prefix field. New frontend creation should prefer `i_id`; backend may resolve an internal `category_code` from `i_id` when possible. |
| `i_id` | string | 否 | Canonical frontend selector for product family/style. For new-product and purchase-task creation, pass a value selected from `GET /v1/erp/iids`. |
| `product_i_id` | string | 否 | Compatibility alias for `i_id`. |
| `material_mode` | enum(preset/other) | 否 | - |
| `material` | string | 否 | - |
| `material_other` | string | 否 | - |
| `new_sku` | string | 否 | - |
| `product_name` | string | 否 | - |
| `product_short_name` | string | 否 | - |
| `design_requirement` | string | 否 | - |
| `retouch_requirements` | array<CreateTaskRetouchRequirementItem> | 否 | Structured demand lines for `retouch_task` only. Other task types must omit this field or receive `400 INVALID_REQUEST` with violation code `field_not_allowed_for_task_type`. |
| `cost_price_mode` | enum(manual/template) | 否 | - |
| `cost_price` | number | 否 | - |
| `quantity` | integer | 否 | - |
| `base_sale_price` | number | 否 | - |
| `reference_link` | string | 否 | - |
| `sync_erp_on_create` | boolean | 否 | When true, backend schedules the ERP product upsert immediately after task creation using the minimal create-time payload (`product_name`, generated/provided sku, and `i_id`). The create response is not blocked by ERP Bridge/OpenWeb latency; later edits can enrich ERP data. |
| `purchase_sku` | string | 否 | - |
| `product_channel` | string | 否 | - |
| `demand_text` | string | 否 | - |
| `copy_text` | string | 否 | - |
| `style_keywords` | string | 否 | - |

##### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {}
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskReadModel | 否 | - |

##### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Validation error. Create-task validation may include machine-readable `error.details.violations` entries describing field-level contract mismatches, including unsupported batch mode, duplicate `batch_items`, mixed top-level single-SKU fields, and invalid batch item fields. |

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

- `GET`: Returns task-derived creator and designer options for the task center advanced filters. The source is historical task actor usage, not the user-management directory, so ordinary task roles can filter by people who appear in tasks without requiring `/v1/users` access.

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

- `GET`: Returns the root task record plus the current workflow snapshot, nullable `procurement`, frontend-friendly `procurement_summary`, full `product_selection` provenance, canonical actor/source fields `requester_id/requester_name`, `creator_id/creator_name`, `designer_id/designer_name`, `current_handler_id/current_handler_name`, compatibility alias `assignee_id/assignee_name`, task org ownership fields `owner_team`, `owner_department`, and `owner_org_team`, and cost-governance read models. `reference_file_refs` is the task-level reference-image summary field; for batch tasks, SKU-specific refs are returned on `sku_items[].reference_file_refs`. `design_assets` and `asset_versions` are the formal design-asset detail fields, and batch-task SKU scope is expressed by `scope_sku_code`. `matched_rule_governance` exposes matched-rule lineage context, `override_summary` is the lightweight current summary, `governance_audit_summary` points to the read-only override timeline, and `override_governance_boundary` exposes the current governance-boundary summary fields. `task_event_logs` remain the general task event layer. Use `/v1/tasks/{id}/detail` for the full aggregate page and `/v1/tasks/{id}/cost-overrides` for the read-only governance audit timeline. For `purchase_task`, `procurement_summary` carries arrival and warehouse handoff state plus lightweight product-selection provenance. Main task-flow read detail is globally visible to task-facing authenticated roles; mutating actions remain separately action-gated by role, status, handler/assignee, and organization scope.

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
- `GET` 允许角色: Ops, Designer, CustomizationOperator, CustomizationReviewer, Audit_A, Audit_B, Warehouse, Outsource, Admin, SuperAdmin, HRAdmin, OrgAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
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
    "task_status": "string",
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
    "task_status": "string",
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

## PATCH /v1/tasks/{id}/procurement

### 简介
支持方法: PATCH。

- `PATCH`: Creates or updates the dedicated procurement record used by `purchase_task` readiness, coordination summaries, and structured procurement sub-status. Status remains explicit and mutable, and `/v1/tasks/{id}/procurement/advance` provides the minimal lifecycle transition action. This write path requires both an allowed role and a matching minimum org or handler scope over canonical task ownership.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `PATCH` 允许角色: Ops, Warehouse, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
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
| `status` | enum(draft/prepared/in_progress/completed) | 是 | - |
| `procurement_price` | number | 否 | - |
| `quantity` | integer | 否 | - |
| `supplier_name` | string | 否 | - |
| `purchase_remark` | string | 否 | - |
| `expected_delivery_at` | string | 否 | - |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_id": 123,
    "status": "draft",
    "procurement_price": 12.3
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ProcurementRecord | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with task-action `deny_code` details when the actor is outside the allowed org scope. |
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |
| 409 | 见 `error.code` | 见 `deny_code` | Task is not a purchase task |

### curl 示例
```bash
curl -X PATCH https://api.example.com/v1/tasks/<id>/procurement \
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

## POST /v1/tasks/{id}/procurement/advance

### 简介
支持方法: POST。

- `POST`: Performs minimal procurement lifecycle transitions for `purchase_task`: `prepare`, `start`, `complete`, or `reopen`. This write path now also requires both an allowed role and a matching minimum org/handler scope over canonical task ownership.

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
| `operator_id` | integer | 否 | Optional override for compatibility. Defaults to the current authenticated actor. |
| `action` | enum(prepare/start/complete/reopen) | 是 | - |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_id": 123,
    "status": "draft",
    "procurement_price": 12.3
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | ProcurementRecord | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid action or missing required draft data |
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with task-action `deny_code` details when the actor is outside the allowed org scope. |
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |
| 409 | 见 `error.code` | 见 `deny_code` | Task is not a purchase task or the transition is invalid |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/procurement/advance \
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
      "source_mode": "...",
      "product_id": "..."
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

- `POST`: `POST /v1/tasks/{id}/assign` now carries bounded semantics under the same route: - `PendingAssign` (regular lane): assign is allowed for the existing operation/management path within the allowed org scope. A Designer may also self-claim an unassigned task by sending their own user id as `designer_id`; success sets `designer_id` and `current_handler_id`, then moves the task to `InProgress`. Target user must be an active `Designer`. - `PendingCustomizationProduction` (customization lane): Ops/Admin/SuperAdmin (and other existing assign scopes) may assign an active `CustomizationOperator` as `designer_id`. Success writes `designer_id` and `current_handler_id`, keeps `task_status` at `PendingCustomizationProduction`, and syncs the `customization` module to `in_progress` (not the `design` module). Pure `Designer` targets are rejected with `target_assignee_not_customization_operator`. Customization operators self-claim through `POST /v1/tasks/{id}/modules/customization/claim` instead of this route. - `InProgress` (regular lane): the same route acts as reassign. Allowed actors are requester/initiator (`requester_id` or `creator_id`), the current owning-group `TeamLead`, and scoped management roles (`DepartmentAdmin`, `DesignDirector`, `RoleAdmin`, `HRAdmin`, `SuperAdmin`, `Admin`). Ordinary Ops users without those conditions are denied. Target user must remain an active `Designer`. - Audit / warehouse / close states remain denied with machine-readable `PERMISSION_DENIED` details such as `task_not_reassignable`. - `purchase_task` cannot be assigned or reassigned to a designer.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
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
    "source_mode": "existing_product",
    "product_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | Task | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with machine-readable task-action details such as `missing_required_role`, `task_out_of_department_scope`, `task_out_of_team_scope`, `task_not_reassignable`, or `task_reassign_requires_requester_or_manager`. |
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |
| 409 | 见 `error.code` | 见 `deny_code` | Invalid task state such as attempting designer assignment on `purchase_task` |

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

## POST /v1/tasks/{id}/submit-design

### 简介
支持方法: POST。

- `POST`: Supports two submit modes: 1) Compatibility single-asset submit (`asset_type` + `file_name`) which creates one `task_assets` record. 2) Batch submit (`assets[]`) which completes multiple upload sessions in a single action and persists canonical `design_assets`/`asset_versions` with SKU scope. Re-entry is allowed from `RejectedByAuditA` and `RejectedByAuditB`. Delivery upload-session completion advances normal design task status to `PendingAuditA` when current status is one of `PendingAssign`, `Assigned`, `InProgress`, `RejectedByAuditA`, or `RejectedByAuditB`, and for multi-SKU batch tasks the gate waits until required SKU-scoped delivery assets are complete. Customization-lane tasks also use this endpoint: `CustomizationOperator` submits the customization design/delivery from `PendingCustomizationProduction`, the backend records/updates `last_customization_operator_id`, and successful delivery submission advances the task to `PendingCustomizationReview`. Customization tasks then continue through `POST /v1/tasks/{id}/customization/review`; they must not be sent through the normal audit endpoints. For `retouch_task`, delivery submission completes the task directly (`task_status=Completed`), clears `current_handler_id`, marks the `retouch` module completed, and never triggers audit. `purchase_task` cannot submit design. This action uses minimum role plus org or handler gating.

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
成功响应: `见 OpenAPI responses`

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
curl -X POST https://api.example.com/v1/tasks/<id>/submit-design \
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
- `POST` 允许角色: Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
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

- `GET`: Compatibility-only alias for `GET /v1/assets/upload-sessions/{session_id}`. Obsolete for frontend rollout.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Designer, Ops, Audit_A, Audit_B。
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
- `POST` 允许角色: Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
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
- `POST` 允许角色: Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
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

## POST /v1/tasks/{id}/assets/upload

### 简介
支持方法: POST。

- `POST`: **Deprecated browser contract:** `multipart/form-data` uploads (`file` + `file_role`) are **not supported** on this path and return **410** with `UPLOAD_ENDPOINT_DEPRECATED`. **Supported:** `application/json` body identical to `POST /v1/assets/upload-sessions` (backend-selected upload strategy). **Design drafts (source / delivery / preview):** use the unified asset upload-session contract, upload bytes using the returned remote plan, then call `.../complete`. Preferred entrypoint for new code: `/v1/assets/upload-sessions` instead of this legacy URL.

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
| 404 | 见 `error.code` | 见 `deny_code` | Task or asset not found |
| 410 | 见 `error.code` | 见 `deny_code` | `UPLOAD_ENDPOINT_DEPRECATED` when `Content-Type` is `multipart/form-data` or `application/x-www-form-urlencoded`. Use asset-center upload session JSON handoff + OSS upload + complete (see response `details`). |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/assets/upload \
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

## POST /v1/tasks/{id}/warehouse/prepare

### 简介
支持方法: POST。

- `POST`: Evaluates PRD-aligned warehouse readiness. Purchase tasks may enter the warehouse path without design/audit, but they must complete procurement arrival before warehouse handoff; design task types still require final asset and approved audit path. This action now uses minimum role + org scope gating over canonical task ownership.

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
| `operator_id` | integer | 否 | Optional override for compatibility. Defaults to the current authenticated actor. |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_no": "string",
    "source_mode": "existing_product",
    "product_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | Task | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with task-action `deny_code` details when the actor is outside the allowed org scope. |
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |
| 409 | 见 `error.code` | 见 `deny_code` | Warehouse blocking reasons prevent handoff |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/warehouse/prepare \
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

## POST /v1/tasks/{id}/close

### 简介
支持方法: POST。

- `POST`: Requires the task to be in explicit `PendingClose` mainline state. This action now uses minimum role + canonical-owner org gating instead of role name alone. `Admin`/`SuperAdmin`/`RoleAdmin`/`HRAdmin` may cross org scope but still cannot bypass the `PendingClose` status gate. `Ops`, `Warehouse`, and scoped management roles must still match the task canonical owner department/team scope. On readiness failure, `error.details` returns: - `task_type` - `workflow` - `closable` - `cannot_close_reasons`

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
| `operator_id` | integer | 否 | Optional override for compatibility. Defaults to the current authenticated actor. |
| `remark` | string | 否 | - |

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
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with task-action `deny_code` details when the actor is outside the allowed org scope. |
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |
| 409 | 见 `error.code` | 见 `deny_code` | Task is not ready to close; see `error.details.workflow` and `error.details.cannot_close_reasons` |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/close \
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

## POST /v1/tasks/{id}/audit/claim

### 简介
支持方法: POST。

- `POST`: `auditor_id` is optional and defaults to the current authenticated actor. This action uses minimum role plus org scope gating over canonical task ownership. `Audit_A` may only claim `PendingAuditA`; `Audit_B` may only claim `PendingAuditB`; when a current handler already exists, non-management actors must match that handler instead of taking over globally.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Audit_A, Audit_B, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `auditor_id` | integer | 否 | Optional override for compatibility. Defaults to the current authenticated actor. |
| `stage` | enum(A/B/outsource_review) | 是 | - |

### 响应体 schema
成功响应: `200`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with task-action `deny_code` details when the actor is outside the allowed org scope. |
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |
| 409 | 见 `error.code` | 见 `deny_code` | Invalid task state |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/audit/claim \
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

## POST /v1/tasks/{id}/audit/approve

### 简介
支持方法: POST。

- `POST`: `auditor_id` is optional and defaults to the current authenticated actor. Approval clears the current audit handler so the next audit or warehouse stage must be explicitly claimed or received. This action uses minimum role plus org or handler gating over canonical task ownership. `Audit_A` can only approve stage A (`PendingAuditA`), `Audit_B` can only approve stage B (`PendingAuditB`), and non-management actors must be the current handler.

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
成功响应: `见 OpenAPI responses`

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
curl -X POST https://api.example.com/v1/tasks/<id>/audit/approve \
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

## POST /v1/tasks/{id}/audit/reject

### 简介
支持方法: POST。

- `POST`: `auditor_id` is optional and defaults to the current authenticated actor. Audit rejection routes the task back to the designer handler so rework is explicit. This action uses minimum role plus org or handler gating over canonical task ownership. `Audit_A` can only reject stage A, `Audit_B` can only reject stage B, and non-management actors must be the current handler.

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
成功响应: `见 OpenAPI responses`

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
curl -X POST https://api.example.com/v1/tasks/<id>/audit/reject \
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

## POST /v1/tasks/{id}/audit/transfer

### 简介
支持方法: POST。

- `POST`: `from_auditor_id` is optional and defaults to the current authenticated actor. This action uses minimum role plus org or handler gating over canonical task ownership. `Audit_A` can only transfer stage A, `Audit_B` can only transfer stage B, and non-management actors must currently own the handler slot.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Audit_A, Audit_B, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `from_auditor_id` | integer | 否 | Optional override for compatibility. Defaults to the current authenticated actor. |
| `to_auditor_id` | integer | 是 | - |
| `stage` | enum(A/B/outsource_review) | 是 | - |
| `comment` | string | 否 | - |

### 响应体 schema
成功响应: `200`

无 JSON 响应体或响应体由文件流承载。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with task-action `deny_code` details when the actor is outside the allowed org scope. |
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |
| 409 | 见 `error.code` | 见 `deny_code` | Invalid task state |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/audit/transfer \
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

- `POST`: Create audit handover

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
    "id": 123,
    "handover_no": "string",
    "task_id": 123,
    "from_auditor_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AuditHandover | 否 | - |

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
curl -X POST https://api.example.com/v1/tasks/<id>/audit/handover \
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

## GET /v1/tasks/{id}/audit/handovers

### 简介
支持方法: GET。

- `GET`: Ordered by `created_at DESC`. This endpoint keeps the pre-Step-05 `data`-only list shape.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Audit_A, Audit_B, Admin。
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
| `data` | array<AuditHandover> | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |

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

- `POST`: `auditor_id` is optional and defaults to the current authenticated actor. Takeover restores explicit audit ownership by setting the task handler to the takeover auditor. This action uses minimum role plus org scope gating over canonical task ownership. `Audit_A` can only take over stage A handovers, `Audit_B` can only take over stage B handovers, and management roles remain state-gated.

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
成功响应: `见 OpenAPI responses`

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
curl -X POST https://api.example.com/v1/tasks/<id>/audit/takeover \
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

## POST /v1/tasks/{id}/outsource

### 简介
支持方法: POST。

- `POST`: Compatibility-only legacy late-branch entry retained for historical tasks. New integrations must create customization-lane tasks with `customization_required=true` instead of using this route.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Outsource, Ops, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `operator_id` | integer | 是 | - |
| `vendor_name` | string | 是 | - |
| `outsource_type` | string | 是 | - |
| `delivery_requirement` | string | 否 | - |
| `settlement_note` | string | 否 | - |

### 响应体 schema
成功响应: `201 application/json`

```json
{
  "data": {
    "id": 123,
    "outsource_no": "string",
    "task_id": 123,
    "vendor_name": "string"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | OutsourceOrder | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |
| 409 | 见 `error.code` | 见 `deny_code` | Task not in PendingOutsource |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/outsource \
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

## GET /v1/outsource-orders

### 简介
支持方法: GET。

- `GET`: Compatibility-only legacy list for historical late-branch outsource records. New integrations should use `/v1/customization-jobs` for the unified customization lane. `vendor` is matched with fuzzy `LIKE` against `vendor_name`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Outsource, Ops, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `task_id` | query | integer | 否 | - |
| `status` | query | string | 否 | - |
| `vendor` | query | string | 否 | - |
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
      "outsource_no": "...",
      "task_id": "...",
      "vendor_name": "..."
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
| `data` | array<OutsourceOrder> | 否 | - |
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
curl -X GET https://api.example.com/v1/outsource-orders \
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

## GET /v1/warehouse/receipts

### 简介
支持方法: GET。

- `GET`: Returns paginated warehouse receipts. `receiver_id` filters against the current receipt owner/receiver.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: Warehouse, Ops, Admin。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `task_id` | query | integer | 否 | - |
| `status` | query | enum(received/rejected/completed) | 否 | - |
| `receiver_id` | query | integer | 否 | - |
| `workflow_lane` | query | enum(normal/customization) | 否 | - |
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
      "task_id": "...",
      "receipt_no": "...",
      "workflow_lane": "..."
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
| `data` | array<WarehouseReceipt> | 否 | - |
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
curl -X GET https://api.example.com/v1/warehouse/receipts \
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
| `task_type` | query | array<enum(original_product_development/new_product_development/purchase_task)> | 否 | Applies the same task-list filter semantics as `/v1/tasks`. Supports comma-separated multi-value queries. |
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
| `task_type` | query | array<enum(original_product_development/new_product_development/purchase_task)> | 否 | Applies the same task-list filter semantics as `/v1/tasks`. Supports comma-separated multi-value queries. |
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

## POST /v1/tasks/{id}/warehouse/receive

### 简介
支持方法: POST。

- `POST`: Mark warehouse receipt as received

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
    "id": 123,
    "task_id": 123,
    "receipt_no": "string",
    "workflow_lane": "normal"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | WarehouseReceipt | 否 | - |

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
curl -X POST https://api.example.com/v1/tasks/<id>/warehouse/receive \
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

## POST /v1/tasks/{id}/warehouse/reject

### 简介
支持方法: POST。

- `POST`: Reject warehouse receipt

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
    "id": 123,
    "task_id": 123,
    "receipt_no": "string",
    "workflow_lane": "normal"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | WarehouseReceipt | 否 | - |

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
curl -X POST https://api.example.com/v1/tasks/<id>/warehouse/reject \
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

## POST /v1/tasks/{id}/warehouse/complete

### 简介
支持方法: POST。

- `POST`: Requires a prior receive record, and the task must have `sku_code`. This endpoint moves the task into explicit `PendingClose` rather than closing it directly. `receiver_id` is optional and defaults to the current authenticated actor. This action uses minimum role plus org or handler gating over canonical task ownership. Non-management actors must be the current warehouse handler.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: Warehouse, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `receiver_id` | integer | 否 | Optional override for compatibility. Defaults to the current authenticated actor. |
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_id": 123,
    "receipt_no": "string",
    "workflow_lane": "normal"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | WarehouseReceipt | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Task missing sku_code |
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` with `deny_code` such as `task_out_of_department_scope`, `task_out_of_team_scope`, `task_not_assigned_to_actor`, or `task_status_not_actionable`. |
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |
| 409 | 见 `error.code` | 见 `deny_code` | Invalid task or warehouse state |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/warehouse/complete \
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

## POST /v1/tasks/{id}/customization/review

### 简介
支持方法: POST。

- `POST`: Dedicated customization reviewer entry. The primary `customization_job` is created at task creation. Review may optionally write business-entered review reference data on that record (`customization_level_code`, `customization_level_name`, `review_reference_unit_price`, `review_reference_weight_factor`, `customization_note`); approved decisions do not require level or pricing fields. Customization tasks reach this endpoint after `CustomizationOperator` submits design through `POST /v1/tasks/{id}/submit-design` and the task enters `PendingCustomizationReview`. `return_to_designer` returns the task to `PendingCustomizationProduction` for customization-operator rework, preferring `last_customization_operator_id` and falling back to `designer_id` for historical tasks. Approved customization reviews enter the warehouse chain through `PendingWarehouseReceive`. Review does not freeze execution settlement pricing.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: CustomizationReviewer, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `reviewer_id` | integer | 否 | - |
| `source_asset_id` | integer | 否 | - |
| `customization_level_code` | string | 否 | Optional business-entered review reference level code. Omitted for lightweight approved reviews. |
| `customization_level_name` | string | 否 | Optional business-entered review reference level name. Omitted for lightweight approved reviews. |
| `customization_price` | number | 否 | Business-entered review reference unit price. Not the execution freeze snapshot. |
| `customization_weight_factor` | number | 否 | Business-entered review reference weight factor. Not the execution freeze snapshot. |
| `customization_note` | string | 否 | Reviewer-entered business note for this review record. |
| `customization_review_decision` | enum(approved/return_to_designer/reviewer_fixed) | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {}
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | any | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid customization_review_decision |
| 409 | 见 `error.code` | 见 `deny_code` | Invalid workflow state transition |
| 403 | 见 `error.code` | 见 `deny_code` | Permission denied |
| 404 | 见 `error.code` | 见 `deny_code` | Task not found |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/customization/review \
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

## GET /v1/customization-jobs

### 简介
支持方法: GET。

- `GET`: Lists customization work records without ERP order-detail dependency, including tasks that just entered the customization lane at creation time, pricing snapshot fields (`pricing_worker_type`, `unit_price`, `weight_factor`), current effective稿 tracking, and stored `order_no` trace.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: CustomizationReviewer, CustomizationOperator, Ops, Designer, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `task_id` | query | integer | 否 | - |
| `status` | query | string | 否 | - |
| `operator_id` | query | integer | 否 | - |
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
      "task_id": "...",
      "source_asset_id": "...",
      "current_asset_id": "..."
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
| `data` | array<CustomizationJob> | 否 | - |
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
curl -X GET https://api.example.com/v1/customization-jobs \
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

## GET /v1/customization-jobs/{id}

### 简介
支持方法: GET。

- `GET`: Get customization job detail

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: CustomizationReviewer, CustomizationOperator, Ops, Designer, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
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
    "source_asset_id": 123,
    "current_asset_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CustomizationJob | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 404 | 见 `error.code` | 见 `deny_code` | Customization job not found |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/customization-jobs/<id> \
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

## POST /v1/customization-jobs/{id}/effect-preview

### 简介
支持方法: POST。

- `POST`: Customization-operator work entry. The first successful submission freezes pricing snapshot fields by `(employment_type + customization_level_code)` into `pricing_worker_type`, `unit_price`, and `weight_factor`. `decision_type=effect_preview` enters second review, while `decision_type=final` skips effect review and advances directly to production transfer.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: CustomizationOperator, Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
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
| `current_asset_id` | integer | 否 | - |
| `order_no` | string | 否 | - |
| `decision_type` | enum(final/effect_preview) | 否 | - |
| `note` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_id": 123,
    "source_asset_id": 123,
    "current_asset_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CustomizationJob | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Missing or invalid pricing rule snapshot input |
| 409 | 见 `error.code` | 见 `deny_code` | Invalid workflow state transition |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/customization-jobs/<id>/effect-preview \
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

## POST /v1/customization-jobs/{id}/effect-review

### 简介
支持方法: POST。

- `POST`: Effect review only accepts jobs in `pending_effect_review`; `return_to_designer` sends workflow back to effect revision, and `reviewer_fixed` may replace the effective working稿 through `current_asset_id` before advancing to production transfer.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: CustomizationReviewer, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | path | integer | 是 | - |

Content-Type: `application/json`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `reviewer_id` | integer | 否 | - |
| `current_asset_id` | integer | 否 | - |
| `customization_review_decision` | enum(approved/return_to_designer/reviewer_fixed) | 否 | - |
| `customization_level_code` | string | 否 | - |
| `customization_level_name` | string | 否 | - |
| `customization_price` | number | 否 | - |
| `customization_weight_factor` | number | 否 | - |
| `customization_note` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_id": 123,
    "source_asset_id": 123,
    "current_asset_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CustomizationJob | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 409 | 见 `error.code` | 见 `deny_code` | Invalid workflow state transition |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/customization-jobs/<id>/effect-review \
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

## POST /v1/customization-jobs/{id}/production-transfer

### 简介
支持方法: POST。

- `POST`: Production transfer requires `pending_production_transfer` job status, updates task last customization operator snapshot for warehouse reject backflow, and records bounded transfer trace fields for later robot or system integration.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: CustomizationOperator, Designer, Ops, Admin, SuperAdmin, HRAdmin, RoleAdmin, DepartmentAdmin, TeamLead, DesignDirector。
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
| `current_asset_id` | integer | 否 | - |
| `transfer_channel` | string | 否 | - |
| `transfer_reference` | string | 否 | - |
| `note` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_id": 123,
    "source_asset_id": 123,
    "current_asset_id": 123
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | CustomizationJob | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 409 | 见 `error.code` | 见 `deny_code` | Invalid workflow state transition |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/customization-jobs/<id>/production-transfer \
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

## POST /v1/audit

### 简介
支持方法: POST。

- `POST`: [V6] Submit audit decision

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
    "action": "string",
    "jobs": [
      "..."
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | AuditSubmitResult | 否 | - |

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
curl -X POST https://api.example.com/v1/audit \
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

- `GET`: [V6] Get rule template by type
- `PUT`: [V6] Upsert rule template by type

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
| `type` | path | string | 是 | - |

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
| `type` | path | string | 是 | - |

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

- `POST`: Trigger a task module action

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

- `GET`: Downloads the Excel assist workbook for creating one task at a time with `mode=single`. `task_type=new_product_development` columns: `产品款式编码`, `产品名称`, `设计要求` (required); optional `规格尺寸`, `材质`, `材质备注`, `备注`. `task_type=purchase_task` columns: `产品款式编码`, `产品名称`, `数量`, `规格尺寸` (required); optional `备注`. `task_type=original_product_development` columns: `SKU编码`, `修改要求` (required); optional `规格尺寸`, `备注`. Product name and category are enriched from ERP during `parse-excel`, not collected in the template. The workbook has no sample data rows; `parse-excel` rejects more than one non-empty data row.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。

### 请求体 schema
参数:

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `task_type` | query | enum(new_product_development/purchase_task/original_product_development) | 是 | - |
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

- `POST`: Parses a single-task Excel assist upload into a `draft` plus row-level `violations`. Does not create tasks. `mode` must be `single`. For `new_product_development`, required columns: `产品款式编码`, `产品名称`, `设计要求`. For `purchase_task`, required: `产品款式编码`, `产品名称`, `数量` (positive integer), `规格尺寸`; optional `备注`. For `original_product_development`, required: `SKU编码`, `修改要求`; optional `规格尺寸`, `备注`. Parsed `sku_code` values are resolved through ERP product search; unknown SKU returns `product_not_found`. More than one non-empty data row returns `multiple_rows_not_allowed`. Invalid quantity returns `invalid_quantity`. Parsed `product_i_id` values (new/purchase only) are validated against ERP i_id options when configured.

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
| `task_type` | enum(new_product_development/purchase_task/original_product_development) | 是 | - |
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
    "order_no": "string"
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

- `POST`: Manager-only file maintenance operation. The backend copies objects into the target upload directory prefix, updates directory snapshots, records an event, and best-effort removes old objects.

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

- `POST`: Soft-deletes unsettled files, refreshes submission totals, and reprices or voids the related item. Submitters may delete only their own files; managers may delete any unlocked file.

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

## POST /v1/asset-workbench/settlement/supplements/excel

### 简介
支持方法: POST。

- `POST`: Batch-creates approved supplement rows for a business month. Each row still checks payee/month supplement permission and duplicate hints.

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

- `POST`: Creates a direct-upload session. When upload directories are configured, `upload_directory_id` is required and the session stores the directory name, prefix, and difficulty snapshot.

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

- `POST`: Complete asset workbench upload session

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
- `POST`: Publishes an existing system or external asset by source reference. Existing `asset_id` remains supported for system assets; external assets use `source_type=external` with `source_ref`/`resource_id` such as `ext-123`. Clients can download only published enabled rows.

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
| `source_type` | enum(system/external) | 否 | - |
| `source_ref` | string | 否 | - |
| `resource_id` | string | 否 | - |
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
    "source_type": "system",
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
| `source_type` | enum(system/external) | 否 | - |
| `source_ref` | string | 否 | - |
| `resource_id` | string | 否 | - |
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
    "source_type": "system",
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

- `GET`: Returns a direct download manifest only when the material is published and enabled for clients.

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
    "filename": "string",
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
    "source_type": "system",
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

