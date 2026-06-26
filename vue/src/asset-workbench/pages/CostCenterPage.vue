<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import {
  assetWorkbenchApi,
  type DeductionRuleRow,
  type PriceMatrixRow,
  type PromoCouponRow,
  type WelfareRuleRow,
} from '@aw/shared/api/assetWorkbenchApi'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'

interface GridColumn {
  key: string
  label: string
  width: number
  align?: 'left' | 'right' | 'center'
}

type PromoGridRow = PromoCouponRow & { value_label: number | string }

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
const priceGridRows = computed(() => priceRows.value as unknown as Record<string, unknown>[])
const deductionGridRows = computed(() => deductionRows.value as unknown as Record<string, unknown>[])
const welfareGridRows = computed(() => welfareRows.value as unknown as Record<string, unknown>[])
const promoRowsWithLabels = computed<PromoGridRow[]>(() =>
  promoRows.value.map((row) => ({
    ...row,
    value_label: row.amount ?? row.percent ?? '',
  })),
)
const promoGridRows = computed(() => promoRowsWithLabels.value as unknown as Record<string, unknown>[])
const priceGridColumns = computed<GridColumn[]>(() => [
  { key: 'worker_type', label: '类型', width: 96 },
  { key: 'job_grade', label: '岗级', width: 96 },
  { key: 'difficulty_class', label: '难度', width: 132 },
  { key: 'unit_price', label: '单价', width: 96, align: 'right' },
  { key: 'enabled', label: '启用', width: 88 },
])
const deductionGridColumns = computed<GridColumn[]>(() => [
  { key: 'worker_type', label: '类型', width: 96 },
  { key: 'job_grade', label: '岗级', width: 96 },
  { key: 'difficulty_class', label: '难度', width: 120 },
  { key: 'deduction_amount', label: '每错扣减', width: 112, align: 'right' },
])
const welfareGridColumns = computed<GridColumn[]>(() => [
  { key: 'rule_name', label: '名称', width: 140 },
  { key: 'worker_type', label: '类型', width: 96 },
  { key: 'job_grade', label: '岗级', width: 96 },
  { key: 'amount', label: '金额', width: 96, align: 'right' },
])
const promoGridColumns = computed<GridColumn[]>(() => [
  { key: 'coupon_code', label: '编码', width: 120 },
  { key: 'mode', label: '模式', width: 120 },
  { key: 'priority', label: '优先级', width: 96, align: 'right' },
  { key: 'value_label', label: '值', width: 96, align: 'right' },
])

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
    <div class="aw-page-heading">
      <div>
        <p class="aw-eyebrow">Price matrix</p>
        <h2>成本中心</h2>
      </div>
      <button class="aw-primary-button" type="button" @click="createPriceRule">新增价目</button>
    </div>
    <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
    <div class="aw-panel">
      <h3>价目矩阵时间带</h3>
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
      <p class="aw-copy" v-else>当前返回 {{ totals.price }} 条价目规则</p>
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
        <template #cell="{ column, value }">
          <strong v-if="column.key === 'unit_price'">{{ value }}</strong>
          <span v-else>{{ value }}</span>
        </template>
      </WorkbenchDataGrid>
    </div>

    <div class="aw-three-column">
      <div class="aw-panel">
        <h3>出错扣减</h3>
        <p class="aw-copy">结算时读取出错 Excel 与扣减规则，提交时不冻结扣减结果。</p>
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
        <button class="aw-secondary-button" type="button" @click="createDeductionRule">新增扣减</button>
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
          <template #cell="{ column, value }">
            <strong v-if="column.key === 'deduction_amount'">{{ value }}</strong>
            <span v-else>{{ value }}</span>
          </template>
        </WorkbenchDataGrid>
        <p v-else class="aw-copy">当前返回 {{ totals.deduction }} 条扣减规则</p>
      </div>

      <div class="aw-panel">
        <h3>福利补贴</h3>
        <p class="aw-copy">福利按人月生成 settlement item，不挂在单条提交 item 上。</p>
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
        <button class="aw-secondary-button" type="button" @click="createWelfareRule">新增福利</button>
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
          <template #cell="{ column, value }">
            <strong v-if="column.key === 'amount'">{{ value }}</strong>
            <span v-else>{{ value }}</span>
          </template>
        </WorkbenchDataGrid>
        <p v-else class="aw-copy">当前返回 {{ totals.welfare }} 条福利规则</p>
      </div>

      <div class="aw-panel">
        <h3>大促价格卷</h3>
        <p class="aw-copy">v1 单券命中：一口价优先，其余按最高优先级选择，不叠加。</p>
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
        <button class="aw-secondary-button" type="button" @click="createPromoCoupon">新增大促券</button>
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
          <template #cell="{ column, value }">
            <strong v-if="column.key === 'value_label'">{{ value }}</strong>
            <span v-else>{{ value }}</span>
          </template>
        </WorkbenchDataGrid>
        <p v-else class="aw-copy">当前返回 {{ totals.promo }} 条大促券</p>
      </div>
    </div>
  </section>
</template>
