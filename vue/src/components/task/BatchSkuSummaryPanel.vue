<template>
  <section class="summary-panel">
    <div class="summary-head">
      <h4 class="summary-title">已添加商品</h4>
      <p v-if="!items.length" class="summary-empty">尚未添加商品</p>
    </div>

    <template v-if="items.length">
      <ul class="stat-list">
        <li>已添加 <strong>{{ items.length }}</strong> 个商品</li>
        <li>
          SKU 已生成：<strong>{{ skuReadyCount }}/{{ items.length }}</strong>
        </li>
        <li>已修改：<strong>{{ modifiedCount }}</strong></li>
        <li>待完善 / 异常：<strong>{{ abnormalCount }}</strong></li>
      </ul>
      <p class="ref-hint">{{ refHint }}</p>

      <div class="preview-block">
        <p class="preview-label">商品预览</p>
        <ol class="preview-list">
          <li v-for="(row, i) in previewRows" :key="row.clientKey" class="preview-row">
            <span class="preview-no">商品 {{ i + 1 }}</span>
            <span class="preview-sku">{{ skuLabel(row) }}</span>
            <span class="preview-name text-ellipsis">{{ row.productName || '未填写' }}</span>
            <span class="preview-status">{{ statusLabel(row) }}</span>
          </li>
        </ol>
        <p v-if="restCount > 0" class="rest-hint">+{{ restCount }} 个</p>
      </div>
    </template>

    <div class="summary-actions">
      <BaseButton
        variant="primary"
        size="sm"
        :disabled="addDisabled"
        :title="addDisabled ? addDisabledReason : ''"
        @click="$emit('add')"
      >
        新增商品
      </BaseButton>
      <BaseButton variant="secondary" size="sm" :disabled="!items.length" @click="$emit('manage')">
        管理商品
      </BaseButton>
      <BaseButton v-if="items.length" variant="secondary" size="sm" class="danger-weak" @click="onClear">
        清空商品
      </BaseButton>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { TaskBatchItem, TaskKind } from '@/domain/types'
import { getBatchItemDisplayStatus, batchItemStatusLabel } from '@/domain/batch-task-create'
import BaseButton from '@/components/base/BaseButton.vue'

const props = defineProps<{
  items: TaskBatchItem[]
  taskKind: TaskKind
  /** 未保存模板时禁止新增 */
  templateSaved?: boolean
}>()

const emit = defineEmits<{
  add: []
  manage: []
  clear: []
}>()

const addDisabled = computed(() => props.templateSaved === false)
const addDisabledReason = '请先点击「填写模板」并保存模板后再新增商品'

const PREVIEW_MAX = 5

const previewRows = computed(() => props.items.slice(0, PREVIEW_MAX))
const restCount = computed(() => Math.max(0, props.items.length - PREVIEW_MAX))

const skuReadyCount = computed(() =>
  props.items.filter((it) => {
    const s = props.taskKind === 'PURCHASE_TASK' ? it.purchaseSku?.trim() : it.newSku?.trim()
    return Boolean(s)
  }).length,
)

const modifiedCount = computed(() => props.items.filter((it) => it._editedFromTemplate).length)

const pendingCount = computed(() =>
  props.items.filter((it) => getBatchItemDisplayStatus(it, props.taskKind) === 'pending').length,
)
const duplicateSkuCount = computed(() => {
  const bag = new Map<string, number>()
  for (const it of props.items) {
    const sku = (props.taskKind === 'PURCHASE_TASK' ? it.purchaseSku : it.newSku)?.trim()
    if (!sku) continue
    bag.set(sku, (bag.get(sku) ?? 0) + 1)
  }
  return Array.from(bag.values()).filter((n) => n > 1).length
})
const abnormalCount = computed(() => pendingCount.value + duplicateSkuCount.value)
const duplicateKeys = computed(() => {
  const bag = new Map<string, TaskBatchItem[]>()
  for (const row of props.items) {
    const sku = (props.taskKind === 'PURCHASE_TASK' ? row.purchaseSku : row.newSku)?.trim()
    if (!sku) continue
    bag.set(sku, [...(bag.get(sku) ?? []), row])
  }
  const keys = new Set<string>()
  for (const rows of bag.values()) {
    if (rows.length > 1) rows.forEach((r) => keys.add(r.clientKey))
  }
  return keys
})
const refHint = computed(() => {
  const withRef = props.items.filter((it) => (it.referenceFileRefs?.length ?? 0) > 0).length
  if (withRef === 0) return '参考图：无'
  if (withRef === props.items.length) return '参考图：全部商品已含参考图'
  return `参考图：${withRef}/${props.items.length} 个商品含参考图`
})

function skuLabel(row: TaskBatchItem) {
  const s = props.taskKind === 'PURCHASE_TASK' ? row.purchaseSku : row.newSku
  return s?.trim() || '未生成'
}

function statusLabel(row: TaskBatchItem) {
  if (duplicateKeys.value.has(row.clientKey)) return '异常'
  if (!(row.productName ?? '').trim()) return '待完善'
  return batchItemStatusLabel(getBatchItemDisplayStatus(row, props.taskKind))
}

function onClear() {
  if (typeof window !== 'undefined') {
    const ok = window.confirm('确认清空全部已添加商品吗？该操作无法恢复。')
    if (!ok) return
  }
  emit('clear')
}
</script>

<style scoped>
.summary-panel {
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  padding: 0.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.summary-head {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.summary-title {
  margin: 0;
  font-size: 0.875rem;
  font-weight: 600;
  color: #0f172a;
}
.summary-empty {
  margin: 0;
  font-size: 0.8125rem;
  color: #64748b;
}
.stat-list {
  margin: 0;
  padding-left: 1.1rem;
  font-size: 0.8125rem;
  color: #334155;
  line-height: 1.6;
}
.ref-hint {
  margin: 0;
  font-size: 0.75rem;
  color: #475569;
}
.preview-block {
  border-top: 1px solid #e2e8f0;
  padding-top: 0.5rem;
}
.preview-label {
  margin: 0 0 0.35rem;
  font-size: 0.75rem;
  color: #64748b;
}
.preview-list {
  margin: 0;
  padding-left: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}
.preview-row {
  display: grid;
  grid-template-columns: 4rem minmax(0, 1fr) minmax(0, 1.2fr) 3.5rem;
  gap: 0.35rem;
  font-size: 0.75rem;
  align-items: center;
}
.preview-no {
  color: #64748b;
}
.preview-sku {
  font-weight: 600;
  color: #0f172a;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.text-ellipsis {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #475569;
}
.preview-status {
  text-align: right;
  color: #64748b;
}
.rest-hint {
  margin: 0.35rem 0 0;
  font-size: 0.75rem;
  color: #64748b;
}
.summary-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.danger-weak {
  color: #b91c1c;
  border-color: #fecaca;
  background: #fff;
}
</style>
