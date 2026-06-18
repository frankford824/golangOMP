import http from '@/services/http'

export interface MaterializedPreviewImage {
  displaySrc: string
  objectUrl?: string
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
  if (!image?.objectUrl) return
  URL.revokeObjectURL(image.objectUrl)
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
  try {
    const res = await http.get<Blob>(url, { responseType: 'blob' })
    const blob = res.data
    if (!(blob instanceof Blob)) return undefined
    const type = (blob.type || '').toLowerCase()
    if (type && !type.startsWith('image/')) return undefined
    const objectUrl = URL.createObjectURL(blob)
    return { displaySrc: objectUrl, objectUrl }
  } catch {
    return undefined
  }
}
