<!--
  本仓库未引入 Element Plus；实现为 Tailwind + 原生拖拽区 + BaseButton。
  若改用 Element Plus，可替换为：
  - 拖拽区：<el-upload drag :http-request="customUpload" :show-file-list="false" :limit="1">
  - 进度：<el-progress type="line" :percentage="percent" :status="progressStatus" />
  - 按钮：<el-button> 取消 / 重试
-->
<template>
  <div class="simple-upload w-full max-w-lg">
    <!-- 选择区：点击或拖拽（等价于 el-upload drag） -->
    <div
      v-if="!file"
      class="upload-dropzone flex cursor-pointer flex-col items-center justify-center rounded-xl border-2 border-dashed border-stone-300 bg-stone-50/80 px-6 py-10 text-center transition-colors hover:border-stone-400 hover:bg-stone-50"
      :class="{ 'border-stone-500 bg-stone-100': dragOver }"
      role="button"
      tabindex="0"
      @click="openPicker"
      @keydown.enter.prevent="openPicker"
      @keydown.space.prevent="openPicker"
      @dragenter.prevent="dragOver = true"
      @dragleave.prevent="onDragLeave"
      @dragover.prevent="dragOver = true"
      @drop.prevent="onDrop"
    >
      <input
        ref="inputRef"
        type="file"
        class="sr-only"
        :accept="accept"
        @change="onInputChange"
      />
      <span class="text-sm font-medium text-stone-700">点击或拖拽文件到此处</span>
      <span class="mt-1 text-xs text-stone-500">单文件上传</span>
    </div>

    <!-- 已选文件：文件名（截断 + title tooltip）+ 大小 -->
    <div v-else class="rounded-xl border border-stone-200 bg-white p-4 shadow-sm">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <p
            class="truncate text-sm font-medium text-stone-800"
            :title="file.name"
          >
            {{ file.name }}
          </p>
          <p class="mt-0.5 text-xs text-stone-500">{{ formattedSize }}</p>
        </div>
        <button
          v-if="phase === 'idle' || phase === 'error'"
          type="button"
          class="shrink-0 text-xs text-stone-500 underline decoration-stone-300 hover:text-stone-800"
          @click="clearFile"
        >
          移除
        </button>
      </div>

      <!-- 上传中 / 成功后最小停留：进度条（等价于 el-progress line） -->
      <div v-if="phase === 'uploading' || phase === 'holding'" class="mt-4">
        <div class="mb-1 flex items-center justify-between text-xs text-stone-600">
          <span>{{ holdLabel }}</span>
          <span class="tabular-nums">{{ displayPercent }}%</span>
        </div>
        <div class="h-2 w-full overflow-hidden rounded-full bg-stone-200">
          <div
            class="h-full rounded-full bg-stone-600 transition-[width] duration-150 ease-out"
            :class="phase === 'holding' ? 'bg-stone-500' : 'bg-stone-600'"
            :style="{ width: `${displayPercent}%` }"
          />
        </div>
      </div>

      <!-- 成功 -->
      <div
        v-if="phase === 'success'"
        class="mt-4 flex items-center gap-2 text-sm font-medium text-emerald-700"
      >
        <CheckCircle2 class="h-5 w-5 shrink-0 text-emerald-600" aria-hidden="true" />
        <span>上传成功</span>
      </div>

      <!-- 失败 -->
      <div v-if="phase === 'error'" class="mt-4 space-y-3">
        <p class="text-sm text-red-600">{{ errorMessage }}</p>
        <div class="flex flex-wrap gap-2">
          <BaseButton size="sm" variant="secondary" @click="retry">重试</BaseButton>
          <BaseButton size="sm" variant="ghost" @click="clearFile">重新选择</BaseButton>
        </div>
      </div>

      <!-- 操作：空闲时开始上传；上传中可取消 -->
      <div v-if="phase === 'idle' && file" class="mt-4">
        <BaseButton size="sm" variant="primary" @click="startUpload">开始上传</BaseButton>
      </div>
      <div v-if="phase === 'uploading' || phase === 'holding'" class="mt-4">
        <BaseButton size="sm" variant="ghost" @click="cancelUpload">取消上传</BaseButton>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * 单文件上传 + 「最小展示时长」：axios 已成功返回后仍保持 100% 与「处理中…」至少 minDisplayMs，
 * 避免小文件/快网速下进度条一闪而过。失败立即展示错误 + 重试，无假进度动画。
 */
import { computed, onBeforeUnmount, ref, shallowRef } from 'vue'
import axios, { type AxiosProgressEvent, isAxiosError, isCancel } from 'axios'
import { CheckCircle2 } from 'lucide-vue-next'
import BaseButton from '@/components/base/BaseButton.vue'

const props = withDefaults(
  defineProps<{
    /** 上传接口（占位默认 /api/upload，可按项目 vite proxy 调整） */
    action?: string
    /** 成功返回后，进度区至少保留的毫秒数（建议 800–1200） */
    minDisplayMs?: number
    inputAccept?: string
    /** form 字段名 */
    fieldName?: string
  }>(),
  {
    action: '/api/upload',
    minDisplayMs: 1000,
    inputAccept: undefined,
    fieldName: 'file',
  },
)

const emit = defineEmits<{
  success: [payload: unknown]
  error: [err: unknown]
}>()

type Phase = 'idle' | 'uploading' | 'holding' | 'success' | 'error'

const inputRef = ref<HTMLInputElement | null>(null)
const file = shallowRef<File | null>(null)
const phase = ref<Phase>('idle')
const percent = ref(0)
const errorMessage = ref('')
const dragOver = ref(false)

let abortController: AbortController | null = null
/** 上传开始时间，用于计算「最小展示时长」剩余等待 */
let uploadStartedAt = 0
/** holding 阶段结束定时器，组件卸载时需清理 */
let holdTimer: ReturnType<typeof setTimeout> | null = null

const accept = computed(() => props.inputAccept ?? undefined)

const formattedSize = computed(() => (file.value ? formatFileSize(file.value.size) : ''))

/** 展示用百分比：瞬间到 100% 也直接显示 100 */
const displayPercent = computed(() => Math.min(100, Math.max(0, percent.value)))

const holdLabel = computed(() =>
  phase.value === 'holding' ? '处理中…' : '上传中…',
)

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
}

function openPicker() {
  inputRef.value?.click()
}

function onInputChange(e: Event) {
  const input = e.target as HTMLInputElement
  const f = input.files?.[0]
  input.value = ''
  if (f) setFile(f)
}

function onDragLeave(e: DragEvent) {
  const related = e.relatedTarget as Node | null
  if (related && (e.currentTarget as HTMLElement).contains(related)) return
  dragOver.value = false
}

function onDrop(e: DragEvent) {
  dragOver.value = false
  const f = e.dataTransfer?.files?.[0]
  if (f) setFile(f)
}

function setFile(f: File) {
  clearTimersAndAbort()
  file.value = f
  phase.value = 'idle'
  percent.value = 0
  errorMessage.value = ''
}

function clearFile() {
  clearTimersAndAbort()
  file.value = null
  phase.value = 'idle'
  percent.value = 0
  errorMessage.value = ''
}

function clearTimersAndAbort() {
  if (holdTimer != null) {
    clearTimeout(holdTimer)
    holdTimer = null
  }
  abortController?.abort()
  abortController = null
}

/**
 * 在请求已成功结束后调用：若从开始上传到现在不足 minDisplayMs，则补足等待，
 * 期间保持 100% +「处理中…」，再进入 success。
 */
async function waitMinDisplaySinceStart(): Promise<void> {
  const minMs = Math.max(0, props.minDisplayMs)
  const elapsed = Date.now() - uploadStartedAt
  const remain = minMs - elapsed
  if (remain > 0) {
    await new Promise<void>((resolve) => {
      holdTimer = setTimeout(() => {
        holdTimer = null
        resolve()
      }, remain)
    })
  }
}

function onUploadProgress(ev: AxiosProgressEvent) {
  const total = ev.total ?? ev.loaded
  if (total <= 0) {
    percent.value = ev.loaded > 0 ? 100 : 0
    return
  }
  percent.value = Math.round((ev.loaded * 100) / total)
}

async function startUpload() {
  const f = file.value
  if (!f || phase.value === 'uploading' || phase.value === 'holding') return

  clearTimersAndAbort()
  errorMessage.value = ''
  percent.value = 0
  phase.value = 'uploading'
  uploadStartedAt = Date.now()
  abortController = new AbortController()

  const form = new FormData()
  form.append(props.fieldName, f)

  try {
    const res = await axios.post(props.action, form, {
      signal: abortController.signal,
      headers: { 'Content-Type': 'multipart/form-data' },
      onUploadProgress,
    })

    // 服务端已成功：进度条先到 100（onUploadProgress 通常已触发；双保险）
    percent.value = 100
    // 最小展示时长内保持 100% +「处理中…」，不立刻切成功态
    phase.value = 'holding'
    await waitMinDisplaySinceStart()
    phase.value = 'success'
    emit('success', res.data)
  } catch (err) {
    if (isCancel(err)) {
      phase.value = 'idle'
      percent.value = 0
      return
    }
    phase.value = 'error'
    percent.value = 0
    errorMessage.value = isAxiosError(err)
      ? String(err.response?.data?.message ?? err.message ?? '上传失败')
      : err instanceof Error
        ? err.message
        : '上传失败'
    emit('error', err)
  } finally {
    abortController = null
  }
}

function cancelUpload() {
  if (phase.value === 'holding') {
    clearTimersAndAbort()
    phase.value = 'idle'
    percent.value = 0
    return
  }
  if (phase.value === 'uploading') {
    abortController?.abort()
  }
}

function retry() {
  if (!file.value) return
  phase.value = 'idle'
  percent.value = 0
  errorMessage.value = ''
  void startUpload()
}

onBeforeUnmount(() => {
  clearTimersAndAbort()
})

defineExpose({
  startUpload,
  clearFile,
})
</script>
