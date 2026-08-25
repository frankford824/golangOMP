// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { DataScopeEnum, RoleEnum } from '@/types'
import { usePermissionsStore } from '@/stores/permissions'

const mocks = vi.hoisted(() => ({
  getById: vi.fn(), getDetail: vi.fn(), listTaskEvents: vi.fn(), listAuditHandovers: vi.fn(), auditHandover: vi.fn(), auditTakeover: vi.fn(), patchBusinessInfo: vi.fn(), patchSkuItem: vi.fn(), patchSkuItemCostInfo: vi.fn(), cancel: vi.fn(),
  taskBundle: vi.fn(), uploadReference: vi.fn(), replaceReference: vi.fn(), getPlanning: vi.fn(), downloadPlanning: vi.fn(), getDesigners: vi.fn(), listAssets: vi.fn(), resolveAssetDownload: vi.fn(), runRetouchBatchDownload: vi.fn(), push: vi.fn(), back: vi.fn(), route: { params: { id: '41' } },
}))
vi.mock('@/services/api/tasksApi', () => ({ tasksApi: mocks }))
vi.mock('@/services/api/assetsApi', async (loadOriginal) => ({
  ...(await loadOriginal<typeof import('@/services/api/assetsApi')>()),
  assetsApi: { list: mocks.listAssets },
}))
vi.mock('@/services/api/resourceGroupsApi', async (loadOriginal) => ({
  ...(await loadOriginal<typeof import('@/services/api/resourceGroupsApi')>()),
  resourceGroupsApi: { taskBundle: mocks.taskBundle },
}))
vi.mock('@/domain/asset-access', async (loadOriginal) => ({
  ...(await loadOriginal<typeof import('@/domain/asset-access')>()),
  fetchAssetDownloadMetaResolved: mocks.resolveAssetDownload,
}))
vi.mock('vue-router', () => ({
  useRoute: () => mocks.route,
  useRouter: () => ({ push: mocks.push, back: mocks.back }),
  onBeforeRouteLeave: vi.fn(),
  onBeforeRouteUpdate: vi.fn(),
}))
vi.mock('@/services/upload/assetUploadFlow', () => ({ uploadReferenceFileRef: mocks.uploadReference }))
vi.mock('@/domain/retouch-requirement-batch-download', async (loadOriginal) => ({
  ...(await loadOriginal<typeof import('@/domain/retouch-requirement-batch-download')>()),
  runRetouchBatchDownload: mocks.runRetouchBatchDownload,
}))
vi.mock('@/services/api/planningSkuApi', () => ({ planningSkuApi: { getTask: mocks.getPlanning, downloadTask: mocks.downloadPlanning } }))
vi.mock('@/services/api/usersApi', () => ({ usersApi: { getDesigners: mocks.getDesigners } }))

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
        SkuResourceMatrix: { props: ['taskReferences'], template: '<div class="matrix-stub">资源矩阵 {{ taskReferences?.length || 0 }}</div>' },
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
    window.history.replaceState({}, '')
    mocks.getById.mockResolvedValue({ data: { data: baseTask } })
    mocks.taskBundle.mockResolvedValue(bundle)
    mocks.listTaskEvents.mockResolvedValue({ data: { data: { items: [{ id: 1, event_type: 'task.updated', title: '审核已领取', created_at: '2026-07-16' }] } } })
    mocks.listAuditHandovers.mockResolvedValue({ data: { data: { items: [{ id: 9, handover_no: 'HO-9', status: 'pending_takeover', allowed_actions: ['task.audit.takeover'] }] } } })
    mocks.auditHandover.mockResolvedValue({})
    mocks.auditTakeover.mockResolvedValue({})
    mocks.patchBusinessInfo.mockResolvedValue({})
    mocks.patchSkuItem.mockResolvedValue({})
    mocks.patchSkuItemCostInfo.mockResolvedValue({})
    mocks.cancel.mockResolvedValue({})
    mocks.listAssets.mockResolvedValue({ data: { data: [] } })
    mocks.resolveAssetDownload.mockResolvedValue({ status: 'not_found', message: '资源不存在' })
    mocks.runRetouchBatchDownload.mockResolvedValue({ ok: true, writtenCount: 2, failureCount: 0 })
    mocks.uploadReference.mockResolvedValue({ asset_id: 'ref-2', filename: '补充.png' })
    mocks.replaceReference.mockResolvedValue({})
    mocks.getPlanning.mockResolvedValue({ task_id: 41, task_no: 'RW-041', task_status: 'Completed', workflow_revision: 3, items: [] })
    mocks.downloadPlanning.mockResolvedValue(undefined)
    mocks.getDesigners.mockResolvedValue({ data: { data: [{ id: 99, display_name: '定制设计师' }] } })
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
    expect(wrapper.get('.resource-rail .rail-column.references').text()).toContain('参考.jpg')
    expect(wrapper.find('.references-card').exists()).toBe(false)
    expect(wrapper.findAll('button').filter((item) => item.text() === '完整任务信息')).toHaveLength(1)
    expect(wrapper.text()).not.toContain('完整资料')
    expect(wrapper.text()).not.toContain('人员与组织')
    expect(wrapper.text()).toContain('审核已领取')
    await wrapper.findAll('button').find((item) => item.text() === '审核协作')?.trigger('click')
    expect(bodyText()).toContain('HO-9')
    expect(document.body.querySelector('.handover-form')).not.toBeNull()
    expect(bodyText()).toContain('接手')
    ;(dialog().querySelector<HTMLButtonElement>('.close-button'))?.click()
    await flushPromises()
    await wrapper.findAll('button').find((item) => item.text().includes('SKU 资源总览'))?.trigger('click')
    expect(bodyText()).toContain('资源矩阵 1')
  })

  it('uses a contextual back label when task detail was opened from the dashboard', async () => {
    window.history.replaceState({ back: '/' }, '')
    mocks.getDetail.mockResolvedValue({ data: { data: { task: baseTask, reference_file_refs: [], events: [] } } })

    const wrapper = mountView()
    await flushPromises()

    const backButton = wrapper.get('.back-button')
    expect(backButton.text()).toBe('返回')
    await backButton.trigger('click')
    expect(mocks.back).toHaveBeenCalledOnce()
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

    await wrapper.findAll('button').find((item) => item.text() === '审核协作')?.trigger('click')
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
    const retouchTask = { ...baseTask, task_type: 'retouch_task', task_status: 'InProgress', allowed_actions: [], reference_file_refs: [] }
    mocks.getById.mockResolvedValue({ data: { data: retouchTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: retouchTask, task_detail: { design_requirement: '去除背景杂物。' }, reference_file_refs: [], retouch_requirements: [{
      id: 91,
      description: '清理主图背景',
      remark: '保留产品阴影',
      reference_file_refs: [{ ref_id: 'requirement-ref', filename: '需求参考图.jpg', mime_type: 'image/jpeg', download_url: 'https://files/requirement-ref' }],
      source_assets: [
        { id: 201, current_version: { id: 301, file_name: '主图.png', download_url: 'https://files/source-main.png', mime_type: 'image/png' } },
        { id: 202, current_version: { id: 302, file_name: '细节图.png', download_url: 'https://files/source-detail.png', mime_type: 'image/png' } },
      ],
    }] } } })
    mocks.taskBundle.mockResolvedValue({
      task_id: 41,
      workflow_revision: 3,
      groups: [{
        id: 91,
        task_id: 41,
        scope_kind: 'retouch_requirement',
        retouch_requirement_id: 91,
        lock_version: 1,
        migration_incomplete: false,
        finalized_revision: {
          id: 191,
          group_id: 91,
          revision_no: 1,
          status: 'finalized',
          mode: 'single',
          source_stage: 'retouch',
          created_by: 1,
          legacy_migration: true,
          created_at: '2026-07-16T08:00:00Z',
          references: [],
          items: [],
        },
      }],
    })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('修图任务')
    expect(wrapper.text()).toContain('修图要求')
    expect(wrapper.text()).toContain('修图范围 · 1 项')
    expect(wrapper.text()).toContain('去除背景杂物')
    expect(wrapper.text()).toContain('修图任务无需独立源文件')
    expect(wrapper.text()).not.toContain('提交修图成品')
    expect(wrapper.get('.resource-rail .rail-column.references').text()).toContain('1 个附件')
    const batchButton = wrapper.findAll('button').find((item) => item.text() === '批量下载全部')
    expect(batchButton).toBeDefined()
    await batchButton?.trigger('click')
    expect(mocks.runRetouchBatchDownload).toHaveBeenCalledWith(
      expect.objectContaining({
        entries: expect.arrayContaining([
          expect.objectContaining({ assetId: 201, zipPath: '需求1/素材文件' }),
          expect.objectContaining({ assetId: 202, zipPath: '需求1/素材文件' }),
        ]),
        assetIdCount: 2,
      }),
      expect.stringContaining('素材文件'),
      expect.any(Function),
    )
    expect(wrapper.text()).toContain('已打包 2 个待修素材')
    expect(wrapper.find('.references-card').exists()).toBe(false)
    await wrapper.findAll('button').find((item) => item.text() === '参考资料总览')?.trigger('click')
    expect(dialog().getAttribute('aria-label')).toBe('运营参考图')
    expect(dialog().textContent).toContain('需求参考图.jpg')
    ;(dialog().querySelector<HTMLButtonElement>('.close-button'))?.click()
    await flushPromises()
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

  it('replaces the selected task-level reference after uploading its successor', async () => {
    const designTask = { ...baseTask, task_status: 'InProgress', allowed_actions: ['task.reference.append', 'task.business_info.edit'] }
    const oldReference = { asset_id: 'legacy-old', ref_id: 'legacy-old', filename: '旧参考图.png', mime_type: 'image/png', download_url: '/old.png' }
    mocks.getById.mockResolvedValue({ data: { data: designTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: designTask, task_detail: { design_requirement: '完成主图。' }, reference_file_refs: [oldReference] } } })
    mocks.uploadReference.mockResolvedValueOnce({ asset_id: '54551', ref_id: 'new-ref', filename: '新参考图.png' })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((item) => item.text() === '完整任务信息')?.trigger('click')
    await flushPromises()
    const attachmentButton = [...document.body.querySelectorAll<HTMLButtonElement>('button')].find((item) => item.textContent?.includes('查看全部附件'))
    expect(attachmentButton).toBeDefined()
    attachmentButton?.click()
    await flushPromises()
    const replaceButton = [...document.body.querySelectorAll<HTMLButtonElement>('button')].find((item) => item.textContent?.includes('替换当前参考图'))
    expect(replaceButton).toBeDefined()
    replaceButton?.click()
    await flushPromises()

    const input = wrapper.get('input[aria-label="替换当前任务参考附件"]')
    Object.defineProperty(input.element, 'files', { configurable: true, value: [new File(['png'], '新参考图.png', { type: 'image/png' })] })
    await input.trigger('change')
    await flushPromises()

    expect(mocks.uploadReference).toHaveBeenCalledWith(expect.any(File), expect.objectContaining({ taskId: '41', ownerModuleKey: 'basic_info', uploadPolicy: 'append_only' }))
    expect(mocks.replaceReference).toHaveBeenCalledWith('41', { old_ref_id: 'legacy-old', new_asset_id: 54551 })
    expect(wrapper.text()).toContain('已将“旧参考图.png”替换为“新参考图.png”')
  })

  it('does not infer reference attachment access from design or audit actions', async () => {
    const designTask = { ...baseTask, task_status: 'InProgress', allowed_actions: ['task.design.submit'] }
    mocks.getById.mockResolvedValue({ data: { data: designTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: designTask, task_detail: {}, reference_file_refs: [] } } })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('input[aria-label="补充任务参考附件"]').exists()).toBe(false)
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
    mocks.getPlanning.mockResolvedValue({
      task_id: 41,
      task_no: 'RW-041',
      task_status: 'Completed',
      workflow_revision: 3,
      items: [{
        task_sku_item_id: 501,
        sequence_no: 1,
        sku_code: 'PLAN-001',
        quantity: 20,
        revision: {
          id: 51,
          version_no: 1,
          description_spec: '红色礼盒',
          quantity: 20,
          product_image_name: '礼盒.jpg',
          product_image_url: 'https://cloneb-oss.example/plan-001.jpg',
        },
      }],
    })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('SKU 与策划信息已经生成')
    expect(wrapper.text()).toContain('PLAN-001')
    expect(wrapper.text()).toContain('红色礼盒')
    expect(wrapper.get('.planning-image img').attributes('src')).toBe('https://cloneb-oss.example/plan-001.jpg')
    expect(mocks.getPlanning).toHaveBeenCalledWith(41)
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
    expect(dialog().textContent).toContain('SKU-041-A 成本规则试算与解释')
  })

  it('lets catalog managers update one batch SKU specification and manual cost with an audit reason', async () => {
    const permissions = usePermissionsStore()
    permissions.setCurrentUser({
      id: '9',
      name: '运营管理员',
      role: RoleEnum.OPS,
      departmentId: '',
      groupId: '',
      dataScope: DataScopeEnum.GLOBAL,
      permissions: [],
    })
    permissions.actions = ['catalog.manage']
    const skuItem = {
      id: 51,
      sku_code: 'SKU-041-A',
      product_name_snapshot: '子项水杯',
      product_i_id: 'ERP-041-A',
      quantity: 20,
      cost_price: 12.5,
      variant_json: { spec_text: '旧规格', size_text: '10×20cm', width: 10, height: 20, area: 200 },
    }
    mocks.getDetail.mockResolvedValue({ data: { data: { task: baseTask, task_detail: {}, reference_file_refs: [], sku_items: [skuItem] } } })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find((item) => item.text() === '完整任务信息')?.trigger('click')

    const activeDialog = dialog()
    const labels = [...activeDialog.querySelectorAll('.sku-editor label')]
    const inputFor = (name: string) => labels.find((label) => label.textContent?.includes(name))?.querySelector<HTMLInputElement>('input')
    const specInput = inputFor('规格')
    const costInput = inputFor('当前/人工成本')
    const reasonInput = inputFor('成本调整原因')
    expect(specInput?.value).toBe('旧规格')
    if (specInput) specInput.value = '新规格'
    specInput?.dispatchEvent(new Event('input', { bubbles: true }))
    if (costInput) costInput.value = '15.8'
    costInput?.dispatchEvent(new Event('input', { bubbles: true }))
    if (reasonInput) reasonInput.value = '供应商报价调整'
    reasonInput?.dispatchEvent(new Event('input', { bubbles: true }))
    const saveButton = [...activeDialog.querySelectorAll('button')].find((button) => button.textContent?.includes('保存该 SKU'))
    expect(saveButton).toBeDefined()
    saveButton?.click()
    await flushPromises()

    expect(mocks.patchSkuItem).toHaveBeenCalledWith('41', 51, expect.objectContaining({ spec_text: '新规格', quantity: 20 }))
    expect(mocks.patchSkuItemCostInfo).toHaveBeenCalledWith('41', 51, expect.objectContaining({
      cost_price: 15.8,
      manual_cost_override: true,
      manual_cost_override_reason: '供应商报价调整',
    }))
  })

  it('lets the creator update business fields and audited manual cost on their own active batch task', async () => {
    const permissions = usePermissionsStore()
    permissions.setCurrentUser({
      id: '240',
      name: '运营创建者',
      role: RoleEnum.OPS,
      departmentId: '',
      groupId: '',
      dataScope: DataScopeEnum.SELF,
      permissions: [],
    })
    permissions.actions = ['task.create']
    const ownTask = {
      ...baseTask,
      creator_id: 240,
      task_type: 'new_product_development',
      task_status: 'InProgress',
      allowed_actions: ['task.business_info.edit'],
    }
    mocks.getById.mockResolvedValue({ data: { data: ownTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: {
      task: ownTask,
      task_detail: {},
      reference_file_refs: [],
      sku_items: [{ id: 52, sku_code: 'SKU-OWN', product_name_snapshot: '自己创建的子项', cost_price: 19.9 }],
    } } })
    const wrapper = mountView()
    await flushPromises()
    const editButton = wrapper.findAll('button').find((item) => item.text() === '编辑任务信息')
    expect(editButton).toBeDefined()
    await editButton?.trigger('click')

    expect(dialog().querySelector('.task-business-editor')).not.toBeNull()
    expect(dialog().textContent).toContain('自己创建的未结单任务，可修改业务字段与成本；人工改价需填写原因')
    const costInput = [...dialog().querySelectorAll('label')]
      .find((label) => label.textContent?.includes('当前/人工成本'))
      ?.querySelector<HTMLInputElement>('input')
    expect(costInput?.disabled).toBe(false)
    const productInput = dialog().querySelector<HTMLInputElement>('input')
    expect(productInput?.disabled).toBe(false)

    if (!productInput) throw new Error('expected task product input')
    productInput.value = '更新后的整单名称'
    productInput.dispatchEvent(new Event('input', { bubbles: true }))
    const requirementInput = [...dialog().querySelectorAll<HTMLTextAreaElement>('.task-business-editor textarea')][0]
    requirementInput.value = '更新整张任务单的设计要求'
    requirementInput.dispatchEvent(new Event('input', { bubbles: true }))
    await wrapper.vm.$nextTick()
    ;([...dialog().querySelectorAll<HTMLButtonElement>('button')].find((item) => item.textContent?.trim() === '保存任务信息'))?.click()
    await flushPromises()

    expect(mocks.patchBusinessInfo).toHaveBeenCalledWith('41', expect.objectContaining({
      product_name: '更新后的整单名称',
      design_requirement: '更新整张任务单的设计要求',
      remark: '创建人编辑整张任务单',
    }))

    const reasonInput = [...dialog().querySelectorAll('label')]
      .find((label) => label.textContent?.includes('成本调整原因'))
      ?.querySelector<HTMLInputElement>('input')
    if (!costInput || !reasonInput) throw new Error('expected editable SKU cost fields')
    costInput.value = '20.5'
    costInput.dispatchEvent(new Event('input', { bubbles: true }))
    reasonInput.value = '核对供应商报价后修正'
    reasonInput.dispatchEvent(new Event('input', { bubbles: true }))
    ;([...dialog().querySelectorAll<HTMLButtonElement>('button')]
      .find((item) => item.textContent?.trim() === '保存该 SKU'))?.click()
    await flushPromises()

    expect(mocks.patchSkuItemCostInfo).toHaveBeenCalledWith('41', 52, expect.objectContaining({
      cost_price: 20.5,
      manual_cost_override: true,
      manual_cost_override_reason: '核对供应商报价后修正',
    }))
  })

  it('keeps another creator task read-only for a task.create-only account', async () => {
    const permissions = usePermissionsStore()
    permissions.setCurrentUser({
      id: '241',
      name: '运营成员',
      role: RoleEnum.OPS,
      departmentId: '',
      groupId: '',
      dataScope: DataScopeEnum.SELF,
      permissions: [],
    })
    permissions.actions = ['task.create']
    const otherTask = {
      ...baseTask,
      creator_id: 240,
      task_type: 'new_product_development',
      task_status: 'InProgress',
      allowed_actions: [],
    }
    mocks.getById.mockResolvedValue({ data: { data: otherTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: {
      task: otherTask,
      task_detail: {},
      reference_file_refs: [],
      sku_items: [{ id: 53, sku_code: 'SKU-OTHER', product_name_snapshot: '他人创建的子项', cost_price: 21 }],
    } } })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find((item) => item.text() === '完整任务信息')?.trigger('click')

    expect(wrapper.findAll('button').some((item) => item.text() === '编辑任务信息')).toBe(false)
    expect(dialog().querySelector('.task-business-editor')).toBeNull()
    expect(dialog().textContent).toContain('当前账号只读')
    expect([...dialog().querySelectorAll<HTMLInputElement>('.sku-editor input')].every((input) => input.disabled)).toBe(true)
  })

  it('shows all batch SKU codes without opening the details workspace', async () => {
    const batchTask = { ...baseTask, task_status: 'InProgress', allowed_actions: [] }
    mocks.getById.mockResolvedValue({ data: { data: batchTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: {
      task: batchTask,
      task_detail: {},
      reference_file_refs: [],
      sku_items: [
        { id: 71, sku_code: 'SKU-BATCH-001' },
        { id: 72, sku_code: 'SKU-BATCH-002' },
        { id: 73, sku_code: 'SKU-BATCH-003' },
      ],
    } } })
    const wrapper = mountView()
    await flushPromises()

    const strip = wrapper.get('.sku-visibility-strip')
    expect(strip.text()).toContain('本任务全部 SKU')
    expect(strip.text()).toContain('SKU-BATCH-001')
    expect(strip.text()).toContain('SKU-BATCH-002')
    expect(strip.text()).toContain('SKU-BATCH-003')
  })

  it('explains an unavailable resource area without dropping the rest of the task', async () => {
    mocks.taskBundle.mockRejectedValue({
      status: 409,
      responseData: {
        error: { code: 'INVALID_STATE_TRANSITION', details: { migration_incomplete: true, expected_groups: 2, actual_groups: 1 } },
      },
    })
    mocks.listAssets.mockResolvedValue({ data: { data: [
      { id: '71', file_role: 'source', asset_kind: 'source', file_name: '历史源文件.psd', download_url: 'https://files/legacy-source' },
      { id: '72', file_role: 'delivery', asset_kind: 'delivery', file_name: '历史成品.jpg' },
      { id: '73', file_role: 'reference', asset_kind: 'reference', file_name: '历史参考图.jpg', download_url: 'https://files/reference' },
    ] } })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('.bundle-unavailable').exists()).toBe(true)
    expect(wrapper.get('.bundle-unavailable').text()).toContain('任务资源迁移尚未完成')
    expect(wrapper.text()).toContain(baseTask.task_no)
    expect(mocks.listTaskEvents).toHaveBeenCalled()

    expect(mocks.listAssets).toHaveBeenCalledWith('41')
    const legacy = wrapper.get('.legacy-assets')
    expect(legacy.text()).toContain('历史源文件.psd')
    expect(legacy.get('a').attributes('href')).toBe('https://files/legacy-source')
    expect(legacy.text()).toContain('文件已不可用')
    expect(legacy.text()).not.toContain('历史参考图.jpg')
    expect(legacy.text()).toContain('1/2 份可下载')
    wrapper.unmount()
  })

  it('bridges legacy files when migration created an empty resource-group shell', async () => {
    mocks.taskBundle.mockResolvedValue({
      task_id: 41,
      workflow_revision: 3,
      groups: [{
        id: 5495,
        scope_kind: 'retouch_requirement',
        scope_ref_id: 292,
        working_revision: null,
        finalized_revision: null,
      }],
    })
    mocks.listAssets.mockResolvedValue({ data: { data: [
      { id: '31062', file_role: 'source', asset_kind: 'source' },
    ] } })
    mocks.resolveAssetDownload.mockResolvedValue({
      status: 'ok',
      downloadUrl: '/v1/assets/files/tasks/RW-20260731-A-003319/source.zip',
      filename: '开学掷骰子场景p图-管艳红-.zip',
    })

    const wrapper = mountView()
    await flushPromises()

    expect(mocks.listAssets).toHaveBeenCalledWith('41')
    expect(mocks.resolveAssetDownload).toHaveBeenCalledWith('31062')
    expect(wrapper.find('.bundle-unavailable').exists()).toBe(false)
    const bridge = wrapper.get('.legacy-resource-bridge')
    expect(bridge.text()).toContain('开学掷骰子场景p图-管艳红-.zip')
    expect(bridge.text()).toContain('不会伪装成一次新的设计提交')
    expect(bridge.get('a').attributes('href')).toBe('/v1/assets/files/tasks/RW-20260731-A-003319/source.zip')
    wrapper.unmount()
  })

  it('exposes the explicit task termination action and records a reason', async () => {
    const activeTask = { ...baseTask, task_status: 'InProgress', allowed_actions: ['task.terminate'] }
    mocks.getById.mockResolvedValue({ data: { data: activeTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: activeTask, task_detail: {}, reference_file_refs: [] } } })
    const prompt = vi.spyOn(window, 'prompt')
    const wrapper = mountView()
    await flushPromises()

    const terminate = wrapper.findAll('button').find((item) => item.text() === '终止任务')
    expect(terminate).toBeDefined()
    await terminate?.trigger('click')
    await flushPromises()

    const dialog = document.querySelector('.terminate-dialog')
    expect(dialog).not.toBeNull()
    const confirm = () => Array.from(dialog!.querySelectorAll('button')).find((item) => item.textContent?.includes('确认终止'))
    expect(confirm()?.disabled).toBe(true)

    const reason = dialog!.querySelector('textarea') as HTMLTextAreaElement
    reason.value = '业务需求取消'
    reason.dispatchEvent(new Event('input'))
    await flushPromises()

    confirm()?.click()
    await flushPromises()
    expect(mocks.cancel).toHaveBeenCalledWith('41', { reason: '业务需求取消', force: false })
    expect(prompt).not.toHaveBeenCalled()
    prompt.mockRestore()
    wrapper.unmount()
  })

  it('requires a second explicit confirmation before force-terminating claimed work', async () => {
    const activeTask = { ...baseTask, task_status: 'InProgress', allowed_actions: ['task.terminate'] }
    mocks.getById.mockResolvedValue({ data: { data: activeTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: activeTask, task_detail: {}, reference_file_refs: [] } } })
    mocks.cancel
      .mockRejectedValueOnce({ status: 409, denyCode: 'task_already_claimed' })
      .mockResolvedValueOnce({})
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((item) => item.text() === '终止任务')?.trigger('click')
    await flushPromises()
    const terminateDialog = document.querySelector('.terminate-dialog')!
    const reason = terminateDialog.querySelector('textarea') as HTMLTextAreaElement
    reason.value = '客户取消已开工任务'
    reason.dispatchEvent(new Event('input'))
    await flushPromises()

    const findConfirm = () => Array.from(terminateDialog.querySelectorAll('button'))
      .find((item) => item.classList.contains('primary-button'))
    findConfirm()?.click()
    await flushPromises()

    expect(terminateDialog.textContent).toContain('继续会强制关闭所有未完成模块')
    expect(terminateDialog.textContent).toContain('仍然强制终止')
    expect(mocks.cancel).toHaveBeenNthCalledWith(1, '41', { reason: '客户取消已开工任务', force: false })

    findConfirm()?.click()
    await flushPromises()
    expect(mocks.cancel).toHaveBeenNthCalledWith(2, '41', { reason: '客户取消已开工任务', force: true })
    expect(document.querySelector('.terminate-dialog')).toBeNull()
    wrapper.unmount()
  })

  it('keeps creator-owned terminal tasks immutable', async () => {
    const permissions = usePermissionsStore()
    permissions.setCurrentUser({
      id: '240',
      name: '运营创建者',
      role: RoleEnum.OPS,
      departmentId: '',
      groupId: '',
      dataScope: DataScopeEnum.SELF,
      permissions: [],
    })
    permissions.actions = ['task.create']
    const completedTask = {
      ...baseTask,
      creator_id: 240,
      task_status: 'Completed',
      allowed_actions: [],
    }
    mocks.getById.mockResolvedValue({ data: { data: completedTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: completedTask, task_detail: {}, reference_file_refs: [] } } })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.findAll('button').some((item) => item.text() === '编辑任务信息')).toBe(false)
    await wrapper.findAll('button').find((item) => item.text() === '完整任务信息')?.trigger('click')
    expect(dialog().querySelector('.task-business-editor')).toBeNull()
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
    await wrapper.findAll('button').find((item) => item.text() === '参考资料总览')?.trigger('click')
    expect(dialog().getAttribute('aria-label')).toBe('任务级参考附件')
    expect(dialog().textContent).toContain('下载文件')
    expect(dialog().textContent).toContain('参考.jpg')
    expect(dialog().textContent).not.toContain('人员与组织')
  })

  it('labels task.reopen as reopening and updated_at only as task information update time', async () => {
    const reopenTask = {
      ...baseTask,
      task_type: 'retouch_task',
      task_status: 'Completed',
      allowed_actions: ['task.reopen'],
      updated_at: '2026-07-24T10:00:00Z',
    }
    mocks.getById.mockResolvedValue({ data: { data: reopenTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: reopenTask, task_detail: {}, reference_file_refs: [] } } })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.findAll('button').some((item) => item.text() === '修改结单文件')).toBe(true)
    expect(wrapper.text()).not.toContain('提交修图成品')
    expect(wrapper.get('.hero-facts').text()).toContain('任务信息更新时间')
    expect(wrapper.get('.hero-facts').text()).toContain('2026/07/24 10:00')
    expect(wrapper.get('.hero-facts').text()).not.toContain('完成时间')
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

  it('keeps one primary entry for each task-detail operation', async () => {
    mocks.getDetail.mockResolvedValue({ data: { data: { task: baseTask, task_detail: {}, reference_file_refs: baseTask.reference_file_refs } } })
    const wrapper = mountView()
    await flushPromises()

    const labels = wrapper.findAll('button').map((button) => button.text().trim())
    expect(labels.filter((label) => label === '进入审核工作台')).toHaveLength(1)
    expect(labels.filter((label) => label === '审核协作')).toHaveLength(1)
    expect(labels.filter((label) => label === '完整任务信息')).toHaveLength(1)
    expect(labels.filter((label) => label.includes('SKU 资源总览'))).toHaveLength(1)
    expect(labels).not.toContain('查看任务级参考附件')
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

  it('loads the customization candidate pool when assigning a customization task', async () => {
    const permissions = usePermissionsStore()
    permissions.setCurrentUser({
      id: '9',
      name: '运营管理员',
      role: RoleEnum.OPS,
      departmentId: '',
      groupId: '',
      dataScope: DataScopeEnum.GLOBAL,
      permissions: [],
    })
    permissions.actions = ['task.assign']
    const customizationTask = {
      ...baseTask,
      task_type: 'regular_customization',
      business_lane: 'customization',
      task_status: 'PendingAssign',
      allowed_actions: ['task.assign'],
    }
    mocks.getById.mockResolvedValue({ data: { data: customizationTask } })
    mocks.getDetail.mockResolvedValue({ data: { data: { task: customizationTask, task_detail: {}, reference_file_refs: [] } } })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find((item) => item.text() === '指派设计')?.trigger('click')
    await flushPromises()

    expect(mocks.getDesigners).toHaveBeenCalledWith({ workflowLane: 'customization' })
    wrapper.unmount()
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
