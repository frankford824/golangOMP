<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import { Download, X } from 'lucide-vue-next'

import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'

const props = defineProps<{
  open: boolean
  title: string
  previewUrl?: string
  fallbackSrc?: string
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

function handleKeydown(event: KeyboardEvent) {
  if (!props.open || event.key !== 'Escape') return
  event.preventDefault()
  event.stopPropagation()
  event.stopImmediatePropagation()
  emit('close')
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown, { capture: true })
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown, { capture: true })
})
</script>

<template>
  <section v-if="open" class="aw-preview-dialog" role="dialog" aria-modal="true" :aria-label="title || '素材预览'">
    <div class="aw-preview-dialog__stage" @click.self="emit('close')">
      <AssetPreviewMedia
        v-if="previewUrl || fallbackSrc"
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
      <button v-if="downloadLabel" class="aw-primary-button" type="button" @click="emit('download')">
        <Download :size="16" aria-hidden="true" />
        {{ downloadLabel }}
      </button>
    </aside>
  </section>
</template>
