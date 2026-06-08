<template>
  <section class="sku-section" aria-label="SKU 子项">
    <div class="sku-section-head">
      <h2 class="sku-section-title">
        批量子项
        <span class="sku-section-count">{{ items.length }} 项</span>
      </h2>
      <span class="sku-filing-badge" :class="filingBadgeClass">
        {{ filingStatusLabel }}
      </span>
    </div>

    <div class="sku-table-wrap">
      <table class="sku-table">
        <thead>
          <tr>
            <th class="sku-th">#</th>
            <th class="sku-th">SKU</th>
            <th class="sku-th">产品名称</th>
            <th class="sku-th">款式编码</th>
            <th class="sku-th sku-th--wide">设计要求</th>
            <th class="sku-th">成本</th>
            <th class="sku-th">参考图</th>
            <th class="sku-th">状态</th>
            <th class="sku-th">ERP 同步</th>
            <th class="sku-th">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(item, index) in items"
            :key="`${item.id ?? 'row'}-${index}`"
            class="sku-row"
          >
            <td class="sku-td sku-td--seq">{{ item.sequenceNo ?? index + 1 }}</td>
            <td class="sku-td sku-td--mono">{{ dash(item.skuCode) }}</td>
            <td class="sku-td">{{ dash(item.productNameSnapshot) }}</td>
            <td class="sku-td sku-td--mono">{{ dash(item.productIId) }}</td>
            <td class="sku-td sku-td--req">{{ trunc(item.designRequirement) }}</td>
            <td class="sku-td sku-td--cost">
              <div class="sku-cost-cell" :title="skuCostTooltip(item)">
                <span class="sku-cost-value" :class="{ 'sku-cost-value--empty': skuCostAmount(item) == null }">
                  {{ formatSkuCost(item) }}
                </span>
                <span v-if="skuCostMeta(item)" class="sku-cost-meta">{{ skuCostMeta(item) }}</span>
              </div>
            </td>
            <td class="sku-td">
              <div v-if="toThumbItems(item).length" class="sku-refs-cell">
                <AssetThumbStrip :items="toThumbItems(item)" size="sm" empty-text="未上传" />
              </div>
              <span v-else class="sku-empty-ref">未上传</span>
            </td>
            <td class="sku-td">
              <span class="sku-status-pill">{{ skuItemStatusLabelCn(item.skuStatus) }}</span>
            </td>
            <td class="sku-td">
              <div class="sku-sync-cell">
                <FilingStatusBadge
                  v-if="item.erp_sync_required !== false"
                  :status="resolvedSkuFilingStatus(item)"
                  :error-message="skuFilingErrorMessage(item)"
                />
                <span v-else class="sku-sync-not-required">无需同步</span>
                <span v-if="item.erp_sync_version != null" class="sku-sync-meta">
                  v{{ item.erp_sync_version }}
                </span>
                <span v-if="item.last_filed_at" class="sku-sync-meta">
                  {{ formatFiledAt(item.last_filed_at) }}
                </span>
              </div>
            </td>
            <td class="sku-td">
              <div class="sku-op-stack">
                <button
                  type="button"
                  class="sku-action-btn sku-action-btn--primary"
                  :disabled="!canUploadDesign"
                  :title="canUploadDesign ? '' : disabledUploadTitle"
                  @click="$emit('upload-design', { item, index })"
                >
                  {{ uploadDesignLabel }}
                </button>
                <button
                  type="button"
                  class="sku-action-btn"
                  @click="$emit('edit', { item, index })"
                >
                  编辑/成本
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import AssetThumbStrip, { type AssetThumbItem } from '@/components/task-detail/AssetThumbStrip.vue'
import FilingStatusBadge from '@/components/business/FilingStatusBadge.vue'
import type { TaskSkuItem } from '@/domain/types/task'
import { skuItemStatusLabelCn } from '@/domain/mappers/read-model-labels-cn'
import { formatErpSyncFailureMessage } from '@/utils/business-copy'

const props = defineProps<{
  items: TaskSkuItem[]
  filingStatus?: string | null
  canUploadDesign?: boolean
  uploadDesignLabel?: string
  disabledUploadTitle?: string
}>()

defineEmits<{
  edit: [{ item: TaskSkuItem; index: number }]
  'upload-design': [{ item: TaskSkuItem; index: number }]
}>()

const canUploadDesign = computed(() => props.canUploadDesign !== false)
const uploadDesignLabel = computed(() => props.uploadDesignLabel || '上传设计稿')
const disabledUploadTitle = computed(() => props.disabledUploadTitle || '当前状态不可上传设计稿')

const filingStatusLabel = computed(() => {
  const raw = String(props.filingStatus ?? '').trim()
  if (!raw) return '未建档'
  if (raw === 'filed') return '建档完成'
  if (raw === 'filing') return '同步中'
  if (raw === 'pending_filing') return '待同步'
  if (raw === 'filing_failed') return '同步失败'
  return raw
})

const filingBadgeClass = computed(() => {
  const raw = String(props.filingStatus ?? '').trim()
  if (raw === 'filed') return 'sku-filing-badge--done'
  if (raw === 'filing') return 'sku-filing-badge--progress'
  if (raw === 'filing_failed') return 'sku-filing-badge--error'
  return 'sku-filing-badge--default'
})

function dash(value: unknown): string {
  const text = String(value ?? '').trim()
  return text || '-'
}

function trunc(value: unknown): string {
  const text = String(value ?? '').trim()
  if (!text) return '-'
  return text.length > 36 ? `${text.slice(0, 36)}...` : text
}

function skuCostAmount(item: TaskSkuItem): number | undefined {
  if (typeof item.costPrice === 'number' && Number.isFinite(item.costPrice)) return item.costPrice
  if (typeof item.estimatedCost === 'number' && Number.isFinite(item.estimatedCost)) return item.estimatedCost
  return undefined
}

function formatSkuCost(item: TaskSkuItem): string {
  const amount = skuCostAmount(item)
  if (amount == null) return item.requiresManualReview ? '待补成本' : '-'
  return `¥${amount.toFixed(3)}`
}

function skuCostMeta(item: TaskSkuItem): string {
  if (item.manualCostOverride === true || item.costPriceMode === 'manual') return '手动'
  if (typeof item.costPrice === 'number' && Number.isFinite(item.costPrice)) return '已计算'
  if (typeof item.estimatedCost === 'number' && Number.isFinite(item.estimatedCost)) return '预估'
  if (item.requiresManualReview === true) return '待补'
  return ''
}

function skuCostTooltip(item: TaskSkuItem): string {
  const amount = skuCostAmount(item)
  const ruleName = String(item.costRuleName ?? '').trim()
  if (amount == null) return item.requiresManualReview ? '成本待补充后再同步 ERP' : '暂无成本'
  const source = skuCostMeta(item) || '成本'
  return ruleName ? `${source} ${amount.toFixed(3)}；规则：${ruleName}` : `${source} ${amount.toFixed(3)}`
}

function toThumbItems(item: TaskSkuItem): AssetThumbItem[] {
  const refs = item.referenceFileRefs ?? []
  return refs
    .map((ref, idx) => {
      const src = String(ref.download_url ?? '').trim()
      if (!src) return null
      return {
        key: `sku-ref-${item.id ?? item.skuCode ?? 'row'}-${idx}`,
        src,
        alt: String(ref.filename ?? `参考图 ${idx + 1}`),
        label: String(ref.filename ?? `图 ${idx + 1}`),
      }
    })
    .filter((row) => row != null) as AssetThumbItem[]
}

function resolvedSkuFilingStatus(item: TaskSkuItem): string | undefined {
  const taskLevelStatus = String(props.filingStatus ?? '').trim()
  if (taskLevelStatus) return taskLevelStatus
  const skuStatus = String(item.filing_status ?? '').trim()
  if (skuStatus) return skuStatus
  return undefined
}

function skuFilingErrorMessage(item: TaskSkuItem): string {
  return formatErpSyncFailureMessage(item.filing_error_message ?? '')
}

function formatFiledAt(value: string): string {
  const text = String(value ?? '').trim()
  if (!text) return '-'
  const normalized = text.replace('T', ' ')
  return normalized.length > 16 ? normalized.slice(0, 16) : normalized
}

</script>

<style scoped>
.sku-section {
  border: 1px solid var(--dv-border-soft, #e8ecf4);
  border-radius: var(--dv-r-outer, 1.25rem);
  background: #fff;
  box-shadow: var(--dv-surface-elev, 0 1px 3px rgba(15, 23, 42, 0.07));
  padding: 1.15rem 1.2rem 1.2rem;
}

.sku-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
}

.sku-section-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 800;
  color: #151a21;
  letter-spacing: -0.01em;
  line-height: 1.3;
}

.sku-section-count {
  display: inline-flex;
  align-items: center;
  margin-left: 0.4rem;
  padding: 0.1rem 0.45rem;
  border-radius: 9999px;
  background: #f1f5f9;
  color: #64748b;
  font-size: 0.6875rem;
  font-weight: 700;
  vertical-align: middle;
}

.sku-filing-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  min-height: 1.5rem;
  padding: 0.2rem 0.6rem;
  border-radius: 9999px;
  font-size: 0.6875rem;
  font-weight: 700;
  white-space: nowrap;
  flex-shrink: 0;
}

.sku-filing-badge--done {
  color: #067647;
  background: #ecfdf3;
  border: 1px solid #a7f3d0;
}

.sku-filing-badge--progress {
  color: #175cd3;
  background: #eff8ff;
  border: 1px solid #bfdbfe;
}

.sku-filing-badge--error {
  color: #b42318;
  background: #fef3f2;
  border: 1px solid #fecdc8;
}

.sku-filing-badge--default {
  color: #667085;
  background: #f9fafb;
  border: 1px solid #e4e7ec;
}

.sku-table-wrap {
  overflow-x: auto;
  border-radius: 0.75rem;
  border: 1px solid #eaecf0;
}

.sku-table {
  width: 100%;
  min-width: 960px;
  border-collapse: collapse;
}

.sku-th {
  padding: 0.55rem 0.65rem;
  text-align: left;
  font-size: 0.6875rem;
  font-weight: 700;
  letter-spacing: 0.02em;
  color: #667085;
  background: #f9fafb;
  border-bottom: 1px solid #eaecf0;
  white-space: nowrap;
}

.sku-th--wide {
  min-width: 140px;
}

.sku-td {
  padding: 0.6rem 0.65rem;
  text-align: left;
  font-size: 0.8125rem;
  color: #344054;
  vertical-align: middle;
  line-height: 1.45;
}

.sku-td--seq {
  color: #98a2b3;
  font-variant-numeric: tabular-nums;
  width: 2.5rem;
}

.sku-td--mono {
  font-family: 'SF Mono', 'Cascadia Code', 'Consolas', monospace;
  font-size: 0.75rem;
  letter-spacing: 0.01em;
}

.sku-td--req {
  min-width: 140px;
  max-width: 220px;
  color: #475467;
}

.sku-td--cost {
  min-width: 92px;
}

.sku-cost-cell {
  display: inline-flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.12rem;
  white-space: nowrap;
}

.sku-cost-value {
  font-weight: 800;
  color: #111827;
  font-variant-numeric: tabular-nums;
}

.sku-cost-value--empty {
  color: #98a2b3;
  font-weight: 700;
}

.sku-cost-meta {
  color: #667085;
  font-size: 0.6875rem;
  font-weight: 600;
  line-height: 1;
}

.sku-row {
  border-bottom: 1px solid #f2f4f7;
  transition: background 0.12s ease;
}

.sku-row:last-child {
  border-bottom: none;
}

.sku-row:hover {
  background: #f9fafb;
}

.sku-empty-ref {
  color: #d0d5dd;
  font-size: 0.75rem;
}

.sku-status-pill {
  display: inline-flex;
  align-items: center;
  border: 1px solid #e4e7ec;
  border-radius: 9999px;
  padding: 0.12rem 0.5rem;
  color: #344054;
  background: #f9fafb;
  font-size: 0.6875rem;
  font-weight: 600;
}

.sku-sync-cell {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}

.sku-sync-meta {
  color: #98a2b3;
  font-size: 0.6875rem;
  font-variant-numeric: tabular-nums;
}

.sku-sync-not-required {
  display: inline-flex;
  align-items: center;
  border-radius: 9999px;
  border: 1px solid #e4e7ec;
  background: #f9fafb;
  color: #667085;
  padding: 0.1rem 0.45rem;
  font-size: 0.6875rem;
  font-weight: 600;
}

.sku-op-stack {
  display: inline-flex;
  gap: 0.35rem;
}

.sku-action-btn {
  border: 1px solid #d0d5dd;
  border-radius: 0.5rem;
  background: #fff;
  padding: 0.3rem 0.6rem;
  font-size: 0.6875rem;
  font-weight: 700;
  color: #344054;
  cursor: pointer;
  transition: background 0.12s ease, border-color 0.12s ease;
}

.sku-action-btn:hover {
  background: #f9fafb;
  border-color: #98a2b3;
}

.sku-action-btn:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.sku-action-btn--primary {
  background: #151a21;
  color: #fff;
  border-color: #151a21;
}

.sku-action-btn--primary:hover {
  background: #0f1218;
}
</style>
