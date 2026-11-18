import { describe, expect, it } from 'vitest'
import type { AccessAssignment, AccessRole } from '@/services/api/accessPolicyApi'
import { planOARoleReplacement } from './userRoles'

const roles: AccessRole[] = [
  { id: 1, code: 'operations', name: '运营', description: '', permissions: [], version: 1, system_protected: false },
  { id: 2, code: 'custom_specialty', name: '专项查询', description: '', permissions: [], version: 1, system_protected: false },
]
const scoped: AccessAssignment = {
  role_id: 2, source_type: 'direct', scope_mode: 'selected_org',
  subjects: [{ subject_type: 'team', subject_id: 15 }, { subject_type: 'department', subject_id: 21 }],
}
function plan(rows: AccessAssignment[]) {
  return planOARoleReplacement(rows, roles, new Set(['operations']), new Set(['operations']), () => 'global')
}

describe('OA role replacement', () => {
  it('retains direct specialist organization scopes and leaves inherited rows for server-side projection', () => {
    const orgGrant: AccessAssignment = { role_id: 2, source_type: 'org_policy', scope_mode: 'own_team', subjects: [] }
    expect(plan([scoped, orgGrant])).toEqual({ assignments: [
      { role_id: 2, scope_mode: 'selected_org', subjects: scoped.subjects },
      { role_id: 1, scope_mode: 'global', subjects: [] },
    ] })
  })

  it('keeps the original single scope from a migration assignment and merges duplicate selected organizations', () => {
    const legacy = { ...scoped, source_type: 'migration' }
    const second: AccessAssignment = { ...scoped, subjects: [...scoped.subjects, { subject_type: 'team', subject_id: 42 }] }
    expect(plan([legacy]).assignments?.[0].subjects).toEqual(scoped.subjects)
    expect(plan([legacy, second]).assignments?.[0].subjects).toEqual(second.subjects)
  })

  it('keeps an existing global grant regardless of the order of narrower scopes', () => {
    const team: AccessAssignment = { ...scoped, scope_mode: 'own_team', subjects: [] }
    const global: AccessAssignment = { ...scoped, scope_mode: 'global', subjects: [] }
    expect(plan([scoped, team, global]).assignments?.[0]).toEqual({ role_id: 2, scope_mode: 'global', subjects: [] })
  })

  it('blocks incompatible dynamic scope combinations and unknown roles', () => {
    expect(plan([scoped, { ...scoped, scope_mode: 'own_team', subjects: [] }])).toMatchObject({ blocked: expect.stringContaining('多种组织范围') })
    expect(plan([{ ...scoped, role_id: 17 }])).toMatchObject({ blocked: expect.stringContaining('未列出') })
  })
})
