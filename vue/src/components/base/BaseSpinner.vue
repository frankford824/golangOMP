<template>
  <span
    class="base-spinner"
    :class="{ 'base-spinner--inline': inline }"
    :style="spinnerStyle"
    role="status"
    :aria-label="label"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    size?: string | number
    color?: string
    label?: string
    inline?: boolean
  }>(),
  {
    size: 24,
    color: 'currentColor',
    label: '加载中',
    inline: false,
  },
)

const spinnerStyle = computed(() => {
  const size = typeof props.size === 'number' ? `${props.size}px` : props.size
  return {
    '--spinner-size': size,
    '--spinner-color': props.color,
  }
})
</script>

<style scoped>
.base-spinner {
  display: inline-block;
  width: var(--spinner-size);
  height: var(--spinner-size);
  flex: 0 0 auto;
  border: max(2px, calc(var(--spinner-size) / 10)) solid color-mix(in srgb, var(--spinner-color) 18%, transparent);
  border-top-color: var(--spinner-color);
  border-radius: 999px;
  color: var(--spinner-color);
  animation: base-spinner-rotate 0.72s linear infinite;
}

.base-spinner--inline {
  vertical-align: -0.15em;
}

@keyframes base-spinner-rotate {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .base-spinner {
    animation-duration: 1.8s;
  }
}
</style>
