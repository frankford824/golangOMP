<template>
  <ModuleSection :module="module" title="P 图精修" eyebrow="retouch">
    <div class="mt-3 flex items-center gap-2">
      <button
        v-if="showSubmit"
        type="button"
        class="retouch-submit-btn"
        :disabled="submitting"
        @click="$emit('submit')"
      >
        {{ submitting ? '提交中...' : '提交精修' }}
      </button>
    </div>
    <p class="mt-3 text-sm text-[var(--v1-text-secondary)]">精修模块不渲染审核动作。</p>
  </ModuleSection>
</template>

<script setup lang="ts">
import ModuleSection from '@/components/task-detail/ModuleSection.vue'
import type { ModuleSummary } from '@/services/apiTypes'

defineProps<{
  module?: ModuleSummary
  showSubmit?: boolean
  submitting?: boolean
}>()

defineEmits<{
  submit: []
}>()
</script>

<style scoped>
.retouch-submit-btn {
  min-height: 2rem;
  border: 1px solid #151a21;
  border-radius: 0.5rem;
  background: #151a21;
  color: #fff;
  font-size: 0.75rem;
  font-weight: 700;
  padding: 0.35rem 0.7rem;
}
.retouch-submit-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
</style>
