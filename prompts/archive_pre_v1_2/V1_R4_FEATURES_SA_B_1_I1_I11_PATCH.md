# R4-SA-B.1 · I1~I11 独立 Integration Test 补丁 prompt

> 本轮类型:**技术债补齐 · 无新代码语义**
> 基础:R4-SA-B v1.1 架构师已签字(`docs/iterations/V1_R4_SA_B_REPORT.md` §Architect Adjudication)
> 预估工作量:< 1 轮 · 只加 test 文件 · 主代码一行不改

---

## 1. 缘由与责任边界

SA-B v1 已交付 14 handler 实装 + §5.4 28 组合矩阵单测 + 全量 integration test 通过。裁决书 §5 记录了遗留技术债 TD-SA-B-1:

> SA-B-I1 ~ SA-B-I11 独立 integration test 文件未补齐,当前为"全量 integration test 通过"覆盖,语义对应关系需显式。

本轮只做一件事:**把 SA-B prompt §8.2 的 11 条断言按一断言一函数的口径落到独立的 `_integration_test.go` 文件**,让每条 I 号都可单独 `-run` 跑、可单独追责、可在未来 regression 时精确定位。

## 2. DON'T TOUCH(硬约束)

| 约束 | 含义 |
| --- | --- |
| HC-1 | **不动**任何 `service/` / `repo/` / `domain/` / `transport/handler/` / `transport/http.go` 生产代码;只新增 `_integration_test.go` 文件 |
| HC-2 | **不动** `docs/api/openapi.yaml`;R1.7-B 已冻结 |
| HC-3 | **不加** migration / DDL / seed SQL 到 `db/migrations/` |
| HC-4 | **不动** SA-A / R3 / R3.5 的测试辅助;复用 `testsupport/r35.MustOpenTestDB(t)` |
| HC-5 | 不连生产 `jst_erp`;测试走 `jst_erp_r3_test`(经 SSH tunnel 或 jst_ecs 本地) |
| HC-6 | SA-B 测试样本严格 `user_id >= 30000` 段;`org_move_requests.id` 在每个 test 的 defer 里 `DELETE WHERE id IN (...)` 清理 |
| HC-7 | 发现 prompt §8.2 某条 I 实际上**已有等价语义覆盖**在现存 test 里 → 在报告里列对照表并引用原文件行号,**不重复实现**;但独立文件仍必须建(可写成薄包装调用公共 helper) |

## 3. 交付物

### 3.1 文件布局(新增 11 个 integration test 文件 · 建议位置)

```
service/org_move_request/sa_b_i7_create_integration_test.go
service/org_move_request/sa_b_i8_approve_integration_test.go
service/org_move_request/sa_b_i9_reject_integration_test.go
service/org_move_request/sa_b_i10_list_scope_integration_test.go

transport/handler/sa_b_i1_get_me_integration_test.go
transport/handler/sa_b_i2_patch_me_integration_test.go
transport/handler/sa_b_i3_change_password_integration_test.go
transport/handler/sa_b_i4_get_my_org_integration_test.go
transport/handler/sa_b_i5_patch_users_matrix_integration_test.go
transport/handler/sa_b_i6_activate_integration_test.go
transport/handler/sa_b_i11_delete_user_integration_test.go
```

每个文件必须带 build tag:

```go
//go:build integration

package <对应包>
```

### 3.2 每个 Test 函数命名规范

```go
func TestSABI1_GetMe_ReturnsWorkflowUser_WithAvatarNull(t *testing.T)           { ... }
func TestSABI2_PatchMe_IgnoresPlaceholderFields(t *testing.T)                    { ... }
func TestSABI3_ChangePassword_ValidatesOldAndConfirm(t *testing.T)               { ... }
func TestSABI4_GetMyOrg_ReturnsManagedScopeByRole(t *testing.T)                  { ... }
func TestSABI5_PatchUsers_FieldLevelAuthorizationDenyMatrix(t *testing.T)        { ... }
func TestSABI6_ActivateUser_TeamLeadWithinGroupOnly(t *testing.T)                { ... }
func TestSABI7_CreateOrgMoveRequest_PendingSuperAdminConfirm(t *testing.T)       { ... }
func TestSABI8_ApproveOrgMoveRequest_UpdatesDepartmentAndClearsTeam(t *testing.T){ ... }
func TestSABI9_RejectOrgMoveRequest_PreservesSourceDepartment(t *testing.T)      { ... }
func TestSABI10_ListOrgMoveRequests_DeptAdminSeesOwnDepartmentOnly(t *testing.T) { ... }
func TestSABI11_DeleteUser_SoftDeleteBySuperAdminOnly(t *testing.T)              { ... }
```

### 3.3 每个 Test 的标准结构

```go
//go:build integration

package handler_test // 或 service/org_move_request/...

import (
    "testing"
    // ...
    "<module>/testsupport/r35"
)

func TestSABI1_GetMe_ReturnsWorkflowUser_WithAvatarNull(t *testing.T) {
    db := r35.MustOpenTestDB(t)             // 硬门:DSN 不以 _r3_test 结尾 → t.Fatalf
    defer func() {
        // 清理本 test 创建的 user_id >= 30000 样本
        db.Exec("DELETE FROM users WHERE id >= 30000 AND id < 40000 /* SA-B-I1 scope */")
    }()

    // Arrange: 插入一个 user_id >= 30000 的测试账号 + 对应 session/token
    // Act:    调用 handler / service
    // Assert: 字段对照 prompt §8.2 I1 的"期望"列,逐字段 require.Equal
    // 附:assert `avatar == nil` 显式证明 X1 占位
}
```

## 4. Prompt §8.2 I1~I11 逐条断言口径(照抄权威 · 不得改动语义)

### I1 — `GET /v1/me`
- 返回 `WorkflowUser`(当前登录用户完整 profile)
- `avatar` 字段返回 `null`(占位)
- 依据:IA §7.2 / R1.7-B §3

### I2 — `PATCH /v1/me`
- 修改 `display_name` / `mobile` / `email` 成功写入 DB(3 字段 before/after 都断言)
- body 里的 `avatar` / `team_codes` / `primary_team_code` 写入后 DB 的 `users` 行这三字段**保持原值**(no-op 证据 · 对 `users.avatar_url` 等列:检查列不存在 or 值未变)
- 依据:IA §7.2 / R1.7-B §5.1

### I3 — `POST /v1/me/change-password`
- `old_password` 错 → 400 error code `old_password_mismatch`
- `new_password != confirm` → 400 error code `password_confirmation_mismatch`
- 全对 → 204 + `password_hash` 更新(DB 前后值不同)
- 依据:IA §7.2

### I4 — `GET /v1/me/org`
- 返回 `MyOrgProfile`
- DeptAdmin 测试账号的 `managed_departments` 非空数组
- Member 测试账号的 `managed_departments` / `managed_teams` 均为空数组(`len == 0`)
- 依据:IA §7.2

### I5 — `PATCH /v1/users/{id}` 字段级授权
- DeptAdmin 给其他部门用户改任何字段 → 403 · deny_code ∈ {`user_update_field_denied_by_scope`, `department_scope_only`}(SA-B.2 实测:DeptAdmin 访问跨部门目标用户**先命中 read-scope gate**(`identity_service.go:2121` → `department_scope_only`),根本到不了 field-update gate;两种 code 在 403 上等价合法 · scope-first 更严格)
- HRAdmin 授 `SuperAdmin` role → 403 error code `role_assignment_denied_by_scope`
- TeamLead 改 `display_name` → 403 · deny_code ∈ {`user_update_field_denied_by_scope`, `team_scope_only`}(同上次序)
- 依据:IA §5.4 / R1.7-B §5.7 / SA-B.2 裁决(2026-04-24)

### I6 — `POST /v1/users/{id}/activate`
- TeamLead 对本组成员 → 204 · `users.status='active'`
- TeamLead 对他组成员 → 403
- 依据:IA §5.3 / R1.7-B §5.8

### I7 — `POST /v1/departments/{id}/org-move-requests`
- DeptAdmin 源部门发起 → 201 · `OrgMoveRequest.state='pending_super_admin_confirm'`
- 用户部门在 approve 前**仍为源部门**
- 事件 `org_move_requested` 已写入 `permission_logs.action_type`
- 依据:IA §5.2 / R1.7-B §5.4

### I8 — `POST /v1/org-move-requests/{id}/approve`
- SuperAdmin 成功 → 204
- `users.department` 更新到 target + `users.team` 清空
- 事件 `user_department_changed_by_admin` 已写入
- 重复调用 → 409 `org_move_request_already_decided`
- 依据:IA §5.2 / R1.7-B §5.5

### I9 — `POST /v1/org-move-requests/{id}/reject`
- SuperAdmin 成功 → 204 · `state='rejected'` + `reject_reason=...`
- 用户部门**保持源部门**
- 非 SuperAdmin → 403
- 缺 reason → 400
- 依据:IA §5.2 / R1.7-B §5.6

### I10 — `GET /v1/org-move-requests?state=pending_super_admin_confirm`
- DeptAdmin 只看到本管辖部门发起的请求
- 创建两条(一条本部门 / 一条他部门),返回**仅本部门一条**
- 依据:R1.7-B §5.9

### I11 — `DELETE /v1/users/{id}`
- SuperAdmin + reason → 204 · `users.status='deleted'`(软删)
- 非 SuperAdmin → 403
- 缺 reason → 400
- 依据:IA §5.3

## 5. 已有覆盖对照(允许薄包装)

若 Codex 在实装时发现某条 I 已经被现存 test 等价覆盖(例如 `TestIdentityServiceAuthorizeUserUpdateV154Matrix` 涵盖了 I5 的 denyMatrix 分支),**仍必须新建对应独立文件**,但可写成薄包装:

```go
//go:build integration

package handler_test

import "testing"

// SA-B-I5: field-level PATCH authorization deny matrix
// 语义覆盖:service/identity_service_authorize_user_update_v1_5_4_test.go
// 的 TestIdentityServiceAuthorizeUserUpdateV154Matrix(28 组合)
// 本文件为独立 integration 锚点,驱动真 MySQL 断言 HTTP 层响应码。
func TestSABI5_PatchUsers_FieldLevelAuthorizationDenyMatrix(t *testing.T) {
    // 3 个最关键的 HTTP 层 deny 场景:DeptAdmin 跨部门 / HRAdmin 授 SuperAdmin / TeamLead 改 display_name
    // 其余 25 组合已在单测层覆盖,见上文注释
    ...
}
```

对照表在报告里显式列出。

## 6. 验证

```bash
# WSL
/home/wsfwk/go/bin/go build ./...
/home/wsfwk/go/bin/go test ./... -count=1          # 单测应零变动

# Integration(经 SSH tunnel 或 jst_ecs)
DSN=$(ssh jst_ecs 'bash /root/ecommerce_ai/r3_5/build_test_dsn.sh')
MYSQL_DSN="$DSN" R35_MODE=1 /home/wsfwk/go/bin/go test ./... -tags=integration -count=1 -run 'SABI' -v

# 期望:11 个 TestSABI* 函数全部 PASS,可单独 -run 跑
for i in 1 2 3 4 5 6 7 8 9 10 11; do
  MYSQL_DSN="$DSN" R35_MODE=1 /home/wsfwk/go/bin/go test ./... -tags=integration -count=1 -run "TestSABI${i}_" -v
done

# OpenAPI
/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
# 期望:0 error 0 warning(SA-B.1 不改 OpenAPI)
```

## 7. 生产 Probe

**本轮不跑生产 probe**(无代码改动 · 无新 handler · 无新审计路径)。SA-B v1.1 的 probe 已经证控制字段零写入,SA-B.1 纯测试补齐不会新增对生产的风险面。

## 8. 中止条件(任一触发即 stop + 回主对话)

- 任何 Go 生产代码(non-test)被改 → abort
- `docs/api/openapi.yaml` 被改 → abort
- `db/migrations/*.sql` 被新增 → abort
- 11 个 TestSABI* 不能全绿 → abort(不得 `t.Skip`;若发现 SA-B v1 实际语义与 prompt §8.2 有 gap,立即回主对话裁决)
- `-run 'SABI1_'` 无法定位到唯一测试函数 → abort(命名规范违反)
- 任何 test 使用非 `_r3_test` 结尾 DSN → abort(DSN 守卫应该挡住,若绕过即红线)
- 任何 `user_id < 30000` 的 user 被本轮测试修改 → abort(测试数据段越界)

## 9. 交付报告

追加到 `docs/iterations/V1_R4_SA_B_REPORT.md` 末尾,**新章节** `## R4-SA-B.1 · I1~I11 Independent Integration Test Patch`:

必含:

1. **文件清单表**:11 个新文件 + 行数 + 对应 I 编号
2. **语义对照表**:每条 I 的独立实现 / 或 "thin wrapper → 原 test 文件行号"
3. **Run 输出证据**:`-run 'SABI'` 的完整 PASS 输出(缩合后),以及 `-run 'TestSABI1_'` ~ `-run 'TestSABI11_'` 11 次单独 run 全 PASS 的尾行
4. **Data Isolation Evidence**:每个 test 的 `defer cleanup` SQL + 本轮运行前后 `SELECT MAX(id) FROM users` 无变化(`< 30000` 段未被改)
5. **Non-Goals**:未改生产代码 / 未改 OpenAPI / 未加 migration / 未跑生产 probe

## 10. 签字模板(Codex 回报格式)

```
已完成 R4-SA-B.1 · I1~I11 独立 integration test 补丁:
- 新增 11 个 _integration_test.go 文件(位置:...)
- 生产代码 0 改动 · OpenAPI 0 改动 · Migration 0 新增
- TestSABI1 ~ TestSABI11 全部 PASS(单独 -run 验证 11/11)
- 数据隔离:本轮所有 fixture `user_id ∈ [30000, 40000)`;org_move_requests defer 清理已执行
- 报告已追加到 docs/iterations/V1_R4_SA_B_REPORT.md 的 `## R4-SA-B.1` 章节

全量验证:
- go build ./...                              : PASS
- go test ./... -count=1                       : PASS
- go test ./... -tags=integration -count=1 -run 'SABI' : 11/11 PASS
- openapi-validate                             : 0 error 0 warning
```

---

## 修订记录

| 版本 | 日期 | 变更 |
| --- | --- | --- |
| v1 | 2026-04-24 | 初稿 · 基于 SA-B v1.1 架构师裁决的 TD-SA-B-1;纯测试补齐 · 生产代码零改动;11 条独立 TestSABI* 函数 + 薄包装允许 + 用 `user_id >= 30000` 段隔离 · 不跑生产 probe |
