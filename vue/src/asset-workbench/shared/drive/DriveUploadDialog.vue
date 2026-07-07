<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { CheckCircle2, FileUp, LoaderCircle, Upload, XCircle } from 'lucide-vue-next'

import {
  createDriveUploadQueue,
  driveUploadRelativePath,
  filesFromDriveDrop,
  isSafeDriveUploadPath,
  uploadDriveQueue,
  type DriveUploadQueueItem,
  type DriveUploadQueueStatus,
} from '@aw/shared/drive/useDriveUpload'
import { useUploadCenterStore } from '@aw/shared/drive/uploadCenter.store'
import IconfontActionIcon from '@aw/shared/icons/IconfontActionIcon.vue'

const props = defineProps<{
  open: boolean
  directoryId?: number
  directoryName: string
  difficultyClass?: string
  initialFiles?: File[]
  allowedFileTypes?: string[]
}>()

const emit = defineEmits<{
  close: []
  uploaded: [count: number]
}>()

const queue = ref<DriveUploadQueueItem[]>([])
const inputRef = ref<HTMLInputElement | null>(null)
const folderInputRef = ref<HTMLInputElement | null>(null)
const dragging = ref(false)
const busy = ref(false)
const error = ref('')
const uploadCenter = useUploadCenterStore()

const allowedFileTypes = computed(() =>
  (props.allowedFileTypes ?? [])
    .map((value) => value.trim().toLowerCase().replace(/^\.+/, ''))
    .filter(Boolean),
)

const acceptString = computed(() => {
  if (!allowedFileTypes.value.length) return ''
  return allowedFileTypes.value.map((value) => (value.includes('/') ? value : `.${value}`)).join(',')
})

const allowedLabel = computed(() => (allowedFileTypes.value.length ? allowedFileTypes.value.join('、') : '全部格式'))

watch(
  () => props.open,
  (open) => {
    if (open) {
      queue.value = createDriveUploadQueue(filterAllowedFiles(props.initialFiles ?? []))
      error.value = ''
      busy.value = false
    }
  },
  { immediate: true },
)

const targetLabel = computed(() => {
  const dir = props.directoryName || '未分类'
  return dir
})

const canUpload = computed(() => {
  if (busy.value) return false
  return queue.value.some((item) => item.status === 'queued' || item.status === 'failed')
})

function openPicker() {
  inputRef.value?.click()
}

function openFolderPicker() {
  folderInputRef.value?.click()
}

function handleInput(event: Event) {
  const target = event.target as HTMLInputElement
  enqueue(target.files)
  target.value = ''
}

function handleFolderInput(event: Event) {
  const target = event.target as HTMLInputElement
  enqueue(target.files)
  target.value = ''
}

async function handleDrop(event: DragEvent) {
  dragging.value = false
  try {
    enqueue(await filesFromDriveDrop(event.dataTransfer))
  } catch {
    error.value = '文件夹读取失败，请改用“选择文件夹”'
  }
}

function enqueue(files: FileList | File[] | null | undefined) {
  if (!files?.length) {
    error.value = '没有读取到可上传文件'
    return
  }
  error.value = ''
  queue.value = [...queue.value, ...createDriveUploadQueue(filterAllowedFiles(files))]
}

function filterAllowedFiles(files: FileList | File[]) {
  const values = Array.from(files).filter((file) => file.size > 0)
  const accepted = values.filter((file) => fileAllowed(file) && filePathAllowed(file))
  const rejectedCount = values.length - accepted.length
  if (rejectedCount > 0) {
    error.value = `已跳过 ${rejectedCount} 个不符合目录要求或格式限制的文件（允许：${allowedLabel.value}）`
  }
  return accepted
}

function fileAllowed(file: File) {
  const allowed = allowedFileTypes.value
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

function filePathAllowed(file: File) {
  return isSafeDriveUploadPath(driveUploadRelativePath(file))
}

function removeItem(id: string) {
  queue.value = queue.value.filter((item) => item.id !== id)
}

async function runUpload() {
  if (!canUpload.value) return
  busy.value = true
  error.value = ''
  const uploadableItems = queue.value.filter((item) => item.status === 'queued' || item.status === 'failed')
  uploadCenter.addItems(uploadableItems.map((item) => item.file), {
    source: 'drive-dialog',
    uploadDirectoryId: props.directoryId,
    uploadDirectoryName: props.directoryName,
    difficultyClass: props.difficultyClass,
    preserveIds: uploadableItems.map((item) => item.id),
  })
  try {
    const uploadedCount = await uploadDriveQueue(queue.value, {
      directoryId: props.directoryId,
      difficultyClass: props.difficultyClass,
      onItemChange: (item) => {
        queue.value = [...queue.value]
        uploadCenter.updateItem(item.id, {
          status: item.status,
          progress: item.progress,
          sessionId: item.sessionId,
          error: item.error || '',
          uploadDirectoryId: props.directoryId,
          uploadDirectoryName: props.directoryName,
          difficultyClass: props.difficultyClass,
        })
      },
    })
    uploadCenter.updateItems(
      queue.value.filter((item) => item.status === 'uploaded').map((item) => item.id),
      { status: 'submitted', progress: 100, error: '' },
    )
    emit('uploaded', uploadedCount)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '上传完成但归档失败'
    uploadCenter.updateItems(
      queue.value.filter((item) => item.status === 'uploaded').map((item) => item.id),
      { status: 'uploaded', error: error.value },
    )
  } finally {
    busy.value = false
  }
}

function statusIcon(status: DriveUploadQueueStatus) {
  if (status === 'uploaded') return CheckCircle2
  if (status === 'failed') return XCircle
  if (status === 'uploading') return LoaderCircle
  return FileUp
}
</script>

<template>
  <Teleport to="body">
    <section v-if="open" class="aw-token-scope aw-drive-upload" role="dialog" aria-modal="true" aria-label="上传到当前文件夹">
      <div class="aw-drive-upload__backdrop" @click="emit('close')" />
      <div class="aw-drive-upload__panel">
        <header class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">上传到当前文件夹</p>
            <h3>{{ targetLabel }}</h3>
          </div>
          <button class="aw-icon-action" type="button" aria-label="关闭" @click="emit('close')">
            <IconfontActionIcon name="close" :size="16" />
          </button>
        </header>

        <div
          class="aw-dropzone"
          :class="{ 'aw-dropzone--active': dragging }"
          tabindex="0"
          @dragenter.prevent="dragging = true"
          @dragover.prevent="dragging = true"
          @dragleave="dragging = false"
          @drop.prevent="handleDrop"
          @keydown.enter.prevent="openPicker"
          @keydown.space.prevent="openPicker"
        >
          <FileUp :size="28" aria-hidden="true" />
          <strong>拖拽文件或文件夹到此处</strong>
          <span>上传后自动归档到该目录 · 允许：{{ allowedLabel }}</span>
          <div class="aw-dropzone__actions">
            <button class="aw-secondary-button" type="button" @click="openPicker">选择文件</button>
            <button class="aw-secondary-button" type="button" @click="openFolderPicker">选择文件夹</button>
          </div>
        </div>

        <input ref="inputRef" class="aw-visually-hidden" type="file" multiple :accept="acceptString" aria-label="选择上传文件" @change="handleInput" />
        <input
          ref="folderInputRef"
          class="aw-visually-hidden"
          type="file"
          multiple
          webkitdirectory
          directory
          :accept="acceptString"
          aria-label="选择上传文件夹"
          @change="handleFolderInput"
        />

        <div v-if="queue.length" class="aw-drive-upload__queue">
          <article v-for="item in queue" :key="item.id" class="aw-drive-upload__row">
            <component :is="statusIcon(item.status)" :size="18" aria-hidden="true" />
            <div class="aw-drive-upload__row-body">
              <strong>{{ item.relativePath || item.file.name }}</strong>
              <span v-if="item.status === 'uploading'" class="aw-upload-progress"><span :style="{ width: `${item.progress}%` }" /></span>
              <small v-else-if="item.error" class="aw-upload-row__error">{{ item.error }}</small>
              <small v-else-if="item.status === 'uploaded'">已完成</small>
            </div>
            <button v-if="item.status !== 'uploading'" class="aw-link-button" type="button" @click="removeItem(item.id)">移除</button>
          </article>
        </div>

        <p v-if="error" class="aw-inline-alert">{{ error }}</p>

        <footer class="aw-drive-upload__foot">
          <button class="aw-secondary-button" type="button" :disabled="busy" @click="emit('close')">取消</button>
          <button class="aw-primary-button" type="button" :disabled="!canUpload" @click="runUpload">
            <Upload :size="16" aria-hidden="true" />
            {{ busy ? '上传中…' : '上传并归档' }}
          </button>
        </footer>
      </div>
    </section>
  </Teleport>
</template>
