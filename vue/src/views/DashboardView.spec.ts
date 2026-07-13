// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { TaskOperationalOverview } from '@/types/dashboard'

const { operationalOverview, push } = vi.hoisted(() => ({
  operationalOverview: vi.fn(),
  push: vi.fn(),
}))

vi.mock('@/services/api/tasksApi', () => ({
  tasksApi: { operationalOverview },
}))
vi.mock('@/composables/usePermission', () => ({
  usePermission: () => ({ can: () => true }),
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push }),
}))

import DashboardView from './DashboardView.vue'

function overview(designPending: number, generatedAt: string): TaskOperationalOverview {
  return {
    generated_at: generatedAt,
    time_zone: 'Asia/Shanghai',
    period_start: '2026-07-12T16:00:00Z',
    period_end: '2026-07-13T16:00:00Z',
    health_status: 'ok',
    counts: {
      total_tasks: 1817,
      active_tasks: 978,
      design_pending: designPending,
      pending_audit: 457,
      handover: 0,
      customization_in_progress: 54,
      pending_warehouse_receive: 136,
      overdue: 938,
      due_today: 29,
      today_created: 5,
      today_completed: 5,
    },
    kpis: {
      week_created: 5,
      week_created_completed: 1,
      week_completion_rate: 20,
      week_audit_decisions: 2,
      week_audit_rejected: 1,
      week_reject_rate: 50,
      week_completed: 5,
      average_processing_hours: 8.25,
      average_processing_sample_count: 5,
      exact_completion_sample_count: 4,
      fallback_completion_sample_count: 1,
      completion_event_coverage_rate: 80,
    },
    trend: Array.from({ length: 7 }, (_, index) => ({
      date: `2026-07-${String(index + 7).padStart(2, '0')}`,
      created: index,
      completed: index + 1,
      due: index + 2,
    })),
    status_distribution: [
      { key: 'design_ops', name: '设计/运营待推进', count: designPending },
      { key: 'audit', name: '待审核', count: 457 },
      { key: 'customization', name: '定制协同', count: 54 },
      { key: 'warehouse', name: '待仓库', count: 166 },
      { key: 'completed', name: '已完成/终止', count: 839 },
    ],
    recent_tasks: [
      {
        task_id: 2317,
        task_no: 'RW-20260713-A-002314',
        product_name: '测试产品',
        owner_name: '设计甲',
        task_status: 'InProgress',
        deadline_at: '2026-07-13T10:00:00Z',
      },
    ],
    recent_events: [],
  }
}

describe('DashboardView authoritative refresh', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    operationalOverview.mockReset()
    operationalOverview
      .mockResolvedValueOnce({ data: { data: overview(303, '2026-07-13T01:17:22Z') } })
      .mockResolvedValueOnce({ data: { data: overview(304, '2026-07-13T01:18:22Z') } })
      .mockResolvedValue({ data: { data: overview(305, '2026-07-13T01:19:22Z') } })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads full overview, supports manual refresh, and refreshes every 30 seconds', async () => {
    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          DashboardKpiCard: { template: '<div class="kpi-stub">{{ title }} {{ value }}</div>', props: ['title', 'value'] },
          DashboardTrendChart: true,
          DashboardTaskSnapshotTable: true,
          RecentEventStream: true,
          RiskListCard: true,
          StatusSkeleton: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.get('.board-header-aside strong').text()).toBe('303')
    expect(wrapper.text()).toContain('全局进行中任务 978')
    expect(wrapper.text()).toContain('本周完成率 20.0%')

    const refreshButton = wrapper.findAll('button').find((button) => button.text().includes('立即刷新'))
    expect(refreshButton).toBeTruthy()
    await refreshButton!.trigger('click')
    await flushPromises()
    expect(wrapper.get('.board-header-aside strong').text()).toBe('304')

    await vi.advanceTimersByTimeAsync(30_000)
    await flushPromises()
    expect(wrapper.get('.board-header-aside strong').text()).toBe('305')
    expect(operationalOverview).toHaveBeenCalledTimes(3)

    wrapper.unmount()
  })
})
