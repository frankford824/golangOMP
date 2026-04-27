# 任务主流程

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

任务创建、列表、详情、模块动作、分派、取消、归档与工作流操作。

## Family 约定

- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 本文件覆盖 `95` 个 `/v1` path；同一路径多 method 合并在同一节。

## POST /v1/tasks/prepare-product-codes

### 简介
支持方法: POST。

- `POST`: Allocates unique default product codes for task-create UIs. Default format is fixed to `NS + {CATEGORY_CODE} + {6-digit sequence}`. This endpoint does not require frontend code-rule/template selection and is available for `new_product_development` and `purchase_task`.

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
| `task_type` | enum(new_product_development/purchase_task) | 是 | - |
| `category_code` | string | 否 | Required when `batch_items` is omitted. |
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks

### 简介
支持方法: GET, POST。

- `GET`: Returns the frontend-oriented task list with projected `workflow`, aggregated `warehouse_status`, stable `product_selection` summary, `procurement_summary`, canonical actor/source fields `requester_id/requester_name`, `creator_id/creator_name`, `designer_id/designer_name`, `current_handler_id/current_handler_name`, and task org ownership fields `owner_team`, `owner_department`, and `owner_org_team`. For `purchase_task`, `procurement_summary` carries procurement-to-warehouse coordination state plus lightweight product-selection provenance. Board queue `query_template` payloads are designed to be consumed directly by this endpoint. `workflow_lane` is the canonical list/workbench split selector for distinguishing the normal lane from the customization lane. List reads apply the current canonical-org visibility rules plus stage-based visibility for mid-lane roles (`Audit_A`, `Audit_B`, `Warehouse`, `Outsource`, `CustomizationOperator`, `CustomizationReviewer`) where the backend-authorized status set and lane match. This is not a full ABAC engine.
- `POST`: Creates one task. For `original_product_development`, narrow by category or `search_entry_code`, call `GET /v1/erp/products`, choose one product, and submit that result through `product_selection`. Legacy `product_id`, `sku_code`, and `product_name_snapshot` fields remain accepted for compatibility. Current create rules: - `original_product_development` is existing-product only. - when `product_id` is null, backend resolves ERP/local binding before create-tx using this priority: `product_id` -> `product_selection.erp_product.product_id` -> `product_selection.erp_product.sku_code` -> top-level `sku_code`. - ERP-side codes are treated as bridge binding keys and are normalized to a local `products.id`; they are not used as local primary keys directly. - frontend should not send `source_mode`; backend infers it from `task_type`. - `new_product_development` infers `source_mode=new_product` and auto-generates `sku_code` when omitted. - `purchase_task` no longer depends on design/audit assumptions at entry; creation initializes a draft procurement record so read models expose procurement state immediately. - customization workflow is decoupled from ERP order-detail APIs; no ERP order-info matching/sync dependency is required at runtime. - `customization_required=true` is the canonical way to create a customization-lane task; that task enters `PendingCustomizationReview` immediately and does not pass through the normal design workbench first. - legacy `is_outsource` / `need_outsource` create intent is folded into the same customization lane for compatibility, but new integrations must not use those fields as workflow selectors. - customization-lane create now also creates one primary `customization_job` immediately so `/v1/customization-jobs` visibility exists before review approval. - customization classification is business-configurable through `customization_level_code` and `customization_level_name`; do not assume fixed `A/B/C` levels. - default task product-code rule is backend-only and fixed to `NS + category_short_code(2 uppercase letters) + 6-digit sequence`; frontend no longer configures code-rules/rule-templates for task `sku_code` generation. - category short code generation priority is backend-owned: explicit map first (e.g. `KT_STANDARD -> KT`), otherwise first two alphabet letters from `category_code` (uppercased), then deterministic fallback to two letters. - sequence allocation for default task product-code uses `(prefix, category_short_code)` scope so different `category_code` values that collapse to one short code still remain unique. - `batch_sku_mode=multiple` is supported only for `new_product_development` and `purchase_task`; `original_product_development` returns `400 INVALID_REQUEST` with machine-readable `error.details.violations`. - batch mode writes one mother task plus multiple `task_sku_items` in one transaction and keeps `sku_code` / `primary_sku_code` aligned to the first child SKU for compatibility. - create now also appends `task.created`, and multi-SKU creates additionally append `task.batch_items_created`. - `reference_images` is no longer accepted. If present, backend returns `400 INVALID_REQUEST` and requires the reference-upload flow. - `reference_file_refs` must be objects returned by `POST /v1/tasks/reference-upload` or the compatibility task-create asset-center flow; forged, missing, incomplete, or unauthorized refs return `400 INVALID_REQUEST` with `invalid_reference_file_refs`.

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
| `need_outsource` | query | boolean | 否 | - |
| `overdue` | query | boolean | 否 | When `true`, filters `deadline_at < now` and excludes `Completed`/`Archived`/`Cancelled`; when `false`, returns the complement set. |
| `keyword` | query | string | 否 | Matches `task_no`, `sku_code`, or `product_name_snapshot`. |
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
| `task_type` | enum(original_product_development/new_product_development/purchase_task/retouch_task/customer_customization/regular_customization) | 是 | - |
| `source_mode` | enum(existing_product/new_product) | 否 | - |
| `owner_team` | string | 是 | Required compatibility owner-team input. Supported `/v1/org/options` org-team values with deterministic task mappings may be normalized before validation and persisted into canonical ownership fields. Unsupported values return `invalid_owner_team`. |
| `owner_department` | string | 否 | Optional canonical task owner department hint. When provided with `owner_org_team` or a compatible org-team `owner_team`, backend validates consistency before create. |
| `owner_org_team` | string | 否 | Optional canonical task owner org-team hint. When omitted, backend may resolve it from `owner_team` when the mapping is deterministic. |
| `due_at` | string | 否 | - |
| `deadline_at` | string | 否 | - |
| `creator_id` | integer | 否 | - |
| `operator_group_id` | integer | 否 | - |
| `designer_id` | integer | 否 | - |
| `priority` | enum(low/normal/high/urgent/critical) | 否 | - |
| `is_outsource` | boolean | 否 | Compatibility-only legacy create flag. When true, backend normalizes the request into `customization_required=true`. |
| `customization_required` | boolean | 否 | Canonical creation-time customization lane selector. When true, task enters customization review directly, bypasses the normal design workbench, and immediately gets one primary `customization_job`. |
| `customization_source_type` | enum(new_product/existing_product) | 否 | Business source classification inside the customization lane; it does not select the lane by itself. |
| `reference_file_refs` | array<ReferenceFileRef> | 否 | Reference file ref objects returned by `POST /v1/tasks/reference-upload` or the compatibility upload flow. Object arrays are the formal contract. `POST /v1/tasks` rejects direct `reference_images` payloads with `400 INVALID_REQUEST`. |
| `remark` | string | 否 | - |
| `note` | string | 否 | - |
| `batch_sku_mode` | enum(single/multiple) | 否 | - |
| `source_draft_id` | integer | 否 | Optional task draft source linkage. Source: V1_INFORMATION_ARCHITECTURE §3.5.9. |
| `batch_items` | array<CreateTaskBatchItem> | 否 | - |
| `product_id` | integer | 否 | - |
| `sku_code` | string | 否 | - |
| `product_name_snapshot` | string | 否 | - |
| `product_selection` | any | 否 | - |
| `change_request` | string | 否 | - |
| `category_code` | string | 否 | Required for `batch_sku_mode=multiple` on both `new_product_development` and `purchase_task`. |
| `material_mode` | enum(preset/other) | 否 | - |
| `material` | string | 否 | - |
| `material_other` | string | 否 | - |
| `new_sku` | string | 否 | - |
| `product_name` | string | 否 | - |
| `product_short_name` | string | 否 | - |
| `design_requirement` | string | 否 | - |
| `cost_price_mode` | enum(manual/template) | 否 | - |
| `cost_price` | number | 否 | - |
| `quantity` | integer | 否 | - |
| `base_sale_price` | number | 否 | - |
| `reference_link` | string | 否 | - |
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}

### 简介
支持方法: GET。

- `GET`: Returns the root task record plus the current workflow snapshot, nullable `procurement`, frontend-friendly `procurement_summary`, full `product_selection` provenance, canonical actor/source fields `requester_id/requester_name`, `creator_id/creator_name`, `designer_id/designer_name`, `current_handler_id/current_handler_name`, compatibility alias `assignee_id/assignee_name`, task org ownership fields `owner_team`, `owner_department`, and `owner_org_team`, and cost-governance read models. `reference_file_refs` is the task-level reference-image summary field; for batch tasks, SKU-specific refs are returned on `sku_items[].reference_file_refs`. `design_assets` and `asset_versions` are the formal design-asset detail fields, and batch-task SKU scope is expressed by `scope_sku_code`. `matched_rule_governance` exposes matched-rule lineage context, `override_summary` is the lightweight current summary, `governance_audit_summary` points to the read-only override timeline, and `override_governance_boundary` exposes the current governance-boundary summary fields. `task_event_logs` remain the general task event layer. Use `/v1/tasks/{id}/detail` for the full aggregate page and `/v1/tasks/{id}/cost-overrides` for the read-only governance audit timeline. For `purchase_task`, `procurement_summary` carries arrival and warehouse handoff state plus lightweight product-selection provenance. This read path applies the current minimum task-action org gating. This is not a full ABAC engine.

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
- `GET` 允许角色: 已登录 / scope-aware。
- `PATCH` 允许角色: 已登录 / scope-aware。
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
    "product_name_snapshot": "string",
    "product_selection": {
      "selected_product_id": "...",
      "selected_product_name": "...",
      "selected_product_sku_code": "...",
      "matched_category_code": "..."
    }
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
| `product_selection` | TaskProductSelectionContext | 否 | Full original-product provenance contract for task read and detail views. It extends the lightweight summary with the local matched mapping snapshot and an additive ERP Bridge product snapshot. |
| `category` | string | 否 | - |
| `category_id` | integer | 否 | - |
| `category_code` | string | 否 | - |
| `spec_text` | string | 否 | - |
| `material` | string | 否 | - |
| `size_text` | string | 否 | - |
| `reference_link` | string | 否 | - |
| `reference_file_refs` | array<ReferenceFileRef> | 否 | - |
| `note` | string | 否 | - |
| `remark` | string | 否 | - |

##### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_id": 123,
    "task_no": "string",
    "task_type": "original_product_development"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskDetail | 否 | - |

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
- `GET` 允许角色: 已登录 / scope-aware。
- `PATCH` 允许角色: 已登录 / scope-aware。
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
    "task_no": "string",
    "task_type": "original_product_development"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskDetail | 否 | - |

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/cost-quote/preview

### 简介
支持方法: POST。

- `POST`: Runs cost-rule preview using task defaults plus optional request overrides.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/tasks/{id}/business-info

### 简介
支持方法: PATCH。

- `PATCH`: Maintains PRD V2.0 front-loaded category/spec/cost/filed information used by warehouse handoff and close-readiness checks. When category plus minimal width/height/area/quantity/process inputs are present, the backend also triggers skeleton cost preview and persists `estimated_cost`, rule provenance, governed `matched_rule_version`, and manual-review state. Existing-product tasks may also persist or rebind `product_selection` here so the selected product stays traceable back to local mapped-search provenance and optional ERP Bridge external snapshot fields. Filing now uses backend state-machine auto triggers and idempotent payload comparison. Legacy `trigger_filing` and `filed_at` remain compatibility forced triggers. Bridge remains the ERP/JST adapter and mutation executor; MAIN decides business boundary and records filing traces/status. `cost_price` is the current effective internal cost, while `manual_cost_override` distinguishes business-side override from system prefill; `prefill_source`, `prefill_at`, `override_actor`, and `override_at` provide governance trace, and override state changes append a dedicated `cost_override_events` audit record. This remains a narrow filing/master-data boundary only, not a broad ERP docking, approval flow, finance system, procurement/WMS integration, or raw ERP mutation API family on MAIN. Historical tasks are not auto-recomputed by later rule changes; new rule changes affect future preview/prefill only. Procurement preparation is maintained separately via `/v1/tasks/{id}/procurement`.

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
| `operator_id` | integer | 否 | Optional override for compatibility. Defaults to the current authenticated actor. |
| `category` | string | 否 | - |
| `category_id` | integer | 否 | - |
| `category_code` | string | 否 | - |
| `spec_text` | string | 否 | - |
| `material` | string | 否 | - |
| `size_text` | string | 否 | - |
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
| `remark` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {
    "id": 123,
    "task_id": 123,
    "task_no": "string",
    "task_type": "original_product_development"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskDetail | 否 | - |

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/filing-status

### 简介
支持方法: GET。

- `GET`: Returns filing state-machine status, missing fields, and retry hints for frontend display.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/filing/retry

### 简介
支持方法: POST。

- `POST`: Forces one filing retry attempt using current task payload snapshot and updates filing status fields. This write path now also requires both an allowed role and a matching minimum org scope over canonical task ownership.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## PATCH /v1/tasks/{id}/procurement

### 简介
支持方法: PATCH。

- `PATCH`: Creates or updates the dedicated procurement record used by `purchase_task` readiness, coordination summaries, and structured procurement sub-status. Status remains explicit and mutable, and `/v1/tasks/{id}/procurement/advance` provides the minimal lifecycle transition action. This write path requires both an allowed role and a matching minimum org or handler scope over canonical task ownership.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/procurement/advance

### 简介
支持方法: POST。

- `POST`: Performs minimal procurement lifecycle transitions for `purchase_task`: `prepare`, `start`, `complete`, or `reopen`. This write path now also requires both an allowed role and a matching minimum org/handler scope over canonical task ownership.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/detail

### 简介
支持方法: GET。

- `GET`: Returns the frontend aggregate detail. `/v1/tasks/{id}` exposes the lighter read model with workflow, `procurement_summary`, top-level `product_selection`, and canonical task org ownership. This detail route adds aggregate sections including governance summaries, design-asset detail, and supporting read models for the full page. `task_event_logs` remain the general task event stream; the override audit layer is governance-specific and read-only; the boundary layer is a placeholder governance summary rather than an approval or finance system. For `purchase_task`, `procurement_summary` remains the procurement-to-warehouse coordination summary and carries lightweight `product_selection`, while top-level and nested task-detail `product_selection` keep the full provenance contract.

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
    "task_no": "string",
    "task_type": "original_product_development"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | TaskDetail | 否 | - |

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/cost-overrides

### 简介
支持方法: GET。

- `GET`: Returns the dedicated read-only cost override and governance audit timeline for one task. This timeline records override-specific audit facts such as previous estimated cost, override cost, matched rule and version context, actor and time, and release events. `override_governance_boundary` reuses the same boundary summary object exposed by task, detail, and procurement reads. `task_event_logs` remain the general task event stream and coexist with this governance-specific audit layer. This endpoint is not an approval flow, finance system, accounting contract, or ERP writeback contract.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/cost-overrides/{event_id}/review

### 简介
支持方法: POST。

- `POST`: Adds or updates the approval-side placeholder boundary for one dedicated `cost_override_events` row. This is a skeleton governance handoff only; it is not a real approval workflow, identity approval chain, or permission model.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/cost-overrides/{event_id}/finance-mark

### 简介
支持方法: POST。

- `POST`: Adds or updates the finance-side placeholder boundary for one dedicated `cost_override_events` row. This is a future finance-handoff skeleton only; it is not a real finance system, ledger, reconciliation, settlement, invoice, or ERP writeback interface.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/assign

### 简介
支持方法: POST。

- `POST`: `POST /v1/tasks/{id}/assign` now carries two bounded semantics under the same route: - `PendingAssign`: assign is allowed for the existing operation/management path within the allowed org scope. Success sets `designer_id` and `current_handler_id`, then moves the task to `InProgress`. - `InProgress`: the same route acts as reassign. Allowed actors are requester/initiator (`requester_id` or `creator_id`), the current owning-group `TeamLead`, and scoped management roles (`DepartmentAdmin`, `DesignDirector`, `RoleAdmin`, `HRAdmin`, `SuperAdmin`, `Admin`). Ordinary Ops users without those conditions are denied. - Audit / warehouse / close states remain denied with machine-readable `PERMISSION_DENIED` details such as `task_not_reassignable`. - `purchase_task` cannot be assigned or reassigned to a designer.

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
| `designer_id` | integer | 否 | Designer/assignee user id. Omit or send null on a single-task reassign to clear the assignee and return an InProgress task to PendingAssign. |
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/batch/assign

### 简介
支持方法: POST。

- `POST`: Assigns multiple tasks in one request and returns assignment results.

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
| `task_ids` | array<integer> | 是 | - |
| `designer_id` | integer | 是 | - |
| `reason` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {}
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload |
| 403 | 见 `error.code` | 见 `deny_code` | `PERMISSION_DENIED` when one or more tasks fall outside the actor org scope. |
| 409 | 见 `error.code` | 见 `deny_code` | One or more tasks in invalid state |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/batch/assign \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/batch/remind

### 简介
支持方法: POST。

- `POST`: Sends reminders for multiple tasks and emits reminder events with traceable batch request id.

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
| `task_ids` | array<integer> | 是 | - |
| `reason` | string | 否 | - |

### 响应体 schema
成功响应: `200 application/json`

```json
{
  "data": {}
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `data` | object | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 400 | 见 `error.code` | 见 `deny_code` | Invalid request payload |
| 409 | 见 `error.code` | 见 `deny_code` | One or more tasks in invalid state |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/batch/remind \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/submit-design

### 简介
支持方法: POST。

- `POST`: Supports two submit modes: 1) Compatibility single-asset submit (`asset_type` + `file_name`) which creates one `task_assets` record. 2) Batch submit (`assets[]`) which completes multiple upload sessions in a single action and persists canonical `design_assets`/`asset_versions` with SKU scope. Re-entry is allowed from `RejectedByAuditA` and `RejectedByAuditB`. Delivery upload-session completion advances task status to `PendingAuditA` when current status is one of `PendingAssign`, `Assigned`, `InProgress`, `RejectedByAuditA`, or `RejectedByAuditB`, and for multi-SKU batch tasks the gate waits until required SKU-scoped delivery assets are complete. `purchase_task` cannot submit design. This action uses minimum role plus org or handler gating.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/assets

### 简介
支持方法: GET。

- `GET`: Canonical task-scoped design-asset list path. Returns the same resource model as `GET /v1/assets?task_id={id}` and keeps task detail pages on one explicit task context route while `/v1/assets` remains the canonical cross-task resource namespace.

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
curl -X GET https://api.example.com/v1/tasks/<id>/assets \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/assets/timeline

### 简介
支持方法: GET。

- `GET`: Returns the append-only task-asset timeline ordered by `version_no ASC`. This is a compatibility-only standalone refresh view, obsolete for frontend rollout, and not the primary design-asset aggregation surface for new frontend upload integration.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/assets/{asset_id}/versions

### 简介
支持方法: GET。

- `GET`: Compatibility-only alias for `GET /v1/tasks/{id}/asset-center/assets/{asset_id}/versions`. Obsolete for frontend rollout.

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
curl -X GET https://api.example.com/v1/tasks/<id>/assets/<asset_id>/versions \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/assets/{asset_id}/download

### 简介
支持方法: GET。

- `GET`: Compatibility-only alias for `GET /v1/tasks/{id}/asset-center/assets/{asset_id}/download`. Obsolete for frontend rollout.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/assets/{asset_id}/versions/{version_id}/download

### 简介
支持方法: GET。

- `GET`: Compatibility-only alias for `GET /v1/tasks/{id}/asset-center/assets/{asset_id}/versions/{version_id}/download`. Obsolete for frontend rollout.

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
curl -X POST https://api.example.com/v1/tasks/<id>/assets/upload-sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/assets/upload-sessions/{session_id}

### 简介
支持方法: GET。

- `GET`: Compatibility-only alias for `GET /v1/assets/upload-sessions/{session_id}`. Obsolete for frontend rollout.

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
curl -X GET https://api.example.com/v1/tasks/<id>/assets/upload-sessions/<session_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/assets/upload

### 简介
支持方法: POST。

- `POST`: **Deprecated browser contract:** `multipart/form-data` uploads (`file` + `file_role`) are **not supported** on this path and return **410** with `UPLOAD_ENDPOINT_DEPRECATED`. **Supported:** `application/json` body identical to `POST /v1/assets/upload-sessions` (backend-selected upload strategy). **Design drafts (source / delivery / preview):** use the unified asset upload-session contract, upload bytes using the returned remote plan, then call `.../complete`. Preferred entrypoint for new code: `/v1/assets/upload-sessions` instead of this legacy URL.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/warehouse/prepare

### 简介
支持方法: POST。

- `POST`: Evaluates PRD-aligned warehouse readiness. Purchase tasks may enter the warehouse path without design/audit, but they must complete procurement arrival before warehouse handoff; design task types still require final asset and approved audit path. This action now uses minimum role + org scope gating over canonical task ownership.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/assets/mock-upload

### 简介
支持方法: POST。

- `POST`: Creates a `task_assets` record without changing task status. Intended for prototype reference or attachment areas. This route can optionally bind a placeholder `upload_request_id` and emit structured `storage_ref` metadata, but it remains mock or placeholder only and is not a stable real-upload contract.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/close

### 简介
支持方法: POST。

- `POST`: Requires the task to be in explicit `PendingClose` mainline state. This action now uses minimum role + canonical-owner org gating instead of role name alone. `Admin`/`SuperAdmin`/`RoleAdmin`/`HRAdmin` may cross org scope but still cannot bypass the `PendingClose` status gate. `Ops`, `Warehouse`, and scoped management roles must still match the task canonical owner department/team scope. On readiness failure, `error.details` returns: - `task_type` - `workflow` - `closable` - `cannot_close_reasons`

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/audit/claim

### 简介
支持方法: POST。

- `POST`: `auditor_id` is optional and defaults to the current authenticated actor. This action uses minimum role plus org scope gating over canonical task ownership. `Audit_A` may only claim `PendingAuditA`; `Audit_B` may only claim `PendingAuditB`; when a current handler already exists, non-management actors must match that handler instead of taking over globally.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/audit/transfer

### 简介
支持方法: POST。

- `POST`: `from_auditor_id` is optional and defaults to the current authenticated actor. This action uses minimum role plus org or handler gating over canonical task ownership. `Audit_A` can only transfer stage A, `Audit_B` can only transfer stage B, and non-management actors must currently own the handler slot.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/audit/handover

### 简介
支持方法: POST。

- `POST`: `from_auditor_id` is optional and defaults to the current authenticated actor. Handover creation clears the current handler; further audit actions must wait for `takeover`. This action uses minimum role plus org or handler gating over canonical task ownership. `Audit_A` can only hand over stage A, `Audit_B` can only hand over stage B, and non-management actors must currently own the handler slot.

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
curl -X POST https://api.example.com/v1/tasks/<id>/audit/handover \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/audit/handovers

### 简介
支持方法: GET。

- `GET`: Ordered by `created_at DESC`. This endpoint keeps the pre-Step-05 `data`-only list shape.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/outsource

### 简介
支持方法: POST。

- `POST`: Compatibility-only legacy late-branch entry retained for historical tasks. New integrations must create customization-lane tasks with `customization_required=true` instead of using this route.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/outsource-orders

### 简介
支持方法: GET。

- `GET`: Compatibility-only legacy list for historical late-branch outsource records. New integrations should use `/v1/customization-jobs` for the unified customization lane. `vendor` is matched with fuzzy `LIKE` against `vendor_name`.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/warehouse/receipts

### 简介
支持方法: GET。

- `GET`: Returns paginated warehouse receipts. `receiver_id` filters against the current receipt owner/receiver.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/task-board/summary

### 简介
支持方法: GET。

- `GET`: Frontend-ready aggregate entry for role-based workbenches. Returns preset queues with queue identifiers, queue conditions, counts, sample tasks, `normalized_filters`, `/v1/tasks`-ready `query_template` metadata, and lightweight ownership-hint fields built on top of projected `workflow`, task-item `product_selection` summary, and `procurement_summary.coordination_status`. Queue aggregation uses a shared board-level candidate pool and preserves the stable external queue contract. Ownership hints are advisory only and do not introduce enforced queue ownership persistence.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/task-board/queues

### 简介
支持方法: GET。

- `GET`: Frontend-ready aggregate queue endpoint. Returns preset queues with queue conditions, total counts, paginated task lists, `normalized_filters`, `/v1/tasks`-ready `query_template` metadata, and lightweight ownership-hint fields so workbenches can render inbox or task-board columns directly and drill into list view without rebuilding queue logic. Task items in these queues carry the same `product_selection` summary used by `/v1/tasks`, while detail endpoints keep the full provenance object. Queue aggregation uses a shared board-level candidate pool and preserves the stable external queue contract. Ownership hints are advisory only and do not introduce enforced queue ownership persistence.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
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
- `GET` 允许角色: 已登录 / scope-aware。
- `PATCH` 允许角色: 已登录 / scope-aware。
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/export-templates

### 简介
支持方法: GET。

- `GET`: Returns the static export-template catalog for the current export-center skeleton. These templates only describe placeholder export intent over stable read models; they do not imply a real template engine or file-generation pipeline.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/integration/connectors

### 简介
支持方法: GET。

- `GET`: Returns the static connector catalog for the current integration-center boundary. Most connectors remain placeholder-only. `erp_bridge_product_upsert` represents the narrow task business-info filing trace.

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
- `GET` 允许角色: 已登录 / scope-aware。
- `POST` 允许角色: 已登录 / scope-aware。
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/integration/call-logs/{id}

### 简介
支持方法: GET。

- `GET`: Returns one internal integration call log record with request/response payload snapshots, layered lifecycle timestamps, latest execution summary, separate retry/replay admission hints, latest retry/replay action summaries, and shared adapter/handoff summaries. This remains an internal trace surface, not a general integration execution platform.

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
- `GET` 允许角色: 已登录 / scope-aware。
- `POST` 允许角色: 已登录 / scope-aware。
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/integration/call-logs/{id}/retry

### 简介
支持方法: POST。

- `POST`: Internal/admin placeholder retry route. `retry` is allowed only when the latest visible outcome is a retryable failed execution and creates a new execution attempt beneath the same call log. It does not introduce a real retry scheduler, queue, callback, or external executor.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/integration/call-logs/{id}/replay

### 简介
支持方法: POST。

- `POST`: Internal or admin placeholder replay route. `replay` re-drives the existing call-log envelope through a new execution attempt for troubleshooting or controlled redelivery semantics, including previously succeeded or cancelled logs. This is not a real external replay engine.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/integration/call-logs/{id}/executions/{execution_id}/advance

### 简介
支持方法: POST。

- `POST`: Internal/admin placeholder execution-state advancement route. This advances one persisted execution through `prepared|dispatched|received|completed|failed|cancelled` while synchronizing the parent call-log lifecycle summary. It still does not introduce a real external executor, callback processor, or retry scheduler.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/integration/call-logs/{id}/advance

### 简介
支持方法: POST。

- `POST`: Backward-compatible internal or admin call-log lifecycle advancement route. `queued` requeues the parent call log directly, while `sent`, `succeeded`, `failed`, and `cancelled` reuse the explicit execution boundary so call-log lifecycle and execution lifecycle remain layered. This route does not introduce a real integration worker, callback, or retry engine. It is compatibility-only and should not be treated as the preferred execution API.

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
- `GET` 允许角色: 已登录 / scope-aware。
- `POST` 允许角色: 已登录 / scope-aware。
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/export-jobs/{id}

### 简介
支持方法: GET。

- `GET`: Returns one persisted export job skeleton with full placeholder download-handoff metadata plus lightweight lifecycle-audit summaries. Detail payloads also expose `can_start`, `can_attempt`, `can_retry`, `can_dispatch`, `can_redispatch`, admission reason fields (`can_*_reason`, `dispatchability_reason`, `attemptability_reason`, `latest_admission_decision`), `start_mode`, `execution_mode`, `adapter_mode`, `dispatch_mode`, `storage_mode`, `delivery_mode`, `adapter_ref_summary`, `resource_ref_summary`, `handoff_ref_summary`, `execution_boundary`, `storage_boundary`, `delivery_boundary`, `dispatch_count`, `latest_dispatch`, `attempt_count`, `latest_attempt`, `latest_dispatch_event`, `latest_runner_event`, `is_expired`, and `can_refresh` so frontend or internal tools can distinguish export-job lifecycle from dispatch handoff state, placeholder execution-attempt state, placeholder storage representation, and placeholder delivery handoff state. `result_ref` is not a real file location, signed URL, or storage integration.

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
- `GET` 允许角色: 已登录 / scope-aware。
- `POST` 允许角色: 已登录 / scope-aware。
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/export-jobs/{id}/dispatches/{dispatch_id}/advance

### 简介
支持方法: POST。

- `POST`: Internal/admin placeholder dispatch-state advancement route. This endpoint advances one persisted dispatch handoff to `received`, `rejected`, `expired`, or `not_executed` without introducing a real scheduler callback or worker lifecycle. Dispatch state stays separate from both export-job lifecycle and execution-attempt lifecycle.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `POST` 允许角色: 已登录 / scope-aware。
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/export-jobs/{id}/attempts

### 简介
支持方法: GET。

- `GET`: Internal/admin placeholder execution-attempt inspection route for export jobs. This endpoint returns persisted attempt records such as trigger source, execution mode, adapter key, and terminal attempt status, plus additive attempt-level admission hints (`blocks_new_attempt`, `next_attempt_admission_reason`) so current placeholder runner-adapter boundary behavior is visible without pretending a real scheduler, worker lease, or distributed runner platform exists.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/export-jobs/{id}/events

### 简介
支持方法: GET。

- `GET`: Returns the export-job lifecycle audit timeline ordered oldest to newest. Event payload is audit context only and must not be interpreted as a full runner log stream or proof of real file generation/download delivery.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/export-jobs/{id}/claim-download

### 简介
支持方法: POST。

- `POST`: Claims placeholder download handoff for a ready export job. This does not start a real file transfer and does not return file bytes; it records a handoff-claim audit event and returns structured placeholder handoff metadata for frontend consumption. This action is allowed only when the export job is `ready` and the current placeholder handoff is not expired. Expired ready handoff returns a placeholder-expired invalid-state response and requires refresh.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/export-jobs/{id}/download

### 简介
支持方法: GET。

- `GET`: Reads structured placeholder download handoff metadata for a ready export job. This endpoint is the current read boundary only: it does not return real file bytes, signed URLs, NAS paths, or object-storage references. A `download_read` audit event is appended to the existing export-job event chain each time this handoff metadata is read. This action is allowed only when the export job is `ready` and the current placeholder handoff is not expired. Expired ready handoff requires refresh.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/export-jobs/{id}/refresh-download

### 简介
支持方法: POST。

- `POST`: Refreshes expired placeholder download handoff for a ready export job. Refresh rotates the placeholder `result_ref.ref_key`, extends `expires_at`, appends `result_ref_updated` and `download_refreshed` audit events, and returns refreshed handoff metadata. This endpoint is placeholder-only and does not mint signed URLs, return file bytes, re-run export generation, or connect to NAS/object storage.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/export-jobs/{id}/start

### 简介
支持方法: POST。

- `POST`: Internal/admin placeholder runner-initiation boundary for export jobs. This route formalizes the `queued -> running` start contract without introducing a real async runner, scheduler, file generator, NAS integration, or object storage. It is allowed only when the current export job status is `queued`, and a latest `submitted` dispatch blocks start until it is received or otherwise resolved. Admission reasons are exposed through `can_start_reason` and `can_attempt_reason`. Successful start creates or consumes one placeholder dispatch handoff: if latest dispatch is `received`, start consumes it; if no startable dispatch exists, start may auto-create one placeholder submitted and received dispatch when no startable dispatch exists before creating the new attempt. This remains a skeleton only and does not imply a real scheduler or asynchronous dispatch platform.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/export-jobs/{id}/advance

### 简介
支持方法: POST。

- `POST`: Internal or admin skeleton endpoint for manually advancing export-job lifecycle state. This endpoint updates placeholder lifecycle, execution-attempt visibility, and download-handoff metadata while writing audit-trace events. `action=start` remains available for compatibility, but `POST /v1/export-jobs/{id}/start` is the preferred explicit placeholder runner-initiation boundary.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/warehouse/receive

### 简介
支持方法: POST。

- `POST`: Task must be in `PendingWarehouseReceive`. `receiver_id` is optional and defaults to the current authenticated actor. A previously rejected receipt may be received again after re-prepare, and the current handler is set to the receiver. This action uses minimum role plus org or handler gating over canonical task ownership. If a current handler already exists, non-management actors must match that handler instead of taking over the task.

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
curl -X POST https://api.example.com/v1/tasks/<id>/warehouse/receive \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/warehouse/reject

### 简介
支持方法: POST。

- `POST`: Requires `reject_reason` or `remark`. Task must be in `PendingWarehouseReceive`. `receiver_id` is optional and defaults to the current authenticated actor. Purchase tasks return to `PendingAssign`, design or audit tasks return to designer rework, and customization-lane tasks return to `last_customization_operator_id` instead of restarting the whole lane. `reject_category` is the bounded statistics field for warehouse/QC error classification. This action uses minimum role plus org or handler gating over canonical task ownership. Non-management actors must be the current warehouse handler.

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
curl -X POST https://api.example.com/v1/tasks/<id>/warehouse/reject \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/warehouse/complete

### 简介
支持方法: POST。

- `POST`: Requires a prior receive record, and the task must have `sku_code`. This endpoint moves the task into explicit `PendingClose` rather than closing it directly. `receiver_id` is optional and defaults to the current authenticated actor. This action uses minimum role plus org or handler gating over canonical task ownership. Non-management actors must be the current warehouse handler.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/tasks/{id}/customization/review

### 简介
支持方法: POST。

- `POST`: Dedicated customization reviewer entry. The primary `customization_job` is created at task creation. Review writes business-entered review reference data on that record (`customization_level_code`, `customization_level_name`, `review_reference_unit_price`, `review_reference_weight_factor`, `customization_note`), moves approved tasks into customization production, and keeps `return_to_designer` tasks inside the customization review lane instead of sending them through the normal design workbench. Review does not freeze execution settlement pricing.

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
| `reviewer_id` | integer | 否 | - |
| `source_asset_id` | integer | 否 | - |
| `customization_level_code` | string | 否 | Business-entered review reference level code. |
| `customization_level_name` | string | 否 | Business-entered review reference level name. |
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
| 400 | 见 `error.code` | 见 `deny_code` | Invalid decision or missing required review fields |
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/customization-jobs

### 简介
支持方法: GET。

- `GET`: Lists customization work records without ERP order-detail dependency, including tasks that just entered the customization lane at creation time, pricing snapshot fields (`pricing_worker_type`, `unit_price`, `weight_factor`), current effective稿 tracking, and stored `order_no` trace.

### 鉴权与 RBAC
- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。
- `GET` 允许角色: 已登录 / scope-aware。
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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/customization-jobs/{id}

### 简介
支持方法: GET。

- `GET`: Get customization job detail

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/customization-jobs/{id}/effect-preview

### 简介
支持方法: POST。

- `POST`: Customization-operator work entry. The first successful submission freezes pricing snapshot fields by `(employment_type + customization_level_code)` into `pricing_worker_type`, `unit_price`, and `weight_factor`. `decision_type=effect_preview` enters second review, while `decision_type=final` skips effect review and advances directly to production transfer.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/customization-jobs/{id}/effect-review

### 简介
支持方法: POST。

- `POST`: Effect review only accepts jobs in `pending_effect_review`; `return_to_designer` sends workflow back to effect revision, and `reviewer_fixed` may replace the effective working稿 through `current_asset_id` before advancing to production transfer.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/customization-jobs/{id}/production-transfer

### 简介
支持方法: POST。

- `POST`: Production transfer requires `pending_production_transfer` job status, updates task last customization operator snapshot for warehouse reject backflow, and records bounded transfer trace fields for later robot or system integration.

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/{id}/events

### 简介
支持方法: GET。

- `GET`: Ordered by `sequence ASC`. This is a narrow business trace stream and key create, assign, submit-design, audit, procurement, warehouse, and close events may include before/after state plus handler or result context. Customization replacement-related events additionally carry trace fields such as `previous_asset_id`, `current_asset_id`, `replacement_actor_id`, `workflow_lane`, and `source_department`.

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
curl -X GET https://api.example.com/v1/tasks/<id>/events \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/code-rules

### 简介
支持方法: GET。

- `GET`: List code rules

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/code-rules/{id}/preview

### 简介
支持方法: GET。

- `GET`: Preview generated code

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
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/code-rules/generate-sku

### 简介
支持方法: POST。

- `POST`: Generate SKU code

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
| `rule_id` | integer | 是 | - |

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
curl -X POST https://api.example.com/v1/code-rules/generate-sku \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
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
curl -X GET https://api.example.com/v1/sku/list \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
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
成功响应: `201`

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
curl -X POST https://api.example.com/v1/sku \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
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
curl -X GET https://api.example.com/v1/sku/<id> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
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
curl -X GET https://api.example.com/v1/sku/<id>/sync_status \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## POST /v1/audit

### 简介
支持方法: POST。

- `POST`: Legacy audit route retained for compatibility only. Canonical MAIN audit behavior lives under task-centric `/v1/tasks/{id}/audit/*` routes.

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
curl -X POST https://api.example.com/v1/audit \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
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
curl -X POST https://api.example.com/v1/agent/sync \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
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
curl -X POST https://api.example.com/v1/agent/pull_job \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
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
curl -X POST https://api.example.com/v1/agent/heartbeat \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
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
curl -X GET https://api.example.com/v1/incidents \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
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
curl -X GET https://api.example.com/v1/policies \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

## GET /v1/tasks/pool

### 简介
支持方法: GET。

- `GET`: List task pool entries

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
| `data` | array<object> | 否 | - |

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | 见 `error.code` | 见 `deny_code` | Unauthorized |
| 501 | 见 `error.code` | 见 `deny_code` | Reserved for R3 |

### curl 示例
```bash
curl -X GET https://api.example.com/v1/tasks/pool \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
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
| 501 | 见 `error.code` | 见 `deny_code` | Reserved for R3 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/modules/<module_key>/claim \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
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
| 501 | 见 `error.code` | 见 `deny_code` | Reserved for R3 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/modules/<module_key>/actions/<action> \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
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
| 501 | 见 `error.code` | 见 `deny_code` | Reserved for R3 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/modules/<module_key>/reassign \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
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
| 501 | 见 `error.code` | 见 `deny_code` | Reserved for R3 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/modules/<module_key>/pool-reassign \
  -H "Authorization: Bearer $TOKEN"
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
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
| 501 | 见 `error.code` | 见 `deny_code` | Reserved for R3 |

### curl 示例
```bash
curl -X POST https://api.example.com/v1/tasks/<id>/cancel \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"example":"value"}'
```

### 前端最佳实践
- `GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。
- 模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。
- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。
- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。

