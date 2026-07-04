<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowDownAZ, ArrowUpAZ, Download, Eye, FileDown, Inbox, Pencil, RefreshCw, Save, Trash2, X } from 'lucide-vue-next'

import { buildTimestampedZipFilename, downloadBatchAsZip } from '@/utils/batchZipDownload'
import { useAssetWorkbenchSessionStore } from '@aw/app/session.store'
import {
  assetWorkbenchApi,
  type DriveDirectoryRow,
  type DriveFileRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { formatMoney } from '@aw/shared/format/number'
import DriveThumb from '@aw/shared/drive/DriveThumb.vue'
import WorkbenchPreviewDialog from '@aw/shared/preview/WorkbenchPreviewDialog.vue'

type SortBy = 'created_at' | 'owner' | 'directory' | 'name' | 'format'
type SortDir = 'asc' | 'desc'

const session = useAssetWorkbenchSessionStore()
const route = useRoute()
const capabilities = computed(() => new Set(session.bootstrap?.capabilities ?? []))
const canManageDrive = computed(() => capabilities.value.has('asset.workbench.manage'))

const pageSize = 50
const exportLimit = 5000
const skeletonRowCount = 8
const dateTimeFormatter = new Intl.DateTimeFormat('zh-CN', {
  timeZone: 'Asia/Shanghai',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
})

const directories = ref<DriveDirectoryRow[]>([])
const files = ref<DriveFileRow[]>([])
const selectedFile = ref<DriveFileRow | null>(null)
const selectedIds = ref<Set<number>>(new Set())
const loading = ref(false)
const actionLoading = ref(false)
const exporting = ref(false)
const error = ref('')
const notice = ref('')
const page = ref(1)
const total = ref(0)
const query = ref('')
const owner = ref('')
const createdFrom = ref('')
const createdTo = ref('')
const directory = ref('all')
const sortBy = ref<SortBy>('created_at')
const sortDir = ref<SortDir>('desc')
const moveTargetDirectoryId = ref(0)
const actionReason = ref('')
const editingFileId = ref<number | null>(null)
const editingName = ref('')
const previewOpen = ref(false)
const previewUrl = ref('')
const previewEmptyLabel = ref('')
const previewTitle = ref('')
const previewMimeType = ref('')
const previewRows = ref<Array<[string, string]>>([])

let requestSeq = 0
let listAbortController: AbortController | null = null

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
const allPageSelected = computed(() => files.value.length > 0 && files.value.every((file) => selectedIds.value.has(file.id)))
const filteredDirectoryLabel = computed(() => {
  if (directory.value === 'all') return '全部分类'
  if (directory.value === 'unassigned') return '未分类'
  const item = directories.value.find((dir) => String(dir.directory_id || '') === directory.value)
  return item?.name || '指定分类'
})
const totalAmount = computed(() => files.value.reduce((sum, file) => sum + Number(file.gross_amount || 0), 0))
const totalCount = computed(() => files.value.reduce((sum, file) => sum + Number(file.page_count || 0), 0))
const activeFilterCount = computed(() =>
  [query.value.trim(), owner.value.trim(), createdFrom.value, createdTo.value, directory.value !== 'all' ? directory.value : ''].filter(Boolean).length,
)
const directoryOptions = computed(() => directories.value.filter((dir) => Number(dir.directory_id || 0) > 0))

function directoryParams() {
  if (directory.value === 'unassigned') return { unassigned: true }
  const id = Number(directory.value)
  if (id > 0) return { dir_id: id }
  return {}
}

function formatDateTime(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return dateTimeFormatter.format(date).replace(/\//g, '-')
}

function formatSize(size?: number): string {
  if (!size) return '—'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

function fileDisplayName(file: DriveFileRow): string {
  return file.display_name || file.original_filename || `文件 ${file.id}`
}

function secondaryFilename(file: DriveFileRow): string {
  const original = file.original_filename || ''
  return original && original !== fileDisplayName(file) ? original : ''
}

function fileOwnerLabel(file: DriveFileRow): string {
  return file.owner_name || file.owner_username || (file.owner_user_id ? `用户 ${file.owner_user_id}` : '—')
}

function fileOwnerSecondary(file: DriveFileRow): string {
  const secondary = file.owner_username || (file.owner_user_id ? `ID ${file.owner_user_id}` : '')
  return secondary && secondary !== fileOwnerLabel(file) ? secondary : ''
}

function fileExtLabel(file: DriveFileRow): string {
  const rawType = file.file_type?.trim().replace(/^\./, '')
  const name = file.original_filename || file.display_name || ''
  const suffix = name.includes('.') ? name.split('.').pop()?.trim() : ''
  const ext = (rawType || suffix || '').replace(/^\./, '')
  return ext ? ext.toUpperCase() : '—'
}

function fileMimeLabel(file: DriveFileRow): string {
  return file.mime_type || ''
}

function fileFormatLabel(file: DriveFileRow): string {
  const extLabel = fileExtLabel(file)
  const mime = fileMimeLabel(file)
  if (mime) return extLabel !== '—' ? `${extLabel} · ${mime}` : mime
  return extLabel
}

function statusText(value?: string) {
  const map: Record<string, string> = {
    pending: '待处理',
    unpriced: '待补价',
    priced: '已计件',
    passed: '已通过',
    needs_fix: '需修',
    ready: '可预览',
    failed: '预览失败',
    processing: '处理中',
    unsettled: '未结算',
    in_batch: '批次中',
    settled: '已结算',
  }
  const normalized = String(value || '').trim()
  return map[normalized] || normalized || '—'
}

function statusToneClass(value?: string) {
  const normalized = String(value || '').trim()
  if (['priced', 'passed', 'ready', 'settled'].includes(normalized)) return 'aw-chip--success'
  if (['pending', 'unpriced', 'processing'].includes(normalized)) return 'aw-chip--warn'
  if (['needs_fix', 'failed'].includes(normalized)) return 'aw-chip--danger'
  if (normalized === 'in_batch') return 'aw-chip--info'
  return 'aw-chip--neutral'
}

function filePreviewRows(file: DriveFileRow): Array<[string, string]> {
  return [
    ['上传人', fileOwnerLabel(file)],
    ['上传时间', formatDateTime(file.created_at)],
    ['分类', file.upload_directory_name || '未分类'],
    ['作品名称', fileDisplayName(file)],
    ['原始文件名', file.original_filename || '—'],
    ['格式', fileFormatLabel(file)],
    ['数量', file.page_count ? `${file.page_count}` : '—'],
    ['计件金额', formatMoney(file.gross_amount || 0)],
    ['计件状态', statusText(file.pricing_status)],
    ['文件大小', formatSize(file.file_size)],
  ]
}

function csvEscape(value: unknown) {
  const text = String(value ?? '')
  if (/[",\n\r]/.test(text)) return `"${text.replace(/"/g, '""')}"`
  return text
}

function fileToExportRow(file: DriveFileRow) {
  return [
    fileOwnerLabel(file),
    formatDateTime(file.created_at),
    file.upload_directory_name || '未分类',
    fileDisplayName(file),
    file.original_filename || '',
    fileFormatLabel(file),
    file.page_count || '',
    formatMoney(file.gross_amount || 0),
    statusText(file.pricing_status),
    formatSize(file.file_size),
  ]
}

async function loadDirectories() {
  directories.value = await assetWorkbenchApi.driveDirectories()
}

async function loadFiles(nextPage = page.value) {
  const requestID = ++requestSeq
  listAbortController?.abort()
  listAbortController = new AbortController()
  loading.value = true
  error.value = ''
  try {
    const result = await assetWorkbenchApi.driveFiles(
      {
        ...directoryParams(),
        q: query.value.trim() || undefined,
        owner: owner.value.trim() || undefined,
        created_from: createdFrom.value || undefined,
        created_to: createdTo.value || undefined,
        sort_by: sortBy.value,
        sort_dir: sortDir.value,
        page: nextPage,
        page_size: pageSize,
      },
      listAbortController.signal,
    )
    if (requestID !== requestSeq) return
    files.value = result.items
    total.value = result.total
    page.value = nextPage
    if (selectedFile.value) {
      selectedFile.value = result.items.find((file) => file.id === selectedFile.value?.id) || null
    }
    selectedIds.value = new Set([...selectedIds.value].filter((id) => result.items.some((file) => file.id === id)))
  } catch (err) {
    if ((err as DOMException)?.name === 'AbortError') return
    files.value = []
    total.value = 0
    error.value = err instanceof Error ? err.message : '上传记录加载失败'
  } finally {
    if (requestID === requestSeq) loading.value = false
  }
}

async function applyFilters() {
  page.value = 1
  selectedIds.value = new Set()
  await loadFiles(1)
}

async function resetFilters() {
  query.value = ''
  owner.value = ''
  createdFrom.value = ''
  createdTo.value = ''
  directory.value = 'all'
  await applyFilters()
}

async function changePage(delta: number) {
  const next = page.value + delta
  if (next < 1 || next > totalPages.value) return
  await loadFiles(next)
}

async function setSort(nextSort: SortBy) {
  if (sortBy.value === nextSort) sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  else {
    sortBy.value = nextSort
    sortDir.value = nextSort === 'created_at' ? 'desc' : 'asc'
  }
  await applyFilters()
}

function sortIcon(field: SortBy) {
  if (sortBy.value !== field) return null
  return sortDir.value === 'asc' ? ArrowUpAZ : ArrowDownAZ
}

function selectFile(file: DriveFileRow) {
  selectedFile.value = selectedFile.value?.id === file.id ? null : file
}

function closeDetail() {
  selectedFile.value = null
}

function toggleFile(file: DriveFileRow, checked: boolean) {
  const next = new Set(selectedIds.value)
  if (checked) next.add(file.id)
  else next.delete(file.id)
  selectedIds.value = next
  selectedFile.value = file
}

function togglePageSelection(checked: boolean) {
  selectedIds.value = checked ? new Set(files.value.map((file) => file.id)) : new Set()
}

function clearSelection() {
  selectedIds.value = new Set()
}

function startEditName(file: DriveFileRow) {
  editingFileId.value = file.id
  editingName.value = fileDisplayName(file)
  selectedFile.value = file
}

function cancelEditName() {
  editingFileId.value = null
  editingName.value = ''
}

async function saveDisplayName(file: DriveFileRow) {
  const nextName = editingName.value.trim()
  if (!nextName) {
    error.value = '作品名称不能为空'
    return
  }
  actionLoading.value = true
  error.value = ''
  notice.value = ''
  try {
    const updated = await assetWorkbenchApi.updateSubmissionFile(file.id, { display_name: nextName })
    files.value = files.value.map((item) => (item.id === file.id ? { ...item, display_name: updated.display_name || nextName } : item))
    if (selectedFile.value?.id === file.id) selectedFile.value = { ...selectedFile.value, display_name: updated.display_name || nextName }
    cancelEditName()
    notice.value = '作品名称已保存'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '作品名称保存失败'
  } finally {
    actionLoading.value = false
  }
}

async function openFilePreview(file: DriveFileRow) {
  selectedFile.value = file
  previewOpen.value = true
  previewTitle.value = fileDisplayName(file)
  previewMimeType.value = file.mime_type || ''
  previewRows.value = filePreviewRows(file)
  previewUrl.value = ''
  previewEmptyLabel.value = '正在加载预览…'
  try {
    const meta = await assetWorkbenchApi.getFilePreview(file.id)
    previewUrl.value = meta.preview_url || ''
    previewEmptyLabel.value = meta.preview_url ? '' : '暂无可展示预览，可下载原文件查看'
  } catch (err) {
    previewEmptyLabel.value = err instanceof Error ? err.message : '预览加载失败'
  }
}

async function downloadFile(file: DriveFileRow) {
  actionLoading.value = true
  error.value = ''
  try {
    const meta = await assetWorkbenchApi.getFileDownload(file.id)
    if (meta.download_url) window.open(meta.download_url, '_blank', 'noopener,noreferrer')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '下载链接生成失败'
  } finally {
    actionLoading.value = false
  }
}

async function downloadSelectedFiles() {
  const ids = [...selectedIds.value]
  if (!ids.length) return
  actionLoading.value = true
  error.value = ''
  notice.value = '正在准备打包下载'
  try {
    const manifest = await assetWorkbenchApi.batchDownloadFiles(ids)
    await downloadBatchAsZip({
      items: manifest.items.map((item) => ({
        key: String(item.file_id),
        filename: item.filename,
        downloadURL: item.download_url,
        fallbackName: `file-${item.file_id}`,
      })),
      serverFailures: (manifest.failures ?? []).map((failure) => `file_id=${failure.file_id} reason=${failure.reason}`),
      zipFilename: buildTimestampedZipFilename('asset-upload-overview'),
    })
    const failureCount = manifest.failures?.length || 0
    notice.value = failureCount ? `已下载 ${manifest.items.length} 个，${failureCount} 个失败` : `已下载 ${manifest.items.length} 个文件`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '批量下载失败'
  } finally {
    actionLoading.value = false
  }
}

async function moveSelectedFiles() {
  const ids = [...selectedIds.value]
  if (!ids.length || !moveTargetDirectoryId.value || !canManageDrive.value) return
  actionLoading.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await assetWorkbenchApi.batchMoveFiles(ids, moveTargetDirectoryId.value, actionReason.value || '上传总览移动文件')
    notice.value = result.failures?.length ? `已移动 ${result.files?.length || 0} 个，${result.failures.length} 个失败` : `已移动 ${result.files?.length || 0} 个文件`
    selectedIds.value = new Set()
    await loadFiles()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '移动失败'
  } finally {
    actionLoading.value = false
  }
}

async function deleteSelectedFiles() {
  const ids = [...selectedIds.value]
  const reason = actionReason.value.trim()
  if (!ids.length || !reason || !canManageDrive.value) return
  actionLoading.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await assetWorkbenchApi.batchDeleteFiles(ids, reason)
    notice.value = result.failures?.length ? `已删除 ${result.deleted?.length || 0} 个，${result.failures.length} 个失败` : `已删除 ${result.deleted?.length || 0} 个文件`
    selectedIds.value = new Set()
    await loadFiles()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '删除失败'
  } finally {
    actionLoading.value = false
  }
}

async function exportCurrentFilter() {
  exporting.value = true
  error.value = ''
  notice.value = ''
  try {
    const rows: DriveFileRow[] = []
    let nextPage = 1
    for (;;) {
      const result = await assetWorkbenchApi.driveFiles({
        ...directoryParams(),
        q: query.value.trim() || undefined,
        owner: owner.value.trim() || undefined,
        created_from: createdFrom.value || undefined,
        created_to: createdTo.value || undefined,
        sort_by: sortBy.value,
        sort_dir: sortDir.value,
        page: nextPage,
        page_size: 200,
      })
      rows.push(...result.items)
      if (rows.length >= result.total || rows.length >= exportLimit || result.items.length === 0) break
      nextPage += 1
    }
    const header = ['创建人', '创建日期', '分类', '作品名称', '原始文件名', '格式', '数量', '计件金额', '状态', '文件大小']
    const csv = [header, ...rows.map(fileToExportRow)].map((row) => row.map(csvEscape).join(',')).join('\n')
    const blob = new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `上传总览-${new Date().toISOString().slice(0, 10)}.csv`
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
    notice.value = rows.length >= exportLimit && rows.length < total.value ? `已导出前 ${exportLimit} 条，请缩小筛选范围导出完整结果` : `已导出 ${rows.length} 条记录`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '导出失败'
  } finally {
    exporting.value = false
  }
}

onMounted(async () => {
  if (typeof route.query.q === 'string') query.value = route.query.q
  await Promise.all([loadDirectories(), loadFiles(1)])
})

onBeforeUnmount(() => {
  listAbortController?.abort()
})
</script>

<template>
  <section class="aw-upload-ledger aw-token-scope">
    <header class="aw-upload-ledger__hero">
      <div class="aw-upload-ledger__hero-copy">
        <p class="aw-eyebrow">上传总览</p>
        <h2>全站上传台账</h2>
        <span>按创建人、创建日期、分类、作品名称和格式跟踪已上传作品。</span>
      </div>
      <dl class="aw-upload-ledger__summary" aria-label="当前筛选汇总">
        <div>
          <dt>匹配记录</dt>
          <dd>{{ total }}</dd>
        </div>
        <div>
          <dt>本页数量</dt>
          <dd>{{ totalCount }}</dd>
        </div>
        <div>
          <dt>本页金额</dt>
          <dd>{{ formatMoney(totalAmount) }}</dd>
        </div>
      </dl>
    </header>

    <section class="aw-upload-ledger__toolbar" aria-label="上传记录筛选">
      <form class="aw-upload-ledger__filters" @submit.prevent="applyFilters">
        <label>
          <span>关键词</span>
          <input v-model.trim="query" type="search" placeholder="作品名称 / 文件名 / 格式" />
        </label>
        <label>
          <span>创建人</span>
          <input v-model.trim="owner" placeholder="姓名或账号" />
        </label>
        <label>
          <span>分类</span>
          <select v-model="directory">
            <option value="all">全部分类</option>
            <option value="unassigned">未分类</option>
            <option v-for="dir in directoryOptions" :key="dir.directory_id ?? dir.name" :value="String(dir.directory_id)">
              {{ dir.name }}
            </option>
          </select>
        </label>
        <label>
          <span>开始日期</span>
          <input v-model="createdFrom" type="date" />
        </label>
        <label>
          <span>结束日期</span>
          <input v-model="createdTo" type="date" />
        </label>
        <div class="aw-upload-ledger__filter-actions">
          <button class="aw-primary-button" type="submit" :disabled="loading">查询</button>
          <button v-if="activeFilterCount" class="aw-grid-button" type="button" @click="resetFilters">重置 {{ activeFilterCount }}</button>
        </div>
      </form>
      <div class="aw-upload-ledger__tools">
        <button class="aw-secondary-button" type="button" :disabled="loading" @click="loadFiles()">
          <RefreshCw :size="15" aria-hidden="true" />
          刷新
        </button>
        <button class="aw-secondary-button" type="button" :disabled="exporting || loading" @click="exportCurrentFilter">
          <FileDown :size="15" aria-hidden="true" />
          {{ exporting ? '导出中' : '导出' }}
        </button>
      </div>
    </section>

    <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
    <p v-if="error" class="aw-inline-alert aw-inline-alert--error">{{ error }}</p>

    <section v-if="selectedIds.size" class="aw-upload-ledger__batch" aria-label="批量操作">
      <strong>已选择 {{ selectedIds.size }} 个作品</strong>
      <button class="aw-secondary-button" type="button" :disabled="actionLoading" @click="downloadSelectedFiles">
        <Download :size="15" aria-hidden="true" />
        下载
      </button>
      <template v-if="canManageDrive">
        <select v-model.number="moveTargetDirectoryId" aria-label="移动目标分类">
          <option :value="0">移动到分类…</option>
          <option v-for="dir in directoryOptions" :key="dir.directory_id ?? dir.name" :value="Number(dir.directory_id)">{{ dir.name }}</option>
        </select>
        <button class="aw-secondary-button" type="button" :disabled="actionLoading || !moveTargetDirectoryId" @click="moveSelectedFiles">移动</button>
        <input v-model.trim="actionReason" placeholder="删除原因或移动备注" aria-label="操作原因" />
        <button class="aw-secondary-button aw-secondary-button--danger" type="button" :disabled="actionLoading || !actionReason.trim()" @click="deleteSelectedFiles">
          <Trash2 :size="15" aria-hidden="true" />
          删除
        </button>
      </template>
      <button class="aw-grid-button" type="button" @click="clearSelection">取消选择</button>
    </section>

    <section class="aw-upload-ledger__content" :class="{ 'has-detail': Boolean(selectedFile) }">
      <div class="aw-upload-ledger__table-wrap">
        <table class="aw-upload-ledger__table">
          <colgroup>
            <col class="aw-upload-ledger__col-check" />
            <col class="aw-upload-ledger__col-thumb" />
            <col class="aw-upload-ledger__col-owner" />
            <col class="aw-upload-ledger__col-date" />
            <col class="aw-upload-ledger__col-dir" />
            <col class="aw-upload-ledger__col-name" />
            <col class="aw-upload-ledger__col-format" />
            <col class="aw-upload-ledger__col-count" />
            <col class="aw-upload-ledger__col-amount" />
            <col class="aw-upload-ledger__col-status" />
            <col class="aw-upload-ledger__col-size" />
            <col class="aw-upload-ledger__col-actions" />
          </colgroup>
          <thead>
            <tr>
              <th class="aw-upload-ledger__check">
                <input type="checkbox" :checked="allPageSelected" aria-label="选择当前页" @change="togglePageSelection(($event.target as HTMLInputElement).checked)" />
              </th>
              <th>预览</th>
              <th>
                <button type="button" @click="setSort('owner')">
                  创建人
                  <component :is="sortIcon('owner')" v-if="sortIcon('owner')" :size="13" />
                </button>
              </th>
              <th>
                <button type="button" @click="setSort('created_at')">
                  创建时间
                  <component :is="sortIcon('created_at')" v-if="sortIcon('created_at')" :size="13" />
                </button>
              </th>
              <th>
                <button type="button" @click="setSort('directory')">
                  分类
                  <component :is="sortIcon('directory')" v-if="sortIcon('directory')" :size="13" />
                </button>
              </th>
              <th>
                <button type="button" @click="setSort('name')">
                  作品名称
                  <component :is="sortIcon('name')" v-if="sortIcon('name')" :size="13" />
                </button>
              </th>
              <th>
                <button type="button" @click="setSort('format')">
                  格式
                  <component :is="sortIcon('format')" v-if="sortIcon('format')" :size="13" />
                </button>
              </th>
              <th class="aw-upload-ledger__num">数量</th>
              <th class="aw-upload-ledger__num">金额</th>
              <th>状态</th>
              <th class="aw-upload-ledger__num">大小</th>
              <th class="aw-upload-ledger__center">操作</th>
            </tr>
          </thead>
          <tbody>
            <template v-if="loading">
              <tr v-for="i in skeletonRowCount" :key="`skeleton-${i}`" class="aw-upload-ledger__row-skeleton" aria-hidden="true">
                <td class="aw-upload-ledger__check"><span class="aw-upload-ledger__skeleton aw-upload-ledger__skeleton--dot" /></td>
                <td><span class="aw-upload-ledger__skeleton aw-upload-ledger__skeleton--thumb" /></td>
                <td colspan="10"><span class="aw-upload-ledger__skeleton" :style="{ width: `${92 - (i % 4) * 14}%` }" /></td>
              </tr>
            </template>
            <tr v-else-if="files.length === 0">
              <td colspan="12">
                <div class="aw-upload-ledger__empty">
                  <Inbox :size="30" aria-hidden="true" />
                  <strong>没有匹配的上传记录</strong>
                  <p>试试调整关键词、创建人、分类或日期范围。</p>
                  <button v-if="activeFilterCount" class="aw-grid-button" type="button" @click="resetFilters">清除全部筛选</button>
                </div>
              </td>
            </tr>
            <template v-else>
              <tr
                v-for="file in files"
                :key="file.id"
                :class="{ 'is-selected': selectedFile?.id === file.id }"
                @click="selectFile(file)"
                @dblclick="openFilePreview(file)"
              >
                <td class="aw-upload-ledger__check">
                  <input
                    type="checkbox"
                    :checked="selectedIds.has(file.id)"
                    :aria-label="`选择 ${fileDisplayName(file)}`"
                    @click.stop
                    @change="toggleFile(file, ($event.target as HTMLInputElement).checked)"
                  />
                </td>
                <td>
                  <button class="aw-upload-ledger__thumb" type="button" :aria-label="`预览 ${fileDisplayName(file)}`" @click.stop="openFilePreview(file)">
                    <DriveThumb :file-id="file.id" :filename="fileDisplayName(file)" :mime-type="file.mime_type" :preview-status="file.preview_status" size="sm" />
                  </button>
                </td>
                <td class="aw-upload-ledger__owner">
                  <strong :title="fileOwnerLabel(file)">{{ fileOwnerLabel(file) }}</strong>
                  <small v-if="fileOwnerSecondary(file)" :title="fileOwnerSecondary(file)">{{ fileOwnerSecondary(file) }}</small>
                </td>
                <td class="aw-upload-ledger__date">{{ formatDateTime(file.created_at) }}</td>
                <td>
                  <span class="aw-chip aw-chip--neutral" :title="file.upload_directory_name || '未分类'">{{ file.upload_directory_name || '未分类' }}</span>
                </td>
                <td class="aw-upload-ledger__name">
                  <form v-if="editingFileId === file.id" @submit.prevent="saveDisplayName(file)" @click.stop>
                    <input v-model.trim="editingName" aria-label="作品名称" />
                    <button type="submit" :disabled="actionLoading" aria-label="保存作品名称">
                      <Save :size="14" aria-hidden="true" />
                    </button>
                    <button type="button" aria-label="取消编辑" @click="cancelEditName">
                      <X :size="14" aria-hidden="true" />
                    </button>
                  </form>
                  <button v-else type="button" @click.stop="startEditName(file)">
                    <span>
                      <strong :title="fileDisplayName(file)">{{ fileDisplayName(file) }}</strong>
                      <small v-if="secondaryFilename(file)" :title="secondaryFilename(file)">{{ secondaryFilename(file) }}</small>
                    </span>
                    <Pencil :size="14" aria-hidden="true" />
                  </button>
                </td>
                <td class="aw-upload-ledger__format">
                  <strong>{{ fileExtLabel(file) }}</strong>
                  <small v-if="fileMimeLabel(file)" :title="fileMimeLabel(file)">{{ fileMimeLabel(file) }}</small>
                </td>
                <td class="aw-upload-ledger__num">{{ file.page_count || '—' }}</td>
                <td class="aw-upload-ledger__num">{{ formatMoney(file.gross_amount || 0) }}</td>
                <td>
                  <span class="aw-chip" :class="statusToneClass(file.pricing_status)">{{ statusText(file.pricing_status) }}</span>
                </td>
                <td class="aw-upload-ledger__num">{{ formatSize(file.file_size) }}</td>
                <td>
                  <div class="aw-upload-ledger__row-actions">
                    <button type="button" aria-label="预览" @click.stop="openFilePreview(file)">
                      <Eye :size="15" aria-hidden="true" />
                    </button>
                    <button type="button" aria-label="下载" @click.stop="downloadFile(file)">
                      <Download :size="15" aria-hidden="true" />
                    </button>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>

      <aside v-if="selectedFile" class="aw-upload-ledger__detail" aria-label="当前作品详情">
        <div class="aw-upload-ledger__detail-head">
          <h3 :title="fileDisplayName(selectedFile)">{{ fileDisplayName(selectedFile) }}</h3>
          <button type="button" class="aw-upload-ledger__detail-close" aria-label="关闭详情" @click="closeDetail">
            <X :size="15" aria-hidden="true" />
          </button>
        </div>
        <div class="aw-upload-ledger__detail-thumb">
          <DriveThumb :file-id="selectedFile.id" :filename="fileDisplayName(selectedFile)" :mime-type="selectedFile.mime_type" :preview-status="selectedFile.preview_status" />
        </div>
        <dl>
          <template v-for="[label, value] in filePreviewRows(selectedFile)" :key="label">
            <dt>{{ label }}</dt>
            <dd :title="value">{{ value }}</dd>
          </template>
        </dl>
        <div class="aw-upload-ledger__detail-actions">
          <button class="aw-primary-button" type="button" @click="openFilePreview(selectedFile)">打开预览</button>
          <button class="aw-secondary-button" type="button" :disabled="actionLoading" @click="downloadFile(selectedFile)">
            <Download :size="15" aria-hidden="true" />
            下载
          </button>
        </div>
      </aside>
    </section>

    <footer class="aw-upload-ledger__pager">
      <span>{{ filteredDirectoryLabel }} · 共 {{ total }} 条 · 第 {{ page }} / {{ totalPages }} 页</span>
      <div>
        <button class="aw-grid-button" type="button" :disabled="page <= 1 || loading" @click="changePage(-1)">上一页</button>
        <button class="aw-grid-button" type="button" :disabled="page >= totalPages || loading" @click="changePage(1)">下一页</button>
      </div>
    </footer>

    <WorkbenchPreviewDialog
      :open="previewOpen"
      :title="previewTitle"
      eyebrow="上传作品预览"
      :preview-url="previewUrl"
      :mime-type="previewMimeType"
      :empty-label="previewEmptyLabel"
      :meta-rows="previewRows"
      @close="previewOpen = false"
      @download="selectedFile && downloadFile(selectedFile)"
    />
  </section>
</template>
