<template>
  <div class="flex flex-col gap-1">
    <label v-if="label" :for="textareaId" class="text-sm font-medium text-[rgb(var(--yb-text-muted-strong))]">
      {{ label }}
    </label>
    <textarea
      :id="textareaId"
      :value="modelValue"
      :rows="rows"
      :placeholder="placeholder"
      :disabled="disabled"
      :aria-label="label ? undefined : (placeholder || '多行文本输入')"
      :aria-invalid="error ? 'true' : undefined"
      :aria-describedby="describedBy"
      class="w-full min-h-[3rem] rounded-lg border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface-soft)_/_0.8)] px-3 py-2 text-sm text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-placeholder))] shadow-sm outline-none transition focus:border-[rgb(var(--yb-brand-border-strong))] focus:ring-2 focus:ring-[rgb(var(--yb-brand-accent)_/_0.24)] disabled:cursor-not-allowed disabled:bg-[rgb(var(--yb-surface-muted))] disabled:text-[rgb(var(--yb-text-faint))]"
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
    modelValue?: string
    label?: string
    placeholder?: string
    rows?: number
    disabled?: boolean
    hint?: string
    error?: string
  }>(),
  {
    modelValue: '',
    rows: 3,
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [string]
}>()

const uid = useId()
const textareaId = `${uid}-textarea`
const hintId = `${uid}-hint`
const errorId = `${uid}-error`
const describedBy = computed(() => {
  const ids: string[] = []
  if (props.error) ids.push(errorId)
  if (props.hint) ids.push(hintId)
  return ids.length ? ids.join(' ') : undefined
})

function onInput(e: Event) {
  const target = e.target as HTMLTextAreaElement
  emit('update:modelValue', target.value)
}
</script>
