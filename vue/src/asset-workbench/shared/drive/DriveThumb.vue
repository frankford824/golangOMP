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
let retryTimer: number | null = null
let retryCount = 0
let requested = false
const MAX_PREVIEW_RETRIES = 6

const isImage = () => {
  const mime = String(props.mimeType || '').toLowerCase()
  if (mime === 'application/pdf') return true
  if (mime.startsWith('image/')) return !mime.includes('photoshop') && !mime.includes('vnd.adobe')
  return /\.(jpe?g|png|gif|webp|bmp|svg|pdf)$/i.test(props.filename || '')
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
    if (meta.preview_url) {
      previewUrl.value = meta.preview_url
      return
    }
    if (meta.preparing || meta.status === 'pending' || meta.status === 'processing') {
      requested = false
      if (retryCount < MAX_PREVIEW_RETRIES) {
        const delay = Math.min(12_000, 1_500 * 2 ** retryCount)
        retryCount += 1
        retryTimer = window.setTimeout(() => {
          retryTimer = null
          void loadPreview()
        }, delay)
      } else {
        failed.value = true
      }
      return
    }
    failed.value = true
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
  if (retryTimer != null) window.clearTimeout(retryTimer)
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
