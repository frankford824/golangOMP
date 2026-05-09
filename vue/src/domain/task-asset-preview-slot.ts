import type { TaskAssetVersion } from './types/task'
import { toRelativeAssetUrl } from '@/utils/url'

const RASTER_PREVIEW_EXT = /\.(jpe?g|png|webp|gif)$/i

/**
 * 大图区是否走 `AssetPreviewMedia`（含：栅格 fileRefs，或 PSD 等 nonPreview 但带 `assetRootId` 时走 GET /v1/assets/{id}/preview）。
 */
export function slotUsesAssetPreviewMedia(v: TaskAssetVersion, fileIdx: number): boolean {
  const r = v.fileRefs?.length ?? 0
  const np = v.nonPreviewFiles?.length ?? 0
  if (fileIdx < r) return true
  if (fileIdx >= r && fileIdx < r + np) {
    if (v.previewAvailable === false) return false
    return !!(v.assetRootId && String(v.assetRootId).trim())
  }
  return false
}

/** 仅栅格 URL，避免把 PSD 直通地址当作 `<img>` fallback */
export function rasterFallbackForAssetPreviewSlot(
  v: TaskAssetVersion,
  fileIdx: number,
): string | undefined {
  const r = v.fileRefs?.length ?? 0
  if (fileIdx < r) {
    const u = v.fileRefs[fileIdx]
    return u?.trim() ? u : undefined
  }
  const np = v.nonPreviewFiles?.[fileIdx - r]
  const u = np?.url?.trim()
  if (!u || !RASTER_PREVIEW_EXT.test(u)) return undefined
  return toRelativeAssetUrl(u) ?? u
}

/** 直链下载（若仅有 assetRootId，下载组件仍可走 /download） */
export function downloadHrefForAssetPreviewSlot(
  v: TaskAssetVersion,
  fileIdx: number,
): string | undefined {
  const r = v.fileRefs?.length ?? 0
  if (fileIdx < r) return v.fileRefs[fileIdx]?.trim() || undefined
  return v.nonPreviewFiles?.[fileIdx - r]?.url?.trim() || undefined
}
