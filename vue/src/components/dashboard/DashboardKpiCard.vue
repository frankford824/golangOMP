<template>
  <div
    class="kpi-card"
    :class="{ 'kpi-card--clickable': isClickable }"
    @click="navigate"
  >
    <p class="kpi-card__label">{{ title }}</p>
    <p class="kpi-card__value">{{ displayValue }}</p>
    <p v-if="hint" class="kpi-card__hint">{{ hint }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'

const props = withDefaults(
  defineProps<{
    title: string
    value: number | string
    hint?: string
    route?: string
  }>(),
  { hint: '', route: '' }
)

const router = useRouter()
const displayValue = computed(() =>
  typeof props.value === 'number' ? String(props.value) : props.value
)
const isClickable = computed(() => !!props.route)

function navigate() {
  if (props.route) void router.push(props.route)
}
</script>

<style scoped>
.kpi-card {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  min-height: 5.5rem;
  padding: 1.25rem;
  background: rgb(var(--yb-surface));
  border-radius: 0.75rem;
}
.kpi-card--clickable {
  cursor: pointer;
  transition: background-color 0.15s ease, box-shadow 0.15s ease;
}
.kpi-card--clickable:hover {
  background: rgb(var(--yb-surface));
  box-shadow:
    0 2px 10px rgb(var(--yb-shadow) / 0.08),
    0 0 0 1px rgb(var(--yb-text-disabled) / 0.6);
}
.kpi-card__label {
  margin: 0;
  font-size: 0.75rem;
  font-weight: 500;
  color: rgb(var(--yb-text-muted-strong));
}
.kpi-card__value {
  margin: 0;
  font-size: 2rem;
  font-weight: 700;
  line-height: 1.1;
  letter-spacing: -0.02em;
  color: rgb(var(--yb-text-navy));
  font-variant-numeric: tabular-nums;
}
.kpi-card__hint {
  margin: 0;
  font-size: 0.625rem;
  color: rgb(var(--yb-text-placeholder));
}
</style>
