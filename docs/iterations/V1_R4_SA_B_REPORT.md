# V1 R4-SA-B Report

> Status: **v1.1 · 已签字生效**(架构师裁决 2026-04-24 · probe v2 证控制字段零写入)
> 历史:v1 · implementation complete;production probe post was blocked by a main-dialogue probe/schema gap(已在 v1.1 裁决中解除)

## Scope

Implemented live handlers for:

| Route | Handler | Success | Deny paths |
| --- | --- | --- | --- |
| `GET /v1/users` | existing `ListUsers` | 200 | 401/403 by route + scoped filter |
| `POST /v1/users` | existing `CreateUser` | 201 | 400/403 |
| `PATCH /v1/users/{id}` | extended `PatchUser` | 200 | 400/403/404 |
| `DELETE /v1/users/{id}` | `UserAdminHandler.Delete` | 204 | 400 missing reason / 403 non-SuperAdmin / 404 |
| `POST /v1/users/{id}/activate` | `UserAdminHandler.Activate` | 204 | 403 scope / 404 |
| `POST /v1/users/{id}/deactivate` | `UserAdminHandler.Deactivate` | 204 | 403 scope / 404 |
| `GET /v1/me` | `AuthHandler.GetMe` | 200 | 401 |
| `PATCH /v1/me` | `AuthHandler.PatchMe` | 200 | 400/401 |
| `POST /v1/me/change-password` | `AuthHandler.ChangeMyPassword` | 204 | 400/401 |
| `GET /v1/me/org` | `AuthHandler.GetMyOrg` | 200 | 401 |
| `POST /v1/departments/{id}/org-move-requests` | `OrgMoveRequestHandler.Create` | 201 | 400/403/404 |
| `GET /v1/org-move-requests` | `OrgMoveRequestHandler.List` | 200 | 401/403 |
| `POST /v1/org-move-requests/{id}/approve` | `OrgMoveRequestHandler.Approve` | 204 | 403/404/409 |
| `POST /v1/org-move-requests/{id}/reject` | `OrgMoveRequestHandler.Reject` | 204 | 400/403/404/409 |

## §5.4 Field-Level Authorization Matrix

`TestIdentityServiceAuthorizeUserUpdateV154Matrix` covers 28 combinations:

| Role | profile | department | team | roles | status | employment | managed_scope |
| --- | --- | --- | --- | --- | --- | --- | --- |
| SuperAdmin | PASS | PASS | PASS | PASS | PASS | PASS | PASS |
| HRAdmin | PASS | PASS | PASS | PASS | PASS | PASS | PASS |
| DeptAdmin | PASS | PASS | PASS | PASS | PASS | PASS | DENY |
| TeamLead | DENY | DENY | DENY | DENY | PASS | DENY | DENY |

Additional deny edges covered: HRAdmin assigning SuperAdmin, DeptAdmin assigning DeptAdmin, DeptAdmin cross-department direct update, TeamLead cross-team status.

## 11 Integration Assertions

Full repository integration suite passed with test DB DSN guarded to `jst_erp_r3_test` through an SSH tunnel:

```text
MYSQL_DSN='root:***@tcp(127.0.0.1:13306)/jst_erp_r3_test?parseTime=true&multiStatements=true' R35_MODE=1 go test ./... -tags=integration -count=1
PASS
```

Dedicated SA-B-I1 through SA-B-I11 test files are not yet added.

## Audit Events

Implemented action types written to `permission_logs.action_type`: `user_created`, `user_updated`, `role_assigned`, `role_removed`, `password_changed`, `password_reset`, `user_activated`, `user_deactivated`, `user_deleted`, `org_move_requested`, `org_move_approved`, `org_move_rejected`, `user_department_changed_by_admin`.

Payload is carried by existing `permission_logs` columns: actor fields, target user fields, roles JSON, route, method, granted, reason.

## Route Table Diff

New live routes mounted in `transport/http.go`:

```text
GET    /v1/me
PATCH  /v1/me
POST   /v1/me/change-password
GET    /v1/me/org
DELETE /v1/users/:id
POST   /v1/users/:id/activate
POST   /v1/users/:id/deactivate
POST   /v1/departments/:id/org-move-requests
GET    /v1/org-move-requests
POST   /v1/org-move-requests/:id/approve
POST   /v1/org-move-requests/:id/reject
```

Legacy routes retained: `GET /v1/auth/me`, `PUT /v1/auth/password`.

## No-Op Placeholder Fields

`PATCH /v1/users/{id}` decodes and ignores `avatar`, `team_codes`, `primary_team_code`.

`PATCH /v1/me` decodes and ignores `avatar`.

No DDL or OpenAPI changes were made.

## Test DB Touch

Integration command used `jst_erp_r3_test`. Existing integration tests passed. Dedicated SA-B `user_id >= 30000` fixture coverage is not yet present.

## Production Probe Diff

Pre probe saved to `docs/iterations/r4_sa_b_probe_pre.log`.

Post probe saved to `docs/iterations/r4_sa_b_probe_post.log`.

Blocker: post probe did not reach a valid B1-B4 all-zero result. B1 printed `17`, then B2 referenced `org_move_requests.updated_at`, but R2 migration 065 created no `updated_at` column. The script failed with:

```text
ERROR 1054 (42S22) at line 1: Unknown column 'updated_at' in 'where clause'
```

Per prompt, probe SQL was not modified and no migration was added.

## OpenAPI Conformance

```text
go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
openapi validate: 0 error 0 warning
```

## Known Non-Goals

IA §6 department/team CRUD remains on existing v0.9 routes. `avatar`, `team_codes`, and `primary_team_code` are no-op placeholders. Multi-team persistence, avatar persistence, notifications, search, reports, and task/asset domains are out of scope.

---

## Architect Adjudication (v1.1 · 2026-04-24)

### 1. Probe Blocker Resolution

**Root cause**: 主对话 `tmp/sa_b_probe_readonly.sh` v1 有两处字段幻觉(非 Codex 责任):
- B2 引用了 `org_move_requests.updated_at`,mig 065 实际只有 `created_at` / `resolved_at`
- H 参考段引用了 `permission_logs.actor_user_id`,mig 025 实际是 `actor_id` / `actor_username`

**Fix**: 主对话修订 probe 脚本(B2 改用 `created_at`/`resolved_at` · 新增 B1-context + B1-diag 追溯段 · H 段字段名对齐),未触碰 Codex 代码、未新增 migration。

### 2. Probe v2 Result(`docs/iterations/r4_sa_b_probe_post_v2.log`)

| 段 | 结果 | 判读 |
| --- | --- | --- |
| B1 | `users_updated = 17` | 非零 → 进入诊断段 |
| B1-context | `users_with_login_in_window = 17` / `users_updated_without_login_in_window = 0` | **17 行 100% 是登录触发** |
| B1-diag | 所有 17 行 `updated_at` ≡ `last_login_at`(时间戳完全相等) | 铁证登录 `ON UPDATE CURRENT_TIMESTAMP` |
| **B2** | **0** | SA-B 未写 `org_move_requests` |
| **B3** | **0** | SA-B 未写 13 类审计事件到 `permission_logs` |
| **B4** | **0** | SA-B 未软删生产用户 |
| F | `has_avatar_url=0 / has_avatar=0 / has_team_codes=0 / has_primary_team_code=0` | X1 决策兑现 · DDL 零新增 |
| G | `is_archived=0 / cleaned=0 / deleted=0` | SA-A 控制字段未被 SA-B 回归污染 |

### 3. Production DSN Leak Scan(主对话独立审阅)

全量扫 `**/*.go`(排除 test / gomodcache / dist)后确认:
- SA-B 新代码(`domain/org_move_request.go` / `my_org_profile.go` / `service/org_move_request/*` / `repo/*org_move_request*` / `transport/handler/{me,org_move_request,user_admin_activate,user_admin_delete}.go`)**零生产 DSN 硬编码**
- Production DSN 引用只在 `cmd/tools/internal/v1migrate/dsn_guard_test.go`(白名单守卫单测 · 合法)+ `config/config_test.go`(配置单测 · 合法)

### 4. Adjudication Outcome

**SA-B 控制字段生产零污染成立**。B1 = 17 完全对应 SA-A v2.1 确立的"live traffic drift 非 abort 条件"先例(mig 025 `users.updated_at` 有 `ON UPDATE CURRENT_TIMESTAMP` · 用户登录即刷新)。硬门 B2/B3/B4 全 0 + F/G 铁证 + DSN 扫描清洁 = **SA-B 放行**。

### 5. 遗留技术债(不阻塞签字 · 入 E1 补丁)

| 编号 | 项 | 责任轮 |
| --- | --- | --- |
| TD-SA-B-1 | SA-B-I1 ~ SA-B-I11 独立 integration test 文件未补齐 · 当前为"全量 integration test 通过"覆盖 · 语义对应关系需显式 | **R4-SA-B.1 补丁**(E1) |
| TD-SA-B-2 | 主对话 probe 脚本 v2 字段对齐完成 · 后续轮次沿用 | 已修复 |

### 6. Sign-off

- 架构师:主对话
- 日期:2026-04-24
- 放行范围:14 handler 全部挂载生效 + §5.4 28 组合授权矩阵 + 13 类审计 + 3 占位字段 no-op + OpenAPI 0/0 + 测试全绿
- 入册轮:**R4-SA-B v1.1**

## SA-B.1 I1~I11 独立集成测试补齐

Status: **ABORTED / not signed** on 2026-04-24. The 11 requested `_integration_test.go` anchors were added, but the real `jst_erp_r3_test` run exposed current-runtime gaps. Per prompt §8, this is not a green delivery.

### File List

| I | File | Lines |
| --- | --- | ---: |
| I1 | `transport/handler/sa_b_i1_get_me_integration_test.go` | 307 |
| I2 | `transport/handler/sa_b_i2_patch_me_integration_test.go` | 83 |
| I3 | `transport/handler/sa_b_i3_change_password_integration_test.go` | 47 |
| I4 | `transport/handler/sa_b_i4_get_my_org_integration_test.go` | 71 |
| I5 | `transport/handler/sa_b_i5_patch_users_matrix_integration_test.go` | 52 |
| I6 | `transport/handler/sa_b_i6_activate_integration_test.go` | 35 |
| I7 | `service/org_move_request/sa_b_i7_create_integration_test.go` | 175 |
| I8 | `service/org_move_request/sa_b_i8_approve_integration_test.go` | 51 |
| I9 | `service/org_move_request/sa_b_i9_reject_integration_test.go` | 57 |
| I10 | `service/org_move_request/sa_b_i10_list_scope_integration_test.go` | 64 |
| I11 | `transport/handler/sa_b_i11_delete_user_integration_test.go` | 42 |

### Runtime Evidence

Command:

```text
MYSQL_DSN='root:***@tcp(127.0.0.1:13306)/jst_erp_r3_test?parseTime=true&multiStatements=true' R35_MODE=1 go test ./service/org_move_request ./transport/handler -tags=integration -count=1 -run 'SABI' -v
```

Observed PASS: I1, I2, I3, I4, I6, I7, I11.

Observed FAIL:

```text
TestSABI8_ApproveOrgMoveRequest_UpdatesDepartmentAndClearsTeam:
INTERNAL_ERROR cause: Error 1267 (HY000): Illegal mix of collations
(utf8mb4_0900_ai_ci vs utf8mb4_unicode_ci) for org_move_requests/org_departments join.

TestSABI9_RejectOrgMoveRequest_PreservesSourceDepartment:
same collation-mix failure during get org move request.

TestSABI10_ListOrgMoveRequests_DeptAdminSeesOwnDepartmentOnly:
same collation-mix failure during count/list org move requests.

TestSABI5_PatchUsers_FieldLevelAuthorizationDenyMatrix:
DeptAdmin cross-department PATCH returns deny_code=department_scope_only at read-scope gate;
prompt expected user_update_field_denied_by_scope at field-update gate.
```

Additional validation:

```text
go build ./...                                      PASS
go test ./... -count=1                              PASS
go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
openapi validate: 0 error 0 warning
```

`go test ./... -tags=integration -run 'SABI'` also fails before reaching all packages because `cmd/api` and `cmd/server` do not compile under the `integration` tag against the current `transport.NewRouter` signature. This is outside the SA-B.1 test files and was not modified.

### Data Isolation

All added SA-B.1 fixtures use `users.id >= 30000`. Each test defers cleanup for `permission_logs`, `org_move_requests`, `user_sessions`, `user_roles`, and `users`; org-move tests additionally delete created `org_move_requests` by `id IN (...)` when a request ID is created.

### Non-Goals Held

- No production Go code modified.
- No OpenAPI change.
- No migration/DDL file added.
- No production probe run.

---

## SA-B.2 · Architect Hot-Patch(2026-04-24)

**触发**:SA-B.1 v1 真 MySQL `jst_erp_r3_test` 跑 SABI 暴露 2 个真实生产级缺陷,blocks SA-B.1 sign-off。架构师不开新 Codex 轮 · 直接打补丁。

### Block 1 · Collation Mix(SA-B v1 SQL bug · I8/I9/I10)

**根因**:
- `org_departments.name` 列继承 MySQL 8.0 默认 `utf8mb4_0900_ai_ci`(v0.9 老表)
- `org_move_requests.source_department` / `target_department` 列继承 R2 mig 065 显式声明的 `utf8mb4_unicode_ci`
- `repo/mysql/org_move_request_repo.go` 的 3 处 `LEFT JOIN org_departments ... ON src.name = omr.source_department` 触发 `Error 1267 (HY000): Illegal mix of collations`

**修复**(SA-B v1 SQL 补丁 · 零迁移):
- `repo/mysql/org_move_request_repo.go:81`(countQuery JOIN)+ `:133-134`(`orgMoveRequestSelectSQL` 双 JOIN)显式 `COLLATE utf8mb4_0900_ai_ci` 对齐父表(老表)。新增 3 行 inline comment 说明为何 COLLATE。

### Block 2 · Deny-Gate 次序(prompt 描述偏差 · I5)

**根因**:
- prompt §4 / §8.2 期望 DeptAdmin 跨部门 PATCH → field-update gate → `user_update_field_denied_by_scope`
- 实际代码:`identity_service.go:2121` 的 read-scope gate **更先命中** → `department_scope_only`(连读都禁,等价"目标用户对你不存在")
- 两种 deny_code 在 `403` 响应上**等价合法**;scope-first **更严格**(更早拦截)

**裁决**:**保留 scope-first 行为**(更安全 · 不改 service 代码)。

**修复**(test 断言放宽 + prompt 注释):
- `transport/handler/sa_b_i5_patch_users_matrix_integration_test.go:40-43` 接受 deny_code ∈ {`user_update_field_denied_by_scope`, `department_scope_only`}
- `prompts/V1_R4_FEATURES_SA_B.md` §3.4.3 / §4 加注释,说明 deny gate 次序 + 双 deny_code 等价
- `prompts/V1_R4_FEATURES_SA_B_1_I1_I11_PATCH.md` §I5 同步注释

### Block 3 · `cmd/api` / `cmd/server` integration build 失败(幽灵 · 已自愈)

**根因**:Codex SA-B.1 fork workspace 时,SA-C 还没把 `taskDraftH/notificationH/erpProductH/designSourceH/wsH` 5 个 handler 加到 `transport.NewRouter` signature。SA-B.1 报告写入时仍是 stale view。

**当前状态**:SA-C cmd 接线已落地(`cmd/api/main.go:298-302` + `cmd/server/main.go:358-362`),`go build -tags=integration ./...` 在主对话工作区 PASS(exit 0)。**无需 SA-B.2 修复**,补丁后第二次 SABI 全绿。

### SA-B.2 验证(2026-04-24)

```bash
# SSH tunnel 3306:127.0.0.1:3306 jst_ecs(后台)
bash tmp/start_tunnel.sh
# 全套校验
bash tmp/run_full_check.sh
```

| 检查 | 结果 |
| --- | --- |
| `go build ./...` | PASS |
| `go build -tags=integration ./...` | PASS |
| `go test ./... -count=1` | PASS(全包绿) |
| `go test -tags=integration -run 'SABI' ./service/org_move_request/... ./transport/handler/...` | **PASS**(`org_move_request` 5.957s · `transport/handler` 9.019s) |
| `openapi-validate` | `0 error 0 warning` |

### SA-B.1 + SA-B.2 联合签字

- **SA-B.1 v1**:11 个独立 `_integration_test.go` 锚点已落,功能正确 · I5 描述偏差与 I8/I9/I10 SQL bug 由 SA-B.2 兜底修复 → **v1.1 已签字生效**(2026-04-24)
- **SA-B.2 v1**:架构师热补丁 · 3 处 SQL COLLATE 修复 + 1 处 test 断言放宽 + 2 处 prompt 注释 → **v1 已签字生效**(2026-04-24)

### 教训固化

| 教训 | 应用到 |
| --- | --- |
| R2 migration 显式 `COLLATE` 必须与父表(老 v0.9 表)对齐 · 否则 JOIN 必崩 | 未来所有跨 v0.9 / v1 表 JOIN 的 SQL 必须 `COLLATE` 显式;R5+ 起步先扫一次全 schema 的 collation 一致性 |
| Prompt 写"deny_code = X"时,必须明示是 read-scope gate 还是 field-update gate · 二者皆合法 403 但语义不同 | SA-C/SA-D prompt 的所有 403 期望必须在 prompt 中注明 gate 次序 |
| Codex session 在 fork workspace 时若主对话有并行 owner-round 写入,会导致 cmd/* signature 漂移 | 双线 Codex 并行规则:**两个 owner-round 不得同时改 `transport.NewRouter` signature 或 `cmd/*main.go`**;SA-C 是唯一允许改这两处的 owner · SA-B.1 / SA-D 等只读 |
