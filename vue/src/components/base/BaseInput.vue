<template>
  <div class="flex flex-col gap-1">
    <label v-if="label" class="text-sm font-medium text-slate-600">
      {{ label }}
    </label>
    <input
      :type="type"
      :value="modelValue"
      :placeholder="placeholder"
      :disabled="disabled"
      class="w-full h-11 rounded-xl border border-stone-200 bg-stone-50/80 px-3 text-sm text-stone-800 placeholder:text-stone-400 outline-none transition focus:border-stone-400 focus:ring-1 focus:ring-stone-300 disabled:cursor-not-allowed disabled:bg-stone-100 disabled:text-stone-400"
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
withDefaults(
  defineProps<{
    modelValue?: string | number
    label?: string
    placeholder?: string
    type?: string
    disabled?: boolean
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

function onInput(e: Event) {
  const target = e.target as HTMLInputElement
  emit('update:modelValue', target.value)
}
</script>

