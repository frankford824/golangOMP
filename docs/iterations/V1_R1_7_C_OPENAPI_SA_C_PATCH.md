# V1 R1.7-C · OpenAPI SA-C Patch Report

> Status: **v1.0 · 已签字生效**(架构师主对话直接打 · 2026-04-24)
> 依据:`prompts/V1_ROADMAP.md` R4-SA-C 起稿前置 · 沿用 R1.7-B 成功模板
> 路径:11 条 `x-owner-round: R4-SA-C`(4 task-drafts + 4 notifications + 1 erp/by-code + 1 design-sources/search + 1 /ws/v1)

---

## 1. 缘由

R4-SA-B v1.1 签字后进入 P3 顺序第 3 轮(SA-C)起稿阶段。主对话按 R1.7-B 流程先扫描 SA-C 11 条路径的 schema 完整度,发现:

- **2 处 block**:`/v1/design-sources/search` 完全无 query 参数;`/ws/v1` 无 frame 契约说明
- **8 处 minor**:11 条 path 全部残留 `501 Reserved for R4-SA-C` · 多处缺 401/403/404/502 · 多处无 `x-rbac-placeholder`

若不先打 R1.7-C 补丁,SA-C 起稿时 Codex 会按 SA-A v2.1 / SA-B v1 先例触发 abort("schema 与权威文档不一致")。

## 2. 架构问答收敛(6 问)

架构师原推荐与 IA v1.1 签字版本核对,发现 4 处冲突,**立即收敛到权威对齐版**(零新决策 · 全部按权威文档):

| 题 | 原推荐 | **权威对齐版**(采用)| 权威来源 |
| --- | --- | --- | --- |
| Q1 WS | A2:新建 `docs/V1_WEBSOCKET_FRAMES.md` | **A2'**:`/ws/v1` description 直接 link `V1_INFORMATION_ARCHITECTURE §9`(无需新建文档) | IA §9 已完整定义 message 格式 + 3 事件 + 15s 回退轮询 |
| Q2 notifications list | B2:`?unread=&page=&page_size=` | **B2'**:`?is_read=&limit=&cursor=`(cursor-based) | IA §8.3 原文 |
| Q3 design-sources/search | C1:`?keyword=&owner_team_code=&page=&page_size=` | **C1'**:`?keyword=&page=&size=`(3 参数) | V1_CUSTOMIZATION_WORKFLOW §3.2.2 原文 |
| Q4 NotificationType enum | D2:加 `task_timeout` | **D1**:保持 5 类 `task_assigned_to_me` / `task_rejected` / `claim_conflict` / `pool_reassigned` / `task_cancelled` | IA §8.1 原文(明言 v1 不加 `task_mentioned` · 无 `task_timeout`) |
| Q5 TaskDraftPayload | E1:`additionalProperties:true` 不强校验 | **E1 不变** | IA §3.5.9:"body shape 与 `POST /v1/tasks` 完全一致" · 松绑合理 |
| Q6 drafts 授权 | F1:仅 owner | **F1 不变** | IA §3.5.9:"仅草稿创建者"可读/删 · 权威原文支持 |

### 为什么停下收敛

按 `CLAUDE.md`:当文档冲突,OpenAPI → 权威文档 → `transport/http.go` 序列,权威文档相对新 OpenAPI 为 source of truth。原推荐 D2/B2/C1 与 IA v1.1 签字版冲突,若直接打 → 重走 R1.5 "字段幻觉" 教训。故架构师主对话停止并汇报、用户确认"对齐"后才开打。

## 3. 打补丁范围(零代码 · 零 migration · 仅 OpenAPI)

### 3.1 11 路径总览

| # | Path | Method | 改动 |
| --- | --- | --- | --- |
| C1 | `POST /v1/task-drafts` | POST | 去 501 + `x-rbac-placeholder` + 400/401 · description link IA §3.5.9 + IA-A15/A16 规则 |
| C2 | `GET /v1/me/task-drafts` | GET | 去 501 + 补 `?task_type=&limit=&cursor=` 三参数(IA §3.5.9 原文)+ response 加 `next_cursor` + 401 |
| C3 | `GET /v1/task-drafts/{draft_id}` | GET | 去 501 + 403 "Not the draft owner"(F1)+ 401 |
| C4 | `DELETE /v1/task-drafts/{draft_id}` | DELETE | 去 501 + 403(F1)+ 401 |
| C5 | `GET /v1/erp/products/by-code?code=` | GET | 去 501 + **404**(ERP 无此 code · §3.1.2 失败策略)+ **502**(ERP 5xx/timeout)+ 401 |
| C6 | `GET /v1/design-sources/search` | GET | 去 501 + **补 `?keyword=&page=&size=` 三参数**(C1' · §3.2.2 原文)+ response 加 `total/page/size` + 401 |
| C7 | `GET /v1/me/notifications` | GET | 去 501 + **补 `?is_read=&limit=&cursor=` 三参数**(B2' · §8.3 原文)+ response 加 `next_cursor` + 401 |
| C8 | `POST /v1/me/notifications/{id}/read` | POST | 去 501 + 403 "Not notification owner" + 404 + 401 |
| C9 | `POST /v1/me/notifications/read-all` | POST | 去 501 + 401 |
| C10 | `GET /v1/me/notifications/unread-count` | GET | 去 501 + 401 |
| C11 | `GET /ws/v1` | GET(Upgrade) | 去 501 + description **link IA §9 完整 frame 契约**(message format / 3 event types / 15s 回退)+ 401 · OpenAPI 3.0 不支持 async 故不再在 schema 定义 frame |

### 3.2 Schema 改动

**零新增 schema**:
- `TaskDraft` / `TaskDraftPayload` / `Notification` / `NotificationType` / `ERPProductSnapshot` / `DesignSourceEntry` / `SearchResultGroup` 现有定义全部保留不动
- `NotificationType` enum 保持 5 值不变(D1)

### 3.3 `x-rbac-placeholder` 追加

11 条路径全部补 `x-rbac-placeholder: { auth_mode: session_token_authenticated }`(或 `bearer_authenticated` · `/ws/v1`),标记 v1 等待 R6 真实 RBAC 路由策略归档。

## 4. 验证

```bash
$ /home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
warning: both GOPATH and GOROOT are the same directory (/home/wsfwk/go); see https://go.dev/wiki/InstallTroubleshooting
openapi validate: 0 error 0 warning
```

```bash
$ rg "Reserved for R4-SA-C" docs/api/openapi.yaml
(no matches)
```

## 5. SA-C 实装口径内嵌(给 R4-SA-C prompt 起草用)

**5.1 task-drafts(C1~C4)**:
- 授权:owner 单向(IA §3.5.9)· DeptAdmin/SuperAdmin 也不能读/删他人草稿 · 单测覆盖 `GetDraft_OtherUser → 403`
- IA-A15:`POST /v1/tasks` body 带 `source_draft_id` 且创建成功 → 同事务内 `DELETE task_drafts WHERE id = source_draft_id`
- IA-A16:同 `(owner_user_id, task_type)` 超 20 条 → `POST /v1/task-drafts` 先删最老后插入(非事务内业务规则;可用 `ORDER BY created_at ASC LIMIT 1` + DELETE)
- 7 天过期:后端定时任务(可复用 SA-A 清理 job 骨架 · **默认 disabled**)清理 `expires_at < NOW()`;v1 先落代码 · R7+ 启用 cron

**5.2 notifications(C7~C10)**:
- 所有路径严格 scope 到 `user_id = current_user` · 即使 SuperAdmin 也不读他人 notifications(隐私;IA §8 未明言但 notifications 本就是私人消息队列)
- `notification_type` 5 类枚举严格校验(IA §8.1)· 生成器只能写这 5 类 · 生产若发现历史行存在其他值应该 log warn(R1.5 式守卫)
- cursor 格式:`base64(created_at_unix_ms + ':' + id)` · limit 边界 `[1, 100]` · `is_read=true/false` 参数转 `'0'/'1'` 过滤

**5.3 erp/by-code(C5)**:
- 失败策略(§3.1.2):上游 5xx/timeout → 502 · 上游未找到 → 404 · 不使用 200 + `success:false` 这种含糊语义
- 复用现有 `service/erp_bridge/*`(v0.9 已存 · 接 `/open/combine/sku/query`)

**5.4 design-sources/search(C6)**:
- v1 MVP(§3.2.2):仅 keyword 全文(file_name + origin_task_id)+ created_at DESC · 无 owner_team_code 过滤 · 无高级过滤
- 权限:登录用户可查(不限 team · 定制组 / 设计组 / 审核组都可能需要)

**5.5 /ws/v1(C11)**:
- Bearer token 鉴权(IA §9.3)
- 3 event types(IA §9.1):`task_pool_count_changed`(订阅维度 team_code)/ `my_task_updated`(user_id)/ `notification_arrived`(user_id)
- Codex 在 Go 侧使用 `github.com/gorilla/websocket` 或等价包;frame format `{"type":"...","payload":{...}}` 严格
- 断线无需服务端兜底 · 前端 15s 回退轮询兜底

## 6. 遗留项(不纳入本轮)

| 项 | 归属 |
| --- | --- |
| R5+ 独立 WebSocket frame 契约文档 `docs/V1_WEBSOCKET_FRAMES.md` | 若 R4-SA-C 实装过程中发现 IA §9 不够详细,由 SA-C 回报主对话扩展;R4 完成后再评估独立文档 |
| `NotificationType` 扩展(`task_timeout` / `task_mentioned`)| R7+ 专门一轮 · 需先修 IA §8.1 签字后再动 OpenAPI |
| `design-sources/search` 高级过滤(`sku_prefix` / `category_code` / `owner_team_code`)| R7+(IA §3.2.2 MVP 明确不在 v1 范围) |

## 7. 签字

- 架构师:主对话
- 日期:2026-04-24
- openapi-validate:`0 error 0 warning`
- 501 残留:`0`
- 入册:**R1.7-C v1.0**
- 下一步:起草 `prompts/V1_R4_FEATURES_SA_C.md`(通知中心 + WebSocket + ERP by-code + 设计源文件搜索 + 草稿端点)

---

## 修订记录

| 版本 | 日期 | 变更 |
| --- | --- | --- |
| v1.0 | 2026-04-24 | 初稿 · 11 path 补齐 + 零 schema 新增 · 架构 6 问全部收敛到权威对齐版(A2'/B2'/C1'/D1/E1/F1)· openapi-validate 0/0 |
