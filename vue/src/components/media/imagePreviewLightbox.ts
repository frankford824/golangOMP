import type { InjectionKey } from 'vue'

export type ImagePreviewLightboxItem = {
  src: string
  title?: string
  alt?: string
  downloadUrl?: string
}

export type OpenImagePreviewLightboxOptions = {
  title?: string
  items?: ImagePreviewLightboxItem[]
  index?: number
}

export type OpenImagePreviewLightbox = (
  src: string,
  options?: OpenImagePreviewLightboxOptions,
) => void

export const IMAGE_PREVIEW_LIGHTBOX_KEY: InjectionKey<OpenImagePreviewLightbox> = Symbol('image-preview-lightbox')
