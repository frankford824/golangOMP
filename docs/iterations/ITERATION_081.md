# ITERATION_081 — 最小阻塞修复 + 重部署 + 服务器复验（已收口）

**Date**: 2026-03-18  
**Scope**: 8080 主工程真实环境验收（5 模块）+ `docs/api/openapi.yaml` 全量语法清洗与契约补齐  
**结论状态**: **达到可收口**（本轮指定阻塞项已全部修复并复验通过）

## 0A) 原品开发创建任务 500（ERP 绑定解析）专项补丁（同日增补）

### 现象与日志
- 线上 trace：`9837a36e-ebaa-4262-9a91-6bc1ff3d7a47`，`POST /v1/tasks` 返回 `500`。
- 在服务增加事务失败日志后，复现同类 payload 得到真实错误：
  - `create task: insert task_detail: Error 1136 (21S01): Column count doesn't match value count at row 1`

### 根因
- `repo/mysql/task.go` 中 `INSERT INTO task_details` 列数 57，但 VALUES 占位符仅 55，导致创建事务内 DB 写入失败并被包装成 `internal error during create task tx`。

### 最小修复
- 修复占位符：`task_details` INSERT 占位符补齐至 57。
- 同步补强原品开发商品绑定归一逻辑（创建入口）：
  - 优先级：`top.product_id` -> `product_selection.erp_product.product_id` -> `product_selection.erp_product.sku_code` -> `top.sku_code`
  - `EnsureLocalProduct` 增加 `sku_code` 绑定回退键。
  - 增加绑定路径日志与失败日志，字段包含 `trace_id/task_type/top_product_id/erp_product_id/erp_sku_code/binding_path`。

### 复测
- Case A（`product_id=null` + `erp_product.product_id+sku_code`）：
  - `201`，`task_id=95`，归一本地 `product_id=485`。
- Case B（`product_id=null` + 仅 `sku_code`）：
  - `201`，`task_id=96`，归一本地 `product_id=485`。
- 两个 case 均不再出现 500。

## 0) 本轮最小阻塞修复结论（新增）

- 根因：线上运行二进制与当前代码/契约未对齐（仍在旧发布物），导致“代码存在但线上 404/错路由”。
- 最小代码修复：
  - `transport/http.go`：`/tasks/batch/*` 注册顺序前移，避免被 `/:id/assign` 吞路由。
  - `service/task_asset_center_service.go`：delivery complete 自动推审状态扩展到 `PendingAssign/Assigned`（并保留 `InProgress/RejectedByAuditA/RejectedByAuditB`）。
- 重部署：发布 `v0.6` 后，MAIN 进程为 `PID=3777316`，`/proc/3777316/exe -> /root/ecommerce_ai/releases/v0.6/ecommerce-api`。
- 阻塞复验通过：
  - A 批量路由：`batch/remind` 200；`batch/assign` 命中批量 handler（200，非 `invalid task id`）。
  - B 商品/成本 5 接口：路由均生效（4 个 200 + preview 按前置条件 400）。
  - C 资产下载：download / version-download 均 200。
  - D delivery 推审：`PendingAssign` 任务完成 delivery 上传后状态变更为 `PendingAuditA`（任务 84，`updated_at=2026-03-18T04:22:21+08:00`）。

## 1) 服务器运行态证据

- 目标机：`223.4.249.11`（`root`，目录 `/root/ecommerce_ai`）
- 三服务监听：
  - `*:8080 -> ecommerce-api (pid=3589336)`
  - `*:8081 -> erp_bridge (pid=3589373)`
  - `127.0.0.1:8082 -> erp_bridge_sync (pid=3589421)`
- `health`：
  - `GET http://127.0.0.1:8080/health -> {"status":"ok"}`
  - `GET http://127.0.0.1:8081/health -> {"status":"ok"}`
  - `GET http://127.0.0.1:8082/health -> {"status":"ok"}`
- `main.env` 存在并已脱敏核对关键键：`DB_*`、`UPLOAD_SERVICE_*`、`AUTH_SETTINGS_FILE` 等。

## 2) DB / migration 证据与最小修复

### 初始发现
- `task_details` 缺失以下列（041/042/043 对应）：
  - `filing_status`
  - `filing_error_message`
  - `note`
  - `reference_file_refs_json`
  - `cost_price_source`

### 本轮执行
- 已在服务器库执行 041/042/043 对应 DDL（最小补执行）。
- 复核后 5 列全部存在（`information_schema.columns` 计数均为 1）。

### 兼容性补修（关键）
- 由于线上运行二进制仍按旧写入路径创建任务，新增 `TEXT NOT NULL` 列触发 `POST /v1/tasks` 500。
- 已做最小兼容修复（仅数据库列约束）：
  - 将 `filing_error_message` / `note` / `reference_file_refs_json` 改为 `TEXT NULL`。
- 修后复验：三类任务创建均恢复 `201`。

## 3) 五模块实操验收结论（按状态分级）

### 模块 1：登录、组织、角色、数据权限
- **已实现且已验收**
  - `POST /v1/auth/login`（`username+password`）成功。
  - `GET /v1/auth/me` 成功返回 `roles` 与 `frontend_access`。
  - 普通用户（member）访问 `tasks/board/detail` 返回 `403`，具备 `required_roles` 细节。
  - 非 Admin 执行高敏感角色变更 `POST /v1/users/1/roles` 返回 `403 PERMISSION_DENIED`。
- **已实现但未联调**
  - `/v1/me` 在当前线上为 `404`（仅 `/v1/auth/me` 可用）。
- **风险**
  - DataScopeResolver “范围差异”在 Admin vs Designer 实测中未观察到任务集合差异（同集 20 条），仅验证了角色门禁，不构成范围裁剪充分证据。

### 模块 2：工单中心（列表/看板/筛选/批量）
- **已实现且已验收**
  - 列表：`GET /v1/tasks` 成功；`task_type`、`keyword`、`priority`、`missing_fields_only` 均返回 200 且有计数差异。
  - 看板：`GET /v1/task-board/summary|queues` 对授权角色可用。
- **部分实现**
  - `POST /v1/tasks/batch/remind` 当前线上 `404`。
  - `POST /v1/tasks/batch/assign` 当前线上被路由到 `/:id/assign`（`invalid task id`），说明批量路由未生效。
  - `TaskListItem` 当前线上返回字段集中不含 `filing_status/missing_fields/missing_fields_summary_cn/cost*` 等本轮目标字段。
- **风险**
  - 批量催办无 `task.reminded` 真实事件闭环证据（接口不可达）。

### 模块 3：任务创建与建档
- **已实现且已验收**
  - `new_product_development`：创建 `201`，详情 `200`。
  - `purchase_task`：创建 `201`，详情 `200`。
  - `original_product_development`：创建 `201`，详情 `200`，`product_id` 可走 ERP facade id。
- **部分实现**
  - `GET /v1/tasks/{id}/detail` 返回中未见 `filing_status` 字段（值为 `null/缺失`），与 041 目标未形成接口可见闭环。
  - `trigger_filing` 调用可返回 `200`，但 filed/filing_failed 路径未在当前环境形成完整 ERP 建档闭环证据。
- **风险**
  - 建档状态机字段虽已补列，但线上读模型未稳定暴露。

### 模块 4：商品信息与成本维护
- **未实现/失败（以线上实操为准）**
  - `GET/PATCH /v1/tasks/{id}/product-info`：`404`
  - `GET/PATCH /v1/tasks/{id}/cost-info`：`404`
  - `POST /v1/tasks/{id}/cost-quote/preview`：`404`
- **风险**
  - 与代码侧“已完成”不一致，当前线上不可联调。

### 模块 5：设计资产中心
- **已实现且已验收（核心流程）**
  - upload-session 创建、complete、资产列表、版本列表均可实操。
  - reference/source(含伪 PSD)/delivery 四类上传会话与 complete 均成功。
  - 资产事件链写入可见：`task.asset.upload_session.created/completed`、`task.asset.version.created`。
  - source/PSD 语义正确：`preview_available=false`、`access_policy=source_controlled`、`public_url=null`、`source_file_requires_private_network=true`。
  - reference/delivery 语义正确：`preview_available=true`、`public_download_allowed=true`。
  - 历史版本：同 asset（id=8）二次上传后版本从 v1 增至 v2（版本列表可见）。
- **部分实现/失败**
  - 下载契约接口 `GET /v1/tasks/{id}/assets/{asset_id}/download` 与 version-download 当前线上 `404`。
  - delivery 上传后任务未推进到 `PendingAuditA`（任务仍 `PendingAssign`），未形成审核推进闭环。
- **风险**
  - 上传会话远端直传接口 `/upload/files` 返回 `task_ref is required`（上传侧字段契约待对齐）；但 MAIN `complete` 仍可完成版本落库。

## 4) OpenAPI 清洗结果

### 解析
- 清洗前：`yaml.safe_load` 失败  
  - `ScannerError: mapping values are not allowed here`
  - 位置：`docs/api/openapi.yaml` line 7990
- 清洗后：YAML 全量解析通过（`YAML_PARSE_OK`）。

### 本轮清洗动作（不改业务语义）
- 修复 line 7990 损坏描述（改为 block scalar，去除引号冲突/冒号扫描歧义）。
- 补齐五模块重点缺口路径定义：
  - `POST /v1/tasks/batch/assign`
  - `POST /v1/tasks/batch/remind`
  - `GET /v1/tasks/{id}/assets/{asset_id}/download`
  - `GET /v1/tasks/{id}/assets/{asset_id}/versions/{version_id}/download`
- 统一补齐关键 schema（契约命名层）：
  - `TaskProductInfo`
  - `TaskCostInfo`
  - `TaskCostQuotePreviewResponse`
  - `AssetDownloadInfo`
  - `ErrorResponse`
  - `ValidationViolation`

### 仍未收口项
- `/v1/tasks/{id}/filing-status`、`/v1/me`、`/v1/me/permissions` 仍未定义（按“若存在”原则保留现状）。
- 线上运行版本与 OpenAPI（以及仓库代码）仍存在路由能力差异（典型：per-task 商品/成本、批量催办、下载接口）。

## 5) 本轮修改清单

- 文档：
  - `docs/api/openapi.yaml`
  - `docs/iterations/ITERATION_081.md`
  - `CURRENT_STATE.md`
  - `MODEL_HANDOVER.md`
  - `ITERATION_INDEX.md`
  - `docs/FRONTEND_ALIGNMENT_v0.5.md`
- 服务器数据库（真实环境）：
  - 执行 041/042/043 对应 DDL
  - 兼容性修复：3 个 TEXT 列改为可空

## 6) 测试与校验

- 本地：`go test ./...` 通过。
- OpenAPI：YAML 全量 parse 通过。
- 服务器：运行态、DB 字段、五模块接口/文件流均有实操证据（含失败证据）。

## 7) 一句话结论

当前五模块**未达到可收口**；最小阻塞是“线上运行能力与目标契约未对齐”（模块 2 批量路由、模块 4 五接口、模块 5 下载与 delivery 推审闭环仍缺）。

