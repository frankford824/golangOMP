<template>
  <!-- v4.2 修复：老板要求 + 弹窗统一脱离页面 stacking context，避免被内容层或滚动容器干扰 -->
  <Teleport to="body">
    <transition name="fade-scale">
      <div
        v-if="modelValue"
        class="fixed inset-0 z-[3000] flex items-center justify-center bg-stone-900/40 p-3 sm:p-4"
        role="dialog"
        aria-modal="true"
      >
        <div
          :class="[
            'w-full max-w-[calc(100vw-1.5rem)] sm:max-w-[calc(100vw-2rem)] max-h-[88dvh] rounded-2xl border border-stone-200/80 bg-white shadow-float overflow-hidden flex flex-col',
            panelClass,
          ]"
        >
          <header class="flex-shrink-0 px-4 sm:px-5 pt-4 sm:pt-5 pb-3 sm:pb-4 flex items-center justify-between gap-3">
            <h2 class="min-w-0 text-base font-headline font-bold text-slate-900">
              {{ title }}
            </h2>
            <button
              type="button"
              class="modal-close-btn text-slate-400 hover:text-slate-600"
              aria-label="关闭"
              @click="close"
            >
              ×
            </button>
          </header>
          <div class="flex-1 min-h-0 overflow-y-auto px-4 sm:px-5 pb-1 text-sm text-slate-700">
            <slot />
          </div>
          <slot name="footer">
            <footer class="flex-shrink-0 flex flex-wrap justify-end gap-2 px-4 sm:px-5 py-3 sm:py-4 border-t border-slate-100">
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

/* Light admin modal skin. Style-only. */
.fixed.inset-0 {
  z-index: 7100 !important;
  background: rgba(15, 23, 42, 0.18) !important;
}

.fixed.inset-0 > div {
  border-color: #e5e7eb !important;
  background: #ffffff !important;
  color: #374151 !important;
  box-shadow: 0 20px 40px -12px rgba(15, 23, 42, 0.18) !important;
}

h2 {
  color: #111827 !important;
}

.flex-1 {
  color: #374151 !important;
}

footer {
  border-color: #e5e7eb !important;
}

button {
  color: inherit;
}

.modal-close-btn {
  display: inline-flex;
  width: 2rem;
  height: 2rem;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border: 1px solid transparent;
  border-radius: 0.6rem;
  background: transparent;
  font-size: 1.35rem;
  line-height: 1;
}

.modal-close-btn:hover {
  border-color: #e5e7eb;
  background: #f9fafb;
}

@media (max-width: 640px) {
  .fixed.inset-0 {
    align-items: flex-end;
  }

  .fixed.inset-0 > div {
    max-height: calc(100dvh - 1rem) !important;
    border-radius: 1rem 1rem 0.75rem 0.75rem !important;
  }

  footer :deep(button) {
    flex: 1 1 8rem;
  }
}

/* Teleport 到 body 后在 #app 外，需局部复用 main.css 蓝色主按钮皮肤 */
footer :deep(button.bg-stone-600) {
  background: #2563eb !important;
  border-color: #2563eb !important;
  color: #ffffff !important;
  box-shadow: 0 1px 2px rgba(37, 99, 235, 0.2) !important;
}

footer :deep(button.bg-stone-600:not(:disabled):hover) {
  background: #1d4ed8 !important;
  border-color: #1d4ed8 !important;
  color: #ffffff !important;
}

footer :deep(button.bg-stone-600:not(:disabled):active) {
  background: #1e40af !important;
  border-color: #1e40af !important;
  color: #ffffff !important;
}

footer :deep(button.bg-stone-600:focus-visible) {
  outline: none;
  box-shadow:
    0 1px 2px rgba(37, 99, 235, 0.2),
    0 0 0 3px rgba(37, 99, 235, 0.15) !important;
}

footer :deep(button.bg-stone-600:disabled) {
  cursor: not-allowed;
  opacity: 0.6;
}
</style>
