import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const { getDetailMock, getByIdMock } = vi.hoisted(() => ({
  getDetailMock: vi.fn(),
  getByIdMock: vi.fn(),
}))

vi.mock('@/services/api/tasksApi', () => ({
  tasksApi: {
    getDetail: getDetailMock,
    getById: getByIdMock,
  },
}))

import { useTasksStore } from '@/stores/tasks'
import { canUploadDesignDelivery } from '@/domain/task-actions'

function rejectedTaskEnvelope(designSubStatusLocation: 'flat' | 'workflow') {
  return {
    task: {
      id: 2759,
      task_no: 'RW-20260721-A-002756',
      task_type: 'new_product_development',
      task_status: 'RejectedByAuditA',
      designer_id: 228,
      current_handler_id: 228,
      created_at: '2026-07-21T03:31:48.000Z',
      updated_at: '2026-07-21T08:36:11.000Z',
    },
    ...(designSubStatusLocation === 'flat'
      ? { design_sub_status: 'rework_required' }
      : {
          workflow: {
            sub_status: {
              design: { code: 'rework_required' },
            },
          },
        }),
  }
}

describe('useTasksStore rejected design status compatibility', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getDetailMock.mockReset()
    getByIdMock.mockReset()
    getByIdMock.mockResolvedValue({ data: { data: {} } })
  })

  it.each(['flat', 'workflow'] as const)(
    'maps backend %s rework_required to the frontend REJECTED state',
    async (designSubStatusLocation) => {
      getDetailMock.mockResolvedValue({
        data: { data: rejectedTaskEnvelope(designSubStatusLocation) },
      })

      const store = useTasksStore()
      await store.loadTaskById('2759')

      expect(store.list).toHaveLength(1)
      expect(store.getById('2759')).toMatchObject({
        id: '2759',
        status: 'RejectedByAuditA',
        designSubStatus: 'REJECTED',
      })
      expect(canUploadDesignDelivery(store.getById('2759')!)).toBe(true)
    },
  )
})
