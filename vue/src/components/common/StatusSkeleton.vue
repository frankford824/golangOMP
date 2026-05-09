<template>
  <div class="skeleton-group" :class="{ loading }">
    <template v-if="loading">
      <div class="skeleton-line" v-for="n in lines" :key="n" />
    </template>
    <template v-else>
      <slot />
    </template>
  </div>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    loading?: boolean
    lines?: number
  }>(),
  { loading: false, lines: 3 }
)
</script>

<style scoped>
.skeleton-group {
  min-height: 2rem;
}
.skeleton-line {
  height: 1rem;
  background: linear-gradient(90deg, #f1f5f9 25%, #e2e8f0 50%, #f1f5f9 75%);
  background-size: 200% 100%;
  animation: shimmer 1.2s ease-in-out infinite;
  border-radius: 4px;
  margin-bottom: 0.5rem;
}
.skeleton-line:last-child {
  margin-bottom: 0;
}
@keyframes shimmer {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -200% 0;
  }
}
</style>
