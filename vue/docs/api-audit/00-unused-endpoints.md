# 未使用接口清单（建议前端清理 / 后端下线）

> 回到索引：[`README.md`](./README.md)

均已经过全局 `Grep` 双重确认，仅在定义文件中出现一次，无任何组件 / 视图 / Store 实际调用。

## 1. 真实 HTTP 接口（21 个）

| # | 请求方法 | URL | 定义位置（函数名） | 备注 |
| --- | --- | --- | --- | --- |
| 1 | `PUT` | `/v1/auth/password` | `authApi.changePassword` (`src/services/api/authApi.ts:56`) | 前端尚未实现"修改密码"页 |
| 2 | `GET` | `/v1/access-rules` | `authApi.getAccessRules` (`authApi.ts:63`) | 权限目录由 `frontend_access` 已覆盖 |
| 3 | `GET` | `/v1/task-board/summary` | `tasksApi.boardSummary` (`tasksApi.ts:34`) | 看板摘要改走本地 `tasksStore.list` 聚合 |
| 4 | `GET` | `/v1/task-board/queues` | `tasksApi.boardQueues` (`tasksApi.ts:42`) | 同上 |
| 5 | `GET` | `/v1/tasks/{id}/detail` | `tasksApi.getDetail` (`tasksApi.ts:67`) | 全部改用主读模型 `GET /v1/tasks/{id}` |
| 6 | `POST` | `/v1/tasks/{id}/outsource` | `tasksApi.createTaskOutsource` (`tasksApi.ts:218`) | 已 `@deprecated`，新链路走 `/v1/customization-jobs` |
| 7 | `GET` | `/v1/tasks/{id}/business-info` | `tasksApi.getBusinessInfo` (`tasksApi.ts:273`) | 只用 `PATCH`，读模型并入主详情 |
| 8 | `GET` | `/v1/tasks/{id}/product-info` | `tasksApi.getProductInfo` (`tasksApi.ts:290`) | 未落地到页面 |
| 9 | `PATCH` | `/v1/tasks/{id}/product-info` | `tasksApi.patchProductInfo` (`tasksApi.ts:298`) | 未落地到页面 |
| 10 | `GET` | `/v1/tasks/{id}/cost-info` | `tasksApi.getCostInfo` (`tasksApi.ts:305`) | 未落地到页面 |
| 11 | `PATCH` | `/v1/tasks/{id}/cost-info` | `tasksApi.patchCostInfo` (`tasksApi.ts:313`) | 未落地到页面 |
| 12 | `POST` | `/v1/tasks/{id}/cost-quote/preview` | `tasksApi.costQuotePreview` (`tasksApi.ts:320`) | 未落地到页面 |
| 13 | `GET` | `/v1/tasks/{id}/filing-status` | `tasksApi.getFilingStatus` (`tasksApi.ts:345`) | `FilingStatusCard` 只调 `retryFiling`，读模型来自主详情 |
| 14 | `POST` | `/v1/users/{id}/roles` | `usersApi.assignRoles` (`usersApi.ts:57`) | 统一使用 `PUT /v1/users/{id}/roles`(replaceRoles) |
| 15 | `DELETE` | `/v1/users/{id}/roles/{role}` | `usersApi.removeRole` (`usersApi.ts:72`) | 同上 |
| 16 | `PUT` | `/v1/org/departments/{id}` | `orgApi.updateOrgDepartment` (`orgApi.ts:122`) | 启用/停用未在 UI 暴露 |
| 17 | `PUT` | `/v1/org/teams/{id}` | `orgApi.updateOrgTeam` (`orgApi.ts:127`) | 同上 |
| 18 | `GET` | `/v1/assets/upload-sessions/{id}` | `assetsApi.getUploadSessionStatus` (`assetsApi.ts:229`) | 未轮询 session 状态，流程直接走 complete |
| 19 | `GET` | `/v1/rule-templates/{type}` | `rulesApi.getByType` (`rulesApi.ts:36`) | `RuleConfigView` 仅用列表接口 |
| 20 | `GET` | `/v1/erp/products/{id}` | `erpApi.getProduct` (`erpApi.ts:23`) | 选品只用列表搜索 |
| 21 | `GET` | `/v1/erp/categories` | `erpApi.getCategories` (`erpApi.ts:31`) | 分类来自本地 `/v1/categories` |

**后端行动建议**：逐项确认是否保留。若保留请同步 `docs/openapi.yaml` 并纳入自动化回归；若下线请在 OpenAPI 中显式 `deprecated: true` 并注明迁移路径，前端同步移除函数。

## 2. 非 HTTP 占位 stub（19 个，仅返回 `Promise.resolve(null/[])`）

无任何联调契约，建议前端本轮直接删除，避免 IDE 自动补全误导开发者。

- `src/services/api/adminApi.ts`：`fetchUsers`、`createUser`、`fetchAuditLog`
- `src/services/api/designApi.ts`：`assignDesigner`、`submitDesignVersion`、`submitToAudit`
- `src/services/api/financeApi.ts`：`fetchCostSummary`、`exportFinanceData`
- `src/services/api/kpiApi.ts`：`fetchDesignerKpi`
- `src/services/api/productsApi.ts`：`fetchProductList`、`fetchProductById`（返回 mock）
- `src/services/api/purchaseApi.ts`：`updatePurchaseInfo`、`markPurchased`、`fetchPurchaseTasks`
- `src/services/api/warehouseApi.ts`：`receiveTask`、`returnTask`、`batchReceiveTasks`、`fetchWarehousePendingTasks`
- `src/services/api/design.ts`：`uploadDeliveryFileViaAssetSession`（已被 `design.store` 直接改用 `prepareTaskAssetUploadSession / completePreparedTaskAssetUploadSession`）

## 3. 已 `@deprecated` 的兼容出口（4 个）

位于 `src/services/api/tasksApi.ts` 底部，无任何调用方：

- `fetchTaskList`
- `fetchTaskById`
- `createTask`
- `updateTask`

建议随 Store 重构一并删除。

## 清理动作建议

1. 从 `src/services/api/*.ts` 中删除上述 21 + 19 + 4 = **44 个函数**。
2. 同步更新 `src/services/api/index.ts` 的 re-export 列表，避免外部误引。
3. 若对应 TS 类型（payload / response）不再被任何在用函数引用，一并从 `src/services/apiTypes.ts` 移除。
4. 提交一次专门的 `chore(api): prune unused endpoints` commit，便于后端 review 清单。
