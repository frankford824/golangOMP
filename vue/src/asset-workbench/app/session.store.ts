import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { assetWorkbenchApi, type AssetWorkbenchBootstrap, type WorkbenchEntryResult } from '@aw/shared/api/assetWorkbenchApi'
import { hasAnyCapability as checkAnyCapability } from './access'

interface RefreshOptions {
  force?: boolean
  background?: boolean
}

const STALE_AFTER_MS = 60_000

export const useAssetWorkbenchSessionStore = defineStore('assetWorkbenchSession', () => {
  const bootstrap = ref<AssetWorkbenchBootstrap | null>(null)
  const entry = ref<WorkbenchEntryResult | null>(null)
  const loading = ref(false)
  const entryLoading = ref(false)
  const error = ref('')
  const entryError = ref('')
  const lastLoadedAt = ref(0)
  const lastEntryLoadedAt = ref(0)

  let bootstrapController: AbortController | null = null
  let entryController: AbortController | null = null
  let bootstrapPromise: Promise<AssetWorkbenchBootstrap | null> | null = null
  let entryPromise: Promise<WorkbenchEntryResult | null> | null = null

  const isAdmin = computed(() => bootstrap.value?.is_admin === true)
  const capabilities = computed(() => bootstrap.value?.capabilities ?? [])
  const isBootstrapStale = computed(() => !lastLoadedAt.value || Date.now() - lastLoadedAt.value > STALE_AFTER_MS)
  const isEntryStale = computed(() => !lastEntryLoadedAt.value || Date.now() - lastEntryLoadedAt.value > STALE_AFTER_MS)

  function hasAnyCapability(required: readonly string[] = []) {
    return checkAnyCapability(bootstrap.value, required)
  }

  async function refresh(options: RefreshOptions = {}): Promise<AssetWorkbenchBootstrap | null> {
    if (bootstrap.value && !options.force) {
      if (isBootstrapStale.value && !bootstrapPromise) {
        void refresh({ force: true, background: true })
      }
      return bootstrap.value
    }
    if (bootstrapPromise) return bootstrapPromise

    bootstrapController?.abort()
    bootstrapController = new AbortController()
    if (!options.background) loading.value = true
    error.value = ''

    bootstrapPromise = assetWorkbenchApi
      .bootstrap(bootstrapController.signal)
      .then((next) => {
        bootstrap.value = next
        lastLoadedAt.value = Date.now()
        return next
      })
      .catch((err) => {
        if (!options.background || !bootstrap.value) {
          error.value = err instanceof Error ? err.message : '加载工作台信息失败'
        }
        return bootstrap.value
      })
      .finally(() => {
        if (!options.background) loading.value = false
        bootstrapPromise = null
      })

    return bootstrapPromise
  }

  async function loadEntry(options: RefreshOptions = {}): Promise<WorkbenchEntryResult | null> {
    if (entry.value && !options.force) {
      if (entry.value.bootstrap && !bootstrap.value) setBootstrap(entry.value.bootstrap)
      if (isEntryStale.value && !entryPromise) {
        void loadEntry({ force: true, background: true })
      }
      return entry.value
    }
    if (entryPromise) return entryPromise

    entryController?.abort()
    entryController = new AbortController()
    if (!options.background) entryLoading.value = true
    entryError.value = ''

    entryPromise = assetWorkbenchApi
      .entry(entryController.signal)
      .then((next) => {
        entry.value = next
        lastEntryLoadedAt.value = Date.now()
        if (next.bootstrap) setBootstrap(next.bootstrap)
        return next
      })
      .catch((err) => {
        if (!options.background || !entry.value) {
          entryError.value = err instanceof Error ? err.message : '工作台入口加载失败'
        }
        return entry.value
      })
      .finally(() => {
        if (!options.background) entryLoading.value = false
        entryPromise = null
      })

    return entryPromise
  }

  async function ensureReady() {
    const currentEntry = await loadEntry()
    if (currentEntry?.state === 'ready' && !bootstrap.value) {
      await refresh()
    }
    return bootstrap.value
  }

  function setBootstrap(next: AssetWorkbenchBootstrap | null) {
    bootstrap.value = next
    lastLoadedAt.value = next ? Date.now() : 0
    error.value = ''
  }

  function setEntry(next: WorkbenchEntryResult | null) {
    entry.value = next
    lastEntryLoadedAt.value = next ? Date.now() : 0
    if (next?.bootstrap) setBootstrap(next.bootstrap)
    entryError.value = ''
  }

  function reset() {
    bootstrapController?.abort()
    entryController?.abort()
    bootstrapPromise = null
    entryPromise = null
    bootstrap.value = null
    entry.value = null
    loading.value = false
    entryLoading.value = false
    error.value = ''
    entryError.value = ''
    lastLoadedAt.value = 0
    lastEntryLoadedAt.value = 0
  }

  return {
    bootstrap,
    entry,
    loading,
    entryLoading,
    error,
    entryError,
    capabilities,
    isAdmin,
    refresh,
    loadEntry,
    ensureReady,
    setBootstrap,
    setEntry,
    reset,
    hasAnyCapability,
  }
})
