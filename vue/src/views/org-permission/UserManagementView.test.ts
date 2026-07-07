// @vitest-environment jsdom
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import UserManagementView from './UserManagementView.vue'
import { usersApi } from '@/services/api/usersApi'
import { usePermissionsStore } from '@/stores/permissions'
import { DataScopeEnum, RoleEnum, type PermissionUser } from '@/types'

const permissionMock = vi.hoisted(() => ({
  allowedActions: new Set<string>(),
  currentUser: {
    value: { id: '1' },
  },
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
    replaceRoles: vi.fn(),
    create: vi.fn(),
    deactivate: vi.fn(),
    activate: vi.fn(),
    resetPassword: vi.fn(),
  },
}))

vi.mock('@/services/api/orgApi', () => ({
  fetchOrgOwnershipOptions: vi.fn(async () => ({
    departmentOptions: [{ value: 'Design', label: '设计部' }],
    teamOptions: [
      { value: 'Design-A', label: '设计一组', department: 'Design' },
      { value: 'legacy-floating', label: '旧无部门组' },
    ],
  })),
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
          roles: ['Audit_B', 'Outsource', 'SuperAdmin'],
          status: 'active',
        },
      },
    } as never)
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('renders only actor-assignable roles as checkboxes and preserves locked roles on save', async () => {
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
    expect(checkboxes.map((item) => item.value)).toEqual(['Member', 'AssetSubmitter'])
    const modalText = modal?.textContent ?? ''
    expect(modalText).toContain('历史/不可编辑角色')
    expect(modalText).toContain('历史兼容：常规审核旧编码')
    expect(modalText).toContain('外协')
    expect(modalText).toContain('超级管理员')
    expect(modalText).toContain('素材工作台角色')
    expect(modalText).toContain('素材工作台-交付人员')
    expect(modalText).not.toContain('Super Admin')
    expect(modalText).not.toContain('Outsource')
    expect(modalText).not.toContain('普通审核B')

    checkboxes[0]?.click()
    await flushPromises()
    const saveButton = Array.from(document.body.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === '保存角色',
    )
    saveButton?.click()
    await flushPromises()

    expect(usersApi.replaceRoles).toHaveBeenCalledWith('2', {
      roles: ['Audit_B', 'Outsource', 'SuperAdmin'],
    })
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
})
