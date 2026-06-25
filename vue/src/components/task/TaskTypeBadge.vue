<template>
  <span class="task-type-badge" :class="badgeClass">{{ label }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { TaskBusinessType } from '@/domain/types/task'

const props = defineProps<{ type: TaskBusinessType | string }>()

const LABELS: Record<TaskBusinessType, string> = {
  ORIGINAL_PRODUCT_DEV: '原品开发',
  NEW_PRODUCT_DEV: '新品开发',
  PURCHASE_TASK: '采购任务',
  RETOUCH_TASK: 'P 图任务',
}

const label = computed(() => LABELS[props.type as TaskBusinessType] ?? props.type)

const badgeClass = computed(() => {
  if (props.type === 'PURCHASE_TASK') return 'badge-purchase'
  if (props.type === 'NEW_PRODUCT_DEV') return 'badge-new'
  if (props.type === 'RETOUCH_TASK') return 'badge-retouch'
  return 'badge-original'
})
</script>

<style scoped>
.task-type-badge {
  display: inline-block;
  padding: 0.125rem 0.5rem;
  font-size: 0.6875rem;
  font-weight: 500;
  border-radius: 9999px;
  white-space: nowrap;
}
.badge-original {
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
}
.badge-new {
  background: rgb(var(--yb-success-ui-soft));
  color: rgb(var(--yb-success-strong));
}
.badge-purchase {
  background: rgb(var(--yb-warning-highlight));
  color: rgb(var(--yb-warning-badge-text));
}
.badge-retouch {
  background: rgb(var(--yb-purple-soft));
  color: rgb(var(--yb-purple-text));
}
</style>
