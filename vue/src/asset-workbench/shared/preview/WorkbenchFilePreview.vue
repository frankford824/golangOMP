<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'

import { assetWorkbenchApi, type FilePreviewMeta } from '@aw/shared/api/assetWorkbenchApi'

const props = withDefaults(
  defineProps<{
    fileId: number
    alt?: string
    deferUntilVisible?: boolean
  }>(),
  {
    deferUntilVisible: true,
  },
)

const emit = defineEmits<{
  preview: [payload: { fileId: number; title: string; previewUrl: string; meta: FilePreviewMeta | null; statusText: string }]
}>()

const meta = ref<FilePreviewMeta | null>(null)
const loading = ref(false)
const error = ref('')
let controller: AbortController | null = null
let retryTimer: number | null = null
let retryCount = 0
let requestSeq = 0
const MAX_PREVIEW_RETRIES = 6

function clearRetryTimer() {
  if (retryTimer != null) {
    window.clearTimeout(retryTimer)
    retryTimer = null
  }
}

function scheduleRetry() {
  clearRetryTimer()
  if (retryCount >= MAX_PREVIEW_RETRIES) return
  const delay = Math.min(12_000, 1_500 * 2 ** retryCount)
  retryCount += 1
  retryTimer = window.setTimeout(() => {
    retryTimer = null
    void loadPreview()
  }, delay)
}

const previewUrl = computed(() => meta.value?.preview_url ?? '')
const previewTitle = computed(() => `预览 ${props.alt || `文件 ${props.fileId}`}`)
const statusText = computed(() => {
  if (loading.value) return '加载预览'
  if (error.value) return error.value
  if (meta.value?.preparing) return '预览生成中'
  if (meta.value?.status === 'failed') return meta.value.error || '预览失败'
  if (meta.value?.status === 'not_applicable') return '不支持预览'
  return ''
})

async function loadPreview() {
  if (!props.fileId || loading.value) return
  const seq = ++requestSeq
  loading.value = true
  error.value = ''
  controller?.abort()
  controller = new AbortController()
  try {
    const nextMeta = await assetWorkbenchApi.getFilePreview(props.fileId, controller.signal)
    if (seq !== requestSeq) return
    meta.value = nextMeta
    if (meta.value.preparing || meta.value.status === 'pending' || meta.value.status === 'processing') {
      scheduleRetry()
    } else {
      clearRetryTimer()
    }
  } catch (err) {
    if (seq !== requestSeq) return
    if ((err as DOMException)?.name !== 'AbortError') {
      error.value = err instanceof Error ? err.message : '预览加载失败'
    }
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

async function openPreview() {
  if (!meta.value && !loading.value) {
    await loadPreview()
  }
  emit('preview', {
    fileId: props.fileId,
    title: props.alt || `文件 ${props.fileId}`,
    previewUrl: previewUrl.value,
    meta: meta.value,
    statusText: statusText.value || '暂无可展示预览',
  })
}

onMounted(() => {
  void loadPreview()
})

watch(
  () => props.fileId,
  () => {
    requestSeq += 1
    controller?.abort()
    loading.value = false
    clearRetryTimer()
    retryCount = 0
    meta.value = null
    void loadPreview()
  },
)

onBeforeUnmount(() => {
  requestSeq += 1
  clearRetryTimer()
  controller?.abort()
})
</script>

<template>
  <button class="aw-preview-tile aw-preview-tile--button" type="button" :aria-label="previewTitle" :title="previewTitle" @click="openPreview">
    <AssetPreviewMedia
      v-if="previewUrl"
      :resolved-preview-url="previewUrl"
      :alt="alt ?? '资产预览'"
      :defer-until-visible="deferUntilVisible"
      @open-full="openPreview"
    />
    <span v-else>{{ statusText || '等待预览' }}</span>
  </button>
</template>
