# 认证模块（Auth / `src/services/api/authApi.ts`）

> 回到索引：[`README.md`](./README.md) · 本模块共 **4 个在用接口**；2 个未使用接口见 [`00-unused-endpoints.md`](./00-unused-endpoints.md)。

---

## 3.B.1 获取注册下拉选项

- **请求路径**：`/v1/auth/register-options`
- **请求方法**：`GET`
- **定义**：`authApi.registerOptions`
- **主要调用位置**：`src/views/AuthView.vue:267`、`src/composables/useTeamOptions.ts:35`
- **前端传参**：无（仅可选 `AbortSignal`）。

**前端期望返回**（反推自 `AuthView.vue:268-275` 的多形态兼容解包）：

```json
{
  "data": {
    "departments": [
      {
        "name": "string",
        "teams": [ "string | { name: string }" ]
      }
    ]
  }
}
```

前端兼容：顶层数组 `[{ data: { departments } }]`、裸对象 `{ departments }`、裸字符串数组 `["部门A","部门B"]`；每个部门可返回字符串或 `{ name, teams }`。

---

## 3.B.2 注册

- **请求路径**：`/v1/auth/register`
- **请求方法**：`POST`
- **定义**：`authApi.register`
- **调用位置**：`src/views/AuthView.vue:365`

**前端传参 (Body, `RegisterPayload`)**：

| 字段 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| `username` | Body | `string` | 是 | `trim()` 后非空 |
| `password` | Body | `string` | 是 | 前端强制 ≥8 字符 |
| `display_name` | Body | `string` | 是 | |
| `department` | Body | `string` | 是 | 来自 register-options 名称（注意：不是 id） |
| `team` | Body | `string` | 是 | 未选时兜底为 `department` |
| `mobile` | Body | `string` | 是 | |
| `email` | Body | `string` | 可选 | 为空串时不发送 |
| `admin_key` | Body | `string` | 可选 | 开通部门管理员时使用 |

**期望返回**：`BackendUser` 结构即可（注册成功后前端立即 `loginWithCredentials` 重走登录流程，不直接消费 body）。

---

## 3.B.3 登录

- **请求路径**：`/v1/auth/login`
- **请求方法**：`POST`
- **定义**：`authApi.login`
- **调用位置**：`src/stores/permissions.ts:211` (`loginWithCredentials`)
- **前端传参 (Body)**：`{ username: string, password: string }`

**期望返回** (`LoginResponse`，兼容 `{ data: LoginResponse }` 与顶层裸对象)：

```json
{
  "data": {
    "user": {
      "id": "string | number",
      "username": "string",
      "display_name": "string",
      "department": "string",
      "team": "string",
      "roles": ["string"],
      "frontend_access": {
        "menus": ["string"],
        "pages": ["string"],
        "actions": ["string"],
        "roles": ["string"],
        "is_super_admin": "boolean",
        "is_department_admin": "boolean",
        "is_group_leader": "boolean",
        "view_all": "boolean",
        "managed_departments": ["string"],
        "managed_teams": ["string"]
      }
    },
    "session": { "token": "string" },
    "token": "string"
  }
}
```

前端 token 取值顺序：`data.session.token` → `data.token`（任一非空即可，见 `permissions.ts:221`）。

---

## 3.B.4 当前登录用户

- **请求路径**：`/v1/auth/me`
- **请求方法**：`GET`
- **定义**：`authApi.me`
- **调用位置**：`src/stores/permissions.ts:239` (`restoreSession`) — 路由守卫初始化时调用
- **前端传参**：无。

**期望返回**：与登录响应的 `user` 部分等价，但支持以下 canonical 字段之外的兼容读取：`user.id | user_id`、`user.display_name | displayName | username`。`frontend_access` 字段结构同上。401 时后端应返回标准 401，由 `http.ts` 拦截器自动跳转登录页。
