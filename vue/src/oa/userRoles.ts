import type { AccessAssignment, AccessRole, ScopeMode } from '@/services/api/accessPolicyApi'

export const coreUserRoleGroups = [
  { category: 'business', title: '主业务角色', codes: ['customization_operator', 'auditor', 'designer', 'operations'] },
  { category: 'asset_workbench', title: '素材业务角色', codes: ['asset_submitter', 'asset_manager', 'asset_settlement'] },
  { category: 'management', title: '管理权限', codes: ['department_admin', 'super_admin'] },
]

const validModes = new Set<ScopeMode>(['self', 'own_department', 'own_team', 'selected_org', 'global'])

export function planOARoleReplacement(
  assignments: AccessAssignment[],
  roles: AccessRole[],
  selected: Set<string>,
  editable: Set<string>,
  defaultScope: (code: string) => ScopeMode,
): { assignments: AccessAssignment[]; blocked?: never } | { blocked: string; assignments?: never } {
  const known = new Map(roles.map(role => [role.id, role]))
  const retained = new Map<number, AccessAssignment>()
  const global = new Set(assignments.filter(row => row.source_type !== 'org_policy' && row.scope_mode === 'global').map(row => row.role_id))
  const existing = new Set<number>()
  for (const assignment of assignments) {
    if (assignment.source_type === 'org_policy') continue
    if (assignment.source_type !== 'direct' && assignment.source_type !== 'migration') {
      return { blocked: '现有授权有未知来源，已保留原授权。' }
    }
    const role = known.get(assignment.role_id)
    if (!role || role.archived_at) return { blocked: '现有授权包含未列出或已停用的角色，批量保存无法原样保留。' }
    existing.add(role.id)
    if (editable.has(role.code) && !selected.has(role.code)) continue
    const subjects = assignment.subjects ?? []
    if (!validModes.has(assignment.scope_mode)
      || (assignment.scope_mode === 'selected_org') !== (subjects.length > 0)
      || subjects.some(subject => !['department', 'team'].includes(subject.subject_type) || !Number.isSafeInteger(subject.subject_id) || subject.subject_id < 1)) {
      return { blocked: '现有授权的组织范围无法通过此页面完整提交。已保留原授权。' }
    }
    if (global.has(role.id) && assignment.scope_mode !== 'global') continue
    const current = retained.get(role.id)
    if (!current) {
      retained.set(role.id, {
        role_id: role.id,
        scope_mode: assignment.scope_mode,
        subjects: [...new Map(subjects.map(({ subject_type, subject_id }) => [`${subject_type}:${subject_id}`, { subject_type, subject_id }])).values()],
      })
      continue
    }
    if (current.scope_mode !== assignment.scope_mode) {
      return { blocked: '同一角色有多种组织范围，现有接口无法保留其组合。已保留原授权。' }
    }
    const scope = new Map(current.subjects.map(subject => [`${subject.subject_type}:${subject.subject_id}`, subject]))
    for (const { subject_type, subject_id } of subjects) scope.set(`${subject_type}:${subject_id}`, { subject_type, subject_id })
    current.subjects = [...scope.values()]
  }
  const next = [...retained.values()]
  for (const role of roles) {
    if (existing.has(role.id) || !editable.has(role.code) || !selected.has(role.code)) continue
    next.push({ role_id: role.id, scope_mode: defaultScope(role.code), subjects: [] })
  }
  return { assignments: next }
}
