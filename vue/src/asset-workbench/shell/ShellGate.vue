<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useAssetWorkbenchBootstrap } from '../app/useAssetWorkbenchBootstrap'
import SimpleShell from './SimpleShell.vue'
import WorkbenchShell from './WorkbenchShell.vue'

const route = useRoute()
const router = useRouter()
const { bootstrap, loading, error, refresh } = useAssetWorkbenchBootstrap()

const allowedSimplePaths = new Set(['/', '/upload', '/my-settlement', '/account'])
const isAdmin = computed(() => bootstrap.value?.is_admin === true)

function redirectSimpleUser() {
  if (!bootstrap.value || isAdmin.value) return
  if (!allowedSimplePaths.has(route.path)) {
    void router.replace('/')
  }
}

onMounted(() => {
  void refresh()
})

watch([bootstrap, () => route.path], redirectSimpleUser)
</script>

<template>
  <div v-if="loading && !bootstrap" class="aw-root aw-root--simple">
    <main class="aw-simple">
      <p class="aw-inline-alert">正在加载工作台</p>
    </main>
  </div>
  <div v-else-if="error && !bootstrap" class="aw-root aw-root--simple">
    <main class="aw-simple">
      <p class="aw-inline-alert">{{ error }}</p>
    </main>
  </div>
  <WorkbenchShell v-else-if="isAdmin" />
  <SimpleShell v-else :bootstrap="bootstrap" />
</template>
