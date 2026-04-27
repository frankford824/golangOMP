# V1 · R1.7-B OpenAPI SA-B 补丁 · 执行报告

> 版本:**v1.0 · 已签字生效**
> 日期:2026-04-17
> 触发:R4-SA-B 起草前置小补丁(按 R1.7-A 报告 §5 "SA-B 5 block 待补")
> 执行者:主对话(架构/Prompt designer)直接打补丁(与 R1.7-A 同款路径 A)
> 范围:**SA-B 14 ops 全扫 + 实补 11 path 编辑 + 5 新 schema + `WorkflowUser.avatar` 扩展**
> 认证文档:`docs/V1_INFORMATION_ARCHITECTURE.md` v1.1 §5 §7.2

---

## 1. 背景

R1.7-A 报告 §5 明列 SA-B 的 5 block 待补(me 三端 schema、change-password body、users PATCH body),并标记"需裁决 v1 多组模型 vs v0.9 单组 `team`"与"需裁决扩 Actor 还是新建 MyProfile"两项架构问题。

本轮 R1.7-B 在架构问答中收敛了四个决策点:

| 决策 | 选项 | 含义 |
| --- | --- | --- |
| Q1 /v1/me response schema | **A3** | `GET /v1/me` → `WorkflowUser`;`GET /v1/me/org` → 新 `MyOrgProfile`;`Actor` schema 不动 |
| Q2 v1 多组模型 | **B2** | 延到 R5+ 独立轮落 DDL;v1 用 `team` 单值 + `managed_teams[]` 表达 |
| Q3 PATCH /v1/users/{id} body | **C2** | 补 `roles[]`;保留 `team_codes[]` / `primary_team_code` optional 占位 |
| X1 占位字段服务端行为 | **X1** | schema 保留 · 服务端收到即忽略 · DDL 零变 · `WorkflowUser` 加 `avatar:nullable` 同样忽略 · 全部延 R5+ 单独 migration 轮落地 |

---

## 2. SA-B 14 ops gap 全扫

| # | 路径 | 方法 | 扫描结论 |
| --- | --- | --- | --- |
| 1 | `/v1/users` | GET | ✅ 已完整(query/response/pagination 齐全) |
| 2 | `/v1/users` | POST | ✅ 已完整(`CreateManagedUserRequest` 已含 `roles[]`) |
| 3 | `/v1/users/{id}` | PATCH | ❌ 缺 `roles[]` · 缺 403/404 · 按 C2 需加 `team_codes/primary_team_code/avatar` 占位 |
| 4 | `/v1/users/{id}` | DELETE | ✅ 已完整 |
| 5 | `/v1/me` | GET | ❌ response `Actor` 语义错位(按 A3 应为 `WorkflowUser`) · 残留 501 |
| 6 | `/v1/me` | PATCH | ❌ 缺 requestBody · response `Actor` 语义错位 · 残留 501 |
| 7 | `/v1/me/change-password` | POST | ❌ 缺 requestBody · 残留 501 |
| 8 | `/v1/me/org` | GET | ❌ response `Actor` 语义错位(按 A3 应为新 `MyOrgProfile`) · 残留 501 |
| 9 | `/v1/users/{id}/activate` | POST | ❌ 缺 403/404 ErrorResponse · 残留 501 |
| 10 | `/v1/users/{id}/deactivate` | POST | ❌ 同 9 |
| 11 | `/v1/departments/{id}/org-move-requests` | POST | ❌ 缺 requestBody · 缺 403/404 · 残留 501 |
| 12 | `/v1/org-move-requests` | GET | ❌ 缺 query(state/user_id/page/size) · 缺 pagination · 残留 501 |
| 13 | `/v1/org-move-requests/{id}/approve` | POST | ❌ 缺 403/404/409 · 残留 501 |
| 14 | `/v1/org-move-requests/{id}/reject` | POST | ❌ 缺 requestBody(reason required) · 缺 403/404/409 · 残留 501 |

**需编辑**:11 path(第 3 / 5~14)+ 5 新 schema + 1 现有 schema 扩展(`WorkflowUser.avatar`)

**无改动**:第 1 / 2 / 4(已完整)

---

## 3. 本轮实补清单(16 处)

### 3.1 新增 schema(5 个)

| # | schema | 作用 | 权威 |
| --- | --- | --- | --- |
| S1 | `MyOrgProfile` | `GET /v1/me/org` response `data`;含 `department / team / managed_departments[] / managed_teams[] / roles[]` | IA §7.2(我的组织) |
| S2 | `UpdateMyProfileRequest` | `PATCH /v1/me` requestBody;含 `display_name / mobile / email / avatar(placeholder)` | IA §7.2(账户信息 · 编辑) |
| S3 | `ChangePasswordRequest` | `POST /v1/me/change-password` requestBody;`old_password / new_password / confirm` 三字段 required | IA §7.2(安全 · 改密) |
| S4 | `CreateOrgMoveRequestPayload` | `POST /v1/departments/{id}/org-move-requests` requestBody;`user_id / target_department_id / reason` | IA §5.2 |
| S5 | `RejectReasonRequest` | `POST /v1/org-move-requests/{id}/reject` requestBody;`reason` required | IA §5.2 |

### 3.2 现有 schema 扩展(1 个)

| # | schema | 变更 | 注释 |
| --- | --- | --- | --- |
| S6 | `WorkflowUser` | 加字段 `avatar: { type: string, nullable: true }` | X1 placeholder · 服务端忽略 · `users` DDL 无 `avatar_url` 列 · R5+ 单独落地 |

### 3.3 path 编辑(11 个)

| # | 路径 | 方法 | 编辑 |
| --- | --- | --- | --- |
| P1 | `PATCH /v1/users/{id}` | — | body 加 `roles[]` / `avatar(placeholder)` / `team_codes(placeholder)` / `primary_team_code(placeholder)`;加 `403`/`404` ErrorResponse |
| P2 | `GET /v1/me` | — | response `data` 从 `Actor` → `WorkflowUser`;加 `x-rbac-placeholder` `session_token_authenticated`;**移除 501** |
| P3 | `PATCH /v1/me` | — | 新增 requestBody `UpdateMyProfileRequest`;response `data` 从 `Actor` → `WorkflowUser`;加 `400` ErrorResponse;**移除 501** |
| P4 | `POST /v1/me/change-password` | — | 新增 requestBody `ChangePasswordRequest`;加 `400` ErrorResponse;**移除 501** |
| P5 | `GET /v1/me/org` | — | response `data` 从 `Actor` → `MyOrgProfile`;**移除 501** |
| P6 | `POST /v1/users/{id}/activate` | — | 加 `x-rbac-placeholder`(SuperAdmin/HRAdmin/DepartmentAdmin/TeamLead);加 `403`/`404`;**移除 501** |
| P7 | `POST /v1/users/{id}/deactivate` | — | 同 P6 |
| P8 | `POST /v1/departments/{id}/org-move-requests` | — | 新增 requestBody `CreateOrgMoveRequestPayload`;response 从 `200` 改 `201`(REST 语义);加 `400`/`403`/`404`;**移除 501** |
| P9 | `GET /v1/org-move-requests` | — | 加 query `state` / `user_id` / `source_department_id` / `page` / `page_size`;response 加 `pagination`;**移除 501** |
| P10 | `POST /v1/org-move-requests/{id}/approve` | — | 加 `x-rbac-placeholder`(SuperAdmin);加 `403`/`404`/`409`;**移除 501** |
| P11 | `POST /v1/org-move-requests/{id}/reject` | — | 新增 requestBody `RejectReasonRequest`;加 `400`/`403`/`404`/`409`;**移除 501** |

### 3.4 不做的事

- **不新增任何 DDL / migration**(Q2=B2 + X1 决策;`avatar / team_codes / primary_team_code` 服务端统一忽略)
- **不动 `Actor` schema**(Q1=A3;Actor 仍承载"事件发送者 / 接单人"轻量语义,19 个 $ref 零影响)
- **不改 SA-A / SA-C / SA-D 的任何路径**
- 不新增 `DenyCode` 枚举(`ErrorResponse.error.code` 的 `oneOf` 机制已足以承载 field-level / scope-level 拒绝)
- 不把 `POST /v1/me/change-password` 改成 `PATCH /v1/me/password`(保留 IA §7.2 既定端点命名)

---

## 4. 验证

```bash
wsl bash -lc "cd /mnt/c/Users/wsfwk/Downloads/yongboWorkflow/go && \
  /home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml"
# exit=0
# openapi validate: 0 error 0 warning
```

结论:**SA-B 14 条路径 schema 与 `V1_INFORMATION_ARCHITECTURE.md v1.1` §5.2 / §5.3 / §5.4 / §7.2 完全对齐(在 Q2=B2 + X1 的前向兼容占位模式下)**。

---

## 5. R4-SA-B 必读口径(交接给下一轮 prompt 用)

> 以下条目需原文写入 `prompts/V1_R4_FEATURES_SA_B.md` §必读输入 或 §实现约束,避免 Codex 实装时误解占位字段的 DDL 状态:

1. **`users.avatar_url` 列不存在**。`UpdateMyProfileRequest.avatar` 与 `PATCH /v1/users/{id}` 的 `avatar` 在 R4-SA-B 实现中需显式丢弃(JSON 解码后不写任何字段),但不返回 `400`;以保持"schema 合法 + 服务端 no-op"的向前兼容契约。
2. **`users.team_codes` / `users.primary_team_code` 列不存在**。`PATCH /v1/users/{id}` 的 `team_codes[]` / `primary_team_code` 同样走 no-op。v1 单值 `team` 字段继续是唯一持久化口径;多组模型延后到 R5+ 独立迁移轮。
3. **`MyOrgProfile.managed_departments` / `managed_teams` 直接复用 `user.ManagedDepartments` / `user.ManagedTeams` 现有聚合**(`domain.User.ManagedDepartments` / `ManagedTeams` 已就位),不引入新表。
4. **`POST /v1/departments/{id}/org-move-requests` 返回 `201`**(不是 200)。实装时 handler 需显式 `w.WriteHeader(201)` 并保证 body `data` 为创建的 `OrgMoveRequest`。
5. **`/v1/org-move-requests/{id}/approve` 成功时必须写审计事件** `user_department_changed_by_admin`(IA §5.2 明文),source/target department + actor 在 payload 中。
6. **`/v1/org-move-requests/{id}/reject` 的 `reason` 字段 required**。服务端写入 `org_move_requests.reason` 列(R2 migration 065 已建表)。
7. **`PATCH /v1/users/{id}` 的 `roles[]` 字段级授权** 严格按 IA §5.4 执行:
   - DeptAdmin **不可授** `DeptAdmin` / `SuperAdmin` / `HRAdmin`
   - HRAdmin **不可授** `SuperAdmin`
   - TeamLead **不可** 改 `roles[]`(整条字段级 403)
   - 违反返回 `403` + `ErrorResponse.error.code = role_assignment_denied_by_scope`(code 字符串可由 SA-B 自行取名,但须加进 R3 `DenyCode` 枚举后续轮才统一)
8. **`POST /v1/users/{id}/activate` / `deactivate` 的 TeamLead 分支**:只能操作本组成员;否则 `403`。DeptAdmin 同理限本部门。HRAdmin / SuperAdmin 全局。
9. **`GET /v1/org-move-requests` 的 DeptAdmin 可见范围**:强制服务端过滤 `source_department_id IN caller.managed_departments`。query 中的 `source_department_id` 若与服务端强制范围冲突,按服务端优先。

---

## 6. SA-C / SA-D 待补清单(维持不变)

本轮**不动** SA-C / SA-D 的 OpenAPI。清单仍记录在 `docs/iterations/V1_R1_7_OPENAPI_SA_A_PATCH.md` §5,未来 R1.7-C / R1.7-D 轮各开一次小补丁:

- **R1.7-C 待补**(3 block + 1 minor):`/v1/design-sources/search` query / `/v1/me/task-drafts` query / `/v1/me/notifications` query / `POST /v1/task-drafts` 显式 `draft_id`
- **R1.7-D 待补**(2 block):`/v1/search` 加 `limit` query / `SearchResultGroup` 固化子字段(需裁决"低权限 `users[]` 返回空数组"的形式)

---

## 7. 对 R4-SA-B prompt 的影响

`prompts/V1_R4_FEATURES_SA_B.md` 本轮**尚未起草**,主对话下一步即起草。起草时:

- §1 必读输入追加:
  - `docs/iterations/V1_R1_7_B_OPENAPI_SA_B_PATCH.md`(本报告)
  - `docs/V1_INFORMATION_ARCHITECTURE.md` §5 §7.2
- §9 失败终止条件直接采用 R4-SA-A v2.1 的新模板(**"控制字段零写入"** 而非"行数 ±0")
- §实装约束内嵌本报告 §5 全部 9 条口径
- §controlled fields 清单需枚举 SA-B 写入的 `users` / `org_move_requests` 列(`users.status` / `users.team` / `users.department` / `users.roles` / `users.managed_*` / `org_move_requests.*`);SA-B 之外的表严格零写入

---

## 8. 签字矩阵

| 角色 | 签字 |
| --- | --- |
| 架构(本对话) | **已签**(2026-04-17) |
| 后端 | **已同步**(2026-04-17,OpenAPI 0/0 · 无 DDL 影响 · 无生产影响;占位字段 X1 服务端忽略) |
| 前端 | **已同步**(2026-04-17,`/v1/me` 走 `WorkflowUser` 向前兼容 · `MyOrgProfile` 为新增子类 · `Actor` 零影响) |
| 产品 | **已同步**(2026-04-17,字段新增均来自 IA §5.2 / §5.3 / §5.4 / §7.2 已签字文档;`avatar/team_codes/primary_team_code` placeholder 的"v1 名义化 · R5+ 实装"口径记录在 IA 不变的前提下由 prompt 执行约束承载) |

---

## 9. 变更记录

| 版本 | 日期 | 变更 |
| --- | --- | --- |
| v1.0 | 2026-04-17 | 初稿即签字。R4-SA-B 起草前置小补丁;架构问答收敛 Q1=A3 / Q2=B2 / Q3=C2 / X1=schema 保留服务端忽略;本轮实补 SA-B 16 处(5 新 schema + 1 `WorkflowUser.avatar` 扩展 + 11 path 编辑);`openapi-validate` 0/0;9 条 SA-B 实装口径收录 §5 供下一轮 prompt 消化;SA-C/SA-D 补丁清单不变 |
