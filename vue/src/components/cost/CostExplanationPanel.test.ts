// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ getRule: vi.fn(), preview: vi.fn(), record: vi.fn() }))
vi.mock('@/services/api/costManagementApi', async (loadOriginal) => ({
  ...(await loadOriginal<typeof import('@/services/api/costManagementApi')>()),
  costManagementApi: { getCostRule: mocks.getRule, previewCostRule: mocks.preview },
}))
vi.mock('@/services/api/workflowTelemetryApi', () => ({
  workflowTelemetryApi: { recordEvent: mocks.record },
}))

import CostExplanationPanel from './CostExplanationPanel.vue'

describe('CostExplanationPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.getRule.mockResolvedValue({ id: 17, category_code: 'KT_STANDARD' })
    mocks.preview.mockResolvedValue({
      matched_rule_id: 17,
      matched_rule_version: 3,
      estimated_cost: 14.2,
      rule_source: 'governed',
      rule_group: 'cup',
      match_mode: 'binding_product_i_id',
      requires_manual_review: false,
      explanation: '按面积和数量计算。',
    })
    mocks.record.mockResolvedValue({})
  })

  it('previews without mutating business data and records explicit user feedback', async () => {
    const wrapper = mount(CostExplanationPanel, {
      props: {
        open: true,
        title: 'SKU-1 成本规则试算与解释',
        taskId: 8,
        taskSkuItemId: 18,
        skuCode: 'SKU-1',
        resourceId: '18',
        seed: {
          categoryCode: 'cup',
          productIID: 'ERP-1',
          width: 0.2,
          height: 0.3,
          area: 0.06,
          quantity: 2,
          currentCost: 14.2,
          currentRuleName: '杯具规则',
          currentRuleVersion: 3,
        },
      },
    })

    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.preview).toHaveBeenCalledWith(expect.objectContaining({
      category_code: 'cup',
      product_i_id: 'ERP-1',
      width: 0.2,
      quantity: 2,
    }))
    expect(wrapper.text()).toContain('商品 i_id 精确绑定')
    expect(wrapper.text()).toContain('结果与当前记录未发现明显冲突')

    await wrapper.findAll('button').find((button) => button.text() === '符合预期')?.trigger('click')
    await flushPromises()
    expect(mocks.record).toHaveBeenCalledWith(expect.objectContaining({
      action: 'cost.preview.feedback',
      task_id: 8,
      task_sku_item_id: 18,
      outcome: 'expected',
    }))
    expect(wrapper.text()).toContain('规则与输入仍保持不变')
  })

  it('requires a note before recording a disputed result', async () => {
    const wrapper = mount(CostExplanationPanel, {
      props: { open: true, title: '成本解释', seed: { categoryCode: 'cup' } },
    })
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    const disputed = wrapper.findAll('button').find((button) => button.text() === '结果有疑问')
    expect(disputed?.attributes('disabled')).toBeDefined()
    await wrapper.get('.feedback textarea').setValue('预期成本应为 12 元，请核对类目。')
    await disputed?.trigger('click')
    await flushPromises()
    expect(mocks.record).toHaveBeenCalledWith(expect.objectContaining({
      outcome: 'needs_review',
      payload: expect.objectContaining({ feedback_note: '预期成本应为 12 元，请核对类目。' }),
    }))
  })

  it('explains why a historical cost snapshot cannot be recalculated', async () => {
    const wrapper = mount(CostExplanationPanel, {
      props: {
        open: true,
        title: '历史成本解释',
        seed: {
          currentCost: 9.32,
          currentRuleName: '常规KT板基础单价',
          currentRuleVersion: 1,
        },
      },
    })

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(mocks.preview).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toBe(
      '当前规则快照缺少可复算的规则分组 / 类目编码，请先补充后再试算。',
    )
  })

  it('uses the persisted rule id to recover the governed group from historical display text', async () => {
    const wrapper = mount(CostExplanationPanel, {
      props: {
        open: true,
        title: '历史成本解释',
        seed: {
          categoryCode: '常规kt板',
          currentRuleId: 17,
          currentCost: 9.32,
          currentRuleName: '常规KT板基础单价',
          width: 55,
          height: 140,
          area: 0.77,
        },
      },
    })

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(mocks.getRule).toHaveBeenCalledWith(17)
    expect(mocks.preview).toHaveBeenCalledWith(expect.objectContaining({
      category_code: 'KT_STANDARD',
      area: 0.77,
    }))
  })
})
