<template>
  <button
    type="button"
    class="rounded px-3 py-1 text-xs"
    :class="
      disabled
        ? 'cursor-not-allowed bg-slate-200 text-slate-500'
        : 'bg-[var(--v1-bg-primary)] text-white hover:opacity-90'
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
