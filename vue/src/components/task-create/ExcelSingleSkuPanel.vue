<template>
  <section class="excel-panel">
    <header class="excel-header">
      <div>
        <p class="excel-eyebrow">单 SKU</p>
        <p class="excel-title">Excel 辅助导入</p>
      </div>
      <span class="excel-status">{{ hasDraft ? '已解析' : '待上传' }}</span>
    </header>

    <ol class="excel-steps" aria-label="Excel 单任务导入步骤">
      <li
        v-for="step in stepItems"
        :key="step.value"
        class="excel-step"
        :class="{ 'is-active': activeStep === step.value, 'is-done': activeStep > step.value }"
      >
        <span class="step-index">{{ step.value }}</span>
        <span class="step-label">{{ step.label }}</span>
      </li>
    </ol>

    <div class="excel-actions-card">
      <button
        type="button"
        class="hh-btn hh-btn-primary"
        :disabled="downloading"
        @click="downloadTemplate"
      >
        {{ downloading ? '下载中...' : '下载模板' }}
      </button>

      <input
        ref="fileInput"
        type="file"
        accept=".xlsx,.xls"
        class="sr-only"
        @change="onFileChange"
      />

      <button type="button" class="hh-btn hh-btn-file" @click="openFilePicker">选择文件</button>

      <button
        type="button"
        class="hh-btn hh-btn-secondary"
        :disabled="!selectedFile || parsing"
        @click="parseFile"
      >
        {{ parsing ? '解析中...' : '解析预览' }}
      </button>

      <button v-if="hasDraft" type="button" class="hh-btn hh-btn-ghost" @click="reset">重新上传</button>
    </div>

    <div class="excel-meta">
      <p class="file-line">
        <span class="file-dot"></span>
        {{ selectedFileName }}
      </p>
      <p v-if="templateName" class="template-line">模板：{{ templateName }}</p>
      <p v-if="isPurchase" class="rule-line">
        每次上传仅创建 <strong>1 个</strong> 采购任务。<br />
        必填：产品款式编码、产品名称、数量、规格尺寸。<br />
        可选：备注。<br />
        本版 Excel 不支持参考图；如需参考图请在任务创建后于详情页上传。
      </p>
      <p v-else class="rule-line">
        每次上传仅创建 <strong>1 个</strong> 新款单 SKU 任务。<br />
        必填：产品款式编码、产品名称、设计要求。<br />
        可选：规格尺寸、材质、材质备注、备注。<br />
        参考图请在任务创建后于详情页上传（本版 Excel 不支持嵌入图）。
      </p>
    </div>

    <p v-if="errorText" class="excel-error">{{ errorText }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  excelAssistApi,
  normalizeSingleTaskDraft,
  type ExcelAssistSingleTaskType,
  type ExcelAssistViolation,
  type SingleTaskExcelDraft,
} from '@/services/api/excelAssistApi'
import { resolveApiUserMessage } from '@/utils/api-message-zh'

const props = withDefaults(
  defineProps<{
    taskType?: ExcelAssistSingleTaskType
  }>(),
  {
    taskType: 'new_product_development',
  },
)

const emit = defineEmits<{
  parsed: [payload: { draft: SingleTaskExcelDraft; violations: ExcelAssistViolation[] }]
  reset: []
}>()

const fileInput = ref<HTMLInputElement | null>(null)
const downloading = ref(false)
const parsing = ref(false)
const templateName = ref('')
const selectedFile = ref<File | null>(null)
const draft = ref<SingleTaskExcelDraft | null>(null)
const violations = ref<ExcelAssistViolation[]>([])
const errorText = ref('')

const stepItems = [
  { value: 1, label: '模板' },
  { value: 2, label: '说明' },
  { value: 3, label: '上传' },
  { value: 4, label: '预览' },
]

const hasDraft = computed(() => draft.value != null)
const activeStep = computed(() => {
  if (hasDraft.value) return 4
  if (selectedFile.value) return 3
  if (templateName.value) return 2
  return 1
})
const selectedFileName = computed(() => selectedFile.value?.name ?? '未选择任何文件')

const isPurchase = computed(() => props.taskType === 'purchase_task')

function openFilePicker(): void {
  fileInput.value?.click()
}

async function downloadTemplate(): Promise<void> {
  downloading.value = true
  errorText.value = ''
  try {
    const res = await excelAssistApi.downloadTemplate(props.taskType)
    const blob = res.data instanceof Blob ? res.data : new Blob([res.data as BlobPart])
    const disposition = String(res.headers?.['content-disposition'] ?? '')
    const match = disposition.match(/filename\*?=(?:UTF-8'')?"?([^";]+)/i)
    templateName.value = decodeURIComponent(match?.[1] ?? 'excel-assist-single-template.xlsx')
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = templateName.value
    a.click()
    URL.revokeObjectURL(url)
  } catch (err) {
    errorText.value = `模板下载失败：${resolveApiUserMessage(err)}`
  } finally {
    downloading.value = false
  }
}

function onFileChange(event: Event): void {
  const file = (event.target as HTMLInputElement).files?.[0] ?? null
  selectedFile.value = file
  draft.value = null
  violations.value = []
  errorText.value = ''
}

async function parseFile(): Promise<void> {
  if (!selectedFile.value) return
  parsing.value = true
  errorText.value = ''
  try {
    const res = await excelAssistApi.parseExcel(selectedFile.value, props.taskType)
    const raw = res.data as {
      data?: { draft?: SingleTaskExcelDraft; violations?: ExcelAssistViolation[] }
      draft?: SingleTaskExcelDraft
      violations?: ExcelAssistViolation[]
    }
    const data = raw.data && typeof raw.data === 'object' ? raw.data : raw
    draft.value = normalizeSingleTaskDraft(data.draft)
    violations.value = data.violations ?? []
    emit('parsed', { draft: draft.value, violations: violations.value })
    if (!draft.value.product_name && violations.value.length === 0) {
      errorText.value = '解析结果为空，请检查文件内容。'
    }
  } catch (err) {
    errorText.value = `文件解析失败：${resolveApiUserMessage(err)}`
  } finally {
    parsing.value = false
  }
}

function reset(): void {
  selectedFile.value = null
  draft.value = null
  violations.value = []
  errorText.value = ''
  if (fileInput.value) fileInput.value.value = ''
  emit('reset')
}
</script>

<style scoped>
.excel-panel {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  border: 1px solid #e6eaf0;
  border-radius: 0.875rem;
  background: #fff;
  padding: 0.75rem;
  font-size: 0.75rem;
}
.excel-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}
.excel-eyebrow {
  margin: 0 0 0.1rem;
  color: #8a94a3;
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.04em;
}
.excel-title {
  margin: 0;
  color: #171c22;
  font-size: 0.9rem;
  font-weight: 800;
}
.excel-status {
  flex: none;
  border-radius: 999px;
  background: #f4f5f7;
  padding: 0.18rem 0.55rem;
  color: #5b6573;
  font-size: 0.68rem;
  font-weight: 700;
}
.excel-steps {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.35rem;
  margin: 0;
  padding: 0;
  list-style: none;
}
.excel-step {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.3rem;
  border-radius: 0.5rem;
  padding: 0.25rem 0.35rem;
  color: #8a94a3;
}
.excel-step.is-active {
  background: #eef4ff;
  color: #2563eb;
}
.excel-step.is-done {
  color: #3d7a57;
}
.step-index {
  display: inline-flex;
  width: 1.1rem;
  height: 1.1rem;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: currentColor;
  color: #fff;
  font-size: 0.62rem;
  font-weight: 700;
}
.excel-step.is-active .step-index,
.excel-step.is-done .step-index {
  background: #2563eb;
}
.excel-actions-card {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}
.hh-btn {
  border-radius: 0.5rem;
  padding: 0.35rem 0.65rem;
  font-size: 0.72rem;
  font-weight: 600;
  cursor: pointer;
  border: 1px solid transparent;
}
.hh-btn-primary {
  background: #2563eb;
  color: #fff;
}
.hh-btn-secondary {
  background: #f4f5f7;
  color: #171c22;
  border-color: #e6eaf0;
}
.hh-btn-file {
  background: #fff;
  border-color: #d5dbe3;
}
.hh-btn-ghost {
  background: transparent;
  color: #5b6573;
}
.hh-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  border: 0;
}
.excel-meta {
  color: #5b6573;
  line-height: 1.45;
}
.file-line {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  margin: 0;
}
.file-dot {
  width: 0.35rem;
  height: 0.35rem;
  border-radius: 999px;
  background: #94a3b8;
}
.template-line,
.rule-line {
  margin: 0.25rem 0 0;
}
.excel-error {
  margin: 0;
  color: #b91c1c;
}
</style>
