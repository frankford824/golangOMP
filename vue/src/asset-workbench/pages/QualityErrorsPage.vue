<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { Calculator, Download, FileSpreadsheet, RefreshCw, UploadCloud } from 'lucide-vue-next'

import {
  assetWorkbenchApi,
  type DeductionRuleRow,
  type ErrorImportBatchRow,
  type SettlementPreview,
  type SettlementPayrollRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { exportErrorImportTemplateWorkbook } from '@aw/features/export/settlementExport'
import { currentBusinessMonth, isImportBusinessMonth } from '@aw/shared/format/businessMonth'
import { formatInt, formatMoney } from '@aw/shared/format/number'
import { formatShanghaiDateTime } from '@aw/shared/format/dateTime'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { chipClass, workerTypeMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import PersonnelPicker from '@aw/shared/ui/PersonnelPicker.vue'
import QualityImportReview from '@aw/features/settlement/QualityImportReview.vue'
import { resolveApiUserMessage } from '@/utils/api-message-zh'
import { useAssetWorkbenchSessionStore } from '@aw/app/session.store'

type DeductionGridRow = DeductionRuleRow & Record<string, unknown> & {
  worker_type_label: string
  deduction_amount_label: string
  effective_range: string
  enabled_label: string
}

const route = useRoute()
const requestedMonth = String(route.query.month ?? '')
const month = ref(/^\d{4}-(0[1-9]|1[0-2])$/.test(requestedMonth) && requestedMonth <= currentBusinessMonth()
  ? requestedMonth : currentBusinessMonth())
const session = useAssetWorkbenchSessionStore()
const preview = ref<SettlementPreview | null>(null)
const deductionRules = ref<DeductionRuleRow[]>([])
const importOpen = ref(false)
const importFeedback = ref('')
const importWarning = ref(false)
const importError = ref('')
const importMonthInput = ref<HTMLInputElement | null>(null)
const importAction = ref<HTMLButtonElement | null>(null)
const importReview = ref<HTMLElement | null>(null)
const historyPage = ref(1)
const historyPageSize = 10
const selectedImportID = ref<number | null>(null)
const historyDetailPage = ref(1)
const historyDetailPageSize = 25
const loading = ref(false)
const dragActive = ref(false)
const notice = ref('')
const error = ref('')
const fileInputRef = ref<HTMLInputElement | null>(null)
const reviewFile = ref<File | null>(null)
const importMonth = ref('')
const latestImportMonth = currentBusinessMonth()
const importMonthReady = computed(() => isImportBusinessMonth(importMonth.value, latestImportMonth))
const actualDeductions = ref<Awaited<ReturnType<typeof assetWorkbenchApi.qualityActualDeductions>> | null>(null)
const requestedPayee = Number(route.query.payee_id)
const deductionPayeeID = ref(Number.isSafeInteger(requestedPayee) && requestedPayee > 0 ? requestedPayee : 0)
const deductionRowType = ref<SettlementPayrollRow['row_type']>(
  route.query.row_type === 'supplement_piecework' ? 'supplement_piecework' : 'normal_piecework',
)
const deductionInput = ref('')
const savingDeduction = ref(false)
let loadSequence = 0

const ruleColumns = [
  { key: 'worker_type_label', label: '人员类型', width: 120 },
  { key: 'job_grade', label: '岗级', width: 100 },
  { key: 'difficulty_class', label: '出错分类', width: 120 },
  { key: 'deduction_amount_label', label: '每张扣款', width: 120, align: 'right' as const },
  { key: 'effective_range', label: '生效时间', width: 220 },
  { key: 'enabled_label', label: '状态', width: 100 },
]

const activeRules = computed(() => deductionRules.value.filter((rule) => ruleAppliesToMonth(rule, month.value)))
const canPreviewSettlement = computed(() => session.hasAnyCapability(['asset.workbench.settlement']))
const canViewRules = computed(() => session.hasAnyCapability(['asset.workbench.settlement', 'asset.workbench.cost_center.manage']))
const canManageRules = computed(() => session.hasAnyCapability(['asset.workbench.cost_center.manage']))
const historyRequest = usePageRequest(signal =>
  assetWorkbenchApi.listErrorImports({ business_month: month.value, page: historyPage.value, page_size: historyPageSize }, signal))
const historyDetailRequest = usePageRequest(signal =>
  selectedImportID.value === null ? Promise.resolve(null) : assetWorkbenchApi.getErrorImportDetail(selectedImportID.value, signal))
const history = computed(() => historyRequest.data.value?.items ?? [])
const historyTotal = computed(() => historyRequest.data.value?.total ?? 0)
const historyLastPage = computed(() => Math.max(1, Math.ceil(historyTotal.value / historyPageSize)))
const historyLoading = historyRequest.loading
const historyError = computed(() => historyRequest.error.value ? '导入记录加载失败，请重试。' : '')
const historyDetail = historyDetailRequest.data
const historyDetailRecords = computed(() => historyDetail.value?.records ?? [])
const historyDetailLastPage = computed(() => Math.max(1, Math.ceil(historyDetailRecords.value.length / historyDetailPageSize)))
const visibleHistoryDetail = computed(() => {
  const start = (historyDetailPage.value - 1) * historyDetailPageSize
  return historyDetailRecords.value.slice(start, start + historyDetailPageSize)
})
const historyDetailLoading = historyDetailRequest.loading
const historyDetailError = computed(() => historyDetailRequest.error.value ? '导入明细加载失败，请重试。' : '')

function closeHistoryDetail() {
  selectedImportID.value = null
  historyDetailPage.value = 1
  historyDetailRequest.reset()
}

function changeHistoryPage(delta: number) {
  historyPage.value += delta
  closeHistoryDetail()
  void historyRequest.run()
}

function openHistoryDetail(id: number) {
  if (selectedImportID.value === id) {
    closeHistoryDetail()
    return
  }
  closeHistoryDetail()
  selectedImportID.value = id
  void historyDetailRequest.run()
}

function refreshQualityPage() {
  void loadQualityOverview()
  if (canPreviewSettlement.value) void historyRequest.run()
}

function matchStatusLabel(status: string) {
  if (status === 'matched') return '已匹配'
  if (status === 'ambiguous') return '多匹配'
  if (status === 'unmatched') return '未匹配'
  return status || '—'
}

const ruleGridRows = computed<DeductionGridRow[]>(() =>
  activeRules.value.map((rule) => ({
    ...rule,
    worker_type_label: workerTypeMeta(rule.worker_type).label,
    deduction_amount_label: formatMoney(rule.deduction_amount),
    effective_range: rule.effective_to ? `${rule.effective_from} 至 ${rule.effective_to}` : `${rule.effective_from} 起`,
    enabled_label: rule.enabled ? '生效中' : '已停用',
  })),
)
const totalErrors = computed(() => preview.value?.totals.error_count ?? 0)
const totalDeduction = computed(() => preview.value?.totals.deduction_amount ?? 0)
const canEditActualDeduction = computed(() =>
  actualDeductions.value?.business_month === month.value && !actualDeductions.value.locked && !loading.value && !savingDeduction.value,
)
const selectedPayroll = computed(() => {
  const result = preview.value
  if (loading.value || !result || result.business_month !== month.value) return undefined
  return result.payroll_rows.find(
    row => row.payee_user_id === deductionPayeeID.value && row.row_type === deductionRowType.value,
  )
})
const savedDeduction = computed(() => actualDeductions.value?.deductions.find(
  row => row.payee_user_id === deductionPayeeID.value && (row.payroll_row_type ?? 'normal_piecework') === deductionRowType.value,
)?.quality_actual_deduction_amount)
const hasMatchedQualityRecords = computed(() => actualDeductions.value?.eligible_payroll_rows?.some(
  row => row.payee_user_id === deductionPayeeID.value && row.payroll_row_type === deductionRowType.value,
) ?? false)
const canMaintainActualDeduction = computed(() => canEditActualDeduction.value && hasMatchedQualityRecords.value)
const enteredDeduction = computed(() => {
  if (deductionInput.value === '') return null
  const amount = Number(deductionInput.value)
  const rounded = Math.round(amount * 100) / 100
  return Number.isFinite(amount) && amount >= 0 && amount <= 9999999999.99 &&
    Math.abs(amount - rounded) <= 1e-8 ? rounded : null
})
const hasUnsavedDeduction = computed(() =>
  enteredDeduction.value !== null && enteredDeduction.value !== savedDeduction.value,
)
const draftActualNet = computed(() => {
  const row = selectedPayroll.value
  const amount = enteredDeduction.value
  if (!row || amount === null || !hasUnsavedDeduction.value || !canMaintainActualDeduction.value) return null
  const difference = Math.round((row.deduction_amount - amount) * 10000) / 10000
  return row.net_amount + difference
})

function fillSavedDeduction() {
  deductionInput.value = savedDeduction.value === undefined ? '' : String(savedDeduction.value)
}

async function saveActualDeduction() {
  const amount = enteredDeduction.value
  if (!canMaintainActualDeduction.value || amount === null) return
  const targetMonth = month.value
  const targetPayee = deductionPayeeID.value
  savingDeduction.value = true
  error.value = ''
  notice.value = ''
  try {
    await assetWorkbenchApi.saveQualityActualDeduction(targetMonth, targetPayee, amount, deductionRowType.value)
    if (month.value !== targetMonth) return
    notice.value = amount === 0 ? '已保存，实际扣款为 ¥0.00。' : '已保存质检实际扣款。'
    await loadQualityOverview({ keepFeedback: true })
  } catch (err) {
    error.value = resolveApiUserMessage(err, { fallback: '保存质检实际扣款失败' })
  } finally {
    savingDeduction.value = false
  }
}

function ruleAppliesToMonth(rule: DeductionRuleRow, businessMonth: string) {
  if (!rule.enabled) return false
  const startMonth = rule.effective_from.slice(0, 7)
  const endMonth = rule.effective_to?.slice(0, 7) || ''
  return startMonth <= businessMonth && (!endMonth || endMonth >= businessMonth)
}

async function loadQualityOverview(options: { keepFeedback?: boolean } = {}) {
  const request = ++loadSequence
  const targetMonth = month.value
  loading.value = true
  actualDeductions.value = null
  if (!options.keepFeedback) {
    notice.value = ''
    error.value = ''
  }
  try {
    const [previewResult, rulesResult, actualResult] = await Promise.all([
      canPreviewSettlement.value ? assetWorkbenchApi.previewSettlement(targetMonth) : Promise.resolve(null),
      canViewRules.value ? assetWorkbenchApi.listDeductionRules({ page: 1, page_size: 100 }) : Promise.resolve(null),
      canPreviewSettlement.value ? assetWorkbenchApi.qualityActualDeductions(targetMonth) : Promise.resolve(null),
    ])
    if (request !== loadSequence) return
    preview.value = previewResult
    deductionRules.value = rulesResult?.items ?? []
    actualDeductions.value = actualResult
    fillSavedDeduction()
  } catch (err) {
    if (request === loadSequence) error.value = resolveApiUserMessage(err, { fallback: '出错记录数据加载失败，请稍后重试' })
  } finally {
    if (request === loadSequence) loading.value = false
  }
}

async function downloadTemplate() {
  importError.value = ''
  try {
    await exportErrorImportTemplateWorkbook()
  } catch (err) {
    importError.value = resolveApiUserMessage(err, { fallback: '模板生成失败，请稍后重试' })
  }
}

async function startQualityImport() {
  importFeedback.value = ''
  importError.value = ''
  importOpen.value = true
  await nextTick()
  importMonthInput.value?.focus()
}

function chooseFile() {
  if (importMonthReady.value) fileInputRef.value?.click()
}

function handleFileInput(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (file) void prepareReview(file)
}

function handleDrop(event: DragEvent) {
  dragActive.value = false
  if (!importMonthReady.value) return
  const file = Array.from(event.dataTransfer?.files ?? []).find((item) => /\.xlsx$/i.test(item.name))
  if (!file) {
    importError.value = '请选择 .xlsx 格式的 Excel 文件'
    return
  }
  void prepareReview(file)
}

async function prepareReview(file: File) {
  if (!importOpen.value || !importMonthReady.value || reviewFile.value) return
  importError.value = ''
  reviewFile.value = file
  await nextTick()
  importReview.value?.focus({ preventScroll: true })
  importReview.value?.scrollIntoView({ block: 'nearest' })
}

function closeQualityImport() {
  reviewFile.value = null
  importOpen.value = false
  importMonth.value = ''
  dragActive.value = false
  importError.value = ''
}

async function cancelQualityImport() {
  closeQualityImport()
  await nextTick()
  importAction.value?.focus()
}

async function handleQualityImported(result: ErrorImportBatchRow) {
  closeQualityImport()
  const monthChanged = month.value !== result.business_month
  month.value = result.business_month
  const notCounted = result.unmatched_rows + result.ambiguous_rows
  importWarning.value = notCounted > 0
  importFeedback.value = `${result.business_month} 已导入 ${formatInt(result.total_rows)} 行，${formatInt(result.matched_rows)} 行计入扣款${notCounted ? `，${formatInt(notCounted)} 行未匹配或重名，未计入扣款` : ''}。`
  if (!monthChanged) {
    historyPage.value = 1
    closeHistoryDetail()
    if (canPreviewSettlement.value) void historyRequest.run()
    void loadQualityOverview({ keepFeedback: true })
  }
  await nextTick()
  importAction.value?.focus()
}

watch(month, () => {
  closeQualityImport()
  importFeedback.value = ''
  importWarning.value = false
  historyPage.value = 1
  closeHistoryDetail()
  historyRequest.reset()
  refreshQualityPage()
}, { flush: 'sync' })

onMounted(refreshQualityPage)
</script>

<template>
  <section class="aw-page-stack aw-quality-errors-page">
    <section class="aw-data-surface aw-quality-history" aria-labelledby="aw-quality-history-title">
      <div class="aw-quality-history__toolbar">
        <div class="aw-quality-history__summary">
          <h3 id="aw-quality-history-title">质检导入记录</h3>
          <p v-if="canPreviewSettlement" class="aw-copy">
            {{ formatInt(historyTotal) }} 份导入
            <template v-if="preview?.business_month === month"> · 出错 {{ formatInt(totalErrors) }} 张 · 应扣 {{ formatMoney(totalDeduction) }}</template>
          </p>
        </div>
        <div class="aw-inline-actions" id="aw-quality-import">
          <label class="aw-month-control">
            <span>结算月</span>
            <input v-model="month" type="month" lang="zh-CN" aria-label="出错记录月份" :max="latestImportMonth" :disabled="savingDeduction || !!reviewFile" />
          </label>
          <a v-if="canPreviewSettlement" class="aw-secondary-button" href="#aw-actual-deductions">维护实际扣款</a>
          <a v-if="canViewRules" class="aw-secondary-button" href="#aw-deduction-rules">查看扣款规则</a>
          <button class="aw-secondary-button" type="button" :disabled="loading || historyLoading" @click="refreshQualityPage">
            <RefreshCw :size="16" aria-hidden="true" /> 刷新
          </button>
          <button
            ref="importAction"
            class="aw-primary-button"
            type="button"
            :aria-expanded="importOpen"
            :disabled="importOpen"
            @click="startQualityImport"
          >
            <UploadCloud :size="16" aria-hidden="true" /> 导入 Excel
          </button>
        </div>
      </div>
      <div v-if="importOpen && !reviewFile" class="aw-quality-history__import-tools" aria-label="选择导入文件" role="group">
        <label class="aw-month-control">
          <span>导入结算年月</span>
          <input ref="importMonthInput" v-model="importMonth" type="month" lang="zh-CN" :max="latestImportMonth" required />
        </label>
        <button
          class="aw-secondary-button"
          :class="{ 'is-active': dragActive }"
          type="button"
          :disabled="!importMonthReady"
          @click="chooseFile"
          @dragenter.prevent="dragActive = importMonthReady"
          @dragover.prevent
          @dragleave.prevent="dragActive = false"
          @drop.prevent="handleDrop"
        >选择或拖入 Excel</button>
        <button class="aw-secondary-button" type="button" @click="downloadTemplate">
          <Download :size="16" aria-hidden="true" /> 下载模板
        </button>
        <button class="aw-secondary-button" type="button" @click="cancelQualityImport">取消</button>
      </div>
      <input ref="fileInputRef" type="file" accept=".xlsx" aria-label="选择出错记录 Excel" hidden
        :disabled="!importOpen || !importMonthReady || !!reviewFile" @change="handleFileInput" />
      <div role="status" class="aw-quality-history__feedback">
        <p v-if="importFeedback" class="aw-inline-alert" :class="{ 'aw-inline-alert--warning': importWarning }">{{ importFeedback }}</p>
        <p v-else-if="importOpen && !importMonthReady && !reviewFile" class="aw-copy">先选择结算年月，再选择 Excel。</p>
      </div>
      <p v-if="importError" class="aw-inline-alert aw-inline-alert--error" role="alert">{{ importError }}</p>

      <p v-if="!canPreviewSettlement" class="aw-copy">结算人员可查看已导入记录。你可以在这里导入并查看本次结果。</p>
      <template v-else>
        <p v-if="historyLoading" class="aw-copy" role="status">正在读取导入记录…</p>
        <p v-else-if="historyError" class="aw-inline-alert aw-inline-alert--error" role="alert">{{ historyError }}</p>
        <div v-if="history.length" class="aw-quality-history__table">
          <table aria-label="本月质检导入记录">
            <thead>
              <tr><th scope="col">文件</th><th scope="col">导入时间</th><th scope="col">匹配行数</th><th scope="col">需复核</th><th scope="col">质检应扣款</th><th scope="col">操作</th></tr>
            </thead>
            <tbody>
              <tr v-for="batch in history" :key="batch.id">
                <th scope="row">{{ batch.original_filename || batch.import_no }}</th>
                <td>{{ formatShanghaiDateTime(batch.created_at) }}</td>
                <td>{{ formatInt(batch.matched_rows) }} / {{ formatInt(batch.total_rows) }}</td>
                <td>{{ formatInt(batch.unmatched_rows + batch.ambiguous_rows) }} 行</td>
                <td class="aw-quality-history__amount">{{ formatMoney(batch.deduction_amount) }}</td>
                <td>
                  <button class="aw-secondary-button" type="button" :aria-expanded="selectedImportID === batch.id"
                    :aria-label="`${selectedImportID === batch.id ? '收起' : '查看'}明细：${batch.original_filename || batch.import_no}`"
                    @click="openHistoryDetail(batch.id)">{{ selectedImportID === batch.id ? '收起明细' : '查看明细' }}</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else-if="!historyLoading && !historyError" class="aw-copy">本月还没有质检导入记录。</p>
        <div v-if="historyLastPage > 1" class="aw-inline-actions aw-quality-history__pagination">
          <button class="aw-secondary-button" type="button" :disabled="historyLoading || historyPage <= 1" @click="changeHistoryPage(-1)">上一页</button>
          <span class="aw-copy">{{ historyPage }} / {{ historyLastPage }}</span>
          <button class="aw-secondary-button" type="button" :disabled="historyLoading || historyPage >= historyLastPage" @click="changeHistoryPage(1)">下一页</button>
        </div>
        <section v-if="selectedImportID !== null" class="aw-quality-history__detail" aria-label="导入明细">
          <div class="aw-quality-history__toolbar">
            <h4>{{ historyDetail?.original_filename || historyDetail?.import_no || '导入明细' }}</h4>
            <RouterLink class="aw-link-button" :to="{ path: '/settlement', query: { month }, hash: '#aw-quality-import-history' }">
              管理导入批次
            </RouterLink>
          </div>
          <p v-if="historyDetailLoading" class="aw-copy" role="status">正在读取导入明细…</p>
          <p v-else-if="historyDetailError" class="aw-inline-alert aw-inline-alert--error" role="alert">{{ historyDetailError }}</p>
          <div v-else-if="historyDetail?.records?.length" class="aw-quality-history__table">
            <table aria-label="出错记录明细">
              <thead><tr><th scope="col">日期</th><th scope="col">出错人</th><th scope="col">难度</th><th scope="col">出错张数</th><th scope="col">应扣款</th><th scope="col">匹配结果</th></tr></thead>
              <tbody>
                <tr v-for="record in visibleHistoryDetail" :key="record.id">
                  <td>{{ record.occurred_date?.slice(0, 10) || '—' }}</td><td>{{ record.payee_name || '—' }}</td>
                  <td>{{ record.difficulty_class || '—' }}</td><td>{{ formatInt(record.error_count) }}</td>
                  <td>{{ formatMoney(record.deduction_amount) }}</td><td>{{ matchStatusLabel(record.match_status) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-else class="aw-copy">该文件没有可显示的记录。</p>
          <div v-if="historyDetailLastPage > 1" class="aw-inline-actions aw-quality-history__pagination">
            <button class="aw-secondary-button" type="button" :disabled="historyDetailPage <= 1" @click="historyDetailPage--">上一页明细</button>
            <span class="aw-copy">{{ historyDetailPage }} / {{ historyDetailLastPage }}</span>
            <button class="aw-secondary-button" type="button" :disabled="historyDetailPage >= historyDetailLastPage" @click="historyDetailPage++">下一页明细</button>
          </div>
        </section>
      </template>
      <div v-if="reviewFile" ref="importReview" class="aw-quality-history__review" tabindex="-1">
        <QualityImportReview :file="reviewFile" :business-month="importMonth"
          @close="cancelQualityImport" @imported="handleQualityImported" />
      </div>
    </section>

    <p v-if="notice" class="aw-inline-alert" role="status">{{ notice }}</p>
    <p v-if="error" class="aw-inline-alert aw-inline-alert--error" role="alert">{{ error }}</p>
    <main class="aw-quality-errors__workspace">
      <section v-if="canPreviewSettlement" class="aw-quality-errors__rules aw-data-surface">
        <div class="aw-settlement-section__head">
          <div>
            <h3 id="aw-actual-deductions">质检实际扣款</h3>
            <p>{{ month }} · 按人员和工资条类型录入，以最新金额为准。实发按实际扣款计算，未录入或填 0 时不扣。</p>
          </div>
        </div>
        <p v-if="actualDeductions?.locked" class="aw-inline-alert">该月已有生成或确认的批次，实扣金额已锁定。</p>
        <form class="aw-form-grid" @submit.prevent="saveActualDeduction">
          <PersonnelPicker v-model="deductionPayeeID" label="扣款人员" clearable @selected="fillSavedDeduction" @cleared="fillSavedDeduction" />
          <label>
            工资条类型
            <select v-model="deductionRowType" aria-label="工资条类型" :disabled="savingDeduction" @change="fillSavedDeduction">
              <option value="normal_piecework">日常计件工资</option>
              <option value="supplement_piecework">补录计件工资</option>
            </select>
          </label>
          <label>
            质检实际扣款（元）
            <input v-model="deductionInput" type="number" min="0" max="9999999999.99" step="0.01" required :disabled="!canMaintainActualDeduction" />
          </label>
          <p v-if="canEditActualDeduction && deductionPayeeID && !hasMatchedQualityRecords" class="aw-copy aw-form-grid__full">
            所选人员、月份和工资条类型没有匹配的质检导入记录。请先导入后再维护实际扣款。
          </p>
          <div class="aw-inline-actions aw-form-grid__full">
            <span v-if="selectedPayroll">质检应扣 {{ formatMoney(selectedPayroll.deduction_amount) }}</span>
            <span v-if="selectedPayroll">应发 {{ formatMoney(selectedPayroll.net_amount) }}</span>
            <span v-if="draftActualNet !== null" class="aw-cell-money" role="status">保存前实发预览 {{ formatMoney(draftActualNet) }}</span>
            <span v-else-if="selectedPayroll">实发 {{ formatMoney(selectedPayroll.actual_net_amount ?? selectedPayroll.net_amount) }}</span>
            <button class="aw-primary-button" type="submit" :disabled="!canMaintainActualDeduction || enteredDeduction === null">
              {{ savingDeduction ? '保存中…' : '保存实际扣款' }}
            </button>
          </div>
        </form>
      </section>

      <section class="aw-quality-errors__rules">
        <div class="aw-settlement-section__head">
          <div>
            <h3 id="aw-deduction-rules">本月扣款规则</h3>
            <p>导入成功后，系统按下面的“每张扣款”自动计算，不读取 Excel 中的金额。</p>
          </div>
          <RouterLink v-if="canManageRules" class="aw-secondary-button" :to="{ path: '/settings/pricing', query: { section: 'deduction' } }">
            <Calculator :size="16" aria-hidden="true" />
            维护扣款规则
          </RouterLink>
        </div>
        <p v-if="!canViewRules" class="aw-inline-alert">
          当前账号可以导入出错记录，但不能查看工资汇总或扣款规则；导入结果仍会由结算人员复核。
        </p>
        <WorkbenchDataGrid
          v-else-if="ruleGridRows.length"
          :columns="ruleColumns"
          :rows="ruleGridRows"
          row-key="id"
          storage-key="quality-error-deduction-rules"
          :height="Math.min(360, 48 + ruleGridRows.length * 40)"
          :row-height="40"
          aria-label="本月生效扣款规则"
        >
          <template #cell="{ column, value }">
            <span v-if="column.key === 'worker_type_label'" :class="chipClass('info')">{{ value }}</span>
            <span v-else-if="column.key === 'enabled_label'" :class="chipClass('success')">{{ value }}</span>
            <span v-else>{{ value }}</span>
          </template>
        </WorkbenchDataGrid>
        <div v-else class="aw-empty-state">
          <FileSpreadsheet :size="28" aria-hidden="true" />
          <h3>本月没有可用扣款规则</h3>
          <p>可以先导入记录，但没有匹配规则的行不会产生扣款金额。请先到计价设置维护规则。</p>
          <RouterLink v-if="canManageRules" class="aw-primary-button" to="/settings/pricing">去设置每张扣款</RouterLink>
        </div>
      </section>
    </main>
  </section>
</template>
