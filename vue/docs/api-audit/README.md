# 前端接口现状与联调需求文档

> 由前端工作区（`d:\vue`）全量静态扫描而来，口径：`src/services/api/*`、`src/services/api/design.ts`、`src/services/upload/assetUploadFlow.ts`（复用 `assetsApi.*`）。`src/services/upload/ossDirectUpload.ts` 直接向 OSS 签名 URL 发 `PUT`，属于第三方直传，不计入联调契约。

## 阅读顺序建议

1. 先读本页 **全局统计概览** 了解盘面。
2. 再读 [`00-unused-endpoints.md`](./00-unused-endpoints.md) 快速完成「应清理 / 应下线」的决策。
3. 逐模块阅读 `01-*` ~ `13-*`（每个模块是一份独立可发给后端的联调契约）。
4. 联调前必读 [`99-integration-notes.md`](./99-integration-notes.md)（幂等、命名、响应形态、上传链路等跨模块约束）。

## 1. 全局统计概览

- 定义总数（真实 HTTP 接口函数，不含占位 stub）：**89**
- 🟢 前端正在使用的接口数：**68**
- 🔴 定义但未使用（建议前端清理 / 后端下线）：**21**
- 额外占位 stub（函数体仅 `Promise.resolve(null/[])`，并未发起任何 HTTP 请求）：**19**
  - 分布：`adminApi.ts`(3)、`designApi.ts`(3)、`financeApi.ts`(2)、`kpiApi.ts`(1)、`productsApi.ts`(2)、`purchaseApi.ts`(3)、`warehouseApi.ts` 占位函数(4)、`services/api/design.ts::uploadDeliveryFileViaAssetSession`(1)。
- 另有 **4 个 `@deprecated` 兼容出口**（`fetchTaskList` / `fetchTaskById` / `createTask` / `updateTask`，位于 `src/services/api/tasksApi.ts` 底部），无任何调用方，建议随 Store 重构一并删除。

### 按模块分布

| 模块 | 定义文件 | 定义 | 使用 | 未使用 | 详情文件 |
| --- | --- | ---: | ---: | ---: | --- |
| 任务 Task | `tasksApi.ts` | 36 | 25 | 11 | [`01-task.md`](./01-task.md) |
| 认证 Auth | `authApi.ts` | 6 | 4 | 2 | [`02-auth.md`](./02-auth.md) |
| 用户 User | `usersApi.ts` | 10 | 8 | 2 | [`03-user.md`](./03-user.md) |
| 组织 Org | `orgApi.ts` | 5 | 3 | 2 | [`04-org.md`](./04-org.md) |
| 资产 Asset | `assetsApi.ts` | 10 | 9 | 1 | [`05-asset.md`](./05-asset.md) |
| 定制 Customization | `customizationApi.ts` | 6 | 6 | 0 | [`06-customization.md`](./06-customization.md) |
| 外协 Outsource | `outsourceApi.ts` | 1 | 1（⚠️deprecated） | 0 | [`07-outsource.md`](./07-outsource.md) |
| 仓库回执 Warehouse | `warehouseApi.ts` | 1 | 1 | 0 | [`08-warehouse-receipt.md`](./08-warehouse-receipt.md) |
| 日志 Logs | `logsApi.ts` | 4 | 4 | 0 | [`09-logs.md`](./09-logs.md) |
| 审计日志 Audit-log | `auditLogApi.ts` | 1 | 1 | 0 | [`10-audit-log.md`](./10-audit-log.md) |
| 分类 Category | `categoriesApi.ts` | 3 | 3 | 0 | [`11-category.md`](./11-category.md) |
| 规则模板 Rule | `rulesApi.ts` | 3 | 2 | 1 | [`12-rule.md`](./12-rule.md) |
| ERP | `erpApi.ts` | 3 | 1 | 2 | [`13-erp.md`](./13-erp.md) |

## 2. 通用技术约定

所有业务接口（除 OSS 直传 / reference fallback 的 multipart 外）满足以下通用约定：

- **HTTP 封装**：`src/services/http.ts`，`axios.create({ baseURL: '/', timeout: 30s, headers: { 'Content-Type': 'application/json' } })`。
- **鉴权**：请求拦截器自动注入 `Authorization: Bearer <token>`；token 存储在 `localStorage`。
- **401**：响应拦截器捕获后清 token、跳登录页。
- **400**：经 `mapRawBackendMessageToZh`（`src/utils/api-message-zh.ts`）把后端 `error.code / error.message` 翻译成中文。
- **错误对象**：统一封成 `HttpAppError`，调用方可读 `error.responseData` 拿到原始 body（如 `audit/reject` 的字段级 `details`）。
- **响应体外层**：约定 `{ data: ... }`（列表多附 `{ data: [...], pagination: { total, page, page_size } }`）。前端对 `body.data` / `body.items` / `body.data.items` / `body.data.data` / 裸数组均做兼容兜底，建议后端统一为 `{ data, pagination }`（详见 [`99-integration-notes.md`](./99-integration-notes.md) §4.2）。
- **幂等**：按 `.cursorrules`，所有写入型动作应带 `action_id (UUID v4)`。当前前端漏传清单见 [`99-integration-notes.md`](./99-integration-notes.md) §4.3。

## 3. 文件清单

| 文件 | 内容 |
| --- | --- |
| [`README.md`](./README.md) | 全局统计、通用约定、索引（本页） |
| [`00-unused-endpoints.md`](./00-unused-endpoints.md) | 21 个未使用接口 + 19 个占位 stub + 4 个 deprecated 兼容函数的完整清理清单 |
| [`01-task.md`](./01-task.md) | 任务模块 25 个在用接口详情 |
| [`02-auth.md`](./02-auth.md) | 认证 4 个在用接口详情 |
| [`03-user.md`](./03-user.md) | 用户 8 个在用接口详情 |
| [`04-org.md`](./04-org.md) | 组织 3 个在用接口详情 |
| [`05-asset.md`](./05-asset.md) | 资产 9 个在用接口详情（含 OSS 直传契约） |
| [`06-customization.md`](./06-customization.md) | 定制单 6 个在用接口详情 |
| [`07-outsource.md`](./07-outsource.md) | 外协订单 1 个 deprecated 接口 |
| [`08-warehouse-receipt.md`](./08-warehouse-receipt.md) | 仓库回执 1 个在用接口（含已知契约缺口） |
| [`09-logs.md`](./09-logs.md) | 日志 4 个在用接口 |
| [`10-audit-log.md`](./10-audit-log.md) | 审计日志 1 个在用接口 |
| [`11-category.md`](./11-category.md) | 分类 3 个在用接口 |
| [`12-rule.md`](./12-rule.md) | 规则模板 2 个在用接口 |
| [`13-erp.md`](./13-erp.md) | ERP 1 个在用接口 |
| [`99-integration-notes.md`](./99-integration-notes.md) | 联调注意事项 / 异常点全量汇总（跨模块） |
