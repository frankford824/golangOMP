<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useAssetWorkbenchBootstrap } from '../app/useAssetWorkbenchBootstrap'
import { assetWorkbenchApi, type WorkbenchEntryResult } from '../shared/api/assetWorkbenchApi'
import SimpleShell from './SimpleShell.vue'
import WorkbenchShell from './WorkbenchShell.vue'

const route = useRoute()
const router = useRouter()
const { bootstrap, loading, error, setBootstrap } = useAssetWorkbenchBootstrap()
const entry = ref<WorkbenchEntryResult | null>(null)
const entryLoading = ref(false)
const entryError = ref('')

const allowedSimplePaths = new Set(['/', '/upload', '/my-settlement', '/account'])
const isAdmin = computed(() => bootstrap.value?.is_admin === true)
const blockedState = computed(() => entry.value && entry.value.state !== 'ready' ? entry.value : null)

function redirectSimpleUser() {
  if (!bootstrap.value || isAdmin.value) return
  if (!allowedSimplePaths.has(route.path)) {
    void router.replace('/')
  }
}

onMounted(() => {
  void loadEntry()
})

watch([bootstrap, () => route.path], redirectSimpleUser)

async function loadEntry() {
  entryLoading.value = true
  entryError.value = ''
  try {
    const result = await assetWorkbenchApi.entry()
    entry.value = result
    setBootstrap(result.bootstrap ?? null)
  } catch (err) {
    entryError.value = err instanceof Error ? err.message : '工作台入口加载失败'
  } finally {
    entryLoading.value = false
  }
}
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
