<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import {
  assetWorkbenchApi,
  type SettlementBatchDetail,
  type SettlementBatchRow,
  type SettlementPayrollRow,
  type SettlementPreview,
  type SettlementSupplementRow,
  type SupplementPermissionRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { exportSettlementWorkbook } from '@aw/features/export/settlementExport'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'

interface GridColumn {
  key: string
  label: string
  width: number
  align?: 'left' | 'right' | 'center'
}

type PayrollGridRow = SettlementPayrollRow & { grid_id: string; row_label: string }
type MetricGridRow = { id: string; metric: string; scope: string; note: string; amount: number }
type BatchGridRow = SettlementBatchRow & { status_label: string }
type BatchItemGridRow = {
  id: number
  item_type_label: string
  payee_user_id: number
  quantity: number
  direction_label: string
  amount: number
}
type PermissionGridRow = SupplementPermissionRow & { status_label: string; reason_label: string }

const month = ref(new Date().toISOString().slice(0, 7))
const preview = ref<SettlementPreview | null>(null)
const batches = ref<SettlementBatchRow[]>([])
const supplements = ref<SettlementSupplementRow[]>([])
const supplementPermissions = ref<SupplementPermissionRow[]>([])
const eligibleSupplementMonths = ref<string[]>([])
const selectedBatch = ref<SettlementBatchDetail | null>(null)
const pendingCancelBatch = ref<SettlementBatchRow | null>(null)
const cancelReason = ref('')
const loading = ref(false)
const exporting = ref(false)
const eligibleMonthsLoading = ref(false)
const error = ref('')
const notice = ref('')
const errorInputRef = ref<HTMLInputElement | null>(null)
const supplementForm = ref({
  payee_user_id: 0,
  order_no: '',
  difficulty_class: 'A',
  page_count: 1,
  gross_amount: 0,
  finalized: true,
})
const permissionForm = ref({
  payee_user_id: 0,
  enabled: true,
  reason: '',
})
const adjustmentForm = ref({
  payee_user_id: 0,
  adjustment_type: 'adjustment',
  direction: 'credit',
  amount: 0,
  reason: '',
})
const batchStatusLabels: Record<string, string> = {
  generated: '待确认',
  confirmed: '已确认',
  cancelled: '已取消',
  reversed: '已冲正',
}
const settlementItemTypeLabels: Record<string, string> = {
  gross_piecework: '正常计件',
  error_deduction: '出错扣减',
  welfare: '福利补贴',
  supplement: '补录计件',
  adjustment: '补差',
  reversal: '冲正',
}
const supplementStatusLabels: Record<string, string> = {
  pending: '待审核',
  approved: '已批准',
  rejected: '已拒绝',
  reversed: '已冲正',
}

const totals = computed(() => preview.value?.totals)
const payrollRows = computed(() => preview.value?.payroll_rows ?? [])
const totalMetricRows = computed(() => {
  const value = totals.value
  if (!value) return []
  return [
    { id: 'gross', metric: '正常计件', scope: `${value.item_count} 单`, note: `${value.page_count} 页`, amount: value.gross_amount },
    { id: 'deduction', metric: '出错扣减', scope: `${value.error_count} 个出错`, note: '按导入出错表计算', amount: value.deduction_amount },
    { id: 'supplement', metric: '补录计件', scope: '漏传补录', note: '单独工资行', amount: value.supplement_amount },
    { id: 'net', metric: '应结净额', scope: '本月合计', note: '正常工资行 + 补录工资行', amount: value.net_amount },
  ] satisfies MetricGridRow[]
})
const metricGridRows = computed(() => totalMetricRows.value as unknown as Record<string, unknown>[])
const payrollGridRows = computed(() => payrollRows.value.map(toPayrollGridRow) as unknown as Record<string, unknown>[])
const batchGridRows = computed(() => batches.value.map(toBatchGridRow) as unknown as Record<string, unknown>[])
const batchItemGridRows = computed(() => (selectedBatch.value?.items ?? []).map(toBatchItemGridRow) as unknown as Record<string, unknown>[])
const batchPayrollGridRows = computed(() => (selectedBatch.value?.payroll_rows ?? []).map(toPayrollGridRow) as unknown as Record<string, unknown>[])
const permissionGridRows = computed(() => supplementPermissions.value.map(toPermissionGridRow) as unknown as Record<string, unknown>[])
const supplementRowsWithLabels = computed(() =>
  supplements.value.map((row) => ({
    ...row,
    status_label: supplementStatusLabel(row.status),
    duplicate_label: row.duplicate_hint_json?.has_duplicates ? '可能重复' : '无重复提示',
  })),
)
const supplementGridRows = computed(() => supplementRowsWithLabels.value as unknown as Record<string, unknown>[])
const metricGridColumns = computed<GridColumn[]>(() => [
  { key: 'metric', label: '指标', width: 108 },
  { key: 'scope', label: '范围', width: 132 },
  { key: 'note', label: '说明', width: 160 },
  { key: 'amount', label: '金额', width: 112, align: 'right' },
])
const payrollGridColumns = computed<GridColumn[]>(() => [
  { key: 'payee_user_id', label: '人员', width: 96 },
  { key: 'row_label', label: '工资条', width: 140 },
  { key: 'item_count', label: '单数', width: 88, align: 'right' },
  { key: 'page_count', label: '页数', width: 88, align: 'right' },
  { key: 'gross_amount', label: '毛额', width: 100, align: 'right' },
  { key: 'deduction_amount', label: '扣减', width: 100, align: 'right' },
  { key: 'welfare_amount', label: '福利', width: 100, align: 'right' },
  { key: 'supplement_amount', label: '补录', width: 100, align: 'right' },
  { key: 'adjustment_amount', label: '调整', width: 100, align: 'right' },
  { key: 'net_amount', label: '净额', width: 112, align: 'right' },
])
const batchGridColumns = computed<GridColumn[]>(() => [
  { key: 'batch_no', label: '批次号', width: 190 },
  { key: 'business_month', label: '业务月', width: 108 },
  { key: 'status_label', label: '状态', width: 96 },
  { key: 'net_amount', label: '净额', width: 112, align: 'right' },
  { key: 'actions', label: '动作', width: 180, align: 'center' },
])
const batchItemGridColumns = computed<GridColumn[]>(() => [
  { key: 'item_type_label', label: '结算项目', width: 150 },
  { key: 'payee_user_id', label: '人员', width: 96 },
  { key: 'quantity', label: '数量', width: 88, align: 'right' },
  { key: 'direction_label', label: '方向', width: 88 },
  { key: 'amount', label: '金额', width: 112, align: 'right' },
])
const permissionGridColumns = computed<GridColumn[]>(() => [
  { key: 'payee_user_id', label: '人员', width: 96 },
  { key: 'business_month', label: '业务月', width: 108 },
  { key: 'status_label', label: '状态', width: 96 },
  { key: 'reason_label', label: '备注', width: 180 },
])
const supplementGridColumns = computed<GridColumn[]>(() => [
  { key: 'payee_user_id', label: '人员', width: 96 },
  { key: 'order_no', label: '订单号', width: 150 },
  { key: 'status_label', label: '状态', width: 96 },
  { key: 'duplicate_label', label: '查重', width: 108 },
  { key: 'gross_amount', label: '补录金额', width: 112, align: 'right' },
])

function payrollRowLabel(row: SettlementPayrollRow) {
  return row.row_type === 'supplement_piecework' ? '补录计件工资' : '正常计件工资'
}

function batchStatusLabel(status: string) {
  return batchStatusLabels[status] ?? status
}

function settlementItemTypeLabel(type: string) {
  return settlementItemTypeLabels[type] ?? type
}

function supplementStatusLabel(status: string) {
  return supplementStatusLabels[status] ?? status
}

function directionLabel(direction: string) {
  const labels: Record<string, string> = {
    credit: '增加',
    debit: '扣回',
  }
  return labels[direction] ?? direction
}

function toPayrollGridRow(row: SettlementPayrollRow): PayrollGridRow {
  return {
    ...row,
    grid_id: `${row.payee_user_id}-${row.row_type}`,
    row_label: payrollRowLabel(row),
  }
}

function toBatchGridRow(row: SettlementBatchRow): BatchGridRow {
  return {
    ...row,
    status_label: batchStatusLabel(row.status),
  }
}

function toBatchItemGridRow(row: SettlementBatchDetail['items'][number]): BatchItemGridRow {
  return {
    id: row.id,
    item_type_label: settlementItemTypeLabel(row.item_type),
    payee_user_id: row.payee_user_id,
    quantity: row.quantity,
    direction_label: directionLabel(row.direction),
    amount: row.amount,
  }
}

function toPermissionGridRow(row: SupplementPermissionRow): PermissionGridRow {
  return {
    ...row,
    status_label: row.enabled ? '已开放' : '已关闭',
    reason_label: row.reason || '无备注',
  }
}

function gridRowAsBatch(row: Record<string, unknown>): SettlementBatchRow {
  return row as unknown as SettlementBatchRow
}

async function loadSettlement(options: { keepNotice?: boolean } = {}) {
  loading.value = true
  error.value = ''
  if (!options.keepNotice) notice.value = ''
  try {
    const [previewResult, batchResult, supplementResult, permissionResult] = await Promise.all([
      assetWorkbenchApi.previewSettlement(month.value),
      assetWorkbenchApi.listSettlementBatches({ business_month: month.value, page: 1, page_size: 20 }),
      assetWorkbenchApi.listSettlementSupplements({ business_month: month.value, page: 1, page_size: 20 }),
      assetWorkbenchApi.listSupplementPermissions({ business_month: month.value, page: 1, page_size: 50 }),
    ])
    preview.value = previewResult
    batches.value = batchResult.items
    supplements.value = supplementResult.items
    supplementPermissions.value = permissionResult.items
    selectedBatch.value = null
  } catch (err) {
    error.value = err instanceof Error ? err.message : '结算数据加载失败'
  } finally {
    loading.value = false
  }
}

async function generateBatch() {
  error.value = ''
  notice.value = ''
  try {
    const batch = await assetWorkbenchApi.generateSettlementBatch(month.value)
    notice.value = `已生成批次：${batch.batch_no}`
    await loadSettlement({ keepNotice: true })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '生成批次失败'
  }
}

async function confirmBatch(batch: SettlementBatchRow) {
  error.value = ''
  notice.value = ''
  try {
    await assetWorkbenchApi.confirmSettlementBatch(batch.id)
    notice.value = `已确认批次：${batch.batch_no}`
    await loadSettlement({ keepNotice: true })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '确认批次失败'
  }
}

function startCancelBatch(batch: SettlementBatchRow) {
  pendingCancelBatch.value = batch
  cancelReason.value = ''
}

async function cancelBatch() {
  const batch = pendingCancelBatch.value
  if (!batch) return
  const reason = cancelReason.value.trim()
  if (!reason) {
    error.value = '请填写取消原因'
    return
  }
  error.value = ''
  notice.value = ''
  try {
    await assetWorkbenchApi.cancelSettlementBatch(batch.id, reason)
    notice.value = `已取消批次：${batch.batch_no}`
    pendingCancelBatch.value = null
    cancelReason.value = ''
    await loadSettlement({ keepNotice: true })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '取消批次失败'
  }
}

async function openBatch(batch: SettlementBatchRow) {
  error.value = ''
  try {
    selectedBatch.value = await assetWorkbenchApi.getSettlementBatchDetail(batch.id)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '批次明细加载失败'
  }
}

function openErrorImport() {
  errorInputRef.value?.click()
}

async function handleErrorImport(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  target.value = ''
  if (!file) return
  error.value = ''
  notice.value = ''
  try {
    const batch = await assetWorkbenchApi.importErrorExcel(month.value, file)
    notice.value = `出错 Excel 已导入：匹配 ${batch.matched_rows} 行，未匹配 ${batch.unmatched_rows} 行，多匹配 ${batch.ambiguous_rows} 行`
    await loadSettlement({ keepNotice: true })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '出错 Excel 导入失败'
  }
}

async function upsertSupplementPermission() {
  error.value = ''
  notice.value = ''
  try {
    const saved = await assetWorkbenchApi.upsertSupplementPermission({
      payee_user_id: permissionForm.value.payee_user_id,
      business_month: month.value,
      enabled: permissionForm.value.enabled,
      reason: permissionForm.value.reason,
    })
    notice.value = saved.enabled ? `已开放 ${saved.payee_user_id} 的 ${saved.business_month} 补录` : `已关闭 ${saved.payee_user_id} 的 ${saved.business_month} 补录`
    permissionForm.value.reason = ''
    await loadSettlement({ keepNotice: true })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '更新补录权限失败'
  }
}

async function loadSupplementEligibleMonths() {
  if (!permissionForm.value.payee_user_id) {
    error.value = '请先填写开放人员 ID'
    return
  }
  error.value = ''
  notice.value = ''
  eligibleMonthsLoading.value = true
  try {
    eligibleSupplementMonths.value = await assetWorkbenchApi.listSupplementEligibleMonths(permissionForm.value.payee_user_id)
    if (eligibleSupplementMonths.value.length && !eligibleSupplementMonths.value.includes(month.value)) {
      month.value = eligibleSupplementMonths.value[0]
    }
    notice.value = eligibleSupplementMonths.value.length ? '已读取可补录月份' : '该人员暂无已确认结算月份'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '读取可补录月份失败'
  } finally {
    eligibleMonthsLoading.value = false
  }
}

async function createSupplement() {
  error.value = ''
  notice.value = ''
  try {
    const payload = {
      ...supplementForm.value,
      business_month: month.value,
      status: 'approved',
    }
    await assetWorkbenchApi.createSettlementSupplement(payload)
    notice.value = '月度补录已创建'
    supplementForm.value.order_no = ''
    supplementForm.value.page_count = 1
    supplementForm.value.gross_amount = 0
    await loadSettlement({ keepNotice: true })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '创建补录失败'
  }
}

async function createAdjustment() {
  const batch = selectedBatch.value?.batch
  if (!batch) return
  error.value = ''
  notice.value = ''
  try {
    const created = await assetWorkbenchApi.createSettlementAdjustment(batch.id, {
      ...adjustmentForm.value,
      payload_json: {
        created_from: 'settlement_page',
        batch_no: batch.batch_no,
      },
    })
    notice.value = `已追加${created.adjustment_type === 'reversal' ? '冲正' : '补差'}：${created.amount}`
    adjustmentForm.value.amount = 0
    adjustmentForm.value.reason = ''
    await loadSettlement({ keepNotice: true })
    selectedBatch.value = await assetWorkbenchApi.getSettlementBatchDetail(batch.id)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '创建冲正/补差失败'
  }
}

async function exportSettlement() {
  if (!preview.value && !selectedBatch.value) return
  error.value = ''
  notice.value = ''
  exporting.value = true
  try {
    await exportSettlementWorkbook({
      businessMonth: selectedBatch.value?.batch.business_month ?? month.value,
      preview: preview.value,
      batchDetail: selectedBatch.value,
    })
    notice.value = selectedBatch.value ? '已导出批次工资条与明细' : '已导出预览工资条'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '导出结算工资条失败'
  } finally {
    exporting.value = false
  }
}

onMounted(() => {
  void loadSettlement()
})
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-heading">
      <div>
        <p class="aw-eyebrow">工资结算</p>
        <h2>结算统计</h2>
      </div>
      <div class="aw-button-row">
        <button class="aw-secondary-button" type="button" @click="openErrorImport">导入出错</button>
        <button class="aw-secondary-button" type="button" @click="() => loadSettlement()">生成预览</button>
        <button class="aw-secondary-button" type="button" :disabled="exporting || (!preview && !selectedBatch)" @click="exportSettlement">
          导出工资条
        </button>
        <button class="aw-primary-button" type="button" @click="generateBatch">生成批次</button>
      </div>
    </div>
    <input ref="errorInputRef" class="aw-visually-hidden" type="file" accept=".xlsx,.xls" @change="handleErrorImport" />
    <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
    <div class="aw-panel">
      <h3>本月工资预览</h3>
      <p class="aw-copy">
        先导入出错表，再生成预览。每个员工固定展示两条工资行：正常计件工资一条，补录计件工资一条；没有补录时补录金额为 0。
      </p>
      <p v-if="loading" class="aw-copy">正在加载 {{ month }} 结算预览</p>
      <p v-else-if="error" class="aw-copy">{{ error }}</p>
      <WorkbenchDataGrid
        v-else-if="totals"
        :columns="metricGridColumns"
        :rows="metricGridRows"
        row-key="id"
        storage-key="settlement-metrics"
        :height="170"
        :row-height="34"
      >
        <template #cell="{ column, value }">
          <strong v-if="column.key === 'amount'">{{ value }}</strong>
          <span v-else>{{ value }}</span>
        </template>
      </WorkbenchDataGrid>
      <WorkbenchDataGrid
        v-if="payrollRows.length"
        :columns="payrollGridColumns"
        :rows="payrollGridRows"
        row-key="grid_id"
        storage-key="settlement-preview-payroll"
        group-by="payee_user_id"
        :height="260"
        :row-height="34"
      >
        <template #cell="{ column, value }">
          <strong v-if="column.key === 'net_amount'">{{ value }}</strong>
          <span v-else>{{ value }}</span>
        </template>
      </WorkbenchDataGrid>
    </div>

    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <input v-model="month" type="month" />
        <button type="button" @click="() => loadSettlement()">刷新</button>
      </div>
      <WorkbenchDataGrid
        v-if="batches.length"
        :columns="batchGridColumns"
        :rows="batchGridRows"
        row-key="id"
        storage-key="settlement-batches"
        group-by="status_label"
        :height="260"
        :row-height="36"
      >
        <template #cell="{ row, column, value }">
          <div v-if="column.key === 'actions'" class="aw-inline-actions">
          <button type="button" @click="openBatch(gridRowAsBatch(row))">明细</button>
          <button v-if="gridRowAsBatch(row).status === 'generated'" type="button" @click="confirmBatch(gridRowAsBatch(row))">
            确认
            </button>
            <button v-if="gridRowAsBatch(row).status === 'generated'" type="button" @click="startCancelBatch(gridRowAsBatch(row))">
              取消
            </button>
          </div>
          <strong v-else-if="column.key === 'net_amount'">{{ value }}</strong>
          <span v-else>{{ value }}</span>
        </template>
      </WorkbenchDataGrid>
      <div v-if="pendingCancelBatch" class="aw-panel">
        <div class="aw-grid-toolbar">
          <span>取消结算批次</span>
          <span>{{ pendingCancelBatch.batch_no }}</span>
        </div>
        <label class="aw-field">
          <span>取消原因</span>
          <input v-model="cancelReason" required />
        </label>
        <div class="aw-inline-actions">
          <button class="aw-primary-button" type="button" @click="cancelBatch">确认取消</button>
          <button class="aw-secondary-button" type="button" @click="pendingCancelBatch = null">返回</button>
        </div>
      </div>
      <div v-if="!batches.length" class="aw-empty-state">
        <h3>还没有结算批次</h3>
        <p>生成批次会锁定本月可结算明细；未确认前可以取消后重新生成，确认后只能做冲正或补差。</p>
      </div>
    </div>

    <div v-if="selectedBatch" class="aw-detail-panel">
      <div class="aw-detail-panel__head">
        <div>
          <p class="aw-eyebrow">批次明细</p>
          <h3>{{ selectedBatch.batch.batch_no }}</h3>
        </div>
        <strong class="aw-money">{{ selectedBatch.batch.net_amount }}</strong>
      </div>
      <WorkbenchDataGrid
        :columns="batchItemGridColumns"
        :rows="batchItemGridRows"
        row-key="id"
        storage-key="settlement-batch-items"
        group-by="item_type_label"
        :height="260"
        :row-height="34"
      >
        <template #cell="{ column, value }">
          <strong v-if="column.key === 'amount'">{{ value }}</strong>
          <span v-else>{{ value }}</span>
        </template>
      </WorkbenchDataGrid>
      <WorkbenchDataGrid
        v-if="selectedBatch.payroll_rows.length"
        :columns="payrollGridColumns"
        :rows="batchPayrollGridRows"
        row-key="grid_id"
        storage-key="settlement-batch-payroll"
        group-by="payee_user_id"
        :height="260"
        :row-height="34"
      >
        <template #cell="{ column, value }">
          <strong v-if="column.key === 'net_amount'">{{ value }}</strong>
          <span v-else>{{ value }}</span>
        </template>
      </WorkbenchDataGrid>
      <div v-if="selectedBatch.batch.status === 'confirmed'" class="aw-panel">
        <h3>冲正 / 补差</h3>
        <p class="aw-copy">已确认批次不直接改原始明细。需要补发或扣回时，在这里追加调整记录并保留原因。</p>
        <div class="aw-form-grid">
          <label>
            人员 ID
            <input v-model.number="adjustmentForm.payee_user_id" type="number" min="1" />
          </label>
          <label>
            类型
            <select v-model="adjustmentForm.adjustment_type">
              <option value="adjustment">补差</option>
              <option value="reversal">冲正</option>
            </select>
          </label>
          <label>
            方向
            <select v-model="adjustmentForm.direction">
              <option value="credit">增加</option>
              <option value="debit">扣回</option>
            </select>
          </label>
          <label>
            金额
            <input v-model.number="adjustmentForm.amount" type="number" min="0" />
          </label>
          <label>
            原因
            <input v-model="adjustmentForm.reason" />
          </label>
        </div>
        <button class="aw-secondary-button" type="button" @click="createAdjustment">追加调整</button>
      </div>
      <p v-else class="aw-copy">批次确认后才能追加冲正或补差。</p>
    </div>

    <div class="aw-panel">
      <h3>月度补录</h3>
      <p class="aw-copy">补录按人员 + 业务月手动开放。已批准补录会在工资条中单独形成补录计件工资行，没有补录也展示 0。</p>
      <div class="aw-form-grid">
        <label>
          开放人员 ID
          <input v-model.number="permissionForm.payee_user_id" type="number" min="1" />
        </label>
        <label>
          可补录月份
          <select v-model="month" :disabled="eligibleSupplementMonths.length === 0">
            <option v-if="eligibleSupplementMonths.length === 0" :value="month">{{ month }}</option>
            <option v-for="item in eligibleSupplementMonths" :key="item" :value="item">{{ item }}</option>
          </select>
        </label>
        <label>
          开关
          <select v-model="permissionForm.enabled">
            <option :value="true">开放</option>
            <option :value="false">关闭</option>
          </select>
        </label>
        <label>
          原因
          <input v-model="permissionForm.reason" />
        </label>
      </div>
      <button class="aw-secondary-button" type="button" :disabled="eligibleMonthsLoading" @click="loadSupplementEligibleMonths">
        读取可补录月份
      </button>
      <button class="aw-secondary-button" type="button" @click="upsertSupplementPermission">更新补录权限</button>
      <WorkbenchDataGrid
        v-if="supplementPermissions.length"
        :columns="permissionGridColumns"
        :rows="permissionGridRows"
        row-key="id"
        storage-key="settlement-supplement-permissions"
        group-by="status_label"
        :height="220"
        :row-height="34"
      >
        <template #cell="{ column, value }">
          <strong v-if="column.key === 'reason_label'">{{ value }}</strong>
          <span v-else>{{ value }}</span>
        </template>
      </WorkbenchDataGrid>
      <p v-else class="aw-copy">当前月份还没有开放补录权限</p>
      <div class="aw-form-grid">
        <label>
          人员 ID
          <input v-model.number="supplementForm.payee_user_id" type="number" min="1" />
        </label>
        <label>
          订单号
          <input v-model="supplementForm.order_no" />
        </label>
        <label>
          难度
          <select v-model="supplementForm.difficulty_class">
            <option value="A">A</option>
            <option value="B">B</option>
            <option value="C">C</option>
            <option value="A+小夜灯">A+小夜灯</option>
          </select>
        </label>
        <label>
          页数
          <input v-model.number="supplementForm.page_count" min="1" type="number" />
        </label>
        <label>
          补录金额
          <input v-model.number="supplementForm.gross_amount" min="0" type="number" />
        </label>
        <label class="aw-inline-check">
          <input v-model="supplementForm.finalized" type="checkbox" />
          定稿
        </label>
      </div>
      <button class="aw-secondary-button" type="button" @click="createSupplement">创建补录</button>
      <WorkbenchDataGrid
        v-if="supplements.length"
        :columns="supplementGridColumns"
        :rows="supplementGridRows"
        row-key="id"
        storage-key="settlement-supplements"
        group-by="status_label"
        :height="220"
        :row-height="34"
      >
        <template #cell="{ column, value }">
          <strong v-if="column.key === 'gross_amount'">{{ value }}</strong>
          <span v-else>{{ value }}</span>
        </template>
      </WorkbenchDataGrid>
      <p v-else class="aw-copy">当前月份没有补录记录</p>
    </div>
  </section>
</template>
