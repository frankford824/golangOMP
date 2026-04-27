# v1.0 前端集成指南

> 最终发布日期: 2026-04-16  
> 后端版本: v1.0  
> 后端环境: 唯一线上环境 (已完成基线重置)

---

## G1. 官方角色模型

| 角色 | 代码 | 产品定位 |
|------|------|----------|
| 运营 | `Ops` | 任务创建/管理 |
| 部门管理员 | `DepartmentAdmin` | 部门用户/任务治理 |
| 组长 | `TeamLead` | 组内任务管理 |
| 设计师 | `Designer` | 设计作业 |
| 定制操作员 | `CustomizationOperator` | 定制美工作业 |
| 普通审核 | `Audit_A` | 常规设计审核 |
| 定制审核 | `Audit_B` / `CustomizationReviewer` | 定制流程审核 |
| 仓库 | `Warehouse` | 统一入库/出库 |
| 人事管理员 | `HRAdmin` | 全公司用户管理 |
| 超级管理员 | `SuperAdmin` | 全权限 |

**兼容角色 (新功能禁止使用):** `Admin`, `OrgAdmin`, `RoleAdmin`, `DesignDirector`, `DesignReviewer`, `Outsource`, `ERP`

---

## G2. 角色菜单/页面预期

### SuperAdmin
- **菜单**: user_admin, org_admin, role_admin, logs_center, + 所有业务菜单
- **页面**: admin_users, admin_roles, admin_permission_logs, admin_operation_logs, org_options
- **能力**: 全权限, 可跨部门操作

### HRAdmin
- **菜单**: user_admin, org_admin, logs_center
- **页面**: admin_users, admin_permission_logs, admin_operation_logs, org_options
- **能力**: 全公司用户管理, 查看操作日志

### DepartmentAdmin
- **菜单**: org_admin, user_admin
- **页面**: department_users, org_options
- **能力**: 查看本部门成员, 跨组移动成员, 分配未分配用户, 创建/禁用账户, 重置密码, 跨组任务重分配

### TeamLead
- **菜单**: (无独立菜单)
- **页面**: team_users
- **能力**: 查看本部门全部任务, 仅操作本组任务, 不能创建/禁用账户

### Ops
- **菜单**: task_create, business_info, task_board, task_list, warehouse_receive, warehouse_processing, export_center, resource_management, customization_management
- **页面**: task_board, task_list, task_create, assets_index, task_assets, asset_detail, customization_jobs, customization_job_detail
- **能力**: 创建任务, 分配任务, 管理业务信息

### Designer
- **菜单**: design_workspace, task_list, export_center, resource_management
- **页面**: design_workspace, my_tasks, design_submit, design_rework, export_jobs, assets_index, task_assets, asset_detail
- **能力**: 提交设计, 上传资产

### CustomizationOperator
- **菜单**: customization_management, resource_management, task_list
- **页面**: customization_jobs, customization_job_detail, task_assets, asset_detail, assets_index, task_list
- **能力**: 效果预览提交, 生产转交, 资产上传

### Audit_A (普通审核)
- **菜单**: audit_queue, task_board, task_list, export_center
- **页面**: task_board, task_list, audit_workspace, export_jobs
- **能力**: 认领/批准/驳回审核

### Audit_B / CustomizationReviewer (定制审核)
- **菜单**: audit_queue / customization_management, task_board, task_list, resource_management
- **页面**: 审核工作台, 定制 jobs 列表
- **能力**: 定制审核评审, 效果评审, 替换稿件 (可追溯)

### Warehouse
- **菜单**: warehouse_receive, warehouse_processing, task_board, task_list, export_center
- **页面**: warehouse_receive, warehouse_processing, task_list, task_board, export_jobs
- **能力**: 接收/驳回/完成入库

---

## G3. 规范后端入口

### 认证与组织

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1/auth/login` | POST | 登录 |
| `/v1/auth/me` | GET | 当前用户信息 (含 frontend_access) |
| `/v1/auth/password` | PUT | 修改密码 |
| `/v1/org/options` | GET | 部门/团队选项 |
| `/v1/roles` | GET | 角色列表 |
| `/v1/users` | GET | 用户列表 |
| `/v1/users` | POST | 创建用户 |
| `/v1/users/:id` | GET/PATCH | 用户详情/更新 |
| `/v1/users/:id/password` | PUT | 重置密码 |
| `/v1/users/:id/roles` | POST/PUT/DELETE | 角色管理 |
| `/v1/access-rules` | GET | 路由权限目录 |

### 任务

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1/tasks/reference-upload` | POST | 任务创建前参考图上传 |
| `/v1/tasks/prepare-product-codes` | POST | 预览产品编码 |
| `/v1/tasks` | POST | 创建任务 |
| `/v1/tasks` | GET | 任务列表 (支持 `workflow_lane` 过滤) |
| `/v1/tasks/:id` | GET | 任务详情 |
| `/v1/tasks/:id/detail` | GET | 任务聚合详情 |
| `/v1/tasks/:id/product-info` | GET/PATCH | 产品信息 |
| `/v1/tasks/:id/cost-info` | GET/PATCH | 成本信息 |
| `/v1/tasks/:id/business-info` | PATCH | 业务信息 |
| `/v1/tasks/:id/filing-status` | GET | 归档状态 |
| `/v1/tasks/:id/assign` | POST | 分配任务 |
| `/v1/tasks/batch/assign` | POST | 批量分配 |
| `/v1/tasks/:id/submit-design` | POST | 提交设计 (支持批量) |
| `/v1/tasks/:id/events` | GET | 任务事件日志 |
| `/v1/tasks/:id/close` | POST | 关闭任务 |

### 定制流程

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1/tasks/:id/customization/review` | POST | 定制审核 |
| `/v1/customization-jobs` | GET | 定制 job 列表 |
| `/v1/customization-jobs/:id` | GET | 定制 job 详情 |
| `/v1/customization-jobs/:id/effect-preview` | POST | 效果预览提交 |
| `/v1/customization-jobs/:id/effect-review` | POST | 效果评审 |
| `/v1/customization-jobs/:id/production-transfer` | POST | 生产转交 |

### 审核

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1/tasks/:id/audit/claim` | POST | 认领审核 |
| `/v1/tasks/:id/audit/approve` | POST | 批准 |
| `/v1/tasks/:id/audit/reject` | POST | 驳回 |
| `/v1/tasks/:id/audit/transfer` | POST | 转交 |
| `/v1/tasks/:id/audit/handover` | POST | 交接 |
| `/v1/tasks/:id/audit/takeover` | POST | 接管 |

### 资产

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1/assets` | GET | 资产资源列表 |
| `/v1/assets/:id` | GET | 资产详情 |
| `/v1/assets/:id/download` | GET | 下载元数据 |
| `/v1/assets/:id/preview` | GET | 预览 (含 PSD 二级预览) |
| `/v1/assets/upload-sessions` | POST | 创建上传会话 |
| `/v1/assets/upload-sessions/:session_id` | GET | 获取会话状态 |
| `/v1/assets/upload-sessions/:session_id/complete` | POST | 完成会话 |
| `/v1/assets/upload-sessions/:session_id/cancel` | POST | 取消会话 |
| `/v1/tasks/:id/assets` | GET | 任务关联资产列表 |

### 仓库

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1/warehouse/receipts` | GET | 仓库收据列表 (支持 `workflow_lane` 过滤) |
| `/v1/tasks/:id/warehouse/receive` | POST | 接收 |
| `/v1/tasks/:id/warehouse/reject` | POST | 驳回 |
| `/v1/tasks/:id/warehouse/complete` | POST | 完成 |

### 日志

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1/audit-logs` | GET | 审计日志 (审核角色+管理角色) |
| `/v1/operation-logs` | GET | 操作日志 (仅 HRAdmin/SuperAdmin) |

---

## G4. 流程语义

### 普通车道 (workflow_lane=normal)

```
创建 → PendingAssign → Assigned → InProgress → PendingAuditA
  → [审核通过] → PendingWarehouseReceive → PendingClose → Completed
  → [审核驳回] → RejectedByAuditA → 回到设计师
```

### 定制车道 (workflow_lane=customization)

```
创建(customization_required=true) → PendingCustomizationReview
  → [审核通过] → PendingCustomizationProduction
  → [效果预览] → PendingEffectReview
  → [效果评审通过] → PendingProductionTransfer
  → [生产转交] → PendingWarehouseQC
  → [仓库接收] → Completed
  → [仓库驳回] → RejectedByWarehouse → 回到上次操作员
```

### 仓库统一入库
- 仓库收据同时支持普通和定制车道
- 读模型暴露: `workflow_lane`, `source_department`, `task_type`

### 审核替换可追溯性
- 替换操作记录 `previous_asset_id` + `current_asset_id` + `replacement_actor_id`
- 通过 `/v1/tasks/:id/events` 可查询完整替换链

### 审核参考值 vs 冻结值
- `review_reference_unit_price` / `review_reference_weight_factor`: 审核阶段的参考值
- `unit_price` / `weight_factor` / `pricing_worker_type`: 执行冻结快照值
- 前端展示时需区分两者，不要混淆

---

## G5. 兼容路由 (新开发禁止使用)

| 路由 | 替代 | 说明 |
|------|------|------|
| `/v1/task-create/asset-center/upload-sessions*` | `POST /v1/tasks/reference-upload` | 创建前参考图上传 |
| `/v1/tasks/:id/asset-center/*` | `/v1/tasks/:id/assets` + `/v1/assets/*` | 资产中心 |
| `/v1/tasks/:id/assets/upload-sessions/*` | `/v1/assets/upload-sessions` | 上传会话 |
| `/v1/tasks/:id/assets/upload` | `/v1/assets/upload-sessions` | 已废弃 |
| `/v1/tasks/:id/outsource` | `customization_required=true` 创建 | 外包创建 |
| `/v1/outsource-orders` | `/v1/customization-jobs` | 外包列表 |
| `/v1/products/*` | `/v1/erp/products*` | ERP 产品 |
| `/v1/assets/files/{path}` | `/v1/assets/:id/download` | 文件代理 |

---

## G6. 前端 UAT 检查清单

### 角色逐一验证

- [ ] **SuperAdmin**: 登录→用户管理→角色管理→组织配置→所有菜单可见→可跨部门操作
- [ ] **HRAdmin**: 登录→用户管理→操作日志可见→审计日志可见
- [ ] **DepartmentAdmin**: 登录→本部门用户可见→可跨组移动用户→可创建用户→可重置密码
- [ ] **TeamLead**: 登录→本部门任务可见→仅本组任务可操作→不能创建用户
- [ ] **Ops**: 登录→创建任务→分配任务→任务列表→业务信息管理
- [ ] **Designer**: 登录→设计工作台→提交设计→资产上传→重做通知
- [ ] **CustomizationOperator**: 登录→定制管理→效果预览提交→生产转交→资产上传
- [ ] **Audit_A (普通审核)**: 登录→审核队列→认领→批准/驳回
- [ ] **CustomizationReviewer (定制审核)**: 登录→定制管理→定制审核→效果评审→替换稿件可追溯
- [ ] **Warehouse**: 登录→仓库接收→驳回→完成→列表含 workflow_lane/source_department/task_type

### 普通车道端到端

- [ ] Ops 创建普通任务
- [ ] Ops 分配给设计师
- [ ] 设计师上传资产并提交
- [ ] 审核认领→批准
- [ ] 仓库接收→完成
- [ ] 任务关闭

### 定制车道端到端

- [ ] Ops 创建定制任务 (customization_required=true)
- [ ] 定制审核评审
- [ ] 定制操作员效果预览提交
- [ ] 定制审核效果评审
- [ ] 定制操作员生产转交
- [ ] 仓库接收→完成
- [ ] 检查事件日志中的替换可追溯性
- [ ] 检查参考价 vs 冻结价显示

### 权限安全检查

- [ ] 普通用户不能访问审计日志
- [ ] TeamLead 不能创建用户
- [ ] DepartmentAdmin 只能管理本部门
- [ ] 操作日志仅 HRAdmin/SuperAdmin 可见
- [ ] 兼容路由返回 Deprecation 头

---

## 官方部门结构

| 部门 | 团队 |
|------|------|
| 运营部 | 淘系一组, 淘系二组, 天猫一组, 天猫二组, 拼多多南京组, 拼多多池州组 |
| 设计研发部 | 默认组 |
| 定制美工部 | 默认组 |
| 审核部 | 普通审核组, 定制审核组 |
| 云仓部 | 默认组 |
| 人事部 | 默认组 |

---

## 环境信息

- **后端地址**: 同当前部署的唯一线上环境
- **数据状态**: 已完成 v1.0 基线重置，仅保留 admin 超管账户和主数据
- **可立即开始**: 创建新用户、新任务、新资产进行 UAT
