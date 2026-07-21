<template>
  <button
    :type="type"
    class="inline-flex items-center justify-center rounded-xl text-sm font-medium font-headline transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgb(var(--yb-brand-accent)_/_0.45)] focus-visible:ring-offset-2 active:scale-[0.98] disabled:cursor-not-allowed disabled:border-[rgb(var(--yb-border))] disabled:bg-[rgb(var(--yb-surface-muted))] disabled:text-[rgb(var(--yb-text-faint))] disabled:shadow-none disabled:hover:border-[rgb(var(--yb-border))] disabled:hover:bg-[rgb(var(--yb-surface-muted))] disabled:hover:text-[rgb(var(--yb-text-faint))]"
    :class="buttonClass"
    :disabled="disabled || loading"
    @click="onClick"
  >
    <BaseSpinner
      v-if="loading"
      class="mr-1"
      :size="16"
      inline
    />
    <span><slot /></span>
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import BaseSpinner from './BaseSpinner.vue'

const props = withDefaults(
  defineProps<{
    variant?: 'primary' | 'secondary' | 'ghost' | 'danger'
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
  'border border-[rgb(var(--yb-brand))] bg-[rgb(var(--yb-brand))] text-[rgb(var(--yb-text-inverse))] hover:border-[rgb(var(--yb-brand-strong))] hover:bg-[rgb(var(--yb-brand-strong))]'
const baseSecondary =
  'border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface))] text-[rgb(var(--yb-text))] hover:border-[rgb(var(--yb-brand-border))] hover:bg-[rgb(var(--yb-brand-soft))] hover:text-[rgb(var(--yb-brand-strong))]'
const baseGhost =
  'border border-transparent bg-transparent text-[rgb(var(--yb-text-muted-strong))] hover:border-transparent hover:bg-[rgb(var(--yb-brand-soft))] hover:text-[rgb(var(--yb-brand))]'
const baseDanger =
  'border border-[rgb(var(--yb-danger))] bg-[rgb(var(--yb-danger))] text-[rgb(var(--yb-text-inverse))] hover:border-[rgb(var(--yb-danger-text))] hover:bg-[rgb(var(--yb-danger-text))]'
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
      : baseDanger

  const sizeClass = props.size === 'sm' ? sizeSm : sizeMd
  return `${variantClass} ${sizeClass}`
})

function onClick(e: MouseEvent) {
  if (props.disabled || props.loading) return
  emit('click', e)
}
</script>
