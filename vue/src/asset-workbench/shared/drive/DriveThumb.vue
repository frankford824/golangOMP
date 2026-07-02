<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { FileText, ImageOff } from 'lucide-vue-next'

import { assetWorkbenchApi } from '@aw/shared/api/assetWorkbenchApi'

const props = defineProps<{
  fileId: number
  filename: string
  mimeType?: string
  previewStatus?: string
  size?: 'sm' | 'md'
}>()

const rootRef = ref<HTMLElement | null>(null)
const previewUrl = shallowRef('')
const loading = ref(false)
const failed = ref(false)
let observer: IntersectionObserver | null = null
let controller: AbortController | null = null
let requested = false

const isImage = () => {
  const mime = String(props.mimeType || '').toLowerCase()
  if (mime.startsWith('image/')) return !mime.includes('photoshop') && !mime.includes('vnd.adobe')
  return /\.(jpe?g|png|gif|webp|bmp|svg)$/i.test(props.filename || '')
}

async function loadPreview() {
  if (requested) return
  requested = true
  if (!isImage()) {
    failed.value = true
    return
  }
  loading.value = true
  controller = new AbortController()
  try {
    const meta = await assetWorkbenchApi.getFilePreview(props.fileId, controller.signal)
    if (meta.preview_url) previewUrl.value = meta.preview_url
    else failed.value = true
  } catch (err) {
    if ((err as DOMException)?.name !== 'AbortError') failed.value = true
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (!rootRef.value) return
  observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          void loadPreview()
          observer?.disconnect()
          break
        }
      }
    },
    { rootMargin: '25%' },
  )
  observer.observe(rootRef.value)
})

onBeforeUnmount(() => {
  observer?.disconnect()
  controller?.abort()
})
</script>

<template>
  <span ref="rootRef" class="aw-drive-thumb" :class="{ 'aw-drive-thumb--sm': size === 'sm' }">
    <img v-if="previewUrl" :src="previewUrl" :alt="filename" loading="lazy" />
    <span v-else-if="loading" class="aw-drive-thumb__ph" aria-hidden="true" />
    <FileText v-else-if="!failed" :size="20" aria-hidden="true" />
    <ImageOff v-else :size="20" aria-hidden="true" />
  </span>
</template>
