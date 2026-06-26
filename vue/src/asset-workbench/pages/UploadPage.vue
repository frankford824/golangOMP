<script setup lang="ts">
import { computed, ref } from 'vue'

import { uploadWorkbenchFile } from '@aw/features/upload/uploadFlow'
import { assetWorkbenchApi, type SubmissionFileRow } from '@aw/shared/api/assetWorkbenchApi'
import WorkbenchFilePreview from '@aw/shared/preview/WorkbenchFilePreview.vue'

type QueueStatus = 'queued' | 'uploading' | 'uploaded' | 'failed'

interface QueueItem {
  id: string
  file: File
  orderNo: string
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

const uploadedItems = computed(() => queue.value.filter((item) => item.status === 'uploaded'))
const canSubmit = computed(() => uploadedItems.value.length > 0 && !uploading.value && !submitting.value)
const totalPages = computed(() => queue.value.reduce((sum, item) => sum + item.pageCount, 0))

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
      difficultyClass: 'A',
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
        difficulty_class: item.difficultyClass,
        finalized: item.finalized,
        page_count: item.pageCount,
        item_count: 1,
        upload_session_ids: item.sessionId ? [item.sessionId] : [],
      })),
    })
    submittedFiles.value = detail.items.flatMap((item) => item.files)
    notice.value = '提交已创建'
    queue.value = queue.value.filter((item) => item.status !== 'uploaded')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '提交失败'
  } finally {
    submitting.value = false
  }
}

function removeItem(id: string) {
  queue.value = queue.value.filter((item) => item.id !== id)
}

function filenameWithoutExt(filename: string) {
  const dot = filename.lastIndexOf('.')
  return dot > 0 ? filename.slice(0, dot) : filename
}
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-heading">
      <div>
        <p class="aw-eyebrow">Submission items</p>
        <h2>成品上传中心</h2>
      </div>
      <div class="aw-button-row">
        <button class="aw-secondary-button" type="button" @click="openFilePicker">选择文件</button>
        <button class="aw-primary-button" type="button" :disabled="uploading || queue.length === 0" @click="uploadAll">
          上传队列
        </button>
      </div>
    </div>

    <input ref="inputRef" class="aw-visually-hidden" type="file" multiple @change="handleInput" />

    <div class="aw-dropzone" tabindex="0" @dragover.prevent @drop.prevent="handleDrop">
      <strong>拖拽文件到这里</strong>
      <span>文件名作为订单号，上传完成后生成 submission item。</span>
    </div>

    <p v-if="error" class="aw-inline-alert">{{ error }}</p>
    <p v-else-if="notice" class="aw-inline-alert">{{ notice }}</p>

    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <span>{{ queue.length }} 个文件</span>
        <span>{{ totalPages }} 页</span>
        <button type="button" :disabled="!canSubmit" @click="createSubmission">创建提交</button>
      </div>
      <div v-if="queue.length" class="aw-upload-list">
        <div v-for="item in queue" :key="item.id">
          <input v-model="item.orderNo" aria-label="订单号" />
          <select v-model="item.difficultyClass" aria-label="难度类">
            <option value="A">A</option>
            <option value="B">B</option>
            <option value="C">C</option>
            <option value="A+小夜灯">A+小夜灯</option>
          </select>
          <input v-model.number="item.pageCount" aria-label="页数" min="1" type="number" />
          <label class="aw-inline-check">
            <input v-model="item.finalized" type="checkbox" />
            定稿
          </label>
          <strong>{{ item.status === 'uploading' ? `${item.progress}%` : item.status }}</strong>
          <button type="button" :disabled="item.status === 'uploading'" @click="removeItem(item.id)">移除</button>
        </div>
      </div>
      <div v-else class="aw-empty-state">
        <h3>等待文件</h3>
        <p>上传完成后，提交时只冻结 worker type、岗级、难度类、基础价和大促毛额。</p>
      </div>
    </div>

    <div v-if="submittedFiles.length" class="aw-panel aw-panel--stage">
      <h3>提交预览</h3>
      <p class="aw-copy">源文件预览由 worker 异步生成；ready 后通过工作台 preview endpoint 注入展示 URL。</p>
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
