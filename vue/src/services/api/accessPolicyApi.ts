import http from '@/services/http'

export type ScopeMode = 'self' | 'own_department' | 'own_team' | 'selected_org' | 'global'
export type ScopeSubjectType = 'department' | 'team'

export interface AccessPermission {
  code: string
  module: string
  name: string
  description: string
  risk_level: 'normal' | 'high'
  enabled: boolean
}

export interface AccessRolePermission {
  code: string
  task_types?: string[]
}

export interface AccessRole {
  id: number
  code: string
  name: string
  description: string
  system_protected: boolean
  version: number
  archived_at?: string | null
  permissions: AccessRolePermission[]
}

export interface AccessAssignment {
  id?: number
  user_id?: number
  role_id: number
  role_code?: string
  role_name?: string
  scope_mode: ScopeMode
  subjects: Array<{ subject_type: ScopeSubjectType; subject_id: number; subject_name?: string }>
  source_type?: 'direct' | 'migration' | 'org_policy' | string
  source_ref_id?: number
  version?: number
}

export interface EffectiveAccess {
  user_id: number
  policy_revision: number
  permissions: string[]
  assignments: AccessAssignment[]
  sources: Array<Record<string, unknown>>
}

export interface OrgPolicy {
  id?: number
  subject_type: ScopeSubjectType
  subject_id: number
  role_id: number
  scope_mode: ScopeMode
  enabled: boolean
  version?: number
  reason?: string
}

export interface AccessPolicyEvent {
  id: number
  policy_revision: number
  actor_id: number
  action: string
  target_type: string
  target_id: string
  reason: string
  created_at: string
}

export interface AccessUserOption {
  id: number
  display_name?: string
  username?: string
  department?: string
  department_id?: number
  team?: string
  team_id?: number
}

/** Operations that may carry a per-role task-type matrix. */
export const TASK_TYPE_SCOPED_OPERATIONS = new Set([
  'task.create',
  'task.assign',
  'task.reassign',
  'task.terminate',
  'task.upload_source',
  'task.audit',
])

export const TASK_TYPE_OPTIONS = [
  { value: 'original_product_development', label: '原品开发' },
  { value: 'new_product_development', label: '新品开发' },
  { value: 'retouch_task', label: '修图任务' },
  { value: 'customer_customization', label: '客户定制' },
  { value: 'regular_customization', label: '常规定制' },
  { value: 'sku_planning', label: '策划 SKU' },
] as const

const unwrap = <T>(response: { data?: { data?: T } | T }): T => {
  const body = response.data as { data?: T } | T
  return body && typeof body === 'object' && 'data' in body ? (body.data as T) : (body as T)
}

function normalizeRolePermissions(raw: unknown): AccessRolePermission[] {
  if (!Array.isArray(raw)) return []
  return raw.map((item) => {
    if (typeof item === 'string') return { code: item, task_types: [] }
    const row = item as AccessRolePermission
    return { code: String(row.code || ''), task_types: Array.isArray(row.task_types) ? row.task_types.map(String) : [] }
  }).filter((item) => item.code)
}

function normalizeRole(role: AccessRole): AccessRole {
  return { ...role, permissions: normalizeRolePermissions(role.permissions) }
}

export const accessPolicyApi = {
  async permissions(): Promise<AccessPermission[]> {
    return unwrap(await http.get('/v1/access/permissions'))
  },
  async roles(includeArchived = false): Promise<AccessRole[]> {
    const rows = unwrap<AccessRole[]>(await http.get('/v1/access/roles', { params: { include_archived: includeArchived } }))
    return (rows || []).map(normalizeRole)
  },
  async createRole(payload: {
    code: string
    name: string
    description: string
    permissions: AccessRolePermission[]
    reason: string
    expected_policy_revision: number
  }): Promise<{ policy_revision: number; role: AccessRole }> {
    const result = unwrap<{ policy_revision: number; role: AccessRole }>(await http.post('/v1/access/roles', payload))
    return { ...result, role: normalizeRole(result.role) }
  },
  async updateRole(role: AccessRole, name: string, description: string, policyRevision: number, reason: string): Promise<{ policy_revision: number; role: AccessRole }> {
    const result = unwrap<{ policy_revision: number; role: AccessRole }>(await http.patch(`/v1/access/roles/${role.id}`, {
      name, description, expected_version: role.version, expected_policy_revision: policyRevision, reason,
    }))
    return { ...result, role: normalizeRole(result.role) }
  },
  async archiveRole(role: AccessRole, policyRevision: number, reason: string): Promise<{ policy_revision: number; role: AccessRole }> {
    return unwrap(await http.post(`/v1/access/roles/${role.id}/archive`, {
      expected_version: role.version, expected_policy_revision: policyRevision, reason,
    }))
  },
  async replaceRolePermissions(role: AccessRole, permissions: AccessRolePermission[], policyRevision: number, reason: string) {
    return unwrap(await http.put(`/v1/access/roles/${role.id}/permissions`, {
      permissions,
      expected_role_version: role.version,
      expected_policy_revision: policyRevision,
      reason,
    }))
  },
  async effective(userId: number): Promise<EffectiveAccess> {
    return unwrap(await http.get(`/v1/access/users/${userId}/effective`))
  },
  async replaceUserAssignments(userId: number, assignments: AccessAssignment[], policyRevision: number, reason: string) {
    return unwrap(await http.put(`/v1/access/users/${userId}/assignments`, {
      assignments,
      expected_policy_revision: policyRevision,
      reason,
    }))
  },
  async searchUsers(keyword: string): Promise<AccessUserOption[]> {
    const body = unwrap<AccessUserOption[] | { items?: AccessUserOption[] }>(await http.get('/v1/access/users', { params: { q: keyword, page: 1, page_size: 30 } }))
    return Array.isArray(body) ? body : body.items || []
  },
  async orgPolicies(subjectType: ScopeSubjectType, subjectId: number): Promise<OrgPolicy[]> {
    return unwrap(await http.get(`/v1/access/org-policies/${subjectType}/${subjectId}`))
  },
  async replaceOrgPolicies(subjectType: ScopeSubjectType, subjectId: number, policies: OrgPolicy[], policyRevision: number, reason: string): Promise<{ policy_revision: number; org_policies: OrgPolicy[] }> {
    return unwrap(await http.put(`/v1/access/org-policies/${subjectType}/${subjectId}`, { policies, expected_policy_revision: policyRevision, reason }))
  },
  async events(limit = 100): Promise<AccessPolicyEvent[]> {
    return unwrap(await http.get('/v1/access/events', { params: { limit } }))
  },
}
