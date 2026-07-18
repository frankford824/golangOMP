// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { NButton, NCheckbox, NInput } from 'naive-ui'
import { describe, expect, it, vi } from 'vitest'
import TaskFilterPanel, { type TaskListFilters } from './TaskFilterPanel.vue'

vi.mock('@/composables/useTaskFilterOptions', async () => {
  const { ref } = await import('vue')
  return { useTaskFilterOptions: () => ({ creatorOptions: ref([]), assigneeOptions: ref([]) }) }
})
vi.mock('@/composables/useOrgOwnershipFilterOptions', async () => {
  const { ref } = await import('vue')
  return { useOrgOwnershipFilterOptions: () => ({ departmentOptions: ref([]), teamOptions: ref([]) }) }
})
const filters = (): TaskListFilters => ({
  status: [], taskCategory: '', taskType: '', priority: '', creatorId: '', assigneeId: '',
  dateFrom: '', dateTo: '', overdueOnly: false, ownerDepartment: '', ownerOrgTeam: '',
})

function buttonByText(wrapper: ReturnType<typeof mount>, text: string) {
  const found = wrapper.findAllComponents(NButton).find((item) => item.text().includes(text))
  if (!found) throw new Error(`missing button ${text}`)
  return found
}

describe('TaskFilterPanel', () => {
  it('keeps edits local and emits one committed snapshot only when applied', async () => {
    const original = filters()
    const wrapper = mount(TaskFilterPanel, { props: { filters: original, keyword: '' } })

    wrapper.findComponent(NInput).vm.$emit('update:value', ' RW-100 ')
    wrapper.findComponent(NCheckbox).vm.$emit('update:checked', true)
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('apply')).toBeUndefined()
    expect(original.overdueOnly).toBe(false)

    await buttonByText(wrapper, '应用筛选').trigger('click')
    const payload = wrapper.emitted('apply')?.[0] as [TaskListFilters, string]
    expect(payload[0].overdueOnly).toBe(true)
    expect(payload[1]).toBe('RW-100')
    expect(wrapper.emitted('apply')).toHaveLength(1)
  })

  it('resets the local draft without mutating parent props', async () => {
    const original = { ...filters(), status: ['PendingAudit'] as TaskListFilters['status'], ownerDepartment: '10' }
    const wrapper = mount(TaskFilterPanel, { props: { filters: original, keyword: 'SKU-A' } })

    await buttonByText(wrapper, '重置').trigger('click')
    const payload = wrapper.emitted('reset')?.[0] as [TaskListFilters, string]
    expect(payload[0]).toEqual(filters())
    expect(payload[1]).toBe('')
    expect(original.status).toEqual(['PendingAudit'])
    expect(original.ownerDepartment).toBe('10')
  })
})
