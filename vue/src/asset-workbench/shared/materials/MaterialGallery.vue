<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Download, Eye, FileArchive, FileImage, FileText } from 'lucide-vue-next'

import type { SystemAssetRow } from '@aw/shared/api/assetWorkbenchApi'
import { chipClass, systemPreviewMeta } from '@aw/shared/format/status'
import {
  canAttemptSystemAssetPreview,
  resolvedSystemAssetThumbnailUrl,
} from '@aw/shared/materials/systemAssetPreview'
import { computeGalleryVirtualScroll } from '@aw/shared/materials/materialGalleryVirtualScroll'

const props = withDefaults(
  defineProps<{
    items: SystemAssetRow[]
    selectedIds: Set<number>
    previewUrls?: Record<number, string>
    previewLoadingIds?: Set<number>
    activeId?: number | null
    loading?: boolean
    error?: string
  }>(),
  {
    activeId: null,
    loading: false,
    error: '',
  },
)

const emit = defineEmits<{
  preview: [asset: SystemAssetRow]
  download: [asset: SystemAssetRow]
  select: [asset: SystemAssetRow]
  toggle: [asset: SystemAssetRow, checked: boolean, index: number, range: boolean]
  visible: [assets: SystemAssetRow[]]
  retry: []
}>()

const galleryRef = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const viewportHeight = ref(720)
const containerWidth = ref(960)
const cardMinWidth = 224
const rowHeight = 306
const gridPadding = 12
const gridGap = 12
let resizeObserver: ResizeObserver | null = null

const virtualScroll = computed(() =>
  computeGalleryVirtualScroll({
    itemCount: props.items.length,
    scrollTop: scrollTop.value,
    viewportHeight: viewportHeight.value,
    containerWidth: containerWidth.value,
    cardMinWidth,
    rowHeight,
    gridPadding,
    gridGap,
  }),
)
const visibleItems = computed(() =>
  props.items.slice(virtualScroll.value.startIndex, virtualScroll.value.endIndex).map((asset, offset) => ({
    asset,
    index: virtualScroll.value.startIndex + offset,
  })),
)
const gridStyle = computed(() => ({
  '--aw-material-gallery-columns': `repeat(${virtualScroll.value.columnCount}, minmax(0, 1fr))`,
  '--aw-material-gallery-offset': `${virtualScroll.value.top}px`,
  '--aw-material-gallery-height': `${virtualScroll.value.totalHeight}px`,
}))

function titleOf(asset: SystemAssetRow) {
  return asset.product_name || asset.original_filename || asset.file_name || asset.task_no || `素材 ${asset.id}`
}

function codeOf(asset: SystemAssetRow) {
  const sku = asset.scope_sku_code || asset.sku_code || asset.primary_sku_code
  return sku ? `SKU ${sku}` : `素材 ${asset.id}`
}

function typeLabel(asset: SystemAssetRow) {
  const mime = (asset.mime_type || '').toLowerCase()
  if (mime.includes('photoshop')) return 'PSD'
  if (mime.includes('pdf')) return 'PDF'
  if (mime.startsWith('image/')) return mime.replace('image/', '').toUpperCase()
  if (mime.includes('zip') || mime.includes('rar') || mime.includes('7z')) return '压缩包'
  return asset.mime_type || '文件'
}

function isImageLike(asset: SystemAssetRow) {
  const mime = (asset.mime_type || '').toLowerCase()
  return mime.startsWith('image/') && !mime.includes('photoshop') && !mime.includes('vnd.adobe')
}

function iconFor(asset: SystemAssetRow) {
  const label = typeLabel(asset)
  if (isImageLike(asset)) return FileImage
  if (label === 'PDF') return FileText
  if (label === '压缩包') return FileArchive
  return FileImage
}

function previewUrlFor(asset: SystemAssetRow) {
  return resolvedSystemAssetThumbnailUrl(asset, props.previewUrls?.[asset.id])
}

function isPreviewLoading(asset: SystemAssetRow) {
  return canPreview(asset) && !previewUrlFor(asset) && Boolean(props.previewLoadingIds?.has(asset.id))
}

function canPreview(asset: SystemAssetRow) {
  return canAttemptSystemAssetPreview(asset)
}

function updateSize() {
  const el = galleryRef.value
  if (el) containerWidth.value = Math.max(cardMinWidth, el.clientWidth)
}

function updateViewportWindow() {
  const el = galleryRef.value
  viewportHeight.value = window.innerHeight
  if (!el) {
    scrollTop.value = 0
    return
  }
  const rect = el.getBoundingClientRect()
  const galleryTop = window.scrollY + rect.top
  scrollTop.value = Math.max(0, window.scrollY - galleryTop)
  updateSize()
}

function bindResizeObserver() {
  resizeObserver?.disconnect()
  if (typeof ResizeObserver === 'undefined' || !galleryRef.value) {
    updateSize()
    return
  }
  resizeObserver = new ResizeObserver(updateSize)
  resizeObserver.observe(galleryRef.value)
}

function toggleAsset(asset: SystemAssetRow, checked: boolean, index: number, event: Event) {
  const range = 'shiftKey' in event && Boolean((event as MouseEvent).shiftKey)
  emit('toggle', asset, checked, index, range)
}

onMounted(() => {
  updateViewportWindow()
  bindResizeObserver()
  window.addEventListener('scroll', updateViewportWindow, { passive: true })
  window.addEventListener('resize', updateViewportWindow)
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  window.removeEventListener('scroll', updateViewportWindow)
  window.removeEventListener('resize', updateViewportWindow)
})

watch(
  () => props.items,
  () => {
    scrollTop.value = 0
    updateViewportWindow()
  },
)

watch(
  visibleItems,
  (items) => {
    emit('visible', items.map((item) => item.asset))
  },
  { immediate: true },
)
</script>

<template>
  <section ref="galleryRef" class="aw-material-gallery" aria-label="素材浏览器">
    <div v-if="loading" class="aw-material-gallery__loading">正在检索素材</div>
    <div v-else-if="error" class="aw-empty-state">
      <h3>素材加载失败</h3>
      <p>{{ error }}</p>
      <button class="aw-secondary-button" type="button" @click="emit('retry')">重试</button>
    </div>
    <div v-else-if="items.length === 0" class="aw-empty-state">
      <h3>没有可见素材</h3>
      <p>调整关键词或筛选条件后再试。素材库只展示你已开通查看范围内的素材。</p>
    </div>
    <div v-else class="aw-material-gallery__viewport" :style="{ height: gridStyle['--aw-material-gallery-height'] }">
      <div class="aw-material-gallery__grid" :style="gridStyle">
        <article
          v-for="{ asset, index } in visibleItems"
          :key="asset.id"
          class="aw-material-card"
          :class="{
            'aw-material-card--selected': selectedIds.has(asset.id),
            'aw-material-card--active': activeId === asset.id,
            'aw-material-card--download-only': !canPreview(asset),
          }"
          tabindex="0"
          @click="emit('select', asset)"
          @keydown.enter.prevent="emit('select', asset)"
          @keydown.space.prevent="emit('toggle', asset, !selectedIds.has(asset.id), index, false)"
        >
          <button
            class="aw-material-card__media"
            :class="{ 'aw-material-card__media--loading': isPreviewLoading(asset) }"
            type="button"
            :disabled="!canPreview(asset)"
            :title="canPreview(asset) ? '预览素材' : '当前素材只能下载'"
            @click.stop="emit('preview', asset)"
          >
            <img
              v-if="previewUrlFor(asset)"
              :src="previewUrlFor(asset)"
              :alt="titleOf(asset)"
              loading="lazy"
              decoding="async"
            />
            <span v-else-if="isPreviewLoading(asset)" class="aw-material-card__file">
              <span>加载中</span>
            </span>
            <span v-else class="aw-material-card__file">
              <component :is="iconFor(asset)" class="aw-material-card__icon" :size="30" aria-hidden="true" />
              <span>{{ typeLabel(asset) }}</span>
            </span>
          </button>
          <div class="aw-material-card__body">
            <strong>{{ titleOf(asset) }}</strong>
            <span class="aw-material-card__code">{{ codeOf(asset) }}</span>
            <div class="aw-material-card__meta">
              <span>{{ typeLabel(asset) }}</span>
              <span :class="chipClass(systemPreviewMeta(canPreview(asset)).tone)">
                {{ systemPreviewMeta(canPreview(asset)).label }}
              </span>
            </div>
          </div>
          <div class="aw-material-card__actions" @click.stop>
            <label class="aw-material-card__check" :title="selectedIds.has(asset.id) ? '取消选择' : '选择'">
              <input
                type="checkbox"
                :checked="selectedIds.has(asset.id)"
                :aria-label="`${selectedIds.has(asset.id) ? '取消选择' : '选择'}素材 ${titleOf(asset)}`"
                @change="toggleAsset(asset, ($event.target as HTMLInputElement).checked, index, $event)"
              />
            </label>
            <button type="button" :disabled="!canPreview(asset)" title="预览" @click="emit('preview', asset)">
              <Eye :size="15" aria-hidden="true" />
            </button>
            <button type="button" title="下载" @click="emit('download', asset)">
              <Download :size="15" aria-hidden="true" />
            </button>
          </div>
        </article>
      </div>
    </div>
  </section>
</template>
