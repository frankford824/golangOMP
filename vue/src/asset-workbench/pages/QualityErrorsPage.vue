<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { Calculator, CheckCircle2, Download, FileSpreadsheet, RefreshCw, UploadCloud } from 'lucide-vue-next'

import {
  assetWorkbenchApi,
  type DeductionRuleRow,
  type SettlementPreview,
} from '@aw/shared/api/assetWorkbenchApi'
import { exportErrorImportTemplateWorkbook } from '@aw/features/export/settlementExport'
import { currentBusinessMonth } from '@aw/shared/format/businessMonth'
import { formatInt, formatMoney } from '@aw/shared/format/number'
import { chipClass, workerTypeMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import SpreadsheetWorkbench from '@aw/shared/spreadsheet/SpreadsheetWorkbench.vue'
import { buildImportReviewSource, workbookReviewRowsToFiles } from '@aw/shared/spreadsheet/excelReview'
import type { WorkbenchSpreadsheetActionPayload, WorkbenchSpreadsheetSource } from '@aw/shared/spreadsheet/types'
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
const importing = ref(false)
const dragActive = ref(false)
const notice = ref('')
const error = ref('')
const fileInputRef = ref<HTMLInputElement | null>(null)
const reviewSource = ref<WorkbenchSpreadsheetSource | null>(null)
const reviewRevision = ref(0)

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

function ruleAppliesToMonth(rule: DeductionRuleRow, businessMonth: string) {
  if (!rule.enabled) return false
  const startMonth = rule.effective_from.slice(0, 7)
  const endMonth = rule.effective_to?.slice(0, 7) || ''
  return startMonth <= businessMonth && (!endMonth || endMonth >= businessMonth)
}

async function loadQualityOverview(options: { keepFeedback?: boolean } = {}) {
  loading.value = true
  if (!options.keepFeedback) {
    notice.value = ''
    error.value = ''
  }
  try {
    const [previewResult, rulesResult] = await Promise.all([
      canPreviewSettlement.value ? assetWorkbenchApi.previewSettlement(month.value) : Promise.resolve(null),
      canViewRules.value ? assetWorkbenchApi.listDeductionRules({ page: 1, page_size: 100 }) : Promise.resolve(null),
    ])
    preview.value = previewResult
    deductionRules.value = rulesResult?.items ?? []
  } catch (err) {
    error.value = resolveApiUserMessage(err, { fallback: '出错记录数据加载失败，请稍后重试' })
  } finally {
    loading.value = false
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

async function handleFileInput(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (file) await prepareReview(file)
}

async function handleDrop(event: DragEvent) {
  dragActive.value = false
  const file = Array.from(event.dataTransfer?.files ?? []).find((item) => /\.(xlsx|xls)$/i.test(item.name))
  if (!file) {
    error.value = '请选择 Excel 文件（.xlsx 或 .xls）'
    return
  }
  await prepareReview(file)
}

async function prepareReview(file: File) {
  importing.value = true
  notice.value = ''
  error.value = ''
  try {
    reviewSource.value = await buildImportReviewSource('error-deduction', [file], ++reviewRevision.value)
    notice.value = `已载入 ${file.name}，请先核对必填项，再确认导入到 ${month.value}。`
  } catch (err) {
    reviewSource.value = null
    error.value = resolveApiUserMessage(err, { fallback: 'Excel 读取失败，请检查文件格式' })
  } finally {
    importing.value = false
  }
}

async function handleReviewAction(payload: WorkbenchSpreadsheetActionPayload) {
  if (payload.action.key === 'cancel_import') {
    reviewSource.value = null
    notice.value = ''
    return
  }
  if (payload.action.key !== 'confirm_import' || !reviewSource.value || importing.value) return
  importing.value = true
  notice.value = ''
  error.value = ''
  try {
    const files = await workbookReviewRowsToFiles(reviewSource.value, payload.sheets, 'asset-workbench-quality-errors')
    let matched = 0
    let unmatched = 0
    let ambiguous = 0
    let total = 0
    for (const file of files) {
      const result = await assetWorkbenchApi.importErrorExcel(month.value, file)
      matched += result.matched_rows
      unmatched += result.unmatched_rows
      ambiguous += result.ambiguous_rows
      total += result.total_rows
    }
    lastImportWarningCount.value = unmatched + ambiguous
    notice.value = `已导入 ${formatInt(total)} 行：成功匹配 ${formatInt(matched)} 行，未匹配 ${formatInt(unmatched)} 行，多人重名 ${formatInt(ambiguous)} 行。`
    reviewSource.value = null
    await loadQualityOverview({ keepFeedback: true })
  } catch (err) {
    error.value = resolveApiUserMessage(err, { fallback: '出错记录导入失败，请检查表格内容后重试' })
  } finally {
    importing.value = false
  }
}

watch(month, () => {
  reviewSource.value = null
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
          <input v-model="month" type="month" lang="zh-CN" aria-label="出错记录月份" />
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
            accept=".xlsx,.xls"
            aria-label="选择出错记录 Excel"
            @change="handleFileInput"
          />
          <SpreadsheetWorkbench
            v-if="reviewSource"
            :source="reviewSource"
            :height="470"
            @close="reviewSource = null"
            @action="handleReviewAction"
          />
          <button
            v-else
            class="aw-quality-errors__dropzone"
            :class="{ 'is-active': dragActive }"
            type="button"
            :disabled="importing"
            @click="chooseFile"
            @dragenter.prevent="dragActive = true"
            @dragover.prevent="dragActive = true"
            @dragleave.prevent="dragActive = false"
            @drop.prevent="handleDrop"
          >
            <UploadCloud :size="30" aria-hidden="true" />
            <strong>{{ importing ? '正在读取 Excel…' : '选择或拖入出错记录 Excel' }}</strong>
            <span>导入前会先显示表格校对，不会直接写入。</span>
          </button>

          <div class="aw-quality-errors__required" aria-label="Excel 必填列">
            <span><CheckCircle2 :size="15" aria-hidden="true" />日期</span>
            <span><CheckCircle2 :size="15" aria-hidden="true" />出错人</span>
            <span><CheckCircle2 :size="15" aria-hidden="true" />出错分类</span>
            <span><CheckCircle2 :size="15" aria-hidden="true" />出错张数</span>
          </div>
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
