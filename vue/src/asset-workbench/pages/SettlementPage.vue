<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Download, Table2 } from 'lucide-vue-next'

import {
  assetWorkbenchApi,
  type DifficultyClassRow,
  type SettlementBatchDetail,
  type SettlementBatchRow,
  type SettlementPayrollRow,
  type SettlementPreview,
  type SettlementSupplementRow,
  type SupplementPermissionRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { exportErrorImportTemplateWorkbook, exportSettlementWorkbook, exportSupplementImportTemplateWorkbook } from '@aw/features/export/settlementExport'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import LedgerReadout from '@aw/shared/console/LedgerReadout.vue'
import SettlementHubTabs from '@aw/shared/console/SettlementHubTabs.vue'
import { currentBusinessMonth } from '@aw/shared/format/businessMonth'
import { difficultyCodes } from '@aw/shared/format/difficulty'
import { formatInt, formatMoney } from '@aw/shared/format/number'
import {
  batchStatusMeta,
  chipClass,
  directionMeta,
  duplicateMeta,
  enabledMeta,
  itemTypeMeta,
  supplementStatusMeta,
  workerTypeMeta,
} from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import SpreadsheetWorkbench from '@aw/shared/spreadsheet/SpreadsheetWorkbench.vue'
import { buildImportReviewSource, workbookReviewRowsToFiles } from '@aw/shared/spreadsheet/excelReview'
import type {
  WorkbenchSpreadsheetActionPayload,
  WorkbenchSpreadsheetColumn,
  WorkbenchSpreadsheetSource,
  WorkbenchSpreadsheetValidation,
} from '@aw/shared/spreadsheet/types'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'
import { resolveApiUserMessage } from '@/utils/api-message-zh'

interface GridColumn {
  key: string
  label: string
  width: number
  align?: 'left' | 'right' | 'center'
}

type PayrollGridRow = SettlementPayrollRow & { grid_id: string; row_label: string; payee_name_label: string; worker_type_label: string }
type BatchGridRow = SettlementBatchRow & { status_label: string }
type BatchItemGridRow = {
  id: number
  item_type: string
  item_type_label: string
  payee_user_id: number
  quantity: number
  direction: string
  direction_label: string
  amount: number
}
type PermissionGridRow = SupplementPermissionRow & { status_label: string; reason_label: string }
type SupplementGridRow = SettlementSupplementRow & { status_label: string; duplicate_label: string; supplement_date_label: string; action: string }
type SettlementSectionKey = 'preview' | 'batches' | 'supplements' | 'adjustments'

const month = ref(defaultBusinessMonth())
const preview = ref<SettlementPreview | null>(null)
const batches = ref<SettlementBatchRow[]>([])
const supplements = ref<SettlementSupplementRow[]>([])
const supplementPermissions = ref<SupplementPermissionRow[]>([])
const difficultyRows = ref<DifficultyClassRow[]>([])
const eligibleSupplementMonths = ref<string[]>([])
const entryEligibleSupplementMonths = ref<string[]>([])
const entryEligiblePayeeUserId = ref(0)
const supplementMonth = ref(month.value)
const selectedBatch = ref<SettlementBatchDetail | null>(null)
const pendingCancelBatch = ref<SettlementBatchRow | null>(null)
const pendingDeleteSupplement = ref<SettlementSupplementRow | null>(null)
const cancelReason = ref('')
const supplementDeleteReason = ref('')
const exporting = ref(false)
const generatingBatch = ref(false)
const confirmingBatchId = ref<number | null>(null)
const eligibleMonthsLoading = ref(false)
const entryEligibleMonthsLoading = ref(false)
const activeSettlementSection = ref<SettlementSectionKey>('preview')
const notice = ref('')
const errorInputRef = ref<HTMLInputElement | null>(null)
const supplementInputRef = ref<HTMLInputElement | null>(null)
const settlementSpreadsheetOpen = ref(false)
const importReviewSource = ref<WorkbenchSpreadsheetSource | null>(null)
const importReviewLoading = ref(false)
const pendingImportKind = ref<'error-deduction' | 'supplement' | ''>('')
const importReviewRevision = ref(0)
const supplementPage = ref(1)
const supplementPageSize = ref(20)
const supplementTotal = ref(0)
const supplementSearch = ref('')
const supplementPayeeFilter = ref('')
const supplementStatusFilter = ref('')
const supplementDateFilter = ref('')
const supplementSortBy = ref('supplement_date')
const supplementSortDir = ref<'asc' | 'desc'>('desc')
const settlementRequest = usePageRequest(
  async () => {
    const [previewResult, batchResult, supplementResult, permissionResult, difficultyResult] = await Promise.all([
      assetWorkbenchApi.previewSettlement(month.value),
      assetWorkbenchApi.listSettlementBatches({ business_month: month.value, page: 1, page_size: 20 }),
      assetWorkbenchApi.listSettlementSupplements(buildSupplementListParams()),
      assetWorkbenchApi.listSupplementPermissions({ business_month: month.value, page: 1, page_size: 50 }),
      assetWorkbenchApi.listDifficultyClasses(),
    ])
    return {
      preview: previewResult,
      batches: batchResult.items,
      supplements: supplementResult.items,
      supplementTotal: supplementResult.total,
      supplementPermissions: permissionResult.items,
      difficulties: difficultyResult,
    }
  },
  null,
  '结算数据加载失败',
)
const loading = settlementRequest.loading
const error = settlementRequest.error
const supplementForm = ref({
  payee_user_id: 0,
  order_no: '',
  supplement_date: todayDate(),
  difficulty_class: '',
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
const adjustmentMode = computed({
  get: () => (adjustmentForm.value.direction === 'debit' ? 'debit' : 'credit'),
  set: (value: 'credit' | 'debit') => {
    adjustmentForm.value.direction = value
    adjustmentForm.value.adjustment_type = value === 'debit' ? 'reversal' : 'adjustment'
  },
})
const moneyColumns = new Set(['gross_amount', 'deduction_amount', 'welfare_amount', 'supplement_amount', 'adjustment_amount', 'net_amount', 'amount'])
const intColumns = new Set(['item_count', 'page_count', 'error_count', 'quantity'])
const difficultyOptions = computed(() => difficultyCodes(difficultyRows.value))

const totals = computed(() => preview.value?.totals)
const payrollRows = computed(() => preview.value?.payroll_rows ?? [])
const ledgerSegments = computed(() => {
  const value = totals.value
  if (!value) {
    return [
      { key: 'gross', label: '日常计件', value: formatMoney(0), hint: '待生成预览', expandable: true },
      { key: 'deduction', label: '质检扣款', value: formatMoney(0), hint: '结算时读取质检错误表', expandable: true },
      { key: 'supplement', label: '补录计件', value: formatMoney(0), hint: '工资条第二行', expandable: true },
      { key: 'net', label: '应结净额', value: formatMoney(0), hint: '计件 + 补录 - 质检扣款', money: true, expandable: true },
    ]
  }
  return [
    { key: 'gross', label: '日常计件', value: formatMoney(value.gross_amount), hint: `${formatInt(value.item_count)} 单`, expandable: true },
    { key: 'deduction', label: '质检扣款', value: formatMoney(value.deduction_amount), hint: `${formatInt(value.error_count)} 个出错`, expandable: true },
    { key: 'supplement', label: '补录计件', value: formatMoney(value.supplement_amount), hint: '单独工资行', expandable: true },
    { key: 'net', label: '应结净额', value: formatMoney(value.net_amount), hint: '正常工资行 + 补录工资行', money: true, expandable: true },
  ]
})
const payrollGridRows = computed(() => payrollRows.value.map(toPayrollGridRow) as unknown as Record<string, unknown>[])
const batchGridRows = computed(() => batches.value.map(toBatchGridRow) as unknown as Record<string, unknown>[])
const batchItemGridRows = computed(() => (selectedBatch.value?.items ?? []).map(toBatchItemGridRow) as unknown as Record<string, unknown>[])
const batchPayrollGridRows = computed(() => (selectedBatch.value?.payroll_rows ?? []).map(toPayrollGridRow) as unknown as Record<string, unknown>[])
const permissionGridRows = computed(() => supplementPermissions.value.map(toPermissionGridRow) as unknown as Record<string, unknown>[])
const supplementRowsWithLabels = computed<SupplementGridRow[]>(() =>
  supplements.value.map((row) => ({
    ...row,
    status_label: supplementStatusMeta(row.status).label,
    duplicate_label: duplicateMeta(row.duplicate_hint_json?.has_duplicates).label,
    supplement_date_label: supplementDateOf(row) || '未填写',
    action: 'actions',
  })),
)
const filteredSupplementRows = computed(() => supplementRowsWithLabels.value)
const supplementGridRows = computed(() => filteredSupplementRows.value as unknown as Record<string, unknown>[])
const supplementCanPrev = computed(() => supplementPage.value > 1)
const supplementCanNext = computed(() => supplementPage.value * supplementPageSize.value < supplementTotal.value)
const manualSupplementDuplicateWarning = computed(() => {
  const name = supplementForm.value.order_no.trim().toLowerCase()
  const payeeID = Number(supplementForm.value.payee_user_id || 0)
  if (!name || !payeeID) return ''
  const matches = supplements.value.filter((row) =>
    row.payee_user_id === payeeID &&
    row.business_month === supplementMonth.value &&
    row.status !== 'voided' &&
    row.order_no.trim().toLowerCase() === name,
  )
  if (!matches.length) return ''
  return `已有 ${formatInt(matches.length)} 条同名补录记录，请确认是否重复上传。`
})
const entryEligibleReady = computed(() =>
  entryEligiblePayeeUserId.value === supplementForm.value.payee_user_id && entryEligibleSupplementMonths.value.length > 0,
)
const selectedBatchConfirmed = computed(() => selectedBatch.value?.batch.status === 'confirmed')
const hasGeneratedBatch = computed(() => batches.value.some((batch) => batch.status === 'generated'))
const settlementNavGroups = computed(() => [
  {
    label: '结算流程',
    items: [
      {
        key: 'preview' as const,
        title: '预览工资',
        meta: '先核对本月计件、扣款、补录和净额',
        count: payrollRows.value.length ? `${formatInt(payrollRows.value.length)} 行` : '待预览',
      },
      {
        key: 'batches' as const,
        title: '批次确认',
        meta: '生成批次后锁定本月可结算明细',
        count: `${formatInt(batches.value.length)} 个`,
      },
    ],
  },
  {
    label: '补充处理',
    items: [
      {
        key: 'supplements' as const,
        title: '补录管理',
        meta: '开放补录月份、导入或手动补发',
        count: `${formatInt(supplements.value.length)} 条`,
      },
      {
        key: 'adjustments' as const,
        title: '工资调整',
        meta: '批次确认后，记录需要补发或扣回的金额',
        count: selectedBatchConfirmed.value ? '可记录' : '待选批次',
      },
    ],
  },
])
const payrollGridColumns = computed<GridColumn[]>(() => [
  { key: 'payee_name_label', label: '姓名', width: 116 },
  { key: 'worker_type_label', label: '人员类型', width: 96 },
  { key: 'payee_user_id', label: '人员编号', width: 96 },
  { key: 'row_label', label: '工资条', width: 140 },
  { key: 'item_count', label: '单数', width: 88, align: 'right' },
  { key: 'gross_amount', label: '毛额', width: 100, align: 'right' },
  { key: 'deduction_amount', label: '质检扣款', width: 112, align: 'right' },
  { key: 'welfare_amount', label: '福利', width: 100, align: 'right' },
  { key: 'supplement_amount', label: '补录', width: 100, align: 'right' },
  { key: 'adjustment_amount', label: '调整', width: 100, align: 'right' },
  { key: 'net_amount', label: '净额', width: 112, align: 'right' },
])
const batchGridColumns = computed<GridColumn[]>(() => [
  { key: 'batch_no', label: '批次号', width: 190 },
  { key: 'business_month', label: '结算月', width: 108 },
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
  { key: 'business_month', label: '结算月', width: 108 },
  { key: 'status_label', label: '状态', width: 96 },
  { key: 'reason_label', label: '备注', width: 180 },
])
const supplementGridColumns = computed<GridColumn[]>(() => [
  { key: 'payee_user_id', label: '人员', width: 96 },
  { key: 'order_no', label: '文件/作品名称', width: 170 },
  { key: 'supplement_date_label', label: '补录日期', width: 110 },
  { key: 'status_label', label: '状态', width: 96 },
  { key: 'duplicate_label', label: '查重', width: 108 },
  { key: 'gross_amount', label: '补录金额', width: 112, align: 'right' },
  { key: 'action', label: '动作', width: 120, align: 'center' },
])
const settlementSpreadsheetSource = computed<WorkbenchSpreadsheetSource>(() => ({
  id: 'asset-settlement-spreadsheet',
  revision: `${month.value}:${payrollRows.value.length}:${batches.value.length}:${supplements.value.length}:${supplementPermissions.value.length}`,
  mode: 'settlement',
  title: `${month.value} 结算表格工作台`,
  description: '用于集中核对工资条、批次和补录记录。这里不替代批次确认、删除、调整等原页面动作。',
  readonly: true,
  actions: [
    { key: 'refresh_settlement', label: '刷新预览', tone: 'neutral' },
    { key: 'export_settlement', label: '导出工资条', tone: 'money', disabled: exporting.value || (!preview.value && !selectedBatch.value) },
    { key: 'open_error_import', label: '导入质检扣款', tone: 'neutral' },
    { key: 'open_supplement_import', label: '导入补录', tone: 'neutral' },
  ],
  sheets: [
    {
      id: 'payroll',
      name: '工资条',
      rowKey: 'grid_id',
      readonly: true,
      freezeHeader: true,
      columns: toSpreadsheetColumns(payrollGridColumns.value),
      rows: payrollGridRows.value,
    },
    {
      id: 'batches',
      name: '批次',
      rowKey: 'id',
      readonly: true,
      freezeHeader: true,
      columns: toSpreadsheetColumns(batchGridColumns.value.filter((column) => column.key !== 'actions')),
      rows: batchGridRows.value,
    },
    {
      id: 'supplements',
      name: '补录',
      rowKey: 'id',
      readonly: true,
      freezeHeader: true,
      columns: toSpreadsheetColumns(supplementGridColumns.value.filter((column) => column.key !== 'action')),
      rows: supplementGridRows.value,
      validations: supplementSpreadsheetValidations.value,
    },
  ],
}))
const supplementSpreadsheetValidations = computed<WorkbenchSpreadsheetValidation[]>(() =>
  supplementRowsWithLabels.value
    .filter((row) => row.duplicate_hint_json?.has_duplicates)
    .map((row) => ({
      rowKey: row.id,
      columnKey: 'duplicate_label',
      tone: 'warn',
      message: `${row.order_no} 存在重复提示，导入或确认前需要复核。`,
    })),
)

function payrollRowLabel(row: SettlementPayrollRow) {
  return row.row_type === 'supplement_piecework' ? '补录计件工资' : '日常计件工资'
}

function toPayrollGridRow(row: SettlementPayrollRow): PayrollGridRow {
  return {
    ...row,
    grid_id: `${row.payee_user_id}-${row.row_type}`,
    row_label: payrollRowLabel(row),
    payee_name_label: row.payee_name || `用户 ${row.payee_user_id}`,
    worker_type_label: workerTypeMeta(row.worker_type || '').label,
  }
}

function toBatchGridRow(row: SettlementBatchRow): BatchGridRow {
  return {
    ...row,
    status_label: batchStatusMeta(row.status).label,
  }
}

function toSpreadsheetColumns(columns: GridColumn[]): WorkbenchSpreadsheetColumn[] {
  return columns.map((column) => ({
    key: column.key,
    label: column.label,
    width: column.width,
    align: column.align,
    readonly: true,
    kind: isMoneyColumn(column.key) ? 'money' : intColumns.has(column.key) ? 'number' : column.key.includes('status') ? 'status' : 'text',
  }))
}

function toBatchItemGridRow(row: SettlementBatchDetail['items'][number]): BatchItemGridRow {
  return {
    id: row.id,
    item_type: row.item_type,
    item_type_label: itemTypeMeta(row.item_type).label,
    payee_user_id: row.payee_user_id,
    quantity: row.quantity,
    direction: row.direction,
    direction_label: directionMeta(row.direction).label,
    amount: row.amount,
  }
}

function toPermissionGridRow(row: SupplementPermissionRow): PermissionGridRow {
  return {
    ...row,
    status_label: enabledMeta(row.enabled).label,
    reason_label: row.reason || '无备注',
  }
}

function gridRowAsBatch(row: Record<string, unknown>): SettlementBatchRow {
  return row as unknown as SettlementBatchRow
}

function gridRowAsPermission(row: Record<string, unknown>): PermissionGridRow {
  return row as unknown as PermissionGridRow
}

function gridRowAsSupplement(row: Record<string, unknown>): SupplementGridRow {
  return row as unknown as SupplementGridRow
}

function gridRowAsBatchItem(row: Record<string, unknown>): BatchItemGridRow {
  return row as unknown as BatchItemGridRow
}

function isMoneyColumn(key: string) {
  return moneyColumns.has(key)
}

function gridValue(key: string, value: unknown) {
  if (moneyColumns.has(key)) return formatMoney(value)
  if (intColumns.has(key)) return formatInt(value)
  return value || '—'
}

function defaultBusinessMonth() {
  return currentBusinessMonth()
}

function todayDate() {
  const value = new Date()
  return `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, '0')}-${String(value.getDate()).padStart(2, '0')}`
}

function businessMonthFromDate(value: string) {
  return /^\d{4}-\d{2}-\d{2}$/.test(value) ? value.slice(0, 7) : ''
}

function supplementDateOf(row: SettlementSupplementRow) {
  return String(row.supplement_date || row.duplicate_hint_json?.supplement_date || '')
}

function settlementActionError(err: unknown, fallback: string) {
  return resolveApiUserMessage(err, { fallback })
}

function buildSupplementListParams() {
  const params: Record<string, unknown> = {
    business_month: month.value,
    page: supplementPage.value,
    page_size: supplementPageSize.value,
    sort_by: supplementSortBy.value,
    sort_dir: supplementSortDir.value,
  }
  const name = supplementSearch.value.trim()
  if (name) params.order_no = name
  const payeeID = Number(supplementPayeeFilter.value)
  if (Number.isInteger(payeeID) && payeeID > 0) params.payee_user_id = payeeID
  if (supplementStatusFilter.value) params.status = supplementStatusFilter.value
  if (supplementDateFilter.value) params.supplement_date = supplementDateFilter.value
  return params
}

async function querySupplements() {
  supplementPage.value = 1
  await loadSettlement({ keepNotice: true })
}

async function resetSupplementQuery() {
  supplementSearch.value = ''
  supplementPayeeFilter.value = ''
  supplementStatusFilter.value = ''
  supplementDateFilter.value = ''
  supplementSortBy.value = 'supplement_date'
  supplementSortDir.value = 'desc'
  supplementPage.value = 1
  supplementPageSize.value = 20
  await loadSettlement({ keepNotice: true })
}

async function changeSupplementPage(delta: number) {
  const nextPage = Math.max(1, supplementPage.value + delta)
  if (nextPage === supplementPage.value) return
  supplementPage.value = nextPage
  await loadSettlement({ keepNotice: true })
}

async function loadSettlement(options: { keepNotice?: boolean } = {}) {
  error.value = ''
  if (!options.keepNotice) notice.value = ''
  const data = await settlementRequest.run()
  if (!data) return
  preview.value = data.preview
  batches.value = data.batches
  supplements.value = data.supplements
  supplementTotal.value = data.supplementTotal
  supplementPermissions.value = data.supplementPermissions
  difficultyRows.value = data.difficulties
  if (!difficultyOptions.value.includes(supplementForm.value.difficulty_class)) {
    supplementForm.value.difficulty_class = difficultyOptions.value[0] || ''
  }
  selectedBatch.value = null
}

async function generateBatch() {
  if (generatingBatch.value) return
  error.value = ''
  notice.value = ''
  generatingBatch.value = true
  try {
    const batch = await assetWorkbenchApi.generateSettlementBatch(month.value)
    notice.value = `已生成批次：${batch.batch_no}`
    activeSettlementSection.value = 'batches'
    await loadSettlement({ keepNotice: true })
  } catch (err) {
    error.value = settlementActionError(err, '生成批次失败，请确认本月是否还有待结算明细。')
  } finally {
    generatingBatch.value = false
  }
}

async function confirmBatch(batch: SettlementBatchRow) {
  if (confirmingBatchId.value) return
  error.value = ''
  notice.value = ''
  confirmingBatchId.value = batch.id
  try {
    await assetWorkbenchApi.confirmSettlementBatch(batch.id)
    notice.value = `已确认批次：${batch.batch_no}`
    await loadSettlement({ keepNotice: true })
  } catch (err) {
    error.value = settlementActionError(err, '确认批次失败，请检查人员工资资料是否完整。')
  } finally {
    confirmingBatchId.value = null
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
    error.value = settlementActionError(err, '取消批次失败')
  }
}

async function openBatch(batch: SettlementBatchRow) {
  error.value = ''
  try {
    selectedBatch.value = await assetWorkbenchApi.getSettlementBatchDetail(batch.id)
    activeSettlementSection.value = 'batches'
  } catch (err) {
    error.value = settlementActionError(err, '批次明细加载失败')
  }
}

function openErrorImport() {
  errorInputRef.value?.click()
}

function openSupplementImport() {
  supplementInputRef.value?.click()
}

async function downloadErrorTemplate() {
  error.value = ''
  notice.value = ''
  try {
    await exportErrorImportTemplateWorkbook()
    notice.value = '出错导入模板已生成'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '模板生成失败'
  }
}

async function downloadSupplementTemplate() {
  error.value = ''
  notice.value = ''
  try {
    await exportSupplementImportTemplateWorkbook()
    notice.value = '补录导入模板已生成'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '补录模板生成失败'
  }
}

async function handleErrorImport(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  target.value = ''
  if (!file) return
  await prepareImportReview('error-deduction', [file])
}

async function prepareImportReview(kind: 'error-deduction' | 'supplement', files: File[]) {
  if (!files.length) return
  error.value = ''
  notice.value = ''
  importReviewLoading.value = true
  try {
    pendingImportKind.value = kind
    importReviewSource.value = await buildImportReviewSource(kind, files, ++importReviewRevision.value)
    notice.value = kind === 'supplement' ? `已载入 ${formatInt(files.length)} 个补录 Excel，确认前可先校对。` : '质检扣款 Excel 已载入，确认前可先校对。'
  } catch (err) {
    pendingImportKind.value = ''
    importReviewSource.value = null
    error.value = err instanceof Error ? err.message : 'Excel 预览加载失败'
  } finally {
    importReviewLoading.value = false
  }
}

async function handleSupplementImport(event: Event) {
  const target = event.target as HTMLInputElement
  const files = Array.from(target.files ?? [])
  target.value = ''
  await prepareImportReview('supplement', files)
}

async function handleSupplementDrop(event: DragEvent) {
  const files = Array.from(event.dataTransfer?.files ?? []).filter(isExcelFile)
  await prepareImportReview('supplement', files)
}

function isExcelFile(file: File) {
  return /\.(xlsx|xls)$/i.test(file.name)
}

async function importSupplementFiles(files: File[]) {
  if (!files.length) return
  error.value = ''
  notice.value = ''
  try {
    if (!supplementMonth.value) {
      error.value = '请选择补录月份'
      return
    }
    let created = 0
    let failed = 0
    const messages: string[] = []
    for (const file of files) {
      try {
        const result = await assetWorkbenchApi.importSettlementSupplementsExcel(supplementMonth.value, file)
        created += result.created?.length ?? 0
        failed += result.failures?.length ?? 0
        messages.push(...(result.failures ?? []).slice(0, 3).map((item) => `${file.name} 第 ${item.row} 行：${item.reason}`))
      } catch (err) {
        failed += 1
        messages.push(`${file.name}：${err instanceof Error ? err.message : '导入失败'}`)
      }
    }
    notice.value = `补录 Excel 已导入：文件 ${formatInt(files.length)} 个，成功 ${formatInt(created)} 行，失败 ${formatInt(failed)} 项`
    if (messages.length > 0) {
      error.value = messages.slice(0, 5).join('；')
    }
    await loadSettlement({ keepNotice: true })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '补录 Excel 批量导入失败'
  }
}

async function handleImportReviewAction(payload: WorkbenchSpreadsheetActionPayload) {
  if (payload.action.key === 'cancel_import') {
    cancelImportReview()
    return
  }
  if (payload.action.key !== 'confirm_import' || !importReviewSource.value || !pendingImportKind.value) return
  importReviewLoading.value = true
  error.value = ''
  notice.value = ''
  try {
    const files = await workbookReviewRowsToFiles(importReviewSource.value, payload.sheets, `asset-workbench-${pendingImportKind.value}`)
    if (pendingImportKind.value === 'error-deduction') {
      await importErrorReviewFiles(files)
    } else {
      await importSupplementFiles(files)
    }
    cancelImportReview({ keepNotice: true })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '导入校对表提交失败'
  } finally {
    importReviewLoading.value = false
  }
}

async function importErrorReviewFiles(files: File[]) {
  let matched = 0
  let unmatched = 0
  let ambiguous = 0
  for (const file of files) {
    const batch = await assetWorkbenchApi.importErrorExcel(month.value, file)
    matched += batch.matched_rows
    unmatched += batch.unmatched_rows
    ambiguous += batch.ambiguous_rows
  }
  notice.value = `质检出错 Excel 已导入：匹配 ${formatInt(matched)} 行，未匹配 ${formatInt(unmatched)} 行，多匹配 ${formatInt(ambiguous)} 行`
  await loadSettlement({ keepNotice: true })
}

function cancelImportReview(options: { keepNotice?: boolean } = {}) {
  importReviewSource.value = null
  pendingImportKind.value = ''
  if (!options.keepNotice) notice.value = ''
}

async function handleSettlementSpreadsheetAction(payload: WorkbenchSpreadsheetActionPayload) {
  if (payload.action.key === 'refresh_settlement') {
    await loadSettlement()
    return
  }
  if (payload.action.key === 'export_settlement') {
    await exportSettlement()
    return
  }
  if (payload.action.key === 'open_error_import') {
    openErrorImport()
    return
  }
  if (payload.action.key === 'open_supplement_import') {
    openSupplementImport()
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
    error.value = err instanceof Error ? err.message : '保存补录开放设置失败'
  }
}

async function loadSupplementEligibleMonths() {
  if (!permissionForm.value.payee_user_id) {
    error.value = '请先填写要开放补录的人员编号'
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

async function loadEntrySupplementEligibleMonths() {
  if (!supplementForm.value.payee_user_id) {
    error.value = '请先填写补录人员编号'
    return
  }
  error.value = ''
  notice.value = ''
  entryEligibleMonthsLoading.value = true
  try {
    const months = await assetWorkbenchApi.listSupplementEligibleMonths(supplementForm.value.payee_user_id)
    entryEligibleSupplementMonths.value = months
    entryEligiblePayeeUserId.value = supplementForm.value.payee_user_id
    if (months.length && !months.includes(supplementMonth.value)) {
      supplementMonth.value = months[0]
    }
    notice.value = months.length ? '已读取该人员可补录月份' : '该人员暂无已确认结算月份'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '读取补录录入月份失败'
  } finally {
    entryEligibleMonthsLoading.value = false
  }
}

function requireEntrySupplementMonth() {
  if (!supplementForm.value.payee_user_id) {
    error.value = '请填写补录人员编号'
    return ''
  }
  if (entryEligiblePayeeUserId.value !== supplementForm.value.payee_user_id) {
    error.value = '请先读取该人员的可补录月份'
    return ''
  }
  if (!entryEligibleSupplementMonths.value.length) {
    error.value = '该人员暂无可补录月份'
    return ''
  }
  if (!entryEligibleSupplementMonths.value.includes(supplementMonth.value)) {
    error.value = '请选择该人员的可补录月份'
    return ''
  }
  return supplementMonth.value
}

async function createSupplement() {
  error.value = ''
  notice.value = ''
  const dateMonth = businessMonthFromDate(supplementForm.value.supplement_date)
  if (!dateMonth) {
    error.value = '请选择补录日期'
    return
  }
  supplementMonth.value = dateMonth
  const selectedMonth = requireEntrySupplementMonth()
  if (!selectedMonth) return
  try {
    const payload = {
      ...supplementForm.value,
      business_month: selectedMonth,
      status: 'approved',
    }
    await assetWorkbenchApi.createSettlementSupplement(payload)
    notice.value = '月度补录已创建'
    supplementForm.value.order_no = ''
    supplementForm.value.supplement_date = todayDate()
    supplementForm.value.page_count = 1
    supplementForm.value.gross_amount = 0
    await loadSettlement({ keepNotice: true })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '创建补录失败'
  }
}

function startDeleteSupplement(row: SettlementSupplementRow) {
  pendingDeleteSupplement.value = row
  supplementDeleteReason.value = ''
}

async function deleteSupplement() {
  const row = pendingDeleteSupplement.value
  if (!row) return
  const reason = supplementDeleteReason.value.trim()
  if (!reason) {
    error.value = '请填写删除原因'
    return
  }
  error.value = ''
  notice.value = ''
  try {
    await assetWorkbenchApi.deleteSettlementSupplement(row.id, reason)
    notice.value = `已删除补录：${row.order_no}`
    pendingDeleteSupplement.value = null
    supplementDeleteReason.value = ''
    await loadSettlement({ keepNotice: true })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '删除补录失败'
  }
}

async function createAdjustment() {
  const batch = selectedBatch.value?.batch
  if (!batch) return
  error.value = ''
  notice.value = ''
  try {
    const adjustmentActionLabel = adjustmentForm.value.direction === 'debit' ? '扣回金额' : '补发金额'
    const created = await assetWorkbenchApi.createSettlementAdjustment(batch.id, {
      ...adjustmentForm.value,
      payload_json: {
        created_from: 'settlement_page',
        batch_no: batch.batch_no,
      },
    })
    notice.value = `已记录${adjustmentActionLabel}：${formatMoney(created.amount)}`
    adjustmentForm.value.amount = 0
    adjustmentForm.value.reason = ''
    await loadSettlement({ keepNotice: true })
    selectedBatch.value = await assetWorkbenchApi.getSettlementBatchDetail(batch.id)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '工资调整保存失败'
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

watch(month, (value) => {
  supplementPage.value = 1
  if (!entryEligibleSupplementMonths.value.length) {
    supplementMonth.value = value
  }
})

watch(
  () => supplementForm.value.supplement_date,
  (value) => {
    const nextMonth = businessMonthFromDate(value)
    if (nextMonth) {
      supplementMonth.value = nextMonth
    }
  },
)

onMounted(() => {
  void loadSettlement()
})
</script>

<template>
  <section class="aw-page-stack">
    <SettlementHubTabs />
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">本月结算</p>
        <h2>工资结算</h2>
        <p>按“预览工资 → 批次确认 → 补录处理 → 工资调整”的顺序操作，避免把结算动作混在同一屏。</p>
      </div>
      <div class="aw-page-bar__actions">
        <label class="aw-month-control">
          <span>结算月</span>
          <input v-model="month" type="month" aria-label="结算月份" />
        </label>
        <button class="aw-secondary-button" type="button" @click="() => loadSettlement()">刷新</button>
        <button class="aw-secondary-button" type="button" @click="settlementSpreadsheetOpen = !settlementSpreadsheetOpen">
          <Table2 :size="16" aria-hidden="true" />
          {{ settlementSpreadsheetOpen ? '收起表格工作台' : '表格工作台' }}
        </button>
      </div>
    </div>
    <input
      ref="errorInputRef"
      class="aw-visually-hidden"
      type="file"
      accept=".xlsx,.xls"
      aria-label="导入质检扣款 Excel"
      @change="handleErrorImport"
    />
    <input
      ref="supplementInputRef"
      class="aw-visually-hidden"
      type="file"
      accept=".xlsx,.xls"
      multiple
      aria-label="导入补录 Excel"
      @change="handleSupplementImport"
    />
    <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
    <p v-if="importReviewLoading" class="aw-inline-alert">正在处理导入校对表…</p>

    <SpreadsheetWorkbench
      v-if="importReviewSource"
      :source="importReviewSource"
      :height="460"
      @close="cancelImportReview"
      @action="handleImportReviewAction"
    />
    <SpreadsheetWorkbench
      v-if="settlementSpreadsheetOpen"
      :source="settlementSpreadsheetSource"
      :height="520"
      @close="settlementSpreadsheetOpen = false"
      @action="handleSettlementSpreadsheetAction"
    />

    <div class="aw-settlement-workbench">
      <aside class="aw-settlement-nav" aria-label="结算操作导航">
        <div class="aw-settlement-nav__head">
          <strong>{{ month }} 结算</strong>
          <span>按顺序处理，当前只展示一个工作区</span>
        </div>
        <div v-for="group in settlementNavGroups" :key="group.label" class="aw-settlement-nav__group">
          <p>{{ group.label }}</p>
          <button
            v-for="item in group.items"
            :key="item.key"
            class="aw-settlement-nav__item"
            :class="{ 'is-active': activeSettlementSection === item.key }"
            type="button"
            :aria-current="activeSettlementSection === item.key ? 'page' : undefined"
            @click="activeSettlementSection = item.key"
          >
            <span>
              <strong>{{ item.title }}</strong>
              <small>{{ item.meta }}</small>
            </span>
            <em>{{ item.count }}</em>
          </button>
        </div>
      </aside>

      <main class="aw-settlement-workspace">
        <section v-if="activeSettlementSection === 'preview'" class="aw-settlement-section">
          <div class="aw-settlement-section__head">
            <div>
              <h3>预览工资</h3>
              <p>先核对本月工资预览；质检扣款表只影响预览和后续批次，不会直接确认发薪。</p>
            </div>
            <div class="aw-inline-actions">
              <button class="aw-secondary-button" type="button" @click="downloadErrorTemplate">
                <Download :size="16" aria-hidden="true" />
                下载质检扣款表
              </button>
              <button class="aw-secondary-button" type="button" @click="openErrorImport">导入质检扣款</button>
              <button class="aw-secondary-button" type="button" :disabled="exporting || (!preview && !selectedBatch)" @click="exportSettlement">
                导出工资条
              </button>
            </div>
          </div>

          <LedgerReadout :eyebrow="`结算台账 · ${month}`" title="本月工资预览" :segments="ledgerSegments">
            <template #actions>
              <button class="aw-console-button" type="button" @click="() => loadSettlement()">刷新预览</button>
            </template>
            <template #detail>
              <div class="aw-sheet-detail">
                <WorkbenchDataGrid
                  v-if="payrollRows.length"
                  :columns="payrollGridColumns"
                  :rows="payrollGridRows"
                  row-key="grid_id"
                  storage-key="settlement-ledger-payroll"
                  group-by="payee_user_id"
                  :height="440"
                  :row-height="34"
                >
                  <template #cell="{ column, value }">
                    <span v-if="isMoneyColumn(column.key)" class="aw-cell-money">{{ gridValue(column.key, value) }}</span>
                    <span v-else-if="column.align === 'right'" class="aw-cell-num">{{ gridValue(column.key, value) }}</span>
                    <span v-else>{{ gridValue(column.key, value) }}</span>
                  </template>
                </WorkbenchDataGrid>
                <div v-else class="aw-empty-state">
                  <h3>还没有可结算明细</h3>
                  <p>导入质检扣款表并生成预览后，这里会按人员分组显示每条工资行的计件、质检扣款与净额。</p>
                </div>
              </div>
            </template>
          </LedgerReadout>

          <div class="aw-panel">
            <div class="aw-panel__head">
              <h3>工资条明细</h3>
              <span class="aw-chip aw-chip--neutral">无补录时第二行显示 0</span>
            </div>
            <AsyncBoundary
              :loading="loading"
              :error="error"
              :loading-label="`正在加载 ${month} 结算预览`"
              @retry="() => loadSettlement()"
            >
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
                  <span v-if="isMoneyColumn(column.key)" class="aw-cell-money">{{ gridValue(column.key, value) }}</span>
                  <span v-else-if="column.align === 'right'" class="aw-cell-num">{{ gridValue(column.key, value) }}</span>
                  <span v-else>{{ gridValue(column.key, value) }}</span>
                </template>
              </WorkbenchDataGrid>
              <div v-else class="aw-empty-state">
                <h3>还没有可结算明细</h3>
                <p>生成预览后会在这里显示工资条明细。</p>
              </div>
            </AsyncBoundary>
          </div>
        </section>

        <section v-if="activeSettlementSection === 'batches'" class="aw-settlement-section">
          <div class="aw-settlement-section__head">
            <div>
              <h3>批次确认</h3>
              <p>生成批次会锁定本月可结算明细；未确认前可以取消后重新生成。</p>
            </div>
            <button class="aw-primary-button" type="button" :disabled="generatingBatch || loading || hasGeneratedBatch" @click="generateBatch">
              {{ generatingBatch ? '生成中…' : hasGeneratedBatch ? '已有待确认批次' : '生成批次' }}
            </button>
          </div>

          <div class="aw-data-surface">
            <div class="aw-grid-toolbar">
              <span>结算批次</span>
              <span>{{ formatInt(batches.length) }} 个批次</span>
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
                  <button
                    v-if="gridRowAsBatch(row).status === 'generated'"
                    type="button"
                    :disabled="confirmingBatchId === gridRowAsBatch(row).id"
                    @click="confirmBatch(gridRowAsBatch(row))"
                  >
                    {{ confirmingBatchId === gridRowAsBatch(row).id ? '确认中…' : '确认' }}
                  </button>
                  <button v-if="gridRowAsBatch(row).status === 'generated'" type="button" @click="startCancelBatch(gridRowAsBatch(row))">
                    取消
                  </button>
                </div>
                <span
                  v-else-if="column.key === 'status_label'"
                  :class="chipClass(batchStatusMeta(gridRowAsBatch(row).status).tone)"
                >
                  {{ value }}
                </span>
                <span v-else-if="isMoneyColumn(column.key)" class="aw-cell-money">{{ gridValue(column.key, value) }}</span>
                <span v-else>{{ gridValue(column.key, value) }}</span>
              </template>
            </WorkbenchDataGrid>
            <div v-if="pendingCancelBatch" class="aw-panel">
              <div class="aw-panel__head">
                <div>
                  <h3>取消结算批次</h3>
                  <p class="aw-copy">{{ pendingCancelBatch.batch_no }}</p>
                </div>
                <span :class="chipClass(batchStatusMeta(pendingCancelBatch.status).tone)">
                  {{ batchStatusMeta(pendingCancelBatch.status).label }}
                </span>
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
              <p>先在“预览工资”确认金额，再生成批次。</p>
            </div>
          </div>

          <div v-if="selectedBatch" class="aw-detail-panel">
            <div class="aw-detail-panel__head">
              <div>
                <p class="aw-eyebrow">批次明细</p>
                <h3>{{ selectedBatch.batch.batch_no }}</h3>
              </div>
              <div class="aw-inline-actions">
                <span :class="chipClass(batchStatusMeta(selectedBatch.batch.status).tone)">{{ batchStatusMeta(selectedBatch.batch.status).label }}</span>
                <strong class="aw-cell-money">{{ formatMoney(selectedBatch.batch.net_amount) }}</strong>
                <button
                  v-if="selectedBatch.batch.status === 'generated'"
                  class="aw-primary-button"
                  type="button"
                  :disabled="confirmingBatchId === selectedBatch.batch.id"
                  @click="confirmBatch(selectedBatch.batch)"
                >
                  {{ confirmingBatchId === selectedBatch.batch.id ? '确认中…' : '确认批次' }}
                </button>
                <button v-if="selectedBatch.batch.status === 'generated'" class="aw-secondary-button" type="button" @click="startCancelBatch(selectedBatch.batch)">取消批次</button>
                <button v-if="selectedBatch.batch.status === 'confirmed'" class="aw-secondary-button" type="button" @click="activeSettlementSection = 'adjustments'">去记录工资调整</button>
              </div>
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
              <template #cell="{ row, column, value }">
                <span v-if="column.key === 'item_type_label'" :class="chipClass(itemTypeMeta(gridRowAsBatchItem(row).item_type).tone)">{{ value }}</span>
                <span v-else-if="column.key === 'direction_label'" :class="chipClass(directionMeta(gridRowAsBatchItem(row).direction).tone)">{{ value }}</span>
                <span v-else-if="isMoneyColumn(column.key)" class="aw-cell-money">{{ gridValue(column.key, value) }}</span>
                <span v-else-if="column.align === 'right'" class="aw-cell-num">{{ gridValue(column.key, value) }}</span>
                <span v-else>{{ gridValue(column.key, value) }}</span>
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
                <span v-if="isMoneyColumn(column.key)" class="aw-cell-money">{{ gridValue(column.key, value) }}</span>
                <span v-else-if="column.align === 'right'" class="aw-cell-num">{{ gridValue(column.key, value) }}</span>
                <span v-else>{{ gridValue(column.key, value) }}</span>
              </template>
            </WorkbenchDataGrid>
          </div>
        </section>

        <section v-if="activeSettlementSection === 'supplements'" class="aw-settlement-section">
          <div class="aw-settlement-section__head">
            <div>
              <h3>补录管理</h3>
              <p>员工先申请，管理员按自然月开放补录权限；权限可随时关闭。补录金额在工资条中单独成行。</p>
            </div>
            <div class="aw-inline-actions">
              <button class="aw-secondary-button" type="button" @click="downloadSupplementTemplate">补录模板</button>
              <button class="aw-secondary-button" type="button" @click="openSupplementImport">导入补录</button>
            </div>
          </div>

          <div class="aw-two-column">
            <div class="aw-panel">
              <div class="aw-panel__head">
                <div>
                  <h3>补录开放</h3>
                  <p class="aw-copy">收到申请后，按人员和自然月开放；关闭后该人员不能再提交该月补录。</p>
                </div>
                <span :class="chipClass(enabledMeta(permissionForm.enabled).tone)">{{ enabledMeta(permissionForm.enabled).label }}</span>
              </div>
              <div class="aw-form-grid">
                <label>
                  开放人员编号
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
              <div class="aw-inline-actions">
                <button class="aw-secondary-button" type="button" :disabled="eligibleMonthsLoading" @click="loadSupplementEligibleMonths">
                  读取可补录月份
                </button>
                <button class="aw-primary-button" type="button" @click="upsertSupplementPermission">保存开放设置</button>
              </div>
            </div>

            <div class="aw-panel">
              <div class="aw-panel__head">
                <div>
                  <h3>补录录入</h3>
                  <p class="aw-copy">已批准补录会在工资条中单独形成补录计件工资行。</p>
                </div>
                <span class="aw-chip aw-chip--info">无补录显示 0</span>
              </div>
              <div class="aw-form-grid">
                <label>
                  人员编号
                  <input v-model.number="supplementForm.payee_user_id" type="number" min="1" />
                </label>
                <label>
                  补录月份
                  <select v-model="supplementMonth" :disabled="!entryEligibleReady">
                    <option v-if="!entryEligibleReady" :value="supplementMonth">{{ supplementMonth }}</option>
                    <option v-for="item in entryEligibleSupplementMonths" :key="item" :value="item">{{ item }}</option>
                  </select>
                </label>
                <label>
                  补录日期
                  <input v-model="supplementForm.supplement_date" type="date" />
                </label>
                <label>
                  文件/作品名称
                  <input v-model="supplementForm.order_no" placeholder="填写文件名或作品名" />
                </label>
                <label>
                  难度
                  <select v-model="supplementForm.difficulty_class">
                    <option v-for="difficulty in difficultyOptions" :key="difficulty" :value="difficulty">{{ difficulty }}</option>
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
              <div class="aw-inline-actions">
                <button class="aw-secondary-button" type="button" :disabled="entryEligibleMonthsLoading" @click="loadEntrySupplementEligibleMonths">
                  读取录入月份
                </button>
                <button class="aw-primary-button" type="button" @click="createSupplement">创建补录</button>
              </div>
              <p v-if="manualSupplementDuplicateWarning" class="aw-inline-alert aw-inline-alert--warning">
                {{ manualSupplementDuplicateWarning }}
              </p>
              <p class="aw-copy">补录日期只用于说明想补哪一天，结算按该日期所在自然月进入“补录计件工资”。上传图片和分类仍按上传文件模块的目录规则执行。</p>
              <div class="aw-inline-alert" @dragover.prevent @drop.prevent="handleSupplementDrop">
                <span>补录 Excel 批量导入</span>
                <button class="aw-secondary-button" type="button" @click="openSupplementImport">选择文件</button>
              </div>
            </div>
          </div>

          <div class="aw-data-surface">
            <div class="aw-grid-toolbar">
              <span>补录开放记录</span>
              <span>{{ formatInt(supplementPermissions.length) }} 条</span>
            </div>
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
              <template #cell="{ row, column, value }">
                <span
                  v-if="column.key === 'status_label'"
                  :class="chipClass(gridRowAsPermission(row).enabled ? 'success' : 'neutral')"
                >
                  {{ value }}
                </span>
                <span v-else-if="column.align === 'right'" class="aw-cell-num">{{ gridValue(column.key, value) }}</span>
                <span v-else>{{ gridValue(column.key, value) }}</span>
              </template>
            </WorkbenchDataGrid>
            <p v-else class="aw-copy">当前月份还没有补录开放记录</p>
          </div>

          <div class="aw-data-surface">
            <div class="aw-grid-toolbar">
              <span>补录查询</span>
              <span>第 {{ formatInt(supplementPage) }} 页 · {{ formatInt(filteredSupplementRows.length) }} / {{ formatInt(supplementTotal) }} 条</span>
            </div>
            <div class="aw-form-grid aw-form-grid--compact">
              <label>
                文件/作品名称（精确）
                <input v-model.trim="supplementSearch" placeholder="完整文件名或作品名" @keydown.enter.prevent="querySupplements" />
              </label>
              <label>
                人员编号（精确）
                <input v-model.trim="supplementPayeeFilter" inputmode="numeric" placeholder="例如 1001" @keydown.enter.prevent="querySupplements" />
              </label>
              <label>
                状态
                <select v-model="supplementStatusFilter">
                  <option value="">全部状态</option>
                  <option value="draft">草稿</option>
                  <option value="approved">已批准</option>
                  <option value="in_batch">批次中</option>
                  <option value="settled">已结算</option>
                  <option value="voided">已作废</option>
                </select>
              </label>
              <label>
                补录日期
                <input v-model="supplementDateFilter" type="date" />
              </label>
              <label>
                排序字段
                <select v-model="supplementSortBy">
                  <option value="supplement_date">补录日期</option>
                  <option value="payee_user_id">人员编号</option>
                  <option value="order_no">作品名称</option>
                  <option value="gross_amount">补录金额</option>
                  <option value="created_at">创建时间</option>
                  <option value="updated_at">更新时间</option>
                </select>
              </label>
              <label>
                排序方向
                <select v-model="supplementSortDir">
                  <option value="desc">从新到旧 / 从大到小</option>
                  <option value="asc">从旧到新 / 从小到大</option>
                </select>
              </label>
              <label>
                每页
                <select v-model.number="supplementPageSize" @change="querySupplements">
                  <option :value="20">20 条</option>
                  <option :value="50">50 条</option>
                  <option :value="100">100 条</option>
                </select>
              </label>
            </div>
            <div class="aw-inline-actions">
              <button class="aw-primary-button" type="button" @click="querySupplements">查询补录</button>
              <button class="aw-secondary-button" type="button" @click="resetSupplementQuery">重置条件</button>
            </div>
            <WorkbenchDataGrid
              v-if="filteredSupplementRows.length"
              :columns="supplementGridColumns"
              :rows="supplementGridRows"
              row-key="id"
              storage-key="settlement-supplements"
              group-by="status_label"
              :height="220"
              :row-height="34"
            >
              <template #cell="{ row, column, value }">
                <div v-if="column.key === 'action'" class="aw-inline-actions">
                  <button
                    type="button"
                    :disabled="['in_batch', 'settled', 'voided'].includes(gridRowAsSupplement(row).status)"
                    @click="startDeleteSupplement(gridRowAsSupplement(row))"
                  >
                    删除
                  </button>
                </div>
                <span
                  v-else-if="column.key === 'status_label'"
                  :class="chipClass(supplementStatusMeta(gridRowAsSupplement(row).status).tone)"
                >
                  {{ value }}
                </span>
                <span
                  v-else-if="column.key === 'duplicate_label'"
                  :class="chipClass(duplicateMeta(gridRowAsSupplement(row).duplicate_hint_json?.has_duplicates).tone)"
                >
                  {{ value }}
                </span>
                <span v-else-if="isMoneyColumn(column.key)" class="aw-cell-money">{{ gridValue(column.key, value) }}</span>
                <span v-else>{{ gridValue(column.key, value) }}</span>
              </template>
            </WorkbenchDataGrid>
            <div v-if="supplementTotal > 0" class="aw-drive-pager">
              <button class="aw-secondary-button" type="button" :disabled="!supplementCanPrev || loading" @click="changeSupplementPage(-1)">
                上一页
              </button>
              <span>第 {{ formatInt(supplementPage) }} 页</span>
              <button class="aw-secondary-button" type="button" :disabled="!supplementCanNext || loading" @click="changeSupplementPage(1)">
                下一页
              </button>
            </div>
            <div v-if="pendingDeleteSupplement" class="aw-panel">
              <div class="aw-panel__head">
                <div>
                  <h3>删除补录</h3>
                  <p class="aw-copy">{{ pendingDeleteSupplement.order_no }} · {{ pendingDeleteSupplement.business_month }}</p>
                </div>
                <span :class="chipClass(supplementStatusMeta(pendingDeleteSupplement.status).tone)">
                  {{ supplementStatusMeta(pendingDeleteSupplement.status).label }}
                </span>
              </div>
              <label class="aw-field">
                <span>删除原因</span>
                <input v-model="supplementDeleteReason" required />
              </label>
              <div class="aw-inline-actions">
                <button class="aw-primary-button" type="button" @click="deleteSupplement">确认删除</button>
                <button class="aw-secondary-button" type="button" @click="pendingDeleteSupplement = null">取消</button>
              </div>
            </div>
            <p v-else-if="supplements.length" class="aw-copy">没有匹配当前查询条件的补录记录</p>
            <p v-else class="aw-copy">当前月份没有补录记录</p>
          </div>
        </section>

        <section v-if="activeSettlementSection === 'adjustments'" class="aw-settlement-section">
          <div class="aw-settlement-section__head">
            <div>
              <h3>工资调整</h3>
              <p>已确认批次不直接修改原始工资明细。需要补发或扣回时，在这里记录金额和原因。</p>
            </div>
            <button class="aw-secondary-button" type="button" @click="activeSettlementSection = 'batches'">选择批次</button>
          </div>

          <div v-if="selectedBatch" class="aw-panel">
            <div class="aw-panel__head">
              <div>
                <p class="aw-eyebrow">当前批次</p>
                <h3>{{ selectedBatch.batch.batch_no }}</h3>
              </div>
              <span :class="chipClass(batchStatusMeta(selectedBatch.batch.status).tone)">{{ batchStatusMeta(selectedBatch.batch.status).label }}</span>
            </div>
            <template v-if="selectedBatch.batch.status === 'confirmed'">
              <div class="aw-form-grid">
                <label>
                  人员编号
                  <input v-model.number="adjustmentForm.payee_user_id" type="number" min="1" />
                </label>
                <label>
                  处理方式
                  <select v-model="adjustmentMode">
                    <option value="credit">补发给员工</option>
                    <option value="debit">从工资中扣回</option>
                  </select>
                </label>
                <label>
                  金额
                  <input v-model.number="adjustmentForm.amount" type="number" min="0" />
                </label>
                <label class="aw-form-grid__full">
                  原因
                  <input v-model="adjustmentForm.reason" />
                </label>
              </div>
              <button class="aw-primary-button" type="button" @click="createAdjustment">保存调整记录</button>
            </template>
            <p v-else class="aw-copy">批次确认后才能记录工资调整。</p>
          </div>
          <div v-else class="aw-empty-state">
            <h3>还没有选择批次</h3>
            <p>先进入“批次确认”，打开一个已确认批次后再记录工资调整。</p>
            <button class="aw-primary-button" type="button" @click="activeSettlementSection = 'batches'">去选择批次</button>
          </div>
        </section>
      </main>
    </div>
  </section>
</template>
