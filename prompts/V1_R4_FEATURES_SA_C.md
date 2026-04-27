# V1 · R4-SA-C · 通知中心 + WebSocket + ERP by-code + 设计源文件搜索 + 草稿端点

> 版本:**v1.1**(2026-04-24 · 架构师裁决补丁:§4.1 R3 触点 DI scaffold 合规化 + §11 dedicated I1~I11 测试改 SA-C.1 补丁轮交付)
> 状态:**v1 主体功能已签字**(2026-04-24)· dedicated integration tests 由 SA-C.1 补丁交付
> 依赖:R1(OpenAPI 冻结)+ R2 v3(`notifications` / `task_drafts` 表已生产落地)+ R3 v1 + R3.5 验证 + SA-A v2.1 签字 + SA-B v1.1 签字 + **R1.7-C OpenAPI SA-C 补丁**(2026-04-24 签字)
> 执行环境:**本地代码 + `jst_ecs` 上的 `jst_erp_r3_test` 测试库**(R3.5/SA-A/SA-B 已搭好 · 直接复用);生产 `jst_erp` 零写入
> 禁止前置:R1.7-C + SA-B v1.1 任一未签字时不得启动;SA-B.1 技术债补丁与本轮互不阻塞(可并行)

---

## 0. 本轮目标(一句话)

> **把 OpenAPI 冻结的 11 条 R4-SA-C 路径(4 task-drafts + 4 notifications + erp/by-code + design-sources/search + /ws/v1)从 `501` 补齐到 `V1_INFORMATION_ARCHITECTURE.md` v1.1 §3.5.9 / §8 / §9 + `V1_CUSTOMIZATION_WORKFLOW.md` v1.1 §3.1.2 / §3.2.2 合同**,落地通知生成器(§8.5 同事务)+ WebSocket hub 3 event types(§9.1)+ 草稿 7 天过期代码骨架(cron 默认 disabled · 沿用 SA-A 模板);**不改 OpenAPI**、**不动 R2 DDL**、**不碰用户管理/资产/搜索/报表**。

---

## 1. 必读输入

1. `docs/V1_INFORMATION_ARCHITECTURE.md` **v1.1** · **本轮唯一权威**
   - §3.5.9 任务草稿端点(**本轮主轴之一**)· IA-A13~A16(7 天过期 / 20 条上限 / 级联删除)
   - §4.2 全局搜索(SA-D 范围 · SA-C 不做)
   - §7.2 个人中心 "我的任务 → 草稿" 子 tab 读 `GET /v1/me/task-drafts`
   - §8 通知中心(**本轮主轴之二**)· 8.1 枚举 5 类 · 8.2 `notifications` 表结构 · 8.3 端点 · 8.4 仅站内 · 8.5 生成策略
   - §9 WebSocket 实时推送(**本轮主轴之三**)· 9.1 触发范围 3 事件 · 9.3 `/ws/v1` Bearer 鉴权 · 15s 回退轮询
2. `docs/V1_CUSTOMIZATION_WORKFLOW.md` **v1.1**
   - §3.1.2 ERP 产品编码查询(失败策略:上游 5xx/timeout → 502;未找到 → 404)
   - §3.2.2 设计源文件查询(v1 MVP · `keyword/page/size` 3 参数 · 无高级过滤)
3. `docs/V1_MODULE_ARCHITECTURE.md` **v1.2**
   - §6.2 deny_code 枚举(SA-C **不得新增**;403 全部用 `module_action_role_denied` / `draft_not_owner` / `notification_not_owner` 等自取 code)
   - §8.2 `tasks.priority` 4 值 `low/normal/high/critical`(通知按 priority 分流的扩展延到 R7+,v1 不做)
4. `docs/iterations/V1_R1_7_C_OPENAPI_SA_C_PATCH.md` **v1.0**(**本轮核心 · 5 条实装口径明文化**)
   - §5.1 task-drafts 授权 + IA-A15/A16 · §5.2 notifications scope · §5.3 erp/by-code 失败策略 · §5.4 design-sources/search MVP · §5.5 /ws/v1 三事件契约
5. `docs/iterations/V1_R2_REPORT.md` v1 + `V1_R3_REPORT.md` v1 + `V1_R3_5_INTEGRATION_VERIFICATION.md` v1 + `V1_R4_SA_A_REPORT.md` v1.1 + `V1_R4_SA_B_REPORT.md` v1.1
   - 真生产基线 · R3.5 测试库 · 本轮 integration 复用 `jst_erp_r3_test`
6. `docs/api/openapi.yaml` · 仅阅读以下 11 条路径的 schema(**禁止修改**):
   - `POST /v1/task-drafts` · `GET /v1/me/task-drafts` · `GET /v1/task-drafts/{draft_id}` · `DELETE /v1/task-drafts/{draft_id}`
   - `GET /v1/me/notifications` · `POST /v1/me/notifications/{id}/read` · `POST /v1/me/notifications/read-all` · `GET /v1/me/notifications/unread-count`
   - `GET /v1/erp/products/by-code` · `GET /v1/design-sources/search`
   - `GET /ws/v1`
7. 现有代码(**必读 · 本轮是扩展而非重建**):
   - `domain/auth_identity.go`(`User` / `UserSession` / `PermissionAction` 常量)· 只追加 · 不改现有
   - `service/erp_bridge/*`(**重点**:v0.9 已接 `/open/combine/sku/query` · SA-C 仅复用 · 不重建)
   - `service/module_action/*`(**重点**:R3 事件写入点 · SA-C 在**事件写入成功后**追加 1 行通知生成器调用 + WS 广播调用;具体文件由 Codex 按下文 §4 触点白名单定位,不得改事件核心逻辑)
   - `service/task_pool/claim.go`(R3)· SA-C 在 CAS 成功后追加 `wsHub.BroadcastPoolCountChanged(team_code)` 1 行
   - `testsupport/r35`(R3.5 测试 DSN 守卫 · 直接用)
   - R2 migration 063:`db/migrations/063_v1_0_task_drafts.sql`(`task_drafts` 表 · SA-C 会写的新表之一)
   - R2 migration 064:`db/migrations/064_v1_0_notifications.sql`(`notifications` 表 · SA-C 会写的新表之一)

禁止引用:R1.5 之前的文档;`docs/archive/*`;任何 R4-SA-A / SA-B / SA-D owner 的代码目录(`service/asset_center` / `service/asset_lifecycle` / `service/org_move_request` / `service/search` / `service/report_l1` 等)

---

## 2. 真生产基线(从 R2/R3.5/SA-A/SA-B 报告 + 2026-04-24 pre-probe 固化)

**Pre-probe 实测**(日志:`docs/iterations/r4_sa_c_probe_pre.log` · `mysql_server_time_utc = 2026-04-24 09:37:38`,此时间为 post probe 的窗口起点参数):

Codex 实现读路径时必须认到这些事实:

| 事实 | 影响 |
| --- | --- |
| `notifications` 表(R2 mig 064)生产 **0 条** · `task_drafts` 表(R2 mig 063)生产 **0 条**(2026-04-24 09:37:38 UTC 实测) | SA-C integration test 必须走 `jst_erp_r3_test` · 生产 probe B1/B2 期望**严格 0**(本轮生产不上 SA-C 代码);live traffic 不影响这两表(v0.9 无写入路径)|
| `task_module_events` 表生产 **300 行** · event_type 分布锁定:`migrated_from_v0_9`×298 / `backfill_placeholder`×2(R2 backfill 正确形态) | Post probe B4 不得出现这 2 种之外的 event_type;SA-C 生成器可读此表但**禁止扩新 event_type**(R3 枚举冻结) |
| **`design_sources` 表生产不存在**(pre-probe F 段已确认) | SA-C **必须**走 `task_assets WHERE source_module_key='design'` 回退 stub 路径;stub 基线 **278 行** design 类 `task_assets`;`service/design_source/service.go` 里 `design_sources` 分支可先写但生产走 fallback;**严禁**为 SA-C 新建 `design_sources` 表 |
| `NotificationType` v1 冻结 **5 类**:`task_assigned_to_me` / `task_rejected` / `claim_conflict` / `pool_reassigned` / `task_cancelled`(IA §8.1 · §8 段落明言 v1 不加 `task_mentioned`)| 通知生成器只能写这 5 个 value;**不得写** `task_timeout` 等未在枚举中的值;生成器写入前断言 type ∈ {5 类},否则 error |
| ERP Bridge `/open/combine/sku/query` 上游地址 `http://<host>:8081`(沿用 v0.9 `service/erp_bridge/*` 配置) | SA-C 不新建 HTTP 客户端;**复用现有** ERP client · 只加 `/v1/erp/products/by-code` handler thin wrapper |
| 生产 `task_module_events` 表有 300 行历史数据(R3.5 基线);R3 每笔模块动作都写 1~N 条事件 | 通知生成器**必须在事件写入成功的同事务中**触发;fire-and-forget 失败不得回滚主事务(IA §8.5);**生成器内部自身抛出的 panic 必须被 recover**,避免污染 R3 事务 |
| `/ws/v1` v1 事件只有 3 类(§9.1):`task_pool_count_changed` / `my_task_updated` / `notification_arrived`;不做全员广播 | WebSocket hub 的订阅维度严格 `user_id` + `team_code`;**不得**推送未在此 3 类中的任何 type |
| 前端 15s 回退轮询(IA §9.3)| 服务端**不需要**实现 WS 重连队列 / 消息持久化 · 丢包由前端轮询兜底 · v1 keep it simple |
| 生产 `permission_logs` 基线 **29678 行**(pre-probe) · live traffic 稳定增长 | post probe 窗口内 `permission_logs` 总行数 drift 允许;但 `action_type IN ('draft_access_denied','notification_access_denied')` 必须 **0** |
| SA-A/SA-B 控制字段(is_archived / cleaned_at / deleted_at / org_move_requests / users.status='deleted')生产全 **0**(pre-probe H 段) | 可用于 post probe 的跨域回归校验 · 若任一 SA-A/SA-B 字段非 0 → SA-C 跨域污染 → abort |

---

## 3. 交付范围

### 3.1 Domain / 模型层

| 文件 | 作用 |
| --- | --- |
| `domain/task_draft.go`(**新建**)| `TaskDraft` struct + 字段对齐 mig 063;`TaskDraftPayloadRaw` 使用 `json.RawMessage`(镜像 POST /v1/tasks body · 不强校验 · IA §3.5.9);`TaskDraftListItem` 用于 `GET /v1/me/task-drafts` 响应 |
| `domain/notification.go`(**新建**)| `Notification` struct + `NotificationType` enum(**5 值严格**)+ `NotificationPayload` 按 5 类事件规范化结构(每类 payload 键对齐 IA §8.1 表 · 如 `{task_id, module_key, assigned_by, reason}` 对 `task_assigned_to_me`)|
| `domain/websocket_event.go`(**新建**)| `WebSocketEvent` struct:`{Type string; Payload json.RawMessage}` + 3 常量 `WebSocketEventTaskPoolCountChanged` / `WebSocketEventMyTaskUpdated` / `WebSocketEventNotificationArrived` |
| `domain/erp_product_snapshot.go`(**若未存在则新建**;v0.9 若已有 `ERPProduct` 等价类型则复用)| `ERPProductSnapshot` 对齐 OpenAPI schema + Customization §3.1.2 字段 |
| `domain/design_source_entry.go`(**新建**)| `DesignSourceEntry` 对齐 OpenAPI schema + Customization §3.2.2 返回字段 7 项(`id/file_name/owner_team_code/preview_url/version_no/origin_task_id/created_at`)|

**严禁**:改 R3 落地的任何 domain 文件的现有字段;加 `notifications` / `task_drafts` 的 DDL 列;扩展 `NotificationType` 枚举(5 值冻结);新建 `TaskDraftPayload` 的严格 schema(保持 `json.RawMessage` 松绑)。

### 3.2 Service 层

| 包 | 文件 | 职责 |
| --- | --- | --- |
| `service/task_draft`(**新建**)| `service.go` | `CreateOrUpdate(ctx, actor, raw json.RawMessage) (*TaskDraft, *AppError)` · 解析 body 是否含 `draft_id`(有则 UPDATE,无则 INSERT)· 断言 owner_user_id = actor.ID · **IA-A16 执行**:`SELECT COUNT(*) FROM task_drafts WHERE owner_user_id=? AND task_type=?`,若 >= 20 则先 `DELETE ... ORDER BY created_at ASC LIMIT 1`(非事务内约束) |
|  | `service.go` | `List(ctx, actor, filter ListDraftFilter) (items []TaskDraftListItem, nextCursor string, *AppError)` · cursor-based 分页 · cursor 编码 `base64(updated_at_unix_ms + ':' + draft_id)` · 严格 `owner_user_id = actor.ID` |
|  | `service.go` | `Get(ctx, actor, draftID int64) (*TaskDraft, *AppError)` · **仅 owner** · 非 owner 返回 `AppError{Code:"draft_not_owner", HTTP:403}`(与 404 区分) |
|  | `service.go` | `Delete(ctx, actor, draftID int64) *AppError` · 仅 owner · 删除成功返回 nil |
|  | `service.go` | `DeleteBySourceDraftID(ctx tx, draftID int64) error` — **供 R3 `/v1/tasks` 创建成功后在同事务内调用**(IA-A15);不对外暴露 HTTP;权限由调用方保证(调用方应确认 `actor.ID == draft.owner_user_id`)|
|  | `service.go` | `CleanupExpired(ctx) (cleaned int, err error)` — **代码骨架**(沿用 SA-A 清理 job 模板)· 删除 `expires_at < NOW() AND expires_at IS NOT NULL`;本轮**不挂 cron**(`config.TaskDraftCleanupJob.Enabled = false` 默认)· R7+ 启用 |
| `service/notification`(**新建**)| `generator.go` | `NewGenerator(repo, userRepo, wsHub, logger) *Generator` · `GenerateForEvent(ctx, tx, evt TaskModuleEvent) error` — 按 IA §8.5 规则将 `task_module_events` 映射到 0~N 条 `notifications` 行;同事务写入;失败 log warn 不 return err(fire-and-forget 语义)· panic 必须 recover |
|  | `generator_rules.go` | 规则表(参见 §4 下文的映射)· 5 条 NotificationType → event_type 映射 · 输入 `evt TaskModuleEvent` · 输出 `[]NotificationCandidate{UserID, Type, Payload}` |
|  | `service.go` | `List(ctx, actor, filter) (items []Notification, nextCursor string, *AppError)` · cursor-based · 严格 `user_id = actor.ID` · `is_read` 可选过滤 |
|  | `service.go` | `MarkRead(ctx, actor, id int64) *AppError` · 严格 owner;非 owner → 403 `notification_not_owner`;not found → 404;已读调用幂等(`UPDATE ... SET is_read=1, read_at=NOW() WHERE id=? AND user_id=?`,0 rows affected 时再 SELECT 判断区分 403/404) |
|  | `service.go` | `MarkAllRead(ctx, actor) *AppError` · `UPDATE notifications SET is_read=1, read_at=NOW() WHERE user_id=? AND is_read=0` |
|  | `service.go` | `UnreadCount(ctx, actor) (int, *AppError)` · `SELECT COUNT(*) FROM notifications WHERE user_id=? AND is_read=0` |
|  | `service.go` | `CreateNotification(ctx, tx, userID, ntype, payload) (*Notification, error)` — **供生成器与 WS hub 共同调用**;写入后在同事务结尾触发 `wsHub.BroadcastToUser(userID, WebSocketEventNotificationArrived, payload)`(注:事务提交后才推送;实现上可用 `tx.AfterCommit(func)` 钩子或事务外二次触发)|
| `service/websocket`(**新建**)| `hub.go` | `NewHub(logger) *Hub` · in-memory connection map `map[userID][]*Connection` + `map[teamCode][]*Connection`;连接注册 / 反注册;并发安全(`sync.RWMutex`)· **进程内 hub 不做跨实例广播**(v1 仅单实例 · IA §9.2 不做广播扩散) |
|  | `hub.go` | `BroadcastToUser(userID int64, event domain.WebSocketEvent)` · 向 userID 所有连接推送 |
|  | `hub.go` | `BroadcastToTeam(teamCode string, event domain.WebSocketEvent)` · 向 teamCode 所有连接推送(用于 `task_pool_count_changed`) |
|  | `connection.go` | `Connection` wraps `*websocket.Conn`;`WritePump` / `ReadPump`(读仅用于心跳 · v1 不接受客户端命令) |
| `service/erp_product`(**新建 thin wrapper**)| `service.go` | `LookupByCode(ctx, code string) (*domain.ERPProductSnapshot, *AppError)` · 调用现有 `erpBridge.QueryCombineSKU(code)` · 映射错误:上游 404 → `AppError{HTTP:404, Code:"erp_product_not_found"}`;上游 5xx/timeout → `AppError{HTTP:502, Code:"erp_upstream_failure"}`;不缓存(v1 每次都查上游)|
| `service/design_source`(**新建**)| `service.go` | `Search(ctx, actor, keyword string, page, size int) (items []DesignSourceEntry, total int, *AppError)` · `SELECT id, file_name, owner_team_code, preview_url_key, version_no, origin_task_id, created_at FROM design_sources WHERE file_name LIKE CONCAT('%',?,'%') OR CAST(origin_task_id AS CHAR) LIKE CONCAT('%',?,'%') ORDER BY created_at DESC LIMIT ? OFFSET ?`;若 `design_sources` 表不存在(Customization §3.2.2 说 "v1 先仅覆盖首选来源"),退回 `task_assets WHERE source_module_key='design' AND lifecycle_state IN ('active','closed_retained','archived')` 做 stub;首选来源在生产未确认时由 Codex 在报告里列一句"`design_sources` 表未建 · 回退 task_assets" · **不得为 SA-C 新建 `design_sources` 表**(违反 §9 no-new-migrations) |

**严禁**:新建 `service/report_*` / `service/search/*` / `service/asset_*` 任何包;改 SA-B `service/identity_service.go` 任何方法;改 R3 `service/permission` / `service/blueprint` / `service/module` / `service/task_aggregator` 任何方法。

### 3.3 Repo 层

| 文件 | 作用 |
| --- | --- |
| `repo/task_draft_repo.go`(**新建**)| `TaskDraftRepo` interface:`Create/Get/List/Delete/DeleteExpired/CountByOwnerAndType/DeleteOldestByOwnerAndType` |
| `repo/mysql/task_draft_repo.go`(**新建**)| MySQL 实现 · 显式列名 SELECT · 列与 mig 063 对齐 · 不 `SELECT *` |
| `repo/notification_repo.go`(**新建**)| `NotificationRepo` interface:`Create/Get/List/MarkRead/MarkAllRead/UnreadCount` |
| `repo/mysql/notification_repo.go`(**新建**)| MySQL 实现 · 列与 mig 064 对齐 · `INDEX (user_id, is_read, created_at)` 利用 |

**严禁**:改 v0.9 / R3 / SA-A / SA-B 任何已有 repo。

### 3.4 Transport 层

#### 3.4.1 替换 501 stub(11 条)

| handler | 路径 | 角色门控 |
| --- | --- | --- |
| `handleTaskDraftCreateOrUpdate` | `POST /v1/task-drafts` | `session_token_authenticated`(登录用户)|
| `handleMyTaskDraftList` | `GET /v1/me/task-drafts` | 登录用户 · scope 强制 `owner_user_id = actor.ID` |
| `handleTaskDraftGet` | `GET /v1/task-drafts/{draft_id}` | 登录用户 + owner only |
| `handleTaskDraftDelete` | `DELETE /v1/task-drafts/{draft_id}` | 登录用户 + owner only |
| `handleERPProductByCode` | `GET /v1/erp/products/by-code` | 登录用户 |
| `handleDesignSourceSearch` | `GET /v1/design-sources/search` | 登录用户 |
| `handleMyNotificationList` | `GET /v1/me/notifications` | 登录用户 · scope `user_id = actor.ID` |
| `handleMyNotificationMarkRead` | `POST /v1/me/notifications/{id}/read` | 登录用户 + owner |
| `handleMyNotificationMarkAllRead` | `POST /v1/me/notifications/read-all` | 登录用户 |
| `handleMyNotificationUnreadCount` | `GET /v1/me/notifications/unread-count` | 登录用户 |
| `handleWebSocketUpgrade` | `GET /ws/v1` | Bearer token 鉴权 · 升级到 WS |

#### 3.4.2 新建文件

- `transport/handler/task_draft.go`(4 handler)
- `transport/handler/notification.go`(4 handler)
- `transport/handler/erp_product.go`(1 handler · thin)
- `transport/handler/design_source.go`(1 handler · thin)
- `transport/ws/websocket_handler.go`(1 handler · 负责 HTTP→WS upgrade · Bearer 鉴权 · 注册到 hub)

#### 3.4.3 路由挂载(`transport/http.go` 内追加)

```go
v1.POST("/task-drafts", withAuthenticated(...), taskDraftH.CreateOrUpdate)
v1.GET("/me/task-drafts", withAuthenticated(...), taskDraftH.MyList)
v1.GET("/task-drafts/:draft_id", withAuthenticated(...), taskDraftH.Get)
v1.DELETE("/task-drafts/:draft_id", withAuthenticated(...), taskDraftH.Delete)

v1.GET("/erp/products/by-code", withAuthenticated(...), erpProductH.ByCode)
v1.GET("/design-sources/search", withAuthenticated(...), designSourceH.Search)

v1.GET("/me/notifications", withAuthenticated(...), notificationH.MyList)
v1.POST("/me/notifications/:id/read", withAuthenticated(...), notificationH.MarkRead)
v1.POST("/me/notifications/read-all", withAuthenticated(...), notificationH.MarkAllRead)
v1.GET("/me/notifications/unread-count", withAuthenticated(...), notificationH.UnreadCount)

// WebSocket 注意:在 router root 挂载,不走 /v1 前缀(对齐 OpenAPI /ws/v1)
r.GET("/ws/v1", wsH.Upgrade)
```

---

## 4. 通知生成器 · 事件触点白名单(SA-C 允许改 R3 的唯一口径)

**IA §8.5 要求**:"后端在 `task_module_events` 写入时同事务触发'通知生成器'"。SA-C 为实现此要求,**仅允许**在 R3 现有的事件写入点追加 1 行调用:

### 4.1 R3 触点白名单(允许追加调用 · 不得改核心逻辑)

> **架构师裁决补丁(2026-04-24 · v1.1)**:"1 行追加"的 intent 是**热路径副作用 1 调用**(单 hot-path call),不是**整文件 1 行 diff**。允许的 DI 脚手架包括:
> 1. 在 R3 service struct(`ClaimService` / `ActionService`)上**追加 optional 字段**(如 `notificationGen claimNotificationGenerator`、`wsHub claimWebSocketHub`)+ 接口声明
> 2. 引入 functional `Option` pattern(`WithNotificationGenerator(gen)` / `WithWebSocketHub(hub)`)+ `New*` 构造器接受 `opts ...Option`
> 3. 热路径调用点用 `if s.notificationGen != nil { _ = s.notificationGen.GenerateForEvent(ctx, tx, evt) }` 包裹(nil-check 是 defensive boilerplate · 不算业务分支)
> 4. 包级 singleton / global 变量 → **禁止**(劣化可测试性)
>
> **必须保持的不变量**:
> - CAS 业务逻辑(modules.ClaimCAS / event payload / state transition)零变更
> - transaction boundary 不动 · 不得在外面/里面新建 tx
> - 不得新增 `task_module_events.event_type` 枚举值
> - 通知失败必须 fire-and-forget(`_ =` 显式吞错 · 不阻 R3 主路径)
> - WS broadcast **必须在 RunInTx 之后**(commit 后 · 失败不 rollback claim)

| R3 文件(由 Codex 按 grep 结果定位) | 允许的追加 | 不允许的改动 |
| --- | --- | --- |
| `service/module_action/*.go` 中每一处 `INSERT INTO task_module_events` 语句之后 | `notificationGen.GenerateForEvent(ctx, tx, evt)` 1 调用(传入事务 tx + 刚写入的 event)+ struct field + Option func + interface decl(DI scaffold 允许) | 改事件本身 payload / 改字段 / 删事件写入 / 改 transaction boundary / 包级 global |
| `service/task_pool/claim.go`(或 `claim_cas.go`)CAS 成功分支(tx commit 之后) | `wsHub.BroadcastPoolCountChanged(claimedTeam, poolCount)` 1 调用(注:`poolCount` v1 可硬编码 0 · R5+ tighten · 不阻签)| 改 CAS 逻辑 / 改 claim 业务分支 / 把 broadcast 放进 tx 内 |

**如果 R3 的事件写入没有统一入口**(Codex grep 发现写入散落在多个 handler 里),则 Codex 必须在每处追加 1 调用,保证 100% 覆盖;发现任何一处被漏加即是 SA-C bug。

### 4.2 事件 → 通知 映射规则(`service/notification/generator_rules.go`)

规则基于 IA §8.1 枚举表 + §5 审核驳回 + Q4 超时(v1 不触发超时通知,预留到 R7+):

| `task_module_events.event_type` | 生成的 `NotificationType` | 通知接收人(user_id) | payload 键 |
| --- | --- | --- | --- |
| `assigned`(reassign 发生 · 含 reassign_reason)| `task_assigned_to_me` | 新的 `claimed_by_user_id`(事件 payload 中) | `{task_id, module_key, assigned_by: evt.actor_id, reason: evt.reason}` |
| `rejected`(audit 驳回)| `task_rejected` | 被驳回模块的 `claimed_by_user_id`(事件 payload 需含 `claimed_by`)| `{task_id, reject_reason: evt.reason}` |
| `claim_conflict`(抢占失败 · R3 CAS 败方) | `claim_conflict` | R3 CAS 败方的 user_id(事件 payload 中 · 若 R3 目前未写此事件,则 SA-C **不可新增事件类型**,只能在 `service/task_pool/claim.go` CAS 败方分支追加 generator.Notify 等价调用 · 不经 `task_module_events` 表)| `{task_id, module_key}` |
| `pool_created`(新任务进池 · 按 IA §8.1 规则"我所在组新增一条 pool 任务"即每个 owner_team_code 成员) | `pool_reassigned` | `SELECT id FROM users WHERE team = target_team_code AND status='active' AND id <> task.created_by`(事件 payload 中含 team_code + task_id) | `{task_id, module_key}` |
| `cancelled`(任务被 cancel · R3 已有) | `task_cancelled` | `SELECT DISTINCT claimed_by_user_id FROM task_modules WHERE task_id = evt.task_id AND claimed_by_user_id IS NOT NULL` 并排除 `evt.actor_id` | `{task_id, cancel_reason: evt.reason, cancelled_by: evt.actor_id}` |

**规则引擎铁律**:
- 写入 `notifications` 行之前,必须断言 `notification_type ∈ {5 值}`;不等于 5 值中任一 → log warn 并跳过,**不 error 也不 abort**(fire-and-forget · IA §8.5)
- payload 构造不得读 `task_module_events` 之外的外部表(除非 R3 事件 payload 缺少必要字段 · 此时通过同事务 SELECT · 禁止跨事务)
- panic 必须 recover · 不污染 R3 事务

### 4.3 WS 推送 · `notification_arrived` 时机

通知写入 `notifications` 成功后,**事务提交之后**触发 `wsHub.BroadcastToUser(userID, WebSocketEventNotificationArrived, payload)`。具体实现可两种:

| 方式 | 说明 |
| --- | --- |
| **推荐**:`AfterCommit` 钩子 | 在同事务对象 `tx` 上注册钩子 `tx.AfterCommit(func(){ wsHub.Broadcast... })`;R3 事务框架若无此能力,走下一方式 |
| 备选:两段式 | 事务提交后在生成器外层包一个 defer,事务成功才调用 broadcast |

**绝不允许**:在事务回滚时已经推送 WS(会让前端收到不存在的通知)。

---

## 5. WebSocket Hub 契约(IA §9 权威)

### 5.1 事件类型(3 类 · v1 冻结)

| 事件 type | Payload 结构 | 推送维度 |
| --- | --- | --- |
| `task_pool_count_changed` | `{team_code: string, pool_count: int}` | 按 `team_code` 订阅的连接(SA-C 注册时从 Session 读 user.team;亦支持 user 选择订阅多 team) |
| `my_task_updated` | `{task_id: int, module_key: string, new_state: string, event_id: int}` | 按 `user_id` 订阅 |
| `notification_arrived` | `{notification_id: int, notification_type: string, unread_count: int}` | 按 `user_id` 订阅 |

### 5.2 Frame 格式(IA §9.3)

```json
{ "type": "task_pool_count_changed", "payload": { "team_code": "ops_team_a", "pool_count": 17 } }
```

服务端发送 · 客户端**只读**(v1 不接受客户端自定义 frame · 仅接受 `ping/pong` 心跳)。

### 5.3 鉴权

- Upgrade 请求 header `Authorization: Bearer <token>` 或 query `?access_token=<token>`(前者优先)
- 鉴权失败返回 401 + JSON(不 upgrade)
- token 过期中途连接保持(v1 不做主动 kick · R7+ 再评估)

### 5.4 心跳与超时

- 服务端每 30s 发一次 WebSocket `PingMessage`
- 客户端无响应 60s 视为断开 · 清理连接 map
- 客户端主动 Close → 立即反注册

### 5.5 单实例约束(重要)

- v1 WebSocket hub 是**进程内**的(IA §9.2 不做全员广播,v1 单实例即可)
- 如果将来上多实例,需要 Pub/Sub(Redis Pub/Sub 或等价)扩散 · 本轮**不做**
- Codex 不实现 Redis 订阅层;hub 只做 in-memory map

---

## 6. 审计事件清单(SA-C 必须落 / 不必落的写入)

SA-C 的多数动作**不需要**写 `permission_logs`,因为不是敏感操作(草稿本人可见 / 通知本人可见 / ERP/设计搜索只读 / WS 只读推送)。但**以下两种必须写**:

| 触发端点 | `permission_logs.action_type` | payload keys |
| --- | --- | --- |
| `DELETE /v1/task-drafts/{draft_id}`(non-owner 尝试)| `draft_access_denied` | `{actor, draft_id, reason: "not_owner"}` |
| `POST /v1/me/notifications/{id}/read`(non-owner 尝试)| `notification_access_denied` | `{actor, notification_id, reason: "not_owner"}` |

(正常 owner 操作草稿 / 通知不写审计 · 减少噪音)

---

## 7. 执行环境

**复用 R3.5 + SA-A + SA-B 的 `jst_erp_r3_test`**(在 `jst_ecs`):

```bash
ssh jst_ecs 'mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -p"$DB_PASS" -e "SHOW DATABASES LIKE '\''jst_erp_r3_test'\'';"'

DSN=$(ssh jst_ecs 'bash /root/ecommerce_ai/r3_5/build_test_dsn.sh')
MYSQL_DSN="$DSN" R35_MODE=1 /home/wsfwk/go/bin/go test ./service/task_draft/... ./service/notification/... ./service/websocket/... -tags=integration -count=1 -v
```

**严格守则**:
- 所有 integration test 必须走 `testsupport/r35.MustOpenTestDB(t)`,DSN 不以 `_r3_test` 结尾即 `t.Fatalf`
- SA-C 追加的测试样本 `task_id >= 40000`(SA-A `task_id >= 20000`;SA-B `user_id >= 30000`;SA-C `task_id >= 40000`);`task_drafts.id` / `notifications.id` 无需特殊隔离,测试后 `DELETE WHERE id IN (...)` 清理
- WebSocket integration test 用 `httptest.NewServer` + `gorilla/websocket.Dialer` 本地建连 · 不依赖生产 WS

---

## 8. 测试要求

### 8.1 单元测试(100% 绿)

- `service/task_draft/service_test.go`:Create(无 draft_id)/Update(有 draft_id)/IA-A16 超 20 条先删最老 · Get(owner 204 / non-owner 403 / 404)/ Delete 类似 / DeleteBySourceDraftID(IA-A15)
- `service/notification/generator_test.go`:5 类事件 × 5 条规则的表驱动;断言 NotificationType 严格 ∈ {5 值};未知事件类型 log warn 不报 error;panic recover 验证
- `service/notification/service_test.go`:List/MarkRead(owner/non-owner/not-found)/MarkAllRead/UnreadCount
- `service/websocket/hub_test.go`:注册/反注册/BroadcastToUser/BroadcastToTeam 并发安全 · 用 goroutine 数 100 的并发 test
- `service/erp_product/service_test.go`:上游 200 / 404 / 5xx / timeout 四场景映射(mock `erpBridge.QueryCombineSKU`)
- `service/design_source/service_test.go`:keyword LIKE + page/size 边界(page=0/size>100 拒绝)

### 8.2 Integration Tests(build tag `integration`)

| # | 断言 | 对应验收 |
| --- | --- | --- |
| SA-C-I1 | `POST /v1/task-drafts` body 镜像 POST /v1/tasks · 新建返回 `draft_id`;再带 `draft_id` POST 走 UPDATE · DB 行不新增(同一 draft_id)| IA §3.5.9 / R1.7-C §5.1 |
| SA-C-I2 | 同一 owner + task_type 已有 20 条草稿,第 21 条 POST → 最老 1 条被删除 · 总数仍 20(IA-A16)| IA §3.5.9 IA-A16 |
| SA-C-I3 | 模拟 `POST /v1/tasks` 带 `source_draft_id` 创建成功 → 调用 `TaskDraftService.DeleteBySourceDraftID(tx, id)` · 事务提交后 draft 已删 · 事务回滚则 draft 不删(IA-A15)| IA §3.5.9 IA-A15 |
| SA-C-I4 | `GET /v1/task-drafts/{other_user_draft_id}` → 403 `draft_not_owner`;`DELETE` 同 · 审计 `draft_access_denied` 已写 | R1.7-C §5.1 F1 |
| SA-C-I5 | 模拟 R3 写 `task_module_events` event_type=`assigned` + `claimed_by=U` · 同事务触发 generator · `notifications` 新增 1 行 type=`task_assigned_to_me` user_id=U;事务回滚则通知不存在 | IA §8.5 |
| SA-C-I6 | `GET /v1/me/notifications?is_read=false&limit=5` 返回当前用户未读 · 不返回其他 user 的通知(即使 SuperAdmin 调也只看自己);`next_cursor` 正确指向下一页 | IA §8.3 B2' |
| SA-C-I7 | `POST /v1/me/notifications/{other_user_notif_id}/read` → 403 · 审计 `notification_access_denied` 已写 | R1.7-C §5.2 |
| SA-C-I8 | `POST /v1/me/notifications/read-all` 只更新 `user_id = actor.ID` 行 · 其他用户未读数不变 | IA §8.3 |
| SA-C-I9 | `GET /v1/erp/products/by-code?code=EXIST` → 200 + snapshot;`code=NOTEXIST` → 404 `erp_product_not_found`;上游 mock 返回 500 → 502 `erp_upstream_failure` | Custom §3.1.2 |
| SA-C-I10 | `GET /v1/design-sources/search?keyword=...&page=1&size=20` 返回匹配项 + `total`;page=0 → 400;size=101 → 400 · 回退到 `task_assets` stub 时仍能返 200(若 `design_sources` 表未建) | Custom §3.2.2 |
| SA-C-I11 | WebSocket 测试:用 `httptest.Server` + `gorilla/websocket.Dialer` 建连 `ws://...test.../ws/v1?access_token=...` · 注册 user_id=U + team_code=T · hub.BroadcastToUser(U, ...) 客户端收到 frame `{"type":"notification_arrived","payload":{...}}` · hub.BroadcastToTeam(T, ...) 同样收到;非订阅 user/team 不收到 | IA §9.1 / §9.3 |

### 8.3 全量回归

```bash
# WSL(Windows AppControl 会拦 Go test · WSL 无限制)
/home/wsfwk/go/bin/go build ./...
/home/wsfwk/go/bin/go test ./... -count=1

# Integration
DSN=$(ssh jst_ecs 'bash /root/ecommerce_ai/r3_5/build_test_dsn.sh')
MYSQL_DSN="$DSN" R35_MODE=1 /home/wsfwk/go/bin/go test ./... -tags=integration -count=1

# OpenAPI
/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml  # 期望 0 error 0 warning
```

### 8.4 生产 Probe(零污染证据 · 沿用 SA-A v2.1 / SA-B v1.1 模板)

Probe 脚本由主对话打包交付:`tmp/sa_c_probe_readonly.sh`(已落盘 · 真实列名对齐 mig 063/064:`notifications.notification_type` + `task_drafts.payload` JSON 列);**本轮 Codex 不自行写 probe SQL** · 如脚本缺字段回报主对话。

执行两阶段:

```bash
# Pre(已由主对话于 2026-04-24 09:37:38 UTC 执行完成;日志 docs/iterations/r4_sa_c_probe_pre.log)
# Codex 不需要重跑 pre

# Post(SA-C 跑完后 · 传 pre 的 mysql_server_time_utc='2026-04-24 09:37:38' 作参数 · 4 条硬门聚合期望全 0)
ssh jst_ecs "bash /tmp/sa_c_probe.sh '2026-04-24 09:37:38'" | tee docs/iterations/r4_sa_c_probe_post.log
```

Post 阶段 SA-C 控制字段聚合硬门(4 条):

| 脚本段 | 聚合 | 期望值 | 含义 |
| --- | --- | --- | --- |
| B1 | `SELECT COUNT(*) FROM notifications WHERE created_at >= :sa_c_start_time` | **0**(如有非 0 · 必须通过 B1-context 枚举分布 + B1-diag 明细解释为 live traffic) | SA-C 未写生产 `notifications`(测试走 `jst_erp_r3_test`) |
| B2 | `SELECT COUNT(*) FROM task_drafts WHERE created_at >= :sa_c_start_time OR updated_at >= :sa_c_start_time` | **0** | SA-C 未写生产 `task_drafts` |
| B3 | `SELECT COUNT(*) FROM permission_logs WHERE created_at >= :sa_c_start_time AND action_type IN ('draft_access_denied','notification_access_denied')` | **0** | SA-C 未写审计到生产 |
| B4 · 复合 | `SELECT notification_type, COUNT(*) FROM notifications GROUP BY notification_type`(§C 段)必须仅命中 5 值枚举;`SELECT event_type, COUNT(*) FROM task_module_events WHERE created_at >= :sa_c_start_time`(§B4 段)不得出现 R3 未声明的新 event_type | 仅 5 值 NotificationType + 仅已知 R3 event_type | SA-C 未扩枚举 |

**行数 drift 非 abort**(沿用 SA-A v2.1 / SA-B v1.1 裁决):生产其他表(`tasks` / `task_modules` / `task_assets` / `users` 等)任何行数变化均为 live traffic · 不触发 abort;只要 B1~B4 聚合符合期望即通过。

**若 B1 非 0**(生产有新通知):必须先确认生产是否已上 SA-C 代码(如果上线则 B1 期望非 0 但 notification_type 必须仅在 5 值内 · 且 B1-diag 明细可解释);未上线则 B1 必须 0,非 0 即污染。

---

## 9. 交付物清单

1. Domain 层 5 新文件(§3.1)
2. Service 层 5 新包(§3.2) · **包含通知生成器规则引擎 · 包含 WebSocket hub · 不含 cron 启用**
3. Repo 层 4 新文件(§3.3)
4. Transport 层 5 新 handler 文件 + `transport/ws/*` + `transport/http.go` 11 路由挂载(§3.4)
5. **R3 触点 1 行追加**(§4.1):`service/module_action/*.go` 每处事件写入后 `notificationGen.GenerateForEvent(tx, evt)` + `service/task_pool/claim.go` CAS 成功后 broadcast · 报告必须明列改动行号
6. 11 条 integration test(build tag `integration` · §8.2)+ 单测 6 组(§8.1)
7. `docs/iterations/V1_R4_SA_C_REPORT.md`:

**报告强制章节**:

- `## Scope` · 11 条 handler 清单 + 每条的 deny 路径 + 返回码矩阵
- `## §4 通知生成器规则表` · 5 类 event_type × 5 类 NotificationType 的实装断言
- `## §5 WebSocket Hub 契约` · 3 事件 type 的 payload 示例 + 连接注册/反注册的证据
- `## §4.1 R3 触点 diff` · 对每个被追加 1 行的 R3 文件列出:旧行 → 新行(行号 + 改动内容)+ 确认没动其他逻辑
- `## 11 Integration Assertions` · SA-C-I1~I11 的实际 SQL / HTTP / WS 数据证据
- `## ERP Bridge Contract` · 200/404/502 三种上游响应 × 映射后的 AppError code 证据
- `## Design Source Fallback` · `design_sources` 表存在时走一等来源 · 不存在时退到 `task_assets` stub 的证据(含 `SHOW CREATE TABLE` 输出)
- `## NotificationType Enum Lock` · 生成器写入前的 assert 代码 + 测试"注入非法 type 返回 warn" 的断言
- `## Test DB Touch` · `jst_erp_r3_test` 中 SA-C 测试数据范围(`task_id >= 40000` 段 + `task_drafts.id` + `notifications.id`);执行后的 defer 清理证据
- `## Production Probe Diff` · §8.4 四条控制字段聚合全 0 的 SQL + 输出
- `## OpenAPI Conformance` · 0/0
- `## Known Non-Goals` · IA §4.2 全局搜索(SA-D)+ 报表(SA-D)+ 多实例 WS Pub/Sub(R7+)+ `task_timeout` 通知(R7+)+ `task_mentioned` 通知(R7+)+ WS 消息持久化(不做 · 前端 15s 轮询兜底)

---

## 10. 失败终止条件

- 11 条 R4-SA-C 路径在 OpenAPI 里 schema 字段与 IA §3.5.9 / §8 / §9 / Customization §3.1.2 / §3.2.2 对不上 → **回报主对话**(而非 abort 重起一轮;R1.7-C 已签字,若有残漏由主对话裁决是否开 R1.7-C.1)
- 任何 SA-C 代码触发非白名单表的写入(写 `tasks` / `task_assets` / `asset_*` / `users` / `org_move_requests` / `user_roles` 任一)→ abort
- 任何 DSN 守卫失效(测试跑出 `jst_erp` 写入)→ abort
- integration 断言任一 FAIL · WS integration 不得 `t.Skip`
- 生产 probe **控制字段聚合**任一 ≠ 期望值(见 §8.4 表)→ abort;**表行数 drift 不算违反**
- 通知生成器尝试写入非 5 值 `NotificationType` → abort
- `task_module_events` 新增 event_type(SA-C 不得扩 R3 事件枚举)→ abort
- 需要新 migration 才能通过测试 → abort(`design_sources` 表未建则用 `task_assets` stub · 不建表)
- 需要在 `transport/http.go` 下线 `/v1/auth/*` 或改动 SA-B 任何路由才能通过测试 → abort
- R3 文件被改动超过 `§4.1 触点白名单`(改了 CAS 业务逻辑 / 改了 event payload / 改了 transaction boundary / 加了包级 global)→ abort;**DI scaffold(struct field + Option func + interface decl)是合规追加 · 不触发 abort**(v1.1 补丁)
- WebSocket hub 引入 Redis / gRPC / Kafka 等分布式 Pub/Sub 依赖 → abort(v1 单实例)

---

## 11. 控制字段白名单(SA-C 允许写入的表/列)

| 表 | 允许写入的列 | 备注 |
| --- | --- | --- |
| `task_drafts` | 全部列(mig 063 · insert/update/delete) | 严格 `owner_user_id = actor.ID` |
| `notifications` | 全部列(mig 064 · insert/update · 仅 `is_read/read_at` 可 update) | 严格 `user_id = target_user_id` |
| `permission_logs` | 全部列(insert only) | 仅限 `draft_access_denied` / `notification_access_denied` 两个 action_type |
| `task_module_events` | **零改动**(R3 表;SA-C 只读)| 通知生成器只读此表 payload |
| `users` / `user_roles` | **零改动** | SA-B 域 |
| `tasks` / `task_modules` / `task_assets` / `asset_*` / `org_move_requests` | **零改动** | SA-A / SA-B / R3 域 |

**R3 代码触点白名单**(见 §4.1):
- `service/module_action/*.go` — 每处事件写入后追加 1 行 `notificationGen.GenerateForEvent(tx, evt)`
- `service/task_pool/claim.go` — CAS 成功后追加 1 行 `wsHub.BroadcastToTeam(team_code, ...)` · CAS 败方分支追加 1 行 `notificationGen.NotifyClaimConflict(ctx, loser_user_id, task_id, module_key)`

**绝对禁止**:改 R3 事件 payload 结构;改 R3 transaction boundary;新增 R3 事件 type;在 SA-C 里创建任何 cron / scheduler / background worker 启用(`CleanupExpired` 代码写好 · `Enabled=false` · R7+ 启)。

---

## 12. 给 Codex 的最后一句话

> SA-C 是 R4 的第 3 轮,前面有 SA-A / SA-B 已签字,后面还有 SA-D 1 轮。
> 你的范围只有"**草稿 + 通知 + WebSocket + ERP 桥接 + 设计源搜索**",用户管理 / 资产 / 任务 / 全局搜索 / 报表 全不归你。
> v0.9 已有的 `service/erp_bridge/*` 是你的**起点**,不重建 HTTP 客户端;R3 的 `service/module_action/*` 是你**追加 1 行**的地方,不改业务逻辑;WebSocket hub 只做进程内 map + 3 事件推送,**不引 Redis**。
> 通知生成器只能写 5 值 `NotificationType`;事件写入非 5 值 → warn 并跳过(fire-and-forget)· 绝不 panic 主事务。
> 遇到需要改 OpenAPI / 改 DDL / 新增 `NotificationType` enum / 动 SA-A/SA-B 目录 / 引分布式 Pub/Sub 的冲动,**立即 abort** 并报告,让主对话裁决。
> 不要在生产 `jst_erp` 上跑任何东西,测试库是 `jst_erp_r3_test`;控制字段聚合 4 条全 0 才是零污染证据,表行数漂移是正常 live traffic。

---

## 变更记录

| 版本 | 日期 | 变更 |
| --- | --- | --- |
| v1 | 2026-04-24 | 初稿 · 基于 R1.7-C v1.0 / IA v1.1 §3.5.9 §8 §9 / Customization v1.1 §3.1.2 §3.2.2 / SA-A v2.1 + SA-B v1.1 模板固化。11 handler 清单锁定(`x-owner-round: R4-SA-C`);通知生成器 5 值 enum 冻结 · 5 类 event_type 映射规则内嵌 §4.2;WebSocket hub 单实例约束 · 3 事件类型契约 §5 内嵌;R3 触点白名单严格到"1 行追加" § 4.1;`design_sources` 表未建时退 `task_assets` stub(不新建表);草稿 IA-A15/A16 事务规则 §3.2 锁定;7 天过期清理 job 代码骨架 · cron 默认 disabled(沿用 SA-A 模板 · R7+ 启用)· 生产 probe 走 4 条控制字段聚合(行数 drift 非 abort 沿用 SA-A v2.1) |
