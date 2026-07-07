import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { driveUploadRelativePath } from './useDriveUpload'

export type UploadCenterStatus = 'queued' | 'uploading' | 'uploaded' | 'submitting' | 'submitted' | 'failed'
export type UploadCenterSource = 'upload-page' | 'drive-dialog'

export interface UploadCenterItem {
  id: string
  source: UploadCenterSource
  file: File
  relativePath: string
  displayName: string
  fileSize: number
  progress: number
  status: UploadCenterStatus
  finalized: boolean
  pageCount: number
  uploadDirectoryId?: number
  uploadDirectoryName?: string
  difficultyClass?: string
  sessionId?: string
  error?: string
  createdAt: number
  updatedAt: number
}

interface AddUploadCenterItemsOptions {
  source: UploadCenterSource
  uploadDirectoryId?: number
  uploadDirectoryName?: string
  difficultyClass?: string
  finalized?: boolean
  pageCount?: number
  preserveIds?: string[]
}

const MAX_VISIBLE_HISTORY = 80

export const useUploadCenterStore = defineStore('assetWorkbenchUploadCenter', () => {
  const items = ref<UploadCenterItem[]>([])
  const panelOpen = ref(false)
  const compact = ref(true)

  const activeItems = computed(() =>
    items.value.filter((item) => item.status === 'queued' || item.status === 'uploading' || item.status === 'submitting'),
  )
  const failedItems = computed(() => items.value.filter((item) => item.status === 'failed'))
  const pendingRecordItems = computed(() => items.value.filter((item) => item.status === 'uploaded'))
  const finishedItems = computed(() => items.value.filter((item) => item.status === 'submitted'))
  const visibleItems = computed(() =>
    [...items.value]
      .sort((a, b) => {
        const activeDelta = statusWeight(a.status) - statusWeight(b.status)
        if (activeDelta !== 0) return activeDelta
        return b.updatedAt - a.updatedAt
      })
      .slice(0, MAX_VISIBLE_HISTORY),
  )
  const uploadPageItems = computed(() =>
    items.value.filter((item) => item.source === 'upload-page' && item.status !== 'submitted'),
  )
  const hasItems = computed(() => items.value.length > 0)
  const hasActive = computed(() => activeItems.value.length > 0)
  const overallProgress = computed(() => {
    if (!items.value.length) return 0
    const weighted = items.value.reduce((sum, item) => sum + itemProgressValue(item), 0)
    return Math.round(weighted / items.value.length)
  })
  const summaryText = computed(() => {
    if (!items.value.length) return '暂无上传任务'
    if (hasActive.value) return `上传中 ${activeItems.value.length} 个 · 待生成记录 ${pendingRecordItems.value.length} 个 · 已完成 ${finishedItems.value.length} 个`
    if (failedItems.value.length) return `失败 ${failedItems.value.length} 个 · 待生成记录 ${pendingRecordItems.value.length} 个`
    if (pendingRecordItems.value.length) return `待生成记录 ${pendingRecordItems.value.length} 个`
    return `已完成 ${finishedItems.value.length} 个`
  })

  function openPanel() {
    panelOpen.value = true
    compact.value = false
  }

  function closePanel() {
    panelOpen.value = false
    compact.value = true
  }

  function addItems(files: File[], options: AddUploadCenterItemsOptions): UploadCenterItem[] {
    const now = Date.now()
    const next = files.map((file, index) => {
      const relativePath = driveUploadRelativePath(file)
      return {
        id: options.preserveIds?.[index] || crypto.randomUUID?.() || `${now}-${index}-${Math.random()}`,
        source: options.source,
        file,
        relativePath,
        displayName: relativePath || file.name,
        fileSize: file.size,
        progress: 0,
        status: 'queued' as UploadCenterStatus,
        finalized: options.finalized ?? true,
        pageCount: options.pageCount ?? 1,
        uploadDirectoryId: options.uploadDirectoryId,
        uploadDirectoryName: options.uploadDirectoryName,
        difficultyClass: options.difficultyClass,
        createdAt: now,
        updatedAt: now,
      }
    })
    for (const item of next) {
      const index = items.value.findIndex((current) => current.id === item.id)
      if (index >= 0) items.value[index] = { ...items.value[index], ...item }
      else items.value.push(item)
    }
    if (next.length) openPanel()
    return next
  }

  function updateItem(id: string, patch: Partial<Omit<UploadCenterItem, 'id' | 'createdAt'>>) {
    const item = items.value.find((current) => current.id === id)
    if (!item) return
    Object.assign(item, patch, { updatedAt: Date.now() })
  }

  function updateItems(ids: string[], patch: Partial<Omit<UploadCenterItem, 'id' | 'createdAt'>>) {
    for (const id of ids) updateItem(id, patch)
  }

  function removeItem(id: string) {
    const item = items.value.find((current) => current.id === id)
    if (item?.status === 'uploading' || item?.status === 'submitting') return
    items.value = items.value.filter((current) => current.id !== id)
  }

  function removeItems(ids: string[]) {
    const idSet = new Set(ids)
    items.value = items.value.filter((item) => !idSet.has(item.id) || item.status === 'uploading' || item.status === 'submitting')
  }

  function clearFinished() {
    items.value = items.value.filter((item) => item.status !== 'submitted')
    if (!items.value.length) closePanel()
  }

  function clearIdle() {
    items.value = items.value.filter((item) => item.status === 'uploading' || item.status === 'submitting')
    if (!items.value.length) closePanel()
  }

  return {
    items,
    panelOpen,
    compact,
    activeItems,
    failedItems,
    pendingRecordItems,
    finishedItems,
    visibleItems,
    uploadPageItems,
    hasItems,
    hasActive,
    overallProgress,
    summaryText,
    openPanel,
    closePanel,
    addItems,
    updateItem,
    updateItems,
    removeItem,
    removeItems,
    clearFinished,
    clearIdle,
  }
})

function itemProgressValue(item: UploadCenterItem) {
  if (item.status === 'submitted') return 100
  if (item.status === 'uploaded' || item.status === 'submitting') return 96
  if (item.status === 'failed') return Math.max(0, Math.min(100, item.progress))
  return Math.max(0, Math.min(100, item.progress))
}

function statusWeight(status: UploadCenterStatus) {
  if (status === 'uploading' || status === 'submitting') return 0
  if (status === 'failed') return 1
  if (status === 'queued') return 2
  if (status === 'uploaded') return 3
  return 4
}
