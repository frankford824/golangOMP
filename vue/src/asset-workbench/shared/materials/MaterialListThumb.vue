<script setup lang="ts">
import { shallowRef, watch } from 'vue'
import { FileArchive, FileImage, FileText } from 'lucide-vue-next'

import type { SystemAssetRow } from '@aw/shared/api/assetWorkbenchApi'
import {
  resolvedSystemAssetThumbnailUrl,
} from '@aw/shared/materials/systemAssetPreview'

const props = defineProps<{
  asset: SystemAssetRow
  cachedUrl?: string
}>()

const previewUrl = shallowRef(resolvedSystemAssetThumbnailUrl(props.asset, props.cachedUrl))

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

watch(() => [props.asset, props.cachedUrl], syncResolvedPreview)
</script>

<template>
  <span class="aw-drive-thumb aw-drive-thumb--sm">
    <img v-if="previewUrl" :src="previewUrl" :alt="asset.original_filename || asset.file_name || ''" loading="lazy" />
    <component :is="iconFor()" v-else :size="18" aria-hidden="true" />
  </span>
</template>
