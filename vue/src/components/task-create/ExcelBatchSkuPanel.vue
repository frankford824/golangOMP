<template>
  <section class="excel-panel">
    <header class="excel-header">
      <div>
        <p class="excel-eyebrow">批量 SKU</p>
        <p class="excel-title">Excel 四步导入</p>
      </div>
      <span class="excel-status">{{ preview.length > 0 ? `${preview.length} 行` : '待上传' }}</span>
    </header>

    <ol class="excel-steps" aria-label="Excel 批量导入步骤">
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
        accept=".xlsx,.xls,.csv"
        class="sr-only"
        @change="onFileChange"
      />

      <button
        type="button"
        class="hh-btn hh-btn-file"
        @click="openFilePicker"
      >
        选择文件
      </button>

      <button
        type="button"
        class="hh-btn hh-btn-secondary"
        :disabled="!selectedFile || parsing"
        @click="parseFile"
      >
        {{ parsing ? '解析中...' : '解析预览' }}
      </button>

      <button
        v-if="preview.length > 0"
        type="button"
        class="hh-btn hh-btn-ghost"
        @click="reset"
      >
        重新上传
      </button>
    </div>

    <div class="excel-meta">
      <p class="file-line">
        <span class="file-dot"></span>
        {{ selectedFileName }}
      </p>
      <p v-if="templateName" class="template-line">模板：{{ templateName }}</p>
      <p class="rule-line">
        请按模板填写后上传，最多支持 200 行。<br>
        必填：产品名称、设计要求；可选：产品款式编码、参考图。<br>
        参考图请放在对应产品所在行。
      </p>
    </div>

    <div v-if="!props.hidePreview && preview.length > 0" class="preview-card">
      <div class="preview-header">
        <p>解析预览</p>
        <span>合计 {{ preview.length }} 行 · 错误 {{ violations.length }} 条</span>
      </div>
      <div class="preview-table-wrap">
        <table class="preview-table">
          <thead>
            <tr>
              <th>行</th>
              <th>产品名</th>
              <th>设计要求</th>
              <th>产品款式编码</th>
              <th>参考图</th>
              <th>错误</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(row, idx) in preview"
              :key="idx"
              :class="{ 'has-error': rowErrors(idx + 1).length }"
            >
              <td>{{ idx + 1 }}</td>
              <td>{{ row.product_name || '—' }}</td>
              <td class="cell-ellipsis">{{ row.design_requirement || '—' }}</td>
              <td>{{ row.product_i_id || '—' }}</td>
              <td>
                <div v-if="row.reference_file_refs?.length" class="ref-thumbs">
                  <span
                    v-for="ref in row.reference_file_refs"
                    :key="ref.ref_id"
                    class="ref-thumb-item"
                    :title="ref.filename"
                  >
                    <img
                      v-if="isImage(ref.mime_type)"
                      :src="ref.download_url"
                      :alt="ref.filename"
                      class="ref-thumb-img"
                    />
                    <span v-else class="ref-thumb-file">{{ ref.filename }}</span>
                  </span>
                </div>
                <span v-else>—</span>
              </td>
              <td>
                <span v-for="err in rowErrors(idx + 1)" :key="err.column + err.code" class="err-tag">
                  {{ err.column }} · {{ err.message || err.code }}
                </span>
                <span v-if="rowErrors(idx + 1).length === 0">—</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <p v-if="errorText" class="excel-error">{{ errorText }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { batchSkuApi, normalizeBatchPreviewRow } from '@/services/api/batchSkuApi'
import type { BatchPreviewRow, BatchViolation } from '@/services/api/batchSkuApi'
import { resolveApiUserMessage } from '@/utils/api-message-zh'

type PreviewRow = BatchPreviewRow
type Violation = BatchViolation

const emit = defineEmits<{
  parsed: [payload: { preview: PreviewRow[]; violations: Violation[] }]
  reset: []
}>()

const props = withDefaults(
  defineProps<{
    taskType?: 'new_product_development' | 'purchase_task'
    hidePreview?: boolean
  }>(),
  {
    taskType: 'new_product_development',
    hidePreview: false,
  },
)

const fileInput = ref<HTMLInputElement | null>(null)
const downloading = ref(false)
const parsing = ref(false)
const templateName = ref('')
const selectedFile = ref<File | null>(null)
const preview = ref<PreviewRow[]>([])
const violations = ref<Violation[]>([])
const errorText = ref('')

const stepItems = [
  { value: 1, label: '模板' },
  { value: 2, label: '说明' },
  { value: 3, label: '上传' },
  { value: 4, label: '预览' },
]

const activeStep = computed(() => {
  if (preview.value.length > 0) return 4
  if (selectedFile.value) return 3
  if (templateName.value) return 2
  return 1
})
const selectedFileName = computed(() => selectedFile.value?.name ?? '未选择任何文件')

function openFilePicker(): void {
  fileInput.value?.click()
}

function rowErrors(row: number): Violation[] {
  return violations.value.filter((v) => v.row === row)
}

async function downloadTemplate(): Promise<void> {
  downloading.value = true
  errorText.value = ''
  try {
    const res = await batchSkuApi.downloadTemplate(props.taskType)
    const blob = res.data instanceof Blob ? res.data : new Blob([res.data as BlobPart])
    const disposition = String(res.headers?.['content-disposition'] ?? '')
    const match = disposition.match(/filename\\*?=(?:UTF-8'')?\"?([^\";]+)/i)
    templateName.value = decodeURIComponent(match?.[1] ?? 'batch-sku-template.xlsx')
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
  preview.value = []
  violations.value = []
  errorText.value = ''
}

function isImage(mimeType: string): boolean {
  return mimeType.startsWith('image/')
}

async function parseFile(): Promise<void> {
  if (!selectedFile.value) return
  parsing.value = true
  errorText.value = ''
  try {
    const res = await batchSkuApi.parseExcel(selectedFile.value, props.taskType)
    const raw = res.data as {
      data?: { preview?: PreviewRow[]; violations?: Violation[] }
      preview?: PreviewRow[]
      violations?: Violation[]
    }
    const data = raw.data && typeof raw.data === 'object' ? raw.data : raw
    const rows = data.preview ?? []
    preview.value = rows.map((row) => normalizeBatchPreviewRow(row))
    violations.value = data.violations ?? []
    emit('parsed', { preview: preview.value, violations: violations.value })
    if (preview.value.length === 0) {
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
  preview.value = []
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
  gap: 0.28rem;
  border-radius: 999px;
  background: #f4f5f7;
  padding: 0.25rem 0.35rem;
  color: #8a94a3;
}
.excel-step.is-active {
  background: #22272e;
  color: #fff;
  box-shadow: 0 4px 10px rgba(34, 39, 46, 0.16);
}
.excel-step.is-done {
  background: #edf5ff;
  color: #2f80ed;
}
.step-index {
  display: inline-flex;
  width: 1rem;
  height: 1rem;
  flex: none;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.9);
  color: inherit;
  font-size: 0.6rem;
  font-weight: 800;
}
.step-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.65rem;
  font-weight: 800;
}
.excel-actions-card {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.45rem;
  border-radius: 0.75rem;
  border: 1px solid #e6eaf0;
  background: #f7f8fa;
  padding: 0.55rem;
}
.hh-btn {
  display: inline-flex;
  min-height: 2rem;
  align-items: center;
  justify-content: center;
  border-radius: 0.625rem;
  border: 1px solid transparent;
  padding: 0.35rem 0.65rem;
  font-size: 0.72rem;
  font-weight: 800;
  line-height: 1;
  transition: transform 0.15s ease, background 0.15s ease, border-color 0.15s ease, opacity 0.15s ease;
}
.hh-btn:not(:disabled):active {
  transform: scale(0.98);
}
.hh-btn:disabled {
  cursor: not-allowed;
  opacity: 0.52;
}
.hh-btn-primary {
  background: #22272e;
  color: #fff;
  box-shadow: 0 4px 12px rgba(34, 39, 46, 0.18);
}
.hh-btn-primary:not(:disabled):hover {
  background: #171c22;
}
.hh-btn-file,
.hh-btn-secondary,
.hh-btn-ghost {
  border-color: #e1e6ee;
  background: #fff;
  color: #22272e;
}
.hh-btn-file:not(:disabled):hover,
.hh-btn-secondary:not(:disabled):hover,
.hh-btn-ghost:not(:disabled):hover {
  border-color: #cbd5e1;
  background: #f4f5f7;
}
.hh-btn-ghost {
  grid-column: 1 / -1;
  color: #5b6573;
}
.excel-meta {
  display: flex;
  flex-direction: column;
  gap: 0.22rem;
  border-radius: 0.7rem;
  background: #fafbfc;
  padding: 0.5rem 0.6rem;
  color: #6b7280;
}
.file-line,
.template-line,
.rule-line {
  margin: 0;
}
.file-line {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  color: #22272e;
  font-weight: 700;
  word-break: break-all;
}
.file-dot {
  width: 0.42rem;
  height: 0.42rem;
  flex: none;
  border-radius: 999px;
  background: #cbd5e1;
}
.preview-card {
  border: 1px solid #e6eaf0;
  border-radius: 0.75rem;
  background: #fff;
  padding: 0.6rem;
}
.preview-header {
  display: flex;
  justify-content: space-between;
  gap: 0.5rem;
  margin-bottom: 0.4rem;
}
.preview-header p,
.preview-header span {
  margin: 0;
}
.preview-header p {
  color: #171c22;
  font-weight: 800;
}
.preview-header span {
  color: #8a94a3;
}
.preview-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.68rem;
}
.preview-table th {
  background: #f4f5f7;
  color: #5b6573;
  font-weight: 800;
}
.preview-table th,
.preview-table td {
  padding: 0.28rem 0.35rem;
  text-align: left;
  border-top: 1px solid #eef1f5;
}
.preview-table-wrap {
  overflow-x: auto;
}
.preview-table tr.has-error {
  background: #fff1f2;
  color: #b91c1c;
}
.cell-ellipsis {
  max-width: 8rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ref-thumbs {
  display: flex;
  gap: 0.25rem;
  flex-wrap: wrap;
  max-width: 7rem;
}
.ref-thumb-item {
  display: inline-flex;
  align-items: center;
}
.ref-thumb-img {
  width: 1.75rem;
  height: 1.75rem;
  object-fit: cover;
  border-radius: 0.25rem;
  border: 1px solid #e6eaf0;
}
.ref-thumb-file {
  max-width: 4rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.6rem;
  color: #5b6573;
  background: #f4f5f7;
  border-radius: 0.25rem;
  padding: 0.1rem 0.25rem;
}
.err-tag {
  display: inline-block;
  margin-right: 0.2rem;
}
.excel-error {
  margin: 0;
  color: #dc2626;
  font-size: 0.72rem;
}
.sr-only {
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

/* Phase 4: light admin Excel batch panel skin. Style-only. */
.excel-panel {
  border-color: #e5e7eb;
  background: #ffffff;
  color: #111827;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.06);
}

.excel-eyebrow,
.excel-status,
.preview-header span,
.rule-line,
.template-line {
  color: #6b7280;
}

.excel-title,
.preview-header p,
.file-line {
  color: #111827;
}

.excel-status,
.excel-step,
.excel-actions-card,
.excel-meta,
.preview-card {
  border: 1px solid #e5e7eb;
  background: #f9fafb;
}

.excel-step.is-active {
  border-color: #93c5fd;
  background: #eff6ff;
  color: #2563eb;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.1);
}

.excel-step.is-done {
  border-color: #86efac;
  background: #f0fdf4;
  color: #15803d;
}

.step-index {
  background: #e5e7eb;
  color: #374151;
}

.hh-btn-primary {
  border-color: #2563eb;
  background: #2563eb;
  color: #fff;
  box-shadow: none;
}

.hh-btn-primary:not(:disabled):hover {
  background: #1d4ed8;
  border-color: #1d4ed8;
}

.hh-btn-file,
.hh-btn-secondary,
.hh-btn-ghost {
  border-color: #d1d5db;
  background: #ffffff;
  color: #374151;
}

.hh-btn-file:not(:disabled):hover,
.hh-btn-secondary:not(:disabled):hover,
.hh-btn-ghost:not(:disabled):hover {
  border-color: #93c5fd;
  background: #f9fafb;
  color: #111827;
}

.file-dot {
  background: #2563eb;
  box-shadow: none;
}

.preview-table th {
  background: #f3f4f6;
  color: #374151;
}

.preview-table td {
  color: #111827;
}

.preview-table th,
.preview-table td {
  border-top-color: #e5e7eb;
}

.preview-table tr.has-error {
  background: #fef2f2;
  color: #b91c1c;
}

.ref-thumb-img {
  border-color: #e5e7eb;
  background: #ffffff;
}

.ref-thumb-file {
  background: #eff6ff;
  color: #1d4ed8;
}

.excel-error {
  color: #b91c1c;
}
</style>
