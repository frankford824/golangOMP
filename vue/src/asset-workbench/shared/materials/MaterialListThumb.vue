<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, shallowRef, watch } from 'vue'
import { FileArchive, FileImage, FileText } from 'lucide-vue-next'

import type { SystemAssetRow } from '@aw/shared/api/assetWorkbenchApi'
import {
  resolvedSystemAssetThumbnailUrl,
} from '@aw/shared/materials/systemAssetPreview'

const props = defineProps<{
  asset: SystemAssetRow
  cachedUrl?: string
}>()

const emit = defineEmits<{
  previewNeeded: []
}>()

const previewUrl = shallowRef(resolvedSystemAssetThumbnailUrl(props.asset, props.cachedUrl))
const root = shallowRef<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null
let requested = false

function iconFor() {
  const mime = String(props.asset.mime_type || '').toLowerCase()
  if (mime.includes('zip') || mime.includes('rar') || mime.includes('7z')) return FileArchive
  if (mime.includes('pdf')) return FileText
  return FileImage
}

function syncResolvedPreview() {
  const resolved = resolvedSystemAssetThumbnailUrl(props.asset, props.cachedUrl)
  previewUrl.value = resolved
}

function requestPreview() {
  if (previewUrl.value || requested) return
  requested = true
  emit('previewNeeded')
}

function observeVisibility() {
  observer?.disconnect()
  observer = null
  if (previewUrl.value || !root.value) return
  if (typeof IntersectionObserver === 'undefined') {
    requestPreview()
    return
  }
  observer = new IntersectionObserver((entries) => {
    if (!entries.some((entry) => entry.isIntersecting)) return
    observer?.disconnect()
    requestPreview()
  }, { rootMargin: '25%' })
  observer.observe(root.value)
}

watch(() => [props.asset, props.cachedUrl], async () => {
  syncResolvedPreview()
  if (previewUrl.value) {
    observer?.disconnect()
    return
  }
  requested = false
  await nextTick()
  observeVisibility()
})

onMounted(observeVisibility)
onBeforeUnmount(() => observer?.disconnect())
</script>

<template>
  <span ref="root" class="aw-drive-thumb aw-drive-thumb--sm">
    <img v-if="previewUrl" :src="previewUrl" :alt="asset.original_filename || asset.file_name || ''" loading="lazy" />
    <component :is="iconFor()" v-else :size="18" aria-hidden="true" />
  </span>
</template>
