<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import DesktopSlot from './DesktopSlot.vue'
import { oaReturnLabel } from '@/oa/pageTitle'
import OAPageReturn from './OAPageReturn.vue'
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
const label = computed(() => oaReturnLabel(target.value))
</script>

<template>
  <OAPageReturn :to="target">{{ label }}</OAPageReturn>
  <DesktopSlot target="title">
    <nav class="asset-detail-nav" aria-label="资产详情导航">
      <span class="asset-path-title">资产详情</span>
      <span v-if="code" class="asset-path-code" :title="code">{{ code }}</span>
    </nav>
  </DesktopSlot>
</template>

<style scoped>
.asset-detail-nav { display: flex; min-width: 0; align-items: center; gap: 12px; }
.asset-path-title { font: 600 15px/1.4 var(--oa-font); white-space: nowrap; }
.asset-path-code { overflow: hidden; max-width: 18ch; text-overflow: ellipsis; white-space: nowrap; color: var(--oa-secondary); font: 400 13px/1.5 var(--yb-font-mono); }
</style>
