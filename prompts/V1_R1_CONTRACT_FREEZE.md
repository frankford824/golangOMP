# V1 R1 · 契约冻结 + HTTP 骨架

> 轮次：**R1 / 6**
> 目的：在四份 v1.0 权威文档已签字的前提下，**一次性把所有新增 / 变更的 HTTP 契约冻结到 `docs/api/openapi.yaml`**，并在 `transport/http.go` 注册完整路由骨架（handler 全部返回 `501 Not Implemented`），为后续 R2 ~ R6 锁定"输入边界"。
> 严格禁止：在本轮实现任何业务逻辑、改 domain、改 repo、动迁移。
> 产出：一个可 `go build` 通过、`go test ./transport/...` 通过、且对所有新路由返回结构化 501 的 PR。

---

## 0. 只读上下文（Codex 进场时必须先读，按顺序）

1. `CLAUDE.md`
2. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
3. `docs/V1_MODULE_ARCHITECTURE.md`（v4 · 已签字）
4. `docs/V1_INFORMATION_ARCHITECTURE.md`（v3 · 已签字）
5. `docs/V1_ASSET_OWNERSHIP.md`（v1 · 已签字）
6. `docs/V1_CUSTOMIZATION_WORKFLOW.md`（v2 · 已签字）
7. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`（兼容策略）
8. `docs/api/openapi.yaml`（需增改的目标文件；现状 6297 行）
9. `transport/http.go`（路由注册处）
10. `transport/route_access_catalog.go`（菜单可见性登记）
11. `db/migrations/058_v1_0_org_team_department_scoped_uniqueness.sql`（确认当前号段底线）

> 本轮 **不** 要读：`repo/`、`service/` 内任何业务实现；不要主动跑 migration；不要动任何测试数据。

---

## 1. 本轮范围（MUST DO）

### 1.1 OpenAPI 新增 / 修改的路径清单（权威，全集）

按主文档 §9.1 / §9.1.1 + IA §3.5 / §3.5.9 / §4 / §5 / §6 / §7 / §8 + 资产 §5 + 定制 §3.1.2 / §3.2.2 汇总如下。**每一条都必须在本轮落入 `docs/api/openapi.yaml`，且注明 `x-owner-round: R2|R3|R4|R5|R6`（便于后续轮次 grep 认领）**。

| # | Method | Path | x-owner-round | 出处 |
| --- | --- | --- | --- | --- |
| **任务容器 / 模块** |
| 1 | GET | `/v1/tasks` | R3 | 主 §9.1 |
| 2 | GET | `/v1/tasks/{id}/detail` | R3 | 主 §9.1 / §9.2 |
| 3 | GET | `/v1/tasks/pool` | R3 | 主 §9.1 / §7.1 |
| 4 | POST | `/v1/tasks/{id}/modules/{module_key}/claim` | R3 | 主 §9.1 |
| 5 | POST | `/v1/tasks/{id}/modules/{module_key}/actions/{action}` | R3 | 主 §9.1 |
| 6 | POST | `/v1/tasks/{id}/modules/{module_key}/reassign` | R3 | 主 §9.1 |
| 7 | POST | `/v1/tasks/{id}/modules/{module_key}/pool-reassign` | R3 | 主 §9.1 |
| 8 | POST | `/v1/tasks/{id}/cancel` | R3 | 主 §9.1.1 |
| 9 | POST | `/v1/tasks`（保留，body 新增 `source_draft_id` / `batch_sku_mode=multiple`） | R3 | 主 §9.1 + IA §3.5.9 |
| **草稿（v1 必做）** |
| 10 | POST | `/v1/task-drafts` | R4-SA-C | IA §3.5.9 |
| 11 | GET | `/v1/me/task-drafts` | R4-SA-C | IA §3.5.9 |
| 12 | GET | `/v1/task-drafts/{draft_id}` | R4-SA-C | IA §3.5.9 |
| 13 | DELETE | `/v1/task-drafts/{draft_id}` | R4-SA-C | IA §3.5.9 |
| **批量 SKU Excel** |
| 14 | GET | `/v1/tasks/batch-create/template.xlsx` | R5 | IA §3.5.4 |
| 15 | POST | `/v1/tasks/batch-create/parse-excel` | R5 | IA §3.5.4 |
| **资产管理中心** |
| 16 | GET | `/v1/assets/search` | R4-SA-A | 资产 §5 |
| 17 | GET | `/v1/assets/{asset_id}` | R4-SA-A | 资产 §5 |
| 18 | GET | `/v1/assets/{asset_id}/download` | R4-SA-A | 资产 §5 |
| 19 | GET | `/v1/assets/{asset_id}/versions/{version_id}/download` | R4-SA-A | 资产 §4 |
| 20 | POST | `/v1/assets/{asset_id}/archive` | R4-SA-A | 资产 §5 |
| 21 | POST | `/v1/assets/{asset_id}/restore` | R4-SA-A | 资产 §5 |
| 22 | DELETE | `/v1/assets/{asset_id}` | R4-SA-A | 资产 §5（SuperAdmin） |
| **ERP 编码查询（定制必须）** |
| 23 | GET | `/v1/erp/products/by-code` | R4-SA-C | 定制 §3.1.2 |
| **设计源文件查询（定制必须）** |
| 24 | GET | `/v1/design-sources/search` | R4-SA-C | 定制 §3.2.2 |
| **全局搜索** |
| 25 | GET | `/v1/search` | R4-SA-D | IA §4 |
| **通知中心** |
| 26 | GET | `/v1/me/notifications` | R4-SA-C | IA §8.3 |
| 27 | POST | `/v1/me/notifications/{id}/read` | R4-SA-C | IA §8.3 |
| 28 | POST | `/v1/me/notifications/read-all` | R4-SA-C | IA §8.3 |
| 29 | GET | `/v1/me/notifications/unread-count` | R4-SA-C | IA §8.3 |
| **个人中心** |
| 30 | GET | `/v1/me` | R4-SA-B | IA §7.2 |
| 31 | PATCH | `/v1/me` | R4-SA-B | IA §7.2 |
| 32 | POST | `/v1/me/change-password` | R4-SA-B | IA §7.2 |
| 33 | GET | `/v1/me/org` | R4-SA-B | IA §7.2 |
| **组织（用户 / 部门 / 组 / 跨部门调配）** |
| 34 | GET | `/v1/users`（筛选扩展） | R4-SA-B | IA §5.1 |
| 35 | POST | `/v1/users` | R4-SA-B | IA §5.1 |
| 36 | PATCH | `/v1/users/{id}` | R4-SA-B | IA §5.1 |
| 37 | DELETE | `/v1/users/{id}` | R4-SA-B | IA §5.1 |
| 38 | POST | `/v1/users/{id}/activate` | R4-SA-B | IA §5.4 |
| 39 | POST | `/v1/users/{id}/deactivate` | R4-SA-B | IA §5.4 |
| 40 | POST | `/v1/departments/{id}/org-move-requests` | R4-SA-B | IA §5.2 |
| 41 | GET | `/v1/org-move-requests` | R4-SA-B | IA §5.2 |
| 42 | POST | `/v1/org-move-requests/{id}/approve` | R4-SA-B | IA §5.2 |
| 43 | POST | `/v1/org-move-requests/{id}/reject` | R4-SA-B | IA §5.2 |
| **报表 L1（SuperAdmin）** |
| 44 | GET | `/v1/reports/l1/cards` | R4-SA-D | IA §6 |
| 45 | GET | `/v1/reports/l1/throughput` | R4-SA-D | IA §6 |
| 46 | GET | `/v1/reports/l1/module-dwell` | R4-SA-D | IA §6 |
| **WebSocket** |
| 47 | GET | `/ws/v1`（升级握手） | R4-SA-C | IA §9 |

> **硬约束**：上表 47 条路径是本轮**必须全部落 OpenAPI 的目标**，一条都不能少。handler 可以全部 501，但路由必须登记、schema 必须写完整。

### 1.2 OpenAPI 必须定义的 Schema 清单（新增）

以下 schemas 必须全部新增到 `components/schemas/`，字段口径以四份权威文档为唯一真源（每个字段旁注"源 §x.x"即可）：

- `TaskDetail`（主 §9.2）
- `TaskModule` / `TaskModuleState` / `TaskModuleScope` / `TaskModuleProjection`
- `TaskModuleAction`（枚举：claim / submit / approve / reject / reassign / pool_reassign / asset_upload_session_create / update_reference_files / update_basic_info / update_deadline / update_priority / close_task / cancel_task）
- `TaskCancelRequest`（`{ reason, force? }`，主 §9.1.1）
- `TaskPriority`（枚举：`normal / urgent / critical`）
- `DerivedStatus`（枚举见主 §10.1）
- `DenyCode`（枚举见主 §6.2）
- `TaskDraft` / `TaskDraftPayload`
- `BatchCreateParseResult`（含 `preview[]` / `violations[]`，IA §3.5.4）
- `Asset`（扩展 `source_module_key`、`lifecycle_state`，资产 §2 / §7）
- `AssetLifecycleState`（`active / closed_retained / archived / auto_cleaned / deleted`，资产 §7）
- `ReferenceFileRef`（扩展 `owner_module_key`，资产 §3）
- `ERPProductSnapshot`（定制 §3.1.2）
- `DesignSourceEntry`（定制 §3.2.2）
- `SearchResultGroup`（IA §4）
- `Notification` / `NotificationType`（IA §8.1；`task_mentioned` **不在 v1 枚举**，见 IA §8.1 v3 注）
- `OrgMoveRequest` / `OrgMoveRequestState`
- `Actor`（含 `frontend_actions` / `managed_departments` / `managed_teams`，主 §5.2）
- `L1Card`（IA §6）

### 1.3 必须登记的 deny_code（错误响应统一）

所有新路由的 4xx 响应必须使用 `ErrorResponse` 统一 schema，`error.code` 枚举至少覆盖：

```
task_not_found
module_not_instantiated
module_out_of_scope
module_state_mismatch
module_action_role_denied
module_claim_conflict
module_blueprint_missing_team
task_already_claimed        # §9.1.1 的 409
asset_gone                  # 410，资产 §7.4
draft_not_found
draft_quota_exceeded        # 同用户同 task_type 超 20
org_move_requires_superadmin
ws_upgrade_failed
```

### 1.4 `transport/http.go` 路由注册

- 按 1.1 列的顺序注册 47 条路由
- Handler 统一返回：
  ```go
  http.Error(w, `{"error":{"code":"not_implemented","message":"reserved for R{owner_round}"}}`, http.StatusNotImplemented)
  ```
  （`owner_round` 查 1.1 表的 `x-owner-round` 列；本轮严禁填真实逻辑）
- 保留现有旧路由，**不动** v0.9 / v1.0 已上线路由
- 新路由必须登记到 `transport/route_access_catalog.go`（用于菜单可见性；具体 `allowed_roles` 按 IA §2.2 + §5 填入）

### 1.5 测试

本轮 `transport/http_test.go` 追加：

1. `TestV1R1_RouteRegistered_All47Paths` — 对 1.1 的 47 条 path 各发一次请求（带合法 auth stub），断言 **状态码为 501**、**响应 body 含 `not_implemented`、含正确的 `owner_round` 字符串**。
2. `TestV1R1_RouteAccessCatalog_Shape` — 断言 `route_access_catalog.go` 已登记 47 条新路由，并按 `allowed_roles` 筛选后菜单符合 IA §2.2 的矩阵（例如 `/v1/reports/*` 仅 SuperAdmin 可见）。
3. `TestV1R1_OpenAPI_Lint` — 跑 `openapi-cli lint docs/api/openapi.yaml`，断言 0 error 0 warning（如当前仓库尚无此工具，使用 `go test` 调用 `openapi3.NewLoader().LoadFromFile` 做结构合法性校验即可）。

---

## 2. 严禁触碰（DO NOT TOUCH）

| 路径 / 概念 | 原因 |
| --- | --- |
| `domain/**` | R2/R3 职责 |
| `repo/**` | R2/R3 职责 |
| `service/**` | R3/R4 职责 |
| `db/migrations/**` | R2 独占（迁移号段不能并发） |
| `config/frontend_access.json` | R3/R5 职责（菜单 actions 字段瘦身） |
| v0.9 已有路由的响应结构 | 兼容期不动 |
| 任何 `task_action_rules.go` / `task_action_authorizer.go` / `data_scope.go` 的业务逻辑 | R3 职责 |
| 前端任何文件（`frontend/` 或 Vue 工程） | R5 职责 |

---

## 3. 验收脚本（Codex 必须自跑并贴出）

```bash
# 1. 编译
go build ./...

# 2. 全量单测（至少保持当前绿灯）
go test ./... -count=1

# 3. 本轮新增测试单独跑一次，明确 PASS
go test ./transport/... -run "V1R1" -v -count=1

# 4. 手工校验 OpenAPI 可加载
go run ./cmd/tools/openapi-validate docs/api/openapi.yaml  # 若无此命令，则用任一 OpenAPI 3.0 loader 验证

# 5. 路径清点脚本（Codex 内嵌执行）
grep -E "^\s{2}/v1/|^\s{2}/ws/v1" docs/api/openapi.yaml | wc -l
# 期望值 = 原有 count + 47（新增条数；若 OpenAPI 原已有部分路径如 /v1/users，按"新增路径"实际条数核对，报告中列清单）
```

所有 5 项必须 PASS。若有 1 项 FAIL，Codex 必须自行定位并修正，不得交付半成品。

---

## 4. 产出物（Codex 提交 PR 时必须包含）

- `docs/api/openapi.yaml` 改动
- `transport/http.go` 改动
- `transport/route_access_catalog.go` 改动
- `transport/http_test.go` 改动 / 新增
- `docs/iterations/V1_R1_REPORT.md` — 本轮变更摘要（≤ 200 行）：
  - 新增路径清单（带 `x-owner-round` 列）
  - 新增 schemas 清单
  - OpenAPI 行数 before / after
  - 测试运行结果截图文字版
  - 遗留 / 冲突点（如有）

---

## 5. 回滚策略

本轮因不动 domain / repo / service / migration，**零数据风险**。如需回滚：`git revert <本轮 commit>` 即可，对线上运行无任何影响。

---

## 6. 与后续轮次的交接点

- **R2** 将使用本轮 `components/schemas/TaskModule / TaskModuleEvent / TaskDraft / Notification / OrgMoveRequest / Asset.lifecycle_state` 等 schema 反推数据库表字段；R2 不得修改 schema，**只能在 schema 未覆盖的字段上做内部扩展**。
- **R3** 将基于 1.1 表中 `x-owner-round: R3` 的 9 条路由填入真实逻辑；handler 内部可调整错误 code，但 **不得改 path / method / 请求体结构**。
- **R4 的 4 个 subagent** 基于 `x-owner-round` 的 `R4-SA-A/B/C/D` 四个 bucket 各自认领，**严禁跨 bucket 修改**。
- **R5** 基于本轮 OpenAPI 生成前端 TypeScript 类型；后端**不得**在 R5 再追加任何新路径（如需追加，必须先回 R1 改 OpenAPI 再改前端，否则拒收）。
- **R6** 基于所有 `x-owner-round` 回顾是否全部 handler 已落地；如有剩余 501，必须在 R6 内处理或明确推迟到 v1.1。

---

## 附录 A · 如需人工 review 的 4 个决策点

Codex 如遇下列情况应**停下并在 PR 评论里标记问题**，不得自作主张：

1. 某条路径在现有 OpenAPI 已部分存在（如 `/v1/users`），是"扩展"还是"替换"请留 `# TODO(R1-ambiguity): ...` 注释并在 V1_R1_REPORT.md 中列出，由人工裁决。
2. `ErrorResponse` schema 若与现有 v0.9 错误响应形状不兼容，按"新路由用新 schema，旧路由保持"处理，不要做全局替换。
3. 若 `transport/route_access_catalog.go` 的现有登记格式与 IA §2.2 的矩阵无法 1:1 映射，保留现有格式，**用注释补齐新路由的角色要求**，等 R4-SA-B 落地组织菜单时再重构。
4. 若 `openapi-validate` 报错源于**现有** schema（非本轮新增），不修，只在报告中标记"preexisting lint error"。

---

## 附录 B · 命名约定

- `x-owner-round`：OpenAPI 扩展字段，取值 `R1 / R2 / R3 / R4-SA-A / R4-SA-B / R4-SA-C / R4-SA-D / R5 / R6`
- operationId 统一：`<resource>_<action>`，小驼峰；例：`taskModuleClaim`、`taskCancel`、`assetArchive`
- 所有新 schema 前缀 `V1` 可选（仅当与现有同名 schema 冲突时使用，如 `V1TaskDetail`）

---

## 附录 C · Codex 执行清单（按顺序打钩）

- [ ] 读完 §0 的 11 份上下文
- [ ] 在 `docs/api/openapi.yaml` 新增 47 条路径 + 所有 §1.2 schemas
- [ ] 在 `transport/http.go` 注册 47 条路由，全部 501
- [ ] 在 `transport/route_access_catalog.go` 登记 47 条新路由 + 角色矩阵
- [ ] `transport/http_test.go` 新增 3 个测试
- [ ] 跑 §3 五步验收全绿
- [ ] 产出 `docs/iterations/V1_R1_REPORT.md`
- [ ] 提交 PR，标题格式：`[V1 R1] Contract freeze: 47 routes + schemas (501 stubs)`
- [ ] 回到本文件 §6 核对"与后续轮次的交接点"是否都对得上

---

**签字 / 版本**
- 起草：架构组 · 2026-04-17
- 对齐源：主 v4 / IA v3 / 资产 v1 / 定制 v2（均已签字）
- 本 prompt 修改历史：
  | 版本 | 日期 | 变更 |
  | --- | --- | --- |
  | v1 | 2026-04-17 | 初稿，随 R1 启动 |
