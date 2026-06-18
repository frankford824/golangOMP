import http from '@/services/http'

export interface MaterializedPreviewImage {
  displaySrc: string
  objectUrl?: string
  cacheKey?: string
}

interface CachedMaterializedPreviewImage {
  displaySrc: string
  objectUrl: string
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
      URL.revokeObjectURL(cached.objectUrl)
      materializedCache.delete(key)
    }
  }
  if (materializedCache.size <= PREVIEW_BLOB_CACHE_MAX_ENTRIES) return
  const evictable = Array.from(materializedCache.entries())
    .filter(([, cached]) => cached.refCount === 0)
    .sort((a, b) => a[1].lastUsedAt - b[1].lastUsedAt)
  for (const [key, cached] of evictable) {
    if (materializedCache.size <= PREVIEW_BLOB_CACHE_MAX_ENTRIES) break
    URL.revokeObjectURL(cached.objectUrl)
    materializedCache.delete(key)
  }
}

async function fetchSameOriginPreview(url: string): Promise<CachedMaterializedPreviewImage | undefined> {
  return withBlobFetchSlot(async () => {
    const res = await http.get<Blob>(url, { responseType: 'blob' })
    const blob = res.data
    if (!(blob instanceof Blob)) return undefined
    const type = (blob.type || '').toLowerCase()
    if (type && !type.startsWith('image/')) return undefined
    const objectUrl = URL.createObjectURL(blob)
    return {
      displaySrc: objectUrl,
      objectUrl,
      refCount: 0,
      expiresAt: 0,
      lastUsedAt: Date.now(),
    }
  })
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
