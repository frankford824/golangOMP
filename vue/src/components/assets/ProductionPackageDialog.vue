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

        <nav class="package-tabs" aria-label="生产打包方式">
          <button type="button" :class="{ active: mode === 'batch' }" @click="mode = 'batch'">批量搜索下载</button>
          <button type="button" :class="{ active: mode === 'excel' }" @click="mode = 'excel'">Excel 仓库外发</button>
        </nav>

        <div class="package-body">
          <section v-if="mode === 'batch'" class="package-section">
            <div class="package-copy">
              <h3>按 SKU、任务号或文件关键词批量查找</h3>
              <p>系统资源与外部资源会同时匹配；套装中的每个组件都会保留，不会只取第一张。</p>
            </div>
            <label class="wide-field">
              <span>每行一个查询词</span>
              <textarea v-model="termsText" rows="7" placeholder="HSC34548&#10;GK000804" />
            </label>
            <div class="field-grid">
              <label><span>文件格式</span><select v-model="batchFormat"><option value="image">全部生产图片（含 TIF）</option><option value="jpg_png">JPG / PNG</option><option value="design">设计源文件</option><option value="archive">压缩包</option><option value="all">全部格式</option></select></label>
              <label><span>资源类型</span><select v-model="batchKind"><option value="delivery">最终成品</option><option value="source">源文件</option><option value="all">全部资源</option></select></label>
            </div>
            <div class="action-row">
              <button type="button" class="secondary-button" :disabled="busy" @click="searchBatch">{{ busy ? '查询中…' : '查询资源' }}</button>
              <button type="button" class="primary-button" :disabled="busy || !selectedRefs.length" @click="downloadBatch">下载所选 ZIP（{{ selectedRefs.length }}）</button>
              <button v-if="batchRows.length" type="button" class="quiet-button" @click="exportBatchReport">导出 Excel</button>
            </div>
            <div v-if="batchRows.length" class="result-list">
              <article v-for="row in batchRows" :key="row.term">
                <div><strong>{{ row.term }}</strong><span :class="`status-${row.status}`">{{ row.message }}</span></div>
                <label v-for="asset in row.assets || []" :key="assetRef(asset)" class="asset-choice">
                  <input type="checkbox" :checked="selectedRefs.includes(assetRef(asset))" @change="toggleAsset(assetRef(asset))" />
                  <span>{{ asset.file_name || asset.original_filename || asset.resource_id || asset.id }}</span>
                  <small>{{ asset.source_type === 'external' ? '外部资源' : '系统资源' }}</small>
                </label>
              </article>
            </div>
          </section>

          <section v-else class="package-section">
            <div class="package-copy">
              <h3>上传仓库 Excel 并生成完整生产包</h3>
              <p>支持 XLS/XLSX、JPG/PNG/TIF/TIFF。套装会按资源目录展开为文件夹；原始外部资源身份保持只读。</p>
            </div>
            <label class="file-picker">
              <input type="file" accept=".xlsx,.xls" :disabled="busy" @change="handleExcelFile" />
              <span>{{ excelFileName || '选择 Excel 文件' }}</span>
            </label>
            <div v-if="excelManifest" class="summary-grid">
              <span><small>匹配行</small><strong>{{ excelManifest.success_count }}</strong></span>
              <span><small>文件数</small><strong>{{ excelManifest.total_files }}</strong></span>
              <span><small>失败行</small><strong>{{ excelManifest.failure_count }}</strong></span>
              <span><small>套装目录</small><strong>{{ excelSetCount }}</strong></span>
            </div>
            <div v-if="excelManifest" class="action-row">
              <button type="button" class="primary-button" :disabled="busy || !excelManifest.items.length" @click="downloadExcelPackage">下载生产 ZIP</button>
              <button type="button" class="quiet-button" @click="exportExcelReport">导出匹配结果 Excel</button>
            </div>
            <div v-if="excelManifest?.failures?.length" class="failure-list">
              <strong>异常明细</strong>
              <p v-for="(failure, index) in excelManifest.failures.slice(0, 30)" :key="index">{{ failure.sku_code || failure.sku_name || `第 ${failure.row_number || '-'} 行` }}：{{ failure.message || failure.reason }}</p>
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
import { computed, ref } from 'vue'
import type { BackendAsset } from '@/services/apiTypes'
import {
  assetsApi,
  type AssetBatchSearchResult,
  type AssetExcelPackageManifest,
} from '@/services/api/assetsApi'
import { buildTimestampedZipFilename, downloadBatchAsZip } from '@/utils/batchZipDownload'
import { resolveExcelPackageSetFolders, resolveExcelPackageZipFilename } from '@/utils/excelPackageZip'

defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()
const mode = ref<'batch' | 'excel'>('batch')
const termsText = ref('')
const batchFormat = ref<'jpg_png' | 'image' | 'design' | 'archive' | 'all'>('image')
const batchKind = ref<'delivery' | 'source' | 'all'>('delivery')
const batchRows = ref<AssetBatchSearchResult[]>([])
const selectedRefs = ref<string[]>([])
const excelFileName = ref('')
const excelManifest = ref<AssetExcelPackageManifest | null>(null)
const busy = ref(false)
const status = ref('')
const error = ref('')
const excelSetFolders = computed(() => excelManifest.value ? resolveExcelPackageSetFolders(excelManifest.value.items) : [])
const excelSetCount = computed(() => new Set(excelSetFolders.value.filter(Boolean)).size)

function close() {
  if (!busy.value) emit('close')
}

function assetRef(asset: BackendAsset): string {
  return String(asset.resource_id || asset.id || '').trim()
}

function toggleAsset(value: string) {
  selectedRefs.value = selectedRefs.value.includes(value)
    ? selectedRefs.value.filter((item) => item !== value)
    : [...selectedRefs.value, value]
}

function unwrap<T>(response: { data?: { data?: T } | T }): T | undefined {
  const body = response.data
  return body && typeof body === 'object' && 'data' in body ? body.data : body as T | undefined
}

async function searchBatch() {
  const terms = Array.from(new Set(termsText.value.split(/[\n,，;；\s]+/).map((item) => item.trim()).filter(Boolean)))
  if (!terms.length) {
    error.value = '请至少输入一个 SKU、任务号或关键词。'
    return
  }
  busy.value = true
  error.value = ''
  status.value = `正在查询 ${terms.length} 项…`
  try {
    const response = await assetsApi.batchSearchAssets({ terms, format_filter: batchFormat.value, asset_kind: batchKind.value })
    const manifest = unwrap(response)
    batchRows.value = manifest?.results || []
    selectedRefs.value = batchRows.value.flatMap((row) => row.assets || []).map(assetRef).filter(Boolean)
    status.value = `已匹配 ${manifest?.matched_count || 0} 项，未匹配 ${manifest?.failed_count || 0} 项。`
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '批量查询失败。'
  } finally {
    busy.value = false
  }
}

async function downloadBatch() {
  busy.value = true
  error.value = ''
  try {
    const response = await assetsApi.batchDownload(selectedRefs.value, { namingMode: 'business' })
    const manifest = unwrap(response)
    if (!manifest?.items?.length) throw new Error('没有可下载的资源。')
    const result = await downloadBatchAsZip({
      zipFilename: buildTimestampedZipFilename('生产打包'),
      items: manifest.items.map((item) => ({
        key: item.resource_id || String(item.asset_id),
        filename: item.filename,
        downloadURL: item.download_url,
      })),
      serverFailures: (manifest.failures || []).map((item) => `${item.resource_id || item.asset_id}: ${item.reason}`),
      normalizeNestedZipFilenames: true,
      onStatus: (message) => { status.value = message },
    })
    status.value = `已生成 ZIP：${result.writtenCount} 个文件。`
    if (result.failureCount) error.value = `${result.failureCount} 个文件失败，明细已写入 ZIP。`
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '生产打包失败。'
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
  busy.value = true
  error.value = ''
  status.value = '正在解析 Excel 并匹配系统与外部资源…'
  try {
    const response = await assetsApi.excelPackagePreviewFile(file)
    excelManifest.value = unwrap(response) || null
    if (!excelManifest.value) throw new Error('后端未返回打包清单。')
    status.value = `已匹配 ${excelManifest.value.total_files} 个生产文件。`
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : 'Excel 生产打包解析失败。'
  } finally {
    busy.value = false
    input.value = ''
  }
}

async function downloadExcelPackage() {
  const manifest = excelManifest.value
  if (!manifest) return
  busy.value = true
  error.value = ''
  try {
    const folders = resolveExcelPackageSetFolders(manifest.items)
    const items = manifest.items.flatMap((item, index) => {
      const quantity = Math.max(1, Math.trunc(Number(item.quantity) || 1))
      return Array.from({ length: quantity }, (_, copyIndex) => ({
        key: `${item.resource_id || item.asset_id}-${copyIndex + 1}`,
        filename: resolveExcelPackageZipFilename(item, copyIndex + 1, { includeBusinessPrefix: !folders[index] }),
        zipPath: folders[index] || undefined,
        downloadURL: item.download_url,
        failureHint: `${item.sku_code || item.sku_name}: download_failed`,
      }))
    })
    const result = await downloadBatchAsZip({
      zipFilename: buildTimestampedZipFilename('生产打包-仓库外发'),
      items,
      serverFailures: (manifest.failures || []).map((item) => `${item.sku_code || item.sku_name || item.row_number}: ${item.message || item.reason}`),
      onStatus: (message) => { status.value = message },
    })
    status.value = `已生成生产包：${result.writtenCount} 个文件，${excelSetCount.value} 个套装目录。`
    if (result.failureCount) error.value = `${result.failureCount} 项异常，明细已写入 ZIP。`
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '生成生产 ZIP 失败。'
  } finally {
    busy.value = false
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
  const blob = new Blob([bytes], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
}

function exportBatchReport() {
  void exportRows('生产打包-批量搜索结果.xlsx', batchRows.value.flatMap((row) =>
    (row.assets?.length ? row.assets : [undefined]).map((asset) => ({
      查询词: row.term,
      状态: row.status,
      说明: row.message,
      资源ID: asset ? assetRef(asset) : '',
      文件名: asset?.file_name || asset?.original_filename || '',
      来源: asset?.source_type || '',
    })),
  ))
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
.package-layer{position:fixed;inset:0;z-index:120;display:grid;place-items:center;padding:1rem}.package-backdrop{position:absolute;inset:0;border:0;background:rgb(var(--yb-overlay-night)/.48)}.package-dialog{position:relative;width:min(58rem,calc(100vw - 2rem));max-height:calc(100vh - 2rem);display:grid;grid-template-rows:auto auto minmax(0,1fr) auto;overflow:hidden;border:1px solid rgb(var(--yb-border));border-radius:1.2rem;background:rgb(var(--yb-surface));box-shadow:0 24px 70px rgb(var(--yb-shadow)/.26)}.package-dialog>header,.package-dialog>footer{display:flex;align-items:center;justify-content:space-between;gap:1rem;padding:1rem 1.25rem;border-bottom:1px solid rgb(var(--yb-border))}.package-dialog>header p{margin:0;color:rgb(var(--yb-brand));font-size:.68rem;font-weight:850;letter-spacing:.12em}.package-dialog h2{margin:.2rem 0 0}.close-button{width:2.5rem;height:2.5rem;border:1px solid rgb(var(--yb-border));border-radius:.7rem;background:transparent;font-size:1.5rem}.package-tabs{display:flex;gap:.4rem;padding:.75rem 1.25rem;border-bottom:1px solid rgb(var(--yb-border))}.package-tabs button{min-height:2.5rem;border:0;border-radius:.65rem;padding:0 1rem;background:transparent;color:rgb(var(--yb-text-muted));font-weight:750}.package-tabs button.active{background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand))}.package-body{overflow:auto;padding:1.25rem}.package-section{display:grid;gap:1rem}.package-copy h3,.package-copy p{margin:0}.package-copy p{margin-top:.35rem;color:rgb(var(--yb-text-muted));font-size:.85rem}.wide-field,.field-grid label{display:grid;gap:.4rem;color:rgb(var(--yb-text-muted));font-size:.78rem}.wide-field textarea,.field-grid select,.file-picker{box-sizing:border-box;width:100%;border:1px solid rgb(var(--yb-border));border-radius:.75rem;background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.wide-field textarea{padding:.75rem;resize:vertical}.field-grid{display:grid;grid-template-columns:1fr 1fr;gap:.8rem}.field-grid select{min-height:2.65rem;padding:0 .7rem}.action-row{display:flex;flex-wrap:wrap;gap:.65rem}.primary-button,.secondary-button,.quiet-button{min-height:2.55rem;border:1px solid rgb(var(--yb-border));border-radius:.7rem;padding:0 1rem;background:rgb(var(--yb-surface));font-weight:750}.primary-button{border-color:rgb(var(--yb-brand));background:rgb(var(--yb-brand));color:white}.secondary-button{border-color:rgb(var(--yb-brand-border));color:rgb(var(--yb-brand))}.result-list{display:grid;gap:.7rem}.result-list article,.failure-list{display:grid;gap:.5rem;padding:.8rem;border:1px solid rgb(var(--yb-border));border-radius:.8rem}.result-list article>div{display:flex;justify-content:space-between;gap:1rem}.asset-choice{display:grid;grid-template-columns:auto minmax(0,1fr) auto;align-items:center;gap:.55rem}.asset-choice small{color:rgb(var(--yb-text-muted))}.status-not_found,.status-error,.error{color:rgb(var(--yb-danger))}.file-picker{min-height:5rem;display:grid;place-items:center;border-style:dashed;cursor:pointer}.file-picker input{position:absolute;opacity:0;pointer-events:none}.summary-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:.65rem}.summary-grid span{display:grid;gap:.2rem;padding:.8rem;border-radius:.8rem;background:rgb(var(--yb-surface-soft))}.summary-grid small{color:rgb(var(--yb-text-muted))}.summary-grid strong{font-size:1.3rem}.failure-list p{margin:0;color:rgb(var(--yb-danger));font-size:.8rem}.package-dialog>footer{border-top:1px solid rgb(var(--yb-border));border-bottom:0}.package-dialog>footer span{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:rgb(var(--yb-text-muted));font-size:.8rem}@media(max-width:640px){.package-layer{padding:0}.package-dialog{width:100vw;max-height:100vh;height:100vh;border-radius:0}.field-grid,.summary-grid{grid-template-columns:1fr 1fr}.package-body{padding:1rem}.asset-choice{grid-template-columns:auto minmax(0,1fr)}.asset-choice small{grid-column:2}} 
</style>
