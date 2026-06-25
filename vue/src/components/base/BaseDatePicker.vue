<template>
  <div class="flex flex-col gap-1">
    <label v-if="label" :for="dateId" class="text-sm font-medium text-[rgb(var(--yb-text-muted-strong))]">
      {{ label }}
    </label>
    <input
      :id="dateId"
      type="date"
      :value="displayValue"
      :disabled="disabled"
      :min="min"
      :max="max"
      :aria-label="label ? undefined : '日期选择'"
      :aria-invalid="error ? 'true' : undefined"
      :aria-describedby="describedBy"
      class="w-full h-11 rounded-lg border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface-soft)_/_0.8)] px-3 text-sm text-[rgb(var(--yb-text))] shadow-sm outline-none transition focus:border-[rgb(var(--yb-brand-border-strong))] focus:ring-2 focus:ring-[rgb(var(--yb-brand-accent)_/_0.24)] disabled:cursor-not-allowed disabled:bg-[rgb(var(--yb-surface-muted))] disabled:text-[rgb(var(--yb-text-faint))]"
      @input="onInput"
    />
    <p v-if="hint" :id="hintId" class="text-xs text-[rgb(var(--yb-text-faint))]">
      {{ hint }}
    </p>
    <p v-if="error" :id="errorId" class="text-xs text-[rgb(var(--yb-danger))]">
      {{ error }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, useId } from 'vue'
import { toBeijingDateInputValue } from '@/utils/date'

const props = withDefaults(
  defineProps<{
    modelValue?: string
    label?: string
    disabled?: boolean
    hint?: string
    error?: string
    min?: string
    max?: string
  }>(),
  {
    modelValue: '',
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [string]
}>()

const uid = useId()
const dateId = `${uid}-date`
const hintId = `${uid}-hint`
const errorId = `${uid}-error`
const describedBy = computed(() => {
  const ids: string[] = []
  if (props.error) ids.push(errorId)
  if (props.hint) ids.push(hintId)
  return ids.length ? ids.join(' ') : undefined
})

const displayValue = computed(() => {
  const v = props.modelValue
  return toBeijingDateInputValue(v)
})

function onInput(e: Event) {
  const target = e.target as HTMLInputElement
  const val = target.value
  if (!val) {
    emit('update:modelValue', '')
    return
  }
  emit('update:modelValue', val)
}
</script>
