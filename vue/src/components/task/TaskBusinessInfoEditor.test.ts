// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ patchBusinessInfo: vi.fn() }))
vi.mock('@/services/api/tasksApi', () => ({ tasksApi: mocks }))

import TaskBusinessInfoEditor from './TaskBusinessInfoEditor.vue'

const task = {
  product_name_snapshot: '生日挂布',
  priority: 'normal',
  deadline_at: '2026-08-08T10:00:00Z',
  design_requirement: '出单画图',
  task_type: 'new_product_development',
}

describe('TaskBusinessInfoEditor priority and deadline', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.patchBusinessInfo.mockResolvedValue({})
  })

  it('only exposes canonical priority values and submits deadline as RFC3339', async () => {
    const wrapper = mount(TaskBusinessInfoEditor, { props: { taskId: 3860, task } })
    const priority = wrapper.get('select')
    expect(priority.findAll('option').map((option) => option.attributes('value'))).toEqual(['normal', 'high', 'drawing'])

    await priority.setValue('high')
    await wrapper.get('input[type="datetime-local"]').setValue('2026-08-09T13:00')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(mocks.patchBusinessInfo).toHaveBeenCalledWith('3860', expect.objectContaining({
      priority: 'high',
      deadline_at: new Date('2026-08-09T13:00').toISOString(),
    }))
  })

  it('maps legacy critical values to the canonical high priority', () => {
    const wrapper = mount(TaskBusinessInfoEditor, { props: { taskId: 3860, task: { ...task, priority: 'critical' } } })
    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('high')
  })
})
