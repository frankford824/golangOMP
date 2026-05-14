<template>
  <!-- v4.2 修复：老板要求 + 弹窗统一脱离页面 stacking context，避免被内容层或滚动容器干扰 -->
  <Teleport to="body">
    <transition name="fade-scale">
      <div
        v-if="modelValue"
        class="fixed inset-0 z-[3000] flex items-center justify-center bg-stone-900/40"
        role="dialog"
        aria-modal="true"
      >
        <div
          :class="[
            'w-full max-h-[88vh] rounded-2xl border border-stone-200/80 bg-white shadow-float overflow-hidden flex flex-col',
            panelClass,
          ]"
        >
          <header class="flex-shrink-0 px-5 pt-5 pb-4 flex items-center justify-between">
            <h2 class="text-base font-headline font-bold text-slate-900">
              {{ title }}
            </h2>
            <button
              type="button"
              class="text-slate-400 hover:text-slate-600"
              @click="close"
            >
              ×
            </button>
          </header>
          <div class="flex-1 min-h-0 overflow-y-auto px-5 pb-1 text-sm text-slate-700">
            <slot />
          </div>
          <slot name="footer">
            <footer class="flex-shrink-0 flex justify-end gap-2 px-5 py-4 border-t border-slate-100">
              <BaseButton variant="secondary" size="sm" @click="close">
                {{ cancelText }}
              </BaseButton>
              <BaseButton
                v-if="showConfirm"
                variant="primary"
                size="sm"
                @click="confirm"
              >
                {{ confirmText }}
              </BaseButton>
            </footer>
          </slot>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup lang="ts">
import { watch, onUnmounted } from 'vue'
import BaseButton from './BaseButton.vue'

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    title?: string
    showConfirm?: boolean
    confirmText?: string
    cancelText?: string
    /** 为 true 时隐藏默认 footer（取消/确认），由调用方通过 #footer 插槽完全自定义 */
    customFooter?: boolean
    /** 覆盖默认 max-w-3xl，例如批量创建任务加宽 */
    panelClass?: string
  }>(),
  {
    title: '',
    showConfirm: true,
    confirmText: '确认',
    cancelText: '取消',
    customFooter: false,
    panelClass: 'max-w-3xl',
  },
)

const emit = defineEmits<{
  'update:modelValue': [boolean]
  confirm: []
  cancel: []
}>()

watch(
  () => props.modelValue,
  (open) => {
    if (typeof document === 'undefined') return
    document.body.style.overflow = open ? 'hidden' : ''
  },
  { immediate: true },
)

onUnmounted(() => {
  if (typeof document === 'undefined') return
  document.body.style.overflow = ''
})

function close() {
  emit('update:modelValue', false)
  emit('cancel')
}

function confirm() {
  emit('confirm')
}
</script>

<style scoped>
.fade-scale-enter-active,
.fade-scale-leave-active {
  transition: opacity 150ms ease-out, transform 150ms ease-out;
}

.fade-scale-enter-from,
.fade-scale-leave-to {
  opacity: 0;
  transform: scale(0.95);
}

/* Apple Music / iOS liquid glass modal skin. Style-only. */
.fixed.inset-0 {
  z-index: 7100 !important;
  background:
    radial-gradient(circle at 18% 5%, rgba(255, 45, 141, 0.18), transparent 30rem),
    radial-gradient(circle at 86% 0%, rgba(100, 210, 255, 0.14), transparent 32rem),
    rgba(0, 0, 0, 0.82) !important;
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
}

.fixed.inset-0 > div {
  border-color: var(--yb-music-border-strong) !important;
  background:
    radial-gradient(circle at 6% 0%, rgba(255, 45, 141, 0.08), transparent 18rem),
    radial-gradient(circle at 100% 0%, rgba(100, 210, 255, 0.08), transparent 20rem),
    linear-gradient(145deg, rgba(17, 24, 39, 0.99), rgba(7, 12, 20, 0.995)) !important;
  color: var(--yb-music-text-2) !important;
  box-shadow: 0 34px 90px -38px rgba(0, 0, 0, 0.95) !important;
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

h2 {
  color: #fff !important;
}

.flex-1 {
  color: var(--yb-music-text-2) !important;
}

footer {
  border-color: rgba(255, 255, 255, 0.12) !important;
}

button {
  color: var(--yb-music-text-2);
}
</style>
