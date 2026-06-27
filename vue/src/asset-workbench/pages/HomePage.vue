<script setup lang="ts">
import { onMounted } from 'vue'

import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import DashboardPage from './DashboardPage.vue'
import SimpleHomePage from './simple/SimpleHomePage.vue'

const { bootstrap, loading, error, refresh } = useAssetWorkbenchBootstrap()

onMounted(() => {
  void refresh()
})
</script>

<template>
  <DashboardPage v-if="bootstrap?.is_admin" />
  <SimpleHomePage v-else-if="bootstrap && !bootstrap.is_admin" />
  <p v-else-if="error" class="aw-inline-alert">{{ error }}</p>
  <p v-else-if="loading" class="aw-inline-alert">正在加载工作台</p>
</template>
