import type { ReferenceFileRef } from '@/services/api/assetsApi'
import type { BackendAsset, BackendAssetVersion } from '@/services/apiTypes'
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

function backendReferenceAssetKind(asset: BackendAsset): string {
  const rec = asset as Record<string, unknown>
  return String(
    rec.asset_kind ?? rec.assetKind ?? rec.asset_type ?? rec.assetType ?? rec.file_role ?? '',
  ).toLowerCase()
}

function backendReferenceAssetVersions(asset: BackendAsset): BackendAssetVersion[] {
  const rec = asset as Record<string, unknown>
  const list = rec.versions
  if (Array.isArray(list)) return list as BackendAssetVersion[]
  const current = rec.current_version ?? rec.currentVersion
  return current && typeof current === 'object' ? [current as BackendAssetVersion] : []
}

function pickReferenceVersionDisplayUrl(version: BackendAssetVersion): string {
  const rec = version as Record<string, unknown>
  const d1 = typeof rec.download_url === 'string' ? rec.download_url.trim() : ''
  const d2 = typeof rec.downloadUrl === 'string' ? rec.downloadUrl.trim() : ''
  const dlRaw = d1 || d2
  const dl = dlRaw ? (toRelativeAssetUrl(dlRaw) ?? dlRaw) : ''
  if (dl) return dl
  const publicUrl = toRelativeAssetUrl(version.public_url) ?? version.public_url
  return publicUrl ?? ''
}

function readRetouchRequirementId(asset: BackendAsset): number | undefined {
  const rec = asset as Record<string, unknown>
  const raw = rec.retouch_requirement_id ?? rec.retouchRequirementId
  if (raw == null || raw === '') return undefined
  const n = typeof raw === 'number' ? raw : Number.parseInt(String(raw), 10)
  if (!Number.isFinite(n) || n <= 0) return undefined
  return n
}

/**
 * Exclude P 图需求级资产，避免与 retouch_requirements[].reference_file_refs / source_assets 重复展示在任务级参考图区。
 */
export function filterTaskLevelBackendReferenceAssets(assets: BackendAsset[]): BackendAsset[] {
  if (!assets.length) return []
  return assets.filter((asset) => readRetouchRequirementId(asset) == null)
}

export function isTaskLevelBackendReferenceAsset(asset: BackendAsset): boolean {
  return readRetouchRequirementId(asset) == null && backendReferenceAssetKind(asset) === 'reference'
}

export function mergeReferenceFileRefsPreferBackend(
  legacyRefs: ReferenceFileRef[],
  backendRefs: ReferenceFileRef[],
): ReferenceFileRef[] {
  if (backendRefs.length > 0) return dedupeReferenceFileRefs(backendRefs)
  return dedupeReferenceFileRefs(legacyRefs)
}

/**
 * Map GET /v1/tasks/{id}/assets reference rows into ReferenceFileRef for ops detail display.
 */
export function referenceFileRefsFromBackendReferenceAssets(assets: BackendAsset[]): ReferenceFileRef[] {
  if (!assets.length) return []
  const out: ReferenceFileRef[] = []
  for (const asset of filterTaskLevelBackendReferenceAssets(assets)) {
    if (backendReferenceAssetKind(asset) !== 'reference') continue
    const assetRec = asset as Record<string, unknown>
    const rootAssetId = trimRefField(assetRec.asset_id ?? assetRec.assetId ?? asset.id)
    for (const version of backendReferenceAssetVersions(asset)) {
      const downloadUrl = pickReferenceVersionDisplayUrl(version)
      if (!downloadUrl) continue
      const verRec = version as Record<string, unknown>
      const versionAssetId = trimRefField(verRec.asset_id ?? verRec.assetId)
      const ref: ReferenceFileRef = { download_url: downloadUrl }
      const assetId = versionAssetId || rootAssetId
      if (assetId) ref.asset_id = assetId
      const refId = trimRefField(verRec.ref_id ?? verRec.storage_ref_id ?? verRec.storageRefId)
      if (refId) ref.ref_id = refId
      const uploadRequestId = trimRefField(verRec.upload_request_id ?? verRec.uploadRequestId)
      if (uploadRequestId) ref.upload_request_id = uploadRequestId
      const storageKey = trimRefField(verRec.storage_key ?? verRec.storageKey)
      if (storageKey) ref.storage_key = storageKey
      const filename = trimRefField(
        verRec.file_name ?? verRec.original_filename ?? verRec.originalFilename,
      )
      if (filename) ref.filename = filename
      const mimeType = trimRefField(verRec.mime_type ?? verRec.mimeType)
      if (mimeType) ref.mime_type = mimeType
      const expiresAt = trimRefField(verRec.download_url_expires_at ?? verRec.downloadUrlExpiresAt)
      if (expiresAt) ref.download_url_expires_at = expiresAt
      const fileSizeRaw = verRec.file_size ?? verRec.fileSize
      if (typeof fileSizeRaw === 'number' && !Number.isNaN(fileSizeRaw)) {
        ref.file_size = fileSizeRaw
      } else if (fileSizeRaw != null && trimRefField(fileSizeRaw)) {
        const parsed = Number(fileSizeRaw)
        if (!Number.isNaN(parsed)) ref.file_size = parsed
      }
      out.push(ref)
    }
  }
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
