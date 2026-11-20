import type { LocationQuery, RouteLocationNormalizedLoaded, Router } from 'vue-router'
import { getToken } from '@/services/http'
import { dateInChina } from './api'
import { safeReturnPath } from './returnNavigation'

// A tab remembers query controls only. Start fresh on sign-in or the next Beijing day.
let identity: string | null = null
let day = ''
let shared: Record<string, string> = {}
const perPage = new Map<string, Record<string, string>>()
const visits = new Map<string, unknown>()
let visitSequence = 0
const visitPrefix = Math.random().toString(36).slice(2)

function currentSession() {
  const token = getToken()
  const currentDay = dateInChina()
  if (identity !== token || day !== currentDay) {
    identity = token
    day = currentDay
    shared = {}
    perPage.clear()
    visits.clear()
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


export function orderVisit<T>(name: string, persist?: { owner: string | number; select: (value: T) => T }) {
  const state = typeof window === 'undefined' ? undefined : window.history.state
  let historyEntry: string | undefined = state?.oaOrderVisit
  if (typeof state?.position === 'number' && !historyEntry) {
    historyEntry = visitPrefix + '-' + ++visitSequence
    window.history.replaceState({ ...state, oaOrderVisit: historyEntry }, '')
  }
  const identityAtEntry = getToken()
  const key = historyEntry ? historyEntry + ':' + name : ''
  return {
    read(): T | undefined {
      if (!currentSession() || !key) return
      if (visits.has(key)) return visits.get(key) as T
      const saved = state?.oaOrderListPage
      if (persist && saved?.key === key && saved.owner === persist.owner && saved.day === day) return saved.value as T
    },
    save(snapshot: T, persistControls = true) {
      if (!currentSession() || !key || identity !== identityAtEntry) return
      visits.set(key, snapshot)
      // Keep only applied list controls on their own browser entry after a reload.
      const entry = window.history.state
      if (persist && persistControls && entry?.oaOrderVisit === historyEntry) {
        const value = persist.select(snapshot)
        const saved = entry.oaOrderListPage
        if (saved?.key !== key || saved.owner !== persist.owner || saved.day !== day || JSON.stringify(value) !== JSON.stringify(saved.value)) {
          window.history.replaceState({ ...entry, oaOrderListPage: { key, owner: persist.owner, day, value } }, '')
        }
      }
      if (visits.size > 80) visits.delete(visits.keys().next().value!)
    },
  }
}

export function orderOffset(value: unknown, maximum = 10000) {
  const n = typeof value === 'string' ? Number(value) : value
  return typeof n === 'number' && Number.isInteger(n) && n >= 0 && n <= maximum && n % 100 === 0 ? n : 0
}

export function orderReturnQuery(router: Router, path: string, query: Record<string, string>, source: string) {
  return { returnTo: source, returnFallback: router.resolve({ path, query }).fullPath }
}

const orderReturnRoutes = /^(?:\/operations(?:\/(?:products(?:\/[^/]+)?|rankings|pricing|orders\/(?:detail|ledger|refunds)))?|\/me\/(?:performance|notifications)|\/cost-rules)$/
export function orderReturnSource(route: RouteLocationNormalizedLoaded) {
  return safeReturnPath(route.query.returnTo, path => orderReturnRoutes.test(path))
}
export function orderReturnTarget(route: RouteLocationNormalizedLoaded) {
  return safeReturnPath(route.query.returnFallback, path => orderReturnRoutes.test(path)) ?? orderReturnSource(route)
}
