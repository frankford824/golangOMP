// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import Commerce from './Commerce.vue'

const { orderAPI } = vi.hoisted(() => ({ orderAPI: vi.fn() }))
vi.mock('vue-router', () => ({
  RouterLink: { template: '<a><slot /></a>' },
  useRoute: () => ({ path: '/operations', query: {} }),
}))
vi.mock('@/services/http', () => ({ getToken: () => 'test-session' }))
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
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-11-18T08:30:00Z'))
})
afterEach(() => vi.useRealTimers())

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
