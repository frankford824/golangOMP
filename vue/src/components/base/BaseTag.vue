<template>
  <span
    class="inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium"
    :class="tagClass"
  >
    <slot />
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    tone?: 'default' | 'success' | 'warning' | 'danger'
  }>(),
  {
    tone: 'default',
  },
)

const tagClass = computed(() => {
  if (props.tone === 'success') return 'border-emerald-200 bg-emerald-50 text-emerald-700'
  if (props.tone === 'warning') return 'border-amber-200 bg-amber-50 text-amber-700'
  if (props.tone === 'danger') return 'border-red-200 bg-red-50 text-red-700'
  return 'border-slate-200 bg-slate-50 text-slate-700'
})
</script>

