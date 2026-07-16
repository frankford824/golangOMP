<template>
  <span class="status-tag" :class="`status-tag--${kind}`">{{ label }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ status: string }>()

const statusLabels: Record<string, string> = {
  Draft: '草稿',
  PendingAssign: '待指派',
  Assigned: '已指派',
  InProgress: '设计中',
  PendingAudit: '待审核',
  Completed: '已结单',
  Archived: '已归档',
  Blocked: '阻塞',
  Cancelled: '已取消',
}

const label = computed(() => statusLabels[props.status] || props.status)
const kind = computed(() => {
  if (['Completed', 'Archived'].includes(props.status)) return 'success'
  if (['Cancelled', 'Blocked'].includes(props.status)) return 'danger'
  if (props.status === 'PendingAudit') return 'warning'
  if (['Assigned', 'InProgress'].includes(props.status)) return 'active'
  return 'neutral'
})
</script>

<style scoped>
.status-tag{display:inline-flex;align-items:center;padding:3px 9px;border:1px solid rgb(var(--yb-border));border-radius:999px;font-size:12px;font-weight:700;background:rgb(var(--yb-surface-muted));color:rgb(var(--yb-text-muted-strong))}.status-tag--success{background:rgb(var(--yb-success-soft));border-color:rgb(var(--yb-success-border));color:rgb(var(--yb-success-strong))}.status-tag--active{background:rgb(var(--yb-brand-soft));border-color:rgb(var(--yb-brand-border));color:rgb(var(--yb-brand-strong))}.status-tag--warning{background:rgb(var(--yb-warning-soft));border-color:rgb(var(--yb-warning-border-soft));color:rgb(var(--yb-warning-text))}.status-tag--danger{background:rgb(var(--yb-danger-soft));border-color:rgb(var(--yb-danger-border));color:rgb(var(--yb-danger-text))}
</style>
