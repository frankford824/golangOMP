<template>
  <div class="flex flex-col gap-1">
    <label v-if="label" :for="inputId" class="text-sm font-medium text-[rgb(var(--yb-text-muted-strong))]">
      {{ label }}
    </label>
    <input
      :id="inputId"
      :type="type"
      :value="modelValue"
      :placeholder="placeholder"
      :disabled="disabled"
      :maxlength="maxlength"
      :aria-invalid="error ? 'true' : undefined"
      :aria-describedby="describedBy"
      class="w-full h-11 rounded-xl border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface-soft)_/_0.8)] px-3 text-sm text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-placeholder))] outline-none transition focus:border-[rgb(var(--yb-brand-border-strong))] focus:ring-1 focus:ring-[rgb(var(--yb-brand-accent)_/_0.35)] disabled:cursor-not-allowed disabled:bg-[rgb(var(--yb-surface-muted))] disabled:text-[rgb(var(--yb-text-faint))]"
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

const props = withDefaults(
  defineProps<{
    modelValue?: string | number
    label?: string
    placeholder?: string
    type?: string
    disabled?: boolean
    maxlength?: number | string
    hint?: string
    error?: string
  }>(),
  {
    modelValue: '',
    type: 'text',
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [string | number]
}>()

const uid = useId()
const inputId = `${uid}-input`
const hintId = `${uid}-hint`
const errorId = `${uid}-error`
const describedBy = computed(() => {
  const ids: string[] = []
  if (props.error) ids.push(errorId)
  if (props.hint) ids.push(hintId)
  return ids.length ? ids.join(' ') : undefined
})

function onInput(e: Event) {
  const target = e.target as HTMLInputElement
  emit('update:modelValue', target.value)
}
</script>
