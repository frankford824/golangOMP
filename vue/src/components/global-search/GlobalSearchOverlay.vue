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
        :aria-busy="isSearching"
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
      <div class="mt-4 max-h-[min(70vh,640px)] space-y-4 overflow-y-auto pr-1">
        <div v-if="showMinimumHint" class="search-state-card" role="status">
          请输入至少 2 个字或完整编号，减少无效扫描。
        </div>
        <div v-else-if="isSearching && !hasAnyResults" class="search-skeleton" role="status" aria-live="polite">
          <span class="sr-only">正在搜索</span>
          <div v-for="index in 4" :key="index" class="search-skeleton-row">
            <span />
            <span />
          </div>
        </div>
        <div v-else-if="requestState === 'error'" class="search-state-card search-state-card--error" role="alert">
          <div>
            <strong>搜索暂时不可用</strong>
            <p>{{ errorMessage }}</p>
          </div>
          <button type="button" @click="retrySearch">重试</button>
        </div>
        <div v-else-if="requestState === 'success' && !hasAnyResults" class="search-state-card" role="status">
          没有找到匹配内容，可以换完整编号或更具体的关键词。
        </div>
        <section
          v-for="group in panelGroups"
          v-show="hasAnyResults"
          :key="group.key"
          class="rounded-lg border border-[var(--v1-border)] bg-[var(--v1-bg-surface-soft)] p-3"
        >
          <div class="mb-2 flex flex-wrap items-baseline justify-between gap-2">
            <p class="text-xs font-semibold text-[var(--v1-text-secondary)]">
              {{ group.label }}
              <span class="font-normal text-[var(--v1-text-muted)]">（{{ group.countLabel }}）</span>
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
          </div>
        </section>
      </div>
      <div class="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-[var(--v1-text-muted)]">
        <p v-if="slowHintVisible">搜索仍在进行，已自动减少弱网传输量...</p>
        <p v-else-if="requestState === 'refreshing'">已显示最近结果，正在后台更新。</p>
        <p v-if="networkConstrained">当前网络较慢，已启用省流量模式。</p>
      </div>
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
  type GlobalSearchOverlayBundle,
  type V1GlobalSearchScope,
} from '@/domain/global-search'
import { searchApi } from '@/services/api/searchApi'
import { usePermission } from '@/composables/usePermission'
import { PermissionEnum } from '@/types'

const PREVIEW_LIMIT = 5
const PREVIEW_FETCH_LIMIT = PREVIEW_LIMIT + 1
const CACHE_TTL_MS = 30_000
const CACHE_MAX_ENTRIES = 24
const FAST_DEBOUNCE_MS = 220
const CONSTRAINED_DEBOUNCE_MS = 480

type SearchRequestState = 'idle' | 'debouncing' | 'loading' | 'refreshing' | 'success' | 'error'
type NetworkInformationLike = EventTarget & {
  effectiveType?: string
  saveData?: boolean
  rtt?: number
  downlink?: number
}
type NavigatorWithConnection = Navigator & {
  connection?: NetworkInformationLike
}
type CacheEntry = {
  bundle: GlobalSearchOverlayBundle
  savedAt: number
}

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [boolean] }>()
const router = useRouter()
const inputRef = ref<HTMLInputElement | null>(null)
const { can } = usePermission()

const keyword = ref('')
const scope = ref<V1GlobalSearchScope>('all')
const slowHintVisible = ref(false)
const resultBundle = ref(emptyGlobalSearchBundle())
const activeFlatIndex = ref(0)
const requestState = ref<SearchRequestState>('idle')
const errorMessage = ref('')
const networkConstrained = ref(false)
const online = ref(true)
const resultCache = new Map<string, CacheEntry>()

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
  const showUsers = can(PermissionEnum.ACCESS_VIEW) || can(PermissionEnum.ACCESS_MANAGE)
  const base = allGroups.value.filter((g) => g.key !== 'users' || showUsers)
  if (scope.value === 'all') return base
  return base.filter((g) => g.key === scope.value)
})

type PanelGroup = {
  key: 'tasks' | 'assets' | 'products' | 'users'
  label: string
  items: GlobalSearchOverlayHit[]
  totalCount: number
  countLabel: string
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
      countLabel: isPreview && full.length > PREVIEW_LIMIT ? `${PREVIEW_LIMIT}+` : String(full.length),
      hasMore: isPreview && full.length > PREVIEW_LIMIT,
    }
  })
})

const hasAnyResults = computed(() =>
  panelGroups.value.some((group) => group.items.length > 0),
)

const normalizedKeyword = computed(() => keyword.value.trim())
const showMinimumHint = computed(() =>
  normalizedKeyword.value.length > 0 && Array.from(normalizedKeyword.value).length < 2,
)
const isSearching = computed(() =>
  requestState.value === 'debouncing' ||
  requestState.value === 'loading' ||
  requestState.value === 'refreshing',
)

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

function cacheKey(q: string, searchScope: V1GlobalSearchScope, limit: number): string {
  return `${searchScope}:${limit}:${q.toLocaleLowerCase()}`
}

function readCachedResult(key: string): GlobalSearchOverlayBundle | null {
  const cached = resultCache.get(key)
  if (!cached) return null
  if (Date.now() - cached.savedAt > CACHE_TTL_MS) {
    resultCache.delete(key)
    return null
  }
  resultCache.delete(key)
  resultCache.set(key, cached)
  return cached.bundle
}

function writeCachedResult(key: string, bundle: GlobalSearchOverlayBundle): void {
  resultCache.delete(key)
  resultCache.set(key, { bundle, savedAt: Date.now() })
  while (resultCache.size > CACHE_MAX_ENTRIES) {
    const oldest = resultCache.keys().next().value
    if (typeof oldest !== 'string') break
    resultCache.delete(oldest)
  }
}

function requestLimit(searchScope: V1GlobalSearchScope): number {
  if (searchScope === 'all') return PREVIEW_FETCH_LIMIT
  return networkConstrained.value ? 10 : 20
}

function syncNetworkProfile(): void {
  if (typeof navigator === 'undefined') return
  online.value = navigator.onLine
  const connection = (navigator as NavigatorWithConnection).connection
  const effectiveType = connection?.effectiveType?.toLowerCase() ?? ''
  networkConstrained.value =
    !navigator.onLine ||
    connection?.saveData === true ||
    ['slow-2g', '2g', '3g'].includes(effectiveType) ||
    Number(connection?.rtt ?? 0) >= 600 ||
    (Number(connection?.downlink ?? 0) > 0 && Number(connection?.downlink ?? 0) < 1.5)
}

async function performSearch(q: string, searchScope: V1GlobalSearchScope): Promise<void> {
  if (!online.value) {
    requestState.value = 'error'
    errorMessage.value = '当前处于离线状态，请恢复网络后重试。'
    return
  }
  const limit = requestLimit(searchScope)
  const key = cacheKey(q, searchScope, limit)
  const cached = readCachedResult(key)
  if (cached) {
    resultBundle.value = cached
    requestState.value = 'refreshing'
  } else {
    requestState.value = 'loading'
  }
  searchAbort?.abort()
  const ac = new AbortController()
  searchAbort = ac
  slowHintVisible.value = false
  slowHintTimer = window.setTimeout(() => {
    if (!ac.signal.aborted && open.value) slowHintVisible.value = true
  }, networkConstrained.value ? 900 : 650)
  try {
    const res = await searchApi.query({ keyword: q, scope: searchScope, limit }, ac.signal)
    if (ac.signal.aborted || normalizedKeyword.value !== q || scope.value !== searchScope) return
    const body = parseV1GlobalSearchResponse(res.data)
    const bundle = body
      ? mapV1SearchResultsToOverlayBundle(body.results)
      : emptyGlobalSearchBundle()
    resultBundle.value = bundle
    writeCachedResult(key, bundle)
    activeFlatIndex.value = 0
    requestState.value = 'success'
    errorMessage.value = ''
  } catch {
    if (ac.signal.aborted) return
    if (!cached) resultBundle.value = emptyGlobalSearchBundle()
    requestState.value = 'error'
    errorMessage.value = navigator.onLine
      ? '服务响应失败，已有结果不会被清空。'
      : '网络已断开，请恢复后重试。'
  } finally {
    if (!ac.signal.aborted) {
      if (slowHintTimer !== undefined) {
        window.clearTimeout(slowHintTimer)
        slowHintTimer = undefined
      }
      slowHintVisible.value = false
    }
  }
}

function scheduleSearch(): void {
  const q = normalizedKeyword.value
  if (!open.value || !q || Array.from(q).length < 2) {
    clearSearchTimers()
    searchAbort?.abort()
    searchAbort = undefined
    resultBundle.value = emptyGlobalSearchBundle()
    slowHintVisible.value = false
    errorMessage.value = ''
    requestState.value = 'idle'
    return
  }
  clearSearchTimers()
  searchAbort?.abort()
  requestState.value = 'debouncing'
  errorMessage.value = ''
  const qAtSchedule = q
  const scopeAtSchedule = scope.value
  debounceTimer = window.setTimeout(
    () => void performSearch(qAtSchedule, scopeAtSchedule),
    networkConstrained.value ? CONSTRAINED_DEBOUNCE_MS : FAST_DEBOUNCE_MS,
  )
}

watch([keyword, scope, open, networkConstrained], scheduleSearch)

function retrySearch(): void {
  clearSearchTimers()
  syncNetworkProfile()
  const q = normalizedKeyword.value
  if (q && Array.from(q).length >= 2) void performSearch(q, scope.value)
}

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

function onOnlineStateChange(): void {
  const wasOffline = !online.value
  syncNetworkProfile()
  if (wasOffline && online.value && open.value && normalizedKeyword.value.length >= 2) {
    retrySearch()
  }
}

onMounted(() => {
  syncNetworkProfile()
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('online', onOnlineStateChange)
  window.addEventListener('offline', onOnlineStateChange)
  ;(navigator as NavigatorWithConnection).connection?.addEventListener('change', syncNetworkProfile)
})
onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('online', onOnlineStateChange)
  window.removeEventListener('offline', onOnlineStateChange)
  ;(navigator as NavigatorWithConnection).connection?.removeEventListener('change', syncNetworkProfile)
  clearSearchTimers()
  searchAbort?.abort()
})
</script>

<style scoped>
.global-search-overlay.fixed.inset-0 {
  z-index: 7000;
  background: rgb(var(--yb-shadow) / 0.16);
}

.search-state-card {
  display: flex;
  min-height: 5rem;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border: 1px solid var(--v1-border);
  border-radius: 0.75rem;
  background: var(--v1-bg-surface-soft);
  padding: 1rem;
  color: var(--v1-text-secondary);
  font-size: 0.82rem;
}

.search-state-card strong {
  display: block;
  color: var(--v1-text-primary);
}

.search-state-card p {
  margin: 0.2rem 0 0;
}

.search-state-card button {
  flex: none;
  border: 1px solid var(--v1-border);
  border-radius: 0.55rem;
  background: rgb(var(--yb-surface));
  padding: 0.45rem 0.8rem;
  color: var(--v1-text-primary);
  font-weight: 700;
}

.search-state-card--error {
  border-color: rgb(var(--yb-danger) / 0.3);
  background: rgb(var(--yb-danger-soft));
}

.search-skeleton {
  display: grid;
  gap: 0.55rem;
  border: 1px solid var(--v1-border);
  border-radius: 0.75rem;
  padding: 0.9rem;
}

.search-skeleton-row {
  display: grid;
  gap: 0.35rem;
}

.search-skeleton-row span {
  height: 0.72rem;
  border-radius: 999px;
  background: linear-gradient(
    90deg,
    rgb(var(--yb-surface-muted)),
    rgb(var(--yb-surface)),
    rgb(var(--yb-surface-muted))
  );
  background-size: 220% 100%;
  animation: search-skeleton-shimmer 1.2s ease-in-out infinite;
}

.search-skeleton-row span:first-child {
  width: min(65%, 25rem);
}

.search-skeleton-row span:last-child {
  width: min(42%, 17rem);
}

@keyframes search-skeleton-shimmer {
  to {
    background-position: -220% 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .search-skeleton-row span {
    animation: none;
  }
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

</style>
