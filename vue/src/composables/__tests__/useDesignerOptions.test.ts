import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
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
  let usePermissionsStore: (typeof import('@/stores/permissions'))['usePermissionsStore']
  let useDesignerOptions: (typeof import('@/composables/useDesignerOptions'))['useDesignerOptions']

  beforeAll(async () => {
    ;[{ usePermissionsStore }, { useDesignerOptions }] = await Promise.all([
      import('@/stores/permissions'),
      import('@/composables/useDesignerOptions'),
    ])
  }, 15000)

  beforeEach(() => {
    setActivePinia(createPinia())
    getDesignersMock.mockClear()
  })

  function signInOpsUser(): void {
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
  }

  it('requests customization lane when workflowLane is customization', async () => {
    signInOpsUser()
    const lane = ref<'customization' | undefined>('customization')
    const { loadDesigners } = useDesignerOptions({
      autoLoad: false,
      workflowLane: lane,
    })

    await loadDesigners()

    expect(getDesignersMock).toHaveBeenCalledWith({ workflowLane: 'customization' })
  })

  it('omits workflow_lane for normal designer pool', async () => {
    signInOpsUser()
    const { loadDesigners } = useDesignerOptions({ autoLoad: false })

    await loadDesigners()

    expect(getDesignersMock).toHaveBeenCalledWith(undefined)
  })
})
