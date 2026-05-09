# 任务模块（Task / `src/services/api/tasksApi.ts`）

> 回到索引：[`README.md`](./README.md) · 本模块共 **25 个在用接口**；11 个未使用接口见 [`00-unused-endpoints.md`](./00-unused-endpoints.md)。

## 模块通用说明

- 所有 URL 不带 `/api` 前缀（`http` 模块 `baseURL='/'`），由后端网关 / Nginx 统一路由到 `/v1/...`。
- 所有请求默认附加 `Authorization: Bearer <token>`；401 由 `http.ts` 拦截跳登录；400 由 `mapRawBackendMessageToZh` 转中文。
- 响应体外层统一约定为 `{ data: ... }` 或 `{ data: [...], pagination: {...} }`；前端对 `body.data` / `body.items` / 裸数组三种形态做兼容（见 `stores/tasks.ts:985`）。

---

## 3.A.1 获取任务列表

- **请求路径**：`/v1/tasks`
- **请求方法**：`GET`
- **定义文件**：`src/services/api/tasksApi.ts` (`tasksApi.list`)
- **主要调用位置**：`src/stores/tasks.ts:985` (`loadTaskList`)、`src/views/AssetsIndexView.vue:571`（反查 taskId）

**前端传参 (Request)**：来源 `TaskListParams`（`src/services/apiTypes.ts:131`），均在 Query 中。

| 字段名 | 位置 | 类型推断 | 必填/可选 | 说明/默认值 |
| --- | --- | --- | --- | --- |
| `page` | Query | `number` | 可选 | 默认 1；`loadTasks()` 默认 `page_size=500` |
| `page_size` | Query | `number` | 可选 | 默认 500（管理台），`AssetsIndexView` 用 100 |
| `keyword` | Query | `string` | 可选 | 通用搜索词（任务号 / SKU / 名称） |
| `status` | Query | `string` | 可选 | 主状态过滤，取自 `domain/enums/task-status.ts` |
| `task_type` | Query | `string` | 可选 | `new_product_development` / `original_product_development` 等 |
| `workflow_lane` | Query | `'normal' \| 'customization' \| string` | 可选 | 定制泳道切换 |
| `assignee_id` | Query | `string` | 可选 | 旧字段，兼容 |
| `designer_id` | Query | `string` | 可选 | v0.9：按设计师筛选 |
| `group_id` | Query | `string` | 可选 | 组筛选 |
| `department` | Query | `string` | 可选 | 旧字段 |
| `owner_department` | Query | `string` | 可选 | 规范归属：部门 |
| `owner_org_team` | Query | `string` | 可选 | 规范归属：组织树团队 |

**前端期望返回 (Expected Response)**（反推自 `stores/tasks.ts:986-993` 与 `domain/mappers/*`）：

```json
{
  "data": [
    {
      "id": "string | number",
      "task_no": "string",
      "workflow": {
        "main_status": "string",
        "design_sub_status": "string | null",
        "audit_sub_status": "string | null",
        "warehouse_sub_status": "string | null",
        "purchase_sub_status": "string | null"
      },
      "task_type": "string",
      "workflow_lane": "normal | customization",
      "priority": "low | normal | high | urgent",
      "owner_department": "string | null",
      "owner_org_team": "string | null",
      "owner_team": "string | null",
      "designer_id": "number | null",
      "designer_name": "string | null",
      "assignee_id": "number | null",
      "product_selection": {
        "source_match_type": "string",
        "erp_product": { "product_id": "number", "sku_code": "string", "product_name": "string", "name": "string" }
      },
      "procurement_summary": { "status": "string", "expected_delivery_at": "string | null" },
      "reference_file_refs": [
        { "asset_id": "string", "ref_id": "string", "filename": "string", "mime_type": "string", "file_size": "number", "download_url": "string | null" }
      ],
      "design_assets": "BackendAsset[] 可选",
      "asset_versions": "BackendAssetVersion[] 可选",
      "deadline_at": "string | null",
      "created_at": "string",
      "updated_at": "string"
    }
  ],
  "pagination": { "total": "number", "page": "number", "page_size": "number" }
}
```

兼容回退（前端按此顺序探测）：`body.data` → `body`（裸数组）→ `body.items` → `body.tasks`。

---

## 3.A.2 获取任务详情（主读模型）

- **请求路径**：`/v1/tasks/{id}`
- **请求方法**：`GET`
- **定义文件**：`tasksApi.ts` (`tasksApi.getById`)
- **主要调用位置**：`src/stores/tasks.ts:1075` (`loadTaskById`)、`src/stores/outsource.ts:67`、`src/views/CustomizationJobsView.vue:953`

**前端传参**：`id` 走 Path，无 Query/Body。

| 字段 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| `id` | Path | `string` | 是 | 任务 ID |

**前端期望返回**（反推自 `normalizeBackendTask` 与 `mappers/reference-file-refs.ts`、`mappers/asset-versions-from-api.ts`）：

```json
{
  "data": {
    "id": "string",
    "task_no": "string",
    "workflow": { "main_status": "string", "design_sub_status": "string | null", "audit_sub_status": "string | null", "warehouse_sub_status": "string | null" },
    "task_type": "string",
    "workflow_lane": "normal | customization",
    "product_selection": { "erp_product": { "sku_code": "string", "product_id": "number", "product_name": "string" }, "source_match_type": "string" },
    "reference_file_refs": [ { "asset_id": "string", "filename": "string", "mime_type": "string", "download_url": "string | null" } ],
    "design_assets": [ { "id": "string", "file_role": "string", "versions": [ { "id": "string", "version": "number", "file_name": "string", "download_mode": "direct | proxy | public | private_network", "preview_available": "boolean" } ] } ],
    "asset_versions": "BackendAssetVersion[]",
    "designer_id": "number | null",
    "designer_name": "string | null",
    "filing_status": "string | null",
    "filing_error_message": "string | null",
    "missing_fields": "string[] | null",
    "created_at": "string",
    "updated_at": "string"
  }
}
```

---

## 3.A.3 任务业务事件流

- **请求路径**：`/v1/tasks/{id}/events`
- **请求方法**：`GET`
- **定义**：`tasksApi.listTaskEvents`（对 id `encodeURIComponent`）
- **调用位置**：`src/components/logs/EventLogDrawer.vue:117`、`src/views/CustomizationJobDetailView.vue:347`
- **前端传参**：`id` Path。

**期望返回**（反推自 `domain/mappers/task-events-from-api.ts`）：

```json
{
  "data": [
    {
      "id": "string | number",
      "event_type": "string",
      "actor_id": "number | null",
      "actor_name": "string",
      "summary": "string",
      "payload": "object | null",
      "created_at": "string"
    }
  ]
}
```

---

## 3.A.4 创建任务

- **请求路径**：`/v1/tasks`
- **请求方法**：`POST`
- **定义**：`tasksApi.create`（同一路径也被 `customizationApi.createCustomizationTask` 使用，Body 字段差异见下）
- **调用位置**：`src/stores/tasks.ts:1372`（普通/采购任务）、`src/components/customization/CustomizationCreateDialog.vue:677`（定制任务）

**前端传参 (Body JSON)** — 字段来自 `TaskCreateForm` 与 `customizationApi.buildCustomizationTaskCreateBody`：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `task_type` | `'new_product_development' \| 'original_product_development' \| 'purchase_task' \| ...` | 是 | |
| `requester_id` | `number \| null` | 是 | 由登录用户注入 |
| `requester_name` | `string` | 是 | |
| `designer_id` | `number \| null` | 可选 | 预分配设计师 |
| `owner_department` | `string` | 可选 | |
| `owner_org_team` | `string` | 可选 | |
| `owner_team` | `string` | 是 | `trim()` 后为空则报错 |
| `deadline_at` | `string`(ISO) | 是 | |
| `priority` | `'low' \| 'normal' \| 'high' \| 'urgent'` | 可选 | `medium` 前端统一映射为 `normal` |
| `customization_required` | `boolean` | 可选 | 定制链路 `true` |
| `customization_source_type` | `'existing_product' \| 'new_product'` | 可选（定制必填） | |
| `source_mode` | `'existing_product'` | 仅原有产品开发 | |
| `product_id` | `number \| null` | 可选 | ERP 选品结果 |
| `product_name`, `product_name_snapshot` | `string` | 可选 | |
| `sku_code` | `string \| null` | 可选 | 仅原有产品开发 |
| `change_request` | `string` | 原有产品开发必填 | 设计改动说明 |
| `design_requirement` | `string` | 新品开发必填 | |
| `category_code`, `product_short_name` | `string` | 可选 | 新品开发 |
| `reference_file_refs` | `Array<{ asset_id, ref_id, filename, mime_type, file_size, download_url, source, status }>` | 可选 | 已 sanitize，旧 `url` 字段会被转成 `download_url`（`toRelativeAssetUrl`） |
| `material_mode`, `material_other` | `string` | 可选 | `material === 'OTHER'` 时追加 |
| `product_selection` | `{ selected_product_id: number }` 或 `{ defer_local_product_binding: true, erp_product: {...} }` | 可选 | 仅原有产品开发 |
| `remark` | `string` | 可选 | 默认 `"订单号：xxx"` |

**期望返回**：

```json
{ "data": { "id": "string | number", "task_no": "string", "product_category_code": "string?", "...其余 task 字段": "..." } }
```

前端兼容 `{ id, ... }` 外层裸返回（`pickTaskFromBody`）。

---

## 3.A.5 预展示商品编码

- **请求路径**：`/v1/tasks/prepare-product-codes`
- **请求方法**：`POST`
- **定义**：`tasksApi.prepareProductCodes`
- **调用位置**：`src/stores/tasks.ts:1442`（创建流程中生成 SKU 预览）
- **前端传参 (Body)**：`Record<string, unknown>`，字段来自 `task-create-rules.ts` 中构造的 `preparePayload`，核心字段：`task_type`、`category_code`、`material`、`material_other`、`owner_department`、`reference_product_id` 等。

**期望返回**：

```json
{ "data": { "codes": [ { "sku_code": "string", "sequence_no": "number" } ] } }
```

---

## 3.A.6 指派 / 重新指派设计师

- **请求路径**：`/v1/tasks/{id}/assign`
- **请求方法**：`POST`
- **定义**：`tasksApi.assign`
- **调用位置**：`src/stores/tasks.ts:1519、1544`（首次 / 重新指派共用同一 URL）

**前端传参 (Body)**：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `designer_id` | `number (int64)` | 是 | 必须可 `parseInt`，否则前端抛错 |
| `designer_name` | `string` | 可选 | |

**期望返回**：仅关注 2xx，前端成功后立即 `loadTaskById(id)` 拉最新详情；不依赖响应体字段。

---

## 3.A.7 批量提醒

- **请求路径**：`/v1/tasks/batch/remind`
- **请求方法**：`POST`
- **定义**：`tasksApi.batchRemind`
- **调用位置**：`src/views/TaskListView.vue:725`
- **Body**：`{ task_ids: number[] }`（前端已确保数值数组，对应后端 `[]int64`）

**期望返回**：`{ data: { items: [ { task_id: number, result: 'ok' | 'skipped' | string, reason?: string } ] } }`（只读 `data` 非关键字段，UI 仅展示成功/失败计数）。

---

## 3.A.8 批量指派

- **请求路径**：`/v1/tasks/batch/assign`
- **请求方法**：`POST`
- **定义**：`tasksApi.batchAssign`
- **调用位置**：`src/views/TaskListView.vue:741`
- **Body**：`{ task_ids: number[], designer_id: number, designer_name?: string }`

**期望返回**：同 3.A.7 形态的 batch items；前端成功后整表 `refreshList(true)`。

---

## 3.A.9 设计提交审核

- **请求路径**：`/v1/tasks/{id}/submit-design`
- **请求方法**：`POST`
- **定义**：`tasksApi.submitDesign`
- **调用位置**：`src/stores/tasks.ts:1560`（`useDesignStore` 的上传完成链路 → submit-design）

**Body (`SubmitDesignPayload`, `apiTypes.ts:182`)**：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `assets` | `SubmitDesignAssetItem[]` | 是 | 基于已 complete 的上传 session |
| `assets[].upload_session_id` | `string` | 是 | |
| `assets[].asset_kind` | `'source' \| 'delivery'` | 可选 | |
| `assets[].target_sku_code` | `string` | batch delivery 必填 | 需与 session 创建时一致 |
| `remark` | `string` | 可选 | |

**期望返回**：前端不解析响应，只关心 2xx 推动流转；随后 `loadTaskById`。

---

## 3.A.10 定制审核入口

- **请求路径**：`/v1/tasks/{id}/customization/review`
- **请求方法**：`POST`
- **定义**：`tasksApi.submitCustomizationReview`
- **调用位置**：`src/stores/tasks.ts:1660`

**Body**：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `reviewer_id` | `number \| string` | 可选 | |
| `source_asset_id` | `number \| string \| null` | 可选 | |
| `customization_level_code` | `string` | 可选 | |
| `customization_level_name` | `string` | 可选 | |
| `customization_price` | `number \| null` | 可选 | |
| `customization_weight_factor` | `number \| null` | 可选 | |
| `customization_note` | `string` | 可选 | |

**期望返回**：前端只读状态码。

---

## 3.A.11 审核领取

- **请求路径**：`/v1/tasks/{id}/audit/claim`
- **请求方法**：`POST`
- **Body**：`{ stage: 'A' | 'B' | 'outsource_review' }` — `stage` 必填，与 OpenAPI 一致（`tasksApi.ts:150`）。
- **调用**：`stores/tasks.ts:1567`（`claimAudit`）。

---

## 3.A.12 审核通过

- **请求路径**：`/v1/tasks/{id}/audit/approve`
- **请求方法**：`POST`
- **Body**：`{ stage: string, next_status: string, comment?: string }` — 后端要求 `stage` 与 `next_status` 均必填。
- **调用**：`stores/tasks.ts:1580` (`passAudit`)、`stores/tasks.ts:1637`（转复审：`stage='A', next_status='PendingAuditB'`）。

---

## 3.A.13 审核驳回

- **请求路径**：`/v1/tasks/{id}/audit/reject`
- **请求方法**：`POST`
- **Body**：`{ stage: string, comment: string }` — `stage`（问题分类）、`comment`（审核说明）均必填。
- **调用**：`stores/tasks.ts:1594`。

---

## 3.A.14 审核交班

- **请求路径**：`/v1/tasks/{id}/audit/handover`
- **请求方法**：`POST`
- **Body**：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `to_auditor_id` | `number` | 是 | |
| `reason` | `string` | 是 | |
| `current_judgement` | `string` | 可选 | |
| `risk_remark` | `string` | 可选 | |

- **调用**：`src/views/AuditWorkbenchView.vue:1018`。

---

## 3.A.15 查询交班记录

- **请求路径**：`/v1/tasks/{id}/audit/handovers`
- **请求方法**：`GET`
- **调用**：`src/views/AuditWorkbenchView.vue:951`

**期望返回**（反推自 `AuditWorkbenchView` 数据绑定）：

```json
{
  "data": [
    { "id": "number", "to_auditor_id": "number", "to_auditor_name": "string",
      "from_auditor_id": "number", "from_auditor_name": "string",
      "reason": "string", "current_judgement": "string | null",
      "risk_remark": "string | null", "status": "pending | taken_over | cancelled",
      "created_at": "string" }
  ]
}
```

---

## 3.A.16 接手交班

- **请求路径**：`/v1/tasks/{id}/audit/takeover`
- **请求方法**：`POST`
- **Body**：`{ handover_id: number }`
- **调用**：`AuditWorkbenchView.vue:963`。

---

## 3.A.17 审核转交

- **请求路径**：`/v1/tasks/{id}/audit/transfer`
- **请求方法**：`POST`
- **Body**：`{ to_auditor_id: number, reason?: string }`
- **调用**：`stores/tasks.ts:1605` (`transferAudit`)。

---

## 3.A.18 仓库接收

- **请求路径**：`/v1/tasks/{id}/warehouse/receive`
- **请求方法**：`POST`
- **Body**：`Record<string, unknown>`（当前默认空对象 `{}`；未来可扩展 `warehouse_location_code` 等）
- **调用**：`stores/tasks.ts:1696`。

---

## 3.A.19 仓库驳回

- **请求路径**：`/v1/tasks/{id}/warehouse/reject`
- **请求方法**：`POST`
- **Body**：`{ reason: string, category?: string, action_id?: string }`（`action_id` 为幂等 UUID，见 `.cursorrules` 幂等红线）
- **调用**：`stores/tasks.ts:1707`。

---

## 3.A.20 仓库完成

- **请求路径**：`/v1/tasks/{id}/warehouse/complete`
- **请求方法**：`POST`
- **Body**：`{}`（占位，当前无字段）
- **调用**：`stores/tasks.ts:1734`。

---

## 3.A.21 正式关单

- **请求路径**：`/v1/tasks/{id}/close`
- **请求方法**：`POST`
- **Body**：`{}`（占位）。**异常契约**：后端 `409` 时返回 `cannot_close_reasons: string[]`，前端按此弹对比框（见 `.cursorrules` CAS 处理，实际读取在 `closeTask` 错误分支）。
- **调用**：`stores/tasks.ts:1732`。

---

## 3.A.22 更新业务信息

- **请求路径**：`/v1/tasks/{id}/business-info`
- **请求方法**：`PATCH`
- **Body**：`BusinessInfoPatch`（`apiTypes.ts:168`，`Record<string, unknown>`）—— 支持局部字段；真实字段集合为 `TaskCloseMasterDataSupplement.vue` / `PurchaseBusinessInfoSupplement.vue` / `TaskCreateModal.vue` 里拼出的 `bizPatch`，含：`product_name`、`product_short_name`、`category_code`、`sku_code`、`owner_team`、`owner_department`、`priority`、`deadline_at`、`remark` 等。
- **调用**：
  - `components/task-detail/TaskCloseMasterDataSupplement.vue:176`
  - `components/task/TaskCreateModal.vue:1081`
  - `components/purchase/PurchaseBusinessInfoSupplement.vue:164`

---

## 3.A.23 创建 / 更新采购 procurement 草稿

- **请求路径**：`/v1/tasks/{id}/procurement`
- **请求方法**：`PATCH`
- **Body** 字段（由 `PurchaseBusinessInfoSupplement.vue` 与 `PurchaseWarehousePanel.vue` 提供）：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `expected_delivery_at` | `string` | 可选 |
| `purchase_channel` | `string` | 可选 |
| `purchase_contact` | `string` | 可选 |
| `cost_total`, `cost_note` | `number/string` | 可选 |
| `action_id` | `string`(UUID v4) | 幂等，强约束（见 `.cursorrules`） |

- **调用**：`PurchaseBusinessInfoSupplement.vue:168`、`PurchaseWarehousePanel.vue:152`。

---

## 3.A.24 采购生命周期推进

- **请求路径**：`/v1/tasks/{id}/procurement/advance`
- **请求方法**：`POST`
- **Body**：`{ action: 'prepare' | 'start' | 'complete' | 'reopen', remark?: string, action_id?: string }`
- **调用**：`components/purchase/PurchaseWarehousePanel.vue:159、164、169、173`。

---

## 3.A.25 重试建档同步

- **请求路径**：`/v1/tasks/{id}/filing/retry`
- **请求方法**：`POST`
- **Body**：`{}`（无字段）
- **调用**：
  - `components/task-detail/TaskCloseMasterDataSupplement.vue:190`
  - `components/task-detail/FilingStatusCard.vue:86`

**期望返回**：后端可在响应 `data` 返回最新 `filing_status / filing_error_message / missing_fields[] / erp_sync_version`（与 `FilingStatusResponse` 对齐，见 `apiTypes.ts:118`）。
