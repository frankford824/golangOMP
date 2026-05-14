export interface BatchZipDownloadSource {
  key: string
  filename?: string
  downloadURL?: string
  fallbackName?: string
  failureHint?: string
}

export interface BatchZipDownloadOptions {
  items: BatchZipDownloadSource[]
  zipFilename: string
  serverFailures?: string[]
  concurrency?: number
  onStatus?: (message: string) => void
}

export interface BatchZipDownloadResult {
  writtenCount: number
  failureCount: number
  failures: string[]
}

interface ZipGenerateMetadata {
  percent: number
}

export function sanitizeZipEntryName(name: string, fallback: string): string {
  const cleaned = name
    .trim()
    .replace(/[\\/:*?"<>|\u0000-\u001f]/g, '_')
    .replace(/\.\./g, '_')
  return cleaned || fallback
}

export function ensureUniqueZipEntryName(filename: string, usedNames: Map<string, number>): string {
  const normalized = filename.trim() || 'asset'
  const count = (usedNames.get(normalized) ?? 0) + 1
  usedNames.set(normalized, count)
  if (count === 1) return normalized

  const dotIndex = normalized.lastIndexOf('.')
  if (dotIndex <= 0) return `${normalized} (${count})`
  return `${normalized.slice(0, dotIndex)} (${count})${normalized.slice(dotIndex)}`
}

export async function mapWithConcurrency<T, R>(
  items: T[],
  concurrency: number,
  worker: (item: T, index: number) => Promise<R>,
): Promise<R[]> {
  if (!items.length) return []
  const results = new Array<R>(items.length)
  let cursor = 0
  const workerCount = Math.min(Math.max(1, concurrency), items.length)

  await Promise.all(
    Array.from({ length: workerCount }, async () => {
      for (;;) {
        const index = cursor
        cursor += 1
        if (index >= items.length) return
        results[index] = await worker(items[index], index)
      }
    }),
  )
  return results
}

export function buildTimestampedZipFilename(prefix: string): string {
  const now = new Date()
  const pad = (value: number) => String(value).padStart(2, '0')
  const stamp = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`
  return `${prefix}-${stamp}.zip`
}

function downloadBlob(blob: Blob, filename: string) {
  const objectURL = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = objectURL
  link.download = filename
  link.rel = 'noopener'
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.setTimeout(() => URL.revokeObjectURL(objectURL), 1000)
}

export async function downloadBatchAsZip(options: BatchZipDownloadOptions): Promise<BatchZipDownloadResult> {
  const { default: JSZip } = await import('jszip')
  const zip = new JSZip()
  const usedNames = new Map<string, number>()
  const failures: string[] = [...(options.serverFailures ?? [])]
  const total = options.items.length
  let completed = 0
  let writtenCount = 0
  const concurrency = Math.min(Math.max(1, options.concurrency ?? 4), 8)

  await mapWithConcurrency(options.items, concurrency, async (item) => {
    const key = String(item.key ?? '').trim() || 'item'
    const downloadURL = String(item.downloadURL ?? '').trim()
    const fallback = String(item.fallbackName ?? '').trim() || key
    const filename = ensureUniqueZipEntryName(
      sanitizeZipEntryName(item.filename ?? '', fallback),
      usedNames,
    )
    if (!downloadURL) {
      failures.push(item.failureHint || `key=${key} filename=${filename} reason=missing_download_url`)
      completed += 1
      options.onStatus?.(`正在下载并打包 ${completed}/${total}`)
      return
    }
    try {
      const response = await fetch(downloadURL, { credentials: 'omit', mode: 'cors' })
      if (!response.ok) {
        failures.push(item.failureHint || `key=${key} filename=${filename} reason=http_${response.status}`)
        return
      }
      const blob = await response.blob()
      zip.file(filename, blob, { binary: true, compression: 'STORE' })
      writtenCount += 1
    } catch (err) {
      const reason = err instanceof Error ? err.message : 'fetch_failed'
      failures.push(item.failureHint || `key=${key} filename=${filename} reason=${reason}`)
    } finally {
      completed += 1
      options.onStatus?.(`正在下载并打包 ${completed}/${total}`)
    }
  })

  if (failures.length > 0) {
    zip.file('download_errors.txt', failures.join('\n') + '\n')
  }
  if (writtenCount === 0) {
    throw new Error('没有文件成功写入 ZIP')
  }

  options.onStatus?.('正在生成 ZIP')
  const blob = await zip.generateAsync(
    {
      type: 'blob',
      compression: 'STORE',
      streamFiles: true,
    },
    (metadata: ZipGenerateMetadata) => {
      options.onStatus?.(`正在生成 ZIP ${Math.floor(metadata.percent)}%`)
    },
  )
  downloadBlob(blob, options.zipFilename)

  return {
    writtenCount,
    failureCount: failures.length,
    failures,
  }
}
