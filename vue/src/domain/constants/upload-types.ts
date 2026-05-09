export const ALLOWED_UPLOAD_EXTENSIONS = [
  'psd',
  'ps',
  'ai',
  'cdr',
  'plt',
  'pdf',
  'jpg',
  'jpeg',
  'png',
  'tif',
  'tiff',
  'webp',
  'bmp',
  'gif',
  'svg',
] as const

export const INLINE_PREVIEWABLE_EXTENSIONS = [
  'jpg',
  'jpeg',
  'png',
  'webp',
  'gif',
  'bmp',
  'svg',
  'pdf',
] as const

export const BITMAP_DELIVERY_EXTENSIONS = ['jpg', 'jpeg', 'png', 'webp'] as const

const ALLOWED_UPLOAD_EXTENSION_SET = new Set<string>(ALLOWED_UPLOAD_EXTENSIONS)
const INLINE_PREVIEWABLE_EXTENSION_SET = new Set<string>(INLINE_PREVIEWABLE_EXTENSIONS)
const BITMAP_DELIVERY_EXTENSION_SET = new Set<string>(BITMAP_DELIVERY_EXTENSIONS)

export const UPLOAD_ACCEPT_ATTRIBUTE = ALLOWED_UPLOAD_EXTENSIONS.map((ext) => `.${ext}`).join(',')

export function getUploadFileExtension(filename: string): string {
  const trimmed = filename.trim()
  if (!trimmed) return ''
  const ext = trimmed.split('.').pop()
  return ext ? ext.toLowerCase() : ''
}

export function isAllowedUploadFile(filename: string): boolean {
  return ALLOWED_UPLOAD_EXTENSION_SET.has(getUploadFileExtension(filename))
}

export function canPreviewUploadInline(filename: string): boolean {
  return INLINE_PREVIEWABLE_EXTENSION_SET.has(getUploadFileExtension(filename))
}

export function isBitmapDeliveryFile(filename: string): boolean {
  return BITMAP_DELIVERY_EXTENSION_SET.has(getUploadFileExtension(filename))
}
