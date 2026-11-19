<script setup lang="ts">
import DesktopSlot from './DesktopSlot.vue'

defineProps<{ module: string; identifier?: string | number }>()
</script>

<template>
  <DesktopSlot target="title">
    <nav class="oa-detail-crumb" aria-label="当前位置">
      <span>{{ module }}</span><span aria-hidden="true">/</span>
      <span class="oa-detail-id" aria-current="page" :title="String(identifier ?? '')">{{ identifier || '详情' }}</span>
    </nav>
  </DesktopSlot>
</template>

<style scoped>
.oa-detail-crumb { display: flex; align-items: center; min-width: 0; gap: 8px; white-space: nowrap; font: 600 15px/1.4 var(--oa-font); }
.oa-detail-crumb [aria-hidden] { color: var(--oa-secondary); }
.oa-detail-id { min-width: 0; max-width: 21ch; overflow: hidden; text-overflow: ellipsis; font: 500 13px/1.5 var(--yb-font-mono); }
</style>
