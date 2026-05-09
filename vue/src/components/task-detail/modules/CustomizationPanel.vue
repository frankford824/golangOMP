<template>
  <ModuleSection :module="module" title="定制交付" eyebrow="customization">
    <p class="mt-3 text-sm text-[var(--v1-text-secondary)]">定制任务不渲染 design Panel。</p>
  </ModuleSection>
</template>

<script setup lang="ts">
import ModuleSection from '@/components/task-detail/ModuleSection.vue'
import type { ModuleSummary } from '@/services/apiTypes'

defineProps<{ module?: ModuleSummary }>()
</script>
