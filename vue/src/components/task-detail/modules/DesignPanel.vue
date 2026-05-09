<template>
  <ModuleSection :module="module" title="设计交付" eyebrow="design">
    <p class="mt-3 text-sm text-[var(--v1-text-secondary)]">设计动作按 allowed_actions 渲染。</p>
  </ModuleSection>
</template>

<script setup lang="ts">
import ModuleSection from '@/components/task-detail/ModuleSection.vue'
import type { ModuleSummary } from '@/services/apiTypes'

defineProps<{ module?: ModuleSummary }>()
</script>
