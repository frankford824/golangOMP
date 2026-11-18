<script setup lang="ts">
import { ChartNoAxesColumn, Files, Inbox, ListFilter, SearchX } from 'lucide-vue-next'

withDefaults(defineProps<{
  title: string
  compact?: boolean
  description?: string
  kind?: 'list' | 'search' | 'inbox' | 'chart' | 'files'
}>(), { kind: 'list' })
const icons = { list: ListFilter, search: SearchX, inbox: Inbox, chart: ChartNoAxesColumn, files: Files }
</script>

<template>
  <div class="oa-empty-state" :class="{ 'oa-empty-state--compact': compact }" role="status">
    <span class="oa-empty-state__mark" aria-hidden="true"><component :is="icons[kind]" :size="24" :stroke-width="1.5" /></span>
    <strong>{{ title }}</strong>
    <span v-if="description" class="oa-empty-state__description">{{ description }}</span>
    <div v-if="$slots.default" class="oa-empty-state__actions"><slot /></div>
  </div>
</template>

<style scoped>
.oa-empty-state { display: grid; justify-items: center; align-content: center; gap: 4px; min-height: 112px; padding: 16px; text-align: center; color: var(--oa-ink); }
.oa-empty-state__mark { display: grid; place-items: center; width: 44px; height: 44px; border-radius: 12px; margin-bottom: 4px; border: 1px solid rgb(var(--yb-brand-border)); background: rgb(var(--yb-brand-soft)); color: var(--oa-accent); }
.oa-empty-state strong { font-size: 14px; line-height: 1.55; font-weight: 600; }
.oa-empty-state__description { max-width: 36em; color: var(--oa-secondary); font-size: 12px; line-height: 1.45; }
.oa-empty-state__actions { margin-top: 4px; }
.oa-empty-state--compact { min-height: 64px; padding: 8px; }
.oa-empty-state--compact .oa-empty-state__mark { width: 32px; height: 32px; border-radius: 8px; margin-bottom: 0; }
.oa-empty-state--compact .oa-empty-state__mark svg { width: 18px; height: 18px; }
@media (prefers-reduced-motion: no-preference) {
  .oa-empty-state__mark { animation: oa-empty-enter 200ms ease-out both; }
  @keyframes oa-empty-enter { from { opacity: 0; transform: translateY(4px); } to { opacity: 1; transform: translateY(0); } }
}
</style>
