# 外协订单（Outsource / `src/services/api/outsourceApi.ts`）

> 回到索引：[`README.md`](./README.md) · 本模块共 **1 个在用接口（⚠️ `@deprecated`）**。

---

## 3.G.1 ⚠️ 外协订单列表（deprecated，仅兼容）

- **请求路径**：`/v1/outsource-orders`
- **请求方法**：`GET`
- **定义**：`listOutsourceOrders`（`@deprecated`，业务上应改走 [`06-customization.md`](./06-customization.md) §3.F.2 `/v1/customization-jobs`）
- **调用**：`src/stores/outsource.ts:48`

**前端传参 (Query, `OutsourceOrderListQuery`)**：

```ts
{
  task_id?: number,
  status?: string,
  vendor?: string,
  page?: number,
  page_size?: number
}
```

**期望返回**：`{ data: { items: BackendOutsourceOrderRaw[], pagination: { total, page, page_size } } }` 或兼容形态。

`BackendOutsourceOrderRaw` 字段（见 `apiTypes.ts:322`）：`id, outsource_no, task_id, vendor_name, outsource_type, delivery_requirement, settlement_note, status, returned_at, created_at, updated_at, task_no, sku, product_name`。

**前端联动**：还会为每个 `task_id` 并发调用 Task 模块 3.A.2 `GET /v1/tasks/{id}` 补 `taskNo / SKU / productName`（当列表项自带这些字段时跳过）。

**联调建议**：

- 后端在 OpenAPI 中显式标注 `deprecated: true`，并在响应头 `Deprecation / Sunset` 中指引迁移到 `/v1/customization-jobs*`。
- 前端本轮可以将 `src/stores/outsource.ts` 全量替换为 `listCustomizationJobs`，消除该接口对任务详情读模型的 N+1 依赖。
