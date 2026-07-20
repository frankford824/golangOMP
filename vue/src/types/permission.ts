export const RoleEnum = {
  // ─── 组织层级角色（管理体系）──────────────────────────────────────────────────
  SUPER_ADMIN: 'super_admin',
  HR_ADMIN: 'hr_admin',
  DEPT_ADMIN: 'dept_admin',
  GROUP_LEADER: 'group_leader',
  MEMBER: 'member',
  // ─── 职位角色（岗位体系，与层级角色正交，基层员工可直接使用）──────────────────
  OPS: 'ops',          // 运营：创建任务、指派、成本管控、结单
  DESIGNER: 'designer', // 设计师：资产上传、提交审核
  CUSTOMIZATION_OPERATOR: 'customization_operator', // 定制美工：作图、上传、效果产出
  AUDIT_A: 'audit_a',   // 常规审核：覆盖常规审核与交接复核状态
  /** @deprecated 常规审核旧编码，仅兼容历史账号；新授权使用 AUDIT_A */
  AUDIT_B: 'audit_b',
  /** @deprecated 使用 AUDIT_A */
  AUDITOR: 'auditor',
  WAREHOUSE: 'warehouse', // 仓库员：接收/退回/归档
} as const

export type RoleEnumValue = (typeof RoleEnum)[keyof typeof RoleEnum]

export const DataScopeEnum = {
  SELF: 'self',
  GROUP: 'group',
  DEPARTMENT: 'department',
  GLOBAL: 'global',
} as const

export type DataScopeEnumValue = (typeof DataScopeEnum)[keyof typeof DataScopeEnum]

export const PermissionEnum = {
  // 任务
  TASK_VIEW: 'task:view',
  TASK_CREATE: 'task:create',
  PLANNING_SKU_CREATE: 'planning_sku.create',
  TASK_EDIT: 'task:edit',
  /** v0.6 对齐：K 节，商品/成本编辑与试算仅 Ops/Warehouse/Admin */
  TASK_EDIT_COST: 'task:edit-cost',
  TASK_ASSIGN: 'task:assign',
  TASK_AUDIT: 'task:audit',
  TASK_WAREHOUSE: 'task:warehouse',
  WAREHOUSE_RECEIVE: 'warehouse:receive',
  WAREHOUSE_RETURN: 'warehouse:return',
  TASK_COMPLETE: 'task:complete',
  TASK_ARCHIVE: 'task:archive',
  TASK_CLOSE: 'task:close',
  TASK_FORCE_CLOSE: 'task:force_close',
  // 设计
  DESIGN_WORK: 'design.work',
  DESIGN_UPLOAD: 'design.upload',
  DESIGN_SUBMIT: 'design.submit',
  // 定制 / 仓库
  OUTSOURCE_VIEW: 'outsource:view',
  WAREHOUSE_VIEW: 'warehouse:view',
  // 数据 / 配置
  RULES_EDIT: 'rules:edit',
  ORG_MANAGE: 'org:manage',
  EXPORT_TASKS: 'export:tasks',
  AUDIT_VIEW: 'audit:view',
  // KPI / 财务
  KPI_VIEW: 'kpi:view',
  FINANCE_VIEW: 'finance:view',
  REPORT_VIEW: 'report.view',
} as const

export type PermissionEnumValue = (typeof PermissionEnum)[keyof typeof PermissionEnum]

export interface PermissionUser {
  id: string
  account?: string
  username?: string
  name: string
  avatar?: string
  avatarUrl?: string
  role: RoleEnumValue
  departmentId: string
  groupId: string
  dataScope: DataScopeEnumValue
  permissions: PermissionEnumValue[]
}

export interface Department {
  id: string
  name: string
}

export interface Group {
  id: string
  name: string
  departmentId: string
}
