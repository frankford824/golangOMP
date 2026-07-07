/**
 * POST /v1/users/{id}/roles 使用 OpenAPI V7Role 等稳定 code；
 * 用户管理界面使用简短展示名（含空格），需映射后再提交。
 * @see docs/openapi.yaml components/schemas/V7Role
 */
export const WORKFLOW_ROLE_DISPLAY_TO_API: Record<string, string> = {
  超级管理员: 'SuperAdmin',
  人事管理员: 'HRAdmin',
  部门管理员: 'DepartmentAdmin',
  组长: 'TeamLead',
  成员: 'Member',
  运营: 'Ops',
  设计师: 'Designer',
  定制美工: 'CustomizationOperator',
  常规审核: 'Audit_A',
  定制审核: 'CustomizationReviewer',
  仓库人员: 'Warehouse',
  '素材工作台-交付人员': 'AssetSubmitter',
  '素材工作台-作品管理': 'AssetManager',
  '素材工作台-计价配置': 'AssetTemplateAdmin',
  '素材工作台-结算财务': 'AssetSettlement',
}

/**
 * 接口 role code → 界面中文（仅展示；提交仍用 code）。
 * 未列出的 code 回退到 WORKFLOW_ROLE_DISPLAY_TO_API 反查，再不行则原样返回。
 * @see docs/openapi.yaml `V7Role`
 */
export const WORKFLOW_ROLE_API_LABEL_ZH: Record<string, string> = {
  SuperAdmin: '超级管理员',
  HRAdmin: '人事管理员',
  DepartmentAdmin: '部门管理员',
  TeamLead: '组长',
  Member: '成员',
  Ops: '运营',
  Designer: '设计师',
  CustomizationOperator: '定制美工',
  Audit_A: '常规审核',
  Audit_B: '历史兼容：常规审核旧编码',
  CustomizationReviewer: '定制审核',
  Warehouse: '仓库人员',
  AssetSubmitter: '素材工作台-交付人员',
  AssetManager: '素材工作台-作品管理',
  AssetTemplateAdmin: '素材工作台-计价配置',
  AssetSettlement: '素材工作台-结算财务',
  Outsource: '外协',
  ERP: '企管(ERP)',
}

/**
 * @deprecated 历史兼容角色展示映射，仅用于后端仍返回这些角色 code 时的显示兜底。
 * 禁止在新逻辑中使用。
 */
export const LEGACY_ROLE_API_LABEL_ZH: Record<string, string> = {
  Admin: '管理员',
  OrgAdmin: '组织管理员',
  RoleAdmin: '角色管理员',
  DesignDirector: '设计总监',
  DesignReviewer: '设计审核',
}

/**
 * 前端主角色/会话中的 snake_case 与 `frontend_access` 小写名 → 中文（仅展示）。
 */
const ROLE_SLUG_TO_ZH: Record<string, string> = {
  super_admin: '超级管理员',
  superadmin: '超级管理员',
  hr_admin: '人事管理员',
  dept_admin: '部门管理员',
  group_leader: '组长',
  teamlead: '组长',
  member: '成员',
  ops: '运营',
  designer: '设计师',
  customization_operator: '定制美工',
  audit_a: '常规审核',
  audit_b: '历史兼容：常规审核旧编码',
  auditor: '常规审核',
  warehouse: '仓库人员',
  outsource: '外协',
  erp: '企管(ERP)',
  customization_reviewer: '定制审核',
  customizationreviewer: '定制审核',
  asset_submitter: '素材工作台-交付人员',
  assetsubmitter: '素材工作台-交付人员',
  asset_manager: '素材工作台-作品管理',
  assetmanager: '素材工作台-作品管理',
  asset_template_admin: '素材工作台-计价配置',
  assettemplateadmin: '素材工作台-计价配置',
  asset_settlement: '素材工作台-结算财务',
  assetsettlement: '素材工作台-结算财务',
}

function labelTableInsensitive(
  table: Record<string, string>,
  key: string,
): string | null {
  const direct = table[key]
  if (direct) return direct
  const k = Object.keys(table).find((x) => x.toLowerCase() === key.toLowerCase())
  return k && table[k] !== undefined ? table[k] : null
}

function reverseCodeToDisplayInsensitive(api: string): string | null {
  const hit = Object.entries(WORKFLOW_ROLE_DISPLAY_TO_API).find(
    ([, code]) => code.toLowerCase() === api.toLowerCase(),
  )
  return hit ? hit[0] : null
}

export function workflowRoleDisplayToApi(display: string): string {
  return WORKFLOW_ROLE_DISPLAY_TO_API[display] ?? display
}

export function workflowRoleApiToDisplay(api: string): string {
  const t = String(api ?? '').trim()
  if (!t) return ''
  const slug = t.toLowerCase()
  if (ROLE_SLUG_TO_ZH[slug]) return ROLE_SLUG_TO_ZH[slug]

  const fromActive = labelTableInsensitive(WORKFLOW_ROLE_API_LABEL_ZH, t)
  if (fromActive) return fromActive
  const fromLegacy = labelTableInsensitive(LEGACY_ROLE_API_LABEL_ZH, t)
  if (fromLegacy) return fromLegacy
  return reverseCodeToDisplayInsensitive(t) ?? t
}

/**
 * 主角色/单条 role code 的界面文案（如个人中心、用户列表的「角色」列）。
 * 不修改、不传参；与接口值可能为 snake_case 或 V7Role PascalCase 均可。
 */
export function formatUserRoleForDisplay(role: string | null | undefined): string {
  const t = String(role ?? '').trim()
  if (!t) return '-'
  return workflowRoleApiToDisplay(t) || '-'
}

/**
 * 列表/弹层展示：将接口返回的 role code 转为界面用标签（全站展示层统一入口；不传/提交仍用 code）。
 */
export function formatWorkflowRolesForDisplay(apiRoles: string[] | undefined): string {
  if (!apiRoles?.length) return '无'
  return apiRoles.map(workflowRoleApiToDisplay).join('、')
}
