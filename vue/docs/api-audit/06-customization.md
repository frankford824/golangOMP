# 定制模块（Customization / `src/services/api/customizationApi.ts`）

> 回到索引：[`README.md`](./README.md) · 本模块共 **6 个在用接口**，无未使用接口。
>
> 其中 3.F.1 复用 `POST /v1/tasks`（见 [`01-task.md`](./01-task.md) §3.A.4），Body 字段差异汇总在下方。

---

## 3.F.1 创建定制任务（复用 `POST /v1/tasks`）

- **请求路径**：`/v1/tasks`
- **请求方法**：`POST`
- **定义**：`createCustomizationTask`（复用 Task 模块 3.A.4 的同一 URL，Body 结构差异见 `buildCustomizationTaskCreateBody`）
- **调用**：`src/components/customization/CustomizationCreateDialog.vue:677`。

定制链路必传字段：`customization_required=true`、`customization_source_type`、`source_mode`、`change_request`、`product_selection`、`material_mode`、`material_other`、`remark`。

详见 [`01-task.md`](./01-task.md) §3.A.4 Body 字段定义及 [`99-integration-notes.md`](./99-integration-notes.md) §4.6 中关于 `POST /v1/tasks` 路径复用的讨论。

---

## 3.F.2 定制单列表

- **请求路径**：`/v1/customization-jobs`
- **请求方法**：`GET`
- **定义**：`listCustomizationJobs`
- **调用**：`src/views/CustomizationJobsView.vue:923`、`src/components/task-detail/AuditOutsourceBlock.vue:660`

**前端传参 (Query, `CustomizationJobQuery`)**：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `task_id` | `number \| string` | |
| `status` | `string` | |
| `operator_id` | `number \| string` | |
| `page` / `page_size` | `number` | CustomizationJobsView 默认 20；AuditOutsourceBlock 默认 20 |

**期望返回**：

```json
{
  "data": {
    "items": ["CustomizationJobRaw"],
    "pagination": { "page": "number", "page_size": "number", "total": "number" }
  }
}
```

前端兼容五种形态：顶层数组、`data` 数组、`data.items` 数组、`data.data` 数组、`items` 数组（见 `CustomizationJobsView.vue:927-935`）。

`CustomizationJobRaw` 主要字段（`apiTypes.ts:349`）：`id, task_id, source_asset_id, previous_asset_id, current_asset_id, customization_level_code/name, review_reference_unit_price, review_reference_weight_factor, unit_price, weight_factor, note, customization_review_decision, decision_type, assigned_operator_id, employment_type, workflow_lane, source_department, status, warehouse_reject_reason, warehouse_reject_category, created_at, updated_at`。

---

## 3.F.3 定制单详情

- **请求路径**：`/v1/customization-jobs/{id}`
- **请求方法**：`GET`
- **定义**：`getCustomizationJobDetail`
- **调用**：`CustomizationJobsView.vue:852`、`src/views/CustomizationJobDetailView.vue:341`
- **期望返回**：`{ data: CustomizationJobRaw }` 或裸 `CustomizationJobRaw`；前端 `extractCustomizationJob` 兼容解包。

---

## 3.F.4 效果预览提交

- **请求路径**：`/v1/customization-jobs/{id}/effect-preview`
- **请求方法**：`POST`
- **定义**：`submitCustomizationEffectPreview`
- **调用**：`CustomizationJobsView.vue:874`

**Body**：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `operator_id` | `number \| string` | 是 | 前端强校验非空 |
| `current_asset_id` | `number \| string \| null` | 可选 | |
| `note` | `string` | 可选 | |

---

## 3.F.5 效果审核提交

- **请求路径**：`/v1/customization-jobs/{id}/effect-review`
- **请求方法**：`POST`
- **定义**：`submitCustomizationEffectReview`
- **调用**：`CustomizationJobsView.vue:883`

**Body**：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `reviewer_id` | `number \| string` | 是 | |
| `customization_review_decision` | `'approved' \| 'return_to_designer' \| 'reviewer_fixed'` | 是 | |
| `customization_level_code` | `string` | 可选 | |
| `customization_level_name` | `string` | 可选 | |
| `customization_price` | `number \| null` | 可选 | |
| `customization_weight_factor` | `number \| null` | 可选 | |
| `customization_note` | `string` | 可选 | |

---

## 3.F.6 生产流转

- **请求路径**：`/v1/customization-jobs/{id}/production-transfer`
- **请求方法**：`POST`
- **定义**：`submitCustomizationProductionTransfer`
- **调用**：`CustomizationJobsView.vue:896`
- **Body**：同 3.F.4（`operator_id, current_asset_id, note`）。
