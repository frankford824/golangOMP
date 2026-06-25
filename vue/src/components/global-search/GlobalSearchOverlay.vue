<template>
  <div v-if="open" class="global-search-overlay fixed inset-0 z-50 bg-[rgb(var(--yb-shadow)/0.4)] p-6" @click.self="close">
    <div
      class="global-search-panel mx-auto max-w-4xl rounded-xl border border-[var(--v1-border)] bg-[rgb(var(--yb-surface))] p-4 shadow-2xl"
      role="dialog"
      aria-modal="true"
      aria-label="全局搜索"
    >
      <input
        ref="inputRef"
        v-model="keyword"
        class="w-full rounded-md border border-[var(--v1-border)] bg-[var(--v1-bg-surface-soft)] px-3 py-2 text-sm text-[var(--v1-text-primary)] outline-none"
        placeholder="搜索任务、资产、产品、用户"
      />
      <div class="mt-3 flex flex-wrap gap-2 text-xs">
        <button
          v-for="item in scopes"
          :key="item"
          type="button"
          class="rounded-full px-2.5 py-1"
          :class="
            scope === item
              ? 'bg-[var(--v1-bg-primary)] text-[rgb(var(--yb-text-inverse))]'
              : 'bg-[var(--v1-bg-surface-soft)] text-[var(--v1-text-secondary)]'
          "
          @click="scope = item"
        >
          {{ scopeLabel(item) }}
        </button>
      </div>
      <section
        v-if="predictionSuggestions.length"
        class="global-prediction-section mt-4 rounded-lg border border-[rgb(var(--yb-brand-border))] bg-[rgb(var(--yb-brand-soft)/0.7)] p-3"
      >
        <div class="mb-2 flex items-center justify-between gap-2">
          <p class="text-xs font-semibold text-[rgb(var(--yb-text-body))]">预测提示</p>
          <span class="text-[10px] text-[rgb(var(--yb-text-muted))]">不消耗 AI 调用</span>
        </div>
        <div class="grid gap-2 sm:grid-cols-2">
          <button
            v-for="item in predictionSuggestions"
            :key="item.id"
            type="button"
            class="prediction-card"
            @click="goPrediction(item)"
          >
            <span class="prediction-source">{{ item.source || '系统提示' }}</span>
            <strong>{{ item.title }}</strong>
            <small v-if="item.detail">{{ item.detail }}</small>
            <em>{{ item.action_label || '查看' }}</em>
          </button>
        </div>
      </section>
      <div class="mt-4 max-h-[min(70vh,640px)] space-y-4 overflow-y-auto pr-1">
        <section
          v-for="group in panelGroups"
          :key="group.key"
          class="rounded-lg border border-[var(--v1-border)] bg-[var(--v1-bg-surface-soft)] p-3"
        >
          <div class="mb-2 flex flex-wrap items-baseline justify-between gap-2">
            <p class="text-xs font-semibold text-[var(--v1-text-secondary)]">
              {{ group.label }}
              <span class="font-normal text-[var(--v1-text-muted)]">（{{ group.totalCount }}）</span>
            </p>
            <button
              v-if="group.hasMore"
              type="button"
              class="text-xs font-medium text-[var(--v1-text-primary)] underline decoration-[var(--v1-border)] underline-offset-2 hover:opacity-80"
              @click="goToScope(group.key)"
            >
              查看全部{{ group.label }}
            </button>
          </div>
          <div class="max-h-[min(48vh,320px)] space-y-1 overflow-y-auto text-sm text-[var(--v1-text-primary)]">
            <button
              v-for="(result, rIdx) in group.items"
              :key="result.id + group.key"
              type="button"
              class="flex w-full items-start gap-2 rounded px-2 py-2 text-left transition-colors"
              :class="[
                isActiveResult(group, rIdx) ? 'bg-[rgb(var(--yb-surface))] shadow-sm' : 'hover:bg-[rgb(var(--yb-surface))]/70',
                isNavigable(group.key) ? '' : 'cursor-default',
              ]"
              @mouseenter="setActive(group, rIdx)"
              @click="goResult(result, group.key)"
            >
              <div class="min-w-0 flex-1">
                <div
                  class="truncate font-medium leading-snug text-[var(--v1-text-primary)]"
                  v-html="highlight(result.title)"
                />
                <div
                  v-if="result.subtitle"
                  class="mt-0.5 truncate text-xs leading-snug text-[var(--v1-text-secondary)]"
                  v-html="highlight(result.subtitle)"
                />
              </div>
              <div class="flex shrink-0 flex-col items-end gap-1">
                <span
                  v-if="result.badgeLabel || result.statusLabel"
                  class="max-w-[5.5rem] truncate rounded bg-[rgb(var(--yb-surface))] px-1.5 py-0.5 text-[10px] text-[var(--v1-text-secondary)] ring-1 ring-[var(--v1-border)]"
                >
                  {{ result.badgeLabel || result.statusLabel }}
                </span>
                <span
                  v-if="!isNavigable(group.key)"
                  class="rounded-full bg-[var(--v1-bg-surface-soft)] px-1.5 text-[10px] text-[var(--v1-text-muted)]"
                >
                  只读
                </span>
              </div>
            </button>
            <p v-if="group.totalCount === 0" class="px-2 py-3 text-xs text-[var(--v1-text-muted)]">
              无结果
            </p>
          </div>
        </section>
      </div>
      <p v-if="slowHintVisible" class="mt-2 text-xs text-[var(--v1-text-muted)]">搜索较慢，正在查询...</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  emptyGlobalSearchBundle,
  mapV1SearchResultsToOverlayBundle,
  parseV1GlobalSearchResponse,
  type GlobalSearchOverlayHit,
  type V1GlobalSearchScope,
} from '@/domain/global-search'
import { searchApi } from '@/services/api/searchApi'
import { predictionsApi, type PredictionSuggestion } from '@/services/api/predictionsApi'
import { useAuth } from '@/composables/useAuth'

const PREVIEW_LIMIT = 5

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [boolean] }>()
const router = useRouter()
const inputRef = ref<HTMLInputElement | null>(null)
const { isDeptAdminPlus } = useAuth()

const keyword = ref('')
const scope = ref<V1GlobalSearchScope>('all')
const slowHintVisible = ref(false)
const resultBundle = ref(emptyGlobalSearchBundle())
const predictionSuggestions = ref<PredictionSuggestion[]>([])
const activeFlatIndex = ref(0)

const scopes: V1GlobalSearchScope[] = ['all', 'tasks', 'assets', 'products', 'users']

const open = computed(() => props.open)

const scopeLabel = (s: V1GlobalSearchScope): string => {
  const m: Record<V1GlobalSearchScope, string> = {
    all: '全部',
    tasks: '任务',
    assets: '资产',
    products: '产品',
    users: '用户',
  }
  return m[s]
}

const allGroups = computed(() => [
  { key: 'tasks' as const, label: '任务', items: resultBundle.value.tasks },
  { key: 'assets' as const, label: '资产', items: resultBundle.value.assets },
  { key: 'products' as const, label: '产品', items: resultBundle.value.products },
  { key: 'users' as const, label: '用户', items: resultBundle.value.users },
])

const filteredGroups = computed(() => {
  const showUsers = isDeptAdminPlus.value
  const base = allGroups.value.filter((g) => g.key !== 'users' || showUsers)
  if (scope.value === 'all') return base
  return base.filter((g) => g.key === scope.value)
})

type PanelGroup = {
  key: 'tasks' | 'assets' | 'products' | 'users'
  label: string
  items: GlobalSearchOverlayHit[]
  totalCount: number
  hasMore: boolean
}

const panelGroups = computed((): PanelGroup[] => {
  const isPreview = scope.value === 'all'
  return filteredGroups.value.map((g) => {
    const full = g.items
    const items = isPreview ? full.slice(0, PREVIEW_LIMIT) : full
    return {
      key: g.key,
      label: g.label,
      items,
      totalCount: full.length,
      hasMore: isPreview && full.length > PREVIEW_LIMIT,
    }
  })
})

const flatResults = computed(() => {
  const out: { groupKey: string; index: number; result: GlobalSearchOverlayHit }[] = []
  for (const g of panelGroups.value) {
    g.items.forEach((result, index) => {
      out.push({ groupKey: g.key, index, result })
    })
  }
  return out
})

let debounceTimer: number | undefined
let slowHintTimer: number | undefined
let searchAbort: AbortController | undefined
let predictionTimer: number | undefined
let predictionAbort: AbortController | undefined

function clearSearchTimers(): void {
  if (debounceTimer !== undefined) {
    window.clearTimeout(debounceTimer)
    debounceTimer = undefined
  }
  if (slowHintTimer !== undefined) {
    window.clearTimeout(slowHintTimer)
    slowHintTimer = undefined
  }
}

function clearPredictionTimer(): void {
  if (predictionTimer !== undefined) {
    window.clearTimeout(predictionTimer)
    predictionTimer = undefined
  }
}

watch([keyword, scope, open], () => {
  if (!open.value || !keyword.value.trim()) {
    clearSearchTimers()
    searchAbort?.abort()
    searchAbort = undefined
    resultBundle.value = emptyGlobalSearchBundle()
    slowHintVisible.value = false
    return
  }
  clearSearchTimers()
  searchAbort?.abort()
  searchAbort = new AbortController()
  const ac = searchAbort
  slowHintVisible.value = false
  slowHintTimer = window.setTimeout(() => {
    if (open.value && keyword.value.trim()) slowHintVisible.value = true
  }, 600)
  debounceTimer = window.setTimeout(async () => {
    const q = keyword.value.trim()
    const sc = scope.value
    try {
      const res = await searchApi.query({ keyword: q, scope: sc }, ac.signal)
      if (ac.signal.aborted) return
      const body = parseV1GlobalSearchResponse(res.data)
      resultBundle.value = body
        ? mapV1SearchResultsToOverlayBundle(body.results)
        : emptyGlobalSearchBundle()
      activeFlatIndex.value = 0
    } catch {
      if (ac.signal.aborted) return
      resultBundle.value = emptyGlobalSearchBundle()
    } finally {
      if (!ac.signal.aborted) {
        if (slowHintTimer !== undefined) {
          window.clearTimeout(slowHintTimer)
          slowHintTimer = undefined
        }
        slowHintVisible.value = false
      }
    }
  }, 300)
})

watch([keyword, scope, open], () => {
  clearPredictionTimer()
  predictionAbort?.abort()
  predictionAbort = undefined
  if (!open.value) {
    predictionSuggestions.value = []
    return
  }
  predictionSuggestions.value = []
  const ac = new AbortController()
  predictionAbort = ac
  predictionTimer = window.setTimeout(async () => {
    try {
      const bundle = await predictionsApi.search(
        { keyword: keyword.value.trim(), scope: scope.value, limit: 6 },
        ac.signal,
      )
      if (ac.signal.aborted) return
      predictionSuggestions.value = bundle.suggestions
    } catch {
      if (!ac.signal.aborted) predictionSuggestions.value = []
    }
  }, 250)
})

function goToScope(s: Exclude<V1GlobalSearchScope, 'all'>): void {
  scope.value = s
}

function isActiveResult(group: PanelGroup, rIdx: number): boolean {
  const flat = flatResults.value[activeFlatIndex.value]
  return Boolean(flat && flat.groupKey === group.key && flat.index === rIdx)
}

function setActive(group: PanelGroup, rIdx: number): void {
  const i = flatResults.value.findIndex((f) => f.groupKey === group.key && f.index === rIdx)
  if (i >= 0) {
    activeFlatIndex.value = i
  }
}

function close(): void {
  emit('update:open', false)
}

const HTML_ESCAPE_MAP: Record<string, string> = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;',
}

function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (ch) => HTML_ESCAPE_MAP[ch] ?? ch)
}

// 先按原文切分匹配段，再对每段做 HTML 转义，避免后端返回内容经 v-html 注入。
function highlight(text: string): string {
  const key = keyword.value.trim()
  if (!key) return escapeHtml(text)
  const escapedPattern = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const re = new RegExp(escapedPattern, 'gi')
  let out = ''
  let last = 0
  for (const match of text.matchAll(re)) {
    const idx = match.index ?? 0
    out += escapeHtml(text.slice(last, idx))
    out += `<mark class="bg-[rgb(var(--yb-warning-highlight))]">${escapeHtml(match[0])}</mark>`
    last = idx + match[0].length
  }
  out += escapeHtml(text.slice(last))
  return out
}

function isNavigable(groupKey: string): boolean {
  return groupKey === 'tasks' || groupKey === 'assets'
}

function goResult(result: GlobalSearchOverlayHit, groupKey: string): void {
  if (!isNavigable(groupKey)) return
  if (groupKey === 'tasks') {
    void router.push(`/tasks/${result.id}`)
  } else if (groupKey === 'assets') {
    void router.push(`/asset-center/${result.id}`)
  }
  close()
}

function goPrediction(item: PredictionSuggestion): void {
  const targetID = String(item.target_id ?? '').trim()
  switch (item.target_type) {
    case 'task':
      if (targetID) void router.push(`/tasks/${targetID}`)
      break
    case 'asset':
      if (targetID) void router.push(`/asset-center/${targetID}`)
      break
    case 'task_center':
      void router.push('/tasks')
      break
    case 'data_center':
      void router.push('/data-center')
      break
    case 'logs':
      void router.push('/data-center')
      break
    default:
      if (item.action_type === 'open_task' && targetID) void router.push(`/tasks/${targetID}`)
      else if (item.action_type === 'open_asset' && targetID) void router.push(`/asset-center/${targetID}`)
      else return
  }
  close()
}

function onKeydown(event: KeyboardEvent): void {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    emit('update:open', !open.value)
  }
  if (event.key === 'Escape' && open.value) {
    close()
  }
  if (!open.value) return
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    activeFlatIndex.value = Math.min(
      activeFlatIndex.value + 1,
      Math.max(0, flatResults.value.length - 1),
    )
  }
  if (event.key === 'ArrowUp') {
    event.preventDefault()
    activeFlatIndex.value = Math.max(activeFlatIndex.value - 1, 0)
  }
  if (event.key === 'Enter') {
    const item = flatResults.value[activeFlatIndex.value]
    if (item) {
      goResult(item.result, item.groupKey)
    }
  }
}

watch(open, async (value) => {
  if (!value) return
  await nextTick()
  inputRef.value?.focus()
})

onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  clearSearchTimers()
  clearPredictionTimer()
  searchAbort?.abort()
  predictionAbort?.abort()
})
</script>

<style scoped>
.global-search-overlay.fixed.inset-0 {
  z-index: 7000;
  background: rgb(var(--yb-shadow) / 0.16);
}

.global-search-panel.mx-auto.max-w-4xl {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  box-shadow: 0 24px 48px -18px rgb(var(--yb-shadow) / 0.18);
}

.global-search-panel input {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
  color: rgb(var(--yb-text));
}

.global-search-panel input:focus,
.global-search-panel input:focus-visible {
  border-color: rgb(var(--yb-border));
  box-shadow: none;
}

:global(#app .global-search-panel input:focus),
:global(#app .global-search-panel input:focus-visible) {
  border-color: rgb(var(--yb-border));
}

.global-search-panel input::placeholder {
  color: rgb(var(--yb-text-faint));
}

.global-search-panel button {
  border: 1px solid rgb(var(--yb-border));
  color: rgb(var(--yb-text-body));
}

.global-prediction-section {
  background:
    linear-gradient(
      120deg,
      rgb(var(--yb-brand) / 0.08),
      rgb(var(--yb-info-accent) / 0.08),
      rgb(var(--yb-brand) / 0.08)
    ),
    rgb(var(--yb-info-wash));
  background-size: 220% 100%;
  animation: global-stream-panel 8s linear infinite;
}

.prediction-card {
  position: relative;
  display: grid;
  gap: 0.25rem;
  min-height: 5.25rem;
  padding: 0.625rem 0.75rem;
  overflow: hidden;
  border: 1px solid rgb(var(--yb-info-cyan-border));
  border-radius: 0.5rem;
  background: rgb(var(--yb-surface));
  text-align: left;
  transition: transform 180ms ease, border-color 180ms ease, box-shadow 180ms ease;
  animation: global-card-enter 420ms ease both;
}

.prediction-card:hover {
  transform: translateY(-2px);
  border-color: rgb(var(--yb-info-cyan-border-strong));
  box-shadow: 0 14px 28px -22px rgb(var(--yb-info-cyan-shadow) / 0.8);
}

.prediction-card::after {
  content: "";
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: linear-gradient(110deg, transparent 0%, rgb(var(--yb-info-accent) / 0.14) 42%, transparent 72%);
  transform: translateX(-120%);
  transition: transform 650ms ease;
}

.prediction-card:hover::after {
  transform: translateX(120%);
}

.prediction-card strong {
  color: rgb(var(--yb-text-navy));
  font-size: 0.8125rem;
  line-height: 1.3;
}

.prediction-card small {
  color: rgb(var(--yb-text-soft));
  font-size: 0.75rem;
  line-height: 1.35;
}

.prediction-card em,
.prediction-source {
  width: max-content;
  max-width: 100%;
  border-radius: 999px;
  font-style: normal;
  font-size: 0.6875rem;
  line-height: 1.2;
}

.prediction-source {
  color: rgb(var(--yb-info-text));
}

.prediction-card em {
  margin-top: 0.125rem;
  padding: 0.125rem 0.45rem;
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
}

@keyframes global-stream-panel {
  from { background-position: 0% 50%; }
  to { background-position: 220% 50%; }
}

@keyframes global-card-enter {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.global-search-panel section {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
}

.global-search-panel p,
.global-search-panel div,
.global-search-panel span {
  color: rgb(var(--yb-text-body));
}

.global-search-panel .shadow-sm {
  box-shadow: 0 0 0 1px rgb(var(--yb-brand) / 0.12);
}

.global-search-panel :deep(mark) {
  border-radius: 0.25rem;
  background: rgb(var(--yb-brand-subtle));
  color: rgb(var(--yb-brand-context-strong));
}

@media (prefers-reduced-motion: reduce) {
  .global-prediction-section,
  .prediction-card {
    animation: none;
  }

  .prediction-card,
  .prediction-card::after {
    transition: none;
  }
}
</style>
