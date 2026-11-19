export const oaCookieSessionEnabled = import.meta.env.VITE_OA_FEATURES === 'true'
export const OA_LEGACY_TOKEN_KEY = 'yb.oa.formal.token.v1'
export const OA_SESSION_EXPIRED = 'oa-session-expired'

export const OA_SESSION_CHANGE_KEY = 'yb.oa.formal-session.changed'
let sessionId: string | null = null
let legacyBearer: string | null = null
let sequence = 0

export function getOASessionId(): string | null {
  return sessionId
}

export function markOASession(active: boolean): void {
  sessionId = active ? 'oa-session-' + ++sequence : null
  window.dispatchEvent(new Event('asset-media-session-reset'))
}

export function takeLegacyOABearer(): string | null {
  if (!oaCookieSessionEnabled) return null
  try {
    legacyBearer ??= localStorage.getItem(OA_LEGACY_TOKEN_KEY)
    localStorage.removeItem(OA_LEGACY_TOKEN_KEY)
  } catch {
    // Cookie authentication works even when browser storage is unavailable.
  }
  return legacyBearer
}

export function discardLegacyOABearer(): void {
  legacyBearer = null
}

export function announceOASessionChange(): void {
  discardLegacyOABearer()
  try {
    localStorage.setItem(OA_SESSION_CHANGE_KEY, crypto.randomUUID())
  } catch {
    // Session changes still apply to the current tab without browser storage.
  }
}

export function watchOASessionChanges(onExpire: () => void): void {
  takeLegacyOABearer()
  if (window.location.hostname === 'oa.yongbo.cloud') {
    try {
      localStorage.removeItem('access_token')
    } catch {
      // Some browsers disable storage in private or restricted contexts.
    }
  }
  window.addEventListener('storage', (event) => {
    if (event.key === OA_SESSION_CHANGE_KEY) window.location.reload()
  })
  window.addEventListener(OA_SESSION_EXPIRED, onExpire)
}
