import { experienceApi, type ExperienceBehaviorEventRequest } from '@/services/api/experienceApi'
import { getToken } from '@/services/http'

const endpoint = '/v1/experience/behavior-events:batch'
const maxBatchSize = 50

let pageInstanceId = ''
let flushTimer: number | null = null
let listenersRegistered = false
let behaviorCaptureEnabled = false
let behaviorSampleRate = 1
const queue: ExperienceBehaviorEventRequest[] = []

export type ExperienceBehaviorDraft = Omit<ExperienceBehaviorEventRequest, 'client_event_id'> &
  Partial<Pick<ExperienceBehaviorEventRequest, 'client_event_id'>>

function ensurePageInstanceId(): string {
  if (pageInstanceId) return pageInstanceId
  pageInstanceId =
    typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID()
      : `page-${Date.now()}-${Math.random().toString(36).slice(2)}`
  return pageInstanceId
}

function createClientEventId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `evt-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

function currentRoute(): string {
  return typeof window !== 'undefined' ? `${window.location.pathname}${window.location.search}` : ''
}

function registerFlushListeners(): void {
  if (listenersRegistered || typeof window === 'undefined' || typeof document === 'undefined') return
  listenersRegistered = true
  window.addEventListener('pagehide', () => {
    void flushExperienceBehaviorQueue({ keepalive: true })
  })
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') {
      void flushExperienceBehaviorQueue({ keepalive: true })
    }
  })
}

function scheduleFlush(): void {
  if (typeof window === 'undefined' || flushTimer) return
  flushTimer = window.setTimeout(() => {
    flushTimer = null
    void flushExperienceBehaviorQueue()
  }, 500)
}

export function setExperienceBehaviorEnabled(enabled: boolean | undefined | null): void {
  behaviorCaptureEnabled = Boolean(enabled)
}

export function setExperienceBehaviorSampleRate(rate: number | undefined | null): void {
  const parsed = Number(rate ?? 1)
  if (!Number.isFinite(parsed)) {
    behaviorSampleRate = 1
    return
  }
  behaviorSampleRate = Math.min(1, Math.max(0, parsed))
}

export function recordExperienceBehavior(event: ExperienceBehaviorDraft): void {
  if (!behaviorCaptureEnabled) return
  if (behaviorSampleRate <= 0) return
  if (behaviorSampleRate < 1 && Math.random() >= behaviorSampleRate) return
  registerFlushListeners()
  queue.push({
    ...event,
    client_event_id: event.client_event_id || createClientEventId(),
    page_instance_id: event.page_instance_id || ensurePageInstanceId(),
    occurred_at: event.occurred_at || new Date().toISOString(),
    route_name: event.route_name || currentRoute(),
  })
  if (queue.length >= maxBatchSize) {
    void flushExperienceBehaviorQueue()
    return
  }
  scheduleFlush()
}

export async function flushExperienceBehaviorQueue(options?: { keepalive?: boolean }): Promise<void> {
  if (queue.length === 0) return
  const events = queue.splice(0, maxBatchSize)
  if (flushTimer) {
    window.clearTimeout(flushTimer)
    flushTimer = null
  }

  try {
    if (options?.keepalive && typeof fetch === 'function') {
      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
        'X-Frontend-Version': import.meta.env.VITE_APP_VERSION ?? 'v1',
      }
      const token = getToken()
      if (token) headers.Authorization = `Bearer ${token}`
      await fetch(endpoint, {
        method: 'POST',
        headers,
        body: JSON.stringify({ events }),
        keepalive: true,
      })
      return
    }
    await experienceApi.behaviorEvents({ events })
  } catch {
    // Behavior telemetry is intentionally best-effort and must never block UX.
  }
}
