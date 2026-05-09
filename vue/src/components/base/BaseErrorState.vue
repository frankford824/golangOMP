<template>
  <div class="flex flex-col items-center justify-center gap-2 py-8 text-center text-sm">
    <p class="text-red-600">
      {{ title }}
    </p>
    <p v-if="description" class="text-xs text-slate-500">
      {{ description }}
    </p>
    <BaseButton
      v-if="showRetry"
      variant="secondary"
      size="sm"
      @click="$emit('retry')"
    >
      重试
    </BaseButton>
  </div>
</template>

<script setup lang="ts">
import BaseButton from './BaseButton.vue'

withDefaults(
  defineProps<{
    title?: string
    description?: string
    showRetry?: boolean
  }>(),
  {
    title: '加载失败',
    description: '',
    showRetry: true,
  },
)

defineEmits<{
  retry: []
}>()
</script>

