// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const mocks = vi.hoisted(() => ({
  getById: vi.fn(), getDetail: vi.fn(), listTaskEvents: vi.fn(), listAuditHandovers: vi.fn(), auditHandover: vi.fn(), auditTakeover: vi.fn(),
  taskBundle: vi.fn(), uploadReference: vi.fn(), downloadPlanning: vi.fn(), push: vi.fn(), route: { params: { id: '41' } },
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
  onBeforeRouteUpdate: vi.fn(),
}))
vi.mock('@/services/upload/assetUploadFlow', () => ({ uploadReferenceFileRef: mocks.uploadReference }))
vi.mock('@/services/api/planningSkuApi', () => ({ planningSkuApi: { downloadTask: mocks.downloadPlanning } }))

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
    attachTo: document.body,
    global: {
      stubs: {
        WorkflowProgress: { template: '<div class="progress-stub">四步流程</div>' },
        TaskStatusTag: { template: '<span class="status-stub">状态</span>' },
        SkuResourceMatrix: { template: '<div class="matrix-stub">资源矩阵</div>' },
        ResourceWorkflowPanel: { props: ['skuModeHints'], template: '<div class="workflow-stub">{{ skuModeHints?.[\'\'] ? \'任务级套装提示\' : \'审核动作\' }}</div>' },
        TaskDetailAtmosphere: { template: '<div class="atmosphere-stub" />' },
      },
    },
  })
}

function bodyText() {
  return document.body.textContent || ''
}

function dialog() {
  const element = document.body.querySelector<HTMLElement>('[role="dialog"]')
  if (!element) throw new Error('expected workspace dialog')
  return element
}

describe('TaskDetailV8View business context', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mocks.getById.mockResolvedValue({ data: { data: baseTask } })
    mocks.taskBundle.mockResolvedValue(bundle)
    mocks.listTaskEvents.mockResolvedValue({ data: { data: { items: [{ id: 1, event_type: 'task.updated', title: '审核已领取', created_at: '2026-07-16' }] } } })
    mocks.listAuditHandovers.mockResolvedValue({ data: { data: { items: [{ id: 9, handover_no: 'HO-9', status: 'pending_takeover', allowed_actions: ['task.audit.takeover'] }] } } })
    mocks.auditHandover.mockResolvedValue({})
    mocks.auditTakeover.mockResolvedValue({})
    mocks.uploadReference.mockResolvedValue({ asset_id: 'ref-2', filename: '补充.png' })
    mocks.downloadPlanning.mockResolvedValue(undefined)
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('keeps normal-task requirements, notes, references, timeline, and handover actions', async () => {
    mocks.getDetail.mockResolvedValue({ data: { data: { task: baseTask, task_detail: { design_requirement: baseTask.requirement_description, note: baseTask.operation_note }, reference_file_refs: baseTask.reference_file_refs, events: [] } } })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('突出杯盖结构并保留白底')
    expect(wrapper.text()).toContain('客户周五前需要初稿')
    expect(wrapper.get('.reference-preview a').attributes('href')).toBe('https://files/reference')
    expect(wrapper.get('.reference-preview img').attributes('src')).toBe('https://files/reference')
    expect(wrapper.text()).toContain('审核已领取')
    await wrapper.get('.collaboration-fab').trigger('click')
    expect(bodyText()).toContain('HO-9')
    expect(document.body.querySelector('.handover-form')).not.toBeNull()
    expect(bodyText()).toContain('接手')
    ;(dialog().querySelector<HTMLButtonElement>('.close-button'))?.click()
    await flushPromises()
    await wrapper.findAll('button').find((item) => item.text().includes('查看全部文件'))?.trigger('click')
    expect(bodyText()).toContain('资源矩阵')
  })

  it('combines the full detail envelope with authoritative task allowed actions', async () => {
    const detailTask = { ...baseTask } as Partial<typeof baseTask>
    detailTask.allowed_actions = null as unknown as string[]
    mocks.getById.mockResolvedValue({ data: { data: { ...baseTask, allowed_actions: ['task.audit.approve'] } } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: detailTask, task_detail: { design_requirement: '核对定稿。' }, reference_file_refs: [] } } })
    const wrapper = mountView()
    await flushPromises()

    expect(mocks.getById).toHaveBeenCalledWith('41')
    expect(wrapper.findAll('button').some((item) => item.text() === '进入审核工作台')).toBe(true)
  })

  it('uses item-level takeover actions instead of broad task audit permission', async () => {
    mocks.getDetail.mockResolvedValue({ data: { data: { task: { ...baseTask, allowed_actions: ['task.audit.approve'] }, task_detail: {}, reference_file_refs: baseTask.reference_file_refs } } })
    mocks.listAuditHandovers.mockResolvedValue({ data: { data: [{ id: 9, handover_no: 'HO-9', status: 'pending_takeover', allowed_actions: [] }] } })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('.collaboration-fab').trigger('click')
    expect(bodyText()).toContain('HO-9')
    expect([...document.body.querySelectorAll('button')].some((item) => item.textContent?.trim() === '接手')).toBe(false)
    expect(document.body.querySelector('.handover-form')).toBeNull()
  })

  it('labels customization requirements without creating a separate workflow page', async () => {
    const customizationTask = { ...baseTask, task_type: 'regular_customization', business_lane: 'normal' }
    mocks.getById.mockResolvedValue({ data: { data: customizationTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: customizationTask, task_detail: { design_requirement: '按客户包装尺寸定制。' }, reference_file_refs: [] } } })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('常规定制 · 定制')
    expect(wrapper.text()).toContain('定制需求')
    expect(wrapper.text()).toContain('按客户包装尺寸定制')
    expect(wrapper.text()).toContain('四步流程')
    await wrapper.findAll('button').find((item) => item.text() === '进入审核工作台')?.trigger('click')
    expect(bodyText()).toContain('审核动作')
  })

  it('shows retouch requirements and the same resource authority', async () => {
    const retouchTask = { ...baseTask, task_type: 'retouch_task', task_status: 'InProgress', allowed_actions: [] }
    mocks.getById.mockResolvedValue({ data: { data: retouchTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: retouchTask, task_detail: { design_requirement: '去除背景杂物。' }, reference_file_refs: [], retouch_requirements: [{ id: 91, description: '清理主图背景', remark: '保留产品阴影' }] } } })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('修图任务')
    expect(wrapper.text()).toContain('修图要求')
    expect(wrapper.text()).toContain('去除背景杂物')
    expect(wrapper.text()).not.toContain('提交修图成品')
    await wrapper.findAll('button').find((item) => item.text() === '完整任务信息')?.trigger('click')
    expect(dialog().textContent).toContain('清理主图背景')
    expect(dialog().textContent).toContain('保留产品阴影')
    expect(mocks.taskBundle).toHaveBeenCalledWith(41)
  })

  it('supplements references only when the task exposes the exact reference action', async () => {
    const designTask = { ...baseTask, task_status: 'InProgress', allowed_actions: ['task.reference.append'] }
    mocks.getById.mockResolvedValue({ data: { data: designTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: designTask, task_detail: { design_requirement: '完成主图。' }, reference_file_refs: [] } } })
    const wrapper = mountView()
    await flushPromises()
    const input = wrapper.get('input[aria-label="补充任务参考附件"]')
    Object.defineProperty(input.element, 'files', { configurable: true, value: [new File(['png'], '补充.png', { type: 'image/png' })] })
    await input.trigger('change')
    await flushPromises()
    expect(mocks.uploadReference).toHaveBeenCalledWith(expect.any(File), expect.objectContaining({ taskId: '41', ownerModuleKey: 'basic_info', uploadPolicy: 'append_only' }))
    wrapper.unmount()
  })

  it('does not infer reference attachment access from design or audit actions', async () => {
    const designTask = { ...baseTask, task_status: 'InProgress', allowed_actions: ['task.design.submit'] }
    mocks.getById.mockResolvedValue({ data: { data: designTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: designTask, task_detail: {}, reference_file_refs: [] } } })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('input[aria-label="补充任务参考附件"]').exists()).toBe(true)
    expect(wrapper.findAll('button').some((item) => item.text().includes('补充附件'))).toBe(false)
  })

  it('does not query audit collaboration while a design task is still in progress', async () => {
    const designTask = { ...baseTask, task_status: 'InProgress', allowed_actions: ['task.design.submit'] }
    mocks.getById.mockResolvedValue({ data: { data: designTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: designTask, task_detail: { design_requirement: '完成设计源文件。' }, reference_file_refs: [] } } })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('进入设计提交')
    expect(mocks.listAuditHandovers).not.toHaveBeenCalled()
    expect(wrapper.find('.collaboration-fab').exists()).toBe(false)
  })

  it('renders planning-SKU results without treating planning images as design assets', async () => {
    mocks.getDetail.mockResolvedValue({ data: { data: { task: { ...baseTask, task_type: 'sku_planning', task_status: 'Completed', allowed_actions: [] }, task_detail: { design_requirement: '生成三款杯子 SKU。' }, reference_file_refs: [] } } })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('SKU 与策划信息已经生成')
    const exportButton = wrapper.findAll('button').find((item) => item.text() === '导出策划结果')
    expect(exportButton).toBeDefined()
    await exportButton?.trigger('click')
    expect(mocks.downloadPlanning).toHaveBeenCalledWith(41)
    expect(mocks.taskBundle).not.toHaveBeenCalled()
    expect(mocks.listAuditHandovers).not.toHaveBeenCalled()
    expect(wrapper.find('.workflow-stub').exists()).toBe(false)
    expect(wrapper.find('.collaboration-fab').exists()).toBe(false)
  })

  it('opens a compact details workspace with restored specifications and SKU context', async () => {
    mocks.getDetail.mockResolvedValue({ data: { data: { task: baseTask, task_detail: { design_requirement: '保留白底。', spec_text: '500ml', material: '不锈钢', note: '优先出主图。' }, reference_file_refs: baseTask.reference_file_refs, sku_items: [{ id: 51, sku_code: 'SKU-041-A', set_mode_hint: true }] } } })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find((item) => item.text() === '完整任务信息')?.trigger('click')
    expect(dialog().textContent).toContain('500ml')
    expect(dialog().textContent).toContain('不锈钢')
    expect(dialog().textContent).toContain('SKU-041-A')
    expect(dialog().textContent).toContain('运营建议套装 · 设计可调整')
  })

  it('keeps operational timing, cost, copy, and ERP context in the complete-information workspace', async () => {
    const enrichedTask = {
      ...baseTask,
      priority: '紧急',
      due_at: '2026-07-18T10:00:00Z',
      quantity: 240,
      cost_price: '12.50',
      cost_price_mode: '按件',
      product_short_name: '随行杯',
      copy_content: '一键开盖，随行不漏水。',
      style_keywords: '清爽、通透、夏日',
      reference_link: 'https://example.test/reference',
      filing_status: '等待同步',
      erp_sync_required: true,
    }
    mocks.getById.mockResolvedValue({ data: { data: enrichedTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: enrichedTask, task_detail: {}, reference_file_refs: [] } } })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((item) => item.text() === '完整任务信息')?.trigger('click')
    const text = dialog().textContent || ''
    for (const expected of ['紧急', '240', '¥12.50', '按件', '随行杯', '一键开盖，随行不漏水。', '清爽、通透、夏日', 'https://example.test/reference', '等待同步']) {
      expect(text).toContain(expected)
    }
  })

  it('translates raw workflow values and keeps attachments in their own preview workspace', async () => {
    const rawTask = { ...baseTask, priority: 'normal', filing_status: 'filed', task_status: 'PendingAudit' }
    mocks.getById.mockResolvedValue({ data: { data: rawTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: rawTask, task_detail: { design_requirement: '保留原始比例。' }, reference_file_refs: baseTask.reference_file_refs } } })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((item) => item.text() === '完整任务信息')?.trigger('click')
    expect(dialog().textContent).toContain('普通')
    expect(dialog().textContent).toContain('已完成 ERP 建档')
    expect(dialog().textContent).not.toMatch(/\b(normal|filed)\b/u)

    ;(dialog().querySelector<HTMLButtonElement>('.close-button'))?.click()
    await flushPromises()
    await wrapper.findAll('button').find((item) => item.text() === '查看参考附件')?.trigger('click')
    expect(dialog().getAttribute('aria-label')).toBe('参考附件')
    expect(dialog().textContent).toContain('下载文件')
    expect(dialog().textContent).toContain('参考.jpg')
    expect(dialog().textContent).not.toContain('人员与组织')
  })

  it('places the authoritative file chain before the current-stage command area', async () => {
    mocks.taskBundle.mockResolvedValue({ task_id: 41, workflow_revision: 3, groups: [{ id: 501, task_id: 41, scope_kind: 'sku', lock_version: 1, sku_code: 'SKU-041', working_revision: { id: 601, group_id: 501, revision_no: 1, status: 'submitted', mode: 'set', source_stage: 'design', source_file: { task_asset_id: 701, file_name: '设计源文件.psd' }, items: [] } }] })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: { ...baseTask, task_status: 'InProgress', allowed_actions: ['task.design.submit'] }, task_detail: {}, reference_file_refs: baseTask.reference_file_refs } } })
    const wrapper = mountView()
    await flushPromises()

    const resourceRail = wrapper.get('.resource-rail').element
    const commandStrip = wrapper.get('.command-strip').element
    expect(resourceRail.compareDocumentPosition(commandStrip) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(wrapper.text()).toContain('设计阶段不上传成品')
  })

  it('shows assignment only from the exact backend task action', async () => {
    const manageableTask = { ...baseTask, task_status: 'InProgress', allowed_actions: ['task.assign'] }
    mocks.getById.mockResolvedValue({ data: { data: manageableTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: manageableTask, task_detail: {}, reference_file_refs: [] } } })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.findAll('button').some((item) => item.text() === '指派设计')).toBe(true)

    wrapper.unmount()
    const designerOnlyTask = { ...manageableTask, allowed_actions: ['task.design.submit'] }
    mocks.getById.mockResolvedValue({ data: { data: designerOnlyTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: designerOnlyTask, task_detail: {}, reference_file_refs: [] } } })
    const secondWrapper = mountView()
    await flushPromises()
    expect(secondWrapper.findAll('button').some((item) => item.text() === '指派设计')).toBe(false)
  })

  it('passes the task-level set suggestion to original-product resource groups', async () => {
    const designTask = { ...baseTask, task_status: 'InProgress', allowed_actions: ['task.design.submit'] }
    mocks.getById.mockResolvedValue({ data: { data: designTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: designTask, task_detail: { design_requirement: '调整旧款主图。', set_mode_hint: true }, reference_file_refs: [] } } })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find((item) => item.text() === '进入设计提交')?.trigger('click')
    expect(bodyText()).toContain('任务级套装提示')
  })

  it('shows the newest activity first and never exposes raw event codes to business users', async () => {
    mocks.getDetail.mockResolvedValue({ data: { data: { task: baseTask, task_detail: {}, reference_file_refs: [] } } })
    mocks.listTaskEvents.mockResolvedValue({ data: { data: [
      { id: 11, event_type: 'task.created', created_at: '2026-07-16T08:00:00Z' },
      { id: 12, event_type: 'task.design.submitted', created_at: '2026-07-16T09:00:00Z' },
    ] } })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('.current-event').text()).toContain('设计已提交审核')
    expect(wrapper.text()).not.toContain('task.design.submitted')
    await wrapper.findAll('button').find((item) => item.text().startsWith('历史'))?.trigger('click')
    const timelineRows = [...dialog().querySelectorAll('.full-timeline li')]
    expect(timelineRows[0]?.textContent).toContain('设计已提交审核')
    expect(timelineRows[1]?.textContent).toContain('任务已创建')
    expect(dialog().textContent).not.toContain('task.design.submitted')
  })

  it('keeps keyboard focus inside workspaces and restores it after Escape', async () => {
    mocks.getDetail.mockResolvedValue({ data: { data: { task: baseTask, task_detail: {}, reference_file_refs: [] } } })
    const wrapper = mountView()
    await flushPromises()
    const trigger = wrapper.findAll('button').find((item) => item.text() === '完整任务信息')
    expect(trigger).toBeDefined()
    ;(trigger?.element as HTMLElement).focus()
    await trigger?.trigger('click')
    await flushPromises()
    const activeDialog = dialog()
    const close = activeDialog.querySelector<HTMLButtonElement>('.close-button')
    expect(document.activeElement).toBe(close)
    close?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true }))
    expect(activeDialog.contains(document.activeElement)).toBe(true)
    ;(document.activeElement as HTMLElement).dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await flushPromises()
    expect(document.body.querySelector('[role="dialog"]')).toBeNull()
    expect(document.activeElement).toBe(trigger?.element)
  })
})
