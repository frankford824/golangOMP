<script setup lang="ts">
import { ref } from 'vue'

import { useReveal } from '@aw/shared/motion/useReveal'

const root = ref<HTMLElement | null>(null)

useReveal(root)
</script>

<template>
  <div ref="root" class="aw-motion-reveal">
    <slot />
  </div>
</template>
