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
  fetcher?: typeof fetch
  handoff?: (meta: DownloadTransferMeta) => void
  now?: () => number
  saveBlob?: (blob: Blob, filename: string) => void
  maxBufferedBytes?: number
  progressIntervalMs?: number
}

const DEFAULT_MAX_BUFFERED_BYTES = 768 * 1024 * 1024

export async function transferDownload(
  meta: DownloadTransferMeta,
  signal: AbortSignal,
  onProgress: (progress: DownloadTransferProgress) => void,
  runtime: DownloadTransferRuntime = {},
): Promise<DownloadTransferResult> {
  const downloadUrl = meta.downloadUrl.trim()
  if (!downloadUrl) throw new Error('当前文件暂时无法下载，请稍后重试')

  const maxBufferedBytes = positiveNumber(runtime.maxBufferedBytes) || DEFAULT_MAX_BUFFERED_BYTES
  const expectedBytes = positiveNumber(meta.fileSize)
  const handoff = runtime.handoff ?? handoffToBrowser
  if (expectedBytes > maxBufferedBytes) {
    handoff(meta)
    return { mode: 'browser', receivedBytes: 0, totalBytes: expectedBytes, speedBytesPerSecond: 0 }
  }

  const fetcher = runtime.fetcher ?? fetch
  let response: Response
  try {
    response = await fetcher(downloadUrl, {
      cache: 'no-store',
      credentials: 'same-origin',
      signal,
    })
  } catch (error) {
    if (signal.aborted) throw error
    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      throw new Error('网络连接已断开，请恢复网络后重试')
    }
    handoff(meta)
    return { mode: 'browser', receivedBytes: 0, totalBytes: expectedBytes, speedBytesPerSecond: 0 }
  }

  if (!response.ok) throw new Error('下载请求失败（' + response.status + '）')

  const responseBytes = positiveNumber(response.headers.get('content-length'))
  const totalBytes = expectedBytes || responseBytes
  if (totalBytes > maxBufferedBytes) {
    await response.body?.cancel()
    handoff(meta)
    return { mode: 'browser', receivedBytes: 0, totalBytes, speedBytesPerSecond: 0 }
  }

  const now = runtime.now ?? (() => performance.now())
  const startedAt = now()
  let lastSampleAt = startedAt
  let lastSampleBytes = 0
  let lastEmitAt = startedAt
  let receivedBytes = 0
  let speedBytesPerSecond = 0
  const chunks: BlobPart[] = []
  const intervalMs = Math.max(0, runtime.progressIntervalMs ?? 250)

  if (!response.body) {
    const blob = await response.blob()
    receivedBytes = blob.size
    const elapsedMs = Math.max(1, now() - startedAt)
    speedBytesPerSecond = (receivedBytes * 1000) / elapsedMs
    onProgress(progressSnapshot(receivedBytes, totalBytes || receivedBytes, speedBytesPerSecond, true))
    ;(runtime.saveBlob ?? saveBlob)(blob, meta.filename)
    return { mode: 'tracked', receivedBytes, totalBytes: totalBytes || receivedBytes, speedBytesPerSecond }
  }

  const reader = response.body.getReader()
  for (;;) {
    const result = await reader.read()
    if (result.done) break
    if (!result.value?.byteLength) continue

    receivedBytes += result.value.byteLength
    if (receivedBytes > maxBufferedBytes) {
      await reader.cancel()
      handoff(meta)
      return { mode: 'browser', receivedBytes: 0, totalBytes: totalBytes || expectedBytes, speedBytesPerSecond: 0 }
    }
    const bufferedChunk = new Uint8Array(result.value.byteLength)
    bufferedChunk.set(result.value)
    chunks.push(bufferedChunk)

    const sampledAt = now()
    const sampleElapsedMs = sampledAt - lastSampleAt
    if (sampleElapsedMs >= intervalMs) {
      const instantSpeed = ((receivedBytes - lastSampleBytes) * 1000) / Math.max(1, sampleElapsedMs)
      speedBytesPerSecond = speedBytesPerSecond > 0
        ? speedBytesPerSecond * 0.65 + instantSpeed * 0.35
        : instantSpeed
      lastSampleAt = sampledAt
      lastSampleBytes = receivedBytes
    }
    if (sampledAt - lastEmitAt >= intervalMs) {
      onProgress(progressSnapshot(receivedBytes, totalBytes, speedBytesPerSecond, false))
      lastEmitAt = sampledAt
    }
  }

  const elapsedMs = Math.max(1, now() - startedAt)
  if (speedBytesPerSecond <= 0) speedBytesPerSecond = (receivedBytes * 1000) / elapsedMs
  const resolvedTotalBytes = totalBytes || receivedBytes
  onProgress(progressSnapshot(receivedBytes, resolvedTotalBytes, speedBytesPerSecond, true))
  const blob = new Blob(chunks, { type: meta.mimeType || response.headers.get('content-type') || 'application/octet-stream' })
  ;(runtime.saveBlob ?? saveBlob)(blob, meta.filename)
  return { mode: 'tracked', receivedBytes, totalBytes: resolvedTotalBytes, speedBytesPerSecond }
}

function progressSnapshot(
  receivedBytes: number,
  totalBytes: number,
  speedBytesPerSecond: number,
  completed: boolean,
): DownloadTransferProgress {
  const progress = totalBytes > 0
    ? Math.min(completed ? 100 : 99, Math.round((receivedBytes / totalBytes) * 100))
    : completed ? 100 : 0
  return { receivedBytes, totalBytes, speedBytesPerSecond, progress }
}

function positiveNumber(value: unknown): number {
  const number = Number(value)
  return Number.isFinite(number) && number > 0 ? number : 0
}

function safeFilename(filename: string): string {
  return filename.split(/[\\/]/).pop()?.trim() || '下载文件'
}

function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = safeFilename(filename)
  link.rel = 'noopener'
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 30_000)
}

function handoffToBrowser(meta: DownloadTransferMeta) {
  const link = document.createElement('a')
  link.href = meta.downloadUrl
  link.download = safeFilename(meta.filename)
  link.target = '_blank'
  link.rel = 'noopener noreferrer'
  document.body.appendChild(link)
  link.click()
  link.remove()
}
