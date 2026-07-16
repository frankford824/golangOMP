// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  permissions: vi.fn(), roles: vi.fn(), effective: vi.fn(), searchUsers: vi.fn(),
  replaceUserAssignments: vi.fn(), orgPolicies: vi.fn(), replaceOrgPolicies: vi.fn(),
  createRole: vi.fn(), updateRole: vi.fn(), archiveRole: vi.fn(), replaceRolePermissions: vi.fn(), events: vi.fn(),
}))
vi.mock('@/services/api/accessPolicyApi', async (loadOriginal) => ({
  ...(await loadOriginal<typeof import('@/services/api/accessPolicyApi')>()),
  accessPolicyApi: api,
}))
vi.mock('@/stores/permissions', () => ({ usePermissionsStore: () => ({ currentUser: { id: 1 } }) }))

import AccessPolicyView from './AccessPolicyView.vue'

const roles = [
  { id: 10, code: 'member', name: '成员', description: '', system_protected: true, version: 1, permissions: ['account.use'] },
  { id: 20, code: 'reviewer', name: '审核人员', description: '处理审核', system_protected: false, version: 2, permissions: ['task.audit.decision'] },
  { id: 30, code: 'asset_manager', name: '资产管理员', description: '管理资产', system_protected: false, version: 3, permissions: ['asset.view'] },
]

const directAssignments = [
  { id: 1, role_id: 20, role_name: '审核人员', scope_mode: 'selected_org' as const, subjects: [{ subject_type: 'department' as const, subject_id: 10 }, { subject_type: 'team' as const, subject_id: 11 }], source_type: 'direct' },
  { id: 2, role_id: 30, role_name: '资产管理员', scope_mode: 'self' as const, subjects: [], source_type: 'direct' },
]

function tab(wrapper: ReturnType<typeof mount>, name: string) {
  const result = wrapper.findAll('.tabs button').find((button) => button.text() === name)
  if (!result) throw new Error(`missing tab ${name}`)
  return result
}

describe('AccessPolicyView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.permissions.mockResolvedValue([{ code: 'asset.view', module: 'asset', name: '查看资产', description: '查看授权范围内资产', risk_level: 'normal', enabled: true }])
    api.roles.mockResolvedValue(roles)
    api.effective.mockImplementation(async (userId: number) => userId === 2 ? {
      user_id: 2,
      policy_revision: 8,
      permissions: ['task.audit.decision', 'asset.view'],
      assignments: [...directAssignments, { id: 3, role_id: 10, role_name: '成员', scope_mode: 'own_department', subjects: [], source_type: 'org_policy' }],
      sources: [],
    } : { user_id: 1, policy_revision: 8, permissions: [], assignments: [], sources: [] })
    api.searchUsers.mockResolvedValue([{ id: 2, display_name: '李审核', department: '设计部', department_id: 10 }])
    api.replaceUserAssignments.mockResolvedValue({ policy_revision: 9 })
    api.orgPolicies.mockResolvedValue([
      { id: 1, subject_type: 'department', subject_id: 10, role_id: 20, scope_mode: 'own_department', enabled: true },
      { id: 2, subject_type: 'department', subject_id: 10, role_id: 30, scope_mode: 'selected_org', enabled: false },
    ])
    api.replaceOrgPolicies.mockImplementation(async (_type: string, _id: number, policies: unknown[]) => ({ policy_revision: 9, org_policies: policies }))
    api.createRole.mockResolvedValue({ policy_revision: 9, role: roles[1] })
    api.updateRole.mockResolvedValue({ policy_revision: 9, role: roles[1] })
    api.archiveRole.mockResolvedValue({ policy_revision: 9, role: { ...roles[1], archived_at: 'now' } })
    api.replaceRolePermissions.mockResolvedValue({ policy_revision: 9 })
    api.events.mockResolvedValue([{ id: 1, policy_revision: 8, actor_id: 1, action: 'assignment.replace', target_type: 'user', target_id: '2', reason: '职责调整', created_at: '2026-07-16T00:00:00Z' }])
  })

  async function chooseUser(wrapper: ReturnType<typeof mount>) {
    await tab(wrapper, '人员授权').trigger('click')
    await wrapper.get('.panel-head input').setValue('李')
    await wrapper.get('.panel-head form').trigger('submit')
    await flushPromises()
    await wrapper.get('.user-results button').trigger('click')
    await flushPromises()
  }

  it('preserves every direct role and stable-ID subject while keeping organization policy read-only', async () => {
    const wrapper = mount(AccessPolicyView)
    await flushPromises()
    await chooseUser(wrapper)

    expect(wrapper.findAll('.assignment-card')).toHaveLength(2)
    expect(wrapper.findAll('.subject-row')).toHaveLength(2)
    expect(wrapper.get('.inherited').text()).toContain('组织策略带来的权限（只读）')
    await wrapper.get('.assignment-editor > label.wide input').setValue('调整审核范围')
    await wrapper.get('.assignment-editor').trigger('submit')
    await flushPromises()

    expect(api.replaceUserAssignments).toHaveBeenCalledWith(2, [
      { role_id: 20, scope_mode: 'selected_org', subjects: [{ subject_type: 'department', subject_id: 10 }, { subject_type: 'team', subject_id: 11 }] },
      { role_id: 30, scope_mode: 'self', subjects: [] },
    ], 8, '调整审核范围')
    expect(api.replaceUserAssignments.mock.calls[0][1]).not.toEqual(expect.arrayContaining([expect.objectContaining({ source_type: 'org_policy' })]))
  })

  it('removes one direct role without losing the other and explains duplicate-role conflicts', async () => {
    const wrapper = mount(AccessPolicyView)
    await flushPromises()
    await chooseUser(wrapper)

    const roleSelects = wrapper.findAll('.assignment-card select').filter((select) => select.element.closest('label')?.textContent?.includes('业务角色'))
    await roleSelects[1].setValue('20')
    expect(wrapper.text()).toContain('同一个业务角色只能配置一次')
    expect(wrapper.get('.assignment-editor .primary').attributes('disabled')).toBeDefined()

    await roleSelects[1].setValue('30')
    await wrapper.get('[aria-label="删除第 1 条直接授权"]').trigger('click')
    await wrapper.get('.assignment-editor > label.wide input').setValue('取消审核职责')
    await wrapper.get('.assignment-editor').trigger('submit')
    await flushPromises()
    expect(api.replaceUserAssignments).toHaveBeenLastCalledWith(2, [{ role_id: 30, scope_mode: 'self', subjects: [] }], 8, '取消审核职责')
  })

  it('loads and replaces the complete organization-policy collection', async () => {
    const wrapper = mount(AccessPolicyView)
    await flushPromises()
    await tab(wrapper, '组织策略').trigger('click')
    await wrapper.get('.org-selector input').setValue('10')
    await wrapper.get('.org-selector').trigger('submit')
    await flushPromises()
    expect(wrapper.findAll('.org-policy-card')).toHaveLength(2)

    const scopes = wrapper.findAll('.org-policy-card select').filter((select) => select.element.closest('label')?.textContent?.includes('生效范围'))
    await scopes[0].setValue('global')
    await wrapper.get('.org-editor > label.wide input').setValue('仅调整第一条')
    await wrapper.get('.org-editor').trigger('submit')
    await flushPromises()
    expect(api.replaceOrgPolicies).toHaveBeenCalledWith('department', 10, [
      expect.objectContaining({ id: 1, role_id: 20, scope_mode: 'global', enabled: true }),
      expect.objectContaining({ id: 2, role_id: 30, scope_mode: 'selected_org', enabled: false }),
    ], 8, '仅调整第一条')
  })

  it('offers role lifecycle and searchable audit records without engineering jargon', async () => {
    const wrapper = mount(AccessPolicyView)
    await flushPromises()
    expect(wrapper.text()).not.toContain('乐观锁')
    expect(wrapper.text()).not.toContain('409')
    await wrapper.findAll('.role-list button').find((button) => button.text() === '新建')?.trigger('click')
    const dialogInputs = wrapper.findAll('.dialog input')
    await dialogInputs[0].setValue('designer_reviewer')
    await dialogInputs[1].setValue('设计审核')
    await dialogInputs[2].setValue('新增审核职责')
    await wrapper.get('.dialog').trigger('submit')
    await flushPromises()
    expect(api.createRole).toHaveBeenCalledWith(expect.objectContaining({ code: 'designer_reviewer', name: '设计审核', reason: '新增审核职责' }))

    await tab(wrapper, '变更记录').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('职责调整')
  })
})
