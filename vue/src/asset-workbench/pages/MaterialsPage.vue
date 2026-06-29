<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Download, Grid3X3, List, Search, X } from 'lucide-vue-next'

import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'
import { assetWorkbenchApi, type SystemAssetPreviewMeta, type SystemAssetRow } from '@aw/shared/api/assetWorkbenchApi'
import { buildTimestampedZipFilename, downloadBatchAsZip } from '@/utils/batchZipDownload'
import { formatInt } from '@aw/shared/format/number'
import { chipClass, systemPreviewMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import MaterialGallery from '@aw/shared/materials/MaterialGallery.vue'
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
  display_no: string | number
  display_name: string
  display_type: string
  preview_label: string
  task_label: string
}

const keyword = ref('')
const rows = ref<SystemAssetRow[]>([])
const total = ref(0)
const selectedAssetIds = ref<Set<number>>(new Set())
const activeAsset = ref<SystemAssetRow | null>(null)
const previewAsset = ref<SystemAssetRow | null>(null)
const previewMeta = ref<SystemAssetPreviewMeta | null>(null)
const previewUrls = ref<Record<number, string>>({})
const previewLoadingIds = ref<Set<number>>(new Set())
const viewMode = ref<ViewMode>('gallery')
const typeFilter = ref('all')
const previewFilter = ref('all')
const loading = ref(false)
const previewLoading = ref(false)
const error = ref('')
const notice = ref('')
let controller: AbortController | null = null
let previewController: AbortController | null = null
let lastSelectedIndex = -1

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
    display_no: codeOf(row),
    display_name: titleOf(row),
    display_type: typeLabel(row),
    preview_label: systemPreviewMeta(canPreviewMaterial(row)).label,
    task_label: row.task_no || '无任务号',
  })),
)
const materialGridRows = computed(() => materialRowsWithLabels.value as unknown as Record<string, unknown>[])
const materialGridColumns = computed<GridColumn[]>(() => [
  { key: 'select', label: '选择', width: 84, align: 'center' },
  { key: 'display_no', label: '编码', width: 160 },
  { key: 'display_name', label: '素材名称', width: 260 },
  { key: 'display_type', label: '类型', width: 120 },
  { key: 'task_label', label: '任务', width: 150 },
  { key: 'preview_label', label: '预览', width: 120 },
  { key: 'actions', label: '动作', width: 140, align: 'center' },
])
const selectedCount = computed(() => selectedAssetIds.value.size)
const selectedLabel = computed(() => (selectedCount.value > 0 ? `已选 ${formatInt(selectedCount.value)} 个素材` : '未选择素材'))
const activeDetailRows = computed(() => {
  const asset = activeAsset.value
  if (!asset) return []
  return [
    ['编码', codeOf(asset)],
    ['名称', titleOf(asset)],
    ['文件类型', typeLabel(asset)],
    ['任务号', asset.task_no || '—'],
    ['产品名', asset.product_name || '—'],
    ['原文件', asset.original_filename || asset.file_name || '—'],
    ['资源 ID', asset.resource_id || '—'],
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

async function searchMaterials() {
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await assetWorkbenchApi.systemSearch({ q: keyword.value, limit: 100 }, controller.signal)
    rows.value = result.items
    total.value = result.total
    selectedAssetIds.value = new Set()
    activeAsset.value = result.items[0] ?? null
    previewUrls.value = {}
    lastSelectedIndex = -1
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') return
    error.value = err instanceof Error ? err.message : '素材库搜索失败'
  } finally {
    loading.value = false
  }
}

function titleOf(asset: SystemAssetRow) {
  return asset.product_name || asset.original_filename || asset.file_name || asset.task_no || `素材 ${asset.id}`
}

function codeOf(asset: SystemAssetRow) {
  return asset.asset_no || asset.resource_id || asset.id
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

async function ensurePreview(asset: SystemAssetRow, silent = false) {
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
    const meta = await assetWorkbenchApi.previewSystemAsset(asset.id)
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

async function openAssetPreview(asset: SystemAssetRow) {
  activeAsset.value = asset
  previewController?.abort()
  previewController = new AbortController()
  previewLoading.value = true
  error.value = ''
  notice.value = ''
  try {
    const meta = await ensurePreview(asset)
    previewMeta.value = meta
    if (resolvedSystemAssetPreviewUrl(meta)) {
      previewAsset.value = asset
    } else if (canPreviewMaterial(asset)) {
      notice.value = '这个素材暂时没有可展示的预览图'
    }
  } finally {
    previewLoading.value = false
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
    error.value = err instanceof Error ? err.message : '系统素材下载失败'
  }
}

async function downloadSelectedAssets() {
  const ids = Array.from(selectedAssetIds.value)
  if (!ids.length) {
    notice.value = '请选择要下载的素材'
    return
  }
  notice.value = '正在生成系统素材下载包'
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
    notice.value = `已打包 ${result.writtenCount} 个系统素材，失败 ${result.failureCount} 个`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '系统素材批量下载失败'
  }
}

onBeforeUnmount(() => {
  controller?.abort()
  previewController?.abort()
})

onMounted(() => {
  void searchMaterials()
})
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">只读素材</p>
        <h2>模板素材库</h2>
        <p>用缩略图墙快速查阅系统素材，进入视区后按需加载预览，明细模式用于批量核对。</p>
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
        <input v-model="keyword" type="search" placeholder="搜索编码、款式编码、产品名或趋势" @keydown.enter="searchMaterials" />
        <button class="aw-primary-button" type="button" @click="searchMaterials">
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
        <select v-model="previewFilter" aria-label="预览能力筛选">
          <option value="all">全部预览状态</option>
          <option value="previewable">可预览</option>
          <option value="download_only">只下载</option>
        </select>
        <span class="aw-chip aw-chip--neutral">{{ formatInt(filteredRows.length) }} / {{ formatInt(total) }} 条</span>
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
          <p v-if="loading" class="aw-copy">正在搜索系统素材</p>
          <p v-else-if="error" class="aw-copy">{{ error }}</p>
          <WorkbenchDataGrid
            v-else-if="filteredRows.length"
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
            <p>输入编码、产品名、款式或关键词后搜索。只有有素材库权限的账号可以查看和下载。</p>
          </div>
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
            <span>{{ codeOf(activeAsset) }}</span>
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
              {{ codeOf(asset) }}
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
  </section>
</template>
