# 资产工作台实施计划

资产工作台是独立运行在 `assets.yongbo.cloud` 的交付计件结算系统，不进入现有运营系统菜单和页面体系，但复用同一套账号、Session、OSS 直传、后端预览渲染和审计能力。

## v1 产品边界

- 人员档案：`users` 继续作为登录身份，工作台私有字段写入 `asset_workbench_profiles`，岗级历史写入 `asset_workbench_profile_grade_periods`。
- 成品上传：一次 `submission` 是提交批次；`submission_items` 是订单/计件明细；`submission_files` 是挂在 item 下的文件对象。
- 成本中心：价目矩阵、出错扣减规则、福利规则、大促券基础能力。
- 结算：以 `submission_items` 为最小结算单位，按标准人月聚合生成结算预览和批次，并固定输出每员工/月 2 条工资行。
- 素材库：系统资产只读搜索走 `/v1/asset-workbench/system-search`，不扩大旧 `/v1/assets/search` 权限面。

## 核心数据合同

- 提交时只冻结 `worker_type`、`job_grade`、`difficulty_class`、基础单价、大促命中快照和计件毛额。
- 出错扣减不在提交时冻结；结算时读取 `error_records` 和当时命中的 `deduction_rules`，生成 `error_deduction` 结算行并冻结规则快照。
- 福利补贴不挂单条 item；批次生成时按 `payee_user_id + business_month` 生成 `welfare` 结算行。
- 工资条固定为每员工/月 2 条：正常计件工资 1 条，补录计件工资 1 条；没有补录时补录计件工资为 0。
- 已确认批次追加的 `adjustment/reversal` 进入正常计件工资行的“调整”列，补录计件工资行只承载补录金额。
- 次月补录漏上传时，补录记录仍关联原 `business_month`，在该业务月工资条中单独形成补录计件工资行。
- 补录默认关闭，必须按 `payee_user_id + business_month` 显式开放后才允许创建补录记录；开放前必须确认该员工该业务月已有 confirmed 结算批次。
- 防重复结算下沉到 `submission_items.current_settlement_batch_id + settlement_status`。
- 数据库存 UTC；业务月份统一按 `Asia/Shanghai` 切月。
- PII 按业务要求明文落库，但界面脱敏、权限控制、查看/导出审计必须做。

## 结算行类型

`settlement_items.item_type` 固定为：

- `gross_piecework`
- `error_deduction`
- `welfare`
- `supplement`
- `adjustment`
- `reversal`

`submission_items` 只保存毛额，不保存最终净额；净额由 settlement batch 汇总得到。

## 延期业务项

以下口径不阻塞 v1 基础开发，后续以扩展方式补齐：

- 福利补贴细则：v1 建表和 UI 预留，复杂全勤、无差错奖、出勤联动后续实现。
- 大促券叠加：v1 默认一口价优先于涨幅，多个命中取最高优先级，不做多券叠加。
- 月度补录查重：v1 做订单号 + 人员 + 月份提示查重，不做强排他。
- 素材库元数据来源：v1 优先复用 system asset + 扩展元数据表。
- 专用导出格式：v1 提供工资条、人月汇总、批次明细 xlsx 导出；如财务最终 Excel 样式有固定模板，再单独补专用导出模板。

## 前后端维护合同

- 后端新增 `service/asset_workbench`、`repo/asset_workbench.go`、`repo/mysql/asset_workbench_*`、`transport/handler/asset_workbench.go`、`transport/routes_asset_workbench.go`。
- 生产 HTTP 入口固定为 `cmd/server/main.go`；`cmd/api/main.go` 仅兼容保留。
- `/v1/asset-workbench/*` 注册在共享 router；预览 worker 只在 `cmd/server` 通过 `workers.GroupDeps` 启动。
- 预览 worker 进入数据阶段后必须用 `SELECT ... FOR UPDATE SKIP LOCKED` 领取 pending preview files，并写 worker lease。
- 上传会话过期清理由 `AssetWorkbenchUploadExpiryWorker` 统一处理；默认 dry-run 以外不允许页面侧自行清理 OSS 分片。
- 前端入口固定为 `asset.html`、`vite.asset.config.ts`、`src/asset-workbench/main.ts`。
- 工作台前端采用 FSD-lite：`app/pages/widgets/features/entities/shared`。
- 禁止 import 主站 `AppShell`、router、views、`main.css`。
- 工作台 CSS 只能走 `.aw-root`、`--aw-*`、`aw-`；由 `npm run asset:audit` 卡结构和样式漂移。
- 页面和业务 TS/Vue 源码不得硬编码 `px/ms` 尺寸或动效值；统一通过 `--aw-*` token 与 `styles/recipes.css` 承载，避免后期再做 CSS 冗余治理。
- 工作台页面禁止 `prompt/confirm/alert`，需原因的操作必须使用页面内表单并保留可见状态，保证高频键盘流程和审计语义稳定。
- `AssetPreviewMedia` 只通过 `resolvedPreviewUrl` 复用，工作台 adapter 先调用工作台 preview endpoint。
- 外部新人自注册走 workbench 专用 `RegisterAssetWorkbenchUser` 路径，明确只授予 `AssetSubmitter`，再写入 pending 的 `asset_workbench_profiles`；不得复用通用 `IdentityService.Register` 作为公网注册入口。
- HR/结算角色可在工作台人员页定级、调整人员类型和状态；列表响应默认脱敏 PII，HR 更新空 PII 字段时后端保留旧值，避免把脱敏掩码或空值写回数据库。
- 结算预览和批次明细都必须返回 `payroll_rows`，用于工资条、导出和复核，禁止页面临时拼工资条口径；批次详情工资条必须反映 `adjustment/reversal` 后的净额。
- 工资条导出由 `src/asset-workbench/features/export/settlementExport.ts` 统一生成，页面不得复制导出列定义。
- 复杂列表统一使用 `src/asset-workbench/shared/grid/WorkbenchDataGrid.vue`，列宽、列序、分组和虚拟滚动在组件内治理，页面不得复制一套临时数据网格。

## external 下线与清理

- 生产保持 `EXTERNAL_ASSETS_ENABLED=false`；主站隐藏 external 来源筛选，并过滤旧 external 返回项。
- `ext-*` 详情、预览、下载在 external service disabled 时返回 404，不再扩大旧 `/v1/assets/*` 的 external 能力。
- 清理脚本为 `deploy/cleanup-external-asset-previews.sh`，默认只导出：
  - `external_asset_records`
  - `external_asset_sync_runs`
  - `oss_preview_key` 待删列表
- 只有显式传 `--execute --bucket oss://...` 才删除旧 external preview 对象；原始对象不触碰。

## 交互与性能验收

- 本地操作反馈小于 100ms；命令面板打开小于 50ms。
- 核心流程支持纯键盘完成：搜索、上传、勾选、质检、生成批次。
- 写操作使用乐观更新，失败回滚。
- 数据网格支持列宽/列序记忆、保存视图、行内编辑、分组、虚拟滚动。
- 动效 token 固定 `150/200/300ms`，`prefers-reduced-motion` 全量降级。
- 禁止近黑底 + 单荧光色皮肤，保留 Grading Desk 冷石墨调色台。

## 交付阶段

1. Codex 维护治理、独立前端 shell、构建/审计脚本。
2. 账户档案、岗级历史、自注册、HR 维护。
3. 价目矩阵、扣减规则、福利规则、大促券基础能力。
4. 上传会话、提交批次、submission items/files、毛额快照。
5. 预览 worker、preview endpoint、前端预览 adapter。
6. 素材库只读搜索、预览、下载、批量下载。
7. 出错 Excel 导入、匹配、unmatched/ambiguous 处理。
8. 标准人月报表、两条工资行、结算预览、批次生成/确认/取消、冲正补差。
9. 月度补录基础版与按人按月开放权限。
10. external 下线、nginx/DNS/TLS、发布脚本。
11. 后续填补：复杂福利、复杂大促叠加、强查重、专用导出模板。

## 验收命令

- `go test ./domain ./config ./service/asset_workbench ./workers ./transport ./cmd/server ./cmd/api`
- `cd vue && npm run build`
- `cd vue && npm run asset:audit`
- `cd vue && npm run build:asset`
- 覆盖重点：HR 更新保留 PII、生成批次按 `submission_items` attach、取消 generated 批次释放 item 后可重结算、工作台无 `prompt/confirm/alert`。
