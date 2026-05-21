import type { ReferenceFileRef } from '@/services/api/assetsApi'
import { toRelativeAssetUrl } from '@/utils/url'

/**
 * `reference_file_refs`：字符串（历史 URL）或 `ReferenceFileRef` 对象。
 *
 * v1.18+：后端返回的 presigned `download_url` 带 `download_url_expires_at`，
 * 此时 URL 是外部 OSS 直链，**不得** 经 `toRelativeAssetUrl` 折叠为相对路径。
 * 仅对无过期标记的 legacy URL 继续做同源折叠。
 */
function trimRefField(value: unknown): string {
  return String(value ?? '').trim()
}

/** Stable dedupe key for detail-page reference display (first match wins). */
export function referenceFileRefDedupeKey(ref: ReferenceFileRef, index: number): string {
  const refId = trimRefField(ref.ref_id)
  if (refId) return `ref_id:${refId}`
  const assetId = trimRefField(ref.asset_id)
  if (assetId) return `asset_id:${assetId}`
  const uploadRequestId = trimRefField(ref.upload_request_id)
  if (uploadRequestId) return `upload_request_id:${uploadRequestId}`
  const storageKey = trimRefField(ref.storage_key)
  if (storageKey) return `storage_key:${storageKey}`
  const downloadUrl = trimRefField(ref.download_url)
  if (downloadUrl) return `download_url:${downloadUrl}`
  const legacyUrl = trimRefField((ref as { url?: string }).url)
  if (legacyUrl) return `url:${legacyUrl}`
  const filename = trimRefField(ref.filename)
  const fileSize = ref.file_size != null ? String(ref.file_size) : ''
  if (filename || fileSize) return `filename_size:${filename}\0${fileSize}`
  return `__index:${index}`
}

/**
 * Dedupe reference_file_refs for task-detail display.
 * Different ref_id/asset_id are never collapsed even when filename matches.
 */
export function dedupeReferenceFileRefs(refs: ReferenceFileRef[]): ReferenceFileRef[] {
  if (!refs.length) return []
  const seen = new Set<string>()
  const out: ReferenceFileRef[] = []
  refs.forEach((ref, index) => {
    const key = referenceFileRefDedupeKey(ref, index)
    if (seen.has(key)) return
    seen.add(key)
    out.push(ref)
  })
  return out
}

export function parseReferenceFileRefs(raw: unknown): ReferenceFileRef[] {
  if (!Array.isArray(raw)) return []
  return raw.flatMap((item) => {
    if (typeof item === 'string') {
      const url = (toRelativeAssetUrl(item) ?? item.trim()) || ''
      return url ? [{ download_url: url } as ReferenceFileRef] : []
    }
    if (item && typeof item === 'object') {
      const obj = { ...(item as ReferenceFileRef) }
      if (obj.download_url && !obj.download_url_expires_at) {
        obj.download_url = toRelativeAssetUrl(obj.download_url as string) ?? obj.download_url
      }
      return obj.download_url ? [obj] : []
    }
    return []
  })
}
