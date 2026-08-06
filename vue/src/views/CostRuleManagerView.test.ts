// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  listCostRules: vi.fn(),
  createCostRule: vi.fn(),
  updateCostRule: vi.fn(),
  listBindings: vi.fn(),
  listCandidates: vi.fn(),
  createBinding: vi.fn(),
  listRuns: vi.fn(),
  getRun: vi.fn(),
  createRun: vi.fn(),
  applyRun: vi.fn(),
  syncRun: vi.fn(),
  preview: vi.fn(),
  dashboard: vi.fn(),
}))

vi.mock('@/services/api/categoriesApi', () => ({
  categoriesApi: {
    listCostRules: mocks.listCostRules,
    createCostRule: mocks.createCostRule,
    updateCostRule: mocks.updateCostRule,
  },
}))
vi.mock('@/services/api/costManagementApi', () => ({
  costManagementApi: {
    listCostRuleBindings: mocks.listBindings,
    listUnboundCostRuleCandidates: mocks.listCandidates,
    createCostRuleBinding: mocks.createBinding,
    listCostRecalculationRuns: mocks.listRuns,
    getCostRecalculationRun: mocks.getRun,
    getCostDashboard: mocks.dashboard,
    createCostRecalculationRun: mocks.createRun,
    applyCostRecalculationRun: mocks.applyRun,
    syncCostRecalculationRunERP: mocks.syncRun,
    previewCostRule: mocks.preview,
  },
}))

import CostRuleManagerView from './CostRuleManagerView.vue'

const previewRun = {
  id: 7,
  run_no: 'CR-007',
  status: 'previewed',
  mode: 'all_matching',
  created_at: '2026-07-18T08:00:00Z',
  summary: { total_count: 3, applied_count: 0, erp_synced_count: 0, conflict_count: 0 },
}

describe('CostRuleManagerView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listCostRules.mockResolvedValue({ data: { data: [{ rule_id: 11, rule_name: 'KT 板基础单价', category_code: 'KT_BOARD', product_family: 'KT 板', rule_type: 'fixed_unit_price', base_price: 12.5, priority: 100, is_active: true }] } })
    mocks.listBindings.mockResolvedValue({ data: [{ id: 21, i_id_raw: 'STYLE-01', normalized_i_id: 'STYLE-01', rule_group: 'KT_BOARD', display_name: '标准 KT 板', is_active: true }] })
    mocks.listCandidates.mockResolvedValue({ data: [] })
    mocks.listRuns.mockResolvedValue({ data: [previewRun] })
    mocks.dashboard.mockResolvedValue({ total_count: 12, total_records: 12, groups: [], tags: [{ code: 'erp_mismatch', label: 'ERP 差异', count: 2 }] })
    mocks.preview.mockResolvedValue({ estimated_cost: 25, explanation: '按两平方米固定单价计算。' })
    mocks.getRun.mockResolvedValue({ ...previewRun, items: [{ id: 1, status: 'previewed', sku_code: 'SKU-A', task_no: 'RW-01', old_cost_price: 20, new_cost_price: 25 }] })
    mocks.applyRun.mockResolvedValue({ run: { ...previewRun, status: 'applied', summary: { total_count: 3, applied_count: 3, erp_synced_count: 0 } } })
    mocks.syncRun.mockResolvedValue({ run: { ...previewRun, status: 'erp_syncing' } })
    mocks.createRun.mockResolvedValue({ id: 8, run_no: 'CR-008', status: 'previewed' })
    mocks.updateCostRule.mockResolvedValue({})
  })

  it('shows rule bindings, ERP differences, and previews cost without writing data', async () => {
    const wrapper = mount(CostRuleManagerView, { attachTo: document.body })
    await flushPromises()

    expect(wrapper.text()).toContain('KT 板')
    expect(wrapper.text()).toContain('STYLE-01')
    expect(wrapper.text()).toContain('2 个 ERP 差异')
    await wrapper.get('.calculate-button').trigger('click')
    await flushPromises()
    expect(mocks.preview).toHaveBeenCalledWith(expect.objectContaining({ rule_group: 'KT_BOARD', quantity: 1 }))
    expect(wrapper.text()).toContain('¥ 25.00')
    expect(mocks.applyRun).not.toHaveBeenCalled()
  })

  it('keeps the calculator aligned with the selected rule group', async () => {
    mocks.listCostRules.mockResolvedValue({
      data: {
        data: [
          { rule_id: 11, rule_name: 'KT 板基础单价', category_code: 'KT_BOARD', product_family: 'KT 板', rule_type: 'fixed_unit_price', base_price: 12.5, priority: 100, is_active: true },
          { rule_id: 12, rule_name: '写真布基础单价', category_code: 'PHOTO_CLOTH', product_family: '写真布', rule_type: 'fixed_unit_price', base_price: 8, priority: 100, is_active: true },
        ],
      },
    })
    const wrapper = mount(CostRuleManagerView, { attachTo: document.body })
    await flushPromises()

    await wrapper.findAll('.rule-groups > button').find((button) => button.text().includes('写真布'))?.trigger('click')
    await flushPromises()

    expect((wrapper.get('.calculator select').element as HTMLSelectElement).value).toBe('PHOTO_CLOTH')
  })

  it('requires an explicit update before ERP synchronization', async () => {
    const wrapper = mount(CostRuleManagerView, { attachTo: document.body })
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text() === '查看影响')?.trigger('click')
    await flushPromises()
    expect(document.body.textContent).toContain('SKU-A')
    expect(document.body.textContent).toContain('¥20.00 → ¥25.00')

    await wrapper.findAll('button').find((button) => button.text() === '确认更新')?.trigger('click')
    await flushPromises()
    expect(mocks.applyRun).toHaveBeenCalledWith(7)
    expect(mocks.syncRun).not.toHaveBeenCalled()
  })

  it('saves a rule and creates only an impact preview', async () => {
    const wrapper = mount(CostRuleManagerView, { attachTo: document.body })
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text() === '编辑')?.trigger('click')
    await flushPromises()
    const form = document.body.querySelector('form')
    expect(form).not.toBeNull()
    form?.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()

    expect(mocks.updateCostRule).toHaveBeenCalledWith(11, expect.objectContaining({ category_code: 'KT_BOARD' }))
    expect(mocks.createRun).toHaveBeenCalledWith(expect.objectContaining({ mode: 'all_matching', filters: { rule_group: 'KT_BOARD' } }))
    expect(mocks.applyRun).not.toHaveBeenCalled()
    expect(mocks.syncRun).not.toHaveBeenCalled()
  })
})
