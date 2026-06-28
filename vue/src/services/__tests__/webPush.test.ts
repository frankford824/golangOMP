import { describe, expect, it } from 'vitest'
import { base64UrlToUint8Array } from '@/services/webPush'

describe('webPush', () => {
  it('converts base64url VAPID keys to Uint8Array', () => {
    const bytes = base64UrlToUint8Array('SGVsbG8td29ybGQ')
    expect(Array.from(bytes)).toEqual(Array.from(new TextEncoder().encode('Hello-world')))
  })
})
