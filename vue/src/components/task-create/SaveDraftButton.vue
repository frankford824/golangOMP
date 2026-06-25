<template>
  <button
    type="button"
    class="rounded px-3 py-1 text-xs"
    :class="
      disabled
        ? 'cursor-not-allowed bg-[rgb(var(--yb-border-slate))] text-[rgb(var(--yb-text-muted))]'
        : 'bg-[var(--v1-bg-primary)] text-[rgb(var(--yb-text-inverse))] hover:opacity-90'
    "
    :disabled="disabled"
    @click="$emit('save')"
  >
    保存草稿
  </button>
</template>

<script setup lang="ts">
defineProps<{ disabled?: boolean }>()
defineEmits<{ save: [] }>()
</script>
