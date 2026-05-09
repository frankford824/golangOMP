<template>
  <div
    class="flex flex-col items-center justify-center gap-2 py-8 text-center text-sm text-slate-500"
  >
    <div class="mb-1 h-8 w-8 rounded-full border border-dashed border-slate-200" />
    <p class="text-sm font-medium text-slate-600">
      {{ title }}
    </p>
    <p v-if="description" class="text-xs text-slate-400">
      {{ description }}
    </p>
    <slot />
  </div>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    title?: string
    description?: string
  }>(),
  {
    title: '暂无数据',
    description: '',
  },
)
</script>

