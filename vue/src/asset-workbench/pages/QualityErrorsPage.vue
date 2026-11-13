<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { Calculator, CheckCircle2, Download, FileSpreadsheet, RefreshCw, UploadCloud } from 'lucide-vue-next'

import {
  assetWorkbenchApi,
  type DeductionRuleRow,
  type ErrorImportBatchRow,
  type SettlementPreview,
  type SettlementPayrollRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { exportErrorImportTemplateWorkbook } from '@aw/features/export/settlementExport'
import { currentBusinessMonth } from '@aw/shared/format/businessMonth'
import { formatInt, formatMoney } from '@aw/shared/format/number'
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

const month = ref(currentBusinessMonth())
const session = useAssetWorkbenchSessionStore()
const preview = ref<SettlementPreview | null>(null)
const deductionRules = ref<DeductionRuleRow[]>([])
const lastImportWarningCount = ref(0)
const loading = ref(false)
const dragActive = ref(false)
const notice = ref('')
const error = ref('')
const fileInputRef = ref<HTMLInputElement | null>(null)
const reviewFile = ref<File | null>(null)
const actualDeductions = ref<Awaited<ReturnType<typeof assetWorkbenchApi.qualityActualDeductions>> | null>(null)
const deductionPayeeID = ref(0)
const deductionRowType = ref<SettlementPayrollRow['row_type']>('normal_piecework')
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
const importHasWarnings = computed(() => lastImportWarningCount.value > 0)
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
)?.quality_actual_deduction_amount ?? 0)
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
  if (!row || amount === null || !hasUnsavedDeduction.value || actualDeductions.value?.locked ||
    (row.item_count === 0 && row.error_count === 0 && row.welfare_amount === 0)) return null
  const applicable = amount === 0 ? row.deduction_amount : amount
  const difference = Math.round((row.deduction_amount - applicable) * 10000) / 10000
  return row.net_amount + difference
})

function fillSavedDeduction() {
  deductionInput.value = savedDeduction.value > 0 ? String(savedDeduction.value) : ''
}

async function saveActualDeduction() {
  const amount = enteredDeduction.value
  if (!canEditActualDeduction.value || !deductionPayeeID.value || amount === null) return
  const targetMonth = month.value
  const targetPayee = deductionPayeeID.value
  savingDeduction.value = true
  error.value = ''
  notice.value = ''
  try {
    await assetWorkbenchApi.saveQualityActualDeduction(targetMonth, targetPayee, amount, deductionRowType.value)
    if (month.value !== targetMonth) return
    notice.value = amount === 0 ? '已恢复按质检应扣计算。' : '已保存质检实际扣款。'
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
  error.value = ''
  try {
    await exportErrorImportTemplateWorkbook()
    notice.value = '出错记录模板已生成，请填写前四个必填项后再导入。'
  } catch (err) {
    error.value = resolveApiUserMessage(err, { fallback: '模板生成失败，请稍后重试' })
  }
}

function chooseFile() {
  fileInputRef.value?.click()
}

function handleFileInput(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (file) prepareReview(file)
}

function handleDrop(event: DragEvent) {
  dragActive.value = false
  const file = Array.from(event.dataTransfer?.files ?? []).find((item) => /\.xlsx$/i.test(item.name))
  if (!file) {
    error.value = '请选择 .xlsx 格式的 Excel 文件'
    return
  }
  prepareReview(file)
}

function prepareReview(file: File) {
  notice.value = ''
  error.value = ''
  reviewFile.value = file
}

async function handleQualityImported(result: ErrorImportBatchRow) {
  lastImportWarningCount.value = result.unmatched_rows + result.ambiguous_rows
  notice.value = `已导入 ${formatInt(result.total_rows)} 行，${formatInt(result.matched_rows)} 行计入扣款，${formatInt(lastImportWarningCount.value)} 行未计入扣款。`
  reviewFile.value = null
  await loadQualityOverview({ keepFeedback: true })
}

watch(month, () => {
  reviewFile.value = null
  lastImportWarningCount.value = 0
  void loadQualityOverview()
})

onMounted(() => void loadQualityOverview())
</script>

<template>
  <section class="aw-page-stack aw-quality-errors-page">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">出错记录</p>
        <h2>导入出错表，自动计算扣款</h2>
        <p>这里只处理脱机 Excel，不连接生产质检审批。系统按出错日期、人员、分类和张数自动换算扣款，表格中不需要填写金额。</p>
      </div>
      <div class="aw-page-bar__actions">
        <label class="aw-month-control">
          <span>扣款月份</span>
          <input v-model="month" type="month" lang="zh-CN" aria-label="出错记录月份" :disabled="savingDeduction || !!reviewFile" />
        </label>
        <button class="aw-secondary-button" type="button" :disabled="loading" @click="loadQualityOverview()">
          <RefreshCw :size="16" aria-hidden="true" />
          刷新
        </button>
      </div>
    </div>

    <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
    <p v-if="error" class="aw-inline-alert aw-inline-alert--error" role="alert">{{ error }}</p>
    <p v-if="importHasWarnings" class="aw-inline-alert aw-inline-alert--warning">
      上次导入有未匹配或重名记录，这些行不会参与扣款。请修正姓名或出错分类后重新导入。
    </p>

    <div class="aw-metric-grid aw-quality-errors__metrics">
      <article class="aw-metric-card">
        <span>{{ month }} 出错张数</span>
        <strong>{{ canPreviewSettlement ? formatInt(totalErrors) : '—' }}</strong>
        <small>{{ canPreviewSettlement ? '已成功匹配并进入本月计算' : '需要结算权限查看本月汇总' }}</small>
      </article>
      <article class="aw-metric-card">
        <span>预计扣款</span>
        <strong class="aw-money">{{ canPreviewSettlement ? formatMoney(totalDeduction) : '—' }}</strong>
        <small>{{ canPreviewSettlement ? '刷新后与结算预览保持一致' : '导入权限不代表工资查看权限' }}</small>
      </article>
      <article class="aw-metric-card">
        <span>本月生效规则</span>
        <strong>{{ canViewRules ? formatInt(activeRules.length) : '—' }}</strong>
        <small>{{ canViewRules ? '按人员类型、岗级和出错分类匹配' : '由结算或计价管理员维护' }}</small>
      </article>
    </div>

    <div class="aw-quality-errors__workbench">
      <aside class="aw-quality-errors__steps" aria-label="出错记录导入步骤">
        <div>
          <span>1</span>
          <strong>下载模板</strong>
          <p>使用统一列名，避免姓名、分类或日期无法识别。</p>
        </div>
        <div>
          <span>2</span>
          <strong>填写四项必填</strong>
          <p>日期、出错人、出错分类、出错张数；其他信息均可选填。</p>
        </div>
        <div>
          <span>3</span>
          <strong>校对后导入</strong>
          <p>系统先标出缺失内容，确认后才写入并参与结算。</p>
        </div>
      </aside>

      <main class="aw-quality-errors__workspace">
        <section class="aw-quality-errors__import">
          <div class="aw-settlement-section__head">
            <div>
              <h3>导入 {{ month }} 出错记录</h3>
              <p>同一张图片出现多个错误时，按实际扣款张数填写，不按问题描述条数重复计算。</p>
            </div>
            <button class="aw-secondary-button" type="button" @click="downloadTemplate">
              <Download :size="16" aria-hidden="true" />
              下载模板
            </button>
          </div>

          <input
            ref="fileInputRef"
            class="aw-visually-hidden"
            type="file"
            accept=".xlsx"
            aria-label="选择出错记录 Excel"
            @change="handleFileInput"
          />
          <QualityImportReview
            v-if="reviewFile"
            :file="reviewFile"
            :month="month"
            @close="reviewFile = null"
            @imported="handleQualityImported"
          />
          <button
            v-else
            class="aw-quality-errors__dropzone"
            :class="{ 'is-active': dragActive }"
            type="button"
            @click="chooseFile"
            @dragenter.prevent="dragActive = true"
            @dragover.prevent="dragActive = true"
            @dragleave.prevent="dragActive = false"
            @drop.prevent="handleDrop"
          >
            <UploadCloud :size="30" aria-hidden="true" />
            <strong>选择或拖入出错记录 Excel</strong>
            <span>填写订单号或文件名可自动识别日常与补录作品。</span>
          </button>

          <div class="aw-quality-errors__required" aria-label="Excel 必填列">
            <span><CheckCircle2 :size="15" aria-hidden="true" />日期</span>
            <span><CheckCircle2 :size="15" aria-hidden="true" />出错人</span>
            <span><CheckCircle2 :size="15" aria-hidden="true" />出错分类</span>
            <span><CheckCircle2 :size="15" aria-hidden="true" />出错张数</span>
          </div>
        </section>

        <section v-if="canPreviewSettlement" class="aw-quality-errors__rules aw-data-surface">
          <div class="aw-settlement-section__head">
            <div>
              <h3>质检实际扣款</h3>
              <p>{{ month }} · 按人员和工资条类型录入。填 0 恢复按该行质检应扣计算。</p>
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
              <input v-model="deductionInput" type="number" min="0" max="9999999999.99" step="0.01" required :disabled="!canEditActualDeduction || !deductionPayeeID" />
            </label>
            <div class="aw-inline-actions aw-form-grid__full">
              <span v-if="selectedPayroll">质检应扣 {{ formatMoney(selectedPayroll.deduction_amount) }}</span>
              <span v-if="selectedPayroll">应发 {{ formatMoney(selectedPayroll.net_amount) }}</span>
              <span v-if="draftActualNet !== null" class="aw-cell-money" role="status">保存前实发预览 {{ formatMoney(draftActualNet) }}</span>
              <span v-else-if="selectedPayroll">实发 {{ formatMoney(selectedPayroll.actual_net_amount ?? selectedPayroll.net_amount) }}</span>
              <button class="aw-primary-button" type="submit" :disabled="!canEditActualDeduction || !deductionPayeeID || enteredDeduction === null">
                {{ savingDeduction ? '保存中…' : '保存实际扣款' }}
              </button>
            </div>
          </form>
        </section>

        <section class="aw-quality-errors__rules">
          <div class="aw-settlement-section__head">
            <div>
              <h3>本月扣款规则</h3>
              <p>导入成功后，系统按下面的“每张扣款”自动计算，不读取 Excel 中的金额。</p>
            </div>
            <RouterLink v-if="canManageRules" class="aw-secondary-button" to="/settings/pricing">
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
    </div>
  </section>
</template>
