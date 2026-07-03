<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { CheckCircle2, ChevronDown, ChevronUp, FileUp, LoaderCircle, XCircle } from 'lucide-vue-next'

import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import { uploadWorkbenchFile } from '@aw/features/upload/uploadFlow'
import { assetWorkbenchApi, type DifficultyClassRow, type FilePreviewMeta, type SubmissionFileRow, type UploadDirectoryRow } from '@aw/shared/api/assetWorkbenchApi'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { currentBusinessMonth } from '@aw/shared/format/businessMonth'
import { difficultyCodes, firstDifficultyCode } from '@aw/shared/format/difficulty'
import { formatFileSize, formatInt } from '@aw/shared/format/number'
import WorkbenchFilePreview from '@aw/shared/preview/WorkbenchFilePreview.vue'
import WorkbenchPreviewDialog from '@aw/shared/preview/WorkbenchPreviewDialog.vue'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'
import { parseApiErrorPayload, resolveApiUserMessage } from '@/utils/api-message-zh'

type QueueStatus = 'queued' | 'uploading' | 'uploaded' | 'failed'

interface QueueItem {
  id: string
  file: File
  difficultyClass: string
  finalized: boolean
  pageCount: number
  progress: number
  status: QueueStatus
  sessionId?: string
  uploadDirectoryId?: number
  uploadDirectoryName?: string
  error?: string
}

interface UploadContext {
  directories: UploadDirectoryRow[]
  difficulties: DifficultyClassRow[]
}

interface UploadBatchResult {
  total: number
  success: number
  failed: number
}

const queue = ref<QueueItem[]>([])
const inputRef = ref<HTMLInputElement | null>(null)
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
const isSimpleUser = computed(() => bootstrap.value?.is_admin === false)
const requiresUploadDirectory = computed(() => uploadDirectories.value.length > 0)
const selectedUploadDirectory = computed(() => uploadDirectories.value.find((item) => item.id === selectedUploadDirectoryId.value))
const canUseUploadDirectory = computed(() => !requiresUploadDirectory.value || selectedUploadDirectoryId.value > 0)
const hasPendingUploads = computed(() => queue.value.some((item) => item.status === 'queued' || item.status === 'failed'))
const canSubmit = computed(() => {
  if (uploadedItems.value.length === 0 || uploading.value || submitting.value) return false
  return true
})
const canSimpleSubmit = computed(() => {
  if (!isSimpleUser.value) return false
  if (!queue.value.length || uploading.value || submitting.value) return false
  return canUseUploadDirectory.value && queue.value.every((item) => item.status !== 'uploading')
})
const totalPages = computed(() => queue.value.reduce((sum, item) => sum + item.pageCount, 0))
const difficultyOptions = computed(() => difficultyCodes(difficultyRows.value))
const uploadStats = computed(() => {
  const total = queue.value.length
  const uploaded = queue.value.filter((item) => item.status === 'uploaded').length
  const failed = queue.value.filter((item) => item.status === 'failed').length
  const uploadingCount = queue.value.filter((item) => item.status === 'uploading').length
  const progressTotal = queue.value.reduce((sum, item) => {
    if (item.status === 'uploaded') return sum + 100
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
  if (queue.value.some((item) => item.status === 'failed')) return '会先重试失败文件，再提交作品'
  if (queue.value.some((item) => item.status === 'uploaded')) return `将提交 ${formatInt(uploadedItems.value.length)} 个已上传文件`
  return `将上传并提交 ${formatInt(queue.value.length)} 个文件`
})
const adminUploadLabel = computed(() => {
  if (uploading.value) return '正在上传'
  if (submitting.value) return '正在生成记录'
  if (uploadedItems.value.length > 0 && !hasPendingUploads.value) return `生成提交记录 ${uploadedItems.value.length} 个`
  if (queue.value.some((item) => item.status === 'failed')) return '重试并生成记录'
  return '上传并生成记录'
})
const adminUploadHint = computed(() => {
  if (!queue.value.length) return '先选择文件，或把文件拖到上传区'
  if (!canUseUploadDirectory.value) return '先选择这批文件进入的上传目录'
  if (queue.value.some((item) => item.status === 'failed')) return '会重试失败文件，成功后自动生成提交记录'
  if (uploadedItems.value.length > 0 && !hasPendingUploads.value) return '将生成提交记录，生成后进入上传记录'
  return `将上传并生成 ${formatInt(queue.value.length)} 个提交记录`
})
const submitButtonLabel = computed(() => {
  if (submitting.value) return isSimpleUser.value ? '正在交作品' : '正在创建提交'
  if (uploadedItems.value.length === 0) return '先上传文件'
  return isSimpleUser.value ? `交作品 ${uploadedItems.value.length} 个` : `生成提交记录 ${uploadedItems.value.length} 个`
})
const canUseAdminPrimaryAction = computed(() => {
  if (uploading.value || submitting.value || queue.value.length === 0) return false
  if (uploadedItems.value.length > 0 && !hasPendingUploads.value) return canSubmit.value
  return canUseUploadDirectory.value
})

function openFilePicker() {
  inputRef.value?.click()
}

function handleInput(event: Event) {
  const target = event.target as HTMLInputElement
  enqueueFiles(target.files)
  target.value = ''
}

function handleDrop(event: DragEvent) {
  draggingFiles.value = false
  enqueueFiles(event.dataTransfer?.files)
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

function enqueueFiles(files: FileList | null | undefined) {
  if (!files?.length) return
  notice.value = ''
  error.value = ''
  submittedFiles.value = []
  lastUploadResult.value = null
  lastSubmissionResult.value = null
  for (const file of Array.from(files)) {
    queue.value.push({
      id: crypto.randomUUID?.() ?? `${Date.now()}-${Math.random()}`,
      file,
      difficultyClass: selectedUploadDirectory.value?.difficulty_class ?? firstDifficultyCode(difficultyRows.value),
      finalized: true,
      pageCount: 1,
      progress: 0,
      status: 'queued',
    })
  }
}

async function uploadQueuedItems() {
  if (!canUseUploadDirectory.value) {
    error.value = '先选择这次上传要进入的目录'
    return false
  }
  uploading.value = true
  error.value = ''
  notice.value = ''
  lastUploadResult.value = null
  const uploadDirectoryId = selectedUploadDirectoryId.value || undefined
  const uploadDirectoryName = selectedUploadDirectory.value?.name ?? ''
  const uploadDirectoryDifficulty = selectedUploadDirectory.value?.difficulty_class ?? firstDifficultyCode(difficultyRows.value)
  for (const item of queue.value) {
    if (item.status !== 'queued' && item.status !== 'failed') continue
    item.status = 'uploading'
    item.error = ''
    item.progress = 0
    item.uploadDirectoryId = uploadDirectoryId
    item.uploadDirectoryName = uploadDirectoryName
    item.difficultyClass = uploadDirectoryDifficulty
    try {
      const uploaded = await uploadWorkbenchFile(item.file, {
        uploadDirectoryId,
        expectedBusinessMonth: pageBusinessMonth,
        onProgress: (progress) => {
          item.progress = progress.percent
        },
      })
      item.sessionId = uploaded.sessionId
      item.progress = 100
      item.status = 'uploaded'
    } catch (err) {
      item.status = 'failed'
      item.error = resolveApiUserMessage(err, { fallback: '上传失败，请重试' })
    }
  }
  uploading.value = false
  const failed = queue.value.filter((item) => item.status === 'failed').length
  const success = queue.value.filter((item) => item.status === 'uploaded').length
  lastUploadResult.value = { total: queue.value.length, success, failed }
  if (failed === 0 && queue.value.length > 0) {
    notice.value = isSimpleUser.value
      ? `上传完成：成功 ${formatInt(success)} 个，失败 0 个`
      : `文件已上传：成功 ${formatInt(success)} 个。请继续生成提交记录，生成后才会进入上传记录。`
  }
  return failed === 0
}

async function runAdminPrimaryAction() {
  if (uploadedItems.value.length > 0 && !hasPendingUploads.value) {
    await createSubmission()
    return
  }
  const uploaded = await uploadQueuedItems()
  if (!uploaded || queue.value.some((item) => item.status === 'failed')) {
    error.value = '部分文件上传失败'
    return
  }
  await createSubmission()
}

async function createSubmission() {
  if (!uploadedItems.value.length) return
  submitting.value = true
  error.value = ''
  notice.value = ''
  try {
    const detail = await assetWorkbenchApi.createSubmission({
      notes: '',
      expected_business_month: pageBusinessMonth,
      month_rollover_ack: false,
      items: uploadedItems.value.map((item) => ({
        difficulty_class: item.difficultyClass || selectedUploadDirectory.value?.difficulty_class || undefined,
        finalized: item.finalized,
        page_count: item.pageCount,
        item_count: 1,
        upload_session_ids: item.sessionId ? [item.sessionId] : [],
      })),
    })
    submittedFiles.value = detail.items.flatMap((item) => item.files)
    lastSubmissionResult.value = { total: uploadedItems.value.length, success: submittedFiles.value.length, failed: 0 }
    notice.value = isSimpleUser.value
      ? `作品已交上去：${formatInt(submittedFiles.value.length)} 个文件，可在上传记录里查看。`
      : `提交记录已生成：${formatInt(submittedFiles.value.length)} 个文件，可在查改作品里查看。`
    queue.value = queue.value.filter((item) => item.status !== 'uploaded')
  } catch (err) {
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
    const uploaded = await uploadQueuedItems()
    if (!uploaded || queue.value.some((item) => item.status === 'failed')) {
      error.value = '有文件没传成功，请点重试后再交。'
      return
    }
  }
  await createSubmission()
}

async function retryAndSubmit() {
  if (isSimpleUser.value) {
    await submitSimple()
    return
  }
  await runAdminPrimaryAction()
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

function removeItem(id: string) {
  queue.value = queue.value.filter((item) => item.id !== id)
  const next = new Set(expandedItemIds.value)
  next.delete(id)
  expandedItemIds.value = next
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
    failed: '上传失败',
  }
  return labels[status]
}

function statusTone(status: QueueStatus) {
  const tones: Record<QueueStatus, string> = {
    queued: 'aw-chip--neutral',
    uploading: 'aw-chip--info',
    uploaded: 'aw-chip--success',
    failed: 'aw-chip--danger',
  }
  return tones[status]
}

function statusIcon(status: QueueStatus) {
  if (status === 'uploaded') return CheckCircle2
  if (status === 'failed') return XCircle
  if (status === 'uploading') return LoaderCircle
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

    <input ref="inputRef" class="aw-visually-hidden" type="file" multiple aria-label="选择作品文件" @change="handleInput" />

    <div class="aw-panel">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">上传目录</p>
          <h3>选择这次文件进入的位置</h3>
        </div>
        <span v-if="selectedUploadDirectory" class="aw-chip aw-chip--info">{{ selectedUploadDirectory.name }} · {{ uploadDirectoryDifficulty(selectedUploadDirectory) }}</span>
        <span v-else class="aw-chip aw-chip--neutral">默认目录</span>
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
      <strong>{{ queue.length ? '继续拖拽文件到这里' : '拖拽文件到这里' }}</strong>
      <span>{{ isSimpleUser ? '拖入或选择文件后，点击提交上传会自动完成上传和提交。' : '拖入或选择文件后，点击一次即可上传并生成提交记录。' }}</span>
      <div class="aw-dropzone__actions">
        <button class="aw-secondary-button" type="button" @click="openFilePicker">选择文件</button>
        <button v-if="isSimpleUser" class="aw-primary-button" type="button" :disabled="!canSimpleSubmit" @click="submitSimple">
          {{ simpleSubmitLabel }}
        </button>
        <button v-else class="aw-primary-button" type="button" :disabled="!canUseAdminPrimaryAction" @click="runAdminPrimaryAction">
          {{ adminUploadLabel }}
        </button>
      </div>
      <small class="aw-dropzone__hint">{{ isSimpleUser ? simpleSubmitHint : adminUploadHint }}</small>
    </div>

    <p v-if="error" class="aw-inline-alert">{{ error }}</p>
    <p v-else-if="notice" class="aw-inline-alert">{{ notice }}</p>

    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <span>{{ formatInt(queue.length) }} 个文件</span>
        <span v-if="queue.length">{{ formatInt(uploadStats.percent) }}%</span>
        <span v-if="queue.length">成功 {{ formatInt(uploadStats.uploaded) }} · 失败 {{ formatInt(uploadStats.failed) }}</span>
        <span v-if="!isSimpleUser">{{ formatInt(totalPages) }} 页</span>
        <button v-if="isSimpleUser" type="button" :disabled="!canSimpleSubmit" @click="submitSimple">{{ simpleSubmitLabel }}</button>
        <button v-else type="button" :disabled="!canSubmit" @click="createSubmission">{{ submitButtonLabel }}</button>
      </div>
      <div v-if="queue.length && isSimpleUser" class="aw-simple-upload-list">
        <article v-for="item in queue" :key="item.id" class="aw-simple-upload-item">
          <div class="aw-simple-upload-item__main">
            <component :is="statusIcon(item.status)" :size="22" aria-hidden="true" />
            <div>
              <strong>{{ item.file.name }}</strong>
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
            <button class="aw-secondary-button" type="button" :disabled="item.status === 'uploading'" @click="toggleItemDetails(item.id)">
              <span>{{ expandedItemIds.has(item.id) ? '收起信息' : '更多信息' }}</span>
              <ChevronUp v-if="expandedItemIds.has(item.id)" :size="16" aria-hidden="true" />
              <ChevronDown v-else :size="16" aria-hidden="true" />
            </button>
            <button class="aw-secondary-button" type="button" :disabled="item.status === 'uploading'" @click="removeItem(item.id)">移除</button>
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
          <span class="aw-cell-text">{{ item.file.name }}</span>
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
          <button type="button" :disabled="item.status === 'uploading'" @click="removeItem(item.id)">移除</button>
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
        上传：共 {{ formatInt(lastUploadResult.total) }} 个，成功 {{ formatInt(lastUploadResult.success) }} 个，失败 {{ formatInt(lastUploadResult.failed) }} 个。
      </p>
      <p v-if="lastSubmissionResult" class="aw-copy">
        提交：已生成 {{ formatInt(lastSubmissionResult.success) }} 个文件记录。
      </p>
      <div v-if="queue.some((item) => item.status === 'failed')" class="aw-inline-actions">
        <button class="aw-primary-button" type="button" :disabled="uploading || submitting || !canUseUploadDirectory" @click="retryAndSubmit">重试并提交</button>
      </div>
      <ul v-if="queue.some((item) => item.status === 'failed')" class="aw-upload-result-list">
        <li v-for="item in queue.filter((entry) => entry.status === 'failed')" :key="item.id">
          <strong>{{ item.file.name }}</strong>
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
