// @vitest-environment jsdom
import { flushPromises, mount,enableAutoUnmount } from '@vue/test-utils'
import { beforeEach,afterEach, describe, expect, it, vi } from 'vitest'
enableAutoUnmount(afterEach)

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
  cancelRun: vi.fn(),
  preview: vi.fn(),
  dashboard: vi.fn(),
  boundSKUs:vi.fn(),updateBinding:vi.fn(),erp:vi.fn(),
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
    listBoundSKUs:mocks.boundSKUs,updateCostRuleBinding:mocks.updateBinding,
    listCostRuleBindings: mocks.listBindings,
    listUnboundCostRuleCandidates: mocks.listCandidates,
    createCostRuleBinding: mocks.createBinding,
    listCostRecalculationRuns: mocks.listRuns,
    getCostRecalculationRun: mocks.getRun,
    getCostDashboard: mocks.dashboard,
    createCostRecalculationRun: mocks.createRun,
    applyCostRecalculationRun: mocks.applyRun,
    syncCostRecalculationRunERP: mocks.syncRun,
    cancelCostRecalculationRun: mocks.cancelRun,
    previewCostRule: mocks.preview,
  },
}))
vi.mock('@/services/api/erpApi',()=>({erpApi:{getIids:mocks.erp}}))

import CostRuleManagerView from './CostRuleManagerView.vue'
import {emptyCostInput,emptyCostModel} from '@/domain/cost-model'

const previewRun = {
  id: 7,
  run_no: 'CR-007',
  status: 'previewed',
  mode: 'all_matching',
  created_at: '2026-07-18T08:00:00Z',
  summary: { total_count: 3, applied_count: 0, erp_synced_count: 0, conflict_count: 0 },
}

describe('CostRuleManagerView', () => {
  it('renders print prices as two editable business fields without exposing internal configuration',async()=>{
    mocks.listCostRules.mockResolvedValue({data:{data:[{rule_id:11,rule_name:'A4打印',category_code:'KT_BOARD',rule_type:'size_based_formula',formula_expression:'print_side:single=0.3,double=0.4',priority:10,is_active:true,source:'phase_020_sample'}]}})
    const wrapper=mount(CostRuleManagerView,{attachTo:document.body});await flushPromises()
    expect(wrapper.text()).toContain('单面 0.3 元/张，双面 0.4 元/张')
    expect(wrapper.text()).not.toContain('print_side:')
    await wrapper.findAll('button').find(b=>b.text()==='编辑')!.trigger('click');await flushPromises()
    const form=document.querySelector('form.modal-card')!
    expect(form.textContent).toContain('单面价格（元/张）');expect(form.textContent).not.toMatch(/优先级|面积阈值|含税倍率|替代规则 ID|尺寸公式|旧参数/)
    form.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));await flushPromises()
    expect(mocks.updateCostRule).toHaveBeenCalledWith(11,expect.objectContaining({formula_expression:'print_side:single=0.3,double=0.4',priority:10,source:'phase_020_sample'}))
  })
  it('previews an unsaved unified model with structured input without saving it', async () => {
    mocks.listCostRules.mockResolvedValue({data:{data:[{rule_id:11,rule_name:'KT方案',category_code:'KT_BOARD',rule_type:'cost_model',is_active:true,formula_expression:JSON.stringify({...emptyCostModel(),material:'KT',unit_price:12.5})}]}})
    const wrapper=mount(CostRuleManagerView)
    await flushPromises()
    await wrapper.findAll('button').find(b=>b.text()==='编辑方案价格')?.trigger('click')
    await flushPromises()
    // The editor seeds the current rate; no write happens until explicit save.
    expect(wrapper.find('[aria-label="统一计价方案"]').exists()).toBe(true)
    await wrapper.get('.calculator-toggle').trigger('click')
    await wrapper.get('.calculate-button').trigger('click')
    await flushPromises()
    expect(mocks.preview).toHaveBeenCalledWith(expect.objectContaining({model:expect.objectContaining({basis:'area',unit_price:12.5}),input:emptyCostInput()}))
    expect(mocks.createCostRule).not.toHaveBeenCalled()
    expect(mocks.updateCostRule).not.toHaveBeenCalled()
  })
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
    mocks.cancelRun.mockResolvedValue({ ...previewRun, status: 'cancelled' })
    mocks.createRun.mockResolvedValue({ id: 8, run_no: 'CR-008', status: 'previewed' })
    mocks.createCostRule.mockResolvedValue({})
    mocks.updateCostRule.mockResolvedValue({})
    mocks.boundSKUs.mockResolvedValue({data:[{id:1,sku_code:'CGK001',product_name:'测试产品',style_code:'STYLE-01',cost_price:1.2,status:'generated'}],pagination:{total:1,page:1,page_size:20}})
    mocks.erp.mockResolvedValue({data:{data:[]}})
  })

  it('shows rule bindings, ERP differences, and previews cost without writing data', async () => {
    const wrapper = mount(CostRuleManagerView, { attachTo: document.body })
    await flushPromises()

    expect(wrapper.text()).toContain('KT 板')
    expect(wrapper.text()).toContain('STYLE-01')
    expect(wrapper.text()).toContain('2 个 ERP 差异')
    expect(wrapper.find('.calculator').exists()).toBe(false)
    expect(wrapper.text()).toContain('CGK001')
    await wrapper.get('.calculator-toggle').trigger('click')
    await wrapper.get('.calculate-button').trigger('click')
    await flushPromises()
    expect(mocks.preview).toHaveBeenCalledWith(expect.objectContaining({ rule_group: 'KT_BOARD', quantity: 1 }))
    expect(wrapper.text()).toContain('¥ 25.00')
    expect(mocks.applyRun).not.toHaveBeenCalled()
    await wrapper.get('.calculator-toggle').trigger('click')
    expect(wrapper.find('.calculator').exists()).toBe(false)
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

    await wrapper.get('.group-search select').setValue('all')
    await wrapper.findAll('.rule-groups > button').find((button) => button.text().includes('写真布'))?.trigger('click')
    await flushPromises()

    await wrapper.get('.calculator-toggle').trigger('click')
    expect((wrapper.get('.calculator select').element as HTMLSelectElement).value).toBe('PHOTO_CLOTH')
  })

  it('exposes conflicting historical rule evidence instead of guessing a binding', async () => {
    mocks.listCandidates.mockResolvedValue({ data: [{
      normalized_i_id: 'STYLE-CONFLICT',
      erp_i_id: 'STYLE-CONFLICT',
      suggested_rule_groups: ['KT_BOARD', 'PHOTO_CLOTH'],
      suggested_group_count: 2,
      mapping_confidence: 'conflict',
      match_count: 18,
      example_sku_code: 'SKU-18',
    }] })
    const wrapper = mount(CostRuleManagerView, { attachTo: document.body })
    await flushPromises()

    expect(wrapper.text()).toContain('需要选择方案')
    expect(wrapper.text()).toContain('1')
    await wrapper.findAll('button').find((button) => button.text() === '处理未绑定款式')?.trigger('click')
    await flushPromises()
    expect(document.body.textContent).toContain('历史使用过多个方案')
    expect(document.body.textContent).toContain('确认绑定此组')
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

  it('allows an obsolete preview to be cancelled before creating a replacement', async () => {
    const wrapper = mount(CostRuleManagerView, { attachTo: document.body })
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text() === '取消预览')?.trigger('click')
    await flushPromises()

    expect(mocks.cancelRun).toHaveBeenCalledWith(7)
    expect(wrapper.text()).toContain('本次成本影响预览已取消')
  })

  it('saves a rule and creates only an impact preview', async () => {
    const wrapper = mount(CostRuleManagerView, { attachTo: document.body })
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text() === '编辑')?.trigger('click')
    await flushPromises()
    const form = document.body.querySelector('form.modal-card')
    expect(form).not.toBeNull()
    form?.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()

    expect(mocks.updateCostRule).toHaveBeenCalledWith(11, expect.objectContaining({ category_code: 'KT_BOARD' }))
    expect(mocks.createRun).toHaveBeenCalledWith(expect.objectContaining({ mode: 'all_matching', filters: { rule_group: 'KT_BOARD' } }))
    expect(mocks.applyRun).not.toHaveBeenCalled()
    expect(mocks.syncRun).not.toHaveBeenCalled()
  })

  it('creates an exact impact preview from normalized SKU codes', async () => {
    const wrapper = mount(CostRuleManagerView, { attachTo: document.body })
    await flushPromises()
    await wrapper.get('.exact-run-builder textarea').setValue('DZA000036, DZA000037\nDZA000036')
    await wrapper.get('.exact-run-builder button').trigger('click')
    await flushPromises()

    expect(mocks.createRun).toHaveBeenCalledWith({
      mode: 'explicit',
      sku_codes: ['DZA000036', 'DZA000037'],
      reason: '指定 SKU 成本修复预览',
    })
  })

  it('preserves governed formula and supersession fields when editing a rule', async () => {
    mocks.listBindings.mockResolvedValue({data:[{id:2,i_id_raw:'AC',normalized_i_id:'AC',rule_group:'ACRYLIC',is_active:true}]})
    mocks.listCostRules.mockResolvedValue({ data: { data: [{
      rule_id: 26,
      rule_name: '教师节亚克力面积成本',
      category_code: 'ACRYLIC',
      product_family: 'material',
      rule_type: 'size_based_formula',
      formula_expression: 'keyword_area_unit_price:教师节=264',
      supersedes_rule_id: 25,
      priority: 10,
      is_active: true,
    }] } })
    const wrapper = mount(CostRuleManagerView, { attachTo: document.body })
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text() === '编辑')?.trigger('click')
    await flushPromises()
    expect(document.body.textContent).toContain('适用产品关键词')
    expect(document.body.textContent).not.toContain('keyword_area_unit_price:')
    expect(document.body.textContent).not.toContain('优先级')
    document.body.querySelector('form.modal-card')?.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()

    expect(mocks.updateCostRule).toHaveBeenCalledWith(26, expect.objectContaining({
      formula_expression: 'keyword_area_unit_price:教师节=264',
      supersedes_rule_id: 25,
    }))
  })
})
