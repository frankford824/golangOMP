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

const props = defineProps<{ status: MainTaskStatus }>()

const label = computed(() => getMainTaskStatusLabel(props.status))

type SemanticKind = 'success' | 'processing' | 'warning' | 'error' | 'neutral'

function getSemanticKind(status: MainTaskStatus): SemanticKind {
  if (status === 'CLOSED') return 'success'
  if (status === 'BLOCKED') return 'error'
  if (status === 'DRAFT') return 'neutral'
  if (status === 'READY_TO_CLOSE') return 'warning'
  if (status === 'WAREHOUSE_PENDING' || status === 'WAREHOUSE_PROCESSING') return 'processing'
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
