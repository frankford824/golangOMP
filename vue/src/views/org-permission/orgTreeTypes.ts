/**
 * 组织树节点类型：UserManagementView 与 OrgTreePanel 共用。
 */
export interface OrgTreeTeam {
  id?: string
  value: string
  label: string
  department: string
  enabled: boolean
  /** 小组当前人数(含停用账号);后端未返回时为 undefined */
  memberCount?: number
}

export interface OrgTreeDepartment {
  id?: string
  value: string
  label: string
  enabled: boolean
  /** 部门当前人数(含停用账号);后端未返回时为 undefined */
  memberCount?: number
  teams: OrgTreeTeam[]
}
