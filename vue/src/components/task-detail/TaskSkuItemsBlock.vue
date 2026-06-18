<!--
  @deprecated 任务详情已合并至 ProductCodeBlock（并列商品切换）。保留文件仅供历史对照，勿在新代码中引用。
-->
<template>
  <section
    class="detail-block h-full flex flex-col rounded-lg border border-gray-200 bg-white shadow-sm p-6"
  >
    <div class="block-header">
      <div class="flex items-center gap-2">
        <span class="block-icon">S</span>
        <h3 class="block-title">SKU 子项</h3>
      </div>
    </div>

    <!-- 当前查看子项面板（整张卡片主体） -->
    <div v-if="currentItem" class="current-panel">
      <div class="current-panel-header">
        <div class="current-title-wrap">
          <p class="current-title">当前子项</p>
          <p class="current-subtitle">
            子项 #{{ currentItem.sequenceNo ?? currentDisplayIndex }} · {{ currentItem.skuCode ?? '—' }}
          </p>
        </div>
        <div class="header-right">
          <span class="status-pill" :class="statusToneClass">{{ statusLabel }}</span>
          <div class="current-nav">
            <button
              type="button"
              class="nav-btn"
              :disabled="currentIndex <= 0"
              @click="step(-1)"
            >
              上一项
            </button>
            <span class="nav-indicator">{{ currentDisplayIndex }} / {{ totalItems }}</span>
            <button
              type="button"
              class="nav-btn"
              :disabled="currentIndex >= totalItems - 1"
              @click="step(1)"
            >
              下一项
            </button>
          </div>
        </div>
      </div>

      <!-- 轻量子项切换器：不展示长 SKU -->
      <div v-if="totalItems > 1" class="item-tabs" aria-label="SKU 子项切换">
        <button
          v-for="(item, idx) in items"
          :key="item.skuCode ?? `tab-${idx}`"
          type="button"
          class="item-tab"
          :class="{ 'item-tab-active': idx === currentIndex }"
          @click="setCurrentIndex(idx)"
        >
          子项 {{ item.sequenceNo ?? idx + 1 }}
        </button>
      </div>

      <div class="core-info">
        <div class="core-row">
          <span class="core-label">产品名称</span>
          <span class="core-value">{{ currentItem.productNameSnapshot ?? task.productName }}</span>
        </div>
        <div class="core-row">
          <span class="core-label">当前 SKU</span>
          <span class="core-value core-mono">{{ currentItem.skuCode ?? '—' }}</span>
        </div>
        <div class="core-row">
          <span class="core-label">当前状态</span>
          <span class="core-value">{{ statusLabel }}</span>
        </div>
      </div>

      <div v-if="attributeRows.length > 0" class="attr-grid">
        <div v-for="row in attributeRows" :key="row.key" class="attr-row">
          <span class="attr-label">{{ row.label }}</span>
          <span class="attr-value" :class="{ 'core-mono': row.mono }">{{ row.value }}</span>
        </div>
      </div>

      <div v-if="itemDesignRequirement" class="requirement-brief item-req-brief">
        <span class="requirement-label">本子项设计需求</span>
        <p class="requirement-text">{{ itemDesignRequirement }}</p>
      </div>

      <div v-if="designRequirementSummary && !itemDesignRequirement" class="requirement-brief">
        <span class="requirement-label">设计需求摘要</span>
        <p class="requirement-text">{{ designRequirementSummary }}</p>
      </div>

      <div class="sub-ref-section">
        <span class="sub-ref-label">当前子项参考图</span>
        <div v-if="subItemReferenceUrls.length > 0" class="sub-ref-thumb-grid">
          <button
            v-for="(ref, i) in subItemReferenceUrls"
            :key="'sref-' + i"
            type="button"
            class="sub-ref-thumb-btn"
            @click="openSubReferencePreview(i)"
          >
            <img :src="ref" :alt="`子项参考图 ${i + 1}`" class="sub-ref-thumb-img" @error="onSubRefImageError(ref)" />
          </button>
        </div>
        <p v-else class="sub-ref-empty">暂无参考图</p>
      </div>
    </div>

  </section>
</template>

<script setup lang="ts">
import { computed, inject, ref, watch } from 'vue'
import type { ComputedRef } from 'vue'
import type { Task, TaskSkuItem } from '@/domain/types/task'
import type { ReferenceFileRef } from '@/services/api/assetsApi'
import { TASK_DETAIL_KEY } from '@/composables/task-detail-key'
import { useTasksStore } from '@/stores/tasks'
import {
  IMAGE_PREVIEW_LIGHTBOX_KEY,
  type ImagePreviewLightboxItem,
  type OpenImagePreviewLightbox,
} from '@/components/media/imagePreviewLightbox'

const injected = inject<ComputedRef<Task | null>>(TASK_DETAIL_KEY)
if (!injected) throw new Error('[TaskSkuItemsBlock] 必须在 TaskDetailView 内使用')

const taskRef = injected
const openLightbox = inject<OpenImagePreviewLightbox>(IMAGE_PREVIEW_LIGHTBOX_KEY, () => {})
const tasksStore = useTasksStore()

const task = computed(() => taskRef.value!)
const items = computed<TaskSkuItem[]>(() => task.value.skuItems ?? [])
const totalItems = computed(() => items.value.length)

const currentIndex = ref(0)

const currentDisplayIndex = computed(() => (totalItems.value === 0 ? 0 : currentIndex.value + 1))

const currentItem = computed<TaskSkuItem | null>(() => {
  if (!totalItems.value) return null
  const idx = Math.min(Math.max(currentIndex.value, 0), totalItems.value - 1)
  return items.value[idx] ?? null
})

const statusLabel = computed(() => {
  const status = (currentItem.value?.skuStatus ?? '').trim()
  if (!status) return '未设置'
  return status
})

const statusToneClass = computed(() => {
  const raw = (currentItem.value?.skuStatus ?? '').toLowerCase()
  if (raw.includes('audit') || raw.includes('review') || raw.includes('待审核')) return 'pill-warning'
  if (raw.includes('warehouse') || raw.includes('入库')) return 'pill-success'
  if (raw.includes('progress') || raw.includes('设计中')) return 'pill-info'
  if (raw.includes('generated') || raw.includes('create') || raw.includes('生成')) return 'pill-default'
  return 'pill-default'
})

const designRequirementSummary = computed(() => {
  const text = (task.value.designRequirement ?? '').trim()
  if (!text) return ''
  if (text.length <= 72) return text
  return `${text.slice(0, 72)}...`
})

const itemDesignRequirement = computed(() => {
  const text = (currentItem.value?.designRequirement ?? '').trim()
  if (!text) return ''
  if (text.length <= 120) return text
  return `${text.slice(0, 120)}...`
})

const subItemReferenceRefs = computed((): ReferenceFileRef[] => currentItem.value?.referenceFileRefs ?? [])
const subItemReferenceUrls = computed(() =>
  subItemReferenceRefs.value.map((r) => r.download_url ?? '').filter(Boolean),
)
const subItemReferencePreviewItems = computed((): ImagePreviewLightboxItem[] =>
  subItemReferenceRefs.value
    .map((refObj, index) => {
      const src = String(refObj.download_url ?? '').trim()
      const title = refObj.filename?.trim() || `子项参考图 ${index + 1}`
      return src ? { src, title, alt: title, downloadUrl: src } : null
    })
    .filter((item) => item != null) as ImagePreviewLightboxItem[],
)
const retriedRefIds = ref(new Set<string>())

function onSubRefImageError(url: string) {
  const refObj = subItemReferenceRefs.value.find((r) => r.download_url === url)
  if (!refObj) return
  const key = refObj.asset_id ?? url
  if (retriedRefIds.value.has(key)) return
  retriedRefIds.value.add(key)
  tasksStore.refreshReferenceUrls(task.value.id)
}

function openSubReferencePreview(index: number) {
  const item = subItemReferencePreviewItems.value[index]
  if (!item?.src) return
  openLightbox(item.src, {
    title: item.title,
    items: subItemReferencePreviewItems.value,
    index,
  })
}

const attributeRows = computed(() => {
  const rows: Array<{ key: string; label: string; value: string; mono?: boolean }> = []
  const shortName = currentItem.value?.productShortName?.trim()
  if (shortName) rows.push({ key: 'short', label: '简称', value: shortName })

  const category = (
    currentItem.value?.categoryCode ??
    task.value.newProductCategoryCode ??
    task.value.erpCategoryCode ??
    ''
  ).trim()
  if (category) rows.push({ key: 'category', label: '分类编码', value: category, mono: true })

  const material = (currentItem.value?.materialMode ?? task.value.newProductMaterial ?? '').trim()
  if (material) rows.push({ key: 'material', label: '材质', value: material })

  const quantity = currentItem.value?.quantity != null ? String(currentItem.value.quantity) : ''
  const price = currentItem.value?.baseSalePrice != null ? String(currentItem.value.baseSalePrice) : ''
  if (quantity || price) {
    const parts = []
    if (quantity) parts.push(`数量 ${quantity}`)
    if (price) parts.push(`基本售价 ${price}`)
    rows.push({ key: 'qty_price', label: '数量/售价', value: parts.join(' · ') })
  }
  return rows
})

function setCurrentIndex(idx: number) {
  if (idx < 0 || idx >= totalItems.value) return
  currentIndex.value = idx
}

function step(delta: number) {
  const next = currentIndex.value + delta
  if (next < 0 || next >= totalItems.value) return
  currentIndex.value = next
}

watch(
  () => [task.value.id, totalItems.value] as const,
  () => {
    if (!totalItems.value) {
      currentIndex.value = 0
    } else if (currentIndex.value >= totalItems.value) {
      currentIndex.value = totalItems.value - 1
    }
    retriedRefIds.value = new Set()
  },
  { immediate: true },
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
.current-panel {
  margin-top: 0.75rem;
  padding: 0.875rem;
  border: 1px solid #e2e8f0;
  border-radius: 0.625rem;
  background: #f8fafc;
}
.current-panel-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.625rem;
}
.current-title-wrap {
  min-width: 0;
}
.current-title {
  margin: 0;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: #64748b;
}
.current-subtitle {
  margin: 0.25rem 0 0;
  font-size: 0.875rem;
  color: #0f172a;
  font-weight: 700;
  word-break: break-word;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.status-pill {
  padding: 0.125rem 0.45rem;
  border-radius: 999px;
  border: 1px solid #cbd5e1;
  background: #f1f5f9;
  color: #475569;
  font-size: 0.6875rem;
  font-weight: 600;
  white-space: nowrap;
}
.item-tabs {
  display: flex;
  gap: 0.375rem;
  margin: 0 0 0.625rem;
  overflow-x: auto;
  padding-bottom: 0.125rem;
}
.item-tab {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.25rem 0.55rem;
  border-radius: 999px;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
  font-size: 0.75rem;
  color: #475569;
  cursor: pointer;
  white-space: nowrap;
  font-weight: 500;
}
.item-tab-active {
  border-color: #93c5fd;
  background: #eff6ff;
  color: #1d4ed8;
}
.core-info {
  background: #fff;
  border: 1px solid #e6edf5;
  border-radius: 0.5rem;
  padding: 0.625rem 0.7rem;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}
.core-row {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
}
.core-label {
  min-width: 4.4rem;
  font-size: 0.75rem;
  color: #64748b;
  font-weight: 600;
}
.core-value {
  font-size: 0.8125rem;
  color: #0f172a;
}
.core-mono {
  font-family: var(--yb-font-data);
  font-size: 0.75rem;
}
.attr-grid {
  margin-top: 0.55rem;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.4rem 0.8rem;
}
.attr-row {
  display: flex;
  align-items: baseline;
  gap: 0.375rem;
}
.attr-label {
  font-size: 0.75rem;
  font-weight: 600;
  color: #64748b;
  flex-shrink: 0;
}
.attr-value {
  font-size: 0.8125rem;
  color: #1e293b;
  min-width: 0;
  word-break: break-word;
}
.pill-default {
  border-color: #cbd5e1;
  background: #f1f5f9;
  color: #475569;
}
.pill-info {
  border-color: #bfdbfe;
  background: #eff6ff;
  color: #1d4ed8;
}
.pill-warning {
  border-color: #fde68a;
  background: #fffbeb;
  color: #b45309;
}
.pill-success {
  border-color: #bbf7d0;
  background: #f0fdf4;
  color: #15803d;
}
.current-nav {
  display: flex;
  align-items: center;
  gap: 0.35rem;
}
.nav-btn {
  padding: 0.2rem 0.6rem;
  font-size: 0.75rem;
  border-radius: 999px;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
  color: #475569;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s, color 0.12s;
}
.nav-btn:hover:not(:disabled) {
  background: #eff6ff;
  border-color: #bfdbfe;
  color: #1d4ed8;
}
.nav-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.nav-indicator {
  font-size: 0.75rem;
  color: #64748b;
  min-width: 2.6rem;
  text-align: center;
}
.requirement-brief {
  margin-top: 0.625rem;
  padding-top: 0.5rem;
  border-top: 1px dashed #dbe2ea;
}
.requirement-label {
  font-size: 0.6875rem;
  font-weight: 600;
  color: #64748b;
}
.requirement-text {
  margin: 0.25rem 0 0;
  font-size: 0.75rem;
  color: #334155;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.item-req-brief .requirement-text {
  -webkit-line-clamp: 4;
  line-clamp: 4;
}
.sub-ref-section {
  margin-top: 0.625rem;
  padding-top: 0.5rem;
  border-top: 1px dashed #dbe2ea;
}
.sub-ref-label {
  display: block;
  font-size: 0.6875rem;
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-bottom: 0.375rem;
}
.sub-ref-thumb-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(88px, 1fr));
  gap: 0.45rem;
}
.sub-ref-thumb-btn {
  display: block;
  padding: 0;
  border: none;
  background: none;
  cursor: zoom-in;
  border-radius: 6px;
  overflow: hidden;
}
.sub-ref-thumb-img {
  width: 100%;
  aspect-ratio: 1;
  object-fit: cover;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
  background: #fff;
  display: block;
}
.sub-ref-empty {
  margin: 0;
  font-size: 0.8125rem;
  color: #94a3b8;
}
</style>
