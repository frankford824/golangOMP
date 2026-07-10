import type { SystemAssetPreviewMeta, SystemAssetRow } from '@aw/shared/api/assetWorkbenchApi'

function normalizedMime(value?: string | null) {
  return String(value || '').trim().toLowerCase()
}

function extensionOf(filename?: string | null) {
  const value = String(filename || '').trim().toLowerCase()
  const match = value.match(/\.([a-z0-9]+)(?:[?#].*)?$/)
  return match?.[1] || ''
}

export function systemAssetFilename(asset: SystemAssetRow | SystemAssetPreviewMeta) {
  if ('filename' in asset && asset.filename) return asset.filename
  if ('original_filename' in asset && asset.original_filename) return asset.original_filename
  if ('file_name' in asset && asset.file_name) return asset.file_name
  return ''
}

export function materialAssetKey(asset: SystemAssetRow | SystemAssetPreviewMeta) {
  const sourceType = 'source_type' in asset ? String(asset.source_type || '').trim() : ''
  const sourceRef = 'source_ref' in asset ? String(asset.source_ref || '').trim() : ''
  const resourceId = 'resource_id' in asset ? String(asset.resource_id || '').trim() : ''
  const numericId = 'asset_id' in asset ? asset.asset_id : asset.id
  if (sourceType === 'external') return resourceId || sourceRef || `external:${numericId}`
  return resourceId || sourceRef || `system:${numericId}`
}

export function isSystemAssetImagePreviewable(asset: SystemAssetRow | SystemAssetPreviewMeta) {
  const mime = normalizedMime(asset.mime_type)
  if (mime.startsWith('image/')) {
    return !mime.includes('photoshop') && !mime.includes('vnd.adobe')
  }
  return ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp', 'svg'].includes(extensionOf(systemAssetFilename(asset)))
}

export function isSystemAssetPdfPreviewable(asset: SystemAssetRow | SystemAssetPreviewMeta) {
  const mime = normalizedMime(asset.mime_type)
  return mime === 'application/pdf' || extensionOf(systemAssetFilename(asset)) === 'pdf'
}

export function isSystemAssetDerivedPreviewCandidate(asset: SystemAssetRow | SystemAssetPreviewMeta) {
  const mime = normalizedMime(asset.mime_type)
  if (mime.includes('photoshop') || mime.includes('illustrator')) return true
  return ['psd', 'psb', 'ai', 'eps', 'ps', 'tif', 'tiff', 'heic', 'heif', 'avif'].includes(extensionOf(systemAssetFilename(asset)))
}

export function isPdfMimeOrFilename(mimeType?: string | null, filename?: string | null) {
  const mime = normalizedMime(mimeType)
  if (mime === 'application/pdf') return true
  return extensionOf(filename) === 'pdf'
}

export function isVideoMimeOrFilename(mimeType?: string | null, filename?: string | null) {
  const mime = normalizedMime(mimeType)
  if (mime.startsWith('video/')) return true
  return ['mp4', 'webm', 'mov', 'm4v'].includes(extensionOf(filename))
}

export function canAttemptSystemAssetPreview(asset: SystemAssetRow | SystemAssetPreviewMeta) {
  if (asset.preview_available) return true
  if (isSystemAssetImagePreviewable(asset)) return true
  if (isSystemAssetPdfPreviewable(asset)) return true
  if (isSystemAssetDerivedPreviewCandidate(asset)) return true
  return isVideoMimeOrFilename(asset.mime_type, systemAssetFilename(asset))
}

export function resolvedSystemAssetPreviewUrl(asset?: SystemAssetRow | SystemAssetPreviewMeta | null) {
  if (!asset) return ''
  if ('preview_url' in asset && asset.preview_url) return asset.preview_url
  if ('download_url' in asset && asset.download_url && isSystemAssetImagePreviewable(asset)) return asset.download_url
  if ('download_url' in asset && asset.download_url && isSystemAssetPdfPreviewable(asset)) return asset.download_url
  if ('download_url' in asset && asset.download_url && isVideoMimeOrFilename(asset.mime_type, systemAssetFilename(asset))) return asset.download_url
  return ''
}

/** Gallery thumbnails: prefer server preview_url only; avoid loading full originals. */
export function resolvedSystemAssetThumbnailUrl(
  asset?: SystemAssetRow | SystemAssetPreviewMeta | null,
  cachedUrl?: string,
) {
  if (cachedUrl) return cachedUrl
  if (!asset) return ''
  if ('preview_url' in asset && asset.preview_url) return asset.preview_url
  return ''
}
