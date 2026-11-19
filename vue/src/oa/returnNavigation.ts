import type { RouteLocationRaw, Router } from 'vue-router'

/** Accept only an internal path in the caller's intended route family. */
export function safeReturnPath(value: unknown, allowed: (path: string) => boolean): string | undefined {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//') || value.length > 8192) return
  if (/[\\\u0000-\u001f\u007f]/.test(value) || /%(?:5c|00|0a|0d)/i.test(value)) return
  try {
    const url = new URL(value, 'https://return.invalid')
    if (url.origin !== 'https://return.invalid' || url.pathname.startsWith('//') || !allowed(url.pathname)) return
    return url.pathname + url.search + url.hash
  } catch { return }
}

// A search result can originate in any mounted, authenticated OA workspace.
// The router still applies its normal guards when the user returns.
export function workspaceOrigin(router: Router, value: unknown): string | undefined {
  return safeReturnPath(value, path => {
    if (path === '/403' || path === '/login' || path === router.currentRoute.value.path) return false
    const resolved = router.resolve(path)
    return resolved.meta.requiresAuth === true && resolved.matched.length > 0 && !resolved.matched.some(record => record.redirect)
  })
}

// Vue Router stores the preceding in-app entry (including its filters) in history.state.back.
export function detailReturnTarget(router: Router, currentPath: string, listPath: string): string {
  const back = router.options.history.state?.back
  const previous = safeReturnPath(back, path => router.resolve(path).matched.some(record => !record.redirect))
  return previous && previous !== currentPath ? previous : listPath
}

export function goBackTo(router: Router, to: RouteLocationRaw, source?: string) {
  const target = router.resolve(to)
  const back = router.options.history.state?.back
  if (typeof back === 'string' && (back === target.fullPath || (back === source && safeReturnPath(back, () => true)) ||
    (!target.hash && !Object.keys(target.query).length && safeReturnPath(back, p => p === target.path)))) {
    router.back()
    return
  }
  return router.replace(to)
}
