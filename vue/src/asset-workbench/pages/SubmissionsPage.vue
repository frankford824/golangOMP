<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { X } from 'lucide-vue-next'
import { useRouter } from 'vue-router'

import {
  assetWorkbenchApi,
  type AssetWorkbenchEventRow,
  type AssetWorkbenchSavedView,
  type DifficultyClassRow,
  type FilePreviewMeta,
  type SubmissionDetail,
  type SubmissionFileRow,
  type SubmissionItemRow,
  type SubmissionRow,
  type UploadDirectoryRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import { exportQCImportTemplateWorkbook } from '@aw/features/export/settlementExport'
import { buildTimestampedZipFilename, downloadBatchAsZip } from '@/utils/batchZipDownload'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { difficultyCodes, firstDifficultyCode } from '@aw/shared/format/difficulty'
import { formatInt, formatMoney } from '@aw/shared/format/number'
import { chipClass, previewStatusMeta, pricingStatusMeta, qcStatusMeta, submissionStatusMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import { useScrollLock } from '@aw/shared/motion/useScrollLock'
import WorkbenchFilePreview from '@aw/shared/preview/WorkbenchFilePreview.vue'
import WorkbenchPreviewDialog from '@aw/shared/preview/WorkbenchPreviewDialog.vue'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'

type ItemActionKind = 'needs_fix' | 'void' | 'reprice'
type SubmissionOrderBy = 'submitted_at' | 'file_type' | 'file_name'
type SubmissionOrderDir = 'desc' | 'asc'

interface GridColumn {
  key: string
  label: string
  width: number
  align?: 'left' | 'right' | 'center'
}

type DetailItemGridRow = SubmissionItemRow & { file_count: number; action: string }
type DetailFileGridRow = SubmissionFileRow & { selected: boolean; action: string; page_count: number; preview_tile: string }
type SubmissionGridRow = SubmissionRow & { status_label: string; submitter_label: string; submitted_label: string }
type QCRejectionMessage = { item_id: number; order_no: string; reason: string; created_at: string; actor_label: string }
type FilePreviewPayload = {
  title: string
  previewUrl: string
  meta: FilePreviewMeta | null
  statusText: string
  sourceLabel?: string
  expiresAt?: string
}

interface PendingItemAction {
  kind: ItemActionKind
  item: SubmissionItemRow
  reason: string
}

interface PendingSubmissionVoid {
  submission: SubmissionRow
  reason: string
}

const rows = ref<SubmissionRow[]>([])
const total = ref(0)
const savedViews = ref<AssetWorkbenchSavedView[]>([])
const uploadDirectories = ref<UploadDirectoryRow[]>([])
const difficultyRows = ref<DifficultyClassRow[]>([])
const selectedDetail = ref<SubmissionDetail | null>(null)
const selectedFileIds = ref<Set<number>>(new Set())
const qcRejectionMessages = ref<QCRejectionMessage[]>([])
const pendingAction = ref<PendingItemAction | null>(null)
const pendingSubmissionVoid = ref<PendingSubmissionVoid | null>(null)
const editingItem = ref<SubmissionItemRow | null>(null)
const qcInputRef = ref<HTMLInputElement | null>(null)
const detailLoading = ref(false)
const qcRejectionLoading = ref(false)
const fileActionSaving = ref(false)
const notice = ref('')
const viewName = ref('默认维护视图')
const groupBy = ref('business_month')
const density = ref('compact')
const orderBy = ref<SubmissionOrderBy>('submitted_at')
const orderDir = ref<SubmissionOrderDir>('desc')
const moveDirectoryId = ref(0)
const deleteReason = ref('')
const previewDialog = ref<{
  open: boolean
  title: string
  previewUrl: string
  fallbackSrc: string
  emptyLabel: string
  metaRows: Array<[string, string]>
}>({
  open: false,
  title: '',
  previewUrl: '',
  fallbackSrc: '',
  emptyLabel: '',
  metaRows: [],
})
const editForm = ref({
  order_no: '',
  difficulty_class: '',
  finalized: true,
  page_count: 1,
  reason: '',
})
const { bootstrap, refresh: refreshBootstrap } = useAssetWorkbenchBootstrap()
const router = useRouter()
const { lock: lockDetailDialog, unlock: unlockDetailDialog } = useScrollLock('aw-submission-dialog-locked')
const isSimpleUser = computed(() => bootstrap.value?.is_admin === false)
const submissionsRequest = usePageRequest(
  () => assetWorkbenchApi.listSubmissions({ page: 1, page_size: 50, order_by: orderBy.value, order_dir: orderDir.value }),
  { items: [], total: 0 },
  '提交列表加载失败',
)
const loading = submissionsRequest.loading
const error = submissionsRequest.error
const difficultyOptions = computed(() => difficultyCodes(difficultyRows.value))

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
    submitter_label: row.submitter_name || row.submitter_username || `用户 ${row.submitter_user_id}`,
    submitted_label: formatDateTime(row.submitted_at),
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
    page_count: pageCountForFile(file),
    preview_tile: 'preview',
    action: 'actions',
  })),
)
const detailFileGridRows = computed(() => detailFileRows.value as unknown as Record<string, unknown>[])
const detailDialogOpen = computed(() => Boolean(selectedDetail.value || detailLoading.value))
const detailDialogRef = ref<HTMLElement | null>(null)
const submissionGridColumns = computed<Array<{ key: string; label: string; width: number; align?: 'left' | 'right' | 'center' }>>(() => {
  const columns: Array<{ key: string; label: string; width: number; align?: 'left' | 'right' | 'center' }> = [
    { key: 'submission_no', label: '提交批次', width: 180 },
    { key: 'submitter_label', label: '创建人', width: 132 },
    { key: 'business_month', label: '结算月', width: 108 },
    { key: 'submitted_label', label: '创建时间', width: 168 },
    { key: 'status_label', label: '状态', width: 96 },
    { key: 'item_count', label: '单数', width: 88, align: 'right' },
    { key: 'page_count', label: '页数', width: 88, align: 'right' },
    { key: 'gross_total', label: '毛额', width: 108, align: 'right' },
    { key: 'action', label: '动作', width: 96, align: 'center' },
  ]
  return isSimpleUser.value ? columns.filter((column) => column.key !== 'submitter_label') : columns
})
const detailItemGridColumns = computed<GridColumn[]>(() => [
  { key: 'order_no', label: '订单号', width: 150 },
  { key: 'difficulty_class', label: '难度', width: 108 },
  { key: 'pricing_status', label: '计价', width: 96 },
  { key: 'qc_status', label: '质检', width: 96 },
  { key: 'gross_amount', label: '毛额', width: 108, align: 'right' },
  { key: 'file_count', label: '文件', width: 84, align: 'right' },
  { key: 'action', label: '动作', width: 360, align: 'center' },
])
const detailFileGridColumns = computed<GridColumn[]>(() => [
  { key: 'selected', label: '选择', width: 84, align: 'center' },
  { key: 'preview_tile', label: '缩略图', width: 112, align: 'center' },
  { key: 'original_filename', label: '文件名', width: 240 },
  { key: 'file_type', label: '类型', width: 108 },
  { key: 'page_count', label: '页数', width: 84, align: 'right' },
  { key: 'preview_status', label: '预览', width: 108 },
  { key: 'action', label: '动作', width: 96, align: 'center' },
])
const detailFileRowHeight = 64
const detailFileGroupHeight = 34
const detailFileGridHeight = computed(() => {
  const visibleFileRows = Math.min(Math.max(selectedFiles.value.length, 2), 3)
  return detailFileGroupHeight + visibleFileRows * (detailFileRowHeight + 1)
})

const downloadableFileIDs = computed(() => selectedFiles.value.map((file) => file.id))
const selectedFileCount = computed(() => selectedFileIds.value.size)
const downloadTargetFileIDs = computed(() => {
  const selected = Array.from(selectedFileIds.value)
  return selected.length ? selected : downloadableFileIDs.value
})
const downloadButtonLabel = computed(() => {
  const count = downloadTargetFileIDs.value.length
  if (!count) return '批量下载'
  return selectedFileCount.value ? `下载已选 ${formatInt(count)} 个` : `下载全部 ${formatInt(count)} 个`
})
const detailSelectionLabel = computed(() => {
  const totalFiles = downloadableFileIDs.value.length
  if (!totalFiles) return '暂无文件'
  return `已选 ${formatInt(selectedFileCount.value)} / 共 ${formatInt(totalFiles)} 个文件`
})
const selectedPageCount = computed(() => {
  const selectedItemIds = new Set<number>()
  for (const file of selectedFiles.value) {
    if (selectedFileIds.value.has(file.id)) selectedItemIds.add(file.submission_item_id)
  }
  let total = 0
  const detail = selectedDetail.value
  if (!detail) return 0
  for (const entry of detail.items) {
    if (selectedItemIds.has(entry.item.id)) total += entry.item.page_count || 0
  }
  return total
})
const needsFixItems = computed(() =>
  (selectedDetail.value?.items ?? [])
    .map((entry) => entry.item)
    .filter((item) => item.qc_status === 'needs_fix'),
)
const qcRejectionRows = computed(() => {
  const byID = new Map(qcRejectionMessages.value.map((item) => [item.item_id, item]))
  return needsFixItems.value.map((item) => byID.get(item.id) ?? {
    item_id: item.id,
    order_no: item.order_no || `明细 ${item.id}`,
    reason: qcRejectionLoading.value ? '正在读取驳回原因' : '未记录驳回原因',
    created_at: '',
    actor_label: '',
  })
})
const pendingActionTitle = computed(() => {
  if (!pendingAction.value) return ''
  if (pendingAction.value.kind === 'needs_fix') return '标记需修'
  if (pendingAction.value.kind === 'void') return '作废明细'
  return '重新计价'
})
const canManageItems = computed(() => {
  const capabilities = bootstrap.value?.capabilities ?? []
  return capabilities.includes('asset.workbench.manage') || capabilities.includes('asset.workbench.settlement')
})
const canManageFiles = computed(() => bootstrap.value?.capabilities.includes('asset.workbench.manage') === true)
const pageEyebrow = computed(() => (isSimpleUser.value ? '我的记录' : '交付维护'))
const pageTitle = computed(() => (isSimpleUser.value ? '我的上传记录' : '提交批次维护'))
const pageDescription = computed(() =>
  isSimpleUser.value
    ? '这里显示已经生成提交记录的文件。只上传文件但没有提交时，不会进入记录。'
    : '按批次核对作品、预览图片、维护质检状态和下载文件；出错扣减在结算页导入。',
)
const emptyTitle = computed(() => (isSimpleUser.value ? '还没有上传记录' : '还没有提交明细'))
const emptyDescription = computed(() =>
  isSimpleUser.value
    ? '点“交作品”并完成提交后，这里会显示你的文件、处理状态和下载入口。'
    : '当前没有可维护的提交。上传成品并生成提交记录后，可以在这里质检、修正、下载和保存常用视图。',
)
const detailDescription = computed(() =>
  isSimpleUser.value
    ? '查看本次提交的明细、处理状态、文件预览和下载入口。'
    : '本窗口只维护作品状态和文件；质检出错扣款请到结算页导入出错扣减表。',
)
let detailRequestSerial = 0

async function loadSubmissions() {
  const result = await submissionsRequest.run()
  rows.value = result?.items ?? []
  total.value = result?.total ?? 0
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

function pageCountForFile(file: SubmissionFileRow) {
  const detail = selectedDetail.value
  if (!detail) return 0
  return detail.items.find((entry) => entry.item.id === file.submission_item_id)?.item.page_count ?? 0
}

async function loadSavedViews() {
  if (isSimpleUser.value) {
    savedViews.value = []
    return
  }
  try {
    savedViews.value = await assetWorkbenchApi.listSavedViews('submissions')
  } catch {
    savedViews.value = []
  }
}

async function loadUploadDirectories() {
  if (isSimpleUser.value) {
    uploadDirectories.value = []
    return
  }
  try {
    uploadDirectories.value = await assetWorkbenchApi.listUploadDirectoriesAdmin()
    if (!moveDirectoryId.value && uploadDirectories.value[0]) moveDirectoryId.value = uploadDirectories.value[0].id
  } catch {
    uploadDirectories.value = []
  }
}

async function loadDifficultyClasses() {
  try {
    difficultyRows.value = await assetWorkbenchApi.listDifficultyClasses()
  } catch {
    difficultyRows.value = []
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
  const requestSerial = ++detailRequestSerial
  detailLoading.value = true
  error.value = ''
  notice.value = ''
  selectedFileIds.value = new Set()
  qcRejectionMessages.value = []
  try {
    const detail = await assetWorkbenchApi.getSubmissionDetail(submissionId)
    if (requestSerial !== detailRequestSerial) return
    selectedDetail.value = detail
    await loadQCRejectionMessages()
  } catch (err) {
    if (requestSerial !== detailRequestSerial) return
    error.value = err instanceof Error ? err.message : '提交详情加载失败'
  } finally {
    if (requestSerial === detailRequestSerial) detailLoading.value = false
  }
}

function closeSubmissionDetail() {
  detailRequestSerial += 1
  detailLoading.value = false
  selectedDetail.value = null
  selectedFileIds.value = new Set()
  qcRejectionMessages.value = []
  pendingAction.value = null
  editingItem.value = null
  previewDialog.value.open = false
}

function dismissSubmissionDetailDialog() {
  if (previewDialog.value.open) {
    closeFilePreview()
    return
  }
  closeSubmissionDetail()
}

async function loadQCRejectionMessages() {
  const items = needsFixItems.value
  if (!items.length) {
    qcRejectionMessages.value = []
    return
  }
  qcRejectionLoading.value = true
  const messages = await Promise.all(items.map(loadQCRejectionMessage))
  qcRejectionMessages.value = messages
  qcRejectionLoading.value = false
}

async function loadQCRejectionMessage(item: SubmissionItemRow): Promise<QCRejectionMessage> {
  try {
    const events = await assetWorkbenchApi.listEvents({
      event_type: 'item.qc_updated',
      entity_type: 'submission_item',
      entity_id: item.id,
      page: 1,
      page_size: 5,
    })
    const event = events.items.find((entry) => entry.reason?.trim()) ?? events.items[0]
    return rejectionMessageFromEvent(item, event)
  } catch {
    return {
      item_id: item.id,
      order_no: item.order_no || `明细 ${item.id}`,
      reason: '驳回原因读取失败',
      created_at: '',
      actor_label: '',
    }
  }
}

function rejectionMessageFromEvent(item: SubmissionItemRow, event?: AssetWorkbenchEventRow): QCRejectionMessage {
  return {
    item_id: item.id,
    order_no: item.order_no || `明细 ${item.id}`,
    reason: event?.reason?.trim() || '未记录驳回原因',
    created_at: event?.created_at ?? '',
    actor_label: event?.actor_display_name || event?.actor_username || (event?.actor_user_id ? `用户 ${event.actor_user_id}` : ''),
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

function itemNeedsGrade(item: SubmissionItemRow) {
  return item.pricing_status === 'pending_grade'
}

async function goToProfileGrading(item: SubmissionItemRow) {
  if (!item.payee_user_id) return
  await router.push({ path: '/settings/people', query: { user_id: String(item.payee_user_id) } })
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
  const ids = [...downloadTargetFileIDs.value]
  if (!ids.length) {
    notice.value = '本批次没有可下载文件'
    return
  }
  notice.value = selectedFileCount.value ? '正在生成已选文件下载包' : '正在生成全部文件下载包'
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
    const message = err instanceof Error ? err.message : '批量下载失败'
    error.value = `${message}；也可以先点单个文件的“下载”链接。`
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

function openQCImport() {
  if (!selectedDetail.value) {
    notice.value = '请先打开一个提交批次'
    return
  }
  qcInputRef.value?.click()
}

async function downloadQCTemplate() {
  notice.value = ''
  error.value = ''
  try {
    await exportQCImportTemplateWorkbook()
    notice.value = '状态导入表已生成'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '状态导入表生成失败'
  }
}

async function handleQCImport(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  target.value = ''
  if (!file) return
  const detail = selectedDetail.value
  if (!detail) {
    error.value = '请先打开一个提交批次'
    return
  }
  notice.value = ''
  error.value = ''
  try {
    const result = await assetWorkbenchApi.importSubmissionItemQCExcel(detail.submission.business_month, file)
    const updated = result.updated?.length ?? 0
    const failed = result.failures?.length ?? 0
    notice.value = `状态表已导入：成功 ${formatInt(updated)} 行，失败 ${formatInt(failed)} 行`
    if (failed > 0) {
      error.value = result.failures.slice(0, 5).map((item) => `第 ${item.row} 行：${item.reason}`).join('；')
    }
    await refreshSelectedDetail()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '质检 Excel 导入失败'
  }
}

function startItemAction(item: SubmissionItemRow, kind: ItemActionKind) {
  pendingAction.value = { item, kind, reason: '' }
}

function startSubmissionVoid(submission: SubmissionRow) {
  pendingSubmissionVoid.value = { submission, reason: '' }
}

async function executeSubmissionVoid() {
  const action = pendingSubmissionVoid.value
  if (!action) return
  const reason = action.reason.trim()
  if (!reason) {
    error.value = '请填写作废原因'
    return
  }
  notice.value = ''
  error.value = ''
  try {
    const updated = await assetWorkbenchApi.voidSubmission(action.submission.id, reason)
    notice.value = `已作废提交批次：${updated.submission_no}`
    if (selectedDetail.value?.submission.id === updated.id) {
      selectedDetail.value = null
      selectedFileIds.value = new Set()
      qcRejectionMessages.value = []
    }
    pendingSubmissionVoid.value = null
    await loadSubmissions()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '提交批次作废失败'
  }
}

function startEditItem(item: SubmissionItemRow) {
  editingItem.value = item
  editForm.value = {
    order_no: item.order_no,
    difficulty_class: item.difficulty_class || firstDifficultyCode(difficultyRows.value),
    finalized: item.finalized,
    page_count: item.page_count || 1,
    reason: '',
  }
}

function openFilePreview(payload: FilePreviewPayload) {
  previewDialog.value = {
    open: true,
    title: payload.title,
    previewUrl: payload.previewUrl,
    fallbackSrc: payload.previewUrl,
    emptyLabel: payload.statusText,
    metaRows: [
      ['展示来源', payload.sourceLabel || (payload.previewUrl ? '预览图' : '—')],
      ['预览状态', payload.meta?.status || (payload.previewUrl ? 'ready' : '等待生成')],
      ['过期时间', payload.expiresAt || payload.meta?.expires_at || '—'],
    ],
  }
}

function filePreviewStatusText(meta: FilePreviewMeta | null) {
  if (meta?.preparing) return '预览生成中'
  if (meta?.status === 'failed') return meta.error || '预览失败'
  if (meta?.status === 'not_applicable') return '不支持预览'
  return meta?.preview_url ? '' : '暂无可展示预览'
}

function fileCanUseOriginalPreview(file: SubmissionFileRow) {
  const mime = (file.mime_type || '').toLowerCase()
  if (mime.startsWith('image/')) return true
  const name = (file.original_filename || '').toLowerCase()
  return /\.(jpe?g|png|webp|gif|bmp|svg)$/i.test(name)
}

async function openFilePreviewFromRow(file: SubmissionFileRow) {
  notice.value = ''
  error.value = ''
  const title = file.original_filename || `文件 ${file.id}`
  openFilePreview({
    title,
    previewUrl: '',
    meta: null,
    statusText: '正在加载预览…',
    sourceLabel: '加载中',
  })
  let previewError = ''
  try {
    const meta = await assetWorkbenchApi.getFilePreview(file.id)
    if (meta.preview_url) {
      openFilePreview({
        title,
        previewUrl: meta.preview_url,
        meta,
        statusText: filePreviewStatusText(meta),
        sourceLabel: '预览图',
      })
      return
    }
    previewError = filePreviewStatusText(meta)
  } catch (err) {
    previewError = err instanceof Error ? err.message : '预览加载失败'
  }

  if (fileCanUseOriginalPreview(file)) {
    try {
      const download = await assetWorkbenchApi.getFileDownload(file.id)
      openFilePreview({
        title,
        previewUrl: download.download_url,
        meta: null,
        statusText: '',
        sourceLabel: '原文件预览',
        expiresAt: download.expires_at,
      })
      return
    } catch (err) {
      const fallbackError = err instanceof Error ? err.message : '原文件预览链接生成失败'
      previewError = previewError ? `${previewError}；${fallbackError}` : fallbackError
    }
  }

  openFilePreview({
    title,
    previewUrl: '',
    meta: null,
    statusText: previewError || '暂无可展示预览',
    sourceLabel: '—',
  })
}

async function openFilePreviewFromTile(file: SubmissionFileRow, payload: FilePreviewPayload) {
  if (payload.previewUrl) {
    openFilePreview({
      ...payload,
      sourceLabel: payload.sourceLabel || '预览图',
    })
    return
  }
  await openFilePreviewFromRow(file)
}

function closeFilePreview() {
  previewDialog.value.open = false
}

async function saveItemEdit() {
  const item = editingItem.value
  if (!item) return
  notice.value = ''
  error.value = ''
  try {
    const updated = await assetWorkbenchApi.updateSubmissionItem(item.id, {
      order_no: editForm.value.order_no,
      difficulty_class: editForm.value.difficulty_class,
      finalized: editForm.value.finalized,
      page_count: editForm.value.page_count,
      reason: editForm.value.reason || '维护区行内编辑',
    })
    notice.value = `已更新 ${updated.order_no}`
    editingItem.value = null
    await refreshSelectedDetail()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '明细保存失败'
  }
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
      notice.value = `已重计价 ${action.item.order_no}：${pricingStatusMeta(updated.pricing_status).label}`
    }
    pendingAction.value = null
    await refreshSelectedDetail()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '明细操作失败'
  }
}

async function moveSelectedFiles() {
  const ids = Array.from(selectedFileIds.value)
  if (!ids.length) {
    notice.value = '请选择要移动的文件'
    return
  }
  if (!moveDirectoryId.value) {
    error.value = '请选择目标上传目录'
    return
  }
  fileActionSaving.value = true
  notice.value = ''
  error.value = ''
  try {
    const result = await assetWorkbenchApi.batchMoveFiles(ids, moveDirectoryId.value, '维护区批量移动文件')
    notice.value = `已移动 ${formatInt(result.files?.length ?? 0)} 个文件，失败 ${formatInt(result.failures?.length ?? 0)} 个`
    selectedFileIds.value = new Set()
    await refreshSelectedDetail()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '批量移动失败'
  } finally {
    fileActionSaving.value = false
  }
}

async function deleteSelectedFiles() {
  const ids = Array.from(selectedFileIds.value)
  if (!ids.length) {
    notice.value = '请选择要删除的文件'
    return
  }
  if (!deleteReason.value.trim()) {
    error.value = '删除文件必须填写原因'
    return
  }
  fileActionSaving.value = true
  notice.value = ''
  error.value = ''
  try {
    const result = await assetWorkbenchApi.batchDeleteFiles(ids, deleteReason.value.trim())
    notice.value = `已删除 ${formatInt(result.deleted?.length ?? 0)} 个文件，失败 ${formatInt(result.failures?.length ?? 0)} 个`
    selectedFileIds.value = new Set()
    deleteReason.value = ''
    await refreshSelectedDetail()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '批量删除失败'
  } finally {
    fileActionSaving.value = false
  }
}

onMounted(async () => {
  await refreshBootstrap()
  await Promise.all([loadSubmissions(), loadSavedViews(), loadUploadDirectories(), loadDifficultyClasses()])
})

onBeforeUnmount(() => {
  unlockDetailDialog()
})

watch(
  detailDialogOpen,
  async (open) => {
    if (open) {
      lockDetailDialog()
      await nextTick()
      detailDialogRef.value?.focus()
    } else {
      unlockDetailDialog()
    }
  },
  { immediate: true },
)
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">{{ pageEyebrow }}</p>
        <h2>{{ pageTitle }}</h2>
        <p>{{ pageDescription }}</p>
      </div>
      <div v-if="!isSimpleUser" class="aw-page-bar__actions">
        <button class="aw-secondary-button" type="button" @click="downloadQCTemplate">下载状态导入表</button>
        <button class="aw-primary-button" type="button" @click="saveView">保存视图</button>
      </div>
    </div>
    <input
      ref="qcInputRef"
      class="aw-visually-hidden"
      type="file"
      accept=".xlsx,.xls"
      aria-label="导入质检状态表 Excel"
      @change="handleQCImport"
    />
    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <template v-if="!isSimpleUser">
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
        </template>
        <select v-model="orderBy" aria-label="排序字段" @change="loadSubmissions()">
          <option value="submitted_at">按创建时间</option>
          <option value="file_type">按文件类型</option>
          <option value="file_name">按文件名</option>
        </select>
        <select v-model="orderDir" aria-label="排序方向" @change="loadSubmissions()">
          <option value="desc">倒序</option>
          <option value="asc">正序</option>
        </select>
        <span>{{ formatInt(total) }} 个批次</span>
      </div>
      <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
      <div v-if="!isSimpleUser && savedViews.length" class="aw-button-row">
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
      <AsyncBoundary
        :loading="loading"
        :error="error"
        loading-label="正在加载提交列表"
        @retry="loadSubmissions"
      >
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
            <div v-if="column.key === 'action'" class="aw-inline-actions aw-inline-actions--compact">
              <button class="aw-grid-button" type="button" @click="openSubmission(gridRowAsSubmission(row))">
                {{ isSimpleUser ? '查看' : '文件' }}
              </button>
              <button
                v-if="canManageItems && gridRowAsSubmission(row).status !== 'voided'"
                class="aw-grid-button"
                type="button"
                @click="startSubmissionVoid(gridRowAsSubmission(row))"
              >
                作废
              </button>
            </div>
            <span
              v-else-if="column.key === 'status_label'"
              :class="chipClass(submissionStatusMeta(gridRowAsSubmission(row).status).tone)"
            >
              {{ value }}
            </span>
            <span v-else-if="column.key === 'submitter_label'" class="aw-cell-text">{{ value }}</span>
            <span v-else-if="column.key === 'gross_total'" class="aw-cell-money">{{ formatMoney(value) }}</span>
            <span v-else-if="column.align === 'right'" class="aw-cell-num">{{ formatInt(value) }}</span>
            <span v-else>{{ value || '—' }}</span>
          </template>
        </WorkbenchDataGrid>
        <div v-else class="aw-empty-state">
          <h3>{{ emptyTitle }}</h3>
          <p>{{ emptyDescription }}</p>
        </div>
      </AsyncBoundary>
      <div v-if="pendingSubmissionVoid" class="aw-dialog-backdrop" role="presentation" @click.self="pendingSubmissionVoid = null">
        <section class="aw-confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="submission-void-title">
        <div class="aw-panel__head">
          <div>
            <h3 id="submission-void-title">作废提交批次</h3>
            <p class="aw-copy">{{ pendingSubmissionVoid.submission.submission_no }}</p>
          </div>
          <span :class="chipClass(submissionStatusMeta(pendingSubmissionVoid.submission.status).tone)">
            {{ submissionStatusMeta(pendingSubmissionVoid.submission.status).label }}
          </span>
        </div>
        <label class="aw-field">
          <span>作废原因</span>
          <input v-model.trim="pendingSubmissionVoid.reason" />
        </label>
        <div class="aw-inline-actions">
          <button class="aw-primary-button" type="button" @click="executeSubmissionVoid">确认作废</button>
          <button class="aw-secondary-button" type="button" @click="pendingSubmissionVoid = null">取消</button>
        </div>
        </section>
      </div>
    </div>

    <Teleport to="body">
      <div v-if="detailDialogOpen" class="aw-token-scope aw-submission-dialog-layer">
        <div class="aw-submission-dialog__backdrop" role="presentation" @click="dismissSubmissionDetailDialog" />
        <section
          ref="detailDialogRef"
          class="aw-submission-dialog"
          role="dialog"
          aria-modal="true"
          aria-labelledby="submission-detail-title"
          aria-describedby="submission-detail-description"
          tabindex="-1"
        >
                <div class="aw-page-bar">
                  <div class="aw-page-bar__copy">
                    <p class="aw-eyebrow">批次明细</p>
                    <h2 id="submission-detail-title">{{ selectedDetail?.submission.submission_no || '批次文件与状态' }}</h2>
                    <p id="submission-detail-description">{{ detailDescription }}</p>
                  </div>
                  <div class="aw-page-bar__actions">
                    <span class="aw-chip aw-chip--neutral">{{ detailSelectionLabel }}</span>
                    <span v-if="selectedPageCount" class="aw-chip aw-chip--neutral">已选页数 {{ formatInt(selectedPageCount) }}</span>
                    <button class="aw-secondary-button" type="button" :disabled="!downloadableFileIDs.length" @click="downloadSelectedFiles">
                      {{ downloadButtonLabel }}
                    </button>
                    <button v-if="!isSimpleUser" class="aw-secondary-button" type="button" :disabled="!selectedDetail" @click="openQCImport">导入状态表</button>
                    <label class="aw-inline-check">
                      <input
                        type="checkbox"
                        :checked="selectedFileIds.size > 0 && selectedFileIds.size === downloadableFileIDs.length"
                        @change="toggleAllFiles(($event.target as HTMLInputElement).checked)"
                      />
                      <span>全选</span>
                    </label>
                    <button class="aw-secondary-button" type="button" @click="closeSubmissionDetail">
                      <X :size="16" aria-hidden="true" />
                      关闭
                    </button>
                  </div>
                </div>
                <div class="aw-submission-dialog__body">
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
          <div v-if="column.key === 'action'" class="aw-inline-actions aw-inline-actions--compact">
            <template v-if="canManageItems">
              <button
                v-if="itemNeedsGrade(gridRowAsItem(row))"
                class="aw-grid-button aw-grid-button--strong"
                type="button"
                @click="goToProfileGrading(gridRowAsItem(row))"
              >
                去定级
              </button>
              <button class="aw-grid-button" type="button" @click="startEditItem(gridRowAsItem(row))">编辑</button>
              <button class="aw-grid-button" type="button" @click="updateItemQC(gridRowAsItem(row), 'checked')">通过</button>
              <button class="aw-grid-button" type="button" @click="startItemAction(gridRowAsItem(row), 'needs_fix')">需修</button>
              <button class="aw-grid-button" type="button" @click="startItemAction(gridRowAsItem(row), 'reprice')">重计价</button>
              <button class="aw-grid-button" type="button" @click="startItemAction(gridRowAsItem(row), 'void')">作废</button>
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
      <div v-if="selectedFiles.length" class="aw-panel aw-file-maintenance-panel">
        <div class="aw-panel__head">
          <div>
            <h3>{{ canManageFiles ? '文件批量维护' : '文件列表' }}</h3>
            <p class="aw-copy">
              {{
                canManageFiles
                  ? '这里负责移动目录和删除文件；下载按钮未勾选时默认下载本批次全部文件。'
                  : '这里查看本批次文件、预览状态和下载入口。'
              }}
            </p>
          </div>
          <span class="aw-chip aw-chip--neutral">{{ detailSelectionLabel }}</span>
        </div>
        <div v-if="canManageFiles" class="aw-batch-maintenance-form">
          <div class="aw-batch-maintenance-group">
            <label class="aw-field">
              <span>移动到</span>
              <select v-model.number="moveDirectoryId" :disabled="!uploadDirectories.length">
                <option :value="0">选择上传目录</option>
                <option v-for="directory in uploadDirectories" :key="directory.id" :value="directory.id">
                  {{ directory.name }}
                </option>
              </select>
            </label>
            <button
              class="aw-secondary-button"
              type="button"
              :disabled="fileActionSaving || selectedFileIds.size === 0 || !moveDirectoryId"
              @click="moveSelectedFiles"
            >
              移动所选
            </button>
          </div>
          <div class="aw-batch-maintenance-group">
            <label class="aw-field">
              <span>删除原因</span>
              <input v-model.trim="deleteReason" placeholder="删除文件必须填写原因" />
            </label>
            <button
              class="aw-secondary-button"
              type="button"
              :disabled="fileActionSaving || selectedFileIds.size === 0 || !deleteReason.trim()"
              @click="deleteSelectedFiles"
            >
              删除所选
            </button>
          </div>
        </div>
        <WorkbenchDataGrid
          :columns="detailFileGridColumns"
          :rows="detailFileGridRows"
          row-key="id"
          storage-key="submission-detail-files"
          group-by="preview_status"
          :height="detailFileGridHeight"
          :row-height="detailFileRowHeight"
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
            <button
              v-else-if="column.key === 'action'"
              class="aw-grid-button"
              type="button"
              @click="downloadFile(gridRowAsFile(row))"
            >
              下载
            </button>
            <WorkbenchFilePreview
              v-else-if="column.key === 'preview_tile'"
              class="aw-preview-tile--table"
              :file-id="gridRowAsFile(row).id"
              :alt="gridRowAsFile(row).original_filename"
              :defer-until-visible="false"
              @preview="openFilePreviewFromTile(gridRowAsFile(row), $event)"
            />
            <button
              v-else-if="column.key === 'original_filename'"
              class="aw-link-button aw-file-name-button"
              type="button"
              @click="openFilePreviewFromRow(gridRowAsFile(row))"
            >
              {{ gridRowAsFile(row).original_filename || `文件 ${gridRowAsFile(row).id}` }}
            </button>
            <span v-else-if="column.key === 'file_type'">{{ gridRowAsFile(row).file_type || gridRowAsFile(row).mime_type }}</span>
            <span v-else-if="column.key === 'page_count'" class="aw-cell-num">{{ formatInt(value) }}</span>
            <button
              v-else-if="column.key === 'preview_status'"
              class="aw-preview-status-button"
              :class="chipClass(previewStatusMeta(gridRowAsFile(row).preview_status).tone)"
              type="button"
              @click="openFilePreviewFromRow(gridRowAsFile(row))"
            >
              {{ previewStatusMeta(gridRowAsFile(row).preview_status).label }}
            </button>
            <span v-else>{{ value || '—' }}</span>
          </template>
        </WorkbenchDataGrid>
      </div>
      <div v-else-if="selectedDetail && !detailLoading" class="aw-empty-state">
        <h3>没有文件</h3>
      </div>
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
      <div v-if="needsFixItems.length" class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <h3>质检驳回列表</h3>
            <p class="aw-copy">当前批次中标记为需修的明细和最近驳回原因。</p>
          </div>
          <span class="aw-chip aw-chip--warn">{{ formatInt(needsFixItems.length) }} 条</span>
        </div>
        <div class="aw-compact-list">
          <div v-for="item in qcRejectionRows" :key="item.item_id" class="aw-compact-list__item">
            <div>
              <strong>{{ item.order_no }}</strong>
              <p class="aw-copy">{{ item.reason }}</p>
            </div>
            <span class="aw-chip aw-chip--warn">
              {{ item.actor_label || '质检' }}{{ item.created_at ? ` · ${formatDateTime(item.created_at)}` : '' }}
            </span>
          </div>
        </div>
      </div>
      <div v-if="editingItem" class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <h3>编辑明细</h3>
            <p class="aw-copy">{{ editingItem.order_no }}</p>
          </div>
          <span :class="chipClass(pricingStatusMeta(editingItem.pricing_status).tone)">
            {{ pricingStatusMeta(editingItem.pricing_status).label }}
          </span>
        </div>
        <div class="aw-grid-toolbar">
          <label class="aw-field">
            <span>订单号</span>
            <input v-model.trim="editForm.order_no" />
          </label>
          <label class="aw-field">
            <span>难度</span>
            <select v-model="editForm.difficulty_class">
              <option v-for="difficulty in difficultyOptions" :key="difficulty" :value="difficulty">
                {{ difficulty }}
              </option>
            </select>
          </label>
          <label class="aw-field">
            <span>页数</span>
            <input v-model.number="editForm.page_count" min="1" type="number" />
          </label>
          <label class="aw-inline-check">
            <input v-model="editForm.finalized" type="checkbox" />
            <span>已定稿</span>
          </label>
          <label class="aw-field">
            <span>原因</span>
            <input v-model.trim="editForm.reason" placeholder="默认记录为维护区行内编辑" />
          </label>
        </div>
        <div class="aw-inline-actions">
          <button class="aw-primary-button" type="button" @click="saveItemEdit">保存明细</button>
          <button class="aw-secondary-button" type="button" @click="editingItem = null">取消</button>
        </div>
      </div>
      <WorkbenchPreviewDialog
        :open="previewDialog.open"
        :title="previewDialog.title"
        :preview-url="previewDialog.previewUrl"
        :fallback-src="previewDialog.fallbackSrc"
        :empty-label="previewDialog.emptyLabel"
        :meta-rows="previewDialog.metaRows"
        eyebrow="文件预览"
        @close="closeFilePreview"
      />
                </div>
              </section>
      </div>
    </Teleport>
  </section>
</template>
