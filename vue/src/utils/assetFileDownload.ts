/**
 * Single-file download with original upload filename.
 * Reuses naming sanitization aligned with batch zip downloads.
 */
import { fetchAssetDownloadMetaResolved } from '@/domain/asset-access'
import { sanitizeZipEntryName } from '@/utils/batchZipDownload'

export interface DownloadAssetFileOptions {
  assetId?: string
  downloadUrl?: string
  /** reference_file_refs.filename or source original_filename */
  preferredFilename?: string
  signal?: AbortSignal
}

export interface DownloadAssetFileResult {
  ok: boolean
  message?: string
}

const PREVIEW_DERIVATIVE_NAMES = new Set(['preview.webp', 'design-thumb.webp'])

export function parseNumericAssetId(raw: unknown): string | undefined {
  const text = String(raw ?? '').trim()
  if (!text || !/^\d+$/.test(text)) return undefined
  const n = Number.parseInt(text, 10)
  if (!Number.isFinite(n) || n <= 0) return undefined
  return String(n)
}

export function isPreviewDerivativeFilename(filename: string): boolean {
  const base = filename.trim().split(/[/\\]/).pop()?.toLowerCase() ?? ''
  return PREVIEW_DERIVATIVE_NAMES.has(base)
}

/**
 * Pick save-as name: prefer operator upload name; never replace a real name with preview.webp.
 */
export function resolveAssetSaveFilename(preferredFilename: string, metaFilename: string): string {
  const preferred = sanitizeZipEntryName(preferredFilename.trim(), '')
  const meta = sanitizeZipEntryName(metaFilename.trim(), '')
  if (preferred) {
    if (meta && isPreviewDerivativeFilename(meta) && !isPreviewDerivativeFilename(preferred)) {
      return preferred
    }
    return preferred
  }
  if (meta && !isPreviewDerivativeFilename(meta)) return meta
  if (meta) return meta
  return 'download'
}

export function triggerBrowserURLDownload(downloadUrl: string, filename: string, signal?: AbortSignal) {
  if (signal?.aborted) throw signal.reason ?? new DOMException('Aborted', 'AbortError')
  const link = document.createElement('a')
  link.href = downloadUrl
  link.download = filename
  link.rel = 'noopener'
  document.body.appendChild(link)
  link.click()
  link.remove()
}

/**
 * Download one file (no zip). Prefer GET /v1/assets/{id}/download for canonical filename + URL.
 */
export async function downloadAssetFileWithOriginalFilename(
  options: DownloadAssetFileOptions,
): Promise<DownloadAssetFileResult> {
  const numericAssetId = parseNumericAssetId(options.assetId)
  const preferredFilename = String(options.preferredFilename ?? '').trim()
  const fallbackUrl = String(options.downloadUrl ?? '').trim()

  if (numericAssetId) {
    const meta = await fetchAssetDownloadMetaResolved(numericAssetId, options.signal)
    if (meta.status === 'not_found') return { ok: false, message: '资源不存在' }
    if (meta.status === 'forbidden') return { ok: false, message: '无权限下载该资源' }
    if (meta.status !== 'ok' || !meta.downloadUrl) {
      if (!fallbackUrl) {
        return { ok: false, message: meta.message ?? '获取下载地址失败' }
      }
    } else {
      const saveName = resolveAssetSaveFilename(meta.filename ?? '', preferredFilename)
      try {
        triggerBrowserURLDownload(meta.downloadUrl, saveName, options.signal)
        return { ok: true }
      } catch (err) {
        const msg = err instanceof Error ? err.message : '下载失败'
        return { ok: false, message: msg }
      }
    }
  }

  if (!fallbackUrl) {
    return { ok: false, message: '缺少可下载地址' }
  }
  const saveName = resolveAssetSaveFilename(
    preferredFilename,
    filenameFromUrl(fallbackUrl),
  )
  if (!saveName || saveName === 'download') {
    return { ok: false, message: '缺少原始文件名，无法保存' }
  }
  if (isPreviewDerivativeFilename(saveName) && preferredFilename && !isPreviewDerivativeFilename(preferredFilename)) {
    return { ok: false, message: '无法使用预览文件代替原始素材，请刷新页面后重试' }
  }
  try {
    triggerBrowserURLDownload(fallbackUrl, saveName, options.signal)
    return { ok: true }
  } catch (err) {
    const msg = err instanceof Error ? err.message : '下载失败'
    return { ok: false, message: msg }
  }
}

function filenameFromUrl(raw: string): string {
  const trimmed = raw.trim()
  if (!trimmed) return ''
  try {
    const path = new URL(trimmed, window.location.origin).pathname
    const segment = path.split('/').pop() ?? ''
    return decodeURIComponent(segment)
  } catch {
    const clean = trimmed.split('?')[0].split('#')[0]
    return decodeURIComponent(clean.split('/').pop() ?? '')
  }
}
