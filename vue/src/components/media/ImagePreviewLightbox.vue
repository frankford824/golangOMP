<template>
  <Teleport to="body">
    <div
      v-if="modelValue && activeItem"
      class="image-preview-lightbox"
      role="dialog"
      aria-modal="true"
      :aria-label="ariaLabel"
      @click.self="close"
      @wheel.prevent="handleWheel"
    >
      <div class="image-preview-toolbar" @click.stop>
        <div class="image-preview-title-wrap">
          <div class="image-preview-title" :title="activeTitle">{{ activeTitle }}</div>
          <div v-if="displayItems.length > 1" class="image-preview-count">
            {{ activeIndex + 1 }} / {{ displayItems.length }}
          </div>
        </div>
        <div class="image-preview-actions">
          <button
            type="button"
            class="image-preview-action"
            title="上一张"
            aria-label="上一张"
            :disabled="!canStepPrev"
            @click="step(-1)"
          >
            <ChevronLeft :size="16" />
          </button>
          <button
            type="button"
            class="image-preview-action"
            title="下一张"
            aria-label="下一张"
            :disabled="!canStepNext"
            @click="step(1)"
          >
            <ChevronRight :size="16" />
          </button>
          <span class="image-preview-divider" aria-hidden="true" />
          <button
            type="button"
            class="image-preview-action"
            title="缩小"
            aria-label="缩小预览"
            :disabled="zoom <= ZOOM_MIN"
            @click="zoomBy(-0.2)"
          >
            <Minus :size="16" />
          </button>
          <button
            type="button"
            class="image-preview-action image-preview-action--wide"
            title="重置缩放"
            aria-label="重置缩放"
            @click="resetZoom"
          >
            {{ zoomLabel }}
          </button>
          <button
            type="button"
            class="image-preview-action"
            title="放大"
            aria-label="放大预览"
            :disabled="zoom >= ZOOM_MAX"
            @click="zoomBy(0.2)"
          >
            <Plus :size="16" />
          </button>
          <button
            type="button"
            class="image-preview-action"
            title="适应窗口"
            aria-label="适应窗口"
            @click="resetZoom"
          >
            <RotateCcw :size="16" />
          </button>
          <button
            type="button"
            class="image-preview-action"
            title="下载"
            aria-label="下载当前图片"
            :disabled="downloading || !activeItem"
            @click="downloadActive"
          >
            <Download :size="16" />
          </button>
          <a
            class="image-preview-action"
            :class="{ 'image-preview-action--disabled': !activeOpenHref }"
            title="新窗口打开"
            aria-label="新窗口打开预览图"
            :href="activeOpenHref || '#'"
            target="_blank"
            rel="noopener"
            @click="onOpenLinkClick"
          >
            <ExternalLink :size="16" />
          </a>
          <button
            type="button"
            class="image-preview-close"
            title="关闭"
            aria-label="关闭预览"
            @click="close"
          >
            <X :size="18" />
          </button>
        </div>
      </div>
      <button
        v-if="displayItems.length > 1"
        type="button"
        class="image-preview-nav image-preview-nav--prev"
        title="上一张"
        aria-label="上一张"
        :disabled="!canStepPrev"
        @click.stop="step(-1)"
      >
        <ChevronLeft :size="24" />
      </button>
      <button
        v-if="displayItems.length > 1"
        type="button"
        class="image-preview-nav image-preview-nav--next"
        title="下一张"
        aria-label="下一张"
        :disabled="!canStepNext"
        @click.stop="step(1)"
      >
        <ChevronRight :size="24" />
      </button>
      <div
        class="image-preview-stage"
        @click.self="close"
        @pointerdown="onStagePointerDown"
        @pointerup="onStagePointerUp"
        @touchstart.passive="onStageTouchStart"
        @touchmove.prevent="onStageTouchMove"
        @touchend="onStageTouchEnd"
      >
        <div v-if="activePhase === 'loading'" class="image-preview-state" role="status">
          加载预览...
        </div>
        <div v-else-if="activePhase === 'error'" class="image-preview-state image-preview-state--error" role="alert">
          <span>{{ activeError }}</span>
          <button type="button" class="image-preview-retry" @click.stop="loadActiveItem">重试</button>
        </div>
        <img
          v-else-if="activeDisplaySrc"
          :src="activeDisplaySrc"
          :alt="activeItem.alt || activeTitle"
          class="image-preview-img"
          :style="imageStyle"
          draggable="false"
          @click.stop
          @dblclick.stop="toggleZoom"
        />
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  ChevronLeft,
  ChevronRight,
  Download,
  ExternalLink,
  Minus,
  Plus,
  RotateCcw,
  X,
} from 'lucide-vue-next'
import type { ImagePreviewLightboxItem } from './imagePreviewLightbox'
import { fetchAssetPreviewMeta } from '@/domain/asset-access'
import {
  materializePreviewImageUrl,
  normalizePreviewAssetId,
  revokeMaterializedPreviewImage,
  type MaterializedPreviewImage,
} from '@/domain/asset-preview-image'
import { downloadAssetFileWithOriginalFilename } from '@/utils/assetFileDownload'

const ZOOM_MIN = 0.5
const ZOOM_MAX = 4

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    items?: ImagePreviewLightboxItem[]
    initialIndex?: number
    ariaLabel?: string
    fallbackTitle?: string
  }>(),
  {
    items: () => [],
    initialIndex: 0,
    ariaLabel: '图片预览',
    fallbackTitle: '图片预览',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const activeIndex = ref(0)
const zoom = ref(1)
const activeDisplaySrc = ref('')
const activePhase = ref<'idle' | 'loading' | 'ready' | 'error'>('idle')
const activeError = ref('预览加载失败')
const downloading = ref(false)
const pointerStart = ref<{ id: number; x: number; y: number; t: number } | null>(null)
const pinchStart = ref<{ distance: number; zoom: number } | null>(null)

let activeMaterializedImage: MaterializedPreviewImage | null = null
let loadSeq = 0

const displayItems = computed(() =>
  (props.items ?? [])
    .map((item) => ({
      ...item,
      src: String(item.src ?? '').trim(),
      previewAssetId: String(item.previewAssetId ?? '').trim(),
      fallbackAssetId: String(item.fallbackAssetId ?? '').trim(),
      fallbackSrc: String(item.fallbackSrc ?? '').trim(),
      resolvedPreviewUrl: String(item.resolvedPreviewUrl ?? '').trim(),
      title: String(item.title ?? '').trim(),
      alt: String(item.alt ?? '').trim(),
      downloadUrl: String(item.downloadUrl ?? '').trim(),
      preferredFilename: String(item.preferredFilename ?? '').trim(),
    }))
    .filter(itemHasLoadableSource),
)

const activeItem = computed(() => displayItems.value[activeIndex.value] ?? null)
const activeTitle = computed(() => activeItem.value?.title || props.fallbackTitle)
const activeOpenHref = computed(() => {
  const item = activeItem.value
  if (!item) return ''
  return item.downloadUrl || item.fallbackSrc || item.resolvedPreviewUrl || item.src
})
const zoomLabel = computed(() => `${Math.round(zoom.value * 100)}%`)
const imageStyle = computed(() => ({
  transform: `scale(${zoom.value})`,
}))
const canStepPrev = computed(() => activeIndex.value > 0)
const canStepNext = computed(() => activeIndex.value < displayItems.value.length - 1)

function clampIndex(value: number): number {
  const max = Math.max(0, displayItems.value.length - 1)
  if (!Number.isFinite(value)) return 0
  return Math.min(max, Math.max(0, Math.trunc(value)))
}

function clampZoom(value: number): number {
  return Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, Number(value.toFixed(2))))
}

function itemHasLoadableSource(item: ImagePreviewLightboxItem): boolean {
  return Boolean(
    item.src ||
      item.previewAssetId ||
      item.fallbackAssetId ||
      item.fallbackSrc ||
      item.resolvedPreviewUrl ||
      item.downloadUrl,
  )
}

function itemSignature(item: ImagePreviewLightboxItem): string {
  return [
    item.src,
    item.previewAssetId,
    item.fallbackAssetId,
    item.fallbackSrc,
    item.resolvedPreviewUrl,
    item.downloadUrl,
  ].join('\x1f')
}

function clearActiveMaterializedImage() {
  revokeMaterializedPreviewImage(activeMaterializedImage)
  activeMaterializedImage = null
}

function setActiveImage(image: MaterializedPreviewImage) {
  clearActiveMaterializedImage()
  activeMaterializedImage = image
  activeDisplaySrc.value = image.displaySrc
  activePhase.value = 'ready'
}

function uniqueNonEmpty(values: Array<string | undefined>): string[] {
  const out: string[] = []
  const seen = new Set<string>()
  for (const value of values) {
    const text = String(value ?? '').trim()
    if (!text || seen.has(text)) continue
    seen.add(text)
    out.push(text)
  }
  return out
}

async function resolveActiveImage(item: ImagePreviewLightboxItem): Promise<MaterializedPreviewImage | undefined> {
  if (item.src.startsWith('blob:') || item.src.startsWith('data:')) {
    return materializePreviewImageUrl(item.src)
  }
  for (const assetId of uniqueNonEmpty([normalizePreviewAssetId(item.previewAssetId), normalizePreviewAssetId(item.fallbackAssetId)])) {
    const meta = await fetchAssetPreviewMeta(assetId).catch(() => null)
    if (meta?.status === 'ok' && meta.displayUrl) {
      const image = await materializePreviewImageUrl(meta.displayUrl)
      if (image) return image
    }
  }
  for (const url of uniqueNonEmpty([item.resolvedPreviewUrl, item.src, item.fallbackSrc, item.downloadUrl])) {
    const image = await materializePreviewImageUrl(url)
    if (image) return image
  }
  return undefined
}

async function loadActiveItem() {
  const item = activeItem.value
  const my = ++loadSeq
  clearActiveMaterializedImage()
  activeDisplaySrc.value = ''
  if (!props.modelValue || !item) {
    activePhase.value = 'idle'
    return
  }
  activePhase.value = 'loading'
  activeError.value = '预览加载中'
  const image = await resolveActiveImage(item)
  if (my !== loadSeq) {
    revokeMaterializedPreviewImage(image)
    return
  }
  if (image) {
    setActiveImage(image)
    return
  }
  activeError.value = '当前图片暂时无法预览'
  activePhase.value = 'error'
}

function close() {
  emit('update:modelValue', false)
}

function step(delta: -1 | 1) {
  const next = activeIndex.value + delta
  if (next < 0 || next >= displayItems.value.length) return
  activeIndex.value = next
}

function zoomBy(delta: number) {
  zoom.value = clampZoom(zoom.value + delta)
}

function resetZoom() {
  zoom.value = 1
}

function toggleZoom() {
  zoom.value = zoom.value > 1 ? 1 : 2
}

function handleWheel(event: WheelEvent) {
  zoomBy(event.deltaY > 0 ? -0.15 : 0.15)
}

function handleKeydown(event: KeyboardEvent) {
  if (!props.modelValue) return
  if (event.key === 'Escape') {
    event.preventDefault()
    close()
    return
  }
  if (event.key === 'ArrowLeft') {
    event.preventDefault()
    step(-1)
    return
  }
  if (event.key === 'ArrowRight') {
    event.preventDefault()
    step(1)
    return
  }
  if (event.key === '+' || event.key === '=') {
    event.preventDefault()
    zoomBy(0.2)
    return
  }
  if (event.key === '-') {
    event.preventDefault()
    zoomBy(-0.2)
    return
  }
  if (event.key === '0') {
    event.preventDefault()
    resetZoom()
  }
}

function onOpenLinkClick(event: MouseEvent) {
  if (activeOpenHref.value) return
  event.preventDefault()
}

async function downloadActive() {
  const item = activeItem.value
  if (!item || downloading.value) return
  downloading.value = true
  const result = await downloadAssetFileWithOriginalFilename({
    assetId: item.previewAssetId || item.fallbackAssetId || undefined,
    downloadUrl: item.downloadUrl || item.fallbackSrc || item.resolvedPreviewUrl || item.src || undefined,
    preferredFilename: item.preferredFilename || item.title || item.alt || 'image',
  })
  downloading.value = false
  if (!result.ok) {
    window.alert(result.message ?? '下载失败，请稍后重试')
  }
}

function onStagePointerDown(event: PointerEvent) {
  if (event.pointerType === 'mouse' && event.button !== 0) return
  pointerStart.value = {
    id: event.pointerId,
    x: event.clientX,
    y: event.clientY,
    t: Date.now(),
  }
}

function onStagePointerUp(event: PointerEvent) {
  const start = pointerStart.value
  pointerStart.value = null
  if (!start || start.id !== event.pointerId) return
  const dx = event.clientX - start.x
  const dy = event.clientY - start.y
  const dt = Date.now() - start.t
  if (dt > 900 || Math.abs(dx) < 52 || Math.abs(dy) > 90) return
  if (dx < 0) step(1)
  else step(-1)
}

function touchDistance(touches: TouchList): number {
  if (touches.length < 2) return 0
  const a = touches[0]
  const b = touches[1]
  return Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY)
}

function onStageTouchStart(event: TouchEvent) {
  if (event.touches.length !== 2) return
  pinchStart.value = {
    distance: touchDistance(event.touches),
    zoom: zoom.value,
  }
}

function onStageTouchMove(event: TouchEvent) {
  const start = pinchStart.value
  if (!start || event.touches.length !== 2 || start.distance <= 0) return
  const next = start.zoom * (touchDistance(event.touches) / start.distance)
  zoom.value = clampZoom(next)
}

function onStageTouchEnd(event: TouchEvent) {
  if (event.touches.length < 2) pinchStart.value = null
}

function setBodyLock(locked: boolean) {
  if (typeof document === 'undefined') return
  document.body.classList.toggle('asset-preview-open', locked)
}

watch(
  () => [props.modelValue, props.initialIndex, displayItems.value.map(itemSignature).join('|')] as const,
  ([open, index]) => {
    if (!open) return
    activeIndex.value = clampIndex(index)
    resetZoom()
    void loadActiveItem()
  },
  { immediate: true },
)

watch(activeIndex, () => {
  resetZoom()
  void loadActiveItem()
})
watch(
  () => props.modelValue,
  (open) => {
    setBodyLock(open)
    if (open) void loadActiveItem()
    else {
      ++loadSeq
      clearActiveMaterializedImage()
      activeDisplaySrc.value = ''
      activePhase.value = 'idle'
    }
  },
  { immediate: true },
)

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown)
  clearActiveMaterializedImage()
  setBodyLock(false)
})
</script>

<style scoped>
.image-preview-lightbox {
  position: fixed;
  inset: 0;
  z-index: 2147483000;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: 0.75rem;
  padding: 1rem;
  background: rgb(var(--yb-overlay-night) / 0.88);
  color: rgb(var(--yb-surface-subtle));
  cursor: default;
  isolation: isolate;
}

.image-preview-toolbar {
  width: min(62rem, calc(100vw - 2rem));
  min-height: 2.75rem;
  justify-self: center;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.4rem 0.5rem 0.4rem 0.85rem;
  border: 1px solid rgb(var(--yb-border-slate) / 0.22);
  border-radius: 8px;
  background: rgb(var(--yb-shadow) / 0.94);
  box-shadow: 0 1.25rem 3rem rgb(var(--yb-black) / 0.28);
}

.image-preview-title-wrap {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.image-preview-title {
  min-width: 0;
  overflow: hidden;
  color: rgb(var(--yb-surface-subtle));
  font-size: 0.875rem;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.image-preview-count {
  flex: 0 0 auto;
  color: rgb(var(--yb-text-disabled));
  font-size: 0.75rem;
}

.image-preview-actions {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  gap: 0.35rem;
}

.image-preview-action,
.image-preview-close,
.image-preview-nav {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid rgb(var(--yb-border-slate) / 0.2);
  border-radius: 8px;
  background: rgb(var(--yb-text-deep) / 0.86);
  color: rgb(var(--yb-surface-subtle));
  line-height: 1;
  transition: background-color 0.15s ease, border-color 0.15s ease, transform 0.15s ease;
}

.image-preview-action,
.image-preview-close {
  width: 2.25rem;
  height: 2rem;
}

.image-preview-action:hover,
.image-preview-close:hover,
.image-preview-nav:hover {
  border-color: rgb(var(--yb-brand-border-strong) / 0.8);
  background: rgb(var(--yb-brand) / 0.86);
  transform: translateY(-1px);
}

.image-preview-action:disabled,
.image-preview-action--disabled,
.image-preview-nav:disabled {
  cursor: not-allowed;
  opacity: 0.45;
  transform: none;
}

.image-preview-action--wide {
  width: 4.25rem;
  font-size: 0.78rem;
  font-weight: 700;
}

.image-preview-close {
  background: rgb(var(--yb-danger-overlay) / 0.9);
}

.image-preview-divider {
  width: 1px;
  height: 1.5rem;
  background: rgb(var(--yb-border-slate) / 0.2);
}

.image-preview-stage {
  min-height: 0;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: auto;
  overscroll-behavior: contain;
  padding: 0.25rem 3.5rem;
  touch-action: pan-x pan-y pinch-zoom;
}

.image-preview-img {
  display: block;
  max-width: min(96vw, 87.5rem);
  max-height: calc(100vh - 6.5rem);
  object-fit: contain;
  border-radius: 6px;
  background: rgb(var(--yb-surface));
  box-shadow: 0 1.5rem 4rem rgb(var(--yb-black) / 0.42);
  transform-origin: center center;
  transition: transform 0.15s ease;
  user-select: none;
}

.image-preview-state {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  min-width: min(24rem, calc(100vw - 3rem));
  min-height: 8rem;
  border: 1px solid rgb(var(--yb-border-slate) / 0.2);
  border-radius: 8px;
  padding: 1rem;
  background: rgb(var(--yb-shadow) / 0.84);
  color: rgb(var(--yb-border-slate));
  font-size: 0.875rem;
}

.image-preview-state--error {
  color: rgb(var(--yb-danger-border));
}

.image-preview-retry {
  border: 1px solid rgb(var(--yb-danger-border) / 0.42);
  border-radius: 8px;
  padding: 0.35rem 0.75rem;
  background: rgb(var(--yb-danger-overlay) / 0.72);
  color: rgb(var(--yb-surface));
  font-size: 0.8125rem;
  font-weight: 700;
}

.image-preview-nav {
  position: fixed;
  top: 50%;
  z-index: 1;
  width: 3rem;
  height: 4.5rem;
  transform: translateY(-50%);
}

.image-preview-nav:hover {
  transform: translateY(calc(-50% - 1px));
}

.image-preview-nav--prev {
  left: 1rem;
}

.image-preview-nav--next {
  right: 1rem;
}

:global(body.asset-preview-open) {
  overflow: hidden;
}

@media (max-width: 720px) {
  .image-preview-lightbox {
    gap: 0.5rem;
    padding: max(0.5rem, env(safe-area-inset-top)) max(0.5rem, env(safe-area-inset-right))
      max(0.75rem, env(safe-area-inset-bottom)) max(0.5rem, env(safe-area-inset-left));
  }

  .image-preview-toolbar {
    width: 100%;
    align-items: center;
    gap: 0.5rem;
    padding: 0.45rem;
  }

  .image-preview-actions {
    width: auto;
    max-width: 58vw;
    overflow-x: auto;
    padding-bottom: 0.05rem;
  }

  .image-preview-stage {
    padding: 0;
  }

  .image-preview-nav {
    width: 2.5rem;
    height: 3.5rem;
  }

  .image-preview-nav--prev {
    left: 0.5rem;
  }

  .image-preview-nav--next {
    right: 0.5rem;
  }

  .image-preview-title-wrap {
    flex: 1 1 auto;
  }

  .image-preview-action,
  .image-preview-close {
    width: 2.35rem;
    height: 2.35rem;
  }

  .image-preview-action--wide {
    width: 3.4rem;
    font-size: 0.72rem;
  }

  .image-preview-img {
    max-width: 100vw;
    max-height: calc(100dvh - 5.5rem);
    border-radius: 4px;
  }
}
</style>
