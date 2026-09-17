// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ dashboard: vi.fn(), push: vi.fn() }))
vi.mock('@/services/api/tasksApi', () => ({ tasksApi: { designDepartmentDashboard: mocks.dashboard } }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: mocks.push }) }))

import DesignDepartmentDashboardView from './DesignDepartmentDashboardView.vue'

const payload = {
  generated_at: '2026-09-17T08:00:00Z', time_zone: 'Asia/Shanghai', department_id: 14, department_name: '视觉研创部',
  period_start: '2026-08-18T16:00:00Z', period_end: '2026-09-18T16:00:00Z', date_basis: 'created', granularity: 'day',
  summary: { task_count: 10, completed_task_count: 8, active_task_count: 2, overdue_task_count: 1, due_soon_task_count: 1, design_submission_count: 10, source_file_count: 12, final_file_count: 18, design_file_count: 30, sku_count: 15, resource_unit_count: 15, member_count: 2, contributing_member_count: 2, completion_rate: 80, first_pass_rate: 75, on_time_rate: 87.5, average_turnaround_hours: 8, median_turnaround_hours: 6, p90_turnaround_hours: 14, average_files_per_task: 3, turnaround_sample_count: 8, deadline_sample_count: 8, rejected_task_count: 2, audit_reject_event_count: 2 },
  trend: [{ period: '2026-09-17', task_count: 10, completed_count: 8, design_file_count: 30, sku_count: 15 }],
  people: [{ user_id: 228, display_name: '王亚琳', username: '王亚琳', team_name: '默认组', status: 'active', task_count: 6, completed_task_count: 5, active_task_count: 1, overdue_task_count: 1, design_file_count: 20, source_file_count: 8, final_file_count: 12, sku_count: 9, design_submission_count: 6, rejected_task_count: 1, audit_reject_event_count: 1, completion_rate: 83.3, first_pass_rate: 80, on_time_rate: 80, average_turnaround_hours: 7, workload_share: 60, task_types: [] }],
  task_types: [{ key: 'new_product_development', label: '新品开发', task_count: 10, design_file_count: 30, sku_count: 15, share: 100 }],
  statuses: [{ key: 'Completed', label: '已完成', task_count: 8, design_file_count: 24, sku_count: 12, share: 80 }],
  priorities: [{ key: 'normal', label: '普通', task_count: 10, design_file_count: 30, sku_count: 15, share: 100 }], business_lanes: [],
  file_types: [{ key: 'source', label: '设计源文件', task_count: 12, design_file_count: 0, sku_count: 0, share: 0 }],
  recent_tasks: [{ task_id: 1, task_no: 'RW-1', product_name: '测试产品', designer_id: 228, designer_name: '王亚琳', task_type: 'new_product_development', task_status: 'Completed', priority: 'normal', created_at: '2026-09-17T00:00:00Z', source_file_count: 1, final_file_count: 2, sku_count: 1, audit_reject_count: 0, overdue: false }],
  options: { members: [{ user_id: 228, display_name: '王亚琳', username: '王亚琳', team_name: '默认组', status: 'active' }], task_types: ['new_product_development'], statuses: ['Completed'], priorities: ['normal'], teams: [], date_bases: ['created'], granularities: ['day'] },
  definitions: { task_count: '任务去重' },
}

describe('DesignDepartmentDashboardView', () => {
  beforeEach(() => { vi.clearAllMocks(); mocks.dashboard.mockResolvedValue({ data: { data: payload } }) })
  it('renders department KPIs, people and task detail from the authoritative endpoint', async () => {
    const wrapper = mount(DesignDepartmentDashboardView, { global: { stubs: { DesignDepartmentTrendChart: true, DesignDepartmentBreakdownChart: true } } })
    await flushPromises()
    expect(mocks.dashboard).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('视觉研创部数据看板')
    expect(wrapper.text()).toContain('设计文件量')
    expect(wrapper.text()).toContain('30')
    expect(wrapper.text()).toContain('王亚琳')
    expect(wrapper.text()).toContain('RW-1')
    expect(wrapper.text()).toContain('一次通过率')
    wrapper.unmount()
  })

  it('applies person and period filters to the same endpoint', async () => {
    const wrapper = mount(DesignDepartmentDashboardView, { global: { stubs: { DesignDepartmentTrendChart: true, DesignDepartmentBreakdownChart: true } } })
    await flushPromises()
    await wrapper.get('.person-link').trigger('click')
    await flushPromises()
    expect(mocks.dashboard).toHaveBeenLastCalledWith(expect.objectContaining({ designer_ids: '228' }), expect.any(AbortSignal))
    await wrapper.findAll('.preset-row button')[0].trigger('click')
    await flushPromises()
    const args = mocks.dashboard.mock.calls[mocks.dashboard.mock.calls.length - 1]?.[0]
    expect(args.start_date).toBeTruthy()
    expect(args.end_date).toBeTruthy()
    wrapper.unmount()
  })
})
