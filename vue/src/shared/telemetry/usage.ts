import type { Router, RouteLocationNormalizedLoaded } from 'vue-router'
import { currentUsageContext, safeUsageURL, setUsageContext, type UsageApp, type UsageContext } from './context'

type Outcome = 'succeeded' | 'failed' | 'cancelled' | 'handed_off' | 'pending'
type Metadata = Record<string, string | number | boolean | undefined>
type Event = {
  event_type: 'page_view' | 'page_engagement' | 'user_action'
  action: string
  page_url: string
  page_name: string
  outcome: Outcome
  occurred_at: string
  task_id?: number
  payload: Metadata
}
type Send = (event: Event, token: string) => Promise<unknown>
const installed = new WeakMap<Router, () => void>()
let publish: ((type: Event['event_type'], action: string, outcome: Outcome, metadata?: Metadata, context?: UsageContext) => void) | undefined

const newID = () => globalThis.crypto?.randomUUID?.() ?? 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, key => {
  const value = Math.floor(Math.random() * 16)
  return (key === 'x' ? value : (value & 3) | 8).toString(16)
})

// No URL, filename, error message, request body or search values in usage data.
function errorMetadata(error: unknown): Metadata {
  const e = error as { name?: string; code?: string; status?: number; response?: { status?: number } } | null
  return {
    error_code: /^[A-Za-z][A-Za-z0-9_.-]{0,63}$/.test(e?.code ?? '') ? e!.code : 'CLIENT_ERROR',
    error_type: /^[A-Za-z][A-Za-z0-9]{0,63}$/.test(e?.name ?? '') ? e!.name : 'Error',
    http_status: e?.status ?? e?.response?.status,
  }
}

export function beginUsageOperation(action: string, metadata: Metadata = {}) {
  const context = currentUsageContext()
  if (!context) return (_outcome: Outcome, _error?: unknown, _result?: Metadata) => undefined
  const operationID = newID()
  const started = performance.now()
  let finished = false
  if (context) publish?.('user_action', action, 'pending', { ...metadata, operation_id: operationID, phase: 'started' }, context)
  return (outcome: Outcome, error?: unknown, result: Metadata = {}) => {
    if (finished) return
    finished = true
    // Keep the originating page, but never attribute a delayed completion to a new account.
    if (!context || currentUsageContext()?.session_id !== context.session_id) return
    publish?.('user_action', action, outcome, {
      ...metadata, ...result, ...(error ? errorMetadata(error) : {}),
      operation_id: operationID, phase: 'finished', duration_ms: Math.max(0, Math.round(performance.now() - started)),
    }, context)
  }
}

export async function observeUsageOperation<T>(action: string, metadata: Metadata, run: () => Promise<T>, success: Outcome = 'succeeded'): Promise<T> {
  const finish = beginUsageOperation(action, metadata)
  try { const result = await run(); finish(success); return result }
  catch (error) {
    const name = (error as { name?: string } | null)?.name
    finish(name === 'AbortError' || name === 'CanceledError' ? 'cancelled' : 'failed', error)
    throw error
  }
}

export function installUsageTelemetry(router: Router, options: { appId: UsageApp; getToken: () => string | null; send?: Send }): () => void {
  const existing = installed.get(router)
  if (existing) return existing
  if (typeof window === 'undefined') return () => undefined
  const readToken = () => { try { return options.getToken() } catch { return null } }
  const send: Send = options.send ?? (async (event, token) => {
    // Dedicated transport: never recursively instrument itself or clear login on telemetry failure.
    const response = await fetch('/v1/trace-events', {
      method: 'POST', credentials: 'same-origin', keepalive: true,
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}`, 'X-Usage-App': options.appId },
      body: JSON.stringify(event), signal: AbortSignal.timeout(5000),
    })
    if (!response.ok) throw new Error('usage_delivery_failed')
  })
  let token: string | null = null
  let sessionID = ''
  let page: UsageContext | undefined
  let visibleSince: number | undefined
  let activityAt = Date.now()
  let inFlight = 0
  let dropped = 0
  let disposed = false
  let lastErrorAt = -Infinity

  const emit: NonNullable<typeof publish> = (type, action, outcome, metadata = {}, originating = page) => {
    try {
      if (disposed || !originating || !token || readToken() !== token) return
      if (inFlight >= 16) { dropped++; return }
      const allowed = new Set(['operation_id', 'upload_session_id', 'phase', 'phase_detail', 'transfer_mode', 'error_code', 'error_type', 'http_status', 'file_size', 'received_bytes', 'duration_ms', 'visible_ms', 'requested_files', 'successful_files', 'failed_files'])
      const safeMetadata = Object.fromEntries(Object.entries(metadata).filter(([key]) => allowed.has(key)))
      const taskID = Number(metadata.task_id ?? originating.task_id)
      const event: Event = {
        event_type: type, action, outcome, page_url: originating.page_url,
        page_name: originating.page_name, occurred_at: new Date().toISOString(),
        task_id: Number.isSafeInteger(taskID) && taskID > 0 ? taskID : undefined,
        payload: {
          ...safeMetadata, telemetry_schema: 'usage.v1', app_id: options.appId,
          session_id: originating.session_id, page_view_id: originating.page_view_id,
          route_pattern: originating.route_pattern, automation: originating.automation,
          client_event_id: newID(), dropped_events: dropped,
        },
      }
      dropped = 0; inFlight++
      ;(async () => send(event, token!))().catch(() => { dropped++ }).finally(() => { inFlight-- })
    } catch { /* Analytics must never interrupt navigation or file transfers. */ }
  }
  publish = emit

  function flushEngagement() {
    if (visibleSince == null) return
    const duration = Math.min(60_000, Math.max(0, Math.round(performance.now() - visibleSince)))
    visibleSince = undefined
    if (duration > 0) emit('page_engagement', 'page.visible', 'succeeded', { visible_ms: duration })
  }
  function enter(to: RouteLocationNormalizedLoaded) {
    flushEngagement()
    const currentToken = readToken()
    if (!currentToken || to.path === '/login' || to.meta.public) {
      token = currentToken; page = undefined; setUsageContext(undefined); return
    }
    if (currentToken !== token || !sessionID || Date.now() - activityAt >= 30 * 60_000) sessionID = newID()
    token = currentToken; activityAt = Date.now()
    const routePattern = to.matched[to.matched.length - 1]?.path ?? to.path
    const taskID = routePattern.startsWith('/tasks/:id') ? Number(to.params.id) : NaN
    page = {
      app_id: options.appId, session_id: sessionID, page_view_id: newID(),
      route_pattern: routePattern,
      page_url: safeUsageURL(new URL(to.path, window.location.origin).href),
      page_name: String(to.meta.label ?? to.meta.title ?? to.name ?? to.path).slice(0, 128),
      automation: navigator.webdriver === true,
      task_id: Number.isSafeInteger(taskID) && taskID > 0 ? taskID : undefined,
    }
    setUsageContext(page, readToken)
    emit('page_view', 'page.open', 'succeeded')
    if (document.visibilityState === 'visible') visibleSince = performance.now()
  }
  const removeRoute = router.afterEach((to, _from, failure) => { if (!failure) enter(to) })
  const visibility = () => {
    if (document.visibilityState !== 'visible') { flushEngagement(); return }
    if (readToken() !== token || Date.now() - activityAt >= 30 * 60_000) enter(router.currentRoute.value)
    else if (page && visibleSince == null) visibleSince = performance.now()
  }
  const reset = () => { flushEngagement(); page = undefined; token = null; sessionID = ''; setUsageContext(undefined) }
  const activity = (event: globalThis.Event) => {
    if (!event.isTrusted || !page) return
    if (Date.now() - activityAt >= 30 * 60_000) enter(router.currentRoute.value)
    activityAt = Date.now()
  }
  const restore = (event: PageTransitionEvent) => { if (event.persisted) enter(router.currentRoute.value) }
  const onError = (event: globalThis.Event) => {
    if (Date.now() - lastErrorAt < 30_000) { dropped++; return }
    lastErrorAt = Date.now()
    const error = 'reason' in event ? event.reason : 'error' in event ? event.error : undefined
    emit('user_action', 'client.error', 'failed', errorMetadata(error))
  }
  document.addEventListener('visibilitychange', visibility)
  window.addEventListener('pagehide', flushEngagement)
  window.addEventListener('pageshow', restore)
  document.addEventListener('pointerdown', activity, true)
  document.addEventListener('keydown', activity, true)
  window.addEventListener('asset-media-session-reset', reset)
  window.addEventListener('error', onError)
  window.addEventListener('unhandledrejection', onError)
  // Report bounded visible intervals; never tick in a hidden tab.
  const timer = window.setInterval(() => {
    if (document.visibilityState !== 'visible' || !page) return
    flushEngagement()
    if (readToken() === token) visibleSince = performance.now()
  }, 60_000)
  const stop = () => {
    flushEngagement(); disposed = true; removeRoute(); window.clearInterval(timer)
    document.removeEventListener('visibilitychange', visibility)
    window.removeEventListener('pagehide', flushEngagement)
    window.removeEventListener('pageshow', restore)
    document.removeEventListener('pointerdown', activity, true)
    document.removeEventListener('keydown', activity, true)
    window.removeEventListener('asset-media-session-reset', reset)
    window.removeEventListener('error', onError)
    window.removeEventListener('unhandledrejection', onError)
    if (publish === emit) { publish = undefined; setUsageContext(undefined) }
    installed.delete(router)
  }
  installed.set(router, stop)
  return stop
}
