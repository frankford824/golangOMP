<template>
  <div class="flex flex-col gap-1">
    <label v-if="label" class="text-sm font-medium text-slate-600">
      {{ label }}
    </label>
    <input
      type="date"
      :value="displayValue"
      :disabled="disabled"
      :min="min"
      :max="max"
      class="w-full h-11 rounded-lg border border-slate-200 bg-slate-50 px-3 text-sm text-slate-700 shadow-sm outline-none transition focus:border-slate-400 focus:ring-2 focus:ring-slate-900/5 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400"
      @input="onInput"
    />
    <p v-if="hint" class="text-xs text-slate-400">
      {{ hint }}
    </p>
    <p v-if="error" class="text-xs text-red-600">
      {{ error }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
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
