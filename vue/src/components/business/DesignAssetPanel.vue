<template>
  <section class="grid w-full min-h-0 grid-cols-1 gap-3">
    <!-- 选文件后：本地假进度（非真实上传） -->
    <div
      v-if="pickAnimating"
      class="rounded-lg border border-slate-200 bg-white p-4 shadow-sm transition-opacity duration-200"
    >
      <div class="flex min-w-0 flex-col gap-1">
        <p class="min-w-0 text-sm font-medium text-slate-800" :title="pickFileTitle">
          <span class="truncate">{{ pickFileName }}</span>
          <span class="whitespace-nowrap text-slate-500"> · {{ pickFileSizeLabel }}</span>
        </p>
        <p class="text-xs text-slate-600">{{ DESIGN_UPLOAD_COPY.pickProcessing }}</p>
      </div>
      <div class="mt-3">
        <div class="mb-1 flex items-center justify-between text-xs text-slate-600">
          <span class="tabular-nums">{{ pickPercentRounded }}%</span>
        </div>
        <div class="h-2 w-full origin-left overflow-hidden rounded-full bg-slate-200">
          <div
            class="h-full w-full origin-left rounded-full bg-blue-600 transition-transform duration-150 ease-out"
            :style="{ transform: `scaleX(${pickScale})` }"
          />
        </div>
      </div>
    </div>

    <!-- 提交审核：真实上传进度 + 成功 / 失败（与 design.store 会话同步） -->
    <div
      v-if="deliverySession"
      class="rounded-lg border border-slate-200 bg-white p-4 shadow-sm transition-opacity duration-200"
    >
      <div class="flex min-w-0 flex-col gap-1">
        <p class="min-w-0 text-sm font-medium text-slate-800" :title="sessionFileTitle">
          <span class="truncate">{{ deliverySession.fileName }}</span>
          <span class="whitespace-nowrap text-slate-500">
            · {{ formatFileSizeBytes(deliverySession.fileSizeBytes) }}
          </span>
        </p>
        <p v-if="deliverySession.phase === 'uploading'" class="text-xs text-slate-600">
          {{ DESIGN_UPLOAD_COPY.uploadingToServer }}
          <template v-if="deliverySession.multipartLabel">
            · {{ deliverySession.multipartLabel }}
          </template>
        </p>
      </div>

      <div
        v-if="deliverySession.phase === 'uploading' || deliverySession.phase === 'success'"
        class="mt-3"
      >
        <div class="mb-1 flex items-center justify-between text-xs text-slate-600">
          <span class="tabular-nums">{{ serverPercentRounded }}%</span>
        </div>
        <div class="h-2 w-full origin-left overflow-hidden rounded-full bg-slate-200">
          <div
            class="h-full w-full origin-left rounded-full bg-blue-600 transition-transform duration-150 ease-out"
            :style="{ transform: `scaleX(${serverProgressScale})` }"
          />
        </div>
      </div>

      <div
        v-if="deliverySession.phase === 'success'"
        class="mt-3 flex items-center gap-2 text-sm font-medium text-emerald-700 transition-opacity duration-200"
      >
        <CheckCircle2 class="h-5 w-5 shrink-0 text-emerald-600" aria-hidden="true" />
        <span>{{ DESIGN_UPLOAD_COPY.uploadComplete }}</span>
      </div>

      <div
        v-if="deliverySession.phase === 'error'"
        class="mt-3 space-y-3 transition-opacity duration-200"
      >
        <div class="flex items-start gap-2 text-sm text-red-600">
          <XCircle class="mt-0.5 h-5 w-5 shrink-0" aria-hidden="true" />
          <span>{{ deliverySession.errorMessage }}</span>
        </div>
        <BaseButton size="sm" variant="secondary" @click="onRetrySubmit">
          {{ DESIGN_UPLOAD_COPY.retry }}
        </BaseButton>
      </div>
    </div>

    <template v-if="canUpload">
      <p v-if="uploadContextLabel" class="upload-context-label">{{ uploadContextLabel }}</p>
      <p class="upload-size-limit-hint">{{ DESIGN_UPLOAD_COPY.sizeLimitHint }}</p>
      <p v-if="useStagingBuckets" class="upload-size-limit-hint">{{ DESIGN_UPLOAD_COPY.batchSubmitHint }}</p>
      <p
        v-if="multiBucketSubmitStats && !designStore.isSubmitting && !deliverySession"
        class="batch-submit-summary"
      >
        待提交汇总：{{ multiBucketSubmitStats.products }} 个商品 · {{ multiBucketSubmitStats.files }} 个文件（提交审核时一并上传）
      </p>
      <div v-if="pendingFiles.length > 0" class="staging-area">
        <div class="staging-header">
          <span class="staging-label">
            {{ DESIGN_UPLOAD_COPY.pendingLabel }}（{{ pendingFiles.length }}）
          </span>
          <button
            type="button"
            class="staging-clear"
            :disabled="blockInteraction"
            @click="clearPendingFiles"
          >
            清空
          </button>
        </div>
        <div class="staging-grid">
          <div
            v-for="(pf, i) in pendingFiles"
            :key="i"
            class="staging-item"
            :class="{ 'staging-item-psd': !pf.previewUrl }"
          >
            <img
              v-if="pf.previewUrl"
              :src="pf.previewUrl"
              :alt="pf.fileName"
              class="staging-thumb"
            />
            <div
              v-else
            class="staging-file-card"
            >
            <FileIconFallback
              :name="pf.fileName"
              :size-text="formatFileSizeBytes(pf.file.size)"
            />
            </div>
            <button
              type="button"
              class="staging-remove"
              :disabled="blockInteraction"
              @click="removePendingFile(i)"
            >
              x
            </button>
          </div>
          <div
            class="staging-add"
            :class="{ 'cursor-not-allowed opacity-50': blockInteraction }"
            @dragover.prevent="onDragOver"
            @drop.prevent="onDrop"
            @click="!blockInteraction && fileInputRef?.click()"
          >
            <input
              ref="fileInputRef"
              type="file"
              class="sr-only"
              :accept="accept"
              multiple
              :disabled="blockInteraction"
              @change="handleFileChange"
            />
            <span class="staging-add-icon">+</span>
          </div>
        </div>
      </div>

      <div
        v-else
        class="upload-area"
        :class="{ 'cursor-not-allowed opacity-50': blockInteraction }"
        @click="!blockInteraction && openEmptyPicker()"
      >
        <input
          ref="emptyInputRef"
          type="file"
          class="sr-only"
          :accept="accept"
          multiple
          :disabled="blockInteraction"
          @change="handleFileChange"
        />
        <span class="upload-icon-text">+</span>
        <span class="upload-hint">上传本次设计稿（可多选）</span>
        <span v-if="readingLocal" class="upload-uploading">{{ DESIGN_UPLOAD_COPY.reading }}</span>
      </div>
      <p v-if="uploadPickError" class="upload-pick-error">{{ uploadPickError }}</p>
    </template>

    <div v-if="canSubmitAudit" class="submit-row flex flex-wrap items-center gap-3">
      <BaseButton
        variant="primary"
        :disabled="totalStagedFileCount === 0 || blockInteraction"
        @click="onSubmitAudit"
      >
        {{ props.submitButtonLabel || DESIGN_UPLOAD_COPY.submitAudit }}
      </BaseButton>
      <span v-if="totalStagedFileCount === 0" class="submit-hint text-sm text-slate-400">
        {{ DESIGN_UPLOAD_COPY.submitHintNeedFiles }}
      </span>
      <span
        v-else-if="designStore.isSubmitting"
        class="submit-hint text-sm text-slate-400"
      >
        {{ DESIGN_UPLOAD_COPY.submitHintUploading }}
      </span>
      <span v-else-if="submitError && !deliverySession" class="submit-hint text-sm text-red-600">
        {{ submitError }}
      </span>
      <span v-else class="submit-hint text-sm text-slate-400">
        {{ DESIGN_UPLOAD_COPY.submitHintIdle }}
      </span>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount, reactive } from 'vue'
import { storeToRefs } from 'pinia'
import { CheckCircle2, XCircle } from 'lucide-vue-next'
import BaseButton from '@/components/base/BaseButton.vue'
import FileIconFallback from '@/components/base/FileIconFallback.vue'
import { useDesignStore } from '@/stores/design.store'
import {
  DESIGN_UPLOAD_COPY,
  DESIGN_UPLOAD_TIMING,
  DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES,
  DESIGN_UPLOAD_MAX_FILE_SIZE_MB,
  designUploadTooLargeMessage,
} from '@/domain/copy/design-upload'
import { formatFileSizeBytes } from '@/domain/formatters/file-size'
import type { DesignDeliveryAuditBatch, DesignPendingFile } from '@/domain/types/design-upload'
import { shouldWarnForMissingBitmapDelivery } from '@/domain/design-submit-validation'
import {
  UPLOAD_ACCEPT_ATTRIBUTE,
  canPreviewUploadInline,
  getUploadFileExtension,
  isAllowedUploadFile,
} from '@/domain/constants/upload-types'

const EMPTY_PENDING: DesignPendingFile[] = []

const props = withDefaults(
  defineProps<{
    taskId: string
    canUpload: boolean
    canSubmitAudit: boolean
    accept?: string
    /** 批量 SKU：上传区标题，如「上传主 SKU 设计稿」 */
    uploadContextLabel?: string
    /** 写入 upload-sessions complete / submit-design 的 remark 后缀，沿用既有字段（单商品或作分桶后备） */
    deliveryRemarkSuffix?: string
    /** 当前 Tab 对应 SKU（批量任务必传）；单商品可不传 */
    activeSkuCode?: string
    /** 待提交区分桶键，建议 `taskId::${activeSkuCode}` 或 `taskId::__row_${index}` */
    stagingBucketKey?: string
    /** 按 SKU 生成交付备注后缀（批量任务） */
    getDeliveryRemarkSuffixBySku?: (skuCode: string) => string
    /** 由 bucketKey 解析 `target_sku_code`（处理无 SKU 时的 `__row_` 键） */
    resolveStagingTargetSku?: (bucketKey: string) => string | undefined
    /** 覆盖提交按钮文案（默认使用「提交审核」） */
    submitButtonLabel?: string
  }>(),
  {
    accept: UPLOAD_ACCEPT_ATTRIBUTE,
    uploadContextLabel: '',
    deliveryRemarkSuffix: '',
    activeSkuCode: '',
    stagingBucketKey: '',
    getDeliveryRemarkSuffixBySku: undefined,
    resolveStagingTargetSku: undefined,
    submitButtonLabel: '',
  },
)

const emit = defineEmits<{
  success: []
}>()

const designStore = useDesignStore()
const { session: deliverySession } = storeToRefs(designStore)
/** 多商品分桶提交：有 stagingBucketKey 即启用（与 activeSkuCode 解耦，避免无 SKU 时串桶） */
const isMultiSkuBucketMode = computed(() => !!props.stagingBucketKey?.trim())

const serverPercentRounded = computed(() => {
  const s = deliverySession.value
  if (!s) return 0
  return Math.min(100, Math.max(0, Math.round(s.displayPercent)))
})
const serverProgressScale = computed(() => {
  const s = deliverySession.value
  if (!s) return 0
  return Math.min(1, Math.max(0, s.displayPercent / 100))
})
const sessionFileTitle = computed(
  () =>
    deliverySession.value
      ? `${deliverySession.value.fileName} · ${formatFileSizeBytes(deliverySession.value.fileSizeBytes)}`
      : '',
)

/** 单商品：沿用单列表 */
const singlePending = ref<DesignPendingFile[]>([])
/** 批量多商品：按 stagingBucketKey（如 taskId::sku 或 taskId::__row_i）分桶 */
const pendingByBucket = reactive<Record<string, DesignPendingFile[]>>({})

const useStagingBuckets = computed(() => !!props.stagingBucketKey?.trim())
const currentBucketKey = computed(() => (props.stagingBucketKey || '').trim())

const pendingFiles = computed(() => {
  if (!useStagingBuckets.value) return singlePending.value
  return pendingByBucket[currentBucketKey.value] ?? EMPTY_PENDING
})

const totalStagedFileCount = computed(() => {
  if (!useStagingBuckets.value) return singlePending.value.length
  let n = 0
  for (const arr of Object.values(pendingByBucket)) n += arr?.length ?? 0
  return n
})

const multiBucketSubmitStats = computed((): { products: number; files: number } | null => {
  if (!useStagingBuckets.value) return null
  let products = 0
  let files = 0
  for (const arr of Object.values(pendingByBucket)) {
    if (arr?.length) {
      products += 1
      files += arr.length
    }
  }
  return files ? { products, files } : null
})

const readingLocal = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)
const emptyInputRef = ref<HTMLInputElement | null>(null)
const submitError = ref('')
const uploadPickError = ref('')

/** 选文件假进度 */
const pickAnimating = ref(false)
const pickPercent = ref(0)
const pickFileName = ref('')
const pickFileSizeLabel = ref('')
let pickRafId = 0

const pickPercentRounded = computed(() =>
  Math.min(100, Math.max(0, Math.round(pickPercent.value))),
)
const pickScale = computed(() => Math.min(1, Math.max(0, pickPercent.value / 100)))
const pickFileTitle = computed(
  () => `${pickFileName.value} · ${pickFileSizeLabel.value}`,
)

const blockInteraction = computed(
  () =>
    readingLocal.value ||
    pickAnimating.value ||
    (deliverySession.value != null && deliverySession.value.phase !== 'error'),
)

function cancelPickAnimation() {
  if (pickRafId) cancelAnimationFrame(pickRafId)
  pickRafId = 0
}

function runFakePickProgress(lastFile: File) {
  cancelPickAnimation()
  pickFileName.value = lastFile.name
  pickFileSizeLabel.value = formatFileSizeBytes(lastFile.size)
  pickAnimating.value = true
  pickPercent.value = 0
  const duration = DESIGN_UPLOAD_TIMING.pickFakeDurationMs
  const start = performance.now()
  function tick(now: number) {
    const t = Math.min(1, (now - start) / duration)
    const eased = 1 - (1 - t) ** 3
    pickPercent.value = eased * 100
    if (t < 1) {
      pickRafId = requestAnimationFrame(tick)
    } else {
      pickPercent.value = 100
      pickAnimating.value = false
      pickRafId = 0
    }
  }
  pickRafId = requestAnimationFrame(tick)
}

function getFileExtension(file: File): string {
  const ext = getUploadFileExtension(file.name).toUpperCase()
  return ext || 'FILE'
}

function revokePendingFilePreview(file: DesignPendingFile) {
  if (file.objectUrl) URL.revokeObjectURL(file.objectUrl)
}

function getMutablePendingList(): DesignPendingFile[] {
  if (!useStagingBuckets.value) return singlePending.value
  const key = currentBucketKey.value
  if (!pendingByBucket[key]) pendingByBucket[key] = []
  return pendingByBucket[key]
}

function clearAllStaging() {
  cancelPickAnimation()
  pickAnimating.value = false
  uploadPickError.value = ''
  singlePending.value.forEach(revokePendingFilePreview)
  singlePending.value = []
  for (const key of Object.keys(pendingByBucket)) {
    pendingByBucket[key]?.forEach(revokePendingFilePreview)
    delete pendingByBucket[key]
  }
}

/** 清空当前 Tab 对应桶（或单商品整表） */
function clearPendingFiles() {
  cancelPickAnimation()
  pickAnimating.value = false
  uploadPickError.value = ''
  if (!useStagingBuckets.value) {
    singlePending.value.forEach(revokePendingFilePreview)
    singlePending.value = []
    return
  }
  const arr = pendingByBucket[currentBucketKey.value]
  if (arr?.length) {
    arr.forEach(revokePendingFilePreview)
    pendingByBucket[currentBucketKey.value] = []
  }
}

function removePendingFile(index: number) {
  const list = !useStagingBuckets.value
    ? singlePending.value
    : (pendingByBucket[currentBucketKey.value] ?? [])
  const [removed] = list.splice(index, 1)
  if (removed) revokePendingFilePreview(removed)
}

function openEmptyPicker() {
  emptyInputRef.value?.click()
}

function onDragOver(e: DragEvent) {
  if (blockInteraction.value) return
  e.preventDefault()
}

function onDrop(e: DragEvent) {
  if (blockInteraction.value) return
  const files = e.dataTransfer?.files
  if (files?.length) readIntoPending(files)
}

function handleFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const files = input.files
  if (files?.length) readIntoPending(files)
  input.value = ''
}

function readIntoPending(files: FileList) {
  const pickedFiles = Array.from(files)
  const oversizedFiles = pickedFiles.filter((file) => file.size > DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES)
  const unsupportedFiles = pickedFiles.filter((file) => !isAllowedUploadFile(file.name))
  const validFiles = pickedFiles.filter(
    (file) =>
      isAllowedUploadFile(file.name) &&
      file.size > 0 &&
      file.size <= DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES,
  )
  const errors: string[] = []
  if (oversizedFiles.length > 0) {
    errors.push(
      oversizedFiles.length === 1
        ? designUploadTooLargeMessage(oversizedFiles[0]?.name)
        : `有 ${oversizedFiles.length} 个文件超过 ${DESIGN_UPLOAD_MAX_FILE_SIZE_MB}MB，已拒绝上传`,
    )
  }
  if (unsupportedFiles.length > 0) {
    errors.push(
      unsupportedFiles.length === 1
        ? `不支持的文件类型：${unsupportedFiles[0]?.name ?? ''}`
        : `有 ${unsupportedFiles.length} 个文件类型不受支持，已拒绝上传`,
    )
  }
  uploadPickError.value = errors.join('；')
  if (!validFiles.length) return
  readingLocal.value = true
  try {
    const target = getMutablePendingList()
    for (const file of validFiles) {
      const previewable = canPreviewUploadInline(file.name)
      if (previewable) {
        const objectUrl = URL.createObjectURL(file)
        target.push({
          file,
          fileName: file.name,
          previewUrl: objectUrl,
          extension: getFileExtension(file),
          objectUrl,
        })
      } else {
        target.push({
          file,
          fileName: file.name,
          previewUrl: null,
          extension: getFileExtension(file),
        })
      }
    }
    const last = validFiles[validFiles.length - 1]
    if (last) runFakePickProgress(last)
  } finally {
    readingLocal.value = false
  }
}

function stagingBucketSuffix(bucketKey: string): string {
  const prefix = `${props.taskId}::`
  if (!bucketKey.startsWith(prefix)) return ''
  return bucketKey.slice(prefix.length).trim()
}

function collectBatchesForSubmit(): DesignDeliveryAuditBatch[] {
  const keys = Object.keys(pendingByBucket).sort()
  const out: DesignDeliveryAuditBatch[] = []
  for (const key of keys) {
    const row = pendingByBucket[key]
    if (!row?.length) continue
    const skuCode =
      props.resolveStagingTargetSku?.(key) ??
      (() => {
        const s = stagingBucketSuffix(key)
        if (s.startsWith('__row_')) return ''
        return s
      })()
    const suffix = skuCode
      ? (props.getDeliveryRemarkSuffixBySku?.(skuCode) ?? props.deliveryRemarkSuffix ?? '')
      : (props.deliveryRemarkSuffix ?? '')
    out.push({
      files: row.map((p) => p.file),
      remarkSuffix: suffix,
      targetSkuCode: skuCode || undefined,
    })
  }
  return out
}

async function onSubmitAudit() {
  submitError.value = ''
  uploadPickError.value = ''
  const staged = useStagingBuckets.value
    ? Object.values(pendingByBucket).flatMap((arr) => arr ?? [])
    : singlePending.value
  const oversized = staged.find((item) => item.file.size > DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES)
  if (oversized) {
    submitError.value = designUploadTooLargeMessage(oversized.file.name)
    return
  }
  if (shouldWarnForMissingBitmapDelivery(staged.map((item) => item.fileName))) {
    const confirmed = window.confirm(
      '当前未包含位图交付图(JPG/PNG/WEBP)。审核员可能需要您补充预览图。确认继续提交？',
    )
    if (!confirmed) return
  }
  if (useStagingBuckets.value) {
    const batches = collectBatchesForSubmit()
    if (!batches.length) return
    const result = await designStore.submitDeliveryAuditBatches(props.taskId, batches)
    if (result.ok) {
      clearAllStaging()
      emit('success')
    } else {
      submitError.value = result.message
    }
    return
  }

  const files = singlePending.value.map((p) => p.file)
  if (!files.length) return
  const result = await designStore.submitDeliveryAuditSequence(props.taskId, files, {
    remarkSuffix: props.deliveryRemarkSuffix || undefined,
    // 单 SKU 任务不传 target_sku_code，避免命中后端归属校验后再降级重试。
    targetSkuCode: isMultiSkuBucketMode.value ? (props.activeSkuCode || undefined) : undefined,
  })
  if (result.ok) {
    clearAllStaging()
    emit('success')
  } else {
    submitError.value = result.message
  }
}

async function onRetrySubmit() {
  submitError.value = ''
  const result = await designStore.retryDeliveryAudit(props.taskId, {
    remarkSuffix: props.deliveryRemarkSuffix || undefined,
    // 与首次提交一致：仅多 SKU 分桶场景传 target_sku_code。
    targetSkuCode: isMultiSkuBucketMode.value ? (props.activeSkuCode || undefined) : undefined,
  })
  if (result.ok) {
    clearAllStaging()
    emit('success')
  } else if (!result.ok) {
    submitError.value = result.message
  }
}

watch(
  () => props.taskId,
  () => {
    clearAllStaging()
    submitError.value = ''
    designStore.clearSubmitState()
  },
)

onBeforeUnmount(() => {
  cancelPickAnimation()
  clearAllStaging()
})

defineExpose({
  clearPendingFiles,
})
</script>

<style scoped>
.upload-context-label {
  margin: 0 0 0.375rem;
  font-size: 0.8125rem;
  font-weight: 600;
  color: rgb(51 65 85);
}
.upload-size-limit-hint {
  margin: 0 0 0.375rem;
  font-size: 0.75rem;
  color: rgb(100 116 139);
}
.upload-pick-error {
  margin: 0.5rem 0 0;
  font-size: 0.75rem;
  color: rgb(220 38 38);
}
.staging-area {
  margin-top: 0.25rem;
  border: 1.5px dashed rgb(147 197 253);
  border-radius: 6px;
  padding: 0.625rem;
  background: rgb(240 247 255);
}
.staging-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.5rem;
}
.staging-label {
  font-size: 0.6875rem;
  font-weight: 600;
  color: rgb(29 78 216);
}
.staging-clear {
  font-size: 0.6875rem;
  color: rgb(148 163 184);
  background: none;
  border: none;
  cursor: pointer;
  padding: 0;
}
.staging-clear:hover:not(:disabled) {
  color: rgb(220 38 38);
}
.staging-clear:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.staging-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
  align-items: center;
}
.staging-item {
  position: relative;
  width: 56px;
  height: 56px;
}
.staging-item-psd {
  width: 72px;
  min-height: 64px;
  height: auto;
  align-self: flex-start;
}
.staging-thumb {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 4px;
  border: 1px solid rgb(191 219 254);
  display: block;
}
.staging-file-card {
  width: 100%;
  height: 100%;
  border-radius: 4px;
  border: 1px solid rgb(191 219 254);
  background: #fff;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 0.25rem;
  gap: 0.125rem;
  text-align: center;
}
.staging-file-ext {
  font-size: 0.625rem;
  font-weight: 700;
  color: rgb(29 78 216);
}
.staging-file-name {
  font-size: 0.5rem;
  line-height: 1.1;
  color: rgb(71 85 105);
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.staging-file-size {
  font-size: 0.5rem;
  line-height: 1.1;
  color: rgb(100 116 139);
  font-variant-numeric: tabular-nums;
}
.staging-remove {
  position: absolute;
  top: -4px;
  right: -4px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.5);
  color: #fff;
  border: none;
  font-size: 0.5rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
}
.staging-remove:hover:not(:disabled) {
  background: rgb(220 38 38);
}
.staging-remove:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.staging-add {
  position: relative;
  width: 56px;
  height: 56px;
  border: 1.5px dashed rgb(147 197 253);
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  background: #fff;
  transition: border-color 0.15s;
}
.staging-add:hover {
  border-color: rgb(24 144 255);
}
.staging-add-icon {
  font-size: 1.25rem;
  color: rgb(147 197 253);
  line-height: 1;
  pointer-events: none;
}
.upload-area {
  min-height: 2.125rem;
  border: 1px solid #e4e7ec;
  border-radius: var(--dv-r-control, 0.625rem);
  background: #f2f4f7;
  color: #344054;
  font-size: 0.75rem;
  font-weight: 700;
  padding: 0.45rem 0.8rem;
  cursor: pointer;
  transition: background 0.15s;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.35rem;
  width: fit-content;
  text-align: left;
}
.upload-area:hover {
  background: #e9edf3;
}
.upload-icon-text {
  font-size: 0.875rem;
  color: inherit;
  line-height: 1;
}
.upload-hint {
  font-size: 0.75rem;
  color: inherit;
  white-space: nowrap;
}
.upload-uploading {
  font-size: 0.75rem;
  color: #1d4ed8;
}
.submit-row {
  padding-top: 0.5rem;
  border-top: 1px solid rgb(241 245 249);
}
.batch-submit-summary {
  margin: 0 0 0.375rem;
  font-size: 0.75rem;
  line-height: 1.35;
  color: rgb(71 85 105);
}
</style>
