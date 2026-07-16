// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  create: vi.fn(),
  parseExcel: vi.fn(),
  uploadImage: vi.fn(),
  retryFailedERP: vi.fn(),
  exportSelection: vi.fn(),
}))

vi.mock('@/services/api/planningSkuApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@/services/api/planningSkuApi')>()
  return {
    ...original,
    planningSkuApi: {
      ...original.planningSkuApi,
      create: mocks.create,
      parseExcel: mocks.parseExcel,
      uploadImage: mocks.uploadImage,
      retryFailedERP: mocks.retryFailedERP,
      exportSelection: mocks.exportSelection,
      templateURL: () => '/template.xlsx',
      exportTaskURL: (id: number) => `/export/${id}.xlsx`,
    },
  }
})

import PlanningSKUCreateView from './PlanningSKUCreateView.vue'

function installViewport(mobile = false) {
  vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: mobile, media: '', onchange: null, addListener: vi.fn(), removeListener: vi.fn(), addEventListener: vi.fn(), removeEventListener: vi.fn(), dispatchEvent: vi.fn() })))
}

describe('PlanningSKUCreateView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    installViewport(false)
    mocks.create.mockResolvedValue({ task_id: 1, task_no: 'PLAN-1', task_status: 'Completed', workflow_revision: 1, items: [] })
    mocks.parseExcel.mockResolvedValue({ planning_sku_items: [], errors: [], valid: true })
    mocks.uploadImage.mockResolvedValue('image-ref-1')
    mocks.retryFailedERP.mockResolvedValue({ queued: 1, resync: false })
    mocks.exportSelection.mockResolvedValue(undefined)
  })

  it('explains every blocking field and focuses the first invalid row', async () => {
    const wrapper = mount(PlanningSKUCreateView, { attachTo: document.body })
    await flushPromises()
    expect(wrapper.text()).toContain('还有 1 处信息需要完善')
    expect(wrapper.text()).toContain('产品描述 / 规格不能为空')
    expect(wrapper.get('.primary-button').attributes('disabled')).toBeDefined()

    await wrapper.get('input[type="number"]').setValue('0')
    await wrapper.get('input[placeholder="12.50"]').setValue('12.999')
    await wrapper.get('input[type="url"]').setValue('ftp://invalid')
    expect(wrapper.text()).toContain('数量必须是正整数')
    expect(wrapper.text()).toContain('最多保留两位小数')
    expect(wrapper.text()).toContain('必须以 http:// 或 https:// 开头')

    await wrapper.findAll('button').find((item) => item.text() === '定位第一处')?.trigger('click')
    await flushPromises()
    expect(document.activeElement?.getAttribute('aria-invalid')).toBe('true')
    wrapper.unmount()
  })

  it('renders a bounded virtual window for 200 rows and reaches the final row', async () => {
    const wrapper = mount(PlanningSKUCreateView)
    const add = wrapper.findAll('button').find((item) => item.text() === '添加一行')
    if (!add) throw new Error('missing add row button')
    for (let index = 1; index < 200; index += 1) await add.trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="virtual-total"]').text()).toContain('当前 200 行')
    expect(wrapper.findAll('[data-testid="planning-row"]').length).toBeLessThan(10)
    const viewport = wrapper.get('.virtual-list')
    ;(viewport.element as HTMLElement).scrollTop = 200 * 370
    await viewport.trigger('scroll')
    await flushPromises()
    const indexes = wrapper.findAll('[data-testid="planning-row"]').map((row) => Number(row.attributes('data-row-index')))
    expect(indexes).toContain(199)
  })

  it('uses mobile row cards and merges Excel row errors into the same validation summary', async () => {
    installViewport(true)
    mocks.parseExcel.mockResolvedValue({
      planning_sku_items: [{ client_item_id: 'excel-1', description_spec: '玻璃杯 500ml', quantity: 2 }],
      errors: [{ row: 4, field: 'reference_url', reason: '链接格式错误' }],
      valid: false,
    })
    const wrapper = mount(PlanningSKUCreateView)
    const excel = wrapper.find('input[accept=".xlsx"]')
    Object.defineProperty(excel.element, 'files', { configurable: true, value: [new File(['xlsx'], 'items.xlsx')] })
    await excel.trigger('change')
    await flushPromises()

    expect(wrapper.find('table').exists()).toBe(false)
    expect(wrapper.findAll('.planning-row')).toHaveLength(1)
    expect(wrapper.text()).toContain('Excel 第 4 行 · reference_url：链接格式错误')
    expect(wrapper.get('.primary-button').attributes('disabled')).toBeDefined()
  })

  it('filters failed ERP rows, retries them, and exports the checked result set', async () => {
    mocks.create.mockResolvedValue({
      task_id: 9,
      task_no: 'PLAN-9',
      task_status: 'Completed',
      workflow_revision: 1,
      items: [
        { task_sku_item_id: 91, sequence_no: 1, sku_code: 'SKU-91', quantity: 1, erp_status: 'succeeded' },
        { task_sku_item_id: 92, sequence_no: 2, sku_code: 'SKU-92', quantity: 1, erp_status: 'retry' },
      ],
    })
    const wrapper = mount(PlanningSKUCreateView)
    await wrapper.get('textarea[placeholder="产品名称、材质、尺寸、工艺等"]').setValue('玻璃杯 500ml')
    await wrapper.findAll('button').find((item) => item.text().includes('生成 1 个 SKU'))?.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('同步失败 1')
    await wrapper.findAll('.result-filters button').find((item) => item.text().includes('同步失败'))?.trigger('click')
    expect(wrapper.text()).toContain('SKU-92')
    expect(wrapper.text()).not.toContain('SKU-91')

    await wrapper.findAll('button').find((item) => item.text().includes('重试失败项'))?.trigger('click')
    await flushPromises()
    expect(mocks.retryFailedERP).toHaveBeenCalledWith(9)
    expect(wrapper.text()).toContain('已提交 1 个失败项重试')

    await wrapper.findAll('button').find((item) => item.text().includes('导出已勾选'))?.trigger('click')
    await flushPromises()
    expect(mocks.exportSelection).toHaveBeenCalledWith([91, 92])
  })
})
