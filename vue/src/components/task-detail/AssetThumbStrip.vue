<template>
  <div class="thumb-strip" data-test="asset-thumb-strip">
    <div v-if="!normalizedItems.length" class="thumb-empty">{{ emptyText }}</div>
    <div v-else class="thumb-row" role="list">
      <button
        v-for="item in normalizedItems"
        :key="item.key"
        type="button"
        class="thumb-btn"
        :class="[{ 'thumb-btn--sm': size === 'sm', 'thumb-btn--md': size === 'md' }, item.unavailable ? 'thumb-btn--placeholder' : '']"
        :title="item.label || item.alt"
        role="listitem"
        @click="onThumbClick(item)"
      >
        <img
          v-if="item.src && !item.previewAssetId && !item.unavailable"
          :src="item.src"
          :alt="item.alt"
          class="thumb-img"
          loading="lazy"
        />
        <AssetPreviewMedia
          v-else-if="item.previewAssetId && !item.unavailable"
          class="thumb-media"
          :asset-id="item.previewAssetId"
          :fallback-asset-id="item.fallbackAssetId || null"
          :fallback-src="item.src || null"
          :alt="item.alt"
          img-class="thumb-media-shell"
          inner-img-class="thumb-img"
          :defer-until-visible="true"
          @open-full="openLightbox"
        />
        <span v-else class="thumb-placeholder">{{ item.label || '文件' }}</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject } from 'vue'
import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'

export type AssetThumbItem = {
  key: string
  src?: string
  previewAssetId?: string
  fallbackAssetId?: string
  alt: string
  downloadUrl?: string
  label?: string
  unavailable?: boolean
}

const OPEN_LIGHTBOX_KEY = 'task-detail-open-lightbox'

const props = withDefaults(
  defineProps<{
    items: AssetThumbItem[]
    emptyText?: string
    size?: 'sm' | 'md'
  }>(),
  {
    emptyText: '暂无图片',
    size: 'sm',
  },
)
const emit = defineEmits<{
  select: [key: string]
}>()

const normalizedItems = computed(() =>
  props.items
    .map((item) => ({
      ...item,
      src: (item.src ?? '').trim(),
      previewAssetId: (item.previewAssetId ?? '').trim(),
      fallbackAssetId: (item.fallbackAssetId ?? '').trim(),
      downloadUrl: (item.downloadUrl ?? '').trim(),
      label: (item.label ?? '').trim(),
    }))
    .filter((item) => item.key.trim().length > 0),
)

const openLightbox = inject<(src: string) => void>(OPEN_LIGHTBOX_KEY, () => {})

function onThumbClick(item: AssetThumbItem) {
  emit('select', item.key)
  if (item.unavailable && item.downloadUrl) {
    window.open(item.downloadUrl, '_blank', 'noopener')
    return
  }
  if (item.src) openLightbox(item.src)
}
</script>

<style scoped>
.thumb-strip {
  width: 100%;
}
.thumb-row {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  overflow-x: auto;
  padding-bottom: 0.25rem;
  -webkit-overflow-scrolling: touch;
}
.thumb-btn {
  padding: 0;
  flex: 0 0 auto;
  border: 1px solid #dbe3ee;
  border-radius: 0.375rem;
  background: #ffffff;
  overflow: hidden;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}
.thumb-btn--sm {
  width: 3rem;
  height: 3rem;
}
.thumb-btn--md {
  width: 4rem;
  height: 4rem;
}
.thumb-btn:hover {
  border-color: #98a2b3;
}
.thumb-btn--placeholder {
  background: #f8fafc;
}
.thumb-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.thumb-media {
  width: 100%;
  height: 100%;
}
.thumb-media :deep(.apm) {
  width: 100%;
  height: 100%;
  min-height: 0;
}
.thumb-media :deep(.apm-img) {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 0;
}
.thumb-media :deep(.apm-placeholder),
.thumb-media :deep(.apm-empty) {
  min-height: 0;
  height: 100%;
  border: 0;
  border-radius: 0;
  padding: 0.125rem;
}
.thumb-placeholder {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  padding: 0.25rem;
  text-align: center;
  font-size: 0.625rem;
  line-height: 1.2;
  color: #667085;
  background: #f8fafc;
}
.thumb-empty {
  font-size: 0.75rem;
  color: #98a2b3;
}
</style>
