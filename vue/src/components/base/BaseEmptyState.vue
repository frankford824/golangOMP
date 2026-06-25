<template>
  <div
    class="flex flex-col items-center justify-center gap-2 py-8 text-center text-sm text-[rgb(var(--yb-text-muted))]"
  >
    <div class="mb-1 h-8 w-8 rounded-full border border-dashed border-[rgb(var(--yb-border))]" />
    <p class="text-sm font-medium text-[rgb(var(--yb-text-muted-strong))]">
      {{ title }}
    </p>
    <p v-if="description" class="text-xs text-[rgb(var(--yb-text-faint))]">
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
