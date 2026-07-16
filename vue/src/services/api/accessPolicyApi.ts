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

export interface AccessRole {
  id: number
  code: string
  name: string
  description: string
  system_protected: boolean
  version: number
  archived_at?: string | null
  permissions: string[]
}

export interface AccessAssignment {
	 id?: number
	 user_id?: number
	role_id: number
	role_code?: string
	role_name?: string
	scope_mode: ScopeMode
	subjects: Array<{ subject_type: ScopeSubjectType; subject_id: number }>
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

export interface AccessUserOption { id: number; display_name?: string; username?: string; department?: string; department_id?: number; team?: string; team_id?: number }

const unwrap = <T>(response: { data?: { data?: T } | T }): T => {
  const body = response.data as { data?: T } | T
  return body && typeof body === 'object' && 'data' in body ? (body.data as T) : (body as T)
}

export const accessPolicyApi = {
  async permissions(): Promise<AccessPermission[]> {
    return unwrap(await http.get('/v1/access/permissions'))
  },
  async roles(includeArchived = false): Promise<AccessRole[]> {
    return unwrap(await http.get('/v1/access/roles', { params: { include_archived: includeArchived } }))
  },
  async createRole(payload: { code: string; name: string; description: string; permissions: string[]; reason: string; expected_policy_revision: number }): Promise<{ policy_revision: number; role: AccessRole }> {
    return unwrap(await http.post('/v1/access/roles', payload))
  },
  async updateRole(role: AccessRole, name: string, description: string, policyRevision: number, reason: string): Promise<{ policy_revision: number; role: AccessRole }> {
    return unwrap(await http.patch(`/v1/access/roles/${role.id}`, { name, description, expected_version: role.version, expected_policy_revision: policyRevision, reason }))
  },
  async archiveRole(role: AccessRole, policyRevision: number, reason: string): Promise<{ policy_revision: number; role: AccessRole }> {
    return unwrap(await http.post(`/v1/access/roles/${role.id}/archive`, { expected_version: role.version, expected_policy_revision: policyRevision, reason }))
  },
  async replaceRolePermissions(role: AccessRole, permissions: string[], policyRevision: number, reason: string) {
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
