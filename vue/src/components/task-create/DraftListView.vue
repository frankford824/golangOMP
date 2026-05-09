<template>
  <div class="space-y-2">
    <article
      v-for="draft in drafts"
      :key="draft.id"
      class="flex items-center justify-between rounded-md border border-slate-800 bg-slate-900 px-3 py-2"
    >
      <div>
        <p class="text-sm text-slate-100">{{ draft.task_type }}</p>
        <p class="text-xs text-slate-400">到期：{{ displayTime(draft.expires_at) }}</p>
      </div>
      <button type="button" class="text-xs text-red-300" @click="$emit('remove', draft.id)">删除</button>
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
