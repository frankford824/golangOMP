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
  if (props.tone === 'success') {
    return 'border-[rgb(var(--yb-success-border))] bg-[rgb(var(--yb-success-soft))] text-[rgb(var(--yb-success-text))]'
  }
  if (props.tone === 'warning') {
    return 'border-[rgb(var(--yb-warning-border-soft))] bg-[rgb(var(--yb-warning-soft))] text-[rgb(var(--yb-warning-text))]'
  }
  if (props.tone === 'danger') {
    return 'border-[rgb(var(--yb-danger-border))] bg-[rgb(var(--yb-danger-soft))] text-[rgb(var(--yb-danger-text))]'
  }
  return 'border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface-soft))] text-[rgb(var(--yb-text-body))]'
})
</script>
