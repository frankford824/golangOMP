import http from '@/services/http'

export interface MaterializedPreviewImage {
  displaySrc: string
  objectUrl?: string
  cacheKey?: string
}

interface CachedMaterializedPreviewImage {
  displaySrc: string
  objectUrl?: string
  refCount: number
  expiresAt: number
  lastUsedAt: number
}

const PREVIEW_BLOB_CACHE_TTL_MS = 2 * 60_000
const PREVIEW_BLOB_CACHE_MAX_ENTRIES = 160
const PREVIEW_BLOB_FETCH_CONCURRENCY = 5

let activeBlobFetches = 0
const blobFetchQueue: Array<() => void> = []
const materializedCache = new Map<string, CachedMaterializedPreviewImage>()
const materializedInflight = new Map<string, Promise<CachedMaterializedPreviewImage | undefined>>()

export function normalizePreviewAssetId(raw: string | number | null | undefined): string {
  const value = String(raw ?? '').trim()
  if (!/^\d+$/.test(value)) return ''
  return value
}

export function isSameOriginPreviewUrl(raw: string): boolean {
  const url = raw.trim()
  if (!url) return false
  if (url.startsWith('/')) return true
  if (typeof window === 'undefined') return false
  try {
    return new URL(url, window.location.origin).origin === window.location.origin
  } catch {
    return false
  }
}

export function revokeMaterializedPreviewImage(image: MaterializedPreviewImage | null | undefined): void {
  if (image?.cacheKey) {
    const cached = materializedCache.get(image.cacheKey)
    if (cached) {
      cached.refCount = Math.max(0, cached.refCount - 1)
      cached.lastUsedAt = Date.now()
      if (cached.refCount === 0) {
        cached.expiresAt = Date.now() + PREVIEW_BLOB_CACHE_TTL_MS
      }
      evictMaterializedCache()
      return
    }
  }
  if (!image?.objectUrl) return
  URL.revokeObjectURL(image.objectUrl)
}

function withBlobFetchSlot<T>(worker: () => Promise<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    const run = () => {
      activeBlobFetches += 1
      worker()
        .then(resolve, reject)
        .finally(() => {
          activeBlobFetches = Math.max(0, activeBlobFetches - 1)
          blobFetchQueue.shift()?.()
        })
    }
    if (activeBlobFetches < PREVIEW_BLOB_FETCH_CONCURRENCY) {
      run()
    } else {
      blobFetchQueue.push(run)
    }
  })
}

function retainCachedImage(cacheKey: string, cached: CachedMaterializedPreviewImage): MaterializedPreviewImage {
  cached.refCount += 1
  cached.lastUsedAt = Date.now()
  cached.expiresAt = 0
  return { displaySrc: cached.displaySrc, objectUrl: cached.objectUrl, cacheKey }
}

function ensureCachedImage(cacheKey: string, image: CachedMaterializedPreviewImage): CachedMaterializedPreviewImage {
  const cached = materializedCache.get(cacheKey)
  if (cached) return cached
  materializedCache.set(cacheKey, image)
  evictMaterializedCache()
  return image
}

function evictMaterializedCache(): void {
  const now = Date.now()
  for (const [key, cached] of materializedCache) {
    if (cached.refCount === 0 && cached.expiresAt > 0 && cached.expiresAt <= now) {
      if (cached.objectUrl) URL.revokeObjectURL(cached.objectUrl)
      materializedCache.delete(key)
    }
  }
  if (materializedCache.size <= PREVIEW_BLOB_CACHE_MAX_ENTRIES) return
  const evictable = Array.from(materializedCache.entries())
    .filter(([, cached]) => cached.refCount === 0)
    .sort((a, b) => a[1].lastUsedAt - b[1].lastUsedAt)
  for (const [key, cached] of evictable) {
    if (materializedCache.size <= PREVIEW_BLOB_CACHE_MAX_ENTRIES) break
    if (cached.objectUrl) URL.revokeObjectURL(cached.objectUrl)
    materializedCache.delete(key)
  }
}

async function fetchSameOriginPreview(url: string): Promise<CachedMaterializedPreviewImage | undefined> {
  return withBlobFetchSlot(async () => {
    const res = await http.get<Blob>(url, { responseType: 'blob' })
    const blob = res.data
    if (!(blob instanceof Blob)) return undefined
    const directImageURL = await extractPreviewDownloadURLFromJSONBlob(blob)
    if (directImageURL) {
      return {
        displaySrc: directImageURL,
        refCount: 0,
        expiresAt: 0,
        lastUsedAt: Date.now(),
      }
    }
    const renderableBlob = await renderablePreviewBlob(blob, url)
    if (!renderableBlob) return undefined
    const objectUrl = URL.createObjectURL(renderableBlob)
    return {
      displaySrc: objectUrl,
      objectUrl,
      refCount: 0,
      expiresAt: 0,
      lastUsedAt: Date.now(),
    }
  })
}

async function extractPreviewDownloadURLFromJSONBlob(blob: Blob): Promise<string> {
  const type = (blob.type || '').toLowerCase()
  if (type && !type.includes('json')) return ''
  let parsed: unknown
  try {
    parsed = JSON.parse(await blob.text())
  } catch {
    return ''
  }
  const data = isRecord(parsed) && isRecord(parsed.data) ? parsed.data : parsed
  if (!isRecord(data)) return ''
  if (data.preview_available === false) return ''
  const downloadURL = typeof data.download_url === 'string' ? data.download_url.trim() : ''
  if (!downloadURL) return ''
  const mimeType = typeof data.mime_type === 'string' ? data.mime_type.toLowerCase() : ''
  if (!mimeType.startsWith('image/') && !inferPreviewImageMimeType(downloadURL)) return ''
  return downloadURL
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

async function renderablePreviewBlob(blob: Blob, url: string): Promise<Blob | undefined> {
  const type = (blob.type || '').toLowerCase()
  if (type.startsWith('image/')) return blob
  if (type && type !== 'application/octet-stream' && type !== 'binary/octet-stream') return undefined
  const inferred = inferPreviewImageMimeType(url) || (await sniffPreviewImageMimeType(blob))
  if (!inferred && !isAssetPreviewAPIPath(url)) return undefined
  return new Blob([blob], { type: inferred || 'image/png' })
}

function inferPreviewImageMimeType(raw: string): string {
  let pathname = raw.trim()
  try {
    pathname = new URL(pathname, window.location.origin).pathname
  } catch {
    pathname = pathname.split(/[?#]/, 1)[0] ?? pathname
  }
  let decoded = pathname
  try {
    decoded = decodeURIComponent(pathname)
  } catch {
    decoded = pathname
  }
  const lower = decoded.toLowerCase()
  if (/\.jpe?g$/i.test(lower)) return 'image/jpeg'
  if (/\.png$/i.test(lower)) return 'image/png'
  if (/\.webp$/i.test(lower)) return 'image/webp'
  if (/\.gif$/i.test(lower)) return 'image/gif'
  if (/\.bmp$/i.test(lower)) return 'image/bmp'
  if (/\.svg$/i.test(lower)) return 'image/svg+xml'
  return ''
}

function isAssetPreviewAPIPath(raw: string): boolean {
  let pathname = raw.trim()
  try {
    pathname = new URL(pathname, window.location.origin).pathname
  } catch {
    pathname = pathname.split(/[?#]/, 1)[0] ?? pathname
  }
  return /^\/v1\/assets\/\d+\/preview$/i.test(pathname)
}

async function sniffPreviewImageMimeType(blob: Blob): Promise<string> {
  const header = new Uint8Array(await blob.slice(0, 16).arrayBuffer())
  if (header.length >= 3 && header[0] === 0xff && header[1] === 0xd8 && header[2] === 0xff) {
    return 'image/jpeg'
  }
  if (
    header.length >= 8 &&
    header[0] === 0x89 &&
    header[1] === 0x50 &&
    header[2] === 0x4e &&
    header[3] === 0x47 &&
    header[4] === 0x0d &&
    header[5] === 0x0a &&
    header[6] === 0x1a &&
    header[7] === 0x0a
  ) {
    return 'image/png'
  }
  if (header.length >= 6) {
    const gif = String.fromCharCode(...header.slice(0, 6))
    if (gif === 'GIF87a' || gif === 'GIF89a') return 'image/gif'
  }
  if (header.length >= 12) {
    const riff = String.fromCharCode(...header.slice(0, 4))
    const webp = String.fromCharCode(...header.slice(8, 12))
    if (riff === 'RIFF' && webp === 'WEBP') return 'image/webp'
  }
  if (header.length >= 2 && header[0] === 0x42 && header[1] === 0x4d) return 'image/bmp'
  return ''
}

export async function materializePreviewImageUrl(
  raw: string,
): Promise<MaterializedPreviewImage | undefined> {
  const url = raw.trim()
  if (!url) return undefined
  if (url.startsWith('data:') || url.startsWith('blob:')) {
    return { displaySrc: url }
  }
  if (!isSameOriginPreviewUrl(url)) {
    return { displaySrc: url }
  }
  const cacheKey = url
  const cached = materializedCache.get(cacheKey)
  if (cached) return retainCachedImage(cacheKey, cached)
  const existingInflight = materializedInflight.get(cacheKey)
  if (existingInflight) {
    const inflightResult = await existingInflight
    return inflightResult ? retainCachedImage(cacheKey, ensureCachedImage(cacheKey, inflightResult)) : undefined
  }
  const pending = fetchSameOriginPreview(url)
  materializedInflight.set(cacheKey, pending)
  try {
    const image = await pending
    if (!image) return undefined
    return retainCachedImage(cacheKey, ensureCachedImage(cacheKey, image))
  } catch {
    return undefined
  } finally {
    materializedInflight.delete(cacheKey)
  }
}
