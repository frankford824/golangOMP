<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import { settlementHubTabs } from '@aw/app/navigation'

const SETTLEMENT_HUB_TAB_KEY = 'aw-settlement-hub-tab'

const route = useRoute()
const router = useRouter()
const tablistRef = ref<HTMLElement | null>(null)
const { bootstrap } = useAssetWorkbenchBootstrap()

const tabs = computed(() => settlementHubTabs(bootstrap.value))
const activePath = computed(() => route.path)

function isActive(path: string) {
  return activePath.value === path
}

function rememberTab(path: string) {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(SETTLEMENT_HUB_TAB_KEY, path)
}

async function goTab(path: string) {
  rememberTab(path)
  await router.push(path)
}

function onTabKeydown(event: KeyboardEvent, index: number) {
  const total = tabs.value.length
  if (!total) return
  if (event.key === 'ArrowRight') {
    event.preventDefault()
    void goTab(tabs.value[(index + 1) % total].path)
    return
  }
  if (event.key === 'ArrowLeft') {
    event.preventDefault()
    void goTab(tabs.value[(index - 1 + total) % total].path)
    return
  }
  if (event.key === 'Home') {
    event.preventDefault()
    void goTab(tabs.value[0].path)
    return
  }
  if (event.key === 'End') {
    event.preventDefault()
    void goTab(tabs.value[total - 1].path)
  }
}

async function scrollActiveIntoView() {
  await nextTick()
  const activeEl = tablistRef.value?.querySelector<HTMLElement>('[aria-selected="true"]')
  activeEl?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
}

watch(
  () => route.path,
  () => {
    if (tabs.value.some((tab) => tab.path === route.path)) {
      rememberTab(route.path)
    }
    void scrollActiveIntoView()
  },
)

onMounted(() => {
  void scrollActiveIntoView()
})
</script>

<template>
  <nav v-if="tabs.length > 1" ref="tablistRef" class="aw-hub-tabs" role="tablist" aria-label="结算与统计">
    <button
      v-for="(tab, index) in tabs"
      :key="tab.path"
      class="aw-hub-tabs__tab"
      :class="{ 'aw-hub-tabs__tab--active': isActive(tab.path) }"
      type="button"
      role="tab"
      :aria-selected="isActive(tab.path)"
      :tabindex="isActive(tab.path) ? 0 : -1"
      @click="goTab(tab.path)"
      @keydown="onTabKeydown($event, index)"
    >
      <span>{{ tab.label }}</span>
      <small>{{ tab.subtitle }}</small>
    </button>
  </nav>
</template>
