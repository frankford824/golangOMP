# 联调注意事项 / 异常点全量汇总

> 回到索引：[`README.md`](./README.md) · 本文件记录跨模块的共性约束和前端目前已做的兼容兜底，用于在真实联调前拉齐认知，避免返工。

## 4.1 命名一致性

1. **`auditLogApi.list` 的 Query 命名用了 camelCase**（`taskNo / pageSize / start / end`），与其他接口的 snake_case 风格不一致，建议后端提供**两种兼容**或前端改写为 `task_no / page_size / start_at / end_at`。参考 `src/services/api/auditLogApi.ts:9-17`。
2. **`authApi.register` 的 `department` 传的是部门名称而非 id**：因为 `/v1/auth/register-options` 只暴露 name（兼容裸字符串数组），建议后端保留 name→id 的映射能力，或在 register-options 中同时返回 `id/name` 供前端升级。
3. **`createOrgTeam` 的 `department_id` 同时支持 `number/string`**：前端在能转成纯数字时传 `number`，否则传原字符串。后端应统一接受两种，或前端固定成 `number`。

## 4.2 响应形态兼容

前端对几乎所有列表接口做了"顶层数组 / `data` 数组 / `data.items` / `items` / `data.data`"**五种**形态的兜底读取。建议后端统一为：

```json
{ "data": [ ... ], "pagination": { "total": number, "page": number, "page_size": number } }
```

可降低前端解析歧义。当前已出现至少以下 5 处兜底代码：

- `stores/tasks.ts:985`
- `AssetsIndexView.vue:686-728`
- `CustomizationJobsView.vue:927-935`
- `LogsManagementView.vue:575-592`
- `rulesApi.loadRules`

此外：

- 批量接口（`batch/remind` / `batch/assign`）响应契约被前端忽略（仅 `refreshList`），若后端希望做"部分成功+部分失败"的 UI，需前端补逐条展示。
- `audit-logs` 接口失败时前端目前静默回落到 mock（见 [`10-audit-log.md`](./10-audit-log.md)），生产环境建议改成错误可见。

## 4.3 幂等 & CAS（按 `.cursorrules` 的强制要求）

1. **所有写入型动作须携带 `action_id (UUID v4)`**。当前前端仍漏传 `action_id` 的接口（需补齐）：
   - 任务：`assign`、`submit-design`、`audit/claim`、`audit/approve`、`audit/reject`、`audit/handover`、`audit/takeover`、`audit/transfer`、`warehouse/receive`、`warehouse/complete`、`batch/remind`、`batch/assign`
   - 资产：`/v1/assets/upload-sessions`（会话本身视为幂等；但若重试 session 创建应由前端复用同一 action_id）
   - 定制：`effect-preview`、`effect-review`、`production-transfer`
   - 组织：`createOrgDepartment`、`createOrgTeam`
   - 用户：`usersApi.create`、`patch`、`resetPassword`、`replaceRoles`
2. `POST /v1/tasks/{id}/close` 在 **409 `VERSION_CONFLICT` / `CANNOT_CLOSE`** 时，响应体须带 `details.cannot_close_reasons: string[]`，前端按此弹"原因"对比框：

   ```json
   { "error": { "code": "CANNOT_CLOSE", "details": { "cannot_close_reasons": ["string", "..."] } } }
   ```

3. `audit/approve`、`audit/reject`、`audit/claim` 的 `stage` 字段前端可能传入 `'A' | 'B' | 'outsource_review'` 三种值；后端缺失时应返回 `400` 并附带 `error.details.field='stage'`，由 `mapRawBackendMessageToZh` 转中文提示。

## 4.4 上传链路（资产中心）

1. **Content-Type 强一致**：create-session 的 `mime_type` 必须与 `oss_direct.required_upload_content_type` 一致，前端 `runOssDirectUploadPlan` 会预校验；两者不一致时 OSS 会直接 `SignatureDoesNotMatch`。
2. **Multipart finalize 必填字段**：complete 阶段 `oss_upload_id / oss_object_key / oss_parts[] / upload_content_type` 任何一个缺失，前端立即阻断并自动 `cancel`。
3. **任务状态切换期间的 complete**：后端返回 `PERMISSION_DENIED + deny_code=task_status_not_actionable + action=asset_upload_session_complete` 时，前端**不再调 cancel**，保留服务端现场，避免数据丢失。
4. **reference fallback**：`POST /v1/tasks/reference-upload` 当前只接受 `file` 一个 multipart 字段，建议后端保持该契约。
5. **`reference_file_refs` 字段的两种来源**：
   - `POST /v1/assets/upload-sessions` 成功后 complete 响应沉淀到任务详情；
   - 创建前无 `task_id` 走 `POST /v1/tasks/reference-upload` fallback（唯一 `multipart/form-data`），返回同构 ref。
   - 两种来源都需保证 `download_url` 为**规范业务入口**（相对路径，经 `toRelativeAssetUrl` 规范化）；已废弃的 `url` 字段请后端勿再返回。

## 4.5 接口弃用与清理

1. **已标记 deprecated 且业务链已迁移**：
   - `POST /v1/tasks/{id}/outsource`
   - `GET /v1/outsource-orders`
   
   建议后端在 OpenAPI 中标注 `deprecated: true` 并给出迁移路径 `/v1/customization-jobs*`。
2. [`00-unused-endpoints.md`](./00-unused-endpoints.md) 列出的 **21 个真实未使用接口**建议后端逐项确认：
   - 如保留：请保持契约稳定并纳入自动化回归。
   - 如不保留：请同步更新 `docs/openapi.yaml` 并移除实现，前端同步清理 `src/services/api/*`。
3. 非 HTTP 的 19 个占位 stub 建议前端本轮直接删除，避免在 IDE 自动补全中误导开发者。

## 4.6 特殊异构

1. **`GET /v1/warehouse/receipts` 不含 `pending_receive` 状态**，导致前端必须额外 `tasksStore.forceRefreshList()` 从任务读模型合成"待接收"行。建议后端在回执读模型侧新增 `pending_receive` 或提供独立读模型。详见 [`08-warehouse-receipt.md`](./08-warehouse-receipt.md)。
2. **`POST /v1/tasks` 路径被两种任务类型复用**（普通任务 / 定制任务），字段结构差异显著。建议后端在 OpenAPI 里显式 `oneOf`，或拆分路径 `/v1/tasks/customization` + `/v1/tasks` 以便前后端独立演进。参考 [`01-task.md`](./01-task.md) §3.A.4 + [`06-customization.md`](./06-customization.md) §3.F.1。
3. **`GET /v1/tasks/{id}` 返回字段非常宽**（workflow / product_selection / reference_file_refs / design_assets / asset_versions / procurement_summary / filing_* / designer_* 等），前端通过 `normalizeBackendTask` + 多个 mapper 规范化。建议后端在 OpenAPI 固化字段契约，避免字段 rename 导致前端链路受损。

## 4.7 错误转义

- 前端错误文案统一经 `src/utils/api-message-zh.ts` 的 `mapRawBackendMessageToZh` 转中文。后端如新增错误码/消息请先同步此映射，避免直显英文 stack。
- `HttpAppError.responseData` 会把后端 400 原始 body 透出给调用方，`audit/reject` 等交互页可直接读取 `error.details` 做字段级定位。
