# V1 前端联调接口文档索引

> Revision: V8 current contract (2026-07-20)
> Source: docs/api/openapi.yaml

当前真相入口: [V1_BACKEND_SOURCE_OF_TRUTH.md](../V1_BACKEND_SOURCE_OF_TRUTH.md)

> Contract: V8 shared-backend，覆盖 main-ops 与 asset-workbench 当前公开接口。

## §0 Base URL 与鉴权

- 生产: `https://<prod-host>` 或联调反代地址。
- 本地/隧道: `http://127.0.0.1:18080`。
- 鉴权: `Authorization: Bearer <token>`。
- 成功响应常见包装: `{"data": ...}`；以各接口 OpenAPI response schema 为准。

## §1 联调起步 6 步

1. `POST /v1/auth/login` 获取 token。
2. `GET /v1/me` 校验当前用户。
3. `GET /v1/tasks` 拉任务列表。
4. `GET /v1/tasks/{id}/detail` 拉首屏聚合详情。
5. 使用 `/v1/tasks/{id}/asset-center/*` 联调任务资产。
6. 使用 `/v1/tasks/batch-create/template.xlsx` 与 `/parse-excel` 联调 Excel 批量预览。

## §2 错误码总表

| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | UNAUTHENTICATED | - | 未登录、token 缺失或 token 过期。 |
| 403 | PERMISSION_DENIED | 见接口返回 | 角色、组织范围、字段级授权或流程状态不允许。 |
| 404 | NOT_FOUND | - | 资源不存在或当前用户不可见。 |
| 409 | CONFLICT | 见接口返回 | 状态竞态、重复操作或版本冲突。 |
| 422 | VALIDATION_ERROR | - | 请求参数或业务字段校验失败。 |
| 500 | INTERNAL | - | 后端内部错误；联调时带 trace/log 找后端排查。 |

常见 deny_code:

- `task_create_field_denied_by_scope`
- `task_out_of_scope`
- `task_out_of_stage_scope`
- `task_not_assigned_to_actor`
- `task_status_not_actionable`
- `task_not_reassignable`
- `module_action_role_denied`
- `department_scope_only`
- `team_scope_only`
- `org_admin_scope_only`
- `user_update_field_denied_by_scope`
- `role_assignment_denied_by_scope`
- `management_access_required`
- `asset_version_race_retry`
- `workflow_lane_unsupported`
- `old_password_mismatch`
- `password_confirmation_required`
- `password_confirmation_mismatch`

## §3 显式权限模型

- 业务授权只认 `auth_*` 角色、能力与稳定组织 ID 范围。
- `Member` 是基础身份；`SuperAdmin` 是受保护角色。
- 前端菜单和动作以后端有效能力及 `allowed_actions` 为准，不按旧角色名、部门名或状态自行推断。
- 具体路由能力要求以各接口 OpenAPI 扩展字段和运行时 middleware 为准。

## §4 路由分类

- 当前 `/v1` 与 `/ws/v1` 路径以本次从 OpenAPI 生成的 family 索引为准。
- 任务主流程只包含创建、设计、统一审核与结单；已退役流程不属于当前合同。

## §5 Family 索引

| Family | 文档 | path 数 |
|---|---|---|
| 认证与登录 | [V1_API_AUTH.md](V1_API_AUTH.md) | 5 |
| 当前用户 | [V1_API_ME.md](V1_API_ME.md) | 6 |
| 用户与管理审计 | [V1_API_USERS.md](V1_API_USERS.md) | 6 |
| 组织架构 | [V1_API_ORG.md](V1_API_ORG.md) | 7 |
| 任务主流程 | [V1_API_TASKS.md](V1_API_TASKS.md) | 177 |
| 任务资产中心 | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) | 1 |
| 资产资源库 | [V1_API_ASSETS.md](V1_API_ASSETS.md) | 16 |
| 任务草稿 | [V1_API_DRAFTS.md](V1_API_DRAFTS.md) | 2 |
| 通知 | [V1_API_NOTIFICATIONS.md](V1_API_NOTIFICATIONS.md) | 9 |
| Excel 批量创建 | [V1_API_BATCH.md](V1_API_BATCH.md) | 2 |
| ERP 与业务字典 | [V1_API_ERP.md](V1_API_ERP.md) | 23 |
| 搜索 | [V1_API_SEARCH.md](V1_API_SEARCH.md) | 2 |
| WebSocket | [V1_API_WS.md](V1_API_WS.md) | 0 个 `/v1` path + `/ws/v1` |
| 全量速查 | [V1_API_CHEATSHEET.md](V1_API_CHEATSHEET.md) | 256 |

## §6 联调硬门

- 所有请求必须走 Bearer token，公开登录/注册和公开随机资源除外。
- 首屏详情优先使用 `GET /v1/tasks/{id}/detail`，不要并发拼旧 detail 子接口。
- 前端必须展示后端 `error.code` 或 `deny_code`。
- 新页面只接 canonical 路径。
- WebSocket 只做实时提示，最终一致状态回读 HTTP。
- Excel 批量创建以 parse preview 的 `violations` 为准，不在前端复制完整业务校验。

## §7 已退役业务边界

- 不建设任何未启用的占位接口；当前文档仅描述已挂载且可使用的合同。
- 历史证据只保留在明确的 archive/迁移边界，不得作为新前端或后续模型的实现依据。

