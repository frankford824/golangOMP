import { describe, it, expect } from 'vitest'
import { parseReferenceFileRefs } from '../reference-file-refs'

describe('parseReferenceFileRefs', () => {
  it('returns empty array for non-array input', () => {
    expect(parseReferenceFileRefs(null)).toEqual([])
    expect(parseReferenceFileRefs(undefined)).toEqual([])
    expect(parseReferenceFileRefs('just a string')).toEqual([])
    expect(parseReferenceFileRefs(42)).toEqual([])
  })

  it('wraps bare string URLs into ReferenceFileRef objects', () => {
    const result = parseReferenceFileRefs(['/v1/assets/files/abc.jpg'])
    expect(result).toHaveLength(1)
    expect(result[0]).toMatchObject({ download_url: '/v1/assets/files/abc.jpg' })
  })

  it('filters out empty strings', () => {
    expect(parseReferenceFileRefs(['', '  ', '/ok.jpg'])).toHaveLength(1)
    expect(parseReferenceFileRefs(['', '  ', '/ok.jpg'])[0].download_url).toBe('/ok.jpg')
  })

  it('passes through object refs with download_url', () => {
    const input = [
      {
        asset_id: 'AST-001',
        download_url: 'https://oss.example.com/signed?token=abc',
        download_url_expires_at: '2026-04-21T12:00:00+08:00',
        filename: 'logo.png',
      },
    ]
    const result = parseReferenceFileRefs(input)
    expect(result).toHaveLength(1)
    expect(result[0].asset_id).toBe('AST-001')
    expect(result[0].download_url).toBe('https://oss.example.com/signed?token=abc')
    expect(result[0].download_url_expires_at).toBe('2026-04-21T12:00:00+08:00')
    expect(result[0].filename).toBe('logo.png')
  })

  it('does NOT apply toRelativeAssetUrl when download_url_expires_at is present', () => {
    const absUrl = 'https://oss.example.com/signed?token=abc'
    const result = parseReferenceFileRefs([
      { download_url: absUrl, download_url_expires_at: '2026-04-21T12:00:00+08:00' },
    ])
    expect(result[0].download_url).toBe(absUrl)
  })

  it('applies toRelativeAssetUrl for legacy objects without expires_at', () => {
    const result = parseReferenceFileRefs([
      { download_url: '/v1/assets/files/foo.jpg' },
    ])
    expect(result[0].download_url).toBe('/v1/assets/files/foo.jpg')
  })

  it('preserves attachment metadata when download_url is redacted', () => {
    expect(parseReferenceFileRefs([{ asset_id: 'AST-001', filename: '受限参考图.jpg', mime_type: 'image/jpeg' }])).toEqual([
      { asset_id: 'AST-001', filename: '受限参考图.jpg', mime_type: 'image/jpeg' },
    ])
  })

  it('filters out objects without a URL or display metadata', () => {
    expect(parseReferenceFileRefs([{ download_url: '' }])).toHaveLength(0)
    expect(parseReferenceFileRefs([{ download_url: null }])).toHaveLength(0)
  })

  it('handles mixed array of strings and objects', () => {
    const result = parseReferenceFileRefs([
      '/v1/assets/files/legacy.jpg',
      { download_url: 'https://oss.example.com/new.jpg', download_url_expires_at: '2026-04-21T12:00:00Z' },
      '',
      null,
      42,
    ])
    expect(result).toHaveLength(2)
    expect(result[0].download_url).toBe('/v1/assets/files/legacy.jpg')
    expect(result[1].download_url).toBe('https://oss.example.com/new.jpg')
  })
})
