<script setup lang="ts">
defineProps<{
  loading?: boolean
  error?: string
  empty?: boolean
  loadingLabel?: string
  emptyLabel?: string
  retryLabel?: string
}>()

defineEmits<{
  retry: []
}>()
</script>

<template>
  <div v-if="loading" class="aw-async-state" role="status">
    {{ loadingLabel || '正在加载' }}
  </div>
  <div v-else-if="error" class="aw-async-state aw-async-state--error" role="alert">
    <span>{{ error }}</span>
    <button class="aw-secondary-button" type="button" @click="$emit('retry')">
      {{ retryLabel || '重试' }}
    </button>
  </div>
  <div v-else-if="empty" class="aw-async-state">
    {{ emptyLabel || '暂无数据' }}
  </div>
  <slot v-else />
</template>
