import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const assignMock = vi.fn(async () => ({}))
const reassignModuleMock = vi.fn(async () => ({}))
const listMock = vi.fn(async () => ({ data: { data: [], pagination: { total: 0 } } }))
const getDetailMock = vi.fn(async () => ({ data: { data: { task: {} } } }))
const getByIdMock = vi.fn(async () => ({ data: { data: {} } }))

vi.mock('@/services/api/tasksApi', () => ({
  tasksApi: {
    assign: assignMock,
    reassignModule: reassignModuleMock,
    list: listMock,
    getDetail: getDetailMock,
    getById: getByIdMock,
  },
}))

function mockListWithTask(task: Record<string, unknown>) {
  listMock.mockResolvedValueOnce({
    data: {
      data: [task],
      pagination: { total: 1, page: 1, page_size: 500 },
    },
  })
}

function baseRawTask(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: '101',
    task_no: 'TASK-101',
    task_type: 'new_product_development',
    task_status: 'PendingAssign',
    requester_id: '1',
    requester_name: 'req',
    creator_id: '1',
    creator_name: 'req',
    created_at: '2026-05-15T00:00:00.000Z',
    updated_at: '2026-05-15T00:00:00.000Z',
    ...overrides,
  }
}

describe('useTasksStore assign routing', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    assignMock.mockClear()
    reassignModuleMock.mockClear()
    listMock.mockClear()
    getDetailMock.mockClear()
    getByIdMock.mockClear()
    getDetailMock.mockResolvedValue({ data: { data: { task: {} } } })
    getByIdMock.mockResolvedValue({ data: { data: {} } })
  })

  it('retouch_task 首次指派时调用 tasksApi.assign', async () => {
    const { useTasksStore } = await import('@/stores/tasks')
    const store = useTasksStore()
    mockListWithTask(
      baseRawTask({
        id: '201',
        task_type: 'retouch_task',
        task_status: 'PendingAssign',
      }),
    )
    await store.loadTasks()

    await store.assignTask('201', { assigneeId: '9', assigneeName: 'Alice' })

    expect(assignMock).toHaveBeenCalledWith(
      '201',
      expect.objectContaining({ designer_id: 9, designer_name: 'Alice' }),
    )
    expect(reassignModuleMock).not.toHaveBeenCalled()
  })

  it('retouch_task 重新指派时调用 tasksApi.assign', async () => {
    const { useTasksStore } = await import('@/stores/tasks')
    const store = useTasksStore()
    mockListWithTask(
      baseRawTask({
        id: '202',
        task_type: 'retouch_task',
        task_status: 'InProgress',
        designer_id: '7',
        designer_name: 'Bob',
      }),
    )
    await store.loadTasks()

    await store.reassignDesignerTask('202', { assigneeId: '10', assigneeName: 'Carol' })

    expect(assignMock).toHaveBeenCalledWith(
      '202',
      expect.objectContaining({ designer_id: 10, designer_name: 'Carol' }),
    )
    expect(reassignModuleMock).not.toHaveBeenCalled()
  })

  it('retouch_task 清空指派时调用 tasksApi.assign', async () => {
    const { useTasksStore } = await import('@/stores/tasks')
    const store = useTasksStore()
    mockListWithTask(
      baseRawTask({
        id: '203',
        task_type: 'retouch_task',
        task_status: 'InProgress',
        designer_id: '7',
        designer_name: 'Bob',
      }),
    )
    await store.loadTasks()

    await store.clearDesignerAssignee('203', '回退待指派')

    expect(assignMock).toHaveBeenCalledWith(
      '203',
      expect.objectContaining({ designer_id: null, remark: '回退待指派' }),
    )
    expect(reassignModuleMock).not.toHaveBeenCalled()
  })

  it('retouch_task 指派流程不再调用 tasksApi.reassignModule', async () => {
    const { useTasksStore } = await import('@/stores/tasks')
    const store = useTasksStore()
    mockListWithTask(
      baseRawTask({
        id: '204',
        task_type: 'retouch_task',
        task_status: 'PendingAssign',
      }),
    )
    await store.loadTasks()

    await store.assignTask('204', { assigneeId: '11', assigneeName: 'David' })

    expect(reassignModuleMock).not.toHaveBeenCalled()
  })

  it('普通 design task 指派逻辑保持为 tasksApi.assign', async () => {
    const { useTasksStore } = await import('@/stores/tasks')
    const store = useTasksStore()
    mockListWithTask(
      baseRawTask({
        id: '205',
        task_type: 'new_product_development',
        task_status: 'PendingAssign',
      }),
    )
    await store.loadTasks()

    await store.assignTask('205', { assigneeId: '12', assigneeName: 'Eve' })

    expect(assignMock).toHaveBeenCalledWith(
      '205',
      expect.objectContaining({ designer_id: 12, designer_name: 'Eve' }),
    )
    expect(reassignModuleMock).not.toHaveBeenCalled()
  })
})
