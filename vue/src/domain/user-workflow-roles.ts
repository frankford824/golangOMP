/**
 * 会话中的历史身份 code 只用于展示迁移结果，不能作为前端授权依据。
 * 页面动作统一读取后端下发的 capabilities 与 allowed_actions。
 */
const ROLE_LABELS: Record<string, string> = {
  admin: '超级管理员',
  roleadmin: '超级管理员',
  role_admin: '超级管理员',
  accessadmin: '超级管理员',
  access_admin: '超级管理员',
  erp: '超级管理员',
  erpoperator: '超级管理员',
  erp_operator: '超级管理员',
  superadmin: '超级管理员',
  super_admin: '超级管理员',
  hradmin: '素材人员管理员',
  hr_admin: '素材人员管理员',
  assetprofileadmin: '素材人员管理员',
  asset_profile_admin: '素材人员管理员',
  departmentadmin: '部门管理员',
  department_admin: '部门管理员',
  dept_admin: '部门管理员',
  orgadmin: '部门管理员',
  org_admin: '部门管理员',
  teamlead: '部门管理员',
  team_lead: '部门管理员',
  group_leader: '部门管理员',
  designdirector: '部门管理员',
  design_director: '部门管理员',
  member: '系统基础角色',
  ops: '运营',
  operations: '运营',
  designer: '常规设计师',
  customizationoperator: '定制设计师',
  customization_operator: '定制设计师',
  auditor: '审核',
  audit_a: '审核',
  audit_b: '审核',
  customizationreviewer: '审核',
  customization_reviewer: '审核',
  assetsubmitter: '素材交付人员',
  asset_submitter: '素材交付人员',
  assetmanager: '素材管理员',
  asset_manager: '素材管理员',
  assettemplateadmin: '计价配置管理员',
  asset_template_admin: '计价配置管理员',
  assetsettlement: '结算管理员',
  asset_settlement: '结算管理员',
}

function normalizedRoleKey(role: string): string {
  return role.trim().split('-').join('_').toLowerCase()
}

export function workflowRoleApiToDisplay(role: string): string {
  const key = normalizedRoleKey(String(role ?? ''))
  if (!key) return ''
  const compact = key.split('_').join('')
  if (ROLE_LABELS[key]) return ROLE_LABELS[key]
  if (ROLE_LABELS[compact]) return ROLE_LABELS[compact]
  return '历史身份（已停用）'
}

export function formatUserRoleForDisplay(role: string | null | undefined): string {
  const value = workflowRoleApiToDisplay(String(role ?? ''))
  return value || '-'
}

export function formatWorkflowRolesForDisplay(apiRoles: string[] | undefined): string {
  if (!apiRoles?.length) return '无'
  const labels = [...new Set(apiRoles.map(workflowRoleApiToDisplay).filter(Boolean))]
  return labels.length ? labels.join('、') : '无'
}
