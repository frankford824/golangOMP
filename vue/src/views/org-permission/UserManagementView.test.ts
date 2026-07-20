// @vitest-environment jsdom
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import UserManagementView from './UserManagementView.vue'
import { usersApi } from '@/services/api/usersApi'
import {
  createOrgDepartment,
  createOrgTeam,
  deleteOrgDepartment,
  deleteOrgTeam,
  fetchOrgOwnershipOptions,
  updateOrgDepartment,
  updateOrgTeam,
} from '@/services/api/orgApi'
import { patchUserMembership } from '@/composables/useOrgPermissionData'
import { usePermissionsStore } from '@/stores/permissions'
import { DataScopeEnum, RoleEnum, type PermissionUser } from '@/types'
import { accessPolicyApi } from '@/services/api/accessPolicyApi'

const permissionMock = vi.hoisted(() => ({
  allowedActions: new Set<string>(),
  currentUser: {
    value: { id: '1' },
  },
}))
const routerMock = vi.hoisted(() => ({ replace: vi.fn(), query: {} as Record<string, string> }))
vi.mock('vue-router', () => ({
  useRoute: () => ({ query: routerMock.query }),
  useRouter: () => ({ replace: routerMock.replace }),
}))

vi.mock('@/composables/usePermission', () => ({
  usePermission: () => ({
    can: (key: string | string[]) => {
      const keys = Array.isArray(key) ? key : [key]
      return keys.some((item) => permissionMock.allowedActions.has(item))
    },
    currentUser: permissionMock.currentUser,
  }),
}))

vi.mock('@/services/api/usersApi', () => ({
  usersApi: {
    listRoles: vi.fn(),
    list: vi.fn(),
    getById: vi.fn(),
    patch: vi.fn(),
    replaceRoles: vi.fn(),
    create: vi.fn(),
    deactivate: vi.fn(),
    activate: vi.fn(),
    resetPassword: vi.fn(),
  },
}))

vi.mock('@/services/api/accessPolicyApi', () => ({
  accessPolicyApi: {
    roles: vi.fn(),
    effective: vi.fn(),
    replaceUserAssignments: vi.fn(),
    orgPolicies: vi.fn(),
    replaceOrgPolicies: vi.fn(),
  },
}))

vi.mock('@/services/api/orgApi', () => ({
  fetchOrgOwnershipOptions: vi.fn(async () => ({
    departmentOptions: [{ value: 'Design', label: '设计部' }],
    teamOptions: [
      { value: 'Design-A', label: '设计一组', department: 'Design' },
      { value: 'legacy-floating', label: '旧无部门组' },
    ],
    departmentRecords: [{ id: '1', name: 'Design', enabled: true }],
    teamRecords: [{ id: '2', name: 'Design-A', departmentId: '1', departmentName: 'Design', enabled: true }],
  })),
  createOrgDepartment: vi.fn(),
  createOrgTeam: vi.fn(),
  updateOrgDepartment: vi.fn(),
  updateOrgTeam: vi.fn(),
  mergeOrgDepartment: vi.fn(),
  mergeOrgTeam: vi.fn(),
  deleteOrgDepartment: vi.fn(),
  deleteOrgTeam: vi.fn(),
  departmentsAndGroupsFromOrgOptions: vi.fn(() => ({
    departments: [{ id: 'Design', name: '设计部' }],
    groups: [{ id: 'Design-A', name: '设计一组', departmentId: 'Design' }],
  })),
}))

vi.mock('@/composables/useOrgPermissionData', () => ({
  patchUserMembership: vi.fn(),
  clearUserMembership: vi.fn(),
}))

const BaseButtonStub = {
  props: ['disabled'],
  emits: ['click'],
  template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>',
}

const BaseInputStub = {
  props: ['modelValue'],
  emits: ['update:modelValue'],
  template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
}

const BaseSelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  template: `
    <select :value="modelValue" @change="$emit('update:modelValue', $event.target.value)">
      <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
    </select>
  `,
}

const BaseDataTableStub = {
  template: '<div data-test="user-table"></div>',
}

const EmptyStub = {
  template: '<div><slot /></div>',
}

const BaseErrorStateStub = {
  props: ['title'],
  template: '<div>{{ title }}<slot /></div>',
}

function mountView(user: PermissionUser = {
  id: '1',
  name: 'HR',
  role: RoleEnum.HR_ADMIN,
  departmentId: 'Design',
  groupId: 'Design-A',
  dataScope: DataScopeEnum.GLOBAL,
  permissions: [],
}) {
  const pinia = createPinia()
  setActivePinia(pinia)
  const store = usePermissionsStore()
  store.setCurrentUser(user)
  return mount(UserManagementView, {
    global: {
      plugins: [pinia],
      stubs: {
        BaseButton: BaseButtonStub,
        BaseInput: BaseInputStub,
        BaseSelect: BaseSelectStub,
        BaseDataTable: BaseDataTableStub,
        BaseTablePager: EmptyStub,
        BaseSkeleton: EmptyStub,
        BaseEmptyState: EmptyStub,
        BaseErrorState: BaseErrorStateStub,
      },
    },
  })
}

describe('UserManagementView role governance', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    vi.clearAllMocks()
    permissionMock.allowedActions.clear()
    permissionMock.allowedActions.add('role.assign')
    permissionMock.allowedActions.add('role.read')
    permissionMock.allowedActions.add('user.manage')
    permissionMock.allowedActions.add('org.manage')
    permissionMock.allowedActions.add('access.view')
    permissionMock.allowedActions.add('access.manage')
    vi.mocked(usersApi.listRoles).mockResolvedValue({
      data: {
        data: [
          {
            role: 'Member',
            name: 'Member',
            category: 'business',
            assignable: true,
            assignable_by_current_actor: true,
            deprecated: false,
            hidden_by_default: false,
          },
          {
            role: 'AssetSubmitter',
            name: '素材工作台-交付人员',
            category: 'asset_workbench',
            assignable: true,
            assignable_by_current_actor: true,
            deprecated: false,
            hidden_by_default: false,
          },
          {
            role: 'Audit_B',
            name: '历史兼容：常规审核旧编码',
            category: 'compatibility',
            assignable: false,
            assignable_by_current_actor: false,
            deprecated: true,
            hidden_by_default: true,
            assignment_note: '常规审核旧编码，不允许新增分配',
          },
          {
            role: 'Outsource',
            name: 'Outsource',
            category: 'compatibility',
            assignable: false,
            assignable_by_current_actor: false,
            deprecated: true,
            hidden_by_default: true,
            assignment_note: '历史兼容角色',
          },
          {
            role: 'SuperAdmin',
            name: 'Super Admin',
            category: 'management',
            assignable: true,
            assignable_by_current_actor: false,
            deprecated: false,
            hidden_by_default: false,
          },
        ],
      },
    } as never)
    vi.mocked(usersApi.list).mockResolvedValue({
      data: { data: [], pagination: { total: 0, page: 1, page_size: 20 } },
    } as never)
    vi.mocked(usersApi.getById).mockResolvedValue({
      data: {
        data: {
          id: '2',
          username: 'target',
          display_name: '目标用户',
          department: 'Design',
          team: 'Design-A',
          roles: ['Member', 'Audit_B', 'Outsource', 'SuperAdmin'],
          status: 'active',
        },
      },
    } as never)
    vi.mocked(usersApi.replaceRoles).mockResolvedValue({
      data: {
        data: {
          id: '2',
          username: 'target',
          roles: ['Member', 'Audit_B', 'Outsource', 'SuperAdmin'],
          status: 'active',
        },
      },
    } as never)
    vi.mocked(accessPolicyApi.roles).mockResolvedValue([
      { id: 1, code: 'member', name: '成员', description: '基础身份', system_protected: true, version: 1, permissions: [] },
      { id: 2, code: 'operations', name: '运营', description: '任务创建和运营处理', system_protected: false, version: 1, permissions: [] },
      { id: 3, code: 'auditor', name: '审核员', description: '统一任务审核', system_protected: false, version: 1, permissions: [] },
      { id: 4, code: 'asset_manager', name: '素材管理员', description: '素材管理与发布', system_protected: false, version: 1, permissions: [] },
      { id: 5, code: 'super_admin', name: '超级管理员', description: '系统保护角色', system_protected: true, version: 1, permissions: [] },
    ])
    vi.mocked(accessPolicyApi.effective).mockImplementation(async (userId: number) => ({
      user_id: userId,
      policy_revision: 7,
      permissions: ['access.manage'],
      assignments: userId === 2 ? [
        { id: 10, user_id: 2, role_id: 1, role_code: 'member', role_name: '成员', scope_mode: 'self', subjects: [], source_type: 'migration', version: 1 },
        { id: 11, user_id: 2, role_id: 2, role_code: 'operations', role_name: '运营', scope_mode: 'selected_org', subjects: [
          { subject_type: 'department', subject_id: 1 },
          { subject_type: 'team', subject_id: 2 },
        ], source_type: 'direct', version: 2 },
        { user_id: 2, role_id: 3, role_code: 'auditor', role_name: '审核员', scope_mode: 'selected_org', subjects: [
          { subject_type: 'department', subject_id: 1 },
        ], source_type: 'org_policy', source_ref_id: 20, version: 1 },
      ] : [
        { id: 1, user_id: userId, role_id: 5, role_code: 'super_admin', role_name: '超级管理员', scope_mode: 'global', subjects: [], source_type: 'direct', version: 1 },
      ],
      sources: [],
    }))
    vi.mocked(accessPolicyApi.replaceUserAssignments).mockResolvedValue({ policy_revision: 8 } as never)
    vi.mocked(accessPolicyApi.orgPolicies).mockResolvedValue([
      { id: 20, subject_type: 'department', subject_id: 1, role_id: 2, scope_mode: 'selected_org', enabled: true, version: 1 },
    ])
    vi.mocked(accessPolicyApi.replaceOrgPolicies).mockResolvedValue({ policy_revision: 8, org_policies: [] })
    vi.mocked(usersApi.patch).mockResolvedValue({ data: { data: {} } } as never)
    vi.mocked(usersApi.create).mockResolvedValue({ data: { data: {} } } as never)
    vi.mocked(usersApi.resetPassword).mockResolvedValue({ data: { data: {} } } as never)
    vi.mocked(usersApi.deactivate).mockResolvedValue({ data: {} } as never)
    vi.mocked(usersApi.activate).mockResolvedValue({ data: {} } as never)
    vi.mocked(createOrgDepartment).mockResolvedValue({ id: '3', name: '内容部' } as never)
    vi.mocked(createOrgTeam).mockResolvedValue({ id: '4', name: '内容一组', department_id: '1' } as never)
    vi.mocked(updateOrgDepartment).mockResolvedValue(undefined as never)
    vi.mocked(updateOrgTeam).mockResolvedValue(undefined as never)
    vi.mocked(patchUserMembership).mockResolvedValue(undefined as never)
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('keeps role assignment inside user detail and preserves direct scopes without copying inherited roles', async () => {
    const wrapper = mountView()
    await flushPromises()

    await (wrapper.vm as unknown as { openDetail: (row: unknown) => Promise<void> }).openDetail({
      id: '2',
      username: 'target',
      roles: [],
    })
    await flushPromises()

    const modal = document.body.querySelector('.um-modal')
    expect(modal).not.toBeNull()
    const checkboxes = Array.from(
      document.body.querySelectorAll<HTMLInputElement>('.um-modal input[type="checkbox"]'),
    )
    expect(checkboxes.map((item) => item.value)).toEqual(['super_admin', 'member', 'operations', 'auditor', 'asset_manager'])
    expect(checkboxes.find((item) => item.value === 'member')?.disabled).toBe(true)
    expect(checkboxes.find((item) => item.value === 'operations')?.checked).toBe(true)
    expect(checkboxes.find((item) => item.value === 'auditor')?.checked).toBe(false)
    const modalText = modal?.textContent ?? ''
    expect(modalText).toContain('由部门或小组自动应用')
    expect(modalText).toContain('审核员')
    expect(modalText).not.toContain('权限与范围')

    checkboxes.find((item) => item.value === 'asset_manager')?.click()
    await flushPromises()
    const saveButton = Array.from(document.body.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === '保存角色',
    )
    saveButton?.click()
    await flushPromises()

    expect(accessPolicyApi.replaceUserAssignments).toHaveBeenCalledWith(2, [
      { role_id: 1, scope_mode: 'self', subjects: [] },
      { role_id: 2, scope_mode: 'selected_org', subjects: [
        { subject_type: 'department', subject_id: 1 },
        { subject_type: 'team', subject_id: 2 },
      ] },
      { role_id: 4, scope_mode: 'global', subjects: [] },
    ], 7, '在用户详情中更新工作角色')
    expect(usersApi.replaceRoles).not.toHaveBeenCalled()
  })

  it('uses a compact organization role dialog instead of a separate permission workspace', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).not.toContain('角色与权限')

    const vm = wrapper.vm as unknown as {
      openOrgAccess: (type: 'department' | 'team', id: number) => Promise<void>
      selectedOrgPolicyRoleCodes: string[]
      saveOrgPolicyRoles: () => Promise<void>
    }
    await vm.openOrgAccess('department', 1)
    await flushPromises()

    const dialog = document.body.querySelector('.org-policy-modal')
    expect(dialog?.textContent).toContain('部门：Design · 默认角色')
    expect(dialog?.textContent).not.toContain('能力目录')
    expect(vm.selectedOrgPolicyRoleCodes).toEqual(['operations'])
    vm.selectedOrgPolicyRoleCodes.push('auditor')
    await vm.saveOrgPolicyRoles()

    expect(accessPolicyApi.replaceOrgPolicies).toHaveBeenCalledWith('department', 1, [
      { subject_type: 'department', subject_id: 1, role_id: 2, scope_mode: 'selected_org', enabled: true, version: 1 },
      { subject_type: 'department', subject_id: 1, role_id: 3, scope_mode: 'selected_org', enabled: true },
    ], 7, '在组织树中更新默认工作角色')
  })

  it('saves employee number and validates employee number in Chinese copy', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openDetail: (row: unknown) => Promise<void>
      basicForm: { display_name: string; employee_no: string }
      submitBasicInfo: () => Promise<void>
    }

    await vm.openDetail({ id: '2', username: 'target', roles: [] })
    await flushPromises()

    vm.basicForm.display_name = '目标用户改名'
    vm.basicForm.employee_no = '88'
    await vm.submitBasicInfo()
    await flushPromises()

    expect(usersApi.patch).toHaveBeenCalledWith('2', {
      display_name: '目标用户改名',
      employee_no: 88,
    })

    vi.mocked(usersApi.patch).mockClear()
    vm.basicForm.employee_no = '10000'
    await vm.submitBasicInfo()
    await flushPromises()

    expect(usersApi.patch).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('工号必须是 0 到 9999 之间的纯数字。')
  })

  it('creates users with employee number and preserves Member role', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      createForm: {
        username: string
        employee_no: string
        display_name: string
        department: string
        team: string
        mobile: string
        email: string
        password: string
        roles: string[]
        status: 'active' | 'disabled'
      }
      createUser: () => Promise<void>
    }

    Object.assign(vm.createForm, {
      username: 'new_user',
      employee_no: '99',
      display_name: '新员工',
      department: 'Design',
      team: 'Design-A',
      mobile: '13800009999',
      email: '',
      password: 'Init1234',
      roles: ['AssetSubmitter'],
      status: 'active',
    })
    await vm.createUser()
    await flushPromises()

    expect(usersApi.create).toHaveBeenCalledWith({
      username: 'new_user',
      employee_no: 99,
      display_name: '新员工',
      department: 'Design',
      team: 'Design-A',
      mobile: '13800009999',
      email: undefined,
      password: 'Init1234',
      roles: ['Member', 'AssetSubmitter'],
      status: 'active',
    })
  })

  it('requires a department before exposing create-user team options', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      createForm: { department: string; team: string }
      createTeamOptions: Array<{ value: string; department?: string }>
    }

    vm.createForm.department = ''
    await flushPromises()
    expect(vm.createTeamOptions).toEqual([])

    vm.createForm.department = 'Design'
    await flushPromises()
    expect(vm.createTeamOptions.map((option) => option.value)).toEqual(['Design-A'])
    expect(vm.createTeamOptions.every((option) => option.department === 'Design')).toBe(true)
  })

  it('calls real org master APIs for create rename and delete actions', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      orgActionName: string
      openCreateDepartment: () => void
      openCreateTeam: (department: unknown) => void
      openEditDepartment: (department: unknown) => void
      openDeleteTeam: (team: unknown) => void
      submitOrgAction: () => Promise<void>
    }

    vm.openCreateDepartment()
    vm.orgActionName = '内容部'
    await vm.submitOrgAction()
    await flushPromises()
    expect(fetchOrgOwnershipOptions).toHaveBeenCalledWith({ includeDisabled: true, throwOnError: true })
    expect(createOrgDepartment).toHaveBeenCalledWith({ name: '内容部' })

    vm.openCreateTeam({ id: '1', value: 'Design', label: '设计部', teams: [] })
    vm.orgActionName = '设计二组'
    await vm.submitOrgAction()
    await flushPromises()
    expect(createOrgTeam).toHaveBeenCalledWith({ department_id: '1', name: '设计二组' })

    vm.openEditDepartment({ id: '1', value: 'Design', label: '设计部', teams: [] })
    vm.orgActionName = '设计中心'
    await vm.submitOrgAction()
    await flushPromises()
    expect(updateOrgDepartment).toHaveBeenCalledWith('1', { name: '设计中心' })

    vm.openDeleteTeam({ id: '2', value: 'Design-A', label: '设计一组', department: 'Design' })
    await vm.submitOrgAction()
    await flushPromises()
    expect(updateOrgTeam).toHaveBeenCalledWith('2', { enabled: false })
  })

  it('keeps membership and account operations bound to their scoped endpoints', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openDetail: (row: unknown) => Promise<void>
      membershipForm: { department: string; team: string }
      resetPasswordValue: string
      submitMembership: () => Promise<void>
      resetPassword: () => Promise<void>
      setUserStatus: (status: 'active' | 'disabled') => Promise<void>
    }

    await vm.openDetail({ id: '2', username: 'target', roles: [] })
    await flushPromises()

    vm.membershipForm.department = 'Design'
    vm.membershipForm.team = 'Design-B'
    await vm.submitMembership()
    await flushPromises()
    expect(patchUserMembership).toHaveBeenCalledWith('2', 'Design', 'Design-B')

    vm.resetPasswordValue = 'NewPass123'
    await vm.resetPassword()
    await flushPromises()
    expect(usersApi.resetPassword).toHaveBeenCalledWith('2', { password: 'NewPass123' })

    await vm.setUserStatus('disabled')
    await flushPromises()
    expect(usersApi.deactivate).toHaveBeenCalledWith('2')
  })

  it('fails closed for DepartmentAdmin without a department scope', async () => {
    permissionMock.allowedActions.clear()
    permissionMock.allowedActions.add('department.manage')
    const wrapper = mountView({
      id: '3',
      name: 'DeptAdmin',
      role: RoleEnum.DEPT_ADMIN,
      departmentId: '',
      groupId: '',
      dataScope: DataScopeEnum.DEPARTMENT,
      permissions: [],
    })
    await flushPromises()

    expect(wrapper.text()).not.toContain('全部组织')
    expect(wrapper.text()).toContain('当前账号缺少部门管理范围')
    expect(usersApi.list).not.toHaveBeenCalled()
  })

  it('fails closed when DepartmentAdmin scope is no longer an active org option', async () => {
    permissionMock.allowedActions.clear()
    permissionMock.allowedActions.add('department.manage')
    vi.mocked(fetchOrgOwnershipOptions).mockResolvedValueOnce({
      departmentOptions: [{ value: '定制美工部', label: '定制美工部' }],
      teamOptions: [{ value: '默认组', label: '默认组', department: '定制美工部' }],
      departmentRecords: [{ id: '8', name: '定制美工部', enabled: true }],
      teamRecords: [{ id: '18', name: '默认组', departmentId: '8', departmentName: '定制美工部', enabled: true }],
    })

    const wrapper = mountView({
      id: '4',
      name: 'Legacy DeptAdmin',
      role: RoleEnum.DEPT_ADMIN,
      departmentId: '设计部',
      groupId: '定制美工组',
      dataScope: DataScopeEnum.DEPARTMENT,
      permissions: [],
    })
    await flushPromises()

    expect(wrapper.text()).toContain('当前账号部门范围已停用或不存在')
    expect(usersApi.list).not.toHaveBeenCalled()
  })

  it('bulk purges disabled orgs even when member counts remain', async () => {
    vi.mocked(fetchOrgOwnershipOptions).mockResolvedValue({
      departmentOptions: [{ value: 'Design', label: '设计部' }],
      teamOptions: [{ value: 'Design-A', label: '设计一组', department: 'Design' }],
      departmentRecords: [
        { id: '1', name: 'Design', enabled: true, memberCount: 3 },
        { id: '9', name: 'OldDept', enabled: false, memberCount: 4 },
      ],
      teamRecords: [
        { id: '2', name: 'Design-A', departmentId: '1', departmentName: 'Design', enabled: true, memberCount: 3 },
        { id: '7', name: 'OldTeam', departmentId: '1', departmentName: 'Design', enabled: false, memberCount: 2 },
        { id: '8', name: 'OldDeptTeam', departmentId: '9', departmentName: 'OldDept', enabled: false, memberCount: 1 },
      ],
    } as never)
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openPurgeAllEmpty: () => void
      submitOrgAction: () => Promise<void>
      removableDisabledOrgCount: number
    }

    // OldDept 与 OldTeam 各计 1 项;OldDeptTeam 随部门级联删除,不单独计数。
    // memberCount 不再阻断删除,后端会把相关账号归入未分配池。
    expect(vm.removableDisabledOrgCount).toBe(2)
    expect(wrapper.text()).toContain('一键清理停用组织（2 项）')

    vm.openPurgeAllEmpty()
    await vm.submitOrgAction()
    await flushPromises()

    expect(deleteOrgTeam).toHaveBeenCalledWith('7')
    expect(deleteOrgTeam).not.toHaveBeenCalledWith('8')
    expect(deleteOrgDepartment).toHaveBeenCalledWith('9')
  })

  it('applies keyword only after explicit search and clears it with the input', async () => {
    const wrapper = mountView()
    await flushPromises()
    vi.mocked(usersApi.list).mockClear()
    const vm = wrapper.vm as unknown as {
      keyword: string
      departmentFilter: string
      onSearch: () => void
    }

    // 输入到一半、未点查询:切换组织不应把半截关键词带进请求。
    vm.keyword = '张三'
    vm.departmentFilter = 'Design'
    await flushPromises()
    for (const [params] of vi.mocked(usersApi.list).mock.calls) {
      expect(params).not.toMatchObject({ keyword: '张三' })
    }

    vi.mocked(usersApi.list).mockClear()
    vm.onSearch()
    await flushPromises()
    expect(usersApi.list).toHaveBeenCalledWith(
      expect.objectContaining({ keyword: '张三' }),
      expect.anything(),
    )

    // 清空输入框即取消关键词过滤,无需再点一次查询。
    vi.mocked(usersApi.list).mockClear()
    vm.keyword = ''
    await flushPromises()
    expect(usersApi.list).toHaveBeenCalled()
    for (const [params] of vi.mocked(usersApi.list).mock.calls) {
      expect(params).not.toMatchObject({ keyword: '张三' })
    }
  })

  it('refreshes org member counts after membership changes', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openDetail: (row: unknown) => Promise<void>
      membershipForm: { department: string; team: string }
      submitMembership: () => Promise<void>
    }

    await vm.openDetail({ id: '2', username: 'target', roles: [] })
    await flushPromises()
    vi.mocked(fetchOrgOwnershipOptions).mockClear()

    vm.membershipForm.department = 'Design'
    vm.membershipForm.team = 'Design-B'
    await vm.submitMembership()
    await flushPromises()

    expect(fetchOrgOwnershipOptions).toHaveBeenCalled()
  })

  it('drops stale legacy department and team filters before listing users', async () => {
    const wrapper = mountView()
    await flushPromises()
    vi.mocked(usersApi.list).mockClear()

    const vm = wrapper.vm as unknown as {
      departmentFilter: string
      teamFilter: string
    }
    vm.departmentFilter = '设计部'
    vm.teamFilter = '定制美工组'
    await flushPromises()

    expect(vm.departmentFilter).toBe('')
    expect(vm.teamFilter).toBe('')
    for (const [params] of vi.mocked(usersApi.list).mock.calls) {
      expect(params).not.toMatchObject({ department: '设计部' })
      expect(params).not.toMatchObject({ team: '定制美工组' })
    }
  })
})
