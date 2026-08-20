<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import {
  assetWorkbenchApi,
  type DeductionRuleRow,
  type DifficultyClassRow,
  type PriceMatrixRow,
  type PromoCouponRow,
  type WelfareRuleRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { useRoutePageCopy } from '@aw/app/useRoutePageCopy'
import { difficultyCodes, difficultyOptionsWithAll, firstDifficultyCode } from '@aw/shared/format/difficulty'
import { formatMoney, formatPercent } from '@aw/shared/format/number'
import { chipClass, enabledMeta, promoModeMeta, workerTypeMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'
import { resolveApiUserMessage } from '@/utils/api-message-zh'

interface GridColumn {
  key: string
  label: string
  width: number
  align?: 'left' | 'right' | 'center'
}

type PriceGridRow = PriceMatrixRow & { worker_type_label: string; unit_price_label: string; enabled_label: string; action: string }
type DeductionGridRow = DeductionRuleRow & { worker_type_label: string; deduction_amount_label: string; enabled_label: string; action: string }
type WelfareGridRow = WelfareRuleRow & { worker_type_label: string; amount_label: string; enabled_label: string; action: string }
type PromoGridRow = PromoCouponRow & { worker_type_label: string; mode_label: string; value_label: string; enabled_label: string; action: string }
type DifficultyGridRow = DifficultyClassRow & { enabled_label: string; action: string }
type CostCenterData = Awaited<ReturnType<typeof fetchCostCenter>>
type PricingSectionKey = 'guide' | 'difficulty' | 'price' | 'deduction' | 'welfare' | 'promo'

const parttimeGrades = ['J1', 'J2', 'J3']
const fulltimeGrades = ['P1', 'P2', 'P3', 'P4', 'S1', 'S2', 'M1', 'M2']
const allGradeOptions = ['all']

const difficultyRows = ref<DifficultyClassRow[]>([])
const priceRows = ref<PriceMatrixRow[]>([])
const deductionRows = ref<DeductionRuleRow[]>([])
const welfareRows = ref<WelfareRuleRow[]>([])
const promoRows = ref<PromoCouponRow[]>([])
const activePricingSection = ref<PricingSectionKey>('guide')
const priceSupersedeId = ref(0)
const confirmingPriceSupersede = ref(false)
const pendingPriceToggle = ref<PriceGridRow | null>(null)
const deductionSupersedeId = ref(0)
const welfareSupersedeId = ref(0)
const promoSupersedeId = ref(0)
const difficultyEditingCode = ref('')
const totals = ref({
  price: 0,
  deduction: 0,
  welfare: 0,
  promo: 0,
})
const notice = ref('')
const { label: pageLabel, subtitle: pageSubtitle } = useRoutePageCopy('/settings/pricing')
const costCenterRequest = usePageRequest<CostCenterData>(fetchCostCenter, null, '计价设置加载失败')
const loading = costCenterRequest.loading
const error = costCenterRequest.error
const today = shanghaiDateInput()
const difficultyForm = ref({
  code: '',
  name: '',
  description: '',
  enabled: true,
  sort_order: 50,
})
const priceForm = ref({
  worker_type: 'fulltime',
  job_grade: 'P1',
  difficulty_class: '',
  unit_price: 0,
  effective_from: today,
  effective_to: '',
  remark: '',
})
const deductionForm = ref({
  worker_type: 'all',
  job_grade: 'all',
  difficulty_class: '',
  deduction_amount: 0,
  effective_from: today,
  effective_to: '',
})
const welfareForm = ref({
  rule_name: '月度补贴',
  worker_type: 'all',
  job_grade: 'all',
  rule_type: 'manual_monthly',
  amount: 0,
  effective_from: today,
  effective_to: '',
})
const promoForm = ref({
  coupon_code: '',
  coupon_name: '',
  mode: 'fixed_price',
  amount: 0,
  percent: 0,
  priority: 10,
  worker_type: 'all',
  job_grade: 'all',
  difficulty_class: 'all',
  eligible_user_ids: '',
  eligible_codes: '',
  effective_from: today,
  effective_to: '',
})
const priceGradeOptions = computed(() => gradeOptionsFor(priceForm.value.worker_type, false))
const deductionGradeOptions = computed(() => gradeOptionsFor(deductionForm.value.worker_type, true))
const welfareGradeOptions = computed(() => gradeOptionsFor(welfareForm.value.worker_type, true))
const promoGradeOptions = computed(() => gradeOptionsFor(promoForm.value.worker_type, true))
const difficultyOptionCodes = computed(() => difficultyCodes(difficultyRows.value))
const difficultyOptionCodesWithAll = computed(() => difficultyOptionsWithAll(difficultyRows.value))
const pricingNavGroups = computed(() => [
  {
    label: '一级菜单：基础档案',
    items: [
      { key: 'guide' as const, title: '配置向导', meta: '先看设置顺序', count: '建议先读' },
      { key: 'difficulty' as const, title: '难度分类', meta: '三级字段：代码 / 名称 / 启用', count: `${difficultyRows.value.length} 项` },
    ],
  },
  {
    label: '一级菜单：日常计价',
    items: [
      { key: 'price' as const, title: '单价设置', meta: '三级字段：人员类型 / 岗级 / 难度 / 单价 / 生效日', count: `${totals.value.price} 条` },
    ],
  },
  {
    label: '一级菜单：结算调整',
    items: [
      { key: 'deduction' as const, title: '质检扣款', meta: '按导入的质检错误在结算时扣款', count: `${totals.value.deduction} 条` },
      { key: 'welfare' as const, title: '福利补贴', meta: '按人员和月份发放，不绑定单个文件', count: `${totals.value.welfare} 条` },
      { key: 'promo' as const, title: '临时活动价', meta: '临时一口价、加价或涨幅', count: `${totals.value.promo} 条` },
    ],
  },
])
const difficultyRowsWithLabels = computed<DifficultyGridRow[]>(() =>
  safeRows<DifficultyClassRow>(difficultyRows.value).map((row) => ({
    ...row,
    enabled_label: enabledMeta(row.enabled).label,
    action: 'actions',
  })),
)
const difficultyGridRows = computed(() => difficultyRowsWithLabels.value as unknown as Record<string, unknown>[])
const priceRowsWithLabels = computed<PriceGridRow[]>(() =>
  safeRows<PriceMatrixRow>(priceRows.value).map((row) => ({
    ...row,
    worker_type_label: workerTypeMeta(row.worker_type).label,
    unit_price_label: formatMoney(row.unit_price),
    enabled_label: enabledMeta(row.enabled).label,
    action: 'actions',
  })),
)
const deductionRowsWithLabels = computed<DeductionGridRow[]>(() =>
  safeRows<DeductionRuleRow>(deductionRows.value).map((row) => ({
    ...row,
    worker_type_label: workerTypeMeta(row.worker_type).label,
    deduction_amount_label: formatMoney(row.deduction_amount),
    enabled_label: enabledMeta(row.enabled).label,
    action: 'actions',
  })),
)
const welfareRowsWithLabels = computed<WelfareGridRow[]>(() =>
  safeRows<WelfareRuleRow>(welfareRows.value).map((row) => ({
    ...row,
    worker_type_label: workerTypeMeta(row.worker_type).label,
    amount_label: formatMoney(row.amount),
    enabled_label: enabledMeta(row.enabled).label,
    action: 'actions',
  })),
)
const priceGridRows = computed(() => priceRowsWithLabels.value as unknown as Record<string, unknown>[])
const priceSupersedeSource = computed(() => priceRowsWithLabels.value.find((row) => row.id === priceSupersedeId.value) ?? null)
const priceSupersedeCutoff = computed(() => {
  if (!priceForm.value.effective_from) return ''
  return addDaysToDateInput(priceForm.value.effective_from, -1)
})
const priceSupersedeConfirmText = computed(() => {
  const source = priceSupersedeSource.value
  if (!source) return ''
  return `确认从 ${priceForm.value.effective_from || '待填写'} 起，将 ${priceRuleIdentity(source)} 从 ${formatMoney(source.unit_price)} 调整为 ${formatMoney(priceForm.value.unit_price)}？`
})
const deductionGridRows = computed(() => deductionRowsWithLabels.value as unknown as Record<string, unknown>[])
const welfareGridRows = computed(() => welfareRowsWithLabels.value as unknown as Record<string, unknown>[])
const promoRowsWithLabels = computed<PromoGridRow[]>(() =>
  safeRows<PromoCouponRow>(promoRows.value).map((row) => ({
    ...row,
    worker_type_label: workerTypeMeta(row.worker_type).label,
    mode_label: promoModeMeta(row.mode).label,
    value_label: row.mode === 'markup_rate' ? formatPercent(row.percent ?? 0) : formatMoney(row.amount ?? 0),
    enabled_label: enabledMeta(row.enabled).label,
    action: 'actions',
  })),
)
const promoGridRows = computed(() => promoRowsWithLabels.value as unknown as Record<string, unknown>[])
const difficultyGridColumns = computed<GridColumn[]>(() => [
  { key: 'code', label: '代码', width: 120 },
  { key: 'name', label: '名称', width: 120 },
  { key: 'description', label: '说明', width: 220 },
  { key: 'sort_order', label: '排序', width: 80, align: 'right' },
  { key: 'enabled_label', label: '状态', width: 88 },
  { key: 'action', label: '动作', width: 156, align: 'center' },
])
const priceGridColumns = computed<GridColumn[]>(() => [
  { key: 'worker_type_label', label: '类型', width: 96 },
  { key: 'job_grade', label: '岗级', width: 96 },
  { key: 'difficulty_class', label: '难度', width: 132 },
  { key: 'unit_price_label', label: '单价', width: 104, align: 'right' },
  { key: 'enabled_label', label: '状态', width: 88 },
  { key: 'action', label: '动作', width: 196, align: 'center' },
])
const deductionGridColumns = computed<GridColumn[]>(() => [
  { key: 'worker_type_label', label: '类型', width: 96 },
  { key: 'job_grade', label: '岗级', width: 96 },
  { key: 'difficulty_class', label: '难度', width: 120 },
  { key: 'deduction_amount_label', label: '每错扣款', width: 112, align: 'right' },
  { key: 'enabled_label', label: '启用', width: 88 },
  { key: 'action', label: '动作', width: 156, align: 'center' },
])
const welfareGridColumns = computed<GridColumn[]>(() => [
  { key: 'rule_name', label: '名称', width: 140 },
  { key: 'worker_type_label', label: '类型', width: 96 },
  { key: 'job_grade', label: '岗级', width: 96 },
  { key: 'amount_label', label: '金额', width: 104, align: 'right' },
  { key: 'enabled_label', label: '启用', width: 88 },
  { key: 'action', label: '动作', width: 156, align: 'center' },
])
const promoGridColumns = computed<GridColumn[]>(() => [
  { key: 'coupon_code', label: '编码', width: 120 },
  { key: 'mode_label', label: '模式', width: 120 },
  { key: 'worker_type_label', label: '类型', width: 96 },
  { key: 'priority', label: '优先级', width: 96, align: 'right' },
  { key: 'value_label', label: '值', width: 96, align: 'right' },
  { key: 'enabled_label', label: '启用', width: 88 },
  { key: 'action', label: '动作', width: 156, align: 'center' },
])

function safeRows<T>(rows: T[] | unknown): T[] {
  return Array.isArray(rows) ? rows : []
}

function gridRowAsPrice(row: Record<string, unknown>): PriceGridRow {
  return row as unknown as PriceGridRow
}

function gridRowAsDeduction(row: Record<string, unknown>): DeductionGridRow {
  return row as unknown as DeductionGridRow
}

function gridRowAsWelfare(row: Record<string, unknown>): WelfareGridRow {
  return row as unknown as WelfareGridRow
}

function gridRowAsPromo(row: Record<string, unknown>): PromoGridRow {
  return row as unknown as PromoGridRow
}

function gridRowAsDifficulty(row: Record<string, unknown>): DifficultyGridRow {
  return row as unknown as DifficultyGridRow
}

async function fetchCostCenter() {
  const [difficulties, prices, deductions, welfare, promos] = await Promise.all([
    assetWorkbenchApi.listDifficultyClassesAdmin(),
    assetWorkbenchApi.listPriceMatrix({ page: 1, page_size: 20 }),
    assetWorkbenchApi.listDeductionRules({ page: 1, page_size: 20 }),
    assetWorkbenchApi.listWelfareRules({ page: 1, page_size: 20 }),
    assetWorkbenchApi.listPromoCoupons({ page: 1, page_size: 20 }),
  ])
  return { difficulties, prices, deductions, welfare, promos }
}

async function loadCostCenter() {
  const data = await costCenterRequest.run()
  if (!data) return
  const prices = Array.isArray(data.prices.items) ? data.prices.items : []
  const deductions = Array.isArray(data.deductions.items) ? data.deductions.items : []
  const welfare = Array.isArray(data.welfare.items) ? data.welfare.items : []
  const promos = Array.isArray(data.promos.items) ? data.promos.items : []
  difficultyRows.value = data.difficulties
  priceRows.value = prices
  deductionRows.value = deductions
  welfareRows.value = welfare
  promoRows.value = promos
  totals.value = {
    price: data.prices.total || prices.length,
    deduction: data.deductions.total || deductions.length,
    welfare: data.welfare.total || welfare.length,
    promo: data.promos.total || promos.length,
  }
  syncDifficultyDefaults()
}

function startOfShanghaiDay(date: string) {
  return date ? `${date}T00:00:00+08:00` : ''
}

function endOfShanghaiDay(date: string) {
  return date ? `${date}T23:59:59+08:00` : undefined
}

function toDateInput(value?: string) {
  if (!value) return ''
  if (/^\d{4}-\d{2}-\d{2}/.test(value)) return value.slice(0, 10)
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value.slice(0, 10)
  return date.toISOString().slice(0, 10)
}

function shanghaiDateInput(date = new Date()) {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(date)
  const values = Object.fromEntries(parts.map((part) => [part.type, part.value]))
  return `${values.year}-${values.month}-${values.day}`
}

function addDaysToDateInput(value: string, days: number) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!match) return value
  const [, year, month, day] = match
  const date = new Date(Date.UTC(Number(year), Number(month) - 1, Number(day) + days))
  return date.toISOString().slice(0, 10)
}

function nextPriceVersionEffectiveFrom(row: PriceGridRow) {
  const currentStart = toDateInput(row.effective_from)
  const earliest = currentStart ? addDaysToDateInput(currentStart, 1) : today
  return earliest > today ? earliest : today
}

function gradeOptionsFor(workerType: string, allowAll: boolean) {
  if (workerType === 'fulltime') return fulltimeGrades
  if (workerType === 'parttime') return parttimeGrades
  return allowAll ? allGradeOptions : [...parttimeGrades, ...fulltimeGrades]
}

function normalizeGradeForWorker(form: { worker_type: string; job_grade: string }, allowAll: boolean) {
  const options = gradeOptionsFor(form.worker_type, allowAll)
  if (!options.includes(form.job_grade)) {
    form.job_grade = options[0] ?? ''
  }
}

function normalizeDifficultySelection(value: string, allowAll: boolean) {
  if (allowAll && value === 'all') return value
  const options = difficultyOptionCodes.value
  return options.includes(value) ? value : firstDifficultyCode(difficultyRows.value)
}

function syncDifficultyDefaults() {
  priceForm.value.difficulty_class = normalizeDifficultySelection(priceForm.value.difficulty_class, false)
  deductionForm.value.difficulty_class = normalizeDifficultySelection(deductionForm.value.difficulty_class, true)
  promoForm.value.difficulty_class = normalizeDifficultySelection(promoForm.value.difficulty_class, true)
}

function resetDifficultyForm() {
  difficultyEditingCode.value = ''
  difficultyForm.value = {
    code: '',
    name: '',
    description: '',
    enabled: true,
    sort_order: (difficultyRows.value.length + 1) * 10,
  }
}

function startDifficultyEdit(row: DifficultyGridRow) {
  difficultyEditingCode.value = row.code
  difficultyForm.value = {
    code: row.code,
    name: row.name,
    description: row.description,
    enabled: row.enabled,
    sort_order: row.sort_order,
  }
}

async function saveDifficultyClass() {
  error.value = ''
  notice.value = ''
  try {
    const payload = {
      code: difficultyForm.value.code.trim(),
      name: difficultyForm.value.name.trim(),
      description: difficultyForm.value.description.trim(),
      enabled: difficultyForm.value.enabled,
      sort_order: difficultyForm.value.sort_order,
    }
    if (difficultyEditingCode.value) {
      await assetWorkbenchApi.updateDifficultyClass(difficultyEditingCode.value, payload)
      notice.value = '难度档案已更新'
    } else {
      await assetWorkbenchApi.createDifficultyClass(payload)
      notice.value = '难度档案已创建'
    }
    resetDifficultyForm()
    await loadCostCenter()
  } catch (err) {
    error.value = costCenterError(err, '难度分类保存失败')
  }
}

async function toggleDifficultyClass(row: DifficultyGridRow) {
  error.value = ''
  notice.value = ''
  try {
    await assetWorkbenchApi.updateDifficultyClass(row.code, { enabled: !row.enabled })
    notice.value = '难度状态已更新'
    await loadCostCenter()
  } catch (err) {
    error.value = costCenterError(err, '难度状态更新失败')
  }
}

function parseCSVNumbers(raw: string) {
  return raw
    .split(/[,\n，\s]+/)
    .map((item) => Number(item.trim()))
    .filter((item) => Number.isFinite(item) && item > 0)
}

function parseCSVStrings(raw: string) {
  return raw
    .split(/[,\n，\s]+/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function priceRuleIdentity(row: Pick<PriceMatrixRow, 'worker_type' | 'job_grade' | 'difficulty_class'>) {
  return `${workerTypeMeta(row.worker_type).label} / ${row.job_grade} / ${row.difficulty_class}`
}

function costCenterError(err: unknown, fallback: string) {
  return resolveApiUserMessage(err, { fallback })
}

function submitPriceRuleAction() {
  if (priceSupersedeId.value) {
    openPriceSupersedeConfirm()
    return
  }
  void createPriceRule()
}

function openPriceSupersedeConfirm() {
  error.value = ''
  if (!priceSupersedeSource.value) {
    error.value = '请先选择要修改的单价规则'
    return
  }
  if (!priceForm.value.effective_from) {
    error.value = '请先填写新版生效日'
    return
  }
  if (!Number.isFinite(priceForm.value.unit_price) || priceForm.value.unit_price < 0) {
    error.value = '请填写有效的新单价'
    return
  }
  confirmingPriceSupersede.value = true
}

function cancelPriceSupersede() {
  priceSupersedeId.value = 0
  confirmingPriceSupersede.value = false
}

async function createPriceRule() {
  error.value = ''
  notice.value = ''
  const supersedeSource = priceSupersedeSource.value
  const payload = {
    ...priceForm.value,
    remark: priceForm.value.remark.trim() || undefined,
    effective_from: startOfShanghaiDay(priceForm.value.effective_from),
    effective_to: endOfShanghaiDay(priceForm.value.effective_to),
  }
  try {
    if (priceSupersedeId.value) {
      await assetWorkbenchApi.supersedePriceMatrix(priceSupersedeId.value, payload)
      priceSupersedeId.value = 0
      confirmingPriceSupersede.value = false
      notice.value = supersedeSource
        ? `${priceRuleIdentity(supersedeSource)} 已完成改价，旧规则自动截止到 ${priceSupersedeCutoff.value}`
        : '价格修改已发布，旧规则会自动截止到新版生效日前一天'
    } else {
      await assetWorkbenchApi.createPriceMatrix(payload)
      notice.value = '单价规则已创建'
    }
    await loadCostCenter()
  } catch (err) {
    error.value = costCenterError(err, '单价规则创建失败')
  }
}

async function createDeductionRule() {
  error.value = ''
  notice.value = ''
  const payload = {
    ...deductionForm.value,
    effective_from: startOfShanghaiDay(deductionForm.value.effective_from),
    effective_to: endOfShanghaiDay(deductionForm.value.effective_to),
  }
  try {
    if (deductionSupersedeId.value) {
      await assetWorkbenchApi.supersedeDeductionRule(deductionSupersedeId.value, payload)
      deductionSupersedeId.value = 0
      notice.value = '质检扣款已创建替代版本'
    } else {
      await assetWorkbenchApi.createDeductionRule(payload)
      notice.value = '质检扣款已创建'
    }
    await loadCostCenter()
  } catch (err) {
    error.value = costCenterError(err, '质检扣款创建失败')
  }
}

async function createWelfareRule() {
  error.value = ''
  notice.value = ''
  const payload = {
    ...welfareForm.value,
    config_json: { mode: 'manual_monthly' },
    effective_from: startOfShanghaiDay(welfareForm.value.effective_from),
    effective_to: endOfShanghaiDay(welfareForm.value.effective_to),
  }
  try {
    if (welfareSupersedeId.value) {
      await assetWorkbenchApi.supersedeWelfareRule(welfareSupersedeId.value, payload)
      welfareSupersedeId.value = 0
      notice.value = '福利规则已创建替代版本'
    } else {
      await assetWorkbenchApi.createWelfareRule(payload)
      notice.value = '福利规则已创建'
    }
    await loadCostCenter()
  } catch (err) {
    error.value = costCenterError(err, '福利补贴创建失败')
  }
}

async function createPromoCoupon() {
  error.value = ''
  notice.value = ''
  const payload = {
    ...promoForm.value,
    amount: promoForm.value.mode === 'markup_rate' ? undefined : promoForm.value.amount,
    percent: promoForm.value.mode === 'markup_rate' ? promoForm.value.percent : undefined,
    eligible_user_ids_json: parseCSVNumbers(promoForm.value.eligible_user_ids),
    eligible_codes_json: parseCSVStrings(promoForm.value.eligible_codes),
    effective_from: startOfShanghaiDay(promoForm.value.effective_from),
    effective_to: endOfShanghaiDay(promoForm.value.effective_to),
  }
  try {
    if (promoSupersedeId.value) {
      await assetWorkbenchApi.supersedePromoCoupon(promoSupersedeId.value, payload)
      promoSupersedeId.value = 0
      notice.value = '临时活动价已创建替代版本'
    } else {
      await assetWorkbenchApi.createPromoCoupon(payload)
      notice.value = '临时活动价已创建'
    }
    await loadCostCenter()
  } catch (err) {
    error.value = costCenterError(err, '临时活动价创建失败')
  }
}

async function togglePriceRule(row: PriceGridRow) {
  await toggleRule(() => assetWorkbenchApi.updatePriceMatrix(row.id, { enabled: !row.enabled, reason: row.enabled ? '停用单价规则' : '启用单价规则' }))
}

async function requestTogglePriceRule(row: PriceGridRow) {
  if (row.enabled) {
    pendingPriceToggle.value = row
    notice.value = ''
    error.value = ''
    return
  }
  await togglePriceRule(row)
}

async function confirmPriceToggle() {
  const row = pendingPriceToggle.value
  if (!row) return
  pendingPriceToggle.value = null
  await togglePriceRule(row)
}

async function toggleDeductionRule(row: DeductionGridRow) {
  await toggleRule(() => assetWorkbenchApi.updateDeductionRule(row.id, { enabled: !row.enabled, reason: row.enabled ? '停用质检扣款' : '启用质检扣款' }))
}

async function toggleWelfareRule(row: WelfareGridRow) {
  await toggleRule(() => assetWorkbenchApi.updateWelfareRule(row.id, { enabled: !row.enabled, reason: row.enabled ? '停用福利规则' : '启用福利规则' }))
}

async function togglePromoRule(row: PromoGridRow) {
  await toggleRule(() => assetWorkbenchApi.updatePromoCoupon(row.id, { enabled: !row.enabled, reason: row.enabled ? '停用临时活动价' : '启用临时活动价' }))
}

async function toggleRule(action: () => Promise<unknown>) {
  error.value = ''
  notice.value = ''
  try {
    await action()
    notice.value = '规则状态已更新'
    await loadCostCenter()
  } catch (err) {
    error.value = costCenterError(err, '状态更新失败')
  }
}

function startPriceSupersede(row: PriceGridRow) {
  priceSupersedeId.value = row.id
  confirmingPriceSupersede.value = false
  priceForm.value = {
    worker_type: row.worker_type,
    job_grade: row.job_grade,
    difficulty_class: row.difficulty_class,
    unit_price: row.unit_price,
    effective_from: nextPriceVersionEffectiveFrom(row),
    effective_to: '',
    remark: '',
  }
  notice.value = `已进入 ${priceRuleIdentity(row)} 的改价流程，请填写新单价和生效日后预览发布`
}

function startDeductionSupersede(row: DeductionGridRow) {
  deductionSupersedeId.value = row.id
  deductionForm.value = {
    worker_type: row.worker_type,
    job_grade: row.job_grade,
    difficulty_class: row.difficulty_class,
    deduction_amount: row.deduction_amount,
    effective_from: toDateInput(row.effective_from) || today,
    effective_to: toDateInput(row.effective_to),
  }
  notice.value = `已带入质检扣款 ${row.id}，保存后会生成替代版本`
}

function startWelfareSupersede(row: WelfareGridRow) {
  welfareSupersedeId.value = row.id
  welfareForm.value = {
    rule_name: row.rule_name,
    worker_type: row.worker_type,
    job_grade: row.job_grade,
    rule_type: row.rule_type,
    amount: row.amount,
    effective_from: toDateInput(row.effective_from) || today,
    effective_to: toDateInput(row.effective_to),
  }
  notice.value = `已带入福利规则 ${row.id}，保存后会生成替代版本`
}

function startPromoSupersede(row: PromoGridRow) {
  promoSupersedeId.value = row.id
  promoForm.value = {
    coupon_code: row.coupon_code,
    coupon_name: row.coupon_name,
    mode: row.mode,
    amount: row.amount ?? 0,
    percent: row.percent ?? 0,
    priority: row.priority,
    worker_type: row.worker_type,
    job_grade: row.job_grade,
    difficulty_class: row.difficulty_class,
    eligible_user_ids: '',
    eligible_codes: '',
    effective_from: toDateInput(row.effective_from) || today,
    effective_to: toDateInput(row.effective_to),
  }
  notice.value = `已带入临时活动价 ${row.id}，保存后会生成替代版本`
}

onMounted(async () => {
  await loadCostCenter()
})
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">设置</p>
        <h2>{{ pageLabel }}</h2>
        <p>{{ pageSubtitle }}。普通配置只需要先维护「难度分类」，再维护「单价设置」；质检扣款、福利补贴、临时活动价属于结算调整，不需要每天处理。</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-primary-button" type="button" @click="activePricingSection = 'price'">进入单价设置</button>
      </div>
    </div>
    <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
    <p v-if="error" class="aw-inline-alert aw-inline-alert--error">{{ error }}</p>

    <div class="aw-pricing-settings">
      <aside class="aw-pricing-nav" aria-label="计价设置导航">
        <div class="aw-pricing-nav__head">
          <strong>计价设置</strong>
          <span>按顺序配置，不要从底层表格开始</span>
        </div>
        <div v-for="group in pricingNavGroups" :key="group.label" class="aw-pricing-nav__group">
          <p>{{ group.label }}</p>
          <button
            v-for="item in group.items"
            :key="item.key"
            class="aw-pricing-nav__item"
            :class="{ 'is-active': activePricingSection === item.key }"
            type="button"
            :aria-current="activePricingSection === item.key ? 'page' : undefined"
            @click="activePricingSection = item.key"
          >
            <span>
              <strong>{{ item.title }}</strong>
              <small>{{ item.meta }}</small>
            </span>
            <em>{{ item.count }}</em>
          </button>
        </div>
      </aside>

      <main class="aw-pricing-workspace">
        <section v-if="activePricingSection === 'guide'" class="aw-panel aw-pricing-guide">
          <div class="aw-panel__head">
            <div>
              <h3>先按这 3 步设置</h3>
              <p class="aw-copy">这里不要求理解系统字段，只要按业务顺序维护即可。单价会在作品提交时自动匹配，结算时按已匹配的价格计算。</p>
            </div>
          </div>
          <div class="aw-pricing-steps" aria-label="推荐设置顺序">
            <button type="button" @click="activePricingSection = 'difficulty'">
              <span>第一步</span>
              <strong>定义难度分类</strong>
              <small>例如 A/B/C 或专项分类。上传目录会绑定这个分类，后面单价也按它匹配。</small>
            </button>
            <button type="button" @click="activePricingSection = 'price'">
              <span>第二步</span>
              <strong>维护每类作品单价</strong>
              <small>选择人员类型、岗级、难度、单价和生效日。日常改价只改这一块。</small>
            </button>
            <button type="button" @click="activePricingSection = 'deduction'">
              <span>第三步</span>
              <strong>按需开启结算调整</strong>
              <small>质检扣款、福利补贴、临时活动价只在结算侧使用，不影响日常上传。</small>
            </button>
          </div>
          <div class="aw-pricing-explain">
            <div>
              <strong>普通管理员每天看哪里</strong>
              <p>只看「单价设置」。需要新增上传目录分类时，再回到「难度分类」。</p>
            </div>
            <div>
              <strong>什么不会重算</strong>
              <p>已提交、已结算的历史记录会保留当时价格快照；新单价只影响生效日之后的新提交。</p>
            </div>
          </div>
        </section>

        <div v-if="activePricingSection === 'difficulty'" class="aw-panel aw-pricing-section">
      <div class="aw-panel__head">
        <div>
          <h3>难度分类</h3>
          <p class="aw-copy">先把作品难度分清楚。上传目录会绑定这个分类，单价设置也会按这个分类匹配。</p>
        </div>
        <span class="aw-chip aw-chip--neutral">{{ difficultyRows.length }} 项</span>
      </div>
      <div class="aw-pricing-field-help">
        <span><b>代码</b>：系统识别用，创建后不要随意改</span>
        <span><b>名称</b>：给管理员看的显示名</span>
        <span><b>启用</b>：停用后不会再给新配置选择</span>
      </div>
      <div class="aw-form-grid">
        <label>
          代码
          <input v-model.trim="difficultyForm.code" :disabled="Boolean(difficultyEditingCode)" placeholder="例如 D 或 A+专项" />
        </label>
        <label>
          名称
          <input v-model.trim="difficultyForm.name" placeholder="例如 D类" />
        </label>
        <label>
          排序
          <input v-model.number="difficultyForm.sort_order" type="number" />
        </label>
        <div class="aw-field aw-toggle-field">
          <span>启用状态</span>
          <div class="aw-toggle-control">
            <button
              class="aw-switch-button"
              :class="{ 'is-on': difficultyForm.enabled }"
              type="button"
              role="switch"
              :aria-checked="difficultyForm.enabled"
              @click="difficultyForm.enabled = !difficultyForm.enabled"
            >
              <span aria-hidden="true"></span>
            </button>
            <div>
              <strong>{{ difficultyForm.enabled ? '启用' : '停用' }}</strong>
              <small>{{ difficultyForm.enabled ? '新上传目录可以选择这个分类' : '只保留历史记录，不再给新配置选择' }}</small>
            </div>
          </div>
        </div>
        <label class="aw-form-grid__full">
          说明
          <input v-model.trim="difficultyForm.description" placeholder="给管理员看的计价说明" />
        </label>
      </div>
      <div class="aw-inline-actions aw-form-action">
        <span v-if="difficultyEditingCode" class="aw-chip aw-chip--info">编辑 {{ difficultyEditingCode }}</span>
        <button v-if="difficultyEditingCode" class="aw-secondary-button" type="button" @click="resetDifficultyForm">取消编辑</button>
        <button class="aw-secondary-button" type="button" @click="saveDifficultyClass">{{ difficultyEditingCode ? '保存难度' : '新增难度' }}</button>
      </div>
      <WorkbenchDataGrid
        v-if="difficultyRows.length"
        :columns="difficultyGridColumns"
        :rows="difficultyGridRows"
        row-key="code"
        storage-key="cost-center-difficulties"
        :height="220"
        :row-height="34"
      >
        <template #cell="{ row, column, value }">
          <div v-if="column.key === 'action'" class="aw-grid-actions">
            <button type="button" @click="startDifficultyEdit(gridRowAsDifficulty(row))">编辑</button>
            <button type="button" @click="toggleDifficultyClass(gridRowAsDifficulty(row))">
              {{ gridRowAsDifficulty(row).enabled ? '停用' : '启用' }}
            </button>
          </div>
          <span v-else-if="column.key === 'enabled_label'" :class="chipClass(enabledMeta(gridRowAsDifficulty(row).enabled).tone)">{{ value }}</span>
          <span v-else>{{ value || '—' }}</span>
        </template>
      </WorkbenchDataGrid>
    </div>
    <div v-if="activePricingSection === 'price'" class="aw-panel aw-pricing-section">
      <div class="aw-panel__head">
        <div>
          <h3>单价设置</h3>
          <p class="aw-copy">设置「谁、做哪类作品、从哪天开始、每件多少钱」。日常改价只需要维护这里。</p>
        </div>
        <div class="aw-inline-actions">
          <span v-if="priceSupersedeId" class="aw-chip aw-chip--info">改价草稿 #{{ priceSupersedeId }}</span>
          <button v-if="priceSupersedeId" class="aw-secondary-button" type="button" @click="cancelPriceSupersede">取消改价</button>
          <button class="aw-secondary-button" type="button" @click="submitPriceRuleAction">{{ priceSupersedeId ? '预览并发布' : '新增' }}</button>
        </div>
      </div>
      <p v-if="priceSupersedeId" class="aw-inline-alert">
        正在修改价格：人员类型、岗级、难度已锁定，只调整新单价和生效日。旧规则会自动截止到新版生效日前一天，历史提交和已结算记录不会重算。
      </p>
      <div v-if="priceSupersedeSource" class="aw-price-change-card" aria-live="polite">
        <div class="aw-price-change-card__item">
          <span>当前单价规则</span>
          <strong>{{ priceRuleIdentity(priceSupersedeSource) }}</strong>
        </div>
        <div class="aw-price-change-card__item aw-price-change-card__item--money">
          <span>当前单价</span>
          <strong>{{ formatMoney(priceSupersedeSource.unit_price) }}</strong>
        </div>
        <div class="aw-price-change-card__item aw-price-change-card__item--money">
          <span>新单价</span>
          <strong>{{ formatMoney(priceForm.unit_price) }}</strong>
        </div>
        <div class="aw-price-change-card__item">
          <span>旧规则截止日</span>
          <strong>{{ priceSupersedeCutoff || '待填写' }}</strong>
        </div>
        <div class="aw-price-change-card__item aw-price-change-card__item--impact">
          <span>影响范围</span>
          <strong>从新版生效日之后的新提交开始使用；过往月度、历史提交、已结算记录保持原单价快照。</strong>
      </div>
      </div>
      <div class="aw-pricing-field-help">
        <span><b>人员类型</b>：全职或兼职</span>
        <span><b>岗级</b>：人员档案里的等级</span>
        <span><b>难度</b>：上传目录绑定的难度分类</span>
        <span><b>生效日</b>：从这一天之后的新提交开始使用新单价</span>
      </div>
      <div class="aw-form-grid">
        <label>
          类型
          <input v-if="priceSupersedeId" :value="workerTypeMeta(priceForm.worker_type).label" disabled />
          <select v-else v-model="priceForm.worker_type" @change="normalizeGradeForWorker(priceForm, false)">
            <option value="fulltime">全职</option>
            <option value="parttime">兼职</option>
          </select>
        </label>
        <label>
          岗级
          <input v-if="priceSupersedeId" :value="priceForm.job_grade" disabled />
          <select v-else v-model="priceForm.job_grade">
            <option v-for="grade in priceGradeOptions" :key="grade" :value="grade">{{ grade }}</option>
          </select>
        </label>
        <label>
          难度
          <input v-if="priceSupersedeId" :value="priceForm.difficulty_class" disabled />
          <select v-else v-model="priceForm.difficulty_class">
            <option v-for="difficulty in difficultyOptionCodes" :key="difficulty" :value="difficulty">{{ difficulty }}</option>
          </select>
        </label>
        <label>
          单价
          <input v-model.number="priceForm.unit_price" min="0" type="number" />
        </label>
        <label>
          生效日
          <input v-model="priceForm.effective_from" type="date" />
        </label>
        <label>
          失效日
          <input v-model="priceForm.effective_to" type="date" />
        </label>
        <label class="aw-form-grid__full">
          改价备注
          <textarea v-model="priceForm.remark" placeholder="例如：2026-07 兼职 J1 A 类价格维护"></textarea>
        </label>
      </div>
      <AsyncBoundary
        :loading="loading"
        :error="error"
        loading-label="正在加载单价设置"
        @retry="loadCostCenter"
      >
        <p class="aw-copy">已配置 {{ totals.price }} 条单价规则</p>
      </AsyncBoundary>
      <div class="aw-timeline-band">
        <span class="aw-timeline-band__past">过期</span>
        <span class="aw-timeline-band__active">生效中</span>
        <span class="aw-timeline-band__future">未来</span>
      </div>
      <WorkbenchDataGrid
        v-if="priceRows.length"
        :columns="priceGridColumns"
        :rows="priceGridRows"
        row-key="id"
        storage-key="cost-center-price"
        group-by="difficulty_class"
        :height="260"
        :row-height="34"
      >
        <template #cell="{ row, column, value }">
          <div v-if="column.key === 'action'" class="aw-grid-actions">
            <button class="aw-secondary-button" type="button" @click="startPriceSupersede(gridRowAsPrice(row))">修改价格</button>
            <button
              class="aw-secondary-button"
              :class="{ 'aw-secondary-button--danger': gridRowAsPrice(row).enabled }"
              type="button"
              :aria-label="`${gridRowAsPrice(row).enabled ? '停用' : '启用'}单价规则：${priceRuleIdentity(gridRowAsPrice(row))}`"
              @click="requestTogglePriceRule(gridRowAsPrice(row))"
            >
              {{ gridRowAsPrice(row).enabled ? '停用' : '启用' }}
            </button>
          </div>
          <span v-else-if="column.key === 'worker_type_label'" :class="chipClass(workerTypeMeta(gridRowAsPrice(row).worker_type).tone)">{{ value }}</span>
          <span v-else-if="column.key === 'enabled_label'" :class="chipClass(enabledMeta(gridRowAsPrice(row).enabled).tone)">{{ value }}</span>
          <span v-else-if="column.key === 'unit_price_label'" class="aw-cell-money">{{ value }}</span>
          <span v-else>{{ value || '—' }}</span>
        </template>
      </WorkbenchDataGrid>
      <div v-if="confirmingPriceSupersede && priceSupersedeSource" class="aw-dialog-backdrop" role="presentation" @click.self="confirmingPriceSupersede = false">
        <section class="aw-confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="price-supersede-confirm-title">
          <div>
            <p class="aw-eyebrow">改价确认</p>
            <h3 id="price-supersede-confirm-title">确认发布这次价格修改</h3>
            <p class="aw-copy">{{ priceSupersedeConfirmText }}</p>
          </div>
          <div class="aw-confirm-dialog__summary">
            <div>
              <span>当前单价</span>
              <strong>{{ formatMoney(priceSupersedeSource.unit_price) }}</strong>
            </div>
            <div>
              <span>新单价</span>
              <strong>{{ formatMoney(priceForm.unit_price) }}</strong>
            </div>
            <div>
              <span>旧规则自动截止</span>
              <strong>{{ priceSupersedeCutoff }}</strong>
            </div>
            <p>旧规则不需要手动停用；历史提交和已结算记录不会重算。</p>
          </div>
          <div class="aw-inline-actions">
            <button class="aw-secondary-button" type="button" @click="confirmingPriceSupersede = false">返回修改</button>
            <button class="aw-primary-button" type="button" @click="createPriceRule">确认发布</button>
          </div>
        </section>
      </div>
      <div v-if="pendingPriceToggle" class="aw-dialog-backdrop" role="presentation" @click.self="pendingPriceToggle = null">
        <section class="aw-confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="price-toggle-confirm-title">
          <div>
            <p class="aw-eyebrow">应急停用</p>
            <h3 id="price-toggle-confirm-title">确认停用这条单价规则</h3>
            <p class="aw-copy">
              {{ priceRuleIdentity(pendingPriceToggle) }} 停用后不再参与新提交计价；已按历史规则计算过的提交和结算不会重算。
            </p>
          </div>
          <div class="aw-inline-actions">
            <button class="aw-secondary-button" type="button" @click="pendingPriceToggle = null">取消</button>
            <button class="aw-secondary-button aw-secondary-button--danger" type="button" @click="confirmPriceToggle">确认停用</button>
          </div>
        </section>
      </div>
    </div>

        <div v-if="activePricingSection === 'deduction'" class="aw-panel aw-pricing-section">
        <div class="aw-panel__head">
          <div>
            <h3>质检扣款</h3>
            <p class="aw-copy">结算时读取质检错误记录，再按这里的金额扣款。日常上传不需要设置这一项。</p>
          </div>
        </div>
        <div class="aw-pricing-field-help">
          <span><b>人员类型/岗级</b>：可以选择全部，也可以只对某一类人员生效</span>
          <span><b>难度</b>：可以选择全部或指定难度分类</span>
          <span><b>每错扣款</b>：每条质检错误扣多少钱</span>
        </div>
        <div class="aw-form-grid">
          <label>
            类型
            <select v-model="deductionForm.worker_type" @change="normalizeGradeForWorker(deductionForm, true)">
              <option value="all">全部</option>
              <option value="fulltime">全职</option>
              <option value="parttime">兼职</option>
            </select>
          </label>
          <label>
            岗级
            <select v-model="deductionForm.job_grade">
              <option v-for="grade in deductionGradeOptions" :key="grade" :value="grade">{{ grade }}</option>
            </select>
          </label>
          <label>
            难度
            <select v-model="deductionForm.difficulty_class">
              <option v-for="difficulty in difficultyOptionCodesWithAll" :key="difficulty" :value="difficulty">{{ difficulty }}</option>
            </select>
          </label>
          <label>
            每错扣款
            <input v-model.number="deductionForm.deduction_amount" min="0" type="number" />
          </label>
          <label>
            生效日
            <input v-model="deductionForm.effective_from" type="date" />
          </label>
          <label>
            失效日
            <input v-model="deductionForm.effective_to" type="date" />
          </label>
        </div>
        <div class="aw-inline-actions aw-form-action">
          <span v-if="deductionSupersedeId" class="aw-chip aw-chip--info">替代 #{{ deductionSupersedeId }}</span>
          <button v-if="deductionSupersedeId" class="aw-secondary-button" type="button" @click="deductionSupersedeId = 0">取消替代</button>
          <button class="aw-secondary-button" type="button" @click="createDeductionRule">{{ deductionSupersedeId ? '保存替代' : '新增扣款规则' }}</button>
        </div>
        <WorkbenchDataGrid
          v-if="deductionRows.length"
          :columns="deductionGridColumns"
          :rows="deductionGridRows"
          row-key="id"
          storage-key="cost-center-deductions"
          group-by="difficulty_class"
          :height="220"
          :row-height="34"
        >
          <template #cell="{ row, column, value }">
            <div v-if="column.key === 'action'" class="aw-inline-actions">
              <button type="button" @click="toggleDeductionRule(gridRowAsDeduction(row))">{{ gridRowAsDeduction(row).enabled ? '停用' : '启用' }}</button>
              <button type="button" @click="startDeductionSupersede(gridRowAsDeduction(row))">替代</button>
            </div>
            <span v-else-if="column.key === 'worker_type_label'" :class="chipClass(workerTypeMeta(gridRowAsDeduction(row).worker_type).tone)">{{ value }}</span>
            <span v-else-if="column.key === 'enabled_label'" :class="chipClass(enabledMeta(gridRowAsDeduction(row).enabled).tone)">{{ value }}</span>
            <span v-else-if="column.key === 'deduction_amount_label'" class="aw-cell-money">{{ value }}</span>
            <span v-else>{{ value || '—' }}</span>
          </template>
        </WorkbenchDataGrid>
        <p v-else class="aw-copy">已配置 {{ totals.deduction }} 条质检扣款规则</p>
      </div>

        <div v-if="activePricingSection === 'welfare'" class="aw-panel aw-pricing-section">
        <div class="aw-panel__head">
          <div>
            <h3>福利补贴</h3>
            <p class="aw-copy">按人员和月份发放补贴，不归属到单个订单。没有固定补贴时可以不配置。</p>
          </div>
        </div>
        <div class="aw-pricing-field-help">
          <span><b>名称</b>：例如月度补贴、临时补贴</span>
          <span><b>人员类型/岗级</b>：决定哪些人可以拿到</span>
          <span><b>金额</b>：结算时增加到对应人员名下</span>
        </div>
        <div class="aw-form-grid">
          <label>
            名称
            <input v-model="welfareForm.rule_name" />
          </label>
          <label>
            类型
            <select v-model="welfareForm.worker_type" @change="normalizeGradeForWorker(welfareForm, true)">
              <option value="all">全部</option>
              <option value="fulltime">全职</option>
              <option value="parttime">兼职</option>
            </select>
          </label>
          <label>
            岗级
            <select v-model="welfareForm.job_grade">
              <option v-for="grade in welfareGradeOptions" :key="grade" :value="grade">{{ grade }}</option>
            </select>
          </label>
          <label>
            金额
            <input v-model.number="welfareForm.amount" min="0" type="number" />
          </label>
          <label>
            生效日
            <input v-model="welfareForm.effective_from" type="date" />
          </label>
          <label>
            失效日
            <input v-model="welfareForm.effective_to" type="date" />
          </label>
        </div>
        <div class="aw-inline-actions aw-form-action">
          <span v-if="welfareSupersedeId" class="aw-chip aw-chip--info">替代 #{{ welfareSupersedeId }}</span>
          <button v-if="welfareSupersedeId" class="aw-secondary-button" type="button" @click="welfareSupersedeId = 0">取消替代</button>
          <button class="aw-secondary-button" type="button" @click="createWelfareRule">{{ welfareSupersedeId ? '保存替代' : '新增福利' }}</button>
        </div>
        <WorkbenchDataGrid
          v-if="welfareRows.length"
          :columns="welfareGridColumns"
          :rows="welfareGridRows"
          row-key="id"
          storage-key="cost-center-welfare"
          group-by="worker_type"
          :height="220"
          :row-height="34"
        >
          <template #cell="{ row, column, value }">
            <div v-if="column.key === 'action'" class="aw-inline-actions">
              <button type="button" @click="toggleWelfareRule(gridRowAsWelfare(row))">{{ gridRowAsWelfare(row).enabled ? '停用' : '启用' }}</button>
              <button type="button" @click="startWelfareSupersede(gridRowAsWelfare(row))">替代</button>
            </div>
            <span v-else-if="column.key === 'worker_type_label'" :class="chipClass(workerTypeMeta(gridRowAsWelfare(row).worker_type).tone)">{{ value }}</span>
            <span v-else-if="column.key === 'enabled_label'" :class="chipClass(enabledMeta(gridRowAsWelfare(row).enabled).tone)">{{ value }}</span>
            <span v-else-if="column.key === 'amount_label'" class="aw-cell-money">{{ value }}</span>
            <span v-else>{{ value || '—' }}</span>
          </template>
        </WorkbenchDataGrid>
        <p v-else class="aw-copy">已配置 {{ totals.welfare }} 条福利补贴规则</p>
      </div>

        <div v-if="activePricingSection === 'promo'" class="aw-panel aw-pricing-section">
        <div class="aw-panel__head">
          <div>
            <h3>临时活动价</h3>
            <p class="aw-copy">用于临时活动、特殊订单或指定编码价格。没有活动价时可以不配置。</p>
          </div>
        </div>
        <div class="aw-pricing-field-help">
          <span><b>一口价</b>：直接覆盖为指定价格</span>
          <span><b>加价</b>：在原单价上增加固定金额</span>
          <span><b>涨幅</b>：按百分比调整</span>
          <span><b>生效编码/人员</b>：可限制只对指定订单或人员生效</span>
        </div>
        <div class="aw-form-grid">
          <label>
            编码
            <input v-model="promoForm.coupon_code" />
          </label>
          <label>
            名称
            <input v-model="promoForm.coupon_name" />
          </label>
          <label>
            模式
            <select v-model="promoForm.mode">
              <option value="fixed_price">一口价</option>
              <option value="markup_amount">加价</option>
              <option value="markup_rate">涨幅</option>
            </select>
          </label>
          <label>
            金额
            <input v-model.number="promoForm.amount" min="0" type="number" />
          </label>
          <label>
            涨幅 %
            <input v-model.number="promoForm.percent" min="0" type="number" />
          </label>
          <label>
            优先级
            <input v-model.number="promoForm.priority" min="1" type="number" />
          </label>
          <label>
            人员类型
            <select v-model="promoForm.worker_type" @change="normalizeGradeForWorker(promoForm, true)">
              <option value="all">全部</option>
              <option value="fulltime">全职</option>
              <option value="parttime">兼职</option>
            </select>
          </label>
          <label>
            岗级
            <select v-model="promoForm.job_grade">
              <option v-for="grade in promoGradeOptions" :key="grade" :value="grade">{{ grade }}</option>
            </select>
          </label>
          <label>
            难度
            <select v-model="promoForm.difficulty_class">
              <option v-for="difficulty in difficultyOptionCodesWithAll" :key="difficulty" :value="difficulty">{{ difficulty }}</option>
            </select>
          </label>
          <label>
            生效日
            <input v-model="promoForm.effective_from" type="date" />
          </label>
          <label>
            失效日
            <input v-model="promoForm.effective_to" type="date" />
          </label>
          <label class="aw-form-grid__full">
            生效人员 ID
            <input v-model="promoForm.eligible_user_ids" placeholder="多个 ID 用逗号、空格或换行分隔" />
          </label>
          <label class="aw-form-grid__full">
            生效编码
            <input v-model="promoForm.eligible_codes" placeholder="订单号或编码，多个用逗号、空格或换行分隔" />
          </label>
        </div>
        <div class="aw-inline-actions aw-form-action">
          <span v-if="promoSupersedeId" class="aw-chip aw-chip--info">替代 #{{ promoSupersedeId }}</span>
          <button v-if="promoSupersedeId" class="aw-secondary-button" type="button" @click="promoSupersedeId = 0">取消替代</button>
          <button class="aw-secondary-button" type="button" @click="createPromoCoupon">{{ promoSupersedeId ? '保存替代' : '新增活动价' }}</button>
        </div>
        <WorkbenchDataGrid
          v-if="promoRows.length"
          :columns="promoGridColumns"
          :rows="promoGridRows"
          row-key="id"
          storage-key="cost-center-promo"
          group-by="mode"
          :height="220"
          :row-height="34"
        >
          <template #cell="{ row, column, value }">
            <div v-if="column.key === 'action'" class="aw-inline-actions">
              <button type="button" @click="togglePromoRule(gridRowAsPromo(row))">{{ gridRowAsPromo(row).enabled ? '停用' : '启用' }}</button>
              <button type="button" @click="startPromoSupersede(gridRowAsPromo(row))">替代</button>
            </div>
            <span v-else-if="column.key === 'mode_label'" :class="chipClass(promoModeMeta(gridRowAsPromo(row).mode).tone)">{{ value }}</span>
            <span v-else-if="column.key === 'worker_type_label'" :class="chipClass(workerTypeMeta(gridRowAsPromo(row).worker_type).tone)">{{ value }}</span>
            <span v-else-if="column.key === 'enabled_label'" :class="chipClass(enabledMeta(gridRowAsPromo(row).enabled).tone)">{{ value }}</span>
            <span v-else-if="column.key === 'value_label'" class="aw-cell-money">{{ value }}</span>
            <span v-else>{{ value || '—' }}</span>
          </template>
        </WorkbenchDataGrid>
        <p v-else class="aw-copy">已配置 {{ totals.promo }} 条临时活动价</p>
        </div>
      </main>
    </div>
  </section>
</template>
