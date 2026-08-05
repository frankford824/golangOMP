import type JSZipType from 'jszip'

export interface BatchZipDownloadSource {
  key: string
  filename?: string
  /** Optional path inside ZIP, e.g. `需求1/参考图/foo.png`. Slashes sanitized when set. */
  zipPath?: string
  downloadURL?: string
  fallbackName?: string
  failureHint?: string
}

export interface BatchZipDownloadOptions {
  items: BatchZipDownloadSource[]
  zipFilename: string
  serverFailures?: string[]
  concurrency?: number
  /** Normalize nested ZIP entry names and remove macOS metadata in the derived download only. */
  normalizeNestedZipFilenames?: boolean
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

type JSZipConstructor = typeof JSZipType

const NESTED_ZIP_NORMALIZE_MAX_BYTES = 64 * 1024 * 1024

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

export function resolveBatchDownloadCredentials(
  rawURL: string,
  pageOrigin = typeof window !== 'undefined' ? window.location.origin : 'http://localhost',
): RequestCredentials {
  const trimmed = rawURL.trim()
  if (!trimmed) return 'same-origin'
  try {
    const target = new URL(trimmed, pageOrigin)
    return target.origin === new URL(pageOrigin).origin ? 'same-origin' : 'omit'
  } catch {
    // A relative compatibility path still belongs to the authenticated app.
    return 'same-origin'
  }
}

export function assertUsableBatchDownloadPayload(size: number, contentType: string): void {
  if (!Number.isFinite(size) || size <= 0) {
    throw new Error('downloaded_file_is_empty')
  }
  const normalizedType = contentType.trim().toLowerCase().split(';', 1)[0]
  if (
    normalizedType === 'application/json' ||
    normalizedType === 'application/problem+json' ||
    normalizedType === 'application/xml' ||
    normalizedType === 'text/xml' ||
    normalizedType === 'text/html'
  ) {
    throw new Error(`downloaded_error_payload_${normalizedType.replace(/[^a-z0-9]+/g, '_')}`)
  }
}

export function decodeNestedZipEntryFilename(bytes: Uint8Array | string[]): string {
  const input = bytes instanceof Uint8Array
    ? bytes
    : Uint8Array.from(bytes, (value) => value.charCodeAt(0) & 0xff)
  for (const encoding of ['utf-8', 'gb18030']) {
    try {
      return new TextDecoder(encoding, { fatal: true }).decode(input)
    } catch {
      // Try the next common Chinese ZIP filename encoding.
    }
  }
  return Array.from(input, (value) => String.fromCharCode(value)).join('')
}

function isMacOSZipMetadataPath(path: string): boolean {
  const normalized = path.replace(/\\/g, '/').replace(/^\/+/, '')
  if (normalized === '__MACOSX' || normalized.startsWith('__MACOSX/')) return true
  return normalized.split('/').pop() === '.DS_Store'
}

export async function normalizeNestedZipBlob(
  blob: Blob,
  filename: string,
  JSZipImpl?: JSZipConstructor,
): Promise<Blob> {
  if (!filename.trim().toLowerCase().endsWith('.zip')) return blob
  if (blob.size <= 0 || blob.size > NESTED_ZIP_NORMALIZE_MAX_BYTES) return blob

  try {
    const JSZip = JSZipImpl ?? (await import('jszip')).default
    const nestedZip = await JSZip.loadAsync(await blob.arrayBuffer(), {
      checkCRC32: true,
      decodeFileName: decodeNestedZipEntryFilename,
    })
    for (const path of Object.keys(nestedZip.files)) {
      if (isMacOSZipMetadataPath(path)) nestedZip.remove(path)
    }
    const normalized = await nestedZip.generateAsync({
      type: 'uint8array',
      compression: 'DEFLATE',
      compressionOptions: { level: 6 },
      streamFiles: true,
    })
    const normalizedBuffer = normalized.buffer.slice(
      normalized.byteOffset,
      normalized.byteOffset + normalized.byteLength,
    ) as ArrayBuffer
    return new Blob([normalizedBuffer], { type: 'application/zip' })
  } catch {
    // Preserve encrypted, unsupported, corrupt, or unusually encoded source
    // archives byte-for-byte instead of turning normalization into data loss.
    return blob
  }
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
  const blobCache = new Map<string, Promise<Blob>>()

  const loadBlob = (downloadURL: string) => {
    const cached = blobCache.get(downloadURL)
    if (cached) return cached
    const pending = (async () => {
      const response = await fetch(downloadURL, {
        credentials: resolveBatchDownloadCredentials(downloadURL),
        mode: 'cors',
      })
      if (!response.ok) throw new Error(`http_${response.status}`)
      const blob = await response.blob()
      assertUsableBatchDownloadPayload(blob.size, response.headers.get('content-type') ?? blob.type)
      return blob
    })()
    blobCache.set(downloadURL, pending)
    return pending
  }

  await mapWithConcurrency(options.items, concurrency, async (item) => {
    const key = String(item.key ?? '').trim() || 'item'
    const downloadURL = String(item.downloadURL ?? '').trim()
    const fallback = String(item.fallbackName ?? '').trim() || key
    const baseName = sanitizeZipEntryName(item.filename ?? '', fallback)
    const zipPathRaw = String(item.zipPath ?? '').trim()
    const entryPath = zipPathRaw
      ? zipPathRaw
          .split(/[/\\]+/)
          .filter(Boolean)
          .map((segment) => sanitizeZipEntryName(segment, 'file'))
          .join('/')
      : ''
    const filename = ensureUniqueZipEntryName(
      entryPath ? `${entryPath}/${baseName}` : baseName,
      usedNames,
    )
    if (!downloadURL) {
      failures.push(item.failureHint || `key=${key} filename=${filename} reason=missing_download_url`)
      completed += 1
      options.onStatus?.(`正在下载并打包 ${completed}/${total}`)
      return
    }
    try {
      const blob = await loadBlob(downloadURL)
      const payload = options.normalizeNestedZipFilenames
        ? await normalizeNestedZipBlob(blob, filename, JSZip)
        : blob
      zip.file(filename, payload, { binary: true, compression: 'STORE' })
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
