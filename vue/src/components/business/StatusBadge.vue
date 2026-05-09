<template>
  <span class="status-badge" :class="badgeClass">{{ label }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { getTaskStatusLabel } from '@/domain/enums/task-status'
import { getOutsourceOrderStatusLabel } from '@/domain/enums/outsource'
import { getWarehouseReceiptStatusLabel } from '@/domain/enums/warehouse'
import type { TaskStatus } from '@/domain/types/task'
import type { OutsourceOrderStatus } from '@/domain/types/outsource'
import type { WarehouseReceiptStatus } from '@/domain/types/warehouse'

type BadgeKind = 'task' | 'outsource' | 'warehouse'

const props = withDefaults(
  defineProps<{
    kind: BadgeKind
    status: TaskStatus | OutsourceOrderStatus | WarehouseReceiptStatus
    variant?: 'default' | 'success' | 'warning' | 'danger'
  }>(),
  { variant: 'default' }
)

const label = computed(() => {
  if (props.kind === 'task') return getTaskStatusLabel(props.status as TaskStatus)
  if (props.kind === 'outsource') return getOutsourceOrderStatusLabel(props.status as OutsourceOrderStatus)
  if (props.kind === 'warehouse') return getWarehouseReceiptStatusLabel(props.status as WarehouseReceiptStatus)
  return String(props.status)
})

const badgeClass = computed(() => {
  if (props.variant !== 'default') return `variant-${props.variant}`
  const s = props.status as string
  if (s === 'Completed' || s === 'Archived' || s === 'received' || s === 'archived' || s === 'review_passed') return 'variant-success'
  if (s.includes('Rejected') || s === 'Cancelled' || s === 'Blocked' || s === 'returned' || s === 'review_rejected') return 'variant-danger'
  if (s.includes('Pending') || s === 'Outsourcing' || s === 'pending' || s === 'returned') return 'variant-warning'
  return 'variant-default'
})
</script>

<style scoped>
.status-badge {
  display: inline-block;
  padding: 0.2rem 0.5rem;
  font-size: 0.75rem;
  font-weight: 500;
  border-radius: 4px;
}
.variant-default {
  background: #f1f5f9;
  color: #475569;
}
.variant-success {
  background: rgb(16 185 129 / 0.15);
  color: #059669;
}
.variant-warning {
  background: rgb(245 158 11 / 0.15);
  color: #d97706;
}
.variant-danger {
  background: rgb(220 38 38 / 0.1);
  color: #dc2626;
}
</style>
