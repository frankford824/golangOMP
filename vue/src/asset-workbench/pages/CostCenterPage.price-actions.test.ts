// @vitest-environment jsdom
import { defineComponent, h } from 'vue'
import { flushPromises, mount, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const listDifficultyClassesAdmin = vi.hoisted(() => vi.fn())
const listPriceMatrix = vi.hoisted(() => vi.fn())
const listDeductionRules = vi.hoisted(() => vi.fn())
const listWelfareRules = vi.hoisted(() => vi.fn())
const listPromoCoupons = vi.hoisted(() => vi.fn())
const updatePriceMatrix = vi.hoisted(() => vi.fn())
const createPriceMatrix = vi.hoisted(() => vi.fn())
const supersedePriceMatrix = vi.hoisted(() => vi.fn())
const previewBatchReprice = vi.hoisted(() => vi.fn())
const confirmBatchReprice = vi.hoisted(() => vi.fn())

vi.mock('@aw/app/session.store', () => ({
  useAssetWorkbenchSessionStore: () => ({ bootstrap: { capabilities: ['asset.workbench.settlement'] } }),
}))

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
      createPriceMatrix,
      supersedePriceMatrix,
      previewBatchReprice,
      confirmBatchReprice,
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
      { id: 2, code: 'D', name: 'A+视频', description: '', enabled: true, sort_order: 2 },
    ])
    listPriceMatrix.mockResolvedValue({ items: [enabledRule, disabledRule], total: 2, page: 1, page_size: 20 })
    listDeductionRules.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    listWelfareRules.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    listPromoCoupons.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    updatePriceMatrix.mockResolvedValue({ ...enabledRule, enabled: false })
    createPriceMatrix.mockResolvedValue({ ...enabledRule, id: 43 })
    supersedePriceMatrix.mockResolvedValue({ ...enabledRule, id: 44 })
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

  it('copies a price draft to an editable target and confirms creation without updating the source', async () => {
    const wrapper = shallowMount(CostCenterPage, { global: { stubs: { WorkbenchDataGrid: WorkbenchDataGridStub, AsyncBoundary: true } } })
    await flushPromises()
    await textButton(wrapper, '进入单价设置').trigger('click')
    await textButton(wrapper, '修改价格').trigger('click')

    const form = wrapper.get('.aw-pricing-section .aw-form-grid')
    const from = form.findAll('input[type="date"]')[0]!
    await from.setValue('2026-11-01')
    expect(form.findAll('input[disabled]')).toHaveLength(3)

    await textButton(wrapper, '复制为新规则').trigger('click')
    expect(wrapper.text()).toContain('复制自 #41')
    expect((form.get('input[type="number"]').element as HTMLInputElement).value).toBe('0.17')
    expect((from.element as HTMLInputElement).value).toBe('2026-11-01')
    await textButton(wrapper, '预览新规则').trigger('click')
    expect(wrapper.text()).toContain('请更改人员类型、岗级或难度')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)

    const [worker, grade, difficulty] = form.findAll('select')
    await worker!.setValue('parttime')
    await grade!.setValue('J2')
    await difficulty!.setValue('D')
    await form.get('input[type="number"]').setValue('2')
    await form.findAll('input[type="date"]')[1]!.setValue('2026-11-23')
    await textButton(wrapper, '预览新规则').trigger('click')

    const dialog = wrapper.get('[role="dialog"][aria-labelledby="price-confirm-title"]')
    expect(dialog.text()).toContain('确认新增单价规则')
    expect(dialog.text()).toContain('兼职 / J2 / D')
    expect(dialog.text()).toContain('2026-11-01 至 2026-11-23')
    expect(createPriceMatrix).not.toHaveBeenCalled()
    await textButton(wrapper, '返回修改').trigger('click')
    expect((grade!.element as HTMLSelectElement).value).toBe('J2')
    await textButton(wrapper, '预览新规则').trigger('click')
    await textButton(wrapper, '确认新增').trigger('click')
    await flushPromises()

    expect(createPriceMatrix).toHaveBeenCalledExactlyOnceWith({
      worker_type: 'parttime', job_grade: 'J2', difficulty_class: 'D', unit_price: 2,
      effective_from: '2026-11-01T00:00:00+08:00', effective_to: '2026-11-23T23:59:59+08:00', remark: undefined,
    })
    expect(supersedePriceMatrix).not.toHaveBeenCalled()
    expect(updatePriceMatrix).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('publishes a same-target price change as a new version of the selected rule', async () => {
    const wrapper = shallowMount(CostCenterPage, { global: { stubs: { WorkbenchDataGrid: WorkbenchDataGridStub, AsyncBoundary: true } } })
    await flushPromises()
    await textButton(wrapper, '进入单价设置').trigger('click')
    await textButton(wrapper, '修改价格').trigger('click')

    const form = wrapper.get('.aw-pricing-section .aw-form-grid')
    await form.get('input[type="number"]').setValue('0.5')
    await form.findAll('input[type="date"]')[0]!.setValue('2026-11-25')
    await textButton(wrapper, '预览并发布').trigger('click')
    expect(wrapper.get('[role="dialog"]').text()).toContain('2026-11-24')
    await textButton(wrapper, '确认发布').trigger('click')
    await flushPromises()

    expect(supersedePriceMatrix).toHaveBeenCalledExactlyOnceWith(41, {
      worker_type: 'fulltime', job_grade: 'P1', difficulty_class: 'C', unit_price: 0.5,
      effective_from: '2026-11-25T00:00:00+08:00', effective_to: undefined, remark: undefined,
    })
    expect(createPriceMatrix).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('requires a fresh preview after changing the batch repricing scope', async () => {
    const preview = {
      rule_id: 41, business_month: '2026-11', count: 1, old_gross: 0, new_gross: 0.17, preview_token: 'first',
      items: [{ item_id: 50, payee_user_id: 42, submitted_on: '2026-11-18', order_no: 'QA-50', old_gross: 0, new_gross: 0.17 }],
    }
    previewBatchReprice.mockResolvedValueOnce(preview).mockResolvedValueOnce({ ...preview, preview_token: 'second' })
    confirmBatchReprice.mockResolvedValue({ updated: 1, old_gross: 0, new_gross: 0.17 })
    const wrapper = mount(CostCenterPage, { global: { stubs: { WorkbenchDataGrid: WorkbenchDataGridStub, AsyncBoundary: true, PersonnelPicker: true } } })
    await flushPromises()
    await textButton(wrapper, '进入单价设置').trigger('click')
    await textButton(wrapper, '批量重算').trigger('click')
    const dialog = wrapper.get('[role="dialog"][aria-labelledby="batch-reprice-title"]')
    await dialog.get('input[type="month"]').setValue('2026-11')
    await dialog.findAll('input[type="date"]')[0]!.setValue('2026-11-01')
    await textButton(wrapper, '预览重算').trigger('click')
    await flushPromises()

    expect(previewBatchReprice).toHaveBeenLastCalledWith(expect.objectContaining({
      rule_id: 41, business_month: '2026-11', submitted_from: '2026-11-01', include_priced: true,
    }), expect.any(AbortSignal))
    expect(dialog.text()).toContain('重算毛额')
    expect(dialog.text()).toContain('¥0.17')
    expect(confirmBatchReprice).not.toHaveBeenCalled()

    await dialog.findAll('input[type="date"]')[1]!.setValue('2026-11-23')
    expect(dialog.text()).not.toContain('确认重算 1 件')
    await textButton(wrapper, '预览重算').trigger('click')
    await flushPromises()
    await textButton(wrapper, '确认重算 1 件').trigger('click')
    await flushPromises()

    expect(confirmBatchReprice).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({
      rule_id: 41, submitted_from: '2026-11-01', submitted_to: '2026-11-23',
    }), 'second')
    expect(dialog.get('[role="status"]').text()).toContain('已重算 1 件作品')
    expect(dialog.text()).not.toContain('确认重算 1 件')
    await textButton(wrapper, '完成').trigger('click')
    expect(wrapper.text()).toContain('已重算 1 件作品')
    wrapper.unmount()
  })

  it('limits preview to the selected people and prevents an empty selection becoming everyone', async () => {
    previewBatchReprice.mockResolvedValueOnce({ count: 0, items: [] })
    const wrapper = mount(CostCenterPage, { global: { stubs: { WorkbenchDataGrid: WorkbenchDataGridStub, AsyncBoundary: true, PersonnelPicker: true } } })
    await flushPromises()
    await textButton(wrapper, '进入单价设置').trigger('click')
    await textButton(wrapper, '批量重算').trigger('click')
    const dialog = wrapper.get('[aria-labelledby="batch-reprice-title"]')
    await dialog.get('select').setValue('selected')
    expect(textButton(wrapper, '预览重算').attributes('disabled')).toBeDefined()
    const picker = wrapper.getComponent({ name: 'PersonnelPicker' })
    for (const id of [42, 43, 42]) picker.vm.$emit('selected', { user_id: id, name: '测试' + id, personnel_no: 'P' + id })
    await flushPromises()
    expect(dialog.get('[aria-label="已选人员"]').findAll('li')).toHaveLength(2)
    await textButton(wrapper, '预览重算').trigger('click')
    await flushPromises()
    expect(previewBatchReprice).toHaveBeenLastCalledWith(expect.objectContaining({ payee_user_ids: [42,43] }), expect.any(AbortSignal))
    expect(dialog.text()).toContain('0 件需要重算')
    await dialog.get('[aria-label="移除测试42"]').trigger('click')
    expect(dialog.text()).not.toContain('预览完成')
    await dialog.get('[aria-label="移除测试43"]').trigger('click')
    expect(textButton(wrapper, '预览重算').attributes('disabled')).toBeDefined()
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
