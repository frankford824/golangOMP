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
import type { MainTaskStatus } from '@/domain/types/task'
import { getMainTaskStatusLabel } from '@/domain/enums/task-status'

const props = defineProps<{
  status: MainTaskStatus
  /** 任务中心等场景覆盖展示文案（不改变语义色，仍按 status 着色） */
  labelOverride?: string
}>()

const label = computed(() => {
  const override = props.labelOverride?.trim()
  if (override) return override
  return getMainTaskStatusLabel(props.status)
})

type SemanticKind = 'success' | 'processing' | 'warning' | 'error' | 'neutral'

function getSemanticKind(status: MainTaskStatus): SemanticKind {
  if (status === 'COMPLETED' || status === 'ARCHIVED') return 'success'
  if (status === 'BLOCKED' || status === 'CANCELLED') return 'error'
  if (status === 'DRAFT') return 'neutral'
  if (status === 'PENDING_AUDIT') return 'warning'
  return 'processing'
}

function getStatusStyle(kind: SemanticKind): string {
  if (kind === 'success') {
    return 'bg-[rgb(var(--yb-success-soft))] text-[rgb(var(--yb-success-strong))] border-[rgb(var(--yb-success-border))]'
  }
  if (kind === 'processing') {
    return 'bg-[rgb(var(--yb-brand-soft))] text-[rgb(var(--yb-brand-strong))] border-[rgb(var(--yb-brand-border))]'
  }
  if (kind === 'warning') {
    return 'bg-[rgb(var(--yb-warning-soft))] text-[rgb(var(--yb-warning-text))] border-[rgb(var(--yb-warning-border-soft))]'
  }
  if (kind === 'error') {
    return 'bg-[rgb(var(--yb-danger-soft))] text-[rgb(var(--yb-danger-text))] border-[rgb(var(--yb-danger-border))]'
  }
  return 'bg-[rgb(var(--yb-surface-muted))] text-[rgb(var(--yb-text-muted-strong))] border-[rgb(var(--yb-border))]'
}

const styleClass = computed(() => getStatusStyle(getSemanticKind(props.status)))
</script>
