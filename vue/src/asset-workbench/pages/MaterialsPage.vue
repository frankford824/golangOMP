<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { ChevronLeft, ChevronRight, Download, FileImage, Grid3X3, List, Pencil, Save, Search, X } from 'lucide-vue-next'

import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import {
  assetWorkbenchApi,
  type ClientMaterialRow,
  type DifficultyClassRow,
  type SystemAssetPreviewMeta,
  type SystemAssetRow,
  type UploadDirectoryRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { buildTimestampedZipFilename, downloadBatchAsZip } from '@/utils/batchZipDownload'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { difficultyCodes, firstDifficultyCode } from '@aw/shared/format/difficulty'
import { formatInt } from '@aw/shared/format/number'
import { chipClass, systemPreviewMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import MaterialGallery from '@aw/shared/materials/MaterialGallery.vue'
import WorkbenchPreviewDialog from '@aw/shared/preview/WorkbenchPreviewDialog.vue'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'
import {
  canAttemptSystemAssetPreview,
  isSystemAssetImagePreviewable,
  materialAssetKey,
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
  display_resource_key: string
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
const selectedAssetIds = ref<Set<string>>(new Set())
const activeAsset = ref<SystemAssetRow | null>(null)
const previewAsset = ref<SystemAssetRow | null>(null)
const previewMeta = ref<SystemAssetPreviewMeta | null>(null)
const clientPreviewMaterial = ref<ClientMaterialRow | null>(null)
const clientPreviewMeta = ref<SystemAssetPreviewMeta | null>(null)
const previewUrls = ref<Record<string, string>>({})
const previewLoadingIds = ref<Set<string>>(new Set())
const viewMode = ref<ViewMode>('gallery')
const typeFilter = ref('all')
const previewFilter = ref('all')
const previewLoading = ref(false)
const previewError = ref('')
const searchError = ref('')
const pageError = ref('')
const clientMaterialsError = ref('')
const notice = ref('')
const { bootstrap } = useAssetWorkbenchBootstrap()
const isSimpleUser = computed(() => bootstrap.value?.is_admin === false)
const clientMaterials = ref<ClientMaterialRow[]>([])
const adminClientMaterials = ref<ClientMaterialRow[]>([])
const uploadDirectories = ref<UploadDirectoryRow[]>([])
const difficultyRows = ref<DifficultyClassRow[]>([])
const selectedClientMaterialIds = ref<Set<number>>(new Set())
const clientMaterialsLoading = ref(false)
const clientMaterialKeyword = ref('')
const clientPreviewUrls = ref<Record<number, string>>({})
const clientPreviewLoadingIds = ref<Set<number>>(new Set())
const clientMaterialAssetId = ref('')
const clientMaterialTitle = ref('')
const clientMaterialDescription = ref('')
const directoryName = ref('')
const directoryPrefix = ref('')
const directoryDescription = ref('')
const directoryDifficulty = ref('A')
const editingDirectoryId = ref(0)
const directoryEditForm = ref({
  name: '',
  oss_prefix: '',
  description: '',
  difficulty_class: '',
  enabled: true,
  sort_order: 0,
})
const directoryDifficultyOptions = computed(() => difficultyCodes(difficultyRows.value))
const materialsRequest = usePageRequest(
  (signal) => assetWorkbenchApi.systemSearch({ q: keyword.value, source: 'all', page: page.value, page_size: pageSize.value }, signal),
  { items: [], total: 0, page: 1, size: 0 },
  '素材库搜索失败',
)
const loading = materialsRequest.loading
const error = materialsRequest.error
let previewController: AbortController | null = null
const previewInflight = new Map<string, Promise<SystemAssetPreviewMeta | null>>()
let lastSelectedIndex = -1
let initializedMode: 'admin' | 'client' | '' = ''

const downloadableAssetIds = computed(() => filteredRows.value.map((row) => materialAssetKey(row)).filter(Boolean))
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
    display_resource_key: materialAssetKey(row),
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
const allFilteredAssetsSelected = computed(() => {
  const ids = downloadableAssetIds.value
  return ids.length > 0 && ids.every((id) => selectedAssetIds.value.has(id))
})
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
    ['来源', asset.source_label || (asset.source_type === 'external' ? '外部资源' : '系统资源')],
    ['SKU', styleCodeOf(asset) || '—'],
    ['名称', titleOf(asset)],
    ['文件类型', typeLabel(asset)],
    ['任务号', asset.task_no || (asset.source_type === 'external' ? '外部资源' : '—')],
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
const previewDialogOpen = computed(() => Boolean(previewAsset.value || clientPreviewMaterial.value))
const previewDialogMimeType = computed(() => {
  if (previewAsset.value) return previewAsset.value.mime_type || ''
  if (clientPreviewMaterial.value) return clientPreviewMaterial.value.mime_type_snapshot || ''
  return ''
})
const previewDialogFilename = computed(() => {
  if (previewAsset.value) {
    return previewAsset.value.original_filename || previewAsset.value.file_name || ''
  }
  if (clientPreviewMaterial.value) return clientPreviewMaterial.value.filename_snapshot || ''
  return ''
})
const previewDialogTitle = computed(() => {
  if (previewAsset.value) return titleOf(previewAsset.value)
  if (clientPreviewMaterial.value) return clientMaterialTitleOf(clientPreviewMaterial.value)
  return '素材预览'
})
const previewDialogUrl = computed(() => {
  if (previewAsset.value) return resolvedSystemAssetPreviewUrl(previewMeta.value)
  return resolvedSystemAssetPreviewUrl(clientPreviewMeta.value)
})
const previewDialogFallback = computed(() => {
  if (previewAsset.value) return previewMeta.value?.download_url || ''
  return clientPreviewMeta.value?.download_url || ''
})
const previewDialogRows = computed<Array<[string, string]>>(() => {
  if (previewAsset.value) {
    return activeDetailRows.value.map(([label, value]) => [label, String(value || '—')])
  }
  const material = clientPreviewMaterial.value
  if (!material) return []
  return [
    ['SKU', clientMaterialSkuOf(material) || '未标注'],
    ['名称', clientMaterialTitleOf(material)],
    ['文件名', clientMaterialFilenameOf(material)],
    ['类型', material.mime_type_snapshot || '文件'],
    ['说明', material.description || '—'],
  ]
})
const previewDialogEmptyLabel = computed(() => {
  if (previewLoading.value) return '正在加载预览'
  if (previewError.value) return previewError.value
  return '暂无可展示预览'
})

async function searchMaterials(resetPage = true) {
  if (resetPage) page.value = 1
  notice.value = ''
  const result = await materialsRequest.run()
  searchError.value = error.value
  if (!result) return
  searchError.value = ''
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
  if (asset.source_type === 'external') return asset.resource_id || `ext-${asset.id}`
  return asset.scope_sku_code || asset.sku_code || asset.primary_sku_code || ''
}

function clientMaterialTitleOf(material: ClientMaterialRow) {
  return material.title || material.filename_snapshot || `素材 ${material.resource_id || material.asset_id}`
}

function clientMaterialSkuOf(material: ClientMaterialRow) {
  return material.scope_sku_code || material.sku_code || material.primary_sku_code || ''
}

function clientMaterialFilenameOf(material: ClientMaterialRow) {
  return material.filename_snapshot || material.resource_id || `asset_id=${material.asset_id}`
}

function clientMaterialPreviewProbe(material: ClientMaterialRow): SystemAssetPreviewMeta {
  return {
    asset_id: material.asset_id,
    source_type: material.source_type,
    source_ref: material.source_ref || material.resource_id,
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
    material.source_type,
    material.source_ref,
    material.resource_id,
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
      if (!asset) continue
      const key = materialAssetKey(asset)
      if (checked) next.add(key)
      else next.delete(key)
    }
  } else if (checked) {
    next.add(materialAssetKey(row))
  } else {
    next.delete(materialAssetKey(row))
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
  const key = materialAssetKey(asset)
  const inlinePreviewUrl = String(('preview_url' in asset && asset.preview_url) || '').trim()
  if (inlinePreviewUrl) {
    previewUrls.value = { ...previewUrls.value, [key]: inlinePreviewUrl }
    return {
      asset_id: asset.id,
      status: 'ready',
      preparing: false,
      preview_url: inlinePreviewUrl,
      download_url: 'download_url' in asset ? asset.download_url : undefined,
      preview_available: true,
    } as SystemAssetPreviewMeta
  }
  if (!canPreviewMaterial(asset) && !silent) {
    notice.value = '这个素材当前只能下载，不能在线预览'
    return null
  }
  if (previewUrls.value[key]) {
    return {
      asset_id: asset.id,
      source_type: asset.source_type,
      source_ref: asset.resource_id,
      status: 'ready',
      preparing: false,
      preview_url: previewUrls.value[key],
      preview_available: true,
    } as SystemAssetPreviewMeta
  }

  const inflight = previewInflight.get(key)
  if (inflight) return inflight

  let task!: Promise<SystemAssetPreviewMeta | null>
  task = (async (): Promise<SystemAssetPreviewMeta | null> => {
    const next = new Set(previewLoadingIds.value)
    next.add(key)
    previewLoadingIds.value = next
    try {
      const meta = await assetWorkbenchApi.previewMaterialAsset(asset, signal)
      const previewUrl = resolvedSystemAssetPreviewUrl(meta)
      if (previewUrl && isSystemAssetImagePreviewable(meta)) {
        previewUrls.value = { ...previewUrls.value, [key]: previewUrl }
      }
      return {
        ...meta,
        preview_url: previewUrl || meta.preview_url,
        preview_available: meta.preview_available || Boolean(previewUrl),
      }
    } catch (err) {
      if (signal?.aborted) return null
      if (!silent) {
        previewError.value = err instanceof Error ? err.message : '素材预览加载失败'
      }
      return null
    } finally {
      const done = new Set(previewLoadingIds.value)
      done.delete(key)
      previewLoadingIds.value = done
      if (previewInflight.get(key) === task) {
        previewInflight.delete(key)
      }
    }
  })()

  previewInflight.set(key, task)
  return task
}

async function preloadVisiblePreviews(assets: SystemAssetRow[]) {
  const candidates = assets
    .filter((asset) => isSystemAssetImagePreviewable(asset) && canPreviewMaterial(asset) && !previewUrls.value[materialAssetKey(asset)])
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

async function loadClientMaterialPreviewMeta(material: ClientMaterialRow) {
  if (!canPreviewClientMaterial(material)) return clientMaterialPreviewProbe(material)
  if (clientPreviewUrls.value[material.id]) {
    return {
      asset_id: material.asset_id,
      source_type: material.source_type,
      source_ref: material.source_ref || material.resource_id,
      status: 'ready',
      preparing: false,
      filename: material.filename_snapshot,
      mime_type: material.mime_type_snapshot,
      preview_url: clientPreviewUrls.value[material.id],
      preview_available: true,
    } as SystemAssetPreviewMeta
  }
  const meta = await assetWorkbenchApi.previewClientMaterial(material.id)
  const previewUrl = resolvedSystemAssetPreviewUrl(meta)
  if (previewUrl && isSystemAssetImagePreviewable(meta)) {
    clientPreviewUrls.value = { ...clientPreviewUrls.value, [material.id]: previewUrl }
  }
  return {
    ...meta,
    preview_url: previewUrl || meta.preview_url,
    preview_available: meta.preview_available || Boolean(previewUrl),
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
  clientPreviewMaterial.value = null
  clientPreviewMeta.value = null
  previewController?.abort()
  const controller = new AbortController()
  previewController = controller
  previewLoading.value = true
  previewError.value = ''
  notice.value = ''
  previewAsset.value = asset
  previewMeta.value = null
  try {
    const meta = await ensurePreview(asset, false, controller.signal)
    if (controller.signal.aborted) return
    previewMeta.value = meta
    if (!meta && canPreviewMaterial(asset)) {
      previewError.value = '素材预览加载失败'
    }
  } finally {
    if (previewController === controller) previewLoading.value = false
  }
}

async function openClientMaterialPreview(material: ClientMaterialRow) {
  previewController?.abort()
  previewAsset.value = null
  previewMeta.value = null
  clientPreviewMaterial.value = material
  clientPreviewMeta.value = clientMaterialPreviewProbe(material)
  previewLoading.value = true
  previewError.value = ''
  try {
    clientPreviewMeta.value = await loadClientMaterialPreviewMeta(material)
  } catch (err) {
    previewError.value = err instanceof Error ? err.message : '素材预览加载失败'
  } finally {
    previewLoading.value = false
  }
}

function closePreview() {
  previewAsset.value = null
  previewMeta.value = null
  clientPreviewMaterial.value = null
  clientPreviewMeta.value = null
  previewError.value = ''
}

function downloadPreviewAsset() {
  if (previewAsset.value) {
    void downloadAsset(previewAsset.value)
    return
  }
  if (clientPreviewMaterial.value) {
    void downloadClientMaterial(clientPreviewMaterial.value)
  }
}

async function downloadAsset(row: SystemAssetRow) {
  notice.value = ''
  pageError.value = ''
  try {
    const info = await assetWorkbenchApi.downloadMaterialAsset(row)
    if (!info.download_url) {
      throw new Error('当前素材没有可用下载链接')
    }
    window.open(info.download_url, '_blank', 'noopener,noreferrer')
    notice.value = `已生成下载链接：${info.filename || titleOf(row)}`
  } catch (err) {
    pageError.value = err instanceof Error ? err.message : '素材下载失败'
  }
}

async function downloadSelectedAssets() {
  const selected = rows.value.filter((row) => selectedAssetIds.value.has(materialAssetKey(row)))
  if (!selected.length) {
    notice.value = '请选择要下载的素材'
    return
  }
  notice.value = '正在生成素材下载包'
  pageError.value = ''
  try {
    const systemOnly = selected.every((row) => row.source_type !== 'external')
    const items: Array<{ key: string; filename: string; downloadURL: string; fallbackName: string }> = []
    const failures: string[] = []
    if (systemOnly) {
      const manifest = await assetWorkbenchApi.batchDownloadSystemAssets(selected.map((row) => row.id))
      items.push(
        ...manifest.items.map((item) => ({
          key: String(item.asset_id),
          filename: item.filename,
          downloadURL: item.download_url,
          fallbackName: `system-asset-${item.asset_id}`,
        })),
      )
      failures.push(...(manifest.failures ?? []).map((failure) => `asset_id=${failure.asset_id} reason=${failure.reason}`))
    } else {
      const settled = await Promise.allSettled(
        selected.map(async (asset) => {
          const info = await assetWorkbenchApi.downloadMaterialAsset(asset)
          if (!info.download_url) throw new Error('download_url_unavailable')
          return {
            key: materialAssetKey(asset),
            filename: info.filename || asset.original_filename || asset.file_name || materialAssetKey(asset),
            downloadURL: info.download_url,
            fallbackName: materialAssetKey(asset),
          }
        }),
      )
      settled.forEach((result, index) => {
        const asset = selected[index]
        if (result.status === 'fulfilled') items.push(result.value)
        else failures.push(`${materialAssetKey(asset)} reason=${result.reason instanceof Error ? result.reason.message : String(result.reason)}`)
      })
    }
    const result = await downloadBatchAsZip({
      items,
      serverFailures: failures,
      zipFilename: buildTimestampedZipFilename('asset-workbench-material-assets'),
      onStatus: (message) => {
        notice.value = message
      },
    })
    notice.value = `已打包 ${result.writtenCount} 个素材，失败 ${result.failureCount} 个`
  } catch (err) {
    pageError.value = err instanceof Error ? err.message : '素材批量下载失败'
  }
}

async function loadClientMaterials(admin = false) {
  clientMaterialsLoading.value = true
  clientMaterialsError.value = ''
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
    clientMaterialsError.value = err instanceof Error ? err.message : '客户端素材加载失败'
  } finally {
    clientMaterialsLoading.value = false
  }
}

async function loadUploadDirectoriesAdmin() {
  try {
    const [directories, difficulties] = await Promise.all([
      assetWorkbenchApi.listUploadDirectoriesAdmin(),
      assetWorkbenchApi.listDifficultyClasses(),
    ])
    uploadDirectories.value = directories
    difficultyRows.value = difficulties
    if (!directoryDifficultyOptions.value.includes(directoryDifficulty.value)) {
      directoryDifficulty.value = firstDifficultyCode(difficultyRows.value)
    }
    if (editingDirectoryId.value && !directories.some((directory) => directory.id === editingDirectoryId.value)) {
      cancelUploadDirectoryEdit()
    }
  } catch (err) {
    pageError.value = err instanceof Error ? err.message : '上传目录加载失败'
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
  pageError.value = ''
  try {
    const info = await assetWorkbenchApi.downloadClientMaterial(row.id)
    if (!info.download_url) throw new Error('当前素材没有可用下载链接')
    window.open(info.download_url, '_blank', 'noopener,noreferrer')
    notice.value = `已生成下载链接：${info.filename || row.title}`
  } catch (err) {
    pageError.value = err instanceof Error ? err.message : '素材下载失败'
  }
}

async function downloadSelectedClientMaterials() {
  const ids = Array.from(selectedClientMaterialIds.value)
  if (!ids.length) {
    notice.value = '请选择要下载的素材'
    return
  }
  notice.value = '正在生成素材下载包'
  pageError.value = ''
  try {
    const manifest = await assetWorkbenchApi.batchDownloadClientMaterials(ids)
    const result = await downloadBatchAsZip({
      items: manifest.items.map((item) => ({
        key: item.source_ref || String(item.asset_id),
        filename: item.filename,
        downloadURL: item.download_url,
        fallbackName: `client-material-${item.material_id || item.source_ref || item.asset_id}`,
      })),
      serverFailures: (manifest.failures ?? []).map((failure) => `material_id=${failure.material_id || '-'} source=${failure.source_ref || failure.asset_id} reason=${failure.reason}`),
      zipFilename: buildTimestampedZipFilename('asset-workbench-client-materials'),
      onStatus: (message) => {
        notice.value = message
      },
    })
    notice.value = `已打包 ${result.writtenCount} 个素材，失败 ${result.failureCount} 个`
  } catch (err) {
    pageError.value = err instanceof Error ? err.message : '素材批量下载失败'
  }
}

async function publishClientMaterial(asset?: SystemAssetRow) {
  const manualRef = clientMaterialAssetId.value.trim()
  const sourceType = asset?.source_type === 'external' || manualRef.startsWith('ext-') || manualRef.startsWith('external:') ? 'external' : 'system'
  const sourceRef = asset ? (asset.resource_id || (sourceType === 'external' ? `ext-${asset.id}` : String(asset.id))) : manualRef
  const assetId = asset?.id || (sourceType === 'system' ? Number.parseInt(manualRef, 10) : 0)
  if (!sourceRef || (sourceType === 'system' && (!Number.isFinite(assetId) || assetId <= 0))) {
    pageError.value = '请输入系统素材数字 ID 或 ext- 外部资源 ID'
    return
  }
  pageError.value = ''
  try {
    await assetWorkbenchApi.createClientMaterial({
      asset_id: asset?.id || (assetId > 0 ? assetId : undefined),
      source_type: sourceType,
      source_ref: sourceRef,
      resource_id: sourceRef,
      title: clientMaterialTitle.value || (asset ? titleOf(asset) : undefined),
      description: clientMaterialDescription.value,
      enabled: true,
      sort_order: adminClientMaterials.value.length + 1,
    })
    clientMaterialAssetId.value = ''
    clientMaterialTitle.value = ''
    clientMaterialDescription.value = ''
    notice.value = '已发布给客户端'
    await loadClientMaterials(true)
  } catch (err) {
    pageError.value = err instanceof Error ? err.message : '发布客户端素材失败'
  }
}

async function toggleClientMaterialEnabled(row: ClientMaterialRow) {
  try {
    await assetWorkbenchApi.updateClientMaterial(row.id, { enabled: !row.enabled })
    await loadClientMaterials(true)
  } catch (err) {
    pageError.value = err instanceof Error ? err.message : '更新客户端素材失败'
  }
}

async function removeClientMaterial(row: ClientMaterialRow) {
  try {
    await assetWorkbenchApi.deleteClientMaterial(row.id)
    notice.value = '已下架客户端素材'
    await loadClientMaterials(true)
  } catch (err) {
    pageError.value = err instanceof Error ? err.message : '下架客户端素材失败'
  }
}

async function createUploadDirectory() {
  if (!directoryName.value.trim() || !directoryPrefix.value.trim()) {
    pageError.value = '目录名称和 OSS 前缀必填'
    return
  }
  pageError.value = ''
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
    directoryDifficulty.value = firstDifficultyCode(difficultyRows.value)
    notice.value = '上传目录已创建'
    await loadUploadDirectoriesAdmin()
  } catch (err) {
    pageError.value = err instanceof Error ? err.message : '创建上传目录失败'
  }
}

async function toggleUploadDirectory(row: UploadDirectoryRow) {
  try {
    await assetWorkbenchApi.updateUploadDirectory(row.id, { enabled: !row.enabled })
    await loadUploadDirectoriesAdmin()
  } catch (err) {
    pageError.value = err instanceof Error ? err.message : '更新上传目录失败'
  }
}

function startUploadDirectoryEdit(row: UploadDirectoryRow) {
  editingDirectoryId.value = row.id
  directoryEditForm.value = {
    name: row.name,
    oss_prefix: row.oss_prefix,
    description: row.description,
    difficulty_class: row.difficulty_class || firstDifficultyCode(difficultyRows.value),
    enabled: row.enabled,
    sort_order: row.sort_order,
  }
}

function cancelUploadDirectoryEdit() {
  editingDirectoryId.value = 0
  directoryEditForm.value = {
    name: '',
    oss_prefix: '',
    description: '',
    difficulty_class: firstDifficultyCode(difficultyRows.value),
    enabled: true,
    sort_order: 0,
  }
}

async function saveUploadDirectory(row: UploadDirectoryRow) {
  const name = directoryEditForm.value.name.trim()
  const ossPrefix = directoryEditForm.value.oss_prefix.trim()
  if (!name || !ossPrefix) {
    pageError.value = '目录名称和 OSS 前缀必填'
    return
  }
  if (!directoryEditForm.value.difficulty_class) {
    pageError.value = '请选择计价分类'
    return
  }
  pageError.value = ''
  try {
    await assetWorkbenchApi.updateUploadDirectory(row.id, {
      name,
      oss_prefix: ossPrefix,
      description: directoryEditForm.value.description,
      difficulty_class: directoryEditForm.value.difficulty_class,
      enabled: directoryEditForm.value.enabled,
      sort_order: directoryEditForm.value.sort_order,
    })
    notice.value = '上传目录已更新'
    cancelUploadDirectoryEdit()
    await loadUploadDirectoriesAdmin()
  } catch (err) {
    pageError.value = err instanceof Error ? err.message : '更新上传目录失败'
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
        <p v-if="pageError" class="aw-inline-alert aw-inline-alert--error">{{ pageError }}</p>
        <AsyncBoundary :loading="clientMaterialsLoading" :error="clientMaterialsError" loading-label="正在加载素材" @retry="loadClientMaterials(false)">
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
              <button
                class="aw-material-client-thumb"
                :class="{ 'aw-material-client-thumb--empty': !clientMaterialPreviewUrl(material) }"
                type="button"
                :aria-label="`预览 ${clientMaterialTitleOf(material)}`"
                @click="openClientMaterialPreview(material)"
              >
                <img v-if="clientMaterialPreviewUrl(material)" :src="clientMaterialPreviewUrl(material)" :alt="clientMaterialTitleOf(material)" loading="lazy" decoding="async" />
                <FileImage v-else :size="22" aria-hidden="true" />
              </button>
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
        <h2>素材库</h2>
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
            :checked="allFilteredAssetsSelected"
            @change="toggleAllAssets(($event.target as HTMLInputElement).checked)"
          />
          <span>全选当前结果</span>
        </label>
      </div>

      <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
      <p v-if="pageError" class="aw-inline-alert aw-inline-alert--error">{{ pageError }}</p>
    </div>

    <div class="aw-material-browser" :class="{ 'aw-material-browser--detail': viewMode === 'table' }">
      <div class="aw-material-browser__main">
        <MaterialGallery
          v-if="viewMode === 'gallery'"
          :items="filteredRows"
          :selected-ids="selectedAssetIds"
          :active-id="activeAsset ? materialAssetKey(activeAsset) : null"
          :preview-urls="previewUrls"
          :preview-loading-ids="previewLoadingIds"
          :loading="loading"
          :error="searchError"
          @select="selectAsset"
          @toggle="toggleAsset"
          @preview="openAssetPreview"
          @download="downloadAsset"
          @visible="preloadVisiblePreviews"
          @retry="searchMaterials()"
        />
        <div v-else class="aw-data-surface">
          <AsyncBoundary
            :loading="loading"
            :error="searchError"
            loading-label="正在搜索素材库"
            @retry="searchMaterials"
          >
            <WorkbenchDataGrid
              v-if="filteredRows.length"
              :columns="materialGridColumns"
              :rows="materialGridRows"
              row-key="display_resource_key"
              storage-key="materials"
              group-by="display_type"
              :height="520"
              :row-height="36"
            >
              <template #cell="{ row, column, value }">
                <label v-if="column.key === 'select'" class="aw-inline-check">
                  <input
                    type="checkbox"
                    :checked="selectedAssetIds.has(materialAssetKey(gridRowAsAsset(row)))"
                    @change="toggleAsset(gridRowAsAsset(row), ($event.target as HTMLInputElement).checked)"
                  />
                  <span>{{ gridRowAsAsset(row).resource_id || gridRowAsAsset(row).id }}</span>
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

    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <span>客户端素材</span>
        <span>{{ formatInt(adminClientMaterials.length) }} 个已发布</span>
        <button type="button" :disabled="!activeAsset" @click="activeAsset && publishClientMaterial(activeAsset)">发布当前素材</button>
      </div>
      <div class="aw-material-admin-form">
        <input v-model="clientMaterialAssetId" type="text" placeholder="系统ID / ext-外部ID" aria-label="素材资源 ID" />
        <input v-model="clientMaterialTitle" type="text" placeholder="展示名称" aria-label="展示名称" />
        <input v-model="clientMaterialDescription" type="text" placeholder="说明" aria-label="说明" />
        <button class="aw-secondary-button" type="button" @click="publishClientMaterial()">按 ID 发布</button>
      </div>
      <div v-if="adminClientMaterials.length" class="aw-material-admin-list">
        <article v-for="material in adminClientMaterials" :key="material.id" class="aw-material-admin-item aw-material-admin-item--client">
          <button
            class="aw-material-client-thumb aw-material-admin-thumb"
            :class="{ 'aw-material-client-thumb--empty': !clientMaterialPreviewUrl(material) }"
            type="button"
            :aria-label="`预览 ${clientMaterialTitleOf(material)}`"
            @click="openClientMaterialPreview(material)"
          >
            <img v-if="clientMaterialPreviewUrl(material)" :src="clientMaterialPreviewUrl(material)" :alt="clientMaterialTitleOf(material)" loading="lazy" decoding="async" />
            <FileImage v-else :size="28" aria-hidden="true" />
          </button>
          <div class="aw-material-admin-copy">
            <strong>{{ clientMaterialTitleOf(material) }}</strong>
            <small>{{ material.source_label || (material.source_type === 'external' ? '外部资源' : '系统资源') }} · {{ clientMaterialSkuOf(material) || material.resource_id || '未标注' }}</small>
            <small v-if="material.description">{{ material.description }}</small>
          </div>
          <span class="aw-material-admin-file">{{ material.filename_snapshot || material.resource_id || `asset_id=${material.asset_id}` }}</span>
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
        <article
          v-for="directory in uploadDirectories"
          :key="directory.id"
          class="aw-material-admin-item"
          :class="{ 'aw-material-admin-item--editing': editingDirectoryId === directory.id }"
        >
          <template v-if="editingDirectoryId === directory.id">
            <input v-model="directoryEditForm.name" type="text" aria-label="编辑目录名称" />
            <input v-model="directoryEditForm.oss_prefix" type="text" aria-label="编辑 OSS 前缀" />
            <select v-model="directoryEditForm.difficulty_class" aria-label="编辑计价分类">
              <option v-for="option in directoryDifficultyOptions" :key="option" :value="option">{{ option }}</option>
            </select>
            <input v-model="directoryEditForm.description" type="text" aria-label="编辑说明" />
            <label class="aw-inline-check">
              <input v-model="directoryEditForm.enabled" type="checkbox" />
              启用
            </label>
            <button type="button" @click="saveUploadDirectory(directory)">
              <Save :size="16" aria-hidden="true" />
              保存
            </button>
            <button type="button" @click="cancelUploadDirectoryEdit">
              <X :size="16" aria-hidden="true" />
              取消
            </button>
          </template>
          <template v-else>
            <strong>{{ directory.name }}</strong>
            <span>{{ directory.oss_prefix }}</span>
            <span class="aw-chip aw-chip--info">计价 {{ directory.difficulty_class || '未设置' }}</span>
            <span class="aw-chip" :class="directory.enabled ? 'aw-chip--success' : 'aw-chip--neutral'">{{ directory.enabled ? '已启用' : '已停用' }}</span>
            <button type="button" @click="startUploadDirectoryEdit(directory)">
              <Pencil :size="16" aria-hidden="true" />
              编辑
            </button>
            <button type="button" @click="toggleUploadDirectory(directory)">{{ directory.enabled ? '停用' : '启用' }}</button>
          </template>
        </article>
      </div>
      <p v-else class="aw-copy">没有上传目录时，客户端会继续使用默认目录。</p>
    </div>

    <WorkbenchPreviewDialog
      :open="previewDialogOpen"
      :title="previewDialogTitle"
      :preview-url="previewDialogUrl"
      :fallback-src="previewDialogFallback"
      :mime-type="previewDialogMimeType"
      :filename="previewDialogFilename"
      :meta-rows="previewDialogRows"
      :empty-label="previewDialogEmptyLabel"
      eyebrow="暗场预览"
      download-label="下载这个素材"
      @close="closePreview"
      @download="downloadPreviewAsset"
    />
    </template>
  </section>
</template>
