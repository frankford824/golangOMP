export interface DownloadTransferMeta {
  downloadUrl: string
  filename: string
  fileSize: number
  mimeType?: string
}

export interface DownloadTransferProgress {
  receivedBytes: number
  totalBytes: number
  speedBytesPerSecond: number
  progress: number
}

export interface DownloadTransferResult {
  mode: 'tracked' | 'browser'
  receivedBytes: number
  totalBytes: number
  speedBytesPerSecond: number
}

interface DownloadTransferRuntime {
  handoff?: (meta: DownloadTransferMeta) => void
}

/** Originals and ZIPs use the native download manager. Fetching the body first
 * can transfer it twice, consumes unbounded memory, and cannot prove it was saved. */
export async function transferDownload(
  meta: DownloadTransferMeta,
  signal: AbortSignal,
  _onProgress: (progress: DownloadTransferProgress) => void,
  runtime: DownloadTransferRuntime = {},
): Promise<DownloadTransferResult> {
  if (signal.aborted) throw new DOMException('下载已取消', 'AbortError')
  const downloadUrl = meta.downloadUrl.trim()
  if (!downloadUrl) throw new Error('当前文件暂时无法下载，请稍后重试')
  const parsed = new URL(downloadUrl, window.location.origin)
  if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') throw new Error('下载地址协议无效')
  if (typeof navigator !== 'undefined' && navigator.onLine === false) {
    throw new Error('网络连接已断开，请恢复网络后重试')
  }
  const filename = meta.filename.split(/[\\/]/).pop()?.trim() || '下载文件'
  ;(runtime.handoff ?? handoffToBrowser)({ ...meta, downloadUrl, filename })
  return {
    mode: 'browser',
    receivedBytes: 0,
    totalBytes: Number.isFinite(meta.fileSize) && meta.fileSize > 0 ? meta.fileSize : 0,
    speedBytesPerSecond: 0,
  }
}

function handoffToBrowser(meta: DownloadTransferMeta) {
  const link = document.createElement('a')
  link.href = meta.downloadUrl
  link.download = meta.filename
  link.target = '_blank'
  link.rel = 'noopener noreferrer'
  document.body.appendChild(link)
  link.click()
  link.remove()
}
