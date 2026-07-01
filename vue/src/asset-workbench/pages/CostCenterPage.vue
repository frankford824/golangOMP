<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import {
  assetWorkbenchApi,
  type DeductionRuleRow,
  type PriceMatrixRow,
  type PromoCouponRow,
  type WelfareRuleRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { useRoutePageCopy } from '@aw/app/useRoutePageCopy'
import { formatMoney, formatPercent } from '@aw/shared/format/number'
import { chipClass, enabledMeta, promoModeMeta, workerTypeMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'

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
type CostCenterData = Awaited<ReturnType<typeof fetchCostCenter>>

const difficultyOptions = ['A', 'B', 'C', 'A+小夜灯']
const difficultyOptionsWithAll = ['all', ...difficultyOptions]
const parttimeGrades = ['J1', 'J2', 'J3']
const fulltimeGrades = ['P1', 'P2', 'P3', 'P4', 'S1', 'S2', 'M1', 'M2']
const allGradeOptions = ['all']

const priceRows = ref<PriceMatrixRow[]>([])
const deductionRows = ref<DeductionRuleRow[]>([])
const welfareRows = ref<WelfareRuleRow[]>([])
const promoRows = ref<PromoCouponRow[]>([])
const priceSupersedeId = ref(0)
const deductionSupersedeId = ref(0)
const welfareSupersedeId = ref(0)
const promoSupersedeId = ref(0)
const totals = ref({
  price: 0,
  deduction: 0,
  welfare: 0,
  promo: 0,
})
const notice = ref('')
const { label: pageLabel, subtitle: pageSubtitle } = useRoutePageCopy('/settings/pricing')
const costCenterRequest = usePageRequest<CostCenterData>(fetchCostCenter, null, '成本中心加载失败')
const loading = costCenterRequest.loading
const error = costCenterRequest.error
const today = shanghaiDateInput()
const priceForm = ref({
  worker_type: 'fulltime',
  job_grade: 'P1',
  difficulty_class: 'A',
  unit_price: 0,
  effective_from: today,
  effective_to: '',
})
const deductionForm = ref({
  worker_type: 'all',
  job_grade: 'all',
  difficulty_class: 'A',
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
const priceRowsWithLabels = computed<PriceGridRow[]>(() =>
  priceRows.value.map((row) => ({
    ...row,
    worker_type_label: workerTypeMeta(row.worker_type).label,
    unit_price_label: formatMoney(row.unit_price),
    enabled_label: enabledMeta(row.enabled).label,
    action: 'actions',
  })),
)
const deductionRowsWithLabels = computed<DeductionGridRow[]>(() =>
  deductionRows.value.map((row) => ({
    ...row,
    worker_type_label: workerTypeMeta(row.worker_type).label,
    deduction_amount_label: formatMoney(row.deduction_amount),
    enabled_label: enabledMeta(row.enabled).label,
    action: 'actions',
  })),
)
const welfareRowsWithLabels = computed<WelfareGridRow[]>(() =>
  welfareRows.value.map((row) => ({
    ...row,
    worker_type_label: workerTypeMeta(row.worker_type).label,
    amount_label: formatMoney(row.amount),
    enabled_label: enabledMeta(row.enabled).label,
    action: 'actions',
  })),
)
const priceGridRows = computed(() => priceRowsWithLabels.value as unknown as Record<string, unknown>[])
const deductionGridRows = computed(() => deductionRowsWithLabels.value as unknown as Record<string, unknown>[])
const welfareGridRows = computed(() => welfareRowsWithLabels.value as unknown as Record<string, unknown>[])
const promoRowsWithLabels = computed<PromoGridRow[]>(() =>
  promoRows.value.map((row) => ({
    ...row,
    worker_type_label: workerTypeMeta(row.worker_type).label,
    mode_label: promoModeMeta(row.mode).label,
    value_label: row.mode === 'markup_rate' ? formatPercent(row.percent ?? 0) : formatMoney(row.amount ?? 0),
    enabled_label: enabledMeta(row.enabled).label,
    action: 'actions',
  })),
)
const promoGridRows = computed(() => promoRowsWithLabels.value as unknown as Record<string, unknown>[])
const priceGridColumns = computed<GridColumn[]>(() => [
  { key: 'worker_type_label', label: '类型', width: 96 },
  { key: 'job_grade', label: '岗级', width: 96 },
  { key: 'difficulty_class', label: '难度', width: 132 },
  { key: 'unit_price_label', label: '单价', width: 104, align: 'right' },
  { key: 'enabled_label', label: '状态', width: 88 },
  { key: 'action', label: '动作', width: 188, align: 'center' },
])
const deductionGridColumns = computed<GridColumn[]>(() => [
  { key: 'worker_type_label', label: '类型', width: 96 },
  { key: 'job_grade', label: '岗级', width: 96 },
  { key: 'difficulty_class', label: '难度', width: 120 },
  { key: 'deduction_amount_label', label: '每错扣减', width: 112, align: 'right' },
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

async function fetchCostCenter() {
  const [prices, deductions, welfare, promos] = await Promise.all([
    assetWorkbenchApi.listPriceMatrix({ page: 1, page_size: 20 }),
    assetWorkbenchApi.listDeductionRules({ page: 1, page_size: 20 }),
    assetWorkbenchApi.listWelfareRules({ page: 1, page_size: 20 }),
    assetWorkbenchApi.listPromoCoupons({ page: 1, page_size: 20 }),
  ])
  return { prices, deductions, welfare, promos }
}

async function loadCostCenter() {
  const data = await costCenterRequest.run()
  if (!data) return
  priceRows.value = data.prices.items
  deductionRows.value = data.deductions.items
  welfareRows.value = data.welfare.items
  promoRows.value = data.promos.items
  totals.value = {
    price: data.prices.total,
    deduction: data.deductions.total,
    welfare: data.welfare.total,
    promo: data.promos.total,
  }
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

async function createPriceRule() {
  error.value = ''
  notice.value = ''
  const payload = {
    ...priceForm.value,
    effective_from: startOfShanghaiDay(priceForm.value.effective_from),
    effective_to: endOfShanghaiDay(priceForm.value.effective_to),
  }
  try {
    if (priceSupersedeId.value) {
      await assetWorkbenchApi.supersedePriceMatrix(priceSupersedeId.value, payload)
      priceSupersedeId.value = 0
      notice.value = '价目新版已发布，旧规则会自动截止到新版生效日前一天'
    } else {
      await assetWorkbenchApi.createPriceMatrix(payload)
      notice.value = '价目规则已创建'
    }
    await loadCostCenter()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '价目规则创建失败'
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
      notice.value = '扣减规则已创建替代版本'
    } else {
      await assetWorkbenchApi.createDeductionRule(payload)
      notice.value = '扣减规则已创建'
    }
    await loadCostCenter()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '扣减规则创建失败'
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
    error.value = err instanceof Error ? err.message : '福利规则创建失败'
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
      notice.value = '大促价格券已创建替代版本'
    } else {
      await assetWorkbenchApi.createPromoCoupon(payload)
      notice.value = '大促价格券已创建'
    }
    await loadCostCenter()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '大促价格券创建失败'
  }
}

async function togglePriceRule(row: PriceGridRow) {
  await toggleRule(() => assetWorkbenchApi.updatePriceMatrix(row.id, { enabled: !row.enabled, reason: row.enabled ? '停用价目规则' : '启用价目规则' }))
}

async function toggleDeductionRule(row: DeductionGridRow) {
  await toggleRule(() => assetWorkbenchApi.updateDeductionRule(row.id, { enabled: !row.enabled, reason: row.enabled ? '停用扣减规则' : '启用扣减规则' }))
}

async function toggleWelfareRule(row: WelfareGridRow) {
  await toggleRule(() => assetWorkbenchApi.updateWelfareRule(row.id, { enabled: !row.enabled, reason: row.enabled ? '停用福利规则' : '启用福利规则' }))
}

async function togglePromoRule(row: PromoGridRow) {
  await toggleRule(() => assetWorkbenchApi.updatePromoCoupon(row.id, { enabled: !row.enabled, reason: row.enabled ? '停用大促价格券' : '启用大促价格券' }))
}

async function toggleRule(action: () => Promise<unknown>) {
  error.value = ''
  notice.value = ''
  try {
    await action()
    notice.value = '规则状态已更新'
    await loadCostCenter()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '规则状态更新失败'
  }
}

function startPriceSupersede(row: PriceGridRow) {
  priceSupersedeId.value = row.id
  priceForm.value = {
    worker_type: row.worker_type,
    job_grade: row.job_grade,
    difficulty_class: row.difficulty_class,
    unit_price: row.unit_price,
    effective_from: nextPriceVersionEffectiveFrom(row),
    effective_to: '',
  }
  notice.value = `已带入价目规则 ${row.id}，保存后会发布同一类型/岗级/难度的新版；旧规则自动截止到新版生效日前一天`
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
  notice.value = `已带入扣减规则 ${row.id}，保存后会生成替代版本`
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
  notice.value = `已带入大促价格券 ${row.id}，保存后会生成替代版本`
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
        <p>{{ pageSubtitle }}。集中维护价目矩阵、出错扣减、福利补贴与大促价格券。价目按生效日接班，提交记录保留当时的计价快照，扣减和福利在结算时计算。</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-primary-button" type="button" @click="createPriceRule">{{ priceSupersedeId ? '发布新版价目' : '新增价目' }}</button>
      </div>
    </div>
    <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
    <div class="aw-panel">
      <div class="aw-panel__head">
        <div>
          <h3>价目矩阵时间带</h3>
          <p class="aw-copy">按人员类型、岗级和难度命中单价；新版从生效日开始接班，旧版自动截止到前一天。</p>
        </div>
        <div class="aw-inline-actions">
          <span v-if="priceSupersedeId" class="aw-chip aw-chip--info">新版接棒 #{{ priceSupersedeId }}</span>
          <button v-if="priceSupersedeId" class="aw-secondary-button" type="button" @click="priceSupersedeId = 0">取消新版</button>
          <button class="aw-secondary-button" type="button" @click="createPriceRule">{{ priceSupersedeId ? '确认发布新版' : '新增' }}</button>
        </div>
      </div>
      <p v-if="priceSupersedeId" class="aw-inline-alert">
        正在发布新版价目：只允许调整同一类型、岗级、难度的单价与日期。旧规则会保留并自动截止到新版生效日前一天，不需要再手动停用；历史提交和已结算记录不会重算。
      </p>
      <div class="aw-form-grid">
        <label>
          类型
          <select v-model="priceForm.worker_type" @change="normalizeGradeForWorker(priceForm, false)">
            <option value="fulltime">全职</option>
            <option value="parttime">兼职</option>
          </select>
        </label>
        <label>
          岗级
          <select v-model="priceForm.job_grade">
            <option v-for="grade in priceGradeOptions" :key="grade" :value="grade">{{ grade }}</option>
          </select>
        </label>
        <label>
          难度
          <select v-model="priceForm.difficulty_class">
            <option v-for="difficulty in difficultyOptions" :key="difficulty" :value="difficulty">{{ difficulty }}</option>
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
      </div>
      <AsyncBoundary
        :loading="loading"
        :error="error"
        loading-label="正在加载价目矩阵"
        @retry="loadCostCenter"
      >
        <p class="aw-copy">已配置 {{ totals.price }} 条价目规则</p>
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
          <div v-if="column.key === 'action'" class="aw-inline-actions">
            <button type="button" @click="togglePriceRule(gridRowAsPrice(row))">{{ gridRowAsPrice(row).enabled ? '应急停用' : '启用' }}</button>
            <button type="button" @click="startPriceSupersede(gridRowAsPrice(row))">发布新版</button>
          </div>
          <span v-else-if="column.key === 'worker_type_label'" :class="chipClass(workerTypeMeta(gridRowAsPrice(row).worker_type).tone)">{{ value }}</span>
          <span v-else-if="column.key === 'enabled_label'" :class="chipClass(enabledMeta(gridRowAsPrice(row).enabled).tone)">{{ value }}</span>
          <span v-else-if="column.key === 'unit_price_label'" class="aw-cell-money">{{ value }}</span>
          <span v-else>{{ value || '—' }}</span>
        </template>
      </WorkbenchDataGrid>
    </div>

    <div class="aw-three-column">
      <div class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <h3>出错扣减</h3>
            <p class="aw-copy">结算时读取出错 Excel 与扣减规则，提交时不冻结扣减结果。</p>
          </div>
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
              <option v-for="difficulty in difficultyOptionsWithAll" :key="difficulty" :value="difficulty">{{ difficulty }}</option>
            </select>
          </label>
          <label>
            每错扣减
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
          <button class="aw-secondary-button" type="button" @click="createDeductionRule">{{ deductionSupersedeId ? '保存替代' : '新增扣减' }}</button>
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
        <p v-else class="aw-copy">已配置 {{ totals.deduction }} 条扣减规则</p>
      </div>

      <div class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <h3>福利补贴</h3>
            <p class="aw-copy">福利按人员和月份发放，不归属到单个订单。</p>
          </div>
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
        <p v-else class="aw-copy">已配置 {{ totals.welfare }} 条福利规则</p>
      </div>

      <div class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <h3>大促价格券</h3>
            <p class="aw-copy">同一订单只采用一条大促规则；一口价优先，其他按优先级选择。</p>
          </div>
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
              <option v-for="difficulty in difficultyOptionsWithAll" :key="difficulty" :value="difficulty">{{ difficulty }}</option>
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
          <button class="aw-secondary-button" type="button" @click="createPromoCoupon">{{ promoSupersedeId ? '保存替代' : '新增大促券' }}</button>
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
        <p v-else class="aw-copy">已配置 {{ totals.promo }} 条大促规则</p>
      </div>
    </div>
  </section>
</template>
