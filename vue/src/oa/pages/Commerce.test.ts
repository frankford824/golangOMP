// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import Commerce from './Commerce.vue'

const { orderAPI, routeQuery, session } = vi.hoisted(() => ({ orderAPI: vi.fn(), routeQuery: {} as Record<string, string>, session: { number: 0 } }))
vi.mock('vue-router', () => ({
  RouterLink: { template: '<a><slot /></a>' },
  useRoute: () => ({ path: '/operations', fullPath: '/operations', query: routeQuery }),
  useRouter: () => ({ resolve: ({ path }: { path: string }) => ({ fullPath: path }) }),
}))
vi.mock('@/services/http', () => ({ getToken: () => 'test-session-' + session.number }))
vi.mock('@/composables/usePermission', () => ({ usePermission: () => ({ can: () => false }) }))
vi.mock('@/stores/permissions', () => ({ usePermissionsStore: () => ({ hasMenu: () => false }) }))
vi.mock('../access', () => ({
  useOAAccess: () => ({ can: (action: string) => action === 'orders.view', currentProfile: { value: { id: '7' } } }),
}))
vi.mock('../api', async (importOriginal) => ({
  ...await importOriginal<typeof import('../api')>(),
  api: orderAPI,
  dateInChina: () => '2026-11-18',
}))

beforeEach(() => {
  session.number++
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-11-18T08:30:00Z'))
})
afterEach(() => vi.useRealTimers())

it('sorts all available ranking rows precisely and keeps equal amounts in source order with missing values last', async () => {
  orderAPI.mockImplementation(async (path: string) => {
    if (path.includes('/rankings?')) return { rows: [
      { key: 'large', payable_amount: '9007199254740992.02' },
      { key: 'empty' },
      { key: 'smaller', payable_amount: '9007199254740992.01' },
      { key: 'equal', payable_amount: '9007199254740992.010' },
    ] }
    if (path.includes('/summary?')) return { rows: [
      { shop_id: 'large', order_count: 1, verified_gross: '1.01', item_pay_verified_total: '10' },
      { shop_id: 'smaller', order_count: 99, verified_gross: '1.02', item_pay_verified_total: '50' },
    ] }
    return { rows: [] }
  })
  const wrapper = mount(Commerce, { props: { mode: 'ranking' } })
  await flushPromises()
  const ranks = () => wrapper.findAll('tbody tr').map(row => row.findAll('td').slice(0, 2).map(cell => cell.text()))
  expect(wrapper.text()).toContain('所选范围全量排序')
  await wrapper.find('.commerce-ranking-sort label:nth-child(2) select').setValue('asc')
  expect(ranks()).toEqual([['1', 'smaller'], ['2', 'equal'], ['3', 'large'], ['4', 'empty']])
  await wrapper.find('.commerce-ranking-sort label:nth-child(1) select').setValue('grossRate')
  expect(ranks()).toEqual([['1', 'smaller'], ['2', 'large'], ['3', 'empty'], ['4', 'equal']])
  wrapper.unmount()
})

it('keeps paged product ranks, joins names by shop and ID, and looks up a missing label on demand', async () => {
  orderAPI.mockClear()
  orderAPI.mockImplementation(async (path: string) => {
    if (path.includes('/rankings?')) return { has_more: true, rows: [
      { key: 'same-id', shop_id: 's1', item_pay_total: '50' },
      { key: 'same-id', shop_id: 's2', item_pay_total: '20' },
    ] }
    if (path.includes('/products?')) {
      const query = new URL(path, 'https://oa.test').searchParams
      return { rows: query.has('product')
        ? [{ shop_id: 's2', shop_item_id: 'same-id', product_name: '第二家商品' }]
        : [
          { shop_id: 's1', shop_item_id: 'same-id', product_name: '第一家商品' },
          { shop_id: 's1', shop_item_id: 'same-id', product_name: '第一家另一商品名' },
        ] }
    }
    return { rows: [] }
  })
  const wrapper = mount(Commerce, { props: { mode: 'ranking' } })
  await flushPromises()
  await wrapper.find('form select').setValue('product')
  await wrapper.find('form').trigger('submit')
  await flushPromises()
  expect(wrapper.text()).toContain('仅排序本页，保留原榜名次')
  expect(wrapper.text()).toContain('第一家商品')
  expect(wrapper.text()).toContain('SKU 明细有多个名称')
  expect(wrapper.text()).toContain('商品名称未返回')
  await wrapper.find('.commerce-ranking-sort label:nth-child(2) select').setValue('asc')
  expect(wrapper.findAll('tbody .commerce-rank').map(cell => cell.text())).toEqual(['2', '1'])
  const productRequests = () => orderAPI.mock.calls.map(([path]) => String(path)).filter(path => path.includes('/products?'))
  expect(productRequests()).toHaveLength(1)
  await wrapper.find('tbody button').trigger('click')
  await flushPromises()
  expect(wrapper.text()).toContain('第二家商品')
  expect(productRequests()).toHaveLength(2)
  expect(new URL(productRequests()[1], 'https://oa.test').searchParams.get('shop_id')).toBe('s2')
  expect(new URL(productRequests()[1], 'https://oa.test').searchParams.get('product')).toBe('same-id')
  wrapper.unmount()
})

it('loads shop-product-SKU rows with the applied filters and remembers the SKU sort on return', async () => {
  const priorState = window.history.state
  window.history.replaceState({ position: 1 }, '')
  Object.assign(routeQuery, { group: 'sku', shop_id: 's1', from: '2026-11-17', to: '2026-11-18', include_brush: '1' })
  orderAPI.mockClear()
  orderAPI.mockImplementation(async (path: string) => path.includes('/products?') ? { has_more: path.endsWith('offset=0'), rows: [
    { shop_id: 's1', shop_item_id: 'p1', sku: 'same-sku', product_name: '商品一', quantity: '18', item_pay_total: '99' },
    { shop_id: 's1', shop_item_id: 'p2', sku: 'same-sku', product_name: '商品二', quantity: '2', item_pay_total: '9' },
  ] } : { rows: [] })
  const wrapper = mount(Commerce, { props: { mode: 'ranking' } })
  await flushPromises()
  const request = orderAPI.mock.calls.map(([path]) => String(path)).find(path => path.includes('/products?'))
  expect(request).toBeDefined()
  expect(Object.fromEntries(new URL(request!, 'https://oa.test').searchParams)).toEqual({
    from: '2026-11-17', to: '2026-11-18', shop_id: 's1', include_brush: '1', offset: '0',
  })
  await wrapper.find('.commerce-ranking-sort label:nth-child(1) select').setValue('quantity')
  await wrapper.find('.commerce-ranking-sort label:nth-child(2) select').setValue('asc')
  expect(wrapper.findAll('tbody tr').map(row => row.find('strong, a').text())).toEqual(['商品二', '商品一'])
  await wrapper.find('.workspace-pager button:last-child').trigger('click')
  await flushPromises()
  expect(wrapper.findAll('tbody .commerce-rank').map(cell => cell.text())).toEqual(['102', '101'])
  wrapper.unmount()

  const returned = mount(Commerce, { props: { mode: 'ranking' } })
  await flushPromises()
  expect((returned.find('.commerce-ranking-sort label:nth-child(1) select').element as HTMLSelectElement).value).toBe('quantity')
  expect((returned.find('.commerce-ranking-sort label:nth-child(2) select').element as HTMLSelectElement).value).toBe('asc')
  expect(returned.find('.workspace-pager').text()).toContain('第 2 页')
  returned.unmount()
  for (const key of Object.keys(routeQuery)) delete routeQuery[key]
  window.history.replaceState(priorState, '')
})

it('keeps the overview usable with null freshness and renders a scoped Top 10 and hourly comparison', async () => {
  orderAPI.mockImplementation(async (path: string) => {
    if (path.includes('/products?')) return {
      freshness: null,
      rows: Array.from({ length: 12 }, (_, index) => ({ product_name: '商品 ' + index, sku: 'SKU' + index })),
    }
    if (path.includes('/hourly?')) return {
      rows: [
        { day: '2026-11-17', hour: 4, payable_amount: '400' },
        { day: '2026-11-17', hour: 5, payable_amount: '500' },
        { day: '2026-11-18', hour: 4, payable_amount: '600' },
      ],
    }
    return { rows: [], freshness: null }
  })

  const wrapper = mount(Commerce, { props: { mode: 'overview' } })
  await flushPromises()
  expect(wrapper.get('h1').text()).toBe('经营总览')
  expect(wrapper.findAll('.commerce-results tbody tr')).toHaveLength(10)
  expect(wrapper.get('.commerce-chart').attributes('aria-label')).toContain('分时订单应付')
  expect(wrapper.findAll('.commerce-chart circle').map(point => point.attributes('aria-label'))).toEqual([
    '2026-11-18 04:00 ¥600.00', '2026-11-17 04:00 ¥400.00',
  ])
  expect(wrapper.get('.commerce-comparison').text()).toContain('+50.0%')
  const chart = wrapper.get('.commerce-chart')
  await chart.trigger('focus')
  expect(wrapper.get('.commerce-chart-tooltip').text()).toContain('所选日 ¥600.00')
  expect(wrapper.get('.commerce-chart-tooltip').text()).toContain('前一日 ¥400.00')
  await chart.trigger('keydown', { key: 'ArrowLeft' })
  expect(wrapper.get('.commerce-chart-tooltip').text()).toContain('03:00')
  await chart.trigger('keydown', { key: 'Escape' })
  expect(wrapper.find('.commerce-chart-tooltip').exists()).toBe(false)
  expect(orderAPI).toHaveBeenCalledWith('/oa-api/orders/hourly?from=2026-11-17&to=2026-11-18')
  wrapper.unmount()
})

it('falls back to a daily trend without changing the selected day', async () => {
  orderAPI.mockImplementation(async (path: string) => {
    if (path.includes('/hourly?')) throw new Error('unavailable')
    return { rows: [{ day: '2026-11-18', payable_amount: '25' }], freshness: null }
  })
  const wrapper = mount(Commerce, { props: { mode: 'overview' } })
  await flushPromises()
  expect(wrapper.get('.commerce-chart').attributes('aria-label')).toBe('近 7 日订单应付')
  expect(wrapper.findAll('input[type=date]').map(input => (input.element as HTMLInputElement).value)).toEqual(['2026-11-18', '2026-11-18'])
  wrapper.unmount()
})


it.each([
  { hours: [3, 4], baseline: [3], stale: false, basis: 'one date has an unmatched completed hour' },
  { hours: [16], baseline: [16], stale: false, basis: 'only the current incomplete hour is available' },
  { hours: [3, 4], baseline: [3, 4], stale: true, basis: 'the hourly snapshot is stale' },
])('hides the change when $basis', async ({ hours, baseline, stale }) => {
  orderAPI.mockImplementation(async (path: string) => {
    if (path.includes('/hourly?')) return {
      stale,
      rows: [
        ...hours.map(hour => ({ day: '2026-11-18', hour, payable_amount: '600' })),
        ...baseline.map(hour => ({ day: '2026-11-17', hour, payable_amount: '400' })),
      ],
    }
    return { rows: [], freshness: null }
  })
  const wrapper = mount(Commerce, { props: { mode: 'overview' } })
  await flushPromises()
  expect(wrapper.find('.commerce-comparison').exists()).toBe(false)
  wrapper.unmount()
})

it('falls back after an unassigned ownership response and preserves an explicit ownership filter', async () => {
  orderAPI.mockClear()
  orderAPI.mockImplementation(async (path: string) =>
    path.includes('/products?') && path.includes('mine=1') ? { rows: [], scope_state: 'unassigned' } : { rows: [] })
  const initial = mount(Commerce, { props: { mode: 'products' } })
  await flushPromises()
  expect((initial.get('select[name="product-scope"]').element as HTMLSelectElement).value).toBe('overview')
  expect(orderAPI.mock.calls.filter(([path]) => path.includes('/products?')).map(([path]) => new URL(path, 'http://localhost').searchParams.get('mine'))).toEqual(['1', null])
  initial.unmount()

  routeQuery.scope = 'mine'
  const explicit = mount(Commerce, { props: { mode: 'products' } })
  await flushPromises()
  expect((explicit.get('select[name="product-scope"]').element as HTMLSelectElement).value).toBe('mine')
  expect(explicit.text()).toContain('暂无经营归属')
  expect(orderAPI.mock.calls.filter(([path]) => path.includes('/products?')).map(([path]) => new URL(path, 'http://localhost').searchParams.get('mine'))).toEqual(['1', null, '1'])
  await explicit.findAll('button').find(button => button.text() === '切换为全部可见范围')!.trigger('click')
  await flushPromises()
  expect((explicit.get('select[name="product-scope"]').element as HTMLSelectElement).value).toBe('overview')
  expect(orderAPI.mock.calls.filter(([path]) => path.includes('/products?')).map(([path]) => new URL(path, 'http://localhost').searchParams.get('mine'))).toEqual(['1', null, '1', null])
  explicit.unmount()
  delete routeQuery.scope
})

it('displays amount-based coverage separately from the review action and keeps unknown distinct from zero', async () => {
  routeQuery.scope = 'overview'
  orderAPI.mockImplementation(async (path: string) => path.includes('/products?') ? { rows: [
    { sku: 'A', shop_id: 'shop', verified_coverage: 0, verified_gross: null },
    { sku: 'B', shop_id: 'shop', verified_coverage: 0.89999 },
    { sku: 'C', shop_id: 'shop', verified_coverage: 0.9 },
    { sku: 'D', shop_id: 'shop', verified_coverage: null },
  ] } : { rows: [] })
  const wrapper = mount(Commerce, { props: { mode: 'products' } })
  try {
    await flushPromises()
    const table = wrapper.get('.commerce-product-table')
    expect(table.findAll('tbody tr').map(row => row.findAll('td')[6]!.text()))
      .toEqual(['仅覆盖 0.0%', '仅覆盖 <90%', '90.0%', '—'])
    expect(table.findAll('.sku-check-action').map(button => button.text())).toEqual(Array(4).fill('核价'))
  } finally {
    wrapper.unmount()
    delete routeQuery.scope
  }
})
