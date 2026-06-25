<template>
  <div v-if="isDeptAdminPlus" class="flex items-center gap-2 text-xs">
    <span class="text-[rgb(var(--yb-text-faint))]">优先级</span>
    <select class="rounded border border-[rgb(var(--yb-border-inverse-soft))] bg-[rgb(var(--yb-surface-neutral-inverse))] px-2 py-1 text-[rgb(var(--yb-text-inverse))]">
      <option value="low">low</option>
      <option value="normal">normal</option>
      <option value="high">high</option>
      <option value="critical">critical</option>
    </select>
  </div>
  <label v-else class="flex items-center gap-2 text-xs text-[rgb(var(--yb-text-faint))]">
    <input type="checkbox" />
    是否加急
  </label>
</template>

<script setup lang="ts">
import { useAuth } from '@/composables/useAuth'

const { isDeptAdminPlus } = useAuth()
</script>
