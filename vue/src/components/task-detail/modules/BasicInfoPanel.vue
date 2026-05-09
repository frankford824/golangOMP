<template>
  <ModuleSection :module="module" title="基础信息 / 商品需求" eyebrow="basic_info">
    <p class="mt-3 text-sm text-[var(--v1-text-secondary)]">基础信息由任务聚合详情中的 basic_info 模块驱动。</p>
  </ModuleSection>
</template>

<script setup lang="ts">
import ModuleSection from '@/components/task-detail/ModuleSection.vue'
import type { ModuleSummary } from '@/services/apiTypes'

defineProps<{ module?: ModuleSummary }>()
</script>
