import {
  experienceApi,
  type ExperienceBehaviorEventRequest,
  type ExperienceClientConfig,
} from '@/services/api/experienceApi'
import { getToken } from '@/services/http'

const endpoint = '/v1/experience/behavior-events:batch'
const maxBatchSize = 50
const maxSampleDecisionCacheSize = 1000

let pageInstanceId = ''
let flushTimer: number | null = null
let listenersRegistered = false
let behaviorConfigKnown = false
let behaviorConfigPromise: Promise<void> | null = null
let behaviorCaptureEnabled = false
let behaviorSampleRate = 1
let enabledSurfaces = new Set<string>()
let sampleDecisions = new Map<string, boolean>()
const queue: ExperienceBehaviorEventRequest[] = []
const pendingDrafts: ExperienceBehaviorDraft[] = []

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
  behaviorConfigKnown = true
  behaviorCaptureEnabled = Boolean(enabled)
}

export function setExperienceBehaviorSampleRate(rate: number | undefined | null): void {
  behaviorConfigKnown = true
  updateExperienceBehaviorSampleRate(rate)
}

export function setExperienceBehaviorEnabledSurfaces(surfaces: string[] | undefined | null): void {
  behaviorConfigKnown = true
  updateExperienceBehaviorEnabledSurfaces(surfaces)
}

export function configureExperienceBehavior(config: ExperienceClientConfig | null): void {
  behaviorConfigKnown = true
  behaviorCaptureEnabled = Boolean(config?.behavior_capture_enabled)
  updateExperienceBehaviorSampleRate(config?.behavior_sample_rate ?? 1)
  updateExperienceBehaviorEnabledSurfaces(config?.enabled_surfaces ?? [])
  drainPendingExperienceBehavior()
}

export function resetExperienceBehaviorForTests(): void {
  if (flushTimer && typeof window !== 'undefined') {
    window.clearTimeout(flushTimer)
  }
  pageInstanceId = ''
  flushTimer = null
  behaviorConfigKnown = false
  behaviorConfigPromise = null
  behaviorCaptureEnabled = false
  behaviorSampleRate = 1
  enabledSurfaces = new Set<string>()
  sampleDecisions = new Map<string, boolean>()
  queue.splice(0, queue.length)
  pendingDrafts.splice(0, pendingDrafts.length)
}

export function recordExperienceBehavior(event: ExperienceBehaviorDraft): void {
  if (!behaviorConfigKnown) {
    if (pendingDrafts.length < maxBatchSize) pendingDrafts.push(event)
    void loadExperienceBehaviorClientConfig()
    return
  }
  enqueueExperienceBehavior(event)
}

function enqueueExperienceBehavior(event: ExperienceBehaviorDraft): void {
  if (!behaviorCaptureEnabled) return
  if (!enabledSurfaces.has(String(event.surface ?? '').trim())) return
  if (!shouldRecordExperienceBehaviorSample(event)) return
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

async function loadExperienceBehaviorClientConfig(): Promise<void> {
  if (behaviorConfigKnown) return
  if (!behaviorConfigPromise) {
    behaviorConfigPromise = experienceApi
      .clientConfig()
      .then((res) => {
        if (!behaviorConfigKnown) configureExperienceBehavior(res.data?.data ?? null)
        drainPendingExperienceBehavior()
      })
      .catch(() => {
        if (!behaviorConfigKnown) configureExperienceBehavior(null)
        drainPendingExperienceBehavior()
      })
      .finally(() => {
        behaviorConfigPromise = null
      })
  }
  return behaviorConfigPromise
}

function drainPendingExperienceBehavior(): void {
  if (pendingDrafts.length === 0) return
  const drafts = pendingDrafts.splice(0, pendingDrafts.length)
  drafts.forEach((draft) => enqueueExperienceBehavior(draft))
}

function updateExperienceBehaviorSampleRate(rate: number | undefined | null): void {
  const parsed = Number(rate ?? 1)
  if (!Number.isFinite(parsed)) {
    behaviorSampleRate = 1
    return
  }
  behaviorSampleRate = Math.min(1, Math.max(0, parsed))
}

function updateExperienceBehaviorEnabledSurfaces(surfaces: string[] | undefined | null): void {
  enabledSurfaces = new Set((surfaces ?? []).map((item) => String(item).trim()).filter(Boolean))
}

function shouldRecordExperienceBehaviorSample(event: ExperienceBehaviorDraft): boolean {
  if (behaviorSampleRate <= 0) return false
  if (behaviorSampleRate >= 1) return true
  const key = experienceBehaviorSampleKey(event)
  if (sampleDecisions.has(key)) {
    return sampleDecisions.get(key) === true
  }
  if (sampleDecisions.size >= maxSampleDecisionCacheSize) {
    sampleDecisions.clear()
  }
  const accepted = Math.random() < behaviorSampleRate
  sampleDecisions.set(key, accepted)
  return accepted
}

function experienceBehaviorSampleKey(event: ExperienceBehaviorDraft): string {
  const surface = String(event.surface ?? '').trim()
  const suggestionKey = String(event.suggestion_event_id || event.suggestion_stable_key || '').trim()
  if (suggestionKey) return `${surface}:suggestion:${suggestionKey}`
  const targetType = String(event.target_type ?? '').trim()
  const targetID = String(event.target_id ?? '').trim()
  if (targetType || targetID) return `${surface}:target:${targetType}:${targetID}`
  return `${surface}:page:${event.page_instance_id || ensurePageInstanceId()}`
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
