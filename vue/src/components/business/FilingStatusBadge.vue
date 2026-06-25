<template>
  <span
    v-if="label !== '--'"
    class="filing-status-badge inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium border"
    :class="styleClass"
    :title="tooltip"
  >
    {{ label }}
  </span>
  <span v-else class="text-slate-400 text-xs">--</span>
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
  if (tone === 'success') return 'bg-emerald-100 text-emerald-800 border-emerald-200'
  if (tone === 'warning') return 'bg-amber-100 text-amber-800 border-amber-200'
  if (tone === 'error') return 'bg-red-100 text-red-800 border-red-200'
  if (tone === 'info') return 'bg-blue-100 text-blue-800 border-blue-200'
  return 'bg-slate-100 text-slate-600 border-slate-200'
}

const styleClass = computed(() =>
  getStyleClass(getTaskFilingStatusTone(props.status ?? undefined, props.taskType)),
)
</script>
