# MODEL_v0.4_memory.md — Model Handover Memory for v0.4

# ARCHIVE ONLY
#
# NOT SOURCE OF TRUTH
# DO NOT USE FOR NEW INTEGRATION OR CURRENT SPEC DECISIONS
# SEE `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
# SEE `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`

**Last updated**: 2026-03-18
**Purpose**: Prevent the next model from losing development progress and engineering memory.

## 纠偏说明（ITERATION_083，2026-03-18）

以下历史表述已由架构法医审计纠正，以 `docs/TRUTH_SOURCE_ALIGNMENT.md` 为准：

- **categories**：31 行为开发样例，**不是**生产真实分类中心；业务分类主语义 = **款式编码（i_id）**。
- **products**：副本/缓存/承接表，**不是**商品搜索唯一真相源。
- **jst_inventory**：同步驻留层，**不是**前台搜索主表。
- **OpenWeb 主链**：设计目标为主链；live 是否已切需按 `ERP_REAL_LINK_VERIFICATION` 验收确认；Code Implemented ≠ Live Effective。

## 0. ITERATION_079 关键更新（JST 用户同步预埋）

### confirmed_facts
- Bridge 增加 JST getcompanyusers 适配：`/open/webapi/userapi/company/getcompanyusers`
- MAIN 通过 Bridge `GET /v1/erp/users` 获取 JST 用户，MAIN 不直接调 OpenWeb
- Admin 接口：`GET /v1/admin/jst-users`、`POST /v1/admin/jst-users/import-preview`、`POST /v1/admin/jst-users/import`
- 本地 users 表扩展：`jst_u_id`、`jst_raw_snapshot_json`（migration 038）
- 导入策略：jst_u_id > loginId(若存在) > username；新建用户 disabled + 随机密码
- 角色映射默认关闭，`write_roles=true` 时可选写入 user_roles
- JST 仅作数据源，不接管鉴权

### inferences
- 真实 JST 环境若未返回 loginId，使用 jst_u_id 或 jst_{u_id} 作为 username
- 真实验收需 JST 凭证与真实环境

### next_best_actions
1. 运行 migration 038 后部署
2. 有 JST 凭证时验证 `GET /v1/erp/users` 真实拉取
3. 验证 import-preview 与 import（dry_run）流程

## 1. ITERATION_078 关键更新（语义收正 + 新路由上线 + 11仓验收）

### confirmed_facts
- 已完成 `v0.4` 原版本覆盖部署（不升版本号），并通过三服务巡检：
  - 8080 pid `3546054` health `200`
  - 8081 pid `3546082` health `200`
  - 8082 pid `3546261` health `200`
  - `/proc/<pid>/exe` 均非 deleted
- 语义口径在 Bridge 返回契约中已实测对齐：
  - `sku_id` = 聚水潭商品唯一编码
  - `i_id` = 聚水潭款式编码（分类维度）
  - `name` = 商品名称
  - `short_name` = 商品简称
  - `wms_co_id` = 仓库维度
- 新路由已 live 生效并完成权限回归：
  - `POST /v1/erp/products/style/update`：Admin/Ops=`200`，roleless=`403`
  - `GET /v1/erp/warehouses`：Admin/Ops=`200`，roleless=`403`
- upsert/style 写回结果中已可见路径分流：
  - upsert 返回 `route=itemskubatchupload`
  - style-update 返回 `route=itemupload`
- 11 仓口径已在 `GET /v1/erp/warehouses` 返回完整落地（11 条 `wms_co_id`）。
- `shelve/unshelve/virtual_qty` 当前 live 行为仍为 hybrid fallback 稳态：
  - 返回 `message=stored locally` + `sync_log_id`
  - sync log 请求体已带 `wms_co_id`，并承接 `bin_id/carry_id/box_no`
- 权限实测保持正确：
  - Admin/Ops 对 ERP 读写路由 `200`
  - roleless 对 ERP 读写路由 `403`
- short_name 模板能力已在后端就位：
  - 规则文件：`config/erp_short_name_rules.json`
  - 生成逻辑：`service/erp_short_name_template.go`
  - 已接入 upsert 与 style-update 归一化流程

### inferences
- Bridge 已具备 MAIN 后续 ERP 扩展所需的关键契约面（语义字段、双写路径、仓库维度、模板化简称）。
- 目前 remaining 远端收敛点集中在上游业务上下文（仓位/箱号/有效库存数据）而非路由或签名连通性问题。
- 继续保持 live `hybrid + fallback` 是当前最安全策略。

### next_best_actions
1. 用上游确认过的有效仓位/箱号上下文再次复测 shelve/unshelve，争取 remote 成功回包。
2. 按官方可接受最小字段组合复测 virtual_qty，争取从 fallback 收敛到 remote success。
3. 在 MAIN 侧逐步接入新增字段（优先 `i_id/short_name/wms_co_id/s_price`），并保持 Bridge 契约不回退。

## 1. ITERATION_077 关键更新（剩余三写接口收敛）

### confirmed_facts
- 8081 Bridge 已接入 OpenWeb 签名规则（复用 8082 已验证口径）：
  - `sign = md5(app_secret + 按 key 升序拼接 key+value)`（小写十六进制）
  - 参与签名参数：`app_key/access_token/timestamp/charset/version/biz`（不含 `sign`）
  - `timestamp = Unix 秒`
- 8081 写接口到官方 OpenWeb 映射已落地并继续生效：
  - `POST /v1/erp/products/upsert` -> `/open/webapi/itemapi/itemsku/itemskubatchupload`
  - `POST /v1/erp/products/shelve/batch` -> `/open/webapi/wmsapi/openshelve/skubatchshelve`
  - `POST /v1/erp/products/unshelve/batch` -> `/open/webapi/wmsapi/openoffshelve/skubatchoffshelve`
  - `POST /v1/erp/inventory/virtual-qty` -> `/open/webapi/itemapi/iteminventory/batchupdatewmsvirtualqtys`
- live bridge 模式保持 `hybrid`：
  - `ERP_REMOTE_MODE=hybrid`
  - `ERP_REMOTE_BASE_URL=https://openapi.jushuitan.com`
  - `ERP_REMOTE_AUTH_MODE=openweb`
  - `ERP_REMOTE_FALLBACK_LOCAL_ON_ERROR=true`
- upsert 已完成真实 remote 命中证据：
  - bridge 日志出现 `remote_erp_openweb_request_completed`
  - URL 命中官方 `https://openapi.jushuitan.com/open/webapi/itemapi/itemsku/itemskubatchupload`
  - `status_code=200`
- 三条剩余写接口已完成“真实上游回包 + fallback”证据闭环：
  - `shelve_batch`：
    - 官方 URL：`/open/webapi/wmsapi/openshelve/skubatchshelve`
    - 上游回包：`code=100`, `msg=上架仓位不能为空`
    - Bridge 行为：`erp_remote_shelve_batch_failed_fallback_local` -> `erp_remote_shelve_batch_fallback_local_success`
  - `unshelve_batch`：
    - 官方 URL：`/open/webapi/wmsapi/openoffshelve/skubatchoffshelve`
    - 上游回包：`code=100`, `msg=指定箱不存在`
    - Bridge 行为：`erp_remote_unshelve_batch_failed_fallback_local` -> `erp_remote_unshelve_batch_fallback_local_success`
  - `virtual_qty`：
    - 官方 URL：`/open/webapi/itemapi/iteminventory/batchupdatewmsvirtualqtys`
    - 上游原始回包：`code=0`, `msg=未获取到有效的传入数据`, `data=null`
    - Bridge 新增业务校验后按 remote reject 处理并 fallback：
      - `erp_remote_virtual_inventory_failed_fallback_local` -> `erp_remote_virtual_inventory_fallback_local_success`
- 8081 OpenWeb 请求可观测性已增强：
  - 新增 `remote_erp_openweb_request_started` 日志点
- 权限回归实证（remote/hybrid 改造后）：
  - Admin: 8081 读写 `200`
  - Ops: 8081 读写 `200`
  - Roleless: 8081 读写 `403`
- 三服务状态保持：
  - 8080 pid `3527927` / health ok
  - 8081 pid `3527959` / health ok
  - 8082 pid `3528131` / health ok
  - `/proc/<pid>/exe` 均非 deleted（以三服务巡检输出为准）

### inferences
- OpenWeb upsert 链路已真实打通并可稳定命中官方接口；“remote 正式可达”已被事实验证。
- Shelve/unshelve/virtual-qty 当前阻塞点已收敛到上游业务约束（仓位/箱/有效数据），不是“无 remote 能力”。
- 在上游约束未解除前，`hybrid + fallback` 仍是当前 live 最稳妥边界；不应切纯 `remote`。

### next_best_actions
1. 对接方需补齐/确认 `skubatchshelve` 所需仓位上下文（仓位字段与可用仓位主数据），否则会持续返回 `上架仓位不能为空`。
2. 对接方需补齐/确认 `skubatchoffshelve` 所需箱号/容器上下文（可识别箱），否则会持续返回 `指定箱不存在`。
3. 对接方需给出 `batchupdatewmsvirtualqtys` 可接受的最小必填字段组合与样例（当前上游回包为“未获取到有效的传入数据”）。
4. 维持 live `hybrid`，每次上游配置变更后复测三接口并记录官方回包 code/msg，再决定是否扩大 remote 成功范围。

> 注：下方“Current v0.4 System State”含历史阶段信息，若与本页顶部冲突，以 ITERATION_077 段落为当前事实。

## 1. Current v0.4 System State

### Live Deployment
- Server: `223.4.249.11:8080` (MAIN service)
- PID: 3507849 (updated 2026-03-17)
- Binary: `/root/ecommerce_ai/releases/v0.4/ecommerce-api`
- Health: `GET /v1/auth/me` returns 401 (correct — auth required)
- Database: MySQL at `223.4.249.11:3306`, database `jst_erp`
- TZ: `Asia/Shanghai` (set via `TZ` env variable, propagated to MySQL DSN `loc`)
- ERP Bridge at port 8081: **RUNNING** (PID 3507876, updated 2026-03-17)
- Bridge binary: `/root/ecommerce_ai/releases/v0.4/erp_bridge` (same source as MAIN, different port)
- JST sync at port 8082: **RUNNING** (PID 3508034)
- `/proc/<pid>/exe` for both MAIN and Bridge: points to v0.4 binaries, NOT deleted

### MAIN(8080) ERPSyncWorker 当前真实状态（2026-03-17）
- **旧 10 分钟同步历史归属**: 已确认旧 `*/10 * * * * -> syncSvc.IncrementalSync(10)` 来自废弃入口 `cmd/api/main.go`，不是当前 live 运行源
- **当前 live 同步 owner**: MAIN(8080) `ERPSyncWorker` + `/v1/products/sync/*`
- **当前 live 节奏**: `/v1/products/sync/status` 证实 `scheduler_enabled=true`, `interval_seconds=300`，即 5 分钟，不是旧 10 分钟
- **当前 source_mode 真实含义**: `stub` 表示 MAIN sync 数据源是本地 stub JSON，而不是 Bridge query、也不是外部 ERP 正式接口
- **故障前 live 真实表现**:
  - latest scheduled run = `noop`
  - manual run = `noop`
  - 原因不是 worker 死亡，而是稳定空跑
- **已确认根因**:
  - `/proc/3416820/cwd = /root`
  - `ERP_SYNC_STUB_FILE=config/erp_products_stub.json` 使用相对路径
  - 实际 stub 文件位于 release 目录 `.../releases/v0.4/config/erp_products_stub.json`
  - 因此 live 进程把 stub 路径解析到不存在的 `/root/config/...`，命中 `os.ErrNotExist`，服务按设计记录 `noop`
- **恢复方式**: 修复 `deploy/run-with-env.sh`，让进程在实际二进制目录下启动，保证 release 包内相对配置路径可用
- **状态接口增强**: `ERPSyncStatus` 新增 `resolved_stub_file` 和 `stub_file_exists`，后续可直接看出 worker 实际读取路径
- **恢复后 live 结果**:
  - 新 PID: `3450797`
  - `/proc/3450797/exe -> /root/ecommerce_ai/releases/v0.4/ecommerce-api`
  - `/proc/3450797/cwd -> /root/ecommerce_ai/releases/v0.4`
  - manual `/v1/products/sync/run` => `success`, `total_received=2`, `total_upserted=2`
  - 首次 post-deploy scheduled run => `success`, `started_at=2026-03-17T10:58:38+08:00`, `total_upserted=2`
  - `/v1/products/search?keyword=ERP Stub` 与 `/v1/erp/products?q=ERP Stub` 都返回两条 stub 商品
- **当前真实职责**:
  - MAIN 定时/手动同步本地 `products` 缓存
  - 记录 `erp_sync_runs`
  - 为 `/v1/products/search`、`/v1/products/{id}` 提供本地缓存基座
  - 在 Bridge live `local` 模式下，也间接成为 Bridge 本地查询数据基座
- **不是当前职责**:
  - 不是外部 ERP 正式直连同步
  - 不是 Bridge 主执行器
  - 不是云端 Bridge 主源码改造点

### Bridge(8081) Remote ERP 接入状态（2026-03-17）
- **当前 live 运行模式**: `ERP_REMOTE_MODE=local`（`/root/ecommerce_ai/shared/bridge.env`）
- **localERPBridgeClient 作用**: 继续作为默认写回实现，`POST /v1/erp/products/upsert` 返回 `status=accepted, message=stored locally`，数据落本地 MySQL
- **remote ERP client 作用**: 新增 `service/erp_bridge_remote_client.go`，用于在 `remote/hybrid` 模式下将 upsert 请求发往外部 ERP API（可配置 baseURL/path/auth/sign/retry/timeout）
- **切换机制**:
  - `local`: Bridge 全量走本地 client（安全默认）
  - `remote`: Bridge upsert 走外部 ERP client（search/detail/category 在当前实现下不走外部）
  - `hybrid`: upsert 优先走外部，失败时按 `ERP_REMOTE_FALLBACK_LOCAL_ON_ERROR` 回落本地
- **MAIN -> Bridge -> ERP 当前真实链路**: MAIN(8080) 继续通过 `ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081` 调 Bridge；Bridge live 仍是 local 模式，因此当前真实落点是本地 MySQL，而非外部 ERP
- **外部 ERP 正式接入进度**: 代码与配置入口已就绪（remote client + 模式开关 + 部署验证）；尚未获得“外部 ERP 正式响应”证据，不能宣告已接通
- **未完成/阻塞项**:
  1) 缺外部 ERP 正式 `base_url + path + 鉴权签名规则` 的最终确认
  2) 缺可用正式凭证（app_key/app_secret/token）与白名单放通结果
  3) 未完成 Bridge->外部 ERP 的线上真实 upsert 回执验证
- **下一模型推荐阅读顺序**:
  1) `docs/iterations/ITERATION_072.md`
  2) `deploy/run-with-env.sh`
  3) `service/erp_sync_service.go`
  4) `workers/erp_sync_worker.go`
  5) `service/erp_bridge_local_client.go`
  6) `service/erp_bridge_service.go`
  7) `cmd/server/main.go`
  8) `config/config.go` + `deploy/bridge.env.example`
  9) `docs/iterations/ITERATION_071.md`
  10) `CURRENT_STATE.md` 最新顶部状态

### `/v1/erp/*` 权限分级验证与修复（2026-03-17）
- **双账号真实对照已完成**:
  - `Admin` 账号：`/v1/auth/me` 返回 `roles=["Admin"]`
  - `Ops` 账号：`/v1/auth/me` 返回 `roles=["Ops"]`
  - 两者在 `8081` 上均通过以下路由（均 `200`）:
    - `GET /v1/erp/products`
    - `GET /v1/erp/products/{id}`
    - `GET /v1/erp/categories`
    - `POST /v1/erp/products/upsert`
    - `GET /v1/erp/sync-logs`
    - `GET /v1/erp/sync-logs/{id}`
    - `POST /v1/erp/products/shelve/batch`
    - `POST /v1/erp/products/unshelve/batch`
    - `POST /v1/erp/inventory/virtual-qty`
- **修复前风险已被实证**:
  - roleless（无业务角色）有效会话也可 `200` 访问读写 ERP 路由，权限边界过宽。
- **最小修复已上线**（仅改 `transport/http.go`）:
  - 读路由：`Ops/Designer/Audit_A/Audit_B/Warehouse/Outsource/ERP/Admin`
  - 写路由：`Ops/Warehouse/ERP/Admin`
  - sync-log 路由：`Ops/Warehouse/ERP/Admin`
- **修复后实证**:
  - roleless 会话对 `GET /v1/erp/products`、`GET /v1/erp/categories`、`POST /v1/erp/products/upsert` 均返回 `403`。
- **当前权限策略结论**:
  - 不再是“任意有效会话可访问”；
  - 已收敛为“需要会话 + 满足路由角色集”。

### 8081 remote 正式验收最新状态（2026-03-17）
- **验收尝试状态**: 已做 live 参数/日志/链路核查，但未达到可切 `hybrid/remote` 的前置条件。
- **当前 live 参数事实**:
  - `ERP_REMOTE_MODE=local`
  - `ERP_REMOTE_BASE_URL` 为空
  - `ERP_REMOTE_AUTH_MODE=none`
  - `ERP_REMOTE_AUTH_HEADER_TOKEN/APP_KEY/APP_SECRET/ACCESS_TOKEN` 均为空
  - timeout/retry/fallback 参数存在且生效
- **真实外部 ERP 正式写回**: 仍未完成（无外部正式回包证据）。
- **最后阻塞项清单**:
  1) 外部 ERP 正式 `base_url` 未就绪
  2) 远端鉴权凭证（token/app_key/app_secret/access_token）未就绪
  3) 外部签名/时间戳/nonce 规则与验收样例未最终落地
  4) 白名单/外联条件在本轮无可验证放通证据
- **推荐下一步动作**:
  1) 先补齐 `base_url + auth/sign` 正式参数与白名单
  2) 优先 `hybrid` 验收，强制保留 fallback
  3) 捕获“remote 命中 + 上游回包 + fallback 分支”三类日志证据
  4) 仅在 remote 正式回包成立后再宣告写回打通

### MAIN / Bridge / ERP 推荐未来关系（文字图）
- **当前 live**:
  - `MAIN ERPSyncWorker -> local stub file -> MAIN products`
  - `MAIN /v1/erp/* facade -> Bridge(8081 local mode) -> shared local products/category data`
  - `MAIN business-info filing -> Bridge upsert -> local DB`
- **推荐未来**:
  - `MAIN ERPSyncWorker -> Bridge query/export contract -> MAIN products + erp_sync_runs`
  - `Bridge -> external ERP` 负责 adapter/query/mutation 语义
  - `MAIN` 保持 task/business/cache/scheduler owner，不直接耦合外部 ERP 适配细节

### Auth System
- Session-based authentication with bearer tokens
- Roles are case-sensitive: `Ops`, `Designer`, `Audit_A`, `Audit_B`, `Warehouse`, `Outsource`, `Admin`, `DepartmentAdmin`, `ERP`
- Admin user: `admin` / `<ADMIN_PASSWORD>` (from `auth_identity.json` config)
- Routes with `APIReadinessReadyForFrontend` require session tokens (debug headers rejected)
- Routes with `APIReadinessInternalPlaceholder` accept debug headers (X-User-ID + X-User-Roles)

## 2. Main Flows Verified as Working (Blackbox Tested 2026-03-16)

### Original Product Development (existing_product source mode)
```
Create task (with product_selection.erp_product)
  -> Assign designer
  -> Submit design (asset_type=delivery, file_name required)
  -> Audit claim (stage=A)
  -> Audit approve (stage=A, next_status=PendingWarehouseReceive)
  -> Update business info (category_code, spec_text, cost_price, filed_at)
  -> Warehouse receive
  -> Warehouse complete
  -> Close task
  -> Status: Completed
```

### New Product Development (new_product source mode)
```
Create task (product_name, product_short_name, category_code, material_mode=preset|other, material)
  -> Same design/audit/warehouse/close flow as above
  -> Status: Completed
```

### Purchase Task (new_product source mode)
```
Create task (cost_price_mode=manual, cost_price, quantity, base_sale_price, purchase_sku, product_channel)
  -> Update business info (category, spec, cost, filed_at)
  -> Update procurement (status=draft, procurement_price, quantity, supplier_name)
  -> Advance procurement: prepare -> start -> complete
  -> Prepare warehouse
  -> Warehouse receive -> complete
  -> Close task
  -> Status: Completed
```

## 3. Task Creation Rules Summary

All three types require: `task_type`, `owner_team`, `deadline_at` (or `due_at`), `creator_id`

### original_product_development
- Source mode: `existing_product` (auto-inferred)
- Requires: `product_selection.erp_product.product_id`
- Optional: `designer_id`, `priority`, `demand_text`, `reference_images`, `remark`

### new_product_development
- Source mode: `new_product` (auto-inferred)
- Requires: `product_name`, `category_code`, `material_mode` (`preset` or `other`), `material` (when `material_mode=other`)
- Optional: `product_short_name`, `designer_id`, `priority`, `demand_text`, `remark`

### purchase_task
- Source mode: `new_product` (auto-inferred)
- Requires: `cost_price_mode` (`manual` or `template`), `cost_price` (when manual), `quantity`
- Optional: `purchase_sku`, `product_channel`, `base_sale_price`, `product_name`, `remark`

Full spec: `docs/TASK_CREATE_RULES.md`

## 4. ERP Search & Binding State

- `GET /v1/erp/products` — keyword search, pagination (local DB, not external ERP)
- `GET /v1/erp/products/{id}` — detail with fallback search
- `GET /v1/erp/categories` — list active categories from DB
- `POST /v1/erp/products/upsert` — ERP product writeback (added 2026-03-17, was missing and caused 404)
- ERP Bridge (port 8081): **DEPLOYED AND RUNNING** (restored 2026-03-17)
- Local ERP Bridge Client: used when `SERVER_PORT == ERP_BRIDGE_PORT` (same-process mode); Bridge on 8081 uses local client to DB
- Current production: MAIN at 8080, Bridge at 8081 (different ports, MAIN uses HTTP client to call Bridge)
- ERP filing via `PATCH /v1/tasks/:id/business-info` with `filed_at`: non-blocking fallback retained as safety net; normal path now succeeds (Bridge online)
- MAIN -> Bridge call chain verified: MAIN sends `POST http://127.0.0.1:8081/v1/erp/products/upsert` with bearer token, Bridge returns 200 with `{"status":"accepted","message":"stored locally"}`

## 5. Asset & Upload State

- Mock upload: `POST /v1/tasks/:id/assets/mock-upload` (MockPlaceholderOnly readiness)
- Real upload sessions: `POST /v1/tasks/:id/assets/upload-sessions` (ReadyForFrontend)
- Submit design: `POST /v1/tasks/:id/submit-design` — creates asset AND advances status when `asset_type=delivery`
- Asset types: `source`, `delivery`, `reference`, `preview`
- Access policy: LAN/Tailscale/public URLs populated based on `AssetAccessPolicy` config
- Actual file storage: NOT implemented in v0.4 (mock/placeholder only)

## 6. Audit & Warehouse State

### Audit
- Stage A: claim -> approve/reject
- Stage B: claim -> approve/reject (optional, can skip A->PendingWarehouseReceive)
- Valid approve transitions:
  - `PendingAuditA` -> `PendingAuditB` | `PendingWarehouseReceive` | `PendingOutsource`
  - `PendingAuditB` -> `PendingWarehouseReceive`
  - `PendingOutsourceReview` -> `PendingWarehouseReceive`

### Warehouse
- `POST /:id/warehouse/prepare` — requires business info (filed_at, category, spec, cost) + final delivery asset (for design tasks) or completed procurement (for purchase tasks)
- `POST /:id/warehouse/receive` — warehouse staff acknowledges receipt
- `POST /:id/warehouse/complete` — marks warehouse done, status -> PendingClose

### Close
- `POST /:id/close` — requires PendingClose status + all business info fields set

## 7. Known Remaining Issues / Gaps

1. ~~**ERP Bridge service not deployed**~~ **RESOLVED 2026-03-17** — Bridge now running on 8081; MAIN -> Bridge -> DB writeback verified
2. **Real file upload/storage not implemented** — mock upload only; NAS agent integration pending
3. **Org hierarchy / SSO** — not implemented
4. **KPI / BI** — not implemented
5. **Frontend** — not part of this backend repo; frontend display issues (timestamps, etc.) to be handled separately
6. **Events endpoint** — events are recorded but `GET /v1/tasks/:id/events` response structure may need verification for frontend consumption
7. **Outsource flow** — code-complete but not blackbox-tested in this round (design tasks skip outsource in typical flow)
8. **Export center** — infrastructure scaffolded but not production-ready
9. **External ERP writeback** — Bridge currently stores locally to DB via `localERPBridgeClient`; actual external ERP/JST API integration not yet wired (Bridge acts as DB adapter only)

## 8. Fixes Applied in This Round

| Issue | Resolution |
|---|---|
| ERP filing blocking task close | Made `performERPBridgeFiling` UpsertProduct error non-blocking (call log records failure) |
| TaskListItem missing fields | Added `owner_team`, `priority`, `created_at`, `is_outsource` |
| Keyword search missing task_id | Added `CAST(t.id AS CHAR) = ?` to search clause |
| Timestamp consistency | Verified correct — backend Asia/Shanghai consistent, no fix needed |
| Task creation rules | Verified aligned with PRD — no fix needed |
| **ERP Bridge upsert route missing (2026-03-17)** | Added `POST /v1/erp/products/upsert` handler + route registration; root cause of all Bridge 404 errors on filing |

## 9. Blackbox Test Results (2026-03-16)

| Round | Task Type | Steps | Result |
|---|---|---|---|
| 1 | Original Product Development | Create -> Assign -> Design -> Audit A -> BizInfo -> WH -> Close | PASSED |
| 2 | New Product Development | Create -> Assign -> Design -> Audit A -> BizInfo -> WH -> Close | PASSED |
| 3 | Purchase Task | Create -> BizInfo -> Procurement (draft->prepare->start->complete) -> WH -> Close | PASSED |

Test script: `dist/blackbox-test.sh` (run on server via SSH)

## 10. ERP Bridge Deployment State (2026-03-17)

### Current Bridge Online Status
- **Bridge process**: RUNNING, PID 3416790, port 8081
- **MAIN -> Bridge call**: RESTORED, `ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081`
- **ERP writeback**: Bridge uses `localERPBridgeClient` (DB-backed); actual external ERP API not yet integrated
- **Non-blocking fallback**: RETAINED as safety net in `performERPBridgeFiling`; under normal operation the upsert succeeds (200) and the fallback does not trigger
- **Non-blocking fallback position**: temporary safety net, NOT the primary path; the primary path is now MAIN -> Bridge HTTP -> DB upsert

### Root Cause of Prior Bridge Failure
- `POST /v1/erp/products/upsert` was NOT registered in `transport/http.go`
- All MAIN -> Bridge upsert calls returned 404
- `performERPBridgeFiling` caught the 404 as an error, logged it as failed in `integration_call_logs`, and returned nil (non-blocking)
- This made it appear as though filing was working, but no actual writeback occurred

### Files Changed in This Fix
- `transport/handler/erp_bridge.go` — added `UpsertProduct` handler method
- `transport/http.go` — registered `POST /v1/erp/products/upsert` route under the `/erp` group

### Deployment & Verification Checklist for Next Model
- Verify Bridge PID: `cat /root/ecommerce_ai/run/erp_bridge.pid` + `kill -0 <pid>`
- Verify Bridge port: `ss -tlnp | grep 8081`
- Verify Bridge binary: `ls -la /proc/<pid>/exe` (should NOT say `(deleted)`)
- Verify Bridge env: `cat /root/ecommerce_ai/shared/bridge.env` (must have `SERVER_PORT=8081`)
- Verify MAIN env: `cat /root/ecommerce_ai/shared/main.env` (must have `ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081`)
- Test Bridge directly: `curl -X POST -H "Authorization: Bearer <token>" http://127.0.0.1:8081/v1/erp/products/upsert -d '{"product_id":"test"}'`
- Test MAIN -> Bridge: create a task with `product_selection.erp_product`, then `PATCH /v1/tasks/:id/business-info` with `filed_at`

### Config Files to Check
- `.vscode/deploy.local.env` — `DEPLOY_BRIDGE_BASE_URL=http://127.0.0.1:8081`
- Remote `main.env` — `ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081`, `SERVER_PORT=8080`
- Remote `bridge.env` — `SERVER_PORT=8081`, `ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081`
- `deploy/bridge.env.example` — template for bridge config
- `deploy/main.env.example` — template for main config

## 11. Recommended Reading Order for Next Model

1. **This file** (`MODEL_v0.4_memory.md`) — start here, especially sections 10 (Bridge state) and 7 (remaining gaps)
2. `CURRENT_STATE.md` — overall system state and API readiness
3. `docs/api/openapi.yaml` — API spec (v0.68.0+)
4. `docs/TASK_CREATE_RULES.md` — field-level validation rules
5. `docs/ERP_SEARCH_CAPABILITY.md` — ERP query boundary
6. `docs/ASSET_UPLOAD_INTEGRATION.md` — upload architecture
7. `MODEL_HANDOVER.md` — prior model's handover notes
8. `docs/iterations/ITERATION_070.md` — Bridge restoration iteration doc
9. `docs/iterations/ITERATION_069.md` — v0.4 closure iteration doc
10. `domain/enums_v7.go` — all enum values (TaskStatus, MaterialMode, CostPriceMode, etc.)
11. `transport/http.go` — complete route registry (includes ERP upsert route)
12. `service/task_service.go` — filing logic in `performERPBridgeFiling`
13. `service/erp_bridge_client.go` — Bridge HTTP client
14. `service/erp_bridge_local_client.go` — `ShouldUseLocalERPBridgeClient` logic
15. `config/auth_identity.json` — admin user + department config
16. `.vscode/deploy.local.env` — deployment config (host, port, passwords)

## 12. 三服务边界与 Bridge 补齐（2026-03-17）

### 三服务形态（确认）
- `8080 = MAIN`（业务层）
- `8081 = Bridge`（统一 ERP/JST 适配层）
- `8082 = JST sync`（常驻 JST 同步服务）

### 8081 已补齐能力
- `GET /v1/erp/sync-logs`
- `GET /v1/erp/sync-logs/{id}`
- `POST /v1/erp/products/shelve/batch`
- `POST /v1/erp/products/unshelve/batch`
- `POST /v1/erp/inventory/virtual-qty`

### 8081 当前能力全景
- 查询：products list/detail/categories
- 写入：upsert/shelve/unshelve/virtual-qty
- 观测：sync-logs list/detail
- 运行策略：local / remote / hybrid（hybrid 保留 fallback）

### 8081 当前仍缺口（诚实记录）
- 外部 ERP 正式契约与凭证依赖外部条件，尚不能仅凭仓内代码确认“正式打通”
- 远端 query 全量替换策略暂未推进（保持最小风险）

### 8082 职责（保留，不废弃）
- JST 官方接口拉取同步（全量/增量）
- 回调与本地缓存刷新
- 常驻探活与增量触发继续由 8082 服务承担

### 线上运行验证（2026-03-17 补充）
- 8081：`/health` 返回 `200`；`/v1/erp/products`、`/v1/erp/products/{id}`、`/v1/erp/categories`、`/v1/erp/products/upsert` 以及新增 `sync-logs/shelve/unshelve/virtual-qty` 均返回 `401`（会话鉴权生效，且不再是历史 `404`）。
- 8082：初次探测 `HTTP:000`（端口未监听），后通过 `/root/ecommerce_ai/scripts/start-sync.sh --base-dir /root/ecommerce_ai` 恢复。
- 恢复后 8082：`/health`、`/internal/jst/ping`、`/jst/sync/inc` 均返回 `200`。
- 共存：`8080/8081/8082` 同时监听；`/proc/<pid>/exe` 均非 `(deleted)`。

### 部署运行注意事项
- 当前 `deploy/deploy.sh` 常规 cutover 仅管理 `8080(main)` 与 `8081(bridge)` 启停。
- `8082`（`erp_bridge_sync`）需在发布后单独确认运行状态，必要时执行 `start-sync.sh` 恢复。

### 8081 带 token 验收（2026-03-17 更新）
- 已使用真实会话登录：`POST /v1/auth/login` 成功签发 bearer token，`GET /v1/auth/me` 返回 `200`。
- 已完成 8081 全部 ERP 路由成功路径：
  - `GET /v1/erp/products`
  - `GET /v1/erp/products/{id}`
  - `GET /v1/erp/categories`
  - `POST /v1/erp/products/upsert`
  - `GET /v1/erp/sync-logs`
  - `GET /v1/erp/sync-logs/{id}`
  - `POST /v1/erp/products/shelve/batch`
  - `POST /v1/erp/products/unshelve/batch`
  - `POST /v1/erp/inventory/virtual-qty`
- 本轮实测使用测试对象：
  - `product_id = bridge-accept-1773724718`
  - `sku_code = BRIDGE-ACCEPT-1773724718`
  - `sync_log_id = 5/6/7/8`
- 失败路径也已实测：
  - 未认证返回 `401`
  - 非法分页/空 payload 返回 `400`
  - 不存在的 product / sync-log 返回 `404`
- 结论更新：
  - 当前 8081 不再只是“路由存在 + 鉴权生效”，而是已完成真实会话下的 query/write/log success-path acceptance。

### 三服务巡检与 8082 自恢复（2026-03-17 更新）
- 已新增：
  - `deploy/check-three-services.sh`
  - `deploy/start-sync.sh`
  - `deploy/stop-sync.sh`
- `deploy/verify-runtime.sh` 现会调用三服务巡检脚本。
- `deploy/deploy.sh` 的 cutover 发布后校验现会追加 `--auto-recover-8082`。
- 三服务巡检输出同时包含：
  - 人类可读摘要行
  - `KEY=VALUE` 机器可解析字段
  - `JSON_SUMMARY=...`
- 巡检维度：
  - `8080/8081/8082 /health`
  - pid 文件与 pid 存活
  - TCP 监听
  - `/proc/<pid>/exe` 是否 deleted
- 8082 自恢复策略：
  - 仅当 `8082` 异常时触发
  - 仅执行 `stop-sync.sh`/`start-sync.sh`
  - 不影响 `8080/8081`
- 已完成实机演练：
  - 正常巡检一次：三服务均 `ok`
  - 手动停掉 `8082`
  - 执行 `check-three-services.sh --auto-recover-8082`
  - `8082` 被重新拉起，恢复后 `/health`、`/internal/jst/ping`、`/jst/sync/inc` 均返回 `200`

### 旧十分钟同步与当前关系
- 旧 `cmd/api` 十分钟同步是历史路径
- 当前职责分离是：MAIN 业务、Bridge 适配、8082 JST source sync

### MAIN 接入 Bridge 建议
- MAIN 只依赖 `/v1/erp/*` 业务契约
- 禁止 MAIN 直接耦合 JST/OpenWeb 细节
- 通过 `sync_log_id` 做写入可追溯

### 下一步优先动作
1. 保持发布后固定执行 `verify-runtime.sh` 或 `check-three-services.sh --auto-recover-8082`
2. 如需推进外部 ERP 正式联调，再补 remote query 正式契约与凭证验证
3. 若 8082 二进制后续也纳入发布包，再单独规划其构建/分发收口

### 新推荐阅读顺序
1. `docs/iterations/ITERATION_073.md`
2. `docs/api/openapi.yaml`
3. `service/erp_bridge_service.go`
4. `service/erp_bridge_remote_client.go`
5. `service/erp_bridge_local_client.go`
6. `transport/handler/erp_bridge.go`
7. `transport/http.go`
8. `CURRENT_STATE.md`
9. `MODEL_HANDOVER.md`
