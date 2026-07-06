<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { FileArchive, ImageOff } from 'lucide-vue-next'

import type { ArchiveVirtualFile } from '@aw/shared/api/assetWorkbenchApi'
import { createArchiveEntryObjectUrl } from '@aw/shared/drive/archiveEntryBlob'

const props = defineProps<{
  fileId: number
  file: ArchiveVirtualFile
}>()

const rootRef = ref<HTMLElement | null>(null)
const previewUrl = shallowRef('')
const loading = ref(false)
const failed = ref(false)
let observer: IntersectionObserver | null = null
let controller: AbortController | null = null
let requested = false

const canLoadImage = computed(() => {
  const mime = (props.file.mime_type || '').toLowerCase()
  if (mime.startsWith('image/')) return true
  return /\.(jpe?g|png|gif|webp|bmp|svg)$/i.test(props.file.name || props.file.path || '')
})

function revokePreviewUrl() {
  if (!previewUrl.value) return
  URL.revokeObjectURL(previewUrl.value)
  previewUrl.value = ''
}

async function loadPreview() {
  if (requested) return
  requested = true
  if (!canLoadImage.value) {
    failed.value = true
    return
  }
  loading.value = true
  controller = new AbortController()
  try {
    previewUrl.value = await createArchiveEntryObjectUrl(props.fileId, props.file, controller.signal)
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
  revokePreviewUrl()
})
</script>

<template>
  <span ref="rootRef" class="aw-archive-virtual-thumb">
    <img v-if="previewUrl" :src="previewUrl" :alt="file.name" loading="lazy" />
    <span v-else-if="loading" class="aw-drive-thumb__ph" aria-hidden="true" />
    <ImageOff v-else-if="failed" :size="24" aria-hidden="true" />
    <FileArchive v-else :size="26" aria-hidden="true" />
  </span>
</template>
