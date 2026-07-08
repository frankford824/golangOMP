/**
 * 用户与角色管理页面共享类型与纯展示格式化函数:
 * UserManagementView 及其拆分出的弹窗组件共用。
 */

export interface UserRow {
  id: string
  username: string
  employee_no?: number | null
  display_name?: string
  department?: string
  team?: string
  roles: string[]
  status?: 'active' | 'disabled' | string
  frontend_access?: unknown
}

export interface RoleOption {
  code: string
  display: string
  category: 'management' | 'business' | 'asset_workbench' | 'compatibility' | string
  assignable: boolean
  assignableByCurrentActor: boolean
  deprecated: boolean
  hiddenByDefault: boolean
}

export interface RoleOptionGroup {
  category: string
  title: string
  roles: RoleOption[]
}

export interface CreateUserForm {
  username: string
  employee_no: string
  display_name: string
  department: string
  team: string
  mobile: string
  email: string
  password: string
  roles: string[]
  status: 'active' | 'disabled'
}

export interface SelectOptionItem {
  value: string
  label: string
  department?: string
}

/** 仅展示；查询/提交仍用 active、disabled */
export function formatUserStatusForDisplay(status: string | undefined): string {
  if (status == null || status === '') return '—'
  if (status === 'active') return '启用'
  if (status === 'disabled') return '已禁用'
  return status
}

export function formatEmployeeNo(value: number | null | undefined): string {
  return value == null ? '待维护工号' : `工号 ${value}`
}

export function lockedRoleTooltip(role: RoleOption): string {
  if (role.category === 'compatibility' || role.deprecated) {
    return '当前账号已持有历史保留角色；本页不再支持新增分配。'
  }
  return '当前账号已持有该角色；本页不支持由当前登录账号调整。'
}
