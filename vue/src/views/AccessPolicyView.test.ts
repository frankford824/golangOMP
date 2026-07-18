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
vi.mock('@/services/api/orgApi', () => ({
  fetchOrgOwnershipOptions: vi.fn(async () => ({
    departmentOptions: [{ value: '10', label: '设计部' }],
    teamOptions: [{ value: '11', label: '一组', department: '10' }],
    departmentRecords: [{ id: '10', name: '设计部' }],
    teamRecords: [{ id: '11', name: '一组', departmentId: '10', departmentName: '设计部' }],
  })),
}))
vi.mock('@/stores/permissions', () => ({ usePermissionsStore: () => ({ currentUser: { id: 1 } }) }))

import AccessPolicyView from './AccessPolicyView.vue'

const roles = [
  { id: 10, code: 'member', name: '普通成员', description: '', system_protected: true, version: 1, permissions: [{ code: 'account.use' }] },
  { id: 20, code: 'auditor', name: '审核', description: '处理审核', system_protected: false, version: 2, permissions: [{ code: 'task.audit', task_types: ['original_product_development'] }] },
  { id: 30, code: 'asset', name: '资产', description: '管理资产', system_protected: false, version: 3, permissions: [{ code: 'asset.view' }] },
]

const directAssignments = [
  { id: 1, role_id: 20, role_name: '审核', scope_mode: 'selected_org' as const, subjects: [{ subject_type: 'department' as const, subject_id: 10 }, { subject_type: 'team' as const, subject_id: 11 }], source_type: 'direct' },
  { id: 2, role_id: 30, role_name: '资产', scope_mode: 'self' as const, subjects: [], source_type: 'direct' },
]

function tab(wrapper: ReturnType<typeof mount>, name: string) {
  const result = wrapper.findAll('.tabs button').find((button) => button.text() === name)
  if (!result) throw new Error(`missing tab ${name}`)
  return result
}

describe('AccessPolicyView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.permissions.mockResolvedValue([
      { code: 'asset.view', module: 'asset', name: '查看资产', description: '查看授权范围内资产', risk_level: 'normal', enabled: true },
      { code: 'task.audit', module: 'task', name: '审核任务', description: '通过或打回设计', risk_level: 'high', enabled: true },
    ])
    api.roles.mockResolvedValue(roles)
    api.effective.mockImplementation(async (userId: number) => userId === 2 ? {
      user_id: 2,
      policy_revision: 8,
      permissions: ['task.audit', 'asset.view'],
      assignments: [...directAssignments, { id: 3, role_id: 10, role_name: '普通成员', scope_mode: 'own_department', subjects: [], source_type: 'org_policy' }],
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
  })

  it('shows task-type picker for scoped operations on the role panel', async () => {
    const wrapper = mount(AccessPolicyView)
    await flushPromises()
    const auditor = wrapper.findAll('.role-list > button').find((button) => button.text().includes('审核'))
    await auditor!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('允许的任务类型')
    expect(wrapper.text()).toContain('原品开发')
  })
})
