<script setup lang="ts">
import { computed, onMounted } from 'vue'

import { useAssetWorkbenchBootstrap } from '../app/useAssetWorkbenchBootstrap'
import SimpleShell from './SimpleShell.vue'
import WorkbenchShell from './WorkbenchShell.vue'

const { bootstrap, loading, error, entry, entryLoading, entryError, loadEntry } = useAssetWorkbenchBootstrap()
const isAdmin = computed(() => bootstrap.value?.is_admin === true)
const blockedState = computed(() => entry.value && entry.value.state !== 'ready' ? entry.value : null)

onMounted(() => {
  void loadEntry()
})
</script>

<template>
  <div v-if="(loading || entryLoading) && !bootstrap" class="aw-root aw-root--simple">
    <main class="aw-simple">
      <p class="aw-inline-alert">正在加载工作台</p>
    </main>
  </div>
  <div v-else-if="(error || entryError) && !bootstrap" class="aw-root aw-root--simple">
    <main class="aw-simple">
      <p class="aw-inline-alert">{{ error || entryError }}</p>
    </main>
  </div>
  <div v-else-if="blockedState" class="aw-root aw-root--simple">
    <main class="aw-simple">
      <section class="aw-simple-card">
        <p class="aw-eyebrow">资产工作台</p>
        <h1>{{ blockedState.state === 'not_member' ? '尚未开通' : blockedState.state === 'pending' ? '等待处理' : blockedState.state === 'merged' ? '请使用主账号登录' : '暂不可用' }}</h1>
        <p>{{ blockedState.message }}</p>
      </section>
    </main>
  </div>
  <WorkbenchShell v-else-if="isAdmin" />
  <SimpleShell v-else :bootstrap="bootstrap" />
</template>
