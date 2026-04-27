# R4-SA-C.1 · SA-C-I1~I11 独立 Integration Test 补丁 prompt

> 本轮类型:**技术债补齐 · 无新代码语义**
> 基础:R4-SA-C v1 主体功能已签字(`docs/iterations/V1_R4_SA_C_REPORT.md` §SA-C Architect Adjudication · 2026-04-24)
> 类比模板:R4-SA-B.1(`prompts/V1_R4_FEATURES_SA_B_1_I1_I11_PATCH.md`)
> 预估工作量:< 1 轮 · 只加 test 文件 · 主代码一行不改

---

## 1. 缘由与责任边界

SA-C v1 已交付 11 路由实装 + 通知生成器 + WS hub + design-sources fallback + production probe 4 硬门全 0。架构师裁决 §SA-C 记录了 1 项遗留技术债:

> SA-C-I1 ~ SA-C-I11 独立 integration test 文件未补齐;当前由 R3 子集 + SABI 全量覆盖,但未做"一断言一函数"的精确锚点。

本轮只做一件事:**把 `prompts/V1_R4_FEATURES_SA_C.md` §8.4 的 11 条断言按"一断言一函数"的口径落到独立的 `_integration_test.go` 文件**,让每条 I 号都可单独 `-run` 跑、可单独追责、可在未来 regression 时精确定位。

## 2. DON'T TOUCH(硬约束)

| 约束 | 含义 |
| --- | --- |
| HC-1 | **不动**任何 `service/` / `repo/` / `domain/` / `transport/handler/` / `transport/http.go` / `transport/ws/` 生产代码;只新增 `_integration_test.go` 文件 |
| HC-2 | **不动** `docs/api/openapi.yaml`;R1.7-C 已冻结 |
| HC-3 | **不加** migration / DDL / seed SQL 到 `db/migrations/`(`task_drafts` / `notifications` 已由 R2 mig 063/064 落地) |
| HC-4 | **不动** SA-A / SA-B / R3 / R3.5 的测试辅助;复用 `testsupport/r35.MustOpenTestDB(t)` |
| HC-5 | 不连生产 `jst_erp`;测试走 `jst_erp_r3_test`(经 SSH tunnel 或 jst_ecs 本地);DSN guard 强制 `*_r3_test` |
| HC-6 | SA-C 测试样本严格 `user_id >= 40000` 段(SA-B 占 30000+ · SA-A 占 20000+ · 不撞);`task_drafts.id` / `notifications.id` 在每个 test 的 defer 里 `DELETE WHERE owner_user_id IN (...)` / `DELETE WHERE user_id IN (...)` 清理 |
| HC-7 | 发现 prompt §8.4 某条 I 实际上**已有等价语义覆盖**在现存 test 里 → 在报告里列对照表并引用原文件行号,**不重复实现**;但独立文件仍必须建(可写成薄包装调用公共 helper) |
| HC-8 | **不动** `service/erp_bridge/*`;ERP by-code 测试应使用 mock / 跳过(jst_erp_r3_test 不联 8081);允许 build tag 跳过 ERP 上游测试,在报告中明示 |

## 3. 交付物

### 3.1 文件布局(新增 11 个 integration test 文件 · 建议位置)

```
service/task_draft/sa_c_i1_create_draft_integration_test.go
service/task_draft/sa_c_i2_list_my_drafts_integration_test.go
service/task_draft/sa_c_i3_get_draft_integration_test.go
service/task_draft/sa_c_i4_delete_draft_integration_test.go

service/notification/sa_c_i5_list_notifications_integration_test.go
service/notification/sa_c_i6_mark_read_integration_test.go
service/notification/sa_c_i7_read_all_integration_test.go
service/notification/sa_c_i8_unread_count_integration_test.go

service/erp_product/sa_c_i9_erp_by_code_integration_test.go
service/design_source/sa_c_i10_design_source_search_integration_test.go
transport/ws/sa_c_i11_websocket_handshake_integration_test.go
```

每个文件必须带 build tag:

```go
//go:build integration

package <对应包>
```

### 3.2 每个 Test 函数命名规范

```go
func TestSACI1_CreateTaskDraft_OwnerOnlyAccessible(t *testing.T)                     { ... }
func TestSACI2_ListMyTaskDrafts_ReturnsOwnerScopedRecords(t *testing.T)              { ... }
func TestSACI3_GetTaskDraft_NonOwnerReturns403DraftNotOwner(t *testing.T)            { ... }
func TestSACI4_DeleteTaskDraft_NonOwnerReturns403_Idempotent(t *testing.T)           { ... }
func TestSACI5_ListNotifications_ReturnsOnlyOwnUserNotifications(t *testing.T)       { ... }
func TestSACI6_MarkNotificationRead_NonOwnerReturns403(t *testing.T)                 { ... }
func TestSACI7_ReadAllNotifications_FlipsAllUnreadInScope(t *testing.T)              { ... }
func TestSACI8_UnreadCount_ZeroAfterReadAll(t *testing.T)                            { ... }
func TestSACI9_GetERPProductByCode_404OrUpstream502OrSuccess(t *testing.T)           { ... }
func TestSACI10_DesignSourceSearch_FallbackToTaskAssetsStub(t *testing.T)            { ... }
func TestSACI11_WebSocketHandshake_BearerAuthOrUnauthorized(t *testing.T)            { ... }
```

### 3.3 每个 Test 的标准结构

```go
//go:build integration

package <对应包>_test

import (
    "testing"
    "time"
    // ...

    "workflow/testsupport/r35"
)

func TestSACI1_CreateTaskDraft_OwnerOnlyAccessible(t *testing.T) {
    db := r35.MustOpenTestDB(t)              // DSN guard 强制 *_r3_test
    ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
    defer cancel()

    // 1) 准备 owner user_id=40001(SA-C 段)
    fixtureUserID := int64(40001)
    t.Cleanup(func() {
        _, _ = db.ExecContext(ctx, `DELETE FROM task_drafts WHERE owner_user_id = ?`, fixtureUserID)
        _, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, fixtureUserID)
    })

    // 2) seed user / task type / payload
    // 3) call POST /v1/task-drafts via in-process handler
    // 4) assert 201 + draft_id 回包含 + DB 行存在 + owner_user_id 正确
    // 5) 第二个 user_id=40002 调 GET /v1/task-drafts/{draft_id} → 403 draft_not_owner
}
```

**关键 helper**(可在 `service/task_draft/sa_c_test_helper_test.go` 等共享):
- `seedUser(t, db, id, dept, team, roles, status)` — 创建临时用户
- `seedTaskDraftFor(t, db, ownerID, taskType, payload)` — 直接 insert task_drafts(避免依赖 handler 串联)
- `callHandler(t, method, path, actor, body)` — 调用 handler;复用 SA-B 的 stub middleware
- `cleanupSAC(t, db, userIDs)` — 兜底清理 task_drafts + notifications + users

### 3.4 11 条断言要点(对照 `prompts/V1_R4_FEATURES_SA_C.md` §8.4)

#### I1 — `POST /v1/task-drafts`
- owner=40001 创建 → `201` · 返回 `draft_id` + `expires_at = now()+7d ± 1h`
- 同 owner 创建第 21 条 → `400 task_draft_quota_exceeded`(IA-A14 上限 20)
- payload 任意 JSON body 接受(IA §3.5.9)

#### I2 — `GET /v1/me/task-drafts`
- owner=40001 看到自己 N 条;owner=40002 看到 0 条
- cursor `?cursor=BASE64(updated_at_unix:id)` 翻页(R1.7-C §C.2)

#### I3 — `GET /v1/task-drafts/{draft_id}`
- owner 直读 `200`;非 owner `403 draft_not_owner`(注意:非 `draft_not_found` · 防探测)

#### I4 — `DELETE /v1/task-drafts/{draft_id}`
- owner 删 `204`,DB 行消失
- 非 owner 删 `403 draft_not_owner`
- 已删除再删 `204` 幂等(或 `404 task_draft_not_found` · 二选一 · 报告里固定一种)

#### I5 — `GET /v1/me/notifications`
- 接 `?is_read=false` / `?notification_type=task_assigned_to_me` 过滤
- 仅返回 `user_id = actor.ID` 的 row(scope 隔离)
- cursor R1.7-C §B2'

#### I6 — `POST /v1/me/notifications/{id}/read`
- owner 标记 `204` · `is_read=1` + `read_at` 写入
- 非 owner `403 notification_not_owner`(防探测;不返 404)

#### I7 — `POST /v1/me/notifications/read-all`
- 把当前 actor 名下 `is_read=0` 全部翻为 `1` · `204`
- 不影响其他 user 的 unread

#### I8 — `GET /v1/me/notifications/unread-count`
- read-all 之前 == N · read-all 之后 == 0
- response shape `{count: int}`

#### I9 — `GET /v1/erp/products/by-code`
- 上游成功 → `200` + `ERPProductSnapshot`(snapshot 的最小字段子集即可)
- 上游 404 → `404 erp_product_not_found`
- 上游 timeout/5xx → `502 erp_upstream_failure`
- **本测试可标记为 `t.Skip("ERP upstream not available in r3_test")`** · 但断言 handler 在 mock client 注入 error 时正确分流;允许使用 `service.ERPBridgeService` mock

#### I10 — `GET /v1/design-sources/search`
- `keyword=` 命中 `task_assets WHERE source_module_key='design'` fallback(`design_sources` 不存在)
- `page=1&size=20` 翻页 · 不超过 size=100
- 返回 `DesignSourceEntry` 数组

#### I11 — `GET /ws/v1`
- 缺 Bearer → `401 UNAUTHORIZED`
- 有效 Bearer → 升级 101;客户端 send/recv 一帧 ping 不崩
- 服务端 broadcast `task_pool_count_changed` → 客户端能 recv(可用 `time.AfterFunc(50ms)` 触发 hub broadcast)

### 3.5 测试数据隔离

- 用户 id 段:`40000~49999`(SA-C 专用)
- task_drafts 隔离:`owner_user_id IN (40001..40010)`
- notifications 隔离:`user_id IN (40001..40010)`
- t.Cleanup 必须按依赖顺序逆序删:`task_drafts` → `notifications` → `permission_logs WHERE actor_id IN (...)` → `users`

## 4. 验证清单(本轮 Codex 必须 STRICTLY 跑完才能签字)

```bash
# SSH tunnel 必须先在 WSL 中启动
bash tmp/start_tunnel.sh

cd /mnt/c/Users/wsfwk/Downloads/yongboWorkflow/go
DSN="$(ssh jst_ecs 'bash /root/ecommerce_ai/r3_5/build_test_dsn.sh')"

# A) build 全绿
/home/wsfwk/go/bin/go build -tags=integration ./...   # → exit 0

# B) 单条 I 跑(11 次)
for i in 1 2 3 4 5 6 7 8 9 10 11; do
  MYSQL_DSN="$DSN" R35_MODE=1 /home/wsfwk/go/bin/go test \
    -tags=integration -count=1 -run "TestSACI${i}_" \
    ./service/task_draft/... \
    ./service/notification/... \
    ./service/erp_product/... \
    ./service/design_source/... \
    ./transport/ws/...
done  # → 11/11 PASS

# C) 全量 -run SACI 跑(批量)
MYSQL_DSN="$DSN" R35_MODE=1 /home/wsfwk/go/bin/go test \
  -tags=integration -count=1 -run 'SACI' \
  ./service/task_draft/... ./service/notification/... \
  ./service/erp_product/... ./service/design_source/... \
  ./transport/ws/...      # → ok

# D) 不破坏 SA-A / SA-B / R3 全量
MYSQL_DSN="$DSN" R35_MODE=1 /home/wsfwk/go/bin/go test \
  -tags=integration -count=1 -run 'SAAI|SABI|TestModuleAction|TestTaskPool|TestTaskCancel|TestTaskAggregator' \
  ./service/...   ./transport/handler/...   # → ok

# E) unit
/home/wsfwk/go/bin/go test ./... -count=1   # → ok

# F) openapi
/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
# → 0 error 0 warning
```

## 5. 报告输出 → `docs/iterations/V1_R4_SA_C_REPORT.md` 末尾追加 §SA-C.1

报告必须包含(类比 SA-B.1 报告的 `## SA-C.1 · ...`):

1. **§SA-C.1 §1 验证命令实跑日志**(A~F 6 步全部 PASS 的输出快照,前后 5 行即可)
2. **§SA-C.1 §2 11 条 I 对照表**:`I号 → 测试函数名 → 文件 → 复用现有 helper 的引用行号(若有)`
3. **§SA-C.1 §3 数据隔离审计**:`SELECT MAX(id), MIN(id) FROM users WHERE id BETWEEN 40000 AND 49999;` 跑完后必须为 `NULL/NULL`(全部清干净)
4. **§SA-C.1 §4 已知不达标**:I9 ERP 上游若不可达,标记 `Skipped: ERP upstream not in r3_test scope`(架构师接受)
5. **§SA-C.1 §5 sign-off candidate**:声明 SA-C 整体可签字(SA-C v1 + SA-C.1 联合)

## 6. ABORT 条件

- 任何 production 代码(`service/` / `repo/` / `domain/` / `transport/handler/` / `transport/http.go` / `transport/ws/`)被改 → abort
- 任何 SA-A / SA-B / R3 已存在的 test 被修改 → abort
- 任何新增 migration / DDL → abort
- DSN guard 失效 / 测试连到 `jst_erp` → abort
- 验证清单 §4 任一步 fail → abort(报告必须列原始错误日志)

## 7. 不在本轮范围

- multi-instance WS Pub/Sub(R7+)
- `task_timeout` / `task_mentioned` 通知类型(R7+)
- WS message persistence(R7+)
- 报告 L0/L1 仪表板(SA-D)
- 全局搜索(SA-D)
- ERP 上游真实联调(超出 r3_test 环境)
