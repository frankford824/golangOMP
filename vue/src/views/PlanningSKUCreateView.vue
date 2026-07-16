<template>
  <main class="planning-page">
    <header class="hero">
      <div>
        <p class="eyebrow">批量生成</p>
        <h1>策划 SKU</h1>
        <p>一次生成 1–200 个 SKU 及对应策划信息，创建成功后任务立即结单。</p>
      </div>
      <div class="hero-actions">
        <a class="secondary-button" :href="planningSkuApi.templateURL(erpSyncMode === 'async')">下载 Excel 模板</a>
        <label class="secondary-button file-button">
          导入 Excel
          <input type="file" accept=".xlsx" @change="onExcel" />
        </label>
      </div>
    </header>

    <section v-if="result" class="result-card">
      <div>
        <p class="eyebrow">创建完成</p>
        <h2>{{ result.task_no }}</h2>
        <p>已生成 {{ result.items.length }} 个 SKU，任务状态为已结单。</p>
      </div>
      <div class="result-actions">
        <button class="secondary-button" @click="copyAll">复制全部 SKU</button>
        <a class="primary-button" :href="planningSkuApi.exportTaskURL(result.task_id)">导出结果</a>
      </div>
      <div class="sku-results">
        <button v-for="item in result.items" :key="item.task_sku_item_id" class="sku-pill" @click="copy(item.sku_code)">
          <span>{{ item.sequence_no }}</span><strong>{{ item.sku_code }}</strong><small>{{ item.erp_status || '无需 ERP' }}</small>
        </button>
      </div>
    </section>

    <section v-else class="editor-card">
      <div class="editor-toolbar">
        <div>
          <h2>策划明细</h2>
          <p>{{ rows.length }} / 200 行，只渲染当前可见行。</p>
        </div>
        <div class="toolbar-actions">
          <label class="switch"><input v-model="erpSync" type="checkbox" /><span>创建后异步同步 ERP</span></label>
          <button class="secondary-button" :disabled="rows.length >= 200" @click="addRow">添加一行</button>
        </div>
      </div>

      <section v-if="validationSummary.length || excelErrors.length" class="validation-panel" role="alert">
        <div>
          <strong>还有 {{ validationSummary.length + excelErrors.length }} 处信息需要完善。</strong>
          <p>修正全部问题后才能生成；任一行失败都不会创建任务。</p>
        </div>
        <button v-if="validationSummary.length" class="secondary-button" @click="locateFirstError">定位第一处</button>
        <ul>
          <li v-for="item in validationSummary.slice(0, 12)" :key="'row-' + item.row + '-' + item.field">第 {{ item.row }} 行 · {{ item.message }}</li>
          <li v-for="item in excelErrors.slice(0, 12)" :key="'excel-' + item.row + '-' + item.field">Excel 第 {{ item.row }} 行 · {{ item.field }}：{{ item.reason }}</li>
        </ul>
      </section>
      <div v-if="submitError" class="error-panel" role="alert">{{ submitError }}</div>

      <div
        ref="viewport"
        class="virtual-list"
        aria-label="策划 SKU 明细"
        :style="{ '--planning-row-height': rowHeight + 'px' }"
        @scroll="onScroll"
      >
        <div class="virtual-spacer" :style="{ height: totalHeight + 'px' }">
          <div class="virtual-window" :style="{ transform: 'translateY(' + windowOffset + 'px)' }">
            <article
              v-for="entry in visibleRows"
              :key="entry.row.client_item_id"
              class="planning-row"
              :class="{ invalid: rowErrors(entry.row).length > 0 }"
              :data-row-index="entry.index"
              data-testid="planning-row"
            >
              <header class="row-head">
                <div><span>第 {{ entry.index + 1 }} 行</span><strong>{{ entry.row.description_spec || '未填写产品描述' }}</strong></div>
                <button class="remove-row" :disabled="rows.length === 1" :aria-label="'删除第 ' + (entry.index + 1) + ' 行'" @click="removeRow(entry.index)">删除</button>
              </header>
              <div class="row-fields">
                <section class="image-field">
                  <span class="field-label">产品图片（选填）</span>
                  <label class="image-upload" :class="{ uploaded: entry.row.image_upload_ref }">
                    <input type="file" accept="image/*" :disabled="Boolean(entry.row.uploading)" @change="onImage($event, entry.row)" />
                    <span>{{ entry.row.uploading ? '上传中…' : entry.row.image_name || '选择图片' }}</span>
                  </label>
                  <button v-if="entry.row.image_upload_ref" class="remove-image" @click="clearImage(entry.row)">移除图片</button>
                </section>
                <label class="description-field">产品描述 / 规格 <span>必填</span><textarea v-model.trim="entry.row.description_spec" rows="3" maxlength="4000" placeholder="产品名称、材质、尺寸、工艺等" :aria-invalid="Boolean(fieldError(entry.row, 'description_spec'))" /></label>
                <label>数量 <span>必填</span><input v-model.number="entry.row.quantity" type="number" min="1" step="1" :aria-invalid="Boolean(fieldError(entry.row, 'quantity'))" /></label>
                <label>目标价（选填）<input v-model.trim="entry.row.target_price" inputmode="decimal" placeholder="12.50" :aria-invalid="Boolean(fieldError(entry.row, 'target_price'))" /></label>
                <label>备注（选填）<textarea v-model.trim="entry.row.note" rows="2" maxlength="2000" /></label>
                <label>参考链接（选填）<input v-model.trim="entry.row.reference_url" type="url" placeholder="https://" :aria-invalid="Boolean(fieldError(entry.row, 'reference_url'))" /></label>
                <label v-if="erpSync">ERP 产品 i_id <span>必填</span><input v-model.trim="entry.row.erp_product_i_id" :aria-invalid="Boolean(fieldError(entry.row, 'erp_product_i_id'))" /></label>
                <label v-if="erpSync">ERP 产品名称 <span>必填</span><input v-model.trim="entry.row.erp_product_name" :aria-invalid="Boolean(fieldError(entry.row, 'erp_product_name'))" /></label>
              </div>
              <ul v-if="rowErrors(entry.row).length" class="row-errors">
                <li v-for="issue in rowErrors(entry.row)" :key="issue.field">{{ issue.message }}</li>
              </ul>
            </article>
          </div>
        </div>
      </div>

      <footer class="editor-footer">
        <div>
          <p>SKU 编号由服务端统一规则原子分配；任一行失败，整单不会创建。</p>
          <small data-testid="virtual-total">当前 {{ rows.length }} 行，页面仅保留 {{ visibleRows.length }} 个可编辑行节点。</small>
        </div>
        <button class="primary-button" :disabled="submitting || !isValid" @click="submit">
          {{ submitting ? '正在生成…' : '生成 ' + rows.length + ' 个 SKU 并结单' }}
        </button>
      </footer>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { planningSkuApi, type PlanningSKUCreateResult, type PlanningSKUInput } from '@/services/api/planningSkuApi'

type PlanningRow = PlanningSKUInput & { image_name?: string; uploading?: boolean; image_error?: string }
type RowIssue = { field: string; message: string }

const clientCreateId = crypto.randomUUID()
const newRow = (): PlanningRow => ({ client_item_id: crypto.randomUUID(), description_spec: '', quantity: 1, target_price: '', note: '', reference_url: '', erp_product_i_id: '', erp_product_name: '' })
const rows = ref<PlanningRow[]>([newRow()])
const erpSync = ref(false)
const submitting = ref(false)
const submitError = ref('')
const excelErrors = ref<Array<{ row: number; field: string; reason: string }>>([])
const result = ref<PlanningSKUCreateResult | null>(null)
const viewport = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const viewportHeight = ref(620)
const rowHeight = ref(370)
const overscan = 2
const erpSyncMode = computed(() => erpSync.value ? 'async' : 'none')
const pricePattern = /^\d{1,10}(\.\d{1,2})?$/

function rowErrors(row: PlanningRow): RowIssue[] {
  const issues: RowIssue[] = []
  const descriptionLength = row.description_spec.trim().length
  if (!descriptionLength) issues.push({ field: 'description_spec', message: '产品描述 / 规格不能为空。' })
  else if (descriptionLength > 4000) issues.push({ field: 'description_spec', message: '产品描述 / 规格不能超过 4000 字。' })
  if (!Number.isInteger(Number(row.quantity)) || Number(row.quantity) <= 0) issues.push({ field: 'quantity', message: '数量必须是正整数。' })
  if (row.target_price && (!pricePattern.test(row.target_price) || Number(row.target_price) <= 0)) issues.push({ field: 'target_price', message: '目标价需大于 0，最多保留两位小数。' })
  if ((row.note || '').length > 2000) issues.push({ field: 'note', message: '备注不能超过 2000 字。' })
  if (row.reference_url && !/^https?:\/\//i.test(row.reference_url)) issues.push({ field: 'reference_url', message: '参考链接必须以 http:// 或 https:// 开头。' })
  if (erpSync.value && !row.erp_product_i_id?.trim()) issues.push({ field: 'erp_product_i_id', message: '启用 ERP 同步时必须填写产品 i_id。' })
  if (erpSync.value && !row.erp_product_name?.trim()) issues.push({ field: 'erp_product_name', message: '启用 ERP 同步时必须填写产品名称。' })
  if (row.image_error) issues.push({ field: 'product_image', message: row.image_error })
  return issues
}

function fieldError(row: PlanningRow, field: string): string {
  return rowErrors(row).find((item) => item.field === field)?.message || ''
}

const validationSummary = computed(() => rows.value.flatMap((row, index) => rowErrors(row).map((issue) => ({ ...issue, row: index + 1, index }))))
const isValid = computed(() => rows.value.length >= 1 && rows.value.length <= 200 && validationSummary.value.length === 0 && excelErrors.value.length === 0)
const totalHeight = computed(() => rows.value.length * rowHeight.value)
const visibleStart = computed(() => Math.max(0, Math.floor(scrollTop.value / rowHeight.value) - overscan))
const visibleCount = computed(() => Math.ceil(viewportHeight.value / rowHeight.value) + overscan * 2)
const visibleEnd = computed(() => Math.min(rows.value.length, visibleStart.value + visibleCount.value))
const visibleRows = computed(() => rows.value.slice(visibleStart.value, visibleEnd.value).map((row, offset) => ({ row, index: visibleStart.value + offset })))
const windowOffset = computed(() => visibleStart.value * rowHeight.value)

function refreshViewportMetrics() {
  rowHeight.value = window.matchMedia('(max-width: 760px)').matches ? 980 : 370
  viewportHeight.value = viewport.value?.clientHeight || Math.min(window.innerHeight * 0.64, 720)
}
function onScroll() { scrollTop.value = viewport.value?.scrollTop || 0 }
function addRow() {
  if (rows.value.length >= 200) return
  rows.value.push(newRow())
  excelErrors.value = []
}
function removeRow(index: number) {
  if (rows.value.length <= 1) return
  rows.value.splice(index, 1)
  scrollTop.value = Math.min(scrollTop.value, Math.max(0, totalHeight.value - viewportHeight.value))
}
async function locateFirstError() {
  const first = validationSummary.value[0]
  if (!first || !viewport.value) return
  viewport.value.scrollTop = first.index * rowHeight.value
  scrollTop.value = viewport.value.scrollTop
  await nextTick()
  const row = viewport.value.querySelector<HTMLElement>('[data-row-index="' + first.index + '"]')
  row?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()
}
async function submit() {
  if (!isValid.value) { await locateFirstError(); return }
  submitting.value = true
  submitError.value = ''
  try { result.value = await planningSkuApi.create(rows.value, erpSyncMode.value, clientCreateId) }
  catch (reason) { submitError.value = reason instanceof Error ? reason.message : '策划 SKU 创建失败。' }
  finally { submitting.value = false }
}
async function onExcel(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  submitError.value = ''
  try {
    const parsed = await planningSkuApi.parseExcel(file, erpSync.value)
    excelErrors.value = parsed.errors
    if (parsed.planning_sku_items.length) rows.value = parsed.planning_sku_items.slice(0, 200).map((item) => ({ ...item, client_item_id: item.client_item_id || crypto.randomUUID() }))
    scrollTop.value = 0
    if (viewport.value) viewport.value.scrollTop = 0
  } catch (reason) { submitError.value = reason instanceof Error ? reason.message : 'Excel 解析失败。' }
  finally { input.value = '' }
}
async function onImage(event: Event, row: PlanningRow) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  row.uploading = true
  row.image_error = ''
  try {
    row.image_upload_ref = await planningSkuApi.uploadImage(file, clientCreateId, row.client_item_id)
    row.image_name = file.name
  } catch (reason) {
    row.image_error = reason instanceof Error ? reason.message : '图片上传失败。'
  } finally {
    row.uploading = false
    input.value = ''
  }
}
function clearImage(row: PlanningRow) { row.image_upload_ref = ''; row.image_name = ''; row.image_error = '' }
async function copy(value: string) { await navigator.clipboard.writeText(value) }
async function copyAll() { if (result.value) await copy(result.value.items.map((item) => item.sku_code).join('\n')) }
onMounted(() => { refreshViewportMetrics(); window.addEventListener('resize', refreshViewportMetrics) })
onBeforeUnmount(() => window.removeEventListener('resize', refreshViewportMetrics))
</script>

<style scoped>
.planning-page{max-width:1440px;margin:0 auto;padding:30px;display:grid;gap:22px}.hero,.editor-toolbar,.editor-footer,.result-card>div:first-child,.result-actions{display:flex;align-items:center;justify-content:space-between;gap:18px}.hero h1,.result-card h2{margin:4px 0;font-size:32px}.hero p,.editor-toolbar p,.editor-footer p,.result-card p{color:rgb(var(--yb-text-muted));margin:0}.eyebrow{color:rgb(var(--yb-brand));font-size:11px;letter-spacing:.14em;font-weight:900}.hero-actions,.toolbar-actions,.result-actions{display:flex;gap:10px}.editor-card,.result-card{border:1px solid rgb(var(--yb-border));border-radius:20px;background:rgb(var(--yb-surface));overflow:hidden;box-shadow:0 12px 30px rgb(var(--yb-shadow) / .05)}.editor-toolbar,.editor-footer,.result-card{padding:20px 22px}.editor-toolbar{border-bottom:1px solid rgb(var(--yb-border))}.editor-toolbar h2{margin:0 0 3px}.primary-button,.secondary-button{display:inline-flex;align-items:center;justify-content:center;min-height:40px;padding:0 16px;border-radius:11px;font-weight:750;text-decoration:none;cursor:pointer}.primary-button{border:0;background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse))}.secondary-button{border:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.primary-button:disabled,.secondary-button:disabled{opacity:.45;cursor:not-allowed}.file-button input{display:none}.switch{display:flex;gap:8px;align-items:center;font-size:13px}.switch input{width:auto}.validation-panel,.error-panel{margin:16px 22px 0;padding:14px;border-radius:12px;background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}.validation-panel{display:grid;grid-template-columns:1fr auto;gap:10px}.validation-panel p{margin:3px 0}.validation-panel ul{grid-column:1/-1;margin:0;padding-left:20px}.virtual-list{height:min(64vh,720px);overflow:auto;overscroll-behavior:contain;background:rgb(var(--yb-surface-soft))}.virtual-spacer{position:relative}.virtual-window{position:absolute;inset:0 0 auto}.planning-row{box-sizing:border-box;height:calc(var(--planning-row-height) - 12px);margin:6px 14px;padding:16px;border:1px solid rgb(var(--yb-border));border-radius:15px;background:rgb(var(--yb-surface));overflow:auto}.planning-row.invalid{border-color:rgb(var(--yb-danger-border))}.row-head{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-bottom:12px}.row-head div{display:grid;gap:2px}.row-head span{font-size:12px;color:rgb(var(--yb-text-muted))}.row-head strong{max-width:72ch;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.remove-row,.remove-image{border:0;background:transparent;color:rgb(var(--yb-danger-text));cursor:pointer}.row-fields{display:grid;grid-template-columns:150px minmax(260px,2fr) repeat(3,minmax(130px,1fr));gap:12px;align-items:start}.row-fields label,.image-field{display:grid;gap:6px;font-size:13px}.row-fields label>span{color:rgb(var(--yb-danger-text));font-size:11px}.description-field{grid-row:span 2}.field-label{font-size:13px}.image-upload{display:block;padding:10px;border:1px dashed rgb(var(--yb-border));border-radius:9px;color:rgb(var(--yb-text-muted));cursor:pointer;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.image-upload.uploaded{border-style:solid;border-color:rgb(var(--yb-success-border));color:rgb(var(--yb-success-strong))}.image-upload input{display:none}.row-fields textarea,.row-fields input{width:100%;box-sizing:border-box;border:1px solid rgb(var(--yb-border));border-radius:9px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));padding:9px 10px}.row-fields textarea:focus,.row-fields input:focus{outline:2px solid rgb(var(--yb-brand-soft));border-color:rgb(var(--yb-brand))}.row-fields [aria-invalid="true"]{border-color:rgb(var(--yb-danger-border))}.row-errors{display:flex;flex-wrap:wrap;gap:6px 18px;margin:12px 0 0;padding:10px 10px 10px 28px;border-radius:10px;background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text));font-size:12px}.editor-footer small{display:block;margin-top:4px;color:rgb(var(--yb-text-muted))}.sku-results{grid-column:1/-1;display:grid;grid-template-columns:repeat(auto-fill,minmax(230px,1fr));gap:10px;margin-top:18px}.sku-pill{display:grid;grid-template-columns:28px 1fr auto;align-items:center;gap:8px;padding:12px;border:1px solid rgb(var(--yb-border));border-radius:12px;background:rgb(var(--yb-surface));text-align:left;cursor:pointer}.sku-pill span,.sku-pill small{color:rgb(var(--yb-text-muted))}
@media(max-width:1050px){.row-fields{grid-template-columns:130px repeat(2,minmax(0,1fr))}.description-field{grid-column:span 2}}
@media(max-width:760px){.planning-page{padding:16px}.hero,.editor-toolbar,.editor-footer{align-items:flex-start;flex-direction:column}.hero-actions,.toolbar-actions{width:100%;flex-wrap:wrap}.editor-toolbar,.editor-footer{padding:16px}.validation-panel,.error-panel{margin:12px 12px 0}.validation-panel{grid-template-columns:1fr}.validation-panel ul{grid-column:auto}.virtual-list{height:68vh}.planning-row{margin:6px 8px;padding:14px}.row-head{align-items:flex-start}.row-fields{grid-template-columns:1fr}.description-field{grid-column:auto;grid-row:auto}.row-errors{display:grid}.editor-footer .primary-button{width:100%}.result-card{padding:16px}.result-actions{align-items:stretch;flex-direction:column}}
</style>
