import { describe, expect, it } from 'vitest'
import JSZip from 'jszip'
import {
  assertUsableBatchDownloadPayload,
  decodeNestedZipEntryFilename,
  ensureUniqueZipEntryName,
  normalizeNestedZipBlob,
  resolveBatchDownloadCredentials,
  sanitizeZipEntryName,
} from '@/utils/batchZipDownload'

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

describe('batchZipDownload request safety', () => {
  const pageOrigin = 'https://yongbo.cloud'

  it('keeps credentials for protected same-origin file routes', () => {
    expect(resolveBatchDownloadCredentials('/v1/assets/files/tasks/reference.zip', pageOrigin)).toBe('same-origin')
    expect(resolveBatchDownloadCredentials('https://yongbo.cloud/v1/assets/files/tasks/reference.zip', pageOrigin)).toBe(
      'same-origin',
    )
  })

  it('omits credentials for presigned OSS URLs', () => {
    expect(resolveBatchDownloadCredentials('https://bucket.oss-cn-hangzhou.aliyuncs.com/reference.zip?sig=1', pageOrigin)).toBe(
      'omit',
    )
  })

  it('rejects empty and error-document payloads before writing ZIP entries', () => {
    expect(() => assertUsableBatchDownloadPayload(0, 'application/zip')).toThrow('downloaded_file_is_empty')
    expect(() => assertUsableBatchDownloadPayload(120, 'application/json; charset=utf-8')).toThrow(
      'downloaded_error_payload_application_json',
    )
    expect(() => assertUsableBatchDownloadPayload(6127121, 'application/zip')).not.toThrow()
    expect(() => assertUsableBatchDownloadPayload(6127121, 'application/octet-stream')).not.toThrow()
  })

  it('recovers UTF-8 Chinese names when the ZIP UTF-8 flag is missing', () => {
    const rawName = new TextEncoder().encode('医师节桌牌拉旗策划7.8.xlsx')
    expect(decodeNestedZipEntryFilename(rawName)).toBe('医师节桌牌拉旗策划7.8.xlsx')
  })

  it('normalizes a nested ZIP copy and removes macOS metadata', async () => {
    const source = new JSZip()
    source.file('医师节桌牌拉旗策划7.8.xlsx', 'xlsx-bytes')
    source.file('__MACOSX/._医师节桌牌拉旗策划7.8.xlsx', 'apple-double')
    source.file('.DS_Store', 'finder-metadata')
    const sourceBytes = await source.generateAsync({ type: 'uint8array', compression: 'DEFLATE' })
    const sourceBuffer = sourceBytes.buffer.slice(
      sourceBytes.byteOffset,
      sourceBytes.byteOffset + sourceBytes.byteLength,
    ) as ArrayBuffer
    const sourceBlob = new Blob([sourceBuffer], { type: 'application/zip' })

    const normalizedBlob = await normalizeNestedZipBlob(sourceBlob, '资料.zip', JSZip)
    const normalized = await JSZip.loadAsync(await normalizedBlob.arrayBuffer())

    expect(Object.keys(normalized.files)).toContain('医师节桌牌拉旗策划7.8.xlsx')
    expect(Object.keys(normalized.files).some((path) => path.startsWith('__MACOSX/'))).toBe(false)
    expect(Object.keys(normalized.files)).not.toContain('.DS_Store')
    expect(await normalized.file('医师节桌牌拉旗策划7.8.xlsx')?.async('string')).toBe('xlsx-bytes')
  })
})
