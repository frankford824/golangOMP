<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { FileArchive, ImageOff } from 'lucide-vue-next'

import type { ArchiveVirtualFile } from '@aw/shared/api/assetWorkbenchApi'
import { loadArchiveThumbnail, maxAutoThumbnailBytes } from './archiveThumbnails'
import { createArchiveEntryObjectUrl } from '@aw/shared/drive/archiveEntryBlob'

const props = defineProps<{
  fileId: number
  paused?: boolean
  file: ArchiveVirtualFile
}>()

const rootRef = ref<HTMLElement | null>(null)
const previewUrl = shallowRef('')
const loading = ref(false)
const failed = ref(false)
let observer: IntersectionObserver | null = null
let visible = false
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
  if (props.paused || !visible || controller || previewUrl.value || failed.value || !canLoadImage.value) return
  loading.value = true
  const request = new AbortController()
  controller = request
  try {
    const url = await loadArchiveThumbnail(() => createArchiveEntryObjectUrl(props.fileId, props.file, request.signal, 'thumbnail'), request.signal)
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

function pauseRequest() {
  controller?.abort()
  controller = null
  loading.value = false
}

watch(() => props.paused, (paused) => {
  if (paused) pauseRequest()
  else void loadPreview()
})

onMounted(() => {
  if (!rootRef.value || !canLoadImage.value) return
  observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        visible = entry.isIntersecting
        if (visible) {
          void loadPreview()
        } else {
          pauseRequest()
          if (!props.paused) revokePreviewUrl()
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
    <img v-if="previewUrl" :src="previewUrl" :alt="file.name" decoding="async" />
    <span v-else-if="loading" class="aw-drive-thumb__ph" aria-hidden="true" />
    <ImageOff v-else-if="failed" :size="24" aria-hidden="true" />
    <FileArchive v-else :size="26" aria-hidden="true" />
  </span>
</template>
