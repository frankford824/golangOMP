# 日志模块（Logs / `src/services/api/logsApi.ts`）

> 回到索引：[`README.md`](./README.md) · 本模块共 **4 个在用接口**，无未使用接口。

---

## 3.I.1 权限变更日志

- **请求路径**：`/v1/permission-logs`
- **请求方法**：`GET`
- **定义**：`logsApi.permissionLogs`
- **调用**：`src/views/logs/LogsManagementView.vue:560`

**Query**：`{ page: number, page_size: number, user_id?: string }`

**期望返回**：`{ data: PermissionLog[], pagination: { total: number } }`。

`PermissionLog` 字段（见 `apiTypes.ts:231`）：

```ts
{
  id: string | number,
  actor_id: string | number,
  actor_username: string,
  actor_source: string,
  action_type: string,
  target_user_id: string | number,
  target_username: string,
  method: string,
  route_path: string,
  granted: boolean,
  reason: string,
  created_at: string
}
```

---

## 3.I.2 操作日志（聚合）

- **请求路径**：`/v1/operation-logs`
- **请求方法**：`GET`
- **定义**：`logsApi.operationLogs`
- **调用**：`LogsManagementView.vue:514、530`（支持"超页自动回退"：先按当前页请求，若超过 `maxPage` 再请求 `maxPage`）

**Query**：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `source` | `'task_event' \| 'export_event' \| 'integration_call'` | 可选 |
| `event_type` | `string` | 可选 |
| `page` / `page_size` | `number` | |

**期望返回**：`{ data: OperationLogEntry[], pagination: { total: unknown } }`。

`OperationLogEntry` 字段：`source, log_id, reference_type, reference_id, event_type, summary, actor_id, actor_type, status, payload, created_at`。前端允许 `pagination.total` 非 number 的兜底计算。

---

## 3.I.3 服务器日志

- **请求路径**：`/v1/server-logs`
- **请求方法**：`GET`
- **定义**：`logsApi.serverLogs`
- **调用**：`LogsManagementView.vue:588`

**Query**：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `level` | `'info' \| 'warn' \| 'error'` | 可选 |
| `keyword` | `string` | `trim()` 为空不发送 |
| `since` / `until` | `string (ISO)` | 前端 `new Date(v).toISOString()` |
| `page` / `page_size` | `number` | |

**期望返回**：`ServerLog[]` 或 `{ data: ServerLog[], pagination: { total } }`，前端兼容裸数组。

`ServerLog = { id, level, msg, details, created_at }`，`details` 可对象或可 JSON 解析字符串。

---

## 3.I.4 清理服务器日志

- **请求路径**：`/v1/server-logs/clean`
- **请求方法**：`POST`
- **定义**：`logsApi.serverLogsClean`
- **调用**：`LogsManagementView.vue:609`
- **Body**：`{ reason: string, older_than_hours?: number }`（`reason` 必填，`older_than_hours` 默认 24）
- **期望返回**：2xx；成功后前端立刻 `loadServerLogs()` 刷新。
