import { describe, it, expect } from 'vitest'
import { isReferenceUrlExpiringSoon, isReferenceUrlDefinitelyExpired } from '../referenceUrl'

describe('isReferenceUrlExpiringSoon', () => {
  it('returns false when download_url_expires_at is missing', () => {
    expect(isReferenceUrlExpiringSoon({})).toBe(false)
    expect(isReferenceUrlExpiringSoon({ download_url_expires_at: undefined })).toBe(false)
  })

  it('returns false when download_url_expires_at is invalid', () => {
    expect(isReferenceUrlExpiringSoon({ download_url_expires_at: 'not-a-date' })).toBe(false)
  })

  it('returns false when expiry is more than 60s away', () => {
    const now = Date.now()
    const expiresAt = new Date(now + 120_000).toISOString()
    expect(isReferenceUrlExpiringSoon({ download_url_expires_at: expiresAt }, now)).toBe(false)
  })

  it('returns true when expiry is within 60s', () => {
    const now = Date.now()
    const expiresAt = new Date(now + 30_000).toISOString()
    expect(isReferenceUrlExpiringSoon({ download_url_expires_at: expiresAt }, now)).toBe(true)
  })

  it('returns true when already expired', () => {
    const now = Date.now()
    const expiresAt = new Date(now - 5_000).toISOString()
    expect(isReferenceUrlExpiringSoon({ download_url_expires_at: expiresAt }, now)).toBe(true)
  })

  it('returns true at exactly 60s boundary', () => {
    const now = Date.now()
    const expiresAt = new Date(now + 60_000).toISOString()
    expect(isReferenceUrlExpiringSoon({ download_url_expires_at: expiresAt }, now)).toBe(true)
  })

  it('handles +08:00 timezone offset', () => {
    const now = new Date('2026-04-21T04:00:00Z').getTime()
    const expiresAt = '2026-04-21T12:00:30+08:00'
    expect(isReferenceUrlExpiringSoon({ download_url_expires_at: expiresAt }, now)).toBe(true)
  })
})

describe('isReferenceUrlDefinitelyExpired', () => {
  it('returns false when download_url_expires_at is missing', () => {
    expect(isReferenceUrlDefinitelyExpired({})).toBe(false)
  })

  it('returns false when download_url_expires_at is invalid', () => {
    expect(isReferenceUrlDefinitelyExpired({ download_url_expires_at: 'garbage' })).toBe(false)
  })

  it('returns false when not yet expired', () => {
    const now = Date.now()
    const expiresAt = new Date(now + 60_000).toISOString()
    expect(isReferenceUrlDefinitelyExpired({ download_url_expires_at: expiresAt }, now)).toBe(false)
  })

  it('returns true when expired', () => {
    const now = Date.now()
    const expiresAt = new Date(now - 1_000).toISOString()
    expect(isReferenceUrlDefinitelyExpired({ download_url_expires_at: expiresAt }, now)).toBe(true)
  })

  it('returns true at exactly the expiry time', () => {
    const now = Date.now()
    const expiresAt = new Date(now).toISOString()
    expect(isReferenceUrlDefinitelyExpired({ download_url_expires_at: expiresAt }, now)).toBe(true)
  })
})
