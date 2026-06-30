<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Download, RefreshCw } from 'lucide-vue-next'

import SettlementHubTabs from '@aw/shared/console/SettlementHubTabs.vue'
import { exportSettlementReportWorkbook } from '@aw/features/export/settlementExport'
import { assetWorkbenchApi, type SettlementReport, type SettlementReportDifficultyMetric, type SettlementReportRow } from '@aw/shared/api/assetWorkbenchApi'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { formatInt, formatMoney, formatPercent } from '@aw/shared/format/number'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'

interface GridColumn {
  key: string
  label: string
  width: number
  align?: 'left' | 'right' | 'center'
}

type ReportGridRow = Record<string, unknown> & {
  grid_id: string
  row_type: SettlementReportRow['row_type']
}

const month = ref(defaultBusinessMonth())
const exporting = ref(false)

const reportRequest = usePageRequest<SettlementReport>(
  (signal) => assetWorkbenchApi.settlementReport(month.value, signal),
  null,
  '计件报表加载失败',
)
const loading = reportRequest.loading
const error = reportRequest.error
const report = computed(() => reportRequest.data.value)
const totals = computed(() => report.value?.totals)
const difficultyClasses = computed(() => report.value?.difficulty_classes ?? [])
const normalRows = computed(() => buildGridRows(domainRows.value.filter((row) => row.row_type === 'normal_piecework')))
const supplementRows = computed(() => buildGridRows(domainRows.value.filter((row) => row.row_type === 'supplement_piecework')))
const domainRows = computed(() => report.value?.rows ?? [])
const summaryCards = computed(() => {
  const row = totals.value
  return [
    { label: '订单数', value: formatInt(row?.order_count), hint: '非空订单号去重' },
    { label: '作图量', value: formatInt(row?.page_count), hint: `${formatInt(row?.item_count)} 单` },
    { label: '出错扣减', value: formatMoney(row?.deduction_amount), hint: `${formatInt(row?.error_count)} 个出错` },
    { label: '应结净额', value: formatMoney(row?.net_amount), hint: `正常 ${formatMoney(row?.gross_amount)} · 补录 ${formatMoney(row?.supplement_amount)}`, money: true },
  ]
})
const difficultySummary = computed(() => totals.value?.difficulty_metrics ?? [])

const baseColumns: GridColumn[] = [
  { key: 'creator_name', label: '创建人', width: 132 },
  { key: 'job_grade', label: '岗级', width: 84 },
  { key: 'created_date', label: '创建日期', width: 112 },
  { key: 'order_count', label: '订单数', width: 88, align: 'right' },
  { key: 'item_count', label: '单数', width: 80, align: 'right' },
  { key: 'page_count', label: '作图量', width: 88, align: 'right' },
  { key: 'gross_amount', label: '正常金额', width: 112, align: 'right' },
  { key: 'supplement_amount', label: '补录金额', width: 112, align: 'right' },
  { key: 'error_count', label: '出错数', width: 84, align: 'right' },
  { key: 'deduction_amount', label: '扣减', width: 104, align: 'right' },
  { key: 'welfare_amount', label: '福利', width: 104, align: 'right' },
  { key: 'net_amount', label: '净额', width: 112, align: 'right' },
  { key: 'error_rate', label: '出错率', width: 92, align: 'right' },
  { key: 'page_count_share', label: '月作图占比', width: 116, align: 'right' },
  { key: 'error_count_share', label: '月出错占比', width: 116, align: 'right' },
  { key: 'month_amount_share', label: '月金额占比', width: 116, align: 'right' },
]
const columns = computed<GridColumn[]>(() => [
  ...baseColumns,
  ...difficultyClasses.value.flatMap((difficulty) => difficultyColumns(difficulty)),
])
const moneyColumns = new Set(['gross_amount', 'supplement_amount', 'deduction_amount', 'welfare_amount', 'net_amount'])
const intColumns = new Set(['order_count', 'item_count', 'page_count', 'error_count'])
const percentColumns = new Set(['error_rate', 'page_count_share', 'error_count_share', 'month_amount_share'])

function loadReport() {
  void reportRequest.run()
}

async function exportReport() {
  if (!report.value) return
  exporting.value = true
  try {
    await exportSettlementReportWorkbook({ businessMonth: month.value, report: report.value })
  } finally {
    exporting.value = false
  }
}

function buildGridRows(rows: SettlementReportRow[]): ReportGridRow[] {
  return rows.map((row) => {
    const output: ReportGridRow = {
      ...row,
      grid_id: `${row.payee_user_id}-${row.row_type}`,
      row_type: row.row_type,
    }
    const metrics = new Map(row.difficulty_metrics.map((metric) => [metric.difficulty_class, metric]))
    for (const difficulty of difficultyClasses.value) {
      const metric = metrics.get(difficulty)
      Object.assign(output, flattenDifficultyForGrid(difficulty, metric))
    }
    delete output.difficulty_metrics
    return output
  })
}

function flattenDifficultyForGrid(difficulty: string, metric?: SettlementReportDifficultyMetric) {
  return {
    [difficultyKey(difficulty, 'item_count')]: metric?.item_count ?? 0,
    [difficultyKey(difficulty, 'page_count')]: metric?.page_count ?? 0,
    [difficultyKey(difficulty, 'gross_amount')]: metric?.gross_amount ?? 0,
    [difficultyKey(difficulty, 'error_count')]: metric?.error_count ?? 0,
    [difficultyKey(difficulty, 'deduction_amount')]: metric?.deduction_amount ?? 0,
    [difficultyKey(difficulty, 'page_count_share')]: metric?.page_count_share ?? 0,
  }
}

function difficultyColumns(difficulty: string): GridColumn[] {
  const label = difficultyLabel(difficulty)
  return [
    { key: difficultyKey(difficulty, 'item_count'), label: `${label}单`, width: 84, align: 'right' },
    { key: difficultyKey(difficulty, 'page_count'), label: `${label}量`, width: 84, align: 'right' },
    { key: difficultyKey(difficulty, 'gross_amount'), label: `${label}金额`, width: 104, align: 'right' },
    { key: difficultyKey(difficulty, 'error_count'), label: `${label}错`, width: 84, align: 'right' },
    { key: difficultyKey(difficulty, 'deduction_amount'), label: `${label}扣`, width: 104, align: 'right' },
    { key: difficultyKey(difficulty, 'page_count_share'), label: `${label}占比`, width: 104, align: 'right' },
  ]
}

function gridValue(key: string, value: unknown) {
  if (isMoneyColumn(key)) return formatMoney(value)
  if (isPercentColumn(key)) return formatPercent(Number(value ?? 0) * 100)
  if (isIntColumn(key)) return formatInt(value)
  return value || '—'
}

function isMoneyColumn(key: string) {
  return moneyColumns.has(key) || key.endsWith('_gross_amount') || key.endsWith('_deduction_amount')
}

function isIntColumn(key: string) {
  return intColumns.has(key) || key.endsWith('_item_count') || key.endsWith('_page_count') || key.endsWith('_error_count')
}

function isPercentColumn(key: string) {
  return percentColumns.has(key) || key.endsWith('_page_count_share')
}

function difficultyKey(difficulty: string, field: string): string {
  return `difficulty_${difficulty.replace(/[^a-zA-Z0-9]+/g, '_')}_${field}`
}

function difficultyLabel(value: string): string {
  return value === 'unclassified' ? '未定级' : value
}

function defaultBusinessMonth() {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
  }).formatToParts(new Date())
  const year = parts.find((part) => part.type === 'year')?.value ?? new Date().getFullYear().toString()
  const monthPart = parts.find((part) => part.type === 'month')?.value ?? '01'
  return `${year}-${monthPart}`
}

onMounted(loadReport)
</script>

<template>
  <section class="aw-page-stack aw-reports-page">
    <SettlementHubTabs />
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">计件统计</p>
        <h2>计件报表</h2>
        <p>订单数按非空订单号去重，金额按当前待结算成品和已批准补录计算。</p>
      </div>
    </div>

    <section class="aw-panel">
      <div class="aw-report-toolbar">
        <label class="aw-field aw-report-toolbar__month">
          业务月
          <input v-model="month" type="month" />
        </label>
        <div class="aw-report-toolbar__actions">
          <button class="aw-secondary-button" type="button" :disabled="loading" @click="loadReport">
            <RefreshCw :size="16" aria-hidden="true" />
            刷新
          </button>
          <button class="aw-primary-button" type="button" :disabled="exporting || !report" @click="exportReport">
            <Download :size="16" aria-hidden="true" />
            导出报表
          </button>
        </div>
        <div class="aw-report-toolbar__notes">
          <span class="aw-chip aw-chip--neutral">订单数=非空订单号去重</span>
          <span class="aw-chip aw-chip--info">数据=待结算成品+已批准补录</span>
        </div>
      </div>
      <AsyncBoundary :loading="loading" :error="error" :empty="!report" loading-label="正在加载计件报表" empty-label="暂无报表数据" @retry="loadReport">
        <div class="aw-metric-grid">
          <article v-for="card in summaryCards" :key="card.label" class="aw-metric-card">
            <span>{{ card.label }}</span>
            <strong :class="{ 'aw-money': card.money }">{{ card.value }}</strong>
            <small>{{ card.hint }}</small>
          </article>
        </div>
      </AsyncBoundary>
    </section>

    <AsyncBoundary :loading="loading" :error="error" :empty="!report" loading-label="正在加载计件报表" empty-label="暂无报表数据" @retry="loadReport">
      <section class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <h3>难度构成</h3>
            <p class="aw-copy">按全月作图量拆分，金额为对应难度的小计金额。</p>
          </div>
        </div>
        <div v-if="difficultySummary.length" class="aw-metric-grid">
          <article v-for="metric in difficultySummary" :key="metric.difficulty_class" class="aw-metric-card">
            <span>{{ difficultyLabel(metric.difficulty_class) }}</span>
            <strong>{{ formatInt(metric.page_count) }}</strong>
            <small>{{ formatMoney(metric.gross_amount) }} · 出错 {{ formatInt(metric.error_count) }} · {{ formatPercent(metric.month_page_count_share * 100) }}</small>
          </article>
        </div>
        <div v-else class="aw-empty-state">
          <h3>没有难度明细</h3>
          <p>当前业务月没有可展示的计件或补录数据。</p>
        </div>
      </section>

      <section class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <h3>正常计件</h3>
            <p class="aw-copy">正常计件包含结算时扣减、福利与难度分列。</p>
          </div>
        </div>
        <WorkbenchDataGrid
          v-if="normalRows.length"
          :columns="columns"
          :rows="normalRows"
          row-key="grid_id"
          storage-key="settlement-report-normal"
          :height="520"
          :row-height="36"
          aria-label="正常计件报表"
        >
          <template #cell="{ column, value }">
            <span v-if="isMoneyColumn(column.key)" class="aw-cell-money">{{ gridValue(column.key, value) }}</span>
            <span v-else-if="column.align === 'right'" class="aw-cell-num">{{ gridValue(column.key, value) }}</span>
            <span v-else class="aw-cell-text">{{ gridValue(column.key, value) }}</span>
          </template>
        </WorkbenchDataGrid>
        <div v-else class="aw-empty-state">
          <h3>没有正常计件</h3>
          <p>当前业务月没有待结算的正常计件明细。</p>
        </div>
      </section>

      <section class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <h3>补录计件</h3>
            <p class="aw-copy">补录计件单独成行，不混入正常计件工资行。</p>
          </div>
        </div>
        <WorkbenchDataGrid
          v-if="supplementRows.length"
          :columns="columns"
          :rows="supplementRows"
          row-key="grid_id"
          storage-key="settlement-report-supplement"
          :height="360"
          :row-height="36"
          aria-label="补录计件报表"
        >
          <template #cell="{ column, value }">
            <span v-if="isMoneyColumn(column.key)" class="aw-cell-money">{{ gridValue(column.key, value) }}</span>
            <span v-else-if="column.align === 'right'" class="aw-cell-num">{{ gridValue(column.key, value) }}</span>
            <span v-else class="aw-cell-text">{{ gridValue(column.key, value) }}</span>
          </template>
        </WorkbenchDataGrid>
        <div v-else class="aw-empty-state">
          <h3>没有补录计件</h3>
          <p>当前业务月没有已批准的补录计件。</p>
        </div>
      </section>
    </AsyncBoundary>
  </section>
</template>

<style scoped>
.aw-reports-page > .aw-page-bar {
  display: block;
}

.aw-reports-page .aw-page-bar__copy {
  width: 100%;
}

.aw-reports-page .aw-page-bar__copy h2 {
  white-space: nowrap;
}

</style>
