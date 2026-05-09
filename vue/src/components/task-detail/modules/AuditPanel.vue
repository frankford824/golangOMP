<template>
  <ModuleSection :module="module" title="审核" eyebrow="audit">
    <div class="mt-3 rounded-lg border border-dashed border-[var(--v1-border)] p-3 text-sm text-[var(--v1-text-secondary)]">
      审核参考图通过 update_reference_files 动作维护，不参与资产版本流。
    </div>
  </ModuleSection>
</template>

<script setup lang="ts">
import ModuleSection from '@/components/task-detail/ModuleSection.vue'
import type { ModuleSummary } from '@/services/apiTypes'

defineProps<{ module?: ModuleSummary }>()
</script>
