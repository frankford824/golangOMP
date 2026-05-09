# v1.0 收口版 PRD 与前端实现差异分析报告

> 生成日期：2026-04-16
> 对照文档：《v1.0 收口版产品边界说明（正式稿）》
> 分析范围：前端 Vue 项目（`d:\vue\src`）全量代码

---

## 摘要

本报告逐条对照 PRD 中定义的产品边界，与当前前端代码实现进行比对，标注 **吻合 / 偏差 / 缺失** 三种状态，并给出修改建议及风险评估。

**整体结论：** 普通任务主线基本吻合；定制任务主线、审核分组、角色模型、组织管理权限边界、日志可见性规则存在多处偏差或缺失，需要在 v1.0 收口前集中治理。

---

## 目录

1. [角色模型](#1-角色模型)
2. [组织模型（部门与组）](#2-组织模型部门与组)
3. [管理权限边界](#3-管理权限边界)
4. [菜单与页面显隐](#4-菜单与页面显隐)
5. [普通任务主线](#5-普通任务主线)
6. [定制任务主线](#6-定制任务主线)
7. [评审参考值与执行冻结值](#7-评审参考值与执行冻结值)
8. [审核替换与资产追溯](#8-审核替换与资产追溯)
9. [云仓统一入口](#9-云仓统一入口)
10. [日志与系统能力边界](#10-日志与系统能力边界)
11. [兼容逻辑治理](#11-兼容逻辑治理)
12. [总览表](#12-总览表)
13. [风险矩阵](#13-风险矩阵)
14. [建议优先级排序](#14-建议优先级排序)

---

## 1. 角色模型

### PRD 要求

正式保留角色：Ops、DepartmentAdmin、TeamLead、Designer、CustomizationOperator、Warehouse、HRAdmin、SuperAdmin。

审核以"审核部 + 组"表达（普通审核组 / 定制美工审核组），不强调独立角色名。

不再保留的角色：OrgAdmin、Admin 等历史角色。

### 前端现状

| PRD 角色 | 前端 RoleEnum | 前端 Workflow Role Map | 状态 |
|----------|--------------|----------------------|------|
| Ops | `RoleEnum.OPS` = `'ops'` | `'运营' → 'Ops'` | ✅ 吻合 |
| Designer | `RoleEnum.DESIGNER` = `'designer'` | `'设计师' → 'Designer'` | ✅ 吻合 |
| Warehouse | `RoleEnum.WAREHOUSE` = `'warehouse'` | `'仓储' → 'Warehouse'` | ✅ 吻合 |
| SuperAdmin | `RoleEnum.SUPER_ADMIN` = `'super_admin'` | `SuperAdmin → '超级管理员'` | ✅ 吻合 |
| DepartmentAdmin | `RoleEnum.DEPT_ADMIN` = `'dept_admin'` | `'部门管理员' → 'DepartmentAdmin'` | ✅ 吻合 |
| TeamLead | 无独立 RoleEnum 值 | `TeamLead → '组长'`（仅展示层） | ⚠️ 偏差 |
| HRAdmin | 无 RoleEnum 值 | `HRAdmin → '人事管理员'`（仅展示层） | ⚠️ 偏差 |
| CustomizationOperator | 无 RoleEnum 值 | 无映射 | ❌ 缺失 |

**关键偏差：**

1. **`RoleEnum` 中缺少 `CustomizationOperator`。** 前端完全没有该角色的枚举值，`permission.ts` 中无定义，`applyFrontendAccess()` 角色推断逻辑不包含该角色。定制美工在前端无身份标识，无法做独立的菜单/操作控制。

2. **`HRAdmin` 在 `RoleEnum` 中不存在。** `applyFrontendAccess()` 角色推断链为：`super_admin > group_leader > dept_admin > warehouse > auditor > designer > ops`。若后端下发 `roles: ['HRAdmin']`，前端会回落到 `MEMBER`，丢失其应有的管理级权限。PRD 要求 HRAdmin 与 SuperAdmin 同级，但前端代码完全没有这个映射。

3. **`TeamLead` 在 `RoleEnum` 中映射为 `GROUP_LEADER`。** 这个映射在 `applyFrontendAccess` 中通过 `is_group_leader` flag 或 `roles` 包含 `group_leader` 实现，与 PRD 的 `TeamLead` 名称不完全一致但功能可通。然而后端下发的 `TeamLead` 字符串在 lowercase 后变为 `teamlead`，**不等于** `group_leader`，需确认后端是否同时下发 `is_group_leader: true`，否则也会回落。

4. **`Auditor` 审核角色仍作为独立枚举存在。** `RoleEnum.AUDITOR = 'auditor'` 是前端正式的角色枚举值，与 PRD "审核以部门+组表达，不强调独立角色名"的要求矛盾。前端 `isAuditor` computed 判断依赖 `TASK_AUDIT` 权限而非部门归属。

5. **历史角色 OrgAdmin、Admin、RoleAdmin、DesignDirector、DesignReviewer 仍存在于展示映射中。** `WORKFLOW_ROLE_API_LABEL_ZH` 和 `WORKFLOW_ROLE_DISPLAY_TO_API` 仍包含 `OrgAdmin → '组织管理员'`、`Admin → '管理员'`、`RoleAdmin → '角色管理员'`、`DesignDirector → '设计总监'` 等。虽然是展示兼容，但这些角色继续出现在用户管理角色选择器中。

### 修改建议

| 项 | 动作 | 风险 |
|----|------|------|
| 新增 `CustomizationOperator` 到 RoleEnum | 需同时更新 `applyFrontendAccess` 推断链、`scopeMap`、侧边栏 ROLE_LABELS | **中**：需后端同步下发角色标识 |
| 新增 `HRAdmin` 映射逻辑 | 在 `applyFrontendAccess` 中识别 `hradmin`，映射为 `SUPER_ADMIN` 同级权限，或新增独立枚举 | **高**：影响登录后全局权限推断 |
| 确认 TeamLead 后端下发方式 | 若后端 roles 中包含 `TeamLead`（非 `group_leader`），需在推断链补一条 | **低**：可通过 `is_group_leader` flag 兜底 |
| 审核角色治理 | 长期需将审核判断从"有 `task:audit` 权限"改为"属于审核部+某审核组"的组织归属判断 | **高**：涉及审核流程全链路 |
| 清理历史角色展示 | 从 `WORKFLOW_ROLE_DISPLAY_TO_API` / `WORKFLOW_ROLE_API_LABEL_ZH` 移除不再保留的角色 | **低**：纯展示层 |

---

## 2. 组织模型（部门与组）

### PRD 要求

六个正式部门：运营部、设计研发部、定制美工部、审核部、云仓部、人事部。

审核部含两个组：普通审核组、定制美工审核组。运营部含六个组。其余部门各含默认组。

### 前端现状

- 前端 `Department` / `Group` 类型仅为 `{ id, name }` / `{ id, name, departmentId }`，**没有硬编码部门列表**。
- 部门和组结构完全来自 `GET /v1/org/options` 后端接口，前端只做展示和选择器填充。
- `OrgPermissionView.vue` 支持创建部门、创建组、分配成员，但 UI 中没有体现 PRD 中的固定部门体系。

**结论：✅ 基本吻合。** 前端不硬编码组织结构是合理的——部门和组应由后端/管理员维护。只要后端数据库中的部门组结构与 PRD 一致即可。

**但有一个问题：** 审核部的"普通审核组"与"定制美工审核组"的区分在前端审核工作台中 **没有体现**。审核工作台是一个统一队列，不按审核组分流（详见第 5、6 节）。

---

## 3. 管理权限边界

### 3.1 DepartmentAdmin 权限

| PRD 要求的能力 | 前端实现 | 状态 |
|--------------|---------|------|
| 查看本部门全部成员 | `OrgPermissionView` 按部门展示成员 | ✅ |
| 调整本部门成员所属组 | `OrgPermissionView` 支持 `patchUserMembership` | ✅ |
| 归属未分配人员到本部门 | `OrgPermissionView` 候选面板 + 分配操作 | ✅ |
| 创建新账号 | `UserManagementView` 中 `usersApi.create` | ⚠️ 见下 |
| 停用账号 | `UserManagementView` 中 `usersApi.patch` status | ⚠️ 见下 |
| 重置密码 | `UserManagementView` 中 `usersApi.resetPassword` | ⚠️ 见下 |
| 跨组改派本部门任务 | `TaskDetailView` 中改派逻辑 + `task-reassign-policy.ts` | ✅ |
| 查看本部门任务 | `DataScopeEnum.DEPARTMENT` 过滤 | ✅ |

**偏差：** `UserManagementView` 的访问控制用的是 `can('org:manage')`，不是 `isDeptAdmin`。PRD 要求 DepartmentAdmin 可以创建账号、停用账号、重置密码。前端代码的 `org:manage` 能否被后端授予给 DepartmentAdmin 取决于后端实现。**但 `UserManagementView` 的空状态文案写着"仅超级管理员可管理用户与角色"**，这与 PRD 的 DepartmentAdmin 定义矛盾。

### 3.2 TeamLead 权限

| PRD 要求 | 前端实现 | 状态 |
|---------|---------|------|
| 只能管理自己组员 | `DataScopeEnum.GROUP` 由 `scopeMap[GROUP_LEADER]` 设定 | ⚠️ 偏差 |
| 可以看归属部门全部任务 | 需要 scope 为 DEPARTMENT | ❌ 当前 scope 为 GROUP |
| 可以改派本组任务 | `task-reassign-policy.ts` 中 `isGroupLeader && hasPermission(DESIGN_WORK)` | ✅ |
| 不可以改派跨组任务 | 仅前端启发式判断，后端 403 兜底 | ✅ |
| 不能创建账号 | 无独立限制，取决于后端是否授予 `org:manage` | ✅ |

**关键偏差：** PRD 要求组长"可以看归属部门全部任务"，但 `scopeMap` 中 `GROUP_LEADER` 对应的数据范围是 `DataScopeEnum.GROUP`（仅本组），不是 `DEPARTMENT`。这意味着组长在任务列表中只能看到本组任务，而 PRD 要求应能看到整个部门的任务。

### 3.3 HRAdmin 与 SuperAdmin

PRD 要求两者同级，都可进入业务流程，都具备公司级管理权限。

**前端现状：** HRAdmin 不在 `RoleEnum` 中，无法被 `isSuperAdmin` 等 computed 识别。若 HRAdmin 的 `frontend_access` 不包含 `is_super_admin: true`，则前端不会给予超管级权限。这是一个 **严重偏差**。

### 3.4 "有菜单即至少能做核心事"

前端实现菜单显隐依赖 `menus` + `pages` + `modules` 三层门控，与后端 `frontend_access` 紧密耦合。**只要后端正确下发这三个字段，前端可以保证"有菜单就能进页面"。** 但"核心动作可做"还依赖 `actions` 字段，两者可能不同步。

**风险：** 若后端给某角色发了 `menus: ['audit_workbench']` 但没发 `actions: ['task:audit']`，则用户能看到审核菜单但无法执行任何审核操作。当前无前端侧一致性校验。

### 修改建议

| 项 | 动作 | 风险 |
|----|------|------|
| UserManagementView 空状态文案修正 | 改为"需 org:manage 权限"或按角色动态文案 | **低** |
| TeamLead 数据范围调整 | `scopeMap[GROUP_LEADER]` 从 `GROUP` 改为 `DEPARTMENT`（或新增"可查看部门/只能操作本组"的双层机制） | **高**：影响全局数据过滤 |
| HRAdmin 识别逻辑 | 需要在前端建立 HRAdmin → 同等 SuperAdmin 的映射 | **高** |

---

## 4. 菜单与页面显隐

### PRD 要求 vs 前端 MENU_CONFIG 对照

| PRD 部门 | PRD 菜单 | 前端 MENU_CONFIG key | 匹配 |
|---------|---------|---------------------|------|
| 运营部 | 首页 | `dashboard` | ✅ |
| 运营部 | 任务中心 | `task_list` | ✅ |
| 运营部 | 资产管理 | `resource_management` (需 module) | ✅ |
| 设计研发部 | 首页 | `dashboard` | ✅ |
| 设计研发部 | 设计中心 | `design_workbench` | ✅ |
| 设计研发部 | 资产管理 | `resource_management` | ✅ |
| 定制美工部 | 首页 | `dashboard` | ✅ |
| 定制美工部 | 定制设计中心 | `customization_management` (需 module) | ✅ |
| 定制美工部 | 资产管理 | `resource_management` | ✅ |
| 审核部 | 首页 | `dashboard` | ✅ |
| 审核部 | 审核中心 | `audit_workbench` | ✅ |
| 审核部 | 资产管理 | `resource_management` | ✅ |
| 云仓部 | 首页 | `dashboard` | ✅ |
| 云仓部 | 云仓中心 | `warehouse` | ✅ |
| 云仓部 | 资产管理 | `resource_management` | ✅ |
| 人事部 | 首页 | `dashboard` | ✅ |
| 人事部 | KPI 管理中心 | `kpi` | ⚠️ |
| 各部门管理 | 组织权限 | `org_permission` | ✅ |

**偏差与多余项：**

1. **前端有但 PRD 未提及的菜单：**
   - `export_center`（导出中心）
   - `audit_log`（审计日志）
   - `logs_manage`（日志管理）
   - `finance`（财务核算）
   - `rules`（规则及模板）
   - `user_manage`（用户与角色）

   这些菜单在 MENU_CONFIG 中存在，对应路由和页面也已实现。PRD 未将它们列入各部门的正式菜单表中，但部分（如日志）在第 13 节有提及。**建议确认这些菜单是否属于 v1.0 收口范围。**

2. **PRD 中"部门管理额外展示组织权限页面"** — 前端通过 `org_permission` page key 控制。DeptAdmin 是否能看到这个菜单取决于后端是否在其 `frontend_access.pages` 中下发 `org_permission`。前端本身无硬编码限制。

3. **人事部 KPI 管理中心标注"暂未开发"** — 前端有 `KpiView.vue` 路由和菜单项，但内部实现尚不清楚是否为完整功能还是占位页面。

### 修改建议

| 项 | 动作 | 风险 |
|----|------|------|
| 确认非 PRD 列举菜单的归属 | 与产品确认 export_center、finance、rules、user_manage 是否为 v1.0 正式功能 | **低** |
| KPI 页面实现状态确认 | 检查 KpiView 是否为占位，若是需标注"即将上线" | **低** |

---

## 5. 普通任务主线

### 5.1 标准流程

| PRD 阶段 | 前端 WorkflowProgress 步骤 | MainTaskStatus 映射 | 状态 |
|---------|--------------------------|-------------------|------|
| 创建 | 创建 | DRAFT / CREATED | ✅ |
| 分配 | （无独立步骤，归入设计） | INFO_PENDING (PENDING_ASSIGN) | ⚠️ |
| 设计 | 设计 | INFO_PENDING | ✅ |
| 普通审核 | 审核 | INFO_PENDING | ✅ |
| 云仓 | 仓库接收 | WAREHOUSE_PENDING / WAREHOUSE_PROCESSING | ✅ |
| 完成 | 结单 | READY_TO_CLOSE / CLOSED | ✅ |

**偏差：** PRD 将"分配"列为独立阶段，但 `WorkflowProgress` 将创建后直接跳到"设计"步骤，分配隐含在设计阶段内部（designSubStatus = PENDING_ASSIGN）。已废弃的 `WorkflowSidebar` 曾有独立的"指派"步骤，但已不再使用。

**这是一个展示层偏差，功能上不影响流转，但若产品要求用户清晰看到"分配"为独立节点，需修改 `WorkflowProgress`。**

### 5.2 改派规则

| PRD 改派者 | 前端实现 | 状态 |
|-----------|---------|------|
| 发起人 | `task-reassign-policy.ts`: `isRequester \|\| isInitiator` + `TASK_CREATE` 权限 | ✅ |
| 部门管理 | `isDeptAdmin && hasPermission(TASK_ASSIGN)` | ✅ |
| 超管 | `isSuperAdmin` → true | ✅ |
| 组长仅限本组 | `isGroupLeader && hasPermission(DESIGN_WORK)` — 但无组范围限制 | ⚠️ 偏差 |

**偏差：** `canUserScheduleDesignerReassignment` 对组长只检查 `isGroupLeader && hasPermission(DESIGN_WORK)`，**没有检查任务是否属于本组**。PRD 要求组长仅可改派本组任务，前端依赖后端 403 兜底，但 UI 上会错误显示改派按钮。

### 5.3 审核通过后流转

PRD："由云仓端点击接收，后端决定下一步流转。"

前端：`WarehouseView` 提供"接收"按钮 → `tasksApi.warehouseReceive(id)` → 后端推进状态。吻合。

### 修改建议

| 项 | 动作 | 风险 |
|----|------|------|
| WorkflowProgress 增加"分配"步骤 | 在创建与设计之间插入分配节点 | **低**：纯展示，不影响逻辑 |
| 组长改派范围检查 | `canUserScheduleDesignerReassignment` 增加 `task.ownerOrgTeam === user.groupId` 判断 | **中**：需确认 task 上组归属字段的准确性 |

---

## 6. 定制任务主线

### 6.1 定位：独立主线

PRD："定制任务是一条独立主线，不是普通任务的附属支线。"

**前端现状：** 定制任务 **不是** 独立的 TaskType，而是通过以下方式与普通任务区分：
- `workflowLane: 'customization'`（字段级标记）
- `customizationRequired: true`（flag）
- `customizationSourceType`（来源类型）

**并且存在独立的 customization-job 资源**（`/v1/customization-jobs`），有自己的列表、详情、状态机。

**评估：** 前端架构上定制任务 **不完全独立**。Task 仍然是同一个实体表，只是通过 lane + flag 区分。同时又有 customization-job 作为补充资源。这种混合模式基本可用，但产品叙事上需要明确"一个 task + 一个 job"的关系。

### 6.2 定制流程对照

| PRD 阶段 | 前端实现 | 状态 |
|---------|---------|------|
| 创建 | `CustomizationCreateDialog` → `createCustomizationTask` → POST /v1/tasks | ✅ |
| 定制审核（标价模板） | 定制管理列表中 `customization/review` 操作 | ⚠️ 部分 |
| 定制美工作图 | customization-job 状态 `pending_customization_production` | ✅ |
| 效果图提交 | `effect-preview` POST | ✅ |
| 效果审核 | `effect-review` POST | ✅ |
| 转生产 | `production-transfer` POST | ✅ |
| 云仓统一流转 | 云仓接收（同普通任务） | ✅ |
| 完成 | 结单 | ✅ |

**偏差与问题：**

1. **WorkflowProgress 不支持定制任务流程。** `WorkflowProgress.vue` 只有两种路径：采购（创建→采购→仓库→结单）和非采购（创建→设计→审核→仓库→结单）。**定制任务没有专属的 workflow 步骤序列**——创建→定制审核→定制美工作图→效果审核→转生产→云仓→完成 这条线在进度条中不可见。定制任务使用的是同一个非采购模板，但实际经历的阶段完全不同。

2. **审核工作台未区分普通审核与定制审核。** `AuditWorkbenchView` 是一个统一队列，虽然通过 `WorkflowLaneTag` 显示"普通"/"定制"标签，但 **没有按审核组（普通审核组 / 定制美工审核组）分流**。PRD 中审核部有两个组，暗示不同组处理不同类型的审核。

3. **定制审核的"根据难易程度标价模板"** — 当前效果审核 payload (`CustomizationEffectReviewPayload`) 包含 `customization_level_code`、`customization_price`、`customization_weight_factor`，功能基本可用，但 **前端没有"标价模板"选择器**（如难易等级对应固定价格的模板表），只有自由填写。

4. **"定稿直接走生产审批"的二次流程** — 前端代码中 **没有实现** 定制审核中"定稿可直接走生产审批"的分支逻辑。

### 6.3 定制美工（CustomizationOperator）

PRD 定义该角色负责作图、上传、执行类产出。

**前端现状：** 无 `CustomizationOperator` 角色枚举、无专属菜单配置、无专属工作台。定制美工的工作通过 `customization-jobs` 列表完成，但缺乏身份标识——前端无法区分当前用户是"定制美工"还是其他角色。

### 6.4 定制审核能力

| PRD 要求能力 | 前端实现 | 状态 |
|------------|---------|------|
| 审核 | effect-review | ✅ |
| 修稿 | 未见独立修稿功能 | ❌ 缺失 |
| 判级 | `customization_level_code` 字段 | ✅ |
| 填写参考价 | `customization_price` 字段 | ✅ |
| 填写权重系数 | `customization_weight_factor` 字段 | ✅ |
| 填写备注 | `note` 字段 | ✅ |
| 替换当前有效稿 | 未见独立替换功能 | ❌ 缺失 |
| 打回上游 | effect-review reject 操作 | ✅ |

### 修改建议

| 项 | 动作 | 风险 |
|----|------|------|
| WorkflowProgress 新增定制流程路径 | 当 `workflowLane === 'customization'` 时渲染定制专属步骤序列 | **中**：需梳理定制 mainStatus/subStatus 到步骤的映射 |
| 审核工作台分组筛选 | 增加"审核类型"筛选（普通/定制），或根据用户所属审核组自动过滤 | **中**：需后端支持按审核组过滤 |
| 新增 CustomizationOperator 角色 | RoleEnum + 推断链 + scopeMap + ROLE_LABELS | **中** |
| 定制审核修稿/替换功能 | 新建资产替换流程 UI | **高**：需后端 API 支持 |
| 标价模板选择器 | 新增"难易等级→价格模板"的前端选择器组件 | **中**：需后端提供模板数据 |
| 二次流程（定稿直走生产） | 需产品细化规则后再实现 | **高**：流程复杂度增加 |

---

## 7. 评审参考值与执行冻结值

### PRD 要求

审核阶段填写"业务参考数据"，执行阶段冻结为"真实结算快照"。需要支持：可填写、可保存、可查询、与执行冻结值区分。

### 前端现状

`CustomizationJobDetailView.vue` 已实现两组字段：

| 字段组 | 前端字段 | 状态 |
|-------|---------|------|
| 审核参考 - 定制等级 | `customization_level_code` / `customization_level_name` | ✅ |
| 审核参考 - 参考单价 | `review_reference_unit_price` | ✅ |
| 审核参考 - 参考系数 | `review_reference_weight_factor` | ✅ |
| 执行冻结 - 单价 | `unit_price` | ✅ |
| 执行冻结 - 系数 | `weight_factor` | ✅ |
| 执行冻结 - 工类型 | `pricing_worker_type` / `employment_type` | ✅ |

**结论：✅ 基本吻合。** 前端已区分"审核参考"与"执行冻结"两组数据，字段命名清晰。

**小偏差：** PRD 用"难易等级"一词，前端用"定制等级"（`customization_level`）。建议统一术语。

### 7.1 冻结后表达

PRD 要求效果图提交后同时表达"已上传效果图"和"已完成价格冻结"。

**前端现状：** `CustomizationJobDetailView` 展示两组字段，但 **没有明确的"冻结状态"标记或视觉提示**（如冻结图标、不可编辑状态锁定提示）。建议增加冻结状态的视觉表达。

---

## 8. 审核替换与资产追溯

### PRD 要求

审核改稿替换有效稿时，须保留：替换人、替换前稿件、当前有效稿件、所属任务、备注。资产底层统一，前端视图分开。

### 前端现状

1. **版本时间线存在**：`DesignAssetBlock.vue` 显示 V1/V2/V3… 版本列表，每个版本有上传人和上传时间。
2. **`EventLogDrawer`** 可展示稿件替换事件，`task-events-from-api.ts` 中有"稿件替换"相关映射。
3. **但缺少结构化的替换追溯字段**：
   - 没有"替换人"字段（区分于"上传人"）
   - 没有"替换前稿件"的显式引用
   - 没有"审核员改稿"与"设计师上传"的来源区分
   - 版本列表是时间线式的，无法直观看出哪个版本替换了哪个版本

4. **资产管理统一但视图区分不完整**：
   - `AssetsIndexView` 是统一资产列表
   - `AssetDetailView` 有 `workflow_lane` / `source_department` 显示
   - 但资产列表缺少按 lane 筛选（普通/定制）的功能

### 修改建议

| 项 | 动作 | 风险 |
|----|------|------|
| 资产替换追溯结构化 | 版本记录增加 `replacedBy`（替换人ID）、`replacedAssetId`（被替换稿件ID）、`replaceNote`（替换备注）字段 | **高**：需后端 API + 数据模型支持 |
| 版本列表增加"替换"标记 | 在时间线中标注"审核替换"来源 | **中** |
| 资产列表增加 lane 筛选 | `AssetsIndexView` 增加"业务线"筛选项 | **低** |

---

## 9. 云仓统一入口

### PRD 要求

明确区分任务来源（普通/定制）、来源部门、task_type。拒收后按对应主线回退。

### 前端现状

1. **✅ 业务线筛选已实现**：`WarehouseView` 有 `laneFilter` 下拉（空/normal/customization），调用 `listWarehouseReceipts` 时传 `workflow_lane` 参数。
2. **✅ 来源部门展示**：`WarehouseReceiptPanel` 和 `WarehouseReceiptTable` 展示 `sourceDepartment`。
3. **✅ 定制任务标注**：当 `workflow_lane === 'customization'` 时有脚注提示。

**偏差：**
- `task_type`（ORIGINAL_PRODUCT_DEV / NEW_PRODUCT_DEV / PURCHASE_TASK）是否在云仓列表中展示？需确认 `WarehouseReceiptTable` 列定义中是否包含 `taskType` / `businessType` 列。

### 修改建议

| 项 | 动作 | 风险 |
|----|------|------|
| 确认 task_type 列是否展示 | 检查 WarehouseReceiptTable 是否显示任务类型列 | **低** |

---

## 10. 日志与系统能力边界

### PRD 要求

| 日志类型 | 可见范围 |
|---------|---------|
| 审计日志 | 审核相关角色 + 管理角色 |
| 操作日志 | 仅 HRAdmin 与 SuperAdmin |

### 前端现状

| 路由 | 页面 | 访问控制 | 与 PRD 对照 |
|------|------|---------|------------|
| `/audit-log` | AuditLogView | `can('audit:view')` | ⚠️ 部分吻合 |
| `/logs` | LogsManagementView | `can('audit:view') \|\| can('org:manage')` | ⚠️ 偏差 |
| `/logs` 内服务器日志 tab | 仅 `isSuperAdmin` | ✅ |

**偏差：**

1. **审计日志 `/audit-log`**：PRD 说"审核相关角色与管理角色可见"。前端用 `audit:view` 权限控制。只要后端给审核角色和管理角色都下发 `audit:view` action，就能吻合。但"管理角色"是否包含 DeptAdmin？需确认。

2. **操作日志 `/logs`**：PRD 说"仅 HRAdmin 与 SuperAdmin 可见"。前端控制为 `audit:view || org:manage`。如果后端给 DeptAdmin 发了 `org:manage`，则 DeptAdmin 也能进入日志页面，这 **违反** PRD 要求。

3. **HRAdmin 识别问题**：由于 HRAdmin 在前端不被识别为超管同级角色，其日志访问取决于后端下发的 actions。若后端不下发 `audit:view` 或 `org:manage`，HRAdmin 可能连日志都看不到。

### 修改建议

| 项 | 动作 | 风险 |
|----|------|------|
| `/logs` 访问控制收紧 | 改为 `isSuperAdmin \|\| isHRAdmin` | **中**：需先建立 HRAdmin 识别机制 |
| 与后端确认 `audit:view` 下发范围 | 确保只有审核角色+管理角色+超管拿到 `audit:view` | **低** |

---

## 11. 兼容逻辑治理

### PRD 要求

"不影响现有功能可以删除，但必须先确认真实使用情况，禁止误删仍在用接口。"

### 前端现状的兼容遗留

| 遗留项 | 位置 | 说明 |
|-------|------|------|
| `LegacyTaskStatus` 全套 23 个值 | `domain/types/task.ts` | 与新 MainTaskStatus/SubStatus 并存，大量 domain 逻辑仍依赖旧值 |
| `WorkflowSidebar.vue` | `components/task/` | 标注 @deprecated，无人引用但文件仍在 |
| `/outsource` 路由 | `router/index.ts` | 仍存在，指向 OutsourceView（兼容跳转页） |
| `outsourceApi.listOutsourceOrders` | `services/api/outsourceApi.ts` | 标注 deprecated，由 outsource store 调用 |
| `outsource.ts` store | `stores/outsource.ts` | 仍在使用 deprecated API |
| `WORKFLOW_ROLE_DISPLAY_TO_API` 中 `Admin` | `domain/user-workflow-roles.ts` | Admin 已不是正式角色，但仍可通过 UI 赋予 |
| `OrgAdmin`/`RoleAdmin`/`DesignDirector` 展示映射 | `domain/user-workflow-roles.ts` | 非正式角色仍有展示支持 |
| `MENU_KEY_COMPAT_ALIASES` / `PAGE_KEY_COMPAT_ALIASES` | `stores/permissions.ts` | 旧菜单/页面 key 别名映射 |
| `normalizeFlatTaskStatusFromApi` | `stores/tasks.ts` | snake_case → Legacy 状态的大量兼容映射 |
| `task-action-availability.ts` | `domain/` | 独立于 permission 体系的旧状态→操作可用性映射 |

**评估：** 兼容面较广但基本可控。主要风险在于 `LegacyTaskStatus` 与新状态体系的双轨运行——大量 `task-actions.ts` 中的判断仍基于旧状态值（如 `canAssign` 检查 `PendingAssign`），而 `WorkflowProgress` 使用新的 `mainStatus`。两套体系在中间层（`enrichTaskDomainFields`）做映射，但映射不完整（定制相关的 legacy 状态 fallback 到 `INFO_PENDING`）。

### 修改建议

| 项 | 动作 | 风险 |
|----|------|------|
| 删除 WorkflowSidebar.vue | 确认无引用后直接删除 | **低** |
| OutsourceView 重定向 | 将 `/outsource` 路由改为 redirect 到 `/customization-jobs` | **低** |
| 清理旧角色展示映射 | 从 WORKFLOW_ROLE_DISPLAY_TO_API 移除 Admin，从 WORKFLOW_ROLE_API_LABEL_ZH 移除不再保留的角色 | **中**：需确认后端是否仍返回这些角色 |
| Legacy 状态迁移计划 | 制定从 LegacyTaskStatus → MainStatus+SubStatus 的渐进迁移方案 | **高**：影响面最广 |

---

## 12. 总览表

| PRD 章节 | 核心要求 | 前端状态 | 偏差等级 |
|---------|---------|---------|---------|
| §5 角色模型 | 8 个正式角色 | 缺 CustomizationOperator、HRAdmin 映射不完整 | 🔴 高 |
| §6.1 DeptAdmin | 完整管理能力 | 功能可用但文案/权限判断有偏差 | 🟡 中 |
| §6.2 TeamLead | 看部门、操作本组 | 数据范围错误（GROUP 应为 DEPARTMENT） | 🔴 高 |
| §6.3 HRAdmin≈SuperAdmin | 同级管理权限 | 前端无法识别 HRAdmin | 🔴 高 |
| §7 菜单显隐 | 按部门分配 | 基本吻合，依赖后端正确下发 | 🟢 低 |
| §8 普通任务主线 | 创建→分配→设计→审核→云仓→完成 | "分配"步骤未独立展示 | 🟡 中 |
| §9 定制任务主线 | 独立完整流程 | 流程进度条缺失、审核未分组 | 🔴 高 |
| §10 评审参考值/冻结值 | 两组字段区分 | ✅ 已实现 | 🟢 低 |
| §11 审核替换追溯 | 结构化追溯信息 | 仅有版本时间线，无结构化替换记录 | 🔴 高 |
| §12 云仓统一入口 | 区分来源 | ✅ 已实现 lane 筛选 + 来源部门 | 🟢 低 |
| §13 日志可见性 | 操作日志仅超管/HR | 当前控制条件过宽 | 🟡 中 |
| §14 兼容治理 | 不再扩大 | 双轨状态体系仍在，但未扩大 | 🟡 中 |

---

## 13. 风险矩阵

| 风险 | 影响范围 | 发生概率 | 严重度 | 说明 |
|------|---------|---------|-------|------|
| HRAdmin 登录后权限错误 | 人事部全部用户 | 高 | 高 | 无法被识别，回落为 MEMBER |
| TeamLead 任务列表数据不全 | 所有组长 | 高 | 高 | 只能看本组，PRD 要求看部门 |
| 定制美工无身份标识 | 定制美工部全体 | 高 | 中 | 菜单可进但角色不明确 |
| 定制任务 WorkflowProgress 误导 | 所有查看定制任务的用户 | 高 | 中 | 显示普通任务步骤而非定制步骤 |
| 审核替换资产不可追溯 | 合规与审计 | 中 | 高 | 替换操作无结构化记录 |
| 组长改派按钮越界显示 | 有跨组任务的组长 | 中 | 低 | 后端 403 兜底但体验差 |
| Legacy 状态映射不完整 | 定制任务详情页 | 中 | 中 | 定制状态 fallback 到 INFO_PENDING |
| 操作日志权限过宽 | DeptAdmin 可进 | 低 | 中 | 违反 PRD 仅超管/HR 可见 |

---

## 14. 建议优先级排序

### P0（收口前必须完成）

1. **HRAdmin 角色识别**：在 `applyFrontendAccess` 中增加 HRAdmin → SuperAdmin 同级权限映射。不做这个改动，人事部将无法正常使用系统。
2. **TeamLead 数据范围修正**：`scopeMap[GROUP_LEADER]` 改为 `DEPARTMENT`，同时在操作层面（改派、编辑）仍限制为本组。需引入"查看范围"与"操作范围"的分离机制。
3. **CustomizationOperator 角色注册**：新增枚举、推断链、scopeMap 条目。否则定制美工部无法与设计师角色区分。

### P1（收口阶段应完成）

4. **定制任务 WorkflowProgress 专属路径**：当 `workflowLane === 'customization'` 时渲染定制流程步骤（创建→定制审核→定制作图→效果审核→转生产→云仓→完成）。
5. **审核工作台分组筛选**：至少增加"业务线"筛选项（普通/定制），让审核员可以按自己的审核组职责过滤任务。
6. **操作日志权限收紧**：`/logs` 页面的 `canView` 从 `audit:view || org:manage` 改为 `isSuperAdmin || isHRAdmin`。
7. **资产替换追溯基础结构**：与后端协商替换记录 API 格式，前端准备版本时间线中的"替换"标记展示。

### P2（收口后优化）

8. 普通任务 WorkflowProgress 增加"分配"独立步骤。
9. 组长改派按钮增加本组范围前端校验。
10. 清理 `WorkflowSidebar.vue`、`/outsource` 重定向、旧角色展示映射。
11. 定制审核"标价模板"选择器。
12. 定制二次流程（定稿直走生产）。
13. Legacy 状态体系渐进迁移方案制定。
14. `UserManagementView` 空状态文案修正。
15. 资产列表增加 lane 筛选。

---

## 附录 A：关键文件索引

| 文件 | 职责 |
|------|------|
| `src/types/permission.ts` | RoleEnum、PermissionEnum、DataScopeEnum 定义 |
| `src/stores/permissions.ts` | 会话、RBAC、角色推断、组织数据 |
| `src/composables/usePermission.ts` | 权限检查 composable |
| `src/domain/user-workflow-roles.ts` | API 角色码 ↔ 中文展示映射 |
| `src/layouts/AppShell.vue` | MENU_CONFIG、visibleMenus 侧边栏控制 |
| `src/router/index.ts` | 路由定义、meta.requiredPageKey/requiredPermissions |
| `src/domain/types/task.ts` | Task 类型、Legacy/Main/Sub 状态定义 |
| `src/domain/task-actions.ts` | 工作流状态谓词（canAssign、canAudit 等） |
| `src/domain/task-reassign-policy.ts` | 设计师改派权限策略 |
| `src/components/task/WorkflowProgress.vue` | 任务流程进度条 |
| `src/views/AuditWorkbenchView.vue` | 审核工作台 |
| `src/views/WarehouseView.vue` | 云仓中心 |
| `src/views/CustomizationJobsView.vue` | 定制管理列表 |
| `src/views/CustomizationJobDetailView.vue` | 定制任务详情 |
| `src/services/api/customizationApi.ts` | 定制 API |
| `src/domain/mappers/asset-versions-from-api.ts` | 资产版本映射 |

## 附录 B：后端确认事项清单

以下事项需与后端团队确认后才能最终确定前端改动方案：

1. `frontend_access.roles` 中 HRAdmin 的下发方式——是独立 role 还是 `is_super_admin: true`？
2. TeamLead 的下发方式——`roles: ['TeamLead']` 还是 `is_group_leader: true`？
3. CustomizationOperator 是否作为独立 role 下发？
4. `frontend_access.actions` 中 `audit:view`、`org:manage` 的角色分配策略。
5. 审核任务是否支持按审核组（`audit_group`）过滤 API？
6. 资产替换是否有结构化 API（替换人、替换前后稿件 ID）？
7. 定制任务的 mainStatus/subStatus 流转是否已与普通任务区分？
8. `/v1/products/*` 等兼容接口的真实调用情况。
