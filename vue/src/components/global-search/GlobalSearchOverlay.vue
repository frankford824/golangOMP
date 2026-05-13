<template>
  <div v-if="open" class="fixed inset-0 z-50 bg-slate-900/40 p-6" @click.self="close">
    <div
      class="mx-auto max-w-4xl rounded-xl border border-[var(--v1-border)] bg-white p-4 shadow-2xl"
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
              ? 'bg-[var(--v1-bg-primary)] text-white'
              : 'bg-[var(--v1-bg-surface-soft)] text-[var(--v1-text-secondary)]'
          "
          @click="scope = item"
        >
          {{ scopeLabel(item) }}
        </button>
      </div>
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
                isActiveResult(group, rIdx) ? 'bg-white shadow-sm' : 'hover:bg-white/70',
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
                  v-if="result.statusLabel"
                  class="max-w-[5.5rem] truncate rounded bg-white px-1.5 py-0.5 text-[10px] text-[var(--v1-text-secondary)] ring-1 ring-[var(--v1-border)]"
                >
                  {{ result.statusLabel }}
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

function highlight(text: string): string {
  const key = keyword.value.trim()
  if (!key) return text
  const escaped = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return text.replace(new RegExp(escaped, 'gi'), (match) => `<mark class="bg-amber-100">${match}</mark>`)
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

onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  clearSearchTimers()
  searchAbort?.abort()
})
</script>
