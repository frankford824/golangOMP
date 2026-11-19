<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import DetailBreadcrumb from './DetailBreadcrumb.vue'
import { oaReturnLabel } from '@/oa/pageTitle'
import OAPageReturn from './OAPageReturn.vue'
import { detailReturnTarget } from '../returnNavigation'

defineProps<{ code?: string }>()
const route = useRoute()
const router = useRouter()
const target = computed(() => detailReturnTarget(router, route.fullPath, '/asset-center'))
const label = computed(() => oaReturnLabel(target.value))
</script>

<template>
  <OAPageReturn :to="target">{{ label }}</OAPageReturn>
  <DetailBreadcrumb module="资产管理" :identifier="code || String(route.params.id)" />
</template>
