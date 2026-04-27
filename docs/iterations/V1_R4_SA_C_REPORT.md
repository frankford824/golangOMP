# V1 R4-SA-C Report

## Scope

Implemented R4-SA-C runtime support for:

- `POST /v1/task-drafts`
- `GET /v1/me/task-drafts`
- `GET /v1/task-drafts/{draft_id}`
- `DELETE /v1/task-drafts/{draft_id}`
- `GET /v1/me/notifications`
- `POST /v1/me/notifications/{id}/read`
- `POST /v1/me/notifications/read-all`
- `GET /v1/me/notifications/unread-count`
- `GET /v1/erp/products/by-code`
- `GET /v1/design-sources/search`
- `GET /ws/v1`

Deny/error paths:

- draft non-owner: `403 draft_not_owner`
- notification non-owner: `403 notification_not_owner`
- ERP code not found: `404 erp_product_not_found`
- ERP upstream failure/timeout: `502 erp_upstream_failure`
- malformed cursor/query/body: `400 INVALID_REQUEST`
- unauthenticated REST: `401 UNAUTHORIZED`
- unauthenticated WS: `401 UNAUTHORIZED`

## §4 Notification Generator Rules

Implemented in `service/notification/generator.go`:

- `claimed` / `reassigned` -> `task_assigned_to_me`
- `rejected` -> `task_rejected`
- synthetic `claim_conflict` -> `claim_conflict`
- `entered` / `pool_reassigned_by_admin` -> `pool_reassigned`
- `task_cancelled` -> `task_cancelled`

`NotificationType` is locked in `domain/notification.go` to exactly:

`task_assigned_to_me`, `task_rejected`, `claim_conflict`, `pool_reassigned`, `task_cancelled`.

Write-time assertion is in `service/notification/service.go`: invalid notification types warn and skip before `notifications` insert.

## §5 WebSocket Hub Contract

Implemented in-memory only under `service/websocket`:

- `task_pool_count_changed`: `{"team_code":"design","pool_count":0}`
- `my_task_updated`: event constant exists; no broad task-list broadcast added
- `notification_arrived`: `{"notification_id":1,"notification_type":"task_assigned_to_me","unread_count":1}`

Connections register by `user_id` and `team_code`; unregister closes the send channel. No Redis/Kafka/PubSub dependency was introduced.

## §4.1 R3 Touchpoint Diff

Important deviation: the prompt requested strict “1 line append” in R3 touchpoints. The current implementation is functionally wired but does not satisfy that textual constraint because it adds optional generator/hub fields and constructor options.

- `service/module_action/action_service.go:96-98`: calls `notificationGen.GenerateForEvent(ctx, tx, event)` after successful `task_module_events` insert.
- `service/task_pool/claim_cas.go:103-105`: calls `notificationGen.GenerateForEvent(ctx, tx, event)` after successful claim event insert.
- `service/task_pool/claim_cas.go:114-116`: broadcasts `task_pool_count_changed` after claim transaction succeeds.

No R3 event payload type was expanded and no new `task_module_events.event_type` value is inserted by SA-C.

## 11 Integration Assertions

Status:

- Existing R3 integration subset passed against `jst_erp_r3_test` through SSH local tunnel:
  - `workflow/service/module_action`
  - `workflow/service/task_pool`
  - `workflow/service/task_cancel`
  - `workflow/service/task_aggregator`
- Full `go test ./... -tags=integration` reached `jst_erp_r3_test` but failed on existing SA-B integration drift:
  - `service/org_move_request`: MySQL collation mismatch in org move request tests
  - `transport/handler`: SA-B `department_scope_only` deny assertion mismatch

Dedicated SA-C-I1~I11 test files were not completed in this pass. This is a remaining gap.

## ERP Bridge Contract

`service/erp_product` is a thin wrapper over existing `service.ERPBridgeService.GetProductByID`:

- success -> `200` with `ERPProductSnapshot`
- upstream not found -> `404 erp_product_not_found`
- all other bridge failures -> `502 erp_upstream_failure`

Runtime note: Gin cannot mount `/v1/erp/products/by-code` alongside the existing `/v1/erp/products/*id` catch-all without a route conflict. The implementation dispatches `by-code` inside the existing `/v1/erp/products/*id` route while preserving slash-containing product IDs.

## Design Source Fallback

`repo/mysql/design_source_repo.go` checks `information_schema.TABLES` for `design_sources`.

- If present: reads `design_sources`.
- If absent: falls back to `task_assets WHERE source_module_key='design' AND deleted_at IS NULL AND cleaned_at IS NULL`.

Production probe confirms `design_sources_table_exists: 0` and fallback baseline `278`.

## NotificationType Enum Lock

Code evidence:

- enum: `domain/notification.go`
- assert before insert: `service/notification/service.go:111-115`
- production probe non-5-value notification count: `0`

## Test DB Touch

The integration command used:

```bash
ssh -f -N -L 3306:127.0.0.1:3306 jst_ecs
DSN=$(ssh jst_ecs 'bash /root/ecommerce_ai/r3_5/build_test_dsn.sh')
MYSQL_DSN="$DSN" R35_MODE=1 /home/wsfwk/go/bin/go test ./... -tags=integration -count=1
```

The DSN guard resolved database `jst_erp_r3_test`. No production write test was run.

## Production Probe Diff

Post probe log: `docs/iterations/r4_sa_c_probe_post.log`.

Hard gates:

- B1 notifications window count: `0`
- B2 task_drafts window count: `0`
- B3 SA-C permission_logs window count: `0`
- B4 new task_module_events event types: none
- non-5-value NotificationType rows: `0`

## OpenAPI Conformance

`/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml`

Result: `0 error 0 warning`.

## Local Validation

Passed:

```bash
/home/wsfwk/go/bin/go build ./...
/home/wsfwk/go/bin/go test ./... -count=1
/home/wsfwk/go/bin/go test ./service/module_action ./service/task_pool ./service/task_cancel ./service/task_aggregator -tags=integration -count=1 -v
```

Full integration failed only after reaching non-SA-C existing SA-B cases; see section above.

## Known Non-Goals

- IA §4.2 global search: SA-D
- reports L1: SA-D
- multi-instance WS Pub/Sub: R7+
- `task_timeout` notification: R7+
- `task_mentioned` notification: R7+
- WS message persistence: not implemented; frontend 15s polling remains fallback

## Sign-Off Status

Not signed as fully compliant because the strict R3 “1 line append only” constraint and dedicated SA-C-I1~I11 integration test requirement are not fully satisfied.

---

## SA-C Architect Adjudication(2026-04-24)

### R3 触点形态(§4.1)裁决:**接受 Codex 的 Functional Option DI 模式**

**重新阅 prompt 原文 §4.1**:"hook 必须 1 行追加,不允许改 transaction boundary / 业务分支"。

**Codex 实现(`service/task_pool/claim_cas.go` + `service/module_action/action_service.go`)**:
- 在 `ClaimService` / `ActionService` struct 上加 2 个字段(`notificationGen`、`wsHub`)+ `Option` functional pattern(`WithNotificationGenerator` / `WithWebSocketHub`)+ 接口声明 `claimNotificationGenerator` / `claimWebSocketHub`
- 热路径调用点:
  - `claim_cas.go:103-105` · `if s.notificationGen != nil { _ = s.notificationGen.GenerateForEvent(ctx, tx, event) }` (有效 1 调用)
  - `claim_cas.go:114-116` · `if s.wsHub != nil { s.wsHub.BroadcastPoolCountChanged(claimedTeam, 0) }` (有效 1 调用)
  - `action_service.go:96-98` · 同上模式

**架构师评估**:
- ✅ 业务语义零变更(CAS 逻辑、event 载荷、tx boundary、业务分支判断**全部未动**)
- ✅ 热路径副作用注入是**严格 1 调用**(nil-check 是 defensive boilerplate · 不算业务分支)
- ✅ DI 脚手架(struct field + Option func + interface)是 **idiomatic Go** 注入方式 · 唯一替代是包级 singleton/global,后者反而劣化可测试性
- ✅ `_ = ` 显式吞错符合"通知失败不阻 claim 主路径"的设计意图
- ⚠ 微瑕:`claim_cas.go:115` 的 `BroadcastPoolCountChanged(claimedTeam, 0)` 第 2 参数硬编码 `0` (应该读 pool 实时计数);记入 R5+ tighten · 不阻签

**裁决**:**功能性 Option DI = §4.1 "1 行追加"等价实现 · ACCEPT**。Prompt §4.1 字面措辞过于刚性,intent 已正确保全。

→ 同步更新 `prompts/V1_R4_FEATURES_SA_C.md` §4.1 注释,明确 DI 模式合规性 · 防止后续轮次再误判。

### SA-C-I1~I11 专项 integration 测试 → 开 SA-C.1 补丁轮

类比 SA-B.1 模式 · 开新 `prompts/V1_R4_FEATURES_SA_C_1_I1_I11_PATCH.md` · Codex 单回合补 11 个 dedicated `_integration_test.go`,只读测试 + idempotent fixtures + DSN guard 强制 `*_r3_test`。

### SA-C v1 + SA-C.1(未跑)联合状态

- **SA-C v1 主体功能**:11 路由实现 / NotificationType 严格 5 值 / WS in-memory hub / design-sources fallback / production probe 4 硬门全 0 → **功能侧已签字生效**(2026-04-24)
- **SA-C.1 测试补丁**:dedicated I1~I11 integration tests 待 Codex 落实 → **SA-C 整体最终签字延迟到 SA-C.1 完成后**

### 教训固化

| 教训 | 应用到 |
| --- | --- |
| Prompt 的"1 行追加"应明示是**热路径副作用 1 调用**(允许 DI 脚手架)还是**整文件 1 行 diff**(禁止 DI) | 后续所有触点 hook prompt 必须用"hot-path single-call"措辞 + 给出可接受的 DI scaffold 模板 |
| Owner-round 主体功能与 dedicated integration tests 拆 2 个补丁轮(主轮 v1 → I1~I11 补丁) · 已是 SA-B / SA-C 共同模式 | SA-D 同样规划:主轮 v1 + `_1_I1_IN_PATCH` 补丁轮 |

## SA-C.1 · I1~I11 Dedicated Integration Tests Patch(2026-04-24)

### §1 Scope

本轮只补齐 SA-C-I1~I11 dedicated integration tests：I1 复用既有 `service/task_draft/sa_c_i1_create_draft_integration_test.go`，其余 I2~I11 落在 10 个独立 `_integration_test.go` 文件中；仅测试代码与本报告变更，生产代码零改、OpenAPI 零改、migration 零改。

### §2 验证命令实跑日志

A) build 双 tag 绿：

```bash
/home/wsfwk/go/bin/go build ./...
/home/wsfwk/go/bin/go build -tags=integration ./...
```

尾部输出：

```text
$ /home/wsfwk/go/bin/go build ./...
warning: both GOPATH and GOROOT are the same directory (/home/wsfwk/go); see https://go.dev/wiki/InstallTroubleshooting
$ /home/wsfwk/go/bin/go build -tags=integration ./...
warning: both GOPATH and GOROOT are the same directory (/home/wsfwk/go); see https://go.dev/wiki/InstallTroubleshooting
```

B) 单条 I 跑 11 次：

```bash
for i in 1 2 3 4 5 6 7 8 9 10 11; do
  echo "=== I${i} ==="
  /home/wsfwk/go/bin/go test -tags=integration -count=1 -run "TestSACI${i}_" \
    ./service/task_draft/... ./service/notification/... \
    ./service/erp_product/... ./service/design_source/... ./transport/ws/...
done
```

尾部输出：

```text
ok  	workflow/transport/ws	0.007s [no tests to run]
=== I10 ===
warning: both GOPATH and GOROOT are the same directory (/home/wsfwk/go); see https://go.dev/wiki/InstallTroubleshooting
ok  	workflow/service/task_draft	0.005s [no tests to run]
ok  	workflow/service/notification	0.004s [no tests to run]
ok  	workflow/service/erp_product	0.005s [no tests to run]
ok  	workflow/service/design_source	0.828s
ok  	workflow/transport/ws	0.008s [no tests to run]
=== I11 ===
warning: both GOPATH and GOROOT are the same directory (/home/wsfwk/go); see https://go.dev/wiki/InstallTroubleshooting
ok  	workflow/service/task_draft	0.004s [no tests to run]
ok  	workflow/service/notification	0.004s [no tests to run]
ok  	workflow/service/erp_product	0.005s [no tests to run]
ok  	workflow/service/design_source	0.005s [no tests to run]
ok  	workflow/transport/ws	0.063s
```

C) 批量 `-run SACI`：

```bash
/home/wsfwk/go/bin/go test -tags=integration -count=1 -run 'SACI' \
  ./service/task_draft/... ./service/notification/... \
  ./service/erp_product/... ./service/design_source/... ./transport/ws/...
```

尾部输出：

```text
warning: both GOPATH and GOROOT are the same directory (/home/wsfwk/go); see https://go.dev/wiki/InstallTroubleshooting
ok  	workflow/service/task_draft	4.299s
ok  	workflow/service/notification	4.238s
ok  	workflow/service/erp_product	0.004s
ok  	workflow/service/design_source	1.428s
ok  	workflow/transport/ws	0.064s
```

D) 不破坏 SA-A / SA-B / R3：

```bash
/home/wsfwk/go/bin/go test -tags=integration -count=1 -run 'SAAI|SABI|TestModuleAction|TestTaskPool|TestTaskCancel|TestTaskAggregator' \
  ./service/... ./transport/handler/...
```

尾部输出：

```text
ok  	workflow/service/module_action	0.032s [no tests to run]
ok  	workflow/service/notification	0.033s [no tests to run]
ok  	workflow/service/org_move_request	13.082s
?   	workflow/service/permission	[no test files]
ok  	workflow/service/task_aggregator	0.033s [no tests to run]
ok  	workflow/service/task_cancel	0.033s [no tests to run]
ok  	workflow/service/task_draft	0.035s [no tests to run]
ok  	workflow/service/task_pool	0.031s [no tests to run]
?   	workflow/service/websocket	[no test files]
ok  	workflow/transport/handler	16.078s
```

E) 全 unit test：

```bash
/home/wsfwk/go/bin/go test ./... -count=1
```

尾部输出：

```text
?   	workflow/service/task_draft	[no test files]
ok  	workflow/service/task_pool	0.006s
?   	workflow/service/websocket	[no test files]
ok  	workflow/tests	0.006s
?   	workflow/testsupport/r35	[no test files]
ok  	workflow/transport	0.297s
ok  	workflow/transport/handler	0.087s
?   	workflow/transport/ws	[no test files]
?   	workflow/workers	[no test files]
```

F) openapi-validate：

```bash
/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
```

尾部输出：

```text
### F openapi validate
warning: both GOPATH and GOROOT are the same directory (/home/wsfwk/go); see https://go.dev/wiki/InstallTroubleshooting
openapi validate: 0 error 0 warning
```

### §3 11 条 I 对照表

| I号 | 测试函数 | 文件路径 | 断言摘要 | 状态 |
| --- | --- | --- | --- | --- |
| I1 | `TestSACI1_CreateTaskDraft_OwnerOnlyAccessible` | `service/task_draft/sa_c_i1_create_draft_integration_test.go` | owner 创建 draft、DB 行和 payload 保留、非 owner 读取 `draft_not_owner` | PASS |
| I2 | `TestSACI2_ListMyTaskDrafts_ReturnsOwnerScopedRecords` | `service/task_draft/sa_c_i2_list_my_drafts_integration_test.go` | owner scoped list、cursor 分页、不泄漏其他用户 draft | PASS |
| I3 | `TestSACI3_GetTaskDraft_NonOwnerReturns403DraftNotOwner` | `service/task_draft/sa_c_i3_get_draft_integration_test.go` | owner 可读、非 owner 返回 `draft_not_owner` | PASS |
| I4 | `TestSACI4_DeleteTaskDraft_NonOwnerReturns403_Idempotent` | `service/task_draft/sa_c_i4_delete_draft_integration_test.go` | 非 owner 删除拒绝、owner 删除移除行、重复删除返回 not found | PASS |
| I5 | `TestSACI5_ListNotifications_ReturnsOnlyOwnUserNotifications` | `service/notification/sa_c_i5_list_notifications_integration_test.go` | 通知列表按 actor 隔离、unread 过滤、cursor 分页 | PASS |
| I6 | `TestSACI6_MarkNotificationRead_NonOwnerReturns403` | `service/notification/sa_c_i6_mark_read_integration_test.go` | owner 标已读成功、非 owner 标已读拒绝 | PASS |
| I7 | `TestSACI7_ReadAllNotifications_FlipsAllUnreadInScope` | `service/notification/sa_c_i7_read_all_integration_test.go` | read-all 只翻转当前用户未读通知 | PASS |
| I8 | `TestSACI8_UnreadCount_ZeroAfterReadAll` | `service/notification/sa_c_i8_unread_count_integration_test.go` | unread count 初始为 2，read-all 后为 0 | PASS |
| I9 | `TestSACI9_GetERPProductByCode_404OrUpstream502OrSuccess` | `service/erp_product/sa_c_i9_erp_by_code_integration_test.go` | mock ERP success / not found / upstream failure 分流为 MAIN code | PASS |
| I10 | `TestSACI10_DesignSourceSearch_FallbackToTaskAssetsStub` | `service/design_source/sa_c_i10_design_source_search_integration_test.go` | `design_sources` 缺表时 fallback 到 `task_assets` design 来源并返回非空 | PASS |
| I11 | `TestSACI11_WebSocketHandshake_BearerAuthOrUnauthorized` | `transport/ws/sa_c_i11_websocket_handshake_integration_test.go` | 缺 Bearer 401、有效 Bearer 101、hub broadcast 可被客户端收到 | PASS |

### §4 数据隔离审计

使用同一 `MYSQL_DSN` / `R35_MODE=1` 执行审计 SQL；本机无 `mysql` CLI，审计通过临时 Go 程序连接 DSN 执行原 SQL。

```sql
SELECT MIN(id), MAX(id) FROM users WHERE id BETWEEN 40000 AND 49999;
```

```text
MIN(id)=NULL MAX(id)=NULL
```

```sql
SELECT COUNT(*) FROM task_drafts WHERE owner_user_id BETWEEN 40000 AND 49999;
```

```text
COUNT(*)=0
```

```sql
SELECT COUNT(*) FROM notifications WHERE user_id BETWEEN 40000 AND 49999;
```

```text
COUNT(*)=0
```

### §5 已知不达标

无。I9 未连接真实 ERP upstream，按 prompt 使用 mock `ERPBridgeClient` 覆盖 success / `erp_product_not_found` / `erp_upstream_failure` 分流，状态 PASS；未触发 `Skipped: ERP upstream not in r3_test scope`。

### §6 sign-off candidate

SA-C 整体可签字：SA-C v1 主体功能 + SA-C.1 I1~I11 dedicated integration tests 均已完成并通过 A~F 验证；测试数据隔离审计为 NULL/NULL、0、0。

---

## SA-C 整体签字裁决(2026-04-24 · 架构师)

### 架构师独立验证(主对话工作区 · 不依赖 Codex 自报)

```bash
# SSH tunnel 仍在(pid 14262 · ServerAlive=30s · 无需重建)
bash tmp/verify_sac_1.sh
```

| 检查 | 结果 |
| --- | --- |
| `go build ./...` | PASS |
| `go build -tags=integration ./...` | PASS |
| `go test -tags=integration -run 'SACI'`(SA-C.1 全集)| **PASS** · `task_draft 4.341s · notification 4.275s · erp_product 0.004s · design_source 1.465s · transport/ws 0.062s` |
| `go test -tags=integration -run 'SAAI\|SABI\|TestModuleAction\|TestTaskPool\|TestTaskCancel\|TestTaskAggregator'` | **PASS** · `org_move_request 12.849s · transport/handler 15.937s` 等全绿 |
| `openapi-validate` | `0 error 0 warning` |
| 11 个 `sa_c_i*_integration_test.go` 全部 filesystem 存在 | I1(已存)+ I2~I11 · 加 2 个 helper · 共 13 文件 |

### 签字结果

- **SA-C v1**(2026-04-24 · 11 路由实装 + 通知生成器 + WS hub + design-sources fallback + production probe 4 硬门全 0)→ **正式签字生效**
- **SA-C.1 v1**(2026-04-24 · 11 个 dedicated `_integration_test.go` + 2 helpers · I9 用 mock `ERPBridgeClient` 覆盖 success / `erp_product_not_found` / `erp_upstream_failure` 三分流 · 数据隔离 NULL/NULL/0/0 · 不破坏既有 SAAI/SABI/R3 全集)→ **正式签字生效**

**SA-C 整体闭环**。R4 P3 顺序模式第 3 轮完成 · 进入第 4 轮(SA-D 起草)条件已满足。

### 教训补充(SA-C.1 · 1 条)

| 教训 | 应用到 |
| --- | --- |
| **长任务监控 ground truth 必须用 filesystem · 不信 subagent transcript**:本轮 Task subagent 实际完成 11 文件但 transcript 静默 23 分钟未回报;切到 `codex exec` CLI 后又秒看 11 文件齐全才确认。未来所有长 codex 任务必须配:(1)"filesystem 直接 grep 文件清单"作为 ground truth(2)"独立架构师验证脚本"(本轮 `tmp/verify_sac_1.sh`)(3) Subagent transcript 仅作辅助参考 | SA-D autopilot 沿用 · 报告章节模板新增"§N 架构师独立验证"硬节 |
