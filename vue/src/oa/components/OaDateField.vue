<script setup lang="ts">
import { NDatePicker } from 'naive-ui'

const value = defineModel<string>({ required: true })
const props = defineProps<{ label: string; required?: boolean; max?: string; error?: string; hint?: string }>()

function update(date: string | [string, string] | null) {
  value.value = typeof date === 'string' ? date : ''
}

function afterLimit(timestamp: number) {
  if (!props.max) return false
  const date = new Date(timestamp)
  const key = [
    String(date.getFullYear()).padStart(4, '0'),
    String(date.getMonth() + 1).padStart(2, '0'),
    String(date.getDate()).padStart(2, '0'),
  ].join('-')
  return key > props.max
}
</script>

<template>
  <label class="oa-date-field">
    <span>{{ label }}<i v-if="required" aria-hidden="true">*</i></span>
    <NDatePicker
      type="date"
      :formatted-value="value || null"
      value-format="yyyy-MM-dd"
      format="yyyy-MM-dd"
      placeholder="选择日期"
      :is-date-disabled="afterLimit"
      :status="error ? 'error' : undefined"
      clearable
      @update:formatted-value="update"
    />
    <small v-if="hint" class="oa-muted">{{ hint }}</small>
    <small v-if="error" class="fee-error" role="alert">{{ error }}</small>
  </label>
</template>

<style scoped>
.oa-date-field { min-width: 0; }
.oa-date-field :deep(.n-input__input-el) { min-height: var(--n-height); border: 0; background: transparent; }
</style>
