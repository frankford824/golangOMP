<template>
  <div class="upload-panel" :class="{ 'upload-panel-compact': compact }">
    <!-- 统一走 canonical asset upload-session -->
    <div
      class="upload-zone"
      :class="{ 'upload-zone-disabled': uploading, 'upload-zone-drag-active': isDragActive }"
      @click="openFilePicker"
      @dragenter.prevent="onDragEnter"
      @dragover.prevent="onDragOver"
      @dragleave.prevent="onDragLeave"
      @drop.prevent="handleDrop"
    >
      <input
        ref="fileInput"
        type="file"
        :accept="UPLOAD_ACCEPT_ATTRIBUTE"
        multiple
        class="hidden-input"
        :disabled="uploading"
        @change="onFileChange"
      />
      <span class="upload-plus">+</span>
      <p>{{ compact ? '上传参考图/附件' : `点击或拖拽上传参考图（任意格式，单文件不超过 ${REFERENCE_UPLOAD_MAX_FILE_SIZE_MB}MB）` }}</p>
      <p class="upload-hint">
        {{ uploadHintText }}
      </p>
    </div>
    <div v-if="compact" class="compact-upload-summary">
      <span>已上传 {{ fileRefs.length }} 个</span>
      <span>单文件 ≤{{ REFERENCE_UPLOAD_MAX_FILE_SIZE_MB }}MB</span>
    </div>
    <p v-if="limitError" class="limit-error">{{ limitError }}</p>
    <p v-if="uploadError" class="limit-error">{{ uploadError }}</p>

    <!-- 缩略图预览网格 -->
    <div v-if="fileRefs.length" class="thumb-grid">
      <div v-for="(item, i) in fileRefs" :key="i" class="thumb-item">
        <img
          v-if="shouldRenderImage(item, i)"
          :src="item.previewUrl"
          :alt="`参考图 ${i + 1}`"
          class="thumb-img"
          @error="onThumbError(i)"
        />
        <div v-else class="thumb-placeholder">
          <FileIconFallback
            :name="resolveFilename(item, i)"
            :url="item.localObjectUrl ? null : item.previewUrl"
          />
        </div>
        <button
          type="button"
          class="thumb-remove"
          :disabled="uploading"
          @click.stop="remove(i)"
        >
          x
        </button>
        <span class="thumb-label">参考图 {{ i + 1 }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import FileIconFallback from '@/components/base/FileIconFallback.vue'
import { toRelativeAssetUrl } from '@/utils/url'
import { formatUploadFailureMessage } from '@/utils/upload-errors'
import { uploadReferenceFileRef } from '@/services/upload/assetUploadFlow'
import { useTasksStore } from '@/stores/tasks'
import {
  REFERENCE_UPLOAD_MAX_FILE_SIZE_BYTES,
  REFERENCE_UPLOAD_MAX_FILE_SIZE_MB,
  isAcceptableReferenceFile,
  referenceFileTooLargeMessage,
} from '@/domain/constants/reference-upload'
import {
  UPLOAD_ACCEPT_ATTRIBUTE,
  canPreviewUploadInline,
  getUploadFileExtension,
  isAllowedUploadFile,
} from '@/domain/constants/upload-types'

interface RefItem {
  ref: Record<string, unknown> | string
  previewUrl: string
  localObjectUrl?: boolean
  loadFailed?: boolean
}

const props = withDefaults(defineProps<{
  modelValue?: (Record<string, unknown> | string)[]
  taskId?: string | null
  targetSkuCode?: string
  ownerModuleKey?: string
  uploadPolicy?: 'append_only' | 'replace' | string
  compact?: boolean
}>(), {
  modelValue: () => [],
  taskId: null,
  targetSkuCode: undefined,
  ownerModuleKey: undefined,
  uploadPolicy: undefined,
  compact: false,
})

const tasksStore = useTasksStore()

const fileInput = ref<HTMLInputElement | null>(null)
const fileRefs = ref<RefItem[]>([])
const limitError = ref('')
const uploadError = ref('')
const uploading = ref(false)
const isDragActive = ref(false)
const dragDepth = ref(0)
const retriedThumbKeys = ref(new Set<string>())

const emit = defineEmits<{
  'update:modelValue': [(Record<string, unknown> | string)[]]
  change: [(Record<string, unknown> | string)[]]
}>()

const uploadHintText = computed(() =>
  props.compact
    ? (uploading.value ? '上传中...' : '点击选择，或拖拽文件到此处')
    : (
  props.taskId?.trim()
    ? '已关联当前任务，上传的参考图会直接绑定到该任务。'
    : '创建前先上传参考图，任务创建成功后会自动关联到任务。'
    )
)

function emitRefs() {
  const refs = fileRefs.value.map((it) => it.ref)
  emit('update:modelValue', refs)
  emit('change', refs)
}

function resolvePreviewUrl(refItem: Record<string, unknown> | string): string {
  if (typeof refItem === 'string') {
    return toRelativeAssetUrl(refItem) ?? refItem
  }
  const rawUrl = refItem.download_url as string | undefined
  return rawUrl ? (toRelativeAssetUrl(rawUrl) ?? rawUrl) : ''
}

function syncFromModelValue() {
  const nextItems = props.modelValue
    .map((refItem) => {
      const matched = fileRefs.value.find((item) => item.ref === refItem || item.previewUrl === resolvePreviewUrl(refItem))
      if (matched) return matched
      const previewUrl = resolvePreviewUrl(refItem)
      if (!previewUrl) return null
      return { ref: refItem, previewUrl }
    })
    .filter((item): item is RefItem => item != null)

  const removedObjectUrls = fileRefs.value.filter((item) =>
    item.localObjectUrl && !nextItems.includes(item),
  )
  removedObjectUrls.forEach((item) => {
    if (item.previewUrl.startsWith('blob:')) {
      URL.revokeObjectURL(item.previewUrl)
    }
  })
  fileRefs.value = nextItems
}

function hasDraggedFiles(e: DragEvent): boolean {
  const types = e.dataTransfer?.types
  return Array.isArray(types) && types.includes('Files')
}

function openFilePicker() {
  if (uploading.value) return
  fileInput.value?.click()
}

function onDragEnter(e: DragEvent) {
  if (uploading.value || !hasDraggedFiles(e)) return
  dragDepth.value += 1
  isDragActive.value = true
}

function onDragOver(e: DragEvent) {
  if (uploading.value || !hasDraggedFiles(e)) return
  isDragActive.value = true
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = 'copy'
  }
}

function onDragLeave(e: DragEvent) {
  if (!hasDraggedFiles(e)) return
  dragDepth.value = Math.max(0, dragDepth.value - 1)
  if (dragDepth.value === 0) {
    isDragActive.value = false
  }
}

function handleDrop(e: DragEvent) {
  isDragActive.value = false
  dragDepth.value = 0
  if (uploading.value) return
  const files = e.dataTransfer?.files
  if (files?.length) processFiles(files)
}

function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  if (input.files?.length) processFiles(input.files)
  input.value = ''
}

async function processFiles(files: FileList) {
  if (uploading.value) return
  limitError.value = ''
  uploadError.value = ''
  const picked = Array.from(files)
  const oversized = picked.filter((f) => f.size > REFERENCE_UPLOAD_MAX_FILE_SIZE_BYTES)
  const unsupported = picked.filter((f) => !isAllowedUploadFile(f.name))
  const validFiles = picked.filter(
    (f) =>
      isAllowedUploadFile(f.name) &&
      isAcceptableReferenceFile(f) &&
      f.size <= REFERENCE_UPLOAD_MAX_FILE_SIZE_BYTES,
  )
  const errors: string[] = []
  if (oversized.length > 0) {
    errors.push(
      oversized.length === 1
        ? referenceFileTooLargeMessage(oversized[0]?.name)
        : `有 ${oversized.length} 个文件超过 ${REFERENCE_UPLOAD_MAX_FILE_SIZE_MB}MB，已拒绝上传`,
    )
  }
  if (unsupported.length > 0) {
    errors.push(
      unsupported.length === 1
        ? `不支持的文件类型：${unsupported[0]?.name ?? ''}`
        : `有 ${unsupported.length} 个文件类型不受支持，已拒绝上传`,
    )
  }
  limitError.value = errors.join('；')
  if (!validFiles.length) return

  uploading.value = true
  try {
    for (const file of validFiles) {
      const ref = await uploadReferenceFileRef(file, {
        taskId: props.taskId,
        targetSkuCode: props.targetSkuCode,
        ownerModuleKey: props.ownerModuleKey,
        uploadPolicy: props.uploadPolicy,
      })
      const preview = URL.createObjectURL(file)
      fileRefs.value.push({ ref, previewUrl: preview, localObjectUrl: true })
      emitRefs()
    }
  } catch (e) {
    uploadError.value = formatUploadFailureMessage('reference_upload', e)
  } finally {
    uploading.value = false
  }
}

function onThumbError(index: number) {
  const item = fileRefs.value[index]
  if (!item) return
  if (item.localObjectUrl) {
    item.loadFailed = true
    return
  }
  item.loadFailed = true
  const taskId = props.taskId?.trim()
  if (!taskId) return
  const refObj = item.ref
  const key = typeof refObj === 'object' && refObj !== null
    ? ((refObj as Record<string, unknown>).asset_id as string | undefined) ?? item.previewUrl
    : item.previewUrl
  if (retriedThumbKeys.value.has(key)) return
  retriedThumbKeys.value.add(key)
  tasksStore.refreshReferenceUrls(taskId)
}

function resolveFilename(item: RefItem, index: number): string {
  if (typeof item.ref === 'object' && item.ref !== null) {
    const name = item.ref.filename as string | undefined
    if (name?.trim()) return name
  }
  return `参考图 ${index + 1}`
}

function shouldRenderImage(item: RefItem, index: number): boolean {
  if (item.loadFailed) return false
  const name = resolveFilename(item, index)
  const ext = getUploadFileExtension(name)
  if (!ext) return true
  return canPreviewUploadInline(name)
}

function remove(index: number) {
  const item = fileRefs.value[index]
  if (item?.previewUrl?.startsWith('blob:')) {
    URL.revokeObjectURL(item.previewUrl)
  }
  fileRefs.value.splice(index, 1)
  emitRefs()
}

watch(
  () => props.modelValue,
  () => {
    syncFromModelValue()
  },
  { deep: true, immediate: true },
)
</script>

<style scoped>
.upload-panel {
  border: 1px dashed #cbd5e1;
  border-radius: 8px;
  padding: 0.75rem;
  background: #f8fafc;
}
.upload-panel-compact {
  border-style: solid;
  padding: 0.5rem;
  background: #fff;
  border-color: #dbe5f3;
  border-radius: 0.75rem;
}
.upload-zone {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.25rem;
  padding: 1.25rem;
  cursor: pointer;
  border-radius: 6px;
  transition: background 0.15s, border-color 0.15s;
}
.upload-zone-drag-active {
  background: #e8f1ff;
  box-shadow: inset 0 0 0 1px #3b82f6;
}
.upload-panel-compact .upload-zone {
  flex-direction: row;
  justify-content: flex-start;
  min-height: 2.75rem;
  padding: 0.35rem 0.6rem;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
  border-radius: 0.625rem;
}
.upload-panel-compact .upload-plus {
  font-size: 0.9rem;
  color: #475569;
}
.upload-panel-compact .upload-zone p {
  color: #334155;
  font-weight: 500;
}
.upload-panel-compact .upload-hint {
  margin-left: auto;
  color: #94a3b8;
  font-weight: 400;
  min-width: 0;
  text-align: right;
  line-height: 1.35;
  white-space: normal;
}
.compact-upload-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-top: 0.35rem;
  font-size: 0.6875rem;
  color: #64748b;
}
.upload-zone:hover:not(.upload-zone-disabled) { background: #f0f7ff; }
.upload-zone-disabled { cursor: not-allowed; opacity: 0.7; }
.hidden-input {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
.upload-plus { font-size: 1.375rem; color: #94a3b8; line-height: 1; }
.upload-zone p {
  margin: 0;
  font-size: 0.75rem;
  color: #94a3b8;
}
.upload-hint {
  font-size: 0.6875rem;
  color: #94a3b8;
  margin-top: 0.125rem;
}
.limit-error {
  margin: 0.5rem 0 0;
  font-size: 0.75rem;
  color: #dc2626;
}
.thumb-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(80px, 1fr));
  gap: 0.5rem;
  margin-top: 0.75rem;
}
.upload-panel-compact .thumb-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  margin-top: 0.45rem;
}
.upload-panel-compact .thumb-item {
  width: 44px;
}
.upload-panel-compact .thumb-label {
  display: none;
}
.thumb-item {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.25rem;
}
.thumb-img {
  width: 100%;
  aspect-ratio: 1;
  object-fit: cover;
  border-radius: 4px;
  border: 1px solid #e2e8f0;
  background: #fff;
  display: block;
}
.thumb-placeholder {
  width: 100%;
  aspect-ratio: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  border: 1px dashed #cbd5e1;
  background: #f1f5f9;
  padding: 0.25rem;
  overflow: hidden;
}
.thumb-placeholder-label {
  font-size: 0.5625rem;
  color: #94a3b8;
  text-align: center;
  word-break: break-all;
  line-height: 1.3;
}
.thumb-remove {
  position: absolute;
  top: 2px;
  right: 2px;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: rgba(0,0,0,0.45);
  color: #fff;
  border: none;
  font-size: 0.625rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  padding: 0;
}
.thumb-remove:hover:not(:disabled) { background: #dc2626; }
.thumb-remove:disabled { opacity: 0.5; cursor: not-allowed; }
.thumb-label {
  font-size: 0.625rem;
  color: #94a3b8;
  text-align: center;
}

/* Apple Music / iOS liquid glass upload panel skin. Style-only. */
.upload-panel,
.upload-panel-compact {
  border-color: rgba(148, 163, 184, 0.24);
  background: rgba(8, 13, 22, 0.70);
  color: #dce7f7;
}

.upload-zone,
.upload-panel-compact .upload-zone {
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(7, 12, 20, 0.72);
}

.upload-zone:hover:not(.upload-zone-disabled),
.upload-zone-drag-active {
  border-color: rgba(125, 211, 252, 0.44);
  background: rgba(34, 48, 71, 0.78);
  box-shadow: 0 0 0 3px rgba(100, 210, 255, 0.10);
}

.upload-plus,
.upload-panel-compact .upload-plus {
  color: #aee9ff;
}

.upload-zone p,
.upload-panel-compact .upload-zone p {
  color: #dce7f7;
}

.upload-hint,
.upload-panel-compact .upload-hint,
.compact-upload-summary,
.thumb-label,
.thumb-placeholder-label {
  color: #8fa0b8;
}

.thumb-img {
  border-color: rgba(148, 163, 184, 0.24);
  background: rgba(7, 12, 20, 0.82);
}

.thumb-placeholder {
  border-color: rgba(148, 163, 184, 0.24);
  background: rgba(10, 15, 24, 0.72);
}

.limit-error {
  color: #ffb4ad;
}
</style>
