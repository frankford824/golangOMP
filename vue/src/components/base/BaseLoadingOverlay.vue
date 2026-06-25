<template>
  <Teleport v-if="teleport" to="body">
    <transition name="loading-fade">
      <div v-if="active" :class="overlayClass" role="status" aria-live="polite">
        <div class="loading-panel">
          <BaseSpinner :size="spinnerSize" color="#2563eb" />
          <div v-if="label || description" class="loading-copy">
            <strong v-if="label">{{ label }}</strong>
            <span v-if="description">{{ description }}</span>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
  <transition v-else name="loading-fade">
    <div v-if="active" :class="overlayClass" role="status" aria-live="polite">
      <div class="loading-panel">
        <BaseSpinner :size="spinnerSize" color="#2563eb" />
        <div v-if="label || description" class="loading-copy">
          <strong v-if="label">{{ label }}</strong>
          <span v-if="description">{{ description }}</span>
        </div>
      </div>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import BaseSpinner from './BaseSpinner.vue'

const props = withDefaults(
  defineProps<{
    active?: boolean
    fixed?: boolean
    teleport?: boolean
    label?: string
    description?: string
    spinnerSize?: string | number
    dim?: boolean
  }>(),
  {
    active: true,
    fixed: false,
    teleport: false,
    label: '',
    description: '',
    spinnerSize: 28,
    dim: true,
  },
)

const overlayClass = computed(() => [
  'loading-overlay',
  props.fixed ? 'loading-overlay--fixed' : 'loading-overlay--absolute',
  props.dim ? 'loading-overlay--dim' : 'loading-overlay--clear',
])
</script>

<style scoped>
.loading-overlay {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
  pointer-events: auto;
}

.loading-overlay--fixed {
  position: fixed;
  inset: 0;
  z-index: 7600;
}

.loading-overlay--absolute {
  position: absolute;
  inset: 0;
  z-index: 20;
  border-radius: inherit;
}

.loading-overlay--dim {
  background: rgba(248, 250, 252, 0.78);
  backdrop-filter: blur(2px);
}

.loading-overlay--clear {
  background: transparent;
}

.loading-panel {
  display: inline-flex;
  max-width: min(24rem, calc(100vw - 2rem));
  align-items: center;
  gap: 0.75rem;
  border: 1px solid #dbeafe;
  border-radius: 0.75rem;
  background: #ffffff;
  padding: 0.75rem 0.9rem;
  color: #111827;
  box-shadow: 0 18px 36px -24px rgba(15, 23, 42, 0.42);
}

.loading-copy {
  display: grid;
  min-width: 0;
  gap: 0.15rem;
}

.loading-copy strong {
  color: #111827;
  font-size: 0.85rem;
  font-weight: 800;
  line-height: 1.25;
}

.loading-copy span {
  color: #6b7280;
  font-size: 0.75rem;
  line-height: 1.35;
}

.loading-fade-enter-active,
.loading-fade-leave-active {
  transition: opacity 0.14s ease;
}

.loading-fade-enter-from,
.loading-fade-leave-to {
  opacity: 0;
}

@media (max-width: 640px) {
  .loading-panel {
    width: 100%;
    justify-content: center;
  }
}

@media (prefers-reduced-transparency: reduce) {
  .loading-overlay--dim {
    backdrop-filter: none;
  }
}
</style>
