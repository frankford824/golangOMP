/** 浏览器未识别 type 时按扩展名兜底（如 .psd 上传 mime_type 常为空） */
const EXT_MIME: Record<string, string> = {
  psd: 'image/vnd.adobe.photoshop',
  ai: 'application/postscript',
  zip: 'application/zip',
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
  png: 'image/png',
  gif: 'image/gif',
  webp: 'image/webp',
  bmp: 'image/bmp',
  heic: 'image/heic',
  heif: 'image/heif',
  tiff: 'image/tiff',
  tif: 'image/tiff',
}

export function guessMimeTypeFromFilename(filename: string): string | undefined {
  const ext = filename.split('.').pop()?.toLowerCase()
  if (!ext) return undefined
  return EXT_MIME[ext]
}

export function resolveFileMimeType(file: File): string {
  if (file.type && file.type.trim()) return file.type
  return guessMimeTypeFromFilename(file.name) ?? 'application/octet-stream'
}
