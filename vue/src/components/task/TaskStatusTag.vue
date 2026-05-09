<template>
  <span
    class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium border"
    :class="styleClass"
  >
    {{ label }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { LegacyTaskStatus as TaskStatus } from '@/domain/types/task'

const props = defineProps<{ status: TaskStatus }>()

const statusLabels: Record<TaskStatus, string> = {
  Draft: '草稿',
  PendingAssign: '待指派',
  InProgress: '进行中',
  PendingAuditA: '待审核',
  RejectedByAuditA: '审核打回',
  PendingAuditB: '待审核',
  RejectedByAuditB: '审核打回',
  PendingOutsource: '待定制',
  Outsourcing: '定制中',
  PendingOutsourceReview: '定制中',
  PendingCustomizationReview: '定制中',
  PendingCustomizationProduction: '待定制生产',
  PendingEffectReview: '待效果审核',
  PendingEffectRevision: '待效果返修',
  PendingProductionTransfer: '待生产流转',
  PendingWarehouseQC: '待仓库质检',
  RejectedByWarehouse: '仓库驳回',
  PendingWarehouseReceive: '待仓库接收',
  PendingClose: '待结单',
  Completed: '已完成',
  Archived: '已归档',
  Blocked: '阻塞',
  Cancelled: '已取消',
}

const label = computed(() => statusLabels[props.status] ?? props.status)

type SemanticKind = 'success' | 'processing' | 'warning' | 'error' | 'neutral'

function getSemanticKind(status: TaskStatus): SemanticKind {
  if (status === 'Completed' || status === 'Archived') return 'success'
  if (status === 'Draft' || status === 'PendingAssign') return 'neutral'
  if (
    status === 'RejectedByAuditA' ||
    status === 'RejectedByAuditB' ||
    status === 'RejectedByWarehouse' ||
    status === 'Cancelled' ||
    status === 'Blocked'
  ) {
    return 'error'
  }
  if (
    status === 'PendingAuditA' ||
    status === 'PendingAuditB' ||
    status === 'PendingEffectReview' ||
    status === 'PendingWarehouseQC' ||
    status === 'PendingWarehouseReceive' ||
    status === 'PendingClose'
  ) {
    return 'warning'
  }
  return 'processing'
}

function getStatusStyle(kind: SemanticKind): string {
  if (kind === 'success') return 'bg-emerald-100 text-emerald-800 border-emerald-200'
  if (kind === 'processing') return 'bg-blue-100 text-blue-800 border-blue-200'
  if (kind === 'warning') return 'bg-amber-100 text-amber-800 border-amber-200'
  if (kind === 'error') return 'bg-red-100 text-red-800 border-red-200'
  return 'bg-slate-100 text-slate-600 border-slate-200'
}

const styleClass = computed(() => getStatusStyle(getSemanticKind(props.status)))
</script>
