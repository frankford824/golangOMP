// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { NButton, NCheckbox, NDatePicker, NSelect } from 'naive-ui'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TaskFilterPanel, { type TaskListFilters } from './TaskFilterPanel.vue'

const optionsState = vi.hoisted(() => ({
  loadError: '',
  loading: false,
  departments: [] as Array<{ label: string; value: string }>,
  loadFilterOptions: vi.fn(),
}))
vi.mock('@/composables/useTaskFilterOptions', async () => {
  const { computed, ref } = await import('vue')
  return {
    useTaskFilterOptions: () => ({
      creatorOptions: ref([]),
      assigneeOptions: ref([]),
      ownerDepartmentOptions: computed(() => optionsState.departments),
      ownerTeamOptions: ref([]),
      loadFilterOptions: optionsState.loadFilterOptions,
      loadError: computed(() => optionsState.loadError),
      loading: computed(() => optionsState.loading),
    }),
  }
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
  beforeEach(() => {
    optionsState.loadError = ''
    optionsState.loading = false
    optionsState.departments = []
    optionsState.loadFilterOptions.mockClear()
  })

  it('tells the user why the organization and people dropdowns are empty', async () => {
    const empty = mount(TaskFilterPanel, { props: { filters: filters(), keyword: '' } })
    expect(empty.get('.options-status').text()).toContain('没有可选的部门、团队或人员')
    expect(empty.find('.options-status.is-error').exists()).toBe(false)
    empty.unmount()

    optionsState.loadError = '加载任务筛选候选失败，请稍后重试'
    const failed = mount(TaskFilterPanel, { props: { filters: filters(), keyword: '' } })
    const status = failed.get('.options-status.is-error')
    expect(status.text()).toContain('加载任务筛选候选失败')
    await status.get('button').trigger('click')
    expect(optionsState.loadFilterOptions).toHaveBeenCalledTimes(1)
    failed.unmount()

    optionsState.loadError = ''
    optionsState.departments = [{ label: '设计部', value: '1' }]
    const loaded = mount(TaskFilterPanel, { props: { filters: filters(), keyword: '' } })
    expect(loaded.find('.options-status').exists()).toBe(false)
    loaded.unmount()
  })

  it('keeps edits local and emits one committed snapshot only when applied', async () => {
    const original = filters()
    const wrapper = mount(TaskFilterPanel, { props: { filters: original, keyword: '' } })

    await wrapper.get('.keyword-section input').setValue(' RW-100 ')
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

  it('groups business filters in a drawer-friendly layout and exposes an explicit close action', async () => {
    const wrapper = mount(TaskFilterPanel, { props: { filters: filters(), keyword: '' } })
    expect(wrapper.findAll('.filter-section h3').map((item) => item.text())).toEqual(['任务属性', '组织归属', '相关人员', '创建时间', '精确查找'])
    await wrapper.get('.filter-heading button').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('uses named status chips for accessible multi-status filtering', async () => {
    const wrapper = mount(TaskFilterPanel, { props: { filters: filters(), keyword: '' } })
    const pendingAudit = wrapper.findAll('.status-option').find((item) => item.text() === '待审核')
    expect(pendingAudit?.attributes('aria-pressed')).toBe('false')

    await pendingAudit!.trigger('click')
    expect(pendingAudit?.attributes('aria-pressed')).toBe('true')
    await buttonByText(wrapper, '应用筛选').trigger('click')

    const payload = wrapper.emitted('apply')?.[0] as [TaskListFilters, string]
    expect(payload[0].status).toEqual(['PendingAudit'])
  })

  it('renders dropdowns inside the drawer stacking context and exposes the three priority choices', () => {
    const wrapper = mount(TaskFilterPanel, { props: { filters: filters(), keyword: '' } })
    expect(wrapper.findAllComponents(NSelect).every((select) => select.props('to') === false)).toBe(true)
    expect(wrapper.findAllComponents(NDatePicker).every((picker) => picker.props('to') === false)).toBe(true)
    const priority = wrapper.findAllComponents(NSelect).find((select) => select.attributes('aria-label') === '优先级')
    expect(priority?.props('options')).toEqual([
      { label: '普通', value: 'normal' },
      { label: '加急', value: 'high' },
      { label: '出单画图', value: 'drawing' },
    ])
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
