// @vitest-environment jsdom
import { beforeEach, expect, it, vi } from 'vitest'
import { orderFilters, rememberOrderFilters } from './orderFilterMemory'

const session = vi.hoisted(() => ({ token: 'session-A', day: '2026-11-19' }))
vi.mock('@/services/http', () => ({ getToken: () => session.token }))
vi.mock('./api', () => ({ dateInChina: () => session.day }))

beforeEach(() => {
  session.token = ''
  orderFilters('overview', {})
  session.token = 'session-A'
  session.day = '2026-11-19'
})

it('preserves applied range and page-specific controls on return while explicit links win', () => {
  rememberOrderFilters('products', { from: '2026-11-13', to: session.day, shop_id: 'A' }, { scope: 'overview', exception: 'coverage' })
  expect(orderFilters('overview', {})).toMatchObject({ shop_id: 'A', from: '2026-11-13' })
  expect(orderFilters('products', {})).toMatchObject({ scope: 'overview', exception: 'coverage' })
  expect(orderFilters('products', { from: session.day, to: session.day, scope: 'mine' })).not.toHaveProperty('shop_id')
  expect(orderFilters('products', { scope: 'mine' }).scope).toBe('mine')
  expect(orderFilters('personal', {})).toEqual({})
})

it('discards controls after changing login or Beijing day', () => {
  rememberOrderFilters('ranking', { from: session.day, to: session.day }, { group: 'person' })
  session.token = 'session-B'
  expect(orderFilters('ranking', {})).toEqual({})
  rememberOrderFilters('ranking', { from: session.day, to: session.day })
  session.day = '2026-11-20'
  expect(orderFilters('ranking', {})).toEqual({})
})

it('restores list controls after a reload only for the same user, entry and business day', async () => {
  const original = window.history.state
  try {
    window.history.replaceState({ position: 2 }, '')
    const value = { filters: { shop_id: 'shop-a' }, offset: 100, row: 'selected-only-in-memory' }
    const policy = { owner: 7, select: (v: typeof value) => ({ ...v, row: '' }) }
    const first = await import('./orderFilterMemory')
    first.orderVisit('products', policy).save(value)
    vi.resetModules()
    const fresh = await import('./orderFilterMemory')
    expect(fresh.orderVisit('products', policy).read()).toEqual({ ...value, row: '' })
    expect(fresh.orderVisit('products', { ...policy, owner: 8 }).read()).toBeUndefined()
    expect(fresh.orderVisit('ranking', policy).read()).toBeUndefined()
    session.day = '2026-11-20'
    expect(fresh.orderVisit('products', policy).read()).toBeUndefined()
  } finally {
    window.history.replaceState(original, '')
  }
})
