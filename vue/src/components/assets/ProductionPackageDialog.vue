<template>
  <Teleport to="body">
    <div v-if="open" class="package-layer" @keydown.esc="close">
      <button class="package-backdrop" aria-label="关闭生产打包" @click="close" />
      <section class="package-dialog" role="dialog" aria-modal="true" aria-labelledby="production-package-title">
        <header>
          <div>
            <p>统一生产出口</p>
            <h2 id="production-package-title">生产打包</h2>
          </div>
          <button type="button" class="close-button" aria-label="关闭" @click="close">×</button>
        </header>

        <div class="package-body">
          <section class="package-section">
            <div class="package-copy">
              <h3>批量查询并生成统一生产包</h3>
              <p>
                每行一个
                SKU，重复编码会按原行数重复出包。只取当前最终成品；单图直接放文件，套装按“编码+商品名称”放入一个文件夹。
              </p>
            </div>
            <label class="wide-field">
              <span>每行一个 SKU（保留重复行）</span>
              <textarea v-model="termsText" rows="7" placeholder="HSC34548&#10;GK000804" />
            </label>
            <div class="field-grid">
              <label
                ><span>生产文件格式</span
                ><select v-model="packageFormat" @change="clearManifest">
                  <option value="tif">TIF / TIFF（镂空生产）</option>
                  <option value="jpg">JPG</option>
                  <option value="png">PNG</option>
                  <option value="jpg_png">JPG / PNG（自动取一种）</option>
                  <option value="image">生产图片（自动取一种）</option>
                </select></label
              >
              <label class="file-picker compact-picker">
                <input type="file" accept=".xlsx,.xls" :disabled="busy" @change="handleExcelFile" />
                <span>{{ excelFileName || '或上传 XLS / XLSX 表格' }}</span>
              </label>
            </div>
            <div class="action-row">
              <button type="button" class="primary-button" :disabled="busy" @click="buildManualPackage">
                {{ busy ? '服务端打包中…' : '生成并下载生产 ZIP' }}
              </button>
              <button
                type="button"
                class="secondary-button"
                :disabled="!downloadUrl"
                @click="downloadExcelPackage"
              >
                {{ downloadUrl ? '再次下载生产 ZIP' : '生成后可再次下载' }}
              </button>
              <button v-if="excelManifest" type="button" class="quiet-button" @click="exportExcelReport">
                导出匹配结果 Excel
              </button>
            </div>
            <p class="download-hint">生成完成后会自动下载；如果浏览器拦截或需要重复下载，可点击“再次下载生产 ZIP”。</p>
            <div v-if="excelManifest" class="summary-grid">
              <span
                ><small>匹配行</small><strong>{{ excelManifest.success_count }}</strong></span
              >
              <span
                ><small>生产文件</small><strong>{{ excelManifest.total_files }}</strong></span
              >
              <span
                ><small>失败行</small><strong>{{ excelManifest.failure_count }}</strong></span
              >
              <span
                ><small>套装目录</small><strong>{{ excelSetCount }}</strong></span
              >
            </div>
            <div v-if="excelManifest?.failures?.length" class="failure-list">
              <strong>异常明细</strong>
              <p v-for="(failure, index) in excelManifest.failures.slice(0, 30)" :key="index">
                {{ failure.sku_code || failure.sku_name || `第 ${failure.row_number || '-'} 行` }}：{{
                  failure.message || failure.reason
                }}
              </p>
            </div>
          </section>
        </div>

        <footer>
          <span :class="{ error: !!error }">{{ error || status || '等待操作' }}</span>
          <button type="button" class="quiet-button" @click="close">关闭</button>
        </footer>
      </section>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import {
  assetsApi,
  type AssetExcelPackageFormat,
  type AssetExcelPackageJob,
  type AssetExcelPackageManifest,
  type AssetExcelPackageRow,
} from '@/services/api/assetsApi'

defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()
const termsText = ref('')
const packageFormat = ref<AssetExcelPackageFormat>('tif')
const excelFileName = ref('')
const excelManifest = ref<AssetExcelPackageManifest | null>(null)
const busy = ref(false)
const status = ref('')
const error = ref('')
const downloadUrl = ref('')
const downloadFilename = ref('生产打包.zip')
let jobController: AbortController | null = null
const excelSetCount = computed(
  () => new Set((excelManifest.value?.items || []).map((item) => item.package_folder).filter(Boolean)).size,
)

function close() {
  jobController?.abort()
  jobController = null
  busy.value = false
  emit('close')
}

onBeforeUnmount(() => jobController?.abort())

function unwrap<T>(response: { data?: { data?: T } | T }): T | undefined {
  const body = response.data
  return body && typeof body === 'object' && 'data' in body ? body.data : (body as T | undefined)
}

function clearManifest() {
  excelManifest.value = null
	downloadUrl.value = ''
  status.value = ''
  error.value = ''
}

function manualRows(): AssetExcelPackageRow[] {
  return termsText.value
    .split(/[\n,，;；\s]+/)
    .map((item) => item.trim())
    .filter(Boolean)
    .map((skuCode, index) => ({
      row_number: index + 1,
      order_no: skuCode,
      sku_code: skuCode,
      quantity: 1,
    }))
}

async function buildManualPackage() {
  const rows = manualRows()
  if (!rows.length) {
    error.value = '请至少输入一个 SKU 编码，或上传 Excel。'
    return
  }
  excelManifest.value = null
  downloadUrl.value = ''
  busy.value = true
  error.value = ''
  excelFileName.value = ''
  status.value = `正在按 ${rows.length} 行生成生产清单…`
  try {
		jobController?.abort()
		jobController = new AbortController()
    const response = await assetsApi.createExcelPackageJob(rows, packageFormat.value, jobController.signal)
		const job = unwrap(response)
		if (!job?.job_id) throw new Error('后端未返回打包任务。')
		await pollPackageJob(job.job_id, jobController.signal)
		downloadExcelPackage()
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '生产清单查询失败。'
  } finally {
    busy.value = false
  }
}

async function handleExcelFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  excelFileName.value = file.name
  excelManifest.value = null
  downloadUrl.value = ''
  busy.value = true
  error.value = ''
  termsText.value = ''
  status.value = '正在解析 Excel 并生成统一生产清单…'
  try {
		jobController?.abort()
		jobController = new AbortController()
    const response = await assetsApi.createExcelPackageFileJob(file, packageFormat.value, jobController.signal)
		const job = unwrap(response)
		if (!job?.job_id) throw new Error('后端未返回打包任务。')
		await pollPackageJob(job.job_id, jobController.signal)
		downloadExcelPackage()
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : 'Excel 生产打包解析失败。'
  } finally {
    busy.value = false
    input.value = ''
  }
}

async function downloadExcelPackage() {
	if (!downloadUrl.value) return
	const link = document.createElement('a')
	link.href = downloadUrl.value
	link.download = downloadFilename.value
	link.rel = 'noopener'
	link.click()
}

async function pollPackageJob(jobId: string, signal: AbortSignal) {
	for (;;) {
		if (signal.aborted) throw new DOMException('已取消', 'AbortError')
		const response = await assetsApi.getExcelPackageJob(jobId, signal)
		const job = unwrap<AssetExcelPackageJob>(response)
		if (!job) throw new Error('无法读取打包任务状态。')
		status.value = job.status === 'queued' ? '打包任务已排队…' : `服务端打包中：${job.processed_count}/${job.total_count}`
		if (job.status === 'failed' || job.status === 'expired') throw new Error(job.error_message || '生产打包任务失败。')
		if (job.status === 'succeeded') {
			excelManifest.value = job.manifest || null
			downloadUrl.value = job.download_url || ''
			downloadFilename.value = job.filename || '生产打包.zip'
			if (!downloadUrl.value || !excelManifest.value) throw new Error('打包完成，但下载信息不完整。')
			status.value = `打包完成：${excelManifest.value.total_files} 个文件，可直接下载。`
			return
		}
		await new Promise<void>((resolve, reject) => {
			const timer = window.setTimeout(resolve, 2000)
			signal.addEventListener('abort', () => { window.clearTimeout(timer); reject(new DOMException('已取消', 'AbortError')) }, { once: true })
		})
	}
}

async function exportRows(filename: string, rows: Array<Record<string, string | number>>) {
  const ExcelJS = await import('exceljs')
  const workbook = new ExcelJS.Workbook()
  const sheet = workbook.addWorksheet('生产打包结果')
  const keys = Array.from(new Set(rows.flatMap((row) => Object.keys(row))))
  sheet.columns = keys.map((key) => ({ header: key, key, width: 24 }))
  rows.forEach((row) => sheet.addRow(row))
  const bytes = await workbook.xlsx.writeBuffer()
  const blob = new Blob([bytes], {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
}

function exportExcelReport() {
  const manifest = excelManifest.value
  if (!manifest) return
  const successRows = manifest.items.map((item) => ({
    行号: item.row_number || '',
    订单号: item.order_no,
    SKU: item.sku_code,
    文件名: item.filename,
    数量: item.quantity,
    来源: item.source_type || '',
    套装目录: item.package_folder || '',
    状态: '已匹配',
  }))
  const failureRows = (manifest.failures || []).map((item) => ({
    行号: item.row_number || '',
    订单号: item.order_no || '',
    SKU: item.sku_code || item.sku_name || '',
    文件名: '',
    数量: item.quantity || '',
    来源: '',
    套装目录: '',
    状态: item.message || item.reason,
  }))
  void exportRows('生产打包-Excel匹配结果.xlsx', [...successRows, ...failureRows])
}
</script>

<style scoped>
.package-layer {
  position: fixed;
  inset: 0;
  z-index: 120;
  display: grid;
  place-items: center;
  padding: 1rem;
}
.package-backdrop {
  position: absolute;
  inset: 0;
  border: 0;
  background: rgb(var(--yb-overlay-night) / 0.48);
}
.package-dialog {
  position: relative;
  width: min(58rem, calc(100vw - 2rem));
  max-height: calc(100vh - 2rem);
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  overflow: hidden;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 1.2rem;
  background: rgb(var(--yb-surface));
  box-shadow: 0 24px 70px rgb(var(--yb-shadow) / 0.26);
}
.package-dialog > header,
.package-dialog > footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid rgb(var(--yb-border));
}
.package-dialog > header p {
  margin: 0;
  color: rgb(var(--yb-brand));
  font-size: 0.68rem;
  font-weight: 850;
  letter-spacing: 0.12em;
}
.package-dialog h2 {
  margin: 0.2rem 0 0;
}
.close-button {
  width: 2.5rem;
  height: 2.5rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.7rem;
  background: transparent;
  font-size: 1.5rem;
}
.package-body {
  overflow: auto;
  padding: 1.25rem;
}
.package-section {
  display: grid;
  gap: 1rem;
}
.package-copy h3,
.package-copy p {
  margin: 0;
}
.package-copy p {
  margin-top: 0.35rem;
  color: rgb(var(--yb-text-muted));
  font-size: 0.85rem;
}
.wide-field,
.field-grid label {
  display: grid;
  gap: 0.4rem;
  color: rgb(var(--yb-text-muted));
  font-size: 0.78rem;
}
.wide-field textarea,
.field-grid select,
.file-picker {
  box-sizing: border-box;
  width: 100%;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text));
}
.wide-field textarea {
  padding: 0.75rem;
  resize: vertical;
}
.field-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.8rem;
}
.field-grid select {
  min-height: 2.65rem;
  padding: 0 0.7rem;
}
.action-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem;
}
.download-hint {
  margin: -0.15rem 0 0;
  color: rgb(var(--yb-text-muted));
  font-size: 0.78rem;
}
.primary-button,
.secondary-button,
.quiet-button {
  min-height: 2.55rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.7rem;
  padding: 0 1rem;
  background: rgb(var(--yb-surface));
  font-weight: 750;
}
.primary-button {
  border-color: rgb(var(--yb-brand));
  background: rgb(var(--yb-brand));
  color: white;
}
.secondary-button {
  border-color: rgb(var(--yb-brand-border));
  color: rgb(var(--yb-brand));
}
.error {
  color: rgb(var(--yb-danger));
}
.file-picker {
  min-height: 5rem;
  display: grid;
  place-items: center;
  border-style: dashed;
  cursor: pointer;
}
.file-picker.compact-picker {
  min-height: 2.65rem;
  padding: 0.5rem 0.75rem;
  text-align: center;
}
.file-picker input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}
.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 0.65rem;
}
.summary-grid span {
  display: grid;
  gap: 0.2rem;
  padding: 0.8rem;
  border-radius: 0.8rem;
  background: rgb(var(--yb-surface-soft));
}
.summary-grid small {
  color: rgb(var(--yb-text-muted));
}
.summary-grid strong {
  font-size: 1.3rem;
}
.failure-list {
  display: grid;
  gap: 0.5rem;
  padding: 0.8rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.8rem;
}
.failure-list p {
  margin: 0;
  color: rgb(var(--yb-danger));
  font-size: 0.8rem;
}
.package-dialog > footer {
  border-top: 1px solid rgb(var(--yb-border));
  border-bottom: 0;
}
.package-dialog > footer span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: rgb(var(--yb-text-muted));
  font-size: 0.8rem;
}
@media (max-width: 640px) {
  .package-layer {
    padding: 0;
  }
  .package-dialog {
    width: 100vw;
    max-height: 100vh;
    height: 100vh;
    border-radius: 0;
  }
  .field-grid,
  .summary-grid {
    grid-template-columns: 1fr 1fr;
  }
  .package-body {
    padding: 1rem;
  }
}
</style>
