// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  route: { query: { intent: 'planning_sku' } as Record<string, string> },
  push: vi.fn(), replace: vi.fn(),
  create: vi.fn(), addTask: vi.fn(), parseBatch: vi.fn(), getIids: vi.fn(), getDraft: vi.fn(),
  permissions: new Set(['task.create', 'planning_sku.create']),
}))

vi.mock('vue-router', () => ({
  useRoute: () => mocks.route,
  useRouter: () => ({ push: mocks.push, replace: mocks.replace }),
  onBeforeRouteLeave: vi.fn(),
}))
vi.mock('@/composables/useTaskDraft', () => ({
  useTaskDraft: () => ({ save: vi.fn(), update: vi.fn(), getById: mocks.getDraft, saving: false }),
}))
vi.mock('@/stores/tasks', () => ({ useTasksStore: () => ({ addTask: mocks.addTask, getById: vi.fn() }) }))
vi.mock('@/services/api/planningSkuApi', () => ({
  planningSkuApi: {
    create: mocks.create,
    parseExcel: vi.fn(), uploadImage: vi.fn(), retryFailedERP: vi.fn(), exportSelection: vi.fn(),
    exportTaskURL: (id: number) => `/v1/tasks/${id}/planning-skus/export.xlsx`,
  },
}))
vi.mock('@/services/api/batchSkuApi', () => ({
  batchSkuApi: { parseExcel: mocks.parseBatch },
  normalizeBatchPreviewRow: (row: unknown) => row,
  formatBatchViolationMessage: (issue: { message?: string; code?: string }) => issue.message || issue.code || '请检查这一行',
}))
vi.mock('@/services/api/erpApi', () => ({ erpApi: { getProductByCode: vi.fn(), getIids: mocks.getIids } }))
vi.mock('@/services/upload/assetUploadFlow', () => ({ uploadReferenceFileRef: vi.fn() }))
vi.mock('@/services/upload/retouchRequirementUpload', () => ({ uploadRetouchRequirementPendingAssets: vi.fn() }))
vi.mock('@/composables/usePermission', () => ({ usePermission: () => ({ can: (permission: string) => mocks.permissions.has(permission) }) }))

import UnifiedTaskCreateView from './UnifiedTaskCreateView.vue'

describe('UnifiedTaskCreateView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.permissions = new Set(['task.create', 'planning_sku.create'])
    mocks.route.query = { intent: 'planning_sku' }
    mocks.create.mockResolvedValue({
      task_id: 88,
      task_no: 'RW-088',
      items: [{ task_sku_item_id: 1, sequence_no: 1, sku_code: 'SKU-001', erp_status: 'not_filed' }],
    })
    mocks.getIids.mockResolvedValue({ data: { data: [{ i_id: 'KT_STANDARD' }] } })
  })

  it('uses the planning redirect intent, validates rows, and renders the executable result flow', async () => {
    const wrapper = mount(UnifiedTaskCreateView, {
      global: {
        stubs: {
          UnifiedTaskGrid: { template: '<div class="grid-stub" />', methods: { readRowsFromWorkbook() {} } },
          IIdSelector: { template: '<div class="iid-stub" />' },
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })
    expect(wrapper.get('.compose-page').attributes('data-compose-intent')).toBe('planning_sku')
    expect(wrapper.text()).toContain('只要 SKU 编码明细')
    expect(wrapper.text()).toContain('还有 2 处需要完善')

    await wrapper.get('[data-row-index="0"] textarea').setValue('亚克力立牌 20cm')
    await wrapper.get('[data-row-index="0"] input[type="number"]').setValue('2')
    await flushPromises()
    const submit = wrapper.get('.validation-dock .primary-button')
    expect(submit.attributes('disabled')).toBeUndefined()
    await submit.trigger('click')
    await flushPromises()

    expect(mocks.create).toHaveBeenCalledWith([
      expect.objectContaining({ description_spec: '亚克力立牌 20cm', quantity: 2 }),
    ], 'none', expect.any(String))
    expect(wrapper.text()).toContain('RW-088')
    expect(wrapper.text()).toContain('SKU-001')
    wrapper.unmount()
  })

  it('keeps the legacy planning URL as one workbench intent instead of a second page', () => {
    const wrapper = mount(UnifiedTaskCreateView, {
      global: { stubs: { UnifiedTaskGrid: true, IIdSelector: true, RouterLink: true } },
    })
    expect(wrapper.find('.planning-page').exists()).toBe(false)
    expect(wrapper.findAll('.intent-card')).toHaveLength(4)
    expect(wrapper.get('.intent-card[aria-pressed="true"]').text()).toContain('只要 SKU 编码')
  })

  it('shows only planning to a planning-only creator', () => {
    mocks.permissions = new Set(['planning_sku.create'])
    const wrapper = mount(UnifiedTaskCreateView, {
      global: { stubs: { UnifiedTaskGrid: true, IIdSelector: true, RouterLink: true } },
    })
    expect(wrapper.findAll('.intent-card')).toHaveLength(1)
    expect(wrapper.get('.intent-card').text()).toContain('只要 SKU 编码')
    wrapper.unmount()
  })

  it('does not expose planning to a task-only creator', () => {
    mocks.permissions = new Set(['task.create'])
    mocks.route.query = { intent: 'planning_sku' }
    const wrapper = mount(UnifiedTaskCreateView, {
      global: { stubs: { UnifiedTaskGrid: true, IIdSelector: true, RouterLink: true } },
    })
    expect(wrapper.findAll('.intent-card')).toHaveLength(3)
    expect(wrapper.text()).not.toContain('只要 SKU 编码')
    expect(wrapper.get('.compose-page').attributes('data-compose-intent')).toBe('modify_existing')
    wrapper.unmount()
  })

  it('imports the existing new-design workbook into the same Univer row model', async () => {
    mocks.route.query = { intent: 'new_design' }
    mocks.parseBatch.mockResolvedValue({
      data: { data: { preview: [{ source_row: 2, product_name: '导入款', design_requirement: '白底排版', product_i_id: 'KT_STANDARD', variant_json: { width: 1.2 } }], violations: [] } },
    })
    const wrapper = mount(UnifiedTaskCreateView, {
      global: { stubs: { UnifiedTaskGrid: true, IIdSelector: true, RouterLink: true } },
    })
    const input = wrapper.get('.file-button input').element as HTMLInputElement
    Object.defineProperty(input, 'files', { configurable: true, value: [new File(['xlsx'], 'new-design.xlsx')] })
    await wrapper.get('.file-button input').trigger('change')
    await flushPromises()

    expect(mocks.parseBatch).toHaveBeenCalledWith(expect.any(File))
    expect(wrapper.get('.compose-page').attributes('data-row-count')).toBe('1')
    expect(wrapper.text()).toContain('导入款')
  })

  it('blocks restored retouch drafts until local source files are selected again', async () => {
    mocks.route.query = { intent: 'retouch', draft_id: 'draft-1' }
    mocks.getDraft.mockResolvedValue({
      id: 'draft-1',
      payload: {
        intent: 'retouch',
        rows: [{
          id: 'retouch-row',
          design_requirement: '清理背景',
          reference_assets: [],
          source_assets: [{ id: 'source-1', name: 'original.psd', status: 'local' }],
          set_mode_hint: false,
        }],
      },
    })
    const wrapper = mount(UnifiedTaskCreateView, {
      global: { stubs: { UnifiedTaskGrid: true, IIdSelector: true, RouterLink: true } },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('本地素材不会保存在草稿中，请重新选择该文件')
    expect(wrapper.text()).toContain('还有 1 处需要完善')
    expect(wrapper.get('.validation-dock .primary-button').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('keeps intent-change confirmation keyboard focus inside the dialog and restores it', async () => {
    const wrapper = mount(UnifiedTaskCreateView, {
      attachTo: document.body,
      global: { stubs: { UnifiedTaskGrid: true, IIdSelector: true, RouterLink: true } },
    })
    await flushPromises()
    await wrapper.get('[data-row-index="0"] textarea').setValue('尚未保存的策划说明')
    const trigger = wrapper.findAll('.intent-card').find((item) => item.text().includes('只修图'))
    if (!trigger) throw new Error('missing retouch intent')
    ;(trigger.element as HTMLButtonElement).focus()
    await trigger.trigger('click')
    await flushPromises()
    const modal = document.querySelector<HTMLElement>('[role="alertdialog"]')
    if (!modal) throw new Error('missing confirmation dialog')
    expect((document.activeElement as HTMLElement)?.textContent).toBe('先不了')
    modal.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await flushPromises()
    expect(document.querySelector('[role="alertdialog"]')).toBeNull()
    expect(document.activeElement).toBe(trigger.element)
    wrapper.unmount()
  })
})
