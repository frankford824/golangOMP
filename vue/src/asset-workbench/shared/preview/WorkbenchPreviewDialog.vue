<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { Download, ExternalLink, X } from 'lucide-vue-next'

import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'
import { isPdfMimeOrFilename } from '@aw/shared/materials/systemAssetPreview'

const props = defineProps<{
  open: boolean
  title: string
  previewUrl?: string
  fallbackSrc?: string
  mimeType?: string
  filename?: string
  alt?: string
  eyebrow?: string
  emptyLabel?: string
  metaRows?: Array<[string, string]>
  downloadLabel?: string
}>()

const emit = defineEmits<{
  close: []
  download: []
}>()

const displayUrl = computed(() => (props.previewUrl || props.fallbackSrc || '').trim())
const isPdf = computed(() => isPdfMimeOrFilename(props.mimeType, props.filename || props.title))
const showPdfFrame = computed(() => isPdf.value && Boolean(displayUrl.value))
const showImagePreview = computed(() => !isPdf.value && Boolean(displayUrl.value))

function handleKeydown(event: KeyboardEvent) {
  if (!props.open || event.key !== 'Escape') return
  event.preventDefault()
  event.stopPropagation()
  event.stopImmediatePropagation()
  emit('close')
}

function openInNewWindow() {
  const url = displayUrl.value
  if (!url) return
  window.open(url, '_blank', 'noopener,noreferrer')
}

let previousBodyOverflow = ''

function syncBodyScrollLock(open: boolean) {
  if (typeof document === 'undefined') return
  if (open) {
    previousBodyOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return
  }
  document.body.style.overflow = previousBodyOverflow
}

watch(
  () => props.open,
  (open) => {
    syncBodyScrollLock(open)
  },
  { immediate: true },
)

onMounted(() => {
  window.addEventListener('keydown', handleKeydown, { capture: true })
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown, { capture: true })
  syncBodyScrollLock(false)
})
</script>

<template>
  <Teleport to="body">
    <section v-if="open" class="aw-token-scope aw-preview-dialog" role="dialog" aria-modal="true" :aria-label="title || '素材预览'">
      <div class="aw-preview-dialog__stage" @click.self="emit('close')">
        <iframe
          v-if="showPdfFrame"
          class="aw-preview-dialog__media aw-preview-dialog__pdf"
          :src="displayUrl"
          :title="title"
        />
        <AssetPreviewMedia
          v-else-if="showImagePreview"
          class="aw-preview-dialog__media"
          :resolved-preview-url="previewUrl || ''"
          :fallback-src="fallbackSrc"
          :alt="alt || title"
        />
        <div v-else class="aw-preview-dialog__empty">
          {{ emptyLabel || '暂无可展示预览' }}
        </div>
      </div>
      <aside class="aw-preview-dialog__side">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">{{ eyebrow || '预览' }}</p>
            <h3>{{ title }}</h3>
          </div>
          <button class="aw-secondary-button" type="button" @click="emit('close')">
            <X :size="16" aria-hidden="true" />
            关闭
          </button>
        </div>
        <dl v-if="metaRows?.length" class="aw-material-detail__list">
          <div v-for="[label, value] in metaRows" :key="label">
            <dt>{{ label }}</dt>
            <dd>{{ value || '—' }}</dd>
          </div>
        </dl>
        <div class="aw-inline-actions">
          <button v-if="showPdfFrame" class="aw-secondary-button" type="button" @click="openInNewWindow">
            <ExternalLink :size="16" aria-hidden="true" />
            新窗口打开
          </button>
          <button v-if="downloadLabel" class="aw-primary-button" type="button" @click="emit('download')">
            <Download :size="16" aria-hidden="true" />
            {{ downloadLabel }}
          </button>
        </div>
      </aside>
    </section>
  </Teleport>
</template>
