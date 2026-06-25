<!--
  @deprecated 任务详情已合并至 ProductCodeBlock（并列商品切换）。保留文件仅供历史对照，勿在新代码中引用。
-->
<template>
  <section
    class="detail-block h-full flex flex-col rounded-lg border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface))] shadow-sm p-6"
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
        <div v-if="subItemReferenceRefs.length > 0" class="sub-ref-thumb-grid">
          <button
            v-for="(refObj, i) in subItemReferenceRefs"
            :key="'sref-' + i"
            type="button"
            class="sub-ref-thumb-btn"
            @click="openSubReferencePreview(i)"
          >
            <AssetPreviewMedia
              :asset-id="referencePreviewAssetId(refObj) || null"
              :resolved-preview-url="refObj.download_url || null"
              :fallback-src="refObj.download_url || null"
              :alt="`子项参考图 ${i + 1}`"
              img-class="sub-ref-thumb-media"
              inner-img-class="sub-ref-thumb-img"
              :defer-until-visible="true"
              @open-full="(url, context) => openSubReferencePreview(i, url, context)"
            />
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
import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'
import type { Task, TaskSkuItem } from '@/domain/types/task'
import type { ReferenceFileRef } from '@/services/api/assetsApi'
import { TASK_DETAIL_KEY } from '@/composables/task-detail-key'
import {
  IMAGE_PREVIEW_LIGHTBOX_KEY,
  type ImagePreviewLightboxItem,
  type OpenImagePreviewLightbox,
} from '@/components/media/imagePreviewLightbox'

const injected = inject<ComputedRef<Task | null>>(TASK_DETAIL_KEY)
if (!injected) throw new Error('[TaskSkuItemsBlock] 必须在 TaskDetailView 内使用')

const taskRef = injected
const openLightbox = inject<OpenImagePreviewLightbox>(IMAGE_PREVIEW_LIGHTBOX_KEY, () => {})

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
const subItemReferencePreviewItems = computed((): ImagePreviewLightboxItem[] =>
  subItemReferenceRefs.value
    .map((refObj, index) => {
      const src = String(refObj.download_url ?? '').trim()
      const title = refObj.filename?.trim() || `子项参考图 ${index + 1}`
      const previewAssetId = referencePreviewAssetId(refObj)
      return src || previewAssetId
        ? {
            src,
            previewAssetId,
            resolvedPreviewUrl: src || undefined,
            fallbackSrc: src || undefined,
            title,
            alt: title,
            preferredFilename: title,
            downloadUrl: src,
          }
        : null
    })
    .filter((item) => item != null) as ImagePreviewLightboxItem[],
)

function referencePreviewAssetId(refObj: ReferenceFileRef | undefined): string | undefined {
  const id = String(refObj?.asset_id ?? refObj?.ref_id ?? '').trim()
  return id || undefined
}

function openSubReferencePreview(
  index: number,
  url?: string,
  context?: {
    assetId?: string
    fallbackAssetId?: string
    fallbackSrc?: string
    resolvedPreviewUrl?: string
  },
) {
  const item = subItemReferencePreviewItems.value[index]
  const src = String(url || item?.src || '').trim()
  if (!item || (!src && !item.previewAssetId)) return
  const items = [...subItemReferencePreviewItems.value]
  items[index] = {
    ...item,
    src: src || item.src,
    previewAssetId: context?.assetId || item.previewAssetId,
    fallbackAssetId: context?.fallbackAssetId || item.fallbackAssetId,
    fallbackSrc: context?.fallbackSrc || item.fallbackSrc,
    resolvedPreviewUrl: context?.resolvedPreviewUrl || item.resolvedPreviewUrl,
  }
  openLightbox(src, {
    title: item.title,
    items,
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
  background: rgb(var(--yb-surface-subtle));
  color: rgb(var(--yb-text-placeholder));
  font-size: 0.75rem;
  flex-shrink: 0;
}
.block-title {
  font-size: 0.875rem;
  font-weight: 600;
  color: rgb(var(--yb-text-deep));
  margin: 0;
}
.current-panel {
  margin-top: 0.75rem;
  padding: 0.875rem;
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 0.625rem;
  background: rgb(var(--yb-surface-subtle));
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
  color: rgb(var(--yb-text-muted-strong));
}
.current-subtitle {
  margin: 0.25rem 0 0;
  font-size: 0.875rem;
  color: rgb(var(--yb-text-navy));
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
  border: 1px solid rgb(var(--yb-text-disabled));
  background: rgb(var(--yb-surface-slate));
  color: rgb(var(--yb-text-soft));
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
  border: 1px solid rgb(var(--yb-border-slate));
  background: rgb(var(--yb-surface-subtle));
  font-size: 0.75rem;
  color: rgb(var(--yb-text-soft));
  cursor: pointer;
  white-space: nowrap;
  font-weight: 500;
}
.item-tab-active {
  border-color: rgb(var(--yb-brand-border-strong));
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
}
.core-info {
  background: rgb(var(--yb-surface));
  border: 1px solid rgb(var(--yb-border-blue-faint));
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
  color: rgb(var(--yb-text-muted-strong));
  font-weight: 600;
}
.core-value {
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-navy));
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
  color: rgb(var(--yb-text-muted-strong));
  flex-shrink: 0;
}
.attr-value {
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-deep));
  min-width: 0;
  word-break: break-word;
}
.pill-default {
  border-color: rgb(var(--yb-text-disabled));
  background: rgb(var(--yb-surface-slate));
  color: rgb(var(--yb-text-soft));
}
.pill-info {
  border-color: rgb(var(--yb-brand-border));
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
}
.pill-warning {
  border-color: rgb(var(--yb-warning-border-soft));
  background: rgb(var(--yb-warning-soft));
  color: rgb(var(--yb-warning-text));
}
.pill-success {
  border-color: rgb(var(--yb-success-border));
  background: rgb(var(--yb-success-ui-soft));
  color: rgb(var(--yb-success-strong));
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
  border: 1px solid rgb(var(--yb-border-slate));
  background: rgb(var(--yb-surface-subtle));
  color: rgb(var(--yb-text-soft));
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s, color 0.12s;
}
.nav-btn:hover:not(:disabled) {
  background: rgb(var(--yb-brand-soft));
  border-color: rgb(var(--yb-brand-border));
  color: rgb(var(--yb-brand-strong));
}
.nav-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.nav-indicator {
  font-size: 0.75rem;
  color: rgb(var(--yb-text-muted-strong));
  min-width: 2.6rem;
  text-align: center;
}
.requirement-brief {
  margin-top: 0.625rem;
  padding-top: 0.5rem;
  border-top: 1px dashed rgb(var(--yb-border-blue-muted));
}
.requirement-label {
  font-size: 0.6875rem;
  font-weight: 600;
  color: rgb(var(--yb-text-muted-strong));
}
.requirement-text {
  margin: 0.25rem 0 0;
  font-size: 0.75rem;
  color: rgb(var(--yb-text-slate));
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
  border-top: 1px dashed rgb(var(--yb-border-blue-muted));
}
.sub-ref-label {
  display: block;
  font-size: 0.6875rem;
  font-weight: 600;
  color: rgb(var(--yb-text-muted-strong));
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
.sub-ref-thumb-media,
.sub-ref-thumb-btn :deep(.sub-ref-thumb-media),
.sub-ref-thumb-btn :deep(.apm),
.sub-ref-thumb-img {
  width: 100%;
  aspect-ratio: 1;
  object-fit: cover;
  border-radius: 6px;
  border: 1px solid rgb(var(--yb-border-slate));
  background: rgb(var(--yb-surface));
  display: block;
}
.sub-ref-thumb-btn :deep(.apm-placeholder),
.sub-ref-thumb-btn :deep(.apm-empty) {
  min-height: 0;
  height: auto;
  aspect-ratio: 1;
  padding: 0.2rem;
}
.sub-ref-empty {
  margin: 0;
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-placeholder));
}
</style>
