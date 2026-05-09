# V1 前端方案（与模块化架构 v4 + 三份子文档高度对齐）

> 状态：Draft v2 · 合并 FE Plan 评审回执（task-cancel / 草稿端点 / audit 参考图 / critical 下拉 / @UI 高亮），待签字
> 参考文档（以下四份为唯一权威源）：
> 1. `V1_MODULE_ARCHITECTURE.md`（**Draft v4** · 骨架；§9.1.1 `POST /v1/tasks/{id}/cancel` 双语义端点、§6 Layer 3 audit 可挂审核参考图）
> 2. `V1_INFORMATION_ARCHITECTURE.md`（**Draft v3** · §3.5.9 任务草稿端点族 / §7.2 草稿子 tab / §8.1 `task_mentioned` v1 不做）
> 3. `V1_ASSET_OWNERSHIP.md`（Draft v1 · 资产/版本/生命周期；本轮无改动）
> 4. `V1_CUSTOMIZATION_WORKFLOW.md`（**Draft v2** · §3.1.1 / §3.2.1 通用字段对齐 G1、`critical` 对 DeptAdmin+ 保留；§3.2.1 `design_source_lookup_id` v1 单选）
>
> 本文只规定**前端**落地方案：路由、页面、组件、交互、校验、状态映射、API 消费方式、埋点/降级，**不改动任何代码**。
> 冲突裁决：本文与主文档冲突时以主文档为准；与子文档冲突时按主文档→子文档的层级裁决。

> 2026-04-25 执行口径：本轮前端实现以本文 Draft v2 为基础，并采纳最新 UI 反馈：任务中心统一卡片展示、创建任务弹窗统一尺寸与紧凑布局、创建态参考图/附件上传采用按钮式入口。若业务补充诉求仍与本文已入档决策冲突（例如采购批量 SKU、旧式录入模板/管理商品弹窗、审核上传正式资产稿件），先记录为后续变更议题，不在本轮代码中实现。

---

## 0. 目录

1. 范围与原则
2. 菜单与路由
3. 顶部布局（全局搜索 / 通知 badge / 头像下拉）
4. 任务中心（4 tab + 二级筛选 + 统一卡片）
5. 任务池 Tab + 接单交互（CAS 冲突处理）
6. 任务详情页（一屏化骨架 + 7 个模块 Panel）
7. 创建任务弹窗总控 + 保存草稿（`/v1/task-drafts` 消费）
8. 原品开发（单）
9. 新品开发（单 SKU）
10. 新品开发（批量 SKU · Excel 唯一入口）
11. 采购任务（单 SKU）
12. P 图任务
13. 客户定制（ERP 编码查询联动）
14. 常规定制（设计源文件查询联动）
15. 上传组件（格式/大小/分片/断点续传/STS）
16. 参考图 owner_module_key 前端归属
17. 审核 always-latest + 历史快照 + 410 GONE 降级
18. 资产管理中心页面
19. 组织菜单（用户 / 部门 / 组）
20. 个人中心（头像下拉 6 板块）
21. 全局搜索（Ctrl+K）
22. 通知中心 + WebSocket
23. 报表菜单（SuperAdmin · L1 卡片）
24. 前端组件化与目录结构建议
25. deny_code → 前端降级处理
26. 展示状态映射表（derived_status → 用户语言）
27. 兼容路由与 Feature Flag
28. 前端验收标准
29. 风险与回滚前端侧
30. 待确认项

---

## 1. 范围与原则

### 1.1 本文覆盖范围（前端落地）
- 登录后所有可见页面 / 菜单 / 组件 / 交互。
- 仅与 4 份文档声明的 API 契约消费方式有关的部分（不涉及后端实现）。
- 菜单可见性、权限降级、状态映射、批量 SKU Excel 流程、定制任务查询联动、资产中心、组织管理、全局搜索、通知中心、个人中心、L1 报表。

### 1.2 前端设计原则
1. **契约驱动渲染**：详情页按 `modules[].scope` + `modules[].allowed_actions` 动态渲染，不硬编码角色判断。
2. **模块未实例化不渲染**：`deny_code=module_not_instantiated` → 整块 DOM 不出现。
3. **作用域外只读**：`module_out_of_scope` → 内容可见、操作区不渲染。
4. **CAS 冲突友好化**：接单失败（`module_claim_conflict`）统一 toast "该任务已被他人领取"，并自动刷新列表。
5. **退化路径先行**：任意一项未就绪（WebSocket 断线、资产 410 GONE、ERP 上游失败、通知失败）都要有可见降级展示。
6. **新品开发批量 SKU = Excel 唯一入口**：**禁止**保留旧多行内联编辑 UI。
7. **其他任务类型不提供批量入口**：采购 / 原款 / 精修 / 定制全部只走单 SKU 表单。
8. **下载不门控**：Layer 1 可见即可下载（与资产中心口径一致），不再在前端埋任务组别检查。

### 1.2.1 当前代码偏差清单（2026-04-25）
- 任务中心仍保留上一版表格 / 看板 / 卡片切换实现，需要收敛为统一卡片视图。
- Dashboard 与风险卡片仍残留 `/audit`、`/customization-jobs`、`/warehouse` 等旧工作台入口，需要统一收敛到任务中心或任务详情。
- 创建任务弹窗仍为双栏结构，右侧存在"创建总览"，底部操作按钮绑定在右栏，未完全对齐 §7.1。
- P 图任务在当前代码中仍以"补图 + 原款开发"叠加表达，尚未作为 `retouch_task` 一等创建类型贯穿类型、payload 与 mock。
- 本地 mock 尚未覆盖详情页关键动作（设计提交、审核通过 / 驳回、仓库接收 / 归档、基础信息 patch），离线验证闭环不完整。

### 1.3 本文不包括
- 后端 SQL / Go 代码、blueprint 常量、OpenAPI 契约内部实现。
- R5 后的 L2/L3 报表前端设计。
- 移动端 / OpenAPI 外部开放。
- 偏好 / 登录历史 / 我的数据（个人中心不包含，信息架构 §7.3）。

---

## 2. 菜单与路由

### 2.1 一级菜单（顶部导航）

| 菜单 | 路由 | 可见角色 | 说明 |
| --- | --- | --- | --- |
| 任务中心 | `/tasks` | 全员 | 4 tab（见 §4） |
| 组织 | `/org` | SuperAdmin / HRAdmin / DeptAdmin（可写）/ TeamLead（只读） | Member 不可见 |
| 资产管理中心 | `/assets` | 全员 | 下载开放，归档/删除仅 SuperAdmin |
| 报表 | `/reports` | 仅 SuperAdmin | v1 仅 L1 卡片 |

其余放**头像下拉**：账户信息 / 安全 / 我的组织 / 我的任务 / 我的待接单 / 通知中心 / 退出登录。

### 2.2 菜单可见性矩阵（对齐信息架构 §2）

| 菜单 | SuperAdmin | HRAdmin | DeptAdmin | TeamLead | Member |
| --- | --- | --- | --- | --- | --- |
| 任务中心 | ✓ | ✓ | ✓ | ✓ | ✓ |
| 组织 | ✓（全写） | ✓（全写，除删部门） | ✓（本部门） | ✓（只读本组） | ✗（不可见） |
| 资产管理中心 | ✓（含归档/删除） | ✓ | ✓ | ✓ | ✓ |
| 报表 | ✓ | ✗ | ✗ | ✗ | ✗ |
| 头像下拉 · 个人中心 | ✓ | ✓ | ✓ | ✓ | ✓ |

> 菜单开关由瘦身版 `config/frontend_access.json` 控制（信息架构 §10）；前端不再读其中的 `actions` 字段做权限判断。

### 2.3 路由表（建议）

```
/                                  → 重定向 /tasks
/tasks                             任务中心（默认 tab=全任务）
/tasks?tab=pool                    任务中心 · 任务池
/tasks?tab=mine                    任务中心 · 我的任务
/tasks?tab=archived                任务中心 · 已归档
/tasks/:id                         任务详情页
/tasks/create                      创建任务（以 modal 路由形式出现，可分享 URL）
/tasks/create?draft_id=:id         从草稿继续编辑（按 draft.task_type 打开对应弹窗，回填 payload）
/me/task-drafts                    个人中心 · 我的任务 · 草稿子 tab（深链入口，跳 /tasks?tab=mine&sub=drafts）

/org                               组织总览
/org/users                         用户
/org/departments                   部门
/org/teams                         组
/org/move-requests                 跨部门调配（SuperAdmin 待办）

/assets                            资产管理中心列表
/assets/:id                        资产详情（版本列表 + 归档/删除入口 SuperAdmin）

/reports                           报表首页（SuperAdmin）
/reports/task-throughput           任务吞吐看板
/reports/module-dwell              模块驻留时长

/me                                账户信息
/me/security                       安全（改密）
/me/org                            我的组织（只读）
/me/notifications                  通知中心

/login / /logout                   登录态
/403 /404 /410 /500                降级页
```

---

## 3. 顶部布局

```
┌──────────────────────────────────────────────────────────────────────┐
│  [Logo]    任务中心  组织  资产管理中心  报表(SA)    [🔍 搜索(Ctrl+K)]  🔔(N)  [头像▼]  │
└──────────────────────────────────────────────────────────────────────┘
```

- Logo 左对齐；一级菜单居中；右侧：全局搜索 + 通知 badge + 头像下拉。
- `🔔(N)` 是通知中心未读数 badge；WebSocket `notification_arrived` 推送时 +1，超过 99 显示 `99+`。
- 头像点击展开 6 板块下拉（§20）。

---

## 4. 任务中心

### 4.1 页面骨架

```
[Tabs]  全任务 | 任务池 | 我的任务 | 已归档                       [刷新] [创建任务]
[二级筛选行]
  类型: 全部 | 常规(原款/新款/采购/精修) | 定制(客户定制/常规定制)
  优先级: normal | urgent | critical
  状态: (derived_status 枚举)
  创建者: (成员选择器，支持按组过滤)
  接单人: (同上)
  日期范围: [起] ~ [止]
  关键字: 任务号 / 产品编码 / 标题
[列表区 - 统一卡片视图]  ←  不再提供表格 / 看板 / 卡片切换
[加载更多 / 分页]
```

### 4.2 Tab 默认查询（对齐信息架构 §3.1）

| Tab | 后端查询 | 默认排序 |
| --- | --- | --- |
| 全任务 | `GET /v1/tasks?status=all` | `priority DESC, created_at DESC` |
| 任务池 | `GET /v1/tasks/pool?module_key=any&pool_team_code=<actor所有teams>` | `priority DESC, created_at ASC` |
| 我的任务 | `GET /v1/tasks?filter=mine` | `updated_at DESC` |
| 已归档 | `GET /v1/tasks?status=archived` | `archived_at DESC` |

### 4.3 卡片字段（信息架构 §3.3 最小集转为卡片呈现）

```
任务号 | 任务大类(常规/定制) | 类型 | 优先级 | derived_status | 创建者(组) | 当前处理人(组) | 创建时间 | 更新时间 | 操作(查看详情)
```

- 任务号：可点击进入详情。
- 任务大类：卡片顶部以 Tag 明确展示"常规任务"或"定制任务"。
- 类型：Tag 彩色（原款/新款/采购/精修/客户定制/常规定制 各一色）。
- 优先级：`normal` 不显示 Tag；`urgent` 橙"加急"；`critical` 红"紧急"。
- derived_status：按 §26 映射为用户语言文案。
- 创建者(组) / 当前处理人(组)：`姓名 · 组名`；hover 显示部门。
- 创建时间 / 更新时间：相对时间（如"2 小时前"），hover 显示绝对时间。
- 操作列：仅"查看详情"；快捷操作不放在列表，统一放详情页。
- 卡片布局需避免大面积留白，桌面端使用响应式网格，多字段以紧凑 meta 行展示。

### 4.4 可复合筛选（§3.2）
- 类型、优先级、`derived_status`、创建者/接单人（按组过滤）、日期范围、关键字。
- 关键字命中：任务号 / 产品编码 / 标题 模糊。
- 筛选参数**同步 URL query string**，支持分享 URL。

### 4.5 分页与刷新
- 无限加载 50/页，滚动到底部加载下一页；"刷新列表"按钮重置到第一页。
- WebSocket `task_pool_count_changed` 推送到达时，任务池 tab 的数字 badge 实时变化；当前列表不自动插入新行（避免打断操作），给出"列表有更新，点此刷新"的悬浮提示。

### 4.6 排序
- 不做自定义排序列头；三类排序固定（见 §4.2）。
- 未来 R7+ 再考虑。

---

## 5. 任务池 Tab 与接单交互

### 5.1 任务池 Tab 差异
- 列出的是 **Module 维度** 的 pending 记录（非任务维度），每条卡片/行显示 `任务号 + 模块名(design/audit/customization/procurement/retouch/warehouse)`。
- 若同一任务有多个并行 pending 模块（极少），各自一条。
- 过滤默认按当前 `actor.TeamCodes` 匹配 `pool_team_code`（后端已筛，前端不再二次过滤）。

### 5.2 接单按钮行为
- 点击"接单" → `POST /v1/tasks/{id}/modules/{module_key}/claim` body `{ confirm_pool_team_code: "<actor主组code>" }`。
- 成功：
  - 列表条目消失或直接跳详情页（默认**跳详情页**，方便立即开工）。
  - WebSocket 后续会推 `task_pool_count_changed` 给同组其他成员。
- 失败：
  - `module_claim_conflict` → Toast "该任务已被他人领取"，自动把该条从列表移除。
  - `module_blueprint_missing_team` → 错误横幅"池组配置缺失，请联系管理员"，不移除条目。
  - `module_state_mismatch` → Toast "该模块状态已变化，请刷新"。
  - `module_action_role_denied` → 按钮应事先隐藏；若确实触达此错误，Toast "你没有权限接单此任务"。

### 5.3 多组用户接单语义
- `claimed_team_code` 由后端根据 actor 与 pool 的交集自动确定（主文档 §7.4 Q7.4）。
- 前端在接单前的 `confirm_pool_team_code` 仅做客户端校验，不参与组归属推断。

---

## 6. 任务详情页

### 6.1 入口与请求
- 入口：`/tasks/:id`。
- 单次调用：`GET /v1/tasks/:id/detail` 获取 `{ task, modules[], timeline, comments }`（一次返回，**禁止**二次请求）。
- 验收指标（主文档 A10）：详情页一屏呈现所有实例化模块，无二次请求。

### 6.2 布局

```
┌──────────────────────────────────────────────────────────────────────┐
│ 任务号 · 类型 tag · 优先级 tag · derived_status badge · 截止时间      │
│ 发起人 · 创建时间 · 当前责任人     [编辑basic] [作废] [关闭] [复制]    │
├────────────────────────────────────────────────────┬─────────────────┤
│ 左主体 70%（按 blueprint 顺序渲染模块卡片）         │ 右栏 30%         │
│  § basic_info                                     │  § 状态时间线      │
│  § design (若实例化)                              │   (task_module    │
│  § retouch (若实例化)                             │    _events)       │
│  § customization (若实例化)                       │  § 当前责任人卡    │
│  § audit (若实例化)                               │  § 评论 / 备注    │
│  § procurement (若实例化)                         │  § 锚点导航       │
│  § warehouse (若实例化)                           │                  │
│  § 历史版本汇总(跨模块)                            │                  │
│  § 操作日志(折叠)                                  │                  │
└────────────────────────────────────────────────────┴─────────────────┘
```

- 未实例化模块（`deny_code=module_not_instantiated`）整块不渲染。
- 作用域外（`module_out_of_scope`）：卡片内容可读，操作区不渲染。
- 每个模块卡片左上状态 chip（`pending_claim / in_progress / submitted / approved / rejected / closed` 等）+ 右上动作区。

### 6.3 各模块 Panel 设计

#### 6.3.1 `basic_info` Panel

**数据来源**：`module.projection` + task 元数据。

**展示字段**：
- 通用：任务号、任务类型、优先级、截止时间、发起人+所属运营组、备注。
- 原款：ERP 产品快照（名称/尺寸/分类）、产品尺寸、修改要求。
- 新款（单 SKU）：产品分类编码、新品 SKU、产品名称、规格尺寸、设计需求说明。
- 新款（批量 SKU）：任务级类目 / 材料 / 成本模式 / 基础价 / 任务级设计要求 / 任务级参考图；商品列表（折叠表格，展示后端回写的 N 条 items 的 SKU/产品名/规格尺寸/需求摘要）。
- 采购：采购 SKU、产品名称、规格尺寸、数量、成本计价方式、成本（若手动录入）。
- 精修（P 图任务）：原图、修改要求。
- 客户定制：线上订单号、下单时间、ERP 产品编码（+ ERP 快照卡）、客户附件（`owner_module_key=customization` 的参考挂在定制模块，仅需求/订单信息留在 basic_info）。
- 常规定制：设计源文件快照（文件名/版本号/来源任务号/预览缩略图）、需求说明。

**参考图/附件区域**：按 `owner_module_key=basic_info` 挂载（资产归属 §3）。

**动作（allowed_actions）**：
- `update_basic_info`：创建者/运营组长/运营管理员/SuperAdmin。
- `update_reference_files`：同上；接单后只增不删（前端根据 `allowed_actions` 控制按钮）。
- `update_deadline` / `update_priority`：同上。
- `cancel_task`：未接单前；创建者 + 本组组长。
- `close_task`：DeptAdmin 任意节点（强制填原因，弹出 `<CancelReasonModal>`）。

#### 6.3.2 `design` Panel（仅原款/新款实例化）

**展示**：
- 资产版本列表（每版：`v{n}` + 文件名 + uploader 姓名 + 时间 + 备注），按 `version_no` 降序。
- 当前接单人 + 组。

**动作**：
- `claim` → 接单。
- `submit` → 提交；提交前校验"是否至少有一个版本"。
- `reassign` → 组长/部门管理员；弹组内成员选择器。
- `asset_upload_session_create` → 上传新版本，走统一上传组件（§15）。

**状态映射到展示状态**：见 §26。

#### 6.3.3 `retouch` Panel（仅 P 图任务）

- 同 design 结构，但卡片标题"精修"，不展示审核相关动作。
- 池组 = `design_retouch`，接单后 `claimed_team_code` 固定 `design_retouch`。

#### 6.3.4 `customization` Panel（仅客户定制 / 常规定制）

**对齐定制工作流 §7 呈现草图**：

```
┌── customization 模块卡片 ─────────────────────────────────────┐
│ 状态徽章: in_progress                                        │
│ 接单人: 李四 · customization_art 组 · 2026-04-10             │
│                                                             │
│ 需求信息(只读，来自 basic_info 输入字段)                      │
│  ├ [客户定制] 订单号 / ERP产品编码 / 下单时间 / ERP快照卡     │
│  └ [常规定制] 设计源文件快照(文件名/版本/来源任务)             │
│                                                             │
│ 客户附件 / 定制参考（owner_module_key=customization）         │
│                                                             │
│ 资产版本列表（按 version_no 降序）                            │
│   v3  终稿 v3.psd · 李四 · 2026-04-12                        │
│   v2  终稿 v2.psd · 李四 · 2026-04-11 (已被驳回)             │
│   v1  沟通稿.jpg · 李四 · 2026-04-10                         │
│                                                             │
│ 动作按钮（按 allowed_actions 呈现）                           │
│   [上传新版本] [提交审核] [改派]                              │
└─────────────────────────────────────────────────────────────┘
```

**动作（对齐定制工作流 §2.2）**：
- `customization.claim`：成员/组长/部门管理员（定制美工部）。
- `customization.reassign`：组长 + 部门管理员。
- `customization.pool_reassign`：仅部门管理员。
- `customization.asset_upload_session_create / _complete / _cancel`：`claimed_by` 本人或本组组长。
- `customization.submit`：仅 `claimed_by` 本人（`self_only`）。
- `customization.reopen`：**系统自动触发（audit.rejected）**，前端**不渲染此按钮**。

**reopen 呈现**：
- 卡片顶部显示红色 banner "该模块已被审核驳回，驳回原因：XXX（{审核人} · {时间}）"，引用 `task_module_events` 的最新 `reopened` 事件 payload。
- 若原 `claimed_by` 已停用，banner 附加"原接单人已停用，已退回任务池等待重新接单"（对应 `reopen_fallback_to_pool` 事件）。

#### 6.3.5 `audit` Panel（原款/新款/客户定制/常规定制）

**展示**：
- **当前资产版本 = 最新版本 (latest)**（资产归属 §4.3）。
- 审核历史记录：每条 audit 事件旁显示"审核当时快照 = v{M}"，点击可下载该历史版本；若 410 GONE，降级显示"该版本资产已按保留策略清理，归档时间 XXX"（§17）。
- **审核参考图区域**（主文档 Draft v4 §6 Layer 3 注）：独立渲染在资产版本列表下方，标题"审核参考"，来源 `reference_file_refs.owner_module_key=audit`。审核人可在此挂说明图/标注稿，与上游成品资产物理隔离。

**动作**：
- `claim` → 接单。
- `approve` → 通过；blueprint 自动触发 `warehouse.enter`。
- `reject` → 打回；强制弹出 `<RejectReasonModal>` 填原因：
  - 原款/新款 → 后端 blueprint 触发 `design.reopen`，前端 Toast "已驳回至设计"。
  - 客户定制/常规定制 → 后端触发 `customization.reopen`，前端 Toast "已驳回至定制"。
  - 前端**不提供**"打回任意人/任意节点"的自由选择（blueprint 驱动；保留 @评论做软通信）。
- `reassign` → 组长/审核管理员。
- `update_reference_files` → **审核参考图**上传/替换/删除入口（审核阶段 `in_progress` 状态下可用；`claimed_by` 本人 + 本组组长 + 审核管理员）。前端在 audit Panel 顶部渲染「上传审核参考」按钮并限定 `owner_module_key=audit`，**严格走 `update_reference_files` 动作，不走 `asset_upload_session_create`**（主文档 Draft v4 §6 Layer 3 明确；参考图不是资产，不参与版本流）。
- **不渲染** `asset_upload_session_create`（主文档 §6 明确：审核阶段不上传资产；只能挂参考图）。

**参考图 vs 资产 UI 区分**（防止误操作）：

| 区域 | 来源 | 上传入口 | 版本化 |
| --- | --- | --- | --- |
| 资产版本列表 | 上游 design / retouch / customization 产出 | 无（audit 阶段只读） | 是（`version_no` 线性递增） |
| 审核参考 | `reference_file_refs.owner_module_key=audit` | 「上传审核参考」按钮（`update_reference_files`） | 否（覆盖式替换） |

#### 6.3.6 `procurement` Panel（仅采购任务）

- 展示：采购 SKU、数量、规格尺寸、成本计价方式、成本。
- 若 `pricing_mode=template` 且未计算出成本，展示"成本将在接单时自动计算"占位。
- 动作：`claim / submit / reassign`。

#### 6.3.7 `warehouse` Panel（所有产品类任务）

- 展示：上游最终资产集合（聚合 design/retouch/customization 的最新版本）+ 接收状态 + 归档编号（若有）。
- 动作：`start` → `receive` → `archive` → `reject`（驳回上游）。
- 一键"打包下载 ZIP"（后端异步打包，通知中心推通知 `archive_ready`）。

#### 6.3.8 历史版本汇总

- 跨 module 聚合：参考图 / 设计稿 / 精修稿 / 定制稿 / 审核稿 / 最终稿。
- 分组展示 + 版本对比（图对比）。

#### 6.3.9 状态时间线（右栏）

- 数据源：`task_module_events`。
- 事件类型 i18n：
  - `entered` "进入 {module}"
  - `claimed` "{姓名} 接单"
  - `submitted` "{姓名} 提交"
  - `approved` "{姓名} 通过"
  - `rejected` "{姓名} 驳回（原因：...）"
  - `reopened` "自动回流（关联驳回事件 #{id}）"
  - `reopen_fallback_to_pool` "原接单人已停用，退回池"
  - `reassigned` "组长 {A} 改派 {B} → {C}"
  - `pool_reassigned_by_admin` "部门管理员跨组调度"
  - `pool_reassigned` "部门管理员跨组调度"
  - `migrated_from_v0_9` "历史数据迁移"
  - `asset_auto_cleaned` "资产按保留策略清理"
  - `asset_deleted_by_admin` "SuperAdmin 删除资产（原因：...）"
  - `task_cancelled` "任务已作废（原因：...）"
  - `forcibly_closed` "任务作废联动强制关闭"
  - `close_task` "部门管理员关闭任务（原因：...）"

#### 6.3.10 评论 / 备注

- 所有登录用户可评论（不等于可操作任务）。
- 已归档任务评论区整体**只读**（对齐 §30 F-U11 决策：不开"新评论须先解归档"通道）。
- 支持 `@成员` **仅做 UI 高亮**（v1 决策，对齐信息架构 Draft v3 §8.1）：
  - 前端识别 `@xxx` 语法，将命中用户名展示为高亮 chip；悬停显示被 @ 用户头像 + 姓名 + 所在组 mini 卡。
  - **不生成 `task_mentioned` 通知**、**不推送 WebSocket**、**不写 `notifications` 行**。`task_mentioned` 作为 `notification_type` 延后到 v1.x 再评估。
  - `@` 的联想下拉数据源：`GET /v1/users?department=&team=&keyword=` 前端节流 300ms；低权限（Member/TeamLead）看不到 `users` 分组时联想降级为"仅本组成员 + 任务相关模块接单人 + 任务创建者"。
- 评论数据模型沿用现有 `task_comments` 表（若暂无后端支持则不阻塞 v1 发布，视研发排期降级为"无评论区"隐藏区块，由 Feature Flag `FEATURE_V1_COMMENTS` 控制）。

### 6.4 任务级动作

| 动作 | 入口 | 权限 | 端点 | 请求体 | 确认弹窗 |
| --- | --- | --- | --- | --- | --- |
| 编辑 basic_info | 详情页顶部 "编辑基础信息" | `update_basic_info` | `POST /v1/tasks/{id}/modules/basic_info/actions/update_basic_info` | — | 内嵌表单，保存即生效 |
| 作废（cancel） | 顶部菜单「作废」 | 任何模块 `pending_claim` 之前；创建者 / 本组组长 | `POST /v1/tasks/{id}/cancel` | `{ reason, force: false }` | `<CancelReasonModal>`（必填 reason） |
| 关闭（close） | 顶部菜单「关闭」 | DeptAdmin / SuperAdmin 任意节点 | `POST /v1/tasks/{id}/cancel` | `{ reason, force: true }` | `<CloseReasonModal>`（必填 reason，二次确认） |
| 复制任务 | 顶部菜单 | 创建者角色 | 走创建弹窗 + `source_draft_id`（若希望落草稿） | — | 克隆 basic_info + 参考图，生成新 Draft 跳转创建页 |

#### 6.4.1 作废 / 关闭共用端点（主文档 Draft v4 §9.1.1）

**前端规则**：

- **两个产品按钮（作废 / 关闭）共用同一端点 `POST /v1/tasks/{id}/cancel`**，仅通过 body 的 `force` 字段区分语义。没有独立的 `close` 端点。
- **按钮可见性**：
  - `作废`：未接单前（`basic_info` 以外的模块均处于 `pending_claim` 或未实例化）；展示给任务创建者 / 本组运营组长。
  - `关闭`：任意节点；仅对 `DeptAdmin` / `SuperAdmin` 展示（按 `allowed_actions` 或 actor 角色判断；两者之一即可渲染）。

**409 分流（关键）**：

- 前端点击「作废」→ `POST cancel { force: false }`。
- 若返回 `409 task_already_claimed`（后端判定存在已接单模块），前端**不直接失败**：
  1. Toast 提示"任务已被接单，无法作废"
  2. 若 actor 是 `DeptAdmin` / `SuperAdmin`，弹出二次确认模态："是否走"关闭"流程强制终止任务？（将级联强制关闭所有运行中模块）"，确认后以同 reason 重发 `{ force: true }`。
  3. 若 actor 非 DeptAdmin+，提示"请联系部门管理员执行关闭操作"。
- 「关闭」按钮直接发 `{ force: true }`，不做上述二次确认（按钮本身已仅对 DeptAdmin+ 可见）。

**事件读取**：

- 详情页 `task_module_events` 时间线区分：
  - `task_cancelled` → 图标/文案"任务已作废（{reason}）"（`force=false` 路径）
  - `forcibly_closed` → 图标/文案"任务被强制关闭（{reason}）"（`force=true` 路径，每个被级联的模块一条）
- 前端无需额外拉取；`GET /v1/tasks/{id}/detail` 返回的 `derived_status` 将相应切换到 `cancelled` 或 `closed`，由 §26 展示状态映射表呈现。

---

## 7. 创建任务弹窗总控

### 7.1 弹窗骨架

```
┌──────────────── 创建任务 ────────────────┐
│ [一级 tag] 常规任务 | 定制任务            │
│ [二级 tag]                               │
│   常规:原款开发|新款开发|采购任务|P图任务 │
│   定制:客户定制|常规定制                 │
│ ───────────────────────────────────── │
│                                         │
│   主表单区（按二级 tag 动态切换，800px）  │
│                                         │
│ ───────────────────────────────────── │
│              [取消] [保存草稿] [提交]    │
└────────────────────────────────────────┘
```

**全局规则**：
- **一律去除右侧"创建总览"**，所有任务类型使用统一弹窗宽高，不再因单品 / 批量切换不同规格。
- 单 SKU 任务主表单 ≤ 一屏；批量（仅新品开发）走紧凑 Excel 流程（§10），未解析前仅展示下载模板 / 上传 / 解析入口。
- 创建弹窗字段采用紧凑布局：控制字段间距、textarea 行数与提示文案高度，避免短文案占用大面积留白。
- 创建态上传字段使用按钮式入口，按钮旁展示已上传数量与"单文件 ≤300MB"提示；已上传内容用小缩略图或紧凑列表展示。详情页可继续使用完整上传组件（§15）。
- tag 切换：同大类切换保留字段；跨大类切换弹确认 "将丢失已填内容，是否继续？"。
- **保存草稿**：`POST /v1/task-drafts`（信息架构 Draft v3 §3.5.9），覆盖所有 7 种创建弹窗，详见 §7.4。
- **关闭弹窗拦截**：字段非空时关闭弹窗 → 弹出三选一对话框「保存为草稿 / 丢弃 / 取消」（对应 IA-A13）。
- 提交：事务内创建 Task → blueprint 实例化 modules → 入池；成功后跳详情页。若由草稿进入，请求体携带 `source_draft_id`，后端事务内级联删除该草稿（IA-A15）。

### 7.2 全局通用字段（对齐主文档 G1 + 定制工作流 Draft v2 §3.1.1 / §3.2.1）

- `task_deadline`：**所有 7 种任务类型必填**（含客户定制 / 常规定制；定制工作流 Draft v2 已显式对齐）。
- `remark`：所有任务类型可选（含两类定制任务）。
- `task_priority`：所有任务类型可选，UI 按**当前登录用户角色**差异化渲染：

  | actor 角色 | 控件 | 取值 |
  | --- | --- | --- |
  | `Member` / `TeamLead` | **单开关 "是否加急"** | OFF → `normal`；ON → `urgent` |
  | `DepartmentAdmin` / `SuperAdmin` | **下拉** `normal / urgent / critical` 三选一 | 默认 `normal` |

  - **`critical` 仅 DeptAdmin+ 可见可选**（对齐定制工作流 Draft v2 §3.1.1 注、IA 决策 F-U3）。
  - Member / TeamLead 视角下 `critical` 下拉项**不渲染**；若服务端 `task_priority=critical` 来自已有草稿/历史记录，在低权限用户角度以只读徽章 `🔴 critical`（不可编辑）呈现。
  - 提交时前端再次校验：actor 为 Member / TeamLead 的请求体若出现 `task_priority=critical`，前端拦截并弹出"当前角色不可设置 critical 优先级"。
  - `critical` 的自动 SLA 升级（normal → urgent → critical）保留给 R7+（主文档 U6），本轮不做。

### 7.3 任务编号前缀表（前端不生成，只在成功后展示）

| task_type | 前缀 | 来源 |
| --- | --- | --- |
| original_product_development | 按现有规则 | 后端 |
| new_product_development | 按现有规则 | 后端 |
| purchase_task | 按现有规则 | 后端 |
| retouch_task | 按现有规则 | 后端 |
| customer_customization | `CC-YYYYMM-NNNNNN` | 定制工作流 §3.1.3 |
| regular_customization | `RC-YYYYMM-NNNNNN` | 定制工作流 §3.2.3 |

### 7.4 保存草稿（`/v1/task-drafts`）——覆盖 7 种创建弹窗

> 信息架构 Draft v3 §3.5.9 新增端点族；v1 必做能力；对齐 IA-A13 ~ IA-A16。

#### 7.4.1 适用范围

全部 7 种创建弹窗均提供统一的 `[保存草稿]` 按钮，位于弹窗底部 `[取消] [保存草稿] [提交]` 三联按钮的中间位：

| # | 任务类型 | 对应弹窗 / 页面 |
| --- | --- | --- |
| 1 | `original_product_development` | §8 原款开发 |
| 2 | `new_product_development`（单 SKU） | §9 新款开发单 SKU |
| 3 | `new_product_development`（批量 SKU） | §10 批量 SKU · Excel 入口 |
| 4 | `purchase_task` | §11 采购任务（仅单 SKU） |
| 5 | `retouch_task` | §12 P 图任务 |
| 6 | `customer_customization` | §13 客户定制 |
| 7 | `regular_customization` | §14 常规定制 |

#### 7.4.2 前端消费端点

| 端点 | 场景 | Body / Query |
| --- | --- | --- |
| `POST /v1/task-drafts` | 新建或更新草稿；body shape 与 `POST /v1/tasks` 完全一致（可带任意未填完的字段），额外可携带 `draft_id`（有则更新） | `{ draft_id?, task_type, payload: { ...同 POST /v1/tasks } }` |
| `GET /v1/me/task-drafts?task_type=&limit=&cursor=` | 个人中心"我的任务 · 草稿"子 tab 列表 | — |
| `GET /v1/task-drafts/{draft_id}` | 回填创建弹窗 | — |
| `DELETE /v1/task-drafts/{draft_id}` | 用户手动删除 | — |
| `POST /v1/tasks` with `source_draft_id` | 正式创建时携带；后端事务内级联删除草稿（IA-A15） | body 含 `source_draft_id` |

#### 7.4.3 交互规则

1. **按钮状态**：无任何字段输入时「保存草稿」按钮为 disabled；至少 1 个字段输入即解禁。
2. **保存行为**：
   - 首次保存 → `POST /v1/task-drafts`（无 `draft_id`）→ 成功后回写 `draft_id` 到本地状态；后续点击「保存草稿」走更新路径（带 `draft_id`）。
   - 批量 SKU（§10）模式下，**任务级字段 + 已解析的 Excel 预览结果**一并落入 `payload`；若 Step 3/4 已产生 `violations`，预览表与错误同样随草稿保留，下次回填直接恢复到 Step 4。
   - 客户定制 / 常规定制的 ERP 或设计源文件查询结果可保存（允许草稿存在未通过强阻校验的状态；但再次提交正式任务时仍需通过 §13.2 / §14.2 的强阻逻辑）。
3. **关闭拦截**（对应 IA-A13）：字段非空时关闭弹窗触发对话框：

   ```
   ┌─ 表单未提交 ─────────────────┐
   │ 检测到您填写了部分字段，        │
   │ 如何处理？                     │
   │                              │
   │  [保存为草稿] [丢弃] [取消]     │
   └─────────────────────────────┘
   ```
   - `保存为草稿` → 静默走 POST `/v1/task-drafts` 后关闭；
   - `丢弃` → 直接关闭（若之前已保存过草稿，草稿仍保留，不删除，用户可去"草稿"tab 手动删除）；
   - `取消` → 留在弹窗。
4. **成功提交级联删除**：弹窗正式点击「提交」创建任务时，若存在 `draft_id`，请求体注入 `source_draft_id`；后端在事务内删除该草稿（IA-A15）。前端成功回调无需额外 `DELETE`。
5. **批次约束（IA-A16）**：同一用户同一 `task_type` 活跃草稿上限 **20 条**；超上限时后端先删最老，后插入新的。前端在草稿列表顶部展示 `N / 20` 用量，用量 ≥ 18 时给出"接近草稿上限"提示。
6. **过期策略（IA-A14）**：草稿 **7 天**过期后硬删；前端列表展示"剩余 N 天"；过期的草稿不再出现在列表（后端定时 job 已删）。

#### 7.4.4 回填逻辑

- 从"个人中心 → 我的任务 → 草稿"tab 或任务中心顶部"继续编辑草稿"快捷入口点击某条草稿 →`GET /v1/task-drafts/{draft_id}` → 按 `task_type` 打开对应创建弹窗，并把 `payload` 字段灌入表单状态。
- 若草稿中引用的参考图 OSS 对象已被"孤儿文件清理 job"清理（信息架构 §3.5.9 末段说明），前端在文件列表位置降级显示"此附件已清理，请重新上传"灰态条目，不阻断草稿继续编辑。

#### 7.4.5 不做（与子文档保持对齐）

- 草稿**不跨用户共享**、不支持协同编辑。
- 草稿**不参与**全局搜索 / 报表统计（§21 全局搜索与 §23 报表均不包含草稿）。
- 草稿不产生任务号，不入池，不触发通知。

---

## 8. 原款开发创建弹窗（单）

### 8.1 字段（保留 v2 方案并保持与主文档 §4 一致）

| # | 字段 | 标记 | 控件 | 必填 | 说明 |
| --- | --- | --- | --- | --- | --- |
| 1 | 选择产品（ERP） | 保留 | ERP 搜索选择器 | 是 | 选中后回填产品名称/尺寸/建议分类 |
| 2 | 产品名称 | 逻辑变更 | 输入 | 是 | 可覆盖 |
| 3 | 产品尺寸 | 新增 | 输入 | 否 | 默认 ERP 值 |
| 4 | 修改要求 | 保留 | 富文本 | 是 | ≤ 2000 字 |
| 5 | 参考图/附件 | 保留/逻辑变更 | 上传组件 | 否 | 挂 basic_info |
| 6 | 任务截止时间 | 保留 | 日期时间 | 是 | |
| 7 | 是否加急 | 新增 | 开关 | 否 | |
| 8 | 备注 | 保留 | 多行 | 否 | |

**删除**：所属运营组（系统自动）/ 指派设计师（走池接单）/ 优先级（改"是否加急"）。

### 8.2 ERP 选品联动
- 接口：`GET /v1/erp/products/by-code?code=...` 或已有 `GET /v1/erp/products/search?keyword=...`（由后端提供）。
- 回填后标记"已绑定 ERP（code: XXX）"；若用户二次修改产品名称/尺寸，提交时提示"与 ERP 不一致"（不阻断）。
- 差异保存到 `basic_info.data.erp_origin_snapshot`。

### 8.3 Blueprint 与池
- task_type = `original_product_development`，blueprint `basic_info → design → audit → warehouse`；`design` 池 `design_standard`，`audit` 池 `audit_standard`。

---

## 9. 新款开发（单 SKU）创建弹窗

### 9.1 字段

| # | 字段 | 标记 | 控件 | 必填 | 说明 |
| --- | --- | --- | --- | --- | --- |
| 1 | 产品分类编码 | 逻辑变更 | 自由输入（历史联想） | 是 | 由下拉改为自由填写 |
| 2 | 新品 SKU | 逻辑变更 | 只读 `提交后自动生成` | — | 后端生成 |
| 3 | 产品名称 | 保留 | 输入 | 是 | ≤ 64 字 |
| 4 | 规格尺寸 | 保留 | 输入 | 否 | |
| 5 | 设计需求说明 | 保留 | 富文本 | 是 | ≤ 2000 字 |
| 6 | 参考图/附件 | 保留/逻辑变更 | 上传组件 | 否 | 挂 basic_info |
| 7 | 任务截止时间 | 保留 | 日期时间 | 是 | |
| 8 | 是否加急 | 新增 | 开关 | 否 | |
| 9 | 备注 | 保留 | 多行 | 否 | |

**删除**：产品材质 / 产品简称 / 产品参考链接 / 所属运营组 / 指派设计师 / 优先级。

### 9.2 提交后
- 成功后跳详情页，顶部醒目展示生成的新品 SKU + "复制 SKU" 按钮。

---

## 10. 新款开发（批量 SKU · Excel 唯一入口）⭐

> **本节对齐信息架构 §3.5，完全替代上一版"录入模板 + 新增/管理商品弹窗"设计。**

### 10.1 作用范围
- **仅**任务类型 `new_product_development` 的"批量 SKU"子入口。
- 单品 SKU 入口保留不变。
- **其他任务类型**（原款/精修/客户定制/常规定制/采购）**不提供** Excel 批量入口——本轮采购任务只保留单 SKU 创建（详见 §11）。

### 10.2 页面结构（创建页·批量 SKU 模式）

```
┌────────────────── 创建任务 · 新款开发 · 批量 SKU ──────────────────┐
│                                                                   │
│ ─ 任务级字段（统一填写一次，不进 Excel） ─                          │
│    类目:        [自由输入/下拉]                                    │
│    材料模式:    [下拉]                                            │
│    成本模式:    [下拉]                                            │
│    基础价:      [数字]                                            │
│    任务级设计要求: [富文本]                                        │
│    任务级参考图:  [上传组件（多图）]                               │
│    任务截止时间:  [日期时间]                                       │
│    是否加急:      [开关]                                          │
│                                                                   │
│ ─ SKU 列表（Excel 四步走）─                                       │
│   Step 1  [下载 SKU 模板 (.xlsx)]                                 │
│   Step 2  本地填写 Excel                                          │
│   Step 3  [上传 Excel ↑]                                          │
│   Step 4  预览表（只读，错误标红）                                 │
│           行1: SKU=xxx 产品名=xxx ... ✓                            │
│           行2: ⚠ 第3列"规格尺寸"不能为空                            │
│           行3: ⚠ 第5列枚举"材料模式"值不在字典                     │
│           ...                                                     │
│           [重新上传]  [取消本次导入]                               │
│                                                                   │
│                         [取消] [保存草稿] [提交]                   │
└───────────────────────────────────────────────────────────────────┘
```

**前端禁用项**（§3.5.7）：
- 禁止保留/渲染"多行内联编辑器"组件（行新增/行删除/行内校验/跨行参考图挂载）。
- 禁止暴露竞品链接引用 / 变量轴矩阵 / 混合模式 / 竞品自动抓取入口。
- 禁止为其他任务类型开放 Excel 批量入口。

### 10.3 四步交互细节

**Step 1 · 下载模板**
- 点击"下载 SKU 模板"：`GET /v1/tasks/batch-create/template.xlsx?task_type=new_product_development`。
- 后端动态生成多 sheet Excel：

  | Sheet | 内容 |
  | --- | --- |
  | `SKU 数据` | 列 = 新品开发单品 SKU 必填字段 1:1 映射（随代码演进自动跟随） |
  | `填写说明` | 每列含义 / 是否必填 / 取值规则（静态） |
  | `字典·类目` | 从 `category_*` 查询（动态） |
  | `字典·材料模式` | 当前材料模式枚举（动态） |
  | `字典·成本模式` | 当前成本模式枚举（动态） |

- `SKU 数据` 的枚举列使用 Excel "数据校验 → 序列" 引用字典 sheet，离线填写有下拉。
- 文件名：`新款开发-批量SKU模板-{YYYYMMDD}.xlsx`。

**Step 2 · 本地填写**
- 用户在 Excel 中逐行填写；离线下拉校验。
- 前端在此期间保持创建弹窗状态不变；支持"保存草稿"先存任务级字段。

**Step 3 · 上传 Excel**
- 点击"上传 Excel"：`POST /v1/tasks/batch-create/parse-excel`（multipart）。
- **解析端点不创建任务**，只返回预览与错误（§3.5.4）。
- 返回结构：

  ```json
  {
    "preview": [
      { "row": 1, "values": { "sku": "NP-xxx", "product_name": "...", ... } },
      ...
    ],
    "violations": [
      { "row": 2, "column": "spec_size", "code": "REQUIRED_MISSING", "message": "不能为空" },
      { "row": 3, "column": "material_mode", "code": "ENUM_MISMATCH", "message": "值不在字典" }
    ]
  }
  ```

- 前端解析响应 P95 < 2s（IA-A11 验收）。

**Step 4 · 预览 + 错误定位 + 确认**
- 预览表**只读**；错误行整行标红。
- 鼠标悬停错误单元格显示浮窗：`行X 列Y 字段名：错误原因`。
- 预览区顶部错误 summary："共 5 行，其中 2 行存在错误"。
- 按钮：
  - `[重新上传]`：清空预览，回到 Step 3。
  - `[取消本次导入]`：清空预览，保留任务级字段。
- **不提供就地修改**（避免维护两套字段口径）；运营必须回到本地 Excel 修正重新上传。
- 错误全部清零后，"提交"按钮解禁。

**提交**
- 走**现有** `POST /v1/tasks`，body 中 `batch_sku_mode=multiple` + `items[]` 来自解析结果 + 任务级字段。
- 后端在事务内为每个 item 生成 SKU（NP-...），实例化 `basic_info + design(pending_claim, pool=design_standard)`。
- 成功后跳详情页，批量商品以内嵌表格呈现。

### 10.4 校验与边界
- SKU 列表不能为空（0 行）→ banner "请先下载模板并上传至少一条 SKU"。
- 任务级必填项缺失 → 底部 banner。
- Excel 文件大小限制：建议 ≤ 20MB（超出本地拦截，提示使用分批）。
- 解析失败（文件非 xlsx / 格式损坏）→ Toast "无法解析 Excel 文件，请确认使用最新模板"。

### 10.5 验收（信息架构 §3.5.8 → IA-A9 ~ A12）
- IA-A9：批量 SKU 模式下**不再**展示旧多行内联编辑器。
- IA-A10：模板 Excel 列集合与单品 SKU 必填字段 100% 一致（依赖后端动态生成）。
- IA-A11：上传 100 行 Excel 解析 P95 < 2s。
- IA-A12：解析端点不创建任务，仅 `POST /v1/tasks` 创建。

---

## 11. 采购任务（单 SKU）创建弹窗

> **本轮采购任务不提供批量入口**（信息架构 §3.5.1 显式排除）。

### 11.1 字段

| # | 字段 | 标记 | 控件 | 必填 | 说明 |
| --- | --- | --- | --- | --- | --- |
| 1 | 产品分类编码 | 新增 | 自由输入 | 是 | |
| 2 | 采购 SKU | 逻辑变更 | 只读 `提交后自动生成` | — | |
| 3 | 产品名称 | 保留 | 输入 | 是 | |
| 4 | 规格尺寸 | 改名 | 输入 | 是 | 原"规格说明" |
| 5 | 数量 | 保留 | 数字 | 是 | ≥ 1 |
| 6 | 成本计价方式 | 改名/逻辑变更 | 下拉 `手动录入/按模板` | 是 | 原"成本来源" |
| 7 | 成本 | 改名/条件必填 | 数字 | 条件必填 | 仅"手动录入"时显示并必填 |
| 8 | 任务截止时间 | 保留 | 日期时间 | 是 | |
| 9 | 是否加急 | 新增 | 开关 | 否 | |
| 10 | 备注 | 保留 | 多行 | 否 | |

**删除**：产品渠道 / 基本售价 / 所属组 / 优先级。

### 11.2 成本计价方式联动

| 选项 | UI | 后端时机 |
| --- | --- | --- |
| 手动录入 | "成本"可见必填 | 提交时即 `procurement.data.cost` |
| 按模板 | 隐藏"成本"输入；显示提示"将按产品尺寸规则在采购接单后自动计算" | `procurement.claim` 时后端计算并回写 |

### 11.3 Blueprint 与池
- task_type = `purchase_task`，blueprint `basic_info → procurement → warehouse`；`procurement` 池 `procurement_main`。

---

## 12. P 图任务创建弹窗

### 12.1 字段

| # | 字段 | 必填 | 说明 |
| --- | --- | --- | --- |
| 1 | 图片/附件上传 | 是 | ≥ 1 张；格式 jpg/png/tif/pdf/psd/ai/webp/bmp/gif；单张 ≤ 300MB |
| 2 | 修改要求 | 是 | 富文本；支持粘贴图做标注 |
| 3 | 任务截止时间 | 是 | 默认 now + 2h |
| 4 | 是否加急 | 否 | 默认 OFF |
| 5 | 备注 | 否 | |

### 12.2 Blueprint 与池
- task_type = `retouch_task`，blueprint `basic_info → retouch → warehouse`（**不走 audit**，主文档 §4 说明 4.1）。
- `retouch` 池 `design_retouch`。
- 前端文案统一为"P 图任务"；后端命名沿用 `retouch_task`。

### 12.3 详情页呈现
- 仅渲染：basic_info / retouch / warehouse / 日志 / 评论。
- design / audit / customization / procurement 全部不渲染。

### 12.4 扩展位
- 若未来业务需要审核，blueprint 加 `retouch.submitted → audit.enter` 即可，前端按 `modules[]` 驱动自动渲染审核区。

---

## 13. 客户定制创建弹窗（ERP 编码查询联动）

### 13.1 字段（严格对齐定制工作流 §3.1.1）

| # | 字段 | 必填 | 控件 | 说明 |
| --- | --- | --- | --- | --- |
| 1 | 线上订单号 (`online_order_no`) | 是 | 输入 | 运营手填 |
| 2 | 下单时间 (`ordered_at`) | 是 | 日期时间 | ≤ now |
| 3 | ERP 产品编码 (`erp_product_code`) | 是 | 输入 + 「从 ERP 查询」按钮 | 必须经 `/v1/erp/products/by-code` 校验 |
| 4 | ERP 产品快照 | 只读 | 卡片 | 查询成功后回填 |
| 5 | 需求信息 (`requirement_note`) | 是 | 富文本 | |
| 6 | 客户需求附件 (`attachments[]`) | 否 | 上传 | 挂 `owner_module_key=customization` |
| 7 | 任务截止时间 | **是** | 日期时间 | 通用字段（定制工作流 Draft v2 已显式对齐 G1） |
| 8 | 优先级 / 是否加急 | 否 | Member/TeamLead 为开关；DeptAdmin+ 为下拉 | 通用字段；`critical` 仅 DeptAdmin+ 下拉可见（详见 §7.2） |
| 9 | 备注 | 否 | 多行 | 通用字段 |

> **不包含**：组合编码、计价等级、客户确认等字段。

### 13.2 ERP 查询联动（对齐定制工作流 §3.1.2）

- 点击「从 ERP 查询」 → `GET /v1/erp/products/by-code?code={erp_product_code}`。
- **失败策略（强阻）**：
  - 上游 5xx / timeout：Toast "ERP 查询失败，请稍后重试"，**禁用提交按钮**。
  - "未找到"：Toast "该产品编码不存在于 ERP，请核实后重试"，**禁用提交按钮**。
- 查询成功：
  - 返回字段渲染为只读快照卡（产品名称 / 规格 / 分类 / 供应商 / ...）。
  - 标志位 `erpVerified=true`，解禁提交按钮。
- 提交时再次校验 `erpVerified === true`，双保险防止绕过。
- 任务与 ERP 通过 `erp_product_code` 锚定；ERP 侧后续变更不回流到任务快照。

### 13.3 提交后
- task_no 前缀：`CC-YYYYMM-NNNNNN`。
- blueprint 实例化：`basic_info(active) + customization(pending_claim, pool=customization_art)`。
- `audit / warehouse` 由 `customization.submitted / audit.approved` 触发。

### 13.4 详情页呈现
- basic_info（客户信息子卡片：订单号/下单时间/ERP 编码 + ERP 快照） / customization（定制交付）/ audit / warehouse / 日志 / 评论。
- **不渲染** design。

---

## 14. 常规定制创建弹窗（设计源文件查询联动）

### 14.1 字段（严格对齐定制工作流 §3.2.1）

| # | 字段 | 必填 | 控件 | 说明 |
| --- | --- | --- | --- | --- |
| 1 | 设计源文件 (`design_source_lookup_id`) | 是 | 设计源文件查询控件 | 必须命中 `/v1/design-sources/search` 结果 |
| 2 | 设计源文件快照 (`design_source_snapshot`) | 自动回填 | 只读卡片 | 回填字段 `{ id, file_name, preview_url, owner_team_code, created_at, version_no, origin_task_id }` |
| 3 | 需求信息 (`requirement_note`) | 是 | 富文本 | |
| 4 | 任务截止时间 | **是** | 日期时间 | 通用字段（定制工作流 Draft v2 已显式对齐 G1） |
| 5 | 优先级 / 是否加急 | 否 | Member/TeamLead 为开关；DeptAdmin+ 为下拉 | 通用字段；`critical` 仅 DeptAdmin+ 下拉可见（详见 §7.2） |
| 6 | 备注 | 否 | 多行 | 通用字段 |
| 7 | （可选）定制参考附件 | 否 | 上传 | 挂 `owner_module_key=customization`，作为补充说明资料 |

> **v1 硬约束**：`design_source_lookup_id` **单选**（定制工作流 Draft v2 §3.2.1）。前端查询结果列表仅允许选中 1 条；重选时自动替换。v1 不开多选口子。
> **不包含**：组合编码、计价等级、订单号、ERP 查询、附图（业务通用附件不与客户需求附件同列）。

### 14.2 设计源文件查询（对齐定制工作流 §3.2.2）

**组件**：`<DesignSourcePicker>`，嵌入表单。

- 关键字搜索输入 → `GET /v1/design-sources/search?keyword={q}&page=&size=`。
- 结果列表展示字段：`file_name / preview_url(缩略图) / owner_team_code / created_at / version_no / origin_task_id`。
- 按创建时间倒序；v1 MVP 仅支持文件名 / 任务号关键字搜索。
- 单选；选中后：
  - 卡片化回显选中源文件。
  - 标志 `sourceVerified=true`，解禁提交。
  - 点"更换源文件"可重新搜索。
- 无结果：显示"未找到匹配的设计源文件，请联系设计研发部"。
- 数据源说明文案："来源于设计部产出的源文件与已归档任务的资产"。

### 14.3 提交后
- task_no 前缀：`RC-YYYYMM-NNNNNN`。
- blueprint 同客户定制：`basic_info → customization → audit → warehouse`；`customization` 池 `customization_art`，`audit` 池 `audit_customization`。

### 14.4 详情页呈现
- basic_info（原图引用卡：设计源文件快照 + 来源任务链接） / customization / audit / warehouse / 日志 / 评论。
- **不渲染** design。

### 14.5 客户定制 vs 常规定制差异

| 维度 | 客户定制 | 常规定制 |
| --- | --- | --- |
| task_type | `customer_customization` | `regular_customization` |
| 前缀 | `CC-` | `RC-` |
| 线上订单号 / 下单时间 / ERP 产品编码 | 必填 | 不存在 |
| ERP 查询 | 必做 | 不需要 |
| 设计源文件查询 | 不需要 | 必填 |
| 客户附件 | 常有 | 可选（定制参考附件） |
| blueprint | 完全一致 | 完全一致 |
| customization 池 | `customization_art` | `customization_art` |
| audit 池 | `audit_customization` | `audit_customization` |

---

## 15. 上传组件（统一口径）

### 15.1 规则

| 维度 | 值 |
| --- | --- |
| 允许格式 | 设计类：ps / ai / jpg / jpeg / png / tif / tiff / pdf / plt / cdr；P 图补充：webp / bmp / gif |
| 单文件大小 | ≤ 300MB（本地即拦截，Toast "单张不超过 300MB"） |
| 单批数量 | ≤ 50 文件 |
| 分片 | ≥ 100MB 强制分片（OSS 直传） |
| 鉴权 | STS 临时凭证（后端签发），前端不持有主 AK |
| 失败策略 | 单文件失败不影响整批；失败项列顶 + 支持"重试/移除" |
| 断点续传 | 支持 |
| 预览 | 图片 Lightbox；PDF 内嵌；PSD/AI/CDR/PLT 显示文件图标 + 下载 |
| 打包下载 | 云仓区或资产详情页"打包下载 ZIP"（后端异步打包 + 通知中心 `archive_ready`） |

### 15.2 挂载语义（资产归属 §3.2）

| 业务位置 | owner_module_key |
| --- | --- |
| 任务级参考图（`context=task_reference_upload`） | `basic_info` |
| SKU 级参考图（批量 SKU items 关联） | `basic_info` |
| 审核参考（`context=audit_reference`） | `audit` |
| 定制参考（`context=customization_review_reference / customization_attachment`） | `customization` |
| 客户定制订单附件 | `customization` |
| 设计稿源文件 / 成品 | `design` |
| 精修成品 | `retouch` |
| 定制终稿 | `customization` |
| 云仓归档回写 | `warehouse`（v1 无独立资产，仅引用） |

### 15.3 `<UploadCenter>` Props

- `ownerModuleKey`: 传入当前模块 key，用于区分挂载点。
- `context`: 更细分语义（如 `task_reference_upload / audit_reference / customization_attachment`）。
- `taskId` / `taskModuleId`：关联上下文。
- `allowedFormats`: 默认集合 + 任务类型覆盖。
- `maxSingle`: 300MB。
- `maxBatch`: 50。
- `onUploaded(asset)`: 回调。

---

## 16. 参考图 owner_module_key 前端归属

### 16.1 渲染规则
- 详情页按 `owner_module_key` 分组：
  - `basic_info` → basic_info Panel 内"参考图/附件"区。
  - `audit` → audit Panel 内"审核参考"区。
  - `customization` → customization Panel 内"客户附件/定制参考"区。
- 资产可见性 = 所在模块的 Layer 2 作用域（资产归属 §3.3）。

### 16.2 下载
- **任务内部**下载：Layer 1 可见即可下载（不再在前端按组别过滤）。
- **资产管理中心**下载：同上。
- **分享链接**：预签名 URL，默认有效期 1h；URL 到期自动失效（资产归属 §8）。

---

## 17. 审核 always-latest + 历史快照 + 410 GONE 降级

### 17.1 当前审核页
- 显示"当前资产版本 = v{N}（latest）"。
- 点击预览/下载走最新资产（资产归属 §4.3）。

### 17.2 审核历史记录
- 每条 audit 事件旁显示"审核当时快照 = v{M}"（来自 `task_module_events.payload.asset_versions_snapshot`）。
- 点击"下载该版本"：
  - 若版本仍在 OSS 中 → 正常下载。
  - 若返回 **410 GONE**（已被 365 天自动清理，资产归属 §7.4）→ 前端降级显示浮窗："该版本资产已按保留策略清理，归档时间 {cleaned_at}。请联系管理员是否需从冷备恢复。"

### 17.3 事件 payload 读取
- `asset_versions_snapshot[]` 中的 `storage_key` 可能为 null（已清理），但 `asset_id / version_id / version_no / file_name` 仍可读。
- 前端兜底：若 `storage_key=null`，自动替换下载按钮为"该版本已清理"灰态。

---

## 18. 资产管理中心页面

### 18.1 定位
- 一级菜单，所有登录用户可见（资产归属 §5.1 / §5.4）。
- 与任务详情页内的 `/v1/tasks/{id}/asset-center/*` 并存（后者是任务内入口，前者是跨任务视图）。

### 18.2 列表页

```
/assets
[筛选栏]
  关键字: (file_name / 任务号 / 标题)
  模块: 全部 | design | retouch | customization | audit | basic_info | warehouse | procurement
  归属组: (team_code 选择器)
  是否归档: 默认"否"；可选 "是 / 全部"
  任务状态: open | closed | archived | all
  创建时间: [起] ~ [止]
[操作]  [重置] [筛选]

[列表]
  缩略图 | 文件名 | 模块 | 归属组 | 来源任务号 | 最新版本 | 创建时间 | 归档状态 | 操作
  [📎]   foo.psd  design design_standard YB-2026-00123 v3 2026-04-10 active [下载][详情]
  ...
```

- 查询：`GET /v1/assets/search?keyword=&module_key=&owner_team_code=&is_archived=&task_status=&created_from=&created_to=&page=&size=`。
- 分页或无限滚动。

### 18.3 详情页

```
/assets/:id
┌───────────────────────────────────────────────┐
│ 文件名 · 模块 · 归属组 · 来源任务链接            │
│ [下载最新版本]  [查看版本列表]                  │
│ [归档] [恢复] [删除]  ← 仅 SuperAdmin 可见      │
├───────────────────────────────────────────────┤
│ 版本列表（按 version_no 降序）                  │
│  v3  file_v3.psd · 张三 · 2026-04-12 · [下载]   │
│  v2  file_v2.psd · 张三 · 2026-04-11 · [下载]   │
│  v1  file_v1.psd · 张三 · 2026-04-10 · [下载]   │
└───────────────────────────────────────────────┘
```

### 18.4 SuperAdmin 动作

| 动作 | API | 必填 |
| --- | --- | --- |
| 归档 | `POST /v1/assets/{asset_id}/archive` | 原因 |
| 恢复 | `POST /v1/assets/{asset_id}/restore` | — |
| 删除 | `DELETE /v1/assets/{asset_id}` | 原因（写 `asset_deleted_by_admin` 事件） |

- 三者均要求二次确认弹窗 `<ReasonConfirmModal>`。
- 非 SuperAdmin 角色按钮**不渲染**。

### 18.5 筛选默认值
- `is_archived=0`（默认不显示归档资产）。
- `task_status=all`。
- 关键字 + 模块 + 归属组组合过滤。

---

## 19. 组织菜单（用户 / 部门 / 组）

### 19.1 页面树

```
/org                                    组织总览（首页 kpi）
/org/users                              用户列表 + 增删改 + 启停
/org/departments                        部门列表
/org/teams                              组列表
/org/move-requests                      跨部门调配（SuperAdmin 待办）
```

### 19.2 用户列表（`/org/users`）

**筛选**：部门、组、关键字（姓名/账号/邮箱）、角色、是否启用。

**操作**（按字段级授权矩阵 §5.4 渲染按钮）：

| 操作 | SuperAdmin | HRAdmin | DeptAdmin（本部门） | TeamLead（本组） |
| --- | --- | --- | --- | --- |
| 新建用户 | ✓ | ✓ | ✗ | ✗ |
| 编辑 nickname/phone/email | ✓ | ✓ | ✓ | ✗ |
| 调整 department / team_codes | ✓ | ✓ | ✓（本部门内调组） | ✗ |
| 跨部门移动 | ✓（直改） | ✓（直改） | 通过 move-request | ✗ |
| 修改 roles | ✓ | ✓（不授 SuperAdmin） | ✓（不授 DeptAdmin） | ✗ |
| 启停账号 | ✓ | ✓ | ✓（本部门） | ✓（本组） |
| 删除 | ✓ | ✗ | ✗ | ✗ |

**字段级冲突 UI**：若当前用户只有"启停"权限（TeamLead），则 `PATCH /v1/users/{id}` 其他字段输入框全部 `readonly + hover 提示"无权限"`。

**跨部门修改**：DeptAdmin 若在编辑界面修改成员的 `department` 为本部门以外 → 前端弹窗："跨部门移动需通过'移出部门'流程发起，并由 SuperAdmin 确认"，拦截后跳转到"发起移出"流。

### 19.3 跨部门移动工作流（对齐信息架构 §5.2）

**DeptAdmin 侧（源部门）**：
- 用户详情页按钮"移出部门"，弹窗选择"目标部门/组"（可选），填"原因"。
- 提交 → `POST /v1/departments/{id}/org-move-requests` → 状态 `pending_super_admin_confirm`。
- 发起后列表该用户显示"调配中"灰色 Badge。

**SuperAdmin 侧**：
- `/org/move-requests` 列表展示所有 `pending_super_admin_confirm` 请求。
- 对每条可 `approve / reject`：
  - approve → `POST /v1/org-move-requests/{id}/approve` → 成员 department/team_codes 更新 + 审计事件。
  - reject → `POST /v1/org-move-requests/{id}/reject` → 通知发起人。

### 19.4 部门列表（`/org/departments`）

| 操作 | SuperAdmin | HRAdmin |
| --- | --- | --- |
| 新建部门 | ✓ | ✗ |
| 重命名 | ✓ | ✓ |
| 删除 | ✓（需清空成员，否则 409） | ✗ |

### 19.5 组列表（`/org/teams`）

| 操作 | SuperAdmin | HRAdmin | DeptAdmin（本部门） |
| --- | --- | --- | --- |
| 新建组 | ✓ | ✓ | ✓ |
| 修改（名称/描述）| ✓ | ✓ | ✓ |
| 删除 | ✓ | ✓ | ✗ |

- 删除组：前端先查"组内成员数 + 活跃任务占用 pool_team_code"，两者非 0 即弹窗提示"请先清空成员并释放活跃任务"，避免触发 409。

### 19.6 TeamLead 只读视图
- Member 不可见"组织"菜单（信息架构 §2）。
- TeamLead 进入 `/org` 仅看本组用户列表，**除启停账号外全部 readonly**。

---

## 20. 个人中心（头像下拉）

头像点击展开下拉：

```
┌─────────────────────────┐
│ 👤 当前用户姓名           │
│    所属部门 / 主组          │
├─────────────────────────┤
│ 👤 账户信息              │  → /me
│ 🔒 安全（改密）           │  → /me/security
│ 🏢 我的组织（只读）        │  → /me/org
│ 📋 我的任务              │  → /tasks?tab=mine
│ 📥 我的待接单             │  → /tasks?tab=pool
│ 🔔 通知中心              │  → /me/notifications
├─────────────────────────┤
│ ⎋ 退出登录              │
└─────────────────────────┘
```

### 20.1 账户信息（`/me`）
- 字段：nickname / phone / email / 头像。
- API：`GET /v1/me` / `PATCH /v1/me`。

### 20.2 安全（`/me/security`）
- 改密表单：旧密码 + 新密码 + 确认新密码。
- API：`POST /v1/me/change-password`。
- 校验：新密码强度规则（8+ 位 + 含字母和数字）。

### 20.3 我的组织（`/me/org`）
- **只读**。展示：部门 / 组列表 / 当前角色。
- 若是 DeptAdmin / TeamLead，额外展示 `managed_departments` / `managed_teams`。
- API：`GET /v1/me/org`。

### 20.4 我的任务 / 我的待接单

- 跳转任务中心对应 tab，不建独立页。
- **"我的任务"子 tab 结构（信息架构 Draft v3 §7.2 更新）**：

  | 子 tab | 数据源 | 备注 |
  | --- | --- | --- |
  | 进行中 | `GET /v1/tasks?filter=mine&status=in_progress` | 默认激活；任一模块 `claimed_by=actor_id` 或 task.creator_id=actor_id 且未终态 |
  | 已完成 | `GET /v1/tasks?filter=mine&status=closed` | 含归档 |
  | **草稿** ⭐（v2 新增） | `GET /v1/me/task-drafts?task_type=&limit=&cursor=` | **对应信息架构 Draft v3 §7.2 新增要求**；展示用户全部 7 种任务类型的活跃草稿 |

- "草稿"子 tab 列表列：

  ```
  任务类型 | 摘要（标题/产品名/SKU/订单号，按 task_type 映射取首要字段）| 最近更新时间 | 剩余过期天数 | 操作（继续编辑 / 删除）
  ```

  - 顶部工具条：任务类型过滤器 + `N / 20` 用量徽章（接近上限 DesignTokens.color.warning 配色提示）。
  - 点击「继续编辑」→ `GET /v1/task-drafts/{draft_id}` → 按 `task_type` 打开对应创建弹窗并回填（详见 §7.4.4）。
  - 点击「删除」→ `DELETE /v1/task-drafts/{draft_id}`；二次确认弹窗「此操作将永久删除草稿，无法恢复」。
  - 空态文案："暂无草稿。你在创建任务时点击"保存草稿"即可在这里继续编辑。"

### 20.5 通知中心 → §22。

### 20.6 不做（信息架构 §7.3）
- 偏好（工作台排序 / 主题 / 默认过滤）。
- 我的数据 / 报表。
- 登录历史 / Token 管理。

---

## 21. 全局搜索（Ctrl+K）

### 21.1 入口
- 顶部导航栏右侧放大镜图标。
- 快捷键 `Ctrl+K` / `Cmd+K`（Mac）打开全屏 overlay。

### 21.2 交互
- 输入框自动 focus；输入 1 字符即开始搜索（debounce 300ms）。
- `ESC` 关闭。
- 键盘 `↑/↓` 切换结果，`Enter` 打开。

### 21.3 作用域切换
- 顶部 tag：`all / tasks / assets / products / users`（默认 all）。
- API：`GET /v1/search?q=&scope=&limit=20`。

### 21.4 结果分组展示

```
┌─ 🔍 搜索 "YB-2026"                        ESC关闭 ─┐
│                                                   │
│ 📋 任务 (3)                                       │
│   YB-2026-00123 · 原款开发 · 李四                  │
│   YB-2026-00124 · 新款开发 · 张三                  │
│   ...                                             │
│                                                   │
│ 📎 资产 (5)                                       │
│   CC-2026-main.psd · design · 常规设计组           │
│   ...                                             │
│                                                   │
│ 🏷 产品 (2)                                       │
│   SKU-xxx · 产品名称                              │
│   ...                                             │
│                                                   │
│ 👥 用户（仅 DeptAdmin+ 可见）                       │
└───────────────────────────────────────────────────┘
```

- `users` 对象仅 `DeptAdmin` 及以上可见；Member/TeamLead 直接不渲染该分组（API 返回空数组）。
- 高亮命中关键字。

### 21.5 点击跳转
- 任务 → `/tasks/:id`。
- 资产 → `/assets/:id`。
- 产品（ERP 快照）→ 显示只读详情浮窗（无独立详情页）。
- 用户 → `/org/users/:id`（DeptAdmin+）。

### 21.6 性能指标（信息架构 IA-A3）
- P95 < 300ms（10w 任务规模）。前端若超 600ms 显示"搜索较慢，正在查询..."。

---

## 22. 通知中心 + WebSocket

### 22.1 通知中心页面（`/me/notifications`）

**布局**：

```
┌──────────────────────────────────────┐
│ 通知中心                [标记全部已读] │
├──────────────────────────────────────┤
│ [筛选] 全部 | 未读                     │
├──────────────────────────────────────┤
│ 🔵 task_assigned_to_me                │
│   你被组长 {姓名} 改派到任务 YB-2026-00123  │
│   原因：... · 2h 前                    │
│   [标记已读]                          │
│                                      │
│ 🔴 task_rejected                      │
│   你承接的任务 CC-202604-000012 被审核驳回  │
│   原因：... · 5h 前                    │
│   [查看任务]                          │
│                                      │
│ ⚪ task_assigned_to_me (已读)          │
│   ...                                │
└──────────────────────────────────────┘
```

### 22.2 通知类型（信息架构 Draft v3 §8.1）

| type | 触发 | payload |
| --- | --- | --- |
| `task_assigned_to_me` | 被 reassign | `{ task_id, module_key, assigned_by, reason }` |
| `task_rejected` | 我作为 claimed_by 的模块被 audit 驳回 | `{ task_id, reject_reason }` |
| `claim_conflict` | 接单抢先（占位） | `{ task_id, module_key }` |
| `pool_reassigned` | 我所在组新增 pool 任务（可配置） | `{ task_id, module_key }` |
| `task_cancelled` | 我参与的任务被作废 / 关闭（`task_cancelled` 或 `forcibly_closed` 级联触发） | `{ task_id, cancel_reason, cancelled_by, force }` |

> **`task_mentioned` v1 不做**（IA Draft v3 §8.1 明确）：评论区 `@` 仅做 UI 高亮（详见 §6.3.10），前端**不推送通知**、**不写 `notifications` 行**、**不订阅 WebSocket**。`task_mentioned` 延后到 v1.x 再评估。

### 22.3 API

| 端点 | 说明 |
| --- | --- |
| `GET /v1/me/notifications?is_read=&limit=&cursor=` | 列表 |
| `POST /v1/me/notifications/{id}/read` | 标记已读 |
| `POST /v1/me/notifications/read-all` | 全部已读 |
| `GET /v1/me/notifications/unread-count` | 右上 badge |

### 22.4 WebSocket（信息架构 §9）

**连接**：`/ws/v1?token=<bearer>`。

**推送事件（仅 3 类）**：

| type | 推送给 | 用途 |
| --- | --- | --- |
| `task_pool_count_changed` | 池组成员（按 team_code 订阅） | 任务池 tab 数字实时变 |
| `my_task_updated` | 具体 user_id | 我的任务列表实时变 |
| `notification_arrived` | 具体 user_id | 头像 badge 实时 +1 |

**消息格式**：`{ "type": "...", "payload": {...} }`。

**降级**：WebSocket 断线时，前端自动启用 **15s 轮询**：
- `/v1/me/notifications/unread-count`：更新 badge。
- 任务池 tab：重新 `GET /v1/tasks/pool`。

**不做**：
- 所有任务列表广播。
- 报表卡片广播（R5 单独评估）。
- 外部推送（企微/钉钉/邮件）—— 本轮仅站内（Q7.1）。

---

## 23. 报表菜单（SuperAdmin · L1 卡片）

### 23.1 首页（`/reports`）

```
┌────────────────── 报表 · L1 实时卡片 ──────────────────┐
│ [刷新] [说明]                                          │
│                                                       │
│ ┌────────────┐ ┌────────────┐ ┌────────────┐          │
│ │  待接单(池) │ │ 设计进行中   │ │ 审核驻留平均  │          │
│ │    125     │ │    48       │ │   4.3h     │          │
│ └────────────┘ └────────────┘ └────────────┘          │
│                                                       │
│ ┌────────────┐ ┌────────────┐                        │
│ │ 今日归档    │ │ 今日新建    │                        │
│ │    23      │ │    56      │                        │
│ └────────────┘ └────────────┘                        │
└───────────────────────────────────────────────────────┘
```

- 数据源：v1 直接 query `task_module_events`（信息架构 §8.4）。
- 实时：WebSocket 推送任务池计数 / 我的任务变更（仅 2 类推向个人端；报表卡片目前 R5 评估前 fallback 30s 轮询刷新）。
- 权限：菜单仅 SuperAdmin 可见；直接 URL 访问被非 SuperAdmin 触达时跳 `/403`。

### 23.2 子页
- `/reports/task-throughput` 任务吞吐看板（卡片 + 趋势图）。
- `/reports/module-dwell` 模块驻留时长（箱线图/直方图，R5 扩展）。
- `/reports/performance` 个人/组效能（R5+ 预留，v1 不做）。

---

## 24. 前端组件化与目录结构建议

### 24.1 建议目录

```
src/
├── layouts/
│   └── AppShell.vue                顶部导航 + 路由 outlet
├── components/
│   ├── base/                        通用基础
│   │   ├── UploadCenter.vue         统一上传（分片/STS/300MB）
│   │   ├── FileIconFallback.vue     (已存在)
│   │   ├── CancelReasonModal.vue
│   │   ├── CloseReasonModal.vue
│   │   ├── RejectReasonModal.vue
│   │   ├── ReasonConfirmModal.vue
│   │   └── ErpProductPicker.vue     ERP 编码查询控件
│   ├── task-card/                   任务中心卡片/行
│   │   ├── TaskListRow.vue          表格行
│   │   └── TaskListCard.vue         卡片模式（可选）
│   ├── task-create/                 创建弹窗族
│   │   ├── TaskCreateModal.vue      总控
│   │   ├── forms/
│   │   │   ├── OriginalForm.vue
│   │   │   ├── NewProductSingleForm.vue
│   │   │   ├── NewProductBatchForm.vue   ← Excel 四步走
│   │   │   ├── PurchaseSingleForm.vue
│   │   │   ├── RetouchForm.vue
│   │   │   ├── CustomerCustomizationForm.vue
│   │   │   └── RegularCustomizationForm.vue
│   │   ├── ExcelBatchSkuPanel.vue    四步走独立组件
│   │   ├── DesignSourcePicker.vue    设计源文件查询
│   │   ├── SaveDraftButton.vue       统一"保存草稿"按钮（共享 useTaskDraft）
│   │   ├── DraftListView.vue         个人中心 · 草稿子 tab
│   │   └── CloseDraftConfirmModal.vue  关闭弹窗三选一（保存/丢弃/取消）
│   ├── task-detail/                  详情页
│   │   ├── TaskDetailLayout.vue      双栏骨架
│   │   ├── ModuleSection.vue         通用模块容器
│   │   ├── modules/
│   │   │   ├── BasicInfoPanel.vue
│   │   │   ├── DesignPanel.vue
│   │   │   ├── RetouchPanel.vue
│   │   │   ├── CustomizationPanel.vue
│   │   │   ├── AuditPanel.vue
│   │   │   ├── ProcurementPanel.vue
│   │   │   └── WarehousePanel.vue
│   │   ├── TimelineSidebar.vue
│   │   ├── CommentsSidebar.vue
│   │   └── HistoryVersionGroup.vue
│   ├── asset-center/                  资产管理中心
│   │   ├── AssetListView.vue
│   │   └── AssetDetailView.vue
│   ├── org/                           组织菜单
│   │   ├── UsersView.vue
│   │   ├── DepartmentsView.vue
│   │   ├── TeamsView.vue
│   │   └── MoveRequestsView.vue
│   ├── global-search/
│   │   └── GlobalSearchOverlay.vue    Ctrl+K 浮层
│   ├── notification/
│   │   ├── NotificationCenter.vue
│   │   └── NotificationBadge.vue
│   └── reports/
│       ├── ReportsHome.vue
│       ├── TaskThroughputView.vue
│       └── ModuleDwellView.vue
├── composables/
│   ├── useTaskDetail.ts              拉取 + 解析 modules[] + allowed_actions
│   ├── useModulePermission.ts        按 scope/allowed_actions 决定渲染
│   ├── useWebSocket.ts               /ws/v1 + 15s 轮询回退
│   ├── useUploadSession.ts           OSS 直传分片
│   ├── useErpProduct.ts              by-code 查询
│   ├── useDesignSourceSearch.ts      /v1/design-sources/search
│   ├── useTaskDraft.ts               /v1/task-drafts 全端点族（新建/更新/读取/删除/回填）
│   ├── useTaskCancel.ts              /v1/tasks/{id}/cancel 双语义端点（force 切换 + 409 分流）
│   └── useNotifications.ts           unread-count + 列表
├── services/api/
│   ├── tasksApi.ts
│   ├── assetsApi.ts
│   ├── searchApi.ts                   /v1/search
│   ├── erpApi.ts                      /v1/erp/products/by-code
│   ├── designSourcesApi.ts            /v1/design-sources/search
│   ├── notificationsApi.ts
│   ├── orgApi.ts
│   ├── batchSkuApi.ts                 /v1/tasks/batch-create/*
│   └── taskDraftsApi.ts               /v1/task-drafts · /v1/me/task-drafts
├── stores/
│   ├── auth.store.ts                  actor + menus
│   ├── tasks.store.ts
│   ├── taskDrafts.store.ts            草稿列表 / 单条 / 用量徽章
│   └── notifications.store.ts
├── views/
│   ├── TaskListView.vue
│   ├── TaskDetailView.vue
│   ├── TaskCreateView.vue
│   ├── AssetsIndexView.vue
│   ├── AssetDetailView.vue
│   ├── OrgView.vue (+ 下四个子)
│   ├── ReportsView.vue (+ 子)
│   └── profile/ (...)
└── router/
    └── index.ts
```

### 24.2 已存在组件（不删除，部分需改造）

从 git status 可见已存在以下组件，改造策略：

| 组件 | 改造策略 |
| --- | --- |
| `AuditAssetPanel.vue` / `AuditQueuePanel.vue` / `AuditWorkbenchView.vue` | 保留作为兼容路由入口（主文档 §9.3 "保留 3 个迭代周期"），R4 后下线 |
| `DesignWorkbenchView.vue` | 同上 |
| `CustomizationJobsView.vue` / `CustomizationJobDetailView.vue` | 同上；客户定制/常规定制新链路走 TaskCreate + TaskDetail |
| `OutsourceView.vue` | 同上，R4 下线 |
| `WarehouseView.vue` / `WarehouseReceiptPanel.vue` | 功能迁入 TaskDetail 的 WarehousePanel；旧视图保留兼容 |
| `AssetsIndexView.vue` / `TaskAssetsView.vue` | 改造为新资产管理中心页面或保留任务内视图 |
| `TaskCreateModal.vue` / `TaskCreateView.vue` | 按 §7-§14 重构 |
| `TaskCreateNewProductForm.vue` | 新品单 SKU 逻辑保留；批量入口切换到 ExcelBatchSkuPanel |
| `TaskSkuTemplateSection.vue` / `BatchSkuItemEditor.vue` / `AddBatchItemsDialog.vue` | **R3 后移除**（旧多行内联编辑，信息架构 §3.5 IA-A9 要求不再展示） |
| `ReferenceUploadPanel.vue` | 统一到 UploadCenter；加入 owner_module_key 支持 |
| `TaskDetailView.vue` + `task-detail/*` | 改造为 ModuleSection 驱动式渲染 |
| `DashboardView.vue` | 首页重写：我的待接单 + 我的进行中 + 我发起的 + 通知入口 |
| `LogsManagementView.vue` | 保留，仅 SuperAdmin 可见（现状）|
| `EvidenceModal.vue` / `CASConflictModal.vue` / `EventLogDrawer.vue` | 复用于详情页 |

### 24.3 Feature Flag 策略
- 引入 `VITE_FEATURE_V1_MODULES=true` 控制新详情页 / 新创建弹窗开关。
- 兼容路由（`/v1/tasks/{id}/audit_a_claim` 等）在 R4 前保留；前端仍向旧 API 回退。
- R3 全员切换后，下次 R4 即可删除 flag 与旧视图。

---

## 25. deny_code → 前端降级处理

对齐主文档 §6.2：

| deny_code | 前端处理 |
| --- | --- |
| `module_not_instantiated` | 整块 Panel 不渲染 |
| `module_out_of_scope` | Panel 内容只读呈现，操作区不渲染 |
| `module_state_mismatch` | 动作按钮隐藏（不是 disable，避免用户误点） |
| `module_action_role_denied` | 按钮隐藏，不弹 toast |
| `module_claim_conflict` | Toast "该任务已被他人领取"，自动从池列表移除该条 |
| `module_blueprint_missing_team` | 顶部红色 banner "池组配置缺失，请联系管理员"，Sentry 上报告警 |
| `task_already_claimed` | `POST /v1/tasks/{id}/cancel` with `force=false` 时后端判定存在已接单模块；前端走 §6.4.1 分流（Toast + DeptAdmin+ 二次确认改走 `force=true`） |
| 任意 4xx 返回 `asset_*` 410 GONE | 资产按钮灰态 "该版本已清理" + 鼠标悬停 tooltip 显示清理时间 |

**兜底**：
- 登录过期（401）→ 自动跳 `/login`。
- 其他未知错误 → Toast "网络异常，请稍后重试" + Sentry 上报。

---

## 26. 展示状态映射表

对齐主文档 §10.1 + 资产 §7.1 + 定制工作流 §2.1。

| 展示状态（用户语言） | derived_status | 颜色 | 附加叠加 |
| --- | --- | --- | --- |
| 待接单 | 当前推进模块 = `pending_claim` | 灰蓝 | |
| 设计进行中 | `design.in_progress` | 蓝 | |
| 精修进行中 | `retouch.in_progress` | 蓝（图标区分） | |
| 定制处理中 | `customization.in_progress` | 紫 | |
| 待审核 | `audit.pending_claim` | 橙 | |
| 审核中 | `audit.in_progress` | 橙深 | |
| 审核打回 | 最近 `audit.rejected` 且下游已 reopen 未再 submit | 红 | |
| 采购中 | `procurement.in_progress` | 棕 | |
| 待接收 | `warehouse.pending` / `warehouse.preparing` | 青 | |
| 已归档 | `warehouse.completed` 或任务归档 | 绿 | |
| 已关闭 | `POST /v1/tasks/{id}/cancel` `force=true` 后 task 置 `closed`（任意节点，DeptAdmin+） | 深灰 | 事件 `forcibly_closed` |
| 已作废 | `POST /v1/tasks/{id}/cancel` `force=false` 后 task 置 `cancelled`（未接单前） | 深灰 | 事件 `task_cancelled` |
| 已逾期（叠加） | now > deadline 且未终态 | 红色小红点 | 与主状态叠加 |

---

## 27. 兼容路由与 Feature Flag

### 27.1 兼容旧路由（主文档 §9.3）
- 旧路由 `/v1/tasks/{id}/audit_a_claim` 等保留 3 个迭代周期。
- R3 期间前端优先调新路由 `/v1/tasks/{id}/modules/*`；若 404 则回退旧路由（保持 UAT 可用）。
- R4 起前端全部收敛到新路由，旧视图整体下线。

### 27.2 Feature Flag 列表

| Flag | 默认 | 用途 | 下线轮次 |
| --- | --- | --- | --- |
| `FEATURE_V1_MODULES` | true (R3) | 启用新详情页 + 新创建弹窗 | R5 后移除 flag，保留行为 |
| `FEATURE_V1_BATCH_EXCEL` | true (R3) | 启用新批量 SKU Excel 入口 | R5 |
| `FEATURE_V1_GLOBAL_SEARCH` | true (R3) | 顶栏全局搜索 | R5 |
| `FEATURE_V1_NOTIFICATIONS` | true (R3) | 通知中心 + WebSocket | R5 |
| `FEATURE_V1_ASSET_CENTER` | true (R3) | 资产管理中心跨任务视图 | R5 |
| `FEATURE_V1_ORG_MENU` | true (R3) | 组织菜单 + 三级授权 | R5 |
| `FEATURE_V1_TASK_DRAFTS` | true (R3) | 创建弹窗"保存草稿" + 个人中心草稿子 tab（`/v1/task-drafts`） | R5 |
| `FEATURE_V1_CANCEL_UNIFIED` | true (R3) | 任务作废 / 关闭共用 `POST /v1/tasks/{id}/cancel`（按 `force` 分流） | R4 |
| `FEATURE_V1_COMMENTS` | false → true | 评论区（含 `@` UI 高亮，无通知） | 依后端排期；不阻塞其他能力 |
| `FEATURE_L1_REPORTS` | true (R5) | 报表 L1 卡片 | R7+ |

---

## 28. 前端验收标准（挑取跨文档条目汇总）

### 28.1 来自主文档
- **A2**：任一登录用户访问 `/v1/tasks/{id}/detail` 返回 200，详情页正常渲染（除硬删除）。
- **A10**：详情页一屏呈现所有实例化模块，无二次请求；创建者可编辑 basic_info，非接单者在 design/customization 等模块只读。

### 28.2 来自信息架构
- **IA-A1**：顶部导航呈现 4 个一级菜单（任务中心 / 组织 / 资产管理中心 / 报表），报表仅 SuperAdmin 可见。
- **IA-A2**：全局搜索对任务号、产品编码、资产文件名关键字均能命中。
- **IA-A3**：`/v1/search?q=` 在 10w 任务规模下 P95 < 300ms；前端兜底 600ms 显示"正在查询..."。
- **IA-A4**：DeptAdmin 对本部门外用户跨部门修改返回 409；前端拦截为"走移出流程"弹窗。
- **IA-A5**：TeamLead PATCH /v1/users/{id} 除 is_active 外任何字段返回 403；前端其他字段 readonly。
- **IA-A6**：头像下拉 6 板块完整（账户 / 安全 / 我的组织 / 我的任务 / 我的待接单 / 通知中心）。
- **IA-A7**：通知中心在 audit 驳回事件 5s 内生成一条 `task_rejected` 通知；前端 badge 实时 +1。
- **IA-A8**：WebSocket 仅推送 3 类事件；池 tab 数字在接单 1s 内更新。
- **IA-A9**：批量 SKU 模式下不再展示旧多行内联编辑器。
- **IA-A10**：模板 Excel 列集合与单品 SKU 必填字段 100% 一致。
- **IA-A11**：上传 100 行 Excel 解析 P95 < 2s。
- **IA-A12**：解析端点不创建任务，仅 `POST /v1/tasks` 创建。
- **IA-A13**（Draft v3 新增）：创建弹窗关闭前若字段非空，前端提示"保存为草稿 / 丢弃 / 取消"三选一。
- **IA-A14**（Draft v3 新增）：草稿列表中 7 天过期项由后端硬删，前端列表不再展示；过期前展示"剩余 N 天"。
- **IA-A15**（Draft v3 新增）：`POST /v1/tasks` body 带 `source_draft_id` 且创建成功后，对应草稿在同事务内删除（前端无需 `DELETE`）。
- **IA-A16**（Draft v3 新增）：同 `owner_user_id + task_type` 草稿数接近 20 时前端提示接近上限；用量徽章正确显示 `N / 20`。

### 28.3 来自资产归属
- **AS-A1**：`task_assets.source_module_key` 所有资产 NOT NULL。
- **AS-A2**：`reference_file_refs.owner_module_key` 按规则回填；前端按归属渲染到对应模块卡。
- **AS-A3**：audit 每条事件展示"审核当时快照"。
- **AS-A4**：资产管理中心 `GET /v1/assets/search` 所有登录用户可用；DELETE 对非 SuperAdmin 返回 403，前端按钮隐藏。
- **AS-A6**：已清理资产下载 410 GONE，前端降级为"该版本资产已清理"灰态。
- **AS-A7**：审核快照在资产清理后仍可读取元数据。

### 28.4 来自定制工作流
- **C-A1**：定制任务详情页不渲染 `design` Panel。
- **C-A2**：客户定制创建表单的 ERP 查询失败时，提交按钮禁用。
- **C-A3**：常规定制创建表单的 `design_source_lookup_id` 必须从 `/v1/design-sources/search` 结果中选择。
- **C-A4**：audit 驳回后，前端 customization Panel 顶部展示 reopen banner（含 `reject_reason` + `reject_audit_event_id` 链接）。
- **C-A5**：原 claimed_by 停用时，customization Panel banner 附加"已退回任务池"文案。
- **C-A6**：迁移后，定制任务详情页不出现 design Panel。
- **C-A7**（Draft v2 新增）：常规定制创建表单 `design_source_lookup_id` 仅支持**单选**；重选替换，UI 不暴露多选控件。
- **C-A8**（Draft v2 新增）：两类定制任务的优先级控件，DeptAdmin+ 可见 `critical` 下拉项，Member/TeamLead 退化为"是否加急"单开关。

### 28.5 本 Draft v2 新增前端自检
- **FE-A1**：7 种创建弹窗底部均渲染 `[保存草稿]` 按钮；无字段输入时 disabled。
- **FE-A2**：弹窗非空状态关闭时触发三选一对话框（保存/丢弃/取消）。
- **FE-A3**：成功提交正式创建时若存在 `draft_id`，body 自动注入 `source_draft_id`。
- **FE-A4**：头像下拉 → 我的任务 → 草稿子 tab 正确消费 `GET /v1/me/task-drafts`，列表含用量徽章与"继续编辑 / 删除"操作。
- **FE-A5**：详情页「作废」/「关闭」按钮共用 `POST /v1/tasks/{id}/cancel`；作废触发 `force=false`，关闭触发 `force=true`；409 正确走二次确认分流。
- **FE-A6**：「关闭」按钮仅 DeptAdmin / SuperAdmin 可见；非高权限用户不渲染。
- **FE-A7**：评论区 `@成员` 仅触发 UI 高亮 + 悬停卡片；不调用通知 API，不在 `notifications` 中生成行。
- **FE-A8**：audit Panel 渲染独立"审核参考"上传入口，走 `update_reference_files`（而非 `asset_upload_session_create`）；文件落到 `reference_file_refs.owner_module_key=audit`。
- **FE-A9**：优先级控件按 actor 角色切换：Member/TeamLead → 开关；DeptAdmin+ → 下拉（含 `critical`）。低权限角色下发 `critical` 请求前端拦截。

---

## 29. 风险与回滚前端侧

| # | 风险 | 等级 | 前端缓解 |
| --- | --- | --- | --- |
| F-1 | 旧"多行内联编辑"在 R3 下线后用户不适应 Excel | 中 | 批量 SKU 页面顶部持续 14 天显示引导横幅 "新版使用 Excel 批量导入，点此查看使用指南"；提供视频/图文 |
| F-2 | 兼容路由期间新旧视图切换引起数据不一致 | 中 | 所有新路由失败时 Sentry 上报 + 自动退回旧路由；R4 前禁止关闭兼容开关 |
| F-3 | 评论 @ 触发的通知类型 v1 不做（IA Draft v3 §8.1 固化） | 低 | 前端仅 UI 高亮 + 悬停 mini 卡；不调通知 API（与 Draft v2 §6.3.10 保持一致） |
| F-4 | 资产中心全员可下载可能泄露敏感文件 | 中 | v1 按架构决策执行；前端日志记录每次下载触发事件，便于追溯 |
| F-5 | ERP / 设计源文件查询首次调用冷启动慢 | 低 | UI loading 骨架 + "首次加载稍慢，请稍候"提示 |
| F-6 | WebSocket 断网后 badge 数字不同步 | 低 | 15s 轮询回退 `/v1/me/notifications/unread-count` |
| F-7 | 报表卡片数据源在 R5 前不稳定 | 低 | 报表页顶部显示"数据可能延迟，刷新重试" |
| F-8 | DeptAdmin 跨部门误操作 | 中 | 修改 department/team_codes 时前端弹"跨部门变更将走 SuperAdmin 确认流"，拦截错误直改 |
| F-9 | TeamLead 调启停账号误操作 | 低 | 启停确认弹窗 + 显示账号用户名 + 文案确认 |
| F-10 | 批量 SKU Excel 文件过大导致页面卡顿 | 低 | 本地预校验文件大小 > 20MB 拦截；上传状态 loading 遮罩 |
| F-11 | 资产 410 GONE 不可恢复 | 低 | 前端文案明确"请联系管理员判断是否从冷备恢复"；不提供前端恢复按钮（SuperAdmin 走后端） |

---

## 30. 待确认项（前端相关）

> **本章 Draft v2 更新**：F-U1 ~ F-U12 均已随"三文档联合签字回执"落定；本章降级为历史决策索引，不再作为"未决"清单。后续若出现新的前端议题，以递增编号追加到 §30.2。

### 30.1 F-U1 ~ F-U12 决策索引（已入档）

| # | 议题 | 最终决策 | 固化位置 |
| --- | --- | --- | --- |
| F-U1 | 任务中心默认视图：表格 vs 卡片 | 采纳默认**表格** | 信息架构 Draft v3 §3.3（已在） |
| F-U2 | 采购任务是否保留批量入口 | **仅单 SKU**，不做批量 | 信息架构 Draft v3 §3.5.1（已在） |
| F-U3 | 定制任务通用字段（`task_deadline` / `task_priority` / `remark`） | **必填 / 可选，与主文档 G1 对齐** | 定制工作流 Draft v2 §3.1.1 / §3.2.1 |
| F-U4 | 评论 @ 是否生成通知 | **v1 不做** `task_mentioned`，前端仅 UI 高亮 | IA Draft v3 §8.1；本文 §6.3.10 / §22.2 |
| F-U5 | 任务池 tab 接单成功后是否跳详情 | **采纳**（接单即跳详情） | 本文 §5.2（前端交互决策，不改文档） |
| F-U6 | 审核"打回任意人/任意节点"废弃的软替代 | **v1 不做**；@UI 高亮即可（与 F-U4 合并） | 同 F-U4 |
| F-U7 | 客户定制 ERP 查询失败是否允许"占位"提交 | **强阻** | 定制工作流 Draft v2 §3.1.2（已在） |
| F-U8 | 常规定制是否允许多选设计源文件 | **v1 单选** | 定制工作流 Draft v2 §3.2.1 |
| F-U9 | P 图任务默认截止时间 2h | **采纳** | 本文 §12.1（前端默认值，不改文档） |
| F-U10 | 敏感字段（订单号/成本）对非相关组是否打码 | **v1 不做**，全量可见 | 前端交互决策，不改文档 |
| F-U11 | 已归档任务的评论是否仍可编辑 | **只读**（不开解归档通道） | 本文 §6.3.10 |
| F-U12 | 顶部 banner "工作台合并" 14 天引导 | **采纳** | 本文 §29 F-1（前端交互决策，不改文档） |

### 30.2 Draft v2 增量待确认项（新）

| # | 议题 | 建议 | 决策方 |
| --- | --- | --- | --- |
| F-U13 | 草稿 tab 是否支持多任务类型批量删除 | 建议 v1 仅支持单条删除，降低误删风险 | 产品 |
| F-U14 | 草稿回填后是否保留 `draft_id` 做二次保存 | 建议保留（详见 §7.4.3），下一次保存走更新路径 | 前端 |
| F-U15 | 作废/关闭 2 次弹窗（409 分流）是否需要再加一级 DeptAdmin 身份口令 | 建议 v1 不加（DeptAdmin 已经是受限角色），后续按安全评估 | 安全/产品 |
| F-U16 | 审核参考图最大数量 | 建议 v1 同资产：≤ 20 张，单张 ≤ 50MB（小于成品资产 300MB） | 产品 + 后端 |
| F-U17 | Member/TeamLead 视角如果看到历史 `critical` 任务，是否允许"降级为 urgent" | 建议 v1 不可修改（只读显示），降级只能由 DeptAdmin+ 进行 | 产品 |

---

## 附录 A：关键 API 一览（前端消费视角）

| API | 用途 | 场景 |
| --- | --- | --- |
| `GET /v1/tasks` | 任务列表 | 任务中心全任务/我的/已归档 |
| `GET /v1/tasks/pool` | 待接单池 | 任务池 tab |
| `GET /v1/tasks/{id}/detail` | 任务详情 | 详情页一次性聚合 |
| `POST /v1/tasks` | 创建任务（含 `batch_sku_mode=multiple`）| 所有创建弹窗 |
| `POST /v1/tasks/{id}/modules/{key}/claim` | 接单 | 任务池 + 详情页 |
| `POST /v1/tasks/{id}/modules/{key}/actions/{action}` | 通用动作触发 | 详情页各模块 |
| `POST /v1/tasks/{id}/modules/{key}/reassign` | 组内改派 | 详情页组长操作 |
| `POST /v1/tasks/{id}/modules/{key}/pool-reassign` | 跨组调度 | 详情页部门管理员 |
| `POST /v1/tasks/{id}/cancel` | **作废 / 关闭共用端点**（主文档 Draft v4 §9.1.1），body `{ reason, force: false \| true }`；409 分流见 §6.4.1 | 详情页 |
| `GET /v1/tasks/batch-create/template.xlsx?task_type=new_product_development` | 批量模板下载 | 批量 SKU Step 1 |
| `POST /v1/tasks/batch-create/parse-excel` | Excel 解析 | 批量 SKU Step 3 |
| `POST /v1/task-drafts` | **新建 / 更新草稿**（IA Draft v3 §3.5.9） | 所有创建弹窗「保存草稿」 |
| `GET /v1/me/task-drafts?task_type=&limit=&cursor=` | 我的草稿列表 | 个人中心 · 草稿子 tab |
| `GET /v1/task-drafts/{draft_id}` | 单条草稿回填 | 继续编辑草稿入口 |
| `DELETE /v1/task-drafts/{draft_id}` | 删除草稿 | 草稿子 tab 行内按钮 |
| `GET /v1/assets/search` | 资产搜索 | 资产中心 + 全局搜索 |
| `GET /v1/assets/{asset_id}` | 资产详情 | 资产中心详情 |
| `GET /v1/assets/{asset_id}/download` | 最新下载 | 各处下载 |
| `GET /v1/assets/{asset_id}/versions/{version_id}/download` | 版本下载 | 审核历史 |
| `POST /v1/assets/{asset_id}/archive` / `restore` / `DELETE` | SuperAdmin 动作 | 资产中心 |
| `GET /v1/search?q=&scope=&limit=` | 全局搜索 | 顶栏 Ctrl+K |
| `GET /v1/erp/products/by-code?code=` | ERP 编码查询 | 客户定制创建 |
| `GET /v1/design-sources/search?keyword=&page=&size=` | 设计源文件查询 | 常规定制创建 |
| `GET /v1/users?department=&team=&keyword=` | 用户列表 | 组织 |
| `POST /v1/users` / `PATCH /v1/users/{id}` / `DELETE /v1/users/{id}` | 用户 CRUD | 组织 |
| `POST /v1/users/{id}/activate` / `deactivate` | 启停 | 组织（TeamLead+ 可调） |
| `POST /v1/departments/{id}/org-move-requests` | 发起跨部门移出 | DeptAdmin |
| `POST /v1/org-move-requests/{id}/approve` / `reject` | SuperAdmin 审 | 组织 |
| `GET /v1/teams` / `POST /v1/teams` / `PATCH /v1/teams/{id}` / `DELETE /v1/teams/{id}` | 组 CRUD | 组织 |
| `GET /v1/me` / `PATCH /v1/me` | 账户信息 | 个人中心 |
| `POST /v1/me/change-password` | 改密 | 个人中心 · 安全 |
| `GET /v1/me/org` | 我的组织 | 个人中心 |
| `GET /v1/me/notifications` | 通知列表 | 通知中心 |
| `GET /v1/me/notifications/unread-count` | 未读数 | 顶栏 badge |
| `POST /v1/me/notifications/{id}/read` / `read-all` | 已读 | 通知中心 |
| `/ws/v1` | WebSocket | 实时推送 |

---

## 附录 B：变更记录

| 版本 | 日期 | 变更 |
| --- | --- | --- |
| Draft v1 | 2026-04-23 | 初稿；基于主文档 Draft v3 + 三份子文档 Draft v1/v2 对齐前端落地 |
| **Draft v2** | **2026-04-23** | 合并三文档联合签字回执（主文档 v4 / IA v3 / 定制工作流 v2 / 资产 v1 无改）：<br>① §6.3.5 audit Panel 新增"审核参考图"上传入口（走 `update_reference_files`，不走 `asset_upload_session_create`），明确参考图 vs 资产 UI 区分；<br>② §6.3.10 评论区 `@` 降级为仅 UI 高亮，不推送通知，不写 `notifications`；<br>③ §6.4 / 新增 §6.4.1 作废 / 关闭共用 `POST /v1/tasks/{id}/cancel`，按 `force` 字段分流；新增 409 `task_already_claimed` 分流逻辑；<br>④ §7.1 ~ §7.4 新增"保存草稿"按钮 + `/v1/task-drafts` 端点族消费逻辑，覆盖全部 7 种创建弹窗；关闭弹窗三选一拦截；`source_draft_id` 级联删除；<br>⑤ §7.2 通用字段 `task_priority` 按 actor 角色分级：Member/TeamLead = 开关，DeptAdmin+ = 下拉（含 `critical`）；<br>⑥ §13.1 / §14.1 客户定制 + 常规定制通用字段对齐定制工作流 Draft v2 §3.1.1 / §3.2.1；`design_source_lookup_id` v1 单选硬约束；<br>⑦ §20.4 个人中心 · 我的任务下新增"草稿"子 tab，含用量徽章与过期天数；<br>⑧ §22.2 通知类型表下方明确 `task_mentioned` v1 不做；<br>⑨ §25 deny_code 表新增 `task_already_claimed`；<br>⑩ §26 展示状态映射区分"已关闭"（`force=true` / `forcibly_closed`）与"已作废"（`force=false` / `task_cancelled`）；<br>⑪ §28 新增验收 IA-A13 ~ IA-A16、C-A7 ~ C-A8、FE-A1 ~ FE-A9；<br>⑫ §30 F-U1 ~ F-U12 标记为"已入档"并附固化位置；新增 F-U13 ~ F-U17 增量议题；<br>⑬ 附录 A 补充 `/v1/task-drafts` 端点族、`/v1/tasks/{id}/cancel` 双语义说明。 |

**签字门槛**：
- 产品：______________________
- 前端：______________________
- 后端：______________________

---

> 本文档为前端落地唯一权威；若与后端实现出现细节分歧，先以本文+4 份源文档为准，必要时走变更流程更新本文与源文档。
