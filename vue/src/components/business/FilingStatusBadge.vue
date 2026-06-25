<template>
  <span
    v-if="label !== '--'"
    class="filing-status-badge inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium border"
    :class="styleClass"
    :title="tooltip"
  >
    {{ label }}
  </span>
  <span v-else class="text-[rgb(var(--yb-text-faint))] text-xs">--</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  getTaskFilingStatusLabel,
  getTaskFilingStatusTone,
  getTaskFilingStatusDescription,
} from '@/utils/filing-status'
import { formatErpSyncFailureMessage } from '@/utils/business-copy'

const props = withDefaults(
  defineProps<{
    status?: string | null
    taskType?: string | null
    missingFieldsSummary?: string | null
    errorMessage?: string | null
  }>(),
  { status: undefined, taskType: undefined, missingFieldsSummary: undefined, errorMessage: undefined },
)

const label = computed(() => getTaskFilingStatusLabel(props.status, props.taskType))
const businessErrorMessage = computed(() => formatErpSyncFailureMessage(props.errorMessage ?? ''))

const tooltip = computed(() =>
  getTaskFilingStatusDescription(props.status ?? undefined, props.taskType, {
    missingFieldsSummary: props.missingFieldsSummary ?? undefined,
    errorMessage: businessErrorMessage.value || undefined,
  }),
)

function getStyleClass(tone: string): string {
  if (tone === 'success') {
    return 'bg-[rgb(var(--yb-success-soft))] text-[rgb(var(--yb-success-strong))] border-[rgb(var(--yb-success-border))]'
  }
  if (tone === 'warning') {
    return 'bg-[rgb(var(--yb-warning-soft))] text-[rgb(var(--yb-warning-text))] border-[rgb(var(--yb-warning-border-soft))]'
  }
  if (tone === 'error') {
    return 'bg-[rgb(var(--yb-danger-soft))] text-[rgb(var(--yb-danger-text))] border-[rgb(var(--yb-danger-border))]'
  }
  if (tone === 'info') {
    return 'bg-[rgb(var(--yb-brand-soft))] text-[rgb(var(--yb-brand-strong))] border-[rgb(var(--yb-brand-border))]'
  }
  return 'bg-[rgb(var(--yb-surface-muted))] text-[rgb(var(--yb-text-muted-strong))] border-[rgb(var(--yb-border))]'
}

const styleClass = computed(() =>
  getStyleClass(getTaskFilingStatusTone(props.status ?? undefined, props.taskType)),
)
</script>
