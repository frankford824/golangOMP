<template>
  <span v-if="oaEnabled && downloadUrl && supported" class="original-dimensions">
    <button v-if="!result || failed" type="button" :disabled="loading" :aria-label="`读取 ${name || '文件'} 的原图分辨率`" @click.stop="load">
      {{ loading ? '读取中…' : failed ? '重试' : '查看原图分辨率' }}
    </button>
    <span role="status">{{ result }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { readOriginalImageDimensions } from '@/domain/original-image-dimensions'
import { oaEnabled } from '@/oa/access'
import { resourceGroupsApi } from '@/services/api/resourceGroupsApi'

const props = defineProps<{ downloadUrl?: string | null; name?: string | null; mimeType?: string | null; taskAssetId?: number | string | null; thumbnailUrl?: string | null }>()
const supported = computed(() => /\.(?:jpe?g|png|gif|webp|psd|psb)$/i.test(props.name || '') ||
  /^(?:image\/(?:jpe?g|png|gif|webp|vnd\.adobe\.photoshop)|application\/(?:x-)?photoshop)$/i.test(props.mimeType || ''))
const loading = ref(false)
const result = ref('')
const failed = ref(false)
let pending: AbortController | undefined

function reset() {
  pending?.abort()
  pending = undefined
  loading.value = false
  result.value = ''
  failed.value = false
}
watch(() => [props.downloadUrl, props.name, props.taskAssetId, props.thumbnailUrl], reset)
onBeforeUnmount(reset)

async function load() {
  if (!props.downloadUrl || loading.value) return
  const request = new AbortController()
  const timeout = window.setTimeout(() => request.abort(), 8000)
  pending = request
  loading.value = true
  failed.value = false
  result.value = ''
  try {
    const url = props.taskAssetId
      ? (await resourceGroupsApi.downloadTaskAsset(Number(props.taskAssetId), request.signal)).download_url
      : props.downloadUrl
    const dimensions = url && url !== props.thumbnailUrl ? await readOriginalImageDimensions(url, request.signal) : undefined
    if (!dimensions) throw new Error('Original file dimensions unavailable')
    if (pending === request) result.value = `原图 ${dimensions.width} × ${dimensions.height} px`
  } catch {
    if (pending === request) {
      failed.value = true
      result.value = '原图分辨率暂不可读取'
    }
  } finally {
    window.clearTimeout(timeout)
    if (pending === request) {
      pending = undefined
      loading.value = false
    }
  }
}
</script>

<style scoped>
.original-dimensions { display: block; color: rgb(var(--yb-text-muted-strong)); font-size: 12px; }
.original-dimensions button { min-height: 24px; padding: 0; border: 0; background: transparent; color: rgb(var(--yb-brand)); text-align: left; cursor: pointer; margin-inline-end: 6px; }
.original-dimensions button:disabled { opacity: .65; cursor: wait; }
</style>
