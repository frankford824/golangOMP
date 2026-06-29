<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref } from 'vue'
import { Maximize2, X } from 'lucide-vue-next'

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
const layerRef = ref<HTMLElement | null>(null)
const backdropRef = ref<HTMLElement | null>(null)
const surfaceRef = ref<HTMLElement | null>(null)
const contentRef = ref<HTMLElement | null>(null)
const closeBtnRef = ref<HTMLButtonElement | null>(null)

let originEl: HTMLElement | null = null
let lastFocused: HTMLElement | null = null
let busy = false

const OPEN_EASE = 'cubic-bezier(0.2, 0, 0, 1)'
const CLOSE_EASE = 'cubic-bezier(0.4, 0, 1, 1)'

function prefersReducedMotion() {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

function segmentId(segment: LedgerSegment, index: number) {
  return segment.key || `seg-${index}`
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.stopPropagation()
    void closeSegment()
  }
}

async function openSegment(segment: LedgerSegment, event: MouseEvent) {
  if (activeSegment.value || busy) return
  busy = true
  originEl = event.currentTarget as HTMLElement
  lastFocused = (document.activeElement as HTMLElement) ?? null
  activeSegment.value = segment
  document.body.classList.add('aw-ledger-locked')
  window.addEventListener('keydown', onKeydown, true)

  await nextTick()
  const surface = surfaceRef.value
  const backdrop = backdropRef.value
  closeBtnRef.value?.focus()

  if (backdrop) {
    backdrop.animate([{ opacity: 0 }, { opacity: 1 }], { duration: 240, easing: 'ease', fill: 'both' })
  }

  if (!surface || !originEl || prefersReducedMotion()) {
    busy = false
    return
  }

  const first = originEl.getBoundingClientRect()
  const last = surface.getBoundingClientRect()
  const dx = first.left - last.left
  const dy = first.top - last.top
  const sx = Math.max(0.0001, first.width / last.width)
  const sy = Math.max(0.0001, first.height / last.height)

  surface.style.transformOrigin = 'top left'
  const surfaceAnim = surface.animate(
    [
      { transform: `translate(${dx}px, ${dy}px) scale(${sx}, ${sy})` },
      { transform: 'translate(0, 0) scale(1, 1)' },
    ],
    { duration: 400, easing: OPEN_EASE, fill: 'both' },
  )

  // The body table reveal is a static motion, owned by recipes.css
  // (.aw-ledger-sheet__body animation) so business source stays token-only.

  surfaceAnim.finished.finally(() => {
    busy = false
  })
}

async function closeSegment() {
  if (!activeSegment.value || busy) return
  busy = true
  window.removeEventListener('keydown', onKeydown, true)

  const surface = surfaceRef.value
  const backdrop = backdropRef.value
  const tasks: Promise<unknown>[] = []

  if (backdrop) {
    tasks.push(backdrop.animate([{ opacity: 1 }, { opacity: 0 }], { duration: 220, easing: 'ease', fill: 'both' }).finished.catch(() => {}))
  }

  if (surface && originEl && !prefersReducedMotion()) {
    const first = originEl.getBoundingClientRect()
    const last = surface.getBoundingClientRect()
    const dx = first.left - last.left
    const dy = first.top - last.top
    const sx = Math.max(0.0001, first.width / last.width)
    const sy = Math.max(0.0001, first.height / last.height)
    surface.style.transformOrigin = 'top left'
    contentRef.value?.animate([{ opacity: 1 }, { opacity: 0 }], { duration: 130, easing: CLOSE_EASE, fill: 'both' })
    tasks.push(
      surface
        .animate(
          [
            { transform: 'translate(0, 0) scale(1, 1)' },
            { transform: `translate(${dx}px, ${dy}px) scale(${sx}, ${sy})` },
          ],
          { duration: 300, easing: CLOSE_EASE, fill: 'both' },
        )
        .finished.catch(() => {}),
    )
  }

  await Promise.all(tasks)
  teardown()
}

function teardown() {
  activeSegment.value = null
  originEl = null
  document.body.classList.remove('aw-ledger-locked')
  lastFocused?.focus?.()
  lastFocused = null
  busy = false
}

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown, true)
  document.body.classList.remove('aw-ledger-locked')
})
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
    <div v-if="activeSegment" ref="layerRef" class="aw-ledger-sheet-layer">
      <div ref="backdropRef" class="aw-ledger-sheet__backdrop" @click="closeSegment" />
      <div
        ref="surfaceRef"
        class="aw-ledger-sheet"
        role="dialog"
        aria-modal="true"
        :aria-label="`${activeSegment.label} 明细`"
      >
        <header class="aw-ledger-sheet__head">
          <div class="aw-ledger-sheet__head-copy">
            <p class="aw-eyebrow">{{ eyebrow }} · {{ activeSegment.label }}</p>
            <p class="aw-ledger-sheet__value" :class="{ 'is-money': activeSegment.money }">
              {{ activeSegment.value }}
            </p>
            <p v-if="activeSegment.hint" class="aw-ledger-sheet__hint">{{ activeSegment.hint }}</p>
          </div>
          <button ref="closeBtnRef" class="aw-ledger-sheet__close" type="button" aria-label="收起明细" @click="closeSegment">
            <X :size="18" aria-hidden="true" />
          </button>
        </header>
        <div ref="contentRef" class="aw-ledger-sheet__body">
          <slot name="detail" :segment="activeSegment">
            <p class="aw-copy">这一项暂无展开明细。</p>
          </slot>
        </div>
      </div>
    </div>
  </Teleport>
</template>
