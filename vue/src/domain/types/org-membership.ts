/**
 * 组织与权限页：按组管人（真实用户归属），与 task canonical 无关。
 * 字段尽量 optional，兼容后端 WorkflowUser 变体。
 */

export type CandidateUserActionState = 'join' | 'move' | 'in_current_group'

/** 与 GET /v1/users 列表项对齐的归一化用户（组织维度） */
export interface OrgUser {
  id: string
  username?: string
  real_name?: string
  display_name?: string
  displayName?: string
  /** 部门展示名；后端可能是 string 或 { name } */
  department?: string | { name?: string }
  /** 小组/团队名 */
  team?: string
  /** 部分接口用 group 作为 team 别名 */
  group?: string
  roles?: string[]
  joined_at?: string
  created_at?: string
  status?: string
}

/** 左侧树：小组节点（选中粒度为组） */
export interface OrgTreeGroupNode {
  /** 稳定键：用于选中态、统计 */
  key: string
  departmentName: string
  teamName: string
  /** 来自本地 store 时可回填，便于移动/删除 */
  storeDepartmentId?: string
  storeGroupId?: string
  /** 是否来自 GET /v1/org/options（否则来自本地 store 或用户数据推导） */
  fromApi?: boolean
}

export interface OrgTreeDepartmentNode {
  name: string
  expanded: boolean
  groups: OrgTreeGroupNode[]
  /** 与 permissions store 中部门 id 同名绑定时存在，用于重命名/删除部门 */
  storeDepartmentId?: string
}
