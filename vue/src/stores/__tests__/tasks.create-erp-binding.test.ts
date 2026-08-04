import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const createMock = vi.fn()
const getDetailMock = vi.fn()
const getByIdMock = vi.fn()

vi.mock('@/services/api/tasksApi', () => ({
  tasksApi: {
    create: createMock,
    getDetail: getDetailMock,
    getById: getByIdMock,
  },
}))

const createdTask = {
  id: '3001',
  task_no: 'RW-3001',
  task_type: 'original_product_development',
  task_status: 'PendingAssign',
  product_name_snapshot: '数字编码 ERP 商品',
  sku_code: '12324546567',
  created_at: '2026-07-29T00:00:00.000Z',
  updated_at: '2026-07-29T00:00:00.000Z',
}

describe('useTasksStore existing ERP product creation', () => {
  let useTasksStore: (typeof import('@/stores/tasks'))['useTasksStore']

  beforeAll(async () => {
    ;({ useTasksStore } = await import('@/stores/tasks'))
  }, 15000)

  beforeEach(() => {
    setActivePinia(createPinia())
    createMock.mockReset()
    getDetailMock.mockReset()
    getByIdMock.mockReset()
    createMock.mockResolvedValue({ data: { data: createdTask } })
    getDetailMock.mockResolvedValue({ data: { data: { task: createdTask } } })
    getByIdMock.mockResolvedValue({ data: { data: createdTask } })
  })

  it('sends a numeric ERP code as a deferred ERP snapshot instead of local product_id', async () => {
    const store = useTasksStore()
    const erpSnapshot = {
      product_id: '12324546567',
      sku_code: '12324546567',
      product_name: '数字编码 ERP 商品',
    }

    await store.addTask({
      taskType: 'ORIGINAL_PRODUCT_DEV',
      productId: null,
      sku: '12324546567',
      productName: '数字编码 ERP 商品',
      erpProductSnapshot: erpSnapshot,
      designRequirement: '更换图案',
      dueAt: '2026-07-31T12:00:00.000Z',
    } as Parameters<typeof store.addTask>[0] & { erpProductSnapshot: typeof erpSnapshot })

    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      task_type: 'original_product_development',
      product_id: null,
      sku_code: '12324546567',
      defer_local_product_binding: true,
      product_selection: {
        defer_local_product_binding: true,
        erp_product: erpSnapshot,
      },
    }), undefined, undefined)
  })
})
