<script setup lang="ts">
import { computed, onActivated, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { RouterLink } from 'vue-router'
import { CheckCircle2, ChevronDown, ChevronUp, FileUp, LoaderCircle, Table2, XCircle } from 'lucide-vue-next'

import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import { uploadWorkbenchFile } from '@aw/features/upload/uploadFlow'
import { assetWorkbenchApi, type DifficultyClassRow, type FilePreviewMeta, type SubmissionFileRow, type UploadDirectoryRow } from '@aw/shared/api/assetWorkbenchApi'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import {
  driveUploadRelativePath,
  filesFromDriveDrop,
  groupDriveUploadPieceworkItems,
  isSafeDriveUploadPath,
  resolveDriveUploadFailureMessage,
  runDriveUploadPool,
} from '@aw/shared/drive/useDriveUpload'
import { useUploadCenterStore, type UploadCenterItem, type UploadCenterStatus } from '@aw/shared/drive/uploadCenter.store'
import { currentBusinessMonth } from '@aw/shared/format/businessMonth'
import { difficultyCodes, firstDifficultyCode } from '@aw/shared/format/difficulty'
import { formatFileSize, formatInt } from '@aw/shared/format/number'
import WorkbenchFilePreview from '@aw/shared/preview/WorkbenchFilePreview.vue'
import WorkbenchPreviewDialog from '@aw/shared/preview/WorkbenchPreviewDialog.vue'
import SpreadsheetWorkbench from '@aw/shared/spreadsheet/SpreadsheetWorkbench.vue'
import type {
  WorkbenchSpreadsheetActionPayload,
  WorkbenchSpreadsheetSource,
  WorkbenchSpreadsheetValidation,
} from '@aw/shared/spreadsheet/types'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'
import { parseApiErrorPayload, resolveApiUserMessage } from '@/utils/api-message-zh'

defineOptions({ name: 'AssetUploadPage' })

type QueueItem = UploadCenterItem
type QueueStatus = UploadCenterStatus

interface UploadContext {
  directories: UploadDirectoryRow[]
  difficulties: DifficultyClassRow[]
}

interface UploadBatchResult {
  total: number
  success: number
  failed: number
}

const uploadCenter = useUploadCenterStore()
const { uploadPageItems: queue } = storeToRefs(uploadCenter)
const inputRef = ref<HTMLInputElement | null>(null)
const folderInputRef = ref<HTMLInputElement | null>(null)
const uploading = ref(false)
const submitting = ref(false)
const error = ref('')
const notice = ref('')
const submittedFiles = ref<SubmissionFileRow[]>([])
const uploadDirectories = ref<UploadDirectoryRow[]>([])
const difficultyRows = ref<DifficultyClassRow[]>([])
const selectedUploadDirectoryId = ref(0)
const lastUploadResult = ref<UploadBatchResult | null>(null)
const lastSubmissionResult = ref<UploadBatchResult | null>(null)
const expandedItemIds = ref<Set<string>>(new Set())
const draggingFiles = ref(false)
const uploadSpreadsheetOpen = ref(false)
const pageBusinessMonth = currentBusinessMonth()
const previewDialog = ref<{
  open: boolean
  title: string
  previewUrl: string
  emptyLabel: string
  metaRows: Array<[string, string]>
}>({
  open: false,
  title: '',
  previewUrl: '',
  emptyLabel: '',
  metaRows: [],
})
const { bootstrap, refresh } = useAssetWorkbenchBootstrap()
const contextRequest = usePageRequest<UploadContext>(
  async () => {
    await refresh()
    const [nextDirectories, nextDifficulties] = await Promise.all([
      assetWorkbenchApi.listUploadDirectories(),
      assetWorkbenchApi.listDifficultyClasses(),
    ])
    return { directories: nextDirectories, difficulties: nextDifficulties }
  },
  { directories: [], difficulties: [] },
  '上传目录加载失败',
)
const contextLoading = contextRequest.loading
const contextError = contextRequest.error

const uploadedItems = computed(() => queue.value.filter((item) => item.status === 'uploaded'))
const queuedItems = computed(() => queue.value.filter((item) => item.status === 'queued'))
const failedItems = computed(() => queue.value.filter((item) => item.status === 'failed'))
const uploadPieceworkGroups = computed(() => groupDriveUploadPieceworkItems(queue.value))
const uploadedPieceworkGroups = computed(() => groupDriveUploadPieceworkItems(uploadedItems.value))
const isSimpleUser = computed(() => bootstrap.value?.is_admin === false)
const canManageUploadDirectories = computed(() => bootstrap.value?.capabilities?.includes('asset.workbench.manage') === true)
const requiresUploadDirectory = computed(() => uploadDirectories.value.length > 0)
const selectedUploadDirectory = computed(() => uploadDirectories.value.find((item) => item.id === selectedUploadDirectoryId.value))
const canUseUploadDirectory = computed(() => !requiresUploadDirectory.value || selectedUploadDirectoryId.value > 0)
const hasPendingUploads = computed(() => queuedItems.value.length > 0)
const canSubmit = computed(() => {
  if (uploadedItems.value.length === 0 || uploading.value || submitting.value) return false
  return true
})
const canSimpleSubmit = computed(() => {
  if (!isSimpleUser.value) return false
  if (!queue.value.length || uploading.value || submitting.value) return false
  return canUseUploadDirectory.value && queue.value.every((item) => item.status !== 'uploading' && item.status !== 'submitting') && (queuedItems.value.length > 0 || uploadedItems.value.length > 0)
})
const canRetryFailedUploads = computed(() => !uploading.value && !submitting.value && canUseUploadDirectory.value && failedItems.value.length > 0)
const totalPieceworkPages = computed(() =>
  uploadPieceworkGroups.value.reduce((sum, group) => sum + (group.isFolder ? 1 : group.items[0]?.pageCount || 1), 0),
)
const uploadUnitLabel = computed(() => pieceworkFileLabel(uploadPieceworkGroups.value.length, queue.value.length))
const simplePieceworkConfirmation = computed(() => {
  const count = uploadPieceworkGroups.value.length
  return `${formatInt(count)} 个作品 = 计件数量`
})
const uploadedUnitLabel = computed(() => pieceworkFileLabel(uploadedPieceworkGroups.value.length, uploadedItems.value.length))
const difficultyOptions = computed(() => difficultyCodes(difficultyRows.value))
const selectedAllowedFileTypes = computed(() => normalizedAllowedFileTypes(selectedUploadDirectory.value?.allowed_file_types ?? []))
const selectedAllowedLabel = computed(() => (selectedAllowedFileTypes.value.length ? selectedAllowedFileTypes.value.join('、') : '全部格式'))
const selectedAcceptString = computed(() => {
  if (!selectedAllowedFileTypes.value.length) return ''
  return selectedAllowedFileTypes.value.map((value) => (value.includes('/') ? value : `.${value}`)).join(',')
})
const uploadStats = computed(() => {
  const total = queue.value.length
  const uploaded = queue.value.filter((item) => item.status === 'uploaded' || item.status === 'submitted').length
  const failed = queue.value.filter((item) => item.status === 'failed').length
  const uploadingCount = queue.value.filter((item) => item.status === 'uploading' || item.status === 'submitting').length
  const progressTotal = queue.value.reduce((sum, item) => {
    if (item.status === 'uploaded' || item.status === 'submitted') return sum + 100
    if (item.status === 'submitting') return sum + 96
    if (item.status === 'failed') return sum + item.progress
    if (item.status === 'uploading') return sum + item.progress
    return sum
  }, 0)
  return {
    total,
    uploaded,
    failed,
    uploading: uploadingCount,
    percent: total > 0 ? Math.round(progressTotal / total) : 0,
  }
})
const simpleSubmitLabel = computed(() => {
  if (uploading.value) return '正在上传'
  if (submitting.value) return '正在提交'
  return '提交上传'
})
const simpleSubmitHint = computed(() => {
  if (contextLoading.value) return '正在加载上传目录'
  if (!queue.value.length) return '先选择文件，或把文件拖到上传区'
  if (!canUseUploadDirectory.value) return '先选择这批文件进入的上传目录'
  if (failedItems.value.length && !queuedItems.value.length && !uploadedItems.value.length) return '失败文件不会自动重传，请点击“重试失败文件”'
  if (failedItems.value.length && queuedItems.value.length) return `将只上传新文件 ${formatInt(queuedItems.value.length)} 个，失败文件需单独重试`
  if (queue.value.some((item) => item.status === 'uploaded')) return `将提交 ${uploadedUnitLabel.value}`
  return `将上传并提交 ${uploadUnitLabel.value}`
})
const adminUploadLabel = computed(() => {
  if (uploading.value) return '正在上传'
  if (submitting.value) return '正在生成记录'
  if (uploadedItems.value.length > 0 && !hasPendingUploads.value) return `生成提交记录 ${uploadedPieceworkGroups.value.length} 个`
  if (queuedItems.value.length > 0) return `上传新文件 ${queuedItems.value.length} 个并生成记录`
  return '上传并生成记录'
})
const adminUploadHint = computed(() => {
  if (!queue.value.length) return '先选择文件，或把文件拖到上传区'
  if (!canUseUploadDirectory.value) return '先选择这批文件进入的上传目录'
  if (failedItems.value.length && !queuedItems.value.length && !uploadedItems.value.length) return '失败文件不会自动重传，请点击“重试失败文件”'
  if (failedItems.value.length && queuedItems.value.length) return `将只上传新文件 ${formatInt(queuedItems.value.length)} 个，失败文件需单独重试`
  if (uploadedItems.value.length > 0 && !hasPendingUploads.value) return `将为 ${uploadedUnitLabel.value} 生成提交记录，生成后进入上传记录`
  return `将上传并生成 ${uploadUnitLabel.value} 的提交记录`
})
const uploadContinuityHint = computed(() => {
  if (uploading.value) return '正在上传。你可以切到看收入或网盘，回到本页仍能看到进度。请不要关闭浏览器窗口。'
  if (submitting.value) return '正在生成上传记录。你可以先切到其他页面，回到本页仍能看到结果。'
  if (failedItems.value.length) return '有文件上传失败，失败项已保留，但不会自动重复上传。需要重传时请点击“重试失败文件”。'
  if (queue.value.some((item) => item.status === 'uploaded')) return '文件已上传，回到本页可以继续生成上传记录。'
  return ''
})
const submitButtonLabel = computed(() => {
  if (submitting.value) return isSimpleUser.value ? '正在交作品' : '正在创建提交'
  if (uploadedItems.value.length === 0) return '先上传文件'
  return isSimpleUser.value ? `交作品 ${uploadedPieceworkGroups.value.length} 个` : `生成提交记录 ${uploadedPieceworkGroups.value.length} 个`
})
const canUseAdminPrimaryAction = computed(() => {
  if (uploading.value || submitting.value || queue.value.length === 0) return false
  if (uploadedItems.value.length > 0 && !hasPendingUploads.value) return canSubmit.value
  return canUseUploadDirectory.value && queuedItems.value.length > 0
})
const uploadSpreadsheetValidations = computed<WorkbenchSpreadsheetValidation[]>(() =>
  queue.value.flatMap((item) => {
    const validations: WorkbenchSpreadsheetValidation[] = []
    if (!item.difficultyClass) {
      validations.push({ rowKey: item.id, columnKey: 'difficultyClass', tone: 'warn', message: `${item.file.name} 缺少难度` })
    }
    if (!Number.isFinite(item.pageCount) || item.pageCount < 1) {
      validations.push({ rowKey: item.id, columnKey: 'pageCount', tone: 'danger', message: `${item.file.name} 页数必须大于 0` })
    }
    if (item.status === 'failed') {
      validations.push({ rowKey: item.id, columnKey: 'status', tone: 'danger', message: item.error || `${item.file.name} 上传失败` })
    }
    return validations
  }),
)
const uploadSpreadsheetSource = computed<WorkbenchSpreadsheetSource>(() => ({
  id: 'asset-upload-queue-spreadsheet',
  revision: queue.value
    .map((item) => `${item.id}:${item.relativePath}:${item.status}:${item.difficultyClass}:${item.pageCount}:${item.finalized}:${item.error ?? ''}`)
    .join('|'),
  mode: 'import-review',
  title: '上传队列表格校对',
  description: '用于批量校正难度、页数和定稿状态。文件名、目录、状态和大小只读，提交仍走原上传流程。',
  readonly: false,
  actions: [
    { key: 'apply_upload_queue', label: '应用到队列', tone: 'success', disabled: queue.value.length === 0 },
    { key: 'open_file_picker', label: '继续选文件', tone: 'neutral' },
    {
      key: 'submit_upload_queue',
      label: isSimpleUser.value ? simpleSubmitLabel.value : adminUploadLabel.value,
      tone: 'success',
      disabled: isSimpleUser.value ? !canSimpleSubmit.value : !canUseAdminPrimaryAction.value,
    },
  ],
  sheets: [
    {
      id: 'upload_queue',
      name: '待上传文件',
      rowKey: 'id',
      readonly: false,
      freezeHeader: true,
      columns: [
        { key: 'id', label: 'ID', width: 140, readonly: true },
        { key: 'file_name', label: '文件名', width: 260, readonly: true },
        { key: 'directory', label: '目录', width: 180, readonly: true },
        { key: 'difficultyClass', label: '难度', width: 120 },
        { key: 'pageCount', label: '页数', width: 88, kind: 'number', align: 'right' },
        { key: 'finalized', label: '定稿', width: 88, kind: 'boolean', align: 'center' },
        { key: 'size', label: '大小', width: 110, readonly: true },
        { key: 'status', label: '状态', width: 110, kind: 'status', readonly: true },
        { key: 'error', label: '错误', width: 220, readonly: true },
      ],
      rows: queue.value.map((item) => ({
        id: item.id,
        file_name: queueItemDisplayName(item),
        directory: item.uploadDirectoryName || selectedUploadDirectory.value?.name || '默认目录',
        difficultyClass: item.difficultyClass || uploadDirectoryDifficulty(selectedUploadDirectory.value),
        pageCount: item.pageCount,
        finalized: item.finalized,
        size: formatFileSize(item.file.size),
        status: statusLabel(item.status),
        error: item.error ?? '',
      })),
      validations: uploadSpreadsheetValidations.value,
    },
  ],
}))

function openFilePicker() {
  inputRef.value?.click()
}

function openFolderPicker() {
  folderInputRef.value?.click()
}

function handleInput(event: Event) {
  const target = event.target as HTMLInputElement
  enqueueFiles(target.files)
  target.value = ''
}

function handleFolderInput(event: Event) {
  const target = event.target as HTMLInputElement
  enqueueFiles(target.files)
  target.value = ''
}

async function handleDrop(event: DragEvent) {
  draggingFiles.value = false
  try {
    enqueueFiles(await filesFromDriveDrop(event.dataTransfer))
  } catch {
    error.value = '文件夹读取失败，请改用“选择文件夹”'
  }
}

function handleDragEnter(event: DragEvent) {
  if (!isFileDrag(event)) return
  draggingFiles.value = true
}

function handleDragLeave(event: DragEvent) {
  if (!event.currentTarget || !(event.currentTarget as HTMLElement).contains(event.relatedTarget as Node | null)) {
    draggingFiles.value = false
  }
}

function handleDropzoneKeydown(event: KeyboardEvent) {
  if (event.target !== event.currentTarget) return
  if (event.key !== 'Enter' && event.key !== ' ') return
  event.preventDefault()
  openFilePicker()
}

function isFileDrag(event: DragEvent) {
  return Array.from(event.dataTransfer?.types ?? []).includes('Files')
}

function enqueueFiles(files: FileList | File[] | null | undefined) {
  const values = Array.from(files ?? [])
  if (!values.length) {
    error.value = '没有读取到可上传文件'
    return
  }
  const { accepted, rejected } = filterUploadFiles(values)
  if (!accepted.length) {
    error.value = `没有读取到可上传文件。请确认文件夹内有文件，且格式符合当前目录要求（允许：${selectedAllowedLabel.value}）。`
    return
  }
  notice.value = rejected > 0
    ? `已跳过 ${formatInt(rejected)} 个空文件、系统隐藏文件或不符合目录格式限制的文件（允许：${selectedAllowedLabel.value}）。`
    : ''
  error.value = ''
  submittedFiles.value = []
  lastUploadResult.value = null
  lastSubmissionResult.value = null
  uploadCenter.addItems(accepted, {
    source: 'upload-page',
    uploadDirectoryId: selectedUploadDirectoryId.value || undefined,
    uploadDirectoryName: selectedUploadDirectory.value?.name ?? '',
    difficultyClass: selectedUploadDirectory.value?.difficulty_class ?? firstDifficultyCode(difficultyRows.value),
    finalized: true,
    pageCount: 1,
  })
}

function filterUploadFiles(files: File[]) {
  const allowed = selectedAllowedFileTypes.value
  const accepted = files.filter((file) => file.size > 0 && uploadFileAllowed(file, allowed) && uploadFilePathAllowed(file))
  return { accepted, rejected: files.length - accepted.length }
}

function uploadFilePathAllowed(file: File) {
  return isSafeDriveUploadPath(driveUploadRelativePath(file))
}

function uploadFileAllowed(file: File, allowed: string[]) {
  if (!allowed.length) return true
  const ext = file.name.includes('.') ? file.name.split('.').pop()?.toLowerCase() || '' : ''
  const mimeType = file.type.trim().toLowerCase()
  return allowed.some((value) => {
    if (ext && value === ext) return true
    if (mimeType && value === mimeType) return true
    if (mimeType && value.endsWith('/*')) return mimeType.startsWith(value.slice(0, -1))
    return false
  })
}

function normalizedAllowedFileTypes(values: string[]) {
  return values
    .map((value) => value.trim().toLowerCase().replace(/^\.+/, ''))
    .filter(Boolean)
}

function pieceworkFileLabel(pieceworkCount: number, fileCount: number) {
  if (fileCount <= 0) return '0 个作品'
  if (pieceworkCount === fileCount) return `${formatInt(pieceworkCount)} 个作品`
  return `${formatInt(pieceworkCount)} 个作品 · ${formatInt(fileCount)} 个文件`
}

async function uploadQueuedItems(includeFailed = false) {
  if (!canUseUploadDirectory.value) {
    error.value = '先选择这次上传要进入的目录'
    return false
  }
  const uploadTargets = queue.value.filter((item) => item.status === 'queued' || (includeFailed && item.status === 'failed'))
  if (!uploadTargets.length) {
    error.value = includeFailed ? '没有需要重试的失败文件' : '没有新的待上传文件'
    return false
  }
  uploading.value = true
  error.value = ''
  notice.value = ''
  lastUploadResult.value = null
  const uploadDirectoryId = selectedUploadDirectoryId.value || undefined
  const uploadDirectoryName = selectedUploadDirectory.value?.name ?? ''
  const uploadDirectoryDifficulty = selectedUploadDirectory.value?.difficulty_class ?? firstDifficultyCode(difficultyRows.value)
  const uploadBatchId = crypto.randomUUID?.() ?? `${Date.now()}-${Math.random()}`
  const validTargets: QueueItem[] = []
  for (const item of uploadTargets) {
    const allowed = selectedAllowedFileTypes.value
    if (!uploadFileAllowed(item.file, allowed) || !uploadFilePathAllowed(item.file)) {
      item.status = 'failed'
      item.error = `当前目录不允许上传这个文件。请确认格式或文件夹路径（允许：${selectedAllowedLabel.value}）。`
      uploadCenter.updateItem(item.id, { status: item.status, error: item.error })
      continue
    }
    validTargets.push(item)
  }
  try {
    await runDriveUploadPool(validTargets, async (item) => {
      item.status = 'uploading'
      item.error = ''
      item.progress = 0
      item.uploadDirectoryId = uploadDirectoryId
      item.uploadDirectoryName = uploadDirectoryName
      item.difficultyClass = uploadDirectoryDifficulty
      try {
        const uploaded = await uploadWorkbenchFile(item.file, {
          uploadDirectoryId,
          uploadBatchId,
          relativePath: item.relativePath,
          isFolderUpload: item.relativePath.includes('/'),
          expectedBusinessMonth: pageBusinessMonth,
          onProgress: (progress) => {
            item.progress = progress.percent
            uploadCenter.updateItem(item.id, { progress: progress.percent, status: 'uploading' })
          },
        })
        item.sessionId = uploaded.sessionId
        item.progress = 100
        item.status = 'uploaded'
        uploadCenter.updateItem(item.id, { sessionId: item.sessionId, progress: 100, status: 'uploaded', error: '' })
      } catch (err) {
        item.status = 'failed'
        item.error = resolveDriveUploadFailureMessage(err)
        uploadCenter.updateItem(item.id, { status: 'failed', error: item.error, progress: item.progress })
      }
    })
  } finally {
    uploading.value = false
  }
  const failed = uploadTargets.filter((item) => item.status === 'failed').length
  const success = uploadTargets.filter((item) => item.status === 'uploaded').length
  lastUploadResult.value = { total: uploadTargets.length, success, failed }
  if (failed === 0 && uploadTargets.length > 0) {
    notice.value = isSimpleUser.value
      ? `上传完成：成功 ${formatInt(success)} 个文件，失败 0 个`
      : `文件已上传：成功 ${formatInt(success)} 个文件。请继续生成提交记录，生成后才会进入上传记录。`
  }
  return failed === 0
}

async function runAdminPrimaryAction() {
  if (uploadedItems.value.length > 0 && !hasPendingUploads.value) {
    await createSubmission()
    return
  }
  await uploadQueuedItems(false)
  const successful = uploadedItems.value.length
  const failed = failedItems.value.length
  if (!successful) {
    error.value = '没有文件上传成功，请重试'
    return
  }
  const successfulPieces = uploadedPieceworkGroups.value.length
  await createSubmission()
  if (failed > 0 && !error.value) {
    notice.value = `已为 ${formatInt(successfulPieces)} 个作品生成提交记录，包含 ${formatInt(successful)} 个文件；${formatInt(failed)} 个文件上传失败，已保留在列表里可继续重试。`
  }
}

async function createSubmission() {
  if (!uploadedItems.value.length) return
  submitting.value = true
  error.value = ''
  notice.value = ''
  const submittingItems = [...uploadedItems.value]
  const submittingIds = submittingItems.map((item) => item.id)
  const pieceworkGroups = groupDriveUploadPieceworkItems(submittingItems)
  uploadCenter.updateItems(submittingIds, { status: 'submitting', error: '' })
  try {
    const detail = await assetWorkbenchApi.createSubmission({
      notes: '',
      expected_business_month: pageBusinessMonth,
      month_rollover_ack: false,
      items: pieceworkGroups.map((group) => {
        const firstItem = group.items[0]
        return {
          difficulty_class: firstItem?.difficultyClass || selectedUploadDirectory.value?.difficulty_class || undefined,
          finalized: group.items.every((item) => item.finalized),
          page_count: group.isFolder ? 1 : firstItem?.pageCount || 1,
          item_count: 1,
          upload_session_ids: group.items.map((item) => item.sessionId).filter(Boolean) as string[],
        }
      }),
    })
    submittedFiles.value = detail.items.flatMap((item) => item.files)
    lastSubmissionResult.value = { total: pieceworkGroups.length, success: submittedFiles.value.length, failed: 0 }
    notice.value = isSimpleUser.value
      ? `作品已交上去：${formatInt(pieceworkGroups.length)} 个作品，包含 ${formatInt(submittedFiles.value.length)} 个文件，可在上传记录里查看。`
      : `提交记录已生成：${formatInt(pieceworkGroups.length)} 个作品，包含 ${formatInt(submittedFiles.value.length)} 个文件，可在查改作品里查看。`
    uploadCenter.updateItems(submittingIds, { status: 'submitted', progress: 100, error: '' })
  } catch (err) {
    uploadCenter.updateItems(submittingIds, { status: 'uploaded' })
    error.value = submissionErrorMessage(err)
  } finally {
    submitting.value = false
  }
}

function submissionErrorMessage(err: unknown) {
  const parsed = parseApiErrorPayload(err)
  if (parsed.code === 'MONTH_ROLLOVER_REQUIRED') {
    return '上传已完成，但当前已跨入新的结算月份。请刷新页面后重新确认提交，避免计入错误账期。'
  }
  return `上传已完成，但提交失败：${resolveApiUserMessage(err, { fallback: '请稍后重试' })}`
}

async function submitSimple() {
  if (!canSimpleSubmit.value) return
  for (const item of queue.value) {
    item.difficultyClass = selectedUploadDirectory.value?.difficulty_class ?? item.difficultyClass
  }
  if (hasPendingUploads.value) {
    await uploadQueuedItems(false)
    if (!uploadedItems.value.length) {
      error.value = '没有文件上传成功，请重试。'
      return
    }
  }
  const successful = uploadedItems.value.length
  const successfulPieces = uploadedPieceworkGroups.value.length
  const failed = queue.value.filter((item) => item.status === 'failed').length
  await createSubmission()
  if (failed > 0 && !error.value) {
    notice.value = `已提交成功 ${formatInt(successfulPieces)} 个作品，包含 ${formatInt(successful)} 个文件；${formatInt(failed)} 个文件上传失败，已保留在列表里可继续重试。`
  }
}

async function retryAndSubmit() {
  if (!canRetryFailedUploads.value) return
  for (const item of queue.value) {
    if (item.status === 'failed') {
      item.difficultyClass = selectedUploadDirectory.value?.difficulty_class ?? item.difficultyClass
    }
  }
  await uploadQueuedItems(true)
  if (!uploadedItems.value.length) {
    error.value = '没有文件上传成功，请重试。'
    return
  }
  await createSubmission()
}

function selectUploadDirectory(directory: UploadDirectoryRow) {
  selectedUploadDirectoryId.value = directory.id
  for (const item of queue.value) {
    if (item.status === 'queued' || item.status === 'failed') {
      item.uploadDirectoryId = directory.id
      item.uploadDirectoryName = directory.name
      item.difficultyClass = directory.difficulty_class || firstDifficultyCode(difficultyRows.value)
    }
  }
}

function uploadDirectoryLocation(directory: UploadDirectoryRow) {
  return directory.oss_prefix ? `目录位置：${directory.oss_prefix}` : '目录位置：默认'
}

function uploadDirectoryDifficulty(directory?: UploadDirectoryRow) {
  return directory?.difficulty_class || firstDifficultyCode(difficultyRows.value)
}

function queueItemDisplayName(item: QueueItem) {
  return item.relativePath || item.file.name
}

function removeItem(id: string) {
  uploadCenter.removeItem(id)
  const next = new Set(expandedItemIds.value)
  next.delete(id)
  expandedItemIds.value = next
}

async function handleUploadSpreadsheetAction(payload: WorkbenchSpreadsheetActionPayload) {
  if (payload.action.key === 'open_file_picker') {
    openFilePicker()
    return
  }
  if (payload.action.key === 'submit_upload_queue') {
    if (isSimpleUser.value) {
      await submitSimple()
    } else {
      await runAdminPrimaryAction()
    }
    return
  }
  if (payload.action.key !== 'apply_upload_queue') return
  const rows = payload.sheets.find((sheet) => sheet.sheetId === 'upload_queue')?.rows ?? []
  const rowByID = new Map(rows.map((row) => [String(row.id), row]))
  for (const item of queue.value) {
    const row = rowByID.get(item.id)
    if (!row || item.status === 'uploading' || item.status === 'submitting') continue
    const pageCount = Number(row.pageCount)
    item.difficultyClass = String(row.difficultyClass ?? item.difficultyClass).trim() || item.difficultyClass
    item.pageCount = Number.isFinite(pageCount) && pageCount > 0 ? Math.floor(pageCount) : item.pageCount
    item.finalized =
      typeof row.finalized === 'boolean' ? row.finalized : ['true', '1', '是', '定稿', '已完成'].includes(String(row.finalized ?? '').trim())
  }
  notice.value = '已把表格修改应用到上传队列'
}

function toggleItemDetails(id: string) {
  const next = new Set(expandedItemIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expandedItemIds.value = next
}

function statusLabel(status: QueueStatus) {
  const labels: Record<QueueStatus, string> = {
    queued: '待上传',
    uploading: '上传中',
    uploaded: '已上传',
    submitting: '生成记录中',
    submitted: '已完成',
    failed: '上传失败',
  }
  return labels[status]
}

function statusTone(status: QueueStatus) {
  const tones: Record<QueueStatus, string> = {
    queued: 'aw-chip--neutral',
    uploading: 'aw-chip--info',
    uploaded: 'aw-chip--success',
    submitting: 'aw-chip--info',
    submitted: 'aw-chip--success',
    failed: 'aw-chip--danger',
  }
  return tones[status]
}

function statusIcon(status: QueueStatus) {
  if (status === 'uploaded' || status === 'submitted') return CheckCircle2
  if (status === 'failed') return XCircle
  if (status === 'uploading' || status === 'submitting') return LoaderCircle
  return FileUp
}

function openFilePreview(payload: { title: string; previewUrl: string; meta: FilePreviewMeta | null; statusText: string }) {
  previewDialog.value = {
    open: true,
    title: payload.title,
    previewUrl: payload.previewUrl,
    emptyLabel: payload.statusText,
    metaRows: [
      ['预览状态', payload.meta?.status || '等待生成'],
      ['过期时间', payload.meta?.expires_at || '—'],
    ],
  }
}

function closeFilePreview() {
  previewDialog.value.open = false
}

async function loadContext() {
  const next = await contextRequest.run()
  uploadDirectories.value = next?.directories ?? []
  difficultyRows.value = next?.difficulties ?? []
  if (!selectedUploadDirectoryId.value && uploadDirectories.value[0]) {
    selectedUploadDirectoryId.value = uploadDirectories.value[0].id
  }
}

onMounted(() => {
  void loadContext()
})

onActivated(() => {
  void loadContext()
})
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">{{ isSimpleUser ? '交作品' : '成品交付' }}</p>
        <h2>{{ isSimpleUser ? '把做好的文件交上来' : '成品上传中心' }}</h2>
        <p>{{ isSimpleUser ? '先选上传目录，再拖入文件。点一次提交，系统会自动处理。' : '批量拖拽文件，提交前校正难度、页数和定稿状态，系统会在上传成功后自动生成记录。' }}</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-secondary-button" type="button" @click="openFilePicker">选择文件</button>
        <button class="aw-secondary-button" type="button" @click="openFolderPicker">选择文件夹</button>
        <button class="aw-secondary-button" type="button" @click="uploadSpreadsheetOpen = !uploadSpreadsheetOpen">
          <Table2 :size="16" aria-hidden="true" />
          {{ uploadSpreadsheetOpen ? '收起表格校对' : '表格校对' }}
        </button>
        <span class="aw-action-stack">
          <button v-if="isSimpleUser" class="aw-primary-button" type="button" :disabled="!canSimpleSubmit" @click="submitSimple">
            <FileUp :size="16" aria-hidden="true" />
            {{ simpleSubmitLabel }}
          </button>
          <button v-else class="aw-primary-button" type="button" :disabled="!canUseAdminPrimaryAction" @click="runAdminPrimaryAction">
            <FileUp :size="16" aria-hidden="true" />
            {{ adminUploadLabel }}
          </button>
          <small>{{ isSimpleUser ? simpleSubmitHint : adminUploadHint }}</small>
        </span>
      </div>
    </div>

    <input ref="inputRef" class="aw-visually-hidden" type="file" multiple :accept="selectedAcceptString" aria-label="选择作品文件" @change="handleInput" />
    <input
      ref="folderInputRef"
      class="aw-visually-hidden"
      type="file"
      multiple
      webkitdirectory
      directory
      :accept="selectedAcceptString"
      aria-label="选择作品文件夹"
      @change="handleFolderInput"
    />

    <div class="aw-panel">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">上传目录</p>
          <h3>选择这次文件进入的位置</h3>
        </div>
        <div class="aw-inline-actions">
          <span v-if="selectedUploadDirectory" class="aw-chip aw-chip--info">{{ selectedUploadDirectory.name }} · {{ uploadDirectoryDifficulty(selectedUploadDirectory) }}</span>
          <span v-else class="aw-chip aw-chip--neutral">默认目录</span>
          <RouterLink v-if="canManageUploadDirectories" class="aw-secondary-button" to="/settings/upload-directories">管理上传目录</RouterLink>
        </div>
      </div>
      <AsyncBoundary
        :loading="contextLoading"
        :error="contextError"
        loading-label="正在加载上传目录"
        @retry="loadContext"
      >
        <div v-if="uploadDirectories.length" class="aw-template-option-grid">
          <button
            v-for="directory in uploadDirectories"
            :key="directory.id"
            class="aw-template-option"
            :class="{ 'aw-template-option--active': selectedUploadDirectoryId === directory.id }"
            type="button"
            @click="selectUploadDirectory(directory)"
          >
            <strong>{{ directory.name }}</strong>
            <span>{{ uploadDirectoryLocation(directory) }}</span>
            <span>计价分类：{{ uploadDirectoryDifficulty(directory) }}</span>
          </button>
        </div>
        <div v-else class="aw-empty-state">
          <h3>使用默认目录</h3>
          <p>管理端配置上传目录后，这里会要求先选择目录再上传。</p>
        </div>
      </AsyncBoundary>
    </div>

    <div
      class="aw-dropzone"
      :class="{ 'aw-dropzone--active': draggingFiles }"
      tabindex="0"
      @dragenter.prevent="handleDragEnter"
      @dragover.prevent="handleDragEnter"
      @dragleave="handleDragLeave"
      @drop.prevent="handleDrop"
      @keydown="handleDropzoneKeydown"
    >
      <FileUp :size="30" aria-hidden="true" />
      <strong>{{ queue.length ? '继续拖拽文件或文件夹到这里' : '拖拽文件或文件夹到这里' }}</strong>
      <span>{{ isSimpleUser ? '点击提交上传会自动完成上传和提交。' : '拖入文件或文件夹后，点击一次即可上传并生成提交记录。' }}允许：{{ selectedAllowedLabel }}</span>
      <div class="aw-dropzone__actions">
        <button class="aw-secondary-button" type="button" @click="openFilePicker">文件</button>
        <button class="aw-secondary-button" type="button" @click="openFolderPicker">文件夹</button>
        <button v-if="isSimpleUser" class="aw-primary-button" type="button" :disabled="!canSimpleSubmit" @click="submitSimple">
          {{ simpleSubmitLabel }}
        </button>
        <button v-else class="aw-primary-button" type="button" :disabled="!canUseAdminPrimaryAction" @click="runAdminPrimaryAction">
          {{ adminUploadLabel }}
        </button>
      </div>
      <p v-if="isSimpleUser" class="aw-dropzone__piecework-confirmation">
        <span>仔细核对作品数量后，点击提交上传。</span>
        <strong>{{ simplePieceworkConfirmation }}</strong>
      </p>
      <small v-if="!isSimpleUser" class="aw-dropzone__hint">{{ adminUploadHint }}</small>
    </div>

    <p v-if="error" class="aw-inline-alert">{{ error }}</p>
    <p v-else-if="notice" class="aw-inline-alert">{{ notice }}</p>
    <p v-if="uploadContinuityHint" class="aw-inline-alert aw-inline-alert--info">{{ uploadContinuityHint }}</p>

    <section v-if="failedItems.length" class="aw-upload-failure-callout" aria-live="polite">
      <div>
        <strong>{{ formatInt(failedItems.length) }} 个文件上传失败</strong>
        <p>继续选择新文件后，普通上传只会处理新文件；失败文件不会自动重复上传，需要你手动点击重试。</p>
      </div>
      <div class="aw-upload-failure-callout__actions">
        <button class="aw-primary-button" type="button" :disabled="!canRetryFailedUploads" @click="retryAndSubmit">重试失败文件</button>
        <button class="aw-secondary-button" type="button" :disabled="uploading || submitting" @click="uploadCenter.removeItems(failedItems.map((item) => item.id))">
          清除失败记录
        </button>
      </div>
    </section>

    <SpreadsheetWorkbench
      v-if="uploadSpreadsheetOpen"
      :source="uploadSpreadsheetSource"
      :height="460"
      @close="uploadSpreadsheetOpen = false"
      @action="handleUploadSpreadsheetAction"
    />

    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <span>{{ uploadUnitLabel }}</span>
        <span v-if="queue.length">{{ formatInt(uploadStats.percent) }}%</span>
        <span v-if="queue.length">文件成功 {{ formatInt(uploadStats.uploaded) }} · 失败 {{ formatInt(uploadStats.failed) }}</span>
        <span v-if="!isSimpleUser">{{ formatInt(totalPieceworkPages) }} 页</span>
        <button v-if="isSimpleUser" type="button" :disabled="!canSimpleSubmit" @click="submitSimple">{{ simpleSubmitLabel }}</button>
        <button v-else type="button" :disabled="!canSubmit" @click="createSubmission">{{ submitButtonLabel }}</button>
      </div>
      <div v-if="queue.length && isSimpleUser" class="aw-simple-upload-list">
        <article v-for="item in queue" :key="item.id" class="aw-simple-upload-item">
          <div class="aw-simple-upload-item__main">
            <component :is="statusIcon(item.status)" :size="22" aria-hidden="true" />
            <div>
              <strong :title="queueItemDisplayName(item)">{{ queueItemDisplayName(item) }}</strong>
              <span>{{ item.uploadDirectoryName || selectedUploadDirectory?.name || '默认目录' }} · 计价 {{ item.difficultyClass || uploadDirectoryDifficulty(selectedUploadDirectory) }} · {{ formatFileSize(item.file.size) }}</span>
            </div>
            <span class="aw-chip" :class="statusTone(item.status)">
              {{ item.status === 'uploading' ? `${item.progress}%` : statusLabel(item.status) }}
            </span>
          </div>
          <div v-if="item.status === 'uploading'" class="aw-upload-progress" aria-label="上传进度">
            <span :style="{ width: `${item.progress}%` }" />
          </div>
          <div class="aw-simple-upload-item__actions">
            <button class="aw-secondary-button" type="button" :disabled="item.status === 'uploading' || item.status === 'submitting'" @click="toggleItemDetails(item.id)">
              <span>{{ expandedItemIds.has(item.id) ? '收起信息' : '更多信息' }}</span>
              <ChevronUp v-if="expandedItemIds.has(item.id)" :size="16" aria-hidden="true" />
              <ChevronDown v-else :size="16" aria-hidden="true" />
            </button>
            <button class="aw-secondary-button" type="button" :disabled="item.status === 'uploading' || item.status === 'submitting'" @click="removeItem(item.id)">移除</button>
          </div>
          <div v-if="expandedItemIds.has(item.id)" class="aw-simple-upload-item__details">
            <label class="aw-field">
              <span>张数</span>
              <input v-model.number="item.pageCount" aria-label="张数" min="1" type="number" />
            </label>
            <label class="aw-inline-check">
              <input v-model="item.finalized" type="checkbox" />
              已完成
            </label>
          </div>
          <p v-if="item.error" class="aw-upload-row__error">{{ item.error }}</p>
        </article>
      </div>
      <div v-else-if="queue.length" class="aw-upload-list">
        <div v-for="item in queue" :key="item.id" class="aw-upload-row">
          <span class="aw-cell-text" :title="queueItemDisplayName(item)">{{ queueItemDisplayName(item) }}</span>
          <select v-model="item.difficultyClass" aria-label="难度类">
            <option v-for="option in difficultyOptions" :key="option" :value="option">{{ option }}</option>
          </select>
          <input v-model.number="item.pageCount" aria-label="页数" min="1" type="number" />
          <label class="aw-inline-check">
            <input v-model="item.finalized" type="checkbox" />
            定稿
          </label>
          <strong>{{ item.status === 'uploading' ? `${item.progress}%` : statusLabel(item.status) }}</strong>
          <span class="aw-cell-text">{{ item.uploadDirectoryName || selectedUploadDirectory?.name || '默认目录' }}</span>
          <button type="button" :disabled="item.status === 'uploading' || item.status === 'submitting'" @click="removeItem(item.id)">移除</button>
          <span v-if="item.error" class="aw-upload-row__error">{{ item.error }}</span>
        </div>
      </div>
      <div v-else class="aw-empty-state">
        <h3>等待文件</h3>
        <p>{{ isSimpleUser ? '支持一次交多个文件。交上去以后，可以在看收入里查看金额。' : '支持批量拖拽上传。完成后进入维护专区，管理员可以质检、修正、下载和结算。' }}</p>
      </div>
    </div>

    <div v-if="lastUploadResult || lastSubmissionResult" class="aw-panel aw-panel--stage">
      <h3>本次上传结论</h3>
      <p v-if="lastUploadResult" class="aw-copy">
        上传：共 {{ formatInt(lastUploadResult.total) }} 个文件，成功 {{ formatInt(lastUploadResult.success) }} 个，失败 {{ formatInt(lastUploadResult.failed) }} 个。
      </p>
      <p v-if="lastSubmissionResult" class="aw-copy">
        提交：已生成 {{ formatInt(lastSubmissionResult.total) }} 个作品，包含 {{ formatInt(lastSubmissionResult.success) }} 个文件记录。
      </p>
      <div v-if="failedItems.length" class="aw-inline-actions">
        <button class="aw-primary-button" type="button" :disabled="!canRetryFailedUploads" @click="retryAndSubmit">重试失败文件</button>
      </div>
      <ul v-if="failedItems.length" class="aw-upload-result-list">
        <li v-for="item in failedItems" :key="item.id">
          <strong>{{ queueItemDisplayName(item) }}</strong>
          <span>{{ item.error || '上传失败' }}</span>
        </li>
      </ul>
    </div>

    <div v-if="submittedFiles.length" class="aw-panel aw-panel--stage">
      <h3>{{ isSimpleUser ? '已交上的文件' : '提交预览' }}</h3>
      <p class="aw-copy">{{ isSimpleUser ? '预览图生成需要一点时间。你可以继续交新的作品。' : '预览图生成需要一点时间。生成完成后，可以在维护专区继续查看和下载源文件。' }}</p>
      <div class="aw-preview-grid">
        <WorkbenchFilePreview
          v-for="file in submittedFiles"
          :key="file.id"
          :file-id="file.id"
          :alt="file.original_filename"
          @preview="openFilePreview"
        />
      </div>
    </div>

    <WorkbenchPreviewDialog
      :open="previewDialog.open"
      :title="previewDialog.title"
      :preview-url="previewDialog.previewUrl"
      :empty-label="previewDialog.emptyLabel"
      :meta-rows="previewDialog.metaRows"
      eyebrow="文件预览"
      @close="closeFilePreview"
    />
  </section>
</template>
