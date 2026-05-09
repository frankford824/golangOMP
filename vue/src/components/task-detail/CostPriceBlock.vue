<template>
  <!-- 仅新品开发 / 采购任务展示成本区：与创建 3-in-1 收集项一致；原品创建未收集成本字段故不展示 -->
  <section
    v-if="isNewProduct || isPurchase"
    class="detail-block h-full flex flex-col rounded-lg border border-gray-200 bg-white shadow-sm p-6"
    :class="{ 'cost-block--tiles': variant === 'tiles' }"
  >
    <div class="block-header">
      <div class="block-header-row">
        <span class="block-icon">C</span>
        <h3 class="block-title">成本与价格</h3>
        <span v-if="variant === 'tiles' && costTitleSuffix" class="cost-title-suffix">{{ costTitleSuffix }}</span>
      </div>
    </div>

    <!-- v6 磁贴：侧栏高密度展示（成本来源已并入标题，避免单独一行「—」） -->
    <div v-if="variant === 'tiles' && isNewProduct" class="cost-tiles">
      <div class="cost-tiles-row">
        <div class="cost-tile">
          <span class="cost-tile-label">成本单价</span>
          <span class="cost-tile-value">{{ formatMoneyShort(task.newProductCostUnitPrice) }}</span>
        </div>
        <div class="cost-tile">
          <span class="cost-tile-label">数量</span>
          <span class="cost-tile-value">{{ task.newProductQuantity != null ? String(task.newProductQuantity) : '—' }}</span>
        </div>
        <div class="cost-tile">
          <span class="cost-tile-label">基本售价</span>
          <span class="cost-tile-value">{{ formatMoneyShort(task.basePriceAmount) }}</span>
        </div>
      </div>
    </div>
    <div v-else-if="variant === 'tiles' && isPurchase" class="cost-tiles">
      <div class="cost-tiles-row cost-tiles-row--2">
        <div class="cost-tile">
          <span class="cost-tile-label">数量</span>
          <span class="cost-tile-value">{{ purchaseQuantityLine }}</span>
        </div>
        <div class="cost-tile">
          <span class="cost-tile-label">成本单价</span>
          <span class="cost-tile-value">{{ formatMoneyShort(task.costPrice?.amount) }}</span>
        </div>
        <div class="cost-tile">
          <span class="cost-tile-label">基本售价</span>
          <span class="cost-tile-value">{{ formatMoneyShort(task.basePriceAmount) }}</span>
        </div>
      </div>
    </div>

    <!-- 新品开发：与 TaskCreateNewProductForm 一致（只读） -->
    <dl v-else-if="isNewProduct" class="info-grid">
      <div class="info-row">
        <dt>成本来源</dt>
        <dd>{{ costModeLabel }}</dd>
      </div>
      <div class="info-row">
        <dt>成本单价</dt>
        <dd>{{ formatMoney(task.newProductCostUnitPrice) }}</dd>
      </div>
      <div class="info-row">
        <dt>数量（非必填）</dt>
        <dd>{{ task.newProductQuantity != null ? String(task.newProductQuantity) : '—' }}</dd>
      </div>
      <div class="info-row">
        <dt>基本售价（非必填）</dt>
        <dd>{{ formatMoney(task.basePriceAmount) }}</dd>
      </div>
    </dl>

    <!-- 采购任务：与 TaskCreatePurchaseForm 一致（只读） -->
    <dl v-else-if="isPurchase" class="info-grid">
      <div class="info-row">
        <dt>成本来源</dt>
        <dd>{{ costModeLabel }}</dd>
      </div>
      <div class="info-row">
        <dt>数量</dt>
        <dd>{{ purchaseQuantityLine }}</dd>
      </div>
      <div class="info-row">
        <dt>成本单价</dt>
        <dd>{{ formatMoney(task.costPrice?.amount) }}</dd>
      </div>
      <div class="info-row">
        <dt>基本售价</dt>
        <dd>{{ formatMoney(task.basePriceAmount) }}</dd>
      </div>
    </dl>
  </section>
</template>

<script setup lang="ts">
import { computed, inject } from 'vue'
import type { ComputedRef } from 'vue'
import type { Task } from '@/domain/types/task'
import { TASK_DETAIL_KEY } from '@/composables/task-detail-key'

withDefaults(
  defineProps<{
    /** v6 侧栏：磁贴排布，减少纵向占位 */
    variant?: 'card' | 'tiles'
  }>(),
  { variant: 'card' },
)

const injected = inject<ComputedRef<Task | null>>(TASK_DETAIL_KEY)
if (!injected) throw new Error('[CostPriceBlock] 必须在 TaskDetailView 内使用')

const task = computed(() => injected.value!)

const isNewProduct = computed(
  () => task.value.businessType === 'NEW_PRODUCT_DEV' || task.value.taskType === 'NEW_PRODUCT_DEV',
)
const isPurchase = computed(
  () => task.value.businessType === 'PURCHASE_TASK' || task.value.taskType === 'PURCHASE_TASK',
)

const costModeLabel = computed(() => {
  const m = task.value.costPriceMode
  if (m === 'manual') return '手动录入'
  if (m === 'template') return '按模板/系统计算'
  return '—'
})

/** 磁贴标题行内展示：无明确模式时不显示，避免出现孤立 em dash */
const costTitleSuffix = computed(() => {
  const m = task.value.costPriceMode
  if (m === 'manual') return '· 手动录入'
  if (m === 'template') return '· 按模板/系统计算'
  return ''
})

const purchaseQuantityLine = computed(() => {
  const q = task.value.purchaseInfo?.quantity
  if (q != null && Number.isFinite(Number(q))) return String(q)
  return '—'
})

function formatMoney(n: number | undefined | null): string {
  if (n == null || Number.isNaN(Number(n))) return '—'
  return `${Number(n)} CNY`
}

function formatMoneyShort(n: number | undefined | null): string {
  if (n == null || Number.isNaN(Number(n))) return '—'
  return `${Number(n)}`
}
</script>

<style scoped>
.block-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.75rem;
}
.block-header-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}
.cost-title-suffix {
  font-size: 0.6875rem;
  font-weight: 500;
  color: rgb(100 116 139);
  margin: 0;
}
.block-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.25rem;
  height: 1.25rem;
  border-radius: 0.375rem;
  background: rgb(248 250 252);
  color: rgb(148 163 184);
  font-size: 0.75rem;
  flex-shrink: 0;
}
.block-title {
  font-size: 0.875rem;
  font-weight: 600;
  color: rgb(30 41 59);
  margin: 0;
}
.info-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem 1.5rem;
  margin: 0;
}
.info-row {
  display: contents;
}
dt {
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgb(100 116 139);
}
dd {
  font-size: 0.875rem;
  color: rgb(15 23 42);
  margin: 0;
}

.cost-block--tiles {
  padding: 0.75rem 0.625rem 0.875rem;
}
.cost-block--tiles .block-header {
  margin-bottom: 0.5rem;
}
.cost-block--tiles .block-icon {
  background: rgb(241 245 249);
  color: rgb(100 116 139);
}
.cost-block--tiles .block-title {
  color: rgb(15 23 42);
  font-size: 0.8125rem;
}
.cost-tiles-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  justify-content: flex-start;
}
.cost-tiles-row--2 .cost-tile {
  flex: 0 0 auto;
  min-width: 4.25rem;
  max-width: 6.75rem;
}
.cost-tile {
  flex: 0 0 auto;
  min-width: 4.25rem;
  max-width: 6.75rem;
  padding: 0.4rem 0.45rem;
  border-radius: 0.5rem;
  background: rgb(248 250 252);
  border: 1px solid rgb(226 232 240);
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}
.cost-tile-label {
  font-size: 0.625rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: rgb(71 85 105);
}
.cost-tile-value {
  font-size: 0.8125rem;
  font-weight: 600;
  color: rgb(15 23 42);
  font-variant-numeric: tabular-nums;
}
</style>
