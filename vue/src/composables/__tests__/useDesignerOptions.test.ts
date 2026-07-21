import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { DataScopeEnum, PermissionEnum, RoleEnum } from '@/types'

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

  function signInUserWithActions(actions: string[]): void {
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
    permissions.actions = actions
  }

  function signInOpsUser(): void {
    signInUserWithActions([PermissionEnum.TASK_CREATE])
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

  it('allows audit handover capability to request audit candidates', async () => {
    signInUserWithActions([PermissionEnum.TASK_AUDIT_HANDOVER])
    const lane = ref<'audit' | undefined>('audit')
    const { loadDesigners } = useDesignerOptions({
      autoLoad: false,
      workflowLane: lane,
    })

    await loadDesigners()

    expect(getDesignersMock).toHaveBeenCalledWith({ workflowLane: 'audit' })
  })

  it('does not request candidates for legacy display roles without capability', async () => {
    signInUserWithActions([])
    const lane = ref<'audit' | undefined>('audit')
    const { loadDesigners } = useDesignerOptions({
      autoLoad: false,
      workflowLane: lane,
    })

    await loadDesigners()

    expect(getDesignersMock).not.toHaveBeenCalled()
  })
})
