const maxHeaderBytes = 256 * 1024

interface PixelDimensions {
  width: number
  height: number
}

export function originalImageDimensions(bytes: Uint8Array): PixelDimensions | undefined {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
  const u16 = (offset: number) => view.getUint16(offset)
  const u32 = (offset: number) => view.getUint32(offset)
  const u24le = (offset: number) => bytes[offset] + 256 * bytes[offset + 1] + 65536 * bytes[offset + 2]
  const matches = (offset: number, signature: string) => [...signature].every((char, index) => bytes[offset + index] === char.charCodeAt(0))
  const size = (width: number, height: number) => width > 0 && height > 0 ? { width, height } : undefined

  if (bytes.length >= 24 && matches(0, '\x89PNG\r\n\x1a\n') && u32(8) === 13 && matches(12, 'IHDR')) {
    return size(u32(16), u32(20))
  }
  if (bytes.length >= 13 && (matches(0, 'GIF87a') || matches(0, 'GIF89a'))) {
    return size(view.getUint16(6, true), view.getUint16(8, true))
  }
  if (bytes.length >= 26 && matches(0, '8BPS') && [1, 2].includes(u16(4))) {
    return size(u32(18), u32(14))
  }
  if (bytes.length >= 30 && matches(0, 'RIFF') && matches(8, 'WEBP')) {
    if (matches(12, 'VP8X') && view.getUint32(16, true) === 10) return size(u24le(24) + 1, u24le(27) + 1)
    if (matches(12, 'VP8L') && view.getUint32(16, true) >= 5 && bytes[20] === 0x2f) {
      const packed = view.getUint32(21, true)
      return size((packed & 0x3fff) + 1, ((packed >>> 14) & 0x3fff) + 1)
    }
    if (matches(12, 'VP8 ') && view.getUint32(16, true) >= 10 && matches(23, '\x9d\x01\x2a')) {
      return size(view.getUint16(26, true) & 0x3fff, view.getUint16(28, true) & 0x3fff)
    }
  }
  if (bytes.length < 4 || u16(0) !== 0xffd8) return undefined

  // JPEG dimensions live in the first start-of-frame segment. Skip APP/EXIF
  // segments by length, including embedded thumbnail images.
  let offset = 2
  while (offset + 3 < bytes.length && bytes[offset++] === 0xff) {
    while (bytes[offset] === 0xff) offset++
    if (offset + 2 >= bytes.length) break
    const marker = bytes[offset++]
    if (marker === 0xda || marker === 0xd9) break
    const length = u16(offset)
    if (length < 2) break
    if (marker >= 0xc0 && marker <= 0xcf && ![0xc4, 0xc8, 0xcc].includes(marker)) {
      if (length < 8 || offset + 7 > bytes.length) break
      return size(u16(offset + 5), u16(offset + 3))
    }
    offset += length
  }
}

function isOriginalURL(url: URL): boolean {
  // Reject known rendition URLs even when a legacy record or redirect mislabels one.
  return ['https:', 'http:'].includes(url.protocol) && !url.username && !url.password &&
    !url.searchParams.has('x-oss-process') && !url.searchParams.has('rendition') &&
    !/\/(?:preview|thumbnail)(?:\/|$)/i.test(url.pathname)
}

export async function readOriginalImageDimensions(rawURL: string, signal: AbortSignal): Promise<PixelDimensions | undefined> {
  const url = new URL(rawURL, window.location.href)
  if (!isOriginalURL(url)) return undefined

  const response = await fetch(url, {
    headers: { Range: `bytes=0-${maxHeaderBytes - 1}` },
    credentials: 'same-origin',
    referrerPolicy: 'no-referrer',
    signal,
  })
  if (!response.body) return undefined
  const reader = response.body.getReader()
  try {
    if (response.status !== 200 && response.status !== 206) return undefined
    if (response.redirected && !isOriginalURL(new URL(response.url))) return undefined
    const contentRange = response.headers.get('Content-Range')
    if (contentRange && !/^bytes 0-\d+\//i.test(contentRange)) return undefined
    const header = new Uint8Array(maxHeaderBytes)
    let length = 0
    // Some storage gateways ignore Range. Stop reading as soon as we know the
    // size, then cancel the stream even when the response is a full-file 200.
    while (length < header.length) {
      const { done, value } = await reader.read()
      if (done) break
      const chunk = value.subarray(0, header.length - length)
      header.set(chunk, length)
      length += chunk.length
      const dimensions = originalImageDimensions(header.subarray(0, length))
      if (dimensions) return dimensions
    }
  } finally {
    await reader.cancel().catch(() => undefined)
  }
}
