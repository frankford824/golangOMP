<template>
  <div
    class="kpi-card ring-1 ring-slate-200/70 shadow-sm shadow-slate-900/5"
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
  background: #ffffff;
  border-radius: 0.75rem;
}
.kpi-card--clickable {
  cursor: pointer;
  transition: background-color 0.15s ease, box-shadow 0.15s ease;
}
.kpi-card--clickable:hover {
  background: #ffffff;
  box-shadow:
    0 2px 10px rgba(15, 23, 42, 0.08),
    0 0 0 1px rgb(203 213 225 / 0.6);
}
.kpi-card__label {
  margin: 0;
  font-size: 0.75rem;
  font-weight: 500;
  color: #64748b;
}
.kpi-card__value {
  margin: 0;
  font-size: 2rem;
  font-weight: 700;
  line-height: 1.1;
  letter-spacing: -0.02em;
  color: #0f172a;
  font-variant-numeric: tabular-nums;
}
.kpi-card__hint {
  margin: 0;
  font-size: 0.625rem;
  color: #94a3b8;
}
</style>
