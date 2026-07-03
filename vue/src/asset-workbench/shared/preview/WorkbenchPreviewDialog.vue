<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { Download, ExternalLink, Maximize2, Minimize2, Minus } from 'lucide-vue-next'

import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'
import IconfontActionIcon from '@aw/shared/icons/IconfontActionIcon.vue'
import { isPdfMimeOrFilename, isVideoMimeOrFilename } from '@aw/shared/materials/systemAssetPreview'

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
  windowed?: boolean
}>()

const emit = defineEmits<{
  close: []
  download: []
}>()

const displayUrl = computed(() => (props.previewUrl || props.fallbackSrc || '').trim())
const isPdf = computed(() => isPdfMimeOrFilename(props.mimeType, props.filename || props.title))
const isVideo = computed(() => isVideoMimeOrFilename(props.mimeType, props.filename || props.title))
const showPdfFrame = computed(() => isPdf.value && Boolean(displayUrl.value))
const showVideoPreview = computed(() => isVideo.value && Boolean(displayUrl.value))
const showImagePreview = computed(() => !isPdf.value && !isVideo.value && Boolean(displayUrl.value))
const windowMode = ref<'normal' | 'maximized' | 'minimized'>('normal')
const windowRect = reactive({ left: 96, top: 64, width: 1120, height: 720 })
const isWindowMaximized = computed(() => props.windowed === true && windowMode.value === 'maximized')
const isWindowMinimized = computed(() => props.windowed === true && windowMode.value === 'minimized')
const previewWindowStyle = computed(() => {
  if (!props.windowed) return undefined
  if (isWindowMaximized.value) {
    const margin = 12
    return {
      '--aw-preview-window-left': `${margin}px`,
      '--aw-preview-window-top': `${margin}px`,
      '--aw-preview-window-width': `calc(100vw - ${margin * 2}px)`,
      '--aw-preview-window-height': `calc(100vh - ${margin * 2}px)`,
    }
  }
  return {
    '--aw-preview-window-left': `${windowRect.left}px`,
    '--aw-preview-window-top': `${windowRect.top}px`,
    '--aw-preview-window-width': `${windowRect.width}px`,
    '--aw-preview-window-height': `${windowRect.height}px`,
  }
})

let interaction:
  | {
      type: 'move' | 'resize'
      pointerID: number
      startX: number
      startY: number
      left: number
      top: number
      width: number
      height: number
    }
  | null = null

const minWindowWidth = 720
const minWindowHeight = 440

function viewportSize() {
  if (typeof window === 'undefined') return { width: 1280, height: 800 }
  return { width: window.innerWidth, height: window.innerHeight }
}

function centerPreviewWindow() {
  const viewport = viewportSize()
  const margin = 24
  const width = Math.min(1120, Math.max(minWindowWidth, viewport.width - margin * 2))
  const height = Math.min(720, Math.max(minWindowHeight, viewport.height - margin * 2))
  windowRect.width = width
  windowRect.height = height
  windowRect.left = Math.max(margin, Math.round((viewport.width - width) / 2))
  windowRect.top = Math.max(margin, Math.round((viewport.height - height) / 2))
}

function clampPreviewWindow() {
  const viewport = viewportSize()
  const margin = 8
  windowRect.width = Math.min(Math.max(windowRect.width, minWindowWidth), Math.max(minWindowWidth, viewport.width - margin * 2))
  windowRect.height = Math.min(Math.max(windowRect.height, minWindowHeight), Math.max(minWindowHeight, viewport.height - margin * 2))
  windowRect.left = Math.min(Math.max(margin, windowRect.left), Math.max(margin, viewport.width - windowRect.width - margin))
  windowRect.top = Math.min(Math.max(margin, windowRect.top), Math.max(margin, viewport.height - windowRect.height - margin))
}

function handleKeydown(event: KeyboardEvent) {
  if (!props.open || event.key !== 'Escape') return
  event.preventDefault()
  event.stopPropagation()
  event.stopImmediatePropagation()
  emit('close')
}

function startWindowMove(event: PointerEvent) {
  if (!props.windowed || isWindowMaximized.value || event.button !== 0) return
  event.preventDefault()
  interaction = {
    type: 'move',
    pointerID: event.pointerId,
    startX: event.clientX,
    startY: event.clientY,
    left: windowRect.left,
    top: windowRect.top,
    width: windowRect.width,
    height: windowRect.height,
  }
  window.addEventListener('pointermove', handleWindowPointerMove)
  window.addEventListener('pointerup', stopWindowInteraction, { once: true })
}

function startWindowResize(event: PointerEvent) {
  if (!props.windowed || isWindowMaximized.value || event.button !== 0) return
  event.preventDefault()
  event.stopPropagation()
  interaction = {
    type: 'resize',
    pointerID: event.pointerId,
    startX: event.clientX,
    startY: event.clientY,
    left: windowRect.left,
    top: windowRect.top,
    width: windowRect.width,
    height: windowRect.height,
  }
  window.addEventListener('pointermove', handleWindowPointerMove)
  window.addEventListener('pointerup', stopWindowInteraction, { once: true })
}

function handleWindowPointerMove(event: PointerEvent) {
  if (!interaction || event.pointerId !== interaction.pointerID) return
  if (interaction.type === 'move') {
    windowRect.left = interaction.left + event.clientX - interaction.startX
    windowRect.top = interaction.top + event.clientY - interaction.startY
    clampPreviewWindow()
    return
  }
  windowRect.width = interaction.width + event.clientX - interaction.startX
  windowRect.height = interaction.height + event.clientY - interaction.startY
  clampPreviewWindow()
}

function stopWindowInteraction() {
  interaction = null
  window.removeEventListener('pointermove', handleWindowPointerMove)
}

function minimizeWindow() {
  if (!props.windowed) return
  windowMode.value = 'minimized'
}

function restoreWindow() {
  if (!props.windowed) return
  windowMode.value = 'normal'
  clampPreviewWindow()
}

function toggleMaximizeWindow() {
  if (!props.windowed) return
  if (isWindowMinimized.value) {
    restoreWindow()
    return
  }
  windowMode.value = isWindowMaximized.value ? 'normal' : 'maximized'
  if (windowMode.value === 'normal') clampPreviewWindow()
}

function handleStageClick() {
  if (!props.windowed) emit('close')
}

function openInNewWindow() {
  const url = displayUrl.value
  if (!url) return
  window.open(url, '_blank', 'noopener,noreferrer')
}

let previousBodyOverflow = ''

function syncBodyScrollLock(open: boolean) {
  if (typeof document === 'undefined') return
  if (props.windowed) {
    document.body.style.overflow = previousBodyOverflow
    return
  }
  if (open) {
    previousBodyOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return
  }
  document.body.style.overflow = previousBodyOverflow
}

watch(
  () => [props.open, props.windowed] as const,
  (open) => {
    syncBodyScrollLock(open[0])
    if (open[0] && open[1]) {
      windowMode.value = 'normal'
      centerPreviewWindow()
    }
  },
  { immediate: true },
)

watch(
  () => props.title,
  () => {
    if (props.open && props.windowed) {
      windowMode.value = 'normal'
      centerPreviewWindow()
    }
  },
)

onMounted(() => {
  window.addEventListener('keydown', handleKeydown, { capture: true })
  window.addEventListener('resize', clampPreviewWindow)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown, { capture: true })
  window.removeEventListener('resize', clampPreviewWindow)
  stopWindowInteraction()
  syncBodyScrollLock(false)
})
</script>

<template>
  <Teleport to="body">
    <button
      v-if="open && isWindowMinimized"
      class="aw-token-scope aw-preview-dialog__dock"
      type="button"
      @click="restoreWindow"
    >
      <span>{{ title || '素材预览' }}</span>
      <strong>恢复</strong>
    </button>
    <section
      v-if="open && !isWindowMinimized"
      class="aw-token-scope aw-preview-dialog"
      :class="{ 'aw-preview-dialog--windowed': windowed, 'is-maximized': isWindowMaximized }"
      :style="previewWindowStyle"
      role="dialog"
      :aria-modal="windowed ? 'false' : 'true'"
      :aria-label="title || '素材预览'"
    >
      <header v-if="windowed" class="aw-preview-dialog__titlebar" @pointerdown="startWindowMove" @dblclick="toggleMaximizeWindow">
        <div class="aw-preview-dialog__traffic" aria-hidden="true">
          <span />
          <span />
          <span />
        </div>
        <div class="aw-preview-dialog__title">
          <span>{{ eyebrow || '预览' }}</span>
          <strong>{{ title }}</strong>
        </div>
        <div class="aw-preview-dialog__window-actions">
          <button type="button" title="最小化" aria-label="最小化" @pointerdown.stop @click="minimizeWindow">
            <Minus :size="15" aria-hidden="true" />
          </button>
          <button type="button" :title="isWindowMaximized ? '还原' : '最大化'" :aria-label="isWindowMaximized ? '还原' : '最大化'" @pointerdown.stop @click="toggleMaximizeWindow">
            <Minimize2 v-if="isWindowMaximized" :size="15" aria-hidden="true" />
            <Maximize2 v-else :size="15" aria-hidden="true" />
          </button>
          <button type="button" title="关闭" aria-label="关闭" @pointerdown.stop @click="emit('close')">
            <IconfontActionIcon name="close" :size="15" />
          </button>
        </div>
      </header>
      <div class="aw-preview-dialog__content">
        <div class="aw-preview-dialog__stage" @click.self="handleStageClick">
          <iframe
            v-if="showPdfFrame"
            class="aw-preview-dialog__media aw-preview-dialog__pdf"
            :src="displayUrl"
            :title="title"
          />
          <video
            v-else-if="showVideoPreview"
            class="aw-preview-dialog__media aw-preview-dialog__video"
            :src="displayUrl"
            controls
            preload="metadata"
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
              <IconfontActionIcon name="close" :size="16" />
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
      </div>
      <button v-if="windowed && !isWindowMaximized" class="aw-preview-dialog__resize" type="button" aria-label="调整窗口大小" @pointerdown="startWindowResize" />
    </section>
  </Teleport>
</template>
