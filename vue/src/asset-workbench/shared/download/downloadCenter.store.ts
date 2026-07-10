import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import {
  transferDownload,
  type DownloadTransferMeta,
  type DownloadTransferProgress,
  type DownloadTransferResult,
} from './downloadTransfer'

export type DownloadCenterStatus = 'queued' | 'preparing' | 'downloading' | 'completed' | 'handed_off' | 'failed' | 'cancelled'

export interface DownloadCenterItem {
  id: string
  key: string
  displayName: string
  sourceLabel: string
  fileSize: number
  receivedBytes: number
  totalBytes: number
  speedBytesPerSecond: number
  progress: number
  status: DownloadCenterStatus
  error?: string
  createdAt: number
  updatedAt: number
}

export interface DownloadCenterRequest {
  key: string
  displayName: string
  sourceLabel?: string
  fileSize?: number
  resolve: (signal: AbortSignal) => Promise<DownloadTransferMeta>
  transfer?: (
    meta: DownloadTransferMeta,
    signal: AbortSignal,
    onProgress: (progress: DownloadTransferProgress) => void,
  ) => Promise<DownloadTransferResult>
}

export interface DownloadCenterEnqueueResult {
  item: DownloadCenterItem
  duplicate: boolean
}

const MAX_CONCURRENT_DOWNLOADS = 2
const MAX_VISIBLE_HISTORY = 60

export const useDownloadCenterStore = defineStore('assetWorkbenchDownloadCenter', () => {
  const items = ref<DownloadCenterItem[]>([])
  const panelOpen = ref(false)
  const resolvers = new Map<string, DownloadCenterRequest['resolve']>()
  const transfers = new Map<string, NonNullable<DownloadCenterRequest['transfer']>>()
  const controllers = new Map<string, AbortController>()

  const activeItems = computed(() => items.value.filter((item) => isActiveStatus(item.status)))
  const failedItems = computed(() => items.value.filter((item) => item.status === 'failed'))
  const completedItems = computed(() => items.value.filter((item) => item.status === 'completed'))
  const handedOffItems = computed(() => items.value.filter((item) => item.status === 'handed_off'))
  const hasItems = computed(() => items.value.length > 0)
  const hasActive = computed(() => activeItems.value.length > 0)
  const visibleItems = computed(() =>
    [...items.value]
      .sort((a, b) => {
        const activeDelta = statusWeight(a.status) - statusWeight(b.status)
        if (activeDelta !== 0) return activeDelta
        return b.updatedAt - a.updatedAt
      })
      .slice(0, MAX_VISIBLE_HISTORY),
  )
  const overallProgress = computed(() => {
    if (!activeItems.value.length) return completedItems.value.length || handedOffItems.value.length ? 100 : 0
    return Math.round(activeItems.value.reduce((sum, item) => sum + item.progress, 0) / activeItems.value.length)
  })
  const summaryText = computed(() => {
    if (!items.value.length) return '暂无下载任务'
    if (hasActive.value) return '下载中 ' + activeItems.value.length + ' 个 · 已完成 ' + completedItems.value.length + ' 个'
    if (failedItems.value.length) return '需要处理 ' + failedItems.value.length + ' 个 · 已完成 ' + completedItems.value.length + ' 个'
    if (handedOffItems.value.length) return '浏览器下载 ' + handedOffItems.value.length + ' 个 · 已完成 ' + completedItems.value.length + ' 个'
    return '已完成 ' + completedItems.value.length + ' 个'
  })

  function openPanel() {
    panelOpen.value = true
  }

  function closePanel() {
    panelOpen.value = false
  }

  function enqueue(request: DownloadCenterRequest): DownloadCenterEnqueueResult {
    const existing = items.value.find((item) => item.key === request.key && isActiveStatus(item.status))
    if (existing) {
      openPanel()
      return { item: existing, duplicate: true }
    }

    const retryable = items.value.find((item) => item.key === request.key && (item.status === 'failed' || item.status === 'cancelled'))
    if (retryable) {
      resolvers.set(retryable.id, request.resolve)
      if (request.transfer) transfers.set(retryable.id, request.transfer)
      else transfers.delete(retryable.id)
      Object.assign(retryable, {
        displayName: request.displayName || retryable.displayName,
        sourceLabel: request.sourceLabel || retryable.sourceLabel,
        fileSize: positiveNumber(request.fileSize) || retryable.fileSize,
      })
      retry(retryable.id)
      return { item: retryable, duplicate: false }
    }

    const now = Date.now()
    const item: DownloadCenterItem = {
      id: crypto.randomUUID?.() || now + '-' + Math.random(),
      key: request.key,
      displayName: request.displayName || '下载文件',
      sourceLabel: request.sourceLabel || '素材文件',
      fileSize: positiveNumber(request.fileSize),
      receivedBytes: 0,
      totalBytes: positiveNumber(request.fileSize),
      speedBytesPerSecond: 0,
      progress: 0,
      status: 'queued',
      createdAt: now,
      updatedAt: now,
    }
    items.value.push(item)
    resolvers.set(item.id, request.resolve)
    if (request.transfer) transfers.set(item.id, request.transfer)
    openPanel()
    schedulePump()
    return { item, duplicate: false }
  }

  function cancel(id: string) {
    const item = itemByID(id)
    if (!item || !isActiveStatus(item.status)) return
    if (item.status === 'queued') {
      updateItem(item, { status: 'cancelled', error: undefined, speedBytesPerSecond: 0 })
      schedulePump()
      return
    }
    controllers.get(id)?.abort()
  }

  function retry(id: string) {
    const item = itemByID(id)
    if (!item || !resolvers.has(id) || isActiveStatus(item.status)) return
    updateItem(item, {
      status: 'queued',
      receivedBytes: 0,
      totalBytes: item.fileSize,
      speedBytesPerSecond: 0,
      progress: 0,
      error: undefined,
    })
    openPanel()
    schedulePump()
  }

  function removeItem(id: string) {
    const item = itemByID(id)
    if (!item || isActiveStatus(item.status)) return
    items.value = items.value.filter((current) => current.id !== id)
    resolvers.delete(id)
    transfers.delete(id)
    controllers.delete(id)
    if (!items.value.length) closePanel()
  }

  function clearFinished() {
    const removable = items.value.filter((item) => item.status === 'completed' || item.status === 'handed_off' || item.status === 'cancelled')
    for (const item of removable) {
      resolvers.delete(item.id)
      transfers.delete(item.id)
      controllers.delete(item.id)
    }
    const ids = new Set(removable.map((item) => item.id))
    items.value = items.value.filter((item) => !ids.has(item.id))
    if (!items.value.length) closePanel()
  }

  async function runTask(id: string) {
    const item = itemByID(id)
    const resolve = resolvers.get(id)
    if (!item || item.status !== 'queued' || !resolve) return

    const controller = new AbortController()
    controllers.set(id, controller)
    updateItem(item, { status: 'preparing', progress: 2, error: undefined })
    try {
      const meta = await resolve(controller.signal)
      if (controller.signal.aborted) throw new DOMException('下载已取消', 'AbortError')
      updateItem(item, {
        displayName: meta.filename || item.displayName,
        fileSize: positiveNumber(meta.fileSize) || item.fileSize,
        totalBytes: positiveNumber(meta.fileSize) || item.totalBytes,
        status: 'downloading',
        progress: 0,
      })
      const transfer = transfers.get(id) ?? transferDownload
      const result = await transfer(meta, controller.signal, (progress) => {
        updateItem(item, {
          receivedBytes: progress.receivedBytes,
          totalBytes: progress.totalBytes || item.totalBytes,
          speedBytesPerSecond: progress.speedBytesPerSecond,
          progress: progress.progress,
        })
      })
      if (result.mode === 'browser') {
        updateItem(item, {
          status: 'handed_off',
          receivedBytes: 0,
          totalBytes: result.totalBytes || item.totalBytes,
          speedBytesPerSecond: 0,
          progress: 0,
        })
      } else {
        updateItem(item, {
          status: 'completed',
          receivedBytes: result.receivedBytes,
          totalBytes: result.totalBytes,
          speedBytesPerSecond: result.speedBytesPerSecond,
          progress: 100,
        })
      }
    } catch (error) {
      if (controller.signal.aborted || isAbortError(error)) {
        updateItem(item, { status: 'cancelled', speedBytesPerSecond: 0, error: undefined })
      } else {
        updateItem(item, {
          status: 'failed',
          speedBytesPerSecond: 0,
          error: error instanceof Error ? error.message : '下载失败，请稍后重试',
        })
      }
    } finally {
      controllers.delete(id)
      schedulePump()
    }
  }

  let pumpScheduled = false
  function schedulePump() {
    if (pumpScheduled) return
    pumpScheduled = true
    queueMicrotask(() => {
      pumpScheduled = false
      while (activeItems.value.length < MAX_CONCURRENT_DOWNLOADS) {
        const next = items.value.find((item) => item.status === 'queued')
        if (!next) break
        void runTask(next.id)
      }
    })
  }

  function itemByID(id: string) {
    return items.value.find((item) => item.id === id)
  }

  function updateItem(item: DownloadCenterItem, patch: Partial<DownloadCenterItem>) {
    Object.assign(item, patch, { updatedAt: Date.now() })
  }

  return {
    items,
    panelOpen,
    activeItems,
    failedItems,
    completedItems,
    handedOffItems,
    visibleItems,
    hasItems,
    hasActive,
    overallProgress,
    summaryText,
    openPanel,
    closePanel,
    enqueue,
    cancel,
    retry,
    removeItem,
    clearFinished,
  }
})

function positiveNumber(value: unknown): number {
  const number = Number(value)
  return Number.isFinite(number) && number > 0 ? number : 0
}

function isActiveStatus(status: DownloadCenterStatus) {
  return status === 'queued' || status === 'preparing' || status === 'downloading'
}

function statusWeight(status: DownloadCenterStatus) {
  if (status === 'preparing' || status === 'downloading') return 0
  if (status === 'queued') return 1
  if (status === 'failed') return 2
  if (status === 'cancelled') return 3
  return 4
}

function isAbortError(error: unknown) {
  return error instanceof DOMException && error.name === 'AbortError'
}
