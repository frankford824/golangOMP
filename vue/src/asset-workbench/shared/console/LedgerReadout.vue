<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { Maximize2, X } from 'lucide-vue-next'
import { DialogClose, DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'

import { animateOpacity, ledgerCloseMotion, ledgerOpenMotion } from '@aw/shared/motion/useFlip'
import { prefersReducedMotion, readAssetMotionTokens } from '@aw/shared/motion/tokens'
import { useScrollLock } from '@aw/shared/motion/useScrollLock'

interface LedgerSegment {
  key?: string
  label: string
  value: string | number
  hint?: string
  money?: boolean
  expandable?: boolean
}

defineProps<{
  eyebrow: string
  title: string
  segments: LedgerSegment[]
}>()

const activeSegment = ref<LedgerSegment | null>(null)
const backdropRef = ref<HTMLElement | null>(null)
const surfaceRef = ref<HTMLElement | null>(null)
const contentRef = ref<HTMLElement | null>(null)
const closeBtnRef = ref<HTMLButtonElement | null>(null)
const { lock, unlock } = useScrollLock('aw-ledger-locked')

let originEl: HTMLElement | null = null
let lastFocused: HTMLElement | null = null
let busy = false

function segmentId(segment: LedgerSegment, index: number) {
  return segment.key || `seg-${index}`
}

async function openSegment(segment: LedgerSegment, event: MouseEvent) {
  if (activeSegment.value || busy) return
  busy = true
  originEl = event.currentTarget as HTMLElement
  lastFocused = (document.activeElement as HTMLElement) ?? null
  activeSegment.value = segment
  lock()

  await nextTick()
  const surface = surfaceRef.value
  const backdrop = backdropRef.value
  void focusCloseButton()
  const motion = readAssetMotionTokens(surface)

  if (backdrop && !prefersReducedMotion()) {
    animateOpacity(backdrop, 0, 1, motion.standard, 'ease')
  }

  if (!surface || !originEl || prefersReducedMotion()) {
    busy = false
    return
  }

  const surfaceAnim = ledgerOpenMotion(surface, originEl)

  // The body table reveal is a static motion, owned by recipes.css
  // (.aw-ledger-sheet__body animation) so business source stays token-only.

  surfaceAnim.finished.catch(() => {}).finally(() => {
    busy = false
  })
}

function onDialogOpenChange(open: boolean) {
  if (!open) void closeSegment()
}

async function focusCloseButton() {
  await nextTick()
  const focus = () => {
    const closeButton =
      (closeBtnRef.value instanceof HTMLButtonElement ? closeBtnRef.value : null) ??
      document.querySelector<HTMLButtonElement>('.aw-ledger-sheet__close')
    closeButton?.focus()
  }
  window.requestAnimationFrame(focus)
  window.setTimeout(focus, readAssetMotionTokens().fast)
}

async function closeSegment() {
  if (!activeSegment.value || busy) return
  busy = true

  const surface = surfaceRef.value
  const backdrop = backdropRef.value
  const tasks: Promise<unknown>[] = []
  const motion = readAssetMotionTokens(surface)

  if (backdrop && !prefersReducedMotion()) {
    tasks.push(animateOpacity(backdrop, 1, 0, motion.exit, 'ease').finished.catch(() => {}))
  }

  if (surface && originEl && !prefersReducedMotion()) {
    if (contentRef.value) {
      animateOpacity(contentRef.value, 1, 0, motion.shift, motion.easeExit)
    }
    tasks.push(ledgerCloseMotion(surface, originEl).finished.catch(() => {}))
  }

  await Promise.all(tasks)
  teardown()
}

function teardown() {
  activeSegment.value = null
  originEl = null
  unlock()
  lastFocused?.focus?.()
  lastFocused = null
  busy = false
}

onBeforeUnmount(() => {
  unlock()
})

watch(
  activeSegment,
  (segment) => {
    if (segment) {
      void focusCloseButton()
    }
  },
  { flush: 'post' },
)
</script>

<template>
  <section class="aw-console-hero">
    <div class="aw-console-hero__head">
      <div>
        <p class="aw-eyebrow">{{ eyebrow }}</p>
        <h2 class="aw-console-hero__title">{{ title }}</h2>
      </div>
      <div class="aw-console-hero__actions">
        <slot name="actions" />
      </div>
    </div>

    <div class="aw-ledger">
      <component
        :is="segment.expandable ? 'button' : 'div'"
        v-for="(segment, index) in segments"
        :key="segmentId(segment, index)"
        class="aw-ledger__cell"
        :class="{
          'aw-ledger__cell--money': segment.money,
          'aw-ledger__cell--expandable': segment.expandable,
        }"
        :type="segment.expandable ? 'button' : undefined"
        @click="segment.expandable ? openSegment(segment, $event) : undefined"
      >
        <span class="aw-ledger__label">
          {{ segment.label }}
          <Maximize2 v-if="segment.expandable" :size="12" class="aw-ledger__expand-hint" aria-hidden="true" />
        </span>
        <span class="aw-ledger__value">
          {{ segment.value }}
        </span>
        <span v-if="segment.hint" class="aw-ledger__hint">{{ segment.hint }}</span>
      </component>
    </div>
  </section>

  <Teleport to="body">
    <DialogRoot :open="Boolean(activeSegment)" @update:open="onDialogOpenChange">
      <DialogPortal>
        <div v-if="activeSegment" class="aw-token-scope aw-ledger-sheet-layer">
          <DialogOverlay as-child>
            <div ref="backdropRef" class="aw-ledger-sheet__backdrop" @click="closeSegment" />
          </DialogOverlay>
          <DialogContent
            as-child
            :aria-label="`${activeSegment.label} 明细`"
            @open-auto-focus.prevent="focusCloseButton"
            @close-auto-focus.prevent
          >
            <div ref="surfaceRef" class="aw-ledger-sheet">
              <header class="aw-ledger-sheet__head">
                <div class="aw-ledger-sheet__head-copy">
                  <DialogTitle as-child>
                    <p class="aw-eyebrow">{{ eyebrow }} · {{ activeSegment.label }}</p>
                  </DialogTitle>
                  <p class="aw-ledger-sheet__value" :class="{ 'is-money': activeSegment.money }">
                    {{ activeSegment.value }}
                  </p>
                  <p v-if="activeSegment.hint" class="aw-ledger-sheet__hint">{{ activeSegment.hint }}</p>
                  <DialogDescription as-child>
                    <p class="aw-visually-hidden">{{ activeSegment.hint || `${activeSegment.label} 明细` }}</p>
                  </DialogDescription>
                </div>
                <DialogClose as-child>
                  <button ref="closeBtnRef" class="aw-ledger-sheet__close" type="button" aria-label="收起明细" autofocus>
                    <X :size="18" aria-hidden="true" />
                  </button>
                </DialogClose>
              </header>
              <div ref="contentRef" class="aw-ledger-sheet__body">
                <slot name="detail" :segment="activeSegment">
                  <p class="aw-copy">这一项暂无展开明细。</p>
                </slot>
              </div>
            </div>
          </DialogContent>
        </div>
      </DialogPortal>
    </DialogRoot>
  </Teleport>
</template>
