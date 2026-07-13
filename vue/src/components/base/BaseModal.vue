<template>
  <!-- v4.2 修复：老板要求 + 弹窗统一脱离页面 stacking context，避免被内容层或滚动容器干扰 -->
  <Teleport to="body">
    <transition name="fade-scale">
      <div
        v-if="modelValue"
        class="fixed inset-0 z-[7100] flex items-center justify-center bg-[rgb(var(--yb-shadow)_/_0.18)] p-3 sm:p-4"
        role="dialog"
        aria-modal="true"
      >
        <div
          ref="panelRef"
          tabindex="-1"
          :class="[
            'w-full max-w-[calc(100vw-1.5rem)] sm:max-w-[calc(100vw-2rem)] max-h-[88dvh] rounded-2xl border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface))] text-[rgb(var(--yb-text-body))] shadow-float overflow-hidden flex flex-col focus:outline-none',
            panelClass,
          ]"
          :style="panelMaxWidth ? { maxWidth: panelMaxWidth } : undefined"
        >
          <header class="flex-shrink-0 px-4 sm:px-5 pt-4 sm:pt-5 pb-3 sm:pb-4 flex items-center justify-between gap-3">
            <h2 class="min-w-0 text-base font-headline font-bold text-[rgb(var(--yb-text))]">
              {{ title }}
            </h2>
            <button
              type="button"
              class="modal-close-btn text-[rgb(var(--yb-text-faint))] hover:text-[rgb(var(--yb-text-muted-strong))]"
              aria-label="关闭"
              @click="close"
            >
              ×
            </button>
          </header>
          <div class="flex-1 min-h-0 overflow-y-auto px-4 sm:px-5 pb-1 text-sm text-[rgb(var(--yb-text-body))]">
            <slot />
          </div>
          <slot name="footer">
            <footer class="flex-shrink-0 flex flex-wrap justify-end gap-2 px-4 sm:px-5 py-3 sm:py-4 border-t border-[rgb(var(--yb-border))]">
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
import { ref, toRef } from 'vue'
import BaseButton from './BaseButton.vue'
import { useModalA11y } from '../../composables/useModalA11y'

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
    /** 需要精确桌面宽度时使用；内联 max-width 可稳定覆盖响应式宽度类。 */
    panelMaxWidth?: string
  }>(),
  {
    title: '',
    showConfirm: true,
    confirmText: '确认',
    cancelText: '取消',
    customFooter: false,
    panelClass: 'max-w-3xl',
    panelMaxWidth: '',
  },
)

const emit = defineEmits<{
  'update:modelValue': [boolean]
  confirm: []
  cancel: []
}>()

const panelRef = ref<HTMLElement | null>(null)

function close() {
  emit('update:modelValue', false)
  emit('cancel')
}

// Focus trap + Esc + initial/returned focus + stacked scroll-lock counter.
useModalA11y(toRef(props, 'modelValue'), panelRef, { onEsc: close })

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
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
}

@media (max-width: 640px) {
  .fixed.inset-0 {
    align-items: flex-end;
  }

  .fixed.inset-0 > div {
    max-height: calc(100dvh - 1rem);
    border-radius: 1rem 1rem 0.75rem 0.75rem;
  }

  footer :deep(button) {
    flex: 1 1 8rem;
  }
}

</style>
