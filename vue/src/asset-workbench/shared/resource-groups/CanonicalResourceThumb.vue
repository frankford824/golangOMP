<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'

import { resourceGroupsApi, type FlatResourceItem } from '@/services/api/resourceGroupsApi'
import type { CanonicalResourceFile } from './canonicalResource'
import { resolveCanonicalPreview } from './canonicalResource'

const props = defineProps<{
  item?: FlatResourceItem
  file?: CanonicalResourceFile | null
  alt: string
}>()

const url = ref('')
const failed = ref(false)
let requestID = 0
let abortController: AbortController | null = null
const identity = computed(() => props.item
  ? `${props.item.group_id}:${props.item.revision_id}:${props.item.resource_role}:${props.item.resource_item_id}`
  : `${props.file?.taskAssetId || 0}:${props.file?.previewUrl || ''}`)

async function load() {
  const currentRequest = ++requestID
  abortController?.abort()
  abortController = new AbortController()
  url.value = ''
  failed.value = false
  try {
    if (props.item) {
      const meta = await resolveCanonicalPreview(props.item, resourceGroupsApi.get, abortController.signal)
      if (currentRequest === requestID) url.value = String(meta.download_url || '')
      return
    }
    const file = props.file
    if (!file) return
    if (file.taskAssetId) {
      const meta = await resourceGroupsApi.previewTaskAsset(file.taskAssetId, abortController.signal)
      if (currentRequest === requestID) url.value = String(meta.download_url || '')
      return
    }
    if (currentRequest === requestID) url.value = file.previewUrl || file.downloadUrl || ''
  } catch {
    if (currentRequest === requestID) failed.value = true
  }
}

watch(identity, load, { immediate: true })
onBeforeUnmount(() => abortController?.abort())
</script>

<template>
  <img v-if="url && !failed" :src="url" :alt="alt" loading="lazy" @error="failed = true" />
  <span v-else class="aw-canonical-thumb__fallback" aria-hidden="true">{{ failed ? 'NO PREVIEW' : 'LOADING' }}</span>
</template>
