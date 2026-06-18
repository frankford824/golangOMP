<template>
  <div ref="rootEl" class="apm" :class="imgClass">
    <template v-if="phase === 'loading'">
      <div class="apm-placeholder apm-loading" role="status" aria-live="polite">加载预览…</div>
    </template>
    <template v-else-if="phase === 'deferred'">
      <div class="apm-placeholder apm-muted" role="status">进入可视区域后加载预览</div>
    </template>
    <template v-else-if="phase === 'not_found'">
      <div class="apm-empty" role="img" aria-label="资源不存在">
        <img :src="placeholderSrc" alt="" class="apm-placeholder-img" />
      </div>
    </template>
    <template v-else-if="phase === 'unavailable'">
      <div class="apm-empty apm-empty--stack" aria-label="当前不可预览，仅可下载">
        <img :src="placeholderSrc" alt="" class="apm-placeholder-img" />
        <button type="button" class="apm-retry apm-retry--below" @click.stop="reload">重试</button>
      </div>
    </template>
    <template v-else-if="phase === 'preparing'">
      <div class="apm-placeholder apm-loading" role="status" aria-live="polite">
        <span>正在准备预览…</span>
        <button type="button" class="apm-retry apm-retry--below" @click.stop="reload">刷新</button>
      </div>
    </template>
    <template v-else-if="phase === 'error'">
      <div class="apm-empty apm-empty--stack" role="alert">
        <img :src="placeholderSrc" alt="" class="apm-placeholder-img" />
        <div class="apm-empty-caption">{{ errorHint }}</div>
        <button v-if="fallbackSrc" type="button" class="apm-retry" @click.stop="useFallbackOnly">
          使用缓存图
        </button>
      </div>
    </template>
    <template v-else-if="displaySrc">
      <img
        :src="displaySrc"
        :alt="alt || ''"
        class="apm-img"
        :class="innerImgClass"
        loading="lazy"
        @click.stop="onOpenFull"
      />
    </template>
    <template v-else>
      <div class="apm-empty" role="img" aria-label="无预览">
        <img :src="placeholderSrc" alt="" class="apm-placeholder-img" />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed, onBeforeUnmount, onMounted, nextTick } from 'vue'
import assetPreviewPlaceholder from '@/assets/default.png'
import { fetchAssetPreviewMeta } from '@/domain/asset-access'
import {
  materializePreviewImageUrl,
  revokeMaterializedPreviewImage,
  type MaterializedPreviewImage,
} from '@/domain/asset-preview-image'

const placeholderSrc = assetPreviewPlaceholder

const props = withDefaults(
  defineProps<{
    /** 资产根 id；缺省时仅用 fallbackSrc */
    assetId?: string | null
    /** 兼容某些后端实现以版本 id 作为 preview 路径参数 */
    fallbackAssetId?: string | null
    fallbackSrc?: string | null
    /**
     * 父级已解析的预览展示 URL（如详情弹窗已跑过 preview 主链），
     * 设置后不再重复请求 GET /preview。
     */
    resolvedPreviewUrl?: string | null
    alt?: string
    /** 附加在最外层容器 */
    imgClass?: string | string[] | Record<string, boolean>
    /** 附加在 img 上 */
    innerImgClass?: string | string[] | Record<string, boolean>
    /**
     * 为 true 时仅在根节点进入视口（含 margin）后再发起预览请求，用于缩略图网格等。
     */
    deferUntilVisible?: boolean
  }>(),
  {
    assetId: null,
    fallbackAssetId: null,
    fallbackSrc: null,
    resolvedPreviewUrl: null,
    alt: '',
    deferUntilVisible: false,
  },
)

const emit = defineEmits<{
  /** 用户点击可展示图时，传出用于灯箱的 URL 和资产上下文 */
  'open-full': [
    url: string,
    context: {
      assetId?: string
      fallbackAssetId?: string
      fallbackSrc?: string
      resolvedPreviewUrl?: string
    },
  ]
}>()

type Phase = 'idle' | 'deferred' | 'loading' | 'ready' | 'preparing' | 'not_found' | 'unavailable' | 'error'
const phase = ref<Phase>('idle')
const displaySrc = ref('')
const errorHint = ref('加载失败')
const rootEl = ref<HTMLElement | null>(null)

const innerImgClass = computed(() => props.innerImgClass)

let seq = 0
let materializedImage: MaterializedPreviewImage | null = null
let io: IntersectionObserver | null = null
let prepareRetryTimer: number | null = null
/** 已满足「进入视区」条件，或无需 defer */
const viewportGateOpen = ref(!props.deferUntilVisible)

function clearObjectUrl() {
  revokeMaterializedPreviewImage(materializedImage)
  materializedImage = null
}

function disconnectDeferIo() {
  if (io && rootEl.value) {
    try {
      io.unobserve(rootEl.value)
    } catch {
      /* ignore */
    }
  }
  io?.disconnect()
  io = null
}

function clearPrepareRetryTimer() {
  if (prepareRetryTimer != null) {
    window.clearTimeout(prepareRetryTimer)
    prepareRetryTimer = null
  }
}

function schedulePrepareRetry() {
  clearPrepareRetryTimer()
  prepareRetryTimer = window.setTimeout(() => {
    prepareRetryTimer = null
    void runLoad()
  }, 12_000)
}

function bindDeferIo() {
  disconnectDeferIo()
  if (!props.deferUntilVisible) {
    viewportGateOpen.value = true
    return
  }
  viewportGateOpen.value = false
  phase.value = 'deferred'
  const el = rootEl.value
  if (!el || typeof IntersectionObserver === 'undefined') {
    viewportGateOpen.value = true
    void runLoad()
    return
  }
  io = new IntersectionObserver(
    (entries) => {
      if (!entries.some((e) => e.isIntersecting)) return
      disconnectDeferIo()
      viewportGateOpen.value = true
      void runLoad()
    },
    { root: null, rootMargin: '180px 0px', threshold: 0.01 },
  )
  io.observe(el)
}

async function materializeDisplaySrc(url: string): Promise<string | undefined> {
  const image = await materializePreviewImageUrl(url)
  if (!image) return undefined
  clearObjectUrl()
  materializedImage = image
  return image.displaySrc
}

async function runLoad() {
  const my = ++seq
  clearPrepareRetryTimer()
  if (props.deferUntilVisible && !viewportGateOpen.value) {
    phase.value = 'deferred'
    return
  }

  const resolved = (props.resolvedPreviewUrl ?? '').trim()
  if (resolved) {
    phase.value = 'loading'
    clearObjectUrl()
    displaySrc.value = ''
    const renderable = await materializeDisplaySrc(resolved)
    if (my !== seq) return
    if (renderable) {
      displaySrc.value = renderable
      phase.value = 'ready'
    } else {
      errorHint.value = '无法展示该预览地址'
      phase.value = 'error'
    }
    return
  }

  const primaryId = props.assetId?.trim() || ''
  const secondaryId = props.fallbackAssetId?.trim() || ''
  if (!primaryId && !secondaryId) {
    clearObjectUrl()
    const fallback = await materializeDisplaySrc((props.fallbackSrc ?? '').trim())
    displaySrc.value = fallback ?? ''
    phase.value = displaySrc.value ? 'ready' : 'idle'
    return
  }
  phase.value = 'loading'
  clearObjectUrl()
  displaySrc.value = ''
  let res = await fetchAssetPreviewMeta(primaryId || secondaryId)
  const canTrySecondary =
    Boolean(primaryId) &&
    Boolean(secondaryId) &&
    secondaryId !== primaryId &&
    (res.status === 'not_found' || res.status === 'error')
  if (canTrySecondary) {
    res = await fetchAssetPreviewMeta(secondaryId)
  }
  if (my !== seq) return
  if (res.status === 'ok' && res.displayUrl) {
    const renderable = await materializeDisplaySrc(res.displayUrl)
    if (my !== seq) return
    if (renderable) {
      displaySrc.value = renderable
      phase.value = 'ready'
      return
    }
  }
  if (res.status === 'not_found') {
    phase.value = 'not_found'
    return
  }
  if (res.status === 'unavailable') {
    if (props.fallbackSrc?.trim()) {
      const fallback = await materializeDisplaySrc(props.fallbackSrc.trim())
      if (my !== seq) return
      if (fallback) {
        displaySrc.value = fallback
        phase.value = 'ready'
      } else {
        phase.value = 'unavailable'
      }
    } else {
      phase.value = 'unavailable'
    }
    return
  }
  if (res.status === 'preparing') {
    errorHint.value = res.message ?? '正在准备预览'
    phase.value = 'preparing'
    schedulePrepareRetry()
    return
  }
  errorHint.value = res.message ?? '加载失败'
  phase.value = 'error'
  if (props.fallbackSrc?.trim()) {
    const fallback = await materializeDisplaySrc(props.fallbackSrc.trim())
    if (my !== seq) return
    if (fallback) {
      displaySrc.value = fallback
      phase.value = 'ready'
    }
  }
}

function reload() {
  void runLoad()
}

function useFallbackOnly() {
  void (async () => {
    if (props.fallbackSrc?.trim()) {
      const fallback = await materializeDisplaySrc(props.fallbackSrc.trim())
      if (fallback) {
        displaySrc.value = fallback
        phase.value = 'ready'
      }
    }
  })()
}

function scheduleAfterPropChange() {
  if (props.deferUntilVisible) {
    viewportGateOpen.value = false
    phase.value = 'deferred'
    void nextTick(() => bindDeferIo())
  } else {
    viewportGateOpen.value = true
    void runLoad()
  }
}

watch(
  () =>
    [
      props.assetId,
      props.fallbackAssetId,
      props.fallbackSrc,
      props.resolvedPreviewUrl,
      props.deferUntilVisible,
    ] as const,
  () => {
    scheduleAfterPropChange()
  },
  { immediate: true },
)

onMounted(() => {
  if (props.deferUntilVisible) {
    void nextTick(() => bindDeferIo())
  }
})

onBeforeUnmount(() => {
  disconnectDeferIo()
  clearPrepareRetryTimer()
  clearObjectUrl()
})

function onOpenFull() {
  const u = displaySrc.value.trim()
  if (!u) return
  emit('open-full', u, {
    assetId: props.assetId?.trim() || undefined,
    fallbackAssetId: props.fallbackAssetId?.trim() || undefined,
    fallbackSrc: props.fallbackSrc?.trim() || undefined,
    resolvedPreviewUrl: props.resolvedPreviewUrl?.trim() || undefined,
  })
}
</script>

<style scoped>
.apm {
  position: relative;
  width: 100%;
  min-height: 2.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
}
.apm-img {
  display: block;
  max-width: 100%;
  max-height: 100%;
  border-radius: 0.5rem;
  object-fit: contain;
  cursor: zoom-in;
}
.apm-placeholder {
  width: 100%;
  min-height: 2.5rem;
  border-radius: 0.5rem;
  border: 1px dashed #cbd5e1;
  background: #f8fafc;
  color: #475569;
  font-size: 0.75rem;
  line-height: 1.35;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  gap: 0.25rem;
  padding: 0.5rem;
}
.apm-loading {
  color: #334155;
}
.apm-muted {
  color: #64748b;
}
.apm-bad {
  border-color: #fecaca;
  background: #fef2f2;
  color: #b91c1c;
}
.apm-warn {
  border-color: #fde68a;
  background: #fffbeb;
  color: #92400e;
}
.apm-subhint {
  font-size: 0.6875rem;
  color: #a16207;
}
.apm-retry {
  border: 1px solid #cbd5e1;
  border-radius: 999px;
  background: #fff;
  color: #0f172a;
  font-size: 0.6875rem;
  line-height: 1;
  padding: 0.25rem 0.5rem;
  cursor: pointer;
}
.apm-retry:hover {
  background: #f8fafc;
}
.apm-retry--below {
  flex-shrink: 0;
}
.apm-empty {
  width: 100%;
  flex: 1 1 auto;
  align-self: stretch;
  min-height: 2.5rem;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0.25rem;
}
.apm-empty--stack {
  flex-direction: column;
  gap: 0.5rem;
}
.apm-placeholder-img {
  display: block;
  max-width: 100%;
  max-height: 100%;
  width: auto;
  height: auto;
  object-fit: contain;
}
.apm-empty--stack .apm-placeholder-img {
  flex: 1 1 auto;
  min-height: 0;
}
.apm-empty-caption {
  font-size: 0.6875rem;
  color: #b91c1c;
  text-align: center;
  line-height: 1.35;
  max-width: 100%;
}
</style>
