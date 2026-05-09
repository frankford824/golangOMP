<template>
  <button
    type="button"
    class="inline-flex items-center rounded-full border border-transparent px-1 py-0.5 text-xs transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-600 focus-visible:ring-offset-2"
    :class="modelValue ? 'bg-emerald-600 text-white' : 'bg-slate-200 text-slate-600'"
    :aria-pressed="modelValue"
    @click="toggle"
  >
    <span
      class="flex h-5 w-7 shrink-0 items-center overflow-hidden rounded-full"
      aria-hidden
    >
      <span
        class="inline-block h-4 w-4 shrink-0 rounded-full bg-white shadow transition"
        :class="modelValue ? 'translate-x-3' : 'translate-x-0'"
      />
    </span>
    <span class="ml-1">
      <slot />
    </span>
  </button>
</template>

<script setup lang="ts">
const props = defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [boolean]
}>()

function toggle() {
  emit('update:modelValue', !props.modelValue)
}
</script>

