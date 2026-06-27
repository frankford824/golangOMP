<script setup lang="ts">
interface LedgerSegment {
  key?: string
  label: string
  value: string | number
  hint?: string
  money?: boolean
}

defineProps<{
  eyebrow: string
  title: string
  segments: LedgerSegment[]
}>()
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
      <div
        v-for="segment in segments"
        :key="segment.key || segment.label"
        class="aw-ledger__cell"
        :class="{ 'aw-ledger__cell--money': segment.money }"
      >
        <span class="aw-ledger__label">{{ segment.label }}</span>
        <span class="aw-ledger__value">
          {{ segment.value }}
        </span>
        <span v-if="segment.hint" class="aw-ledger__hint">{{ segment.hint }}</span>
      </div>
    </div>
  </section>
</template>
