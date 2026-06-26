<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import { uploadWorkbenchFile } from '@aw/features/upload/uploadFlow'
import { assetWorkbenchApi, type SubmissionFileRow, type WorkbenchTemplateRow } from '@aw/shared/api/assetWorkbenchApi'
import WorkbenchFilePreview from '@aw/shared/preview/WorkbenchFilePreview.vue'

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
const difficultyOptions = ['A', 'B', 'C', 'A+小夜灯']
const { bootstrap, refresh } = useAssetWorkbenchBootstrap()

const uploadedItems = computed(() => queue.value.filter((item) => item.status === 'uploaded'))
const isSimpleUser = computed(() => bootstrap.value?.is_admin === false)
const canSubmit = computed(() => {
  if (uploadedItems.value.length === 0 || uploading.value || submitting.value) return false
  if (!isSimpleUser.value) return true
  return uploadedItems.value.every((item) => item.templateId > 0)
})
const totalPages = computed(() => queue.value.reduce((sum, item) => sum + item.pageCount, 0))
const selectedTemplate = computed(() => templates.value.find((item) => item.id === selectedTemplateId.value))
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

async function uploadAll() {
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

function filenameWithoutExt(filename: string) {
  const dot = filename.lastIndexOf('.')
  return dot > 0 ? filename.slice(0, dot) : filename
}

async function loadContext() {
  await refresh()
  templates.value = await assetWorkbenchApi.listMyTemplates()
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
        <p>{{ isSimpleUser ? '先选作品类型，再拖入文件。文件名会自动当作订单号。' : '批量拖拽文件，提交前校正订单号、难度、页数和定稿状态。' }}</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-secondary-button" type="button" @click="openFilePicker">选择文件</button>
        <button class="aw-primary-button" type="button" :disabled="uploading || queue.length === 0" @click="uploadAll">
          上传队列
        </button>
      </div>
    </div>

    <input ref="inputRef" class="aw-visually-hidden" type="file" multiple @change="handleInput" />

    <div v-if="isSimpleUser" class="aw-panel">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">作品类型</p>
          <h3>选择这次要交的类型</h3>
        </div>
      </div>
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
    </div>

    <div class="aw-dropzone" tabindex="0" @dragover.prevent @drop.prevent="handleDrop">
      <strong>拖拽文件到这里</strong>
      <span>{{ isSimpleUser ? '文件名会自动当作订单号；选好类型后直接上传。' : '系统会默认把文件名识别为订单号；提交前可以逐条修改难度、页数和定稿状态。' }}</span>
    </div>

    <p v-if="error" class="aw-inline-alert">{{ error }}</p>
    <p v-else-if="notice" class="aw-inline-alert">{{ notice }}</p>

    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <span>{{ queue.length }} 个文件</span>
        <span>{{ totalPages }} 页</span>
        <button type="button" :disabled="!canSubmit" @click="createSubmission">{{ submitButtonLabel }}</button>
      </div>
      <div v-if="queue.length" class="aw-upload-list">
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
        <p>{{ isSimpleUser ? '支持批量拖拽上传。上传后可以在看收入里查看金额。' : '支持批量拖拽上传。完成后进入维护专区，管理员可以质检、修正、下载和结算。' }}</p>
      </div>
    </div>

    <div v-if="submittedFiles.length" class="aw-panel aw-panel--stage">
      <h3>提交预览</h3>
      <p class="aw-copy">预览图生成需要一点时间。生成完成后，可以在维护专区继续查看和下载源文件。</p>
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
