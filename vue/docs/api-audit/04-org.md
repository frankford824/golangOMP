# 组织模块（Org / `src/services/api/orgApi.ts`）

> 回到索引：[`README.md`](./README.md) · 本模块共 **3 个在用接口**；2 个未使用接口（`updateOrgDepartment`、`updateOrgTeam`）见 [`00-unused-endpoints.md`](./00-unused-endpoints.md)。

---

## 3.D.1 查询组织选项

- **请求路径**：`/v1/org/options`
- **请求方法**：`GET`
- **定义**：`fetchOrgOwnershipOptions`
- **调用位置**：
  - `src/composables/useOrgOwnershipFilterOptions.ts:30`
  - `src/composables/useOrgPermissionData.ts:84`
  - `src/composables/useTeamOptions.ts:20`
  - `src/views/org-permission/UserManagementView.vue:309`
- **前端传参**：无。

**期望返回**（前端兼容 A/B/C 三种形态，见 `orgApi.ts:146-225`，**推荐后端按 B 输出**）：

```json
{
  "data": {
    "departments": [
      {
        "id": "string | number",
        "name": "string",
        "enabled": "boolean",
        "teams": [
          { "id": "string | number", "name": "string", "enabled": "boolean" }
        ]
      }
    ]
  }
}
```

兼容形态：A 顶层 `{ teams: [...] }`、C `{ teams_by_department: { 运营部: ["一组"] } }`。异常时前端静默返回空列表。

---

## 3.D.2 创建部门

- **请求路径**：`/v1/org/departments`
- **请求方法**：`POST`
- **定义**：`createOrgDepartment`
- **调用**：`src/views/org-permission/OrgPermissionView.vue:442`
- **Body**：`{ name: string, enabled: boolean }`（`enabled` 默认 `true`，`name` 已 `trim()`）

**期望返回**：

```json
{ "data": { "id": "string | number", "name": "string" } }
```

兼容裸 `{ id, name }`；前端若缺 `id` 会抛"创建部门成功但未返回 id"。

---

## 3.D.3 创建小组

- **请求路径**：`/v1/org/teams`
- **请求方法**：`POST`
- **定义**：`createOrgTeam`
- **调用**：`OrgPermissionView.vue:474`

**Body**：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | `string` | 是 | |
| `department_id` | `number \| string` | 是 | 纯数字会被转 `Number` 传；否则传字符串 |
| `enabled` | `boolean` | 可选 | 默认 `true` |

**期望返回**：

```json
{ "data": { "id": "string | number", "name": "string", "department_id": "string | number" } }
```

前端将 `id`、`name`、`department_id`（兼容 `departmentId`）映射回 UI；无 id 抛错。
