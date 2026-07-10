import type { SystemAssetDownloadInfo, SystemAssetPreviewMeta } from '@aw/shared/api/assetWorkbenchApi'

export interface PreparedDownloadWaitOptions {
  attempts?: number
  intervalMs?: number
  delay?: (milliseconds: number) => Promise<void>
  onWaiting?: (attempt: number) => void
}

export function downloadIsPreparing(info: SystemAssetDownloadInfo | null | undefined): boolean {
  return Boolean(info && !info.download_url && String(info.access_hint ?? '').includes('prepare_required'))
}

export async function waitForPreparedDownload(
  initial: SystemAssetDownloadInfo,
  refresh: () => Promise<SystemAssetDownloadInfo>,
  options: PreparedDownloadWaitOptions = {},
): Promise<SystemAssetDownloadInfo> {
  if (initial.download_url) return initial
  if (!downloadIsPreparing(initial)) throw new Error('当前文件暂时无法下载，请稍后重试')

  const attempts = Math.max(1, options.attempts ?? 90)
  const intervalMs = Math.max(0, options.intervalMs ?? 2_000)
  const delay = options.delay ?? ((milliseconds: number) => new Promise<void>((resolve) => window.setTimeout(resolve, milliseconds)))
  let lastError: unknown

  for (let attempt = 1; attempt <= attempts; attempt += 1) {
    options.onWaiting?.(attempt)
    await delay(intervalMs)
    let current: SystemAssetDownloadInfo
    try {
      current = await refresh()
    } catch (error) {
      lastError = error
      continue
    }
    if (current.download_url) return current
    if (!downloadIsPreparing(current)) throw new Error('当前文件暂时无法下载，请稍后重试')
  }

  if (lastError instanceof Error) throw lastError
  throw new Error('文件仍在准备中，请稍后再试')
}

export function previewIsPreparing(meta: SystemAssetPreviewMeta | null | undefined): boolean {
  return Boolean(meta && !meta.preview_url && (meta.preparing || meta.status === 'pending'))
}

export async function waitForPreparedPreview(
  initial: SystemAssetPreviewMeta,
  refresh: () => Promise<SystemAssetPreviewMeta>,
  options: PreparedDownloadWaitOptions = {},
): Promise<SystemAssetPreviewMeta> {
  if (initial.preview_url || initial.download_url) return initial
  if (!previewIsPreparing(initial)) return initial

  const attempts = Math.max(1, options.attempts ?? 60)
  const intervalMs = Math.max(0, options.intervalMs ?? 3_000)
  const delay = options.delay ?? ((milliseconds: number) => new Promise<void>((resolve) => window.setTimeout(resolve, milliseconds)))

  for (let attempt = 1; attempt <= attempts; attempt += 1) {
    options.onWaiting?.(attempt)
    await delay(intervalMs)
    const current = await refresh()
    if (current.preview_url || current.download_url || !previewIsPreparing(current)) return current
  }

  throw new Error('预览仍在生成中，请稍后再试')
}
