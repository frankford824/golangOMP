# 仓库回执（Warehouse Receipts / `src/services/api/warehouseApi.ts`）

> 回到索引：[`README.md`](./README.md) · 本模块共 **1 个在用接口**，另有 4 个非 HTTP 占位 stub 见 [`00-unused-endpoints.md`](./00-unused-endpoints.md)。

---

## 3.H.1 仓库回执列表

- **请求路径**：`/v1/warehouse/receipts`
- **请求方法**：`GET`
- **定义**：`listWarehouseReceipts`
- **调用**：`src/views/WarehouseView.vue:159`

**前端传参 (Query, `WarehouseReceiptListParams`)**：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `page` | `number` | 默认 1 |
| `page_size` | `number` | 默认 500（仓库中心一次全量） |
| `workflow_lane` | `'normal' \| 'customization'` | 空值或非枚举不下发（前端已过滤） |
| `task_id` | `number \| string` | |
| `status` | `string` | `received / rejected / completed / packing / pending_receive / ...` |
| `receiver_id` | `number \| string` | |

**期望返回**（反推自 `warehouseReceiptRawToReceiptRow`，兼容 `data` 字段或裸数组）：

```json
{
  "data": [
    {
      "id": "string",
      "task_id": "string | number",
      "task_no": "string",
      "sku": "string",
      "product_name": "string",
      "workflow_lane": "normal | customization",
      "source_department": "string | null",
      "status": "received | rejected | completed | packing | pending_receive",
      "receipt_status": "string (兼容别名)",
      "business_type": "string (e.g. PURCHASE_TASK)",
      "task_type": "string (备选)",
      "is_batch_task": "boolean",
      "batch_item_count": "number",
      "need_outsource": "boolean",
      "design_requirement": "string | null",
      "audit_completed_at": "string | null",
      "updated_at": "string | null",
      "filing_status": "string | null",
      "missing_fields_summary_cn": "string | null",
      "filing_error_message": "string | null",
      "task": { "任意嵌套 task 字段": "string" },
      "task_summary": { "任意嵌套 task 摘要字段": "string" }
    }
  ]
}
```

## ⚠️ 已知契约缺口

当前 `WarehouseReceipt.status` 的枚举仅 `received | rejected | completed`，**缺失 `pending_receive`**。因此前端 `WarehouseView` 必须额外调用 `tasksStore.forceRefreshList()`，从任务读模型合成"待接收"行。

**联调建议**：

1. 后端在回执读模型侧新增 `pending_receive` 状态（或提供独立读模型如 `/v1/warehouse/pending-receipts`），让前端只调一次该接口即可渲染完整仓库中心列表。
2. 在契约对齐前，保留现状但请确保 `tasksStore.forceRefreshList()` 所依赖的主任务列表接口（Task §3.A.1）性能足够承载仓库人员的大批量刷新。
