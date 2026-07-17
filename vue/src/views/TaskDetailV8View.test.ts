// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  getById: vi.fn(), listTaskEvents: vi.fn(), listAuditHandovers: vi.fn(), auditHandover: vi.fn(), auditTakeover: vi.fn(),
  taskBundle: vi.fn(), push: vi.fn(), route: { params: { id: '41' } },
}))
vi.mock('@/services/api/tasksApi', () => ({ tasksApi: mocks }))
vi.mock('@/services/api/resourceGroupsApi', async (loadOriginal) => ({
  ...(await loadOriginal<typeof import('@/services/api/resourceGroupsApi')>()),
  resourceGroupsApi: { taskBundle: mocks.taskBundle },
}))
vi.mock('vue-router', () => ({
  useRoute: () => mocks.route,
  useRouter: () => ({ push: mocks.push }),
  onBeforeRouteLeave: vi.fn(),
}))

import TaskDetailV8View from './TaskDetailV8View.vue'

const bundle = { task_id: 41, workflow_revision: 3, groups: [] }
const baseTask = {
  id: 41,
  task_no: 'RW-041',
  task_type: 'original_product_development',
  task_status: 'PendingAudit',
  workflow_revision: 3,
  workflow_contract_version: 2,
  business_lane: 'normal',
  allowed_actions: ['task.audit.approve', 'task.audit.handover'],
  product_name_snapshot: '水杯主图',
  primary_sku_code: 'SKU-041',
  current_handler_name: '审核甲',
  owner_department: '设计部',
  owner_org_team: '主图组',
  requirement_description: '突出杯盖结构并保留白底。',
  operation_note: '客户周五前需要初稿。',
  reference_file_refs: [{ id: 1, file_name: '参考.jpg', download_url: 'https://files/reference' }],
}

function mountView() {
  return mount(TaskDetailV8View, {
    global: {
      stubs: {
        WorkflowProgress: { template: '<div class="progress-stub">四步流程</div>' },
        TaskStatusTag: { template: '<span class="status-stub">状态</span>' },
        SkuResourceMatrix: { template: '<div class="matrix-stub">资源矩阵</div>' },
        ResourceWorkflowPanel: { template: '<div class="workflow-stub">审核动作</div>' },
      },
    },
  })
}

describe('TaskDetailV8View business context', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.taskBundle.mockResolvedValue(bundle)
    mocks.listTaskEvents.mockResolvedValue({ data: { data: [{ id: 1, event_type: 'task.updated', title: '审核已领取', created_at: '2026-07-16' }] } })
    mocks.listAuditHandovers.mockResolvedValue({ data: { data: [{ id: 9, handover_no: 'HO-9', status: 'pending_takeover', allowed_actions: ['task.audit.takeover'] }] } })
    mocks.auditHandover.mockResolvedValue({})
    mocks.auditTakeover.mockResolvedValue({})
  })

  it('keeps normal-task requirements, notes, references, timeline, and handover actions', async () => {
    mocks.getById.mockResolvedValue({ data: { data: baseTask } })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('突出杯盖结构并保留白底')
    expect(wrapper.text()).toContain('客户周五前需要初稿')
    expect(wrapper.get('.reference-list a').attributes('href')).toBe('https://files/reference')
    expect(wrapper.text()).toContain('审核已领取')
    expect(wrapper.text()).toContain('HO-9')
    expect(wrapper.text()).toContain('发起交班')
    expect(wrapper.text()).toContain('接手')
    expect(wrapper.text()).toContain('资源矩阵')
  })

  it('uses item-level takeover actions instead of broad task audit permission', async () => {
    mocks.getById.mockResolvedValue({ data: { data: { ...baseTask, allowed_actions: ['task.audit.approve'] } } })
    mocks.listAuditHandovers.mockResolvedValue({ data: { data: [{ id: 9, handover_no: 'HO-9', status: 'pending_takeover', allowed_actions: [] }] } })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('HO-9')
    expect(wrapper.findAll('button').some((item) => item.text() === '接手')).toBe(false)
    expect(wrapper.text()).not.toContain('发起交班')
  })

  it('labels customization requirements without creating a separate workflow page', async () => {
    mocks.getById.mockResolvedValue({ data: { data: { ...baseTask, business_lane: 'customization', requirement_description: '按客户包装尺寸定制。' } } })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('定制需求')
    expect(wrapper.text()).toContain('按客户包装尺寸定制')
    expect(wrapper.text()).toContain('四步流程')
    expect(wrapper.text()).toContain('审核动作')
  })

  it('shows retouch requirements and the same resource authority', async () => {
    mocks.getById.mockResolvedValue({ data: { data: { ...baseTask, task_type: 'retouch_task', task_status: 'InProgress', requirement_description: '去除背景杂物。' } } })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('修图任务')
    expect(wrapper.text()).toContain('修图要求')
    expect(wrapper.text()).toContain('去除背景杂物')
    expect(mocks.taskBundle).toHaveBeenCalledWith(41)
  })

  it('renders planning-SKU results without treating planning images as design assets', async () => {
    mocks.getById.mockResolvedValue({ data: { data: { ...baseTask, task_type: 'sku_planning', task_status: 'Completed', allowed_actions: [], requirement_description: '生成三款杯子 SKU。' } } })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('策划 SKU 已生成')
    expect(wrapper.text()).toContain('不进入设计成品或客户素材')
    expect(wrapper.get('.planning-card a').attributes('href')).toBe('/v1/tasks/41/planning-skus/export.xlsx')
    expect(mocks.taskBundle).not.toHaveBeenCalled()
    expect(mocks.listAuditHandovers).not.toHaveBeenCalled()
    expect(wrapper.find('.workflow-stub').exists()).toBe(false)
    expect(wrapper.find('.collaboration-card').exists()).toBe(false)
  })
})
