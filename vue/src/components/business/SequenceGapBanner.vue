<template>
  <div v-if="visible" class="sequence-gap-banner" role="alert">
    <span class="banner-text">检测到事件断序，请重新校准以同步最新状态。</span>
    <BaseButton variant="primary" size="sm" :loading="recalibrating" @click="onRecalibrate">
      重新校准
    </BaseButton>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import BaseButton from '@/components/base/BaseButton.vue'

defineProps<{ visible: boolean }>()

const emit = defineEmits<{ recalibrate: [] }>()

const recalibrating = ref(false)

async function onRecalibrate() {
  recalibrating.value = true
  emit('recalibrate')
  await new Promise((r) => setTimeout(r, 800))
  recalibrating.value = false
}
</script>

<style scoped>
.sequence-gap-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.75rem 1rem;
  background: #fef3c7;
  border: 1px solid #f59e0b;
  border-radius: 8px;
  margin-bottom: 1rem;
  font-size: 0.875rem;
  color: #92400e;
}
.banner-text {
  flex: 1;
}
</style>
