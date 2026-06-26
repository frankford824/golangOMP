<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'

import { assetWorkbenchApi, type SystemAssetRow } from '@aw/shared/api/assetWorkbenchApi'
import { buildTimestampedZipFilename, downloadBatchAsZip } from '@/utils/batchZipDownload'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'

interface GridColumn {
  key: string
  label: string
  width: number
  align?: 'left' | 'right' | 'center'
}

type MaterialGridRow = SystemAssetRow & {
  display_no: string | number
  display_name: string
  display_type: string
  preview_label: string
}

const keyword = ref('')
const rows = ref<SystemAssetRow[]>([])
const total = ref(0)
const selectedAssetIds = ref<Set<number>>(new Set())
const loading = ref(false)
const error = ref('')
const notice = ref('')
let controller: AbortController | null = null

const downloadableAssetIds = computed(() => rows.value.map((row) => row.id).filter((id) => id > 0))
const materialRowsWithLabels = computed<MaterialGridRow[]>(() =>
  rows.value.map((row) => ({
    ...row,
    display_no: row.asset_no || row.resource_id || row.id,
    display_name: row.product_name || row.task_no || row.file_name || '',
    display_type: row.mime_type || 'unknown',
    preview_label: row.preview_available ? '可预览' : '只下载',
  })),
)
const materialGridRows = computed(() => materialRowsWithLabels.value as unknown as Record<string, unknown>[])
const materialGridColumns = computed<GridColumn[]>(() => [
  { key: 'select', label: '选择', width: 84, align: 'center' },
  { key: 'display_no', label: '编码', width: 150 },
  { key: 'display_name', label: '名称', width: 220 },
  { key: 'display_type', label: '类型', width: 120 },
  { key: 'preview_label', label: '预览', width: 96 },
  { key: 'actions', label: '动作', width: 96, align: 'center' },
])

async function searchMaterials() {
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await assetWorkbenchApi.systemSearch({ q: keyword.value, limit: 40 }, controller.signal)
    rows.value = result.items
    total.value = result.total
    selectedAssetIds.value = new Set()
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') return
    error.value = err instanceof Error ? err.message : '素材库搜索失败'
  } finally {
    loading.value = false
  }
}

function toggleAsset(row: SystemAssetRow, checked: boolean) {
  const next = new Set(selectedAssetIds.value)
  if (checked) next.add(row.id)
  else next.delete(row.id)
  selectedAssetIds.value = next
}

function toggleAllAssets(checked: boolean) {
  selectedAssetIds.value = checked ? new Set(downloadableAssetIds.value) : new Set()
}

function gridRowAsAsset(row: Record<string, unknown>): MaterialGridRow {
  return row as unknown as MaterialGridRow
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
    notice.value = `已生成下载链接：${info.filename || row.file_name || row.asset_no || row.id}`
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
})
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-heading">
      <div>
        <p class="aw-eyebrow">只读素材</p>
        <h2>模板素材库</h2>
      </div>
      <button class="aw-secondary-button" type="button" @click="downloadSelectedAssets">批量下载</button>
    </div>
    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <input v-model="keyword" type="search" placeholder="搜索编码、款式编码、产品名或趋势" @keydown.enter="searchMaterials" />
        <button type="button" @click="searchMaterials">搜索</button>
        <label class="aw-inline-check">
          <input
            type="checkbox"
            :checked="selectedAssetIds.size > 0 && selectedAssetIds.size === downloadableAssetIds.length"
            @change="toggleAllAssets(($event.target as HTMLInputElement).checked)"
          />
          <span>全选</span>
        </label>
      </div>
      <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
      <p v-if="loading" class="aw-copy">正在搜索系统素材</p>
      <p v-else-if="error" class="aw-copy">{{ error }}</p>
      <WorkbenchDataGrid
        v-else-if="rows.length"
        :columns="materialGridColumns"
        :rows="materialGridRows"
        row-key="id"
        storage-key="materials"
        group-by="display_type"
        :height="420"
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
          <button v-else-if="column.key === 'actions'" type="button" @click="downloadAsset(gridRowAsAsset(row))">下载</button>
          <strong v-else-if="column.key === 'preview_label'">{{ value }}</strong>
          <span v-else>{{ value }}</span>
        </template>
      </WorkbenchDataGrid>
      <div v-else class="aw-empty-state">
        <h3>还没有搜索结果</h3>
        <p>输入编码、产品名、款式或关键词后搜索。只有有素材库权限的账号可以查看和下载。</p>
      </div>
    </div>
  </section>
</template>
