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
          <a
            class="image-preview-action"
            title="新窗口打开"
            aria-label="新窗口打开预览图"
            :href="activeItem.downloadUrl || activeItem.src"
            target="_blank"
            rel="noopener"
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
      <div class="image-preview-stage" @click.stop>
        <img
          :src="activeItem.src"
          :alt="activeItem.alt || activeTitle"
          class="image-preview-img"
          :style="imageStyle"
          draggable="false"
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
  ExternalLink,
  Minus,
  Plus,
  RotateCcw,
  X,
} from 'lucide-vue-next'
import type { ImagePreviewLightboxItem } from './imagePreviewLightbox'

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

const displayItems = computed(() =>
  (props.items ?? [])
    .map((item) => ({
      ...item,
      src: String(item.src ?? '').trim(),
      title: String(item.title ?? '').trim(),
      alt: String(item.alt ?? '').trim(),
      downloadUrl: String(item.downloadUrl ?? '').trim(),
    }))
    .filter((item) => item.src.length > 0),
)

const activeItem = computed(() => displayItems.value[activeIndex.value] ?? null)
const activeTitle = computed(() => activeItem.value?.title || props.fallbackTitle)
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

function setBodyLock(locked: boolean) {
  if (typeof document === 'undefined') return
  document.body.classList.toggle('asset-preview-open', locked)
}

watch(
  () => [props.modelValue, props.initialIndex, displayItems.value.map((item) => item.src).join('|')] as const,
  ([open, index]) => {
    if (!open) return
    activeIndex.value = clampIndex(index)
    resetZoom()
  },
  { immediate: true },
)

watch(activeIndex, () => resetZoom())
watch(
  () => props.modelValue,
  (open) => setBodyLock(open),
  { immediate: true },
)

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown)
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
  background: rgba(2, 6, 23, 0.88);
  color: #f8fafc;
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
  border: 1px solid rgba(226, 232, 240, 0.22);
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.94);
  box-shadow: 0 1.25rem 3rem rgba(0, 0, 0, 0.28);
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
  color: #f8fafc;
  font-size: 0.875rem;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.image-preview-count {
  flex: 0 0 auto;
  color: #cbd5e1;
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
  border: 1px solid rgba(226, 232, 240, 0.2);
  border-radius: 8px;
  background: rgba(30, 41, 59, 0.86);
  color: #f8fafc;
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
  border-color: rgba(147, 197, 253, 0.8);
  background: rgba(37, 99, 235, 0.86);
  transform: translateY(-1px);
}

.image-preview-action:disabled,
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
  background: rgba(127, 29, 29, 0.9);
}

.image-preview-divider {
  width: 1px;
  height: 1.5rem;
  background: rgba(226, 232, 240, 0.2);
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
}

.image-preview-img {
  display: block;
  max-width: min(96vw, 87.5rem);
  max-height: calc(100vh - 6.5rem);
  object-fit: contain;
  border-radius: 6px;
  background: #fff;
  box-shadow: 0 1.5rem 4rem rgba(0, 0, 0, 0.42);
  transform-origin: center center;
  transition: transform 0.15s ease;
  user-select: none;
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
    padding: 0.75rem;
  }

  .image-preview-toolbar {
    width: 100%;
    align-items: flex-start;
    flex-direction: column;
  }

  .image-preview-actions {
    width: 100%;
    overflow-x: auto;
  }

  .image-preview-stage {
    padding: 0.25rem 0;
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
}
</style>
