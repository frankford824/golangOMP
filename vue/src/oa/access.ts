import { computed, shallowRef } from 'vue'
import http, { getToken } from '@/services/http'
import { OA_SESSION_CHANGE_KEY } from '@/services/oaSession'
import { usePermissionsStore } from '@/stores/permissions'
import { oaRequestError, oaResponseTraceId, showOaRequestError } from './requestErrors'

export const oaEnabled = import.meta.env.VITE_OA_FEATURES === 'true'

const REQUEST_TIMEOUT = 7_000
const FRESH_FOR = 30_000
const RETRY_AFTER = 5_000
const KEEP_CONFIRMED_FOR = 5 * 60_000

type OAAccess = {
  id: string | number
  activated: boolean
  super_admin?: boolean
  actions: Record<string, boolean>
}

type Session = { token: string; userId: string }
type AccessState = 'loading' | 'ready' | 'denied' | 'unavailable'
type Snapshot = Session & {
  state: AccessState
  checkedAt: number
  confirmed?: { access: OAAccess; checkedAt: number }
}

const snapshot = shallowRef<Snapshot>()
const pending = shallowRef<(Session & { controller: AbortController; result: Promise<void> })>()
let expiry: ReturnType<typeof setTimeout> | undefined
let accessError: ReturnType<typeof showOaRequestError>

function matchingSession(candidate: Session | undefined, session: Session | undefined) {
  return !!candidate && !!session && candidate.token === session.token && candidate.userId === session.userId
}

function currentSession(): Session | undefined {
  const token = getToken()
  const user = usePermissionsStore().currentUser
  return token && user ? { token, userId: String(user.id) } : undefined
}

function clearOAAccessError() {
  if (accessError && oaRequestError.value === accessError) oaRequestError.value = undefined
  accessError = undefined
}

function resetOAAccess() {
  clearOAAccessError()
  pending.value?.controller.abort()
  pending.value = undefined
  snapshot.value = undefined
  clearTimeout(expiry)
}

if (oaEnabled && typeof window !== 'undefined') {
  window.addEventListener('asset-media-session-reset', resetOAAccess)
  window.addEventListener('storage', event => {
    if (event.key === OA_SESSION_CHANGE_KEY || event.key === null) resetOAAccess()
  })
}

function isTransientStatus(status: number) {
  return status === 408 || status === 429 || status >= 500
}

type Check =
  | { state: 'ready'; access: OAAccess }
  | { state: 'denied' }
  | { state: 'unavailable'; retain: boolean; retry: boolean; traceId?: string }

async function checkAccess(session: Session, signal: AbortSignal): Promise<Check> {
  try {
    // The supplemental endpoint must not drive the core session's HTTP interceptor.
    const response = await http.get<{ data?: OAAccess }>('/oa-api/me', {
      timeout: REQUEST_TIMEOUT, signal, validateStatus: () => true,
    })
    const { status, data } = response
    if (status === 401 || status === 403) return { state: 'denied' }
    if (status !== 200) {
      const transient = isTransientStatus(status)
      return { state: 'unavailable', retain: transient, retry: transient && status !== 429, traceId: oaResponseTraceId(response) }
    }
    const access = data?.data
    if (access?.activated === false || (access?.id != null && String(access.id) !== session.userId)) return { state: 'denied' }
    if (access?.activated !== true || String(access.id) !== session.userId || !access.actions || typeof access.actions !== 'object' || Array.isArray(access.actions)) {
      return { state: 'unavailable', retain: true, retry: true, traceId: oaResponseTraceId(response) }
    }
    return { state: 'ready', access }
  } catch {
    return { state: 'unavailable', retain: true, retry: true }
  }
}

export async function ensureOAAccess(force = false) {
  if (!oaEnabled) return
  const session = currentSession()
  if (!session) return
  const inFlight = pending.value
  if (inFlight && matchingSession(inFlight, session)) return inFlight.result
  const prior = matchingSession(snapshot.value, session) ? snapshot.value : undefined
  const reuseFor = prior?.state === 'unavailable' ? RETRY_AFTER : FRESH_FOR
  if (!force && prior && prior.state !== 'loading' && Date.now() - prior.checkedAt < reuseFor) return
  clearOAAccessError()
  if (!prior) {
    resetOAAccess()
    snapshot.value = { ...session, state: 'loading', checkedAt: 0 }
  }
  const controller = new AbortController()
  const result = (async () => {
    let check = await checkAccess(session, controller.signal)
    if (check.state === 'unavailable' && check.retry && !controller.signal.aborted) {
      await new Promise(resolve => setTimeout(resolve, 250))
      if (!controller.signal.aborted && matchingSession(session, currentSession())) {
        check = await checkAccess(session, controller.signal)
      }
    }
    if (controller.signal.aborted || !matchingSession(session, currentSession())) return

    clearTimeout(expiry)
    const checkedAt = Date.now()
    const confirmed = check.state === 'ready' ? { access: check.access, checkedAt }
      : check.state === 'unavailable' && check.retain && prior?.confirmed
        && checkedAt - prior.confirmed.checkedAt < KEEP_CONFIRMED_FOR ? prior.confirmed : undefined
    const value = { ...session, state: check.state, checkedAt, confirmed }
    snapshot.value = value
    if (check.state === 'unavailable') {
      accessError = showOaRequestError({ message: '暂时无法核验 OA 权限。', traceId: check.traceId })
    }
    if (confirmed && check.state === 'unavailable') {
      expiry = setTimeout(() => {
        if (snapshot.value === value) snapshot.value = { ...value, confirmed: undefined }
      }, KEEP_CONFIRMED_FOR - (checkedAt - confirmed.checkedAt))
    }
  })()
  pending.value = { ...session, controller, result }
  try {
    await result
  } finally {
    if (pending.value?.result === result) pending.value = undefined
  }
}

export function useOAAccess() {
  const permissions = oaEnabled ? usePermissionsStore() : undefined
  const userId = computed(() => permissions?.currentUser ? String(permissions.currentUser.id) : undefined)
  const current = computed(() => {
    const copy = snapshot.value
    const token = getToken()
    return copy && copy.token === token && copy.userId === userId.value ? copy : undefined
  })
  const state = computed(() => current.value?.state ?? 'loading')
  const checking = computed(() => {
    const active = pending.value
    return !!active && active.token === getToken() && active.userId === userId.value
  })
  const effective = computed(() => current.value?.confirmed?.access)
  const currentProfile = computed(() => {
    const user = permissions?.currentUser
    return user ? { id: user.id, name: user.name, super_admin: effective.value?.super_admin === true } : undefined
  })
  const activated = computed(() => effective.value?.activated === true)
  const can = (key: string) => effective.value?.actions[key] === true
  return { activated, can, currentProfile, state, checking }
}
