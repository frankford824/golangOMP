<template>
  <span v-if="normalizedLane" class="lane-tag" :class="[laneClass, sizeClass]">
    <span v-if="normalizedLane === 'customization'" class="lane-tag__icon" aria-hidden="true">●</span>
    <span class="lane-tag__label">{{ laneLabel }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

type Lane = 'normal' | 'customization'

const props = withDefaults(
  defineProps<{
    lane?: Lane | string | null
    size?: 'sm' | 'md'
  }>(),
  {
    lane: null,
    size: 'sm',
  },
)

const normalizedLane = computed<Lane | ''>(() => {
  const raw = String(props.lane ?? '').trim().toLowerCase()
  if (raw === 'customization') return 'customization'
  if (raw === 'normal') return 'normal'
  return ''
})

const laneLabel = computed(() => {
  if (normalizedLane.value === 'customization') return '定制'
  if (normalizedLane.value === 'normal') return '常规'
  return ''
})

const laneClass = computed(() =>
  normalizedLane.value === 'customization' ? 'is-customization' : 'is-normal',
)

const sizeClass = computed(() => (props.size === 'md' ? 'size-md' : 'size-sm'))
</script>

<style scoped>
.lane-tag {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  border-radius: 999px;
  font-weight: 600;
  line-height: 1.2;
  white-space: nowrap;
}

.lane-tag.size-sm {
  padding: 0.125rem 0.5rem;
  font-size: 0.6875rem;
}

.lane-tag.size-md {
  padding: 0.2rem 0.65rem;
  font-size: 0.8125rem;
}

.lane-tag__icon {
  font-size: 0.55em;
  line-height: 1;
  display: inline-flex;
  align-items: center;
}

.lane-tag__label {
  display: inline-flex;
  align-items: center;
}

.lane-tag.is-customization {
  background: #7c3aed;
  color: #fff;
  border: 1px solid #7c3aed;
}

.lane-tag.is-normal {
  background: transparent;
  color: #475569;
  border: 1px solid #cbd5e1;
}
</style>
