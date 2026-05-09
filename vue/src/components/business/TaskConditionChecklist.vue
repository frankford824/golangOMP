<template>
  <section class="info-block">
    <h4 class="text-sm font-semibold text-slate-900">关单条件检查</h4>
    <div v-if="!hasReasons" class="mt-2 text-xs text-emerald-600">
      条件已满足，可按需在仓库节点结单。
    </div>
    <div v-else class="mt-2">
      <p class="text-xs font-medium text-red-500">当前不可结单原因：</p>
      <ul class="mt-1 list-disc pl-4 space-y-1 text-xs text-red-500">
        <li v-for="reason in reasons" :key="reason">
          {{ reason }}
        </li>
      </ul>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  reasons: string[]
}>()

const hasReasons = computed(() => props.reasons.length > 0)
</script>

<style scoped>
.info-block {
  @apply p-4 bg-white border border-slate-200 rounded-lg;
}
</style>

