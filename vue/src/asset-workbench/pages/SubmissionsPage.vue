<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import {
  assetWorkbenchApi,
  type AssetWorkbenchSavedView,
  type SubmissionDetail,
  type SubmissionFileRow,
  type SubmissionItemRow,
  type SubmissionRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import { buildTimestampedZipFilename, downloadBatchAsZip } from '@/utils/batchZipDownload'
import { formatInt, formatMoney } from '@aw/shared/format/number'
import { chipClass, previewStatusMeta, pricingStatusMeta, qcStatusMeta, submissionStatusMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'

type ItemActionKind = 'needs_fix' | 'void' | 'reprice'

interface GridColumn {
  key: string
  label: string
  width: number
  align?: 'left' | 'right' | 'center'
}

type DetailItemGridRow = SubmissionItemRow & { file_count: number; action: string }
type DetailFileGridRow = SubmissionFileRow & { selected: boolean; action: string }
type SubmissionGridRow = SubmissionRow & { status_label: string }

interface PendingItemAction {
  kind: ItemActionKind
  item: SubmissionItemRow
  reason: string
}

const rows = ref<SubmissionRow[]>([])
const total = ref(0)
const savedViews = ref<AssetWorkbenchSavedView[]>([])
const selectedDetail = ref<SubmissionDetail | null>(null)
const selectedFileIds = ref<Set<number>>(new Set())
const pendingAction = ref<PendingItemAction | null>(null)
const loading = ref(false)
const detailLoading = ref(false)
const error = ref('')
const notice = ref('')
const viewName = ref('默认维护视图')
const groupBy = ref('business_month')
const density = ref('compact')
const { bootstrap, refresh: refreshBootstrap } = useAssetWorkbenchBootstrap()

const selectedFiles = computed(() => {
  const detail = selectedDetail.value
  if (!detail) return []
  return detail.items.flatMap((entry) => entry.files)
})
const gridRowHeight = computed(() => (density.value === 'compact' ? 34 : 44))
const submissionRowsWithLabels = computed<SubmissionGridRow[]>(() =>
  rows.value.map((row) => ({
    ...row,
    status_label: submissionStatusMeta(row.status).label,
  })),
)
const submissionGridRows = computed(() => submissionRowsWithLabels.value as unknown as Record<string, unknown>[])
const detailItemRows = computed<DetailItemGridRow[]>(() =>
  (selectedDetail.value?.items ?? []).map((entry) => ({
    ...entry.item,
    file_count: entry.files.length,
    action: 'actions',
  })),
)
const detailItemGridRows = computed(() => detailItemRows.value as unknown as Record<string, unknown>[])
const detailFileRows = computed<DetailFileGridRow[]>(() =>
  selectedFiles.value.map((file) => ({
    ...file,
    selected: selectedFileIds.value.has(file.id),
    action: 'actions',
  })),
)
const detailFileGridRows = computed(() => detailFileRows.value as unknown as Record<string, unknown>[])
const submissionGridColumns = computed<Array<{ key: string; label: string; width: number; align?: 'left' | 'right' | 'center' }>>(() => [
  { key: 'submission_no', label: '提交批次', width: 180 },
  { key: 'business_month', label: '结算月', width: 108 },
  { key: 'status_label', label: '状态', width: 96 },
  { key: 'item_count', label: '单数', width: 88, align: 'right' },
  { key: 'page_count', label: '页数', width: 88, align: 'right' },
  { key: 'gross_total', label: '毛额', width: 108, align: 'right' },
  { key: 'action', label: '动作', width: 96, align: 'center' },
])
const detailItemGridColumns = computed<GridColumn[]>(() => [
  { key: 'order_no', label: '订单号', width: 150 },
  { key: 'difficulty_class', label: '难度', width: 108 },
  { key: 'pricing_status', label: '计价', width: 96 },
  { key: 'qc_status', label: '质检', width: 96 },
  { key: 'gross_amount', label: '毛额', width: 108, align: 'right' },
  { key: 'file_count', label: '文件', width: 84, align: 'right' },
  { key: 'action', label: '动作', width: 230, align: 'center' },
])
const detailFileGridColumns = computed<GridColumn[]>(() => [
  { key: 'selected', label: '选择', width: 84, align: 'center' },
  { key: 'original_filename', label: '文件名', width: 240 },
  { key: 'file_type', label: '类型', width: 108 },
  { key: 'preview_status', label: '预览', width: 108 },
  { key: 'action', label: '动作', width: 96, align: 'center' },
])

const downloadableFileIDs = computed(() => selectedFiles.value.map((file) => file.id))
const pendingActionTitle = computed(() => {
  if (!pendingAction.value) return ''
  if (pendingAction.value.kind === 'needs_fix') return '标记需修'
  if (pendingAction.value.kind === 'void') return '作废明细'
  return '重新计价'
})
const canManageItems = computed(() => bootstrap.value?.capabilities.includes('asset.workbench.manage') === true)

async function loadSubmissions() {
  loading.value = true
  error.value = ''
  try {
    const result = await assetWorkbenchApi.listSubmissions({ page: 1, page_size: 50 })
    rows.value = result.items
    total.value = result.total
  } catch (err) {
    error.value = err instanceof Error ? err.message : '提交列表加载失败'
  } finally {
    loading.value = false
  }
}

async function loadSavedViews() {
  try {
    savedViews.value = await assetWorkbenchApi.listSavedViews('submissions')
  } catch {
    savedViews.value = []
  }
}

async function saveView() {
  notice.value = ''
  error.value = ''
  try {
    const saved = await assetWorkbenchApi.upsertSavedView({
      view_type: 'submissions',
      view_name: viewName.value,
      is_default: true,
      config_json: {
        group_by: groupBy.value,
        density: density.value,
        columns: ['submission_no', 'business_month', 'status', 'gross_total'],
      },
    })
    notice.value = `已保存视图：${saved.view_name}`
    await loadSavedViews()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '保存视图失败'
  }
}

function applyView(view: AssetWorkbenchSavedView) {
  viewName.value = view.view_name
  const config = view.config_json ?? {}
  if (typeof config.group_by === 'string') groupBy.value = config.group_by
  if (typeof config.density === 'string') density.value = config.density
}

async function openSubmission(row: SubmissionRow) {
  await loadSubmissionDetail(row.id)
}

async function loadSubmissionDetail(submissionId: number) {
  detailLoading.value = true
  error.value = ''
  notice.value = ''
  selectedFileIds.value = new Set()
  try {
    selectedDetail.value = await assetWorkbenchApi.getSubmissionDetail(submissionId)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '提交详情加载失败'
  } finally {
    detailLoading.value = false
  }
}

async function refreshSelectedDetail() {
  const submissionId = selectedDetail.value?.submission.id
  if (submissionId) await loadSubmissionDetail(submissionId)
  await loadSubmissions()
}

function toggleFile(file: SubmissionFileRow, checked: boolean) {
  const next = new Set(selectedFileIds.value)
  if (checked) next.add(file.id)
  else next.delete(file.id)
  selectedFileIds.value = next
}

function toggleAllFiles(checked: boolean) {
  selectedFileIds.value = checked ? new Set(downloadableFileIDs.value) : new Set()
}

function gridRowAsItem(row: Record<string, unknown>): DetailItemGridRow {
  return row as unknown as DetailItemGridRow
}

function gridRowAsSubmission(row: Record<string, unknown>): SubmissionGridRow {
  return row as unknown as SubmissionGridRow
}

function gridRowAsFile(row: Record<string, unknown>): DetailFileGridRow {
  return row as unknown as DetailFileGridRow
}

async function downloadFile(file: SubmissionFileRow) {
  notice.value = ''
  error.value = ''
  try {
    const meta = await assetWorkbenchApi.getFileDownload(file.id)
    window.open(meta.download_url, '_blank', 'noopener,noreferrer')
    notice.value = `已生成下载链接：${meta.filename}`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '下载链接生成失败'
  }
}

async function downloadSelectedFiles() {
  const ids = Array.from(selectedFileIds.value)
  if (!ids.length) {
    notice.value = '请选择要下载的文件'
    return
  }
  notice.value = '正在生成下载包'
  error.value = ''
  try {
    const manifest = await assetWorkbenchApi.batchDownloadFiles(ids)
    const result = await downloadBatchAsZip({
      items: manifest.items.map((item) => ({
        key: String(item.file_id),
        filename: item.filename,
        downloadURL: item.download_url,
        fallbackName: `file-${item.file_id}`,
      })),
      serverFailures: (manifest.failures ?? []).map((failure) => `file_id=${failure.file_id} reason=${failure.reason}`),
      zipFilename: buildTimestampedZipFilename('asset-workbench-files'),
      onStatus: (message) => {
        notice.value = message
      },
    })
    notice.value = `已打包 ${result.writtenCount} 个文件，失败 ${result.failureCount} 个`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '批量下载失败'
  }
}

async function updateItemQC(item: SubmissionItemRow, qcStatus: string) {
  notice.value = ''
  error.value = ''
  try {
    await assetWorkbenchApi.updateSubmissionItemQC(item.id, { qc_status: qcStatus })
    notice.value = `已更新 ${item.order_no}`
    await refreshSelectedDetail()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '质检状态更新失败'
  }
}

function startItemAction(item: SubmissionItemRow, kind: ItemActionKind) {
  pendingAction.value = { item, kind, reason: '' }
}

async function executePendingAction() {
  const action = pendingAction.value
  if (!action) return
  if (action.kind !== 'reprice' && !action.reason.trim()) {
    error.value = '请填写操作原因'
    return
  }
  notice.value = ''
  error.value = ''
  try {
    if (action.kind === 'needs_fix') {
      await assetWorkbenchApi.updateSubmissionItemQC(action.item.id, { qc_status: 'needs_fix', reason: action.reason.trim() })
      notice.value = `已标记需修：${action.item.order_no}`
    } else if (action.kind === 'void') {
      await assetWorkbenchApi.voidSubmissionItem(action.item.id, action.reason.trim())
      notice.value = `已作废 ${action.item.order_no}`
    } else {
      const updated = await assetWorkbenchApi.repriceSubmissionItem(action.item.id, action.reason.trim())
      notice.value = `已重计价 ${action.item.order_no}：${updated.pricing_status}`
    }
    pendingAction.value = null
    await refreshSelectedDetail()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '明细操作失败'
  }
}

onMounted(async () => {
  await Promise.all([refreshBootstrap(), loadSubmissions(), loadSavedViews()])
})
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">交付维护</p>
        <h2>资产维护专区</h2>
        <p>批量检查提交、文件预览、质检状态和结算前重计价。视图会记住分组、密度和列设置。</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-secondary-button" type="button" @click="downloadSelectedFiles">批量下载</button>
        <button class="aw-primary-button" type="button" @click="saveView">保存视图</button>
      </div>
    </div>
    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <input v-model="viewName" aria-label="视图名称" />
        <select v-model="groupBy" aria-label="分组字段">
          <option value="business_month">按月份</option>
          <option value="status">按状态</option>
          <option value="submitter_user_id">按提交人</option>
        </select>
        <select v-model="density" aria-label="表格密度">
          <option value="compact">紧凑</option>
          <option value="comfortable">舒展</option>
        </select>
        <span>{{ formatInt(total) }} 个批次</span>
      </div>
      <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
      <div v-if="savedViews.length" class="aw-button-row">
        <button
          v-for="view in savedViews"
          :key="view.id"
          class="aw-secondary-button"
          type="button"
          @click="applyView(view)"
        >
          {{ view.view_name }}
        </button>
      </div>
      <WorkbenchDataGrid
        v-if="rows.length"
        :columns="submissionGridColumns"
        :rows="submissionGridRows"
        row-key="id"
        storage-key="submissions"
        :group-by="groupBy"
        :height="420"
        :row-height="gridRowHeight"
      >
        <template #cell="{ row, column, value }">
          <button v-if="column.key === 'action'" type="button" @click="openSubmission(gridRowAsSubmission(row))">文件</button>
          <span
            v-else-if="column.key === 'status_label'"
            :class="chipClass(submissionStatusMeta(gridRowAsSubmission(row).status).tone)"
          >
            {{ value }}
          </span>
          <span v-else-if="column.key === 'gross_total'" class="aw-cell-money">{{ formatMoney(value) }}</span>
          <span v-else-if="column.align === 'right'" class="aw-cell-num">{{ formatInt(value) }}</span>
          <span v-else>{{ value || '—' }}</span>
        </template>
      </WorkbenchDataGrid>
      <div v-else class="aw-empty-state">
        <h3>还没有提交明细</h3>
        <p v-if="loading">正在加载提交列表</p>
        <p v-else-if="error">{{ error }}</p>
        <p v-else>当前没有可维护的提交。上传成品后，可以在这里质检、修正、下载和保存常用视图。</p>
      </div>
    </div>

    <div v-if="selectedDetail || detailLoading" class="aw-data-surface">
      <div class="aw-page-bar">
        <div class="aw-page-bar__copy">
          <p class="aw-eyebrow">提交文件</p>
          <h2>{{ selectedDetail?.submission.submission_no || '提交文件' }}</h2>
          <p>按明细处理质检、重计价、作废和文件批量下载。</p>
        </div>
        <div class="aw-page-bar__actions">
          <label class="aw-inline-check">
            <input
              type="checkbox"
              :checked="selectedFileIds.size > 0 && selectedFileIds.size === downloadableFileIDs.length"
              @change="toggleAllFiles(($event.target as HTMLInputElement).checked)"
            />
            <span>全选</span>
          </label>
        </div>
      </div>
      <p v-if="detailLoading" class="aw-copy">正在加载文件</p>
      <WorkbenchDataGrid
        v-else-if="selectedDetail?.items.length"
        :columns="detailItemGridColumns"
        :rows="detailItemGridRows"
        row-key="id"
        storage-key="submission-detail-items"
        group-by="qc_status"
        :height="300"
        :row-height="gridRowHeight"
      >
        <template #cell="{ row, column, value }">
          <div v-if="column.key === 'action'" class="aw-inline-actions">
            <template v-if="canManageItems">
              <button type="button" @click="updateItemQC(gridRowAsItem(row), 'checked')">通过</button>
              <button type="button" @click="startItemAction(gridRowAsItem(row), 'needs_fix')">需修</button>
              <button type="button" @click="startItemAction(gridRowAsItem(row), 'reprice')">重计价</button>
              <button type="button" @click="startItemAction(gridRowAsItem(row), 'void')">作废</button>
            </template>
            <span v-else class="aw-chip aw-chip--neutral">只读</span>
          </div>
          <span
            v-else-if="column.key === 'pricing_status'"
            :class="chipClass(pricingStatusMeta(gridRowAsItem(row).pricing_status).tone)"
          >
            {{ pricingStatusMeta(gridRowAsItem(row).pricing_status).label }}
          </span>
          <span
            v-else-if="column.key === 'qc_status'"
            :class="chipClass(qcStatusMeta(gridRowAsItem(row).qc_status).tone)"
          >
            {{ qcStatusMeta(gridRowAsItem(row).qc_status).label }}
          </span>
          <span v-else-if="column.key === 'gross_amount'" class="aw-cell-money">{{ formatMoney(value) }}</span>
          <span v-else-if="column.align === 'right'" class="aw-cell-num">{{ formatInt(value) }}</span>
          <span v-else>{{ value || '—' }}</span>
        </template>
      </WorkbenchDataGrid>
      <div v-if="pendingAction" class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <h3>{{ pendingActionTitle }}</h3>
            <p class="aw-copy">{{ pendingAction.item.order_no }}</p>
          </div>
          <span :class="chipClass(qcStatusMeta(pendingAction.item.qc_status).tone)">
            {{ qcStatusMeta(pendingAction.item.qc_status).label }}
          </span>
        </div>
        <label class="aw-field">
          <span>操作原因</span>
          <input v-model="pendingAction.reason" :required="pendingAction.kind !== 'reprice'" />
        </label>
        <div class="aw-inline-actions">
          <button class="aw-primary-button" type="button" @click="executePendingAction">确认执行</button>
          <button class="aw-secondary-button" type="button" @click="pendingAction = null">取消</button>
        </div>
      </div>
      <WorkbenchDataGrid
        v-if="selectedFiles.length"
        :columns="detailFileGridColumns"
        :rows="detailFileGridRows"
        row-key="id"
        storage-key="submission-detail-files"
        group-by="preview_status"
        :height="300"
        :row-height="gridRowHeight"
      >
        <template #cell="{ row, column, value }">
          <label v-if="column.key === 'selected'" class="aw-inline-check">
            <input
              type="checkbox"
              :checked="gridRowAsFile(row).selected"
              @change="toggleFile(gridRowAsFile(row), ($event.target as HTMLInputElement).checked)"
            />
            <span>{{ gridRowAsFile(row).id }}</span>
          </label>
          <button v-else-if="column.key === 'action'" type="button" @click="downloadFile(gridRowAsFile(row))">下载</button>
          <span v-else-if="column.key === 'file_type'">{{ gridRowAsFile(row).file_type || gridRowAsFile(row).mime_type }}</span>
          <span
            v-else-if="column.key === 'preview_status'"
            :class="chipClass(previewStatusMeta(gridRowAsFile(row).preview_status).tone)"
          >
            {{ previewStatusMeta(gridRowAsFile(row).preview_status).label }}
          </span>
          <span v-else>{{ value || '—' }}</span>
        </template>
      </WorkbenchDataGrid>
      <div v-else class="aw-empty-state">
        <h3>没有文件</h3>
      </div>
    </div>
  </section>
</template>
