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

export function isSystemAssetImagePreviewable(asset: SystemAssetRow | SystemAssetPreviewMeta) {
  const mime = normalizedMime(asset.mime_type)
  if (mime.startsWith('image/')) {
    return !mime.includes('photoshop') && !mime.includes('vnd.adobe')
  }
  return ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp', 'svg'].includes(extensionOf(systemAssetFilename(asset)))
}

export function canAttemptSystemAssetPreview(asset: SystemAssetRow | SystemAssetPreviewMeta) {
  if (asset.preview_available) return true
  if (isSystemAssetImagePreviewable(asset)) return true
  const mime = normalizedMime(asset.mime_type)
  return mime === 'application/pdf' || extensionOf(systemAssetFilename(asset)) === 'pdf'
}

export function resolvedSystemAssetPreviewUrl(asset?: SystemAssetRow | SystemAssetPreviewMeta | null) {
  if (!asset) return ''
  if ('preview_url' in asset && asset.preview_url) return asset.preview_url
  if ('download_url' in asset && asset.download_url && isSystemAssetImagePreviewable(asset)) return asset.download_url
  return ''
}
