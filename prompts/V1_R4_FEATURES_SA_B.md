# V1 · R4-SA-B · 组织菜单 + 用户管理 + 跨部门调配 + 个人中心

> 版本:**v1**(2026-04-17 · R1.7-B OpenAPI SA-B 补丁落地后起草)
> 状态:**待 Codex 执行**(R4 顺序 4 轮第 2 轮)
> 依赖:R1(OpenAPI 冻结)+ R2 v3(`org_move_requests` 表已生产落地)+ R3 v1 + R3.5 验证 + **R1.7-B SA-B OpenAPI 补丁**(2026-04-17 签字)+ **R4-SA-A v2.1 签字**
> 执行环境:**本地代码 + `jst_ecs` 上的 `jst_erp_r3_test` 测试库**(R3.5 / SA-A 已搭好,直接复用);生产 `jst_erp` 零写入
> 禁止前置:R1.7-B + R4-SA-A 任一未签字时不得启动

---

## 0. 本轮目标(一句话)

> **把 OpenAPI 冻结的 14 条 R4-SA-B 路径从 `501` / 旧路径 / 部分实现 补齐到 `V1_INFORMATION_ARCHITECTURE.md` v1.1 §5 §7.2 合同**,落地组织菜单下用户管理的三级授权矩阵(§5.4)+ 个人中心 4 端点 + 跨部门调配 4 端点;**不改 OpenAPI**、**不动 R2 DDL**、**不碰资产/任务/通知/搜索**。

---

## 1. 必读输入

1. `docs/V1_INFORMATION_ARCHITECTURE.md` **v1.1** · **本轮唯一权威**
   - §5 用户管理(**本轮主轴**)
     - §5.1 三级授权模型(SuperAdmin / HRAdmin / DeptAdmin / TeamLead / Member)
     - §5.2 跨部门移动工作流(DeptAdmin 发起 → SuperAdmin 确认)
     - §5.3 API 表(12 条路由的角色矩阵)
     - §5.4 字段级授权矩阵 PATCH /v1/users/{id}(**逐字段逐角色精确口径**)
     - §5.5 不走审批(SuperAdmin / HRAdmin / DeptAdmin 在自己范围直接生效)
   - §6 组织架构调整(SA-B 不落 §6 的部门/组增删改 —— 那些路径不在 owner-round=R4-SA-B 清单里,已有旧路由承载;本轮**不动现有 `/v1/org/departments` / `/v1/org/teams` 路由**)
   - §7.2 个人中心子页清单(`/v1/me` / `PATCH /v1/me` / `/v1/me/change-password` / `/v1/me/org`)
2. `docs/V1_MODULE_ARCHITECTURE.md` v1.2
   - §6.2 deny_code 枚举(SA-B **不得新增**;`403` 全部用 `module_action_role_denied` 或自取字符串 code,新枚举项是下一轮决策)
3. `docs/iterations/V1_R1_7_B_OPENAPI_SA_B_PATCH.md` **v1.0**(本轮核心 · 9 条实装口径已明文化)
   - §5 给 Codex 的 9 条实装口径(`avatar/team_codes/primary_team_code` no-op · org-move-request 201 · approve 写审计 · §5.4 role 字段级授权 · TeamLead 仅本组 activate/deactivate · DeptAdmin list 强制过滤 source_department)
4. `docs/iterations/V1_R2_REPORT.md` v1 + `docs/iterations/V1_R3_REPORT.md` v1 + `docs/iterations/V1_R3_5_INTEGRATION_VERIFICATION.md` v1 + `docs/iterations/V1_R4_SA_A_REPORT.md` v1.1
   - 真生产基线 · R3.5 测试库 · 本轮 integration 复用 `jst_erp_r3_test`
5. `docs/api/openapi.yaml` · 仅阅读以下 14 条路径的 schema(**禁止修改**):
   - `GET /v1/users` · `POST /v1/users` · `PATCH /v1/users/{id}` · `DELETE /v1/users/{id}`
   - `POST /v1/users/{id}/activate` · `POST /v1/users/{id}/deactivate`
   - `GET /v1/me` · `PATCH /v1/me` · `POST /v1/me/change-password` · `GET /v1/me/org`
   - `POST /v1/departments/{id}/org-move-requests` · `GET /v1/org-move-requests`
   - `POST /v1/org-move-requests/{id}/approve` · `POST /v1/org-move-requests/{id}/reject`
6. 现有代码(**必读 · 本轮是扩展而非重建**):
   - `domain/auth_identity.go`(`User` / `UserSession` / `Role` / `FrontendAccessView` / 所有 `PermissionAction*` 常量)
   - `domain/frontend_access.go`(`BuildFrontendAccess` / `ScopesFlex`)
   - `service/identity_service.go`(**重点**:`ChangePassword` / `UpdateUser` / `authorizeUserUpdate` 已有 v0.9 实现,SA-B 在其上扩 §5.4 字段级授权矩阵 · 不另建 `service/me` 包)
   - `transport/handler/user_admin.go`(**重点**:`ListUsers` / `CreateUser` / `GetUser` / `PatchUser` / `ResetPassword` / `AddRoles` / `SetRoles` / `RemoveRole` 已有,SA-B 在其上补 activate / deactivate / delete)
   - `transport/handler/auth.go`(现有 `Me` / `ChangePassword` 挂在 `/v1/auth/me` / `/v1/auth/password`;SA-B 加挂到 `/v1/me` / `/v1/me/change-password`,**复用现有 service 方法**)
   - `service/permission/authorize_module_action.go`(R3 落地,**本轮不用**,但不得修改)
   - `testsupport/r35`(R3.5 测试 DSN 守卫,本轮直接用)
   - R2 migration 065:`db/migrations/065_v1_0_org_move_requests.sql`(SA-B 唯一会写入的新表)

禁止引用:R1.5 之前的文档;`docs/archive/*`;任何 R4-SA-A / SA-C / SA-D owner 的代码目录(`service/asset_center` / `service/asset_lifecycle` / `service/notification` / `service/search` / `service/report_l1` 等)

---

## 2. 真生产基线(从 R2/R3.5/SA-A 报告固化)

Codex 实现读路径时必须认到这些事实:

| 事实 | 影响 |
| --- | --- |
| `users` 表生产行数 = R2 post 基线(不在此列出具体数字,以 SA-A post 为准);`jst_erp_r3_test` 与生产同规模 | SA-B list 分页默认 `page_size=20` 即可覆盖大部分场景;pagination 必须返回真实 `total` |
| `org_move_requests` 表(R2 migration 065)目前 **0 条**;生产无历史数据 | SA-B integration test 必须自建样本(`defer` 清理) |
| `users.team` 是**单值** string 列;`users.managed_departments` / `managed_teams` 由 `user_managed_scope` 外表承载(v0.9);无 `team_codes` / `primary_team_code` / `avatar_url` 列(R1.7-B §5 已确认) | `team_codes[]` / `primary_team_code` / `avatar` 服务端**必须 no-op**(解码后丢弃 · 不返回 400) |
| `permission_logs` 表(R1 已落 · 实际列名 `action_type` · mig 029)承载 SA-B **所有**审计事件;**`operation_logs` 表在 v1 DDL 未建**,前端 ACL 里 `admin_operation_logs` 只是页面标识 | SA-B 所有审计统一落 `permission_logs.action_type`(§6 列名术语"event_type"读作该列);**不得为 SA-B 新建 `operation_logs` 表**(违反 §9 no-new-migrations 硬约束);§6 表中 `operation_logs.event_type` 当前一律写到 `permission_logs.action_type` |
| 生产 `users.roles` 的 SuperAdmin 数量 = 1(固定账号);HRAdmin = 1(`HRAdmin` 用户);DeptAdmin 分布在 4 部门 | §5.4 的"不可授 SuperAdmin / 不可授 DeptAdmin" 分支必须有 SQL 级防御 + 单测 |

---

## 3. 交付范围

### 3.1 Domain / 模型层

| 文件 | 作用 |
| --- | --- |
| `domain/org_move_request.go` | `OrgMoveRequest` struct + `OrgMoveRequestState` 枚举(`pending_super_admin_confirm` / `approved` / `rejected`);字段对齐 migration 065 ;`OrgMoveRequestEvent` 常量(用于审计 payload);**不加新列** |
| `domain/my_org_profile.go` | `MyOrgProfile` struct(`Department` / `Team` / `ManagedDepartments[]` / `ManagedTeams[]` / `Roles[]`);字段对齐 OpenAPI `MyOrgProfile` schema |
| `domain/auth_identity.go`(**只追加**,不改现有字段)| 追加 `PermissionAction` 常量:`PermissionActionOrgMoveRequested` / `PermissionActionOrgMoveApproved` / `PermissionActionOrgMoveRejected` / `PermissionActionUserActivated` / `PermissionActionUserDeactivated` |

**严禁**:改 `domain/auth_identity.go` 的 `User` / `FrontendAccessView` 现有字段;加 `users.avatar_url` / `users.team_codes` / `users.primary_team_code` 任何新 DDL 列。

### 3.2 Service 层

| 包 | 文件 | 职责 |
| --- | --- | --- |
| `service/identity_service.go`(**扩展现有**)| 同文件追加 | `UpdateUser` 现有方法内**按 §5.4 字段级授权矩阵收紧**(下文 §4 全表对照);`ChangePassword` 现有方法**确认** `old_password/new_password/confirm` 三验证齐全(缺的补) |
|  |  | 新增 `ActivateUser(ctx, actor, targetID) *AppError` / `DeactivateUser(ctx, actor, targetID) *AppError`:TeamLead 限本组、DeptAdmin 限本部门、HRAdmin/SuperAdmin 全局;写 `permission_logs` + `operation_logs` |
|  |  | 新增 `DeleteUser(ctx, actor, targetID, reason string) *AppError`:**仅 SuperAdmin**;必填 `reason`;软删实现(若 `users.deleted_at` 列不存在,退回 `users.status = 'deleted'` + 禁 login 的现有机制,**不加 DDL 列**);写审计 |
|  |  | 新增 `GetMe(ctx, actor) *User` / `UpdateMe(ctx, actor, p UpdateMeParams) (*User, *AppError)`(复用 `UpdateUser` 内部,但 scope 锁 `actor.ID == targetID`);`GetMyOrg(ctx, actor) *MyOrgProfile` |
| `service/org_move_request`(**新建**)| `service.go` | `Create(ctx, actor, sourceDeptID, p CreateOrgMoveRequestParams) (*OrgMoveRequest, *AppError)` — DeptAdmin/HRAdmin/SuperAdmin 发起;断言 `sourceDeptID ∈ actor.managed_departments`(DeptAdmin 分支);断言目标用户当前所属部门 = sourceDeptID;写 `org_move_requests` + 审计 |
|  | `service.go` | `List(ctx, actor, filter ListOrgMoveFilter) (items []OrgMoveRequest, total int, *AppError)` — 服务端按 caller 角色强制过滤(DeptAdmin 仅可见本管辖部门发起的请求;SuperAdmin/HRAdmin 全局);query `state` / `user_id` / `source_department_id` / `page` / `page_size` |
|  | `service.go` | `Approve(ctx, actor, requestID int64) *AppError` — **仅 SuperAdmin**;事务:① UPDATE `org_move_requests` 置 `state=approved, decided_by_user_id=actor.ID, decided_at=NOW()`;② UPDATE `users` `department = target_department_id` + 清空 `team`(因为跨部门 team 语义断开;由 HR 后续重新分组);③ 写 `operation_logs.event_type = user_department_changed_by_admin` payload 含 source/target/actor/reason |
|  | `service.go` | `Reject(ctx, actor, requestID int64, reason string) *AppError` — **仅 SuperAdmin**;必填 reason;UPDATE 置 `state=rejected, reject_reason, decided_by, decided_at`;写审计 `org_move_rejected_by_admin` |
|  | `service_test.go` | 表驱动覆盖 create/list/approve/reject × 各角色 × 各状态组合 |

**严禁**:新建 `service/me` 包(重复);改 R3 落地的 `service/permission/` 任何文件;动 `service/asset_*` / `service/blueprint/` 等 non-SA-B 包。

### 3.3 Repo 层

| 文件 | 作用 |
| --- | --- |
| `repo/org_move_request_repo.go`(**新建**)| `OrgMoveRequestRepo` interface:`Create` / `Get` / `List(filter)` / `UpdateState` |
| `repo/mysql/org_move_request_repo.go`(**新建**)| MySQL 实现;SELECT 必须显式列名(不 `SELECT *`);UPDATE 用 CAS(`WHERE state='pending_super_admin_confirm'` 防止 decide 冲突)返回 rows affected 判断是否 `409` |
| `repo/user_repo.go`(**扩展现有**)| 追加方法:`Activate(ctx, id)` / `Deactivate(ctx, id)` / `SoftDelete(ctx, id, deletedBy int64, reason string)`;若现有 user_repo 已有等价方法,**复用**而非新加 |
| `repo/mysql/user_repo.go`(**扩展现有**)| 同上 MySQL 实现 |

**严禁**:改 v0.9 现有 user_repo 的 `CreateUser` / `UpdateUser` / `GetUser` / `ListUsers`(只追加 Activate/Deactivate/SoftDelete 方法)。

### 3.4 Transport 层

#### 3.4.1 替换 501 stub(6 条)

| handler | 路径 | 角色门控 |
| --- | --- | --- |
| `handleMeGet` | `GET /v1/me` | 登录用户(`session_token_authenticated`)|
| `handleMePatch` | `PATCH /v1/me` | 登录用户;服务端忽略 `avatar` 字段(no-op) |
| `handleMeChangePassword` | `POST /v1/me/change-password` | 登录用户 · 服务端强制 `old_password` 匹配 + `new_password == confirm` |
| `handleMeOrg` | `GET /v1/me/org` | 登录用户 |
| `handleUserActivate` | `POST /v1/users/{id}/activate` | SuperAdmin / HRAdmin / DeptAdmin(本部门) / TeamLead(本组)|
| `handleUserDeactivate` | `POST /v1/users/{id}/deactivate` | 同上 |

#### 3.4.2 org-move-requests 4 handler(全新)

| handler | 路径 | 角色门控 |
| --- | --- | --- |
| `handleOrgMoveRequestCreate` | `POST /v1/departments/{id}/org-move-requests` | DeptAdmin(源部门)/ HRAdmin / SuperAdmin;返回 **201** |
| `handleOrgMoveRequestList` | `GET /v1/org-move-requests` | SuperAdmin / HRAdmin / DeptAdmin;服务端按角色强制过滤 |
| `handleOrgMoveRequestApprove` | `POST /v1/org-move-requests/{id}/approve` | **仅 SuperAdmin**;CAS 防重复;`409` on already-decided |
| `handleOrgMoveRequestReject` | `POST /v1/org-move-requests/{id}/reject` | **仅 SuperAdmin**;必填 `reason`;CAS 防重复 |

新建文件:
- `transport/handler/me.go`(4 个 `handleMe*`)
- `transport/handler/org_move_request.go`(4 个 `handleOrgMoveRequest*`)
- `transport/handler/user_admin_activate.go`(2 个 activate/deactivate · 不与 `user_admin.go` 合并是因为 SA-B 新代码需隔离 owner)
- `transport/handler/user_admin_delete.go`(DELETE /v1/users/{id} · SuperAdmin 门控 · 必填 reason · 返回 204)

#### 3.4.3 强化现有 `PatchUser`(扩展 · 不替换)

`transport/handler/user_admin.go` `PatchUser` 现有方法内:
1. 解析 body:除现有 `display_name/status/employment_type/department/team/group/email/mobile/managed_departments/managed_teams` 外,**新增** `roles[]` / `avatar(ignored)` / `team_codes(ignored)` / `primary_team_code(ignored)` 字段
2. `avatar` / `team_codes` / `primary_team_code` 解码后**丢弃不存**(R1.7-B §5.1、§5.2 明文)
3. `roles[]` 非空时走新增的 `identityService.SetUserRoles(ctx, actor, targetID, roles)` → 内部按 §5.4 矩阵(DeptAdmin 不得授 DeptAdmin/SuperAdmin/HRAdmin;HRAdmin 不得授 SuperAdmin;TeamLead 整条 roles 字段 403)
4. 字段级违规返回 `403` + `ErrorResponse.error.code = "user_update_field_denied_by_scope"`(自取 code 字符串,**不加入** deny_code 枚举)。**注意 deny gate 次序**(SA-B.2 实测确认):DeptAdmin / TeamLead 访问**不在自己 scope 内**的目标用户时,**先命中 read-scope gate** → 返回 `department_scope_only` / `team_scope_only`(参见 `service/identity_service.go` 内的 `identityPermissionDenied("department_scope_only", ...)`),根本到不了 field-update gate。两种 deny_code 在 `403` 响应上**等价合法**,前者更严格(连读都禁,等价于"不存在"));测试断言应同时接受两种 code · 不要求强制走到 field gate。

#### 3.4.4 路由挂载

`transport/http.go` 内追加:
```go
v1.GET("/me", withAuthenticated(...), meH.Get)
v1.PATCH("/me", withAuthenticated(...), meH.Patch)
v1.POST("/me/change-password", withAuthenticated(...), meH.ChangePassword)
v1.GET("/me/org", withAuthenticated(...), meH.GetOrg)

v1.DELETE("/users/:id", access(..., RoleSuperAdmin), userAdminDeleteH.Delete)
v1.POST("/users/:id/activate", access(..., RoleSuperAdmin, RoleHRAdmin, RoleDeptAdmin, RoleTeamLead), userAdminActivateH.Activate)
v1.POST("/users/:id/deactivate", access(..., RoleSuperAdmin, RoleHRAdmin, RoleDeptAdmin, RoleTeamLead), userAdminActivateH.Deactivate)

v1.POST("/departments/:id/org-move-requests", access(..., RoleSuperAdmin, RoleHRAdmin, RoleDeptAdmin), orgMoveH.Create)
v1.GET("/org-move-requests", access(..., RoleSuperAdmin, RoleHRAdmin, RoleDeptAdmin), orgMoveH.List)
v1.POST("/org-move-requests/:id/approve", access(..., RoleSuperAdmin), orgMoveH.Approve)
v1.POST("/org-move-requests/:id/reject", access(..., RoleSuperAdmin), orgMoveH.Reject)
```

**保留旧路由不动**:`/v1/auth/me` / `/v1/auth/password` 继续存在(v0.9 前端可能仍在用);SA-B 只是**加挂** v1 路径,不下线旧路径(下线是 R6 的事)。

---

## 4. §5.4 字段级授权矩阵(权威 · 必须逐字段实装)

| 字段 | SuperAdmin | HRAdmin | DeptAdmin(本部门)| TeamLead(本组)| 否则返回 |
| --- | --- | --- | --- | --- | --- |
| `display_name` / `email` / `mobile` | ✓ | ✓ | ✓(本部门用户)| ✗ | 403 `user_update_field_denied_by_scope` |
| `department` | ✓(直改)| ✓(直改)| 仅本部门内调组(即 `new_department == caller.department`);跨部门必须走 `/v1/departments/{id}/org-move-requests` | ✗ | 403 |
| `team` / `group` | ✓ | ✓ | ✓(本部门内)| ✗ | 403 |
| `roles[]` | ✓ 全集 | ✓(**不可授 `SuperAdmin`**)| ✓(**不可授 `DeptAdmin` / `SuperAdmin` / `HRAdmin`** · 且仅限本部门用户)| ✗(TeamLead 整条 roles 字段 403)| 403 `role_assignment_denied_by_scope` |
| `status`(经 activate/deactivate 端点)| ✓ | ✓ | ✓(本部门)| ✓(**仅本组成员 · 仅改 status**)| 403 |
| `employment_type` | ✓ | ✓ | ✓(本部门)| ✗ | 403 |
| `managed_departments` / `managed_teams` | ✓ | ✓ | ✗(DeptAdmin 不改管辖)| ✗ | 403 |
| `avatar` / `team_codes[]` / `primary_team_code` | — | — | — | — | **no-op(解码后丢弃 · 不返 400 · 不返 403)** |

**DeptAdmin 跨部门移动**:严格走 `POST /v1/departments/{id}/org-move-requests` → `POST /v1/org-move-requests/{id}/approve`,**不允许**在 `PATCH /v1/users/{id}` 里用 `department` 字段跨部门直改。服务端检测到 `caller.role=DeptAdmin AND new_department ≠ caller.department AND new_department ≠ old_department` → 403 + 错误 code `user_update_field_denied_by_scope`。

---

## 5. 审计事件清单(SA-B 必须落的写入)

| 触发端点 | `permission_logs.event_type` 或 `operation_logs.event_type` | payload keys |
| --- | --- | --- |
| `POST /v1/users` | `user_created` | `actor`, `target_user_id`, `department`, `team`, `roles` |
| `PATCH /v1/users/{id}`(非 roles 字段变)| `user_updated` | `actor`, `target`, `changed_fields[]` |
| `PATCH /v1/users/{id}`(roles 字段变)| `role_assigned` / `role_removed` | `actor`, `target`, `roles_added[]`, `roles_removed[]` |
| `PATCH /v1/users/{id}`(department 变)| `user_department_changed_by_admin`(SuperAdmin/HRAdmin 直改)或 `user_department_changed_via_org_move`(走 org-move approve 触发) | `actor`, `target`, `from_department`, `to_department` |
| `PUT /v1/users/{id}/password` | `password_reset` | `actor`, `target` |
| `POST /v1/me/change-password` | `password_changed` | `actor=target` |
| `POST /v1/users/{id}/activate` | `user_status_changed` `{ to: "active" }` | `actor`, `target`, `to=active` |
| `POST /v1/users/{id}/deactivate` | `user_status_changed` `{ to: "disabled" }` | `actor`, `target`, `to=disabled` |
| `DELETE /v1/users/{id}` | `user_deleted` | `actor`, `target`, `reason` |
| `POST /v1/departments/{id}/org-move-requests` | `org_move_requested` | `actor`, `target_user_id`, `source_dept`, `target_dept`, `reason` |
| `POST /v1/org-move-requests/{id}/approve` | `org_move_approved` + `user_department_changed_by_admin`(两条事件同事务)| 同 create + `decided_by` |
| `POST /v1/org-move-requests/{id}/reject` | `org_move_rejected` | `actor`, `request_id`, `reason` |

所有审计写入必须与业务写入**同事务**(若业务事务回滚,审计不应残留)。

---

## 6. DON'T TOUCH

- `docs/api/openapi.yaml`(R1.7-B 已补齐 · 若发现 14 条路径 schema 不够,**立即回主对话**而非 abort;主对话裁决是否开 R1.7-B.1 补丁)
- `db/migrations/001~068`(SA-B 不加迁移;`avatar_url` / `team_codes` / `primary_team_code` 列 v1 不新增,R5+ 单独轮落)
- R3 落地的 `service/blueprint` / `service/module` / `service/permission` / `service/task_pool` / `service/task_cancel` / `service/task_aggregator` / `service/module_action`
- R4-SA-A 落地的 `service/asset_center` / `service/asset_lifecycle`
- 任何 `service/notification` / `transport/ws` / `service/erp_bridge/by_code.go` / `service/design_source/search.go` / `service/task_drafts/*` 相关(R4-SA-C)
- 任何 `service/search` / `service/report_l1` / 前端 dist 资源(R4-SA-D / R5)
- `/v1/org/departments` / `/v1/org/teams` 已挂载的部门/组 CRUD(现有 v0.9 路由继续承载 IA §6 · SA-B 不动;若 IA §6 明显缺路径,**回报主对话**而非自行新增)
- `/v1/auth/me` / `/v1/auth/password` 旧路径(保留 · R6 下线)
- 生产 `jst_erp` 的任何写入;本轮所有 integration 走 `jst_erp_r3_test`
- R3.5 / SA-A 的 `testsupport/r35` DSN 守卫(只用,不改)

---

## 7. 执行环境

**复用 R3.5 + SA-A 的 `jst_erp_r3_test`**(在 `jst_ecs`):

```bash
ssh jst_ecs 'mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -p"$DB_PASS" -e "SHOW DATABASES LIKE '\''jst_erp_r3_test'\'';"'

DSN=$(ssh jst_ecs 'bash /root/ecommerce_ai/r3_5/build_test_dsn.sh')
MYSQL_DSN="$DSN" R35_MODE=1 /home/wsfwk/go/bin/go test ./service/identity_service_test.go ./service/org_move_request/... -tags=integration -count=1 -v
```

**严格守则**:

- 所有 integration test 必须走 `testsupport/r35.MustOpenTestDB(t)`,DSN 不以 `_r3_test` 结尾即 `t.Fatalf`
- SA-B 追加的测试样本 `user_id >= 30000` 段隔离(SA-A 用 `task_id >= 20000` 段;SA-B 用 `user_id >= 30000` 段);`org_move_request.id` 无需特殊隔离,测试后 `DELETE WHERE id IN (...)` 清理

---

## 8. 测试要求

### 8.1 单元测试(100% 绿)

- `service/identity_service_authorize_user_update_v1_5_4_test.go`(**新文件**)—— 表驱动覆盖 §5.4 矩阵 4 角色 × 7 字段类别 = 28 组合;每组合两个方向(被动拒 / 主动过)
- `service/identity_service_activate_test.go` / `_deactivate_test.go`:TeamLead 跨组 / DeptAdmin 跨部门 / SuperAdmin 全局 覆盖
- `service/org_move_request/service_test.go`:create / list(3 角色) / approve / reject / CAS 409
- `transport/handler/me_test.go`:4 端点的 happy path + 未登录 401 + `avatar` 解码后不进 DB
- `transport/handler/org_move_request_test.go`:4 端点的 happy path + 非 SuperAdmin approve/reject → 403
- `transport/handler/user_admin_delete_test.go`:非 SuperAdmin → 403 · 缺 reason → 400 · 成功 204
- `transport/handler/user_admin_activate_test.go`:TeamLead 操作本组 204 · 操作他组 403

### 8.2 Integration Tests(build tag `integration`)

| # | 断言 | 对应验收 |
| --- | --- | --- |
| SA-B-I1 | `GET /v1/me` 返回 `WorkflowUser`(当前登录用户完整 profile);`avatar` 字段返回 `null`(占位) | IA §7.2 / R1.7-B §3 |
| SA-B-I2 | `PATCH /v1/me` 修改 `display_name/mobile/email` 成功写入 DB;body 里的 `avatar` / `team_codes` / `primary_team_code` 写入后 DB 的 `users` 行这三字段保持原值(no-op 证据) | IA §7.2 / R1.7-B §5.1 |
| SA-B-I3 | `POST /v1/me/change-password` old_password 错 → 400 `old_password_mismatch`;new_password ≠ confirm → 400 `password_confirmation_mismatch`;全对 → 204 + `password_hash` 更新 | IA §7.2 |
| SA-B-I4 | `GET /v1/me/org` 返回 `MyOrgProfile`;DeptAdmin 测试账号的 `managed_departments` 非空,Member 测试账号的 `managed_*` 为空数组 | IA §7.2 |
| SA-B-I5 | `PATCH /v1/users/{id}` DeptAdmin 给其他部门用户改字段 → 403 `user_update_field_denied_by_scope`;HRAdmin 授 `SuperAdmin` → 403 `role_assignment_denied_by_scope`;TeamLead 改 `display_name` → 403 | IA §5.4 / R1.7-B §5.7 |
| SA-B-I6 | `POST /v1/users/{id}/activate` TeamLead 对本组成员 204 `users.status=active`;TeamLead 对他组成员 → 403 | IA §5.3 / R1.7-B §5.8 |
| SA-B-I7 | `POST /v1/departments/{id}/org-move-requests` DeptAdmin 源部门发起 → 201 `OrgMoveRequest.state=pending_super_admin_confirm`;用户部门在 approve 前仍为源部门;事件 `org_move_requested` 已写入 | IA §5.2 / R1.7-B §5.4 |
| SA-B-I8 | `POST /v1/org-move-requests/{id}/approve` SuperAdmin 成功 → 204 + `users.department` 更新到 target + `users.team` 清空 + 事件 `user_department_changed_by_admin` 已写入;重复调用 → 409 `org_move_request_already_decided` | IA §5.2 / R1.7-B §5.5 |
| SA-B-I9 | `POST /v1/org-move-requests/{id}/reject` SuperAdmin 成功 → 204 + `state=rejected, reject_reason=...`;用户部门保持源部门;非 SuperAdmin → 403;缺 reason → 400 | IA §5.2 / R1.7-B §5.6 |
| SA-B-I10 | `GET /v1/org-move-requests?state=pending_super_admin_confirm` DeptAdmin 只看到本管辖部门发起的请求(创建两条,一条本部门一条他部门,返回仅本部门一条) | R1.7-B §5.9 |
| SA-B-I11 | `DELETE /v1/users/{id}` SuperAdmin + reason → 204;`users.status=deleted`(软删);非 SuperAdmin → 403;缺 reason → 400 | IA §5.3 |

### 8.3 全量回归

```bash
/home/wsfwk/go/bin/go build ./...
/home/wsfwk/go/bin/go test ./... -count=1
MYSQL_DSN=... R35_MODE=1 /home/wsfwk/go/bin/go test ./... -tags=integration -count=1
/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
```

### 8.4 生产 Probe(零污染证据 · SA-A v2.1 模板)

Probe 脚本已由主对话打包在仓库:[`tmp/sa_b_probe_readonly.sh`](../tmp/sa_b_probe_readonly.sh)(严格只读 · 无 INSERT/UPDATE/DELETE/DDL)。**不要自行改写 probe SQL,也不要在本轮创建新的 probe 脚本**;如脚本缺字段报回主对话。

两阶段执行(pre / post):

```bash
# Phase 1: SA-B 开跑前(记录 baseline · 取 mysql_server_time_utc 作为 sa_b_start_time)
scp tmp/sa_b_probe_readonly.sh jst_ecs:/tmp/sa_b_probe.sh
ssh jst_ecs 'bash /tmp/sa_b_probe.sh' | tee docs/iterations/r4_sa_b_probe_pre.log

# 从 r4_sa_b_probe_pre.log 的 === META === 段摘出 mysql_server_time_utc,记为 $SA_B_START
# 例如: SA_B_START='2026-04-17 12:00:00'

# Phase 2: SA-B 跑完后(传 baseline · §8.4 四条聚合 B1/B2/B3/B4 应全 0)
ssh jst_ecs "bash /tmp/sa_b_probe.sh '$SA_B_START'" | tee docs/iterations/r4_sa_b_probe_post.log
```

post 阶段 SA-B 控制字段聚合硬门(对应脚本 B1 / B2 / B3 / B4 段输出):

| 脚本段 | 聚合 | 期望值 | 含义 |
| --- | --- | --- | --- |
| B1 | `users_updated_after_sa_b_start` | **0** | SA-B 未改生产任何 user(PATCH /v1/users, /v1/me, activate, deactivate, delete 均未触生产) |
| B2 | `org_move_created_after_sa_b_start` | **0** | SA-B 未写生产 `org_move_requests`(生成/审批/驳回皆走测试库) |
| B3 | `sa_b_permission_logs_after_start` | **0** | SA-B 未写 §6 审计事件到生产 `permission_logs`(列名 `action_type` · mig 029) |
| B4 | `users_soft_deleted_after_start` | **0** | SA-B 未软删生产用户(DELETE /v1/users/{id} 未打到生产) |

**行数 drift 不是 abort 条件**(沿用 SA-A v2.1 决策):生产 live traffic 可能在 §A 基线中增加 `users` / `permission_logs` / `tasks` 等行,SA-B 的"零污染"证据是上述 B1/B2/B3/B4 四条聚合全 0,**而非表行数一致**。§H 的最近 5 条 `permission_logs` 仅供观察 live traffic 形态参考。

---

## 9. 交付物清单

1. `domain/org_move_request.go` · `my_org_profile.go` · 追加 PermissionAction 常量到 `auth_identity.go`
2. `service/identity_service.go` 扩展(§5.4 字段级授权 + activate/deactivate/delete/getMe/updateMe/getMyOrg)
3. `service/org_move_request/*`(service + test)
4. `repo/org_move_request_repo.go` + `repo/mysql/org_move_request_repo.go`
5. `repo/user_repo.go` / `repo/mysql/user_repo.go` 追加 Activate/Deactivate/SoftDelete 方法
6. `transport/handler/me.go`(4 handler)· `org_move_request.go`(4 handler)· `user_admin_activate.go`(2 handler)· `user_admin_delete.go`(1 handler)· `user_admin.go` PatchUser 扩展
7. `transport/http.go` 追加 10 条路由挂载
8. 11 条 integration test(build tag `integration`)
9. `docs/iterations/V1_R4_SA_B_REPORT.md`:

**报告强制章节**:

- `## Scope` · 14 条 handler 清单 + 每条的 deny 路径 + 返回码矩阵
- `## §5.4 Field-Level Authorization Matrix` · 28 组合单测结果表
- `## 11 Integration Assertions` · SA-B-I1 ~ SA-B-I11 的实际 SQL / HTTP 数据证据
- `## Audit Events` · 12 类事件的示例 payload JSON(从 integration test 抓出)
- `## Route Table Diff` · `transport/http.go` 新增 10 条路由 · 保留 `/v1/auth/me` / `/v1/auth/password` 旧路由的证据
- `## No-Op Placeholder Fields` · `avatar` / `team_codes` / `primary_team_code` 解码后丢弃的 3 个具体测试用例
- `## Test DB Touch` · `jst_erp_r3_test` 中 SA-B 测试数据范围(`user_id >= 30000` 段 + 本轮创建的 org_move_request.id);执行后的 defer 清理证据
- `## Production Probe Diff` · 4 条控制字段聚合全 0 的 SQL + 输出
- `## OpenAPI Conformance` · 0/0
- `## Known Non-Goals` · IA §6 部门/组增删改(沿用 v0.9 路由)+ `avatar` 实际存储(R5+)+ 多组模型 team_codes 实际存储(R5+)+ 通知推送(SA-C)

---

## 10. 失败终止条件

- 14 条 R4-SA-B 路径在 OpenAPI 里 schema 字段与 §5 / §7.2 对不上 → **回报主对话**(而非 abort 重起一轮;R1.7-B 已签字,若有残漏由主对话裁决是否开 R1.7-B.1)
- 任何 SA-B 代码触发非白名单表的写入(写 `tasks` / `task_modules` / `task_assets` / `asset_*` / `notifications` / 搜索索引 / 报表聚合 任一)→ abort
- 任何 DSN 守卫失效(测试跑出 `jst_erp` 写入)→ abort
- integration 断言任一 FAIL
- 生产 probe **控制字段聚合**任一 ≠ 期望值(见 §8.4 表)→ abort;**表行数 drift 不算违反**
- 需要新 deny_code 枚举才能通过测试 → abort(改主 §6.2 是下一轮决策)
- 需要新 migration 才能通过测试 → abort(`avatar` / `team_codes` / `primary_team_code` 一律 no-op,若测试要求真持久化 → 改测试期望)
- 需要在 `transport/http.go` 下线 `/v1/auth/me` 或 `/v1/auth/password` 才能通过测试 → abort(旧路径保留是 R6 决策)

---

## 11. 控制字段白名单(SA-B 允许写入的表/列)

| 表 | 允许写入的列 | 备注 |
| --- | --- | --- |
| `users` | `status` / `team` / `department` / `display_name` / `email` / `mobile` / `employment_type` / `password_hash` / `updated_at` | **不含** `avatar_url`(不存在)/ `team_codes`(不存在)/ `primary_team_code`(不存在) |
| `user_roles` | 全部列(create/delete 行) | roles[] 分配 |
| `org_move_requests` | 全部列(create/update 行) | R2 migration 065 新表 |
| `permission_logs` | 全部列(insert only) | 审计 |
<!-- operation_logs 表 v1 DDL 未建;SA-B 不得新增迁移(§9)· 所有审计事件统一落 permission_logs.action_type;本行保留空以标注"不可写" -->
| `user_managed_scope`(若 v0.9 存在,核对)| `managed_departments` / `managed_teams` | 仅 HR/SuperAdmin 改管辖 |

**绝对禁止**:写 `tasks*` / `task_modules` / `task_module_events` / `task_assets` / `asset_*` / `notifications` / `task_drafts` / `reference_file_refs` / `reports_*` 任一表的任一列。

---

## 12. 给 Codex 的最后一句话

> SA-B 是 R4 的第 2 轮,前面有 SA-A 已签字,后面还有 C/D 2 轮。
> 你的范围只有"**组织菜单下的用户管理 + 跨部门调配 + 个人中心**",资产 / 任务 / 通知 / 搜索 / 报表 全不归你。
> v0.9 已有的 `identityService.UpdateUser` / `ChangePassword` / `userAdminH.PatchUser` 是你的**起点**,不要重建;按 IA §5.4 收紧授权矩阵 + 补齐 activate/deactivate/delete/me 四族端点 + 建 org_move_request 新功能即可。
> 遇到需要改 OpenAPI / 改 DDL / 加 deny_code / 动其他 owner 目录的冲动,**立即 abort** 并报告,让主对话裁决。
> 不要在生产 `jst_erp` 上跑任何东西,测试库是 `jst_erp_r3_test`;控制字段聚合全 0 才是零污染证据,表行数漂移是正常 live traffic。

---

## 变更记录

| 版本 | 日期 | 变更 |
| --- | --- | --- |
| v1 | 2026-04-17 | 初稿 · 基于 R1.7-B v1.0 / IA v1.1 §5 §7.2 / SA-A v2.1 失败条件模板固化。14 handler 清单锁定(`x-owner-round: R4-SA-B`);§5.4 字段级授权 28 组合矩阵展开;§5.2 跨部门 org-move-request 4 端点全流程;现有 `identityService` / `userAdminH` 扩展优先(不重建);`avatar` / `team_codes` / `primary_team_code` 服务端 no-op · DDL 零新增;`/v1/auth/me` / `/v1/auth/password` 旧路径保留;integration 走 `jst_erp_r3_test` · 测试数据 `user_id >= 30000` 段隔离;生产 probe 用控制字段聚合而非行数 drift |
