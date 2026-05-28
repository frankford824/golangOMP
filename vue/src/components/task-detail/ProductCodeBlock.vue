<template>
  <section
    class="detail-block h-full flex flex-col rounded-lg border border-gray-200 bg-white shadow-sm p-6"
    :class="{ 'product-block--compact': compactLayout }"
  >
    <div class="block-header">
      <div class="flex items-center gap-2">
        <span class="block-icon">P</span>
        <h3 class="block-title">商品与编码信息</h3>
      </div>
    </div>

    <!-- 并列商品切换（运行态无主次；多条 sku_items 时展示） -->
    <div v-if="productRows.length > 1" class="product-tabs-wrap">
      <div class="product-tabs-scroll" role="tablist" aria-label="并列商品切换">
        <button
          v-for="row in productRows"
          :key="'ptab-' + row.index"
          type="button"
          role="tab"
          class="product-tab"
          :class="{ 'product-tab-active': row.index === safeProductIndex }"
          :aria-selected="row.index === safeProductIndex"
          :title="productTabTitle(row)"
          @click="setProductIndex(row.index)"
        >
          <span class="product-tab-main">{{ row.skuCode || row.label }}</span>
          <span class="product-tab-sub">{{ row.productName }}</span>
          <span class="product-tab-meta">{{ filingStatusLabel(row.filingStatus) }}</span>
        </button>
      </div>
    </div>

    <!-- 原品开发 -->
    <template v-if="isOriginal">
      <div v-if="task.productSource === 'existing'" class="product-thumb-row">
        <div
          class="product-thumb-wrap"
          :class="{ 'cursor-zoom-in': showImage && !imageLoadFailed }"
          @click="showImage && !imageLoadFailed ? (lightboxSrc = task.productImageUrl!) : null"
        >
          <img
            v-if="showImage && !imageLoadFailed"
            :src="task.productImageUrl"
            alt="原产品图"
            class="product-thumb-img"
            @error="onImageError"
          />
          <span v-else class="product-thumb-placeholder">无图</span>
        </div>
        <span class="product-thumb-label">原产品图</span>
      </div>
      <div v-if="lightboxSrc" class="lightbox-overlay" @click="lightboxSrc = null">
        <img :src="lightboxSrc" alt="原产品图大图" class="lightbox-img" />
      </div>
      <dl class="info-grid" :class="{ 'info-grid--compact': compactLayout }">
        <div class="info-row">
          <dt>SKU</dt>
          <dd class="font-mono text-sm">{{ currentRow.skuCode ?? '未绑定' }}</dd>
        </div>
        <div class="info-row">
          <dt>产品名称</dt>
          <dd>{{ currentRow.productName }}</dd>
        </div>
        <div class="info-row">
          <dt>分类</dt>
          <dd>{{ categoryLineOriginal }}</dd>
        </div>
      </dl>
      <div v-if="currentRow.designRequirement ?? task.designRequirement" class="req-box">
        <span class="req-label">修改要求</span>
        <p class="req-text">{{ currentRow.designRequirement ?? task.designRequirement }}</p>
      </div>
      <div v-if="currentRow.skuStatus" class="status-pill-row">
        <span class="status-pill-label">状态</span>
        <span class="status-pill">{{ skuStatusDisplay }}</span>
      </div>
    </template>

    <!-- 新品开发 -->
    <template v-else-if="isNewProduct">
      <div
        class="product-new-body"
        :class="{ 'product-new-body--compact': compactLayout, 'product-new-body--has-ref': showReferenceBlock }"
      >
      <div v-if="showReferenceBlock" class="main-sku-ref-block">
        <div class="main-ref-head">
          <span class="main-ref-label">参考图（{{ currentRow.label }}）</span>
          <span v-if="currentReferenceUrls.length > 1" class="main-ref-hint">缩略图切换</span>
        </div>
        <div v-if="currentReferenceUrls.length > 0" class="main-ref-hero-stack">
          <button
            type="button"
            class="main-ref-hero-btn"
            :class="{ 'main-ref-hero-btn--failed': activeRefUrl && refLoadFailedSet.has(activeRefUrl) }"
            :title="'放大查看参考图 ' + (activeRefIdx + 1)"
            @click="activeRefUrl && !refLoadFailedSet.has(activeRefUrl) && (lightboxSrc = activeRefUrl)"
          >
            <img
              v-if="activeRefUrl && !refLoadFailedSet.has(activeRefUrl)"
              :src="activeRefUrl"
              :alt="`参考图 ${activeRefIdx + 1}`"
              class="main-ref-hero-img"
              @error="onRefImageError(activeRefUrl!)"
            />
            <span v-else class="main-ref-hero-placeholder">参考图加载失败</span>
          </button>
          <div
            v-if="currentReferenceUrls.length > 1"
            class="main-ref-film-strip"
            role="tablist"
            aria-label="参考图切换"
          >
            <button
              v-for="(refUrl, i) in currentReferenceUrls"
              :key="'mstrip-' + i"
              type="button"
              role="tab"
              :aria-selected="i === activeRefIdx"
              class="main-ref-strip-btn"
              :class="{ 'main-ref-strip-btn-active': i === activeRefIdx }"
              @click="activeRefIdx = i"
            >
              <img
                v-if="!refLoadFailedSet.has(refUrl)"
                :src="refUrl"
                :alt="`参考 ${i + 1}`"
                class="main-ref-strip-img"
                @error="onRefImageError(refUrl)"
              />
              <span v-else class="main-ref-strip-placeholder">{{ i + 1 }}</span>
            </button>
          </div>
        </div>
        <p v-else class="main-ref-empty">暂无参考图</p>
      </div>
      <div v-if="lightboxSrc" class="lightbox-overlay" @click="lightboxSrc = null">
        <img :src="lightboxSrc" alt="参考图大图" class="lightbox-img" @click.stop />
      </div>
      <dl class="info-grid" :class="{ 'info-grid--compact': compactLayout }">
        <div class="info-row">
          <dt>SKU</dt>
          <dd class="font-mono text-sm">{{ currentRow.skuCode ?? '未绑定' }}</dd>
        </div>
        <div class="info-row">
          <dt>产品名称</dt>
          <dd>{{ currentRow.productName }}</dd>
        </div>
        <div v-if="currentRow.productShortName" class="info-row">
          <dt>产品简称</dt>
          <dd>{{ currentRow.productShortName }}</dd>
        </div>
        <div class="info-row">
          <dt>产品分类编码</dt>
          <dd class="text-sm">{{ categoryCodeDisplay }}</dd>
        </div>
        <div class="info-row">
          <dt>产品材质</dt>
          <dd>{{ materialLineDisplay }}</dd>
        </div>
      </dl>
      <div v-if="currentRow.designRequirement" class="req-box">
        <span class="req-label">设计需求说明</span>
        <p class="req-text">{{ currentRow.designRequirement }}</p>
      </div>
      <div v-if="task.productReferenceUrl" class="req-box mt-1">
        <span class="req-label">产品参考链接</span>
        <p class="req-text">
          <a :href="task.productReferenceUrl" class="link-ref" target="_blank" rel="noopener noreferrer">{{
            task.productReferenceUrl
          }}</a>
        </p>
      </div>
        <div v-if="currentRow.skuStatus" class="status-pill-row">
          <span class="status-pill-label">状态</span>
          <span class="status-pill">{{ skuStatusDisplay }}</span>
        </div>
        <div v-if="showSkuCostPanel" class="sku-cost-panel">
          <div class="sku-cost-head">
            <div>
              <span class="sku-cost-label">子项成本</span>
              <strong>{{ formatCostMoney(currentRow.costPrice) }}</strong>
            </div>
            <span class="sku-cost-mode">{{ skuCostModeLabel }}</span>
          </div>
          <div class="sku-cost-edit">
            <input
              v-model.number="skuCostDraft"
              type="number"
              min="0"
              step="0.001"
              class="sku-cost-input"
              placeholder="维护成本"
            />
            <button type="button" class="sku-cost-save" :disabled="savingSkuCost" @click="saveSkuCost">
              {{ savingSkuCost ? '保存中' : '保存成本' }}
            </button>
          </div>
          <p v-if="skuCostError" class="sku-cost-error">{{ skuCostError }}</p>
          <p v-else class="sku-cost-hint">
            {{ skuCostHint }}
          </p>
        </div>
      </div>
    </template>

    <!-- 采购任务 -->
    <template v-else-if="isPurchase">
      <dl class="info-grid" :class="{ 'info-grid--compact': compactLayout }">
        <div class="info-row">
          <dt>采购 SKU</dt>
          <dd class="font-mono text-sm">{{ currentRow.skuCode ?? '未绑定' }}</dd>
        </div>
        <div class="info-row">
          <dt>产品名称</dt>
          <dd>{{ currentRow.productName }}</dd>
        </div>
        <div v-if="task.productChannel" class="info-row">
          <dt>产品渠道（非必填）</dt>
          <dd>{{ task.productChannel }}</dd>
        </div>
        <div v-if="currentRow.quantity != null" class="info-row">
          <dt>数量</dt>
          <dd>{{ currentRow.quantity }}</dd>
        </div>
        <div v-if="currentRow.baseSalePrice != null" class="info-row">
          <dt>基本售价</dt>
          <dd>{{ currentRow.baseSalePrice }}</dd>
        </div>
        <div class="info-row">
          <dt>成本</dt>
          <dd>{{ formatCostMoney(currentRow.costPrice) }}</dd>
        </div>
      </dl>
      <div v-if="showSkuCostPanel" class="sku-cost-panel">
        <div class="sku-cost-head">
          <div>
            <span class="sku-cost-label">子项成本</span>
            <strong>{{ formatCostMoney(currentRow.costPrice) }}</strong>
          </div>
          <span class="sku-cost-mode">{{ skuCostModeLabel }}</span>
        </div>
        <div class="sku-cost-edit">
          <input
            v-model.number="skuCostDraft"
            type="number"
            min="0"
            step="0.001"
            class="sku-cost-input"
            placeholder="维护成本"
          />
          <button type="button" class="sku-cost-save" :disabled="savingSkuCost" @click="saveSkuCost">
            {{ savingSkuCost ? '保存中' : '保存成本' }}
          </button>
        </div>
        <p v-if="skuCostError" class="sku-cost-error">{{ skuCostError }}</p>
        <p v-else class="sku-cost-hint">
          {{ skuCostHint }}
        </p>
      </div>
      <div v-if="currentRow.skuStatus" class="status-pill-row">
        <span class="status-pill-label">状态</span>
        <span class="status-pill">{{ skuStatusDisplay }}</span>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { ref, computed, inject, watch } from 'vue'
import type { ComputedRef } from 'vue'
import type { Task } from '@/domain/types/task'
import type { ReferenceFileRef } from '@/services/api/assetsApi'
import { TASK_DETAIL_KEY } from '@/composables/task-detail-key'
import { TASK_DETAIL_PRODUCT_INDEX_KEY } from '@/composables/task-detail-product-index'
import { buildParallelProductRows, type TaskParallelProductRow } from '@/domain/task-parallel-products'
import { materialModeLabelCn, skuItemStatusLabelCn } from '@/domain/mappers/read-model-labels-cn'
import { useCategoryOptions } from '@/composables/useCategoryOptions'
import { useTasksStore } from '@/stores/tasks'
import { tasksApi } from '@/services/api/tasksApi'

withDefaults(
  defineProps<{
    /** 任务详情右栏：缩略参考 + 字段区并排，减轻 SKU～材质横向留白 */
    compactLayout?: boolean
  }>(),
  { compactLayout: false },
)

const injected = inject<ComputedRef<Task | null>>(TASK_DETAIL_KEY)
if (!injected) throw new Error('[ProductCodeBlock] 必须在 TaskDetailView 内使用')

const productCtx = inject(TASK_DETAIL_PRODUCT_INDEX_KEY, null)

const task = computed(() => injected.value!)
const lightboxSrc = ref<string | null>(null)
const imageLoadFailed = ref(false)
const showImage = computed(() => !!task.value?.productImageUrl)

const isOriginal = computed(
  () => task.value.businessType === 'ORIGINAL_PRODUCT_DEV' || task.value.taskType === 'ORIGINAL_PRODUCT_DEV',
)
const isNewProduct = computed(
  () => task.value.businessType === 'NEW_PRODUCT_DEV' || task.value.taskType === 'NEW_PRODUCT_DEV',
)
const isPurchase = computed(
  () => task.value.businessType === 'PURCHASE_TASK' || task.value.taskType === 'PURCHASE_TASK',
)

const productRows = computed(() => buildParallelProductRows(task.value))
const { options: categoryOptions } = useCategoryOptions()

const safeProductIndex = computed(() => {
  const ctxIdx = productCtx?.productIndex.value ?? 0
  const max = Math.max(0, productRows.value.length - 1)
  return Math.min(Math.max(0, ctxIdx), max)
})

const currentRow = computed(() => productRows.value[safeProductIndex.value] ?? productRows.value[0])

function setProductIndex(i: number) {
  productCtx?.setProductIndex(i)
}

watch(
  () => productRows.value.length,
  (n) => {
    if (productCtx && productCtx.productIndex.value >= n) {
      productCtx.setProductIndex(Math.max(0, n - 1))
    }
  },
)

const tasksStore = useTasksStore()
const skuCostDraft = ref<number | undefined>(undefined)
const savingSkuCost = ref(false)
const skuCostError = ref('')

const showReferenceBlock = computed(() => isNewProduct.value)
const currentReferenceRefs = computed((): ReferenceFileRef[] => currentRow.value?.referenceFileRefs ?? [])
const currentReferenceUrls = computed(() => currentReferenceRefs.value.map((r) => r.download_url ?? '').filter(Boolean))

const refLoadFailedSet = ref(new Set<string>())
const retriedRefIds = ref(new Set<string>())

function refKey(refObj: ReferenceFileRef): string {
  return refObj.asset_id ?? refObj.download_url ?? ''
}

function onRefImageError(url: string) {
  refLoadFailedSet.value.add(url)
  const refObj = currentReferenceRefs.value.find((r) => r.download_url === url)
  if (!refObj) return
  const key = refKey(refObj)
  if (!key || retriedRefIds.value.has(key)) return
  retriedRefIds.value.add(key)
  tasksStore.refreshReferenceUrls(task.value.id)
}

/** 与「设计工作台」一致：多参考图时主预览 + 底部缩略条切换 */
const activeRefIdx = ref(0)
const activeRefUrl = computed((): string | null => {
  const urls = currentReferenceUrls.value
  if (!urls.length) return null
  const i = Math.min(Math.max(0, activeRefIdx.value), urls.length - 1)
  return urls[i] ?? null
})

watch(
  () =>
    [task.value?.id, safeProductIndex.value, currentReferenceUrls.value.join('|')] as const,
  () => {
    activeRefIdx.value = 0
    refLoadFailedSet.value = new Set()
    retriedRefIds.value = new Set()
  },
)

watch(
  () => currentReferenceUrls.value.length,
  (n) => {
    if (activeRefIdx.value >= n) activeRefIdx.value = Math.max(0, n - 1)
  },
)

const categoryLineOriginal = computed(() => {
  const t = task.value
  const name = t.erpCategoryName?.trim()
  const code = t.erpCategoryCode?.trim()
  if (name && code) return `${name}（${code}）`
  if (name) return name
  if (code) return code
  return '—'
})

const categoryLabelByCode = computed(() => {
  const map = new Map<string, string>()
  for (const opt of categoryOptions.value) {
    const code = String(opt.value ?? '').trim()
    const label = String(opt.label ?? '').trim()
    if (!code || !label || code === '请选择分类') continue
    map.set(code, label)
  }
  return map
})

const categoryCodeDisplay = computed(() => {
  const code = currentRow.value.categoryCode?.trim()
  if (!code) return '—'
  const directTaskCode = task.value.newProductCategoryCode?.trim()
  const directTaskLabel =
    directTaskCode === code
      ? (task.value.categoryName?.trim() || task.value.category?.trim() || '')
      : ''
  const label = categoryLabelByCode.value.get(code)?.trim() || directTaskLabel
  if (!label || label === code) return code
  return `${label}（${code}）`
})

const skuStatusDisplay = computed(() => skuItemStatusLabelCn(currentRow.value.skuStatus))

const showSkuCostPanel = computed(() => (isNewProduct.value || isPurchase.value) && currentRow.value.id != null)
const skuCostModeLabel = computed(() => {
  if (currentRow.value.manualCostOverride === true || currentRow.value.costPriceMode === 'manual') return '手动维护'
  if (currentRow.value.requiresManualReview === true && currentRow.value.costPrice == null) return '缺尺寸，成本待维护'
  if (currentRow.value.requiresManualReview === true) return '需人工确认'
  if (currentRow.value.costPrice != null) return '系统计算'
  return '未生成'
})
const skuCostHint = computed(() => {
  if (currentRow.value.requiresManualReview === true && currentRow.value.costPrice == null) {
    return '缺尺寸，成本待维护；保存成本后该 SKU 会重新同步 ERP c_price。'
  }
  return `估算 ${formatCostMoney(currentRow.value.estimatedCost)}；保存后该 SKU 将按子项成本同步 ERP。`
})

function formatCostMoney(value: number | undefined): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '—'
  return `${value.toFixed(3)} CNY`
}

async function saveSkuCost() {
  const skuItemID = currentRow.value.id
  if (skuItemID == null) return
  const value = Number(skuCostDraft.value)
  if (!Number.isFinite(value) || value < 0) {
    skuCostError.value = '请输入有效成本'
    return
  }
  savingSkuCost.value = true
  skuCostError.value = ''
  try {
    await tasksApi.patchSkuItemCostInfo(task.value.id, skuItemID, {
      cost_price: value,
      manual_cost_override: true,
      manual_cost_override_reason: '仓库/运营手动维护子项成本',
      remark: `维护子项成本 ${currentRow.value.skuCode ?? ''}`.trim(),
    })
    await tasksStore.loadTaskById(task.value.id)
  } catch (err) {
    skuCostError.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    savingSkuCost.value = false
  }
}

const materialLineDisplay = computed(() =>
  materialModeLabelCn(currentRow.value.materialMode, task.value.newProductMaterialOther),
)

watch(
  () => [task.value?.id, safeProductIndex.value, currentRow.value.costPrice] as const,
  () => {
    skuCostDraft.value = currentRow.value.costPrice ?? currentRow.value.estimatedCost
    skuCostError.value = ''
  },
  { immediate: true },
)

function filingStatusLabel(status: string | undefined): string {
  const raw = String(status ?? '').trim()
  if (!raw) return '未建档'
  if (raw === 'filed') return '已建档'
  if (raw === 'filing') return '建档中'
  if (raw === 'pending_filing') return '待建档'
  if (raw === 'filing_failed') return '建档失败'
  return raw
}

function productTabTitle(row: TaskParallelProductRow): string {
  const sku = row.skuCode?.trim() || row.label
  const name = row.productName?.trim() || '—'
  const req = row.designRequirement?.trim() || '未填写设计需求'
  const filing = filingStatusLabel(row.filingStatus)
  return `${sku} · ${name} · ${filing} · ${req}`
}

function onImageError() {
  imageLoadFailed.value = true
}

watch(
  () => [task.value?.id, task.value?.productImageUrl] as const,
  () => {
    imageLoadFailed.value = false
  },
)
</script>

<style scoped>
.block-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.25rem;
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
.product-tabs-wrap {
  margin-bottom: 0.75rem;
}
.product-tabs-scroll {
  display: flex;
  flex-wrap: nowrap;
  gap: 0.375rem;
  overflow-x: auto;
  padding-bottom: 0.25rem;
  scrollbar-width: thin;
}
.product-tab {
  flex: 0 0 auto;
  min-width: 9.5rem;
  padding: 0.35rem 0.65rem 0.4rem;
  border-radius: 0.75rem;
  border: 1px solid rgb(226 232 240);
  background: rgb(248 250 252);
  color: rgb(71 85 105);
  cursor: pointer;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.1rem;
}
.product-tab:hover {
  border-color: rgb(203 213 225);
  color: rgb(30 41 59);
}
.product-tab-active {
  background: rgb(15 23 42);
  border-color: rgb(15 23 42);
  color: #fff;
}
.product-tab-main {
  font-size: 0.72rem;
  font-weight: 700;
  line-height: 1.1rem;
}
.product-tab-sub {
  max-width: 11.5rem;
  font-size: 0.7rem;
  line-height: 1.05rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  opacity: 0.95;
}
.product-tab-meta {
  font-size: 0.66rem;
  line-height: 1rem;
  opacity: 0.8;
}
.status-pill-row {
  margin-top: 0.5rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.8125rem;
}
.status-pill-label {
  color: rgb(100 116 139);
  font-weight: 600;
  font-size: 0.6875rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.status-pill {
  display: inline-flex;
  padding: 0.15rem 0.5rem;
  border-radius: 999px;
  background: rgb(241 245 249);
  color: rgb(51 65 85);
  font-size: 0.75rem;
}
.sku-cost-panel {
  margin-top: 0.75rem;
  border: 1px solid rgb(226 232 240);
  border-radius: 0.625rem;
  background: rgb(248 250 252);
  padding: 0.75rem;
}
.sku-cost-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  color: rgb(15 23 42);
}
.sku-cost-label,
.sku-cost-mode {
  display: block;
  font-size: 0.7rem;
  line-height: 1rem;
  color: rgb(100 116 139);
}
.sku-cost-mode {
  white-space: nowrap;
  border-radius: 999px;
  background: #fff;
  border: 1px solid rgb(226 232 240);
  padding: 0.1rem 0.45rem;
}
.sku-cost-edit {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.625rem;
}
.sku-cost-input {
  min-width: 0;
  flex: 1;
  height: 2rem;
  border: 1px solid rgb(203 213 225);
  border-radius: 0.5rem;
  background: #fff;
  padding: 0 0.625rem;
  font-size: 0.8125rem;
  outline: none;
}
.sku-cost-input:focus {
  border-color: rgb(37 99 235);
  box-shadow: 0 0 0 2px rgb(191 219 254);
}
.sku-cost-save {
  flex: 0 0 auto;
  height: 2rem;
  border: 1px solid rgb(37 99 235);
  border-radius: 0.5rem;
  background: rgb(37 99 235);
  color: #fff;
  padding: 0 0.75rem;
  font-size: 0.8125rem;
  font-weight: 600;
}
.sku-cost-save:disabled {
  opacity: 0.65;
  cursor: wait;
}
.sku-cost-hint,
.sku-cost-error {
  margin: 0.5rem 0 0;
  font-size: 0.72rem;
  line-height: 1.15rem;
}
.sku-cost-hint {
  color: rgb(100 116 139);
}
.sku-cost-error {
  color: rgb(220 38 38);
}
.info-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem 1.5rem;
  margin: 0 0 0.25rem;
}
.product-block--compact .info-grid,
.info-grid.info-grid--compact {
  grid-template-columns: minmax(min-content, max-content) minmax(0, 1fr);
  column-gap: 0.55rem;
  row-gap: 0.3rem;
}
.product-new-body {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
/* 任务详情紧凑模式：主预览高度与设计资产区接近 */
.product-new-body--compact.product-new-body--has-ref .main-ref-hero-img {
  max-height: min(32vh, 280px);
}
@media (min-width: 720px) {
  .product-new-body--compact.product-new-body--has-ref {
    flex-direction: row;
    flex-wrap: wrap;
    align-items: flex-start;
    gap: 0.65rem 1rem;
  }
  .product-new-body--compact.product-new-body--has-ref .main-sku-ref-block {
    flex: 0 0 min(40%, 17.5rem);
    min-width: 11rem;
    max-width: 19rem;
    width: 100%;
    margin-bottom: 0;
  }
  .product-new-body--compact.product-new-body--has-ref .info-grid {
    flex: 1 1 min(0, 28rem);
    max-width: none;
    margin-bottom: 0;
  }
}
.product-block--compact {
  padding: 1rem 1.125rem;
}
.product-block--compact .info-grid,
.product-block--compact .info-grid.info-grid--compact {
  gap: 0.45rem 0.85rem;
  max-width: 40rem;
}
.product-new-body--compact .req-box,
.product-new-body--compact .status-pill-row {
  flex: 1 1 100%;
  width: 100%;
}
.info-row {
  display: contents;
}
dt,
.field-label,
.row-label {
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
.req-box {
  background: rgb(248 250 252);
  border: 1px solid rgb(226 232 240);
  border-radius: 0.5rem;
  padding: 0.5rem 0.75rem;
}
.product-block--compact .req-box {
  padding: 0.45rem 0.6rem;
  max-width: 42rem;
}
.product-block--compact .req-label {
  margin-bottom: 0.2rem;
}
.product-block--compact .req-text {
  font-size: 0.8125rem;
  line-height: 1.45;
}
.req-label {
  font-size: 0.6875rem;
  font-weight: 600;
  color: rgb(100 116 139);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.req-text {
  font-size: 0.8125rem;
  color: rgb(51 65 85);
  margin: 0.25rem 0 0;
  white-space: pre-wrap;
}
.link-ref {
  color: rgb(37 99 235);
  word-break: break-all;
}

.main-sku-ref-block {
  margin-bottom: 0.75rem;
  padding: 0.5rem 0.625rem;
  border-radius: 0.5rem;
  border: 1px solid rgb(226 232 240);
  background: rgb(248 250 252);
}
.main-ref-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-bottom: 0.375rem;
}
.main-ref-label {
  font-size: 0.6875rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: rgb(100 116 139);
}
.main-ref-hint {
  font-size: 0.6875rem;
  font-weight: 500;
  color: rgb(129 140 248);
  white-space: nowrap;
}
.main-ref-hero-stack {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  min-width: 0;
}
.main-ref-hero-btn {
  display: block;
  width: 100%;
  padding: 0;
  min-height: 10rem;
  border: 2px solid rgb(203 213 225);
  border-radius: 0.75rem;
  background: rgb(248 250 252);
  cursor: zoom-in;
  overflow: hidden;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.main-ref-hero-btn:hover {
  border-color: rgb(147 197 253);
  box-shadow: 0 4px 14px rgb(59 130 246 / 0.12);
}
.main-ref-hero-img {
  width: 100%;
  height: 100%;
  min-height: 10rem;
  max-height: min(40vh, 360px);
  object-fit: contain;
  display: block;
  background: rgb(248 250 252);
}
.main-ref-hero-btn--failed {
  cursor: default;
}
.main-ref-hero-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 10rem;
  font-size: 0.8125rem;
  color: rgb(148 163 184);
}
.main-ref-film-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.main-ref-strip-btn {
  width: 3rem;
  height: 3rem;
  padding: 0;
  border-radius: 0.5rem;
  border: 2px solid rgb(226 232 240);
  background: #fff;
  cursor: pointer;
  overflow: hidden;
  transition: border-color 0.12s, box-shadow 0.12s;
  flex-shrink: 0;
}
.main-ref-strip-btn:hover {
  border-color: rgb(147 197 253);
}
.main-ref-strip-btn-active {
  border-color: rgb(79 70 229);
  box-shadow: 0 0 0 2px rgb(79 70 229 / 0.14);
}
.main-ref-strip-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.main-ref-strip-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  font-size: 0.625rem;
  color: rgb(148 163 184);
  background: rgb(241 245 249);
}
.main-ref-empty {
  margin: 0;
  font-size: 0.8125rem;
  color: rgb(148 163 184);
}

.product-thumb-row {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.25rem;
  margin-bottom: 0.75rem;
}
.product-thumb-wrap {
  width: 56px;
  height: 56px;
  border-radius: 6px;
  background: rgb(241 245 249);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  flex-shrink: 0;
}
.product-thumb-wrap.cursor-zoom-in {
  cursor: zoom-in;
}
.product-thumb-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.product-thumb-placeholder {
  font-size: 0.75rem;
  color: rgb(148 163 184);
}
.product-thumb-label {
  font-size: 0.6875rem;
  font-weight: 600;
  color: rgb(100 116 139);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.lightbox-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  cursor: zoom-out;
}
.lightbox-img {
  max-width: 90vw;
  max-height: 90vh;
  object-fit: contain;
  border-radius: 6px;
}
</style>
