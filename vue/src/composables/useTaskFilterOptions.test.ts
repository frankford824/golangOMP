// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, nextTick, ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { tasksApi } from '@/services/api/tasksApi'
import { useTaskFilterOptions } from './useTaskFilterOptions'

vi.mock('@/services/api/tasksApi', () => ({
  tasksApi: {
    filterOptions: vi.fn(),
  },
}))

describe('useTaskFilterOptions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('hydrates task-scoped actor and organization options from one endpoint', async () => {
    vi.mocked(tasksApi.filterOptions).mockResolvedValue({
      data: {
        data: {
          creators: [{ id: 7, name: '创建人甲' }],
          designers: [{ id: 8, name: '设计师乙' }],
          owner_departments: [{ id: 31, name: '设计研发部' }],
          owner_teams: [
            { id: 41, name: '设计一组', department_id: 31, department_name: '设计研发部' },
            { id: 42, name: '定制一组', department_id: 32, department_name: '定制美工部' },
          ],
        },
      },
    } as never)
    const selectedDepartment = ref('')
    let result!: ReturnType<typeof useTaskFilterOptions>
    mount(defineComponent({
      setup() {
        result = useTaskFilterOptions(true, '全部', () => selectedDepartment.value)
        return () => null
      },
    }))

    await flushPromises()
    expect(result.creatorOptions.value).toEqual([
      { value: '', label: '全部' },
      { value: '7', label: '创建人甲' },
    ])
    expect(result.assigneeOptions.value).toEqual([
      { value: '', label: '全部' },
      { value: '8', label: '设计师乙' },
    ])
    expect(result.ownerDepartmentOptions.value).toEqual([
      { value: '设计研发部', label: '设计研发部' },
    ])
    expect(result.ownerTeamOptions.value).toEqual([
      { value: '设计一组', label: '设计一组' },
      { value: '定制一组', label: '定制一组' },
    ])

    selectedDepartment.value = '设计研发部'
    await nextTick()
    expect(result.ownerTeamOptions.value).toEqual([
      { value: '设计一组', label: '设计一组' },
    ])
  })
})
