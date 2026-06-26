<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import {
  assetWorkbenchApi,
  type DeductionRuleRow,
  type PriceMatrixRow,
  type PromoCouponRow,
  type WelfareRuleRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { formatMoney, formatPercent } from '@aw/shared/format/number'
import { chipClass, enabledMeta, promoModeMeta, workerTypeMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'

interface GridColumn {
  key: string
  label: string
  width: number
  align?: 'left' | 'right' | 'center'
}

type PriceGridRow = PriceMatrixRow & { worker_type_label: string; unit_price_label: string; enabled_label: string }
type DeductionGridRow = DeductionRuleRow & { worker_type_label: string; deduction_amount_label: string; enabled_label: string }
type WelfareGridRow = WelfareRuleRow & { worker_type_label: string; amount_label: string; enabled_label: string }
type PromoGridRow = PromoCouponRow & { worker_type_label: string; mode_label: string; value_label: string; enabled_label: string }

const priceRows = ref<PriceMatrixRow[]>([])
const deductionRows = ref<DeductionRuleRow[]>([])
const welfareRows = ref<WelfareRuleRow[]>([])
const promoRows = ref<PromoCouponRow[]>([])
const totals = ref({
  price: 0,
  deduction: 0,
  welfare: 0,
  promo: 0,
})
const loading = ref(false)
const error = ref('')
const notice = ref('')
const today = new Date().toISOString().slice(0, 10)
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
  effective_from: today,
  effective_to: '',
})
const priceRowsWithLabels = computed<PriceGridRow[]>(() =>
  priceRows.value.map((row) => ({
    ...row,
    worker_type_label: workerTypeMeta(row.worker_type).label,
    unit_price_label: formatMoney(row.unit_price),
    enabled_label: enabledMeta(row.enabled).label,
  })),
)
const deductionRowsWithLabels = computed<DeductionGridRow[]>(() =>
  deductionRows.value.map((row) => ({
    ...row,
    worker_type_label: workerTypeMeta(row.worker_type).label,
    deduction_amount_label: formatMoney(row.deduction_amount),
    enabled_label: enabledMeta(row.enabled).label,
  })),
)
const welfareRowsWithLabels = computed<WelfareGridRow[]>(() =>
  welfareRows.value.map((row) => ({
    ...row,
    worker_type_label: workerTypeMeta(row.worker_type).label,
    amount_label: formatMoney(row.amount),
    enabled_label: enabledMeta(row.enabled).label,
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
  })),
)
const promoGridRows = computed(() => promoRowsWithLabels.value as unknown as Record<string, unknown>[])
const priceGridColumns = computed<GridColumn[]>(() => [
  { key: 'worker_type_label', label: '类型', width: 96 },
  { key: 'job_grade', label: '岗级', width: 96 },
  { key: 'difficulty_class', label: '难度', width: 132 },
  { key: 'unit_price_label', label: '单价', width: 104, align: 'right' },
  { key: 'enabled_label', label: '启用', width: 88 },
])
const deductionGridColumns = computed<GridColumn[]>(() => [
  { key: 'worker_type_label', label: '类型', width: 96 },
  { key: 'job_grade', label: '岗级', width: 96 },
  { key: 'difficulty_class', label: '难度', width: 120 },
  { key: 'deduction_amount_label', label: '每错扣减', width: 112, align: 'right' },
  { key: 'enabled_label', label: '启用', width: 88 },
])
const welfareGridColumns = computed<GridColumn[]>(() => [
  { key: 'rule_name', label: '名称', width: 140 },
  { key: 'worker_type_label', label: '类型', width: 96 },
  { key: 'job_grade', label: '岗级', width: 96 },
  { key: 'amount_label', label: '金额', width: 104, align: 'right' },
  { key: 'enabled_label', label: '启用', width: 88 },
])
const promoGridColumns = computed<GridColumn[]>(() => [
  { key: 'coupon_code', label: '编码', width: 120 },
  { key: 'mode_label', label: '模式', width: 120 },
  { key: 'worker_type_label', label: '类型', width: 96 },
  { key: 'priority', label: '优先级', width: 96, align: 'right' },
  { key: 'value_label', label: '值', width: 96, align: 'right' },
  { key: 'enabled_label', label: '启用', width: 88 },
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

async function loadCostCenter() {
  loading.value = true
  error.value = ''
  try {
    const [prices, deductions, welfare, promos] = await Promise.all([
      assetWorkbenchApi.listPriceMatrix({ page: 1, page_size: 20 }),
      assetWorkbenchApi.listDeductionRules({ page: 1, page_size: 20 }),
      assetWorkbenchApi.listWelfareRules({ page: 1, page_size: 20 }),
      assetWorkbenchApi.listPromoCoupons({ page: 1, page_size: 20 }),
    ])
    priceRows.value = prices.items
    deductionRows.value = deductions.items
    welfareRows.value = welfare.items
    promoRows.value = promos.items
    totals.value = {
      price: prices.total,
      deduction: deductions.total,
      welfare: welfare.total,
      promo: promos.total,
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '成本中心加载失败'
  } finally {
    loading.value = false
  }
}

function startOfShanghaiDay(date: string) {
  return date ? `${date}T00:00:00+08:00` : ''
}

function endOfShanghaiDay(date: string) {
  return date ? `${date}T23:59:59+08:00` : undefined
}

async function createPriceRule() {
  error.value = ''
  notice.value = ''
  try {
    await assetWorkbenchApi.createPriceMatrix({
      ...priceForm.value,
      effective_from: startOfShanghaiDay(priceForm.value.effective_from),
      effective_to: endOfShanghaiDay(priceForm.value.effective_to),
    })
    notice.value = '价目规则已创建'
    await loadCostCenter()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '价目规则创建失败'
  }
}

async function createDeductionRule() {
  error.value = ''
  notice.value = ''
  try {
    await assetWorkbenchApi.createDeductionRule({
      ...deductionForm.value,
      effective_from: startOfShanghaiDay(deductionForm.value.effective_from),
      effective_to: endOfShanghaiDay(deductionForm.value.effective_to),
    })
    notice.value = '扣减规则已创建'
    await loadCostCenter()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '扣减规则创建失败'
  }
}

async function createWelfareRule() {
  error.value = ''
  notice.value = ''
  try {
    await assetWorkbenchApi.createWelfareRule({
      ...welfareForm.value,
      config_json: { mode: 'manual_monthly' },
      effective_from: startOfShanghaiDay(welfareForm.value.effective_from),
      effective_to: endOfShanghaiDay(welfareForm.value.effective_to),
    })
    notice.value = '福利规则已创建'
    await loadCostCenter()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '福利规则创建失败'
  }
}

async function createPromoCoupon() {
  error.value = ''
  notice.value = ''
  try {
    await assetWorkbenchApi.createPromoCoupon({
      ...promoForm.value,
      amount: promoForm.value.mode === 'markup_rate' ? undefined : promoForm.value.amount,
      percent: promoForm.value.mode === 'markup_rate' ? promoForm.value.percent : undefined,
      effective_from: startOfShanghaiDay(promoForm.value.effective_from),
      effective_to: endOfShanghaiDay(promoForm.value.effective_to),
    })
    notice.value = '大促价格卷已创建'
    await loadCostCenter()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '大促价格卷创建失败'
  }
}

onMounted(async () => {
  await loadCostCenter()
})
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">计价规则</p>
        <h2>成本中心</h2>
        <p>集中维护价目矩阵、出错扣减、福利补贴与大促价格卷。提交只冻结毛额，扣减和福利在结算时计算。</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-primary-button" type="button" @click="createPriceRule">新增价目</button>
      </div>
    </div>
    <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
    <div class="aw-panel">
      <div class="aw-panel__head">
        <div>
          <h3>价目矩阵时间带</h3>
          <p class="aw-copy">按人员类型、岗级和难度命中单价。</p>
        </div>
        <button class="aw-secondary-button" type="button" @click="createPriceRule">新增</button>
      </div>
      <div class="aw-form-grid">
        <label>
          类型
          <select v-model="priceForm.worker_type">
            <option value="fulltime">全职</option>
            <option value="parttime">兼职</option>
            <option value="all">全部</option>
          </select>
        </label>
        <label>
          岗级
          <input v-model="priceForm.job_grade" />
        </label>
        <label>
          难度
          <input v-model="priceForm.difficulty_class" />
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
      <p class="aw-copy" v-if="loading">正在加载价目矩阵</p>
      <p class="aw-copy" v-else-if="error">{{ error }}</p>
      <p class="aw-copy" v-else>已配置 {{ totals.price }} 条价目规则</p>
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
          <span v-if="column.key === 'worker_type_label'" :class="chipClass(workerTypeMeta(gridRowAsPrice(row).worker_type).tone)">{{ value }}</span>
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
            <select v-model="deductionForm.worker_type">
              <option value="all">全部</option>
              <option value="fulltime">全职</option>
              <option value="parttime">兼职</option>
            </select>
          </label>
          <label>
            岗级
            <input v-model="deductionForm.job_grade" />
          </label>
          <label>
            难度
            <input v-model="deductionForm.difficulty_class" />
          </label>
          <label>
            每错扣减
            <input v-model.number="deductionForm.deduction_amount" min="0" type="number" />
          </label>
        </div>
        <button class="aw-secondary-button aw-form-action" type="button" @click="createDeductionRule">新增扣减</button>
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
            <span v-if="column.key === 'worker_type_label'" :class="chipClass(workerTypeMeta(gridRowAsDeduction(row).worker_type).tone)">{{ value }}</span>
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
            <select v-model="welfareForm.worker_type">
              <option value="all">全部</option>
              <option value="fulltime">全职</option>
              <option value="parttime">兼职</option>
            </select>
          </label>
          <label>
            岗级
            <input v-model="welfareForm.job_grade" />
          </label>
          <label>
            金额
            <input v-model.number="welfareForm.amount" min="0" type="number" />
          </label>
        </div>
        <button class="aw-secondary-button aw-form-action" type="button" @click="createWelfareRule">新增福利</button>
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
            <span v-if="column.key === 'worker_type_label'" :class="chipClass(workerTypeMeta(gridRowAsWelfare(row).worker_type).tone)">{{ value }}</span>
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
            <h3>大促价格卷</h3>
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
        </div>
        <button class="aw-secondary-button aw-form-action" type="button" @click="createPromoCoupon">新增大促券</button>
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
            <span v-if="column.key === 'mode_label'" :class="chipClass(promoModeMeta(gridRowAsPromo(row).mode).tone)">{{ value }}</span>
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
