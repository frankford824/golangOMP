<script setup lang="ts">
defineProps<{ section: string; title: string; subtitle?: string }>()
</script>

<template>
  <header class="oa-page-head">
    <div>
      <p class="oa-crumb">{{ section }}</p>
      <h1>{{ title }}</h1>
      <p v-if="subtitle" class="oa-subtitle">{{ subtitle }}</p>
    </div>
    <div v-if="$slots.default" class="oa-actions"><slot /></div>
  </header>
</template>
