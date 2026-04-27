> V1 Source of Truth: `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`

# V1 前端联调接口文档索引

> Revision: V1.1-A2 contract drift purge (2026-04-27)
> Source: docs/api/openapi.yaml (post V1.1-A2)
> 与 v1.21 生产实际响应对齐


> Release: v1.21 · Backend: V1.0 + V1.1-A1 · Production detail P99 warm 32.933ms / cold 32.995ms。

## 修订历史

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.21 release | 2026-04-25 | 首版 16 份 family 联调文档 |
| V1.1-A2 contract drift purge | 2026-04-27 | 全量对齐 v1.21 实际响应 schema · detail 接口由 30+ 富字段更正为 5 段精简 · 共 6 处 P0/P1 schema 修订 |

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
- `reports_super_admin_only`
- `asset_version_race_retry`
- `audit_log_access_denied`
- `workflow_lane_unsupported`
- `old_password_mismatch`
- `password_confirmation_required`
- `password_confirmation_mismatch`

## §3 RBAC 角色矩阵

| 角色 | 主要权限点 |
|---|---|
| `SuperAdmin` | 全局管理、报表、危险操作、用户管理。 |
| `HRAdmin` | 组织与用户管理范围内操作。 |
| `DepartmentAdmin` | 本部门用户与任务管理。 |
| `TeamLead` | 本组任务管理与人员协作。 |
| `Ops` | 运营/客服任务创建、分派与跟进。 |
| `Designer` | 设计模块领取、提交与资产处理。 |
| `CustomizationOperator` | 定制模块处理。 |
| `Audit_A / Audit_B / CustomizationReviewer` | 审核相关模块动作。 |
| `Warehouse / Member` | 仓库或普通成员范围内可见任务与操作。 |

## §4 路由分类

- Canonical: `/v1/auth/*`, `/v1/me*`, `/v1/users*`, `/v1/erp/products*`, `/v1/tasks*`, `/v1/tasks/{id}/asset-center/*`, `/v1/task-drafts*`, `/v1/me/notifications*`, `/v1/reports/l1/*`, `/ws/v1`。
- Compatibility: `/v1/products*`, `/v1/task-create/asset-center/*`, 以及 transport 中 `withCompatibilityRoute` 标记的旧入口。
- Deprecated: transport 中 `withDeprecatedRoute` 标记的旧入口；新前端不要接。

## §5 Family 索引

| Family | 文档 | path 数 |
|---|---|---|
| 认证与登录 | [V1_API_AUTH.md](V1_API_AUTH.md) | 5 |
| 当前用户 | [V1_API_ME.md](V1_API_ME.md) | 4 |
| 用户与管理审计 | [V1_API_USERS.md](V1_API_USERS.md) | 18 |
| 组织架构 | [V1_API_ORG.md](V1_API_ORG.md) | 9 |
| 任务主流程 | [V1_API_TASKS.md](V1_API_TASKS.md) | 95 |
| 任务资产中心 | [V1_API_TASK_ASSETS.md](V1_API_TASK_ASSETS.md) | 16 |
| 资产资源库 | [V1_API_ASSETS.md](V1_API_ASSETS.md) | 15 |
| 任务草稿 | [V1_API_DRAFTS.md](V1_API_DRAFTS.md) | 2 |
| 通知 | [V1_API_NOTIFICATIONS.md](V1_API_NOTIFICATIONS.md) | 4 |
| Excel 批量创建 | [V1_API_BATCH.md](V1_API_BATCH.md) | 2 |
| ERP 与业务字典 | [V1_API_ERP.md](V1_API_ERP.md) | 27 |
| 搜索 | [V1_API_SEARCH.md](V1_API_SEARCH.md) | 3 |
| L1 报表 | [V1_API_REPORTS.md](V1_API_REPORTS.md) | 3 |
| WebSocket | [V1_API_WS.md](V1_API_WS.md) | 0 个 `/v1` path + `/ws/v1` |
| 全量速查 | [V1_API_CHEATSHEET.md](V1_API_CHEATSHEET.md) | 203 |

## §6 联调硬门

- 所有请求必须走 Bearer token，公开登录/注册除外。
- 首屏详情优先使用 `GET /v1/tasks/{id}/detail`，不要并发拼旧 detail 子接口。
- 前端必须展示后端 `error.code` 或 `deny_code`。
- 新页面只接 canonical 路径。
- WebSocket 只做实时提示，最终一致状态回读 HTTP。
- Excel 批量创建以 parse preview 的 `violations` 为准，不在前端复制完整业务校验。

## §7 Deprecated / Compatibility 清单

- `/v1/task-create/asset-center/*`: 创建前资产上传兼容入口。
- `/v1/products*`: 老本地缓存商品入口，新联调用 `/v1/erp/products*`。
- `/v1/tasks/{id}/audit_a_claim`、`/v1/tasks/{id}/audit_b_claim`: 老审核领取别名。
- 所有 `withCompatibilityRoute` / `withDeprecatedRoute` 标记路径不得作为新前端主入口。

