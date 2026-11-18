<script setup lang="ts">
import DesktopSlot from './DesktopSlot.vue'
import { computed, nextTick, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useOAAccess } from '../access'

defineOptions({ inheritAttrs: false })
defineProps<{ query?: Record<string, string> }>()
const route = useRoute()
const { currentProfile, can } = useOAAccess()
const nav = ref<HTMLElement>()
const links = computed(() => [
  { path: '/operations', title: '经营总览' },
  { path: '/operations/products', title: '商品表现' },
  ...(can('pricing.view') ? [{ path: '/operations/pricing', title: 'SKU 核价' }] : []),
  { path: '/operations/rankings', title: '经营排行' },
  { path: '/operations/orders/detail', title: '订单明细' },
  { path: '/operations/orders/refunds', title: '售后退款' },
  ...(currentProfile.value?.super_admin ? [{ path: '/operations/orders/ledger', title: '经营台账' }] : []),
  { path: '/me/performance', title: '我的业绩' },
])
function active(path: string) {
  return route.path === path || (path === '/operations/products' && route.path.startsWith(path + '/'))
}
watch(() => route.path, async () => {
  await nextTick()
  const el = nav.value
  const selected = el?.querySelector<HTMLElement>('[aria-current="page"]')
  if (el && selected && el.scrollWidth > el.clientWidth) {
    el.scrollLeft += selected.getBoundingClientRect().left - el.getBoundingClientRect().left - 16
  }
}, { immediate: true })
</script>

<template>
  <DesktopSlot target="tabs">
  <nav v-bind="$attrs" ref="nav" class="operation-nav" aria-label="经营工作区">
    <RouterLink v-for="link in links" :key="link.path" :to="{ path: link.path, query }"
      :class="{ selected: active(link.path) }" :aria-current="active(link.path) ? 'page' : undefined">{{ link.title }}</RouterLink>
  </nav>
  </DesktopSlot>
</template>

<style scoped>
.operation-nav { display: flex; align-items: stretch; gap: 24px; min-width: 0; max-width: 100%; overflow-x: auto; border-block-end: 1px solid var(--oa-line); scrollbar-width: thin; }
.operation-nav a { flex: none; display: inline-flex; align-items: center; min-height: 40px; padding-inline: 4px; border-bottom: 2px solid transparent; color: var(--oa-secondary); font-size: 14px; font-weight: 400; line-height: 1.5; text-decoration: none; }
.operation-nav a:hover, .operation-nav a.selected { color: var(--oa-accent); }
.operation-nav a.selected { border-bottom-color: currentColor; font-weight: 600; }
.operation-nav a:focus-visible { outline: 2px solid var(--oa-accent); outline-offset: -2px; border-radius: 4px; }
@media (max-width: 850px) { .operation-nav { gap: 12px; } }
</style>
