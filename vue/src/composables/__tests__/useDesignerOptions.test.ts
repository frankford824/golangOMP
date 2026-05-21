import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { DataScopeEnum, RoleEnum } from '@/types'

const getDesignersMock = vi.fn(async () => ({
  data: {
    data: [{ id: 301, display_name: '美工甲' }],
  },
}))

vi.mock('@/services/api/usersApi', () => ({
  usersApi: {
    getDesigners: getDesignersMock,
  },
}))

describe('useDesignerOptions workflowLane', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getDesignersMock.mockClear()
  })

  it('requests customization lane when workflowLane is customization', async () => {
    const { usePermissionsStore } = await import('@/stores/permissions')
    const permissions = usePermissionsStore()
    permissions.setCurrentUser({
      id: '9',
      name: 'Ops User',
      role: RoleEnum.OPS,
      departmentId: '',
      groupId: '',
      dataScope: DataScopeEnum.GLOBAL,
      permissions: [],
    })
    permissions.roles = ['Ops']

    const { useDesignerOptions } = await import('@/composables/useDesignerOptions')
    const lane = ref<'customization' | undefined>('customization')
    const { loadDesigners } = useDesignerOptions({
      autoLoad: false,
      workflowLane: lane,
    })

    await loadDesigners()

    expect(getDesignersMock).toHaveBeenCalledWith({ workflowLane: 'customization' })
  })

  it('omits workflow_lane for normal designer pool', async () => {
    const { usePermissionsStore } = await import('@/stores/permissions')
    const permissions = usePermissionsStore()
    permissions.setCurrentUser({
      id: '9',
      name: 'Ops User',
      role: RoleEnum.OPS,
      departmentId: '',
      groupId: '',
      dataScope: DataScopeEnum.GLOBAL,
      permissions: [],
    })
    permissions.roles = ['Ops']

    const { useDesignerOptions } = await import('@/composables/useDesignerOptions')
    const { loadDesigners } = useDesignerOptions({ autoLoad: false })

    await loadDesigners()

    expect(getDesignersMock).toHaveBeenCalledWith(undefined)
  })
})
