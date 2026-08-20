// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import JSZip from 'jszip'

const mocks = vi.hoisted(() => ({
  auditDecision: vi.fn(),
  reopen: vi.fn(),
  submitDesign: vi.fn(),
  triggerModuleAction: vi.fn(),
  upload: vi.fn(),
  deleteAsset: vi.fn(),
}))

vi.mock('@/services/api/resourceGroupsApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@/services/api/resourceGroupsApi')>()
  return {
    ...original,
    resourceGroupsApi: {
      ...original.resourceGroupsApi,
      auditDecision: mocks.auditDecision,
      reopen: mocks.reopen,
      submitDesign: mocks.submitDesign,
    },
  }
})

vi.mock('@/services/upload/assetUploadFlow', () => ({
  uploadTaskFileViaAssetSession: mocks.upload,
}))

vi.mock('@/services/api/assetsApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@/services/api/assetsApi')>()
  return {
    ...original,
    assetsApi: {
      ...original.assetsApi,
      deleteAsset: mocks.deleteAsset,
    },
  }
})

vi.mock('@/services/api/tasksApi', () => ({
  tasksApi: {
    triggerModuleAction: mocks.triggerModuleAction,
  },
}))

import ResourceWorkflowPanel from './ResourceWorkflowPanel.vue'
import type { ResourceBundle } from '@/services/api/resourceGroupsApi'

function arrayBuffer(blob: Blob): Promise<ArrayBuffer> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(reader.error)
    reader.onload = () => resolve(reader.result as ArrayBuffer)
    reader.readAsArrayBuffer(blob)
  })
}

function bundle(): ResourceBundle {
  return {
    task_id: 41,
    workflow_revision: 7,
    groups: [{
      id: 9,
      task_id: 41,
      scope_kind: 'sku',
      task_sku_item_id: 51,
      sku_code: 'SKU-001',
      product_name: '测试商品名称',
      lock_version: 3,
      migration_incomplete: false,
      working_revision: {
        id: 21,
        group_id: 9,
        revision_no: 1,
        status: 'submitted',
      mode: 'single',
	      source_stage: 'design',
	      created_by: 7,
	      legacy_migration: false,
	      created_at: '2026-07-22T08:00:00Z',
        source_file: { task_asset_id: 101, file_name: 'design.psd' },
        items: [{
          id: 31,
          revision_id: 21,
          task_asset_id: 102,
          sort_order: 1,
          file: { task_asset_id: 102, file_name: 'final.png' },
        }],
        references: [],
      },
    }],
  }
}

function mountPanel(actions: string[]) {
  return mount(ResourceWorkflowPanel, {
    props: { taskId: 41, taskType: 'design', bundle: bundle(), allowedActions: actions },
    attachTo: document.body,
  })
}

function bundleWithGroups(count: number): ResourceBundle {
  const seed = bundle()
  return {
    ...seed,
    groups: Array.from({ length: count }, (_, index) => ({
      ...seed.groups[0],
      id: index + 1,
      task_sku_item_id: index + 100,
      sku_code: `SKU-${String(index + 1).padStart(3, '0')}`,
      working_revision: seed.groups[0].working_revision ? {
        ...seed.groups[0].working_revision,
        id: index + 1000,
        group_id: index + 1,
      } : undefined,
    })),
  }
}

function button(wrapper: ReturnType<typeof mountPanel>, label: string) {
  const target = wrapper.findAll('button').find((item) => item.text() === label)
  if (!target) throw new Error(`missing button ${label}`)
  return target
}

describe('ResourceWorkflowPanel action contract', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.auditDecision.mockResolvedValue(bundle())
    mocks.reopen.mockResolvedValue(bundle())
    mocks.submitDesign.mockResolvedValue(bundle())
    mocks.triggerModuleAction.mockResolvedValue({ data: { data: { task_id: 41 } } })
    mocks.upload.mockReset()
    mocks.upload.mockResolvedValue({ version: { id: 201 } })
    mocks.deleteAsset.mockReset()
    mocks.deleteAsset.mockResolvedValue(undefined)
  })

  it.each([
    {
      name: 'approve only',
      actions: ['task.audit.approve'],
      visible: ['上传最终成品图', '确认定稿并结单'],
      hidden: ['打回设计'],
    },
    {
      name: 'return only',
      actions: ['task.audit.return_to_design'],
      visible: ['打回设计'],
      hidden: ['确认定稿并结单'],
    },
    {
      name: 'aggregate decision',
      actions: ['task.audit.decision'],
      visible: ['打回设计', '上传最终成品图', '确认定稿并结单'],
      hidden: [],
    },
    {
      name: 'empty actions',
      actions: [],
      visible: [],
      hidden: ['打回设计', '确认定稿并结单'],
    },
  ])('renders the $name matrix without exposing unauthorized calls', async ({ actions, visible, hidden }) => {
    const wrapper = mountPanel(actions)
    for (const label of visible) expect(wrapper.text()).toContain(label)
    for (const label of hidden) expect(wrapper.text()).not.toContain(label)
    expect(mocks.auditDecision).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('makes task.reopen authoritative when stale retouch submit permission is also present', () => {
    const wrapper = mount(ResourceWorkflowPanel, {
      props: {
        taskId: 41,
        taskType: 'retouch_task',
        bundle: bundle(),
        allowedActions: ['task.design.submit', 'task.reopen'],
      },
    })

    expect(wrapper.get('.workspace-head h2').text()).toBe('修改已结单文件')
    expect(wrapper.find('.reopen-dock').exists()).toBe(true)
    expect(wrapper.find('.command-dock:not(.reopen-dock):not(.audit-dock)').exists()).toBe(false)
    expect(wrapper.text()).toContain('审核修改成品/源文件')
    expect(wrapper.text()).toContain('确认重开并修改文件')
    expect(wrapper.text()).not.toContain('提交修图成品')
  })

  it('shows the operations set suggestion without changing the design decision', () => {
    const wrapper = mount(ResourceWorkflowPanel, {
      props: {
        taskId: 41,
        taskType: 'design',
        bundle: bundle(),
        skuModeHints: { 'SKU-001': true },
        allowedActions: ['task.design.submit'],
      },
    })
    expect(wrapper.text()).toContain('运营建议套装 · 最终由设计判定')
    expect(wrapper.get('.mode-control button.selected').text()).toBe('单图')
  })

  it('prepares the customization module only after submit-design reports it is not ready', async () => {
    mocks.submitDesign
      .mockRejectedValueOnce(new Error('定制任务尚未完成设计准备'))
      .mockResolvedValueOnce(bundle())
    const wrapper = mount(ResourceWorkflowPanel, {
      props: {
        taskId: 41,
        taskType: 'new_product_development',
        businessLane: 'customization',
        bundle: bundle(),
        allowedActions: ['task.design.submit'],
      },
    })

    await button(wrapper as ReturnType<typeof mountPanel>, '确认模式并提交源文件').trigger('click')
    await flushPromises()

    expect(mocks.triggerModuleAction).toHaveBeenCalledWith('41', 'customization', 'submit')
    expect(mocks.submitDesign).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('设计源文件与模式已提交审核。')
    wrapper.unmount()
  })

  it('uses task references before the first resource revision and reports missing files honestly', () => {
    const emptySourceBundle = bundle()
    if (emptySourceBundle.groups[0].working_revision) {
      emptySourceBundle.groups[0].working_revision.source_file = undefined
    }
    const wrapper = mount(ResourceWorkflowPanel, {
      props: {
        taskId: 41,
        taskType: 'design',
        bundle: emptySourceBundle,
        referenceCount: 3,
        allowedActions: ['task.design.submit'],
      },
    })
    expect(wrapper.text()).toContain('3 份参考资料')
    expect(wrapper.text()).toContain('还需上传 1 份设计源文件')
    expect(wrapper.text()).not.toContain('模式与源文件已就绪')
  })

  it('shows audit references and the SKU product name inside the audit workbench', () => {
    const wrapper = mount(ResourceWorkflowPanel, {
      props: {
        taskId: 41,
        taskType: 'design',
        bundle: bundle(),
        referenceCount: 1,
        taskReferences: [{
          asset_id: 'reference-1',
          filename: '运营参考图.png',
          mime_type: 'image/png',
          download_url: 'https://example.test/reference.png',
        }],
        allowedActions: ['task.audit.approve'],
      },
      global: {
        stubs: {
          AssetPreviewMedia: { template: '<img class="reference-preview-stub" />' },
          ImagePreviewLightbox: true,
        },
      },
    })

    expect(wrapper.get('.audit-references').text()).toContain('运营参考图.png')
    expect(wrapper.get('.sku-product-name').text()).toBe('测试商品名称')
    expect(wrapper.find('.reference-preview-stub').exists()).toBe(true)
    wrapper.unmount()
  })

  it('requires a reason before return, cancels safely, and confirms only once', async () => {
    let resolveDecision: ((value: ResourceBundle) => void) | undefined
    mocks.auditDecision.mockReturnValue(new Promise<ResourceBundle>((resolve) => { resolveDecision = resolve }))
    const wrapper = mountPanel(['task.audit.return_to_design'])
    const returnButton = button(wrapper, '打回设计')
    expect(returnButton.attributes('disabled')).toBeDefined()

    await wrapper.get('.audit-dock input').setValue('需要补充主视图')
    await returnButton.trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="dialog"]').text()).toContain('任务回到设计处理中')

    await button(wrapper, '取消').trigger('click')
    expect(mocks.auditDecision).not.toHaveBeenCalled()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)

    await button(wrapper, '打回设计').trigger('click')
    const confirmButton = button(wrapper, '确认执行')
    await confirmButton.trigger('click')
    await confirmButton.trigger('click')
    expect(mocks.auditDecision).toHaveBeenCalledTimes(1)
    expect(mocks.auditDecision).toHaveBeenCalledWith(41, expect.anything(), 'return_to_design', '需要补充主视图')
    resolveDecision?.(bundle())
    await flushPromises()
    wrapper.unmount()
  })

  it('uses the designer mode and sends a newly staged final when approving', async () => {
    const wrapper = mountPanel(['task.audit.approve'])
    expect(wrapper.get('.mode-control button.selected').text()).toBe('单图')
    expect(wrapper.get('.mode-control button.selected').attributes('disabled')).toBeUndefined()
    const fileInput = wrapper.get('.final-drop input[type="file"]')
    const replacement = new File(['png'], 'reviewed.png', { type: 'image/png' })
    Object.defineProperty(fileInput.element, 'files', { configurable: true, value: [replacement] })
    await fileInput.trigger('change')
    await flushPromises()

    await button(wrapper, '确认定稿并结单').trigger('click')
    expect(wrapper.get('[role="dialog"]').text()).toContain('本次定稿成为最终成品')
    await button(wrapper, '确认通过并结单').trigger('click')
    await flushPromises()

    expect(mocks.auditDecision).toHaveBeenCalledWith(
      41,
      expect.anything(),
      'approve',
      '',
      [{
        group_id: 9,
        expected_group_lock_version: 3,
        mode: 'single',
        final_task_asset_ids: [201],
      }],
    )
    wrapper.unmount()
  })

  it('accepts a PDF as the single finalized delivery', async () => {
    mocks.upload.mockResolvedValueOnce({ version: { id: 211 } })
    const wrapper = mountPanel(['task.audit.approve'])
    const fileInput = wrapper.get('.final-drop input[type="file"]')
    expect(fileInput.attributes('accept')).toContain('application/pdf')
    expect(wrapper.get('.final-drop').text()).toContain('PDF')

    const pdf = new File(['%PDF-1.7'], '审核成品.pdf', { type: 'application/pdf' })
    Object.defineProperty(fileInput.element, 'files', { configurable: true, value: [pdf] })
    await fileInput.trigger('change')
    await flushPromises()

    expect(mocks.upload).toHaveBeenCalledWith(
      '41',
      pdf,
      expect.objectContaining({ asset_kind: 'delivery', target_sku_code: 'SKU-001', remark: '审核成品.pdf' }),
      undefined,
    )
    expect(wrapper.text()).toContain('审核成品.pdf')
    expect(wrapper.text()).not.toContain('成品只支持图片')
    wrapper.unmount()
  })

  it('allows the auditor to change one mode and apply the first mode to every SKU', async () => {
    const auditBundle = bundleWithGroups(3)
    if (auditBundle.groups[0].working_revision) auditBundle.groups[0].working_revision.mode = 'set'
    const wrapper = mount(ResourceWorkflowPanel, {
      props: {
        taskId: 41,
        taskType: 'design',
        bundle: auditBundle,
        allowedActions: ['task.audit.approve'],
      },
    })

    expect(wrapper.text()).toContain('审核可调整')
    expect(wrapper.text()).toContain('统一按首个模式变更全部')
    await wrapper.findAll('.mode-control').at(1)?.findAll('button').find((item) => item.text() === '套装')?.trigger('click')
    await wrapper.findAll('button').find((item) => item.text().includes('统一按首个模式变更全部'))?.trigger('click')

    expect(wrapper.findAll('.mode-control button.selected').every((item) => item.text() === '套装')).toBe(true)
    expect(wrapper.emitted('dirty-change')?.some((entry) => entry[0] === true)).toBe(true)
    wrapper.unmount()
  })

  it('supports Escape, focus containment, and focus restoration', async () => {
    const wrapper = mountPanel(['task.audit.approve'])
    const fileInput = wrapper.get('.final-drop input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', { configurable: true, value: [new File(['png'], 'reviewed.png', { type: 'image/png' })] })
    await fileInput.trigger('change')
    await flushPromises()
    const trigger = button(wrapper, '确认定稿并结单')
    ;(trigger.element as HTMLButtonElement).focus()
    await trigger.trigger('click')
    await flushPromises()
    expect((document.activeElement as HTMLElement)?.textContent).toBe('取消')

    const confirm = button(wrapper, '确认通过并结单')
    ;(confirm.element as HTMLButtonElement).focus()
    await wrapper.get('[role="dialog"]').trigger('keydown', { key: 'Tab' })
    expect((document.activeElement as HTMLElement)?.textContent).toBe('×')

    await wrapper.get('[role="dialog"]').trigger('keydown', { key: 'Escape' })
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(document.activeElement).toBe(trigger.element)
    expect(mocks.auditDecision).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('renders a bounded editor window for 200 SKU groups and reports unsaved changes', async () => {
    const wrapper = mount(ResourceWorkflowPanel, {
      props: { taskId: 41, taskType: 'design', bundle: bundleWithGroups(200), allowedActions: ['task.design.submit'] },
    })
    await flushPromises()
    expect(wrapper.findAll('.sku-workbench').length).toBeLessThan(10)
    const viewport = wrapper.get('[data-testid="resource-editor-viewport"]')
    ;(viewport.element as HTMLElement).scrollTop = 199 * 330
    await viewport.trigger('scroll')
    await flushPromises()
    expect(wrapper.find('[data-group-index="199"]').exists()).toBe(true)

    const setButton = wrapper.findAll('.mode-control button').find((item) => item.text() === '套装')
    await setButton?.trigger('click')
    expect(wrapper.emitted('dirty-change')?.some((entry) => entry[0] === true)).toBe(true)
    wrapper.unmount()
  })

  it('lets design choose single or set, accepts one source, and never submits finals', async () => {
    const wrapper = mountPanel(['task.design.submit'])
    expect(wrapper.text()).toContain('设计阶段只提交源文件')
    expect(wrapper.text()).toContain('审核阶段上传')
    expect(wrapper.find('.final-drop').exists()).toBe(false)
    await wrapper.findAll('.mode-control button').find((item) => item.text() === '套装')?.trigger('click')
    await button(wrapper, '确认模式并提交源文件').trigger('click')
    await flushPromises()
    expect(mocks.submitDesign).toHaveBeenCalledWith(41, expect.anything(), [{
      group_id: 9,
      expected_group_lock_version: 3,
      mode: 'set',
      source_task_asset_id: 101,
      final_task_asset_ids: [],
    }])
    wrapper.unmount()
  })

  it('packages source files selected together or in sequence into one effective source asset', async () => {
    mocks.upload
      .mockResolvedValueOnce({ version: { id: 301 } })
      .mockResolvedValueOnce({ version: { id: 302 } })
    const wrapper = mountPanel(['task.design.submit'])
    const sourceInput = wrapper.get('.source-drop input[type="file"]')
    const psd = new File(['psd'], '主图.psd', { type: 'application/octet-stream' })
    Object.defineProperty(sourceInput.element, 'files', { configurable: true, value: [psd] })
    await sourceInput.trigger('change')
    await flushPromises()
    expect(mocks.upload.mock.calls[0][1]).toBe(psd)

    const ai = new File(['ai'], '刀版.ai', { type: 'application/octet-stream' })
    const nextSourceInput = wrapper.get('.source-drop input[type="file"]')
    Object.defineProperty(nextSourceInput.element, 'files', { configurable: true, value: [ai] })
    await nextSourceInput.trigger('change')
    await flushPromises()
    await vi.waitFor(() => expect(mocks.upload).toHaveBeenCalledTimes(2))

    const bundled = mocks.upload.mock.calls[1][1] as File
    expect(bundled.name).toContain('设计源文件-2份.zip')
    const zip = await JSZip.loadAsync(await arrayBuffer(bundled))
    expect(Object.keys(zip.files)).toEqual(['001_主图.psd', '002_刀版.ai', 'manifest.json'])
    expect(wrapper.text()).toContain('本次已打包 2 份源文件')
    expect(wrapper.text()).toContain('主图.psd、刀版.ai')

    await button(wrapper, '确认模式并提交源文件').trigger('click')
    await flushPromises()
    expect(mocks.submitDesign).toHaveBeenCalledWith(41, expect.anything(), [expect.objectContaining({
      source_task_asset_id: 302,
    })])
    wrapper.unmount()
  })

  it('removes an unsubmitted source upload and restores the upload control', async () => {
    const emptySourceBundle = bundle()
    if (emptySourceBundle.groups[0].working_revision) {
      emptySourceBundle.groups[0].working_revision.source_file = undefined
    }
    mocks.upload.mockResolvedValueOnce({ asset: { id: '8801' }, version: { id: 301 } })
    const wrapper = mount(ResourceWorkflowPanel, {
      props: { taskId: 41, taskType: 'design', bundle: emptySourceBundle, allowedActions: ['task.design.submit'] },
    })
    const sourceInput = wrapper.get('.source-drop input[type="file"]')
    Object.defineProperty(sourceInput.element, 'files', {
      configurable: true,
      value: [new File(['wrong'], '上传错了.psd', { type: 'application/octet-stream' })],
    })
    await sourceInput.trigger('change')
    await flushPromises()

    expect(wrapper.text()).toContain('上传错了.psd')
    await wrapper.get('[aria-label="移除未提交的设计源文件"]').trigger('click')
    await flushPromises()

    expect(mocks.deleteAsset).toHaveBeenCalledWith('8801', { reason: '未提交前移除上传错误的设计源文件' })
    expect(wrapper.text()).not.toContain('上传错了.psd')
    expect(wrapper.text()).toContain('已移除未提交源文件，可以重新上传。')
    expect(wrapper.text()).toContain('选择一份或多份 PSD、AI、PSB、TIF、ZIP 等源文件')
    wrapper.unmount()
  })

  it('accepts one ZIP containing a complete ordered final set', async () => {
    const setBundle = bundle()
    if (setBundle.groups[0].working_revision) setBundle.groups[0].working_revision.mode = 'set'
    mocks.upload
      .mockResolvedValueOnce({ version: { id: 401 } })
      .mockResolvedValueOnce({ version: { id: 402 } })
    const wrapper = mount(ResourceWorkflowPanel, {
      props: { taskId: 41, taskType: 'design', bundle: setBundle, allowedActions: ['task.audit.approve'] },
    })
    const zip = new JSZip()
    zip.file('02-背面.png', 'back')
    zip.file('01-正面.jpg', 'front')
    const archive = new File([await zip.generateAsync({ type: 'blob' })], '套装成品.zip', { type: 'application/zip' })
    const finalInput = wrapper.get('.final-drop input[type="file"]')
    Object.defineProperty(finalInput.element, 'files', { configurable: true, value: [archive] })
    await finalInput.trigger('change')
    await flushPromises()
    await vi.waitFor(() => expect(mocks.upload).toHaveBeenCalledTimes(2))

    expect(mocks.upload.mock.calls.map((call) => (call[1] as File).name)).toEqual(['01-正面.jpg', '02-背面.png'])
    expect(wrapper.text()).toContain('当前 2 个文件')
    await button(wrapper as ReturnType<typeof mountPanel>, '确认定稿并结单').trigger('click')
    await button(wrapper as ReturnType<typeof mountPanel>, '确认通过并结单').trigger('click')
    await flushPromises()
    expect(mocks.auditDecision).toHaveBeenCalledWith(41, expect.anything(), 'approve', '', [{
      group_id: 9,
      expected_group_lock_version: 3,
      mode: 'set',
      final_task_asset_ids: [401, 402],
    }])
    wrapper.unmount()
  })

  it('restores the designer source when audit replacement is cancelled', async () => {
    mocks.upload.mockResolvedValueOnce({ version: { id: 202 } })
    const wrapper = mountPanel(['task.audit.approve'])
    const replacementToggle = wrapper.get('.replace-toggle input')
    await replacementToggle.setValue(true)
    const sourceInput = wrapper.get('.source-drop input[type="file"]')
    Object.defineProperty(sourceInput.element, 'files', { configurable: true, value: [new File(['psd'], 'audit.psd')] })
    await sourceInput.trigger('change')
    await flushPromises()
    expect(wrapper.text()).toContain('audit.psd')

    await replacementToggle.setValue(false)
    expect(wrapper.text()).toContain('design.psd')
    expect(wrapper.text()).not.toContain('audit.psd')

    const finalInput = wrapper.get('.final-drop input[type="file"]')
    Object.defineProperty(finalInput.element, 'files', { configurable: true, value: [new File(['png'], 'reviewed.png', { type: 'image/png' })] })
    await finalInput.trigger('change')
    await flushPromises()
    await button(wrapper, '确认定稿并结单').trigger('click')
    await button(wrapper, '确认通过并结单').trigger('click')
    await flushPromises()
    expect(mocks.auditDecision).toHaveBeenCalledWith(41, expect.anything(), 'approve', '', [{
      group_id: 9,
      expected_group_lock_version: 3,
      mode: 'single',
      final_task_asset_ids: [201],
    }])
    wrapper.unmount()
  })
})
