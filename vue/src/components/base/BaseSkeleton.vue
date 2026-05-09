<template>
  <div
    class="animate-pulse bg-slate-100"
    :class="circle ? 'rounded-full' : 'rounded-lg'"
    :style="{ width, height }"
  />
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    width?: string
    height?: string
    circle?: boolean
  }>(),
  {
    width: '100%',
    height: '1rem',
    circle: false,
  },
)
</script>


