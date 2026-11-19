import type { LocationQuery, LocationQueryRaw } from 'vue-router'
import { safeReturnPath } from './returnNavigation'

export function peopleReturnSource(value: unknown) {
  return safeReturnPath(value, (path) => path === '/users')
}

export function shopBindingReturnSource(value: unknown) {
  return safeReturnPath(value, (path) => path === '/users' || path === '/users/roster')
}

export function directoryQuery(query: LocationQuery): LocationQueryRaw {
  const { user_id: _user, tab: _tab, ...rest } = query
  return rest
}

export function positiveQueryNumber(value: unknown, fallback = 1) {
  if (typeof value !== 'string' || !/^[1-9]\d*$/.test(value)) return fallback
  const number = Number(value)
  return Number.isSafeInteger(number) ? number : fallback
}
