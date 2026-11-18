<script setup lang="ts">
defineProps<{ label: string; min: string; max: string }>()
const value = defineModel<string>({ required: true })
</script>

<template>
  <span class="order-date-field">
    <input v-model="value" type="date" :aria-label="label" :min="min" :max="max" required />
    <span class="order-date-value" aria-hidden="true">{{ value || '—' }}</span>
  </span>
</template>

<style scoped>
.order-date-field { display: inline-flex; position: relative; min-width: 0; }
.order-date-value {
  position: absolute;
  inset-block: 3px;
  inset-inline: 4px 24px;
  display: grid;
  align-items: center;
  pointer-events: none;
  background: var(--oa-surface);
  color: var(--oa-ink);
  font: 400 13px/1.5 var(--yb-font-data, monospace);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.order-date-field:focus-within .order-date-value { display: none; }
</style>
