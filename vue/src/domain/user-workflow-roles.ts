/**
 * 会话中的历史身份 code 只用于展示迁移结果，不能作为前端授权依据。
 * 页面动作统一读取后端下发的 capabilities 与 allowed_actions。
 */
const ROLE_LABELS: Record<string, string> = {
  superadmin: '超级管理员',
  super_admin: '超级管理员',
  hradmin: '人事管理员',
  hr_admin: '人事管理员',
  departmentadmin: '部门管理员',
  dept_admin: '部门管理员',
  teamlead: '组长',
  group_leader: '组长',
  member: '成员',
  ops: '运营',
  designer: '设计师',
  customizationoperator: '定制设计师',
  customization_operator: '定制设计师',
  auditor: '审核员',
  audit_a: '审核员',
  audit_b: '审核员',
  customizationreviewer: '审核员',
  customization_reviewer: '审核员',
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
