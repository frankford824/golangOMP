<script setup lang="ts">
defineProps<{ module: string; identifier?: string | number }>()
</script>

<template>
  <header class="oa-detail-heading">
    <nav class="oa-detail-crumb" aria-label="当前位置">
      <span>{{ module }}</span><span aria-hidden="true">/</span>
      <span class="oa-detail-id" aria-current="page" :title="String(identifier ?? '')">{{ identifier || '详情' }}</span>
    </nav>
    <slot />
  </header>
</template>

<style scoped>
.oa-detail-heading { display:flex; align-items:center; justify-content:space-between; gap:12px; min-width:0; flex:none; padding-block:4px; }
.oa-detail-heading > :last-child { flex-shrink:0; }
.oa-detail-crumb { display: flex; align-items: center; min-width: 0; gap: 8px; white-space: nowrap; font: 600 15px/1.4 var(--oa-font); }
.oa-detail-crumb [aria-hidden] { color: var(--oa-secondary); }
.oa-detail-id { min-width: 0; max-width: 21ch; overflow: hidden; text-overflow: ellipsis; font: 500 13px/1.5 var(--yb-font-mono); }
</style>
