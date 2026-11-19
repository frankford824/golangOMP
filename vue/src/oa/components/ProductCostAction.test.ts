// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import ProductCostAction from './ProductCostAction.vue'

const mocks = vi.hoisted(() => ({
  canView: true, canMaintain: true,
  list: vi.fn(), maintain: vi.fn(), send: vi.fn(),
}))
vi.mock('@/services/http', () => ({ getToken: () => 'test-session' }))
vi.mock('@/composables/usePermission', () => ({
  usePermission: () => ({ can: (action: string) => action === 'catalog.view' && mocks.canView }),
}))
vi.mock('../access', () => ({
  useOAAccess: () => ({ can: (action: string) => action === 'cost.request.handle' && mocks.canMaintain, currentProfile: { value: { id: '7' } } }),
}))
vi.mock('../api', async original => ({ ...await original<typeof import('../api')>(), send: mocks.send }))
vi.mock('@/services/api/costMaintenanceApi', () => ({
  costMaintenanceApi: { list: mocks.list, maintain: mocks.maintain },
  maintenanceStatuses: { pending: '系统已生效 · 待同步' },
}))

const row = {
  id: 23, sku_code: 'ABC001', cost_price: 7.0123, erp_cost: null, revision: 3,
  status: 'pending', reviewed: false, token: 'cost-token',
}
const target = {
  sku: 'ABC001', shop: 'shop-1', shopName: '一号店', productName: '纸盒', quantity: '12',
  sales: '41.25', coverage: '81.4%', from: '2026-11-19', to: '2026-11-19', today: true, includeBrush: false,
}

beforeEach(() => {
  vi.resetAllMocks()
  mocks.canView = true
  mocks.canMaintain = true
  vi.stubGlobal('matchMedia', () => ({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn() }))
  mocks.list.mockResolvedValue({ data: [row], next_before_id: 0 })
})
afterEach(() => vi.unstubAllGlobals())

const open = async () => {
  const wrapper = mount(ProductCostAction, { props: { split: true }, global: { stubs: ['RouterLink'] } })
  wrapper.vm.open(target)
  await flushPromises()
  return wrapper
}

it('paginates past a similar SKU and confirms the exact canonical row with its observed ERP baseline', async () => {
  mocks.list.mockResolvedValueOnce({ data: [{ ...row, id: 42, sku_code: 'ABC001-2' }], next_before_id: 42 })
  mocks.maintain.mockResolvedValue({ ...row, reviewed: true })
  const wrapper = await open()
  expect(mocks.list.mock.calls[1]?.[0]).toMatchObject({ before_id: 42, keyword: 'ABC001', review_state: 'all' })
  expect(wrapper.get('[aria-label="商品经营摘要"]').text()).toContain('纸盒')
  expect(wrapper.get('[aria-label="商品经营摘要"]').text()).toContain('今日销量12')
  expect(wrapper.get('[aria-label="商品经营摘要"]').text()).toContain('今日销售额¥41.25')
  expect(wrapper.get('[aria-label="商品经营摘要"]').text()).toContain('成交成本核实覆盖率81.4%')
  expect(wrapper.get('[aria-label="当前编码成本"]').text()).toContain('系统单位成本¥7.0123ERP 最近观测单位成本—')
  await wrapper.findAll('button').find(button => button.text() === '确认当前系统成本')!.trigger('click')
  await flushPromises()
  expect(mocks.maintain).toHaveBeenCalledWith(23, { action: 'confirm', token: 'cost-token', revision: 3, erp_cost: null })
  expect(wrapper.text()).toContain('系统成本已确认')
  expect(mocks.send).not.toHaveBeenCalled()
  wrapper.unmount()
})

it('submits price and reason through native maintenance and requires a fresh baseline after conflict', async () => {
  mocks.list.mockResolvedValue({ data: [{ ...row, cost_price: null, erp_cost: 0 }], next_before_id: 0 })
  mocks.maintain.mockRejectedValue(Object.assign(new Error('ERP 观测成本已变化'), { status: 409 }))
  const wrapper = await open()
  expect(wrapper.get('[aria-label="当前编码成本"]').text()).toContain('系统单位成本—ERP 最近观测单位成本¥0')
  await wrapper.findAll('button').find(button => button.text() === '修改成本')!.trigger('click')
  await wrapper.get('input[type=number]').setValue('2.3456')
  await wrapper.get('textarea').setValue('核对最新材质')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(mocks.maintain).toHaveBeenCalledWith(23, {
    action: 'modify', token: 'cost-token', revision: 3, erp_cost: 0, cost_price: 2.3456,
    reason_code: 'cost_incorrect', reason_detail: '核对最新材质',
  })
  expect(wrapper.get('[role=alert]').text()).toBe('ERP 观测成本已变化')
  expect(wrapper.find('form').exists()).toBe(false)
  await wrapper.findAll('button').find(button => button.text() === '刷新当前成本')!.trigger('click')
  await flushPromises()
  expect(mocks.list).toHaveBeenCalledTimes(2)
  wrapper.unmount()
})

it('keeps cost hidden without read permission and sends a review request using the selected commerce context', async () => {
  mocks.canView = false
  mocks.canMaintain = false
  mocks.send.mockResolvedValue({ existing: false })
  const wrapper = await open()
  expect(mocks.list).not.toHaveBeenCalled()
  expect(wrapper.text()).toContain('当前账号无编码成本查看权限')
  await wrapper.get('textarea').setValue('核对采购成本')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(mocks.send).toHaveBeenCalledWith('/oa-api/orders/cost-requests', {
    sku: 'ABC001', shop_id: 'shop-1', from: '2026-11-19', to: '2026-11-19',
    include_brush: false, reason: '核对采购成本', request_key: expect.any(String),
  })
  expect(mocks.maintain).not.toHaveBeenCalled()
  wrapper.unmount()
})

it('keeps both costs unknown and offers review when the SKU has no maintenance record', async () => {
  mocks.list.mockResolvedValue({ data: [], next_before_id: 0 })
  const wrapper = await open()
  try {
    expect(wrapper.get('[aria-label="当前编码成本"]').text()).toContain('系统单位成本—ERP 最近观测单位成本—')
    expect(wrapper.findAll('button').some(button => button.text() === '确认当前系统成本')).toBe(false)
    expect(wrapper.get('textarea').exists()).toBe(true)
  } finally {
    wrapper.unmount()
  }
})
