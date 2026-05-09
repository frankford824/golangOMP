import type { ReferenceFileRef } from '@/services/api/assetsApi'

/** Proactive refresh window: trigger refetch 60s before actual expiry. */
const REFRESH_WINDOW_MS = 60_000

export function isReferenceUrlExpiringSoon(
  ref: Pick<ReferenceFileRef, 'download_url_expires_at'>,
  now: number = Date.now(),
): boolean {
  if (!ref?.download_url_expires_at) return false
  const expiresAt = new Date(ref.download_url_expires_at).getTime()
  if (!Number.isFinite(expiresAt)) return false
  return expiresAt - now <= REFRESH_WINDOW_MS
}

export function isReferenceUrlDefinitelyExpired(
  ref: Pick<ReferenceFileRef, 'download_url_expires_at'>,
  now: number = Date.now(),
): boolean {
  if (!ref?.download_url_expires_at) return false
  const expiresAt = new Date(ref.download_url_expires_at).getTime()
  return Number.isFinite(expiresAt) && expiresAt <= now
}
