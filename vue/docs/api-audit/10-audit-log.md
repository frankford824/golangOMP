# 审计日志（AuditLog / `src/services/api/auditLogApi.ts`）

> 回到索引：[`README.md`](./README.md) · 本模块共 **1 个在用接口**，无未使用接口。

---

## 3.J.1 审计记录列表

- **请求路径**：`/v1/audit-logs`
- **请求方法**：`GET`
- **定义**：`auditLogApi.list`
- **调用**：`src/stores/audits.ts:33` (`loadAuditLogs`)

**Query (`AuditLogListParams`)**：`{ taskNo?, auditor?, action?, start?, end?, page?, pageSize? }`

> ⚠️ **字段名使用 camelCase**，与项目其他接口的 snake_case 风格不一致（见 [`99-integration-notes.md`](./99-integration-notes.md) §4.1）。联调时请与后端对齐命名，建议改为 `task_no / start_at / end_at / page_size`。

**期望返回**（反推自 `mapAuditRecord`）：

```json
{
  "data": [
    {
      "id": "string | number",
      "task_id": "string | number",
      "auditor_id": "string | number",
      "auditor_name": "string",
      "action": "pass | reject | return | takeover | ...",
      "comment": "string | null",
      "problem_category": "string | null",
      "affect_launch": "boolean | null",
      "need_outsource": "boolean | null",
      "created_at": "string"
    }
  ]
}
```

兼容 `{ items: [...] }` 与裸数组。

**⚠️ 行为提醒**：接口失败时前端当前**静默回落到 mock**（需联调确认：生产环境应改为抛错可见，避免"审计全是假数据"却无任何提示）。
