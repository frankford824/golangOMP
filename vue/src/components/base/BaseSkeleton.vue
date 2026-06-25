<template>
  <div
    class="base-skeleton"
    :class="[
      circle ? 'base-skeleton--circle' : 'base-skeleton--block',
      animated ? 'base-skeleton--animated' : '',
    ]"
    :style="{ width, height }"
    aria-hidden="true"
  />
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    width?: string
    height?: string
    circle?: boolean
    animated?: boolean
  }>(),
  {
    width: '100%',
    height: '1rem',
    circle: false,
    animated: true,
  },
)
</script>

<style scoped>
.base-skeleton {
  position: relative;
  overflow: hidden;
  flex: 0 0 auto;
  background: rgb(var(--yb-border-quiet));
}

.base-skeleton--block {
  border-radius: 0.5rem;
}

.base-skeleton--circle {
  border-radius: 999px;
}

.base-skeleton--animated::after {
  content: '';
  position: absolute;
  inset: 0;
  transform: translateX(-100%);
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgb(var(--yb-surface) / 0.72) 46%,
    transparent 100%
  );
  animation: base-skeleton-shimmer 1.4s ease-in-out infinite;
}

@keyframes base-skeleton-shimmer {
  100% {
    transform: translateX(100%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .base-skeleton--animated::after {
    animation: none;
  }
}
</style>
