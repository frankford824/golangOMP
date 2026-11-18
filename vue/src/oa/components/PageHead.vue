<script setup lang="ts">
defineProps<{ title: string; subtitle?: string }>()
</script>

<template>
  <header class="yb-page-surface yb-page-header-row oa-page-head" data-page-header="oa">
    <div class="yb-page-heading-copy">
      <h1 class="yb-page-title">{{ title }}</h1>
      <p v-if="subtitle" class="yb-page-subtitle">{{ subtitle }}</p>
    </div>
    <div v-if="$slots.default" class="oa-actions"><slot /></div>
  </header>
</template>
