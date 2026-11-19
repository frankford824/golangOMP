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
  useOAAccess: () => ({ can: (action: string) => action === 'orders.view', currentProfile: { value: { id: '7', super_admin: true } } }),
}))
vi.mock('../api', async (importOriginal) => ({
  ...await importOriginal<typeof import('../api')>(),
  api: orderAPI,
  dateInChina: () => '2026-11-18',
}))

beforeEach(() => {
  session.number++
  orderAPI.mockReset()
  for (const key of Object.keys(routeQuery)) delete routeQuery[key]
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-11-18T08:30:00Z'))
})
afterEach(() => vi.useRealTimers())

function rankingRequests() {
  return orderAPI.mock.calls.map(([path]) => new URL(path, 'https://oa.test'))
    .filter(url => url.pathname === '/oa-api/orders/rankings')
    .map(url => Object.fromEntries(url.searchParams))
}

it('uses global service order and ranks on later pages, and starts a new sort from the first row', async () => {
  Object.assign(routeQuery, { group: 'product', rankMetric: 'amount', rankDirection: 'asc', offset: '10100' })
  orderAPI.mockImplementation(async (path: string) => path.includes('/rankings?') ? {
    has_more: true, total_rows: 10201, rows: [
      { shop_id: 's1', shop_item_id: 'p1', title: '宝贝标题', product_name: 'SKU 名称', item_pay_total: '60' },
      { shop_id: 's2', key: 'p2', product_name: '另一个 SKU 名称', item_pay_total: '20' },
    ],
  } : { rows: [] })
  const wrapper = mount(Commerce, { props: { mode: 'ranking' } })
  await flushPromises()
  expect(rankingRequests()).toEqual([expect.objectContaining({ group: 'product', sort: 'amount', order: 'asc', offset: '10100' })])
  expect(wrapper.text()).toContain('共 10,201 条')
  expect(wrapper.findAll('tbody .commerce-rank').map(cell => cell.text())).toEqual(['10101', '10102'])
  expect(wrapper.findAll('tbody tr').map(row => row.findAll('td')[1].text())).toEqual(['宝贝标题商品 ID p1', 'p2'])
  expect(orderAPI.mock.calls.some(([path]) => path.includes('/products?'))).toBe(false)
  await wrapper.find('.workspace-pager button:last-child').trigger('click')
  await flushPromises()
  expect(rankingRequests().at(-1)?.offset).toBe('10200')
  await wrapper.find('.commerce-ranking-sort label:nth-child(2) select').setValue('desc')
  await flushPromises()
  expect(rankingRequests().at(-1)).toMatchObject({ offset: '0', order: 'desc' })
  expect(wrapper.findAll('tbody .commerce-rank').map(cell => cell.text())).toEqual(['1', '2'])
  wrapper.unmount()
})

it('keeps the latest ranking request during fast sort changes and retries a failed selection', async () => {
  const pending: { resolve: (value: { rows: { key: string }[] }) => void; reject: (error: Error) => void }[] = []
  orderAPI.mockImplementation((path: string) => path.includes('/rankings?')
    ? new Promise((resolve, reject) => pending.push({ resolve, reject }))
    : Promise.resolve({ rows: [] }))
  const wrapper = mount(Commerce, { props: { mode: 'ranking' } })
  await flushPromises()
  await wrapper.find('.commerce-ranking-sort label:nth-child(1) select').setValue('grossRate')
  await wrapper.find('.commerce-ranking-sort label:nth-child(2) select').setValue('asc')
  expect(rankingRequests().at(-1)).toMatchObject({ sort: 'grossRate', order: 'asc', offset: '0' })
  pending[2].reject(new Error('当前查询失败'))
  pending[0].resolve({ rows: [{ key: '旧列表' }] })
  pending[1].resolve({ rows: [{ key: '过期排序' }] })
  await flushPromises()
  expect(wrapper.get('[role=alert]').text()).toContain('当前查询失败')
  expect(wrapper.find('tbody').exists()).toBe(false)
  await wrapper.get('[role=alert] button').trigger('click')
  expect(rankingRequests().at(-1)).toEqual(rankingRequests().at(-2))
  pending[3].resolve({ rows: [{ key: '新排名' }] })
  await flushPromises()
  expect(wrapper.find('tbody').text()).toContain('新排名')
  expect(wrapper.find('[role=alert]').exists()).toBe(false)
  wrapper.unmount()
})

it('loads shop-SKU rows and restores the submitted SKU sort and page on browser return', async () => {
  const priorState = window.history.state
  window.history.replaceState({ position: 1 }, '')
  Object.assign(routeQuery, { group: 'sku', shop_id: 's1', from: '2026-11-17', to: '2026-11-18', include_brush: '1', sort: 'quantity', order: 'asc' })
  orderAPI.mockImplementation(async (path: string) => path.includes('/rankings?') ? { has_more: new URL(path, 'https://oa.test').searchParams.get('offset') === '0', rows: [
    { shop_id: 's1', sku: 'sku-1', product_name: '实际 SKU 名称', quantity: '18', item_pay_total: '99' },
  ] } : { rows: [] })
  const wrapper = mount(Commerce, { props: { mode: 'ranking' } })
  await flushPromises()
  expect(rankingRequests()[0]).toEqual({
    from: '2026-11-17', to: '2026-11-18', shop_id: 's1', include_brush: '1', offset: '0', group: 'sku', sort: 'quantity', order: 'asc',
  })
  expect(wrapper.get('tbody').text()).toContain('实际 SKU 名称')
  expect(wrapper.get('tbody').text()).toContain('SKU sku-1')
  expect(wrapper.get('tbody').text()).toContain('18')
  expect(wrapper.get('tbody').text()).not.toContain('商品 ID')
  await wrapper.find('.commerce-ranking-sort label:nth-child(2) select').setValue('desc')
  await flushPromises()
  await wrapper.find('.workspace-pager button:last-child').trigger('click')
  await flushPromises()
  wrapper.unmount()

  const returned = mount(Commerce, { props: { mode: 'ranking' } })
  await flushPromises()
  expect(rankingRequests().at(-1)).toMatchObject({ group: 'sku', sort: 'quantity', order: 'desc', offset: '100' })
  expect(returned.find('.workspace-pager').text()).toContain('第 2 页')
  expect(returned.find('tbody .commerce-rank').text()).toBe('101')
  returned.unmount()
  window.history.replaceState(priorState, '')
})

it('displays roster names, archived status, and missing-name IDs in the personnel ranking', async () => {
  routeQuery.group = 'person'
  orderAPI.mockImplementation(async (path: string) => path.includes('/rankings?') ? { rows: [
    { key: 'old-key', person_id: 'PI-1', name: '归档人员', archived: 1 },
    { key: 'old-key-2', person_id: 'PI-2', name: ' ', archived: 0 },
  ] } : { rows: [] })
  const wrapper = mount(Commerce, { props: { mode: 'ranking' } })
  await flushPromises()
  const people = wrapper.findAll('tbody tr')
  expect(people[0].text()).toContain('归档人员')
  expect(people[0].text()).toContain('PI-1')
  expect(people[0].text()).toContain('已归档')
  expect(people[1].text()).toContain('PI-2')
  expect(people[1].text()).not.toContain('已归档')
  wrapper.unmount()
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
