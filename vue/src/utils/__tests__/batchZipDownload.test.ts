import { describe, expect, it } from 'vitest'
import { ensureUniqueZipEntryName, sanitizeZipEntryName } from '@/utils/batchZipDownload'

describe('batchZipDownload zipPath naming', () => {
  it('dedupes within same directory using (2) suffix', () => {
    const used = new Map<string, number>()
    const dir = '需求1/参考图'
    const base = 'photo.jpg'
    const first = ensureUniqueZipEntryName(`${dir}/${base}`, used)
    const second = ensureUniqueZipEntryName(`${dir}/${base}`, used)
    expect(first).toBe(`${dir}/${base}`)
    expect(second).toBe(`${dir}/photo (2).jpg`)
  })

  it('allows same basename in different directories', () => {
    const used = new Map<string, number>()
    const a = ensureUniqueZipEntryName(
      `${sanitizeZipEntryName('需求1', 'req')}/${sanitizeZipEntryName('参考图', 'ref')}/same.jpg`,
      used,
    )
    const b = ensureUniqueZipEntryName(
      `${sanitizeZipEntryName('需求2', 'req')}/${sanitizeZipEntryName('参考图', 'ref')}/same.jpg`,
      used,
    )
    expect(a).toBe('需求1/参考图/same.jpg')
    expect(b).toBe('需求2/参考图/same.jpg')
  })
})
