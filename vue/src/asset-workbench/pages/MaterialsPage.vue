<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { ChevronLeft, ChevronRight, Download, FileImage, Grid3X3, List, Search, X } from 'lucide-vue-next'

import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'
import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import {
  assetWorkbenchApi,
  type ClientMaterialRow,
  type SystemAssetPreviewMeta,
  type SystemAssetRow,
  type UploadDirectoryRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { buildTimestampedZipFilename, downloadBatchAsZip } from '@/utils/batchZipDownload'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { formatInt } from '@aw/shared/format/number'
import { chipClass, systemPreviewMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import MaterialGallery from '@aw/shared/materials/MaterialGallery.vue'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'
import {
  canAttemptSystemAssetPreview,
  isSystemAssetImagePreviewable,
  resolvedSystemAssetPreviewUrl,
} from '@aw/shared/materials/systemAssetPreview'

interface GridColumn {
  key: string
  label: string
  width: number
  align?: 'left' | 'right' | 'center'
}

type ViewMode = 'gallery' | 'table'

type MaterialGridRow = SystemAssetRow & {
  display_style_code: string
  display_name: string
  display_type: string
  preview_label: string
  task_label: string
  created_label: string
}

const keyword = ref('')
const rows = ref<SystemAssetRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(30)
const selectedAssetIds = ref<Set<number>>(new Set())
const activeAsset = ref<SystemAssetRow | null>(null)
const previewAsset = ref<SystemAssetRow | null>(null)
const previewMeta = ref<SystemAssetPreviewMeta | null>(null)
const previewUrls = ref<Record<number, string>>({})
const previewLoadingIds = ref<Set<number>>(new Set())
const viewMode = ref<ViewMode>('gallery')
const typeFilter = ref('all')
const previewFilter = ref('all')
const previewLoading = ref(false)
const notice = ref('')
const { bootstrap } = useAssetWorkbenchBootstrap()
const isSimpleUser = computed(() => bootstrap.value?.is_admin === false)
const clientMaterials = ref<ClientMaterialRow[]>([])
const adminClientMaterials = ref<ClientMaterialRow[]>([])
const uploadDirectories = ref<UploadDirectoryRow[]>([])
const selectedClientMaterialIds = ref<Set<number>>(new Set())
const clientMaterialsLoading = ref(false)
const clientMaterialKeyword = ref('')
const clientPreviewUrls = ref<Record<number, string>>({})
const clientPreviewLoadingIds = ref<Set<number>>(new Set())
const clientMaterialAssetId = ref<number | null>(null)
const clientMaterialTitle = ref('')
const clientMaterialDescription = ref('')
const directoryName = ref('')
const directoryPrefix = ref('')
const directoryDescription = ref('')
const directoryDifficulty = ref('A')
const directoryDifficultyOptions = ['A', 'B', 'C', 'A+小夜灯']
const materialsRequest = usePageRequest(
  (signal) => assetWorkbenchApi.systemSearch({ q: keyword.value, page: page.value, page_size: pageSize.value }, signal),
  { items: [], total: 0, page: 1, size: 0 },
  '素材库搜索失败',
)
const loading = materialsRequest.loading
const error = materialsRequest.error
let previewController: AbortController | null = null
let lastSelectedIndex = -1
let initializedMode: 'admin' | 'client' | '' = ''

const downloadableAssetIds = computed(() => filteredRows.value.map((row) => row.id).filter((id) => id > 0))
const fileTypeOptions = computed(() => {
  const values = new Set<string>()
  for (const row of rows.value) values.add(typeBucket(row))
  return Array.from(values).sort()
})
const filteredRows = computed(() =>
  rows.value.filter((row) => {
    if (typeFilter.value !== 'all' && typeBucket(row) !== typeFilter.value) return false
    const canPreview = canPreviewMaterial(row)
    if (previewFilter.value === 'previewable' && !canPreview) return false
    if (previewFilter.value === 'download_only' && canPreview) return false
    return true
  }),
)
const materialRowsWithLabels = computed<MaterialGridRow[]>(() =>
  filteredRows.value.map((row) => ({
    ...row,
    display_style_code: styleCodeOf(row) || '—',
    display_name: titleOf(row),
    display_type: typeLabel(row),
    preview_label: systemPreviewMeta(canPreviewMaterial(row)).label,
    task_label: row.task_no || '无任务号',
    created_label: formatDateTime(row.created_at),
  })),
)
const materialGridRows = computed(() => materialRowsWithLabels.value as unknown as Record<string, unknown>[])
const materialGridColumns = computed<GridColumn[]>(() => [
  { key: 'select', label: '选择', width: 84, align: 'center' },
  { key: 'display_style_code', label: 'SKU', width: 170 },
  { key: 'display_name', label: '素材名称', width: 260 },
  { key: 'display_type', label: '类型', width: 120 },
  { key: 'task_label', label: '任务', width: 150 },
  { key: 'created_label', label: '创建时间', width: 170 },
  { key: 'preview_label', label: '预览', width: 120 },
  { key: 'actions', label: '动作', width: 140, align: 'center' },
])
const selectedCount = computed(() => selectedAssetIds.value.size)
const selectedLabel = computed(() => (selectedCount.value > 0 ? `已选 ${formatInt(selectedCount.value)} 个素材` : '未选择素材'))
const selectedClientCount = computed(() => selectedClientMaterialIds.value.size)
const selectedClientLabel = computed(() => (selectedClientCount.value > 0 ? `已选 ${formatInt(selectedClientCount.value)} 个素材` : '未选择素材'))
const filteredClientMaterials = computed(() => {
  const query = clientMaterialKeyword.value.trim().toLowerCase()
  if (!query) return clientMaterials.value
  return clientMaterials.value.filter((material) => clientMaterialSearchText(material).includes(query))
})
const filteredClientMaterialIds = computed(() => filteredClientMaterials.value.map((item) => item.id))
const allFilteredClientMaterialsSelected = computed(() => {
  const ids = filteredClientMaterialIds.value
  return ids.length > 0 && ids.every((id) => selectedClientMaterialIds.value.has(id))
})
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
const activeDetailRows = computed(() => {
  const asset = activeAsset.value
  if (!asset) return []
  return [
    ['SKU', styleCodeOf(asset) || '—'],
    ['名称', titleOf(asset)],
    ['文件类型', typeLabel(asset)],
    ['任务号', asset.task_no || '—'],
    ['产品名', asset.product_name || '—'],
    ['创建人', creatorOf(asset) || '—'],
    ['创建时间', formatDateTime(asset.created_at)],
    ['原文件', asset.original_filename || asset.file_name || '—'],
  ]
})
const relatedAssets = computed(() => {
  const asset = activeAsset.value
  if (!asset) return []
  return rows.value
    .filter((item) => item.id !== asset.id)
    .filter((item) => {
      const sameProduct = Boolean(asset.product_name && item.product_name === asset.product_name)
      const sameTask = Boolean(asset.task_no && item.task_no === asset.task_no)
      const sameType = Boolean(asset.mime_type && item.mime_type === asset.mime_type)
      return sameProduct || sameTask || sameType
    })
    .slice(0, 8)
})

async function searchMaterials(resetPage = true) {
  if (resetPage) page.value = 1
  notice.value = ''
  const result = await materialsRequest.run()
  if (!result) return
  rows.value = result.items
  total.value = result.total
  selectedAssetIds.value = new Set()
  activeAsset.value = result.items[0] ?? null
  previewUrls.value = {}
  lastSelectedIndex = -1
}

async function goMaterialsPage(nextPage: number) {
  page.value = Math.min(totalPages.value, Math.max(1, nextPage))
  await searchMaterials(false)
}

function titleOf(asset: SystemAssetRow) {
  return asset.product_name || asset.original_filename || asset.file_name || asset.task_no || `素材 ${asset.id}`
}

function styleCodeOf(asset: SystemAssetRow) {
  return asset.scope_sku_code || asset.sku_code || asset.primary_sku_code || ''
}

function clientMaterialTitleOf(material: ClientMaterialRow) {
  return material.title || material.filename_snapshot || `素材 ${material.asset_id}`
}

function clientMaterialSkuOf(material: ClientMaterialRow) {
  return material.scope_sku_code || material.sku_code || material.primary_sku_code || ''
}

function clientMaterialFilenameOf(material: ClientMaterialRow) {
  return material.filename_snapshot || `asset_id=${material.asset_id}`
}

function clientMaterialPreviewProbe(material: ClientMaterialRow): SystemAssetPreviewMeta {
  return {
    asset_id: material.asset_id,
    status: material.preview_available ? 'ready' : 'not_applicable',
    preparing: false,
    filename: material.filename_snapshot,
    mime_type: material.mime_type_snapshot,
    preview_available: Boolean(material.preview_available),
  }
}

function canPreviewClientMaterial(material: ClientMaterialRow) {
  return canAttemptSystemAssetPreview(clientMaterialPreviewProbe(material))
}

function clientMaterialPreviewUrl(material: ClientMaterialRow) {
  return clientPreviewUrls.value[material.id] || ''
}

function clientMaterialSearchText(material: ClientMaterialRow) {
  return [
    clientMaterialTitleOf(material),
    clientMaterialSkuOf(material),
    clientMaterialFilenameOf(material),
    material.description,
    String(material.asset_id),
  ]
    .join(' ')
    .toLowerCase()
}

function creatorOf(asset: SystemAssetRow) {
  return asset.created_by_name || asset.created_by_username || asset.task_creator_name || asset.task_creator_username || ''
}

function formatDateTime(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function typeBucket(asset: SystemAssetRow) {
  const mime = (asset.mime_type || '').toLowerCase()
  if (mime.includes('photoshop')) return 'PSD'
  if (mime.includes('pdf')) return 'PDF'
  if (mime.startsWith('image/')) return '图片'
  if (mime.includes('zip') || mime.includes('rar') || mime.includes('7z')) return '压缩包'
  return '其他'
}

function typeLabel(asset: SystemAssetRow) {
  const bucket = typeBucket(asset)
  if (bucket === '图片' && asset.mime_type) return asset.mime_type.replace('image/', '').toUpperCase()
  return bucket
}

function toggleAsset(row: SystemAssetRow, checked: boolean, index = -1, range = false) {
  const next = new Set(selectedAssetIds.value)
  if (range && lastSelectedIndex >= 0 && index >= 0) {
    const start = Math.min(lastSelectedIndex, index)
    const end = Math.max(lastSelectedIndex, index)
    for (let i = start; i <= end; i += 1) {
      const asset = filteredRows.value[i]
      if (asset) next.add(asset.id)
    }
  } else if (checked) {
    next.add(row.id)
  } else {
    next.delete(row.id)
  }
  selectedAssetIds.value = next
  lastSelectedIndex = index
}

function toggleAllAssets(checked: boolean) {
  selectedAssetIds.value = checked ? new Set(downloadableAssetIds.value) : new Set()
  lastSelectedIndex = -1
}

function selectAsset(row: SystemAssetRow) {
  activeAsset.value = row
}

function gridRowAsAsset(row: Record<string, unknown>): MaterialGridRow {
  return row as unknown as MaterialGridRow
}

function canPreviewMaterial(asset: SystemAssetRow) {
  return canAttemptSystemAssetPreview(asset)
}

async function ensurePreview(asset: SystemAssetRow, silent = false, signal?: AbortSignal) {
  const directUrl = resolvedSystemAssetPreviewUrl(asset)
  if (directUrl) {
    previewUrls.value = { ...previewUrls.value, [asset.id]: directUrl }
    return {
      asset_id: asset.id,
      status: 'ready',
      preparing: false,
      preview_url: directUrl,
      download_url: 'download_url' in asset ? asset.download_url : undefined,
      preview_available: true,
    } as SystemAssetPreviewMeta
  }
  if (!canPreviewMaterial(asset) && !silent) {
    notice.value = '这个素材当前只能下载，不能在线预览'
    return null
  }
  if (previewUrls.value[asset.id]) {
    return {
      asset_id: asset.id,
      status: 'ready',
      preparing: false,
      preview_url: previewUrls.value[asset.id],
      preview_available: true,
    } as SystemAssetPreviewMeta
  }
  if (previewLoadingIds.value.has(asset.id)) return null
  const next = new Set(previewLoadingIds.value)
  next.add(asset.id)
  previewLoadingIds.value = next
  try {
    const meta = await assetWorkbenchApi.previewSystemAsset(asset.id, signal)
    const previewUrl = resolvedSystemAssetPreviewUrl(meta)
    if (previewUrl && isSystemAssetImagePreviewable(meta)) {
      previewUrls.value = { ...previewUrls.value, [asset.id]: previewUrl }
    }
    return {
      ...meta,
      preview_url: previewUrl || meta.preview_url,
      preview_available: meta.preview_available || Boolean(previewUrl),
    }
  } catch (err) {
    if (signal?.aborted) return null
    if (!silent) error.value = err instanceof Error ? err.message : '素材预览加载失败'
    return null
  } finally {
    const done = new Set(previewLoadingIds.value)
    done.delete(asset.id)
    previewLoadingIds.value = done
  }
}

async function preloadVisiblePreviews(assets: SystemAssetRow[]) {
  const candidates = assets
    .filter((asset) => isSystemAssetImagePreviewable(asset) && canPreviewMaterial(asset) && !previewUrls.value[asset.id])
    .slice(0, 8)
  await Promise.allSettled(candidates.map((asset) => ensurePreview(asset, true)))
}

async function ensureClientMaterialPreview(material: ClientMaterialRow) {
  if (!canPreviewClientMaterial(material) || clientPreviewUrls.value[material.id] || clientPreviewLoadingIds.value.has(material.id)) return
  const next = new Set(clientPreviewLoadingIds.value)
  next.add(material.id)
  clientPreviewLoadingIds.value = next
  try {
    const meta = await assetWorkbenchApi.previewClientMaterial(material.id)
    const previewUrl = resolvedSystemAssetPreviewUrl(meta)
    if (previewUrl && isSystemAssetImagePreviewable(meta)) {
      clientPreviewUrls.value = { ...clientPreviewUrls.value, [material.id]: previewUrl }
    }
  } catch {
    // 缩略图失败不阻断下载列表。
  } finally {
    const done = new Set(clientPreviewLoadingIds.value)
    done.delete(material.id)
    clientPreviewLoadingIds.value = done
  }
}

async function preloadClientMaterialPreviews(materials: ClientMaterialRow[]) {
  const candidates = materials
    .filter((material) => canPreviewClientMaterial(material) && !clientPreviewUrls.value[material.id])
    .slice(0, 12)
  await Promise.allSettled(candidates.map((material) => ensureClientMaterialPreview(material)))
}

async function openAssetPreview(asset: SystemAssetRow) {
  activeAsset.value = asset
  previewController?.abort()
  const controller = new AbortController()
  previewController = controller
  previewLoading.value = true
  error.value = ''
  notice.value = ''
  try {
    const meta = await ensurePreview(asset, false, controller.signal)
    if (controller.signal.aborted) return
    previewMeta.value = meta
    if (meta && resolvedSystemAssetPreviewUrl(meta)) {
      previewAsset.value = asset
    } else if (canPreviewMaterial(asset)) {
      notice.value = '这个素材暂时没有可展示的预览图'
    }
  } finally {
    if (previewController === controller) previewLoading.value = false
  }
}

function closePreview() {
  previewAsset.value = null
  previewMeta.value = null
}

async function downloadAsset(row: SystemAssetRow) {
  notice.value = ''
  error.value = ''
  try {
    const info = await assetWorkbenchApi.downloadSystemAsset(row.id)
    if (!info.download_url) {
      throw new Error('当前素材没有可用下载链接')
    }
    window.open(info.download_url, '_blank', 'noopener,noreferrer')
    notice.value = `已生成下载链接：${info.filename || titleOf(row)}`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '素材下载失败'
  }
}

async function downloadSelectedAssets() {
  const ids = Array.from(selectedAssetIds.value)
  if (!ids.length) {
    notice.value = '请选择要下载的素材'
    return
  }
  notice.value = '正在生成素材下载包'
  error.value = ''
  try {
    const manifest = await assetWorkbenchApi.batchDownloadSystemAssets(ids)
    const result = await downloadBatchAsZip({
      items: manifest.items.map((item) => ({
        key: String(item.asset_id),
        filename: item.filename,
        downloadURL: item.download_url,
        fallbackName: `system-asset-${item.asset_id}`,
      })),
      serverFailures: (manifest.failures ?? []).map((failure) => `asset_id=${failure.asset_id} reason=${failure.reason}`),
      zipFilename: buildTimestampedZipFilename('asset-workbench-system-assets'),
      onStatus: (message) => {
        notice.value = message
      },
    })
    notice.value = `已打包 ${result.writtenCount} 个素材，失败 ${result.failureCount} 个`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '素材批量下载失败'
  }
}

async function loadClientMaterials(admin = false) {
  clientMaterialsLoading.value = true
  error.value = ''
  try {
    const rows = await assetWorkbenchApi.listClientMaterials(admin)
    if (admin) {
      adminClientMaterials.value = rows
    } else {
      clientMaterials.value = rows
    }
    clientPreviewUrls.value = {}
    void preloadClientMaterialPreviews(rows)
    selectedClientMaterialIds.value = new Set()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '客户端素材加载失败'
  } finally {
    clientMaterialsLoading.value = false
  }
}

async function loadUploadDirectoriesAdmin() {
  try {
    uploadDirectories.value = await assetWorkbenchApi.listUploadDirectoriesAdmin()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '上传目录加载失败'
  }
}

function toggleClientMaterial(row: ClientMaterialRow, checked: boolean) {
  const next = new Set(selectedClientMaterialIds.value)
  if (checked) next.add(row.id)
  else next.delete(row.id)
  selectedClientMaterialIds.value = next
}

function toggleAllClientMaterials(checked: boolean) {
  selectedClientMaterialIds.value = checked ? new Set(filteredClientMaterialIds.value) : new Set()
}

async function downloadClientMaterial(row: ClientMaterialRow) {
  notice.value = ''
  error.value = ''
  try {
    const info = await assetWorkbenchApi.downloadClientMaterial(row.id)
    if (!info.download_url) throw new Error('当前素材没有可用下载链接')
    window.open(info.download_url, '_blank', 'noopener,noreferrer')
    notice.value = `已生成下载链接：${info.filename || row.title}`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '素材下载失败'
  }
}

async function downloadSelectedClientMaterials() {
  const ids = Array.from(selectedClientMaterialIds.value)
  if (!ids.length) {
    notice.value = '请选择要下载的素材'
    return
  }
  notice.value = '正在生成素材下载包'
  error.value = ''
  try {
    const manifest = await assetWorkbenchApi.batchDownloadClientMaterials(ids)
    const result = await downloadBatchAsZip({
      items: manifest.items.map((item) => ({
        key: String(item.asset_id),
        filename: item.filename,
        downloadURL: item.download_url,
        fallbackName: `client-material-${item.asset_id}`,
      })),
      serverFailures: (manifest.failures ?? []).map((failure) => `asset_id=${failure.asset_id} reason=${failure.reason}`),
      zipFilename: buildTimestampedZipFilename('asset-workbench-client-materials'),
      onStatus: (message) => {
        notice.value = message
      },
    })
    notice.value = `已打包 ${result.writtenCount} 个素材，失败 ${result.failureCount} 个`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '素材批量下载失败'
  }
}

async function publishClientMaterial(asset?: SystemAssetRow) {
  const assetId = asset?.id || clientMaterialAssetId.value || 0
  if (!assetId) {
    error.value = '请输入要发布的素材 ID'
    return
  }
  error.value = ''
  try {
    await assetWorkbenchApi.createClientMaterial({
      asset_id: assetId,
      title: clientMaterialTitle.value || (asset ? titleOf(asset) : undefined),
      description: clientMaterialDescription.value,
      enabled: true,
      sort_order: adminClientMaterials.value.length + 1,
    })
    clientMaterialAssetId.value = null
    clientMaterialTitle.value = ''
    clientMaterialDescription.value = ''
    notice.value = '已发布给客户端'
    await loadClientMaterials(true)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '发布客户端素材失败'
  }
}

async function toggleClientMaterialEnabled(row: ClientMaterialRow) {
  try {
    await assetWorkbenchApi.updateClientMaterial(row.id, { enabled: !row.enabled })
    await loadClientMaterials(true)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '更新客户端素材失败'
  }
}

async function removeClientMaterial(row: ClientMaterialRow) {
  try {
    await assetWorkbenchApi.deleteClientMaterial(row.id)
    notice.value = '已下架客户端素材'
    await loadClientMaterials(true)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '下架客户端素材失败'
  }
}

async function createUploadDirectory() {
  if (!directoryName.value.trim() || !directoryPrefix.value.trim()) {
    error.value = '目录名称和 OSS 前缀必填'
    return
  }
  error.value = ''
  try {
    await assetWorkbenchApi.createUploadDirectory({
      name: directoryName.value,
      oss_prefix: directoryPrefix.value,
      description: directoryDescription.value,
      difficulty_class: directoryDifficulty.value,
      enabled: true,
      sort_order: uploadDirectories.value.length + 1,
    })
    directoryName.value = ''
    directoryPrefix.value = ''
    directoryDescription.value = ''
    directoryDifficulty.value = 'A'
    notice.value = '上传目录已创建'
    await loadUploadDirectoriesAdmin()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '创建上传目录失败'
  }
}

async function toggleUploadDirectory(row: UploadDirectoryRow) {
  try {
    await assetWorkbenchApi.updateUploadDirectory(row.id, { enabled: !row.enabled })
    await loadUploadDirectoriesAdmin()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '更新上传目录失败'
  }
}

onBeforeUnmount(() => {
  previewController?.abort()
})

function initializeMaterialsMode(isAdmin: boolean | undefined) {
  if (isAdmin === undefined) return
  const mode = isAdmin ? 'admin' : 'client'
  if (initializedMode === mode) return
  initializedMode = mode
  if (mode === 'client') {
    void loadClientMaterials(false)
  } else {
    void searchMaterials(false)
    void loadClientMaterials(true)
    void loadUploadDirectoriesAdmin()
  }
}

watch(() => bootstrap.value?.is_admin, initializeMaterialsMode, { immediate: true })

watch(
  filteredClientMaterials,
  (materials) => {
    void preloadClientMaterialPreviews(materials)
  },
  { immediate: false },
)
</script>

<template>
  <section class="aw-page-stack">
    <template v-if="isSimpleUser">
      <div class="aw-page-bar">
        <div class="aw-page-bar__copy">
          <p class="aw-eyebrow">素材下载</p>
          <h2>可下载素材</h2>
          <p>这里展示管理端发布给你的素材，支持单个下载和批量打包。</p>
        </div>
        <div class="aw-page-bar__actions">
          <button class="aw-primary-button" type="button" :disabled="!selectedClientMaterialIds.size" @click="downloadSelectedClientMaterials">
            <Download :size="16" aria-hidden="true" />
            批量下载
          </button>
        </div>
      </div>

      <div class="aw-data-surface">
        <div class="aw-material-search aw-material-search--client">
          <input v-model="clientMaterialKeyword" type="search" placeholder="搜索 SKU、素材名称或文件名" />
          <span class="aw-chip aw-chip--neutral">{{ formatInt(filteredClientMaterials.length) }} / {{ formatInt(clientMaterials.length) }} 个素材</span>
        </div>
        <div class="aw-material-client-toolbar">
          <span>{{ formatInt(filteredClientMaterials.length) }} 个可见素材</span>
          <span>{{ selectedClientLabel }}</span>
          <label class="aw-inline-check">
            <input
              type="checkbox"
              :checked="allFilteredClientMaterialsSelected"
              @change="toggleAllClientMaterials(($event.target as HTMLInputElement).checked)"
            />
            <span>全选当前结果</span>
          </label>
        </div>
        <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
        <p v-if="error" class="aw-inline-alert">{{ error }}</p>
        <AsyncBoundary :loading="clientMaterialsLoading" :error="error" loading-label="正在加载素材" @retry="loadClientMaterials(false)">
          <div v-if="filteredClientMaterials.length" class="aw-material-client-list">
            <article v-for="material in filteredClientMaterials" :key="material.id" class="aw-material-client-item">
              <label class="aw-inline-check aw-material-client-item__check">
                <input
                  type="checkbox"
                  :aria-label="`${selectedClientMaterialIds.has(material.id) ? '取消选择' : '选择'}素材 ${clientMaterialTitleOf(material)}`"
                  :checked="selectedClientMaterialIds.has(material.id)"
                  @change="toggleClientMaterial(material, ($event.target as HTMLInputElement).checked)"
                />
              </label>
              <div class="aw-material-client-thumb" :class="{ 'aw-material-client-thumb--empty': !clientMaterialPreviewUrl(material) }">
                <img v-if="clientMaterialPreviewUrl(material)" :src="clientMaterialPreviewUrl(material)" :alt="clientMaterialTitleOf(material)" loading="lazy" decoding="async" />
                <FileImage v-else :size="22" aria-hidden="true" />
              </div>
              <div class="aw-material-client-copy">
                <strong>{{ clientMaterialTitleOf(material) }}</strong>
                <span>SKU {{ clientMaterialSkuOf(material) || '未标注' }}</span>
                <span>{{ clientMaterialFilenameOf(material) }}</span>
                <small v-if="material.description">{{ material.description }}</small>
              </div>
              <span :class="chipClass(systemPreviewMeta(canPreviewClientMaterial(material)).tone)">
                {{ systemPreviewMeta(canPreviewClientMaterial(material)).label }}
              </span>
              <button class="aw-secondary-button" type="button" @click="downloadClientMaterial(material)">下载</button>
            </article>
          </div>
          <div v-else class="aw-empty-state">
            <h3>{{ clientMaterials.length ? '没有匹配素材' : '暂无可下载素材' }}</h3>
            <p>{{ clientMaterials.length ? '换一个 SKU、名称或文件名再试。' : '管理端发布素材后，会显示在这里。' }}</p>
          </div>
        </AsyncBoundary>
      </div>
    </template>

    <template v-else>
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">只读素材</p>
        <h2>模板素材库</h2>
        <p>用缩略图墙快速查阅素材库文件，进入视区后按需加载预览，明细模式用于批量核对。</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-primary-button" type="button" @click="downloadSelectedAssets">
          <Download :size="16" aria-hidden="true" />
          批量下载
        </button>
      </div>
    </div>

    <div class="aw-data-surface">
      <div class="aw-material-search">
        <input v-model="keyword" type="search" placeholder="搜索 SKU、产品名或文件名" @keydown.enter="searchMaterials()" />
        <button class="aw-primary-button" type="button" @click="searchMaterials()">
          <Search :size="16" aria-hidden="true" />
          搜索
        </button>
        <div class="aw-view-toggle" aria-label="素材展示模式">
          <button type="button" :aria-pressed="viewMode === 'gallery'" @click="viewMode = 'gallery'">
            <Grid3X3 :size="15" aria-hidden="true" />
            浏览
          </button>
          <button type="button" :aria-pressed="viewMode === 'table'" @click="viewMode = 'table'">
            <List :size="15" aria-hidden="true" />
            明细
          </button>
        </div>
      </div>
      <div class="aw-material-search__filters">
        <select v-model="typeFilter" aria-label="文件类型筛选">
          <option value="all">全部类型</option>
          <option v-for="type in fileTypeOptions" :key="type" :value="type">{{ type }}</option>
        </select>
        <select v-model="previewFilter" aria-label="预览状态筛选">
          <option value="all">全部预览状态</option>
          <option value="previewable">可预览</option>
          <option value="download_only">只下载</option>
        </select>
        <span class="aw-chip aw-chip--neutral">{{ formatInt(filteredRows.length) }} / {{ formatInt(total) }} 条</span>
        <div class="aw-inline-actions" aria-label="素材分页">
          <button class="aw-secondary-button" type="button" :disabled="page <= 1 || loading" @click="goMaterialsPage(page - 1)">
            <ChevronLeft :size="15" aria-hidden="true" />
            上一页
          </button>
          <span class="aw-chip aw-chip--neutral">第 {{ formatInt(page) }} / {{ formatInt(totalPages) }} 页</span>
          <button class="aw-secondary-button" type="button" :disabled="page >= totalPages || loading" @click="goMaterialsPage(page + 1)">
            下一页
            <ChevronRight :size="15" aria-hidden="true" />
          </button>
        </div>
        <label class="aw-inline-check">
          <input
            type="checkbox"
            :checked="selectedAssetIds.size > 0 && selectedAssetIds.size === downloadableAssetIds.length"
            @change="toggleAllAssets(($event.target as HTMLInputElement).checked)"
          />
          <span>全选当前结果</span>
        </label>
      </div>

      <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
      <p v-if="error" class="aw-inline-alert">{{ error }}</p>
    </div>

    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <span>客户端素材</span>
        <span>{{ formatInt(adminClientMaterials.length) }} 个已发布</span>
        <button type="button" :disabled="!activeAsset" @click="activeAsset && publishClientMaterial(activeAsset)">发布当前素材</button>
      </div>
      <div class="aw-material-admin-form">
        <input v-model.number="clientMaterialAssetId" type="number" min="1" placeholder="素材 ID" aria-label="素材 ID" />
        <input v-model="clientMaterialTitle" type="text" placeholder="展示名称" aria-label="展示名称" />
        <input v-model="clientMaterialDescription" type="text" placeholder="说明" aria-label="说明" />
        <button class="aw-secondary-button" type="button" @click="publishClientMaterial()">按 ID 发布</button>
      </div>
      <div v-if="adminClientMaterials.length" class="aw-material-admin-list">
        <article v-for="material in adminClientMaterials" :key="material.id" class="aw-material-admin-item aw-material-admin-item--client">
          <div class="aw-material-client-thumb aw-material-admin-thumb" :class="{ 'aw-material-client-thumb--empty': !clientMaterialPreviewUrl(material) }">
            <img v-if="clientMaterialPreviewUrl(material)" :src="clientMaterialPreviewUrl(material)" :alt="clientMaterialTitleOf(material)" loading="lazy" decoding="async" />
            <FileImage v-else :size="28" aria-hidden="true" />
          </div>
          <div class="aw-material-admin-copy">
            <strong>{{ clientMaterialTitleOf(material) }}</strong>
            <small>SKU {{ clientMaterialSkuOf(material) || '未标注' }}</small>
            <small v-if="material.description">{{ material.description }}</small>
          </div>
          <span class="aw-material-admin-file">{{ material.filename_snapshot || `asset_id=${material.asset_id}` }}</span>
          <span :class="chipClass(systemPreviewMeta(canPreviewClientMaterial(material)).tone)">
            {{ systemPreviewMeta(canPreviewClientMaterial(material)).label }}
          </span>
          <span class="aw-chip" :class="material.enabled ? 'aw-chip--success' : 'aw-chip--neutral'">{{ material.enabled ? '已发布' : '已停用' }}</span>
          <button type="button" @click="toggleClientMaterialEnabled(material)">{{ material.enabled ? '停用' : '启用' }}</button>
          <button type="button" @click="removeClientMaterial(material)">下架</button>
        </article>
      </div>
      <p v-else class="aw-copy">还没有发布给客户端的素材。</p>
    </div>

    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <span>上传目录</span>
        <span>{{ formatInt(uploadDirectories.length) }} 个目录</span>
      </div>
      <div class="aw-material-admin-form">
        <input v-model="directoryName" type="text" placeholder="目录名称" aria-label="目录名称" />
        <input v-model="directoryPrefix" type="text" placeholder="OSS 前缀，如 studio-a" aria-label="OSS 前缀" />
        <select v-model="directoryDifficulty" aria-label="计价分类">
          <option v-for="option in directoryDifficultyOptions" :key="option" :value="option">{{ option }}</option>
        </select>
        <input v-model="directoryDescription" type="text" placeholder="说明" aria-label="说明" />
        <button class="aw-secondary-button" type="button" @click="createUploadDirectory">创建目录</button>
      </div>
      <div v-if="uploadDirectories.length" class="aw-material-admin-list">
        <article v-for="directory in uploadDirectories" :key="directory.id" class="aw-material-admin-item">
          <strong>{{ directory.name }}</strong>
          <span>{{ directory.oss_prefix }}</span>
          <span class="aw-chip aw-chip--info">计价 {{ directory.difficulty_class || 'A' }}</span>
          <span class="aw-chip" :class="directory.enabled ? 'aw-chip--success' : 'aw-chip--neutral'">{{ directory.enabled ? '已启用' : '已停用' }}</span>
          <button type="button" @click="toggleUploadDirectory(directory)">{{ directory.enabled ? '停用' : '启用' }}</button>
        </article>
      </div>
      <p v-else class="aw-copy">没有上传目录时，客户端会继续使用默认目录。</p>
    </div>

    <div class="aw-material-browser" :class="{ 'aw-material-browser--detail': viewMode === 'table' }">
      <div class="aw-material-browser__main">
        <MaterialGallery
          v-if="viewMode === 'gallery'"
          :items="filteredRows"
          :selected-ids="selectedAssetIds"
          :active-id="activeAsset?.id"
          :preview-urls="previewUrls"
          :loading="loading"
          @select="selectAsset"
          @toggle="toggleAsset"
          @preview="openAssetPreview"
          @download="downloadAsset"
          @visible="preloadVisiblePreviews"
        />
        <div v-else class="aw-data-surface">
          <AsyncBoundary
            :loading="loading"
            :error="error"
            loading-label="正在搜索素材库"
            @retry="searchMaterials"
          >
            <WorkbenchDataGrid
              v-if="filteredRows.length"
              :columns="materialGridColumns"
              :rows="materialGridRows"
              row-key="id"
              storage-key="materials"
              group-by="display_type"
              :height="520"
              :row-height="36"
            >
              <template #cell="{ row, column, value }">
                <label v-if="column.key === 'select'" class="aw-inline-check">
                  <input
                    type="checkbox"
                    :checked="selectedAssetIds.has(gridRowAsAsset(row).id)"
                    @change="toggleAsset(gridRowAsAsset(row), ($event.target as HTMLInputElement).checked)"
                  />
                  <span>{{ gridRowAsAsset(row).id }}</span>
                </label>
                <div v-else-if="column.key === 'actions'" class="aw-inline-actions">
                  <button type="button" :disabled="!canPreviewMaterial(gridRowAsAsset(row))" @click="openAssetPreview(gridRowAsAsset(row))">
                    预览
                  </button>
                  <button type="button" @click="downloadAsset(gridRowAsAsset(row))">下载</button>
                </div>
                <span
                  v-else-if="column.key === 'preview_label'"
                  :class="chipClass(systemPreviewMeta(canPreviewMaterial(gridRowAsAsset(row))).tone)"
                >
                  {{ value }}
                </span>
                <span v-else class="aw-cell-text">{{ value }}</span>
              </template>
            </WorkbenchDataGrid>
            <div v-else class="aw-empty-state">
              <h3>还没有搜索结果</h3>
              <p>输入 SKU、产品名或文件名后搜索。只有已开通素材库的账号可以查看和下载。</p>
            </div>
          </AsyncBoundary>
        </div>
        <div v-if="selectedAssetIds.size" class="aw-material-action-bar">
          <span>{{ selectedLabel }}</span>
          <div class="aw-inline-actions">
            <button class="aw-secondary-button" type="button" @click="selectedAssetIds = new Set()">清空选择</button>
            <button class="aw-primary-button" type="button" @click="downloadSelectedAssets">下载所选素材</button>
          </div>
        </div>
      </div>

      <aside class="aw-material-browser__side">
        <section v-if="activeAsset" class="aw-panel aw-material-detail">
          <div class="aw-material-detail__hero">
            <p class="aw-eyebrow">当前素材</p>
            <h3>{{ titleOf(activeAsset) }}</h3>
            <span>SKU {{ styleCodeOf(activeAsset) || '未标注' }}</span>
          </div>
          <dl class="aw-material-detail__list">
            <div v-for="[label, value] in activeDetailRows" :key="label">
              <dt>{{ label }}</dt>
              <dd>{{ value }}</dd>
            </div>
          </dl>
          <div class="aw-inline-actions">
            <button class="aw-primary-button" type="button" :disabled="!canPreviewMaterial(activeAsset) || previewLoading" @click="openAssetPreview(activeAsset)">
              {{ previewLoading ? '加载中' : '预览素材' }}
            </button>
            <button class="aw-secondary-button" type="button" @click="downloadAsset(activeAsset)">下载</button>
          </div>
        </section>
        <section class="aw-panel">
          <div class="aw-panel__head">
            <div>
              <p class="aw-eyebrow">相关素材</p>
              <h3>同类参考</h3>
            </div>
            <span class="aw-chip aw-chip--neutral">{{ relatedAssets.length }} 个</span>
          </div>
          <div v-if="relatedAssets.length" class="aw-material-related">
            <button
              v-for="asset in relatedAssets"
              :key="asset.id"
              class="aw-secondary-button"
              type="button"
              @click="selectAsset(asset)"
            >
              {{ styleCodeOf(asset) || titleOf(asset) }}
            </button>
          </div>
          <p v-else class="aw-copy">选择一个素材后，这里会按产品名、任务号和文件类型提示相关素材。</p>
        </section>
      </aside>
    </div>

    <section v-if="previewAsset" class="aw-material-preview" role="dialog" aria-modal="true" aria-label="素材预览">
      <div class="aw-material-preview__stage" @click.self="closePreview">
        <AssetPreviewMedia
          class="aw-material-preview__media"
          :resolved-preview-url="resolvedSystemAssetPreviewUrl(previewMeta)"
          :fallback-src="previewMeta?.download_url"
          :alt="titleOf(previewAsset)"
        />
      </div>
      <aside class="aw-material-preview__side">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">暗场预览</p>
            <h3>{{ titleOf(previewAsset) }}</h3>
          </div>
          <button class="aw-secondary-button" type="button" @click="closePreview">
            <X :size="16" aria-hidden="true" />
            关闭
          </button>
        </div>
        <dl class="aw-material-detail__list">
          <div v-for="[label, value] in activeDetailRows" :key="`preview-${label}`">
            <dt>{{ label }}</dt>
            <dd>{{ value }}</dd>
          </div>
        </dl>
        <button class="aw-primary-button" type="button" @click="downloadAsset(previewAsset)">下载这个素材</button>
      </aside>
    </section>
    </template>
  </section>
</template>
