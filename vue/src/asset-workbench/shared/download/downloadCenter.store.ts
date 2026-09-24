import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { MediaPreparationPending } from './preparedDownload'
import http from '@/services/http'
import { resolveMediaAccess, type MediaAccess } from '@/services/mediaDelivery'

import {
  transferDownload,
  type DownloadTransferMeta,
  type DownloadTransferProgress,
  type DownloadTransferResult,
} from './downloadTransfer'

export type DownloadCenterStatus = 'queued' | 'preparing' | 'waiting' | 'ready' | 'downloading' | 'completed' | 'handed_off' | 'failed' | 'cancelled'

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
  requestId?: string
  jobId?: string
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

interface PreparedRequestRow {
  request_id: string
  cancelled: boolean
  access_reference?: MediaAccess
  created_at: string
  updated_at: string
  job: { job_id: string; resource_id: string; filename?: string; state: string; phase: string; processed_bytes: number; total_bytes: number; error_code?: string }
}

export const useDownloadCenterStore = defineStore('assetWorkbenchDownloadCenter', () => {
  const items = ref<DownloadCenterItem[]>([])
  const panelOpen = ref(false)
  const resolvers = new Map<string, DownloadCenterRequest['resolve']>()
  const transfers = new Map<string, NonNullable<DownloadCenterRequest['transfer']>>()
  const controllers = new Map<string, AbortController>()
  const preparationNotice = ref('')
  let restoring = false
  let sessionEpoch = 0
  if (typeof window !== 'undefined') window.addEventListener('asset-media-session-reset', () => {
    sessionEpoch += 1
    for (const controller of controllers.values()) controller.abort()
    items.value = []; resolvers.clear(); transfers.clear(); controllers.clear(); panelOpen.value = false
  })

  async function refreshPreparationRequests() {
    if (restoring) return
    restoring = true
    const epoch = sessionEpoch
    try {
      const response = await http.get<{ data: PreparedRequestRow[] }>('/v1/assets/media/requests')
      if (epoch !== sessionEpoch) return
      for (const row of response.data.data || []) {
        const access = row.access_reference
        if (row.cancelled || !access || access.purpose !== 'download' || access.rendition !== 'original') continue
        let item = items.value.find((entry) => entry.requestId === row.request_id)
        if (item && (isActiveStatus(item.status) || item.status === 'handed_off')) continue
        const status: DownloadCenterStatus = row.job.state === 'succeeded' ? 'ready' : ['failed', 'stale'].includes(row.job.state) ? 'failed' : 'waiting'
        if (!item) {
          item = { id: `prepared:${row.request_id}`, key: `prepared:${row.request_id}`, requestId: row.request_id, jobId: row.job.job_id, displayName: row.job.filename || row.job.resource_id, sourceLabel: '后台准备', fileSize: 0, receivedBytes: 0, totalBytes: 0, speedBytesPerSecond: 0, progress: 0, status, createdAt: Date.parse(row.created_at), updatedAt: Date.parse(row.updated_at) }
          items.value.push(item)
        }
        updateItem(item, { status, error: status === 'failed' ? row.job.error_code || '准备失败' : status === 'ready' ? '文件已就绪，点击下载' : `${row.job.phase || '排队'}${row.job.total_bytes > 0 ? ` · ${row.job.processed_bytes}/${row.job.total_bytes} 字节` : ''}` })
        resolvers.set(item.id, async (signal) => {
          const info = await resolveMediaAccess(access, signal)
          if (!info.download_url && (info.state === 'queued' || info.state === 'processing')) throw new MediaPreparationPending(info.request_id || row.request_id, info.job_id || row.job.job_id)
          if (!info.download_url) throw new Error('文件尚不可下载，请刷新准备状态')
          return { downloadUrl: info.download_url, filename: info.filename, fileSize: info.file_size, mimeType: info.mime_type }
        })
      }
      preparationNotice.value = ''
    } catch { preparationNotice.value = '准备记录暂时无法刷新，可稍后重试' }
    finally { restoring = false }
  }

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
    if (!activeItems.value.length) return completedItems.value.length && !handedOffItems.value.length ? 100 : 0
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
    void refreshPreparationRequests()
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

    const retryable = items.value.find((item) => item.key === request.key && (item.status === 'failed' || item.status === 'cancelled' || item.status === 'waiting'))
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
    updateItem(item, { status: 'preparing', progress: 0, error: undefined })
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
      if (error instanceof MediaPreparationPending) {
        updateItem(item, { status: 'waiting', progress: 0, speedBytesPerSecond: 0, requestId: error.requestId, jobId: error.jobId, error: error.message })
      } else if (controller.signal.aborted || isAbortError(error)) {
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
      while (controllers.size < MAX_CONCURRENT_DOWNLOADS) {
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
    preparationNotice,
    refreshPreparationRequests,
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
