<template>
  <div class="space-y-2">
    <article
      v-for="draft in drafts"
      :key="draft.id"
      class="flex items-center justify-between rounded-md border border-[rgb(var(--yb-border-inverse-soft))] bg-[rgb(var(--yb-surface-neutral-inverse-deep))] px-3 py-2"
    >
      <div>
        <p class="text-sm text-[rgb(var(--yb-text-inverse))]">{{ draft.task_type }}</p>
        <p class="text-xs text-[rgb(var(--yb-text-faint))]">到期：{{ displayTime(draft.expires_at) }}</p>
      </div>
      <button type="button" class="text-xs text-[rgb(var(--yb-danger-border-hover))]" @click="$emit('remove', draft.id)">删除</button>
    </article>
  </div>
</template>

<script setup lang="ts">
import { formatDateTimeBeijing } from '@/utils/date'

defineProps<{ drafts: Array<{ id: string; task_type: string; expires_at: string }> }>()
defineEmits<{ remove: [string] }>()

function displayTime(value: string): string {
  return formatDateTimeBeijing(value) || value || '-'
}
</script>
