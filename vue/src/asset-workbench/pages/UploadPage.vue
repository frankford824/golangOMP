<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { CheckCircle2, ChevronDown, ChevronUp, FileUp, LoaderCircle, XCircle } from 'lucide-vue-next'

import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import { uploadWorkbenchFile } from '@aw/features/upload/uploadFlow'
import { assetWorkbenchApi, type SubmissionFileRow, type WorkbenchTemplateRow } from '@aw/shared/api/assetWorkbenchApi'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { formatFileSize, formatInt } from '@aw/shared/format/number'
import WorkbenchFilePreview from '@aw/shared/preview/WorkbenchFilePreview.vue'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'

type QueueStatus = 'queued' | 'uploading' | 'uploaded' | 'failed'

interface QueueItem {
  id: string
  file: File
  orderNo: string
  templateId: number
  difficultyClass: string
  finalized: boolean
  pageCount: number
  progress: number
  status: QueueStatus
  sessionId?: string
  error?: string
}

const queue = ref<QueueItem[]>([])
const inputRef = ref<HTMLInputElement | null>(null)
const uploading = ref(false)
const submitting = ref(false)
const error = ref('')
const notice = ref('')
const submittedFiles = ref<SubmissionFileRow[]>([])
const templates = ref<WorkbenchTemplateRow[]>([])
const selectedTemplateId = ref(0)
const expandedItemIds = ref<Set<string>>(new Set())
const difficultyOptions = ['A', 'B', 'C', 'A+小夜灯']
const { bootstrap, refresh } = useAssetWorkbenchBootstrap()
const contextRequest = usePageRequest<WorkbenchTemplateRow[]>(
  async () => {
    await refresh()
    return assetWorkbenchApi.listMyTemplates()
  },
  [],
  '作品类型加载失败',
)
const contextLoading = contextRequest.loading
const contextError = contextRequest.error

const uploadedItems = computed(() => queue.value.filter((item) => item.status === 'uploaded'))
const isSimpleUser = computed(() => bootstrap.value?.is_admin === false)
const hasPendingUploads = computed(() => queue.value.some((item) => item.status === 'queued' || item.status === 'failed'))
const canSubmit = computed(() => {
  if (uploadedItems.value.length === 0 || uploading.value || submitting.value) return false
  if (!isSimpleUser.value) return true
  return uploadedItems.value.every((item) => item.templateId > 0)
})
const canSimpleSubmit = computed(() => {
  if (!isSimpleUser.value) return false
  if (!queue.value.length || uploading.value || submitting.value) return false
  return selectedTemplateId.value > 0 && queue.value.every((item) => item.status !== 'uploading')
})
const totalPages = computed(() => queue.value.reduce((sum, item) => sum + item.pageCount, 0))
const selectedTemplate = computed(() => templates.value.find((item) => item.id === selectedTemplateId.value))
const simplePrimaryLabel = computed(() => {
  if (uploading.value) return '正在上传'
  if (submitting.value) return '正在交上去'
  if (!queue.value.length) return '先选择文件'
  if (!selectedTemplateId.value) return '先选作品类型'
  return `交上去 ${formatInt(queue.value.length)} 个文件`
})
const submitButtonLabel = computed(() => {
  if (submitting.value) return isSimpleUser.value ? '正在交作品' : '正在创建提交'
  if (uploadedItems.value.length === 0) return '先上传队列'
  if (isSimpleUser.value && uploadedItems.value.some((item) => item.templateId <= 0)) return '先选择作品类型'
  return isSimpleUser.value ? `交作品 ${uploadedItems.value.length} 个` : `创建提交 ${uploadedItems.value.length} 单`
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
  enqueueFiles(event.dataTransfer?.files)
}

function enqueueFiles(files: FileList | null | undefined) {
  if (!files?.length) return
  notice.value = ''
  error.value = ''
  submittedFiles.value = []
  for (const file of Array.from(files)) {
    queue.value.push({
      id: crypto.randomUUID?.() ?? `${Date.now()}-${Math.random()}`,
      file,
      orderNo: filenameWithoutExt(file.name),
      templateId: selectedTemplateId.value,
      difficultyClass: selectedTemplate.value?.difficulty_class ?? 'A',
      finalized: true,
      pageCount: 1,
      progress: 0,
      status: 'queued',
    })
  }
}

async function uploadQueuedItems() {
  uploading.value = true
  error.value = ''
  notice.value = ''
  for (const item of queue.value) {
    if (item.status !== 'queued' && item.status !== 'failed') continue
    item.status = 'uploading'
    item.error = ''
    item.progress = 0
    try {
      const uploaded = await uploadWorkbenchFile(item.file, {
        onProgress: (progress) => {
          item.progress = progress.percent
        },
      })
      item.sessionId = uploaded.sessionId
      item.progress = 100
      item.status = 'uploaded'
    } catch (err) {
      item.status = 'failed'
      item.error = err instanceof Error ? err.message : '上传失败'
    }
  }
  uploading.value = false
  return queue.value.every((item) => item.status !== 'failed')
}

async function uploadAll() {
  const ok = await uploadQueuedItems()
  if (!ok) {
    error.value = isSimpleUser.value ? '有文件没传成功，请点重试后再交。' : '部分文件上传失败'
  }
}

async function createSubmission() {
  if (!uploadedItems.value.length) return
  submitting.value = true
  error.value = ''
  notice.value = ''
  try {
    const detail = await assetWorkbenchApi.createSubmission({
      notes: '',
      items: uploadedItems.value.map((item) => ({
        order_no: item.orderNo,
        template_id: item.templateId || undefined,
        difficulty_class: item.difficultyClass,
        finalized: item.finalized,
        page_count: item.pageCount,
        item_count: 1,
        upload_session_ids: item.sessionId ? [item.sessionId] : [],
      })),
    })
    submittedFiles.value = detail.items.flatMap((item) => item.files)
    notice.value = isSimpleUser.value ? '作品已交上去' : '提交已创建'
    queue.value = queue.value.filter((item) => item.status !== 'uploaded')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '提交失败'
  } finally {
    submitting.value = false
  }
}

async function submitSimple() {
  if (!canSimpleSubmit.value) return
  if (!selectedTemplateId.value) {
    error.value = '先选这次要交的作品类型'
    return
  }
  if (queue.value.some((item) => item.templateId <= 0)) {
    for (const item of queue.value) {
      item.templateId = selectedTemplateId.value
      item.difficultyClass = selectedTemplate.value?.difficulty_class ?? item.difficultyClass
    }
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

function selectTemplate(template: WorkbenchTemplateRow) {
  selectedTemplateId.value = template.id
  for (const item of queue.value) {
    item.templateId = template.id
    item.difficultyClass = template.difficulty_class
  }
}

function templateName(templateId: number) {
  return templates.value.find((item) => item.id === templateId)?.name ?? '未选择'
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

function filenameWithoutExt(filename: string) {
  const dot = filename.lastIndexOf('.')
  return dot > 0 ? filename.slice(0, dot) : filename
}

async function loadContext() {
  const next = await contextRequest.run()
  templates.value = next ?? []
  if (!selectedTemplateId.value && templates.value[0]) {
    selectedTemplateId.value = templates.value[0].id
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
        <p>{{ isSimpleUser ? '先选作品类型，再拖入文件。点一次交上去，系统会自动处理。' : '批量拖拽文件，提交前校正订单号、难度、页数和定稿状态。' }}</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-secondary-button" type="button" @click="openFilePicker">选择文件</button>
        <button v-if="isSimpleUser" class="aw-primary-button" type="button" :disabled="!canSimpleSubmit" @click="submitSimple">
          {{ simplePrimaryLabel }}
        </button>
        <button v-else class="aw-primary-button" type="button" :disabled="uploading || queue.length === 0" @click="uploadAll">
          上传队列
        </button>
      </div>
    </div>

    <input ref="inputRef" class="aw-visually-hidden" type="file" multiple aria-label="选择作品文件" @change="handleInput" />

    <div v-if="isSimpleUser" class="aw-panel">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">作品类型</p>
          <h3>选择这次要交的类型</h3>
        </div>
      </div>
      <AsyncBoundary
        :loading="contextLoading"
        :error="contextError"
        loading-label="正在加载作品类型"
        @retry="loadContext"
      >
        <div v-if="templates.length" class="aw-template-option-grid">
          <button
            v-for="template in templates"
            :key="template.id"
            class="aw-template-option"
            :class="{ 'aw-template-option--active': selectedTemplateId === template.id }"
            type="button"
            @click="selectTemplate(template)"
          >
            <strong>{{ template.name }}</strong>
            <span>{{ template.category || '常规作品' }}</span>
          </button>
        </div>
        <div v-else class="aw-empty-state">
          <h3>还没有可选类型</h3>
          <p>管理员下发作品类型后，你就能在这里选择并上传。</p>
        </div>
      </AsyncBoundary>
    </div>

    <div class="aw-dropzone" tabindex="0" @dragover.prevent @drop.prevent="handleDrop">
      <strong>拖拽文件到这里</strong>
      <span>{{ isSimpleUser ? '文件名会自动识别；选好类型后点交上去。' : '系统会默认把文件名识别为订单号；提交前可以逐条修改难度、页数和定稿状态。' }}</span>
    </div>

    <p v-if="error" class="aw-inline-alert">{{ error }}</p>
    <p v-else-if="notice" class="aw-inline-alert">{{ notice }}</p>

    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <span>{{ formatInt(queue.length) }} 个文件</span>
        <span v-if="!isSimpleUser">{{ formatInt(totalPages) }} 页</span>
        <button v-if="isSimpleUser" type="button" :disabled="!canSimpleSubmit" @click="submitSimple">{{ simplePrimaryLabel }}</button>
        <button v-else type="button" :disabled="!canSubmit" @click="createSubmission">{{ submitButtonLabel }}</button>
      </div>
      <div v-if="queue.length && isSimpleUser" class="aw-simple-upload-list">
        <article v-for="item in queue" :key="item.id" class="aw-simple-upload-item">
          <div class="aw-simple-upload-item__main">
            <component :is="statusIcon(item.status)" :size="22" aria-hidden="true" />
            <div>
              <strong>{{ item.file.name }}</strong>
              <span>{{ templateName(item.templateId) }} · {{ formatFileSize(item.file.size) }}</span>
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
              <span>文件名识别</span>
              <input v-model="item.orderNo" aria-label="文件名识别" />
            </label>
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
          <label class="aw-field aw-upload-row__order">
            <span>订单号</span>
            <input v-model="item.orderNo" aria-label="订单号" />
          </label>
          <span v-if="isSimpleUser" class="aw-chip aw-chip--info">{{ templateName(item.templateId) }}</span>
          <select v-else v-model="item.difficultyClass" aria-label="难度类">
            <option v-for="option in difficultyOptions" :key="option" :value="option">{{ option }}</option>
          </select>
          <input v-model.number="item.pageCount" aria-label="页数" min="1" type="number" />
          <label class="aw-inline-check">
            <input v-model="item.finalized" type="checkbox" />
            定稿
          </label>
          <strong>{{ item.status === 'uploading' ? `${item.progress}%` : statusLabel(item.status) }}</strong>
          <button type="button" :disabled="item.status === 'uploading'" @click="removeItem(item.id)">移除</button>
          <span v-if="item.error" class="aw-upload-row__error">{{ item.error }}</span>
        </div>
      </div>
      <div v-else class="aw-empty-state">
        <h3>等待文件</h3>
        <p>{{ isSimpleUser ? '支持一次交多个文件。交上去以后，可以在看收入里查看金额。' : '支持批量拖拽上传。完成后进入维护专区，管理员可以质检、修正、下载和结算。' }}</p>
      </div>
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
        />
      </div>
    </div>
  </section>
</template>
