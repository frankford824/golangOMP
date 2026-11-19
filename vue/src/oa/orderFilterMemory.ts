import type { LocationQuery } from 'vue-router'
import { getToken } from '@/services/http'
import { dateInChina } from './api'

// A tab remembers query controls only. Start fresh on sign-in or the next Beijing day.
let identity: string | null = null
let day = ''
let shared: Record<string, string> = {}
const perPage = new Map<string, Record<string, string>>()

function currentSession() {
  const token = getToken()
  const currentDay = dateInChina()
  if (identity !== token || day !== currentDay) {
    identity = token
    day = currentDay
    shared = {}
    perPage.clear()
  }
  return Boolean(token)
}

export function orderFilters(mode: string, route: LocationQuery) {
  if (!currentSession() || mode === 'detail' || mode === 'personal') return route
  const specific = { ...perPage.get(mode) }
  const explicitRange = ['from', 'to', 'shop_id', 'include_brush'].some(key => key in route)
  if (explicitRange && ['from', 'to', 'shop_id', 'platform'].some(key => (route[key] ?? '') !== (shared[key] ?? ''))) {
    delete specific.online_order_no
  }
  return { ...specific, ...(explicitRange ? {} : shared), ...route }
}

export function rememberOrderFilters(mode: string, query: Record<string, string>, specific: Record<string, string> = {}) {
  if (!currentSession() || mode === 'detail' || mode === 'personal') return
  shared = { from: query.from, to: query.to, include_brush: query.include_brush ?? '0' }
  if (query.shop_id) shared.shop_id = query.shop_id
  perPage.set(mode, { ...specific })
}
