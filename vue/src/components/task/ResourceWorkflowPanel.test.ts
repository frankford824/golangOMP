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
      visible: ['上传修改后通过', '通过并结单'],
      hidden: ['打回设计'],
    },
    {
      name: 'return only',
      actions: ['task.audit.return_to_design'],
      visible: ['打回设计'],
      hidden: ['上传修改后通过', '通过并结单'],
    },
    {
      name: 'aggregate decision',
      actions: ['task.audit.decision'],
      visible: ['打回设计', '上传修改后通过', '通过并结单'],
      hidden: [],
    },
    {
      name: 'empty actions',
      actions: [],
      visible: [],
      hidden: ['打回设计', '上传修改后通过', '通过并结单'],
    },
  ])('renders the $name matrix without exposing unauthorized calls', async ({ actions, visible, hidden }) => {
    const wrapper = mountPanel(actions)
    for (const label of visible) expect(wrapper.text()).toContain(label)
    for (const label of hidden) expect(wrapper.text()).not.toContain(label)
    expect(mocks.auditDecision).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('requires a reason before return, cancels safely, and confirms only once', async () => {
    let resolveDecision: ((value: ResourceBundle) => void) | undefined
    mocks.auditDecision.mockReturnValue(new Promise<ResourceBundle>((resolve) => { resolveDecision = resolve }))
    const wrapper = mountPanel(['task.audit.return_to_design'])
    const returnButton = button(wrapper, '打回设计')
    expect(returnButton.attributes('disabled')).toBeDefined()

    await wrapper.get('.audit-bar input').setValue('需要补充主视图')
    await returnButton.trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="dialog"]').text()).toContain('任务将回到设计处理中')

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

  it('sends changed groups when an approver uploads a replacement final', async () => {
    const wrapper = mountPanel(['task.audit.approve'])
    await button(wrapper, '上传修改后通过').trigger('click')
    const fileInput = wrapper.findAll('input[type="file"]')[1]
    const replacement = new File(['png'], 'reviewed.png', { type: 'image/png' })
    Object.defineProperty(fileInput.element, 'files', { configurable: true, value: [replacement] })
    await fileInput.trigger('change')
    await flushPromises()

    await button(wrapper, '通过并结单').trigger('click')
    expect(wrapper.get('[role="dialog"]').text()).toContain('审核上传的源文件和成品将替换当前提交')
    await button(wrapper, '确认执行').trigger('click')
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
        source_task_asset_id: 101,
        final_task_asset_ids: [201],
      }],
    )
    wrapper.unmount()
  })

  it('supports Escape, focus containment, and focus restoration', async () => {
    const wrapper = mountPanel(['task.audit.approve'])
    const trigger = button(wrapper, '通过并结单')
    ;(trigger.element as HTMLButtonElement).focus()
    await trigger.trigger('click')
    await flushPromises()
    expect((document.activeElement as HTMLElement)?.textContent).toBe('取消')

    const confirm = button(wrapper, '确认执行')
    ;(confirm.element as HTMLButtonElement).focus()
    await wrapper.get('[role="dialog"]').trigger('keydown', { key: 'Tab' })
    expect((document.activeElement as HTMLElement)?.textContent).toBe('取消')

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
    expect(wrapper.findAll('.edit-card').length).toBeLessThan(10)
    const viewport = wrapper.get('[data-testid="resource-editor-viewport"]')
    ;(viewport.element as HTMLElement).scrollTop = 200 * 390
    await viewport.trigger('scroll')
    await flushPromises()
    expect(wrapper.find('[data-group-index="199"]').exists()).toBe(true)

    const select = wrapper.get('select')
    await select.setValue('set')
    expect(wrapper.emitted('dirty-change')?.some((entry) => entry[0] === true)).toBe(true)
    wrapper.unmount()
  })
})
