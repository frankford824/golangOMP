/**
 * Display roles are labels only. Runtime authorization comes exclusively from
 * `frontend_access.actions` and the backend `allowed_actions` contract.
 */
export const RoleEnum = {
  SUPER_ADMIN: 'super_admin',
  HR_ADMIN: 'hr_admin',
  DEPT_ADMIN: 'dept_admin',
  GROUP_LEADER: 'group_leader',
  MEMBER: 'member',
  OPS: 'ops',
  DESIGNER: 'designer',
  CUSTOMIZATION_OPERATOR: 'customization_operator',
  AUDITOR: 'auditor',
} as const

export type RoleEnumValue = (typeof RoleEnum)[keyof typeof RoleEnum]

export const DataScopeEnum = {
  SELF: 'self',
  GROUP: 'group',
  DEPARTMENT: 'department',
  GLOBAL: 'global',
} as const

export type DataScopeEnumValue = (typeof DataScopeEnum)[keyof typeof DataScopeEnum]

/** Current V8 capability codes. No retired warehouse, outsource or A/B audit aliases. */
export const PermissionEnum = {
  ACCOUNT_USE: 'account.use',
  TASK_VIEW: 'task.view',
  TASK_CREATE: 'task.create',
  TASK_ASSIGN: 'task.assign',
  TASK_REASSIGN: 'task.reassign',
  TASK_TERMINATE: 'task.terminate',
  TASK_DESIGN_SUBMIT: 'task.upload_source',
  TASK_AUDIT: 'task.audit',
  TASK_AUDIT_HANDOVER: 'task.audit_handover',
  TASK_REOPEN: 'task.reopen',
  PLANNING_SKU_VIEW: 'planning_sku.view',
  PLANNING_SKU_CREATE: 'planning_sku.create',
  PLANNING_SKU_EDIT: 'planning_sku.edit',
  PLANNING_SKU_EXPORT: 'planning_sku.export',
  PLANNING_SKU_ERP_SYNC: 'planning_sku.erp_sync',
  PLANNING_SKU_ERP_RETRY: 'planning_sku.erp_retry',
  ASSET_VIEW: 'asset.view',
  ASSET_DOWNLOAD: 'asset.download',
  ASSET_EXPORT: 'asset.export',
  ASSET_PUBLISH: 'asset.publish',
  ASSET_MANAGE: 'asset.manage',
  CATALOG_VIEW: 'catalog.view',
  CATALOG_MANAGE: 'catalog.manage',
  ERP_MANAGE: 'erp.manage',
  REPORT_VIEW: 'report.view',
  SYSTEM_MANAGE: 'system.manage',
  ACCESS_VIEW: 'access.view',
  ACCESS_MANAGE: 'access.manage',
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
