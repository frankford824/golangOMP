<template>
  <div class="sub-status-panel">
    <div class="panel-title">子状态汇总</div>

    <template v-if="!isPurchase">
      <div class="status-row">
        <span class="row-label">设计</span>
        <span class="row-value" :class="designClass">
          {{ task.designSubStatus ? getDesignSubStatusLabel(task.designSubStatus) : '-' }}
        </span>
      </div>
      <div class="status-row">
        <span class="row-label">审核</span>
        <span class="row-value" :class="auditClass">
          {{ task.auditSubStatus ? getAuditSubStatusLabel(task.auditSubStatus) : '-' }}
        </span>
      </div>
    </template>

    <div v-if="isPurchase" class="status-row">
      <span class="row-label">采购</span>
      <span class="row-value">
        {{ task.purchaseSubStatus ? getPurchaseSubStatusLabel(task.purchaseSubStatus) : '-' }}
      </span>
    </div>

    <div class="status-row">
      <span class="row-label">仓库</span>
      <span class="row-value" :class="warehouseClass">
        {{ task.warehouseSubStatus ? getWarehouseSubStatusLabel(task.warehouseSubStatus) : '-' }}
      </span>
    </div>

    <div class="status-row">
      <span class="row-label">结单</span>
      <span class="row-value" :class="closeClass">
        {{ task.closeStatus ? closeLabel : '-' }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Task } from '@/domain/types/task'
import {
  getDesignSubStatusLabel,
  getAuditSubStatusLabel,
  getWarehouseSubStatusLabel,
  getPurchaseSubStatusLabel,
  getCloseStatusLabel,
} from '@/domain/enums/task-status'

const props = defineProps<{ task: Task }>()

const isPurchase = computed(() => props.task.businessType === 'PURCHASE_TASK')
const closeLabel = computed(() =>
  props.task.closeStatus ? getCloseStatusLabel(props.task.closeStatus) : '-',
)

const designClass = computed(() => {
  const s = props.task.designSubStatus
  if (s === 'FINALIZED') return 'text-emerald-600'
  if (s === 'REJECTED') return 'text-red-600'
  if (s === 'IN_PROGRESS' || s === 'PENDING_AUDIT') return 'text-blue-600'
  return ''
})

const auditClass = computed(() => {
  const s = props.task.auditSubStatus
  if (s === 'PASSED') return 'text-emerald-600'
  if (s === 'REJECTED') return 'text-red-600'
  if (s === 'IN_PROGRESS') return 'text-blue-600'
  return ''
})

const warehouseClass = computed(() => {
  const s = props.task.warehouseSubStatus
  if (s === 'RECEIVED' || s === 'DONE') return 'text-emerald-600'
  if (s === 'RETURNED') return 'text-red-600'
  return ''
})

const closeClass = computed(() => {
  const s = props.task.closeStatus
  if (s === 'CLOSED') return 'text-emerald-600 font-medium'
  if (s === 'READY') return 'text-blue-600'
  return ''
})
</script>

<style scoped>
.sub-status-panel {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.panel-title {
  font-size: 0.6875rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: #94a3b8;
  margin-bottom: 0.25rem;
}
.status-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.8125rem;
}
.row-label { color: #64748b; }
.row-value { color: #374151; font-weight: 500; }
</style>
