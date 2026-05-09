<template>
  <button
    :type="type"
    class="inline-flex items-center justify-center rounded-xl text-sm font-medium font-headline transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-stone-300 focus-visible:ring-offset-2 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-60"
    :class="buttonClass"
    :disabled="disabled || loading"
    @click="onClick"
  >
    <span
      v-if="loading"
      class="mr-1 inline-block h-4 w-4 animate-spin rounded-full border-2 border-transparent border-t-current"
    />
    <span><slot /></span>
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    variant?: 'primary' | 'secondary' | 'ghost' | 'danger' | 'outsource'
    size?: 'sm' | 'md'
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
    loading?: boolean
  }>(),
  {
    variant: 'primary',
    size: 'md',
    type: 'button',
    disabled: false,
    loading: false,
  },
)

const emit = defineEmits<{
  click: [MouseEvent]
}>()

const basePrimary =
  'border border-stone-600 bg-stone-600 text-stone-50 hover:bg-stone-500 hover:border-stone-500'
const baseSecondary =
  'border border-stone-200 bg-stone-50 text-stone-800 hover:bg-stone-100 hover:border-stone-300'
const baseGhost =
  'border border-transparent bg-transparent text-stone-600 hover:bg-stone-50 hover:border-transparent'
const baseDanger =
  'border border-red-600 bg-red-600 text-white hover:bg-red-700 hover:border-red-700'
const baseOutsource =
  'border border-amber-300 bg-amber-50 text-amber-800 hover:bg-amber-100'

const sizeSm = 'h-8 px-3 text-xs'
const sizeMd = 'h-10 px-3.5 text-sm'

const buttonClass = computed(() => {
  const variantClass =
    props.variant === 'primary'
      ? basePrimary
      : props.variant === 'secondary'
      ? baseSecondary
      : props.variant === 'ghost'
      ? baseGhost
      : props.variant === 'outsource'
      ? baseOutsource
      : baseDanger

  const sizeClass = props.size === 'sm' ? sizeSm : sizeMd
  return `${variantClass} ${sizeClass}`
})

function onClick(e: MouseEvent) {
  if (props.disabled || props.loading) return
  emit('click', e)
}
</script>

