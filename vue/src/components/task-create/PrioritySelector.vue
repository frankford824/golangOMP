<template>
  <div v-if="isDeptAdminPlus" class="flex items-center gap-2 text-xs">
    <span class="text-slate-400">优先级</span>
    <select class="rounded border border-slate-700 bg-slate-800 px-2 py-1 text-slate-100">
      <option value="low">low</option>
      <option value="normal">normal</option>
      <option value="high">high</option>
      <option value="critical">critical</option>
    </select>
  </div>
  <label v-else class="flex items-center gap-2 text-xs text-slate-300">
    <input type="checkbox" />
    是否加急
  </label>
</template>

<script setup lang="ts">
import { useAuth } from '@/composables/useAuth'

const { isDeptAdminPlus } = useAuth()
</script>
