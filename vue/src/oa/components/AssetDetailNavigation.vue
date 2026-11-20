<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import DetailBreadcrumb from './DetailBreadcrumb.vue'
import { oaReturnLabel } from '@/oa/pageTitle'
import OAPageReturn from './OAPageReturn.vue'
import AssetViewTabs from './AssetViewTabs.vue'
import { assetListReturn } from '../assetNavigation'
import { detailReturnTarget, goBackTo, workspaceOrigin } from '../returnNavigation'

defineProps<{ code?: string }>()
const route = useRoute()
const router = useRouter()
const list = computed(() => router.resolve(assetListReturn(route.query.assetFrom)))
const target = computed(() => detailReturnTarget(router, route.fullPath, workspaceOrigin(router, route.query.globalFrom) || list.value.fullPath))
const origin = computed(() => {
  const previous = router.resolve(target.value)
  return previous.name === 'AssetsIndex' ? previous : list.value
})
const flat = computed(() => ['resource_role', 'file_format', 'resource_owner_id', 'created_from', 'created_to'].some(key => Boolean(origin.value.query[key])))
function openView(ledger: boolean) {
  if (ledger === (origin.value.query.view === 'ledger') && origin.value.fullPath === target.value) {
    void goBackTo(router, target.value)
  } else {
    void router.push({ name: 'AssetsIndex', query: { ...origin.value.query, view: ledger ? 'ledger' : undefined }, hash: origin.value.hash })
  }
}
const label = computed(() => oaReturnLabel(target.value))
</script>

<template>
  <AssetViewTabs v-if="!flat" :ledger="origin.query.view === 'ledger'" @select="openView" />
  <DetailBreadcrumb module="资产管理" :identifier="code || String(route.params.id)">
    <OAPageReturn inline :to="target">{{ label }}</OAPageReturn>
  </DetailBreadcrumb>
</template>
