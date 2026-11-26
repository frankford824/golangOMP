<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { FileArchive, ImageOff } from 'lucide-vue-next'

import type { ArchiveVirtualFile } from '@aw/shared/api/assetWorkbenchApi'
import { loadArchiveThumbnail, maxAutoThumbnailBytes } from './archiveThumbnails'
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


const canLoadImage = computed(() => {
  if (!props.file.preview_url || props.file.file_size > maxAutoThumbnailBytes || props.file.file_size <= 0) return false
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
  if (controller || previewUrl.value || failed.value || !canLoadImage.value) return
  loading.value = true
  const request = new AbortController()
  controller = request
  try {
    const url = await loadArchiveThumbnail(() => createArchiveEntryObjectUrl(props.fileId, props.file, request.signal), request.signal)
    if (request.signal.aborted) URL.revokeObjectURL(url)
    else previewUrl.value = url
  } catch {
    if (!request.signal.aborted) failed.value = true
  } finally {
    if (controller === request) {
      loading.value = false
      controller = null
    }
  }
}

onMounted(() => {
  if (!rootRef.value || !canLoadImage.value) return
  observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          void loadPreview()
        } else {
          controller?.abort()
          controller = null
          loading.value = false
          revokePreviewUrl()
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
