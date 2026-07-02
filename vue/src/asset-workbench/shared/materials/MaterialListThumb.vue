<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { FileArchive, FileImage, FileText, ImageOff } from 'lucide-vue-next'

import { assetWorkbenchApi, type SystemAssetRow } from '@aw/shared/api/assetWorkbenchApi'
import {
  canAttemptSystemAssetPreview,
  isSystemAssetImagePreviewable,
  materialAssetKey,
  resolvedSystemAssetThumbnailUrl,
} from '@aw/shared/materials/systemAssetPreview'

const props = defineProps<{
  asset: SystemAssetRow
  cachedUrl?: string
}>()

const emit = defineEmits<{
  loaded: [key: string, url: string]
}>()

const rootRef = ref<HTMLElement | null>(null)
const previewUrl = shallowRef(resolvedSystemAssetThumbnailUrl(props.asset, props.cachedUrl))
const loading = ref(false)
const failed = ref(false)
let observer: IntersectionObserver | null = null
let requested = false
let previewKey = materialAssetKey(props.asset)

function iconFor() {
  const mime = String(props.asset.mime_type || '').toLowerCase()
  if (mime.includes('zip') || mime.includes('rar') || mime.includes('7z')) return FileArchive
  if (mime.includes('pdf')) return FileText
  return FileImage
}

async function loadPreview() {
  if (requested) return
  requested = true
  const asset = props.asset
  const key = materialAssetKey(asset)
  if (previewUrl.value) return
  if (!canAttemptSystemAssetPreview(asset) || !isSystemAssetImagePreviewable(asset)) {
    failed.value = true
    return
  }
  loading.value = true
  try {
    const meta = asset.material_id
      ? await assetWorkbenchApi.previewClientMaterial(asset.material_id)
      : await assetWorkbenchApi.previewMaterialAsset(asset)
    const url = meta.preview_url || ''
    if (url) {
      previewUrl.value = url
      emit('loaded', key, url)
    } else {
      failed.value = true
    }
  } catch {
    failed.value = true
  } finally {
    loading.value = false
  }
}

function syncResolvedPreview() {
  const key = materialAssetKey(props.asset)
  const resolved = resolvedSystemAssetThumbnailUrl(props.asset, props.cachedUrl)
  if (key !== previewKey) {
    previewKey = key
    requested = Boolean(resolved)
    failed.value = false
    loading.value = false
    previewUrl.value = resolved
    return
  }
  if (resolved && resolved !== previewUrl.value) {
    previewUrl.value = resolved
    requested = true
    failed.value = false
  }
}

onMounted(() => {
  if (previewUrl.value || !rootRef.value) return
  if (typeof IntersectionObserver === 'undefined') {
    void loadPreview()
    return
  }
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
    { rootMargin: '40%' },
  )
  observer.observe(rootRef.value)
})

watch(() => [props.asset, props.cachedUrl], syncResolvedPreview)

onBeforeUnmount(() => {
  observer?.disconnect()
})
</script>

<template>
  <span ref="rootRef" class="aw-drive-thumb aw-drive-thumb--sm">
    <img v-if="previewUrl" :src="previewUrl" :alt="asset.original_filename || asset.file_name || ''" loading="lazy" />
    <span v-else-if="loading" class="aw-drive-thumb__ph" aria-hidden="true" />
    <component :is="iconFor()" v-else-if="!failed" :size="18" aria-hidden="true" />
    <ImageOff v-else :size="18" aria-hidden="true" />
  </span>
</template>
