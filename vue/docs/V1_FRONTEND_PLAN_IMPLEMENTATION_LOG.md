# V1 Frontend Plan Implementation Log

## Completed Batches

- Phase 0: mock infrastructure, env flags, ws mock bootstrap, API stubs.
- Phase 0: Pencil web conventions loaded and documented in `designs/V1_PENCIL_CONVENTIONS.md`.
- Phase C: `v1-app-shell.pen` designed and exported preview.
- Phase C/A baseline: top nav shell, global search/notification/avatar components, route shells, task center/detail/create scaffolds, draft flow baseline.
- Phase A hard remove baseline: legacy workbench views and related route entries removed.
- Phase B baseline: mock module events, notification push trigger, upload complete persistence, initial E2E skeleton test.
- Pool/API category 字段语义对齐（本轮）：
  - `TaskListView` 的「任务池」Tab 改为角色分流：执行角色走 `filter=module_pool` → `/v1/tasks/pool`；运营/管理员走 `/v1/tasks` + `task_status=PendingAssign`，避免 admin 命中空模块池。
  - 接单动作收紧为「仅模块池且行内存在 `module_key`」才显示；`claimModule` 不再使用 `design/procurement` 前端兜底。
  - 仪表盘「待审核」「需交班」不再跳 `?tab=pool`，统一跳 `/tasks?status=PendingAuditA,PendingAuditB`；「查看任务池」保留。
  - `business-info` 类目写入改为语义分流：新增 `buildCategoryPatchFields`（内部码 `KT_*/OUT_*` → `category_code`，中文/i_id/展示值 → `category`），并替换创建后预填、结单补全、采购补全三处 PATCH。
  - GET 回显补齐：`normalizeBackendTask` 在 `category_code` 为空时回落 `category_name/category`；详情合并器补 `task_detail.category`；详情标题与补全面板 hydrate 支持 `erpIId/categoryName/category` 回落。
  - Mock 对齐：`/v1/tasks/pool` 按 `pool_team_code` 过滤（默认 admin 团队为空命中）；`/v1/tasks` 支持 `task_status=PendingAssign` 和 `owner_team/owner_org_team=未分配池`。
  - 测试补齐：新增 `tests/category-payload.spec.ts`，并扩展 `tests/v1-e2e-scenarios.spec.ts` 覆盖「模块池团队过滤」与「PendingAssign 走 list」。
- V1 全量补齐（本轮）：
  - 浅色 token：`v1-tokens` 在 `main.css` 中置于 `@tailwind` 之前；`AppShell` / 浮层与首页已对齐浅色变量；`BaseTag` 合并为单一实现（`label` + `variant` 别名 + 默认插槽）；`MyOrgView` 从 `departments`/`groups`/`actorDepartment` 解析展示。
  - 全局搜索：`GlobalSearchOverlay` 模板与脚本统一（多分组、权限隐藏 users、`↑/↓/Enter`、高亮、慢查询提示）；`TaskDetailView` 作废 409 分支：部门管理员可「转关闭」(`force: true`)；Mock `POST /v1/tasks` 响应去重；`TaskCreateModal` `verified` 事件用 `unknown` 收口。
  - 草稿/通知 API：`taskDraftsApi.listMine` + store 用 `GET /v1/me/task-drafts`；`NotificationBadge` 优先 `unread-count`；路由 `TASK_REACHABLE_MENUS` 收敛为 `task_list`。
  - Hard remove：删除 `DashboardView.vue`（重复/损坏）与 `TaskCreateView.vue`；`tsconfig` 取消 exclude；旧工作台相关单测已移除；`vitest` 启用 `@vitejs/plugin-vue` + `happy-dom` + `VITE_USE_MOCK` define，全量 96 测通过。

- V1 前端能做即做（本轮，纯前端 + Mock）：
  - 任务中心：`TaskListView` 改为 **参数驱动**：`page`/`page_size`/`sort`/`task_type`/`priority`/`status`/`keyword`/`date_from`/`date_to` 全部进入 `GET /v1/tasks`，不再本地 filter；`TaskCenterFilterBar` 增加日期范围两列；`TaskPoolRow` 直接消费 `module_key`；分页控件、卡片/列表切换；进入/出 URL query 双向同步。
  - Mock 列表：`filterTasks` 增补 `date_from` / `date_to` / `sort`；`POST /v1/tasks/{id}/cancel` 已支持 `force`/409；`mocks/index.ts` 适配器调用 `validateStatus` 统一走 AxiosError（否则 409 不会触发前端 `catch` 分支）。
  - 创建任务：`TaskCreateModal` 对接「关闭三选一」（保存草稿 / 丢弃 / 取消）并支持 `BaseModal` × 按钮触发；`CloseDraftConfirmModal` 文案/顺序按文档重排；`ExcelBatchSkuPanel` 四步完整：下载模板（CSV fallback）→ 说明 → 上传 → 预览表（行级错误高亮）；F-1 新手引导横幅落于 `TaskListView`，以 `localStorage` 持久「不再提示」。
  - 全局搜索：产品/用户分组结果保留为「只读条目」（不跳转、添加「只读」badge），避免伪链接；任务/资产仍照常导航。
  - 任务详情：Panel 能力矩阵（`BasicInfoPanel` / `DesignPanel` / `AuditPanel` / `RetouchPanel` / `CustomizationPanel` / `ProcurementPanel` / `WarehousePanel`）统一从 `module.allowed_actions.actions` 消费、按钮联动；`TaskHeader` 输出状态 / 优先级 tag，并按「状态 + 角色」收敛作废/关闭入口；模块标题中英映射。
  - Timeline：已对接 `GET /v1/tasks/{id}/events`（本轮核对保留）。
  - 通知：`MyNotificationsView` 增加「全部 / 未读」筛选、全部已读、单条已读、WS 刷新；`/v1/me/notifications/mock-generate` 新建通知端点已存在，供 E2E 使用。
  - 报表 L1：路由 `meta.requiredRoles = ['super_admin']`，router guard 落地 `requiredRoles` 拦截；顶栏入口本身已在 `AppShell` 按 `super_admin` 显示。
  - 个人中心：`AccountView` 作为 6 板块 IA 落地页（账户资料 / 安全 / 组织 / 通知 / 草稿 / 我的任务），退出登录入口复用 `permissionsStore.logout`。
  - 测试：`tests/v1-e2e-scenarios.spec.ts` 补充 7 条（分页、pool CAS、409 → force、Excel 四步、read-all 清零等），全量 101/101 通过；`npm run build` 清；无 `vue-tsc` 报错。

## Pending Backend Alignment Gaps

- Task detail contracts are currently mock-first; backend fields may require mapper alignment.
- Task cancellation conflict branch (`409 + force`) is wired client-side but needs backend final deny payload.
- Notification type taxonomy is partially mocked and needs final backend enum lock.
- Batch SKU and Excel parsing are placeholder-grade in mock, pending real API parser.
- Module blueprint transitions are basic and require backend workflow matrix confirmation.
- `allowed_actions` 动作集：各 Panel 目前按 mock 输出（`claim` / `submit` / `asset_upload_session_create` / `update_reference_files` / `approve` / `reject` / `update_quantity` / `receive` / `archive` / `download_bundle`）渲染；待后端模块矩阵正式锁定后做对齐校验。
- 任务详情头部 `cancel` / `close` 的管理员门禁目前用角色前端判断；后端允许动作下发后应改为从读模型读取。
- 报表 L1：当前 `kpi`/`finance` 菜单键叠加 `requiredRoles=['super_admin']`，待后端明确仅 SuperAdmin 可视时可移除菜单键以简化。
- 账户资料：`AccountView` 的昵称/手机/邮箱保存按钮目前为占位，待 `PATCH /v1/me/profile` 就绪后接入。
- 任务池「模块可接单」动作：前端按行 `module_key` 调 `POST /v1/tasks/{id}/modules/{module_key}/claim`；后端团队校验（`confirm_pool_team_code`）需要真实用户 actor 数据才能完整联调。
