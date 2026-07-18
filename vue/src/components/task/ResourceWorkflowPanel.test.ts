// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  auditDecision: vi.fn(),
  reopen: vi.fn(),
  submitDesign: vi.fn(),
  upload: vi.fn(),
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

import ResourceWorkflowPanel from './ResourceWorkflowPanel.vue'
import type { ResourceBundle } from '@/services/api/resourceGroupsApi'

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
      lock_version: 3,
      migration_incomplete: false,
      working_revision: {
        id: 21,
        group_id: 9,
        revision_no: 1,
        status: 'submitted',
        mode: 'single',
        source_stage: 'design',
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
    mocks.upload.mockResolvedValue({ version: { id: 201 } })
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
    expect(wrapper.get('.mode-control button.selected').attributes('disabled')).toBeDefined()
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
