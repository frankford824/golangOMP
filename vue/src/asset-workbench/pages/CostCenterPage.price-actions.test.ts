// @vitest-environment jsdom
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const listDifficultyClassesAdmin = vi.hoisted(() => vi.fn())
const listPriceMatrix = vi.hoisted(() => vi.fn())
const listDeductionRules = vi.hoisted(() => vi.fn())
const listWelfareRules = vi.hoisted(() => vi.fn())
const listPromoCoupons = vi.hoisted(() => vi.fn())
const updatePriceMatrix = vi.hoisted(() => vi.fn())

vi.mock('@aw/shared/api/assetWorkbenchApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@aw/shared/api/assetWorkbenchApi')>()
  return {
    ...original,
    assetWorkbenchApi: {
      ...original.assetWorkbenchApi,
      listDifficultyClassesAdmin,
      listPriceMatrix,
      listDeductionRules,
      listWelfareRules,
      listPromoCoupons,
      updatePriceMatrix,
    },
  }
})

import CostCenterPage from './CostCenterPage.vue'

const enabledRule = {
  id: 41,
  worker_type: 'fulltime',
  job_grade: 'P1',
  difficulty_class: 'C',
  unit_price: 0.17,
  effective_from: '2026-08-19T00:00:00+08:00',
  enabled: true,
}

const disabledRule = {
  ...enabledRule,
  id: 42,
  worker_type: 'parttime',
  job_grade: 'J1',
  unit_price: 0.4,
  enabled: false,
}

const WorkbenchDataGridStub = defineComponent({
  props: { rows: { type: Array, default: () => [] } },
  setup(props, { slots }) {
    return () => h('div', { class: 'grid-stub' }, (props.rows as Array<Record<string, unknown>>).map((row) =>
      h('div', { class: 'grid-row' }, slots.cell?.({ row, column: { key: 'action' }, value: 'actions' })),
    ))
  },
})

function textButton(wrapper: ReturnType<typeof shallowMount>, text: string) {
  const found = wrapper.findAll('button').find((button) => button.text().trim() === text)
  if (!found) throw new Error(`missing button ${text}`)
  return found
}

describe('CostCenterPage price rule actions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listDifficultyClassesAdmin.mockResolvedValue([
      { id: 1, code: 'C', name: 'C', description: '', enabled: true, sort_order: 1 },
    ])
    listPriceMatrix.mockResolvedValue({ items: [enabledRule, disabledRule], total: 2, page: 1, page_size: 20 })
    listDeductionRules.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    listWelfareRules.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    listPromoCoupons.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    updatePriceMatrix.mockResolvedValue({ ...enabledRule, enabled: false })
  })

  it('shows price rules from separate API pages in one grid', async () => {
    const firstPage = Array.from({ length: 100 }, (_, index) => ({ ...enabledRule, id: index + 1 }))
    const videoRule = { ...enabledRule, id: 101, worker_type: 'parttime', job_grade: 'J1', difficulty_class: 'D', unit_price: 2 }
    listPriceMatrix.mockImplementation(async ({ page }) => ({
      items: page === 1 ? firstPage : [videoRule], total: 101, page, page_size: 100,
    }))
    const wrapper = shallowMount(CostCenterPage, { global: { stubs: { WorkbenchDataGrid: WorkbenchDataGridStub, AsyncBoundary: true } } })
    await flushPromises()
    await textButton(wrapper, '进入单价设置').trigger('click')

    expect(listPriceMatrix).toHaveBeenCalledTimes(2)
    expect(listPriceMatrix).toHaveBeenNthCalledWith(1, { page: 1, page_size: 100 }, expect.any(AbortSignal))
    expect(listPriceMatrix).toHaveBeenNthCalledWith(2, { page: 2, page_size: 100 }, expect.any(AbortSignal))
    expect(listDeductionRules).toHaveBeenCalledTimes(1)
    expect(wrapper.find('button[aria-label="停用单价规则：全职 / P1 / C"]').exists()).toBe(true)
    expect(wrapper.find('button[aria-label="停用单价规则：兼职 / J1 / D"]').exists()).toBe(true)
    expect(wrapper.find('[aria-label="单价规则分页"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('shows direct stop and enable actions and keeps confirmation for stopping an active rule', async () => {
    const wrapper = shallowMount(CostCenterPage, {
      global: {
        stubs: {
          WorkbenchDataGrid: WorkbenchDataGridStub,
          AsyncBoundary: true,
        },
      },
    })
    await flushPromises()

    await textButton(wrapper, '进入单价设置').trigger('click')

    expect(wrapper.text()).not.toContain('更多')
    expect(textButton(wrapper, '停用').attributes('aria-label')).toContain('停用单价规则：全职 / P1 / C')
    expect(textButton(wrapper, '启用').attributes('aria-label')).toContain('启用单价规则：兼职 / J1 / C')

    await textButton(wrapper, '停用').trigger('click')

    const dialog = wrapper.get('[role="dialog"][aria-labelledby="price-toggle-confirm-title"]')
    expect(dialog.text()).toContain('停用后不再参与新提交计价')
    expect(updatePriceMatrix).not.toHaveBeenCalled()

    await textButton(wrapper, '确认停用').trigger('click')
    await flushPromises()

    expect(updatePriceMatrix).toHaveBeenCalledWith(41, { enabled: false, reason: '停用单价规则' })

    await textButton(wrapper, '启用').trigger('click')
    await flushPromises()

    expect(updatePriceMatrix).toHaveBeenCalledWith(42, { enabled: true, reason: '启用单价规则' })
  })
})
