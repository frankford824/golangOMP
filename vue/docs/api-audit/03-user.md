# 用户模块（User / `src/services/api/usersApi.ts`）

> 回到索引：[`README.md`](./README.md) · 本模块共 **8 个在用接口**；2 个未使用接口（`assignRoles`、`removeRole`）见 [`00-unused-endpoints.md`](./00-unused-endpoints.md)。

---

## 3.C.1 获取设计师列表

- **请求路径**：`/v1/users/designers`
- **请求方法**：`GET`
- **定义**：`usersApi.getDesigners`
- **调用位置**：`src/views/TaskListView.vue:702`、`src/views/TaskDetailView.vue:482`、`src/composables/useDesignerOptions.ts:25`
- **前端传参**：无。

**期望返回**（反推自 `TaskListView.vue:706-710`）：

```json
{
  "data": [
    { "id": "string | number", "username": "string", "display_name": "string" }
  ]
}
```

前端仅消费 `id`、`display_name`（兜底 `username`）。

---

## 3.C.2 用户列表（分页 / 筛选）

- **请求路径**：`/v1/users`
- **请求方法**：`GET`
- **定义**：`usersApi.list`
- **调用位置**：`src/views/org-permission/UserManagementView.vue:346`、`src/composables/useOrgPermissionData.ts:83`、`src/components/audit/AuditHandoverDialog.vue:65`

**前端传参 (Query)**：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `page` | `number` | 是 | |
| `page_size` | `number` | 是 | UserManagement 用 `pageSize.value`（默认 20）；`useOrgPermissionData`/`AuditHandoverDialog` 用 500 |
| `keyword` | `string` | 可选 | `trim()` 为空不发送 |
| `status` | `'active' \| 'disabled'` | 可选 | |
| `role` | `string` | 可选 | 角色 code |
| `department` | `string` | 可选 | 部门管理员数据范围兜底注入 |
| `team` | `string` | 可选 | |

**期望返回**：

```json
{
  "data": [
    {
      "id": "string | number",
      "username": "string",
      "display_name": "string",
      "department": "string",
      "team": "string | null",
      "group": "string | null",
      "roles": ["string"],
      "status": "active | disabled",
      "frontend_access": "FrontendAccess"
    }
  ],
  "pagination": { "total": "number", "page": "number", "page_size": "number" }
}
```

前端做 `team = team ?? group` 的兜底读取（见 `mapRawUser`）。

---

## 3.C.3 获取用户详情

- **请求路径**：`/v1/users/{id}`
- **请求方法**：`GET`
- **定义**：`usersApi.getById`
- **调用位置**：`UserManagementView.vue:380`、`src/composables/useOrgPermissionData.ts:51`
- **前端传参**：`id` Path。
- **期望返回**：`{ data: <同 3.C.2 单条> }`，兼容裸 `{ ...user }`。

---

## 3.C.4 角色目录

- **请求路径**：`/v1/roles`
- **请求方法**：`GET`
- **定义**：`usersApi.listRoles`
- **调用位置**：`UserManagementView.vue:319`
- **期望返回**：前端兼容三种形态，任一即可：`["admin","designer"]`、`[{ code, role, name }]`、`{ data: [...] }`。

---

## 3.C.5 覆盖用户角色

- **请求路径**：`/v1/users/{id}/roles`
- **请求方法**：`PUT`
- **定义**：`usersApi.replaceRoles`
- **调用**：`UserManagementView.vue:407`
- **Body**：`{ roles: string[] }`（前端用 role code）
- **期望返回**：`{ data: <user> }` 或裸 user；前端成功后以返回的 `roles` 覆盖 UI 状态。

---

## 3.C.6 更新用户

- **请求路径**：`/v1/users/{id}`
- **请求方法**：`PATCH`
- **定义**：`usersApi.patch`
- **调用**：
  - `UserManagementView.vue:429` — `{ status: 'active' | 'disabled' }`
  - `useOrgPermissionData.ts:69` — `{ department?: string, team?: string, display_name?: string, ... }`（局部字段）
- **Body**：`Record<string, unknown>`（仅变更字段）
- **期望返回**：`{ data: <user> }`，前端不强依赖响应字段（成功后重新拉列表）。

---

## 3.C.7 创建用户

- **请求路径**：`/v1/users`
- **请求方法**：`POST`
- **定义**：`usersApi.create`
- **调用**：`UserManagementView.vue:479`

**Body**：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `username` | `string` | 是 | |
| `display_name` | `string` | 是 | |
| `department` | `string` | 是 | |
| `team` | `string` | 是 | |
| `mobile` | `string` | 是 | |
| `email` | `string` | 可选 | 空串不发送 |
| `password` | `string` | 是 | 默认 `Init1234` |
| `roles` | `string[]` | 可选 | |
| `status` | `'active' \| 'disabled'` | 可选 | |

**期望返回**：2xx 即可，前端紧接 `loadUsers` 刷列表。

---

## 3.C.8 重置用户密码

- **请求路径**：`/v1/users/{id}/password`
- **请求方法**：`PUT`
- **定义**：`usersApi.resetPassword`
- **调用**：`UserManagementView.vue:448`
- **Body**：`{ password: string }`（前端已 `trim()`）
- **期望返回**：2xx。
