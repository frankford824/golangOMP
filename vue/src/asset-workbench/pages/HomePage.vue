<script setup lang="ts">
import { computed, onMounted } from 'vue'

import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'
import DashboardPage from './DashboardPage.vue'
import SimpleHomePage from './simple/SimpleHomePage.vue'

const { bootstrap, loading: sessionLoading, error: sessionError, refresh } = useAssetWorkbenchBootstrap()
const bootstrapRequest = usePageRequest(() => refresh())
const loading = computed(() => (bootstrapRequest.loading.value || sessionLoading.value) && !bootstrap.value)
const error = computed(() => bootstrapRequest.error.value || sessionError.value)

onMounted(() => {
  void bootstrapRequest.run()
})
</script>

<template>
  <AsyncBoundary
    :loading="loading"
    :error="error"
    :empty="!bootstrap"
    loading-label="正在加载工作台"
    empty-label="暂无工作台信息"
    @retry="bootstrapRequest.run"
  >
    <DashboardPage v-if="bootstrap?.is_admin" />
    <SimpleHomePage v-else-if="bootstrap && !bootstrap.is_admin" />
  </AsyncBoundary>
</template>
