<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from 'lucide-vue-next'
import DesktopSlot from './DesktopSlot.vue'
import OAReturnLink from './OAReturnLink.vue'
import { safeReturnPath, workspaceOrigin } from '../returnNavigation'

defineProps<{ code?: string }>()
const route = useRoute()
const router = useRouter()
const searchOrigin = computed(() => workspaceOrigin(router, route.query.globalFrom))
const target = computed(() => {
  const isAssetList = (path: string) => path === '/asset-center'
  return searchOrigin.value || safeReturnPath(route.query.assetFrom, isAssetList)
    || safeReturnPath(router.options.history.state?.back, isAssetList) || '/asset-center'
})
const label = computed(() => {
  if (searchOrigin.value) return '返回搜索来源'
  return new URL(target.value, 'https://return.invalid').searchParams.get('view') === 'ledger'
    ? '返回规格台账' : '返回资产中心'
})
</script>

<template>
  <DesktopSlot target="title">
    <nav class="asset-detail-nav" aria-label="资产详情导航">
      <OAReturnLink class="asset-back" :to="target"><ArrowLeft :size="16" aria-hidden="true" />{{ label }}</OAReturnLink>
      <span class="asset-path-title">资产详情</span>
      <span v-if="code" class="asset-path-code" :title="code">{{ code }}</span>
    </nav>
  </DesktopSlot>
</template>

<style scoped>
.asset-detail-nav { display: flex; min-width: 0; align-items: center; gap: 12px; }
.asset-detail-nav :deep(.asset-back) {
  display: inline-flex; align-items: center; justify-content: center; gap: 6px; flex: none;
  height: 32px; padding: 0 10px; border: 1px solid var(--yb-line-control); border-radius: 6px;
  background: rgb(var(--yb-surface)); color: var(--oa-ink); font: 500 13px/1 var(--oa-font);
  text-decoration: none; white-space: nowrap;
}
.asset-detail-nav :deep(.asset-back:hover) { color: var(--oa-accent); border-color: var(--oa-accent); }
.asset-detail-nav :deep(.asset-back:focus-visible) { outline: 2px solid var(--oa-accent); outline-offset: 2px; }
.asset-path-title { font: 600 15px/1.4 var(--oa-font); white-space: nowrap; }
.asset-path-code { overflow: hidden; max-width: 18ch; text-overflow: ellipsis; white-space: nowrap; color: var(--oa-secondary); font: 400 13px/1.5 var(--yb-font-mono); }
</style>
